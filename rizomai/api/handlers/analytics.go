// Handlers de analytics (Fase 6): métricas por post e por profile.
package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/rizomai/rizomai/api/middleware"
	"github.com/rizomai/rizomai/api/respond"
	"github.com/rizomai/rizomai/internal/domain"
)

// GetPostAnalytics — GET /v1/analytics/posts/{id}: métricas agregadas do post
// (por plataforma — views/likes/comments/shares).
func (h *Handlers) GetPostAnalytics(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())
	id := r.PathValue("id")

	metrics, err := h.Store.GetPostAnalytics(r.Context(), teamID, id)
	if isNotFound(err) {
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "Post não encontrado",
			map[string]any{"resource": "post", "id": id})
		return
	}
	if err != nil {
		log.Printf("get post analytics: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	if metrics == nil {
		metrics = []domain.PostAnalytics{}
	}
	writeData(w, http.StatusOK, metrics)
}

// GetProfileAnalytics — GET /v1/analytics/profile?days=30&profileId=...:
// agregação por plataforma nos últimos N dias.
func (h *Handlers) GetProfileAnalytics(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())
	profileID := r.URL.Query().Get("profileId")
	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}

	if profileID == "" {
		// Sem profileId → primeiro profile do team.
		profiles, _, err := h.Store.ListProfiles(r.Context(), teamID, 1, 0)
		if err != nil || len(profiles) == 0 {
			respond.Error(w, http.StatusNotFound, "NOT_FOUND", "Nenhum profile no tenant — crie um profile primeiro", nil)
			return
		}
		profileID = profiles[0].ID
	}

	agg, err := h.Store.GetProfileAnalytics(r.Context(), teamID, profileID, days)
	if err != nil {
		log.Printf("get profile analytics: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	if agg == nil {
		agg = []domain.ProfileAnalytics{}
	}
	writeData(w, http.StatusOK, map[string]any{
		"profileId": profileID,
		"days":      days,
		"metrics":   agg,
	})
}
