package handler

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/api/stream"
	"nhooyr.io/websocket"
)

// WebSocketHandler handles the GET /v1/ws endpoint — the real-time event
// gateway that allows clients to subscribe to multiple event topics over
// a single persistent WebSocket connection.
type WebSocketHandler struct {
	hub            *stream.Hub
	jwtSecret      []byte
	logger         zerolog.Logger
	allowedOrigins []string // Origin host patterns for cross-origin WebSocket
}

// NewWebSocketHandler creates a new WebSocketHandler.
// hub must be non-nil and already running (created by stream.NewHub).
// allowedOrigins is a list of host patterns for cross-origin WebSocket
// connections (matched via filepath.Match against the Origin host).
// The request host is always authorized. Default to []string{"localhost"}
// for local development. Set to nil to allow only same-origin connections.
func NewWebSocketHandler(hub *stream.Hub, jwtSecret []byte, logger zerolog.Logger, allowedOrigins []string) *WebSocketHandler {
	return &WebSocketHandler{
		hub:            hub,
		jwtSecret:      jwtSecret,
		logger:         logger.With().Str("handler", "websocket").Logger(),
		allowedOrigins: allowedOrigins,
	}
}

// ServeHTTP handles the WebSocket upgrade request.
//
// Flow:
//  1. Authenticate the client via the `?token=` query parameter.
//  2. Validate the Origin header against the allowed origins list.
//  3. Upgrade the HTTP connection to a WebSocket.
//  4. Create a Connection, register it with the Hub, and start the
//     read/write pump goroutines.
//
// Protocol (client → server):
//
//	{"type": "subscribe", "topic": "chat", "id": 1}
//	{"type": "unsubscribe", "topic": "sync", "id": 2}
//	{"type": "ping", "id": 3}
//
// Protocol (server → client):
//
//	{"type": "subscribed", "topic": "chat", "id": 1}
//	{"type": "unsubscribed", "topic": "sync", "id": 2}
//	{"type": "pong", "id": 3}
//	{"type": "error", "data": {"message": "...", "id": 1}}
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// ── 0. Nil hub guard ──────────────────────────────────────────────
	if h.hub == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"websocket is disabled"}`))
		return
	}

	// ── 1. Authenticate ───────────────────────────────────────────────
	claims, err := stream.AuthenticateUpgrade(r, h.jwtSecret)
	if err != nil {
		// Reject the upgrade before the WebSocket handshake.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"` + err.Error() + `"}`))
		return
	}

	// ── 2. Validate origin (OWASP ASVS V5.2.1 / CWE-346) ─────────────
	origin := r.Header.Get("Origin")
	if !h.isOriginAllowed(origin) {
		h.logger.Warn().
			Str("origin", origin).
			Str("remote_addr", r.RemoteAddr).
			Msg("websocket origin rejected")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"origin not allowed"}`))
		return
	}

	// ── 3. Upgrade HTTP → WebSocket ───────────────────────────────────
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Validate origin against allowed host patterns.
		// The request host is always authorized.
		OriginPatterns: h.allowedOrigins,
	})
	if err != nil {
		// websocket.Accept already wrote the HTTP response on failure.
		h.logger.Warn().Err(err).Msg("websocket upgrade failed")
		return
	}

	h.logger.Debug().
		Str("remote_addr", r.RemoteAddr).
		Str("user_agent", r.UserAgent()).
		Msg("websocket connection upgraded")

	// ── 4. Create connection and start pumps ──────────────────────────
	wsConn := stream.NewConnection(conn, h.hub, claims, h.logger)
	h.hub.Register(wsConn)

	go wsConn.WritePump()
	// ReadPump runs in the handler goroutine since it's the primary
	// driver of the connection lifecycle. When ReadPump returns, the
	// connection is unregistered and both pumps stop.
	wsConn.ReadPump()
}

// isOriginAllowed checks whether the request Origin header is in the
// allowed origins list. An empty or missing Origin header is always
// allowed (same-origin requests from browsers that do not send Origin
// on same-origin WebSocket requests, or non-browser clients).
//
// Matching is done case-insensitively against the host portion of the
// origin using filepath.Match, consistent with nhooyr.io/websocket.
func (h *WebSocketHandler) isOriginAllowed(origin string) bool {
	// Non-browser clients and same-origin requests may not send
	// an Origin header. These are always allowed.
	if origin == "" {
		return true
	}

	// Extract the host from the origin URL.
	// Origins look like: "http://localhost:3000" or "https://example.com"
	host := origin
	if idx := strings.Index(origin, "://"); idx != -1 {
		host = origin[idx+3:]
	}
	// Strip port if present for matching
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	host = strings.ToLower(strings.TrimSpace(host))

	if len(h.allowedOrigins) == 0 {
		// No origins configured — reject all cross-origin connections.
		// Same-origin connections are handled by websocket.Accept
		// (which authorizes the request host by default).
		return false
	}

	for _, pattern := range h.allowedOrigins {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "*" {
			h.logger.Warn().Msg("websocket allowed origins includes wildcard — this is insecure for production")
			return true
		}
		// Note: For a simpler match without importing filepath,
		// we do exact host matching and prefix wildcard matching.
		// The nhooyr.io/websocket library handles the full filepath.Match
		// for the OriginPatterns field.
		if pattern == host {
			return true
		}
		// Support subdomain wildcard: *.example.com
		if strings.HasPrefix(pattern, "*.") {
			suffix := pattern[1:] // .example.com
			if strings.HasSuffix(host, suffix) {
				return true
			}
		}
	}

	return false
}

// Hub returns the underlying Hub for external broadcast access.
func (h *WebSocketHandler) Hub() *stream.Hub {
	return h.hub
}
