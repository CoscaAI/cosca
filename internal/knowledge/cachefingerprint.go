// Cache de conhecimento externo por fingerprint de query com TTL configurável.
//
// Regra do Don: "se 50 tarefas perguntarem 'Como fazer X no Bubblewrap?', não
// faça 50 buscas." Este arquivo implementa o elo central desse cache:
//
//	Query fingerprint → Knowledge cache → valid? sim → reutiliza / não → refresh
//
// A fingerprint é CANÔNICA: a query é normalizada (lowercase, trim,
// colapso de whitespace, remoção de pontuação) e combinada com os parâmetros
// significativos (limit, offset, filtros, modos) antes do SHA-256. Queries
// equivalentes após a normalização compartilham a MESMA fingerprint — a
// string bruta da query nunca é usada como chave.
//
// A política é OPTO-IN: SearchWithPolicy com política nula/desabilitada
// delega para o Search original (comportamento zero-change). O TTL padrão é
// de 5 minutos, igual ao cache de busca legado.
package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/search"
)

// DefaultSearchCacheTTL is the default TTL for fingerprint-cached search
// results — the same 5 minutes the legacy search cache uses.
const DefaultSearchCacheTTL = 5 * time.Minute

// fingerprintCachePrefix namespaces fingerprint entries inside the shared
// multi-level cache so they never collide with the legacy searchCacheKey
// namespace (which is prefixed "search" via cache.Key).
const fingerprintCachePrefix = "searchfp:"

// CachePolicy controls the fingerprint-based knowledge cache.
type CachePolicy struct {
	// TTL is how long a cached result is valid before a refresh. Values <= 0
	// fall back to DefaultSearchCacheTTL (5 minutes).
	TTL time.Duration
	// Enabled toggles the fingerprint cache. When false, SearchWithPolicy
	// delegates to the original Search path — zero behavior change.
	Enabled bool
}

// DefaultCachePolicy returns the default policy: enabled, TTL 5 minutes.
func DefaultCachePolicy() CachePolicy {
	return CachePolicy{TTL: DefaultSearchCacheTTL, Enabled: true}
}

// ── Query fingerprinting ────────────────────────────────────────────────

var (
	fingerprintPunctRE = regexp.MustCompile(`[^\pL\pN\s]+`)
	fingerprintSpaceRE = regexp.MustCompile(`\s+`)
)

// normalizeQuery canonicalizes a query so that equivalent phrasings collapse
// to a single fingerprint: lowercased, trimmed, punctuation stripped, and
// whitespace collapsed to single spaces. Unicode letters/numbers survive
// (accents and non-Latin scripts are preserved).
func normalizeQuery(query string) string {
	s := strings.ToLower(strings.TrimSpace(query))
	s = fingerprintPunctRE.ReplaceAllString(s, " ")
	s = fingerprintSpaceRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// significantParams renders the search parameters that change the result set
// in a deterministic, ordering-insensitive form. Param order within slices
// and maps does not affect the fingerprint.
func significantParams(params search.SearchParams) string {
	types := append([]string(nil), params.Types...)
	sort.Strings(types)

	tags := make([]string, 0, len(params.Tags))
	for k, v := range params.Tags {
		tags = append(tags, k+"="+v)
	}
	sort.Strings(tags)

	return fmt.Sprintf("%d|%d|%v|%s|%v|%d|%t|%t|%t|%t|%f",
		params.Limit,
		params.Offset,
		types,
		params.Path,
		tags,
		params.Since.Unix(),
		params.EnableFTS,
		params.EnableVector,
		params.EnableGraph,
		params.EnableFacets,
		params.MinScore,
	)
}

// QueryFingerprint computes the canonical SHA-256 fingerprint of a query and
// its significant search parameters. Queries that are equivalent after
// normalization (case, punctuation, whitespace) share a fingerprint; the raw
// query string is never used as the key.
func QueryFingerprint(query string, params search.SearchParams) string {
	h := sha256.Sum256([]byte(normalizeQuery(query) + "\x1f" + significantParams(params)))
	return hex.EncodeToString(h[:])
}

// ── Fingerprint cache statistics ─────────────────────────────────────────

// fingerprintStatsEntry tracks per-engine fingerprint-cache activity without
// touching Engine's struct (which lives in knowledge.go).
type fingerprintStatsEntry struct {
	hits    atomic.Int64 // cache hits served
	misses  atomic.Int64 // cache misses (re-fetches)
	lastTTL atomic.Int64 // nanoseconds of the last policy TTL used
	used    atomic.Bool  // whether any enabled policy ran on this engine
}

var (
	fpStatsMu sync.Mutex
	fpStats   = make(map[*Engine]*fingerprintStatsEntry)
)

func statsFor(e *Engine) *fingerprintStatsEntry {
	fpStatsMu.Lock()
	defer fpStatsMu.Unlock()
	entry, ok := fpStats[e]
	if !ok {
		entry = &fingerprintStatsEntry{}
		fpStats[e] = entry
	}
	return entry
}

// CacheStats reports fingerprint-cache statistics for this engine.
type CacheStats struct {
	// Hits is the number of fingerprint cache hits served in this process.
	Hits int64 `json:"hits"`
	// Misses is the number of fingerprint cache misses (re-fetches).
	Misses int64 `json:"misses"`
	// Entries is the total number of entries in the backing cache.
	Entries int `json:"entries"`
	// TTL is the fingerprint cache TTL in effect.
	TTL time.Duration `json:"ttl"`
	// Enabled reports whether an enabled policy has been used.
	Enabled bool `json:"enabled"`
	// Err expõe falhas de leitura do cache subjacente (cache quebrado)
	// para distinguir "0 entradas (vazio)" de "0 entradas (indisponível)".
	Err string `json:"err,omitempty"`
}

// CacheStats returns current fingerprint-cache statistics.
func (e *Engine) CacheStats() CacheStats {
	st := statsFor(e)
	out := CacheStats{
		Hits:    st.hits.Load(),
		Misses:  st.misses.Load(),
		TTL:     time.Duration(st.lastTTL.Load()),
		Enabled: st.used.Load(),
	}
	if out.TTL <= 0 {
		out.TTL = DefaultSearchCacheTTL
	}
	if e.cache != nil {
		cs := e.cache.Stats()
		out.Entries = cs.MemoryEntries + cs.SQLiteEntries + cs.FileEntries
		if cs.SQLiteErr != "" || cs.FileErr != "" {
			out.Err = cs.SQLiteErr + cs.FileErr
		}
	}
	return out
}

// ClearCache clears the backing cache (all levels) and resets the fingerprint
// statistics.
func (e *Engine) ClearCache() error {
	if e.cache != nil {
		if err := e.cache.Clear(); err != nil {
			return err
		}
	}
	st := statsFor(e)
	st.hits.Store(0)
	st.misses.Store(0)
	st.used.Store(false)
	return nil
}

// ── SearchWithPolicy ─────────────────────────────────────────────────────

// SearchWithPolicy performs a hybrid search behind the fingerprint cache.
//
// When policy.Enabled and the engine has a cache, results are cached under
// QueryFingerprint with policy.TTL (falling back to the default 5 minutes).
// Equivalent queries — e.g. 50 tasks asking "How do I do X in Bubblewrap?"
// with only cosmetic differences — share one fingerprint and therefore one
// external-knowledge fetch instead of 50. When policy is nil or disabled,
// SearchWithPolicy delegates to Search: behavior is identical to the legacy
// path.
func (e *Engine) SearchWithPolicy(ctx context.Context, params search.SearchParams, policy *CachePolicy) (*search.SearchResults, error) {
	pol := CachePolicy{}
	if policy != nil {
		pol = *policy
	}
	if !pol.Enabled {
		return e.Search(ctx, params)
	}
	if pol.TTL <= 0 {
		pol.TTL = DefaultSearchCacheTTL
	}

	e.mu.RLock()
	if !e.initialized {
		e.mu.RUnlock()
		return nil, fmt.Errorf("knowledge engine not initialized")
	}
	e.mu.RUnlock()

	fp := QueryFingerprint(params.Query, params)
	key := fingerprintCachePrefix + fp

	st := statsFor(e)
	st.used.Store(true)
	st.lastTTL.Store(int64(pol.TTL))

	// Check multi-tier cache (Memory → SQLite → Filesystem)
	if e.cache != nil {
		if cached, ok := e.cache.Get(key); ok {
			// Memory cache returns original value types; SQLite/FS return
			// JSON-deserialized types.
			if results, ok := cached.(*search.SearchResults); ok {
				st.hits.Add(1)
				log.Debug().Str("fingerprint", fp).Msg("search fingerprint cache hit (memory)")
				return results, nil
			}
			if jsonStr, ok := cached.(string); ok {
				var results search.SearchResults
				if err := json.Unmarshal([]byte(jsonStr), &results); err == nil {
					st.hits.Add(1)
					log.Debug().Str("fingerprint", fp).Msg("search fingerprint cache hit (json)")
					return &results, nil
				}
			}
		}
	}

	// Cache miss — execute full hybrid search
	st.misses.Add(1)
	results, err := e.search.Search(ctx, params)
	if err != nil {
		return nil, err
	}

	// Store result in all enabled cache levels with the policy TTL
	if e.cache != nil && results != nil {
		serialized, jsonErr := json.Marshal(results)
		if jsonErr == nil {
			if setErr := e.cache.Set(key, string(serialized), pol.TTL); setErr != nil {
				log.Warn().Err(setErr).Str("fingerprint", fp).Msg("search fingerprint cache set failed")
			}
		}
	}

	return results, nil
}
