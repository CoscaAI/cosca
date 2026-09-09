# cosca-testing — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Test implementation (unit, integration, E2E, contracts) | 0.25 | 0 | — | → |

## Strengths
- Test architecture design following the testing pyramid (many unit, fewer integration, very few E2E)
- Unit test and integration test implementation with test fixtures and factories maintenance
- Test coverage tracking and flaky test elimination for reliable test suites

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Follow AAA pattern with descriptive names and no interdependence between tests
- Mock external dependencies; test edges and errors; never change production code — test what exists
- Delegate unit tests to cosca-specialist-testing-unit, integration to cosca-specialist-testing-integration, E2E to cosca-specialist-testing-e2e
- Maintain test fixtures and manage test data; report what fails without modifying source

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
