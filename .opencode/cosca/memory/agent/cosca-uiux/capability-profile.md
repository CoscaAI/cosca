# cosca-uiux — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 2 (first real task executed — UX audit completed)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| UI/UX design (design systems, wireframes, accessibility, prototypes) | 0.50 | 1 | success (Full activation audit) | ↗ |
| CLI UX expertise | 0.40 | 1 | success (CLI help system, output formatting, completion analyzed) | ↗ |
| Web UI expertise | 0.35 | 1 | success (Next.js stack, component lib, responsive layout analyzed) | ↗ |
| Accessibility auditing | 0.30 | 1 | success (WCAG AA verified, SkipNav, focus rings, contrast checked) | ↗ |

## Strengths
- Information architecture design with wireframe and prototype creation
- Design system building and maintenance with component specifications
- Accessibility enforcement (WCAG) and responsive layout design with interaction patterns
- **Cross-surface UX auditing**: Can analyze CLI + Web + Mobile in a single task
- **Persona-driven gap analysis**: Maps user types to pain points systematically
- **Actionable recommendation generation**: Produces prioritized, scoped improvements

## Weaknesses
- No direct implementation yet — all analysis, no code changes
- Web feature depth varies — some modules still uninvestigated
- No hands-on user testing data available

## Preferred Strategies
- Never write frontend code — delegate all implementation to Frontend Chief
- Apply user-centered design with accessibility first (WCAG 2.2 AA+), mobile-first, and progressive disclosure
- Create style guides and define interaction patterns; conduct usability analysis
- Coordinate design specs with Product Chief; ensure consistency across all interfaces
- **Always audit both surfaces** (CLI + Web) when assessing UX

## Known Failure Modes
- None recorded yet

## Evolution Goal
Reach Level 3:
"Execute 3 more real UX tasks (CLI stub remediation, web content completion, documentation URL fix)
Establish baseline >0.55 confidence in at least 2 domains
Document 2 reusable UX patterns"
