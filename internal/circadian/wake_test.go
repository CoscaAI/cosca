package circadian

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// createWakeTestDB creates a minimal SQLite knowledge base at dbPath with the
// tables kernel.Memory.Stats expects, seeded with one learning document and
// one knowledge entry. It does not depend on the real .cosca/knowledge.db.
func createWakeTestDB(t *testing.T, dbPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	for _, s := range []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY, path TEXT, hash TEXT, title TEXT,
			doc_type TEXT, metadata_json TEXT, frontmatter_json TEXT,
			size INTEGER, token_count INTEGER, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY, document_id TEXT, content TEXT, heading TEXT,
			section_type TEXT, position INTEGER, hash TEXT, token_count INTEGER,
			metadata_json TEXT, embedding BLOB)`,
		`CREATE TABLE IF NOT EXISTS vectors (
			id TEXT PRIMARY KEY, vector BLOB NOT NULL,
			metadata TEXT NOT NULL DEFAULT '{}',
			document_id TEXT NOT NULL DEFAULT '',
			chunk_id TEXT NOT NULL DEFAULT '',
			entity_id TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')))`,
		`CREATE TABLE IF NOT EXISTS knowledge_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT, category TEXT NOT NULL,
			sub_category TEXT NOT NULL DEFAULT '', title TEXT NOT NULL,
			content TEXT NOT NULL, tags TEXT NOT NULL DEFAULT '[]',
			confidence REAL NOT NULL DEFAULT 0.5, source TEXT NOT NULL,
			updated_at TEXT NOT NULL, content_hash TEXT NOT NULL)`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}

	for _, s := range []string{
		`INSERT INTO documents (id, path, title, doc_type) VALUES
			('doc-1', '/p/.cosca/memory/agent/cosca-kernel/learnings.md',
			 'L01 — Wake Test', 'markdown')`,
		`INSERT INTO chunks (id, document_id, content, heading) VALUES
			('chunk-1', 'doc-1', 'wake ritual test content', 'L01')`,
		`INSERT INTO knowledge_entries (category, title, content, tags, confidence, source, updated_at, content_hash) VALUES
			('heuristics', 'H-001', 'wake ritual test entry', '["test"]', 0.9, 'test', '2026-07-31', 'h1')`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("seed data: %v", err)
		}
	}
}

// TestWake_WithTempDir verifies the full ritual against a temp knowledge
// base: memory is loaded, the knowledge base is valid, the constitution is
// loaded and the engine returns to the awake state.
func TestWake_WithTempDir(t *testing.T) {
	coscaDir := t.TempDir()
	createWakeTestDB(t, filepath.Join(coscaDir, "knowledge.db"))

	e := New()
	reach(t, e, StateSleeping) // enter the ORC window first

	result, err := e.Wake(context.Background(), coscaDir)
	if err != nil {
		t.Fatalf("Wake: %v", err)
	}
	if result == nil {
		t.Fatal("Wake returned a nil result")
	}
	if !result.KnowledgeValid {
		t.Error("KnowledgeValid = false, want true")
	}
	if !result.ConstitutionLoaded {
		t.Error("ConstitutionLoaded = false, want true")
	}
	if !result.ClockSynced {
		t.Error("ClockSynced = false, want true")
	}
	if len(result.MemoryStats) == 0 {
		t.Error("MemoryStats empty, want populated")
	}
	if result.MemoryStats["documents"] < 1 {
		t.Errorf("documents = %d, want >= 1", result.MemoryStats["documents"])
	}
	if got := e.State(); got != StateAwake {
		t.Errorf("State() = %s after wake, want %s", got, StateAwake)
	}
	if result.Summary == "" {
		t.Error("Summary is empty")
	}
	if result.At.IsZero() {
		t.Error("At is the zero time")
	}

	// The ritual must record all six steps in order, all OK.
	wantSteps := []string{
		"load_memory", "validate_knowledge", "read_constitution",
		"sync_clock", "show_summary", "accept_commands",
	}
	if len(result.Steps) != len(wantSteps) {
		t.Fatalf("expected %d wake steps, got %d", len(wantSteps), len(result.Steps))
	}
	for i, name := range wantSteps {
		if result.Steps[i].Name != name {
			t.Errorf("step %d = %q, want %q", i, result.Steps[i].Name, name)
		}
		if result.Steps[i].Status != "ok" {
			t.Errorf("step %s status = %q, want ok", name, result.Steps[i].Status)
		}
		if result.Steps[i].Detail == "" {
			t.Errorf("step %s has an empty detail", name)
		}
	}
}

// TestWake_NoDir verifies the ritual degrades gracefully when the cosca
// directory does not exist: memory and knowledge steps fail, but the ritual
// continues, completes all six steps and never returns a fatal error.
func TestWake_NoDir(t *testing.T) {
	coscaDir := filepath.Join(t.TempDir(), "does-not-exist")

	e := New()
	reach(t, e, StateSleeping)

	result, err := e.Wake(context.Background(), coscaDir)
	if err != nil {
		t.Fatalf("Wake should degrade gracefully for a missing dir, got: %v", err)
	}
	if result == nil {
		t.Fatal("Wake returned a nil result")
	}
	if len(result.Steps) != 6 {
		t.Fatalf("expected 6 wake steps even with failures, got %d", len(result.Steps))
	}

	// Failed steps are recorded as errors but the ritual continues.
	for _, step := range result.Steps {
		if step.Name == "load_memory" && step.Status != "error" {
			t.Errorf("load_memory status = %q, want error", step.Status)
		}
		if step.Name == "validate_knowledge" && step.Status != "error" {
			t.Errorf("validate_knowledge status = %q, want error", step.Status)
		}
		if step.Name == "accept_commands" && step.Status != "ok" {
			t.Errorf("accept_commands status = %q, want ok (ritual completes)", step.Status)
		}
	}
	if result.KnowledgeValid {
		t.Error("KnowledgeValid = true for a missing knowledge base, want false")
	}
	// The constitution is compiled into the kernel, so it loads regardless.
	if !result.ConstitutionLoaded {
		t.Error("ConstitutionLoaded = false, want true")
	}
	// The engine still transitions to awake — the ritual completes.
	if got := e.State(); got != StateAwake {
		t.Errorf("State() = %s after wake, want %s", got, StateAwake)
	}
}
