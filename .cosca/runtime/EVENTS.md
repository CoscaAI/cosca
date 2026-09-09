# Event Driven Architecture — Extracted from KERNEL.md §4

> **Source**: KERNEL.md v3.0.1 §4 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 4. EVENT DRIVEN ARCHITECTURE

Every behavior in the Kernel generates events. **Nothing happens without events.** All events are published to the **Event Bus**. The Event Bus is the central nervous system of the Runtime — all communication between components, layers, and consumers flows through it.

---

### 4.1 Event Bus Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                           EVENT BUS                                       │
│                                                                          │
│  ┌──────────────┐  ┌──────────────────────┐  ┌──────────────────────┐   │
│  │  PUBLISHERS  │  │       BROKER          │  │     CONSUMERS        │   │
│  │              │  │                       │  │                      │   │
│  │ • Kernel     │──▶│  ┌────────────────┐  │──▶│ • Observability     │   │
│  │ • Bootstrap  │  │  │    TOPICS       │  │  │ • Audit Trail       │   │
│  │ • Discovery  │  │  │                │  │  │ • Dashboard (SSE)   │   │
│  │ • Context    │  │  │ session/       │  │  │ • Memory Engine     │   │
│  │ • Memory     │  │  │ workflow/      │  │  │ • Learning Engine   │   │
│  │ • Capability │  │  │ capability/    │  │  │ • Evolution Engine  │   │
│  │ • Workflow   │  │  │ quality/       │  │  │ • Recovery Engine   │   │
│  │ • Planning   │  │  │ system/        │  │  │ • Provider Failover │   │
│  │ • Execution  │  │  │ health/        │  │  │ • Plugin System     │   │
│  │ • Review     │  │  │ governance/    │  │  │ • SDK Webhooks      │   │
│  │ • Quality    │  │  └────────────────┘  │  │ • CLI Subscribers   │   │
│  │ • Health     │  │  ┌────────────────┐  │  │ • API Consumers     │   │
│  │ • Recovery   │  │  │    QUEUES      │  │  │                     │   │
│  │ • Providers  │  │  │ priority/      │  │  │                     │   │
│  │ • Plugins    │  │  │ retry/         │  │  │                     │   │
│  │ • SDK        │  │  │ dead-letter/   │  │  │                     │   │
│  └──────────────┘  │  └────────────────┘  │  └──────────────────────┘   │
│                    └──────────────────────┘                              │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                    EVENT STORE (Persistence Layer)                │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────────┐  │   │
│  │  │ Redis    │  │  DB      │  │  S3/GCS  │  │ Schema Registry│  │   │
│  │  │ (recent) │  │ (active) │  │ (archive)│  │ (schemas)      │  │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └────────────────┘  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

---

### 4.2 Event Bus Topology

#### Topic Hierarchy

Events are organized into a hierarchical topic structure:

```
events/v1/{domain}/{entity}/{action}
```

| Topic Pattern | Example | Description |
|--------------|---------|-------------|
| `events/v1/session/{id}/started` | Session started | Session lifecycle |
| `events/v1/workflow/{id}/created` | Workflow created | Workflow lifecycle |
| `events/v1/capability/{id}/resolved` | Capability resolved | Capability operations |
| `events/v1/quality/gate/{n}/passed` | Gate 2 passed | Quality lifecycle |
| `events/v1/system/health/{component}` | DB health check | System operations |
| `events/v1/governance/decision/{id}` | ADR created | Governance operations |

#### Topic Partitioning

| Topic | Partitions | Routing Key | Ordering |
|-------|-----------|-------------|----------|
| `session/` | 4 (by session_id hash) | `session_id` | Per-session ordering guaranteed |
| `workflow/` | 4 (by workflow_id hash) | `workflow_id` | Per-workflow ordering guaranteed |
| `capability/` | 2 (by capability_id hash) | `capability_id` | Best-effort |
| `quality/` | 2 | `gate_id` | Best-effort |
| `system/` | 1 | `component` | Global ordering |
| `health/` | 1 | `component` | Global ordering |
| `governance/` | 1 | `decision_id` | Global ordering |

---

### 4.3 Official Event Catalog

#### Session Lifecycle Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `SessionStarted` | `{ session_id, timestamp, runtime_type, workspace_path }` | Kernel | All | At-least-once |
| `BootstrapStarted` | `{ session_id, config_hash, provider_count }` | Kernel | Observability, Audit | At-least-once |
| `BootstrapCompleted` | `{ session_id, duration_ms, config, resolved_paths }` | Kernel | Observability, Memory | At-least-once |
| `SessionFinished` | `{ session_id, summary, duration_ms, quality_score, memory_updates[] }` | Kernel | Memory, Learning, Evolution | Exactly-once |

#### Discovery & Context Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `DiscoveryStarted` | `{ session_id, workspace_path, scan_depth }` | Discovery Engine | Observability, Audit | At-least-once |
| `DiscoveryCompleted` | `{ session_id, framework, language, database, dependencies[], build_system, test_framework, ci_cd, architecture_pattern, duration_ms }` | Discovery Engine | Context Engine, Memory | At-least-once |
| `ContextLoaded` | `{ session_id, context_size_bytes, memory_count, stores_loaded[] }` | Context Engine | Kernel, Memory | At-least-once |
| `MemoryLoaded` | `{ session_id, stores_loaded[], total_entries, duration_ms }` | Memory Engine | Kernel, Context | At-least-once |

#### Capability & Workflow Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `CapabilityResolved` | `{ session_id, request_type, capabilities[{id, confidence, provider}], resolution_duration_ms }` | Capability Engine | Workflow Engine, Audit | At-least-once |
| `CapabilityRegistered` | `{ capability_id, name, version, provider, category }` | Capability Engine | COSCA_INDEX, Registry | Exactly-once |
| `CapabilityDeprecated` | `{ capability_id, replacement_id, sunset_date, migration_path }` | Capability Engine | Registry, Consumers | Exactly-once |
| `WorkflowResolved` | `{ session_id, workflow_id, steps[], composition_pattern, estimated_duration_ms }` | Workflow Engine | Planning Engine, Audit | At-least-once |
| `WorkflowRegistered` | `{ workflow_id, steps[], category, owner }` | Workflow Engine | COSCA_INDEX, Registry | Exactly-once |

#### Planning & Execution Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `PlanCreated` | `{ session_id, plan_id, dag_nodes[], dag_edges[], risk_assessment, success_criteria[] }` | Planning Engine | Scheduler, Audit | At-least-once |
| `PlanUpdated` | `{ session_id, plan_id, updated_nodes[], updated_edges[], reason }` | Planning Engine | Scheduler, Audit | At-least-once |
| `PlanApproved` | `{ session_id, plan_id, approved_by, approval_timestamp }` | CTO/Architecture | Execution, Audit | Exactly-once |
| `ExecutionStarted` | `{ session_id, plan_id, steps_total, parallel_groups, scheduled_at }` | Execution Engine | Observability, Dashboard | At-least-once |
| `ExecutionCompleted` | `{ session_id, plan_id, step_results[], total_duration_ms, retry_count }` | Execution Engine | Review Engine, Observability | At-least-once |
| `ExecutionFailed` | `{ session_id, plan_id, failed_step, error, attempts, recovery_action }` | Execution Engine | Recovery Engine, Observability | At-least-once |
| `StepStarted` | `{ session_id, plan_id, step_id, capability, provider, started_at }` | Execution Engine | Dashboard, Observability | At-least-once |
| `StepCompleted` | `{ session_id, plan_id, step_id, output, duration_ms, quality_score }` | Execution Engine | Orchestrator, Dashboard | At-least-once |
| `StepRetrying` | `{ session_id, plan_id, step_id, attempt, max_retries, backoff_ms, error }` | Execution Engine | Observability, Recovery | At-least-once |

#### Review & Quality Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `ReviewStarted` | `{ session_id, artifact_id, review_type, reviewer, scope }` | Review Engine | Observability, Audit | At-least-once |
| `ReviewCompleted` | `{ session_id, artifact_id, score, issues[{severity, description, location}], passed }` | Review Engine | QA Chief, Memory | At-least-once |
| `ReviewFixRequested` | `{ session_id, artifact_id, fix_instructions[], deadline, reviewer }` | Review Engine | Execution, Author | At-least-once |
| `QualityPassed` | `{ session_id, gate_id, score, checks_passed[], quality_score }` | Quality Engine | Release Chief, Dashboard | Exactly-once |
| `QualityFailed` | `{ session_id, gate_id, score, checks_failed[], blocking_issues[], escalation }` | Quality Engine | Kernel, Review Engine | Exactly-once |
| `QualityGateStarted` | `{ session_id, gate_id, checks[], started_at }` | Quality Engine | Dashboard, Observability | At-least-once |
| `BenchmarkCompleted` | `{ session_id, benchmark_id, scores{}, percentile_rank, duration_ms }` | Benchmark Engine | Knowledge, Dashboard | At-least-once |

#### Documentation & Knowledge Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `DocumentationStarted` | `{ session_id, docs_types[] }` | Documentation Engine | Observability | At-least-once |
| `DocumentationUpdated` | `{ session_id, docs_updated[{type, path, version}], duration_ms }` | Documentation Engine | Memory, Knowledge | At-least-once |
| `KnowledgeStored` | `{ session_id, store_type, entry_count, embedding_generated, duration_ms }` | Knowledge Engine | Memory, Learning Engine | At-least-once |
| `KnowledgeSearched` | `{ session_id, query, result_count, search_type, duration_ms }` | Knowledge Engine | Observability | Best-effort |
| `MemoryUpdated` | `{ session_id, store, action, key, tags[], size_bytes }` | Memory Engine | Context Engine, Dashboard | At-least-once |
| `MemoryPromoted` | `{ session_id, key, from_store, to_store, reason }` | Memory Engine | Learning Engine | At-least-once |

#### Learning & Evolution Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `LearningStarted` | `{ session_id, session_summary, pattern_count }` | Learning Engine | Observability | At-least-once |
| `LearningCompleted` | `{ session_id, patterns_extracted[], agent_memory_updated, knowledge_stored[] }` | Learning Engine | Memory, Evolution | At-least-once |
| `EvolutionTriggered` | `{ trigger_type, scope, last_run, reason }` | Evolution Engine | Knowledge, Analytics | At-least-once |
| `EvolutionCompleted` | `{ session_id, improvements[], deprecated_items[], new_patterns[], duration_ms }` | Evolution Engine | Memory, Governance | At-least-once |

#### Health & System Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `HealthStatusChanged` | `{ session_id, component, status{healthy, degraded, down}, previous_status, message, duration_ms }` | Health Monitor | Dashboard, Alerting | At-least-once |
| `HealthCheckPassed` | `{ session_id, component, check_type, latency_ms }` | Health Monitor | Observability | Best-effort |
| `HealthCheckFailed` | `{ session_id, component, check_type, error, consecutive_failures }` | Health Monitor | Alerting, Recovery | At-least-once |
| `CircuitBreakerOpened` | `{ session_id, component, error_rate, threshold, opened_at }` | Circuit Breaker | Alerting, Failover | Exactly-once |
| `CircuitBreakerClosed` | `{ session_id, component, duration_open_ms, recovery_action }` | Circuit Breaker | Dashboard, Providers | Exactly-once |
| `CircuitBreakerHalfOpened` | `{ session_id, component, probe_count, success_threshold }` | Circuit Breaker | Observability | At-least-once |

#### Provider & Failover Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `ProviderFailed` | `{ session_id, provider_id, error, latency_ms, consecutive_failures }` | Provider Interface | Circuit Breaker, Failover | At-least-once |
| `ProviderRecovered` | `{ session_id, provider_id, latency_ms, recovery_duration_ms }` | Provider Interface | Circuit Breaker | At-least-once |
| `ProviderDegraded` | `{ session_id, provider_id, current_latency_ms, threshold_latency_ms, error_rate }` | Provider Interface | Observability, Dashboard | At-least-once |
| `ProviderFailoverStarted` | `{ session_id, from_provider, to_provider, reason }` | Failover Engine | Dashboard, Audit | Exactly-once |
| `ProviderFailoverCompleted` | `{ session_id, from_provider, to_provider, duration_ms, success }` | Failover Engine | Dashboard, Audit | Exactly-once |
| `ProviderLatencySpike` | `{ session_id, provider_id, current_latency, p50_latency, p99_latency, spike_ratio }` | Monitoring | Alerting, Auto-failover | At-least-once |

#### Recovery & Resilience Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `KernelRecovered` | `{ session_id, recovery_type{retry, checkpoint, failover, resume}, checkpoint_id, duration_ms }` | Recovery Engine | Kernel, Dashboard | Exactly-once |
| `KernelFailed` | `{ session_id, error, state_at_failure, last_checkpoint, diagnostics{} }` | Kernel | CEO, User | Exactly-once |
| `RecoveryStarted` | `{ session_id, recovery_strategy, target_state, checkpoint_available }` | Recovery Engine | Dashboard, Observability | At-least-once |
| `RecoveryCompleted` | `{ session_id, recovery_strategy, restored_state, duration_ms, data_loss{yes, no} }` | Recovery Engine | Kernel, Memory | Exactly-once |
| `RecoveryFailed` | `{ session_id, recovery_strategy, error, exhausted_attempts, escalation }` | Recovery Engine | CEO, Alerting | Exactly-once |
| `CheckpointCreated` | `{ session_id, checkpoint_id, state, size_bytes, artifact_count }` | Execution Engine | Recovery, Storage | At-least-once |
| `SnapshotRestored` | `{ session_id, snapshot_id, age_hours, stores_restored[], data_loss }` | Recovery Engine | Audit, Dashboard | Exactly-once |

#### Hot Reload Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `HotReloadTriggered` | `{ session_id, file_changed, change_type{modified, created, deleted}, file_hash }` | File Watcher | Kernel, Runtime | At-least-once |
| `HotReloadStarted` | `{ session_id, file, parser_version }` | Kernel | Dashboard, Observability | At-least-once |
| `HotReloadCompleted` | `{ session_id, file, duration_ms, artifacts_updated{}, cache_invalidated{} }` | Kernel | Dashboard, Runtime | At-least-once |
| `HotReloadFailed` | `{ session_id, file, error, state{rolled_forward, rolled_back} }` | Kernel | Dashboard, Alerting | Exactly-once |
| `HotReloadPartial` | `{ session_id, file, failed_components[], succeeded_components[], warning }` | Kernel | Dashboard, Observability | At-least-once |

#### Governance & Feature Flag Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `FeatureFlagChanged` | `{ flag_name, previous_state, new_state, changed_by, reason, effective_immediately }` | Feature Flag Engine | Runtime, Dashboard | Exactly-once |
| `FeatureFlagCreated` | `{ flag_name, phase{experimental, beta, stable}, owner, description }` | Feature Flag Engine | Registry, Dashboard | Exactly-once |
| `PolicyCreated` | `{ policy_id, domain, rules[], severity, effective_date }` | Policy Engine | Compliance, Dashboard | Exactly-once |
| `PolicyViolation` | `{ session_id, policy_id, rule, violated_by, action_taken, severity }` | Policy Engine | Compliance, Audit | Exactly-once |
| `DecisionRecorded` | `{ session_id, decision_id, type, authority, rationale, alternatives[] }` | Architecture Chief | Memory, Governance | Exactly-once |

#### Sync & Pipeline Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `SyncStarted` | `{ session_id, targets[]{type, store}, total_entries }` | Sync Engine | Dashboard, Observability | At-least-once |
| `SyncCompleted` | `{ session_id, synced_stores[], failed_stores[], duration_ms, total_bytes }` | Sync Engine | Dashboard, Memory | At-least-once |
| `SyncFailed` | `{ session_id, store, error, remaining_entries, retry_count }` | Sync Engine | Recovery, Observability | At-least-once |
| `PipelineStageStarted` | `{ session_id, stage_name, pipeline_type }` | Pipeline Engine | Dashboard, Observability | At-least-once |
| `PipelineStageCompleted` | `{ session_id, stage_name, duration_ms, artifacts_processed }` | Pipeline Engine | Orchestrator, Dashboard | At-least-once |
| `PipelineCompleted` | `{ session_id, pipeline_type, stages_total, total_duration_ms, success }` | Pipeline Engine | Kernel, Audit | At-least-once |

#### Cross-Layer & Integration Events

| Event | Payload | Publisher | Consumers | Delivery |
|-------|---------|-----------|-----------|----------|
| `EscalationTriggered` | `{ session_id, level{1-5}, reason, from_layer, to_role }` | Runtime | Organizational Layer | Exactly-once |
| `EscalationResolved` | `{ session_id, level, resolution, decided_by, outcome }` | Organizational Layer | Runtime | Exactly-once |
| `ComplianceCheckPassed` | `{ session_id, check_type, domain, score }` | Compliance Engine | Audit, Dashboard | Exactly-once |
| `ComplianceCheckFailed` | `{ session_id, check_type, domain, violations[], required_action }` | Compliance Engine | Governance, Alerting | Exactly-once |
| `LayerIsolationViolation` | `{ session_id, rule_id{L-001..L-010}, violating_component, action_taken }` | Isolation Monitor | Audit, CEO | Exactly-once |
| `ExternalWebhookDelivered` | `{ session_id, webhook_url, event_type, status_code, duration_ms }` | Webhook Engine | Audit, Dashboard | Best-effort |
| `ExternalWebhookFailed` | `{ session_id, webhook_url, event_type, status_code, error, retry_count }` | Webhook Engine | Alerting, Retry | At-least-once |

---

### 4.4 Event Schema & Serialization

#### Standard Event Envelope

Every event transported through the Event Bus uses this standard envelope:

```json
{
  "event": "EventName",
  "version": "1.0",
  "id": "uuid",
  "session_id": "uuid",
  "timestamp": "2026-07-15T12:00:00.000Z",
  "publisher": "component-name",
  "topic": "events/v1/{domain}/{entity}/{action}",
  "partition_key": "string",
  "correlation_id": "uuid",
  "causation_id": "uuid",
  "payload": {},
  "metadata": {
    "runtime_type": "opencode | claude-code | codex | cosca-runtime | sdk",
    "environment": "development | staging | production",
    "tenant_id": "string (multi-tenant)",
    "retry_count": 0,
    "original_timestamp": "ISO8601"
  }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `event` | Yes | Canonical event name (PascalCase) |
| `version` | Yes | Event schema version (SemVer) |
| `id` | Yes | Unique event identifier (UUID v7, time-sortable) |
| `session_id` | Yes | Originating session |
| `timestamp` | Yes | Event creation time (ISO8601, UTC) |
| `publisher` | Yes | Component that published the event |
| `topic` | Yes | Canonical topic path |
| `partition_key` | Yes | Key for topic partitioning |
| `correlation_id` | Yes | Traces entire request/transaction across events |
| `causation_id` | Yes | ID of event that caused this event (parent trace) |
| `payload` | Yes | Event-specific data |
| `metadata` | Yes | Enrichment context |

#### Serialization Format

```yaml
serialization:
  wire_format: "JSON (UTF-8, no BOM)"
  binary_format: "Protocol Buffers (proto3, for high-throughput)"
  schema_registry: "Confluent Schema Registry | Apicurio"
  
  compatibility:
    - "BACKWARD: new schema can read old data"
    - "FORWARD: old schema can read new data"
    - "Validation: on publish AND on consume"
    
  compression:
    algorithm: "gzip (level 6) for payload > 1KB"
    min_size_bytes: 1024
    max_ratio: 0.8  # Skip if compression ratio < 0.8
```

#### Schema Registry Integration

```yaml
schema_registry:
  purpose: "Enforce event schema compatibility"
  
  operations:
    register:
      trigger: "New event type or schema change"
      validation: "Compatibility check against previous version"
      
    validate:
      trigger: "Every publish (producer-side)"
      action: "Reject if schema incompatible"
      
    evolve:
      compatibility: "BACKWARD_TRANSITIVE"
      rules:
        - "May add optional fields"
        - "May not remove required fields"
        - "May not change field types"
        - "May change field names (with annotation)"
        
  storage:
    backend: "Database (source of truth)"
    cache: "Redis (fast lookup)"
    retention: "All schema versions indefinitely"
```

---

### 4.5 Event Delivery Guarantees

Every event has a specified **delivery guarantee** that determines how the Event Bus handles publish, consume, and failure scenarios.

#### Delivery Levels

| Level | Guarantee | Description | Use Cases | Cost |
|-------|-----------|-------------|-----------|------|
| **Exactly-once** | Event consumed exactly once | Idempotent consumers, deduplication, transaction log | Governance decisions, finance, compliance, session completion | High |
| **At-least-once** | Event consumed at least once | Automatic retry on failure, persistent queue | Workflow events, capability resolution, execution results | Medium |
| **Best-effort** | Event delivered once, no retry | Fire-and-forget, periodic cleanup | Health checks, metrics, logs, debug events | Low |

#### Exactly-Once Delivery Protocol

```yaml
exactly_once:
  mechanism: "Transaction log + idempotent consumer + deduplication"
  
  producer_side:
    - "Write event to transaction log (WAL)"
    - "Commit WAL before publishing to bus"
    - "Include deduplication key in event metadata"
    
  broker_side:
    - "Deduplicate by event.id (UUID v7)"
    - "Store in persistent topic log"
    - "Acknowledge only after durable storage"
    
  consumer_side:
    - "Idempotent processing (same event → same result)"
    - "Record processed event.id in consumer offset store"
    - "Skip if event.id already processed"
    
  failure:
    - "On producer failure: retry until WAL committed"
    - "On broker failure: leader election, no data loss"
    - "On consumer failure: reprocess from last committed offset"
```

#### At-Least-Once Delivery Protocol

```yaml
at_least_once:
  mechanism: "Persistent queue + retry + redelivery"
  
  retry_policy:
    max_attempts: 5
    backoff: "exponential (1s, 2s, 4s, 8s, 16s)"
    max_backoff_ms: 30000
    
  redelivery:
    on_failure: "Re-queue with retry_count + 1"
    on_timeout: "Re-queue with retry_count + 1"
    dead_letter_after: 5
    
  ordering:
    per_partition: "guaranteed"
    cross_partition: "not guaranteed"
```

---

### 4.6 Event Retention & Replay

#### Retention Policy

| Tier | Storage | Retention | Queryable | Format | Compressed |
|------|---------|-----------|-----------|--------|------------|
| **Hot** | Redis streams | 24 hours | Real-time (sub-second) | JSON | No |
| **Warm** | Database (partitioned) | 90 days | Seconds | JSON | No |
| **Cold** | Object storage (S3/GCS) | 1 year | Minutes | JSON Lines (gzip) | Yes (~10:1) |
| **Archive** | Object storage | 7 years (compliance) | Hours (restore required) | JSON Lines (gzip) | Yes (~10:1) |
| **Compliance** | Immutable object storage | Indefinite | Hours (audit only) | JSON Lines (gzip) + signature | Yes |

#### Event Replay

```yaml
replay:
  supported: true
  
  trigger:
    - "Consumer recovery after crash"
    - "Debugging and RCA"
    - "Testing and simulation"
    - "Data backfill"
    - "Compliance audit"
    
  modes:
    full:
      description: "Replay all events from a point in time"
      time_range: "start_timestamp → end_timestamp"
      rate: "1x, 10x, 100x (configurable)"
      
    filtered:
      description: "Replay events matching filter criteria"
      filters: ["event_name", "session_id", "publisher", "topic"]
      
    session:
      description: "Replay all events for a specific session"
      session_id: "uuid"
      preserve_order: true
      
  limitations:
    - "Hot tier only (last 24h) for real-time replay"
    - "Warm+ tiers require load from persistent storage"
    - "Archive tier restore time: 1-10 minutes"
```

---

### 4.7 Event Filtering & Routing

#### Consumer Subscription Model

```yaml
subscription:
  consumer_group:
    id: "consumer-group-name"
    members: ["consumer-1", "consumer-2"]
    rebalance: "cooperative-sticky"
    
  topic_filter:
    pattern: "events/v1/{domain}/*"
    include: ["session/", "workflow/"]
    exclude: ["health/", "system/metrics"]
    
  event_filter:
    include: ["SessionStarted", "SessionFinished", "Execution*"]
    exclude: ["HealthCheckPassed"]
    
  quality_of_service:
    delivery: "at-least-once"
    max_retries: 5
    dead_letter: true
```

#### Routing Rules

| Rule | Condition | Action |
|------|-----------|--------|
| Priority routing | `event.metadata.priority == critical` | Route to immediate queue, notify on-call |
| Session isolation | `event.session_id == active_session` | Route to session-specific consumer |
| Audit routing | `event.publisher in [governance, compliance]` | Route to immutable audit log |
| Dashboard routing | `event in [SessionStarted,Execution*,Quality*]` | Route to SSE stream |
| Archive routing | `event.timestamp < now - 90d` | Route to cold storage |
| Dead letter | `event.metadata.retry_count > 5` | Route to DLQ, notify admin |

---

### 4.8 Event Priority & Backpressure

#### Priority Levels

| Level | Value | Queue | Latency SLA | Droppable |
|-------|-------|-------|-------------|-----------|
| **CRITICAL** | 100 | Immediate | < 100ms | No |
| **HIGH** | 75 | Priority | < 1s | No |
| **NORMAL** | 50 | Standard | < 5s | No |
| **LOW** | 25 | Background | < 60s | Yes (under backpressure) |
| **DEBUG** | 0 | Background | Best-effort | Yes |

#### Backpressure Mechanism

```yaml
backpressure:
  detection:
    metric: "consumer_lag > threshold"
    threshold: 1000  # Unprocessed events
    window_ms: 10000
    
  actions:
    level_1:  # Lag > 1000
      action: "Scale up consumers"
      details: "Add consumer group members"
      
    level_2:  # Lag > 5000
      action: "Drop LOW and DEBUG events"
      details: "Prioritize CRITICAL and HIGH"
      
    level_3:  # Lag > 20000
      action: "Reject new NON-CRITICAL publishes"
      details: "Return 429 Too Many Requests"
      
    level_4:  # Lag > 50000
      action: "Circuit breaker on Event Bus"
      details: "Fall back to local logging, replay on recovery"
      
  recovery:
    - "Replay dropped events from warm tier"
    - "Resume normal consumption"
    - "Publish BusRecovered event"
```

---

### 4.9 Event Bus Health & Metrics

#### Health Checks

| Check | Interval | Timeout | Failure Action |
|-------|----------|---------|----------------|
| Bus connectivity | 5s | 2s | Retry connection, alert |
| Publish latency | 10s | 5s | Log warning if > 200ms |
| Consumer lag | 10s | 5s | Alert if lag > 1000 |
| Queue depth | 10s | 5s | Alert if depth > 10000 |
| Dead letter count | 60s | 5s | Alert if DLQ > 100 |
| Schema registry | 30s | 5s | Degraded mode (skip validation) |
| Storage health | 30s | 5s | Failover to secondary storage |

#### Bus Metrics

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `eventbus.published.total` | Counter | topic, publisher | Total events published |
| `eventbus.published.bytes` | Histogram | topic | Event size distribution |
| `eventbus.published.latency_ms` | Histogram | topic | Publish latency |
| `eventbus.consumed.total` | Counter | topic, consumer_group | Total events consumed |
| `eventbus.consumed.latency_ms` | Histogram | topic, consumer_group | End-to-end latency |
| `eventbus.consumer.lag` | Gauge | topic, consumer_group | Consumer lag |
| `eventbus.queue.depth` | Gauge | queue | Current queue depth |
| `eventbus.retry.count` | Counter | topic | Retry events |
| `eventbus.dead_letter.count` | Counter | topic | Dead letter events |
| `eventbus.dropped.count` | Counter | priority | Dropped events (backpressure) |
| `eventbus.throughput.events_per_sec` | Gauge | topic | Events per second |
| `eventbus.error.count` | Counter | operation | Bus errors |
| `eventbus.schema.validation.count` | Counter | result | Schema validation results |
| `eventbus.replay.count` | Counter | mode | Replay operations |

---

### 4.10 Event Bus Security

#### Authentication & Authorization

```yaml
security:
  authentication:
    mechanism: "JWT (JSON Web Tokens)"
    issuer: "Kernel"
    audience: "event-bus"
    token_lifetime: "session duration"
    
  authorization:
    model: "RBAC (Role-Based Access Control)"
    roles:
      publisher:
        - "Can publish to specific topics"
        - "Validated against publisher registry"
      consumer:
        - "Can subscribe to specific topics"
        - "Validated against consumer registry"
      admin:
        - "Can create/modify topics"
        - "Can manage consumer groups"
        - "Can access dead letter queue"
        
  encryption:
    in_transit: "TLS 1.3"
    at_rest: "AES-256-GCM (events in storage)"
    key_rotation: "90 days"
    
  audit:
    - "All publish attempts logged (success + failure)"
    - "All consume attempts logged (success + failure)"
    - "All authorization failures logged and alerted"
```

#### Topic Access Control

| Topic Pattern | Publishers | Consumers |
|--------------|-----------|-----------|
| `events/v1/session/` | Kernel | All engines |
| `events/v1/workflow/` | Workflow Engine | Planning, Execution |
| `events/v1/capability/` | Capability Engine | Workflow Engine |
| `events/v1/quality/` | Quality Engine | Review Chief, Dashboard |
| `events/v1/system/` | Health Monitor | Dashboard, Alerting |
| `events/v1/governance/` | Policy Engine | Audit, Compliance |
| `events/v1/health/` | All components | Health Monitor, Dashboard |

---

### 4.11 Event Correlation

Every event carries a **correlation_id** that traces the entire chain of events triggered by a single root request.

#### Correlation Model

```
User Request
  │
  correlation_id = X
  ▼
SessionStarted (correlation_id = X, causation_id = null)
  │
  ▼
BootstrapStarted (correlation_id = X, causation_id = SessionStarted.id)
  │
  ▼
DiscoveryCompleted (correlation_id = X, causation_id = BootstrapCompleted.id)
  │
  ▼
CapabilityResolved (correlation_id = X, causation_id = DiscoveryCompleted.id)
  │
  ├──→ WorkflowResolved (correlation_id = X, causation_id = CapabilityResolved.id)
  │         │
  │         ├──→ ExecutionStarted (correlation_id = X, causation_id = PlanCreated.id)
  │         │         │
  │         │         ├──→ StepCompleted (correlation_id = X, causation_id = ExecutionStarted.id)
  │         │         └──→ ExecutionCompleted (correlation_id = X, causation_id = StepCompleted.id)
  │         │
  │         └──→ ReviewCompleted (correlation_id = X, causation_id = ExecutionCompleted.id)
  │
  └──→ SessionFinished (correlation_id = X, causation_id = SyncCompleted.id)
```

#### Correlation Queries

| Query | Description | Implementation |
|-------|-------------|----------------|
| `GET /events?correlation_id=X` | Trace entire request chain | Query by correlation_id index |
| `GET /events?causation_id=Y` | Find events caused by event Y | Query by causation_id index |
| `GET /events?session_id=Z` | All events in a session | Query by session_id index |
| `GET /traces/{correlation_id}` | Full trace tree | Graph query of causation links |

---

### 4.12 Event Choreography

The Event Bus enables **event-driven choreography** — workflows where each step is triggered by events produced by previous steps, without a central orchestrator.

#### Choreography Pattern

```
Step A publishes "TaskCompleted"
  │
  ▼
Event Bus routes to subscribed consumers
  │
  ├── Step B consumes → publishes "StepBCompleted"
  │     │
  │     ├── Step D consumes → publishes "StepDCompleted"
  │     └── Step E consumes → publishes "StepECompleted"
  │
  └── Step C consumes → publishes "StepCCompleted"
        │
        └── Aggregator consumes {B, C, D, E} → publishes "AllCompleted"
```

#### Saga Pattern (Distributed Transactions)

```yaml
saga:
  pattern: "Choreography-based saga"
  
  steps:
    - capability: "CAP-ENG-001"
      action: "Deploy service"
      compensator: "Rollback deployment"
      success_event: "DeploymentCompleted"
      failure_event: "DeploymentFailed"
      
    - capability: "CAP-DATA-004"
      action: "Run migration"
      compensator: "Revert migration"
      success_event: "MigrationCompleted"
      failure_event: "MigrationFailed"
      
  compensation:
    trigger: "Any failure event in saga"
    action: "Execute compensators in reverse order"
    completion: "Publish SagaCompensated"
```

---

### 4.13 Event Sourcing

Events MAY be used as the **source of truth** for state reconstruction (Event Sourcing pattern).

```yaml
event_sourcing:
  supported: true
  scope: "Per-session | Per-workflow | Per-capability"
  
  state_reconstruction:
    method: "Replay all events for aggregate from start"
    performance: "O(n) where n = events for aggregate"
    optimization: "Snapshots every 100 events"
    
  snapshot:
    frequency: "Every 100 events or every hour"
    storage: "Same as event store"
    recovery: "Load latest snapshot + replay events since snapshot"
    
  use_cases:
    - "Session state recovery after crash"
    - "Workflow execution audit"
    - "Capability performance analysis"
    - "Compliance traceability"
    - "Debugging and RCA"
```

---

### 4.14 Dead Letter Queue

Unprocessable events are routed to the **Dead Letter Queue (DLQ)**.

#### DLQ Architecture

```yaml
dead_letter_queue:
  trigger_conditions:
    - "Retry count exceeded max (5)"
    - "Schema validation failed"
    - "Consumer returned permanent error"
    - "Event TTL expired before processing"
    
  storage:
    primary: "Dedicated DLQ topic (persistent)"
    retention: "30 days"
    max_size: "10,000 events (then oldest dropped)"
    
  management:
    inspect: "List DLQ events with metadata"
    replay: "Replay selected events to original topic"
    discard: "Acknowledge and remove from DLQ"
    alert: "Notification when DLQ > 100 events"
    
  analysis:
    - "Group by error type"
    - "Group by publisher"
    - "Identify systemic failures"
    - "Trigger incident if DLQ growth rate > threshold"
```

---

### 4.15 Event Bus Configuration

The Event Bus behavior is controlled by a formal configuration schema.

```yaml
eventbus_config:
  broker:
    type: "redis | kafka | in-memory"
    hosts: ["localhost:6379"]
    cluster_mode: true
    
  topics:
    auto_create: true
    default_partitions: 4
    replication_factor: 2
    
  persistence:
    hot_retention_hours: 24
    warm_retention_days: 90
    cold_retention_days: 365
    archive_enabled: true
    
  security:
    tls_enabled: true
    auth_required: true
    acl_enabled: true
    
  performance:
    max_message_bytes: 1048576  # 1MB
    max_batch_bytes: 5242880    # 5MB
    compression: true
    compression_algorithm: "gzip"
    
  consumer:
    max_poll_records: 500
    max_poll_interval_ms: 300000
    heartbeat_interval_ms: 3000
    session_timeout_ms: 10000
    
  monitoring:
    metrics_enabled: true
    health_check_enabled: true
    tracing_enabled: true
```


