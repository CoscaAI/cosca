# cosca-critic — Memory Index

> **📖 Leia o [AGENT_PRIMER.md](../AGENT_PRIMER.md) antes de agir.**

> Cross-reference for fast semantic retrieval. DNA v3.0.

## Memory Files

| File | Purpose | Entries |
|------|---------|---------|
| [learnings.md](learnings.md) | Semantic learning journal | 2 entries (1 seed + 1 real task) |
| [failures.md](failures.md) | Negative memory — failed critiques | 0 failures |
| [patterns.md](patterns.md) | Reusable critique patterns | — (pending) |
| [evolution.md](evolution.md) | Capability evolution timeline | Level 1 |
| [capability-profile.md](capability-profile.md) | Self-model: strengths, weaknesses, confidence | 1 domain scored (0.40) |

## Semantic Tags

| Tag | Learnings |
|-----|-----------|
| #decision-critique | L1-SEED-001, TASK-001 |
| #onda-2 | TASK-001 |
| #confidence-model | TASK-001 |
| #risk-assessment | L1-SEED-001, TASK-001 |
| #cross-reference | TASK-001 |
| #adversarial-review | TASK-001 |
| #agent-activation | TASK-001 |

## Cross-Reference Map

| This Agent's Learning | References |
|----------------------|------------|
| TASK-001 (Onda 2 Review) | onda-2-plan.md, RISK_REGISTRY.md, bug/INDEX.md, semantic/INDEX.md, cognitive-state.md, LEARNING_PROTOCOL.md, ADR-005, ADR-006 |

## Metacognition Pipeline

Every task flows through: SELF-ASSESS → RETRIEVE MEMORY → PLAN STRATEGY → EXECUTE → VERIFY RESULT → CRITIQUE OWN WORK → EXTRACT PATTERN → UPDATE CAPABILITY MODEL

See: [workflows/metacognition-pipeline.md](../../../workflows/metacognition-pipeline.md)

## Distinction from cosca-review

cosca-review reviews CODE (per-PR checklist: SOLID, security, performance).
cosca-critic reviews DECISIONS (per-decision adversarial: risks, alternatives, scale implications).
