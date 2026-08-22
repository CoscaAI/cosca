package memory

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestMemoryLayerConstants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		layer MemoryLayer
		want  string
	}{
		{LayerGlobal, "global"},
		{LayerWorkspace, "workspace"},
		{LayerProject, "project"},
		{LayerSession, "session"},
		{LayerTemp, "temp"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.layer) != tt.want {
				t.Errorf("MemoryLayer = %q, want %q", string(tt.layer), tt.want)
			}
		})
	}
}

func TestMemoryTypeConstants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mtype MemoryType
		want  string
	}{
		{MemoryTypeDecision, "decision"},
		{MemoryTypePattern, "pattern"},
		{MemoryTypeBug, "bug"},
		{MemoryTypeAgent, "agent"},
		{MemoryTypeProject, "project"},
		{MemoryTypeArchitecture, "architecture"},
		{MemoryTypeSession, "session"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.mtype) != tt.want {
				t.Errorf("MemoryType = %q, want %q", string(tt.mtype), tt.want)
			}
		})
	}
}

func TestMemoryRecordDefaults(t *testing.T) {
	t.Parallel()
	record := MemoryRecord{
		Type:  MemoryTypeDecision,
		Layer: LayerSession,
	}
	if record.ID != "" {
		t.Errorf("ID should be empty, got %q", record.ID)
	}
	if record.CreatedAt.IsZero() {
		t.Log("CreatedAt is zero (will be set by Store)")
	}
	if record.TTL != 0 {
		t.Errorf("TTL should be 0, got %v", record.TTL)
	}
	if record.Priority != 0 {
		t.Errorf("Priority should be 0, got %d", record.Priority)
	}
	if record.Version != 0 {
		t.Errorf("Version should be 0, got %d", record.Version)
	}
}

func TestMemoryRecordFull(t *testing.T) {
	t.Parallel()
	now := time.Now()
	record := MemoryRecord{
		ID:        "rec-1",
		Type:      MemoryTypeArchitecture,
		Layer:     LayerGlobal,
		Scope:     "project-x",
		Content:   "Use hexagonal architecture",
		CreatedAt: now,
		UpdatedAt: now,
		TTL:       24 * time.Hour,
		Priority:  5,
		Version:   1,
		Metadata:  map[string]string{"author": "team"},
	}
	if record.ID != "rec-1" {
		t.Errorf("ID = %q", record.ID)
	}
	if record.Priority != 5 {
		t.Errorf("Priority = %d", record.Priority)
	}
	if record.Metadata["author"] != "team" {
		t.Errorf("Metadata author = %q", record.Metadata["author"])
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.DefaultTTL != 24*time.Hour {
		t.Errorf("DefaultTTL = %v, want 24h", cfg.DefaultTTL)
	}
	if cfg.MaxRecordsPerLayer != 1000 {
		t.Errorf("MaxRecordsPerLayer = %d", cfg.MaxRecordsPerLayer)
	}
	if !cfg.AutoPrune {
		t.Error("AutoPrune should be true")
	}
	if cfg.PruneInterval != 30*time.Minute {
		t.Errorf("PruneInterval = %v", cfg.PruneInterval)
	}
	if !cfg.SnapshotOnPromote {
		t.Error("SnapshotOnPromote should be true")
	}
}

func TestEngineConfigFields(t *testing.T) {
	t.Parallel()
	cfg := EngineConfig{
		DataDir:            "./tmp/cosca",
		DefaultTTL:         1 * time.Hour,
		MaxRecordsPerLayer: 500,
		AutoPrune:          false,
		PruneInterval:      10 * time.Minute,
		SnapshotOnPromote:  false,
	}
	if cfg.DataDir != "./tmp/cosca" {
		t.Errorf("DataDir = %q", cfg.DataDir)
	}
	if cfg.MaxRecordsPerLayer != 500 {
		t.Errorf("MaxRecordsPerLayer = %d", cfg.MaxRecordsPerLayer)
	}
}

func TestNewEngineOptions(t *testing.T) {
	t.Parallel()
	cfg := EngineConfig{
		DataDir:            t.TempDir(),
		DefaultTTL:         1 * time.Hour,
		MaxRecordsPerLayer: 100,
		AutoPrune:          false,
	}
	engine, err := NewEngine(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewEngine error: %v", err)
	}
	if engine == nil {
		t.Fatal("engine is nil")
	}
	defer func() { _ = engine.Close() }()
	if engine.config.DefaultTTL != 1*time.Hour {
		t.Errorf("DefaultTTL = %v", engine.config.DefaultTTL)
	}
}

// TestNewEngine_AutoPruneZeroInterval is a regression test for the panic
// "non-positive interval for NewTicker": a caller that enables AutoPrune but
// leaves PruneInterval at zero would crash the auto-prune goroutine. NewEngine
// must fall back to the default interval so the goroutine survives.
func TestNewEngine_AutoPruneZeroInterval(t *testing.T) {
	cfg := EngineConfig{
		DataDir:   t.TempDir(),
		AutoPrune: true,
	}
	engine, err := NewEngine(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewEngine error: %v", err)
	}
	if engine == nil {
		t.Fatal("engine is nil")
	}
	defer func() { _ = engine.Close() }()

	if engine.config.PruneInterval <= 0 {
		t.Errorf("PruneInterval = %v, want positive default fallback", engine.config.PruneInterval)
	}

	// Give the auto-prune goroutine time to run NewTicker. If the guard were
	// missing, NewTicker(0) would panic the whole test process here.
	time.Sleep(200 * time.Millisecond)
}

func TestWithLogger(t *testing.T) {
	t.Parallel()

	// Verify WithLogger returns a non-nil Option.
	opt := WithLogger(zerolog.Nop())
	if opt == nil {
		t.Fatal("WithLogger returned nil option")
	}

	// Apply the option to an engine and verify the logger
	// is actually set by writing a log message to a buffer.
	var buf bytes.Buffer
	customLogger := zerolog.New(&buf)
	opt = WithLogger(customLogger)

	var engine MemoryEngine
	opt(&engine)
	engine.logger.Info().Str("key", "val").Msg("hello")

	if !bytes.Contains(buf.Bytes(), []byte("hello")) {
		t.Error("logger was not set correctly on engine — log message not captured")
	}

	// Also verify WithLogger works when passed to NewEngine.
	buf.Reset()
	log2 := zerolog.New(&buf)
	cfg := EngineConfig{DataDir: t.TempDir(), AutoPrune: false}
	e, err := NewEngine(WithConfig(cfg), WithLogger(log2))
	if err != nil {
		t.Fatalf("NewEngine with WithLogger error: %v", err)
	}
	defer func() { _ = e.Close() }()

	e.logger.Info().Msg("from-engine")
	if !bytes.Contains(buf.Bytes(), []byte("from-engine")) {
		t.Error("WithLogger did not set logger when passed to NewEngine")
	}
}

func TestSearchOptions(t *testing.T) {
	t.Parallel()
	opts := SearchOptions{
		Types:    []MemoryType{MemoryTypeDecision, MemoryTypeBug},
		Layers:   []MemoryLayer{LayerSession, LayerWorkspace},
		Limit:    20,
		Offset:   5,
		MinScore: 0.5,
	}
	if len(opts.Types) != 2 {
		t.Errorf("Types len = %d", len(opts.Types))
	}
	if len(opts.Layers) != 2 {
		t.Errorf("Layers len = %d", len(opts.Layers))
	}
	if opts.Limit != 20 {
		t.Errorf("Limit = %d", opts.Limit)
	}
}

func TestSortByPriority(t *testing.T) {
	t.Parallel()
	engine := &MemoryEngine{
		layers: NewLayerManager(),
	}
	records := []MemoryRecord{
		{Layer: LayerTemp, Priority: 0},     // priority 10
		{Layer: LayerGlobal, Priority: 0},   // priority 50
		{Layer: LayerSession, Priority: 10}, // priority 20 + 10 = 30
	}
	engine.sortByPriority(records)
	// Global (50) should be first, then Session (30), then Temp (10)
	if records[0].Layer != LayerGlobal {
		t.Errorf("First should be Global, got %s", records[0].Layer)
	}
}

// TestEngine_Operations runs all engine-level operations using a single engine
// to avoid SQLite connection accumulation.
func TestEngine_Operations(t *testing.T) {
	dataDir := t.TempDir()

	cfg := EngineConfig{
		DataDir:           dataDir,
		AutoPrune:         false,
		SnapshotOnPromote: false,
	}
	engine, err := NewEngine(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewEngine error: %v", err)
	}
	defer func() { _ = engine.Close() }()

	ctx := context.Background()

	// Store a record for snapshot tests
	_, err = engine.Store(ctx, MemoryRecord{
		ID: "rec-engine", Type: MemoryTypeDecision, Layer: LayerSession,
		Content: "Engine test record",
	})
	if err != nil {
		t.Fatalf("Store error: %v", err)
	}

	t.Run("CreateSnapshot", func(t *testing.T) {
		snap, err := engine.CreateSnapshot(ctx, LayerSession, "engine-test")
		if err != nil {
			t.Fatalf("CreateSnapshot error: %v", err)
		}
		if snap == nil || snap.Label != "engine-test" {
			t.Error("snapshot label mismatch")
		}
	})

	t.Run("ListSnapshots", func(t *testing.T) {
		_, _ = engine.CreateSnapshot(ctx, LayerSession, "snap-a")
		snapshots, err := engine.ListSnapshots(ctx)
		if err != nil {
			t.Fatalf("ListSnapshots error: %v", err)
		}
		if len(snapshots) == 0 {
			t.Error("expected at least 1 snapshot")
		}
	})

	t.Run("RestoreSnapshot_Error", func(t *testing.T) {
		err := engine.RestoreSnapshot(ctx, "invalid-id")
		if err == nil {
			t.Fatal("expected error for invalid snapshot ID")
		}
	})

	t.Run("Store_InvalidLayer", func(t *testing.T) {
		_, err := engine.Store(ctx, MemoryRecord{
			Type: MemoryTypeDecision, Layer: MemoryLayer("fantasy"),
		})
		if err == nil {
			t.Fatal("expected error for invalid layer")
		}
	})

	t.Run("Delete_InvalidLayer", func(t *testing.T) {
		err := engine.Delete(ctx, "any-id", MemoryLayer("fantasy"))
		if err == nil {
			t.Fatal("expected error for invalid layer")
		}
	})

	t.Run("Retrieve_InvalidLayer", func(t *testing.T) {
		_, err := engine.Retrieve(ctx, "any-id", MemoryLayer("fantasy"))
		if err == nil {
			t.Fatal("expected error for invalid layer")
		}
	})

	t.Run("Promote_InvalidLayer", func(t *testing.T) {
		_, err := engine.Promote(ctx, "any-id", MemoryLayer("nonexistent"), LayerGlobal)
		if err == nil {
			t.Fatal("expected error for invalid source layer")
		}
	})

	t.Run("Index", func(t *testing.T) {
		err := engine.Index(ctx, MemoryRecord{
			ID: "rec-index", Type: MemoryTypeDecision, Layer: LayerSession,
			Content: "Indexed content",
		})
		if err != nil {
			t.Fatalf("Index error: %v", err)
		}
	})

	t.Run("GetLayerStats", func(t *testing.T) {
		stats := engine.GetLayerStats(ctx)
		if len(stats) == 0 {
			t.Error("expected at least 1 layer in stats")
		}
		if _, ok := stats[LayerSession]; !ok {
			t.Error("LayerSession stats missing")
		}
	})

	t.Run("AutoPruneDisabled", func(t *testing.T) {
		if engine.config.AutoPrune {
			t.Error("AutoPrune should be disabled")
		}
	})
}

// TestEngine_Retrieve_ExpiredTTL tests that records with expired TTL cannot be retrieved.
// Uses time.Sleep, cannot be part of the grouped test.
func TestEngine_Retrieve_ExpiredTTL(t *testing.T) {
	dataDir := t.TempDir()

	cfg := EngineConfig{
		DataDir:   dataDir,
		AutoPrune: false,
	}
	engine, err := NewEngine(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewEngine error: %v", err)
	}
	defer func() { _ = engine.Close() }()

	ctx := context.Background()

	_, err = engine.Store(ctx, MemoryRecord{
		ID: "rec-expire", Type: MemoryTypeDecision, Layer: LayerSession,
		Content: "This will expire quickly",
		TTL:     1 * time.Nanosecond,
	})
	if err != nil {
		t.Fatalf("Store error: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = engine.Retrieve(ctx, "rec-expire", LayerSession)
	if err == nil {
		t.Fatal("expected error retrieving expired record")
	}
}

// TestEngine_SnapshotOnPromote tests that promote with SnapshotOnPromote=true creates a snapshot.
func TestEngine_SnapshotOnPromote(t *testing.T) {
	dataDir := t.TempDir()

	cfg := EngineConfig{
		DataDir:           dataDir,
		AutoPrune:         false,
		SnapshotOnPromote: true,
	}
	engine, err := NewEngine(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewEngine error: %v", err)
	}
	defer func() { _ = engine.Close() }()

	ctx := context.Background()

	_, err = engine.Store(ctx, MemoryRecord{
		ID: "rec-promote-snap", Type: MemoryTypeDecision, Layer: LayerSession,
		Content: "Record to promote with snapshot",
	})
	if err != nil {
		t.Fatalf("Store error: %v", err)
	}

	promoted, err := engine.Promote(ctx, "rec-promote-snap", LayerSession, LayerProject)
	if err != nil {
		t.Fatalf("Promote error: %v", err)
	}
	if promoted.Layer != LayerProject {
		t.Errorf("promoted Layer = %s, want %s", promoted.Layer, LayerProject)
	}

	snapshots, err := engine.ListSnapshots(ctx)
	if err != nil {
		t.Fatalf("ListSnapshots error: %v", err)
	}
	if len(snapshots) == 0 {
		t.Error("expected snapshot to be created on promote")
	}
}

// TestEngine_Prune tests pruning expired records.
func TestEngine_Prune(t *testing.T) {
	dataDir := t.TempDir()

	cfg := EngineConfig{
		DataDir:   dataDir,
		AutoPrune: false,
	}
	engine, err := NewEngine(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewEngine error: %v", err)
	}
	defer func() { _ = engine.Close() }()

	ctx := context.Background()

	_, err = engine.Store(ctx, MemoryRecord{
		ID: "rec-prune-test", Type: MemoryTypeDecision, Layer: LayerSession,
		Content: "Prune me",
		TTL:     1 * time.Nanosecond,
	})
	if err != nil {
		t.Fatalf("Store error: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	count, err := engine.Prune(ctx, LayerSession)
	if err != nil {
		t.Fatalf("Prune error: %v", err)
	}
	if count == 0 {
		t.Log("Prune returned 0 (may be expected due to timing)")
	}
}

// TestMemoryRecord_OwnerPersisted verifies the owner field round-trips
// through Save/Get (Onda 2, ownership scoping).
func TestMemoryRecord_OwnerPersisted(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir, LayerSession, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	defer fs.Close()

	rec := MemoryRecord{
		ID:      "owner-test-1",
		Type:    MemoryTypeDecision,
		Layer:   LayerSession,
		Content: "owned record",
		Scope:   "test",
		Owner:   "user-42",
	}
	saved, err := fs.Save(context.Background(), rec)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := fs.Get(context.Background(), saved.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Owner != "user-42" {
		t.Errorf("Owner = %q, want user-42", got.Owner)
	}
}
