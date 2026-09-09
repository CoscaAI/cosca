# WORKFLOW: incident-response

> **Version**: 1.0.0 | **Category**: ops | **Estimated Duration**: 15-120 min | **Status**: active | **Owner**: Monitoring Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Respond to production incidents following structured process. Minimize mean-time-to-resolution (MTTR) while ensuring proper documentation, stakeholder communication, and post-incident learning.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| alert_source | String | Yes | How incident was detected (monitoring, user report, automated check) |
| severity | String | Yes | critical, high, medium, low |
| affected_services | String[] | Yes | Services impacted by the incident |
| symptoms | String | Yes | Observed symptoms and impact description |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| Resolution status | String | Resolved, Mitigated, or Monitoring |
| Incident timeline | Document | Chronological log of all actions |
| Root cause analysis | Document | RCA findings |
| Post-mortem | Document | Lessons learned and action items |

## PRECONDITIONS
1. Incident detection mechanism active (monitoring, alerts, user reports)
2. On-call roster available and current
3. Incident response runbooks exist for known scenarios

## POSTCONDITIONS
1. Service restored and verified healthy
2. Root cause identified and documented
3. Post-mortem completed within 48 hours
4. Action items tracked with owners and deadlines

## STEPS
### Step 1: Detection & Acknowledgment
- **Chief**: Monitoring Chief
- **Specialists**: SRE
- **Task**: Acknowledge alert within SLA, create incident ticket
- **Output**: Incident ticket created

### Step 2: Triage & Severity Assessment
- **Chief**: Monitoring Chief
- **Specialists**: Incident Commander
- **Task**: Assess impact, assign severity, declare incident if critical/high
- **Output**: Severity classification

### Step 3: Investigation
- **Chief**: Affected service Chief
- **Specialists**: Service team
- **Task**: Gather logs, metrics, traces; investigate root cause
- **Output**: Investigation findings

### Step 4: Mitigation
- **Chief**: Affected service Chief
- **Specialists**: DevOps, Service team
- **Task**: Apply fix, rollback, or workaround; verify service restoration
- **Output**: Mitigation confirmation

### Step 5: Communication
- **Chief**: CEO (for critical) / Monitoring Chief
- **Specialists**: —
- **Task**: Update stakeholders, status page, affected users
- **Output**: Status updates

### Step 6: Post-Mortem
- **Chief**: Monitoring Chief
- **Specialists**: All involved Chiefs
- **Task**: Conduct root cause analysis, document lessons, assign action items
- **Output**: Post-mortem report

## VALIDATION
1. All affected services confirmed operational
2. Monitoring and alerting functional for restored services
3. Incident timeline complete and accurate
4. Post-mortem action items assigned with owners

## SUCCESS CRITERIA
- [ ] Incident acknowledged within SLA
- [ ] Timeline documented with all actions
- [ ] Root cause identified and verified
- [ ] Service restored within SLO
- [ ] Post-mortem completed within 48h
- [ ] Action items tracked to closure

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Root cause not identified | Escalate to CTO, engage engineering team |
| Service not recovering | Escalate to Infrastructure Chief, activate DR plan |
| Security incident involved | Escalate to Security Chief, follow security incident procedures |
| Post-mortem overdue | Escalate to CTO, block team's next release |

## RELATED
- [Monitoring Chief](../departments/monitoring/SKILL.md)
- [Disaster Recovery workflow](./disaster-recovery.md)
- [Incident Response skill](../skills/reliability/INCIDENT_RESPONSE.md)
- [ENTERPRISE_REDUNDANCY.md](../identidade/ENTERPRISE_REDUNDANCY.md)

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |
