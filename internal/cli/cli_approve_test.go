//
// Tests for `cosca approve --trace` (internal/cli/approve.go) — Trace ID
// universal + ledger append-only (.cosca/trace.db), opt-in.
//
// Covers:
//   - `approve --trace` (confirmado + testes passando) registra TESTS_STARTED,
//     TEST_PASSED e APPROVED no trace.db
//   - Sem --trace, nenhuma trace.db é criada (zero side effects)
//
// Usa o formatter-injection pattern (mesmo do cli_delegate_test.go) e o
// fluxo real de aprovação: confirmação "y" → go test → audit → decisão.
//
// NOTE: os testes chdir() para um diretório temporário — não rodam em
// paralelo.

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/trace"
)

// runApprove executa `cosca approve` em dir com os argumentos dados,
// injetando o OutputFormatter no contexto (formatter-injection pattern).
func runApprove(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	if stdin != "" {
		root.SetIn(strings.NewReader(stdin))
	}
	root.SetContext(newContextWithFormatter(context.Background(),
		NewOutputFormatter(&buf, OutputFormatText, false, false, false)))
	root.SetArgs(args)
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v\noutput:\n%s", err, buf.String())
	}
	return buf.String()
}

// extractTraceID extrai o Trace ID universal impresso por `approve --trace`.
func extractTraceID(t *testing.T, output string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if id, ok := trace.Parse(strings.TrimPrefix(strings.TrimSpace(line), "Trace: ")); ok {
			return id.String()
		}
	}
	t.Fatalf("no Trace ID in output:\n%s", output)
	return ""
}

// TestApproveCommand_TraceFlag — `approve --trace` (opt-in) e zero side
// effects sem a flag.
func TestApproveCommand_TraceFlag(t *testing.T) {
	t.Run("with --trace records TESTS_STARTED, TEST_PASSED, APPROVED", func(t *testing.T) {
		dir := t.TempDir()
		createApprovalTestModule(t, dir)
		planPath := filepath.Join(dir, "plan.json")
		writeTestFile(t, planPath, approvalTestPlanJSON)

		output := runApprove(t, dir, "y\n", "approve", "--plan", planPath, "--trace")
		if !strings.Contains(output, "Trace: TRACE-") {
			t.Errorf("output missing Trace ID; output:\n%s", output)
		}

		storePath := filepath.Join(dir, ".cosca", "trace.db")
		if _, err := os.Stat(storePath); err != nil {
			t.Fatalf("expected trace.db to be created: %v", err)
		}
		store, err := trace.NewStore(storePath)
		if err != nil {
			t.Fatalf("open trace store: %v", err)
		}
		defer func() { _ = store.Close() }()

		events, err := store.Get(extractTraceID(t, output))
		if err != nil {
			t.Fatalf("get trace events: %v", err)
		}
		results := make(map[string]string)
		for _, e := range events {
			results[e.Action] = e.Result
		}
		for _, want := range []string{"TESTS_STARTED", "TEST_PASSED", "APPROVED"} {
			if _, ok := results[want]; !ok {
				t.Errorf("trace db missing %s event; events: %+v", want, events)
			}
		}
		for action, wantResult := range map[string]string{
			"TESTS_STARTED": "running",
			"TEST_PASSED":   "passed",
			"APPROVED":      "approved",
		} {
			if got := results[action]; got != wantResult {
				t.Errorf("%s result = %q, want %q", action, got, wantResult)
			}
		}
	})

	t.Run("without --trace no trace db is created", func(t *testing.T) {
		dir := t.TempDir()
		createApprovalTestModule(t, dir)
		planPath := filepath.Join(dir, "plan.json")
		writeTestFile(t, planPath, approvalTestPlanJSON)

		runApprove(t, dir, "y\n", "approve", "--plan", planPath)

		if _, err := os.Stat(filepath.Join(dir, ".cosca", "trace.db")); !os.IsNotExist(err) {
			t.Errorf("expected no trace.db without --trace (stat err=%v)", err)
		}
	})
}
