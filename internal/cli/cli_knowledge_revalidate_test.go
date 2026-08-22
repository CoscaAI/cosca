//
// Tests for `cosca knowledge revalidate` (internal/cli/knowledge.go).
//
// Covers:
//   - Registration of `revalidate` in the knowledge command
//   - Command properties (Use/Short/Args) and the `--dry-run` flag
//   - `revalidate` on an unchanged file → OK, status unchanged, persisted
//     LastVerified refreshed
//   - `revalidate` after the source file changes → STALE persisted + the
//     pt-BR message "Esse conhecimento precisa ser revalidado."
//   - `revalidate --dry-run` → shows what would change but writes nothing
//   - `revalidate <id>` unknown id → error
//   - Evidence without hash/path → SKIP row (no error)
//
// Uses the formatter-injection pattern from cli_knowledge_law_test.go:
// `runLawCommand` + `chdirTemp` + `seedLawsFile`.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// revalidateItemWithFile cria um item com uma evidência cujo SHA256 é o hash
// real do arquivo de origem em <dir>/<rel>.
func revalidateItemWithFile(t *testing.T, dir, rel, content string) *knowledge.KnowledgeItem {
	t.Helper()
	writeCLIFile(t, dir, rel, content)
	hash, err := knowledge.HashFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	return &knowledge.KnowledgeItem{
		ID:         "K-442",
		Title:      "API X funciona dessa maneira",
		Confidence: 0.90,
		Evidence: []knowledge.Evidence{
			{
				ID:          "E-1",
				Kind:        "test",
				Source:      "github.com/CoscaAI/cosca",
				Description: "teste reproduzível",
				Provenance:  knowledge.ProvenanceReproducible,
				Repository:  "CoscaAI/cosca",
				Commit:      "7a91c2",
				Path:        rel,
				SHA256:      hash,
			},
		},
	}
}

// =============================================================================
// Registration
// =============================================================================

func TestKnowledgeRevalidateCommand_RegisteredInKnowledge(t *testing.T) {
	cmd := NewKnowledgeCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "revalidate" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("revalidate subcommand not registered in knowledge command")
	}
}

func TestKnowledgeRevalidateCommand_Properties(t *testing.T) {
	cmd := NewKnowledgeRevalidateCommand()
	if cmd == nil {
		t.Fatal("NewKnowledgeRevalidateCommand returned nil")
	}
	if cmd.Use != "revalidate [id]" {
		t.Errorf("expected Use='revalidate [id]', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	if cmd.Long == "" {
		t.Error("expected non-empty Long description")
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
	if cmd.Args == nil {
		t.Fatal("expected Args validator")
	}
	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("revalidate with no args should be allowed: %v", err)
	}
	if err := cmd.Args(cmd, []string{"K-442"}); err != nil {
		t.Errorf("revalidate with one arg should be allowed: %v", err)
	}
	if err := cmd.Args(cmd, []string{"K-442", "K-443"}); err == nil {
		t.Error("revalidate with two args should fail")
	}
	if cmd.Flags().Lookup("dry-run") == nil {
		t.Error("expected --dry-run flag")
	}
}

// =============================================================================
// `revalidate` — unchanged → OK
// =============================================================================

func TestKnowledgeRevalidate_UnchangedOK(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	item := revalidateItemWithFile(t, dir, "internal/runtime/foo.go", "v1")
	seedLawsFile(t, dir, item)

	cmd := NewKnowledgeRevalidateCommand()
	output, err := runLawCommand(t, cmd, []string{"K-442"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{
		"Revalidação de conhecimento",
		"Item", "Evidência", "Caminho", "SHA256 registrado", "SHA256 atual", "Status",
		"K-442", "E-1", "internal/runtime/foo.go",
		"OK",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q; output:\n%s", want, output)
		}
	}
	if strings.Contains(output, "STALE") {
		t.Errorf("unchanged file should not be STALE; output:\n%s", output)
	}

	// Persistido: status inalterado (não-STALE), sem evidência apagada.
	items := readLawsFile(t, dir)
	if len(items) != 1 || items[0].ID != "K-442" {
		t.Fatalf("unexpected laws: %+v", items)
	}
	if items[0].Status == knowledge.StatusStale {
		t.Error("status must not be STALE after an unchanged revalidation")
	}
	if len(items[0].Evidence) != 1 {
		t.Fatalf("evidence must never be deleted: len = %d", len(items[0].Evidence))
	}
}

// =============================================================================
// `revalidate` — modified source → STALE
// =============================================================================

func TestKnowledgeRevalidate_MarksStale(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	item := revalidateItemWithFile(t, dir, "internal/runtime/foo.go", "v1")
	seedLawsFile(t, dir, item)

	// Meses depois → nova release → arquivo alterado.
	abs := filepath.Join(dir, "internal", "runtime", "foo.go")
	if err := os.WriteFile(abs, []byte("v2 — a API mudou"), 0o644); err != nil {
		t.Fatalf("modify source file: %v", err)
	}

	cmd := NewKnowledgeRevalidateCommand()
	output, err := runLawCommand(t, cmd, []string{"K-442"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{
		"K-442", "E-1", "internal/runtime/foo.go",
		"STALE",
		"Esse conhecimento precisa ser revalidado.",
		"Revalidação gravada",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q; output:\n%s", want, output)
		}
	}

	// Persistido: status STALE, nada apagado, hash registrado intacto.
	items := readLawsFile(t, dir)
	if items[0].Status != knowledge.StatusStale {
		t.Errorf("status = %q, want STALE", items[0].Status)
	}
	if items[0].LastVerified.IsZero() {
		t.Error("LastVerified should be set")
	}
	ev := items[0].Evidence[0]
	if ev.SHA256 != item.Evidence[0].SHA256 {
		t.Errorf("recorded hash must stay the original; got %q", ev.SHA256)
	}
}

// =============================================================================
// `revalidate --dry-run` — nothing written
// =============================================================================

func TestKnowledgeRevalidate_DryRunWritesNothing(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	item := revalidateItemWithFile(t, dir, "internal/runtime/foo.go", "v1")
	seedLawsFile(t, dir, item)

	// Fonte muda, mas o dry-run só mostra o que mudaria.
	abs := filepath.Join(dir, "internal", "runtime", "foo.go")
	if err := os.WriteFile(abs, []byte("v2"), 0o644); err != nil {
		t.Fatalf("modify source file: %v", err)
	}

	cmd := NewKnowledgeRevalidateCommand()
	_ = cmd.Flags().Set("dry-run", "true")
	output, err := runLawCommand(t, cmd, []string{"K-442"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{
		"DRY-RUN", "STALE", "Esse conhecimento precisa ser revalidado.",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q; output:\n%s", want, output)
		}
	}
	if strings.Contains(output, "Revalidação gravada") {
		t.Errorf("dry-run must not claim a write; output:\n%s", output)
	}

	// Nada foi gravado: status continua não-STALE no disco.
	items := readLawsFile(t, dir)
	if items[0].Status == knowledge.StatusStale {
		t.Error("dry-run must not persist STALE")
	}
}

// =============================================================================
// `revalidate <id>` — unknown id, skipped evidence
// =============================================================================

func TestKnowledgeRevalidate_UnknownItem(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeRevalidateCommand()
	if _, err := runLawCommand(t, cmd, []string{"K-999"}); err == nil {
		t.Fatal("expected error for unknown item")
	}
}

func TestKnowledgeRevalidate_SkipsEvidenceWithoutHash(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	seedLawsFile(t, dir, &knowledge.KnowledgeItem{
		ID:         "K-01",
		Title:      "Nunca executar como root automaticamente",
		Confidence: 0.95,
		Evidence: []knowledge.Evidence{
			{ID: "F001", Kind: "incident", Source: "F001", Description: "Fuga da jail"},
		},
	})

	cmd := NewKnowledgeRevalidateCommand()
	output, err := runLawCommand(t, cmd, []string{"K-01"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !strings.Contains(output, "SKIP") {
		t.Errorf("evidence without hash should be SKIP; output:\n%s", output)
	}
	if strings.Contains(output, "STALE") {
		t.Errorf("skipped evidence must not be STALE; output:\n%s", output)
	}
	if strings.Contains(output, "Esse conhecimento precisa ser revalidado.") {
		t.Errorf("skipped evidence must not trigger the stale message; output:\n%s", output)
	}

	// Nada mudou (só pulou) → não gravou refresh (LastVerified zero).
	items := readLawsFile(t, dir)
	if !items[0].LastVerified.IsZero() {
		t.Error("all-skipped revalidation must not refresh LastVerified")
	}
}

// =============================================================================
// `revalidate` no args — all items
// =============================================================================

func TestKnowledgeRevalidate_NoArgsRevalidatesAll(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	seedLawsFile(t, dir,
		revalidateItemWithFile(t, dir, "internal/runtime/foo.go", "v1"),
		&knowledge.KnowledgeItem{
			ID:         "K-02",
			Title:      "Registro público desabilitado por padrão",
			Confidence: 0.90,
			Evidence: []knowledge.Evidence{
				{ID: "red-team-A2", Kind: "audit", Source: "red-team-A2", Description: "achado"},
			},
		},
	)

	cmd := NewKnowledgeRevalidateCommand()
	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{"K-442", "K-02", "OK", "SKIP"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q; output:\n%s", want, output)
		}
	}
}

// compile-time guard: the command constructor must match the cobra contract.
var _ = NewKnowledgeRevalidateCommand()
