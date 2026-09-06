---
name: schema-migration
description: Use when the user asks to write or run a schema migration (DDL, versioning, rollback) for a database.
---

# Schema Migration

> **Version**: 1.0.0 | **Status**: active | **Owner**: Migration Chief | **Last Updated**: 2026-07-27

## Purpose
Plan and execute database schema migrations with guaranteed rollback.

## Process
1. Analyze current schema from internal/sqlite/schema.go.
2. Design Up migration: CREATE/ALTER TABLE with IF NOT EXISTS.
3. Design Down migration: exact reverse of Up, tested for complete rollback.
4. Compute checksum: SHA-256 of UpSQL via computeChecksum().
5. Register migration: append Migration struct to defaultMigrations().
6. Test apply: run Up(), verify schema, query data.
7. Test rollback: run Down(), verify schema reverted, no data loss.

## Success Criteria
- Migration applies without errors
- Rollback restores exact previous state
- Checksum verified before and after migration
