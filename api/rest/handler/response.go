// Package handler provides HTTP handlers for the Cosca REST API.
package handler

import (
	"encoding/json"
	"log"
	"net/http"
)

// writeJSON writes a JSON response with the given status code and data.
// Logs encoding errors since the status code has already been sent.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("writeJSON: failed to encode response (status=%d): %v", status, err)
	}
}

// writeError writes a JSON error response with the given status code and message.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// getQueryParam extracts a query parameter from the request URL.
func getQueryParam(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}
