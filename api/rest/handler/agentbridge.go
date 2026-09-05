package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/CoscaAI/cosca/internal/agentbridge"
)

// AgentBridgeHandler handles REST API requests for the AgentBridge — the
// control plane that supervises agent sessions and the bridge process.
type AgentBridgeHandler struct {
	sessions *agentbridge.Store
	events   *agentbridge.EventStore
	bridge   *agentbridge.BridgeMonitor
}

// NewAgentBridgeHandler creates an AgentBridgeHandler with fresh stores.
// If sessions is nil, fresh stores are created (isolated per handler).
func NewAgentBridgeHandler(sessions *agentbridge.Store, events *agentbridge.EventStore, bridge *agentbridge.BridgeMonitor) *AgentBridgeHandler {
	if sessions == nil {
		sessions = agentbridge.NewStore()
	}
	if events == nil {
		events = agentbridge.NewEventStore(500)
	}
	if bridge == nil {
		bridge = agentbridge.NewBridgeMonitor()
	}
	return &AgentBridgeHandler{sessions: sessions, events: events, bridge: bridge}
}

// Status returns the overall AgentBridge status: bridge monitor + session
// count + summary.
func (h *AgentBridgeHandler) Status(w http.ResponseWriter, r *http.Request) {
	if h.sessions == nil || h.bridge == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "agent bridge store not available"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"bridge":        h.bridge.State(),
		"session_count": h.sessions.Count(),
		"sessions":      h.sessions.List(),
	})
}

// Sessions returns the list of agent sessions.
func (h *AgentBridgeHandler) Sessions(w http.ResponseWriter, r *http.Request) {
	if h.sessions == nil || h.bridge == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "agent bridge store not available"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": h.sessions.List(),
	})
}

// SessionEvents returns the event stream of a single session.
func (h *AgentBridgeHandler) SessionEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "session id required"})
		return
	}
	if h.sessions == nil || h.bridge == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "agent bridge store not available"})
		return
	}
	if _, err := h.sessions.Get(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session": id,
		"events":  h.events.ForSession(id),
	})
}

// SessionEventsStream streams session events as SSE. Clients resume from the
// Last-Event-ID header (seq > last) so no event is lost across reconnects.
func (h *AgentBridgeHandler) SessionEventsStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "session id required"})
		return
	}
	// Guard de nil (mesmo padrão do bug #17): um handler sem sessions store
	// deve responder 503, nunca nil-deref panic.
	if h.sessions == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "sessions store not available"})
		return
	}
	if _, err := h.sessions.Get(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": err.Error()})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "streaming unsupported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Initial snapshot of existing events.
	for _, ev := range h.events.ForSession(id) {
		writeSSE(w, ev)
	}
	flusher.Flush()

	// Poll loop (simple SSE without a push hub): every 500ms send new
	// events with seq > last delivered. This is the foundation; a real
	// WebSocket push hub (stream.Hub) replaces it in the bridge phase.
	last := lastSeq(h.events.ForSession(id))
	ctx := r.Context()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			newEvents := h.events.Since(id, last)
			for _, ev := range newEvents {
				writeSSE(w, ev)
				if ev.Seq > last {
					last = ev.Seq
				}
			}
			if len(newEvents) > 0 {
				flusher.Flush()
			}
		}
	}
}

// writeSSE writes one event as an SSE frame.
func writeSSE(w http.ResponseWriter, ev agentbridge.Event) {
	raw, _ := json.Marshal(ev)
	_, _ = w.Write([]byte("id: "))
	_, _ = w.Write([]byte(itoa(ev.Seq)))
	_, _ = w.Write([]byte("\ndata: "))
	_, _ = w.Write(raw)
	_, _ = w.Write([]byte("\n\n"))
}

// lastSeq returns the highest seq in a slice of events (0 when empty).
func lastSeq(events []agentbridge.Event) int64 {
	if len(events) == 0 {
		return 0
	}
	return events[len(events)-1].Seq
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ─── Action endpoints (the harness trinomial over HTTP) ───────────────

// createSessionRequest is the payload for POST /v1/agentbridge/sessions.
type createSessionRequest struct {
	ID       string `json:"id"`
	Agent    string `json:"agent"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

// CreateSession creates a new agent session (agent.create).
func (h *AgentBridgeHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r, bodyLimitSmall)
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid JSON body"})
		return
	}
	if req.Agent == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "agent is required"})
		return
	}
	if req.ID == "" {
		req.ID = "ses_" + randomHex(6)
	}
	sess := h.sessions.Create(req.ID, req.Agent, req.Model, req.Provider)
	h.events.Append(sess.ID, agentbridge.EventBridgeHello, "session created", h.sessions.NextSeq())
	writeJSON(w, http.StatusCreated, sess)
}

// detachSessionRequest optionally carries resume coordinates.
type detachSessionRequest struct {
	BridgePort  int    `json:"bridge_port"`
	BridgeToken string `json:"bridge_token"`
	SandboxID   string `json:"sandbox_id"`
}

// DetachSession parks a session with resume coordinates (harness doDetach).
func (h *AgentBridgeHandler) DetachSession(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r, bodyLimitSmall)
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "session id required"})
		return
	}
	var req detachSessionRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // body optional

	lastSeen := h.lastSeqFor(id)
	coords := &agentbridge.ResumeCoords{
		BridgePort:    req.BridgePort,
		BridgeToken:   req.BridgeToken,
		LastSeenEvent: lastSeen,
		SandboxID:     req.SandboxID,
	}
	if err := h.sessions.Detach(id, coords); err != nil {
		writeJSON(w, http.StatusConflict, map[string]interface{}{"error": err.Error()})
		return
	}
	h.events.Append(id, agentbridge.EventBridgeDetach, "session detached", h.sessions.NextSeq())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session": id, "status": "suspended", "resume": coords,
	})
}

// ResumeSession re-attaches a suspended session (attach → rerun → replay).
func (h *AgentBridgeHandler) ResumeSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "session id required"})
		return
	}
	if err := h.sessions.Resume(id); err != nil {
		writeJSON(w, http.StatusConflict, map[string]interface{}{"error": err.Error()})
		return
	}
	h.bridge.SetResumeStrategy(agentbridge.ResumeAttach)
	h.events.Append(id, agentbridge.EventStreamStart, "session resumed", h.sessions.NextSeq())
	writeJSON(w, http.StatusOK, map[string]interface{}{"session": id, "status": "idle"})
}

// StopSession persists and halts a session (harness doStop).
func (h *AgentBridgeHandler) StopSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "session id required"})
		return
	}
	if err := h.sessions.Stop(id); err != nil {
		writeJSON(w, http.StatusConflict, map[string]interface{}{"error": err.Error()})
		return
	}
	h.events.Append(id, agentbridge.EventFinish, "session stopped", h.sessions.NextSeq())
	writeJSON(w, http.StatusOK, map[string]interface{}{"session": id, "status": "stopped"})
}

// DestroySession removes a session entirely (harness doDestroy).
func (h *AgentBridgeHandler) DestroySession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "session id required"})
		return
	}
	if err := h.sessions.Destroy(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"session": id, "destroyed": true})
}

// EmitEvent appends a protocol event to a session's stream (used by the
// bridge phase to feed the SSE; exposed here for testing/tooling).
type emitEventRequest struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (h *AgentBridgeHandler) EmitEvent(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r, bodyLimitSmall)
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "session id required"})
		return
	}
	if _, err := h.sessions.Get(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": err.Error()})
		return
	}
	var req emitEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Type == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "type is required"})
		return
	}
	ev := h.events.Append(id, agentbridge.EventType(req.Type), req.Message, h.sessions.NextSeq())
	writeJSON(w, http.StatusCreated, ev)
}

// lastSeqFor returns the last event seq for a session (0 if none).
func (h *AgentBridgeHandler) lastSeqFor(id string) int64 {
	events := h.events.ForSession(id)
	if len(events) == 0 {
		return 0
	}
	return events[len(events)-1].Seq
}

// randomHex returns n random bytes hex-encoded (for session IDs).
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "000000"
	}
	return hex.EncodeToString(b)
}
