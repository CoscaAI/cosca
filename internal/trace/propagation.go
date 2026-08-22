package trace

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// ── W3C Trace Context (https://www.w3.org/TR/trace-context/) ──────────

const (
	// TraceparentHeader is the W3C trace context header.
	TraceparentHeader = "traceparent"
	// TracestateHeader carries vendor-specific trace data.
	TracestateHeader = "tracestate"

	// Version 00 is the current W3C spec version.
	traceVersion = "00"
	// TraceFlagsSampled indicates the trace is sampled.
	TraceFlagsSampled = "01"
)

// SpanID is a 16-character hex string (8 bytes).
type SpanID string

// NewSpanID generates a random W3C span ID.
func NewSpanID() SpanID {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// fallback: use timestamp for determinism
		n := uint64(nowNano())
		for i := 0; i < 8; i++ {
			b[i] = byte(n >> (i * 8))
		}
	}
	return SpanID(hex.EncodeToString(b[:]))
}

// Traceparent represents a parsed W3C traceparent header value.
// Format: {version}-{trace_id}-{parent_id}-{trace_flags}
// Example: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
type Traceparent struct {
	Version    string
	TraceID    string // 32-char hex
	ParentID   SpanID // 16-char hex
	TraceFlags string
}

// String returns the W3C-encoded traceparent header value.
func (tp Traceparent) String() string {
	return fmt.Sprintf("%s-%s-%s-%s", tp.Version, tp.TraceID, tp.ParentID, tp.TraceFlags)
}

// NewTraceparent creates a W3C traceparent from a Cosca TraceID.
// The Cosca TraceID (TRACE-YYYYMMDD-XXXX) is hashed to a 32-char hex
// compatible with W3C format.
func NewTraceparent(coscaID TraceID) Traceparent {
	// Convert Cosca ID to stable 32-char hex
	traceHex := coscaToHex(coscaID)

	return Traceparent{
		Version:    traceVersion,
		TraceID:    traceHex,
		ParentID:   NewSpanID(),
		TraceFlags: TraceFlagsSampled,
	}
}

// NewChildSpan creates a new span as a child of this traceparent.
func (tp Traceparent) NewChildSpan() Traceparent {
	return Traceparent{
		Version:    tp.Version,
		TraceID:    tp.TraceID,
		ParentID:   NewSpanID(),
		TraceFlags: tp.TraceFlags,
	}
}

// ParseTraceparent parses a W3C traceparent header value.
func ParseTraceparent(val string) (Traceparent, bool) {
	parts := strings.Split(val, "-")
	if len(parts) != 4 {
		return Traceparent{}, false
	}
	if parts[0] != "00" {
		return Traceparent{}, false
	}
	if len(parts[1]) != 32 || !isHex(parts[1]) {
		return Traceparent{}, false
	}
	if len(parts[2]) != 16 || !isHex(parts[2]) {
		return Traceparent{}, false
	}
	if parts[3] != "00" && parts[3] != "01" {
		return Traceparent{}, false
	}
	return Traceparent{
		Version:    parts[0],
		TraceID:    parts[1],
		ParentID:   SpanID(parts[2]),
		TraceFlags: parts[3],
	}, true
}

// Inject sets the W3C trace context headers on an HTTP request.
func (tp Traceparent) Inject(req *http.Request) {
	req.Header.Set(TraceparentHeader, tp.String())
	req.Header.Set("X-Cosca-Trace", string(tp.TraceID))
}

// Extract reads W3C trace context from an HTTP request.
func ExtractTraceparent(req *http.Request) (Traceparent, bool) {
	val := req.Header.Get(TraceparentHeader)
	if val == "" {
		return Traceparent{}, false
	}
	return ParseTraceparent(val)
}

// PropagateStep creates a child span for a step within a trace, returning
// both the new traceparent for downstream calls and the SpanID for recording.
func PropagateStep(parent Traceparent) (child Traceparent, spanID SpanID) {
	child = parent.NewChildSpan()
	return child, child.ParentID
}

// ── Helpers ─────────────────────────────────────────────────────────────

func coscaToHex(id TraceID) string {
	// Deriva um TraceID W3C estável (32-char hex) do Cosca TraceID via SHA-256:
	// o esquema anterior copiava os primeiros 16 bytes do ID e preenchia com
	// um hash fraco — COLISIONANTE para IDs que compartilham o prefixo.
	sum := sha256.Sum256([]byte(id.String()))
	return hex.EncodeToString(sum[:16])
}

func isHex(s string) bool {
	if s == "" {
		return false // string vazia não é um valor hex válido
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func nowNano() int64 {
	return 0 // simplified; caller should use time.Now().UnixNano()
}
