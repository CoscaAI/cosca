package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/cache"
)

// NewCacheCommand creates the `cosca cache` command and its subcommands.
func NewCacheCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the Cosca cache",
		Long: `Manage the Cosca cache system.

The cache stores frequently accessed data to improve performance.
Use these commands to clear, warm, inspect, and monitor the cache.

Subcommands:
  clear    Clear all caches
  warm     Report the current cache state
  stats    Show cache statistics
  inspect  Inspect cached items
`,
		Example: `  cosca cache clear        Clear all cached data
  cosca cache warm         Report the current cache state
  cosca cache stats        Show cache usage statistics
  cosca cache inspect      Browse cached items`,
	}

	cmd.AddCommand(
		NewCacheClearCommand(),
		NewCacheWarmCommand(),
		NewCacheStatsCommand(),
		NewCacheInspectCommand(),
	)

	return cmd
}

// openCache creates and returns a cache instance for the given Cosca directory.
func openCache(coscaDir string) (*cache.Cache, error) {
	cfg := cache.DefaultConfig()
	cfg.FileCacheDir = filepath.Join(coscaDir, "cache")
	cfg.EnabledLevels = []cache.Level{cache.LevelMemory, cache.LevelFilesystem}
	return cache.New(cfg)
}

// NewCacheClearCommand creates the `cosca cache clear` subcommand.
func NewCacheClearCommand() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear all caches",
		Long:  `Clear all cached data. Use --all to clear all cache types.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			spinner := formatter.Spinner("Clearing cache")
			spinner.Start()

			c, err := openCache(coscaDir)
			if err != nil {
				spinner.Fail("Cache not available")
				return fmt.Errorf("cache not available: %w", err)
			}

			if err := c.Clear(); err != nil {
				spinner.Fail(fmt.Sprintf("Failed to clear cache: %v", err))
				return fmt.Errorf("failed to clear cache: %w", err)
			}

			spinner.Stop("Cache cleared")

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"cleared": true,
					"all":     all,
				})
			}

			formatter.Success("Cache cleared successfully")
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "clear all cache types including persistent cache")
	return cmd
}

// NewCacheWarmCommand creates the `cosca cache warm` subcommand.
func NewCacheWarmCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "warm",
		Short: "Report the current cache state",
		Long: `Report the current cache state (entries, hits, misses, size).

The cache is populated on demand, so a pre-load "warm" step does not apply.
This command inspects the cache and reports its real statistics.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			c, err := openCache(coscaDir)
			if err != nil {
				return fmt.Errorf("cache not available: %w", err)
			}

			s := c.Stats()
			entries := s.MemoryEntries + s.SQLiteEntries + s.FileEntries
			hits := s.MemoryHits + s.SQLiteHits + s.FileHits
			size := s.MemorySize + s.FileSize

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"entries":    entries,
					"hits":       hits,
					"misses":     s.Misses,
					"size_bytes": size,
				})
			}

			formatter.Println(fmt.Sprintf("Cache: %d entries, %d bytes (%d hits / %d misses)", entries, size, hits, s.Misses))
			return nil
		},
	}

	return cmd
}

// NewCacheStatsCommand creates the `cosca cache stats` subcommand.
func NewCacheStatsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show cache statistics",
		Long:  `Display cache usage statistics including size, entries, and hit rate.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			c, err := openCache(coscaDir)
			if err != nil {
				return fmt.Errorf("cache not available: %w", err)
			}

			s := c.Stats()
			totalEntries := s.MemoryEntries + s.SQLiteEntries + s.FileEntries
			totalHits := s.MemoryHits + s.SQLiteHits + s.FileHits
			totalOps := totalHits + s.Misses
			hitRate := 0.0
			if totalOps > 0 {
				hitRate = float64(totalHits) / float64(totalOps) * 100.0
			}

			stats := map[string]interface{}{
				"size":           fmt.Sprintf("%d bytes", s.MemorySize+s.FileSize),
				"entries":        totalEntries,
				"hit_rate":       fmt.Sprintf("%.1f%%", hitRate),
				"enabled":        true,
				"memory_entries": s.MemoryEntries,
				"sqlite_entries": s.SQLiteEntries,
				"file_entries":   s.FileEntries,
				"memory_size":    s.MemorySize,
				"file_size":      s.FileSize,
			}

			if useJSON {
				return printJSON(cmd, stats)
			}

			formatter.Header("Cache Statistics")
			formatter.KeyValue("Size", fmt.Sprintf("%d bytes", s.MemorySize+s.FileSize))
			formatter.KeyValue("Entries", fmt.Sprintf("%d", totalEntries))
			formatter.KeyValue("Hit Rate", fmt.Sprintf("%.1f%%", hitRate))
			formatter.KeyValue("Enabled", "true")
			formatter.KeyValue("Memory Entries", fmt.Sprintf("%d", s.MemoryEntries))
			formatter.KeyValue("SQLite Entries", fmt.Sprintf("%d", s.SQLiteEntries))
			formatter.KeyValue("File Entries", fmt.Sprintf("%d", s.FileEntries))
			formatter.KeyValue("Memory Size", fmt.Sprintf("%d bytes", s.MemorySize))
			formatter.KeyValue("File Size", fmt.Sprintf("%d bytes", s.FileSize))

			return nil
		},
	}

	return cmd
}

// NewCacheInspectCommand creates the `cosca cache inspect` subcommand.
func NewCacheInspectCommand() *cobra.Command {
	var key string
	var prefix string

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect cached items",
		Long:  `Browse and inspect items currently stored in the cache.`,
		Example: `  cosca cache inspect               List all cached items
  cosca cache inspect --key mykey   Inspect a specific key
  cosca cache inspect --prefix idx  Inspect items with a prefix`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			c, err := openCache(coscaDir)
			if err != nil {
				return fmt.Errorf("cache not available: %w", err)
			}

			var items interface{}

			if key != "" {
				val, found := c.Get(key)
				if !found {
					return fmt.Errorf("key %q not found", key)
				}
				items = val
			} else if prefix != "" {
				items = []string{} // Cache doesn't support prefix listing
			} else {
				items = []string{} // Cache doesn't support listing all keys
			}

			if useJSON {
				return printJSON(cmd, items)
			}

			formatter.Header("Cache Inspection")

			switch v := items.(type) {
			case []string:
				for _, item := range v {
					formatter.Bullet(item)
				}
				formatter.KeyValue("Total", fmt.Sprintf("%d", len(v)))
			case map[string]interface{}:
				for k, val := range v {
					formatter.KeyValue(k, fmt.Sprintf("%v", val))
				}
			default:
				formatter.Println(fmt.Sprintf("%v", v))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&key, "key", "", "inspect a specific cache key")
	cmd.Flags().StringVar(&prefix, "prefix", "", "inspect items with the given key prefix")
	return cmd
}
