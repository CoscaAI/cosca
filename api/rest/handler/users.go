package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/auth"
)

const (
	maxUserUsernameLength = 50
	maxUserPasswordLength = 128
	maxUserEmailLength    = 254
)

// UsersHandler handles user management endpoints (admin only).
type UsersHandler struct {
	store      *auth.UserStore
	auditStore *audit.Store
}

// NewUsersHandler creates a new UsersHandler.
func NewUsersHandler(store *auth.UserStore, auditStore *audit.Store) *UsersHandler {
	return &UsersHandler{store: store, auditStore: auditStore}
}

// createUserRequest is the expected JSON body for POST /v1/users.
type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
	Email    string `json:"email,omitempty"`
}

// updateRoleRequest is the expected JSON body for PUT /v1/users/{id}/role.
type updateRoleRequest struct {
	Role string `json:"role"`
}

// List handles GET /v1/users — returns all registered users (admin only).
func (h *UsersHandler) List(w http.ResponseWriter, _ *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "user store not available")
		return
	}

	users := h.store.List()
	writeJSON(w, http.StatusOK, users)
}

// Create handles POST /v1/users — creates a new user (admin only).
func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "user store not available")
		return
	}

	var req createUserRequest
	limitBody(w, r, bodyLimitSmall)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.Username == "" || req.Password == "" || req.Role == "" {
		writeError(w, http.StatusBadRequest, "username, password, and role are required")
		return
	}

	if len(req.Username) > maxUserUsernameLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("username too long: max %d characters", maxUserUsernameLength))
		return
	}

	if len(req.Password) > maxUserPasswordLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("password too long: max %d characters", maxUserPasswordLength))
		return
	}

	if len(req.Email) > maxUserEmailLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("email too long: max %d characters", maxUserEmailLength))
		return
	}

	user, err := h.store.Create(req.Username, req.Password, req.Role, req.Email)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	user.PasswordHash = ""
	LogEvent(h.auditStore, r, "user.create", "user:"+user.ID, audit.DetailsJSON(map[string]string{"username": req.Username, "role": req.Role}), "success")
	writeJSON(w, http.StatusCreated, user)
}

// Delete handles DELETE /v1/users/{id} — removes a user (admin only).
func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "user store not available")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "user id is required")
		return
	}

	if err := h.store.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	LogEvent(h.auditStore, r, "user.delete", "user:"+id, audit.DetailsJSON(map[string]string{"user_id": id}), "success")
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// UpdateRole handles PUT /v1/users/{id}/role — updates a user's role (admin only).
func (h *UsersHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "user store not available")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "user id is required")
		return
	}

	var req updateRoleRequest
	limitBody(w, r, bodyLimitSmall)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.Role == "" {
		writeError(w, http.StatusBadRequest, "role is required")
		return
	}

	if err := h.store.UpdateRole(id, req.Role); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	LogEvent(h.auditStore, r, "user.update_role", "user:"+id, audit.DetailsJSON(map[string]string{"user_id": id, "new_role": req.Role}), "success")
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
