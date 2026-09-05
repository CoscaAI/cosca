package worldmodel

import (
	"context"
	"time"
)

// ──────────────────────────────────────────────────────────────
// VisionProvider — percepção visual do agente
// ──────────────────────────────────────────────────────────────

// Detection represents a detected object in a frame.
type Detection struct {
	BoundingBox AABB    `json:"bounding_box"`
	Label       string  `json:"label"`
	Confidence  float64 `json:"confidence"`
	ClassID     int     `json:"class_id"`
}

// Mask represents a segmentation mask for an object.
type Mask struct {
	BoundingBox AABB    `json:"bounding_box"`
	Pixels      []byte  `json:"pixels"`       // RLE-encoded or raw mask
	Area        int     `json:"area"`          // pixel count
	Label       string  `json:"label"`
}

// VisionProvider is the interface for visual perception.
//
// Implementations:
//   - CLIP: zero-shot classification + embeddings
//   - SAM2: prompt-based segmentation
//   - GroundingDINO: text-prompted object detection
//   - Depth Anything V2: monocular depth estimation
type VisionProvider interface {
	// Detect finds objects in a frame.
	Detect(ctx context.Context, frame []byte) ([]Detection, error)

	// Segment finds objects matching a text prompt and returns masks.
	Segment(ctx context.Context, frame []byte, prompt string) ([]Mask, error)

	// Classify returns the most likely class from candidates.
	Classify(ctx context.Context, frame []byte, candidates []string) (string, float64, error)

	// Embed returns a dense feature vector for the frame.
	Embed(ctx context.Context, frame []byte) ([]float32, error)

	// Depth returns a per-pixel depth map (meters).
	Depth(ctx context.Context, frame []byte) ([][]float32, error)
}

// ──────────────────────────────────────────────────────────────
// SpatialProvider — entendimento espacial
// ──────────────────────────────────────────────────────────────

// PointCloud is a set of 3D points.
type PointCloud struct {
	Points    []Vec3    `json:"points"`
	Colors    []Vec3    `json:"colors,omitempty"`    // RGB per point
	Normals   []Vec3    `json:"normals,omitempty"`   // surface normal per point
	Timestamp time.Time `json:"timestamp"`
}

// Mesh is a 3D mesh (vertices + faces + textures).
type Mesh struct {
	Vertices  []Vec3     `json:"vertices"`
	Faces     [][3]int   `json:"faces"`      // triangle indices
	UVs       [][2]float64 `json:"uvs,omitempty"`
	TextureURL string    `json:"texture_url,omitempty"`
}

// SpatialProvider is the interface for spatial understanding.
//
// Implementations:
//   - ORB-SLAM3: visual SLAM (localization + mapping)
//   - Instant-NGP: neural 3D reconstruction
//   - Spatial Reasoning VLM: relational understanding
type SpatialProvider interface {
	// Localize returns the agent's 6DoF pose from a frame.
	Localize(ctx context.Context, frame []byte, imu []float64) (Pose6DoF, error)

	// Map returns the current 3D point cloud map.
	Map(ctx context.Context) (PointCloud, error)

	// Reconstruct builds a 3D mesh from multiple frames.
	Reconstruct(ctx context.Context, frames [][]byte) (Mesh, error)

	// SpatialReasoning answers spatial questions about the observation.
	SpatialReasoning(ctx context.Context, question string, observation SpatialObservation) (string, error)
}

// ──────────────────────────────────────────────────────────────
// AudioProvider — percepção e ação de áudio
// ──────────────────────────────────────────────────────────────

// AudioProvider is the interface for audio perception and action.
//
// Implementations:
//   - whisper.cpp: speech-to-text
//   - Coqui TTS: text-to-speech + voice cloning
//   - AudioCraft: music + ambient sound generation
//   - SenseVoice: audio classification + emotion detection
type AudioProvider interface {
	// Transcribe converts speech audio to text.
	Transcribe(ctx context.Context, audio []byte) (string, error)

	// Synthesize converts text to speech audio.
	Synthesize(ctx context.Context, text string, voice string) ([]byte, error)

	// ClassifySound identifies the type of a sound.
	ClassifySound(ctx context.Context, audio []byte) (string, float64, error)

	// GenerateAmbient creates ambient audio from a description.
	GenerateAmbient(ctx context.Context, description string) ([]byte, error)
}

// ──────────────────────────────────────────────────────────────
// VFXProvider — efeitos visuais procedurais
// ──────────────────────────────────────────────────────────────

// Frame is a single frame of visual effect output.
type Frame struct {
	Step      int64              `json:"step"`
	Data      []byte             `json:"data"`      // raw pixel data or encoded
	Width     int                `json:"width"`
	Height    int                `json:"height"`
	Metadata  map[string]float64 `json:"metadata"` // particles count, fluid density, etc.
}

// VFXProvider is the interface for procedural visual effects.
//
// Implementations:
//   - Taichi: particles, fluids, cloth, fire, smoke
type VFXProvider interface {
	// Simulate runs a complete VFX simulation and returns all frames.
	Simulate(ctx context.Context, config VFXConfig) ([]Frame, error)

	// Step advances the simulation by one frame.
	Step(ctx context.Context) ([]Frame, error)

	// Reset resets the simulation to initial state.
	Reset(ctx context.Context) error
}

// ──────────────────────────────────────────────────────────────
// DestructionProvider — física e destruição
// ──────────────────────────────────────────────────────────────

// DestructionProvider is the interface for physical simulation and destruction.
//
// Implementations:
//   - MuJoCo: multi-joint dynamics, contact, ragdoll
//   - Box2D: 2D physics
//   - Jolt: lightweight 3D physics
type DestructionProvider interface {
	// Apply applies a physical action and returns the resulting world state.
	Apply(ctx context.Context, action PhysicalAction) (WorldState, error)

	// Query queries the physics state (overlap, raycast, distance).
	Query(ctx context.Context, query PhysicsQuery) (PhysicsResult, error)
}

// ──────────────────────────────────────────────────────────────
// SimulationProvider — simulação emergente
// ──────────────────────────────────────────────────────────────

// SimulationProvider is the interface for emergent world simulation.
//
// Implementations:
//   - Mesa: agent-based modeling (ecosystems, economy, crowds)
//   - Climate rules: weather, seasons, erosion
type SimulationProvider interface {
	// Step advances the simulation by n steps and returns each step.
	Step(ctx context.Context, n int) ([]SimulationStep, error)

	// GetState returns the current world state.
	GetState(ctx context.Context) (WorldState, error)

	// Inject injects a world event (spawn, destroy, weather change).
	Inject(ctx context.Context, event WorldEvent) error
}
