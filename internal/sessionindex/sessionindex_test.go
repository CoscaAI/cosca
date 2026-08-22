//
// Tests for the session index (internal/sessionindex).
//
// Covers:
//   - IndexSessions parses a temp jsonl (meta + messages + malformed + usage
//     lines) and returns the correct indexed count
//   - Re-index is idempotent (upsert by message_id — no duplicates)
//   - Missing sessions dir is graceful (0, nil)
//   - Search matches content, respects limit, and tolerates malformed queries
//     (`"`, `:`) without erroring
//   - sanitizeFTS5Query strips FTS5 grammar
//   - SessionLineage returns base session + descendants (transitively), and
//     handles absent parent_session_id gracefully
//
// Uses a temp SQLite DB (t.TempDir) — never the real knowledge.db.

package sessionindex

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// openTempDB opens a fresh SQLite DB inside a temp dir.
func openTempDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "session.db"))
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// writeSession writes a jsonl session file under <tmp>/.cosca/sessions/.
func writeSession(t *testing.T, coscaDir, name, content string) {
	t.Helper()
	sessionsDir := filepath.Join(coscaDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o700); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write session %s: %v", name, err)
	}
}

// countMessages returns the total number of rows in the FTS5 table.
func countMessages(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM session_messages").Scan(&n); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	return n
}

const voiceJSONL = `{"type":"meta","id":"voice","model":"deepseek","agent":"default","created_at":"2026-08-01T22:45:34Z","updated_at":"2026-08-02T08:13:44Z"}
{"type":"message","role":"user","content":"Ola, tudo bem?","timestamp":"2026-08-02T08:13:44.21377824Z"}
{"type":"message","role":"assistant","content":"Salve, chef. O último assunto foi o CKL.","timestamp":"2026-08-02T08:13:44.21377824Z"}
{"type":"usage","prompt_tokens":1,"completion_tokens":2,"total_tokens":3}
this is not json
{"type":"message","role":"user","content":"Kernel, tu fala normal.","timestamp":"2026-08-02T08:13:44.21377824Z"}
`

// =============================================================================
// IndexSessions
// =============================================================================

func TestIndexSessions_ParsesAndCounts(t *testing.T) {
	db := openTempDB(t)
	coscaDir := t.TempDir()
	writeSession(t, coscaDir, "voice.jsonl", voiceJSONL)

	n, err := IndexSessions(coscaDir, db)
	if err != nil {
		t.Fatalf("IndexSessions: %v", err)
	}
	// 3 message records; the usage line, the malformed line and the meta line
	// must not be indexed.
	if n != 3 {
		t.Fatalf("indexed %d, want 3", n)
	}
	if got := countMessages(t, db); got != 3 {
		t.Fatalf("session_messages rows = %d, want 3", got)
	}
}

func TestIndexSessions_ReindexIsIdempotent(t *testing.T) {
	db := openTempDB(t)
	coscaDir := t.TempDir()
	writeSession(t, coscaDir, "voice.jsonl", voiceJSONL)

	if _, err := IndexSessions(coscaDir, db); err != nil {
		t.Fatalf("first index: %v", err)
	}
	n, err := IndexSessions(coscaDir, db)
	if err != nil {
		t.Fatalf("second index: %v", err)
	}
	if n != 3 {
		t.Fatalf("re-index returned %d, want 3", n)
	}
	if got := countMessages(t, db); got != 3 {
		t.Fatalf("after re-index session_messages rows = %d, want 3 (no dupes)", got)
	}
}

func TestIndexSessions_MultipleFiles(t *testing.T) {
	db := openTempDB(t)
	coscaDir := t.TempDir()
	writeSession(t, coscaDir, "voice.jsonl", voiceJSONL)
	writeSession(t, coscaDir, "chat.jsonl",
		"{\"type\":\"meta\",\"id\":\"chat\"}\n"+
			"{\"type\":\"message\",\"role\":\"user\",\"content\":\"cérebro em foco\",\"timestamp\":\"2026-08-02T09:00:00Z\"}\n")

	n, err := IndexSessions(coscaDir, db)
	if err != nil {
		t.Fatalf("IndexSessions: %v", err)
	}
	if n != 4 {
		t.Fatalf("indexed %d, want 4 (3 voice + 1 chat)", n)
	}
	if got := countMessages(t, db); got != 4 {
		t.Fatalf("session_messages rows = %d, want 4", got)
	}
}

func TestIndexSessions_MissingSessionsDir(t *testing.T) {
	db := openTempDB(t)
	coscaDir := t.TempDir() // no sessions/ subdir
	n, err := IndexSessions(coscaDir, db)
	if err != nil {
		t.Fatalf("IndexSessions on empty dir: %v", err)
	}
	if n != 0 {
		t.Fatalf("indexed %d, want 0", n)
	}
}

// =============================================================================
// Search
// =============================================================================

func TestSearch_MatchesContent(t *testing.T) {
	db := openTempDB(t)
	coscaDir := t.TempDir()
	writeSession(t, coscaDir, "voice.jsonl", voiceJSONL)
	if _, err := IndexSessions(coscaDir, db); err != nil {
		t.Fatalf("index: %v", err)
	}

	hits, err := Search(context.Background(), db, "CKL", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	h := hits[0]
	if h.SessionID != "voice" || h.Role != "assistant" {
		t.Errorf("unexpected hit: %+v", h)
	}
	if !strings.Contains(h.Content, "CKL") {
		t.Errorf("content does not contain query: %q", h.Content)
	}
	if h.Line != 3 {
		t.Errorf("line = %d, want 3 (assistant message is jsonl line 3)", h.Line)
	}
}

func TestSearch_RespectsLimit(t *testing.T) {
	db := openTempDB(t)
	coscaDir := t.TempDir()
	// 5 messages all containing "tudo".
	var sb strings.Builder
	sb.WriteString("{\"type\":\"meta\",\"id\":\"voice\"}\n")
	for i := 0; i < 5; i++ {
		sb.WriteString("{\"type\":\"message\",\"role\":\"user\",\"content\":\"tudo bem de novo\",\"timestamp\":\"2026-08-02T08:13:44Z\"}\n")
	}
	writeSession(t, coscaDir, "voice.jsonl", sb.String())
	if _, err := IndexSessions(coscaDir, db); err != nil {
		t.Fatalf("index: %v", err)
	}

	hits, err := Search(context.Background(), db, "tudo", 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2 (limit)", len(hits))
	}
}

func TestSearch_MalformedQueryDoesNotError(t *testing.T) {
	db := openTempDB(t)
	coscaDir := t.TempDir()
	writeSession(t, coscaDir, "voice.jsonl", voiceJSONL)
	if _, err := IndexSessions(coscaDir, db); err != nil {
		t.Fatalf("index: %v", err)
	}

	for _, q := range []string{`"`, `:`, `"CKL"`, `foo:bar`, `(`, `*`, `a OR b`, `-`} {
		hits, err := Search(context.Background(), db, q, 10)
		if err != nil {
			t.Fatalf("Search(%q) errored: %v", q, err)
		}
		// A pure-grammar query may return nothing, but must never panic/error.
		_ = hits
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	db := openTempDB(t)
	hits, err := Search(context.Background(), db, "   ", 10)
	if err != nil {
		t.Fatalf("Search on empty query: %v", err)
	}
	if hits != nil {
		t.Fatalf("expected nil hits for empty query, got %d", len(hits))
	}
}

func TestSearch_NoIndexYet(t *testing.T) {
	// Search on a DB whose schema exists but was never indexed → zero hits,
	// no error.
	db := openTempDB(t)
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	hits, err := Search(context.Background(), db, "qualquer coisa", 10)
	if err != nil {
		t.Fatalf("Search on empty index: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected 0 hits, got %d", len(hits))
	}
}

// =============================================================================
// sanitizeFTS5Query
// =============================================================================

func TestSanitizeFTS5Query(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"hello world", `"hello" "world"`},
		{"  hello   world  ", `"hello" "world"`},
		{`"tatuagem"`, `"tatuagem"`},
		// Dois-pontos são STRIPPED (não dividem a palavra) — nunca quebram a
		// sintaxe FTS5, e o termo sobrevivente é casado literalmente.
		{`foo:bar baz:`, `"foobar" "baz"`},
		{`(a OR b)`, `"a" "OR" "b"`},
		{`CKL`, `"CKL"`},
		{`a* b^ c`, `"a" "b" "c"`},
		{`"`, ``},
		{":", ``},
		{"", ``},
		{"   ", ``},
		{"cosca-voice", `"cosca-voice"`},
		{`olá, mundo!`, `"olá" "mundo"`},
	}
	for _, c := range cases {
		got := sanitizeFTS5Query(c.in)
		if got != c.want {
			t.Errorf("sanitizeFTS5Query(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// =============================================================================
// SessionLineage
// =============================================================================

const lineageChainJSONL = `{"type":"meta","id":"root","parent_session_id":""}
{"type":"message","role":"user","content":"raiz","timestamp":"2026-08-02T08:00:00Z"}
`

const lineageChildJSONL = `{"type":"meta","id":"child","parent_session_id":"root"}
{"type":"message","role":"user","content":"filho","timestamp":"2026-08-02T09:00:00Z"}
`

const lineageGrandchildJSONL = `{"type":"meta","id":"grandchild","parent_session_id":"child"}
{"type":"message","role":"user","content":"neto","timestamp":"2026-08-02T10:00:00Z"}
`

const lineageOrphanJSONL = `{"type":"meta","id":"orphan"}
{"type":"message","role":"user","content":"sem pais","timestamp":"2026-08-02T11:00:00Z"}
`

func TestSessionLineage_BasePlusDescendants(t *testing.T) {
	db := openTempDB(t)
	coscaDir := t.TempDir()
	writeSession(t, coscaDir, "root.jsonl", lineageChainJSONL)
	writeSession(t, coscaDir, "child.jsonl", lineageChildJSONL)
	writeSession(t, coscaDir, "grandchild.jsonl", lineageGrandchildJSONL)
	writeSession(t, coscaDir, "orphan.jsonl", lineageOrphanJSONL)
	if _, err := IndexSessions(coscaDir, db); err != nil {
		t.Fatalf("index: %v", err)
	}

	got, err := SessionLineage(db, "root")
	if err != nil {
		t.Fatalf("SessionLineage: %v", err)
	}
	want := []string{"root", "child", "grandchild"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lineage = %v, want %v", got, want)
	}
}

func TestSessionLineage_NoParentGraceful(t *testing.T) {
	db := openTempDB(t)
	coscaDir := t.TempDir()
	writeSession(t, coscaDir, "orphan.jsonl", lineageOrphanJSONL)
	if _, err := IndexSessions(coscaDir, db); err != nil {
		t.Fatalf("index: %v", err)
	}

	got, err := SessionLineage(db, "orphan")
	if err != nil {
		t.Fatalf("SessionLineage: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"orphan"}) {
		t.Errorf("lineage = %v, want [orphan]", got)
	}
}

func TestSessionLineage_UnknownSession(t *testing.T) {
	db := openTempDB(t)
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	got, err := SessionLineage(db, "never-indexed")
	if err != nil {
		t.Fatalf("SessionLineage: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"never-indexed"}) {
		t.Errorf("lineage = %v, want [never-indexed]", got)
	}
}
