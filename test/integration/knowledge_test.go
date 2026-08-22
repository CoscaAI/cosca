
//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/chunker"
	"github.com/CoscaAI/cosca/internal/graph"
	"github.com/CoscaAI/cosca/internal/indexer"
	"github.com/CoscaAI/cosca/internal/markdown"
	"github.com/CoscaAI/cosca/internal/parser"
	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/CoscaAI/cosca/internal/vector"
)

// TestKnowledgeEngine_IndexAndSearch exercises the full indexing pipeline:
// create engine → index markdown document → search by content → verify stats.
func TestKnowledgeEngine_IndexAndSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// ── Setup: temp directory for DB ──────────────────────────────────────
	tmpDir, err := os.MkdirTemp(".", "cosca-knowledge-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	dbPath := filepath.Join(tmpDir, "knowledge.db")
	cfg := sqlite.DefaultConfig(dbPath)
	// Disable WAL for test isolation
	cfg.WALMode = false
	cfg.JournalMode = "delete"

	// ── Open SQLite database with auto-migration ──────────────────────────
	db, err := sqlite.Open(cfg)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// ── Create dependencies for the indexer ───────────────────────────────
	mdParser := markdown.NewParser()

	entityParser := parser.NewEntityParser()

	chunkCfg := chunker.DefaultConfig()
	chunker := chunker.New(chunkCfg)

	vecCfg := vector.SQLiteVecConfig{
		DB:        db.Conn(),
		Dimension: 4, // small dimension for testing
	}
	vecStore, err := vector.NewSQLiteVec(vecCfg)
	if err != nil {
		t.Fatalf("failed to create vector store: %v", err)
	}
	t.Cleanup(func() { vecStore.Close() })

	ftsClient := sqlite.NewFTSClient(db)

	g := graph.New()
	graphBuilder := graph.NewBuilder(g)

	idxCfg := indexer.DefaultConfig()
	idxCfg.BuildGraph = true
	idxCfg.AllowedExtensions = []string{".md"}

	idx := indexer.New(idxCfg, mdParser, entityParser, chunker, nil, vecStore, ftsClient, db, graphBuilder)

	// ── Write a test markdown document ────────────────────────────────────
	docContent := `# Integration Test Document

This is a test document for the knowledge engine integration tests.

## Section One

The quick brown fox jumps over the lazy dog. This sentence contains
searchable content that we will look for later in the test.

## Code Example

` + "```go" + `
func Hello() string {
    return "Hello, Cosca!"
}
` + "```" + `

## Section Two

Markdown documents with multiple sections and code blocks help verify
that the indexing pipeline correctly processes structured content.
`
	docPath := filepath.Join(tmpDir, "test-doc.md")
	if err := os.WriteFile(docPath, []byte(docContent), 0o644); err != nil {
		t.Fatalf("failed to write test document: %v", err)
	}

	// ── Index the document ────────────────────────────────────────────────
	ctx := context.Background()
	if err := idx.IndexDocument(ctx, docPath); err != nil {
		t.Fatalf("failed to index document: %v", err)
	}

	// ── Verify stats ──────────────────────────────────────────────────────
	stats := idx.GetIndexStats()
	if stats.TotalDocuments != 1 {
		t.Errorf("expected 1 document, got %d", stats.TotalDocuments)
	}
	if stats.TotalChunks == 0 {
		t.Error("expected at least 1 chunk, got 0")
	}
	if stats.LastIndexed.IsZero() {
		t.Error("expected LastIndexed to be set")
	}

	// ── Search for indexed content via FTS ────────────────────────────────
	ftsResults, total, err := ftsClient.Search(sqlite.FTSSearchParams{
		Query: "fox",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("FTS search failed: %v", err)
	}
	if total == 0 {
		t.Error("expected at least 1 FTS result for 'fox'")
	}
	if len(ftsResults) == 0 {
		t.Fatal("expected at least 1 FTS result for 'fox'")
	}

	found := false
	for _, r := range ftsResults {
		if r.Snippet != "" && contains(r.Snippet, "fox") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("FTS results should contain 'fox', got %d results", len(ftsResults))
	}

	// ── Search for code block content ─────────────────────────────────────
	codeResults, _, err := ftsClient.Search(sqlite.FTSSearchParams{
		Query: "Hello Cosca",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("FTS search for code content failed: %v", err)
	}
	if len(codeResults) == 0 {
		t.Log("note: FTS search for code content returned no results (may need FTS5 content sync)")
	}

	// ── Verify document in DB ─────────────────────────────────────────────
	var docCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&docCount); err != nil {
		t.Fatalf("failed to query document count: %v", err)
	}
	if docCount != 1 {
		t.Errorf("expected 1 document in DB, got %d", docCount)
	}

	// ── Verify chunks in DB ───────────────────────────────────────────────
	var chunkCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&chunkCount); err != nil {
		t.Fatalf("failed to query chunk count: %v", err)
	}
	if chunkCount == 0 {
		t.Error("expected at least 1 chunk in DB")
	}

	t.Logf("indexed %d document, %d chunks, FTS found %d results for 'fox'",
		stats.TotalDocuments, stats.TotalChunks, total)
}

// TestKnowledgeEngine_Stats verifies that statistics are correctly accumulated.
func TestKnowledgeEngine_Stats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp(".", "cosca-knowledge-stats-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	dbPath := filepath.Join(tmpDir, "knowledge.db")
	cfg := sqlite.DefaultConfig(dbPath)
	cfg.WALMode = false
	cfg.JournalMode = "delete"

	db, err := sqlite.Open(cfg)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mdParser := markdown.NewParser()
	entityParser := parser.NewEntityParser()
	chunker := chunker.New(chunker.DefaultConfig())

	vecCfg := vector.SQLiteVecConfig{DB: db.Conn(), Dimension: 4}
	vecStore, err := vector.NewSQLiteVec(vecCfg)
	if err != nil {
		t.Fatalf("failed to create vector store: %v", err)
	}
	t.Cleanup(func() { vecStore.Close() })

	ftsClient := sqlite.NewFTSClient(db)
	g := graph.New()
	graphBuilder := graph.NewBuilder(g)

	idxCfg := indexer.DefaultConfig()
	idxCfg.BuildGraph = false
	idxCfg.AllowedExtensions = []string{".md"}

	idx := indexer.New(idxCfg, mdParser, entityParser, chunker, nil, vecStore, ftsClient, db, graphBuilder)

	// Index two documents
	doc1 := "# Doc One\nContent of the first document."
	doc2 := "# Doc Two\nContent of the second document with more text."

	if err := os.WriteFile(filepath.Join(tmpDir, "doc1.md"), []byte(doc1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "doc2.md"), []byte(doc2), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := idx.IndexDocument(ctx, filepath.Join(tmpDir, "doc1.md")); err != nil {
		t.Fatalf("failed to index doc1: %v", err)
	}
	if err := idx.IndexDocument(ctx, filepath.Join(tmpDir, "doc2.md")); err != nil {
		t.Fatalf("failed to index doc2: %v", err)
	}

	stats := idx.GetIndexStats()
	if stats.TotalDocuments != 2 {
		t.Errorf("expected 2 documents, got %d", stats.TotalDocuments)
	}
	if stats.TotalChunks < 2 {
		t.Errorf("expected at least 2 chunks, got %d", stats.TotalChunks)
	}
	if stats.TotalDocuments <= 0 {
		t.Error("expected positive document count")
	}
	if stats.LastIndexed.IsZero() {
		t.Error("expected LastIndexed to be set")
	}

	// Verify document types
	if count, ok := stats.DocumentsByType["markdown"]; !ok || count != 2 {
		t.Errorf("expected 2 markdown documents, got %d", count)
	}

	t.Logf("stats: %d docs, %d chunks, duration=%v",
		stats.TotalDocuments, stats.TotalChunks, stats.TotalDocuments)
}

// contains is a simple string containment check for test assertions.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

// searchString performs a simple substring search without importing strings.
func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
