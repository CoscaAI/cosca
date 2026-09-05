# Failures Knowledge Domain

> **Category**: Failures | **Version**: 1.0.0 | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-29

## Purpose

Negative memory — the most valuable and most underutilized learning resource (see heuristic H-019). This domain catalogues every failure: bugs with causality trees, incidents, audits, reviews, and the risk registry. The goal is systematic learning from every mistake.

## Directory Structure

```
failures/
├── bugs/           (8 bugs + causality tree template)
├── incidents/      (0 — catalogued in best-practices/playbooks/)
├── audits/         (1 coverage audit)
├── reviews/        (2 review reports: Onda 2 + Onda 3)
└── risks/          (1 risk registry)
```

## Sub-Directories

| Directory | Description | Files | Index |
|-----------|-------------|------:|-------|
| [`bugs/`](bugs/INDEX.md) | Bug registry with 4-level causality trees (N1–N4) | 9 (8 bugs + template) | [bugs/INDEX.md](bugs/INDEX.md) |
| [`incidents/`](incidents/INDEX.md) | Incident catalog — currently empty, playbooks exist in best-practices | 0 | [incidents/INDEX.md](incidents/INDEX.md) |
| [`audits/`](audits/INDEX.md) | Audit reports — coverage, architecture, compliance | 1 | [audits/INDEX.md](audits/INDEX.md) |
| [`reviews/`](reviews/INDEX.md) | Code and architecture review reports | 2 | [reviews/INDEX.md](reviews/INDEX.md) |
| [`risks/`](risks/INDEX.md) | Project risk registry — probability, impact, mitigation | 1 | [risks/INDEX.md](risks/INDEX.md) |

## Causality Tree Framework

All bugs since 2026-07-28 follow the **Causality Tree v2.0.0** format, which traces every failure through 4 levels:

```
BUG (symptom)
 ├── N1: CAUSA DIRETA — what broke in code (specific line, function, structure)
 ├── N2: CAUSA ARQUITETURAL — why the system allowed this to happen
 ├── N3: CAUSA DE PROCESSO — which process failure allowed N1+N2 to reach main
 └── N4: PREVENÇÃO SISTÊMICA — what prevents this bug class from recurring
```

See [`bugs/CAUSALITY_TREE_TEMPLATE.md`](bugs/CAUSALITY_TREE_TEMPLATE.md) for the full template.

## Heuristic Reference

Failures are the primary source of anti-pattern heuristics. The following heuristics were extracted directly from failure analysis:

| Heuristic | Extracted From |
|-----------|---------------|
| H-009 (memory drift fiction) | Knowledge drift incidents |
| H-012 (bug registry underestimates) | Bug discovery gap |
| H-013 (architecture-code gap) | Architecture audit + Bug-008 |
| H-016 (declarative state machine gap) | Bug-006 (Restart broken) |
| H-017 (flaky test nondeterministic) | Bug-003 (race conditions) |
| H-019 (negative memory underutilized) | Systemic observation across all failures |

## Statistics

| Category | Count |
|----------|------:|
| Bugs (with causality trees) | 8 |
| Bugs — Blocker | 1 (bug-006) |
| Bugs — Critical | 1 (bug-007) |
| Bugs — Major | 1 (bug-008) |
| Bugs — High | 2 (bug-003, bug-005) |
| Bugs — Medium | 2 (bug-001, bug-004) |
| Bugs — Low | 1 (bug-002) |
| Incidents | 0 (playbooks exist) |
| Audits | 1 |
| Reviews | 2 |
| Risks (registry) | 14 (3 critical, 4 high, 7 medium) |
| Template files | 1 (causality tree) |

---

*"Negative memory is the most valuable and most underutilized learning resource." — Heuristic H-019*
