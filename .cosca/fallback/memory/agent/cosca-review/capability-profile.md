# cosca-review — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Code review (quality, architecture compliance, security, standards) | 0.25 | 0 | — | → |

## Strengths
- Comprehensive code change review for quality, SOLID principles, and coding standards
- Architecture compliance review and security best practices verification
- Code smell and anti-pattern identification with actionable improvement suggestions

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Review using checklist: SOLID, architecture patterns, no security vulnerabilities, performance, tests, error handling, no dead code, no hardcoded secrets, DRY, SRP
- Report and review only — never implement changes or make architecture decisions (flag for Architecture Chief)
- Review all deliverables before QA; check performance implications and documentation completeness
- Approve or reject deliverables based on standards compliance

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
