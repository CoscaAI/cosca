// Package middleware provides HTTP middleware for the Cosca REST API.
package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

// securityHeaders applied to every API response.
//
// These headers follow OWASP best practices and help protect against common
// web vulnerabilities (clickjacking, MIME-sniffing, side-channel leaks).
var securityHeaders = map[string]string{
	"X-Frame-Options":                   "DENY",
	"X-Content-Type-Options":            "nosniff",
	"Referrer-Policy":                   "strict-origin-when-cross-origin",
	"Permissions-Policy":                "camera=(), microphone=(), geolocation=()",
	"X-Permitted-Cross-Domain-Policies": "none",
}

// cspTemplate is the base Content-Security-Policy value applied to API responses.
//
//   - 'unsafe-inline' and 'unsafe-eval' are included because Next.js requires
//     them during development — this is a known tradeoff documented in the
//     OWASP Top 10 review (A05).
//   - connect-src is configurable via COSCA_CSP_CONNECT_SRC env var, falling
//     back to localhost:14120 for development.
//   - frame-src 'none' and object-src 'none' prevent clickjacking vectors.
const cspTemplate = "" +
	"default-src 'self'; " +
	"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' blob: data:; " +
	"font-src 'self'; " +
	"connect-src %s; " +
	"frame-src 'none'; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'"

// defaultCSPConnectSrc is the default value for the CSP connect-src directive
// used when COSCA_CSP_CONNECT_SRC is not set. Localhost:14120 covers the
// default REST API address and ws:// for WebSocket-based SSE streaming.
const defaultCSPConnectSrc = "'self' http://localhost:14120 ws://localhost:14120"

// buildCSPHeader returns the full CSP header string with the configured
// connect-src directive. Reads COSCA_CSP_CONNECT_SRC from the environment;
// falls back to defaultCSPConnectSrc if not set.
func buildCSPHeader() string {
	connectSrc := os.Getenv("COSCA_CSP_CONNECT_SRC")
	if connectSrc == "" {
		connectSrc = defaultCSPConnectSrc
	}
	return fmt.Sprintf(cspTemplate, connectSrc)
}

// hstsHeader is the HTTP Strict-Transport-Security value.
// Only included when HTTPS is detected (TLS connection or X-Forwarded-Proto).
const hstsHeader = "max-age=31536000; includeSubDomains"

// SecurityHeadersMiddleware returns an HTTP middleware that adds security
// headers (CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy,
// Permissions-Policy) to every API response.
//
// HSTS is added only when the request indicates HTTPS (via TLS or the
// X-Forwarded-Proto header), so it is safe for local development.
func SecurityHeadersMiddleware() func(http.Handler) http.Handler {
	// Check environment override for HSTS in development.
	forceHSTS := os.Getenv("COSCA_FORCE_HSTS") == "true"
	// Build the CSP header once at middleware creation time.
	cspValue := buildCSPHeader()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Security headers.
			for k, v := range securityHeaders {
				w.Header().Set(k, v)
			}

			// Content-Security-Policy.
			w.Header().Set("Content-Security-Policy", cspValue)

			// HSTS: only when behind TLS or when explicitly forced.
			isTLS := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
			if isTLS || forceHSTS {
				w.Header().Set("Strict-Transport-Security", hstsHeader)
			}

			next.ServeHTTP(w, r)
		})
	}
}
