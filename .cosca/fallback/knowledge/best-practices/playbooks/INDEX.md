# Playbooks — Step-by-Step Operational Guides

> **Category**: Best Practices → Playbooks | **Version**: 2.0.0 | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-29

## Purpose

Concrete, executable step-by-step guides for common Cosca scenarios. Unlike workflow definitions (which define structure), playbooks show actual commands, decisions, and outputs.

## Available Playbooks

### Operational Playbooks (Incident Response)

| # | Playbook | Severity | Owner | Description |
|:-:|----------|:--------:|-------|-------------|
| 1 | [Coverage Regression Response](incident-coverage-regression.md) | critical | cosca-qa | Respond when test coverage drops below threshold. Detects conflicting thresholds, identifies 0% coverage gaps, guides remediation. |
| 2 | [Knowledge Drift Response](incident-knowledge-drift.md) | critical | cosca-documentation | Respond when documentation diverges from code reality. Detects fabrication (PostgreSQL fantasy, GDPR fabrication), stale references, broken links. |
| 3 | [Agent Activation Failure Response](incident-agent-activation-failure.md) | high | cosca-kernel | Respond when an agent fails to activate or operate. Diagnoses permission issues, DNA validation failures, activation gate problems. |

### Enterprise Playbooks

| Playbook | Workflow | Duration | Author | Description |
|----------|----------|:--------:|--------|-------------|
| [Migration: Zero to Production](playbook-migration-zero-to-prod.md) | migration-execution | 2–5 days | Migration Chief | Complete migration execution from development to production. Covers planning, execution, validation, rollback. |
| [Compliance Audit GDPR](playbook-compliance-audit.md) | compliance-audit | 3–5 days | Compliance Chief | GDPR compliance audit preparation and execution. Covers data mapping, gap analysis, remediation planning. |
| [Incident Response: 502 API Down](playbook-incident-response.md) | incident-response | 15–120 min | Monitoring Chief | Production API outage response. Covers detection, triage, mitigation, post-mortem. |

## How to Use

1. Select the playbook matching your scenario
2. Follow the steps in order
3. Adapt commands to your specific environment (paths, thresholds, tool names)
4. Check off prerequisites before starting
5. Document lessons learned for playbook improvements

## Incident Sources

These playbooks are grounded in real Cosca incidents:

| Incident | Date | Files | Playbook |
|----------|------|-------|----------|
| Coverage threshold crisis (4 conflicting values) | 2026-07-29 | `memory/audit/coverage-audit-2026-07-29.md`, `memory/testing/coverage.md` | #1 |
| jail.go at 0% coverage (6 security functions) | 2026-07-29 | `pkg/cosca/jail.go` | #1 |
| PostgreSQL fantasy (docs claimed RDS, reality SQLite) | 2026-07-28 | `memory/agent/cosca-database/learnings.md` | #2 |
| GDPR/SOC2/ISO fabrication (fake compliance dates) | 2026-07-28 | `memory/agent/cosca-security/learnings.md` | #2 |
| 63 broken references (doc-validator) | 2026-07-28 | `.github/workflows/scripts/doc-validator.sh` | #2 |
| cosca-paradigm activation gate (3-month requirement) | 2026-07-28 | `memory/agent/cosca-kernel/learnings.md` | #3 |
| opencode.json permission staleness (wrong user paths) | 2026-07-29 | `.opencode/opencode.json` | #3 |
| 51/55 agents without failures.md | 2026-07-29 | `memory/agent/cosca-semantic-memory/learnings.md` | #3 |

---

*Previously v1.1.0 — upgraded to v2.0.0 with renumbering to match new knowledge repository structure.*
