package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/rizomai/rizomai/api/middleware"
	"github.com/rizomai/rizomai/api/respond"
	"github.com/rizomai/rizomai/internal/domain"
)

// ListProfiles — GET /v1/profiles (paginado, escopo do team autenticado).
func (h *Handlers) ListProfiles(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())
	page, limit := pageLimit(r)
	offset := (page - 1) * limit

	profiles, total, err := h.Store.ListProfiles(r.Context(), teamID, limit, offset)
	if err != nil {
		log.Printf("list profiles: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	writeList(w, http.StatusOK, profiles, page, limit, total)
}

// CreateProfile — POST /v1/profiles (com Idempotency-Key, ADR-005 §1.3).
func (h *Handlers) CreateProfile(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())

	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey != "" {
		if status, body, err := h.Store.GetIdempotency(r.Context(), teamID, idemKey); err == nil {
			replayResponse(w, status, body)
			return
		}
	}

	var payload domain.ProfileCreatePayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	if verr := payload.Validate(); verr != nil {
		respond.Error(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Dados inválidos",
			map[string]any{"fields": verr.Fields})
		return
	}

	id, err := domain.NewProfileID()
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	now := time.Now().UTC()
	p := &domain.Profile{ID: id, TeamID: teamID, Name: payload.Name, CreatedAt: now, UpdatedAt: now}

	if err := h.Store.CreateProfile(r.Context(), p); err != nil {
		log.Printf("create profile: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	body := writeData(w, http.StatusCreated, p)
	if idemKey != "" {
		if err := h.Store.SaveIdempotency(r.Context(), teamID, idemKey, "", http.StatusCreated, body); err != nil {
			log.Printf("save idempotency: %v", err)
		}
	}
}

// GetProfile — GET /v1/profiles/{id}.
func (h *Handlers) GetProfile(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())
	id := r.PathValue("id")

	p, err := h.Store.GetProfile(r.Context(), teamID, id)
	if isNotFound(err) {
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "Perfil não encontrado",
			map[string]any{"resource": "profile", "id": id})
		return
	}
	if err != nil {
		log.Printf("get profile: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	writeData(w, http.StatusOK, p)
}
