package handler

import (
	"net/http"
	"time"

	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/CoscaAI/cosca/internal/runtime"
	"github.com/CoscaAI/cosca/internal/skills"
	"github.com/CoscaAI/cosca/internal/workflows"
)

// StatsHandler handles the platform-wide aggregated statistics endpoint.
type StatsHandler struct {
	agentsMgr    *agents.Manager
	skillsMgr    *skills.Manager
	providersMgr *providers.Manager
	workflowsMgr *workflows.Manager
	rt           *runtime.Runtime
}

// NewStatsHandler creates a new StatsHandler.
// Any manager or the runtime can be nil — the handler gracefully
// returns zero/empty values for unavailable subsystems.
func NewStatsHandler(
	agentsMgr *agents.Manager,
	skillsMgr *skills.Manager,
	providersMgr *providers.Manager,
	workflowsMgr *workflows.Manager,
	rt *runtime.Runtime,
) *StatsHandler {
	return &StatsHandler{
		agentsMgr:    agentsMgr,
		skillsMgr:    skillsMgr,
		providersMgr: providersMgr,
		workflowsMgr: workflowsMgr,
		rt:           rt,
	}
}

// --- Response types ---

// ProvidersStats holds aggregated provider statistics.
type ProvidersStats struct {
	Total  int    `json:"total"`
	Active string `json:"active,omitempty"`
}

// PlatformStats is the aggregated platform statistics response.
type PlatformStats struct {
	Agents    int            `json:"agents"`
	Skills    int            `json:"skills"`
	Providers ProvidersStats `json:"providers"`
	Workflows int            `json:"workflows"`
	Uptime    string         `json:"uptime,omitempty"`
	Version   string         `json:"version,omitempty"`
	Health    string         `json:"health,omitempty"`
}

// --- Handler ---

// Get handles GET /v1/stats
func (h *StatsHandler) Get(w http.ResponseWriter, _ *http.Request) {
	stats := PlatformStats{
		Providers: ProvidersStats{},
	}

	// Agent count.
	if h.agentsMgr != nil {
		stats.Agents = len(h.agentsMgr.List())
	}

	// Skill count.
	if h.skillsMgr != nil {
		stats.Skills = len(h.skillsMgr.List())
	}

	// Provider stats.
	if h.providersMgr != nil {
		providers := h.providersMgr.List()
		stats.Providers.Total = len(providers)
		status := h.providersMgr.Status()
		if status.Active != "" {
			stats.Providers.Active = status.Active
		}
	}

	// Workflow count.
	if h.workflowsMgr != nil {
		stats.Workflows = len(h.workflowsMgr.List())
	}

	// Runtime info.
	if h.rt != nil {
		cfg := h.rt.Config()
		stats.Version = cfg.Version
		stats.Health = string(h.rt.Health())

		stateSnapshot := h.rt.State().Get()
		stats.Uptime = stateSnapshot.Uptime.Round(time.Second).String()
	}

	writeJSON(w, http.StatusOK, stats)
}
