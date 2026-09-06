---
name: quality-gate
description: Use when the user asks to run quality gate checks (Gate 0-4) on a deliverable before acceptance.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: QA Chief | **Last Updated**: 2026-07-23

# QUALITY GATE SKILL

## Description
Execute quality gate checks as defined in QUALITY_GATES.md. Validates deliverables against Gate 0-4 standards including request validation, plan soundness, implementation quality, release readiness, and post-release health.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| gate_number | Yes | `0`, `1`, `2.1`, `2.2`, `2.3`, `2.4`, `2.5`, `2.6`, `3`, `4` |
| artifacts | Yes | Code, docs, configs, or deliverables to check |
| thresholds | No | Custom pass/fail thresholds (overrides defaults from QUALITY_GATES.md) |
| context | No | Additional context for the quality check (PR number, commit SHA) |

## Outputs
| Output | Description |
|--------|-------------|
| Gate report | Per-check pass/fail with evidence |
| Quality score | Overall score (0-10) per QUALITY_GATES formula |
| Failed checks | List of failed checks with details and fix guidance |
| Decision | Pass, Pass-with-suggestions, Changes-Required, Rejected, Blocked |

## Quality Score Formula
```
OVERALL = (Architecture × 0.20) + (Code Quality × 0.20) + (Security × 0.25) 
        + (Performance × 0.10) + (Testing × 0.15) + (Documentation × 0.10)
```

| Score | Grade | Action |
|-------|-------|--------|
| 9.0-10.0 | A | Approved |
| 7.0-8.9 | B | Approved with suggestions |
| 5.0-6.9 | C | Changes requested |
| 3.0-4.9 | D | Rejected, major rework |
| 0.0-2.9 | F | Blocked, unsafe |

## Process
1. Identify which quality gate to execute based on workflow stage
2. Load gate definitions from QUALITY_GATES.md
3. Collect required artifacts for each check
4. Execute automated checks first (linting, scanning, validation)
5. Report automated results
6. Queue manual review checks for Review Chief
7. Calculate weighted quality score
8. Determine pass/fail based on grade thresholds
9. Generate gate report with findings
10. Store result in memory for audit trail

## Success Criteria
- [ ] All checks in the gate executed
- [ ] Quality score calculated correctly
- [ ] Failed checks documented with fix guidance
- [ ] Decision communicated to stakeholders
- [ ] Result stored in audit trail

## Related
- [QA Chief](../../departments/qa/SKILL.md)
- [QUALITY_GATES.md](../../QUALITY_GATES.md) — Gate definitions
- [Code Review](../../skills/code-quality/CODE_REVIEW.md)
- [Security Audit](../../skills/security/SECURITY_AUDIT.md)
- [Performance Audit](../../skills/performance/PERFORMANCE_AUDIT.md)
