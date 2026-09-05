package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/api/rest/handler"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// =============================================================================
// M5 — forced password change
// =============================================================================

// TestLogin_MustChangePassword verifies that an account flagged for a forced
// password change (e.g. the dev-mode admin) still receives 403 on login with
// a machine-readable body, while the auth cookies (the token pair) are still
// emitted so the /v1/auth/change-password flow can proceed.
func TestLogin_MustChangePassword(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("mustchange", "password123", "editor", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.RequirePasswordChange(user.ID); err != nil {
		t.Fatalf("RequirePasswordChange: %v", err)
	}

	authHandler := handler.NewAuthHandler(store, testJWTSecret, newTestAuditStore(t))

	body := `{"username":"mustchange","password":"password123"}`
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	authHandler.Login(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["error"] != "password_change_required" {
		t.Errorf("expected error 'password_change_required', got %v", resp["error"])
	}
	if resp["must_change_password"] != true {
		t.Errorf("expected must_change_password=true, got %v", resp["must_change_password"])
	}

	// Tokens are still emitted as httpOnly cookies so the user can call
	// /v1/auth/change-password with an authenticated session.
	cookies := w.Result().Cookies()
	if !hasCookie(cookies, "cosca_access_token") {
		t.Error("expected cosca_access_token cookie despite 403 (tokens still emitted)")
	}
	if !hasCookie(cookies, "cosca_refresh_token") {
		t.Error("expected cosca_refresh_token cookie despite 403 (tokens still emitted)")
	}
}

// TestChangePassword verifies POST /v1/auth/change-password validates the
// current password, swaps in the new one, clears the must_change_password
// flag, and revokes all server-side refresh tokens so every other session
// must re-authenticate (M5).
func TestChangePassword(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("changepw", "oldpass123", "editor", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ts, err := internalauth.NewTokenStoreInMemory()
	if err != nil {
		t.Fatalf("NewTokenStoreInMemory: %v", err)
	}
	defer ts.Close()

	authHandler := handler.NewAuthHandler(store, testJWTSecret, newTestAuditStore(t), ts)

	// Log in to mint a refresh session tracked server-side.
	loginBody := `{"username":"changepw","password":"oldpass123"}`
	loginReq := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	authHandler.Login(loginW, loginReq)
	if loginW.Code != http.StatusOK {
		t.Fatalf("login: got %d: %s", loginW.Code, loginW.Body.String())
	}
	refreshCookie := findCookie(loginW.Result().Cookies(), "cosca_refresh_token")
	if refreshCookie == nil {
		t.Fatal("expected cosca_refresh_token cookie after login")
	}
	refreshClaims, err := internalauth.ValidateRefreshToken(refreshCookie.Value, testJWTSecret)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if !ts.IsValid(user.ID, refreshClaims.JTI) {
		t.Fatal("refresh token should be valid before password change")
	}

	// Change the password with the correct current password.
	claims := &internalauth.Claims{Sub: user.ID, Username: user.Username, Role: user.Role, Type: "access"}
	changeBody := `{"current_password":"oldpass123","new_password":"newpass456"}`
	changeReq := httptest.NewRequest("POST", "/v1/auth/change-password", strings.NewReader(changeBody))
	changeReq = changeReq.WithContext(newClaimsContext(claims))
	changeReq.Header.Set("Content-Type", "application/json")
	changeW := httptest.NewRecorder()
	authHandler.ChangePassword(changeW, changeReq)

	if changeW.Code != http.StatusOK {
		t.Fatalf("change-password: got %d: %s", changeW.Code, changeW.Body.String())
	}

	// The must_change_password flag is cleared and old sessions revoked.
	updated, err := store.GetByID(user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.MustChangePassword {
		t.Error("expected must_change_password to be cleared after password change")
	}
	if ts.IsValid(user.ID, refreshClaims.JTI) {
		t.Error("refresh token should be revoked after password change")
	}

	// The old password no longer works; the new one does.
	if _, err := store.Authenticate(user.Username, "oldpass123"); err == nil {
		t.Error("old password must no longer authenticate")
	}
	authUser, err := store.Authenticate(user.Username, "newpass456")
	if err != nil {
		t.Fatalf("new password should authenticate: %v", err)
	}
	if authUser.MustChangePassword {
		t.Error("MustChangePassword must be false after password change")
	}
}

// =============================================================================
// M7 — refresh is cookie-only
// =============================================================================

// TestRefresh_RequiresCookie verifies the M7 contract: without the
// cosca_refresh_token cookie the refresh endpoint always answers 401
// (never 400), so a missing cookie is not confused with a body error.
func TestRefresh_RequiresCookie(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	authHandler := handler.NewAuthHandler(store, testJWTSecret, newTestAuditStore(t))

	req := httptest.NewRequest("POST", "/v1/auth/refresh", nil)
	w := httptest.NewRecorder()

	authHandler.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without refresh cookie, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRefresh_CookieWorks verifies a valid cosca_refresh_token cookie yields
// a rotated token pair (200).
func TestRefresh_CookieWorks(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("cookieok", "password123", "viewer", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ts, err := internalauth.NewTokenStoreInMemory()
	if err != nil {
		t.Fatalf("NewTokenStoreInMemory: %v", err)
	}
	defer ts.Close()

	authHandler := handler.NewAuthHandler(store, testJWTSecret, newTestAuditStore(t), ts)

	pair, err := internalauth.GenerateTokenPair(*user, testJWTSecret)
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	refreshClaims, err := internalauth.ValidateRefreshToken(pair.RefreshToken, testJWTSecret)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if err := ts.StoreRefresh(user.ID, refreshClaims.JTI, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("StoreRefresh: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "cosca_refresh_token", Value: pair.RefreshToken})
	w := httptest.NewRecorder()

	authHandler.Refresh(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid refresh cookie, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["access_token"] == nil || resp["access_token"] == "" {
		t.Error("expected access_token in cookie-based refresh response")
	}
	if resp["refresh_token"] == nil || resp["refresh_token"] == "" {
		t.Error("expected refresh_token in cookie-based refresh response")
	}
}
