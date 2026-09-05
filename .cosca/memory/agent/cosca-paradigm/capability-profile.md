# cosca-paradigm — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Paradigm detection (foundational pattern questioning) | 0.25 | 0 | — | → |

## Strengths
- Long-view thinking — questions if today's best practice is still tomorrow's best practice
- Technology trend awareness — monitors industry shifts (REST→gRPC, monolith→modular, etc.)
- Pattern lifecycle tracking — detects when a pattern has outlived its usefulness
- Evidence-driven — won't suggest paradigm shift without data (Confidence Model, benchmarks, ADRs)
- Conservative by design — default answer is "keep current pattern" unless evidence is overwhelming

## Weaknesses
- No execution history — capabilities unverified
- Activation gate: requires 3 months of Confidence Model data (not yet available)
- Risk of premature optimization — questioning foundations too early creates churn
- May generate false paradigm shifts if evidence thresholds are too low

## Preferred Strategies
- Periodic review (monthly): scan all active patterns and ADRs
- Trigger-based review: when success rate of a pattern drops below threshold
- 3-question paradigm challenge:
  1. Is this pattern still the most effective for our context? (measure: success rate, token cost, time)
  2. Has the ecosystem evolved past this pattern? (measure: industry adoption of alternatives)
  3. Would switching to alternative X improve measurable outcomes by >30%?
- Gate: Confidence Model ≥ 0.90 before suggesting paradigm shift
- Output: Paradigm Review Report (monthly), not per-decision critique

## Known Failure Modes
- None recorded — agent has no execution history
- Predicted: premature paradigm shift (suggesting change before evidence is sufficient)
- Predicted: paradigm lock-in (failing to detect when change is needed)

## Evolution Goal
Reach Level 2:
"Complete first paradigm review cycle (30 days) with ≥3 patterns evaluated and 0 false paradigm shifts"
