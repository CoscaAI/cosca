package spatial

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// SLAM Adapter (ORB-SLAM3 / OpenVSLAM)
// ──────────────────────────────────────────────────────────────

// SLAMConfig configures the SLAM adapter.
type SLAMConfig struct {
	Backend string `json:"backend"` // "orb_slam3", "open_vslam", "nice_slam"
	Device  string `json:"device"`  // "cpu", "cuda"
	Script  string `json:"script"`
}

// SLAMAdapter wraps visual SLAM for localization and mapping.
type SLAMAdapter struct {
	config SLAMConfig
}

// NewSLAMAdapter creates a SLAM adapter.
func NewSLAMAdapter(config SLAMConfig) *SLAMAdapter {
	if config.Script == "" {
		config.Script = "adapters/spatial/slam.py"
	}
	return &SLAMAdapter{config: config}
}

// Localize returns the agent's 6DoF pose from a frame.
func (a *SLAMAdapter) Localize(ctx context.Context, frame []byte, imu []float64) (worldmodel.Pose6DoF, error) {
	payload, _ := json.Marshal(map[string]any{
		"image": frame,
		"imu":   imu,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "localize",
		Payload: payload,
	}, 30*time.Second)
	if err != nil {
		return worldmodel.Pose6DoF{}, fmt.Errorf("slam localize: %w", err)
	}

	var result struct {
		Position [3]float64 `json:"position"`
		Rotation [4]float64 `json:"rotation"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return worldmodel.Pose6DoF{}, err
	}

	return worldmodel.Pose6DoF{
		Position: worldmodel.Vec3{X: result.Position[0], Y: result.Position[1], Z: result.Position[2]},
		Rotation: worldmodel.Quat{W: result.Rotation[0], X: result.Rotation[1], Y: result.Rotation[2], Z: result.Rotation[3]},
	}, nil
}

// Map returns the current 3D point cloud map.
func (a *SLAMAdapter) Map(ctx context.Context) (worldmodel.PointCloud, error) {
	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "map",
		Payload: []byte(`{}`),
	}, 10*time.Second)
	if err != nil {
		return worldmodel.PointCloud{}, fmt.Errorf("slam map: %w", err)
	}

	var result struct {
		Points    [][3]float64 `json:"points"`
		Colors    [][3]float64 `json:"colors,omitempty"`
		Timestamp time.Time    `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return worldmodel.PointCloud{}, err
	}

	pc := worldmodel.PointCloud{
		Points:    make([]worldmodel.Vec3, len(result.Points)),
		Colors:    make([]worldmodel.Vec3, len(result.Colors)),
		Timestamp: result.Timestamp,
	}
	for i, p := range result.Points {
		pc.Points[i] = worldmodel.Vec3{X: p[0], Y: p[1], Z: p[2]}
	}
	for i, c := range result.Colors {
		pc.Colors[i] = worldmodel.Vec3{X: c[0], Y: c[1], Z: c[2]}
	}

	return pc, nil
}

// ──────────────────────────────────────────────────────────────
// 3D Reconstruction Adapter (Instant-NGP / Meshroom)
// ──────────────────────────────────────────────────────────────

// ReconstructConfig configures the 3D reconstruction adapter.
type ReconstructConfig struct {
	Backend string `json:"backend"` // "instant_ngp", "meshroom", "nice_slam"
	Device  string `json:"device"`
	Script  string `json:"script"`
}

// ReconstructAdapter wraps 3D reconstruction tools.
type ReconstructAdapter struct {
	config ReconstructConfig
}

// NewReconstructAdapter creates a reconstruction adapter.
func NewReconstructAdapter(config ReconstructConfig) *ReconstructAdapter {
	if config.Script == "" {
		config.Script = "adapters/spatial/reconstruct.py"
	}
	return &ReconstructAdapter{config: config}
}

// Reconstruct builds a 3D mesh from a point cloud.
func (a *ReconstructAdapter) Reconstruct(ctx context.Context, pc worldmodel.PointCloud) (worldmodel.Mesh, error) {
	points := make([][3]float64, len(pc.Points))
	for i, p := range pc.Points {
		points[i] = [3]float64{p.X, p.Y, p.Z}
	}

	payload, _ := json.Marshal(map[string]any{
		"points": points,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "reconstruct",
		Payload: payload,
	}, 60*time.Second)
	if err != nil {
		return worldmodel.Mesh{}, fmt.Errorf("reconstruct: %w", err)
	}

	var result struct {
		Vertices  [][3]float64 `json:"vertices"`
		Faces     [][3]int     `json:"faces"`
		TextureURL string      `json:"texture_url,omitempty"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return worldmodel.Mesh{}, err
	}

	mesh := worldmodel.Mesh{
		Vertices:   make([]worldmodel.Vec3, len(result.Vertices)),
		Faces:      make([][3]int, len(result.Faces)),
		TextureURL: result.TextureURL,
	}
	for i, v := range result.Vertices {
		mesh.Vertices[i] = worldmodel.Vec3{X: v[0], Y: v[1], Z: v[2]}
	}
	copy(mesh.Faces, result.Faces)

	return mesh, nil
}

// ReconstructFromFrames builds a mesh from multiple frames.
func (a *ReconstructAdapter) ReconstructFromFrames(ctx context.Context, frames [][]byte) (worldmodel.Mesh, error) {
	payload, _ := json.Marshal(map[string]any{
		"frame_count": len(frames),
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "reconstruct_frames",
		Payload: payload,
	}, 120*time.Second)
	if err != nil {
		return worldmodel.Mesh{}, fmt.Errorf("reconstruct frames: %w", err)
	}

	var result struct {
		Vertices [][3]float64 `json:"vertices"`
		Faces    [][3]int     `json:"faces"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return worldmodel.Mesh{}, err
	}

	mesh := worldmodel.Mesh{
		Vertices: make([]worldmodel.Vec3, len(result.Vertices)),
		Faces:    make([][3]int, len(result.Faces)),
	}
	for i, v := range result.Vertices {
		mesh.Vertices[i] = worldmodel.Vec3{X: v[0], Y: v[1], Z: v[2]}
	}
	copy(mesh.Faces, result.Faces)

	return mesh, nil
}

// ──────────────────────────────────────────────────────────────
// Spatial Reasoning Adapter (VLM)
// ──────────────────────────────────────────────────────────────

// ReasoningConfig configures the spatial reasoning adapter.
type ReasoningConfig struct {
	Backend string `json:"backend"` // "spatial_vlm", "g2vlm", "3d_thinker"
	Device  string `json:"device"`
	Script  string `json:"script"`
}

// ReasoningAdapter wraps spatial reasoning VLMs.
type ReasoningAdapter struct {
	config ReasoningConfig
}

// NewReasoningAdapter creates a reasoning adapter.
func NewReasoningAdapter(config ReasoningConfig) *ReasoningAdapter {
	if config.Script == "" {
		config.Script = "adapters/spatial/reasoning.py"
	}
	return &ReasoningAdapter{config: config}
}

// Reason infers spatial relations and entities from point cloud.
func (a *ReasoningAdapter) Reason(ctx context.Context, pose worldmodel.Pose6DoF, pc worldmodel.PointCloud) ([]worldmodel.SpatialRelation, []worldmodel.WorldEntity, error) {
	points := make([][3]float64, len(pc.Points))
	for i, p := range pc.Points {
		points[i] = [3]float64{p.X, p.Y, p.Z}
	}

	payload, _ := json.Marshal(map[string]any{
		"pose": map[string]any{
			"position": [3]float64{pose.Position.X, pose.Position.Y, pose.Position.Z},
			"rotation": [4]float64{pose.Rotation.W, pose.Rotation.X, pose.Rotation.Y, pose.Rotation.Z},
		},
		"points": points,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "reason",
		Payload: payload,
	}, 30*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("spatial reasoning: %w", err)
	}

	var result struct {
		Relations []struct {
			Subject    string  `json:"subject"`
			Object     string  `json:"object"`
			Relation   string  `json:"relation"`
			Distance   float64 `json:"distance"`
			Confidence float64 `json:"confidence"`
		} `json:"relations"`
		Entities []struct {
			ID         string    `json:"id"`
			Type       string    `json:"type"`
			Position   [3]float64 `json:"position"`
			Label      string    `json:"label"`
			Confidence float64   `json:"confidence"`
		} `json:"entities"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, nil, err
	}

	relations := make([]worldmodel.SpatialRelation, len(result.Relations))
	for i, r := range result.Relations {
		relations[i] = worldmodel.SpatialRelation{
			Subject:    r.Subject,
			Object:     r.Object,
			Relation:   r.Relation,
			Distance:   r.Distance,
			Confidence: r.Confidence,
		}
	}

	entities := make([]worldmodel.WorldEntity, len(result.Entities))
	for i, e := range result.Entities {
		entities[i] = worldmodel.WorldEntity{
			ID:         e.ID,
			Type:       worldmodel.EntityType(e.Type),
			Position:   worldmodel.Vec3{X: e.Position[0], Y: e.Position[1], Z: e.Position[2]},
			Label:      e.Label,
			Confidence: e.Confidence,
			LastSeen:   time.Now(),
		}
	}

	return relations, entities, nil
}
