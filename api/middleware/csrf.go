package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

// CSRFMiddleware implements the double-submit cookie pattern for CSRF protection.
//
// For mutating requests (POST, PUT, DELETE, PATCH), it checks that:
//  1. The X-CSRF-Token header matches the csrf_token cookie
//  2. Both are present and non-empty
//
// GET, HEAD, and OPTIONS requests are never checked.
// Requests to paths in publicPaths are also never checked (e.g., login,
// health endpoints must work without a pre-existing CSRF token).
func CSRFMiddleware(publicPaths []string) func(http.Handler) http.Handler {
	public := make(map[string]bool, len(publicPaths))
	for _, p := range publicPaths {
		public[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip CSRF check for safe methods.
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Skip CSRF check for public paths (e.g., login, refresh).
			if public[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Skip CSRF check for API key authenticated requests — these
			// are programmatic (non-browser) and have no CSRF cookie.
			if r.Header.Get("X-API-Key") != "" {
				next.ServeHTTP(w, r)
				return
			}

			// Get token from cookie.
			cookie, err := r.Cookie("csrf_token")
			if err != nil || cookie.Value == "" {
				http.Error(w, `{"error":"csrf token missing"}`, http.StatusForbidden)
				return
			}

			// Get token from header.
			header := r.Header.Get("X-CSRF-Token")
			if header == "" {
				http.Error(w, `{"error":"csrf header missing"}`, http.StatusForbidden)
				return
			}

			// Compare using constant-time comparison to prevent timing attacks.
			if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
				http.Error(w, `{"error":"csrf token mismatch"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GenerateCSRFToken generates a random CSRF token (32 bytes, hex-encoded)
// suitable for use with the double-submit cookie pattern.
func GenerateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SetCSRFCookie sets the CSRF token as a cookie on the response.
//
// The cookie is HttpOnly=false so that JavaScript can read it for inclusion
// in the X-CSRF-Token header. The Secure flag is set automatically based on
// whether the request arrived over HTTPS (TLS or X-Forwarded-Proto: https).
// SameSite=Lax still blocks the cookie on CROSS-SITE requests (the primary
// CSRF defense) while allowing same-site requests that differ only by port or
// subdomain — required for the dev topology where the frontend (:7000) and
// backend (:14120) are different origins. In production behind a reverse
// proxy serving both on one origin, SameSite=Strict could be re-enabled.
func SetCSRFCookie(w http.ResponseWriter, r *http.Request, token string) {
	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    token,
		Path:     "/",
		HttpOnly: false, // Must be readable by JavaScript for double-submit.
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours.
	})
}
