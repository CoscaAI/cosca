# Analytics & Observability Audit Report

> **Author**: cosca-analytics | **Date**: 2026-07-28 | **Task**: Ond 5 Activation
> **Status**: Complete | **Version**: 1.0.0

---

## 1. Executive Summary

O projeto CoscaAI possui uma fundação sólida de telemetria e métricas composta por **4 subsistemas independentes** que cobrem coleta de eventos, métricas de runtime, métricas de orquestração e exposição Prometheus. Foram identificados **13 tipos de eventos de telemetria**, **8 contadores de operação de runtime**, um **pipeline completo de métricas de orquestração**, e **exposição Prometheus com proteção bearer-token**. No entanto, não há dashboards visuais, nem definições de SLO, nem regras de alerta. O projeto está no **Nível 2 de maturidade observability** (coleta presente, visualização/alertas ausentes).

---

## 2. What Is Being Tracked Today

### 2.1 Telemetry Events (`internal/telemetry/`)
| Event Type | Metadata | Purpose |
|-----------|----------|---------|
| `command_executed` | command, duration | CLI usage patterns |
| `index_completed` | source, files_count | Index operations |
| `search_performed` | query_type, result_count | Search volume |
| `context_built` | context_type, tokens | Context construction |
| `memory_stored` | layer, memory_type | Memory operations |
| `plugin_installed` | plugin, version | Plugin ecosystem |
| `error_occurred` | operation, error_code | Error taxonomy |
| `runtime_started` | version, mode | Startup tracking |
| `runtime_stopped` | uptime_seconds, reason | Shutdown tracking |
| `provider_called` | provider, model, tokens | AI provider usage |
| `config_changed` | key, source | Configuration changes |
| `update_checked` | current/latest version | Update awareness |
| `sync_completed` | sync_type, items | Sync operations |

**Storage**: SQLite local (`telemetry.db`) → batched HTTP reporting to `telemetry.cosca.enterprise`
**Privacy**: No PII, no file contents, opt-in/opt-out via `COSCA_TELEMETRY_ENABLED`

### 2.2 Runtime Metrics (`internal/runtime/metrics.go`)
| Metric | Type | Fields |
|--------|------|--------|
| Uptime | Gauge | Seconds since start |
| Operation Counters | Counter ×8 | index, search, context_build, memory_store, plugin_call, error, sync, event |
| Duration Histograms | Histogram ×4 | P50/P95/P99 for index, search, context, memory |
| Component Health | Gauge | Per-component status (healthy/unhealthy/unknown) |
| System Metrics | Gauge | Goroutines (current/min/max/avg), Memory (alloc/total), GC stats |

### 2.3 Orchestration Metrics (`internal/orchestration/metrics.go`)
| Metric Group | What's Tracked |
|-------------|----------------|
| Pipeline | Total/successful/failed requests, success rate, avg duration |
| LLM | Calls, tokens (prompt+completion), errors, fallbacks |
| Router | Decision method (explicit/keyword/search/fallback) |
| Cache | Embed cache hits/misses |
| Memory (MAG) | Memories retrieved, stored, store errors |
| Per-Stage | Calls, successes, errors, error rate, min/avg/max duration |

> ⚠️ **Gap**: Orchestration metrics are in-memory only — **not exposed via Prometheus**.

### 2.4 Prometheus Exposition (`internal/metrics/prometheus.go`)
Exposed at `:8371/metrics` (bearer-token protected):

| Section | Metrics |
|---------|---------|
| Process | goroutines, memory_alloc_bytes, memory_sys_bytes, total_alloc_bytes, gc_pause_ns, gc_total |
| Runtime | uptime_seconds, health, state_info, operations_total (×8), duration_seconds (×12), goroutines, component_health |
| Knowledge | documents_total, chunks_total, entities_total, vectors_total, graph_nodes, db_size_bytes, embedding_requests/tokens, cache_entries/hits/misses by tier |
| Memory | records_total, records_size_bytes, layer_records, layer_size_bytes (per layer) |
| HTTP | requests_total, request_duration_seconds_sum/count (by method, path, status) |
| gRPC | requests_total, request_duration_seconds_sum/count (by method, code) |

### 2.5 REST API Endpoints
| Endpoint | Returns |
|----------|---------|
| `GET /v1/stats` | Agents/skills/providers/workflows count + uptime + version + health |
| `GET /v1/analytics` | Search analytics: total searches, top queries, zero-result queries, 30-day trends |
| `GET /v1/knowledge/stats` | Document/chunk/entity/vector counts, graph stats, embedding stats |
| `GET /v1/memory/stats` | Records/size per layer |
| `GET /v1/graph/stats` | Graph nodes, edges, density, component analysis |
| `GET /health` | Liveness probe |
| `GET /ready` | Readiness probe with subsystem checks (knowledge, memory, runtime) |
| `GET /metrics` (port 8371) | Full Prometheus exposition |

### 2.6 Other Data Sources
- **Audit Log** (`internal/audit/`): SQLite-backed security/admin event log with list/prune (admin-only)
- **Session Auto-Recording** (`internal/memory/session.go`): Saves session summary on shutdown (commits, files changed, quality score delta, uptime)
- **Confidence Tracker** (`internal/confidence/tracker.go`): Per-agent confidence scores by domain

---

## 3. Gaps (Observability Maturity Assessment)

### 3.1 P0 — Critical Gaps (Breaking Observability)
| Gap | Impact | Current State |
|-----|--------|---------------|
| **Orchestration metrics not exposed** | LLM costs, router decisions, cache hit rates invisible to Prometheus/Grafana | In-memory only |
| **No error rate SLOs** | No definition of "acceptable" error rate — reactive only | Error counts exist but no target |
| **No latency SLOs** | No P99 latency targets for any endpoint | P50/P95/P99 exist but no thresholds |
| **No cost tracking** | LLM provider costs not tracked per call or aggregated | Provider calls tracked, but no $/token |

### 3.2 P1 — Significant Gaps (Major Blind Spots)
| Gap | Impact | Recommendation |
|-----|--------|----------------|
| **No dashboard UI** | Metrics are raw Prometheus text — Don/CEO can't consume them | Build or embed Grafana dashboard JSON |
| **No alerting rules** | No alerts for error spikes, latency degradation, health changes | Define AlertManager rules for P0 metrics |
| **No distributed tracing** | Can't trace request flow across subsystems | Add OpenTelemetry instrumentation |
| **Metrics reset on restart** | Runtime counters lost — no historical aggregation | Pipeline to time-series DB (Prometheus long-term storage) |
| **No business KPIs** | Don can't see adoption, churn, DAU, cost/request | Define business metrics layer |
| **Confidence tracker not exposed** | Agent reliability scores not visible in monitoring | Expose via Prometheus or dashboard |

### 3.3 P2 — Nice to Have (Future Enhancements)
| Gap | Recommendation |
|-----|----------------|
| No A/B testing framework | Define experiment framework with statistical validity |
| Telemetry server-side analytics | Build analytics dashboard on telemetry server for aggregate usage patterns |
| No saturation metrics | Add queue depths, connection pool usage, thread pool saturation |
| No change-based metrics | Track metrics delta per deploy/version (canary analysis) |
| Session summaries not aggregated | Build session-over-session trend analysis from auto-recorded sessions |

---

## 4. Proposed Consolidated Dashboard (Executive View)

### Dashboard: "Cosca Executive Overview"

#### Row 1 — Health At-a-Glance
| Panel | Source | Type |
|-------|--------|------|
| System Health | `/v1/stats` health field | Status indicator (green/yellow/red) |
| Uptime | `cosca_runtime_uptime_seconds` | Stat (days:hours:minutes) |
| Active Agents | `/v1/stats` agents count | Stat |
| Active Providers | `/v1/stats` providers | Stat |

#### Row 2 — Traffic & Performance
| Panel | Source | Type |
|-------|--------|------|
| Requests/sec (24h) | `cosca_http_requests_total` rate | Time series |
| P95 Latency (24h) | `cosca_http_request_duration_seconds` | Time series |
| Error Rate % | `cosca_runtime_operations_total{operation="error"}` rate / total | Gauge |
| Success Rate % | Runtime snapshot success_rate | Gauge |

#### Row 3 — Operations Volume
| Panel | Source | Type |
|-------|--------|------|
| Operations by Type (stacked) | `cosca_runtime_operations_total` × 8 | Time series (stacked) |
| LLM Calls + Tokens | Orchestration metrics (to be exposed) | Time series |
| Search Volume + Zero-Result Rate | `/v1/analytics` | Time series + gauge |

#### Row 4 — Data Layer
| Panel | Source | Type |
|-------|--------|------|
| Knowledge Index | `cosca_knowledge_documents_total`, `chunks_total`, `vectors_total` | Stats |
| Memory Records by Layer | `cosca_memory_layer_records` | Bar chart |
| Cache Hit Rate | `cosca_knowledge_cache_hits_total` / (hits + misses) | Gauge |
| DB Size | `cosca_knowledge_db_size_bytes` | Stat |

#### Row 5 — System Resources
| Panel | Source | Type |
|-------|--------|------|
| Memory Usage | `cosca_process_memory_alloc_bytes` | Time series |
| Goroutines | `cosca_runtime_goroutines` | Time series |
| GC Pause | `cosca_process_gc_pause_ns` | Time series |
| Component Health Grid | `cosca_runtime_component_health` | Status grid |

---

## 5. Prioritized Recommendations

### P0 (Do First — This Sprint)
1. **Expose orchestration metrics via Prometheus** — add `writeOrchestrationMetrics()` to `prometheus.go` so LLM costs, router decisions, and cache hit rates enter the observability pipeline
2. **Define SLOs for 3 key signals**:
   - Error Rate SLO: < 1% of requests over 5m window
   - P99 Latency SLO: < 2s for search, < 5s for context build
   - Uptime SLO: 99.9% monthly
3. **Add cost tracking** — store $/token per provider in config and compute `cosca_llm_cost_dollars_total` counter

### P1 (Do Next — Next Sprint)
4. **Build/embed Grafana dashboard JSON** — use the schema from Section 4
5. **Add alerting rules** — AlertManager rules for:
   - Error rate > 1% for 5m → warning
   - P99 latency > 2x baseline → warning
   - Component health degraded → critical
   - High goroutine count (> 1000) → warning
6. **Expose confidence tracker via Prometheus** — add `cosca_agent_confidence` gauge per agent

### P2 (Backlog)
7. Add OpenTelemetry distributed tracing
8. Implement time-series retention (VictoriaMetrics/Thanos)
9. Build business metrics layer (DAU, adoption rate, cost/request)
10. A/B testing framework
11. Session-over-session trend analysis

---

## 6. Files Reviewed

| File | Lines | Purpose |
|------|-------|---------|
| `internal/telemetry/telemetry.go` | 517 | Core telemetry system (SQLite, event recording, global Emit) |
| `internal/telemetry/events.go` | 270 | 13 event types + constructors |
| `internal/telemetry/reporter.go` | 247 | Batched HTTP reporting to telemetry endpoint |
| `internal/runtime/metrics.go` | 461 | Runtime counters, histograms, percentiles, component health, system metrics |
| `internal/orchestration/metrics.go` | 331 | Pipeline, LLM, router, cache, MAG metrics |
| `internal/metrics/prometheus.go` | 525 | Full Prometheus exposition (process, runtime, knowledge, memory, HTTP, gRPC) |
| `api/middleware/metrics.go` | 60 | HTTP metrics capture middleware |
| `api/rest/handler/analytics.go` | 143 | `/v1/analytics` endpoint (search trends, top queries) |
| `api/rest/handler/stats.go` | 105 | `/v1/stats` aggregated platform stats |
| `internal/config/config.go` | 899 | Config with metrics/telemetry feature flags |
| `internal/config/defaults.go` | 187 | Default ports: API 8370, Metrics 8371, gRPC 8372 |
| `internal/audit/audit.go` | 279 | SQLite audit log store |
| `internal/memory/session.go` | 173 | Session auto-recording on shutdown |
| `internal/cli/serve.go` | 582 | Serve command with metrics server setup |

---

*Audit completed by cosca-analytics. Next: track resolution of P0 gaps in future tasks.*
