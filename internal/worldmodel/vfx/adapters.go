package vfx

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Taichi Adapter
// ──────────────────────────────────────────────────────────────

// TaichiConfig configures the Taichi adapter.
type TaichiConfig struct {
	Device string `json:"device"` // "cpu", "cuda", "metal", "vulkan"
	Script string `json:"script"`
}

// TaichiAdapter wraps Taichi for procedural VFX simulation.
type TaichiAdapter struct {
	config TaichiConfig
}

// NewTaichiAdapter creates a Taichi adapter.
func NewTaichiAdapter(config TaichiConfig) *TaichiAdapter {
	if config.Script == "" {
		config.Script = "adapters/vfx/taichi.py"
	}
	return &TaichiAdapter{config: config}
}

// Simulate runs a complete VFX simulation and returns all frames.
func (a *TaichiAdapter) Simulate(ctx context.Context, config worldmodel.VFXConfig) ([]worldmodel.Frame, error) {
	payload, _ := json.Marshal(map[string]any{
		"type":       string(config.Type),
		"resolution": config.Resolution,
		"seed":       config.Seed,
		"duration":   config.Duration,
		"frame_rate": config.FrameRate,
		"origin":     [3]float64{config.Origin.X, config.Origin.Y, config.Origin.Z},
		"params":     config.Params,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "simulate",
		Payload: payload,
	}, 120*time.Second)
	if err != nil {
		return nil, fmt.Errorf("taichi simulate: %w", err)
	}

	var result struct {
		Frames []struct {
			Step     int64              `json:"step"`
			Width    int                `json:"width"`
			Height   int                `json:"height"`
			Data     []byte             `json:"data"`
			Metadata map[string]float64 `json:"metadata"`
		} `json:"frames"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	frames := make([]worldmodel.Frame, len(result.Frames))
	for i, f := range result.Frames {
		frames[i] = worldmodel.Frame{
			Step:     f.Step,
			Data:     f.Data,
			Width:    f.Width,
			Height:   f.Height,
			Metadata: f.Metadata,
		}
	}

	return frames, nil
}

// Step advances the simulation by one frame.
func (a *TaichiAdapter) Step(ctx context.Context) (*worldmodel.Frame, error) {
	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "step",
		Payload: []byte(`{}`),
	}, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("taichi step: %w", err)
	}

	var result struct {
		Step     int64              `json:"step"`
		Width    int                `json:"width"`
		Height   int                `json:"height"`
		Data     []byte             `json:"data"`
		Metadata map[string]float64 `json:"metadata"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &worldmodel.Frame{
		Step:     result.Step,
		Data:     result.Data,
		Width:    result.Width,
		Height:   result.Height,
		Metadata: result.Metadata,
	}, nil
}

// Reset resets the simulation to initial state.
func (a *TaichiAdapter) Reset(ctx context.Context) error {
	_, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "reset",
		Payload: []byte(`{}`),
	}, 10*time.Second)
	return err
}
