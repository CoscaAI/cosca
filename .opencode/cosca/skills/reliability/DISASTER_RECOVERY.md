---
name: disaster-recovery
description: Use when the user asks to plan or test disaster recovery, including backup strategy, RPO/RTO, failover, and recovery procedures.
---

# DISASTER RECOVERY — Cosca

> **Version**: 2.0.0 | **Status**: active | **Owner**: DevOps Chief | **Last Updated**: 2026-07-26

## Description
Disaster recovery plan for Cosca platform. Covers backup strategy, RPO/RTO targets, recovery procedures, and testing. Cosca uses embedded SQLite — recovery is simpler than distributed databases, but still requires planning.

## RPO / RTO Targets
| Tier | RPO | RTO | Data |
|------|-----|-----|------|
| Knowledge DB | 1 hour | 15 min | knowledge.db (FTS5 + vectors) |
| Memory records | 24 hours | 1 hour | .cosca/memory/ (markdown files) |
| User/auth data | 1 hour | 15 min | users.json, API keys |
| Config | Git-recoverable | 5 min | .cosca/config.yml, .opencode/ |

## Backup Strategy

### 1. SQLite Backup (knowledge.db)
```bash
# Online backup (WAL mode required)
sqlite3 knowledge.db ".backup 'backups/knowledge-$(date +%Y%m%d-%H%M).db'"

# Verify backup
sqlite3 backups/knowledge-*.db "PRAGMA integrity_check"

# Automated via cron/systemd timer
# /etc/cron.hourly/cosca-backup
#!/bin/bash
BACKUP_DIR=/var/backups/cosca
mkdir -p $BACKUP_DIR
sqlite3 /path/to/knowledge.db ".backup '$BACKUP_DIR/knowledge-$(date +%Y%m%d-%H%M).db'"
find $BACKUP_DIR -name "knowledge-*.db" -mtime +7 -delete
```

### 2. Memory Records Backup
```bash
# Memory is markdown files — simple tar
tar -czf backups/memory-$(date +%Y%m%d).tar.gz .cosca/memory/

# Or rsync for incremental
rsync -av --delete .cosca/memory/ backups/memory/
```

### 3. Kubernetes Persistent Volume Backup
```yaml
# Velero backup (if using Kubernetes)
velero backup create cosca-daily --include-namespaces cosca-prod \\
  --include-resources persistentvolumeclaims,persistentvolumes \\
  --storage-location aws-s3 --ttl 720h

# Restore
velero restore create --from-backup cosca-daily
```

## Recovery Procedures

### Scenario 1: Corrupted knowledge.db
```bash
# 1. Stop indexing
cosca runtime stop-indexing

# 2. Restore from latest backup
cp backups/knowledge-$(ls -t backups/ | head -1) knowledge.db

# 3. Verify integrity
sqlite3 knowledge.db "PRAGMA integrity_check"

# 4. Re-sync (catch up changes since backup)
cosca knowledge sync

# 5. Resume indexing
cosca runtime start-indexing
```

### Scenario 2: Deleted .cosca/ directory
```bash
# 1. Re-initialize
cosca install

# 2. Restore memory from backup
tar -xzf backups/memory-*.tar.gz

# 3. Re-index knowledge
cosca knowledge sync --force

# 4. Restore users
cp backups/users.json .cosca/users.json
```

### Scenario 3: Complete cluster loss (Kubernetes)
```bash
# 1. Restore PV from Velero/snapshot
velero restore create --from-backup cosca-daily

# 2. Re-deploy with Helm
helm upgrade --install cosca deploy/helm/cosca/ -f values.prod.yaml

# 3. Verify health
curl https://cosca.example.com/health

# 4. Verify data integrity
cosca doctor
```

## DR Testing Schedule
| Test | Frequency | Owner |
|------|-----------|-------|
| Backup verification | Daily (automated) | DevOps |
| Restore from backup | Monthly | DevOps |
| Full DR simulation | Quarterly | DevOps + QA |
| Cross-region failover | Bi-annual | Infrastructure |

## RTO Validation
```bash
# Time-tagged DR test
echo "DR test started at: $(date)"
# Execute recovery steps
echo "DR test completed at: $(date)"
# RTO = completed - started
```

## Key Rules
1. Backups must be OFFSITE (not on same disk/region as primary)
2. Test restore MONTHLY (backup without restore testing is not a backup)
3. WAL mode REQUIRED for online backups
4. Encrypt backups at rest (especially if containing user data)
5. Document recovery time in post-mortem of every incident

## Process
1. **Asset Inventory**: List all critical assets — database (SQLite file), configs (.cosca/, .opencode/), secrets, runtime state.
2. **RPO/RTO Definition**: Define Recovery Point Objective (max data loss) and Recovery Time Objective (max downtime) per asset.
3. **Backup Strategy**: Configure automated backups (database: daily snapshots, configs: git versioned, secrets: encrypted export).
4. **Recovery Procedure**: Document step-by-step restore process — stop service, restore SQLite from backup, verify integrity, restart.
5. **Failover Plan**: Define multi-region failover if applicable (DNS switch, load balancer redirect, read replica promotion).
6. **Test Schedule**: Schedule quarterly DR drills. Simulate total loss and measure actual recovery time vs RTO.
7. **Documentation**: Maintain DR runbook in knowledge/runbooks/. Update after every architecture change.
8. **Compliance**: Ensure DR plan meets regulatory requirements (data residency, encryption at rest, audit trail).
