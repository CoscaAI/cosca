package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// responseWriter wraps http.ResponseWriter to capture the status code
// for logging purposes.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	wroteBytes int64
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

// WriteHeader captures the status code and delegates to the original writer.
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response size and delegates to the original writer.
func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.wroteBytes += int64(n)
	return n, err
}

// sensitiveQueryParams are query parameter names whose values must never be
// written to logs (they may carry JWTs, API keys, or passwords). Matching is
// case-insensitive.
var sensitiveQueryParams = map[string]bool{
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
	"api_key":       true,
	"key":           true,
	"password":      true,
}

// redactQuery returns a copy of a raw query string with the values of
// sensitive parameters (token, access_token, refresh_token, api_key, key,
// password) replaced by "REDACTED". Non-sensitive parameters are preserved
// exactly — original ordering, keys, and encoding are kept. When no sensitive
// parameter is present the original string is returned unchanged.
func redactQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	parts := strings.Split(rawQuery, "&")
	redacted := false
	for i, part := range parts {
		if part == "" {
			continue
		}
		key := part
		if idx := strings.IndexByte(part, '='); idx >= 0 {
			key = part[:idx]
		}
		if sensitiveQueryParams[strings.ToLower(key)] {
			parts[i] = key + "=REDACTED"
			redacted = true
		}
	}
	if !redacted {
		return rawQuery
	}
	return strings.Join(parts, "&")
}

// LoggingMiddleware returns an HTTP middleware that logs every request
// using zerolog. The log entry includes: method, path, status code,
// duration, remote address, and response size.
func LoggingMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := newResponseWriter(w)

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			// Build a log event with request context
			event := logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapped.statusCode).
				Dur("duration", duration).
				Str("remote_addr", r.RemoteAddr).
				Int64("response_bytes", wrapped.wroteBytes)

			// Add query string if present (but not for health endpoints to
			// avoid log noise). Sensitive parameters (token, api_key, ...)
			// are redacted so credentials never reach the logs.
			if r.URL.RawQuery != "" {
				event = event.Str("query", redactQuery(r.URL.RawQuery))
			}

			// Use different log levels based on status code
			if wrapped.statusCode >= 500 {
				event.Msg("request completed with server error")
			} else if wrapped.statusCode >= 400 {
				event.Msg("request completed with client error")
			} else {
				event.Msg("request completed")
			}
		})
	}
}
