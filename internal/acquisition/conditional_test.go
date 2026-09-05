package acquisition

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testClient devolve um cliente de aquisição habilitado para testes locais
// (AllowRemote + AllowLoopback para falar com os httptest em 127.0.0.1).
func testClient() *Client {
	c := NewClient(5*time.Second, 3, 1<<20)
	c.AllowRemote = true
	c.AllowLoopback = true
	return c
}

// TestFetchConditional_304 validates o caso central do ADR-042: um request
// condicional com ETag replayado → upstream responde 304, Changed=false, Body=nil.
func TestFetchConditional_304(t *testing.T) {
	etag := `"v1"`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		_, _ = w.Write([]byte("data-versao-1"))
	}))
	defer server.Close()

	c := testClient()
	ctx := context.Background()

	// 1º poll: sem validators → 200 + ETag.
	r1, err := c.FetchConditional(ctx, server.URL, Validators{})
	if err != nil {
		t.Fatalf("primeiro fetch: %v", err)
	}
	if !r1.Changed || string(r1.Body) != "data-versao-1" {
		t.Fatalf("r1 = %+v, want changed, body data-versao-1", r1)
	}
	if r1.Validators.ETag != etag {
		t.Fatalf("ETag = %q, want %q", r1.Validators.ETag, etag)
	}

	// 2º poll: replay ETag → 304, Changed=false, Body=nil, sem erro.
	r2, err := c.FetchConditional(ctx, server.URL, r1.Validators)
	if err != nil {
		t.Fatalf("segundo fetch (304): %v", err)
	}
	if r2.Changed {
		t.Errorf("Changed deveria ser false em 304")
	}
	if r2.Body != nil {
		t.Errorf("Body deveria ser nil em 304, got %q", r2.Body)
	}
	if r2.Validators.ETag != etag {
		t.Errorf("validators ETag = %q, want %q (preservado)", r2.Validators.ETag, etag)
	}
}

// TestFetchConditional_LastModified valida o validador Last-Modified.
func TestFetchConditional_LastModified(t *testing.T) {
	lm := "Wed, 01 Jan 2026 00:00:00 GMT"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Modified-Since") == lm {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Last-Modified", lm)
		_, _ = w.Write([]byte("body"))
	}))
	defer server.Close()

	c := testClient()
	ctx := context.Background()
	r1, err := c.FetchConditional(ctx, server.URL, Validators{})
	if err != nil {
		t.Fatalf("primeiro: %v", err)
	}
	if r1.Validators.LastModified != lm {
		t.Fatalf("Last-Modified = %q, want %q", r1.Validators.LastModified, lm)
	}
	r2, err := c.FetchConditional(ctx, server.URL, r1.Validators)
	if err != nil {
		t.Fatalf("segundo (304): %v", err)
	}
	if r2.Changed {
		t.Errorf("Changed deveria ser false em 304 via Last-Modified")
	}
}

// TestFetchConditional_DecodesGzip valida a decodificação do corpo pelo header
// Content-Encoding (gzip) — não pelo Accept-Encoding.
func TestFetchConditional_DecodesGzip(t *testing.T) {
	plain := []byte("conteúdo comprimido de aquisição")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write(plain)
	_ = gz.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buf.Bytes())
	}))
	defer server.Close()

	c := testClient()
	r, err := c.FetchConditional(context.Background(), server.URL, Validators{})
	if err != nil {
		t.Fatalf("fetch gzip: %v", err)
	}
	if !strings.EqualFold(string(r.Body), string(plain)) {
		t.Fatalf("corpo decodificado = %q, want %q", r.Body, plain)
	}
}

// TestFetchConditional_SendsHonestUA valida que a requisição envia um User-Agent
// identificador honesto (não um browser-UA spoofed).
func TestFetchConditional_SendsHonestUA(t *testing.T) {
	var gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	c := testClient()
	if _, err := c.FetchConditional(context.Background(), server.URL, Validators{}); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !strings.Contains(gotUA, "Cosca-Acquisition") {
		t.Fatalf("User-Agent = %q, want conter Cosca-Acquisition", gotUA)
	}
}

// TestFetchConditional_Non200IsError valida que status != 200/304 é erro.
func TestFetchConditional_Non200IsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := testClient()
	if _, err := c.FetchConditional(context.Background(), server.URL, Validators{}); err == nil {
		t.Fatal("HTTP 500 deveria ser erro")
	}
}
