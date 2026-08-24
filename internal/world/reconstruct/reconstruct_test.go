package reconstruct

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/CoscaAI/cosca/internal/bridge"
	"github.com/CoscaAI/cosca/internal/ingest"
	"github.com/CoscaAI/cosca/internal/world"
	"github.com/CoscaAI/cosca/internal/world/adapter"
)

// mockClient captures bridge messages (no real WebSocket).
type mockClient struct {
	sent []bridge.Message
}

func (m *mockClient) Connect(ctx context.Context, url string) error { return nil }
func (m *mockClient) Disconnect(ctx context.Context) error          { return nil }
func (m *mockClient) Send(ctx context.Context, msg bridge.Message) error {
	m.sent = append(m.sent, msg)
	return nil
}
func (m *mockClient) Receive(ctx context.Context) (bridge.Message, error) { return bridge.Message{}, nil }
func (m *mockClient) SendFrame(ctx context.Context, f bridge.FramePayload) error {
	return nil
}
func (m *mockClient) SendAction(ctx context.Context, a bridge.ActionPayload) error {
	return nil
}
func (m *mockClient) OnMessage(t bridge.MessageType, h func(bridge.Message)) {}
func (m *mockClient) IsConnected() bool                                      { return true }

// TestReconstructRealWorld materializes the ENTIRE real-world Palhoça OSM
// extract through the UnrealAdapter (mock) — proving the pipeline:
//   Real OSM → World Model → Reconstructor → UnrealAdapter → bridge commands
func TestReconstructRealWorld(t *testing.T) {
	data, err := os.ReadFile("../../ingest/testdata/palhoca_sample.json")
	if err != nil {
		t.Fatal(err)
	}
	res, err := ingest.ParseOSM(data, ingest.DefaultConfig(world.GeoCoordinates{Latitude: -27.6375, Longitude: -48.6765}))
	if err != nil {
		t.Fatal(err)
	}

	mc := &mockClient{}
	ctrl := bridge.NewController(mc)
	adapter := adapter.NewUnreal(ctrl, context.Background())
	recon := New(adapter)

	rec, err := recon.Reconstruct(res.World, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Materialized == 0 {
		t.Fatal("nothing materialized from real world")
	}
	if len(rec.Errors) > 0 {
		t.Fatalf("materialization errors: %v", rec.Errors)
	}

	// Every entity produced a valid import_mesh command with a semantic
	// AssetID (no /Game/ path).
	var importCmds int
	for _, msg := range mc.sent {
		if msg.Type != bridge.MessageImportMesh {
			continue
		}
		importCmds++
		var payload bridge.ImportMeshPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Fatalf("bad payload: %v", err)
		}
		if payload.MeshPath == "" {
			t.Error("empty AssetID in import_mesh")
		}
		if len(payload.MeshPath) > 0 && payload.MeshPath[0] == '/' {
			t.Errorf("AssetID must be semantic, got %q", payload.MeshPath)
		}
	}
	if importCmds == 0 {
		t.Error("no import_mesh commands emitted")
	}
}

// TestReconstructClassFilter materializes only a subset of classes.
func TestReconstructClassFilter(t *testing.T) {
	data, _ := os.ReadFile("../../ingest/testdata/palhoca_sample.json")
	res, _ := ingest.ParseOSM(data, ingest.DefaultConfig(world.GeoCoordinates{Latitude: -27.6375, Longitude: -48.6765}))

	mc := &mockClient{}
	ctrl := bridge.NewController(mc)
	adapter := adapter.NewUnreal(ctrl, context.Background())
	recon := New(adapter)

	// Only structures.
	rec, err := recon.Reconstruct(res.World, Options{OnlyClasses: []world.EntityClass{world.ClassStructure}})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Materialized == 0 {
		t.Error("no structures materialized")
	}
}

// TestAssetMappingSemantic verifies the semantic asset mapping covers classes.
func TestAssetMappingSemantic(t *testing.T) {
	cases := []struct {
		class world.EntityClass
		eType world.EntityType
	}{
		{world.ClassRoad, "road.secondary"},
		{world.ClassStructure, "building.apartments"},
		{world.ClassWater, "water.river"},
		{world.ClassTerrain, "park"},
		{world.ClassVegetation, "tree.oak"},
	}
	for _, c := range cases {
		e := world.Entity{Class: c.class, Type: c.eType}
		id := adapter.MaterializeAssetIDForTest(e)
		if id == "" {
			t.Errorf("class %s type %s → empty asset", c.class, c.eType)
		}
		if id[0] == '/' {
			t.Errorf("asset must be semantic, got %q", id)
		}
	}
}
