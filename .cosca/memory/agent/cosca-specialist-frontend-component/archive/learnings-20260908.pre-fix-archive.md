# cosca-specialist-frontend-component - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-specialist-frontend-component — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-frontend-component |
| **Task** | Initial capability establishment |
| **Technique** | Standard frontend-component patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #frontend-component #baseline #initialization |
| **Related** | .cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core frontend-component patterns documented. Ready for Level 2 techniques. |
| **What was difficult** | N/A (seed initialization) |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — TASK-001: AgentConfidenceCard Component
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-frontend-component |
| **Task** | Create AgentConfidenceCard dashboard component for Cosca platform |
| **Technique** | Level 2 — Multi-state component with full test coverage: 4 states (loading/error/empty/normal), color-coded confidence themes, accessible progress bar, responsive layout using useMobile(), memo optimization, Storybook stories for visual testing |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #frontend-component #dashboard #confidence #accessibility #shadcn #react19 #vitest |
| **Related** | `.cosca/memory/sessions/active/current.md` (Onda 2 agent data), `web/src/features/dashboard/types.ts`, `web/src/features/dashboard/components/agent-confidence-card.tsx` |
| **Learned** | 1) **Confidence data from Onda 2**: 10 agents activated with real confidence scores (0.40-0.75, avg 0.53). Domain mapping aligns with DNA v3.0 departments. 2) **Test patterns in Cosca**: `@/test/utils` exports only custom `render` wrapper — `screen` and `fireEvent` must be imported from `@testing-library/react` directly. 3) **Shadcn/UI New York style**: Uses `rounded-xl` cards, Tailwind CSS variables (`hsl(var(--*))`), `cn()` utility from `@/lib/utils`. 4) **Accessibility**: `role="progressbar"` with `aria-valuenow/min/max` for confidence bars. `role="status"` for status badges. 5) **Responsive**: `useMobile()` hook for adaptive padding (p-4 mobile, p-6 desktop). 6) **Color theming**: Confidence ≥70% → emerald (success), 40-69% → amber (warning), <40% → red (danger). Dark mode support via `dark:` variants. 7) **Pre-existing build issues**: `src/features/templates/data/builtin-templates` is missing — blocks `next build` but not typecheck or tests. 8) **Component architecture**: Sub-components (`ConfidenceProgressBar`, `AgentConfidenceCardSkeleton`, `AgentConfidenceCardEmpty`) as module-private functions. Main component as memoized named export. |
| **What was difficult** | No significant difficulty. Pre-existing `pnpm install` issue with `build-scripts` required using `./node_modules/.bin/tsc` directly. The `screen`/`fireEvent` import issue was unexpected — the test utils wrapper only re-exports `render`. |
| **Confidence Impact** | +0.10 — First task executed successfully. Component compiled clean (0 new TS errors), 23/23 tests passing, all 4 states covered. |
| **Next** | Level 3: Build an interactive dashboard page that composes multiple AgentConfidenceCards with real data fetching via TanStack Query. |

