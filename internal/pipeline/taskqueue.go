package pipeline

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// TaskQueue is a durable, worker-pool-backed task execution queue.
// Tasks are persisted as events in the WorkflowHistory and executed by a
// configurable number of worker goroutines.
//
// Kubernetes Pattern (client-go util/workqueue): the queue is keyed by
// "planID/taskID" (not the task object), deduplicating concurrent enqueues and
// applying per-item exponential backoff on retry — success Forget()s, failure
// AddRateLimited()s, and exceeding maxRetries gives up with a terminal event.
// The WorkflowHistory remains the durable source of truth; the workqueue is the
// in-memory scheduling layer on top of it.
type TaskQueue struct {
	stepRunner *StepRunner
	workers    int
	maxRetries int
	queue      *Workqueue
	recovery   *RecoveryLoop

	mu    sync.Mutex
	tasks map[string]*queuedTask // key -> task object

	startOnce sync.Once
	stopOnce  sync.Once
}

type queuedTask struct {
	PlanID string
	Task   *TaskNode
}

// taskKey builds the dedup key for a plan+task. Uses a NUL separator so keys
// cannot collide across plan/task IDs.
func taskKey(planID, taskID string) string {
	return planID + "\x00" + taskID
}

// NewTaskQueue creates a task queue with the given StepRunner and worker count.
func NewTaskQueue(runner *StepRunner, workers int) *TaskQueue {
	if workers <= 0 {
		workers = 3
	}
	// 5ms base, 10s cap — the same policy as the Kubernetes controller default.
	limiter := NewItemExponentialFailureRateLimiter(5*time.Millisecond, 10*time.Second)
	return &TaskQueue{
		stepRunner: runner,
		workers:    workers,
		maxRetries: 3,
		queue:      NewWorkqueue(limiter),
		tasks:      map[string]*queuedTask{},
	}
}

// SetRecovery wires the RecoveryLoop used to attempt a smart fix before a task
// is retried with backoff. May be nil (retry without a fix attempt).
func (q *TaskQueue) SetRecovery(r *RecoveryLoop) {
	q.recovery = r
}

// Start launches the worker pool. Call before enqueuing tasks. Safe to call
// more than once (only the first call starts workers).
func (q *TaskQueue) Start(ctx context.Context) {
	q.startOnce.Do(func() {
		for i := 0; i < q.workers; i++ {
			go q.worker(ctx, i)
		}
	})
}

// Stop gracefully shuts down the worker pool. Safe to call more than once.
func (q *TaskQueue) Stop() {
	q.stopOnce.Do(func() {
		q.queue.ShutDown()
	})
}

// Enqueue adds a task to the queue (deduplicated by plan/task) and persists it
// as a StepEventStarted. Returns immediately; the task is executed by a worker.
func (q *TaskQueue) Enqueue(ctx context.Context, planID string, task *TaskNode) error {
	key := taskKey(planID, task.ID)

	// Persist as event (durable — survives crash).
	if q.stepRunner.history != nil {
		_ = q.stepRunner.history.Append(planID, StepEvent{
			Type:   StepEventStarted,
			PlanID: planID,
			TaskID: task.ID,
			Agent:  task.Agent,
			Input:  task.Description,
		})
	}

	// Store the object for key lookup, then enqueue the key.
	q.mu.Lock()
	q.tasks[key] = &queuedTask{PlanID: planID, Task: task}
	q.mu.Unlock()

	q.queue.Add(key)
	return nil
}

// EnqueuePlan enqueues all tasks of a plan respecting DependsOn ordering.
// Independent tasks are enqueued immediately; dependent tasks are enqueued
// when their dependencies complete.
func (q *TaskQueue) EnqueuePlan(ctx context.Context, plan *Plan) error {
	planID := q.stepRunner.planID
	if planID == "" {
		planID = plan.ID
	}
	q.stepRunner.SetPlanID(planID)

	if q.stepRunner.history != nil {
		_ = q.stepRunner.history.Append(planID, StepEvent{
			Type:   PlanCreated,
			PlanID: planID,
			Input:  plan.Intent,
		})
	}

	for _, t := range plan.Tasks {
		if len(t.DependsOn) == 0 {
			if err := q.Enqueue(ctx, planID, t); err != nil {
				return err
			}
		}
	}
	return nil
}

// RecoverPending reloads tasks from history that were started but not completed.
// Call this on boot to resume interrupted work.
func (q *TaskQueue) RecoverPending(ctx context.Context) (int, error) {
	if q.stepRunner.history == nil {
		return 0, nil
	}

	plans, err := q.stepRunner.history.ListPlans()
	if err != nil {
		return 0, fmt.Errorf("taskqueue: list plans: %w", err)
	}

	recovered := 0
	for _, planID := range plans {
		events, err := q.stepRunner.history.Load(planID)
		if err != nil || len(events) == 0 {
			continue
		}

		started := map[string]bool{}
		finished := map[string]bool{}
		for _, ev := range events {
			switch ev.Type {
			case StepEventStarted:
				started[ev.TaskID] = true
			case StepEventCompleted, StepEventFailed:
				finished[ev.TaskID] = true
			}
		}

		for taskID := range started {
			if !finished[taskID] {
				task := &TaskNode{
					ID:          taskID,
					Description: fmt.Sprintf("recovered task %s from plan %s", taskID, planID),
					Status:      TaskPending,
				}
				if err := q.Enqueue(ctx, planID, task); err != nil {
					continue
				}
				recovered++
			}
		}
	}
	return recovered, nil
}

// worker is a single consumer loop: Get → execute → Done, with retry backoff.
func (q *TaskQueue) worker(ctx context.Context, id int) {
	for q.processNext(ctx) {
	}
}

// processNext processes one item. Returns false when the queue is shutting down
// or the context is cancelled.
func (q *TaskQueue) processNext(ctx context.Context) bool {
	key, shutdown := q.queue.Get()
	if shutdown {
		return false
	}
	defer q.queue.Done(key)

	select {
	case <-ctx.Done():
		return false
	default:
	}

	q.mu.Lock()
	qt := q.tasks[key]
	q.mu.Unlock()
	if qt == nil {
		// Key without a task (should not happen) — drop and move on.
		q.queue.Forget(key)
		return true
	}

	err := q.executeTask(ctx, qt)
	if err != nil {
		q.handleErr(key, qt, err)
	} else {
		q.queue.Forget(key)
	}
	return true
}

// executeTask runs the task via the StepRunner and returns a non-nil error on
// failure. The StepRunner records the per-attempt StepEvent* history.
func (q *TaskQueue) executeTask(ctx context.Context, qt *queuedTask) error {
	result, err := q.stepRunner.RunStep(ctx, qt.PlanID, qt.Task)
	if err != nil {
		return err
	}
	if result != nil && !result.Success {
		if result.Error != "" {
			return fmt.Errorf("%s", result.Error)
		}
		return fmt.Errorf("task %s failed", qt.Task.ID)
	}
	return nil
}

// handleErr applies the Kubernetes handleErr policy: within the retry budget the
// task is rate-limited (exponential backoff) and retried; beyond it, a terminal
// StepEventFailed is recorded and the task is dropped.
func (q *TaskQueue) handleErr(key string, qt *queuedTask, err error) {
	if q.queue.NumRequeues(key) < q.maxRetries {
		// Retry with backoff. The item is currently in-flight; AddRateLimited
		// schedules a delayed re-add that the deferred Done() releases.
		q.queue.AddRateLimited(key)
		return
	}

	// Exceeded retry budget — terminal failure.
	q.queue.Forget(key)
	if q.stepRunner.history != nil {
		_ = q.stepRunner.history.Append(qt.PlanID, StepEvent{
			Type:     StepEventFailed,
			PlanID:   qt.PlanID,
			TaskID:   qt.Task.ID,
			Agent:    qt.Task.Agent,
			Output:   err.Error(),
			Duration: 0,
		})
	}
}

// ─── Plan execution (DAG orchestration via the workqueue) ───────────────────

// RunPlan executes every task of plan in dependency order through a workqueue:
// independent tasks run concurrently, dependents are enqueued once all their
// dependencies finish, and a failed task is first fixed via the recovery loop
// and then retried with exponential backoff (rate limiter) up to maxRetries.
//
// This is the workqueue-based alternative to StepRunner.RunPlanParallel: it
// replaces the ad-hoc ready-channel fan-out and inline retry with a keyed,
// deduplicating, rate-limited queue. It is self-contained (own workqueue and
// worker goroutines), so it is safe to call concurrently.
func (q *TaskQueue) RunPlan(ctx context.Context, plan *Plan, onProgress ProgressFunc) (*RunPlanResult, error) {
	if err := plan.ValidateDependencies(); err != nil {
		return nil, err
	}
	planID := q.stepRunner.planID
	if planID == "" {
		planID = plan.ID
	}
	q.stepRunner.SetPlanID(planID)
	start := time.Now()

	if q.stepRunner.history != nil {
		_ = q.stepRunner.history.Append(planID, StepEvent{
			Type:   PlanCreated,
			PlanID: planID,
			Input:  plan.Intent,
		})
	}

	// Build the dependency graph.
	taskMap := map[string]*TaskNode{}
	dependents := map[string][]string{}
	remaining := map[string]int{}
	for _, t := range plan.Tasks {
		taskMap[t.ID] = t
		remaining[t.ID] = len(t.DependsOn)
		for _, dep := range t.DependsOn {
			dependents[dep] = append(dependents[dep], t.ID)
		}
	}

	// Plan-scoped workqueue so RunPlan never touches the shared q.queue (which
	// serves the async Enqueue path) and stays safe under concurrent calls.
	wq := NewWorkqueue(NewItemExponentialFailureRateLimiter(5*time.Millisecond, 10*time.Second))

	var (
		tasksDone   atomic.Int64
		tasksFailed atomic.Int64
	)

	// Completion tracker guarded by mu; dependents are enqueued when their last
	// dependency finishes (success or failure — matching RunPlanParallel).
	var mu sync.Mutex
	finished := map[string]bool{}
	pending := len(plan.Tasks)
	doneCh := make(chan struct{})

	// Pre-mark tasks already completed (from durable --resume) so they are not
	// re-executed, and unblock any dependents whose last dependency they were.
	for _, t := range plan.Tasks {
		if t.Status == TaskCompleted || t.Status == TaskSkipped {
			finished[t.ID] = true
			pending--
			tasksDone.Add(1)
			for _, depID := range dependents[t.ID] {
				remaining[depID]--
				if remaining[depID] == 0 {
					wq.Add(taskKey(planID, depID))
				}
			}
		}
	}

	// Enqueue tasks with no dependencies (that are not already completed).
	for _, t := range plan.Tasks {
		if finished[t.ID] {
			continue
		}
		if len(t.DependsOn) == 0 {
			wq.Add(taskKey(planID, t.ID))
		}
	}

	finish := func(taskID string, ok bool) {
		mu.Lock()
		if finished[taskID] {
			mu.Unlock()
			return
		}
		finished[taskID] = true
		pending--
		if ok {
			tasksDone.Add(1)
		} else {
			tasksFailed.Add(1)
		}
		for _, depID := range dependents[taskID] {
			remaining[depID]--
			if remaining[depID] == 0 {
				wq.Add(taskKey(planID, depID))
			}
		}
		if pending == 0 {
			close(doneCh)
		}
		mu.Unlock()
	}

	workerCount := q.workers
	if workerCount > len(plan.Tasks) {
		workerCount = len(plan.Tasks)
	}
	if workerCount < 1 {
		workerCount = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q.planWorker(ctx, planID, taskMap, finish, onProgress, &tasksDone, &tasksFailed, len(plan.Tasks), wq)
		}()
	}

	// Wait for completion or cancellation.
	select {
	case <-doneCh:
	case <-ctx.Done():
	}

	// Shut the plan queue down so workers drain and exit, then join.
	wq.ShutDown()
	wg.Wait()

	result := &RunPlanResult{
		PlanID:      planID,
		TasksTotal:  len(plan.Tasks),
		TasksDone:   int(tasksDone.Load()),
		TasksFailed: int(tasksFailed.Load()),
		DurationMs:  time.Since(start).Milliseconds(),
	}

	if q.stepRunner.history != nil {
		if result.TasksFailed > 0 {
			_ = q.stepRunner.history.Append(planID, StepEvent{
				Type:   PlanFailed,
				PlanID: planID,
				Output: fmt.Sprintf("%d/%d tasks failed", result.TasksFailed, result.TasksTotal),
			})
		} else {
			_ = q.stepRunner.history.Append(planID, StepEvent{
				Type:   PlanCompleted,
				PlanID: planID,
				Output: fmt.Sprintf("%d tasks completed", result.TasksDone),
			})
		}
	}

	return result, nil
}

// planWorker is a single consumer goroutine for RunPlan: Get → execute →
// recover/retry → finish.
func (q *TaskQueue) planWorker(
	ctx context.Context,
	planID string,
	taskMap map[string]*TaskNode,
	finish func(taskID string, ok bool),
	onProgress ProgressFunc,
	tasksDone, tasksFailed *atomic.Int64,
	total int,
	wq *Workqueue,
) {
	for {
		if ctx.Err() != nil {
			return
		}
		key, shutdown := wq.Get()
		if shutdown {
			return
		}

		taskID := taskIDFromKey(key, planID)
		task := taskMap[taskID]
		if task == nil {
			wq.Forget(key)
			wq.Done(key)
			continue
		}

		doneTotal := int(tasksDone.Load() + tasksFailed.Load())
		if onProgress != nil {
			onProgress(taskID, TaskRunning, doneTotal, total)
		}

		runResult, err := q.stepRunner.RunStep(ctx, planID, task)
		if err == nil && runResult != nil && runResult.Success {
			wq.Forget(key)
			wq.Done(key)
			finish(taskID, true)
			if onProgress != nil {
				onProgress(taskID, TaskCompleted, int(tasksDone.Load()+tasksFailed.Load()), total)
			}
			continue
		}

		// Failure: attempt the recovery loop's smart fix before retrying.
		recovered := false
		if q.recovery != nil {
			failureText := ""
			if runResult != nil {
				failureText = runResult.Error
			}
			if err != nil {
				failureText = err.Error()
			}
			rec, recErr := q.recovery.Recover(ctx, task, failureText, q.stepRunner.runner)
			if recErr == nil && rec.RetrySuccessful {
				recovered = true
			}
		}

		if recovered {
			wq.Forget(key)
			wq.Done(key)
			finish(taskID, true)
			if onProgress != nil {
				onProgress(taskID, TaskCompleted, int(tasksDone.Load()+tasksFailed.Load()), total)
			}
			continue
		}

		// Not recovered — rate-limited retry, or terminal failure.
		if wq.NumRequeues(key) < q.maxRetries {
			wq.AddRateLimited(key)
			wq.Done(key)
			continue
		}

		wq.Forget(key)
		wq.Done(key)
		finish(taskID, false)
		if onProgress != nil {
			onProgress(taskID, TaskFailed, int(tasksDone.Load()+tasksFailed.Load()), total)
		}
	}
}

// taskIDFromKey extracts the taskID from a "planID\x00taskID" key.
func taskIDFromKey(key, planID string) string {
	prefix := planID + "\x00"
	if len(key) > len(prefix) && key[:len(prefix)] == prefix {
		return key[len(prefix):]
	}
	return key
}
