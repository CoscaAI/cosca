---
name: backup-testing
description: Use when the user asks to plan or execute backup and restore testing to verify recovery integrity and meet RPO/RTO goals.
---

# Backup Testing

> **Version**: 1.0.0 | **Status**: active | **Owner**: DevOps Chief | **Last Updated**: 2026-07-27

## Purpose
Validate backup integrity and test restore procedures.

## Process
1. Identify backup targets: SQLite database, .cosca/ configs, secrets vault.
2. Execute backup: database snapshot, config tar, encrypted secrets export.
3. Verify backup integrity: checksum validation, file size sanity check.
4. Perform restore drill: restore to clean environment, verify data consistency.
5. Measure recovery time vs RTO target.
6. Document results in DR runbook.

## Success Criteria
- Backup completes without errors
- Checksums match between original and restored
- Recovery time within defined RTO
