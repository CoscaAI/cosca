package memory

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func TestStoreInterface(t *testing.T) {
	t.Parallel()
	// Verify that MockStore satisfies Store interface
	var store Store = &MockStore{}
	_ = store
}

func TestMockStoreDefaults(t *testing.T) {
	t.Parallel()
	store := &MockStore{}
	ctx := context.Background()

	record, err := store.Save(ctx, MemoryRecord{Type: MemoryTypeDecision, Layer: LayerSession})
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if record.ID != "mock-id" {
		t.Errorf("ID = %q, want %q", record.ID, "mock-id")
	}

	got, err := store.Get(ctx, "test-id")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.ID != "test-id" {
		t.Errorf("ID = %q", got.ID)
	}

	if err := store.Delete(ctx, "id"); err != nil {
		t.Errorf("Delete error: %v", err)
	}

	results, err := store.Search(ctx, "query", SearchOptions{})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("results len = %d", len(results))
	}

	if err := store.Index(ctx, MemoryRecord{}); err != nil {
		t.Errorf("Index error: %v", err)
	}

	count, err := store.Prune(ctx)
	if err != nil {
		t.Fatalf("Prune error: %v", err)
	}
	if count != 0 {
		t.Errorf("prune count = %d", count)
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats error: %v", err)
	}
	if stats.Count != 0 {
		t.Errorf("stats count = %d", stats.Count)
	}

	if err := store.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
}

func TestSearchOptionsDefaults(t *testing.T) {
	t.Parallel()
	opts := SearchOptions{}
	if opts.Limit != 0 {
		t.Errorf("Limit = %d", opts.Limit)
	}
	if opts.MinScore != 0 {
		t.Errorf("MinScore = %f", opts.MinScore)
	}
}

func TestSearchOptionsWithFilters(t *testing.T) {
	t.Parallel()
	opts := SearchOptions{
		Types:  []MemoryType{MemoryTypeDecision},
		Layers: []MemoryLayer{LayerSession},
		Limit:  10,
	}
	if len(opts.Types) != 1 {
		t.Errorf("Types = %d", len(opts.Types))
	}
}

func TestIndexEntry(t *testing.T) {
	t.Parallel()
	entry := IndexEntry{
		ID:      "entry-1",
		Content: "test content",
		Score:   0.95,
	}
	if entry.ID != "entry-1" {
		t.Errorf("ID = %q", entry.ID)
	}
	if entry.Score != 0.95 {
		t.Errorf("Score = %f", entry.Score)
	}
}

func TestLayerStatsStruct(t *testing.T) {
	t.Parallel()
	stats := LayerStats{
		Name:            "session",
		Count:           5,
		TotalSize:       1000,
		HighestPriority: 20,
	}
	if stats.Name != "session" {
		t.Errorf("Name = %q", stats.Name)
	}
}

// TestValidateMemoryID verifies the memory ID validator rejects traversal.
func TestValidateMemoryID(t *testing.T) {
	valid := []string{"abc", "abc-123", "abc_123", "abc.123", "550e8400-e29b-41d4-a716-446655440000"}
	invalid := []string{"../etc/passwd", "../../../x", "a/b", "a\\b", "..", "a..b/", "", "a b"}

	for _, id := range valid {
		if err := ValidateMemoryID(id); err != nil {
			t.Errorf("ValidateMemoryID(%q) = %v, want nil", id, err)
		}
	}
	for _, id := range invalid {
		if err := ValidateMemoryID(id); err == nil {
			t.Errorf("ValidateMemoryID(%q) = nil, want error", id)
		}
	}
}

// TestStoreGet_PathTraversalRejected ensures Get rejects traversal IDs.
func TestStoreGet_PathTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir, LayerSession, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	defer fs.Close()

	if _, err := fs.Get(context.Background(), "../../etc/passwd"); err == nil {
		t.Error("Get with traversal ID: expected error, got nil")
	}
}

// TestStoreDelete_PathTraversalRejected ensures Delete rejects traversal IDs.
func TestStoreDelete_PathTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir, LayerSession, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	defer fs.Close()

	if err := fs.Delete(context.Background(), "../../../tmp/pwn"); err == nil {
		t.Error("Delete with traversal ID: expected error, got nil")
	}
}

// TestStoreLegitimateIDStillWorks verifies normal UUID ids still round-trip.
func TestStoreLegitimateIDStillWorks(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir, LayerSession, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	defer fs.Close()

	rec := MemoryRecord{
		ID:      "550e8400-e29b-41d4-a716-446655440000",
		Type:    MemoryTypeDecision,
		Layer:   LayerSession,
		Content: "test",
		Scope:   "test",
	}
	saved, err := fs.Save(context.Background(), rec)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := fs.Get(context.Background(), saved.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Content != "test" {
		t.Errorf("Content = %q, want test", got.Content)
	}
}

// seedAgentRecords adds three records to the given store: one owned by
// agent "alpha", one by agent "beta", and one with no agent at all.
func seedAgentRecords(t *testing.T, ctx context.Context, fs *FileStore) {
	t.Helper()
	recs := []MemoryRecord{
		{ID: "a1", Type: MemoryTypeDecision, Layer: LayerSession, Content: "alpha plan", Metadata: map[string]string{"agent": "alpha"}},
		{ID: "a2", Type: MemoryTypeDecision, Layer: LayerSession, Content: "beta plan", Metadata: map[string]string{"agent": "beta"}},
		{ID: "a3", Type: MemoryTypeDecision, Layer: LayerSession, Content: "shared plan"},
	}
	for _, r := range recs {
		if _, err := fs.Save(ctx, r); err != nil {
			t.Fatalf("Save(%s): %v", r.ID, err)
		}
	}
}

// idsOf returns the sorted set of record IDs returned by a search.
func idsOf(recs []MemoryRecord) []string {
	ids := make([]string, 0, len(recs))
	for _, r := range recs {
		ids = append(ids, r.ID)
	}
	return ids
}

// idsOfEntries returns the set of index-entry IDs returned by a search.
func idsOfEntries(entries []IndexEntry) []string {
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.ID)
	}
	return ids
}

// contains reports whether want is present in got.
func contains(got []string, want string) bool {
	for _, id := range got {
		if id == want {
			return true
		}
	}
	return false
}

// TestSQLiteIndex_SearchAgentFilter verifies agent scoping on the FTS MATCH
// search path (A7): AgentFilter="alpha" returns only alpha + no-agent
// records, never beta; an empty AgentFilter returns everything.
func TestSQLiteIndex_SearchAgentFilter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "index.db")
	idx, err := NewSQLiteIndex(dbPath, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()

	ctx := context.Background()
	recs := []MemoryRecord{
		{ID: "a1", Type: MemoryTypeDecision, Layer: LayerSession, Content: "alpha plan", Metadata: map[string]string{"agent": "alpha"}},
		{ID: "a2", Type: MemoryTypeDecision, Layer: LayerSession, Content: "beta plan", Metadata: map[string]string{"agent": "beta"}},
		{ID: "a3", Type: MemoryTypeDecision, Layer: LayerSession, Content: "shared plan"},
	}
	for _, r := range recs {
		if err := idx.AddRecord(ctx, r); err != nil {
			t.Fatalf("AddRecord(%s): %v", r.ID, err)
		}
	}

	// Search all records with query "plan".
	all, err := idx.Search(ctx, "plan", SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Search all = %d records, want 3", len(all))
	}

	// AgentFilter="alpha": alpha + no-agent, never beta.
	alpha, err := idx.Search(ctx, "plan", SearchOptions{AgentFilter: "alpha", Limit: 10})
	if err != nil {
		t.Fatalf("Search alpha: %v", err)
	}
	alphaIDs := idsOfEntries(alpha)
	if contains(alphaIDs, "a2") {
		t.Errorf("AgentFilter=alpha leaked beta record: %v", alphaIDs)
	}
	if !contains(alphaIDs, "a1") || !contains(alphaIDs, "a3") {
		t.Errorf("AgentFilter=alpha should return a1 and a3, got %v", alphaIDs)
	}

	// AgentFilter="beta": beta + no-agent, never alpha.
	beta, err := idx.Search(ctx, "plan", SearchOptions{AgentFilter: "beta", Limit: 10})
	if err != nil {
		t.Fatalf("Search beta: %v", err)
	}
	betaIDs := idsOfEntries(beta)
	if contains(betaIDs, "a1") {
		t.Errorf("AgentFilter=beta leaked alpha record: %v", betaIDs)
	}
	if !contains(betaIDs, "a2") || !contains(betaIDs, "a3") {
		t.Errorf("AgentFilter=beta should return a2 and a3, got %v", betaIDs)
	}
}

// TestSQLiteIndex_SearchEmptyQueryAgentFilter verifies agent scoping on the
// empty-query branch (recent records).
func TestSQLiteIndex_SearchEmptyQueryAgentFilter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "index.db")
	idx, err := NewSQLiteIndex(dbPath, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()

	ctx := context.Background()
	recs := []MemoryRecord{
		{ID: "a1", Type: MemoryTypeDecision, Layer: LayerSession, Content: "alpha plan", Metadata: map[string]string{"agent": "alpha"}},
		{ID: "a2", Type: MemoryTypeDecision, Layer: LayerSession, Content: "beta plan", Metadata: map[string]string{"agent": "beta"}},
		{ID: "a3", Type: MemoryTypeDecision, Layer: LayerSession, Content: "shared plan"},
	}
	for _, r := range recs {
		if err := idx.AddRecord(ctx, r); err != nil {
			t.Fatalf("AddRecord(%s): %v", r.ID, err)
		}
	}

	all, err := idx.Search(ctx, "", SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Search all = %d records, want 3", len(all))
	}

	alpha, err := idx.Search(ctx, "", SearchOptions{AgentFilter: "alpha", Limit: 10})
	if err != nil {
		t.Fatalf("Search alpha: %v", err)
	}
	alphaIDs := idsOfEntries(alpha)
	if contains(alphaIDs, "a2") {
		t.Errorf("AgentFilter=alpha leaked beta record: %v", alphaIDs)
	}
	if !contains(alphaIDs, "a1") || !contains(alphaIDs, "a3") {
		t.Errorf("AgentFilter=alpha should return a1 and a3, got %v", alphaIDs)
	}
}

// TestFileStore_SearchAgentFilter verifies agent scoping end-to-end through
// FileStore.Save/Search, exercising the agentOf helper on the index insert.
func TestFileStore_SearchAgentFilter(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir, LayerSession, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	defer fs.Close()

	ctx := context.Background()
	seedAgentRecords(t, ctx, fs)

	all, err := fs.Search(ctx, "plan", SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Search all = %d records, want 3", len(all))
	}

	alpha, err := fs.Search(ctx, "plan", SearchOptions{AgentFilter: "alpha", Limit: 10})
	if err != nil {
		t.Fatalf("Search alpha: %v", err)
	}
	alphaIDs := idsOf(alpha)
	if contains(alphaIDs, "a2") {
		t.Errorf("AgentFilter=alpha leaked beta record: %v", alphaIDs)
	}
	if !contains(alphaIDs, "a1") || !contains(alphaIDs, "a3") {
		t.Errorf("AgentFilter=alpha should return a1 and a3, got %v", alphaIDs)
	}

	beta, err := fs.Search(ctx, "plan", SearchOptions{AgentFilter: "beta", Limit: 10})
	if err != nil {
		t.Fatalf("Search beta: %v", err)
	}
	betaIDs := idsOf(beta)
	if contains(betaIDs, "a1") {
		t.Errorf("AgentFilter=beta leaked alpha record: %v", betaIDs)
	}
	if !contains(betaIDs, "a2") || !contains(betaIDs, "a3") {
		t.Errorf("AgentFilter=beta should return a2 and a3, got %v", betaIDs)
	}
}

// TestFileStore_SearchFallback_AgentFilter verifies agent scoping on the
// linear-scan fallback path (index unavailable).
func TestFileStore_SearchFallback_AgentFilter(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir, LayerSession, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	defer fs.Close()

	ctx := context.Background()
	seedAgentRecords(t, ctx, fs)

	all, err := fs.searchFallback(ctx, "plan", SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("searchFallback all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("searchFallback all = %d records, want 3", len(all))
	}

	alpha, err := fs.searchFallback(ctx, "plan", SearchOptions{AgentFilter: "alpha", Limit: 10})
	if err != nil {
		t.Fatalf("searchFallback alpha: %v", err)
	}
	alphaIDs := idsOf(alpha)
	if contains(alphaIDs, "a2") {
		t.Errorf("AgentFilter=alpha leaked beta record: %v", alphaIDs)
	}
	if !contains(alphaIDs, "a1") || !contains(alphaIDs, "a3") {
		t.Errorf("AgentFilter=alpha should return a1 and a3, got %v", alphaIDs)
	}
}

// TestAgentOf verifies the helper prefers the dedicated Agent field over
// Metadata["agent"] and falls back to Metadata for legacy records.
func TestAgentOf(t *testing.T) {
	if got := agentOf(&MemoryRecord{Agent: "dedicated", Metadata: map[string]string{"agent": "legacy"}}); got != "dedicated" {
		t.Errorf("agentOf = %q, want dedicated field to win", got)
	}
	if got := agentOf(&MemoryRecord{Metadata: map[string]string{"agent": "legacy"}}); got != "legacy" {
		t.Errorf("agentOf = %q, want metadata fallback", got)
	}
	if got := agentOf(&MemoryRecord{}); got != "" {
		t.Errorf("agentOf = %q, want empty", got)
	}
	if got := agentOf(&MemoryRecord{Metadata: map[string]string{}}); got != "" {
		t.Errorf("agentOf = %q, want empty for empty metadata", got)
	}
}

// TestFileStore_AgentFieldRoundTrip verifies the dedicated Agent field
// round-trips through YAML frontmatter.
func TestFileStore_AgentFieldRoundTrip(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir, LayerSession, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	defer fs.Close()

	ctx := context.Background()
	rec := MemoryRecord{ID: "rt1", Type: MemoryTypeDecision, Layer: LayerSession, Content: "body", Agent: "cosca-kernel"}
	saved, err := fs.Save(ctx, rec)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.Agent != "cosca-kernel" {
		t.Errorf("saved.Agent = %q, want cosca-kernel", saved.Agent)
	}
	got, err := fs.Get(ctx, "rt1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Agent != "cosca-kernel" {
		t.Errorf("round-trip Agent = %q, want cosca-kernel", got.Agent)
	}
}
