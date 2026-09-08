# cosca-performance - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

# cosca-performance — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Session: 2026-07-28 — Project Performance Baseline (Level 1)

### 2026-07-28 — Architecture Performance Characteristics
| Field | Value |
|-------|-------|
| **Agent** | cosca-performance |
| **Task** | Establish performance baseline from codebase analysis |
| **Technique** | Level 1 — Static analysis: traced SQLite WAL mode config, single-writer architecture, embedded database (no network latency), Go concurrency patterns (goroutines, WaitGroups, mutexes) |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #performance #sqlite #go #concurrency #baseline |
| **Related** | internal/sqlite/db.go, internal/runtime/runtime.go, internal/runtime/metrics.go |
| **Learned** | Performance characteristics from architecture: SQLite WAL mode enables concurrent reads with single writer — typical queries < 1ms (point), < 10ms (FTS5). No network latency (embedded, same process). Runtime uses WaitGroup for goroutine lifecycle, mutex-guarded state transitions. Known bottlenecks: single-writer SQLite limits concurrent write throughput, knowledge index rebuild is full-scan, no connection pooling needed (embedded). Daemon watchdog restarts unhealthy subsystems (30s timeout). Metrics: 8 atomic counters (lock-free), 4 duration histograms (sorted-slice, O(n) insert), no sampling/aggregation. |
| **Next** | Level 2: Run Go benchmarks on search pipeline, measure index rebuild time, profile memory under sustained load, verify goroutine leak patterns |

### 2026-07-28 — Known Performance Bugs
| Field | Value |
|-------|-------|
| **Agent** | cosca-performance |
| **Task** | Catalog known performance issues from bug database |
| **Technique** | Level 1 — Bug triage: reviewed bug directory for performance-related entries |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #performance #bugs #sqlite #database |
| **Related** | memory/bug/bug-005-sqlite-first-run.md |
| **Learned** | One active performance bug: slow dashboard query (bug-005). Root cause unknown — needs query plan analysis via EXPLAIN QUERY PLAN. Other bugs are non-performance (tmp path, lint issues, race conditions, provider caching, sqlite first-run panic). |
| **Next** | Level 2: Root-cause bug-005 with EXPLAIN QUERY PLAN + pprof CPU profile |

---

## Session: 2026-07-28 — Performance Baseline Execution (Level 2)

### 2026-07-28 — Vector Search Scaling (Brute-Force)
| Field | Value |
|-------|-------|
| **Agent** | cosca-performance |
| **Task** | Benchmark vector similarity search at scale |
| **Technique** | Level 2 — Empirical benchmarks with `go test -bench=. -benchmem -benchtime=3s` on real SQLite-backed vector store with 100-10K random 128d vectors |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #performance #vector #sqlite-vec #benchmark #scaling |
| **Measured** | 100vec: 542µs/438KB; 1Kvec: 4.7ms/4.5MB; 5Kvec: 30ms/22.9MB; 10Kvec: 64ms/46.8MB — O(n) linear scaling confirmed |
| **Allocations** | 3.8K allocs/op @ 100vec → 380K allocs/op @ 10Kvec — GC pressure unsustainable |
| **Related** | internal/vector/sqlite_vec.go, internal/vector/vector_bench_test.go |
| **Learned** | Brute-force cosine similarity confirmed via VectorStats().IndexType = "flat_bruteforce". Each search: full table scan → BLOB deserialize → cosine sim → sort. Memory allocation is 2× per vector (float64 slice + SearchResult struct). At production scale (>100K vectors), latencies would exceed 600ms with 468MB allocations — requires ANN index (FAISS/HNSW/usearch). Binary encoding (float64↔bytes) is fast at 265ns but allocates 1024B per vector. Batch insert (50 vectors) is only 20% more expensive than single insert — batching is effective. |
| **Next** | Level 3: Implement HNSW index in pure Go; benchmark at 100K+ vectors; compare against FAISS |

### 2026-07-28 — Bug-005 Root Cause: FTS5 Schema Mismatch
| Field | Value |
|-------|-------|
| **Agent** | cosca-performance |
| **Task** | Root-cause the "slow dashboard query" linked to bug-005 territory |
| **Technique** | Level 2 — Static schema analysis + empirical FTS5 query testing in benchmarks |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #bug #fts5 #sqlite #schema #correctness #critical |
| **Measured** | `documents_fts` uses `content='documents'` (content-sync) referencing a `content` column that doesn't exist in `documents` table |
| **Error** | `SQL logic error: no such column: T.content (1)` on every `documents_fts` MATCH query |
| **Related** | internal/sqlite/schema.go:237-244, internal/sqlite/fts.go:93-98, internal/sqlite/fts_bench_test.go |
| **Learned** | Bug-005's performance investigation revealed a correctness bug: the schema defines `documents_fts` as a content-sync FTS5 table with `content='documents'`, but the `documents` base table has no `content` column. This means document-level FTS search is silently broken — queries return zero results with a logged warning. The "slow dashboard" report was likely caused by users running broader queries to compensate for document search failures, hitting more tables and more data. This is NOT a performance bug — it's a correctness bug that manifests as perceived slowness. Two fix options: (A) remove `content='documents'` since documents don't have body content (lives in chunks), or (B) add `content TEXT` column to documents table. Option A is preferred — zero migration cost, same FTS capability via chunks_fts. |
| **Next** | P0 fix: choose Option A, update schema.go, verify with EXPLAIN QUERY PLAN; re-run benchmarks on fixed schema |

### 2026-07-28 — Search Pipeline Allocation Profile
| Field | Value |
|-------|-------|
| **Agent** | cosca-performance |
| **Task** | Measure search pipeline memory allocations and CPU overhead |
| **Technique** | Level 2 — `go test -bench=. -benchmem -benchtime=2s` on search.Engine with mock dependencies |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #performance #search #benchmark #allocation #hybrid-search |
| **Measured** | Graph-only: 33µs/57KB/187allocs; Vector-only: 43µs/69KB/329allocs; Hybrid: 11µs/14KB/88allocs; Dedup (100 results): 67µs/122KB/530allocs |
| **Related** | internal/search/search.go, internal/search/search_bench_test.go |
| **Learned** | Search pipeline overhead is acceptable (<100µs) for operation counts up to 100 results. The dominant allocator is the vector search path (329 allocs/op from embedding request creation + SearchResult structs). Deduplication via `map[string]bool` scales poorly — 530 allocs for 100 results. The `SearchResult` struct (15 fields with maps) is allocation-heavy — consider pooling or using flat structures for intermediate results. `generateSnippet()` is surprisingly expensive (343ns, 320B) — called per-result. `ftsTableToResultType` is zero-alloc (5.7ns, switch statement inlined). |
| **Next** | Level 3: Add sync.Pool for SearchResult structs; pre-allocate result slices with capacity hints; measure GC pause times under load |

### 2026-07-28 — Dashboard Query Analysis (N+1 Roundtrips)
| Field | Value |
|-------|-------|
| **Agent** | cosca-performance |
| **Task** | Profile `GetStats()` and dashboard query patterns |
| **Technique** | Level 2 — Static code analysis + individual `COUNT(*)` benchmarks |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #performance #dashboard #sql #optimization #caching |
| **Measured** | `COUNT(*) documents`: ~1.2µs; `COUNT(*) chunks`: ~2.5µs; Full GetStats (6 queries): ~13µs |
| **Related** | internal/knowledge/knowledge.go:410-483, internal/sqlite/fts_bench_test.go |
| **Learned** | Individual `COUNT(*)` queries are fast (<3µs) due to SQLite B-tree index scan. The problem is the N+1 roundtrip pattern in GetStats(): documents count → chunks count → entities count → vector stats → graph stats → cache stats = 6 sequential DB queries + graph serialization + os.Stat(). No caching between calls. Recommended fix: add 5s TTL cache on Engine struct, invalidate on index/rebuild events. Also: Sync() has same pattern — `SELECT path FROM documents` loads all document paths, then checks filesystem existence individually — could be optimized with a single query returning paths not on disk. |
| **Next** | Implement stats caching (4h); benchmark dashboard load before/after; add to G7 baseline |

### 2026-07-28 — Benchmark Execution Failure: FTS5 Real Queries
| Field | Value |
|-------|-------|
| **Agent** | cosca-performance |
| **Task** | Execute real FTS5 benchmarks against documents_fts |
| **Technique** | Level 2 — Attempted in-memory SQLite with full schema, real FTS5 MATCH queries |
| **Level** | 2 |
| **Outcome** | failure |
| **Tags** | #failure #benchmark #fts5 #schema-bug |
| **Error** | `SQL logic error: no such column: T.content (1)` — caused by content-sync FTS5 schema mismatch |
| **Related** | internal/sqlite/fts_bench_test.go, internal/sqlite/schema.go |
| **Learned** | Real FTS5 benchmarks blocked by schema bug. All 16 benchmarks in fts_bench_test.go compile but documents_fts queries fail at runtime. Chunks_fts queries work correctly (chunks table has `content` column). Workaround applied: static analysis + chunks_fts-only benchmarks. The benchmark infrastructure is ready — once schema is fixed, benchmarks will run correctly. Failure was productive: it exposed a critical correctness bug that would otherwise remain hidden (the code logs a warning and continues, so it never fails visibly). |
| **Next** | Fix schema first (P0, 2h), then re-run all 16 benchmarks; compare before/after results |

---

### 2026-08-24 — Vector Index READ-ONLY Baseline (real corpus, Level 2→3)

| Field | Value |
|-------|-------|
| **Agent** | cosca-performance |
| **Task** | Medir o estado REAL do índice vetorial (recall/NDCG/distribuição/duplicatas) antes de transformar — READ-ONLY |
| **Technique** | Level 2—Empírico read-only: abrir `knowledge.db` com `modernc.org/sqlite` em `?mode=ro` e replicar em Go puro (int64) os 3 caminhos de score do `SQLiteVec` (float32 oracle, int16 default, int8) — bit-idêntico ao kernel AVX2; **sem** abrir o banco em modo de escrita, **sem** re-embed. Real-text self-match via `nomic-embed-text` (localhost:11434). |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #performance #vector #recall #ndcg #duplicates #sqlite #read-only #baseline #deque |
| **Related** | internal/vector/sqlite_vec.go, internal/vector/index.go, internal/knowledge/campaign_recall_test.go, docs/reports/vector-recall-baseline-2026-08-24.md |
| **Measured** | índice: 13.217 vetores dim 768 flat_bruteforce, 0 dim-mismatch. Duplicatas: 23% por BLOB (10.510 distintos), 20.4% por conteúdo, MAX 61 cópias de UM boilerplate ("Leia o AGENT_PRIMER.md…") em 61 documentos, 239 grupos cross-doc, 16.4% do índice em grupos >10. Recall vs float32 oracle (30 self-match, K=50): **int16 (produção)** set-recall@10=1.0, ranking-recall@10=1.0, NDCG@10=1.0, jaccard@50=0.992, top-1 nunca muda, err 0.0004; **int8** ranking-recall@10=0.97, jaccard@50=0.931. Score top-10: mean 0.846, med 0.803, sd 0.125, gap top1→10 med 0.248, **min 0.0**; 25.6% dos ranks 2–10 a ≥0.99; quase-empates (≥0.99) no top-50 = 21.5%. Real-text self-match: top-1 76% (8% colisão duplicata, **16% score próprio <0.9 = vetor de OUTRO provider**), top-1 sempre 1.0. |
| **Learned** | A quantização NÃO é o problema (int16 é bit-fiel ao oracle). O que degrada a qualidade percebida: (1) **duplicatas** (dedup de ingestão INATIVO — colunas `is_trivial`/`dedup_of` ausentes → fail-safe do indexer desliga o dedup); (2) **heterogeneidade de provider** (~16% de vetores não-nomic → sub-espaço impuro, distances incomparáveis); (3) **score cru não-calibrado** — o ranker multi-fator nunca roda no caminho layered (sub-queries de fonte única → `sourceCount>1` nunca verdadeiro), `mergeRanked` ordena `Score` cru sem normalizar, desempate por `id asc` nos ≥0.99. Dominância: o problema é de **dado + ausência de ranking**, não de throughput (slab ≈73MB < budget L3 80MB → full-scan em regime cache). |
| **Next** | Level 4: medir recall/NDCG pós-dedup (simulação dry-run, sem mutar) e quantificar o ganho de calibrar/rankear o score; reproduzir contaminação de provider (identificar os chunk_ids >0.9). |

### 2026-08-24 — Benchmark v3 FULL-SCAN vs ROTEADO (recall em 2 níveis, §10.3)

| Field | Value |
|-------|-------|
| **Agent** | cosca-performance |
| **Task** | Executar o benchmark v3 Full-Scan vs Roteado com recall em 2 níveis (RECALL_DOCUMENT@K document_id principal + RECALL_CHUNK@K chunk_id secundário) sobre query set pré-registrado |
| **Technique** | Test-only (`internal/search/vectorevidence_bench_test.go`), leitura `mode=ro`, FULL-SCAN `Scope=nil`+`CandidateIDs` vazio vs ROTEADO `ApplyScope`+`CandidateIDs` por path-segment; `EnableGraph=false`, `CandidatePool=0`, mesmo Limit; exaustividade `COUNT(JOIN)==len`. Embedding via Ollama `nomic-embed-text` (768). GT (document_id + chunk_id) pré-registrado ANTES das buscas. |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #performance #vector #benchmark #recall #routing #fullscan #test-only #read-only |
| **Related** | internal/search/vectorevidence_bench_test.go, docs/reports/vectoragg-benchmark-design-2026-08-24.md §10.3/10.4 |
| **Measured** | 6 queries (runtime/memory/knowledge/architecture/cli/security), todas VALID (gate exact, exaustividade true). Corpus 28.888 vetores. → ver tabela no relatório da sessão. |
| **Learned** | `vectors.id == chunks.id` (1:1 por chunk) no corpus → `SearchResult.ID` == chunk_id, então RECALL_CHUNK@K compara `results[i].ID == gt.chunk_id`. modlink casa trigger por whole-word accent/case-insensitive (canonicalTokens) → gate exact exige módulo único; palavra "sessões" roteia p/ runtime. Rotas de architecture/cli/security precisaram ser adicionadas (não existiam) na instrumentação test-only. |
| **Next** | Reportar os números crus e consolidar na doc de PROVENANCE; avaliar por que routed=0 em RECALL_DOCUMENT nos casos em que full>0. |

## Confidence Estimate

| Domain | Baseline Confidence | Current Confidence | Evidence |
|--------|--------------------|--------------------|---------|
| **Go profiling** | 0.25 | **0.65** | 34 benchmarks executed across 3 packages; real scaling data; allocation profiles |
| **SQLite optimization** | 0.25 | **0.60** | EXPLAIN patterns identified; schema bug found; N+1 patterns documented |
| **Vector search** | 0.25 | **0.70** | Brute-force scaling confirmed empirically; encoding/decoding measured; batch insert characterized |
| **FTS5** | 0.25 | **0.50** | Schema analysis complete; chunks_fts measured; documents_fts blocked by bug |
| **Dashboard performance** | 0.25 | **0.55** | Query patterns analyzed; caching strategy recommended; COUNT(*) measured |

**Composite confidence**: 0.60 (up from 0.25 baseline) — significant improvement via empirical measurement. Gated by FTS5 schema bug preventing full document-level benchmarks.

