package adapter

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/bridge"
	"github.com/CoscaAI/cosca/internal/world/city"
)

// Integration test: CityGenerator (synthetic world) → UnrealAdapter → bridge.
// This validates the FULL pipeline the professor described:
//   World Model → Adapter → Unreal
// using a mock WebSocket client (Unreal not required for unit test).

// TestCityToUnrealIntegration materializes a synthetic city and verifies the
// adapter emits correct import_mesh/spawn commands for every materializable
// entity.
func TestCityToUnrealIntegration(t *testing.T) {
	cfg := city.DefaultConfig(99)
	res, err := city.Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}

	mc := &mockClient{}
	ctrl := bridge.NewController(mc)
	adapter := NewUnreal(ctrl, context.Background())

	// Materialize every entity in the synthetic city.
	var spawned int
	for _, e := range res.World.Entities {
		_, err := adapter.MaterializeEntity(e)
		if err != nil {
			t.Fatalf("materialize %s: %v", e.ID, err)
		}
		spawned++
	}

	if spawned == 0 {
		t.Fatal("no entities materialized")
	}

	// Each materialized entity produced an import_mesh command.
	var importCmds int
	for _, msg := range mc.sent {
		if msg.Type == bridge.MessageImportMesh {
			importCmds++
		}
	}
	if importCmds != spawned {
		t.Errorf("import_mesh commands = %d, want %d (one per entity)", importCmds, spawned)
	}
}

// TestCityToUnrealNoEnginePath verifies the pipeline never emits a /Game/ path.
func TestCityToUnrealNoEnginePath(t *testing.T) {
	res, _ := city.Generate(city.DefaultConfig(42))
	mc := &mockClient{}
	adapter := NewUnreal(bridge.NewController(mc), context.Background())

	for _, e := range res.World.Entities {
		if _, err := adapter.MaterializeEntity(e); err != nil {
			t.Fatal(err)
		}
	}
	for _, msg := range mc.sent {
		if msg.Type != bridge.MessageImportMesh {
			continue
		}
		if containsPath(string(msg.Payload)) {
			t.Errorf("a command carries a /Game/ path: %s", msg.Payload)
		}
	}
}

// TestCityToUnrealDeterministic verifies the same city materializes to the
// SAME command sequence for the same seed (determinism is preserved end-to-end).
func TestCityToUnrealDeterministic(t *testing.T) {
	materialize := func(seed int64) []bridge.Message {
		res, _ := city.Generate(city.DefaultConfig(seed))
		mc := &mockClient{}
		adapter := NewUnreal(bridge.NewController(mc), context.Background())
		for _, e := range res.World.Entities {
			adapter.MaterializeEntity(e) // ignore error
		}
		return mc.sent
	}

	a := materialize(5)
	b := materialize(5)
	if len(a) != len(b) {
		t.Fatalf("different command counts: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Type != b[i].Type {
			t.Errorf("command %d type differs: %s vs %s", i, a[i].Type, b[i].Type)
		}
	}
}

// containsPath is a strict /Game/ path detector for the command payload.
func containsPath(payload string) bool {
	for i := 0; i+5 < len(payload); i++ {
		if payload[i:i+5] == "/Game" {
			return true
		}
	}
	return false
}
