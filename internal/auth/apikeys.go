package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─── APIKey ──────────────────────────────────────────────────────────────────

// APIKey represents a stored API key in the system. The plaintext secret is
// never stored — only its SHA-256 hash is persisted after generation.
// The full key is returned once at creation time and then discarded.
type APIKey struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Prefix     string `json:"prefix"` // First 12 chars for UI display: "cosca_sk_AbCd..."
	Hash       string `json:"hash"`   // SHA-256 hash of the full key (persisted for validation)
	Role       string `json:"role"`   // RBAC role for requests authenticated with this key
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	Status     string `json:"status"` // "active" | "revoked"
	CreatedBy  string `json:"created_by"`
}

// ─── Errors ──────────────────────────────────────────────────────────────────

// ErrAPIKeyNotFound is returned when an API key ID is not found in the store.
var (
	ErrAPIKeyNotFound = errors.New("api key not found")
	ErrAPIKeyInvalid  = errors.New("invalid api key")
	ErrAPIKeyExpired  = errors.New("api key expired")
	ErrAPIKeyRevoked  = errors.New("api key revoked")
)

// ─── APIKeyStore ─────────────────────────────────────────────────────────────

// APIKeyStoreConfig holds configuration options for the APIKeyStore.
type APIKeyStoreConfig struct {
	DataDir string // Directory for api-keys.json (empty = in-memory only)
}

// APIKeyStore is a thread-safe store for API keys with optional disk persistence.
type APIKeyStore struct {
	keys     map[string]*APIKey // keyed by ID
	byPrefix map[string]*APIKey // keyed by prefix (first 12 chars) for fast lookup
	byHash   map[string]*APIKey // keyed by SHA-256 hash for validation
	mu       sync.RWMutex
	config   APIKeyStoreConfig
}

// NewAPIKeyStore creates a new APIKeyStore. If the configured data directory
// contains an api-keys.json file, keys are restored from it. When DataDir is
// empty, the store operates in-memory only (useful for tests).
func NewAPIKeyStore(cfg APIKeyStoreConfig) *APIKeyStore {
	s := &APIKeyStore{
		keys:     make(map[string]*APIKey),
		byPrefix: make(map[string]*APIKey),
		byHash:   make(map[string]*APIKey),
		config:   cfg,
	}

	// Restore persisted keys if available.
	// Log errors but proceed — this is non-fatal at startup.
	if err := s.load(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to load api-keys from %s: %v\n", cfg.DataDir, err)
	}

	return s
}

// ─── Key Generation ──────────────────────────────────────────────────────────

// generateSecret generates a random hex string of the given byte length.
func generateSecret(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// hashKey computes the SHA-256 hash of a key string and returns the hex
// representation.
func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// Generate creates a new API key. It returns the ApiKey record (without the
// secret) and the full plaintext key that must be shown to the user once.
//
// The caller is responsible for immediately displaying the plaintext key to
// the user — it is not recoverable afterwards.
//
// The role parameter determines the RBAC role granted to requests
// authenticated with this key (e.g. "admin", "editor").
func (s *APIKeyStore) Generate(name, createdBy, role string, expiresIn time.Duration) (*APIKey, string, error) {
	// Generate the random part of the key (32 bytes → 64 hex chars).
	random, err := generateSecret(32)
	if err != nil {
		return nil, "", err
	}

	// Build the full key.
	fullKey := "cosca_sk_" + random

	// Build the prefix for UI display: first 12 chars + "...".
	prefix := fullKey
	if len(fullKey) > 12 {
		prefix = fullKey[:12] + "..."
	}

	// Hash the full key for storage.
	keyHash := hashKey(fullKey)

	now := time.Now().UTC().Format(time.RFC3339)

	apiKey := &APIKey{
		ID:        uuid.New().String(),
		Name:      name,
		Prefix:    prefix,
		Hash:      keyHash,
		Role:      role,
		CreatedAt: now,
		Status:    "active",
		CreatedBy: createdBy,
	}

	if expiresIn > 0 {
		exp := time.Now().UTC().Add(expiresIn).Format(time.RFC3339)
		apiKey.ExpiresAt = exp
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate prefix (extremely unlikely but defensive).
	if _, exists := s.byPrefix[apiKey.Prefix]; exists {
		return nil, "", fmt.Errorf("prefix collision — please try again")
	}

	s.keys[apiKey.ID] = apiKey
	s.byPrefix[apiKey.Prefix] = apiKey
	s.byHash[apiKey.Hash] = apiKey

	// Persist after mutation.
	if err := s.save(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to save api-keys after generate: %v\n", err)
	}

	return apiKey, fullKey, nil
}

// ─── Validation ──────────────────────────────────────────────────────────────

// Validate checks whether a given plaintext key is valid, active, and
// not expired. On success it returns the associated APIKey and updates
// LastUsedAt. On failure it returns ErrAPIKeyInvalid / ErrAPIKeyExpired /
// ErrAPIKeyRevoked.
func (s *APIKeyStore) Validate(key string) (*APIKey, error) {
	if !strings.HasPrefix(key, "cosca_sk_") {
		return nil, ErrAPIKeyInvalid
	}

	keyHash := hashKey(key)

	s.mu.Lock()
	defer s.mu.Unlock()

	ak, ok := s.byHash[keyHash]
	if !ok {
		return nil, ErrAPIKeyInvalid
	}

	if ak.Status == "revoked" {
		return nil, ErrAPIKeyRevoked
	}

	if ak.ExpiresAt != "" {
		exp, err := time.Parse(time.RFC3339, ak.ExpiresAt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: api key %s has invalid expires_at format %q: %v\n", ak.ID, ak.ExpiresAt, err)
			return nil, ErrAPIKeyExpired
		}
		if time.Now().UTC().After(exp) {
			return nil, ErrAPIKeyExpired
		}
	}

	// Update last used timestamp.
	ak.LastUsedAt = time.Now().UTC().Format(time.RFC3339)

	return ak, nil
}

// ─── CRUD ────────────────────────────────────────────────────────────────────

// List returns all API keys (without the hash). The returned
// slice is a copy so callers cannot mutate the internal map.
func (s *APIKeyStore) List() []APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]APIKey, 0, len(s.keys))
	for _, ak := range s.keys {
		copy := *ak
		// Never expose the hash via the REST API.
		copy.Hash = ""
		result = append(result, copy)
	}
	return result
}

// GetByID returns a single API key by its ID (without the hash),
// or ErrApiKeyNotFound.
func (s *APIKeyStore) GetByID(id string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ak, ok := s.keys[id]
	if !ok {
		return nil, ErrAPIKeyNotFound
	}
	// Return a copy without the hash.
	copy := *ak
	copy.Hash = ""
	return &copy, nil
}

// Revoke marks an API key as revoked. The key remains in the store for
// audit purposes but will fail validation thereafter.
func (s *APIKeyStore) Revoke(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ak, ok := s.keys[id]
	if !ok {
		return ErrAPIKeyNotFound
	}

	ak.Status = "revoked"

	// Persist after mutation.
	if err := s.save(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to save api-keys after revoke: %v\n", err)
	}

	return nil
}

// ─── Persistence ─────────────────────────────────────────────────────────────

// load restores API keys from api-keys.json. If the file does not exist it is
// not considered an error (first-run scenario).
func (s *APIKeyStore) load() error {
	if s.config.DataDir == "" {
		return nil
	}

	path := filepath.Join(s.config.DataDir, "api-keys.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read api-keys file: %w", err)
	}

	var keys []*APIKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("unmarshal api-keys: %w", err)
	}

	for _, ak := range keys {
		s.keys[ak.ID] = ak
		s.byPrefix[ak.Prefix] = ak
		if ak.Hash != "" {
			s.byHash[ak.Hash] = ak
		}
	}

	return nil
}

// save persists all API keys to api-keys.json using an atomic write (temp file
// then rename) to avoid corruption. The hash is included for validation on
// next startup, but is stripped from REST API responses in List/GetByID.
func (s *APIKeyStore) save() error {
	if s.config.DataDir == "" {
		return nil
	}

	path := filepath.Join(s.config.DataDir, "api-keys.json")

	keys := make([]*APIKey, 0, len(s.keys))
	for _, ak := range s.keys {
		keys = append(keys, ak)
	}

	payload, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal api-keys: %w", err)
	}

	// Ensure the data directory exists.
	if err := os.MkdirAll(s.config.DataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// Atomic write: write to a temp file, then rename.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		// Best-effort cleanup of the temp file on failure.
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
