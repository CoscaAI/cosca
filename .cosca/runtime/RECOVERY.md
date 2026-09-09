# Recovery Engine — Extracted from KERNEL.md §15

> **Source**: KERNEL.md v3.0.1 §15 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 15. RECOVERY ENGINE

The Recovery Engine is the **safety net** of the Runtime. When any component fails — a step, a workflow, a session, a provider, or the Runtime itself — the Recovery Engine determines the optimal recovery strategy, executes it, and validates the result. Every failure is an opportunity to recover. Every recovery is logged, measured, and learned from.

---

### 15.1 Recovery Engine Architecture

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              RECOVERY ENGINE                                 │
│                                                                              │
│  ┌──────────────┐     ┌──────────────────┐     ┌─────────────────────────┐  │
│  │   FAILURE    │────▶│  DECISION ENGINE  │────▶│   STRATEGY EXECUTOR    │  │
│  │   DETECTOR   │     │                   │     │                         │  │
│  │              │     │ • Classify error  │     │ • Retry                 │  │
│  │ • Timeout    │     │ • Check severity  │     │ • Checkpoint Restore    │  │
│  │ • Error code │     │ • Consult history │     │ • Rollback              │  │
│  │ • Health     │     │ • Match strategy  │     │ • Workflow Resume       │  │
│  │ • Heartbeat  │     │ • Validate preconditions│ • Session Resume        │  │
│  └──────────────┘     └──────────────────┘     │ • Snapshot Restore      │  │
│                                                  │ • Memory Recovery       │  │
│  ┌──────────────────────────────────────────┐   │ • Provider Failover     │  │
│  │           VALIDATION ENGINE               │   │ • State Recovery        │  │
│  │  Validates recovery outcome,              │   └─────────────────────────┘  │
│  │  determines success/failure               │                               │
│  └──────────────────────────────────────────┘   ┌─────────────────────────┐  │
│                                                  │     ESCALATION ENGINE   │  │
│  ┌──────────────────────────────────────────┐   │  If all strategies fail │  │
│  │           RECOVERY STORE                  │   │  → Escalate up ladder  │  │
│  │  Persists recovery history,               │   └─────────────────────────┘  │
│  │  checkpoints, snapshots                   │                               │
│  └──────────────────────────────────────────┘                               │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

### 15.2 Recovery Decision Engine

When a failure is detected, the Decision Engine classifies it and selects the optimal recovery strategy.

#### Failure Classification

```yaml
failure_classification:
  dimensions:
    - type: "transient | permanent | unknown"
    - scope: "node | workflow | session | system | provider"
    - severity: "low | medium | high | critical"
    - domain: "execution | quality | memory | provider | state | resource"

  classification_rules:
    timeout:
      type: "transient"
      strategy: "retry"
    rate_limit:
      type: "transient"
      strategy: "retry (with backoff)"
    connection_lost:
      type: "transient"
      strategy: "retry"
    provider_error:
      type: "transient (may be permanent)"
      strategy: "provider_failover"
    data_corruption:
      type: "permanent"
      strategy: "snapshot_restore"
    state_invalid:
      type: "permanent"
      strategy: "state_recovery"
    crash:
      type: "unknown (assume transient)"
      strategy: "session_resume"
    resource_exhausted:
      type: "transient"
      strategy: "retry (with backoff)"
```

#### Strategy Selection Algorithm

```yaml
strategy_selection:
  algorithm: "Priority-based matching with fallback chain"

  steps:
    step_1: "Classify failure (type, scope, severity, domain)"
    step_2: "Match primary strategy from classification rules"
    step_3: "Check preconditions for primary strategy"
    step_4: "If preconditions met → execute primary strategy"
    step_5: "If preconditions NOT met → fallback to next strategy"
    step_6: "If all strategies exhausted → escalate"

  preconditions:
    retry: "retry_count < max_retries AND failure is transient"
    checkpoint_restore: "checkpoint exists AND checkpoint is valid"
    rollback: "rollback target exists AND rollback is safe"
    workflow_resume: "workflow state persisted AND resume point known"
    session_resume: "session state persisted AND within TTL"
    snapshot_restore: "snapshot exists AND snapshot age < max_age"
    memory_recovery: "secondary store available"
    provider_failover: "alternative provider available"
    state_recovery: "state machine definition valid"

  fallback_chain:
    transient: "[retry → checkpoint_restore → workflow_resume → escalate]"
    permanent: "[snapshot_restore → rollback → session_resume → escalate]"
    provider: "[provider_failover → retry → degraded_mode → escalate]"
    unknown: "[retry → session_resume → escalate]"
```

---

### 15.3 Recovery Strategy Contracts

Each of the 9 recovery strategies is fully defined with its protocol, preconditions, execution steps, validation, and escalation path.

#### Strategy 1 — Retry

```yaml
recovery_retry:
  name: "Retry"
  id: "REC-RTRY-001"

  applicability:
    failure_types: ["timeout", "rate_limit", "connection_lost", "transient_error"]
    scope: "node | step"

  preconditions:
    - "Retry count < max_retries"
    - "Failure is classified as transient"
    - "No circuit breaker open for this component"

  config:
    max_retries: 3
    backoff_strategy: "exponential_with_jitter"
    base_delay_ms: 1000
    max_delay_ms: 30000
    multiplier: 2.0
    jitter_factor: 0.1

  protocol:
    step_1: "Increment retry count"
    step_2: "Calculate backoff delay (exponential × jitter)"
    step_3: "Publish StepRetrying event"
    step_4: "Wait for backoff duration"
    step_5: "Re-execute failed operation"
    step_6: "If success → publish step_completed, reset retry count"
    step_7: "If failure → loop to step_1"
    step_8: "If max retries exceeded → permanent failure → escalate"

  backoff_formulas:
    exponential: "delay = base × multiplier^attempt"
    jitter: "delay = delay × (1 + random(-jitter_factor, +jitter_factor))"
    full: "delay = min(base × 2^attempt × (1 + random(-0.1, 0.1)), max_delay)"

  validation:
    - "Operation completed successfully"
    - "Output matches expected schema"
    - "No side effects from failed attempts remain"

  escalation:
    after_max_retries: "→ Checkpoint Restore or Workflow Resume"
```

#### Strategy 2 — Checkpoint Restore

```yaml
recovery_checkpoint:
  name: "Checkpoint Restore"
  id: "REC-CHK-001"

  applicability:
    failure_types: ["step_failure_permanent", "plan_failure", "partial_execution_failure"]
    scope: "workflow | plan"

  preconditions:
    - "Checkpoint exists for failed step/plan"
    - "Checkpoint is valid (hash verified, not expired)"
    - "Checkpoint age < max_checkpoint_age (1 hour)"

  config:
    max_checkpoint_age_ms: 3600000  # 1 hour
    verify_checksum: true

  protocol:
    step_1: "Locate most recent valid checkpoint"
    step_2: "Verify checkpoint integrity (hash + timestamp)"
    step_3: "Restore state from checkpoint"
    step_4: "Validate restored state (schema + consistency)"
    step_5: "Resume execution from checkpoint position"
    step_6: "If restore fails → fallback to Workflow Resume"

  checkpoint_schema:
    id: "uuid"
    timestamp: "ISO8601"
    workflow_id: "uuid"
    step_id: "string"
    state: {}  # Serialized state
    artifacts: {}  # Completed step outputs
    hash: "SHA-256 of state + artifacts"

  validation:
    - "Restored state is internally consistent"
    - "All referenced artifacts exist"
    - "No data loss from checkpoint to failure point"

  escalation:
    on_failure: "→ Workflow Resume"
```

#### Strategy 3 — Rollback

```yaml
recovery_rollback:
  name: "Rollback"
  id: "REC-RB-001"

  applicability:
    failure_types: ["irrecoverable_step_failure", "data_integrity_violation", "contract_violation"]
    scope: "step | workflow | session"

  preconditions:
    - "Rollback target exists (previous consistent state)"
    - "Compensating actions available for each completed step"
    - "Rollback is safe (no irreversible side effects)"

  config:
    compensate_completed_steps: true
    max_rollback_steps: 10
    timeout_ms: 60000

  protocol:
    step_1: "Identify completed steps that must be rolled back"
    step_2: "Order steps in reverse execution order"
    step_3: "For each step (reverse order):"
    step_4: "  Execute compensating action"
    step_5: "  Verify compensation succeeded"
    step_6: "  Mark step as rolled_back"
    step_7: "Restore workflow state to pre-execution"
    step_8: "Publish WorkflowRolledBack event"

  compensating_actions:
    db_write: "DELETE inserted rows or UPDATE to previous values"
    file_write: "Restore from backup or revert changes"
    api_call: "Call undo/revert API endpoint"
    memory_write: "Restore previous memory record"
    state_change: "Revert to previous state machine state"

  validation:
    - "All compensating actions completed successfully"
    - "System state matches pre-execution state"
    - "No orphaned resources"

  escalation:
    on_failure: "→ Session Resume or escalate to CTO"
```

#### Strategy 4 — Workflow Resume

```yaml
recovery_workflow_resume:
  name: "Workflow Resume"
  id: "REC-WFR-001"

  applicability:
    failure_types: ["workflow_interrupted", "runtime_crash", "plan_failure"]
    scope: "workflow"

  preconditions:
    - "Workflow state persisted"
    - "Resume point identified (last completed step)"
    - "Workflow definition unchanged"

  config:
    max_attempts: 3
    resume_timeout_ms: 60000

  protocol:
    step_1: "Load persisted workflow state"
    step_2: "Identify last completed step"
    step_3: "Determine next step to execute"
    step_4: "Recreate execution context"
    step_5: "Resume workflow from next step"
    step_6: "Publish WorkflowResumed event"

  state_persistence:
    storage: "Database (workflow_state table)"
    content: "workflow_id, plan_id, completed_steps[], current_state, variables{}"

  validation:
    - "Workflow resumed from correct position"
    - "No steps executed twice (idempotency check)"
    - "All completed step outputs still valid"

  escalation:
    after_max_attempts: "→ Session Resume"
```

#### Strategy 5 — Session Resume

```yaml
recovery_session_resume:
  name: "Session Resume"
  id: "REC-SSR-001"

  applicability:
    failure_types: ["runtime_crash", "session_timeout", "connection_lost"]
    scope: "session"

  preconditions:
    - "Session state persisted"
    - "Session age < max_session_age (24 hours)"
    - "No concurrent session with same ID"

  config:
    max_session_age_ms: 86400000  # 24 hours
    restore_memory: true
    restore_context: true

  protocol:
    step_1: "Load persisted session state"
    step_2: "Verify session integrity (hash + timestamp)"
    step_3: "Restore session context"
    step_4: "Restore memory context (short + long)"
    step_5: "Restore workflow state (if any active)"
    step_6: "Re-establish provider connections"
    step_7: "Publish SessionResumed event"

  session_persistence:
    storage: "Database (session_state table)"
    content: "session_id, context{}, memory_keys[], active_workflow_ids[], created_at, last_activity"
    retention: "24 hours after last activity"

  validation:
    - "All session context restored correctly"
    - "Memory references valid"
    - "Provider connections healthy"
    - "No data loss since last checkpoint"

  escalation:
    on_failure: "→ New session creation (data loss accepted)"
```

#### Strategy 6 — Snapshot Restore

```yaml
recovery_snapshot:
  name: "Snapshot Restore"
  id: "REC-SNP-001"

  applicability:
    failure_types: ["data_corruption", "systematic_error", "bad_deployment"]
    scope: "system | store"

  preconditions:
    - "Snapshot exists for target scope"
    - "Snapshot integrity verified"
    - "Snapshot age < max_snapshot_age (7 days)"

  config:
    max_snapshot_age_ms: 604800000  # 7 days
    full_restore: true  # If false, selective restore
    verify_after_restore: true

  protocol:
    step_1: "Identify affected stores (DB, Redis, Vector, Files)"
    step_2: "Select appropriate snapshot (latest valid before corruption)"
    step_3: "Stop writes to affected stores"
    step_4: "Restore each store from snapshot"
    step_5: "Verify restored data integrity"
    step_6: "Resume normal operations"
    step_7: "Publish SnapshotRestored event"

  snapshot_artifacts:
    database: "pg_dump or table-level export"
    redis: "RDB snapshot or AOF replay"
    vector: "Index export + metadata"
    files: "tar.gz of cosca/ directory"
    combined: "All-of-the-above with consistency timestamp"

  validation:
    - "All stores restored to consistent point-in-time"
    - "Cross-store references valid"
    - "Data volume matches expected"

  escalation:
    on_failure: "→ Rollback to previous snapshot → escalate to CTO"
```

#### Strategy 7 — Memory Recovery

```yaml
recovery_memory:
  name: "Memory Recovery"
  id: "REC-MEM-001"

  applicability:
    failure_types: ["memory_store_corrupted", "memory_write_failed", "memory_index_corrupt"]
    scope: "memory_store"

  preconditions:
    - "Secondary/fallback memory store available"
    - "Memory metadata (index) intact or recoverable"

  config:
    fallback_store: "local_filesystem | secondary_database"
    rebuild_index: true
    max_recovery_time_ms: 240000

  protocol:
    step_1: "Detect memory store failure"
    step_2: "Switch to fallback store (read-only if needed)"
    step_3: "Rebuild memory index from fallback"
    step_4: "Validate index integrity"
    step_5: "Gradually restore primary store from fallback"
    step_6: "Switch back to primary store"
    step_7: "Publish MemoryRecovered event"

  validation:
    - "Fallback store operational"
    - "Index rebuilt with all entries"
    - "No memory entries lost"

  escalation:
    on_failure: "→ Snapshot Restore"
```

#### Strategy 8 — Provider Failover

```yaml
recovery_provider_failover:
  name: "Provider Failover"
  id: "REC-PRF-001"

  applicability:
    failure_types: ["provider_unavailable", "provider_latency", "provider_error_rate_high"]
    scope: "provider"

  preconditions:
    - "Alternative provider configured and healthy"
    - "Circuit breaker OPEN for current provider"
    - "Failover not attempted within cooldown period"

  config:
    failover_order: "providers[].priority"
    cooldown_ms: 60000  # Min time before failing back
    health_check_after_failover: true

  protocol:
    step_1: "Detect provider failure (timeout/error/health)"
    step_2: "Open circuit breaker for failed provider"
    step_3: "Select next healthy provider from priority list"
    step_4: "Verify provider health (health check API)"
    step_5: "Redirect all requests to new provider"
    step_6: "Publish ProviderFailoverCompleted event"
    step_7: "Monitor new provider health"
    step_8: "After cooldown, attempt failback to primary"

  provider_health_check:
    endpoint: "/health"
    timeout_ms: 10000
    required_status: "healthy"

  validation:
    - "New provider accepting requests"
    - "Latency within acceptable range"
    - "Error rate below threshold"

  escalation:
    on_all_failover_exhausted: "→ Degraded mode (no provider) → alert CTO"
```

#### Strategy 9 — State Recovery

```yaml
recovery_state:
  name: "State Recovery"
  id: "REC-STR-001"

  applicability:
    failure_types: ["state_machine_invalid", "state_corruption", "state_transition_error"]
    scope: "runtime_state_machine"

  preconditions:
    - "State machine definition valid"
    - "Previous valid state known (from persistence)"
    - "Recovery path exists in transition matrix"

  config:
    fallback_state: "BOOTSTRAPPING"
    max_recovery_attempts: 3

  protocol:
    step_1: "Detect invalid state or transition error"
    step_2: "Load last persisted valid state"
    step_3: "Validate loaded state against transition matrix"
    step_4: "If valid → restore to that state"
    step_5: "If invalid → transition to BOOTSTRAPPING (safe fallback)"
    step_6: "Publish StateRecovered event"

  state_persistence:
    storage: "Redis + Database (dual-write)"
    frequency: "On every state transition + every 60s heartbeat"

  validation:
    - "Restored state is valid (exists in S set)"
    - "Restored state is reachable from BOOTSTRAPPING"
    - "No pending transitions in corrupted state"

  escalation:
    on_failure: "→ System restart → escalate to CTO"
```

---

### 15.4 Checkpoint System

#### Checkpoint Lifecycle

```
CREATE → STORE → VERIFY → RESTORE → INVALIDATE

CREATE:     Snapshot of execution state at a specific point
STORE:      Persist to Redis (hot) + Database (durable)
VERIFY:     Check integrity (hash + timestamp)
RESTORE:    Load checkpoint and resume execution
INVALIDATE: Mark as stale when superseded by newer checkpoint
```

#### Checkpoint Creation Policy

```yaml
checkpoint_policy:
  automatic:
    - "On every DAG node completion"
    - "On every workflow step completion"
    - "Every 60 seconds during execution"
    - "Before executing destructive operations"

  manual:
    - "User-initiated checkpoint (via API/CLI)"
    - "Pre-deployment checkpoint"

  retention:
    hot: "Last 10 checkpoints in Redis"
    warm: "Last 100 checkpoints in Database"
    cold: "All checkpoints for 7 days in object storage"

  cleanup:
    trigger: "New checkpoint created"
    action: "Remove checkpoints older than retention period"
```

---

### 15.5 Recovery Events

| Event | Trigger | Payload | Consumers |
|-------|---------|---------|-----------|
| `RecoveryStarted` | Recovery strategy selected | strategy, scope, failure_reason | Dashboard, Observability |
| `RetryAttempt` | Retry executed | attempt, max, delay_ms, step_id | Dashboard, Monitoring |
| `CheckpointRestored` | Checkpoint restored | checkpoint_id, step_id, age_ms | Audit, Dashboard |
| `WorkflowRolledBack` | Rollback completed | workflow_id, steps_rolled_back[], duration_ms | Audit, Dashboard |
| `WorkflowResumed` | Workflow resumed | workflow_id, resume_point, completed_steps | Dashboard |
| `SessionResumed` | Session resumed | session_id, age_ms, data_loss | Dashboard, Memory |
| `SnapshotRestored` | Snapshot restored | snapshot_id, stores[], age_hours | Audit, Dashboard |
| `MemoryRecovered` | Memory store recovered | store_type, fallback_used, entries_recovered | Memory, Dashboard |
| `ProviderFailoverCompleted` | Provider failover | from_provider, to_provider, duration_ms | Dashboard, Audit |
| `ProviderFailbackCompleted` | Provider failback | from_provider, to_provider | Dashboard |
| `StateRecovered` | State machine recovered | from_state, to_state, recovery_path | Kernel, Dashboard |
| `RecoveryEscalated` | All strategies exhausted | failure, strategies_tried[], level | CTO, CEO |
| `RecoveryCompleted` | Recovery successful | strategy, duration_ms, outcome | All |
| `RecoveryFailed` | Recovery unsuccessful | strategy, error, escalation | All |

---

### 15.6 Recovery Metrics

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `recovery.attempts.count` | Counter | strategy, scope | Recovery attempts |
| `recovery.success.count` | Counter | strategy | Successful recoveries |
| `recovery.failure.count` | Counter | strategy, error_type | Failed recoveries |
| `recovery.success.rate` | Gauge | strategy | Success rate (0-1) |
| `recovery.duration_ms` | Histogram | strategy | Recovery duration |
| `recovery.retry.count` | Counter | scope, step_type | Retry count |
| `recovery.checkpoint.created` | Counter | scope | Checkpoints created |
| `recovery.checkpoint.restored` | Counter | scope | Checkpoints restored |
| `recovery.checkpoint.age_ms` | Histogram | scope | Age of restored checkpoint |
| `recovery.rollback.steps` | Histogram | scope | Steps rolled back |
| `recovery.failover.count` | Counter | from, to | Failover count |
| `recovery.failback.count` | Counter | from, to | Failback count |
| `recovery.session.resume.count` | Counter | — | Session resumes |
| `recovery.session.data_loss` | Gauge | — | Data loss indicator (0/1) |

---

### 15.7 Recovery Testing

The Recovery Engine MUST be tested regularly to ensure strategies work when needed.

```yaml
recovery_testing:
  frequency: "Weekly (automated) + Monthly (chaos)"

  test_scenarios:
    - scenario: "Node timeout"
      inject: "Kill execution node process"
      expected: "Retry (3x) → Checkpoint Restore → Workflow Resume"

    - scenario: "Database connection lost"
      inject: "Stop database service"
      expected: "Retry (3x) → Circuit breaker → Read replica failover"

    - scenario: "Provider unavailable"
      inject: "Block provider API"
      expected: "Provider Failover → Secondary → Tertiary → Degraded"

    - scenario: "Runtime crash"
      inject: "Kill Runtime process"
      expected: "Session Resume (within 24h TTL)"

    - scenario: "Memory corruption"
      inject: "Corrupt memory store file"
      expected: "Memory Recovery → Fallback store → Index rebuild"

  validation:
    - "All strategies complete within defined timeout"
    - "No data loss for critical operations"
    - "System returns to consistent state"
    - "Recovery events published correctly"

  reporting:
    output: "RecoveryTestReport"
    metrics: ["success_rate", "duration_p50", "duration_p95", "data_loss"]
```

---

### 15.8 Recovery Configuration

```yaml
recovery_config:
  enabled: true

  default_strategy: "retry"
  max_attempts_per_failure: 10

  retry:
    enabled: true
    max_retries: 3
    backoff: "exponential_with_jitter"
    base_delay_ms: 1000
    max_delay_ms: 30000

  checkpoint:
    enabled: true
    auto_checkpoint_interval_ms: 60000
    retention_hot: 10
    retention_warm: 100
    max_age_ms: 3600000

  rollback:
    enabled: true
    compensate: true
    max_steps: 10

  session_resume:
    enabled: true
    max_age_ms: 86400000
    restore_memory: true

  snapshot:
    enabled: true
    max_age_ms: 604800000
    verify_integrity: true

  provider_failover:
    enabled: true
    cooldown_ms: 60000
    health_check_timeout_ms: 10000

  testing:
    enabled: true
    frequency: "weekly"
    chaos_enabled: false  # Enable with caution
```


