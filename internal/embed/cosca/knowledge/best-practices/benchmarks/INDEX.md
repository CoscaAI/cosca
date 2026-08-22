# Benchmarks — Performance & Quality Baseline Registry

> **Category**: Best Practices → Benchmarks | **Version**: 2.0.0 | **Owner**: Cosca Performance Chief | **Last Updated**: 2026-07-29

## Purpose

Benchmark results: agent performance, coverage evolution, runtime baselines, and regression detection thresholds. Benchmarks are re-executed on each significant code change and compared against baselines.

## Benchmark Reports

| # | File | Category | Description | Date |
|---|------|----------|-------------|------|
| 1 | [`runtime-core-baseline.md`](runtime-core-baseline.md) | runtime | Runtime core operations benchmark (EventBus, state transitions, lifecycle) | 2026-07-29 |
| 2 | [`coverage-evolution.md`](coverage-evolution.md) | coverage | Test coverage evolution across packages (runtime, pkg/cosca, CLI, API) | 2026-07-29 |
| 3 | [`agent-activation-progress.md`](agent-activation-progress.md) | agents | Agent activation progress and capability validation metrics | 2026-07-29 |

## Quick Stats

| Metric | Value | Source |
|--------|-------|--------|
| Runtime core coverage | 97.9% | `go test -cover ./internal/runtime/` |
| Fastest runtime op | NewEventBus: 17.8 ns/op | `runtime_bench_test.go` |
| Slowest runtime op | Startup: 5.3 µs/op | `runtime_bench_test.go` |
| Agents activated | 54/55 (98%) | workspace-state.md |
| Confidence avg | ~0.62 | Confidence Model |
| Test score | 85/100 | Coverage audit 2026-07-29 |
| pkg/cosca coverage | 80.0% | `go test -cover` |
| CLI coverage | 71.5% | `go test -cover` |
| Test files | 18+ (core) | Coverage audit |

## Regression Detection

### Runtime Baseline
| Benchmark | Baseline | Trigger (>20% degradation) |
|-----------|:--------:|:--------------------------:|
| NewEventBus | 17.83 ns | > 21.4 ns |
| StateTransition | 8.92 ns | > 10.7 ns |
| Subscribe | 62.24 ns | > 74.7 ns |
| Publish | 593.4 ns | > 712 ns |
| PublishMultiple | 3,916 ns | > 4,699 ns |
| Startup | 5,310 ns | > 6,372 ns |

### Coverage Baseline
| Package | Baseline | Trigger |
|---------|:--------:|:-------:|
| internal/runtime | 97.9% | < 78% |
| pkg/cosca | 80.0% | < 64% |
| internal/cli | 71.5% | < 57% |
| api/rest/handler | 18.8% | < 15% |

## Benchmark Methodology

- **Runtime benchmarks**: Executed via `go test -bench=. -benchmem ./internal/runtime/`
- **Coverage benchmarks**: Executed via `go test -cover ./...` aggregated by package
- **Agent activation**: Extracted from `workspace-state.md` and CEO activation reports
- **Regression trigger**: >20% degradation for runtime performance; coverage drops below 80% of baseline for safety margin

---

*Previously v1.0.0 — upgraded to v2.0.0 with structured report index, regression detection methodology, and coverage baseline integration.*
