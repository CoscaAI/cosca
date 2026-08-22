package cli

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// mustOpenSQLite abre um sqlite de teste (driver modernc, sem cgo).
func mustOpenSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return db
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

// fakeKnowledgeDB cria um knowledge.db fake com entities + relationships.
func fakeKnowledgeDB(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".cosca"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	db := mustOpenSQLite(t, filepath.Join(dir, ".cosca", "knowledge.db"))
	defer db.Close()
	mustExec(t, db, `CREATE TABLE entities (id TEXT PRIMARY KEY, entity_type TEXT NOT NULL, name TEXT NOT NULL, path TEXT NOT NULL)`)
	mustExec(t, db, `CREATE TABLE relationships (id TEXT PRIMARY KEY, source_id TEXT NOT NULL, target_id TEXT NOT NULL, rel_type TEXT NOT NULL, weight REAL NOT NULL DEFAULT 1.0)`)
	mustExec(t, db, `INSERT INTO entities VALUES ('n1','doc','doc-a','a.md'),('n2','skill','skill-b','b.md')`)
	mustExec(t, db, `INSERT INTO relationships VALUES ('r1','n1','n2','contains',1.0)`)
}

func TestNewGraphAdapter_LoadsFromKnowledgeDB(t *testing.T) {
	dir := t.TempDir()
	fakeKnowledgeDB(t, dir)

	a := newGraphAdapter(dir)
	if a.inner == nil {
		t.Fatal("inner nil")
	}
	if got := a.inner.GetNodeCount(); got != 2 {
		t.Fatalf("nodes = %d, want 2", got)
	}
	if got := a.inner.GetEdgeCount(); got != 1 {
		t.Fatalf("edges = %d, want 1", got)
	}
}

func TestNewGraphAdapter_EmptyDir(t *testing.T) {
	a := newGraphAdapter(t.TempDir())
	if a.inner == nil {
		t.Fatal("inner nil")
	}
	if got := a.inner.GetNodeCount(); got != 0 {
		t.Fatalf("nodes = %d, want 0 (sem knowledge.db)", got)
	}
}

func TestGraphAdapter_ExportFormats(t *testing.T) {
	dir := t.TempDir()
	fakeKnowledgeDB(t, dir)
	a := newGraphAdapter(dir)

	out := filepath.Join(t.TempDir(), "g.json")
	if err := a.Export(out, "json"); err != nil {
		t.Fatalf("export json: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("json file: %v", err)
	}
	if err := a.Export(filepath.Join(t.TempDir(), "g.dot"), "dot"); err != nil {
		t.Fatalf("export dot: %v", err)
	}
	if err := a.Export(filepath.Join(t.TempDir(), "g.graphml"), "graphml"); err != nil {
		t.Fatalf("export graphml: %v", err)
	}
	if err := a.Export(filepath.Join(t.TempDir(), "g.bad"), "bad"); err == nil {
		t.Fatal("formato inválido deveria falhar")
	}
}

func TestGraphAdapter_QueryByName(t *testing.T) {
	dir := t.TempDir()
	fakeKnowledgeDB(t, dir)
	a := newGraphAdapter(dir)

	// busca por nome (não por UUID)
	results, err := a.Query("doc-a", 1)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].Relationship != "contains" {
		t.Fatalf("rel = %q, want contains", results[0].Relationship)
	}

	// entidade inexistente → erro explícito
	if _, err := a.Query("nao-existe", 1); err == nil {
		t.Fatal("esperava erro para entidade inexistente")
	}
}

func TestGraphAdapter_Stats(t *testing.T) {
	dir := t.TempDir()
	fakeKnowledgeDB(t, dir)
	a := newGraphAdapter(dir)

	stats, err := a.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.TotalNodes != 2 || stats.TotalEdges != 1 {
		t.Fatalf("stats = %d nós/%d arestas, want 2/1", stats.TotalNodes, stats.TotalEdges)
	}
}
