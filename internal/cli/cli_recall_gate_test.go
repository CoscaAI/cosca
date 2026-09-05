// Tests for the `cosca gate recall` subcommand. No network, no LLM: the
// retriever is a stub and the baseline is a fixture in t.TempDir().
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/grounding"
)

func recallBaselineJSON() string {
	return `{"version":1,"embedding":{"model":"nomic-embed-text","dim":"768"},"queries":[
		{"query":"q1","relevant_chunk_ids":["c1","c2"],"first_relevant_chunk_id":"c1"},
		{"query":"q2","relevant_chunk_ids":["c3"],"first_relevant_chunk_id":"c3"}]}`
}

func recallAuditCmd(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	cmd.SetContext(newContextWithFormatter(context.Background(), f))
	return cmd, &buf
}

func passRetriever() grounding.Retriever {
	return func(_ context.Context, query string, _ int) ([]grounding.Result, error) {
		switch query {
		case "q1":
			return []grounding.Result{{ChunkID: "c1"}, {ChunkID: "c2"}}, nil
		case "q2":
			return []grounding.Result{{ChunkID: "c3"}}, nil
		default:
			return nil, nil
		}
	}
}

func failRetriever() grounding.Retriever {
	return func(_ context.Context, query string, _ int) ([]grounding.Result, error) {
		if query == "erro" {
			return nil, errors.New("engine indisponível")
		}
		return []grounding.Result{{ChunkID: "zzz"}}, nil
	}
}

func TestGateRecallCommand_Properties(t *testing.T) {
	cmd := NewGateRecallCommand()
	if cmd == nil {
		t.Fatal("NewGateRecallCommand returned nil")
	}
	if cmd.Use != "recall" {
		t.Errorf("expected Use='recall', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long description")
	}
	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("recall should accept no args: %v", err)
	}
	for _, flag := range []string{"audit", "strict", "summary", "baseline", "k"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("recall must define --%s", flag)
		}
	}
}

func TestGateRecall_RegisteredInGateTree(t *testing.T) {
	cmd := NewGateCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "recall" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("recall subcommand not registered in gate command")
	}
}

func TestRecallCommand_CheckValidBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "qrels-baseline.json")
	writeTestFile(t, path, recallBaselineJSON())

	cmd := NewGateRecallCommand()
	if err := cmd.ParseFlags([]string{"--baseline", path}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := runGateCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("recall --check: %v", err)
	}
	if !strings.Contains(out, "Baseline válido") {
		t.Errorf("check output missing confirmation: %q", out)
	}
	if !strings.Contains(out, "Queries do gabarito") {
		t.Errorf("check output missing key: %q", out)
	}
	if !strings.Contains(out, "2") {
		t.Errorf("check output missing query count: %q", out)
	}
}

func TestRecallCommand_CheckInvalidBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "qrels-baseline.json")
	writeTestFile(t, path, `[{"query":"q","relevant_chunk_ids":[]}]`)

	cmd := NewGateRecallCommand()
	if err := cmd.ParseFlags([]string{"--baseline", path}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err := runGateCommand(t, cmd, nil)
	if err == nil {
		t.Fatal("recall --check with invalid baseline should fail")
	}
	if !strings.Contains(err.Error(), "relevant_chunk_ids") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRecallCommand_CheckMissingBaseline(t *testing.T) {
	cmd := NewGateRecallCommand()
	if err := cmd.ParseFlags([]string{"--baseline", filepath.Join(t.TempDir(), "nope.json")}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err := runGateCommand(t, cmd, nil)
	if err == nil {
		t.Fatal("recall --check with missing baseline should fail")
	}
}

func TestRecallCommand_CheckSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qrels-baseline.json")
	writeTestFile(t, path, recallBaselineJSON())

	cmd, buf := recallAuditCmd(t)
	f := GetFormatter(cmd)
	if err := runRecallCheck(cmd, f, false, true, path); err != nil {
		t.Fatalf("runRecallCheck summary: %v", err)
	}
	if !strings.Contains(buf.String(), "recall: baseline OK") {
		t.Errorf("summary output: %q", buf.String())
	}
}

func TestRecallCommand_CheckJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qrels-baseline.json")
	writeTestFile(t, path, recallBaselineJSON())

	cmd, buf := recallAuditCmd(t)
	f := GetFormatter(cmd)
	if err := runRecallCheck(cmd, f, true, false, path); err != nil {
		t.Fatalf("runRecallCheck json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, buf.String())
	}
	if parsed["check"] != "baseline" {
		t.Errorf("expected check=baseline, got %v", parsed["check"])
	}
}

func TestRecallAudit_Pass(t *testing.T) {
	cmd, buf := recallAuditCmd(t)
	f := GetFormatter(cmd)
	baseline := []grounding.Qrels{
		{Query: "q1", RelevantChunkIDs: []string{"c1", "c2"}, FirstRelevantChunkID: "c1"},
		{Query: "q2", RelevantChunkIDs: []string{"c3"}, FirstRelevantChunkID: "c3"},
	}
	err := runRecallAudit(cmd, f, false, false, false, passRetriever(), baseline, grounding.DefaultFloor())
	if err != nil {
		t.Fatalf("runRecallAudit: %v", err)
	}
	if !strings.Contains(buf.String(), "RECALL OK") {
		t.Errorf("audit output missing success: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "recall@5") {
		t.Errorf("audit output missing recall@K: %q", buf.String())
	}
}

func TestRecallAudit_StrictFail(t *testing.T) {
	cmd, buf := recallAuditCmd(t)
	f := GetFormatter(cmd)
	baseline := []grounding.Qrels{
		{Query: "q1", RelevantChunkIDs: []string{"c1"}, FirstRelevantChunkID: "c1"},
	}
	err := runRecallAudit(cmd, f, false, true, false, failRetriever(), baseline, grounding.DefaultFloor())
	if err == nil {
		t.Fatalf("strict should fail; output: %q", buf.String())
	}
	var ece ExitCodeError
	if !errors.As(err, &ece) || ece.Code != 1 {
		t.Errorf("expected ExitCodeError{1}, got %v", err)
	}
}

func TestRecallAudit_JSON(t *testing.T) {
	cmd, buf := recallAuditCmd(t)
	f := GetFormatter(cmd)
	baseline := []grounding.Qrels{
		{Query: "q1", RelevantChunkIDs: []string{"c1"}, FirstRelevantChunkID: "c1"},
	}
	if err := runRecallAudit(cmd, f, true, false, false, passRetriever(), baseline, grounding.DefaultFloor()); err != nil {
		t.Fatalf("runRecallAudit: %v", err)
	}
	var rep grounding.RecallReport
	if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
		t.Fatalf("output not RecallReport JSON: %v\n%s", err, buf.String())
	}
	if !rep.Passed {
		t.Errorf("expected Passed=true")
	}
}

func TestRecallSummaryLine(t *testing.T) {
	passed := &grounding.RecallReport{K: 5, RecallAtK: 1.0, FirstRelevantHit: 1.0, Passed: true}
	if got := recallSummaryLine(passed); got != "recall: OK (recall@5 1.00, first 1.00)" {
		t.Errorf("summary passed = %q", got)
	}
	failed := &grounding.RecallReport{K: 5, RecallAtK: 0.2, FirstRelevantHit: 0.1, Passed: false, QueriesErrored: 1}
	if got := recallSummaryLine(failed); !strings.Contains(got, "recall: FAIL") {
		t.Errorf("summary failed = %q", got)
	}
}

func TestRecallResolveBaseline_FindsDefault(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "qrels-baseline.json"), recallBaselineJSON())
	t.Chdir(dir)
	path, err := resolveRecallBaseline("")
	if err != nil {
		t.Fatalf("resolveRecallBaseline: %v", err)
	}
	if path != filepath.Join(dir, "qrels-baseline.json") {
		t.Errorf("resolved %q, want %q", path, filepath.Join(dir, "qrels-baseline.json"))
	}
}
