package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CoscaAI/cosca/api/middleware"
)

// TestSecurityHeaders verifies that security headers are present in the
// response for a normal HTTP request.
func TestSecurityHeaders(t *testing.T) {
	mw := middleware.SecurityHeadersMiddleware()

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/endpoint", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Verify each security header is present.
	if h := w.Header().Get("X-Frame-Options"); h != "DENY" {
		t.Errorf("expected X-Frame-Options 'DENY', got '%s'", h)
	}
	if h := w.Header().Get("X-Content-Type-Options"); h != "nosniff" {
		t.Errorf("expected X-Content-Type-Options 'nosniff', got '%s'", h)
	}
	if h := w.Header().Get("Referrer-Policy"); h != "strict-origin-when-cross-origin" {
		t.Errorf("expected Referrer-Policy 'strict-origin-when-cross-origin', got '%s'", h)
	}
	if h := w.Header().Get("Permissions-Policy"); h == "" {
		t.Error("expected Permissions-Policy to be set")
	}
	if h := w.Header().Get("X-Permitted-Cross-Domain-Policies"); h != "none" {
		t.Errorf("expected X-Permitted-Cross-Domain-Policies 'none', got '%s'", h)
	}
}

// TestCSPHeader verifies that the Content-Security-Policy header is present.
func TestCSPHeader(t *testing.T) {
	mw := middleware.SecurityHeadersMiddleware()

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/endpoint", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("expected Content-Security-Policy header to be set")
	}
	// Verify key directives are present in the CSP.
	if !contains(csp, "default-src") {
		t.Error("expected CSP to contain 'default-src'")
	}
	if !contains(csp, "frame-src 'none'") {
		t.Error("expected CSP to contain 'frame-src'")
	}
	if !contains(csp, "object-src 'none'") {
		t.Error("expected CSP to contain 'object-src'")
	}
	if !contains(csp, "base-uri 'self'") {
		t.Error("expected CSP to contain 'base-uri'")
	}
}

// TestHSTSHeader verifies that the Strict-Transport-Security header is set
// when the X-Forwarded-Proto header indicates HTTPS.
func TestHSTSHeaderWithForwardedProto(t *testing.T) {
	mw := middleware.SecurityHeadersMiddleware()

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/endpoint", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("expected Strict-Transport-Security header to be set when X-Forwarded-Proto is https")
	}
}

// TestNoHSTSForPlainHTTP verifies that HSTS is NOT set for plain HTTP
// requests without TLS or X-Forwarded-Proto.
func TestNoHSTSForPlainHTTP(t *testing.T) {
	mw := middleware.SecurityHeadersMiddleware()

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/endpoint", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if hsts := w.Header().Get("Strict-Transport-Security"); hsts != "" {
		t.Errorf("expected no Strict-Transport-Security header for plain HTTP, got '%s'", hsts)
	}
}

// TestHSTSWithForceFlag verifies that HSTS is set when the environment
// variable COSCA_FORCE_HSTS is true, even without TLS.
func TestHSTSWithForceFlag(t *testing.T) {
	t.Setenv("COSCA_FORCE_HSTS", "true")

	mw := middleware.SecurityHeadersMiddleware()

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/endpoint", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("expected Strict-Transport-Security header when COSCA_FORCE_HSTS is true")
	}
}

// TestSecurityHeadersAllPresent verifies that the handler itself still runs
// and returns the correct status code after security headers are set.
func TestSecurityHeadersHandlerRuns(t *testing.T) {
	mw := middleware.SecurityHeadersMiddleware()

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("POST", "/v1/endpoint", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should have been called")
	}
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201 Created, got %d", w.Code)
	}
	// Security headers should still be present even on non-200.
	if h := w.Header().Get("X-Frame-Options"); h != "DENY" {
		t.Errorf("expected X-Frame-Options on non-200 response, got '%s'", h)
	}
}

// contains is a simple helper to check if a string contains a substring.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
