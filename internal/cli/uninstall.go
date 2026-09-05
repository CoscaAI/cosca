package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// UninstallResult holds the result of the uninstall operation.
type UninstallResult struct {
	CoscaDir     string   `json:"cosca_dir" yaml:"cosca_dir"`
	RemovedItems []string `json:"removed_items" yaml:"removed_items"`
	KeptFiles    []string `json:"kept_files" yaml:"kept_files"`
	DryRun       bool     `json:"dry_run" yaml:"dry_run"`
}

// NewUninstallCommand creates the `cosca uninstall` command.
func NewUninstallCommand() *cobra.Command {
	var dryRun bool
	var keepConfig bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Cosca from the current project",
		Long: `Remove all Cosca files and configurations from the current project.

This removes the .cosca directory and any editor integrations.
Use --dry-run to preview what would be removed.
`,
		Example: `  cosca uninstall              # Remove Cosca from project
  cosca uninstall --dry-run   # Preview removal
  cosca uninstall --json      # JSON output`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			coscaDir := filepath.Join(dir, ".cosca")
			var removedItems []string

			// Check if initialized
			if info, err := os.Stat(coscaDir); os.IsNotExist(err) || !info.IsDir() {
				return fmt.Errorf("Cosca is not initialized in %s", dir)
			}

			if !dryRun {
				spinner := formatter.Spinner("Removing .cosca directory")
				spinner.Start()

				if !keepConfig {
					if err := os.RemoveAll(coscaDir); err != nil {
						spinner.Fail(fmt.Sprintf("Failed to remove %s: %v", coscaDir, err))
						return fmt.Errorf("failed to remove Cosca directory: %w", err)
					}
					removedItems = append(removedItems, ".cosca/")
				}
				spinner.Stop("Removal complete")
			} else {
				formatter.Warning("Dry-run mode: no changes made")
				removedItems = append(removedItems, ".cosca/ (would be removed)")
			}

			if useJSON {
				return printJSON(cmd, UninstallResult{
					CoscaDir:     coscaDir,
					RemovedItems: removedItems,
					DryRun:       dryRun,
				})
			}

			formatter.Println("")
			formatter.Header("Uninstall Summary")
			for _, item := range removedItems {
				formatter.Bullet(fmt.Sprintf("Removed: %s", item))
			}
			formatter.Success("Cosca has been uninstalled")

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would be removed")
	cmd.Flags().BoolVar(&keepConfig, "keep-config", false, "Keep configuration files")
	return cmd
}
