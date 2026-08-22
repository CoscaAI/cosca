# Runtime Metrics — Extracted from KERNEL.md §9

> **Source**: KERNEL.md v3.0.1 §9 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 9. RUNTIME METRICS

All Runtime metrics are collected, tagged, aggregated, and exposed for observability, alerting, cost attribution, and continuous improvement. Every component publishes metrics — nothing runs unmeasured.

---

### 9.1 Metrics Governance

#### Metric Ownership

| Metric Domain | Collector | Owner | Exporter | Review Cycle |
|--------------|-----------|-------|----------|-------------|
| Session & Bootstrap | Kernel | Runtime Chief | Prometheus, Dashboard | Quarterly |
| Discovery & Context | Context Engine | Context Chief | Prometheus, Dashboard | Quarterly |
| Memory | Memory Engine | Memory Chief | Prometheus, Dashboard | Quarterly |
| Capability & Workflow | Capability Engine | Workflow Chief | Prometheus, Dashboard | Quarterly |
| Planning & DAG | Planning Engine | Architecture Chief | Prometheus, Dashboard | Quarterly |
| Execution | Execution Engine | Runtime Chief | Prometheus, Dashboard | Monthly |
| Review & Quality | Review/Quality Engine | QA Chief | Prometheus, Dashboard | Monthly |
| Provider & AI | Provider Interface | AI Chief | Prometheus, Cost | Monthly |
| Database & Cache | Database Provider | Database Chief | Prometheus, Dashboard | Monthly |
| Scheduler | Scheduler Engine | Runtime Chief | Prometheus, Auto-scaler | Monthly |
| Cost | Cost Engine | Analytics Chief | Dashboard, Budget | Monthly |
| Health | Health Monitor | Monitoring Chief | Prometheus, Alerting | Monthly |
| Security | Security Engine | Security Chief | Prometheus, Audit | Monthly |

#### Metric Lifecycle

```
DEFINE → REGISTER → COLLECT → AGGREGATE → EXPOSE → RETIRE

DEFINE:     Metric specification created (name, type, unit, tags, description)
REGISTER:   Metric registered in Metrics Registry
COLLECT:    Component starts publishing metric values
AGGREGATE:  Metrics aggregated (rollup, downsampling)
EXPOSE:     Metrics available via exporters (Prometheus, Dashboard, API)
RETIRE:     Metric no longer collected (deprecated → removed)
```

---

### 9.2 Complete Metric Catalog

Every metric across all domains is fully defined with name, type, unit, tags, and description.

#### 9.2.1 Session & Bootstrap Metrics

| Metric | Type | Unit | Tags | Description |
|--------|------|------|------|-------------|
| `session.started.count` | Counter | count | runtime_type | Total sessions started |
| `session.completed.count` | Counter | count | runtime_type, status | Sessions completed (success/failure) |
| `session.duration_ms` | Histogram | ms | runtime_type | Session duration |
| `session.active` | Gauge | count | runtime_type | Currently active sessions |
| `bootstrap.duration_ms` | Histogram | ms | runtime_type | Bootstrap initialization time |
| `bootstrap.providers.initialized` | Gauge | count | — | Number of providers initialized |

#### 9.2.2 Discovery & Context Metrics

| Metric | Type | Unit | Tags | Description |
|--------|------|------|------|-------------|
| `discovery.duration_ms` | Histogram | ms | framework, language | Workspace discovery time |
| `discovery.files_scanned` | Gauge | count | — | Files scanned during discovery |
| `discovery.dependencies.found` | Gauge | count | — | Dependencies detected |
| `context.load.duration_ms` | Histogram | ms | — | Context loading time |
| `context.size_bytes` | Gauge | bytes | — | Context memory size |
| `context.entries` | Gauge | count | — | Context entries loaded |

#### 9.2.3 Memory Metrics

| Metric | Type | Unit | Tags | Description |
|--------|------|------|------|-------------|
| `memory.load.duration_ms` | Histogram | ms | store_type | Memory loading time per store |
| `memory.entries` | Gauge | count | store_type | Total memory entries per store |
| `memory.size_bytes` | Gauge | bytes | store_type | Memory size per store |
| `memory.read.latency_ms` | Histogram | ms | store_type | Memory read latency |
| `memory.write.latency_ms` | Histogram | ms | store_type | Memory write latency |
| `memory.search.latency_ms` | Histogram | ms | store_type | Memory search latency |
| `memory.promote.count` | Counter | count | from_store, to_store | Memory promotion count |

#### 9.2.4 Capability & Workflow Metrics

| Metric | Type | Unit | Tags | Description |
|--------|------|------|------|-------------|
| `capability.resolution.duration_ms` | Histogram | ms | request_type | Capability resolution time |
| `capability.resolution.count` | Counter | count | capability_id | Capability resolution count |
| `capability.quality.score` | Gauge | score | capability_id | Current capability quality score |
| `capability.quality.slo_attainment` | Gauge | percent | capability_id | SLO attainment percentage |
| `capability.fallback.count` | Counter | count | capability_id, fallback_reason | Capability fallback count |
| `workflow.duration_ms` | Histogram | ms | workflow_id | Workflow execution duration |
| `workflow.step.duration_ms` | Histogram | ms | workflow_id, step | Per-step duration |
| `workflow.count` | Counter | count | workflow_id | Workflow execution count |
| `workflow.failure.count` | Counter | count | workflow_id | Workflow failures |

#### 9.2.5 Planning & DAG Metrics

| Metric | Type | Unit | Tags | Description |
|--------|------|------|------|-------------|
| `planning.time_ms` | Histogram | ms | complexity | Planning time |
| `planning.complexity` | Gauge | score | — | Plan complexity score |
| `dag.nodes` | Gauge | count | — | DAG node count |
| `dag.edges` | Gauge | count | — | DAG edge count |
| `dag.critical_path_duration_ms` | Histogram | ms | — | Critical path duration |
| `dag.validation.duration_ms` | Histogram | ms | — | DAG validation time |
| `dag.validation.violations` | Counter | count | severity | Validation violations |
| `dag.optimization.savings_ms` | Histogram | ms | optimization_type | Time saved by optimization |

#### 9.2.6 Execution Metrics

| Metric | Type | Unit | Tags | Description |
|--------|------|------|------|-------------|
| `execution.time_ms` | Histogram | ms | — | Total execution time |
| `execution.step.count` | Gauge | count | — | Steps executed |
| `execution.parallel.count` | Gauge | count | — | Parallel execution count |
| `execution.worker.count` | Gauge | count | worker_type | Active workers |
| `execution.retry.count` | Counter | count | step_type, capability | Retry count |
| `execution.retry.duration_ms` | Histogram | ms | step_type | Retry duration |
| `execution.failure.count` | Counter | count | step_type, failure_reason | Execution failures |
| `execution.success.count` | Counter | count | — | Successful executions |
| `execution.success.rate` | Gauge | rate | — | Success rate (0-1) |
| `execution.node.duration_ms` | Histogram | ms | node_type, capability | Per-node execution time |
| `execution.node.queue_time_ms` | Histogram | ms | node_type | Time from READY to DISPATCHING |

#### 9.2.7 Review & Quality Metrics

| Metric | Type | Unit | Tags | Description |
|--------|------|------|------|-------------|
| `review.time_ms` | Histogram | ms | review_type | Review duration |
| `review.score` | Gauge | score | review_type | Review score (0-10) |
| `review.issues.found` | Gauge | count | review_type, severity | Issues found in review |
| `review.issues.fixed` | Counter | count | review_type | Issues fixed after review |
| `review.pass.rate` | Gauge | rate | review_type | Review pass rate |
| `quality.gate.score` | Gauge | score | gate_id | Quality gate score |
| `quality.gate.pass.count` | Counter | count | gate_id | Gate passes |
| `quality.gate.fail.count` | Counter | count | gate_id | Gate failures |
| `quality.gate.duration_ms` | Histogram | ms | gate_id | Gate execution time |
| `quality.score.overall` | Gauge | score | — | Overall quality score |
| `quality.test.coverage` | Gauge | percent | — | Test coverage percentage |

#### 9.2.8 Provider & AI Metrics

| Metric | Type | Unit | Tags | Description |
|--------|------|------|------|-------------|
| `provider.latency_ms` | Histogram | ms | provider_id, model | Provider request latency |
| `provider.errors` | Counter | count | provider_id, error_type | Provider errors |
| `provider.failover.count` | Counter | count | from_provider, to_provider | Provider failover count |
| `provider.failover.duration_ms` | Histogram | ms | from_provider, to_provider | Failover duration |
| `provider.requests.total` | Counter | count | provider_id | Total provider requests |
| `provider.availability` | Gauge | percent | provider_id | Provider availability |
| `provider.circuit_breaker.state` | Gauge | state | provider_id | CB state (1=closed, 2=half, 3=open) |
| `provider.circuit_breaker.tripped` | Counter | count | provider_id | CB trip count |
| `embedding.time_ms` | Histogram | ms | model | Embedding generation time |
| `embedding.tokens` | Histogram | count | model | Tokens per embedding |
| `embedding.count` | Counter | count | model | Embedding count |
| `inference.time_ms` | Histogram | ms | model, provider | Inference time |
| `inference.tokens.input` | Histogram | count | model | Input tokens |
| `inference.tokens.output` | Histogram | count | model | Output tokens |

#### 9.2.9 Database & Cache Metrics

| Metric | Type | Unit | Tags | Description |
|--------|------|------|------|-------------|
| `db.latency_ms` | Histogram | ms | db_instance, query_type | Database query latency |
| `db.connections` | Gauge | count | db_instance | Active connections |
| `db.connection.pool.utilization` | Gauge | percent | db_instance | Connection pool usage |
| `db.errors` | Counter | count | db_instance, error_type | Database errors |
| `db.query.count` | Counter | count | db_instance, query_type | Query count |
| `db.slow_queries` | Counter | count | db_instance | Queries > 1s |
| `redis.latency_ms` | Histogram | ms | redis_instance, command | Redis command latency |
| `redis.hit.rate` | Gauge | rate | redis_instance | Cache hit rate |
| `redis.memory.usage` | Gauge | percent | redis_instance | Memory utilization |
| `redis.connections` | Gauge | count | redis_instance | Active connections |
| `redis.errors` | Counter | count | redis_instance | Redis errors |
| `vector.search.latency_ms` | Histogram | ms | index_name | Vector search latency |
| `vector.search.count` | Counter | count | index_name | Vector search count |
| `vector.index.size` | Gauge | bytes | index_name | Vector index size |

#### 9.2.10 Scheduler Metrics

| Metric | Type | Unit | Tags | Description |
|--------|------|------|------|-------------|
| `scheduler.queue.depth` | Gauge | count | queue_name | Queue depth per queue |
| `scheduler.queue.wait_time_ms` | Histogram | ms | queue_name, priority | Time in queue |
| `scheduler.throughput` | Gauge | count/s | queue_name | Items processed per second |
| `scheduler.dispatch.count` | Counter | count | queue_name | Items dispatched |
| `scheduler.priority.inversion.count` | Counter | count | — | Priority inversion events |
| `scheduler.worker.utilization` | Gauge | percent | worker_pool | Worker pool utilization |
| `scheduler.backpressure.level` | Gauge | level | — | Current backpressure level (0-4) |

#### 9.2.11 Cost Metrics

| Metric | Type | Unit | Tags | Description |
|--------|------|------|------|-------------|
| `cost.execution.total_usd` | Counter | usd | session_id | Total execution cost |
| `cost.token.input_usd` | Counter | usd | model, provider | Input token cost |
| `cost.token.output_usd` | Counter | usd | model, provider | Output token cost |
| `cost.provider.usd` | Counter | usd | provider_id | Per-provider cost |
| `cost.capability.usd` | Counter | usd | capability_id | Per-capability cost |
| `cost.session.usd` | Counter | usd | session_id | Per-session cost |
| `cost.daily.total_usd` | Counter | usd | — | Daily total cost |
| `cost.monthly.total_usd` | Counter | usd | — | Monthly total cost |
| `cost.budget.utilization` | Gauge | percent | budget_category | Budget utilization |

#### 9.2.12 Security Metrics

| Metric | Type | Unit | Tags | Description |
|--------|------|------|------|-------------|
| `security.vulnerabilities.critical` | Gauge | count | scanner | Critical vulnerabilities |
| `security.vulnerabilities.high` | Gauge | count | scanner | High vulnerabilities |
| `security.scan.duration_ms` | Histogram | ms | scanner | Security scan duration |
| `security.secrets.detected` | Counter | count | severity | Secrets detected |
| `security.auth.failures` | Counter | count | auth_method | Authentication failures |
| `security.auth.successes` | Counter | count | auth_method | Authentication successes |

#### 9.2.13 Chief & Specialist Metrics

| Metric | Type | Unit | Tags | Description |
|--------|------|------|------|-------------|
| `chief.usage.count` | Counter | count | chief_id | Tasks assigned |
| `chief.latency_ms` | Histogram | ms | chief_id | Chief response time |
| `chief.error.rate` | Gauge | rate | chief_id | Chief error rate |
| `specialist.usage.count` | Counter | count | specialist_type | Specialist invocations |
| `specialist.latency_ms` | Histogram | ms | specialist_type | Specialist response time |
| `specialist.success.rate` | Gauge | rate | specialist_type | Specialist success rate |

---

### 9.3 Metric Schema Registry

Every metric is formally defined in the Metrics Schema Registry.

```yaml
metric_definition:
  name: "metric.name"
  version: "1.0.0"
  status: "active | deprecated | retired"
  
  type: "counter | gauge | histogram | summary"
  
  unit: "ms | count | bytes | usd | score | rate | percent | level | state"
  
  description: "Human-readable description of what this metric measures"
  
  tags:
    - name: "tag_name"
      description: "Tag description"
      required: true | false
      values: []  # Enum if applicable
      
  collector:
    component: "component-name"
    method: "instrumentation | logging | polling"
    interval_ms: 10000  # For gauges
    
  exporter:
    - "prometheus"
    - "dashboard"
    - "cost_engine"
    - "alerting"
    
  aggregation:
    function: "sum | avg | min | max | p50 | p95 | p99 | count"
    window_ms: 60000  # Aggregation window
    
  retention:
    raw: "24 hours"
    p50_p95_p99: "90 days"
    daily_rollup: "1 year"
    monthly_rollup: "7 years"
    
  alert:
    enabled: true | false
    warning_threshold: 0.0
    critical_threshold: 0.0
    
  cost:
    attribution: "per_session | per_capability | per_provider"
    factor: 0.0  # Cost multiplier if applicable
```

---

### 9.4 Metric Collection Pipeline

```
┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐
│Component  │   │ Collector │   │Aggregator│   │ Exporter │   │Consumer  │
│           │   │           │   │          │   │          │   │          │
│ Publish   │──▶│ Buffer    │──▶│ Rollup   │──▶│ Format   │──▶│ Dashboard│
│ metric    │   │ (10s)     │   │ (60s)    │   │          │   │ Prometheus│
│           │   │           │   │          │   │          │   │ Cost     │
│           │   │           │   │          │   │          │   │ Alerting │
└──────────┘   └──────────┘   └──────────┘   └──────────┘   └──────────┘
```

#### Collection Stages

| Stage | Function | Buffer | Latency SLA | Reliability |
|-------|----------|--------|-------------|-------------|
| **Publish** | Component emits metric value | In-process buffer | < 1ms | Best-effort |
| **Collect** | Collector receives and batches | 10s window | < 100ms | At-least-once |
| **Aggregate** | Aggregator rolls up (p50/p95/p99) | 60s window | < 1s | Exactly-once |
| **Export** | Exporter formats for consumer | — | < 100ms | At-least-once |

---

### 9.5 Metric Exporters

| Exporter | Protocol | Format | Port | Endpoint | Consumer |
|----------|----------|--------|------|----------|----------|
| **Prometheus** | HTTP scrape | OpenMetrics | 9090 | `/metrics` | Prometheus server |
| **Dashboard** | SSE push | JSON | — | `/api/v1/metrics` | Web UI |
| **Cost Engine** | Internal | JSON | — | Internal queue | Cost attribution |
| **Alerting** | Internal | JSON | — | Alert rule evaluator | PagerDuty, Email |
| **Log** | File/Stdout | JSON | — | `cosca-metrics.log` | Debugging, archive |
| **OpenTelemetry** | gRPC | OTLP | 4317 | — | OTel collector |

#### Prometheus Metric Types

```
# Counter: cumulative, only increases
metric_name_total{tag="value"} 1234

# Gauge: can go up and down
metric_name{tag="value"} 98.5

# Histogram: observations in buckets
metric_name_bucket{tag="value", le="0.1"} 100
metric_name_bucket{tag="value", le="1.0"} 500
metric_name_bucket{tag="value", le="+Inf"} 600
metric_name_sum{tag="value"} 45000
metric_name_count{tag="value"} 600
```

---

### 9.6 Metric Retention & Rollup

| Tier | Resolution | Retention | Storage | Queryable | Cost |
|------|-----------|-----------|---------|-----------|------|
| **Raw** | Full resolution | 24 hours | Redis/Memory | Real-time | High |
| **1min** | 1-min aggregates | 7 days | TSDB | Seconds | Medium |
| **5min** | 5-min aggregates | 30 days | TSDB | Seconds | Medium |
| **1hour** | Hourly rollups | 1 year | TSDB/DB | Seconds | Low |
| **1day** | Daily rollups | 7 years | DB/Object | Minutes | Very low |

#### Rollup Functions

| Original Type | Rollup Function | Use Case |
|--------------|----------------|----------|
| Counter | Sum, Rate | Total counts, throughput |
| Gauge | Avg, Min, Max | Utilization, utilization range |
| Histogram | p50, p95, p99, Avg | Latency distribution |
| Summary | p50, p95, p99 | Pre-computed quantiles |

---

### 9.7 Metric Dashboards

#### Dashboard: Runtime Overview

```yaml
dashboard_runtime_overview:
  refresh: "10s"
  time_range: "1h | 6h | 24h | 7d"
  
  rows:
    - name: "Session Activity"
      panels:
        - "Active sessions (gauge)"
        - "Session start rate (graph)"
        - "Session duration p50/p95/p99 (graph)"
        
    - name: "Execution Performance"
      panels:
        - "Execution time p50/p95/p99 (graph)"
        - "Steps per session (gauge)"
        - "Parallelism (graph)"
        - "Worker utilization (gauge)"
        
    - name: "Quality & Reliability"
      panels:
        - "Quality score (gauge)"
        - "Success rate (gauge)"
        - "Retry rate (graph)"
        - "Failure rate by type (table)"
        
    - name: "Cost & Usage"
      panels:
        - "Cost per hour (graph)"
        - "Cost by provider (pie)"
        - "Token usage (graph)"
        - "Budget utilization (gauge)"
```

#### Dashboard: Provider Performance

```yaml
dashboard_provider:
  refresh: "30s"
  time_range: "1h | 6h | 24h"
  
  rows:
    - name: "Provider Health"
      panels:
        - "Provider availability (table)"
        - "Circuit breaker states (table)"
        - "Failover events (timeline)"
        
    - name: "Provider Latency"
      panels:
        - "Latency p50/p95/p99 by provider (graph)"
        - "Latency heatmap (heatmap)"
        
    - name: "Provider Cost"
      panels:
        - "Cost by provider (graph)"
        - "Cost per model (table)"
        - "Token usage by model (graph)"
```

#### Dashboard: DAG Execution

```yaml
dashboard_dag:
  refresh: "5s"
  time_range: "Current execution"
  
  rows:
    - name: "DAG Progress"
      panels:
        - "DAG status (state timeline)"
        - "Completed / Total nodes (progress bar)"
        - "Critical path remaining (gauge)"
        
    - name: "Node Execution"
      panels:
        - "Running nodes (table)"
        - "Node duration by type (graph)"
        - "Queue wait time (graph)"
        
    - name: "Parallelism"
      panels:
        - "Parallel count (graph)"
        - "Resource utilization (graph)"
```

---

### 9.8 Metric Alerting

#### Alert Rules

| Rule | Metric | Condition | Severity | Notification |
|------|--------|-----------|----------|-------------|
| High latency | `execution.time_ms` p95 > 300s | 5 min | WARNING | Email |
| Critical latency | `execution.time_ms` p95 > 600s | 2 min | CRITICAL | PagerDuty |
| High failure rate | `execution.failure.count` rate > 0.1 | 5 min | WARNING | Email |
| Critical failure rate | `execution.failure.count` rate > 0.25 | 2 min | CRITICAL | PagerDuty |
| Provider degraded | `provider.latency_ms` p95 > 30s | 5 min | WARNING | Email |
| Provider down | `provider.availability` < 0.95 | 2 min | CRITICAL | PagerDuty |
| Cost anomaly | `cost.daily.total_usd` > 2x baseline | 1 hour | WARNING | Email |
| Budget exceeded | `cost.budget.utilization` > 0.9 | 1 hour | CRITICAL | PagerDuty |
| Queue backlog | `scheduler.queue.depth` > 1000 | 5 min | WARNING | Email |
| Queue critical | `scheduler.queue.depth` > 5000 | 2 min | CRITICAL | PagerDuty |
| Quality drop | `quality.score.overall` < 5.0 | 1 hour | WARNING | Email |
| Quality critical | `quality.score.overall` < 3.0 | 30 min | CRITICAL | PagerDuty |
| Memory high | `memory.size_bytes` > 90th percentile | 1 hour | WARNING | Email |
| DB slow | `db.latency_ms` p95 > 1s | 5 min | WARNING | Email |
| Redis low hit rate | `redis.hit.rate` < 0.5 | 1 hour | WARNING | Email |

---

### 9.9 Metric Labels & Tags Taxonomy

Every metric MUST be tagged with standardized labels.

#### Required Tags (all metrics)

| Tag | Description | Example | Cardinality |
|-----|-------------|---------|-------------|
| `runtime_type` | Runtime implementation | `opencode`, `cosca-runtime` | < 10 |
| `component` | Component name | `execution-engine` | < 50 |
| `session_id` | Session identifier | `uuid` | High (session-scoped) |

#### Optional Tags (domain-specific)

| Tag | Domain | Example | Cardinality |
|-----|--------|---------|-------------|
| `capability_id` | Capability | `CAP-ENG-001` | < 100 |
| `provider_id` | Provider | `openai`, `anthropic` | < 20 |
| `model` | AI Model | `gpt-4`, `claude-3` | < 50 |
| `workflow_id` | Workflow | `feature-development` | < 50 |
| `gate_id` | Quality Gate | `gate-2` | < 10 |
| `store_type` | Memory Store | `project`, `decision` | < 10 |
| `queue_name` | Scheduler Queue | `immediate`, `priority` | < 10 |
| `node_type` | DAG Node | `capability`, `review` | < 10 |
| `review_type` | Review | `code`, `architecture` | < 10 |
| `db_instance` | Database | `primary`, `replica` | < 10 |
| `status` | Status | `success`, `failure` | < 10 |
| `severity` | Severity | `error`, `warn` | < 10 |

#### Cardinality Limits

```yaml
cardinality_limits:
  per_metric_max_tags: 10
  per_tag_max_values: 1000  # Except session_id (high cardinality, use with care)
  high_cardinality_tags: ["session_id"]
  action_on_exceed: "Drop tag from metric, log warning"
```

---

### 9.10 Metric Query API

The Metrics API exposes metrics for programmatic access.

| Endpoint | Method | Description | Parameters |
|----------|--------|-------------|------------|
| `/api/v1/metrics` | GET | List all metric names | — |
| `/api/v1/metrics/{name}` | GET | Get metric definition | — |
| `/api/v1/metrics/{name}/values` | GET | Get metric values | `from`, `to`, `step`, `tags` |
| `/api/v1/metrics/query` | POST | PromQL-like query | `query` (PromQL string) |
| `/api/v1/metrics/dashboard/{name}` | GET | Dashboard data | `from`, `to` |

---

### 9.11 Metric Cost Attribution

Metrics feed into the Cost Engine for cost attribution.

```yaml
cost_attribution:
  model: "per-session with breakdown by capability and provider"
  
  formulas:
    token_cost: "input_tokens × input_price + output_tokens × output_price"
    provider_cost: "request_count × price_per_request"
    execution_cost: "duration_ms × resource_price_per_ms"
    total_cost: "token_cost + provider_cost + execution_cost"
    
  attribution_tags:
    - "session_id"
    - "capability_id"
    - "provider_id"
    - "model"
    
  reports:
    - "Daily cost by session"
    - "Weekly cost by capability"
    - "Monthly cost by provider"
    - "Budget vs actual"
```



