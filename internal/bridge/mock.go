package bridge

import (
	"context"
	"net"
	"net/http"
	"strconv"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// Listen binds a TCP listener on the given port for the mock server.
// It returns the listener so the caller can Close it.
func Listen(port int) (net.Listener, error) {
	return net.Listen("tcp", ":"+strconv.Itoa(port))
}

// ServeMock serves a WebSocket server that echoes every received message
// back as a "pong". This simulates the Unreal WorldSubsystem for local dev.
func ServeMock(l net.Listener) {
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

		for {
			ctx := context.Background()
			var msg Message
			if err := wsjson.Read(ctx, conn, &msg); err != nil {
				return
			}
			// Echo back as pong (ack)
			msg.Type = MessagePong
			wsjson.Write(ctx, conn, msg)
		}
	})

	srv := &http.Server{Handler: mux}
	_ = srv.Serve(l)
}
