package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/CoscaAI/cosca/internal/pipeline"
)

// durablePromptRunID derives a stable run ID from an ad-hoc prompt. The planner
// otherwise generates a fresh PLAN-... id per invocation, which would fragment
// the event log and break resume. Keying the run off the prompt makes repeated
// invocations of the same prompt share one durable log: completed steps are
// replayed and an interrupted run resumes from its first incomplete task.
func durablePromptRunID(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return "PIPE-PROMPT-" + hex.EncodeToString(sum[:5])
}

// maybeResumeDurable inspects the durable event log for the plan's run ID.
//
//   - No events            → nothing to resume (fresh run).
//   - Terminal event       → run already finished; the idempotent replay in
//     RunPlan reuses cached results (re-running is a no-op).
//   - Events, no terminal  → stale run (crash/interrupt). With resume=true the
//     plan state is rebuilt from the log so execution continues from the first
//     incomplete task; without it a hint is printed (RunPlan still replays
//     completed steps, so no progress is lost either way).
//
// Returns true when a stale run was explicitly resumed.
func maybeResumeDurable(wiring *pipelineWiring, formatter *OutputFormatter, plan *pipeline.Plan, resume bool) (bool, error) {
	if wiring == nil || wiring.stepRunner == nil {
		return false, nil
	}
	l := wiring.stepRunner.DurableLog()
	if l == nil {
		return false, nil
	}

	events, err := l.LoadRun(plan.ID)
	if err != nil {
		return false, fmt.Errorf("durable: load run %s: %w", plan.ID, err)
	}
	if len(events) == 0 {
		return false, nil
	}
	if pipeline.IsTerminalEvent(events[len(events)-1].Type) {
		// Already finished. Re-running with the same run ID replays cached
		// results (idempotent) and appends nothing new.
		formatter.Verbose(fmt.Sprintf("Durable run %s already completed; replaying cached results.", plan.ID))
		return false, nil
	}

	done := 0
	for _, ev := range events {
		if ev.Type == pipeline.DurableStepCompleted && ev.Result.IsSuccess() {
			done++
		}
	}

	if !resume {
		formatter.Warning(fmt.Sprintf("Stale durable run detected: %s (%d/%d steps completed). Re-running will replay completed steps; use --resume to continue explicitly.", plan.ID, done, len(plan.Tasks)))
		return false, nil
	}

	formatter.Header("Durable Resume")
	formatter.KeyValue("Run ID", plan.ID)
	formatter.KeyValue("Completed Steps", fmt.Sprintf("%d/%d", done, len(plan.Tasks)))
	if err := wiring.stepRunner.ResumeDurable(plan.ID, plan); err != nil {
		return false, err
	}
	formatter.Success("Resumed from event log — continuing from the first incomplete task.")
	return true, nil
}

// noticeStaleRuns prints any interrupted durable runs for the terminal wiring
// (like the existing session resume) so the operator knows work can continue.
func noticeStaleRuns(formatter *OutputFormatter, stepRunner *pipeline.StepRunner) {
	if formatter == nil || stepRunner == nil {
		return
	}
	l := stepRunner.DurableLog()
	if l == nil {
		return
	}
	runs, err := l.ListRuns()
	if err != nil {
		return
	}
	for _, runID := range runs {
		stale, err := l.IsStale(runID)
		if err != nil || !stale {
			continue
		}
		events, _ := l.LoadRun(runID)
		done := 0
		for _, ev := range events {
			if ev.Type == pipeline.DurableStepCompleted && ev.Result.IsSuccess() {
				done++
			}
		}
		formatter.Warning(fmt.Sprintf("Interrupted durable run %q: %d step(s) completed. Re-run with the same run ID to resume (no progress lost).", runID, done))
	}
}
