# cosca-frontend — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-08-29

## Current Level: 3

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Next.js 15 App Router | 0.90 | 1 | success | ↑ |
| Feature-Slice Architecture | 0.85 | 1 | success | ↑ |
| shadcn/ui + Radix Primitives | 0.80 | 1 | success | → |
| State Management (Context + TanStack Query) | 0.85 | 1 | success | ↑ |
| Auth Pattern (httpOnly cookies, CSRF) | 0.85 | 1 | success | ↑ |
| API Client (fetch wrapper) | 0.75 | 1 | success | → |
| Testing Pyramid (Vitest, Playwright, Storybook) | 0.75 | 1 | success | → |
| PWA (Service Worker, Manifest) | 0.70 | 1 | success | → |
| Three.js/WebGL 3D visualization (shaders, spatial hash, marker map) | 0.82 | brain 3D (app.js) | success | ↑ |
| WebGL detection + graceful 2D fallback | 0.80 | force2D/forceWebGL + swiftshader fallback | success | ↑ |
| CSP 'self' frontend discipline (self-hosted assets) | 0.78 | brainweb | success | ↑ |
| Accessibility (WCAG 2.1 AA) | 0.20 | 0 | — | → |
| Visual Regression Testing | 0.10 | 0 | — | → |
| Feature Flags | 0.10 | 0 | — | → |
| Zustand/Redux (evaluation only) | 0.40 | 1 | success | → |

## Strengths
- **Complete architecture extraction from live codebase**: Scanned 240+ TSX files across 26 routes (2 route groups), 27 feature modules, 14 Radix UI primitives, and 18 shared component groups — producing a comprehensive architecture document without missing a single module.
- **Auth surface understanding**: Traced the httpOnly cookie auth flow end-to-end — tokens never touch JavaScript, auth state detected via cosca_auth_state=true sentinel cookie (non-HttpOnly), CSRF via double-submit pattern with constant-time compare server-side, and auto CSRF header injection in fetch wrapper.
- **Stack characterization**: Accurately identified what's used (Next.js 15 App Router, TanStack Query with staleTime=60s, React Context, next-themes, class-variance-authority, Vitest 43 files, Playwright 6 specs, Storybook 21 stories) and what's NOT used (Zustand, Redux, Jotai, Axios).
- **Feature-slice pattern documentation**: Cataloged all 27 features with their components/hooks/types pattern, barrel exports, and self-contained test files — enabling any agent to navigate the frontend.
- **Testing pyramid assessment**: Mapped the full testing infrastructure: MSW mocking 30+ endpoints, Playwright E2E specs for critical paths, Storybook with a11y and dark mode addons.
- **Three.js/WebGL 3D rendering**: Built the brain 3D view with custom ShaderMaterial (point-size attribute, color gradient by volume position, AdditiveBlending), Bezier axón (synapse) wiring via spatial-hash (27 neighbor cells) to avoid O(n²), a reusable particle pool + signal mesh, real-agent marker mapping (multiple of `verticesSkipStep`), and an honest 2D fallback when WebGL is unavailable/software-rendered.

## Weaknesses
- **No accessibility audit**: Has not evaluated WCAG 2.1 AA compliance despite a11y addon being present in Storybook.
- **No visual regression testing**: Has not integrated Chromatic/Percy or comparable visual testing.
- **No feature flag system**: Has not evaluated whether gradual rollout mechanisms are needed for the 27 feature modules.
- **CSP hardening still open**: the /brain index.html still uses `'unsafe-inline'` (importmap) + `'unsafe-eval'` — the recommendation to move to nonce/hash (H3) is documented but not yet applied.

## Preferred Strategies
- **Architecture extraction from barrel exports**: Uses barrel exports (index.ts) as entry points to trace feature modules, then drills into components/hooks/types subdirectories — builds a complete map before writing any documentation.
- **Provider chain tracing**: Follows the full provider chain (auth → query → theme → app) to understand the initialization order and dependency graph.
- **"What's NOT used" validation**: Actively confirms the absence of libraries like Zustand/Redux/Axios rather than assuming — this prevented misdocumenting state management patterns.

## Known Failure Modes
- None recorded — both learning entries show successful outcomes.

## Evolution Goal
Reach Level 4:
*"Complete WCAG 2.1 AA accessibility audit across all 27 feature modules, integrate visual regression testing (Chromatic/Percy), implement a feature flag system, and harden the /brain CSP (nonce/hash on the importmap, drop 'unsafe-eval', add script-src-attr 'none') — graduating from feature documentation and a first 3D view to production-quality, accessible, and CSP-clean frontend delivery."*
