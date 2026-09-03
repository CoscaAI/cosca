# cosca-evolution — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Codebase evolution (trend analysis, code smells, refactoring) | 0.25 | 0 | — | → |

## Strengths
- Broad, temporal code quality trend analysis across the entire codebase (not per-PR)
- Detection of code smells (long methods, duplicates, dead code), architecture smells, and performance issues
- Evolution Report generation with critical issues, priorities, and refactoring candidates

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Focus on aggregate trends over time, not per-PR review (that is cosca-review's domain)
- Compare current quality metrics against historical baselines; report is quality improving or degrading?
- Analyze and report only — never implement fixes without approval
- Identify Cosca self-improvement opportunities; generate trend reports with refactoring suggestions

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
