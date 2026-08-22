# Runtime State Machine — Extracted from KERNEL.md §3

> **Source**: KERNEL.md v3.0.1 §3 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 3. RUNTIME STATE MACHINE

The Runtime **always** has a current state. State transitions are explicit, tracked, published as events, and persisted for recovery. The state machine is the **single source of truth** for Runtime lifecycle.

---

### 3.1 Formal State Machine Model

The Cosca Runtime State Machine is defined as a **deterministic finite automaton** (DFA):

```
M = (S, Σ, δ, s₀, F)

Where:
  S  = Set of valid states (14 states)
  Σ  = Input alphabet (events that trigger transitions)
  δ  = Transition function: S × Σ → S
  s₀ = Initial state: BOOTSTRAPPING
  F  = Final states: {FINISHED, FAILED}
```

#### State Set S

```
S = {
  BOOTSTRAPPING,    # Runtime initialization
  DISCOVERING,      # Workspace scanning
  LOADING_CONTEXT,  # Context loading
  LOADING_MEMORY,   # Memory loading
  VALIDATING,       # Request validation (Gate 0)
  PLANNING,         # Execution planning
  EXECUTING,        # DAG execution
  REVIEWING,        # Code/architecture/security review
  DOCUMENTING,      # Documentation update
  LEARNING,         # Post-session learning
  SYNCING,          # Knowledge/dashboard sync
  FINISHED,         # Session complete (absorbing)
  FAILED,           # Irrecoverable failure (absorbing)
  RECOVERING        # Recovery in progress
}
```

#### Input Alphabet Σ

```
Σ = {
  bootstrap_completed,    discovery_completed,
  context_loaded,         memory_loaded,
  validation_passed,      validation_failed,
  plan_created,           execution_completed,
  execution_failed,       review_completed,
  documentation_updated,  learning_completed,
  sync_completed,         kernel_recovered,
  kernel_failed,          timeout_elapsed,
  user_interrupt,         recovery_completed
}
```

---

### 3.2 State Diagram

```
                    ┌──────────────────┐
                    │   BOOTSTRAPPING  │
                    └────────┬─────────┘
                             │ bootstrap_completed
                             ▼
                    ┌──────────────────┐
                    │   DISCOVERING    │
                    └────────┬─────────┘
                             │ discovery_completed
                             ▼
                    ┌──────────────────┐
                    │ LOADING_CONTEXT  │
                    └────────┬─────────┘
                             │ context_loaded
                             ▼
                    ┌──────────────────┐
                    │ LOADING_MEMORY   │
                    └────────┬─────────┘
                             │ memory_loaded
                             ▼
                    ┌──────────────────┐
                    │   VALIDATING     │◀──────────┐
                    └────────┬─────────┘           │
                             │                    │
                    ┌────────┴────────┐           │
                    │                 │           │
                    ▼                 ▼           │
              ┌──────────┐   ┌──────────┐         │
              │ PLANNING │   │  FAILED  │         │
              └────┬─────┘   └────┬─────┘         │
                   │              │               │
                   ▼              │               │
              ┌──────────┐        │               │
              │EXECUTING │        │               │
              └────┬─────┘        │               │
                   │              │               │
                   ▼              │               │
              ┌──────────┐        │               │
              │ REVIEWING│        │               │
              └────┬─────┘        │               │
                   │              │               │
                   ▼              │               │
              ┌──────────┐        │               │
              │DOCUMENTING│        │               │
              └────┬─────┘        │               │
                   │              │               │
                   ▼              │               │
              ┌──────────┐        │               │
              │ LEARNING │        │               │
              └────┬─────┘        │               │
                   │              │               │
                   ▼              │               │
              ┌──────────┐        │               │
              │ SYNCING  │        │               │
              └────┬─────┘        │               │
                   │              │               │
          ┌────────┴────────┐     │               │
          │                 │     │               │
          ▼                 ▼     │               │
   ┌──────────┐      ┌──────────┐ │               │
   │ FINISHED │      │  FAILED  │ │               │
   └──────────┘      └────┬─────┘ │               │
                          │       │               │
                          ▼       │               │
                   ┌────────────┐ │               │
                   │ RECOVERING │◄┘               │
                   └──────┬─────┘                 │
                          │ recovery_completed     │
                          └───────────────────────┘
```

---

### 3.3 Complete Transition Matrix

This matrix defines **all valid transitions**. Cells marked ✅ are valid. Empty cells are **invalid** and MUST be rejected by the Runtime.

```
FROM → TO            | BOO | DIS | CXT | MEM | VAL | PLN | EXE | REV | DOC | LRN | SYN | FIN | FAL | REC
═════════════════════|═════|═════|═════|═════|═════|═════|═════|═════|═════|═════|═════|═════|═════|═════
BOOTSTRAPPING        |  —  | ✅  |     |     |     |     |     |     |     |     |     |     | ✅  |
DISCOVERING          |     |  —  | ✅  |     |     |     |     |     |     |     |     |     | ✅  |
LOADING_CONTEXT      |     |     |  —  | ✅  |     |     |     |     |     |     |     |     | ✅  |
LOADING_MEMORY       |     |     |     |  —  | ✅  |     |     |     |     |     |     |     | ✅  |
VALIDATING           |     |     |     |     |  —  | ✅  |     |     |     |     |     |     | ✅  |
PLANNING             |     |     |     |     |     |  —  | ✅  |     |     |     |     |     | ✅  |
EXECUTING            |     |     |     |     |     |     |  —  | ✅  |     |     |     |     | ✅  |
REVIEWING            |     |     |     |     |     |     |     |  —  | ✅  |     |     |     | ✅  |
DOCUMENTING          |     |     |     |     |     |     |     |     |  —  | ✅  |     |     | ✅  |
LEARNING             |     |     |     |     |     |     |     |     |     |  —  | ✅  |     | ✅  |
SYNCING              |     |     |     |     |     |     |     |     |     |     |  —  | ✅  | ✅  |
FINISHED             |     |     |     |     |     |     |     |     |     |     |     |  —  |     |
FAILED               |     |     |     |     |     |     |     |     |     |     |     |     |  —  | ✅
RECOVERING           |     |     |     |     |     | ✅  |     |     |     |     |     | ✅  | ✅  |  —
```

#### Transition Matrix Rules

1. **Exactly one transition per source state on each input event**
2. All states may transition to **FAILED** on any error event
3. **FINISHED** and **FAILED** are absorbing (no outgoing transitions except FAILED → RECOVERING)
4. **RECOVERING** may transition back to **PLANNING** (retry), **FINISHED** (recovered with no remaining work), or **FAILED** (recovery failed)
5. Transitions not in the matrix are **invalid** and MUST be rejected
6. Invalid transitions MUST be logged as state machine violations

---

### 3.4 State Guards & Preconditions

Every state transition has a **guard** — a precondition that MUST be satisfied for the transition to be valid.

| Transition | Guard Condition | Failure Action |
|-----------|----------------|---------------|
| BOOTSTRAPPING → DISCOVERING | Config loaded, paths resolved, providers initialized | Retry bootstrap |
| DISCOVERING → LOADING_CONTEXT | Workspace scan completed (may be partial) | Continue with partial results |
| LOADING_CONTEXT → LOADING_MEMORY | Context loaded successfully | Retry context load |
| LOADING_MEMORY → VALIDATING | At least session and project memory loaded | Continue without global memory |
| VALIDATING → PLANNING | Gate 0 passed, capabilities resolved | Return to request, notify user |
| VALIDATING → FAILED | Gate 0 failed with errors | Log validation failure |
| PLANNING → EXECUTING | Plan generated, DAG validated, no cycles | Retry planning |
| EXECUTING → REVIEWING | All DAG steps completed | Handle partial completion |
| EXECUTING → FAILED | Irrecoverable step failure | Execute recovery |
| REVIEWING → DOCUMENTING | Review score >= minimum threshold | Return to EXECUTING for fixes |
| DOCUMENTING → LEARNING | Documentation updated | Continue with warnings |
| LEARNING → SYNCING | Learnings extracted | Skip learning if already up-to-date |
| SYNCING → FINISHED | All stores synchronized | Fallback to async sync |
| SYNCING → FAILED | Irrecoverable sync failure | Log sync failure |
| FAILED → RECOVERING | Recovery available and permitted | Check recovery policy |
| RECOVERING → PLANNING | Checkpoint restored, retry possible | Fallback to FAILED |
| RECOVERING → FINISHED | Recovered, no remaining work | Finalize session |
| RECOVERING → FAILED | Recovery exhausted all paths | Notify user |

---

### 3.5 State Actions & Side Effects

Each state has defined **entry actions** (executed on entry), **active actions** (executed during the state), and **exit actions** (executed on transition out).

#### Entry, Active, and Exit Actions

```
BOOTSTRAPPING
  Entry:   Load config, resolve virtual paths, initialize Event Bus,
           initialize Health Monitor, detect runtime type
  Active:  Validate configuration integrity, verify provider connectivity
  Exit:    Publish BootstrapCompleted, persist initial state

DISCOVERING
  Entry:   Load Discovery Engine, scan workspace, detect stack
  Active:  Monitor scan progress, emit discovery metrics
  Exit:    Publish DiscoveryCompleted, cache discovery results

LOADING_CONTEXT
  Entry:   Load Context Engine, build session/project/environment context
  Active:  Track context size, validate context integrity
  Exit:    Publish ContextLoaded, store context hash

LOADING_MEMORY
  Entry:   Load Memory Engine, load all memory types in priority order
  Active:  Monitor memory load progress, handle store failures gracefully
  Exit:    Publish MemoryLoaded, index loaded memories

VALIDATING
  Entry:   Apply Gate 0, classify request, resolve capabilities
  Active:  Run validation checks, collect validation metrics
  Exit:    Publish ValidationCompleted/ValidationFailed

PLANNING
  Entry:   Load Planning Engine, generate execution plan, create DAG
  Active:  Validate DAG (no cycles), perform risk assessment
  Exit:    Publish PlanCreated, persist plan

EXECUTING
  Entry:   Load Scheduler, queue DAG nodes, dispatch to workers
  Active:  Monitor execution progress, handle step completions/failures,
           manage retries, track worker health
  Exit:    Publish ExecutionCompleted/ExecutionFailed, aggregate results

REVIEWING
  Entry:   Load Review Engine, perform architecture/code/security review
  Active:  Track review progress, collect review issues
  Exit:    Publish ReviewCompleted, calculate review score

DOCUMENTING
  Entry:   Load Documentation Engine, identify changed artifacts
  Active:  Generate documentation updates, validate doc integrity
  Exit:    Publish DocumentationUpdated

LEARNING
  Entry:   Load Learning Engine, extract session patterns
  Active:  Update agent memory, identify improvement opportunities
  Exit:    Publish LearningCompleted

SYNCING
  Entry:   Sync to Database, Redis, Vector Store, Dashboard
  Active:  Monitor sync progress, handle sync failures per store
  Exit:    Publish SyncCompleted/SyncFailed

FINISHED
  Entry:   Return results to user, persist session context,
           promote short-term to long-term memory
  Active:  (idle — terminal state)
  Exit:    (none — absorbing)

FAILED
  Entry:   Log failure with full diagnostics, notify user,
           trigger Recovery Engine if applicable
  Active:  (idle — terminal state)
  Exit:    (none — except to RECOVERING)

RECOVERING
  Entry:   Load Recovery Engine, identify recovery strategy,
           restore last valid checkpoint
  Active:  Execute recovery plan, validate recovery integrity
  Exit:    Publish KernelRecovered/KernelFailed
```

#### Side Effect Catalog

| Side Effect | Trigger | Scope | Rollback |
|-------------|---------|-------|----------|
| Provider initialization | BOOTSTRAPPING entry | Global | Provider shutdown |
| Workspace cache creation | DISCOVERING exit | Session | Cache invalidation |
| Context hash storage | LOADING_CONTEXT exit | Session | Hash invalidation |
| Memory index build | LOADING_MEMORY exit | Session | Index rebuild |
| Capability lock acquisition | VALIDATING exit | Capability | Lock release |
| Plan persistence | PLANNING exit | Session | Plan deletion |
| Worker pool allocation | EXECUTING entry | Execution | Pool release |
| Artifact snapshot | EXECUTING exit | Artifact | Snapshot garbage collection |
| Review lock acquisition | REVIEWING entry | Artifact | Lock release |
| Doc version increment | DOCUMENTING exit | Document | Version rollback |
| Agent memory update | LEARNING exit | Agent | Memory revert |
| Store transaction begin | SYNCING entry | Store | Transaction rollback |
| Store transaction commit | SYNCING exit | Store | Transaction rollback |
| Session finalization | FINISHED entry | Session | (irreversible) |
| Checkpoint creation | RECOVERING entry | Recovery | Checkpoint invalidation |

---

### 3.6 State Timeouts & Watchdogs

Every state has a **hard timeout**. If the state does not complete within its timeout, a watchdog triggers a recovery flow.

#### State Timeout Catalog

| State | Hard Limit (ms) | Warning At (ms) | Watchdog Action |
|-------|----------------|----------------|-----------------|
| BOOTSTRAPPING | 30,000 | 20,000 | Log warning, retry bootstrap |
| DISCOVERING | 60,000 | 45,000 | Log warning, continue with partial discovery |
| LOADING_CONTEXT | 30,000 | 20,000 | Log warning, retry context load |
| LOADING_MEMORY | 60,000 | 45,000 | Log warning, continue without full memory |
| VALIDATING | 15,000 | 10,000 | Log warning, abort validation |
| PLANNING | 60,000 | 45,000 | Log warning, retry planning |
| EXECUTING | 300,000 | 240,000 | Log warning, checkpoint and continue |
| REVIEWING | 120,000 | 90,000 | Log warning, escalate to Review Chief |
| DOCUMENTING | 60,000 | 45,000 | Log warning, continue without full docs |
| LEARNING | 30,000 | 20,000 | Log warning, skip learning |
| SYNCING | 60,000 | 45,000 | Log warning, continue with partial sync |
| RECOVERING | 120,000 | 90,000 | Log critical, escalate to CEO |

#### Watchdog Mechanism

```yaml
watchdog:
  monitoring_interval_ms: 5000  # Check every 5 seconds
  
  on_timeout:
    action: "state_timeout_elapsed"
    severity: "warning | critical"
    log: true
    metrics: true
    
  on_hard_limit:
    action: "force_transition(FAILED)"
    severity: "critical"
    notify_escalation: true
    create_incident: true
    
  on_recovery_timeout:
    action: "force_transition(FAILED)"
    severity: "critical"
    notify_ceo: true
    create_incident: true
    attempt_full_restart: true
```

---

### 3.7 State Persistence & Recovery

The Runtime MUST persist its state machine state for crash recovery.

#### Persistence Model

```yaml
state_persistence:
  storage:
    primary: "Redis (for speed)"
    secondary: "Database (for durability)"
    
  schema:
    session_id: "uuid"
    current_state: "string"
    previous_state: "string"
    transition_history: []
    last_checkpoint: "timestamp"
    state_data: {}  # Serialized state context
    
  persistence_trigger:
    - On every state transition
    - On every checkpoint
    - Every 60 seconds (heartbeat)
    
  consistency:
    write_strategy: "write_to_both_before_transition"
    validation: "read_your_writes"
    conflict_resolution: "last_write_wins (timestamp-based)"
```

#### Recovery Flow

```yaml
recovery_flow:
  on_crash_or_restart:
    
    step_1: read_persisted_state
    action: "Load from Redis, fallback to Database"
    
    step_2: validate_state
    action: "Check if persisted state is valid"
    on_invalid: "Reset to BOOTSTRAPPING"
    
    step_3: check_heartbeat
    action: "If heartbeat older than 120s, treat as crash"
    
    step_4: determine_recovery_action
    rules:
      - "If FINISHED or FAILED → Start new session"
      - "If EXECUTING → Resume from last checkpoint"
      - "If REVIEWING → Resume review"
      - "If any other → Transition to RECOVERING"
      
    step_5: execute_recovery
    action: "Transition to appropriate state"
    
state_restoration_rules:
  - "State MUST be restored before processing any requests"
  - "Partial state restoration is not permitted"
  - "If state cannot be restored → new session forced"
```

---

### 3.8 State History & Audit

Every state transition is recorded in the **State Audit Log**.

#### Audit Log Schema

```yaml
state_audit_entry:
  session_id: "uuid"
  transition_id: "uuid"
  timestamp: "ISO8601"
  
  from_state: "string"
  to_state: "string"
  trigger_event: "string"
  
  duration_ms: 12345
  
  guard_conditions:
    precondition_met: true
    guard_evaluations: {}
    
  side_effects:
    executed: []
    failed: []
    
  metadata:
    runtime_type: "opencode | claude-code | cosca-runtime"
    initiated_by: "user | system | timer | recovery"
    correlation_id: "uuid"
  
  checkpoint:
    created: true
    checkpoint_id: "uuid"
    data_size_bytes: 1234
```

#### Audit Queries

| Query | Use Case | Implementation |
|-------|----------|----------------|
| "What is the current state?" | Dashboard, Health Check | Read from Redis |
| "What were the last 10 transitions?" | Debugging, RCA | Query state_audit_log |
| "How long did the EXECUTING state take?" | Performance analysis | Aggregate state_audit_log |
| "Which transitions failed most often?" | Reliability analysis | Count FAILED transitions |
| "Show me all RECOVERING transitions this week" | Stability monitoring | Filter by to_state |
| "What was the state when session X crashed?" | Incident response | Read last entry for session |

#### Audit Retention

| Tier | Retention | Storage | Queryable |
|------|-----------|---------|-----------|
| Hot | 7 days | Redis | Real-time |
| Warm | 90 days | Database | Seconds |
| Cold | 1 year | Archive (parquet) | Minutes |
| Compliance | 7 years | S3/GCS | Hours |

---

### 3.9 Nested State Machines

Some Runtime states contain **nested sub-state machines** for finer-grained tracking.

#### EXECUTING Sub-States

```
EXECUTING
  │
  ├── SCHEDULING     → DAG nodes being queued to scheduler
  ├── DISPATCHING    → Workers being assigned to nodes
  ├── RUNNING        → Nodes actively executing
  │     │
  │     ├── STEP_ACTIVE    → Individual step executing
  │     ├── STEP_RETRYING  → Step in retry backoff
  │     └── STEP_COMPLETED → Step finished
  ├── AGGREGATING    → Results being collected
  └── COMPLETING     → Final validation before transition
```

#### RECOVERING Sub-States

```
RECOVERING
  │
  ├── ASSESSING      → Determine recovery strategy
  ├── RESTORING      → Restore from checkpoint/snapshot
  ├── VALIDATING     → Validate restored state integrity
  └── RESUMING       → Resume execution from recovery point
```

#### Sub-State Transition Rules

```
1. Parent state MUST be active for sub-states to exist
2. Sub-states do NOT emit global events (only parent does)
3. Sub-state transitions are tracked in session context only
4. Parent state timeout includes all sub-state timeouts
5. FAILED in any sub-state → FAILED in parent state
```

---

### 3.10 Concurrent State Machines

The Runtime supports multiple concurrent state machine instances.

#### Concurrency Model

```yaml
concurrent_machines:
  session_machine:
    scope: "Per-session"
    count: "1 per active session"
    isolation: "Complete (separate state, memory, events)"
    
  workflow_machine:
    scope: "Per-workflow"
    count: "1 per active workflow"
    parent: "session_machine"
    
  capability_machine:
    scope: "Per-capability"
    count: "1 per executing capability"
    parent: "workflow_machine"
    
  health_machine:
    scope: "Global"
    count: "1 (singleton)"
    purpose: "Track Runtime health independently of sessions"
    
  governance_machine:
    scope: "Global"
    count: "1 (singleton)"
    purpose: "Track governance decisions independently"
```

#### Concurrency Rules

```
1. Each session has its own state machine instance
2. Session machines are fully isolated
3. Global machines (health, governance) are singletons
4. Child machines (workflow, capability) inherit parent isolation
5. Cross-machine communication is via Event Bus only
6. Machine ID = {type}:{scope_id} (e.g., session:uuid-123)
7. Maximum concurrent machines: 1000 (configurable)
```

---

### 3.11 State Machine Hooks (Pre/Post Transition)

The state machine supports **hooks** for plugins, engines, and extensions to observe or intercept transitions.

#### Hook Types

| Hook | Timing | Can Block? | Can Abort? | Use Case |
|------|--------|-----------|-----------|----------|
| **pre_transition** | Before guard evaluation | Yes (async) | Yes | Plugin validation, policy check |
| **post_transition** | After transition completed | No | No | Logging, metrics, event publishing |
| **on_enter** | After entering new state | Yes (sync) | Yes (redirect) | Resource allocation, provider init |
| **on_exit** | Before leaving current state | Yes (sync) | Yes (block) | Resource cleanup, state persistence |
| **on_error** | On transition error | No | No | Error logging, escalation trigger |

#### Hook Registration

```yaml
hook_registration:
  register:
    hook_type: "pre_transition | post_transition | on_enter | on_exit | on_error"
    target_states: ["state1", "state2"]  # Empty = all states
    handler: "engine_name | plugin_id"
    priority: 0-100  # Higher = executed first
    timeout_ms: 5000
    
  execution:
    order: "By priority (descending)"
    isolation: "Each hook runs in its own context"
    failure: "Non-blocking hooks log warning; blocking hooks abort transition"
    
  lifecycle:
    register_on: "Bootstrap"
    deregister_on: "Session end | Engine shutdown"
```

---

### 3.12 State Machine Metrics & Observability

Every state machine operation emits metrics.

#### Per-State Metrics

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `state.duration_ms` | Histogram | state, session_id | Time spent in each state |
| `state.transition.count` | Counter | from_state, to_state, result | Number of transitions |
| `state.transition.duration_ms` | Histogram | from_state, to_state | Time to complete transition |
| `state.current` | Gauge | state | Current state (1 for active, 0 for inactive) |
| `state.timeout.count` | Counter | state | Number of timeouts per state |
| `state.watchdog.triggered` | Counter | state, action | Watchdog triggers |
| `state.hook.duration_ms` | Histogram | hook_type, handler | Hook execution time |
| `state.hook.failure.count` | Counter | hook_type, handler | Hook failures |

#### Global State Machine Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `statemachine.sessions.active` | Gauge | Active session machines |
| `statemachine.sessions.total` | Counter | Total sessions started |
| `statemachine.transitions.total` | Counter | Total transitions across all machines |
| `statemachine.transitions.invalid` | Counter | Invalid transition attempts |
| `statemachine.recovery.count` | Counter | Recovery attempts |
| `statemachine.recovery.success` | Counter | Successful recoveries |
| `statemachine.concurrency.max` | Gauge | Peak concurrent machines |
| `statemachine.error.count` | Counter | State machine errors |

#### Health Check Integration

```yaml
statemachine_health:
  readiness:
    - "Current state is valid"
    - "State machine not in FAILED state"
    - "Heartbeat within threshold"
    
  liveness:
    - "State transitions are processing"
    - "Watchdog not triggered in last 60s"
    - "State persistence writable"
    
  degraded:
    - "RECOVERING state > 30s → log warning"
    - "EXECUTING state > 240s → log warning"
    - "More than 3 consecutive FAILED → alert"
```

---

### 3.13 State Machine Validation

The state machine itself MUST be validated for correctness.

#### Validation Rules

```
Rule SM-001: All states MUST be reachable from initial state
Rule SM-002: No deadlock states (except FINISHED/FAILED)
Rule SM-003: No unreachable transitions (dead transitions)
Rule SM-004: All guard conditions MUST be deterministic
Rule SM-005: All timeout values MUST be positive integers
Rule SM-006: Transition matrix MUST be complete (all valid transitions defined)
Rule SM-007: No self-transitions without explicit definition
Rule SM-008: Entry/exit actions MUST terminate within timeout
Rule SM-009: Side effects MUST be idempotent (safe for retry)
Rule SM-010: State persistence MUST be atomic
```

#### Validation Execution

```yaml
validation:
  frequency:
    - "On every Runtime startup"
    - "On every state machine definition change"
    - "Weekly (automatic)"
    
  methods:
    - "Model checking (reachability, deadlock)"
    - "Transition matrix completeness check"
    - "Guard condition determinism verification"
    - "Timeout value sanity check"
    
  output:
    on_pass: "StateMachineValidationPassed event"
    on_fail: "StateMachineValidationFailed event with violations[]"
    severity: "CRITICAL for SM-001 through SM-005"
    action_on_fail: "Block Runtime startup until resolved"
```

---

### 3.14 Formal Transition Function (δ)

The complete transition function δ: S × Σ → S, defined for all valid transitions:

```
δ(BOOTSTRAPPING,     bootstrap_completed)    → DISCOVERING
δ(BOOTSTRAPPING,     kernel_failed)          → FAILED

δ(DISCOVERING,       discovery_completed)    → LOADING_CONTEXT
δ(DISCOVERING,       kernel_failed)          → FAILED

δ(LOADING_CONTEXT,   context_loaded)         → LOADING_MEMORY
δ(LOADING_CONTEXT,   kernel_failed)          → FAILED

δ(LOADING_MEMORY,    memory_loaded)          → VALIDATING
δ(LOADING_MEMORY,    kernel_failed)          → FAILED

δ(VALIDATING,        validation_passed)      → PLANNING
δ(VALIDATING,        validation_failed)      → FAILED
δ(VALIDATING,        kernel_failed)          → FAILED

δ(PLANNING,          plan_created)           → EXECUTING
δ(PLANNING,          kernel_failed)          → FAILED

δ(EXECUTING,         execution_completed)    → REVIEWING
δ(EXECUTING,         execution_failed)       → FAILED
δ(EXECUTING,         kernel_failed)          → FAILED
δ(EXECUTING,         timeout_elapsed)        → FAILED

δ(REVIEWING,         review_completed)       → DOCUMENTING
δ(REVIEWING,         kernel_failed)          → FAILED

δ(DOCUMENTING,       documentation_updated)  → LEARNING
δ(DOCUMENTING,       kernel_failed)          → FAILED

δ(LEARNING,          learning_completed)     → SYNCING
δ(LEARNING,          kernel_failed)          → FAILED

δ(SYNCING,           sync_completed)         → FINISHED
δ(SYNCING,           kernel_failed)          → FAILED

δ(FINISHED,          _)                      → FINISHED          (absorbing)

δ(FAILED,            kernel_recovered)       → RECOVERING         (if recovery available)
δ(FAILED,            _)                      → FAILED             (absorbing, no recovery)

δ(RECOVERING,        recovery_completed)     → PLANNING           (retry path)
δ(RECOVERING,        kernel_recovered)       → FINISHED           (no remaining work)
δ(RECOVERING,        kernel_failed)          → FAILED             (recovery failed)
```


