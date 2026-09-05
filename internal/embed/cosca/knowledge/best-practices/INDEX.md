# Best Practices Knowledge Domain

> **Category**: Best Practices | **Version**: 1.0.0 | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-29

## Purpose

Operational guidance, compliance assessments, performance benchmarks, and step-by-step playbooks. This domain translates patterns, heuristics, and failure lessons into actionable, executable guidance.

## Directory Structure

```
best-practices/
├── playbooks/       (6 operational + enterprise playbooks)
├── runbooks/        (0 — to be populated)
├── compliance/      (1 GDPR/LGPD assessment)
└── benchmarks/      (3 benchmark reports)
```

## Sub-Directories

| Directory | Description | Files | Index |
|-----------|-------------|------:|-------|
| [`playbooks/`](playbooks/INDEX.md) | Step-by-step operational and enterprise guides | 6 | [playbooks/INDEX.md](playbooks/INDEX.md) |
| [`runbooks/`](runbooks/INDEX.md) | Routine operational procedures (to be populated) | 0 | [runbooks/INDEX.md](runbooks/INDEX.md) |
| [`compliance/`](compliance/INDEX.md) | Regulatory and compliance assessments | 1 | [compliance/INDEX.md](compliance/INDEX.md) |
| [`benchmarks/`](benchmarks/INDEX.md) | Performance and quality benchmarks with baselines | 3 | [benchmarks/INDEX.md](benchmarks/INDEX.md) |

## How to Use

1. **Playbooks** — Use when responding to a specific incident or executing a defined workflow. Each playbook provides prerequisites, step-by-step commands, and escalation paths.

2. **Runbooks** — Use for routine operational tasks (scheduled maintenance, health checks, backup verification). Standard operating procedures for recurring work.

3. **Compliance** — Reference for regulatory requirements (GDPR, LGPD, SOC2). Use during compliance audits or when designing features with data privacy implications.

4. **Benchmarks** — Use to detect performance regressions. Each benchmark defines a baseline and trigger threshold (>20% degradation for runtime, coverage drops below safety margin).

## Statistics

| Category | Count |
|----------|------:|
| Operational playbooks | 3 |
| Enterprise playbooks | 3 |
| Runbooks | 0 |
| Compliance assessments | 1 |
| Benchmarks | 3 |
| **Total** | **7** |

## Relationship to Other Domains

- **Heuristics** — Many playbooks are direct responses to anti-pattern heuristics (e.g., H-009 knowledge drift → knowledge drift playbook)
- **Failures** — Playbooks are activated by incidents; benchmarks detect regressions that could become bugs
- **Patterns** — Enterprise playbooks follow architectural patterns defined in `patterns/`

---

*Best practices transform theoretical knowledge (patterns, heuristics) into executable guidance (playbooks, runbooks). They close the loop from "knowing" to "doing."*
