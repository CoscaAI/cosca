package bridge

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func TestControllerSpawn(t *testing.T) {
	listener, err := Listen(0)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	go ServeMock(listener)

	client := NewWebSocketClient()
	ctrl := NewController(client)
	ctx := context.Background()

	if err := ctrl.Connect(ctx, "ws://127.0.0.1:"+port+"/cosca"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer ctrl.Disconnect(ctx)

	spec := EntitySpec{
		ID:     "cube_1",
		Type:   "cube",
		Pos:    [3]float64{0, 0, 50},
		Scale:  [3]float64{1, 1, 1},
		Tags:   []string{"cosca.object.cube"},
	}
	if err := ctrl.Spawn(ctx, spec); err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	entities := ctrl.GetEntities()
	if _, ok := entities["cube_1"]; !ok {
		t.Error("cube_1 not registered locally")
	}
}

func TestControllerMove(t *testing.T) {
	listener, err := Listen(0)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	go ServeMock(listener)

	client := NewWebSocketClient()
	ctrl := NewController(client)
	ctx := context.Background()

	if err := ctrl.Connect(ctx, "ws://127.0.0.1:"+port+"/cosca"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer ctrl.Disconnect(ctx)

	if err := ctrl.Move(ctx, "cube_1", [3]float64{100, 0, 50}); err != nil {
		t.Fatalf("Move: %v", err)
	}
}

func TestControllerDestroy(t *testing.T) {
	listener, err := Listen(0)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	go ServeMock(listener)

	client := NewWebSocketClient()
	ctrl := NewController(client)
	ctx := context.Background()

	if err := ctrl.Connect(ctx, "ws://127.0.0.1:"+port+"/cosca"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer ctrl.Disconnect(ctx)

	_ = ctrl.Spawn(ctx, EntitySpec{ID: "cube_1", Type: "cube"})
	if err := ctrl.Destroy(ctx, "cube_1"); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if _, ok := ctrl.GetEntities()["cube_1"]; ok {
		t.Error("cube_1 should be removed after Destroy")
	}
}

func TestControllerFrameAndState(t *testing.T) {
	// Server that pushes a frame + state on connect.
	listener, err := Listen(0)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())

	mux := http.NewServeMux()
	mux.HandleFunc("/cosca", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:    []string{"cosca"},
			CompressionMode: websocket.CompressionDisabled,
		})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		// Push a frame
		frameData, _ := json.Marshal(FramePayload{Width: 64, Height: 64, Format: "png", Data: []byte{1, 2, 3}})
		_ = wsjson.Write(context.Background(), conn, Message{Type: MessageFrame, Payload: frameData})

		// Push a state
		stateData, _ := json.Marshal(StateSnapshot{Entities: []EntityState{{ID: "cube_1", Pos: [3]float64{0, 0, 50}}}})
		_ = wsjson.Write(context.Background(), conn, Message{Type: MessageStateSync, Payload: stateData})

		// Keep alive briefly
		time.Sleep(500 * time.Millisecond)
	})
	go http.Serve(listener, mux)

	client := NewWebSocketClient()
	ctrl := NewController(client)
	ctx := context.Background()

	frameCh := make(chan FramePayload, 1)
	stateCh := make(chan StateSnapshot, 1)
	ctrl.SetFrameHandler(func(f FramePayload) { frameCh <- f })
	ctrl.SetStateHandler(func(s StateSnapshot) { stateCh <- s })

	if err := ctrl.Connect(ctx, "ws://127.0.0.1:"+port+"/cosca"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer ctrl.Disconnect(ctx)

	select {
	case f := <-frameCh:
		if f.Width == 0 || f.Height == 0 {
			t.Errorf("frame dims zero: %dx%d", f.Width, f.Height)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no frame received")
	}

	select {
	case s := <-stateCh:
		if len(s.Entities) != 1 {
			t.Errorf("expected 1 entity, got %d", len(s.Entities))
		}
		if s.Entities[0].ID != "cube_1" {
			t.Errorf("entity id: got %v, want cube_1", s.Entities[0].ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no state received")
	}
}
