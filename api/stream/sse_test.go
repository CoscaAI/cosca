package stream

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewSSEWriter_FlusherNotAvailable(t *testing.T) {
	// httptest.ResponseRecorder doesn't implement http.Flusher.
	// We simulate this with a minimal struct.
	type noFlusher struct {
		http.ResponseWriter
	}
	// This would panic in practice — just test with a nil-like scenario.
	// Use a real ResponseWriter that doesn't implement Flusher.
	w := &responseWriterWithoutFlusher{}
	sw, err := NewSSEWriter(w)
	if err == nil {
		t.Error("expected error when Flusher is not available")
	}
	if sw != nil {
		t.Error("expected nil SSEWriter on error")
	}
}

// responseWriterWithoutFlusher implements http.ResponseWriter but NOT http.Flusher.
type responseWriterWithoutFlusher struct {
	header http.Header
}

func (w *responseWriterWithoutFlusher) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *responseWriterWithoutFlusher) Write(b []byte) (int, error) {
	return len(b), nil
}

func (w *responseWriterWithoutFlusher) WriteHeader(statusCode int) {}

func TestNewSSEWriter_SetsHeaders(t *testing.T) {
	tw := NewSSETestWriter()
	sw, err := NewSSEWriter(tw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sw == nil {
		t.Fatal("expected non-nil SSEWriter")
	}

	tests := []struct {
		header string
		want   string
	}{
		{"Content-Type", "text/event-stream"},
		{"Cache-Control", "no-cache"},
		{"Connection", "keep-alive"},
		{"X-Accel-Buffering", "no"},
	}
	for _, tt := range tests {
		got := tw.Header().Get(tt.header)
		if got != tt.want {
			t.Errorf("header %s: got %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestWriteEvent_StringData(t *testing.T) {
	tw := NewSSETestWriter()
	sw, err := NewSSEWriter(tw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = sw.WriteEvent("thinking", "Analyzing request...")
	if err != nil {
		t.Fatalf("WriteEvent failed: %v", err)
	}

	output := tw.Body().String()
	wantPrefix := "data: "
	if !strings.HasPrefix(output, wantPrefix) {
		t.Errorf("expected output to start with %q, got %q", wantPrefix, output)
	}
	wantSuffix := "\n\n"
	if !strings.HasSuffix(output, wantSuffix) {
		t.Errorf("expected output to end with %q, got %q", wantSuffix, output)
	}

	// Parse the JSON payload.
	payload := strings.TrimPrefix(output, "data: ")
	payload = strings.TrimSuffix(payload, "\n\n")

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("failed to parse JSON payload: %v", err)
	}
	if m["type"] != "thinking" {
		t.Errorf("expected type 'thinking', got %v", m["type"])
	}
	if m["content"] != "Analyzing request..." {
		t.Errorf("expected content 'Analyzing request...', got %v", m["content"])
	}
}

func TestWriteEvent_StringData_SpecialChars(t *testing.T) {
	tw := NewSSETestWriter()
	sw, err := NewSSEWriter(tw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Content with quotes, newlines, and unicode.
	specialContent := `Hello "world" with\nnewline and 日本語`
	err = sw.WriteEvent("response", specialContent)
	if err != nil {
		t.Fatalf("WriteEvent failed: %v", err)
	}

	output := tw.Body().String()
	payload := strings.TrimPrefix(output, "data: ")
	payload = strings.TrimSuffix(payload, "\n\n")

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("failed to parse JSON payload with special chars: %v\npayload: %s", err, payload)
	}
	if m["content"] != specialContent {
		t.Errorf("expected content %q, got %v", specialContent, m["content"])
	}
}

func TestWriteEvent_StructuredData(t *testing.T) {
	tw := NewSSETestWriter()
	sw, err := NewSSEWriter(tw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	progress := map[string]interface{}{
		"phase":   "scanning",
		"current": 42,
		"total":   100,
	}
	err = sw.WriteEvent("progress", progress)
	if err != nil {
		t.Fatalf("WriteEvent failed: %v", err)
	}

	output := tw.Body().String()
	payload := strings.TrimPrefix(output, "data: ")
	payload = strings.TrimSuffix(payload, "\n\n")

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("failed to parse JSON payload: %v", err)
	}
	if m["type"] != "progress" {
		t.Errorf("expected type 'progress', got %v", m["type"])
	}
	if m["phase"] != "scanning" {
		t.Errorf("expected phase 'scanning', got %v", m["phase"])
	}
	if m["current"] != float64(42) { // JSON numbers are float64
		t.Errorf("expected current 42, got %v", m["current"])
	}
}

func TestWriteDone(t *testing.T) {
	tw := NewSSETestWriter()
	sw, err := NewSSEWriter(tw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = sw.WriteDone()
	if err != nil {
		t.Fatalf("WriteDone failed: %v", err)
	}

	output := tw.Body().String()
	payload := strings.TrimPrefix(output, "data: ")
	payload = strings.TrimSuffix(payload, "\n\n")

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("failed to parse JSON payload: %v", err)
	}
	if m["type"] != "done" {
		t.Errorf("expected type 'done', got %v", m["type"])
	}
}

func TestWriteDoneWithDuration(t *testing.T) {
	tw := NewSSETestWriter()
	sw, err := NewSSEWriter(tw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = sw.WriteDoneWithDuration(1234)
	if err != nil {
		t.Fatalf("WriteDoneWithDuration failed: %v", err)
	}

	output := tw.Body().String()
	payload := strings.TrimPrefix(output, "data: ")
	payload = strings.TrimSuffix(payload, "\n\n")

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("failed to parse JSON payload: %v", err)
	}
	if m["type"] != "done" {
		t.Errorf("expected type 'done', got %v", m["type"])
	}
	if m["duration_ms"] != float64(1234) {
		t.Errorf("expected duration_ms 1234, got %v", m["duration_ms"])
	}
}

func TestWriteError(t *testing.T) {
	tw := NewSSETestWriter()
	sw, err := NewSSEWriter(tw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = sw.WriteError(errors.New("something went wrong"))
	if err != nil {
		t.Fatalf("WriteError failed: %v", err)
	}

	output := tw.Body().String()
	payload := strings.TrimPrefix(output, "data: ")
	payload = strings.TrimSuffix(payload, "\n\n")

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("failed to parse JSON payload: %v", err)
	}
	if m["type"] != "error" {
		t.Errorf("expected type 'error', got %v", m["type"])
	}
	if m["content"] != "something went wrong" {
		t.Errorf("expected content 'something went wrong', got %v", m["content"])
	}
}

func TestWriteError_Nil(t *testing.T) {
	tw := NewSSETestWriter()
	sw, err := NewSSEWriter(tw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nil error should be a no-op.
	err = sw.WriteError(nil)
	if err != nil {
		t.Fatalf("WriteError(nil) returned unexpected error: %v", err)
	}
	if tw.Body().Len() != 0 {
		t.Errorf("expected empty body for nil error, got: %s", tw.Body().String())
	}
}

func TestClose_PreventsWrites(t *testing.T) {
	tw := NewSSETestWriter()
	sw, err := NewSSEWriter(tw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = sw.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Subsequent writes should return errors.
	err = sw.WriteEvent("thinking", "hello")
	if err == nil {
		t.Error("expected error writing to closed SSEWriter")
	}

	err = sw.WriteDone()
	if err == nil {
		t.Error("expected error from WriteDone on closed SSEWriter")
	}

	err = sw.WriteError(errors.New("err"))
	if err == nil {
		t.Error("expected error from WriteError on closed SSEWriter")
	}
}

func TestClose_Idempotent(t *testing.T) {
	tw := NewSSETestWriter()
	sw, _ := NewSSEWriter(tw)

	if err := sw.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	if err := sw.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}

func TestResponseWriter(t *testing.T) {
	tw := NewSSETestWriter()
	sw, _ := NewSSEWriter(tw)

	got := sw.ResponseWriter()
	if got != tw {
		t.Error("ResponseWriter() should return the original writer")
	}
}

func TestFormatSSEEvent_StringContent(t *testing.T) {
	payload, err := formatSSEEvent("response", "Hello, world!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("failed to parse payload: %v", err)
	}
	if m["type"] != "response" {
		t.Errorf("expected type 'response', got %v", m["type"])
	}
	if m["content"] != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got %v", m["content"])
	}
}

func TestFormatSSEEvent_StructuredContent(t *testing.T) {
	data := map[string]interface{}{
		"duration_ms": int64(1234),
		"tokens":      42,
	}
	payload, err := formatSSEEvent("done", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("failed to parse payload: %v", err)
	}
	if m["type"] != "done" {
		t.Errorf("expected type 'done', got %v", m["type"])
	}
	if m["duration_ms"] != float64(1234) {
		t.Errorf("expected duration_ms 1234, got %v", m["duration_ms"])
	}
	if m["tokens"] != float64(42) {
		t.Errorf("expected tokens 42, got %v", m["tokens"])
	}
}

// Test the full event format matches the legacy sendSSE() output from handler/run.go.
// Legacy format: data: {"type":"<type>","content":<JSON-escaped string>}\n\n
func TestWriteEvent_LegacyFormatCompatibility(t *testing.T) {
	tw := NewSSETestWriter()
	sw, err := NewSSEWriter(tw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// This is the exact call from the original handler.
	_ = sw.WriteEvent("thinking", "Analyzing request and preparing response...")

	output := tw.Body().String()
	expected := `data: {"type":"thinking","content":"Analyzing request and preparing response..."}` + "\n\n"
	if output != expected {
		t.Errorf("output mismatch:\n got:  %q\n want: %q", output, expected)
	}
}

func TestWriteEvent_LegacyFormatCompatibility_WithQuotes(t *testing.T) {
	tw := NewSSETestWriter()
	sw, _ := NewSSEWriter(tw)

	// Content with double quotes should be JSON-escaped properly.
	_ = sw.WriteEvent("response", `He said "hello"`)

	output := tw.Body().String()
	// Quotes should be escaped: He said \"hello\"
	expected := `data: {"type":"response","content":"He said \"hello\""}` + "\n\n"
	if output != expected {
		t.Errorf("output mismatch:\n got:  %q\n want: %q", output, expected)
	}
}

func TestWriteDoneWithDuration_LegacyFormatCompatibility(t *testing.T) {
	tw := NewSSETestWriter()
	sw, _ := NewSSEWriter(tw)

	_ = sw.WriteDoneWithDuration(5678)

	output := tw.Body().String()
	expected := `data: {"type":"done","duration_ms":5678}` + "\n\n"
	if output != expected {
		t.Errorf("output mismatch:\n got:  %q\n want: %q", output, expected)
	}
}

func TestWriteEvent_NonMapStructuredData(t *testing.T) {
	tw := NewSSETestWriter()
	sw, _ := NewSSEWriter(tw)

	// A slice (not a map) should be serialized as raw JSON under "content".
	err := sw.WriteEvent("items", []string{"a", "b"})
	if err != nil {
		t.Fatalf("WriteEvent failed: %v", err)
	}

	output := tw.Body().String()
	payload := strings.TrimPrefix(output, "data: ")
	payload = strings.TrimSuffix(payload, "\n\n")

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("failed to parse JSON payload: %v", err)
	}
	if m["type"] != "items" {
		t.Errorf("expected type 'items', got %v", m["type"])
	}
	// Content should be the JSON representation of the array.
	content, ok := m["content"].([]interface{})
	if !ok {
		t.Fatalf("expected content to be array, got %T: %v", m["content"], m["content"])
	}
	if len(content) != 2 {
		t.Errorf("expected 2 items, got %d", len(content))
	}
}

func TestSSETestWriter_ImplementsFlusher(t *testing.T) {
	tw := NewSSETestWriter()
	sw, err := NewSSEWriter(tw)
	if err != nil {
		t.Fatalf("SSETestWriter should work with NewSSEWriter: %v", err)
	}
	// Flush should be a no-op.
	sw.Flush()
}

// ── WebSocket Hub tests (no real WebSocket needed) ────────────────────────

func mockConn(h *Hub) *Connection {
	return &Connection{
		hub:    h,
		topics: make(map[string]bool),
		send:   make(chan []byte, 16),
		logger: zerolog.Nop(),
		ctx:    context.Background(),
		cancel: func() {},
		done:   make(chan struct{}),
	}
}

func TestHub_NewHub(t *testing.T) {
	logger := zerolog.Nop()
	h := NewHub(logger)
	if h == nil {
		t.Fatal("NewHub returned nil")
	}
	if h.ConnectionCount() != 0 {
		t.Errorf("ConnectionCount = %d, want 0", h.ConnectionCount())
	}
	if h.TopicCount() != 0 {
		t.Errorf("TopicCount = %d, want 0", h.TopicCount())
	}
	// Cancel before shutdown to avoid nil conn panics
	h.cancel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	h.Shutdown(ctx)
}

func TestHub_SubscribeUnsubscribe(t *testing.T) {
	logger := zerolog.Nop()
	h := NewHub(logger)
	h.cancel()

	conn := mockConn(h)

	if !h.Subscribe("chat", conn) {
		t.Error("first Subscribe should return true")
	}
	if h.Subscribe("chat", conn) {
		t.Error("duplicate Subscribe should return false")
	}
	if h.TopicCount() != 1 {
		t.Errorf("TopicCount = %d, want 1", h.TopicCount())
	}

	if !h.Unsubscribe("chat", conn) {
		t.Error("Unsubscribe should return true")
	}
	if h.Unsubscribe("chat", conn) {
		t.Error("double Unsubscribe should return false")
	}
	if h.TopicCount() != 0 {
		t.Errorf("TopicCount = %d, want 0", h.TopicCount())
	}
}

func TestHub_Broadcast(t *testing.T) {
	logger := zerolog.Nop()
	h := NewHub(logger)
	defer func() { h.cancel() }()

	conn := mockConn(h)
	h.Register(conn)
	h.Subscribe("alerts", conn)

	h.Broadcast(BroadcastMessage{
		Topics:  []string{"alerts"},
		Payload: []byte(`{"msg":"hello"}`),
	})
	time.Sleep(15 * time.Millisecond)

	select {
	case msg := <-conn.send:
		if string(msg) != `{"msg":"hello"}` {
			t.Errorf("unexpected broadcast message: %s", msg)
		}
	default:
		t.Error("expected message on conn.send, got nothing")
	}
}

func TestHub_BroadcastJSON(t *testing.T) {
	logger := zerolog.Nop()
	h := NewHub(logger)
	defer func() { h.cancel() }()

	conn := mockConn(h)
	h.Register(conn)
	h.Subscribe("events", conn)

	h.BroadcastJSON([]string{"events"}, OutgoingMessage{
		Type: "test_event",
		Data: map[string]string{"key": "val"},
	})
	time.Sleep(15 * time.Millisecond)

	select {
	case msg := <-conn.send:
		if !strings.Contains(string(msg), "test_event") {
			t.Errorf("unexpected message: %s", msg)
		}
	default:
		t.Error("expected broadcast JSON message")
	}
}

func TestHub_BroadcastEvent(t *testing.T) {
	logger := zerolog.Nop()
	h := NewHub(logger)
	defer func() { h.cancel() }()

	conn := mockConn(h)
	h.Register(conn)
	h.Subscribe("system", conn)

	h.BroadcastEvent([]string{"system"}, "shutdown", nil)
	time.Sleep(15 * time.Millisecond)

	select {
	case msg := <-conn.send:
		if !strings.Contains(string(msg), `"type":"shutdown"`) {
			t.Errorf("unexpected message: %s", msg)
		}
	default:
		t.Error("expected broadcast event message")
	}
}

func TestHub_UnregisterCleansTopics(t *testing.T) {
	logger := zerolog.Nop()
	h := NewHub(logger)
	defer func() { h.cancel() }()

	conn := mockConn(h)
	h.Register(conn)
	h.Subscribe("topic-a", conn)
	h.Subscribe("topic-b", conn)

	if h.TopicCount() != 2 {
		t.Errorf("TopicCount = %d, want 2", h.TopicCount())
	}

	h.Unregister(conn)
	time.Sleep(15 * time.Millisecond)

	if h.TopicCount() != 0 {
		t.Errorf("TopicCount after unregister = %d, want 0", h.TopicCount())
	}
	if h.ConnectionCount() != 0 {
		t.Errorf("ConnectionCount after unregister = %d, want 0", h.ConnectionCount())
	}
}

func TestHub_ConnectionCount(t *testing.T) {
	logger := zerolog.Nop()
	h := NewHub(logger)
	defer func() { h.cancel() }()

	if h.ConnectionCount() != 0 {
		t.Errorf("initial count = %d, want 0", h.ConnectionCount())
	}

	conn := mockConn(h)
	h.Register(conn)
	time.Sleep(15 * time.Millisecond)

	if h.ConnectionCount() != 1 {
		t.Errorf("count after register = %d, want 1", h.ConnectionCount())
	}
}

func TestHub_Id(t *testing.T) {
	logger := zerolog.Nop()
	conn := &Connection{
		topics: make(map[string]bool),
		send:   make(chan []byte, 8),
		logger: logger,
	}
	if conn.id() != "anon" {
		t.Errorf("id() without claims = %q, want 'anon'", conn.id())
	}
}

func TestConnection_Claims(t *testing.T) {
	logger := zerolog.Nop()
	conn := &Connection{
		topics: make(map[string]bool),
		send:   make(chan []byte, 8),
		logger: logger,
	}
	if conn.Claims() != nil {
		t.Error("Claims should be nil by default")
	}
}
