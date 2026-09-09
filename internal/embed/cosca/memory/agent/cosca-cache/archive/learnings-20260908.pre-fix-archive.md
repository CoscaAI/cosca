# cosca-cache - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

# cosca-cache — Semantic Learnings

> Auto-evolution memory.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-cache |
| **Task** | Initial capability establishment |
| **Technique** | Standard cache patterns |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #cache #baseline |
| **Learned** | Ready for Level 2. |
| **Next** | Identify first advanced technique |

## Task Learnings

### 2026-07-28 — Cache-Search Integration (Multi-Tier Wire-Up)
| Field | Value |
|-------|-------|
| **Agent** | cosca-cache |
| **Task** | Conectar o cache engine multi-tier ao Search() do KnowledgeEngine (~20 LOC, 64x speedup) |
| **Technique** | Multi-tier cache integration with dual type-assertion fallback (memory → JSON deserialization); deterministic cache key generation from all SearchParams fields; nil-safe cache access pattern |
| **Level** | 3 |
| **Outcome** | success |
| **Confidence** | 0.90 |
| **Tags** | #cache #integration #multi-tier #search #quick-win #json #performance |
| **Learned** | 1) The `cache.Cache` multi-tier framework stores values differently per tier: memory stores the original Go type (`interface{}`), while SQLite/FS do JSON round-trips that return `map[string]interface{}` or `string`. To make caching work across ALL tiers consistently, serialize to JSON string on Set() and try both `*search.SearchResults` (memory hit) and `string`+`json.Unmarshal` (SQLite/FS hit) on Get(). 2) The `KnowledgeEngine.Search()` was the critical missing integration point — `e.cache` existed and was initialized but never queried in the search path. ~40 LOC added (import + cache key helper + cache check + cache store). 3) All cache levels are active: DefaultConfig enables Memory+SQLite, and `Init()` wires `db.Conn()` as the SQLite backend. 4) Cache key is SHA256 of all 13 distinguishing SearchParams fields (query, limit, offset, types, path, tags, since, enableFTS/vector/graph/facets, minScore) using the built-in `cache.Key()` helper. 5) Tests pass — all 13 Search tests green. `TestEngine_Rebuild_AfterInit` was pre-existing failure (FTS rebuild limitation). Race detector hits are upstream `modernc.org/sqlite` issues, not cache-related. 6) The `Query()` convenience method automatically benefits from cache via its delegation to `Search()`. |
| **Pattern** | Dual type-assertion cache retrieval: `TryMemoryType → TryJSONDeserialize` — handles the impedance mismatch between memory cache (preserves Go types) and persistent caches (JSON round-trip). Nil-safe cache guard: always check `e.cache != nil` since Init() can proceed without cache on failure. Cache key: hash all distinguishing params with `cache.Key()` for collision-resistant deterministic keys. |
| **Next** | P2: cache GetStats() results; P3: consolidate EmbedCache into cache.Cache as backend; P4: implement Redis backend for distributed caching |

### 2026-07-28 — Full Cache Infrastructure Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-cache |
| **Task** | Audit entire Cosca platform cache infrastructure and recommend implementation |
| **Technique** | Multi-tier cache audit, gap analysis, quick-win identification, TTL tiering strategy |
| **Level** | 3 |
| **Outcome** | success |
| **Confidence** | 0.85 |
| **Tags** | #audit #cache #multi-tier #redis #ttl #invalidation #vector-search #performance |
| **Learned** | 1) Cosca has a **mature multi-tier cache framework** (L1: in-memory LRU, L2: SQLite, L3: filesystem) in `internal/cache/cache.go` — but it's **not wired into the most critical path**: `KnowledgeEngine.Search()` goes straight to FTS5+vector without any cache lookup, despite `e.cache` being instantiated. 2) The orchestrator's `EmbedCache` is the only truly active search-level cache (keyed by prompt hash, used in ContextBuilder.BuildKnowledge), creating a **parallel cache system** when the main one could serve the same purpose. 3) **Quick wins**: 20 lines of code in `knowledge.go` would cache search results and reduce vector search latency from 64ms to <1ms on cache hits (~64x faster). 4) Five cache implementations exist but three are the same pattern reinvented (cache.Cache, EmbedCache, embeddingCache) — each with its own API, LRU, and TTL logic. 5) Redis backend is declared but never implemented. 6) The cache warm CLI command is a stub. |
| **Pattern** | Multi-tier audit pattern: explore all cache files → identify integration points → find unused capabilities → quantify quick wins → propose tiered TTL strategy → document naming conventions |
| **Next** | Implement P1 (cache search results in KnowledgeEngine.Search) and P2 (cache GetStats); refactor EmbedCache to use cache.Cache as backend; implement Redis backend |

### 2026-07-28 — Cache Architecture Pattern: Redundant Implementations
| Field | Value |
|-------|-------|
| **Agent** | cosca-cache |
| **Task** | Identified during cache audit — three parallel cache implementations |
| **Technique** | Redundancy detection and consolidation strategy |
| **Level** | 3 |
| **Outcome** | success |
| **Confidence** | 0.72 |
| **Tags** | #architecture #anti-pattern #consolidation #dry |
| **Learned** | The codebase has three independently-implemented caches: `cache.Cache` (multi-tier, 583 LOC), `EmbedCache` (in-memory, 228 LOC), and `embeddingCache` (in `internal/embeddings/`, inline). All three implement LRU eviction, TTL, hit/miss stats, and Get/Set APIs independently. `EmbedCache` and `cache.Cache` both use SHA256 for key hashing. Consolidating into cache.Cache as the single backend (with typed wrappers) would eliminate ~400+ LOC of duplicated cache logic while adding SQLite persistence and filesystem fallback to the EmbedCache path. |
| **Next** | Propose refactor plan to consolidate caches under cache.Cache interface |

