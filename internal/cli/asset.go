package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/asset"
	coscaasset "github.com/CoscaAI/cosca/internal/worldmodel/asset"
)

// NewAssetCommand creates the `cosca asset` command group — the Asset Registry
// (§2 do manifesto Creative/Scientific/Media, Fase 1 etapa 1.2).
func NewAssetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "asset",
		Short: "Manage project assets (content-addressable registry)",
		Long: `Manage project assets in the Asset Registry (.cosca/assets/).

The registry is CONTENT-ADDRESSABLE: the sha256 of the content is the asset
identity. The same content is never duplicated — adding a file that already
exists returns the registered asset. Assets are immutable by identity;
derivatives (thumbnails, transcodes) live in derivatives/ and never touch the
original (non-destructive editing, §2 do manifesto).

Subcommands:
  add <file>   Register a file as an asset
  list         List registered assets
  info <id>    Show asset details (use the first 12 chars of the hash)`,
		Example: `  cosca asset add logo.png --type image
  cosca asset add intro.mp4 --type video
  cosca asset list
  cosca asset info 3f8a2c1b`,
	}
	cmd.AddCommand(
		NewAssetAddCommand(),
		NewAssetListCommand(),
		NewAssetInfoCommand(),
		NewAssetGenCommand(),
	)
	return cmd
}

// resolveAssetRegistry opens the registry rooted at the current project. It
// walks up from the working directory to find the .cosca/ project root.
func resolveAssetRegistry() (*asset.Registry, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("getwd: %w", err)
	}
	root, err := findProjectRoot(wd)
	if err != nil {
		return nil, "", err
	}
	r, err := asset.Open(root)
	if err != nil {
		return nil, "", err
	}
	return r, root, nil
}

// findProjectRoot walks up from dir to find a .cosca/ directory (project root).
func findProjectRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, ".cosca")); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("no project root (.cosca/) found from %s", dir)
		}
		abs = parent
	}
}

// NewAssetAddCommand creates `cosca asset add <file>`.
func NewAssetAddCommand() *cobra.Command {
	var assetType string
	var source string

	cmd := &cobra.Command{
		Use:   "add <file>",
		Short: "Register a file as an asset",
		Long: `Register a file as an asset in the project Asset Registry.

The file is hashed (sha256) and stored content-addressable in
.coscra/assets/objects/. The same content added twice is deduplicated —
the registered asset is returned (never duplicated).

The original file is never moved or modified.`,
		Example: `  cosca asset add logo.png --type image
  cosca asset add demo.wav --type audio
  cosca asset add model.glb --type 3d --source "exported from Blender"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			r, root, err := resolveAssetRegistry()
			if err != nil {
				return err
			}

			path, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("file not found: %s", args[0])
			}

			typ := asset.Type(assetType)
			if !typ.Valid() {
				return fmt.Errorf("invalid asset type %q (valid: %s)", assetType, asset.TypesList())
			}

			a, err := r.AddFile(path, typ, source)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, map[string]any{
					"id":           a.ID,
					"type":         a.Type,
					"size":         a.Size,
					"source":       a.Source,
					"deduplicated": a.Source != path && source == "",
					"registry":     filepath.Join(root, asset.DefaultDir),
				})
			}

			formatter.Success(fmt.Sprintf("Asset registered (%s)", a.Type))
			formatter.KeyValue("ID", a.ID)
			formatter.KeyValue("Type", string(a.Type))
			formatter.KeyValue("Size", formatSize(a.Size))
			formatter.KeyValue("Registry", filepath.Join(root, asset.DefaultDir))
			return nil
		},
	}

	cmd.Flags().StringVar(&assetType, "type", "", "Asset type: image, video, audio, 3d, font, text, data, model, material, script, document")
	cmd.Flags().StringVar(&source, "source", "", "Provenance source (§33/§35): where the asset came from")
	return cmd
}

// NewAssetListCommand creates `cosca asset list`.
func NewAssetListCommand() *cobra.Command {
	var byType string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered assets",
		Example: `  cosca asset list
  cosca asset list --type image
  cosca asset list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			r, root, err := resolveAssetRegistry()
			if err != nil {
				return err
			}

			assets := r.List()
			if byType != "" {
				typ := asset.Type(byType)
				if !typ.Valid() {
					return fmt.Errorf("invalid asset type %q (valid: %s)", byType, asset.TypesList())
				}
				var filtered []*asset.Asset
				for _, a := range assets {
					if a.IsType(typ) {
						filtered = append(filtered, a)
					}
				}
				assets = filtered
			}

			if useJSON {
				return printJSON(cmd, assets)
			}

			if len(assets) == 0 {
				formatter.Warning("No assets registered — add one with 'cosca asset add <file> --type <type>'")
				return nil
			}

			formatter.Header(fmt.Sprintf("Assets (%d)", len(assets)))
			formatter.KeyValue("Registry", filepath.Join(root, asset.DefaultDir))
			formatter.Println("")

			rows := make([][]string, 0, len(assets))
			for _, a := range assets {
				rows = append(rows, []string{
					shortID(a.ID),
					string(a.Type),
					formatSize(a.Size),
					a.Source,
				})
			}
			formatter.Table([]string{"ID", "Type", "Size", "Source"}, rows)
			return nil
		},
	}

	cmd.Flags().StringVar(&byType, "type", "", "Filter by asset type")
	return cmd
}

// NewAssetInfoCommand creates `cosca asset info <id>`.
func NewAssetInfoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <id>",
		Short: "Show asset details",
		Long: `Show details of a registered asset.

The <id> is the sha256 content hash. You can use a prefix (first 12 chars)
as long as it uniquely matches one asset.`,
		Example: `  cosca asset info 3f8a2c1b90de`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			r, _, err := resolveAssetRegistry()
			if err != nil {
				return err
			}

			id, err := resolveAssetID(r, args[0])
			if err != nil {
				return err
			}

			a, ok := r.Get(id)
			if !ok {
				return fmt.Errorf("asset %q not found", args[0])
			}

			if useJSON {
				return printJSON(cmd, a)
			}

			formatter.Header(fmt.Sprintf("Asset %s", shortID(a.ID)))
			formatter.KeyValue("ID", a.ID)
			formatter.KeyValue("Type", string(a.Type))
			formatter.KeyValue("Size", formatSize(a.Size))
			formatter.KeyValue("Version", a.Version)
			if a.Source != "" {
				formatter.KeyValue("Source", a.Source)
			}
			formatter.KeyValue("Added", a.AddedAt)
			if len(a.Dependencies) > 0 {
				formatter.KeyValue("Dependencies", strings.Join(a.Dependencies, ", "))
			}
			if a.Preview != "" {
				formatter.KeyValue("Preview", a.Preview)
			}
			if len(a.Derivatives) > 0 {
				formatter.KeyValue("Derivatives", strings.Join(a.Derivatives, ", "))
			}
			return nil
		},
	}
	return cmd
}

// resolveAssetID resolves a full ID from a prefix (minimum 12 chars).
func resolveAssetID(r *asset.Registry, prefix string) (string, error) {
	if len(prefix) >= 64 {
		return prefix, nil
	}
	if len(prefix) < 12 {
		return "", fmt.Errorf("asset id prefix must be at least 12 chars (got %d)", len(prefix))
	}
	var matches []string
	for _, a := range r.List() {
		if strings.HasPrefix(a.ID, prefix) {
			matches = append(matches, a.ID)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("asset %q not found", prefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("prefix %q is ambiguous (%d matches) — use a longer prefix", prefix, len(matches))
	}
}

// NewAssetGenCommand creates `cosca asset gen` — generate a 3D asset via Blender.
//
// This is the Asset/World Pipeline entry point from the CLI: the Don requests
// a procedural asset, Blender generates it (geometry + materials), the result
// is validated, hashed (sha256), and registered in the Asset Registry. The
// resulting GLB can then be spawned in Unreal via the bridge.
func NewAssetGenCommand() *cobra.Command {
	var assetType, format, output string
	var seed int64
	var size float64
	var register bool

	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate a 3D asset procedurally via Blender",
		Long: `Generate a procedural 3D asset using Blender (headless) and register it.

The Asset/World Pipeline:
  Cosca request -> Blender (headless generate.py) -> GLB/FBX
  -> validate -> sha256 hash -> Asset Registry -> Unreal Bridge (spawn)

Supported types: cube, tree, terrain, building.`,
		Example: `  cosca asset gen --type cube --size 2.0 --out /tmp/cube.glb
  cosca asset gen --type tree --seed 42
  cosca asset gen --type terrain --size 100 --register`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			// Build the Blender adapter
			adapter := coscaasset.NewBlenderAdapter(coscaasset.DefaultBlenderAdapterConfig())
			ctx := cmd.Context()

			// Verify Blender is available
			if _, err := adapter.GetVersion(ctx); err != nil {
				return fmt.Errorf("Blender not available: %w\nInstall Blender or add it to PATH.", err)
			}

			// Normalize format before using it
			if format == "" {
				format = string(coscaasset.FormatGLB)
			}

			// Resolve output path (default into temp if not provided)
			out := output
			if out == "" {
				out = filepath.Join(os.TempDir(), fmt.Sprintf("asset_%d.%s", seed, format))
			}

			req := coscaasset.AssetRequest{
				Type:     coscaasset.AssetType(assetType),
				Params:   map[string]any{"size": size},
				Seed:     seed,
				Format:   coscaasset.ExportFormat(format),
				Output:   out,
				Validate: true,
			}

			result, err := adapter.GenerateAsset(ctx, req)
			if err != nil {
				return fmt.Errorf("generate asset: %w", err)
			}

			if useJSON {
				return printJSON(cmd, result)
			}

			formatter.Header("Asset generated via Blender")
			formatter.KeyValue("ID", result.ID)
			formatter.KeyValue("Type", string(result.Type))
			formatter.KeyValue("Path", result.Path)
			formatter.KeyValue("Format", string(result.Format))
			formatter.KeyValue("SHA256", result.Hash)
			formatter.KeyValue("Valid", fmt.Sprintf("%v", result.Valid))
			if len(result.Issues) > 0 {
				formatter.Warning("Validation issues: " + strings.Join(result.Issues, "; "))
			}
			formatter.KeyValue("Latency", result.Latency.String())

			// Optionally register in the Asset Registry
			if register {
				if !result.Valid {
					return fmt.Errorf("asset is invalid — not registering")
				}
				r, _, err := resolveAssetRegistry()
				if err != nil {
					return err
				}
				reg, err := r.AddFile(result.Path, asset.Type3D, "generated by Blender via cosca asset gen")
				if err != nil {
					return err
				}
				formatter.Success(fmt.Sprintf("Registered as %s (%s)", reg.Type, shortID(reg.ID)))
				if useJSON {
					return printJSON(cmd, map[string]any{
						"generated":  result,
						"registered": reg,
					})
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&assetType, "type", "cube", "Asset type: cube, tree, terrain, building")
	cmd.Flags().StringVar(&format, "format", "glb", "Export format: glb, fbx, usd")
	cmd.Flags().StringVar(&output, "out", "", "Output path (default: temp dir)")
	cmd.Flags().Int64Var(&seed, "seed", 42, "Deterministic seed")
	cmd.Flags().Float64Var(&size, "size", 1.0, "Asset size/scale")
	cmd.Flags().BoolVar(&register, "register", false, "Register the asset in the Asset Registry")
	return cmd
}

// shortID devolve os primeiros 12 chars de um hash para exibição.
func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}
