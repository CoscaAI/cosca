package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestSafeClient devolve um client de teste com transport SEM proxy, para que
// a asserção prove o bloqueio SSRF (não um timeout de proxy do ambiente).
func newTestSafeClient(timeoutSec int, maxRedirects int) *http.Client {
	c := NewSafeClient(time.Duration(timeoutSec)*time.Second, maxRedirects)
	c.Transport = &http.Transport{Proxy: nil}
	return c
}

// TestSafeClient_BlocksRedirectToPrivate é o caso de uso crítico do ADR-041:
// um servidor que serve de "primeiro salto" redireciona para o metadata service.
// O client deve BLOQUEAR o redirect (fail-closed) — a primeira URL parecia
// pública, mas o destino é privado.
func TestSafeClient_BlocksRedirectToPrivate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/", http.StatusFound)
	}))
	defer server.Close()

	_, err := newTestSafeClient(5, 3).Get(server.URL)
	if err == nil {
		t.Fatal("redirect para metadata service deveria ser bloqueado")
	}
}

// TestSafeClient_BlocksRedirectToMetadataHost redireciona para um hostname de
// metadata — rejeitado por NOME (defesa em profundidade, antes do DNS).
func TestSafeClient_BlocksRedirectToMetadataHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://metadata.google.internal/", http.StatusFound)
	}))
	defer server.Close()

	_, err := newTestSafeClient(5, 3).Get(server.URL)
	if err == nil {
		t.Fatal("redirect para hostname de metadata deveria ser bloqueado")
	}
}

// TestSafeClient_MaxRedirects valida o teto de saltos do redirect.
func TestSafeClient_MaxRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path, http.StatusFound)
	}))
	defer server.Close()

	_, err := newTestSafeClient(5, 2).Get(server.URL)
	if err == nil {
		t.Fatal("redirect em loop deveria ser abortado pelo teto de saltos")
	}
}

// TestSafeClient_BlocksNonHTTPSchema redireciona para um protocolo não http(s).
func TestSafeClient_BlocksNonHTTPSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "ftp://example.com/")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	_, err := newTestSafeClient(5, 3).Get(server.URL)
	if err == nil {
		t.Fatal("redirect para protocolo não-http(s) deveria ser bloqueado")
	}
}

// TestSafeClient_InitialRequestAllowed verifica que um primeiro GET (sem
// redirect) não é barrado pelo CheckRedirect — o redirect só é validado nos
// saltos. A conexão AO SERVIDOR é direta (sem proxy) para o teste ser
// determinístico e provar que o client não bloqueia um request legítimo.
func TestSafeClient_InitialRequestAllowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	resp, err := newTestSafeClient(5, 3).Get(server.URL)
	if err != nil {
		t.Fatalf("primeira requisição não deveria ser barrada: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

// TestSafeClient_BlockedErrorIsDescriptive garante que o erro de bloqueio é
// instrutivo (não vazio).
func TestSafeClient_BlockedErrorIsDescriptive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/", http.StatusFound)
	}))
	defer server.Close()

	_, err := newTestSafeClient(5, 3).Get(server.URL)
	if err == nil || err.Error() == "" {
		t.Fatal("erro de bloqueio deveria ser descritivo")
	}
}
