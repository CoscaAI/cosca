package handler

import (
	"encoding/json"
	"net/http"

	"github.com/CoscaAI/cosca/internal/providers"
)

// ProvidersHandler handles REST API requests for the Providers Manager.
type ProvidersHandler struct {
	mgr *providers.Manager
}

// NewProvidersHandler creates a new ProvidersHandler.
// If mgr is nil, the handler returns empty results gracefully.
func NewProvidersHandler(mgr *providers.Manager) *ProvidersHandler {
	return &ProvidersHandler{mgr: mgr}
}

// SetActiveRequest is the JSON body for setting the active provider.
type SetActiveRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// List handles GET /v1/providers
func (h *ProvidersHandler) List(w http.ResponseWriter, _ *http.Request) {
	if h.mgr == nil {
		writeJSON(w, http.StatusOK, []providers.ProviderInfo{})
		return
	}
	items := h.mgr.List()
	writeJSON(w, http.StatusOK, items)
}

// Get handles GET /v1/providers/{name}
func (h *ProvidersHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeError(w, http.StatusServiceUnavailable, "providers manager not available")
		return
	}
	name := r.PathValue("name")
	item, err := h.mgr.Info(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// Test handles POST /v1/providers/{name}/test
func (h *ProvidersHandler) Test(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeError(w, http.StatusServiceUnavailable, "providers manager not available")
		return
	}
	name := r.PathValue("name")
	result, err := h.mgr.Test(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SetActive handles PUT /v1/providers/active
func (h *ProvidersHandler) SetActive(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeError(w, http.StatusServiceUnavailable, "providers manager not available")
		return
	}
	var req SetActiveRequest
	limitBody(w, r, bodyLimitSmall)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}
	if req.Provider == "" {
		writeError(w, http.StatusBadRequest, "provider is required")
		return
	}
	if err := h.mgr.SetActive(req.Provider, req.Model); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h.mgr.Status())
}
