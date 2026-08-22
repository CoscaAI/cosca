package stream

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/auth"
)

// ── SSETestWriter (mock de teste) ───────────────────────────────────────

func TestSSETestWriter(t *testing.T) {
	tw := NewSSETestWriter()
	if tw.Status() != http.StatusOK {
		t.Fatalf("initial status = %d, want 200", tw.Status())
	}

	tw.Header().Set("Content-Type", "text/event-stream")
	if got := tw.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("header = %q", got)
	}

	_, _ = tw.Write([]byte("data: hello\n\n"))
	if got := tw.Body().String(); got != "data: hello\n\n" {
		t.Fatalf("body = %q", got)
	}

	tw.WriteHeader(http.StatusCreated)
	if tw.Status() != http.StatusCreated {
		t.Fatalf("status after WriteHeader = %d", tw.Status())
	}
	tw.Flush() // no-op, não deve panicar
}

// ── SSEWriter ───────────────────────────────────────────────────────────

func TestSSEWriterEvents(t *testing.T) {
	tw := NewSSETestWriter()
	sw, err := NewSSEWriter(tw)
	if err != nil {
		t.Fatalf("NewSSEWriter: %v", err)
	}

	if err := sw.WriteEvent("thinking", "hello"); err != nil {
		t.Fatalf("WriteEvent: %v", err)
	}
	if err := sw.WriteEvent("result", map[string]interface{}{"id": 1}); err != nil {
		t.Fatalf("WriteEvent structured: %v", err)
	}
	if err := sw.WriteDone(); err != nil {
		t.Fatalf("WriteDone: %v", err)
	}
	if err := sw.WriteDoneWithDuration(42); err != nil {
		t.Fatalf("WriteDoneWithDuration: %v", err)
	}
	sw.Flush()

	out := tw.Body().String()
	if !strings.Contains(out, `"type":"thinking"`) || !strings.Contains(out, "hello") {
		t.Fatalf("missing thinking event: %q", out)
	}
	if !strings.Contains(out, `"type":"result"`) || !strings.Contains(out, `"id":1`) {
		t.Fatalf("missing result event: %q", out)
	}
	if !strings.Contains(out, `"type":"done"`) {
		t.Fatalf("missing done event: %q", out)
	}

	if err := sw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if sw.ResponseWriter() != http.ResponseWriter(tw) {
		t.Fatal("ResponseWriter mismatch")
	}
}

func TestSSEWriterWriteError(t *testing.T) {
	tw := NewSSETestWriter()
	sw, _ := NewSSEWriter(tw)
	if err := sw.WriteError(errFake); err != nil {
		t.Fatalf("WriteError: %v", err)
	}
	out := tw.Body().String()
	if !strings.Contains(out, "error") {
		t.Fatalf("error event: %q", out)
	}
}

func TestFormatSSEEvent(t *testing.T) {
	// String: content é JSON-escaped.
	s, err := formatSSEEvent("thinking", "say \"hi\"")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if m["type"] != "thinking" {
		t.Fatalf("type = %v", m["type"])
	}
	if m["content"] != `say "hi"` {
		t.Fatalf("content = %v", m["content"])
	}
	// Estrutura: campos fundidos com type.
	s2, _ := formatSSEEvent("result", map[string]interface{}{"ok": true})
	if !strings.Contains(s2, `"type":"result"`) || !strings.Contains(s2, `"ok":true`) {
		t.Fatalf("structured: %q", s2)
	}
}

var errFake = &fakeErr{"stream error"}

type fakeErr struct{ msg string }

func (e *fakeErr) Error() string { return e.msg }

// ── Auth (JWT upgrade) ──────────────────────────────────────────────────

func TestAuthenticateUpgradeNoSecret(t *testing.T) {
	// Sem segredo → anonymous permitido.
	req := httptest.NewRequest("GET", "http://x", nil)
	claims, err := AuthenticateUpgrade(req, nil)
	if err != nil || claims != nil {
		t.Fatalf("no-secret: claims=%v err=%v (esperado nil/nil)", claims, err)
	}
}

func TestAuthenticateUpgrade(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	// Gera um token válido via auth internals (se disponível) ou pelo pacote
	// auth — aqui usamos um token genérico inválido primeiro.
	req := httptest.NewRequest("GET", "http://x", nil)

	// Sem token → ErrMissingToken.
	if _, err := AuthenticateUpgrade(req, secret); err == nil {
		t.Fatal("missing token must error")
	}

	// Token inválido → ErrTokenViolation.
	req.Header.Set("Authorization", "Bearer invalid-token")
	if _, err := AuthenticateUpgrade(req, secret); err == nil {
		t.Fatal("invalid token must error")
	}

	// Bearer malformado.
	req2 := httptest.NewRequest("GET", "http://x", nil)
	req2.Header.Set("Authorization", "Basic abc")
	if _, err := AuthenticateUpgrade(req2, secret); err == nil {
		t.Fatal("basic auth must not be accepted as bearer")
	}

	// Token via query param.
	req3 := httptest.NewRequest("GET", "http://x?token=invalid", nil)
	if _, err := AuthenticateUpgrade(req3, secret); err == nil {
		t.Fatal("invalid query token must error")
	}
}

func TestBearerToken(t *testing.T) {
	req := httptest.NewRequest("GET", "http://x", nil)
	req.Header.Set("Authorization", "Bearer abc123")
	if got := bearerToken(req); got != "abc123" {
		t.Fatalf("bearer = %q", got)
	}
	// Case-insensitive scheme.
	req.Header.Set("Authorization", "bearer xyz")
	if got := bearerToken(req); got != "xyz" {
		t.Fatalf("bearer lower = %q", got)
	}
	// Sem header.
	req2 := httptest.NewRequest("GET", "http://x", nil)
	if got := bearerToken(req2); got != "" {
		t.Fatalf("no header = %q", got)
	}
	_ = time.Now
	_ = auth.ErrTokenExpired
}
