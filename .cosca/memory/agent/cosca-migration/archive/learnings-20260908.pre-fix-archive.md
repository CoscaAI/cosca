# cosca-migration - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-migration — Semantic Learnings

> Auto-evolution memory.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-migration |
| **Task** | Initial capability establishment |
| **Technique** | Standard migration patterns |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #migration #baseline |
| **Learned** | Ready for Level 2. |
| **Next** | Identify first advanced technique |

## Advanced Learnings

### 2026-07-28 — Migration System Audit (Onda 3)
| Field | Value |
|-------|-------|
| **Agent** | cosca-migration |
| **Task** | Full migration system audit for Cosca platform |
| **Technique** | Multi-layer audit: code review + test execution + gap analysis |
| **Level** | 2 → 3 |
| **Confidence** | 0.95 |
| **Outcome** | success |
| **Tags** | #migration #audit #sqlite #fts5 #bug-fix |
| **Learned** | Three key patterns: (1) Migration Down methods must mirror Up's statement-splitting logic for SQLite drivers that don't support multi-statement Exec. Discovered by test failure — `Down()` used single Exec() while `Up()` correctly split by `"\n\n"`. Fixed with 3-line change. (2) FTS5 content-synced virtual tables (`content='table'`) require exact column name matches against the source table. The `entities_fts` definition references `metadata` but `entities` table has `metadata_json`. This was the SAME bug pattern as `documents_fts` v2 fix but for a different column. Confirmed by `TestFTS_RebuildIndex` failure: `"no such column: T.metadata"`. (3) Test design must account for auto-migrate side effects — when tests mutate the migration list, pre-applied migrations (v2 from auto-migrate) can interfere with expected outcomes. |
| **Next** | Create migration v3 to fix `entities_fts` content-sync bug (same pattern as v2). Add migration locking mechanism. Consider data migration support. |

