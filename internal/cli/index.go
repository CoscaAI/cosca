package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// NewIndexCommand creates the `cosca index` command and its subcommands.
func NewIndexCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Manage the Cosca index",
		Long: `Manage the Cosca file index.

The index tracks all files in your project for fast search and knowledge retrieval.
It supports incremental updates and full rebuilds.

Subcommands:
  rebuild    Rebuild the entire index from scratch
  update     Incremental update of the index
  status     Show index status
  stats      Show index statistics
  verify     Verify index integrity
`,
		Example: `  cosca index rebuild        Rebuild the index
  cosca index update         Incremental update
  cosca index status         Index status overview
  cosca index stats          Detailed index statistics
  cosca index verify         Verify index integrity`,
	}

	cmd.AddCommand(
		NewIndexRebuildCommand(),
		NewIndexUpdateCommand(),
		NewIndexStatusCommand(),
		NewIndexStatsCommand(),
		NewIndexVerifyCommand(),
	)

	return cmd
}

// NewIndexRebuildCommand creates the `cosca index rebuild` subcommand.
func NewIndexRebuildCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild the entire index",
		Long:  `Rebuild the entire file index from scratch. This scans all project files and re-indexes them.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			spinner := formatter.Spinner("Rebuilding index from scratch")
			spinner.Start()

			idx := NewIndexer(dir)
			startTime := time.Now()

			_, err := idx.Rebuild()
			if err != nil {
				spinner.Fail(fmt.Sprintf("Rebuild failed: %v", err))
				return fmt.Errorf("index rebuild failed: %w", err)
			}
			elapsed := time.Since(startTime)

			spinner.Stop("Index rebuilt")

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"status":   "ok",
					"duration": elapsed.Round(time.Millisecond).String(),
				})
			}

			formatter.Success(fmt.Sprintf("Index rebuilt in %s", elapsed.Round(time.Millisecond)))
			return nil
		},
	}

	return cmd
}

// NewIndexUpdateCommand creates the `cosca index update` subcommand.
func NewIndexUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Incrementally update the index",
		Long:  `Scan for changes since the last index update and apply them incrementally.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			spinner := formatter.Spinner("Updating index incrementally")
			spinner.Start()

			idx := NewIndexer(dir)
			_, err := idx.Update()
			if err != nil {
				spinner.Fail(fmt.Sprintf("Update failed: %v", err))
				return fmt.Errorf("index update failed: %w", err)
			}

			spinner.Stop("Index updated")

			if useJSON {
				return printJSON(cmd, map[string]string{"status": "ok"})
			}

			formatter.Success("Index updated successfully")
			return nil
		},
	}

	return cmd
}

// NewIndexStatusCommand creates the `cosca index status` subcommand.
func NewIndexStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show index status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			idx := NewIndexer(dir)
			status := idx.Status()

			if useJSON {
				return printJSON(cmd, status)
			}

			formatter.Header("Index Status")
			formatter.KeyValue("Files Indexed", fmt.Sprintf("%d", status.FilesIndexed))
			formatter.KeyValue("Last Sync", status.LastSync.Format("2006-01-02 15:04:05"))

			return nil
		},
	}

	return cmd
}

// NewIndexStatsCommand creates the `cosca index stats` subcommand.
func NewIndexStatsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show index statistics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			idx := NewIndexer(dir)
			stats := idx.Stats()

			if useJSON {
				return printJSON(cmd, stats)
			}

			formatter.Header("Index Statistics")
			formatter.KeyValue("Total Files", fmt.Sprintf("%d", stats.TotalFiles))
			formatter.KeyValue("Indexed Files", fmt.Sprintf("%d", stats.IndexedFiles))

			return nil
		},
	}

	return cmd
}

// NewIndexVerifyCommand creates the `cosca index verify` subcommand.
func NewIndexVerifyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify index integrity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			spinner := formatter.Spinner("Verifying index integrity")
			spinner.Start()

			idx := NewIndexer(dir)
			result, err := idx.Verify()
			if err != nil {
				spinner.Fail(fmt.Sprintf("Verification failed: %v", err))
				return fmt.Errorf("index verification failed: %w", err)
			}

			spinner.Stop(fmt.Sprintf("Verification complete: %d issues found", len(result.Issues)))

			if useJSON {
				return printJSON(cmd, result)
			}

			if len(result.Issues) == 0 {
				formatter.Success("Index integrity verified — no issues found")
			} else {
				formatter.Warning(fmt.Sprintf("Index verification found %d issues", len(result.Issues)))
			}

			return nil
		},
	}

	return cmd
}
