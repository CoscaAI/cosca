# Phase 4: Final Polish — Implementation Workflow

> **Status:** complete ✅ | **Owner:** Workflow Chief | **Last Updated:** 2026-07-27
> **Completed:** 2026-07-27 | **Delivered:** All 6 blocks (A-F) completed  
> **Target:** 6–8 weeks of disciplined, sequentially-gated sprints — delivered in a single session
> **Dependencies:** Phases 0–3 complete (17 frontend routes, 36 REST endpoints, ~100+ feature files)
> **Related:** [ADR-007 (Frontend Architecture)](../adr/ADR-007-frontend-architecture.md) | [State Audit (historical)](state-audit-2026-07-25.md) | [CHANGELOG](../../CHANGELOG.md)

> 🎉 **Phase 4 is COMPLETE.** All 6 blocks (A-F) were delivered: 15 new pages, full design system (DataGrid, Charts, Notification Center, Markdown, Tour, etc.), security hardening (CSP, CSRF, rate limiting, JWT hardening, OWASP Top 10 review), PWA support (manifest, service worker, offline mode), 354 Vitest tests, 10 Playwright E2E paths, 29 Storybook stories, 50 OpenAPI operations, WCAG AA+ audit. See CHANGELOG.md v1.4.0-dev for full details.

---

## Table of Contents

1. [Phase Overview](#1-phase-overview)
2. [Pre-Implementation Checklist](#2-pre-implementation-checklist)
3. [Agent Mapping](#3-agent-mapping)
4. [Authorization Gates](#4-authorization-gates)
5. [Block A: Remaining Pages — 15 Pages](#5-block-a-remaining-pages)
6. [Block B: Design System & UX](#6-block-b-design-system--ux)
7. [Block C: Performance](#7-block-c-performance)
8. [Block D: Tests & Quality](#8-block-d-tests--quality)
9. [Block E: Security](#9-block-e-security)
10. [Block F: Responsiveness & PWA](#10-block-f-responsiveness--pwa)
11. [Consolidated Success Criteria](#11-consolidated-success-criteria)
12. [Risk Register](#12-risk-register)
13. [Dependency Graph](#13-dependency-graph)

---

## 1. Phase Overview

### 1.1 What Phase 4 Delivers

Phase 4 transforms the Phase 0–3 MVP into a production-grade, enterprise-ready web console:

| Block | Category | Key Output |
|-------|----------|------------|
| **A** | Remaining Pages | 15 new pages: Orchestration, Playground, Pipeline Editor, Audit Log, Secrets Vault, Metrics, Activity Monitor, Provider Compare, Plugin Registry, Context Builder, Templates, Prompts, Analytics, Admin Dashboard, Execution History |
| **B** | Design System & UX | Enterprise DataGrid, Charts (Recharts), Notification Center, Split Panels, Timeline, Onboarding Tour, Keyboard Shortcuts, Breadcrumb, Global Search, Markdown Renderer, Component Docs |
| **C** | Performance | PPR, ISR, Streaming SSR, Bundle optimization (<200KB per route), Image/Font optimization, Caching tuning, Lighthouse >90 |
| **D** | Tests & Quality | Vitest (>80% coverage), RTL, Playwright E2E (10 paths), Storybook (>25 stories), WCAG AA+ (axe-core), Coverage CI gate |
| **E** | Security | CSP headers, CSRF protection, Rate limiting, Security headers, JWT hardening, Input sanitization, OWASP Top 10 review, Secrets vault encryption, Audit logging |
| **F** | Responsiveness & PWA | All breakpoints (320px–2560px), Touch targets >44px, Bottom nav (mobile), PWA manifest + service worker, Offline support |

### 1.2 Total Estimated Effort

| Block | Sprints | Weeks | Agent Hours (est.) |
|-------|---------|-------|---------------------|
| **A** — Pages | 2 | 2 | 60 |
| **B** — Design System | 2 | 2 | 50 |
| **C** — Performance | 0.5 | 0.5 | 20 |
| **D** — Tests & Quality | 1.5 | 1.5 | 40 |
| **E** — Security | 0.5 | 0.5 | 25 |
| **F** — Responsiveness & PWA | 0.5 | 0.5 | 20 |
| Coordination + gates | — | 1 | 15 |
| **TOTAL** | **7** | **6–8 weeks** | **~230** |

---

## 2. Pre-Implementation Checklist

### 2.1 Backend Prerequisites

```
PRIORITY: ✅ ALL COMPLETE | 🔴 P0 = Before Phase 4 | 🟡 P1 = Week 1 | 🟢 P2 = Week 2+
```

#### ✅ P0 — Completed Before Phase 4

- [x] **Wire `POST /v1/run` for Orchestration Console** — Completed
- [x] **Add `POST /v1/run/stream` (SSE) for streaming orchestration** — Completed
- [x] **Add `GET /v1/executions` for execution history** — Completed
- [x] **Add `GET /v1/executions/{id}` for execution detail** — Completed

#### ✅ P1 — Completed During Phase 4

- [x] **Add `GET /v1/metrics` for dashboards** — Completed
- [x] **WebSocket endpoint `GET /ws` for real-time notifications** — Completed
- [x] **Add `GET /v1/knowledge/recent`** — Completed
- [x] **Add `GET /v1/memory/recent`** — Completed

#### ✅ P2 — Completed During Phase 4

- [x] **Audit log storage + API** — Completed
- [x] **Secrets vault CRUD** — Completed
- [x] **Provider compare: `POST /v1/providers/compare`** — Completed

### 2.2 Existing Infrastructure Verification

- [x] **Verify all 36 existing endpoints are functional** — Verified
- [x] **Run full test suite**: `go test ./internal/... ./api/...`, `cd web && pnpm build && pnpm typecheck && pnpm lint` — All pass
- [x] **E2E smoke test**: Login → Dashboard loads → Knowledge search → Memory browse → Runtime status — Verified

### 2.3 Frontend Tooling Prerequisites

Install Phase 4 dependencies:

```bash
cd web
# Design system
pnpm add recharts @radix-ui/react-tabs @radix-ui/react-context-menu @radix-ui/react-popover
pnpm add @radix-ui/react-select @radix-ui/react-toggle-group @radix-ui/react-progress
pnpm add @radix-ui/react-switch @radix-ui/react-slider @radix-ui/react-checkbox
pnpm add @radix-ui/react-label @radix-ui/react-accordion

# Testing (Block D)
pnpm add -D vitest @vitejs/plugin-react @testing-library/react @testing-library/jest-dom
pnpm add -D @testing-library/user-event @playwright/test @storybook/react @storybook/nextjs
pnpm add -D @storybook/addon-essentials @storybook/addon-a11y @storybook/addon-interactions
pnpm add -D @storybook/testing-library axe-core @axe-core/react jsdom msw @vitest/coverage-v8

# PWA (Block F)
pnpm add -D next-pwa @next/bundle-analyzer
```

Verify: `pnpm vitest --version`, `npx playwright --version`, `npx storybook --version`

---

## 3. Agent Mapping

Each action maps to a Primary Agent (does the work), Review Agent (validates), and Approver (authorizes merge).

### 3.1 Block A — Remaining Pages

| Action | Primary Agent | Review Agent | Approver |
|--------|--------------|--------------|----------|
| Orchestration Console | Frontend Chief | Architecture Chief | CTO |
| AI Playground | Frontend + AI Chief | Architecture Chief | CTO |
| Pipeline Editor | Frontend + Backend Chief | Architecture Chief | CTO |
| Execution History | Frontend Chief | Architecture Chief | CTO |
| Audit Log Viewer | Frontend Chief | Security Chief | CTO |
| Secrets Vault | Frontend Chief | Security Chief | CTO |
| System Metrics Dashboard | Frontend Chief | Monitoring Chief | CTO |
| Activity Monitor | Frontend Chief | Monitoring Chief | CTO |
| Provider Compare | Frontend + AI Chief | Architecture Chief | CTO |
| Plugin Registry Browser | Frontend Chief | Architecture Chief | CTO |
| Context Builder | Frontend Chief | Architecture Chief | CTO |
| Template Manager | Frontend Chief | Architecture Chief | CTO |
| Prompt Library | Frontend + AI Chief | Architecture Chief | CTO |
| Search Analytics | Frontend + Analytics Chief | Architecture Chief | CTO |
| Admin Overview | Frontend Chief | Security Chief | CTO |

### 3.2 Block B — Design System & UX

| Action | Primary | Review | Approver |
|--------|---------|--------|----------|
| DataGrid Component | Frontend Chief | UI/UX Chief | CTO |
| Charts (all types) | Frontend Chief | UI/UX Chief | CTO |
| Notification Center | Frontend + Backend Chief | Architecture Chief | CTO |
| Split View / Resizable Panels | Frontend Chief | UI/UX Chief | CTO |
| Timeline / Activity Feed | Frontend Chief | UI/UX Chief | CTO |
| Onboarding Tour | Frontend + UI/UX Chief | Product Chief | CTO |
| Keyboard Shortcuts Panel | Frontend Chief | UI/UX Chief | CTO |
| Breadcrumb Navigation | Frontend Chief | UI/UX Chief | CTO |
| Advanced Global Search | Frontend Chief | Architecture Chief | CTO |
| Markdown Renderer | Frontend Chief | UI/UX Chief | CTO |
| Enhanced Theme Switcher | Frontend Chief | UI/UX Chief | CTO |

### 3.3 Block C — Performance

| Action | Primary | Review | Approver |
|--------|---------|--------|----------|
| PPR configuration | Frontend Chief | Architecture Chief | CTO |
| ISR for static pages | Frontend Chief | Architecture Chief | CTO |
| Streaming SSR | Frontend Chief | Architecture Chief | CTO |
| Bundle analysis + optimization | Frontend Chief | Architecture Chief | CTO |
| Image optimization | Frontend Chief | UI/UX Chief | CTO |
| Font optimization | Frontend Chief | UI/UX Chief | CTO |
| API caching tuning | Frontend Chief | Architecture Chief | CTO |
| Code splitting verification | Frontend + DevOps Chief | Architecture Chief | CTO |

### 3.4 Block D — Tests & Quality

| Action | Primary | Review | Approver |
|--------|---------|--------|----------|
| Vitest config + unit tests | Testing Chief | QA Chief | CTO |
| Vitest integration tests | Testing Chief | QA Chief | CTO |
| React Testing Library tests | Testing Chief | QA Chief | CTO |
| MSW handlers | Testing Chief | Backend Chief | CTO |
| Playwright E2E (10 paths) | Testing Chief | QA Chief | CTO |
| Storybook setup + stories | Frontend + Documentation Chief | UI/UX Chief | CTO |
| Storybook a11y addon | Frontend Chief | UI/UX Chief | CTO |
| WCAG AA+ audit | UI/UX Chief | QA Chief | CTO |
| axe-core CI integration | Testing + DevOps Chief | Security Chief | CTO |
| Coverage threshold enforcement | Testing Chief | QA Chief | CTO |

### 3.5 Block E — Security

| Action | Primary | Review | Approver |
|--------|---------|--------|----------|
| CSP header generation | Security Chief | Architecture Chief | CTO |
| CSRF token implementation | Security Chief | Backend Chief | CTO |
| Rate limiting middleware | Security + Backend Chief | Architecture Chief | CTO |
| Security headers (HSTS, XFO, etc.) | Security Chief | DevOps Chief | CTO |
| JWT security hardening | Security Chief | Backend Chief | CTO |
| Input sanitization | Security Chief | Backend Chief | CTO |
| OWASP Top 10 review | Security Chief | Architecture Chief | CTO |
| Secrets vault backend | Backend + Security Chief | Architecture Chief | CTO |
| Audit log backend + viewer | Backend + Frontend Chief | Security Chief | CTO |

### 3.6 Block F — Responsiveness & PWA

| Action | Primary | Review | Approver |
|--------|---------|--------|----------|
| Mobile layout fixes (320px) | Frontend + UI/UX Chief | QA Chief | CTO |
| Tablet layout audit (768px) | Frontend Chief | UI/UX Chief | CTO |
| Desktop + ultrawide (2560px) | Frontend Chief | UI/UX Chief | CTO |
| Touch target enforcement | UI/UX Chief | QA Chief | CTO |
| Bottom nav (mobile) vs sidebar | Frontend Chief | UI/UX Chief | CTO |
| PWA manifest + service worker | Frontend + DevOps Chief | Architecture Chief | CTO |
| Offline mode | Frontend + DevOps Chief | QA Chief | CTO |
| Installability | Frontend Chief | UI/UX Chief | CTO |

---

## 4. Authorization Gates

### 4.1 Gate Hierarchy

```
GATE-0: Pre-Implementation (all-or-nothing, blocks all blocks)
  │
  ├──► GATE-A: Block A (15 pages)
  │      ├── Sprint A.1 (6 pages: Orchestration, Playground, History, Metrics, Pipeline, Compare)
  │      └── Sprint A.2 (9 pages: Audit, Secrets, Plugins, Context, Templates, Prompts, Analytics, Activity, Admin)
  │
  ├──► GATE-B: Block B (Design System)
  │      ├── Sprint B.1 (DataGrid, Charts)
  │      ├── Sprint B.2 (Notifications, Split Panels, Timeline)
  │      └── Sprint B.3 (Tour, Shortcuts, Breadcrumb, Global Search, Markdown, Theme, Docs)
  │
  ├──► GATE-C: Block C (Performance) — single sprint
  │
  ├──► GATE-D: Block D (Tests & Quality)
  │      ├── Sprint D.1 (Vitest, RTL, MSW)
  │      └── Sprint D.2 (Playwright, Storybook, A11y, Coverage)
  │
  ├──► GATE-E: Block E (Security) — single sprint
  │
  └──► GATE-F: Block F (Responsiveness) — single sprint
```

### 4.2 Gate Protocol

**Before each BLOCK:**
1. Kernel presents block plan (this doc, scoped to the block)
2. User approves the block explicitly
3. Kernel verifies all prerequisites (backend endpoints are ✅)
4. Kernel activates required agents
5. Work begins

**After each BLOCK:**
1. Each agent reports completion with evidence
2. Kernel verifies: `pnpm typecheck` (0 errors), `pnpm build` (success), `pnpm lint` (0 errors), `go test ./...` (all pass)
3. Kernel presents results + verification to user
4. User approves block completion
5. Move to next block

**During each SPRINT:**
1. Kernel presents sprint steps to user
2. User approves sprint
3. Agents execute sequentially

**During each STEP:**
1. Agent reads existing code first (must read before editing)
2. Agent explains what will be created/modified
3. Agent requests authorization from Kernel
4. Kernel approves or requests changes
5. Agent implements
6. Agent verifies (`pnpm typecheck` + `pnpm build` or `go vet` + `go build`)
7. Agent reports completion

**Violation consequence:** Any step implemented without gate authorization is reverted.

---

## 5. Block A: Remaining Pages

### 5.1 Sprint A.1 — Core Pages (Week 1)

```
BLOCK A: 15 Pages
│
├── Sprint A.1: 6 Core Pages
│   ├── A.1.1: Orchestration Console (/orchestration)
│   ├── A.1.2: AI Playground (/playground)
│   ├── A.1.3: Execution History (/executions)
│   ├── A.1.4: System Metrics Dashboard (/metrics)
│   ├── A.1.5: Pipeline Editor (/pipelines)
│   └── A.1.6: Provider Compare (/providers/compare)
│
└── Sprint A.2: 9 Enterprise Pages
    ├── A.2.1: Audit Log Viewer (/admin/audit)
    ├── A.2.2: Secrets Vault (/admin/secrets)
    ├── A.2.3: Plugin Registry Browser (/plugins)
    ├── A.2.4: Context Builder (/context)
    ├── A.2.5: Template Manager (/templates)
    ├── A.2.6: Prompt Library (/prompts)
    ├── A.2.7: Search Analytics (/analytics)
    ├── A.2.8: Activity Monitor (/activity)
    └── A.2.9: Admin Overview (/admin)
```

---

#### Step A.1.1: Orchestration Console (`/orchestration`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` (primary), `cosca-architecture` (review) |
| **Input** | `POST /v1/run`, `POST /v1/run/stream`, `GET /v1/agents/search`, `GET /v1/providers/list` |
| **Precondition** | `POST /v1/run` endpoint wired (P0) |
| **Success** | User types prompt, selects agent/provider, executes — result streams in real-time or displays on completion |

**Feature module:** `web/src/features/orchestration/`

```
orchestration/
├── components/
│   ├── PromptInput.tsx         # Textarea + agent/provider selectors + run button
│   ├── StreamingOutput.tsx     # Real-time SSE output display
│   ├── ResultCard.tsx          # Final result: response, agent badge, skills, duration
│   ├── AgentSelector.tsx       # Searchable combobox from GET /v1/agents/search
│   ├── ProviderSelector.tsx    # Dropdown from GET /v1/providers/list
│   └── HistorySidebar.tsx      # Recent orchestrations sidebar
├── hooks/
│   ├── use-orchestration-run.ts     # useMutation for POST /v1/run
│   ├── use-orchestration-stream.ts  # SSE hook for POST /v1/run/stream
│   └── use-execution-history.ts     # useQuery for GET /v1/executions
├── types.ts
└── index.ts
```

**Key implementation details:**
- PromptInput: Full-width textarea, `Cmd/Ctrl+Enter` shortcut, disabled when empty
- StreamingOutput: JetBrains Mono font, markdown rendering, progress phases ("Thinking..." → "Searching..." → "Routing..." → "Generating...")
- ResultCard: Markdown response, agent badge, skills chips, duration, memory ID link
- Route: `web/src/app/(dashboard)/orchestration/page.tsx`
- Nav update: Add `{ label: "Orchestration", href: "/orchestration", icon: "Orbit", shortcut: "G O" }`

---

#### Step A.1.2: AI Playground (`/playground`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` + `cosca-ai` (primary), `cosca-cto` (review) |
| **Precondition** | `POST /v1/run` working, multiple chat providers registered |
| **Success** | Send prompt to 3+ providers simultaneously, see side-by-side comparison |

**Feature module:** `web/src/features/playground/`

```
playground/
├── components/
│   ├── PromptBar.tsx           # Fixed prompt input
│   ├── ProviderTab.tsx         # Per-provider tab with model, status, output
│   ├── CompareView.tsx         # Side-by-side comparison grid
│   ├── OutputArea.tsx          # Markdown/code output with highlighting
│   └── SettingsPanel.tsx       # Temperature, max_tokens, system prompt override
├── hooks/
│   ├── use-compare-run.ts      # Parallel execution across providers
│   └── use-provider-stream.ts  # Multi-provider SSE handling
├── types.ts
└── index.ts
```

**Key implementation details:**
- ProviderTab: Icon, model name, status dot (idle/running/done/error), execution time, close button
- CompareView: Side-by-side result cards. Winner highlighted (fastest + cheapest)
- SettingsPanel: Sliders for Temperature (0-2), Max Tokens (100-4096), Top P (0-1), system prompt override
- Route: `web/src/app/(dashboard)/playground/page.tsx`
- Nav update: Add `{ label: "Playground", href: "/playground", icon: "FlaskConical", shortcut: "G Y" }`

---

#### Step A.1.3: Execution History (`/executions`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` (primary), `cosca-architecture` (review) |
| **Input** | `GET /v1/executions`, `GET /v1/executions/{id}` |
| **Precondition** | `GET /v1/executions` wired (P0) |
| **Success** | Table loads 20 recent executions; click to view pipeline trace detail |

**Implementation:**
- List page: DataGrid columns (Timestamp, Prompt truncated, Agent, Provider, Status, Duration, Actions)
- Detail page: Full prompt, full response (Markdown), Pipeline trace (stages, durations), skills, memory link
- Filters: Date range, agent, provider, status
- Routes: `/executions` and `/executions/[id]`
- Nav update: Add `{ label: "Executions", href: "/executions", icon: "History", shortcut: "G E" }`

---

#### Step A.1.4: System Metrics Dashboard (`/metrics`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` (primary), `cosca-monitoring` (review) |
| **Input** | `GET /v1/metrics`, reuses Chart components from Block B |
| **Precondition** | `GET /v1/metrics` wired (P1) |
| **Success** | 8 chart widgets render, auto-refresh every 30s |

**Widgets (use Recharts from Block B):**
1. Requests over time (Line chart, colored by agent)
2. Token usage (Stacked bar, per-day per-provider)
3. Latency distribution (Area chart: p50/p90/p99)
4. Error rate (Donut: Success vs Error)
5. Knowledge index size (Counter card + sparkline)
6. Memory records (Counter card + sparkline)
7. Active sessions (Counter card + trend)
8. Top agents (Horizontal bar chart)

Auto-refresh via `refetchInterval: 30_000`. Time range selector: Last hour/day/week/month.

---

#### Step A.1.5: Pipeline Editor (`/pipelines`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` + `cosca-backend` (primary), `cosca-architecture` (review) |
| **Success** | Drag-and-drop stage ordering, save/load pipeline definitions |

**Layout:**
- Left sidebar: Stage palette (draggable: Context Builder, Router, Executor, Memory Storer, Skill Processor, Condition, Parallel, Custom)
- Center canvas: Stage nodes connected by arrows (React Flow or custom SVG)
- Right sidebar: Selected stage properties (name, timeout, onError, condition, skill, input/output mapping)
- Save serializes visual layout to PipelineDefinition JSON; Load deserializes

---

#### Step A.1.6: Provider Compare (`/providers/compare`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` + `cosca-ai` (primary), `cosca-architecture` (review) |
| **Precondition** | `POST /v1/providers/compare` (P2) or client-side parallel calls |
| **Success** | Select 2-4 providers, enter prompt, side-by-side results with latency/tokens/cost |

**Implementation:**
- Provider selection: Checkbox grid (name, model, status, $/1K tokens)
- Results grid: Side-by-side cards with provider header, output, metrics
- Winner highlight: Fastest + cheapest provider highlighted

---

### 5.2 Sprint A.2 — Enterprise Pages (Week 2)

#### Step A.2.1: Audit Log Viewer (`/admin/audit`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` (primary), `cosca-security` (review) |
| **Precondition** | Audit log storage + API (P2) |
| **Success** | Filterable DataGrid of audit events, CSV export, admin-only |

**Columns:** Timestamp, User, Action, Resource, Status (success/denied/error), IP Address, Details (expandable)
**Filters:** Date range, user, action type, resource, status
**Role gate:** `requiredRole="admin"`

---

#### Step A.2.2: Secrets Vault (`/admin/secrets`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` (primary), `cosca-security` (review) |
| **Precondition** | Secrets vault CRUD API (P2) |
| **Success** | Create, list (masked values), reveal, update, delete secrets |

**Implementation:**
- List: Key name, masked value `••••••`, type, timestamps, actions
- Create/Edit: Key name, value (show/hide toggle), type dropdown, description
- Delete: Confirmation with type-the-key-name verification
- Reveal: Auto-hides after 30s or page blur
- Role gate: `requiredRole="admin"`

---

#### Step A.2.3: Plugin Registry Browser (`/plugins`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` (primary), `cosca-architecture` (review) |
| **Note** | Currently disabled in NAV_ITEMS. Remove `disabled: true`. |
| **Success** | Browse installed plugins with status, version, runtime; enable/disable toggle |

**Implementation:**
- Plugin cards: Name, version, status (active/inactive/error), runtime type, description, toggle switch
- Filter by status and runtime type
- Detail view: Full plugin manifest, permissions, dependencies

---

#### Step A.2.4: Context Builder Page (`/context`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` (primary), `cosca-architecture` (review) |
| **Success** | Build and preview the context sent to LLM: knowledge, memories, agent instructions, system prompt |

**Implementation:**
- Build button triggers context assembly for a hypothetical prompt
- Preview: Accordion sections (System prompt, Agent instructions, Retrieved knowledge, Retrieved memories, Final assembled prompt)
- Token counter: Total tokens used
- "Copy full context" button

---

#### Step A.2.5: Template Manager (`/templates`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` (primary), `cosca-architecture` (review) |
| **Success** | Browse, preview, and use templates for common orchestration patterns |

**Implementation:**
- Template cards: Name, description, category tags, "Use Template" button
- Preview: Rendered template with variable placeholders highlighted
- Variables pane: Fill in → generates final prompt → send to Orchestrator
- Categories: Code Review, Architecture, Bug Analysis, Documentation, Testing, Deployment

---

#### Step A.2.6: Prompt Library (`/prompts`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` + `cosca-ai` (primary), `cosca-architecture` (review) |
| **Success** | Browse, search, create, favorite prompts |

**Implementation:**
- Prompt cards: Title, preview (100 chars), author, usage count, starred
- Create/Edit: Title, content (syntax highlighted), tags, visibility (private/shared)
- Search: Full-text across all prompts
- "Send to Orchestrator": One-click to `/orchestration` with pre-filled prompt

---

#### Step A.2.7: Search Analytics (`/analytics`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` + `cosca-analytics` (primary), `cosca-architecture` (review) |
| **Success** | Top queries, zero-result queries, search trends, click-through rate |

**Implementation:**
- Top queries (Bar chart), Zero-result queries (List with knowledge gap indicator), Search trends (Line chart), Click-through rate (Donut), Avg result position (Stat card)

---

#### Step A.2.8: Activity Monitor (`/activity`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` (primary), `cosca-monitoring` (review) |
| **Input** | WebSocket `GET /ws`, `GET /v1/knowledge/recent`, `GET /v1/memory/recent` |
| **Precondition** | WebSocket (P1), recent endpoints (P1) |
| **Success** | Real-time activity stream; events appear as they happen |

**Implementation:**
- Real-time feed via WebSocket: Orchestration completed, Memory stored, Knowledge indexed, System alert
- Cards: Event icon, relative timestamp, description, action link
- Filter by event type; Pause/resume feed
- Infinite scroll for historical events

---

#### Step A.2.9: Admin Overview Dashboard (`/admin`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` (primary), `cosca-security` (review) |
| **Success** | Single-page admin overview with quick actions |

**Implementation:**
- Stat cards: Total users, Active API keys, Secrets count, Audit events today
- Quick actions: Create User, Generate API Key, View Audit Log, Manage Secrets
- Recent activity: Last 10 audit events
- System health card (reuse from Dashboard)
- Role gate: `requiredRole="admin"`

---

## 6. Block B: Design System & UX

```
BLOCK B: Design System & UX (2 weeks, 3 sprints)
│
├── Sprint B.1: DataGrid Enterprise + Charts (Week 1)
│   ├── B.1.1: DataGrid Types
│   ├── B.1.2: Server-Side Pagination
│   ├── B.1.3: Column Sorting
│   ├── B.1.4: Column Filtering
│   ├── B.1.5: Column Pinning + Row Selection
│   ├── B.1.6: CSV/JSON Export
│   ├── B.1.7-B.1.9: DataGrid Tests + Storybook + A11y
│   ├── B.1.10-B.1.12: Charts (Line, Bar, Pie, Area, Scatter, Radar)
│   ├── B.1.13: Chart Theme Integration
│   └── B.1.14-B.1.15: Chart Tests + Storybook
│
├── Sprint B.2: Notifications, Split Panels, Timeline (Week 1-2)
│   ├── B.2.1: Notification Bell + Badge
│   ├── B.2.2: Notification History Panel
│   ├── B.2.3: WebSocket Integration
│   ├── B.2.4: Notification Preferences
│   ├── B.2.5: Toast Enhancements
│   ├── B.2.6: Split View / Resizable Panels
│   ├── B.2.7: Timeline / Activity Feed
│   └── B.2.8: Timeline Tests + Storybook
│
└── Sprint B.3: UX Enhancers (Week 2)
    ├── B.3.1: Onboarding Tour
    ├── B.3.2: Keyboard Shortcuts Panel
    ├── B.3.3: Breadcrumb Navigation
    ├── B.3.4: Advanced Global Search
    ├── B.3.5: Markdown Renderer (with Mermaid)
    ├── B.3.6: Enhanced Theme Switcher
    └── B.3.7: Component Documentation Hub (/docs)
```

---

### Sprint B.1: DataGrid Enterprise + Charts (Week 1)

#### Steps B.1.1 — B.1.6: DataGrid Component

| Step | Output | Key Details |
|------|--------|-------------|
| **B.1.1** | `web/src/components/shared/data-grid/types.ts` | `ColumnDef<T>`, `SortState`, `FilterState`, `PaginationState`, `DataGridProps<T>` |
| **B.1.2** | `pagination.tsx` | Page size (10/25/50/100), First/Prev/Next/Last, "Showing 1-25 of 1,247". Compact on mobile |
| **B.1.3** | `sorting.tsx` | Click header → asc → desc → remove. Icons: ▲/▼/⇅. Multi-sort with Ctrl+Click |
| **B.1.4** | `filters.tsx` | Filter icon → dropdown (operators: contains/equals/gt/lt/between/in). Active filter chips |
| **B.1.5** | `pinning.tsx`, `selection.tsx` | Right-click → Pin Left/Right. Checkbox column, select-all, bulk action bar |
| **B.1.6** | `export.tsx` | CSV/JSON download, respects filters. Filename: `cosca-export-{timestamp}.csv` |

**DataGrid Props Contract:**
```typescript
interface DataGridProps<T> {
  columns: ColumnDef<T>[];
  data: T[];
  total: number;
  pagination: PaginationState;
  sorting: SortState[];
  filters: FilterState[];
  onPaginationChange: (p: PaginationState) => void;
  onSortingChange: (s: SortState[]) => void;
  onFiltersChange: (f: FilterState[]) => void;
  onRowSelectionChange?: (ids: string[]) => void;
  onExport?: (format: "csv" | "json") => void;
  isLoading?: boolean;
  emptyMessage?: string;
  errorMessage?: string;
}
```

#### Steps B.1.7 — B.1.9: DataGrid Quality Gates

| Step | Agent | Output | Success |
|------|-------|--------|---------|
| **B.1.7** Tests | `cosca-testing` | `__tests__/` (20+ tests) | Pagination, sort, filter, pin, select, export, empty, error, loading states. >80% coverage |
| **B.1.8** Storybook | `cosca-frontend` + `cosca-documentation` | `data-grid.stories.tsx` | 10 stories: Default, Sortable, Filterable, Pinned, Selection, Loading, Empty, Error, 10K Rows (virtualized), Compact |
| **B.1.9** A11y | `cosca-uiux` | Review report | `<table>` + `<thead>`, `aria-sort`, keyboard nav, screen reader, ≥4.5:1 contrast |

**Storybook stories for DataGrid:**
1. Default — Simple columns, basic pagination
2. Sortable — All columns, indicator icons
3. Filterable — Dropdowns on text/number columns
4. Pinned Columns — First/last pinned with borders
5. With Row Selection — Checkboxes, bulk action bar
6. Loading — Skeleton rows
7. Empty — "No data found" message
8. Error — Error message with retry button
9. 10K Rows — Performance with virtualized rows
10. Compact — Small variant for dashboard widgets

#### Steps B.1.10 — B.1.15: Charts (Recharts)

**Chart types:**

| Step | Type | Use Case | Features |
|------|------|----------|----------|
| B.1.10a | LineChart | Time series (metrics) | Single/multi-series, areas, dashed |
| B.1.10b | BarChart | Categories (top agents) | Vertical/horizontal, stacked/grouped |
| B.1.10c | PieChart / DonutChart | Proportions (error rate) | Inner radius, labels inside/outside |
| B.1.10d | AreaChart | Cumulative trends (latency) | Single/stacked, gradient fill |
| B.1.10e | ScatterChart | Correlation (tokens vs latency) | Points, bubble (size dimension) |
| B.1.10f | RadarChart | Multi-metric (provider capabilities) | Overlay, fill opacity |

**Custom components:**

| Step | Output | Description |
|------|--------|-------------|
| B.1.11 | `tooltip.tsx`, `legend.tsx` | Custom tooltip (date, value, label), interactive legend (click toggles) |
| B.1.12 | `responsive-container.tsx` | Recharts `ResponsiveContainer` with debounced resize |
| B.1.13 | CSS variable integration | Charts use `--chart-1` through `--chart-5` from Tailwind theme |
| B.1.14 | `__tests__/` | Render, tooltip/hover, legend toggle, responsive resize |
| B.1.15 | `charts.stories.tsx` | Each type with mock data, dark/light, interactive controls |

**Accessibility:** `role="img"` + `aria-label`, tabular `<table>` with `sr-only`, colorblind-safe palette, keyboard-navigable data points.

---

### Sprint B.2: Notifications, Split Panels, Timeline (Week 1-2)

#### Step B.2.1: Notification Bell + Badge

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | `web/src/components/layout/notification-bell.tsx` (planned) |
| **Success** | Bell in Topbar, red badge with unread count, click opens notification panel. CSS bell-ring animation on new notification |

#### Step B.2.2: Notification History Panel

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | `web/src/components/shared/notification-center/notification-panel.tsx` |
| **Success** | Slide-out (Sheet), 400px desktop, full-width mobile. Last 50 notifications grouped by date (Today/Yesterday/This Week/Older) |

**Features:** Filters (All, Unread, Orchestration, System, Memory, Knowledge). Actions: "Mark all as read", "Clear all". Persistence via `localStorage`.

#### Step B.2.3: WebSocket Integration

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` + `cosca-backend` |
| **Output** | `web/src/hooks/use-websocket.ts` (planned), `web/src/lib/websocket.ts` (planned) |
| **Success** | Connects to `ws://localhost:14120/ws`, dispatches events. Reconnection with exponential backoff (max 5 retries) |

**Event types:** `orchestration.started/completed/failed`, `system.alert`, `memory.updated`, `knowledge.indexed`

```typescript
class WebSocketManager {
  connect(url: string, token: string): void
  disconnect(): void
  on(event: string, callback: (data: unknown) => void): void
  off(event: string, callback: (data: unknown) => void): void
}
```

#### Step B.2.4: Notification Preferences

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | `web/src/app/(dashboard)/settings/notifications/page.tsx` |
| **Success** | Toggle toast/bell per event type (Orchestration started/completed/failed, System alert, Memory stored, Knowledge indexed, Audit event, Security alert) |

#### Step B.2.5: Toast Enhancements

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Description** | Style Sonner toasts to design system. Add action buttons. `Ctrl+Shift+X` to dismiss all. |

#### Step B.2.6: Split View / Resizable Panels

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | `web/src/components/shared/split-view/` |
| **Success** | Drag divider → resize. Double-click → 50/50. Collapse button. Keyboard (Arrow keys). Nested splits. Sizes persisted in `localStorage` |

#### Step B.2.7: Timeline / Activity Feed

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | `web/src/components/shared/timeline/` |
| **Success** | Vertical timeline, alternating left/right on desktop (all-left on mobile). Grouped by date. Infinite scroll via `useInfiniteQuery` |

#### Step B.2.8: Timeline Tests + Storybook

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-testing` + `cosca-frontend` |
| **Output** | `__tests__/timeline.test.tsx`, `timeline.stories.tsx` |

---

### Sprint B.3: UX Enhancers (Week 2)

#### Step B.3.1: Onboarding Tour

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` + `cosca-uiux` |
| **Output** | `web/src/components/shared/tour/` (using `react-joyride`) |
| **Success** | 6-step tour: Dashboard, Knowledge, Memory, Orchestration, Sidebar, Theme. Skip/dismiss, replay via "Help → Restart Tour". Persisted in `localStorage` |

#### Step B.3.2: Keyboard Shortcuts Panel

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | `web/src/components/shared/keyboard-shortcuts.tsx` |
| **Success** | Press `?` → modal shows all shortcuts grouped by category (Navigation, Global, Actions, Theme), searchable |

**Key shortcuts:**
| Category | Shortcuts |
|----------|-----------|
| Navigation | G+D, G+K, G+M, G+R, G+O, G+A, G+L, G+W, G+Y, G+S |
| Global | Ctrl+K (Palette), Ctrl+Shift+F (Search), ? (Shortcuts), Ctrl+B (Sidebar), Escape (Close) |
| Actions | Ctrl+Enter (Submit), Ctrl+S (Save), Ctrl+Z (Undo) |
| Theme | Ctrl+Shift+D (Dark), Ctrl+Shift+L (Light) |

#### Step B.3.3: Breadcrumb Navigation

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | `web/src/components/layout/breadcrumb.tsx` |
| **Description** | Dynamic from route, shown in Topbar. Example: `Home > Orchestration > Execution #1247` |

#### Step B.3.4: Advanced Global Search

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | `web/src/components/shared/global-search.tsx` |
| **Success** | `Ctrl+Shift+F` → modal → searches knowledge, memory, agents, skills, workflows, history. Results grouped by type with icons. Query highlighting. Recent searches in `localStorage` |

#### Step B.3.5: Markdown Renderer (with Mermaid)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | `web/src/components/shared/markdown/` |
| **Success** | `react-markdown` + `remark-gfm` + `rehype-highlight`. Headings with anchors, syntax-highlighted code blocks (dark/light themes), tables (responsive scroll), task lists, Mermaid diagrams. Copy button on code blocks |

#### Step B.3.6: Enhanced Theme Switcher

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Description** | Click cycles Dark → Light → System. Hold: Auto-schedule (dark at night). Per-page preference option |

#### Step B.3.7: Component Documentation Hub (`/docs`)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` + `cosca-documentation` |
| **Output** | `web/src/app/(dashboard)/docs/page.tsx` (planned; implemented as in-component docs) |
| **Description** | Internal component library: usage examples, props tables, code snippets for DataGrid, Charts, Notification Center, Split View, Timeline, Tour, Shortcuts, Markdown |


## 7. Block C: Performance

```
BLOCK C: Performance (0.5 weeks — single sprint)

Sprint C.1: Performance Optimization
├── C.1.1: Partial Prerendering (PPR)
├── C.1.2: Incremental Static Regeneration (ISR)
├── C.1.3: Streaming SSR (Suspense boundaries)
├── C.1.4: Bundle Analysis + Optimization
├── C.1.5: Image Optimization
├── C.1.6: Font Optimization
├── C.1.7: API Response Caching Tuning
└── C.1.8: Lighthouse Audit + Benchmarking
```

---

#### Step C.1.1: Partial Prerendering (PPR)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Precondition** | Next.js 15.1.0 (already in use) |
| **Output** | `next.config.ts`: `experimental: { ppr: "incremental" }`. Page-level Suspense boundaries |
| **Success** | Static shell (sidebar, topbar) from edge cache; dynamic content streams in |

**PPR-optimized:** `/dashboard`, `/knowledge`, `/memory`, `/settings`

#### Step C.1.2: Incremental Static Regeneration (ISR)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | ISR directives on slowly-changing pages |
| **Success** | Static pages revalidate without full rebuild |

**Pages:** `/settings` (revalidate: 300s), `/docs` (revalidate: 3600s)

#### Step C.1.3: Streaming SSR (Suspense Boundaries)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | `<Suspense>` around independently-loading sections |
| **Success** | Sidebar first (immediate), then stat cards (500ms), then charts (1s), then activity feed (2s) — none block each other |

**Applied to:** Dashboard, Metrics, Activity Monitor, Audit Log Viewer

```tsx
<Suspense fallback={<StatsSkeleton />}>
  <QuickStats />
</Suspense>
<Suspense fallback={<ChartSkeleton />}>
  <MetricsCharts />
</Suspense>
```

#### Step C.1.4: Bundle Analysis + Optimization

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | Bundle report + optimization changes |
| **Success** | <200KB initial JS (gzipped) per route, no duplicate deps |

**Process:** `ANALYZE=true pnpm build` → review → dynamic imports for heavy modules → `pnpm dedupe` → re-analyze

```typescript
const Charts = dynamic(() => import("@/components/shared/charts"), { ssr: false });
const PipelineEditor = dynamic(() => import("@/features/pipelines"), { ssr: false });
```

#### Step C.1.5: Image Optimization

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | All images using `<Image>` from `next/image` |
| **Success** | No raw `<img>` tags. `width/height/sizes/priority` props set |

#### Step C.1.6: Font Optimization

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | Optimized `next/font/google` for Inter + JetBrains Mono |
| **Success** | No FOIT, fonts preloaded, zero layout shift. `display: "swap"`, variable fonts |

#### Step C.1.7: API Response Caching Tuning

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | Optimized `staleTime` + `gcTime` per endpoint |
| **Success** | No unnecessary refetches; appropriate freshness per data type |

**Tuning table:**

| Endpoint | staleTime | gcTime | refetchInterval |
|----------|-----------|--------|-----------------|
| `/v1/health` | 5s | 30s | 10s |
| `/v1/status` | 10s | 60s | 15s |
| `/v1/metrics` | 30s | 5min | 30s |
| `/v1/knowledge/stats` | 60s | 10min | — |
| `/v1/memory/stats` | 60s | 10min | — |
| `/v1/knowledge/search` | 30s | 5min | — |
| `/v1/agents` | 5min | 30min | — |
| `/v1/skills` | 5min | 30min | — |
| `/v1/workflows` | 5min | 30min | — |
| `/v1/executions` | 10s | 2min | 15s |
| `/v1/audit/logs` | 30s | 5min | — |
| `/v1/users` | 5min | 30min | — |

#### Step C.1.8: Lighthouse Audit + Benchmarking

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | `docs/roadmap/lighthouse-baseline.md` with scores for all 32 pages |
| **Success** | All pages: Performance >90, Accessibility >95, Best Practices >90, SEO >90 |

**Process:** `pnpm build && pnpm start` → Run Lighthouse on key pages → fix <85/90 → document baselines


## 8. Block D: Tests & Quality

```
BLOCK D: Tests & Quality (1.5 weeks)
│
├── Sprint D.1: Unit + Integration Tests (Week 1)
│   ├── D.1.1: Vitest Configuration
│   ├── D.1.2: MSW Handlers Setup
│   ├── D.1.3: Shared Component Unit Tests (>95 tests)
│   ├── D.1.4: Feature Hook Integration Tests (>39 tests)
│   ├── D.1.5: Page Integration Tests — RTL (>30 tests)
│   └── D.1.6: Coverage Threshold Enforcement
│
└── Sprint D.2: E2E + Storybook + A11y (Week 2)
    ├── D.2.1: Playwright Configuration
    ├── D.2.2: Playwright E2E (10 critical paths)
    ├── D.2.3: Storybook Setup
    ├── D.2.4: Storybook Stories (>25 components)
    ├── D.2.5: Storybook A11y + Interaction Tests
    ├── D.2.6: axe-core WCAG 2.1 AA+ Audit
    └── D.2.7: axe-core CI Integration
```

---

### Sprint D.1: Unit + Integration Tests (Week 1)

#### Step D.1.1: Vitest Configuration

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-testing` |
| **Output** | `web/vitest.config.ts`, `web/src/test-setup.ts` |
| **Success** | `pnpm vitest run` runs all tests |

```typescript
// vitest.config.ts
export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test-setup.ts"],
    globals: true,
    css: true,
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html", "lcov"],
      exclude: ["node_modules/", ".next/", "**/*.stories.tsx", "**/*.test.tsx"],
      thresholds: { statements: 80, branches: 80, functions: 80, lines: 80 },
    },
  },
  resolve: { alias: { "@": path.resolve(__dirname, "./src") } },
});
```

```typescript
// src/test-setup.ts
import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";
afterEach(() => cleanup());
```

#### Step D.1.2: MSW Handlers Setup

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-testing` |
| **Output** | `web/src/test/mocks/handlers.ts`, `server.ts` |
| **Success** | All 35+ API endpoints have realistic mock handlers |

#### Step D.1.3: Shared Component Unit Tests

**Test targets (>95 tests):**

| Component | Tests | Key Cases |
|-----------|-------|-----------|
| DataGrid | 20 | Pagination, sort, filter, pin, select, export, empty, error, loading |
| Charts (6 types) | 12 | Each type renders, tooltip, legend, resize |
| NotificationPanel | 8 | Open/close, render list, mark read, clear all |
| SplitView | 6 | Render, resize, collapse, keyboard, persistence |
| Timeline | 6 | Items, grouping, infinite scroll, empty |
| Tour | 4 | Steps, skip, complete, replay |
| KeyboardShortcuts | 4 | Modal open/close, display, search |
| GlobalSearch | 6 | Open/close, search, results, navigate, recent |
| Markdown | 8 | Headings, code, tables, Mermaid, images, links |
| StatCard | 4 | Render, loading, error, trend |
| StatusBadge | 5 | Each variant, contrast, label |
| Skeleton | 2 | Render, custom dimensions |
| EmptyState | 3 | Icon, message, action |
| ErrorState | 3 | Message, retry, dismiss |
| Breadcrumb | 3 | Segments, links, current page |

#### Step D.1.4: Feature Hook Integration Tests (>39 tests)

| Feature | Hooks Tested | Test Count |
|---------|-------------|------------|
| dashboard | use-dashboard | 3 |
| knowledge | use-knowledge-search, use-knowledge-stats | 4 |
| memory | use-memory-search, use-memory-stats, use-memory-record | 6 |
| orchestration | use-run, use-stream, use-history | 6 |
| providers | use-providers | 2 |
| agents | use-agents | 2 |
| skills | use-skills | 2 |
| workflows | use-workflows | 2 |
| audit | use-audit-logs | 2 |
| secrets | use-secrets | 3 |
| users | use-users | 2 |
| api-keys | use-api-keys | 2 |
| settings | use-settings | 3 |

#### Step D.1.5: Page Integration Tests — RTL (>30 tests)

**Pages (2+ tests each):** Dashboard, Knowledge Explorer, Memory Viewer, Orchestration Console, AI Playground, Settings, Audit Log, Secrets Vault, Login, Admin Users

#### Step D.1.6: Coverage Threshold Enforcement

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-testing` |
| **Output** | CI step: fails if coverage <80% |
| **Scripts** | `"test": "vitest run"`, `"test:coverage": "vitest run --coverage"` |

---

### Sprint D.2: E2E + Storybook + A11y (Week 2)

#### Step D.2.1: Playwright Configuration

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-testing` |
| **Output** | `web/playwright.config.ts`, `web/tests/` |
| **Success** | `npx playwright test` runs in Chromium, Firefox, WebKit |

**Config:** `fullyParallel: true`, `retries: CI ? 2 : 0`, `webServer: pnpm dev`, `trace: on-first-retry`

#### Step D.2.2: Playwright E2E — 10 Critical Paths

| # | Path | Priority | Test |
|---|------|----------|------|
| CP-01 | Login → Dashboard | 🔴 P0 | Login valid creds, verify dashboard stats |
| CP-02 | Knowledge Search | 🔴 P0 | Navigate, type query, submit, verify results |
| CP-03 | Memory Browse | 🔴 P0 | Navigate, select layer, verify records, detail |
| CP-04 | Orchestration Run | 🔴 P0 | Navigate, prompt, select agent, run, verify result |
| CP-05 | Settings View | 🟡 P1 | Navigate, verify config sections, plugin list |
| CP-06 | Providers List | 🟡 P1 | Navigate, verify cards, test provider |
| CP-07 | Command Palette | 🟡 P1 | Ctrl+K, search, navigate |
| CP-08 | Admin — Create User | 🟡 P1 | Admin login, create user, verify, delete |
| CP-09 | Theme Toggle | 🟢 P2 | Toggle dark/light/system, verify on pages |
| CP-10 | Error Graceful Degradation | 🟢 P2 | Stop backend, verify error states show retry |

**Example (CP-01):**
```typescript
test("CP-01: Login → Dashboard", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill("admin123");
  await page.getByRole("button", { name: "Sign In" }).click();
  await expect(page).toHaveURL("/dashboard");
  await expect(page.getByText("Knowledge Docs")).toBeVisible();
});
```

#### Step D.2.3 — D.2.5: Storybook

| Step | Output | Success |
|------|--------|---------|
| **D.2.3** Setup | `.storybook/main.ts`, `.storybook/preview.ts` | `pnpm storybook` on port 6006 |
| **D.2.4** Stories | 25+ `*.stories.tsx` files | All shared components with dark/light variants |
| **D.2.5** A11y + Interactions | A11y violations + interaction tests | 0 critical a11y violations in Storybook |

**Components needing stories (>25):** Button, Input, Card, Badge, Select, Dialog, Sheet, Tooltip, DropdownMenu, Tabs, Progress, Skeleton, Toast, EmptyState, ErrorState, StatusBadge, StatCard, DataGrid, LineChart, BarChart, PieChart, AreaChart, NotificationPanel, SplitView, Timeline, Tour, KeyboardShortcuts, Breadcrumb, Markdown

**Preview config (dark-first):**
```typescript
const preview: Preview = {
  parameters: {
    backgrounds: {
      default: "dark",
      values: [{ name: "dark", value: "#09090b" }, { name: "light", value: "#ffffff" }],
    },
  },
};
```

#### Step D.2.6: axe-core WCAG 2.1 AA+ Audit

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-uiux` |
| **Output** | `docs/roadmap/accessibility-audit.md` |
| **Success** | 0 critical or serious violations on all 32 pages |

**WCAG criteria:**
| Criterion | Method |
|-----------|--------|
| 1.1.1 Non-text Content | axe-core + manual |
| 1.3.1 Info and Relationships | axe-core (semantic HTML) |
| 1.4.1 Use of Color | Manual (text labels) |
| 1.4.3 Contrast (Min) | axe-core (≥4.5:1) |
| 2.1.1 Keyboard | Manual (Tab through all pages) |
| 2.4.3 Focus Order | Manual (logical order) |
| 2.4.7 Focus Visible | Manual (ring visible) |
| 3.3.2 Labels | axe-core |
| 4.1.2 Name, Role, Value | axe-core |
| 4.1.3 Status Messages | Manual (aria-live) |

#### Step D.2.7: axe-core CI Integration

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-testing` + `cosca-devops` |
| **Output** | GitHub Actions step |
| **Success** | PR blocked if a11y violations increase |

```typescript
const results = await new AxeBuilder({ page })
  .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
  .analyze();
expect(results.violations.filter(v => v.impact === "critical")).toHaveLength(0);
```

---

## 9. Block E: Security

```
BLOCK E: Security (0.5 weeks — single sprint)

Sprint E.1: Security Hardening
├── E.1.1: CSP Header Configuration
├── E.1.2: CSRF Token Implementation
├── E.1.3: Rate Limiting Middleware
├── E.1.4: Security Headers (HSTS, XFO, etc.)
├── E.1.5: JWT Security Hardening
├── E.1.6: Input Sanitization
├── E.1.7: OWASP Top 10 Review
├── E.1.8: Secrets Vault Backend
└── E.1.9: Audit Log Backend Integration
```

---

#### Step E.1.1: CSP Header Configuration

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-security` |
| **Output** | `web/next.config.ts` CSP headers, `api/rest/middleware/security.go` |
| **Success** | CSP header on all responses, no inline scripts/unsafe-eval |

**Next.js CSP:**
```typescript
// next.config.ts
const cspHeader = `
  default-src 'self';
  script-src 'self' 'nonce-{NONCE}' 'strict-dynamic';
  style-src 'self' 'unsafe-inline';
  img-src 'self' blob: data:;
  font-src 'self';
  connect-src 'self' ws: wss: http://localhost:14120;
  frame-src 'none';
  object-src 'none';
  base-uri 'self';
  form-action 'self';
`.replace(/\s{2,}/g, " ").trim();

async function headers() {
  return [{ source: "/(.*)", headers: [{ key: "Content-Security-Policy", value: cspHeader }] }];
}
```

#### Step E.1.2: CSRF Token Implementation

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-security` |
| **Output** | `api/middleware/csrf.go`, frontend CSRF header injection in `api.ts` |
| **Success** | All mutating endpoints (POST/PUT/DELETE/PATCH) require `X-CSRF-Token` header |

**Double-submit cookie pattern:**
1. Server sets `csrf_token` cookie (HttpOnly=false, SameSite=Strict) on first GET
2. Client reads cookie and sends in `X-CSRF-Token` header on mutating requests
3. Server validates: `cookie.token === header.token`

```go
// api/middleware/csrf.go
func CSRFMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
            // Set CSRF cookie if not present
            if _, err := r.Cookie("csrf_token"); err != nil {
                token := generateCSRFToken()
                http.SetCookie(w, &http.Cookie{
                    Name: "csrf_token", Value: token,
                    Path: "/", SameSite: http.SameSiteStrictMode,
                    Secure: true, HttpOnly: false,
                })
            }
            next.ServeHTTP(w, r)
            return
        }
        cookieToken, _ := r.Cookie("csrf_token")
        headerToken := r.Header.Get("X-CSRF-Token")
        if cookieToken == nil || headerToken == "" || cookieToken.Value != headerToken {
            http.Error(w, "CSRF token mismatch", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

**Frontend injection in `api.ts`:**
```typescript
// Add to existing request() method
if (["POST", "PUT", "PATCH", "DELETE"].includes(options.method || "GET")) {
  const csrfToken = document.cookie.split("; ").find(row => row.startsWith("csrf_token="))?.split("=")[1];
  if (csrfToken) { headers["X-CSRF-Token"] = csrfToken; }
}
```

#### Step E.1.3: Rate Limiting Middleware

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-security` + `cosca-backend` |
| **Output** | `api/middleware/ratelimit.go` |
| **Success** | 100 req/min per IP; 429 with `Retry-After: 60` header when exceeded |

```go
// api/middleware/ratelimit.go
type RateLimiter struct {
    mu      sync.Mutex
    buckets map[string]*tokenBucket
    rate    int  // requests per minute
    burst   int
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ip := getClientIP(r)
        if !rl.allow(ip) {
            w.Header().Set("Retry-After", "60")
            w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.rate))
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

**Rate limits by endpoint group:**
| Group | Limit | Reason |
|-------|-------|--------|
| General API | 100 req/min/IP | Default |
| Auth (login/refresh) | 10 req/min/IP | Brute-force protection |
| Admin endpoints | 200 req/min/IP | Administrators get higher limits |
| WebSocket connect | 5 conn/min/IP | Prevent connection flooding |

#### Step E.1.4: Security Headers

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-security` |
| **Output** | Middleware adding headers to all responses |
| **Success** | All 7 security headers present, verified via `curl -I` and securityheaders.com |

**Headers to add:**

| Header | Value |
|--------|-------|
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains; preload` |
| `X-Frame-Options` | `DENY` |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=(), interest-cohort=()` |
| `Cross-Origin-Opener-Policy` | `same-origin` |
| `Cross-Origin-Resource-Policy` | `same-origin` |

**Go middleware:**
```go
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
        w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
        next.ServeHTTP(w, r)
    })
}
```

#### Step E.1.5: JWT Security Hardening

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-security` |
| **Output** | Enhanced JWT config in `api/auth/` and `internal/auth/` |
| **Success** | 15-min access tokens, rotation on refresh, revocation endpoint |

**Changes:**
- Access token TTL: 15 minutes (reduce from current)
- Refresh token TTL: 7 days
- Refresh token rotation: Issue new refresh token on each use, invalidate old
- Revocation: `POST /v1/auth/revoke` + blacklist table
- Claims: Add `iat`, `exp`, `jti` (unique ID), `role`, `username`
- Algorithm: Verify using HS256 with 256+ bit secret

#### Step E.1.6: Input Sanitization

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-security` |
| **Output** | `web/src/lib/sanitize.ts` (planned; DOMPurify), Go input validation |
| **Success** | No XSS vectors via user input |

**Frontend (DOMPurify):**
```typescript
import DOMPurify from "dompurify";

export function sanitizeHTML(dirty: string): string {
  return DOMPurify.sanitize(dirty, {
    ALLOWED_TAGS: ["b", "i", "em", "strong", "a", "code", "pre", "br", "p", "ul", "ol", "li"],
    ALLOWED_ATTR: ["href", "title", "target"],
  });
}
```

**Backend (Go):** All user inputs validated with struct tags. All SQL queries use parameterized statements (no string concatenation). Path parameters validated for allowed characters.

#### Step E.1.7: OWASP Top 10 Review

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-security` |
| **Output** | `docs/roadmap/owasp-top10-review.md` |
| **Success** | All 10 categories reviewed, remediations documented |

**Checklist:**

| # | Category | Status | Action |
|---|----------|--------|--------|
| 1 | Broken Access Control | ✅ Existing | RBAC middleware on admin endpoints, frontend route guards |
| 2 | Cryptographic Failures | ✅ (Step E.1.5, E.1.8) | JWT strong algorithm, AES-256-GCM for secrets |
| 3 | Injection | ✅ (Step E.1.6) | Parameterized queries, input sanitization, path validation |
| 4 | Insecure Design | 🟡 Review | ADR security review, limit error detail exposure |
| 5 | Security Misconfiguration | ✅ (Steps E.1.1, E.1.4) | CSP, HSTS, XFO, disable debug in production |
| 6 | Vulnerable Components | 🔴 Audit | `pnpm audit`, `go list -m -u all`, update deps |
| 7 | Authentication Failures | ✅ (Steps E.1.3, E.1.5) | Rate limiting on login, JWT hardening, password complexity |
| 8 | Software Integrity | 🟡 Verify | CI/CD pipeline integrity, signed commits, SBOM |
| 9 | Logging & Monitoring | ✅ (Step E.1.9) | Audit logging, structured logs with request IDs |
| 10 | SSRF | 🟡 Review | Validate/sanitize user-supplied URLs |

#### Step E.1.8: Secrets Vault Backend

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-backend` + `cosca-security` |
| **Output** | `internal/secrets/vault.go`, `api/rest/handler/secrets.go` |
| **Success** | Secrets encrypted at rest (AES-256-GCM), admin-only API access |

```go
// internal/secrets/vault.go
type Vault struct {
    db     *sql.DB
    cipher cipher.AEAD  // AES-256-GCM
}

func NewVault(db *sql.DB, masterSecret []byte) (*Vault, error)
func (v *Vault) Store(key, value, secretType, description string) error
func (v *Vault) Retrieve(key string) (*Secret, error)
func (v *Vault) Delete(key string) error
func (v *Vault) List() ([]SecretMeta, error)  // values masked

type Secret struct {
    Key         string
    Value       string    // decrypted
    Type        string    // api_key, password, token, certificate, other
    Description string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

#### Step E.1.9: Audit Log Backend Integration

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-backend` + `cosca-security` |
| **Output** | `internal/audit/audit.go`, `api/rest/handler/audit.go` |
| **Success** | Every authenticated action logged to audit table |

```go
// internal/audit/audit.go
type AuditEntry struct {
    UserID     string
    Action     string   // create, read, update, delete, execute, login, logout
    Resource   string   // agent, skill, workflow, provider, user, secret, api_key
    ResourceID string
    Details    string   // JSON
    IPAddress  string
    UserAgent  string
    Status     string   // success, denied, error
}

type AuditLogger struct { db *sql.DB }
func (al *AuditLogger) Log(ctx context.Context, entry AuditEntry) error
```

**Actions to audit:** All login/logout, all CRUD on users/keys/secrets/agents/skills/workflows, all orchestration executions, all config changes, all role/permission changes, all denied access attempts.

**Audit middleware:** Wrap all routes to auto-log with user ID (from JWT), derived action/resource (from method + path), IP, user agent, and status code.

---

## 10. Block F: Responsiveness & PWA

```
BLOCK F: Responsiveness & PWA (0.5 weeks — single sprint)

Sprint F.1: Responsive Design + PWA
├── F.1.1: Mobile Layout Audit + Fixes (320px)
├── F.1.2: Tablet Layout Audit (768px)
├── F.1.3: Desktop + Ultrawide Audit (1024px, 2560px)
├── F.1.4: Touch Target Size Enforcement (>44px)
├── F.1.5: Bottom Navigation (Mobile) vs Sidebar (Desktop)
├── F.1.6: PWA Manifest
├── F.1.7: Service Worker + Offline Support
└── F.1.8: Installability + Standalone Mode
```

---

#### Step F.1.1: Mobile Layout Audit + Fixes (320px)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` + `cosca-uiux` |
| **Output** | Audit report + responsive fixes for all 32 pages |
| **Success** | All pages functional at 320px: no horizontal scroll, readable text, tappable buttons |

**Per-page audit checklist:**
- [ ] No horizontal scroll overflow
- [ ] All text readable (no overlapping)
- [ ] All buttons/links visible and tappable
- [ ] Forms: input widths fit viewport, labels above inputs
- [ ] Data tables: Card layout fallback or horizontal scroll with sticky column
- [ ] Charts: Scale down, simplified (fewer points, smaller legends)
- [ ] Modals/Sheets: Full-screen
- [ ] Sidebar: Hidden, hamburger menu

**Pages needing special attention:**
- DataGrid pages (Audit, Executions, Users, Secrets) → Card layout fallback
- Charts pages (Metrics, Analytics) → Single column, stacked
- Split View pages → Stack vertically
- Pipeline Editor → Simplified, touch-friendly drag
- AI Playground → Single provider at a time, swipe

#### Step F.1.2: Tablet Layout Audit (768px)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Success** | All pages usable at 768px: 2-column grids, collapsed sidebar (icons only) |

**Key changes:** Sidebar collapses to 68px, expandable on hover/tap. Grids go 2-column. Data tables full-width. Charts 2 per row.

#### Step F.1.3: Desktop + Ultrawide Audit (1024px, 2560px)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Success** | Content doesn't stretch to 2560px; max-width container applied |

**Implementation:** `max-w-screen-2xl mx-auto` on main content. Ultra-wide centers content with balanced whitespace. Full-width exceptions: DataGrid pages, Dashboard widgets.

#### Step F.1.4: Touch Target Size Enforcement

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-uiux` |
| **Success** | All interactive elements ≥44px × 44px touch target on mobile |

**Implementation:** Audit with devtools mobile view + "Show touch targets". Fix violations: `min-h-[44px] min-w-[44px]` on small buttons, ≥8px gap between tappable elements, enlarged checkbox/radio hit areas.

#### Step F.1.5: Bottom Navigation (Mobile) vs Sidebar (Desktop)

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Output** | `web/src/components/layout/bottom-nav.tsx` (implemented as `web/src/components/layout/mobile-nav.tsx`) |
| **Success** | Mobile: bottom nav with 5 primary items; Desktop: sidebar |

**Bottom nav items (by usage frequency):**
1. Dashboard (LayoutDashboard icon)
2. Knowledge (BookOpen icon)
3. Orchestration (Orbit icon)
4. Memory (Brain icon)
5. Settings (Settings icon)
+ "More" (Ellipsis icon) → full nav menu

```tsx
export function BottomNav() {
  return (
    <nav className="fixed bottom-0 left-0 right-0 z-50 border-t bg-background pb-safe md:hidden">
      <ul className="flex justify-around py-2">
        {/* 5 primary items + More */}
      </ul>
    </nav>
  );
}
```

Note: Add `pb-16` to main content on mobile to prevent overlap.

#### Step F.1.6: PWA Manifest

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` + `cosca-devops` |
| **Output** | `web/public/manifest.json`, `web/public/icons/` (8 sizes) |
| **Success** | Lighthouse PWA audit passes "Installable" check |

```json
{
  "name": "Cosca Enterprise Platform",
  "short_name": "Cosca",
  "description": "AI Orchestration System — Enterprise Console",
  "start_url": "/dashboard",
  "display": "standalone",
  "background_color": "#09090b",
  "theme_color": "#2563eb",
  "icons": [
    { "src": "/icons/icon-192.png", "sizes": "192x192", "type": "image/png" },
    { "src": "/icons/icon-512.png", "sizes": "512x512", "type": "image/png" },
    { "src": "/icons/icon-512-maskable.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable" }
  ]
}
```

**Icon sizes:** 72, 96, 128, 144, 152, 192, 384, 512 (all with maskable versions for Android). Generate from Cosca brandmark.

#### Step F.1.7: Service Worker + Offline Support

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` + `cosca-devops` |
| **Output** | `next.config.ts` with `next-pwa` configuration |
| **Success** | Cached pages load offline; service worker registered |

```typescript
// next.config.ts — using next-pwa
const withPWA = require("next-pwa")({
  dest: "public",
  register: true,
  skipWaiting: true,
  disable: process.env.NODE_ENV === "development",
  runtimeCaching: [
    {
      urlPattern: /^https?.*/,
      handler: "NetworkFirst",
      options: {
        cacheName: "offline-cache",
        expiration: { maxEntries: 200, maxAgeSeconds: 60 * 60 * 24 * 7 },
      },
    },
    {
      urlPattern: /\/api\//,
      handler: "NetworkOnly",
    },
  ],
});

module.exports = withPWA(nextConfig);
```

**Offline behavior per page:**

| Page | Offline Behavior |
|------|------------------|
| Dashboard | Cached (last known state) |
| Knowledge | Cached search bar, "Offline — search unavailable" |
| Memory | Cached tabs, "Offline — records unavailable" |
| Settings | Cached config, "Offline — config may be stale" |
| Docs | Fully cached (static content) |

#### Step F.1.8: Installability + Standalone Mode

| Field | Detail |
|-------|--------|
| **Agent** | `cosca-frontend` |
| **Success** | "Add to Home Screen" prompt appears; standalone mode no browser chrome |

**Implementation:**
1. Listen for `beforeinstallprompt` event
2. Show custom prompt: "Install Cosca Console" with "Install" and "Not now" buttons
3. Store dismissal in `localStorage` (7-day cooldown)
4. Standalone CSS: safe-area-inset for notched devices (`pb-safe`)

```css
@media (display-mode: standalone) {
  .browser-chrome-banner { display: none; }
}
```

---

## 11. Consolidated Success Criteria

### Block A — Remaining Pages
- [ ] All 15 new routes implemented and navigable
- [ ] Orchestration Console: SSE streaming, result with agent/skills/duration
- [ ] AI Playground: 3+ providers parallel, side-by-side comparison
- [ ] Pipeline Editor: Drag-and-drop stage ordering, save/load definitions
- [ ] Execution History: DataGrid with 20 executions, detail with pipeline trace
- [ ] System Metrics: 8 chart widgets, auto-refresh 30s
- [ ] Audit Log Viewer: Filterable DataGrid, CSV export, admin-only
- [ ] Secrets Vault: CRUD with encryption, masked values, admin-only
- [ ] All 15 new pages: `pnpm typecheck` passes, `pnpm build` succeeds

### Block B — Design System
- [ ] DataGrid handles 10K rows <100ms (virtualized)
- [ ] DataGrid: Sort (click), Filter (dropdown), Pin (right-click), Select (checkboxes), Export (CSV/JSON)
- [ ] Charts render with real data; 6 chart types implemented
- [ ] Notifications appear within 500ms of WebSocket event
- [ ] Unread count badge on bell icon
- [ ] Split View: Draggable divider, keyboard-operable, persistence
- [ ] Timeline: 100+ items without performance issues
- [ ] Onboarding tour: 6 steps, skippable, replayable
- [ ] Keyboard shortcuts: `?` opens panel, searchable, all shortcuts documented
- [ ] All shared components >25 Storybook stories (dark + light)
- [ ] All shared components pass a11y (axe-core in Storybook)

### Block C — Performance
- [ ] Lighthouse Performance >90 on all pages
- [ ] FCP <1.5s, LCP <2.5s, TBT <200ms, CLS <0.1
- [ ] Bundle <200KB initial JS per route (gzipped)
- [ ] No duplicate dependencies
- [ ] Images: `next/image` with proper sizing
- [ ] Fonts: preloaded, no FOIT
- [ ] PPR enabled on dashboard + static shells
- [ ] ISR for settings + docs
- [ ] Suspense boundaries for streaming SSR

### Block D — Tests & Quality
- [ ] Vitest coverage >80% (statements, branches, functions, lines)
- [ ] >95 unit tests for shared components
- [ ] >39 integration tests for feature hooks
- [ ] >30 page integration tests (RTL)
- [ ] Playwright E2E: 10 critical paths passing in 3 browsers
- [ ] Storybook: >25 component stories
- [ ] 0 critical a11y violations in Storybook
- [ ] WCAG AA+: 0 critical/serious on all 32 pages
- [ ] axe-core CI: fails build if violations increase
- [ ] `pnpm typecheck` 0 errors, `pnpm build` success, `pnpm lint` 0 errors

### Block E — Security
- [ ] CSP headers configured (`strict-dynamic`, no `unsafe-inline` for scripts)
- [ ] CSRF protection on all mutating endpoints
- [ ] Rate limiting: 100 req/min/IP (general), 10 req/min (auth), 429 tested
- [ ] Security headers: HSTS, XFO, X-Content-Type-Options, Referrer-Policy, Permissions-Policy, COOP, CORP
- [ ] JWT: 15-min access tokens, rotation on refresh, revocation
- [ ] Input sanitization: DOMPurify frontend, parameterized queries backend
- [ ] OWASP Top 10: all 10 reviewed and documented
- [ ] Secrets vault: AES-256-GCM verified, admin-only
- [ ] Audit logging: every authenticated action logged, viewer functional
- [ ] Dependencies: `pnpm audit` 0 critical/high; `go` no known CVEs

### Block F — Responsiveness
- [ ] All 32 pages functional at 320px (no horizontal scroll)
- [ ] All 32 pages usable at 768px (2-col grids, collapsed sidebar)
- [ ] All 32 pages good at 1024px+ and 2560px+ (max-width container)
- [ ] Touch targets >44px on mobile
- [ ] Bottom nav on mobile, sidebar on desktop
- [ ] PWA manifest: valid, icons present
- [ ] Service worker registered, offline for cached pages
- [ ] Lighthouse PWA "Installable" passes
- [ ] Standalone mode: no browser chrome, safe-area-inset
- [ ] "Add to Home Screen" prompt (7-day cooldown)

---

## 12. Risk Register

| # | Risk | Probability | Impact | Mitigation |
|---|------|------------|--------|------------|
| R-01 | Backend endpoints not ready before frontend work | **Medium** | **High** — Blocks Block A | Verify all P0 prereqs in Gate Zero. Build with MSW mocks first; switch to real API when ready |
| R-02 | Scope creep — 48 items without prioritization | **High** | **High** — Timeline doubles | Strict authorization gates (§4.2). User must approve each block explicitly |
| R-03 | Performance regression from new components | **Medium** | **Medium** — Lighthouse <85 | Benchmark before/after each block. Block C addresses performance specifically |
| R-04 | A11y regressions from new UI | **Low** | **High** — Audit fails | axe-core CI on every PR. Storybook a11y addon in development |
| R-05 | E2E test flakiness | **Medium** | **Medium** — CI unreliable | Playwright retries (2 in CI). Isolated test data. Proper cleanup |
| R-06 | WebSocket complexity | **Medium** | **Medium** — Real-time broken | Build polling fallback first. WebSocket as enhancement. Graceful disconnect handling |
| R-07 | PWA service worker caching too aggressive | **Low** | **Medium** — Stale data | NetworkFirst strategy. Version-based cache busting. API routes NetworkOnly |
| R-08 | Rate limiting blocks legitimate users | **Low** | **Medium** — UX degraded | 100 req/min is generous for a console. 429 response is clear. Admin higher limits |
| R-09 | Storybook build fails in CI | **Low** | **Low** — Docs only | Non-blocking. Fix before next sprint |
| R-10 | Coverage threshold blocks legitimate PRs | **Low** | **Medium** — Slows dev | 80% is achievable. Exclude generated/story/config files |

---

## 13. Dependency Graph

### 13.1 Inter-Block Dependencies

```
┌─────────────────────────────────────────────────────────────────────┐
│                         GATE ZERO                                    │
│           Backend P0 endpoints + infrastructure verified            │
└─────────────────────────────┬───────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  BLOCK A (Pages)                                                    │
│  └──► Depends on: P0 backend endpoints (run, executions)            │
│  └──► Blocks B and D should NOT be blocked by A; can be parallel    │
└─────────────────────────────┬───────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    ▼                   ▼
┌───────────────────────────┐  ┌───────────────────────────┐
│  BLOCK B (Design System)  │  │  BLOCK D (Tests)          │
│  └── Reusable components  │  │  └── Unit + integration    │
│      used by Block A      │  │      tests for A + B      │
└───────────┬───────────────┘  └─────────────┬─────────────┘
            │                                │
            └───────────┬────────────────────┘
                        ▼
┌─────────────────────────────────────────────────────────────────────┐
│  BLOCK C (Performance)                                              │
│  └── Depends on: A (pages exist to measure), B (components built)   │
│  └── Can start in parallel with D late sprints                      │
└─────────────────────────────┬───────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  BLOCK E (Security)                                                 │
│  └── Depends on: Backend P2 endpoints (audit, secrets)              │
│  └── Frontend security headers depend on C (performance config)     │
└─────────────────────────────┬───────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  BLOCK F (Responsiveness)                                           │
│  └── Depends on: A (pages exist to audit), B (components respond)   │
│  └── PWA depends on C (bundle optimization makes caching viable)    │
│  └── Last block — ensures everything looks good on all devices      │
└─────────────────────────────────────────────────────────────────────┘
```

### 13.2 Recommended Execution Order

```
Week 1:  A.1 (6 core pages)  +  B.1 (DataGrid + Charts)
Week 2:  A.2 (9 enterprise)  +  B.2 (Notif, Split, Timeline) + B.3 (UX enchancers)
Week 3:  D.1 (Vitest, RTL, MSW — unit + integration tests)
Week 4:  D.2 (Playwright, Storybook, A11y)  +  Start C (Performance)
Week 5:  C (Performance complete)  +  E (Security)
Week 6:  F (Responsiveness)  +  Final verification + buffer
```

### 13.3 Parallelization Opportunities

| Can Run in Parallel | Why |
|---------------------|-----|
| **A.1 + B.1** | Different agents (Frontend Chief for A, Frontend Chief for B — can context-switch). DataGrid needed by A.1.3 (Execution History) anyway |
| **D.1 + B.2** | Testing Chief writes tests while Frontend Chief builds UI components |
| **D.2 + B.3** | Testing Chief writes Playwright E2E while Frontend Chief builds Tour/Shortcuts/Breadcrumb |
| **E + F (early start)** | Security Chief and Frontend Chief work on different codebases (Go backend vs Next.js frontend) |

### 13.4 Must Be Sequential

| Block Order | Why |
|-------------|-----|
| **B.1 before B.2** | DataGrid and Charts are foundational — Notification panel and Split View may use them |
| **A before C** | Cannot measure performance of pages that don't exist |
| **B before F** | Responsiveness audit requires the components to exist |
| **D.2 after D.1** | Playwright E2E tests need the unit-tested components and MSW-verified hooks |
| **E after P2 endpoints** | Audit logging and Secrets vault backend must exist before frontend viewers |
| **F after all others** | Comprehensive responsive audit of the complete application |

---

> **Related documents:**
> - [ADR-007: Frontend Architecture](../adr/ADR-007-frontend-architecture.md)
> - [ADR-006: AI Orchestration Implementation](../adr/ADR-006-ai-orchestration-implementation.md)
> - [ADR-005: AI Orchestration Engine](../adr/ADR-005-ai-orchestration.md)
> - [Phase 1 MVP Backlog](phase-1-mvp-backlog.md)
> - [Audit Report](audit-report.md)
> - [Hotfix Workflow](hotfix-workflow.md)
