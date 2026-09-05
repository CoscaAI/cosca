# cosca-specialist-documentation-writer — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Technical documentation (README, ADRs, API docs, guides) | 0.25 | 0 | — | → |

## Strengths
- Architecture Decision Record (ADR) documentation following the standard ADR format
- API reference documentation with endpoint, method, path, request/response, error codes, and curl examples
- Changelog generation in Keep a Changelog format with Mermaid diagram creation

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Follow ADR format: Title, Status, Context, Decision, Rationale, Alternatives, Consequences
- Keep README concise (<500 lines); document every API endpoint; use Mermaid for diagrams
- Keep changelog grouped by Added, Changed, Fixed, Removed; link to commits
- Never write code — document what exists; update docs before merging PRs; report to Documentation Chief

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
