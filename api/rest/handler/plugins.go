package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/CoscaAI/cosca/internal/plugins"
)

// PluginsHandler handles plugin-related API endpoints.
type PluginsHandler struct {
	manager *plugins.Manager
}

// NewPluginsHandler creates a new PluginsHandler.
func NewPluginsHandler(manager *plugins.Manager) *PluginsHandler {
	return &PluginsHandler{manager: manager}
}

// PluginListItem represents a plugin in the API response list.
type PluginListItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author,omitempty"`
	State       string `json:"state"`
	Enabled     bool   `json:"enabled"`
	Runtime     string `json:"runtime"`
}

// List handles GET /v1/plugins.
func (h *PluginsHandler) List(w http.ResponseWriter, r *http.Request) {
	result := []PluginListItem{}

	if h.manager != nil {
		installed := h.manager.List()
		for _, p := range installed {
			result = append(result, PluginListItem{
				ID:          p.Manifest.ID,
				Name:        p.Manifest.Name,
				Version:     p.Manifest.Version,
				Description: p.Manifest.Description,
				Author:      p.Manifest.Author,
				State:       p.State.String(),
				Enabled:     p.Enabled,
				Runtime:     string(p.Manifest.Runtime),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("plugins: encode error: %v", err)
	}
}
