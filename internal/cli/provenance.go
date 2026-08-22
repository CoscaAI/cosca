package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/provenance"
)

// NewProvenanceCommand creates the `cosca provenance` command group —
// Integridade e Provenance (§32-§35), Fase 1 etapa 1.10.
func NewProvenanceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provenance",
		Short: "Provenance — integridade científica/criativa + licenças (§32-35)",
		Long: `Provenance — o registro de integridade do ecossistema (§32-§35).

  §32 SCIENTIFIC INTEGRITY  claims: afirmações classificadas (observed/
      calculated/simulated/generated/hypothesis) com source, data, method,
      assumptions e confidence — nunca inventar resultados.
  §33 CREATIVE INTEGRITY    generation: model, prompt, seed, parameters,
      source assets, processing e edit history — reproduzível.
  §35 LICENSE/PROVENANCE    license: source (cosca-code/third-party/model/
      user-asset), license, version, modifications, attribution e
      compatibilidade com o núcleo proprietário.

Subcommands:
  claim <id> <statement> --kind <k> [--source] [--confidence]
  generation <asset_id> --model <m> [--prompt] [--seed] [--source-asset]
  license <name> --license <spdx> [--source] [--version] [--compatible]
  show                     Show all provenance records`,
		Example: `  cosca provenance claim c1 "0.92 de precisão" --kind calculated --confidence 0.9
  cosca provenance generation a1b2c3 --model whisper --prompt "transcrever"
  cosca provenance license ffmpeg --license GPL-2.0 --source third-party
  cosca provenance show`,
	}
	cmd.AddCommand(
		NewProvenanceClaimCommand(),
		NewProvenanceGenerationCommand(),
		NewProvenanceLicenseCommand(),
		NewProvenanceShowCommand(),
	)
	return cmd
}

// resolveProvenance abre o registry do projeto atual.
func resolveProvenance() (*provenance.Registry, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("getwd: %w", err)
	}
	root, err := findProjectRoot(wd)
	if err != nil {
		return nil, "", err
	}
	r, err := provenance.Open(root)
	if err != nil {
		return nil, "", err
	}
	return r, root, nil
}

// NewProvenanceClaimCommand creates `cosca provenance claim <id> <statement>`.
func NewProvenanceClaimCommand() *cobra.Command {
	var kind, source, method string
	var confidence float64
	var data []string

	cmd := &cobra.Command{
		Use:     "claim <id> <statement>",
		Short:   "Register a scientific claim (§32)",
		Example: `  cosca provenance claim c1 "0.92 de precisão" --kind calculated --confidence 0.9`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			r, root, err := resolveProvenance()
			if err != nil {
				return err
			}
			c := provenance.Claim{
				ID: args[0], Statement: args[1], Kind: provenance.ClaimKind(kind),
				Source: source, Method: method, Data: data, Confidence: confidence,
			}
			if err := r.AddClaim(c); err != nil {
				return err
			}
			formatter.Success(fmt.Sprintf("Claim %s registered (§32 %s)", args[0], kind))
			formatter.KeyValue("Registry", filepath.Join(root, provenance.DefaultDir, provenance.FileName))
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "observed", "Claim kind: observed, calculated, simulated, generated, hypothesis")
	cmd.Flags().StringVar(&source, "source", "", "Source of the claim")
	cmd.Flags().StringVar(&method, "method", "", "Method used")
	cmd.Flags().StringSliceVar(&data, "data", nil, "Referenced data (asset IDs)")
	cmd.Flags().Float64Var(&confidence, "confidence", 0.5, "Confidence 0.0-1.0")
	return cmd
}

// NewProvenanceGenerationCommand creates `cosca provenance generation <asset>`.
func NewProvenanceGenerationCommand() *cobra.Command {
	var model, prompt string
	var seed int64
	var sourceAssets, processing []string

	cmd := &cobra.Command{
		Use:     "generation <asset_id>",
		Short:   "Register creative generation provenance (§33)",
		Example: `  cosca provenance generation a1b2c3 --model whisper --prompt "transcrever" --seed 42`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			r, root, err := resolveProvenance()
			if err != nil {
				return err
			}
			g := provenance.Generation{
				AssetID: args[0], Model: model, Prompt: prompt, Seed: seed,
				SourceAssets: sourceAssets, Processing: processing,
			}
			if err := r.AddGeneration(g); err != nil {
				return err
			}
			formatter.Success(fmt.Sprintf("Generation provenance registered for asset %s (§33 %s)", args[0], model))
			formatter.KeyValue("Seed", fmt.Sprint(seed))
			formatter.KeyValue("Registry", filepath.Join(root, provenance.DefaultDir, provenance.FileName))
			return nil
		},
	}
	cmd.Flags().StringVar(&model, "model", "", "Model used (Model Registry ID)")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt used")
	cmd.Flags().Int64Var(&seed, "seed", 0, "Determinism seed")
	cmd.Flags().StringSliceVar(&sourceAssets, "source-asset", nil, "Source asset IDs (repeatable)")
	cmd.Flags().StringSliceVar(&processing, "processing", nil, "Pipeline steps applied")
	return cmd
}

// NewProvenanceLicenseCommand creates `cosca provenance license <name>`.
func NewProvenanceLicenseCommand() *cobra.Command {
	var source, version, licenseID, modifications, attribution string
	var compatible bool

	cmd := &cobra.Command{
		Use:   "license <name>",
		Short: "Register a dependency license (§35)",
		Example: `  cosca provenance license ffmpeg --license GPL-2.0 --source third-party
  cosca provenance license whisper --license MIT --source model`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			r, root, err := resolveProvenance()
			if err != nil {
				return err
			}
			l := provenance.LicenseRecord{
				Name: args[0], Source: provenance.LicenseSource(source),
				License: licenseID, Version: version,
				Modifications: modifications, Attribution: attribution,
				Compatible: compatible,
			}
			if err := r.AddLicense(l); err != nil {
				return err
			}
			formatter.Success(fmt.Sprintf("License registered: %s (%s)", args[0], licenseID))
			formatter.KeyValue("Compatible", fmt.Sprint(compatible))
			formatter.KeyValue("Registry", filepath.Join(root, provenance.DefaultDir, provenance.FileName))
			return nil
		},
	}
	cmd.Flags().StringVar(&source, "source", "third-party", "Source: cosca-code, third-party, model, user-asset")
	cmd.Flags().StringVar(&licenseID, "license", "", "License SPDX id (MIT, Apache-2.0, GPL-2.0, AGPL...)")
	cmd.Flags().StringVar(&version, "version", "", "Dependency version")
	cmd.Flags().StringVar(&modifications, "modifications", "", "Modifications made")
	cmd.Flags().StringVar(&attribution, "attribution", "", "Attribution required")
	cmd.Flags().BoolVar(&compatible, "compatible", true, "Compatible with the proprietary core")
	return cmd
}

// NewProvenanceShowCommand creates `cosca provenance show`.
func NewProvenanceShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show all provenance records",
		Example: `  cosca provenance show
  cosca provenance show --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			r, root, err := resolveProvenance()
			if err != nil {
				return err
			}
			if useJSON {
				return printJSON(cmd, map[string]any{
					"claims":      r.SortedClaims(),
					"generations": r.SortedGenerations(),
					"licenses":    r.SortedLicenses(),
				})
			}

			formatter.Header("Provenance Registry")
			formatter.KeyValue("Registry", filepath.Join(root, provenance.DefaultDir, provenance.FileName))

			claims := r.SortedClaims()
			formatter.Println("")
			formatter.Header(fmt.Sprintf("Scientific Claims (§32) — %d", len(claims)))
			for _, c := range claims {
				formatter.Bullet(fmt.Sprintf("%s [%s] conf=%.2f — %s", c.ID, c.Kind, c.Confidence, c.Statement))
			}

			gens := r.SortedGenerations()
			formatter.Println("")
			formatter.Header(fmt.Sprintf("Generations (§33) — %d", len(gens)))
			for _, g := range gens {
				formatter.Bullet(fmt.Sprintf("asset %s via %s (seed=%d)", g.AssetID, g.Model, g.Seed))
			}

			lics := r.SortedLicenses()
			formatter.Println("")
			formatter.Header(fmt.Sprintf("Licenses (§35) — %d", len(lics)))
			for _, l := range lics {
				compat := "✓ compatível"
				if !l.Compatible {
					compat = "✗ INCOMPATÍVEL"
				}
				formatter.Bullet(fmt.Sprintf("%s v%s [%s] %s — %s", l.Name, l.Version, l.License, l.Source, compat))
			}
			return nil
		},
	}
	return cmd
}
