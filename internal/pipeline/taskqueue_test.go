package pipeline

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// failingNRunner returns an error on the first n invocations, then succeeds.
type failingNRunner struct {
	n     atomic.Int64
	fails int64
}

func newFailingNRunner(fails int64) *failingNRunner {
	return &failingNRunner{fails: fails}
}

func (f *failingNRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if f.n.Add(1) <= f.fails {
		return nil, fmt.Errorf("simulated failure %d", f.n.Load())
	}
	return &RunResult{Response: "ok", Agent: req.Agent}, nil
}

func (f *failingNRunner) RunStream(ctx context.Context, req RunRequest) (<-chan RunEvent, error) {
	ch := make(chan RunEvent, 1)
	ch <- RunEvent{Type: EventDone}
	close(ch)
	return ch, nil
}

func TestTaskQueueExecutesSuccessfully(t *testing.T) {
	counter := &countingRunner{}

	sr := NewStepRunner(counter)
	q := NewTaskQueue(sr, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)
	defer q.Stop()

	task := &TaskNode{ID: "t1", Description: "build", Agent: "cosca-backend"}
	if err := q.Enqueue(ctx, "plan1", task); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if counter.CallCount() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := counter.CallCount(); got != 1 {
		t.Fatalf("expected 1 execution, got %d", got)
	}
}

func TestTaskQueueDeduplicates(t *testing.T) {
	counter := &countingRunner{}

	sr := NewStepRunner(counter)
	q := NewTaskQueue(sr, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Enqueue BEFORE starting workers: this guarantees all three Adds land in
	// the queue before any consumer can pick the key up. Dedup is then
	// deterministic — if the workers were already running, an Add() racing with
	// an in-flight Get() legitimately marks the key dirty and the item is
	// processed a second time (standard workqueue semantics), which used to
	// make this test flaky under full-suite load.
	task := &TaskNode{ID: "t1", Description: "build", Agent: "cosca-backend"}
	for i := 0; i < 3; i++ {
		if err := q.Enqueue(ctx, "plan1", task); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	q.Start(ctx)
	defer q.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if counter.CallCount() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	// Settle window: after the single execution, no re-execution may happen.
	time.Sleep(50 * time.Millisecond)
	if got := counter.CallCount(); got != 1 {
		t.Fatalf("expected exactly 1 execution after dedup, got %d", got)
	}
}

func TestTaskQueueRetriesThenSucceeds(t *testing.T) {
	// Fails the first 2 attempts, succeeds on the 3rd.
	fr := newFailingNRunner(2)
	sr := NewStepRunner(fr)
	q := NewTaskQueue(sr, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)
	defer q.Stop()

	task := &TaskNode{ID: "t1", Description: "build", Agent: "cosca-backend"}
	if err := q.Enqueue(ctx, "plan1", task); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if fr.n.Load() >= 3 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	// 2 failures + 1 success = 3 attempts, then Forget (no more retries).
	if got := fr.n.Load(); got != 3 {
		t.Fatalf("expected 3 attempts (2 fail + 1 success), got %d", got)
	}
}

func TestTaskQueueGivesUpAfterMaxRetries(t *testing.T) {
	// Always fails.
	fr := newFailingNRunner(1 << 30)
	sr := NewStepRunner(fr)
	q := NewTaskQueue(sr, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx)
	defer q.Stop()

	task := &TaskNode{ID: "t1", Description: "build", Agent: "cosca-backend"}
	if err := q.Enqueue(ctx, "plan1", task); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// maxRetries=3 → 1 initial + 3 retries = 4 attempts, then give up.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if fr.n.Load() >= 4 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := fr.n.Load(); got != 4 {
		t.Fatalf("expected 4 attempts (1 + 3 retries) then give up, got %d", got)
	}

	// After giving up, no more attempts happen.
	time.Sleep(100 * time.Millisecond)
	if got := fr.n.Load(); got != 4 {
		t.Fatalf("expected no further attempts after giving up, got %d", got)
	}
}

// orderRunner records the order in which tasks execute.
type orderRunner struct {
	mu    sync.Mutex
	order []string
}

func (o *orderRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	o.mu.Lock()
	o.order = append(o.order, req.Prompt)
	o.mu.Unlock()
	return &RunResult{
		Response:    "ok: " + req.Prompt,
		Agent:       req.Agent,
		BuildResult: &BuildResult{Success: true, Output: "build ok"},
		TestResult:  &TestResult{Success: true, Passed: 1, Failed: 0},
	}, nil
}

func (o *orderRunner) RunStream(ctx context.Context, req RunRequest) (<-chan RunEvent, error) {
	ch := make(chan RunEvent, 1)
	ch <- RunEvent{Type: EventDone}
	close(ch)
	return ch, nil
}

func (o *orderRunner) Order() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string{}, o.order...)
}

func TestRunPlanDependencyOrder(t *testing.T) {
	or := &orderRunner{}
	sr := NewStepRunner(or)
	q := NewTaskQueue(sr, 3)

	plan := &Plan{
		ID: "p1",
		Tasks: []*TaskNode{
			{ID: "t1", Description: "t1", Status: TaskPending},
			{ID: "t2", Description: "t2", DependsOn: []string{"t1"}, Status: TaskPending},
			{ID: "t3", Description: "t3", DependsOn: []string{"t2"}, Status: TaskPending},
		},
	}

	result, err := q.RunPlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if result.TasksDone != 3 || result.TasksFailed != 0 {
		t.Fatalf("expected 3 done / 0 failed, got %d/%d", result.TasksDone, result.TasksFailed)
	}

	got := or.Order()
	want := []string{"t1", "t2", "t3"}
	if len(got) != len(want) {
		t.Fatalf("expected order %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dependency order violated: expected %v, got %v", want, got)
		}
	}
}

func TestRunPlanIndependent(t *testing.T) {
	or := &orderRunner{}
	sr := NewStepRunner(or)
	q := NewTaskQueue(sr, 3)

	plan := &Plan{
		ID: "p1",
		Tasks: []*TaskNode{
			{ID: "t1", Description: "t1", Status: TaskPending},
			{ID: "t2", Description: "t2", Status: TaskPending},
			{ID: "t3", Description: "t3", Status: TaskPending},
		},
	}

	result, err := q.RunPlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if result.TasksDone != 3 {
		t.Fatalf("expected 3 done, got %d", result.TasksDone)
	}
}

func TestRunPlanRetriesFailingTask(t *testing.T) {
	fr := newFailingNRunner(2) // fails first 2 attempts, succeeds on 3rd
	sr := NewStepRunner(fr)
	q := NewTaskQueue(sr, 1)

	plan := &Plan{
		ID:    "p1",
		Tasks: []*TaskNode{{ID: "t1", Description: "t1", Status: TaskPending}},
	}

	result, err := q.RunPlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if result.TasksFailed != 0 {
		t.Fatalf("expected success after retries, got %d failed", result.TasksFailed)
	}
	if got := fr.n.Load(); got != 3 {
		t.Fatalf("expected 3 attempts (2 fail + 1 success), got %d", got)
	}
}

func TestRunPlanSkipsCompleted(t *testing.T) {
	or := &orderRunner{}
	sr := NewStepRunner(or)
	q := NewTaskQueue(sr, 3)

	// t1 is pre-completed (durable resume); only t2 and t3 should run.
	plan := &Plan{
		ID: "p1",
		Tasks: []*TaskNode{
			{ID: "t1", Description: "t1", Status: TaskCompleted},
			{ID: "t2", Description: "t2", DependsOn: []string{"t1"}, Status: TaskPending},
			{ID: "t3", Description: "t3", DependsOn: []string{"t2"}, Status: TaskPending},
		},
	}

	result, err := q.RunPlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if result.TasksDone != 3 {
		t.Fatalf("expected 3 done (1 skipped + 2 executed), got %d", result.TasksDone)
	}

	got := or.Order()
	if len(got) != 2 || got[0] != "t2" || got[1] != "t3" {
		t.Fatalf("expected only t2,t3 to execute in order, got %v", got)
	}
}
