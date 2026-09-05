package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/license"
)

// NewLicenseCommand creates the `cosca license` command tree.
//
// The license is the CoscaAI security key (see LICENSE Section 2): the
// official repository https://github.com/CoscaAI. Authenticity is verified
// by three cumulative factors (module / manifest / build).
func NewLicenseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "license",
		Short: "Show and verify the Cosca license (chave de segurança)",
		Long: `Show and verify the Cosca license — the CoscaAI security key.

The security key that authenticates Cosca is the official CoscaAI
repository (https://github.com/CoscaAI). Authenticity is verified by three
cumulative factors:

  F1 module   go.mod declares EXACTLY "github.com/CoscaAI/cosca"
  F2 manifest .cosca/manifest.yaml declares identity "cosca-kernel" and
              module "github.com/CoscaAI/cosca"
  F3 build    the binary embeds a traceable version + commit hash

A software that fails any factor is NOT authenticated.

Subcommands:
  show     Display the license text
  verify   Verify the security key (the 3 factors) in the current project`,
		RunE: runLicenseShow,
	}

	cmd.AddCommand(
		newLicenseShowCommand(),
		newLicenseVerifyCommand(),
	)
	return cmd
}

// newLicenseShowCommand creates the `cosca license show` command.
func newLicenseShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display the Cosca license text",
		Long: `Display the Cosca license (LICENSE).

Uses .cosca/framework/LICENSE when present, otherwise LICENSE from
the current directory.`,
		Example: `  cosca license show
  cosca license show --json`,
		Args: cobra.NoArgs,
		RunE: runLicenseShow,
	}
}

// runLicenseShow prints the license text (shared by `license` and
// `license show`).
func runLicenseShow(cmd *cobra.Command, _ []string) error {
	useJSON := IsJSONOutput(cmd)

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Prefer the synced framework copy, fall back to the project root.
	candidates := []string{
		filepath.Join(dir, ".cosca", "framework", "LICENSE"),
		filepath.Join(dir, "LICENSE"),
	}
	var (
		content []byte
		source  string
	)
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			content = data
			source = candidate
			break
		}
	}
	if content == nil {
		return fmt.Errorf("LICENSE não encontrada em %s", dir)
	}

	if useJSON {
		return printJSON(cmd, map[string]string{
			"file":    source,
			"content": string(content),
		})
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(content))
	return nil
}

// newLicenseVerifyCommand creates the `cosca license verify` command.
func newLicenseVerifyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Verify the license security key (3 factors)",
		Long: `Verify the Cosca license security key in the current project.

Checks the three cumulative authenticity factors defined in LICENSE:
F1 (Go module), F2 (.cosca/manifest.yaml identity) and F3 (build metadata).
A software that fails any factor is NOT authenticated.`,
		Example: `  cosca license verify
  cosca license verify --json`,
		Args: cobra.NoArgs,
		RunE: runLicenseVerify,
	}
}

// runLicenseVerify runs the security-key verification and prints the
// factor-by-factor table.
func runLicenseVerify(cmd *cobra.Command, _ []string) error {
	formatter := GetFormatter(cmd)
	useJSON := IsJSONOutput(cmd)

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	status, err := license.VerifyAuthenticity(dir)
	if err != nil {
		return fmt.Errorf("license verification failed: %w", err)
	}

	if useJSON {
		return printJSON(cmd, status)
	}

	formatter.Header("Chave de Segurança — Verificação de Autenticidade")
	formatter.Table([]string{"Fator", "Status"}, [][]string{
		{"F1 módulo Go", checkMark(status.Factors[license.FactorModule])},
		{"F2 manifesto", checkMark(status.Factors[license.FactorManifest])},
		{"F3 build", checkMark(status.Factors[license.FactorBuild])},
	})

	if status.Authenticated {
		formatter.KeyValue("Authenticated", "sim")
		formatter.Success("Chave de segurança válida — origem confirmada em https://github.com/CoscaAI")
	} else {
		formatter.KeyValue("Authenticated", "não")
		formatter.Warning("Chave de segurança inválida — software não autenticado")
		formatter.KeyValue("Detail", status.Detail)
	}
	return nil
}

// checkMark renders ✓/✗ for a factor status.
func checkMark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}
