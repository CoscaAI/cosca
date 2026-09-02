package handlers

import (
	"net/http"

	"github.com/rizomai/rizomai/api/respond"
)

// Connect — GET /v1/connect/{platform} (início do OAuth, ADR-006).
// Fase 3: fluxo real (authUrl + state com PKCE). Por ora: 501 NOT_IMPLEMENTED.
func (h *Handlers) Connect(w http.ResponseWriter, r *http.Request) {
	platform := r.PathValue("platform")
	respond.Error(w, http.StatusNotImplemented, "NOT_IMPLEMENTED",
		"OAuth broker chega na Fase 3",
		map[string]any{"platform": platform})
}
