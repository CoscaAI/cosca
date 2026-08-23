---
name: policy
description: Single source of truth for business rules, operational policies, and governance constraints.
level: 3
---

# POLICY ENGINE

> **Version**: 1.0.0 | **Status**: active | **Owner**: Policy Engine | **Last Updated**: 2026-07-12

## PURPOSE
The Policy Engine is the single source of truth for all business rules, operational policies, and governance constraints in the Cosca ecosystem. It evaluates policies at decision points and enforces them consistently. No policy shall be hardcoded in agent logic — all policies flow through this engine.

**Today, policies are scattered across QUALITY_GATES.md, GOVERNANCE.md, and individual SKILL.md files. This engine centralizes, versions, and enforces them.**

## ACTIVATION
- **On decision point**: Before any significant action (deploy, merge, release, delete)
- **On quality gate**: As part of Gate 0-4 enforcement
- **On agent action**: Before executing sensitive operations
- **On policy change**: Re-evaluate affected decisions

## SCOPE
- Centralized policy registry (all policies in one place)
- Policy evaluation engine (rule engine with precedence)
- Policy versioning and audit trail
- Override mechanism with mandatory justification
- Policy categories (deployment, security, quality, cost, compliance)
- Conflict detection (two policies cannot contradict)
- Policy simulation ("what if this policy existed?")

## OUT OF SCOPE
- Policy creation authority (delegated to respective chiefs)
- Policy enforcement implementation (Policy Engine says NO; chiefs implement the NO)
- Legal compliance interpretation (delegate to Security Chief)
- Performance of policy evaluation (cached, async where possible)

## PROCESS

### 1. Policy Registration
```
Chief proposes policy:
  → Policy Engine validates format and uniqueness
  → Checks for conflicts with existing policies
  → Assigns unique policy_id
  → Versions the policy (semver)
  → Logs in policy audit trail
```

### 2. Policy Evaluation
```
Decision point reached (e.g., "deploy to production"):
  → Policy Engine loads all active policies in scope
  → Evaluates each policy rule against current context
  → Aggregates results: ALLOW / DENY / WARN
  → If DENY: returns blocking policy + override procedure
  → Logs evaluation in decision audit
```

### 3. Policy Override
```
Override requested:
  → Validates requestor has override authority
  → Requires mandatory justification
  → Creates override record with TTL
  → Notifies policy owner
  → Logs in audit trail
```

## POLICY SCHEMA

```yaml
policy:
  id: POL-001
  name: "No Friday Production Deployments"
  version: 1.0.0
  status: active
  category: deployment
  severity: error       # error (block) | warn (allow with warning) | info (log only)
  owner: Release Chief
  approved_by: CTO
  
  rule:
    condition:
      type: day_of_week
      operator: equals
      value: "Friday"
    scope:
      environments: [production]
      workflows: [deployment, release]
    
  action: deny
  
  override:
    allowed: true
    required_role: CTO
    justification_required: true
    max_override_ttl: 24h
    
  message: |
    Production deployments are blocked on Fridays to prevent
    weekend incidents. Override requires CTO approval.
    
  exceptions:
    - condition: "severity == 'critical' AND type == 'security_fix'"
      action: allow
```

## POLICY CATALOG

### Deployment Policies
| ID | Policy | Severity | Owner |
|----|--------|----------|-------|
| POL-DEPLOY-001 | No Friday production deploys | error | Release Chief |
| POL-DEPLOY-002 | All deploys require rollback plan | error | DevOps Chief |
| POL-DEPLOY-003 | Canary deploy minimum 10% traffic for 5 min | warn | DevOps Chief |
| POL-DEPLOY-004 | Database migrations must be reversible | error | Database Chief |
| POL-DEPLOY-005 | Major version releases require CEO approval | error | Release Chief |

### Quality Policies
| ID | Policy | Severity | Owner |
|----|--------|----------|-------|
| POL-QUAL-001 | Test coverage must be ≥ 80% on changed code | error | QA Chief |
| POL-QUAL-002 | No critical/high CVEs in dependencies | error | Security Chief |
| POL-QUAL-003 | Review score must be ≥ 7.0 to merge | error | Review Chief |
| POL-QUAL-004 | All PRs require at least 1 reviewer approval | error | Review Chief |
| POL-QUAL-005 | Performance benchmarks must not degrade > 10% | warn | QA Chief |

### Security Policies
| ID | Policy | Severity | Owner |
|----|--------|----------|-------|
| POL-SEC-001 | All endpoints must enforce authentication | error | Security Chief |
| POL-SEC-002 | Secrets must never be in code or logs | error | Security Chief |
| POL-SEC-003 | HTTPS enforced on all environments | error | Security Chief |
| POL-SEC-004 | OWASP Top 10 scan must pass before release | error | Security Chief |
| POL-SEC-005 | Dependency audit every 24h | warn | Security Chief |

### Cost Policies
| ID | Policy | Severity | Owner |
|----|--------|----------|-------|
| POL-COST-001 | AI token cost per session ≤ $5.00 | warn | CTO |
| POL-COST-002 | Cloud infra cost alert at 80% budget | warn | Infrastructure Chief |
| POL-COST-003 | Idle resources auto-shutdown after 2h | info | Infrastructure Chief |

### Compliance Policies
| ID | Policy | Severity | Owner |
|----|--------|----------|-------|
| POL-COMP-001 | PII must be encrypted at rest and in transit | error | Security Chief |
| POL-COMP-002 | Audit trail retention ≥ 1 year | error | Security Chief |
| POL-COMP-003 | Data deletion requests processed within 30 days | error | Security Chief |

## CONFLICT RESOLUTION

### Precedence Rules
```
1. Security policies override all others (POL-SEC-*)
2. Error severity overrides warn severity
3. More specific scope overrides broader scope
4. Newer policy version overrides older (same ID)
5. Explicit exception overrides general rule
```

### Conflict Detection
```
On policy registration:
  → Search existing policies for overlapping scope
  → Evaluate both policies against same scenario
  → If different outcomes → CONFLICT
  → Require resolution before activation
```

## OVERRIDE WORKFLOW

```
Decision blocked by policy POL-DEPLOY-001
  ↓
Requestor: "Critical security fix, must deploy"
  ↓
Policy Engine: Override requires CTO approval
  ↓
CTO approves with justification: "CVE-2026-1234 — zero-day patch"
  ↓
Override created:
  - policy_id: POL-DEPLOY-001
  - overridden_by: CTO
  - justification: "CVE-2026-1234 — zero-day patch"
  - ttl: 2h
  - audit_id: OVR-2026-07-12-001
  ↓
Deployment proceeds. Override expires in 2h.
Policy owner (Release Chief) notified.
```

## INPUTS

| Input | From | Format |
|-------|------|--------|
| Decision context | Kernel, Chiefs | Decision request payload |
| Policy definitions | Chiefs, CTO | Policy YAML |
| Override request | Chiefs | Override request with justification |
| Policy change request | Policy owners | Change proposal |

## OUTPUTS

| Output | To | Format |
|--------|-----|--------|
| Policy evaluation result | Requesting agent | ALLOW / DENY / WARN |
| Blocking policy details | Requesting agent | Policy ID + message |
| Override confirmation | Requesting agent | Override record |
| Policy audit event | Audit Engine | Audit event |
| Conflict detection report | Policy owners | Conflict report |

## DEPENDENCIES

| Depends On | Why |
|-----------|-----|
| All Chiefs | Policy ownership and creation |
| Audit Engine | Policy evaluation and override audit trail |
| QUALITY_GATES.md | Quality policies reference gate thresholds |
| GOVERNANCE.md | Policy lifecycle and versioning |
| Security Chief | Security policy authority |
| Release Chief | Deployment policy authority |

## CONSTRAINTS

- Policies must be versioned (semver)
- Policy evaluation must complete in < 100ms (cached)
- Override justifications are mandatory and auditable
- No policy may contradict a security policy (SEC takes precedence)
- Policy changes require owner approval (patch) or CTO (major)
- Deprecated policies retained for audit (never deleted, only deprecated)
- Policy engine itself cannot be bypassed (Kernel enforces this)

## QUALITY CRITERIA

- [ ] All active policies have defined owner
- [ ] No conflicting policies (automated conflict check)
- [ ] Policy evaluation correctly resolves DENY > WARN > INFO
- [ ] Override workflow requires correct authority level
- [ ] All policy evaluations logged in audit trail
- [ ] Policy catalog is searchable and filterable
- [ ] Policy simulation returns accurate results

## RELATED

- [QUALITY_GATES.md](../../QUALITY_GATES.md) — Quality thresholds referenced by policies
- [GOVERNANCE.md](../../GOVERNANCE.md) — Policy lifecycle and versioning
- [Security Chief](../../departments/security/SKILL.md) — Security policy authority
- [Release Chief](../../departments/release/SKILL.md) — Deployment policy authority
- [Audit Engine](../audit/SKILL.md) — Policy evaluation audit trail
- [KERNEL.md](../../KERNEL.md) — Policy enforcement at decision points

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Initial Policy Engine — centralized policy registry, evaluation, conflict detection, override workflow |
