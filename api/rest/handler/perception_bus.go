package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CoscaAI/cosca/internal/perception/bus"
)

// BusHandler exposes the Perception Bus (the multimodal vision+audio
// synchroniser, FASE A) over HTTP:
//
//	GET /v1/perception/bus       — SSE stream of the synchronised WorldState
//	GET /v1/perception/bus/state — current synchronised WorldState (JSON)
//
// It is nil-safe: when the bus is not wired (perception.audio.enabled false) or
// no service is present, both endpoints report a clear 503 instead of a
// misleading empty 200. The bus is opt-in and never changes default behaviour.
type BusHandler struct {
	svc *bus.Bus
}

// NewBusHandler creates a BusHandler. svc may be nil (disables the endpoints
// with a clear 503).
func NewBusHandler(svc *bus.Bus) *BusHandler {
	return &BusHandler{svc: svc}
}

// State handles GET /v1/perception/bus/state — the current synchronised
// WorldState (the multimodal projection: ring window + latest vision + latest
// audio + overlap relations).
func (h *BusHandler) State(w http.ResponseWriter, _ *http.Request) {
	if h.svc == nil || !h.svc.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"error": "perception bus disabled — set perception.audio.enabled: true (opt-in) to enable",
		})
		return
	}
	st := h.svc.State()
	if st == nil {
		// The bus is enabled but has not produced a projection yet (e.g. no
		// ingest since start). Report a clear informational state rather than a
		// raw null.
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": true,
			"ready":   false,
			"message": "perception bus started, awaiting first observation",
		})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// Stream handles GET /v1/perception/bus — SSE stream emitting a `bus` event with
// the synchronised WorldState on every update. It emits the current state
// immediately, then streams each subsequent update. A `heartbeat` event is sent
// periodically so idle clients (and any front-end proxy) keep the connection
// alive.
func (h *BusHandler) Stream(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil || !h.svc.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"error": "perception bus disabled — set perception.audio.enabled: true (opt-in) to enable",
		})
		return
	}

	flusher := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering for SSE

	ctx := r.Context()
	ch := h.svc.Watch()
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
	// the synchronised world-state without waiting for the next ingest.
	if st := h.svc.State(); st != nil {
		writeSSE("bus", st)
	} else {
		writeSSE("bus", map[string]interface{}{
			"enabled": true,
			"ready":   false,
			"message": "perception bus started, awaiting first observation",
		})
	}

	for {
		select {
		case <-ctx.Done():
			return
		case st := <-ch:
			writeSSE("bus", st)
		case <-heartbeat.C:
			writeSSE("heartbeat", map[string]interface{}{"ts": time.Now().Unix()})
		}
	}
}
