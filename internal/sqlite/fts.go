// Package sqlite provides full-text search capabilities using SQLite FTS5.
package sqlite

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/rs/zerolog/log"
)

// FTSResult represents a single full-text search result.
type FTSResult struct {
	Rank        float64 `json:"rank"`
	RowID       int64   `json:"rowid"`
	TableName   string  `json:"table_name"`
	Title       string  `json:"title,omitempty"`
	Content     string  `json:"content,omitempty"`
	Heading     string  `json:"heading,omitempty"`
	SectionType string  `json:"section_type,omitempty"`
	Language    string  `json:"language,omitempty"`
	DocType     string  `json:"doc_type,omitempty"`
	EntityType  string  `json:"entity_type,omitempty"`
	Snippet     string  `json:"snippet,omitempty"`
	Highlighted string  `json:"highlighted,omitempty"`
	DocumentID  string  `json:"document_id,omitempty"`
}

// FTSSearchParams defines parameters for full-text search.
type FTSSearchParams struct {
	Query      string
	TableNames []string // If empty, search all FTS tables
	Limit      int
	Offset     int
	MaxSnippet int // Maximum snippet length in characters
}

// FTSClient provides full-text search operations using FTS5 virtual tables.
type FTSClient struct {
	db *DB
}

// NewFTSClient creates a new FTS client.
func NewFTSClient(db *DB) *FTSClient {
	return &FTSClient{db: db}
}

// Search performs a full-text search across FTS5 virtual tables.
// It searches documents_fts, chunks_fts, code_blocks_fts, and entities_fts.
func (c *FTSClient) Search(params FTSSearchParams) ([]FTSResult, int, error) {
	if params.Query == "" {
		return nil, 0, fmt.Errorf("query is required")
	}

	params = c.defaultParams(params)

	start := time.Now()
	defer func() {
		log.Debug().
			Str("query", params.Query).
			Int("result_count", 0).
			Dur("duration", time.Since(start)).
			Msg("fts search completed")
	}()

	tables := c.resolveTables(params.TableNames)

	var allResults []FTSResult
	totalCount := 0

	for _, table := range tables {
		results, count, err := c.searchTable(table, params)
		if err != nil {
			log.Warn().Err(err).Str("table", table).Msg("FTS search failed for table")
			continue
		}
		allResults = append(allResults, results...)
		totalCount += count
	}

	return allResults, totalCount, nil
}

// searchTable performs FTS search on a single virtual table.
func (c *FTSClient) searchTable(table string, params FTSSearchParams) ([]FTSResult, int, error) {
	var query string
	var args []interface{}

	switch table {
	case "documents_fts":
		query = fmt.Sprintf(`
			SELECT rank, rowid, title, content, doc_type,
				snippet(documents_fts, 0, '<b>', '</b>', '...', %[1]d) AS snippet,
				highlight(documents_fts, 0, '<mark>', '</mark>') AS highlighted
			FROM documents_fts
			WHERE documents_fts MATCH ?1
			ORDER BY rank
			LIMIT ?2 OFFSET ?3
		`, params.MaxSnippet)
		args = []interface{}{params.Query, params.Limit, params.Offset}

	case "chunks_fts":
		query = fmt.Sprintf(`
			SELECT rank, rowid, content, heading, section_type,
				snippet(chunks_fts, 0, '<b>', '</b>', '...', %[1]d) AS snippet,
				highlight(chunks_fts, 0, '<mark>', '</mark>') AS highlighted
			FROM chunks_fts
			WHERE chunks_fts MATCH ?1
			ORDER BY rank
			LIMIT ?2 OFFSET ?3
		`, params.MaxSnippet)
		args = []interface{}{params.Query, params.Limit, params.Offset}

	case "code_blocks_fts":
		query = fmt.Sprintf(`
			SELECT rank, rowid, content, language,
				snippet(code_blocks_fts, 0, '<b>', '</b>', '...', %[1]d) AS snippet,
				highlight(code_blocks_fts, 0, '<mark>', '</mark>') AS highlighted
			FROM code_blocks_fts
			WHERE code_blocks_fts MATCH ?1
			ORDER BY rank
			LIMIT ?2 OFFSET ?3
		`, params.MaxSnippet)
		args = []interface{}{params.Query, params.Limit, params.Offset}

	case "entities_fts":
		query = fmt.Sprintf(`
			SELECT rank, rowid, name, entity_type, '' AS metadata,
				snippet(entities_fts, 0, '<b>', '</b>', '...', %[1]d) AS snippet,
				highlight(entities_fts, 0, '<mark>', '</mark>') AS highlighted
			FROM entities_fts
			WHERE entities_fts MATCH ?1
			ORDER BY rank
			LIMIT ?2 OFFSET ?3
		`, params.MaxSnippet)
		args = []interface{}{params.Query, params.Limit, params.Offset}

	case "knowledge_fts":
		// Compiled knowledge entries (patterns, heuristics, ADRs, failures,
		// best-practices). category → Heading, sub_category → SectionType.
		query = fmt.Sprintf(`
			SELECT rank, rowid, title, content, category, sub_category,
				snippet(knowledge_fts, 0, '<b>', '</b>', '...', %[1]d) AS snippet,
				highlight(knowledge_fts, 0, '<mark>', '</mark>') AS highlighted
			FROM knowledge_fts
			WHERE knowledge_fts MATCH ?1
			ORDER BY rank
			LIMIT ?2 OFFSET ?3
		`, params.MaxSnippet)
		args = []interface{}{params.Query, params.Limit, params.Offset}

	default:
		return nil, 0, fmt.Errorf("unknown FTS table: %s", table)
	}

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("fts search %s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	var results []FTSResult
	for rows.Next() {
		var r FTSResult
		r.TableName = table

		switch table {
		case "documents_fts":
			if err := rows.Scan(&r.Rank, &r.RowID, &r.Title, &r.Content, &r.DocType, &r.Snippet, &r.Highlighted); err != nil {
				return nil, 0, fmt.Errorf("scan doc result: %w", err)
			}
		case "chunks_fts":
			if err := rows.Scan(&r.Rank, &r.RowID, &r.Content, &r.Heading, &r.SectionType, &r.Snippet, &r.Highlighted); err != nil {
				return nil, 0, fmt.Errorf("scan chunk result: %w", err)
			}
		case "code_blocks_fts":
			if err := rows.Scan(&r.Rank, &r.RowID, &r.Content, &r.Language, &r.Snippet, &r.Highlighted); err != nil {
				return nil, 0, fmt.Errorf("scan code result: %w", err)
			}
		case "entities_fts":
			if err := rows.Scan(&r.Rank, &r.RowID, &r.Title, &r.EntityType, &r.Content, &r.Snippet, &r.Highlighted); err != nil {
				return nil, 0, fmt.Errorf("scan entity result: %w", err)
			}
		case "knowledge_fts":
			if err := rows.Scan(&r.Rank, &r.RowID, &r.Title, &r.Content, &r.Heading, &r.SectionType, &r.Snippet, &r.Highlighted); err != nil {
				return nil, 0, fmt.Errorf("scan knowledge result: %w", err)
			}
		}

		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration: %w", err)
	}

	// Get total count
	countQuery := c.countQuery(table)
	var totalCount int
	if err := c.db.QueryRow(countQuery, params.Query).Scan(&totalCount); err != nil {
		totalCount = len(results) // fallback
	}

	return results, totalCount, nil
}

// countQuery returns the count query for an FTS table.
func (c *FTSClient) countQuery(table string) string {
	switch table {
	case "documents_fts":
		return "SELECT COUNT(*) FROM documents_fts WHERE documents_fts MATCH ?1"
	case "chunks_fts":
		return "SELECT COUNT(*) FROM chunks_fts WHERE chunks_fts MATCH ?1"
	case "code_blocks_fts":
		return "SELECT COUNT(*) FROM code_blocks_fts WHERE code_blocks_fts MATCH ?1"
	case "entities_fts":
		return "SELECT COUNT(*) FROM entities_fts WHERE entities_fts MATCH ?1"
	default:
		return "SELECT 0"
	}
}

// ResolveChunkIDs maps integer chunks-table rowids to their string ids.
// The layered search uses this to translate FTS5 candidates (identified by
// "chunks_fts_<rowid>") into vector-store candidate IDs for the hybrid-first
// vector layer.
func (c *FTSClient) ResolveChunkIDs(rowids []int64) (map[int64]string, error) {
	if len(rowids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(rowids))
	args := make([]interface{}, len(rowids))
	for i, r := range rowids {
		placeholders[i] = "?"
		args[i] = r
	}
	rows, err := c.db.Query(
		"SELECT rowid, id FROM chunks WHERE rowid IN ("+strings.Join(placeholders, ",")+")",
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("resolve chunk ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[int64]string, len(rowids))
	for rows.Next() {
		var rowid int64
		var id string
		if err := rows.Scan(&rowid, &id); err != nil {
			return nil, fmt.Errorf("scan chunk id: %w", err)
		}
		out[rowid] = id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// IndexDocument adds a document to the FTS index.
//
// The first parameter is the FTS rowid, which MUST be the integer rowid of the
// corresponding documents table row (not the document's string uuid).
// documents_fts is a self-managed FTS5 table, so the write uses INSERT OR
// REPLACE: a document re-indexed over an entry already synced by the
// content-table triggers replaces that entry instead of colliding (UNIQUE) or
// duplicating it.
func (c *FTSClient) IndexDocument(id string, title, content, docType string) error {
	_, err := c.db.Exec(
		`INSERT OR REPLACE INTO documents_fts(rowid, title, content, doc_type)
		 VALUES (?, ?, ?, ?)`,
		id, title, content, docType,
	)
	return err
}

// IndexChunk adds a chunk to the FTS index.
//
// The first parameter is the FTS rowid, which MUST be the integer rowid of the
// corresponding chunks table row (not the chunk's string uuid). chunks_fts is
// an external-content table (content='chunks'); the write is delete-then-insert
// so that re-indexing an entry already synced by the content-table triggers
// replaces it instead of appending stale tokens.
func (c *FTSClient) IndexChunk(id string, content, heading, sectionType string) error {
	if _, err := c.db.Exec(
		`INSERT INTO chunks_fts(chunks_fts, rowid, content, heading, section_type)
		 VALUES ('delete', ?, ?, ?, ?)`,
		id, content, heading, sectionType,
	); err != nil {
		return err
	}
	_, err := c.db.Exec(
		`INSERT INTO chunks_fts(rowid, content, heading, section_type)
		 VALUES (?, ?, ?, ?)`,
		id, content, heading, sectionType,
	)
	return err
}

// IndexCodeBlock adds a code block to the FTS index.
//
// The first parameter is the FTS rowid, which MUST be the integer rowid of the
// corresponding code_blocks table row (not the code block's string uuid).
// code_blocks_fts is an external-content table (content='code_blocks'); the
// write is delete-then-insert so it stays idempotent against entries already
// synced by the content-table triggers.
func (c *FTSClient) IndexCodeBlock(id string, content, language string) error {
	if _, err := c.db.Exec(
		`INSERT INTO code_blocks_fts(code_blocks_fts, rowid, content, language)
		 VALUES ('delete', ?, ?, ?)`,
		id, content, language,
	); err != nil {
		return err
	}
	_, err := c.db.Exec(
		`INSERT INTO code_blocks_fts(rowid, content, language)
		 VALUES (?, ?, ?)`,
		id, content, language,
	)
	return err
}

// IndexEntity adds an entity to the FTS index.
// The entity must already exist in the entities table; its integer rowid
// is looked up and used as the FTS rowid.
func (c *FTSClient) IndexEntity(id string, name, entityType, metadata string) error {
	var rowID int64
	if err := c.db.QueryRow("SELECT rowid FROM entities WHERE id = ?", id).Scan(&rowID); err != nil {
		return fmt.Errorf("lookup entity rowid: %w", err)
	}
	_, err := c.db.Exec(
		`INSERT INTO entities_fts(rowid, name, entity_type, metadata)
		 VALUES (?, ?, ?, ?)`,
		rowID, name, entityType, metadata,
	)
	return err
}

// RemoveDocument removes a document's FTS entries for the self-managed FTS
// tables (documents_fts, entities_fts). Both are self-managed FTS5 tables, so
// plain DELETEs are used (the FTS5 special 'delete' command is invalid without
// content=) and they are idempotent — removing an already-removed rowid is a
// harmless no-op.
//
// The external-content tables (chunks_fts, code_blocks_fts) are deliberately
// NOT touched here: those indexes are kept consistent by the content-table
// DELETE triggers (chunks_ad, code_blocks_ad), which fire when the owning rows
// are deleted from SQLite. Removing the same rowids twice would corrupt the
// external-content index (FTS5 returns SQLITE_CORRUPT_VTAB on a second removal
// of an already-removed rowid).
func (c *FTSClient) RemoveDocument(documentID string) error {
	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer safe.Rollback(tx)

	// Delete from documents_fts — the document's own entry, keyed by the
	// documents table rowid.
	var docRowID int64
	if err := tx.QueryRow("SELECT rowid FROM documents WHERE id = ?", documentID).Scan(&docRowID); err == nil {
		if _, err := tx.Exec("DELETE FROM documents_fts WHERE rowid = ?", docRowID); err != nil {
			return fmt.Errorf("delete document fts: %w", err)
		}
	}

	// Delete from entities_fts — query entity rowids first since the FTS5
	// rowid must be an integer, not a document-id string.
	entityRows, err := tx.Query("SELECT rowid FROM entities WHERE id = ?", documentID)
	if err != nil {
		return fmt.Errorf("query entities: %w", err)
	}
	defer func() { _ = entityRows.Close() }()

	for entityRows.Next() {
		var rowID int64
		if err := entityRows.Scan(&rowID); err != nil {
			return fmt.Errorf("scan entity rowid: %w", err)
		}
		if _, err := tx.Exec("DELETE FROM entities_fts WHERE rowid = ?", rowID); err != nil {
			return fmt.Errorf("delete entity fts: %w", err)
		}
	}
	_ = entityRows.Close()

	return tx.Commit()
}

// RebuildIndex drops and recreates all FTS indexes from source content tables.
func (c *FTSClient) RebuildIndex() error {
	log.Info().Msg("rebuilding FTS indexes")

	tables := []struct {
		FTSName      string
		ContentTable string
	}{
		{"documents_fts", "documents"},
		{"chunks_fts", "chunks"},
		{"code_blocks_fts", "code_blocks"},
		{"entities_fts", "entities"},
	}

	for _, t := range tables {
		// Rebuild using FTS5 rebuild command
		if _, err := c.db.Exec(fmt.Sprintf("INSERT INTO %s(%s) VALUES('rebuild')", t.FTSName, t.FTSName)); err != nil {
			return fmt.Errorf("rebuild %s: %w", t.FTSName, err)
		}
		log.Info().Str("fts_table", t.FTSName).Msg("FTS index rebuilt")
	}

	return nil
}

// OptimizeIndex runs the FTS5 merge command to optimize the index.
func (c *FTSClient) OptimizeIndex() error {
	log.Info().Msg("optimizing FTS indexes")

	tables := []string{"documents_fts", "chunks_fts", "code_blocks_fts", "entities_fts"}
	for _, table := range tables {
		if _, err := c.db.Exec(fmt.Sprintf("INSERT INTO %s(%s) VALUES('optimize')", table, table)); err != nil {
			return fmt.Errorf("optimize %s: %w", table, err)
		}
	}

	return nil
}

// SanitizeFTSQuery sanitizes a user-supplied search query for safe use in an
// FTS5 MATCH expression.
//
// The input is treated as LITERAL search terms, never as FTS5 query
// language. FTS5 operators such as `"`, `*`, `^`, `(`, `)`, `+`, `-` and
// `:` would otherwise let a caller inject their own match grammar (boolean
// expressions, wildcard expansions, column filters, ...) — a lightweight DoS
// vector and a correctness hazard. Query-language features are intentionally
// NOT supported.
//
// Sanitization is allowlist-based:
//   - Terms are split on whitespace.
//   - Within a term, only letters, digits, `_` and `-` are kept; every other
//     character (including all FTS5 operators) is stripped.
//   - Each surviving term is wrapped in double quotes so it is matched as a
//     literal phrase token.
//
// Examples:
//
//	"hello world"      → `"hello" "world"`
//	`a* b^ c`          → `"a" "b" "c"`
//	`(hello OR world)` → `"hello" "OR" "world"`
//	`he"llo`           → `"hello"`
func SanitizeFTSQuery(query string) string {
	// Trim whitespace
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	// Split into terms, strip non-allowlist characters, and quote each.
	terms := strings.Fields(query)
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		term = stripNonAllowlist(term)
		if term == "" {
			continue
		}
		// Defensive escape: FTS5 doubles an embedded quote to match a
		// literal quote. The allowlist already strips quotes, but keeping
		// the escape keeps this function correct if the allowlist is ever
		// relaxed.
		term = strings.ReplaceAll(term, `"`, `""`)
		quoted = append(quoted, `"`+term+`"`)
	}

	return strings.Join(quoted, " ")
}

// allowlistFTSRune reports whether a rune may appear inside a literal FTS5
// term. Everything else (operators, quotes, punctuation) is stripped so the
// user query can never inject FTS5 query grammar.
func allowlistFTSRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

// stripNonAllowlist removes every character that is not on the FTS5
// allowlist.
func stripNonAllowlist(s string) string {
	return strings.Map(func(r rune) rune {
		if allowlistFTSRune(r) {
			return r
		}
		return -1
	}, s)
}

// defaultParams fills in sensible defaults for search parameters.
func (c *FTSClient) defaultParams(params FTSSearchParams) FTSSearchParams {
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Offset < 0 {
		params.Offset = 0
	}
	if params.MaxSnippet <= 0 {
		params.MaxSnippet = 200
	}
	params.Query = SanitizeFTSQuery(params.Query)
	return params
}

// resolveTables returns the list of FTS tables to search.
func (c *FTSClient) resolveTables(tables []string) []string {
	if len(tables) > 0 {
		return tables
	}
	return []string{"documents_fts", "chunks_fts", "code_blocks_fts", "entities_fts", "knowledge_fts"}
}

// Ensure FTSResult implements a convenience String method.
func (r FTSResult) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] score=%.4f", r.TableName, r.Rank)
	if r.Title != "" {
		fmt.Fprintf(&b, " title=%q", r.Title)
	}
	if r.Heading != "" {
		fmt.Fprintf(&b, " heading=%q", r.Heading)
	}
	if r.Snippet != "" {
		fmt.Fprintf(&b, " snippet=%q", truncateText(r.Snippet, 80))
	}
	return b.String()
}

// truncateText truncates text to the given max length, adding ellipsis.
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

// Ensure compile-time checks.
var _ = (FTSClient)(struct{ db *DB }{})
