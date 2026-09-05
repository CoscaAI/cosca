package handler

import (
	"net/http"

	"github.com/CoscaAI/cosca/internal/agents"
)

// AgentsHandler handles REST API requests for the Agents Manager.
type AgentsHandler struct {
	mgr *agents.Manager
}

// NewAgentsHandler creates a new AgentsHandler.
// If mgr is nil, the handler returns empty results gracefully.
func NewAgentsHandler(mgr *agents.Manager) *AgentsHandler {
	return &AgentsHandler{mgr: mgr}
}

// List handles GET /v1/agents
func (h *AgentsHandler) List(w http.ResponseWriter, _ *http.Request) {
	if h.mgr == nil {
		writeJSON(w, http.StatusOK, []agents.Agent{})
		return
	}
	items := h.mgr.List()
	writeJSON(w, http.StatusOK, items)
}

// Search handles GET /v1/agents/search?q=...
func (h *AgentsHandler) Search(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeJSON(w, http.StatusOK, []agents.Agent{})
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
		items = []agents.Agent{}
	}
	writeJSON(w, http.StatusOK, items)
}

// Get handles GET /v1/agents/{name}
func (h *AgentsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeError(w, http.StatusServiceUnavailable, "agents manager not available")
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
