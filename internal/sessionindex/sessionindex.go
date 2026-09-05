// Package sessionindex implements deterministic, LLM-free full-text search
// (FTS5) over past chat sessions.
//
// Sessions are stored as JSONL in .cosca/sessions/{id}.jsonl. Line 1 is a
// `meta` record (id/model/agent; optionally carrying `parent_session_id` for
// session lineage), followed by `message` records (role/content/timestamp).
//
// The index lives in .cosca/session.db — a dedicated SQLite file, never the
// knowledge base. Indexing is explicit (`cosca session index`); nothing
// auto-indexes. Search is a zero-LLM, read-only MATCH over the FTS5 table,
// ordered by message timestamp descending.
package sessionindex

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Hit is a single full-text match against a session message.
type Hit struct {
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
	Line      int    `json:"line"`
}

// sessionRecord is a single line of a session JSONL file.
type sessionRecord struct {
	Type            string `json:"type"`
	ID              string `json:"id"`
	Role            string `json:"role"`
	Content         string `json:"content"`
	Timestamp       string `json:"timestamp"`
	ParentSessionID string `json:"parent_session_id"`
}

// ── Schema ───────────────────────────────────────────────────────────────────

// schemaDDL returns the DDL statements that back the session index.
//
// session_message_rows is the upsertable source of truth (message_id PRIMARY
// KEY); session_messages is the FTS5 virtual table search reads from.
// Triggers keep the two in sync, mirroring the knowledge_entries /
// knowledge_fts pattern in internal/knowledge/compiler.go. Only `content` is
// tokenized: message_id, session_id, role and ts are UNINDEXED (stored, not
// full-text searched) so a query like "cosca" never matches every row via the
// session_id column.
//
// session_messages is a SELF-MANAGED FTS5 table (no content= option), so the
// sync triggers must use a plain `DELETE FROM session_messages WHERE rowid =
// old.rowid` — the FTS5 special 'delete' INSERT command only works on
// external-content tables (migration v5 lesson, internal/sqlite/migrations.go).
func schemaDDL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS session_message_rows (
			message_id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			ts INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS session_messages USING fts5(
			message_id UNINDEXED,
			session_id UNINDEXED,
			role UNINDEXED,
			content,
			ts UNINDEXED,
			tokenize='unicode61'
		)`,
		`CREATE TABLE IF NOT EXISTS session_meta (
			session_id TEXT PRIMARY KEY,
			parent_session_id TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TRIGGER IF NOT EXISTS session_message_rows_ai AFTER INSERT ON session_message_rows BEGIN
			INSERT INTO session_messages(rowid, message_id, session_id, role, content, ts)
			VALUES (new.rowid, new.message_id, new.session_id, new.role, new.content, new.ts);
		END`,
		`CREATE TRIGGER IF NOT EXISTS session_message_rows_ad AFTER DELETE ON session_message_rows BEGIN
			DELETE FROM session_messages WHERE rowid = old.rowid;
		END`,
		`CREATE TRIGGER IF NOT EXISTS session_message_rows_au AFTER UPDATE ON session_message_rows BEGIN
			DELETE FROM session_messages WHERE rowid = old.rowid;
			INSERT INTO session_messages(rowid, message_id, session_id, role, content, ts)
			VALUES (new.rowid, new.message_id, new.session_id, new.role, new.content, new.ts);
		END`,
	}
}

// EnsureSchema creates the session index tables if they do not exist.
// Idempotent; safe to call on every read path so a search before the first
// index simply returns zero hits instead of a "no such table" error.
func EnsureSchema(db *sql.DB) error {
	for _, stmt := range schemaDDL() {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("init session schema: %w", err)
		}
	}
	return nil
}

// ── Indexing ─────────────────────────────────────────────────────────────────

// IndexSessions walks <coscaDir>/sessions/*.jsonl, parses each session file
// and upserts every message into the FTS5 index. Malformed lines and
// non-message records (usage, ...) are skipped, never fatal. Returns the
// number of messages indexed.
//
// Idempotent: each message is upserted by message_id (<sessionID>:<line>),
// so re-running never accumulates duplicates.
func IndexSessions(coscaDir string, db *sql.DB) (int, error) {
	if err := EnsureSchema(db); err != nil {
		return 0, err
	}

	sessionsDir := filepath.Join(coscaDir, "sessions")
	files, err := filepath.Glob(filepath.Join(sessionsDir, "*.jsonl"))
	if err != nil {
		return 0, fmt.Errorf("glob sessions: %w", err)
	}

	total := 0
	for _, f := range files {
		n, err := indexSessionFile(db, f)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// indexSessionFile parses a single session JSONL file into the index.
func indexSessionFile(db *sql.DB, path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open session file: %w", err)
	}
	defer func() { _ = f.Close() }()

	sessionID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	parentID := ""
	count := 0

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}

		var rec sessionRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			// Malformed line — skip, never abort the index.
			continue
		}

		switch rec.Type {
		case "meta":
			if rec.ID != "" {
				sessionID = rec.ID
			}
			parentID = rec.ParentSessionID
			if err := upsertMeta(tx, sessionID, parentID); err != nil {
				return count, err
			}
		case "message":
			if rec.Role == "" || rec.Content == "" {
				continue
			}
			if err := upsertMessage(tx, messageID(sessionID, line), sessionID, rec.Role, rec.Content, parseTimestamp(rec.Timestamp)); err != nil {
				return count, err
			}
			count++
		default:
			// usage and other record types are not searchable.
		}
	}
	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("scan session file: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return count, fmt.Errorf("commit tx: %w", err)
	}
	return count, nil
}

// upsertMessage inserts or replaces a message by message_id. On conflict the
// row is updated in session_message_rows, which fires the au trigger and
// keeps the FTS5 table in sync (no INSERT trigger fires on upsert-update).
func upsertMessage(tx *sql.Tx, messageID, sessionID, role, content string, ts int64) error {
	_, err := tx.Exec(
		`INSERT INTO session_message_rows (message_id, session_id, role, content, ts)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(message_id) DO UPDATE SET
			session_id = excluded.session_id,
			role = excluded.role,
			content = excluded.content,
			ts = excluded.ts`,
		messageID, sessionID, role, content, ts,
	)
	if err != nil {
		return fmt.Errorf("upsert message: %w", err)
	}
	return nil
}

// upsertMeta records a session's lineage metadata (parent_session_id may be
// absent — stored as empty string, handled gracefully everywhere).
func upsertMeta(tx *sql.Tx, sessionID, parentID string) error {
	_, err := tx.Exec(
		`INSERT INTO session_meta (session_id, parent_session_id) VALUES (?, ?)
		 ON CONFLICT(session_id) DO UPDATE SET parent_session_id = excluded.parent_session_id`,
		sessionID, parentID,
	)
	if err != nil {
		return fmt.Errorf("upsert session meta: %w", err)
	}
	return nil
}

// messageID builds the deterministic idempotency key for a message:
// <sessionID>:<jsonl line>. Stable across re-index runs.
func messageID(sessionID string, line int) string {
	return fmt.Sprintf("%s:%d", sessionID, line)
}

// parseTimestamp converts an RFC3339 timestamp into Unix seconds; 0 on empty
// or unparseable input.
func parseTimestamp(s string) int64 {
	if s == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// ── Search ───────────────────────────────────────────────────────────────────

// Search performs a zero-LLM full-text search over indexed session messages.
// The user query is sanitized (sanitizeFTS5Query) so it is always treated as
// literal terms, never FTS5 grammar. Results are ordered by message timestamp
// descending (rowid as tiebreaker), capped at limit.
func Search(ctx context.Context, db *sql.DB, query string, limit int) ([]Hit, error) {
	sanitized := sanitizeFTS5Query(query)
	if sanitized == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	rows, err := db.QueryContext(ctx,
		`SELECT message_id, session_id, role, content, ts
		 FROM session_messages
		 WHERE session_messages MATCH ?1
		 ORDER BY ts DESC, rowid DESC
		 LIMIT ?2`,
		sanitized, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("session search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	hits := make([]Hit, 0, limit)
	for rows.Next() {
		var (
			h         Hit
			messageID string
		)
		if err := rows.Scan(&messageID, &h.SessionID, &h.Role, &h.Content, &h.Timestamp); err != nil {
			return nil, fmt.Errorf("scan session hit: %w", err)
		}
		h.Line = lineFromMessageID(messageID)
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session hits: %w", err)
	}
	return hits, nil
}

// lineFromMessageID recovers the JSONL line number encoded in the message_id
// (the segment after the last colon).
func lineFromMessageID(messageID string) int {
	i := strings.LastIndex(messageID, ":")
	if i < 0 {
		return 0
	}
	n, err := strconv.Atoi(messageID[i+1:])
	if err != nil {
		return 0
	}
	return n
}

// ── Lineage ──────────────────────────────────────────────────────────────────

// SessionLineage returns sessionID followed by every session whose meta
// references it (directly or transitively) as parent_session_id. A session
// whose meta has no parent_session_id yields just [sessionID]. The base
// session itself is always first, even if it is unknown to the index.
func SessionLineage(db *sql.DB, sessionID string) ([]string, error) {
	lineage := []string{sessionID}
	seen := map[string]bool{sessionID: true}
	frontier := []string{sessionID}

	for len(frontier) > 0 {
		var next []string
		for _, parent := range frontier {
			rows, err := db.Query(
				`SELECT session_id FROM session_meta WHERE parent_session_id = ?`,
				parent,
			)
			if err != nil {
				return lineage, fmt.Errorf("lineage query: %w", err)
			}
			for rows.Next() {
				var child string
				if err := rows.Scan(&child); err != nil {
					_ = rows.Close()
					return lineage, fmt.Errorf("lineage scan: %w", err)
				}
				if !seen[child] {
					seen[child] = true
					lineage = append(lineage, child)
					next = append(next, child)
				}
			}
			_ = rows.Close()
			if err := rows.Err(); err != nil {
				return lineage, err
			}
		}
		frontier = next
	}
	return lineage, nil
}

// ── Query sanitization ───────────────────────────────────────────────────────

// sanitizeFTS5Query turns a raw user query into safe, literal FTS5 terms.
//
// FTS5 has its own grammar (", *, ^, (, ), +, -, :, AND/OR/NOT, ...). A raw
// user string passed to MATCH can inject operators and mis-parse or crash.
// The Hermes analysis (session_search_tool.py) flags quote and colon as the
// classic breakers of naive FTS5 queries.
//
// Approach (allowlist-based, mirrors internal/sqlite.SanitizeFTSQuery):
// split on whitespace; inside each term keep only letters, digits, `_` and
// `-`, stripping every other character (quotes, parens, colons, operators).
// Each surviving term is wrapped in double quotes so it is matched as a
// literal phrase token.
//
// Examples:
//
//	`hello world`   → `"hello" "world"`
//	`"tatuagem"`    → `"tatuagem"`
//	`foo:bar baz:`  → `"foo" "bar" "baz"`
//	`(a OR b)`      → `"a" "OR" "b"`   (OR matched literally)
//	``              → ``
func sanitizeFTS5Query(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	terms := strings.Fields(query)
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		term = stripNonAllowlist(term)
		if term == "" {
			continue
		}
		quoted = append(quoted, `"`+term+`"`)
	}
	return strings.Join(quoted, " ")
}

// ftsAllowlistRune reports whether a rune may appear inside a literal FTS5
// term. Everything else is stripped so a user query can never inject FTS5
// query grammar.
func ftsAllowlistRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

// stripNonAllowlist removes every character that is not on the allowlist.
func stripNonAllowlist(s string) string {
	return strings.Map(func(r rune) rune {
		if ftsAllowlistRune(r) {
			return r
		}
		return -1
	}, s)
}
