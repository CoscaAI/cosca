//
// Tests for the `cosca gate` command tree (internal/cli/gate.go).
//
// Covers:
//   - Registration of `gate` in the root command
//   - Command properties + arg constraints (move/ledger exigem id; move exige
//     --to e --as; new exige --plan)
//   - `gate new --plan <ref>` → cria G-0001 no estado plan
//   - `gate status` / `gate status G-0001` → estado atual + última transição
//   - `gate move` → ciclo completo com guarda de papel; negação (papel errado)
//     imprime erro claro e não altera o estado
//   - `gate ledger` → histórico imutável em ordem
//   - `approve --gate` → integração opt-in: --dry-run NÃO toca o gate; uma
//     aprovação real com --gate registra o gate (G-XXXX) em approved
//
// NOTE: os testes fazem chdir() e NÃO rodam em paralelo. Formatter injetado
// via newContextWithFormatter (padrão do CLI).

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

	"github.com/CoscaAI/cosca/internal/gate"
)

// chdirGateTemp muda para um diretório temporário e restaura o cwd no
// cleanup. Os comandos do gate operam em <tmp>/.cosca/gate.db.
func chdirGateTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Chdir registra o restore do cwd DEPOIS do RemoveAll do TempDir
	// (cleanups rodam em LIFO) — no Windows não dá para remover o
	// diretório que é o CWD do processo.
	t.Chdir(dir)
	globalFlags = GlobalFlags{}
	return dir
}

// runGateCommand executa o RunE de um subcomando com formatter injetado em um
// buffer (padrão de injeção de formatter do CLI).
func runGateCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
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

// gateDBPath retorna o caminho da base do gate do projeto.
func gateDBPath(dir string) string {
	return filepath.Join(dir, ".cosca", "gate.db")
}

// =============================================================================
// Registration — `gate` no root
// =============================================================================

func TestGateCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "gate" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("gate subcommand not registered in root command")
	}
}

func TestGateCommand_Properties(t *testing.T) {
	cmd := NewGateCommand()
	if cmd == nil {
		t.Fatal("NewGateCommand returned nil")
	}
	if cmd.Use != "gate" {
		t.Errorf("expected Use='gate', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long description")
	}
	expected := []string{"new", "list", "status", "move", "ledger", "catalog", "recall"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing gate subcommand: %s", name)
		}
	}
}

func TestGateCommand_ArgConstraints(t *testing.T) {
	move := NewGateMoveCommand()
	if err := move.Args(move, nil); err == nil {
		t.Error("move should require an id")
	}
	if err := move.Args(move, []string{"G-0001"}); err != nil {
		t.Errorf("move with one arg should be allowed: %v", err)
	}

	ledger := NewGateLedgerCommand()
	if err := ledger.Args(ledger, nil); err == nil {
		t.Error("ledger should require an id")
	}
	if err := ledger.Args(ledger, []string{"G-0001", "G-0002"}); err == nil {
		t.Error("ledger with two args should fail")
	}

	status := NewGateStatusCommand()
	if err := status.Args(status, []string{"G-0001", "G-0002"}); err == nil {
		t.Error("status with two args should fail")
	}

	// move exige --to e --as; new exige --plan.
	for _, f := range []string{"to", "as"} {
		ann := move.Flags().Lookup(f).Annotations[cobra.BashCompOneRequiredFlag]
		if len(ann) != 1 || ann[0] != "true" {
			t.Errorf("move must mark --%s as required", f)
		}
	}
	ann := NewGateNewCommand().Flags().Lookup("plan").Annotations[cobra.BashCompOneRequiredFlag]
	if len(ann) != 1 || ann[0] != "true" {
		t.Error("new must mark --plan as required")
	}
}

// =============================================================================
// Ciclo completo: new → status → move → ledger
// =============================================================================

func TestGateFullCycle(t *testing.T) {
	dir := chdirGateTemp(t)

	// new
	create := NewGateNewCommand()
	_ = create.ParseFlags([]string{"--plan", "plano-deploy-v2"})
	out, err := runGateCommand(t, create, nil)
	if err != nil {
		t.Fatalf("gate new: %v", err)
	}
	if !strings.Contains(out, "G-0001") || !strings.Contains(out, "plan") {
		t.Errorf("gate new output missing G-0001/plan: %q", out)
	}
	if _, err := os.Stat(gateDBPath(dir)); err != nil {
		t.Fatalf("gate.db not created: %v", err)
	}

	// status (sem id → lista)
	out, err = runGateCommand(t, NewGateStatusCommand(), nil)
	if err != nil {
		t.Fatalf("gate status: %v", err)
	}
	if !strings.Contains(out, "G-0001") || !strings.Contains(out, "plan") {
		t.Errorf("status list missing G-0001/plan: %q", out)
	}

	// status G-0001 → estado atual + última transição ("nenhuma ainda")
	out, err = runGateCommand(t, NewGateStatusCommand(), []string{"G-0001"})
	if err != nil {
		t.Fatalf("gate status G-0001: %v", err)
	}
	if !strings.Contains(out, "G-0001") || !strings.Contains(out, "nenhuma ainda") {
		t.Errorf("status G-0001 output: %q", out)
	}

	// move → approving (editor pode)
	move := NewGateMoveCommand()
	_ = move.ParseFlags([]string{"--to", "approving", "--as", "editor"})
	out, err = runGateCommand(t, move, []string{"G-0001"})
	if err != nil {
		t.Fatalf("gate move → approving: %v", err)
	}
	if !strings.Contains(out, "G-0001 movido plan → approving") {
		t.Errorf("move output: %q", out)
	}

	// move → approved (SOMENTE don/admin)
	move = NewGateMoveCommand()
	_ = move.ParseFlags([]string{"--to", "approved", "--as", "don"})
	out, err = runGateCommand(t, move, []string{"G-0001"})
	if err != nil {
		t.Fatalf("gate move → approved: %v", err)
	}
	if !strings.Contains(out, "G-0001 movido approving → approved") {
		t.Errorf("move output: %q", out)
	}

	// ledger → 2 transições em ordem
	out, err = runGateCommand(t, NewGateLedgerCommand(), []string{"G-0001"})
	if err != nil {
		t.Fatalf("gate ledger: %v", err)
	}
	for _, want := range []string{"plan", "approving", "approved", "editor", "don", "2 transição"} {
		if !strings.Contains(out, want) {
			t.Errorf("ledger output missing %q: %q", want, out)
		}
	}

	// status agora mostra a última transição
	out, err = runGateCommand(t, NewGateStatusCommand(), []string{"G-0001"})
	if err != nil {
		t.Fatalf("gate status #2: %v", err)
	}
	if !strings.Contains(out, "approving → approved (por don)") {
		t.Errorf("status should show last transition: %q", out)
	}
}

// =============================================================================
// Guarda de papel — negação via CLI
// =============================================================================

func TestGateMove_DeniedByRoleGuard(t *testing.T) {
	chdirGateTemp(t)

	create := NewGateNewCommand()
	_ = create.ParseFlags([]string{"--plan", "plano"})
	if _, err := runGateCommand(t, create, nil); err != nil {
		t.Fatalf("gate new: %v", err)
	}
	// Editor encaminha para approving.
	move := NewGateMoveCommand()
	_ = move.ParseFlags([]string{"--to", "approving", "--as", "editor"})
	if _, err := runGateCommand(t, move, []string{"G-0001"}); err != nil {
		t.Fatalf("plan→approving by editor: %v", err)
	}

	// Editor NÃO pode aprovar → erro claro e estado inalterado.
	move = NewGateMoveCommand()
	_ = move.ParseFlags([]string{"--to", "approved", "--as", "editor"})
	out, err := runGateCommand(t, move, []string{"G-0001"})
	if err == nil {
		t.Fatalf("editor approving should be denied; output: %q", out)
	}
	if !strings.Contains(err.Error(), "papel 'editor' não pode mover approving→approved") {
		t.Errorf("denial error = %q", err.Error())
	}

	// Estado permanece approving, sem transição extra no ledger.
	rec := readGateRecord(t)
	if rec.State != gate.StateApproving {
		t.Errorf("state after denial = %q, want approving", rec.State)
	}
	if len(rec.Transitions) != 1 {
		t.Errorf("transitions after denial = %d, want 1", len(rec.Transitions))
	}

	// Specialist (executor) NÃO pode executar um plano aprovado.
	move = NewGateMoveCommand()
	_ = move.ParseFlags([]string{"--to", "approved", "--as", "don"})
	if _, err := runGateCommand(t, move, []string{"G-0001"}); err != nil {
		t.Fatalf("approving→approved by don: %v", err)
	}
	move = NewGateMoveCommand()
	_ = move.ParseFlags([]string{"--to", "executed", "--as", "specialist"})
	out, err = runGateCommand(t, move, []string{"G-0001"})
	if err == nil {
		t.Fatalf("specialist executing should be denied; output: %q", out)
	}
	if !strings.Contains(err.Error(), "papel 'specialist' não pode mover approved→executed") {
		t.Errorf("denial error = %q", err.Error())
	}
}

// readGateRecord lê G-0001 da base .cosca/gate.db do cwd atual.
func readGateRecord(t *testing.T) *gate.GateRecord {
	t.Helper()
	store, err := gate.NewGateStore(gateDBPath(mustGetwd(t)))
	if err != nil {
		t.Fatalf("NewGateStore: %v", err)
	}
	defer func() { _ = store.Close() }()
	rec, err := store.Get("G-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil {
		t.Fatal("G-0001 not found")
	}
	return rec
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return wd
}

// =============================================================================
// JSON output
// =============================================================================

func TestGateNew_JSON(t *testing.T) {
	chdirGateTemp(t)
	globalFlags = GlobalFlags{JSON: true}

	create := NewGateNewCommand()
	_ = create.ParseFlags([]string{"--plan", "plano-json"})
	out, err := runGateCommand(t, create, nil)
	if err != nil {
		t.Fatalf("gate new: %v", err)
	}
	var rec gate.GateRecord
	if err := json.Unmarshal([]byte(out), &rec); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if rec.ID != "G-0001" || rec.State != gate.StatePlan || rec.PlanRef != "plano-json" {
		t.Errorf("unexpected json gate record: %+v", rec)
	}
}

// =============================================================================
// approve --gate — integração opt-in
// =============================================================================

// TestApproveWithGate_RecordsGateRecord verifica que uma aprovação confirmada
// com a flag --gate cria o gate do plano (G-0001) e o move até "approved",
// sem quebrar o fluxo existente (markdown + decisão).
func TestApproveWithGate_RecordsGateRecord(t *testing.T) {
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
	root.SetArgs([]string{"approve", "--plan", planPath, "--gate"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v\noutput:\n%s", err, buf.String())
	}

	// Fluxo existente intacto.
	for _, want := range []string{"Plano de Execução", "Aprovação registrada em:", "Trilha de decisão: D-0001"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("approve --gate output missing %q; output:\n%s", want, buf.String())
		}
	}

	// Gate registrado e aprovado.
	if !strings.Contains(buf.String(), "Gate G-0001: plano aprovado") {
		t.Errorf("approve --gate output missing gate confirmation; output:\n%s", buf.String())
	}

	store, err := gate.NewGateStore(filepath.Join(tmpDir, ".cosca", "gate.db"))
	if err != nil {
		t.Fatalf("NewGateStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	rec, err := store.Get("G-0001")
	if err != nil {
		t.Fatalf("Get G-0001: %v", err)
	}
	if rec == nil {
		t.Fatal("gate G-0001 not recorded after approve --gate")
	}
	if rec.State != gate.StateApproved {
		t.Errorf("gate state = %q, want approved", rec.State)
	}
	// plan → approving → approved.
	if len(rec.Transitions) != 2 {
		t.Fatalf("transitions = %d, want 2 (plan→approving→approved)", len(rec.Transitions))
	}
	if rec.Transitions[0].To != gate.StateApproving || rec.Transitions[1].To != gate.StateApproved {
		t.Errorf("unexpected transition sequence: %+v", rec.Transitions)
	}
}

// TestApproveWithGate_DryRunDoesNotTouchGate verifica que --dry-run, mesmo com
// --gate, NÃO cria a base do gate nem executa nada.
func TestApproveWithGate_DryRunDoesNotTouchGate(t *testing.T) {
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
	root.SetArgs([]string{"approve", "--plan", planPath, "--dry-run", "--gate"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, ".cosca", "gate.db")); !os.IsNotExist(err) {
		t.Errorf("--dry-run must not create gate.db; stat err=%v", err)
	}
}

// TestApproveWithoutGate_NoGateInteraction verifica a regra do Don: sem a
// flag --gate, o approve NÃO interage com o gate (zero regressão).
func TestApproveWithoutGate_NoGateInteraction(t *testing.T) {
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
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, ".cosca", "gate.db")); !os.IsNotExist(err) {
		t.Errorf("approve without --gate must not create gate.db; stat err=%v", err)
	}
}

// compile-time guard: os argumentos dos comandos são construídos no padrão cobra.
var (
	_ *cobra.Command = NewGateCommand()
	_ *cobra.Command = NewGateNewCommand()
	_ *cobra.Command = NewGateListCommand()
	_ *cobra.Command = NewGateStatusCommand()
	_ *cobra.Command = NewGateMoveCommand()
	_ *cobra.Command = NewGateLedgerCommand()
)
