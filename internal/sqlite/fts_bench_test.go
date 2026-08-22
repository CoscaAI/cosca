package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// newBenchDB creates an in-memory SQLite database with FTS5 tables for benchmarking.
func newBenchDB(b *testing.B) (*DB, *sql.DB) {
	b.Helper()

	// Use a temp file for WAL mode compatibility
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	cfg := Config{
		Path:            dbPath,
		WALMode:         true,
		BusyTimeout:     5000,
		CacheSize:       -20000,
		ForeignKeys:     true,
		AutoMigrate:     true,
		MaxOpenConns:    1,
		MaxIdleConns:    1,
		ConnMaxLifetime: 0,
	}

	// Create directory
	_ = os.MkdirAll(tmpDir, 0755)

	// Open raw connection for direct access
	rawConn, err := sql.Open("sqlite", dbPath+"?mode=rwc&_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		b.Fatalf("failed to open sqlite: %v", err)
	}
	rawConn.SetMaxOpenConns(1)

	// Create the FTS5 tables
	schema := NewSchema()
	for _, stmt := range schema.Statements() {
		if _, err := rawConn.Exec(stmt); err != nil {
			b.Fatalf("failed to execute DDL: %v\nSQL: %s", err, stmt)
		}
	}

	db := &DB{
		conn: rawConn,
		cfg:  cfg,
	}

	b.Cleanup(func() {
		_ = rawConn.Close()
	})

	return db, rawConn
}

// populateFTSBenchDB inserts test documents into the FTS indexes.
func populateFTSBenchDB(b *testing.B, rawConn *sql.DB, docCount, chunkCount, codeCount, entityCount int) {
	b.Helper()

	// Insert documents
	for i := 0; i < docCount; i++ {
		title := fmt.Sprintf("Document %d — Performance Benchmarking with Go and SQLite", i)
		content := fmt.Sprintf("This is document number %d about performance analysis and optimization techniques in Go. "+
			"The Cosca platform uses SQLite FTS5 for full-text search with BM25 ranking. "+
			"Documents are parsed, chunked, and indexed using the knowledge engine pipeline.", i)
		docType := "markdown"

		_, err := rawConn.Exec(`INSERT INTO documents (id, path, hash, title, doc_type, size, token_count)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("doc-%d", i),
			fmt.Sprintf("/docs/doc-%d.md", i),
			fmt.Sprintf("hash-%d", i),
			title, docType, len(content), 50,
		)
		if err != nil {
			b.Fatalf("failed to insert document: %v", err)
		}
	}

	// Insert chunks
	for i := 0; i < chunkCount; i++ {
		docID := fmt.Sprintf("doc-%d", i%docCount)
		content := fmt.Sprintf("Chunk %d of document %d. This section covers performance profiling, "+
			"memory management, and search optimization using FTS5 and vector similarity. "+
			"The knowledge engine combines multiple search strategies for optimal results.", i, i%docCount)

		_, err := rawConn.Exec(`INSERT INTO chunks (id, document_id, content, heading, section_type, position, hash, token_count)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("chunk-%d", i), docID, content,
			fmt.Sprintf("Section %d", i%10), "text", i, fmt.Sprintf("hash-c-%d", i), 30,
		)
		if err != nil {
			b.Fatalf("failed to insert chunk: %v", err)
		}
	}

	// Insert code blocks
	for i := 0; i < codeCount; i++ {
		docID := fmt.Sprintf("doc-%d", i%docCount)
		content := fmt.Sprintf("func BenchmarkSearch%d(b *testing.B) {\n\tfor i := 0; i < b.N; i++ {\n"+
			"\t\tengine.Search(context.Background(), params)\n\t}\n}", i)

		_, err := rawConn.Exec(`INSERT INTO code_blocks (id, document_id, language, content, position, token_count)
			VALUES (?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("code-%d", i), docID, "go", content, i, 15,
		)
		if err != nil {
			b.Fatalf("failed to insert code block: %v", err)
		}
	}

	// Insert entities
	entityTypes := []string{"agent", "skill", "prompt", "workflow", "template"}
	for i := 0; i < entityCount; i++ {
		name := fmt.Sprintf("Entity %d — Performance Optimization", i)
		eType := entityTypes[i%len(entityTypes)]
		metadataJSON := fmt.Sprintf(`{"version":"%d","status":"active"}`, i)

		_, err := rawConn.Exec(`INSERT INTO entities (id, entity_type, name, path, metadata_json)
			VALUES (?, ?, ?, ?, ?)`,
			fmt.Sprintf("entity-%d", i), eType, name,
			fmt.Sprintf("/entities/entity-%d.md", i), metadataJSON,
		)
		if err != nil {
			b.Fatalf("failed to insert entity: %v", err)
		}
	}
}

// ── FTS5 Search Benchmarks ──────────────────────────────────────────────────

func BenchmarkFTS5SearchDocuments(b *testing.B) {
	db, rawConn := newBenchDB(b)
	populateFTSBenchDB(b, rawConn, 100, 500, 50, 30)

	ftsClient := NewFTSClient(db)
	params := FTSSearchParams{
		Query:      "\"performance\" \"benchmarking\"",
		TableNames: []string{"documents_fts"},
		Limit:      20,
		MaxSnippet: 200,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, count, err := ftsClient.Search(params)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
		_ = results
		_ = count
	}
}

func BenchmarkFTS5SearchChunks(b *testing.B) {
	db, rawConn := newBenchDB(b)
	populateFTSBenchDB(b, rawConn, 100, 500, 50, 30)

	ftsClient := NewFTSClient(db)
	params := FTSSearchParams{
		Query:      "\"performance\" \"search\"",
		TableNames: []string{"chunks_fts"},
		Limit:      20,
		MaxSnippet: 200,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, count, err := ftsClient.Search(params)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
		_ = results
		_ = count
	}
}

func BenchmarkFTS5SearchAllTables(b *testing.B) {
	db, rawConn := newBenchDB(b)
	populateFTSBenchDB(b, rawConn, 100, 500, 50, 30)

	ftsClient := NewFTSClient(db)
	params := FTSSearchParams{
		Query: "\"performance\" \"optimization\"",
		Limit: 20,
		// nil TableNames = search all 4 tables (documents, chunks, code_blocks, entities)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, count, err := ftsClient.Search(params)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
		_ = results
		_ = count
	}
}

func BenchmarkFTS5SearchSingleTerm(b *testing.B) {
	db, rawConn := newBenchDB(b)
	populateFTSBenchDB(b, rawConn, 100, 500, 50, 30)

	ftsClient := NewFTSClient(db)
	params := FTSSearchParams{
		Query:      "\"golang\"",
		TableNames: []string{"documents_fts"},
		Limit:      20,
		MaxSnippet: 200,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, count, err := ftsClient.Search(params)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
		_ = results
		_ = count
	}
}

func BenchmarkFTS5SearchLargeScale(b *testing.B) {
	db, rawConn := newBenchDB(b)
	// Larger dataset: 1000 docs, 5000 chunks
	populateFTSBenchDB(b, rawConn, 1000, 5000, 200, 150)

	ftsClient := NewFTSClient(db)
	params := FTSSearchParams{
		Query:      "\"performance\" \"analysis\"",
		TableNames: []string{"chunks_fts"},
		Limit:      50,
		MaxSnippet: 200,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, count, err := ftsClient.Search(params)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
		_ = results
		_ = count
	}
}

func BenchmarkFTS5WithEdgeCasePhrase(b *testing.B) {
	db, rawConn := newBenchDB(b)
	populateFTSBenchDB(b, rawConn, 100, 500, 50, 30)

	ftsClient := NewFTSClient(db)
	params := FTSSearchParams{
		Query:      "\"cosca\" \"platform\"", // Should match fewer results
		TableNames: []string{"documents_fts"},
		Limit:      20,
		MaxSnippet: 200,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, count, err := ftsClient.Search(params)
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
		_ = results
		_ = count
	}
}

// ── FTS5 Index Operations Benchmarks ────────────────────────────────────────

func BenchmarkFTS5IndexDocument(b *testing.B) {
	db, rawConn := newBenchDB(b)

	ftsClient := NewFTSClient(db)
	id := "test-doc"
	title := "Benchmark Index Document"
	content := "This is content for FTS5 indexing benchmark with performance keywords."
	docType := "markdown"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Cleanup previous
		_, _ = rawConn.Exec("INSERT INTO documents_fts(documents_fts, rowid, title, content, doc_type) VALUES ('delete', ?, '', '', '')", id)
		err := ftsClient.IndexDocument(id, title, content, docType)
		if err != nil {
			b.Fatalf("index failed: %v", err)
		}
	}
}

func BenchmarkFTS5IndexChunk(b *testing.B) {
	db, rawConn := newBenchDB(b)

	ftsClient := NewFTSClient(db)
	id := "test-chunk"
	content := "This is chunk content for FTS5 indexing with search optimization."
	heading := "Performance Section"
	sectionType := "text"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rawConn.Exec("INSERT INTO chunks_fts(chunks_fts, rowid, content, heading, section_type) VALUES ('delete', ?, '', '', '')", id)
		err := ftsClient.IndexChunk(id, content, heading, sectionType)
		if err != nil {
			b.Fatalf("index failed: %v", err)
		}
	}
}

// ── FTS5 Count Query Benchmarks ─────────────────────────────────────────────

func BenchmarkFTS5CountQuery(b *testing.B) {
	_, rawConn := newBenchDB(b)
	populateFTSBenchDB(b, rawConn, 100, 500, 50, 30)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var count int
		err := rawConn.QueryRow("SELECT COUNT(*) FROM documents_fts WHERE documents_fts MATCH ?", "\"performance\"").Scan(&count)
		if err != nil {
			b.Fatalf("count query failed: %v", err)
		}
	}
}

// ── SanitizeFTSQuery Benchmarks ─────────────────────────────────────────────

func BenchmarkSanitizeFTSQuery_Simple(b *testing.B) {
	query := "knowledge engine performance"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeFTSQuery(query)
	}
}

func BenchmarkSanitizeFTSQuery_Complex(b *testing.B) {
	query := "search \"FTS5\" optimization (BM25 OR cosine)"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeFTSQuery(query)
	}
}

// ── Rebuild Index ───────────────────────────────────────────────────────────

func BenchmarkFTS5RebuildIndex(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping rebuild benchmark in short mode")
	}

	dbi, rawConn := newBenchDB(b)
	populateFTSBenchDB(b, rawConn, 100, 500, 50, 30)

	ftsClient := NewFTSClient(dbi)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := ftsClient.RebuildIndex()
		if err != nil {
			b.Fatalf("rebuild failed: %v", err)
		}
	}
}

// ── SQL Query Benchmarks for Dashboard ───────────────────────────────────────

func BenchmarkDashboardDocCountQuery(b *testing.B) {
	_, rawConn := newBenchDB(b)
	populateFTSBenchDB(nil, rawConn, 1000, 5000, 200, 150)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var count int
		_ = rawConn.QueryRow("SELECT COUNT(*) FROM documents").Scan(&count)
	}
}

func BenchmarkDashboardChunkCountQuery(b *testing.B) {
	_, rawConn := newBenchDB(b)
	populateFTSBenchDB(nil, rawConn, 1000, 5000, 200, 150)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var count int
		_ = rawConn.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&count)
	}
}

func BenchmarkDashboardAllStatsQueries(b *testing.B) {
	_, rawConn := newBenchDB(b)
	populateFTSBenchDB(nil, rawConn, 1000, 5000, 200, 150)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate GetStats() - 6 separate DB queries
		var docCount, chunkCount, entityCount, vecCount int
		_ = rawConn.QueryRow("SELECT COUNT(*) FROM documents").Scan(&docCount)
		_ = rawConn.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&chunkCount)
		_ = rawConn.QueryRow("SELECT COUNT(*) FROM entities").Scan(&entityCount)
		_ = rawConn.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&vecCount)
		_ = docCount
		_ = chunkCount
		_ = entityCount
		_ = vecCount
	}
}

// ── EXPLAIN QUERY PLAN Benchmarks ────────────────────────────────────────────

func BenchmarkExplainQueryPlan_FTS5(b *testing.B) {
	_, rawConn := newBenchDB(b)
	populateFTSBenchDB(nil, rawConn, 100, 500, 50, 30)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := rawConn.Query("EXPLAIN QUERY PLAN SELECT rank, rowid, title, content, doc_type FROM documents_fts WHERE documents_fts MATCH ?", "\"performance\"")
		if err != nil {
			b.Fatalf("explain failed: %v", err)
		}
		_ = rows.Close()
	}
}

// ── Helper to check EXPLAIN query plan output ────────────────────────────────

// ExplainFTS5Query returns the EXPLAIN QUERY PLAN output for an FTS5 query.
// This is NOT a benchmark — it's a diagnostic function used by tests.
func ExplainFTS5Query(b *testing.B, rawConn *sql.DB) []string {
	var plans []string
	queries := []struct {
		name  string
		table string
		query string
	}{
		{"documents_fts", "documents_fts", "\"performance\" \"benchmarking\""},
		{"chunks_fts", "chunks_fts", "\"performance\" \"search\""},
		{"code_blocks_fts", "code_blocks_fts", "\"performance\""},
		{"entities_fts", "entities_fts", "\"performance\""},
	}

	for _, q := range queries {
		sql := fmt.Sprintf("EXPLAIN QUERY PLAN SELECT * FROM %s WHERE %s MATCH ?", q.table, q.table)
		rows, err := rawConn.Query(sql, q.query)
		if err != nil {
			plans = append(plans, fmt.Sprintf("%s: ERROR — %v", q.name, err))
			continue
		}
		var buf strings.Builder
		fmt.Fprintf(&buf, "%s:\n", q.name)
		for rows.Next() {
			var detail string
			_ = rows.Scan(&detail)
			fmt.Fprintf(&buf, "  %s\n", detail)
		}
		_ = rows.Close()
		plans = append(plans, buf.String())
	}

	return plans
}
