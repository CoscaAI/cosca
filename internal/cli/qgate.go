package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/pipeline"
)

// NewQGateCommand creates the `cosca qgate` command — Quality Gate pre-commit.
func NewQGateCommand() *cobra.Command {
	var skipSecurityVulns bool
	var noTests bool

	cmd := &cobra.Command{
		Use:   "qgate",
		Short: "Pre-commit quality gate — build, test, security, autonomy dashboard",
		Long: `Run the Cosca Quality Gate before committing.

The gate runs:
  • go build — verifies the project compiles
  • go test  — runs all tests and counts pass/fail
  • go vet   — static analysis
  • Security — scans diff for exposed secrets (API keys, tokens, private keys)
  • Deps     — scans go.mod/go.sum (OSV database) for known vulnerabilities
  • Diff     — analyzes changed files, lines, TODO/FIXME in new code
  • Autonomy — loads metrics from pipeline execution history

Hard blockers (commit is denied):
  • Build failure
  • Test failures
  • Secrets exposed in diff
  • Lint/vet errors
  • CRITICAL/HIGH dependency vulnerabilities (skip with --skip-security-vulns)

Exit code: 0 = ready to commit, 1 = blocked.`,
		Example: `  cosca qgate
  cosca qgate --skip-security-vulns   Skip the OSV dependency scan (offline)
  cosca qgate --no-tests              Skip test execution`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			// Load history for autonomy metrics
			var history *pipeline.WorkflowHistory
			historyDir := filepath.Join(coscaDir, "history")
			if h, err := pipeline.NewWorkflowHistory(historyDir); err == nil {
				history = h
			}

			ctx := context.Background()

			if noTests {
				gate := pipeline.NewGate(dir)
				gate.SetHistory(history)
				gate.SetSkipSecurityVulns(skipSecurityVulns)
				gate.SetSkipTests(true)
				result, err := gate.Run(ctx)
				if err != nil {
					return err
				}
				fmt.Print(result.Display())
				if !result.Ready {
					os.Exit(1)
				}
				return nil
			}

			ready := pipeline.RunGate(ctx, dir, history, skipSecurityVulns)

			if !ready {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&noTests, "no-tests", false, "Skip test execution")
	cmd.Flags().BoolVar(&skipSecurityVulns, "skip-security-vulns", false, "Skip the OSV dependency vulnerability scan")
	return cmd
}
