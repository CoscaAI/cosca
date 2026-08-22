# WORKFLOW: migration-execution

> **Version**: 1.0.0 | **Category**: migration | **Estimated Duration**: 1-5 days | **Status**: active | **Owner**: Migration Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Execute safe, reversible database or system migrations with zero data loss and minimal downtime. Includes schema changes, data transformations, and rollback procedures.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| migration_plan | Document | Yes | Approved migration plan |
| source_schema | Schema | Yes | Current schema definition |
| target_schema | Schema | Yes | Target schema definition |
| rollback_plan | Document | Yes | Tested rollback procedure |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| Migration result | Success/Fail | Migration execution outcome |
| Data integrity report | Report | Pre/post migration data comparison |
| Rollback script | Script | Executed rollback (if needed) |
| Migration log | Log | Complete execution log |

## PRECONDITIONS
1. Migration plan approved by Architecture Chief and CTO
2. Rollback plan tested in staging
3. Data backups completed before migration window
4. Migration window scheduled during low traffic

## POSTCONDITIONS
1. Data integrity verified post-migration
2. Rollback plan still valid (not executed) or executed and verified
3. Migration documented in memory/architecture
4. Stakeholders notified of completion

## DEPENDENCIES
| Workflow | Reason |
|----------|--------|
| release | Migration may be part of release |

## STEPS
### Step 1: Pre-Migration Validation
- **Chief**: Migration Chief
- **Specialists**: Data Migration Engineer
- **Task**: Verify preconditions, validate backups, confirm migration window
- **Output**: Pre-migration checklist signed off

### Step 2: Staging Execution
- **Chief**: Migration Chief
- **Specialists**: Data Migration Engineer, QA Chief
- **Task**: Run migration on staging, validate results, test rollback
- **Output**: Staging migration report

### Step 3: Production Backup
- **Chief**: Database Chief
- **Specialists**: DBA
- **Task**: Create full backup of all affected data
- **Output**: Backup verification

### Step 4: Production Migration
- **Chief**: Migration Chief
- **Specialists**: Data Migration Engineer, DevOps Chief
- **Task**: Execute migration scripts in production
- **Output**: Execution log

### Step 5: Data Integrity Verification
- **Chief**: Database Chief
- **Specialists**: QA Chief, Data Migration Engineer
- **Task**: Verify data integrity, run validation queries
- **Output**: Data integrity report

### Step 6: Rollback Readiness
- **Chief**: Migration Chief
- **Specialists**: DevOps Chief
- **Task**: Keep rollback ready for 48 hours post-migration
- **Output**: Rollback readiness confirmation

### Step 7: Documentation
- **Chief**: Documentation Chief
- **Specialists**: Technical Writer
- **Task**: Document migration execution, lessons learned
- **Output**: Migration report

## VALIDATION
1. Row counts match pre/post migration
2. Data sampling confirms correct transformation
3. Application integration tests pass
4. Performance benchmarks within acceptable range
5. Rollback script verified

## SUCCESS CRITERIA
- [ ] Migration completed within scheduled window
- [ ] Data integrity verified (100% match)
- [ ] No data loss
- [ ] Application functioning correctly post-migration
- [ ] Rollback not required (or successful if needed)
- [ ] Migration documented in memory

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Migration script error | Stop, assess, rollback if needed |
| Data integrity violation | Immediately rollback, escalate to Database Chief |
| Migration window exceeded | Stop at checkpoint, rollback if partially applied |
| Application errors after migration | Rollback if within 48h window |

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |

## RELATED
- [Migration Chief](../departments/migration/SKILL.md)
- [Database Chief](../departments/database/SKILL.md)
- [Data Migration Planning skill](../skills/data/DATA_MIGRATION_PLANNING.md)
- [release.md](./release.md)
