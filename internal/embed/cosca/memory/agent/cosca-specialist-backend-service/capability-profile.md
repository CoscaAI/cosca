# cosca-specialist-backend-service — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Backend service implementation (business logic, domain services) | 0.25 | 0 | — | → |

## Strengths
- Manager pattern implementation (Manager structs with injected dependencies and constructor wiring)
- Business logic encapsulation with data access in the same layer (no separate service/repository)
- Concurrency-safe code using sync.RWMutex, channels, and context.Context propagation

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Follow the manager/store pattern: Managers own both business logic and data access
- Inject dependencies via constructor; wrap errors with context using fmt.Errorf("context: %w", err)
- Never panic() — return errors; use table-driven tests with testify (require/assert)
- Mock at interface boundaries or use in-memory SQLite for testing

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
