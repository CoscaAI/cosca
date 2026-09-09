# cosca-specialist-testing-unit — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Unit testing (AAA pattern, table-driven Go tests, Vitest/React) | 0.25 | 0 | — | → |

## Strengths
- AAA pattern implementation: Arrange (setup), Act (execute), Assert (verify) with descriptive test names
- Table-driven Go tests with testify (require/assert) covering happy path, edge cases, and error paths
- Mock at interface boundaries with no test interdependence; support for t.Parallel() for independent tests

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Go: Use t.TempDir() for temp files (never /tmp); require for preconditions, assert for results
- TypeScript: Vitest describe/it/expect pattern; React Testing Library render/screen/getBy/userEvent; MSW for API mocking
- Test all component states: loading, error, empty, success; never change production code to make tests pass
- Cover nil inputs, empty inputs, and edge cases; mock external dependencies; report to Testing Chief

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
