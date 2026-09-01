//
// Tests for the `cosca evidence` command tree (internal/cli/evidence.go).
//
// Cobre:
//   - Registro de `evidence` (fetch/list/show/promote) no root
//   - `evidence fetch` contra mock server (httptest): A-XXXX criado, SHA-256,
//     proposal Q-XXXX (external, pending), "UNTRUSTED — em quarentena"
//   - Fail-closed: fetch sem --allow-remote recusa; file:// rejeitado
//   - `evidence list` / `evidence show`
//   - `evidence promote A-XXXX --item K-XX`: proposal → promoted e evidência
//     adicionada (P2/P4) a um item de conhecimento temporário; sem --item
//     recusa (nunca auto-promover)
//   - proveniência P2 (repositório) vs P4 (arquivo-fonte buscado)
//
// NOTE: os testes fazem chdir() e NÃO rodam em paralelo. Formatter injetado
// via newContextWithFormatter (padrão do CLI). O mock server usa 127.0.0.1,
// então o teste liga COSCA_ACQUISITION_ALLOW_PRIVATE (opt-in explícito do
// guard de SSRF).
//

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/acquisition"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/quarantine"
)

// chdirEvidenceTemp muda para um diretório temporário e restaura o cwd no
// cleanup. Os comandos do evidence operam em <tmp>/.cosca/...
func chdirEvidenceTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Chdir registra o restore do cwd DEPOIS do RemoveAll do TempDir
	// (cleanups rodam em LIFO) — no Windows não dá para remover o
	// diretório que é o CWD do processo.
	t.Chdir(dir)
	globalFlags = GlobalFlags{}
	return dir
}

// runEvidenceCommand executa o RunE de um subcomando com formatter injetado em
// um buffer (padrão de injeção de formatter do CLI).
func runEvidenceCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	err := cmd.RunE(cmd, args)
	return buf.String(), err
}

// newEvidenceMockServer serve conteúdo determinístico em qualquer path.
func newEvidenceMockServer(t *testing.T, contentType string, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// seedKnowledgeItem grava um item de conhecimento temporário em
// <dir>/.cosca/knowledge/laws.json (via PromotionEngine — API exportada).
func seedKnowledgeItem(t *testing.T, dir, id, title string) {
	t.Helper()
	engine := knowledge.NewPromotionEngine()
	if err := engine.Register(&knowledge.KnowledgeItem{
		ID:         id,
		Title:      title,
		Confidence: 0.9,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := engine.Save(filepath.Join(dir, ".cosca", "knowledge", "laws.json")); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

// readEvidenceArtifact lê os metadados do artefato do projeto atual.
func readEvidenceArtifact(t *testing.T, id string) *acquisition.AcquiredArtifact {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	art, err := acquisition.NewArtifactStore(filepath.Join(dir, ".cosca")).Get(id)
	if err != nil {
		t.Fatalf("Get %s: %v", id, err)
	}
	return art
}

// =============================================================================
// Registration — `evidence` no root
// =============================================================================

func TestEvidenceCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "evidence" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("evidence subcommand not registered in root command")
	}
}

func TestEvidenceCommand_Properties(t *testing.T) {
	cmd := NewEvidenceCommand()
	if cmd == nil {
		t.Fatal("NewEvidenceCommand returned nil")
	}
	if cmd.Use != "evidence" {
		t.Errorf("expected Use='evidence', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long description")
	}
	expected := []string{"fetch", "list", "show", "promote"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing evidence subcommand: %s", name)
		}
	}
}

// =============================================================================
// `evidence fetch` — fail-closed
// =============================================================================

func TestEvidenceFetch_RequiresExplicitAllowRemote(t *testing.T) {
	chdirEvidenceTemp(t)
	srv := newEvidenceMockServer(t, "text/plain", []byte("hello"))

	cmd := NewEvidenceFetchCommand()
	_, err := runEvidenceCommand(t, cmd, []string{srv.URL})
	if err == nil || !strings.Contains(err.Error(), "allow-remote") {
		t.Fatalf("fetch sem --allow-remote deveria recusar, got %v", err)
	}
}

func TestEvidenceFetch_RejectsFileScheme(t *testing.T) {
	chdirEvidenceTemp(t)

	cmd := NewEvidenceFetchCommand()
	cmd.Flags().Set("allow-remote", "true")
	_, err := runEvidenceCommand(t, cmd, []string{"file:///etc/passwd"})
	if err == nil || !strings.Contains(err.Error(), "SSRF") {
		t.Fatalf("file:// deveria ser rejeitado pelo SSRF guard, got %v", err)
	}
}

func TestEvidenceFetch_RejectsPrivateIP(t *testing.T) {
	chdirEvidenceTemp(t)

	cmd := NewEvidenceFetchCommand()
	cmd.Flags().Set("allow-remote", "true")
	_, err := runEvidenceCommand(t, cmd, []string{"http://169.254.169.254/latest/meta-data"})
	if err == nil || !strings.Contains(err.Error(), "SSRF") {
		t.Fatalf("169.254.169.254 deveria ser rejeitado, got %v", err)
	}
}

// =============================================================================
// `evidence fetch` — fluxo completo com mock server
// =============================================================================

func TestEvidenceFetch_WithMockServer(t *testing.T) {
	dir := chdirEvidenceTemp(t)
	// Opt-in explícito do SSRF guard para destinos loopback (mock local).
	t.Setenv("COSCA_ACQUISITION_ALLOW_PRIVATE", "true")

	content := []byte(strings.Repeat("cosca evidence mock content\n", 200))
	srv := newEvidenceMockServer(t, "text/plain; charset=utf-8", content)
	sum := sha256.Sum256(content)
	sha := fmt.Sprintf("%x", sum[:])

	cmd := NewEvidenceFetchCommand()
	cmd.Flags().Set("allow-remote", "true")
	out, err := runEvidenceCommand(t, cmd, []string{srv.URL})
	if err != nil {
		t.Fatalf("evidence fetch: %v\n%s", err, out)
	}

	if !strings.Contains(out, "Artefato A-0001 adquirido") {
		t.Errorf("output deveria citar A-0001: %q", out)
	}
	if !strings.Contains(out, sha) {
		t.Errorf("output deveria citar o SHA-256: %q", out)
	}
	if !strings.Contains(out, "UNTRUSTED") {
		t.Errorf("output deveria marcar UNTRUSTED: %q", out)
	}
	if !strings.Contains(out, "UNTRUSTED — em quarentena, nada promovido automaticamente") {
		t.Errorf("output deveria conter o aviso de quarentena: %q", out)
	}
	if !strings.Contains(out, "Q-0001") {
		t.Errorf("output deveria citar a proposal Q-0001: %q", out)
	}

	// Artefato persistido: metadados + corpo separado.
	meta := filepath.Join(dir, ".cosca", "quarantine", "artifacts", "A-0001.json")
	bodyPath := filepath.Join(dir, ".cosca", "quarantine", "artifacts", "A-0001")
	if _, err := os.Stat(meta); err != nil {
		t.Errorf("metadados do artefato não gravados: %v", err)
	}
	raw, err := os.ReadFile(bodyPath)
	if err != nil || string(raw) != string(content) {
		t.Errorf("corpo do artefato não persistido separadamente: %v", err)
	}

	// Proposal de quarentena Q-0001 (external, pending).
	qstore := quarantine.NewStore(dir)
	prop, err := qstore.Get("Q-0001")
	if err != nil {
		t.Fatalf("proposal Q-0001 não encontrada: %v", err)
	}
	if prop.Status != quarantine.StatusPending {
		t.Errorf("proposal deveria estar pending, got %q", prop.Status)
	}
	if !strings.HasPrefix(prop.Source, "external:") {
		t.Errorf("source deveria ser external:<url>, got %q", prop.Source)
	}
	if strings.Contains(prop.Content, string(content)) {
		t.Error("proposal NÃO deveria conter o corpo completo do artefato")
	}

	art := readEvidenceArtifact(t, "A-0001")
	if art.Provenance != acquisition.ProvenanceUntrusted {
		t.Errorf("proveniência deveria ser UNTRUSTED, got %q", art.Provenance)
	}
	if art.SHA256 != sha {
		t.Errorf("SHA-256 divergente: %s", art.SHA256)
	}
}

// =============================================================================
// `evidence list` / `evidence show`
// =============================================================================

func TestEvidenceList_AfterFetch(t *testing.T) {
	dir := chdirEvidenceTemp(t)
	t.Setenv("COSCA_ACQUISITION_ALLOW_PRIVATE", "true")
	srv := newEvidenceMockServer(t, "text/plain", []byte("body-for-list"))
	fetchCmd := NewEvidenceFetchCommand()
	fetchCmd.Flags().Set("allow-remote", "true")
	if _, err := runEvidenceCommand(t, fetchCmd, []string{srv.URL}); err != nil {
		t.Fatal(err)
	}

	listCmd := NewEvidenceListCommand()
	out, err := runEvidenceCommand(t, listCmd, nil)
	if err != nil {
		t.Fatalf("evidence list: %v", err)
	}
	if !strings.Contains(out, "A-0001") {
		t.Errorf("list deveria conter A-0001: %q", out)
	}
	if !strings.Contains(out, "UNTRUSTED") {
		t.Errorf("list deveria conter UNTRUSTED: %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cosca", "quarantine", "artifacts")); err != nil {
		t.Errorf("diretório de artefatos deveria existir: %v", err)
	}
}

func TestEvidenceList_Empty(t *testing.T) {
	chdirEvidenceTemp(t)
	cmd := NewEvidenceListCommand()
	out, err := runEvidenceCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("evidence list (empty): %v", err)
	}
	if !strings.Contains(out, "Nenhum artefato") {
		t.Errorf("esperava estado vazio, got: %q", out)
	}
}

func TestEvidenceShow_Detail(t *testing.T) {
	chdirEvidenceTemp(t)
	t.Setenv("COSCA_ACQUISITION_ALLOW_PRIVATE", "true")
	content := []byte("show-me")
	srv := newEvidenceMockServer(t, "text/markdown", content)
	sum := sha256.Sum256(content)
	sha := fmt.Sprintf("%x", sum[:])

	fetchCmd := NewEvidenceFetchCommand()
	fetchCmd.Flags().Set("allow-remote", "true")
	if _, err := runEvidenceCommand(t, fetchCmd, []string{srv.URL}); err != nil {
		t.Fatal(err)
	}

	showCmd := NewEvidenceShowCommand()
	out, err := runEvidenceCommand(t, showCmd, []string{"a-1"})
	if err != nil {
		t.Fatalf("evidence show: %v", err)
	}
	if !strings.Contains(out, "A-0001") || !strings.Contains(out, sha) {
		t.Errorf("show deveria conter A-0001 e SHA-256: %q", out)
	}
	if !strings.Contains(out, "UNTRUSTED") {
		t.Errorf("show deveria marcar UNTRUSTED: %q", out)
	}
	wantBodyPath := filepath.Join(".cosca", "quarantine", "artifacts", "A-0001")
	if !strings.Contains(out, filepath.FromSlash(wantBodyPath)) {
		t.Errorf("show deveria indicar o caminho do corpo: %q", out)
	}
}

func TestEvidenceShow_Missing(t *testing.T) {
	chdirEvidenceTemp(t)
	cmd := NewEvidenceShowCommand()
	if _, err := runEvidenceCommand(t, cmd, []string{"A-9999"}); err == nil {
		t.Fatal("show de artefato inexistente deveria falhar")
	}
}

// =============================================================================
// `evidence promote` — promoção manual, nunca automática
// =============================================================================

func TestEvidencePromote_RequiresExplicitItem(t *testing.T) {
	chdirEvidenceTemp(t)
	cmd := NewEvidencePromoteCommand()
	_, err := runEvidenceCommand(t, cmd, []string{"A-0001"})
	if err == nil || !strings.Contains(err.Error(), "--item") {
		t.Fatalf("promote sem --item deveria recusar, got %v", err)
	}
}

func TestEvidencePromote_FullFlow(t *testing.T) {
	dir := chdirEvidenceTemp(t)
	t.Setenv("COSCA_ACQUISITION_ALLOW_PRIVATE", "true")

	// Artefato externo (arquivo-fonte → P4) + item de conhecimento temporário.
	content := []byte("func main() { println(\"evidence\") }\n")
	srv := newEvidenceMockServer(t, "text/x-go", content)
	fetchURL := srv.URL + "/evidence.go"
	sum := sha256.Sum256(content)
	sha := fmt.Sprintf("%x", sum[:])

	fetchCmd := NewEvidenceFetchCommand()
	fetchCmd.Flags().Set("allow-remote", "true")
	if _, err := runEvidenceCommand(t, fetchCmd, []string{fetchURL}); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	seedKnowledgeItem(t, dir, "K-01", "Evidência externa reproduzível")

	// Promoção manual com --item.
	promoteCmd := NewEvidencePromoteCommand()
	promoteCmd.Flags().Set("item", "K-01")
	out, err := runEvidenceCommand(t, promoteCmd, []string{"A-0001"})
	if err != nil {
		t.Fatalf("evidence promote: %v\n%s", err, out)
	}
	if !strings.Contains(out, "promovida manualmente") {
		t.Errorf("output deveria confirmar a promoção: %q", out)
	}
	if !strings.Contains(out, "P4") {
		t.Errorf("output deveria citar proveniência P4 (arquivo-fonte): %q", out)
	}
	if !strings.Contains(out, sha) {
		t.Errorf("output deveria citar o SHA-256: %q", out)
	}

	// Proposal → promoted com PromotedTo = item.
	qstore := quarantine.NewStore(dir)
	prop, err := qstore.Get("Q-0001")
	if err != nil {
		t.Fatalf("proposal: %v", err)
	}
	if prop.Status != quarantine.StatusPromoted {
		t.Errorf("proposal deveria estar promoted, got %q", prop.Status)
	}
	if prop.PromotedTo != "K-01" {
		t.Errorf("PromotedTo deveria ser K-01, got %q", prop.PromotedTo)
	}

	// Evidência adicionada ao item (P4, SHA256, repository host+path).
	lawsPath := filepath.Join(dir, ".cosca", "knowledge", "laws.json")
	engine := knowledge.NewPromotionEngine()
	if err := engine.Load(lawsPath); err != nil {
		t.Fatalf("load laws: %v", err)
	}
	item, ok := engine.Get("K-01")
	if !ok {
		t.Fatal("item K-01 não encontrado")
	}
	found := false
	for _, ev := range item.Evidence {
		if ev.ID == "A-0001" {
			found = true
			if ev.Provenance != knowledge.ProvenanceReproducible {
				t.Errorf("proveniência da evidência deveria ser P4, got %q", ev.Provenance)
			}
			if ev.SHA256 != sha {
				t.Errorf("SHA256 da evidência divergente: %q", ev.SHA256)
			}
			if ev.Repository == "" {
				t.Error("repository da evidência vazio")
			}
			if ev.Retrieved.IsZero() {
				t.Error("retrieved da evidência zero")
			}
		}
	}
	if !found {
		t.Errorf("evidência A-0001 não adicionada ao item: %+v", item.Evidence)
	}

	// O corpo NÃO vai para o conhecimento — continua em quarentena.
	art := readEvidenceArtifact(t, "A-0001")
	if art.Provenance != acquisition.ProvenanceUntrusted {
		t.Errorf("artefato deveria permanecer UNTRUSTED mesmo após promoção, got %q", art.Provenance)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cosca", "quarantine", "artifacts", "A-0001")); err != nil {
		t.Errorf("corpo do artefato deveria continuar em quarentena: %v", err)
	}
}

func TestEvidencePromote_MissingItemRefuses(t *testing.T) {
	chdirEvidenceTemp(t)
	cmd := NewEvidencePromoteCommand()
	if _, err := runEvidenceCommand(t, cmd, []string{"A-0001"}); err == nil {
		t.Fatal("promote sem --item deveria falhar")
	}
}

// =============================================================================
// Proveniência da promoção: P2 (repositório) vs P4 (arquivo-fonte)
// =============================================================================

func TestProvenanceForArtifact(t *testing.T) {
	page := &acquisition.AcquiredArtifact{URL: "https://github.com/CoscaAI/cosca"}
	if got := provenanceForArtifact(page); got != knowledge.ProvenanceIdentifiableRepo {
		t.Errorf("página/repositório deveria ser P2, got %s", got)
	}

	src := &acquisition.AcquiredArtifact{URL: "https://raw.githubusercontent.com/CoscaAI/cosca/main/internal/runtime/foo.go"}
	if got := provenanceForArtifact(src); got != knowledge.ProvenanceReproducible {
		t.Errorf("arquivo-fonte deveria ser P4, got %s", got)
	}

	doc := &acquisition.AcquiredArtifact{URL: "https://example.com/docs/README.md"}
	if got := provenanceForArtifact(doc); got != knowledge.ProvenanceReproducible {
		t.Errorf("documento com extensão deveria ser P4, got %s", got)
	}
}

func TestRepositoryFromURL(t *testing.T) {
	got := repositoryFromURL("https://github.com/CoscaAI/cosca/blob/main/README.md")
	if got != "github.com/CoscaAI/cosca/blob/main/README.md" {
		t.Errorf("repositoryFromURL: got %q", got)
	}
}

// =============================================================================
// compile-time guard
// =============================================================================

var (
	_ *cobra.Command = NewEvidenceCommand()
	_ *cobra.Command = NewEvidenceFetchCommand()
	_ *cobra.Command = NewEvidenceListCommand()
	_ *cobra.Command = NewEvidenceShowCommand()
	_ *cobra.Command = NewEvidencePromoteCommand()
	_                = time.Now
)
