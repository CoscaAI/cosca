// Package destruction implements the destruction and fracture pipeline for the Living World.
//
// Pipeline flow:
//
//	PhysicalAction (impact/force) + EntityState (mesh, material)
//	  → Fracture (Voronoi/Markov/Cutoff: how does it break?)
//	  → Fragments (pieces with physics properties)
//	  → Debris (dust, particles, sound)
//	  → DestructionResult (fragments + debris + structural impact)
//
// The destruction system makes the world physically believable:
// when you shoot a wall, it doesn't just disappear — it shatters realistically.
package destruction

import (
	"context"
	"fmt"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Pipeline configuration
// ──────────────────────────────────────────────────────────────

// PipelineConfig configures the destruction pipeline.
type PipelineConfig struct {
	Fracture *FractureConfig `json:"fracture,omitempty"`
	MaxFrags int             `json:"max_fragments"` // max fragments per fracture
}

// DefaultPipelineConfig returns sensible defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Fracture: &FractureConfig{
			Method:   "voronoi",
			MaxFrags: 20,
			Seed:     0,
			Script:   "adapters/destruction/fracture.py",
		},
		MaxFrags: 20,
	}
}

// ──────────────────────────────────────────────────────────────
// Pipeline
// ──────────────────────────────────────────────────────────────

// Pipeline orchestrates destruction adapters.
type Pipeline struct {
	config  PipelineConfig
	fracture *FractureAdapter
}

// NewPipeline creates a destruction pipeline.
func NewPipeline(config PipelineConfig) *Pipeline {
	p := &Pipeline{config: config}
	if config.Fracture != nil {
		p.fracture = NewFractureAdapter(*config.Fracture)
	}
	return p
}

// DestructionResult is the result of a destruction simulation.
type DestructionResult struct {
	Fragments []worldmodel.Fragment  `json:"fragments"`
	Debris    []worldmodel.Debris    `json:"debris"`
	StructuralImpact float64         `json:"structural_impact"` // 0.0 - 1.0
	Latency   time.Duration          `json:"latency"`
}

// Destroy processes a destruction event.
func (p *Pipeline) Destroy(ctx context.Context, action worldmodel.PhysicalAction, mesh worldmodel.Mesh, material worldmodel.Material) (*DestructionResult, error) {
	start := time.Now()

	if p.fracture == nil {
		return nil, fmt.Errorf("no fracture adapter configured")
	}

	fragments, err := p.fracture.Fracture(ctx, mesh, material, action)
	if err != nil {
		return nil, fmt.Errorf("fracture: %w", err)
	}

	// Truncate fragments
	if len(fragments) > p.config.MaxFrags {
		fragments = fragments[:p.config.MaxFrags]
	}

	// Generate debris
	debris := p.generateDebris(fragments, action)

	// Compute structural impact
	impact := p.computeStructuralImpact(fragments, mesh)

	return &DestructionResult{
		Fragments:        fragments,
		Debris:           debris,
		StructuralImpact: impact,
		Latency:          time.Since(start),
	}, nil
}

// generateDebris creates particle/sound debris from fragments.
func (p *Pipeline) generateDebris(fragments []worldmodel.Fragment, action worldmodel.PhysicalAction) []worldmodel.Debris {
	var debris []worldmodel.Debris

	for i, frag := range fragments {
		// Dust particle
		debris = append(debris, worldmodel.Debris{
			Type:      "dust",
			Position:  frag.Position,
			Velocity:  frag.Velocity.Scale(0.3),
			Lifetime:  2.0,
			Size:      frag.BoundingSphereRadius * 0.1,
			Density:   0.5,
			Timestamp: time.Now(),
		})

		// Sound event
		if i == 0 {
			debris = append(debris, worldmodel.Debris{
				Type:      "sound",
				Position:  action.Impulse,
				Velocity:  worldmodel.Vec3{},
				Lifetime:  0.5,
				Size:      action.Impulse.Length(),
				Density:   0.0,
				Timestamp: time.Now(),
			})
		}
	}

	return debris
}

// computeStructuralImpact estimates how much the destruction weakens the structure.
func (p *Pipeline) computeStructuralImpact(fragments []worldmodel.Fragment, mesh worldmodel.Mesh) float64 {
	if len(mesh.Vertices) == 0 {
		return 0.0
	}

	totalVolume := 0.0
	for _, frag := range fragments {
		totalVolume += frag.Volume
	}

	meshVolume := estimateMeshVolume(mesh)
	if meshVolume <= 0 {
		return 0.0
	}

	ratio := totalVolume / meshVolume
	if ratio > 1.0 {
		ratio = 1.0
	}
	return ratio
}

// estimateMeshVolume estimates mesh volume using bounding box approximation.
func estimateMeshVolume(mesh worldmodel.Mesh) float64 {
	if len(mesh.Vertices) == 0 {
		return 0.0
	}

	min, max := mesh.Vertices[0], mesh.Vertices[0]
	for _, v := range mesh.Vertices {
		min = worldmodel.Vec3{
			X: mathMin(min.X, v.X),
			Y: mathMin(min.Y, v.Y),
			Z: mathMin(min.Z, v.Z),
		}
		max = worldmodel.Vec3{
			X: mathMax(max.X, v.X),
			Y: mathMax(max.Y, v.Y),
			Z: mathMax(max.Z, v.Z),
		}
	}

	size := max.Sub(min)
	return size.X * size.Y * size.Z
}

func mathMin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func mathMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
