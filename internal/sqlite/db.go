// Package sqlite provides the database management layer for the Cosca Knowledge Engine.
// It handles connection pooling, WAL mode, migrations, backup, and restore.
package sqlite

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	// SQLite database driver (pure Go, no CGO required)
	_ "modernc.org/sqlite"

	"github.com/rs/zerolog/log"
)

// DB wraps the sql.DB connection with Cosca-specific management features.
type DB struct {
	conn       *sql.DB
	cfg        Config
	mu         sync.RWMutex
	writeMu    sync.Mutex // serializes multi-step write transactions using this DB
	closed     bool
	migrations *MigrationManager
}

// LockWriter serializes compound write operations that must share this DB's
// single SQLite writer slot. The returned function releases the lock.
// Callers must keep the lock until their transaction is committed or rolled
// back; SQLite's busy timeout still handles writers outside this coordinator.
// The wait itself is not context-cancelable; callers should acquire it only
// immediately before a bounded transaction and rely on busy_timeout for
// cross-process contention.
func (db *DB) LockWriter() func() {
	db.writeMu.Lock()
	return db.writeMu.Unlock
}

// Config defines the SQLite database configuration.
type Config struct {
	// Path is the filesystem path to the SQLite database file.
	Path string

	// WALMode enables Write-Ahead Logging for better concurrent performance.
	WALMode bool

	// BusyTimeout sets the busy timeout in milliseconds (default: 5000).
	BusyTimeout int

	// CacheSize sets the page cache size in KB (default: -20000 = 20MB).
	CacheSize int

	// ForeignKeys enables foreign key constraint enforcement.
	ForeignKeys bool

	// JournalMode sets the SQLite journal mode.
	JournalMode string

	// AutoMigrate enables automatic migration on Open.
	AutoMigrate bool

	// BackupDir is the directory for backup files.
	BackupDir string

	// MaxOpenConns sets the maximum number of open connections.
	MaxOpenConns int

	// MaxIdleConns sets the maximum number of idle connections.
	MaxIdleConns int

	// ConnMaxLifetime sets the maximum connection lifetime.
	ConnMaxLifetime time.Duration
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig(path string) Config {
	return Config{
		Path:            path,
		WALMode:         true,
		BusyTimeout:     5000,
		CacheSize:       -20000,
		ForeignKeys:     true,
		JournalMode:     "wal",
		AutoMigrate:     true,
		BackupDir:       filepath.Join(filepath.Dir(path), "backups"),
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
	}
}

// Open opens or creates the SQLite database with the given configuration.
// It initializes WAL mode, sets pragmas, and optionally runs migrations.
func Open(cfg Config) (*DB, error) {
	// Validate migration ordering before doing anything else
	if err := ValidateMigrations(); err != nil {
		return nil, fmt.Errorf("validate migrations: %w", err)
	}

	// Create the database directory with owner-only permissions (0700) —
	// the knowledge base is not meant to be world-readable (M6b).
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(cfg.Path), 0700); err != nil {
		return nil, fmt.Errorf("restrict database directory permissions: %w", err)
	}

	dsn := cfg.Path
	// Apply connection-local locking options to every pooled connection,
	// including durable databases that opt out of FK enforcement.
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	foreignKeys := "0"
	if cfg.ForeignKeys {
		foreignKeys = "1"
	}
	dsn += sep + "_pragma=foreign_keys(" + foreignKeys + ")&_pragma=busy_timeout(" + fmt.Sprintf("%d", nonZero(cfg.BusyTimeout, 5000)) + ")&_txlock=immediate"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db := &DB{
		conn: conn,
		cfg:  cfg,
	}

	// Configure connection pool
	conn.SetMaxOpenConns(cfg.MaxOpenConns)
	conn.SetMaxIdleConns(cfg.MaxIdleConns)
	conn.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Apply pragmas for performance and safety
	if err := db.configurePragmas(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("configure pragmas: %w", err)
	}

	// Initialize migration manager and run migrations
	db.migrations = NewMigrationManager(db)
	if cfg.AutoMigrate {
		if err := db.migrations.Up(); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("auto-migrate: %w", err)
		}
	}

	// Restrict the database file (and any WAL/shm sidecars) to the owning
	// user. SQLite creates files with 0666 & ~umask (typically 0644), which
	// would expose the knowledge base to other local users. Chmod is applied
	// on every open so pre-existing files are hardened too (M6b). Best-effort:
	// the chmod is non-fatal so a read-only filesystem never blocks startup.
	restrictFilePerms(cfg.Path)

	log.Info().
		Str("path", cfg.Path).
		Bool("wal", cfg.WALMode).
		Bool("migrated", cfg.AutoMigrate).
		Msg("sqlite database opened")

	return db, nil
}

// configurePragmas applies SQLite pragma settings for optimal performance.
func (db *DB) configurePragmas() error {
	pragmas := []string{
		fmt.Sprintf("PRAGMA busy_timeout = %d", nonZero(db.cfg.BusyTimeout, 5000)),
		fmt.Sprintf("PRAGMA cache_size = %d", nonZero(db.cfg.CacheSize, -20000)),
		fmt.Sprintf("PRAGMA foreign_keys = %s", boolToOnOff(db.cfg.ForeignKeys)),
		fmt.Sprintf("PRAGMA journal_mode = %s", nonEmpty(db.cfg.JournalMode, "wal")),
	}

	for _, p := range pragmas {
		if _, err := db.conn.Exec(p); err != nil {
			return fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	return nil
}

// Conn returns the underlying *sql.DB for direct use.
// The caller MUST NOT close this connection; use db.Close() instead.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Close cleanly shuts down the database connection.
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil
	}
	db.closed = true

	// Run a final WAL checkpoint before closing
	if db.cfg.WALMode {
		_, _ = db.conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	}

	return db.conn.Close()
}

// Backup creates a point-in-time backup of the database.
func (db *DB) Backup(name string) (string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	// Backups contain the full knowledge base (including any embedded
	// secrets) — owner-only directory, never world-readable (M6b).
	if err := os.MkdirAll(db.cfg.BackupDir, 0o700); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	// Re-assert the mode on pre-existing directories (MkdirAll leaves
	// existing dirs untouched) so an older 0755 backup dir is hardened too.
	_ = os.Chmod(db.cfg.BackupDir, 0o700)

	timestamp := time.Now().UTC().Format("20060102T150405Z")
	backupName := name
	if backupName == "" {
		backupName = fmt.Sprintf("cosca-kg-backup-%s.db", timestamp)
	} else {
		backupName = fmt.Sprintf("%s-%s.db", name, timestamp)
	}
	backupPath := filepath.Join(db.cfg.BackupDir, backupName)

	// Validate backup path stays within backup directory
	if !isPathWithin(backupPath, db.cfg.BackupDir) {
		return "", fmt.Errorf("backup path escapes backup directory: %s", backupPath)
	}

	// Use SQLite backup API via VACUUM INTO (SQLite 3.27.0+)
	sql := fmt.Sprintf("VACUUM INTO %q", backupPath)
	if _, err := db.conn.Exec(sql); err != nil {
		return "", fmt.Errorf("backup database: %w", err)
	}

	log.Info().Str("path", backupPath).Str("name", name).Msg("database backup created")
	return backupPath, nil
}

// Restore replaces the current database with the given backup file.
func (db *DB) Restore(backupPath string) error {
	// Validate path stays within backup directory
	if !isPathWithin(backupPath, db.cfg.BackupDir) {
		return fmt.Errorf("backup path escapes backup directory: %s", backupPath)
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// Verify the backup file exists
	info, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("backup file is empty: %s", backupPath)
	}

	// Close current connection
	if err := db.conn.Close(); err != nil {
		return fmt.Errorf("close current database: %w", err)
	}

	// Copy backup to current location
	src, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(db.cfg.Path)
	if err != nil {
		return fmt.Errorf("create database file: %w", err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy backup to database: %w", err)
	}

	// Reopen the database
	conn, err := sql.Open("sqlite", db.cfg.Path)
	if err != nil {
		return fmt.Errorf("reopen database after restore: %w", err)
	}
	db.conn = conn

	if err := db.configurePragmas(); err != nil {
		return fmt.Errorf("reconfigure pragmas after restore: %w", err)
	}

	log.Info().Str("backup", backupPath).Msg("database restored from backup")
	return nil
}

// Vacuum reclaims unused space in the database.
func (db *DB) Vacuum() error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	start := time.Now()
	if _, err := db.conn.Exec("VACUUM"); err != nil {
		return fmt.Errorf("vacuum database: %w", err)
	}

	log.Info().Dur("duration", time.Since(start)).Msg("database vacuum completed")
	return nil
}

// IntegrityCheck runs the SQLite integrity_check pragma and returns any issues.
func (db *DB) IntegrityCheck() ([]string, error) {
	rows, err := db.conn.Query("PRAGMA integrity_check")
	if err != nil {
		return nil, fmt.Errorf("integrity check: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var issues []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return nil, fmt.Errorf("scan integrity result: %w", err)
		}
		if result != "ok" {
			issues = append(issues, result)
		}
	}

	return issues, rows.Err()
}

// Stats returns database statistics.
func (db *DB) Stats() sql.DBStats {
	return db.conn.Stats()
}

// Exec is a convenience wrapper around db.conn.Exec.
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.conn.Exec(query, args...)
}

// Query is a convenience wrapper around db.conn.Query.
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.conn.Query(query, args...)
}

// QueryRow is a convenience wrapper around db.conn.QueryRow.
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRow(query, args...)
}

// Prepare is a convenience wrapper around db.conn.Prepare.
func (db *DB) Prepare(query string) (*sql.Stmt, error) {
	return db.conn.Prepare(query)
}

// Begin is a convenience wrapper around db.conn.Begin.
func (db *DB) Begin() (*sql.Tx, error) {
	return db.conn.Begin()
}

// nonZero returns val if non-zero, otherwise defaultVal.
func nonZero(val, defaultVal int) int {
	if val == 0 {
		return defaultVal
	}
	return val
}

// nonEmpty returns val if non-empty, otherwise defaultVal.
func nonEmpty(val, defaultVal string) string {
	if val == "" {
		return defaultVal
	}
	return val
}

// boolToOnOff converts a boolean to "ON" or "OFF" for SQLite pragmas.
func boolToOnOff(b bool) string {
	if b {
		return "ON"
	}
	return "OFF"
}

// restrictFilePerms chmods the database file and its sidecars to 0600.
// SQLite creates files with 0666 & ~umask (typically 0644); this hardens
// the knowledge base so other local users cannot read it. Best-effort:
// files that do not exist yet (e.g. the -wal/-shm sidecars before WAL
// activates) are ignored and other errors are non-fatal.
func restrictFilePerms(path string) {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(p, 0o600); err != nil && !os.IsNotExist(err) {
			// non-fatal: best-effort hardening
		}
	}
}

// isPathWithin returns true if target is within or equal to base directory.
// Uses filepath.Rel to detect path traversal attempts (e.g., "../../etc").
func isPathWithin(target, base string) bool {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}
	// If the relative path starts with "..", it escapes the base directory
	return !strings.HasPrefix(rel, "..") && rel != "."
}
