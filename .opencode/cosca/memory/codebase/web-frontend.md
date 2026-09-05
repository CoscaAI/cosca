---
type: codebase
key: web-frontend
tags: [nextjs, react, frontend, typescript]
timestamp: 2026-07-26T00:00:00Z
status: active
---

# Web Console — Next.js 15 Frontend Structure

## Tech Stack
- **Framework**: Next.js 15 (App Router)
- **UI**: Shadcn/UI + Tailwind CSS
- **State**: TanStack Query + Zustand (auth store)
- **Forms**: React Hook Form + Zod
- **Auth**: JWT with refresh token rotation
- **Package Manager**: pnpm
- **Port**: 3000 (dev), served behind Go API in production

## File Count: 188 TSX/TS files

## Route Map (17 routes)

### Auth Group `(auth)/`
| Route | Page | Purpose |
|-------|------|---------|
| `/login` | `login/page.tsx` | Glass-morphism login form |

### Dashboard Group `(dashboard)/`
| Route | Page | Purpose |
|-------|------|---------|
| `/` | `page.tsx` | Dashboard — health, stat cards, subsystem grid |
| `/knowledge` | `knowledge/page.tsx` | Knowledge Explorer — search, facets, stats |
| `/memory` | `memory/page.tsx` | Memory Viewer — 5-layer tabs, cards |
| `/runtime` | `runtime/page.tsx` | Runtime Monitor — state, subsystems, uptime |
| `/settings` | `settings/page.tsx` | Settings — system info, API status, plugins |
| `/agents` | `agents/page.tsx` | Agent List — card grid |
| `/agents/[name]` | `agents/[name]/page.tsx` | Agent Detail — capabilities, tools |
| `/skills` | `skills/page.tsx` | Skill List — category filters |
| `/skills/[name]` | `skills/[name]/page.tsx` | Skill Detail — instructions, tools |
| `/providers` | `providers/page.tsx` | Provider List — icons, colors |
| `/providers/[name]` | `providers/[name]/page.tsx` | Provider Detail — test connection |
| `/workflows` | `workflows/page.tsx` | Workflow List |
| `/workflows/[name]` | `workflows/[name]/page.tsx` | Workflow Detail — steps, run |
| `/admin/users` | `admin/users/page.tsx` | User Management (admin-only) |
| `/admin/api-keys` | `admin/api-keys/page.tsx` | API Key Management (admin-only) |

## Feature-Based Architecture (12 feature modules)

| Feature | Files | Components | Hooks |
|---------|-------|------------|-------|
| `auth/` | 7 | LoginForm, ProtectedRoute, RoleGate | useAuth, useLogin |
| `dashboard/` | 4 | HealthSummary, SubsystemStatus | useDashboard |
| `knowledge/` | 9 | SearchBar, Results, Facets, Stats | useKnowledgeSearch, useKnowledgeStats |
| `memory/` | 9 | LayerTabs, RecordCard, Detail, Stats | useMemorySearch, useMemoryStats, useMemoryRecord |
| `runtime/` | 4 | StateDisplay, SubsystemList | useRuntime |
| `settings/` | 6 | SystemInfo, ApiStatus, PluginList, ConfigViewer | useSettings |
| `agents/` | 6 | AgentCard, AgentDetail, AgentCapabilities, AgentTools | useAgents |
| `skills/` | 5 | SkillCard, SkillDetail, SkillTools | useSkills |
| `providers/` | 6 | ProviderCard, ProviderDetail, TestResult | useProviders |
| `workflows/` | 7 | WorkflowCard, WorkflowDetail, Steps, RunResult | useWorkflows |
| `users/` | 6 | UserTable, AddUserDialog, RoleSelect | useUsers |
| `api-keys/` | 5 | ApiKeyCard, GenerateKeyDialog | useApiKeys |

## Shared Components
| Component | Purpose |
|-----------|---------|
| `StatusBadge` | Color-coded status indicators |
| `StatCard` | Metric display cards |
| `Skeleton` | Loading placeholders |
| `EmptyState` | No-data illustrations |
| `ErrorState` | Error messages with retry |
| `SkipNav` | Accessibility skip-to-content |
| `CommandPalette` | Cmd+K fuzzy search |
| `DataGrid` | Enterprise table (sort, filter, paginate, pin, export) |
| `Charts` | Area, Bar, Line, Pie + MetricCard |
| `Sidebar` | Navigation + user menu |
| `Topbar` | Breadcrumb + actions |
