// Package secrets provides an encrypted key-value vault for storing
// sensitive configuration values such as API tokens, passwords, and
// certificates. Values are encrypted at rest using AES-256-GCM with a
// key derived from COSCA_JWT_SECRET via HKDF-SHA256.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/privfile"
	_ "modernc.org/sqlite" // Import SQLite driver for secrets vault storage
)

// Secret represents a stored secret in the vault.
type Secret struct {
	Key       string `json:"key"`
	Type      string `json:"type"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	CreatedBy string `json:"created_by"`
	// Value is never serialized to JSON — it is only returned via the Get method.
}

// Common vault errors.
var (
	ErrSecretNotFound = errors.New("secret not found")
	ErrEmptyKey       = errors.New("secret key must not be empty")
	ErrNotInitialized = errors.New("secrets vault not initialized")
)

// deriveKey derives a 256-bit AES key from the master secret using HKDF-SHA256.
// The salt is a fixed string to ensure deterministic key derivation across
// restarts while using HKDF's info parameter for domain separation.
func deriveKey(masterSecret []byte) ([]byte, error) {
	key, err := hkdf.Key(sha256.New, masterSecret, nil, "cosca-secrets-vault-v1", 32)
	if err != nil {
		return nil, fmt.Errorf("hkdf key derivation: %w", err)
	}
	return key, nil
}

// Vault is a thread-safe, SQLite-backed encrypted secrets store. Values are
// encrypted with AES-256-GCM before being persisted and decrypted on retrieval.
type Vault struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
	aesKey []byte
}

// New creates or opens the secrets vault at the given database path. The
// master secret (typically COSCA_JWT_SECRET) is used to derive the AES-256
// encryption key via HKDF-SHA256.
func New(dbPath string, masterSecret []byte) (*Vault, error) {
	if len(masterSecret) < 32 {
		return nil, fmt.Errorf("master secret must be at least 32 bytes for AES-256 key derivation")
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create vault db directory: %w", err)
	}

	// The vault holds encrypted secrets — the file must never be world or
	// group readable. SQLite creates files with the process umask (typically
	// 0644); force 0600 up front and repair legacy files.
	if err := privfile.EnsurePrivateDBFile(dbPath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open vault database: %w", err)
	}

	// Configure SQLite.
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

	// Derive the AES key.
	aesKey, err := deriveKey(masterSecret)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("derive encryption key: %w", err)
	}

	v := &Vault{db: db, dbPath: dbPath, aesKey: aesKey}
	if err := v.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate vault database: %w", err)
	}

	return v, nil
}

// migrate creates the secrets_vault table if it does not exist.
func (v *Vault) migrate() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS secrets_vault (
		key        TEXT PRIMARY KEY,
		value      BLOB NOT NULL,
		type       TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		created_by TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_secrets_created_at ON secrets_vault(created_at);
	`
	_, err := v.db.Exec(ddl)
	return err
}

// encrypt encrypts plaintext using AES-256-GCM with a random 12-byte nonce.
// The returned ciphertext is: nonce (12 bytes) || ciphertext || auth tag (16 bytes).
func (v *Vault) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.aesKey)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// GCM seals and appends the auth tag. The result is nonce || ciphertext+tag.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decrypt decrypts ciphertext produced by encrypt. The input is expected to be
// nonce (12 bytes) || ciphertext || auth tag (16 bytes).
func (v *Vault) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.aesKey)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short (%d bytes, need at least %d)", len(ciphertext), nonceSize)
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// Set stores or updates a secret value. The value is encrypted with
// AES-256-GCM before persistence. If the key already exists, it is updated
// (and updated_at is refreshed).
func (v *Vault) Set(key string, value string, typ string, createdBy string) error {
	if key == "" {
		return ErrEmptyKey
	}

	encrypted, err := v.encrypt([]byte(value))
	if err != nil {
		return fmt.Errorf("encrypt value: %w", err)
	}

	now := time.Now().Unix()

	v.mu.Lock()
	defer v.mu.Unlock()

	// Use INSERT OR REPLACE for upsert semantics. When replacing, we preserve
	// the original created_at unless this is a brand new key.
	var createdAt int64
	existing, _ := v.getLocked(key)
	if existing != nil {
		createdAt = existing.CreatedAt
	} else {
		createdAt = now
	}

	if typ == "" && existing != nil {
		typ = existing.Type
	}

	if createdBy == "" && existing != nil {
		createdBy = existing.CreatedBy
	}

	_, err = v.db.Exec(
		`INSERT OR REPLACE INTO secrets_vault (key, value, type, created_at, updated_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		key, encrypted, typ, createdAt, now, createdBy,
	)
	if err != nil {
		return fmt.Errorf("store secret: %w", err)
	}

	return nil
}

// getLocked retrieves a secret without holding a new lock (caller must hold
// the lock). Only the metadata is returned — the encrypted value is not
// decrypted here.
func (v *Vault) getLocked(key string) (*Secret, error) {
	if v.db == nil {
		return nil, ErrNotInitialized
	}
	var s Secret
	var encrypted []byte
	err := v.db.QueryRow(
		"SELECT key, value, type, created_at, updated_at, created_by FROM secrets_vault WHERE key = ?",
		key,
	).Scan(&s.Key, &encrypted, &s.Type, &s.CreatedAt, &s.UpdatedAt, &s.CreatedBy)

	if err == sql.ErrNoRows {
		return nil, ErrSecretNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query secret: %w", err)
	}

	return &s, nil
}

// Get retrieves and decrypts a secret value. Returns the Secret metadata
// and the decrypted value as a string.
func (v *Vault) Get(key string) (*Secret, string, error) {
	if key == "" {
		return nil, "", ErrEmptyKey
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.db == nil {
		return nil, "", ErrNotInitialized
	}

	var s Secret
	var encrypted []byte
	err := v.db.QueryRow(
		"SELECT key, value, type, created_at, updated_at, created_by FROM secrets_vault WHERE key = ?",
		key,
	).Scan(&s.Key, &encrypted, &s.Type, &s.CreatedAt, &s.UpdatedAt, &s.CreatedBy)

	if err == sql.ErrNoRows {
		return nil, "", ErrSecretNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("query secret: %w", err)
	}

	plaintext, err := v.decrypt(encrypted)
	if err != nil {
		return nil, "", fmt.Errorf("decrypt secret value: %w", err)
	}

	return &s, string(plaintext), nil
}

// Delete removes a secret by key. Returns ErrSecretNotFound if the key does
// not exist.
func (v *Vault) Delete(key string) error {
	if key == "" {
		return ErrEmptyKey
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return ErrNotInitialized
	}

	result, err := v.db.Exec("DELETE FROM secrets_vault WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result: %w", err)
	}
	if rows == 0 {
		return ErrSecretNotFound
	}

	return nil
}

// StoredKey represents a secret key visible in listings (no value exposed).
type StoredKey struct {
	Key       string `json:"key"`
	Type      string `json:"type"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	CreatedBy string `json:"created_by"`
}

// List returns all stored secret keys with their metadata. Values are never
// exposed through this method.
func (v *Vault) List() ([]StoredKey, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	rows, err := v.db.Query(
		"SELECT key, type, created_at, updated_at, created_by FROM secrets_vault ORDER BY key",
	)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	defer rows.Close()

	keys := make([]StoredKey, 0)
	for rows.Next() {
		var sk StoredKey
		if err := rows.Scan(&sk.Key, &sk.Type, &sk.CreatedAt, &sk.UpdatedAt, &sk.CreatedBy); err != nil {
			return nil, fmt.Errorf("scan secret key: %w", err)
		}
		keys = append(keys, sk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate secrets: %w", err)
	}

	if keys == nil {
		keys = make([]StoredKey, 0)
	}

	return keys, nil
}

// Close cleanly shuts down the database connection.
func (v *Vault) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.db != nil {
		_, _ = v.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		return v.db.Close()
	}
	return nil
}

// EncryptForExport encrypts a value with the vault key and returns a hex string
// suitable for export or external storage.
func (v *Vault) EncryptForExport(plaintext string) (string, error) {
	encrypted, err := v.encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(encrypted), nil
}

// DecryptFromImport decrypts a hex-encoded ciphertext previously exported with
// EncryptForExport.
func (v *Vault) DecryptFromImport(hexCiphertext string) (string, error) {
	ciphertext, err := hex.DecodeString(hexCiphertext)
	if err != nil {
		return "", fmt.Errorf("hex decode: %w", err)
	}
	plaintext, err := v.decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
