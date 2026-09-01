package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/CoscaAI/cosca/internal/memory"
)

// Memory Adapter
// =============================================================================

// memoryManagerAdapter wraps memory.MemoryEngine to match CLI expected API.
type memoryManagerAdapter struct {
	inner *memory.MemoryEngine
	dir   string
}

// newMemoryManagerAdapter creates a memory adapter.
// CLI calls memory.NewManager(dir) but real API is memory.NewEngine(opts...).
func newMemoryManagerAdapter(dir string) *memoryManagerAdapter {
	engine, err := memory.NewEngine(
		memory.WithConfig(memory.EngineConfig{
			DataDir: dir,
		}),
	)
	if err != nil {
		return &memoryManagerAdapter{inner: nil, dir: dir}
	}
	return &memoryManagerAdapter{inner: engine, dir: dir}
}

// List lists memory records. Placeholder matching CLI expected signature.
func (a *memoryManagerAdapter) List(memoryType string, limit int) ([]MemoryRecordEx, error) {
	if a.inner == nil {
		return nil, fmt.Errorf("memory not available")
	}
	ctx := context.Background()
	opts := memory.SearchOptions{
		Limit: limit,
	}
	if memoryType != "" {
		opts.Types = []memory.MemoryType{memory.MemoryType(memoryType)}
	}
	records, err := a.inner.Search(ctx, "", opts)
	if err != nil {
		return nil, fmt.Errorf("list memory: %w", err)
	}
	result := make([]MemoryRecordEx, len(records))
	for i, r := range records {
		result[i] = MemoryRecordEx{
			ID:        r.ID,
			Title:     r.Scope,
			Type:      string(r.Type),
			Content:   r.Content,
			Timestamp: r.CreatedAt,
		}
	}
	return result, nil
}

// Get retrieves a memory record by searching all layers.
func (a *memoryManagerAdapter) Get(id string) (MemoryRecordEx, error) {
	if a.inner == nil {
		return MemoryRecordEx{}, fmt.Errorf("memory not available")
	}
	ctx := context.Background()
	// Try each layer
	for _, layer := range []memory.MemoryLayer{
		memory.LayerSession, memory.LayerProject, memory.LayerWorkspace,
		memory.LayerGlobal, memory.LayerTemp,
	} {
		record, err := a.inner.Retrieve(ctx, id, layer)
		if err == nil {
			return MemoryRecordEx{
				ID:        record.ID,
				Title:     record.Scope,
				Type:      string(record.Type),
				Content:   record.Content,
				Timestamp: record.CreatedAt,
				Tags:      []string{},
				Related:   []string{},
			}, nil
		}
	}
	return MemoryRecordEx{}, fmt.Errorf("record %s not found", id)
}

// PromoteByID finds a record across all layers and promotes it to the
// target layer. The Don decides what lives long: default writes are medium
// (7d) and only explicit promotion moves a record to the long tier (L338).
func (a *memoryManagerAdapter) PromoteByID(id string, to memory.MemoryLayer) (*memory.MemoryRecord, error) {
	if a.inner == nil {
		return nil, fmt.Errorf("memory not available")
	}
	ctx := context.Background()
	for _, layer := range []memory.MemoryLayer{
		memory.LayerTemp, memory.LayerSession, memory.LayerProject,
		memory.LayerWorkspace, memory.LayerGlobal, memory.LayerLong,
	} {
		record, err := a.inner.Retrieve(ctx, id, layer)
		if err == nil && record != nil {
			return a.inner.Promote(ctx, id, layer, to)
		}
	}
	return nil, fmt.Errorf("record %s not found in any layer", id)
}

// Search searches memory records.
func (a *memoryManagerAdapter) Search(opts MemorySearchOptions) ([]MemorySearchResult, error) {
	if a.inner == nil {
		return nil, fmt.Errorf("memory not available")
	}
	ctx := context.Background()
	searchOpts := memory.SearchOptions{
		Limit: opts.Limit,
	}
	if opts.Type != "" {
		searchOpts.Types = []memory.MemoryType{memory.MemoryType(opts.Type)}
	}
	records, err := a.inner.Search(ctx, opts.Query, searchOpts)
	if err != nil {
		return nil, fmt.Errorf("search memory: %w", err)
	}
	result := make([]MemorySearchResult, len(records))
	for i, r := range records {
		snippet := r.Content
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		result[i] = MemorySearchResult{
			Title:   r.Scope,
			Type:    string(r.Type),
			Score:   float64(r.Priority) / 100.0,
			Snippet: snippet,
		}
	}
	return result, nil
}

// QueryEpisodic (FASE D) consulta a memória episódica multimodal da engine.
func (a *memoryManagerAdapter) QueryEpisodic(ctx context.Context, q memory.EpisodicQuery) ([]memory.EpisodicRecord, error) {
	if a.inner == nil {
		return nil, fmt.Errorf("memory not available")
	}
	return a.inner.QueryEpisodic(ctx, q)
}

// CreateSnapshot creates a memory snapshot.
func (a *memoryManagerAdapter) CreateSnapshot() (MemorySnapshotEx, error) {
	if a.inner == nil {
		return MemorySnapshotEx{}, fmt.Errorf("memory not available")
	}
	ctx := context.Background()
	snap, err := a.inner.CreateSnapshot(ctx, memory.LayerGlobal, "")
	if err != nil {
		return MemorySnapshotEx{}, fmt.Errorf("create snapshot: %w", err)
	}
	return MemorySnapshotEx{
		ID:          snap.ID,
		Size:        formatBytes(snap.Size),
		RecordCount: len(snap.RecordIDs),
		CreatedAt:   snap.CreatedAt,
	}, nil
}

// ListSnapshots lists memory snapshots.
func (a *memoryManagerAdapter) ListSnapshots() ([]MemorySnapshotEx, error) {
	if a.inner == nil {
		return nil, fmt.Errorf("memory not available")
	}
	ctx := context.Background()
	snapshots, err := a.inner.ListSnapshots(ctx)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	result := make([]MemorySnapshotEx, len(snapshots))
	for i, s := range snapshots {
		result[i] = MemorySnapshotEx{
			ID:          s.ID,
			Size:        formatBytes(s.Size),
			RecordCount: len(s.RecordIDs),
			CreatedAt:   s.CreatedAt,
		}
	}
	return result, nil
}

// RestoreSnapshot restores a memory snapshot.
func (a *memoryManagerAdapter) RestoreSnapshot(id string) error {
	if a.inner == nil {
		return fmt.Errorf("memory not available")
	}
	ctx := context.Background()
	return a.inner.RestoreSnapshot(ctx, id)
}

// Prune prunes expired memory.
func (a *memoryManagerAdapter) Prune(dryRun bool) (PruneResult, error) {
	if a.inner == nil {
		return PruneResult{}, fmt.Errorf("memory not available")
	}
	if dryRun {
		return PruneResult{Removed: 0, SpaceReclaimed: "0 B"}, nil
	}
	ctx := context.Background()
	totalRemoved := 0
	for _, layer := range []memory.MemoryLayer{
		memory.LayerSession, memory.LayerProject, memory.LayerWorkspace,
		memory.LayerGlobal, memory.LayerTemp,
	} {
		count, err := a.inner.Prune(ctx, layer)
		if err != nil {
			continue
		}
		totalRemoved += count
	}
	return PruneResult{
		Removed:        totalRemoved,
		SpaceReclaimed: fmt.Sprintf("%d records", totalRemoved),
	}, nil
}

// ResyncIndex rebuilds the FTS search index from the records on disk,
// per layer. Fixes a drifted index and drops orphaned entries.
func (a *memoryManagerAdapter) ResyncIndex() (map[string]int, error) {
	if a.inner == nil {
		return nil, fmt.Errorf("memory not available")
	}
	result, err := a.inner.ResyncIndex(context.Background())
	if err != nil {
		return nil, fmt.Errorf("resync memory index: %w", err)
	}
	out := make(map[string]int, len(result))
	for layer, n := range result {
		out[string(layer)] = n
	}
	return out, nil
}

// Status returns memory statistics.
func (a *memoryManagerAdapter) Status() MemoryStatusEx {
	stats := MemoryStatusEx{}
	if a.inner != nil {
		layerStats := a.inner.GetLayerStats(context.Background())
		for layer, ls := range layerStats {
			switch layer {
			case memory.LayerSession:
				stats.ShortEntries = ls.Count
			case memory.LayerGlobal:
				stats.LongEntries = ls.Count
			case memory.LayerProject:
				stats.ProjectEntries = ls.Count
			case memory.LayerWorkspace:
				stats.ArchEntries = ls.Count
			case memory.LayerTemp:
				stats.DecisionEntries = ls.Count
			}
		}
		stats.TotalEntries = stats.ShortEntries + stats.LongEntries + stats.ProjectEntries + stats.ArchEntries + stats.DecisionEntries
	}
	return stats
}

// Init initializes the memory system. Placeholder for install flow.
func (a *memoryManagerAdapter) Init() {
	// no-op
}

// Close fecha o engine de memória subjacente, liberando o SQLite index.db
// (memory/index.db). Necessário no Windows: um handle aberto impede a remoção
// do diretório e o os.RemoveAll do t.TempDir() nos testes; em produção evita
// manter o arquivo travado após o comando terminar.
func (a *memoryManagerAdapter) Close() error {
	if a.inner != nil {
		return a.inner.Close()
	}
	return nil
}

// MemoryRecordEx mirrors memory.MemoryRecord for CLI consumption.
type MemoryRecordEx struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Tags      []string  `json:"tags"`
	Related   []string  `json:"related"`
}

// MemorySearchOptions holds search options.
type MemorySearchOptions struct {
	Query string `json:"query"`
	Type  string `json:"type"`
	Limit int    `json:"limit"`
}

// MemorySearchResult holds a search result.
type MemorySearchResult struct {
	Title   string  `json:"title"`
	Type    string  `json:"type"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet"`
}

// MemorySnapshotEx holds snapshot data.
type MemorySnapshotEx struct {
	ID          string    `json:"id"`
	Size        string    `json:"size"`
	RecordCount int       `json:"record_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// PruneResult holds prune operation results.
type PruneResult struct {
	Removed        int    `json:"removed"`
	SpaceReclaimed string `json:"space_reclaimed"`
}

// MemoryStatusEx holds memory statistics.
type MemoryStatusEx struct {
	ShortEntries    int       `json:"short_entries"`
	LongEntries     int       `json:"long_entries"`
	ProjectEntries  int       `json:"project_entries"`
	ArchEntries     int       `json:"arch_entries"`
	DecisionEntries int       `json:"decision_entries"`
	TotalEntries    int       `json:"total_entries"`
	TotalSize       string    `json:"total_size"`
	SnapshotCount   int       `json:"snapshot_count"`
	LastPruned      time.Time `json:"last_pruned"`
}

// =============================================================================
