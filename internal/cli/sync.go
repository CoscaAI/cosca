package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// SyncResult holds the result of the sync operation.
type SyncResult struct {
	ProjectDir   string `json:"project_dir" yaml:"project_dir"`
	FullSync     bool   `json:"full_sync" yaml:"full_sync"`
	FilesScanned int    `json:"files_scanned" yaml:"files_scanned"`
	FilesAdded   int    `json:"files_added" yaml:"files_added"`
	FilesUpdated int    `json:"files_updated" yaml:"files_updated"`
	FilesDeleted int    `json:"files_deleted" yaml:"files_deleted"`
	DryRun       bool   `json:"dry_run" yaml:"dry_run"`
	Duration     string `json:"duration" yaml:"duration"`
	Errors       int    `json:"errors" yaml:"errors"`
}

// NewSyncCommand creates the `cosca sync` command.
func NewSyncCommand() *cobra.Command {
	var fullSync bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize the Cosca index with the filesystem",
		Long: `Synchronize the Cosca index with the current state of the filesystem.

This command scans for new, changed, and deleted files and updates:
  - SQLite database (file metadata)
  - Vector store (embeddings for search)
  - Knowledge graph (entity relationships)
  - Cache (file content cache)

Use --full to rebuild everything from scratch.
Use --dry-run to see what would change without making changes.
`,
		Example: `  cosca sync              # Incremental sync
  cosca sync --full       # Full rebuild
  cosca sync --dry-run    # Preview changes
  cosca sync --json       # Machine-readable output`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			coscaDir := filepath.Join(dir, ".cosca")
			startTime := time.Now()
			errorCount := 0

			// Keep the fallback tree in sync with the embedded framework so
			// the knowledge compiler, ORC and cosca-indexer always read the
			// canonical content off disk.
			if !dryRun {
				if err := materializeFallback(dir); err != nil {
					formatter.Warning(fmt.Sprintf("Fallback materialization: %v", err))
					errorCount++
				}
			}

			// Scan files first to get real counts
			var scanner *ScannerAdapter
			if fullSync {
				scanner = NewFullScanner(dir)
			} else {
				scanner = NewIncrementalScanner(dir)
			}

			var scanResult ScannerChange
			if !dryRun {
				spinner := formatter.Spinner("Scanning filesystem")
				spinner.Start()
				scanResult, err = scanner.Scan()
				spinner.Stop("Scan complete")
				if err != nil {
					formatter.Warning(fmt.Sprintf("Scan warning: %v", err))
					errorCount++
				}
			} else {
				scanResult, _ = scanner.Scan()
				// Dry-run: just scan and report
				formatter.Header("Dry Run - Changes that would be made:")
				formatter.KeyValue("Files detected", fmt.Sprintf("%d", scanResult.FilesScanned))
				formatter.KeyValue("Files to add", fmt.Sprintf("%d", scanResult.Added))
				formatter.KeyValue("Files to update", fmt.Sprintf("%d", scanResult.Updated))
				formatter.KeyValue("Files to delete", fmt.Sprintf("%d", scanResult.Deleted))

				elapsed := time.Since(startTime)
				result := SyncResult{
					ProjectDir:   dir,
					FullSync:     fullSync,
					FilesScanned: scanResult.FilesScanned,
					FilesAdded:   scanResult.Added,
					FilesUpdated: scanResult.Updated,
					FilesDeleted: scanResult.Deleted,
					DryRun:       true,
					Duration:     elapsed.Round(time.Millisecond).String(),
					Errors:       errorCount,
				}
				if useJSON {
					return printJSON(cmd, result)
				}
				formatter.Success("Dry run complete - no changes made")
				return nil
			}

			// Use adapter layer for actual synchronization
			idx := NewIndexer(coscaDir)
			if idx != nil {
				spinner := formatter.Spinner("Synchronizing index")
				spinner.Start()
				if fullSync {
					rebuildResult, rebuildErr := idx.Rebuild()
					if rebuildErr != nil {
						formatter.Warning(fmt.Sprintf("Index rebuild error: %v", rebuildErr))
						errorCount++
					} else {
						scanResult.FilesScanned = rebuildResult.FilesIndexed
						scanResult.Added = rebuildResult.FilesIndexed
					}
				} else {
					updateResult, updateErr := idx.Update()
					if updateErr != nil {
						formatter.Warning(fmt.Sprintf("Index update error: %v", updateErr))
						errorCount++
					} else {
						scanResult.Added = updateResult.Added
						scanResult.Updated = updateResult.Changed
						scanResult.Deleted = updateResult.Removed
					}
				}
				spinner.Stop("Index synchronized")
			}

			ke := NewKnowledgeEngine(coscaDir)
			if ke != nil {
				spinner := formatter.Spinner("Updating knowledge base")
				spinner.Start()
				if fullSync {
					if err := ke.Rebuild(); err != nil {
						formatter.Warning(fmt.Sprintf("Knowledge rebuild error: %v", err))
						errorCount++
					}
				}
				spinner.Stop("Knowledge base updated")
			}

			g := NewGraph(coscaDir)
			if g != nil {
				spinner := formatter.Spinner("Building knowledge graph")
				spinner.Start()
				if err := g.Build(); err != nil {
					formatter.Warning(fmt.Sprintf("Graph build error: %v", err))
					errorCount++
				}
				spinner.Stop("Knowledge graph built")
			}

			// Invalidate cache for deleted files
			cacheMgr := NewCacheManager(filepath.Join(coscaDir, "cache"))
			if cacheMgr != nil && len(scanResult.DeletedFiles) > 0 {
				if err := cacheMgr.Invalidate(scanResult.DeletedFiles); err != nil {
					formatter.Warning(fmt.Sprintf("Cache invalidation warning: %v", err))
				}
			}

			elapsed := time.Since(startTime)
			result := SyncResult{
				ProjectDir:   dir,
				FullSync:     fullSync,
				FilesScanned: scanResult.FilesScanned,
				FilesAdded:   scanResult.Added,
				FilesUpdated: scanResult.Updated,
				FilesDeleted: scanResult.Deleted,
				DryRun:       dryRun,
				Duration:     elapsed.Round(time.Millisecond).String(),
				Errors:       errorCount,
			}

			if useJSON {
				return printJSON(cmd, result)
			}

			formatter.Println("")
			formatter.Header("Sync Summary")
			formatter.KeyValue("Project", dir)
			formatter.KeyValue("Files scanned", fmt.Sprintf("%d", result.FilesScanned))
			formatter.KeyValue("Files added", fmt.Sprintf("%d", result.FilesAdded))
			formatter.KeyValue("Files updated", fmt.Sprintf("%d", result.FilesUpdated))
			formatter.KeyValue("Files deleted", fmt.Sprintf("%d", result.FilesDeleted))
			formatter.KeyValue("Duration", result.Duration)
			if errorCount > 0 {
				formatter.Warning(fmt.Sprintf("%d errors occurred during sync", errorCount))
			}
			formatter.Success("Index synchronized successfully")

			return nil
		},
	}

	cmd.Flags().BoolVarP(&fullSync, "full", "f", false, "Perform a full rebuild")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Preview changes without applying them")
	return cmd
}
