package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rizomai/rizomai/internal/store"
)

// Healthz responde o liveness do binário + status do banco.
// Operacional (fora do contrato /v1): 200 {status: ok, db: ok} ou
// 503 {status: degraded, db: unreachable}.
func Healthz(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := map[string]string{"status": "ok", "db": "ok"}

		if s != nil {
			if err := s.Ping(r.Context()); err != nil {
				status["status"] = "degraded"
				status["db"] = "unreachable"
				writeHealth(w, http.StatusServiceUnavailable, status)
				return
			}
		}

		writeHealth(w, http.StatusOK, status)
	}
}

func writeHealth(w http.ResponseWriter, code int, v map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
