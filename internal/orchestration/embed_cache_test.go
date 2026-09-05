package orchestration

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ─── Helper ───────────────────────────────────────────────────────────────────

func newTestEmbedCache(maxSize int, ttl time.Duration) *EmbedCache {
	return NewEmbedCache(EmbedCacheConfig{
		Enabled: true,
		MaxSize: maxSize,
		TTL:     ttl,
	})
}

func makeResults(query string, count int) *KnowledgeSearchResults {
	results := make([]KnowledgeSearchResult, count)
	for i := 0; i < count; i++ {
		results[i] = KnowledgeSearchResult{
			ID:      "r1",
			Title:   "Test Title",
			Snippet: "Test snippet",
			Score:   0.95,
		}
	}
	return &KnowledgeSearchResults{
		Results:    results,
		TotalCount: count,
		Query:      query,
	}
}

// ─── Test: Basic Get/Set ──────────────────────────────────────────────────────

func TestEmbedCache_GetSet(t *testing.T) {
	cache := newTestEmbedCache(10, 1*time.Hour)

	// Cache miss.
	_, ok := cache.Get("build a REST api endpoint")
	if ok {
		t.Error("expected cache miss, got hit")
	}

	// Set and get.
	want := makeResults("build a REST api endpoint", 3)
	cache.Set("build a REST api endpoint", want)

	got, ok := cache.Get("build a REST api endpoint")
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if got == nil {
		t.Fatal("expected non-nil results")
	}
	if len(got.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(got.Results))
	}
	// Verify stats.
	stats := cache.Stats()
	if stats.Size != 1 {
		t.Errorf("expected size 1, got %d", stats.Size)
	}
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
}

func TestEmbedCache_GetReturnsIndependentResults(t *testing.T) {
	cache := newTestEmbedCache(10, time.Hour)
	original := makeResults("immutable", 1)
	original.Results[0].Content = "original"
	cache.Set("immutable", original)

	got, ok := cache.Get("immutable")
	if !ok {
		t.Fatal("expected cache hit")
	}
	got.Results[0].Content = "caller mutation"
	got.Results = got.Results[:0]

	again, ok := cache.Get("immutable")
	if !ok || len(again.Results) != 1 || again.Results[0].Content != "original" {
		t.Fatalf("cache result was mutable through Get: %+v, hit=%v", again, ok)
	}

	// Set also takes ownership of a snapshot, so later producer mutation is
	// unable to alter the value shared with other requests.
	original.Results[0].Content = "producer mutation"
	again, _ = cache.Get("immutable")
	if again.Results[0].Content != "original" {
		t.Fatalf("cache retained producer-owned result: %+v", again.Results[0])
	}
}

// ─── Test: Expiry ─────────────────────────────────────────────────────────────

func TestEmbedCache_Expiry(t *testing.T) {
	cache := newTestEmbedCache(10, 10*time.Millisecond)

	r := makeResults("test", 1)
	cache.Set("test", r)

	// Should hit immediately.
	_, ok := cache.Get("test")
	if !ok {
		t.Fatal("expected cache hit before expiry")
	}

	// Poll until entry expires (TTL is 10ms).
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()

	for expired := false; !expired; {
		select {
		case <-ticker.C:
			_, ok = cache.Get("test")
			if !ok {
				expired = true
			}
		case <-deadline:
			t.Fatal("timeout waiting for cache expiry")
		}
	}

	// Should miss after expiry.
	if ok {
		t.Error("expected cache miss after expiry")
	}
	// Entry should have been evicted.
	stats := cache.Stats()
	if stats.Size != 0 {
		t.Errorf("expected size 0 after expiry eviction, got %d", stats.Size)
	}
}

// ─── Test: Eviction ───────────────────────────────────────────────────────────

func TestEmbedCache_Eviction(t *testing.T) {
	cache := newTestEmbedCache(3, 1*time.Hour)

	// Fill the cache. Set is synchronous, so insertion order
	// in the internal order slice is deterministic without sleeps.
	cache.Set("prompt-1", makeResults("prompt-1", 1))
	cache.Set("prompt-2", makeResults("prompt-2", 1))
	cache.Set("prompt-3", makeResults("prompt-3", 1))

	if cache.Stats().Size != 3 {
		t.Fatalf("expected size 3, got %d", cache.Stats().Size)
	}

	// Add a 4th entry — should evict the oldest (prompt-1).
	cache.Set("prompt-4", makeResults("prompt-4", 1))

	stats := cache.Stats()
	if stats.Size != 3 {
		t.Errorf("expected size 3 after eviction, got %d", stats.Size)
	}

	// prompt-1 should be evicted.
	_, ok := cache.Get("prompt-1")
	if ok {
		t.Error("expected prompt-1 to be evicted (oldest)")
	}
	// prompt-2, prompt-3, prompt-4 should still exist.
	for _, p := range []string{"prompt-2", "prompt-3", "prompt-4"} {
		_, ok := cache.Get(p)
		if !ok {
			t.Errorf("expected %s to still be in cache", p)
		}
	}
}

// ─── Test: Clear ──────────────────────────────────────────────────────────────

func TestEmbedCache_Clear(t *testing.T) {
	cache := newTestEmbedCache(10, 1*time.Hour)

	cache.Set("a", makeResults("a", 1))
	cache.Set("b", makeResults("b", 1))
	cache.Get("a") // add a hit

	if cache.Stats().Size != 2 {
		t.Fatalf("expected size 2 before clear, got %d", cache.Stats().Size)
	}

	cache.Clear()

	stats := cache.Stats()
	if stats.Size != 0 {
		t.Errorf("expected size 0 after clear, got %d", stats.Size)
	}
	if stats.Hits != 0 {
		t.Errorf("expected hits 0 after clear, got %d", stats.Hits)
	}
	if stats.Misses != 0 {
		t.Errorf("expected misses 0 after clear, got %d", stats.Misses)
	}

	// Should miss on previously-set keys.
	_, ok := cache.Get("a")
	if ok {
		t.Error("expected miss after clear")
	}
}

// ─── Test: Stats Tracking ─────────────────────────────────────────────────────

func TestEmbedCache_Stats(t *testing.T) {
	cache := newTestEmbedCache(10, 1*time.Hour)

	// Miss.
	cache.Get("unknown")
	// Miss again.
	cache.Get("unknown")

	// Set and hit.
	cache.Set("known", makeResults("known", 1))
	cache.Get("known")

	s := cache.Stats()
	if s.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", s.Hits)
	}
	if s.Misses != 2 {
		t.Errorf("expected 2 misses, got %d", s.Misses)
	}
	// hit rate = 1 / 3 ≈ 0.3333
	if s.HitRate < 0.3 || s.HitRate > 0.34 {
		t.Errorf("expected hit rate ~0.3333, got %f", s.HitRate)
	}
	if s.Size != 1 {
		t.Errorf("expected size 1, got %d", s.Size)
	}
	if s.MaxSize != 10 {
		t.Errorf("expected max size 10, got %d", s.MaxSize)
	}
}

// ─── Test: Concurrent Get/Set ─────────────────────────────────────────────────

func TestEmbedCache_Concurrent(t *testing.T) {
	cache := newTestEmbedCache(100, 1*time.Hour)

	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				prompt := "prompt-" + string(rune('A'+(id%26)))
				// Mix of reads and writes.
				if i%3 == 0 {
					cache.Set(prompt, makeResults(prompt, 1))
				} else {
					cache.Get(prompt)
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify consistency: stats should not report negative values.
	stats := cache.Stats()
	if stats.Hits < 0 {
		t.Errorf("hits should not be negative, got %d", stats.Hits)
	}
	if stats.Misses < 0 {
		t.Errorf("misses should not be negative, got %d", stats.Misses)
	}
	if stats.Size < 0 {
		t.Errorf("size should not be negative, got %d", stats.Size)
	}
	if stats.Size > 100 {
		t.Errorf("size should not exceed max size 100, got %d", stats.Size)
	}

	// No race condition should be detected (run with -race).
}

func TestEmbedCache_ExpiredGetCannotEvictReplacement(t *testing.T) {
	cache := newTestEmbedCache(10, time.Hour)
	const prompt = "replacement-race"
	cache.Set(prompt, makeResults("old", 1))

	// Force the observed entry into the expired state without making the test
	// depend on a wall-clock scheduling window.
	cache.mu.Lock()
	cache.entries[cache.cacheKey(prompt)].expiresAt = time.Now().Add(-time.Hour)
	cache.mu.Unlock()

	var misses atomic.Int64
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 200; j++ {
				if got, ok := cache.Get(prompt); !ok || got.Query != "new" {
					// Misses before the replacement are expected; after the
					// replacement, a miss indicates stale eviction.
					misses.Add(1)
				}
			}
		}()
	}
	close(start)
	for i := 0; i < 200; i++ {
		cache.Set(prompt, makeResults("new", 1))
	}
	wg.Wait()

	got, ok := cache.Get(prompt)
	if !ok || got == nil || len(got.Results) != 1 || got.Query != "new" {
		t.Fatalf("replacement was evicted by an expired Get: got=%+v hit=%v misses=%d", got, ok, misses.Load())
	}
}

// ─── Test: Exact Match (Different Prompts) ────────────────────────────────────

func TestEmbedCache_ExactMatch(t *testing.T) {
	cache := newTestEmbedCache(10, 1*time.Hour)

	r1 := makeResults("prompt one", 1)
	r2 := makeResults("prompt two", 2)

	cache.Set("Build a REST API endpoint", r1)
	cache.Set("build a graphql api endpoint", r2)

	// Exact match — same prompt.
	got, ok := cache.Get("Build a REST API endpoint")
	if !ok {
		t.Fatal("expected cache hit for exact prompt")
	}
	if len(got.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(got.Results))
	}

	// Different prompt — should miss.
	_, ok = cache.Get("build a different thing")
	if ok {
		t.Error("expected cache miss for different prompt")
	}
}

// ─── Test: Case and Whitespace Normalization ──────────────────────────────────

func TestEmbedCache_CaseInsensitiveMatch(t *testing.T) {
	cache := newTestEmbedCache(10, 1*time.Hour)

	r := makeResults("original", 1)
	cache.Set("  Build a REST API ENDPOINT  ", r)

	// Same content, different case and whitespace.
	got, ok := cache.Get("build a rest api endpoint")
	if !ok {
		t.Fatal("expected cache hit for case-insensitive, whitespace-normalized match")
	}
	if len(got.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(got.Results))
	}

	got, ok = cache.Get("  BUILD A REST API ENDPOINT  ")
	if !ok {
		t.Fatal("expected cache hit for uppercase match")
	}
	if len(got.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(got.Results))
	}
}

// ─── Test: Nil Results ────────────────────────────────────────────────────────

func TestEmbedCache_NilResults(t *testing.T) {
	cache := newTestEmbedCache(10, 1*time.Hour)

	// Set should not panic on nil results.
	cache.Set("prompt", nil)

	// Nothing should have been stored.
	_, ok := cache.Get("prompt")
	if ok {
		t.Error("expected miss after setting nil results")
	}
	if cache.Stats().Size != 0 {
		t.Errorf("expected size 0, got %d", cache.Stats().Size)
	}
}

// ─── Test: Disabled Cache ─────────────────────────────────────────────────────

func TestEmbedCache_Disabled(t *testing.T) {
	cache := NewEmbedCache(EmbedCacheConfig{
		Enabled: false,
		MaxSize: 10,
		TTL:     1 * time.Hour,
	})

	cache.Set("prompt", makeResults("prompt", 1))
	_, ok := cache.Get("prompt")
	if ok {
		t.Error("expected cache miss when disabled")
	}

	stats := cache.Stats()
	if stats.Size != 0 {
		t.Errorf("expected size 0 when disabled, got %d", stats.Size)
	}
}

// ─── Test: Invalidate Specific Entry ──────────────────────────────────────────

func TestEmbedCache_Invalidate(t *testing.T) {
	cache := newTestEmbedCache(10, 1*time.Hour)

	cache.Set("keep", makeResults("keep", 1))
	cache.Set("remove", makeResults("remove", 1))

	cache.Invalidate("remove")

	// "keep" should still be present.
	_, ok := cache.Get("keep")
	if !ok {
		t.Error("expected 'keep' to still be cached")
	}
	// "remove" should be gone.
	_, ok = cache.Get("remove")
	if ok {
		t.Error("expected 'remove' to be invalidated")
	}
	if cache.Stats().Size != 1 {
		t.Errorf("expected size 1 after invalidation, got %d", cache.Stats().Size)
	}
}

// ─── Test: Overwrite Entry ────────────────────────────────────────────────────

func TestEmbedCache_Overwrite(t *testing.T) {
	cache := newTestEmbedCache(10, 1*time.Hour)

	r1 := makeResults("v1", 1)
	r2 := makeResults("v2", 2)

	cache.Set("prompt", r1)
	cache.Set("prompt", r2) // overwrite

	got, ok := cache.Get("prompt")
	if !ok {
		t.Fatal("expected cache hit after overwrite")
	}
	if len(got.Results) != 2 {
		t.Errorf("expected 2 results after overwrite, got %d", len(got.Results))
	}
	// Size should still be 1 (no duplicate).
	if cache.Stats().Size != 1 {
		t.Errorf("expected size 1 after overwrite, got %d", cache.Stats().Size)
	}
}

// ─── Test: Default Config Values ──────────────────────────────────────────────

func TestEmbedCache_DefaultConfig(t *testing.T) {
	cache := NewEmbedCache(EmbedCacheConfig{
		Enabled: true,
		MaxSize: 0, // Should default to 1000.
		TTL:     0, // Should default to 1h.
	})

	// MaxSize and TTL should have been defaulted.
	r := makeResults("test", 1)
	cache.Set("test", r)
	_, ok := cache.Get("test")
	if !ok {
		t.Error("expected cache hit with default config")
	}
}

// ─── Test: Zero Misses HitRate ────────────────────────────────────────────────

func TestEmbedCache_ZeroMissesHitRate(t *testing.T) {
	cache := newTestEmbedCache(10, 1*time.Hour)

	// Only sets, no gets — HitRate should be 0 (total=0).
	stats := cache.Stats()
	if stats.HitRate != 0 {
		t.Errorf("expected HitRate 0 with no operations, got %f", stats.HitRate)
	}

	cache.Set("a", makeResults("a", 1))
	cache.Get("a") // hit

	stats = cache.Stats()
	// 1 hit, 0 misses -> HitRate = 1.0
	if stats.HitRate != 1.0 {
		t.Errorf("expected HitRate 1.0 with only hits, got %f", stats.HitRate)
	}
}

// ─── Test: EmbedCacheStats Default Embed Cache Config ─────────────────────────

func TestDefaultEmbedCacheConfig(t *testing.T) {
	cfg := DefaultEmbedCacheConfig()

	if !cfg.Enabled {
		t.Error("default config should have Enabled=true")
	}
	if cfg.MaxSize != 1000 {
		t.Errorf("default MaxSize should be 1000, got %d", cfg.MaxSize)
	}
	if cfg.TTL != 1*time.Hour {
		t.Errorf("default TTL should be 1h, got %v", cfg.TTL)
	}
	if cfg.SimilarityThreshold != 0.0 {
		t.Errorf("default SimilarityThreshold should be 0.0, got %f", cfg.SimilarityThreshold)
	}
}

// ─── Test: EmbedCacheStats JSON serialization field check ─────────────────────

func TestEmbedCacheStats_Structure(t *testing.T) {
	s := EmbedCacheStats{
		Size:    5,
		MaxSize: 100,
		Hits:    42,
		Misses:  8,
		HitRate: 0.84,
	}

	if s.Size != 5 {
		t.Errorf("Size mismatch: got %d, want 5", s.Size)
	}
	if s.MaxSize != 100 {
		t.Errorf("MaxSize mismatch: got %d, want 100", s.MaxSize)
	}
	if s.Hits != 42 {
		t.Errorf("Hits mismatch: got %d, want 42", s.Hits)
	}
	if s.Misses != 8 {
		t.Errorf("Misses mismatch: got %d, want 8", s.Misses)
	}
	if s.HitRate != 0.84 {
		t.Errorf("HitRate mismatch: got %f, want 0.84", s.HitRate)
	}
}
