# Scheduler Enterprise — Extracted from KERNEL.md §6

> **Source**: KERNEL.md v3.0.1 §6 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 6. SCHEDULER ENTERPRISE

The Scheduler is the **execution backbone** of the Runtime. It receives DAG nodes from the Execution Graph, routes them through priority-based queues, dispatches them to workers, manages retries and dependencies, and ensures every task is executed with the right priority, at the right time, by the right worker.

---

### 6.1 Scheduler Architecture

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              SCHEDULER                                       │
│                                                                              │
│  ┌──────────┐    ┌──────────────────────────────────────────────────────┐   │
│  │  INPUT   │    │                    DISPATCHER                         │   │
│  │  GATE    │───▶│                                                      │   │
│  │          │    │  ┌──────────┐  ┌──────────┐  ┌──────────┐           │   │
│  │ Validate │    │  │ IMMEDIATE│  │ PRIORITY │  │DEPENDENCY│           │   │
│  │ Classify │    │  │ Queue    │  │ Queue    │  │ Queue    │           │   │
│  │ Prioritize│   │  └────┬─────┘  └────┬─────┘  └────┬─────┘           │   │
│  └──────────┘    │       │             │             │                  │   │
│                  │       ▼             ▼             ▼                  │   │
│                  │  ┌─────────────────────────────────────┐            │   │
│                  │  │         DISPATCH ENGINE              │            │   │
│                  │  │  • Priority arbitration              │            │   │
│                  │  │  • Dependency resolution             │            │   │
│                  │  │  • Resource allocation               │            │   │
│                  │  │  • Worker assignment                 │            │   │
│                  │  └──────────────┬──────────────────────┘            │   │
│                  │                 │                                   │   │
│                  │                 ▼                                   │   │
│                  │  ┌─────────────────────────────────────┐            │   │
│                  │  │           WORKER POOL                │            │   │
│                  │  │  ┌────────┐ ┌────────┐ ┌────────┐  │            │   │
│                  │  │  │Worker 1│ │Worker 2│ │Worker N│  │            │   │
│                  │  │  └────────┘ └────────┘ └────────┘  │            │   │
│                  │  └─────────────────────────────────────┘            │   │
│                  └──────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                    SECONDARY QUEUES                                    │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐             │   │
│  │  │  RETRY   │  │  DELAYED │  │   CRON   │  │BACKGROUND│             │   │
│  │  │  Queue   │  │  Queue   │  │  Queue   │  │ Queue    │             │   │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘             │   │
│  │       │              │             │             │                    │   │
│  │       └──────────────┴─────────────┴─────────────┘                    │   │
│  │                              │                                        │   │
│  │                              ▼                                        │   │
│  │  ┌──────────────────────────────────────────────────────────┐        │   │
│  │  │                    DEAD LETTER QUEUE                      │        │   │
│  │  │  Unprocessable items after all retries exhausted          │        │   │
│  │  └──────────────────────────────────────────────────────────┘        │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                    SCHEDULER STORE                                     │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │   │
│  │  │ Queue State  │  │   History    │  │   Metrics    │               │   │
│  │  │ (Redis)      │  │ (Database)   │  │ (Prometheus) │               │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘               │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

### 6.2 Queue Specifications (Complete)

Each queue has a formal specification with structure, behavior, persistence, and lifecycle.

#### Queue 1 — Immediate Queue

```yaml
queue_immediate:
  name: "Immediate Queue"
  priority: 100
  description: "Highest priority — executes now, blocks until complete"
  
  characteristics:
    concurrency: 10
    persistence: "Memory (no durability needed)"
    ordering: "FIFO within priority"
    preemption: "Can preempt any other queue"
    
  item_schema:
    id: "uuid"
    type: "critical_request | error | escalation"
    payload: {}
    priority: 100
    submitted_at: "ISO8601"
    
  lifecycle:
    - "Submitted by: Kernel, Error Handler, Escalation Engine"
    - "Dispatched to: First available worker"
    - "On success: Log, publish SchedulerTaskCompleted"
    - "On failure: Immediate retry (max 1), then escalate"
    - "Completion: Remove from queue, record in history"
    
  backpressure:
    max_depth: 100
    on_full: "Block caller (backpressure), publish warning"
    
  uses: ["user-facing requests", "error handling", "escalation"]
```

#### Queue 2 — Priority Queue

```yaml
queue_priority:
  name: "Priority Queue"
  priority_range: "50-99"
  description: "Normal execution, priority-sorted — feature work, reviews"
  
  characteristics:
    concurrency: 5
    persistence: "Redis"
    ordering: "Priority (descending) → FIFO within same priority"
    preemption: "Not preempted by background/delayed"
    
  item_schema:
    id: "uuid"
    type: "feature | bug | refactor | review"
    payload: {}
    priority: 50-99
    submitted_at: "ISO8601"
    timeout_ms: 300000
    
  priority_levels:
    critical: { value: 99, label: "Critical feature/bug" }
    high: { value: 75, label: "High-priority work" }
    normal: { value: 50, label: "Standard priority" }
    
  lifecycle:
    - "Submitted by: Kernel after capability resolution"
    - "Sorted by: Priority (desc), then submission time (asc)"
    - "Dispatched to: Worker with capacity"
    - "On success: Log, publish SchedulerTaskCompleted"
    - "On failure: Move to Retry Queue"
    - "On timeout: Move to Retry Queue (if retries remain)"
    
  backpressure:
    max_depth: 1000
    on_warning: depth > 500 → scale workers
    on_critical: depth > 800 → throttle non-critical submissions
    
  uses: ["feature development", "bug fixes", "refactoring", "reviews"]
```

#### Queue 3 — Dependency Queue

```yaml
queue_dependency:
  name: "Dependency Queue"
  priority_range: "0-49"
  description: "Steps waiting for prerequisites to complete"
  
  characteristics:
    concurrency: "Dependent on dependency resolution"
    persistence: "Database"
    ordering: "By dependency DAG (topological order)"
    blocking: "Items are BLOCKED until all dependencies resolve"
    
  item_schema:
    id: "uuid"
    type: "blocked_step"
    payload: {}
    dependencies: ["dep-1", "dep-2"]
    blocking_count: 2
    resolved_count: 0
    submitted_at: "ISO8601"
    
  lifecycle:
    - "Submitted by: DAG Planner for blocked nodes"
    - "Dependencies monitored: On every dependency completion"
    - "Resolution check: When all dependencies = COMPLETED"
    - "On all resolved: Move to Priority Queue (with original priority)"
    - "On dependency failure: Evaluate cascading failure rules"
    - "On timeout: Escalate, notify CTO"
    
  dependency_resolution:
    check_frequency: "on_dependency_completion_event"
    resolve_action: "Move item to Priority Queue"
    fail_action: "If dependency failed permanently → fail this item too"
    
  uses: ["DAG blocked nodes", "inter-workflow dependencies"]
```

#### Queue 4 — Retry Queue

```yaml
queue_retry:
  name: "Retry Queue"
  description: "Failed steps awaiting retry with configurable backoff"
  
  characteristics:
    concurrency: 3
    persistence: "Redis"
    ordering: "By next_retry_at (ascending)"
    
  item_schema:
    id: "uuid"
    original_queue: "priority | dependency | delayed"
    payload: {}
    retry_count: 0
    max_retries: 3
    last_error: ""
    backoff_ms: 5000
    strategy: "exponential | linear | immediate"
    next_retry_at: "ISO8601"
    
  backoff_strategies:
    exponential:
      formula: "delay = base × 2^attempt"
      example: "5s → 10s → 20s"
    linear:
      formula: "delay = base × (attempt + 1)"
      example: "5s → 10s → 15s"
    immediate:
      formula: "delay = 0"
      note: "Use with caution, only for idempotent operations"
    jitter:
      formula: "delay × (1 + random(-0.1, 0.1))"
      note: "Added to any strategy to prevent thundering herd"
      
  lifecycle:
    - "Submitted by: Priority/Delayed Queue on failure"
    - "Waiting: Until next_retry_at timestamp"
    - "Ready: Moves to Priority Queue at specified time"
    - "On success: Remove from retry tracking"
    - "On max retries: Move to Dead Letter Queue"
    - "On max_retries exceeded: Publish TaskFailed permanently"
    
  uses: ["failed steps", "transient errors", "timeout recovery"]
```

#### Queue 5 — Delayed Queue

```yaml
queue_delayed:
  name: "Delayed Queue"
  description: "Steps scheduled for future execution"
  
  characteristics:
    concurrency: 5
    persistence: "Database"
    ordering: "By scheduled_at (ascending)"
    accuracy: "±1s"
    
  item_schema:
    id: "uuid"
    type: "deferred_task"
    payload: {}
    scheduled_at: "ISO8601"
    target_queue: "priority | immediate"
    priority: 50
    submitted_at: "ISO8601"
    
  lifecycle:
    - "Submitted by: User, Planning Engine, Scheduler API"
    - "Waiting: Until scheduled_at timestamp"
    - "Ready: Moves to target_queue at scheduled time"
    - "On past due: Execute immediately"
    - "On cancel: Remove from queue, publish TaskCancelled"
    
  timer_wheel:
    resolution_ms: 1000  # Checks every second
    max_delay_days: 365
    
  uses: ["future-dated tasks", "maintenance windows", "deferred execution"]
```

#### Queue 6 — Cron Queue

```yaml
queue_cron:
  name: "Cron Queue"
  description: "Recurring scheduled tasks"
  
  characteristics:
    concurrency: 3
    persistence: "Database"
    scheduling: "cron expressions (5-field standard)"
    
  item_schema:
    id: "uuid"
    name: "task-name"
    cron_expression: "0 0 * * *"
    timezone: "UTC"
    payload: {}
    target_queue: "priority | background"
    priority: 50
    last_run: "ISO8601 | null"
    next_run: "ISO8601"
    enabled: true
    
  cron_format:
    standard: "minute hour day month weekday"
    examples:
      daily_midnight: "0 0 * * *"
      hourly: "0 * * * *"
      weekly_monday: "0 0 * * 1"
      month_end: "0 0 28-31 * *"
      custom: "*/15 * * * *"  # Every 15 minutes
      
  lifecycle:
    - "Registered by: User, System, Evolution Engine"
    - "Evaluation: Every minute (cron daemon)"
    - "When due: Create task from template, submit to target_queue"
    - "On completion: Update last_run, calculate next_run"
    - "On failure: Log, retry (max 3), then skip this occurrence"
    - "On disable: Stop scheduling, retain configuration"
    
  built_in_crons:
    - name: "knowledge_evolution"
      cron: "0 2 * * *"  # Daily at 2am
      task: "evolution_engine_scan"
    - name: "memory_maintenance"
      cron: "0 3 * * 0"  # Weekly Sunday 3am
      task: "memory_prune"
    - name: "metrics_report"
      cron: "0 7 * * 1"  # Weekly Monday 7am
      task: "generate_metrics_report"
    - name: "health_check_deep"
      cron: "0 */6 * * *"  # Every 6 hours
      task: "deep_health_check"
    - name: "backup"
      cron: "0 4 * * *"  # Daily at 4am
      task: "create_snapshot"
      
  uses: ["scheduled maintenance", "periodic reports", "health checks", "backup"]
```

#### Queue 7 — Background Queue

```yaml
queue_background:
  name: "Background Queue"
  priority: 0
  description: "Low-priority, non-urgent processing"
  
  characteristics:
    concurrency: 10
    persistence: "Redis"
    ordering: "FIFO"
    preemptable: true  # Can be preempted by any other queue
    
  item_schema:
    id: "uuid"
    type: "learning | sync | metric | cleanup"
    payload: {}
    submitted_at: "ISO8601"
    timeout_ms: 600000  # 10 minutes
    
  lifecycle:
    - "Submitted by: Learning Engine, Sync Engine, Evolution Engine"
    - "Dispatched to: Idle workers only"
    - "On success: Log"
    - "On failure: Retry (max 1), then skip"
    - "On preemption: Re-queue (will be retried)"
    
  backpressure:
    max_depth: 5000
    on_full: "Drop oldest items (LIFO drop)"
    
  uses: ["learning", "knowledge sync", "metrics collection", "cleanup", "log rotation"]
```

---

### 6.3 Dead Letter Queue

Items that exhaust all retries are moved to the Dead Letter Queue for inspection and manual handling.

```yaml
dead_letter_queue:
  description: "Items that failed permanently after all retries exhausted"
  
  storage: "Database (persistent, append-only)"
  retention: "30 days"
  max_items: 10000
  
  item_schema:
    id: "uuid"
    original_queue: "string"
    payload: {}
    retry_history: [{attempt, timestamp, error}]
    final_error: ""
    moved_at: "ISO8601"
    status: "pending_review | reviewed | discarded | replayed"
    
  operations:
    inspect: "View DLQ items with full retry history"
    replay: "Re-submit item to original queue"
    discard: "Remove from DLQ permanently"
    replay_all: "Re-submit all items matching filter"
    
  alert_rules:
    - "DLQ count > 100 → Warning notification"
    - "DLQ count > 500 → Critical notification"
    - "DLQ growth rate > 10/hour → Incident creation"
```

---

### 6.4 Dispatch Algorithm

The Dispatcher selects which item to execute next from all queues.

```yaml
dispatch_algorithm:
  name: "Priority-weighted fair scheduling"
  tick_rate_ms: 100  # 10 evaluations per second
  
  algorithm:
    step_1: "Collect ready items from all queues"
    step_2: "Filter: only items with all dependencies resolved"
    step_3: "Filter: only items within resource limits"
    step_4: "Score each item: priority × urgency_factor"
    step_5: "Sort by score (descending)"
    step_6: "Select top N items (N = available workers)"
    step_7: "Assign to workers"
    
  urgency_factor:
    description: "Increases score as item approaches deadline"
    formula: "1 + (wait_time_ms / max_wait_ms)"
    max_factor: 2.0
    
  starvation_prevention:
    mechanism: "Aging"
    rule: "Every 60s in queue → priority +1 (max +10)"
    exception: "Immediate queue (already highest priority)"
    
  worker_assignment:
    strategy: "Least-loaded-first"
    check: "Worker must have capacity for item's resource_profile"
```

---

### 6.5 Worker Management

```yaml
worker_pool:
  min_workers: 2
  max_workers: 20
  scaling: "auto (based on queue depth)"
  
  scaling_rules:
    scale_up:
      trigger: "queue_depth > 500 for > 30s"
      increment: 2
      max: 20
      cooldown_ms: 60000
      
    scale_down:
      trigger: "queue_depth < 50 for > 120s"
      decrement: 1
      min: 2
      cooldown_ms: 240000
      
  worker_schema:
    id: "uuid"
    status: "idle | busy | draining | dead"
    current_item: "uuid | null"
    started_at: "ISO8601"
    items_processed: 0
    resource_profile: { cpu: "low", memory: "medium" }
    
  worker_health:
    check_interval: 5s
    heartbeat_timeout: 15s
    on_death: "Re-queue current item, spawn replacement"
    
  worker_types:
    chief_worker: "Executes capability nodes (max 5)"
    engine_worker: "Executes engine nodes (max 10)"
    specialist_worker: "Executes task nodes (max 20)"
    system_worker: "Executes system operations (max 5)"
```

---

### 6.6 Scheduler Events

| Event | Trigger | Payload | Consumers |
|-------|---------|---------|-----------|
| `SchedulerTaskEnqueued` | Item added to queue | queue, item_id, priority, type | Dashboard, Observability |
| `SchedulerTaskDequeued` | Item removed from queue | queue, item_id, reason | Dashboard, Observability |
| `SchedulerTaskDispatched` | Item sent to worker | queue, item_id, worker_id | Dashboard, Trace |
| `SchedulerTaskCompleted` | Worker finished | queue, item_id, duration_ms, success | Dashboard, Orchestrator |
| `SchedulerTaskFailed` | Worker returned error | queue, item_id, error, attempt | Retry Queue, Alerting |
| `SchedulerTaskRetrying` | Item moved to Retry Queue | queue, item_id, attempt, backoff_ms | Dashboard |
| `SchedulerTaskDeadLetter` | Item sent to DLQ | queue, item_id, retry_history[] | Alerting, Admin |
| `SchedulerQueueDepthWarning` | Queue depth > threshold | queue, depth, threshold | Auto-scaler, Alerting |
| `SchedulerWorkerAdded` | New worker spawned | worker_id, type | Dashboard |
| `SchedulerWorkerRemoved` | Worker terminated | worker_id, reason | Dashboard |
| `SchedulerWorkerHeartbeatTimeout` | Worker heartbeat missed | worker_id, last_heartbeat | Recovery Engine |
| `SchedulerCronTriggered` | Cron task due | cron_id, task_name, scheduled_at | Dashboard |

---

### 6.7 Scheduler Metrics

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `scheduler.queue.depth` | Gauge | queue | Current queue depth |
| `scheduler.queue.enqueued` | Counter | queue | Items enqueued |
| `scheduler.queue.dequeued` | Counter | queue | Items dequeued |
| `scheduler.queue.wait_time_ms` | Histogram | queue, priority | Time in queue |
| `scheduler.dispatch.count` | Counter | queue, worker_type | Items dispatched |
| `scheduler.dispatch.latency_ms` | Histogram | queue | Time to dispatch |
| `scheduler.completion.count` | Counter | queue, status | Items completed |
| `scheduler.failure.count` | Counter | queue, error_type | Items failed |
| `scheduler.retry.count` | Counter | queue | Retry attempts |
| `scheduler.retry.duration_ms` | Histogram | queue | Time in retry queue |
| `scheduler.dlq.count` | Gauge | — | Dead letter queue size |
| `scheduler.worker.active` | Gauge | worker_type | Active workers |
| `scheduler.worker.idle` | Gauge | worker_type | Idle workers |
| `scheduler.worker.utilization` | Gauge | worker_type | Worker utilization % |
| `scheduler.throughput` | Gauge | queue | Items per second |
| `scheduler.cron.triggered` | Counter | cron_id | Cron task triggers |
| `scheduler.backpressure.level` | Gauge | — | Current backpressure level |

---

### 6.8 Scheduler Health

```yaml
scheduler_health:
  readiness:
    - "All queues initialized"
    - "Worker pool has at least min_workers"
    - "Queue storage accessible"
    
  liveness:
    - "Dispatch loop running"
    - "Queue depth not growing unbounded"
    - "Workers reporting heartbeat"
    
  degraded:
    - "Any queue depth > 80% of max → WARNING"
    - "Worker utilization > 90% for > 5min → WARNING"
    - "DLQ count > 100 → WARNING"
```

---

### 6.9 Scheduler Configuration

```yaml
scheduler_config:
  dispatch:
    tick_rate_ms: 100
    max_items_per_tick: 50
    
  queues:
    immediate:
      concurrency: 10
      max_depth: 100
      
    priority:
      concurrency: 5
      max_depth: 1000
      aging_increment: 1
      aging_interval_s: 60
      
    dependency:
      check_on_completion: true
      
    retry:
      concurrency: 3
      default_max_retries: 3
      default_backoff_ms: 5000
      default_strategy: "exponential"
      
    delayed:
      concurrency: 5
      timer_resolution_ms: 1000
      
    cron:
      concurrency: 3
      evaluation_interval_s: 60
      
    background:
      concurrency: 10
      max_depth: 5000
      
  dead_letter:
    retention_days: 30
    alert_threshold: 100
    
  workers:
    min: 2
    max: 20
    auto_scaling: true
    scale_up_threshold: 500
    scale_down_threshold: 50
    heartbeat_timeout_ms: 15000
```



