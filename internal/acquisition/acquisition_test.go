//
// Tests for internal/acquisition — aquisição de evidência externa.
//
// Cobre:
//   - Guard de SSRF fail-closed: file://, ftp://, http://127.0.0.1,
//     http://169.254.169.254, http://[::1], http://10.0.0.5, http://192.168.1.1,
//     http://172.16.0.1 e localhost rejeitados; alvos públicos liberados com
//     AllowRemote (--allow-remote)
//   - Fetch contra httptest server: SHA256, tamanho, content-type, proveniência
//     "UNTRUSTED"; limite de corpo (10 MiB) e teto de redirects (3)
//   - ArtifactStore: persistência separada do corpo em .cosca/quarantine/artifacts
//   - QuarantineArtifact: cria proposal Q-XXXX (pending, external) com referência
//     (nunca dump completo)
//   - NormalizeName sanitiza a URL para nome de arquivo seguro
//

package acquisition

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/quarantine"
)

// =============================================================================
// SSRF guard — fail-closed
// =============================================================================

func TestValidateTarget_RejectsNonHTTPSchemes(t *testing.T) {
	c := NewClient(0, 0, 0)
	c.AllowRemote = true
	for _, u := range []string{
		"file:///etc/passwd",
		"file:///tmp/evil.sh",
		"ftp://ftp.example.com/file",
		"gopher://example.com:70/x",
		"data:text/plain,hello",
	} {
		if err := c.validateTarget(u); err == nil {
			t.Errorf("esquema deveria ser rejeitado: %q", u)
		}
	}
}

func TestValidateTarget_RejectsPrivateLinkLocalIPs(t *testing.T) {
	c := NewClient(0, 0, 0)
	c.AllowRemote = true
	for _, u := range []string{
		"http://127.0.0.1/",
		"http://127.0.0.2/secret",
		"http://10.0.0.5/internal",
		"http://172.16.0.1/router",
		"http://172.31.255.254/admin",
		"http://192.168.1.1/",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/",
		"http://localhost:8080/",
	} {
		if err := c.validateTarget(u); err == nil {
			t.Errorf("IP privado/loopback deveria ser rejeitado: %q", u)
		}
	}
}

func TestValidateTarget_RequiresExplicitAllowRemote(t *testing.T) {
	// Fail-closed: sem AllowRemote, até https público é recusado.
	c := NewClient(0, 0, 0)
	c.AllowRemote = false
	if err := c.validateTarget("https://github.com/CoscaAI/cosca"); err == nil {
		t.Fatal("fetch deveria ser recusado sem --allow-remote")
	}
}

func TestValidateTarget_AllowsPublicTargetsWhenExplicit(t *testing.T) {
	c := NewClient(0, 0, 0)
	c.AllowRemote = true
	// IP público literal — prova que o guard libera destinos públicos.
	if err := c.validateTarget("https://8.8.8.8/dns"); err != nil {
		t.Errorf("IP público deveria ser liberado: %v", err)
	}
	// localhost com AllowLoopback (mock servers locais).
	lb := NewClient(0, 0, 0)
	lb.AllowRemote = true
	lb.AllowLoopback = true
	if err := lb.validateTarget("http://127.0.0.1:8080/x"); err != nil {
		t.Errorf("loopback com AllowLoopback deveria ser liberado: %v", err)
	}
	if err := lb.validateTarget("http://localhost:8080/x"); err != nil {
		t.Errorf("localhost com AllowLoopback deveria ser liberado: %v", err)
	}
}

func TestBlockedIP_Table(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":            true,
		"127.255.255.255":      true,
		"10.0.0.1":             true,
		"10.255.255.255":       true,
		"172.16.0.1":           true,
		"172.31.255.254":       true,
		"172.32.0.1":           false,
		"192.168.0.1":          true,
		"192.168.255.255":      true,
		"169.254.169.254":      true,
		"169.254.0.1":          true,
		"::1":                  true,
		"8.8.8.8":              false,
		"1.1.1.1":              false,
		"173.194.206.100":      false,
		"::ffff:127.0.0.1":     true, // IPv4-mapped loopback
		"2001:4860:4860::8888": false,
	}
	for ip, want := range cases {
		got := blockedIP(net.ParseIP(ip))
		if got != want {
			t.Errorf("blockedIP(%s) = %v, want %v", ip, got, want)
		}
	}
}

// =============================================================================
// Fetch contra httptest — hash, tamanho, content-type, limites
// =============================================================================

func newTestClient(allowLoopback bool) *Client {
	c := NewClient(0, 0, 0)
	c.AllowRemote = true
	c.AllowLoopback = allowLoopback
	return c
}

func TestFetch_HashSizeContentTypeAndUntrustedProvenance(t *testing.T) {
	content := []byte("hello cosca external evidence\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	art, body, err := newTestClient(true).FetchAll(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if string(body) != string(content) {
		t.Fatalf("corpo divergente: %q != %q", body, content)
	}
	sum := sha256.Sum256(content)
	if art.SHA256 != fmt.Sprintf("%x", sum[:]) {
		t.Errorf("SHA256 errado: %s", art.SHA256)
	}
	if art.SizeBytes != int64(len(content)) {
		t.Errorf("tamanho errado: %d", art.SizeBytes)
	}
	if !strings.HasPrefix(art.ContentType, "text/plain") {
		t.Errorf("content-type errado: %q", art.ContentType)
	}
	if art.Provenance != ProvenanceUntrusted {
		t.Errorf("proveniência deveria ser UNTRUSTED, got %q", art.Provenance)
	}
	if art.RetrievedAt.IsZero() {
		t.Error("RetrievedAt não deveria ser zero")
	}
	if art.ID != "" {
		t.Errorf("Fetch não deveria atribuir ID (persistência é do store): %q", art.ID)
	}

	// Fetch simples também funciona.
	art2, err := newTestClient(true).Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if art2.SHA256 != art.SHA256 {
		t.Errorf("Fetch/FetchAll deveriam concordar no hash")
	}
}

func TestFetch_RejectsLoopbackWithoutAllowLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	// AllowRemote ligado, mas loopback NÃO permitido → SSRF guard bloqueia.
	if _, err := newTestClient(false).Fetch(context.Background(), srv.URL); err == nil {
		t.Fatal("fetch para 127.0.0.1 sem AllowLoopback deveria falhar (SSRF guard)")
	}
}

func TestFetch_BodyCapEnforced(t *testing.T) {
	big := make([]byte, 11<<20) // > 10 MiB
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(big)
	}))
	defer srv.Close()

	_, _, err := newTestClient(true).FetchAll(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "excede") {
		t.Fatalf("esperava erro de corpo acima do limite, got %v", err)
	}
}

func TestFetch_RedirectCapEnforced(t *testing.T) {
	// 2 redirects → ok dentro do teto de 3.
	two := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			http.Redirect(w, r, "/b", http.StatusFound)
		case "/b":
			http.Redirect(w, r, "/c", http.StatusFound)
		default:
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer two.Close()

	art, _, err := newTestClient(true).FetchAll(context.Background(), two.URL+"/a")
	if err != nil {
		t.Fatalf("2 redirects deveriam ser permitidos: %v", err)
	}
	if art.SHA256 == "" {
		t.Error("hash vazio após redirects")
	}

	// 3 redirects → erro (parado após 3).
	three := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			http.Redirect(w, r, "/b", http.StatusFound)
		case "/b":
			http.Redirect(w, r, "/c", http.StatusFound)
		case "/c":
			http.Redirect(w, r, "/d", http.StatusFound)
		default:
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer three.Close()

	_, _, err = newTestClient(true).FetchAll(context.Background(), three.URL+"/a")
	if err == nil || !strings.Contains(err.Error(), "redirects") {
		t.Fatalf("3 redirects deveriam estourar o teto, got %v", err)
	}
}

func TestFetch_Non200Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()
	if _, err := newTestClient(true).Fetch(context.Background(), srv.URL); err == nil {
		t.Fatal("HTTP 404 deveria ser erro")
	}
}

// =============================================================================
// ArtifactStore — persistência separada do corpo
// =============================================================================

func TestArtifactStore_AddGetListAndSeparateBody(t *testing.T) {
	root := t.TempDir()
	store := NewArtifactStore(root)
	body := []byte("corpo do artefato externo\n")

	art := &AcquiredArtifact{
		URL:         "https://example.com/doc.txt",
		ContentType: "text/plain",
		SHA256:      fmt.Sprintf("%x", sha256.Sum256(body)),
		SizeBytes:   int64(len(body)),
		RetrievedAt: time.Now().UTC(),
		Provenance:  ProvenanceUntrusted,
	}
	id, err := store.Add(art, body)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id != "A-0001" {
		t.Errorf("primeiro ID deveria ser A-0001, got %s", id)
	}
	if art.ID != "A-0001" {
		t.Errorf("art.ID deveria ser A-0001, got %s", art.ID)
	}

	// Metadados A-0001.json e corpo A-0001 (separados).
	meta := filepath.Join(root, ".cosca", "quarantine", "artifacts", "A-0001.json")
	bodyPath := filepath.Join(root, ".cosca", "quarantine", "artifacts", "A-0001")
	if _, err := os.Stat(meta); err != nil {
		t.Errorf("metadados não gravados: %v", err)
	}
	raw, err := os.ReadFile(bodyPath)
	if err != nil || string(raw) != string(body) {
		t.Errorf("corpo não persistido separadamente: %v", err)
	}

	// Get retorna os metadados.
	got, err := store.Get("a-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "A-0001" || got.SHA256 != art.SHA256 || got.URL != art.URL {
		t.Errorf("Get divergente: %+v", got)
	}
	if got.Provenance != ProvenanceUntrusted {
		t.Errorf("proveniência deveria permanecer UNTRUSTED, got %q", got.Provenance)
	}

	// ReadBody.
	rb, err := store.ReadBody("A-0001")
	if err != nil || string(rb) != string(body) {
		t.Errorf("ReadBody: %v", err)
	}

	// List.
	arts, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(arts) != 1 || arts[0].ID != "A-0001" {
		t.Errorf("List errada: %+v", arts)
	}

	// IDs incrementam.
	art2 := &AcquiredArtifact{URL: "https://example.com/other"}
	id2, err := store.Add(art2, []byte("segundo"))
	if err != nil {
		t.Fatalf("Add segundo: %v", err)
	}
	if id2 != "A-0002" {
		t.Errorf("segundo ID deveria ser A-0002, got %s", id2)
	}
}

func TestArtifactStore_EmptyListNoError(t *testing.T) {
	store := NewArtifactStore(t.TempDir())
	arts, err := store.List()
	if err != nil || len(arts) != 0 {
		t.Fatalf("List vazia deveria ser [], got %+v err %v", arts, err)
	}
}

func TestArtifactStore_GetMissing(t *testing.T) {
	store := NewArtifactStore(t.TempDir())
	if _, err := store.Get("A-9999"); err == nil {
		t.Fatal("Get de artefato inexistente deveria falhar")
	}
}

func TestNormalizeArtifactID(t *testing.T) {
	for in, want := range map[string]string{
		"A-0001": "A-0001",
		"a-1":    "A-0001",
		"A-1":    "A-0001",
		"0002":   "A-0002",
		"2":      "A-0002",
	} {
		got, err := NormalizeArtifactID(in)
		if err != nil || got != want {
			t.Errorf("NormalizeArtifactID(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "B-1", "A-", "-1", "abc"} {
		if _, err := NormalizeArtifactID(bad); err == nil {
			t.Errorf("NormalizeArtifactID(%q) deveria falhar", bad)
		}
	}
}

func TestNormalizeName_SanitizesURL(t *testing.T) {
	cases := map[string]string{
		"https://raw.githubusercontent.com/CoscaAI/cosca/main/README.md": "raw-githubusercontent-com-CoscaAI-cosca-main-README-md",
		"https://example.com/doc.txt":                                    "example-com-doc-txt",
		"https://example.com":                                            "example-com",
		"file:///etc/passwd":                                             "etc-passwd",
		"https://host/../..//traversal":                                  "host-traversal",
		"":                                                               "artifact",
		"not-a-url":                                                      "not-a-url",
	}
	for in, want := range cases {
		if got := NormalizeName(in); got != want {
			t.Errorf("NormalizeName(%q) = %q, want %q", in, got, want)
		}
	}
	// Sem barras nem ".." → sem path traversal.
	for _, in := range []string{"../..", "/etc/passwd", "a/../../b", "..", ".", "...."} {
		name := NormalizeName(in)
		if strings.Contains(name, "/") || strings.Contains(name, "..") {
			t.Errorf("NormalizeName(%q) = %q não deveria conter '/' nem '..'", in, name)
		}
		if name == "" || name == "." || name == ".." {
			t.Errorf("NormalizeName(%q) = %q vazio/reservado", in, name)
		}
	}
}

// =============================================================================
// QuarantineArtifact — proposal de referência, nunca dump completo
// =============================================================================

func TestQuarantineArtifact_CreatesPendingExternalProposal(t *testing.T) {
	root := t.TempDir()
	qstore := quarantine.NewStore(root)
	body := []byte("conteúdo real do artefato que não deve ir para a quarentena")

	art := &AcquiredArtifact{
		ID:          "A-0001",
		URL:         "https://github.com/CoscaAI/cosca/blob/main/README.md",
		ContentType: "text/markdown",
		SHA256:      fmt.Sprintf("%x", sha256.Sum256(body)),
		SizeBytes:   int64(len(body)),
		RetrievedAt: time.Now().UTC(),
		Provenance:  ProvenanceUntrusted,
		Notes:       "trecho curto",
	}

	qID, err := QuarantineArtifact(qstore, art, "teste de integração")
	if err != nil {
		t.Fatalf("QuarantineArtifact: %v", err)
	}
	if qID != "Q-0001" {
		t.Errorf("primeira proposal deveria ser Q-0001, got %s", qID)
	}

	prop, err := qstore.Get(qID)
	if err != nil {
		t.Fatalf("Get proposal: %v", err)
	}
	if prop.Status != quarantine.StatusPending {
		t.Errorf("status deveria ser pending, got %q", prop.Status)
	}
	if !strings.HasPrefix(prop.Source, "external:") {
		t.Errorf("source deveria ser external:<url>, got %q", prop.Source)
	}
	// Referência, não dump completo.
	for _, want := range []string{"A-0001", art.URL, art.SHA256, "quarantine/artifacts/A-0001", "UNTRUSTED"} {
		if !strings.Contains(prop.Content, want) {
			t.Errorf("Content deveria conter %q (referência): %s", want, prop.Content)
		}
	}
	if strings.Contains(prop.Content, string(body)) {
		t.Error("Content NÃO deveria conter o corpo completo do artefato")
	}
	// O corpo completo não está na proposal (não é dump).
	if strings.Contains(prop.Content, "conteúdo real do artefato que não deve ir") {
		t.Error("Content deveria conter apenas o trecho, não o corpo")
	}
}

func TestQuarantineArtifact_RequiresPersistedArtifact(t *testing.T) {
	qstore := quarantine.NewStore(t.TempDir())
	art := &AcquiredArtifact{URL: "https://example.com", ID: ""}
	if _, err := QuarantineArtifact(qstore, art, ""); err == nil {
		t.Fatal("artefato sem ID deveria falhar")
	}
	if _, err := QuarantineArtifact(nil, art, ""); err == nil {
		t.Fatal("store nil deveria falhar")
	}
}

// =============================================================================
// compile-time guard
// =============================================================================

var _ = time.Second
