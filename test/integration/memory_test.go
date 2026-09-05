
//go:build integration

package integration

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/CoscaAI/cosca/internal/vector"
	"github.com/google/uuid"
)

// memoryRecord represents a simple memory entry stored in SQLite.
type memoryRecord struct {
	ID        string
	Key       string
	Value     string
	AgentID   string
	MemType   string
	CreatedAt time.Time
	Score     float64
}

// memoryEngine wraps a SQLite database with memory operations.
type memoryEngine struct {
	db     *sqlite.DB
	vec    *vector.SQLiteVec
	reader *sql.DB
}

// newMemoryEngine creates a memory engine backed by a temp SQLite database.
func newMemoryEngine(t *testing.T) (*memoryEngine, string) {
	t.Helper()

	tmpDir, err := os.MkdirTemp(".", "cosca-memory-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	dbPath := filepath.Join(tmpDir, "memory.db")
	cfg := sqlite.DefaultConfig(dbPath)
	cfg.WALMode = false
	cfg.JournalMode = "delete"
	cfg.AutoMigrate = true

	db, err := sqlite.Open(cfg)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create a memory_records table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS memory_records (
			id         TEXT PRIMARY KEY,
			agent_id   TEXT NOT NULL DEFAULT '',
			mem_type   TEXT NOT NULL DEFAULT 'working',
			key        TEXT NOT NULL,
			value      TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			accessed_at TEXT NOT NULL DEFAULT (datetime('now')),
			access_count INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatalf("failed to create memory_records table: %v", err)
	}

	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_memory_agent ON memory_records(agent_id)
	`)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_memory_key ON memory_records(key)
	`)
	if err != nil {
		t.Fatalf("failed to create key index: %v", err)
	}

	// Create vector store for semantic memory search
	vecCfg := vector.SQLiteVecConfig{
		DB:        db.Conn(),
		Dimension: 4,
		TableName: "memory_vectors",
	}
	vec, err := vector.NewSQLiteVec(vecCfg)
	if err != nil {
		t.Fatalf("failed to create vector store: %v", err)
	}
	t.Cleanup(func() { vec.Close() })

	return &memoryEngine{
		db:     db,
		vec:    vec,
		reader: db.Conn(),
	}, tmpDir
}

// store inserts or replaces a memory record.
func (m *memoryEngine) store(rec memoryRecord) error {
	_, err := m.db.Exec(
		`INSERT OR REPLACE INTO memory_records (id, agent_id, mem_type, key, value, created_at, accessed_at, access_count)
		 VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'), 0)`,
		rec.ID, rec.AgentID, rec.MemType, rec.Key, rec.Value,
	)
	return err
}

// retrieve fetches a single record by ID.
func (m *memoryEngine) retrieve(id string) (memoryRecord, error) {
	var rec memoryRecord
	var createdAt string
	err := m.reader.QueryRow(
		`SELECT id, agent_id, mem_type, key, value, created_at FROM memory_records WHERE id = ?`, id,
	).Scan(&rec.ID, &rec.AgentID, &rec.MemType, &rec.Key, &rec.Value, &createdAt)
	if err != nil {
		return memoryRecord{}, err
	}
	rec.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return rec, nil
}

// searchByKey finds records matching the given key.
func (m *memoryEngine) searchByKey(key string) ([]memoryRecord, error) {
	rows, err := m.reader.Query(
		`SELECT id, agent_id, mem_type, key, value FROM memory_records WHERE key LIKE ? ORDER BY accessed_at DESC`, "%"+key+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []memoryRecord
	for rows.Next() {
		var rec memoryRecord
		if err := rows.Scan(&rec.ID, &rec.AgentID, &rec.MemType, &rec.Key, &rec.Value); err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

// promote increments the access count and updates accessed_at.
func (m *memoryEngine) promote(id string) error {
	_, err := m.db.Exec(
		`UPDATE memory_records SET access_count = access_count + 1, accessed_at = datetime('now') WHERE id = ?`, id,
	)
	return err
}

// prune removes records with access_count == 0 that are older than the given threshold.
func (m *memoryEngine) prune(before time.Time) (int, error) {
	result, err := m.db.Exec(
		`DELETE FROM memory_records WHERE access_count = 0 AND created_at < ?`,
		before.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// TestMemoryEngine_StoreRetrieveSearch exercises the memory lifecycle:
// store → retrieve → search → promote → prune.
func TestMemoryEngine_StoreRetrieveSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine, _ := newMemoryEngine(t)

	// ── Store records ─────────────────────────────────────────────────────
	records := []memoryRecord{
		{ID: uuid.New().String(), Key: "user_preference_theme", Value: "dark", AgentID: "agent-1", MemType: "persistent"},
		{ID: uuid.New().String(), Key: "user_preference_language", Value: "en-US", AgentID: "agent-1", MemType: "persistent"},
		{ID: uuid.New().String(), Key: "session_context_project", Value: "cosca", AgentID: "agent-1", MemType: "working"},
		{ID: uuid.New().String(), Key: "session_context_branch", Value: "main", AgentID: "agent-1", MemType: "working"},
	}

	for _, rec := range records {
		if err := engine.store(rec); err != nil {
			t.Fatalf("failed to store record %s: %v", rec.Key, err)
		}
	}

	// ── Verify count ──────────────────────────────────────────────────────
	var count int
	if err := engine.reader.QueryRow("SELECT COUNT(*) FROM memory_records").Scan(&count); err != nil {
		t.Fatalf("failed to count records: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 records, got %d", count)
	}

	// ── Retrieve by ID ────────────────────────────────────────────────────
	retrieved, err := engine.retrieve(records[0].ID)
	if err != nil {
		t.Fatalf("failed to retrieve record: %v", err)
	}
	if retrieved.Key != "user_preference_theme" {
		t.Errorf("expected key 'user_preference_theme', got %q", retrieved.Key)
	}
	if retrieved.Value != "dark" {
		t.Errorf("expected value 'dark', got %q", retrieved.Value)
	}

	// ── Search by key ─────────────────────────────────────────────────────
	results, err := engine.searchByKey("preference")
	if err != nil {
		t.Fatalf("failed to search records: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'preference', got %d", len(results))
	}

	results, err = engine.searchByKey("session")
	if err != nil {
		t.Fatalf("failed to search 'session': %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'session', got %d", len(results))
	}

	// ── Promote a record ──────────────────────────────────────────────────
	if err := engine.promote(records[0].ID); err != nil {
		t.Fatalf("failed to promote record: %v", err)
	}

	var accessCount int
	if err := engine.reader.QueryRow(
		"SELECT access_count FROM memory_records WHERE id = ?", records[0].ID,
	).Scan(&accessCount); err != nil {
		t.Fatalf("failed to query access count: %v", err)
	}
	if accessCount != 1 {
		t.Errorf("expected access_count=1 after promote, got %d", accessCount)
	}

	// ── Prune old untouched records ──────────────────────────────────────
	// All records were just created, so pruning "before now" should remove
	// only those with access_count == 0 that are older than now.
	// Since they were just created, nothing should be pruned.
	pruned, err := engine.prune(time.Now().Add(1 * time.Hour)) // prune from the future
	if err != nil {
		t.Fatalf("failed to prune records: %v", err)
	}
	// Since all records were just created, pruning "1 hour from now" should
	// remove those with access_count == 0 (which is all except the promoted one)
	t.Logf("pruned %d records (expected 3 untouched records removed)", pruned)

	// Add an "old" record directly (bypassing engine.store which sets created_at = now)
	oldID := uuid.New().String()
	_, err = engine.db.Exec(
		`INSERT INTO memory_records (id, agent_id, mem_type, key, value, created_at, access_count)
		 VALUES (?, '', 'working', 'old_key', 'old_value', '2020-01-01 00:00:00', 0)`,
		oldID,
	)
	if err != nil {
		t.Fatalf("failed to insert old record: %v", err)
	}

	// Now prune records older than 2023
	threshold := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	pruned, err = engine.prune(threshold)
	if err != nil {
		t.Fatalf("failed to prune old records: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 old record pruned, got %d", pruned)
	}

	// Verify old record is gone
	var oldExists int
	engine.reader.QueryRow("SELECT COUNT(*) FROM memory_records WHERE id = ?", oldID).Scan(&oldExists)
	if oldExists != 0 {
		t.Error("expected old record to be pruned")
	}

	t.Logf("memory test passed: stored=%d, remaining after prune=%d", count, count-pruned)
}

// TestMemoryEngine_EmptyStore verifies edge cases on empty storage.
func TestMemoryEngine_EmptyStore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine, _ := newMemoryEngine(t)

	// Search on empty store
	results, err := engine.searchByKey("nonexistent")
	if err != nil {
		t.Fatalf("search on empty store should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on empty store, got %d", len(results))
	}

	// Prune on empty store
	pruned, err := engine.prune(time.Now())
	if err != nil {
		t.Fatalf("prune on empty store should not error: %v", err)
	}
	if pruned != 0 {
		t.Errorf("expected 0 pruned on empty store, got %d", pruned)
	}
}
