package sqlite

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

// ── DefaultConfig tests ──────────────────────────────────────────────────

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("/data/mydb.db")

	assert.Equal(t, "/data/mydb.db", cfg.Path)
	assert.True(t, cfg.WALMode)
	assert.Equal(t, 5000, cfg.BusyTimeout)
	assert.Equal(t, -20000, cfg.CacheSize)
	assert.True(t, cfg.ForeignKeys)
	assert.Equal(t, "wal", cfg.JournalMode)
	assert.True(t, cfg.AutoMigrate)
	assert.Equal(t, filepath.Join(filepath.Dir("/data/mydb.db"), "backups"), cfg.BackupDir)
	assert.Equal(t, 25, cfg.MaxOpenConns)
	assert.Equal(t, 5, cfg.MaxIdleConns)
	assert.Equal(t, 30*time.Minute, cfg.ConnMaxLifetime)
}

// ── Open / Close tests ───────────────────────────────────────────────────

func TestOpen_ValidConfig(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)

	db, err := Open(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)

	assert.NotNil(t, db.Conn())

	err = db.Close()
	require.NoError(t, err)
}

func TestOpen_NestedSubdirCreation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "deep", "nested", "dir", "test.db")
	cfg := DefaultConfig(dbPath)

	db, err := Open(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)

	err = db.Close()
	require.NoError(t, err)

	_, err = os.Stat(filepath.Dir(dbPath))
	require.NoError(t, err)
}

func TestOpen_InvalidPath(t *testing.T) {
	t.Parallel()

	parentFile := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(parentFile, []byte("x"), 0o644))

	dbPath := filepath.Join(parentFile, "subdir", "test.db")
	_, err := Open(DefaultConfig(dbPath))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create database directory")
}

func TestOpen_AutoMigrateDisabled(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.AutoMigrate = false

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='migration_history'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestOpen_CustomPragmaValues(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.BusyTimeout = 10000
	cfg.CacheSize = -40000
	cfg.ForeignKeys = false
	cfg.JournalMode = "delete"

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var fk string
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	require.NoError(t, err)
	assert.Equal(t, "0", fk)

	var jm string
	err = db.QueryRow("PRAGMA journal_mode").Scan(&jm)
	require.NoError(t, err)
	assert.Equal(t, "delete", jm)
}

func TestOpen_ZeroPragmaDefaults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := Config{
		Path:        dbPath,
		AutoMigrate: true,
	}

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Verify the DB opened successfully (pragmas with zero config use nonZero defaults)
	// Note: modernc.org/sqlite may not persist all pragmas the same way as C SQLite
	var journalMode string
	err = db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	require.NoError(t, err)
	// journal_mode should default to "wal" via nonEmpty
	assert.Equal(t, "wal", journalMode)
}

func TestOpen_AllTablesCreated(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
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

	expectedTables := TableNames()
	for _, expected := range expectedTables {
		assert.Contains(t, tables, expected, "table %s should exist", expected)
	}
}

func TestOpen_ForeignKeysEnabled(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var fk string
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	require.NoError(t, err)
	assert.Equal(t, "1", fk)
}

// ── Close tests ──────────────────────────────────────────────────────────

func TestClose_DoubleClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)

	assert.NoError(t, db.Close())
	assert.NoError(t, db.Close())
}

func TestClose_WALCheckpoint(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.WALMode = true

	db, err := Open(cfg)
	require.NoError(t, err)

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS test_close (id INTEGER PRIMARY KEY, val TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO test_close (val) VALUES ('hello')")
	require.NoError(t, err)

	assert.NoError(t, db.Close())
	assert.True(t, db.closed)
}

func TestClose_NoWAL(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.WALMode = false

	db, err := Open(cfg)
	require.NoError(t, err)

	assert.NoError(t, db.Close())
}

// ── Conn / Wrap methods tests ────────────────────────────────────────────

func TestConn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	conn := db.Conn()
	assert.NotNil(t, conn)

	var one int
	err = conn.QueryRow("SELECT 1").Scan(&one)
	require.NoError(t, err)
	assert.Equal(t, 1, one)
}

func TestExec(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	result, err := db.Exec("CREATE TABLE IF NOT EXISTS test_exec (id INTEGER PRIMARY KEY, val TEXT)")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestQuery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rows, err := db.Query("SELECT 1 AS num UNION SELECT 2")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	count := 0
	for rows.Next() {
		var n int
		require.NoError(t, rows.Scan(&n))
		count++
	}
	assert.Equal(t, 2, count)
}

func TestPrepare(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	stmt, err := db.Prepare("SELECT ?")
	require.NoError(t, err)
	require.NotNil(t, stmt)
	defer func() { _ = stmt.Close() }()

	var result int
	err = stmt.QueryRow(42).Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, 42, result)
}

func TestBegin(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NotNil(t, tx)

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS tx_test (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)

	assert.NoError(t, tx.Commit())
}

// ── Operations on closed DB ──────────────────────────────────────────────

func TestOperationsOnClosedDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	require.NoError(t, db.Close())

	_, err = db.Exec("SELECT 1")
	require.Error(t, err)

	_, err = db.Query("SELECT 1")
	require.Error(t, err)

	_, err = db.Prepare("SELECT 1")
	require.Error(t, err)
}

// ── configurePragmas error path ──────────────────────────────────────────

func TestConfigurePragmas_ClosedDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	require.NoError(t, db.Close())

	err = db.configurePragmas()
	require.Error(t, err)
}

// ── Backup / Restore tests ──────────────────────────────────────────────

func TestBackup_Success(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.BackupDir = filepath.Join(t.TempDir(), "backups")

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS test_backup (id INTEGER PRIMARY KEY, val TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO test_backup (val) VALUES ('backup-test')")
	require.NoError(t, err)

	backupPath, err := db.Backup("mytest")
	require.NoError(t, err)
	assert.Contains(t, backupPath, cfg.BackupDir)
	assert.Contains(t, backupPath, "mytest")
	assert.Contains(t, backupPath, ".db")

	info, err := os.Stat(backupPath)
	require.NoError(t, err)
	assert.True(t, info.Size() > 0)
}

func TestBackup_EmptyName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.BackupDir = filepath.Join(t.TempDir(), "backups")

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	backupPath, err := db.Backup("")
	require.NoError(t, err)
	assert.Contains(t, backupPath, "cosca-kg-backup-")
	assert.Contains(t, backupPath, ".db")

	_, err = os.Stat(backupPath)
	require.NoError(t, err)
}

func TestBackup_PathTraversal(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.BackupDir = filepath.Join(t.TempDir(), "backups")

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Backup("../escape")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes backup directory")
}

func TestBackup_CreatesBackupDir(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.BackupDir = filepath.Join(t.TempDir(), "new-backups")

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Backup("test")
	require.NoError(t, err)

	_, err = os.Stat(cfg.BackupDir)
	require.NoError(t, err)
}

func TestRestore_Success(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	backupDir := filepath.Join(t.TempDir(), "backups")
	cfg := DefaultConfig(dbPath)
	cfg.BackupDir = backupDir

	db, err := Open(cfg)
	require.NoError(t, err)

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS test_restore (id INTEGER PRIMARY KEY, val TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO test_restore (val) VALUES ('restore-test')")
	require.NoError(t, err)

	backupPath, err := db.Backup("restore")
	require.NoError(t, err)

	_, err = db.Exec("DROP TABLE IF EXISTS test_restore")
	require.NoError(t, err)

	_ = db.Close()

	db2, err := Open(cfg)
	require.NoError(t, err)

	err = db2.Restore(backupPath)
	require.NoError(t, err)

	var count int
	err = db2.QueryRow("SELECT COUNT(*) FROM test_restore").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	_ = db2.Close()
}

func TestRestore_PathTraversal(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.BackupDir = filepath.Join(t.TempDir(), "backups")

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	err = db.Restore("/etc/passwd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes backup directory")
}

func TestRestore_FileNotFound(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.BackupDir = filepath.Join(t.TempDir(), "backups")

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	err = db.Restore(filepath.Join(cfg.BackupDir, "nonexistent.db"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup file not found")
}

func TestRestore_EmptyFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	backupDir := filepath.Join(t.TempDir(), "backups")
	cfg := DefaultConfig(dbPath)
	cfg.BackupDir = backupDir

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	emptyFile := filepath.Join(backupDir, "empty.db")
	require.NoError(t, os.MkdirAll(backupDir, 0o755))
	require.NoError(t, os.WriteFile(emptyFile, []byte{}, 0o644))

	err = db.Restore(emptyFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup file is empty")
}

// ── Vacuum test ──────────────────────────────────────────────────────────

func TestVacuum(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS test_vacuum (id INTEGER PRIMARY KEY, val TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO test_vacuum (val) VALUES ('data1'), ('data2'), ('data3')")
	require.NoError(t, err)
	_, err = db.Exec("DELETE FROM test_vacuum")
	require.NoError(t, err)

	err = db.Vacuum()
	require.NoError(t, err)
}

// ── IntegrityCheck tests ─────────────────────────────────────────────────

func TestIntegrityCheck_OK(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	issues, err := db.IntegrityCheck()
	require.NoError(t, err)
	assert.Empty(t, issues, "fresh database should pass integrity check")
}

// ── Stats test ───────────────────────────────────────────────────────────

func TestStats(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	stats := db.Stats()
	assert.Equal(t, 25, stats.MaxOpenConnections)
}

// ── Connection pool settings ─────────────────────────────────────────────

func TestConnectionPoolSettings(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.MaxOpenConns = 10
	cfg.MaxIdleConns = 3
	cfg.ConnMaxLifetime = 5 * time.Minute

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	stats := db.Stats()
	assert.Equal(t, 10, stats.MaxOpenConnections)
}

// ── Utility function tests ───────────────────────────────────────────────

func TestNonZero(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 5000, nonZero(0, 5000))
	assert.Equal(t, 100, nonZero(100, 5000))
	assert.Equal(t, -20000, nonZero(0, -20000))
}

func TestNonEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "wal", nonEmpty("", "wal"))
	assert.Equal(t, "delete", nonEmpty("delete", "wal"))
}

func TestBoolToOnOff(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "ON", boolToOnOff(true))
	assert.Equal(t, "OFF", boolToOnOff(false))
}

func TestIsPathWithin(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	assert.True(t, isPathWithin(filepath.Join(tmpDir, "child.db"), tmpDir))
	assert.True(t, isPathWithin(filepath.Join(tmpDir, "a", "b", "c.db"), tmpDir))
	assert.False(t, isPathWithin(tmpDir, tmpDir))
	assert.False(t, isPathWithin(filepath.Join(tmpDir, "..", "escape.db"), tmpDir))
	assert.False(t, isPathWithin("/etc/passwd", tmpDir))
	assert.False(t, isPathWithin(tmpDir, "\x00invalid"))
}

func TestIsPathWithin_DeepNesting(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	deepPath := tmpDir
	for i := 0; i < 10; i++ {
		deepPath = filepath.Join(deepPath, "sub")
	}
	deepPath = filepath.Join(deepPath, "file.db")

	assert.True(t, isPathWithin(deepPath, tmpDir))
}

// ── WAL pragma verification ──────────────────────────────────────────────

func TestWALPragmaApplied(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.WALMode = true
	cfg.JournalMode = "wal"

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var jm string
	err = db.QueryRow("PRAGMA journal_mode").Scan(&jm)
	require.NoError(t, err)
	assert.Equal(t, "wal", jm)
}

func TestBusyTimeoutConfigured(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.BusyTimeout = 8000

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var bt int
	err = db.QueryRow("PRAGMA busy_timeout").Scan(&bt)
	require.NoError(t, err)
	assert.Equal(t, 8000, bt)
}

func TestCacheSizeConfigured(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := DefaultConfig(dbPath)
	cfg.CacheSize = -30000

	db, err := Open(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var cs int
	err = db.QueryRow("PRAGMA cache_size").Scan(&cs)
	require.NoError(t, err)
	assert.Equal(t, -30000, cs)
}

// ── QueryRow convenience method ──────────────────────────────────────────

func TestQueryRow(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var result int
	err = db.QueryRow("SELECT 42").Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, 42, result)
}

// ── Ensure unused imports compile ────────────────────────────────────────

var _ sql.DBStats
