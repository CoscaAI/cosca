// Package audit provides an audit logging system with SQLite-backed persistence.
// It records security-relevant events such as login attempts, user management,
// and API key operations for compliance and forensic purposes.
package audit

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/privfile"
	"github.com/google/uuid"
	_ "modernc.org/sqlite" // Import SQLite driver for audit log storage
)

// AuditEntry represents a single audit log record capturing a security or
// administrative event.
type AuditEntry struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
	Details   string `json:"details"`
	IPAddress string `json:"ip_address"`
	Timestamp int64  `json:"timestamp"`
	Status    string `json:"status"` // success, denied, error
}

// AuditFilters holds optional filter criteria for listing audit entries.
type AuditFilters struct {
	UserID   string
	Action   string
	Resource string
	Status   string
	From     int64 // Unix timestamp — inclusive lower bound on timestamp
	To       int64 // Unix timestamp — inclusive upper bound on timestamp
}

// Store is a thread-safe, SQLite-backed audit log store.
type Store struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// NewStore opens or creates the audit database at the given path. It ensures
// the parent directory exists, creates the audit_logs table with indexes,
// and configures WAL mode for concurrent access.
func NewStore(dbPath string) (*Store, error) {
	// In-memory SQLite DSNs (":memory:...") must NOT materialize a physical
	// file. EnsurePrivateDBFile would otherwise litter the working directory
	// with a zero-byte artifact named after the DSN (e.g. ":memory:?cache=shared")
	// on every test run that opens an in-memory store.
	if !isMemoryDSN(dbPath) {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return nil, fmt.Errorf("create audit db directory: %w", err)
		}

		if err := privfile.EnsurePrivateDBFile(dbPath); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open audit database: %w", err)
	}

	// Configure SQLite for performance and safety.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA cache_size=-20000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	s := &Store{db: db, dbPath: dbPath}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate audit database: %w", err)
	}

	return s, nil
}

// migrate creates the audit_logs table and indexes if they do not exist.
func (s *Store) migrate() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS audit_logs (
		id         TEXT PRIMARY KEY,
		user_id    TEXT NOT NULL DEFAULT '',
		action     TEXT NOT NULL,
		resource   TEXT NOT NULL DEFAULT '',
		details    TEXT NOT NULL DEFAULT '{}',
		ip_address TEXT NOT NULL DEFAULT '',
		timestamp  INTEGER NOT NULL,
		status     TEXT NOT NULL DEFAULT 'success'
	);

	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_logs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_user_id   ON audit_logs(user_id);
	CREATE INDEX IF NOT EXISTS idx_audit_action    ON audit_logs(action);
	`

	_, err := s.db.Exec(ddl)
	return err
}

// Record persists an audit entry. If the entry has no ID, one is generated.
// If it has no timestamp, the current time is used.
func (s *Store) Record(entry *AuditEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp == 0 {
		entry.Timestamp = time.Now().Unix()
	}
	if entry.Details == "" {
		entry.Details = "{}"
	}
	if entry.Status == "" {
		entry.Status = "success"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO audit_logs (id, user_id, action, resource, details, ip_address, timestamp, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.UserID, entry.Action, entry.Resource,
		entry.Details, entry.IPAddress, entry.Timestamp, entry.Status,
	)
	if err != nil {
		return fmt.Errorf("record audit entry: %w", err)
	}

	return nil
}

// List returns a paginated, filtered list of audit entries ordered by
// timestamp descending (newest first).
func (s *Store) List(limit, offset int, filters AuditFilters) ([]AuditEntry, int, error) {
	s.mu.RLock()
	if s.db == nil {
		s.mu.RUnlock()
		return nil, 0, fmt.Errorf("audit store not initialized")
	}
	defer s.mu.RUnlock()

	// Build query with optional filters.
	where := "WHERE 1=1"
	args := make([]interface{}, 0, 4)

	if filters.UserID != "" {
		where += " AND user_id = ?"
		args = append(args, filters.UserID)
	}
	if filters.Action != "" {
		where += " AND action = ?"
		args = append(args, filters.Action)
	}
	if filters.Resource != "" {
		where += " AND resource = ?"
		args = append(args, filters.Resource)
	}
	if filters.Status != "" {
		where += " AND status = ?"
		args = append(args, filters.Status)
	}
	if filters.From > 0 {
		where += " AND timestamp >= ?"
		args = append(args, filters.From)
	}
	if filters.To > 0 {
		where += " AND timestamp <= ?"
		args = append(args, filters.To)
	}

	// Count total matching rows.
	countQuery := "SELECT COUNT(*) FROM audit_logs " + where
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit entries: %w", err)
	}

	// Fetch page.
	if limit <= 0 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	selectQuery := "SELECT id, user_id, action, resource, details, ip_address, timestamp, status FROM audit_logs " +
		where + " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	selectArgs := append(args, limit, offset)

	rows, err := s.db.Query(selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit entries: %w", err)
	}
	defer rows.Close()

	entries := make([]AuditEntry, 0)
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.Resource,
			&e.Details, &e.IPAddress, &e.Timestamp, &e.Status); err != nil {
			return nil, 0, fmt.Errorf("scan audit entry: %w", err)
		}
		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit entries: %w", err)
	}

	if entries == nil {
		entries = make([]AuditEntry, 0)
	}

	return entries, total, nil
}

// Prune removes audit entries older than the given Unix timestamp.
// Returns the number of deleted entries.
func (s *Store) Prune(before int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec("DELETE FROM audit_logs WHERE timestamp < ?", before)
	if err != nil {
		return 0, fmt.Errorf("prune audit entries: %w", err)
	}
	deleted, _ := result.RowsAffected()
	return deleted, nil
}

// GetByID retrieves a single audit entry by its ID. Returns nil if not found.
func (s *Store) GetByID(id string) (*AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var e AuditEntry
	err := s.db.QueryRow(
		"SELECT id, user_id, action, resource, details, ip_address, timestamp, status FROM audit_logs WHERE id = ?",
		id,
	).Scan(&e.ID, &e.UserID, &e.Action, &e.Resource, &e.Details, &e.IPAddress, &e.Timestamp, &e.Status)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get audit entry: %w", err)
	}

	return &e, nil
}

// Close cleanly shuts down the database connection after a WAL checkpoint.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		return s.db.Close()
	}
	return nil
}

// DetailsJSON is a convenience helper that marshals an arbitrary value to a
// JSON string suitable for the Details field.
func DetailsJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":"marshal: %s"}`, err.Error())
	}
	return string(b)
}

// isMemoryDSN reports whether a SQLite path is an in-memory DSN, which must
// not be treated as a filesystem path.
func isMemoryDSN(path string) bool {
	return path == "" || strings.HasPrefix(path, ":memory:")
}
