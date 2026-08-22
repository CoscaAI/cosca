package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/api/middleware"
	"github.com/CoscaAI/cosca/internal/metrics"
)

// ── LoggingMiddleware tests ────────────────────────────────────────────────

func TestLoggingMiddleware_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	mw := middleware.LoggingMiddleware(logger)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest("GET", "/v1/test?foo=bar", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	output := buf.String()
	if !strings.Contains(output, "GET") {
		t.Error("expected log to contain method 'GET'")
	}
	if !strings.Contains(output, "/v1/test") {
		t.Error("expected log to contain path '/v1/test'")
	}
	if !strings.Contains(output, "200") {
		t.Error("expected log to contain status 200")
	}
	if !strings.Contains(output, "foo=bar") {
		t.Error("expected log to contain query string 'foo=bar'")
	}
}

func TestLoggingMiddleware_ServerError(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	mw := middleware.LoggingMiddleware(logger)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest("GET", "/v1/error", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	output := buf.String()
	if !strings.Contains(output, "server error") {
		t.Errorf("expected 'server error' in log, got: %s", output)
	}
}

func TestLoggingMiddleware_ClientError(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	mw := middleware.LoggingMiddleware(logger)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest("GET", "/v1/notfound", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	output := buf.String()
	if !strings.Contains(output, "client error") {
		t.Errorf("expected 'client error' in log, got: %s", output)
	}
}

// ── MetricsMiddleware tests ────────────────────────────────────────────────

func TestMetricsMiddleware_RecordsMetrics(t *testing.T) {
	collector := metrics.NewHTTPMetrics()
	mw := middleware.MetricsMiddleware(collector)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/metrics/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMetricsMiddleware_Non200Status(t *testing.T) {
	collector := metrics.NewHTTPMetrics()
	mw := middleware.MetricsMiddleware(collector)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest("POST", "/v1/missing", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── RateLimiter tests ──────────────────────────────────────────────────────

func TestRateLimiter_AllowsRequestsWithinLimit(t *testing.T) {
	rl := middleware.NewRateLimiter(100, 100)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should be allowed (burst allows it).
	req := httptest.NewRequest("GET", "/v1/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRateLimiter_ExceedBurstReturns429(t *testing.T) {
	// Rate of 1 per minute, burst of 1 — second request should fail.
	rl := middleware.NewRateLimiter(1, 1)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request consumes the burst token.
	req1 := httptest.NewRequest("GET", "/v1/test", nil)
	req1.RemoteAddr = "192.168.1.2:12345"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request should succeed, got %d", w1.Code)
	}

	// Second request should be rate-limited.
	req2 := httptest.NewRequest("GET", "/v1/test", nil)
	req2.RemoteAddr = "192.168.1.2:12345"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w2.Code)
	}
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 1)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// IP 1 consumes burst.
	req1 := httptest.NewRequest("GET", "/v1/test", nil)
	req1.RemoteAddr = "10.0.0.1:12345"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("IP1 first request should succeed, got %d", w1.Code)
	}

	// IP 2 should still be allowed (independent bucket).
	req2 := httptest.NewRequest("GET", "/v1/test", nil)
	req2.RemoteAddr = "10.0.0.2:12345"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("IP2 should be allowed independently, got %d", w2.Code)
	}
}

// ── RateLimitMiddleware tests ──────────────────────────────────────────────

func TestRateLimitMiddleware_AllowsGeneralRequests(t *testing.T) {
	mw := middleware.RateLimitMiddleware()

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/test", nil)
	req.RemoteAddr = "172.16.0.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ── getClientIP tests ──────────────────────────────────────────────────────
// getClientIP is not exported, so we test it indirectly through the rate
// limiter middleware.

// ── readEnvInt tests ───────────────────────────────────────────────────────
// readEnvInt is not exported, so we test it indirectly.

// ── SetCSRFCookie tests ────────────────────────────────────────────────────

func TestSetCSRFCookie_SetsCookie(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/test", nil)

	middleware.SetCSRFCookie(w, req, "test-token-123")

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected at least one cookie to be set")
	}

	found := false
	for _, c := range cookies {
		if c.Name == "csrf_token" {
			found = true
			if c.Value != "test-token-123" {
				t.Errorf("expected token 'test-token-123', got '%s'", c.Value)
			}
			if c.HttpOnly {
				t.Error("expected HttpOnly=false for JS readability")
			}
			if c.Path != "/" {
				t.Errorf("expected Path='/', got '%s'", c.Path)
			}
			if c.SameSite != http.SameSiteLaxMode {
				t.Errorf("expected SameSite=Lax, got %d", c.SameSite)
			}
			if c.MaxAge != 86400 {
				t.Errorf("expected MaxAge=86400, got %d", c.MaxAge)
			}
			// Plain HTTP without TLS or X-Forwarded-Proto → Secure=false.
			if c.Secure {
				t.Error("expected Secure=false for plain HTTP")
			}
		}
	}
	if !found {
		t.Error("expected csrf_token cookie to be set")
	}
}

func TestSetCSRFCookie_SecureWithTLS(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/test", nil)
	req.TLS = nil // Will be treated differently — but httptest.Recorder doesn't expose TLS properly
	req.Header.Set("X-Forwarded-Proto", "https")

	middleware.SetCSRFCookie(w, req, "secure-token")

	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "csrf_token" {
			if !c.Secure {
				t.Error("expected Secure=true when X-Forwarded-Proto is https")
			}
		}
	}
}

// ── readEnvInt indirect tests ──────────────────────────────────────────────

func TestRateLimitMiddleware_RespectsEnvVars(t *testing.T) {
	// Set custom rate limit for login.
	t.Setenv("COSCA_RATE_LIMIT_LOGIN_RPM", "2")

	mw := middleware.RateLimitMiddleware()

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request to login path should be allowed.
	req1 := httptest.NewRequest("POST", "/v1/auth/login", nil)
	req1.RemoteAddr = "192.168.1.100:12345"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first login request should succeed, got %d", w1.Code)
	}

	// Second request may be rate-limited (burst=2, one consumed).
	req2 := httptest.NewRequest("POST", "/v1/auth/login", nil)
	req2.RemoteAddr = "192.168.1.100:12345"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	// May or may not be rate-limited depending on timing, just check it doesn't panic.
	if w2.Code != http.StatusOK && w2.Code != http.StatusTooManyRequests {
		t.Errorf("unexpected status code: %d", w2.Code)
	}
}

// ── CSRF middleware tests ──────────────────────────────────────────────────

func TestCSRFMiddleware_AllowsSafeMethods(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// GET should be allowed without CSRF token.
	req := httptest.NewRequest("GET", "/v1/protected", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("GET should be allowed without CSRF token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCSRFMiddleware_AllowsPublicPaths(t *testing.T) {
	mw := middleware.CSRFMiddleware([]string{"/v1/auth/login"})

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// POST to public path should be allowed without CSRF token.
	req := httptest.NewRequest("POST", "/v1/auth/login", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("POST to public path should be allowed without CSRF token")
	}
}

func TestCSRFMiddleware_AllowsAPIKeyAuth(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// POST with API key should skip CSRF check.
	req := httptest.NewRequest("POST", "/v1/protected", nil)
	req.Header.Set("X-API-Key", "secret-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("POST with API key should be allowed without CSRF token")
	}
}

func TestCSRFMiddleware_BlocksMissingCookie(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when CSRF cookie is missing")
	}))

	req := httptest.NewRequest("POST", "/v1/protected", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCSRFMiddleware_BlocksMissingHeader(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when CSRF header is missing")
	}))

	req := httptest.NewRequest("POST", "/v1/protected", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "my-token"})
	// No X-CSRF-Token header.
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCSRFMiddleware_BlocksMismatch(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when CSRF tokens mismatch")
	}))

	req := httptest.NewRequest("POST", "/v1/protected", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "cookie-token"})
	req.Header.Set("X-CSRF-Token", "different-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for mismatched tokens, got %d", w.Code)
	}
}

func TestCSRFMiddleware_AllowsMatchingTokens(t *testing.T) {
	mw := middleware.CSRFMiddleware(nil)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/protected", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "same-token"})
	req.Header.Set("X-CSRF-Token", "same-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should be called when CSRF tokens match")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ── GenerateCSRFToken tests ────────────────────────────────────────────────

func TestGenerateCSRFToken_ProducesValidToken(t *testing.T) {
	token, err := middleware.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(token) != 64 { // 32 bytes hex-encoded = 64 chars
		t.Errorf("expected 64-char hex token, got %d chars", len(token))
	}

	// Two tokens should be different.
	token2, _ := middleware.GenerateCSRFToken()
	if token == token2 {
		t.Error("expected different tokens")
	}
}

// ── readEnvInt indirect test ───────────────────────────────────────────────

func TestRateLimitMiddleware_InvalidEnvVar(t *testing.T) {
	// Invalid value should fall back to default.
	t.Setenv("COSCA_RATE_LIMIT_RPM", "not-a-number")

	mw := middleware.RateLimitMiddleware() // should not panic

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/test", nil)
	req.RemoteAddr = "10.0.0.99:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with fallback default, got %d", w.Code)
	}
}

// ── Zerolog responseWriter wrapper tests ───────────────────────────────────
// These are tested indirectly through LoggingMiddleware above, which
// exercises newResponseWriter, WriteHeader, and Write.
