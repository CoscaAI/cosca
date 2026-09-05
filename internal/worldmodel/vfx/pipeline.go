// Package vfx implements the procedural visual effects pipeline for the Living World.
//
// Pipeline flow:
//
//	VFXConfig (type, resolution, seed, origin, params)
//	  → Simulate (Taichi: particles, fluids, cloth, fire, smoke)
//	  → Step (advance simulation)
//	  → Frames (sequence of visual effect frames)
//
// The VFX pipeline generates procedural effects that make the world feel alive:
// dust, wind, fire, smoke, fluid, cloth, particles.
package vfx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Pipeline configuration
// ──────────────────────────────────────────────────────────────

// PipelineConfig configures the VFX pipeline.
type PipelineConfig struct {
	Taichi    *TaichiConfig `json:"taichi,omitempty"`
	MaxFrames int           `json:"max_frames"` // max frames per simulation
	MaxFPS    float64       `json:"max_fps"`    // max frame rate
}

// DefaultPipelineConfig returns sensible defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Taichi:    &TaichiConfig{Device: "cpu", Script: "adapters/vfx/taichi.py"},
		MaxFrames: 300,
		MaxFPS:    30.0,
	}
}

// ──────────────────────────────────────────────────────────────
// Pipeline
// ──────────────────────────────────────────────────────────────

// Pipeline orchestrates VFX adapters.
type Pipeline struct {
	config PipelineConfig
	taichi *TaichiAdapter
}

// NewPipeline creates a VFX pipeline.
func NewPipeline(config PipelineConfig) *Pipeline {
	p := &Pipeline{config: config}
	if config.Taichi != nil {
		p.taichi = NewTaichiAdapter(*config.Taichi)
	}
	return p
}

// VFXResult is the result of a VFX simulation.
type VFXResult struct {
	Type    worldmodel.VFXType `json:"type"`
	Frames  []worldmodel.Frame `json:"frames"`
	Count   int                `json:"count"`
	Latency time.Duration      `json:"latency"`
}

// Simulate runs a complete VFX simulation and returns all frames.
func (p *Pipeline) Simulate(ctx context.Context, config worldmodel.VFXConfig) (*VFXResult, error) {
	start := time.Now()

	if p.taichi == nil {
		return nil, fmt.Errorf("no VFX adapter configured")
	}

	frames, err := p.taichi.Simulate(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("vfx simulate: %w", err)
	}

	// Truncate if too many
	if len(frames) > p.config.MaxFrames {
		frames = frames[:p.config.MaxFrames]
	}

	return &VFXResult{
		Type:    config.Type,
		Frames:  frames,
		Count:   len(frames),
		Latency: time.Since(start),
	}, nil
}

// Step advances an existing simulation by one frame.
func (p *Pipeline) Step(ctx context.Context) (*worldmodel.Frame, error) {
	if p.taichi == nil {
		return nil, fmt.Errorf("no VFX adapter configured")
	}
	return p.taichi.Step(ctx)
}

// Reset resets the simulation to initial state.
func (p *Pipeline) Reset(ctx context.Context) error {
	if p.taichi == nil {
		return fmt.Errorf("no VFX adapter configured")
	}
	return p.taichi.Reset(ctx)
}

// ──────────────────────────────────────────────────────────────
// Presets — common VFX configurations
// ──────────────────────────────────────────────────────────────

// DustParticles returns a config for dust/particle effects.
func DustParticles(origin worldmodel.Vec3) worldmodel.VFXConfig {
	return worldmodel.VFXConfig{
		Type:       worldmodel.VFXDust,
		Resolution: 64,
		Seed:       42,
		Duration:   5.0,
		FrameRate:  30.0,
		Origin:     origin,
		Params: map[string]float64{
			"count":    1000,
			"spread":   2.0,
			"gravity":  -0.5,
			"lifetime": 3.0,
			"size":     0.02,
		},
	}
}

// WindEffect returns a config for wind effects.
func WindEffect(direction worldmodel.Vec3) worldmodel.VFXConfig {
	return worldmodel.VFXConfig{
		Type:       worldmodel.VFXWind,
		Resolution: 32,
		Seed:       123,
		Duration:   10.0,
		FrameRate:  30.0,
		Origin:     worldmodel.Vec3{},
		Params: map[string]float64{
			"speed":    direction.Length(),
			"turbulence": 0.3,
			"scale":    5.0,
		},
	}
}

// FireEffect returns a config for fire effects.
func FireEffect(origin worldmodel.Vec3) worldmodel.VFXConfig {
	return worldmodel.VFXConfig{
		Type:       worldmodel.VFXFire,
		Resolution: 64,
		Seed:       456,
		Duration:   8.0,
		FrameRate:  30.0,
		Origin:     origin,
		Params: map[string]float64{
			"intensity": 1.0,
			"spread":    1.0,
			"rise_speed": 2.0,
			"lifetime":  2.0,
		},
	}
}

// SmokeEffect returns a config for smoke effects.
func SmokeEffect(origin worldmodel.Vec3) worldmodel.VFXConfig {
	return worldmodel.VFXConfig{
		Type:       worldmodel.VFXSmoke,
		Resolution: 48,
		Seed:       789,
		Duration:   10.0,
		FrameRate:  30.0,
		Origin:     origin,
		Params: map[string]float64{
			"density":  0.5,
			"spread":   2.0,
			"rise_speed": 1.0,
			"lifetime": 4.0,
		},
	}
}

// FluidEffect returns a config for fluid simulation.
func FluidEffect(origin worldmodel.Vec3) worldmodel.VFXConfig {
	return worldmodel.VFXConfig{
		Type:       worldmodel.VFXFluids,
		Resolution: 64,
		Seed:       101,
		Duration:   5.0,
		FrameRate:  30.0,
		Origin:     origin,
		Params: map[string]float64{
			"viscosity": 0.1,
			"density":   1.0,
			"gravity":   -9.8,
			"particle_count": 2000,
		},
	}
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
		timeout = 60 * time.Second
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
