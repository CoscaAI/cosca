// Package vision implements the perception pipeline for the Living World.
//
// Pipeline flow:
//
//	FRAME (PNG/JPEG bytes)
//	  → Detect (GroundingDINO: "o que tem aqui?")
//	  → Segment (SAM2: "qual a forma?")
//	  → Classify (CLIP: "isso é o quê?")
//	  → Embed (CLIP: feature vector for memory)
//	  → Depth (Depth Anything V2: "quão longe?")
//	  → StructuredObservation (WorldEntity[])
//
// Each adapter communicates with Python subprocesses via JSON.
// The pipeline orchestrates them into a single observation.
package vision

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Pipeline configuration
// ──────────────────────────────────────────────────────────────

// PipelineConfig configures the vision pipeline.
type PipelineConfig struct {
	// Adapters to enable (nil = use defaults)
	Clip       *ClipConfig       `json:"clip,omitempty"`
	SAM        *SAMConfig        `json:"sam,omitempty"`
	Grounding  *GroundingConfig  `json:"grounding,omitempty"`
	Depth      *DepthConfig      `json:"depth,omitempty"`

	// Pipeline settings
	MinConfidence float64 `json:"min_confidence"` // minimum detection confidence
	MaxEntities   int     `json:"max_entities"`   // maximum entities per frame
	Parallel      bool    `json:"parallel"`       // run adapters in parallel
}

// DefaultPipelineConfig returns sensible defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Clip:          &ClipConfig{Model: "ViT-B/32", Device: "cpu"},
		SAM:           &SAMConfig{Model: "sam2_hiera_large", Device: "cpu"},
		Grounding:     &GroundingConfig{Model: "groundingdino_swint", Device: "cpu"},
		Depth:         &DepthConfig{Model: "depth_anything_v2_vitl", Device: "cpu"},
		MinConfidence: 0.3,
		MaxEntities:   50,
		Parallel:      true,
	}
}

// ──────────────────────────────────────────────────────────────
// Pipeline
// ──────────────────────────────────────────────────────────────

// Pipeline orchestrates vision adapters into a structured observation.
type Pipeline struct {
	config    PipelineConfig
	clip      *ClipAdapter
	sam       *SAMAdapter
	grounding *GroundingAdapter
	depth     *DepthAdapter
}

// NewPipeline creates a vision pipeline from config.
func NewPipeline(config PipelineConfig) *Pipeline {
	p := &Pipeline{config: config}
	if config.Clip != nil {
		p.clip = NewClipAdapter(*config.Clip)
	}
	if config.SAM != nil {
		p.sam = NewSAMAdapter(*config.SAM)
	}
	if config.Grounding != nil {
		p.grounding = NewGroundingAdapter(*config.Grounding)
	}
	if config.Depth != nil {
		p.depth = NewDepthAdapter(*config.Depth)
	}
	return p
}

// Observation is the result of processing a single frame.
type Observation struct {
	Entities   []worldmodel.WorldEntity     `json:"entities"`
	Relations  []worldmodel.SpatialRelation `json:"relations"`
	DepthMap   [][]float32                  `json:"depth_map,omitempty"`
	PointCloud []worldmodel.Vec3            `json:"point_cloud,omitempty"`
	Latency    time.Duration                `json:"latency"`
}

// Process processes a frame through the entire vision pipeline.
func (p *Pipeline) Process(ctx context.Context, frame []byte, agentPose worldmodel.Pose6DoF) (*Observation, error) {
	start := time.Now()

	if p.config.Parallel {
		return p.processParallel(ctx, frame, agentPose, start)
	}
	return p.processSequential(ctx, frame, agentPose, start)
}

// processSequential runs adapters one by one.
func (p *Pipeline) processSequential(ctx context.Context, frame []byte, agentPose worldmodel.Pose6DoF, start time.Time) (*Observation, error) {
	obs := &Observation{}

	// Step 1: Detect objects (GroundingDINO)
	var detections []worldmodel.Detection
	if p.grounding != nil {
		var err error
		detections, err = p.grounding.Detect(ctx, frame)
		if err != nil {
			return nil, fmt.Errorf("grounding detect: %w", err)
		}
	}

	// Step 2: Classify each detection (CLIP)
	if p.clip != nil && len(detections) > 0 {
		candidates := make([]string, len(detections))
		for i, d := range detections {
			candidates[i] = d.Label
		}
		// CLIP classification is done per-entity below
		_ = candidates
	}

	// Step 3: Build entities
	for i, det := range detections {
		if det.Confidence < p.config.MinConfidence {
			continue
		}
		if len(obs.Entities) >= p.config.MaxEntities {
			break
		}

		entity := worldmodel.WorldEntity{
			ID:         fmt.Sprintf("entity-%d-%d", time.Now().UnixNano(), i),
			Type:       worldmodel.EntityObject,
			BoundingBox: det.BoundingBox,
			Label:      det.Label,
			Confidence: det.Confidence,
			LastSeen:   time.Now(),
			Position:   det.BoundingBox.Center(), // approximate
		}

		// Step 4: Segment (SAM) — get mask for this entity
		if p.sam != nil {
			masks, err := p.sam.Segment(ctx, frame, det.Label)
			if err == nil && len(masks) > 0 {
				// Attach mask info to metadata
				if entity.Metadata == nil {
					entity.Metadata = make(map[string]string)
				}
				entity.Metadata["mask_area"] = fmt.Sprintf("%d", masks[0].Area)
			}
		}

		// Step 5: Embed (CLIP) — get feature vector
		if p.clip != nil {
			embedding, err := p.clip.Embed(ctx, frame)
			if err == nil {
				entity.Embedding = embedding
			}
		}

		obs.Entities = append(obs.Entities, entity)
	}

	// Step 6: Depth map
	if p.depth != nil {
		depthMap, err := p.depth.EstimateDepth(ctx, frame)
		if err == nil {
			obs.DepthMap = depthMap
			// Assign depth to entities based on bounding box center
			for i := range obs.Entities {
				center := obs.Entities[i].BoundingBox.Center()
				px := int(center.X)
				py := int(center.Y)
				if px >= 0 && px < len(depthMap) && py >= 0 && py < len(depthMap[px]) {
					obs.Entities[i].Depth = float64(depthMap[px][py])
				}
			}
		}
	}

	// Step 7: Compute relations
	obs.Relations = computeRelations(obs.Entities)

	obs.Latency = time.Since(start)
	return obs, nil
}

// processParallel runs adapters concurrently.
func (p *Pipeline) processParallel(ctx context.Context, frame []byte, agentPose worldmodel.Pose6DoF, start time.Time) (*Observation, error) {
	obs := &Observation{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	var detectErr, depthErr error
	var detections []worldmodel.Detection
	var depthMap [][]float32

	// Run detect + depth in parallel
	wg.Add(2)

	go func() {
		defer wg.Done()
		if p.grounding != nil {
			detections, detectErr = p.grounding.Detect(ctx, frame)
		}
	}()

	go func() {
		defer wg.Done()
		if p.depth != nil {
			depthMap, depthErr = p.depth.EstimateDepth(ctx, frame)
		}
	}()

	wg.Wait()

	if detectErr != nil {
		return nil, fmt.Errorf("grounding detect: %w", detectErr)
	}
	if depthErr != nil {
		return nil, fmt.Errorf("depth estimate: %w", depthErr)
	}

	// Build entities from detections
	for i, det := range detections {
		if det.Confidence < p.config.MinConfidence {
			continue
		}
		if len(obs.Entities) >= p.config.MaxEntities {
			break
		}

		entity := worldmodel.WorldEntity{
			ID:          fmt.Sprintf("entity-%d-%d", time.Now().UnixNano(), i),
			Type:        worldmodel.EntityObject,
			BoundingBox: det.BoundingBox,
			Label:       det.Label,
			Confidence:  det.Confidence,
			LastSeen:    time.Now(),
			Position:    det.BoundingBox.Center(),
		}

		// Segment + embed in parallel per entity
		var segWg sync.WaitGroup
		segWg.Add(2)

		go func() {
			defer segWg.Done()
			if p.sam != nil {
				masks, err := p.sam.Segment(ctx, frame, det.Label)
				if err == nil && len(masks) > 0 {
					mu.Lock()
					if entity.Metadata == nil {
						entity.Metadata = make(map[string]string)
					}
					entity.Metadata["mask_area"] = fmt.Sprintf("%d", masks[0].Area)
					mu.Unlock()
				}
			}
		}()

		go func() {
			defer segWg.Done()
			if p.clip != nil {
				embedding, err := p.clip.Embed(ctx, frame)
				if err == nil {
					mu.Lock()
					entity.Embedding = embedding
					mu.Unlock()
				}
			}
		}()

		segWg.Wait()

		// Assign depth
		if depthMap != nil {
			center := entity.BoundingBox.Center()
			px := int(center.X)
			py := int(center.Y)
			if px >= 0 && px < len(depthMap) && py >= 0 && py < len(depthMap[px]) {
				entity.Depth = float64(depthMap[px][py])
			}
		}

		mu.Lock()
		obs.Entities = append(obs.Entities, entity)
		mu.Unlock()
	}

	obs.DepthMap = depthMap
	obs.Relations = computeRelations(obs.Entities)
	obs.Latency = time.Since(start)
	return obs, nil
}

// ──────────────────────────────────────────────────────────────
// Relation computation
// ──────────────────────────────────────────────────────────────

// computeRelations infers spatial relations between entities.
func computeRelations(entities []worldmodel.WorldEntity) []worldmodel.SpatialRelation {
	var relations []worldmodel.SpatialRelation

	for i := range entities {
		for j := i + 1; j < len(entities); j++ {
			a := entities[i]
			b := entities[j]

			dist := a.Position.DistanceTo(b.Position)
			if dist > 20.0 {
				continue // too far apart
			}

			// Determine relation based on relative position
			dx := b.Position.X - a.Position.X
			dz := b.Position.Z - a.Position.Z

			var rel string
			if abs(dx) > abs(dz) {
				if dx > 0 {
					rel = "right_of"
				} else {
					rel = "left_of"
				}
			} else {
				if dz > 0 {
					rel = "behind"
				} else {
					rel = "in_front_of"
				}
			}

			conf := 0.7
			if dist < 2.0 {
				conf = 0.9
				if dist < 0.5 {
					rel = "next_to"
				}
			}

			relations = append(relations, worldmodel.SpatialRelation{
				Subject:    a.ID,
				Object:     b.ID,
				Relation:   rel,
				Distance:   dist,
				Confidence: conf,
			})
		}
	}

	return relations
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ──────────────────────────────────────────────────────────────
// Subprocess helper
// ──────────────────────────────────────────────────────────────

// subprocessRequest is sent to Python adapters via stdin.
type subprocessRequest struct {
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload"`
}

// subprocessResponse is received from Python adapters via stdout.
type subprocessResponse struct {
	OK      bool            `json:"ok"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// runSubprocess sends a request to a Python script and reads the response.
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
