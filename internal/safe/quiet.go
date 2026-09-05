// Package safe provides helper functions that wrap common operations
// where errors are intentionally ignored, making intent explicit
// and self-documenting.
//
// Use these when:
//   - The error cannot be meaningfully handled (cleanup, defer)
//   - The error is expected and irrelevant (file may not exist)
//   - The operation is best-effort and failure is acceptable
//
// Do NOT use when:
//   - The error affects program correctness
//   - The error should be logged for debugging
//   - The caller needs to know about the failure
package safe

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// Close closes a resource implementing io.Closer, ignoring any error.
//
//	defer safe.Close(f)
func Close(c io.Closer) {
	_ = c.Close()
}

// Rollback rolls back a transaction, ignoring any error.
//
//	defer safe.Rollback(tx)
func Rollback(r interface{ Rollback() error }) {
	_ = r.Rollback()
}

// Remove deletes a file at path, ignoring any error.
//
//	safe.Remove("/tmp/temp-file")
func Remove(path string) {
	_ = os.Remove(path)
}

// RemoveAll recursively deletes a path, ignoring any error.
//
//	safe.RemoveAll("/tmp/workdir")
func RemoveAll(path string) {
	_ = os.RemoveAll(path)
}

// WriteFile writes data to a file, ignoring any error.
//
//	safe.WriteFile(path, data, 0o644)
func WriteFile(path string, data []byte, perm os.FileMode) {
	_ = os.WriteFile(path, data, perm)
}

// ── HTTP / Streaming helpers ──

// Write writes data to an http.ResponseWriter, logging any error.
// Use in HTTP handlers and SSE streaming where write failures mean
// the client disconnected and there's nothing to do but stop.
func Write(w http.ResponseWriter, data []byte) {
	if _, err := w.Write(data); err != nil {
		log.Printf("safe.Write: %v", err)
	}
}

// WriteString writes a string to an io.Writer, logging any error.
func WriteString(w io.Writer, s string) {
	if _, err := io.WriteString(w, s); err != nil {
		log.Printf("safe.WriteString: %v", err)
	}
}

// Fprintf formats and writes to an io.Writer, logging any error.
func Fprintf(w io.Writer, format string, a ...any) {
	if _, err := fmt.Fprintf(w, format, a...); err != nil {
		log.Printf("safe.Fprintf: %v", err)
	}
}
