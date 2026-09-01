# cosca-database — Evolution Timeline

> Auto-evolution tracking. Records capability level progression.

## Current Level: 3

## Evolution History

| Date | Level | Capability | Trigger |
|------|-------|------------|---------|
| 2026-07-27 | 1 | Baseline capabilities established | Initial audit |
| 2026-07-28 | 2 | Cross-reference validation: memory claims vs go.mod imports + source code | PostgreSQL→SQLite reality correction |
| 2026-07-28 | 1 | SQLite schema inventory: FTS5, sqlite-vec, migration patterns | Codebase scan |
| 2026-08-29 | 3 | DB integrity audit (8 DBs: `PRAGMA integrity_check` + `foreign_key_check`, referential probes, WAL/SHM & git hygiene, activity.jsonl, nested-duplicate discovery) + participação na mineração Qdrant (comparativo document-first vs vetor-first, joelho de perf ~2,8%, payload index) | Pós-brainweb/ADR-027 |
