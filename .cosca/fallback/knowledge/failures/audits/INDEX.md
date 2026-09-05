# Audit Reports

> **Category**: Failures → Audits | **Version**: 1.0.0 | **Owner**: Cosca QA Chief | **Last Updated**: 2026-07-29

## Purpose

Audit reports — systematic evaluations of code quality, coverage, compliance, and architecture conformance. Audits are triggered by Don orders, scheduled reviews, or gate violations.

## Audit Reports

| # | File | Title | Scope | Date | Status | Executed By |
|---|------|-------|-------|------|:------:|-------------|
| 1 | [`coverage-audit-2026-07-29.md`](coverage-audit-2026-07-29.md) | Cosca Runtime Coverage Audit | Runtime coverage, test quality, threshold compliance | 2026-07-29 | Completed | cosca-kernel + cosca-discovery + cosca-qa |

## Audit Details

### Coverage Audit 2026-07-29

**Trigger**: Don order ("audit the cosca runtime to see how test coverage is")

**Key Findings**:
- Runtime core coverage: 97.9% (excellent)
- `pkg/cosca`: 80.0% (good)
- `internal/cli`: 71.5% (needs improvement)
- `api/rest/handler`: 18.8% (critical gap)
- `pkg/cosca/jail.go`: 0% (6 security functions with zero tests)
- 4 conflicting coverage thresholds found across CI/config/docs

**Outcome**: Triggered coverage regression playbook, threshold reconciliation, and Onda 2 specialist activation.

---

*Audits are the primary discovery mechanism for bugs and quality gaps. Per heuristic H-012, formal registries underestimate real issues — audits discover what registries miss.*
