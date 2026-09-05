# cosca-specialist-testing-e2e — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| E2E testing (complete user journeys, CLI + API + Web Console) | 0.25 | 0 | — | → |

## Strengths
- Complete user journey testing: CLI init → configure → serve → API call → verify
- CLI testing with exec.Command (capture stdout/stderr) and API testing with httptest in full server mode
- Multi-step workflow validation: auth → create → read → update → delete

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Test complete user journeys with real binaries and real servers (no mocks at E2E level)
- Use t.TempDir() for isolated test environments; use context.WithTimeout for operations that may hang
- Test error paths: bad config, network failures, invalid inputs
- Use Playwright for frontend E2E; report to Testing Chief; clean up after each test

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
