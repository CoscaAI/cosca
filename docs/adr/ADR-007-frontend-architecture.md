# ADR-007: Frontend Architecture for Cosca Enterprise Platform Web Console

> **Status:** Accepted ✅ | **Owner:** Architecture Chief | **Last Updated:** 2026-07-27
> **Implementation:** Completed — Phases 0-4 delivered, 21+ pages, 27 feature modules, 36 REST endpoints

## Context

The Cosca is a Go-based enterprise platform providing Knowledge Engine, Memory Engine, AI Orchestration, Plugin System, and Editor Adapters through a REST API. While the CLI is powerful for power users and automation, the platform needs a web console to:

- **Visualize** knowledge graphs, memory layers, and runtime metrics
- **Manage** plugins, providers, and configurations without editing YAML files
- **Monitor** system health, orchestration pipelines, and agent activity
- **Explore** indexed knowledge bases and memory records with rich search
- **Configure** settings, API keys, and provider priorities through a GUI

The web console must serve both Phase 1 MVP (5 core pages: Dashboard, Knowledge Explorer, Memory Viewer, Runtime Monitor, Settings) and scale to Phase 4 enterprise (40+ pages including workflow editor, agent configurator, pipeline debugger, team management, audit logs).

### Current State

As of v1.4.0-dev (2026-07-27), all architecture decisions have been implemented:

- Go backend with 36 REST API endpoints wired to 10 dedicated handlers + JWT auth + RBAC + OpenAPI 3.0 spec
- TypeScript SDK (`sdk/typescript/`, package `@cosca/sdk`) providing typed client with retry logic and error handling for all subsystems
- `web/` directory with 21+ fully-featured pages, 27 feature modules, design system, PWA support
- Multi-container Docker (Go scratch + Node Alpine), docker-compose, Helm charts, Terraform
- OpenAPI 3.0 specification (`api/rest/openapi.yaml`) — 50 operations, 62 schemas
- Frontend fully built: Next.js 15, React 19, TanStack Query, Radix UI, Tailwind, dark-first theme, JWT auth, RBAC, command palette
- Testing: 500+ Go tests, 354 Vitest tests, 10 Playwright E2E paths, 29 Storybook stories
- Security: CSP, CSRF, Rate Limiting, HSTS, JWT hardening, httpOnly cookies, OWASP Top 10 reviewed
- WCAG AA+ accessibility audit completed — 0 critical violations

### Requirements

1. Co-versioned releases with the Go backend (same Helm chart, same version tag)
2. Type-safe data fetching from the REST API using generated types
3. Server-side rendering for initial page loads with streaming for real-time data
4. Dark mode as default with light mode as a user preference
5. Component library that supports 40+ pages without CSS conflicts or bundle bloat
6. API proxy layer in Next.js to avoid CORS issues and enable server-side auth
7. Independent container deployment (Go `scratch` image + Next.js `node:alpine` image)

---

## Decisions

### D-001: Monorepo Structure — `web/` Within `cosca` Repository

**Chosen:** The frontend lives at `web/` inside the existing `cosca` monorepo.

**Why:**

| Concern | Monorepo | Separate Repo |
|---------|----------|---------------|
| Version sync | Single `git tag` releases both | Requires release orchestration |
| OpenAPI spec | Single source of truth, shared reference | Must be mirrored or published |
| CI/CD | One pipeline, sequenced Go → SDK → Web | Two pipelines with dependency ordering |
| SDK consumption | Workspace reference (`transpilePackages`) | Published npm package required |
| Breaking changes | Detected at build time | Detected at integration test time |
| Team workflow | Single PR for cross-cutting changes | Cross-repo PRs with coordination |

The `web/` directory is treated as an independent workspace with its own `package.json`, `tsconfig.json`, and `next.config.ts`. The monorepo boundary is enforced by `.gitignore` rules that keep Go artifacts out of `web/` and `node_modules/` out of the Go module path.

```
cosca/
├── cmd/cosca/             # Go CLI entry point
├── internal/            # Go subsystems (knowledge, memory, orchestration, ...)
├── pkg/                 # Public Go packages
├── api/                 # API specs (OpenAPI, gRPC proto, MCP schema)
│   └── rest/
│       └── openapi.yaml # Single source of truth for frontend SDK types
├── sdk/typescript/      # @cosca/sdk — TypeScript client (axios-based)
├── web/                 # Next.js 15 frontend (this ADR)
│   ├── package.json
│   ├── next.config.ts
│   └── src/
│       ├── app/         # App Router pages and layouts
│       ├── features/    # Feature-based domain modules
│       ├── shared/      # Reusable UI components (shadcn/ui + custom)
│       ├── lib/         # Utilities, hooks, type helpers
│       └── styles/      # Global CSS, Tailwind config, design tokens
├── deploy/              # Helm charts, Terraform, docker-compose
├── Dockerfile           # Go backend
├── Dockerfile.web       # Next.js frontend (added by this ADR)
└── Makefile
```

**Alternative rejected:** Separate repository. Version drift between Go API and TypeScript types would require manual synchronization of OpenAPI specs, leading to type mismatches and integration failures.

---

### D-002: Next.js 15 App Router — Server Components by Default

**Chosen:** Next.js 15 with App Router, React Server Components (RSC) as the default rendering strategy, with Client Components (`"use client"`) only where interactivity is required.

**Why:**

| Feature | App Router (Next.js 15) | Pages Router | Vite + React SPA |
|---------|------------------------|-------------|-------------------|
| Server Components | Yes — zero JS shipped for static content | No | No |
| Streaming SSR | `Suspense` boundaries with streaming | No | No |
| Partial Prerendering | Static shell + dynamic holes | No | No |
| Layout nesting | Nested layouts with preserved state | Manual | N/A |
| File-based routing | `app/dashboard/page.tsx` → `/dashboard` | `pages/dashboard.tsx` | Manual |
| Route groups | `(dashboard)/layout.tsx` for shared layouts | No | No |
| API routes | Co-located `route.ts` for proxy endpoints | `pages/api/` | Separate config |
| Bundle splitting | Automatic per-route code splitting | Per-page | Manual |

**Rendering strategy per page type:**

| Page | Strategy | Rationale |
|------|----------|-----------|
| Dashboard | Static shell + client islands | Metrics widgets need interactivity; layout is static |
| Knowledge Explorer | Server-rendered search + client filters | Initial results from server; faceted filters on client |
| Memory Viewer | Client-heavy | Infinite scroll, real-time updates, drag-and-drop layers |
| Runtime Monitor | Client-heavy | Live metrics via polling, streaming log tail |
| Settings | Static form + client validation | Form rendered server-side; validation on client |

**Layout architecture:**

```
app/
├── layout.tsx               # Root layout: providers, theme, fonts
├── (dashboard)/             # Route group: shared sidebar + header
│   ├── layout.tsx           # Dashboard shell (Server Component)
│   ├── dashboard/
│   │   └── page.tsx         # Dashboard page (mixed RSC + client islands)
│   ├── knowledge/
│   │   ├── page.tsx         # Knowledge explorer
│   │   └── [id]/
│   │       └── page.tsx     # Knowledge detail
│   ├── memory/
│   │   ├── page.tsx         # Memory viewer
│   │   └── [layer]/
│   │       └── page.tsx     # Memory layer detail
│   ├── runtime/
│   │   └── page.tsx         # Runtime monitor
│   └── settings/
│       └── page.tsx         # Settings page
├── api/                     # Next.js API route handlers (proxy to Go backend)
│   └── [[...route]]/
│       └── route.ts         # Catch-all proxy: /api/* → http://localhost:14120/*
└── auth/
    └── login/
        └── page.tsx         # Login page (standalone layout, no sidebar)
```

**Alternative rejected:** Pages Router. The Pages Router lacks Server Components, streaming SSR via `Suspense`, and nested layout persistence. For a dashboard application with shared navigation and per-page data fetching, the App Router's layout isolation and streaming capabilities are essential.

---

### D-003: Feature-Based Directory Architecture

**Chosen:** Organize source code by domain feature rather than by technical layer. Each feature module is self-contained with its own components, hooks, types, and API integration.

**Why:**

ADR-001 established layered architecture for the Go backend (CLI → Runtime → Subsystems → Infrastructure). The frontend mirrors this philosophy but applies it horizontally through feature modules rather than vertically through technical layers.

**Feature module contract:**

```
features/
├── dashboard/
│   ├── components/          # Dashboard-specific components
│   │   ├── SystemHealth.tsx
│   │   ├── RecentActivity.tsx
│   │   ├── QuickStats.tsx
│   │   └── KnowledgeGraph.tsx
│   ├── hooks/               # Dashboard-specific data hooks
│   │   ├── useSystemHealth.ts
│   │   ├── useRecentActivity.ts
│   │   └── useQuickStats.ts
│   ├── api/                 # API integration (TanStack Query wrappers)
│   │   └── queries.ts
│   ├── types/               # Feature-specific TypeScript types
│   │   └── index.ts
│   └── index.ts             # Public API barrel export
│
├── knowledge/
│   ├── components/
│   │   ├── SearchBar.tsx
│   │   ├── SearchResults.tsx
│   │   ├── KnowledgeDetail.tsx
│   │   ├── IndexStatus.tsx
│   │   └── SyncControls.tsx
│   ├── hooks/
│   │   ├── useKnowledgeSearch.ts
│   │   ├── useKnowledgeIndex.ts
│   │   └── useKnowledgeStats.ts
│   ├── api/
│   │   ├── queries.ts       # Read operations (useQuery)
│   │   └── mutations.ts     # Write operations (useMutation)
│   ├── types/
│   │   └── index.ts
│   └── index.ts
│
├── memory/
│   ├── components/
│   │   ├── LayerSelector.tsx
│   │   ├── MemoryTimeline.tsx
│   │   ├── MemoryRecord.tsx
│   │   └── MemorySearch.tsx
│   ├── hooks/
│   │   ├── useMemorySearch.ts
│   │   └── useMemoryLayers.ts
│   ├── api/
│   │   └── queries.ts
│   ├── types/
│   │   └── index.ts
│   └── index.ts
│
├── runtime/
│   ├── components/
│   │   ├── StatusIndicator.tsx
│   │   ├── LogStream.tsx
│   │   ├── MetricsPanel.tsx
│   │   └── OrchestrationTrace.tsx
│   ├── hooks/
│   │   ├── useRuntimeStatus.ts
│   │   ├── useLogStream.ts
│   │   └── useMetrics.ts
│   ├── api/
│   │   └── queries.ts
│   ├── types/
│   │   └── index.ts
│   └── index.ts
│
└── settings/
    ├── components/
    │   ├── ProviderConfig.tsx
    │   ├── PluginManager.tsx
    │   ├── ApiKeyForm.tsx
    │   └── PreferencePanel.tsx
    ├── hooks/
    │   ├── useProviderConfig.ts
    │   └── usePluginManager.ts
    ├── api/
    │   ├── queries.ts
    │   └── mutations.ts
    ├── types/
    │   └── index.ts
    └── index.ts
```

**Cross-feature dependencies:** Features import from `@cosca/sdk` for API types and from `@/shared/` for UI components but never import directly from other features. Shared state between features flows through URL parameters, TanStack Query cache, or React Context defined in the layout layer.

**Why this scales to 40+ pages:**

| Concern | Feature-based | Layer-based (`components/`, `hooks/`, etc.) |
|---------|--------------|---------------------------------------------|
| Module boundaries | Explicit — each feature is a directory | Implicit — naming conventions only |
| Team ownership | One team owns `features/runtime/` | Multiple teams touch `components/` |
| Code splitting | Natural boundary for `lazy()` imports | Requires manual bundle analysis |
| Deletion safety | Delete `features/deprecated/` directory | Files scattered across 5 directories |
| Onboarding | "Work on features/memory/" is clear | "Find components/memory/ and hooks/memory/..." |

**Alternative rejected:** Layer-based organization (`components/`, `hooks/`, `services/`, `types/`). This works for small apps but creates "folder of a thousand files" at 40+ pages. The Go backend uses package-based organization (`internal/knowledge/`, `internal/memory/`) — the frontend mirrors this convention.

---

### D-004: shadcn/ui + Radix Primitives — Component Architecture

**Chosen:** shadcn/ui as the component foundation with Radix UI primitives as the underlying accessibility layer, styled with Tailwind CSS.

**Why:**

The `package.json` already includes the exact Radix packages needed for the Phase 1 component set: `@radix-ui/react-avatar`, `@radix-ui/react-collapsible`, `@radix-ui/react-dropdown-menu`, `@radix-ui/react-scroll-area`, `@radix-ui/react-separator`, `@radix-ui/react-sheet`, `@radix-ui/react-slot`, `@radix-ui/react-tooltip`.

| Library | Ownership Model | Bundle Impact | Customization | Accessibility |
|---------|----------------|---------------|---------------|---------------|
| **shadcn/ui + Radix** | Copy-paste source into project | Only what you use | Full — edit source directly | WAI-ARIA compliant (Radix) |
| Mantine | npm dependency | Tree-shaken but heavy base | Theme object, limited | Partial |
| Chakra UI v3 | npm dependency | Large runtime | Theme tokens, style props | Good |
| Ant Design | npm dependency | Very large | CSS-in-JS overrides | Partial |
| Material UI | npm dependency | Large | Theme provider, `sx` prop | Good |
| Headless UI | Copy-paste or npm | Small (logic only) | Unstyled — full control | Good but limited primitives |

**Decision rationale:**

1. **No vendor lock-in.** shadcn/ui is not a library — it integrates by copying source files into `src/shared/components/ui/`. You own the code. If shadcn/ui is abandoned, the components in your repo continue to work.

2. **Bundle efficiency.** Tree-shaking is guaranteed because you only import the components you copied. No runtime for unused components.

3. **Radix accessibility.** Every Radix primitive (`DropdownMenu`, `Dialog`, `Tooltip`, `Sheet`) adheres to WAI-ARIA authoring practices with keyboard navigation, focus trapping, and screen reader announcements out of the box.

4. **Tailwind theming.** Dark mode, spacing, colors, and typography are defined as Tailwind CSS variables (see D-007). Components inherit these automatically.

5. **Composable patterns.** Components use `class-variance-authority` (cva) for variant management and `clsx` + `tailwind-merge` for conditional class merging — all already in `package.json`.

**Component inventory for Phase 1:**

| Component | Radix Primitive | shadcn/ui | Purpose |
|-----------|----------------|-----------|---------|
| Button | `@radix-ui/react-slot` | Yes | Primary actions, toolbars |
| Input | Native + styling | Yes | Search, forms |
| Card | CSS only | Yes | Content containers |
| DropdownMenu | `@radix-ui/react-dropdown-menu` | Yes | Context menus, settings |
| Sheet | `@radix-ui/react-sheet` | Yes | Side panels, detail views |
| Tooltip | `@radix-ui/react-tooltip` | Yes | Icon labels, help text |
| ScrollArea | `@radix-ui/react-scroll-area` | Yes | Consistent scrollbars |
| Avatar | `@radix-ui/react-avatar` | Yes | User/profile images |
| Separator | `@radix-ui/react-separator` | Yes | Visual dividers |
| Collapsible | `@radix-ui/react-collapsible` | Yes | Expandable sections |
| Toast | n/a (Sonner) | n/a | Notifications |
| Icons | n/a (Lucide React) | n/a | Icon system |

**Alternative rejected:** Mantine v7. While Mantine provides a richer component set (DatePicker, RichTextEditor, Charts), its theming system is less flexible than Tailwind CSS and customizing deeply nested internal styles requires `styles` prop overrides which break on library updates. The bundle includes all component hooks even when tree-shaking CSS.

---

### D-005: TanStack Query for Server State Management

**Chosen:** TanStack Query v5 (`@tanstack/react-query`) for all server state (API data fetching, caching, background refetch). No global state library (Redux, Zustand, Jotai) for server data.

**Why:**

The Cosca web console is a server-state-heavy application. Almost all UI state is derived from the REST API response cache. TanStack Query provides:

| Capability | TanStack Query | Redux Toolkit Query | SWR | Fetch + useState |
|------------|---------------|---------------------|-----|-----------------|
| Caching | ✓ with stale-while-revalidate | ✓ | ✓ | Manual |
| Background refetch | ✓ on focus, reconnect, interval | ✓ | ✓ | Manual |
| Optimistic updates | ✓ via `onMutate` | ✓ | ✓ | Manual |
| Pagination | ✓ `useInfiniteQuery` | ✓ | ✓ | Manual |
| DevTools | ✓ dedicated panel | ✓ Redux DevTools | ✗ | ✗ |
| Mutations | ✓ `useMutation` | ✓ | ✗ built-in | Manual |
| SSR support | ✓ `HydrationBoundary` + `prefetchQuery` | ✓ | ✓ | ✗ |
| Bundle size | ~12 KB gzipped | ~32 KB (requires Redux) | ~5 KB | 0 KB |

**Query key convention:**

```typescript
// features/knowledge/hooks/use-knowledge-search.ts (current implementation)
import { useQuery } from "@tanstack/react-query";
import { aosClient } from "@/lib/coscaent";

export const knowledgeKeys = {
  all: ["knowledge"] as const,
  search: (query: string) => [...knowledgeKeys.all, "search", query] as const,
  detail: (id: string) => [...knowledgeKeys.all, "detail", id] as const,
  stats: () => [...knowledgeKeys.all, "stats"] as const,
};

export function useKnowledgeSearch(query: string) {
  return useQuery({
    queryKey: knowledgeKeys.search(query),
    queryFn: () => aosClient.knowledge.search(query),
    enabled: query.length > 0,
    staleTime: 30_000, // 30 seconds
  });
}
```

**Client-only state:** UI-specific state (sidebar open/closed, active tab, selected rows, form state) stays in React Context or local `useState`. TanStack Query handles the boundary between server data and UI state.

**SSR hydration pattern:**

```typescript
// app/dashboard/page.tsx (Server Component)
import { dehydrate, HydrationBoundary, QueryClient } from "@tanstack/react-query";
import { DashboardClient } from "@/features/dashboard";

export default async function DashboardPage() {
  const queryClient = new QueryClient();
  
  // Prefetch on the server
  await queryClient.prefetchQuery({
    queryKey: knowledgeKeys.stats(),
    queryFn: () => aosClient.knowledge.stats(),
  });

  return (
    <HydrationBoundary state={dehydrate(queryClient)}>
      <DashboardClient />
    </HydrationBoundary>
  );
}
```

**Alternative rejected:** Redux Toolkit. Adding Redux for server state duplicates what TanStack Query does better with less boilerplate. Redux would require manual cache invalidation, optimistic update logic, and request deduplication — all built into TanStack Query. For the tiny amount of true client state (sidebar, theme), React Context or Zustand at ~1 KB is sufficient.

---

### D-006: OpenAPI-Driven Type Generation — Single Source of Truth

**Chosen:** TypeScript types for the frontend are generated from `api/rest/openapi.yaml` using `openapi-ts`. The `@cosca/sdk` package exports these generated types. The OpenAPI spec is the canonical contract between Go backend and TypeScript frontend.

**Why:**

| Approach | Type Safety | Backend Sync | Maintenance |
|----------|------------|-------------|-------------|
| **OpenAPI codegen** | Full — request and response types generated | Automatic from spec changes | Update spec → regenerate |
| Manual type definitions | Requires discipline | Manual sync, drifts over time | Update both sides manually |
| `tRPC` | Full — inferred from backend code | Automatic | Requires Node.js backend |
| GraphQL codegen | Full from schema | Automatic | Requires GraphQL layer |
| No types (`any`) | None — runtime errors | N/A | Lowest effort, highest risk |

**Implementation:**

1. The Go backend generates or maintains `api/rest/openapi.yaml` as part of its build process
2. A `prebuild` script in `web/package.json` runs `openapi-ts` to generate TypeScript types
3. Generated types land in `web/src/lib/api.ts` (planned as codegen from OpenAPI spec)
4. The `@cosca/sdk` package optionally re-exports these types alongside its hand-written client code
4. 5. CI enforces that generated types are up-to-date (diff check on generated types)

**Build pipeline:**

```
Go Backend Build
  └─► go generate ./...       # Generates api/rest/openapi.yaml from Go route decorators
       └─► api/rest/openapi.yaml

Web Frontend Build
  └─► npm run generate:api     # Runs openapi-ts on api/rest/openapi.yaml
       └─► src/lib/api.ts (generated types)
  └─► npm run build            # Next.js build (imports generated types)
```

**Type usage in feature modules (current: `web/src/features/knowledge/hooks/`):**

```typescript
// features/knowledge/hooks/use-knowledge-search.ts (current)
// Planned target: codegen types in dedicated queries module
import type { KnowledgeSearchResult } from "@/lib/api";  // ← generated types (planned auto-generation)

export function useKnowledgeSearch(query: string) {
  return useQuery<KnowledgeSearchResult[]>({  // ← fully typed
    queryKey: knowledgeKeys.search(query),
    queryFn: () => aosClient.knowledge.search(query),
    enabled: query.length > 0,
  });
}
```

**Alternative rejected:** Manual TypeScript interfaces. Without code generation, any field rename in the Go struct would silently desynchronize the frontend types, causing runtime errors that TypeScript cannot catch. The Go team and frontend team iterate independently — the OpenAPI spec is their contract.

---

### D-007: Dark-First Design System — Organic Aesthetic

**Chosen:** Dark mode as the default and primary design target. Light mode as an accessible alternative activated via user preference toggle. All design tokens defined as CSS custom properties through Tailwind CSS variables.

**Why:**

| Rationale | Detail |
|-----------|--------|
| **Developer tooling aesthetic** | CLI tools, terminals, code editors, and developer dashboards are overwhelmingly dark-themed (VS Code Dark+, iTerm2, Warp, Linear, Vercel). Consistency with the user's workspace reduces cognitive friction. |
| **Ops use case** | Runtime monitoring and log inspection often happen during incidents — at night, in dark rooms. A dark interface reduces glare and eye strain during extended monitoring sessions. |
| **Information density** | Dark backgrounds with colored accents (green health, red errors, amber warnings) create higher contrast for status indicators and data visualizations than light backgrounds. |
| **Battery efficiency** | On OLED displays (increasingly common in developer laptops), dark mode measurably reduces power consumption. |

**Design token architecture:**

```css
/* styles/globals.css — Tailwind CSS variable layer */
@layer base {
  :root {
    /* Organic palette — warm, not clinical */
    --background: 240 10% 3.9%;       /* near-black with blue undertone */
    --foreground: 0 0% 98%;           /* off-white text */
    --card: 240 10% 5.9%;             /* slightly lighter than background */
    --card-foreground: 0 0% 98%;
    --popover: 240 10% 7%;            /* elevated surfaces */
    --popover-foreground: 0 0% 98%;
    --primary: 217 91% 60%;           /* blue accent */
    --primary-foreground: 0 0% 98%;
    --secondary: 240 3.7% 15.9%;      /* muted surface */
    --secondary-foreground: 0 0% 98%;
    --muted: 240 3.7% 15.9%;
    --muted-foreground: 240 5% 64.9%; /* subdued text */
    --accent: 240 3.7% 15.9%;
    --accent-foreground: 0 0% 98%;
    --destructive: 0 62.8% 30.6%;     /* red for errors */
    --destructive-foreground: 0 0% 98%;
    --border: 240 3.7% 15.9%;
    --input: 240 3.7% 15.9%;
    --ring: 240 4.9% 83.9%;
    --radius: 0.75rem;                /* rounded corners — organic feel */
    --chart-1: 220 70% 50%;           /* dashboard chart colors */
    --chart-2: 160 60% 45%;
    --chart-3: 30 80% 55%;
    --chart-4: 280 65% 60%;
    --chart-5: 340 75% 55%;
  }

  .light {
    --background: 0 0% 100%;
    --foreground: 240 10% 3.9%;
    --card: 0 0% 98%;
    --card-foreground: 240 10% 3.9%;
    --popover: 0 0% 100%;
    --popover-foreground: 240 10% 3.9%;
    --primary: 217 91% 50%;
    --primary-foreground: 0 0% 100%;
    --secondary: 240 4.8% 95.9%;
    --secondary-foreground: 240 5.9% 10%;
    --muted: 240 4.8% 95.9%;
    --muted-foreground: 240 3.8% 46.1%;
    --accent: 240 4.8% 95.9%;
    --accent-foreground: 240 5.9% 10%;
    --destructive: 0 84.2% 60.2%;
    --destructive-foreground: 0 0% 98%;
    --border: 240 5.9% 90%;
    --input: 240 5.9% 90%;
    --ring: 240 5.9% 10%;
  }
}
```

**Theme switching:** `next-themes` (`v0.4.4`, already in `package.json`) provides SSR-safe theme detection with `<ThemeProvider>`. The `"dark"` class is applied to `<html>` by default with `forcedTheme` until the user sets a preference. Theme preference is persisted in `localStorage` and respected on subsequent visits.

**Typography:** Inter (UI text) + JetBrains Mono (code, logs, commands). Both are variable fonts loaded via `next/font/google` with subset optimization.

**Alternative rejected:** Light-first with dark mode as an afterthought. Designing light-first and retrofitting dark mode produces poor dark themes with inverted colors, low contrast, and visual artifacts. Dark-first ensures the primary experience is polished.

---

### D-008: REST-First API Strategy — REST Now, gRPC-Web Later

**Chosen:** The frontend communicates exclusively with the Go backend through REST API endpoints proxied via Next.js API routes. gRPC-Web is deferred to Phase 3+.

**Why:**

| Factor | REST (Phase 1-2) | gRPC-Web (Phase 3+) |
|--------|------------------|---------------------|
| Backend readiness | REST handlers already 80% implemented in `internal/` | gRPC proto definitions exist (`proto/`) but no server wiring |
| SDK availability | `@cosca/sdk` fully implemented with axios client | No gRPC-Web client exists |
| Browser support | Universal | Requires gRPC-Web proxy (Envoy) or `@grpc/grpc-js` |
| Debugging | curl, browser DevTools Network tab, Postman | Requires gRPC tools (grpcurl, BloomRPC) |
| Type generation | OpenAPI → TypeScript (mature tools) | Proto → TypeScript (protobuf-ts, ts-proto) |
| Streaming | SSE for real-time data | Native bidirectional streaming |
| Cache compatibility | HTTP cache headers, CDN-friendly | Requires application-level caching |

**API proxy architecture:**

```
Browser                          Next.js Server                    Go Backend
  │                                   │                               │
  │  GET /api/knowledge/search?q=foo  │                               │
  ├──────────────────────────────────►│                               │
  │                                   │  GET /api/v1/knowledge/search │
  │                                   ├──────────────────────────────►│
  │                                   │                               │
  │                                   │◄──────────────────────────────┤
  │◄──────────────────────────────────┤                               │
  │                                   │                               │
```

The Next.js catch-all API route (`app/api/[[...route]]/route.ts`) forwards requests to the Go backend at `http://localhost:14120` (configurable via `COSCA_API_URL` environment variable). This proxy:

1. **Resolves CORS** — Browser talks to same-origin Next.js; Next.js talks to Go backend server-to-server
2. **Strips auth headers** — JWT or API key validated in Next.js middleware; internal service token used for Go backend calls
3. **Adds request ID** — Propagates `x-request-id` header for distributed tracing
4. **Handles streaming** — Passes through SSE streams for real-time endpoints

**Phase 3 gRPC-Web migration path:**

When gRPC-Web is adopted, it will be additive:
- REST endpoints continue to serve the web console for CRUD operations
- gRPC-Web endpoints serve streaming-heavy features (real-time log tail, live metrics, pipeline event streams)
- The `@cosca/sdk` gains a `AosGrpcClient` alongside the existing `AosClient`

**Alternative rejected:** gRPC-Web from day one. The Go backend's gRPC server is not yet production-ready. Building and debugging a gRPC-Web pipeline (proto compilation, Envoy proxy, TypeScript codegen, streaming) would delay Phase 1 delivery by 4–6 weeks with no user-facing benefit for the initial 5-page MVP.

---

### D-009: Multi-Container Deployment — Go + Next.js in Separate Containers

**Chosen:** The Go backend and Next.js frontend deploy as separate Docker containers within the same Kubernetes pod (sidecar pattern) or Docker Compose network. Two separate Dockerfiles: `Dockerfile` (Go, scratch-based) and `Dockerfile.web` (Next.js, node:alpine-based).

**Why:**

| Concern | Single Container | Multi-Container (Chosen) |
|---------|-----------------|--------------------------|
| Base image | Must bundle Node.js in scratch (impossible) or use heavy base | Go on scratch (5 MB), Node.js on alpine |
| Scaling | Both scale together | Frontend scales independently |
| Update frequency | Rebuild both for CSS change | Frontend-only deploys without Go rebuild |
| Security surface | Combined | Isolated per container |
| Resource limits | Shared CPU/memory | Independent requests/limits per container |
| Startup time | Sequential (Go then Node.js) | Parallel startup |

**Dockerfile.web (new):**

```dockerfile
# Stage 1: Build
FROM node:22-alpine AS builder

WORKDIR /app

# Install dependencies (cached layer)
COPY web/package.json web/package-lock.json ./
RUN npm ci --production=false

# Copy source and build
COPY web/ ./
COPY sdk/typescript/ ../sdk/typescript/
COPY api/rest/openapi.yaml ../api/rest/openapi.yaml
RUN npm run generate:api && npm run build

# Stage 2: Runtime
FROM node:22-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV COSCA_API_URL=http://localhost:14120

COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public

EXPOSE 3000

CMD ["node", "server.js"]
```

**Kubernetes pod specification:**

```yaml
# deploy/helm/cosca/templates/deployment.yaml (excerpt)
spec:
  containers:
    - name: cosca-backend
      image: "{{ .Values.image.backend.repository }}:{{ .Values.image.backend.tag }}"
      ports:
        - containerPort: 14120
          name: api
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
        limits:
          cpu: 500m
          memory: 512Mi
    
    - name: cosca-frontend
      image: "{{ .Values.image.frontend.repository }}:{{ .Values.image.frontend.tag }}"
      ports:
        - containerPort: 3000
          name: http
      env:
        - name: COSCA_API_URL
          value: "http://localhost:14120"
      resources:
        requests:
          cpu: 50m
          memory: 128Mi
        limits:
          cpu: 200m
          memory: 256Mi
      readinessProbe:
        httpGet:
          path: /api/health
          port: 3000
```

**Docker Compose development environment:**

```yaml
# docker-compose.yml (extension)
services:
  cosca-backend:
    # existing service
  
  cosca-frontend:
    build:
      context: .
      dockerfile: Dockerfile.web
    ports:
      - "3000:3000"
    environment:
      - COSCA_API_URL=http://cosca-backend:14120
    depends_on:
      - cosca-backend
    volumes:
      - ./web/src:/app/src  # hot reload in development
```

**CI/CD pipeline order:**

```
1. Go build + test → produce cosca binary
2. TypeScript SDK build + test → produce @cosca/sdk dist
3. OpenAPI spec validation → ensure api/rest/openapi.yaml is valid
4. Web build + test → produce Next.js standalone output
5. Docker build (parallel):
   a. Dockerfile → cosca-backend image
   b. Dockerfile.web → cosca-frontend image
6. Helm upgrade → rolling update of same pod with both new images
```

**Alternative rejected:** Single container with Node.js serving both Go binary (child process) and Next.js. This complicates the process lifecycle (who handles SIGTERM?), mixes crash domains (Go panic restarts Node.js server), and forces a heavy base image. It violates the single-responsibility principle at the container level.

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Browser / Client                              │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  Next.js 15 App Router (port 3000)                            │   │
│  │                                                                │   │
│  │  ┌──────────────────────────────────────────────────────┐    │   │
│  │  │                    App Layer                           │    │   │
│  │  │  ┌──────────┐  ┌──────────┐  ┌───────────────────┐  │    │   │
│  │  │  │  Pages   │  │ Layouts  │  │  API Route Proxy  │  │    │   │
│  │  │  │  (RSC)   │  │(nested)  │  │  /api/* → :14120   │  │    │   │
│  │  │  └──────────┘  └──────────┘  └───────────────────┘  │    │   │
│  │  └──────────────────────────────────────────────────────┘    │   │
│  │                                                                │   │
│  │  ┌──────────────────────────────────────────────────────┐    │   │
│  │  │                 Feature Modules                       │    │   │
│  │  │  ┌──────────┐  ┌──────────┐  ┌───────────────────┐  │    │   │
│  │  │  │dashboard │  │knowledge │  │     memory        │  │    │   │
│  │  │  │          │  │          │  │                   │  │    │   │
│  │  │  │• Health  │  │• Search  │  │• Layers           │  │    │   │
│  │  │  │• Stats   │  │• Index   │  │• Timeline         │  │    │   │
│  │  │  │• Activity│  │• Detail  │  │• Records          │  │    │   │
│  │  │  └──────────┘  └──────────┘  └───────────────────┘  │    │   │
│  │  │  ┌──────────┐  ┌──────────┐                         │    │   │
│  │  │  │ runtime  │  │ settings │                         │    │   │
│  │  │  │          │  │          │                         │    │   │
│  │  │  │• Status  │  │• Provider│                         │    │   │
│  │  │  │• Logs    │  │• Plugins │                         │    │   │
│  │  │  │• Metrics │  │• API Key │                         │    │   │
│  │  │  └──────────┘  └──────────┘                         │    │   │
│  │  └──────────────────────────────────────────────────────┘    │   │
│  │                                                                │   │
│  │  ┌──────────────────────────────────────────────────────┐    │   │
│  │  │                    Data Layer                         │    │   │
│  │  │  ┌──────────┐  ┌──────────────┐  ┌───────────────┐  │    │   │
│  │  │  │TanStack  │  │openapi-ts    │  │  @cosca/sdk     │  │    │   │
│  │  │  │  Query   │  │(gen types)   │  │  (axios)      │  │    │   │
│  │  │  └──────────┘  └──────────────┘  └───────────────┘  │    │   │
│  │  └──────────────────────────────────────────────────────┘    │   │
│  │                                                                │   │
│  │  ┌──────────────────────────────────────────────────────┐    │   │
│  │  │               Shared UI Layer                         │    │   │
│  │  │  ┌──────────┐  ┌──────────┐  ┌───────────────────┐  │    │   │
│  │  │  │shadcn/ui │  │ Layouts  │  │ Design Tokens     │  │    │   │
│  │  │  │+ Radix   │  │(organic) │  │ (Tailwind vars)   │  │    │   │
│  │  │  └──────────┘  └──────────┘  └───────────────────┘  │    │   │
│  │  └──────────────────────────────────────────────────────┘    │   │
│  │                                                                │   │
│  │  ℹ️  Dark mode by default · Inter + JetBrains Mono fonts     │   │
│  └──────────────────────────────────────────────────────────────┘   │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ HTTP (port 3000 → 14120)
                               │ Server-to-server, same pod network
┌──────────────────────────────┴──────────────────────────────────────┐
│                      Go Backend (port 14120)                          │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                     REST API Layer                            │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────┐  │   │
│  │  │Knowledge │  │  Memory  │  │ Runtime  │  │  Plugins   │  │   │
│  │  │ Handlers │  │ Handlers │  │ Handlers │  │  Handlers  │  │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └────────────┘  │   │
│  └──────────────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                   Cosca Subsystems                              │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────┐  │   │
│  │  │Knowledge │  │  Memory  │  │   AI     │  │  Editors   │  │   │
│  │  │  Engine  │  │  Engine  │  │Orch/tratn│  │  Adapters  │  │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └────────────┘  │   │
│  └──────────────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │               Infrastructure                                  │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────┐  │   │
│  │  │ SQLite   │  │  Cache   │  │Logging   │  │  Metrics   │  │   │
│  │  │ (FTS5)   │  │ (in-mem) │  │(zerolog) │  │ (Prom)     │  │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └────────────┘  │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  ℹ️  Single binary (scratch) · Go 1.22+ · SQLite embedded           │
└──────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────┐
│                     Kubernetes Pod (Helm)                             │
│  ┌──────────────────────────┐  ┌──────────────────────────┐         │
│  │  cosca-backend             │  │  cosca-frontend            │         │
│  │  :14120 (Go scratch)     │  │  :3000 (Node alpine)     │         │
│  │  CPU: 100m-500m          │  │  CPU: 50m-200m           │         │
│  │  Mem: 128Mi-512Mi        │  │  Mem: 128Mi-256Mi        │         │
│  └──────────────────────────┘  └──────────────────────────┘         │
│  ┌──────────────────────────┐                                         │
│  │  PersistentVolume        │                                         │
│  │  /data (SQLite, configs) │                                         │
│  └──────────────────────────┘                                         │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Dependency Graph

```
┌─────────────────────────────────────────────────────────────────┐
│                    web/package.json                              │
│                                                                  │
│  Dependencies:                                                   │
│    next@^15.1.0                                                  │
│    react@^19.0.0, react-dom@^19.0.0                             │
│    @tanstack/react-query@^5.62.0                                 │
│    @radix-ui/* (8 primitives)                                    │
│    tailwindcss@^3.4.0                                            │
│    class-variance-authority, clsx, tailwind-merge                │
│    lucide-react, next-themes, sonner, framer-motion              │
│                                                                  │
│  DevDependencies:                                                │
│    typescript@^5.7.0, @types/react@^19.0.0                      │
│    eslint@^9.0.0, eslint-config-next@^15.1.0                    │
│    prettier@^3.4.0, prettier-plugin-tailwindcss@^0.6.0          │
│    postcss@^8.4.0, autoprefixer@^10.4.0                         │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            │ transpilePackages: ["@cosca/sdk"]
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                    @cosca/sdk (sdk/typescript/)                    │
│                                                                  │
│  Exports:                                                        │
│    AosClient, AosError                                          │
│    KnowledgeAPI, MemoryAPI, ContextAPI                          │
│    RuntimeAPI, PluginsAPI, DiscoveryAPI                         │
│    TypeScript types (AosClientConfig, KnowledgeSearchResult)    │
│                                                                  │
│  Dependencies:                                                   │
│    axios@^1.7.0, uuid@^9.0.0                                    │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            │ HTTP (REST API)
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Go Backend (internal/)                        │
│                                                                  │
│  Exposes:                                                        │
│    GET  /api/v1/knowledge/search                                │
│    GET  /api/v1/knowledge/stats                                 │
│    POST /api/v1/knowledge/index                                 │
│    GET  /api/v1/memory/search                                   │
│    GET  /api/v1/memory/layers                                   │
│    GET  /api/v1/runtime/status                                  │
│    GET  /api/v1/runtime/metrics                                 │
│    GET  /api/v1/plugins/list                                    │
│    POST /api/v1/plugins/install                                 │
│    ... (50+ endpoints)                                          │
│                                                                  │
│  Contract: api/rest/openapi.yaml                                │
└─────────────────────────────────────────────────────────────────┘
```

---

## Consequences

### Positive

- **Type safety end-to-end.** OpenAPI spec → generated TypeScript types → typed TanStack Query hooks → typed components. A Go struct field rename that would break the frontend is caught at build time, not at 3 AM during an incident.

- **Co-versioned releases.** Since the frontend and backend share the same repository and version tag (e.g., `v1.2.0`), there is never ambiguity about which frontend version works with which backend version. The Helm chart deploys both images from the same tag.

- **Unified CI/CD pipeline.** A single GitHub Actions workflow builds Go → SDK → Web → Docker → Helm in sequence. Breaking changes in the API spec fail the web build before reaching production.

- **Server-side rendering with hydration.** Next.js App Router renders dashboard shells on the server with pre-fetched data, delivering a fully formed page on first load. Client-side hydration makes subsequent navigation instant with TanStack Query's cache.

- **Dark-first design.** The interface is optimized for the primary use case (developers and ops engineers working in dark environments). Light mode exists as an accessibility option without compromising the primary experience.

- **Radix accessibility compliance.** All interactive components inherit WAI-ARIA patterns from Radix UI primitives. Keyboard navigation, screen reader announcements, and focus management are built-in, reducing the accessibility audit burden.

- **Independent scaling.** The Next.js frontend and Go backend have separate Kubernetes resource limits and replica counts. A traffic spike on the dashboard does not starve the API of resources, and vice versa.

- **SDK reuse.** The `@cosca/sdk` TypeScript client used by the web console is the same SDK published for third-party developers. Any improvement to error handling, retry logic, or type definitions benefits both internal and external consumers.

### Negative

- **Monorepo complexity.** The repository contains Go, TypeScript, Docker, Helm, Terraform, and Markdown. New contributors must navigate three build systems (`go build`, `npm run build`, `docker build`). Root-level tooling (Makefile, CI) must coordinate across languages.

- **Go + Node.js CI complexity.** The CI pipeline requires both Go 1.22+ and Node.js 22+ runners. Cross-language integration tests (frontend makes real HTTP calls to Go backend) require a `docker-compose` environment or Kubernetes-in-Docker.

- **Two Docker runtimes.** The Go binary runs on `scratch` (5 MB image) while Next.js requires `node:alpine` (~120 MB image). Deployment artifacts, vulnerability scanning, and base image updates must be managed for both.

- **Next.js learning curve for Go team.** The existing team is Go-focused. React Server Components, streaming SSR, the `"use client"` directive boundary, and the App Router's caching semantics differ significantly from traditional SPA development.

- **SSR complexity for real-time data.** Pages that display live metrics require careful `Suspense` boundary placement and client-side polling fallback. Server Components cannot subscribe to WebSocket or SSE streams — real-time features must be client islands.

- **OpenAPI spec must be maintained.** If the Go backend adds a route without updating `api/rest/openapi.yaml`, the generated TypeScript types become incomplete. CI enforcement (diff check) mitigates this but adds friction to backend development.

- **No offline capability.** The web console is a server-rendered application. Unlike a pure SPA with service workers, there is no offline mode. This is acceptable for an ops dashboard (requires live data) but limits mobile field use.

### Neutral

- **REST over gRPC-Web.** REST is universally debuggable but lacks native streaming. The decision to defer gRPC-Web means real-time features (log tail, live metrics) will initially use SSE polling rather than bidirectional streams.

- **shadcn/ui copy-paste model.** Owning component source code provides full control but requires manual updates when upstream shadcn/ui releases new versions. The trade-off is acceptable because UI components stabilize quickly and breaking changes are rare.

- **Feature-based directory structure.** Engineers must decide where to place cross-cutting code (e.g., a `SearchBar` used by both `knowledge` and `memory` features). The convention is: if used by multiple features, promote to `shared/`; if used by one, keep in the feature.

- **Dark-first may alienate some users.** Users accustomed to light interfaces may find the dark default jarring. The theme toggle is prominent (header, first icon) and the preference persists, mitigating this.

- **Container sidecar pattern.** Running two containers in the same pod couples their lifecycle (both must be healthy for the pod to be Ready). This is standard Kubernetes practice but means a frontend misconfiguration can cause the backend to appear unhealthy.

---

## Alternatives Considered

### Separate Repository for Frontend (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Independent CI/CD, dedicated team ownership, simpler repo structure |
| Cons | Version synchronization requires release orchestration (backend v1.2.0 → frontend v1.2.0), OpenAPI spec must be published as a package or mirrored, cross-cutting PRs require coordination across repos, breaking API changes not caught at build time |
| Verdict | Rejected — the overhead of cross-repo coordination outweighs the monorepo complexity, especially for a release pipeline that must deploy backend and frontend atomically |

### Vite + React SPA (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Simpler mental model (pure client-side app), faster build times with esbuild, smaller learning curve for React developers |
| Cons | No server-side rendering — blank page until JavaScript loads, no SEO, no streaming HTML, manual code splitting, requires separate BFF (Backend-For-Frontend) for API aggregation, all auth logic exposed in client bundle |
| Verdict | Rejected — for a dashboard used during incidents, the Time-to-Interactive of an SPA (~3–5 seconds for 1 MB bundle on slow connection) is unacceptable. SSR delivers meaningful content in under 1 second |

### Remix (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Excellent data loading model (`loader` + `useLoaderData`), progressive enhancement by default, smaller ecosystem than Next.js but more focused |
| Cons | Smaller community and ecosystem — fewer examples, templates, and Stack Overflow answers. No equivalent of shadcn/ui's Remix integration (uses Tailwind but lacks the copy-paste component ecosystem). Vercel's investment in Next.js ensures long-term stability |
| Verdict | Rejected — Next.js's larger ecosystem (shadcn/ui, Vercel Analytics, edge middleware, image optimization) provides a faster path to production for the initial 5-page MVP and better scalability for the 40-page Phase 4 target |

### Light-First Design System (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Familiar to general audience, most SaaS defaults, easier to find light-themed inspiration |
| Cons | The audience is developers and ops engineers — the overwhelming majority use dark-themed tools. Designing light-first and retrofitting dark mode produces poor contrast ratios, inverted colors, and visual inconsistencies. Dark-first ensures the primary experience is carefully designed |
| Verdict | Rejected — the primary user persona works in dark environments (terminals, code editors, incident response). The design should meet them where they are |

### Single Container with Supervisord (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Simpler deployment model (one image, one container), no inter-container networking |
| Cons | Must use a heavy base image (e.g., `ubuntu` or `debian-slim`) to bundle both Go binary and Node.js runtime. Process supervision adds complexity (who restarts a crashed Go binary?). Crash domains are shared — an unhandled Node.js exception takes down the Go backend. Resource limits apply to both processes combined, making tuning impossible |
| Verdict | Rejected — the operational simplicity of one container is outweighed by the operational fragility of mixed runtimes in a single process space. Kubernetes sidecar pattern is the standard solution for this problem |

### TanStack Query vs. SWR vs. RTK Query (Evaluated)

| Library | Bundle | DevTools | Mutations | Pagination | SSR | Ecosystem |
|---------|--------|----------|-----------|------------|-----|-----------|
| **TanStack Query v5** | ~12 KB | ✓ Dedicated panel | ✓ `useMutation` | ✓ `useInfiniteQuery` | ✓ `HydrationBoundary` | Largest |
| SWR v2 | ~5 KB | ✗ | ✗ (manual) | ✓ `useSWRInfinite` | ✓ | Smaller |
| RTK Query | ~32 KB | ✓ (Redux) | ✓ | ✓ | ✓ | Redux ecosystem |

**Verdict:** TanStack Query chosen for its mature mutation API (critical for Settings page where users modify providers, install plugins, update API keys), dedicated DevTools for cache inspection, and larger community. SWR's minimalism is appealing but the lack of built-in mutation support means more boilerplate for write operations.

---

## Related ADRs

- [ADR-001: Cosca Enterprise Architecture](ADR-001-cosca-cli-architecture.md) — Go backend layered architecture that the frontend consumes
- [ADR-003: Multi-Runtime Plugin Architecture](ADR-003-plugin-system.md) — Plugin system that the Settings page will manage
- [ADR-005: AI Orchestration Engine (blueprint)](ADR-005-ai-orchestration.md) — Pipeline architecture whose execution traces the Runtime Monitor visualizes
- [ADR-006: AI Orchestration Engine — Implementation Architecture](ADR-006-ai-orchestration-implementation.md) — Streaming and MAG patterns that the web console must support

### Related Files

| File | Description |
|------|-------------|
| `web/package.json` | Frontend dependencies (Next.js 15, React 19, Radix, TanStack Query) |
| `web/next.config.ts` | Next.js configuration (standalone output, optimized imports) |
| `sdk/typescript/` | `@cosca/sdk` — TypeScript client consumed by the web console |
| `api/rest/openapi.yaml` | OpenAPI specification — canonical API contract (50 operations, 62 schemas) |
| `Dockerfile` | Go backend image (scratch, 5 MB) — real |
| `web/Dockerfile` | Next.js frontend image (node:alpine, ~120 MB) — real |
| `deploy/helm/cosca/` | Kubernetes Helm chart — updated with frontend sidecar |
| `docker-compose.yml` | Development environment — updated with `cosca-frontend` service |

---

*This ADR proposes the architectural foundation for the Cosca web console. Implementation will begin after ADR approval. Phase 1 scope: 5 pages (Dashboard, Knowledge Explorer, Memory Viewer, Runtime Monitor, Settings) with the complete architecture described above.*
