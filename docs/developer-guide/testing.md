# Developer Guide: Testing

> **Status**: active | **Owner**: QA Chief | **Last Updated**: 2026-07-29

This guide covers the testing strategy, patterns, and requirements for the Cosca project.

---

## Testing Strategy

Cosca employs a five-tier testing strategy spanning Go backend and TypeScript frontend:

```
┌──────────────────────────────────────────┐
│      E2E Tests (slowest)                  │
│   test/e2e/ + web/tests/ (Playwright)     │
├──────────────────────────────────────────┤
│   Frontend Integration Tests              │
│   web/src/**/*.test.tsx (RTL)             │
├──────────────────────────────────────────┤
│   Frontend Unit Tests (Vitest)            │
│   web/src/**/*.test.ts (hooks, stores)    │
├──────────────────────────────────────────┤
│   Backend Integration Tests               │
│   test/integration/                       │
├──────────────────────────────────────────┤
│   Backend Unit Tests (fastest)            │
│   */*_test.go                             │
├──────────────────────────────────────────┤
│   Go Benchmarks                           │
│   */*_bench_test.go                       │
└──────────────────────────────────────────┘
```

| Tier | Speed | Runs in CI | Coverage Target |
|------|-------|-----------|-----------------|
| Go Unit | <1ms per test | Always | 85%+ |
| Go Integration | <100ms per test | Always | 80%+ |
| Frontend Unit (Vitest) | <50ms per test | Always | 80%+ |
| Frontend Integration (RTL) | <200ms per test | Always | 80%+ |
| E2E (Playwright) | <30s per suite | On main/release | Critical paths |
| Benchmark | Varies | On demand | Regression checks |

---

## Unit Testing

### Framework

Unit tests use the standard `testing` package with the following conventions:

```go
package knowledge

import (
    "context"
    "testing"
)

func TestSearchQueryValidation(t *testing.T) {
    t.Parallel() // Enable parallel execution
    // ... test body
}
```

### Test File Conventions

| File Pattern | Content |
|-------------|---------|
| `engine_test.go` | Tests for engine.go |
| `indexer_test.go` | Tests for indexer.go |
| `searcher_test.go` | Tests for searcher.go |
| `types_test.go` | Tests for types.go |
| `mock_test.go` | Mock implementations |

### Mock Implementations

Each package provides mock implementations of its interfaces for use by other packages:

```go
// mock_test.go inside knowledge package
type MockEngine struct {
    SearchFunc func(ctx context.Context, query Query) (*SearchResult, error)
    IndexFunc  func(ctx context.Context, path string) error
}

func (m *MockEngine) Search(ctx context.Context, query Query) (*SearchResult, error) {
    if m.SearchFunc != nil {
        return m.SearchFunc(ctx, query)
    }
    return &SearchResult{}, nil
}

func (m *MockEngine) Index(ctx context.Context, path string) error {
    if m.IndexFunc != nil {
        return m.IndexFunc(ctx, path)
    }
    return nil
}
```

### Table-Driven Tests

Use table-driven tests for testing multiple scenarios:

```go
func TestParseQuery(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name    string
        input   string
        want    Query
        wantErr bool
    }{
        {
            name:  "simple keyword",
            input: "authentication",
            want:  Query{Text: "authentication", Type: QueryTypeKeyword},
        },
        {
            name:  "filtered query",
            input: "auth type:document",
            want:  Query{Text: "auth", Type: QueryTypeDocument, Filters: map[string]string{"type": "document"}},
        },
        {
            name:    "empty query",
            input:   "",
            wantErr: true,
        },
        {
            name:  "phrase query",
            input: `"user authentication flow"`,
            want:  Query{Text: "user authentication flow", Type: QueryTypePhrase},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseQuery(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseQuery() error = %v, wantErr = %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("ParseQuery() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Subtests

Use subtests to group related assertions:

```go
func TestEngineStartStop(t *testing.T) {
    engine := NewEngine(config)

    t.Run("initial state is uninitialized", func(t *testing.T) {
        if got := engine.State(); got != StateUninitialized {
            t.Errorf("State() = %v, want %v", got, StateUninitialized)
        }
    })

    t.Run("start transitions to running", func(t *testing.T) {
        if err := engine.Start(context.Background()); err != nil {
            t.Fatalf("Start() error = %v", err)
        }
        if got := engine.State(); got != StateRunning {
            t.Errorf("State() = %v, want %v", got, StateRunning)
        }
    })

    t.Run("stop transitions to stopped", func(t *testing.T) {
        if err := engine.Stop(context.Background()); err != nil {
            t.Fatalf("Stop() error = %v", err)
        }
        if got := engine.State(); got != StateStopped {
            t.Errorf("State() = %v, want %v", got, StateStopped)
        }
    })
}
```

### Test Helpers

```go
// testutil contains reusable test utilities
package testutil

import (
    "context"
    "os"
    "path/filepath"
)

// TempDir creates a temporary directory for testing.
func TempDir(t testing.TB) string {
    t.Helper()
    dir, err := os.MkdirTemp("", "cosca-test-*")
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { os.RemoveAll(dir) })
    return dir
}

// WriteFile writes content to a file inside the given directory.
func WriteFile(t testing.TB, dir, name, content string) string {
    t.Helper()
    path := filepath.Join(dir, name)
    if err := os.WriteFile(path, []byte(content), 0644); err != nil {
        t.Fatal(err)
    }
    return path
}
```

---

## Integration Testing

### SQLite In-Memory Database

Integration tests use SQLite in-memory databases for fast, isolated testing:

```go
func TestKnowledgeIntegration(t *testing.T) {
    // Create in-memory SQLite database
    db, err := sql.Open("sqlite3", ":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    // Initialize schema
    _, err = db.Exec(schemaSQL)
    if err != nil {
        t.Fatal(err)
    }

    // Create engine with test database
    engine := NewEngine(Config{DB: db})

    // Run integration tests
    ctx := context.Background()
    if err := engine.Index(ctx, "testdata/sample.md"); err != nil {
        t.Fatal(err)
    }

    results, err := engine.Search(ctx, Query{Text: "authentication"})
    if err != nil {
        t.Fatal(err)
    }
    if results.Total == 0 {
        t.Error("expected at least 1 result")
    }
}
```

### Test Fixtures

Integration test fixtures are stored in `test/testdata/`:

```
test/
├── integration/
│   ├── knowledge_test.go
│   └── memory_test.go
├── testdata/
│   ├── sample.md
│   ├── sample.go
│   ├── docs/
│   │   ├── architecture.md
│   │   └── api.md
│   └── config/
│       └── cosca.yaml
└── e2e/
    └── cosca_e2e_test.go
```

---

## End-to-End Testing

E2E tests exercise the full Cosca runtime with a real filesystem:

```go
func TestAOSInstallE2E(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping E2E test in short mode")
    }

    // Create temporary project directory
    projectDir := t.TempDir()

    // Run cosca install
    cmd := exec.Command("cosca", "install", "--project-dir", projectDir)
    output, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("cosca install failed: %v\nOutput: %s", err, output)
    }

    // Verify installation artifacts
    expectedFiles := []string{
        ".cosca/config.yaml",
        ".cosca/knowledge.db",
        ".cosca/plugins/",
    }
    for _, f := range expectedFiles {
        path := filepath.Join(projectDir, f)
        if _, err := os.Stat(path); os.IsNotExist(err) {
            t.Errorf("expected file %s does not exist", path)
        }
    }

    // Verify runtime starts
    cmd = exec.Command("cosca", "status", "--project-dir", projectDir)
    output, err = cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("cosca status failed: %v\nOutput: %s", err, output)
    }
    if !bytes.Contains(output, []byte("healthy")) {
        t.Errorf("expected healthy status, got: %s", output)
    }
}
```

### E2E Test Categories

| Category | Description | Frequency |
|----------|-------------|-----------|
| Installation | `cosca install`, `cosca init` | Every release |
| Knowledge | `cosca knowledge search`, `cosca knowledge index` | Every release |
| Plugins | `cosca plugin install`, plugin lifecycle | Weekly |
| Editors | `cosca editor detect`, `cosca editor setup` | Weekly |
| Runtime | `cosca runtime start/stop/status` | Every release |
| Upgrade | `cosca update`, migration tests | Major releases |

---

## Benchmark Testing

### Go Benchmarks

Performance-sensitive code paths must include benchmarks:

```go
func BenchmarkFTS5Search(b *testing.B) {
    db := setupBenchmarkDB(b, 10000) // 10K documents
    engine := NewEngine(Config{DB: db})
    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := engine.Search(ctx, Query{Text: "benchmark query"})
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkVectorSearch(b *testing.B) {
    db := setupBenchmarkDB(b, 10000)
    engine := NewEngine(Config{DB: db})
    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := engine.Search(ctx, Query{Text: "benchmark", SearchMode: ModeVector})
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkIndexing(b *testing.B) {
    b.StopTimer()
    db := setupBenchmarkDB(b, 0)
    engine := NewEngine(Config{DB: db})
    ctx := context.Background()

    for i := 0; i < b.N; i++ {
        b.StartTimer()
        err := engine.Index(ctx, "testdata/sample.md")
        b.StopTimer()
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

### Running Benchmarks

```bash
# Run all benchmarks
make test-bench

# Run specific benchmark
go test -bench=BenchmarkFTS5Search -benchmem ./internal/search/

# Compare benchmarks
go test -bench=. -benchmem -count=5 ./internal/search/ | benchstat
```

### Performance Budgets

| Operation | Budget (p50) | Budget (p95) |
|-----------|-------------|-------------|
| FTS5 search (100K docs) | 5ms | 20ms |
| Vector search (100K vectors) | 15ms | 50ms |
| Document indexing | 10ms | 50ms |
| Runtime startup | 200ms | 500ms |
| Plugin init (10 plugins) | 100ms | 300ms |

Performance regressions exceeding 20% will fail CI.

---

## Coverage Requirements

> **CI Operational Threshold**: ≥ 70% statement coverage (global), ≥ 60% branch coverage (basic-block approximation), ≥ 80% on changed code (per-PR diff). These are enforced by CI jobs G5 and G5b. The package targets below are internal goals — the CI gate is the hard floor.

### Package Targets (Current State 2026-07-29)

| Package | Target | Current | Status |
|---------|--------|---------|--------|
| `internal/runtime/` | 85% | **97.9%** | ✅ |
| `internal/agents/` | 70% | **88.8%** | ✅ |
| `pkg/cosca/` | 70% | **80.0%** | ✅ |
| `internal/editors/` | 70% | **78.3%** | ✅ |
| `internal/cli/` | 70% | **71.5%** | ✅ |
| `api/grpcserver/` | 70% | **71.1%** | ✅ |
| `internal/plugins/` | 80% |
| `internal/editors/` | 75% |
| `internal/memory/` | 80% |
| `internal/discovery/` | 75% |
| `internal/cache/` | 85% |
| `internal/config/` | 90% |
| `sdk/go/` (planned) | 75% |

### Coverage Commands

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View coverage by function
go tool cover -func=coverage.out

# View coverage in HTML browser
go tool cover -html=coverage.out

# Check against threshold
make coverage-check
```

---

## CI/CD Integration

### GitHub Actions Workflow

```yaml
# .github/workflows/test.yml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: make lint
      - run: make test-race
      - run: make coverage
      - name: Check coverage threshold (operational: 70% global)
        run: |
          go tool cover -func=coverage.out | grep total | awk '{print $3}' | \
          while read pct; do
            if (( $(echo "${pct%\%} < 70" | bc -l) )); then
              echo "Coverage $pct is below 70% threshold"
              exit 1
            fi
          done
```

---

## Test Commands Reference

```bash
make test                 # Run all tests
make test-unit            # Unit tests only
make test-integration     # Integration tests only
make test-e2e             # E2E tests only
make test-bench           # Run benchmarks
make test-race            # Tests with race detector
make test-short           # Skip E2E tests (short mode)
make test-pkg PKG=./internal/search/  # Test specific package
make test-run RUN=TestName            # Run specific test
make coverage             # Generate coverage report
make coverage-html        # HTML coverage browser
make coverage-func        # Per-function coverage
make coverage-check       # Verify coverage thresholds
```

---

**Related**: [Getting Started](getting-started.md) | [Architecture Guide](architecture.md) | [Quality Gates](../../internal/embed/cosca/QUALITY_GATES.md)

---

## Frontend Testing (Vitest + React Testing Library + Playwright)

### Vitest Setup

The frontend uses **Vitest** as the test runner with `jsdom` environment for React component testing.

```bash
cd web

# Run all tests in watch mode
pnpm vitest

# Run all tests once (CI mode)
pnpm vitest run

# Run with coverage
pnpm vitest run --coverage

# Run a specific test file
pnpm vitest run src/features/knowledge/hooks/use-knowledge-search.test.ts

# Run tests matching a pattern
pnpm vitest run -t "DataGrid"
```

**Configuration:** `web/vitest.config.ts`

```typescript
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    globals: true,
    css: true,
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html", "lcov"],
      exclude: ["node_modules/", ".next/", "**/*.stories.tsx"],
      thresholds: {
        statements: 80,
        branches: 80,
        functions: 80,
        lines: 80,
      },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
```

**Test setup file:** `web/src/test/setup.ts`

```typescript
import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

afterEach(() => {
  cleanup();
});
```

### Vitest Commands Summary

| Command | Description |
|---------|-------------|
| `pnpm vitest` | Run tests in watch mode |
| `pnpm vitest run` | Run all tests once (CI mode) |
| `pnpm vitest run --coverage` | Run with coverage report |
| `pnpm vitest run -t "Pattern"` | Run tests matching pattern |
| `pnpm vitest run path/to/file` | Run specific test file |

### React Testing Library Patterns

#### Rendering Components with Providers

```typescript
// web/src/test/utils.tsx
import { render, RenderOptions } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "next-themes";
import React from "react";

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: Infinity,
      },
    },
  });
}

interface CustomRenderOptions extends Omit<RenderOptions, "wrapper"> {
  queryClient?: QueryClient;
}

export function renderWithProviders(
  ui: React.ReactElement,
  options?: CustomRenderOptions
) {
  const queryClient = options?.queryClient ?? createTestQueryClient();

  function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <ThemeProvider attribute="class" defaultTheme="dark" enableSystem={false}>
        <QueryClientProvider client={queryClient}>
          {children}
        </QueryClientProvider>
      </ThemeProvider>
    );
  }

  return {
    queryClient,
    ...render(ui, { wrapper: Wrapper, ...options }),
  };
}

export * from "@testing-library/react";
export { renderWithProviders as render };
```

#### Component Test Example

```typescript
// Example: Testing a shared component
import { render, screen } from "@/test/test-utils";
import { describe, it, expect } from "vitest";
import { StatusBadge } from "@/components/shared/status-badge";

describe("StatusBadge", () => {
  it("renders with correct status text", () => {
    render(<StatusBadge status="healthy" />);
    expect(screen.getByText("healthy")).toBeInTheDocument();
  });

  it("applies correct color for error status", () => {
    render(<StatusBadge status="error" />);
    const badge = screen.getByText("error");
    expect(badge).toHaveClass("bg-destructive");
  });

  it("renders with pulse animation when enabled", () => {
    render(<StatusBadge status="running" pulse />);
    const badge = screen.getByText("running");
    expect(badge.parentElement).toHaveClass("animate-pulse");
  });
});
```

#### Hook Test Example

```typescript
// Example: Testing a TanStack Query hook with MSW
import { renderHook, waitFor } from "@testing-library/react";
import { useKnowledgeSearch } from "@/features/knowledge/hooks/use-knowledge-search";
import { describe, it, expect } from "vitest";

describe("useKnowledgeSearch", () => {
  it("returns search results for a valid query", async () => {
    const { result } = renderHook(
      () => useKnowledgeSearch("authentication"),
      { wrapper: createWrapper() }
    );

    // Initially loading
    expect(result.current.isLoading).toBe(true);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeDefined();
    expect(result.current.data!.length).toBeGreaterThan(0);
  });
});
```

### MSW (Mock Service Worker) Setup

MSW is used to mock all REST API endpoints in tests:

**File:** `web/src/test/mocks/handlers.ts`

```typescript
import { http, HttpResponse } from "msw";

export const handlers = [
  // Knowledge endpoints
  http.post("http://localhost:14120/v1/knowledge/search", async ({ request }) => {
    const body = await request.json();
    return HttpResponse.json({
      results: [
        {
          id: "doc-001",
          title: "Authentication Flow",
          snippet: "The authentication flow uses OAuth 2.0...",
          score: 0.95,
          type: "document",
          path: "docs/api-reference/auth.md",
        },
      ],
      total: 1,
      facets: { types: [{ value: "document", count: 1 }] },
    });
  }),

  // Auth endpoints
  http.post("http://localhost:14120/v1/auth/login", async ({ request }) => {
    const body = await request.json() as { username: string; password: string };
    if (body.username === "admin" && body.password === "admin") {
      return HttpResponse.json({
        access_token: "mock-access-token",
        refresh_token: "mock-refresh-token",
        expires_in: 86400,
      });
    }
    return HttpResponse.json(
      { error: { code: "INVALID_CREDENTIALS", message: "Invalid username or password" } },
      { status: 401 }
    );
  }),

  // ... all 36 endpoints have handlers
];
```

**File:** `web/src/test/mocks/server.ts`

```typescript
import { setupServer } from "msw/node";
import { handlers } from "./handlers";

export const server = setupServer(...handlers);
```

### Playwright E2E Tests

Playwright tests exercise critical user flows across multiple browsers:

```bash
cd web

# Run all E2E tests
npx playwright test

# Run specific test
npx playwright test login.spec.ts

# Run with UI mode
npx playwright test --ui

# Show browser (headed mode)
npx playwright test --headed
```

**Configuration:** `web/playwright.config.ts`

```typescript
import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: true,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: "html",
  webServer: {
    command: "pnpm dev",
    port: 3000,
    reuseExistingServer: !process.env.CI,
  },
  use: {
    baseURL: "http://localhost:3000",
    trace: "on-first-retry",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "firefox", use: { ...devices["Desktop Firefox"] } },
    { name: "webkit", use: { ...devices["Desktop Safari"] } },
  ],
});
```

**10 Critical E2E Paths:**

| # | Path | Priority |
|---|------|----------|
| CP-01 | Login → Dashboard | P0 |
| CP-02 | Knowledge Search | P0 |
| CP-03 | Memory Browse | P0 |
| CP-04 | Orchestration Run | P0 |
| CP-05 | Settings View | P1 |
| CP-06 | Providers List | P1 |
| CP-07 | Command Palette (Cmd+K) | P1 |
| CP-08 | Admin — Create User | P1 |
| CP-09 | Theme Toggle | P2 |
| CP-10 | Error Graceful Degradation | P2 |

### Storybook

Storybook provides an isolated component development environment:

```bash
cd web

# Start Storybook dev server (port 6006)
pnpm storybook

# Build Storybook for static hosting
pnpm build-storybook
```

**Configuration:** `web/.storybook/main.ts`

```typescript
import type { StorybookConfig } from "@storybook/nextjs";

const config: StorybookConfig = {
  stories: ["../src/**/*.stories.@(ts|tsx)"],
  addons: [
    "@storybook/addon-essentials",
    "@storybook/addon-a11y",
    "@storybook/addon-interactions",
  ],
  framework: {
    name: "@storybook/nextjs",
    options: {},
  },
};

export default config;
```

**29+ Stories across all shared components:** Button, Input, Card, Badge, Select, Dialog, Sheet, Tooltip, DropdownMenu, Tabs, Progress, Skeleton, Toast, EmptyState, ErrorState, StatusBadge, StatCard, DataGrid, LineChart, BarChart, PieChart, AreaChart, NotificationPanel, SplitView, Timeline, Tour, KeyboardShortcuts, Breadcrumb, Markdown.

### Coverage Thresholds

| Category | Minimum |
|----------|---------|
| Statements | 80% |
| Branches | 80% |
| Functions | 80% |
| Lines | 80% |

Thresholds are enforced in CI. PRs that drop below 80% are blocked.
