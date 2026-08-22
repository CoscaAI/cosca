package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// User represents a registered user in the system.
type User struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	PasswordHash   string `json:"password_hash,omitempty"` // bcrypt hash
	Role           string `json:"role"`                    // admin, editor, viewer
	Email          string `json:"email,omitempty"`
	CreatedAt      string `json:"created_at"`
	FailedAttempts int    `json:"failed_attempts"`
	LockedUntil    string `json:"locked_until"` // RFC3339
	// MustChangePassword is true for accounts whose password was provisioned
	// automatically (e.g. the dev-mode admin). The login endpoint answers
	// 403 password_change_required until the user sets their own password.
	MustChangePassword bool `json:"must_change_password"`
}

// cloneUser returns a defensive copy of a user so callers cannot mutate the
// store's internal records. All User fields are value types, so a struct copy
// fully isolates the returned value from the store's maps.
func cloneUser(u *User) *User {
	if u == nil {
		return nil
	}
	cp := *u
	return &cp
}

// UserStoreConfig holds configuration for the UserStore.
type UserStoreConfig struct {
	DB *sql.DB // SQLite database (nil = in-memory only)
}

// Common user store errors.
var (
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidRole        = errors.New("invalid role")
)

// validRoles contains the set of allowed role values.
var validRoles = map[string]bool{
	"admin":  true,
	"editor": true,
	"viewer": true,
}

// UserStore is a thread-safe user store backed by SQLite.
type UserStore struct {
	users      map[string]*User // cache keyed by ID
	byUsername map[string]*User // cache keyed by username
	mu         sync.RWMutex
	db         *sql.DB
}

// NewUserStore creates a new UserStore backed by SQLite.
// If cfg.DB is nil, operates in-memory only (useful for tests).
// On first run with no users and COSCA_DEV_MODE=true, creates an admin
// account with a RANDOM password (never the hardcoded "admin"), prints the
// password once to stderr, and forces a password change on first login so
// the credential does not linger after DEV_MODE is turned off.
func NewUserStore(cfg UserStoreConfig) *UserStore {
	s := &UserStore{
		users:      make(map[string]*User),
		byUsername: make(map[string]*User),
		db:         cfg.DB,
	}

	if s.db != nil {
		if err := s.initSchema(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: failed to init users schema: %v\n", err)
		}
		if err := s.loadFromDB(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: failed to load users from db: %v\n", err)
		}
	}

	// If no users exist, create default admin in dev mode.
	if len(s.users) == 0 {
		if os.Getenv("COSCA_DEV_MODE") == "true" {
			// 16 random bytes → 32 hex chars. Never "admin".
			adminPassword, pwErr := generateSecret(16)
			if pwErr != nil {
				fmt.Fprintf(os.Stderr, "ERROR: failed to generate dev admin password: %v\n", pwErr)
			} else if user, createErr := s.Create("admin", adminPassword, "admin", ""); createErr != nil {
				fmt.Fprintf(os.Stderr, "ERROR: failed to create dev admin: %v\n", createErr)
			} else {
				// Force a password change so the printed credential is
				// retired immediately on first login.
				if err := s.RequirePasswordChange(user.ID); err != nil {
					fmt.Fprintf(os.Stderr, "ERROR: failed to flag dev admin password change: %v\n", err)
				}
				fmt.Fprintf(os.Stderr, "COSCA_DEV_MODE: created admin user with random password: %s (change immediately)\n", adminPassword)
			}
		} else {
			fmt.Fprintf(os.Stderr, "WARNING: No users found. Start with COSCA_DEV_MODE=true or register via /v1/auth/register\n")
		}
	}

	return s
}

// initSchema creates the users table if it does not exist.
func (s *UserStore) initSchema() error {
	if s.db == nil {
		return nil
	}
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id                   TEXT PRIMARY KEY,
			username             TEXT NOT NULL UNIQUE,
			password_hash        TEXT NOT NULL,
			role                 TEXT NOT NULL DEFAULT 'viewer',
			email                TEXT NOT NULL DEFAULT '',
			created_at           TEXT NOT NULL,
			failed_attempts      INTEGER NOT NULL DEFAULT 0,
			locked_until         TEXT NOT NULL DEFAULT '',
			must_change_password INTEGER NOT NULL DEFAULT 0
		)
	`); err != nil {
		return err
	}
	// Migration for databases created before the must_change_password column
	// existed. Duplicate column errors are ignored (same pattern as the
	// memory engine's owner column migration).
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 0`)
	return nil
}

// loadFromDB loads all users from SQLite into the in-memory cache.
func (s *UserStore) loadFromDB() error {
	if s.db == nil {
		return nil
	}
	rows, err := s.db.Query(`SELECT id, username, password_hash, role, email, created_at, failed_attempts, locked_until, must_change_password FROM users`)
	if err != nil {
		return fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		u := &User{}
		var mustChange int
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Email, &u.CreatedAt, &u.FailedAttempts, &u.LockedUntil, &mustChange); err != nil {
			return fmt.Errorf("scan user: %w", err)
		}
		u.MustChangePassword = mustChange != 0
		s.users[u.ID] = u
		s.byUsername[u.Username] = u
	}
	return rows.Err()
}

// persistInsert inserts a user row into SQLite.
func (s *UserStore) persistInsert(u *User) error {
	if s.db == nil {
		return nil
	}
	mustChange := 0
	if u.MustChangePassword {
		mustChange = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO users (id, username, password_hash, role, email, created_at, failed_attempts, locked_until, must_change_password) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.PasswordHash, u.Role, u.Email, u.CreatedAt, u.FailedAttempts, u.LockedUntil, mustChange,
	)
	return err
}

// persistUpdate updates a user row in SQLite by ID.
func (s *UserStore) persistUpdate(u *User) error {
	if s.db == nil {
		return nil
	}
	mustChange := 0
	if u.MustChangePassword {
		mustChange = 1
	}
	_, err := s.db.Exec(
		`UPDATE users SET password_hash=?, role=?, email=?, failed_attempts=?, locked_until=?, must_change_password=? WHERE id=?`,
		u.PasswordHash, u.Role, u.Email, u.FailedAttempts, u.LockedUntil, mustChange, u.ID,
	)
	return err
}

// persistDelete removes a user row from SQLite by ID.
func (s *UserStore) persistDelete(id string) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}

// hashPassword computes a bcrypt hash of the password with cost 12.
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(bytes), nil
}

// Create registers a new user. Returns ErrUserExists if username is taken.
func (s *UserStore) Create(username, password, role, email string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byUsername[username]; exists {
		return nil, ErrUserExists
	}
	if !validRoles[role] {
		return nil, fmt.Errorf("%w: %s (must be admin, editor, or viewer)", ErrInvalidRole, role)
	}

	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	user := &User{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		Email:        email,
		CreatedAt:    now,
	}

	if err := s.persistInsert(user); err != nil {
		return nil, fmt.Errorf("persist user: %w", err)
	}

	s.users[user.ID] = user
	s.byUsername[user.Username] = user

	return cloneUser(user), nil
}

// GetByID returns a user by ID, or ErrUserNotFound.
func (s *UserStore) GetByID(id string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return cloneUser(user), nil
}

// GetByUsername returns a user by username, or ErrUserNotFound.
func (s *UserStore) GetByUsername(username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.byUsername[username]
	if !ok {
		return nil, ErrUserNotFound
	}
	return cloneUser(user), nil
}

// Authenticate verifies username/password. After 5 consecutive failures,
// the account is locked for 15 minutes.
func (s *UserStore) Authenticate(username, password string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.byUsername[username]
	if !ok {
		return nil, ErrInvalidCredentials
	}

	// Check lock.
	if user.LockedUntil != "" {
		lockedUntil, err := time.Parse(time.RFC3339, user.LockedUntil)
		if err != nil {
			return nil, fmt.Errorf("account locked until %s (unreadable lock state)", user.LockedUntil)
		}
		if time.Now().UTC().Before(lockedUntil) {
			return nil, fmt.Errorf("account locked until %s", user.LockedUntil)
		}
		user.FailedAttempts = 0
		user.LockedUntil = ""
		if err := s.persistUpdate(user); err != nil {
			return nil, fmt.Errorf("persist lock reset: %w", err)
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		user.FailedAttempts++
		if user.FailedAttempts >= 5 {
			user.LockedUntil = time.Now().UTC().Add(15 * time.Minute).Format(time.RFC3339)
		}
		if err := s.persistUpdate(user); err != nil {
			return nil, fmt.Errorf("persist failed attempts: %w", err)
		}
		return nil, ErrInvalidCredentials
	}

	// Successful login — reset counter.
	if user.FailedAttempts > 0 {
		user.FailedAttempts = 0
		user.LockedUntil = ""
		if err := s.persistUpdate(user); err != nil {
			return nil, fmt.Errorf("persist reset attempts: %w", err)
		}
	}

	return cloneUser(user), nil
}

// List returns all users (password hash redacted).
func (s *UserStore) List() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]User, 0, len(s.users))
	for _, u := range s.users {
		user := *u
		user.PasswordHash = ""
		result = append(result, user)
	}
	return result
}

// Delete removes a user by ID. Returns ErrUserNotFound if not found.
func (s *UserStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[id]
	if !ok {
		return ErrUserNotFound
	}

	delete(s.users, id)
	delete(s.byUsername, user.Username)
	if err := s.persistDelete(id); err != nil {
		return fmt.Errorf("persist delete: %w", err)
	}

	return nil
}

// UpdateRole changes the role of a user by ID.
func (s *UserStore) UpdateRole(id, role string) error {
	if !validRoles[role] {
		return fmt.Errorf("%w: %s (must be admin, editor, or viewer)", ErrInvalidRole, role)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[id]
	if !ok {
		return ErrUserNotFound
	}

	user.Role = role
	if err := s.persistUpdate(user); err != nil {
		return fmt.Errorf("persist role: %w", err)
	}

	return nil
}

// RequirePasswordChange marks a user as needing a password change on their
// next login (used for the dev-mode admin account and any other provisioned
// credential). While the flag is set, the login endpoint answers 403
// password_change_required.
func (s *UserStore) RequirePasswordChange(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return ErrUserNotFound
	}

	user.MustChangePassword = true
	return s.persistUpdate(user)
}

// ClearPasswordChange clears the must_change_password flag after the user has
// successfully changed their password, so future logins succeed normally.
func (s *UserStore) ClearPasswordChange(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return ErrUserNotFound
	}

	user.MustChangePassword = false
	return s.persistUpdate(user)
}

// ChangePassword validates the user's current password and replaces it with a
// new one. On success the must_change_password flag is cleared and any failed
// attempt counter / lockout is reset. Returns ErrUserNotFound if the user does
// not exist and ErrInvalidCredentials if the current password is wrong.
func (s *UserStore) ChangePassword(userID, currentPassword, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return ErrUserNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}

	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = hash
	user.MustChangePassword = false
	user.FailedAttempts = 0
	user.LockedUntil = ""
	return s.persistUpdate(user)
}
