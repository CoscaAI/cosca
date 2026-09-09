# Execution Graph (DAG) — Extracted from KERNEL.md §5

> **Source**: KERNEL.md v3.0.1 §5 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 5. EXECUTION GRAPH (DAG)

The Kernel **never** executes tasks sequentially. Every execution plan is a **Directed Acyclic Graph (DAG)** — the fundamental execution primitive of the Runtime. The DAG defines WHAT executes, IN WHAT ORDER, and WITH WHAT CONSTRAINTS.

---

### 5.1 Formal DAG Model

An Execution Graph is formally defined as a **weighted directed acyclic graph**:

```
G = (V, E, w, λ, π)

Where:
  V  = Set of execution nodes (vertices)
  E  = Set of dependency edges (directed, acyclic)
  w  = Node weight function: V → {priority, estimated_duration, resource_profile}
  λ  = Node label function: V → {capability_id, step_type, provider}
  π  = Edge label function: E → {dependency_type, data_flow}
```

#### Graph Invariants

```
Invariant G-01: G MUST be acyclic (no directed cycles)
Invariant G-02: G MUST have exactly one source node (no incoming edges)
Invariant G-03: G MUST have at least one sink node (no outgoing edges)
Invariant G-04: Every node MUST be reachable from the source
Invariant G-05: Every node MUST have a path to at least one sink
Invariant G-06: Edge weights MUST be non-negative
Invariant G-07: Node IDs MUST be unique within the DAG
Invariant G-08: Edge references MUST point to existing nodes
```

#### DAG Schema

```yaml
execution_graph:
  id: "dag-uuid"
  session_id: "session-uuid"
  plan_id: "plan-uuid"

  metadata:
    created_at: "ISO8601"
    version: 1
    status: "draft | validated | executing | completed | failed | cancelled"
    total_nodes: 12
    total_edges: 14
    estimated_duration_ms: 60000
    critical_path_length: 4
    parallel_groups: 3

  nodes:
    - id: "node-uuid-1"
      type: "capability | workflow | task | review | quality | documentation | decision | gate | sync"
      capability: "CAP-XXX-XXX"
      name: "Human-readable name"
      description: "What this node does"

      provider:
        type: "chief | engine | specialist | system"
        id: "provider-identity"

      dependencies: []
      priority: 50

      timing:
        timeout_ms: 300000
        estimated_duration_ms: 30000
        deadline: "ISO8601 (optional)"

      retry:
        max: 3
        backoff_ms: 5000
        strategy: "exponential | linear | immediate | jitter"
        max_backoff_ms: 60000

      resource_profile:
        cpu: "low | medium | high"
        memory: "low | medium | high"
        concurrency_key: "resource-pool-name"

      execution:
        type: "sync | async | fire-and-forget"
        cancel_strategy: "graceful | force | skip"

      success_criteria: []
      input_schema: {}
      output_schema: {}

      parallel_group: "group-name (optional)"
      checkpoint: true | false

    - id: "node-uuid-2"
      ...

  edges:
    - from: "node-uuid-1"
      to: "node-uuid-2"
      type: "depends_on | data_flow | control_flow | gate_result"
      data: {}  # Data passed along edge (for data_flow type)
      condition: "expression (for control_flow type)"
      label: "description of dependency"
```

---

### 5.2 DAG Lifecycle

Every DAG progresses through a formal lifecycle with defined states and transitions.

#### Lifecycle Diagram

```
DRAFT
  │ validate
  ▼
VALIDATED
  │ execute
  ▼
EXECUTING
  │
  ├── all nodes complete → COMPLETED
  ├── unrecoverable failure → FAILED
  └── user/system cancel → CANCELLED
```

#### Lifecycle State Definitions

| State | Description | Actions | Transitions To |
|-------|-------------|---------|----------------|
| **DRAFT** | DAG being constructed by Planner | Add/remove nodes, add/remove edges, set weights | VALIDATED |
| **VALIDATED** | DAG passed all validation checks | Calculate critical path, optimize order, allocate resources | EXECUTING |
| **EXECUTING** | Nodes being dispatched to workers | Dispatch nodes, monitor progress, handle completions/failures | COMPLETED, FAILED, CANCELLED |
| **COMPLETED** | All nodes executed successfully | Aggregate results, calculate quality score, emit completion event | — (terminal) |
| **FAILED** | Irrecoverable execution failure | Log failure, handle partial results, trigger recovery | — (terminal) |
| **CANCELLED** | Execution aborted by user/system | Cancel running nodes, clean up resources, emit cancellation event | — (terminal) |

#### Lifecycle Transition Guards

| Transition | Guard | Failure Action |
|-----------|-------|----------------|
| DRAFT → VALIDATED | All invariants G-01 to G-08 pass | Return to DRAFT with validation errors |
| VALIDATED → EXECUTING | All resources allocated, providers ready | Block until ready |
| EXECUTING → COMPLETED | All nodes in COMPLETED or SKIPPED state | Handle incomplete nodes as failures |
| EXECUTING → FAILED | Irrecoverable node failure after retries exhausted | Trigger recovery workflow |
| EXECUTING → CANCELLED | Cancel signal received (user or system) | Graceful shutdown of running nodes |

---

### 5.3 DAG Node Types

Every node in the DAG has a **type** that determines its execution contract.

#### Node Type Catalog

| Type | Symbol | Purpose | Execution Model | Output |
|------|--------|---------|----------------|--------|
| **capability** | ⚙ | Invoke a capability | Provider invokes capability contract | Capability output |
| **workflow** | 🔄 | Execute a sub-workflow | Workflow Engine executes sub-DAG | Workflow result |
| **task** | 📋 | Execute a specific task | Specialist executes task | Task artifacts |
| **review** | 🔍 | Perform review | Review Engine checks artifacts | Review score + issues |
| **quality** | ✅ | Run quality gates | Quality Engine runs gate checks | Gate pass/fail |
| **documentation** | 📝 | Update documentation | Documentation Engine generates docs | Updated documents |
| **decision** | ⚖️ | Make a decision | Organizational Layer decision | Decision record |
| **gate** | 🚧 | Conditional branching | Evaluate condition, route execution | True/False branch |
| **sync** | 🔄 | Synchronize state | Sync Engine writes to stores | Sync confirmation |
| **parallel_group** | ⚡ | Group nodes for parallel execution | Logical grouping, no execution | N/A (structural) |

#### Node Contract

Each node type has a formal contract:

```yaml
node_contract:
  capability:
    input: "Capability input per CAP-XXX-XXX contract"
    output: "Capability output per CAP-XXX-XXX contract"
    execution: "Invoke provider with capability_id"
    failure: "Retry per retry policy, then fail"

  workflow:
    input: "Workflow input parameters"
    output: "Workflow execution results"
    execution: "Load workflow definition, create sub-DAG"
    failure: "Sub-DAG failure → this node failure"

  task:
    input: "Task description, context, constraints"
    output: "Task artifacts, code, documentation"
    execution: "Assign to specialist via task tool"
    failure: "Retry per retry policy, escalate"

  review:
    input: "Artifacts to review, review type"
    output: "Review report, score, issues"
    execution: "Invoke Review Engine"
    failure: "Escalate to Review Chief"

  quality:
    input: "Artifacts, gate_id"
    output: "Gate result, score, violations"
    execution: "Invoke Quality Engine"
    failure: "Blocking gate failure → node fail"

  decision:
    input: "Decision question, options, context"
    output: "Decision record (ADR)"
    execution: "Route to Organizational Layer"
    failure: "Escalate to CTO"

  gate:
    input: "Condition expression, context"
    output: "True/False, with sub-DAG for each branch"
    execution: "Evaluate condition, activate matching branch"
    failure: "Condition error → node fail"

  sync:
    input: "Data to sync, target stores"
    output: "Sync confirmation per store"
    execution: "Invoke Sync Engine"
    failure: "Retry per retry policy, log warning"
```

---

### 5.4 DAG Edge Types

Edges represent dependencies between nodes. Different edge types enable different execution semantics.

#### Edge Type Catalog

| Type | Symbol | Semantics | Execution Effect | Visualization |
|------|--------|-----------|-----------------|---------------|
| **depends_on** | → | Hard dependency: B cannot start until A completes | B waits in dependency queue until A completes | Solid arrow |
| **data_flow** | ─▶ | A's output is B's input | Data passed from A's output to B's input | Dashed arrow |
| **control_flow** | ─◆ | B executes only if A's result matches condition | Conditional routing based on A's output | Diamond arrow |
| **gate_result** | ─● | Gate outcome determines next node | True branch or False branch activated | Colored arrow |
| **notification** | ─○ | A notifies B, B may start (non-blocking) | B receives notification, starts when ready | Dotted arrow |

#### Edge Rules

| Rule | Description | Violation |
|------|-------------|-----------|
| E-01 | Edge MUST connect existing nodes | Invalid node reference error |
| E-02 | Edge MUST NOT create cycle | Cycle detection error, node rejected |
| E-03 | depends_on edges MUST form a partial order | Topological sort failure |
| E-04 | data_flow edges MUST have compatible schemas | Schema mismatch error |
| E-05 | control_flow edges MUST have deterministic conditions | Non-determinism warning |
| E-06 | gate_result edges MUST originate from gate nodes | Edge type violation |
| E-07 | Node MAY have multiple incoming edges (AND join) | N/A |
| E-08 | Node MAY have multiple outgoing edges (AND split) | N/A |

---

### 5.5 DAG Validation Rules

Before execution, every DAG MUST pass formal validation.

#### Validation Rule Set

```
VALIDATION SET V-01..V-20

Structural:
V-01  Graph is acyclic (DFS with back-edge detection)
V-02  Exactly one source node exists
V-03  At least one sink node exists
V-04  All nodes reachable from source (BFS connectivity)
V-05  All nodes reachable to sink (reverse BFS connectivity)
V-06  No orphan nodes (disconnected subgraphs)
V-07  Node IDs are unique

Semantic:
V-08  All capability nodes reference valid capabilities (from registry)
V-09  All provider references are valid (from ORGCHART)
V-10  All data_flow edges have compatible input/output schemas
V-11  All gate conditions are syntactically valid
V-12  All timeouts are positive integers
V-13  All retry policies have valid parameters
V-14  No circular retry dependencies

Resource:
V-15  Total estimated resources within session limits
V-16  No single resource pool exceeded
V-17  Concurrency limits respected per parallel group

Performance:
V-18  Critical path length within acceptable range
V-19  Estimated total duration within session timeout
V-20  Parallel groups balanced (no group >> others)
```

#### Validation Execution

```yaml
dag_validation:
  trigger: "DRAFT → VALIDATED transition"

  execution:
    structural: "O(V + E) topological sort + BFS"
    semantic: "O(V) registry lookups + schema checks"
    resource: "O(V) resource summation"
    performance: "O(V + E) critical path calculation"

  output:
    on_pass:
      status: "VALIDATED"
      metrics:
        validation_duration_ms: 45
        checks_passed: 20
        critical_path: ["node-1", "node-3", "node-7", "node-12"]
        critical_path_duration_ms: 45000
    on_fail:
      status: "DRAFT"
      violations: []
      block_execution: true
```

---

### 5.6 DAG Execution Model

Once validated, the DAG is executed through a formal dispatch cycle.

#### Execution Cycle

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        DAG EXECUTION CYCLE                               │
│                                                                         │
│  For each execution tick (100ms):                                       │
│                                                                         │
│  1. IDENTIFY READY NODES                                                │
│     - Find all nodes where all dependencies are COMPLETED               │
│     - Filter by priority (higher = first)                               │
│     - Filter by resource availability                                   │
│                                                                         │
│  2. DISPATCH READY NODES                                                │
│     - For each ready node:                                              │
│       a. Allocate resources (CPU, memory, concurrency slot)             │
│       b. Mark node status = DISPATCHING                                 │
│       c. Assign to worker (chief, engine, specialist)                   │
│       d. Set execution deadline = now + timeout_ms                      │
│       e. Publish StepStarted event                                      │
│       f. Mark node status = RUNNING                                     │
│                                                                         │
│  3. MONITOR RUNNING NODES                                               │
│     - For each RUNNING node:                                            │
│       a. Check health (heartbeat, timeout)                              │
│       b. On heartbeat miss: flag for investigation                      │
│       c. On timeout: trigger retry or failure                           │
│       d. On completion: process result                                  │
│                                                                         │
│  4. PROCESS COMPLETED NODES                                             │
│     - For each COMPLETED node:                                          │
│       a. Validate output against output_schema                          │
│       b. Pass data to dependent nodes via data_flow edges               │
│       c. Evaluate control_flow/gate_result edges                        │
│       d. Mark node status = COMPLETED                                   │
│       e. Publish StepCompleted event                                    │
│       f. Release allocated resources                                    │
│                                                                         │
│  5. HANDLE FAILED NODES                                                 │
│     - For each FAILED node:                                             │
│       a. Check retry policy                                             │
│       b. If retries remaining: mark as RETRYING, schedule retry         │
│       c. If retries exhausted: mark as FAILED                           │
│       d. Cancel dependent nodes (per cancel_strategy)                   │
│       e. Check if DAG should fail or continue with degraded results     │
│                                                                         │
│  6. CHECK COMPLETION                                                    │
│     - If all nodes COMPLETED or SKIPPED: transition DAG to COMPLETED    │
│     - If any node FAILED and no recovery: transition DAG to FAILED      │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

#### Node Status Lifecycle

```
PENDING
  │ dependencies met
  ▼
READY
  │ resource allocated
  ▼
DISPATCHING
  │ worker assigned
  ▼
RUNNING
  │
  ├── completed successfully → COMPLETED
  ├── timeout/error → FAILED
  │     │
  │     ├── retries remaining → RETRYING → READY
  │     └── retries exhausted → FAILED (permanent)
  │
  └── cancelled → CANCELLED
```

---

### 5.7 DAG Parallelism Model

The Scheduler determines **which nodes execute in parallel** based on the DAG structure.

#### Parallelism Rules

```
Parallelism Rule P-01:
  Nodes with no transitive dependency MAY execute in parallel

Parallelism Rule P-02:
  Nodes in the same parallel_group execute concurrently when ready

Parallelism Rule P-03:
  Nodes with different resource profiles MAY share a parallel group

Parallelism Rule P-04:
  Maximum parallel nodes = min(available_workers, concurrency_limit)

Parallelism Rule P-05:
  Priority inversion: higher-priority nodes MAY preempt lower-priority nodes

Parallelism Rule P-06:
  Data_flow edges create implicit ordering (consumer waits for producer)

Parallelism Rule P-07:
  Parallel groups are bounded by resource pool limits
```

#### Parallelism Decision Algorithm

```yaml
parallelism_algorithm:
  step_1: "Build transitive dependency closure"
  step_2: "Identify independent node sets"
  step_3: "Group by parallel_group label"
  step_4: "Check resource availability per group"
  step_5: "Apply priority ordering within group"
  step_6: "Dispatch up to concurrency_limit nodes"
  step_7: "Monitor utilization, adjust dispatch rate"

  complexity: "O(V + E) per tick"

  re_evaluation: "Every 100ms or on node completion"
```

#### Concurrency Limits

| Resource | Per-Session Limit | Per-Group Limit | Per-Node Limit | Overflow Behavior |
|----------|------------------|-----------------|----------------|-------------------|
| CPU cores | 8 | 4 | 2 | Queue, throttle |
| Memory (GB) | 16 | 8 | 4 | Queue, degrade |
| Worker slots | 20 | 10 | 1 | Queue |
| Network connections | 50 | 25 | 5 | Queue |
| Database connections | 10 | 5 | 1 | Queue |

---

### 5.8 DAG Priority & Preemption

#### Priority Levels (Node-Level)

| Level | Value | Meaning | Preemption Allowed? | Queue |
|-------|-------|---------|--------------------|-------|
| **CRITICAL** | 100 | Must execute immediately | Yes (preempts any) | Immediate |
| **HIGH** | 75 | User-facing, time-sensitive | Yes (preempts STANDARD and below) | Priority |
| **STANDARD** | 50 | Normal execution priority | No (waits for resources) | Standard |
| **LOW** | 25 | Background processing | No | Background |
| **DEFERRED** | 0 | Only when idle | No | Background |

#### Priority Inversion Prevention

```yaml
priority_inversion:
  detection:
    - "Low-priority node holds resource needed by high-priority node"

  prevention:
    method: "Priority inheritance"
    mechanism: "Temporarily raise low-priority node's priority to blocking node's priority"

  resolution:
    - "Resource preemption (if cancel_strategy = force)"
    - "Resource escalation (add temporary capacity)"
    - "Deadline extension for dependent high-priority nodes"
```

#### Preemption Rules

```
Preemption PR-01:
  CRITICAL nodes MAY preempt any running node

Preemption PR-02:
  HIGH nodes MAY preempt STANDARD and below

Preemption PR-03:
  Preemption requires cancel_strategy = "force" on preempted node

Preemption PR-04:
  Preempted node is re-queued to READY state (not restarted from scratch)

Preemption PR-05:
  Preemption is logged in node history

Preemption PR-06:
  Excessive preemption (> 3 per node) escalates to scheduler warning
```

---

### 5.9 DAG Failure Handling

#### Failure Types

| Failure Type | Cause | Detection | Recovery |
|-------------|-------|-----------|----------|
| **Timeout** | Node exceeds timeout_ms | Watchdog timer | Retry (if retries remain) |
| **Execution error** | Provider returns error | Status code | Retry or escalate |
| **Resource exhaustion** | Out of memory, CPU, disk | Resource monitor | Queue or degrade |
| **Provider unavailable** | Provider down or unreachable | Health check | Failover or retry |
| **Contract violation** | Output does not match schema | Schema validation | Reject, fail node |
| **Data dependency fail** | Upstream node failed, data missing | Dependency check | Propagate failure |
| **Cancellation** | User/system cancels node | Cancel signal | Graceful shutdown |

#### Cascading Failure Rules

```
Cascading CF-01:
  If node A fails, all nodes that directly depend on A are blocked

Cascading CF-02:
  Blocked nodes with depends_on A → enter BLOCKED state

Cascading CF-03:
  Blocked nodes with depends_on + data_flow from A → enter FAILED state (data unavailable)

Cascading CF-04:
  Blocked nodes with control_flow from A → follow the non-failing branch

Cascading CF-05:
  If a critical path node fails, entire DAG fails (unless alternative path exists)

Cascading CF-06:
  Non-critical path failures MAY result in degraded completion
```

#### Partial Completion Policy

```yaml
partial_completion:
  enabled: true
  scope: "Non-critical nodes only"

  conditions:
    - "Failed node is not on critical path"
    - "Failed node output is not required by any unexecuted node"
    - "Quality score with partial results >= minimum threshold"

  actions:
    - "Mark failed node as SKIPPED (not FAILED)"
    - "Log partial completion warning"
    - "Adjust final quality score downward"
    - "Include skipped nodes in delivery report"

  on_reject:
    - "If conditions not met → DAG FAILED normally"
    - "Escalate to CTO for manual override"
```

---

### 5.10 DAG Cancellation

#### Cancellation Levels

| Level | Trigger | Action | Data Loss |
|-------|---------|--------|-----------|
| **Graceful** | User cancel, workflow abort | Allow running nodes to complete, block new dispatches | None |
| **Force** | Timeout, system overload | Terminate running nodes immediately | Partial results lost |
| **Rollback** | Data integrity violation | Cancel + revert completed nodes (compensating actions) | None (compensated) |

#### Cancellation Flow

```yaml
cancellation_flow:
  step_1: "Receive cancel signal (graceful | force | rollback)"
  step_2: "Publish DAGCancelling event"
  step_3: "Stop dispatching new nodes"
  step_4: "For each RUNNING node:"
    graceful: "Send cancel notification, wait for completion (max 30s)"
    force: "Terminate immediately"
    rollback: "Terminate + execute compensating action"
  step_5: "Cancel all READY and PENDING nodes"
  step_6: "Release all allocated resources"
  step_7: "Persist DAG state (COMPLETED nodes preserved)"
  step_8: "Publish DAGCancelled event"
  step_9: "Return partial results (if any)"
```

---

### 5.11 DAG Monitoring & Observability

#### Per-Node Metrics

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `dag.node.duration_ms` | Histogram | node_type, capability, provider | Time in RUNNING state |
| `dag.node.queue_time_ms` | Histogram | node_type, priority | Time from READY to DISPATCHING |
| `dag.node.retry.count` | Counter | node_type | Retries per node |
| `dag.node.status` | Gauge | node_id | Current status (1=RUNNING, 0=other) |
| `dag.node.resource.usage` | Gauge | resource_type | Resource consumption |
| `dag.node.dispatch.count` | Counter | node_type | Nodes dispatched |

#### Per-DAG Metrics

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `dag.execution.duration_ms` | Histogram | dag_status | Total DAG execution time |
| `dag.execution.parallelism` | Gauge | — | Peak parallel nodes |
| `dag.execution.critical_path_ms` | Histogram | — | Critical path duration |
| `dag.execution.total_nodes` | Histogram | — | Total nodes in DAG |
| `dag.execution.completion_rate` | Gauge | — | Completed / Total |
| `dag.execution.failure_rate` | Gauge | — | Failed / Total |
| `dag.validation.duration_ms` | Histogram | — | Validation time |
| `dag.validation.violations` | Counter | severity | Validation violations |

#### DAG Health Dashboard

```yaml
dag_health_dashboard:
  sections:
    - name: "Execution Progress"
      widgets:
        - "Progress bar: completed / total nodes"
        - "Timeline: running nodes with ETA"
        - "Status: COMPLETED | FAILED | CANCELLED"

    - name: "Parallelism"
      widgets:
        - "Gauge: current parallel count / max"
        - "Timeline: parallelism over time"
        - "Histogram: node duration by type"

    - name: "Critical Path"
      widgets:
        - "Timeline: critical path nodes with durations"
        - "Gauge: critical path remaining / total"
        - "Alert: if critical path duration > estimate"

    - name: "Failures & Retries"
      widgets:
        - "List: failed nodes with error details"
        - "Counter: retries by node"
        - "Alert: if failure rate > threshold"
```

---

### 5.12 DAG Optimization

#### Critical Path Analysis

```yaml
critical_path_analysis:
  algorithm: "Forward-backward pass (O(V + E))"

  outputs:
    critical_path: ["node-1", "node-3", "node-7", "node-12"]
    critical_path_duration_ms: 45000
    slack_per_node:
      "node-1": 0       # On critical path
      "node-2": 5000     # 5s slack
      "node-3": 0       # On critical path
      "node-7": 0       # On critical path

  uses:
    - "Identify optimization candidates (nodes with high duration on critical path)"
    - "Resource reallocation (add resources to critical path nodes)"
    - "Deadline estimation (is estimated completion within limits?)"
    - "Risk assessment (what if critical path node fails?)"
```

#### Topological Sort

```yaml
topological_sort:
  algorithm: "Kahn's algorithm (BFS-based, O(V + E))"

  guarantees:
    - "Produces valid execution order respecting all dependencies"
    - "Preserves parallel_group boundaries"
    - "Maximizes parallelism (nodes ready earlier are scheduled first)"

  tie_breaking:
    primary: "Priority (higher = earlier)"
    secondary: "Estimated duration (shorter = earlier)"
    tertiary: "Node ID (deterministic ordering)"
```

#### Resource Leveling

```yaml
resource_leveling:
  purpose: "Avoid resource contention by shifting node start times"

  algorithm: "Priority-based resource leveling"

  constraints:
    - "Cannot violate dependency order"
    - "Cannot exceed resource pool limits"
    - "Cannot delay critical path"

  optimization:
    target: "Minimize peak resource usage"
    method: "Delay non-critical nodes within slack"
    result: "Smoothed resource utilization profile"
```

---

### 5.13 DAG Persistence & Recovery

#### Persistence Schema

```yaml
dag_persistence:
  storage:
    active_dag: "Redis (in-memory, fast access)"
    completed_dag: "Database (durable, queryable)"
    archived_dag: "Object storage (long-term)"

  schema:
    dag_id: "uuid (partition key)"
    session_id: "uuid"
    plan_id: "uuid"
    status: "string"
    nodes: "JSON (serialized node array)"
    edges: "JSON (serialized edge array)"
    metrics: "JSON"
    created_at: "timestamp"
    completed_at: "timestamp (nullable)"

  persistence_trigger:
    - "On every node status change"
    - "On DAG status change"
    - "Every 60 seconds (heartbeat checkpoint)"

  recovery:
    on_crash: "Load active DAG from Redis, resume execution"
    on_node_fail: "Re-queue node from last checkpoint"
    on_dag_fail: "Reload DAG, evaluate retry/restart"
```

#### DAG Serialization (for Dashboard)

```json
{
  "dag_id": "uuid",
  "status": "executing",
  "nodes": [
    {
      "id": "node-1",
      "type": "capability",
      "name": "Backend API Development",
      "status": "completed",
      "duration_ms": 45000,
      "dependencies": [],
      "position": {"x": 100, "y": 200}
    },
    {
      "id": "node-2",
      "type": "review",
      "name": "Code Review",
      "status": "running",
      "duration_ms": 12000,
      "dependencies": ["node-1"],
      "position": {"x": 300, "y": 200}
    }
  ],
  "edges": [
    {"from": "node-1", "to": "node-2", "type": "depends_on"}
  ],
  "metrics": {
    "progress": 0.45,
    "parallelism": 3,
    "critical_path_remaining_ms": 35000
  }
}
```

---

### 5.14 Multi-DAG Coordination

Multiple DAGs may execute concurrently within a session or across sessions.

#### Coordination Rules

```
Multi-DAG Rule M-01:
  Each DAG is independent (separate ID, separate execution)

Multi-DAG Rule M-02:
  DAGs MAY share resources through the global resource pool

Multi-DAG Rule M-03:
  DAGs MAY have inter-DAG dependencies (DAG-B waits for DAG-A)

Multi-DAG Rule M-04:
  Inter-DAG dependencies are implemented via Event Bus events

Multi-DAG Rule M-05:
  Maximum concurrent DAGs per session: 5

Multi-DAG Rule M-06:
  Maximum concurrent DAGs globally: 50 (configurable)

Multi-DAG Rule M-07:
  DAG priority applies globally (higher-priority DAG nodes dispatched first)
```

#### Inter-DAG Dependency

```yaml
inter_dag_dependency:
  type: "Event-based"

  mechanism:
    - "DAG-A publishes DAGCompleted event"
    - "DAG-B subscribes to DAG-A completion"
    - "On event: DAG-B dependency resolved, DAG-B starts"

  schema:
    waiting_dag: "dag-uuid-b"
    waiting_node: "node-uuid"
    depends_on_dag: "dag-uuid-a"
    depends_on_event: "DAGCompleted"
```

---

### 5.15 DAG Schema Versioning

The DAG schema evolves over time. All versions are tracked.

```yaml
dag_schema_versioning:
  current_version: "2.0"

  changelog:
    "1.0":
      - "Initial DAG schema"
      - "Nodes: id, type, capability, provider, dependencies, priority"
      - "Edges: from, to, type"

    "2.0":
      - "Added resource_profile to nodes"
      - "Added data_flow and control_flow edge types"
      - "Added success_criteria to nodes"
      - "Added parallel_group for explicit parallelism"
      - "Added checkpoint flag for recovery points"

  compatibility:
    forward: "v2.0 runtimes can execute v1.0 DAGs (with defaults for new fields)"
    backward: "v1.0 runtimes cannot execute v2.0 DAGs"

  migration:
    trigger: "On planner upgrade"
    action: "Update DAG template to latest version"
    rollback: "Revert to previous version template"
```

