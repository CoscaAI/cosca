# Runtime Health — Extracted from KERNEL.md §8

> **Source**: KERNEL.md v3.0.1 §8 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 8. RUNTIME HEALTH

Every Runtime component MUST expose health status. The Health Monitor aggregates individual component health into an **overall Runtime Health Score**, triggers alerts, manages circuit breakers, and coordinates recovery.

---

### 8.1 Health Monitor Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              HEALTH MONITOR                                  │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                        PROBE EXECUTOR                                │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │  │
│  │  │  Readiness   │  │   Liveness   │  │   Startup    │              │  │
│  │  │  Probe       │  │   Probe      │  │   Probe      │              │  │
│  │  │  (10s)       │  │   (30s)      │  │   (5s)       │              │  │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘              │  │
│  └─────────┼──────────────────┼──────────────────┼──────────────────────┘  │
│            │                  │                  │                         │
│            ▼                  ▼                  ▼                         │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                       DEPENDENCY HEALTH CHECKER                       │  │
│  │                                                                       │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────┐ │  │
│  │  │ Database │  │  Redis   │  │ VectorDB │  │ Provider │  │  Bus   │ │  │
│  │  │ Health   │  │  Health  │  │  Health  │  │  Health  │  │ Health │ │  │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └────────┘ │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────┐ │  │
│  │  │Scheduler │  │  Memory  │  │Dashboard │  │  Engine  │  │ Agent  │ │  │
│  │  │ Health   │  │  Health  │  │  Health  │  │  Health  │  │ Health │ │  │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └────────┘ │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                    │                                       │
│                                    ▼                                       │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                         HEALTH AGGREGATOR                             │  │
│  │                                                                       │  │
│  │  Component Statuses → Overall Health Score → Health State             │  │
│  │                                                                       │  │
│  │  Health State: HEALTHY | DEGRADED | UNHEALTHY | CRITICAL              │  │
│  └──────────────────────┬───────────────────────────────────────────────┘  │
│                         │                                                  │
│                         ▼                                                  │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                      CIRCUIT BREAKER MANAGER                          │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────────┐ │  │
│  │  │ Provider   │  │  Engine    │  │  Agent     │  │  Dependency   │ │  │
│  │  │ CB         │  │  CB        │  │  CB        │  │  CB           │ │  │
│  │  └────────────┘  └────────────┘  └────────────┘  └────────────────┘ │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                         │                                                  │
│                         ▼                                                  │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                     ALERTING & NOTIFICATION                           │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────────┐ │  │
│  │  │ Log Alerts │  │ Dashboard  │  │  Publish   │  │  Escalate to   │ │  │
│  │  │            │  │  Alerts    │  │  Event     │  │  On-Call       │ │  │
│  │  └────────────┘  └────────────┘  └────────────┘  └────────────────┘ │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                         HEALTH STORE                                  │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────────┐ │  │
│  │  │  Current   │  │  History   │  │  Incidents │  │  Health Config│ │  │
│  │  │  Status    │  │  (7 days)  │  │  (90 days) │  │  (static)     │ │  │
│  │  └────────────┘  └────────────┘  └────────────┘  └────────────────┘ │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### 8.2 Health Probe Specifications

#### 8.2.1 Readiness Probe

Indicates whether the component is ready to accept requests.

```yaml
readiness_probe:
  purpose: "Determine if component can handle requests"

  endpoint: "/health/ready"
  method: "GET"
  interval: "10s"
  timeout: "5s"
  success_threshold: 1
  failure_threshold: 3

  checks:
    - "Component initialized"
    - "Dependencies available"
    - "Configuration loaded"
    - "Resources allocated"

  response:
    healthy:
      status: 200
      body: { "status": "ready", "component": "name", "latency_ms": 12 }
    unhealthy:
      status: 503
      body: { "status": "not_ready", "component": "name", "reason": "DB not connected" }

  action_on_failure:
    - "Remove from service (load balancer)"
    - "Publish HealthStatusChanged event"
    - "After 3 consecutive failures → escalate"

  aggregation:
    global_readiness: "All critical components ready"
```

#### 8.2.2 Liveness Probe

Indicates whether the component is alive and functioning.

```yaml
liveness_probe:
  purpose: "Detect deadlocked or hung components"

  endpoint: "/health/live"
  method: "GET"
  interval: "30s"
  timeout: "10s"
  success_threshold: 1
  failure_threshold: 3

  checks:
    - "Process responding"
    - "Main loop executing"
    - "No deadlocks detected"
    - "Memory not exhausted"

  response:
    healthy:
      status: 200
      body: { "status": "alive", "component": "name", "uptime_ms": 3600000 }
    unhealthy:
      status: 503
      body: { "status": "dead", "component": "name", "reason": "heap exhausted" }

  action_on_failure:
    - "Restart component"
    - "Preserve queued events"
    - "After 3 consecutive failures → escalate to CTO"
```

#### 8.2.3 Startup Probe

Indicates whether the component has completed initialization.

```yaml
startup_probe:
  purpose: "Determine if component has finished startup"

  endpoint: "/health/startup"
  method: "GET"
  interval: "5s (during startup only)"
  timeout: "5s"
  success_threshold: 1
  failure_threshold: 30  # 150s max startup time

  checks:
    - "Configuration parsed"
    - "Dependencies connected"
    - "Internal state initialized"
    - "First health check passed"

  response:
    started:
      status: 200
      body: { "status": "started", "component": "name", "startup_ms": 45000 }
    not_started:
      status: 503
      body: { "status": "starting", "component": "name", "progress": "75%" }

  action_on_failure:
    - "Retry startup"
    - "After 30 failures → crash loop detection"
    - "Escalate to on-call engineer"
```

---

### 8.3 Dependency Health Matrix

Every Runtime dependency has a defined health check with intervals, thresholds, and failure actions.

| Dependency | Check Type | Interval | Timeout | Success Criteria | Failure Threshold | Circuit Breaker | Fallback |
|-----------|-----------|----------|---------|-----------------|-------------------|----------------|----------|
| **PostgreSQL** | Ping + Query | 15s | 3s | `SELECT 1` < 1s | 3 consecutive failures | Yes (30s open) | Read replica |
| **Redis** | Ping | 10s | 2s | `PING` → `PONG` < 500ms | 3 consecutive failures | Yes (15s open) | Local cache |
| **pgvector** | Ping + Query | 30s | 5s | Query < 3s | 3 consecutive failures | Yes (60s open) | Disable vector search |
| **Event Bus** | Produce + Consume | 10s | 3s | Pub/Sub cycle < 2s | 3 consecutive failures | Yes (30s open) | Local event queue |
| **Provider (Primary)** | Health API | 30s | 10s | Response < 5s | 3 consecutive failures | Yes (60s open) | Secondary provider |
| **Provider (Secondary)** | Health API | 60s | 15s | Response < 10s | 3 consecutive failures | Yes (120s open) | Tertiary provider |
| **Scheduler Queue** | Depth check | 15s | 3s | Depth < 1000 | Depth > 5000 | No (alert only) | Scale workers |
| **Memory Store** | Write + Read | 30s | 5s | Write < 500ms, Read < 200ms | 3 consecutive failures | Yes (30s open) | Secondary store |
| **Dashboard SSE** | Connection check | 30s | 5s | Connection active | Disconnected > 60s | No | Reconnect |
| **File System** | Stat + Write | 60s | 3s | Stat < 100ms, Write < 500ms | 3 consecutive failures | Yes (60s open) | Read-only mode |
| **DNS Resolution** | Lookup | 60s | 2s | Resolution < 1s | 3 consecutive failures | No | Use cached IPs |
| **Container Runtime** | Health API | 30s | 5s | Response < 3s | 3 consecutive failures | Yes (30s open) | Orchestrator restart |

---

### 8.4 Circuit Breaker Configuration

Every circuit breaker follows a formal configuration with thresholds, timeouts, and recovery policies.

#### Circuit Breaker State Machine

```
                    ┌──────────┐
                    │  CLOSED  │  ← Normal operation, requests pass through
                    └────┬─────┘
                         │ failure_threshold exceeded
                         ▼
                    ┌──────────┐
             ┌─────▶│   OPEN   │  ← Requests rejected immediately
             │      └────┬─────┘
             │           │ timeout_duration elapsed
             │           ▼
             │      ┌──────────┐
             │      │HALF_OPEN │  ← Probe requests allowed
             │      └────┬─────┘
             │           │
             ├── success ┤ failure
             │           │
             │           ▼
             │      ┌──────────┐
             └──────│  CLOSED  │  ← Recovery successful
                    └──────────┘
```

#### Circuit Breaker Configuration Table

| Breaker | Scope | failure_threshold | success_threshold | timeout_duration | half_open_max_requests | Metrics | Notification |
|---------|-------|-------------------|-------------------|------------------|------------------------|---------|-------------|
| **Provider Primary** | Per-provider | 5 (in 60s) | 3 (in 30s) | 60s | 2 | provider.cb.state, provider.cb.tripped | PagerDuty |
| **Provider Secondary** | Per-provider | 3 (in 60s) | 2 (in 30s) | 120s | 1 | provider.cb.state, provider.cb.tripped | Email |
| **Database** | Global | 3 (in 30s) | 2 (in 30s) | 30s | 3 | db.cb.state | PagerDuty |
| **Redis** | Global | 3 (in 30s) | 2 (in 30s) | 15s | 3 | redis.cb.state | PagerDuty |
| **Vector Store** | Global | 3 (in 60s) | 2 (in 30s) | 60s | 1 | vector.cb.state | Email |
| **Event Bus** | Global | 3 (in 30s) | 2 (in 15s) | 30s | 2 | bus.cb.state | PagerDuty |
| **Memory Store** | Global | 3 (in 60s) | 2 (in 30s) | 30s | 2 | memory.cb.state | Email |
| **Engine** | Per-engine | 5 (in 120s) | 3 (in 60s) | 60s | 2 | engine.cb.state | Email |
| **Agent** | Per-agent-type | 3 (in 60s) | 2 (in 30s) | 30s | 2 | agent.cb.state | Log only |

#### Circuit Breaker Events

| Event | Trigger | Payload |
|-------|---------|---------|
| `CircuitBreakerOpened` | threshold exceeded | breaker_id, component, error_rate, threshold |
| `CircuitBreakerHalfOpened` | timeout elapsed | breaker_id, component, timeout_duration |
| `CircuitBreakerClosed` | success threshold met | breaker_id, component, recovery_duration_ms |
| `CircuitBreakerTripped` | exceeded half_open max failures | breaker_id, component, total_failures |

---

### 8.5 Health Status Aggregation

Individual component health checks are aggregated into an **overall Runtime Health Score**.

#### Component Health Model

```yaml
component_health:
  component_id: "string"
  component_type: "database | cache | engine | provider | store | bus | agent"

  current_status: "healthy | degraded | unhealthy | unknown"
  previous_status: "healthy | degraded | unhealthy | unknown"

  checks:
    - check_name: "readiness"
      status: "pass | fail"
      latency_ms: 12
      last_checked: "ISO8601"
    - check_name: "liveness"
      status: "pass | fail"
      latency_ms: 8
      last_checked: "ISO8601"

  dependency_health:
    - dependency: "postgresql"
      status: "healthy | degraded | unhealthy"
      latency_ms: 5

  circuit_breaker:
    state: "closed | open | half_open"
    tripped_count: 0
    last_tripped: "ISO8601 | null"

  uptime:
    uptime_percentage: 99.95
    current_session_uptime_ms: 3600000
    last_downtime: "ISO8601 | null"

  metadata:
    version: "1.0.0"
    started_at: "ISO8601"
    last_restart_reason: "string | null"
```

#### Aggregation Rules

```
Aggregation AR-01:
  All critical components HEALTHY → Runtime HEALTHY

Aggregation AR-02:
  Any critical component DEGRADED → Runtime DEGRADED

Aggregation AR-03:
  Any critical component UNHEALTHY → Runtime UNHEALTHY

Aggregation AR-04:
  Multiple critical components UNHEALTHY → Runtime CRITICAL

Aggregation AR-05:
  Non-critical component DEGRADED → Runtime HEALTHY (with warning)

Aggregation AR-06:
  Two or more non-critical components UNHEALTHY → Runtime DEGRADED

Aggregation AR-07:
  Circuit breaker OPEN for any critical dependency → Runtime UNHEALTHY

Aggregation AR-08:
  Recovery in progress → Runtime DEGRADED (with recovery_status)
```

#### Critical vs Non-Critical Components

| Component | Criticality | Impact if Unhealthy |
|-----------|-------------|---------------------|
| PostgreSQL | **CRITICAL** | No persistence, session state lost |
| Redis | **CRITICAL** | No cache, no event bus, state machine degraded |
| Event Bus | **CRITICAL** | No event-driven behavior, no dashboard updates |
| Provider (Primary) | **CRITICAL** | No AI capability, degraded to secondary |
| Scheduler | **CRITICAL** | No task execution |
| Memory Store | **HIGH** | No session memory, degraded context |
| Provider (Secondary) | **HIGH** | No failover AI capability |
| pgvector | **MEDIUM** | No semantic search, keyword fallback |
| Dashboard | **LOW** | No real-time UI, API still works |
| DNS | **LOW** | Cached entries still work |

---

### 8.6 Health Status Change Events

Every health status change generates an event.

```yaml
health_events:
  change_threshold: "Any status change (healthy↔degraded↔unhealthy)"

  event_payload:
    event: "HealthStatusChanged"
    payload:
      component: "postgresql"
      previous_status: "healthy"
      current_status: "degraded"
      reason: "query_latency > 1s"
      latency_ms: 2300
      consecutive_failures: 2

  notification_rules:
    healthy→degraded: "Log warning, publish event"
    healthy→unhealthy: "Log error, publish event, notify on-call"
    degraded→unhealthy: "Log critical, publish event, escalate"
    unhealthy→healthy: "Log info, publish event, close incident"
    degraded→healthy: "Log info, publish event"

  escalation:
    level_1: "3 consecutive unhealthy checks → email to component owner"
    level_2: "5 consecutive unhealthy checks → SMS/PagerDuty to on-call"
    level_3: "10 consecutive unhealthy checks → incident created, CTO notified"
```

---

### 8.7 Recovery Status Tracking

The Health Monitor tracks active and historical recoveries.

```yaml
recovery_status:
  active_recoveries:
    - component: "postgresql"
      recovery_type: "failover_to_replica"
      started_at: "ISO8601"
      duration_ms: 45000
      status: "in_progress | completed | failed"
      checkpoint_id: "uuid"

  recovery_history:
    - component: "redis"
      recovery_type: "restart"
      started_at: "ISO8601"
      completed_at: "ISO8601"
      duration_ms: 12000
      status: "completed"
      success: true

  recovery_metrics:
    - "recovery.count: total recoveries attempted"
    - "recovery.success_rate: successful / total"
    - "recovery.duration_ms: histogram of recovery time"
    - "recovery.last: timestamp of last recovery"
```

---

### 8.8 Failover Status

The Health Monitor tracks failover state and history.

```yaml
failover_status:
  current_failover:
    active: true | false
    from: "primary_provider"
    to: "secondary_provider"
    reason: "latency_threshold_exceeded"
    started_at: "ISO8601"
    duration_ms: 85000

  failover_history:
    - from: "primary_db"
      to: "replica_db"
      reason: "connection_lost"
      triggered_at: "ISO8601"
      resolved_at: "ISO8601"
      duration_ms: 34000
      success: true

  failover_state_machine:
    states: ["ACTIVE", "FAILING_OVER", "FAILED_OVER", "FAILING_BACK", "RESOLVED"]
    transitions:
      - "ACTIVE → FAILING_OVER (on threshold exceeded)"
      - "FAILING_OVER → FAILED_OVER (on failover complete)"
      - "FAILED_OVER → FAILING_BACK (on primary recovery)"
      - "FAILING_BACK → ACTIVE (on failback complete)"
      - "FAILING_OVER → ACTIVE (on failover failure, retry)"
```

---

### 8.9 Resource Health

System resource health is monitored independently.

```yaml
resource_health:
  cpu:
    check_interval: "15s"
    warning_threshold: "70% utilization"
    critical_threshold: "90% utilization"
    action_warning: "Scale up workers"
    action_critical: "Throttle new sessions"

  memory:
    check_interval: "15s"
    warning_threshold: "75% utilization"
    critical_threshold: "90% utilization"
    action_warning: "GC trigger, reduce cache TTL"
    action_critical: "OOM prevention: drop lowest-priority work"

  disk:
    check_interval: "30s"
    warning_threshold: "80% utilization"
    critical_threshold: "95% utilization"
    action_warning: "Archive old data, compress logs"
    action_critical: "Stop non-essential writes, alert on-call"

  network:
    check_interval: "30s"
    warning_threshold: "50% bandwidth utilization"
    critical_threshold: "80% bandwidth utilization"
    action_warning: "Throttle background sync"
    action_critical: "Prioritize essential traffic only"

  file_descriptors:
    check_interval: "30s"
    warning_threshold: "60% of max"
    critical_threshold: "80% of max"
    action_warning: "Investigate leak"
    action_critical: "Restart component"

  goroutines/threads:
    check_interval: "30s"
    warning_threshold: "10000 goroutines"
    critical_threshold: "50000 goroutines"
    action_warning: "Investigate leak"
    action_critical: "Restart component"
```

---

### 8.10 Health Check API

The Health Monitor exposes endpoints for health queries.

#### Endpoints

| Endpoint | Method | Description | Response |
|----------|--------|-------------|----------|
| `/health` | GET | Overall Runtime health | `{ status, score, components }` |
| `/health/ready` | GET | Readiness check | `{ status, checks[] }` |
| `/health/live` | GET | Liveness check | `{ status, uptime_ms }` |
| `/health/startup` | GET | Startup status | `{ status, progress }` |
| `/health/component/{name}` | GET | Specific component health | `{ component, status, checks[] }` |
| `/health/dependencies` | GET | All dependency health | `{ dependencies: { ... } }` |
| `/health/circuit-breakers` | GET | All circuit breaker states | `{ breakers: { ... } }` |
| `/health/resources` | GET | System resource health | `{ cpu, memory, disk, network }` |
| `/health/recovery` | GET | Recovery status | `{ active_recoveries[], history[] }` |
| `/health/failover` | GET | Failover status | `{ current, history[] }` |
| `/health/history` | GET | Health event history (24h) | `{ events: [] }` |

#### Response Schema

```json
{
  "status": "healthy | degraded | unhealthy | critical",
  "score": 98.5,
  "uptime_ms": 86400000,
  "components": {
    "postgresql": { "status": "healthy", "latency_ms": 5, "uptime": 99.99 },
    "redis": { "status": "healthy", "latency_ms": 2, "uptime": 100.0 },
    "provider_primary": { "status": "degraded", "latency_ms": 3500, "uptime": 98.5 }
  },
  "circuit_breakers": {
    "provider_primary": { "state": "closed", "tripped": 0 }
  },
  "resources": {
    "cpu": { "usage_percent": 45, "status": "healthy" },
    "memory": { "usage_percent": 62, "status": "healthy" }
  },
  "last_updated": "ISO8601"
}
```

---

### 8.11 Health Dashboard

```yaml
health_dashboard:
  sections:
    - name: "Overall Health"
      widgets:
        - "Status badge: HEALTHY | DEGRADED | UNHEALTHY | CRITICAL"
        - "Health score gauge: 0-100%"
        - "Uptime: current session + 30-day rolling"

    - name: "Component Health"
      widgets:
        - "Table: Component | Status | Latency | Uptime | CB State"
        - "Color-coded: green=healthy, yellow=degraded, red=unhealthy"
        - "Sort by: status (unhealthy first)"

    - name: "Circuit Breakers"
      widgets:
        - "Table: Breaker | State | Tripped | Last Opened | Recovery ETA"
        - "Alert if any breaker is OPEN for > 5min"

    - name: "Resource Utilization"
      widgets:
        - "Gauges: CPU, Memory, Disk, Network"
        - "Timeline: last hour of resource usage"

    - name: "Recovery & Failover"
      widgets:
        - "Active recoveries: count + details"
        - "Failover status: current + history"
        - "Recovery success rate: pie chart"

    - name: "Health Events Timeline"
      widgets:
        - "Timeline: health status changes over time"
        - "Filter by component, severity, type"
```

---

### 8.12 Health Alerting

#### Alert Rules

| Rule | Condition | Severity | Notification | Cooldown |
|------|-----------|----------|-------------|----------|
| Component unhealthy | Component status = UNHEALTHY | CRITICAL | PagerDuty + SMS + Email | 5 min |
| Component degraded | Component status = DEGRADED > 5 min | WARNING | Email | 15 min |
| Circuit breaker open | CB state = OPEN > 1 min | CRITICAL | PagerDuty | 5 min |
| Resource critical | CPU > 90% or Memory > 90% | CRITICAL | PagerDuty | 5 min |
| Resource warning | CPU > 70% or Memory > 75% | WARNING | Email | 15 min |
| Recovery failed | Recovery status = failed | CRITICAL | PagerDuty + SMS | Immediate |
| Health score drop | Score dropped > 20% in 5 min | WARNING | Email | 15 min |
| Crash loop detected | 5+ restarts in 10 min | CRITICAL | PagerDuty + SMS | Immediate |

#### Notification Channels

| Channel | Priority | Use Case | Rate Limit |
|---------|----------|----------|------------|
| **Dashboard alert banner** | All | Non-critical warnings | Unlimited |
| **Email** | WARNING | Degradations, warnings | 10/hr |
| **SMS** | CRITICAL | Component unhealthy, CB open | 5/hr |
| **PagerDuty** | CRITICAL | Production-impacting issues | On-call rotation |
| **Slack/Teams** | All | All health events (summary) | 1/min |
| **Webhook** | CRITICAL | External integration | Configurable |

---

### 8.13 Health Metrics

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `health.status` | Gauge | component | Current health (1=healthy, 2=degraded, 3=unhealthy, 4=critical) |
| `health.score` | Gauge | — | Overall health score (0-100) |
| `health.check.duration_ms` | Histogram | component, check_type | Health check execution time |
| `health.check.failures` | Counter | component, check_type | Health check failures |
| `health.component.uptime` | Gauge | component | Uptime percentage |
| `health.cb.state` | Gauge | breaker | Circuit breaker state (1=closed, 2=half_open, 3=open) |
| `health.cb.tripped` | Counter | breaker | Circuit breaker trips |
| `health.recovery.count` | Counter | component, type | Recovery attempts |
| `health.recovery.duration_ms` | Histogram | component | Recovery duration |
| `health.recovery.success` | Counter | component | Successful recoveries |
| `health.failover.count` | Counter | from, to | Failover count |
| `health.failover.duration_ms` | Histogram | from, to | Failover duration |
| `health.resource.cpu` | Gauge | — | CPU utilization % |
| `health.resource.memory` | Gauge | — | Memory utilization % |
| `health.resource.disk` | Gauge | — | Disk utilization % |
| `health.alert.count` | Counter | severity, rule | Alert count |
| `health.uptime.total_hours` | Gauge | — | Total uptime in hours |
| `health.consecutive_failures` | Gauge | component | Consecutive check failures |


