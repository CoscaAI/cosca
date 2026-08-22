# Runtime Health

> **Extracted from**: KERNEL.md section 8 | **Lines**: ~641 | **Date**: 2026-07-28

## Health Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| /healthz | GET | Liveness probe |
| /readyz | GET | Readiness probe |
| /healthz/live | GET | Container liveness |
| /healthz/ready | GET | Container readiness |
| /healthz/startup | GET | Startup probe |

## Health Status

```go
type HealthStatus struct {
    Status    string            // healthy, degraded, unhealthy
    Components map[string]ComponentHealth
    Timestamp time.Time
}

type ComponentHealth struct {
    Status    string
    Message   string
    Latency   time.Duration
    LastCheck time.Time
}
```

## Components Monitored

| Component | Check | Interval |
|-----------|-------|----------|
| SQLite DB | PRAGMA integrity_check | 5min |
| Knowledge DB | Query test | 1min |
| Provider Ollama | /api/tags | 30s |
| Memory System | Stats query | 1min |
| Event Bus | Buffer status | 30s |
| Scheduler | Queue depth | 30s |

## Degradation Levels

| Level | Status | Behavior |
|-------|--------|----------|
| 0 | healthy | All systems operational |
| 1 | degraded | Non-critical component down |
| 2 | degraded | Multiple components affected |
| 3 | unhealthy | Critical component down |

## Recovery Actions

| Component | Failure | Auto-Recovery |
|-----------|---------|---------------|
| Provider | Timeout | Retry with backoff |
| DB | Lock | Wait + retry |
| Memory | Full | Prune old entries |
| Event Bus | Full | Drop oldest events |
