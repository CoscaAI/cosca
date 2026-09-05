package handler_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	apiauth "github.com/CoscaAI/cosca/api/auth"
	"github.com/CoscaAI/cosca/api/rest/handler"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// newClaimsContext returns a context carrying the given JWT claims, matching
// how the auth middleware injects them into the request context.
func newClaimsContext(claims *internalauth.Claims) context.Context {
	return context.WithValue(context.Background(), apiauth.ContextKeyClaims, claims)
}

// =============================================================================
// Register — auth.go:321
// =============================================================================

// TestRegisterSuccess verifies Register returns 201 with the new user, a token
// pair, and auth cookies.
func TestRegisterSuccess(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)
	authHandler.RegistrationEnabled = true

	body := `{"username":"newbie","password":"password123","email":"newbie@example.com"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	user, ok := resp["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'user' object in register response")
	}
	if user["username"] != "newbie" {
		t.Errorf("expected username 'newbie', got %v", user["username"])
	}
	// New registrations always default to the viewer role.
	if user["role"] != "viewer" {
		t.Errorf("expected role 'viewer', got %v", user["role"])
	}
	if hash, ok := user["password_hash"].(string); ok && hash != "" {
		t.Error("password hash must not be exposed in register response")
	}

	tokens, ok := resp["tokens"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'tokens' object in register response")
	}
	if access, _ := tokens["access_token"].(string); access == "" {
		t.Error("expected access_token in register response")
	}
	if refresh, _ := tokens["refresh_token"].(string); refresh == "" {
		t.Error("expected refresh_token in register response")
	}

	cookies := w.Result().Cookies()
	for _, name := range []string{"cosca_access_token", "cosca_refresh_token", "cosca_auth_state"} {
		if !hasCookie(cookies, name) {
			t.Errorf("expected %s cookie to be set", name)
		}
	}
}

// TestRegisterDuplicateUsername verifies Register returns 409 for a duplicate username.
func TestRegisterDuplicateUsername(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	if _, err := store.Create("takenuser", "password123", "viewer", ""); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)
	authHandler.RegistrationEnabled = true

	body := `{"username":"takenuser","password":"password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Register(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRegisterMissingFields verifies Register returns 400 when username or
// password is missing.
func TestRegisterMissingFields(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)
	authHandler.RegistrationEnabled = true

	for _, body := range []string{
		`{"username":"","password":"password123"}`,
		`{"username":"someuser","password":""}`,
	} {
		req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		authHandler.Register(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for body %s, got %d: %s", body, w.Code, w.Body.String())
		}
	}
}

// TestRegisterUsernameTooLong verifies Register returns 400 for a long username.
func TestRegisterUsernameTooLong(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)
	authHandler.RegistrationEnabled = true

	body := `{"username":"` + strings.Repeat("u", 51) + `","password":"password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for username over 50 chars, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRegisterPasswordTooShort verifies Register returns 400 for short passwords.
func TestRegisterPasswordTooShort(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)
	authHandler.RegistrationEnabled = true

	body := `{"username":"shortpw","password":"12345"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for password under 6 chars, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRegisterPasswordTooLong verifies Register returns 400 for long passwords.
func TestRegisterPasswordTooLong(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)
	authHandler.RegistrationEnabled = true

	body := `{"username":"longpw","password":"` + strings.Repeat("p", 129) + `"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for password over 128 chars, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRegisterInvalidJSON verifies Register returns 400 for a bad body.
func TestRegisterInvalidJSON(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)
	authHandler.RegistrationEnabled = true

	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRegisterNilStore verifies Register returns 503 when the store is nil.
func TestRegisterNilStore(t *testing.T) {
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(nil, testJWTSecret, auditStore)
	authHandler.RegistrationEnabled = true

	body := `{"username":"x","password":"password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Register(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRegisterDisabledByDefault verifies that registration is DISABLED out
// of the box (fail closed): Register returns 403 and creates no user, even
// with a perfectly valid body.
func TestRegisterDisabledByDefault(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	// Note: RegistrationEnabled intentionally left at its zero value (false).
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	body := `{"username":"newbie","password":"password123","email":"newbie@example.com"}`
	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Register(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden (registration disabled), got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["error"] != "registration is disabled" {
		t.Errorf("expected error 'registration is disabled', got %q", resp["error"])
	}
	// No user may be created while registration is disabled.
	if _, err := store.GetByUsername("newbie"); err == nil {
		t.Error("user must not be created while registration is disabled")
	}
}

// TestRegisterDisabledRejectsInvalidBodyToo verifies that the disabled check
// is fail-closed and runs before any body parsing: even a malformed request
// gets 403, never a validation error or a user creation.
func TestRegisterDisabledRejectsInvalidBodyToo(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	req := httptest.NewRequest("POST", "/v1/auth/register", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Register(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden (registration disabled), got %d: %s", w.Code, w.Body.String())
	}
}

// TestLoginWorksWhenRegistrationDisabled verifies that disabling public
// registration does NOT affect the login flow of existing users.
func TestLoginWorksWhenRegistrationDisabled(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	if _, err := store.Create("existinguser", "password123", "editor", ""); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)
	// Registration remains disabled — login must still work.

	body := `{"username":"existinguser","password":"password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for existing user login, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["access_token"] == nil || resp["access_token"] == "" {
		t.Error("expected access_token in login response")
	}
}

// =============================================================================
// Me — auth.go:246 (remaining branches)
// =============================================================================

// TestMeNilStore verifies Me returns 503 when claims exist but the store is nil.
func TestMeNilStore(t *testing.T) {
	authHandler := handler.NewAuthHandler(nil, testJWTSecret, nil)

	ctx := newClaimsContext(&internalauth.Claims{
		Sub:      "some-user",
		Username: "someuser",
		Role:     "viewer",
		Type:     "access",
	})
	req := httptest.NewRequest("GET", "/v1/auth/me", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	authHandler.Me(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

// TestMeUserNotFound verifies Me returns 404 when the claims reference a user
// that does not exist in the store.
func TestMeUserNotFound(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	ctx := newClaimsContext(&internalauth.Claims{
		Sub:      "does-not-exist",
		Username: "ghost",
		Role:     "viewer",
		Type:     "access",
	})
	req := httptest.NewRequest("GET", "/v1/auth/me", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	authHandler.Me(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// Refresh — auth.go:145 (remaining branches)
// =============================================================================

// TestRefreshInvalidJSON verifies Refresh ignores the request body entirely:
// the refresh token is ONLY read from the cosca_refresh_token cookie (M7),
// so a request without the cookie is answered 401 regardless of the body.
func TestRefreshInvalidJSON(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	req := httptest.NewRequest("POST", "/v1/auth/refresh", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized (no refresh cookie), got %d: %s", w.Code, w.Body.String())
	}
}

// TestRefreshUserNotFound verifies Refresh returns 401 when the token is valid
// but the referenced user no longer exists.
func TestRefreshUserNotFound(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("ghostuser", "password123", "editor", "")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	pair, err := internalauth.GenerateTokenPair(*user, testJWTSecret)
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	// Remove the user so the refresh token lookup fails.
	if err := store.Delete(user.ID); err != nil {
		t.Fatalf("failed to delete test user: %v", err)
	}

	// M7: the refresh token arrives in the cosca_refresh_token cookie.
	req := httptest.NewRequest("POST", "/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "cosca_refresh_token", Value: pair.RefreshToken})
	w := httptest.NewRecorder()

	authHandler.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// Login — auth.go:46 (remaining branches)
// =============================================================================

// TestLoginUsernameTooLong verifies Login returns 400 for a long username.
func TestLoginUsernameTooLong(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	body := `{"username":"` + strings.Repeat("u", 51) + `","password":"password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestLoginPasswordTooLong verifies Login returns 400 for a long password.
func TestLoginPasswordTooLong(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	body := `{"username":"someuser","password":"` + strings.Repeat("p", 129) + `"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestLoginLockedAccount verifies Login returns 429 when the account is locked
// due to too many failed attempts.
func TestLoginLockedAccount(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	if _, err := store.Create("lockeduser", "password123", "viewer", ""); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Lock the account the same way production does: 5 consecutive failed
	// authentication attempts (mutating a GetByUsername result would no
	// longer affect the store — GetByUsername returns a defensive copy).
	for i := 0; i < 5; i++ {
		_, _ = store.Authenticate("lockeduser", "wrongpassword")
	}

	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	body := `{"username":"lockeduser","password":"password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d: %s", w.Code, w.Body.String())
	}
}

// TestLoginAfterLockExpiry verifies Login succeeds once the lock has expired.
// The expired lock is seeded through a DB-backed store so the state is real
// (a lock set 15 minutes ago that has since elapsed), not a mutation of a
// GetByUsername result — which returns a defensive copy and cannot alter the
// store.
func TestLoginAfterLockExpiry(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "users.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	seed := internalauth.NewUserStore(internalauth.UserStoreConfig{DB: db})
	if _, err := seed.Create("unlockeduser", "password123", "viewer", ""); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE users SET locked_until=?, failed_attempts=? WHERE username=?`,
		time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), 5, "unlockeduser",
	); err != nil {
		t.Fatalf("failed to set expired lock: %v", err)
	}

	// A fresh store reloads the row, so the handler observes the expired lock.
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{DB: db})

	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	body := `{"username":"unlockeduser","password":"password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK after lock expiry, got %d: %s", w.Code, w.Body.String())
	}
}
