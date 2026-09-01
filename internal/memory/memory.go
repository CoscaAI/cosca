package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// MemoryLayer represents a layer of memory with specific scope and persistence.
//
//nolint:revive // Stutter name preserved for API compatibility — used as memory.MemoryLayer externally.
type MemoryLayer string

const (
	// LayerGlobal is the global memory layer, shared across all workspaces.
	LayerGlobal MemoryLayer = "global"
	// LayerWorkspace is the workspace-level memory layer.
	LayerWorkspace MemoryLayer = "workspace"
	// LayerProject is the project-level memory layer.
	LayerProject MemoryLayer = "project"
	// LayerSession is the session-level memory layer (ephemeral).
	LayerSession MemoryLayer = "session"
	// LayerTemp is a temporary memory layer (ephemeral, short TTL).
	LayerTemp MemoryLayer = "temp"
	// LayerLong is the long-term memory layer (1 year TTL). Everything else
	// is medium-term (7 days) by default; only explicit promotion moves a
	// record here — the Don decides what lives long (L338).
	LayerLong MemoryLayer = "long"
)

// MemoryType represents the type of memory record.
//
//nolint:revive // Stutter name preserved for API compatibility — used as memory.MemoryType externally.
type MemoryType string

// Predefined memory types.
const (
	MemoryTypeDecision     MemoryType = "decision"
	MemoryTypePattern      MemoryType = "pattern"
	MemoryTypeBug          MemoryType = "bug"
	MemoryTypeAgent        MemoryType = "agent"
	MemoryTypeProject      MemoryType = "project"
	MemoryTypeArchitecture MemoryType = "architecture"
	MemoryTypeSession      MemoryType = "session"
)

// MemoryRecord represents a single memory entry.
//
// Owner is the user ID (claims.Sub) that created the record. Empty Owner
// means system/agent memory, shared globally across all users (A6 — owner
// scoping / IDOR). Existing records written before the Owner field existed
// have no owner in frontmatter and are treated as global.
//
//nolint:revive // Stutter name preserved for API compatibility — used as memory.MemoryRecord externally.
type MemoryRecord struct {
	ID        string            `json:"id" yaml:"id"`
	Type      MemoryType        `json:"type" yaml:"type"`
	Layer     MemoryLayer       `json:"layer" yaml:"layer"`
	Scope     string            `json:"scope,omitempty" yaml:"scope,omitempty"`
	Owner     string            `json:"owner,omitempty" yaml:"owner,omitempty"`
	TenantID  string            `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	Agent     string            `json:"agent,omitempty" yaml:"agent,omitempty"`
	Content   string            `json:"content" yaml:"content"`
	Metadata  map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time         `json:"updated_at" yaml:"updated_at"`
	TTL       time.Duration     `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Priority  int               `json:"priority" yaml:"priority"`
	Version   int               `json:"version" yaml:"version"`
}

// MemoryEngine manages multi-layer memory with store, retrieve, search, and lifecycle operations.
//
//nolint:revive // Stutter name preserved for API compatibility — used as memory.MemoryEngine externally.
type MemoryEngine struct {
	mu        sync.RWMutex
	logger    zerolog.Logger
	stores    map[MemoryLayer]Store
	layers    *LayerManager
	config    EngineConfig
	snapshots *SnapshotManager

	// episodicTTL / episodicMaxRecords são a retenção da camada episódica
	// (FASE D). Configuráveis via ConfigureEpisodicRetention; defaults
	// DefaultEpisodicTTL / DefaultEpisodicMaxRecords.
	episodicTTL        time.Duration
	episodicMaxRecords int
}

// EngineConfig configures the memory engine.
type EngineConfig struct {
	DataDir            string        `json:"data_dir"`
	DefaultTTL         time.Duration `json:"default_ttl"`
	MaxRecordsPerLayer int           `json:"max_records_per_layer"`
	AutoPrune          bool          `json:"auto_prune"`
	PruneInterval      time.Duration `json:"prune_interval"`
	SnapshotOnPromote  bool          `json:"snapshot_on_promote"`
}

// DefaultConfig returns a default engine configuration.
func DefaultConfig() EngineConfig {
	return EngineConfig{
		DefaultTTL:         24 * time.Hour,
		MaxRecordsPerLayer: 1000,
		AutoPrune:          true,
		PruneInterval:      30 * time.Minute,
		SnapshotOnPromote:  true,
	}
}

// Option configures the memory engine.
type Option func(*MemoryEngine)

// WithLogger sets the logger.
func WithLogger(logger zerolog.Logger) Option {
	return func(e *MemoryEngine) {
		e.logger = logger
	}
}

// WithStore sets a store for a specific layer.
func WithStore(layer MemoryLayer, store Store) Option {
	return func(e *MemoryEngine) {
		e.stores[layer] = store
	}
}

// WithConfig sets the engine configuration.
func WithConfig(cfg EngineConfig) Option {
	return func(e *MemoryEngine) {
		e.config = cfg
	}
}

// NewEngine creates a new memory engine.
func NewEngine(opts ...Option) (*MemoryEngine, error) {
	e := &MemoryEngine{
		logger: zerolog.Nop(),
		stores: make(map[MemoryLayer]Store),
		layers: NewLayerManager(),
		config: DefaultConfig(),
	}

	for _, opt := range opts {
		opt(e)
	}

	// Initialize default stores for layers that don't have one
	for _, layer := range e.layers.Layers() {
		if _, ok := e.stores[layer]; !ok {
			store, err := NewFileStore(e.config.DataDir, layer, e.logger)
			if err != nil {
				return nil, fmt.Errorf("creating store for layer %s: %w", layer, err)
			}
			e.stores[layer] = store
		}
	}

	// Initialize snapshot manager if data directory is configured
	if e.config.DataDir != "" {
		snapshots, err := NewSnapshotManager(e.config.DataDir, e, e.logger)
		if err != nil {
			e.logger.Warn().Err(err).Msg("failed to initialize snapshot manager")
		} else {
			e.snapshots = snapshots
		}
	}

	// Defensive: a zero prune interval would panic NewTicker. Fall back to the
	// default (same as DefaultConfig) so any caller enabling AutoPrune without
	// an explicit interval is safe.
	if e.config.PruneInterval <= 0 {
		e.config.PruneInterval = 30 * time.Minute
	}

	// Retenção episódica (FASE D): defaults se não configurada.
	if e.episodicTTL <= 0 {
		e.episodicTTL = DefaultEpisodicTTL
	}
	if e.episodicMaxRecords <= 0 {
		e.episodicMaxRecords = DefaultEpisodicMaxRecords
	}

	if e.config.AutoPrune {
		go e.autoPruneLoop()
	}

	e.logger.Info().
		Int("layers", len(e.stores)).
		Bool("auto_prune", e.config.AutoPrune).
		Msg("memory engine initialized")

	return e, nil
}

// Store stores a memory record.
func (e *MemoryEngine) Store(ctx context.Context, record MemoryRecord) (*MemoryRecord, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	record.UpdatedAt = time.Now()
	if record.TTL == 0 {
		record.TTL = e.config.DefaultTTL
	}

	store, ok := e.stores[record.Layer]
	if !ok {
		return nil, fmt.Errorf("no store for layer %s", record.Layer)
	}

	saved, err := store.Save(ctx, record)
	if err != nil {
		return nil, fmt.Errorf("saving record: %w", err)
	}

	e.logger.Debug().
		Str("id", saved.ID).
		Str("type", string(saved.Type)).
		Str("layer", string(saved.Layer)).
		Msg("memory record stored")

	return saved, nil
}

// Retrieve retrieves a memory record by ID.
func (e *MemoryEngine) Retrieve(ctx context.Context, id string, layer MemoryLayer) (*MemoryRecord, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	store, ok := e.stores[layer]
	if !ok {
		return nil, fmt.Errorf("no store for layer %s", layer)
	}

	record, err := store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("retrieving record: %w", err)
	}

	// Check TTL expiration
	if record.TTL > 0 && time.Since(record.CreatedAt) > record.TTL {
		e.mu.RUnlock()
		e.mu.Lock()
		_ = store.Delete(ctx, id)
		e.mu.Unlock()
		e.mu.RLock()
		return nil, fmt.Errorf("record %s has expired", id)
	}

	return record, nil
}

// Delete removes a memory record by ID from the specified layer.
func (e *MemoryEngine) Delete(ctx context.Context, id string, layer MemoryLayer) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	store, ok := e.stores[layer]
	if !ok {
		return fmt.Errorf("no store for layer %s", layer)
	}

	if err := store.Delete(ctx, id); err != nil {
		return fmt.Errorf("deleting record: %w", err)
	}

	e.logger.Debug().
		Str("id", id).
		Str("layer", string(layer)).
		Msg("memory record deleted")

	return nil
}

// Search searches for memory records across layers.
func (e *MemoryEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]MemoryRecord, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var allRecords []MemoryRecord

	// Determine which layers to search
	layers := opts.Layers
	if len(layers) == 0 {
		layers = e.layers.Layers()
	}

	for _, layer := range layers {
		store, ok := e.stores[layer]
		if !ok {
			continue
		}
		records, err := store.Search(ctx, query, opts)
		if err != nil {
			e.logger.Warn().Err(err).Str("layer", string(layer)).Msg("search failed")
			continue
		}
		allRecords = append(allRecords, records...)
	}

	// Sort by priority (layer priority + record priority)
	e.sortByPriority(allRecords)

	// Limit results
	if opts.Limit > 0 && len(allRecords) > opts.Limit {
		allRecords = allRecords[:opts.Limit]
	}

	return allRecords, nil
}

// Index indexes a memory record for search.
func (e *MemoryEngine) Index(ctx context.Context, record MemoryRecord) error {
	store, ok := e.stores[record.Layer]
	if !ok {
		return fmt.Errorf("no store for layer %s", record.Layer)
	}
	return store.Index(ctx, record)
}

// reindexer is implemented by stores that can rebuild their FTS index from
// the records on disk. Kept as an optional interface so mocks and future
// store backends are not forced to implement it.
type reindexer interface {
	RebuildIndex(ctx context.Context) (int, error)
}

// ResyncIndex rebuilds the FTS search index from the records on disk for
// every layer. It drops orphaned entries (index rows whose files no longer
// exist) and makes previously unindexed files searchable. Use it after a
// manual restore of .cosca or whenever searches return nothing while
// records clearly exist.
func (e *MemoryEngine) ResyncIndex(ctx context.Context) (map[MemoryLayer]int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make(map[MemoryLayer]int)
	for layer, store := range e.stores {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		rb, ok := store.(reindexer)
		if !ok {
			continue
		}
		n, err := rb.RebuildIndex(ctx)
		if err != nil {
			e.logger.Warn().Err(err).Str("layer", string(layer)).Msg("rebuilding layer index")
			continue
		}
		result[layer] = n
	}

	return result, nil
}

// Prune removes expired records from a layer.
func (e *MemoryEngine) Prune(ctx context.Context, layer MemoryLayer) (int, error) {
	store, ok := e.stores[layer]
	if !ok {
		return 0, fmt.Errorf("no store for layer %s", layer)
	}

	count, err := store.Prune(ctx)
	if err != nil {
		return 0, fmt.Errorf("pruning layer %s: %w", layer, err)
	}

	if count > 0 {
		e.logger.Info().
			Int("count", count).
			Str("layer", string(layer)).
			Msg("pruned expired records")
	}

	return count, nil
}

// Promote promotes a memory record to a higher layer (e.g., session -> project -> global).
func (e *MemoryEngine) Promote(ctx context.Context, id string, from, to MemoryLayer) (*MemoryRecord, error) {
	e.mu.Lock()

	fromStore, ok := e.stores[from]
	if !ok {
		e.mu.Unlock()
		return nil, fmt.Errorf("no store for source layer %s", from)
	}

	record, err := fromStore.Get(ctx, id)
	if err != nil {
		e.mu.Unlock()
		return nil, fmt.Errorf("getting record from %s: %w", from, err)
	}

	// Update layer
	record.Layer = to
	record.UpdatedAt = time.Now()

	// Save to target layer
	toStore, ok := e.stores[to]
	if !ok {
		e.mu.Unlock()
		return nil, fmt.Errorf("no store for target layer %s", to)
	}

	promoted, err := toStore.Save(ctx, *record)
	if err != nil {
		e.mu.Unlock()
		return nil, fmt.Errorf("saving to target layer %s: %w", to, err)
	}

	// Remove from source layer
	if err := fromStore.Delete(ctx, id); err != nil {
		e.logger.Warn().Err(err).Str("id", id).Str("from", string(from)).Msg("failed to delete from source after promote")
	}

	// Release engine lock before creating snapshot to avoid deadlock.
	// snapshot.Create calls engine.Search which needs engine.mu.RLock,
	// and Go's sync.RWMutex is not reentrant.
	snapshotOnPromote := e.config.SnapshotOnPromote
	snapshots := e.snapshots
	e.mu.Unlock()

	// Create snapshot if configured
	if snapshotOnPromote && snapshots != nil {
		if _, err := snapshots.Create(ctx, to, fmt.Sprintf("promote-%s-%s-%s", id, from, to)); err != nil {
			e.logger.Warn().Err(err).Str("id", id).Str("to", string(to)).Msg("failed to create snapshot on promote")
		}
	}

	e.logger.Info().
		Str("id", id).
		Str("from", string(from)).
		Str("to", string(to)).
		Msg("memory record promoted")

	return promoted, nil
}

// GetLayerStats returns statistics for a memory layer.
func (e *MemoryEngine) GetLayerStats(ctx context.Context) map[MemoryLayer]LayerStats {
	stats := make(map[MemoryLayer]LayerStats)
	for layer, store := range e.stores {
		s, err := store.Stats(ctx)
		if err != nil {
			e.logger.Warn().Err(err).Str("layer", string(layer)).Msg("getting layer stats")
			continue
		}
		stats[layer] = s
	}
	return stats
}

// GetLayerStatsScoped returns statistics for records visible to one tenant
// owner. It is used by non-admin API callers so aggregate counts cannot leak
// other tenants' memory.
func (e *MemoryEngine) GetLayerStatsScoped(ctx context.Context, owner, tenant string) map[MemoryLayer]LayerStats {
	stats := make(map[MemoryLayer]LayerStats)
	e.mu.RLock()
	defer e.mu.RUnlock()
	for layer, store := range e.stores {
		records, err := store.Search(ctx, "", SearchOptions{OwnerFilter: owner, TenantFilter: tenant, Limit: int(^uint(0) >> 1)})
		if err != nil {
			e.logger.Warn().Err(err).Str("layer", string(layer)).Msg("getting scoped layer stats")
			continue
		}
		var size int64
		for _, record := range records {
			size += int64(len(record.Content))
		}
		stats[layer] = LayerStats{Count: len(records), TotalSize: int(size)}
	}
	return stats
}

// Close closes the memory engine and all stores.
func (e *MemoryEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for layer, store := range e.stores {
		if err := store.Close(); err != nil {
			e.logger.Warn().Err(err).Str("layer", string(layer)).Msg("closing store")
		}
	}

	return nil
}

// sortByPriority sorts records by layer priority then record priority.
func (e *MemoryEngine) sortByPriority(records []MemoryRecord) {
	// Simple stable sort: higher priority first
	for i := 0; i < len(records); i++ {
		for j := i + 1; j < len(records); j++ {
			iPriority := e.layers.LayerPriority(records[i].Layer) + records[i].Priority
			jPriority := e.layers.LayerPriority(records[j].Layer) + records[j].Priority
			if jPriority > iPriority {
				records[i], records[j] = records[j], records[i]
			}
		}
	}
}

// CreateSnapshot creates a snapshot of a memory layer.
func (e *MemoryEngine) CreateSnapshot(ctx context.Context, layer MemoryLayer, label string) (*Snapshot, error) {
	if e.snapshots == nil {
		return nil, fmt.Errorf("snapshot manager not available")
	}
	return e.snapshots.Create(ctx, layer, label)
}

// ListSnapshots returns all available snapshots.
func (e *MemoryEngine) ListSnapshots(ctx context.Context) ([]Snapshot, error) {
	if e.snapshots == nil {
		return nil, fmt.Errorf("snapshot manager not available")
	}
	return e.snapshots.List(ctx)
}

// RestoreSnapshot restores memory state from a snapshot.
func (e *MemoryEngine) RestoreSnapshot(_ context.Context, snapshotID string) error {
	if e.snapshots == nil {
		return fmt.Errorf("snapshot manager not available")
	}
	return e.snapshots.Restore(context.Background(), snapshotID)
}

// autoPruneLoop periodically prunes expired records.
func (e *MemoryEngine) autoPruneLoop() {
	ticker := time.NewTicker(e.config.PruneInterval)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()
		for layer := range e.stores {
			if _, err := e.Prune(ctx, layer); err != nil {
				e.logger.Warn().Err(err).Str("layer", string(layer)).Msg("auto-prune failed")
			}
		}
	}
}
