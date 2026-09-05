package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/api/rest/handler"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// TestNewUsersHandler verifies the constructor returns a non-nil handler.
func TestNewUsersHandler(t *testing.T) {
	h := handler.NewUsersHandler(nil, nil)
	if h == nil {
		t.Fatal("expected non-nil handler with nil store")
	}
}

// TestUsersListEmpty verifies List returns an empty array when no users exist.
func TestUsersListEmpty(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	h := handler.NewUsersHandler(store, nil)

	req := httptest.NewRequest("GET", "/v1/users", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty user list, got %d users", len(resp))
	}
}

// TestUsersListWithUsers verifies List returns users without password hashes.
func TestUsersListWithUsers(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("listuser", "password123", "editor", "list@example.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	h := handler.NewUsersHandler(store, nil)

	req := httptest.NewRequest("GET", "/v1/users", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 user, got %d", len(resp))
	}
	if resp[0]["id"] != user.ID {
		t.Errorf("expected id %q, got %v", user.ID, resp[0]["id"])
	}
	if resp[0]["username"] != "listuser" {
		t.Errorf("expected username 'listuser', got %v", resp[0]["username"])
	}
	if hash, ok := resp[0]["password_hash"].(string); ok && hash != "" {
		t.Error("password hash must not be exposed in List response")
	}
}

// TestUsersListNilStore verifies List returns 503 when the store is nil.
func TestUsersListNilStore(t *testing.T) {
	h := handler.NewUsersHandler(nil, nil)

	req := httptest.NewRequest("GET", "/v1/users", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersCreateSuccess verifies Create returns 201 and never exposes the
// password hash.
func TestUsersCreateSuccess(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	h := handler.NewUsersHandler(store, auditStore)

	body := `{"username":"newuser","password":"password123","role":"viewer","email":"new@example.com"}`
	req := httptest.NewRequest("POST", "/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if id, _ := resp["id"].(string); id == "" {
		t.Error("expected non-empty user id")
	}
	if resp["username"] != "newuser" {
		t.Errorf("expected username 'newuser', got %v", resp["username"])
	}
	if resp["role"] != "viewer" {
		t.Errorf("expected role 'viewer', got %v", resp["role"])
	}
	if hash, ok := resp["password_hash"].(string); ok && hash != "" {
		t.Error("password hash must not be exposed in Create response")
	}
}

// TestUsersCreateDuplicate verifies Create returns 409 for a duplicate username.
func TestUsersCreateDuplicate(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	if _, err := store.Create("dupuser", "password123", "viewer", ""); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	h := handler.NewUsersHandler(store, nil)

	body := `{"username":"dupuser","password":"password123","role":"viewer"}`
	req := httptest.NewRequest("POST", "/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersCreateMissingFields verifies Create returns 400 when required
// fields are missing.
func TestUsersCreateMissingFields(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	h := handler.NewUsersHandler(store, nil)

	for _, body := range []string{
		`{"username":"","password":"password123","role":"viewer"}`,
		`{"username":"u1","password":"","role":"viewer"}`,
		`{"username":"u1","password":"password123","role":""}`,
	} {
		req := httptest.NewRequest("POST", "/v1/users", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for body %s, got %d: %s", body, w.Code, w.Body.String())
		}
	}
}

// TestUsersCreateUsernameTooLong verifies Create returns 400 for a long username.
func TestUsersCreateUsernameTooLong(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	h := handler.NewUsersHandler(store, nil)

	body := `{"username":"` + strings.Repeat("u", 51) + `","password":"password123","role":"viewer"}`
	req := httptest.NewRequest("POST", "/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for username over 50 chars, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersCreatePasswordTooLong verifies Create returns 400 for a long password.
func TestUsersCreatePasswordTooLong(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	h := handler.NewUsersHandler(store, nil)

	body := `{"username":"okuser","password":"` + strings.Repeat("p", 129) + `","role":"viewer"}`
	req := httptest.NewRequest("POST", "/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for password over 128 chars, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersCreateEmailTooLong verifies Create returns 400 for a long email.
func TestUsersCreateEmailTooLong(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	h := handler.NewUsersHandler(store, nil)

	longEmail := strings.Repeat("e", 255) + "@example.com"
	body := `{"username":"okuser","password":"password123","role":"viewer","email":"` + longEmail + `"}`
	req := httptest.NewRequest("POST", "/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for email over 254 chars, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersCreateInvalidJSON verifies Create returns 400 for a bad body.
func TestUsersCreateInvalidJSON(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	h := handler.NewUsersHandler(store, nil)

	req := httptest.NewRequest("POST", "/v1/users", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersCreateNilStore verifies Create returns 503 when the store is nil.
func TestUsersCreateNilStore(t *testing.T) {
	h := handler.NewUsersHandler(nil, nil)

	body := `{"username":"u","password":"password123","role":"viewer"}`
	req := httptest.NewRequest("POST", "/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersDeleteSuccess verifies Delete returns 200 and removes the user.
func TestUsersDeleteSuccess(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("deleteuser", "password123", "viewer", "")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	auditStore := newTestAuditStore(t)
	h := handler.NewUsersHandler(store, auditStore)

	req := httptest.NewRequest("DELETE", "/v1/users/"+user.ID, nil)
	req.SetPathValue("id", user.ID)
	w := httptest.NewRecorder()

	h.Delete(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["status"] != "deleted" {
		t.Errorf("expected status 'deleted', got %v", resp["status"])
	}

	if _, err := store.GetByID(user.ID); err == nil {
		t.Error("expected user to be removed from store")
	}
}

// TestUsersDeleteNotFound verifies Delete returns 404 for a missing user.
func TestUsersDeleteNotFound(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	h := handler.NewUsersHandler(store, nil)

	req := httptest.NewRequest("DELETE", "/v1/users/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersDeleteMissingID verifies Delete returns 400 when the id is empty.
func TestUsersDeleteMissingID(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	h := handler.NewUsersHandler(store, nil)

	req := httptest.NewRequest("DELETE", "/v1/users/", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	h.Delete(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersDeleteNilStore verifies Delete returns 503 when the store is nil.
func TestUsersDeleteNilStore(t *testing.T) {
	h := handler.NewUsersHandler(nil, nil)

	req := httptest.NewRequest("DELETE", "/v1/users/x", nil)
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()

	h.Delete(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersUpdateRoleSuccess verifies UpdateRole returns 200 and updates the role.
func TestUsersUpdateRoleSuccess(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("roleuser", "password123", "viewer", "")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	auditStore := newTestAuditStore(t)
	h := handler.NewUsersHandler(store, auditStore)

	body := `{"role":"editor"}`
	req := httptest.NewRequest("PUT", "/v1/users/"+user.ID+"/role", strings.NewReader(body))
	req.SetPathValue("id", user.ID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRole(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["status"] != "updated" {
		t.Errorf("expected status 'updated', got %v", resp["status"])
	}

	updated, err := store.GetByID(user.ID)
	if err != nil {
		t.Fatalf("failed to load user: %v", err)
	}
	if updated.Role != "editor" {
		t.Errorf("expected role 'editor' after update, got %q", updated.Role)
	}
}

// TestUsersUpdateRoleInvalidRole verifies UpdateRole returns 400 for a role
// that is not admin, editor, or viewer.
func TestUsersUpdateRoleInvalidRole(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("roleuser2", "password123", "viewer", "")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	h := handler.NewUsersHandler(store, nil)

	body := `{"role":"superuser"}`
	req := httptest.NewRequest("PUT", "/v1/users/"+user.ID+"/role", strings.NewReader(body))
	req.SetPathValue("id", user.ID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRole(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid role, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersUpdateRoleNotFound documents the current behavior for a missing
// user: the handler maps the store's ErrUserNotFound to 400 instead of 404.
func TestUsersUpdateRoleNotFound(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	h := handler.NewUsersHandler(store, nil)

	body := `{"role":"editor"}`
	req := httptest.NewRequest("PUT", "/v1/users/nonexistent/role", strings.NewReader(body))
	req.SetPathValue("id", "nonexistent")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRole(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request (current behavior), got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersUpdateRoleMissingID verifies UpdateRole returns 400 when the id is empty.
func TestUsersUpdateRoleMissingID(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	h := handler.NewUsersHandler(store, nil)

	body := `{"role":"editor"}`
	req := httptest.NewRequest("PUT", "/v1/users//role", strings.NewReader(body))
	req.SetPathValue("id", "")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRole(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersUpdateRoleInvalidJSON verifies UpdateRole returns 400 for a bad body.
func TestUsersUpdateRoleInvalidJSON(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	h := handler.NewUsersHandler(store, nil)

	req := httptest.NewRequest("PUT", "/v1/users/x/role", strings.NewReader("not-json"))
	req.SetPathValue("id", "x")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRole(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersUpdateRoleEmptyRole verifies UpdateRole returns 400 when role is empty.
func TestUsersUpdateRoleEmptyRole(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	h := handler.NewUsersHandler(store, nil)

	body := `{"role":""}`
	req := httptest.NewRequest("PUT", "/v1/users/x/role", strings.NewReader(body))
	req.SetPathValue("id", "x")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRole(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUsersUpdateRoleNilStore verifies UpdateRole returns 503 when the store is nil.
func TestUsersUpdateRoleNilStore(t *testing.T) {
	h := handler.NewUsersHandler(nil, nil)

	body := `{"role":"editor"}`
	req := httptest.NewRequest("PUT", "/v1/users/x/role", strings.NewReader(body))
	req.SetPathValue("id", "x")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateRole(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}
