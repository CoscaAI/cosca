# cosca-context — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Context management (session, project, environment) | 0.25 | 0 | — | → |

## Strengths
- Session context building and maintenance refreshed at each session start
- Project state tracking with dependency context and recent change monitoring
- Environment context building (OS, tools, versions) with context summaries for other agents

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Delegate memory persistence to Memory Chief and workspace scanning to Discovery Engine
- Keep context summaries concise (< 500 words); never include secrets or credentials
- Refresh context at every session start under 5 seconds; ensure platform-aware environment context
- Provide accurate, actionable context reports with no stale data from previous sessions

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
