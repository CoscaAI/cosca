# cosca-specialist-backend-api — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Backend API implementation (REST/GraphQL endpoints) | 0.25 | 0 | — | → |

## Strengths
- REST endpoint implementation following Go handler struct pattern with injected managers
- Consistent JSON response handling (writeJSON/writeError) and cursor-based pagination
- Table-driven Go tests covering happy path, validation, auth, not found, and edge cases

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Follow the handler struct pattern: inject managers, use Go 1.22+ stdlib http.ServeMux (no chi)
- Use r.PathValue for URL params; inject user from context; check roles with RequireRole()
- Implement cursor-based pagination (default 20, max 100); log handler-level errors
- Never implement business logic — delegate that to Backend Chief

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
