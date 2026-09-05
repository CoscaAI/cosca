package agentbridge

import (
	"sync"
	"time"
)

// EventType mirrors the bridge protocol's consumer event families (see
// BRIDGE_ARCHITECTURE.md — the 14 stream parts plus control frames).
type EventType string

const (
	// EventStreamStart: a turn began.
	EventStreamStart EventType = "stream-start"
	// EventTextStart/Delta/End: assistant text.
	EventTextStart EventType = "text-start"
	EventTextDelta EventType = "text-delta"
	EventTextEnd   EventType = "text-end"
	// EventReasoningStart/Delta/End: model reasoning (thinking).
	EventReasoningStart EventType = "reasoning-start"
	EventReasoningDelta EventType = "reasoning-delta"
	EventReasoningEnd   EventType = "reasoning-end"
	// EventToolCall: the agent requested a tool (host-side execution).
	EventToolCall EventType = "tool-call"
	// EventToolApprovalRequest: a tool needs the Don's approval.
	EventToolApprovalRequest EventType = "tool-approval-request"
	// EventToolResult: a tool finished (host-side).
	EventToolResult EventType = "tool-result"
	// EventFinishStep: a step finished with usage and reason.
	EventFinishStep EventType = "finish-step"
	// EventFinish: the whole turn finished.
	EventFinish EventType = "finish"
	// EventFileChange: the agent modified a file.
	EventFileChange EventType = "file-change"
	// EventError: an error occurred.
	EventError EventType = "error"
	// EventBridgeHello: bridge handshake.
	EventBridgeHello EventType = "bridge-hello"
	// EventBridgeDetach: resume payload.
	EventBridgeDetach EventType = "bridge-detach"
)

// Event is one item in the event stream rendered by the dashboard.
type Event struct {
	Seq       int64     `json:"seq"`
	SessionID string    `json:"session_id"`
	Type      EventType `json:"type"`
	Message   string    `json:"message"`
	At        time.Time `json:"at"`
}

// EventStore keeps the per-session event streams, bounded.
type EventStore struct {
	mu     sync.RWMutex
	events map[string][]Event // sessionID → events (append-only, bounded)
	cap    int
}

// NewEventStore creates an event store with the given per-session capacity.
func NewEventStore(capacity int) *EventStore {
	if capacity <= 0 {
		capacity = 500
	}
	return &EventStore{events: map[string][]Event{}, cap: capacity}
}

// Append adds an event to a session's stream and returns it with its seq.
func (es *EventStore) Append(sessionID string, etype EventType, message string, seq int64) Event {
	ev := Event{
		Seq:       seq,
		SessionID: sessionID,
		Type:      etype,
		Message:   message,
		At:        time.Now().UTC(),
	}
	es.mu.Lock()
	defer es.mu.Unlock()
	stream := es.events[sessionID]
	stream = append(stream, ev)
	if len(stream) > es.cap {
		// Drop the oldest events, keep the newest cap.
		stream = append([]Event(nil), stream[len(stream)-es.cap:]...)
	}
	es.events[sessionID] = stream
	return ev
}

// ForSession returns a copy of a session's events (oldest first).
func (es *EventStore) ForSession(sessionID string) []Event {
	es.mu.RLock()
	defer es.mu.RUnlock()
	stream := es.events[sessionID]
	out := make([]Event, len(stream))
	copy(out, stream)
	return out
}

// Since returns events for a session with seq greater than after (for
// incremental polling / SSE-style resume).
func (es *EventStore) Since(sessionID string, after int64) []Event {
	es.mu.RLock()
	defer es.mu.RUnlock()
	stream := es.events[sessionID]
	out := []Event{}
	for _, ev := range stream {
		if ev.Seq > after {
			out = append(out, ev)
		}
	}
	return out
}
