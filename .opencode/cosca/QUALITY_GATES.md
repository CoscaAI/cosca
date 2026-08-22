# QUALITY GATES — Canonical Definitions

> **Version**: 1.0.0 | **Status**: active | **Owner**: QA Chief

## Purpose
Single source of truth for all quality gates, metrics, and thresholds. Referenced by KERNEL.md, Review Engine, Quality Engine, and all department chiefs. No other file should duplicate gate definitions.

---

## Gate Architecture

```
Gate 0: Pre-Work       →   Is the work request valid?
Gate 1: Pre-Impl       →   Is the plan sound?
Gate 2: Post-Impl      →   Does the code meet standards?
Gate 3: Pre-Release    →   Is the release safe?
Gate 4: Post-Release   →   Is production healthy?
```

---

## Gate 0 — Pre-Work (Request Validation)

Triggered by Kernel before any work begins.

| Check | Threshold | Severity | Automated |
|-------|-----------|----------|-----------|
| Request classification | Must be one of: feature, bug, refactor, architecture, docs, deploy, research, review | Error | Yes |
| Scope defined | At least 1 sentence describing what needs to be done | Error | No |
| Affected departments identified | At least 1 department mapped | Warn | Yes |
| No conflicting active workflow | No other workflow touching same files/modules | Warn | No |

---

## Gate 1 — Pre-Implementation (Plan Validation)

Triggered by Planning Engine after Executive Plan generation.

| Check | Threshold | Severity | Automated |
|-------|-----------|----------|-----------|
| Architecture review | Plan reviewed by Architecture Chief | Error | No |
| Security review | Security implications assessed | Error | No |
| Dependency check | No circular step dependencies in plan | Error | Yes |
| Resource allocation | All required departments available | Warn | No |
| Risk assessment | Top 3 risks identified with mitigations | Warn | No |
| Success criteria | At least 1 measurable criterion defined | Error | Yes |
| Estimation | Effort estimated (XS/S/M/L/XL) per step | Warn | Yes |

---

## Gate 2 — Post-Implementation (Code Quality)

Triggered by Execution Engine after task completion, enforced by Review Engine + Quality Engine.

### 2.1 — Architecture Compliance
| Check | Threshold | Severity |
|-------|-----------|----------|
| Module boundaries respected | No cross-boundary violations | Error |
| Dependency direction | Dependencies flow toward stable abstractions | Error |
| ADR compliance | Code follows documented ADRs | Error |
| Pattern consistency | Same pattern used for same problem type | Warn |

### 2.2 — Code Quality
| Check | Threshold | Severity |
|-------|-----------|----------|
| SOLID principles | No detected violations | Error |
| DRY principle | Duplication < 5% in changed files | Warn |
| Function length | < 50 lines per function | Warn |
| File length | < 300 lines per file | Warn |
| Parameter count | < 5 per function | Warn |
| Cyclomatic complexity | < 10 per function | Warn |
| Cognitive complexity | < 15 per function | Warn |
| Naming clarity | Descriptive, follows conventions | Warn |
| Dead code | 0 instances | Error |
| Commented-out code | 0 instances | Error |
| Magic numbers | 0 instances, use named constants | Warn |

### 2.3 — Security
| Check | Threshold | Severity |
|-------|-----------|----------|
| OWASP Top 10 | 0 violations | Error |
| Hardcoded secrets | 0 instances | Error |
| Input validation | All external inputs validated | Error |
| Output encoding | All outputs properly encoded | Error |
| SQL injection | Parameterized queries only | Error |
| XSS prevention | Context-appropriate encoding | Error |
| CSRF protection | Anti-CSRF tokens on state-changing ops | Error |
| Authentication check | Protected endpoints enforce auth | Error |
| Authorization check | Permission checks on protected resources | Error |
| Dependency audit | 0 critical/high CVEs | Error |

### 2.4 — Performance
| Check | Threshold | Severity |
|-------|-----------|----------|
| N+1 queries | 0 instances | Error |
| Missing indexes | 0 tables without proper indexes | Warn |
| Lazy/eager loading | Correct strategy per use case | Warn |
| Unnecessary allocations | No large objects in hot paths | Warn |
| Synchronous blocking | No sync ops in async contexts | Warn |

### 2.5 — Testing
| Check | Threshold | Severity |
|-------|-----------|----------|
| Line coverage | > 80% on changed code | Error |
| Branch coverage | > 70% on changed code | Warn |
| Happy path tested | Yes | Error |
| Edge cases tested | At least 2 edge cases | Warn |
| Error paths tested | At least 1 error path | Warn |
| Test independence | No test depends on another | Error |
| Test determinism | 0 flaky tests | Error |
| Test execution time | < 5 min for unit tests | Warn |

### 2.6 — Documentation
| Check | Threshold | Severity |
|-------|-----------|----------|
| API docs | All new/changed endpoints documented | Error |
| ADR | Created if architecture decision made | Error |
| README | Updated if project structure changed | Warn |
| Changelog | Entry added for the change | Warn |
| Code comments | Explain "why", not "what" | Warn |
| TODOs/FIXMEs | 0 new instances without issue reference | Warn |

---

## Gate 3 — Pre-Release

Triggered by Release Chief before deployment.

| Check | Threshold | Severity |
|-------|-----------|----------|
| All Gate 2 checks pass | 0 errors | Error |
| QA sign-off | Obtained from QA Chief | Error |
| All test suites pass | Unit, integration, E2E | Error |
| Security scan | 0 critical/high findings | Error |
| Performance benchmarks | Within acceptable range | Error |
| Documentation complete | All Gate 2.6 items done | Error |
| Release notes | Generated and reviewed | Warn |
| Rollback plan | Documented and tested | Warn |
| Monitoring configured | Alerts set for new endpoints | Warn |
| Stakeholder notification | Relevant parties informed | Warn |

---

## Gate 4 — Post-Release

Triggered by Monitoring Chief after deployment.

| Check | Threshold | Severity |
|-------|-----------|----------|
| Health checks | All endpoints healthy | Error |
| Error rate | < 1% increase | Error |
| Latency | < 10% degradation | Error |
| User feedback | No critical reports in first 24h | Warn |
| Memory/CPU | Within normal range | Warn |

---

## Quality Score Calculation

```
OVERALL = (Architecture × 0.20) + (Code Quality × 0.20) + (Security × 0.25) + (Performance × 0.10) + (Testing × 0.15) + (Documentation × 0.10)
```

| Score Range | Grade | Action |
|-------------|-------|--------|
| 9.0 – 10.0 | A | Approved |
| 7.0 – 8.9 | B | Approved with suggestions |
| 5.0 – 6.9 | C | Changes requested |
| 3.0 – 4.9 | D | Rejected, major rework needed |
| 0.0 – 2.9 | F | Blocked, unsafe to proceed |

---

## Related

- [CONSTITUTION.md](CONSTITUTION.md) — Part IV Step 7 reference
- [Review Engine](engines/review/SKILL.md) — Enforces Gates 2.1–2.6
- [Quality Engine](engines/quality/SKILL.md) — Metrics collection and reporting
- [QA Chief](departments/qa/SKILL.md) — Gate sign-off authority
- [Release Chief](departments/release/SKILL.md) — Gate 3 enforcement
- [Monitoring Chief](departments/monitoring/SKILL.md) — Gate 4 enforcement
- [KERNEL.md](KERNEL.md) — Gate 0 enforcement

---

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | QA Chief | Initial canonical quality gates |

---

> **Enforced by**: Review Engine + Quality Engine + Release Chief | **Last reviewed**: 2026-07-10
