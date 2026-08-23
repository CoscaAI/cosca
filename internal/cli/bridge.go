package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/bridge"
)

// NewBridgeCommand creates the `cosca bridge` command group — the nervous
// system between Cosca (cognition) and Unreal Engine (world/runtime).
func NewBridgeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bridge",
		Short: "Unreal Engine bridge (WebSocket nervous system)",
		Long: `Connect Cosca to Unreal Engine over WebSocket.

Cosca is the CEREBRO (cognition/decision), Unreal is the CORPO (world/runtime).
The bridge is the nervous system: Cosca pushes commands (spawn/move/destroy),
Unreal pushes observations (frames/state).

The Cosca Engine runs a WebSocket SERVER (IWebSocketServer) in Unreal, and
Cosca connects as the CLIENT. For local development without Unreal, use
'cosca bridge serve' to run a mock server.

Subcommands:
  serve    Run a mock WebSocket server (local dev, no Unreal needed)
  connect  Connect to the Unreal server and test the handshake`,
		Example: `  cosca bridge serve --port 9000
  cosca bridge connect --url ws://localhost:9000/cosca`,
	}
	cmd.AddCommand(
		NewBridgeServeCommand(),
		NewBridgeConnectCommand(),
	)
	return cmd
}

// NewBridgeServeCommand runs a mock WebSocket server for local development.
func NewBridgeServeCommand() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run a mock WebSocket server (local dev)",
		Long: `Run a mock WebSocket server that echoes messages back, simulating the
Unreal WorldSubsystem. Useful for developing the Cosca↔Unreal protocol
without launching Unreal.

The server accepts the 'cosca' subprotocol and echoes every received message
back as a 'pong' (ack).`,
		Example: `  cosca bridge serve --port 9000`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			// Serve in a goroutine and wait for SIGINT/SIGTERM.
			listener, err := bridge.Listen(port)
			if err != nil {
				return fmt.Errorf("listen: %w", err)
			}
			defer listener.Close()

			formatter.Success(fmt.Sprintf("Mock bridge server listening on %s", listener.Addr()))
			formatter.Println("  Waiting for Cosca client... (Ctrl+C to stop)")

			go bridge.ServeMock(listener)

			// Wait for signal
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			<-ctx.Done()
			formatter.Println("\n  Stopping bridge server.")
			return nil
		},
	}

	cmd.Flags().IntVar(&port, "port", 9000, "Port to listen on")
	return cmd
}

// NewBridgeConnectCommand connects to the Unreal server and tests the handshake.
func NewBridgeConnectCommand() *cobra.Command {
	var url string
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect to the Unreal server and test the handshake",
		Long: `Connect to the Unreal Engine WebSocket server, send a test command, and
report the ack. Verifies the Cosca↔Unreal protocol end-to-end.

If the server is the mock ('cosca bridge serve'), it echoes 'pong'. If it is
the Unreal WorldSubsystem, it responds with the real ack/state.`,
		Example: `  cosca bridge connect --url ws://localhost:9000/cosca
  cosca bridge connect --url ws://localhost:9000/cosca --interval 1s --json`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			client := bridge.NewWebSocketClient()
			ctx := cmd.Context()

			// Connect
			if err := client.Connect(ctx, url); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer client.Disconnect(ctx)

			// Register a pong handler and send a ping command.
			got := make(chan bridge.Message, 1)
			client.OnMessage(bridge.MessagePong, func(m bridge.Message) {
				select {
				case got <- m:
				default:
				}
			})

			// Send a ping to verify round-trip.
			if err := client.Send(ctx, bridge.Message{Type: bridge.MessagePing}); err != nil {
				return fmt.Errorf("send ping: %w", err)
			}

			// Wait for the ack (round-trip).
			select {
			case ack := <-got:
				if useJSON {
					return printJSON(cmd, map[string]any{
						"connected":  true,
						"url":        url,
						"ack_type":   string(ack.Type),
						"ack_id":     ack.ID,
						"roundtrip":  true,
					})
				}
				formatter.Success("Connected to " + url)
				formatter.KeyValue("Status", "handshake ok")
				formatter.Success("Round-trip OK")
				formatter.KeyValue("AckType", string(ack.Type))
				formatter.KeyValue("AckID", ack.ID)
			case <-time.After(5 * time.Second):
				if useJSON {
					return printJSON(cmd, map[string]any{
						"connected":  true,
						"url":        url,
						"roundtrip":  false,
						"error":      "timeout waiting for ack",
					})
				}
				formatter.Success("Connected to " + url)
				formatter.KeyValue("Status", "handshake ok")
				formatter.Warning("Round-trip timeout — server did not respond within 5s")
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&url, "url", "ws://localhost:9000/cosca", "WebSocket URL of the Unreal server")
	cmd.Flags().DurationVar(&interval, "interval", 0, "Repeat the ping at this interval (0 = once)")
	return cmd
}
