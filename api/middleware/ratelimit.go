package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ── Rate limiter configuration via environment variables ──────────────────
//
//   COSCA_RATE_LIMIT_RPM                 requests per minute per IP (default: 100)
//   COSCA_RATE_LIMIT_LOGIN_RPM           login-specific limit per IP (default: 5)
//   COSCA_RATE_LIMIT_LOGIN_USERNAME_RPM  login-specific limit per username (default: 10)
//   COSCA_RATE_LIMIT_MAX_VISITORS        max tracked visitors before eviction (default: 10000)
//   COSCA_TRUSTED_PROXIES                comma-separated IPs/CIDRs whose X-Forwarded-For is trusted
//
// COSCA_TRUSTED_PROXIES is EMPTY by default — the X-Forwarded-For header is
// IGNORED and the rate limiter keys on RemoteAddr. This prevents clients from
// bypassing the rate limit by rotating a spoofed X-Forwarded-For header. Only
// when the direct peer (RemoteAddr) is a listed proxy/load balancer is the
// leftmost X-Forwarded-For value (the original client) used.

const (
	defaultRateRPM          = 100
	defaultLoginRateRPM     = 5
	defaultLoginUsernameRPM = 10
	defaultMaxVisitors      = 10000
	defaultEvictionWindow   = 2 * time.Minute
)

// RateLimiter implements an in-memory token bucket rate limiter using only
// the standard library. Each unique client key (IP or username) is tracked
// independently.
//
// The limiter refills tokens continuously at a rate of `rate` requests per
// minute, with a maximum burst size of `burst`. When the bucket is empty,
// requests receive HTTP 429 Too Many Requests.
//
// To bound memory usage, the visitor map is evicted when it grows beyond
// maxVisitors: entries that have not been seen within evictionWindow are
// removed (an actively-refreshing visitor is never evicted).
type RateLimiter struct {
	mu             sync.Mutex
	visitors       map[string]*visitor
	rate           int // requests per minute per key
	burst          int // maximum burst size per key
	maxVisitors    int
	evictionWindow time.Duration
}

type visitor struct {
	tokens    float64   // current token count (fractional for smooth refill)
	lastCheck time.Time // last time tokens were refilled
}

// NewRateLimiter creates a rate limiter with the given rate (requests per
// minute per client key) and burst size.
func NewRateLimiter(rate, burst int) *RateLimiter {
	if burst <= 0 {
		burst = rate
	}
	return &RateLimiter{
		visitors:       make(map[string]*visitor),
		rate:           rate,
		burst:          burst,
		maxVisitors:    defaultMaxVisitors,
		evictionWindow: defaultEvictionWindow,
	}
}

// SetEviction configures visitor-map eviction: maxVisitors bounds the number
// of tracked keys before expired entries are swept, and window is the idle
// time after which a visitor entry is considered expired. Values <= 0 keep
// the current setting.
func (rl *RateLimiter) SetEviction(maxVisitors int, window time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if maxVisitors > 0 {
		rl.maxVisitors = maxVisitors
	}
	if window > 0 {
		rl.evictionWindow = window
	}
}

// Allow consumes one token for the given key. It returns true when the
// request is within the rate limit and false when it must be rejected.
// The key is typically a client IP or a login username.
func (rl *RateLimiter) Allow(key string) bool {
	if key == "" {
		key = "unknown"
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Bound memory usage: once the visitor map exceeds maxVisitors, sweep
	// entries whose owners have been idle for longer than the eviction
	// window (DoS protection — a rotating header/key can no longer grow the
	// map without bound).
	if len(rl.visitors) > rl.maxVisitors {
		rl.evictExpired()
	}

	v, exists := rl.visitors[key]
	if !exists {
		v = &visitor{tokens: float64(rl.burst), lastCheck: time.Now()}
		rl.visitors[key] = v
	}

	// Refill tokens proportionally to elapsed time. Using float64
	// tokens avoids the coarse granularity of integer-minutes arithmetic.
	elapsed := time.Since(v.lastCheck).Seconds()
	v.tokens += elapsed * float64(rl.rate) / 60.0
	if v.tokens > float64(rl.burst) {
		v.tokens = float64(rl.burst)
	}
	v.lastCheck = time.Now()

	if v.tokens >= 1.0 {
		v.tokens--
		return true
	}
	return false
}

// evictExpired removes visitor entries that have been idle longer than the
// eviction window. The caller must hold rl.mu.
func (rl *RateLimiter) evictExpired() {
	now := time.Now()
	for key, v := range rl.visitors {
		if now.Sub(v.lastCheck) > rl.evictionWindow {
			delete(rl.visitors, key)
		}
	}
}

// Middleware returns an HTTP middleware that limits requests per client IP
// using the configured rate and burst.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(getClientIP(r)) {
			writeRateLimited(w, rl)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeRateLimited writes the HTTP 429 response used when a rate limit is
// exceeded, informing the client when it may retry.
func writeRateLimited(w http.ResponseWriter, rl *RateLimiter) {
	w.Header().Set("Retry-After", fmt.Sprintf("%.0f", 60.0/(float64(rl.rate)/float64(rl.burst))))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
}

// ── Path-aware rate limiting ──────────────────────────────────────────────

// loginPaths defines the paths that get the stricter login rate limit.
var loginPaths = map[string]bool{
	"/v1/auth/login": true,
}

// RateLimitMiddleware returns an HTTP middleware that applies different rate
// limits based on the request path:
//
//   - Login endpoint (/v1/auth/login): 5 requests/minute per IP AND
//     10 requests/minute per username (brute force protection)
//   - All other endpoints: 100 requests/minute
//
// All limits are configurable via environment variables:
//
//	COSCA_RATE_LIMIT_RPM                 — default rate (default: 100)
//	COSCA_RATE_LIMIT_LOGIN_RPM           — login rate per IP (default: 5)
//	COSCA_RATE_LIMIT_LOGIN_USERNAME_RPM  — login rate per username (default: 10)
//	COSCA_RATE_LIMIT_MAX_VISITORS        — visitor map cap before eviction (default: 10000)
//
// The per-username limit mitigates distributed brute-force attempts: even if
// an attacker rotates spoofed X-Forwarded-For headers across many IPs, the
// attempts against a single username are still throttled (which also protects
// against lockout-DoS of real accounts).
func RateLimitMiddleware() func(http.Handler) http.Handler {
	// Read environment configuration.
	defaultRPM := readEnvInt("COSCA_RATE_LIMIT_RPM", defaultRateRPM)
	loginRPM := readEnvInt("COSCA_RATE_LIMIT_LOGIN_RPM", defaultLoginRateRPM)
	loginUsernameRPM := readEnvInt("COSCA_RATE_LIMIT_LOGIN_USERNAME_RPM", defaultLoginUsernameRPM)
	maxVisitors := readEnvInt("COSCA_RATE_LIMIT_MAX_VISITORS", defaultMaxVisitors)

	generalLimiter := NewRateLimiter(defaultRPM, defaultRPM)
	generalLimiter.SetEviction(maxVisitors, 0)

	loginLimiter := NewRateLimiter(loginRPM, loginRPM)
	loginLimiter.SetEviction(maxVisitors, 0)

	loginUsernameLimiter := NewRateLimiter(loginUsernameRPM, loginUsernameRPM)
	loginUsernameLimiter.SetEviction(maxVisitors, 0)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if loginPaths[r.URL.Path] {
				// Per-IP login limit.
				if !loginLimiter.Allow(getClientIP(r)) {
					writeRateLimited(w, loginLimiter)
					return
				}
				// Additional per-username login limit. The body is read and
				// restored so the downstream handler still sees it.
				if username := loginUsername(r); username != "" {
					if !loginUsernameLimiter.Allow(username) {
						writeRateLimited(w, loginUsernameLimiter)
						return
					}
				}
				next.ServeHTTP(w, r)
				return
			}

			if !generalLimiter.Allow(getClientIP(r)) {
				writeRateLimited(w, generalLimiter)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// loginUsername extracts the username field from a login request body and
// restores the request body so downstream handlers can still read it.
func loginUsername(r *http.Request) string {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var req struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}
	return strings.TrimSpace(req.Username)
}

// readEnvInt reads an integer environment variable or returns a fallback.
func readEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// getClientIP extracts the client IP from the request.
//
// Security: the X-Forwarded-For header is ONLY honored when the direct peer
// (RemoteAddr) is a proxy/load balancer explicitly listed in the
// COSCA_TRUSTED_PROXIES environment variable (comma-separated IPs or CIDR
// networks). By default the header is ignored entirely and the rate limiter
// keys on RemoteAddr, so clients cannot bypass limits by spoofing/rotating
// X-Forwarded-For. When the peer is a trusted proxy, the leftmost address in
// the X-Forwarded-For chain is the original client.
func getClientIP(r *http.Request) string {
	remoteIP := stripPort(r.RemoteAddr)

	// Only trust X-Forwarded-For when the direct connection comes from a
	// configured trusted proxy.
	if isTrustedProxy(remoteIP) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// When multiple proxies are chained, the leftmost address is
			// the original client.
			if idx := strings.IndexByte(xff, ','); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
	}

	return remoteIP
}

// stripPort removes the port portion from a host:port address (e.g. from
// http.Request.RemoteAddr). Addresses without a port are returned as-is.
func stripPort(addr string) string {
	addr = strings.TrimSpace(addr)
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	// Bare IP (no port) or an already-stripped host.
	if ip := net.ParseIP(addr); ip != nil {
		return ip.String()
	}
	return addr
}

// isTrustedProxy reports whether remoteIP is listed in the
// COSCA_TRUSTED_PROXIES environment variable. Entries may be individual IPs
// (e.g. "10.0.0.1") or CIDR networks (e.g. "10.0.0.0/8").
func isTrustedProxy(remoteIP string) bool {
	raw := os.Getenv("COSCA_TRUSTED_PROXIES")
	if strings.TrimSpace(raw) == "" {
		return false
	}

	ip := net.ParseIP(remoteIP)
	if ip == nil {
		return false
	}

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, network, err := net.ParseCIDR(entry)
			if err == nil && network.Contains(ip) {
				return true
			}
			continue
		}
		if peer := net.ParseIP(entry); peer != nil && peer.Equal(ip) {
			return true
		}
	}
	return false
}
