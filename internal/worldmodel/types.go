// Package worldmodel defines the fundamental types for the Cosca Living World.
//
// These types represent the spatial, temporal, and semantic structure of the
// world as perceived and acted upon by the Cosca agent. They are the shared
// vocabulary between perception (Vision, Audio), spatial understanding (SLAM),
// world state (Climate, Simulation), and decision-making (Deliberation).
//
// Design principles:
//   - Pure Go, zero external dependencies (stdlib only)
//   - Immutable value types where possible
//   - JSON-serializable for WebSocket/REST transport
//   - Compatible with both real-time (Unreal) and batch (simulation) pipelines
package worldmodel

import (
	"crypto/sha256"
	"fmt"
	"math"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Geometric primitives
// ──────────────────────────────────────────────────────────────

// Vec3 represents a 3D vector (position, direction, scale).
type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Add returns v + w.
func (v Vec3) Add(w Vec3) Vec3 { return Vec3{v.X + w.X, v.Y + w.Y, v.Z + w.Z} }

// Sub returns v - w.
func (v Vec3) Sub(w Vec3) Vec3 { return Vec3{v.X - w.X, v.Y - w.Y, v.Z - w.Z} }

// Scale returns v * s.
func (v Vec3) Scale(s float64) Vec3 { return Vec3{v.X * s, v.Y * s, v.Z * s} }

// Dot returns the dot product v · w.
func (v Vec3) Dot(w Vec3) float64 { return v.X*w.X + v.Y*w.Y + v.Z*w.Z }

// Cross returns the cross product v × w.
func (v Vec3) Cross(w Vec3) Vec3 {
	return Vec3{
		X: v.Y*w.Z - v.Z*w.Y,
		Y: v.Z*w.X - v.X*w.Z,
		Z: v.X*w.Y - v.Y*w.X,
	}
}

// Length returns the Euclidean norm |v|.
func (v Vec3) Length() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
}

// Normalize returns v / |v|. Returns zero vector if |v| == 0.
func (v Vec3) Normalize() Vec3 {
	l := v.Length()
	if l < 1e-10 {
		return Vec3{}
	}
	return v.Scale(1.0 / l)
}

// DistanceTo returns the Euclidean distance between v and w.
func (v Vec3) DistanceTo(w Vec3) float64 {
	return v.Sub(w).Length()
}

// Quat represents a quaternion (rotation).
type Quat struct {
	W float64 `json:"w"` // scalar part
	X float64 `json:"x"` // i
	Y float64 `json:"y"` // j
	Z float64 `json:"z"` // k
}

// IdentityQuat returns the identity quaternion (no rotation).
func IdentityQuat() Quat { return Quat{W: 1} }

// AABB represents an axis-aligned bounding box.
type AABB struct {
	Min Vec3 `json:"min"` // minimum corner
	Max Vec3 `json:"max"` // maximum corner
}

// Center returns the center of the AABB.
func (a AABB) Center() Vec3 {
	return Vec3{
		X: (a.Min.X + a.Max.X) / 2,
		Y: (a.Min.Y + a.Max.Y) / 2,
		Z: (a.Min.Z + a.Max.Z) / 2,
	}
}

// Size returns the dimensions of the AABB.
func (a AABB) Size() Vec3 {
	return a.Max.Sub(a.Min)
}

// Contains checks if a point is inside the AABB.
func (a AABB) Contains(p Vec3) bool {
	return p.X >= a.Min.X && p.X <= a.Max.X &&
		p.Y >= a.Min.Y && p.Y <= a.Max.Y &&
		p.Z >= a.Min.Z && p.Z <= a.Max.Z
}

// Pose6DoF represents a 6-degrees-of-freedom pose (position + orientation).
type Pose6DoF struct {
	Position Vec3 `json:"position"`
	Rotation Quat `json:"rotation"`
}

// ──────────────────────────────────────────────────────────────
// World entities and relations
// ──────────────────────────────────────────────────────────────

// EntityType classifies world entities.
type EntityType string

const (
	EntityObject     EntityType = "object"
	EntityNPC        EntityType = "npc"
	EntityStructure  EntityType = "structure"
	EntityTerrain    EntityType = "terrain"
	EntityVehicle    EntityType = "vehicle"
	EntityParticle   EntityType = "particle"
	EntityLight      EntityType = "light"
	EntityAudio      EntityType = "audio_source"
	EntityUnknown    EntityType = "unknown"
)

// WorldEntity represents a single entity in the world.
type WorldEntity struct {
	ID          string            `json:"id"`
	Type        EntityType        `json:"type"`
	Position    Vec3              `json:"position"`
	Rotation    Quat              `json:"rotation"`
	Scale       Vec3              `json:"scale"`
	BoundingBox AABB              `json:"bounding_box"`
	Label       string            `json:"label"`        // human-readable: "red car", "oak tree"
	Confidence  float64           `json:"confidence"`   // 0.0 - 1.0
	Depth       float64           `json:"depth"`        // distance to agent (meters)
	Embedding   []float32         `json:"embedding,omitempty"` // feature vector (CLIP/DINOv2)
	Metadata    map[string]string `json:"metadata,omitempty"`
	LastSeen    time.Time         `json:"last_seen"`
	Persistent  bool              `json:"persistent"`   // survives between frames
}

// Hash returns a content hash of the entity (for deduplication).
func (e WorldEntity) Hash() string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s:%.3f:%.3f:%.3f", e.Type, e.Label, e.Position.X, e.Position.Y, e.Position.Z)))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// SpatialRelation describes a spatial relationship between two entities.
type SpatialRelation struct {
	Subject    string  `json:"subject"`    // entity ID
	Object     string  `json:"object"`     // entity ID
	Relation   string  `json:"relation"`   // "left_of", "behind", "near", "on_top_of", "inside"
	Distance   float64 `json:"distance"`   // distance between them (meters)
	Confidence float64 `json:"confidence"` // 0.0 - 1.0
}

// ──────────────────────────────────────────────────────────────
// Audio events
// ──────────────────────────────────────────────────────────────

// AudioEvent represents a detected sound in the world.
type AudioEvent struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`        // "speech", "music", "environment", "alert"
	Source      Vec3      `json:"source"`      // estimated position
	Content     string    `json:"content"`     // transcribed text (if speech)
	Language    string    `json:"language"`    // detected language
	Emotion     string    `json:"emotion"`     // "neutral", "happy", "angry"
	Volume      float64   `json:"volume"`      // 0.0 - 1.0
	Confidence  float64   `json:"confidence"`  // 0.0 - 1.0
	Timestamp   time.Time `json:"timestamp"`
}

// SpatialAudio represents spatial audio analysis results.
type SpatialAudio struct {
	Direction   Vec3    `json:"direction"`    // direction vector
	Distance    float64 `json:"distance"`     // estimated distance (meters)
	Intensity   float64 `json:"intensity"`    // 0.0 - 1.0
	Spatializer string  `json:"spatializer"`  // "hrtf", "ambisonics", "binaural"
}



// ──────────────────────────────────────────────────────────────
// Climate and environment
// ──────────────────────────────────────────────────────────────

// Season represents a world season.
type Season string

const (
	Spring Season = "spring"
	Summer Season = "summer"
	Autumn Season = "autumn"
	Winter Season = "winter"
)

// ClimateState represents the environmental conditions.
type ClimateState struct {
	Temperature float64 `json:"temperature"` // Celsius
	Wind        Vec3    `json:"wind"`        // direction + speed (m/s)
	Rain        float64 `json:"rain"`        // 0.0 (clear) - 1.0 (heavy rain)
	Snow        float64 `json:"snow"`        // 0.0 (none) - 1.0 (heavy snow)
	Fog         float64 `json:"fog"`         // 0.0 (clear) - 1.0 (dense fog)
	TimeOfDay   float64 `json:"time_of_day"` // 0.0 - 24.0 (hours)
	Season      Season  `json:"season"`
}

// ──────────────────────────────────────────────────────────────
// Observations and world state
// ──────────────────────────────────────────────────────────────

// SpatialObservation is a complete observation of the world at a point in time.
type SpatialObservation struct {
	Timestamp   time.Time         `json:"timestamp"`
	AgentPose   Pose6DoF          `json:"agent_pose"`    // where the agent is
	Entities    []WorldEntity     `json:"entities"`      // what the agent sees
	Relations   []SpatialRelation `json:"relations"`     // how entities relate
	AudioEvents []AudioEvent      `json:"audio_events"`  // what the agent hears
	DepthMap    [][]float32       `json:"depth_map,omitempty"` // depth per pixel
	PointCloud  []Vec3            `json:"point_cloud,omitempty"` // 3D points (from SLAM)
}

// EntityCount returns the number of entities in the observation.
func (o SpatialObservation) EntityCount() int {
	return len(o.Entities)
}

// FindEntity finds an entity by ID. Returns nil if not found.
func (o SpatialObservation) FindEntity(id string) *WorldEntity {
	for i := range o.Entities {
		if o.Entities[i].ID == id {
			return &o.Entities[i]
		}
	}
	return nil
}

// FilterByType returns entities of a given type.
func (o SpatialObservation) FilterByType(t EntityType) []WorldEntity {
	var result []WorldEntity
	for _, e := range o.Entities {
		if e.Type == t {
			result = append(result, e)
		}
	}
	return result
}

// FilterByConfidence returns entities with confidence >= threshold.
func (o SpatialObservation) FilterByConfidence(threshold float64) []WorldEntity {
	var result []WorldEntity
	for _, e := range o.Entities {
		if e.Confidence >= threshold {
			result = append(result, e)
		}
	}
	return result
}

// WorldState is the complete state of the world.
type WorldState struct {
	Entities    []WorldEntity     `json:"entities"`
	Relations   []SpatialRelation `json:"relations"`
	Climate     ClimateState      `json:"climate"`
	Timestamp   time.Time         `json:"timestamp"`
	Step        int64             `json:"step"`        // simulation step
}

// EntityCount returns the total number of entities.
func (s WorldState) EntityCount() int {
	return len(s.Entities)
}

// FindEntity finds an entity by ID.
func (s WorldState) FindEntity(id string) *WorldEntity {
	for i := range s.Entities {
		if s.Entities[i].ID == id {
			return &s.Entities[i]
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────
// VFX configuration
// ──────────────────────────────────────────────────────────────

// VFXType classifies visual effects.
type VFXType string

const (
	VFXParticles VFXType = "particles"
	VFXFluids    VFXType = "fluids"
	VFXCloth     VFXType = "cloth"
	VFXFire      VFXType = "fire"
	VFXSmoke     VFXType = "smoke"
	VFXDust      VFXType = "dust"
	VFXWind      VFXType = "wind"
)

// VFXConfig configures a visual effect simulation.
type VFXConfig struct {
	Type        VFXType          `json:"type"`
	Resolution  int              `json:"resolution"`   // particles per axis
	Seed        int64            `json:"seed"`         // deterministic seed
	Duration    float64          `json:"duration"`     // seconds
	FrameRate   float64          `json:"frame_rate"`   // FPS
	Origin      Vec3             `json:"origin"`       // emission point
	Params      map[string]float64 `json:"params"`     // type-specific params
}

// ──────────────────────────────────────────────────────────────
// Physics
// ──────────────────────────────────────────────────────────────

// PhysicalAction represents an action that affects the physics world.
type PhysicalAction struct {
	Type       string  `json:"type"`       // "apply_force", "apply_impulse", "set_velocity"
	TargetID   string  `json:"target_id"`  // entity ID
	Force      Vec3    `json:"force"`      // Newtons
	Impulse    Vec3    `json:"impulse"`    // N·s
	Velocity   Vec3    `json:"velocity"`   // m/s
	Duration   float64 `json:"duration"`   // seconds
}

// PhysicsQuery queries the physics state.
type PhysicsQuery struct {
	Type     string  `json:"type"`     // "overlap", "raycast", "distance"
	Origin   Vec3    `json:"origin"`
	Direction Vec3   `json:"direction"`
	MaxDist  float64 `json:"max_dist"`
	Radius   float64 `json:"radius"`
}

// PhysicsResult is the result of a physics query.
type PhysicsResult struct {
	Hit       bool      `json:"hit"`
	Point     Vec3      `json:"point"`
	Normal    Vec3      `json:"normal"`
	Distance  float64   `json:"distance"`
	EntityID  string    `json:"entity_id,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Simulation
// ──────────────────────────────────────────────────────────────

// WorldEvent is an event injected into the simulation.
type WorldEvent struct {
	Type      string            `json:"type"`      // "spawn", "destroy", "modify", "weather_change"
	EntityID  string            `json:"entity_id,omitempty"`
	Position  Vec3              `json:"position,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// SimulationStep is one step of a simulation.
type SimulationStep struct {
	Step      int64     `json:"step"`
	State     WorldState `json:"state"`
	Events    []WorldEvent `json:"events,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────
