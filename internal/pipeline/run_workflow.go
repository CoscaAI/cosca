package pipeline

import (
	"context"

	"github.com/CoscaAI/cosca/internal/workflow"
)

// RunWorkflow executes a typed-routing workflow through the new
// internal/workflow engine (Google ADK-Go style value-based routing).
//
// It is a thin, additive bridge: the classic Planner/StepRunner DependsOn path
// is unchanged and remains the default for Plan execution. RunWorkflow exists
// so callers can compose and run value-routed graphs programmatically — nodes
// advertise a Route value ("ok", "retry", 1, true) and edges pick the next
// node from that value, the inverse of the task-ID dependency model.
func RunWorkflow(ctx context.Context, w *workflow.Workflow, initial map[string]any) (map[string]any, error) {
	return w.Run(ctx, initial)
}
