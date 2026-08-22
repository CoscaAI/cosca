package stream

import (
	"bytes"
	"net/http"
	"sync"
)

// SSETestWriter implements http.ResponseWriter and http.Flusher for testing.
// It captures all SSE output in a bytes.Buffer so handlers that write SSE
// events can be tested with standard httptest patterns.
//
// Usage:
//
//	tw := stream.NewSSETestWriter()
//	sw, err := stream.NewSSEWriter(tw)
//	_ = sw.WriteEvent("thinking", "hello")
//	output := tw.Body().String() // "data: ...\n\n"
type SSETestWriter struct {
	header http.Header
	buf    bytes.Buffer
	status int
	mu     sync.Mutex
}

// NewSSETestWriter creates a test ResponseWriter that supports flushing.
func NewSSETestWriter() *SSETestWriter {
	return &SSETestWriter{
		header: make(http.Header),
		status: http.StatusOK,
	}
}

// Header returns the HTTP header map for the test writer.
func (tw *SSETestWriter) Header() http.Header {
	return tw.header
}

// Write appends data to the internal buffer.
func (tw *SSETestWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return tw.buf.Write(b)
}

// WriteHeader records the HTTP status code.
func (tw *SSETestWriter) WriteHeader(statusCode int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.status = statusCode
}

// Status returns the recorded HTTP status code.
func (tw *SSETestWriter) Status() int {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return tw.status
}

// Body returns the captured output as a string.
func (tw *SSETestWriter) Body() *bytes.Buffer {
	return &tw.buf
}

// Flush is a no-op for the test writer (data is captured in the buffer
// synchronously).
func (tw *SSETestWriter) Flush() {
	// No-op: all writes go directly to the buffer.
}

// Compile-time interface check.
var (
	_ http.ResponseWriter = (*SSETestWriter)(nil)
	_ http.Flusher        = (*SSETestWriter)(nil)
)
