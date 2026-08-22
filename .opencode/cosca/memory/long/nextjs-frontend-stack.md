---
type: long
key: nextjs-frontend-stack
tags: [nextjs, react, frontend, shadcn, cosca]
timestamp: 2026-07-26T00:00:00Z
status: active
agent: Memory Chief
---

# Next.js 15 Frontend Knowledge (Cosca Web Console)

## Architecture
- **Framework**: Next.js 15 (App Router)
- **UI Library**: Shadcn/UI (Radix primitives + Tailwind)
- **Styling**: Tailwind CSS
- **State Management**: TanStack Query (server state) + Zustand (client state/auth)
- **Forms**: React Hook Form + Zod validation
- **Auth**: JWT with refresh token rotation

## Directory Structure
```
web/
├── src/
│   ├── app/                    ← App Router pages
│   │   ├── (auth)/             ← Auth group (login)
│   │   └── (dashboard)/         ← Dashboard group (16 routes)
│   ├── components/             ← Shared components
│   │   ├── layout/             ← Sidebar, Topbar, Breadcrumb
│   │   ├── shared/             ← StatusBadge, StatCard, Skeleton, DataGrid, Charts
│   │   └── ui/                 ← Shadcn UI primitives
│   ├── features/               ← Feature modules (12)
│   ├── providers/              ← Auth, Query, Theme
│   ├── lib/                    ← API client, utils, constants
│   └── test/                   ← MSW mocks, test utils
```

## Key Patterns

### Feature-Based Architecture
Each feature is a self-contained module:
```
features/agents/
├── components/   ← UI components
├── hooks/        ← Data fetching (TanStack Query)
├── types.ts      ← TypeScript types
└── index.ts      ← Public exports
```

### All States Handled
Every page handles: loading, empty, error, success.
Uses shared `Skeleton`, `EmptyState`, `ErrorState` components.

### API Client Pattern
```typescript
// lib/api.ts
const client = {
    get: async <T>(path: string) => {
        const res = await fetch(`${API_URL}${path}`, {
            headers: { Authorization: `Bearer ${token}` }
        });
        if (res.status === 401) await refreshToken();
        return res.json() as T;
    }
};
```

## Key Dependencies
| Package | Usage |
|---------|-------|
| `next` 15 | React framework |
| `@tanstack/react-query` | Server state |
| `zustand` | Client state (auth store) |
| `react-hook-form` + `zod` | Form validation |
| `@radix-ui/*` | Accessible primitives |
| `tailwindcss` | Utility CSS |
| `recharts` | Charts |
| `@tanstack/react-table` | DataGrid |
