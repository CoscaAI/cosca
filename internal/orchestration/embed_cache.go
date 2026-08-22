package orchestration

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ─── EmbedCacheConfig ─────────────────────────────────────────────────────────

// EmbedCacheConfig configures the embedding response cache.
type EmbedCacheConfig struct {
	// Enabled enables/disables the cache.
	Enabled bool

	// MaxSize is the maximum number of cached entries.
	MaxSize int // default: 1000

	// TTL is how long entries remain valid.
	TTL time.Duration // default: 1h

	// SimilarityThreshold: if two prompts have high similarity,
	// reuse the cached result. 0 = exact match only.
	// Range: 0.0 - 1.0, default: 0.0 (exact match only)
	SimilarityThreshold float64
}

// DefaultEmbedCacheConfig returns sensible defaults for the embedding cache.
func DefaultEmbedCacheConfig() EmbedCacheConfig {
	return EmbedCacheConfig{
		Enabled:             true,
		MaxSize:             1000,
		TTL:                 1 * time.Hour,
		SimilarityThreshold: 0.0,
	}
}

// ─── EmbedCache ───────────────────────────────────────────────────────────────

// EmbedCache caches knowledge search results keyed by prompt hash.
type EmbedCache struct {
	config  EmbedCacheConfig
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	order   []string // insertion order, oldest first
	hits    int64
	misses  int64
}

// cacheEntry holds a single cached knowledge search result.
type cacheEntry struct {
	results   *KnowledgeSearchResults
	prompt    string
	createdAt time.Time
	expiresAt time.Time
}

// cloneKnowledgeSearchResults prevents callers from observing or mutating the
// cache's stored result graph. In particular, context filtering must not alter
// a value that can be returned to another concurrent request.
func cloneKnowledgeSearchResults(results *KnowledgeSearchResults) *KnowledgeSearchResults {
	if results == nil {
		return nil
	}
	cp := *results
	if results.Results != nil {
		cp.Results = make([]KnowledgeSearchResult, len(results.Results))
		copy(cp.Results, results.Results)
	}
	return &cp
}

// NewEmbedCache creates a new embedding cache with the given configuration.
func NewEmbedCache(config EmbedCacheConfig) *EmbedCache {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}
	if config.TTL <= 0 {
		config.TTL = 1 * time.Hour
	}
	return &EmbedCache{
		config:  config,
		entries: make(map[string]*cacheEntry),
	}
}

// Get returns cached results for a prompt if available and not expired.
// Returns nil, false on cache miss.
func (c *EmbedCache) Get(prompt string) (*KnowledgeSearchResults, bool) {
	if !c.config.Enabled {
		return nil, false
	}

	key := c.cacheKey(prompt)

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Check expiry. The entry pointer is immutable, but it may be replaced
	// between this read lock and the eviction lock below.
	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		if current, exists := c.entries[key]; exists && current == entry && time.Now().After(current.expiresAt) {
			delete(c.entries, key)
			c.removeFromOrder(key)
		}
		c.misses++
		c.mu.Unlock()
		log.Debug().Str("prompt_hash", c.cacheKey(prompt)).Int("prompt_length", len(prompt)).Msg("embed cache: entry expired, evicted")
		return nil, false
	}

	c.mu.Lock()
	c.hits++
	c.mu.Unlock()

	return cloneKnowledgeSearchResults(entry.results), true
}

// Set stores results for a prompt in the cache. If the cache is at capacity,
// the oldest entry (by createdAt) is evicted before the new entry is stored.
func (c *EmbedCache) Set(prompt string, results *KnowledgeSearchResults) {
	if !c.config.Enabled {
		return
	}
	if results == nil {
		return
	}

	key := c.cacheKey(prompt)
	now := time.Now()

	entry := &cacheEntry{
		results:   cloneKnowledgeSearchResults(results),
		prompt:    prompt,
		createdAt: now,
		expiresAt: now.Add(c.config.TTL),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// If already present, update in-place (no eviction needed).
	if _, exists := c.entries[key]; exists {
		c.entries[key] = entry
		return
	}

	// Check capacity: evict oldest entry if at MaxSize.
	if len(c.entries) >= c.config.MaxSize {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
		log.Debug().Str("key", oldest).Msg("embed cache: evicted oldest entry (LRU)")
	}

	c.entries[key] = entry
	c.order = append(c.order, key)
}

// Invalidate removes a specific prompt from the cache.
func (c *EmbedCache) Invalidate(prompt string) {
	key := c.cacheKey(prompt)

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
	c.removeFromOrder(key)
}

// Clear removes all entries from the cache.
func (c *EmbedCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
	c.order = nil
	c.hits = 0
	c.misses = 0
}

// Stats returns current cache statistics.
func (c *EmbedCache) Stats() EmbedCacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return EmbedCacheStats{
		Size:    len(c.entries),
		MaxSize: c.config.MaxSize,
		Hits:    c.hits,
		Misses:  c.misses,
		HitRate: hitRate,
	}
}

// ─── EmbedCacheStats ──────────────────────────────────────────────────────────

// EmbedCacheStats is a snapshot of cache statistics.
type EmbedCacheStats struct {
	Size    int     `json:"size"`
	MaxSize int     `json:"max_size"`
	Hits    int64   `json:"hits"`
	Misses  int64   `json:"misses"`
	HitRate float64 `json:"hit_rate"`
}

// ─── Internal Helpers ─────────────────────────────────────────────────────────

// cacheKey computes a deterministic hash for a prompt. The prompt is
// lowercased and trimmed so that minor whitespace/case variations
// produce the same key (exact-match behaviour).
func (c *EmbedCache) cacheKey(prompt string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(prompt))))
	return fmt.Sprintf("%x", h[:16])
}

// removeFromOrder removes a key from the order slice.
// Must be called while c.mu is held (write lock).
// O(n) but called infrequently (only on expiry/invalidation).
func (c *EmbedCache) removeFromOrder(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}
