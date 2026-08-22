# Architecture Overview — Extracted from KERNEL.md §1

> **Source**: KERNEL.md v3.0.1 §1 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 1. ARCHITECTURE OVERVIEW

### 1.1 Two-Layer Architecture

The Kernel separates **Organization** (governance) from **Execution** (runtime) into two distinct architectures.

```
┌──────────────────────────────────────────────────────────────────┐
│                    Cosca KERNEL — TWO-LAYER ARCHITECTURE             │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌─────────────────────────────────────────────────────────┐     │
│  │              ORGANIZATIONAL LAYER                        │     │
│  │              (Governance Only)                           │     │
│  │                                                          │     │
│  │  User → CEO → CTO → Chiefs → Specialists                │     │
│  │                                                          │     │
│  │  Responsibilities:                                       │     │
│  │  • ownership • governance • approvals • quality          │     │
│  │  • escalation • compliance • strategy                    │     │
│  │                                                          │     │
│  │  NEVER executes code. NEVER routes tasks.                │     │
│  │  NEVER implements functionality.                         │     │
│  └───────────────────────┬─────────────────────────────────┘     │
│                          │                                        │
│  ┌───────────────────────▼─────────────────────────────────┐     │
│  │                 RUNTIME LAYER                            │     │
│  │                 (Execution Engine)                       │     │
│  │                                                          │     │
│  │  Request → Bootstrap → Discovery → Context → Memory      │     │
│  │  → Capability Resolution → Workflow Resolution           │     │
│  │  → Planning → DAG → Scheduler → Execution → Review       │     │
│  │  → QA → Documentation → Knowledge Store → Delivery       │     │
│  │                                                          │     │
│  │  Responsibilities:                                       │     │
│  │  • execution • routing • scheduling • state              │     │
│  │  • events • health • metrics • recovery • sync           │     │
│  └─────────────────────────────────────────────────────────┘     │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

---

#### 1.1.1 Organizational Layer — Governance Only

This layer represents **pure governance**. It is responsible for ownership, quality standards, compliance, and strategic decisions. It **never** executes code, routes tasks, or implements features.

```
User Request
  → Cosca Kernel (coordination only)
    → CEO (strategic decisions, resource allocation, roadmap approval)
      → Product Chief (requirements, scope, user stories)
        → CTO (technical planning, technology selection)
          → Department Chiefs (execution domain ownership)
            → Specialists (implementation)
              → Review Chief (code and architecture review)
                → QA Chief (quality validation — Gates 2–3)
                  → Documentation Chief (documentation updates)
                    → Delivery to User
```

| Role | Reports To | Responsibilities | Never |
|------|-----------|-----------------|-------|
| **CEO** | User | Vision, strategy, final decisions, resource allocation, roadmap approval | Implements |
| **CTO** | CEO | Technical strategy, architecture decisions, technology selection, technical debt | Implements |
| **Product Chief** | CEO | Requirements, scope, user stories, backlog, roadmap | Implements |
| **Architecture Chief** | CTO | System design, patterns, ADRs, modular boundaries, technical standards | Implements directly |
| **Department Chiefs** | CTO, Architecture | Domain execution, specialist delegation, quality within domain | Bypass chain of command |
| **Specialists** | Department Chiefs | Implementation, testing, documentation | Communicate with user |
| **Review Chief** | CTO | Code review, architecture review, security review | Implements |
| **QA Chief** | CTO | Quality gates, test strategy, quality metrics | Implements |
| **Documentation Chief** | CTO | README, ADR, API docs, changelog, release notes | Skips review |

Refer to [company/ORGCHART.md](company/ORGCHART.md) for complete organizational structure.

---

#### 1.1.2 Runtime Layer — Execution Specification

This is the **official Runtime execution architecture**. Every implementation MUST follow this pipeline exactly.

```
REQUEST
  │
  ▼
┌─────────────┐
│  BOOTSTRAP  │  → Initialize Runtime, load config, resolve paths
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  DISCOVERY  │  → Scan workspace: framework, language, DB, deps, build system, CI/CD
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   CONTEXT   │  → Load session context, project context, environment context
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   MEMORY    │  → Load all memory types per MEMORY_MODEL.md
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ CAPABILITY  │  → Resolve required capabilities from request (never departments)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  WORKFLOW   │  → Map capabilities → workflows → chiefs → specialists
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  PLANNING   │  → Generate execution plan with DAG, success criteria, risk assessment
└──────┬──────┘
       │
       ▼
┌─────────────┐
│     DAG     │  → Generate directed acyclic graph of execution steps
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  SCHEDULER  │  → Queue, prioritize, dispatch (immediate, priority, dependency, retry, delayed, cron, background)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  EXECUTION  │  → Execute steps through workers (parallel where possible)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   REVIEW    │  → Code review, architecture review, security review
└──────┬──────┘
       │
       ▼
┌─────────────┐
│     QA      │  → Quality gate enforcement (Gates 2–3)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ DOCUMENT    │  → Update README, ADR, API docs, changelog, release notes
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  KNOWLEDGE  │  → Store decisions, patterns, learnings in knowledge stores
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  DELIVERY   │  → Return result to user, persist session, publish completion event
└─────────────┘
```

---


### 1.1.3 Layer Interface Contract

The Organizational Layer and Runtime Layer communicate through a **formal interface contract**. This contract defines how governance influences execution and how execution informs governance — without the Organizational Layer ever touching code or the Runtime Layer ever making governance decisions.

### Layer Communication Model

```
┌─────────────────────────────────────────────────────────────────────┐
│                    LAYER INTERFACE CONTRACT                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌────────────────────────────┐      ┌────────────────────────────┐│
│  │    ORGANIZATIONAL LAYER    │      │       RUNTIME LAYER        ││
│  │    (Governance Only)       │      │    (Execution Engine)      ││
│  │                            │      │                            ││
│  │  ┌──────────────────────┐  │      │  ┌──────────────────────┐  ││
│  │  │ Governance Policies  │──┼──────┼─▶│  Bootstrap & Execute  │  ││
│  │  │ • Strategy           │  │      │  │  • Follow policies    │  ││
│  │  │ • Approvals          │  │      │  │  • Execute workflows  │  ││
│  │  │ • Compliance Rules   │  │      │  │  • Generate events    │  ││
│  │  │ • Quality Standards  │  │      │  │  • Report status      │  ││
│  │  │ • Escalation Matrix  │  │      │  │  • Request decisions  │  ││
│  │  └──────────────────────┘  │      │  └──────────────────────┘  ││
│  │           │                 │      │            │               ││
│  │           │  DECISIONS ▲   │      │  ▼ EVENTS  │               ││
│  │           │  APPROVALS │   │      │  │ STATUS  │               ││
│  │           │  POLICIES  │   │      │  │ METRICS │               ││
│  │           ▼  STRATEGY  │   │      │  │ AUDIT   │               ││
│  │  ┌──────────────────────┐  │      │  └──────────────────────┘  ││
│  │  │ Decision Records     │◀─┼──────┼──│  Event Bus & Metrics   ││
│  │  │ • ADRs               │  │      │  └──────────────────────┘  ││
│  │  │ • Approval History   │  │      │                            ││
│  │  │ • Escalation Log     │  │      │                            ││
│  │  └──────────────────────┘  │      │                            ││
│  │                            │      │                            ││
│  └────────────────────────────┘      └────────────────────────────┘│
│                                                                     │
│  CHANNELS:                                                          │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │ • Policies (Org → Runtime): Governance rules, quality gates,   │ │
│  │   compliance requirements, approval decisions                   │ │
│  │ • Events (Runtime → Org): Status changes, completion reports,  │ │
│  │   quality scores, escalation triggers, metrics                 │ │
│  │ • Decisions (Org ↔ Runtime): Bidirectional ADR flow via        │ │
│  │   Decision Records stored in memory/architecture               │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Layer Communication Channels

| Channel | Direction | Protocol | Frequency | Payload Examples |
|---------|-----------|----------|-----------|------------------|
| **Policy Injection** | Org → Runtime | QUALITY_GATES.md, GOVERNANCE.md, ORGCHART.md | On session start, on policy change | Gate thresholds, approval rules, escalation paths |
| **Decision Request** | Runtime → Org | ADR format via memory/architecture | As needed | Architecture decisions, scope changes, risk acceptance |
| **Decision Response** | Org → Runtime | ADR with approval/rejection status | Synchronous (blocking) | Approved, Rejected with reason, Escalated |
| **Status Report** | Runtime → Org | Event Bus → Session events | Continuous | State, health, progress, metrics |
| **Escalation Trigger** | Runtime → Org | Event → Error Handling table | On failure | Error type, attempts exhausted, recommendation |
| **Quality Report** | Runtime → Org | Quality Gate results | Per gate pass/fail | Score, issues, pass/fail decisions |
| **Strategy Injection** | Org → Runtime | WORKFLOW_ROUTING, product decisions | Per session | Priority, scope, resource allocation |
| **Compliance Binding** | Org → Runtime | Policy Engine | On session start | Must-comply rules, prohibited actions |

### Layer Isolation Rules

These rules are **immutable**. Violations are automatically detected and reported.

| Rule | Domain | Description | Violation Consequence |
|------|--------|-------------|----------------------|
| **L-001** | Organizational | Must NEVER execute code, commands, or scripts | Automatic escalation to CEO |
| **L-002** | Organizational | Must NEVER route tasks directly to specialists | Automatic escalation to CTO |
| **L-003** | Organizational | Must NEVER implement features or fix bugs | Automatic escalation to CEO |
| **L-004** | Organizational | Must NEVER read/write files outside governance docs | Audit log warning |
| **L-005** | Organizational | Must NEVER bypass Capability Resolution | Architecture review flag |
| **L-006** | Runtime | Must NEVER override governance decisions | Automatic rollback |
| **L-007** | Runtime | Must NEVER skip quality gates | Blocked delivery, automatic CTO notification |
| **L-008** | Runtime | Must NEVER modify ORGCHART.md, QUALITY_GATES.md, or GOVERNANCE.md | Read-only enforced |
| **L-009** | Runtime | Must NEVER make strategic or scope decisions | Escalation to Product Chief |
| **L-010** | Both | All cross-layer communication MUST be logged to Audit Trail | Audit failure, compliance violation |

---

### 1.1.4 Organizational Layer — Deep Governance Model

This section defines the **complete governance machinery** of the Organizational Layer. Every governance responsibility listed in the PURPOSE is expanded with its formal mechanism.

#### Governance Domain Catalog

| Domain | Owner | Mechanism | Artifacts | Enforcement |
|--------|-------|-----------|-----------|-------------|
| **Ownership** | CEO + CTO | ORGCHART.md, Department SKILL.md | Responsibility matrix | Auto-validated by Skills Engine |
| **Governance** | CEO | GOVERNANCE.md, Councils | Policy decisions, CDRs | Policy Engine audit |
| **Approvals** | CEO, CTO, Product Chief | Approval matrix, decision authority | Signed ADRs | Gate 1 enforcement |
| **Quality** | QA Chief, CTO | QUALITY_GATES.md | Gate scores, quality reports | Gates 0–4 enforcement |
| **Escalation** | CEO, CTO | Error Handling table, Escalation ladder | Escalation records | Auto-routing by Runtime |
| **Compliance** | Security Chief, CTO | Policy Engine, Compliance Engine | Compliance reports, audit trail | Compliance Engine audit |
| **Strategy** | CEO, Product Chief | Roadmap, backlog, product decisions | Strategic ADRs | Product Chief validation |

#### Decision Authority Matrix (Expanded)

The complete Decision Authority Matrix defines exactly who decides what:

| Decision | Authority | Requires | Can Be Overridden By | Escalation |
|----------|-----------|---------|---------------------|------------|
| Feature scope | Product Chief | CEO alignment | CEO | CEO |
| Release scope | Product Chief + CTO | QA sign-off | CEO | CEO |
| Architecture decision | Architecture Chief | CTO review | CTO (with ADR) | CTO |
| Technology selection | CTO | Architecture Chief input | CEO (strategic) | CEO |
| Framework choice | Backend/Frontend Chief | Architecture Chief approval | CTO | CTO |
| Database schema | Database Chief | Architecture Chief review | CTO | CTO |
| API design | Backend Chief | Architecture Chief review | Architecture Chief | CTO |
| Test strategy | QA Chief | CTO alignment | CTO | CTO |
| Security standard | Security Chief | CTO alignment | CEO (risk acceptance) | CEO |
| Documentation format | Documentation Chief | — | CTO | CTO |
| CI/CD pipeline | DevOps Chief | Security Chief review | CTO | CTO |
| Provider selection | AI Chief | CTO + Security alignment | CEO (cost impact) | CEO |
| Budget allocation | CEO | — | User | User |
| Quality gate threshold | QA Chief | CTO alignment | CEO | CEO |
| Escalation path | CEO | CTO input | CEO | — |
| Compliance requirement | Security Chief | CTO + Legal (future) | CEO (risk acceptance) | CEO |
| Release go/no-go | Release Chief | All Gate 3 passed | CEO | CEO |
| Incident response | Monitoring Chief | Security Chief (if security) | CTO | CTO |

#### Approval Workflow Patterns

All approvals in the Organizational Layer follow one of these three patterns:

```
PATTERN 1 — Sequential (default)
  Requester → Direct Manager → Higher Authority
  Example: Specialist → Chief → CTO

PATTERN 2 — Consensus (cross-department)
  Requester → All Affected Chiefs → CTO
  Example: API change → Backend Chief + Frontend Chief + Database Chief → CTO

PATTERN 3 — Emergency (fast-track)
  Requester → CTO (immediate) → CEO (notification within 1h)
  Example: Security hotfix, production outage
```

| Pattern | Max Time | Quorum | Documentation |
|---------|----------|--------|---------------|
| Sequential | 1 working session | Single-threaded | ADR with approval chain |
| Consensus | 2 working sessions | All affected Chiefs | ADR with sign-off list |
| Emergency | 30 minutes | CTO only | ADR + post-fact review |

#### Escalation Ladder

The Escalation Ladder defines formal trigger conditions and routing:

```
Level 1 — Chief Escalation
  Trigger: Agent timeout > 3 retries OR quality score < 5.0
  Action: Secondary agent activated, Chief notified
  Response Time: < 5 minutes

Level 2 — CTO Escalation
  Trigger: Chief unavailable OR cross-department conflict OR resource shortage
  Action: CTO reviews, reassigns, or overrides
  Response Time: < 15 minutes

Level 3 — CEO Escalation
  Trigger: CTO unavailable OR strategic conflict OR budget decision
  Action: CEO decides, may involve Council
  Response Time: < 1 hour

Level 4 — Council Escalation
  Trigger: Irreconcilable cross-department dispute OR enterprise-wide impact
  Action: Relevant Council convenes, binding decision
  Response Time: < 1 working session

Level 5 — User Escalation
  Trigger: All paths exhausted OR system-wide failure
  Action: Kernel presents full diagnosis, CEO recommends, User decides
  Response Time: Immediate
```

#### Compliance Binding

Every governance decision produces a **binding compliance artifact**:

```
Governance Decision
  │
  ▼
┌──────────────────────┐
│  Policy Declaration  │  → Written to Policy Engine
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  Compliance Rule     │  → Machine-readable rule
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  Runtime Binding     │  → Loaded by Runtime at session start
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  Enforcement         │  → Automatic gate check
└──────────────────────┘
```

All compliance bindings are versioned, dated, and attributed to the decision authority.

#### Strategy Propagation

Strategy flows from the Organizational Layer to the Runtime Layer through a formal propagation chain:

```
CEO Vision (quarterly)
  │  → Strategic ADR
  ▼
Product Roadmap (monthly)
  │  → Product backlog, priority definitions
  ▼
CTO Technical Strategy (monthly)
  │  → Technology decisions, architecture direction
  ▼
Chief Department Plans (sprint)
  │  → Capability assignments, resource allocation
  ▼
Runtime Execution (daily)
  │  → Workflow selection, DAG generation, scheduling
  ▼
Delivery & Feedback (continuous)
  │  → Metrics, quality scores, learning data
  ▼
Governance Review (next cycle)
```

---

### 1.1.5 Runtime Layer — Stage Contracts

Every stage in the Runtime execution pipeline has a **formal Stage Contract**. These contracts define inputs, outputs, error domains, timeout policies, SLAs, and emitted metrics.

#### Stage Contract Template

```yaml
stage:
  name: "Stage Name"
  position: 1-N
  description: "What this stage does"
  
  input_contract:
    required_fields: []
    optional_fields: []
    validation_rules: []
    
  output_contract:
    success_output: {}
    failure_output: {}
    
  error_domains:
    - name: "Error Type"
      severity: "fatal | transient | warning"
      recovery: "retry | skip | abort"
      
  timeout_policy:
    hard_limit_ms: 300000
    warning_at_ms: 240000
    
  sla:
    p50_ms: 5000
    p95_ms: 30000
    p99_ms: 60000
    
  emitted_metrics:
    - metric: "metric_name"
      type: "histogram | gauge | counter"
      unit: "ms | count | bytes"
      
  state_transition:
    entry: "StateName"
    success: "NextState"
    failure: "FailedState"
    
  published_events:
    - event: "EventName"
      trigger: "on_entry | on_success | on_failure"
```

#### Complete Stage Contract Catalog

**Stage 1 — BOOTSTRAP**

```yaml
stage:
  name: "Bootstrap"
  position: 1
  
  input_contract:
    required: [runtime_type, workspace_path, session_id]
    optional: [config_overrides, environment_variables]
    validation: [runtime_type in [opencode, claude-code, codex, cosca-runtime, gemini-cli, cursor, continue, adk-go, sdk]]
    
  output_contract:
    success:
      config: resolved_config
      paths: resolved_virtual_paths
      providers: initialized_providers
    failure:
      error: bootstrap_error
      diagnostics: []
      
  error_domains:
    - config_not_found: [fatal, abort]
    - path_resolution_failed: [fatal, abort]
    - provider_init_failed: [transient, retry(max=3)]
    
  timeout: 30000ms
  sla: { p50: 2000, p95: 10000, p99: 20000 }
  
  metrics: [bootstrap_duration_ms, config_size_bytes, provider_count]
  
  state: BOOTSTRAPPING → DISCOVERING
  events: [BootstrapStarted → BootstrapCompleted | BootstrapFailed]
```

**Stage 2 — DISCOVERY**

```yaml
stage:
  name: "Discovery"
  position: 2
  
  input_contract:
    required: [workspace_path, session_id]
    optional: [framework_hints, language_hints]
    
  output_contract:
    success:
      framework: string
      language: string
      database: string | null
      dependencies: []
      build_system: string | null
      test_framework: string | null
      ci_cd: string | null
      architecture_pattern: string | null
      module_boundaries: []
    failure:
      error: discovery_error
      partial_results: {}
      
  error_domains:
    - workspace_not_found: [fatal, abort]
    - scan_timeout: [transient, retry(max=2)]
    - partial_discovery: [warning, continue_with_partial]
    
  timeout: 60000ms
  sla: { p50: 5000, p95: 30000, p99: 50000 }
  
  metrics: [discovery_duration_ms, files_scanned, deps_found]
  
  state: DISCOVERING → LOADING_CONTEXT
  events: [DiscoveryStarted → DiscoveryCompleted]
```

**Stage 3 — CONTEXT LOADING**

```yaml
stage:
  name: "Context Loading"
  position: 3
  
  input_contract:
    required: [session_id, discovery_results]
    optional: [session_overrides]
    
  output_contract:
    success:
      session_context: {}
      project_context: {}
      environment_context: {}
      context_size_bytes: 0
    failure:
      error: context_error
      
  error_domains:
    - context_corrupted: [fatal, abort]
    - context_too_large: [warning, truncate_and_continue]
    
  timeout: 30000ms
  sla: { p50: 3000, p95: 15000, p99: 25000 }
  
  metrics: [context_load_time_ms, context_size_bytes, context_entries]
  
  state: LOADING_CONTEXT → LOADING_MEMORY
  events: [ContextLoaded]
```

**Stage 4 — MEMORY LOADING**

```yaml
stage:
  name: "Memory Loading"
  position: 4
  
  input_contract:
    required: [session_id, memory_types[]]
    optional: [filter_tags, time_range]
    
  output_contract:
    success:
      stores_loaded: []
      total_entries: 0
      memory_by_type: {}
    failure:
      error: memory_error
      partial_stores: {}
      
  error_domains:
    - store_not_found: [warning, skip_store]
    - store_corrupted: [transient, retry(max=2), fallback_to_backup]
    - global_store_unreachable: [warning, skip_global, continue_local]
    
  timeout: 60000ms
  sla: { p50: 5000, p95: 25000, p99: 50000 }
  
  metrics: [memory_load_time_ms, memory_entries, memory_size_bytes]
  
  state: LOADING_MEMORY → VALIDATING
  events: [MemoryLoaded]
```

**Stage 5 — VALIDATION (Gate 0)**

```yaml
stage:
  name: "Validation"
  position: 5
  
  input_contract:
    required: [request_type, request_payload, session_id]
    validation:
      - request_type in [feature, bug, refactor, architecture, docs, deploy, research, review]
      - scope defined (>= 1 sentence)
      - capabilities resolvable from request
      - no conflicting active workflows
    
  output_contract:
    success:
      validated: true
      capabilities: []
      priority: [critical, high, medium, low]
      complexity: [trivial, simple, medium, complex, epic]
    failure:
      validated: false
      errors: []
      warnings: []
      
  error_domains:
    - invalid_request_type: [fatal, abort]
    - missing_scope: [fatal, abort]
    - unresolvable_capability: [fatal, abort]
    - conflicting_workflow: [warning, queue_or_escalate]
    
  timeout: 15000ms
  sla: { p50: 1000, p95: 5000, p99: 10000 }
  
  metrics: [validation_duration_ms, validation_checks, capability_count]
  
  state: VALIDATING → PLANNING | FAILED
  events: [ValidationCompleted | ValidationFailed]
```

**Stage 6 — PLANNING**

```yaml
stage:
  name: "Planning"
  position: 6
  
  input_contract:
    required: [capabilities[], priority, complexity, session_id]
    optional: [constraints, preferences]
    
  output_contract:
    success:
      plan_id: uuid
      workflow_id: string
      dag_nodes: []
      dag_edges: []
      success_criteria: []
      risk_assessment: {}
      estimated_effort: [XS, S, M, L, XL]
    failure:
      error: planning_error
      
  error_domains:
    - no_workflow_for_capability: [fatal, abort]
    - circular_dependency: [fatal, abort]
    - resource_unavailable: [warning, escalate_to_cto]
    
  timeout: 60000ms
  sla: { p50: 10000, p95: 45000, p99: 55000 }
  
  metrics: [planning_time_ms, dag_nodes, dag_edges, plan_complexity]
  
  state: PLANNING → EXECUTING
  events: [PlanCreated]
```

**Stage 7 — EXECUTION**

```yaml
stage:
  name: "Execution"
  position: 7
  
  input_contract:
    required: [plan_id, dag, session_id]
    
  output_contract:
    success:
      results: {}
      duration_ms: 0
      step_count: 0
      parallel_count: 0
    failure:
      error: execution_error
      partial_results: {}
      failed_steps: []
      
  error_domains:
    - step_timeout: [transient, retry(max=3, exponential)]
    - step_failed_permanent: [fatal, abort_dependent_steps]
    - dependency_failure: [transient, retry(max=3)]
    - resource_exhausted: [fatal, escalate_to_cto]
    
  timeout: 300000ms (configurable per step)
  sla: { p50: 60000, p95: 240000, p99: 290000 }
  
  metrics: [execution_time_ms, step_count, parallel_count, retry_count, worker_count]
  
  state: EXECUTING → REVIEWING | FAILED
  events: [ExecutionStarted → ExecutionCompleted | ExecutionFailed]
```

**Stage 8 — REVIEW**

```yaml
stage:
  name: "Review"
  position: 8
  
  input_contract:
    required: [artifacts[], review_types[], session_id]
    review_types: [architecture, code, security, performance]
    
  output_contract:
    success:
      score: 0.0-10.0
      issues: []
      passed: true
    failure:
      score: 0.0-10.0
      issues: []
      passed: false
      
  error_domains:
    - review_timeout: [transient, retry(max=2)]
    - review_inconclusive: [warning, escalate_to_review_chief]
    
  timeout: 240000ms
  sla: { p50: 30000, p95: 90000, p99: 110000 }
  
  metrics: [review_time_ms, issues_found, review_score]
  
  state: REVIEWING → DOCUMENTING | FAILED
  events: [ReviewStarted → ReviewCompleted]
```

**Stage 9 — QUALITY ASSURANCE**

```yaml
stage:
  name: "Quality Assurance"
  position: 9
  
  input_contract:
    required: [review_results, session_id]
    gates: [Gate 2, Gate 3]
    
  output_contract:
    success:
      gate_results: {}
      overall_score: 0.0-10.0
      passed: true
    failure:
      gate_results: {}
      overall_score: 0.0-10.0
      failed_gates: []
      
  error_domains:
    - quality_timeout: [transient, retry(max=2)]
    - quality_inconclusive: [warning, escalate_to_qa_chief]
    
  timeout: 240000ms
  sla: { p50: 30000, p95: 90000, p99: 110000 }
  
  metrics: [quality_score, gate_pass_rate, test_coverage]
  
  state: REVIEWING → DOCUMENTING | FAILED
  events: [QualityPassed | QualityFailed]
```

**Stage 10 — DOCUMENTATION**

```yaml
stage:
  name: "Documentation"
  position: 10
  
  input_contract:
    required: [changes[], session_id]
    doc_types: [readme, adr, api_docs, db_docs, changelog, release_notes]
    
  output_contract:
    success:
      docs_updated: []
    failure:
      error: documentation_error
      
  error_domains:
    - doc_generation_failed: [transient, retry(max=2)]
    - doc_conflict: [warning, mark_for_review]
    
  timeout: 60000ms
  sla: { p50: 15000, p95: 45000, p99: 55000 }
  
  metrics: [doc_duration_ms, docs_updated_count]
  
  state: DOCUMENTING → LEARNING
  events: [DocumentationUpdated]
```

**Stage 11 — KNOWLEDGE STORE**

```yaml
stage:
  name: "Knowledge Store"
  position: 11
  
  input_contract:
    required: [decisions[], patterns[], learnings[], session_id]
    
  output_contract:
    success:
      stores_updated: []
      entry_count: 0
    failure:
      error: knowledge_error
      
  error_domains:
    - store_write_failed: [transient, retry(max=3)]
    - embedding_failed: [warning, store_without_embeddings]
    
  timeout: 60000ms
  sla: { p50: 10000, p95: 40000, p99: 55000 }
  
  metrics: [knowledge_sync_time_ms, entries_stored]
  
  state: LEARNING → SYNCING
  events: [KnowledgeStored]
```

**Stage 12 — DELIVERY**

```yaml
stage:
  name: "Delivery"
  position: 12
  
  input_contract:
    required: [results, session_id, quality_score]
    
  output_contract:
    success:
      delivery_status: "completed"
      summary: {}
      duration_ms: 0
    failure:
      delivery_status: "failed"
      error: delivery_error
      
  error_domains:
    - delivery_timeout: [transient, retry(max=2)]
    
  timeout: 30000ms
  sla: { p50: 5000, p95: 20000, p99: 25000 }
  
  metrics: [session_duration_ms, delivery_size_bytes]
  
  state: SYNCING → FINISHED | FAILED
  events: [SessionFinished]
```

---

### 1.1.6 Cross-Layer Feedback Loops

The Organizational and Runtime layers interact through six formal feedback loops.

#### Loop 1: Governance → Execution (Policy Injection)

```
Organizational Layer                    Runtime Layer
┌─────────────────┐                   ┌──────────────────┐
│ Policy Created   │                   │                  │
│ (quality gate,   │── Policy Document──▶ Bootstrap reads  │
│  compliance rule,│                   │ policy            │
│  approval dec)   │                   │                  │
└─────────────────┘                   │ Enforce during   │
        ▲                             │ execution         │
        │                             └────────┬─────────┘
        │                                      │
        │         Compliance Report             │
        └───────────────────────────────────────┘
```

#### Loop 2: Execution → Governance (Metrics & Audit)

```
Runtime Layer                          Organizational Layer
┌──────────────────┐                   ┌──────────────────────┐
│ Session completes │── Metric Report──▶│ Quality score review │
│ Events published  │                   │ Decision validation  │
│ Metrics collected │── Audit Log──────▶│ Compliance check     │
└──────────────────┘                   │ Governance review    │
                                        └──────────────────────┘
```

#### Loop 3: Escalation (Runtime → Org)

```
Runtime Layer detects unrecoverable failure
  │
  ▼
Escalation Trigger (per Escalation Ladder)
  │
  ├── Level 1: Chief notified
  ├── Level 2: CTO notified
  ├── Level 3: CEO notified
  ├── Level 4: Council convened
  └── Level 5: User notified
  │
  ▼
Organizational Decision → Runtime resumes or aborts
```

#### Loop 4: Quality Feedback (Runtime → Org → Runtime)

```
Runtime completes Gate 2 (post-implementation)
  │
  ▼
Score < Threshold → Issue Report to Review Chief
  │
  ▼
Review Chief decides: Pass | Fix | Reject
  │
  ├── Pass → Continue to Gate 3
  ├── Fix → Return to Runtime with fix instructions
  └── Reject → Abort, notify CEO
```

#### Loop 5: Strategy Adjustment (Org → Runtime)

```
CEO reviews quarterly metrics
  │
  ▼
Strategic decision (priority shift, scope change)
  │
  ▼
ADR created → Policy Engine updated
  │
  ▼
Runtime picks up new policies on next session start
```

#### Loop 6: Learning & Evolution (Runtime → Org → Runtime)

```
Runtime completes session
  │
  ▼
Learning Engine extracts patterns
  │
  ▼
Patterns stored in knowledge
  │
  ▼
Evolution Engine analyzes across sessions
  │
  ▼
Recommendations → Organizational review → Policy update
  │
  ▼
Runtime adopts improved policies
```

### 1.1.7 Layer Isolation Verification

Every session MUST verify layer isolation. The verification is automatic and produces a report:

```yaml
layer_isolation_report:
  session_id: "uuid"
  timestamp: "ISO8601"
  
  organizational_layer:
    executed_code: false          # L-001
    routed_tasks_directly: false  # L-002
    implemented_features: false   # L-003
    modified_runtime_files: false # L-004
    bypassed_capabilities: false  # L-005
    
  runtime_layer:
    overrode_governance: false    # L-006
    skipped_quality_gates: false  # L-007
    modified_governance_docs: false # L-008
    made_strategic_decisions: false # L-009
    
  cross_layer:
    all_communication_logged: true  # L-010
    decision_records_synced: true
    
  compliance: "PASS | FAIL"
  
  violations: []  # empty = clean
```

This report is stored in `.cosca/memory/architecture/layer-isolation/` after every session.


