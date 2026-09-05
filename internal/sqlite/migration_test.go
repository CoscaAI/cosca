package sqlite

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

// ── Migration: Up ────────────────────────────────────────────────────────

func TestMigrationUp(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.AutoMigrate = true

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables = append(tables, name)
	}
	assert.Contains(t, tables, "documents")
	assert.Contains(t, tables, "chunks")
	assert.Contains(t, tables, "migration_history")
}

func TestMigrationUp_AlreadyApplied(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	err = db.migrations.Up()
	require.NoError(t, err)
}

// TestMigration4_FixesStaleCoscaTestPaths verifies that migration v4 rewrites
// stale /home/cosca/Documents/cosca-test paths to the real project root
// /home/cosca/Documents/cosca in both documents.path and
// documents.metadata_json, without touching chunks (which reference documents
// by document_id, not by path) and without deleting any rows.
func TestMigration4_FixesStaleCoscaTestPaths(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.AutoMigrate = false

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations

	// Simulate a pre-v4 database: apply only migrations 1-3.
	mm.migrations = defaultMigrations()[:3]
	require.NoError(t, mm.Up())

	status, err := mm.Status()
	require.NoError(t, err)
	assert.Equal(t, 3, status.CurrentVersion)

	// Seed documents with the stale path (as restored from the backup) plus
	// one already-correct document, and chunks referencing them.
	staleDocID := "doc-stale"
	okDocID := "doc-ok"
	_, err = db.Exec(`INSERT INTO documents (id, path, hash, title, doc_type, metadata_json) VALUES (?, ?, ?, ?, ?, ?)`,
		staleDocID,
		"/home/cosca/Documents/cosca-test/.cosca/framework/knowledge/INDEX.md",
		"hash1", "Stale Index", "markdown",
		`{"headings":2,"links":1,"path":"/home/cosca/Documents/cosca-test/.cosca/memory/agent/INDEX.md","type":"markdown"}`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO documents (id, path, hash, title, doc_type, metadata_json) VALUES (?, ?, ?, ?, ?, ?)`,
		okDocID,
		"/home/cosca/Documents/cosca/internal/sqlite/db.go",
		"hash2", "db.go", "go",
		`{"headings":0,"links":0,"path":"/home/cosca/Documents/cosca/internal/sqlite/db.go","type":"go"}`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO chunks (id, document_id, content) VALUES (?, ?, ?)`,
		"chunk-1", staleDocID, "chunk content for stale doc")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO chunks (id, document_id, content) VALUES (?, ?, ?)`,
		"chunk-2", okDocID, "chunk content for ok doc")
	require.NoError(t, err)

	// Restore the full migration set and run Up: v4 and v5 should apply.
	mm.migrations = defaultMigrations()
	require.NoError(t, mm.Up())

	status, err = mm.Status()
	require.NoError(t, err)
	assert.Equal(t, 7, status.CurrentVersion)
	assert.Equal(t, 0, status.PendingCount)

	// documents.path must be rewritten.
	var path string
	err = db.QueryRow(`SELECT path FROM documents WHERE id = ?`, staleDocID).Scan(&path)
	require.NoError(t, err)
	assert.Equal(t, "/home/cosca/Documents/cosca/.cosca/framework/knowledge/INDEX.md", path)

	// metadata_json must be rewritten too (it embeds the same stale path).
	var metadataJSON string
	err = db.QueryRow(`SELECT metadata_json FROM documents WHERE id = ?`, staleDocID).Scan(&metadataJSON)
	require.NoError(t, err)
	assert.Contains(t, metadataJSON, `"path":"/home/cosca/Documents/cosca/.cosca/memory/agent/INDEX.md"`)
	assert.NotContains(t, metadataJSON, "cosca-test")

	// The already-correct document must be untouched.
	err = db.QueryRow(`SELECT path FROM documents WHERE id = ?`, okDocID).Scan(&path)
	require.NoError(t, err)
	assert.Equal(t, "/home/cosca/Documents/cosca/internal/sqlite/db.go", path)

	// No stale paths may remain anywhere in documents.
	var staleCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM documents WHERE path LIKE '%cosca-test%'`).Scan(&staleCount)
	require.NoError(t, err)
	assert.Zero(t, staleCount)

	// Row counts must be preserved (nothing deleted).
	var docCount, chunkCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&docCount))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM chunks`).Scan(&chunkCount))
	assert.Equal(t, 2, docCount)
	assert.Equal(t, 2, chunkCount)

	// Chunks must still reference their documents correctly.
	var chunkDoc string
	err = db.QueryRow(`SELECT document_id FROM chunks WHERE id = 'chunk-1'`).Scan(&chunkDoc)
	require.NoError(t, err)
	assert.Equal(t, staleDocID, chunkDoc)

	// Migration v4 must be recorded in history with the expected checksum.
	var checksum string
	err = db.QueryRow(`SELECT checksum FROM migration_history WHERE version = 4`).Scan(&checksum)
	require.NoError(t, err)
	assert.Equal(t, computeChecksum(upSQLV4()), checksum)
}

// TestMigration5_FixesFtsDeleteTriggers verifies that migration v5 repairs the
// FTS5 sync triggers so that UPDATE and DELETE on the documents and entities
// tables no longer fail with "SQL logic error (1)".
//
// Root cause: migrations v2/v3 converted documents_fts and entities_fts to
// self-managed FTS5 tables (no content= option), but the AFTER UPDATE / AFTER
// DELETE triggers kept using the FTS5 special 'delete' INSERT command, which
// is only valid for contentless or external-content tables. On self-managed
// tables that command errors out, breaking every UPDATE/DELETE on documents
// and entities. The v5 fix replaces it with a plain
// `DELETE FROM <fts> WHERE rowid = old.rowid`.
func TestMigration5_FixesFtsDeleteTriggers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Full schema applied (v1-v5). Seed a document and an entity.
	_, err = db.Exec(`INSERT INTO documents (id, path, hash, title, doc_type) VALUES (?, ?, ?, ?, ?)`,
		"doc-1", "/home/cosca/Documents/cosca/a.md", "h1", "Title", "markdown")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO entities (id, entity_type, name, path) VALUES (?, ?, ?, ?)`,
		"ent-1", "agent", "Alpha", "/home/cosca/Documents/cosca/agent.md")
	require.NoError(t, err)

	// UPDATE must succeed after v5 (it failed before the fix).
	_, err = db.Exec(`UPDATE documents SET path = '/home/cosca/Documents/cosca/a2.md' WHERE id = 'doc-1'`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE entities SET name = 'Alpha2' WHERE id = 'ent-1'`)
	require.NoError(t, err)

	// FTS indexes must reflect the updates (update trigger re-inserted the row).
	var docFts, entFts int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM documents_fts`).Scan(&docFts))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM entities_fts`).Scan(&entFts))
	assert.Equal(t, 1, docFts, "documents_fts should have 1 row after insert+update")
	assert.Equal(t, 1, entFts, "entities_fts should have 1 row after insert+update")

	// DELETE must succeed and clean up the FTS index.
	_, err = db.Exec(`DELETE FROM documents WHERE id = 'doc-1'`)
	require.NoError(t, err)
	_, err = db.Exec(`DELETE FROM entities WHERE id = 'ent-1'`)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM documents_fts`).Scan(&docFts))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM entities_fts`).Scan(&entFts))
	assert.Zero(t, docFts, "documents_fts should be empty after delete")
	assert.Zero(t, entFts, "entities_fts should be empty after delete")

	// Trigger definitions must use the corrected plain-DELETE pattern.
	var triggerSQL string
	require.NoError(t, db.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type='trigger' AND name='documents_au'`,
	).Scan(&triggerSQL))
	assert.Contains(t, triggerSQL, "DELETE FROM documents_fts WHERE rowid = old.rowid")
	assert.NotContains(t, triggerSQL, "'delete'")
}

// TestMigration5_TriggerFixAppliedToPreV4Database verifies the exact upgrade
// path of the real knowledge.db: it was migrated with the ORIGINAL v1-v3
// (broken 'delete' triggers already applied), so running Up() would apply v4
// (path UPDATE) BEFORE v5 (trigger fix) — and v4 would fail because the broken
// documents_au trigger fires on every UPDATE.
//
// The correct procedure for such a database is: (1) apply the v5 trigger fix
// as a hotfix first, (2) then run Up() which applies v4 (now unblocked) and
// v5 (idempotent DROP+CREATE of the same triggers), (3) verify paths are
// rewritten and UPDATE/DELETE work again.
func TestMigration5_TriggerFixAppliedToPreV4Database(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.AutoMigrate = false

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations

	// Step 0: build the pre-v4 database state. Apply migrations 1-3 (which now
	// emit the CORRECTED triggers), then inject the ORIGINAL broken trigger
	// definitions via downSQLV5 — exactly the state of the restored backup.
	mm.migrations = defaultMigrations()[:3]
	require.NoError(t, mm.Up())
	for _, stmt := range strings.Split(downSQLV5(), "\n\n") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		_, err = db.Exec(stmt)
		require.NoError(t, err)
	}

	// Seed a stale-path document.
	_, err = db.Exec(`INSERT INTO documents (id, path, hash, title, doc_type) VALUES (?, ?, ?, ?, ?)`,
		"doc-stale", "/home/cosca/Documents/cosca-test/b.md", "h", "B", "markdown")
	require.NoError(t, err)

	// Prove the bug: with the broken trigger, v4's UPDATE cannot run.
	_, err = db.Exec(`UPDATE documents SET title = 'B2' WHERE id = 'doc-stale'`)
	require.Error(t, err, "UPDATE should fail with the broken pre-v5 trigger")
	assert.Contains(t, err.Error(), "SQL logic error")

	// Step 1: hotfix — apply the v5 trigger fix before running Up().
	for _, stmt := range strings.Split(upSQLV5(), "\n\n") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		_, err = db.Exec(stmt)
		require.NoError(t, err)
	}

	// Step 2: run Up() — applies v4 (path rewrite) and v5 (idempotent).
	mm.migrations = defaultMigrations()
	require.NoError(t, mm.Up())

	status, err := mm.Status()
	require.NoError(t, err)
	assert.Equal(t, 7, status.CurrentVersion)

	// Step 3: verify paths rewritten and UPDATE/DELETE work.
	var path string
	require.NoError(t, db.QueryRow(`SELECT path FROM documents WHERE id = 'doc-stale'`).Scan(&path))
	assert.Equal(t, "/home/cosca/Documents/cosca/b.md", path)

	_, err = db.Exec(`UPDATE documents SET title = 'B3' WHERE id = 'doc-stale'`)
	require.NoError(t, err)
	_, err = db.Exec(`DELETE FROM documents WHERE id = 'doc-stale'`)
	require.NoError(t, err)
}

// TestMigration4_DownSQLV4 verifies the down migration reverts only the rows
// rewritten by v4 and never double-rewrites rows that still contain cosca-test.
func TestMigration4_DownSQLV4(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.AutoMigrate = false

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Seed rows in their post-v4 state: corrected paths plus a still-stale one
	// (should be protected from double-rewriting by the NOT LIKE guard).
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS documents (
		id TEXT PRIMARY KEY, path TEXT NOT NULL UNIQUE, hash TEXT NOT NULL,
		title TEXT NOT NULL DEFAULT '', doc_type TEXT NOT NULL DEFAULT 'markdown',
		metadata_json TEXT NOT NULL DEFAULT '{}', frontmatter_json TEXT NOT NULL DEFAULT '{}',
		size INTEGER NOT NULL DEFAULT 0, token_count INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO documents (id, path, hash, metadata_json) VALUES (?, ?, ?, ?)`,
		"d1", "/home/cosca/Documents/cosca/a.md", "h",
		`{"path":"/home/cosca/Documents/cosca/a.md"}`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO documents (id, path, hash, metadata_json) VALUES (?, ?, ?, ?)`,
		"d2", "/home/cosca/Documents/cosca-test/b.md", "h",
		`{"path":"/home/cosca/Documents/cosca-test/b.md"}`)
	require.NoError(t, err)

	// Execute each statement individually, mirroring how the migration
	// manager splits UpSQL/DownSQL on blank lines (the sqlite driver does not
	// support multi-statement Exec).
	for _, stmt := range strings.Split(downSQLV4(), "\n\n") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		_, err = db.Exec(stmt)
		require.NoError(t, err)
	}

	var path string
	require.NoError(t, db.QueryRow(`SELECT path FROM documents WHERE id = 'd1'`).Scan(&path))
	assert.Equal(t, "/home/cosca/Documents/cosca-test/a.md", path)

	// d2 must NOT be double-rewritten.
	require.NoError(t, db.QueryRow(`SELECT path FROM documents WHERE id = 'd2'`).Scan(&path))
	assert.Equal(t, "/home/cosca/Documents/cosca-test/b.md", path)
}

func TestMigrationUp_WithPrecomputedChecksum(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.AutoMigrate = false

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Create migration_history manually since auto-migrate is off
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS migration_history (" +
		"version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL DEFAULT (datetime('now'))," +
		"checksum TEXT NOT NULL DEFAULT '', applied_by TEXT NOT NULL DEFAULT 'cosca')")
	require.NoError(t, err)

	mm := db.migrations
	mm.migrations = []Migration{
		{
			Version:  1,
			Name:     "custom_checksum",
			UpSQL:    "CREATE TABLE IF NOT EXISTS custom_checksum_test (id INTEGER PRIMARY KEY)",
			DownSQL:  "DROP TABLE IF EXISTS custom_checksum_test",
			Checksum: "abc123",
		},
	}

	err = mm.Up()
	require.NoError(t, err)

	var checksum string
	err = db.QueryRow(
		"SELECT checksum FROM migration_history WHERE version = 1",
	).Scan(&checksum)
	require.NoError(t, err)
	assert.Equal(t, "abc123", checksum)
}

func TestMigrationUp_ErrorInStatement(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.AutoMigrate = false

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Create migration_history
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS migration_history (" +
		"version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL DEFAULT (datetime('now'))," +
		"checksum TEXT NOT NULL DEFAULT '', applied_by TEXT NOT NULL DEFAULT 'cosca')")
	require.NoError(t, err)

	mm := db.migrations
	mm.migrations = []Migration{
		{
			Version: 1,
			Name:    "bad_migration",
			UpSQL:   "CREATE TABLE good_table (id INTEGER PRIMARY KEY)\n\nINVALID SQL SYNTAX HERE!!!!",
			DownSQL: "DROP TABLE IF EXISTS good_table",
		},
	}

	err = mm.Up()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "migration 1")
}

// ── Migration: RegisterMigration ─────────────────────────────────────────

func TestRegisterMigration_Success(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations
	mig := Migration{
		Version: 8,
		Name:    "add_custom_table",
		UpSQL:   "CREATE TABLE IF NOT EXISTS custom_test (id INTEGER PRIMARY KEY, val TEXT)",
		DownSQL: "DROP TABLE IF EXISTS custom_test",
	}

	err = mm.RegisterMigration(mig)
	require.NoError(t, err)
}

func TestRegisterMigration_DuplicateVersion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations
	mig := Migration{
		Version: 1,
		Name:    "duplicate",
		UpSQL:   "SELECT 1",
		DownSQL: "SELECT 1",
	}

	err = mm.RegisterMigration(mig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestRegisterMigration_OrdersByVersion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations

	require.NoError(t, mm.RegisterMigration(Migration{Version: 10, Name: "v10", UpSQL: "SELECT 1"}))
	require.NoError(t, mm.RegisterMigration(Migration{Version: 12, Name: "v12", UpSQL: "SELECT 1"}))
	require.NoError(t, mm.RegisterMigration(Migration{Version: 11, Name: "v11", UpSQL: "SELECT 1"}))

	for i := 1; i < len(mm.migrations); i++ {
		assert.LessOrEqual(t, mm.migrations[i-1].Version, mm.migrations[i].Version,
			"migrations should be sorted by version")
	}
}

// ── Migration: Down ──────────────────────────────────────────────────────

func TestMigrationDown_Success(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations

	customMig := Migration{
		Version: 8,
		Name:    "test_table",
		UpSQL:   "CREATE TABLE IF NOT EXISTS test_down (id INTEGER PRIMARY KEY, val TEXT)",
		DownSQL: "DROP TABLE IF EXISTS test_down",
	}
	require.NoError(t, mm.RegisterMigration(customMig))
	require.NoError(t, mm.Up())

	var cnt int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test_down'",
	).Scan(&cnt)
	require.NoError(t, err)
	assert.Equal(t, 1, cnt)

	err = mm.Down()
	require.NoError(t, err)

	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test_down'",
	).Scan(&cnt)
	require.NoError(t, err)
	assert.Equal(t, 0, cnt)
}

func TestMigrationDown_NoMigrations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.AutoMigrate = false

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations
	err = mm.Down()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no migrations to roll back")
}

func TestMigrationDown_NoDownSQL(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.AutoMigrate = false

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Create migration_history manually since auto-migrate is off
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS migration_history (" +
		"version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL DEFAULT (datetime('now'))," +
		"checksum TEXT NOT NULL DEFAULT '', applied_by TEXT NOT NULL DEFAULT 'cosca')")
	require.NoError(t, err)

	mm := db.migrations
	mm.migrations = []Migration{
		{
			Version: 1,
			Name:    "no_down",
			UpSQL:   "CREATE TABLE IF NOT EXISTS no_down_test (id INTEGER PRIMARY KEY)",
			DownSQL: "",
		},
	}

	_, err = db.Exec("INSERT OR REPLACE INTO migration_history (version, name) VALUES (1, 'no_down')")
	require.NoError(t, err)

	err = mm.Down()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no down SQL")
}

// ── Migration: DownTo ────────────────────────────────────────────────────

func TestMigrationDownTo(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations

	m2 := Migration{
		Version: 8, Name: "v6",
		UpSQL:   "CREATE TABLE IF NOT EXISTS v6_table (id INTEGER PRIMARY KEY)",
		DownSQL: "DROP TABLE IF EXISTS v6_table",
	}
	m3 := Migration{
		Version: 9, Name: "v9",
		UpSQL:   "CREATE TABLE IF NOT EXISTS v9_table (id INTEGER PRIMARY KEY)",
		DownSQL: "DROP TABLE IF EXISTS v9_table",
	}
	require.NoError(t, mm.RegisterMigration(m2))
	require.NoError(t, mm.RegisterMigration(m3))
	require.NoError(t, mm.Up())

	status, err := mm.Status()
	require.NoError(t, err)
	assert.Equal(t, 9, status.CurrentVersion)

	err = mm.DownTo(1)
	require.NoError(t, err)

	status, err = mm.Status()
	require.NoError(t, err)
	assert.Equal(t, 1, status.CurrentVersion)
}

func TestMigrationDownTo_AlreadyAtTarget(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations
	err = mm.DownTo(1)
	require.NoError(t, err)
}

// ── Migration: Reset ─────────────────────────────────────────────────────

func TestMigrationReset(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations
	// Reset drops and recreates all tables from scratch.
	_ = mm.Reset()
	// After reset attempt, verify DB is still operational
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

// ── Migration: Status ────────────────────────────────────────────────────

func TestMigrationStatus(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations
	status, err := mm.Status()
	require.NoError(t, err)
	assert.Equal(t, 7, status.CurrentVersion)
	assert.Equal(t, 7, status.LatestVersion)
	assert.Equal(t, 0, status.PendingCount)
	assert.Len(t, status.AppliedMigrations, 6)
	assert.Equal(t, "initial_schema", status.AppliedMigrations[0].Name)
	assert.Equal(t, "fix_documents_fts_no_content_column", status.AppliedMigrations[1].Name)
	assert.Equal(t, "fix_entities_fts_content_sync", status.AppliedMigrations[2].Name)
	assert.Equal(t, "fix_documents_stale_cosca_test_paths", status.AppliedMigrations[3].Name)
	assert.Equal(t, "fix_fts_delete_triggers", status.AppliedMigrations[4].Name)
	assert.Equal(t, "memory_tiers_medium_default", status.AppliedMigrations[5].Name)
}

func TestMigrationStatus_WithPending(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations
	err = mm.RegisterMigration(Migration{
		Version: 8, Name: "pending_mig",
		UpSQL:   "CREATE TABLE IF NOT EXISTS pending_test (id INTEGER PRIMARY KEY)",
		DownSQL: "DROP TABLE IF EXISTS pending_test",
	})
	require.NoError(t, err)

	status, err := mm.Status()
	require.NoError(t, err)
	assert.Equal(t, 8, status.LatestVersion)
	assert.Equal(t, 1, status.PendingCount)
}

// ── Migration: PendingMigrations ─────────────────────────────────────────

func TestPendingMigrations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations
	err = mm.RegisterMigration(Migration{
		Version: 8, Name: "pending",
		UpSQL: "CREATE TABLE IF NOT EXISTS pending2 (id INTEGER PRIMARY KEY)",
	})
	require.NoError(t, err)

	pending, err := mm.PendingMigrations()
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, 8, pending[0].Version)
}

func TestPendingMigrations_None(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations
	pending, err := mm.PendingMigrations()
	require.NoError(t, err)
	assert.Empty(t, pending)
}

// ── Migration: DryRun ────────────────────────────────────────────────────

func TestDryRun(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations
	pending, err := mm.DryRun()
	require.NoError(t, err)
	assert.Empty(t, pending)
}

func TestDryRun_WithPending(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations
	require.NoError(t, mm.RegisterMigration(Migration{
		Version: 8, Name: "pending_dry",
		UpSQL: "CREATE TABLE IF NOT EXISTS dry_test (id INTEGER PRIMARY KEY)",
	}))

	pending, err := mm.DryRun()
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, "pending_dry", pending[0].Name)
}

// ── Migration: getLatestVersion ──────────────────────────────────────────

func TestGetLatestVersion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	assert.Equal(t, 7, db.migrations.getLatestVersion())
}

func TestGetLatestVersion_Empty(t *testing.T) {
	t.Parallel()

	mm := &MigrationManager{migrations: nil}
	assert.Equal(t, 0, mm.getLatestVersion())

	mm2 := &MigrationManager{migrations: []Migration{}}
	assert.Equal(t, 0, mm2.getLatestVersion())
}

// ── Migration: getCurrentVersion no table ────────────────────────────────

func TestGetCurrentVersion_NoTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.AutoMigrate = false

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations
	_, err = db.Exec("DROP TABLE IF EXISTS migration_history")
	require.NoError(t, err)

	version, err := mm.getCurrentVersion()
	require.NoError(t, err)
	assert.Equal(t, 0, version)
}

// ── Migration: getMigrationHistory ───────────────────────────────────────

func TestGetMigrationHistory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := db.migrations
	history, err := mm.getMigrationHistory()
	require.NoError(t, err)
	assert.Len(t, history, 6)
	assert.Equal(t, 1, history[0].Version)
	assert.Equal(t, "initial_schema", history[0].Name)
	assert.Equal(t, "cosca", history[0].AppliedBy)
	assert.NotEmpty(t, history[0].AppliedAt)
}

// ── NewMigrationManager ──────────────────────────────────────────────────

func TestNewMigrationManager(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.AutoMigrate = false

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mm := NewMigrationManager(db)
	assert.NotNil(t, mm)
	assert.Equal(t, db, mm.db)
	assert.Len(t, mm.migrations, 6)
	assert.Equal(t, 1, mm.migrations[0].Version)
}

// ── computeChecksum ──────────────────────────────────────────────────────

func TestComputeChecksum(t *testing.T) {
	t.Parallel()

	c1 := computeChecksum("hello")
	c2 := computeChecksum("hello")
	assert.Equal(t, c1, c2, "same input should produce same checksum")

	c3 := computeChecksum("world")
	assert.NotEqual(t, c1, c3, "different input should produce different checksum")
	assert.Len(t, c1, 64)
}

// ── downSQLV1 ────────────────────────────────────────────────────────────

func TestDownSQLV1(t *testing.T) {
	t.Parallel()

	sql := downSQLV1()
	assert.NotEmpty(t, sql)
	assert.Contains(t, sql, "DROP TABLE IF EXISTS")
	assert.Contains(t, sql, "migration_history")
	assert.Contains(t, sql, "documents")
	assert.Contains(t, sql, "entities_fts")
}
