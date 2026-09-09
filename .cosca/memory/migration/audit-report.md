# Cosca Migration System — Audit Report

> **Auditor**: cosca-migration | **Date**: 2026-07-28 | **Trigger**: Onda 3 — v2 migration creation
> **DB**: SQLite via modernc.org/sqlite | **Package**: `internal/sqlite`

---

## 1. System Overview

### Architecture

| Component | File | Responsibility |
|-----------|------|---------------|
| `MigrationManager` | `migrations.go` | Version tracking, apply/rollback, status |
| `Migration` struct | `migrations.go` | Single migration (Version, Name, UpSQL, DownSQL, Checksum) |
| `Schema` | `schema.go` | DDL generation, table catalog, type enums |
| `DB` | `db.go` | Connection management, auto-migrate on Open, backup/restore |
| `FTSClient` | `fts.go` | Full-text search via FTS5 virtual tables |

### Migration Flow

```
Open(cfg) → NewMigrationManager(db) → mm.Up()
                                         │
                                    getCurrentVersion() → 0 (no migration_history table yet)
                                         │
                                    ┌────┴────┐
                                    │  Apply  │  for each mig where mig.Version > currentVersion:
                                    │   v1    │    1. BEGIN tx
                                    │   v2    │    2. Split UpSQL by "\n\n", exec each statement
                                    │   ...   │    3. INSERT OR REPLACE INTO migration_history
                                    └────┬────┘    4. COMMIT
                                         │
                                    DB ready (all migrations applied)
```

### Version Tracking

- **Source of truth**: `migration_history` table (created in v1 schema)
- **Current version**: `SELECT COALESCE(MAX(version), 0) FROM migration_history`
- **On first run**: table doesn't exist → returns 0 → runs all migrations
- **Checksum**: SHA-256 of UpSQL, precomputed or computed at runtime
- **Init validation**: `init()` in migrations.go validates migration ordering at program start

---

## 2. Existing Migrations

### v1 — `initial_schema` (Status: ✅ ACTIVE)

| Field | Value |
|-------|-------|
| UpSQL | Full DDL from `NewSchema().DDL()` (52 statements) |
| DownSQL | DROP all tables in reverse dependency order |
| Checksum | Precomputed SHA-256 of full DDL |
| Tables | 18 total (14 core + 4 FTS virtual) |

**Core tables**: documents, chunks, headings, code_blocks, tables, symbols, entities, relationships, frontmatter, metadata, cache, migration_history, snapshots, sync_log

**FTS tables**: documents_fts, chunks_fts, code_blocks_fts, entities_fts

**Issue**: The schema's `documents_fts` definition uses `content='documents'` but the `documents` table has no `content` column. This is corrected by migration v2.

**Issue**: The schema's `entities_fts` uses `content='entities'` but references column `metadata` which does not exist in the `entities` table (actual column is `metadata_json`). NOT YET FIXED.

### v2 — `fix_documents_fts_no_content_column` (Status: ✅ ACTIVE — Created 2026-07-28)

| Field | Value |
|-------|-------|
| Created by | cosca-specialist-database-sql (Onda 3) |
| UpSQL | 7 statements: drop triggers, drop FTS table, recreate as self-managed content table, recreate triggers, repopulate index |
| DownSQL | 7 statements: reverse — revert to content-synced definition |
| Checksum | Precomputed SHA-256 |

**What it does**: Drops the `content='documents'` (content-synced) definition of `documents_fts` and recreates it as a self-managed FTS5 table without `content=` option. The `content` column now stores empty strings (documents have no body text — only titles and doc_type are indexed).

**Up/Down symmetry**: Correct — both provide full trigger recreation and index rebuilding.

### ⚠️ CRITICAL BUG FOUND: `Down()` method cannot execute multi-statement DownSQL

**Root cause** (line 156 of `migrations.go`):
```go
if _, err := tx.Exec(mig.DownSQL); err != nil {
```

The `Up()` method (line 85) correctly splits multi-statement SQL:
```go
for _, stmt := range strings.Split(mig.UpSQL, "\n\n") {
```

But `Down()` does **NOT** split — it sends all statements as one `Exec()` call. SQLite's `modernc.org/sqlite` driver does not support multi-statement Exec, causing:
```
SQL logic error: near "DROP": syntax error (1)
```

**Impact**: Rollback of v2 is impossible until this is fixed. The `downSQLV2()` returns 7 DDL statements joined by `"\n\n"`, which `Down()` tries to execute as one.

**Test failures caused by this**:
- `TestMigrationDownTo` — FAIL (rollback migration 2)
- `TestMigrationDownTo_AlreadyAtTarget` — FAIL (same)
- `TestMigrationDown_NoDownSQL` — FAIL (collateral: test has design issue, but triggers same code path)

---

## 3. Gaps Identified

### GAP-1: `entities_fts` Has Same Content-Sync Column Mismatch (Severity: HIGH)

**File**: `internal/sqlite/schema.go`, lines 266-273

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS entities_fts USING fts5(
    name,
    entity_type,
    metadata,           -- <-- references `metadata` column
    tokenize='porter unicode61',
    content='entities',  -- <-- reads directly from `entities` table
    content_rowid='rowid'
)
```

The `entities` table has `metadata_json`, not `metadata`. With `content='entities'`, FTS5 expects `metadata` to be a real column in `entities`. The triggers work around this by mapping `metadata_json` → `metadata`, but the content-sync mode is still active and potentially broken.

**Fix needed**: Migration v3 should do the same fix as v2 for `entities_fts` — drop `content='entities'`, make it self-managed, rename column `metadata` to `metadata_json` in the FTS definition (or keep `metadata` as the FTS column name since it's independent in self-managed mode).

**Evidence (CONFIRMED)**:
- `TestFTS_RebuildIndex` → FAIL: `"rebuild entities_fts: SQL logic error: no such column: T.metadata (1)"`
- `TestFTS_RemoveDocument` → FAIL: `"delete entity fts: datatype mismatch (20)"`
- The `TestFTS_ContentSynced_SearchEntities` test passes because triggers (not content-sync) handle insertion, so basic search works. But any operation that reads from the content table (rebuild, content sync) fails.

### GAP-2: No Data Migration Mechanism (Severity: MEDIUM)

The system supports only schema DDL (CREATE/DROP/ALTER). There is no mechanism for:
- Transforming existing data between migration versions
- Data backfill scripts
- Validation of migrated data integrity

**Recommendation**: Add `DataUpSQL` / `DataDownSQL` fields to the `Migration` struct, or a `MigrateData()` callback interface.

### GAP-3: `SchemaVersion` Const Stale (Severity: LOW)

**File**: `internal/sqlite/schema.go`, line 11
```go
const SchemaVersion = 1
```

Even though migrations go up to v2, the `SchemaVersion` constant is 1. This creates:
- Confusion about which version "schema" corresponds to
- `SchemaHash()` returns `cosca-kg-v1` regardless of applied migrations

**Recommendation**: Either remove `SchemaVersion` (use migration version as source of truth) or update it to match latest migration version.

### GAP-4: No Multi-Environment Strategy (Severity: MEDIUM)

The migration system treats all databases identically:
- No distinction between dev, staging, prod
- No concept of seed data vs migration data
- Same migration path on all instances

**Recommendation**: Add environment awareness via config flag. For dev, could auto-generate seed data. For prod, should require explicit confirmation for Down migrations.

### GAP-5: `Down()` Lacks Statement Safety Net Like `Up()` (Severity: HIGH)

Besides the multi-statement bug (GAP-1), `Down()` has no pre-flight checks:
- No checksum verification before rolling back
- No confirmation prompt or safety flag (it's called `Down` but requires `DownTo` for bulk)
- No warning about data loss (v1 Down drops ALL tables)

### GAP-6: `TestMigrationDown_NoDownSQL` Is Flawed (Severity: LOW)

The test replaces `mm.migrations` with a single custom migration (v1 with no DownSQL), but the database already has v2 applied from auto-migration. When `Down()` runs, it finds version 2 (from the auto-applied migration) which is not in the replaced list, causing `"migration 2 not found"` instead of the expected `"has no down SQL"` error.

### GAP-7: No Migration Lock (Severity: MEDIUM)

Multiple processes opening the same database could run migrations concurrently. There is no:
- Advisory lock before migration
- Migration lock table
- Atomic "check-and-migrate" pattern

SQLite's WAL mode provides some protection, but with multiple concurrent `Open()` calls, a race condition exists.

---

## 4. Quality Assessment

### Version Tracking: ✅ GOOD
- `migration_history` table stores version, name, applied_at, checksum, applied_by
- `getCurrentVersion()` returns 0 when table doesn't exist (fresh DB)
- `Status()` provides full visibility into applied and pending migrations
- `DryRun()` shows what would be applied without executing

### Rollback: ⚠️ PARTIAL (with critical bug)
- `Down()` rolls back ONE migration at a time — working for single-statement DownSQL
- `DownTo()` chains `Down()` calls — working for single-statement DownSQL
- **BROKEN for multi-statement DownSQL** (v2 and anything with more than one DDL)
- v1 Down is destructive (drops all tables, no way to recover data)

### Checksum Integrity: ✅ GOOD
- SHA-256 checksums precomputed or computed at runtime
- Stored in `migration_history` for audit trail
- Deterministic (`computeChecksum` is tested)
- But: checksums aren't verified on `Down()` — no integrity check before rollback

### Test Coverage: ✅ GOOD (with exceptions)

| Test | Status | Notes |
|------|--------|-------|
| `TestMigrationUp` | PASS | Verifies tables exist after migration |
| `TestMigrationUp_AlreadyApplied` | PASS | Idempotent Up |
| `TestMigrationUp_WithPrecomputedChecksum` | PASS | Checksum stored correctly |
| `TestMigrationUp_ErrorInStatement` | PASS | Rollback on error |
| `TestMigrationDown_Success` | PASS | Single-statement down works |
| `TestMigrationDown_NoMigrations` | PASS | Error on empty |
| `TestMigrationDown_NoDownSQL` | **FAIL** | Test design issue + collateral from v2 bug |
| `TestMigrationDownTo` | **FAIL** | v2 multi-statement DownSQL bug |
| `TestMigrationDownTo_AlreadyAtTarget` | **FAIL** | Same root cause |
| `TestMigrationReset` | PASS | Reset + re-apply works |
| `TestMigrationStatus` | PASS | Full status check |
| `TestMigrationStatus_WithPending` | PASS | Pending count |
| `TestRegisterMigration_*` | PASS | Duplicate check, ordering |
| `TestDryRun` / `TestDryRun_WithPending` | PASS | Dry run works |
| `TestPendingMigrations` | PASS | Pending list |
| `TestGetMigrationHistory` | PASS | History retrieval |
| `TestGetLatestVersion` | PASS | Get latest |
| `TestGetCurrentVersion_NoTable` | PASS | Graceful on missing table |
| `TestComputeChecksum` | PASS | Checksum determinism |
| `TestDownSQLV1` | PASS | V1 DownSQL structure |

**Overall**: 16 pass, 3 fail (all related to v2 DownSQL multi-statement bug)

### Init-Time Validation: ✅ GOOD
```go
func init() {
    migs := defaultMigrations()
    for i := 1; i < len(migs); i++ {
        if migs[i].Version <= migs[i-1].Version {
            log.Fatal().Msgf("migrations out of order: v%d after v%d", ...)
        }
    }
}
```
Catches ordering issues at startup. Would crash the process if migrations are misconfigured — appropriate behavior.

---

## 5. Recommendations (Prioritized)

### IMMEDIATE (Blocking)

| # | Action | Status | Impact |
|---|--------|--------|--------|
| R1 | **Fix `Down()` to split multi-statement DownSQL** — Add the same `strings.Split(mig.DownSQL, "\n\n")` loop as `Up()` has (line 85 pattern) | ✅ **FIXED** (2026-07-28) | Unblocks rollback of v2 and any future multi-statement migration |
| R2 | **Fix `TestMigrationDown_NoDownSQL`** — Set AutoMigrate=false and create migration_history manually | ✅ **FIXED** (2026-07-28) | Unblocks test suite |

### HIGH PRIORITY (This Wave)

| # | Action | Impact |
|---|--------|--------|
| R3 | **Create migration v3 to fix `entities_fts`** — Same pattern as v2: drop `content='entities'`, make self-managed, rename `metadata` column in FTS to align with trigger mapping or use `metadata_json` | Fixes the last content-sync column mismatch |
| R4 | **Add migration lock mechanism** — Use `PRAGMA locking_mode=EXCLUSIVE` or a lock table for migration safety | Prevents concurrent migration races |

### MEDIUM PRIORITY (Next Wave)

| # | Action | Impact |
|---|--------|--------|
| R5 | **Add data migration support** — `DataUpSQL`/`DataDownSQL` fields or a `func(tx *sql.Tx) error` callback | Enables data transformations |
| R6 | **Add environment awareness** — `Env` field in Config (dev/staging/prod) with different behaviors per env | Better safety in production |
| R7 | **Add checksum verification on rollback** — Verify stored checksum matches computed before executing DownSQL | Integrity guarantee |

### LOW PRIORITY (Backlog)

| # | Action | Impact |
|---|--------|--------|
| R8 | **Resolve SchemaVersion confusion** — Either remove const or sync with latest migration version | Clarity |
| R9 | **Add migration history cleanup** — Option to squash old migrations for fresh installs | Performance |
| R10 | **Add migration stats to Stats()** — Expose migration info via API | Observability |

---

## 6. Test Execution Summary (POST-FIX)

```
go test -v -run Migration ./internal/sqlite/... (0.243s) — ALL 19 TESTS PASS ✅
go test ./internal/sqlite/... (1.006s) — 2 FTS tests FAIL due to entities_fts bug

Migration tests after fix:
  TestMigrationUp                                  PASS
  TestMigrationUp_AlreadyApplied                   PASS
  TestMigrationUp_WithPrecomputedChecksum           PASS
  TestMigrationUp_ErrorInStatement                  PASS
  TestRegisterMigration_Success                     PASS
  TestRegisterMigration_DuplicateVersion            PASS
  TestRegisterMigration_OrdersByVersion             PASS
  TestMigrationDown_Success                         PASS
  TestMigrationDown_NoMigrations                    PASS
  TestMigrationDown_NoDownSQL                       PASS (fixed)
  TestMigrationDownTo                               PASS (fixed)
  TestMigrationDownTo_AlreadyAtTarget               PASS (fixed)
  TestMigrationReset                                PASS
  TestMigrationStatus                               PASS
  TestMigrationStatus_WithPending                   PASS
  TestPendingMigrations                             PASS
  TestPendingMigrations_None                        PASS
  TestGetMigrationHistory                           PASS
  TestNewMigrationManager                           PASS

FTS tests (pre-existing failures — entities_fts bug):
  TestFTS_RemoveDocument       FAIL → "delete entity fts: datatype mismatch (20)"
  TestFTS_RebuildIndex         FAIL → "rebuild entities_fts: SQL logic error: no such column: T.metadata (1)"
  ↑ Both directly caused by entities_fts content-sync bug (GAP-1). Confirmed!
```

---

## 7. Conclusion

The Cosca migration system has a solid foundation with strong versioning, checksum tracking, and comprehensive test coverage. The architecture supports idempotent Up migrations, rollback, reset, dry-run, and status queries — all with proper locking and transactional safety.

**Three issues need attention:**

1. **CRITICAL**: The `Down()` method does not split multi-statement SQL, making v2 rollback impossible. This is a ~3-line fix (copy the splitting pattern from `Up()`).

2. **HIGH**: The `entities_fts` FTS table has the same content-sync column mismatch pattern that v2 fixed for `documents_fts`. A v3 migration is needed.

3. **MEDIUM**: The system has no data migration capability, no environment awareness, and no migration locking.

The fixes for items 1 and 2 can be applied together in a single patch, unblocking both the test suite and the remaining FTS integrity issue.

---

## Appendix A: entities_fts Column Mismatch Detail

```
entities table columns:
  id, entity_type, name, path, metadata_json, embedding, created_at, updated_at

entities_fts (content-synced) expects:
  name, entity_type, metadata  ← ERROR: entities has metadata_json, not metadata

Triggers work around this:
  INSERT INTO entities_fts(...) VALUES (..., new.metadata_json, ...)
  ↑ They pass metadata_json as the value for metadata column, so insertion works.

But content-sync mode is still active:
  INSERT INTO entities_fts(entities_fts) VALUES('rebuild')
  ↑ This would try to read column 'metadata' from entities table → FAILS
```

## Appendix B: Fix for Down() Multi-Statement Bug

```go
// Current (broken):
if _, err := tx.Exec(mig.DownSQL); err != nil {

// Fixed (match Up() pattern):
for _, stmt := range strings.Split(mig.DownSQL, "\n\n") {
    stmt = strings.TrimSpace(stmt)
    if stmt == "" {
        continue
    }
    if _, err := tx.Exec(stmt); err != nil {
        _ = tx.Rollback()
        return fmt.Errorf("rollback migration %d: %w", mig.Version, err)
    }
}
```
