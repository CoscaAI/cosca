package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/rizomai/rizomai/api/middleware"
	"github.com/rizomai/rizomai/api/respond"
	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/oauth"
)

// webhookCreatePayload espelha WebhookCreate da spec (profileId extra é
// opcional — derivado do team quando ausente).
type webhookCreatePayload struct {
	ProfileID     string            `json:"profileId"`
	Name          string            `json:"name"`
	URL           string            `json:"url"`
	Events        []string          `json:"events"`
	IsActive      *bool             `json:"isActive"`
	CustomHeaders map[string]string `json:"customHeaders"`
}

// CreateWebhook — POST /v1/webhooks (ADR-009 §1.7).
// O secret é gerado server-side, devolvido na resposta UMA única vez e
// armazenado apenas como hash + criptografado (para assinar payloads).
func (h *Handlers) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())

	var payload webhookCreatePayload
	if !decodeJSON(w, r, &payload) {
		return
	}

	verr := map[string]string{}
	if strings.TrimSpace(payload.Name) == "" {
		verr["name"] = "obrigatório"
	} else if len(payload.Name) > 100 {
		verr["name"] = "máximo 100 caracteres"
	}
	if payload.URL == "" {
		verr["url"] = "obrigatório"
	} else if !strings.HasPrefix(payload.URL, "https://") {
		verr["url"] = "HTTPS obrigatório (ADR-009 §1.7)"
	}
	if len(verr) > 0 {
		respond.Error(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Dados inválidos",
			map[string]any{"fields": verr})
		return
	}

	profileID := payload.ProfileID
	if profileID == "" {
		pid, err := h.ensureTeamProfile(r.Context(), teamID)
		if err != nil {
			log.Printf("webhook ensure profile: %v", err)
			respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
			return
		}
		profileID = pid
	}

	events := payload.Events
	if len(events) == 0 {
		events = []string{"post.published", "post.failed"} // default da spec
	}
	isActive := true
	if payload.IsActive != nil {
		isActive = *payload.IsActive
	}

	// Secret: gerado server-side, devolvido UMA vez (spec §1.7).
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	secret := hex.EncodeToString(secretBytes)
	hash := sha256.Sum256([]byte(secret))
	encSecret, err := oauth.Encrypt([]byte(secret), h.TokenKey)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	now := time.Now().UTC()
	id, err := domain.NewWebhookID()
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	wh := &domain.Webhook{
		ID:              id,
		ProfileID:       profileID,
		Name:            payload.Name,
		URL:             payload.URL,
		SecretHash:      hex.EncodeToString(hash[:]),
		SecretEncrypted: encSecret,
		Events:          events,
		IsActive:        isActive,
		CustomHeaders:   payload.CustomHeaders,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := h.Store.CreateWebhook(r.Context(), wh); err != nil {
		log.Printf("create webhook: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	// O secret aparece SOMENTE nesta resposta (mascarado nos demais).
	resp := toWebhookResponse(*wh, secret)
	writeData(w, http.StatusCreated, resp)
}

// ListWebhooks — GET /v1/webhooks (secret mascarado).
func (h *Handlers) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	// Escopo por team: junta webhooks de todos os profiles do team.
	profiles, _, err := h.Store.ListProfiles(r.Context(), middleware.TeamIDFromContext(r.Context()), 1000, 0)
	if err != nil {
		log.Printf("list webhooks profiles: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	var out []webhookResponse
	for _, p := range profiles {
		whs, err := h.Store.ListWebhooksByProfile(r.Context(), p.ID)
		if err != nil {
			log.Printf("list webhooks: %v", err)
			respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
			return
		}
		for _, wh := range whs {
			out = append(out, toWebhookResponse(wh, ""))
		}
	}
	if out == nil {
		out = []webhookResponse{}
	}
	writeData(w, http.StatusOK, out)
}

// webhookResponse serializa o webhook SEM expor o secret (mascarado).
type webhookResponse struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	URL           string            `json:"url"`
	Secret        string            `json:"secret,omitempty"` // só no create
	Events        []string          `json:"events"`
	IsActive      bool              `json:"isActive"`
	CustomHeaders map[string]string `json:"customHeaders,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

func toWebhookResponse(wh domain.Webhook, plainSecret string) webhookResponse {
	return webhookResponse{
		ID: wh.ID, Name: wh.Name, URL: wh.URL, Secret: plainSecret,
		Events: wh.Events, IsActive: wh.IsActive, CustomHeaders: wh.CustomHeaders,
		CreatedAt: wh.CreatedAt, UpdatedAt: wh.UpdatedAt,
	}
}

// ensureTeamProfile devolve o primeiro profile do team ou cria um default.
func (h *Handlers) ensureTeamProfile(ctx context.Context, teamID string) (string, error) {
	profiles, _, err := h.Store.ListProfiles(ctx, teamID, 1, 0)
	if err != nil {
		return "", err
	}
	if len(profiles) > 0 {
		return profiles[0].ID, nil
	}
	return h.ensureProfile(ctx, teamID)
}
