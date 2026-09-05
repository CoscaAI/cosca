// Package simulation implements the world simulation pipeline for the Living World.
//
// Pipeline flow:
//
//	WorldState + WorldEvents
//	  → Step (Taichi: rigid body, fluid, cloth, soft body)
//	  → PhysicsQuery (raycast, overlap, distance)
//	  → Updated WorldState
//
// The simulation pipeline keeps the world physically consistent:
// objects fall, fluids flow, cloth drapes, soft bodies deform.
package simulation

import (
	"context"
	"fmt"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Pipeline configuration
// ──────────────────────────────────────────────────────────────

// PipelineConfig configures the simulation pipeline.
type PipelineConfig struct {
	Taichi *TaichiConfig `json:"taichi,omitempty"`
	MuJoCo *MuJoCoConfig `json:"mujoco,omitempty"`
}

// DefaultPipelineConfig returns sensible defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Taichi: &TaichiConfig{Device: "cpu", Script: "adapters/simulation/taichi.py"},
		MuJoCo: &MuJoCoConfig{Model: "", Device: "cpu", Script: "adapters/simulation/mujoco.py"},
	}
}

// ──────────────────────────────────────────────────────────────
// Pipeline
// ──────────────────────────────────────────────────────────────

// Pipeline orchestrates simulation adapters.
type Pipeline struct {
	config PipelineConfig
	taichi *TaichiSimAdapter
	mujoco *MuJoCoAdapter
}

// NewPipeline creates a simulation pipeline.
func NewPipeline(config PipelineConfig) *Pipeline {
	p := &Pipeline{config: config}
	if config.Taichi != nil {
		p.taichi = NewTaichiSimAdapter(*config.Taichi)
	}
	if config.MuJoCo != nil && config.MuJoCo.Model != "" {
		p.mujoco = NewMuJoCoAdapter(*config.MuJoCo)
	}
	return p
}

// Simulate runs the simulation for N steps.
func (p *Pipeline) Simulate(ctx context.Context, state worldmodel.WorldState, events []worldmodel.WorldEvent, steps int) ([]worldmodel.SimulationStep, error) {
	if p.taichi == nil && p.mujoco == nil {
		return nil, fmt.Errorf("no simulation adapter configured")
	}

	var allSteps []worldmodel.SimulationStep
	current := state

	for i := 0; i < steps; i++ {
		var step worldmodel.SimulationStep
		var err error

		if p.taichi != nil {
			step, err = p.taichi.Step(ctx, current, events)
		} else if p.mujoco != nil {
			step, err = p.mujoco.Step(ctx, current, events)
		}

		if err != nil {
			return nil, fmt.Errorf("simulation step %d: %w", i, err)
		}

		allSteps = append(allSteps, step)
		current = step.State
		events = nil // events only apply to first step
	}

	return allSteps, nil
}

// Query runs a physics query against the current state.
func (p *Pipeline) Query(ctx context.Context, query worldmodel.PhysicsQuery) (*worldmodel.PhysicsResult, error) {
	if p.taichi != nil {
		return p.taichi.Query(ctx, query)
	}
	if p.mujoco != nil {
		return p.mujoco.Query(ctx, query)
	}
	return nil, fmt.Errorf("no simulation adapter configured")
}
