package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// MemoryType identifies the kind of kernel memory.
type MemoryType string

const (
	MemoryLearning  MemoryType = "learning"
	MemoryFailure   MemoryType = "failure"
	MemoryPattern   MemoryType = "pattern"
	MemoryDecision  MemoryType = "decision"
	MemoryKnowledge MemoryType = "knowledge"
)

// Memory is the Kernel's unified access to its consciousness.
//
// It reads from the SQLite knowledge base (.cosca/knowledge.db), which is
// populated by the knowledge compiler from .cosca/framework (authoritative
// versioned source). The kernel NEVER writes to .cosca/framework directly.
type Memory struct {
	db *sql.DB
}

// MemoryItem is a single unit of kernel memory (a learning, failure, or pattern).
type MemoryItem struct {
	ID         string     `json:"id"`
	Type       MemoryType `json:"type"`
	Agent      string     `json:"agent"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	Tags       []string   `json:"tags"`
	Level      int        `json:"level"`
	Confidence float64    `json:"confidence"`
	Source     string     `json:"source"`
	CreatedAt  time.Time  `json:"created_at"`
}

// OpenMemory opens the knowledge base. If the database does not exist or
// cannot be opened, it returns an error — the caller decides the fallback.
func OpenMemory(dbPath string) (*Memory, error) {
	if dbPath == "" {
		dbPath = ".cosca/knowledge.db"
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("kernel: open memory db: %w", err)
	}
	// Verify the database is reachable and has the expected tables.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("kernel: memory db unreachable: %w", err)
	}
	return &Memory{db: db}, nil
}

// Close releases the database handle.
func (m *Memory) Close() error {
	if m == nil || m.db == nil {
		return nil
	}
	return m.db.Close()
}

// Learnings returns all indexed learning documents from the knowledge base.
// Learnings live in `documents` (path contains "learnings.md") with their
// content in `chunks`.
func (m *Memory) Learnings(ctx context.Context, limit int) ([]MemoryItem, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT d.id, d.title, d.path, d.created_at, COUNT(c.id) AS chunk_count
		FROM documents d
		LEFT JOIN chunks c ON c.document_id = d.id
		WHERE d.path LIKE '%learnings.md'
		GROUP BY d.id
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("kernel: query learnings: %w", err)
	}
	defer rows.Close()

	items := []MemoryItem{}
	for rows.Next() {
		var id, title, path string
		var createdAt sql.NullString
		var chunkCount int
		if err := rows.Scan(&id, &title, &path, &createdAt, &chunkCount); err != nil {
			return nil, err
		}
		items = append(items, MemoryItem{
			ID:        id,
			Type:      MemoryLearning,
			Agent:     agentFromPath(path),
			Title:     title,
			Source:    path,
			CreatedAt: parseTimestamp(createdAt.String),
		})
	}
	return items, rows.Err()
}

// LearningsCount returns the number of indexed learning documents.
func (m *Memory) LearningsCount(ctx context.Context) (int, error) {
	var n int
	err := m.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM documents WHERE path LIKE '%learnings.md'`).Scan(&n)
	return n, err
}

// Failures returns all indexed failure documents.
func (m *Memory) Failures(ctx context.Context, limit int) ([]MemoryItem, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT d.id, d.title, d.path
		FROM documents d
		WHERE d.path LIKE '%failures.md'
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("kernel: query failures: %w", err)
	}
	defer rows.Close()

	items := []MemoryItem{}
	for rows.Next() {
		var id, title, path string
		if err := rows.Scan(&id, &title, &path); err != nil {
			return nil, err
		}
		items = append(items, MemoryItem{
			ID:     id,
			Type:   MemoryFailure,
			Agent:  agentFromPath(path),
			Title:  title,
			Source: path,
		})
	}
	return items, rows.Err()
}

// KnowledgeEntries returns compiled knowledge from the knowledge_entries table.
func (m *Memory) KnowledgeEntries(ctx context.Context, category string, limit int) ([]MemoryItem, error) {
	if limit <= 0 {
		limit = 100
	}
	q := `SELECT id, category, title, content, tags, confidence, source, updated_at
	      FROM knowledge_entries`
	var args []interface{}
	if category != "" {
		q += ` WHERE category = ?`
		args = append(args, category)
	}
	q += ` LIMIT ?`
	args = append(args, limit)

	rows, err := m.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("kernel: query knowledge entries: %w", err)
	}
	defer rows.Close()

	var items []MemoryItem
	for rows.Next() {
		var id int
		var cat, title, content, tagsJSON, source, updatedAt string
		var confidence float64
		if err := rows.Scan(&id, &cat, &title, &content, &tagsJSON, &confidence, &source, &updatedAt); err != nil {
			return nil, err
		}
		var tags []string
		_ = json.Unmarshal([]byte(tagsJSON), &tags)
		ts := parseTimestamp(updatedAt)
		items = append(items, MemoryItem{
			ID:         fmt.Sprintf("ke-%d", id),
			Type:       MemoryKnowledge,
			Agent:      "cosca-family",
			Title:      title,
			Content:    content,
			Tags:       tags,
			Confidence: confidence,
			Source:     source,
			CreatedAt:  ts,
		})
	}
	return items, rows.Err()
}

// Search performs a semantic search over indexed documents via FTS5.
// Returns top results ordered by relevance.
//
// Degradation path: if FTS5 is unavailable (table missing), the MATCH query
// errors, or the MATCH query succeeds but returns no rows (e.g., empty FTS
// index), Search falls back to a LIKE scan so callers always get useful
// results instead of a hard failure or an empty response.
func (m *Memory) Search(ctx context.Context, query string, limit int) ([]MemoryItem, error) {
	if limit <= 0 {
		limit = 10
	}
	// Escape double quotes for FTS5 query safety.
	q := strings.ReplaceAll(query, `"`, `""`)
	rows, err := m.db.QueryContext(ctx, `
		SELECT c.document_id, d.path, d.title, c.content, c.heading
		FROM chunks_fts f
		JOIN chunks c ON c.id = f.id
		JOIN documents d ON d.id = c.document_id
		WHERE chunks_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, q, limit)
	if err != nil {
		// FTS5 MATCH errors (e.g., empty query, malformed expression, missing
		// table) fall back to LIKE search.
		return m.searchFallback(ctx, query, limit)
	}
	items, scanErr := scanSearchRows(rows)
	rows.Close() // close before fallback to release the connection
	if scanErr != nil {
		return nil, scanErr
	}
	// FTS5 index empty or query matched nothing — degrade to LIKE for recall.
	if len(items) == 0 {
		return m.searchFallback(ctx, query, limit)
	}
	return items, nil
}

// searchFallback uses LIKE when FTS5 match fails or returns no results.
func (m *Memory) searchFallback(ctx context.Context, query string, limit int) ([]MemoryItem, error) {
	like := "%" + query + "%"
	rows, err := m.db.QueryContext(ctx, `
		SELECT d.id, d.path, d.title, c.content, c.heading
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.content LIKE ? OR d.title LIKE ?
		LIMIT ?`, like, like, limit)
	if err != nil {
		return nil, fmt.Errorf("kernel: search fallback: %w", err)
	}
	defer rows.Close()
	return scanSearchRows(rows)
}

// scanSearchRows scans search result rows (document id, path, title, content,
// heading) into MemoryItems. Shared by the FTS5 path and the LIKE fallback.
func scanSearchRows(rows *sql.Rows) ([]MemoryItem, error) {
	items := []MemoryItem{}
	for rows.Next() {
		var docID, path, title, content, heading string
		if err := rows.Scan(&docID, &path, &title, &content, &heading); err != nil {
			return nil, err
		}
		items = append(items, MemoryItem{
			ID:      docID,
			Type:    memoryTypeFromPath(path),
			Agent:   agentFromPath(path),
			Title:   firstNonEmpty(title, heading, "document"),
			Content: truncate(content, 500),
			Source:  path,
		})
	}
	return items, rows.Err()
}

// Stats returns memory statistics from the knowledge base.
func (m *Memory) Stats(ctx context.Context) (map[string]int, error) {
	stats := make(map[string]int)
	var err error

	if stats["documents"], err = m.count(ctx, "SELECT COUNT(*) FROM documents"); err != nil {
		return nil, err
	}
	if stats["chunks"], err = m.count(ctx, "SELECT COUNT(*) FROM chunks"); err != nil {
		return nil, err
	}
	// vectors is optional: it only exists when vector storage is enabled on
	// this knowledge base. Report 0 instead of failing when the table is absent.
	if stats["vectors"], err = m.countTable(ctx, "vectors"); err != nil {
		return nil, err
	}
	if stats["learnings"], err = m.count(ctx, "SELECT COUNT(*) FROM documents WHERE path LIKE '%learnings.md'"); err != nil {
		return nil, err
	}
	if stats["failures"], err = m.count(ctx, "SELECT COUNT(*) FROM documents WHERE path LIKE '%failures.md'"); err != nil {
		return nil, err
	}
	if stats["knowledge_entries"], err = m.count(ctx, "SELECT COUNT(*) FROM knowledge_entries"); err != nil {
		return nil, err
	}
	return stats, nil
}

func (m *Memory) count(ctx context.Context, q string) (int, error) {
	var n int
	err := m.db.QueryRowContext(ctx, q).Scan(&n)
	return n, err
}

// countTable counts rows in a named table. If the table does not exist in
// this database (e.g., optional vector storage), it returns 0 instead of an
// error, so statistics degrade gracefully.
func (m *Memory) countTable(ctx context.Context, table string) (int, error) {
	var exists int
	err := m.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type IN ('table','view') AND name = ?`, table).Scan(&exists)
	if err != nil {
		return 0, err
	}
	if exists == 0 {
		return 0, nil
	}
	return m.count(ctx, "SELECT COUNT(*) FROM "+table)
}

// ── helpers ────────────────────────────────────────────────────────────────

// parseTimestamp parses a timestamp stored in the knowledge base. The compiler
// writes full timestamps as "2006-01-02 15:04:05" and dates as "2006-01-02";
// both layouts are accepted. Unparseable values yield the zero time.
func parseTimestamp(s string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02"} {
		if ts, err := time.Parse(layout, s); err == nil {
			return ts
		}
	}
	return time.Time{}
}

// agentFromPath extracts the agent name from a memory path like
// ".cosca/memory/agent/cosca-kernel/learnings.md".
func agentFromPath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "agent" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "cosca-family"
}

// memoryTypeFromPath maps a path to a memory type.
func memoryTypeFromPath(path string) MemoryType {
	switch {
	case strings.Contains(path, "learnings.md"):
		return MemoryLearning
	case strings.Contains(path, "failures.md"):
		return MemoryFailure
	case strings.Contains(path, "patterns.md"):
		return MemoryPattern
	default:
		return MemoryKnowledge
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
