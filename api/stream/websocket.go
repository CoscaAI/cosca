package stream

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// ── Protocol message types ──────────────────────────────────────────────

// wsInMessage is the client→server envelope parsed from JSON text frames.
type wsInMessage struct {
	Type  string `json:"type"`
	Topic string `json:"topic,omitempty"`
	ID    int64  `json:"id,omitempty"`
}

// OutgoingMessage is the server→client envelope sent as JSON text frames.
type OutgoingMessage struct {
	Type  string      `json:"type"`
	Topic string      `json:"topic,omitempty"`
	ID    int64       `json:"id,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}

// Protocol constants.
const (
	MsgTypeSubscribe    = "subscribe"
	MsgTypeUnsubscribe  = "unsubscribe"
	MsgTypePing         = "ping"
	MsgTypePong         = "pong"
	MsgTypeSubscribed   = "subscribed"
	MsgTypeUnsubscribed = "unsubscribed"
	MsgTypeError        = "error"

	// Application-level heartbeat (protocol-level ping/pong is handled
	// automatically by nhooyr.io/websocket).
	writeTimeout = 30 * time.Second
	readTimeout  = 60 * time.Second
	pingInterval = 25 * time.Second

	// sendChannelBuffer is the size of each connection's outbound channel.
	sendChannelBuffer = 64
)

// ── Hub ─────────────────────────────────────────────────────────────────

// Hub manages all WebSocket connections and topic subscriptions.
//
// The Hub runs a single event-loop goroutine (via Run) that processes
// register, unregister, and broadcast events in a thread-safe manner.
// Subscribe and Unsubscribe are synchronized via the hub's mutex and
// are safe to call from any goroutine (typically the connection's read
// pump).
type Hub struct {
	connections map[*Connection]bool
	topics      map[string]map[*Connection]bool
	register    chan *Connection
	unregister  chan *Connection
	broadcast   chan BroadcastMessage
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	logger      zerolog.Logger
}

// BroadcastMessage is a message targeted at one or more topics.
// Connections subscribed to any of the listed topics will receive
// the payload.
type BroadcastMessage struct {
	Topics  []string
	Payload []byte
}

// NewHub creates a Hub and starts its event-loop goroutine.
// The caller is responsible for calling Shutdown when the Hub is no
// longer needed.
func NewHub(logger zerolog.Logger) *Hub {
	if logger.GetLevel() == zerolog.Disabled {
		logger = log.With().Str("component", "ws-hub").Logger()
	} else {
		logger = logger.With().Str("component", "ws-hub").Logger()
	}
	ctx, cancel := context.WithCancel(context.Background())

	h := &Hub{
		connections: make(map[*Connection]bool),
		topics:      make(map[string]map[*Connection]bool),
		register:    make(chan *Connection),
		unregister:  make(chan *Connection),
		broadcast:   make(chan BroadcastMessage, 32),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
	}
	go h.Run()
	return h
}

// Run is the event loop that processes register, unregister, and
// broadcast events. It runs until the Hub context is cancelled.
func (h *Hub) Run() {
	defer func() {
		h.logger.Debug().Msg("hub event loop stopped")
	}()

	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.connections[conn] = true
			h.logger.Debug().
				Str("conn_id", conn.id()).
				Int("total", len(h.connections)).
				Msg("connection registered")
			h.mu.Unlock()

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.connections[conn]; ok {
				delete(h.connections, conn)
				// Clean up all topic subscriptions for this connection.
				for topic := range conn.topics {
					if topicConns, exists := h.topics[topic]; exists {
						delete(topicConns, conn)
						if len(topicConns) == 0 {
							delete(h.topics, topic)
						}
					}
				}
				h.logger.Debug().
					Str("conn_id", conn.id()).
					Int("total", len(h.connections)).
					Msg("connection unregistered")
			}
			h.mu.Unlock()

			// Do not close conn.send here. sendJSON can run concurrently with
			// unregister; closing it would introduce a send-on-closed-channel
			// race. Cancellation makes both pumps stop and the channel is owned
			// by the connection for its entire lifetime.
			conn.cancel()

		case msg := <-h.broadcast:
			h.mu.RLock()
			for _, topic := range msg.Topics {
				for conn := range h.topics[topic] {
					select {
					case conn.send <- msg.Payload:
					default:
						// Drop message for slow consumer — prevents
						// one slow client from blocking the entire hub.
						conn.logger.Warn().
							Str("topic", topic).
							Msg("dropping message for slow consumer")
					}
				}
			}
			h.mu.RUnlock()

		case <-h.ctx.Done():
			return
		}
	}
}

// Subscribe adds a connection to a topic. Safe for concurrent use.
// Returns true if the subscription was newly added.
func (h *Hub) Subscribe(topic string, conn *Connection) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.topics[topic] == nil {
		h.topics[topic] = make(map[*Connection]bool)
	}
	if h.topics[topic][conn] {
		return false // already subscribed
	}
	h.topics[topic][conn] = true
	conn.topics[topic] = true

	h.logger.Debug().
		Str("conn_id", conn.id()).
		Str("topic", topic).
		Int("subscribers", len(h.topics[topic])).
		Msg("subscribed to topic")
	return true
}

// Unsubscribe removes a connection from a topic. Safe for concurrent use.
// Returns true if the connection was subscribed.
func (h *Hub) Unsubscribe(topic string, conn *Connection) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	topicConns, exists := h.topics[topic]
	if !exists || !topicConns[conn] {
		return false
	}
	delete(topicConns, conn)
	delete(conn.topics, topic)
	if len(topicConns) == 0 {
		delete(h.topics, topic)
	}

	h.logger.Debug().
		Str("conn_id", conn.id()).
		Str("topic", topic).
		Msg("unsubscribed from topic")
	return true
}

// Register adds a connection to the hub. The connection will start
// receiving broadcasts for topics it subscribes to.
func (h *Hub) Register(conn *Connection) {
	select {
	case h.register <- conn:
	case <-h.ctx.Done():
	}
}

// Unregister removes a connection from the hub and cleans up all its
// topic subscriptions.
func (h *Hub) Unregister(conn *Connection) {
	select {
	case h.unregister <- conn:
	case <-h.ctx.Done():
	}
}

// Broadcast sends a payload to all connections subscribed to any of
// the listed topics. Safe for concurrent use.
func (h *Hub) Broadcast(msg BroadcastMessage) {
	select {
	case h.broadcast <- msg:
	case <-h.ctx.Done():
	}
}

// BroadcastJSON is a convenience wrapper that JSON-marshals the payload
// before broadcasting to the given topics.
func (h *Hub) BroadcastJSON(topics []string, msg OutgoingMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error().Err(err).Strs("topics", topics).Msg("failed to marshal broadcast message")
		return
	}
	h.Broadcast(BroadcastMessage{Topics: topics, Payload: data})
}

// BroadcastEvent is a convenience wrapper for broadcasting a typed event
// to the given topic(s). It constructs an OutgoingMessage with the given
// event type and data payload.
//
// Usage:
//
//	hub.BroadcastEvent([]string{"chat"}, "chat_started", nil)
func (h *Hub) BroadcastEvent(topics []string, eventType string, data interface{}) {
	h.BroadcastJSON(topics, OutgoingMessage{
		Type: eventType,
		Data: data,
	})
}

// Shutdown gracefully shuts down the hub: stops the event loop, closes
// all active connections with a going-away frame, and drains channels.
//
// After the context deadline, connections that haven't finished closing
// are abandoned (their goroutines will exit when they detect the cancelled
// hub context).
func (h *Hub) Shutdown(ctx context.Context) error {
	h.logger.Info().Int("connections", h.ConnectionCount()).Msg("shutting down websocket hub")

	// Signal the Run loop to stop accepting new register/broadcast events.
	h.cancel()

	// Snapshot the connections before stopping their pumps. Do not clear the
	// maps here: they are also the authoritative set of connections that must
	// be drained, and clearing them makes shutdown return before the handlers
	// have actually stopped.
	h.mu.RLock()
	connections := make([]*Connection, 0, len(h.connections))
	for conn := range h.connections {
		connections = append(connections, conn)
	}
	h.mu.RUnlock()

	// Send close frames and cancel all active pumps. A pump which starts after
	// this point observes shutdown and exits without entering its handler.
	done := make([]<-chan struct{}, 0, len(connections))
	for _, conn := range connections {
		done = append(done, conn.stopPumps())
		conn.closeWithReason(websocket.StatusGoingAway, "server shutting down")
		conn.cancel()
	}
	connCount := len(connections)

	// Wait for the real pump/handler goroutines, not merely for bookkeeping to
	// change. The send channel deliberately remains owned by the connection;
	// cancellation is the only shutdown signal, so concurrent senders cannot
	// panic with a send-on-closed-channel.
	for _, pumpDone := range done {
		select {
		case <-ctx.Done():
			h.logger.Warn().
				Int("remaining", h.ConnectionCount()).
				Int("initial", connCount).
				Msg("hub shutdown deadline exceeded, abandoning connections")
			return ctx.Err()
		case <-pumpDone:
		}
	}

	// Run was cancelled above, so unregister messages may no longer be
	// consumed. Remove only the connections whose pumps have finished, while
	// retaining the invariant that the map is never cleared before the wait.
	h.mu.Lock()
	for _, conn := range connections {
		if _, ok := h.connections[conn]; !ok {
			continue
		}
		delete(h.connections, conn)
		for topic := range conn.topics {
			if topicConns := h.topics[topic]; topicConns != nil {
				delete(topicConns, conn)
				if len(topicConns) == 0 {
					delete(h.topics, topic)
				}
			}
		}
	}
	h.mu.Unlock()

	h.logger.Info().Msg("websocket hub shut down cleanly")
	return nil
}

// ConnectionCount returns the number of currently active connections.
func (h *Hub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// TopicCount returns the number of active topics.
func (h *Hub) TopicCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.topics)
}

// ── Connection ──────────────────────────────────────────────────────────

// Connection wraps a WebSocket connection with metadata.
// Each connection runs two goroutines: readPump and writePump.
type Connection struct {
	conn     *websocket.Conn
	hub      *Hub
	topics   map[string]bool
	claims   *internalauth.Claims
	send     chan []byte
	ctx      context.Context
	cancel   context.CancelFunc
	logger   zerolog.Logger
	pumpMu   sync.Mutex
	pumps    int
	done     chan struct{}
	stopping bool
}

// NewConnection creates a Connection wrapping an already-accepted
// WebSocket connection. The caller is responsible for calling
// hub.Register(conn) and then starting the read/write pump goroutines.
func NewConnection(conn *websocket.Conn, hub *Hub, claims *internalauth.Claims, logger zerolog.Logger) *Connection {
	ctx, cancel := context.WithCancel(hub.ctx)
	done := make(chan struct{})
	close(done)

	if logger.GetLevel() == zerolog.Disabled {
		logger = log.With().Str("component", "ws-conn").Logger()
	} else {
		logger = logger.With().Str("component", "ws-conn").Logger()
	}

	return &Connection{
		conn:   conn,
		hub:    hub,
		topics: make(map[string]bool),
		claims: claims,
		send:   make(chan []byte, sendChannelBuffer),
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
		done:   done,
	}
}

// startPump registers a pump before it can observe cancellation. The hub
// does not own send, so this small lifecycle gate is what lets Shutdown wait
// for pumps that really entered their handlers without assuming that every
// Connection has already started both pumps.
func (c *Connection) startPump() bool {
	c.pumpMu.Lock()
	defer c.pumpMu.Unlock()
	if c.stopping {
		return false
	}
	if c.pumps == 0 {
		c.done = make(chan struct{})
	}
	c.pumps++
	return true
}

func (c *Connection) finishPump() {
	c.pumpMu.Lock()
	defer c.pumpMu.Unlock()
	c.pumps--
	if c.pumps == 0 {
		close(c.done)
	}
}

func (c *Connection) stopPumps() <-chan struct{} {
	c.pumpMu.Lock()
	c.stopping = true
	done := c.done
	c.pumpMu.Unlock()
	return done
}

// id returns a short identifier for logging (derived from the claims username).
func (c *Connection) id() string {
	if c.claims != nil {
		return c.claims.Username
	}
	return "anon"
}

// Claims returns the JWT claims associated with this connection.
func (c *Connection) Claims() *internalauth.Claims {
	return c.claims
}

// ReadPump reads incoming messages from the WebSocket connection and
// dispatches them as subscribe, unsubscribe, or ping commands.
//
// It runs until the connection is closed (by either side) or the hub
// context is cancelled. When it exits, it triggers unregistration.
func (c *Connection) ReadPump() {
	if !c.startPump() {
		return
	}
	defer func() {
		c.finishPump()
		c.cancel()
		c.hub.Unregister(c)
		c.logger.Debug().Msg("read pump stopped")
	}()

	for {
		// Set a read deadline so we don't block forever on a dead connection.
		readCtx, readCancel := context.WithTimeout(c.ctx, readTimeout)
		_, data, err := c.conn.Read(readCtx)
		readCancel()
		if err != nil {
			// Expected when connection is closed (by client or server).
			if websocket.CloseStatus(err) != -1 {
				c.logger.Debug().
					Int("status", int(websocket.CloseStatus(err))).
					Msg("websocket closed")
			} else if c.ctx.Err() != nil {
				c.logger.Debug().Msg("connection context cancelled")
			} else {
				c.logger.Error().Err(err).Msg("websocket read error")
			}
			return
		}

		var msg wsInMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.logger.Warn().Err(err).Str("raw", string(data)).Msg("invalid client message")
			c.sendJSON(OutgoingMessage{
				Type: MsgTypeError,
				Data: map[string]string{"message": "invalid JSON: " + err.Error()},
			})
			continue
		}

		switch msg.Type {
		case MsgTypeSubscribe:
			if msg.Topic == "" {
				c.sendJSON(OutgoingMessage{
					Type: MsgTypeError,
					ID:   msg.ID,
					Data: map[string]string{"message": "topic is required for subscribe"},
				})
				continue
			}
			c.hub.Subscribe(msg.Topic, c)
			c.sendJSON(OutgoingMessage{
				Type:  MsgTypeSubscribed,
				Topic: msg.Topic,
				ID:    msg.ID,
			})

		case MsgTypeUnsubscribe:
			if msg.Topic == "" {
				c.sendJSON(OutgoingMessage{
					Type: MsgTypeError,
					ID:   msg.ID,
					Data: map[string]string{"message": "topic is required for unsubscribe"},
				})
				continue
			}
			c.hub.Unsubscribe(msg.Topic, c)
			c.sendJSON(OutgoingMessage{
				Type:  MsgTypeUnsubscribed,
				Topic: msg.Topic,
				ID:    msg.ID,
			})

		case MsgTypePing:
			c.sendJSON(OutgoingMessage{
				Type: MsgTypePong,
				ID:   msg.ID,
			})

		default:
			c.sendJSON(OutgoingMessage{
				Type: MsgTypeError,
				ID:   msg.ID,
				Data: map[string]string{"message": "unknown message type: " + msg.Type},
			})
		}
	}
}

// WritePump reads messages from the send channel and writes them to the
// WebSocket connection. It also sends periodic ping frames to keep the
// connection alive.
//
// It runs until the send channel is closed or the connection context is
// cancelled.
func (c *Connection) WritePump() {
	if !c.startPump() {
		return
	}
	ticker := time.NewTicker(pingInterval)
	defer func() {
		c.finishPump()
		ticker.Stop()
		c.logger.Debug().Msg("write pump stopped")
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// Channel closed — connection is being shut down.
				return
			}
			writeCtx, writeCancel := context.WithTimeout(c.ctx, writeTimeout)
			err := c.conn.Write(writeCtx, websocket.MessageText, message)
			writeCancel()
			if err != nil {
				c.logger.Error().Err(err).Msg("websocket write error")
				return
			}

		case <-ticker.C:
			// Application-level ping to keep connection alive.
			writeCtx, writeCancel := context.WithTimeout(c.ctx, writeTimeout)
			err := c.conn.Write(writeCtx, websocket.MessageText, []byte(`{"type":"ping"}`))
			writeCancel()
			if err != nil {
				c.logger.Error().Err(err).Msg("websocket ping write error")
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

// Close sends a close frame with the given status code and reason.
func (c *Connection) Close(status websocket.StatusCode, reason string) error {
	c.logger.Debug().
		Int("status", int(status)).
		Str("reason", reason).
		Msg("closing connection")
	return c.conn.Close(status, reason)
}

// closeWithReason is the internal close helper that doesn't log.
func (c *Connection) closeWithReason(status websocket.StatusCode, reason string) {
	_ = c.conn.Close(status, reason)
}

// sendJSON marshals an OutgoingMessage and sends it on the connection's
// send channel. If the channel is full, the message is dropped.
func (c *Connection) sendJSON(msg OutgoingMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to marshal ws message")
		return
	}
	select {
	case c.send <- data:
	case <-c.ctx.Done():
		return
	default:
		c.logger.Warn().Msg("send buffer full, dropping message")
	}
}

// ── Compile-time interface checks ───────────────────────────────────────

var _ = wsjson.Write // ensure import is used (wsjson is imported for future Read/Write helpers)
