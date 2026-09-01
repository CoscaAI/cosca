# cosca-specialist-database-sql — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| SQL database (schema design, migrations, query optimization) | 0.25 | 0 | — | → |

## Strengths
- SQLite schema design with FTS5 full-text search and sqlite-vec vector search integration
- Versioned, bidirectional migration creation (Up + Down) with checksum verification
- Query optimization using EXPLAIN QUERY PLAN and parameterized queries

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Use modernc.org/sqlite (pure Go, no CGO); PRAGMA journal_mode=WAL for concurrent reads
- Every migration must have Version, Name, UpSQL, DownSQL, and Checksum — register via init()
- Add indexes on foreign keys and frequent query columns; use NOT NULL, UNIQUE, FOREIGN KEY constraints at DB level
- Never use raw string concatenation — always parameterized queries (?); verify with EXPLAIN QUERY PLAN

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
