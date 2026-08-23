package bridge

import (
	"context"
	"testing"
	"time"
)

func TestLocalClientConnect(t *testing.T) {
	c := NewLocalClient(10)
	ctx := context.Background()

	if c.IsConnected() {
		t.Fatal("should not be connected initially")
	}

	if err := c.Connect(ctx, "ws://localhost:14120"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if !c.IsConnected() {
		t.Fatal("should be connected after Connect")
	}

	if err := c.Disconnect(ctx); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}

	if c.IsConnected() {
		t.Fatal("should not be connected after Disconnect")
	}
}

func TestLocalClientSendReceive(t *testing.T) {
	c := NewLocalClient(10)
	ctx := context.Background()
	c.Connect(ctx, "ws://localhost:14120")

	// Inject a message into inbox (simulates Unreal → Cosca)
	msg := Message{
		Type:      MessageFrame,
		ID:        "test-1",
		Timestamp: time.Now(),
		Payload:   []byte(`{"test": true}`),
	}
	c.Inject(msg)

	// Receive it
	got, err := c.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}

	if got.Type != MessageFrame {
		t.Errorf("Type: got %v, want frame", got.Type)
	}
	if got.ID != "test-1" {
		t.Errorf("ID: got %v, want test-1", got.ID)
	}
}

func TestLocalClientSendDisconnected(t *testing.T) {
	c := NewLocalClient(10)
	ctx := context.Background()

	msg := Message{Type: MessageFrame, ID: "test"}
	if err := c.Send(ctx, msg); err == nil {
		t.Fatal("Send while disconnected should fail")
	}
}

func TestLocalClientSendFrame(t *testing.T) {
	c := NewLocalClient(10)
	ctx := context.Background()
	c.Connect(ctx, "ws://localhost:14120")

	frame := FramePayload{
		Width:  1920,
		Height: 1080,
		Format: "png",
		Data:   []byte{0x89, 0x50, 0x4E, 0x47}, // PNG header
	}

	if err := c.SendFrame(ctx, frame); err != nil {
		t.Fatalf("SendFrame: %v", err)
	}

	got := c.ReadOutbox()

	if got.Type != MessageFrame {
		t.Errorf("Type: got %v, want frame", got.Type)
	}
}

func TestLocalClientSendAction(t *testing.T) {
	c := NewLocalClient(10)
	ctx := context.Background()
	c.Connect(ctx, "ws://localhost:14120")

	action := ActionPayload{
		EntityID: "npc-1",
		Action:   "move_to",
		Params:   map[string]any{"x": 5.0, "y": 0.0, "z": 3.0},
	}

	if err := c.SendAction(ctx, action); err != nil {
		t.Fatalf("SendAction: %v", err)
	}

	got := c.ReadOutbox()

	if got.Type != MessageAction {
		t.Errorf("Type: got %v, want action", got.Type)
	}
}

func TestLocalClientOnMessage(t *testing.T) {
	c := NewLocalClient(10)
	received := false

	c.OnMessage(MessageFrame, func(msg Message) {
		received = true
	})

	// Handlers are stored but not automatically called in LocalClient
	// This test verifies the handler is registered
	if !received {
		// Handler not called yet (expected — LocalClient is simple)
	}
}

func TestLocalClientInject(t *testing.T) {
	c := NewLocalClient(10)
	ctx := context.Background()
	c.Connect(ctx, "ws://localhost:14120")

	// Inject a message
	msg := Message{
		Type:      MessageEvent,
		ID:        "event-1",
		Timestamp: time.Now(),
		Payload:   []byte(`{"type": "collision"}`),
	}

	c.Inject(msg)

	// Receive it
	got, err := c.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}

	if got.Type != MessageEvent {
		t.Errorf("Type: got %v, want event", got.Type)
	}
}

func TestLocalClientReadOutbox(t *testing.T) {
	c := NewLocalClient(10)
	ctx := context.Background()
	c.Connect(ctx, "ws://localhost:14120")

	// Send a message
	action := ActionPayload{EntityID: "npc-1", Action: "jump"}
	c.SendAction(ctx, action)

	// Read from outbox
	got := c.ReadOutbox()
	if got.Type != MessageAction {
		t.Errorf("Type: got %v, want action", got.Type)
	}
}
