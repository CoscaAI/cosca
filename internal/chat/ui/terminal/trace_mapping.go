package terminal

import (
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/trace"
)

// Canonical trace actions used by the operations tree.
const (
	ActionPlanCreated     = "PLAN_CREATED"
	ActionPlanCompleted   = "PLAN_COMPLETED"
	ActionPlanFailed      = "PLAN_FAILED"
	ActionTaskStarted     = "TASK_STARTED"
	ActionTaskCompleted   = "TASK_COMPLETED"
	ActionTaskFailed      = "TASK_FAILED"
	ActionTaskRetrying    = "TASK_RETRYING"
	ActionTaskSkipped     = "TASK_SKIPPED"
	ActionToolStarted     = "TOOL_STARTED"
	ActionToolResult      = "TOOL_RESULT"
	ActionBuildStarted    = "BUILD_STARTED"
	ActionBuildEnd        = "BUILD_END"
	ActionTestStarted     = "TEST_STARTED"
	ActionTestEnd         = "TEST_END"
	ActionExecutionDone   = "EXECUTION_DONE"
	ActionExecutionFailed = "EXECUTION_FAILED"
	ActionContent         = "CONTENT"
)

// MapPipelineEvent converts a pipeline run event into a trace.Event suitable
// for the operations tree. runEventType accepts both the pipeline.RunEventType
// string values ("tool_start", "build_start", "error", ...) and the
// high-level identifiers used by the workflow history ("PlanCreated",
// "StepStarted", "ToolStarted", ...).
func MapPipelineEvent(runEventType string, actor string, details string) trace.Event {
	action, result := mapRunEvent(runEventType)
	if actor == "" {
		actor = "kernel"
	}
	return trace.Event{
		TraceID:   trace.NewID().String(),
		Timestamp: time.Now().Unix(),
		Actor:     actor,
		Action:    action,
		Result:    result,
		Details:   details,
	}
}

// mapRunEvent resolves a run event type string to a canonical action and a
// tree status. Unknown types fall back to an upper-cased action with a
// heuristic result derived from the suffix.
func mapRunEvent(runEventType string) (action, result string) {
	switch strings.TrimSpace(runEventType) {
	case "content":
		return ActionContent, "running"
	case "tool_start", "ToolStarted", "tool_started":
		return ActionToolStarted, "running"
	case "tool_result":
		return ActionToolResult, "success"
	case "build_start", "BuildStarted", "build_started":
		return ActionBuildStarted, "running"
	case "build_end":
		return ActionBuildEnd, "success"
	case "test_start", "TestStarted", "test_started":
		return ActionTestStarted, "running"
	case "test_end":
		return ActionTestEnd, "success"
	case "error", "Error":
		return ActionExecutionFailed, "failed"
	case "done", "Done":
		return ActionExecutionDone, "success"
	case "plan_created", "PlanCreated":
		return ActionPlanCreated, "running"
	case "plan_completed", "PlanCompleted":
		return ActionPlanCompleted, "success"
	case "plan_failed", "PlanFailed":
		return ActionPlanFailed, "failed"
	case "step_started", "StepStarted", "task_started":
		return ActionTaskStarted, "running"
	case "step_completed", "StepCompleted":
		return ActionTaskCompleted, "success"
	case "step_failed", "StepFailed", "task_failed":
		return ActionTaskFailed, "failed"
	case "step_retrying":
		return ActionTaskRetrying, "running"
	case "step_skipped", "StepSkipped":
		return ActionTaskSkipped, "pending"
	default:
		lower := strings.ToLower(runEventType)
		switch {
		case strings.HasSuffix(lower, "_failed"), strings.HasSuffix(lower, "failed"),
			strings.HasSuffix(lower, "_error"), strings.HasSuffix(lower, "error"):
			return strings.ToUpper(runEventType), "failed"
		case strings.HasSuffix(lower, "_done"), strings.HasSuffix(lower, "done"),
			strings.HasSuffix(lower, "_end"), strings.HasSuffix(lower, "_completed"):
			return strings.ToUpper(runEventType), "success"
		case strings.HasSuffix(lower, "_started"), strings.HasSuffix(lower, "started"):
			return strings.ToUpper(runEventType), "running"
		default:
			return strings.ToUpper(runEventType), "pending"
		}
	}
}
