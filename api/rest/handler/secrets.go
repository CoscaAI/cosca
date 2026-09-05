package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	apiauth "github.com/CoscaAI/cosca/api/auth"
	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/secrets"
)

const maxSecretKeyLength = 100

// SecretsHandler handles REST API requests for the secrets vault (admin only).
type SecretsHandler struct {
	vault      *secrets.Vault
	auditStore *audit.Store
}

// NewSecretsHandler creates a new SecretsHandler.
func NewSecretsHandler(vault *secrets.Vault, auditStore *audit.Store) *SecretsHandler {
	return &SecretsHandler{vault: vault, auditStore: auditStore}
}

// createSecretRequest is the expected JSON body for POST /v1/secrets.
type createSecretRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// deleteSecretRequest is the expected JSON body for DELETE /v1/secrets/{key}.
type deleteSecretRequest struct {
	ConfirmKey string `json:"confirm_key"`
}

// secretValueResponse includes the decrypted value in the response.
// The value is auto-hidden — it is only returned once and the client
// should not persist it in logs or localStorage.
type secretValueResponse struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Type      string `json:"type"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	CreatedBy string `json:"created_by"`
}

// List handles GET /v1/secrets — returns all stored keys without their values.
func (h *SecretsHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.vault == nil {
		writeError(w, http.StatusServiceUnavailable, "secrets vault not available")
		return
	}

	keys, err := h.vault.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list secrets: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"secrets": keys,
		"count":   len(keys),
	})
}

// CreateOrUpdate handles POST /v1/secrets — creates or updates a secret.
func (h *SecretsHandler) CreateOrUpdate(w http.ResponseWriter, r *http.Request) {
	if h.vault == nil {
		writeError(w, http.StatusServiceUnavailable, "secrets vault not available")
		return
	}

	var req createSecretRequest
	limitBody(w, r, bodyLimitMedium)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.Key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	if req.Value == "" {
		writeError(w, http.StatusBadRequest, "value is required")
		return
	}

	if len(req.Key) > maxSecretKeyLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("key too long: max %d characters", maxSecretKeyLength))
		return
	}

	createdBy := "system"
	if claims, ok := apiauth.ClaimsFromContext(r.Context()); ok && claims != nil {
		createdBy = claims.Username
	}

	if err := h.vault.Set(req.Key, req.Value, req.Type, createdBy); err != nil {
		LogEvent(h.auditStore, r, "secret.create", "secret:"+req.Key, audit.DetailsJSON(map[string]string{"error": err.Error()}), "error")
		writeError(w, http.StatusInternalServerError, "failed to store secret: "+err.Error())
		return
	}

	LogEvent(h.auditStore, r, "secret.create", "secret:"+req.Key, audit.DetailsJSON(map[string]string{"type": req.Type}), "success")

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "stored",
		"key":    req.Key,
	})
}

// Get handles GET /v1/secrets/{key} — retrieves a single secret with its
// decrypted value. The decrypted value must be handled with care and not
// persisted in logs or client storage.
func (h *SecretsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.vault == nil {
		writeError(w, http.StatusServiceUnavailable, "secrets vault not available")
		return
	}

	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "secret key is required")
		return
	}

	sec, value, err := h.vault.Get(key)
	if err != nil {
		if err == secrets.ErrSecretNotFound {
			LogEvent(h.auditStore, r, "secret.read", "secret:"+key, audit.DetailsJSON(map[string]string{"error": err.Error()}), "error")
			writeError(w, http.StatusNotFound, "secret not found")
		} else {
			LogEvent(h.auditStore, r, "secret.read", "secret:"+key, audit.DetailsJSON(map[string]string{"error": err.Error()}), "error")
			writeError(w, http.StatusInternalServerError, "failed to get secret: "+err.Error())
		}
		return
	}

	LogEvent(h.auditStore, r, "secret.read", "secret:"+key, "{}", "success")

	resp := secretValueResponse{
		Key:       sec.Key,
		Value:     value,
		Type:      sec.Type,
		CreatedAt: sec.CreatedAt,
		UpdatedAt: sec.UpdatedAt,
		CreatedBy: sec.CreatedBy,
	}

	writeJSON(w, http.StatusOK, resp)
}

// Delete handles DELETE /v1/secrets/{key} — deletes a secret. Requires a
// confirmation body with the key name to prevent accidental deletion.
func (h *SecretsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.vault == nil {
		writeError(w, http.StatusServiceUnavailable, "secrets vault not available")
		return
	}

	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "secret key is required")
		return
	}

	var req deleteSecretRequest
	limitBody(w, r, bodyLimitSmall)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.ConfirmKey != key {
		writeError(w, http.StatusBadRequest, "confirm_key must match the secret key being deleted")
		return
	}

	if err := h.vault.Delete(key); err != nil {
		if err == secrets.ErrSecretNotFound {
			LogEvent(h.auditStore, r, "secret.delete", "secret:"+key, audit.DetailsJSON(map[string]string{"error": err.Error()}), "error")
			writeError(w, http.StatusNotFound, "secret not found")
		} else {
			LogEvent(h.auditStore, r, "secret.delete", "secret:"+key, audit.DetailsJSON(map[string]string{"error": err.Error()}), "error")
			writeError(w, http.StatusInternalServerError, "failed to delete secret: "+err.Error())
		}
		return
	}

	LogEvent(h.auditStore, r, "secret.delete", "secret:"+key, "{}", "success")

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
		"key":    key,
	})
}
