# Runtime Contracts — Extracted from KERNEL.md §7

> **Source**: KERNEL.md v3.0.1 §7 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 7. RUNTIME CONTRACTS

Every component MUST respect formal contracts. Contracts define **inputs, outputs, states, events, quality criteria, and behaviors** for every interaction in the Runtime. A contract is a **legally binding interface** — violations are detected, logged, and escalated.

---

### 7.1 Contract Governance

#### Contract Ownership

| Contract | Owner | Approver | Review Cycle |
|----------|-------|----------|-------------|
| Input Contract | Runtime Chief | CTO | Quarterly |
| Output Contract | Runtime Chief | CTO | Quarterly |
| State Contract | Runtime Chief | Architecture Chief | Quarterly |
| Event Contract | Event Bus Owner | CTO | Quarterly |
| Capability Contract | Capability Engine | CTO + Provider Chief | Monthly |
| Workflow Contract | Workflow Chief | CTO | Monthly |
| Execution Contract | Execution Engine | Architecture Chief | Monthly |
| Quality Contract | QA Chief | CTO | Monthly |
| Memory Contract | Memory Chief | CTO | Quarterly |
| Provider Contract | AI Chief | CTO | Monthly |
| Plugin Contract | Workflow Chief | CTO | Quarterly |
| API Contract | API Owner | CTO + Security Chief | Monthly |
| Health Contract | Monitoring Chief | CTO | Quarterly |

#### Contract Lifecycle

```
DRAFT → REVIEW → ACTIVE → DEPRECATED → RETIRED

DRAFT:       Under development, not enforced
REVIEW:      Under review by owner and approver
ACTIVE:      Enforced at runtime, fully supported
DEPRECATED:  Still enforced, migration notice issued (min 30 days)
RETIRED:     No longer enforced, contract archived
```

#### Contract Versioning

All contracts follow SemVer independently:

```yaml
contract_versioning:
  major: "Breaking changes to contract fields or validation rules"
  minor: "New optional fields or new validation rules"
  patch: "Clarifications, examples, documentation fixes"

  compatibility:
    backward: "New consumer can process old producer data"
    forward: "Old consumer can process new producer data"

  changelog:
    required: true
    location: "Each contract's version history"
```

---

### 7.2 Contract Schema (Standard Template)

Every contract follows this standard template:

```yaml
contract:
  id: "contract-uuid"
  name: "Contract Name"
  version: "1.0.0"
  status: "draft | active | deprecated | retired"

  type: "input | output | state | event | capability | workflow | execution | quality | memory | provider | plugin | api | health"

  scope: "component-or-system"

  owner: "Chief or Engine"
  approver: "CTO or Architecture Chief"

  description:
    purpose: "What this contract governs"
    applicability: "Which components must implement this"
    exclusions: "Which scenarios are exempt"

  schema:
    format: "jsonschema | yaml | protobuf | openapi"
    definition: {}  # Full schema definition

  validation:
    rules: []
    severity: "error | warn"
    enforcement: "runtime | static | periodic"

  constraints:
    timeout_ms: 5000
    max_retries: 3
    required_for: ["gate-1", "gate-2"]

  compatibility:
    backward: true
    forward: false

  examples:
    - description: "Example 1"
      input: {}
      output: {}

  changelog:
    - version: "1.0.0"
      date: "2026-07-15"
      changes: ["Initial release"]
```

---

### 7.3 Contract Catalog (13 Contracts)

Each of the 13 Runtime contracts is fully defined below with its schema, validation rules, and enforcement mechanisms.

---

#### 7.3.1 Input Contract

Governs all inputs entering any Runtime component.

```yaml
contract_input:
  id: "CTR-IN-001"
  name: "Input Contract"
  version: "1.0.0"
  type: "input"

  schema:
    format: "jsonschema"
    rules:
      - "All inputs MUST have a type field"
      - "All inputs MUST have a payload field"
      - "All inputs MUST have a metadata block with session_id"
      - "Input size MUST NOT exceed 1MB"

  validation_rules:
    - rule: "IN-01"
      description: "Input MUST contain valid session_id"
      severity: "error"
      enforcement: "runtime"

    - rule: "IN-02"
      description: "Input type MUST be recognized"
      severity: "error"
      enforcement: "runtime"

    - rule: "IN-03"
      description: "Input payload MUST match expected schema"
      severity: "error"
      enforcement: "runtime"

    - rule: "IN-04"
      description: "Input MUST NOT contain executable code"
      severity: "error"
      enforcement: "static"

    - rule: "IN-05"
      description: "Input size SHOULD be < 100KB for real-time operations"
      severity: "warn"
      enforcement: "runtime"

    - rule: "IN-06"
      description: "Input MUST have explicit encoding (UTF-8)"
      severity: "error"
      enforcement: "runtime"

  examples:
    valid:
      type: "feature_request"
      payload: { "description": "Add user authentication", "priority": "high" }
      metadata: { "session_id": "uuid-123", "timestamp": "ISO8601" }
    invalid:
      type: "unknown_type"
      payload: "<script>exec()</script>"
      metadata: {}
```

#### 7.3.2 Output Contract

Governs all outputs produced by any Runtime component.

```yaml
contract_output:
  id: "CTR-OUT-001"
  name: "Output Contract"
  version: "1.0.0"
  type: "output"

  schema:
    format: "jsonschema"
    rules:
      - "All outputs MUST have a status field"
      - "All outputs MUST have a result or error field"
      - "Output size MUST NOT exceed 10MB"

  validation_rules:
    - rule: "OUT-01"
      description: "Output MUST contain status (success | failure | partial)"
      severity: "error"
      enforcement: "runtime"

    - rule: "OUT-02"
      description: "On failure, output MUST include error details"
      severity: "error"
      enforcement: "runtime"

    - rule: "OUT-03"
      description: "Output SHOULD include duration_ms"
      severity: "warn"
      enforcement: "runtime"

    - rule: "OUT-04"
      description: "Output MUST NOT contain secrets or credentials"
      severity: "error"
      enforcement: "static"

    - rule: "OUT-05"
      description: "Output schema MUST match capability contract"
      severity: "error"
      enforcement: "runtime"

  examples:
    success:
      status: "success"
      result: { "data": {}, "summary": "Task completed" }
      metadata: { "duration_ms": 45000 }
    failure:
      status: "failure"
      error: { "code": "TIMEOUT", "message": "Execution exceeded timeout", "details": {} }
      metadata: { "duration_ms": 300000 }
```

#### 7.3.3 State Contract

Governs the Runtime State Machine contract.

```yaml
contract_state:
  id: "CTR-ST-001"
  name: "State Contract"
  version: "1.0.0"
  type: "state"

  schema:
    format: "yaml"
    rules:
      - "State MUST be one of 14 defined states"
      - "Transitions MUST follow transition matrix"
      - "State MUST be persisted on every transition"

  validation_rules:
    - rule: "ST-01"
      description: "Current state MUST be valid (in S set)"
      severity: "error"
      enforcement: "runtime"

    - rule: "ST-02"
      description: "Transition MUST exist in transition matrix"
      severity: "error"
      enforcement: "runtime"

    - rule: "ST-03"
      description: "Guard condition MUST pass before transition"
      severity: "error"
      enforcement: "runtime"

    - rule: "ST-04"
      description: "State MUST NOT remain in non-terminal state beyond timeout"
      severity: "error"
      enforcement: "runtime (watchdog)"

    - rule: "ST-05"
      description: "State persistence MUST complete within 100ms"
      severity: "warn"
      enforcement: "runtime"

  references:
    - "RUNTIME STATE MACHINE §3"
    - "Transition Matrix §3.3"
    - "State Guards §3.4"
```

#### 7.3.4 Event Contract

Governs all events published to the Event Bus.

```yaml
contract_event:
  id: "CTR-EV-001"
  name: "Event Contract"
  version: "1.0.0"
  type: "event"

  schema:
    format: "jsonschema"
    rules:
      - "All events MUST use standard envelope"
      - "All events MUST have unique ID (UUID v7)"
      - "All events MUST have correlation_id"
      - "Event payload MUST match registered schema"

  validation_rules:
    - rule: "EV-01"
      description: "Event MUST conform to Standard Event Envelope"
      severity: "error"
      enforcement: "runtime (schema registry)"

    - rule: "EV-02"
      description: "Event name MUST be registered in Event Catalog"
      severity: "error"
      enforcement: "runtime"

    - rule: "EV-03"
      description: "Publisher MUST be authorized for topic"
      severity: "error"
      enforcement: "runtime (RBAC)"

    - rule: "EV-04"
      description: "Event payload MUST NOT exceed 1MB"
      severity: "error"
      enforcement: "runtime"

    - rule: "EV-05"
      description: "Event SHOULD include causation_id for traceability"
      severity: "warn"
      enforcement: "runtime"

    - rule: "EV-06"
      description: "Event schema MUST be backward compatible"
      severity: "error"
      enforcement: "schema registry"

  references:
    - "EVENT DRIVEN ARCHITECTURE §4"
    - "Event Schema §4.4"
    - "Schema Registry §4.4"
```

#### 7.3.5 Capability Contract

Governs capability definitions and resolution.

```yaml
contract_capability:
  id: "CTR-CP-001"
  name: "Capability Contract"
  version: "1.0.0"
  type: "capability"

  schema:
    format: "yaml"
    rules:
      - "All capabilities MUST have unique ID (CAP-XXX-NNN)"
      - "All capabilities MUST have defined input/output contracts"
      - "All capabilities MUST have a designated provider"
      - "All capabilities MUST have quality criteria"

  validation_rules:
    - rule: "CP-01"
      description: "Capability ID MUST be unique across registry"
      severity: "error"
      enforcement: "registration"

    - rule: "CP-02"
      description: "Capability MUST have valid provider reference"
      severity: "error"
      enforcement: "registration"

    - rule: "CP-03"
      description: "Capability input/output MUST match provider contract"
      severity: "error"
      enforcement: "registration"

    - rule: "CP-04"
      description: "Capability dependencies MUST exist in registry"
      severity: "error"
      enforcement: "registration"

    - rule: "CP-05"
      description: "Capability MUST meet minimum quality score (5.0)"
      severity: "warn"
      enforcement: "periodic"

    - rule: "CP-06"
      description: "Capability SLOs MUST be achievable"
      severity: "warn"
      enforcement: "registration"

  references:
    - "CAPABILITY FIRST ARCHITECTURE §2"
    - "Capability Contract Schema §2.1"
    - "CAPABILITY_CATALOG.md"
    - "CAPABILITY_TEMPLATE.md"
```

#### 7.3.6 Workflow Contract

Governs workflow definitions and execution.

```yaml
contract_workflow:
  id: "CTR-WF-001"
  name: "Workflow Contract"
  version: "1.0.0"
  type: "workflow"

  schema:
    format: "yaml"
    rules:
      - "Workflow MUST have defined steps with clear inputs/outputs"
      - "Workflow MUST have success criteria"
      - "Workflow MUST reference valid capabilities for each step"

  validation_rules:
    - rule: "WF-01"
      description: "Workflow steps MUST form a valid sequence"
      severity: "error"
      enforcement: "registration"

    - rule: "WF-02"
      description: "Each step MUST have at least one input and one output"
      severity: "error"
      enforcement: "registration"

    - rule: "WF-03"
      description: "Workflow MUST have at least one success criterion"
      severity: "error"
      enforcement: "registration"

    - rule: "WF-04"
      description: "Step capabilities MUST exist in capability registry"
      severity: "error"
      enforcement: "registration"

    - rule: "WF-05"
      description: "Workflow MUST NOT exceed 20 steps"
      severity: "warn"
      enforcement: "registration"

  references:
    - "WORKFLOW ROUTING §24"
    - "Workflows/ directory"
    - "Workflow Engine"
```

#### 7.3.7 Execution Contract

Governs DAG execution and node dispatching.

```yaml
contract_execution:
  id: "CTR-EX-001"
  name: "Execution Contract"
  version: "1.0.0"
  type: "execution"

  schema:
    format: "yaml"
    rules:
      - "All execution nodes MUST have unique IDs"
      - "All nodes MUST have timeout settings"
      - "All nodes MUST have retry policy"
      - "Execution DAG MUST be acyclic"

  validation_rules:
    - rule: "EX-01"
      description: "DAG MUST pass validation (V-01 to V-20)"
      severity: "error"
      enforcement: "pre-execution"

    - rule: "EX-02"
      description: "Node timeout MUST be between 1s and 300s"
      severity: "error"
      enforcement: "pre-execution"

    - rule: "EX-03"
      description: "Retry count MUST NOT exceed 10"
      severity: "error"
      enforcement: "pre-execution"

    - rule: "EX-04"
      description: "Node output MUST match output_schema"
      severity: "error"
      enforcement: "post-execution"

    - rule: "EX-05"
      description: "Execution MUST not exceed session timeout"
      severity: "error"
      enforcement: "runtime"

  references:
    - "EXECUTION GRAPH (DAG) §5"
    - "DAG Node Definition §5.1"
    - "Execution Cycle §5.6"
```

#### 7.3.8 Quality Contract

Governs quality gate definitions and enforcement.

```yaml
contract_quality:
  id: "CTR-QL-001"
  name: "Quality Contract"
  version: "1.0.0"
  type: "quality"

  schema:
    format: "yaml"
    rules:
      - "Each gate MUST have defined checks with thresholds"
      - "Each check MUST have severity (error | warn)"
      - "Quality score MUST be calculable via defined formula"

  validation_rules:
    - rule: "QL-01"
      description: "All gates (0-4) MUST be implemented"
      severity: "error"
      enforcement: "audit"

    - rule: "QL-02"
      description: "Each gate check MUST have measurable threshold"
      severity: "error"
      enforcement: "registration"

    - rule: "QL-03"
      description: "Quality score MUST be between 0.0 and 10.0"
      severity: "error"
      enforcement: "runtime"

    - rule: "QL-04"
      description: "Gate failures MUST be logged with details"
      severity: "error"
      enforcement: "runtime"

    - rule: "QL-05"
      description: "Quality checks SHOULD complete within 30s total"
      severity: "warn"
      enforcement: "runtime"

  references:
    - "QUALITY_GATES.md"
    - "Quality Gates §10"
    - "Quality Engine"
```

#### 7.3.9 Memory Contract

Governs memory operations and storage.

```yaml
contract_memory:
  id: "CTR-ME-001"
  name: "Memory Contract"
  version: "1.0.0"
  type: "memory"

  schema:
    format: "yaml"
    rules:
      - "All memory records MUST have YAML frontmatter"
      - "All records MUST have type, key, timestamp, status"
      - "Records MUST be stored in correct location per MEMORY_MODEL.md"

  validation_rules:
    - rule: "ME-01"
      description: "Memory type MUST be valid (short, long, project, architecture, decision, pattern, bug, agent)"
      severity: "error"
      enforcement: "write"

    - rule: "ME-02"
      description: "Memory key MUST be unique within store"
      severity: "error"
      enforcement: "write"

    - rule: "ME-03"
      description: "Memory MUST have valid timestamp (ISO8601)"
      severity: "error"
      enforcement: "write"

    - rule: "ME-04"
      description: "Memory size MUST NOT exceed 1MB per record"
      severity: "warn"
      enforcement: "write"

    - rule: "ME-05"
      description: "Memory store path MUST match MEMORY_MODEL.md"
      severity: "error"
      enforcement: "write"

  references:
    - "MEMORY_MODEL.md"
    - "Memory Management §12"
    - "Memory Engine"
```

#### 7.3.10 Provider Contract

Governs AI provider integration and failover.

```yaml
contract_provider:
  id: "CTR-PR-001"
  name: "Provider Contract"
  version: "1.0.0"
  type: "provider"

  schema:
    format: "yaml"
    rules:
      - "Provider MUST implement standard provider interface"
      - "Provider MUST support health checks"
      - "Provider MUST support timeout and retry"

  validation_rules:
    - rule: "PR-01"
      description: "Provider MUST respond to health check within 5s"
      severity: "error"
      enforcement: "periodic"

    - rule: "PR-02"
      description: "Provider latency MUST NOT exceed 30s p99"
      severity: "error"
      enforcement: "runtime"

    - rule: "PR-03"
      description: "Provider error rate MUST be below 5%"
      severity: "error"
      enforcement: "runtime"

    - rule: "PR-04"
      description: "Provider MUST support circuit breaker pattern"
      severity: "error"
      enforcement: "registration"

    - rule: "PR-05"
      description: "Provider MUST have defined failover strategy"
      severity: "error"
      enforcement: "registration"

    - rule: "PR-06"
      description: "Provider MUST log all requests and responses"
      severity: "error"
      enforcement: "audit"

  references:
    - "PROVIDER_INTERFACE.md"
    - "Runtime Health §8"
    - "Provider Failover"
```

#### 7.3.11 Plugin Contract

Governs plugin integration and lifecycle.

```yaml
contract_plugin:
  id: "CTR-PL-001"
  name: "Plugin Contract"
  version: "1.0.0"
  type: "plugin"

  schema:
    format: "yaml"
    rules:
      - "Plugin MUST declare its hook points"
      - "Plugin MUST declare its permissions"
      - "Plugin MUST declare its dependencies"

  validation_rules:
    - rule: "PL-01"
      description: "Plugin MUST register with Plugin Registry"
      severity: "error"
      enforcement: "registration"

    - rule: "PL-02"
      description: "Plugin MUST NOT exceed allocated permissions"
      severity: "error"
      enforcement: "runtime"

    - rule: "PL-03"
      description: "Plugin MUST handle timeout gracefully"
      severity: "error"
      enforcement: "runtime"

    - rule: "PL-04"
      description: "Plugin MUST NOT block Kernel operations"
      severity: "error"
      enforcement: "runtime"

    - rule: "PL-05"
      description: "Plugin lifecycle MUST follow start → ready → stop"
      severity: "warn"
      enforcement: "registration"

  references:
    - "Plugin Contract §7.3.11"
    - "Plugin System"
```

#### 7.3.12 API Contract

Governs the REST API exposed by the Runtime.

```yaml
contract_api:
  id: "CTR-API-001"
  name: "API Contract"
  version: "1.0.0"
  type: "api"

  schema:
    format: "openapi 3.0"
    rules:
      - "All endpoints MUST be documented with OpenAPI"
      - "All endpoints MUST have auth requirements"
      - "All responses MUST include status codes"

  validation_rules:
    - rule: "API-01"
      description: "All endpoints MUST require authentication"
      severity: "error"
      enforcement: "runtime"

    - rule: "API-02"
      description: "All endpoints MUST validate input (400 on invalid)"
      severity: "error"
      enforcement: "runtime"

    - rule: "API-03"
      description: "All responses MUST include CORS headers"
      severity: "error"
      enforcement: "runtime"

    - rule: "API-04"
      description: "API response time MUST be < 500ms p95"
      severity: "warn"
      enforcement: "monitoring"

    - rule: "API-05"
      description: "API MUST rate-limit per session (100 req/min)"
      severity: "error"
      enforcement: "runtime"

    - rule: "API-06"
      description: "API version MUST be included in URL path (/api/v1/)"
      severity: "error"
      enforcement: "registration"

  endpoints:
    - "/api/v1/status"
    - "/api/v1/health"
    - "/api/v1/state"
    - "/api/v1/metrics"
    - "/api/v1/memory"
    - "/api/v1/config"
    - "/api/v1/capabilities"
    - "/api/v1/workflows"
    - "/api/v1/events"
    - "/api/v1/events/stream"
    - "/api/v1/ws"
    - "/api/v1/command"

  references:
    - "Dashboard Integration §18"
    - "OpenAPI Specification"
```

#### 7.3.13 Health Contract

Governs health check definitions and monitoring.

```yaml
contract_health:
  id: "CTR-HL-001"
  name: "Health Contract"
  version: "1.0.0"
  type: "health"

  schema:
    format: "yaml"
    rules:
      - "Every component MUST implement health checks"
      - "Health checks MUST have defined intervals and timeouts"
      - "Health status MUST be reported to Health Monitor"

  validation_rules:
    - rule: "HL-01"
      description: "All components MUST have readiness probe"
      severity: "error"
      enforcement: "registration"

    - rule: "HL-02"
      description: "All components MUST have liveness probe"
      severity: "error"
      enforcement: "registration"

    - rule: "HL-03"
      description: "Health check interval MUST be < 60s"
      severity: "error"
      enforcement: "registration"

    - rule: "HL-04"
      description: "Health check timeout MUST be < interval"
      severity: "error"
      enforcement: "registration"

    - rule: "HL-05"
      description: "Health failures MUST be logged and alerted"
      severity: "error"
      enforcement: "runtime"

    - rule: "HL-06"
      description: "Circuit breaker MUST be implemented for critical dependencies"
      severity: "warn"
      enforcement: "audit"

  references:
    - "RUNTIME HEALTH §8"
    - "Health Check Definitions §8.2"
    - "Circuit Breaker §8.3"
```

---

### 7.4 Contract Validation Engine

Every contract is validated by the **Contract Validation Engine**.

#### Validation Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                    CONTRACT VALIDATION ENGINE                         │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │ PRE-PUBLISH  │  │  RUNTIME     │  │      PERIODIC            │  │
│  │ Validation    │  │  Validation  │  │      Validation          │  │
│  │              │  │              │  │                          │  │
│  │ • Schema     │  │ • Input      │  │ • Contract compliance    │  │
│  │ • Format     │  │ • Output     │  │ • SLO achievement        │  │
│  │ • Required   │  │ • State      │  │ • Version drift          │  │
│  │ • References │  │ • Events     │  │ • Deprecation tracking   │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
│                                                                     │
│  Output: ValidationReport { contract_id, passed, violations[],      │
│           severity, timestamp }                                     │
└─────────────────────────────────────────────────────────────────────┘
```

#### Validation Levels

| Level | Trigger | Scope | Latency SLA | Action on Failure |
|-------|---------|-------|-------------|-------------------|
| **Pre-publish** | Contract registration/update | Full contract validation | < 1s | Block registration |
| **Runtime** | Every contract interaction | Interaction scope | < 10ms | Reject + log |
| **Periodic** | Every 24h | All active contracts | < 5s | Alert + report |

---

### 7.5 Contract Compatibility & Migration

#### Compatibility Matrix

| Contract Change | Compatible? | Migration Required? | Downtime? |
|----------------|-------------|-------------------|-----------|
| Add optional field | ✅ Backward + Forward | No | No |
| Remove optional field | ⚠️ Forward only | Yes (consumers must update) | No |
| Add required field | ❌ Breaking | Yes (all consumers must update) | Yes |
| Remove required field | ❌ Breaking | Yes (all consumers must update) | Yes |
| Change field type | ❌ Breaking | Yes (all consumers + producers must update) | Yes |
| Add validation rule | ⚠️ Backward only | No (existing data passes) | No |
| Strengthen validation | ⚠️ Depends on data | Maybe | Maybe |

#### Migration Process

```yaml
contract_migration:
  step_1: "Propose contract change (ADR)"
  step_2: "Impact analysis (all consumers identified)"
  step_3: "Version bump (major for breaking changes)"
  step_4: "Dual-run (both versions active simultaneously)"
  step_5: "Consumer migration (one by one, verified)"
  step_6: "Old version deprecation (30-day notice)"
  step_7: "Old version retirement (after sunset date)"

  rollback:
    trigger: "Consumer migration failure"
    action: "Revert to previous contract version"
    duration: "< 1 hour"
```

---

### 7.6 Contract Registry

All contracts are registered in the **Contract Registry**.

```yaml
contract_registry:
  storage:
    primary: "Database (source of truth)"
    cache: "Redis (fast lookup)"

  schema:
    contract_id: "string (unique)"
    name: "string"
    version: "string (SemVer)"
    status: "active | deprecated | retired"
    type: "string (contract type)"
    owner: "string"
    schema: "JSON (full contract definition)"
    validation_rules: "[]"
    created_at: "timestamp"
    updated_at: "timestamp"
    changelog: "[]"

  operations:
    register: "Add new contract"
    update: "Update existing contract (version bump)"
    deprecate: "Mark as deprecated (with sunset date)"
    retire: "Remove from active registry"
    query: "Get contract by id, type, or version"
    validate: "Validate interaction against contract"
```

---

### 7.7 Contract Enforcement & Violations

#### Violation Severity

| Severity | Action | Notification | Escalation |
|----------|--------|-------------|------------|
| **error** | Block operation | Component owner | Chief |
| **warn** | Allow + log | Component owner | None |
| **critical** | Block + escalate | CTO | CEO |

#### Violation Handling Flow

```yaml
violation_handling:
  detect:
    - "Runtime validation failure"
    - "Periodic audit failure"
    - "Schema registry rejection"

  classify:
    - "Severity: error | warn | critical"
    - "Type: input | output | state | event | ..."
    - "Component: publisher | consumer"

  respond:
    error:
      - "Reject operation"
      - "Log violation with full context"
      - "Notify component owner"
    warn:
      - "Allow operation"
      - "Log warning"
      - "Track in contract health metrics"
    critical:
      - "Block operation"
      - "Escalate to CTO immediately"
      - "Create incident"

  report:
    - "Include in Contract Health Dashboard"
    - "Aggregate in weekly contract compliance report"
    - "Flag for Architecture Council review"
```

#### Contract Health Dashboard

```yaml
contract_health_dashboard:
  sections:
    - name: "Contract Compliance"
      metrics:
        - "Active contracts: 13/13"
        - "Deprecated contracts: 0"
        - "Violations (24h): 3"
        - "Compliance score: 98.5%"

    - name: "Violations by Contract"
      chart: "Bar chart of violations per contract type"

    - name: "Violations by Severity"
      chart: "Pie chart: errors vs warnings vs critical"

    - name: "Contract Version Status"
      table: "Contract | Version | Status | Last Updated | Violations"
```

---

### 7.8 Contract Testing

Each contract MUST have associated tests.

```yaml
contract_testing:
  test_types:
    - "Schema validation tests (input matches schema)"
    - "Boundary tests (min/max values, empty, null)"
    - "Compatibility tests (backward + forward)"
    - "Negative tests (invalid inputs, missing fields)"
    - "Performance tests (validation within SLA)"

  test_requirements:
    - "Each contract MUST have ≥ 10 test cases"
    - "Each validation rule MUST have ≥ 1 test"
    - "Compatibility tests MUST cover both directions"
    - "Tests MUST run in CI/CD pipeline"

  test_execution:
    trigger: "On contract registration or update"
    frequency: "Daily (full suite)"
    output: "ContractTestReport { passed, failed, coverage }"
```

---

### 7.9 Contract Templates

Reusable templates for each contract type are maintained at:

```yaml
contract_templates:
  input: "templates/contract/input.yaml"
  output: "templates/contract/output.yaml"
  state: "templates/contract/state.yaml"
  event: "templates/contract/event.yaml"
  capability: "templates/contract/capability.yaml"
  workflow: "templates/contract/workflow.yaml"
  execution: "templates/contract/execution.yaml"
  quality: "templates/contract/quality.yaml"
  memory: "templates/contract/memory.yaml"
  provider: "templates/contract/provider.yaml"
  plugin: "templates/contract/plugin.yaml"
  api: "templates/contract/api.yaml"
  health: "templates/contract/health.yaml"
```

Refer to [RUNTIME_CONTRACT.md](../RUNTIME_CONTRACT.md) for the complete Runtime ↔ Kernel interface contract.

