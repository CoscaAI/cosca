package kernel

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestIdentity verifies the kernel identity is complete.
func TestIdentity(t *testing.T) {
	id := Identity()
	if id.Name != "Cosca Kernel" {
		t.Errorf("expected Cosca Kernel, got %q", id.Name)
	}
	if id.Role == "" {
		t.Error("role must not be empty")
	}
	if id.Project != "Cosca" || id.ProjectVer == "" {
		t.Errorf("project identity incomplete: %q %q", id.Project, id.ProjectVer)
	}
	if id.Model == "" {
		t.Error("model must not be empty")
	}
}

// TestLaws verifies the 6 immutable laws are present.
func TestLaws(t *testing.T) {
	if len(Laws) != 6 {
		t.Fatalf("expected 6 laws, got %d", len(Laws))
	}
	for _, l := range Laws {
		if l.Number == 0 || l.Title == "" || l.Rule == "" {
			t.Errorf("law incomplete: %+v", l)
		}
	}
}

// TestConstitution verifies the 9 constitutional principles are present.
func TestConstitution(t *testing.T) {
	if len(Constitution) != 9 {
		t.Fatalf("expected 9 principles, got %d", len(Constitution))
	}
	for _, p := range Constitution {
		if p.Number == 0 || p.Title == "" || p.Rule == "" || p.Guardian == "" {
			t.Errorf("principle incomplete: %+v", p)
		}
	}
	// P8 — Integridade do embed — must exist (Amendment v1.1.0).
	if Constitution[7].Number != 8 || Constitution[7].Title != "Integridade do embed" {
		t.Errorf("P8 missing or wrong: %+v", Constitution[7])
	}
	// P9 — Integridade do LIVE — espelha a P8 para a zona LIVE (Contrato de Autoridade).
	if Constitution[8].Number != 9 || Constitution[8].Title != "Integridade do LIVE" {
		t.Errorf("P9 missing or wrong: %+v", Constitution[8])
	}
	if !strings.Contains(Constitution[8].Rule, ".opencode/cosca") {
		t.Errorf("P9 rule must protect .opencode/cosca (LIVE): %+v", Constitution[8])
	}
}

// TestLiveZoneProtectedByConstitution verifica que a governança espelha o
// contrato de autoridade e protege a zona LIVE (.opencode/cosca) de remoção
// destrutiva sem confirmação explícita do Don.
func TestLiveZoneProtectedByConstitution(t *testing.T) {
	var livePrinciple *ConstitutionPrinciple
	for i := range Constitution {
		if Constitution[i].Number == 9 {
			livePrinciple = &Constitution[i]
			break
		}
	}
	if livePrinciple == nil {
		t.Fatal("principle protecting the LIVE zone (P9) not found")
	}
	if !strings.Contains(strings.ToLower(livePrinciple.Rule), ".opencode/cosca") {
		t.Errorf("P9 should reference .opencode/cosca, got: %q", livePrinciple.Rule)
	}
	if !strings.Contains(strings.ToLower(livePrinciple.Rule), "don") {
		t.Errorf("P9 should require explicit Don confirmation, got: %q", livePrinciple.Rule)
	}
}

// TestSelfTest verifies the self-assessment runs without error.
func TestSelfTest(t *testing.T) {
	checks := SelfTest()
	if len(checks) < 4 {
		t.Fatalf("expected at least 4 checks, got %d", len(checks))
	}
	for _, c := range checks {
		if !c.OK {
			t.Errorf("self check failed: %s (%s)", c.Name, c.Detail)
		}
	}
}

// setupTestMemory creates a temporary SQLite DB with a minimal schema
// for memory tests. It does NOT depend on the real knowledge.db.
func setupTestMemory(t *testing.T) *Memory {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := openTestDB(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	// Minimal schema mirroring the real knowledge base.
	schema := []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY, path TEXT, hash TEXT, title TEXT,
			doc_type TEXT, metadata_json TEXT, frontmatter_json TEXT,
			size INTEGER, token_count INTEGER, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY, document_id TEXT, content TEXT, heading TEXT,
			section_type TEXT, position INTEGER, hash TEXT, token_count INTEGER,
			metadata_json TEXT, embedding BLOB)`,
		`CREATE TABLE IF NOT EXISTS knowledge_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT, category TEXT NOT NULL,
			sub_category TEXT NOT NULL DEFAULT '', title TEXT NOT NULL,
			content TEXT NOT NULL, tags TEXT NOT NULL DEFAULT '[]',
			confidence REAL NOT NULL DEFAULT 0.5, source TEXT NOT NULL,
			updated_at TEXT NOT NULL, content_hash TEXT NOT NULL)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(id, content, heading)`,
	}
	for _, s := range schema {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}

	// Seed: one learning document with chunks.
	if _, err := db.Exec(`INSERT INTO documents (id, path, title, doc_type) VALUES
		('doc-1', '/home/cosca/.cosca/memory/agent/cosca-kernel/learnings.md', 'L41 — Restauração de Memória', 'markdown')`); err != nil {
		t.Fatalf("seed doc: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO chunks (id, document_id, content, heading) VALUES
		('chunk-1', 'doc-1', 'Backup recovery: 54 agentes restaurados do cosca-test-bk--noop', 'L41'),
		('chunk-2', 'doc-1', 'Pattern 004: restauração cirúrgica com diff direcional', 'Pattern')`); err != nil {
		t.Fatalf("seed chunk: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_entries (category, title, content, tags, confidence, source, updated_at, content_hash) VALUES
		('heuristics', 'H-001', 'Segurança sem teste é fachada', '["security","testing"]', 0.95, 'test', '2026-07-31', 'h1')`); err != nil {
		t.Fatalf("seed knowledge: %v", err)
	}

	return &Memory{db: db}
}

// TestLearnings verifies learning retrieval from the knowledge base.
func TestLearnings(t *testing.T) {
	m := setupTestMemory(t)
	defer m.Close()

	items, err := m.Learnings(context.Background(), 10)
	if err != nil {
		t.Fatalf("learnings: %v", err)
	}
	if len(items) < 1 {
		t.Fatal("expected at least 1 learning")
	}
	if items[0].Type != MemoryLearning {
		t.Errorf("expected learning type, got %s", items[0].Type)
	}
}

// TestLearningsCount verifies the count query.
func TestLearningsCount(t *testing.T) {
	m := setupTestMemory(t)
	defer m.Close()

	n, err := m.LearningsCount(context.Background())
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n < 1 {
		t.Errorf("expected >= 1 learning, got %d", n)
	}
}

// TestLearningsCreatedAt verifies created_at is read from the documents table
// and parsed in the full datetime layout ("2006-01-02 15:04:05").
func TestLearningsCreatedAt(t *testing.T) {
	m := setupTestMemory(t)
	defer m.Close()

	if _, err := m.db.Exec(`UPDATE documents SET created_at = '2026-07-31 10:30:00' WHERE id = 'doc-1'`); err != nil {
		t.Fatalf("set created_at: %v", err)
	}

	items, err := m.Learnings(context.Background(), 10)
	if err != nil {
		t.Fatalf("learnings: %v", err)
	}
	for _, it := range items {
		if it.ID == "doc-1" {
			want := time.Date(2026, 7, 31, 10, 30, 0, 0, time.UTC)
			if !it.CreatedAt.Equal(want) {
				t.Errorf("CreatedAt = %v, want %v", it.CreatedAt, want)
			}
			return
		}
	}
	t.Fatal("doc-1 not returned by Learnings")
}

// TestLearningsCreatedAtDateOnly verifies a date-only created_at
// ("2006-01-02") is accepted without error.
func TestLearningsCreatedAtDateOnly(t *testing.T) {
	m := setupTestMemory(t)
	defer m.Close()

	if _, err := m.db.Exec(`UPDATE documents SET created_at = '2026-07-31' WHERE id = 'doc-1'`); err != nil {
		t.Fatalf("set created_at: %v", err)
	}

	items, err := m.Learnings(context.Background(), 10)
	if err != nil {
		t.Fatalf("learnings: %v", err)
	}
	for _, it := range items {
		if it.ID == "doc-1" {
			want := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
			if !it.CreatedAt.Equal(want) {
				t.Errorf("CreatedAt = %v, want %v", it.CreatedAt, want)
			}
			return
		}
	}
	t.Fatal("doc-1 not returned by Learnings")
}

// TestFailures verifies failure retrieval.
func TestFailures(t *testing.T) {
	m := setupTestMemory(t)
	defer m.Close()

	items, err := m.Failures(context.Background(), 10)
	if err != nil {
		t.Fatalf("failures: %v", err)
	}
	// Seed has no failures; expect empty slice, no error.
	if items == nil {
		t.Error("expected non-nil (possibly empty) slice")
	}
}

// TestKnowledgeEntries verifies compiled knowledge retrieval.
func TestKnowledgeEntries(t *testing.T) {
	m := setupTestMemory(t)
	defer m.Close()

	items, err := m.KnowledgeEntries(context.Background(), "heuristics", 10)
	if err != nil {
		t.Fatalf("knowledge entries: %v", err)
	}
	if len(items) < 1 {
		t.Fatal("expected at least 1 knowledge entry")
	}
	if items[0].Title != "H-001" {
		t.Errorf("expected H-001, got %q", items[0].Title)
	}
}

// TestSearchFallback verifies LIKE-based search works when FTS5 is unavailable.
func TestSearchFallback(t *testing.T) {
	m := setupTestMemory(t)
	defer m.Close()

	items, err := m.Search(context.Background(), "restaurados", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) < 1 {
		t.Fatal("expected at least 1 result")
	}
}

// TestStats verifies stats are computed.
func TestStats(t *testing.T) {
	m := setupTestMemory(t)
	defer m.Close()

	stats, err := m.Stats(context.Background())
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats["documents"] < 1 {
		t.Errorf("expected >= 1 document, got %d", stats["documents"])
	}
	if stats["knowledge_entries"] < 1 {
		t.Errorf("expected >= 1 knowledge entry, got %d", stats["knowledge_entries"])
	}
}

// TestAgentFromPath verifies agent name extraction.
func TestAgentFromPath(t *testing.T) {
	cases := map[string]string{
		"/home/cosca/.cosca/memory/agent/cosca-kernel/learnings.md":    "cosca-kernel",
		"/home/cosca/.cosca/memory/agent/cosca-security/failures.md":   "cosca-security",
		"/home/cosca/.cosca/framework/knowledge/heuristics/H-001.yaml": "cosca-family",
	}
	for path, want := range cases {
		if got := agentFromPath(path); got != want {
			t.Errorf("agentFromPath(%q) = %q, want %q", path, got, want)
		}
	}
}

// TestParseTimestamp verifies flexible timestamp parsing (datetime + date).
func TestParseTimestamp(t *testing.T) {
	cases := []struct {
		in   string
		want string // expected RFC3339 rendering
	}{
		{"2026-07-31 10:30:00", "2026-07-31T10:30:00Z"},
		{"2026-07-31", "2026-07-31T00:00:00Z"},
		{"", "0001-01-01T00:00:00Z"},           // zero time for empty
		{"not a date", "0001-01-01T00:00:00Z"}, // zero time for garbage
	}
	for _, c := range cases {
		got := parseTimestamp(c.in)
		if got.Format(time.RFC3339) != c.want {
			t.Errorf("parseTimestamp(%q) = %v, want %s", c.in, got.Format(time.RFC3339), c.want)
		}
	}
}

// openTestDB opens a test database file.
func openTestDB(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}
