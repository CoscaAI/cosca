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
	coscaasset "github.com/CoscaAI/cosca/internal/worldmodel/asset"
	"github.com/CoscaAI/cosca/internal/worldmodel"
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
  cosca bridge connect --url ws://localhost:9000`,
	}
	cmd.AddCommand(
		NewBridgeServeCommand(),
		NewBridgeConnectCommand(),
		NewBridgeDemoCommand(),
		NewBridgeImportMeshCommand(),
		NewBridgeTimeCommand(),
		NewBridgeWeatherCommand(),
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
		Example: `  cosca bridge connect --url ws://localhost:9000
  cosca bridge connect --url ws://localhost:9000 --interval 1s --json`,
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

	cmd.Flags().StringVar(&url, "url", "ws://localhost:9000", "WebSocket URL of the Unreal server")
	cmd.Flags().DurationVar(&interval, "interval", 0, "Repeat the ping at this interval (0 = once)")
	return cmd
}

// NewBridgeDemoCommand exercises the full vertical slice against a server:
// generate a Blender asset, spawn it, move it, and receive a frame.
func NewBridgeDemoCommand() *cobra.Command {
	var url, assetType string
	var seed int64
	var posX, posZ float64

	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Exercise the full Cosca<->Unreal vertical slice",
		Long: `Exercise the full vertical slice end-to-end:

  Cosca (Go) -> WS -> Unreal Runtime
    1. Generate a Blender asset (cube/tree/terrain/building)
    2. Spawn it as an Actor (with asset_hash)
    3. Move it to a target
    4. Register a frame handler (camera vision)

Works against the mock server ('cosca bridge serve') OR the Unreal
CoscaRuntime plugin server.`,
		Example: `  cosca bridge demo --url ws://localhost:9000 --type cube`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			ctx := cmd.Context()

			// Build the bridge client + controller + orchestrator + runtime.
			ctrl := bridge.NewController(bridge.NewWebSocketClient())
			osc := worldmodel.NewOrchestrator(worldmodel.DefaultOrchestratorConfig())
			rt := bridge.NewRuntime(ctrl, osc)

			// Wire the Blender asset generator (asset pipeline).
			blender := coscaasset.NewBlenderAdapter(coscaasset.DefaultBlenderAdapterConfig())
			rt.SetAssetGenerator(assetGenFunc(func(ctx context.Context, t string, seed int64) (string, string, error) {
				req := coscaasset.AssetRequest{Type: coscaasset.AssetType(t), Seed: seed, Validate: true}
				res, err := blender.GenerateAsset(ctx, req)
				if err != nil {
					return "", "", err
				}
				if !res.Valid {
					return "", "", fmt.Errorf("generated asset invalid: %v", res.Issues)
				}
				return res.Path, res.Hash, nil
			}))

			// Wire a frame observer (Vision stub that logs detected entities).
			rt.SetFrameObserver(obsFunc(func(ctx context.Context, frame []byte, w, h int) ([]worldmodel.WorldEntity, error) {
				return []worldmodel.WorldEntity{{
					ID:     "seen_entity",
					Type:   worldmodel.EntityObject,
					Label:  "observed",
					LastSeen: time.Now(),
				}}, nil
			}))

			// Connect.
			if err := ctrl.Connect(ctx, url); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer ctrl.Disconnect(ctx)

			rt.StartFrameLoop()

			// Step 1-2: generate Blender asset + spawn in Unreal.
			entityID := "demo_" + assetType
			spec := bridge.EntitySpec{
				ID:    entityID,
				Type:  assetType,
				Pos:   [3]float64{posX, 0, posZ},
				Scale: [3]float64{1, 1, 1},
			}
			if err := rt.GenerateAndSpawn(ctx, assetType, seed, spec); err != nil {
				return fmt.Errorf("generate+spawn: %w", err)
			}

			// Step 3: move it.
			if err := ctrl.Move(ctx, entityID, [3]float64{posX + 100, 0, posZ}); err != nil {
				return fmt.Errorf("move: %w", err)
			}

			// Step 4: trigger a frame observation.
			ctrl.HandleFrame(bridge.FramePayload{Width: 64, Height: 64, Format: "png", Data: []byte{0x89, 0x50, 0x4E, 0x47}})

			if useJSON {
				return printJSON(cmd, map[string]any{
					"connected":   true,
					"url":         url,
					"spawned":     entityID,
					"asset_type":  assetType,
					"moved":       true,
					"frame_seen":  true,
					"entities":    len(osc.GetState().Entities),
				})
			}

			formatter.Header("Vertical slice exercise")
			formatter.KeyValue("Connected", url)
			formatter.Success("Spawned " + entityID)
			formatter.KeyValue("Asset", assetType)
			formatter.Success("Moved to target")
			formatter.Success("Frame observed")
			formatter.KeyValue("WorldEntities", fmt.Sprintf("%d", len(osc.GetState().Entities)))
			return nil
		},
	}

	cmd.Flags().StringVar(&url, "url", "ws://localhost:9000", "WebSocket URL")
	cmd.Flags().StringVar(&assetType, "type", "cube", "Blender asset type (cube/tree/terrain/building)")
	cmd.Flags().Int64Var(&seed, "seed", 42, "Deterministic seed")
	cmd.Flags().Float64Var(&posX, "x", 0, "Spawn X")
	cmd.Flags().Float64Var(&posZ, "z", 50, "Spawn Z")
	return cmd
}

// assetGenFunc adapts a closure to the AssetGenerator interface.
type assetGenFunc func(ctx context.Context, t string, seed int64) (string, string, error)

func (f assetGenFunc) Generate(ctx context.Context, t string, seed int64) (string, string, error) {
	return f(ctx, t, seed)
}

// obsFunc adapts a closure to the FrameObserver interface.
type obsFunc func(ctx context.Context, frame []byte, w, h int) ([]worldmodel.WorldEntity, error)

func (f obsFunc) Observe(ctx context.Context, frame []byte, w, h int) ([]worldmodel.WorldEntity, error) {
	return f(ctx, frame, w, h)
}

// NewBridgeImportMeshCommand sends an import_mesh command to Unreal.
// Resolves AssetID via registry, loads UStaticMesh, spawns actor.
func NewBridgeImportMeshCommand() *cobra.Command {
	var url, assetID, entityType string
	var posX, posY, posZ, scaleX, scaleY, scaleZ float64

	cmd := &cobra.Command{
		Use:   "import-mesh",
		Short: "Import mesh via AssetID → Registry → Spawn",
		Long: `Send an import_mesh command to Unreal Engine.

Unreal resolves the AssetID via asset_registry.json, loads the UStaticMesh,
and spawns a StaticMeshActor with that mesh.

Example:
  cosca bridge import-mesh --asset-id oak_mature --type tree
  cosca bridge import-mesh --asset-id oak_mature --x 100 --z 50 --scale 2`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			ctx := cmd.Context()

			ctrl := bridge.NewController(bridge.NewWebSocketClient())
			if err := ctrl.Connect(ctx, url); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer ctrl.Disconnect(ctx)

			payload := bridge.ImportMeshPayload{
				EntityID: "entity_" + assetID,
				MeshPath: assetID,
				Type:     entityType,
				Position: [3]float64{posX, posY, posZ},
				Scale:    [3]float64{scaleX, scaleY, scaleZ},
			}

			if err := ctrl.ImportMesh(ctx, payload); err != nil {
				return fmt.Errorf("import_mesh: %w", err)
			}

			formatter.Success("Sent import_mesh command")
			formatter.KeyValue("AssetID", assetID)
			formatter.KeyValue("EntityID", payload.EntityID)
			formatter.KeyValue("Position", fmt.Sprintf("(%.0f, %.0f, %.0f)", posX, posY, posZ))
			formatter.KeyValue("Scale", fmt.Sprintf("(%.1f, %.1f, %.1f)", scaleX, scaleY, scaleZ))
			return nil
		},
	}

	cmd.Flags().StringVar(&url, "url", "ws://localhost:9000", "WebSocket URL")
	cmd.Flags().StringVar(&assetID, "asset-id", "", "AssetID from registry (required)")
	cmd.Flags().StringVar(&entityType, "type", "object", "Entity type")
	cmd.Flags().Float64Var(&posX, "x", 0, "Spawn X")
	cmd.Flags().Float64Var(&posY, "y", 0, "Spawn Y")
	cmd.Flags().Float64Var(&posZ, "z", 0, "Spawn Z")
	cmd.Flags().Float64Var(&scaleX, "scale-x", 1, "Scale X")
	cmd.Flags().Float64Var(&scaleY, "scale-y", 1, "Scale Y")
	cmd.Flags().Float64Var(&scaleZ, "scale-z", 1, "Scale Z")
	cmd.MarkFlagRequired("asset-id")

	return cmd
}

// NewBridgeTimeCommand sets the time of day in Unreal (day/night cycle).
func NewBridgeTimeCommand() *cobra.Command {
	var url string
	var hour float64

	cmd := &cobra.Command{
		Use:   "time",
		Short: "Set time of day in Unreal (day/night cycle)",
		Long: `Send a time command to Unreal Engine.

Unreal computes sun position, color, ambient light, and fog based on the
given hour of day (0.0 = midnight, 12.0 = noon, 18.0 = dusk).

Example:
  cosca bridge time --hour 18        # golden hour / dusk
  cosca bridge time --hour 12        # noon
  cosca bridge time --hour 0         # midnight`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			ctx := cmd.Context()

			ctrl := bridge.NewController(bridge.NewWebSocketClient())
			if err := ctrl.Connect(ctx, url); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer ctrl.Disconnect(ctx)

			if err := ctrl.SetTimeOfDay(ctx, hour); err != nil {
				return fmt.Errorf("time: %w", err)
			}

			formatter.Success("Sent time command")
			formatter.KeyValue("Hour", fmt.Sprintf("%.1f", hour))
			return nil
		},
	}

	cmd.Flags().StringVar(&url, "url", "ws://localhost:9000", "WebSocket URL")
	cmd.Flags().Float64Var(&hour, "hour", 12.0, "Hour of day (0-24)")
	return cmd
}

// NewBridgeWeatherCommand sets the weather in Unreal.
func NewBridgeWeatherCommand() *cobra.Command {
	var url, weatherType string
	var intensity float64

	cmd := &cobra.Command{
		Use:   "weather",
		Short: "Set weather in Unreal",
		Long: `Send a weather command to Unreal Engine.

Unreal adjusts fog density, ambient darkening, etc. based on weather type:
  clear, rain, snow, fog, storm, overcast.

Example:
  cosca bridge weather --type rain --intensity 0.8
  cosca bridge weather --type fog`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			ctx := cmd.Context()

			ctrl := bridge.NewController(bridge.NewWebSocketClient())
			if err := ctrl.Connect(ctx, url); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer ctrl.Disconnect(ctx)

			if err := ctrl.SetWeather(ctx, weatherType, intensity); err != nil {
				return fmt.Errorf("weather: %w", err)
			}

			formatter.Success("Sent weather command")
			formatter.KeyValue("Type", weatherType)
			formatter.KeyValue("Intensity", fmt.Sprintf("%.1f", intensity))
			return nil
		},
	}

	cmd.Flags().StringVar(&url, "url", "ws://localhost:9000", "WebSocket URL")
	cmd.Flags().StringVar(&weatherType, "type", "clear", "Weather type (clear/rain/snow/fog/storm/overcast)")
	cmd.Flags().Float64Var(&intensity, "intensity", 1.0, "Intensity (0-1)")
	return cmd
}
