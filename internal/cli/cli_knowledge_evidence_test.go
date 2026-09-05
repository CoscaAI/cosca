//
// Tests for `cosca knowledge evidence` (internal/cli/knowledge_evidence.go).
//
// Covers:
//   - Registration of `evidence` (and its subcommands add/list/repro) in the
//     knowledge command
//   - `evidence add` with provenance P4 + commit + sha256 → success and
//     persists the auditable source reference to laws.json
//   - `evidence add` P4 without sha256/commit → clear validation error
//   - `evidence add` invalid provenance → error
//   - `evidence add` unknown item → error
//   - `evidence add` without --provenance → default P0 (desconhecida)
//   - `evidence list <item>` → table with provenance + commit + sha256
//     (abbreviated) + reproducibility
//   - `evidence repro <item>` → shows the fully reproducible evidence
//
// Uses the formatter-injection pattern from cli_knowledge_law_test.go:
// `runLawCommand` + `chdirTemp` + `seedLawsFile`.

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// setEvidenceFlags preenche os flags do `evidence add`. Valores vazios são
// ignorados (flag não informado).
func setEvidenceFlags(t *testing.T, cmd *cobra.Command, item, id, kind, source, desc, provenance string) {
	t.Helper()
	set := func(name, val string) {
		t.Helper()
		if val == "" {
			return
		}
		if err := cmd.Flags().Set(name, val); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	set("item", item)
	set("id", id)
	set("kind", kind)
	set("source", source)
	set("desc", desc)
	set("provenance", provenance)
}

// =============================================================================
// Registration
// =============================================================================

func TestKnowledgeEvidenceCommand_RegisteredInKnowledge(t *testing.T) {
	cmd := NewKnowledgeCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "evidence" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("evidence subcommand not registered in knowledge command")
	}
}

func TestKnowledgeEvidenceCommand_Subcommands(t *testing.T) {
	cmd := NewKnowledgeEvidenceCommand()
	if cmd == nil {
		t.Fatal("NewKnowledgeEvidenceCommand returned nil")
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	expected := []string{"add", "list", "repro"}
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

func TestKnowledgeEvidenceSubcommands_NonNil(t *testing.T) {
	tests := []struct {
		name    string
		factory func() *cobra.Command
		use     string
	}{
		{"add", NewKnowledgeEvidenceAddCommand, "add"},
		{"list", NewKnowledgeEvidenceListCommand, "list <item>"},
		{"repro", NewKnowledgeEvidenceReproCommand, "repro <item>"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.factory()
			if cmd == nil {
				t.Fatalf("factory for %q returned nil", tt.name)
			}
			if !strings.HasPrefix(cmd.Use, tt.use) {
				t.Errorf("expected Use to start with %q, got %q", tt.use, cmd.Use)
			}
			if cmd.Short == "" {
				t.Error("expected non-empty Short description")
			}
			if cmd.RunE == nil {
				t.Error("expected RunE to be set")
			}
		})
	}
}

// =============================================================================
// `evidence add`
// =============================================================================

// TestKnowledgeEvidenceAdd verifies the happy path: P4 + commit + sha256 is
// accepted and the auditable source reference is persisted in laws.json.
func TestKnowledgeEvidenceAdd(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	seedLawsFile(t, dir, &knowledge.KnowledgeItem{
		ID:         "K-01",
		Title:      "Nunca executar como root automaticamente",
		Confidence: 0.95,
	})

	cmd := NewKnowledgeEvidenceAddCommand()
	setEvidenceFlags(t, cmd, "K-01", "E-401", "test", "github.com/CoscaAI/cosca", "teste reproduzível", "P4")
	_ = cmd.Flags().Set("repo", "CoscaAI/cosca")
	_ = cmd.Flags().Set("commit", "7a91c2")
	_ = cmd.Flags().Set("path", "internal/runtime/foo.go")
	_ = cmd.Flags().Set("sha256", "abc123...")

	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{"E-401", "K-01", "P4", "código/teste reproduzível"} {
		if !strings.Contains(output, want) {
			t.Errorf("add output missing %q; output:\n%s", want, output)
		}
	}

	// Persistência: a evidência carregou provenance + source ref completos.
	items := readLawsFile(t, dir)
	if len(items) != 1 || items[0].ID != "K-01" {
		t.Fatalf("unexpected laws: %+v", items)
	}
	if len(items[0].Evidence) != 1 {
		t.Fatalf("expected 1 evidence, got %d", len(items[0].Evidence))
	}
	ev := items[0].Evidence[0]
	if ev.ID != "E-401" {
		t.Errorf("evidence id = %q, want E-401", ev.ID)
	}
	if ev.Provenance != knowledge.ProvenanceReproducible {
		t.Errorf("provenance = %q, want P4", ev.Provenance)
	}
	if ev.Repository != "CoscaAI/cosca" || ev.Commit != "7a91c2" || ev.SHA256 != "abc123..." {
		t.Errorf("source ref not persisted: %+v", ev)
	}
	if ev.Path != "internal/runtime/foo.go" {
		t.Errorf("path = %q, want internal/runtime/foo.go", ev.Path)
	}
	if ev.Retrieved.IsZero() {
		t.Error("retrieved should be set")
	}
}

// TestKnowledgeEvidenceAdd_P4RequiresSHA256 verifies the validation rule: P4
// sem --sha256 (nem --commit) é rejeitado com erro claro.
func TestKnowledgeEvidenceAdd_P4RequiresSHA256(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	for _, missing := range []string{"sha256", "commit"} {
		cmd := NewKnowledgeEvidenceAddCommand()
		setEvidenceFlags(t, cmd, "K-01", "E-401", "test", "src", "desc", "P4")
		_ = cmd.Flags().Set("repo", "CoscaAI/cosca")
		if missing == "sha256" {
			_ = cmd.Flags().Set("commit", "7a91c2")
		} else {
			_ = cmd.Flags().Set("sha256", "abc123...")
		}
		if _, err := runLawCommand(t, cmd, nil); err == nil {
			t.Errorf("expected error for P4 without %s", missing)
		}
	}
}

func TestKnowledgeEvidenceAdd_InvalidProvenance(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeEvidenceAddCommand()
	setEvidenceFlags(t, cmd, "K-01", "E-401", "test", "src", "desc", "P9")
	if _, err := runLawCommand(t, cmd, nil); err == nil {
		t.Fatal("expected error for invalid provenance")
	}
}

func TestKnowledgeEvidenceAdd_ItemNotFound(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeEvidenceAddCommand()
	setEvidenceFlags(t, cmd, "K-999", "E-401", "test", "src", "desc", "P0")
	if _, err := runLawCommand(t, cmd, nil); err == nil {
		t.Fatal("expected error for unknown item")
	}
}

// TestKnowledgeEvidenceAdd_DefaultsToP0 verifies that sem --provenance a
// evidência nasce como P0 (desconhecida).
func TestKnowledgeEvidenceAdd_DefaultsToP0(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	seedLawsFile(t, dir, &knowledge.KnowledgeItem{ID: "K-01", Title: "Lei"})

	cmd := NewKnowledgeEvidenceAddCommand()
	setEvidenceFlags(t, cmd, "K-01", "E-500", "audit", "red-team-A1", "achado de auditoria", "")

	if _, err := runLawCommand(t, cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	items := readLawsFile(t, dir)
	ev := items[0].Evidence[0]
	if ev.Provenance != knowledge.ProvenanceUnknown {
		t.Errorf("default provenance = %q, want P0", ev.Provenance)
	}
}

// =============================================================================
// `evidence list`
// =============================================================================

func TestKnowledgeEvidenceList(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	seedLawsFile(t, dir, &knowledge.KnowledgeItem{
		ID:    "K-01",
		Title: "Lei",
		Evidence: []knowledge.Evidence{
			{
				ID:          "E-401",
				Kind:        "test",
				Source:      "github.com/CoscaAI/cosca",
				Description: "teste reproduzível",
				Provenance:  knowledge.ProvenanceReproducible,
				Repository:  "CoscaAI/cosca",
				Commit:      "7a91c2",
				SHA256:      "abc123def4567890abcdef",
			},
		},
	})

	cmd := NewKnowledgeEvidenceListCommand()
	output, err := runLawCommand(t, cmd, []string{"K-01"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{
		"Evidências de K-01", "ID", "Tipo", "Proveniência", "Commit", "SHA256",
		"Reproduzível", "E-401", "P4", "7a91c2", "abc123def456…", "sim",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("list output missing %q; output:\n%s", want, output)
		}
	}
}

func TestKnowledgeEvidenceList_UnknownItem(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeEvidenceListCommand()
	if _, err := runLawCommand(t, cmd, []string{"K-999"}); err == nil {
		t.Fatal("expected error for unknown item")
	}
}

// =============================================================================
// `evidence repro`
// =============================================================================

func TestKnowledgeEvidenceRepro(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	seedLawsFile(t, dir, &knowledge.KnowledgeItem{
		ID:    "K-01",
		Title: "Lei",
		Evidence: []knowledge.Evidence{
			{
				ID:          "E-1",
				Kind:        "incident",
				Source:      "F001",
				Description: "sem proveniência",
				Provenance:  knowledge.ProvenanceUnknown,
			},
			{
				ID:          "E-2",
				Kind:        "test",
				Source:      "github.com/CoscaAI/cosca",
				Description: "reproduzível",
				Provenance:  knowledge.ProvenanceReproducible,
				Repository:  "CoscaAI/cosca",
				Commit:      "7a91c2",
				SHA256:      "abc123...",
			},
		},
	})

	cmd := NewKnowledgeEvidenceReproCommand()
	output, err := runLawCommand(t, cmd, []string{"K-01"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{
		"Evidências reproduzíveis", "Total de evidências", "Reproduzíveis",
		"1", "E-2", "CoscaAI/cosca@7a91c2",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("repro output missing %q; output:\n%s", want, output)
		}
	}
	if strings.Contains(output, "E-1") && strings.Contains(output, "CoscaAI/cosca@7a91c2") {
		// E-1 (P0) não pode aparecer como reproduzível; verificar que o único
		// bullet reproduzível aponta E-2.
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "  • ") {
				if !strings.Contains(line, "E-2") {
					t.Errorf("reproducible bullet should be E-2, got: %q", line)
				}
			}
		}
	}
}

func TestKnowledgeEvidenceRepro_NoneReproducible(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	seedLawsFile(t, dir, &knowledge.KnowledgeItem{
		ID:    "K-01",
		Title: "Lei",
		Evidence: []knowledge.Evidence{
			{ID: "E-1", Kind: "test", Source: "s", Description: "d", Provenance: knowledge.ProvenanceUnknown},
		},
	})

	cmd := NewKnowledgeEvidenceReproCommand()
	output, err := runLawCommand(t, cmd, []string{"K-01"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !strings.Contains(output, "Reproduzíveis") {
		t.Errorf("expected Reproduzíveis summary; output:\n%s", output)
	}
	if !strings.Contains(output, "Nenhuma evidência é totalmente reproduzível") {
		t.Errorf("expected no-reproducible warning; output:\n%s", output)
	}
}

// compile-time guard: the command constructors must match the cobra contract.
var _ = NewKnowledgeEvidenceCommand()
