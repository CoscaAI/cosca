# Data Migration

> **Version**: 1.0.0 | **Status**: active | **Owner**: Migration Chief | **Last Updated**: 2026-07-27

## Purpose
Design ETL and data transformation pipelines for migrating data between schemas.

## Process
1. Map source schema to target schema: identify transformations needed per column.
2. Design extraction: SELECT with appropriate filters, batch size, ordering.
3. Design transformation: type casting, default values, computed columns, data cleansing.
4. Design loading: INSERT with conflict resolution (REPLACE, IGNORE, UPDATE).
5. Add progress tracking: log every N rows, estimate completion time.
6. Test on copy of production data. Verify row counts match.
7. Document rollback procedure: how to revert data migration if needed.

## Success Criteria
- Row count matches between source and target
- All transformations applied correctly
- Rollback procedure documented and tested
