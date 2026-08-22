package pipeline

import (
	"context"
	"fmt"
	"time"
)

// Reconciler implements the Kubernetes/ArgoCD reconciliation pattern:
// desired state (Plan) vs actual state (event history) → continuous convergence.
//
// On every cycle, it compares the plan's task definitions against the event
// history, detects drift (tasks that should be completed but aren't, or that
// were completed but are now stale), and re-executes drifted tasks via the
// StepRunner until convergence or max cycles.
type Reconciler struct {
	stepRunner *StepRunner
	maxCycles  int
}

// ReconcileResult reports the outcome of a reconciliation cycle.
type ReconcileResult struct {
	PlanID      string
	Cycles      int
	TasksFixed  int
	TasksFailed int
	Converged   bool
	Drifts      []DriftAction
}

// DriftAction describes a detected gap between desired and actual state.
type DriftAction struct {
	TaskID      string
	Description string
	Reason      DriftReason
}

type DriftReason string

const (
	DriftMissing    DriftReason = "missing"    // Task in plan, never executed
	DriftFailed     DriftReason = "failed"     // Task executed but failed
	DriftIncomplete DriftReason = "incomplete" // Task partially executed (crash)
	DriftStale      DriftReason = "stale"      // Task completed but plan changed since
)

// NewReconciler creates a reconciler with the given StepRunner.
func NewReconciler(runner *StepRunner) *Reconciler {
	return &Reconciler{
		stepRunner: runner,
		maxCycles:  5, // prevent infinite loops
	}
}

// SetMaxCycles sets the maximum reconciliation cycles (default 5).
func (r *Reconciler) SetMaxCycles(n int) {
	if n > 0 {
		r.maxCycles = n
	}
}

// Reconcile runs the reconciliation loop until convergence or max cycles.
//
// Flow:
//  1. Load event history → actual state
//  2. Compare plan (desired) → actual state → detect drift
//  3. For each drifted task: re-execute via StepRunner
//  4. If any tasks were fixed, repeat from step 1
//  5. Return result when converged or max cycles reached
func (r *Reconciler) Reconcile(ctx context.Context, plan *Plan) (*ReconcileResult, error) {
	result := &ReconcileResult{
		PlanID: plan.ID,
	}

	for cycle := 1; cycle <= r.maxCycles; cycle++ {
		result.Cycles = cycle

		// 1. Detect drift: compare desired vs actual
		drifts := r.detectDrift(plan)
		if len(drifts) == 0 {
			result.Converged = true
			return result, nil
		}

		result.Drifts = append(result.Drifts, drifts...)

		// 2. Execute drifted tasks
		fixed := 0
		for _, drift := range drifts {
			task := r.findTask(plan, drift.TaskID)
			if task == nil {
				continue
			}

			taskResult, err := r.stepRunner.RunStep(ctx, plan.ID, task)
			if err != nil || (taskResult != nil && !taskResult.Success) {
				result.TasksFailed++
				task.Status = TaskFailed
				task.Result = taskResult
				continue
			}

			task.Status = TaskCompleted
			task.Result = taskResult
			fixed++
		}

		result.TasksFixed += fixed

		// 3. If nothing was fixed this cycle, we've reached max capability
		if fixed == 0 {
			result.Converged = false
			return result, nil
		}
	}

	result.Converged = false
	return result, fmt.Errorf("reconciliation did not converge after %d cycles", r.maxCycles)
}

// detectDrift compares the plan against the event history and identifies gaps.
// Implements Three-Tier Diff (ArgoCD Pattern #2):
//   Tier 1: Server-side diff — desired (plan) vs actual (history events)
//   Tier 2: Normalized diff — strip volatile fields (timestamps, durations)
//   Tier 3: Checkpoint-aware — if checkpoint exists, use as baseline
func (r *Reconciler) detectDrift(plan *Plan) []DriftAction {
	var drifts []DriftAction

	// Load actual state from history
	events, err := r.stepRunner.history.Load(plan.ID)

	// Build actual state: taskID → last known status
	actualState := make(map[string]TaskStatus)
	if err == nil {
		replayed := ReplayPlan(plan, events)
		for _, t := range replayed.Tasks {
			actualState[t.ID] = t.Status
		}
	}

	// ── Tier 2: Normalize — strip volatile fields for comparison ───
	// We normalize by comparing ONLY task status, not timestamps/durations.
	// This prevents false drift from fields that change every run.

	// ── Tier 3: Checkpoint-aware baseline ──────────────────────────
	// If a checkpoint exists, use it as the common ancestor for three-way diff.
	// This lets us detect: "task was completed in checkpoint, but now shows
	// as failed in history" → needs investigation.
	checkpointState := make(map[string]TaskStatus)
	if r.stepRunner.checkpoint != nil {
		cp, cpErr := r.stepRunner.checkpoint.Load(plan.ID)
		if cpErr == nil {
			for tid, status := range cp.TaskStatuses {
				checkpointState[tid] = TaskStatus(status)
			}
		}
	}

	// ── Compare desired vs actual ──────────────────────────────────
	for _, task := range plan.Tasks {
		actual, exists := actualState[task.ID]
		cpStatus, hasCP := checkpointState[task.ID]

		switch {
		case !exists:
			drifts = append(drifts, DriftAction{
				TaskID:      task.ID,
				Description: task.Description,
				Reason:      DriftMissing,
			})

		case actual == TaskFailed:
			drifts = append(drifts, DriftAction{
				TaskID:      task.ID,
				Description: task.Description,
				Reason:      DriftFailed,
			})

		case actual == TaskRunning:
			drifts = append(drifts, DriftAction{
				TaskID:      task.ID,
				Description: task.Description,
				Reason:      DriftIncomplete,
			})

		case actual == TaskPending:
			drifts = append(drifts, DriftAction{
				TaskID:      task.ID,
				Description: task.Description,
				Reason:      DriftMissing,
			})

		case actual == TaskCompleted:
			// ── Tier 3: Checkpoint-aware stale detection ──────────
			// If checkpoint says completed but history differs → investigate
			if hasCP && cpStatus != TaskCompleted && cpStatus != "" {
				// Checkpoint and history disagree — state divergence detected
				drifts = append(drifts, DriftAction{
					TaskID:      task.ID,
					Description: fmt.Sprintf("divergence: checkpoint=%s, actual=%s", cpStatus, actual),
					Reason:      DriftStale,
				})
			} else if r.isStale(task, actualState, events) {
				drifts = append(drifts, DriftAction{
					TaskID:      task.ID,
					Description: task.Description,
					Reason:      DriftStale,
				})
			}
			// Otherwise: converged for this task
		}
	}

	return drifts
}

// isStale checks if a completed task needs re-execution because the plan
// changed since it was last executed (stale result).
func (r *Reconciler) isStale(task *TaskNode, actualState map[string]TaskStatus, events []StepEvent) bool {
	if len(events) == 0 {
		return false
	}

	// Find the most recent PlanCreated event for this plan ID.
	var planCreatedAt time.Time
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == PlanCreated {
			planCreatedAt = events[i].Timestamp
			break
		}
	}

	if planCreatedAt.IsZero() {
		return false
	}

	// If the plan was created after the task's last event, it's stale.
	// (The plan was re-planned, which constitutes a spec change.)
	for _, ev := range events {
		if ev.TaskID == task.ID && ev.Type == StepEventCompleted {
			if ev.Timestamp.Before(planCreatedAt) {
				return true
			}
		}
	}

	return false
}

// ReconcileStalePlans scans the history directory for any plan with incomplete
// tasks (pending, running, failed) and runs reconciliation on each.
// Designed to be called on boot for auto-recovery.
func (r *Reconciler) ReconcileStalePlans(ctx context.Context) (int, error) {
	if r.stepRunner.history == nil {
		return 0, fmt.Errorf("reconciler: no history configured")
	}

	entries, err := r.listHistoryPlans()
	if err != nil {
		return 0, fmt.Errorf("reconciler: list plans: %w", err)
	}

	totalFixed := 0
	for _, planID := range entries {
		events, err := r.stepRunner.history.Load(planID)
		if err != nil || len(events) == 0 {
			continue
		}

		// Check if this plan has incomplete tasks
		plan := &Plan{ID: planID}
		replayed := ReplayPlan(plan, events)
		needsReconcile := false
		for _, t := range replayed.Tasks {
			if t.Status == TaskPending || t.Status == TaskRunning || t.Status == TaskFailed {
				needsReconcile = true
				break
			}
		}
		if !needsReconcile {
			continue
		}

		// Build a minimal plan from replayed state
		reconPlan := &Plan{
			ID:     planID,
			Tasks:  make([]*TaskNode, len(replayed.Tasks)),
			Status: replayed.Status,
		}
		for i, t := range replayed.Tasks {
			cp := *t
			reconPlan.Tasks[i] = &cp
		}

		r.stepRunner.SetPlanID(planID)
		recResult, recErr := r.Reconcile(ctx, reconPlan)
		if recErr == nil {
			totalFixed += recResult.TasksFixed
		}
	}

	return totalFixed, nil
}

// listHistoryPlans returns all plan IDs that have event history files.
func (r *Reconciler) listHistoryPlans() ([]string, error) {
	return r.stepRunner.history.ListPlans()
}

func (r *Reconciler) findTask(plan *Plan, taskID string) *TaskNode {
	for _, t := range plan.Tasks {
		if t.ID == taskID {
			return t
		}
	}
	return nil
}
