package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/bridge"
	"github.com/CoscaAI/cosca/internal/ingest"
	"github.com/CoscaAI/cosca/internal/world"
	"github.com/CoscaAI/cosca/internal/world/adapter"
	"github.com/CoscaAI/cosca/internal/world/reconstruct"
)

// NewWorldCommand creates the `cosca world` command group — the World Model
// inspection tool. This is where the Cosca "explains what it found".
func NewWorldCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "world",
		Short: "World Model — inspecionar/o modelo do mundo",
		Long: `Inspect the Cosca World Model: ingest real data (OSM), explain the
semantic inventory, and freeze reference golden slices.

The World Model is the canonical, renderer-independent representation of the
world. Cosca interprets entities (buildings/roads/avenues/parks) — not meshes.`,
	}
	cmd.AddCommand(
		NewWorldInspectCommand(),
		NewWorldSpawnCommand(),
	)
	return cmd
}

// NewWorldInspectCommand ingests a real OSM dataset and explains it.
func NewWorldInspectCommand() *cobra.Command {
	var input, name string
	var lat, lon float64

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Ingest OSM real e explicar o que o Cosca encontrou",
		Long: `Ingest a real OSM/Overpass JSON extract into the World Model and
print the semantic inventory + validation + fingerprint.

Example:
  cosca world inspect --input internal/ingest/testdata/palhoca_sample.json \
    --name palhoca-1km --lat -27.6375 --lon -48.6765`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			data, err := os.ReadFile(input)
			if err != nil {
				return fmt.Errorf("read input: %w", err)
			}

			cfg := ingest.DefaultConfig(world.GeoCoordinates{Latitude: lat, Longitude: lon})
			res, err := ingest.ParseOSM(data, cfg)
			if err != nil {
				return fmt.Errorf("parse: %w", err)
			}

			// Semantic inventory.
			inv := ingest.InventoryOf(res.World)
			formatter.Header(fmt.Sprintf("World inspect — %s", name))
			formatter.Println(inv.String())

			// Validation.
			vr := world.Validate(res.World)
			if !vr.Valid {
				formatter.Warning(fmt.Sprintf("Validation: %d issue(s)", len(vr.Issues)))
			} else {
				formatter.Success("Validation: OK (0 issues)")
			}

			// Fingerprint.
			fp, _ := world.WorldFingerprint(res.World)
			formatter.KeyValue("Fingerprint", fp)

			// Freeze golden slice.
			golden, err := ingest.BuildGolden(name, res.World, inv)
			if err != nil {
				return fmt.Errorf("golden: %w", err)
			}
			goldenJSON, _ := json.MarshalIndent(golden, "", "  ")
			formatter.KeyValue("GoldenSlice", string(goldenJSON))

			return nil
		},
	}

	cmd.Flags().StringVar(&input, "input", "internal/ingest/testdata/palhoca_sample.json", "OSM/Overpass JSON input")
	cmd.Flags().StringVar(&name, "name", "palhoca-1km", "Golden slice name")
	cmd.Flags().Float64Var(&lat, "lat", -27.6375, "Origin latitude")
	cmd.Flags().Float64Var(&lon, "lon", -48.6765, "Origin longitude")
	return cmd
}

// NewWorldSpawnCommand ingests a real OSM dataset and materializes it into
// Unreal via the RendererAdapter + bridge WebSocket. This is the final arrow:
// World Model → reconstruction → Unreal.
func NewWorldSpawnCommand() *cobra.Command {
	var input, url, class string
	var lat, lon float64
	var maxEntities int

	cmd := &cobra.Command{
		Use:   "spawn",
		Short: "Materializar mundo real de OSM no Unreal (World Model → Unreal)",
		Long: `Ingest a real OSM/Overpass JSON extract into the World Model and
materialize it in the Unreal engine via the WebSocket bridge.

Before running, open the UE editor with a map and press Play so the
CoscaRuntime WebSocket server is listening on port 9000.

Example:
  cosca world spawn --input internal/ingest/testdata/palhoca_sample.json \
    --lat -27.6375 --lon -48.6765 --url ws://localhost:9000`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			data, err := os.ReadFile(input)
			if err != nil {
				return fmt.Errorf("read input: %w", err)
			}

			cfg := ingest.DefaultConfig(world.GeoCoordinates{Latitude: lat, Longitude: lon})
			res, err := ingest.ParseOSM(data, cfg)
			if err != nil {
				return fmt.Errorf("parse: %w", err)
			}

			// Connect to the Unreal bridge.
			ctrl := bridge.NewController(bridge.NewWebSocketClient())
			if err := ctrl.Connect(cmd.Context(), url); err != nil {
				return fmt.Errorf("connect to Unreal: %w", err)
			}
			defer ctrl.Disconnect(cmd.Context())

			// Materialize via UnrealAdapter + Reconstructor.
			adapter := adapter.NewUnreal(ctrl, cmd.Context())
			recon := reconstruct.New(adapter)

			opts := reconstruct.Options{MaxEntities: maxEntities}
			if class != "" {
				opts.OnlyClasses = []world.EntityClass{world.EntityClass(class)}
			}

			rec, err := recon.Reconstruct(res.World, opts)
			if err != nil {
				return fmt.Errorf("reconstruct: %w", err)
			}

			formatter.Success(fmt.Sprintf("Materialized %d entities into Unreal", rec.Materialized))
			formatter.KeyValue("Skipped", fmt.Sprintf("%d", rec.Skipped))
			if len(rec.Errors) > 0 {
				formatter.Warning(fmt.Sprintf("%d materialization error(s)", len(rec.Errors)))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&input, "input", "internal/ingest/testdata/palhoca_sample.json", "OSM/Overpass JSON input")
	cmd.Flags().StringVar(&url, "url", "ws://localhost:9000", "WebSocket URL of the Unreal server")
	cmd.Flags().StringVar(&class, "class", "", "Only materialize this entity class (e.g. structure)")
	cmd.Flags().Float64Var(&lat, "lat", -27.6375, "Origin latitude")
	cmd.Flags().Float64Var(&lon, "lon", -48.6765, "Origin longitude")
	cmd.Flags().IntVar(&maxEntities, "max", 0, "Max entities to materialize (0 = all)")
	return cmd
}
