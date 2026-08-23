// Package bridge — Controller for the Cosca <-> Unreal nervous system.
//
// The Controller wraps a Client and exposes high-level operations that map
// directly to the Cosca Living World contract: spawn/move/destroy entities and
// synchronize WorldState with what Unreal actually reports back.
package bridge

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Entity definition (Cosca WorldEntity <-> Unreal Actor)
// ──────────────────────────────────────────────────────────────

// EntitySpec describes a world entity to spawn in Unreal.
type EntitySpec struct {
	ID     string    `json:"id"`
	Type   string    `json:"type"`      // "npc", "object", "cube", "tree", ...
	Pos    [3]float64 `json:"position"`
	Rot    [4]float64 `json:"rotation"` // quaternion [w,x,y,z]
	Scale  [3]float64 `json:"scale"`
	Tags   []string  `json:"tags,omitempty"`       // GameplayTags vocabulary
	Config map[string]string `json:"config,omitempty"`
	AssetHash string  `json:"asset_hash,omitempty"` // Blender/GLB sha256
}

// StateSnapshot is the world state reported back by Unreal.
type StateSnapshot struct {
	Entities []EntityState `json:"entities"`
	Step     int64         `json:"step"`
}

// EntityState is the real position/state of an entity after physics.
type EntityState struct {
	ID       string    `json:"id"`
	Pos      [3]float64 `json:"position"`
	Reachable bool     `json:"reachable"`
	Collision bool     `json:"collision"`
}

// ──────────────────────────────────────────────────────────────
// Controller
// ──────────────────────────────────────────────────────────────

// Controller orchestrates the Bridge. It keeps a local registry of entity
// specs and reconciles against incoming StateSnapshots from Unreal.
type Controller struct {
	mu        sync.Mutex
	client    Client
	entities  map[string]EntitySpec
	state     StateSnapshot
	onFrame   func(frame FramePayload)
	onState   func(state StateSnapshot)
	connected bool

	// pending acks by message ID
	acks map[string]chan Message
}

// NewController creates a Bridge Controller around a Client.
func NewController(client Client) *Controller {
	return &Controller{
		client:   client,
		entities: make(map[string]EntitySpec),
		acks:     make(map[string]chan Message),
	}
}

// SetFrameHandler registers the handler for incoming camera frames.
func (c *Controller) SetFrameHandler(fn func(frame FramePayload)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onFrame = fn
}

// SetStateHandler registers the handler for incoming state syncs.
func (c *Controller) SetStateHandler(fn func(state StateSnapshot)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onState = fn
}

// Connect connects to the Unreal server and wires up message handlers.
func (c *Controller) Connect(ctx context.Context, url string) error {
	if err := c.client.Connect(ctx, url); err != nil {
		return err
	}
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	// Wire up inbound message types.
	c.client.OnMessage(MessageFrame, func(m Message) {
		var frame FramePayload
		if err := json.Unmarshal(m.Payload, &frame); err == nil {
			if c.onFrame != nil {
				c.onFrame(frame)
			}
		}
	})
	c.client.OnMessage(MessageStateSync, func(m Message) {
		var snap StateSnapshot
		if err := json.Unmarshal(m.Payload, &snap); err == nil {
			c.mu.Lock()
			c.state = snap
			c.mu.Unlock()
			if c.onState != nil {
				c.onState(snap)
			}
		}
	})
	return nil
}

// Disconnect closes the connection.
func (c *Controller) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	return c.client.Disconnect(ctx)
}

// IsConnected reports whether the bridge is connected.
func (c *Controller) IsConnected() bool {
	return c.client.IsConnected()
}

// ──────────────────────────────────────────────────────────────
// Commands (Cosca -> Unreal)
// ──────────────────────────────────────────────────────────────

// Spawn sends a spawn command to Unreal and registers the entity locally.
func (c *Controller) Spawn(ctx context.Context, spec EntitySpec) error {
	c.mu.Lock()
	c.entities[spec.ID] = spec
	c.mu.Unlock()

	data, _ := json.Marshal(spec)
	return c.client.Send(ctx, Message{
		Type:      MessageSpawn,
		Timestamp: time.Now(),
		Payload:   data,
	})
}

// Move sends a move_to command to Unreal.
func (c *Controller) Move(ctx context.Context, entityID string, target [3]float64) error {
	payload := ActionPayload{
		EntityID: entityID,
		Action:   "move_to",
		Params: map[string]any{
			"target": target,
		},
	}
	return c.client.SendAction(ctx, payload)
}

// Destroy sends a destroy command to Unreal and removes the entity locally.
func (c *Controller) Destroy(ctx context.Context, entityID string) error {
	c.mu.Lock()
	delete(c.entities, entityID)
	c.mu.Unlock()

	data, _ := json.Marshal(DestroyPayload{EntityID: entityID})
	return c.client.Send(ctx, Message{
		Type:      MessageDestroy,
		Timestamp: time.Now(),
		Payload:   data,
	})
}

// ──────────────────────────────────────────────────────────────
// State reconciliation
// ──────────────────────────────────────────────────────────────

// GetState returns the latest state snapshot reported by Unreal.
func (c *Controller) GetState() StateSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// GetEntities returns the locally known entity specs.
func (c *Controller) GetEntities() map[string]EntitySpec {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]EntitySpec, len(c.entities))
	for k, v := range c.entities {
		out[k] = v
	}
	return out
}

// SyncState forces Unreal to send a full state snapshot (via ping/request).
func (c *Controller) SyncState(ctx context.Context) error {
	return c.client.Send(ctx, Message{
		Type:      MessagePing,
		Timestamp: time.Now(),
		Payload:   json.RawMessage(`{"request":"state"}`),
	})
}
