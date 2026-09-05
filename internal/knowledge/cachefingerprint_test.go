package knowledge

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CoscaAI/cosca/internal/cache"
	"github.com/CoscaAI/cosca/internal/search"
)

// fingerprintParams builds search params with FTS only (no embeddings needed).
func fingerprintParams(query string, limit int) search.SearchParams {
	p := search.DefaultSearchParams()
	p.Query = query
	p.Limit = limit
	p.EnableFTS = true
	p.EnableVector = false
	return p
}

// memoryOnlyEngine creates an initialized engine whose cache is memory-only so
// tiny TTLs expire deterministically (SQLite levels persist at second
// granularity and would defeat sub-second TTL tests).
func memoryOnlyEngine(t *testing.T) *Engine {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "cachefp_test.db")
	cfg := Config{
		DBPath:            dbPath,
		RootDir:           t.TempDir(),
		AutoMigrate:       true,
		WatchEnabled:      false,
		EmbeddingProvider: "local",
		IndexerConfig:     DefaultConfig().IndexerConfig,
		CacheConfig: cache.Config{
			MemoryMaxEntries: 1000,
			MemoryTTL:        5 * time.Minute,
			EnabledLevels:    []cache.Level{cache.LevelMemory},
		},
		RankingConfig: DefaultConfig().RankingConfig,
		SearchConfig:  search.DefaultSearchParams(),
	}
	engine, err := New(cfg)
	require.NoError(t, err, "New should succeed")
	require.NoError(t, engine.Init(), "Init should succeed")
	t.Cleanup(func() { _ = engine.Close() })
	return engine
}

// searchableEngine seeds a document so queries return real results.
func searchableEngine(t *testing.T) *Engine {
	t.Helper()
	engine := memoryOnlyEngine(t)
	mdPath := writeTempMarkdown(t, engine.cfg.RootDir, "alpha.md",
		"# Alpha\n\nUnique alpha fingerprint search token.")
	require.NoError(t, engine.IndexDocument(context.Background(), mdPath))
	return engine
}

// ── QueryFingerprint ────────────────────────────────────────────────────────

func TestQueryFingerprint_EquivalentQueries(t *testing.T) {
	t.Parallel()

	params := fingerprintParams("como fazer x", 10)

	a := QueryFingerprint("Como Fazer X?", params)
	b := QueryFingerprint("como fazer x", params)

	assert.Equal(t, a, b, "equivalent queries should share one fingerprint")
	assert.Len(t, a, 64, "SHA-256 hex fingerprint should be 64 chars")
}

func TestQueryFingerprint_Normalization(t *testing.T) {
	t.Parallel()

	base := fingerprintParams("como fazer x", 10)
	baseFp := QueryFingerprint("como fazer x", base)

	for _, query := range []string{
		"Como Fazer X?",
		"como fazer x",
		"  COMO   fazer  X!!!  ",
		"como-fazer-x",
		"como_fazer_x",
		"Como fazer x.",
	} {
		assert.Equal(t, baseFp, QueryFingerprint(query, base),
			"query %q should normalize to the same fingerprint", query)
	}

	for _, query := range []string{
		"como fazer y",
		"como fazer xy",
		"fazer x como",
	} {
		assert.NotEqual(t, baseFp, QueryFingerprint(query, base),
			"query %q should produce a different fingerprint", query)
	}
}

func TestQueryFingerprint_DifferentParams(t *testing.T) {
	t.Parallel()

	a := QueryFingerprint("como fazer x", fingerprintParams("como fazer x", 10))
	b := QueryFingerprint("como fazer x", fingerprintParams("como fazer x", 20))
	assert.NotEqual(t, a, b, "different limit should produce different fingerprint")

	c := QueryFingerprint("como fazer y", fingerprintParams("como fazer x", 10))
	assert.NotEqual(t, a, c, "different query should produce different fingerprint")

	pTypes := fingerprintParams("x", 10)
	pTypes.Types = []string{"chunk"}
	d := QueryFingerprint("x", fingerprintParams("x", 10))
	assert.NotEqual(t, d, QueryFingerprint("x", pTypes), "different type filter should produce different fingerprint")
}

func TestQueryFingerprint_UnorderedParams(t *testing.T) {
	t.Parallel()

	p1 := fingerprintParams("alpha", 10)
	p1.Types = []string{"chunk", "document"}
	p1.Tags = map[string]string{"a": "1", "b": "2"}

	p2 := fingerprintParams("alpha", 10)
	p2.Types = []string{"document", "chunk"}
	p2.Tags = map[string]string{"b": "2", "a": "1"}

	assert.Equal(t, QueryFingerprint("alpha", p1), QueryFingerprint("alpha", p2),
		"unordered slice/map params should produce the same fingerprint")
}

func TestDefaultCachePolicy(t *testing.T) {
	t.Parallel()

	p := DefaultCachePolicy()
	assert.True(t, p.Enabled)
	assert.Equal(t, 5*time.Minute, p.TTL)
}

// ── SearchWithPolicy ────────────────────────────────────────────────────────

func TestSearchWithPolicy_NotInitialized(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t, filepath.Join(t.TempDir(), "ni.db"))
	policy := DefaultCachePolicy()
	_, err := engine.SearchWithPolicy(context.Background(), fingerprintParams("x", 5), &policy)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestSearchWithPolicy_Disabled_DelegatesToSearch(t *testing.T) {
	t.Parallel()

	engine := newTestEngine(t, filepath.Join(t.TempDir(), "ni.db"))
	params := fingerprintParams("x", 5)

	_, errSearch := engine.Search(context.Background(), params)
	_, errPolicy := engine.SearchWithPolicy(context.Background(), params, &CachePolicy{Enabled: false})
	require.Error(t, errPolicy)
	assert.Equal(t, errSearch.Error(), errPolicy.Error(),
		"disabled policy must behave identically to the original Search path")
}

func TestSearchWithPolicy_NilPolicy_DelegatesToSearch(t *testing.T) {
	t.Parallel()

	engine := searchableEngine(t)
	params := fingerprintParams("alpha", 10)

	base, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, base)

	res, err := engine.SearchWithPolicy(context.Background(), params, nil)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, base.TotalCount, res.TotalCount,
		"nil policy must return identical results to the original Search")

	stats := engine.CacheStats()
	assert.False(t, stats.Enabled, "nil policy must not mark the fingerprint cache as used")
	assert.Equal(t, int64(0), stats.Misses, "nil policy must not count fingerprint misses")
}

func TestSearchWithPolicy_Disabled_IgnoresCache(t *testing.T) {
	t.Parallel()

	engine := searchableEngine(t)
	params := fingerprintParams("alpha", 10)
	fp := QueryFingerprint(params.Query, params)

	// Poison the fingerprint cache entry with a decoy result.
	decoy := &search.SearchResults{Query: "alpha", TotalCount: 999}
	require.NoError(t, engine.cache.Set(fingerprintCachePrefix+fp, decoy, time.Minute))

	// Disabled policy must NOT consult the fingerprint cache.
	policy := CachePolicy{Enabled: false}
	results, err := engine.SearchWithPolicy(context.Background(), params, &policy)
	require.NoError(t, err)
	require.NotNil(t, results)
	assert.NotEqual(t, 999, results.TotalCount,
		"disabled policy must bypass the cache and perform the real search")

	// Enabled policy MUST return the decoy straight from the cache.
	enabled := DefaultCachePolicy()
	cached, err := engine.SearchWithPolicy(context.Background(), params, &enabled)
	require.NoError(t, err)
	require.NotNil(t, cached)
	assert.Equal(t, 999, cached.TotalCount,
		"enabled policy should hit the fingerprint cache and return the decoy")
}

func TestSearchWithPolicy_Enabled_CachesUnderFingerprint(t *testing.T) {
	t.Parallel()

	engine := searchableEngine(t)
	policy := DefaultCachePolicy()

	// First call — miss, real search performed.
	_, err := engine.SearchWithPolicy(context.Background(), fingerprintParams("alpha", 10), &policy)
	require.NoError(t, err)

	stats := engine.CacheStats()
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, int64(0), stats.Hits)
	assert.True(t, stats.Enabled)
	assert.Equal(t, DefaultSearchCacheTTL, stats.TTL)
	assert.GreaterOrEqual(t, stats.Entries, 1)

	// Second call — equivalent query (case + punctuation) → cache HIT.
	_, err = engine.SearchWithPolicy(context.Background(), fingerprintParams("Alpha?", 10), &policy)
	require.NoError(t, err)

	stats = engine.CacheStats()
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, int64(1), stats.Hits)
}

func TestSearchWithPolicy_TTLRespected(t *testing.T) {
	t.Parallel()

	engine := searchableEngine(t)
	params := fingerprintParams("alpha", 10)

	tiny := DefaultCachePolicy()
	tiny.TTL = 50 * time.Millisecond

	// Miss → stored with 50ms TTL.
	_, err := engine.SearchWithPolicy(context.Background(), params, &tiny)
	require.NoError(t, err)
	assert.Equal(t, int64(1), engine.CacheStats().Misses)

	// Hit while still valid.
	_, err = engine.SearchWithPolicy(context.Background(), params, &tiny)
	require.NoError(t, err)
	assert.Equal(t, int64(1), engine.CacheStats().Hits)

	// TTL expires → re-fetch (miss again).
	time.Sleep(150 * time.Millisecond)
	_, err = engine.SearchWithPolicy(context.Background(), params, &tiny)
	require.NoError(t, err)

	stats := engine.CacheStats()
	assert.Equal(t, int64(1), stats.Hits, "one hit before expiry")
	assert.Equal(t, int64(2), stats.Misses, "expiry must trigger a refresh")
}

func TestSearchWithPolicy_ZeroTTL_FallsBackToDefault(t *testing.T) {
	t.Parallel()

	engine := searchableEngine(t)
	params := fingerprintParams("alpha", 10)

	policy := CachePolicy{Enabled: true, TTL: 0} // 0 → default 5m
	_, err := engine.SearchWithPolicy(context.Background(), params, &policy)
	require.NoError(t, err)

	assert.Equal(t, DefaultSearchCacheTTL, engine.CacheStats().TTL)
}

func TestClearCache_ResetsStatsAndEntries(t *testing.T) {
	t.Parallel()

	engine := searchableEngine(t)
	policy := DefaultCachePolicy()

	_, err := engine.SearchWithPolicy(context.Background(), fingerprintParams("alpha", 10), &policy)
	require.NoError(t, err)

	stats := engine.CacheStats()
	require.Equal(t, int64(1), stats.Misses)
	require.GreaterOrEqual(t, stats.Entries, 1)

	require.NoError(t, engine.ClearCache())

	stats = engine.CacheStats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	assert.Equal(t, 0, stats.Entries)
	assert.False(t, stats.Enabled)
}
