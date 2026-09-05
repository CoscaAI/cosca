package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apiauth "github.com/CoscaAI/cosca/api/auth"
	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/internal/audit"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// testJWTSecret is used for handler auth tests.
var testJWTSecret = []byte("test-handler-secret-for-testing-only-32bytes!")

// newTestAuditStore creates an in-memory SQLite audit store for testing.
func newTestAuditStore(t *testing.T) *audit.Store {
	t.Helper()
	store, err := audit.NewStore(":memory:?cache=shared")
	if err != nil {
		// If SQLite with URI support fails, try with a temp file.
		tmpDir := t.TempDir()
		store, err = audit.NewStore(tmpDir + "/test-audit.db")
		if err != nil {
			t.Fatalf("failed to create test audit store: %v", err)
		}
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// TestLoginValidCredentials verifies login with valid credentials returns
// 200 OK with a token pair.
func TestLoginValidCredentials(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	_, err := store.Create("testuser", "password123", "admin", "")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	body := `{"username":"testuser","password":"password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["access_token"] == nil || resp["access_token"] == "" {
		t.Error("expected access_token in response")
	}
	if resp["refresh_token"] == nil || resp["refresh_token"] == "" {
		t.Error("expected refresh_token in response")
	}
	if resp["expires_in"] == nil {
		t.Error("expected expires_in in response")
	}

	// Verify cookies are set.
	cookies := w.Result().Cookies()
	if !hasCookie(cookies, "cosca_access_token") {
		t.Error("expected cosca_access_token cookie")
	}
	if !hasCookie(cookies, "cosca_refresh_token") {
		t.Error("expected cosca_refresh_token cookie")
	}
	if !hasCookie(cookies, "cosca_auth_state") {
		t.Error("expected cosca_auth_state cookie")
	}
}

// TestLoginInvalidCredentials verifies login with wrong password returns 401.
func TestLoginInvalidCredentials(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	_, err := store.Create("testuser", "password123", "admin", "")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	body := `{"username":"testuser","password":"wrongpassword"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
	}
}

// TestLoginMissingBody verifies login with a missing body returns 400.
func TestLoginMissingBody(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	// Empty body.
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestLoginEmptyUsername verifies login with empty username returns 400.
func TestLoginEmptyUsername(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	body := `{"username":"","password":"password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestLoginNonExistentUser verifies login with a non-existent user returns 401.
func TestLoginNonExistentUser(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	body := `{"username":"nosuchuser","password":"password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
	}
}

// TestMeWithValidToken verifies the Me endpoint returns user data when
// valid claims are present in the request context.
func TestMeWithValidToken(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("meuser", "password123", "editor", "me@example.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	// Build claims matching the created user.
	claims := &internalauth.Claims{
		Sub:      user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     "access",
	}
	ctx := context.WithValue(context.Background(), apiauth.ContextKeyClaims, claims)

	req := httptest.NewRequest("GET", "/v1/auth/me", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	authHandler.Me(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var respUser internalauth.User
	if err := json.Unmarshal(w.Body.Bytes(), &respUser); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if respUser.Username != "meuser" {
		t.Errorf("expected username 'meuser', got '%s'", respUser.Username)
	}
	if respUser.Role != "editor" {
		t.Errorf("expected role 'editor', got '%s'", respUser.Role)
	}
	// Password hash should NOT be exposed.
	if respUser.PasswordHash != "" {
		t.Error("password hash should not be exposed in Me response")
	}
}

// TestMeDoesNotZeroStoredHash verifies the Me endpoint sanitizes the hash
// only on its local copy of the user, never mutating the store's record.
func TestMeDoesNotZeroStoredHash(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("mehash", "password123", "editor", "mehash@example.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	storedHash := user.PasswordHash
	if storedHash == "" {
		t.Fatal("expected a non-empty hash after Create")
	}

	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	claims := &internalauth.Claims{
		Sub:      user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     "access",
	}
	ctx := context.WithValue(context.Background(), apiauth.ContextKeyClaims, claims)
	req := httptest.NewRequest("GET", "/v1/auth/me", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	authHandler.Me(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	// The response must never carry the hash.
	var respUser internalauth.User
	if err := json.Unmarshal(w.Body.Bytes(), &respUser); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if respUser.PasswordHash != "" {
		t.Error("password hash must not be exposed in Me response")
	}

	// And the store's hash must survive the Me call intact.
	after, err := store.GetByID(user.ID)
	if err != nil {
		t.Fatalf("GetByID after Me failed: %v", err)
	}
	if after.PasswordHash != storedHash {
		t.Errorf("Me() zeroed the stored hash: want %q, got %q", storedHash, after.PasswordHash)
	}
}

// TestMeWithoutClaims verifies the Me endpoint returns 401 when no claims
// are present in the context.
func TestMeWithoutClaims(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	req := httptest.NewRequest("GET", "/v1/auth/me", nil)
	w := httptest.NewRecorder()

	authHandler.Me(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRefreshWithValidRefreshToken verifies the Refresh endpoint returns
// a new token pair when provided with a valid refresh token.
//
// M7: the refresh token is ONLY accepted from the cosca_refresh_token
// httpOnly cookie — the body-based flow was removed. The token must also be
// registered in the server-side token store so rotation/reuse detection works.
func TestRefreshWithValidRefreshToken(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("refreshuser", "password123", "editor", "")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	ts, err := internalauth.NewTokenStoreInMemory()
	if err != nil {
		t.Fatalf("failed to create token store: %v", err)
	}
	defer ts.Close()

	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore, ts)

	// Generate a valid refresh token and register it server-side.
	pair, err := internalauth.GenerateTokenPair(*user, testJWTSecret)
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}
	refreshClaims, err := internalauth.ValidateRefreshToken(pair.RefreshToken, testJWTSecret)
	if err != nil {
		t.Fatalf("failed to validate refresh token: %v", err)
	}
	if err := ts.StoreRefresh(user.ID, refreshClaims.JTI, time.Now().Add(7*24*time.Hour)); err != nil {
		t.Fatalf("failed to store refresh token: %v", err)
	}

	// The refresh token arrives in the cookie, never in a JSON body.
	req := httptest.NewRequest("POST", "/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "cosca_refresh_token", Value: pair.RefreshToken})
	w := httptest.NewRecorder()

	authHandler.Refresh(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["access_token"] == nil || resp["access_token"] == "" {
		t.Error("expected new access_token in refresh response")
	}
	if resp["refresh_token"] == nil || resp["refresh_token"] == "" {
		t.Error("expected new refresh_token in refresh response")
	}

	// Verify the new tokens are valid. Note: tokens generated within the
	// same second may be identical (same claims), which is acceptable.
	if resp["access_token"].(string) == "" {
		t.Error("expected non-empty access token")
	}
}

// TestRefreshWithInvalidToken verifies the Refresh endpoint returns 401
// when the cookie carries an invalid refresh token.
func TestRefreshWithInvalidToken(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	req := httptest.NewRequest("POST", "/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "cosca_refresh_token", Value: "invalid-refresh-token"})
	w := httptest.NewRecorder()

	authHandler.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid or expired refresh token") {
		t.Errorf("expected 'invalid or expired refresh token' error, got %s", w.Body.String())
	}
}

// TestRefreshWithoutToken verifies the Refresh endpoint returns 401 when no
// refresh token cookie is present (M7 — cookie-only; never 400, so a missing
// cookie is not confused with a body validation error).
func TestRefreshWithoutToken(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	req := httptest.NewRequest("POST", "/v1/auth/refresh", nil)
	w := httptest.NewRecorder()

	authHandler.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "refresh token missing") {
		t.Errorf("expected 'refresh token missing' error, got %s", w.Body.String())
	}
}

// TestRefreshWithAccessToken verifies the Refresh endpoint rejects a
// token with type "access" instead of "refresh" (A4), even when it arrives
// in the cosca_refresh_token cookie.
func TestRefreshWithAccessToken(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("accesstokuser", "password123", "viewer", "")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	// Generate a pair but use the access token on the refresh endpoint.
	pair, err := internalauth.GenerateTokenPair(*user, testJWTSecret)
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "cosca_refresh_token", Value: pair.AccessToken})
	w := httptest.NewRecorder()

	authHandler.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized (access token used as refresh token), got %d: %s", w.Code, w.Body.String())
	}
}

// TestRefreshViaCookie verifies the Refresh endpoint can read the refresh
// token from the cosca_refresh_token cookie.
func TestRefreshViaCookie(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("cookieuser", "password123", "editor", "")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	pair, err := internalauth.GenerateTokenPair(*user, testJWTSecret)
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	// Send the refresh token via cookie, no JSON body.
	req := httptest.NewRequest("POST", "/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "cosca_refresh_token",
		Value: pair.RefreshToken,
	})
	w := httptest.NewRecorder()

	authHandler.Refresh(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for cookie-based refresh, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["access_token"] == nil || resp["access_token"] == "" {
		t.Error("expected access_token in cookie-based refresh response")
	}
}

// TestLogout verifies the Logout endpoint clears cookies and returns 200.
func TestLogout(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	req := httptest.NewRequest("POST", "/v1/auth/logout", nil)
	w := httptest.NewRecorder()

	authHandler.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	// Verify cookies are cleared (MaxAge=-1).
	cookies := w.Result().Cookies()
	accessCookie := findCookie(cookies, "cosca_access_token")
	if accessCookie == nil {
		t.Error("expected cosca_access_token cookie to be cleared")
	} else if accessCookie.MaxAge != -1 {
		t.Errorf("expected MaxAge=-1 for cleared cookie, got %d", accessCookie.MaxAge)
	}

	refreshCookie := findCookie(cookies, "cosca_refresh_token")
	if refreshCookie == nil {
		t.Error("expected cosca_refresh_token cookie to be cleared")
	} else if refreshCookie.MaxAge != -1 {
		t.Errorf("expected MaxAge=-1, got %d", refreshCookie.MaxAge)
	}

	authCookie := findCookie(cookies, "cosca_auth_state")
	if authCookie == nil {
		t.Error("expected cosca_auth_state cookie to be cleared")
	} else if authCookie.MaxAge != -1 {
		t.Errorf("expected MaxAge=-1, got %d", authCookie.MaxAge)
	}
}

// TestCSRFTokenGeneration verifies the CSRF handler generates a token
// and sets the cookie.
func TestCSRFTokenGeneration(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(store, testJWTSecret, auditStore)

	req := httptest.NewRequest("GET", "/v1/csrf-token", nil)
	w := httptest.NewRecorder()

	authHandler.CSRF(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["token"] == "" {
		t.Error("expected CSRF token in response body")
	}

	// Verify the cookie is set.
	cookies := w.Result().Cookies()
	if !hasCookie(cookies, "csrf_token") {
		t.Error("expected csrf_token cookie to be set")
	}
}

// TestLoginNilStore verifies login returns 503 when the store is nil.
func TestLoginNilStore(t *testing.T) {
	auditStore := newTestAuditStore(t)
	authHandler := handler.NewAuthHandler(nil, testJWTSecret, auditStore)

	body := `{"username":"test","password":"test"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d", w.Code)
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// hasCookie checks if a cookie with the given name exists in the slice.
func hasCookie(cookies []*http.Cookie, name string) bool {
	return findCookie(cookies, name) != nil
}

// findCookie finds a cookie by name, or returns nil.
func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}
