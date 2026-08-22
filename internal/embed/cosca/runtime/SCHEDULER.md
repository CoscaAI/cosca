# Scheduler Enterprise

> **Extracted from**: KERNEL.md section 6 | **Lines**: ~603 | **Date**: 2026-07-28

## Overview

The Scheduler manages task queuing, prioritization, and dispatch. It ensures fair resource allocation and prevents starvation.

## Queue Types

| Queue | Priority | Purpose |
|-------|----------|---------|
| **Critical** | 100 | Security incidents, data loss prevention |
| **High** | 75 | User-facing requests, API calls |
| **Normal** | 50 | Background tasks, workflows |
| **Low** | 25 | Maintenance, cleanup, indexing |
| **Idle** | 0 | Best-effort, when resources available |

## Scheduling Algorithm

Weighted Fair Queuing (WFQ) with aging:

1. Each task gets weight based on priority
2. Tasks are served in weighted round-robin order
3. Older tasks get aging bonus (prevents starvation)
4. Resource limits per agent (max concurrent tasks)

## Worker Pool

```go
type WorkerPool struct {
    workers    []Worker
    maxWorkers int
    taskQueue  chan Task
    metrics    PoolMetrics
}
```

- **Default workers**: CPU count
- **Max concurrent per agent**: 3
- **Task timeout**: 5 minutes (configurable)
- **Idle timeout**: 30 seconds (worker released)

## Priority Inversion Prevention

If a high-priority task is blocked by a low-priority task:
1. Detect dependency chain
2. Boost low-priority task temporarily
3. Complete chain, restore original priority

## Metrics

- Queue depth per priority level
- Average wait time per priority
- Worker utilization percentage
- Task completion rate
- Timeout rate
