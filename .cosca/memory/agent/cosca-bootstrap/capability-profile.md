# cosca-bootstrap — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Workspace initialization (stack detection, context, memory, agent activation) | 0.25 | 0 | — | → |

## Strengths
- Automatic workspace discovery: language, framework, database, architecture pattern detection
- Context creation and memory initialization via delegation to cosca-context
- Agent registry mapping and classification-based agent activation for detected stack

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Execute the 10-phase bootstrap protocol: health check → discovery → context → memory → skills → registry → classification → activation → report → validation
- Delegate context and memory work to cosca-context; use quick bootstrap when `.cosca/` already exists
- Run Gate 0 checks to verify project identified, stack detected, context created, skills loaded
- Activate default chiefs (CEO, CTO, Product, Architecture, Review, QA, Documentation, Security, Context, Memory) plus stack-specific chiefs

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
