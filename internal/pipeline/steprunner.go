package pipeline

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/trace"
)

// StepRunner executes workflow steps as real pipeline tasks.
type StepRunner struct {
	runner       Runner
	history      *WorkflowHistory
	checkpoint   *CheckpointStore
	recoveryLoop *RecoveryLoop
	reconciler   *Reconciler
	plugins      *PluginRegistry
	planID       string

	// enableBuild / enableTest control whether each step runs project
	// build/test verification after the LLM response. Default: true
	// (legacy behaviour). The serve disables them so greetings and
	// conversations do not trigger a full go build+test per request.
	enableBuild bool
	enableTest  bool

	// durable is the opt-in append-only event log. nil disables durable
	// execution entirely (legacy behaviour). When set, completed steps are
	// replayed from the log instead of re-run (idempotent), and a crashed run
	// resumes from its first incomplete task.
	durable *DurableEventLog

	// traceStore optionally mirrors durable events into the universal flight
	// recorder so the desktop's execution tree shows the durable state.
	traceStore *trace.Store
	traceIDMu  sync.Mutex
	traceIDs   map[string]string // runID -> universal Trace ID
}

// NewStepRunner creates a StepRunner that delegates to a Runner.
func NewStepRunner(runner Runner) *StepRunner {
	return &StepRunner{runner: runner, enableBuild: true, enableTest: true}
}

// SetVerification toggles per-step build/test verification. Set to false to
// skip go build+go test after every step (recommended for chat/greeting paths).
func (s *StepRunner) SetVerification(build, test bool) {
	s.enableBuild = build
	s.enableTest = test
}

// StepRunnerOption configures a StepRunner at construction time.
type StepRunnerOption func(*StepRunner) error

// WithDurableLog enables durable execution backed by an append-only event log
// in dir (typically <project>/.cosca/durable-events/).
func WithDurableLog(dir string) StepRunnerOption {
	return func(s *StepRunner) error {
		return s.EnableDurableLog(dir)
	}
}

// NewStepRunnerWithOptions builds a StepRunner and applies functional options.
func NewStepRunnerWithOptions(runner Runner, opts ...StepRunnerOption) (*StepRunner, error) {
	s := NewStepRunner(runner)
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// SetDurableLog attaches an existing event log. Passing nil disables durable
// execution (backward compatible).
func (s *StepRunner) SetDurableLog(l *DurableEventLog) {
	s.durable = l
}

// EnableDurableLog constructs the event log at dir and attaches it.
func (s *StepRunner) EnableDurableLog(dir string) error {
	l, err := NewDurableEventLog(dir)
	if err != nil {
		return err
	}
	s.durable = l
	return nil
}

// SetTraceStore wires the optional flight recorder. Durable events appended
// after this point are mirrored as trace.Events (Actor="pipeline").
func (s *StepRunner) SetTraceStore(store *trace.Store) {
	s.traceStore = store
}

// DurableLog returns the attached event log, or nil when durable execution is
// disabled. Used by CLI wiring for stale-run detection and resumption.
func (s *StepRunner) DurableLog() *DurableEventLog {
	return s.durable
}

// NewDurableStepRunner creates a StepRunner with event sourcing and checkpointing.
// history records every step as an immutable event. checkpoint persists progress
// after each step. recovery handles automatic retry on failure.
func NewDurableStepRunner(runner Runner, history *WorkflowHistory, checkpoint *CheckpointStore, recovery *RecoveryLoop) *StepRunner {
	return &StepRunner{
		runner:       runner,
		history:      history,
		checkpoint:   checkpoint,
		recoveryLoop: recovery,
	}
}

// SetPlanID sets the plan context for event recording.
func (s *StepRunner) SetPlanID(id string) {
	s.planID = id
}

// SetPlugins configures the plugin registry for pre/post step hooks.
func (s *StepRunner) SetPlugins(reg *PluginRegistry) {
	s.plugins = reg
}

// RunStep executes a single workflow step by converting it to a pipeline task.
func (s *StepRunner) RunStep(ctx context.Context, stepName string, task *TaskNode) (*TaskResult, error) {
	if s.runner == nil {
		return nil, fmt.Errorf("steprunner: runner not configured")
	}
	planID := s.planID
	if planID == "" {
		planID = stepName
	}

	start := time.Now()

	s.recordEvent(planID, task, StepEventStarted, "", 0)

	// ── Plugin: pre_execute ───────────────────────────────────
	if s.plugins != nil {
		pc := &PluginContext{Plan: nil, Task: task, StepName: stepName}
		if err := s.plugins.Execute(ctx, ExtStepPreExecute, pc); err != nil {
			s.recordEvent(planID, task, StepEventFailed, err.Error(), time.Since(start).Milliseconds())
			return &TaskResult{Success: false, Error: err.Error()}, err
		}
	}

	req := RunRequest{
		Prompt:     task.Description,
		Agent:      task.Agent,
		IntentType: task.IntentType,
		Options: RunOptions{
			MaxTurns:    10,
			Timeout:     300 * time.Second,
			EnableBuild: s.enableBuild,
			EnableTest:  s.enableTest,
		},
	}

	result, err := s.runner.Run(ctx, req)
	dur := time.Since(start).Milliseconds()

	if err != nil {
		s.recordEvent(planID, task, StepEventFailed, err.Error(), dur)
		// Plugin: on_failure
		if s.plugins != nil {
			pc := &PluginContext{Task: task, StepName: stepName, Error: err}
			_ = s.plugins.Execute(ctx, ExtStepOnFailure, pc)
		}
		return &TaskResult{Success: false, Error: err.Error()}, err
	}

	tr := &TaskResult{
		Success:     true,
		Output:      result.Response,
		Agent:       result.Agent,
		TraceID:     result.TraceID,
		DurationMs:  dur,
		BuildResult: result.BuildResult,
		TestResult:  result.TestResult,
	}

	if result.BuildResult != nil && !result.BuildResult.Success {
		tr.Success = false
		tr.Error = fmt.Sprintf("build failed: %s", result.BuildResult.Output)
		tr.DurationMs = result.BuildResult.DurationMs
		s.recordEvent(planID, task, StepEventFailed, tr.Error, tr.DurationMs)
		return tr, nil
	}

	if result.TestResult != nil && !result.TestResult.Success {
		tr.Success = false
		tr.Error = fmt.Sprintf("tests failed: %d passed, %d failed", result.TestResult.Passed, result.TestResult.Failed)
		tr.DurationMs = result.TestResult.DurationMs
		s.recordEvent(planID, task, StepEventFailed, tr.Error, tr.DurationMs)
		return tr, nil
	}

	s.recordEvent(planID, task, StepEventCompleted, result.Response, dur)

	// ── Plugin: post_execute ──────────────────────────────────
	if s.plugins != nil {
		pc := &PluginContext{Task: task, StepName: stepName, Result: tr}
		if err := s.plugins.Execute(ctx, ExtStepPostExecute, pc); err != nil {
			tr.Success = false
			tr.Error = fmt.Sprintf("post-execute plugin: %v", err)
		}
	}

	return tr, nil
}

func (s *StepRunner) recordEvent(planID string, task *TaskNode, evType StepEventType, output string, durationMs int64) {
	if s.history == nil {
		return
	}

	event := StepEvent{
		Type:     evType,
		PlanID:   planID,
		TaskID:   task.ID,
		Agent:    task.Agent,
		Input:    task.Description,
		Output:   output,
		Duration: durationMs,
	}

	_ = s.history.Append(planID, event)
}

// RunPlanResult is the result of executing a full plan.
type RunPlanResult struct {
	PlanID      string
	TasksTotal  int
	TasksDone   int
	TasksFailed int
	Recovered   int // how many tasks were recovered from history
	DurationMs  int64
	Events      []StepEvent
}

// ProgressFunc is called after each step completes during RunPlan.
// Can be nil for fire-and-forget execution.
type ProgressFunc func(stepName string, status TaskStatus, done, total int)

// RunPlan executes a full plan with event sourcing, checkpoint, and crash recovery.
//
// Flow:
//  1. Check for existing history → if found, replay and resume (crash recovery)
//  2. For each pending task:
//     a. Record StepEventStarted
//     b. Execute via RunStep
//     c. On success: Record StepEventCompleted → save checkpoint
//     d. On failure: RecoveryLoop → retry → completed or StepEventFailed
//  3. Record PlanCompleted or PlanFailed
//
// The plan's task Status fields are updated in-place. Callers can inspect
// plan.Tasks[i].Status after RunPlan returns.
func (s *StepRunner) RunPlan(ctx context.Context, plan *Plan) (*RunPlanResult, error) {
	return s.RunPlanWithProgress(ctx, plan, nil)
}

// RunPlanWithQueue executes a plan through a keyed, rate-limited workqueue
// (TaskQueue) instead of the ad-hoc fan-out in RunPlanParallel. It applies
// dedup by task key, exponential backoff retry, and integrates the recovery
// loop's smart fix before each retry. Dependency order is preserved: dependents
// run once all their dependencies finish.
func (s *StepRunner) RunPlanWithQueue(ctx context.Context, plan *Plan, onProgress ProgressFunc) (*RunPlanResult, error) {
	q := NewTaskQueue(s, 0) // default worker count
	q.SetRecovery(s.recoveryLoop)
	return q.RunPlan(ctx, plan, onProgress)
}

// RunPlanWithProgress executes a full plan with optional progress callback.
func (s *StepRunner) RunPlanWithProgress(ctx context.Context, plan *Plan, onProgress ProgressFunc) (*RunPlanResult, error) {
	startTime := time.Now()
	planID := s.planID
	if planID == "" {
		planID = plan.ID
	}
	s.planID = planID

	// Validate the dependency graph before executing: dangling DependsOn
	// references silently drop tasks, so they must fail loudly.
	if err := plan.ValidateDependencies(); err != nil {
		return nil, err
	}

	result := &RunPlanResult{
		PlanID:     planID,
		TasksTotal: len(plan.Tasks),
	}

	// ── CRASH RECOVERY: check for prior history ──────────────────────
	if s.history != nil {
		recovered := s.tryResumeFromHistory(plan)
		result.Recovered = recovered
	}

	// ── DURABLE REPLAY: rebuild completed state from the event log ──
	// Idempotent: a step with a cached success result is not re-run. Failed
	// steps are left pending so RecoveryLoop retries them on this execution.
	if s.durable != nil {
		result.Recovered += s.replayDurableState(planID, plan)
	}

	// ── Record PlanCreated event ─────────────────────────────────────
	if s.history != nil {
		_ = s.history.Append(planID, StepEvent{
			Type:   PlanCreated,
			PlanID: planID,
			Input:  plan.Intent,
		})
	}

	// ── Durable: plan_created ────────────────────────────────────────
	s.appendDurable(planID, DurablePlanCreated, "", nil)

	// ── Plugin: pipeline.start ──────────────────────────────────────
	if s.plugins != nil {
		pc := &PluginContext{Plan: plan}
		_ = s.plugins.Execute(ctx, ExtPipelineStart, pc)
	}

	// ── Execute tasks in order ───────────────────────────────────────
	order := plan.ExecutionOrder()

	for _, task := range order {
		// Skip already-completed tasks (from crash recovery)
		if task.Status == TaskCompleted || task.Status == TaskSkipped {
			result.TasksDone++
			continue
		}

		// Durable idempotency: reuse a previously-completed result. This is
		// the core of durable execution — a completed step is never re-run.
		if s.durable != nil {
			if cached := s.durableCompleted(planID, task.ID); cached != nil {
				task.Status = TaskCompleted
				task.Result = cached
				result.TasksDone++
				result.Recovered++
				continue
			}
		}

		task.Status = TaskRunning
		s.appendDurable(planID, DurableStepStarted, task.ID, nil)

		// Execute step
		taskResult, runErr := s.RunStep(ctx, planID, task)

		if runErr != nil || (taskResult != nil && !taskResult.Success) {
			// ── Recovery Loop ────────────────────────────────────
			if s.recoveryLoop != nil {
				failureText := ""
				if taskResult != nil {
					failureText = taskResult.Error
				}
				if runErr != nil {
					failureText = runErr.Error()
				}

				recResult, recErr := s.recoveryLoop.Recover(ctx, task, failureText, s.runner)
				if recErr == nil && recResult.RetrySuccessful {
					task.Status = TaskCompleted
					if taskResult != nil {
						taskResult.Success = true
					}
					task.Result = taskResult
					result.TasksDone++
					s.saveCheckpoint(plan)
					s.appendDurable(planID, DurableStepCompleted, task.ID, taskResult)
					continue
				}
			}

			// Recovery failed or not available → task failed
			task.Status = TaskFailed
			task.Result = taskResult
			result.TasksFailed++
			s.appendDurable(planID, DurableStepFailed, task.ID, taskResult)
		} else {
			task.Status = TaskCompleted
			task.Result = taskResult
			result.TasksDone++
			s.appendDurable(planID, DurableStepCompleted, task.ID, taskResult)
		}

		// ── Checkpoint after every step ───────────────────────────
		s.saveCheckpoint(plan)

		// ── Progress callback ──────────────────────────────────
		if onProgress != nil {
			onProgress(task.ID, task.Status, result.TasksDone+result.TasksFailed, result.TasksTotal)
		}
	}

	// ── Reconciliation: converge desired vs actual state ──────────
	// After primary execution, run the reconciler to fix any remaining drift.
	// This handles: tasks that silently failed, stale results after re-plan,
	// and incomplete tasks from prior crash recovery.
	if s.history != nil && result.TasksFailed > 0 {
		reconciler := NewReconciler(s)
		reconciler.SetMaxCycles(3) // tight loop for post-execution convergence
		recResult, recErr := reconciler.Reconcile(ctx, plan)
		if recErr == nil && recResult.TasksFixed > 0 {
			result.TasksDone += recResult.TasksFixed
			result.TasksFailed -= recResult.TasksFixed
			if recResult.Converged {
				// All drifted tasks fixed — update to completed
				result.TasksDone = len(plan.Tasks) - result.TasksFailed
			}
		}
	}

	// ── Record final plan event ─────────────────────────────────────
	if s.history != nil {
		if result.TasksFailed > 0 {
			_ = s.history.Append(planID, StepEvent{
				Type:   PlanFailed,
				PlanID: planID,
				Output: fmt.Sprintf("%d/%d tasks failed", result.TasksFailed, result.TasksTotal),
			})
		} else {
			_ = s.history.Append(planID, StepEvent{
				Type:   PlanCompleted,
				PlanID: planID,
				Output: fmt.Sprintf("%d tasks completed", result.TasksDone),
			})
		}
	}

	// ── Durable terminal event ──────────────────────────────────────
	if s.durable != nil {
		if result.TasksFailed > 0 {
			s.appendDurable(planID, DurableRunFailed, "", nil)
		} else {
			s.appendDurable(planID, DurableRunCompleted, "", nil)
		}
	}

	result.DurationMs = time.Since(startTime).Milliseconds()

	// ── Plugin: pipeline.end ──────────────────────────────────────
	if s.plugins != nil {
		pc := &PluginContext{Plan: plan}
		_ = s.plugins.Execute(ctx, ExtPipelineEnd, pc)
	}

	// Load full event history for the result
	if s.history != nil {
		events, _ := s.history.Load(planID)
		result.Events = events
	}

	return result, nil
}

// tryResumeFromHistory checks for existing plan history and replays completed
// tasks. Returns the number of tasks recovered from prior execution.
func (s *StepRunner) tryResumeFromHistory(plan *Plan) int {
	events, err := s.history.Load(s.planID)
	if err != nil || len(events) == 0 {
		return 0
	}

	replayed := ReplayPlan(plan, events)

	// Apply replayed statuses to the live plan
	taskIdx := make(map[string]int)
	for i, t := range plan.Tasks {
		taskIdx[t.ID] = i
	}
	recovered := 0
	for _, rt := range replayed.Tasks {
		idx, ok := taskIdx[rt.ID]
		if !ok {
			continue
		}
		if rt.Status == TaskCompleted || rt.Status == TaskSkipped {
			plan.Tasks[idx].Status = rt.Status
			plan.Tasks[idx].Result = rt.Result
			recovered++
		} else if rt.Status == TaskFailed {
			plan.Tasks[idx].Status = TaskPending // retry failed tasks
		}
	}

	return recovered
}

// saveCheckpoint persists the current plan state after each step.
func (s *StepRunner) saveCheckpoint(plan *Plan) {
	if s.checkpoint == nil {
		return
	}

	seq, _ := s.history.LastSequence(s.planID)
	cp := Checkpoint{
		PlanID:       s.planID,
		LastSequence: seq,
		TaskStatuses: make(map[string]string, len(plan.Tasks)),
	}
	for _, t := range plan.Tasks {
		cp.TaskStatuses[t.ID] = string(t.Status)
	}
	_ = s.checkpoint.Save(cp)
}

// HasHistory returns true if there's an existing event history for this plan.
func (s *StepRunner) HasHistory() bool {
	if s.history == nil || s.planID == "" {
		return false
	}
	events, err := s.history.Load(s.planID)
	return err == nil && len(events) > 0
}

// ── Staged Execution (Temporal Pattern #2) ─────────────────────────────
//
// Decision (fast) and Execution (async) are separated.
// The caller gets a future (channel) immediately and can continue.
// Tasks execute in background goroutines, respecting DependsOn ordering.

// RunPlanAsync returns a channel that receives the result when execution
// completes. The decision phase (plan creation, event recording) runs
// synchronously; execution runs in a background goroutine.
//
// Usage:
//
//	future := runner.RunPlanAsync(ctx, plan)
//	// ... do other work ...
//	result := <-future
func (s *StepRunner) RunPlanAsync(ctx context.Context, plan *Plan) <-chan *RunPlanResult {
	ch := make(chan *RunPlanResult, 1)

	go func() {
		defer close(ch)
		result, err := s.RunPlan(ctx, plan)
		if err != nil {
			ch <- &RunPlanResult{
				PlanID:      plan.ID,
				TasksTotal:  len(plan.Tasks),
				TasksFailed: len(plan.Tasks),
			}
			return
		}
		ch <- result
	}()

	return ch
}

// RunPlanParallel executes independent tasks concurrently, respecting
// DependsOn ordering. Tasks with no mutual dependencies run in parallel
// (up to 5 concurrent workers).
func (s *StepRunner) RunPlanParallel(ctx context.Context, plan *Plan, onProgress ProgressFunc) (*RunPlanResult, error) {
	startTime := time.Now()
	planID := s.planID
	if planID == "" {
		planID = plan.ID
	}
	s.planID = planID

	// Validate the dependency graph before executing (see RunPlanWithProgress).
	if err := plan.ValidateDependencies(); err != nil {
		return nil, err
	}

	result := &RunPlanResult{
		PlanID:     planID,
		TasksTotal: len(plan.Tasks),
	}

	// ── CRASH RECOVERY ────────────────────────────────────────────
	if s.history != nil {
		result.Recovered = s.tryResumeFromHistory(plan)
	}

	// ── DURABLE REPLAY ────────────────────────────────────────────
	if s.durable != nil {
		result.Recovered += s.replayDurableState(planID, plan)
	}

	// ── Record plan start ────────────────────────────────────────
	if s.history != nil {
		_ = s.history.Append(planID, StepEvent{
			Type:   PlanCreated,
			PlanID: planID,
			Input:  plan.Intent,
		})
	}
	s.appendDurable(planID, DurablePlanCreated, "", nil)
	if s.plugins != nil {
		_ = s.plugins.Execute(ctx, ExtPipelineStart, &PluginContext{Plan: plan})
	}

	// ── CYCLE DETECTION ────────────────────────────────────────────
	if plan.HasCycle() {
		return nil, fmt.Errorf("plan contains a cycle; cannot execute in parallel")
	}

	// ── Parallel execution with dependency graph ──────────────────
	taskMap := make(map[string]*TaskNode)
	dependents := make(map[string][]string) // taskID → tasks that depend on it
	for _, t := range plan.Tasks {
		taskMap[t.ID] = t
		for _, dep := range t.DependsOn {
			dependents[dep] = append(dependents[dep], t.ID)
		}
	}

	var mu sync.Mutex
	done := make(map[string]bool)
	var wg sync.WaitGroup

	// Tasks that completed from history
	for _, t := range plan.Tasks {
		if t.Status == TaskCompleted || t.Status == TaskSkipped {
			done[t.ID] = true
			result.TasksDone++
		}
	}

	// Ready queue + worker semaphore (scaled to 75% of CPU cores, min 2)
	ready := make(chan *TaskNode, len(plan.Tasks))
	workers := max(int(float64(runtime.NumCPU())*0.75), 2)
	sem := make(chan struct{}, workers)

	// Enqueue initially-ready tasks
	for _, t := range plan.Tasks {
		if done[t.ID] {
			continue
		}
		if len(t.DependsOn) == 0 || s.allDepsSatisfied(t.DependsOn, done) {
			wg.Add(1)
			ready <- t
		}
	}

	// Worker loop
	go func() {
		for task := range ready {
			sem <- struct{}{}
			go func(t *TaskNode) {
				defer wg.Done()
				defer func() { <-sem }()

				if ctx.Err() != nil {
					mu.Lock()
					t.Status = TaskFailed
					result.TasksFailed++
					done[t.ID] = true
					s.saveCheckpoint(plan)
					for _, depID := range dependents[t.ID] {
						dep := taskMap[depID]
						if dep != nil && !done[dep.ID] && s.allDepsSatisfied(dep.DependsOn, done) {
							wg.Add(1)
							ready <- dep
						}
					}
					mu.Unlock()
					return
				}

				t.Status = TaskRunning
				s.appendDurable(planID, DurableStepStarted, t.ID, nil)
				taskResult, runErr := s.RunStep(ctx, planID, t)

				mu.Lock()
				if runErr != nil || (taskResult != nil && !taskResult.Success) {
					failureText := ""
					if taskResult != nil {
						failureText = taskResult.Error
					}
					if runErr != nil {
						failureText = runErr.Error()
					}
					recovered := false
					if s.recoveryLoop != nil {
						recResult, recErr := s.recoveryLoop.Recover(ctx, t, failureText, s.runner)
						if recErr == nil && recResult.RetrySuccessful {
							t.Status = TaskCompleted
							if taskResult != nil {
								taskResult.Success = true
							}
							t.Result = taskResult
							result.TasksDone++
							recovered = true
						}
					}
					if !recovered {
						t.Status = TaskFailed
						t.Result = taskResult
						result.TasksFailed++
						s.appendDurable(planID, DurableStepFailed, t.ID, taskResult)
					} else {
						s.appendDurable(planID, DurableStepCompleted, t.ID, taskResult)
					}
				} else {
					t.Status = TaskCompleted
					t.Result = taskResult
					result.TasksDone++
					s.appendDurable(planID, DurableStepCompleted, t.ID, taskResult)
				}
				done[t.ID] = true

				s.saveCheckpoint(plan)
				if onProgress != nil {
					onProgress(t.ID, t.Status, result.TasksDone+result.TasksFailed, result.TasksTotal)
				}

				// Enqueue newly-unblocked dependents
				for _, depID := range dependents[t.ID] {
					dep := taskMap[depID]
					if dep != nil && !done[dep.ID] && s.allDepsSatisfied(dep.DependsOn, done) {
						wg.Add(1)
						ready <- dep
					}
				}
				mu.Unlock()
			}(task)
		}
	}()

	wg.Wait()
	close(ready)

	// ── Finalize ──────────────────────────────────────────────────
	if s.history != nil {
		if result.TasksFailed > 0 {
			_ = s.history.Append(planID, StepEvent{
				Type: PlanFailed, PlanID: planID,
				Output: fmt.Sprintf("%d/%d failed", result.TasksFailed, result.TasksTotal),
			})
		} else {
			_ = s.history.Append(planID, StepEvent{
				Type: PlanCompleted, PlanID: planID,
				Output: fmt.Sprintf("%d completed", result.TasksDone),
			})
		}
	}
	if s.durable != nil {
		if result.TasksFailed > 0 {
			s.appendDurable(planID, DurableRunFailed, "", nil)
		} else {
			s.appendDurable(planID, DurableRunCompleted, "", nil)
		}
	}
	if s.plugins != nil {
		_ = s.plugins.Execute(ctx, ExtPipelineEnd, &PluginContext{Plan: plan})
	}

	result.DurationMs = time.Since(startTime).Milliseconds()
	if s.history != nil {
		result.Events, _ = s.history.Load(planID)
	}
	return result, nil
}

func (s *StepRunner) allDepsSatisfied(deps []string, done map[string]bool) bool {
	for _, d := range deps {
		if !done[d] {
			return false
		}
	}
	return true
}

// ── Durable execution helpers ────────────────────────────────────────────

// replayDurableState rebuilds the completed task state from the run's event
// log. Completed steps are marked done with their cached results; failed
// steps are reset to pending so RecoveryLoop can retry them. Returns the
// number of tasks recovered.
func (s *StepRunner) replayDurableState(runID string, plan *Plan) int {
	events, err := s.durable.LoadRun(runID)
	if err != nil || len(events) == 0 {
		return 0
	}
	replayed := ReplayDurable(plan, events)

	taskIdx := make(map[string]int, len(plan.Tasks))
	for i, t := range plan.Tasks {
		taskIdx[t.ID] = i
	}
	recovered := 0
	for _, rt := range replayed.Tasks {
		idx, ok := taskIdx[rt.ID]
		if !ok {
			continue
		}
		switch rt.Status {
		case TaskCompleted:
			plan.Tasks[idx].Status = TaskCompleted
			plan.Tasks[idx].Result = rt.Result
			recovered++
		case TaskFailed:
			// Failed steps are never cached — retry them on this execution.
			plan.Tasks[idx].Status = TaskPending
		case TaskSkipped:
			plan.Tasks[idx].Status = TaskSkipped
			plan.Tasks[idx].Result = rt.Result
			recovered++
		}
	}
	return recovered
}

// durableCompleted returns the cached success result for a task, or nil when
// the task has no durable step_completed event. Only successes are cached.
func (s *StepRunner) durableCompleted(runID, taskID string) *TaskResult {
	events, err := s.durable.LoadRun(runID)
	if err != nil {
		return nil
	}
	var cached *TaskResult
	for i := range events {
		ev := events[i]
		if ev.Type == DurableStepCompleted && ev.TaskID == taskID && ev.Result.IsSuccess() {
			cached = ev.Result
		}
	}
	if cached == nil {
		return nil
	}
	return cached.Clone()
}

// appendDurable records a durable event (if the log is enabled) and mirrors it
// into the universal flight recorder. plan_created and terminal events are only
// written once per run, keeping the log a clean replay stream.
func (s *StepRunner) appendDurable(runID, eventType, taskID string, result *TaskResult) {
	if s.durable == nil {
		return
	}
	if s.durable.hasEvent(runID, eventType) && (eventType == DurablePlanCreated || IsTerminalEvent(eventType)) {
		return
	}
	ev, err := s.durable.AppendWithTrace(runID, eventType, taskID, s.traceIDForRun(runID), result)
	if err != nil {
		return
	}
	s.emitDurableTrace(runID, eventType, ev)
}

// traceIDForRun returns the universal Trace ID for a run, reusing the one
// persisted in the event log across a resume so the execution tree stays whole.
func (s *StepRunner) traceIDForRun(runID string) string {
	s.traceIDMu.Lock()
	defer s.traceIDMu.Unlock()
	if s.traceIDs == nil {
		s.traceIDs = make(map[string]string)
	}
	if id, ok := s.traceIDs[runID]; ok {
		return id
	}
	if s.durable != nil {
		if events, err := s.durable.LoadRun(runID); err == nil {
			for _, ev := range events {
				if ev.TraceID != "" {
					s.traceIDs[runID] = ev.TraceID
					return ev.TraceID
				}
			}
		}
	}
	id := string(trace.NewID())
	s.traceIDs[runID] = id
	return id
}

// emitDurableTrace mirrors a durable event into the trace flight recorder
// (Actor="pipeline") so the desktop's execution tree shows the durable state.
func (s *StepRunner) emitDurableTrace(runID, eventType string, ev *DurableEvent) {
	if s.traceStore == nil || ev == nil {
		return
	}
	result := "running"
	if ev.Result != nil {
		if ev.Result.Success {
			result = "success"
		} else {
			result = "failed"
		}
	}
	if IsTerminalEvent(eventType) {
		if ev.Result == nil && result == "running" {
			result = "success"
		}
	}
	_ = s.traceStore.Append(trace.Event{
		TraceID: ev.TraceID,
		Actor:   "pipeline",
		Action:  strings.ToUpper(eventType),
		Result:  result,
		Details: ev.TaskID,
	})
}

// ResumeDurable rebuilds the plan state from the run's event log so execution
// continues from the first incomplete task. It is the explicit entry point for
// `--resume`: completed steps are marked done with cached results, pending
// steps stay pending, and a subsequent RunPlan only executes what remains.
func (s *StepRunner) ResumeDurable(runID string, plan *Plan) error {
	if s.durable == nil {
		return fmt.Errorf("durable: event log not configured")
	}
	events, err := s.durable.LoadRun(runID)
	if err != nil {
		return fmt.Errorf("durable: load run %s: %w", runID, err)
	}
	if len(events) == 0 {
		return nil
	}
	replayed := ReplayDurable(plan, events)
	taskIdx := make(map[string]int, len(plan.Tasks))
	for i, t := range plan.Tasks {
		taskIdx[t.ID] = i
	}
	for _, rt := range replayed.Tasks {
		idx, ok := taskIdx[rt.ID]
		if !ok {
			continue
		}
		switch rt.Status {
		case TaskCompleted:
			plan.Tasks[idx].Status = TaskCompleted
			plan.Tasks[idx].Result = rt.Result
		case TaskSkipped:
			plan.Tasks[idx].Status = TaskSkipped
			plan.Tasks[idx].Result = rt.Result
		case TaskFailed:
			plan.Tasks[idx].Status = TaskPending // retry failed steps
		}
	}
	s.planID = runID
	return nil
}
