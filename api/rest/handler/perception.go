package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CoscaAI/cosca/internal/perception"
)

// PerceptionHandler exposes the continuous Perception Loop over HTTP:
//
//	GET /v1/perception/stream  — SSE stream of the perceptual WorldState
//	GET /v1/perception/state  — current perceptual WorldState (JSON)
//
// It is nil-safe: when the perception loop is disabled (perception.enabled
// false) or no service is wired, both endpoints report a clear 503 instead of
// a misleading empty 200. The loop is opt-in and never changes default
// behaviour.
type PerceptionHandler struct {
	svc *perception.Service
}

// NewPerceptionHandler creates a PerceptionHandler. svc may be nil (disables
// the endpoints with a clear 503).
func NewPerceptionHandler(svc *perception.Service) *PerceptionHandler {
	return &PerceptionHandler{svc: svc}
}

// State handles GET /v1/perception/state — the current perceptual WorldState.
func (h *PerceptionHandler) State(w http.ResponseWriter, _ *http.Request) {
	if h.svc == nil || !h.svc.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"error": "perception loop disabled — set perception.enabled: true (opt-in) to enable",
		})
		return
	}
	writeJSON(w, http.StatusOK, h.svc.State())
}

// Stream handles GET /v1/perception/stream — SSE stream emitting a
// `perception` event with the perceptual WorldState on every update. It emits
// the current state immediately, then streams each subsequent update. A
// `heartbeat` event is sent periodically so idle clients (and any front-end
// proxy) keep the connection alive.
func (h *PerceptionHandler) Stream(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil || !h.svc.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"error": "perception loop disabled — set perception.enabled: true (opt-in) to enable",
		})
		return
	}

	flusher := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering for SSE

	ctx := r.Context()
	ch := h.svc.Subscribe()
	defer h.svc.Unsubscribe(ch)

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	writeSSE := func(event string, data interface{}) {
		b, err := json.Marshal(data)
		if err != nil {
			// Never let a marshal failure break the stream.
			b = []byte(`{"error":"marshal failed"}`)
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
			return
		}
		flusher.Flush()
	}

	// Send the current state immediately so a client that just connected sees
	// the world-state without waiting for the next tick.
	if st := h.svc.State(); st != nil {
		writeSSE("perception", st)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case st := <-ch:
			writeSSE("perception", st)
		case <-heartbeat.C:
			writeSSE("heartbeat", map[string]interface{}{"ts": time.Now().Unix()})
		}
	}
}
