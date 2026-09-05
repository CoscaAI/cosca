package simulation

import (
	"context"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Pipeline tests
// ──────────────────────────────────────────────────────────────

func TestPipelineConfigDefaults(t *testing.T) {
	config := DefaultPipelineConfig()

	if config.Taichi == nil {
		t.Error("Taichi config should not be nil")
	}
	if config.MuJoCo == nil {
		t.Error("MuJoCo config should not be nil")
	}
}

func TestPipelineNew(t *testing.T) {
	config := DefaultPipelineConfig()
	p := NewPipeline(config)

	if p == nil {
		t.Fatal("NewPipeline returned nil")
	}
	if p.taichi == nil {
		t.Error("taichi adapter not initialized")
	}
}

func TestPipelineNewMinimal(t *testing.T) {
	config := PipelineConfig{}
	p := NewPipeline(config)

	if p.taichi != nil {
		t.Error("taichi should be nil when not configured")
	}
	if p.mujoco != nil {
		t.Error("mujoco should be nil when not configured")
	}
}

func TestPipelineNoAdapter(t *testing.T) {
	config := PipelineConfig{}
	p := NewPipeline(config)
	ctx := context.Background()

	state := worldmodel.WorldState{Step: 0}
	_, err := p.Simulate(ctx, state, nil, 10)
	if err == nil {
		t.Error("Simulate without adapter should fail")
	}

	_, err = p.Query(ctx, worldmodel.PhysicsQuery{Type: "raycast"})
	if err == nil {
		t.Error("Query without adapter should fail")
	}
}

func TestSimulateNoAdapter(t *testing.T) {
	config := PipelineConfig{}
	p := NewPipeline(config)
	ctx := context.Background()

	state := worldmodel.WorldState{Step: 0}
	_, err := p.Simulate(ctx, state, nil, 5)
	if err == nil {
		t.Error("Simulate without adapter should fail")
	}
}

// ──────────────────────────────────────────────────────────────
// Adapter config tests
// ──────────────────────────────────────────────────────────────

func TestTaichiSimConfig(t *testing.T) {
	config := TaichiConfig{Device: "cuda", Script: "custom/sim.py"}
	adapter := NewTaichiSimAdapter(config)

	if adapter.config.Device != "cuda" {
		t.Errorf("device: got %v, want cuda", adapter.config.Device)
	}
}

func TestTaichiSimConfigDefaultScript(t *testing.T) {
	config := TaichiConfig{Device: "cpu"}
	adapter := NewTaichiSimAdapter(config)

	if adapter.config.Script == "" {
		t.Error("script should have default value")
	}
}

func TestMuJoCoConfig(t *testing.T) {
	config := MuJoCoConfig{Model: "humanoid.xml", Device: "cuda"}
	adapter := NewMuJoCoAdapter(config)

	if adapter.config.Model != "humanoid.xml" {
		t.Errorf("model: got %v, want humanoid.xml", adapter.config.Model)
	}
}

func TestMuJoCoConfigDefaultScript(t *testing.T) {
	config := MuJoCoConfig{Model: "test.xml"}
	adapter := NewMuJoCoAdapter(config)

	if adapter.config.Script == "" {
		t.Error("script should have default value")
	}
}

// ──────────────────────────────────────────────────────────────
// WorldState construction tests
// ──────────────────────────────────────────────────────────────

func TestWorldStateConstruction(t *testing.T) {
	state := worldmodel.WorldState{
		Step: 10,
		Climate: worldmodel.ClimateState{
			Temperature: 25.0,
			Wind:        worldmodel.Vec3{X: 1, Y: 0, Z: 0},
			Rain:        0.3,
		},
		Entities: []worldmodel.WorldEntity{
			{
				ID:       "player_01",
				Type:     worldmodel.EntityObject,
				Position: worldmodel.Vec3{X: 0, Y: 0, Z: 5},
				Label:    "Player",
			},
			{
				ID:       "npc_guard",
				Type:     worldmodel.EntityNPC,
				Position: worldmodel.Vec3{X: 3, Y: 0, Z: 0},
				Label:    "Guard",
			},
		},
	}

	if state.Step != 10 {
		t.Errorf("step: got %v, want 10", state.Step)
	}
	if state.Climate.Temperature != 25.0 {
		t.Errorf("temperature: got %v, want 25.0", state.Climate.Temperature)
	}
	if len(state.Entities) != 2 {
		t.Errorf("entities: got %d, want 2", len(state.Entities))
	}
}

// ──────────────────────────────────────────────────────────────
// SimulationStep tests
// ──────────────────────────────────────────────────────────────

func TestSimulationStepStructure(t *testing.T) {
	step := worldmodel.SimulationStep{
		Step: 42,
		State: worldmodel.WorldState{
			Step: 42,
			Entities: []worldmodel.WorldEntity{
				{ID: "e1", Position: worldmodel.Vec3{X: 1, Y: 2, Z: 3}},
			},
		},
		Events: []worldmodel.WorldEvent{
			{Type: "collision", EntityID: "e1", Timestamp: time.Now()},
		},
	}

	if step.Step != 42 {
		t.Errorf("step: got %v, want 42", step.Step)
	}
	if len(step.State.Entities) != 1 {
		t.Errorf("entities: got %d, want 1", len(step.State.Entities))
	}
	if len(step.Events) != 1 {
		t.Errorf("events: got %d, want 1", len(step.Events))
	}
}

// ──────────────────────────────────────────────────────────────
// PhysicsQuery tests
// ──────────────────────────────────────────────────────────────

func TestPhysicsQueryTypes(t *testing.T) {
	queries := []worldmodel.PhysicsQuery{
		{Type: "raycast", Origin: worldmodel.Vec3{}, Direction: worldmodel.Vec3{Z: -1}, MaxDist: 100},
		{Type: "overlap", Origin: worldmodel.Vec3{}, Radius: 2.0},
		{Type: "distance", Origin: worldmodel.Vec3{}, Direction: worldmodel.Vec3{X: 1}},
	}

	for _, q := range queries {
		if q.Type == "" {
			t.Error("query type should not be empty")
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Event injection tests
// ──────────────────────────────────────────────────────────────

func TestWorldEventTypes(t *testing.T) {
	events := []worldmodel.WorldEvent{
		{Type: "spawn", EntityID: "npc_01", Position: worldmodel.Vec3{X: 5, Y: 0, Z: 0}},
		{Type: "destroy", EntityID: "wall_01"},
		{Type: "modify", EntityID: "door_01", Metadata: map[string]string{"state": "open"}},
		{Type: "weather_change", Metadata: map[string]string{"rain": "0.8"}},
	}

	for _, ev := range events {
		if ev.Type == "" {
			t.Error("event type should not be empty")
		}
		if ev.Timestamp.IsZero() {
			// Timestamp is optional, but let's check it compiles
		}
	}
}
