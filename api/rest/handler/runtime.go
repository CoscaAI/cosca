package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/api/stream"
	"github.com/CoscaAI/cosca/internal/grpcclient"
	"github.com/CoscaAI/cosca/internal/runtime"
)

// RuntimeHandler handles REST API requests for the Runtime Engine.
//
// The handler supports two sources (FASE 2, DDNA-2026-08-07-001):
//   - Local: backed by the in-process runtime.Runtime (rt). Used by
//     `cosca serve` without --api-only (default) and by existing tests.
//   - Remote: backed by the standalone runtime daemon through a
//     grpcclient.RuntimeClient (client). Used by `cosca serve --api-only`.
//
// When client is set, the remote daemon wins over rt — the REST server then
// reports the daemon's real state instead of the serve's own in-memory
// engine. When client is set but the daemon is not reachable, responses are
// HONEST: Status reports state "stopped", Health reports healthy=false with
// a clear warning. Never a generic placeholder.
type RuntimeHandler struct {
	rt     *runtime.Runtime
	client *grpcclient.RuntimeClient
	hub    *stream.Hub
}

// NewRuntimeHandler creates a new RuntimeHandler backed by the in-process
// runtime. If rt is nil, the handler returns placeholder data for all
// endpoints (historical behavior, unchanged).
func NewRuntimeHandler(rt *runtime.Runtime) *RuntimeHandler {
	return &RuntimeHandler{rt: rt}
}

// NewRuntimeHandlerWithClient creates a new RuntimeHandler that consults the
// standalone runtime daemon via gRPC (API-only mode). rt may be nil or set —
// when client is non-nil, Status/Health/StatusStream query the daemon.
func NewRuntimeHandlerWithClient(rt *runtime.Runtime, client *grpcclient.RuntimeClient) *RuntimeHandler {
	return &RuntimeHandler{rt: rt, client: client}
}

// SetHub sets the WebSocket Hub for broadcasting real-time status events.
// May be nil if WebSocket is disabled.
func (h *RuntimeHandler) SetHub(hub *stream.Hub) {
	h.hub = hub
}

// --- Response types ---

// ComponentInfo represents a single runtime component in the API response.
type ComponentInfo struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Uptime  string `json:"uptime,omitempty"`
	Message string `json:"message,omitempty"`
}

// StatusResponse is the JSON response for runtime status.
type StatusResponse struct {
	State      string                   `json:"state"`
	Health     string                   `json:"health"`
	Uptime     string                   `json:"uptime"`
	Version    string                   `json:"version"`
	Components map[string]ComponentInfo `json:"components,omitempty"`
}

// HealthResponse is the JSON response for runtime health.
type HealthResponse struct {
	Healthy  bool     `json:"healthy"`
	Warnings []string `json:"warnings,omitempty"`
}

// --- Remote (daemon) helpers ---

// remoteStatus maps a protobuf StatusResponse from the standalone daemon
// into the REST JSON StatusResponse.
func remoteStatus(pb *cospb.StatusResponse) StatusResponse {
	components := make(map[string]ComponentInfo, len(pb.GetComponents()))
	for name, info := range pb.GetComponents() {
		components[name] = ComponentInfo{
			Name:   info.GetName(),
			Status: info.GetStatus(),
			Uptime: time.Duration(info.GetUptimeSeconds() * float64(time.Second)).Round(time.Second).String(),
		}
	}
	return StatusResponse{
		State:      pb.GetState(),
		Health:     pb.GetHealth(),
		Uptime:     time.Duration(pb.GetUptimeSeconds() * float64(time.Second)).Round(time.Second).String(),
		Version:    pb.GetVersion(),
		Components: components,
	}
}

// daemonDownStatus is the honest "daemon is not active" response. The
// daemon is never a generic "unknown" placeholder — if the standalone
// daemon is expected but not answering, the runtime is reported as stopped.
func (h *RuntimeHandler) daemonDownStatus() StatusResponse {
	return StatusResponse{
		State:   "stopped",
		Health:  "unavailable",
		Uptime:  "0s",
		Version: "0.0.0",
		Components: map[string]ComponentInfo{
			"daemon": {
				Name:    "daemon",
				Status:  "stopped",
				Message: "runtime daemon não está ativo em " + h.client.Addr(),
			},
		},
	}
}

// --- Handlers ---

// Status handles GET /v1/status
func (h *RuntimeHandler) Status(w http.ResponseWriter, _ *http.Request) {
	// Remote source (API-only mode): query the standalone daemon.
	if h.client != nil {
		resp, err := h.client.Status(context.Background())
		if err != nil {
			writeJSON(w, http.StatusOK, h.daemonDownStatus())
			return
		}
		writeJSON(w, http.StatusOK, remoteStatus(resp))
		return
	}

	if h.rt == nil {
		// Return placeholder when runtime is not available
		resp := StatusResponse{
			State:   "unknown",
			Health:  "unknown",
			Uptime:  "0s",
			Version: "0.0.0",
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	state := h.rt.State()
	stateSnapshot := state.Get()
	report := h.rt.HealthReport()
	cfg := h.rt.Config()

	healthStr := "unknown"
	if hs, ok := report["health"].(runtime.ComponentStatus); ok {
		healthStr = string(hs)
	}

	components := make(map[string]ComponentInfo)
	for name, info := range stateSnapshot.Components {
		components[name] = ComponentInfo{
			Name:    info.Name,
			Status:  string(info.Status),
			Uptime:  info.Uptime.Round(time.Second).String(),
			Message: info.Message,
		}
	}

	resp := StatusResponse{
		State:      stateSnapshot.CurrentState.String(),
		Health:     healthStr,
		Uptime:     stateSnapshot.Uptime.Round(time.Second).String(),
		Version:    cfg.Version,
		Components: components,
	}

	writeJSON(w, http.StatusOK, resp)
}

// Health handles GET /v1/health
func (h *RuntimeHandler) Health(w http.ResponseWriter, _ *http.Request) {
	// Remote source (API-only mode): query the standalone daemon.
	if h.client != nil {
		resp, err := h.client.Health(context.Background())
		if err != nil {
			// Honest: the daemon is expected but not active.
			writeJSON(w, http.StatusOK, HealthResponse{
				Healthy:  false,
				Warnings: []string{"runtime daemon não está ativo em " + h.client.Addr()},
			})
			return
		}
		writeJSON(w, http.StatusOK, HealthResponse{
			Healthy:  resp.GetHealthy(),
			Warnings: resp.GetWarnings(),
		})
		return
	}

	if h.rt == nil {
		resp := HealthResponse{
			Healthy:  true,
			Warnings: []string{"runtime not available, reporting healthy by default"},
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	health := h.rt.Health()
	warnings := make([]string, 0)

	if health == runtime.StatusDegraded {
		for name, info := range h.rt.State().AllComponentStatuses() {
			if info.Status == runtime.StatusDegraded {
				warnings = append(warnings, name+": "+info.Message)
			}
		}
	}

	healthy := health == runtime.StatusHealthy || health == runtime.StatusUnknown

	resp := HealthResponse{
		Healthy:  healthy,
		Warnings: warnings,
	}

	writeJSON(w, http.StatusOK, resp)
}

// StatusStream handles GET /v1/status/stream — SSE endpoint that streams
// runtime status changes in real-time.
func (h *RuntimeHandler) StatusStream(w http.ResponseWriter, r *http.Request) {
	flusher := http.NewResponseController(w)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx := r.Context()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	heartbeatTicker := time.NewTicker(10 * time.Second)
	defer heartbeatTicker.Stop()

	lastHealth := ""

	writeSSE := func(event string, data interface{}) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		flusher.Flush()
	}

	// sendRemoteStatus streams the standalone daemon's state. Each tick is a
	// short gRPC round-trip; when the daemon drops mid-stream, an honest
	// "health" event (healthy=false + warning) is emitted instead of silence.
	sendRemoteStatus := func() {
		resp, err := h.client.Status(ctx)
		if err != nil {
			writeSSE("health", map[string]interface{}{
				"healthy": false,
				"status":  "unavailable",
				"warning": err.Error(),
			})
			return
		}

		writeSSE("status", map[string]interface{}{
			"state":          resp.GetState(),
			"health":         resp.GetHealth(),
			"uptime_seconds": resp.GetUptimeSeconds(),
		})

		if resp.GetHealth() != lastHealth {
			lastHealth = resp.GetHealth()
			writeSSE("health", map[string]interface{}{
				"healthy": resp.GetHealth() == "healthy" || resp.GetHealth() == "unknown",
				"status":  resp.GetHealth(),
			})
		}
	}

	sendStatus := func() {
		if h.client != nil {
			sendRemoteStatus()
			return
		}
		if h.rt == nil {
			return
		}
		state := h.rt.State().Current().String()
		health := h.rt.Health()
		healthStr := string(health)
		uptime := h.rt.State().Get().Uptime.Seconds()

		writeSSE("status", map[string]interface{}{
			"state":          state,
			"health":         healthStr,
			"uptime_seconds": uptime,
		})

		if healthStr != lastHealth {
			lastHealth = healthStr
			writeSSE("health", map[string]interface{}{
				"healthy": health == runtime.StatusHealthy || health == runtime.StatusUnknown,
				"status":  healthStr,
			})
		}
	}

	sendStatus()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sendStatus()
		case <-heartbeatTicker.C:
			writeSSE("heartbeat", map[string]interface{}{"ts": time.Now().Unix()})
		}
	}
}
