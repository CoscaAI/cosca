# DATA MIGRATION PLANNING

> **Version**: 1.0.0 | **Status**: active | **Owner**: Database Chief | **Last Updated**: 2026-07-27

## Purpose
Plan and document database schema migrations with rollback strategies.

## Inputs
| Input | From | Format | Required |
|-------|------|--------|----------|
| Current schema | `internal/sqlite/schema.go` | Go source | Yes |
| Proposed changes | Feature specification | Markdown | Yes |
| Migration history | `internal/sqlite/migrations.go` | Go source | Yes |

## Outputs
| Output | To | Format | SLA |
|--------|-----|--------|-----|
| Migration plan | Database Chief | Structured | 1 session |
| Up/Down SQL | `internal/sqlite/migrations.go` | Go code | 1 session |
| Rollback procedure | Database Chief | Markdown | 1 session |

## Process
1. **Current State**: Load `internal/sqlite/schema.go` and identify the latest migration version from `migrations.go`.
2. **Impact Analysis**: Map proposed schema changes to existing tables, indexes, FTS5 virtual tables, and triggers.
3. **Design Migration**: Write `UpSQL` with `CREATE/ALTER TABLE IF NOT EXISTS` patterns. Write `DownSQL` for complete rollback.
4. **Checksum**: Generate SHA-256 checksum via `computeChecksum()` for the UpSQL.
5. **Register**: Add the new `Migration` struct (Version, Name, UpSQL, DownSQL, Checksum) to `defaultMigrations()`.
6. **Test Plan**: Define test cases: apply migration, verify schema, rollback, verify revert, re-apply.
7. **Documentation**: Update `docs/architecture/database-architecture.md` with schema changes.

## Success Criteria
- Migration registered with unique Version number
- UpSQL uses IF NOT EXISTS for idempotency
- DownSQL completely reverts the migration
- Checksum computed and stored
- Test plan covers apply and rollback paths

## Related
- [DATABASE_AUDIT](DATABASE_AUDIT.md)
- [QUERY_OPTIMIZATION](QUERY_OPTIMIZATION.md)
- [../../memory/architecture/database-architecture.md](../../memory/architecture/database-architecture.md)
