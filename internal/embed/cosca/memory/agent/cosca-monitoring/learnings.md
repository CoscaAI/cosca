# Learnings — cosca-monitoring

> **Agent**: cosca-monitoring (Monitoring Chief)
> **Version**: 1.0.0 | **Date**: 2026-07-28
> **Protocol**: [LEARNING_PROTOCOL.md](../../LEARNING_PROTOCOL.md)

---

## Learning #1 — Initial Metrics Landscape Assessment

**Date**: 2026-07-28
**Source**: Task #1 — SLO Definition, Prometheus Metrics, Grafana Dashboard, Alerting Rules
**Domain**: Observability
**Confidence**: 0.72

### Summary

The Cosca platform has a mature metrics infrastructure already in place. The `/metrics` endpoint is fully implemented with 60+ Prometheus metrics across 6 categories (process, runtime, knowledge, memory, HTTP, gRPC). The metrics use only the Go standard library — no external Prometheus client dependency. The implementation is clean, thread-safe, and well-tested with 9 unit tests covering nil inputs, concurrent access, format compliance, and empty collectors.

### Key Findings

1. **Metrics endpoint is production-ready**: `GET /metrics` on port 14121 (configurable via `--metrics-port=14121`), bearer-token authenticated via `COSCA_METRICS_SECRET`, returns Prometheus text exposition format v0.0.4.

2. **8 atomic counters**: index, search, context_build, memory_store, plugin_call, error, sync, event — all exposed as `cosca_runtime_operations_total{operation="..."}`.

3. **4 duration histograms**: index, search, context, memory — each exposing p50, p95, p99 as `cosca_runtime_duration_seconds{operation="...",quantile="..."}`.

4. **HTTP and gRPC middleware**: `MetricsMiddleware` captures (method, path pattern, status, duration) for every REST request. gRPC interceptor captures (full method, status code, duration). Both use mutex-protected maps.

5. **What is missing for SLOs**:
   - No task routing duration histogram (needed for SLO #1)
   - No session bootstrap duration histogram (needed for SLO #3)
   - No agent confidence gauge (needed for business alert)
   - No memory decay rate gauge (needed for business alert)
   - No task counter by agent/status (needed for throughput dashboard)

6. **Documentation gap**: The `docs/runtime/overview.md` documents 7 metrics that don't exist and misses 12 real metrics (QA bug report from cosca-qa). The monitoring deliverables (this task) should supersede the stale docs.

### Impact on SLO Design

- **SLOs #2, #4, #5** are immediately measurable with existing metrics — no code changes needed.
- **SLOs #1, #3** require instrumentation (P0 priority, ~6h total effort).
- Alerting rules are designed to use existing metrics by default, with commented placeholders for not-yet-implemented business metrics.

### Artifacts Produced

| Artifact | Path | Purpose |
|----------|------|---------|
| SLO Document | `.opencode/cosca/memory/monitoring/slos.md` | 5 SLOs with SLIs, targets, error budgets, runbooks |
| Grafana Dashboard | `.opencode/cosca/memory/monitoring/grafana-dashboard.json` | 4-row dashboard: Health, Performance, Data, Quality |
| Alerting Rules | `.opencode/cosca/memory/monitoring/alerting-rules.yml` | 18 alert rules across 6 groups |
| Learnings | `.opencode/cosca/memory/agent/cosca-monitoring/learnings.md` | This file |

### Caveats

1. **SLO targets are calibrated on bench data, not production data**: The performance baselines from cosca-performance are from `go test -bench`, not production traces. Real-world latencies (especially with embedding providers) may differ significantly. SLOs should be recalibrated after 30 days of production data.

2. **Vector search scaling**: The brute-force O(n) vector search means latency SLOs (#2) will break as vector count grows. The 200ms target is generous for 1K vectors but impossible at 100K vectors without ANN index.

3. **FTS5 schema bug**: The `documents_fts` content-sync schema mismatch (bug-005 territory) means FTS5 document-level search is silently broken. Until fixed, search SLO measurements reflect only chunk-level and entity-level FTS5.

4. **Business metrics are aspirational**: The `cosca_tasks_total`, `cosca_agent_confidence`, `cosca_memory_decay_rate` metrics don't exist yet. Alerting rules using them are included but will be silent until instrumentation is added.

### Recommendations for Next Cycle

1. **Implement P0 instrumentation**: `cosca_task_duration_seconds` (4h) + `cosca_session_duration_seconds` (2h) in `internal/runtime/metrics.go`.
2. **Recalibrate SLOs after 30 days**: Collect real p95 values from production and adjust targets.
3. **Integrate with CI quality gate G7**: Wire benchmark regression detection (cosca-performance) into the monitoring pipeline — if benchmarks degrade >10%, fire alert before deployment.
4. **Add SLO burn-rate dashboards**: Multi-window burn rate panels (1h, 6h, 24h) for proactive error budget tracking.
5. **Implement distributed tracing**: Add trace context propagation for end-to-end request tracing across agent calls (beyond the scope of this task but necessary for debugging SLO violations).

### Cross-References

| Reference | Relevance |
|-----------|-----------|
| `internal/metrics/prometheus.go` | Prometheus format implementation |
| `internal/runtime/metrics.go` | Runtime metrics collection (counters + histograms) |
| `internal/cli/serve.go:340` | `/metrics` endpoint handler |
| `api/middleware/metrics.go` | HTTP middleware for request metrics |
| [Performance Baseline Report](../../performance/baseline-report.md) | Latency baselines for SLO calibration |
| [Quality Gates](../../qa/quality-gates.md) | G7 (Performance Baseline) validation criteria |

---

## Domain Confidence Assessment

### Primary Domain: Observability & Monitoring

**Initial Confidence (baseline)**: 0.25 (pre-task)
**Suggested Current Confidence**: 0.72

### Rationale for 0.72

The confidence is set at 0.72 (not higher) because:

1. **Strengths (+)**:
   - Successfully mapped all 60+ existing metrics to SLO requirements
   - Designed 5 well-calibrated SLOs with quantitative targets derived from empirical benchmarks
   - Created comprehensive Grafana dashboard with 20 panels across 4 sections
   - Defined 18 alerting rules with proper severity escalation (warning → critical)
   - Error budget policies with burn-rate alerting
   - Runbooks for each SLO violation scenario

2. **Uncertainties (-)**:
   - No production data to validate SLO targets (estimated from benchmarks only)
   - SLO #2 (Knowledge Search) has high variance due to embedding provider dependency — actual p95 unknown
   - SLO #3 (Session Bootstrap) uses estimated latency decomposition — never measured in production
   - Vector search scaling curve is theoretical (O(n)) — real degradation point depends on hardware
   - Business metrics (agent confidence, task routing, decay rate) are not instrumented yet

3. **What would increase confidence to >0.85**:
   - 30 days of production metrics with actual p95 values
   - Implementation of P0 instrumentation (task + session duration histograms)
   - At least one real SLO violation handled via the runbooks
   - Calibration of alert thresholds against false-positive/false-negative rates

### Sub-Domains

| Domain | Confidence | Notes |
|--------|-----------|-------|
| Prometheus Metrics | 0.85 | Existing implementation is complete and well-tested |
| SLO Definition | 0.75 | Well-structured SLOs but targets need production validation |
| Grafana Dashboarding | 0.70 | Dashboard designed but not tested against real data |
| Alerting Rules | 0.70 | Rules are syntactically correct but thresholds need tuning |
| Distributed Tracing | 0.15 | Not implemented — identified as gap for future cycle |

### Comparison with Baseline

| Metric | Baseline (cosca-qa) | Current | Delta |
|--------|---------------------|---------|-------|
| Overall Monitoring Confidence | 0.25 | 0.72 | +0.47 |
| Metrics Coverage | Unknown | 60+ metrics, 6 categories | — |
| SLOs Defined | 0 | 5 SLOs with SLI/target/budget | — |
| Alerting Rules | 0 | 18 rules, 6 groups | — |
| Dashboards | 0 | 1 Grafana dashboard (20 panels) | — |

The 0.72 confidence satisfies the ≥0.40 threshold required by the Quality Gate standard (G7) and the QA Chief's sign-off criteria.

---

> **Next Learning Expected**: After 30 days of production metrics — recalibration report with real p95 values.
> **Maintainer**: cosca-monitoring (Monitoring Chief)
> **Protocol Version**: 1.0.0
