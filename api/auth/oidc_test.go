package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/api/auth"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// testSecret is a fixed secret used for generating test tokens.
var testSecret = []byte("test-secret-key-for-testing-only-32bytes!")

// generateTestToken creates a valid access token for testing.
func generateTestToken(t *testing.T, sub, username, role string) string {
	t.Helper()
	claims := internalauth.Claims{
		Sub:      sub,
		Username: username,
		Role:     role,
		Type:     "access",
		Iat:      time.Now().Unix(),
		Exp:      time.Now().Add(24 * time.Hour).Unix(),
	}
	token, err := internalauth.GenerateToken(claims, testSecret)
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}
	return token
}

// TestMiddlewareValidToken verifies that a request with a valid Bearer token
// passes through the middleware and stores claims in the context.
func TestMiddlewareValidToken(t *testing.T) {
	token := generateTestToken(t, "user-1", "testuser", "admin")

	mw := auth.Middleware(testSecret, nil, nil)

	var capturedClaims *internalauth.Claims
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			t.Error("expected claims in context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		capturedClaims = claims
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/some-protected-path", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	if capturedClaims == nil {
		t.Fatal("expected claims to be captured")
	}
	if capturedClaims.Sub != "user-1" {
		t.Errorf("expected sub 'user-1', got '%s'", capturedClaims.Sub)
	}
	if capturedClaims.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", capturedClaims.Username)
	}
	if capturedClaims.Role != "admin" {
		t.Errorf("expected role 'admin', got '%s'", capturedClaims.Role)
	}
}

// TestMiddlewareInvalidToken verifies that a request with an invalid Bearer
// token returns 401 Unauthorized.
func TestMiddlewareInvalidToken(t *testing.T) {
	mw := auth.Middleware(testSecret, nil, nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for invalid token")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/some-protected-path", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-here")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", w.Code)
	}
}

// TestMiddlewareNoToken verifies that a request without any auth token
// returns 401 Unauthorized.
func TestMiddlewareNoToken(t *testing.T) {
	mw := auth.Middleware(testSecret, nil, nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when no token is present")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/some-protected-path", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", w.Code)
	}
}

// TestMiddlewarePublicPath verifies that requests to public paths pass
// through the middleware without authentication.
func TestMiddlewarePublicPath(t *testing.T) {
	publicPaths := []string{"/v1/auth/login", "/v1/health"}

	mw := auth.Middleware(testSecret, publicPaths, nil)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Request to a public path without any auth header.
	req := httptest.NewRequest("POST", "/v1/auth/login", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should have been called for public path")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}
}

// TestMiddlewareCookieAuth verifies that a request with a valid token in
// the cosca_access_token cookie passes through the middleware.
func TestMiddlewareCookieAuth(t *testing.T) {
	token := generateTestToken(t, "user-2", "cookieuser", "editor")

	mw := auth.Middleware(testSecret, nil, nil)

	var capturedClaims *internalauth.Claims
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			t.Error("expected claims in context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		capturedClaims = claims
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/protected", nil)
	req.AddCookie(&http.Cookie{
		Name:  "cosca_access_token",
		Value: token,
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}
	if capturedClaims == nil {
		t.Fatal("expected claims to be captured")
	}
	if capturedClaims.Username != "cookieuser" {
		t.Errorf("expected username 'cookieuser', got '%s'", capturedClaims.Username)
	}
}

// TestMiddlewareMalformedAuthHeader verifies that an Authorization header
// without the "Bearer " prefix returns 401.
func TestMiddlewareMalformedAuthHeader(t *testing.T) {
	mw := auth.Middleware(testSecret, nil, nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for malformed header")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/protected", nil)
	// Missing "Bearer " prefix.
	req.Header.Set("Authorization", "some-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", w.Code)
	}
}

// TestClaimsFromContext verifies the ClaimsFromContext helper function.
func TestClaimsFromContext(t *testing.T) {
	t.Run("claims present", func(t *testing.T) {
		claims := &internalauth.Claims{
			Sub:      "user-1",
			Username: "testuser",
			Role:     "admin",
		}
		ctx := context.WithValue(context.Background(), auth.ContextKeyClaims, claims)

		extracted, ok := auth.ClaimsFromContext(ctx)
		if !ok {
			t.Fatal("expected claims to be found")
		}
		if extracted.Sub != "user-1" {
			t.Errorf("expected sub 'user-1', got '%s'", extracted.Sub)
		}
	})

	t.Run("claims absent", func(t *testing.T) {
		ctx := context.Background()
		_, ok := auth.ClaimsFromContext(ctx)
		if ok {
			t.Error("expected claims to be absent")
		}
	})

	t.Run("wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), auth.ContextKeyClaims, "not-claims")
		_, ok := auth.ClaimsFromContext(ctx)
		if ok {
			t.Error("expected claims to be absent when value has wrong type")
		}
	})
}

// TestMiddlewareExpiredToken verifies that an expired token returns 401.
func TestMiddlewareExpiredToken(t *testing.T) {
	expiredClaims := internalauth.Claims{
		Sub:      "user-1",
		Username: "expireduser",
		Role:     "viewer",
		Type:     "access",
		Iat:      time.Now().Add(-48 * time.Hour).Unix(),
		Exp:      time.Now().Add(-24 * time.Hour).Unix(),
	}
	token, err := internalauth.GenerateToken(expiredClaims, testSecret)
	if err != nil {
		t.Fatalf("failed to generate expired test token: %v", err)
	}

	mw := auth.Middleware(testSecret, nil, nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for expired token")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", w.Code)
	}
}
