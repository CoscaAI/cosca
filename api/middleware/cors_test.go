package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CoscaAI/cosca/api/middleware"
)

// TestCORSPreflight verifies that OPTIONS preflight requests are handled
// with appropriate CORS headers and 204 No Content.
func TestCORSPreflight(t *testing.T) {
	mw := middleware.CORSMiddleware("http://localhost:3000")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for OPTIONS preflight")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/v1/endpoint", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content, got %d", w.Code)
	}

	// Verify CORS headers are present.
	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin 'http://localhost:3000', got '%s'", origin)
	}
	if methods := w.Header().Get("Access-Control-Allow-Methods"); methods == "" {
		t.Error("expected Access-Control-Allow-Methods to be set")
	}
	if headers := w.Header().Get("Access-Control-Allow-Headers"); headers == "" {
		t.Error("expected Access-Control-Allow-Headers to be set")
	}
}

// TestCORSAllowedOrigin verifies that a configured origin is echoed back
// in the Access-Control-Allow-Origin header.
func TestCORSAllowedOrigin(t *testing.T) {
	mw := middleware.CORSMiddleware("http://localhost:3000")

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/endpoint", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should have been called for allowed origin")
	}
	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin 'http://localhost:3000', got '%s'", origin)
	}
	// Credentials should be allowed for specific origins.
	if creds := w.Header().Get("Access-Control-Allow-Credentials"); creds != "true" {
		t.Errorf("expected 'true', got '%s'", creds)
	}
}

// TestCORSBlockedOrigin verifies that a request from an unconfigured origin
// does not receive CORS headers and the handler is still called.
func TestCORSBlockedOrigin(t *testing.T) {
	mw := middleware.CORSMiddleware("http://localhost:3000")

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/endpoint", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should still be called for blocked origin (CORS is browser-enforced)")
	}
	// No CORS header should be set for blocked origin.
	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header, got '%s'", origin)
	}
}

// TestCORSAllowAll verifies that wildcard "*" echoes the request origin
// back but does NOT set credentials — the wildcard cannot guarantee that
// only trusted origins make credentialed requests.
func TestCORSAllowAll(t *testing.T) {
	mw := middleware.CORSMiddleware("*")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/endpoint", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Wildcard "*" echoes the request origin back for convenience.
	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "http://any-origin.com" {
		t.Errorf("expected Access-Control-Allow-Origin 'http://any-origin.com', got '%s'", origin)
	}
	// Credentials must NOT be set when using wildcard — any origin
	// would be accepted, which is a security risk for credentialed requests.
	if creds := w.Header().Get("Access-Control-Allow-Credentials"); creds != "" {
		t.Errorf("expected no Access-Control-Allow-Credentials with wildcard, got '%s'", creds)
	}

	// When no Origin header is present, the literal "*" wildcard is used.
	t.Run("no origin header uses literal wildcard", func(t *testing.T) {
		req2 := httptest.NewRequest("GET", "/v1/endpoint", nil)
		// No Origin header set.
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)

		if origin := w2.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
			t.Errorf("expected literal '*', got '%s'", origin)
		}
		if creds := w2.Header().Get("Access-Control-Allow-Credentials"); creds == "true" {
			t.Error("credentials should not be set without Origin header")
		}
	})
}

// TestCORSMultipleOrigins verifies that multiple comma-separated origins work.
func TestCORSMultipleOrigins(t *testing.T) {
	mw := middleware.CORSMiddleware("http://localhost:3000, https://app.example.com")

	t.Run("first origin allowed", func(t *testing.T) {
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/v1/endpoint", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:3000" {
			t.Errorf("expected 'http://localhost:3000', got '%s'", origin)
		}
	})

	t.Run("second origin allowed", func(t *testing.T) {
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/v1/endpoint", nil)
		req.Header.Set("Origin", "https://app.example.com")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "https://app.example.com" {
			t.Errorf("expected 'https://app.example.com', got '%s'", origin)
		}
	})
}

// TestCORSNoOrigin verifies that when no Origin header is present, no CORS
// headers are set.
func TestCORSNoOrigin(t *testing.T) {
	mw := middleware.CORSMiddleware("http://localhost:3000")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/endpoint", nil)
	// No Origin header set.
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header, got '%s'", origin)
	}
}

// TestCORSEmptyAllowedOrigins verifies that an empty string for allowed
// origins results in no CORS headers being set (no origins are allowed).
func TestCORSEmptyAllowedOrigins(t *testing.T) {
	mw := middleware.CORSMiddleware("")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/endpoint", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Empty string means no origins configured — no CORS headers.
	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header, got '%s'", origin)
	}
}

// TestCORS_EmptyOriginsNoAllowOrigin verifies the fail-closed default: with
// an empty (or blank) origins list, the middleware must NOT emit an
// Access-Control-Allow-Origin header — including for preflight requests and
// for values that previously could degrade to a wildcard.
func TestCORS_EmptyOriginsNoAllowOrigin(t *testing.T) {
	for _, origins := range []string{"", ",", " , ", "   "} {
		mw := middleware.CORSMiddleware(origins)
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Simple request with an Origin header.
		req := httptest.NewRequest("GET", "/v1/endpoint", nil)
		req.Header.Set("Origin", "http://evil.com")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "" {
			t.Errorf("origins %q: simple request set Allow-Origin %q, want none", origins, origin)
		}

		// Preflight request with an Origin header.
		pre := httptest.NewRequest("OPTIONS", "/v1/endpoint", nil)
		pre.Header.Set("Origin", "http://evil.com")
		pre.Header.Set("Access-Control-Request-Method", "POST")
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, pre)
		if origin := w2.Header().Get("Access-Control-Allow-Origin"); origin != "" {
			t.Errorf("origins %q: preflight set Allow-Origin %q, want none", origins, origin)
		}
	}
}

// TestCORS_SpecificOrigin verifies that an explicitly configured origin is
// echoed back with credentials — the opt-in path for trusted frontends.
func TestCORS_SpecificOrigin(t *testing.T) {
	mw := middleware.CORSMiddleware("http://localhost:3000")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/endpoint", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:3000" {
		t.Errorf("Allow-Origin = %q, want %q", origin, "http://localhost:3000")
	}
	if creds := w.Header().Get("Access-Control-Allow-Credentials"); creds != "true" {
		t.Errorf("Allow-Credentials = %q, want %q", creds, "true")
	}

	// A non-listed origin must NOT receive CORS headers.
	req2 := httptest.NewRequest("GET", "/v1/endpoint", nil)
	req2.Header.Set("Origin", "http://evil.com")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if origin := w2.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Errorf("unlisted origin got Allow-Origin %q, want none", origin)
	}
}
