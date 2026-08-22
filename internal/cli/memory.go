package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/spf13/cobra"
)

// NewMemoryCommand creates the `cosca memory` command and its subcommands.
func NewMemoryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage Cosca memory",
		Long: `Manage the Cosca memory system.

Memory stores information across sessions including short-term (session),
long-term (cross-session), project, architecture, and decision records.

Subcommands:
  register                      Registrar um aprendizado (fluxo automático)
  list                          List memory records
  show <id>                     Show a memory record
  search <query>                Search memory
  snapshot create               Create a memory snapshot
  snapshot list                 List snapshots
  snapshot restore <id>         Restore a snapshot
  prune                         Prune expired memory
  reindex                       Rebuild the search index from disk
  stats                         Memory statistics
  curated-failures              Curated failure lessons (P5, sanitized)
`,
		Example: `  cosca memory register --title "..." --level 4 --tags "#a #b" --task "..." --learned "..."
  cosca memory list
  cosca memory show mem-abc123
  cosca memory search "database schema"
  cosca memory snapshot create
  cosca memory snapshot list
  cosca memory snapshot restore snap-xyz789
  cosca memory prune
  cosca memory reindex
  cosca memory stats
  cosca memory curated-failures`,
	}

	cmd.AddCommand(
		NewMemoryIntegrityCommand(),
		NewMemoryGuardCommand(),
		NewMemoryRegisterCommand(),
		NewMemoryWatchCommand(),
		NewMemoryListCommand(),
		NewMemoryShowCommand(),
		NewMemorySearchCommand(),
		NewMemorySnapshotCommand(),
		NewMemoryPruneCommand(),
		NewMemoryPromoteCommand(),
		NewMemoryReindexCommand(),
		NewMemoryStatsCommand(),
		NewMemoryCuratedFailuresCommand(),
	)

	return cmd
}

// NewMemoryListCommand creates the `cosca memory list` subcommand.
func NewMemoryListCommand() *cobra.Command {
	var memoryType string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List memory records",
		Long:  `List memory records optionally filtered by type and limited in count.`,
		Example: `  cosca memory list
  cosca memory list --type short
  cosca memory list --type long --limit 50`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := NewMemoryManager(coscaDir)
			defer func() { _ = mgr.Close() }()
			if mgr == nil {
				return fmt.Errorf("memory manager not available")
			}

			records, err := mgr.List(memoryType, limit)
			if err != nil {
				return fmt.Errorf("failed to list memory: %w", err)
			}

			if useJSON {
				return printJSON(cmd, records)
			}

			formatter.Header(fmt.Sprintf("Memory Records (%s)", ifEmpty(memoryType, "all")))
			for _, record := range records {
				formatter.KeyValue(record.ID, record.Title)
				formatter.KeyValue("  Type", record.Type)
				formatter.KeyValue("  Timestamp", record.Timestamp.Format(time.RFC3339))
				formatter.KeyValue("  Tags", fmt.Sprintf("%v", record.Tags))
				formatter.Println("")
			}
			formatter.KeyValue("Total", fmt.Sprintf("%d", len(records)))

			return nil
		},
	}

	cmd.Flags().StringVarP(&memoryType, "type", "t", "", "filter by memory type (short, long, project, arch, decision)")
	cmd.Flags().IntVarP(&limit, "limit", "l", 20, "maximum number of records")
	return cmd
}

// NewMemoryShowCommand creates the `cosca memory show` subcommand.
func NewMemoryShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a memory record",
		Long:  `Display the full details of a specific memory record by its ID.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := NewMemoryManager(coscaDir)
			defer func() { _ = mgr.Close() }()
			if mgr == nil {
				return fmt.Errorf("memory manager not available")
			}

			record, err := mgr.Get(args[0])
			if err != nil {
				return fmt.Errorf("memory record %q not found: %w", args[0], err)
			}

			if useJSON {
				return printJSON(cmd, record)
			}

			formatter.Header(fmt.Sprintf("Memory Record: %s", record.ID))
			formatter.KeyValue("Title", record.Title)
			formatter.KeyValue("Type", record.Type)
			formatter.KeyValue("Status", record.Status)
			formatter.KeyValue("Timestamp", record.Timestamp.Format(time.RFC3339))
			formatter.KeyValue("Tags", fmt.Sprintf("%v", record.Tags))
			formatter.KeyValue("Related", fmt.Sprintf("%v", record.Related))

			if record.Content != "" {
				formatter.Println("")
				formatter.Header("Content")
				formatter.Println(record.Content)
			}

			return nil
		},
	}

	return cmd
}

// NewMemorySearchCommand creates the `cosca memory search` subcommand.
func NewMemorySearchCommand() *cobra.Command {
	var limit int
	var memoryType string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search memory records",
		Long:  `Search across all memory stores for records matching the query.`,
		Example: `  cosca memory search "database schema"
  cosca memory search --type short "current task"
  cosca memory search --limit 5 "architecture decision"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := NewMemoryManager(coscaDir)
			defer func() { _ = mgr.Close() }()
			if mgr == nil {
				return fmt.Errorf("memory manager not available")
			}

			opts := MemorySearchOptions{
				Query: args[0],
				Type:  memoryType,
				Limit: limit,
			}

			results, err := mgr.Search(opts)
			if err != nil {
				return fmt.Errorf("memory search failed: %w", err)
			}

			if useJSON {
				return printJSON(cmd, results)
			}

			formatter.Header(fmt.Sprintf("Memory Search Results for %q", args[0]))
			for i, r := range results {
				formatter.KeyValue(fmt.Sprintf("#%d", i+1), r.Title)
				formatter.KeyValue("  Type", r.Type)
				formatter.KeyValue("  Score", fmt.Sprintf("%.2f", r.Score))
				if r.Snippet != "" {
					formatter.KeyValue("  Snippet", truncate(r.Snippet, 120))
				}
				formatter.Println("")
			}
			formatter.KeyValue("Total", fmt.Sprintf("%d", len(results)))

			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 10, "maximum results")
	cmd.Flags().StringVarP(&memoryType, "type", "t", "", "filter by memory type")
	return cmd
}

// NewMemorySnapshotCommand creates the `cosca memory snapshot` command.
func NewMemorySnapshotCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage memory snapshots",
		Long:  `Create, list, and restore memory snapshots for backup and recovery.`,
	}

	cmd.AddCommand(
		NewMemorySnapshotCreateCommand(),
		NewMemorySnapshotListCommand(),
		NewMemorySnapshotRestoreCommand(),
	)

	return cmd
}

// NewMemorySnapshotCreateCommand creates the `cosca memory snapshot create` subcommand.
func NewMemorySnapshotCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a memory snapshot",
		Long:  `Create a snapshot of all current memory for backup purposes.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			spinner := formatter.Spinner("Creating memory snapshot")
			spinner.Start()

			mgr := NewMemoryManager(coscaDir)
			defer func() { _ = mgr.Close() }()
			if mgr == nil {
				spinner.Fail("Memory manager not available")
				return fmt.Errorf("memory manager not available")
			}

			snapshot, err := mgr.CreateSnapshot()
			if err != nil {
				spinner.Fail(fmt.Sprintf("Snapshot failed: %v", err))
				return fmt.Errorf("failed to create snapshot: %w", err)
			}

			spinner.Stop("Snapshot created")

			if useJSON {
				return printJSON(cmd, snapshot)
			}

			formatter.Success(fmt.Sprintf("Snapshot %s created", snapshot.ID))
			formatter.KeyValue("ID", snapshot.ID)
			formatter.KeyValue("Size", snapshot.Size)
			formatter.KeyValue("Records", fmt.Sprintf("%d", snapshot.RecordCount))
			formatter.KeyValue("Created", snapshot.CreatedAt.Format(time.RFC3339))

			return nil
		},
	}

	return cmd
}

// NewMemorySnapshotListCommand creates the `cosca memory snapshot list` subcommand.
func NewMemorySnapshotListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List memory snapshots",
		Long:  `List all available memory snapshots.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := NewMemoryManager(coscaDir)
			defer func() { _ = mgr.Close() }()
			if mgr == nil {
				return fmt.Errorf("memory manager not available")
			}

			snapshots, err := mgr.ListSnapshots()
			if err != nil {
				return fmt.Errorf("failed to list snapshots: %w", err)
			}

			if useJSON {
				return printJSON(cmd, snapshots)
			}

			if len(snapshots) == 0 {
				formatter.Warning("No snapshots available")
				return nil
			}

			formatter.Header("Memory Snapshots")
			for _, snap := range snapshots {
				formatter.KeyValue(snap.ID, snap.CreatedAt.Format(time.RFC3339))
				formatter.KeyValue("  Size", snap.Size)
				formatter.KeyValue("  Records", fmt.Sprintf("%d", snap.RecordCount))
				formatter.Println("")
			}
			formatter.KeyValue("Total", fmt.Sprintf("%d", len(snapshots)))

			return nil
		},
	}

	return cmd
}

// NewMemorySnapshotRestoreCommand creates the `cosca memory snapshot restore` subcommand.
func NewMemorySnapshotRestoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore a memory snapshot",
		Long:  `Restore memory to a previous state from a snapshot.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			spinner := formatter.Spinner(fmt.Sprintf("Restoring snapshot %s", args[0]))
			spinner.Start()

			mgr := NewMemoryManager(coscaDir)
			defer func() { _ = mgr.Close() }()
			if mgr == nil {
				spinner.Fail("Memory manager not available")
				return fmt.Errorf("memory manager not available")
			}

			if err := mgr.RestoreSnapshot(args[0]); err != nil {
				spinner.Fail(fmt.Sprintf("Restore failed: %v", err))
				return fmt.Errorf("failed to restore snapshot: %w", err)
			}

			spinner.Stop("Snapshot restored")

			if useJSON {
				return printJSON(cmd, map[string]string{
					"status":   "restored",
					"snapshot": args[0],
				})
			}

			formatter.Success(fmt.Sprintf("Snapshot %s restored", args[0]))
			return nil
		},
	}

	return cmd
}

// NewMemoryPruneCommand creates the `cosca memory prune` subcommand.
func NewMemoryPruneCommand() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune expired memory",
		Long:  `Remove expired and stale memory records based on retention policies.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			spinner := formatter.Spinner("Pruning expired memory")
			spinner.Start()

			mgr := NewMemoryManager(coscaDir)
			defer func() { _ = mgr.Close() }()
			if mgr == nil {
				spinner.Fail("Memory manager not available")
				return fmt.Errorf("memory manager not available")
			}

			pruneResult, err := mgr.Prune(dryRun)
			if err != nil {
				spinner.Fail(fmt.Sprintf("Prune failed: %v", err))
				return fmt.Errorf("prune failed: %w", err)
			}

			spinner.Stop("Prune complete")

			if useJSON {
				return printJSON(cmd, pruneResult)
			}

			if dryRun {
				formatter.KeyValue("Would Remove", fmt.Sprintf("%d entries", pruneResult.Removed))
				formatter.KeyValue("Space Would Be Reclaimed", pruneResult.SpaceReclaimed)
				formatter.Println("Run without --dry-run to apply")
			} else {
				formatter.Success(fmt.Sprintf("Removed %d expired entries", pruneResult.Removed))
				formatter.KeyValue("Space Reclaimed", pruneResult.SpaceReclaimed)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview what would be pruned without removing")
	return cmd
}

// NewMemoryPromoteCommand creates the `cosca memory promote` subcommand.
func NewMemoryPromoteCommand() *cobra.Command {
	var tier string

	cmd := &cobra.Command{
		Use:   "promote <id>",
		Short: "Promote a memory record to a higher tier",
		Long: `Promove um registro de memória para o tier long (1 ano).

A regra da casa (L338): TUDO entra como médio (7 dias) — o longo é
reservado, só entra aqui o que o Don decidir promover. "O longo a gente
vai ver o que coloca."

O registro é procurado em todas as camadas (temp/session/project/workspace/
global) e movido para o destino.`,
		Example: `  cosca memory promote mem-abc123 --tier long`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			id := args[0]

			target := memory.LayerLong
			switch tier {
			case "long":
				target = memory.LayerLong
			case "medium":
				target = memory.LayerGlobal
			default:
				return fmt.Errorf("tier inválido: %s (use 'long')", tier)
			}

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			spinner := formatter.Spinner(fmt.Sprintf("Promovendo %s para %s", id, tier))
			spinner.Start()

			mgr := NewMemoryManager(coscaDir)
			defer func() { _ = mgr.Close() }()
			if mgr == nil {
				spinner.Fail("Memory manager not available")
				return fmt.Errorf("memory manager not available")
			}

			record, err := mgr.PromoteByID(id, target)
			if err != nil {
				spinner.Fail(fmt.Sprintf("Promoção falhou: %v", err))
				return fmt.Errorf("promote: %w", err)
			}

			spinner.Stop(fmt.Sprintf("Promovido para %s", tier))

			if useJSON {
				return printJSON(cmd, record)
			}

			formatter.Header("Registro promovido")
			formatter.KeyValue("ID", record.ID)
			formatter.KeyValue("Tier", string(record.Layer))
			formatter.KeyValue("Escopo", record.Scope)
			formatter.KeyValue("Atualizado", record.UpdatedAt.Format("2006-01-02 15:04"))
			formatter.Success("Agora vive 1 ano — só o GC de longo prazo toca nele.")

			return nil
		},
	}

	cmd.Flags().StringVar(&tier, "tier", "long", "Tier de destino: long (1 ano)")
	return cmd
}

// NewMemoryReindexCommand creates the `cosca memory reindex` subcommand.
func NewMemoryReindexCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild the memory search index from disk",
		Long: `Rebuilds the FTS5 search index from the memory files on disk.

Fixes a drifted index (searches returning nothing while records exist)
and drops orphaned entries whose files no longer exist.`,
		Example: `  cosca memory reindex
  cosca memory reindex --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			spinner := formatter.Spinner("Rebuilding memory search index")
			spinner.Start()

			mgr := NewMemoryManager(coscaDir)
			defer func() { _ = mgr.Close() }()
			if mgr == nil {
				spinner.Fail("Memory manager not available")
				return fmt.Errorf("memory manager not available")
			}

			result, err := mgr.ResyncIndex()
			if err != nil {
				spinner.Fail(fmt.Sprintf("Reindex failed: %v", err))
				return fmt.Errorf("reindex failed: %w", err)
			}

			spinner.Stop("Reindex complete")

			if useJSON {
				return printJSON(cmd, result)
			}

			formatter.Header("Memory Reindex")
			total := 0
			for layer, count := range result {
				formatter.KeyValue(layer, fmt.Sprintf("%d records", count))
				total += count
			}
			formatter.KeyValue("Total", fmt.Sprintf("%d records", total))
			return nil
		},
	}

	return cmd
}

// NewMemoryStatsCommand creates the `cosca memory stats` subcommand.
func NewMemoryStatsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Memory statistics",
		Long:  `Display detailed statistics about the memory system.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := NewMemoryManager(coscaDir)
			defer func() { _ = mgr.Close() }()
			if mgr == nil {
				return fmt.Errorf("memory manager not available")
			}

			stats := mgr.Status()

			if useJSON {
				return printJSON(cmd, stats)
			}

			formatter.Header("Memory Statistics")
			formatter.KeyValue("Short-term Entries", fmt.Sprintf("%d", stats.ShortEntries))
			formatter.KeyValue("Long-term Entries", fmt.Sprintf("%d", stats.LongEntries))
			formatter.KeyValue("Project Entries", fmt.Sprintf("%d", stats.ProjectEntries))
			formatter.KeyValue("Architecture Entries", fmt.Sprintf("%d", stats.ArchEntries))
			formatter.KeyValue("Decision Entries", fmt.Sprintf("%d", stats.DecisionEntries))
			formatter.KeyValue("Total Entries", fmt.Sprintf("%d", stats.TotalEntries))
			formatter.KeyValue("Total Size", stats.TotalSize)
			formatter.KeyValue("Snapshot Count", fmt.Sprintf("%d", stats.SnapshotCount))
			formatter.KeyValue("Last Pruned", stats.LastPruned.Format("2006-01-02 15:04:05"))

			return nil
		},
	}

	return cmd
}

// NewMemoryCuratedFailuresCommand creates the `cosca memory curated-failures`
// subcommand. It exposes only the SANITIZED lesson of each failure (P5 — the
// family learns with errors). Internal details (Task, Failed Approach, Root
// Cause, Consequence) are never printed.
func NewMemoryCuratedFailuresCommand() *cobra.Command {
	var agentFilter string
	var domainFilter string

	cmd := &cobra.Command{
		Use:   "curated-failures",
		Short: "Mostra apenas as LIÇÕES curadas das falhas (P5 — sem expor detalhes internos)",
		Long: `Mostra um digest sanitizado das falhas registradas pelos agentes (P5 — a família
aprende com erros). Apenas a LIÇÃO (Lesson), o padrão de evitação (Avoidance
Pattern) e as tags são compartilhados — os detalhes internos (Task, Failed
Approach, Root Cause, Consequence) nunca são expostos.`,
		Example: `  cosca memory curated-failures
  cosca memory curated-failures --agent cosca-kernel
  cosca memory curated-failures --domain backend`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			agentDir := filepath.Join(dir, "internal", "embed", "cosca", "memory", "agent")

			info, statErr := os.Stat(agentDir)
			if statErr != nil || !info.IsDir() {
				return fmt.Errorf("agent memory directory not found: %s", agentDir)
			}

			curated, err := memory.CurateFailuresDir(agentDir)
			if err != nil {
				return fmt.Errorf("failed to curate failures: %w", err)
			}

			filtered := curated[:0]
			for _, f := range curated {
				if agentFilter != "" && f.Agent != agentFilter {
					continue
				}
				if domainFilter != "" && f.Domain != domainFilter {
					continue
				}
				filtered = append(filtered, f)
			}

			sort.SliceStable(filtered, func(i, j int) bool {
				if filtered[i].Agent != filtered[j].Agent {
					return filtered[i].Agent < filtered[j].Agent
				}
				return filtered[i].ID < filtered[j].ID
			})

			if useJSON {
				return printJSON(cmd, filtered)
			}

			if len(filtered) == 0 {
				formatter.Warning("No curated failures found")
				return nil
			}

			formatter.Header("Curated Failures (P5 — lessons only)")
			for _, f := range filtered {
				formatter.KeyValue(f.ID, fmt.Sprintf("%s | %s | %s", f.Agent, f.Date, f.Name))
				formatter.KeyValue("  Lesson", f.Lesson)
				if f.AvoidancePattern != "" {
					formatter.KeyValue("  Avoidance", f.AvoidancePattern)
				}
				if len(f.Tags) > 0 {
					formatter.KeyValue("  Tags", strings.Join(f.Tags, " "))
				}
				formatter.Println("")
			}
			formatter.KeyValue("Total", fmt.Sprintf("%d", len(filtered)))

			return nil
		},
	}

	cmd.Flags().StringVar(&agentFilter, "agent", "", "filter by agent (e.g. cosca-kernel)")
	cmd.Flags().StringVar(&domainFilter, "domain", "", "filter by domain (e.g. backend)")
	return cmd
}
