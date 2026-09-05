---
type: testing
key: test-coverage
tags: [testing, coverage, metrics]
timestamp: 2026-07-26T00:00:00Z
status: active
---

# Cosca — Test Coverage by Package

## Overall: ~78%

## Coverage Heatmap

### High Coverage (>70%)
| Package | Coverage |
|---------|----------|
| internal/agents/ | 88.8% |
| internal/editors/ | 78.3% |
| internal/discovery/ | 57.7% |
| internal/auth/ | 50.2% |

### Medium Coverage (20-50%)
| Package | Coverage |
|---------|----------|
| internal/cache/ | 45.4% |
| internal/context/ | 39.6% |
| internal/config/ | 21.5% |
| internal/chunker/ | 21.9% |
| internal/cli/ | 13.7% |

### Low/No Coverage (<20%)
| Package | Coverage |
|---------|----------|
| internal/chat/ | 5.2% |
| api/auth/ | 0.0% |
| api/middleware/ | 0.0% |
| api/rest/handler/ | 0.0% |
| cmd/cosca/ | 0.0% |
| internal/diagnostics/ | 0.0% |

## Coverage Gaps (Priority)
1. **api/rest/handler/** (0%) — All 11 handlers need tests
2. **api/middleware/** (0%) — CORS, CSRF, logging, rate limit
3. **internal/chat/** (5.2%) — Chat provider registry, hot reload
4. **internal/cli/** (13.7%) — 54 files, only basic coverage

## Benchmarks
| Benchmark | File |
|-----------|------|
| Runtime | internal/runtime/runtime_bench_test.go |
| Search | internal/search/search_bench_test.go |
| Discovery | internal/discovery/discovery_bench_test.go |
| CLI | internal/cli/cli_bench_test.go |
| Memory | internal/memory/memory_bench_test.go |
| Plugins | internal/plugins/plugin_bench_test.go |
