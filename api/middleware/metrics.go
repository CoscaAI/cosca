package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/metrics"
)

// metricsResponseWriter wraps http.ResponseWriter to capture the status code
// for metrics collection. It is a lightweight copy of the responseWriter
// used by LoggingMiddleware, intentionally duplicated to keep concerns
// independent.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newMetricsResponseWriter(w http.ResponseWriter) *metricsResponseWriter {
	return &metricsResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *metricsResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// MetricsMiddleware returns an HTTP middleware that captures request
// method, route pattern, status code, and duration, then records them
// in the provided HTTPMetrics collector for Prometheus exposition.
//
// The middleware uses r.Pattern (Go 1.22+) to obtain the route pattern
// registered on the ServeMux (e.g. "GET /v1/memory/stats"). This ensures
// path parameters like /v1/executions/{id} are grouped correctly.
func MetricsMiddleware(collector *metrics.HTTPMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := newMetricsResponseWriter(w)

			next.ServeHTTP(wrapped, r)

			// Use the matched route pattern for grouping (Go 1.22+).
			// Falls back to r.URL.Path for non-pattern-matched requests.
			path := r.URL.Path
			if r.Pattern != "" {
				// r.Pattern includes the HTTP method prefix (e.g. "GET /v1/health").
				// Strip it to get just the path.
				path = r.Pattern
				if idx := strings.Index(path, " "); idx >= 0 {
					path = path[idx+1:]
				}
			}

			duration := time.Since(start)
			collector.Record(r.Method, path, wrapped.statusCode, duration)
		})
	}
}
