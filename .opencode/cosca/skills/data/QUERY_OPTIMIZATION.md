---
name: query-optimization
description: Use when the user asks to analyze and optimize database queries, including plans, indexes, and rewrites.
---

# QUERY OPTIMIZATION

> **Version**: 1.0.0 | **Status**: active | **Owner**: Database Chief | **Last Updated**: 2026-07-27

## Purpose
Analyze and optimize SQLite queries for performance, proper index usage, and efficient execution plans.

## Inputs
| Input | From | Format | Required |
|-------|------|--------|----------|
| Query source files | `internal/*/` Go source | Go code | Yes |
| Schema + indexes | `internal/sqlite/schema.go` | Go source | Yes |
| EXPLAIN output | SQLite EXPLAIN QUERY PLAN | Text | Yes |

## Outputs
| Output | To | Format | SLA |
|--------|-----|--------|-----|
| Optimization report | Database Chief | Markdown | 1 session |
| Refactored queries | Source files | Go code | 1 session |
| New indexes (if needed) | `internal/sqlite/migrations.go` | Go code | 1 session |

## Process
1. **Query Inventory**: Scan `internal/` for all SQL queries. Build a query index categorized by table and operation (SELECT/INSERT/UPDATE/DELETE).
2. **EXPLAIN Analysis**: Run `EXPLAIN QUERY PLAN` on each query. Flag any `SCAN TABLE` (full table scan) without index usage.
3. **Index Coverage Check**: For each `SCAN TABLE`, determine if an index can be added or if the query can be restructured.
4. **FTS5 Optimization**: Verify FTS5 queries use the correct virtual table syntax and tokenizer configuration.
5. **Parameterization**: Ensure all queries use `?` placeholders, never string concatenation.
6. **Rewrite**: Propose optimized queries. Use subqueries, CTEs (WITH clause), or index hints where appropriate.
7. **Benchmark**: If possible, benchmark before/after query times.

## Success Criteria
- All full table scans identified and addressed
- EXPLAIN output shows index usage for all frequent queries
- Zero unparameterized queries
- Query response time improved or maintained

## Related
- [DATABASE_AUDIT](DATABASE_AUDIT.md)
- [DATA_MIGRATION_PLANNING](DATA_MIGRATION_PLANNING.md)
- [../performance/DATABASE_PERFORMANCE.md](../performance/DATABASE_PERFORMANCE.md)
