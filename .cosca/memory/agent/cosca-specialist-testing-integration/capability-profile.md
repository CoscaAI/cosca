# cosca-specialist-testing-integration — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Integration testing (service boundaries, API contracts, real SQLite) | 0.25 | 0 | — | → |

## Strengths
- Full request/response cycle testing: handler → manager → SQLite using httptest.NewServer + http.Client
- Auth middleware testing: valid JWT, expired JWT, missing JWT, wrong role scenarios
- Concurrent access testing with multiple goroutines hitting the same endpoint

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Test real service boundaries with real SQLite (:memory: or TempDir), never mocks for DB
- Use t.Cleanup() for database teardown; use testify (require for preconditions, assert for results)
- Write table-driven tests for multiple scenarios; cover auth, concurrency, and error paths
- Test full request/response cycles at service boundaries; report to Testing Chief

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
