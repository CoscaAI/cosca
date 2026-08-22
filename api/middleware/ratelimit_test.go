package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ── getClientIP tests ──────────────────────────────────────────────────────

// TestGetClientIP_RemoteAddr verifies that without any X-Forwarded-For header
// the client IP is taken from RemoteAddr with the port stripped.
func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/test", nil)
	req.RemoteAddr = "192.168.1.5:9999"

	if got := getClientIP(req); got != "192.168.1.5" {
		t.Errorf("expected 192.168.1.5, got %q", got)
	}
}

// TestGetClientIP_RemoteAddrNoPort verifies RemoteAddr without a port works.
func TestGetClientIP_RemoteAddrNoPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/test", nil)
	req.RemoteAddr = "203.0.113.10"

	if got := getClientIP(req); got != "203.0.113.10" {
		t.Errorf("expected 203.0.113.10, got %q", got)
	}
}

// TestGetClientIP_NoTrustedProxy verifies that the X-Forwarded-For header is
// IGNORED when COSCA_TRUSTED_PROXIES is not configured — the client cannot
// spoof its IP to bypass the rate limit.
func TestGetClientIP_NoTrustedProxy(t *testing.T) {
	t.Setenv("COSCA_TRUSTED_PROXIES", "")

	req := httptest.NewRequest("GET", "/v1/test", nil)
	req.RemoteAddr = "10.0.0.42:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.66")

	if got := getClientIP(req); got != "10.0.0.42" {
		t.Errorf("expected RemoteAddr 10.0.0.42 (XFF ignored), got %q", got)
	}
}

// TestGetClientIP_TrustedProxy verifies that when the direct peer is listed
// in COSCA_TRUSTED_PROXIES, the leftmost X-Forwarded-For value (the original
// client) is used.
func TestGetClientIP_TrustedProxy(t *testing.T) {
	t.Setenv("COSCA_TRUSTED_PROXIES", "127.0.0.1,10.0.0.0/8")

	req := httptest.NewRequest("GET", "/v1/test", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")

	if got := getClientIP(req); got != "203.0.113.7" {
		t.Errorf("expected leftmost XFF 203.0.113.7, got %q", got)
	}
}

// TestGetClientIP_TrustedProxyCIDR verifies CIDR networks in
// COSCA_TRUSTED_PROXIES match the direct peer.
func TestGetClientIP_TrustedProxyCIDR(t *testing.T) {
	t.Setenv("COSCA_TRUSTED_PROXIES", "192.168.0.0/16")

	req := httptest.NewRequest("GET", "/v1/test", nil)
	req.RemoteAddr = "192.168.77.1:443"
	req.Header.Set("X-Forwarded-For", "198.51.100.9")

	if got := getClientIP(req); got != "198.51.100.9" {
		t.Errorf("expected leftmost XFF 198.51.100.9, got %q", got)
	}
}

// TestGetClientIP_TrustedProxyNoXFF verifies a trusted proxy that does not
// add X-Forwarded-For falls back to the proxy's RemoteAddr.
func TestGetClientIP_TrustedProxyNoXFF(t *testing.T) {
	t.Setenv("COSCA_TRUSTED_PROXIES", "172.16.0.1")

	req := httptest.NewRequest("GET", "/v1/test", nil)
	req.RemoteAddr = "172.16.0.1:8080"

	if got := getClientIP(req); got != "172.16.0.1" {
		t.Errorf("expected fallback to proxy RemoteAddr 172.16.0.1, got %q", got)
	}
}

// ── Eviction tests ─────────────────────────────────────────────────────────

// TestRateLimiter_EvictsExpiredVisitors verifies that once the visitor map
// exceeds maxVisitors, idle entries are swept so the map cannot grow without
// bound (memory DoS protection).
func TestRateLimiter_EvictsExpiredVisitors(t *testing.T) {
	rl := NewRateLimiter(100, 100)
	rl.SetEviction(3, time.Millisecond)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Seed the visitor map with expired entries beyond maxVisitors (3).
	rl.mu.Lock()
	for i := 0; i < 5; i++ {
		rl.visitors["10.0.0.1:1000"+string(rune('0'+i))] = &visitor{tokens: 0, lastCheck: time.Now().Add(-time.Hour)}
	}
	rl.mu.Unlock()

	rl.mu.Lock()
	before := len(rl.visitors)
	rl.mu.Unlock()
	if before != 5 {
		t.Fatalf("expected 5 seeded visitors, got %d", before)
	}

	// A new request (new IP) triggers the eviction sweep because the map
	// exceeds maxVisitors (3).
	req := httptest.NewRequest("GET", "/v1/test", nil)
	req.RemoteAddr = "192.168.50.1:10000"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	rl.mu.Lock()
	after := len(rl.visitors)
	rl.mu.Unlock()

	if after != 1 {
		t.Errorf("expected expired entries swept leaving 1 active visitor, got %d", after)
	}
}

// ── Per-username login rate limit ───────────────────────────────────────────

// TestRateLimit_LoginByUsername verifies the additional per-username login
// limit: 10 rapid attempts against the same username from DIFFERENT IPs are
// allowed, and the 11th is blocked even though every request comes from a
// fresh IP (rotated/spoofed headers cannot bypass the username limit).
func TestRateLimit_LoginByUsername(t *testing.T) {
	t.Setenv("COSCA_RATE_LIMIT_LOGIN_USERNAME_RPM", "10")
	t.Setenv("COSCA_RATE_LIMIT_LOGIN_RPM", "1000") // don't trip the per-IP limit

	mw := RateLimitMiddleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 11; i++ {
		// Valid JSON body — the middleware must be able to parse the
		// username out of it for the per-username limiter to engage.
		body := `{"username":"victim","password":"guess` + strings.Repeat("x", i%3) + `"}`
		req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
		req.RemoteAddr = "203.0.113.0:1234" // same IP for all — but IP limit is high
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if i < 10 {
			if w.Code != http.StatusOK {
				t.Fatalf("attempt %d: expected 200 within limit, got %d", i+1, w.Code)
			}
		} else {
			if w.Code != http.StatusTooManyRequests {
				t.Fatalf("attempt %d: expected 429 (username limit exceeded), got %d", i+1, w.Code)
			}
		}
	}
}

// TestRateLimit_LoginByUsernameDifferentUsernames verifies that the username
// limiter is per-username: different usernames have independent buckets.
func TestRateLimit_LoginByUsernameDifferentUsernames(t *testing.T) {
	t.Setenv("COSCA_RATE_LIMIT_LOGIN_USERNAME_RPM", "2")
	t.Setenv("COSCA_RATE_LIMIT_LOGIN_RPM", "1000")

	mw := RateLimitMiddleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, username := range []string{"alice", "bob", "carol"} {
		for i := 0; i < 2; i++ {
			body := `{"username":"` + username + `","password":"x"}`
			req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
			req.RemoteAddr = "203.0.113.9:1234"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("username %s attempt %d: expected 200, got %d", username, i+1, w.Code)
			}
		}
	}
}
