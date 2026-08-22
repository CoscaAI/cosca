# cosca-specialist-frontend-component — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (seed data — no real task execution yet)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Frontend component implementation (React, Next.js, Shadcn/UI, Tailwind) | 0.25 | 0 | — | → |

## Strengths
- Component implementation handling all states: loading (Skeleton), empty (EmptyState), error (ErrorState with retry), success
- WCAG 2.2 AA+ accessibility compliance with aria-label, role, keyboard navigation, and focus management
- TypeScript-first development with explicit types, composition over inheritance, and shared component architecture

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Use Next.js 15 App Router, Shadcn/UI (Radix + Tailwind), TanStack Query, Zustand, React Hook Form + Zod
- Apply mobile-first responsive design; use useMemo/useCallback where measured necessary; lazy load heavy components
- Test all states with Vitest + React Testing Library; use Storybook for visual testing
- Never make design decisions — follow the design system exactly; report to Frontend Chief

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
