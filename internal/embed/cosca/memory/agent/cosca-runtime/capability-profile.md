# cosca-runtime — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 2

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| State Machine (8 states, 20 transitions) | 0.95 | 1 | success | ↑ |
| Lifecycle Management | 0.85 | 1 | success | ↑ |
| Daemon / Watchdog | 0.85 | 1 | success | ↑ |
| Metrics System (8 counters, 4 histograms) | 0.80 | 1 | success | ↑ |
| Signal Handling (dual registration) | 0.75 | 1 | success | → |
| Graceful Shutdown | 0.70 | 1 | success | → |
| Health Checks | 0.65 | 1 | success | → |
| Hot Reload | 0.10 | 0 | — | → |
| OpenTelemetry Integration | 0.10 | 0 | — | → |
| Integration Testing (20 state transitions) | 0.05 | 0 | — | → |

## Strengths
- **Full code-to-documentation cross-validation**: Read all 5 runtime source files (2,753 lines: runtime.go, daemon.go, lifecycle.go, state.go, metrics.go) and verified all 8 states and 20 transitions — found 3 critical undocumentated discrepancies.
- **Critical bug discovery**: Found that Restart() is functionally broken (Stop() leaves state=Stopped but Start() requires Uninitialized), EventStartupComplete fires before init hooks execute (subscribers receive startup event when nothing is running), and docs claim 7 metrics that don't exist while omitting 12 real ones.
- **Metrics architecture tracing**: Followed the full metrics pipeline — 8x sync/atomic lock-free counters, 4x custom insertion-sort durationHistogram with binary-search Snapshot, sync.Map for component health, no external metrics library — and documented what pkg/cosca/ API fields are not backed by engine metrics.
- **Daemon internals documentation**: Documented undocumented watchdog features — auto-restart unhealthy subsystems (Stop+Start, 30s timeout each), stale PID detection via signal 0 probe, sync loop (5min default), and dual signal handler registration (SIGINT/SIGTERM/SIGHUP in both Runtime and Daemon).

## Weaknesses
- **Has not fixed documented bugs**: All three critical bugs (Restart(), EventStartupComplete timing, metrics misdocumentation) were identified but not resolved.
- **No runtime integration tests**: Has not created test coverage for the 20 state transitions or daemon lifecycle.
- **No hot reload implementation**: Documentation states hot reload exists but codebase has no trigger mechanism — gap identified but not filled.

## Preferred Strategies
- **Line-by-line source tracing**: Reads every source file in the runtime package completely, tracing function calls, state transitions, and goroutine interactions rather than sampling.
- **Doc-vs-code truth table**: Builds explicit comparison tables (documented features vs actual code patterns) to surface discrepancies systematically.
- **Atomic operations tracing**: Follows sync/atomic usage patterns, custom data structure implementations, and public API gaps to understand the complete metrics architecture.

## Known Failure Modes
- None recorded — both learning entries show successful outcomes.

## Evolution Goal
Reach Level 3:
*"Fix the Restart() bug (add Stopped→Uninitialized transition execution), fix EventStartupComplete timing (move after init hooks), implement hot reload trigger, create integration test suite for all 20 state transitions, and add OpenTelemetry/Prometheus metrics export — graduating from documentation archaeology to runtime engineering with production-grade observability."*
