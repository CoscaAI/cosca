# Frontend Architecture

> **Status**: active | **Owner**: Frontend Chief | **Last Updated**: 2026-07-28 | **Version**: 1.4.0-dev

## Overview

The Cosca Web Console is a **Next.js 15** application using the **App Router** with **React 19**, **TypeScript**, **Tailwind CSS v3**, and **Radix UI** primitives. It provides a full dashboard for managing agents, knowledge bases, memory, providers, plugins, workflows, and runtime health.

---

## Tech Stack

| Layer | Technology | Details |
|-------|-----------|---------|
| **Framework** | Next.js 15.1 | App Router, Server Components, Streaming |
| **UI Library** | React 19 | Concurrent features, Server Components |
| **Language** | TypeScript | Strict mode, full type coverage |
| **Styling** | Tailwind CSS 3.4 | Utility-first, dark mode via `next-themes` |
| **Components** | Radix UI | 8 headless primitives (Dialog, DropdownMenu, Tooltip, etc.) |
| **State** | React Context + TanStack Query | Auth via context, server state via Query |
| **Forms** | react-hook-form + zod | Schema-validated, type-safe forms |
| **Charts** | Recharts 3.10 | 8 chart types (Area, Bar, Line, Pie, Radar, Scatter) |
| **Icons** | Lucide React | Consistent icon set |
| **Markdown** | react-markdown + rehype-highlight | Rendered with syntax highlighting |
| **Animations** | Framer Motion 11 | Page transitions, micro-interactions |
| **PWA** | Service Worker + Manifest | Offline support, installable |
| **Testing** | Vitest + Playwright + Storybook | Unit, E2E, visual regression |

---

## Directory Structure

```
web/
├── src/
│   ├── app/                      # Next.js App Router pages
│   │   ├── (auth)/               # Unauthenticated routes
│   │   │   └── login/            # POST /v1/auth/login
│   │   ├── (dashboard)/          # Authenticated routes (24 pages)
│   │   │   ├── layout.tsx        # Sidebar + Topbar shell
│   │   │   ├── page.tsx          # Dashboard home
│   │   │   ├── agents/           # Agent directory + detail ([name])
│   │   │   ├── analytics/        # Usage analytics
│   │   │   ├── context/          # Context builder
│   │   │   ├── executions/       # Execution history ([id])
│   │   │   ├── knowledge/        # Knowledge base search
│   │   │   ├── memory/           # Memory browser
│   │   │   ├── metrics/          # System metrics
│   │   │   ├── orchestration/    # AI Orchestration runner
│   │   │   ├── pipelines/        # Pipeline visual editor
│   │   │   ├── playground/       # LLM provider playground
│   │   │   ├── plugins/          # Plugin marketplace
│   │   │   ├── prompts/          # Prompt library
│   │   │   ├── providers/        # Provider management + compare
│   │   │   ├── runtime/          # Runtime status
│   │   │   ├── settings/         # System settings
│   │   │   ├── skills/           # Skills directory ([name])
│   │   │   ├── templates/        # Workflow templates
│   │   │   ├── workflows/        # Workflow browser ([name])
│   │   │   └── admin/            # Admin: users, API keys, audit, secrets
│   │   └── offline/              # PWA offline fallback
│   ├── components/
│   │   ├── layout/               # Sidebar, Topbar, MobileNav, Breadcrumb
│   │   ├── ui/                   # 14 Radix-based UI primitives
│   │   ├── shared/               # 18 reusable component groups
│   │   ├── command-palette.tsx   # ⌘K/CTRL+K quick actions
│   │   └── command-palette-provider.tsx
│   ├── features/                 # 27 feature modules (domain slices)
│   │   └── {feature}/
│   │       ├── components/       # Feature-specific components
│   │       ├── hooks/            # Feature-specific hooks
│   │       ├── types.ts          # Feature-specific types
│   │       ├── index.ts          # Barrel export
│   │       └── __tests__/        # Feature-specific tests
│   ├── hooks/                    # Global hooks (useMobile, useTablet)
│   ├── lib/                      # API client, constants, utils, useDebounce
│   ├── providers/                # Auth, Query, Theme providers
│   ├── styles/                   # Theme definitions
│   ├── test/                     # Test infrastructure (MSW mocks, setup)
│   └── types/                    # Global TypeScript interfaces
├── public/                       # Static assets + PWA manifest + sw.js
├── e2e/                          # Playwright E2E tests (6 specs)
└── .storybook/                   # Storybook configuration
```

---

## Feature Modules (27)

Each feature is a self-contained domain slice following a consistent convention:

| Feature | Route | Purpose |
|---------|-------|---------|
| `activity` | `/activity` | Activity feed with filtering |
| `admin` | `/admin` | Admin overview (stats, quick actions) |
| `agents` | `/agents` | Agent directory, capability cards, tool detail |
| `analytics` | `/analytics` | Search trends, top queries, metrics summary |
| `api-keys` | `/admin/api-keys` | API key CRUD (generate, revoke) |
| `audit` | `/admin/audit` | Audit log viewer with filtering |
| `auth` | `/login` | Login form, auth store (httpOnly cookies), CSRF |
| `context-builder` | `/context` | Prompt context composer with token counter |
| `dashboard` | `/dashboard` | Health summary, subsystem status |
| `executions` | `/executions` | Execution history with detail view |
| `knowledge` | `/knowledge` | Hybrid search (FTS5 + vector), facets, stats |
| `memory` | `/memory` | Multi-layer memory browser, search, stats |
| `metrics` | `/metrics` | Chart widgets, metric cards |
| `orchestration` | `/orchestration` | Agent + provider selector, streaming output |
| `pipelines` | `/pipelines` | Visual pipeline editor (canvas + palette) |
| `playground` | `/playground` | Multi-provider comparison, streaming |
| `plugins` | `/plugins` | Plugin marketplace (grid + detail) |
| `prompts` | `/prompts` | Prompt library (CRUD, search) |
| `provider-compare` | `/providers/compare` | Side-by-side provider comparison |
| `providers` | `/providers` | Provider management + test results |
| `runtime` | `/runtime` | Runtime state display, subsystem list |
| `secrets` | `/admin/secrets` | Secret manager (create, reveal, delete) |
| `settings` | `/settings` | API status, config viewer, system info |
| `skills` | `/skills` | Skills directory with tool detail |
| `templates` | `/templates` | Workflow template browser |
| `users` | `/admin/users` | User management (CRUD, role assignment) |
| `workflows` | `/workflows` | Workflow browser, steps, run results |

---

## State Management

### Pattern: Context + Query + Module Singletons

| Layer | Technology | What |
|-------|-----------|------|
| **Auth** | Module singleton + Context | httpOnly cookies (no JS-visible tokens), `cosca_auth_state` sentinel cookie |
| **Server state** | TanStack Query v5 | Cache (staleTime: 60s, gcTime: 5min, retry: 1) |
| **Theme** | next-themes (Context) | Dark/Light/System, class-based, cookie-persisted |
| **Command palette** | React Context | Open/close state |

**No Zustand, Redux, or Jotai.** The architecture favors React's native Context for UI state and TanStack Query for data fetching/caching.

### Auth Flow

```
Browser                    Backend
  │                          │
  ├── POST /v1/auth/login ──►
  │                           ├── bcrypt verify (cost 12)
  │                           ├── lockout check (5 fails → 15min)
  │                           ├── GenerateTokenPair (HS256)
  │◄── Set-Cookie: cosca_access_token  (HttpOnly, 24h)
  │◄── Set-Cookie: cosca_refresh_token (HttpOnly, path=/v1/auth/refresh, 7d)
  │◄── Set-Cookie: cosca_auth_state=true (JS-readable, 24h)
  │                           │
  │── GET /v1/auth/me ──────►│
  │   Cookie: cosca_access_token
  │◄── {user} ───────────────│
```

**Key security properties:**
- Tokens never touch JavaScript (httpOnly cookies)
- Auth state detected via `cosca_auth_state` sentinel cookie
- CSRF via double-submit pattern (`csrf_token` cookie + `X-CSRF-Token` header)
- Token rotation on refresh (new access + refresh pair each time)

---

## API Client

### Design: Custom fetch wrapper (`src/lib/api.ts`)

```typescript
const api = new ApiClient("http://localhost:14120")

// GET (credentials: "include" for httpOnly cookies)
api.get<UserResponse>("/v1/auth/me")

// POST (auto-injects X-CSRF-Token header)
api.post<LoginResponse>("/v1/auth/login", { username, password })
```

### Key properties:
- **Auth**: httpOnly cookies sent via `credentials: "include"` — no `Authorization` header
- **CSRF**: Double-submit — `csrf_token` cookie read from document + injected as `X-CSRF-Token` header for all mutating requests
- **401 handling**: Redirects to `/login`
- **Error class**: `ApiError` with `status` and `message`

---

## UI Primitives (14)

All based on **Radix UI** with Tailwind CSS + `class-variance-authority` (shadcn/ui pattern):

| Component | Radix Primitive | Variants |
|-----------|----------------|----------|
| `Avatar` | `@radix-ui/react-avatar` | Size, fallback |
| `Badge` | — (pure Tailwind) | default, secondary, destructive, outline |
| `Breadcrumb` | — (pure composition) | — |
| `Button` | `@radix-ui/react-slot` | default, destructive, outline, secondary, ghost, link; sizes |
| `Card` | — (pure Tailwind) | Header, Content, Footer |
| `Collapsible` | `@radix-ui/react-collapsible` | Open/close animation |
| `Dialog` | `@radix-ui/react-dialog` | Modal, overlay, close button |
| `DropdownMenu` | `@radix-ui/react-dropdown-menu` | Items, submenus, separators |
| `Input` | — (native `<input>`) | Sizes, disabled, error |
| `ScrollArea` | `@radix-ui/react-scroll-area` | Custom scrollbar |
| `Select` | — (native-styled) | — |
| `Separator` | `@radix-ui/react-separator` | Horizontal/vertical |
| `Sheet` | Dialog variant | Slide-in panel (mobile nav) |
| `Tooltip` | `@radix-ui/react-tooltip` | Hover, delay, side |

---

## Shared Components (18)

| Component | Purpose |
|-----------|---------|
| **Charts** (10 files) | Recharts wrappers: Area, Bar, Line, Pie, Radar, Scatter + container, legend, tooltip |
| **Data Grid** (9 files) | Generic sortable/filterable/paginated table with export, selection, pinning |
| **Empty State** | Placeholder for empty lists |
| **Error State** | Error display with retry button |
| **Global Search** | ⌘K modal for knowledge search |
| **Install PWA** | PWA install prompt banner |
| **Keyboard Shortcuts** | `?` modal with all shortcuts |
| **Markdown** | react-markdown renderer with syntax highlighting |
| **Notification Center** | Bell icon + dropdown notification panel |
| **Skeleton** | Loading skeleton placeholder |
| **Skip Nav** | Accessibility skip-to-content link |
| **Split View** | Resizable split-panel layout (adjustable divider) |
| **Stat Card** | Metric display (label, value, icon, trend) |
| **Status Badge** | Colored status indicator (active/error/warning) |
| **SW Registration** | Service worker registration component |
| **Theme Switcher** | Dark/Light/System toggle |
| **Timeline** | Vertical timeline component |
| **Tour** | Product onboarding overlay |

---

## Testing Strategy

### Unit + Integration: Vitest
- **43 test files** across `src/`
- **MSW** for API mocking (30+ endpoint handlers)
- Custom `render()` wrapper with QueryClient + Theme providers
- jsdom environment, globals enabled

### E2E: Playwright
- **6 spec files**: auth, dashboard, navigation, orchestration, admin, helpers
- Auto-starts Go backend (`:14120`) + Next.js dev (`:3000`)
- Chromium-only

### Visual: Storybook 10
- **21 stories** across components
- Addons: accessibility audit, dark mode, docs

---

## PWA Support

| Feature | Implementation |
|---------|---------------|
| **Manifest** | `public/manifest.json` — name "Cosca Console", standalone display |
| **Service Worker** | `public/sw.js` — cache-first, offline fallback to `/offline` |
| **Offline Page** | `/offline` — full offline UI with retry button |
| **Install Prompt** | `beforeinstallprompt` event banner |
| **Icons** | 192×192 and 512×512 SVGs + favicon |

---

> **Related**: [Architecture Overview](../architecture/overview.md) | [Layer Architecture](../architecture/layers.md) | [API Reference](../api-reference/overview.md)
