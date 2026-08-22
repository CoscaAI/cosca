# cosca-database — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 2

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| SQLite (modernc.org/sqlite) | 0.90 | 2 | success | ↑ |
| Schema Inventory & Mapping | 0.85 | 1 | success | → |
| Cross-Reference Validation (memory vs codebase) | 0.90 | 1 | success | ↑ |
| FTS5 Full-Text Search | 0.70 | 1 | success | → |
| sqlite-vec Embeddings | 0.65 | 1 | success | → |
| Migration System | 0.60 | 1 | success | → |
| WAL Mode Tuning | 0.30 | 0 | — | → |
| Query Plan Analysis (EXPLAIN) | 0.20 | 0 | — | → |
| PostgreSQL (aspirational) | 0.40 | 1 | success | ↓ |

## Strengths
- **Reality-first architecture correction**: Detected and fixed the most critical documentation error in the project — that memory claimed enterprise PostgreSQL RDS Multi-AZ when Cosca uses embedded SQLite (modernc.org/sqlite). Replaced aspirational fantasy with accurate architecture.
- **Go module import tracing**: Core technique — verifies memory claims by cross-referencing go.mod imports and actual source code in internal/sqlite/. Never trusts documentation at face value.
- **Complete schema surface mapping**: Traced all SQLite tables (agents, memory, knowledge, providers, skills, workflows, plugins, sessions, config), FTS5 virtual tables for document search, and sqlite-vec extension loading — producing a full access pattern catalog.
- **Search pipeline understanding**: Documents the FTS5 (BM25 ranking) + vector similarity → result fusion pipeline in internal/search/ and graph traversals via internal/graph/.

## Weaknesses
- **No WAL mode performance profiling**: Has not measured concurrent read/write throughput under realistic load or analyzed WAL checkpoint behavior.
- **No query plan analysis**: The active performance bug (bug-005 — slow dashboard query) remains uninvestigated; EXPLAIN QUERY PLAN has not been run.
- **No migration strategy design**: Has not designed migration strategies for the SQLite-to-anything pathway if needed — only mapped the existing system.

## Preferred Strategies
- **go.mod import verification**: Always cross-references go.mod, go.sum, and actual source files before accepting any memory/architecture claims about database technology.
- **Schema surface tracing**: Walks internal/sqlite/schema.go, migrations.go, and fts.go to produce complete table/index/virtual-table catalogs.
- **Access pattern mapping**: Traces Go access patterns through db.go singleton, search pipeline, vector store, and graph traversals to understand the full data flow.

## Known Failure Modes
- None recorded — patterns.md is empty; both learning entries show successful outcomes.

## Evolution Goal
Reach Level 3:
*"Profile SQLite WAL mode under sustained concurrent load, root-cause bug-005 (slow dashboard query) with EXPLAIN QUERY PLAN + pprof, and design a SQLite-to-production migration strategy with benchmarking data — graduating from schema archaeology to performance-informed database engineering."*

## Additive Record — 2026-08-04

| Field | Value |
|-------|-------|
| **Event** | Snapshot execution verification |
| **Assessment** | Read-only |
| **Executed by** | Kernel |
| **Artifact** | `.cosca/backups/knowledge-20260804-013827.db` |
| **Integrity** | `ok` |
| **Mode** | `600` |
