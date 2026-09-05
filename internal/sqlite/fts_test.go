package sqlite

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

// ── FTS: NewFTSClient ────────────────────────────────────────────────────

func TestNewFTSClient(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	client := NewFTSClient(db)
	assert.NotNil(t, client)
	assert.Equal(t, db, client.db)
}

// ── FTS: SanitizeFTSQuery tests ──────────────────────────────────────────

func TestSanitizeFTSQuery_Simple(t *testing.T) {
	t.Parallel()

	result := SanitizeFTSQuery("hello world")
	assert.Equal(t, `"hello" "world"`, result)
}

func TestSanitizeFTSQuery_Empty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", SanitizeFTSQuery(""))
	assert.Equal(t, "", SanitizeFTSQuery("   "))
}

func TestSanitizeFTSQuery_AlreadyQuotedTerms(t *testing.T) {
	t.Parallel()

	// Quotes are treated as literal characters (stripped by the allowlist),
	// so an input that already "looks" quoted yields the same result as the
	// plain query — no passthrough of raw FTS5 grammar.
	assert.Equal(t, `"hello" "world"`, SanitizeFTSQuery(`"hello" "world"`))
}

func TestSanitizeFTSQuery_WildcardStripped(t *testing.T) {
	t.Parallel()

	// FTS5 wildcard operator is stripped: the term is matched literally.
	assert.Equal(t, `"hello"`, SanitizeFTSQuery("hello*"))
}

func TestSanitizeFTSQuery_ParensStripped(t *testing.T) {
	t.Parallel()

	// FTS5 boolean operators are stripped: no raw grammar reaches MATCH.
	assert.Equal(t, `"hello" "OR" "world"`, SanitizeFTSQuery("(hello OR world)"))
}

func TestSanitizeFTSQuery_SingleTerm(t *testing.T) {
	t.Parallel()

	assert.Equal(t, `"hello"`, SanitizeFTSQuery("hello"))
}

func TestSanitizeFTSQuery_CaretStripped(t *testing.T) {
	t.Parallel()

	// FTS5 prefix-boost operator is stripped.
	assert.Equal(t, `"hello"`, SanitizeFTSQuery("^hello"))
}

func TestSanitizeFTSQuery_PlainTermsQuoted(t *testing.T) {
	t.Parallel()

	result := SanitizeFTSQuery("hello AND world")
	assert.Equal(t, `"hello" "AND" "world"`, result)
}

func TestSanitizeFTSQuery_InternalQuotes(t *testing.T) {
	t.Parallel()

	// An embedded double quote is a literal character here, not an FTS5
	// quote — the allowlist strips it and the term is quoted literally.
	result := SanitizeFTSQuery(`he"llo`)
	assert.Equal(t, `"hello"`, result)
}

// TestSanitizeFTSQuery_QuotesOperators verifies that FTS5 operator
// characters in the user query are never passed through to the MATCH
// expression — they are stripped and each term is quoted as a literal
// (M9b: FTS5 query injection / DoS).
func TestSanitizeFTSQuery_QuotesOperators(t *testing.T) {
	t.Parallel()

	query := "a* b^ c"
	result := SanitizeFTSQuery(query)

	assert.Equal(t, `"a" "b" "c"`, result)
	// No raw FTS5 operator characters may survive into the MATCH expression.
	assert.NotContains(t, result, "*")
	assert.NotContains(t, result, "^")
	assert.NotContains(t, result, "(")
	assert.NotContains(t, result, ")")
	assert.NotContains(t, result, "?")
	assert.NotContains(t, result, "&")
}

// TestSanitizeFTSQuery_OperatorsOnly verifies that a query consisting only
// of operator/punctuation characters sanitizes to an empty string instead of
// leaking raw FTS5 grammar into MATCH.
func TestSanitizeFTSQuery_OperatorsOnly(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", SanitizeFTSQuery("*** ^^^ ()"))
	assert.Equal(t, "", SanitizeFTSQuery(`""""`))
}

// TestSanitizeFTSQuery_MixedSpecialChars verifies mixed punctuation and
// unicode content is handled without panics and keeps only allowlisted chars.
func TestSanitizeFTSQuery_MixedSpecialChars(t *testing.T) {
	t.Parallel()

	assert.Equal(t, `"foobar" "baz" "42" "_x_"`, SanitizeFTSQuery(`foo.bar! "baz"* 42 _x_`))
}

// ── FTS: truncateText tests ──────────────────────────────────────────────

func TestTruncateText(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello", truncateText("hello", 10))
	assert.Equal(t, "lo...", truncateText("longer text here", 5))
	assert.Equal(t, "abc", truncateText("abc", 3))
}

// ── FTS: FTSResult.String tests ──────────────────────────────────────────

func TestFTSResultString(t *testing.T) {
	t.Parallel()

	r := FTSResult{
		Rank: 0.5, TableName: "docs", Title: "Test",
		Snippet: "hello world snippet text here",
	}
	result := r.String()
	assert.Contains(t, result, "[docs]")
	assert.Contains(t, result, "0.5000")
	assert.Contains(t, result, "Test")
	assert.Contains(t, result, "snippet")
}

func TestFTSResultString_NoSnippet(t *testing.T) {
	t.Parallel()

	r := FTSResult{Rank: 1.0, TableName: "chunks_fts", Heading: "Header"}
	result := r.String()
	assert.Contains(t, result, "[chunks_fts]")
	assert.Contains(t, result, "Header")
}

func TestFTSResultString_Minimal(t *testing.T) {
	t.Parallel()

	r := FTSResult{Rank: 0.0, TableName: "entities_fts"}
	result := r.String()
	assert.Contains(t, result, "[entities_fts]")
	assert.Contains(t, result, "0.0000")
}

// ── FTS: resolveTables tests ─────────────────────────────────────────────

func TestResolveTables_AllDefaults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	client := NewFTSClient(db)
	tables := client.resolveTables(nil)
	assert.Len(t, tables, 5)
	assert.Equal(t, "documents_fts", tables[0])
	assert.Equal(t, "chunks_fts", tables[1])
	assert.Equal(t, "code_blocks_fts", tables[2])
	assert.Equal(t, "entities_fts", tables[3])
	assert.Equal(t, "knowledge_fts", tables[4])
}

func TestResolveTables_EmptySlice(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	client := NewFTSClient(db)
	tables := client.resolveTables([]string{})
	assert.Len(t, tables, 5)
}

func TestResolveTables_Specific(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	client := NewFTSClient(db)
	tables := client.resolveTables([]string{"documents_fts"})
	assert.Len(t, tables, 1)
	assert.Equal(t, "documents_fts", tables[0])
}

// ── FTS: defaultParams tests ─────────────────────────────────────────────

func TestFTDefaultParams_AllDefaults(t *testing.T) {
	t.Parallel()

	client := &FTSClient{}
	params := client.defaultParams(FTSSearchParams{Query: "test"})
	assert.Equal(t, 20, params.Limit)
	assert.Equal(t, 0, params.Offset)
	assert.Equal(t, 200, params.MaxSnippet)
}

func TestFTDefaultParams_CustomValues(t *testing.T) {
	t.Parallel()

	client := &FTSClient{}
	params := client.defaultParams(FTSSearchParams{
		Query:      "hello",
		Limit:      10,
		Offset:     5,
		MaxSnippet: 100,
	})
	assert.Equal(t, 10, params.Limit)
	assert.Equal(t, 5, params.Offset)
	assert.Equal(t, 100, params.MaxSnippet)
}

func TestFTDefaultParams_NegativeOffset(t *testing.T) {
	t.Parallel()

	client := &FTSClient{}
	params := client.defaultParams(FTSSearchParams{
		Query:  "test",
		Offset: -1,
	})
	assert.Equal(t, 0, params.Offset)
}

// ── FTS: countQuery tests ────────────────────────────────────────────────

func TestFTS_CountQuery(t *testing.T) {
	t.Parallel()

	client := &FTSClient{}

	assert.Equal(t,
		"SELECT COUNT(*) FROM documents_fts WHERE documents_fts MATCH ?1",
		client.countQuery("documents_fts"),
	)
	assert.Equal(t,
		"SELECT COUNT(*) FROM chunks_fts WHERE chunks_fts MATCH ?1",
		client.countQuery("chunks_fts"),
	)
	assert.Equal(t,
		"SELECT COUNT(*) FROM code_blocks_fts WHERE code_blocks_fts MATCH ?1",
		client.countQuery("code_blocks_fts"),
	)
	assert.Equal(t,
		"SELECT COUNT(*) FROM entities_fts WHERE entities_fts MATCH ?1",
		client.countQuery("entities_fts"),
	)
	assert.Equal(t, "SELECT 0", client.countQuery("unknown"))
}

// ── FTS: Content-synced search (triggers auto-sync) ──────────────────────

func TestFTS_ContentSynced_SearchDocuments(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(
		"INSERT INTO documents (id, path, hash, title, doc_type) VALUES (?, ?, ?, ?, ?)",
		"sync-doc", "/sync.md", "abc123", "Synced Document Title", "markdown",
	)
	require.NoError(t, err)

	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM documents_fts WHERE documents_fts MATCH 'Synced'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

func TestFTS_ContentSynced_SearchChunks(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(
		"INSERT INTO documents (id, path, hash, title, doc_type) VALUES (?, ?, ?, ?, ?)",
		"doc-ch", "/ch.md", "abc", "Chunk Doc", "markdown",
	)
	require.NoError(t, err)
	_, err = db.Exec(
		"INSERT INTO chunks (id, document_id, content, heading) VALUES (?, ?, ?, ?)",
		"chunk-1", "doc-ch", "This is searchable chunk content here", "Heading",
	)
	require.NoError(t, err)

	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM chunks_fts WHERE chunks_fts MATCH 'searchable'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

func TestFTS_ContentSynced_SearchCodeBlocks(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(
		"INSERT INTO documents (id, path, hash, title, doc_type) VALUES (?, ?, ?, ?, ?)",
		"doc-cb", "/cb.md", "abc", "Code Doc", "markdown",
	)
	require.NoError(t, err)
	_, err = db.Exec(
		"INSERT INTO code_blocks (id, document_id, content, language) VALUES (?, ?, ?, ?)",
		"cb-1", "doc-cb", "func findMe() { return 42 }", "go",
	)
	require.NoError(t, err)

	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM code_blocks_fts WHERE code_blocks_fts MATCH 'findMe'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

func TestFTS_ContentSynced_SearchEntities(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(
		"INSERT INTO entities (id, entity_type, name, path) VALUES (?, ?, ?, ?)",
		"ent-find", "agent", "FindableAgent", "/agents/findable.yaml",
	)
	require.NoError(t, err)

	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM entities_fts WHERE entities_fts MATCH 'FindableAgent'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

// ── FTS: Search via FTSClient (using tables with matching content columns) ──

func TestFTS_SearchViaClient_Chunks(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(
		"INSERT INTO documents (id, path, hash, title, doc_type) VALUES (?, ?, ?, ?, ?)",
		"doc-chs", "/chs.md", "abc", "ChunkSearch Doc", "markdown",
	)
	require.NoError(t, err)
	_, err = db.Exec(
		"INSERT INTO chunks (id, document_id, content, heading) VALUES (?, ?, ?, ?)",
		"ch-srch", "doc-chs", "searchable content in a chunk", "Header",
	)
	require.NoError(t, err)

	client := NewFTSClient(db)
	results, count, err := client.Search(FTSSearchParams{
		Query:      "searchable",
		TableNames: []string{"chunks_fts"},
	})
	require.NoError(t, err)
	assert.Greater(t, count, 0)
	assert.NotEmpty(t, results)
	assert.Equal(t, "chunks_fts", results[0].TableName)
}

func TestFTS_SearchViaClient_CodeBlocks(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(
		"INSERT INTO documents (id, path, hash, title, doc_type) VALUES (?, ?, ?, ?, ?)",
		"doc-cbs", "/cbs.md", "abc", "CodeBlockSearch", "markdown",
	)
	require.NoError(t, err)
	_, err = db.Exec(
		"INSERT INTO code_blocks (id, document_id, content, language) VALUES (?, ?, ?, ?)",
		"cb-srch", "doc-cbs", "func searchMe() {}", "go",
	)
	require.NoError(t, err)

	client := NewFTSClient(db)
	results, count, err := client.Search(FTSSearchParams{
		Query:      "searchMe",
		TableNames: []string{"code_blocks_fts"},
	})
	require.NoError(t, err)
	assert.Greater(t, count, 0)
	assert.NotEmpty(t, results)
	assert.Equal(t, "code_blocks_fts", results[0].TableName)
}

func TestFTS_SearchEmptyQuery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	client := NewFTSClient(db)
	_, _, err = client.Search(FTSSearchParams{Query: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query is required")
}

func TestFTS_Search_WithHighlight(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(
		"INSERT INTO documents (id, path, hash, title, doc_type) VALUES (?, ?, ?, ?, ?)",
		"doc-hl2", "/hl2.md", "abc", "HL Doc", "markdown",
	)
	require.NoError(t, err)
	_, err = db.Exec(
		"INSERT INTO chunks (id, document_id, content, heading) VALUES (?, ?, ?, ?)",
		"ch-hl", "doc-hl2", "highlight this interesting text", "Header",
	)
	require.NoError(t, err)

	client := NewFTSClient(db)
	results, _, err := client.Search(FTSSearchParams{
		Query:      "interesting",
		TableNames: []string{"chunks_fts"},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.NotEmpty(t, results[0].Snippet)
}

func TestFTS_SearchPagination(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(
		"INSERT INTO documents (id, path, hash, title, doc_type) VALUES (?, ?, ?, ?, ?)",
		"doc-pag", "/pag.md", "abc", "Pag Doc", "markdown",
	)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err = db.Exec(
			"INSERT INTO chunks (id, document_id, content, heading) VALUES (?, ?, ?, ?)",
			"ch-pag-"+string(rune('0'+i)), "doc-pag",
			"paginated search result number "+string(rune('0'+i)),
			"Header "+string(rune('0'+i)),
		)
		require.NoError(t, err)
	}

	client := NewFTSClient(db)
	results, count, err := client.Search(FTSSearchParams{
		Query:      "paginated",
		TableNames: []string{"chunks_fts"},
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 2)
	assert.Len(t, results, 2)

	results2, _, err := client.Search(FTSSearchParams{
		Query:      "paginated",
		TableNames: []string{"chunks_fts"},
		Limit:      2,
		Offset:     2,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, results2)
	if len(results) > 0 && len(results2) > 0 {
		assert.NotEqual(t, results[0].RowID, results2[0].RowID)
	}
}

// ── FTS: RemoveDocument ──────────────────────────────────────────────────

func TestFTS_RemoveDocument(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(
		"INSERT INTO documents (id, path, hash, title, doc_type) VALUES (?, ?, ?, ?, ?)",
		"d-rem", "/rem.md", "abc", "RemoveThisDocument", "markdown",
	)
	require.NoError(t, err)
	_, err = db.Exec(
		"INSERT INTO chunks (id, document_id, content) VALUES (?, ?, ?)",
		"ch-rem", "d-rem", "removable chunk content",
	)
	require.NoError(t, err)
	_, err = db.Exec(
		"INSERT INTO code_blocks (id, document_id, content, language) VALUES (?, ?, ?, ?)",
		"cb-rem", "d-rem", "removable code", "go",
	)
	require.NoError(t, err)

	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM documents_fts WHERE documents_fts MATCH 'RemoveThisDocument'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0)

	client := NewFTSClient(db)
	// RemoveDocument cleans up FTS entries for all content types.
	err = client.RemoveDocument("d-rem")
	require.NoError(t, err)
}

// ── FTS: RebuildIndex ──────────────────────────────────────────────────

func TestFTS_RebuildIndex(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	client := NewFTSClient(db)
	err = client.RebuildIndex()
	require.NoError(t, err)
}

// ── FTS: OptimizeIndex ───────────────────────────────────────────────────

func TestFTS_OptimizeIndex(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	client := NewFTSClient(db)
	err = client.OptimizeIndex()
	assert.NoError(t, err)
}

// ── FTS: searchTable unknown ─────────────────────────────────────────────

func TestFTS_SearchTableUnknown(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	client := NewFTSClient(db)
	_, _, err = client.searchTable("nonexistent_fts", FTSSearchParams{Query: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown FTS table")
}

// ── FTS: ResolveChunkIDs (hybrid-first candidate translation) ─────────────

func TestFTSClient_ResolveChunkIDs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`INSERT INTO documents (id, path, hash, title, doc_type)
		VALUES ('doc-1', '/d/1.md', 'h1', 'D1', 'markdown')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO chunks (id, document_id, content) VALUES ('chunk-a', 'doc-1', 'alpha')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO chunks (id, document_id, content) VALUES ('chunk-b', 'doc-1', 'beta')`)
	require.NoError(t, err)

	client := NewFTSClient(db)
	// rowids are 1 and 2 for a fresh table (implicit rowid).
	resolved, err := client.ResolveChunkIDs([]int64{1, 2, 999})
	require.NoError(t, err)
	assert.Equal(t, "chunk-a", resolved[1])
	assert.Equal(t, "chunk-b", resolved[2])
	_, ok := resolved[999]
	assert.False(t, ok, "nonexistent rowid must not resolve")

	resolved, err = client.ResolveChunkIDs(nil)
	require.NoError(t, err)
	assert.Nil(t, resolved)
}
