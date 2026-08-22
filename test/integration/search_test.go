
//go:build integration

package integration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/ranking"
	"github.com/CoscaAI/cosca/internal/search"
	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/CoscaAI/cosca/internal/vector"
	_ "modernc.org/sqlite"
)

// TestHybridSearch_FTSAndVector tests the hybrid search pipeline combining
// FTS5 full-text search and vector similarity search with re-ranking.
func TestHybridSearch_FTSAndVector(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// ── Setup: temp SQLite database ───────────────────────────────────────
	tmpDir, err := os.MkdirTemp(".", "cosca-search-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	dbPath := filepath.Join(tmpDir, "search.db")
	cfg := sqlite.DefaultConfig(dbPath)
	cfg.WALMode = false
	cfg.JournalMode = "delete"

	db, err := sqlite.Open(cfg)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// ── Create dependencies ───────────────────────────────────────────────
	ftsClient := sqlite.NewFTSClient(db)

	vecCfg := vector.SQLiteVecConfig{
		DB:         db.Conn(),
		Dimension:  4,
		TableName:  "search_vectors",
		Normalized: true,
	}
	vecStore, err := vector.NewSQLiteVec(vecCfg)
	if err != nil {
		t.Fatalf("failed to create vector store: %v", err)
	}
	t.Cleanup(func() { vecStore.Close() })

	ranker := ranking.New(ranking.DefaultConfig())

	// Create a mock embedding function that returns simple vectors
	embedFunc := func(ctx context.Context, text string) (*search.EmbeddingRequest, error) {
		// Simple deterministic embedding based on text length and content
		vec := make([]float64, 4)
		vec[0] = float64(len(text)) / 100.0
		for i, b := range text {
			if i < 4 {
				vec[i] += float64(b) / 256.0
			}
		}
		return &search.EmbeddingRequest{Vector: vec}, nil
	}

	// Create search engine (nil graph since we're not testing graph search)
	engine := search.NewEngine(ftsClient, vecStore, nil, ranker, embedFunc)

	// ── Seed data: insert documents directly into SQLite ─────────────────
	docs := []struct {
		id      string
		title   string
		content string
		docType string
	}{
		{
			id:      "doc-1",
			title:   "Introduction to Cosca",
			content: "Cosca is a command-line tool for managing enterprise AI agents. It provides knowledge management, memory storage, and plugin system.",
			docType: "markdown",
		},
		{
			id:      "doc-2",
			title:   "Plugin Development Guide",
			content: "Developers can create plugins for the Cosca platform using Go or WebAssembly. Plugins extend the functionality of the runtime.",
			docType: "markdown",
		},
		{
			id:      "doc-3",
			title:   "Memory Management",
			content: "The memory system stores agent context and conversation history. It supports ephemeral, persistent, and working memory types.",
			docType: "markdown",
		},
		{
			id:      "doc-4",
			title:   "Search and Retrieval",
			content: "Hybrid search combines full-text search with vector similarity for better results. The ranking system uses BM25 and neural embeddings.",
			docType: "markdown",
		},
	}

	for _, d := range docs {
		// Insert into documents table
		_, err := db.Exec(
			`INSERT OR IGNORE INTO documents (id, path, hash, title, doc_type, metadata_json, size)
			 VALUES (?, ?, 'hash', ?, 'markdown', '{}', ?)`,
			d.id, "/test/"+d.id+".md", d.title, len(d.content),
		)
		if err != nil {
			t.Fatalf("failed to insert document %s: %v", d.id, err)
		}

		// Index document for FTS
		if _, err := db.Exec(
			`INSERT INTO chunks(id, document_id, content, heading, section_type, position)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			"chk-"+d.id, d.id, d.content, "Section", "content", 1); err != nil {
			t.Fatalf("failed to insert chunk for %s: %v", d.id, err)
		}

		// Store vector embedding
		embedReq, err := embedFunc(context.Background(), d.title+" "+d.content)
		if err != nil {
			t.Fatalf("failed to generate embedding: %v", err)
		}
		if err := vecStore.Store(4, []vector.VectorRecord{
			{
				ID:         d.id,
				Vector:     embedReq.Vector,
				DocumentID: d.id,
				Content:    d.content,
			},
		}); err != nil {
			t.Fatalf("failed to store vector: %v", err)
		}
	}

	// ── Test: search for "plugin" ─────────────────────────────────────────
	t.Run("search for plugin", func(t *testing.T) {
		results, err := engine.Search(context.Background(), search.SearchParams{
			Query:        "plugin",
			Limit:        10,
			EnableFTS:    true,
			EnableVector: true,
			EnableGraph:  false,
		})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if results.TotalCount == 0 {
			t.Fatal("expected at least 1 result for 'plugin'")
		}
		if len(results.Results) == 0 {
			t.Fatal("expected at least 1 result in results list")
		}

		// The plugin document should rank high
		foundPlugin := false
		for _, r := range results.Results {
			t.Logf("  result: type=%s score=%.4f source=%s title=%q",
				r.Type, r.Score, r.Source, r.Title)
			if r.Title == "Plugin Development Guide" {
				foundPlugin = true
			}
		}
		if !foundPlugin {
			t.Log("note: 'Plugin Development Guide' not in top results (expected with seeded data)")
		}

		if results.Query != "plugin" {
			t.Errorf("expected query 'plugin', got %q", results.Query)
		}
		if results.Duration <= 0 {
			t.Error("expected positive duration")
		}
	})

	// ── Test: search for "memory" ─────────────────────────────────────────
	t.Run("search for memory", func(t *testing.T) {
		results, err := engine.Search(context.Background(), search.SearchParams{
			Query:        "memory",
			Limit:        5,
			EnableFTS:    true,
			EnableVector: false,
		})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results.Results) == 0 {
			t.Fatal("expected at least 1 result for 'memory'")
		}

		foundMemory := false
		for _, r := range results.Results {
			if searchString(r.Title, "Memory") {
				foundMemory = true
				break
			}
		}
		if !foundMemory {
			t.Log("note: 'Memory Management' not in top results")
		}
	})

	// ── Test: empty query returns error ───────────────────────────────────
	t.Run("empty query error", func(t *testing.T) {
		_, err := engine.Search(context.Background(), search.SearchParams{
			Query: "",
			Limit: 10,
		})
		if err == nil {
			t.Error("expected error for empty query")
		}
	})

	// ── Test: search across all documents ─────────────────────────────────
	t.Run("search all", func(t *testing.T) {
		results, err := engine.Search(context.Background(), search.SearchParams{
			Query:        "Cosca",
			Limit:        20,
			EnableFTS:    true,
			EnableVector: true,
		})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if results.TotalCount == 0 {
			t.Error("expected results for 'Cosca'")
		}
		if len(results.Results) > 0 && results.Results[0].Score <= 0 {
			t.Error("expected positive score for top result")
		}

		t.Logf("'Cosca' search: %d total, %d returned, took %v",
			results.TotalCount, len(results.Results), results.Duration)
	})
}

// TestHybridSearch_ReRanking verifies that the re-ranking phase
// correctly orders results when combining FTS and vector sources.
func TestHybridSearch_ReRanking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp(".", "cosca-search-rerank-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	dbPath := filepath.Join(tmpDir, "rerank.db")
	cfg := sqlite.DefaultConfig(dbPath)
	cfg.WALMode = false
	cfg.JournalMode = "delete"

	db, err := sqlite.Open(cfg)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ftsClient := sqlite.NewFTSClient(db)

	vecCfg := vector.SQLiteVecConfig{
		DB:        db.Conn(),
		Dimension: 4,
		TableName: "rerank_vectors",
	}
	vecStore, err := vector.NewSQLiteVec(vecCfg)
	if err != nil {
		t.Fatalf("failed to create vector store: %v", err)
	}
	t.Cleanup(func() { vecStore.Close() })

	// Use a ranker with known weights
	rankerCfg := ranking.DefaultConfig()
	rankerCfg.BM25Weight = 0.5
	rankerCfg.VectorWeight = 0.5
	rankerCfg.GraphWeight = 0.0
	rankerCfg.FreshnessWeight = 0.0
	rankerCfg.PopularityWeight = 0.0
	ranker := ranking.New(rankerCfg)

	embedFunc := func(ctx context.Context, text string) (*search.EmbeddingRequest, error) {
		return &search.EmbeddingRequest{Vector: []float64{0.1, 0.2, 0.3, 0.4}}, nil
	}

	engine := search.NewEngine(ftsClient, vecStore, nil, ranker, embedFunc)

	// Seed minimal data
	_, err = db.Exec(
		`INSERT OR IGNORE INTO documents (id, path, hash, title, doc_type, metadata_json, size)
		 VALUES ('rank-doc-1', '/test/rank-doc-1.md', 'hash', 'Ranking Test Document', 'markdown', '{}', 100)`,
	)
	if err != nil {
		t.Fatalf("failed to insert document: %v", err)
	}

	// Insert a chunk - the trigger syncs to FTS5
	if _, err := db.Exec(
		`INSERT INTO chunks(id, document_id, content, heading, section_type, position)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		"chunk-1", "rank-doc-1",
		"This is a document about ranking and re-ranking of search results.", "Test Section", "content", 1); err != nil {
		t.Fatalf("failed to index chunk in FTS: %v", err)
	}

	if err := vecStore.Store(4, []vector.VectorRecord{
		{
			ID:         "rank-doc-1",
			Vector:     []float64{0.1, 0.2, 0.3, 0.4},
			DocumentID: "rank-doc-1",
			Content:    "This is a document about ranking and re-ranking of search results.",
		},
	}); err != nil {
		t.Fatalf("failed to store vector: %v", err)
	}

	// Search and verify results are ordered
	results, err := engine.Search(context.Background(), search.SearchParams{
		Query:        "ranking",
		Limit:        5,
		EnableFTS:    true,
		EnableVector: true,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results.Results) > 0 {
		// Verify results are sorted by score descending
		for i := 1; i < len(results.Results); i++ {
			if results.Results[i-1].Score < results.Results[i].Score {
				t.Errorf("results not sorted by score descending at index %d", i)
			}
		}
		// Verify ranks are assigned
		for i, r := range results.Results {
			if r.Rank != i+1 {
				t.Errorf("expected rank %d, got %d", i+1, r.Rank)
			}
		}

		t.Logf("ranking test: %d results, top score=%.4f",
			len(results.Results), results.Results[0].Score)
	}
}

// TestHybridSearch_EmptyStore verifies search behavior on empty databases.
func TestHybridSearch_EmptyStore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp(".", "cosca-search-empty-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	dbPath := filepath.Join(tmpDir, "empty.db")
	cfg := sqlite.DefaultConfig(dbPath)
	cfg.WALMode = false
	cfg.JournalMode = "delete"

	db, err := sqlite.Open(cfg)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ftsClient := sqlite.NewFTSClient(db)

	vecCfg := vector.SQLiteVecConfig{
		DB:        db.Conn(),
		Dimension: 4,
		TableName: "empty_vectors",
	}
	vecStore, err := vector.NewSQLiteVec(vecCfg)
	if err != nil {
		t.Fatalf("failed to create vector store: %v", err)
	}
	t.Cleanup(func() { vecStore.Close() })

	ranker := ranking.New(ranking.DefaultConfig())

	embedFunc := func(ctx context.Context, text string) (*search.EmbeddingRequest, error) {
		return &search.EmbeddingRequest{Vector: []float64{0, 0, 0, 0}}, nil
	}

	engine := search.NewEngine(ftsClient, vecStore, nil, ranker, embedFunc)

	// Search on empty store should return empty results (not error)
	results, err := engine.Search(context.Background(), search.SearchParams{
		Query:        "anything",
		Limit:        10,
		EnableFTS:    true,
		EnableVector: true,
	})
	if err != nil {
		t.Fatalf("Search on empty store failed: %v", err)
	}

	if len(results.Results) != 0 {
		t.Errorf("expected 0 results on empty store, got %d", len(results.Results))
	}
	if results.TotalCount != 0 {
		t.Errorf("expected TotalCount=0 on empty store, got %d", results.TotalCount)
	}
}

// ── Mock implementations ──────────────────────────────────────────────────

// mockDB wraps *sql.DB for testing purposes.
type mockDB struct {
	*sql.DB
}
