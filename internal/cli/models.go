package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/models"
)

// modelsCachePath returns the sudo-aware default cache location (the CLI can
// run under sudo via the auto-jail, so the real user's home is resolved by
// config.UserHomeDir instead of the sudo-polluted HOME).
func modelsCachePath() string {
	return models.CachePathIn(filepath.Join(config.UserHomeDir(), ".config", "cosca"))
}

// NewModelsCommand creates the `cosca models` command and its subcommands.
func NewModelsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Manage the models.dev catalog cache",
		Long: `Manage the synced models.dev catalog (real context windows and pricing).

The catalog is cached locally so Cosca uses REAL per-model context_length and
pricing instead of hardcoded defaults. The cache is an enhancement: if it is
missing or stale, Cosca falls back to its built-in defaults.

Subcommands:
  sync    Fetch the models.dev catalog into the local cache
  list    List cached models (optionally filtered by name/id)
  show    Show details for one cached model
  info    Show cache status
`,
		Example: `  cosca models sync
  cosca models list qwen
  cosca models show deepseek/deepseek-v4-flash
  cosca models info`,
	}

	cmd.AddCommand(
		NewModelsSyncCommand(),
		NewModelsListCommand(),
		NewModelsShowCommand(),
		NewModelsInfoCommand(),
	)

	return cmd
}

// NewModelsSyncCommand creates the `cosca models sync` subcommand.
func NewModelsSyncCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Fetch the models.dev catalog into the local cache",
		Long: `Download the full models.dev catalog (anomalyco, MIT, open-source)
and store it at the local cache path, atomically and owner-only.

No authentication is required — the models.dev api.json endpoint is public.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			cachePath := modelsCachePath()

			spinner := formatter.Spinner("Syncing models.dev catalog")
			spinner.Start()
			cache, err := models.SyncFromAPI(cmd.Context(), models.DefaultAPIURL, cachePath)
			if err != nil {
				spinner.Fail("models.dev sync failed")
				return fmt.Errorf("models sync: %w", err)
			}
			spinner.Stop("models.dev catalog synced")

			size := cacheFileSize(cachePath)
			providers := uniqueProviders(cache)

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"path":       cachePath,
					"models":     len(cache.Models),
					"providers":  len(providers),
					"size_bytes": size,
					"updated_at": cache.UpdatedAt.Format(time.RFC3339),
				})
			}

			formatter.Header("models.dev catalog synced")
			formatter.KeyValue("Models", fmt.Sprintf("%d", len(cache.Models)))
			formatter.KeyValue("Providers", fmt.Sprintf("%d", len(providers)))
			formatter.KeyValue("Size", fmt.Sprintf("%d bytes", size))
			formatter.KeyValue("Path", cachePath)
			return nil
		},
	}
}

// NewModelsListCommand creates the `cosca models list` subcommand.
func NewModelsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list [query]",
		Short: "List cached models",
		Long: `List the models currently in the local cache, sorted by ID.
An optional query filters results to models whose ID or name contains it.`,
		Example: `  cosca models list
  cosca models list qwen
  cosca models list deepseek-v4`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			cache, err := models.LoadCache(modelsCachePath())
			if err != nil {
				return fmt.Errorf("models cache: %w", err)
			}
			if len(cache.Models) == 0 {
				if useJSON {
					return printJSON(cmd, map[string]interface{}{"models": []models.Model{}, "cached": false})
				}
				formatter.Warning("No models cached — run `cosca models sync` first")
				return nil
			}

			query := ""
			if len(args) > 0 {
				query = strings.ToLower(strings.TrimSpace(args[0]))
			}

			rows := make([]models.Model, 0, len(cache.Models))
			for _, m := range cache.Models {
				if query != "" {
					haystack := strings.ToLower(m.ID + " " + m.Name)
					if !strings.Contains(haystack, query) {
						continue
					}
				}
				rows = append(rows, m)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"models":  rows,
					"total":   len(rows),
					"cached":  true,
					"updated": cache.UpdatedAt.Format(time.RFC3339),
				})
			}

			if len(rows) == 0 {
				formatter.Warning(fmt.Sprintf("No cached models match %q", args[0]))
				return nil
			}

			formatter.Header("Cached Models")
			table := make([][]string, 0, len(rows))
			for _, m := range rows {
				ctx := "unknown"
				if m.ContextLength > 0 {
					ctx = fmt.Sprintf("%d", m.ContextLength)
				}
				table = append(table, []string{m.ID, m.Name, ctx, m.Provider})
			}
			formatter.Table([]string{"ID", "Name", "Context", "Provider"}, table)
			formatter.KeyValue("Total", fmt.Sprintf("%d", len(rows)))
			return nil
		},
	}
}

// NewModelsShowCommand creates the `cosca models show` subcommand.
func NewModelsShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <model-id>",
		Short: "Show details for a cached model",
		Long: `Show context window, modalities, pricing, and supported parameters
for a single cached model. The argument may be a full ID
(deepseek/deepseek-v4-flash) or a bare model name (deepseek-v4-flash).`,
		Example: `  cosca models show deepseek/deepseek-v4-flash
  cosca models show qwen2.5-coder:14b-128k`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			cachePath := modelsCachePath()
			m, ok := models.Lookup(cachePath, args[0])
			if !ok {
				if useJSON {
					return printJSON(cmd, map[string]interface{}{"found": false, "query": args[0]})
				}
				formatter.Warning(fmt.Sprintf("Model %q not found in cache — run `cosca models sync` or check the ID", args[0]))
				return nil
			}

			if useJSON {
				return printJSON(cmd, m)
			}

			formatter.Header(m.ID)
			if m.Name != "" {
				formatter.KeyValue("Name", m.Name)
			}
			formatter.KeyValue("Provider", m.Provider)
			ctx := "unknown"
			if m.ContextLength > 0 {
				ctx = fmt.Sprintf("%d", m.ContextLength)
			}
			formatter.KeyValue("Context Length", ctx)
			formatter.KeyValue("Input Modalities", strings.Join(m.InputModalities, ", "))
			formatter.KeyValue("Output Modalities", strings.Join(m.OutputModalities, ", "))
			formatter.KeyValue("Prompt ($/1M)", fmt.Sprintf("%g", m.Pricing.Prompt))
			formatter.KeyValue("Completion ($/1M)", fmt.Sprintf("%g", m.Pricing.Completion))
			formatter.KeyValue("Cache Read ($/1M)", fmt.Sprintf("%g", m.Pricing.InputCacheRead))
			formatter.KeyValue("Cache Write ($/1M)", fmt.Sprintf("%g", m.Pricing.InputCacheWrite))
			formatter.KeyValue("Supported Parameters", strings.Join(m.SupportedParameters, ", "))
			return nil
		},
	}
}

// NewModelsInfoCommand creates the `cosca models info` subcommand.
func NewModelsInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show models cache status",
		Long: `Show whether the models cache exists, when it was last synced,
how many models it holds, and where it lives on disk.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			cachePath := modelsCachePath()
			cache, err := models.LoadCache(cachePath)
			if err != nil {
				return fmt.Errorf("models cache: %w", err)
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"path":        cachePath,
					"exists":      cacheExists(cachePath),
					"models":      len(cache.Models),
					"size_bytes":  cacheFileSize(cachePath),
					"updated_at":  cache.UpdatedAt.Format(time.RFC3339),
					"default_url": models.DefaultAPIURL,
				})
			}

			formatter.Header("Models Cache")
			formatter.KeyValue("Path", cachePath)
			formatter.KeyValue("Cached", fmt.Sprintf("%t", cacheExists(cachePath)))
			formatter.KeyValue("Models", fmt.Sprintf("%d", len(cache.Models)))
			formatter.KeyValue("Size", fmt.Sprintf("%d bytes", cacheFileSize(cachePath)))
			if !cache.UpdatedAt.IsZero() {
				formatter.KeyValue("Last Sync", cache.UpdatedAt.Format(time.RFC3339))
			}
			if len(cache.Models) == 0 {
				formatter.Warning("Cache is empty — run `cosca models sync` first")
			}
			return nil
		},
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// uniqueProviders returns the sorted set of provider names across a cache.
func uniqueProviders(c *models.Cache) []string {
	set := make(map[string]struct{})
	for _, m := range c.Models {
		if m.Provider == "" {
			continue
		}
		set[m.Provider] = struct{}{}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// cacheFileSize returns the on-disk size of the cache file (0 when absent).
func cacheFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// cacheExists reports whether the cache file exists on disk.
func cacheExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
