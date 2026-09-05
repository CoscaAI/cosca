// Package auth provides authentication and authorization middleware for the
// Cosca REST API. It includes JWT bearer token validation, cookie-based
// auth, API key validation, role-based access control, and CSRF protection
// for state-changing endpoints.
package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// contextKey is a private type used for context keys to avoid collisions
// with keys from other packages.
type contextKey string

// ContextKeyClaims is the exported context key used to store and retrieve
// JWT claims from a request context.
const ContextKeyClaims contextKey = "claims"

// Middleware returns an HTTP middleware that validates authentication via
// three methods (checked in order):
//
//  1. X-API-Key header — for programmatic API access. If present and valid,
//     synthetic JWT claims are created from the API key metadata.
//  2. cosca_access_token cookie — for browser clients (XSS-resistant).
//  3. Authorization: Bearer <token> — for API clients.
//
// Requests to publicPaths are passed through without authentication.
//
// On success, the parsed *auth.Claims are stored in the request context
// under the contextKeyClaims key. On failure, a 401 JSON error is returned.
func Middleware(secret []byte, publicPaths []string, apiKeyStore *internalauth.APIKeyStore) func(http.Handler) http.Handler {
	// Build a lookup set for O(1) public path checks.
	public := make(map[string]bool, len(publicPaths))
	for _, p := range publicPaths {
		public[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for public paths.
			if public[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Fail-closed: an empty secret means authentication is
			// misconfigured. Protected routes must NOT be served — a JWT
			// signed with an empty HMAC key would otherwise validate
			// (HS256 accepts an empty key), silently granting access.
			if len(secret) == 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":"authentication not configured (JWT secret missing)"}`))
				return
			}

			// ── Method 1: API Key authentication (X-API-Key header) ────
			apiKeyHeader := r.Header.Get("X-API-Key")
			if apiKeyHeader != "" {
				if apiKeyStore == nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(`{"error":"api key authentication not available"}`))
					return
				}

				ak, err := apiKeyStore.Validate(apiKeyHeader)
				if err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
					return
				}

				// Build synthetic claims from the API key metadata.
				claims := &internalauth.Claims{
					Sub:      "apikey:" + ak.ID,
					Username: "api-key:" + ak.Name,
					Role:     ak.Role,
					Type:     "apikey",
					Iat:      time.Now().Unix(),
				}

				// Set expiration if the key has one.
				if ak.ExpiresAt != "" {
					if exp, err := time.Parse(time.RFC3339, ak.ExpiresAt); err == nil {
						claims.Exp = exp.Unix()
					}
				}

				ctx := context.WithValue(r.Context(), ContextKeyClaims, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// ── Methods 2 & 3: Cookie or Bearer token ──────────────────
			var tokenStr string

			if cookie, err := r.Cookie("cosca_access_token"); err == nil && cookie.Value != "" {
				tokenStr = cookie.Value
			} else {
				authHeader := r.Header.Get("Authorization")
				if !strings.HasPrefix(authHeader, "Bearer ") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
					return
				}
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}

			if tokenStr == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}

			// Validate the JWT — must be an access token. Refresh tokens
			// are rejected: a leaked refresh token can never be used to
			// call protected endpoints.
			claims, err := internalauth.ValidateAccessToken(tokenStr, secret)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}

			// Store claims in context and continue.
			ctx := context.WithValue(r.Context(), ContextKeyClaims, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext extracts JWT claims from a request context.
// It returns the claims and a boolean indicating whether they were found.
// This is the safe way for other packages to retrieve claims without
// knowing the internal context key type.
func ClaimsFromContext(ctx context.Context) (*internalauth.Claims, bool) {
	claims, ok := ctx.Value(ContextKeyClaims).(*internalauth.Claims)
	return claims, ok
}
