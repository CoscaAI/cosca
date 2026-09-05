// Package middleware provides HTTP middleware for the Cosca REST API.
package middleware

import (
	"net/http"
	"strings"
)

// CORSMiddleware returns an HTTP middleware that adds CORS headers to all
// responses and handles preflight OPTIONS requests.
//
// The allowedOrigins parameter is a comma-separated list of origins, or
// a single "*" to explicitly allow all origins (development only).
//
// FAIL-CLOSED: an empty or blank allowedOrigins list sets NO
// Access-Control-Allow-Origin header, so browsers block cross-origin
// access. This is the secure default — operators must explicitly list the
// origins they trust (e.g. --cors-origins "http://localhost:3000").
//
// When "*" is used and the request carries an explicit Origin header, the
// middleware echoes that origin back so that non-credentialed requests work
// from any origin. When no Origin is present, the literal "*" is used.
// Credentials are NEVER set when the wildcard is in use — without an explicit
// allowlist there is no way to guarantee that only trusted origins can make
// credentialed requests.
//
// When specific origins are provided, the middleware echoes back the request
// origin only if it appears in the allowed list, and sets
// Access-Control-Allow-Credentials for matched origins.
func CORSMiddleware(allowedOrigins string) func(http.Handler) http.Handler {
	// Pre-parse the allowed origins into a set for fast lookup.
	origins := parseOrigins(allowedOrigins)

	allowAll := false
	for _, o := range origins {
		if o == "*" {
			allowAll = true
			break
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Determine the allowed origin value for this request.
			var allowOrigin string
			var allowCredentials bool
			if allowAll {
				if origin != "" {
					// Echo the origin back for non-credentialed requests.
					// We do NOT set allowCredentials here because the
					// wildcard "*" cannot guarantee that only trusted
					// origins make credentialed requests — any origin
					// would be accepted.
					allowOrigin = origin
				} else {
					// No Origin header — use wildcard (safe for public
					// endpoints that don't need credentials).
					allowOrigin = "*"
				}
			} else if origin != "" {
				for _, o := range origins {
					if o == origin {
						allowOrigin = origin
						allowCredentials = true
						break
					}
				}
			}

			// Set CORS headers when origin is present and allowed.
			if allowOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
				w.Header().Set("Access-Control-Allow-Methods",
					"GET, POST, PUT, DELETE, PATCH, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers",
					"Content-Type, Authorization, X-Request-ID, X-Cosca-Scope, X-CSRF-Token, Accept, Origin")
				w.Header().Set("Access-Control-Expose-Headers",
					"X-Request-ID, X-Response-Time, Content-Length")
				w.Header().Set("Access-Control-Max-Age", "86400")

				if allowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}

			// Handle preflight requests immediately.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// parseOrigins parses a comma-separated list of allowed origins.
// Returns an empty slice (no origins allowed) when the input is empty or
// contains only separators/whitespace — never a wildcard. The "*" wildcard
// must be passed explicitly.
func parseOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	if raw == "*" {
		return []string{"*"}
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			origins = append(origins, p)
		}
	}
	return origins
}
