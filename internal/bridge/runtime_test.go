package bridge

import (
	"context"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// mockObserver is a test double for FrameObserver.
type mockObserver struct {
	entities []worldmodel.WorldEntity
}

func (m *mockObserver) Observe(ctx context.Context, frame []byte, w, h int) ([]worldmodel.WorldEntity, error) {
	return m.entities, nil
}

// mockAssetGen is a test double for AssetGenerator.
type mockAssetGen struct {
	path string
	hash string
	err  error
}

func (m *mockAssetGen) Generate(ctx context.Context, t string, seed int64) (string, string, error) {
	if m.err != nil {
		return "", "", m.err
	}
	m.path = "/tmp/" + t + ".glb"
	m.hash = "sha256:mockhash"
	return m.path, m.hash, nil
}

func TestRuntimeFrameLoop(t *testing.T) {
	// Runtime frame loop injects observed entities into Orchestrator.
	osc := worldmodel.NewOrchestrator(worldmodel.DefaultOrchestratorConfig())
	ctrl := NewController(&fakeClient{})
	rt := NewRuntime(ctrl, osc)

	rt.SetFrameObserver(&mockObserver{
		entities: []worldmodel.WorldEntity{
			{ID: "tree_1", Type: worldmodel.EntityObject, Label: "oak", Position: worldmodel.Vec3{X: 1, Y: 2, Z: 3}, LastSeen: time.Now()},
		},
	})

	rt.StartFrameLoop()

	// Simulate a frame arriving (via the controller's HandleFrame).
	ctrl.HandleFrame(FramePayload{Width: 64, Height: 64, Data: []byte{1, 2, 3}})

	// Orchestrator should now know about tree_1.
	state := osc.GetState()
	if len(state.Entities) != 1 {
		t.Fatalf("entities: got %d, want 1", len(state.Entities))
	}
	if state.Entities[0].ID != "tree_1" {
		t.Errorf("entity id: got %v, want tree_1", state.Entities[0].ID)
	}
}

func TestRuntimeGenerateAndSpawn(t *testing.T) {
	osc := worldmodel.NewOrchestrator(worldmodel.DefaultOrchestratorConfig())
	ctrl := NewController(&fakeClient{})
	rt := NewRuntime(ctrl, osc)

	rt.SetAssetGenerator(&mockAssetGen{})

	ctx := context.Background()
	err := rt.GenerateAndSpawn(ctx, "tree", 42, EntitySpec{ID: "tree_1"})
	if err != nil {
		t.Fatalf("GenerateAndSpawn: %v", err)
	}

	// Spawn should have been sent + entity registered.
	entities := ctrl.GetEntities()
	if _, ok := entities["tree_1"]; !ok {
		t.Error("tree_1 not registered in controller")
	}
}

func TestRuntimeGenerateNoGenerator(t *testing.T) {
	osc := worldmodel.NewOrchestrator(worldmodel.DefaultOrchestratorConfig())
	ctrl := NewController(&fakeClient{})
	rt := NewRuntime(ctrl, osc)

	err := rt.GenerateAndSpawn(context.Background(), "tree", 42, EntitySpec{ID: "tree_1"})
	if err == nil {
		t.Error("expected error when no asset generator configured")
	}
}

// fakeClient is an in-memory Client that records messages.
type fakeClient struct {
	connected bool
	sent      []Message
	frameHandler func(FramePayload)
}

func (f *fakeClient) Connect(ctx context.Context, url string) error { f.connected = true; return nil }
func (f *fakeClient) Disconnect(ctx context.Context) error { f.connected = false; return nil }
func (f *fakeClient) Send(ctx context.Context, m Message) error { f.sent = append(f.sent, m); return nil }
func (f *fakeClient) Receive(ctx context.Context) (Message, error) { return Message{}, nil }
func (f *fakeClient) SendFrame(ctx context.Context, fp FramePayload) error { return nil }
func (f *fakeClient) SendAction(ctx context.Context, a ActionPayload) error { return nil }
func (f *fakeClient) OnMessage(t MessageType, h func(Message)) {}
func (f *fakeClient) IsConnected() bool { return f.connected }
