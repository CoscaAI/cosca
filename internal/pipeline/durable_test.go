package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// countingRunner simulates pipeline execution and counts how many times each
// task actually ran. Used to prove durable idempotency: a resumed run must NOT
// re-execute already-completed steps.
type countingRunner struct {
	mu    sync.Mutex
	calls int
	byKey map[string]int
}

func (c *countingRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	c.mu.Lock()
	c.calls++
	if c.byKey == nil {
		c.byKey = make(map[string]int)
	}
	c.byKey[req.Prompt]++
	c.mu.Unlock()
	return &RunResult{
		Response:    "ok: " + req.Prompt,
		Agent:       req.Agent,
		BuildResult: &BuildResult{Success: true, Output: "build ok", DurationMs: 1},
		TestResult:  &TestResult{Success: true, Passed: 1, Failed: 0, Output: "tests ok", DurationMs: 1},
	}, nil
}

func (c *countingRunner) RunStream(ctx context.Context, req RunRequest) (<-chan RunEvent, error) {
	ch := make(chan RunEvent, 1)
	ch <- RunEvent{Type: EventDone}
	close(ch)
	return ch, nil
}

func (c *countingRunner) CallCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func durableTestPlan(id string) *Plan {
	return &Plan{
		ID:     id,
		Intent: "durable test",
		Tasks: []*TaskNode{
			{ID: "t1", Description: "durable task one", Agent: "a", Status: TaskPending},
			{ID: "t2", Description: "durable task two", Agent: "a", Status: TaskPending, DependsOn: []string{"t1"}},
			{ID: "t3", Description: "durable task three", Agent: "a", Status: TaskPending, DependsOn: []string{"t2"}},
		},
	}
}

// TestDurableEventLog_AppendLoadRoundTrip verifies the append + load cycle
// preserves order, sequence, and the embedded result.
func TestDurableEventLog_AppendLoadRoundTrip(t *testing.T) {
	l, err := NewDurableEventLog(t.TempDir())
	if err != nil {
		t.Fatalf("NewDurableEventLog: %v", err)
	}

	res := &TaskResult{Success: true, Output: "built", Agent: "cosca-backend", DurationMs: 42}
	events := []struct {
		typ, task string
		res       *TaskResult
	}{
		{DurablePlanCreated, "", nil},
		{DurableStepStarted, "t1", nil},
		{DurableStepCompleted, "t1", res},
		{DurableRunCompleted, "", nil},
	}
	for i, e := range events {
		ev, err := l.Append("run-1", e.typ, e.task, e.res)
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
		if ev.Seq != i+1 {
			t.Fatalf("event %d seq = %d, want %d", i, ev.Seq, i+1)
		}
		if ev.Hash == "" || len(ev.Hash) != 64 {
			t.Fatalf("event %d hash = %q, want 64-hex", i, ev.Hash)
		}
	}

	loaded, err := l.LoadRun("run-1")
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if len(loaded) != 4 {
		t.Fatalf("loaded %d events, want 4", len(loaded))
	}
	for i, ev := range loaded {
		if ev.Seq != i+1 {
			t.Errorf("loaded event %d seq = %d, want %d", i, ev.Seq, i+1)
		}
		if ev.Type != events[i].typ {
			t.Errorf("loaded event %d type = %q, want %q", i, ev.Type, events[i].typ)
		}
	}
	last := loaded[len(loaded)-1]
	if last.Type != DurableRunCompleted {
		t.Errorf("last event type = %q, want run_completed", last.Type)
	}
	if loaded[2].Result == nil || loaded[2].Result.Output != "built" {
		t.Errorf("completed event result lost: %+v", loaded[2].Result)
	}

	runs, err := l.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 || runs[0] != "run-1" {
		t.Errorf("ListRuns = %v, want [run-1]", runs)
	}

	taskID, result, ok, err := l.LastCompleted("run-1")
	if err != nil {
		t.Fatalf("LastCompleted: %v", err)
	}
	if !ok || taskID != "t1" || result == nil || result.Output != "built" {
		t.Errorf("LastCompleted = (%q, %+v, %v), want (t1, built, true)", taskID, result, ok)
	}

	// A run with events but no terminal event is stale.
	stale, err := l.IsStale("run-1")
	if err != nil {
		t.Fatalf("IsStale: %v", err)
	}
	if stale {
		t.Errorf("run-1 has run_completed, IsStale should be false")
	}
}

// TestDurableEventLog_VerifyChainDetectsTampering proves the hash chain
// integrity: mutating a single byte breaks VerifyChain.
func TestDurableEventLog_VerifyChainDetectsTampering(t *testing.T) {
	dir := t.TempDir()
	l, err := NewDurableEventLog(dir)
	if err != nil {
		t.Fatalf("NewDurableEventLog: %v", err)
	}
	_, _ = l.Append("run-t", DurablePlanCreated, "", nil)
	_, _ = l.Append("run-t", DurableStepStarted, "t1", nil)
	_, _ = l.Append("run-t", DurableStepCompleted, "t1", &TaskResult{Success: true, Output: "hello-world", Agent: "a"})

	ok, err := l.VerifyChain("run-t")
	if err != nil || !ok {
		t.Fatalf("VerifyChain on clean chain = (%v, %v), want (true, nil)", ok, err)
	}

	// Tamper: rewrite the file with a mutated event payload.
	path := filepath.Join(dir, "run-t.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	modified := strings.Replace(string(data), "hello-world", "TAMPERED", 1)
	if modified == string(data) {
		t.Fatal("tamper replacement had no effect — log content unexpected")
	}
	if err := os.WriteFile(path, []byte(modified), 0o600); err != nil {
		t.Fatalf("write tampered log: %v", err)
	}

	ok, err = l.VerifyChain("run-t")
	if ok || err == nil {
		t.Fatalf("VerifyChain on tampered chain = (%v, %v), want (false, error)", ok, err)
	}

	// Unknown run: no events → valid (vacuous).
	ok, err = l.VerifyChain("run-missing")
	if err != nil || !ok {
		t.Fatalf("VerifyChain on missing run = (%v, %v), want (true, nil)", ok, err)
	}
}

// TestRunPlan_DurableIdempotentResume proves the core of durable execution:
// running the same run ID twice executes each step exactly once. The second
// run replays cached step_completed results instead of re-running them.
func TestRunPlan_DurableIdempotentResume(t *testing.T) {
	runner := &countingRunner{}
	dir := t.TempDir()
	l, err := NewDurableEventLog(filepath.Join(dir, "events"))
	if err != nil {
		t.Fatalf("NewDurableEventLog: %v", err)
	}

	sr := NewStepRunner(runner)
	sr.SetDurableLog(l)
	plan := durableTestPlan("run-idem")
	sr.SetPlanID(plan.ID)

	res1, err := sr.RunPlan(context.Background(), plan)
	if err != nil {
		t.Fatalf("RunPlan #1: %v", err)
	}
	if res1.TasksDone != 3 || res1.TasksFailed != 0 {
		t.Fatalf("RunPlan #1 result = %+v, want 3 done / 0 failed", res1)
	}
	if runner.CallCount() != 3 {
		t.Fatalf("first run executed %d steps, want 3", runner.CallCount())
	}

	// "Resume": a fresh StepRunner + fresh plan with the same run ID. The
	// durable log must replay t1..t3 without re-executing them.
	plan2 := durableTestPlan("run-idem")
	sr2 := NewStepRunner(runner) // same counting runner: calls accumulate
	sr2.SetDurableLog(l)
	sr2.SetPlanID(plan2.ID)

	res2, err := sr2.RunPlan(context.Background(), plan2)
	if err != nil {
		t.Fatalf("RunPlan #2: %v", err)
	}
	if res2.TasksDone != 3 || res2.TasksFailed != 0 {
		t.Fatalf("RunPlan #2 result = %+v, want 3 done / 0 failed", res2)
	}
	if runner.CallCount() != 3 {
		t.Fatalf("resume re-executed steps: total calls = %d, want 3 (idempotent)", runner.CallCount())
	}
	if res2.Recovered != 3 {
		t.Fatalf("resume recovered %d tasks, want 3", res2.Recovered)
	}
	if plan2.Tasks[0].Result == nil || plan2.Tasks[0].Result.Output != "ok: durable task one" {
		t.Errorf("replayed result missing: %+v", plan2.Tasks[0].Result)
	}

	// Hash chain must still be valid after the append-only replay.
	if ok, err := l.VerifyChain("run-idem"); err != nil || !ok {
		t.Fatalf("VerifyChain after resume = (%v, %v), want (true, nil)", ok, err)
	}
}

// TestRunPlan_DurableCrashResume simulates a crash: events for t1-t2 are
// appended, then a NEW StepRunner (fresh process equivalent) resumes and must
// continue from t3 without re-running t1/t2.
func TestRunPlan_DurableCrashResume(t *testing.T) {
	dir := t.TempDir()
	l, err := NewDurableEventLog(filepath.Join(dir, "events"))
	if err != nil {
		t.Fatalf("NewDurableEventLog: %v", err)
	}

	// "Previous process" appended events for tasks 1-2 then crashed (no
	// run_completed terminal event → stale).
	_, _ = l.AppendWithTrace("run-crash", DurablePlanCreated, "", "TRACE-20260802-ABCDEF01", nil)
	_, _ = l.Append("run-crash", DurableStepStarted, "t1", nil)
	_, _ = l.Append("run-crash", DurableStepCompleted, "t1", &TaskResult{Success: true, Output: "o1", Agent: "a"})
	_, _ = l.Append("run-crash", DurableStepStarted, "t2", nil)
	_, _ = l.Append("run-crash", DurableStepCompleted, "t2", &TaskResult{Success: true, Output: "o2", Agent: "a"})

	stale, err := l.IsStale("run-crash")
	if err != nil {
		t.Fatalf("IsStale: %v", err)
	}
	if !stale {
		t.Fatal("run-crash should be stale (no terminal event)")
	}

	// "New process": fresh runner + fresh plan, resumes from the log.
	runner := &countingRunner{}
	sr := NewStepRunner(runner)
	sr.SetDurableLog(l)
	plan := durableTestPlan("run-crash")
	sr.SetPlanID(plan.ID)

	if err := sr.ResumeDurable("run-crash", plan); err != nil {
		t.Fatalf("ResumeDurable: %v", err)
	}
	if plan.Tasks[0].Status != TaskCompleted || plan.Tasks[0].Result == nil {
		t.Fatalf("t1 not restored: status=%s result=%+v", plan.Tasks[0].Status, plan.Tasks[0].Result)
	}
	if plan.Tasks[1].Status != TaskCompleted {
		t.Fatalf("t2 not restored: status=%s", plan.Tasks[1].Status)
	}
	if plan.Tasks[2].Status != TaskPending {
		t.Fatalf("t3 should be pending, got %s", plan.Tasks[2].Status)
	}

	res, err := sr.RunPlan(context.Background(), plan)
	if err != nil {
		t.Fatalf("RunPlan after resume: %v", err)
	}
	if runner.CallCount() != 1 {
		t.Fatalf("crash resume executed %d steps, want 1 (only t3)", runner.CallCount())
	}
	if res.TasksDone != 3 || res.TasksFailed != 0 {
		t.Fatalf("resume result = %+v, want 3 done / 0 failed", res)
	}
	if res.Recovered != 2 {
		t.Fatalf("resume recovered %d tasks, want 2 (t1, t2)", res.Recovered)
	}

	// The resume run now carries the trace ID persisted by the crashed run.
	if ok, err := l.VerifyChain("run-crash"); err != nil || !ok {
		t.Fatalf("VerifyChain after crash resume = (%v, %v), want (true, nil)", ok, err)
	}
	events, _ := l.LoadRun("run-crash")
	for _, ev := range events {
		if ev.Type == DurableStepStarted && ev.TaskID == "t3" && ev.TraceID != "TRACE-20260802-ABCDEF01" {
			t.Fatalf("resume did not reuse the persisted trace id: got %q", ev.TraceID)
		}
	}
}

// TestRunPlan_DurableFailedStepIsRetried verifies failures are never cached:
// a step that previously failed is executed again on resume (RecoveryLoop
// semantics), while completed steps stay replayed.
func TestRunPlan_DurableFailedStepIsRetried(t *testing.T) {
	dir := t.TempDir()
	l, err := NewDurableEventLog(filepath.Join(dir, "events"))
	if err != nil {
		t.Fatalf("NewDurableEventLog: %v", err)
	}
	_, _ = l.Append("run-fail", DurablePlanCreated, "", nil)
	_, _ = l.Append("run-fail", DurableStepStarted, "t1", nil)
	_, _ = l.Append("run-fail", DurableStepCompleted, "t1", &TaskResult{Success: true, Output: "o1"})
	_, _ = l.Append("run-fail", DurableStepStarted, "t2", nil)
	_, _ = l.Append("run-fail", DurableStepFailed, "t2", &TaskResult{Success: false, Error: "boom"})

	runner := &countingRunner{}
	sr := NewStepRunner(runner)
	sr.SetDurableLog(l)
	plan := durableTestPlan("run-fail")
	sr.SetPlanID(plan.ID)

	if _, err := sr.RunPlan(context.Background(), plan); err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	// t1 replayed (1 exec), t2 re-executed because it failed, t3 executed once.
	if runner.CallCount() != 2 {
		t.Fatalf("steps executed = %d, want 2 (t2 retried + t3)", runner.CallCount())
	}
	if plan.Tasks[1].Status != TaskCompleted {
		t.Errorf("t2 should succeed on retry, got %s", plan.Tasks[1].Status)
	}
}

// TestRunPlan_WithoutDurable_BackwardCompat proves a nil durable log leaves
// behaviour unchanged: every task executes exactly once.
func TestRunPlan_WithoutDurable_BackwardCompat(t *testing.T) {
	runner := &countingRunner{}
	sr := NewStepRunner(runner) // no durable, no history — legacy path
	plan := durableTestPlan("run-legacy")
	sr.SetPlanID(plan.ID)

	res, err := sr.RunPlan(context.Background(), plan)
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if res.TasksDone != 3 || res.TasksFailed != 0 {
		t.Fatalf("result = %+v, want 3 done / 0 failed", res)
	}
	if runner.CallCount() != 3 {
		t.Fatalf("calls = %d, want 3", runner.CallCount())
	}

	// Re-running with a fresh plan and NO durable log re-executes everything
	// (no replay — that is the legacy behaviour).
	plan2 := durableTestPlan("run-legacy")
	if _, err := sr.RunPlan(context.Background(), plan2); err != nil {
		t.Fatalf("RunPlan #2: %v", err)
	}
	if runner.CallCount() != 6 {
		t.Fatalf("calls = %d, want 6 (no replay without durable)", runner.CallCount())
	}
}
