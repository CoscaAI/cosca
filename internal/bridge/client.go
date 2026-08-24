// Package bridge provides the WebSocket client for communicating with Unreal Engine.
//
// The bridge is the "nervous system" between Cosca (cognition) and Unreal (body).
// It handles:
//   - Frame reception (camera images from Unreal)
//   - Action dispatch (commands to Unreal)
//   - State synchronization (world state exchange)
//   - Event streaming (real-time events)
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Message types
// ──────────────────────────────────────────────────────────────

// MessageType classifies bridge messages.
type MessageType string

const (
	// Unreal → Cosca (perception)
	MessageFrame       MessageType = "frame"        // camera frame (PNG/JPEG bytes)
	MessageAudio       MessageType = "audio"        // audio sample (PCM/WAV bytes)
	MessageEvent       MessageType = "event"        // world event (collision, trigger)
	MessageStateSync   MessageType = "state_sync"   // world state snapshot

	// Cosca → Unreal (action)
	MessageAction      MessageType = "action"       // execute action (move, interact)
	MessageSpawn       MessageType = "spawn"        // spawn entity
	MessageDestroy     MessageType = "destroy"      // destroy entity
	MessageModify      MessageType = "modify"       // modify entity properties
	MessageImportMesh  MessageType = "import_mesh"  // import GLB + spawn (AssetID → Registry → Mesh)
	MessageWeather     MessageType = "weather"      // change weather
	MessageTime        MessageType = "time"         // change time of day

	// Bidirectional
	MessagePing        MessageType = "ping"
	MessagePong        MessageType = "pong"
	MessageError       MessageType = "error"
)

// Message is the envelope for all bridge communication.
type Message struct {
	Type      MessageType     `json:"type"`
	ID        string          `json:"id"`        // unique message ID
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`   // type-specific data
}

// ──────────────────────────────────────────────────────────────
// Frame payload (Unreal → Cosca)
// ──────────────────────────────────────────────────────────────

// FramePayload carries a camera frame from Unreal.
type FramePayload struct {
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Format   string `json:"format"`   // "png", "jpeg"
	Data     []byte `json:"data"`     // raw image bytes
	CameraID string `json:"camera_id"`
	Pose     struct {
		Position [3]float64 `json:"position"`
		Rotation [4]float64 `json:"rotation"` // quaternion
	} `json:"pose"`
}

// ──────────────────────────────────────────────────────────────
// Action payload (Cosca → Unreal)
// ──────────────────────────────────────────────────────────────

// ActionPayload sends a command to Unreal.
type ActionPayload struct {
	EntityID string            `json:"entity_id"`
	Action   string            `json:"action"`    // "move_to", "interact", "look_at", "speak"
	Params   map[string]any    `json:"params"`    // action-specific parameters
}

// SpawnPayload creates a new entity in Unreal.
type SpawnPayload struct {
	Type     string            `json:"type"`      // "npc", "object", "particle"
	Position [3]float64        `json:"position"`
	Rotation [4]float64        `json:"rotation"`
	Scale    [3]float64        `json:"scale"`
	Config   map[string]any    `json:"config"`    // entity-specific config
}

// DestroyPayload removes an entity from Unreal.
type DestroyPayload struct {
	EntityID string `json:"entity_id"`
}

// ModifyPayload changes entity properties.
type ModifyPayload struct {
	EntityID string            `json:"entity_id"`
	Properties map[string]any  `json:"properties"`
}

// ──────────────────────────────────────────────────────────────
// Client interface
// ──────────────────────────────────────────────────────────────

// Client is the interface for the Unreal bridge client.
type Client interface {
	// Connect establishes the WebSocket connection.
	Connect(ctx context.Context, url string) error

	// Disconnect closes the connection.
	Disconnect(ctx context.Context) error

	// Send sends a message to Unreal.
	Send(ctx context.Context, msg Message) error

	// Receive returns the next message from Unreal.
	Receive(ctx context.Context) (Message, error)

	// SendFrame sends a camera frame to Cosca (for testing).
	SendFrame(ctx context.Context, frame FramePayload) error

	// SendAction sends an action command to Unreal.
	SendAction(ctx context.Context, action ActionPayload) error

	// OnMessage registers a handler for incoming messages.
	OnMessage(msgType MessageType, handler func(Message))

	// IsConnected returns true if the connection is active.
	IsConnected() bool
}

// ──────────────────────────────────────────────────────────────
// In-memory bridge (for testing / local mode)
// ──────────────────────────────────────────────────────────────

// LocalClient is an in-memory bridge client for testing.
// It does not use WebSocket — messages are queued in memory.
type LocalClient struct {
	mu       sync.Mutex
	connected bool
	inbox    chan Message
	outbox   chan Message
	handlers map[MessageType]func(Message)
}

// NewLocalClient creates an in-memory bridge client.
func NewLocalClient(bufferSize int) *LocalClient {
	return &LocalClient{
		inbox:    make(chan Message, bufferSize),
		outbox:   make(chan Message, bufferSize),
		handlers: make(map[MessageType]func(Message)),
	}
}

func (c *LocalClient) Connect(_ context.Context, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = true
	return nil
}

func (c *LocalClient) Disconnect(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

func (c *LocalClient) Send(_ context.Context, msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.connected {
		return fmt.Errorf("not connected")
	}
	c.outbox <- msg
	return nil
}

func (c *LocalClient) Receive(_ context.Context) (Message, error) {
	msg, ok := <-c.inbox
	if !ok {
		return Message{}, fmt.Errorf("channel closed")
	}
	return msg, nil
}

func (c *LocalClient) SendFrame(_ context.Context, frame FramePayload) error {
	data, _ := json.Marshal(frame)
	return c.Send(context.Background(), Message{
		Type:      MessageFrame,
		ID:        fmt.Sprintf("frame-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Payload:   data,
	})
}

func (c *LocalClient) SendAction(_ context.Context, action ActionPayload) error {
	data, _ := json.Marshal(action)
	return c.Send(context.Background(), Message{
		Type:      MessageAction,
		ID:        fmt.Sprintf("action-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Payload:   data,
	})
}

func (c *LocalClient) OnMessage(msgType MessageType, handler func(Message)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[msgType] = handler
}

func (c *LocalClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// Inject injects a message into the inbox (for testing).
func (c *LocalClient) Inject(msg Message) {
	c.inbox <- msg
}

// ReadOutbox reads the next message from the outbox (for testing).
func (c *LocalClient) ReadOutbox() Message {
	return <-c.outbox
}
