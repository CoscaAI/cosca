// Package sqlite provides a version-based migration system for the Cosca Knowledge Engine.
package sqlite

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Migration defines a single database migration.
type Migration struct {
	Version  int
	Name     string
	UpSQL    string
	DownSQL  string
	Checksum string
}

// MigrationManager handles version-based database migrations.
type MigrationManager struct {
	db         *DB
	mu         sync.RWMutex
	migrations []Migration
}

// NewMigrationManager creates a new migration manager.
func NewMigrationManager(db *DB) *MigrationManager {
	return &MigrationManager{
		db:         db,
		migrations: defaultMigrations(),
	}
}

// RegisterMigration adds a custom migration to the manager.
func (m *MigrationManager) RegisterMigration(mig Migration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate version
	for _, existing := range m.migrations {
		if existing.Version == mig.Version {
			return fmt.Errorf("migration version %d already exists", mig.Version)
		}
	}

	m.migrations = append(m.migrations, mig)
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	return nil
}

// Up runs all pending migrations.
func (m *MigrationManager) Up() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return fmt.Errorf("get current version: %w", err)
	}
	if err := m.validateAppliedMigrations(); err != nil {
		return err
	}

	for _, mig := range m.migrations {
		if mig.Version > currentVersion {
			log.Info().Int("version", mig.Version).Str("name", mig.Name).Msg("applying migration")

			// Compute checksum if not set
			checksum := mig.Checksum
			if checksum == "" {
				checksum = computeChecksum(mig.UpSQL)
			}

			tx, err := m.db.Begin()
			if err != nil {
				return fmt.Errorf("begin migration tx: %w", err)
			}

			// Execute each DDL statement individually (sqlite driver doesn't support multi-statement Exec)
			for _, stmt := range strings.Split(mig.UpSQL, "\n\n") {
				stmt = strings.TrimSpace(stmt)
				if stmt == "" {
					continue
				}
				if _, err := tx.Exec(stmt); err != nil {
					_ = tx.Rollback()
					return fmt.Errorf("migration %d (%s): %w", mig.Version, mig.Name, err)
				}
			}

			// Record migration in history
			_, err = tx.Exec(
				`INSERT OR REPLACE INTO migration_history (version, name, applied_at, checksum, applied_by)
				 VALUES (?, ?, datetime('now'), ?, 'cosca')`,
				mig.Version, mig.Name, checksum,
			)
			if err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("record migration %d: %w", mig.Version, err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit migration %d: %w", mig.Version, err)
			}

			log.Info().Int("version", mig.Version).Str("name", mig.Name).Msg("migration applied")
		}
	}
	if err := m.verifyRegisteredObjects(); err != nil {
		return err
	}

	return nil
}

// validateAppliedMigrations is intentionally fail-closed: a database whose
// history was edited, or whose migration definition drifted, must not be
// advanced using the wrong schema.
func (m *MigrationManager) validateAppliedMigrations() error {
	records, err := m.getMigrationHistory()
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil
		}
		return fmt.Errorf("validate migration history: %w", err)
	}
	for _, r := range records {
		var found *Migration
		for i := range m.migrations {
			if m.migrations[i].Version == r.Version {
				found = &m.migrations[i]
				break
			}
		}
		if found == nil {
			// Durable v6 is an explicit opt-in capability and is therefore
			// intentionally absent from the default manager.
			if r.Version == 6 && r.Name == "durable_run_ledger" {
				continue
			}
			return fmt.Errorf("migration drift: applied version %d is not registered", r.Version)
		}
		expected := found.Checksum
		if expected == "" {
			expected = computeChecksum(found.UpSQL)
		}
		if r.Name != found.Name || r.Checksum != expected {
			return fmt.Errorf("migration drift: version %d name/checksum mismatch", r.Version)
		}
	}
	return nil
}

func (m *MigrationManager) verifyRegisteredObjects() error {
	for _, mig := range m.migrations {
		if mig.Version == 6 && mig.Name == "durable_run_ledger" {
			if err := m.verifyDurableSchema(); err != nil {
				return err
			}
		}
	}
	return nil
}

// verifyDurableSchema is deliberately stricter than a table-name check:
// CREATE IF NOT EXISTS cannot repair an existing, incompatible table.
func (m *MigrationManager) verifyDurableSchema() error {
	tables := map[string][]string{
		"durable_runs":            {"run_id", "workflow_ref", "input_hash", "state", "generation", "fencing_token_hash", "worker_id", "lease_until", "created_at", "updated_at", "completed_at"},
		"durable_steps":           {"run_id", "step_key", "status", "input_hash", "output_ref", "output_hash", "error_ref", "started_at", "completed_at"},
		"durable_events":          {"run_id", "seq", "event_type", "payload_ref", "payload_hash", "created_at"},
		"durable_event_sequences": {"run_id", "next_seq"},
		"durable_checkpoints":     {"run_id", "checkpoint_key", "step_key", "data_ref", "data_hash", "seq", "created_at"},
		"durable_idempotency":     {"run_id", "idempotency_key", "effect_type", "status", "result_ref", "result_hash", "created_at"},
		"durable_effects":         {"run_id", "effect_key", "effect_type", "status", "result_ref", "result_hash", "error_ref", "created_at"},
		"durable_approvals":       {"run_id", "approval_id", "request_hash", "decision", "decided_by", "decided_at"},
	}
	for table, expected := range tables {
		var exists int
		if err := m.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&exists); err != nil {
			return err
		}
		if exists != 1 {
			return fmt.Errorf("migration v6 drift: missing table %s", table)
		}
		rows, err := m.db.Query("PRAGMA table_info(" + table + ")")
		if err != nil {
			return fmt.Errorf("migration v6 drift: inspect %s: %w", table, err)
		}
		actual := make(map[string]bool)
		for rows.Next() {
			var cid, notNull, pk int
			var name, typ string
			var def interface{}
			if err := rows.Scan(&cid, &name, &typ, &notNull, &def, &pk); err != nil {
				_ = rows.Close()
				return err
			}
			actual[name] = true
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return err
		}
		for _, column := range expected {
			if !actual[column] {
				return fmt.Errorf("migration v6 drift: %s missing column %s", table, column)
			}
		}
	}
	indexes := map[string]struct {
		table   string
		columns []string
	}{
		"idx_durable_runs_state":   {"durable_runs", []string{"state"}},
		"idx_durable_runs_lease":   {"durable_runs", []string{"lease_until"}},
		"idx_durable_steps_status": {"durable_steps", []string{"run_id", "status"}},
		"idx_durable_events_type":  {"durable_events", []string{"run_id", "event_type"}},
	}
	for name, want := range indexes {
		var table, column string
		if err := m.db.QueryRow("SELECT tbl_name, sql FROM sqlite_master WHERE type='index' AND name=?", name).Scan(&table, &column); err != nil {
			return fmt.Errorf("migration v6 drift: missing index %s", name)
		}
		rows, err := m.db.Query("PRAGMA index_info(" + name + ")")
		if err != nil {
			return fmt.Errorf("migration v6 drift: inspect index %s: %w", name, err)
		}
		var indexed []string
		for rows.Next() {
			var indexSeq, columnID int
			var column string
			if err := rows.Scan(&indexSeq, &columnID, &column); err != nil {
				_ = rows.Close()
				return err
			}
			indexed = append(indexed, column)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return err
		}
		if table != want.table || len(indexed) != len(want.columns) {
			return fmt.Errorf("migration v6 drift: invalid index %s", name)
		}
		for i := range indexed {
			if indexed[i] != want.columns[i] {
				return fmt.Errorf("migration v6 drift: invalid index %s", name)
			}
		}
	}
	for _, table := range []string{"durable_steps", "durable_events", "durable_event_sequences", "durable_checkpoints", "durable_idempotency", "durable_effects", "durable_approvals"} {
		rows, err := m.db.Query("PRAGMA foreign_key_list(" + table + ")")
		if err != nil {
			return err
		}
		var found bool
		for rows.Next() {
			var id, seq int
			var parent, from, to, onUpdate, onDelete, match string
			if err := rows.Scan(&id, &seq, &parent, &from, &to, &onUpdate, &onDelete, &match); err != nil {
				_ = rows.Close()
				return err
			}
			if from == "run_id" && to == "run_id" && parent == "durable_runs" && onDelete == "CASCADE" {
				found = true
			}
		}
		_ = rows.Close()
		if !found {
			return fmt.Errorf("migration v6 drift: %s missing run_id cascade foreign key", table)
		}
	}
	return nil
}

// Down rolls back the most recent migration. Use with caution.
func (m *MigrationManager) Down() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return fmt.Errorf("get current version: %w", err)
	}

	if currentVersion <= 0 {
		return fmt.Errorf("no migrations to roll back")
	}

	// Find the migration to roll back
	var mig *Migration
	for i := range m.migrations {
		if m.migrations[i].Version == currentVersion {
			mig = &m.migrations[i]
			break
		}
	}

	if mig == nil {
		return fmt.Errorf("migration %d not found", currentVersion)
	}

	if mig.DownSQL == "" {
		return fmt.Errorf("migration %d has no down SQL", mig.Version)
	}

	log.Warn().Int("version", mig.Version).Str("name", mig.Name).Msg("rolling back migration")

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("begin rollback tx: %w", err)
	}

	// Execute each DDL statement individually (sqlite driver doesn't support multi-statement Exec)
	for _, stmt := range strings.Split(mig.DownSQL, "\n\n") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.Exec(stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("rollback migration %d (%s): %w", mig.Version, mig.Name, err)
		}
	}

	// Remove from migration history
	if _, err := tx.Exec("DELETE FROM migration_history WHERE version = ?", mig.Version); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("remove migration history %d: %w", mig.Version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rollback: %w", err)
	}

	log.Warn().Int("version", mig.Version).Str("name", mig.Name).Msg("migration rolled back")
	return nil
}

// DownTo rolls back all migrations down to (but not including) the target version.
func (m *MigrationManager) DownTo(targetVersion int) error {
	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return fmt.Errorf("get current version: %w", err)
	}

	for currentVersion > targetVersion {
		if err := m.Down(); err != nil {
			return fmt.Errorf("rollback to %d: %w", targetVersion, err)
		}
		currentVersion, err = m.getCurrentVersion()
		if err != nil {
			return fmt.Errorf("re-get current version: %w", err)
		}
	}

	return nil
}

// Reset drops everything and re-applies all migrations from scratch.
func (m *MigrationManager) Reset() error {
	log.Warn().Msg("resetting database — all data will be lost")

	// Drop all tables
	tables := TableNames()

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("begin reset tx: %w", err)
	}

	// Drop FTS tables first (they reference content tables)
	ftsTables := []string{"entities_fts", "code_blocks_fts", "chunks_fts", "documents_fts"}
	for _, t := range ftsTables {
		if _, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", t)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("drop FTS table %s: %w", t, err)
		}
	}

	// Drop regular tables
	for _, t := range tables {
		// Skip FTS tables (already dropped) and migration_history (needed for reset)
		if strings.HasSuffix(t, "_fts") || t == "migration_history" {
			continue
		}
		if _, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", t)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("drop table %s: %w", t, err)
		}
	}

	// Clear migration history
	if _, err := tx.Exec("DELETE FROM migration_history"); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear migration history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reset: %w", err)
	}

	// Re-apply all migrations
	return m.Up()
}

// Status returns the current migration state.
func (m *MigrationManager) Status() (*MigrationStatus, error) {
	if err := m.validateAppliedMigrations(); err != nil {
		return nil, err
	}
	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return nil, fmt.Errorf("get current version: %w", err)
	}

	history, err := m.getMigrationHistory()
	if err != nil {
		return nil, fmt.Errorf("get migration history: %w", err)
	}

	status := &MigrationStatus{
		CurrentVersion:    currentVersion,
		LatestVersion:     m.getLatestVersion(),
		PendingCount:      0,
		AppliedMigrations: history,
	}

	// Count pending
	for _, mig := range m.migrations {
		if mig.Version > currentVersion {
			status.PendingCount++
		}
	}

	return status, nil
}

// getCurrentVersion returns the currently applied migration version.
func (m *MigrationManager) getCurrentVersion() (int, error) {
	var version int
	err := m.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM migration_history").Scan(&version)
	if err != nil {
		// Table may not exist on first run — treat as version 0
		if strings.Contains(err.Error(), "no such table") {
			return 0, nil
		}
		return 0, err
	}
	return version, nil
}

// getLatestVersion returns the latest available migration version.
func (m *MigrationManager) getLatestVersion() int {
	if len(m.migrations) == 0 {
		return 0
	}
	return m.migrations[len(m.migrations)-1].Version
}

// getMigrationHistory returns the full migration history.
func (m *MigrationManager) getMigrationHistory() ([]MigrationRecord, error) {
	rows, err := m.db.Query(
		"SELECT version, name, applied_at, checksum, applied_by FROM migration_history ORDER BY version",
	)
	if err != nil {
		return nil, fmt.Errorf("query migration history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var records []MigrationRecord
	for rows.Next() {
		var r MigrationRecord
		if err := rows.Scan(&r.Version, &r.Name, &r.AppliedAt, &r.Checksum, &r.AppliedBy); err != nil {
			return nil, fmt.Errorf("scan migration record: %w", err)
		}
		records = append(records, r)
	}

	return records, rows.Err()
}

// MigrationStatus shows the current state of migrations.
type MigrationStatus struct {
	CurrentVersion    int               `json:"current_version"`
	LatestVersion     int               `json:"latest_version"`
	PendingCount      int               `json:"pending_count"`
	AppliedMigrations []MigrationRecord `json:"applied_migrations"`
}

// MigrationRecord represents a single applied migration.
type MigrationRecord struct {
	Version   int    `json:"version"`
	Name      string `json:"name"`
	AppliedAt string `json:"applied_at"`
	Checksum  string `json:"checksum"`
	AppliedBy string `json:"applied_by"`
}

// defaultMigrations returns the set of built-in migrations.
func defaultMigrations() []Migration {
	return []Migration{
		{
			Version:  1,
			Name:     "initial_schema",
			UpSQL:    NewSchema().DDL(),
			DownSQL:  downSQLV1(),
			Checksum: computeChecksum(NewSchema().DDL()),
		},
		{
			Version:  2,
			Name:     "fix_documents_fts_no_content_column",
			UpSQL:    upSQLV2(),
			DownSQL:  downSQLV2(),
			Checksum: computeChecksum(upSQLV2()),
		},
		{
			Version:  3,
			Name:     "fix_entities_fts_content_sync",
			UpSQL:    upSQLV3(),
			DownSQL:  downSQLV3(),
			Checksum: computeChecksum(upSQLV3()),
		},
		{
			Version:  4,
			Name:     "fix_documents_stale_cosca_test_paths",
			UpSQL:    upSQLV4(),
			DownSQL:  downSQLV4(),
			Checksum: computeChecksum(upSQLV4()),
		},
		{
			Version:  5,
			Name:     "fix_fts_delete_triggers",
			UpSQL:    upSQLV5(),
			DownSQL:  downSQLV5(),
			Checksum: computeChecksum(upSQLV5()),
		},
		{
			Version:  7,
			Name:     "memory_tiers_medium_default",
			UpSQL:    upSQLV7(),
			DownSQL:  downSQLV7(),
			Checksum: computeChecksum(upSQLV7()),
		},
	}
}

// DurableMigration is opt-in because the ledger is an isolated capability.
// Consumers register it on their migration manager before calling Up.
func DurableMigration() Migration {
	return Migration{Version: 6, Name: "durable_run_ledger", UpSQL: upSQLV6(), DownSQL: downSQLV6(), Checksum: computeChecksum(upSQLV6())}
}

// ApplyMigrationSQL executes a migration's UpSQL directly, outside the version
// history. Up() only applies migrations with version > currentVersion, so an
// opt-in migration whose version predates the schema (durable v6 vs knowledge
// v7) would never run on an already-migrated database. The durable DDL is
// idempotent (CREATE ... IF NOT EXISTS), so applying it before Up() is safe
// and lets the post-apply schema verification pass.
func ApplyMigrationSQL(db *DB, upSQL string) error {
	for _, stmt := range strings.Split(upSQL, "\n\n") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// upSQLV6 is deliberately separate from the knowledge schema. Durable runs
// are opt-in at the API level, while their tables are harmless on existing DBs.
func upSQLV6() string {
	return strings.Join([]string{
		`CREATE TABLE IF NOT EXISTS durable_runs (run_id TEXT PRIMARY KEY, workflow_ref TEXT NOT NULL, input_hash TEXT NOT NULL, state TEXT NOT NULL, generation INTEGER NOT NULL DEFAULT 1, fencing_token_hash TEXT NOT NULL, worker_id TEXT NOT NULL DEFAULT '', lease_until TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now')), completed_at TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE IF NOT EXISTS durable_steps (run_id TEXT NOT NULL REFERENCES durable_runs(run_id) ON DELETE CASCADE, step_key TEXT NOT NULL, status TEXT NOT NULL, input_hash TEXT NOT NULL DEFAULT '', output_ref TEXT NOT NULL DEFAULT '', output_hash TEXT NOT NULL DEFAULT '', error_ref TEXT NOT NULL DEFAULT '', started_at TEXT NOT NULL DEFAULT '', completed_at TEXT NOT NULL DEFAULT '', PRIMARY KEY(run_id, step_key))`,
		`CREATE TABLE IF NOT EXISTS durable_events (run_id TEXT NOT NULL REFERENCES durable_runs(run_id) ON DELETE CASCADE, seq INTEGER NOT NULL, event_type TEXT NOT NULL, payload_ref TEXT NOT NULL DEFAULT '', payload_hash TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now')), PRIMARY KEY(run_id, seq))`,
		`CREATE TABLE IF NOT EXISTS durable_event_sequences (run_id TEXT PRIMARY KEY REFERENCES durable_runs(run_id) ON DELETE CASCADE, next_seq INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS durable_checkpoints (run_id TEXT NOT NULL REFERENCES durable_runs(run_id) ON DELETE CASCADE, checkpoint_key TEXT NOT NULL, step_key TEXT NOT NULL DEFAULT '', data_ref TEXT NOT NULL DEFAULT '', data_hash TEXT NOT NULL DEFAULT '', seq INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (datetime('now')), PRIMARY KEY(run_id, checkpoint_key))`,
		`CREATE TABLE IF NOT EXISTS durable_idempotency (run_id TEXT NOT NULL REFERENCES durable_runs(run_id) ON DELETE CASCADE, idempotency_key TEXT NOT NULL, effect_type TEXT NOT NULL, status TEXT NOT NULL, result_ref TEXT NOT NULL DEFAULT '', result_hash TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now')), PRIMARY KEY(run_id, idempotency_key))`,
		`CREATE TABLE IF NOT EXISTS durable_effects (run_id TEXT NOT NULL REFERENCES durable_runs(run_id) ON DELETE CASCADE, effect_key TEXT NOT NULL, effect_type TEXT NOT NULL, status TEXT NOT NULL, result_ref TEXT NOT NULL DEFAULT '', result_hash TEXT NOT NULL DEFAULT '', error_ref TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now')), PRIMARY KEY(run_id, effect_key))`,
		`CREATE TABLE IF NOT EXISTS durable_approvals (run_id TEXT NOT NULL REFERENCES durable_runs(run_id) ON DELETE CASCADE, approval_id TEXT NOT NULL, request_hash TEXT NOT NULL, decision TEXT NOT NULL, decided_by TEXT NOT NULL DEFAULT '', decided_at TEXT NOT NULL DEFAULT (datetime('now')), PRIMARY KEY(run_id, approval_id))`,
		`CREATE INDEX IF NOT EXISTS idx_durable_runs_state ON durable_runs(state)`,
		`CREATE INDEX IF NOT EXISTS idx_durable_runs_lease ON durable_runs(lease_until)`,
		`CREATE INDEX IF NOT EXISTS idx_durable_steps_status ON durable_steps(run_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_durable_events_type ON durable_events(run_id, event_type)`,
	}, "\n\n")
}

func downSQLV6() string {
	return strings.Join([]string{"DROP TABLE IF EXISTS durable_approvals", "DROP TABLE IF EXISTS durable_effects", "DROP TABLE IF EXISTS durable_idempotency", "DROP TABLE IF EXISTS durable_checkpoints", "DROP TABLE IF EXISTS durable_events", "DROP TABLE IF EXISTS durable_event_sequences", "DROP TABLE IF EXISTS durable_steps", "DROP TABLE IF EXISTS durable_runs"}, "\n\n")
}

// upSQLV2 fixes the documents_fts table by removing the content-sync mode
// (content='documents') that references a non-existent `content` column in
// the documents table. Documents do not have a body/content — the FTS5 index
// only needs to store title and doc_type.
//
// The fix drops and recreates the FTS table and its triggers, then rebuilds
// the index from existing document rows. New databases already get the
// corrected schema from migration v1.
func upSQLV2() string {
	return strings.Join([]string{
		// Drop old content-synced triggers
		"DROP TRIGGER IF EXISTS documents_ai",
		"DROP TRIGGER IF EXISTS documents_ad",
		"DROP TRIGGER IF EXISTS documents_au",
		// Drop the broken content-synced FTS table
		"DROP TABLE IF EXISTS documents_fts",
		// Recreate as self-managed content table (no content= option)
		"CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(" +
			"\n\t\ttitle," +
			"\n\t\tcontent," +
			"\n\t\tdoc_type," +
			"\n\t\ttokenize='porter unicode61'" +
			"\n\t)",
		// Recreate insert trigger
		"CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN\n" +
			"\tINSERT INTO documents_fts(rowid, title, content, doc_type)\n" +
			"\tVALUES (new.rowid, new.title, '', new.doc_type);\n" +
			"END",
		// Recreate delete trigger (self-managed FTS5: plain DELETE, not the
		// FTS5 special 'delete' command, which is invalid on tables without
		// content= and fails with "SQL logic error (1)")
		"CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN\n" +
			"\tDELETE FROM documents_fts WHERE rowid = old.rowid;\n" +
			"END",
		// Recreate update trigger
		"CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN\n" +
			"\tDELETE FROM documents_fts WHERE rowid = old.rowid;\n" +
			"\tINSERT INTO documents_fts(rowid, title, content, doc_type)\n" +
			"\tVALUES (new.rowid, new.title, '', new.doc_type);\n" +
			"END",
		// Repopulate the FTS index from existing documents
		"INSERT INTO documents_fts(rowid, title, content, doc_type) SELECT rowid, title, '', doc_type FROM documents",
	}, "\n\n")
}

// downSQLV2 reverts to the content-synced documents_fts definition (for rollback).
func downSQLV2() string {
	return strings.Join([]string{
		// Drop new triggers
		"DROP TRIGGER IF EXISTS documents_ai",
		"DROP TRIGGER IF EXISTS documents_ad",
		"DROP TRIGGER IF EXISTS documents_au",
		// Drop the self-managed FTS table
		"DROP TABLE IF EXISTS documents_fts",
		// Recreate as content-synced (original broken definition)
		"CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(" +
			"\n\t\ttitle," +
			"\n\t\tcontent," +
			"\n\t\tdoc_type," +
			"\n\t\ttokenize='porter unicode61'," +
			"\n\t\tcontent='documents'," +
			"\n\t\tcontent_rowid='rowid'" +
			"\n\t)",
		// Recreate insert trigger (original form)
		"CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN\n" +
			"\tINSERT INTO documents_fts(rowid, title, content, doc_type)\n" +
			"\tVALUES (new.rowid, new.title, '', new.doc_type);\n" +
			"END",
		// Recreate delete trigger
		"CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN\n" +
			"\tINSERT INTO documents_fts(documents_fts, rowid, title, content, doc_type)\n" +
			"\tVALUES ('delete', old.rowid, old.title, '', old.doc_type);\n" +
			"END",
		// Recreate update trigger
		"CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN\n" +
			"\tINSERT INTO documents_fts(documents_fts, rowid, title, content, doc_type)\n" +
			"\tVALUES ('delete', old.rowid, old.title, '', old.doc_type);\n" +
			"\tINSERT INTO documents_fts(rowid, title, content, doc_type)\n" +
			"\tVALUES (new.rowid, new.title, '', new.doc_type);\n" +
			"END",
		// Repopulate from existing documents
		"INSERT INTO documents_fts(rowid, title, content, doc_type) SELECT rowid, title, '', doc_type FROM documents",
	}, "\n\n")
}

// upSQLV3 fixes the entities_fts table by removing the content-sync mode
// (content='entities') that references a non-existent `metadata` column in the
// entities table. The entities table uses `metadata_json`, not `metadata`.
// With content-sync mode, the FTS5 rebuild command tries to SELECT columns
// from the source table by name — and `metadata` doesn't exist, causing
// rebuild (and potentially other operations) to fail.
//
// The fix drops and recreates the FTS table and its triggers as self-managed
// (no content= option), mapping metadata_json → metadata in the triggers.
func upSQLV3() string {
	return strings.Join([]string{
		// Drop old content-synced triggers
		"DROP TRIGGER IF EXISTS entities_ai",
		"DROP TRIGGER IF EXISTS entities_ad",
		"DROP TRIGGER IF EXISTS entities_au",
		// Drop the broken content-synced FTS table
		"DROP TABLE IF EXISTS entities_fts",
		// Recreate as self-managed content table (no content= option)
		"CREATE VIRTUAL TABLE IF NOT EXISTS entities_fts USING fts5(" +
			"\n\t\tname," +
			"\n\t\tentity_type," +
			"\n\t\tmetadata," +
			"\n\t\ttokenize='porter unicode61'" +
			"\n\t)",
		// Recreate insert trigger (maps metadata_json → metadata)
		"CREATE TRIGGER IF NOT EXISTS entities_ai AFTER INSERT ON entities BEGIN\n" +
			"\tINSERT INTO entities_fts(rowid, name, entity_type, metadata)\n" +
			"\tVALUES (new.rowid, new.name, new.entity_type, new.metadata_json);\n" +
			"END",
		// Recreate delete trigger (self-managed FTS5: plain DELETE, not the
		// FTS5 special 'delete' command, which is invalid on tables without
		// content= and fails with "SQL logic error (1)")
		"CREATE TRIGGER IF NOT EXISTS entities_ad AFTER DELETE ON entities BEGIN\n" +
			"\tDELETE FROM entities_fts WHERE rowid = old.rowid;\n" +
			"END",
		// Recreate update trigger
		"CREATE TRIGGER IF NOT EXISTS entities_au AFTER UPDATE ON entities BEGIN\n" +
			"\tDELETE FROM entities_fts WHERE rowid = old.rowid;\n" +
			"\tINSERT INTO entities_fts(rowid, name, entity_type, metadata)\n" +
			"\tVALUES (new.rowid, new.name, new.entity_type, new.metadata_json);\n" +
			"END",
		// Repopulate the FTS index from existing entities
		"INSERT INTO entities_fts(rowid, name, entity_type, metadata) " +
			"SELECT rowid, name, entity_type, metadata_json FROM entities",
	}, "\n\n")
}

// downSQLV3 reverts to the content-synced entities_fts definition (for rollback).
func downSQLV3() string {
	return strings.Join([]string{
		// Drop new triggers
		"DROP TRIGGER IF EXISTS entities_ai",
		"DROP TRIGGER IF EXISTS entities_ad",
		"DROP TRIGGER IF EXISTS entities_au",
		// Drop the self-managed FTS table
		"DROP TABLE IF EXISTS entities_fts",
		// Recreate as content-synced (original broken definition)
		"CREATE VIRTUAL TABLE IF NOT EXISTS entities_fts USING fts5(" +
			"\n\t\tname," +
			"\n\t\tentity_type," +
			"\n\t\tmetadata," +
			"\n\t\ttokenize='porter unicode61'," +
			"\n\t\tcontent='entities'," +
			"\n\t\tcontent_rowid='rowid'" +
			"\n\t)",
		// Recreate insert trigger (original form)
		"CREATE TRIGGER IF NOT EXISTS entities_ai AFTER INSERT ON entities BEGIN\n" +
			"\tINSERT INTO entities_fts(rowid, name, entity_type, metadata)\n" +
			"\tVALUES (new.rowid, new.name, new.entity_type, new.metadata_json);\n" +
			"END",
		// Recreate delete trigger
		"CREATE TRIGGER IF NOT EXISTS entities_ad AFTER DELETE ON entities BEGIN\n" +
			"\tINSERT INTO entities_fts(entities_fts, rowid, name, entity_type, metadata)\n" +
			"\tVALUES ('delete', old.rowid, old.name, old.entity_type, old.metadata_json);\n" +
			"END",
		// Recreate update trigger
		"CREATE TRIGGER IF NOT EXISTS entities_au AFTER UPDATE ON entities BEGIN\n" +
			"\tINSERT INTO entities_fts(entities_fts, rowid, name, entity_type, metadata)\n" +
			"\tVALUES ('delete', old.rowid, old.name, old.entity_type, old.metadata_json);\n" +
			"\tINSERT INTO entities_fts(rowid, name, entity_type, metadata)\n" +
			"\tVALUES (new.rowid, new.name, new.entity_type, new.metadata_json);\n" +
			"END",
		// Repopulate from existing entities
		"INSERT INTO entities_fts(rowid, name, entity_type, metadata) " +
			"SELECT rowid, name, entity_type, metadata_json FROM entities",
	}, "\n\n")
}

// upSQLV4 fixes stale document paths that point to the wrong project root.
//
// Background: .cosca/knowledge.db was restored from a backup that indexed
// documents under /home/cosca/Documents/cosca-test/... instead of the real
// project /home/cosca/Documents/cosca/. All 459 documents carry the stale
// prefix in documents.path, and the same path is embedded in
// documents.metadata_json ("path" key). This migration rewrites both columns
// with a pure string REPLACE — no rows are deleted and no tables are
// recreated. The chunks table has no path column (it references documents by
// document_id), so chunk rows are untouched.
func upSQLV4() string {
	return strings.Join([]string{
		"UPDATE documents SET path = REPLACE(path, '/home/cosca/Documents/cosca-test', '/home/cosca/Documents/cosca') " +
			"WHERE path LIKE '%cosca-test%'",
		"UPDATE documents SET metadata_json = REPLACE(metadata_json, '/home/cosca/Documents/cosca-test', '/home/cosca/Documents/cosca') " +
			"WHERE metadata_json LIKE '%cosca-test%'",
	}, "\n\n")
}

// downSQLV4 reverts the path rewrite performed by migration v4.
// Only rows that currently point into the corrected project root are
// affected; rows already containing cosca-test are left alone to avoid
// double-rewriting.
func downSQLV4() string {
	return strings.Join([]string{
		"UPDATE documents SET path = REPLACE(path, '/home/cosca/Documents/cosca', '/home/cosca/Documents/cosca-test') " +
			"WHERE path LIKE '/home/cosca/Documents/cosca/%' AND path NOT LIKE '%cosca-test%'",
		"UPDATE documents SET metadata_json = REPLACE(metadata_json, '/home/cosca/Documents/cosca', '/home/cosca/Documents/cosca-test') " +
			"WHERE metadata_json LIKE '%/home/cosca/Documents/cosca/%' AND metadata_json NOT LIKE '%cosca-test%'",
	}, "\n\n")
}

// upSQLV5 fixes the FTS5 sync triggers for self-managed (no content=) FTS5
// tables.
//
// Background: migrations v2 (documents_fts) and v3 (entities_fts) converted
// the FTS5 indexes to self-managed tables (dropping the content= option), but
// the AFTER DELETE / AFTER UPDATE triggers kept using the FTS5 special
// 'delete' INSERT command:
//
//	INSERT INTO documents_fts(documents_fts, rowid, ...) VALUES ('delete', ...)
//
// That special command is only valid for contentless (content=”) and
// external-content (content=tablename) FTS5 tables. On self-managed tables it
// fails with "SQL logic error (1)", which made EVERY UPDATE and DELETE on the
// documents and entities tables fail — including the v4 path-rewrite UPDATE.
//
// The fix drops and recreates the affected triggers using a plain
// `DELETE FROM <fts> WHERE rowid = old.rowid` statement, which is the correct
// sync pattern for self-managed FTS5 tables. chunks_fts and code_blocks_fts
// are external-content tables (content='chunks' / content='code_blocks') where
// the special 'delete' command is valid, so their triggers are left untouched.
func upSQLV5() string {
	return strings.Join([]string{
		// Drop the broken self-managed triggers
		"DROP TRIGGER IF EXISTS documents_ad",
		"DROP TRIGGER IF EXISTS documents_au",
		"DROP TRIGGER IF EXISTS entities_ad",
		"DROP TRIGGER IF EXISTS entities_au",
		// Recreate documents delete trigger
		"CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN\n" +
			"\tDELETE FROM documents_fts WHERE rowid = old.rowid;\n" +
			"END",
		// Recreate documents update trigger
		"CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN\n" +
			"\tDELETE FROM documents_fts WHERE rowid = old.rowid;\n" +
			"\tINSERT INTO documents_fts(rowid, title, content, doc_type)\n" +
			"\tVALUES (new.rowid, new.title, '', new.doc_type);\n" +
			"END",
		// Recreate entities delete trigger
		"CREATE TRIGGER IF NOT EXISTS entities_ad AFTER DELETE ON entities BEGIN\n" +
			"\tDELETE FROM entities_fts WHERE rowid = old.rowid;\n" +
			"END",
		// Recreate entities update trigger
		"CREATE TRIGGER IF NOT EXISTS entities_au AFTER UPDATE ON entities BEGIN\n" +
			"\tDELETE FROM entities_fts WHERE rowid = old.rowid;\n" +
			"\tINSERT INTO entities_fts(rowid, name, entity_type, metadata)\n" +
			"\tVALUES (new.rowid, new.name, new.entity_type, new.metadata_json);\n" +
			"END",
	}, "\n\n")
}

// downSQLV5 reverts the sync triggers to the pre-v5 definitions (which used
// the FTS5 special 'delete' command — broken on self-managed tables). Provided
// for rollback fidelity only; not recommended in practice.
func downSQLV5() string {
	return strings.Join([]string{
		// Drop the corrected triggers
		"DROP TRIGGER IF EXISTS documents_ad",
		"DROP TRIGGER IF EXISTS documents_au",
		"DROP TRIGGER IF EXISTS entities_ad",
		"DROP TRIGGER IF EXISTS entities_au",
		// Recreate documents delete trigger (original broken form)
		"CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN\n" +
			"\tINSERT INTO documents_fts(documents_fts, rowid, title, content, doc_type)\n" +
			"\tVALUES ('delete', old.rowid, old.title, '', old.doc_type);\n" +
			"END",
		// Recreate documents update trigger (original broken form)
		"CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN\n" +
			"\tINSERT INTO documents_fts(documents_fts, rowid, title, content, doc_type)\n" +
			"\tVALUES ('delete', old.rowid, old.title, '', old.doc_type);\n" +
			"\tINSERT INTO documents_fts(rowid, title, content, doc_type)\n" +
			"\tVALUES (new.rowid, new.title, '', new.doc_type);\n" +
			"END",
		// Recreate entities delete trigger (original broken form)
		"CREATE TRIGGER IF NOT EXISTS entities_ad AFTER DELETE ON entities BEGIN\n" +
			"\tINSERT INTO entities_fts(entities_fts, rowid, name, entity_type, metadata)\n" +
			"\tVALUES ('delete', old.rowid, old.name, old.entity_type, old.metadata_json);\n" +
			"END",
		// Recreate entities update trigger (original broken form)
		"CREATE TRIGGER IF NOT EXISTS entities_au AFTER UPDATE ON entities BEGIN\n" +
			"\tINSERT INTO entities_fts(entities_fts, rowid, name, entity_type, metadata)\n" +
			"\tVALUES ('delete', old.rowid, old.name, old.entity_type, old.metadata_json);\n" +
			"\tINSERT INTO entities_fts(rowid, name, entity_type, metadata)\n" +
			"\tVALUES (new.rowid, new.name, new.entity_type, new.metadata_json);\n" +
			"END",
	}, "\n\n")
}

// upSQLV7 adds memory tiers to documents: everything is medium-term (7 days)
// by default, long-term (1 year) only via explicit promotion — the Don's
// rule: "coloca tudo em medio, o longo a gente vai ver o que coloca" (L338).
func upSQLV7() string {
	return strings.Join([]string{
		`ALTER TABLE documents ADD COLUMN tier TEXT NOT NULL DEFAULT 'medium'`,
		`ALTER TABLE documents ADD COLUMN expires_at TEXT NOT NULL DEFAULT ''`,
		`UPDATE documents SET expires_at = datetime('now', '+7 days') WHERE expires_at = ''`,
		`CREATE INDEX IF NOT EXISTS idx_documents_tier ON documents(tier)`,
		`CREATE INDEX IF NOT EXISTS idx_documents_expires ON documents(expires_at)`,
	}, "\n\n")
}

// downSQLV7 reverts the tier columns. SQLite 3.35+ supports DROP COLUMN;
// the indexes are dropped first. The backfilled expires_at values are gone
// with the column — re-running upSQLV7 re-backfills everything.
func downSQLV7() string {
	return strings.Join([]string{
		`DROP INDEX IF EXISTS idx_documents_tier`,
		`DROP INDEX IF EXISTS idx_documents_expires`,
		`ALTER TABLE documents DROP COLUMN tier`,
		`ALTER TABLE documents DROP COLUMN expires_at`,
	}, "\n\n")
}

// downSQLV1 drops all tables created in migration v1.
func downSQLV1() string {
	tables := []string{
		"entities_fts", "code_blocks_fts", "chunks_fts", "documents_fts",
		"sync_log", "snapshots", "cache", "metadata", "frontmatter",
		"relationships", "entities", "symbols", "tables",
		"code_blocks", "headings", "chunks", "documents",
		"migration_history",
	}

	// Reverse order to respect foreign keys
	var stmts []string
	for i := len(tables) - 1; i >= 0; i-- {
		stmts = append(stmts, fmt.Sprintf("DROP TABLE IF EXISTS %s", tables[i]))
	}

	return strings.Join(stmts, ";\n")
}

// computeChecksum returns a SHA-256 checksum of the input string.
func computeChecksum(input string) string {
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h)
}

// PendingMigrations returns the list of migrations not yet applied.
func (m *MigrationManager) PendingMigrations() ([]Migration, error) {
	if err := m.validateAppliedMigrations(); err != nil {
		return nil, err
	}
	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return nil, err
	}

	var pending []Migration
	for _, mig := range m.migrations {
		if mig.Version > currentVersion {
			pending = append(pending, mig)
		}
	}

	return pending, nil
}

// DryRun shows what migrations would be applied without actually running them.
func (m *MigrationManager) DryRun() ([]Migration, error) {
	return m.PendingMigrations()
}

// ValidateMigrations checks that the default migrations are ordered correctly by version.
// Returns an error if any migration has a version <= the previous one.
func ValidateMigrations() error {
	migs := defaultMigrations()
	for i := 1; i < len(migs); i++ {
		if migs[i].Version <= migs[i-1].Version {
			return fmt.Errorf("migrations out of order: v%d after v%d", migs[i].Version, migs[i-1].Version)
		}
	}
	return nil
}

// compile-time check
var _ time.Time
