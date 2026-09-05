// Package stream provides shared streaming infrastructure (SSE + WebSocket)
// for the Cosca REST API.
package stream

// StreamEvent represents a generic streaming event with a typed name and
// associated data payload.
type StreamEvent struct {
	Event string
	Data  interface{}
}

// SSE event type constants used across all streaming handlers.
//
// EventResponse carries LLM token content (legacy name "response" preserved
// for backward compatibility). EventToken is an alias.
const (
	EventThinking = "thinking" // Agent is thinking/analyzing before responding
	EventResponse = "response" // LLM token response content (streaming tokens)
	EventToken    = "response" // Alias for EventResponse
	EventDone      = "done"      // Stream completed successfully
	EventError     = "error"     // Stream error (in-band, does not change HTTP status)
	EventProgress  = "progress"  // Progress update (sync, workflows, etc. — future use)
	EventCancelled = "cancelled" // Stream cancelled (client disconnect or explicit cancel) — FINAL state; additive
)
