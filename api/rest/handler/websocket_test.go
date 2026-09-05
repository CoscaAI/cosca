package handler

// Tests for WebSocketHandler (websocket.go). Because isOriginAllowed is
// unexported, this file lives in package handler (white-box) and exercises
// both the origin matcher directly and the full ServeHTTP upgrade flow via a
// real nhooyr.io/websocket client.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/api/stream"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// TestIsOriginAllowed is a table-driven test for the origin allow-list logic.
// It covers the empty-origin rule, no-origins rejection, exact host matching,
// wildcard, subdomain wildcards, case-insensitivity, and port stripping.
func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		origins []string
		want    bool
	}{
		// Empty origin is always allowed (same-origin / non-browser clients).
		{name: "empty origin allowed with nil origins", origin: "", origins: nil, want: true},
		{name: "empty origin allowed with no origins configured", origin: "", origins: []string{}, want: true},

		// No origins configured rejects every cross-origin request.
		{name: "nil origins rejects cross-origin", origin: "http://example.com", origins: nil, want: false},
		{name: "empty origins rejects cross-origin", origin: "http://example.com", origins: []string{}, want: false},

		// Exact host matching.
		{name: "exact http match", origin: "http://example.com", origins: []string{"example.com"}, want: true},
		{name: "exact https match", origin: "https://example.com", origins: []string{"example.com"}, want: true},
		{name: "origin without scheme", origin: "example.com", origins: []string{"example.com"}, want: true},
		{name: "port stripped before matching", origin: "http://example.com:8080", origins: []string{"example.com"}, want: true},

		// Case-insensitive matching on both sides.
		{name: "uppercase origin matches", origin: "HTTP://EXAMPLE.COM", origins: []string{"example.com"}, want: true},
		{name: "uppercase pattern matches", origin: "http://example.com", origins: []string{"EXAMPLE.COM"}, want: true},

		// Wildcard allows everything (with a warning log).
		{name: "wildcard allows any origin", origin: "http://evil.example.com", origins: []string{"*"}, want: true},

		// Subdomain wildcard pattern.
		{name: "subdomain wildcard matches subdomain", origin: "http://api.example.com", origins: []string{"*.example.com"}, want: true},
		{name: "subdomain wildcard rejects apex", origin: "http://example.com", origins: []string{"*.example.com"}, want: false},
		{name: "subdomain wildcard rejects lookalike suffix", origin: "http://evilexample.com", origins: []string{"*.example.com"}, want: false},

		// Non-matching origins are rejected.
		{name: "unlisted origin rejected", origin: "http://other.com", origins: []string{"example.com"}, want: false},
		{name: "second pattern in list matches", origin: "https://a.org", origins: []string{"example.com", "a.org"}, want: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			h := NewWebSocketHandler(nil, nil, zerolog.Nop(), tt.origins)
			if got := h.isOriginAllowed(tt.origin); got != tt.want {
				t.Errorf("isOriginAllowed(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

// newTestWebSocketHandler builds a WebSocketHandler with a running hub and
// registers a shutdown on test cleanup.
func newTestWebSocketHandler(t *testing.T, jwtSecret []byte, allowedOrigins []string) *WebSocketHandler {
	t.Helper()
	hub := stream.NewHub(zerolog.Nop())
	t.Cleanup(func() { _ = hub.Shutdown(context.Background()) })
	return NewWebSocketHandler(hub, jwtSecret, zerolog.Nop(), allowedOrigins)
}

// TestWebSocketServeHTTPMissingToken verifies that an upgrade request without
// a token is rejected with 401 before the WebSocket handshake.
func TestWebSocketServeHTTPMissingToken(t *testing.T) {
	h := newTestWebSocketHandler(t, []byte("test-secret"), nil)

	req := httptest.NewRequest("GET", "/v1/ws", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !strings.Contains(resp["error"], "missing token") {
		t.Errorf("expected 'missing token' error, got %q", resp["error"])
	}
}

// TestWebSocketServeHTTPInvalidToken verifies that a malformed token is
// rejected with 401.
func TestWebSocketServeHTTPInvalidToken(t *testing.T) {
	h := newTestWebSocketHandler(t, []byte("test-secret"), nil)

	req := httptest.NewRequest("GET", "/v1/ws?token=not-a-valid-jwt", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWebSocketServeHTTPOriginBlocked verifies that a cross-origin request
// whose Origin is not in the allow-list is rejected with 403.
func TestWebSocketServeHTTPOriginBlocked(t *testing.T) {
	h := newTestWebSocketHandler(t, nil, []string{"localhost"})

	req := httptest.NewRequest("GET", "/v1/ws", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["error"] != "origin not allowed" {
		t.Errorf("expected error 'origin not allowed', got %q", resp["error"])
	}
}

// validTestToken returns a signed JWT that AuthenticateUpgrade accepts.
func validTestToken(t *testing.T, secret []byte) string {
	t.Helper()
	token, err := internalauth.GenerateToken(internalauth.Claims{
		Sub:      "user-1",
		Username: "tester",
		Role:     "admin",
		Type:     "access", // ValidateAccessToken requires Type == "access"
		Iat:      time.Now().Unix(),
		Exp:      time.Now().Add(time.Hour).Unix(),
	}, secret)
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}
	return token
}

// TestWebSocketServeHTTPUpgrade verifies the full happy path: authenticate,
// origin check, upgrade, subscribe, ping/pong, and hub broadcast delivery.
func TestWebSocketServeHTTPUpgrade(t *testing.T) {
	secret := []byte("test-secret-for-websocket-upgrade")
	h := newTestWebSocketHandler(t, secret, []string{"localhost"})
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token := validTestToken(t, secret)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/ws?token=" + token

	// Cross-origin dial: the Origin header must match the allowed patterns.
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"http://localhost"}},
	})
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// ping → pong round trip.
	if err := wsjson.Write(ctx, conn, map[string]interface{}{"type": "ping", "id": int64(7)}); err != nil {
		t.Fatalf("failed to write ping: %v", err)
	}
	var pong stream.OutgoingMessage
	if err := wsjson.Read(ctx, conn, &pong); err != nil {
		t.Fatalf("failed to read pong: %v", err)
	}
	if pong.Type != stream.MsgTypePong || pong.ID != 7 {
		t.Errorf("expected pong with id=7, got %+v", pong)
	}

	// subscribe → subscribed round trip.
	if err := wsjson.Write(ctx, conn, map[string]interface{}{"type": "subscribe", "topic": "chat", "id": int64(8)}); err != nil {
		t.Fatalf("failed to write subscribe: %v", err)
	}
	var sub stream.OutgoingMessage
	if err := wsjson.Read(ctx, conn, &sub); err != nil {
		t.Fatalf("failed to read subscribed: %v", err)
	}
	if sub.Type != stream.MsgTypeSubscribed || sub.Topic != "chat" || sub.ID != 8 {
		t.Errorf("expected subscribed chat id=8, got %+v", sub)
	}

	// A hub broadcast to the subscribed topic must reach this connection.
	h.hub.BroadcastJSON([]string{"chat"}, stream.OutgoingMessage{Type: "custom-event", Data: "payload"})
	var ev stream.OutgoingMessage
	if err := wsjson.Read(ctx, conn, &ev); err != nil {
		t.Fatalf("failed to read broadcast event: %v", err)
	}
	if ev.Type != "custom-event" {
		t.Errorf("expected broadcast event type 'custom-event', got %+v", ev)
	}
}

// TestWebSocketServeHTTPAnonymous verifies the anonymous path: when no JWT
// secret is configured, an upgrade without a token is accepted.
func TestWebSocketServeHTTPAnonymous(t *testing.T) {
	h := newTestWebSocketHandler(t, nil, nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket anonymously: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	if err := wsjson.Write(ctx, conn, map[string]interface{}{"type": "ping", "id": int64(1)}); err != nil {
		t.Fatalf("failed to write ping: %v", err)
	}
	var pong stream.OutgoingMessage
	if err := wsjson.Read(ctx, conn, &pong); err != nil {
		t.Fatalf("failed to read pong: %v", err)
	}
	if pong.Type != stream.MsgTypePong {
		t.Errorf("expected pong, got %+v", pong)
	}
}
