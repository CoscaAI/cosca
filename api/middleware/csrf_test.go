package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CoscaAI/cosca/api/middleware"
)

// TestCSRFSkipGet verifies that GET requests bypass CSRF checks.
func TestCSRFSkipGet(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/data", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should have been called for GET request")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}
}

// TestCSRFSkipHead verifies that HEAD requests bypass CSRF checks.
func TestCSRFSkipHead(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("HEAD", "/v1/data", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should have been called for HEAD request")
	}
}

// TestCSRFSkipOptions verifies that OPTIONS requests bypass CSRF checks.
func TestCSRFSkipOptions(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/v1/data", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should have been called for OPTIONS request")
	}
}

// TestCSRFCheckPostWithoutToken verifies that POST requests without a CSRF
// token return 403 Forbidden.
func TestCSRFCheckPostWithoutToken(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for POST without CSRF token")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/data", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", w.Code)
	}
}

// TestCSRFCheckPostWithValidToken verifies that POST requests with a valid
// CSRF cookie and header pass through.
func TestCSRFCheckPostWithValidToken(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	token := "test-csrf-token-value"
	req := httptest.NewRequest("POST", "/v1/data", nil)
	req.AddCookie(&http.Cookie{
		Name:  "csrf_token",
		Value: token,
	})
	req.Header.Set("X-CSRF-Token", token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should have been called for POST with valid CSRF token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}
}

// TestCSRFCheckPostWithMismatchedToken verifies that POST requests with a
// mismatched CSRF cookie and header return 403 Forbidden.
func TestCSRFCheckPostWithMismatchedToken(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for mismatched CSRF token")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/data", nil)
	req.AddCookie(&http.Cookie{
		Name:  "csrf_token",
		Value: "correct-token",
	})
	req.Header.Set("X-CSRF-Token", "wrong-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", w.Code)
	}
}

// TestCSRFSkipAPIKey verifies that requests with the X-API-Key header
// bypass CSRF checks.
func TestCSRFSkipAPIKey(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/data", nil)
	req.Header.Set("X-API-Key", "cosca_sk_some-api-key")
	// No CSRF cookie or header.
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should have been called for request with X-API-Key header")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}
}

// TestCSRFSkipPublicPath verifies that requests to public paths bypass
// CSRF checks even for mutating methods.
func TestCSRFSkipPublicPath(t *testing.T) {
	publicPaths := []string{"/v1/auth/login", "/v1/auth/refresh"}

	mw := middleware.CSRFMiddleware(publicPaths)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

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

// TestGenerateCSRFToken verifies that GenerateCSRFToken produces a valid
// hex-encoded token of the correct length.
func TestGenerateCSRFToken(t *testing.T) {
	token, err := middleware.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("GenerateCSRFToken failed: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty CSRF token")
	}
	// 32 bytes * 2 hex chars per byte = 64 characters.
	if len(token) != 64 {
		t.Errorf("expected 64-character hex token, got %d characters: %s", len(token), token)
	}
}

// TestCSRFCheckPutWithoutCookie verifies that PUT requests without a CSRF
// cookie return 403.
func TestCSRFCheckPutWithoutCookie(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for PUT without CSRF cookie")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("PUT", "/v1/data", nil)
	req.Header.Set("X-CSRF-Token", "some-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", w.Code)
	}
}

// TestCSRFCheckDeleteWithoutHeader verifies that DELETE requests with a
// cookie but no header return 403.
func TestCSRFCheckDeleteWithoutHeader(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for DELETE without CSRF header")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("DELETE", "/v1/data", nil)
	req.AddCookie(&http.Cookie{
		Name:  "csrf_token",
		Value: "some-token",
	})
	// No X-CSRF-Token header.
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", w.Code)
	}
}
