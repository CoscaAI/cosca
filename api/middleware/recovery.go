// Package middleware provides HTTP middleware for the Cosca REST API.
package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/rs/zerolog/log"
)

// RecoveryMiddleware wraps an http.Handler and recovers from any panics,
// logging the stack trace and returning a 500 Internal Server Error.
// This MUST be the outermost middleware layer so it catches panics from
// all downstream handlers and middleware.
//
// Without this, a panic in any handler crashes the entire server.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error().
					Interface("panic", rec).
					Str("stack", string(debug.Stack())).
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Msg("HTTP handler panic recovered")
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
