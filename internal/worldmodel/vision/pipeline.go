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
	"context"
	"fmt"
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

	// ModelsDir is the directory the vision .onnx models are loaded from
	// (sovereignty: never hard-code a fixed path). When empty, resolution falls
	// back to $COSCA_MODELS then ~/.cosca/models/vision/. This is propagated to
	// every adapter so a single pipeline config drives all of them.
	ModelsDir string `json:"models_dir,omitempty"`
}

// DefaultPipelineConfig returns sensible defaults.
//
// GroundingDINO uses a zero-shot UI/screenshot text-prompt by default and a
// lowered confidence threshold (GroundingDINO-tiny is a small detector whose
// zero-shot scores on non-COCO imagery run low). MinConfidence matches the
// detection threshold so real detections are surfaced rather than silently
// dropped by the pipeline filter.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Clip:          &ClipConfig{Model: "ViT-B/32", Device: "cpu"},
		SAM:           &SAMConfig{Model: "sam2_hiera_large", Device: "cpu"},
		Grounding:     &GroundingConfig{Model: "groundingdino_swint", Device: "cpu", Prompt: defaultUIGroundingPrompt(), Threshold: defaultGroundingThreshold},
		Depth:         &DepthConfig{Model: "depth_anything_v2_vitl", Device: "cpu"},
		MinConfidence: defaultGroundingThreshold,
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

// NewPipeline creates a vision pipeline from config. The pipeline-level
// ModelsDir is propagated to each adapter that does not declare its own, so a
// single config drives all model resolution (sovereignty: no fixed paths).
func NewPipeline(config PipelineConfig) *Pipeline {
	p := &Pipeline{config: config}
	if config.Clip != nil {
		cfg := *config.Clip
		if cfg.ModelsDir == "" {
			cfg.ModelsDir = config.ModelsDir
		}
		p.clip = NewClipAdapter(cfg)
	}
	if config.SAM != nil {
		cfg := *config.SAM
		if cfg.ModelsDir == "" {
			cfg.ModelsDir = config.ModelsDir
		}
		p.sam = NewSAMAdapter(cfg)
	}
	if config.Grounding != nil {
		cfg := *config.Grounding
		if cfg.ModelsDir == "" {
			cfg.ModelsDir = config.ModelsDir
		}
		p.grounding = NewGroundingAdapter(cfg)
	}
	if config.Depth != nil {
		cfg := *config.Depth
		if cfg.ModelsDir == "" {
			cfg.ModelsDir = config.ModelsDir
		}
		p.depth = NewDepthAdapter(cfg)
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
	// Warnings carries non-fatal degradation notes (e.g. a model that is not
	// downloaded yet). The pipeline never breaks because of these; it informs.
	Warnings []string `json:"warnings,omitempty"`

	// Per-model timings (ms) for the last frame, exposed so the Perception Loop
	// can surface CLIP / GroundingDINO / Depth / SAM latency in its Metrics and
	// let the professor benchmark each model in native Go without Python. They
	// are additive telemetry — absent (0) when a model is not in the pipeline
	// or degrades.
	ClipMS      int64 `json:"clip_ms,omitempty"`
	GroundingMS int64 `json:"grounding_ms,omitempty"`
	DepthMS     int64 `json:"depth_ms,omitempty"`
	SAMMS       int64 `json:"sam_ms,omitempty"`
}

// addWarn records a non-fatal degradation note without aborting the pipeline.
func (o *Observation) addWarn(format string, args ...any) {
	o.Warnings = append(o.Warnings, fmt.Sprintf(format, args...))
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
		gStart := time.Now()
		var err error
		detections, err = p.grounding.Detect(ctx, frame)
		obs.GroundingMS = time.Since(gStart).Milliseconds()
		if err != nil {
			obs.addWarn("grounding detect degraded: %v", err)
			detections = nil
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
			sStart := time.Now()
			masks, err := p.sam.Segment(ctx, frame, det.Label)
			obs.SAMMS += time.Since(sStart).Milliseconds()
			if err != nil {
				obs.addWarn("sam segment degraded: %v", err)
			} else if len(masks) > 0 {
				// Attach mask info to metadata
				if entity.Metadata == nil {
					entity.Metadata = make(map[string]string)
				}
				entity.Metadata["mask_area"] = fmt.Sprintf("%d", masks[0].Area)
			}
		}

		// Step 5: Embed (CLIP) — get feature vector
		if p.clip != nil {
			cStart := time.Now()
			embedding, err := p.clip.Embed(ctx, frame)
			obs.ClipMS += time.Since(cStart).Milliseconds()
			if err != nil {
				obs.addWarn("clip embed degraded: %v", err)
			} else {
				entity.Embedding = embedding
			}
		}

		obs.Entities = append(obs.Entities, entity)
	}

	// Step 6: Depth map
	if p.depth != nil {
		dStart := time.Now()
		depthMap, err := p.depth.EstimateDepth(ctx, frame)
		obs.DepthMS = time.Since(dStart).Milliseconds()
		if err != nil {
			obs.addWarn("depth estimate degraded: %v", err)
		} else {
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
			gStart := time.Now()
			detections, detectErr = p.grounding.Detect(ctx, frame)
			obs.GroundingMS = time.Since(gStart).Milliseconds()
		}
	}()

	go func() {
		defer wg.Done()
		if p.depth != nil {
			dStart := time.Now()
			depthMap, depthErr = p.depth.EstimateDepth(ctx, frame)
			obs.DepthMS = time.Since(dStart).Milliseconds()
		}
	}()

	wg.Wait()

	if detectErr != nil {
		obs.addWarn("grounding detect degraded: %v", detectErr)
		detections = nil
	}
	if depthErr != nil {
		obs.addWarn("depth estimate degraded: %v", depthErr)
		depthMap = nil
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
				sStart := time.Now()
				masks, err := p.sam.Segment(ctx, frame, det.Label)
				obs.SAMMS += time.Since(sStart).Milliseconds()
				if err != nil {
					mu.Lock()
					obs.addWarn("sam segment degraded: %v", err)
					mu.Unlock()
				} else if len(masks) > 0 {
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
				cStart := time.Now()
				embedding, err := p.clip.Embed(ctx, frame)
				obs.ClipMS += time.Since(cStart).Milliseconds()
				if err != nil {
					mu.Lock()
					obs.addWarn("clip embed degraded: %v", err)
					mu.Unlock()
				} else {
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
