# React Implementation — Enterprise Grade

> **Version**: 1.0.0 | **Status**: active | **Owner**: Frontend Chief | **Stack**: React 19, TypeScript, Next.js 15, TanStack Query, Zustand

## Project Structure

```
src/
├── app/                        # Next.js App Router pages
├── features/                   # Feature modules (domain-driven)
│   ├── auth/
│   │   ├── components/         # UI components
│   │   ├── hooks/              # Custom hooks
│   │   ├── api.ts              # API calls (TanStack Query)
│   │   ├── store.ts            # Zustand store (if needed)
│   │   └── types.ts            # TypeScript types
│   └── dashboard/
├── components/shared/          # Reusable components
├── lib/                        # Utilities, API client, auth
└── hooks/                      # Global hooks
```

## API Client with JWT

```typescript
// lib/api.ts
const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
    this.name = "ApiError"
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  })

  if (!res.ok) {
    if (res.status === 401) {
      // Attempt token refresh once.
      const refreshed = await refreshAccessToken()
      if (refreshed) {
        return request<T>(path, options) // retry with new token
      }
      // Redirect to login if refresh fails.
      window.location.href = "/login"
      throw new ApiError(401, "Session expired")
    }
    throw new ApiError(res.status, await res.text())
  }

  return res.json()
}

export const api = {
  get: <T>(path: string) => request<T>(path),

  post: <T>(path: string, body: unknown) =>
    request<T>(path, { method: "POST", body: JSON.stringify(body) }),

  put: <T>(path: string, body: unknown) =>
    request<T>(path, { method: "PUT", body: JSON.stringify(body) }),

  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
}
```

## TanStack Query — All States

```typescript
// features/auth/hooks/useAuth.ts
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"

interface User { id: string; name: string; email: string }
interface LoginInput { email: string; password: string }

export function useCurrentUser() {
  return useQuery({
    queryKey: ["user", "me"],
    queryFn: () => api.get<User>("/api/v1/users/me"),
    staleTime: 5 * 60 * 1000,    // 5 min cache
    retry: (count, error) => {
      if (error instanceof ApiError && error.status === 401) return false
      return count < 2
    },
  })
}

export function useLogin() {
  const qc = useQueryClient()

  return useMutation({
    mutationFn: (input: LoginInput) =>
      api.post<{ accessToken: string; refreshToken: string }>("/auth/login", input),
    onSuccess: (data) => {
      // Store tokens in memory (never localStorage for sensitive data).
      setAccessToken(data.accessToken)
      setRefreshToken(data.refreshToken)
      qc.invalidateQueries({ queryKey: ["user"] })
    },
  })
}
```

## Component — All States Handled

```tsx
// features/dashboard/components/dashboard-view.tsx
import { Skeleton } from "@/components/shared/skeleton"
import { ErrorState } from "@/components/shared/error-state"
import { EmptyState } from "@/components/shared/empty-state"
import { useDashboardData } from "../hooks/useDashboard"

export function DashboardView() {
  const { data, isLoading, error, refetch } = useDashboardData()

  // 1. Loading
  if (isLoading) return <DashboardSkeleton />

  // 2. Error (with retry)
  if (error) return (
    <ErrorState
      title="Failed to load dashboard"
      message={error instanceof Error ? error.message : "Unknown error"}
      onRetry={() => refetch()}
    />
  )

  // 3. Empty
  if (!data || data.items.length === 0) return (
    <EmptyState
      icon={LayoutDashboard}
      title="No data yet"
      description="Start by creating your first project."
      action={{ label: "Create Project", href: "/projects/new" }}
    />
  )

  // 4. Success
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      {data.items.map((item) => (
        <DashboardCard key={item.id} item={item} />
      ))}
    </div>
  )
}

function DashboardSkeleton() {
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <Skeleton key={i} className="h-32 rounded-lg" />
      ))}
    </div>
  )
}
```

## Shared Components

```tsx
// components/shared/error-state.tsx
interface ErrorStateProps {
  title: string
  message: string
  onRetry?: () => void
}

export function ErrorState({ title, message, onRetry }: ErrorStateProps) {
  return (
    <div className="flex flex-col items-center justify-center p-8 text-center" role="alert">
      <AlertTriangle className="h-12 w-12 text-destructive" />
      <h2 className="mt-4 text-lg font-semibold">{title}</h2>
      <p className="mt-2 text-sm text-muted-foreground">{message}</p>
      {onRetry && (
        <Button onClick={onRetry} variant="outline" className="mt-4">
          Try Again
        </Button>
      )}
    </div>
  )
}

// components/shared/empty-state.tsx
interface EmptyStateProps {
  icon: React.ComponentType<{ className?: string }>
  title: string
  description: string
  action?: { label: string; href: string }
}

export function EmptyState({ icon: Icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center p-8 text-center">
      <Icon className="h-12 w-12 text-muted-foreground" />
      <h3 className="mt-4 text-lg font-medium">{title}</h3>
      <p className="mt-2 text-sm text-muted-foreground">{description}</p>
      {action && (
        <Button asChild className="mt-4">
          <Link href={action.href}>{action.label}</Link>
        </Button>
      )}
    </div>
  )
}
```

## Build & Test

```bash
# Development
pnpm dev

# TypeScript check
pnpm tsc --noEmit

# Unit tests
pnpm vitest run

# E2E tests
pnpm playwright test

# Build
pnpm build

# Lint
pnpm eslint .

# Bundle analysis
ANALYZE=true pnpm build
```

## Cosca Integration

```bash
cosca knowledge readiness --stack "react,nextjs,tanstack-query,zustand,zod"
cosca knowledge search "React Server Components pattern"
```

## Security Checklist

- [ ] JWT stored in memory (NOT localStorage/sessionStorage)
- [ ] Refresh token in httpOnly cookie (or in-memory with refresh flow)
- [ ] XSS prevention: React's JSX auto-escapes, no `dangerouslySetInnerHTML` without sanitization
- [ ] CSP headers configured (`Content-Security-Policy` in next.config)
- [ ] CORS handled server-side (not via client-side proxy)
- [ ] No API keys in client-side code (use server-side API routes or BFF)
- [ ] `next lint` + `pnpm audit` clean
- [ ] Images from trusted sources only (next/image `remotePatterns`)
