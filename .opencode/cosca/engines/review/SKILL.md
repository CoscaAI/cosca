> **Version**: 1.0.0 | **Status**: active | **Owner**: Review Engine | **Last Updated**: 2026-07-10

# REVIEW ENGINE

## PURPOSE
The Review Engine provides automated and semi-automated review of all deliverables. It enforces standards, identifies issues, and ensures quality before code reaches QA.

For canonical review criteria, thresholds, and scoring, see [QUALITY_GATES.md](../../QUALITY_GATES.md) Gate 2.

## REVIEW PROCESS

```
Deliverable
    ↓
Automated Checks (linting, typing, tests)
    ↓
Architecture Review
    ↓
Code Quality Review
    ↓
Security Review
    ↓
Performance Review
    ↓
Test Review
    ↓
Documentation Review
    ↓
Review Report → APPROVED / CHANGES REQUESTED / REJECTED
```

## REVIEW REPORT FORMAT
```markdown
# REVIEW REPORT — [Deliverable]

## STATUS: [APPROVED | CHANGES REQUESTED | REJECTED]

## SUMMARY
- Files reviewed: [N]
- Issues found: [N]
- Critical: [N]
- Warnings: [N]
- Suggestions: [N]

## CRITICAL ISSUES
1. [File:Line] — [Issue] — [Recommendation]

## WARNINGS
1. [File:Line] — [Issue] — [Recommendation]

## SUGGESTIONS
1. [File:Line] — [Suggestion]

## COMPLIANCE SCORE
- Architecture: [X/10]
- Code Quality: [X/10]
- Security: [X/10]
- Performance: [X/10]
- Tests: [X/10]
- Documentation: [X/10]
- OVERALL: [X/10]

## REQUIRED ACTIONS
- [ ] [Action item]
```

## INTEGRATION
- Triggered by Execution Engine after task completion
- Uses Review Chief and specialists
- Feeds results to QA Engine
- Stores review history in Memory Engine

## DEPENDENCIES

| File | Purpose |
|------|---------|
| QUALITY_GATES.md | Canonical review criteria and thresholds |

## RELATED
- [QUALITY_GATES.md](../../QUALITY_GATES.md) — Canonical review criteria, thresholds, and scoring
- [Review Chief](../../departments/review/SKILL.md) — Orchestrates review specialists
- [Quality Engine](../quality/SKILL.md) — Consumes review results for quality gates
- [QA Chief](../../departments/qa/SKILL.md) — Receives review reports for QA sign-off

## HISTORY

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-07-10 | Initial version. Extracted review criteria to QUALITY_GATES.md. |
