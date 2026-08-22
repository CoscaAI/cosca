# WORKFLOW: deployment

> **Version**: 1.0.0 | **Status**: active | **Category**: deploy | **Last Updated**: 2026-07-10

## OBJECTIVE
Deploy application to a target environment with zero-downtime strategy, health verification, and automated rollback capability.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| environment | string | Yes | Target environment (staging, production) |
| version | string | Yes | Version or commit hash to deploy |
| strategy | string | No | Deployment strategy (blue-green, canary, rolling) |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| deployment_id | string | Unique deployment identifier |
| status | string | Deployment status (success, failed, rolled_back) |
| verification_report | object | Health check and smoke test results |

## PRECONDITIONS
1. Build artifacts available
2. Target environment accessible
3. Database migrations ready (if applicable)
4. Rollback plan documented

## POSTCONDITIONS
1. New version serving traffic
2. Health checks passing
3. Old version drained (blue-green) or scaled down (rolling)
4. Monitoring configured for new version

## DEPENDENCIES
| Workflow | Reason |
|----------|--------|
| release | Deployment typically follows release workflow |

## STEPS

### Step 1: Pre-Deployment Checks
- **Chief**: DevOps
- **Specialists**: CI/CD Engineer
- **Task**: Verify artifacts, environment access, migration readiness
- **Output**: Pre-flight check report

### Step 2: Database Migrations
- **Chief**: Database
- **Specialists**: Data Migration Engineer
- **Task**: Run pending migrations (if any)
- **Output**: Migration completion report

### Step 3: Deploy New Version
- **Chief**: DevOps
- **Specialists**: Container Engineer, Infrastructure Engineer
- **Task**: Execute deployment strategy (blue-green/canary/rolling)
- **Output**: Deployment execution log

### Step 4: Health Verification
- **Chief**: Monitoring
- **Specialists**: Monitoring Engineer
- **Task**: Run health checks, smoke tests, verify metrics
- **Output**: Health verification report

### Step 5: Traffic Cutover
- **Chief**: DevOps
- **Specialists**: Infrastructure Engineer
- **Task**: Route traffic to new version, drain old version
- **Output**: Traffic cutover confirmation

### Step 6: Post-Deployment Monitoring
- **Chief**: Monitoring
- **Specialists**: Alert Engineer
- **Task**: Monitor for 15 minutes for errors, latency spikes, anomalies
- **Output**: Post-deployment monitoring report

### Step 7: Rollback (if needed)
- **Chief**: DevOps
- **Specialists**: Release Engineer
- **Task**: Execute rollback to previous version
- **Output**: Rollback confirmation

## VALIDATION
1. All health checks green
2. Error rate within normal range
3. Latency within acceptable thresholds
4. Smoke tests passing
5. Database migrations applied successfully

## SUCCESS CRITERIA
- [ ] New version serving traffic
- [ ] Health checks passing for 15+ minutes
- [ ] No critical alerts triggered
- [ ] Rollback plan verified (or executed if needed)

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Health check failure | Retry deploy, escalate if persistent |
| Migration failure | Rollback migration, abort deploy |
| Post-deployment errors | Assess severity; auto-rollback if critical |
| Infrastructure failure | Escalate to Infrastructure Chief |

## RELATED
- [Release Workflow](release.md)
- [DevOps Chief](../departments/devops/SKILL.md)
- [Infrastructure Chief](../departments/infrastructure/SKILL.md)
- [Monitoring Chief](../departments/monitoring/SKILL.md)

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Initial deployment workflow |
