---
type: testing
key: test-strategy
tags: [testing, strategy, pyramid]
timestamp: 2026-07-26T00:00:00Z
status: active
---

# Cosca — Test Strategy

## Test Pyramid
```
        ╱  E2E  ╲          Playwright (web), integration tests
       ╱──────────╲
      ╱ Integration ╲      test/integration/ (7 files)
     ╱────────────────╲
    ╱   Unit Tests      ╲   50+ test packages, *_test.go files
   ╱──────────────────────╲
  ╱    Go Race Detector    ╲  go test -race in CI
 ╱──────────────────────────╲
```

## Test Types
| Type | Tool | Location | Count |
|------|------|----------|-------|
| Unit | Go testing | `*_test.go` in each package | 50+ packages |
| Integration | Go testing | `test/integration/` | 7 files |
| E2E (web) | Playwright | `web/e2e/` | present |
| Component | Storybook | `web/src/**/*.stories.tsx` | 8+ stories |
| Race | Go race detector | CI: `go test -race` | — |
| Benchmark | Go benchmark | `*_bench_test.go` | 6 files |

## Test Commands
```bash
make test-unit       # go test ./... (short)
make test-integration # go test ./test/integration/...
make test-race       # go test -race ./...
make test-coverage   # go test -cover ./...
make test-bench      # go test -bench=. ./...
```

## Quality Gates
- Every PR must pass `make test-unit` + `make test-race`
- New code should maintain or improve coverage
- No flaky tests — flaky tests are immediately fixed or skipped
- AAA pattern (Arrange, Act, Assert) enforced
