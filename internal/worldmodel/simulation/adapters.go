package simulation

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

// ──────────────────────────────────────────────────────────────
// Taichi Simulation Adapter
// ──────────────────────────────────────────────────────────────

// TaichiConfig configures the Taichi simulation adapter.
type TaichiConfig struct {
	Device string `json:"device"` // "cpu", "cuda", "metal", "vulkan"
	Script string `json:"script"`
}

// TaichiSimAdapter wraps Taichi for physics simulation.
type TaichiSimAdapter struct {
	config TaichiConfig
}

// NewTaichiSimAdapter creates a Taichi simulation adapter.
func NewTaichiSimAdapter(config TaichiConfig) *TaichiSimAdapter {
	if config.Script == "" {
		config.Script = "adapters/simulation/taichi.py"
	}
	return &TaichiSimAdapter{config: config}
}

// Step runs one simulation step.
func (a *TaichiSimAdapter) Step(ctx context.Context, state worldmodel.WorldState, events []worldmodel.WorldEvent) (worldmodel.SimulationStep, error) {
	entities := make([]map[string]any, len(state.Entities))
	for i, e := range state.Entities {
		entities[i] = map[string]any{
			"id":       e.ID,
			"type":     string(e.Type),
			"position": [3]float64{e.Position.X, e.Position.Y, e.Position.Z},
			"rotation": [4]float64{e.Rotation.W, e.Rotation.X, e.Rotation.Y, e.Rotation.Z},
			"label":    e.Label,
		}
	}

	eventsRaw := make([]map[string]any, len(events))
	for i, ev := range events {
		eventsRaw[i] = map[string]any{
			"type":      ev.Type,
			"entity_id": ev.EntityID,
			"position":  [3]float64{ev.Position.X, ev.Position.Y, ev.Position.Z},
			"timestamp": ev.Timestamp,
		}
	}

	payload, _ := json.Marshal(map[string]any{
		"step":     state.Step,
		"entities": entities,
		"climate": map[string]any{
			"temperature": state.Climate.Temperature,
			"wind":        [3]float64{state.Climate.Wind.X, state.Climate.Wind.Y, state.Climate.Wind.Z},
			"rain":        state.Climate.Rain,
		},
		"events": eventsRaw,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "step",
		Payload: payload,
	}, 30*time.Second)
	if err != nil {
		return worldmodel.SimulationStep{}, fmt.Errorf("taichi step: %w", err)
	}

	var result struct {
		Step     int64 `json:"step"`
		Entities []struct {
			ID       string    `json:"id"`
			Type     string    `json:"type"`
			Position [3]float64 `json:"position"`
			Rotation [4]float64 `json:"rotation"`
			Label    string    `json:"label"`
		} `json:"entities"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return worldmodel.SimulationStep{}, err
	}

	newState := worldmodel.WorldState{
		Step:     result.Step,
		Climate:  state.Climate,
		Entities: make([]worldmodel.WorldEntity, len(result.Entities)),
	}

	for i, e := range result.Entities {
		newState.Entities[i] = worldmodel.WorldEntity{
			ID:       e.ID,
			Type:     worldmodel.EntityType(e.Type),
			Position: worldmodel.Vec3{X: e.Position[0], Y: e.Position[1], Z: e.Position[2]},
			Rotation: worldmodel.Quat{W: e.Rotation[0], X: e.Rotation[1], Y: e.Rotation[2], Z: e.Rotation[3]},
			Label:    e.Label,
			LastSeen: time.Now(),
		}
	}

	return worldmodel.SimulationStep{
		Step:  result.Step,
		State: newState,
	}, nil
}

// Query runs a physics query.
func (a *TaichiSimAdapter) Query(ctx context.Context, query worldmodel.PhysicsQuery) (*worldmodel.PhysicsResult, error) {
	payload, _ := json.Marshal(map[string]any{
		"type":      query.Type,
		"origin":    [3]float64{query.Origin.X, query.Origin.Y, query.Origin.Z},
		"direction": [3]float64{query.Direction.X, query.Direction.Y, query.Direction.Z},
		"max_dist":  query.MaxDist,
		"radius":    query.Radius,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "query",
		Payload: payload,
	}, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("taichi query: %w", err)
	}

	var result struct {
		Hit      bool    `json:"hit"`
		Point    [3]float64 `json:"point"`
		Normal   [3]float64 `json:"normal"`
		Distance float64 `json:"distance"`
		EntityID string  `json:"entity_id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &worldmodel.PhysicsResult{
		Hit:      result.Hit,
		Point:    worldmodel.Vec3{X: result.Point[0], Y: result.Point[1], Z: result.Point[2]},
		Normal:   worldmodel.Vec3{X: result.Normal[0], Y: result.Normal[1], Z: result.Normal[2]},
		Distance: result.Distance,
		EntityID: result.EntityID,
	}, nil
}

// ──────────────────────────────────────────────────────────────
// MuJoCo Adapter
// ──────────────────────────────────────────────────────────────

// MuJoCoConfig configures the MuJoCo adapter.
type MuJoCoConfig struct {
	Model  string `json:"model"`  // MJCF model path
	Device string `json:"device"` // "cpu", "cuda"
	Script string `json:"script"`
}

// MuJoCoAdapter wraps MuJoCo for high-fidelity physics.
type MuJoCoAdapter struct {
	config MuJoCoConfig
}

// NewMuJoCoAdapter creates a MuJoCo adapter.
func NewMuJoCoAdapter(config MuJoCoConfig) *MuJoCoAdapter {
	if config.Script == "" {
		config.Script = "adapters/simulation/mujoco.py"
	}
	return &MuJoCoAdapter{config: config}
}

// Step runs one MuJoCo simulation step.
func (a *MuJoCoAdapter) Step(ctx context.Context, state worldmodel.WorldState, events []worldmodel.WorldEvent) (worldmodel.SimulationStep, error) {
	payload, _ := json.Marshal(map[string]any{
		"model":  a.config.Model,
		"step":   state.Step,
		"events": events,
	})

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "step",
		Payload: payload,
	}, 30*time.Second)
	if err != nil {
		return worldmodel.SimulationStep{}, fmt.Errorf("mujoco step: %w", err)
	}

	var result struct {
		Step     int64              `json:"step"`
		State    worldmodel.WorldState `json:"state"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return worldmodel.SimulationStep{}, err
	}

	return worldmodel.SimulationStep{
		Step:  result.Step,
		State: result.State,
	}, nil
}

// Query runs a MuJoCo physics query.
func (a *MuJoCoAdapter) Query(ctx context.Context, query worldmodel.PhysicsQuery) (*worldmodel.PhysicsResult, error) {
	payload, _ := json.Marshal(query)

	data, err := runSubprocess(ctx, a.config.Script, subprocessRequest{
		Command: "query",
		Payload: payload,
	}, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("mujoco query: %w", err)
	}

	var result worldmodel.PhysicsResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
