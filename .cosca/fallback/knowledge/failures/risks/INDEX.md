# Risk Registry

> **Category**: Failures → Risks | **Version**: 1.0.0 | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-29

## Purpose

Project risk registry — catalogues platform, agent, memory, process, and roadmap risks with probability, impact, and mitigation strategies. Review cadence: weekly or after any incident.

## Registry

The canonical risk registry is [`RISK_REGISTRY.md`](RISK_REGISTRY.md) (v1.0.0, created 2026-07-28).

## Risk Summary

| Severity | Count | Action |
|:--------:|------:|--------|
| 🔴 Critical | 3 | **This week** — R1 (Onda 2 activation), R2 (Confidence 0.48), R3 (Soak test) |
| 🟠 High | 4 | **This month** — R4 (Benchmark), R5 (gRPC), R6 (Calibrate Decay), R7 (Validate Compression) |
| 🟡 Medium | 7 | **Plan** — R9–R15, most depend on infrastructure or data |
| **Total** | **14** | |

## Top Critical Risks

| # | Risk | Probability | Impact |
|---|------|:----------:|:------:|
| R1 | 41/51 agents never executed — Capability Profiles based on seed data | 90% | High |
| R2 | Average confidence 0.48 (target 0.70) — platform doesn't trust itself | 80% | High |
| R3 | No soak test (>24h) — memory leaks, cache leaks, race conditions invisible | 70% | High |

## Top 3 Immediate Actions

| # | Action | Resolves | Effort |
|---|--------|----------|:------:|
| 1 | Activate Onda 2 — 1 real task for each of 10 most critical agents | R1 + R2 + R12 | 2–3 days |
| 2 | 1h soak test in CI with memory monitoring | R3 | 1 day |
| 3 | Doc-code validator script (cross-reference doc claims with real paths) | R8 | 1 day |

## Related

- **[Platform Evolution v1.4.0](../../../memory/roadmap/platform-evolution-v1.4.0.md)** — Roadmap context
- **[Heuristic H-018](../../heuristics/H-018-confidence-theatre.yaml)** — Confidence theatre (metrics that incentivize reporting success over real improvement)
- **[Bugs](../bugs/INDEX.md)** — Bugs discovered that may relate to registered risks

---

*"Negative memory is the most valuable and most underutilized learning resource." — Heuristic H-019. The risk registry is proactive negative memory — anticipating failures before they occur.*
