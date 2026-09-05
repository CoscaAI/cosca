// Package spatial implements the spatial understanding pipeline for the Living World.
//
// Pipeline flow:
//
//	FRAME (PNG/JPEG bytes) + IMU (optional)
//	  → Localize (SLAM: "onde estou?")
//	  → Map (point cloud: "o que está ao redor?")
//	  → Reconstruct (3D mesh: "como é o mundo?")
//	  → SpatialReasoning ("o que está perto/longe/atras?")
//	  → SpatialObservation (pose + map + relations)
package spatial

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Pipeline configuration
// ──────────────────────────────────────────────────────────────

// PipelineConfig configures the spatial pipeline.
type PipelineConfig struct {
	SLAM              *SLAMConfig          `json:"slam,omitempty"`
	Reconstruct       *ReconstructConfig   `json:"reconstruct,omitempty"`
	Reasoning         *ReasoningConfig     `json:"reasoning,omitempty"`
	MaxPointCloudSize int                  `json:"max_point_cloud_size"`
	MinConfidence     float64              `json:"min_confidence"`
	UpdateRate        float64              `json:"update_rate"`
}

// DefaultPipelineConfig returns sensible defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		SLAM:              &SLAMConfig{Backend: "orb_slam3", Device: "cpu"},
		Reconstruct:       &ReconstructConfig{Backend: "instant_ngp", Device: "cpu"},
		Reasoning:         &ReasoningConfig{Backend: "spatial_vlm", Device: "cpu"},
		MaxPointCloudSize: 100000,
		MinConfidence:     0.3,
		UpdateRate:        10.0,
	}
}

// ──────────────────────────────────────────────────────────────
// Pipeline
// ──────────────────────────────────────────────────────────────

// Pipeline orchestrates spatial adapters.
type Pipeline struct {
	config PipelineConfig
	slam   *SLAMAdapter
	recon  *ReconstructAdapter
	reason *ReasoningAdapter
}

// NewPipeline creates a spatial pipeline.
func NewPipeline(config PipelineConfig) *Pipeline {
	p := &Pipeline{config: config}
	if config.SLAM != nil {
		p.slam = NewSLAMAdapter(*config.SLAM)
	}
	if config.Reconstruct != nil {
		p.recon = NewReconstructAdapter(*config.Reconstruct)
	}
	if config.Reasoning != nil {
		p.reason = NewReasoningAdapter(*config.Reasoning)
	}
	return p
}

// SpatialResult is the result of spatial processing.
type SpatialResult struct {
	Pose       worldmodel.Pose6DoF         `json:"pose"`
	PointCloud worldmodel.PointCloud        `json:"point_cloud"`
	Mesh       *worldmodel.Mesh             `json:"mesh,omitempty"`
	Relations  []worldmodel.SpatialRelation `json:"relations"`
	Entities   []worldmodel.WorldEntity     `json:"entities"`
	Latency    time.Duration                `json:"latency"`
}

// Process processes a single frame through the spatial pipeline.
func (p *Pipeline) Process(ctx context.Context, frame []byte, imu []float64) (*SpatialResult, error) {
	start := time.Now()
	result := &SpatialResult{}

	// Step 1: Localize (SLAM)
	if p.slam != nil {
		pose, err := p.slam.Localize(ctx, frame, imu)
		if err != nil {
			return nil, fmt.Errorf("slam localize: %w", err)
		}
		result.Pose = pose
	}

	// Step 2: Map (point cloud)
	if p.slam != nil {
		pc, err := p.slam.Map(ctx)
		if err != nil {
			return nil, fmt.Errorf("slam map: %w", err)
		}
		if len(pc.Points) > p.config.MaxPointCloudSize {
			pc.Points = pc.Points[:p.config.MaxPointCloudSize]
			if len(pc.Colors) > p.config.MaxPointCloudSize {
				pc.Colors = pc.Colors[:p.config.MaxPointCloudSize]
			}
		}
		result.PointCloud = pc
	}

	// Step 3: Reconstruct (mesh) — optional
	if p.recon != nil && len(result.PointCloud.Points) > 100 {
		mesh, err := p.recon.Reconstruct(ctx, result.PointCloud)
		if err == nil {
			result.Mesh = &mesh
		}
	}

	// Step 4: Spatial reasoning — optional
	if p.reason != nil && len(result.PointCloud.Points) > 0 {
		relations, entities, err := p.reason.Reason(ctx, result.Pose, result.PointCloud)
		if err == nil {
			result.Relations = relations
			result.Entities = entities
		}
	}

	// Step 5: Fallback relation computation from point cloud
	if len(result.Relations) == 0 && len(result.PointCloud.Points) > 0 {
		result.Relations = computeSpatialRelations(result.PointCloud, result.Pose)
	}

	result.Latency = time.Since(start)
	return result, nil
}

// ProcessBatch processes multiple frames for reconstruction.
func (p *Pipeline) ProcessBatch(ctx context.Context, frames [][]byte, imus [][]float64) (*SpatialResult, error) {
	start := time.Now()
	result := &SpatialResult{}

	for i, frame := range frames {
		var imu []float64
		if imus != nil && i < len(imus) {
			imu = imus[i]
		}
		if p.slam != nil {
			pose, err := p.slam.Localize(ctx, frame, imu)
			if err == nil {
				result.Pose = pose
			}
		}
	}

	if p.slam != nil {
		pc, err := p.slam.Map(ctx)
		if err == nil {
			result.PointCloud = pc
		}
	}

	if p.recon != nil && len(frames) > 1 {
		mesh, err := p.recon.ReconstructFromFrames(ctx, frames)
		if err == nil {
			result.Mesh = &mesh
		}
	}

	result.Latency = time.Since(start)
	return result, nil
}

// ──────────────────────────────────────────────────────────────
// Spatial relation computation
// ──────────────────────────────────────────────────────────────

func computeSpatialRelations(pc worldmodel.PointCloud, agentPose worldmodel.Pose6DoF) []worldmodel.SpatialRelation {
	if len(pc.Points) == 0 {
		return nil
	}

	clusters := clusterPoints(pc.Points, 2.0)
	var relations []worldmodel.SpatialRelation
	agentPos := agentPose.Position

	for i, cluster := range clusters {
		if len(cluster) < 3 {
			continue
		}

		center := worldmodel.Vec3{}
		for _, pt := range cluster {
			center = center.Add(pt)
		}
		center = center.Scale(1.0 / float64(len(cluster)))

		dist := agentPos.DistanceTo(center)
		dir := center.Sub(agentPos).Normalize()

		var rel string
		if math.Abs(dir.X) > math.Abs(dir.Z) {
			if dir.X > 0 {
				rel = "right_of_agent"
			} else {
				rel = "left_of_agent"
			}
		} else {
			if dir.Z > 0 {
				rel = "behind_agent"
			} else {
				rel = "in_front_of_agent"
			}
		}

		conf := 0.6
		if dist < 1.0 {
			conf = 0.8
			rel = "next_to_agent"
		}

		relations = append(relations, worldmodel.SpatialRelation{
			Subject:    fmt.Sprintf("region_%d", i),
			Object:     "agent",
			Relation:   rel,
			Distance:   dist,
			Confidence: conf,
		})
	}

	return relations
}

func clusterPoints(points []worldmodel.Vec3, cellSize float64) [][]worldmodel.Vec3 {
	type cellKey struct{ x, y, z int }
	cells := make(map[cellKey][]worldmodel.Vec3)

	for _, p := range points {
		key := cellKey{
			x: int(math.Floor(p.X / cellSize)),
			y: int(math.Floor(p.Y / cellSize)),
			z: int(math.Floor(p.Z / cellSize)),
		}
		cells[key] = append(cells[key], p)
	}

	clusters := make([][]worldmodel.Vec3, 0, len(cells))
	for _, pts := range cells {
		clusters = append(clusters, pts)
	}
	return clusters
}

// ──────────────────────────────────────────────────────────────
// Subprocess helper
// ──────────────────────────────────────────────────────────────

type subprocessRequest struct {
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload"`
}

type subprocessResponse struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

func runSubprocess(ctx context.Context, scriptPath string, req subprocessRequest, timeout time.Duration) (json.RawMessage, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	reqBytes, _ := json.Marshal(req)
	cmd := exec.CommandContext(ctx, "python3", scriptPath)
	cmd.Stdin = bytes.NewReader(reqBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("subprocess failed: %w, stderr: %s", err, stderr.String())
	}

	var resp subprocessResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if !resp.OK {
		return nil, fmt.Errorf("subprocess error: %s", resp.Error)
	}
	return resp.Data, nil
}
