package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/CoscaAI/cosca/api/stream"
	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/workflows"
)

// WorkflowsHandler handles REST API requests for the Workflows Manager.
type WorkflowsHandler struct {
	mgr        *workflows.Manager
	auditStore *audit.Store
	hub        *stream.Hub
}

// NewWorkflowsHandler creates a new WorkflowsHandler.
// If mgr is nil, the handler returns empty results gracefully.
func NewWorkflowsHandler(mgr *workflows.Manager, auditStore *audit.Store) *WorkflowsHandler {
	return &WorkflowsHandler{mgr: mgr, auditStore: auditStore}
}

// SetHub sets the WebSocket Hub for broadcasting real-time events.
// May be nil if WebSocket is disabled.
func (h *WorkflowsHandler) SetHub(hub *stream.Hub) {
	h.hub = hub
}

// List handles GET /v1/workflows
func (h *WorkflowsHandler) List(w http.ResponseWriter, _ *http.Request) {
	if h.mgr == nil {
		writeJSON(w, http.StatusOK, []workflows.Workflow{})
		return
	}
	items := h.mgr.List()
	writeJSON(w, http.StatusOK, items)
}

// Search handles GET /v1/workflows/search?q=...
func (h *WorkflowsHandler) Search(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeJSON(w, http.StatusOK, []workflows.Workflow{})
		return
	}
	query := getQueryParam(r, "q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}
	items, err := h.mgr.Search(query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []workflows.Workflow{}
	}
	writeJSON(w, http.StatusOK, items)
}

// Get handles GET /v1/workflows/{name}
func (h *WorkflowsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeError(w, http.StatusServiceUnavailable, "workflows manager not available")
		return
	}
	name := r.PathValue("name")
	item, err := h.mgr.Get(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// Run handles POST /v1/workflows/{name}/run
func (h *WorkflowsHandler) Run(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeError(w, http.StatusServiceUnavailable, "workflows manager not available")
		return
	}
	name := r.PathValue("name")
	result, err := h.mgr.Run(r.Context(), name)
	if err != nil {
		LogEvent(h.auditStore, r, "workflow.run", "workflow:"+name,
			audit.DetailsJSON(map[string]string{"error": err.Error()}), "error")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	LogEvent(h.auditStore, r, "workflow.run", "workflow:"+name, "{}", "success")
	writeJSON(w, http.StatusOK, result)
}

// RunStream handles POST /v1/workflows/{name}/run/stream — executes a
// workflow and streams step-level progress back as SSE events.
//
// Events emitted:
//   - step: StepProgress (started/completed/failed per step)
//   - done: final Result (on success)
//   - error: error message (on failure)
func (h *WorkflowsHandler) RunStream(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeError(w, http.StatusServiceUnavailable, "workflows manager not available")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "workflow name is required")
		return
	}

	// Parse optional input from body JSON.
	var input map[string]interface{}
	if r.Body != nil && r.ContentLength > 0 {
		limitBody(w, r, bodyLimitMedium)
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			// Empty body or non-JSON is fine — use nil input.
			input = nil
		}
	}
	if input == nil {
		input = make(map[string]interface{})
	}

	// Set up SSE writer.
	sw, err := stream.NewSSEWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	defer sw.Close()

	// Run workflow with progress reporting.
	result, runErr := h.mgr.RunWithProgress(r.Context(), name, input, func(p workflows.StepProgress) {
		// Respect client disconnection.
		select {
		case <-r.Context().Done():
			return
		default:
		}
		_ = sw.WriteEvent("step", p)
		// Forward step progress to WebSocket subscribers.
		if h.hub != nil {
			h.hub.BroadcastEvent([]string{"workflow"}, "step_"+p.Status, map[string]interface{}{
				"workflow":  name,
				"step_name": p.StepName,
				"status":    p.Status,
				"output":    p.Output,
				"step_num":  p.StepNum,
				"total":     p.TotalSteps,
			})
		}
	})

	if runErr != nil {
		LogEvent(h.auditStore, r, "workflow.run.stream", "workflow:"+name,
			audit.DetailsJSON(map[string]string{"error": runErr.Error()}), "error")
		// Broadcast workflow failed to WebSocket subscribers.
		if h.hub != nil {
			h.hub.BroadcastEvent([]string{"workflow"}, "workflow_failed", map[string]interface{}{
				"workflow": name,
				"error":    runErr.Error(),
			})
		}
		_ = sw.WriteError(fmt.Errorf("workflow execution failed: %v", runErr))
		return
	}

	LogEvent(h.auditStore, r, "workflow.run.stream", "workflow:"+name, "{}", "success")

	// Broadcast workflow completed to WebSocket subscribers.
	if h.hub != nil {
		h.hub.BroadcastEvent([]string{"workflow"}, "workflow_completed", map[string]interface{}{
			"workflow": name,
			"status":   result.Status,
		})
	}

	// Send final done event with the Result.
	_ = sw.WriteEvent("done", result)
}
