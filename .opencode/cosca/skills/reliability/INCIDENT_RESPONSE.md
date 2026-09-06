---
name: incident-response
description: Use when the user asks to respond to an incident following detection, triage, containment, resolution, and post-mortem.
---

# INCIDENT RESPONSE — Enterprise Grade

> **Version**: 2.0.0 | **Status**: active | **Owner**: Monitoring Chief | **Last Updated**: 2026-07-26

## Description
Structured incident response for Cosca. Detection → Triage → Containment → Resolution → Post-mortem. With Cosca-specific scenarios, severity SLA, and runbook references.

## Severity SLA
| Level | Response | Resolution | Example |
|-------|----------|------------|---------|
| SEV0 | 5 min | 1 hour | API down, data loss, active breach |
| SEV1 | 15 min | 4 hours | Major feature broken |
| SEV2 | 30 min | 24 hours | Degraded, workaround exists |
| SEV3 | 2 hours | 1 week | Minor bug |

## Cosca Scenarios

### 1. REST API Down
- Detect: health check fails, Prometheus up=0
- Triage: kubectl logs, check OOMKilled, check panics
- Contain: increase memory OR rollback last deploy
- Resolve: fix + verify health 200
- Post-mortem: memory leak? unhandled panic?

### 2. Knowledge Index Corruption
- Detect: search empty/error, cosca doctor FTS5 errors
- Triage: sqlite3 knowledge.db "PRAGMA integrity_check"
- Contain: stop indexing, serve from cache
- Resolve: restore backup OR cosca knowledge sync --force
- Post-mortem: unclean shutdown? WAL checkpoint gap?

### 3. JWT Token Leak
- Detect: security alert, unusual token usage
- Triage: audit logs for token patterns
- Contain: ROTATE secret immediately, invalidate all tokens
- Resolve: regenerate API keys, notify users
- Post-mortem: where was secret exposed? git? log? config?

### 4. WASM Plugin Crash
- Detect: runtime error in plugin execution
- Triage: check plugin logs, wazero panic trace
- Contain: disable offending plugin, fallback to Go native
- Resolve: update plugin or fix sandbox permissions
- Post-mortem: input validation gap? resource limit?

### 5. SQLite Database Locked
- Detect: "database is locked" errors in API
- Triage: check WAL mode, concurrent writes
- Contain: switch to WAL mode if not set, retry with backoff
- Resolve: PRAGMA journal_mode=WAL, PRAGMA busy_timeout=5000
- Post-mortem: missing WAL mode? too many concurrent writers?

## Response Process
1. ACKNOWLEDGE within SLA
2. ASSESS severity using criteria
3. ASSIGN Incident Commander (IC)
4. INVESTIGATE: logs, metrics, traces
5. CONTAIN: stop bleeding first, fix later
6. RESOLVE: permanent fix after testing
7. COMMUNICATE: status updates per SLA
8. POST-MORTEM: within 48h, blameless

## Post-Mortem Template
```markdown
# Incident Post-Mortem: [Title]
- Date: YYYY-MM-DD | Duration: Xh Ym | Severity: SEV0/1/2/3
- IC: [name] | Status: Resolved

## Timeline (UTC)
- 14:00 - Alert triggered
- 14:05 - IC acknowledged
- 14:15 - Root cause identified
- 14:30 - Fix deployed
- 14:35 - Service restored

## Root Cause
[5 Whys analysis]

## Impact
- Users affected: N
- Duration: X minutes
- Data loss: Yes/No (details)

## Action Items
- [ ] [Owner] Short-term fix - by YYYY-MM-DD
- [ ] [Owner] Long-term prevention - by YYYY-MM-DD
```

## Process
1. **Detect**: Monitor alerts (Prometheus metrics, log errors, health check failures). Classify severity (SEV1-SEV4).
2. **Acknowledge**: Acknowledge alert within SLA (SEV1: 5min, SEV2: 15min, SEV3: 1hr, SEV4: 24hr).
3. **Triage**: Assess impact — affected users, data loss risk, revenue impact. Assign incident commander.
4. **Contain**: Implement immediate mitigation (rollback, feature flag off, circuit breaker, rate limit).
5. **Investigate**: Gather logs, metrics, traces. Identify root cause using 5-Whys or fishbone analysis.
6. **Resolve**: Deploy fix. Verify with monitoring. Confirm incident resolved.
7. **Post-Mortem**: Write incident report within 48hr (timeline, root cause, impact, action items). Update runbooks.
8. **Follow-Up**: Track action items to completion. Update monitoring to detect similar incidents earlier.
