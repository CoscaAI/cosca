package memory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// newEngineWithMocks creates a MemoryEngine with MockStore instances for all
// default layers. Use this for testing MemoryEngine operations without
// requiring filesystem access.
func newEngineWithMocks(t *testing.T, opts ...Option) *MemoryEngine {
	t.Helper()

	// Construct engine manually to avoid NewEngine creating FileStores
	// that open SQLite databases (causes deadlock in parallel tests).
	cfg := EngineConfig{
		DefaultTTL:         1 * time.Hour,
		MaxRecordsPerLayer: 100,
		AutoPrune:          false,
	}

	engine := &MemoryEngine{
		logger: zerolog.Nop(),
		stores: make(map[MemoryLayer]Store),
		layers: NewLayerManager(),
		config: cfg,
	}

	// Apply any additional options.
	for _, opt := range opts {
		opt(engine)
	}

	// Provide mock stores for all default layers that don't have one.
	for _, layer := range engine.layers.Layers() {
		if _, ok := engine.stores[layer]; !ok {
			engine.stores[layer] = &MockStore{}
		}
	}

	return engine
}

// recordAt returns a MemoryRecord with the given id and layer.
func recordAt(id string, layer MemoryLayer) MemoryRecord {
	return MemoryRecord{
		ID:      id,
		Type:    MemoryTypeDecision,
		Layer:   layer,
		Content: fmt.Sprintf("content for %s", id),
	}
}

// TestEngineStore_SavesRecord verifies that Store assigns an ID and
// returns the saved record.
func TestEngineStore_SavesRecord(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine := newEngineWithMocks(t)
	defer func() { _ = engine.Close() }()

	record := MemoryRecord{
		Type:    MemoryTypePattern,
		Layer:   LayerSession,
		Content: "test pattern",
	}

	saved, err := engine.Store(ctx, record)
	if err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	if saved == nil {
		t.Fatal("Store() returned nil record")
	}
	if saved.ID == "" {
		t.Error("ID should be assigned")
	}
	if saved.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

// TestEngineStore_UsesProvidedID verifies that Store preserves an
// explicitly provided ID by using a mock with a custom SaveFunc.
func TestEngineStore_UsesProvidedID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mock := &MockStore{
		SaveFunc: func(_ context.Context, record MemoryRecord) (*MemoryRecord, error) {
			// Preserve the original ID without overriding.
			return &record, nil
		},
	}

	engine := newEngineWithMocks(t, WithStore(LayerProject, mock))
	defer func() { _ = engine.Close() }()

	record := MemoryRecord{
		ID:      "my-custom-id",
		Type:    MemoryTypeBug,
		Layer:   LayerProject,
		Content: "bug report",
	}

	saved, err := engine.Store(ctx, record)
	if err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	if saved.ID != "my-custom-id" {
		t.Errorf("ID = %q, want %q", saved.ID, "my-custom-id")
	}
}

// TestEngineStore_AppliesDefaultTTL verifies that Store sets the TTL
// from the engine config when the record has no TTL.
func TestEngineStore_AppliesDefaultTTL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine := newEngineWithMocks(t)
	defer func() { _ = engine.Close() }()

	record := MemoryRecord{
		Type:    MemoryTypeDecision,
		Layer:   LayerSession,
		Content: "ttl test",
	}

	saved, err := engine.Store(ctx, record)
	if err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	if saved.TTL != engine.config.DefaultTTL {
		t.Errorf("TTL = %v, want %v", saved.TTL, engine.config.DefaultTTL)
	}
}

// TestEngineStore_PreservesExplicitTTL verifies that Store does not
// override an explicitly set TTL.
func TestEngineStore_PreservesExplicitTTL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine := newEngineWithMocks(t)
	defer func() { _ = engine.Close() }()

	customTTL := 30 * time.Minute
	record := MemoryRecord{
		Type:    MemoryTypeDecision,
		Layer:   LayerSession,
		Content: "custom ttl",
		TTL:     customTTL,
	}

	saved, err := engine.Store(ctx, record)
	if err != nil {
		t.Fatalf("Store() error: %v", err)
	}
	if saved.TTL != customTTL {
		t.Errorf("TTL = %v, want %v", saved.TTL, customTTL)
	}
}

// TestEngineStore_UnknownLayer verifies that Store returns an error
// for a layer without an associated store.
func TestEngineStore_UnknownLayer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine := newEngineWithMocks(t)
	defer func() { _ = engine.Close() }()

	record := MemoryRecord{
		Type:    MemoryTypeDecision,
		Layer:   MemoryLayer("nonexistent"),
		Content: "should fail",
	}

	_, err := engine.Store(ctx, record)
	if err == nil {
		t.Fatal("expected error for unknown layer, got nil")
	}
}

// TestEngineRetrieve_GetsRecord verifies that Retrieve returns a previously
// stored record.
func TestEngineRetrieve_GetsRecord(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine := newEngineWithMocks(t)
	defer func() { _ = engine.Close() }()

	saved, err := engine.Store(ctx, recordAt("", LayerProject))
	if err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	got, err := engine.Retrieve(ctx, saved.ID, LayerProject)
	if err != nil {
		t.Fatalf("Retrieve() error: %v", err)
	}
	if got.ID != saved.ID {
		t.Errorf("ID = %q, want %q", got.ID, saved.ID)
	}
}

// TestEngineRetrieve_UnknownLayer verifies that Retrieve returns an error
// for a layer without a store.
func TestEngineRetrieve_UnknownLayer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine := newEngineWithMocks(t)
	defer func() { _ = engine.Close() }()

	_, err := engine.Retrieve(ctx, "any-id", MemoryLayer("unknown"))
	if err == nil {
		t.Fatal("expected error for unknown layer, got nil")
	}
}

// TestEngineRetrieve_CustomGetFunc verifies that Retrieve delegates to
// the store's Get function through the MockStore.
func TestEngineRetrieve_CustomGetFunc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mock := &MockStore{
		GetFunc: func(_ context.Context, id string) (*MemoryRecord, error) {
			return &MemoryRecord{ID: id, Content: "from-mock"}, nil
		},
	}

	engine := newEngineWithMocks(t, WithStore(LayerSession, mock))
	defer func() { _ = engine.Close() }()

	got, err := engine.Retrieve(ctx, "test-id", LayerSession)
	if err != nil {
		t.Fatalf("Retrieve() error: %v", err)
	}
	if got.Content != "from-mock" {
		t.Errorf("Content = %q, want %q", got.Content, "from-mock")
	}
}

// TestEngineSearch_AcrossLayers verifies that Search queries all layers
// and returns combined results.
func TestEngineSearch_AcrossLayers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mockSession := &MockStore{
		SearchFunc: func(_ context.Context, _ string, _ SearchOptions) ([]MemoryRecord, error) {
			return []MemoryRecord{
				{ID: "s1", Layer: LayerSession, Content: "session record", Priority: 10},
			}, nil
		},
	}

	mockProject := &MockStore{
		SearchFunc: func(_ context.Context, _ string, _ SearchOptions) ([]MemoryRecord, error) {
			return []MemoryRecord{
				{ID: "p1", Layer: LayerProject, Content: "project record", Priority: 30},
			}, nil
		},
	}

	engine := newEngineWithMocks(t,
		WithStore(LayerSession, mockSession),
		WithStore(LayerProject, mockProject),
	)
	defer func() { _ = engine.Close() }()

	results, err := engine.Search(ctx, "test", SearchOptions{
		Layers: []MemoryLayer{LayerSession, LayerProject},
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Project (priority 30) should come before Session (priority 10).
	if results[0].Layer != LayerProject {
		t.Errorf("expected project result first, got %s", results[0].Layer)
	}
}

// TestEngineSearch_UsesAllLayersByDefault verifies that when no layers
// are specified, all layers are searched.
func TestEngineSearch_UsesAllLayersByDefault(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	searchCount := 0
	mock := &MockStore{
		SearchFunc: func(_ context.Context, _ string, _ SearchOptions) ([]MemoryRecord, error) {
			searchCount++
			return []MemoryRecord{}, nil
		},
	}

	engine := newEngineWithMocks(t,
		WithStore(LayerSession, mock),
		WithStore(LayerProject, mock),
		WithStore(LayerGlobal, mock),
		WithStore(LayerWorkspace, mock),
		WithStore(LayerTemp, mock),
	)
	defer func() { _ = engine.Close() }()

	_, err := engine.Search(ctx, "query", SearchOptions{}) // No layers specified.
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if searchCount != 5 {
		t.Errorf("expected 5 layer searches, got %d", searchCount)
	}
}

// TestEngineSearch_FilterByType verifies that Search passes type filters.
func TestEngineSearch_FilterByType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	capturedOpts := SearchOptions{}
	mock := &MockStore{
		SearchFunc: func(_ context.Context, _ string, opts SearchOptions) ([]MemoryRecord, error) {
			capturedOpts = opts
			return []MemoryRecord{}, nil
		},
	}

	engine := newEngineWithMocks(t, WithStore(LayerSession, mock))
	defer func() { _ = engine.Close() }()

	_, _ = engine.Search(ctx, "query", SearchOptions{
		Types: []MemoryType{MemoryTypeBug, MemoryTypePattern},
		Limit: 5,
	})
	if len(capturedOpts.Types) != 2 {
		t.Errorf("Types len = %d, want 2", len(capturedOpts.Types))
	}
	if capturedOpts.Limit != 5 {
		t.Errorf("Limit = %d, want 5", capturedOpts.Limit)
	}
}

// TestEngineSearch_HandleLayerError verifies that Search continues even
// if one layer's store returns an error.
func TestEngineSearch_HandleLayerError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	goodMock := &MockStore{
		SearchFunc: func(_ context.Context, _ string, _ SearchOptions) ([]MemoryRecord, error) {
			return []MemoryRecord{{ID: "good", Content: "ok"}}, nil
		},
	}

	badMock := &MockStore{
		SearchFunc: func(_ context.Context, _ string, _ SearchOptions) ([]MemoryRecord, error) {
			return nil, fmt.Errorf("search failed")
		},
	}

	engine := newEngineWithMocks(t,
		WithStore(LayerSession, badMock),
		WithStore(LayerProject, goodMock),
	)
	defer func() { _ = engine.Close() }()

	results, err := engine.Search(ctx, "query", SearchOptions{
		Layers: []MemoryLayer{LayerSession, LayerProject},
	})
	if err != nil {
		t.Fatalf("Search() should not fail on partial error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result from good layer, got %d", len(results))
	}
}

// TestEngineSearch_LimitResults verifies that Search respects the limit.
func TestEngineSearch_LimitResults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mock := &MockStore{
		SearchFunc: func(_ context.Context, _ string, _ SearchOptions) ([]MemoryRecord, error) {
			records := make([]MemoryRecord, 10)
			for i := range records {
				records[i] = MemoryRecord{ID: fmt.Sprintf("r%d", i), Layer: LayerSession, Priority: i}
			}
			return records, nil
		},
	}

	engine := newEngineWithMocks(t, WithStore(LayerSession, mock))
	defer func() { _ = engine.Close() }()

	results, err := engine.Search(ctx, "query", SearchOptions{
		Layers: []MemoryLayer{LayerSession},
		Limit:  3,
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}
}

// TestEnginePromote_PromotesRecord verifies that Promote moves a record
// from a source layer to a target layer.
func TestEnginePromote_PromotesRecord(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	sourceRecord := &MemoryRecord{
		ID:      "record-1",
		Type:    MemoryTypeDecision,
		Layer:   LayerSession,
		Content: "promote me",
	}

	mockSession := &MockStore{
		GetFunc: func(_ context.Context, id string) (*MemoryRecord, error) {
			if id == "record-1" {
				return sourceRecord, nil
			}
			return nil, fmt.Errorf("not found")
		},
		DeleteFunc: func(_ context.Context, id string) error {
			return nil
		},
	}

	var savedToProject MemoryRecord
	mockProject := &MockStore{
		SaveFunc: func(_ context.Context, record MemoryRecord) (*MemoryRecord, error) {
			record.ID = "promoted-1"
			record.Layer = LayerProject
			savedToProject = record
			return &record, nil
		},
	}

	engine := newEngineWithMocks(t,
		WithStore(LayerSession, mockSession),
		WithStore(LayerProject, mockProject),
	)
	engine.config.SnapshotOnPromote = false // Disable snapshot for test.
	defer func() { _ = engine.Close() }()

	promoted, err := engine.Promote(ctx, "record-1", LayerSession, LayerProject)
	if err != nil {
		t.Fatalf("Promote() error: %v", err)
	}
	if promoted.ID != "promoted-1" {
		t.Errorf("ID = %q, want %q", promoted.ID, "promoted-1")
	}
	if savedToProject.Content != "promote me" {
		t.Errorf("saved content = %q, want %q", savedToProject.Content, "promote me")
	}
}

// TestEnginePromote_SourceLayerNotFound verifies that Promote returns an
// error when the source layer has no store.
func TestEnginePromote_SourceLayerNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine := newEngineWithMocks(t)
	defer func() { _ = engine.Close() }()

	// Remove the Temp layer's store (or use a non-existent one).
	delete(engine.stores, LayerTemp)

	_, err := engine.Promote(ctx, "any-id", LayerTemp, LayerSession)
	if err == nil {
		t.Fatal("expected error for missing source layer, got nil")
	}
}

// TestEnginePromote_TargetLayerNotFound verifies that Promote returns an
// error when the target layer has no store.
func TestEnginePromote_TargetLayerNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mockSource := &MockStore{
		GetFunc: func(_ context.Context, id string) (*MemoryRecord, error) {
			return &MemoryRecord{ID: id, Content: "data"}, nil
		},
	}

	engine := newEngineWithMocks(t, WithStore(LayerSession, mockSource))
	// Remove target layer's store.
	delete(engine.stores, LayerProject)
	defer func() { _ = engine.Close() }()

	_, err := engine.Promote(ctx, "any-id", LayerSession, LayerProject)
	if err == nil {
		t.Fatal("expected error for missing target layer, got nil")
	}
}

// TestEnginePromote_RecordNotFound verifies that Promote returns an error
// when the record is not found in the source layer.
func TestEnginePromote_RecordNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mockSession := &MockStore{
		GetFunc: func(_ context.Context, id string) (*MemoryRecord, error) {
			return nil, fmt.Errorf("record %s not found", id)
		},
	}

	engine := newEngineWithMocks(t,
		WithStore(LayerSession, mockSession),
		WithStore(LayerProject, &MockStore{}),
	)
	engine.config.SnapshotOnPromote = false
	defer func() { _ = engine.Close() }()

	_, err := engine.Promote(ctx, "missing-id", LayerSession, LayerProject)
	if err == nil {
		t.Fatal("expected error for missing record, got nil")
	}
}

// TestEngineIndex_DelegatesToStore verifies that Index delegates to the
// store's Index method.
func TestEngineIndex_DelegatesToStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	indexCalled := false
	mock := &MockStore{
		IndexFunc: func(_ context.Context, _ MemoryRecord) error {
			indexCalled = true
			return nil
		},
	}

	engine := newEngineWithMocks(t, WithStore(LayerSession, mock))
	defer func() { _ = engine.Close() }()

	err := engine.Index(ctx, MemoryRecord{Layer: LayerSession, Content: "index me"})
	if err != nil {
		t.Fatalf("Index() error: %v", err)
	}
	if !indexCalled {
		t.Error("Index was not delegated to store")
	}
}

// TestEngineIndex_UnknownLayer verifies that Index returns an error for
// an unknown layer.
func TestEngineIndex_UnknownLayer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine := newEngineWithMocks(t)
	defer func() { _ = engine.Close() }()

	err := engine.Index(ctx, MemoryRecord{Layer: MemoryLayer("missing")})
	if err == nil {
		t.Fatal("expected error for unknown layer, got nil")
	}
}

// TestEnginePrune_DelegatesToStore verifies that Prune delegates to the
// store and returns the count.
func TestEnginePrune_DelegatesToStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mock := &MockStore{
		PruneFunc: func(_ context.Context) (int, error) {
			return 5, nil
		},
	}

	engine := newEngineWithMocks(t, WithStore(LayerSession, mock))
	defer func() { _ = engine.Close() }()

	count, err := engine.Prune(ctx, LayerSession)
	if err != nil {
		t.Fatalf("Prune() error: %v", err)
	}
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}
}

// TestEnginePrune_UnknownLayer verifies that Prune returns an error for
// an unknown layer.
func TestEnginePrune_UnknownLayer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine := newEngineWithMocks(t)
	defer func() { _ = engine.Close() }()

	_, err := engine.Prune(ctx, MemoryLayer("nonexistent"))
	if err == nil {
		t.Fatal("expected error for unknown layer, got nil")
	}
}

// TestEngineGetLayerStats_ReturnsStats verifies that GetLayerStats returns
// statistics for all layers.
func TestEngineGetLayerStats_ReturnsStats(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mockSession := &MockStore{
		StatsFunc: func(_ context.Context) (LayerStats, error) {
			return LayerStats{Name: "session", Count: 10}, nil
		},
	}

	engine := newEngineWithMocks(t, WithStore(LayerSession, mockSession))
	defer func() { _ = engine.Close() }()

	stats := engine.GetLayerStats(ctx)
	if len(stats) == 0 {
		t.Fatal("expected non-empty stats map")
	}
	if s, ok := stats[LayerSession]; ok {
		if s.Count != 10 {
			t.Errorf("session count = %d, want 10", s.Count)
		}
	} else {
		t.Error("LayerSession not found in stats")
	}
}

// TestEngineGetLayerStats_HandlesError verifies that GetLayerStats
// continues when one store returns an error.
func TestEngineGetLayerStats_HandlesError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mockBad := &MockStore{
		StatsFunc: func(_ context.Context) (LayerStats, error) {
			return LayerStats{}, fmt.Errorf("stats error")
		},
	}

	mockGood := &MockStore{
		StatsFunc: func(_ context.Context) (LayerStats, error) {
			return LayerStats{Name: "project", Count: 42}, nil
		},
	}

	engine := newEngineWithMocks(t,
		WithStore(LayerSession, mockBad),
		WithStore(LayerProject, mockGood),
	)
	defer func() { _ = engine.Close() }()

	stats := engine.GetLayerStats(ctx)
	// The error store should be skipped.
	if s, ok := stats[LayerProject]; ok {
		if s.Count != 42 {
			t.Errorf("project count = %d, want 42", s.Count)
		}
	} else {
		t.Error("LayerProject should have stats")
	}
}

// TestNewEngine_WithStoreOption verifies that WithStore properly injects
// a custom store for a specific layer.
func TestNewEngine_WithStoreOption(t *testing.T) {
	t.Parallel()

	customStore := &MockStore{
		StatsFunc: func(_ context.Context) (LayerStats, error) {
			return LayerStats{Name: "custom", Count: 99}, nil
		},
	}

	cfg := EngineConfig{DataDir: t.TempDir(), AutoPrune: false}
	engine, err := NewEngine(WithConfig(cfg), WithStore(LayerGlobal, customStore))
	if err != nil {
		t.Fatalf("NewEngine error: %v", err)
	}
	defer func() { _ = engine.Close() }()

	ctx := context.Background()
	stats := engine.GetLayerStats(ctx)
	if s, ok := stats[LayerGlobal]; ok {
		if s.Count != 99 {
			t.Errorf("custom store count = %d, want 99", s.Count)
		}
	} else {
		t.Error("LayerGlobal not found in stats (custom store not used)")
	}
}
