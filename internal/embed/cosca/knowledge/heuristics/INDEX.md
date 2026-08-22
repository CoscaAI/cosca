# Heuristics Knowledge Domain

> **Category**: Heuristics | **Version**: 1.0.0 | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-29

## Purpose

Heuristics are extracted decision rules derived from agent learnings across 17+ agents. They encode patterns (what to do) and anti-patterns (what to avoid) grounded in real operational experience.

## Overview

| Metric | Value |
|--------|------:|
| Total heuristics | 20 |
| Patterns ("do this") | 5 |
| Anti-patterns ("avoid this") | 15 |
| Critical severity | 9 |
| High severity | 11 |
| Confidence range | 0.85 – 0.95 |
| Source agents analyzed | 17 |

## Domain Distribution

| Domain | Count | Heuristics |
|--------|------:|------------|
| **Security** | 3 | H-005 (build is security moment), H-006 (wildcard permissions), H-020 (auto-jail embedded) |
| **Testing** | 2 | H-001 (security without testing), H-002 (CI threshold is truth) |
| **CI/CD** | 2 | H-003 (continue-on-error noop), H-017 (flaky test nondeterministic iteration) |
| **Refactoring** | 2 | H-004 (extract to test), H-016 (declarative state machine gap) |
| **Orchestration** | 3 | H-007 (three-wave orchestration), H-008 (parallel delegation), H-018 (confidence theatre) |
| **Memory** | 3 | H-009 (memory drift fiction), H-010 (update learnings every task), H-019 (negative memory underutilized) |
| **Governance** | 3 | H-011 (conflicting thresholds), H-012 (bug registry underestimates), H-013 (architecture-code gap) |
| **Performance** | 2 | H-014 (N+1 queries), H-015 (duplicate cache implementations) |

## Heuristics Index

### Patterns (5)

| ID | Title | Domain | Severity | Confidence |
|----|-------|--------|:--------:|:----------:|
| H-004 | Extract to test — monolith is the enemy of coverage | refactoring | high | 0.90 |
| H-005 | Build is the most critical security moment — protection must be atomic | security | critical | 0.95 |
| H-007 | Three-wave orchestration: analytic → implementation → review | orchestration | high | 0.85 |
| H-008 | Parallel delegation does not require sequential dependency if context is provided | orchestration | high | 0.88 |
| H-020 | External scripts are single point of failure — embed protection in the binary | security | critical | 0.90 |

### Anti-Patterns (15)

| ID | Title | Domain | Severity | Confidence |
|----|-------|--------|:--------:|:----------:|
| H-001 | Security without testing is facade security | testing | critical | 0.95 |
| H-002 | The threshold that matters is what CI executes, not what documentation says | testing | critical | 0.95 |
| H-003 | `continue-on-error: true` makes any CI gate inoperative | ci-cd | critical | 0.92 |
| H-006 | Wildcard permissions (`.*` or `/**`) violate least privilege and create attack surface | security | critical | 0.92 |
| H-009 | Memory can drift into fiction without verification against source code | memory | critical | 0.93 |
| H-010 | Agents must update learnings.md after EVERY significant task | memory | high | 0.90 |
| H-011 | Conflicting thresholds: audit what executes, not what is documented | governance | critical | 0.93 |
| H-012 | Formal bug registry underestimates real bugs — audit source code for completeness | governance | high | 0.87 |
| H-013 | Documented vs implemented architecture: verify code before trusting docs | governance | critical | 0.90 |
| H-014 | N+1 roundtrips in sequential queries kill performance even with fast queries | performance | high | 0.85 |
| H-015 | Multiple implementations of the same pattern: consolidate into single backend | performance | high | 0.85 |
| H-016 | Declarative state machine with imperative enforcement = bug gap | refactoring | high | 0.88 |
| H-017 | Flaky test from nondeterministic iteration: Go map range + positional access | ci-cd | high | 0.90 |
| H-018 | Confidence theatre: metrics that incentivize reporting success over real improvement | orchestration | high | 0.85 |
| H-019 | Negative memory is the most valuable and most underutilized learning resource | memory | high | 0.87 |

## File Format

Each heuristic is stored as a YAML file (`H-NNN.yaml`) in this directory. The machine-readable index is [`INDEX.yaml`](INDEX.yaml), which is **auto-generated** from the YAML files — do not edit manually.

This `INDEX.md` is the **human-readable companion** to `INDEX.yaml`. It provides the same information in a browsable, documented format suitable for developers and reviewers.

## Relationship to Other Domains

- **Patterns** — Heuristics H-004 through H-020 emerged from applying or violating patterns in practice
- **Failures** — Many heuristics (H-009, H-012, H-013, H-016, H-017) were extracted directly from bug causality trees and incident post-mortems
- **Best Practices** — Heuristics inform playbook design and CI gate configuration

---

*Heuristics are extracted from `memory/agent/*/learnings.md` across 17+ agents with 55+ substantive entries and 1500+ lines reviewed. See [INDEX.yaml](INDEX.yaml) for the machine-readable canonical source.*
