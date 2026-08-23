package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// ──────────────────────────────────────────────────────────────
// WebSocket Client (real)
// ──────────────────────────────────────────────────────────────

// WebSocketClient is a real WebSocket bridge client for Unreal Engine.
// It implements the Client interface, speaking JSON over WebSocket to the
// UCoscaWorldSubsystem WebSocket server running in Unreal.
type WebSocketClient struct {
	mu       sync.Mutex
	conn     *websocket.Conn
	connected bool

	inbox   chan Message
	handlers map[MessageType]func(Message)
	// seq is an incrementing counter for request/response correlation.
	seq int64

	// reconnect backoff config
	reconnectDelay time.Duration
	pingInterval   time.Duration
}

// NewWebSocketClient creates a real WebSocket bridge client.
func NewWebSocketClient() *WebSocketClient {
	return &WebSocketClient{
		inbox:          make(chan Message, 64),
		handlers:       make(map[MessageType]func(Message)),
		reconnectDelay: 2 * time.Second,
		pingInterval:   30 * time.Second,
	}
}

// Connect establishes the WebSocket connection to the Unreal server.
func (c *WebSocketClient) Connect(ctx context.Context, url string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPClient:        http.DefaultClient,
		CompressionMode:   websocket.CompressionDisabled,
	})
	if err != nil {
		return fmt.Errorf("websocket dial %s: %w", url, err)
	}

	c.conn = conn
	c.connected = true

	// Start read pump
	go c.readLoop()

	return nil
}

// Disconnect closes the WebSocket connection.
func (c *WebSocketClient) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		c.connected = false
		return nil
	}

	err := c.conn.Close(websocket.StatusNormalClosure, "cosca disconnect")
	c.conn = nil
	c.connected = false
	return err
}

// Send serializes a Message and writes it to the connection.
func (c *WebSocketClient) Send(ctx context.Context, msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil || !c.connected {
		return fmt.Errorf("not connected")
	}

	// Assign a sequence number if not set
	if msg.ID == "" {
		c.seq++
		msg.ID = fmt.Sprintf("msg-%d", c.seq)
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	if msg.Payload == nil {
		msg.Payload = json.RawMessage("{}")
	}

	return wsjson.Write(ctx, c.conn, msg)
}

// Receive returns the next message from the inbox channel.
// The readLoop (started in Connect) feeds messages into the inbox. If no
// handler is registered for a message's type, it is queued here. If a handler
// is registered, the readLoop dispatches it directly.
func (c *WebSocketClient) Receive(ctx context.Context) (Message, error) {
	select {
	case msg := <-c.inbox:
		return msg, nil
	case <-ctx.Done():
		return Message{}, ctx.Err()
	}
}

// SendFrame sends a camera frame to Cosca (for testing / observation loop).
func (c *WebSocketClient) SendFrame(ctx context.Context, frame FramePayload) error {
	data, _ := json.Marshal(frame)
	return c.Send(ctx, Message{
		Type:      MessageFrame,
		Timestamp: time.Now(),
		Payload:   data,
	})
}

// SendAction sends an action command to Unreal.
func (c *WebSocketClient) SendAction(ctx context.Context, action ActionPayload) error {
	data, _ := json.Marshal(action)
	return c.Send(ctx, Message{
		Type:      MessageAction,
		Timestamp: time.Now(),
		Payload:   data,
	})
}

// OnMessage registers a handler for incoming messages by type.
func (c *WebSocketClient) OnMessage(msgType MessageType, handler func(Message)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[msgType] = handler
}

// IsConnected returns true if the connection is active.
func (c *WebSocketClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected && c.conn != nil
}

// readLoop continuously reads messages from the connection.
// If a handler is registered for a message type, it dispatches directly.
// Otherwise, it queues the message into the inbox for Receive().
func (c *WebSocketClient) readLoop() {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return
	}

	for {
		ctx := context.Background()
		// Read raw WebSocket frame (not wsjson.Read) because the Unreal
		// WebSocketNetworking plugin sends responses as BINARY frames.
		_, data, err := conn.Read(ctx)
		if err != nil {
			// Connection closed or error — mark disconnected
			c.mu.Lock()
			c.connected = false
			c.conn = nil
			c.mu.Unlock()
			close(c.inbox)
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			// Bad JSON — skip
			continue
		}

		c.mu.Lock()
		handler, ok := c.handlers[msg.Type]
		c.mu.Unlock()

		if ok && handler != nil {
			go handler(msg)
		} else {
			select {
			case c.inbox <- msg:
			default:
				// inbox full — drop (backpressure)
			}
		}
	}
}
