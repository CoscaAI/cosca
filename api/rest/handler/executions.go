package handler

import (
	"net/http"
	"strconv"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

// ExecutionsHandler handles execution history endpoints.
type ExecutionsHandler struct {
	store *orchestration.ExecutionStore
}

// NewExecutionsHandler creates a new ExecutionsHandler backed by the given store.
func NewExecutionsHandler(store *orchestration.ExecutionStore) *ExecutionsHandler {
	if store == nil {
		store = orchestration.GetExecutionStore()
	}
	return &ExecutionsHandler{store: store}
}

// List handles GET /v1/executions — returns a paginated list of execution records.
//
// Query parameters:
//
//	limit  — maximum records per page (default: 20)
//	offset — number of records to skip (default: 0)
//	agent  — filter by agent name
//	status — filter by execution status ("success" or "error")
//
// Owner scoping (A6): authenticated users only see their own executions.
// Anonymous/system callers (no claims) see everything.
func (h *ExecutionsHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := parseIntParam(q.Get("limit"), 20)
	offset := parseIntParam(q.Get("offset"), 0)
	agent := q.Get("agent")
	status := q.Get("status")

	// Guard de nil (família Missing Dependency Guard #17/#18): um handler
	// criado sem o construtor (zero-value) deve responder 503, nunca panic.
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "execution store not available")
		return
	}

	executions, total := h.store.List(limit, offset, agent, status, claimsSubject(r))

	resp := map[string]interface{}{
		"executions": executions,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	}

	writeJSON(w, http.StatusOK, resp)
}

// Get handles GET /v1/executions/{id} — returns the full detail of a single
// execution, including pipeline trace if available.
//
// Owner scoping (A6): an authenticated user can only read their own
// executions; someone else's execution is reported as not found (no IDOR, no
// existence leak). System/global executions (empty owner) and anonymous
// callers keep full visibility.
func (h *ExecutionsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "execution id is required")
		return
	}
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "execution store not available")
		return
	}

	exec := h.store.Get(id)
	if exec == nil {
		writeError(w, http.StatusNotFound, "execution not found")
		return
	}

	if userID := claimsSubject(r); userID != "" && exec.UserID != "" && exec.UserID != userID {
		writeError(w, http.StatusNotFound, "execution not found")
		return
	}

	writeJSON(w, http.StatusOK, exec)
}

// parseIntParam parses a string to an integer with a default fallback.
func parseIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return defaultVal
	}
	return v
}
