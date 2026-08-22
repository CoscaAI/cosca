# cosca-runtime — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Session: 2026-07-28 — Runtime Architecture Deep Dive

### 2026-07-28 — Full Runtime Code Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-runtime |
| **Task** | Compare runtime documentation against actual source code, document all discrepancies |
| **Technique** | Level 2 — Code-to-documentation cross-validation: read all 5 runtime source files (runtime.go 682L, daemon.go 494L, lifecycle.go 606L, state.go 510L, metrics.go 461L), compared against docs/runtime/overview.md |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #runtime #state-machine #lifecycle #daemon #metrics #documentation #audit |
| **Related** | internal/runtime/*.go, pkg/cosca/runtime.go, docs/runtime/overview.md |
| **Learned** | 8 states and 20 transitions all confirmed exact match between code and docs. Three critical discrepancies found: 1) Restart() functionally broken — Start() requires Uninitialized state but Restart() calls Stop() (leaves state=Stopped) then Start() immediately fails. Transition Stopped→Uninitialized defined but never executed. 2) EventStartupComplete fires at wrong point — line 333 of runtime.go publishes it during Initializing state, BEFORE any init hooks execute. Subscribers receive startup event when nothing is running. 3) Docs describe 7 metrics that don't exist (runtime.state gauge, runtime.health gauge, runtime.start.duration histogram etc.), but omit 12 real metrics that DO exist (8 atomic counters: index/search/context/memory/plugin/error/sync/event counts; 4 duration histograms with p50/p95/p99: index/search/context/memory; system: goroutines min/max/avg, MemoryUsage, TotalAllocated). Daemon features undocumented: watchdog auto-restarts unhealthy subsystems (Stop+Start, 30s timeout each), stale PID detection via signal 0 probe, sync loop (5min default). Dual signal handling in daemon mode: both Runtime.HandleSignals() and Daemon.handleSignals() register for SIGINT/SIGTERM/SIGHUP — concurrent double-firing is functionally safe (second Stop() hits early-return) but fragile. |
| **Next** | Level 3: Fix Restart() bug, fix EventStartupComplete timing, implement hot reload trigger (documented but missing), create runtime integration test suite for all 20 state transitions |

### 2026-07-28 — Metrics System Architecture
| Field | Value |
|-------|-------|
| **Agent** | cosca-runtime |
| **Task** | Document actual runtime metrics collection architecture |
| **Technique** | Level 1 — Source code tracing: followed Metrics struct fields, atomic operations, histogram implementation, and snapshot generation |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #metrics #runtime #atomic #histogram #go |
| **Related** | internal/runtime/metrics.go |
| **Learned** | Metrics uses atomic operations for counters (8x sync/atomic), custom sorted-slice durationHistogram for percentile calculation (insertion-sort on Record, binary search on Snapshot), sync.Map for component health tracking. No external metrics library (no Prometheus client, no OpenTelemetry SDK). MetricsSnapshot is the public API type (generated on-demand). Some public RuntimeStatus fields (ActiveAgents, ActiveWorkflows, LoadedPlugins, Mode) exist only in pkg/cosca/ API layer — not backed by engine metrics. |
| **Next** | Level 2: Add OpenTelemetry export for metrics, implement Prometheus /metrics endpoint |
