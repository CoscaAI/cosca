package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/cache"
	"github.com/CoscaAI/cosca/internal/editors"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/pipeline"
	"github.com/CoscaAI/cosca/internal/plugins"
	"github.com/CoscaAI/cosca/internal/providers"
)

// SystemStatus holds the complete system status information.
type SystemStatus struct {
	Version   string           `json:"version" yaml:"version"`
	Project   string           `json:"project" yaml:"project"`
	Runtime   RuntimeStatus    `json:"runtime" yaml:"runtime"`
	Index     IndexStatus      `json:"index" yaml:"index"`
	Knowledge KnowledgeStatus  `json:"knowledge" yaml:"knowledge"`
	Memory    MemoryStatusData `json:"memory" yaml:"memory"`
	Cache     CacheStatusData  `json:"cache" yaml:"cache"`
	Providers ProvidersStatus  `json:"providers" yaml:"providers"`
	Plugins   PluginsStatus    `json:"plugins" yaml:"plugins"`
	Editor    EditorStatus     `json:"editor" yaml:"editor"`
	System    SystemResources  `json:"system" yaml:"system"`
}

// RuntimeStatus holds runtime status information.
type RuntimeStatus struct {
	State   string `json:"state" yaml:"state"`
	Uptime  string `json:"uptime" yaml:"uptime"`
	Version string `json:"version" yaml:"version"`
	Healthy bool   `json:"healthy" yaml:"healthy"`
}

// IndexStatus holds index status information.
type IndexStatus struct {
	FilesIndexed int       `json:"files_indexed" yaml:"files_indexed"`
	LastSync     time.Time `json:"last_sync" yaml:"last_sync"`
	Errors       int       `json:"errors" yaml:"errors"`
	IndexSize    string    `json:"index_size" yaml:"index_size"`
}

// KnowledgeStatus holds knowledge engine status.
type KnowledgeStatus struct {
	Entries      int    `json:"entries" yaml:"entries"`
	GraphSize    int    `json:"graph_size" yaml:"graph_size"`
	VectorsCount int    `json:"vectors_count" yaml:"vectors_count"`
	DatabaseSize string `json:"database_size" yaml:"database_size"`
}

// MemoryStatusData holds memory status.
type MemoryStatusData struct {
	ShortEntries    int `json:"short_entries" yaml:"short_entries"`
	LongEntries     int `json:"long_entries" yaml:"long_entries"`
	ProjectEntries  int `json:"project_entries" yaml:"project_entries"`
	ArchEntries     int `json:"arch_entries" yaml:"arch_entries"`
	DecisionEntries int `json:"decision_entries" yaml:"decision_entries"`
}

// CacheStatusData holds cache status.
type CacheStatusData struct {
	Size    string `json:"size" yaml:"size"`
	Entries int    `json:"entries" yaml:"entries"`
	HitRate string `json:"hit_rate" yaml:"hit_rate"`
	Enabled bool   `json:"enabled" yaml:"enabled"`
}

// ProvidersStatus holds provider status.
type ProvidersStatus struct {
	Active     string   `json:"active" yaml:"active"`
	Available  int      `json:"available" yaml:"available"`
	Configured int      `json:"configured" yaml:"configured"`
	Statuses   []string `json:"statuses,omitempty" yaml:"statuses,omitempty"`
}

// PluginsStatus holds plugin status.
type PluginsStatus struct {
	Installed int      `json:"installed" yaml:"installed"`
	Active    int      `json:"active" yaml:"active"`
	List      []string `json:"list,omitempty" yaml:"list,omitempty"`
}

// EditorStatus holds editor integration status.
type EditorStatus struct {
	Detected   string `json:"detected" yaml:"detected"`
	Integrated bool   `json:"integrated" yaml:"integrated"`
	Version    string `json:"version" yaml:"version"`
}

// SystemResources shows system resource usage.
type SystemResources struct {
	MemoryMB   uint64 `json:"memory_mb" yaml:"memory_mb"`
	CPUCores   int    `json:"cpu_cores" yaml:"cpu_cores"`
	GoRoutines int    `json:"go_routines" yaml:"go_routines"`
	DiskFreeMB uint64 `json:"disk_free_mb" yaml:"disk_free_mb"`
}

// NewStatusCommand creates the `cosca status` command.
func NewStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Display Cosca system status",
		Long: `Display comprehensive status information about the Cosca system.

Shows status for:
  - Runtime daemon (state, uptime, health)
  - Index (files indexed, last sync, errors)
  - Knowledge Engine (entries, graph size, vectors)
  - Memory system (entries by type)
  - Cache (size, entries, hit rate)
  - Providers (active, configured)
  - Plugins (installed, active)
  - Editor integration (detected, version)
  - System resources (memory, CPU, disk)
`,
		Example: `  cosca status           # Show all system status
  cosca status --json    # Output in JSON format`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")
			isInitialized := false
			if info, err := os.Stat(coscaDir); err == nil && info.IsDir() {
				isInitialized = true
			}

			status := SystemStatus{
				Version: Version,
				Project: dir,
			}

			if !isInitialized {
				if useJSON {
					return printJSON(cmd, map[string]interface{}{
						"version":     Version,
						"project":     dir,
						"initialized": false,
						"message":     "Cosca is not initialized. Run 'cosca init' to get started.",
					})
				}
				formatter.Warning("Cosca is not initialized in " + dir)
				formatter.Println("Run 'cosca init' to get started.")
				return nil
			}

			// Gather runtime status from the REAL daemon (PID file + process
			// liveness), not a fresh in-memory runtime instance (which would
			// always report "uninitialized" since it's never started). Same
			// logic as doctor's checkRuntime.
			if pid, err := readDaemonPID(filepath.Join(coscaDir, "cosca.pid")); err == nil && pid > 0 && processAlive(pid) {
				status.Runtime = RuntimeStatus{
					State:   "running",
					Uptime:  processUptime(pid).Round(time.Second).String(),
					Version: Version,
					Healthy: true,
				}
			} else {
				status.Runtime = RuntimeStatus{
					State:   "stopped",
					Uptime:  "0s",
					Version: Version,
					Healthy: false,
				}
			}

			// Gather knowledge / index status via knowledge engine
			keCfg := knowledge.DefaultConfig()
			keCfg.DBPath = filepath.Join(coscaDir, "knowledge.db")
			keCfg.RootDir = dir
			ke, err := knowledge.New(keCfg)
			if err == nil {
				// Initialize to open DB and get stats
				if initErr := ke.Init(); initErr == nil {
					stats, statsErr := ke.GetStats()
					if statsErr == nil {
						status.Knowledge = KnowledgeStatus{
							Entries:      stats.DocumentCount + stats.ChunkCount,
							GraphSize:    stats.GraphStats.Nodes,
							VectorsCount: stats.VectorCount,
							DatabaseSize: fmt.Sprintf("%d bytes", stats.DBSize),
						}
						status.Index = IndexStatus{
							FilesIndexed: stats.DocumentCount,
							LastSync:     stats.LastIndexed,
							Errors:       stats.IndexStats.TotalErrors,
							IndexSize:    fmt.Sprintf("%d", stats.IndexStats.TotalDocuments),
						}
					}
					if closeErr := ke.Close(); closeErr != nil {
						_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", closeErr)
					}
				}
			}

			// Gather memory status
			memCfg := memory.EngineConfig{
				DataDir: coscaDir,
			}
			mem, memErr := memory.NewEngine(memory.WithConfig(memCfg))
			if memErr == nil && mem != nil {
				defer func() { _ = mem.Close() }()
				layerStats := mem.GetLayerStats(cmd.Context())
				for layer, ls := range layerStats {
					switch layer {
					case memory.LayerSession:
						status.Memory.ShortEntries = ls.Count
					case memory.LayerGlobal:
						status.Memory.LongEntries = ls.Count
					case memory.LayerProject:
						status.Memory.ProjectEntries = ls.Count
					case memory.LayerWorkspace:
						status.Memory.ArchEntries = ls.Count
					}
					// Count decision-type records across all layers
				}
			}

			// Gather cache status
			cacheCfg := cache.DefaultConfig()
			cacheCfg.FileCacheDir = filepath.Join(coscaDir, "cache")
			c, cacheErr := cache.New(cacheCfg)
			if cacheErr == nil && c != nil {
				cStats := c.Stats()
				totalEntries := cStats.MemoryEntries + cStats.SQLiteEntries + cStats.FileEntries
				totalHits := cStats.MemoryHits + cStats.SQLiteHits + cStats.FileHits
				totalOps := totalHits + cStats.Misses
				hitRate := 0.0
				if totalOps > 0 {
					hitRate = float64(totalHits) / float64(totalOps) * 100.0
				}
				status.Cache = CacheStatusData{
					Size:    fmt.Sprintf("%d bytes", cStats.MemorySize+cStats.FileSize),
					Entries: totalEntries,
					HitRate: fmt.Sprintf("%.1f%%", hitRate),
					Enabled: true,
				}
			}

			// Gather provider status
			p := providers.NewManager()
			if p != nil {
				pStatus := p.Status()
				// providers.NewManager() não carrega o config, então m.active fica
				// sempre vazio. Reporte o provider primário declarado no config
				// (provider.name) como ativo quando estiver configurado — mesma
				// lógica do doctor.
				active := pStatus.Active
				if active == "" {
					active = activeProviderFromConfigDoctor(pStatus.Statuses)
				}
				status.Providers = ProvidersStatus{
					Active:     active,
					Available:  pStatus.Available,
					Configured: pStatus.Configured,
					Statuses:   pStatus.Statuses,
				}
			}

			// Gather plugin status
			pmCfg := plugins.DefaultManagerConfig(coscaDir)
			pm := plugins.NewManager(pmCfg, nil)
			if pm != nil {
				plList := pm.List()
				active := 0
				names := make([]string, 0, len(plList))
				for _, pi := range plList {
					names = append(names, pi.Manifest.ID)
					if pi.Enabled {
						active++
					}
				}
				status.Plugins = PluginsStatus{
					Installed: len(plList),
					Active:    active,
					List:      names,
				}
			}

			// Gather editor status
			editorCfg := editors.DefaultEditorConfig(dir)
			ed := editors.NewManager(editorCfg)
			if ed != nil {
				editorInfo, detErr := ed.Detect()
				detected := ""
				integrated := false
				if detErr == nil {
					detected = editorInfo.Name
					integrated = editorInfo.SetupComplete
				}
				status.Editor = EditorStatus{
					Detected:   detected,
					Integrated: integrated,
					Version:    "",
				}
			}

			// Gather system resources
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			diskFree, _ := getDiskFree(dir)
			status.System = SystemResources{
				MemoryMB:   memStats.Alloc / 1024 / 1024,
				CPUCores:   runtime.NumCPU(),
				GoRoutines: runtime.NumGoroutine(),
				DiskFreeMB: diskFree,
			}

			if useJSON {
				return printJSON(cmd, status)
			}

			// Text output
			formatter.Println("")
			formatter.Header("Cosca System Status")

			// ── Compact summary ─────────────────────────────────────
			healthIcon := "🟢"
			if !status.Runtime.Healthy {
				healthIcon = "🔴"
			}
			fmt.Fprintf(formatter.writer, "  %s  Runtime: %s | Knowledge: %d entries | Vectors: %d | Provider: %s\n\n",
				healthIcon, status.Runtime.State, status.Knowledge.Entries,
				status.Knowledge.VectorsCount, status.Providers.Active)

			// ── Pipeline Analytics (from event history) ────────────
			historyDir := filepath.Join(coscaDir, "history")
			if hist, histErr := pipeline.NewWorkflowHistory(historyDir); histErr == nil {
				store := pipeline.NewAnalyticsStore(hist)
				plans, _ := hist.ListPlans()
				if len(plans) > 0 {
					agg := &pipeline.AggregateAnalytics{}
					for _, pid := range plans {
						a, loadErr := store.Load(pid)
						if loadErr == nil {
							agg.Add(a)
						}
					}
					bar := strings.Repeat("█", int(agg.AvgAutonomyScore*10)) + strings.Repeat("░", 10-int(agg.AvgAutonomyScore*10))
					fmt.Fprintf(formatter.writer, "  📊 Pipeline: %d plans | %d/%d tasks | Autonomy %s %.0f%%\n\n",
						agg.TotalPlans, agg.TasksDone, agg.TotalTasks, bar, agg.AvgAutonomyScore*100)
				}
			}

			// ── Pipeline Plugins ───────────────────────────────────
			reg := pipeline.NewPluginRegistry()
			var cp *pipeline.CheckpointStore
			cpDir := filepath.Join(historyDir, "checkpoints")
			if s, err := pipeline.NewCheckpointStore(cpDir); err == nil {
				cp = s
			}
			pipeline.RegisterBuiltinPlugins(reg, cp)
			pluginNames := reg.List()
			fmt.Fprintf(formatter.writer, "  🔌 Plugins: %s\n\n", strings.Join(pluginNames, ", "))

			formatter.KeyValue("Version", status.Version)
			formatter.KeyValue("Project", status.Project)

			// Runtime
			formatter.Println("")
			formatter.Header("Runtime")
			healthColor := formatter.colors.Green
			if !status.Runtime.Healthy {
				healthColor = formatter.colors.Red
			}
			_, _ = fmt.Fprintf(formatter.writer, "  %sState%s:       %s\n", formatter.colors.Cyan, formatter.colors.Reset, status.Runtime.State)
			_, _ = fmt.Fprintf(formatter.writer, "  %sUptime%s:     %s\n", formatter.colors.Cyan, formatter.colors.Reset, status.Runtime.Uptime)
			_, _ = fmt.Fprintf(formatter.writer, "  %sHealthy%s:   %s%v%s\n", formatter.colors.Cyan, formatter.colors.Reset, healthColor, status.Runtime.Healthy, formatter.colors.Reset)

			// Index
			formatter.Println("")
			formatter.Header("Index")
			formatter.KeyValue("Files Indexed", fmt.Sprintf("%d", status.Index.FilesIndexed))
			formatter.KeyValue("Last Sync", status.Index.LastSync.Format("2006-01-02 15:04:05"))
			formatter.KeyValue("Errors", fmt.Sprintf("%d", status.Index.Errors))
			formatter.KeyValue("Size", status.Index.IndexSize)

			// Knowledge
			formatter.Println("")
			formatter.Header("Knowledge Engine")
			formatter.KeyValue("Entries", fmt.Sprintf("%d", status.Knowledge.Entries))
			formatter.KeyValue("Graph Nodes", fmt.Sprintf("%d", status.Knowledge.GraphSize))
			formatter.KeyValue("Vectors", fmt.Sprintf("%d", status.Knowledge.VectorsCount))
			formatter.KeyValue("Database", status.Knowledge.DatabaseSize)

			// Memory
			formatter.Println("")
			formatter.Header("Memory")
			formatter.KeyValue("Short-term", fmt.Sprintf("%d entries", status.Memory.ShortEntries))
			formatter.KeyValue("Long-term", fmt.Sprintf("%d entries", status.Memory.LongEntries))
			formatter.KeyValue("Project", fmt.Sprintf("%d entries", status.Memory.ProjectEntries))
			formatter.KeyValue("Architecture", fmt.Sprintf("%d entries", status.Memory.ArchEntries))
			formatter.KeyValue("Decisions", fmt.Sprintf("%d entries", status.Memory.DecisionEntries))

			// Cache
			formatter.Println("")
			formatter.Header("Cache")
			formatter.KeyValue("Size", status.Cache.Size)
			formatter.KeyValue("Entries", fmt.Sprintf("%d", status.Cache.Entries))
			formatter.KeyValue("Hit Rate", status.Cache.HitRate)
			formatter.KeyValue("Enabled", fmt.Sprintf("%v", status.Cache.Enabled))

			// Providers
			formatter.Println("")
			formatter.Header("Providers")
			formatter.KeyValue("Active", status.Providers.Active)
			formatter.KeyValue("Available", fmt.Sprintf("%d", status.Providers.Available))
			formatter.KeyValue("Configured", fmt.Sprintf("%d", status.Providers.Configured))

			// Plugins
			formatter.Println("")
			formatter.Header("Plugins")
			formatter.KeyValue("Installed", fmt.Sprintf("%d", status.Plugins.Installed))
			formatter.KeyValue("Active", fmt.Sprintf("%d", status.Plugins.Active))

			// Editor
			formatter.Println("")
			formatter.Header("Editor Integration")
			formatter.KeyValue("Detected", status.Editor.Detected)
			formatter.KeyValue("Integrated", fmt.Sprintf("%v", status.Editor.Integrated))
			formatter.KeyValue("Version", status.Editor.Version)

			// System
			formatter.Println("")
			formatter.Header("System Resources")
			formatter.KeyValue("Memory Usage", fmt.Sprintf("%d MB", status.System.MemoryMB))
			formatter.KeyValue("CPU Cores", fmt.Sprintf("%d", status.System.CPUCores))
			formatter.KeyValue("Go Routines", fmt.Sprintf("%d", status.System.GoRoutines))
			formatter.KeyValue("Disk Free", fmt.Sprintf("%d MB", status.System.DiskFreeMB))

			return nil
		},
	}

	return cmd
}

// getDiskFree returns the free disk space in megabytes for the given path.
// Implementação platform-specific: diskfree_unix.go (syscall.Statfs) e
// diskfree_windows.go (GetDiskFreeSpaceEx).

// processUptime is implemented per-platform:
//   - status_unix.go: reads /proc/<pid>/stat + /proc/uptime
//   - status_windows.go: stub (returns 0)
