// Package handlers contém os handlers HTTP do gateway, 1 arquivo por recurso,
// espelhando openapi/rizomai.yaml (ADR-004).
package handlers

import (
	"encoding/json"
	"net/http"
)

// Healthz responde 200 + {"status":"ok"} — liveness do binário.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
