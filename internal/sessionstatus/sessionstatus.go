// Package sessionstatus provides a cross-session file-lock registry so
// agents in different sessions can coordinate: one session registers the
// files it is working on, another session checks whether those files are
// still in use before touching them.
//
// Table (SQLite, stored in .cosca/session_status.db):
//
//	CREATE TABLE IF NOT EXISTS session_locks (
//	    id         INTEGER PRIMARY KEY AUTOINCREMENT,
//	    session_id TEXT    NOT NULL,
//	    agent      TEXT    NOT NULL DEFAULT '',
//	    file_path  TEXT    NOT NULL,
//	    status     TEXT    NOT NULL DEFAULT 'working',  -- working | ready
//	    commit     TEXT    NOT NULL DEFAULT '',
//	    created_at TEXT    NOT NULL DEFAULT (datetime('now')),
//	    updated_at TEXT    NOT NULL DEFAULT (datetime('now'))
//	);
//
//	AUTO_CLEANUP: when a session transitions any file to 'ready', all its
//	'working' entries for the SAME file are removed (only the latest
//	'ready' row survives).
package sessionstatus

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	// Register the sqlite driver (modernc.org/sqlite).
	_ "modernc.org/sqlite"
)

// Lock holds one file-registration row.
type Lock struct {
	ID        int    `json:"id"`
	SessionID string `json:"session_id"`
	Agent     string `json:"agent"`
	FilePath  string `json:"file_path"`
	Status    string `json:"status"` // "working" or "ready"
	Commit    string `json:"commit"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// FileStatus is the aggregated status of one file across all sessions.
type FileStatus struct {
	FilePath string `json:"file_path"`
	InUse    bool   `json:"in_use"`
	Owner    string `json:"owner,omitempty"`  // session_id currently working
	Agent    string `json:"agent,omitempty"`  // agent that holds the lock
	Ready    bool   `json:"ready"`            // at least one session marked it ready
	Commit   string `json:"commit,omitempty"` // commit hash when marked ready
}

// Store is the SQLite-backed registry.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the session-status database.
func Open(coscaDir string) (*Store, error) {
	dbPath := filepath.Join(coscaDir, "session_status.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open session_status.db: %w", err)
	}
	if err := ensureSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Register marks files as being worked on by a session/agent. Existing
// 'working' entries for the same session+file are overwritten (idempotent).
func (s *Store) Register(sessionID, agent string, files []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, f := range files {
		f = cleanPath(f)
		// Remove any previous 'working' row for this session+file so we
		// never accumulate stale entries.
		if _, err := tx.Exec(
			`DELETE FROM session_locks WHERE session_id=? AND file_path=? AND status='working'`,
			sessionID, f,
		); err != nil {
			return fmt.Errorf("clean old lock: %w", err)
		}
		if _, err := tx.Exec(
			`INSERT INTO session_locks (session_id, agent, file_path, status)
			 VALUES (?, ?, ?, 'working')`,
			sessionID, agent, f,
		); err != nil {
			return fmt.Errorf("insert lock: %w", err)
		}
	}
	return tx.Commit()
}

// Ready transitions all files belonging to sessionID from 'working' to
// 'ready' and records the commit hash.
func (s *Store) Ready(sessionID, commit string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Mark all working entries for this session as ready.
	if _, err := tx.Exec(
		`UPDATE session_locks SET status='ready', commit=?, updated_at=datetime('now')
		 WHERE session_id=? AND status='working'`,
		commit, sessionID,
	); err != nil {
		return fmt.Errorf("update to ready: %w", err)
	}

	// Cleanup: for every file we just marked ready, remove older 'ready'
	// rows from other sessions (keep only the most recent ready per file).
	if _, err := tx.Exec(
		`DELETE FROM session_locks WHERE rowid NOT IN (
			SELECT MAX(rowid) FROM session_locks WHERE status='ready' GROUP BY file_path
		) AND status='ready'`,
	); err != nil {
		return fmt.Errorf("cleanup old ready: %w", err)
	}

	return tx.Commit()
}

// Status returns the aggregated status of all files currently registered.
func (s *Store) Status() ([]FileStatus, error) {
	return s.statusForFile("")
}

// StatusForFile returns the aggregated status of a single file.
func (s *Store) StatusForFile(filePath string) (*FileStatus, error) {
	items, err := s.statusForFile(cleanPath(filePath))
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return &FileStatus{FilePath: filePath, InUse: false}, nil
	}
	return &items[0], nil
}

func (s *Store) statusForFile(filter string) ([]FileStatus, error) {
	query := `SELECT file_path, session_id, agent, status, commit, MAX(updated_at)
		FROM session_locks`
	args := []interface{}{}
	if filter != "" {
		query += ` WHERE file_path = ?`
		args = append(args, filter)
	}
	query += ` GROUP BY file_path ORDER BY file_path`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query status: %w", err)
	}
	defer rows.Close()

	seen := map[string]*FileStatus{}
	for rows.Next() {
		var fp, sid, agent, status, commit, updated string
		if err := rows.Scan(&fp, &sid, &agent, &status, &commit, &updated); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		fs := &FileStatus{
			FilePath: fp,
			Commit:   commit,
		}
		if status == "working" {
			fs.InUse = true
			fs.Owner = sid
			fs.Agent = agent
		}
		if status == "ready" {
			fs.Ready = true
			if commit != "" {
				fs.Commit = commit
			}
		}
		// Merge: a file can be both 'working' (by one session) and
		// 'ready' (by another that already finished).
		if existing, ok := seen[fp]; ok {
			if fs.InUse {
				existing.InUse = true
				existing.Owner = fs.Owner
				existing.Agent = fs.Agent
			}
			if fs.Ready {
				existing.Ready = true
				if fs.Commit != "" {
					existing.Commit = fs.Commit
				}
			}
			_ = updated
		} else {
			seen[fp] = fs
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]FileStatus, 0, len(seen))
	for _, fs := range seen {
		out = append(out, *fs)
	}
	return out, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func ensureSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS session_locks (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT    NOT NULL,
			agent      TEXT    NOT NULL DEFAULT '',
			file_path  TEXT    NOT NULL,
			status     TEXT    NOT NULL DEFAULT 'working',
			commit     TEXT    NOT NULL DEFAULT '',
			created_at TEXT    NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT    NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX IF NOT EXISTS idx_locks_session ON session_locks(session_id);
		CREATE INDEX IF NOT EXISTS idx_locks_file   ON session_locks(file_path);
		CREATE INDEX IF NOT EXISTS idx_locks_status ON session_locks(status);
	`)
	return err
}

func cleanPath(p string) string {
	return strings.TrimSpace(filepath.Clean(p))
}

// SessionID returns a session identifier based on the current time.
// Agents use this when registering their work.
func SessionID() string {
	return time.Now().UTC().Format("20060102")
}
