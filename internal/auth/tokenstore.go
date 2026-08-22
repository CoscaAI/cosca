package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/CoscaAI/cosca/internal/privfile"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite" // SQLite driver for refresh-token revocation store
)

// ErrTokenReuse is returned by RevokeAndRotate when the presented refresh
// token is no longer the active one — indicating a rotated token was reused
// (a classic sign of a stolen/leaked token). The store revokes ALL refresh
// tokens for the user in that case.
var ErrTokenReuse = errors.New("refresh token reuse detected — all sessions revoked")

// TokenStore tracks active refresh tokens server-side so they can be revoked
// (logout) and rotated (refresh) with reuse detection.
//
// A stolen refresh token that is used once is replaced by a new token; when
// the attacker (or the legitimate user) presents the OLD token afterwards,
// RevokeAndRotate detects that it is no longer active and revokes every
// refresh token for the user — killing the compromised session.
//
// The store is backed by a SQLite database (default .cosca/auth_tokens.db,
// gitignored). With a nil *sql.DB it operates in-memory only (used by tests).
type TokenStore struct {
	db *sql.DB
}

// NewTokenStore opens (or creates) the refresh-token revocation database at
// dbPath. The parent directory is created automatically.
func NewTokenStore(dbPath string) (*TokenStore, error) {
	if dbPath == ":memory:" {
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			return nil, fmt.Errorf("open in-memory token database: %w", err)
		}
		return newTokenStoreWithDB(db)
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create token db directory: %w", err)
	}

	// Refresh tokens are credentials — the file must never be world readable.
	// SQLite would create it with the umask (0644); force 0600 and repair
	// legacy files.
	if err := privfile.EnsurePrivateDBFile(dbPath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open token database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("token db pragma: %w", err)
	}

	return newTokenStoreWithDB(db)
}

// NewTokenStoreInMemory creates a TokenStore backed by a shared in-memory
// SQLite database (for tests).
func NewTokenStoreInMemory() (*TokenStore, error) {
	db, err := sql.Open("sqlite", "file:auth_tokens?mode=memory&cache=shared")
	if err != nil {
		return nil, fmt.Errorf("open in-memory token database: %w", err)
	}
	return newTokenStoreWithDB(db)
}

// newTokenStoreWithDB initializes the schema on an existing connection.
func newTokenStoreWithDB(db *sql.DB) (*TokenStore, error) {
	// A single connection keeps an in-memory database consistent and avoids
	// SQLITE_BUSY on concurrent writes.
	db.SetMaxOpenConns(1)

	s := &TokenStore{db: db}
	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// initSchema creates the refresh_tokens table if it does not exist.
func (s *TokenStore) initSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS refresh_tokens (
			user_id    TEXT    NOT NULL,
			jti        TEXT    NOT NULL,
			expires_at INTEGER NOT NULL,
			revoked    INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (user_id, jti)
		)
	`)
	if err != nil {
		return fmt.Errorf("init refresh_tokens schema: %w", err)
	}
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens (expires_at)`)
	if err != nil {
		return fmt.Errorf("init refresh_tokens index: %w", err)
	}

	// Migration for pre-revocation databases (created before the revoked
	// column existed). Duplicate column errors are ignored — the column
	// already exists on fresh or already-migrated databases.
	_, _ = s.db.Exec(`ALTER TABLE refresh_tokens ADD COLUMN revoked INTEGER NOT NULL DEFAULT 0`)
	return nil
}

// Close closes the underlying database connection.
func (s *TokenStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// StoreRefresh records the given refresh token (identified by its jti) as an
// active session for the user, expiring at expiresAt. Multiple active refresh
// tokens per user are allowed (one per device/login).
func (s *TokenStore) StoreRefresh(userID, jti string, expiresAt time.Time) error {
	if userID == "" || jti == "" {
		return errors.New("store refresh token: user_id and jti are required")
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO refresh_tokens (user_id, jti, expires_at, revoked) VALUES (?, ?, ?, 0)`,
		userID, jti, expiresAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("store refresh token: %w", err)
	}
	return nil
}

// IsValid reports whether the given refresh token is currently stored,
// active (not revoked) and not yet expired.
func (s *TokenStore) IsValid(userID, jti string) bool {
	if s == nil || s.db == nil || userID == "" || jti == "" {
		return false
	}
	var expiresAt int64
	err := s.db.QueryRow(
		`SELECT expires_at FROM refresh_tokens WHERE user_id = ? AND jti = ? AND revoked = 0`,
		userID, jti,
	).Scan(&expiresAt)
	if err != nil {
		return false
	}
	return expiresAt > time.Now().Unix()
}

// Revoke marks ALL refresh tokens for the user as revoked (used on logout or
// when token reuse/theft is detected). Revoked rows are kept as tombstones so
// a later presentation of a revoked token can still be detected and rejected.
func (s *TokenStore) Revoke(userID string) error {
	if userID == "" {
		return errors.New("revoke refresh tokens: user_id is required")
	}
	if _, err := s.db.Exec(`UPDATE refresh_tokens SET revoked = 1 WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("revoke refresh tokens for %s: %w", userID, err)
	}
	return nil
}

// RevokeAndRotate rotates a refresh token during a refresh operation.
//
// Reuse detection: when oldJTI is NOT the currently active refresh token but
// the user has other active refresh tokens, the presented token has already
// been rotated/revoked — a strong signal of a stolen token. In that case ALL
// refresh tokens for the user are revoked and ErrTokenReuse is returned so
// the caller can reject the request.
//
// A presented token that is not active while the user has NO active tokens
// but DOES have recorded (revoked/expired) tokens is also rejected with
// ErrTokenReuse: after a logout or a reuse-detection cleanup every session is
// dead, so the token must not be silently re-accepted (fail closed).
//
// Only when the user has no recorded tokens at all (e.g. sessions issued
// before the store existed) is the rotation accepted without error, to keep
// legitimate legacy refresh working.
func (s *TokenStore) RevokeAndRotate(userID, oldJTI, newJTI string) error {
	if userID == "" || oldJTI == "" || newJTI == "" {
		return errors.New("rotate refresh token: user_id, old jti and new jti are required")
	}

	if s.IsValid(userID, oldJTI) {
		// Normal rotation: the presented token is the active one — retire it
		// (keep the row as a tombstone so later reuse is detected).
		if _, err := s.db.Exec(`UPDATE refresh_tokens SET revoked = 1 WHERE user_id = ? AND jti = ?`, userID, oldJTI); err != nil {
			return fmt.Errorf("rotate refresh token: %w", err)
		}
		return nil
	}

	// oldJTI is not active. If the user has OTHER active refresh tokens, the
	// old one was already rotated/revoked — reuse detected (token theft).
	hasActive, err := s.hasAnyActive(userID)
	if err != nil {
		return err
	}
	if hasActive {
		if err := s.Revoke(userID); err != nil { // kill the compromised session
			log.Error().Err(err).Str("user_id", userID).Msg("token revocation failed during reuse detection — compromised session may remain active")
		}
		return fmt.Errorf("%w: user %s, reused jti %s", ErrTokenReuse, userID, oldJTI)
	}

	// No active tokens remain, but the user has recorded tokens — the
	// presented one was revoked (logout) or killed by reuse detection.
	recorded, err := s.hasAnyRecorded(userID)
	if err != nil {
		return err
	}
	if recorded {
		if err := s.Revoke(userID); err != nil { // ensure every session stays dead
			log.Error().Err(err).Str("user_id", userID).Msg("token revocation failed after reuse of revoked token — session may remain active")
		}
		return fmt.Errorf("%w: user %s, revoked jti %s", ErrTokenReuse, userID, oldJTI)
	}
	return nil
}

// hasAnyActive reports whether the user has at least one unexpired, non-revoked
// refresh token recorded.
func (s *TokenStore) hasAnyActive(userID string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = ? AND expires_at > ? AND revoked = 0`,
		userID, time.Now().Unix(),
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("count refresh tokens for %s: %w", userID, err)
	}
	return count > 0, nil
}

// hasAnyRecorded reports whether the user has ANY refresh token row recorded
// (active, revoked or expired) — used to distinguish "legacy session that
// predates the store" from "token that was revoked".
func (s *TokenStore) hasAnyRecorded(userID string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = ?`,
		userID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("count refresh tokens for %s: %w", userID, err)
	}
	return count > 0, nil
}
