//
// Tests for the `cosca conflict` command tree (internal/cli/conflict.go).
//
// Covers:
//   - Registration of `conflict` in the root command
//   - Command properties + arg constraints (show/resolve exigem id; new exige
//     --item, --claim-a e --claim-b)
//   - `conflict new --item K-27 --claim-a "K-27:E-101" --claim-b "K-27:E-203"`
//     → cria CONFLICT-001 em aberto
//   - `conflict list` → tabela com o conflito aberto; `list --resolved` →
//     apenas resolvidos
//   - `conflict show CONFLICT-001` → detalhe completo
//   - `conflict resolve CONFLICT-001` → status resolved + ResolvedAt
//   - JSON output
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

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// chdirConflictTemp muda para um diretório temporário e restaura o cwd no
// cleanup. Os comandos do conflict operam em <tmp>/.cosca/conflict.db.
func chdirConflictTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Chdir registra o restore do cwd DEPOIS do RemoveAll do TempDir
	// (cleanups rodam em LIFO) — no Windows não dá para remover o
	// diretório que é o CWD do processo.
	t.Chdir(dir)
	globalFlags = GlobalFlags{}
	return dir
}

// runConflictCommand executa o RunE de um subcomando com formatter injetado em
// um buffer (padrão de injeção de formatter do CLI).
func runConflictCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
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

// conflictDBPath retorna o caminho da base de conflitos do projeto.
func conflictDBPath(dir string) string {
	return filepath.Join(dir, ".cosca", "conflict.db")
}

// readConflictRecord lê CONFLICT-001 da base .cosca/conflict.db do cwd atual.
func readConflictRecord(t *testing.T) *knowledge.ConflictRecord {
	t.Helper()
	store, err := knowledge.NewConflictStore(conflictDBPath(mustGetwd(t)))
	if err != nil {
		t.Fatalf("NewConflictStore: %v", err)
	}
	defer func() { _ = store.Close() }()
	rec, err := store.Get("CONFLICT-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil {
		t.Fatal("CONFLICT-001 not found")
	}
	return rec
}

// =============================================================================
// Registration — `conflict` no root
// =============================================================================

func TestConflictCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "conflict" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("conflict subcommand not registered in root command")
	}
}

func TestConflictCommand_Properties(t *testing.T) {
	cmd := NewConflictCommand()
	if cmd == nil {
		t.Fatal("NewConflictCommand returned nil")
	}
	if cmd.Use != "conflict" {
		t.Errorf("expected Use='conflict', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long description")
	}
	expected := []string{"new", "list", "show", "resolve"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing conflict subcommand: %s", name)
		}
	}
}

func TestConflictCommand_ArgConstraints(t *testing.T) {
	show := NewConflictShowCommand()
	if err := show.Args(show, nil); err == nil {
		t.Error("show should require an id")
	}
	if err := show.Args(show, []string{"CONFLICT-001"}); err != nil {
		t.Errorf("show with one arg should be allowed: %v", err)
	}
	if err := show.Args(show, []string{"CONFLICT-001", "CONFLICT-002"}); err == nil {
		t.Error("show with two args should fail")
	}

	resolve := NewConflictResolveCommand()
	if err := resolve.Args(resolve, nil); err == nil {
		t.Error("resolve should require an id")
	}
	if err := resolve.Args(resolve, []string{"CONFLICT-001", "CONFLICT-002"}); err == nil {
		t.Error("resolve with two args should fail")
	}

	// new exige --item, --claim-a e --claim-b.
	for _, f := range []string{"item", "claim-a", "claim-b"} {
		ann := NewConflictNewCommand().Flags().Lookup(f).Annotations[cobra.BashCompOneRequiredFlag]
		if len(ann) != 1 || ann[0] != "true" {
			t.Errorf("new must mark --%s as required", f)
		}
	}
}

// =============================================================================
// Ciclo completo: new → list → show → resolve → list
// =============================================================================

func TestConflictFullCycle(t *testing.T) {
	dir := chdirConflictTemp(t)

	// new
	create := NewConflictNewCommand()
	_ = create.ParseFlags([]string{
		"--item", "K-01",
		"--claim-a", "K-01:F001",
		"--claim-b", "K-01:red-team-A1",
		"--desc", "fonte A diz root é seguro, fonte B diz o contrário",
	})
	out, err := runConflictCommand(t, create, nil)
	if err != nil {
		t.Fatalf("conflict new: %v", err)
	}
	if !strings.Contains(out, "CONFLICT-001") || !strings.Contains(out, "K-01") {
		t.Errorf("conflict new output missing CONFLICT-001/K-01: %q", out)
	}
	if _, err := os.Stat(conflictDBPath(dir)); err != nil {
		t.Fatalf("conflict.db not created: %v", err)
	}

	// list (padrão: abertos)
	out, err = runConflictCommand(t, NewConflictListCommand(), nil)
	if err != nil {
		t.Fatalf("conflict list: %v", err)
	}
	if !strings.Contains(out, "CONFLICT-001") || !strings.Contains(out, "K-01") {
		t.Errorf("conflict list missing CONFLICT-001/K-01: %q", out)
	}
	if !strings.Contains(out, "open") {
		t.Errorf("conflict list should show open status: %q", out)
	}

	// show CONFLICT-001 → detalhe completo
	out, err = runConflictCommand(t, NewConflictShowCommand(), []string{"CONFLICT-001"})
	if err != nil {
		t.Fatalf("conflict show: %v", err)
	}
	for _, want := range []string{"CONFLICT-001", "K-01", "K-01:F001", "K-01:red-team-A1", "fonte A diz root é seguro", "open"} {
		if !strings.Contains(out, want) {
			t.Errorf("conflict show output missing %q: %q", want, out)
		}
	}

	// resolve CONFLICT-001
	resolve := NewConflictResolveCommand()
	out, err = runConflictCommand(t, resolve, []string{"CONFLICT-001"})
	if err != nil {
		t.Fatalf("conflict resolve: %v", err)
	}
	if !strings.Contains(out, "CONFLICT-001 resolvido") {
		t.Errorf("conflict resolve output: %q", out)
	}

	rec := readConflictRecord(t)
	if rec.Status != knowledge.ConflictResolved {
		t.Errorf("record status = %q, want resolved", rec.Status)
	}
	if rec.ResolvedAt.IsZero() {
		t.Error("ResolvedAt should be set after resolve")
	}

	// list novamente → o conflito saiu da listagem padrão (abertos)
	out, err = runConflictCommand(t, NewConflictListCommand(), nil)
	if err != nil {
		t.Fatalf("conflict list #2: %v", err)
	}
	if strings.Contains(out, "CONFLICT-001") {
		t.Errorf("resolved conflict should not appear in default list: %q", out)
	}

	// list --resolved → agora aparece
	listResolved := NewConflictListCommand()
	_ = listResolved.ParseFlags([]string{"--resolved"})
	out, err = runConflictCommand(t, listResolved, nil)
	if err != nil {
		t.Fatalf("conflict list --resolved: %v", err)
	}
	if !strings.Contains(out, "CONFLICT-001") || !strings.Contains(out, "resolved") {
		t.Errorf("conflict list --resolved should show CONFLICT-001/resolved: %q", out)
	}
}

func TestConflictResolve_AlreadyResolvedNoOp(t *testing.T) {
	chdirConflictTemp(t)

	create := NewConflictNewCommand()
	_ = create.ParseFlags([]string{"--item", "K-01", "--claim-a", "K-01:E1", "--claim-b", "K-01:E2"})
	if _, err := runConflictCommand(t, create, nil); err != nil {
		t.Fatalf("conflict new: %v", err)
	}

	if _, err := runConflictCommand(t, NewConflictResolveCommand(), []string{"CONFLICT-001"}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	out, err := runConflictCommand(t, NewConflictResolveCommand(), []string{"CONFLICT-001"})
	if err != nil {
		t.Fatalf("second resolve should be a no-op: %v", err)
	}
	if !strings.Contains(out, "já estava resolvido") {
		t.Errorf("second resolve output should warn already resolved: %q", out)
	}

	// Resolve/show de conflito inexistente → erro claro.
	if _, err := runConflictCommand(t, NewConflictResolveCommand(), []string{"CONFLICT-999"}); err == nil {
		t.Error("resolve of missing conflict should fail")
	}
	if _, err := runConflictCommand(t, NewConflictShowCommand(), []string{"CONFLICT-999"}); err == nil {
		t.Error("show of missing conflict should fail")
	}
}

// =============================================================================
// JSON output
// =============================================================================

func TestConflictNew_JSON(t *testing.T) {
	chdirConflictTemp(t)
	globalFlags = GlobalFlags{JSON: true}
	t.Cleanup(func() { globalFlags = GlobalFlags{} })

	create := NewConflictNewCommand()
	_ = create.ParseFlags([]string{"--item", "K-27", "--claim-a", "K-27:E-101", "--claim-b", "K-27:E-203"})
	out, err := runConflictCommand(t, create, nil)
	if err != nil {
		t.Fatalf("conflict new: %v", err)
	}
	var rec knowledge.ConflictRecord
	if err := json.Unmarshal([]byte(out), &rec); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if rec.ID != "CONFLICT-001" || rec.ItemID != "K-27" || rec.ClaimA != "K-27:E-101" || rec.ClaimB != "K-27:E-203" {
		t.Errorf("unexpected json conflict record: %+v", rec)
	}
	if rec.Status != knowledge.ConflictOpen {
		t.Errorf("json status = %q, want open", rec.Status)
	}
}

// compile-time guard: os comandos são construídos no padrão cobra.
var (
	_ *cobra.Command = NewConflictCommand()
	_ *cobra.Command = NewConflictNewCommand()
	_ *cobra.Command = NewConflictListCommand()
	_ *cobra.Command = NewConflictShowCommand()
	_ *cobra.Command = NewConflictResolveCommand()
)
