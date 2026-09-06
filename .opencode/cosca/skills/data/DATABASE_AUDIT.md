# DATABASE AUDIT

> **Version**: 1.0.0 | **Status**: active | **Owner**: Database Chief | **Last Updated**: 2026-07-27

## Purpose
Comprehensive audit of database schema, indexes, queries, and performance characteristics.

## Inputs
| Input | From | Format | Required |
|-------|------|--------|----------|
| Schema file | `internal/sqlite/schema.go` | Go source | Yes |
| Migration history | SQLite migration_history table | SQL query | Yes |
| Query patterns | `internal/*/` source files | Go source | Optional |

## Outputs
| Output | To | Format | SLA |
|--------|-----|--------|-----|
| Audit report | `docs/audit/database-*.md` | Markdown | 1 session |
| Index recommendations | Database Chief | Structured | 1 session |
| Query optimization plan | Database Chief | Structured | 1 session |

## Process
1. **Schema Review**: Load `internal/sqlite/schema.go` and enumerate all tables, columns, types, constraints, and triggers.
2. **Index Analysis**: Verify every FOREIGN KEY has a corresponding index. Check for unused or redundant indexes.
3. **FTS5 Verification**: Validate FTS5 virtual tables have correct tokenizer and column configuration.
4. **Query Pattern Scan**: Search `internal/` for raw SQL queries. Flag any without parameterized placeholders (?, ?, ?).
5. **Migration Integrity**: Query `migration_history` table and verify all migrations have matching checksums.
6. **Performance Profile**: Run `EXPLAIN QUERY PLAN` on the top 5 most frequent query patterns.
7. **Report Generation**: Compile findings with severity classification (CRITICAL/HIGH/MEDIUM/LOW).

## Success Criteria
- All tables enumerated with column types verified
- Zero missing FOREIGN KEY indexes
- Zero unparameterized SQL queries found
- All migration checksums verified
- EXPLAIN output documented for top queries

## Related
- [QUERY_OPTIMIZATION](QUERY_OPTIMIZATION.md)
- [DATA_MIGRATION_PLANNING](DATA_MIGRATION_PLANNING.md)
- [../../workflows/performance-audit.md](../../workflows/performance-audit.md)
- [../../memory/architecture/database-architecture.md](../../memory/architecture/database-architecture.md)
