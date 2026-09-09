---
name: quality
description: Enforces quality gates across all deliverables - standards, automated checks, and metrics.
level: 3
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Quality Engine | **Last Updated**: 2026-07-10

# QUALITY ENGINE

## PURPOSE
The Quality Engine enforces quality gates across all deliverables. It defines standards, runs automated checks, and provides quality metrics.

See [QUALITY_GATES.md](../../identidade/QUALITY_GATES.md) for canonical gate definitions and metric thresholds.

## QUALITY GATES

### Gate 1: Pre-Commit
- Linting passes
- Type checking passes
- Unit tests pass
- No secrets detected

### Gate 2: Pull Request
- Architecture review passed
- Code review passed
- Test coverage maintained or improved
- No new vulnerabilities introduced
- Performance benchmarks not degraded

### Gate 3: Pre-Release
- All tests pass (unit, integration, e2e)
- QA sign-off obtained
- Security scan clean
- Performance benchmarks passed
- Documentation updated

### Gate 4: Post-Release
- Monitoring confirms health
- No new errors in logs
- Performance metrics stable
- User feedback positive

## QUALITY REPORT FORMAT
```markdown
# QUALITY REPORT — [Project/Feature]

## OVERALL GRADE: [A|B|C|D|F]

## CODE QUALITY
- Complexity Score: [X/10]
- Maintainability Index: [X]
- Technical Debt Ratio: [X%]
- Duplication: [X%]

## TEST QUALITY
- Line Coverage: [X%]
- Branch Coverage: [X%]
- Test Count: [N]
- Flaky Tests: [N]

## PERFORMANCE
- API p50: [Xms]
- API p99: [Xms]
- Bundle Size: [XKB]

## SECURITY
- Vulnerabilities: [N] (Critical: [N], High: [N])
- SAST Findings: [N]

## DOCUMENTATION
- Completeness Score: [X/10]

## RECOMMENDATIONS
1. [Improvement]
```

## INTEGRATION
- Triggered by Review Engine after review
- Triggered before release by Release Chief
- Provides metrics to Monitoring Engine
- Stores quality data in Memory Engine

## DEPENDENCIES

| File | Purpose |
|------|---------|
| QUALITY_GATES.md | Canonical gate definitions |

## RELATED
- [QUALITY_GATES.md](../../identidade/QUALITY_GATES.md) — Canonical gate definitions and metric thresholds
- [Review Engine](../review/SKILL.md) — Provides review input to quality gates
- [QA Chief](../../departments/qa/SKILL.md) — Consumes quality reports for sign-off

## HISTORY

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-07-10 | Initial version. Extracted metric definitions to QUALITY_GATES.md. |
