package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	apiauth "github.com/CoscaAI/cosca/api/auth"
	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/auth"
)

const maxAPIKeyNameLength = 100

// APIKeysHandler handles REST API requests for the API Key Store.
type APIKeysHandler struct {
	store      *auth.APIKeyStore
	auditStore *audit.Store
}

// NewAPIKeysHandler creates a new APIKeysHandler.
func NewAPIKeysHandler(store *auth.APIKeyStore, auditStore *audit.Store) *APIKeysHandler {
	return &APIKeysHandler{store: store, auditStore: auditStore}
}

// createAPIKeyRequest is the expected JSON body for POST /v1/api-keys.
type createAPIKeyRequest struct {
	Name          string `json:"name"`
	ExpiresInDays int    `json:"expires_in_days,omitempty"`
}

// apiKeyResponse is the JSON response for a newly created API key.
// The FullKey field is returned only once at creation time.
type apiKeyResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	FullKey   string `json:"key"` // only returned on creation
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Status    string `json:"status"`
	CreatedBy string `json:"created_by"`
}

// List handles GET /v1/api-keys — returns all API keys (without secrets).
func (h *APIKeysHandler) List(w http.ResponseWriter, _ *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "api key store not available")
		return
	}

	keys := h.store.List()
	if keys == nil {
		keys = []auth.APIKey{}
	}
	writeJSON(w, http.StatusOK, keys)
}

// Create handles POST /v1/api-keys — generates a new API key.
func (h *APIKeysHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "api key store not available")
		return
	}

	var req createAPIKeyRequest
	limitBody(w, r, bodyLimitSmall)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if len(req.Name) > maxAPIKeyNameLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("name too long: max %d characters", maxAPIKeyNameLength))
		return
	}

	// Determine who is creating this key from the JWT claims.
	createdBy := "system"
	role := "admin"
	if claims, ok := apiauth.ClaimsFromContext(r.Context()); ok && claims != nil {
		createdBy = claims.Username
		if claims.Role != "" {
			role = claims.Role
		}
	}

	var expiresIn time.Duration
	if req.ExpiresInDays > 0 {
		expiresIn = time.Duration(req.ExpiresInDays) * 24 * time.Hour
	}

	apiKey, fullKey, err := h.store.Generate(req.Name, createdBy, role, expiresIn)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate api key: "+err.Error())
		return
	}

	LogEvent(h.auditStore, r, "apikey.generate", "apikey:"+apiKey.ID, audit.DetailsJSON(map[string]string{"name": req.Name, "role": role}), "success")

	resp := apiKeyResponse{
		ID:        apiKey.ID,
		Name:      apiKey.Name,
		Prefix:    apiKey.Prefix,
		FullKey:   fullKey,
		CreatedAt: apiKey.CreatedAt,
		ExpiresAt: apiKey.ExpiresAt,
		Status:    apiKey.Status,
		CreatedBy: apiKey.CreatedBy,
	}

	writeJSON(w, http.StatusCreated, resp)
}

// Revoke handles DELETE /v1/api-keys/{id} — revokes an API key.
func (h *APIKeysHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "api key store not available")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "api key id is required")
		return
	}

	if err := h.store.Revoke(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	LogEvent(h.auditStore, r, "apikey.revoke", "apikey:"+id, audit.DetailsJSON(map[string]string{"apikey_id": id}), "success")
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}
