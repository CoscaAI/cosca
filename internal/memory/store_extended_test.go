package memory

import (
	"context"
	"os"
	"testing"

	"github.com/rs/zerolog"
)

func TestFileStore_SearchFallback(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	ctx := context.Background()

	store, err := NewFileStore(dataDir, LayerSession, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewFileStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Save records
	store.Save(ctx, MemoryRecord{
		ID:       "rec-1",
		Type:     MemoryTypeDecision,
		Layer:    LayerSession,
		Content:  "Use hexagonal architecture for new services",
		Priority: 10,
	})
	store.Save(ctx, MemoryRecord{
		ID:       "rec-2",
		Type:     MemoryTypeBug,
		Layer:    LayerSession,
		Content:  "Race condition found in payment module",
		Priority: 5,
	})
	store.Save(ctx, MemoryRecord{
		ID:       "rec-3",
		Type:     MemoryTypePattern,
		Layer:    LayerSession,
		Content:  "Always use context.WithTimeout for external calls",
		Priority: 8,
	})

	// Search by keyword
	results, err := store.searchFallback(ctx, "hexagonal", SearchOptions{})
	if err != nil {
		t.Fatalf("searchFallback error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "rec-1" {
		t.Errorf("expected rec-1, got %s", results[0].ID)
	}

	// Search by another keyword
	results, err = store.searchFallback(ctx, "race condition", SearchOptions{})
	if err != nil {
		t.Fatalf("searchFallback error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "rec-2" {
		t.Errorf("expected rec-2, got %s", results[0].ID)
	}
}

func TestFileStore_SearchFallback_EmptyQuery(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	ctx := context.Background()

	store, err := NewFileStore(dataDir, LayerSession, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewFileStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	store.Save(ctx, MemoryRecord{
		ID:      "rec-1",
		Type:    MemoryTypeDecision,
		Layer:   LayerSession,
		Content: "Record one",
	})
	store.Save(ctx, MemoryRecord{
		ID:      "rec-2",
		Type:    MemoryTypeBug,
		Layer:   LayerSession,
		Content: "Record two",
	})

	// Empty query returns all records sorted by priority
	results, err := store.searchFallback(ctx, "", SearchOptions{})
	if err != nil {
		t.Fatalf("searchFallback error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestFileStore_SearchFallback_TypeFilter(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	ctx := context.Background()

	store, err := NewFileStore(dataDir, LayerSession, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewFileStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	store.Save(ctx, MemoryRecord{
		ID:      "rec-1",
		Type:    MemoryTypeDecision,
		Layer:   LayerSession,
		Content: "Architecture decision about microservices",
	})
	store.Save(ctx, MemoryRecord{
		ID:      "rec-2",
		Type:    MemoryTypeBug,
		Layer:   LayerSession,
		Content: "Bug found in architecture validation",
	})

	// Filter by Bug type only
	results, err := store.searchFallback(ctx, "architecture", SearchOptions{
		Types: []MemoryType{MemoryTypeBug},
	})
	if err != nil {
		t.Fatalf("searchFallback error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 bug result, got %d", len(results))
	}
	if results[0].Type != MemoryTypeBug {
		t.Errorf("expected Bug type, got %s", results[0].Type)
	}

	// Filter by Decision type
	results, err = store.searchFallback(ctx, "architecture", SearchOptions{
		Types: []MemoryType{MemoryTypeDecision},
	})
	if err != nil {
		t.Fatalf("searchFallback error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 decision result, got %d", len(results))
	}
	if results[0].Type != MemoryTypeDecision {
		t.Errorf("expected Decision type, got %s", results[0].Type)
	}

	// Filter by both types
	results, err = store.searchFallback(ctx, "architecture", SearchOptions{
		Types: []MemoryType{MemoryTypeDecision, MemoryTypeBug},
	})
	if err != nil {
		t.Fatalf("searchFallback error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results with both types, got %d", len(results))
	}
}

func TestFileStore_SearchFallback_Limit(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	ctx := context.Background()

	store, err := NewFileStore(dataDir, LayerSession, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewFileStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Save multiple matching records
	for i := 0; i < 5; i++ {
		id := "rec-" + string(rune('a'+i))
		store.Save(ctx, MemoryRecord{
			ID:       id,
			Type:     MemoryTypeDecision,
			Layer:    LayerSession,
			Content:  "Test content with matching keyword",
			Priority: i,
		})
	}

	results, err := store.searchFallback(ctx, "keyword", SearchOptions{Limit: 3})
	if err != nil {
		t.Fatalf("searchFallback error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results with limit, got %d", len(results))
	}
}

func TestFileStore_SearchFallback_NoMatch(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	ctx := context.Background()

	store, err := NewFileStore(dataDir, LayerSession, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewFileStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	store.Save(ctx, MemoryRecord{
		ID:      "rec-1",
		Type:    MemoryTypeDecision,
		Layer:   LayerSession,
		Content: "Some unrelated content",
	})

	results, err := store.searchFallback(ctx, "nonexistent", SearchOptions{})
	if err != nil {
		t.Fatalf("searchFallback error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFileStore_SearchFallback_ReadDirError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a FileStore with non-existent directory by manually constructing it
	store := &FileStore{
		logger: zerolog.Nop(),
		dir:    "/nonexistent/path/that/does/not/exist",
		layer:  LayerSession,
	}

	// searchFallback should return error when directory cannot be read
	// But it reads from the dir field; first os.ReadDir will fail
	results, err := store.searchFallback(ctx, "test", SearchOptions{})
	if err == nil {
		t.Errorf("expected error for non-existent directory, but got %d results", len(results))
	}
}

func TestFileStore_SearchFallback_EmptyDir(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	ctx := context.Background()

	store, err := NewFileStore(dataDir, LayerSession, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewFileStore error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// No records saved — search should return empty results
	results, err := store.searchFallback(ctx, "anything", SearchOptions{})
	if err != nil {
		t.Fatalf("searchFallback error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results in empty store, got %d", len(results))
	}
}

// Ensure temp dir is cleaned up
func init() {
	// Cleanup any leftover test data
	_ = os.RemoveAll("/tmp/memory-test")
}
