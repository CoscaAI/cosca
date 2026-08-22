# Incident Catalog

> **Category**: Failures → Incidents | **Version**: 1.0.0 | **Owner**: Cosca Monitoring Chief | **Last Updated**: 2026-07-29

## Status: Empty Catalog

This directory is currently empty. No formal incident records have been filed.

## Incident Playbooks

Operational incident response is handled through playbooks in [`best-practices/playbooks/`](../../best-practices/playbooks/INDEX.md). The following playbooks cover incident types that would generate entries in this catalog:

| Playbook | Incident Type | Severity |
|----------|---------------|:--------:|
| [Coverage Regression Response](../../best-practices/playbooks/incident-coverage-regression.md) | Quality gate failure — coverage drops below threshold | critical |
| [Knowledge Drift Response](../../best-practices/playbooks/incident-knowledge-drift.md) | Documentation/knowledge divergence from code reality | critical |
| [Agent Activation Failure Response](../../best-practices/playbooks/incident-agent-activation-failure.md) | Agent fails to activate or operate in real task | high |
| [Incident Response: 502 API Down](../../best-practices/playbooks/playbook-incident-response.md) | Production API outage | high |

## When to Create an Incident Record

An incident record should be created when:
1. A playbook is activated and executed
2. The incident has a defined start time, impact window, and resolution
3. There are lessons learned that improve playbooks or heuristics

## Related

- **[Playbooks](../../best-practices/playbooks/INDEX.md)** — Step-by-step incident response guides
- **[Bugs](../bugs/INDEX.md)** — Code-level defects (use bug registry for code issues, not incidents)
- **[Heuristic H-019](../../heuristics/H-019-negative-memory-underutilized.yaml)** — "Negative memory is the most valuable and most underutilized learning resource"

---

*Incidents should be filed here when a playbook is activated in response to a real event. Until then, operational guidance lives in playbooks.*
