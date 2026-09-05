//
// Tests for the `cosca cron` command tree (internal/cli/cron.go).
//
// Covers:
//   - Registration of `cron` in the root command
//   - Command properties + arg/flag constraints
//   - `cron add --name --schedule --cmd` → C-0001 em .cosca/cron.db
//   - `cron add` com agenda inválida → erro pt-BR claro
//   - `cron list` → tabela (id, nome, agenda, habilitado, próxima); vazio → aviso
//   - `cron run <id>` → executa o comando UMA vez e registra Runs (marker file)
//   - `cron remove <id>` → remove do .cosca/cron.db
//   - JSON output (add/list)
//
// NOTE: os testes fazem chdir() (mutam o cwd do processo) e NÃO rodam em
// paralelo. Formatter injetado via newContextWithFormatter (padrão do CLI).
// Nenhum daemon é iniciado — os subcomandos testados são pontuais.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/scheduler"
)

// chdirCronTemp muda para um diretório temporário e restaura o cwd no cleanup.
func chdirCronTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Chdir registra o restore do cwd DEPOIS do RemoveAll do TempDir
	// (cleanups rodam em LIFO) — no Windows não dá para remover o
	// diretório que é o CWD do processo.
	t.Chdir(dir)
	return dir
}

// runCronCommand executa o RunE de um subcomando com formatter injetado.
func runCronCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
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

// cronDBPath retorna o caminho da base do cron no diretório dado.
func cronDBPath(dir string) string {
	return filepath.Join(dir, ".cosca", "cron.db")
}

// =============================================================================
// Registration
// =============================================================================

func TestCronCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "cron" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("cron subcommand not registered in root command")
	}
}

func TestCronCommand_Properties(t *testing.T) {
	cmd := NewCronCommand()
	if cmd == nil {
		t.Fatal("NewCronCommand returned nil")
	}
	if cmd.Use != "cron" {
		t.Errorf("expected Use='cron', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long description")
	}
	expected := []string{"add", "list", "remove", "run", "daemon"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing cron subcommand: %s", name)
		}
	}
}

func TestCronCommand_DocumentsNoAutoRun(t *testing.T) {
	cmd := NewCronDaemonCommand()
	if cmd.Long == "" {
		t.Fatal("daemon command must document the no-auto-run rule")
	}
	for _, key := range []string{"nada roda", "foreground", "Ctrl+C"} {
		if !strings.Contains(strings.ToLower(cmd.Long), strings.ToLower(key)) {
			t.Errorf("daemon docs missing %q", key)
		}
	}
}

func TestCron_ArgConstraints(t *testing.T) {
	remove := NewCronRemoveCommand()
	if err := remove.Args(remove, nil); err == nil {
		t.Error("remove should require an id")
	}
	if err := remove.Args(remove, []string{"C-0001"}); err != nil {
		t.Errorf("remove with one arg should be allowed: %v", err)
	}

	run := NewCronRunCommand()
	if err := run.Args(run, []string{"C-0001", "C-0002"}); err == nil {
		t.Error("run with two args should fail")
	}
}

// =============================================================================
// `cron list` — vazio
// =============================================================================

func TestCronList_Empty(t *testing.T) {
	chdirCronTemp(t)
	out, err := runCronCommand(t, NewCronListCommand(), nil)
	if err != nil {
		t.Fatalf("cron list (empty): %v", err)
	}
	if !strings.Contains(out, "Nenhum job cron") {
		t.Errorf("expected empty-state warning, got: %q", out)
	}
}

// =============================================================================
// Ciclo completo: add → list → run → remove
// =============================================================================

func TestCronFullCycle(t *testing.T) {
	dir := chdirCronTemp(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("cron run executa comando via sh -c — requer bash no PATH (não disponível no Windows nativo)")
	}

	// add
	add := NewCronAddCommand()
	_ = add.ParseFlags([]string{
		"--name", "backup",
		"--schedule", "every 30m",
		"--cmd", "echo ok",
	})
	out, err := runCronCommand(t, add, nil)
	if err != nil {
		t.Fatalf("cron add: %v", err)
	}
	if !strings.Contains(out, "C-0001") {
		t.Errorf("add output missing C-0001: %q", out)
	}
	if _, err := os.Stat(cronDBPath(dir)); err != nil {
		t.Fatalf("cron.db not created: %v", err)
	}

	// list (populado)
	out, err = runCronCommand(t, NewCronListCommand(), nil)
	if err != nil {
		t.Fatalf("cron list: %v", err)
	}
	if !strings.Contains(out, "C-0001") || !strings.Contains(out, "backup") {
		t.Errorf("list missing the added job: %q", out)
	}
	if !strings.Contains(out, "every 30m") {
		t.Errorf("list missing the schedule: %q", out)
	}

	// run
	run := NewCronRunCommand()
	out, err = runCronCommand(t, run, []string{"C-0001"})
	if err != nil {
		t.Fatalf("cron run: %v", err)
	}
	if !strings.Contains(out, "sucesso") {
		t.Errorf("run output missing success message: %q", out)
	}

	// verify the run was recorded
	store, err := scheduler.NewStore(cronDBPath(dir))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()
	job, err := store.Get("C-0001")
	if err != nil || job == nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Runs != 1 {
		t.Errorf("Runs after cron run = %d, want 1", job.Runs)
	}

	// remove
	out, err = runCronCommand(t, NewCronRemoveCommand(), []string{"C-0001"})
	if err != nil {
		t.Fatalf("cron remove: %v", err)
	}
	if !strings.Contains(out, "removido") {
		t.Errorf("remove output missing confirmation: %q", out)
	}
	job, _ = store.Get("C-0001")
	if job != nil {
		t.Error("job should be gone after remove")
	}
}

// =============================================================================
// `cron run` — execução real no diretório de trabalho
// =============================================================================

func TestCronRun_ExecutesCommandInCWD(t *testing.T) {
	dir := chdirCronTemp(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("cron run executa comando via sh -c — requer bash no PATH (não disponível no Windows nativo)")
	}

	add := NewCronAddCommand()
	_ = add.ParseFlags([]string{
		"--name", "marker",
		"--schedule", "every 30m",
		"--cmd", "echo ok > marker.txt",
	})
	if _, err := runCronCommand(t, add, nil); err != nil {
		t.Fatalf("add: %v", err)
	}

	if _, err := runCronCommand(t, NewCronRunCommand(), []string{"C-0001"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "marker.txt")); err != nil {
		t.Errorf("marker.txt should exist after cron run: %v", err)
	}
}

func TestCronRun_MissingJobFails(t *testing.T) {
	chdirCronTemp(t)
	_, err := runCronCommand(t, NewCronRunCommand(), []string{"C-9999"})
	if err == nil {
		t.Error("expected error for missing job")
	}
}

func TestCronRemove_MissingJobFails(t *testing.T) {
	chdirCronTemp(t)
	_, err := runCronCommand(t, NewCronRemoveCommand(), []string{"C-9999"})
	if err == nil {
		t.Error("expected error for removing a missing job")
	}
}

// =============================================================================
// `cron add` — validação
// =============================================================================

func TestCronAdd_InvalidSchedule(t *testing.T) {
	chdirCronTemp(t)
	add := NewCronAddCommand()
	_ = add.ParseFlags([]string{
		"--name", "x",
		"--schedule", "banana",
		"--cmd", "true",
	})
	_, err := runCronCommand(t, add, nil)
	if err == nil {
		t.Fatal("expected error for invalid schedule")
	}
	if !strings.Contains(err.Error(), "agenda") {
		t.Errorf("expected clear pt-BR error mentioning agenda, got: %v", err)
	}
}

func TestCronAdd_RequiresFlags(t *testing.T) {
	chdirCronTemp(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing name", []string{"--schedule", "every 30m", "--cmd", "true"}, "name"},
		{"missing schedule", []string{"--name", "x", "--cmd", "true"}, "agenda"},
		{"missing cmd", []string{"--name", "x", "--schedule", "every 30m"}, "command"},
	}
	for _, c := range cases {
		add := NewCronAddCommand()
		_ = add.ParseFlags(c.args)
		if _, err := runCronCommand(t, add, nil); err == nil {
			t.Errorf("%s: expected error", c.name)
			continue
		}
	}
}

// =============================================================================
// JSON output
// =============================================================================

func TestCronAdd_JSON(t *testing.T) {
	chdirCronTemp(t)
	globalFlags = GlobalFlags{JSON: true}
	defer func() { globalFlags = GlobalFlags{} }()

	add := NewCronAddCommand()
	_ = add.ParseFlags([]string{"--name", "T", "--schedule", "daily at 9am", "--cmd", "true"})
	out, err := runCronCommand(t, add, nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	var job scheduler.Job
	if err := json.Unmarshal([]byte(out), &job); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if job.ID != "C-0001" || job.Name != "T" || job.Schedule.Kind != scheduler.KindDaily {
		t.Errorf("unexpected json job: %+v", job)
	}
}

func TestCronList_JSON(t *testing.T) {
	chdirCronTemp(t)
	globalFlags = GlobalFlags{JSON: true}
	defer func() { globalFlags = GlobalFlags{} }()

	add := NewCronAddCommand()
	_ = add.ParseFlags([]string{"--name", "T", "--schedule", "every 30m", "--cmd", "true"})
	if _, err := runCronCommand(t, add, nil); err != nil {
		t.Fatalf("add: %v", err)
	}

	out, err := runCronCommand(t, NewCronListCommand(), nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var jobs []scheduler.Job
	if err := json.Unmarshal([]byte(out), &jobs); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(jobs) != 1 || jobs[0].ID != "C-0001" {
		t.Errorf("unexpected json list: %+v", jobs)
	}
}

// compile-time guard: the command constructors must match the cobra contract.
var (
	_ *cobra.Command = NewCronCommand()
	_ *cobra.Command = NewCronAddCommand()
	_ *cobra.Command = NewCronListCommand()
	_ *cobra.Command = NewCronRemoveCommand()
	_ *cobra.Command = NewCronRunCommand()
	_ *cobra.Command = NewCronDaemonCommand()
)
