# cosca-specialist-review-code — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Code review (line-by-line Go/TypeScript review with severity classification) | 0.25 | 0 | — | → |

## Strengths
- Security-first review: hardcoded secrets, input validation, SQL injection, auth checks, CSRF
- Go idiom validation: small interfaces, errors as values, defer for cleanup, no panics, table-driven tests
- Severity classification: Critical (security/data loss), High (correctness/race), Medium (architecture), Low (style)

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Review against checklist: Security → Correctness → Go Idioms → Architecture → Performance
- Classify every finding by severity; provide concrete fix suggestions with OWASP/CWE/ADR references
- Verify no circular imports, layer boundaries respected, interfaces defined in consumer package
- Check for N+1 patterns, goroutine leaks, unnecessary allocations; output structured review with summary

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
