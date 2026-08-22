//
// Tests for the `cosca decision` command tree (internal/cli/decision.go).
//
// Covers:
//   - Registration of `decision` in the root command
//   - Command properties (subcomandos list/explain)
//   - `decision list` → tabela (id, quando, input, status); vazio → aviso
//   - `decision explain <id>` → trilha completa (todos os campos, rótulos pt-BR)
//   - `decision explain <id> --json` → JSON válido com os campos da trilha
//   - `approve --dry-run` NÃO grava decisão (dry-run seguro)
//   - `approve` (confirmado + testes passam) grava a trilha de decisão D-0001
//   - recordDecision trata store quebrado com elegância (sem pânico; aprovação segue)
//
// NOTE: os testes fazem chdir() (mutam o cwd do processo) e NÃO rodam em
// paralelo. Formatter injetado via newContextWithFormatter (padrão do CLI).

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/audit"
)

// chdirDecisionTemp muda para um diretório temporário e restaura o cwd no
// cleanup. As trilhas de decisão vivem em <tmp>/.cosca/audit.db.
func chdirDecisionTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Chdir registra o restore do cwd DEPOIS do RemoveAll do TempDir
	// (cleanups rodam em LIFO) — no Windows não dá para remover o
	// diretório que é o CWD do processo.
	t.Chdir(dir)
	return dir
}

// runDecisionCommand executa o RunE de um subcomando com formatter injetado
// em um buffer (padrão de injeção de formatter do CLI).
func runDecisionCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
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

// seedDecisionRecord grava uma decisão diretamente no store do projeto temp.
func seedDecisionRecord(t *testing.T, dir string, d audit.DecisionRecord) string {
	t.Helper()
	store, err := audit.NewDecisionStore(filepath.Join(dir, ".cosca", "audit.db"))
	if err != nil {
		t.Fatalf("NewDecisionStore: %v", err)
	}
	defer func() { _ = store.Close() }()
	id, err := store.Record(d)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	return id
}

// =============================================================================
// Registration — `decision` no root
// =============================================================================

func TestDecisionCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "decision" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("decision subcommand not registered in root command")
	}
}

func TestDecisionCommand_Properties(t *testing.T) {
	cmd := NewDecisionCommand()
	if cmd == nil {
		t.Fatal("NewDecisionCommand returned nil")
	}
	if cmd.Use != "decision" {
		t.Errorf("expected Use='decision', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long description")
	}
	expected := []string{"list", "explain"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing decision subcommand: %s", name)
		}
	}
}

func TestDecision_ArgConstraints(t *testing.T) {
	explain := NewDecisionExplainCommand()
	if err := explain.Args(explain, nil); err == nil {
		t.Error("explain should require an id")
	}
	if err := explain.Args(explain, []string{"D-0001"}); err != nil {
		t.Errorf("explain with one arg should be allowed: %v", err)
	}
	if err := explain.Args(explain, []string{"D-0001", "D-0002"}); err == nil {
		t.Error("explain with two args should fail")
	}

	list := NewDecisionListCommand()
	if err := list.Args(list, []string{"x"}); err == nil {
		t.Error("list should reject arguments")
	}
}

// =============================================================================
// `decision list` — vazio
// =============================================================================

func TestDecisionList_Empty(t *testing.T) {
	chdirDecisionTemp(t)
	out, err := runDecisionCommand(t, NewDecisionListCommand(), nil)
	if err != nil {
		t.Fatalf("decision list (empty): %v", err)
	}
	if !strings.Contains(out, "Nenhuma decisão registrada") {
		t.Errorf("expected empty-state warning, got: %q", out)
	}
}

// =============================================================================
// `decision list` — populado
// =============================================================================

func TestDecisionList_Populated(t *testing.T) {
	dir := chdirDecisionTemp(t)

	seedDecisionRecord(t, dir, audit.DecisionRecord{
		Input:    "Aprovar plano de execução do workflow de deploy",
		Approval: "Don / Gate 0",
		Result:   "passed",
		Status:   "approved",
	})

	out, err := runDecisionCommand(t, NewDecisionListCommand(), nil)
	if err != nil {
		t.Fatalf("decision list: %v", err)
	}
	for _, want := range []string{"D-0001", "Trilha de decisão", "Input", "Status", "approved"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q; output:\n%s", want, out)
		}
	}
}

// =============================================================================
// `decision explain <id>` — trilha completa
// =============================================================================

func TestDecisionExplain_PrintsAllFields(t *testing.T) {
	dir := chdirDecisionTemp(t)

	seedDecisionRecord(t, dir, audit.DecisionRecord{
		Input:         "Aplicar a Lei P9 no workflow de deploy",
		KnowledgeUsed: []string{"K-18", "K-91"},
		LawsApplied:   []string{"L-07", "L-13"},
		Evidence:      []string{"E-182", "E-201"},
		Provider:      "ollama",
		Model:         "llama3.1",
		Approval:      "Don / Gate 0",
		Result:        "passed",
		Rollback:      "git revert bdaef4c",
		Status:        "approved",
	})

	out, err := runDecisionCommand(t, NewDecisionExplainCommand(), []string{"D-0001"})
	if err != nil {
		t.Fatalf("decision explain: %v", err)
	}

	for _, want := range []string{
		"DECISION TRACE D-0001",
		"Decision ID: D-0001",
		"Input: Aplicar a Lei P9 no workflow de deploy",
		"Knowledge usado: K-18 K-91",
		"Leis aplicadas: L-07 L-13",
		"Evidências: E-182 E-201",
		"Provider: ollama",
		"Modelo: llama3.1",
		"Aprovação humana: Don / Gate 0",
		"Resultado: passed",
		"Rollback: git revert bdaef4c",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("explain output missing %q; output:\n%s", want, out)
		}
	}
}

func TestDecisionExplain_NotFound(t *testing.T) {
	chdirDecisionTemp(t)
	_, err := runDecisionCommand(t, NewDecisionExplainCommand(), []string{"D-0001"})
	if err == nil {
		t.Fatal("expected error for non-existent decision")
	}
	if !strings.Contains(err.Error(), "não encontrada") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDecisionExplain_JSON(t *testing.T) {
	dir := chdirDecisionTemp(t)

	seedDecisionRecord(t, dir, audit.DecisionRecord{
		Input:         "Aplicar a Lei P9",
		KnowledgeUsed: []string{"K-18"},
		LawsApplied:   []string{"L-07"},
		Evidence:      []string{"E-182"},
		Provider:      "none",
		Approval:      "Don / Gate 0",
		Result:        "passed",
		Status:        "approved",
	})

	globalFlags = GlobalFlags{JSON: true}
	defer func() { globalFlags = GlobalFlags{} }()

	out, err := runDecisionCommand(t, NewDecisionExplainCommand(), []string{"D-0001"})
	if err != nil {
		t.Fatalf("decision explain --json: %v", err)
	}

	var rec audit.DecisionRecord
	if err := json.Unmarshal([]byte(out), &rec); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if rec.DecisionID != "D-0001" {
		t.Errorf("DecisionID = %q, want D-0001", rec.DecisionID)
	}
	if rec.Approval != "Don / Gate 0" {
		t.Errorf("Approval = %q, want 'Don / Gate 0'", rec.Approval)
	}
	if len(rec.KnowledgeUsed) != 1 || rec.KnowledgeUsed[0] != "K-18" {
		t.Errorf("KnowledgeUsed = %v, want [K-18]", rec.KnowledgeUsed)
	}
	if len(rec.LawsApplied) != 1 || rec.LawsApplied[0] != "L-07" {
		t.Errorf("LawsApplied = %v, want [L-07]", rec.LawsApplied)
	}
}

func TestDecisionList_JSON(t *testing.T) {
	dir := chdirDecisionTemp(t)

	seedDecisionRecord(t, dir, audit.DecisionRecord{Input: "a", Result: "passed", Status: "approved"})
	seedDecisionRecord(t, dir, audit.DecisionRecord{Input: "b", Result: "passed", Status: "approved"})

	globalFlags = GlobalFlags{JSON: true}
	defer func() { globalFlags = GlobalFlags{} }()

	out, err := runDecisionCommand(t, NewDecisionListCommand(), nil)
	if err != nil {
		t.Fatalf("decision list --json: %v", err)
	}

	var recs []audit.DecisionRecord
	if err := json.Unmarshal([]byte(out), &recs); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(recs) != 2 {
		t.Fatalf("len = %d, want 2", len(recs))
	}
	if recs[0].DecisionID != "D-0002" || recs[1].DecisionID != "D-0001" {
		t.Errorf("order = %v, want [D-0002 D-0001]", []string{recs[0].DecisionID, recs[1].DecisionID})
	}
}

// =============================================================================
// Integração: `approve` e a trilha de decisão
// =============================================================================

// TestApproveDryRun_DoesNotWriteDecision verifica que --dry-run não grava
// nada na trilha de decisão (dry-run seguro).
func TestApproveDryRun_DoesNotWriteDecision(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	writeTestFile(t, planPath, approvalTestPlanJSON)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"approve", "--plan", planPath, "--dry-run"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	// Dry-run não deve ter criado sequer a base (nada abre o store).
	if _, err := os.Stat(filepath.Join(tmpDir, ".cosca", "audit.db")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not create the decision/audit db; stat err=%v", err)
	}
}

// TestApproveCommand_RecordsDecision verifica que uma aprovação confirmada
// com testes passando grava a trilha de decisão D-0001 (best-effort, junto do
// markdown canônico).
func TestApproveCommand_RecordsDecision(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	createApprovalTestModule(t, tmpDir)
	planPath := filepath.Join(tmpDir, "plan.json")
	writeTestFile(t, planPath, approvalTestPlanJSON)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader("y\n"))
	root.SetArgs([]string{"approve", "--plan", planPath})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v\noutput:\n%s", err, buf.String())
	}

	if !strings.Contains(buf.String(), "Trilha de decisão: D-0001") {
		t.Errorf("approve output missing decision trace line; output:\n%s", buf.String())
	}

	store, err := audit.NewDecisionStore(filepath.Join(tmpDir, ".cosca", "audit.db"))
	if err != nil {
		t.Fatalf("NewDecisionStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	rec, err := store.Get("D-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil {
		t.Fatal("decision D-0001 not found after approve")
	}
	if rec.Approval != "Don / Gate 0" {
		t.Errorf("Approval = %q, want 'Don / Gate 0'", rec.Approval)
	}
	if rec.Result != "passed" {
		t.Errorf("Result = %q, want passed", rec.Result)
	}
	if rec.Status != "approved" {
		t.Errorf("Status = %q, want approved", rec.Status)
	}
	if !strings.Contains(rec.Input, "Plano de Execução") {
		t.Errorf("Input should contain the plan summary (plan.String()); got: %q", rec.Input)
	}
}

// TestRecordDecision_FailingStoreIsGraceful verifica que recordDecision lida
// com um store quebrado sem pânico e sem interromper a aprovação.
func TestRecordDecision_FailingStoreIsGraceful(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Caminho injeção de falha: o "diretório" é um arquivo → MkdirAll falha.
	blocker := filepath.Join(dir, "blocker")
	writeTestFile(t, blocker, "x")
	badCoscaDir := filepath.Join(blocker, "nope")

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	plan, err := loadApprovalPlan(approvalTestPlanJSON)
	if err != nil {
		t.Fatalf("loadApprovalPlan: %v", err)
	}

	// Não deve panificar nem retornar nada — a aprovação segue.
	recordDecision(cmd, badCoscaDir, plan, "passed")

	if !strings.Contains(buf.String(), "aviso") {
		t.Errorf("expected a best-effort warning, got: %q", buf.String())
	}
}
