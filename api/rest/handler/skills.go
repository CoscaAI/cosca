package handler

import (
	"encoding/json"
	"net/http"

	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/skills"
)

// SkillsHandler handles REST API requests for the Skills Manager.
type SkillsHandler struct {
	mgr        *skills.Manager
	auditStore *audit.Store
}

// NewSkillsHandler creates a new SkillsHandler.
// If mgr is nil, the handler returns empty results gracefully.
func NewSkillsHandler(mgr *skills.Manager, auditStore *audit.Store) *SkillsHandler {
	return &SkillsHandler{mgr: mgr, auditStore: auditStore}
}

// List handles GET /v1/skills
func (h *SkillsHandler) List(w http.ResponseWriter, _ *http.Request) {
	if h.mgr == nil {
		writeJSON(w, http.StatusOK, []skills.Skill{})
		return
	}
	items := h.mgr.List()
	writeJSON(w, http.StatusOK, items)
}

// Search handles GET /v1/skills/search?q=...
func (h *SkillsHandler) Search(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeJSON(w, http.StatusOK, []skills.Skill{})
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
		items = []skills.Skill{}
	}
	writeJSON(w, http.StatusOK, items)
}

// Get handles GET /v1/skills/{name}
func (h *SkillsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeError(w, http.StatusServiceUnavailable, "skills manager not available")
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

// Install handles POST /v1/skills/{name}/install
func (h *SkillsHandler) Install(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeError(w, http.StatusServiceUnavailable, "skills manager not available")
		return
	}
	name := r.PathValue("name")
	var req struct {
		Source string `json:"source"`
	}
	limitBody(w, r, bodyLimitSmall)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}
	skill, err := h.mgr.Install(name, req.Source)
	if err != nil {
		LogEvent(h.auditStore, r, "skill.install", "skill:"+name,
			audit.DetailsJSON(map[string]string{"source": req.Source, "error": err.Error()}), "error")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	LogEvent(h.auditStore, r, "skill.install", "skill:"+name,
		audit.DetailsJSON(map[string]string{"source": req.Source}), "success")
	writeJSON(w, http.StatusCreated, skill)
}
