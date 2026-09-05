package stream

import (
	"errors"
	"net/http"
	"strings"

	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// Common WebSocket authentication errors.
var (
	ErrMissingToken   = errors.New("missing token: provide a JWT via Authorization: Bearer <jwt> or ?token=<jwt> query parameter")
	ErrTokenViolation = errors.New("token validation failed")
)

// bearerTokenPrefix is the Authorization header scheme used for JWTs.
const bearerTokenPrefix = "Bearer "

// AuthenticateUpgrade validates a JWT token for WebSocket upgrade
// authentication. The token is accepted from two places, in order of
// preference:
//
//  1. The `Authorization: Bearer <jwt>` header — used by non-browser
//     clients. It never leaks the token into query strings, access logs,
//     or browser history.
//  2. The `?token=<jwt>` query parameter — required for browsers, which
//     cannot set custom headers during the WebSocket upgrade handshake.
//
// The header is tried FIRST because query-string tokens are more exposed;
// the query parameter remains supported so the browser flow keeps working.
//
// The token is validated as an ACCESS token (signature + expiration + type)
// using the internal auth package, so refresh tokens are rejected for
// WebSocket connections.
//
// Returns the parsed Claims on success, or an error describing the failure.
// Callers should respond with http.StatusUnauthorized before attempting
// the WebSocket upgrade.
func AuthenticateUpgrade(r *http.Request, jwtSecret []byte) (*internalauth.Claims, error) {
	if len(jwtSecret) == 0 {
		// No JWT secret configured — allow anonymous connections.
		return nil, nil
	}

	// Prefer the Authorization header (safer — not logged, not in history),
	// then fall back to the ?token= query parameter for browsers.
	tokenStr := bearerToken(r)
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("token")
	}
	if strings.TrimSpace(tokenStr) == "" {
		return nil, ErrMissingToken
	}

	claims, err := internalauth.ValidateAccessToken(tokenStr, jwtSecret)
	if err != nil {
		return nil, errors.Join(ErrTokenViolation, err)
	}

	return claims, nil
}

// bearerToken extracts the token from an "Authorization: Bearer <jwt>"
// header. Returns "" when the header is missing, malformed, or uses a
// different scheme (e.g. "Basic").
func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) <= len(bearerTokenPrefix) || !strings.EqualFold(auth[:len(bearerTokenPrefix)], bearerTokenPrefix) {
		return ""
	}
	return strings.TrimSpace(auth[len(bearerTokenPrefix):])
}
