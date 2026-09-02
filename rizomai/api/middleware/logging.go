// Package middleware contém os middlewares do gateway (ADR-004):
// auth de API key, rate-limit, idempotency e recovery chegam na Fase 2.
package middleware

import (
	"log"
	"net/http"
	"time"
)

// statusWriter captura o status code para o log de acesso.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Logging registra method, path, status e duração de cada request.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}
