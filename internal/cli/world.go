package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/ingest"
	"github.com/CoscaAI/cosca/internal/world"
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
