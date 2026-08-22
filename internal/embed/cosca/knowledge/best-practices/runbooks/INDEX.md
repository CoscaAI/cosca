# Runbooks — Routine Operational Procedures

> **Category**: Best Practices → Runbooks | **Version**: 1.0.0 | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-29

## Status: Empty Catalog

This directory is currently empty. No routine operational runbooks have been defined.

## Runbook vs Playbook

| | Playbook | Runbook |
|---|----------|---------|
| **Trigger** | Incident / event | Schedule / routine |
| **Urgency** | Immediate response | Planned execution |
| **Duration** | Variable (15 min – days) | Predictable |
| **Example** | "API is down — restore service" | "Weekly database backup verification" |

## Planned Runbooks

The following runbooks are planned for future implementation:

| Runbook | Frequency | Owner |
|---------|:---------:|-------|
| Weekly Memory Curation Execution | Weekly | Memory Chief |
| Coverage Baseline Update | Per-release | QA Chief |
| Schema Migration Verification | Per-migration | Migration Chief |
| Provider Health Check | Daily | Provider Chief |
| CI/CD Pipeline Validation | Weekly | DevOps Chief |
| Risk Registry Review | Weekly | Kernel |

## Related

- **[Playbooks](../playbooks/INDEX.md)** — Incident response guides (triggered by events, not schedules)
- **[Benchmarks](../benchmarks/INDEX.md)** — Performance baselines used by runbooks for regression detection
- **[Heuristic H-010](../../heuristics/H-010-update-learnings-every-task.yaml)** — "Agents must update learnings after every significant task" — applies to runbook execution logging

---

*Runbooks document routine operational procedures. They are the "standard operating procedures" for Cosca platform operations.*
