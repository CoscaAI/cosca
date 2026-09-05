# Runtime Metrics

> **Extracted from**: KERNEL.md section 9 | **Lines**: ~556 | **Date**: 2026-07-28

## Metric Types

| Type | Description | Example |
|------|-------------|---------|
| Counter | Monotonically increasing | requests_total |
| Gauge | Can go up/down | queue_depth |
| Histogram | Distribution of values | request_duration |
| Summary | Pre-computed quantiles | latency_p99 |

## Core Metrics

### API Metrics
- cosca_api_requests_total
- cosca_api_request_duration_seconds
- cosca_api_errors_total

### Provider Metrics
- cosca_provider_calls_total
- cosca_provider_duration_seconds
- cosca_provider_errors_total
- cosca_provider_tokens_used

### Memory Metrics
- cosca_memory_stored_total
- cosca_memory_size_bytes
- cosca_memory_pruned_total

### Knowledge Metrics
- cosca_knowledge_items_total
- cosca_knowledge_index_size
- cosca_knowledge_search_duration

### Runtime Metrics
- cosca_runtime_tasks_active
- cosca_runtime_tasks_completed_total
- cosca_runtime_goroutines
- cosca_runtime_memory_bytes

## Export Formats

| Format | Endpoint | Use Case |
|--------|----------|----------|
| Prometheus | /metrics | Grafana, monitoring |
| JSON | /metrics/json | Programmatic access |
| OpenTelemetry | OTLP gRPC | Distributed tracing |

## Labels

All metrics support standard labels:
- instance: Runtime instance ID
- version: Cosca version
- mode: development/production/test
