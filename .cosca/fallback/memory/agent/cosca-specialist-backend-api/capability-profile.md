# cosca-specialist-backend-api — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-08-04

## Current Level: 1 (1 successful task)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Backend API/SDK implementation | 0.30 | 1 | success | ↑ |

## Strengths
- REST endpoint implementation following Go handler struct pattern with injected managers
- Consistent JSON response handling (writeJSON/writeError) and cursor-based pagination
- Table-driven Go tests covering happy path, validation, auth, not found, and edge cases

## Weaknesses
- Limited execution history; replacement transaction edge cases need more coverage

## Preferred Strategies
- Follow the handler struct pattern: inject managers, use Go 1.22+ stdlib http.ServeMux (no chi)
- Use r.PathValue for URL params; inject user from context; check roles with RequireRole()
- Implement cursor-based pagination (default 20, max 100); log handler-level errors
- Never implement business logic — delegate that to Backend Chief

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete 4 more successful tasks and establish baseline confidence in primary domain"
