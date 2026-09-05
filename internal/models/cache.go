package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Cache is the on-disk representation of the synced models.dev catalog.
type Cache struct {
	// Path is the filesystem location of this cache (not serialized).
	Path string `json:"-"`
	// UpdatedAt records when the cache was last synced.
	UpdatedAt time.Time `json:"updated_at"`
	// Models maps model ID (e.g. "deepseek/deepseek-v4-flash") to metadata.
	Models map[string]Model `json:"models"`
}

// NewCache returns an empty cache bound to the given path.
func NewCache(path string) *Cache {
	return &Cache{
		Path:   path,
		Models: make(map[string]Model),
	}
}

// CachePathIn returns the cache path inside the given cosca config home
// directory (e.g. "<home>/.config/cosca"), honoring the COSCA_MODELS_CACHE
// environment override. Callers that need sudo-aware home resolution pass the
// resolved config home themselves.
func CachePathIn(homeDir string) string {
	if p := os.Getenv("COSCA_MODELS_CACHE"); p != "" {
		return p
	}
	return filepath.Join(homeDir, "models.json")
}

// DefaultCachePath returns the default cache location, ~/.config/cosca/
// models.json — the same directory that holds serve.env and config.yaml,
// shared by the binário único `cosca` (chat/exec/mcp).
// COSCA_MODELS_CACHE overrides the location (used by tests and power users).
// Under sudo, prefer CachePathIn with a resolved (non-root) home directory.
func DefaultCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return CachePathIn(filepath.Join(home, ".config", "cosca"))
}

// LoadCache reads the cache at path. A missing or empty file yields an empty
// cache (not an error), so a cold start falls back to hardcoded defaults.
func LoadCache(path string) (*Cache, error) {
	c := NewCache(path)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("read models cache: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return c, nil
	}

	if err := json.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse models cache: %w", err)
	}
	c.Path = path
	if c.Models == nil {
		c.Models = make(map[string]Model)
	}
	return c, nil
}

// SaveCache atomically writes the cache to path with owner-only permissions
// (temp file + rename, 0600 file, 0700 directory).
func SaveCache(path string, c *Cache) error {
	if c == nil {
		return fmt.Errorf("save models cache: nil cache")
	}
	c.Path = path
	if c.Models == nil {
		c.Models = make(map[string]Model)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create models cache directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure models cache directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal models cache: %w", err)
	}
	data = append(data, '\n')

	f, err := os.CreateTemp(dir, ".models-*.tmp")
	if err != nil {
		return fmt.Errorf("create models cache tempfile: %w", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)

	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return fmt.Errorf("secure models cache tempfile: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write models cache tempfile: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync models cache tempfile: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close models cache tempfile: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename models cache tempfile: %w", err)
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// Lookup finds a model by ID. It tries, in order:
//
//  1. Exact match on the full ID (e.g. "deepseek/deepseek-v4-flash").
//  2. Suffix match on the model part, so "deepseek-v4-flash" matches the
//     "deepseek/deepseek-v4-flash" cache entry.
//  3. Case-insensitive match on the model name.
//
// It never errors and returns false when the cache is missing, unreadable, or
// the model is unknown — callers fall back to hardcoded defaults.
func Lookup(cachePath, modelID string) (Model, bool) {
	if modelID == "" {
		return Model{}, false
	}

	c, err := LoadCache(cachePath)
	if err != nil || c == nil || len(c.Models) == 0 {
		return Model{}, false
	}

	query := strings.TrimSpace(modelID)

	// 1. Exact match on the full ID.
	if m, ok := c.Models[query]; ok {
		return m, true
	}

	// 2. Suffix match on the model part after the provider prefix.
	if !strings.Contains(query, "/") {
		var candidates []Model
		suffix := "/" + query
		for id, m := range c.Models {
			if strings.HasSuffix(id, suffix) {
				candidates = append(candidates, m)
			}
		}
		if len(candidates) > 0 {
			return preferCandidate(candidates, query), true
		}
	}

	// 3. Case-insensitive match on the model name.
	for _, m := range c.Models {
		if strings.EqualFold(m.Name, query) {
			return m, true
		}
	}

	return Model{}, false
}

// preferCandidate deterministically picks the best match among models that
// share the same bare name. Preference order:
//
//  1. The provider whose name matches the query's family token
//     ("deepseek-v4-flash" → provider "DeepSeek"), i.e. the origin provider.
//  2. Otherwise the lexicographically smallest ID.
func preferCandidate(candidates []Model, query string) Model {
	family := query
	if i := strings.IndexAny(family, "-:"); i > 0 {
		family = family[:i]
	}
	family = strings.ToLower(family)

	best := candidates[0]
	for _, m := range candidates[1:] {
		mProv := strings.ToLower(m.Provider)
		bestProv := strings.ToLower(best.Provider)
		if mProv == family && bestProv != family {
			best = m
		} else if (mProv == family) == (bestProv == family) && m.ID < best.ID {
			best = m
		}
	}
	return best
}

// ContextWindowFor returns the cached context_length for a model, or 0 when
// the model is unknown or the cache is missing. Callers use the result as the
// effective context window, falling back to their own defaults on 0.
func ContextWindowFor(cachePath, modelID string) int64 {
	m, ok := Lookup(cachePath, modelID)
	if !ok {
		return 0
	}
	return m.ContextLength
}
