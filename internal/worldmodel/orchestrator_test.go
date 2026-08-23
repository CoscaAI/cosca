package worldmodel

import (
	"context"
	"testing"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Config tests
// ──────────────────────────────────────────────────────────────

func TestDefaultOrchestratorConfig(t *testing.T) {
	config := DefaultOrchestratorConfig()

	if config.MaxEntities != 100 {
		t.Errorf("MaxEntities: got %v, want 100", config.MaxEntities)
	}
	if config.UpdateRate != 10.0 {
		t.Errorf("UpdateRate: got %v, want 10.0", config.UpdateRate)
	}
	if config.Vision == nil {
		t.Error("Vision config should not be nil")
	}
	if config.Spatial == nil {
		t.Error("Spatial config should not be nil")
	}
	if config.VFX == nil {
		t.Error("VFX config should not be nil")
	}
	if config.Audio == nil {
		t.Error("Audio config should not be nil")
	}
	if config.Destruction == nil {
		t.Error("Destruction config should not be nil")
	}
	if config.Simulation == nil {
		t.Error("Simulation config should not be nil")
	}
}

// ──────────────────────────────────────────────────────────────
// Orchestrator lifecycle tests
// ──────────────────────────────────────────────────────────────

func TestNewOrchestrator(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewOrchestrator(config)

	if orch == nil {
		t.Fatal("NewOrchestrator returned nil")
	}
	if orch.GetStep() != 0 {
		t.Errorf("initial step: got %v, want 0", orch.GetStep())
	}
}

func TestProcessFrameEmpty(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewOrchestrator(config)
	ctx := context.Background()

	input := FrameInput{
		Timestamp: time.Now(),
	}

	result, err := orch.ProcessFrame(ctx, input)
	if err != nil {
		t.Fatalf("ProcessFrame: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Step != 0 {
		t.Errorf("step: got %v, want 0", result.Step)
	}
	if result.Latency < 0 {
		t.Error("latency should be non-negative")
	}
}

func TestProcessFrameAdvancesStep(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewOrchestrator(config)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		input := FrameInput{Timestamp: time.Now()}
		result, err := orch.ProcessFrame(ctx, input)
		if err != nil {
			t.Fatalf("ProcessFrame step %d: %v", i, err)
		}
		if result.Step != int64(i) {
			t.Errorf("step: got %v, want %d", result.Step, i)
		}
	}

	if orch.GetStep() != 5 {
		t.Errorf("final step: got %v, want 5", orch.GetStep())
	}
}

// ──────────────────────────────────────────────────────────────
// Entity management tests
// ──────────────────────────────────────────────────────────────

func TestMergeEntities(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewOrchestrator(config)

	entities := []WorldEntity{
		{
			ID:         "e1",
			Type:       EntityObject,
			Position:   Vec3{X: 1, Y: 0, Z: 0},
			Label:      "box",
			Confidence: 0.9,
			LastSeen:   time.Now(),
		},
		{
			ID:         "e2",
			Type:       EntityNPC,
			Position:   Vec3{X: 5, Y: 0, Z: 0},
			Label:      "guard",
			Confidence: 0.85,
			LastSeen:   time.Now(),
		},
	}

	orch.mergeEntities(entities)

	state := orch.GetState()
	if len(state.Entities) != 2 {
		t.Errorf("entities: got %d, want 2", len(state.Entities))
	}
}

func TestMergeEntitiesUpdate(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewOrchestrator(config)

	// First frame
	orch.mergeEntities([]WorldEntity{
		{ID: "e1", Position: Vec3{X: 1, Y: 0, Z: 0}, Label: "box", LastSeen: time.Now()},
	})

	// Second frame — same entity, moved
	orch.mergeEntities([]WorldEntity{
		{ID: "e1", Position: Vec3{X: 5, Y: 0, Z: 0}, Label: "box", LastSeen: time.Now()},
	})

	state := orch.GetState()
	if len(state.Entities) != 1 {
		t.Errorf("entities: got %d, want 1 (should not duplicate)", len(state.Entities))
	}
	if state.Entities[0].Position.X != 5 {
		t.Errorf("position X: got %v, want 5 (should update)", state.Entities[0].Position.X)
	}
}

func TestMergeEntitiesPrune(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewOrchestrator(config)

	// Add entity with old timestamp
	orch.mergeEntities([]WorldEntity{
		{ID: "e1", Position: Vec3{X: 1, Y: 0, Z: 0}, LastSeen: time.Now().Add(-10 * time.Second)},
	})

	// Merge new entities — old one should be pruned
	orch.mergeEntities([]WorldEntity{
		{ID: "e2", Position: Vec3{X: 2, Y: 0, Z: 0}, LastSeen: time.Now()},
	})

	state := orch.GetState()
	if len(state.Entities) != 1 {
		t.Errorf("entities: got %d, want 1 (old should be pruned)", len(state.Entities))
	}
	if state.Entities[0].ID != "e2" {
		t.Errorf("remaining entity: got %v, want e2", state.Entities[0].ID)
	}
}

func TestMergeEntitiesMaxLimit(t *testing.T) {
	config := DefaultOrchestratorConfig()
	config.MaxEntities = 3
	orch := NewOrchestrator(config)

	for i := 0; i < 5; i++ {
		orch.mergeEntities([]WorldEntity{
			{ID: "e" + string(rune('0'+i)), Position: Vec3{X: float64(i), Y: 0, Z: 0}, LastSeen: time.Now()},
		})
	}

	state := orch.GetState()
	if len(state.Entities) > 3 {
		t.Errorf("entities: got %d, want <= 3", len(state.Entities))
	}
}

// ──────────────────────────────────────────────────────────────
// GetEntity tests
// ──────────────────────────────────────────────────────────────

func TestGetEntity(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewOrchestrator(config)

	orch.mergeEntities([]WorldEntity{
		{ID: "e1", Label: "box", LastSeen: time.Now()},
	})

	entity, ok := orch.GetEntity("e1")
	if !ok {
		t.Fatal("entity not found")
	}
	if entity.Label != "box" {
		t.Errorf("label: got %v, want box", entity.Label)
	}
}

func TestGetEntityNotFound(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewOrchestrator(config)

	_, ok := orch.GetEntity("nonexistent")
	if ok {
		t.Error("GetEntity with nonexistent ID should return false")
	}
}

// ──────────────────────────────────────────────────────────────
// Event injection tests
// ──────────────────────────────────────────────────────────────

func TestInjectEventDestroy(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewOrchestrator(config)

	orch.mergeEntities([]WorldEntity{
		{ID: "e1", Position: Vec3{X: 1, Y: 0, Z: 0}, LastSeen: time.Now()},
		{ID: "e2", Position: Vec3{X: 2, Y: 0, Z: 0}, LastSeen: time.Now()},
	})

	orch.InjectEvent(WorldEvent{
		Type:     "destroy",
		EntityID: "e1",
	})

	state := orch.GetState()
	if len(state.Entities) != 1 {
		t.Errorf("entities: got %d, want 1", len(state.Entities))
	}
	if state.Entities[0].ID != "e2" {
		t.Errorf("remaining: got %v, want e2", state.Entities[0].ID)
	}
}

func TestInjectEventSpawn(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewOrchestrator(config)

	// Spawn event — entity appears on next vision frame
	orch.InjectEvent(WorldEvent{
		Type:     "spawn",
		EntityID: "e1",
		Position: Vec3{X: 5, Y: 0, Z: 0},
	})

	// No immediate change
	state := orch.GetState()
	if len(state.Entities) != 0 {
		t.Errorf("entities: got %d, want 0 (spawn happens on next frame)", len(state.Entities))
	}
}

func TestInjectEventWeatherChange(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewOrchestrator(config)

	orch.InjectEvent(WorldEvent{
		Type: "weather_change",
		Metadata: map[string]string{"rain": "0.8"},
	})

	// Climate is updated externally, not by event
	state := orch.GetState()
	if state.Climate.Rain != 0 {
		t.Errorf("rain: got %v, want 0 (climate updated externally)", state.Climate.Rain)
	}
}

// ──────────────────────────────────────────────────────────────
// Climate update tests
// ──────────────────────────────────────────────────────────────

func TestUpdateClimate(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewOrchestrator(config)

	orch.UpdateClimate(ClimateState{
		Temperature: 35.0,
		Wind:        Vec3{X: 5, Y: 0, Z: 0},
		Rain:        0.8,
		TimeOfDay:   15.0,
		Season:      Summer,
	})

	state := orch.GetState()
	if state.Climate.Temperature != 35.0 {
		t.Errorf("temperature: got %v, want 35.0", state.Climate.Temperature)
	}
	if state.Climate.Rain != 0.8 {
		t.Errorf("rain: got %v, want 0.8", state.Climate.Rain)
	}
	if state.Climate.Season != Summer {
		t.Errorf("season: got %v, want summer", state.Climate.Season)
	}
}

// ──────────────────────────────────────────────────────────────
// FrameInput/FrameResult structure tests
// ──────────────────────────────────────────────────────────────

func TestFrameInputStructure(t *testing.T) {
	input := FrameInput{
		Camera:     []byte{0x89, 0x50, 0x4E, 0x47},
		IMU:        []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6},
		Timestamp:  time.Now(),
		AudioData:  []byte{0x01, 0x02},
		SampleRate: 16000,
	}

	if len(input.Camera) != 4 {
		t.Errorf("camera: got %d bytes, want 4", len(input.Camera))
	}
	if len(input.IMU) != 6 {
		t.Errorf("imu: got %d values, want 6", len(input.IMU))
	}
	if input.SampleRate != 16000 {
		t.Errorf("sample_rate: got %v, want 16000", input.SampleRate)
	}
}

func TestActionResultStructure(t *testing.T) {
	action := ActionResult{
		Type:   "destruction",
		Action: "fracture",
		Target: "wall_01",
		Metadata: map[string]string{
			"method": "voronoi",
			"frags":  "15",
		},
	}

	if action.Type != "destruction" {
		t.Errorf("type: got %v, want destruction", action.Type)
	}
	if action.Metadata["method"] != "voronoi" {
		t.Errorf("method: got %v, want voronoi", action.Metadata["method"])
	}
}
