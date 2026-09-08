# cosca-database - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

# cosca-database — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Session: 2026-07-28 — Reality Audit

### 2026-07-28 — Stack Reality Discovery: PostgreSQL → SQLite
| Field | Value |
|-------|-------|
| **Agent** | cosca-database |
| **Task** | Verify database architecture documentation against actual codebase |
| **Technique** | Level 2 — Cross-reference validation: compared memory/architecture/database-architecture.md (claimed PostgreSQL 16 RDS Multi-AZ, 32 vCPU, Redis 7 ElastiCache, pgvector) against go.mod (modernc.org/sqlite) and internal/sqlite/ source code |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #sqlite #postgresql #architecture #documentation #memory-audit |
| **Related** | memory/architecture/database-architecture.md, internal/sqlite/db.go, go.mod |
| **Learned** | Critical discrepancy found: memory claimed enterprise PostgreSQL infrastructure but Cosca uses embedded SQLite (modernc.org/sqlite — pure Go, CGo-free). Single-file database, no connection pooling needed, no network latency. FTS5 for full-text search, sqlite-vec extension for embeddings. Actual schema: agents, memory, knowledge, providers, skills, workflows, plugins, sessions, config tables. Limitations: single-writer model, no built-in replication. Replaced aspirational PostgreSQL fantasy with accurate SQLite architecture doc. Pattern: always cross-reference memory claims against go.mod imports and actual source code. |
| **Next** | Level 3: WAL mode performance profiling, analyze query plans for slow dashboard query (bug-005), document migration strategy |

### 2026-07-28 — SQLite Schema Mapping
| Field | Value |
|-------|-------|
| **Agent** | cosca-database |
| **Task** | Map all SQLite tables and their Go access patterns |
| **Technique** | Level 1 — Schema inventory: traced internal/sqlite/schema.go, migrations.go, fts.go to catalog all tables, indexes, and FTS virtual tables |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #sqlite #schema #fts5 #migration #go |
| **Related** | internal/sqlite/schema.go, internal/search/search.go, internal/vector/vector.go |
| **Learned** | Schema uses migrations-based approach. FTS5 virtual tables for document content search. sqlite-vec extension loaded at init. Go access via sqlite/db.go singleton pattern. WAL mode used for concurrent reads. Search pipeline: FTS5 (BM25 ranking) + vector similarity → result fusion in internal/search/. Graph traversals via internal/graph/ on entity relationships. |
| **Next** | Level 2: Profile query performance under load, analyze WAL checkpoint behavior |

## Session: 2026-08-29 — DB Integrity Audit (post brainweb/ADR-027)

> Trigger: Don asked to validate DBs remain clean after recent implementations (brainweb/observatório 3D, activity.jsonl, CLI trace plugin, ADR-027). Audit-only, no file mutation.

### Method & Tooling
sqlite3 CLI was NOT on PATH. Used a standalone Go program built in the temp dir against `modernc.org/sqlite v1.56.0` (already in go.mod/go.sum cache). Two binaries: (1) general inspector running `PRAGMA integrity_check`, `PRAGMA foreign_key_check`, table counts, page_count×page_size; (2) consistency query runner for orphan/referential checks. Opened all DBs read-only (`mode=ro`).

### Integrity — ALL PHYSICALLY CLEAN
Every .db committed `PRAGMA integrity_check → ok` and `PRAGMA foreign_key_check → clean (0 violations)`:
- knowledge.db, trace.db, department.db, audit.db, auth_tokens.db, secrets.db, memory/index.db, `.cosca/.cosca/durable.db`, internal/memory/memory/index.db

### Consistency findings (knowledge.db)
- Referential: 0 orphan chunks/code_blocks/headings/tables/symbols/frontmatter/metadata. 0 vectors w/ dangling chunk_id/document_id/entity_id. documents↔chunks↔vectors FTS counts all equal (2387/38854/38854/36659/2905/16536/4135). FTS mirrors match base tables exactly.
- Embedding column `chunks.embedding` is NULL for ALL 38854 rows — BY DESIGN (embeddings live in the separate `vectors` table; chunks.embedding is vestigial/unused). Not corruption.
- 333 documents share duplicate hashes — LEGITIMATE DEDUP: identical content indexed twice under two canonical paths (`.cosca/fallback/...` vs `internal/embed/cosca/...`). `documents.path` is UNIQUE so no PK clash; same content, two paths. Not corruption.
- One real schema wart: `vectors` declares `document_id/chunk_id/entity_id` as plain TEXT with NO foreign key, and `relationships.source_id/target_id` also no FK. So `foreign_key_check` cannot see a break in these — must rely on explicit SQL probes. NOTE for future: consider adding FKs or a lint.

### WAL/SHM & git hygiene
- ALL `*.db-wal` files are 0 bytes (server closed cleanly / checkpointed). `*.db-shm` are 32KB live-but-harmless.
- gitignore correctly ignores `*.db-wal`, `*.db-shm`, `.cosca/cosca.pid`, `.cosca/logs/`, `.cosca/cache/`, `.cosca/knowledge.db`, `.cosca/backups/`, `.cosca/assets/`, `.cosca/evals/recall/`, `*/**/.cosca/`, knowledge.db.
- **GIT DEBT:** `.cosca/trace.db-shm`, `.cosca/audit.db-shm`, `.cosca/department.db-shm` ARE TRACKED in git index (committed 2026-08-22 in 11054b6) despite `*.db-shm` being ignored — gitignore does not untrack already-committed files. These are the working-tree-dirty-on-every-write sidecars the Don wants out. Fix: `git rm --cached`.
- `.cosca/trace.db` is tracked AND currently shows as modified ` M` (server writes events). By policy the immutable db files ARE versioned (only derived knowledge.db is not), so a dirty trace.db is expected when the flight recorder has new events; not a corruption.

### activity.jsonl (new — recordCommandActivity)
- Lives at `.cosca/activity.jsonl`; append-only JSONL; rotates at ~5MB to `activity.jsonl.1` (single backup). Current size 3.5KB, well under threshold, NO `.1` backup yet = correct.
- Writes redacted records only (command NAME, never args) — good security posture (no secret leakage into the log).
- It creates the file inside `.cosca/` (real project dir) so it does NOT pollute the repo beyond .cosca. It IS currently `?? ` (untracked) in git; NOT yet in gitignore. Decision needed: add `.cosca/activity.jsonl` + `.cosca/activity.jsonl.1` to gitignore (transient runtime log) OR consciously version it.

### Nested-duplicate discovery
- `durable.db` (401KB) exists ONLY at `.cosca/.cosca/durable.db` — a nested double `.cosca` (accidental subdir cosca, gitignored via `*/**/.cosca/`). Root `.cosca/durable.db` does NOT exist. The NESTED one is a test/regression artifact (knowledge schema with 0 rows). Harmless but should be cleaned or confirmed.
- `.cosca/memory/index.db` and `internal/memory/memory/index.db` are duplicated memory FTS indexes; both tracked; empty (0 rows) but versioned by policy.
- `family_chain.dat` = 22.2MB and `knowledge-manifest.json` = 460KB BOTH TRACKED in git. These are the immutable core chain (per ADR-013, core IS versioned), so tracking is intentional — but the gitignore comment "core is small" conflicts with the 22MB family_chain.dat. Confirm with Don whether family_chain.dat should stay.

### Verdict
- Physical integrity: PASS (all clean). Logical consistency: PASS (dedup/hash dups and NULL embeddings are by-design, not anomalies).
- DB "limpeza": COULD USE CLEANUP, not a hard fail — main item is removing the 3 tracked `*.db-shm` sidecars (`git rm --cached`) and deciding gitignore status for `activity.jsonl`.

| **Next** | Add `*.db-shm`/`*.db-wal` tracked-sidecar cleanup; decide `activity.jsonl` gitignore policy; consider CLI command `cosca db audit` |

