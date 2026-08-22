# Execution Graph (DAG)

> **Extracted from**: KERNEL.md section 5 | **Lines**: ~877 | **Date**: 2026-07-28

## Overview

The Runtime plans execution as a Directed Acyclic Graph (DAG), enabling parallel execution of independent tasks while respecting dependencies.

## DAG Structure

```
         ┌──────────┐
         │  START   │
         └────┬─────┘
              │
      ┌───────┼───────┐
      ▼       ▼       ▼
  ┌───────┐ ┌───────┐ ┌───────┐
  │ TaskA │ │ TaskB │ │ TaskC │
  └───┬───┘ └───┬───┘ └───┬───┘
      │         │         │
      └────┬────┘         │
           ▼              │
       ┌───────┐          │
       │ TaskD │◀─────────┘
       └───┬───┘
           │
       ┌───┴───┐
       ▼       ▼
   ┌───────┐ ┌───────┐
   │ TaskE │ │ TaskF │
   └───┬───┘ └───┬───┘
       │         │
       └────┬────┘
            ▼
        ┌───────┐
        │  END  │
        └───────┘
```

## Node Types

| Type | Description | Example |
|------|-------------|---------|
| **Task** | Unit of work | Call AI provider, run tests |
| **Gate** | Quality checkpoint | Code review, security scan |
| **Fork** | Parallel split | Fan-out to multiple agents |
| **Join** | Parallel merge | Wait for all parallel tasks |
| **Condition** | Branch based on result | If tests pass, deploy |

## Execution Rules

1. A node can only start when ALL its predecessors are complete
2. Failed nodes cause downstream nodes to be skipped (unless configured)
3. Maximum parallelism is configurable (default: CPU count)
4. Each node has a timeout (default: 5 minutes)

## Plan Generation

```go
type ExecutionPlan struct {
    Nodes      []PlanNode
    Edges      []PlanEdge
    DAG        *DAG
    Metrics    PlanMetrics
}

type PlanNode struct {
    ID         string
    Type       NodeType
    Capability string
    Timeout    time.Duration
    Retry      RetryPolicy
}
```

## Rollback

If any node fails after partial execution:
1. Completed nodes are marked for rollback
2. Rollback executes in reverse topological order
3. Each node's Rollback() method is called
4. Final state is reported as partially completed
