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
