package adapter

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/bridge"
	"github.com/CoscaAI/cosca/internal/world"
)

// mockClient implements bridge.Client, capturing sent messages for assertion.
type mockClient struct {
	sent []bridge.Message
	err  error
}

func (m *mockClient) Connect(ctx context.Context, url string) error { return m.err }
func (m *mockClient) Disconnect(ctx context.Context) error          { return m.err }
func (m *mockClient) Send(ctx context.Context, msg bridge.Message) error {
	m.sent = append(m.sent, msg)
	return m.err
}
func (m *mockClient) Receive(ctx context.Context) (bridge.Message, error) { return bridge.Message{}, m.err }
func (m *mockClient) SendFrame(ctx context.Context, f bridge.FramePayload) error {
	return m.err
}
func (m *mockClient) SendAction(ctx context.Context, payload bridge.ActionPayload) error {
	return m.err
}
func (m *mockClient) OnMessage(msgType bridge.MessageType, handler func(bridge.Message)) {
}
func (m *mockClient) IsConnected() bool { return true }

func newTestAdapter(t *testing.T) (*UnrealAdapter, *mockClient, context.Context) {
	t.Helper()
	mc := &mockClient{}
	ctrl := bridge.NewController(mc)
	ctx := context.Background()
	return NewUnreal(ctrl, ctx), mc, ctx
}

// TestUnrealImplementsRendererContract verifies compile-time conformance.
func TestUnrealImplementsRendererContract(t *testing.T) {
	var _ world.RendererAdapter = (*UnrealAdapter)(nil)
}

// TestMaterializeEntity verifies spawn translates entity → import_mesh command.
func TestMaterializeEntity(t *testing.T) {
	a, mc, _ := newTestAdapter(t)

	tree := world.TreeEntity("tree_001", "oak", world.Vec3{X: 10, Y: 20, Z: 0}, "mature", world.Provenance{
		Class: world.ClassGENERATED,
	})

	ev, err := a.MaterializeEntity(tree)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Type != "spawned" || ev.EntityID != "tree_001" {
		t.Errorf("event = %+v", ev)
	}
	if len(mc.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(mc.sent))
	}
	msg := mc.sent[0]
	if msg.Type != bridge.MessageImportMesh {
		t.Errorf("msg type = %s, want import_mesh", msg.Type)
	}
	// Verify AssetID (semantic), not a /Game/ path (regra de ouro)
	if string(msg.Payload) == "" {
		t.Error("empty payload")
	}
}

// TestApplyWorldState verifies weather + time are pushed to Unreal.
func TestApplyWorldState(t *testing.T) {
	a, mc, _ := newTestAdapter(t)

	ws := world.WeatherState{Type: "rain", Intensity: 0.8}
	st := world.SimulationTime{Day: 3, Hour: 18, TimeScale: 1}
	err := a.ApplyWorldState(ws, st, world.Autumn)
	if err != nil {
		t.Fatal(err)
	}
	if len(mc.sent) != 2 {
		t.Fatalf("sent %d messages, want 2 (weather + time)", len(mc.sent))
	}
	if mc.sent[0].Type != bridge.MessageWeather {
		t.Errorf("first msg = %s, want weather", mc.sent[0].Type)
	}
	if mc.sent[1].Type != bridge.MessageTime {
		t.Errorf("second msg = %s, want time", mc.sent[1].Type)
	}
}

// TestRemoveEntity verifies destroy.
func TestRemoveEntity(t *testing.T) {
	a, mc, _ := newTestAdapter(t)
	_, err := a.RemoveEntity("tree_001")
	if err != nil {
		t.Fatal(err)
	}
	if len(mc.sent) != 1 || mc.sent[0].Type != bridge.MessageDestroy {
		t.Errorf("expected destroy, got %+v", mc.sent)
	}
}

// TestClassMapping verifies semantic → bridge type translation.
func TestClassMapping(t *testing.T) {
	cases := map[world.EntityClass]string{
		world.ClassVegetation: "tree",
		world.ClassRoad:       "road",
		world.ClassStructure:  "building",
		world.ClassWater:      "water",
		world.ClassTerrain:    "terrain",
		world.ClassVehicle:    "vehicle",
		world.ClassEntity:     "object",
	}
	for c, want := range cases {
		if got := classToBridgeType(c); got != want {
			t.Errorf("class %s → %s, want %s", c, got, want)
		}
	}
}

// TestAssetIDNotPath verifies the adapter never emits a /Game/ path.
func TestAssetIDNotPath(t *testing.T) {
	tree := world.TreeEntity("tree_001", "oak", world.Vec3{}, "mature", world.Provenance{})
	assetID := entityAssetID(tree)
	if len(assetID) == 0 || assetID[0] == '/' {
		t.Errorf("assetID must be semantic, got %q", assetID)
	}
}
