// Package cache provides a multi-level caching system for the Cosca Knowledge Engine.
// It supports in-memory LRU cache, SQLite persistent cache, and filesystem cache
// for large objects, automatically falling back through the levels.
package cache

import (
	"container/list"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/rs/zerolog/log"
)

// Entry represents a cached entry with metadata.
type Entry struct {
	Key         string        `json:"key"`
	Value       interface{}   `json:"value"`
	ContentType string        `json:"content_type"`
	Size        int64         `json:"size"`
	CreatedAt   time.Time     `json:"created_at"`
	ExpiresAt   time.Time     `json:"expires_at"`
	TTL         time.Duration `json:"ttl"`
}

// Stats provides cache statistics.
type Stats struct {
	MemoryEntries int   `json:"memory_entries"`
	SQLiteEntries int   `json:"sqlite_entries"`
	FileEntries   int   `json:"file_entries"`
	MemoryHits    int64 `json:"memory_hits"`
	SQLiteHits    int64 `json:"sqlite_hits"`
	FileHits      int64 `json:"file_hits"`
	Misses        int64 `json:"misses"`
	MemorySize    int64 `json:"memory_size_bytes"`
	FileSize      int64 `json:"file_size_bytes"`

	// Error markers: quando a leitura falha, o contador é reportado como 0
	// e o motivo é exposto aqui — permitindo distinguir "cache vazio real"
	// de "cache indisponível" (não-engolimento silencioso).
	SQLiteErr string `json:"sqlite_error,omitempty"`
	FileErr   string `json:"file_error,omitempty"`
}

// Level represents a cache storage level.
type Level int

// Cache storage levels.
const (
	LevelMemory     Level = iota // In-memory LRU cache
	LevelSQLite                  // SQLite persistent cache
	LevelFilesystem              // Filesystem cache for large objects
)

// Config defines the multi-level cache configuration.
type Config struct {
	// MemoryMaxEntries is the maximum number of entries in memory cache (default: 10000).
	MemoryMaxEntries int

	// MemoryTTL is the default TTL for memory cache entries (default: 5 min).
	MemoryTTL time.Duration

	// SQLiteDB is the SQLite database connection for persistent cache.
	SQLiteDB *sql.DB

	// SQLiteTable is the cache table name in SQLite (default: "cache").
	SQLiteTable string

	// FileCacheDir is the directory for filesystem cache.
	FileCacheDir string

	// FileMaxSize is the maximum file size in bytes to store in filesystem cache.
	FileMaxSize int64

	// DefaultTTL is the default TTL for entries (default: 1 hour).
	DefaultTTL time.Duration

	// EnabledLevels specifies which cache levels to use.
	EnabledLevels []Level
}

// DefaultConfig returns sensible cache defaults.
func DefaultConfig() Config {
	return Config{
		MemoryMaxEntries: 10000,
		MemoryTTL:        5 * time.Minute,
		SQLiteTable:      "cache",
		FileMaxSize:      100 * 1024 * 1024, // 100MB
		DefaultTTL:       1 * time.Hour,
		EnabledLevels:    []Level{LevelMemory, LevelSQLite},
	}
}

// Cache is the multi-level cache manager.
type Cache struct {
	cfg Config

	// Memory level
	memMu      sync.RWMutex
	memEntries map[string]*memCacheEntry
	memList    *list.List
	memHits    atomic.Int64

	// SQLite level
	sqlMu       sync.RWMutex
	sqliteDB    *sql.DB
	sqliteTable string
	sqlHits     atomic.Int64

	// Filesystem level
	fsMu   sync.RWMutex
	fsDir  string
	fsHits atomic.Int64

	// Global stats
	misses atomic.Int64
}

type memCacheEntry struct {
	key       string
	value     interface{}
	size      int64
	expiresAt time.Time
	element   *list.Element
}

// New creates a new multi-level cache.
func New(cfg Config) (*Cache, error) {
	if cfg.MemoryMaxEntries <= 0 {
		cfg.MemoryMaxEntries = 10000
	}
	if cfg.MemoryTTL <= 0 {
		cfg.MemoryTTL = 5 * time.Minute
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = 1 * time.Hour
	}
	if cfg.SQLiteTable == "" {
		cfg.SQLiteTable = "cache"
	}

	c := &Cache{
		cfg:         cfg,
		memEntries:  make(map[string]*memCacheEntry),
		memList:     list.New(),
		sqliteDB:    cfg.SQLiteDB,
		sqliteTable: cfg.SQLiteTable,
		fsDir:       cfg.FileCacheDir,
	}

	// Initialize SQLite cache table if DB is available
	if c.sqliteDB != nil {
		if err := c.initSQLiteTable(); err != nil {
			log.Warn().Err(err).Msg("failed to init sqlite cache table")
		}
	}

	// Initialize filesystem cache directory
	if c.fsDir != "" {
		if err := os.MkdirAll(c.fsDir, 0755); err != nil {
			log.Warn().Err(err).Str("dir", c.fsDir).Msg("failed to create file cache directory")
			c.fsDir = ""
		}
	}

	return c, nil
}

// initSQLiteTable ensures the cache table exists.
func (c *Cache) initSQLiteTable() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			key         TEXT PRIMARY KEY,
			value       BLOB NOT NULL,
			content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
			ttl_seconds INTEGER NOT NULL DEFAULT 3600,
			created_at  TEXT NOT NULL DEFAULT (datetime('now')),
			expires_at  TEXT NOT NULL DEFAULT (datetime('now', '+1 hour'))
		)`, c.sqliteTable)
	_, err := c.sqliteDB.Exec(query)
	return err
}

// levelEnabled checks if a cache level is enabled.
func (c *Cache) levelEnabled(level Level) bool {
	for _, l := range c.cfg.EnabledLevels {
		if l == level {
			return true
		}
	}
	return false
}

// Get retrieves a value from the cache, trying memory -> sqlite -> filesystem.
func (c *Cache) Get(key string) (interface{}, bool) {
	// Try memory
	if c.levelEnabled(LevelMemory) {
		if val, ok := c.getMemory(key); ok {
			c.memHits.Add(1)
			return val, true
		}
	}

	// Try SQLite
	if c.levelEnabled(LevelSQLite) && c.sqliteDB != nil {
		if val, ok := c.getSQLite(key); ok {
			c.sqlHits.Add(1)
			// Promote to memory cache
			if c.levelEnabled(LevelMemory) {
				c.setMemory(key, val, c.cfg.MemoryTTL, 0)
			}
			return val, true
		}
	}

	// Try filesystem
	if c.levelEnabled(LevelFilesystem) && c.fsDir != "" {
		if val, ok := c.getFilesystem(key); ok {
			c.fsHits.Add(1)
			return val, true
		}
	}

	c.misses.Add(1)
	return nil, false
}

// Set stores a value in the cache with the given TTL.
func (c *Cache) Set(key string, value interface{}, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.cfg.DefaultTTL
	}

	size := estimateSize(value)

	// Set in all enabled levels
	if c.levelEnabled(LevelMemory) {
		c.setMemory(key, value, ttl, size)
	}

	if c.levelEnabled(LevelSQLite) && c.sqliteDB != nil {
		if err := c.setSQLite(key, value, ttl); err != nil {
			log.Warn().Err(err).Str("key", key).Msg("sqlite cache set failed")
		}
	}

	if c.levelEnabled(LevelFilesystem) && c.fsDir != "" && size <= c.cfg.FileMaxSize {
		if err := c.setFilesystem(key, value); err != nil {
			log.Warn().Err(err).Str("key", key).Msg("file cache set failed")
		}
	}

	return nil
}

// Delete removes a value from all cache levels.
func (c *Cache) Delete(key string) error {
	if c.levelEnabled(LevelMemory) {
		c.deleteMemory(key)
	}

	if c.levelEnabled(LevelSQLite) && c.sqliteDB != nil {
		if err := c.deleteSQLite(key); err != nil {
			log.Warn().Err(err).Str("key", key).Msg("sqlite cache delete failed")
		}
	}

	if c.levelEnabled(LevelFilesystem) && c.fsDir != "" {
		if err := c.deleteFilesystem(key); err != nil {
			log.Warn().Err(err).Str("key", key).Msg("file cache delete failed")
		}
	}

	return nil
}

// Exists checks if a key exists in any cache level.
func (c *Cache) Exists(key string) bool {
	if c.levelEnabled(LevelMemory) {
		c.memMu.RLock()
		entry, ok := c.memEntries[key]
		c.memMu.RUnlock()
		if ok && time.Now().Before(entry.expiresAt) {
			return true
		}
	}

	if c.levelEnabled(LevelSQLite) && c.sqliteDB != nil {
		var count int
		err := c.sqliteDB.QueryRow(
			fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE key = ? AND expires_at > datetime('now')", c.sqliteTable),
			key,
		).Scan(&count)
		if err == nil && count > 0 {
			return true
		}
	}

	if c.levelEnabled(LevelFilesystem) && c.fsDir != "" {
		path := c.fsPath(key)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// Clear removes all entries from all cache levels.
func (c *Cache) Clear() error {
	if c.levelEnabled(LevelMemory) {
		c.clearMemory()
	}

	if c.levelEnabled(LevelSQLite) && c.sqliteDB != nil {
		if err := c.clearSQLite(); err != nil {
			log.Warn().Err(err).Msg("sqlite cache clear failed")
		}
	}

	if c.levelEnabled(LevelFilesystem) && c.fsDir != "" {
		if err := c.clearFilesystem(); err != nil {
			log.Warn().Err(err).Msg("file cache clear failed")
		}
	}

	return nil
}

// Stats returns cache statistics.
func (c *Cache) Stats() Stats {
	stats := Stats{
		MemoryHits: c.memHits.Load(),
		SQLiteHits: c.sqlHits.Load(),
		FileHits:   c.fsHits.Load(),
		Misses:     c.misses.Load(),
	}

	if c.levelEnabled(LevelMemory) {
		c.memMu.RLock()
		stats.MemoryEntries = len(c.memEntries)
		for _, entry := range c.memEntries {
			stats.MemorySize += entry.size
		}
		c.memMu.RUnlock()
	}

	if c.levelEnabled(LevelSQLite) && c.sqliteDB != nil {
		var count int
		if err := c.sqliteDB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", c.sqliteTable)).Scan(&count); err != nil {
			// Falha exposta (não-engolimento): o contador permanece 0 mas o
			// consumidor consegue distinguir "vazio" de "indisponível".
			stats.SQLiteErr = err.Error()
		} else {
			stats.SQLiteEntries = count
		}
	}

	if c.levelEnabled(LevelFilesystem) && c.fsDir != "" {
		entries, err := os.ReadDir(c.fsDir)
		if err != nil {
			stats.FileErr = err.Error()
		} else {
			stats.FileEntries = len(entries)
			var totalSize int64
			for _, entry := range entries {
				if info, err := entry.Info(); err == nil {
					totalSize += info.Size()
				}
			}
			stats.FileSize = totalSize
		}
	}

	return stats
}

// ── Memory Level ───────────────────────────────────────────────────────────

func (c *Cache) getMemory(key string) (interface{}, bool) {
	c.memMu.RLock()
	entry, ok := c.memEntries[key]
	c.memMu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		c.deleteMemory(key)
		return nil, false
	}

	// Move to front (LRU)
	c.memMu.Lock()
	if entry.element != nil {
		c.memList.MoveToFront(entry.element)
	}
	c.memMu.Unlock()

	return entry.value, true
}

func (c *Cache) setMemory(key string, value interface{}, ttl time.Duration, size int64) {
	c.memMu.Lock()
	defer c.memMu.Unlock()

	// Evict if at capacity
	if len(c.memEntries) >= c.cfg.MemoryMaxEntries {
		if elem := c.memList.Back(); elem != nil {
			evictKey := elem.Value.(string)
			delete(c.memEntries, evictKey)
			c.memList.Remove(elem)
		}
	}

	entry := &memCacheEntry{
		key:       key,
		value:     value,
		size:      size,
		expiresAt: time.Now().Add(ttl),
	}

	// Add or update
	if existing, ok := c.memEntries[key]; ok && existing.element != nil {
		c.memList.Remove(existing.element)
	}
	entry.element = c.memList.PushFront(key)
	c.memEntries[key] = entry
}

func (c *Cache) deleteMemory(key string) {
	c.memMu.Lock()
	defer c.memMu.Unlock()

	if entry, ok := c.memEntries[key]; ok {
		if entry.element != nil {
			c.memList.Remove(entry.element)
		}
		delete(c.memEntries, key)
	}
}

func (c *Cache) clearMemory() {
	c.memMu.Lock()
	defer c.memMu.Unlock()
	c.memEntries = make(map[string]*memCacheEntry)
	c.memList = list.New()
}

// ── SQLite Level ───────────────────────────────────────────────────────────

func (c *Cache) getSQLite(key string) (interface{}, bool) {
	c.sqlMu.RLock()
	defer c.sqlMu.RUnlock()

	var value []byte
	var contentType string
	err := c.sqliteDB.QueryRow(
		fmt.Sprintf("SELECT value, content_type FROM %s WHERE key = ? AND expires_at > datetime('now')", c.sqliteTable),
		key,
	).Scan(&value, &contentType)

	if err != nil {
		return nil, false
	}

	// Try to decode JSON
	var decoded interface{}
	if err := json.Unmarshal(value, &decoded); err == nil {
		return decoded, true
	}

	return value, true
}

func (c *Cache) setSQLite(key string, value interface{}, ttl time.Duration) error {
	c.sqlMu.Lock()
	defer c.sqlMu.Unlock()

	data, err := json.Marshal(value)
	if err != nil {
		data = []byte(fmt.Sprintf("%v", value))
	}

	ttlSeconds := int(ttl.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	}

	_, err = c.sqliteDB.Exec(
		fmt.Sprintf(`INSERT OR REPLACE INTO %s (key, value, content_type, ttl_seconds, expires_at)
		 VALUES (?, ?, 'application/json', ?, datetime('now', '+' || ? || ' seconds'))`, c.sqliteTable),
		key, data, ttlSeconds, ttlSeconds,
	)
	return err
}

func (c *Cache) deleteSQLite(key string) error {
	c.sqlMu.Lock()
	defer c.sqlMu.Unlock()

	_, err := c.sqliteDB.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE key = ?", c.sqliteTable), key)
	return err
}

func (c *Cache) clearSQLite() error {
	c.sqlMu.Lock()
	defer c.sqlMu.Unlock()

	_, err := c.sqliteDB.Exec(
		fmt.Sprintf("DELETE FROM %s", c.sqliteTable))
	return err
}

// ── Filesystem Level ───────────────────────────────────────────────────────

func (c *Cache) fsPath(key string) string {
	hash := sha256.Sum256([]byte(key))
	return filepath.Join(c.fsDir, fmt.Sprintf("%x", hash[:16]))
}

func (c *Cache) getFilesystem(key string) (interface{}, bool) {
	c.fsMu.RLock()
	defer c.fsMu.RUnlock()

	path := c.fsPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var decoded interface{}
	if err := json.Unmarshal(data, &decoded); err == nil {
		return decoded, true
	}

	return data, true
}

func (c *Cache) setFilesystem(key string, value interface{}) error {
	c.fsMu.Lock()
	defer c.fsMu.Unlock()

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	path := c.fsPath(key)
	return os.WriteFile(path, data, 0644)
}

func (c *Cache) deleteFilesystem(key string) error {
	c.fsMu.Lock()
	defer c.fsMu.Unlock()

	path := c.fsPath(key)
	return os.Remove(path)
}

func (c *Cache) clearFilesystem() error {
	c.fsMu.Lock()
	defer c.fsMu.Unlock()

	entries, err := os.ReadDir(c.fsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		safe.Remove(filepath.Join(c.fsDir, entry.Name()))
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────

// estimateSize approximates the memory size of a value.
func estimateSize(v interface{}) int64 {
	if data, err := json.Marshal(v); err == nil {
		return int64(len(data))
	}
	return 1024 // default estimate
}

// Key hashes a string for use as a cache key.
func Key(parts ...string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s", parts)))
	return fmt.Sprintf("%x", h[:16])
}
