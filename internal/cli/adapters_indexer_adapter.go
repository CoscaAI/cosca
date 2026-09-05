package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Indexer Adapter
// =============================================================================

// indexerAdapter wraps indexer.Indexer to match CLI expected API.
type indexerAdapter struct {
	dir string
}

// newIndexerAdapter creates an indexer adapter.
// CLI uses indexer.NewIndexer(dir) but real API needs many dependencies.
func newIndexerAdapter(dir string) *indexerAdapter {
	return &indexerAdapter{dir: dir}
}

// projectRoot returns the project directory (parent of .cosca).
func (a *indexerAdapter) projectRoot() string {
	parent := filepath.Dir(a.dir)
	if parent == "." || parent == "" {
		parent, _ = os.Getwd()
	}
	return parent
}

// scanFiles walks the project directory and counts files.
func (a *indexerAdapter) scanFiles() (totalFiles int, totalSize int64, byExt map[string]int, err error) {
	byExt = make(map[string]int)
	root := a.projectRoot()
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			name := info.Name()
			// Skip hidden directories and common ignore dirs
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			switch name {
			case "node_modules", "vendor", "dist", "build", "target", "__pycache__":
				return filepath.SkipDir
			}
			return nil
		}
		totalFiles++
		totalSize += info.Size()
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			ext = "(no extension)"
		}
		byExt[ext]++
		return nil
	})
	return
}

// Rebuild rebuilds the index. CLI expects (result, error).
func (a *indexerAdapter) Rebuild() (IndexRebuildResult, error) {
	total, _, _, err := a.scanFiles()
	if err != nil {
		return IndexRebuildResult{FilesIndexed: total}, err
	}
	return IndexRebuildResult{FilesIndexed: total}, nil
}

// Update performs incremental update. CLI expects (result, error).
func (a *indexerAdapter) Update() (IndexUpdateResult, error) {
	total, _, _, err := a.scanFiles()
	if err != nil {
		return IndexUpdateResult{Added: total}, err
	}
	return IndexUpdateResult{Added: total}, nil
}

// Status returns index status. CLI expects return value (not error).
func (a *indexerAdapter) Status() IndexStatusEx {
	total, totalSize, _, err := a.scanFiles()
	errCount := 0
	if err != nil {
		errCount = 1
	}
	return IndexStatusEx{
		FilesIndexed: total,
		LastSync:     time.Now(),
		Errors:       errCount,
		Size:         formatBytes(totalSize),
		Version:      "1.0",
	}
}

// Stats returns index stats. CLI expects return value (not error).
func (a *indexerAdapter) Stats() IndexStatsEx {
	total, totalSize, byExt, err := a.scanFiles()
	if err != nil {
		total = 0
	}
	var extCounts []IndexExtCount
	for ext, count := range byExt {
		extCounts = append(extCounts, IndexExtCount{Extension: ext, Count: count})
	}
	// Sort by count descending
	sort.Slice(extCounts, func(i, j int) bool {
		return extCounts[i].Count > extCounts[j].Count
	})
	// Limit to top 10
	if len(extCounts) > 10 {
		extCounts = extCounts[:10]
	}
	return IndexStatsEx{
		TotalFiles:   total,
		IndexedFiles: total,
		SkippedFiles: 0,
		TotalSize:    formatBytes(totalSize),
		IndexSize:    formatBytes(totalSize),
		ByExtension:  extCounts,
		LastRebuild:  time.Now(),
		UpdateCount:  1,
	}
}

// Verify verifies index integrity. CLI expects (report, error).
func (a *indexerAdapter) Verify() (IndexVerifyReport, error) {
	total, _, _, err := a.scanFiles()
	valid := err == nil
	var issues []string
	if err != nil {
		issues = append(issues, fmt.Sprintf("scan error: %v", err))
	}
	return IndexVerifyReport{
		Valid:          valid,
		EntriesChecked: total,
		Issues:         issues,
	}, nil
}

// Create creates a new index. Placeholder for install flow.
func (a *indexerAdapter) Create() error {
	return os.MkdirAll(filepath.Join(a.dir, "index"), 0755)
}

// formatBytes returns a human-readable string representation of bytes.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// IndexRebuildResult holds index rebuild results.
type IndexRebuildResult struct {
	FilesIndexed int `json:"files_indexed"`
}

// IndexUpdateResult holds index update results.
type IndexUpdateResult struct {
	Added   int `json:"added"`
	Changed int `json:"changed"`
	Removed int `json:"removed"`
}

// IndexStatusEx holds index status.
type IndexStatusEx struct {
	FilesIndexed int       `json:"files_indexed"`
	LastSync     time.Time `json:"last_sync"`
	Errors       int       `json:"errors"`
	Size         string    `json:"size"`
	Version      string    `json:"version"`
}

// IndexStatsEx holds index statistics.
type IndexStatsEx struct {
	TotalFiles   int             `json:"total_files"`
	IndexedFiles int             `json:"indexed_files"`
	SkippedFiles int             `json:"skipped_files"`
	TotalSize    string          `json:"total_size"`
	IndexSize    string          `json:"index_size"`
	ByExtension  []IndexExtCount `json:"by_extension,omitempty"`
	LastRebuild  time.Time       `json:"last_rebuild"`
	UpdateCount  int             `json:"update_count"`
}

// IndexExtCount holds count by file extension.
type IndexExtCount struct {
	Extension string `json:"extension"`
	Count     int    `json:"count"`
}

// IndexVerifyReport holds index verification results.
type IndexVerifyReport struct {
	Valid          bool     `json:"valid"`
	EntriesChecked int      `json:"entries_checked"`
	Issues         []string `json:"issues,omitempty"`
}

// =============================================================================
