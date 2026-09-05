//
// Tests for `cosca knowledge claim` (internal/cli/knowledge_claim.go).
//
// Covers:
//   - Registration of `claim` (and its subcommands add/classify/list/status)
//     in the knowledge command
//   - `claim add` with auto-classification (determinística) → CL-XXXX +
//     razão; persiste em .cosca/claims.db
//   - `claim add` with explicit --kind (INFERENCE) → type + reason
//   - `claim add --assumed` overrides validated evidence → ASSUMPTION
//   - `claim add` invalid --kind → error
//   - `claim add` JSON output
//   - `claim classify` dry-run → shows classification WITHOUT creating the db
//   - `claim list` → table (ID | Tipo | Afirmação | Confiança | Confiável)
//   - `claim list` empty state
//   - `claim status <id>` → detail + IsTrustworthy verdict
//   - `claim status` unknown id → error
//
// Uses the formatter-injection pattern from cli_knowledge_evidence_test.go:
// `runLawCommand` + `chdirTemp`.

package cli

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// setClaimAddFlags preenche os sinais de classificação do `claim add` /
// `claim classify`.
func setClaimAddFlags(t *testing.T, cmd *cobra.Command, statement string, evidence int, validated, derived, tested, assumed bool) {
	t.Helper()
	set := func(name, val string) {
		t.Helper()
		if err := cmd.Flags().Set(name, val); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	set("statement", statement)
	set("evidence", strconv.Itoa(evidence))
	set("validated", strconv.FormatBool(validated))
	set("derived", strconv.FormatBool(derived))
	set("tested", strconv.FormatBool(tested))
	set("assumed", strconv.FormatBool(assumed))
}

// =============================================================================
// Registration
// =============================================================================

func TestKnowledgeClaimCommand_RegisteredInKnowledge(t *testing.T) {
	cmd := NewKnowledgeCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "claim" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("claim subcommand not registered in knowledge command")
	}
}

func TestKnowledgeClaimCommand_Subcommands(t *testing.T) {
	cmd := NewKnowledgeClaimCommand()
	if cmd == nil {
		t.Fatal("NewKnowledgeClaimCommand returned nil")
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	expected := []string{"add", "classify", "list", "status"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing claim subcommand: %s", name)
		}
	}
}

func TestKnowledgeClaimSubcommands_NonNil(t *testing.T) {
	tests := []struct {
		name    string
		factory func() *cobra.Command
		use     string
	}{
		{"add", NewKnowledgeClaimAddCommand, "add"},
		{"classify", NewKnowledgeClaimClassifyCommand, "classify"},
		{"list", NewKnowledgeClaimListCommand, "list"},
		{"status", NewKnowledgeClaimStatusCommand, "status <id>"},
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
// `claim add`
// =============================================================================

func TestKnowledgeClaimAdd_AutoClassifiesFact(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeClaimAddCommand()
	setClaimAddFlags(t, cmd, "A API X rejeita tokens JWT", 2, true, false, false, false)

	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{"CL-0001", "FACT", "evidência validada registrada"} {
		if !strings.Contains(output, want) {
			t.Errorf("add output missing %q; output:\n%s", want, output)
		}
	}

	// Persistência: a claim existe em .cosca/claims.db como FACT.
	store, err := knowledge.NewClaimStore(filepath.Join(dir, ".cosca", "claims.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	rec, err := store.Get("CL-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil {
		t.Fatal("claim CL-0001 not persisted")
	}
	if rec.Kind != knowledge.ClaimFact || rec.Statement != "A API X rejeita tokens JWT" {
		t.Errorf("unexpected persisted claim: %+v", rec)
	}
}

func TestKnowledgeClaimAdd_ExplicitKind(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeClaimAddCommand()
	if err := cmd.Flags().Set("statement", "X deriva de Y"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("kind", "INFERENCE"); err != nil {
		t.Fatal(err)
	}

	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{"CL-0001", "INFERENCE", "derivada de outra afirmação"} {
		if !strings.Contains(output, want) {
			t.Errorf("add output missing %q; output:\n%s", want, output)
		}
	}

	// Os sinais foram ignorados: o tipo explícito vence.
	store, err := knowledge.NewClaimStore(filepath.Join(dir, ".cosca", "claims.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	rec, err := store.Get("CL-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil || rec.Kind != knowledge.ClaimInference {
		t.Errorf("expected INFERENCE persisted, got %+v", rec)
	}
}

func TestKnowledgeClaimAdd_AssumedOverridesEvidence(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeClaimAddCommand()
	setClaimAddFlags(t, cmd, "Presumo que o serviço está UP", 3, true, false, false, true)

	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{"ASSUMPTION", "assumida explicitamente pelo autor"} {
		if !strings.Contains(output, want) {
			t.Errorf("add --assumed output missing %q; output:\n%s", want, output)
		}
	}
}

func TestKnowledgeClaimAdd_InvalidKind(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeClaimAddCommand()
	_ = cmd.Flags().Set("statement", "x")
	_ = cmd.Flags().Set("kind", "GUESS")
	if _, err := runLawCommand(t, cmd, nil); err == nil {
		t.Fatal("expected error for invalid kind")
	}
}

func TestKnowledgeClaimAdd_ConfidenceOutOfRange(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeClaimAddCommand()
	_ = cmd.Flags().Set("statement", "x")
	_ = cmd.Flags().Set("confidence", "1.5")
	if _, err := runLawCommand(t, cmd, nil); err == nil {
		t.Fatal("expected error for out-of-range confidence")
	}
}

func TestKnowledgeClaimAdd_MissingStatement(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeClaimAddCommand()
	if _, err := runLawCommand(t, cmd, nil); err == nil {
		t.Fatal("expected error for missing --statement")
	}
}

func TestKnowledgeClaimAdd_JSON(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeClaimAddCommand()
	cmd.PersistentFlags().BoolP("json", "j", false, "json")
	_ = cmd.PersistentFlags().Set("json", "true")
	_ = cmd.Flags().Set("statement", "O timeout padrão é 30s")
	_ = cmd.Flags().Set("evidence", "2")
	_ = cmd.Flags().Set("validated", "true")

	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{
		`"id": "CL-0001"`,
		`"kind": "FACT"`,
		`"reason"`,
		`"trustworthy": true`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("add --json missing %q; output:\n%s", want, output)
		}
	}
}

// =============================================================================
// `claim classify` — dry-run
// =============================================================================

func TestKnowledgeClaimClassify_DryRun(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeClaimClassifyCommand()
	if err := cmd.Flags().Set("statement", "A latência aumenta com o load"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("evidence", "1"); err != nil {
		t.Fatal(err)
	}

	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{"EVIDENCE", "observação registrada", "dry-run", "nada foi gravado"} {
		if !strings.Contains(output, want) {
			t.Errorf("classify output missing %q; output:\n%s", want, output)
		}
	}

	// Nada foi gravado: a base .cosca/claims.db não deve existir.
	if _, err := os.Stat(filepath.Join(dir, ".cosca", "claims.db")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not create the claims db; stat err = %v", err)
	}
}

func TestKnowledgeClaimClassify_Derived(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeClaimClassifyCommand()
	_ = cmd.Flags().Set("statement", "X deriva de Y")
	_ = cmd.Flags().Set("derived", "true")

	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !strings.Contains(output, "INFERENCE") {
		t.Errorf("derived should classify INFERENCE; output:\n%s", output)
	}
}

// =============================================================================
// `claim list`
// =============================================================================

func TestKnowledgeClaimList(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	store, err := knowledge.NewClaimStore(filepath.Join(dir, ".cosca", "claims.db"))
	if err != nil {
		t.Fatalf("NewClaimStore: %v", err)
	}
	if _, err := store.Add(knowledge.ClaimRecord{
		Kind:       knowledge.ClaimFact,
		Statement:  "O timeout padrão é 30s",
		Confidence: 0.9,
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := store.Add(knowledge.ClaimRecord{
		Kind:       knowledge.ClaimAssumption,
		Statement:  "Presumo que o serviço está UP",
		Confidence: 0,
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	store.Close()

	cmd := NewKnowledgeClaimListCommand()
	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{
		"Afirmações classificadas (2)",
		"ID", "Tipo", "Afirmação", "Confiança", "Confiável",
		"CL-0001", "CL-0002", "FACT", "ASSUMPTION",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("list output missing %q; output:\n%s", want, output)
		}
	}
	// FACT (0.90) → confiável "sim"; ASSUMPTION → "não".
	if !strings.Contains(output, "sim") || !strings.Contains(output, "não") {
		t.Errorf("expected both trustworthy verdicts in table; output:\n%s", output)
	}
}

func TestKnowledgeClaimList_Empty(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeClaimListCommand()
	output, err := runLawCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !strings.Contains(output, "Nenhuma afirmação registrada") {
		t.Errorf("expected empty-state warning; output:\n%s", output)
	}
}

// =============================================================================
// `claim status`
// =============================================================================

func TestKnowledgeClaimStatus(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	store, err := knowledge.NewClaimStore(filepath.Join(dir, ".cosca", "claims.db"))
	if err != nil {
		t.Fatalf("NewClaimStore: %v", err)
	}
	if _, err := store.Add(knowledge.ClaimRecord{
		Kind:       knowledge.ClaimEvidence,
		Statement:  "A latência subiu após o deploy",
		Confidence: 0.8,
		Supports:   []string{"E-401"},
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	store.Close()

	cmd := NewKnowledgeClaimStatusCommand()
	output, err := runLawCommand(t, cmd, []string{"CL-0001"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	for _, want := range []string{
		"Afirmação CL-0001", "EVIDENCE", "observação registrada",
		"A latência subiu após o deploy", "E-401",
		"0.80", "Confiável", "sim",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("status output missing %q; output:\n%s", want, output)
		}
	}
}

func TestKnowledgeClaimStatus_NormalizedID(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	store, err := knowledge.NewClaimStore(filepath.Join(dir, ".cosca", "claims.db"))
	if err != nil {
		t.Fatalf("NewClaimStore: %v", err)
	}
	if _, err := store.Add(knowledge.ClaimRecord{Kind: knowledge.ClaimFact, Statement: "Fato"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	store.Close()

	cmd := NewKnowledgeClaimStatusCommand()
	output, err := runLawCommand(t, cmd, []string{"cl-0001"})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !strings.Contains(output, "Afirmação CL-0001") {
		t.Errorf("normalized id should resolve to CL-0001; output:\n%s", output)
	}
}

func TestKnowledgeClaimStatus_NotFound(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeClaimStatusCommand()
	if _, err := runLawCommand(t, cmd, []string{"CL-9999"}); err == nil {
		t.Fatal("expected error for unknown claim")
	}
}

func TestKnowledgeClaimStatus_InvalidID(t *testing.T) {
	chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewKnowledgeClaimStatusCommand()
	if _, err := runLawCommand(t, cmd, []string{"abc"}); err == nil {
		t.Fatal("expected error for invalid claim id")
	}
}

// =============================================================================
// Helpers exercised via a full command tree (knowledge claim through root)
// =============================================================================

func TestKnowledgeClaim_ThroughRootCommand(t *testing.T) {
	dir := chdirTemp(t)
	globalFlags = GlobalFlags{}

	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	cmd.SetContext(newContextWithFormatter(context.Background(), f))
	cmd.SetArgs([]string{"knowledge", "claim", "classify", "--statement", "Fato", "--evidence", "2", "--validated"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("root execute: %v", err)
	}
	if !strings.Contains(buf.String(), "FACT") {
		t.Errorf("expected FACT through root command; output:\n%s", buf.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".cosca", "claims.db")); !os.IsNotExist(err) {
		t.Errorf("classify dry-run must not create the db; stat err = %v", err)
	}
}

// compile-time guard: the command constructors must match the cobra contract.
var (
	_ *cobra.Command = NewKnowledgeClaimCommand()
	_ *cobra.Command = NewKnowledgeClaimAddCommand()
	_ *cobra.Command = NewKnowledgeClaimClassifyCommand()
	_ *cobra.Command = NewKnowledgeClaimListCommand()
	_ *cobra.Command = NewKnowledgeClaimStatusCommand()
)
