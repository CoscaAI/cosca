package pipeline

import (
	"context"
	"testing"
	"time"
)

// countRunner returns a fixed successful result and counts invocations.
type countRunner struct {
	calls int
}

func (r *countRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	r.calls++
	return &RunResult{
		Response:    "ok",
		Agent:       req.Agent,
		BuildResult: &BuildResult{Success: true},
		TestResult:  &TestResult{Success: true},
	}, nil
}

func (r *countRunner) RunStream(ctx context.Context, req RunRequest) (<-chan RunEvent, error) {
	ch := make(chan RunEvent, 1)
	ch <- RunEvent{Type: EventDone}
	close(ch)
	return ch, nil
}

func TestNewReconcilerDefaults(t *testing.T) {
	r := NewReconciler(NewStepRunner(&countRunner{}))
	if r.maxCycles != 5 {
		t.Fatalf("maxCycles = %d, want 5", r.maxCycles)
	}
	r.SetMaxCycles(2)
	if r.maxCycles != 2 {
		t.Fatalf("maxCycles = %d, want 2", r.maxCycles)
	}
	r.SetMaxCycles(0) // ignored
	if r.maxCycles != 2 {
		t.Fatalf("SetMaxCycles(0) must be ignored, got %d", r.maxCycles)
	}
}

func TestReconcileConverged(t *testing.T) {
	h, _ := NewWorkflowHistory(t.TempDir())
	sr := NewStepRunner(&countRunner{})
	sr.history = h

	// Plan already fully completed in history → no drift → converged.
	plan := &Plan{ID: "p1", Tasks: []*TaskNode{{ID: "t1", Description: "task one"}}}
	_ = h.Append("p1", StepEvent{Type: StepEventCompleted, PlanID: "p1", TaskID: "t1", Timestamp: time.Now()})

	r := NewReconciler(sr)
	res, err := r.Reconcile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if !res.Converged {
		t.Fatalf("should converge: %+v", res)
	}
	if res.Cycles != 1 || len(res.Drifts) != 0 {
		t.Fatalf("result: %+v", res)
	}
}

func TestReconcileFixesMissingTasks(t *testing.T) {
	h, _ := NewWorkflowHistory(t.TempDir())
	runner := &countRunner{}
	sr := NewStepRunner(runner)
	sr.history = h

	// Task t1 has no history → drift (missing) → re-executed.
	plan := &Plan{ID: "p2", Tasks: []*TaskNode{{ID: "t1", Description: "run me"}}}
	r := NewReconciler(sr)
	res, err := r.Reconcile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if !res.Converged {
		t.Fatalf("should converge after fixing: %+v", res)
	}
	if res.TasksFixed != 1 {
		t.Fatalf("TasksFixed = %d, want 1", res.TasksFixed)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if plan.Tasks[0].Status != TaskCompleted {
		t.Fatalf("task status = %q", plan.Tasks[0].Status)
	}
}

func TestReconcileDetectDriftReasons(t *testing.T) {
	h, _ := NewWorkflowHistory(t.TempDir())
	sr := NewStepRunner(&countRunner{})
	sr.history = h
	now := time.Now()

	// t1: failed in history → DriftFailed. t2: no history → DriftMissing.
	_ = h.Append("p", StepEvent{Type: StepEventFailed, PlanID: "p", TaskID: "t1", Timestamp: now})
	plan := &Plan{ID: "p", Tasks: []*TaskNode{
		{ID: "t1", Description: "failed task"},
		{ID: "t2", Description: "never ran"},
	}}

	r := NewReconciler(sr)
	drifts := r.detectDrift(plan)
	if len(drifts) != 2 {
		t.Fatalf("drifts = %d, want 2 (%+v)", len(drifts), drifts)
	}
	byTask := map[string]DriftReason{}
	for _, d := range drifts {
		byTask[d.TaskID] = d.Reason
	}
	if byTask["t1"] != DriftFailed {
		t.Fatalf("t1 drift = %v", byTask["t1"])
	}
	if byTask["t2"] != DriftMissing {
		t.Fatalf("t2 drift = %v", byTask["t2"])
	}
}

func TestReconcileNoConvergence(t *testing.T) {
	// Runner that always fails → task never completes. The reconciler detects
	// the drift, fails the task, and returns early with Converged=false (no
	// error) because nothing was fixed in the cycle — the "did not converge
	// after N cycles" error only triggers when each cycle fixes ≥1 task yet
	// drift keeps re-appearing.
	fr := newFailingNRunner(1 << 30)
	h, _ := NewWorkflowHistory(t.TempDir())
	sr := NewStepRunner(fr)
	sr.history = h

	plan := &Plan{ID: "p3", Tasks: []*TaskNode{{ID: "t1", Description: "always fails"}}}
	r := NewReconciler(sr)
	r.SetMaxCycles(3)

	res, err := r.Reconcile(context.Background(), plan)
	if err != nil {
		t.Fatalf("expected early return without error, got %v", err)
	}
	if res.Converged {
		t.Fatalf("must not converge: %+v", res)
	}
	if res.TasksFailed != 1 {
		t.Fatalf("TasksFailed = %d, want 1", res.TasksFailed)
	}
	if plan.Tasks[0].Status != TaskFailed {
		t.Fatalf("task status = %q", plan.Tasks[0].Status)
	}
}

func TestReconcileStalePlansNoHistory(t *testing.T) {
	sr := NewStepRunner(&countRunner{})
	r := NewReconciler(sr)
	if _, err := r.ReconcileStalePlans(context.Background()); err == nil {
		t.Fatal("expected error when no history configured")
	}
}

func TestReconcileStalePlansSkipsComplete(t *testing.T) {
	h, _ := NewWorkflowHistory(t.TempDir())
	_ = h.Append("done", StepEvent{Type: StepEventCompleted, PlanID: "done", TaskID: "t1", Timestamp: time.Now()})
	sr := NewStepRunner(&countRunner{})
	sr.history = h

	r := NewReconciler(sr)
	// No Plan with the completed task can be rebuilt (ReplayPlan of a plan with
	// zero tasks → no incomplete tasks) → 0 fixed, no error.
	fixed, err := r.ReconcileStalePlans(context.Background())
	if err != nil {
		t.Fatalf("ReconcileStalePlans: %v", err)
	}
	if fixed != 0 {
		t.Fatalf("fixed = %d, want 0", fixed)
	}
}

func TestReconcileFindTask(t *testing.T) {
	r := NewReconciler(NewStepRunner(&countRunner{}))
	plan := &Plan{ID: "p", Tasks: []*TaskNode{{ID: "a"}, {ID: "b"}}}
	if got := r.findTask(plan, "b"); got == nil || got.ID != "b" {
		t.Fatalf("findTask(b) = %+v", got)
	}
	if got := r.findTask(plan, "zz"); got != nil {
		t.Fatalf("findTask(zz) = %+v", got)
	}
}

func TestReconcileIsStale(t *testing.T) {
	sr := NewStepRunner(&countRunner{})
	r := NewReconciler(sr)
	now := time.Now()

	// No events → not stale.
	if r.isStale(&TaskNode{ID: "t"}, nil, nil) {
		t.Fatal("no events must not be stale")
	}

	// Completed BEFORE plan re-created → stale.
	events := []StepEvent{
		{Type: StepEventCompleted, TaskID: "t", Timestamp: now.Add(-time.Hour)},
		{Type: PlanCreated, Timestamp: now},
	}
	if !r.isStale(&TaskNode{ID: "t"}, nil, events) {
		t.Fatal("completed before re-plan must be stale")
	}

	// Completed AFTER plan created → fresh.
	events2 := []StepEvent{
		{Type: PlanCreated, Timestamp: now.Add(-time.Hour)},
		{Type: StepEventCompleted, TaskID: "t", Timestamp: now},
	}
	if r.isStale(&TaskNode{ID: "t"}, nil, events2) {
		t.Fatal("completed after plan must be fresh")
	}
}
