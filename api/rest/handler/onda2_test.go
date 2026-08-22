package handler_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/api/rest/handler"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// ── A4: refresh rotation & logout revocation ────────────────────────────────

// TestRefresh_RotatesAndRevokes verifies that refreshing rotates the old
// token (retires it) and registers the new one.
//
// M7: the refresh token is only accepted from the cosca_refresh_token cookie,
// so both requests send it via cookie instead of a JSON body.
func TestRefresh_RotatesAndRevokes(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("rotator", "password123", "editor", "rot@x.com")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ts, err := internalauth.NewTokenStoreInMemory()
	if err != nil {
		t.Fatalf("NewTokenStoreInMemory: %v", err)
	}
	defer ts.Close()

	h := handler.NewAuthHandler(store, testJWTSecret, nil, ts)

	pair, err := internalauth.GenerateTokenPair(*user, testJWTSecret)
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}

	// Extract the real JTI from the refresh token and store it as baseline.
	refreshClaims, err := internalauth.ValidateRefreshToken(pair.RefreshToken, testJWTSecret)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if err := ts.StoreRefresh(user.ID, refreshClaims.JTI, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("StoreRefresh: %v", err)
	}

	refreshCookie := func(token string) *http.Cookie {
		return &http.Cookie{Name: "cosca_refresh_token", Value: token}
	}

	// First refresh with the cookie — should succeed.
	req1 := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	req1.AddCookie(refreshCookie(pair.RefreshToken))
	w1 := httptest.NewRecorder()
	h.Refresh(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first refresh: got %d, want 200 (body: %s)", w1.Code, w1.Body.String())
	}

	// Reusing the ORIGINAL refresh token must now fail (rotated out): the
	// store detects the reuse (A4) and revokes all sessions → 401.
	req2 := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	req2.AddCookie(refreshCookie(pair.RefreshToken))
	w2 := httptest.NewRecorder()
	h.Refresh(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("reusing an already-rotated refresh token: got %d, want 401", w2.Code)
	}
}

// TestLogout_Revokes verifies logout revokes the user's refresh tokens.
func TestLogout_Revokes(t *testing.T) {
	store := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	user, err := store.Create("logouttest", "password123", "editor", "lo@x.com")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ts, err := internalauth.NewTokenStoreInMemory()
	if err != nil {
		t.Fatalf("NewTokenStoreInMemory: %v", err)
	}
	defer ts.Close()

	h := handler.NewAuthHandler(store, testJWTSecret, nil, ts)

	claims := &internalauth.Claims{Sub: user.ID, Username: user.Username, Role: "editor", Type: "access"}
	if err := ts.StoreRefresh(user.ID, "jti-active", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("StoreRefresh: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	req = req.WithContext(newClaimsContext(claims))
	w := httptest.NewRecorder()
	h.Logout(w, req)

	if ts.IsValid(user.ID, "jti-active") {
		t.Error("refresh token still valid after logout — must be revoked")
	}
}

// ── A5: body limits ─────────────────────────────────────────────────────────

// TestRun_BodyTooLarge verifies oversized bodies are rejected.
func TestRun_BodyTooLarge(t *testing.T) {
	h := handler.NewRunHandler(nil, nil, nil)

	big := strings.Repeat("x", 6<<20) // 6 MiB > 5 MiB limit
	body := `{"prompt":"` + big + `"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/run", bytes.NewBufferString(body))
	req = req.WithContext(newClaimsContext(editorClaims()))
	w := httptest.NewRecorder()
	h.Execute(w, req)

	if w.Code != http.StatusRequestEntityTooLarge && w.Code != http.StatusBadRequest {
		t.Errorf("oversized body: got %d, want 413 or 400", w.Code)
	}
}

// ── A6: memory ownership scoping ────────────────────────────────────────────

// TestMemorySearch_OwnerScoped verifies a user only sees their own records
// plus system-global records (owner == "").
func TestMemorySearch_OwnerScoped(t *testing.T) {
	eng := newTestMemoryEngine(t)
	h := handler.NewMemoryHandler(eng, nil)

	ctxA := newClaimsContext(&internalauth.Claims{Sub: "user-a", Username: "a", Role: "editor", Type: "access"})
	ctxB := newClaimsContext(&internalauth.Claims{Sub: "user-b", Username: "b", Role: "editor", Type: "access"})

	storeBody := `{"type":"decision","key":"secret-a","value":"data-a","content":"a-record","layer":"session"}`
	reqS := httptest.NewRequest(http.MethodPost, "/v1/memory/store", strings.NewReader(storeBody))
	reqS = reqS.WithContext(ctxA)
	wS := httptest.NewRecorder()
	h.Store(wS, reqS)
	if wS.Code != http.StatusOK && wS.Code != http.StatusCreated {
		t.Fatalf("store: got %d (body: %s)", wS.Code, wS.Body.String())
	}

	reqB := httptest.NewRequest(http.MethodGet, "/v1/memory/search?query=a-record", nil)
	reqB = reqB.WithContext(ctxB)
	wB := httptest.NewRecorder()
	h.Search(wB, reqB)

	if strings.Contains(wB.Body.String(), "a-record") {
		t.Error("user B saw user A's private memory record")
	}
}
