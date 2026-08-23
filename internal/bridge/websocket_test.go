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

// startMockServer starts an in-process WebSocket server that echoes messages
// back to the client. It returns the ws:// URL to connect to.
func startMockServer(t *testing.T) string {
	t.Helper()
	// Use a random port
	mux := http.NewServeMux()
	mux.HandleFunc("/cosca", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:      []string{"cosca"},
			CompressionMode:   websocket.CompressionDisabled,
		})
		if err != nil {
			return
		}
		// Echo loop: read message, send it back with an ack type
		for {
			ctx := context.Background()
			var msg Message
			if err := wsjson.Read(ctx, conn, &msg); err != nil {
				conn.Close(websocket.StatusNormalClosure, "done")
				return
			}
			// Echo back the message (simulates Unreal ack)
			msg.Type = MessagePong
			wsjson.Write(ctx, conn, msg)
		}
	})

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: mux}
	go server.Serve(l)

	// Derive the ws:// URL from the listener address
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return "ws://127.0.0.1:" + port + "/cosca"
}

func TestWebSocketClientConnectAndSend(t *testing.T) {
	url := startMockServer(t)
	client := NewWebSocketClient()

	ctx := context.Background()
	if err := client.Connect(ctx, url); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Disconnect(ctx)

	if !client.IsConnected() {
		t.Fatal("not connected after Connect")
	}

	// Send a spawn message
	data, _ := json.Marshal(SpawnPayload{
		Type:     "object",
		Position: [3]float64{0, 0, 50},
	})
	err := client.Send(ctx, Message{
		Type:    MessageSpawn,
		Payload: data,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestWebSocketClientReceive(t *testing.T) {
	url := startMockServer(t)
	client := NewWebSocketClient()

	ctx := context.Background()
	if err := client.Connect(ctx, url); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Disconnect(ctx)

	// Send a command, then receive the echo back
	data, _ := json.Marshal(ActionPayload{
		EntityID: "cube_1",
		Action:   "move_to",
		Params:   map[string]any{"target": []float64{100, 0, 50}},
	})
	if err := client.Send(ctx, Message{Type: MessageAction, Payload: data}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Receive the echoed message
	msg, err := client.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if msg.Type != MessagePong {
		t.Errorf("received type: got %v, want pong", msg.Type)
	}
}

func TestWebSocketClientDispatch(t *testing.T) {
	url := startMockServer(t)
	client := NewWebSocketClient()

	ctx := context.Background()
	if err := client.Connect(ctx, url); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Disconnect(ctx)

	// Register a handler for pong
	got := make(chan Message, 1)
	client.OnMessage(MessagePong, func(m Message) {
		got <- m
	})

	// Send a message so the mock echoes it back as pong
	data, _ := json.Marshal(MessageSpawn)
	if err := client.Send(ctx, Message{Type: MessageSpawn, Payload: data}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case m := <-got:
		if m.Type != MessagePong {
			t.Errorf("dispatched type: got %v, want pong", m.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for dispatched message")
	}
}

func TestWebSocketClientSendAction(t *testing.T) {
	url := startMockServer(t)
	client := NewWebSocketClient()

	ctx := context.Background()
	if err := client.Connect(ctx, url); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Disconnect(ctx)

	err := client.SendAction(ctx, ActionPayload{
		EntityID: "cube_2",
		Action:   "interact",
		Params:   map[string]any{"verb": "open"},
	})
	if err != nil {
		t.Fatalf("SendAction: %v", err)
	}
}

func TestWebSocketClientNotConnected(t *testing.T) {
	client := NewWebSocketClient()
	ctx := context.Background()

	err := client.Send(ctx, Message{Type: MessageSpawn})
	if err == nil {
		t.Error("Send without connect should fail")
	}
}
