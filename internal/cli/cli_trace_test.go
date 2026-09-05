//
// Tests for the `cosca trace` command tree (internal/cli/trace.go).
//
// Covers:
//   - Registration of `trace` (and its 5 subcommands) in the root command
//   - Command properties and arg constraints
//   - `trace new` output (Trace ID universal)
//   - `trace event` append + validation (--action/--actor obrigatórios)
//   - `trace show` timeline table (flight recorder)
//   - `trace diff` SUSPICIOUS DIVERGENCE output
//   - `trace latest` output
//
// NOTE: tests that chdir() must NOT run in parallel — they mutate the
// process working directory.

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/trace"
)

// =============================================================================
// Registration — `trace` in the root command
// =============================================================================

func TestTraceCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "trace" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("trace subcommand not registered in root command")
	}
}

func TestTraceCommand_Properties(t *testing.T) {
	cmd := NewTraceCommand()
	if cmd == nil {
		t.Fatal("NewTraceCommand returned nil")
	}
	if cmd.Use != "trace" {
		t.Errorf("expected Use='trace', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	if cmd.Long == "" {
		t.Error("expected non-empty Long description")
	}

	expected := []string{"new", "event", "show", "causal", "diff", "latest"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing trace subcommand: %s", name)
		}
	}
}

func TestTraceCommand_ArgConstraints(t *testing.T) {
	ev := NewTraceEventCommand()
	if err := ev.Args(ev, nil); err == nil {
		t.Error("event with no args should fail")
	}
	if err := ev.Args(ev, []string{"TRACE-20260802-7F92"}); err != nil {
		t.Errorf("event with one arg should be allowed: %v", err)
	}

	show := NewTraceShowCommand()
	if err := show.Args(show, nil); err == nil {
		t.Error("show with no args should fail")
	}

	diff := NewTraceDiffCommand()
	if err := diff.Args(diff, []string{"a"}); err == nil {
		t.Error("diff with one arg should fail")
	}
	if err := diff.Args(diff, []string{"a", "b"}); err != nil {
		t.Errorf("diff with two args should be allowed: %v", err)
	}

	latest := NewTraceLatestCommand()
	if err := latest.Args(latest, nil); err != nil {
		t.Errorf("latest with no args should be allowed: %v", err)
	}
}

// =============================================================================
// executeTrace runs a `trace` subcommand with cwd set to root.
// =============================================================================

func executeTrace(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	}()

	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	formatter := NewOutputFormatter(buf, OutputFormatText, false, false, false)
	cmd.SetContext(newContextWithFormatter(context.Background(), formatter))
	cmd.SetArgs(append([]string{"trace"}, args...))
	err = cmd.Execute()
	return buf.String(), err
}

// fakeTraceTree builds a fake .cosca project tree for CLI tests.
func fakeTraceTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeCLIFile(t, root, ".cosca/config.yaml", "cache:\n  enabled: true\n")
	return root
}

// =============================================================================
// `trace new`
// =============================================================================

func TestTraceNew_Output(t *testing.T) {
	root := fakeTraceTree(t)
	out, err := executeTrace(t, root, "new")
	if err != nil {
		t.Fatalf("trace new: %v", err)
	}
	re := regexp.MustCompile(`TRACE-\d{8}-[0-9A-F]{4}`)
	if !re.MatchString(out) {
		t.Errorf("output sem Trace ID universal: %q", out)
	}
	if !strings.Contains(out, "Trace ID") {
		t.Errorf("output sem chave 'Trace ID': %q", out)
	}
}

// =============================================================================
// `trace event`
// =============================================================================

func TestTraceEvent_AppendAndShow(t *testing.T) {
	root := fakeTraceTree(t)
	id := trace.NewID().String()

	// Falta --action → erro.
	if _, err := executeTrace(t, root, "event", id, "--actor", "kernel"); err == nil {
		t.Error("event sem --action deveria falhar")
	}
	// Falta --actor → erro.
	if _, err := executeTrace(t, root, "event", id, "--action", "TASK_STARTED"); err == nil {
		t.Error("event sem --actor deveria falhar")
	}
	// Trace ID inválido → erro.
	if _, err := executeTrace(t, root, "event", "NAO-EH-TRACE", "--action", "X", "--actor", "kernel"); err == nil {
		t.Error("event com Trace ID inválido deveria falhar")
	}

	// Append válido.
	out, err := executeTrace(t, root, "event", id, "--action", "TASK_STARTED", "--actor", "kernel", "--result", "running", "--details", "tarefa #42")
	if err != nil {
		t.Fatalf("trace event: %v", err)
	}
	if !strings.Contains(out, "TASK_STARTED") || !strings.Contains(out, "kernel") {
		t.Errorf("output do evento incompleto: %q", out)
	}
	if !strings.Contains(out, "registrado") {
		t.Errorf("output sem confirmação de registro: %q", out)
	}

	// Show → timeline com o evento.
	out, err = executeTrace(t, root, "show", id)
	if err != nil {
		t.Fatalf("trace show: %v", err)
	}
	if !strings.Contains(out, "TRACE "+id) {
		t.Errorf("show sem header do trace: %q", out)
	}
	if !strings.Contains(out, "TASK_STARTED") || !strings.Contains(out, "tarefa #42") {
		t.Errorf("show sem o evento registrado: %q", out)
	}
}

// =============================================================================
// `trace diff` — SUSPICIOUS DIVERGENCE
// =============================================================================

func TestTraceDiff_SuspiciousDivergence(t *testing.T) {
	root := fakeTraceTree(t)
	idA := trace.NewID().String()
	idB := trace.NewID().String()

	// Referência A: PLAN_CREATED → TASK_STARTED → TEST_PASSED.
	for _, ev := range []struct {
		action string
		result string
	}{
		{"PLAN_CREATED", "success"},
		{"TASK_STARTED", "running"},
		{"TEST_PASSED", "success"},
	} {
		if _, err := executeTrace(t, root, "event", idA, "--action", ev.action, "--actor", "kernel", "--result", ev.result); err != nil {
			t.Fatalf("event A: %v", err)
		}
	}

	// B diverge: PLAN_CREATED → TASK_STARTED → TEST_FAILED.
	for _, ev := range []struct {
		action string
		result string
	}{
		{"PLAN_CREATED", "success"},
		{"TASK_STARTED", "running"},
		{"TEST_FAILED", "failed"},
	} {
		if _, err := executeTrace(t, root, "event", idB, "--action", ev.action, "--actor", "agent-x", "--result", ev.result); err != nil {
			t.Fatalf("event B: %v", err)
		}
	}

	out, err := executeTrace(t, root, "diff", idA, idB)
	if err != nil {
		t.Fatalf("trace diff: %v", err)
	}
	if !strings.Contains(out, "SUSPICIOUS DIVERGENCE") {
		t.Errorf("diff sem marcador SUSPICIOUS DIVERGENCE: %q", out)
	}
	// Divergência na posição 3 (TEST_FAILED ≠ TEST_PASSED).
	if !strings.Contains(out, "posição 3") {
		t.Errorf("diff sem a posição da divergência: %q", out)
	}
	if !strings.Contains(out, "TEST_PASSED") || !strings.Contains(out, "TEST_FAILED") {
		t.Errorf("diff sem as ações divergentes: %q", out)
	}
}

func TestTraceDiff_NoDivergence(t *testing.T) {
	root := fakeTraceTree(t)
	idA := trace.NewID().String()
	idB := trace.NewID().String()

	for _, id := range []string{idA, idB} {
		for _, ev := range []struct {
			action string
			result string
		}{
			{"PLAN_CREATED", "success"},
			{"TASK_STARTED", "running"},
		} {
			if _, err := executeTrace(t, root, "event", id, "--action", ev.action, "--actor", "kernel", "--result", ev.result); err != nil {
				t.Fatalf("event: %v", err)
			}
		}
	}

	out, err := executeTrace(t, root, "diff", idA, idB)
	if err != nil {
		t.Fatalf("trace diff: %v", err)
	}
	if strings.Contains(out, "SUSPICIOUS DIVERGENCE") {
		t.Errorf("diff de sequências idênticas não deveria divergir: %q", out)
	}
	if !strings.Contains(out, "Sem divergência") {
		t.Errorf("diff sem mensagem de sem-divergência: %q", out)
	}
}

// =============================================================================
// `trace latest`
// =============================================================================

func TestTraceLatest_EmptyAndPopulated(t *testing.T) {
	root := fakeTraceTree(t)

	out, err := executeTrace(t, root, "latest")
	if err != nil {
		t.Fatalf("trace latest (empty): %v", err)
	}
	if !strings.Contains(out, "Nenhum evento") {
		t.Errorf("expected empty-state warning, got: %q", out)
	}

	id := trace.NewID().String()
	if _, err := executeTrace(t, root, "event", id, "--action", "TASK_STARTED", "--actor", "kernel"); err != nil {
		t.Fatalf("event: %v", err)
	}
	out, err = executeTrace(t, root, "latest", "--limit", "5")
	if err != nil {
		t.Fatalf("trace latest: %v", err)
	}
	if !strings.Contains(out, id) || !strings.Contains(out, "TASK_STARTED") {
		t.Errorf("latest sem o evento registrado: %q", out)
	}
}

// =============================================================================
// `trace` with no subcommand → help
// =============================================================================

func TestTraceCommand_NoSubcommand_ShowsHelp(t *testing.T) {
	root := fakeTraceTree(t)
	out, err := executeTrace(t, root)
	if err != nil {
		t.Fatalf("trace with no subcommand should show help without error: %v", err)
	}
	if !strings.Contains(out, "new") || !strings.Contains(out, "diff") {
		t.Errorf("expected help listing subcommands, got: %q", out)
	}
}

// O ledger append-only deve persistir em .cosca/trace.db dentro do projeto.
func TestTrace_StoreFileCreatedInProject(t *testing.T) {
	root := fakeTraceTree(t)
	if _, err := executeTrace(t, root, "new"); err != nil {
		t.Fatalf("trace new: %v", err)
	}
	id := trace.NewID().String()
	if _, err := executeTrace(t, root, "event", id, "--action", "PLAN_CREATED", "--actor", "kernel"); err != nil {
		t.Fatalf("trace event: %v", err)
	}
	storePath := filepath.Join(root, ".cosca", "trace.db")
	if _, err := os.Stat(storePath); err != nil {
		t.Errorf("trace.db não criado no projeto: %v", err)
	}
}

// compile-time guard: the command constructors must match the cobra contract.
var (
	_ *cobra.Command = NewTraceCommand()
	_ *cobra.Command = NewTraceNewCommand()
	_ *cobra.Command = NewTraceEventCommand()
	_ *cobra.Command = NewTraceShowCommand()
	_ *cobra.Command = NewTraceCausalCommand()
	_ *cobra.Command = NewTraceDiffCommand()
	_ *cobra.Command = NewTraceLatestCommand()
)
