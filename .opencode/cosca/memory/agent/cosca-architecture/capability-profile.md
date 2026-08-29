# cosca-architecture — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-08-29

## Current Level: 3

Achieved via: ADR series (011–015, 027), multiple deep code-mining campaigns (code not README under ADR-017), and the Qdrant decision ADR.

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| System architecture design (boundaries, patterns, ADRs) | 0.85 | 6 ADRs | success | ↑ |
| ADR formalization (evidence-gated, reuso-first, honest gap map) | 0.88 | ADR-011/012/013/015/027 | success | ↑ |
| Code-mining / capability-borrowing (ADR-017: código não README, invariantes I1–I8) | 0.86 | 6 minerações deep | success | ↑ |
| Hybrid-search & vector-db trade-offs (Qdrant vs SQLite document-first) | 0.82 | mineração Qdrant + ADR-027 | success | ↑ |
| Reuso / over-engineering discipline (P8, "os bancos reais", stdlib-only) | 0.80 | ADR-015 | success | ↑ |

## Strengths
- **Honest gap-map before design**: reads real source (internal/*) to confirm what exists vs what's invented before proposing an ADR — avoids duplicating infrastructure and avoids over-engineering.
- **Reuse-first / P8 discipline**: anchors every ADR on existing packages (internal/gate, internal/workflow, internal/deliberate) instead of creating new systems.
- **Evidence-gated ADR series**: ADR-011 (deliberação), ADR-012 (2-zonas/Cofre), ADR-013 (bancos modulares), ADR-015 (primitivas neutras stdlib-only), ADR-017 (capability borrowing), ADR-027 (NÃO adotar Qdrant) — all decision-first, no implementation.
- **Code mining (not README)**: extracts real, transferable patterns (query planner, RRF/DBSF, payload index, control loop, byte-offset, temporal model) under an explicit invariants lens (I1–I8) and a borrow-not-assume protocol.
- **Honest rejection**: explicitly lists what NOT to adopt (Qdrant standalone/Edge, Rust→Go port, CSI/RF, Cesium/WebGL stack) with reasoning — the "no" is as valuable as the "yes".

## Weaknesses
- **ADRs only, no implementation**: architecture decisions are produced and delegated; the actual coding/backfill (colunas materializadas, estimador de cardinalidade) is not executed by this agent.
- **No performance measurement of its own designs**: relies on data from others (recall 0.99, ~10ms p50, joelho ~1M) rather than running benchmarks itself.
- **Breadth over depth in mining**: produces many cross-domain patterns but synthesizes fit-to-Cosca rather than going deep on one contributor.

## Preferred Strategies
- **Map the gap before writing**: confirm what exists (grep/read internal/*) → identify real gap → anchor the design on reuse → separate bounded slice from over-engineering.
- **A/B honesty: keep vs reject**: explicitly adjudicate "adopt / adapt / reject" with reasons (invariants I1 = deterministic/no-LLM, I7 = isolamento, single-binary, zero-infra).
- **Document-only, delegate implementation**: ADRs are decision artifacts; implementation is fatiada and delegated to the respective Chiefs.
- **Always reference prior ADRs**: number sequence, existing ADR-002/013/017 as anchors, so the decision chain is traceable.

## Known Failure Modes
- **Numbering collisions in the ADR index**: two ADR-011 existed — mitigated by verifying the next available number from prior ADRs before writing.
- **Over-engineering pull**: the adversarial blocks (multi-archétipe, HHEM local ~400MB, batalha completa) tend to expand scope — mitigated by an explicit "gate de custo / fatia bounded" rule.
- **Reading files as ground truth**: when generalized reports (sub-agent mining) conflict with the actual code path, code is the tiebreaker — validated before touching the brain.

## Evolution Goal
Reach Level 4:
*"Drive the ADR-027 slices to implementation (payload index materializado + B-tree, estimador de cardinalidade filter→vector, facets via SQL, RRF/DBSF) with measured before/after on the real corpus — graduating from decision-making to measured architecture-to-code delivery."*
