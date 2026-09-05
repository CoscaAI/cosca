package stream

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

// SSEWriter is a thin, stdlib-only wrapper around http.ResponseWriter and
// http.Flusher for writing Server-Sent Events (SSE).
//
// NOT safe for concurrent use. All writes must happen from the same
// goroutine. This matches the typical SSE handler pattern where a single
// goroutine loops over a stream of events.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex // protects close detection
	closed  bool
}

// NewSSEWriter creates a new SSEWriter, sets SSE headers on the response,
// and verifies that the ResponseWriter supports streaming.
//
// If the ResponseWriter does not implement http.Flusher (e.g. certain test
// recorders), an error is returned and the caller should respond with a
// standard HTTP error before any body bytes are written.
//
// Headers set:
//   - Content-Type: text/event-stream
//   - Cache-Control: no-cache
//   - Connection: keep-alive
//   - X-Accel-Buffering: no (disables nginx proxy buffering)
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported: http.Flusher not available")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	return &SSEWriter{
		w:       w,
		flusher: flusher,
	}, nil
}

// WriteEvent writes a named SSE event with the given data as a JSON payload.
// It automatically flushes after writing.
//
// String data is formatted as:
//
//	data: {"type":"<eventType>","content":"<data>"}\n\n
//
// Other data types are JSON-marshalled and merged at the top level:
//
//	data: {"type":"<eventType>",...fields}\n\n
func (s *SSEWriter) WriteEvent(eventType string, data interface{}) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("SSEWriter: write on closed writer")
	}
	s.mu.Unlock()

	payload, err := formatSSEEvent(eventType, data)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(s.w, "data: %s\n\n", payload)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// WriteDone sends a [DONE] event signalling successful stream completion.
// Equivalent to WriteEvent with eventType "done" and no data, but uses
// a simpler JSON payload:
//
//	data: {"type":"done"}\n\n
func (s *SSEWriter) WriteDone() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("SSEWriter: write on closed writer")
	}
	s.mu.Unlock()

	_, err := fmt.Fprintf(s.w, "data: {\"type\":\"done\"}\n\n")
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// WriteDoneWithDuration sends a [DONE] event with the stream duration in
// milliseconds. This matches the original format used by POST /v1/run/stream:
//
//	data: {"type":"done","duration_ms":<durationMs>}\n\n
func (s *SSEWriter) WriteDoneWithDuration(durationMs int64) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("SSEWriter: write on closed writer")
	}
	s.mu.Unlock()

	_, err := fmt.Fprintf(s.w, "data: {\"type\":\"done\",\"duration_ms\":%d}\n\n", durationMs)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// WriteError sends an error event in-band (does NOT change the HTTP status
// code). The error message is formatted as:
//
//	data: {"type":"error","content":"<err.Error()>"}\n\n
func (s *SSEWriter) WriteError(err error) error {
	if err == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("SSEWriter: write on closed writer")
	}
	s.mu.Unlock()

	return s.WriteEvent(EventError, err.Error())
}

// Flush flushes any buffered data to the client. Normally you do not need
// to call this directly — WriteEvent, WriteDone, and WriteError all flush
// automatically. Use Flush after writing raw data to the underlying writer.
func (s *SSEWriter) Flush() {
	s.flusher.Flush()
}

// Close marks the writer as closed. After Close, further write attempts
// return an error. This is safe to call multiple times.
//
// Note: Close does NOT close the underlying http.ResponseWriter or TCP
// connection. That is managed by the HTTP server. For context cancellation,
// the caller should monitor r.Context().Done() and call Close() when the
// client disconnects.
func (s *SSEWriter) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// ResponseWriter returns the underlying http.ResponseWriter for advanced
// use cases (e.g. checking if headers were already written).
func (s *SSEWriter) ResponseWriter() http.ResponseWriter {
	return s.w
}

// formatSSEEvent builds the JSON payload string for an SSE event.
func formatSSEEvent(eventType string, data interface{}) (string, error) {
	switch v := data.(type) {
	case string:
		// Matches the legacy sendSSE() format from handler/run.go:
		//   data: {"type":"<eventType>","content":<JSON-string>}\n\n
		escaped, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("marshal SSE string content: %w", err)
		}
		return fmt.Sprintf(`{"type":"%s","content":%s}`, eventType, string(escaped)), nil

	default:
		// For structured data, JSON-marshal the value and merge top-level
		// fields with the "type" key. If the value does not unmarshal as a
		// JSON object we fall back to embedding it under "content".
		dataBytes, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("marshal SSE data: %w", err)
		}
		var dataMap map[string]interface{}
		if err := json.Unmarshal(dataBytes, &dataMap); err != nil {
			// Not a map — embed as raw string under "content".
			return fmt.Sprintf(`{"type":"%s","content":%s}`, eventType, string(dataBytes)), nil
		}
		dataMap["type"] = eventType
		merged, err := json.Marshal(dataMap)
		if err != nil {
			return "", fmt.Errorf("marshal merged SSE event: %w", err)
		}
		return string(merged), nil
	}
}
