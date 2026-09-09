# Capability First Architecture — Extracted from KERNEL.md §2

> **Source**: KERNEL.md v3.0.1 §2 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 2. CAPABILITY FIRST ARCHITECTURE

The Kernel **never** knows departments. The Kernel knows only **Capabilities**.

A Capability is a **formal, contract-bound unit of functionality** with defined inputs, outputs, quality criteria, dependencies, and a designated provider. Every request is resolved to capabilities first — departments, chiefs, and specialists are resolved only after capability selection.

```
Request
  │
  ▼
┌──────────────────────────────────────────────────────────────────┐
│                      CAPABILITY ENGINE                           │
│                                                                  │
│  Request → [Classify] → [Match] → [Resolve Dependencies]         │
│         → [Compose] → [Rank] → [Select] → [Bind Providers]      │
│                                                                  │
│  Output: Ordered list of Capabilities with bound providers       │
└──────────────────────────────────┬───────────────────────────────┘
                                   │
                                   ▼
┌──────────────────────────────────────────────────────────────────┐
│                      WORKFLOW ENGINE                             │
│                                                                  │
│  Capabilities → Workflow → Chiefs → Specialists                  │
└──────────────────────────────────────────────────────────────────┘
```

---

### 2.1 Capability Contract Schema

Every capability in the Cosca ecosystem MUST conform to this contract schema:

```yaml
capability:
  id: "CAP-CAT-NNN"
  name: "Capability Name"

  metadata:
    version: "1.0.0"
    status: "active | deprecated | retired"
    category: "architecture | engineering | quality | security | infrastructure | ai | data | platform | governance | product | operations | integration"
    owner: "Provider Name"
    since: "v1.0"

  description:
    purpose: "What this capability does"
    scope: "Boundaries of what this capability covers"
    limitations: "What this capability does NOT cover"

  input_contract:
    required: []
    optional: []
    validation_rules: []

  output_contract:
    success: {}
    failure: {}
    artifacts: []

  quality_criteria:
    min_score: 0.0         # Minimum quality score (0-10)
    critical_checks: []     # Must-pass checks
    automated_checks: []    # Auto-verifiable checks

  dependencies:
    requires: []            # Capability IDs this depends on
    optional_with: []       # Capabilities that can substitute
    conflicts_with: []      # Capabilities that conflict

  constraints:
    timeout_ms: 300000
    max_retries: 3
    resource_profile: "light | medium | heavy"
    concurrency_limit: 5

  slo:
    p50_duration_ms: 30000
    p95_duration_ms: 240000
    p99_duration_ms: 240000
    availability: 0.995     # 99.5% uptime
    max_failure_rate: 0.01  # 1% max failures

  provider:
    primary: "Chief | Engine Name"
    secondary: "Fallback Provider"
    verification: "Review Engine | QA Chief"
```

#### Contract Fields — Detailed Definitions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | String | Yes | Canonical capability ID (e.g., CAP-ENG-001) |
| `name` | String | Yes | Human-readable capability name |
| `version` | SemVer | Yes | Capability contract version |
| `status` | Enum | Yes | active, deprecated, retired |
| `category` | Enum | Yes | One of 12 canonical categories |
| `owner` | String | Yes | Responsible chief or engine |
| `purpose` | String | Yes | One-sentence capability definition |
| `scope` | String | Yes | What this capability covers |
| `limitations` | String | No | Explicit boundaries |
| `input_contract` | Object | Yes | Required/optional inputs and validation |
| `output_contract` | Object | Yes | Success and failure outputs |
| `quality_criteria` | Object | Yes | Quality score thresholds and checks |
| `dependencies` | Object | No | Required, optional, and conflicting capabilities |
| `constraints` | Object | Yes | Timeout, retry, resource, concurrency limits |
| `slo` | Object | Yes | Duration percentiles, availability, failure rate |
| `provider` | Object | Yes | Primary, secondary, and verification providers |

Refer to [capabilities/CAPABILITY_TEMPLATE.md](../capabilities/CAPABILITY_TEMPLATE.md) for the complete capability contract template.

---

### 2.2 Capability Lifecycle

Every capability follows a formal lifecycle with defined states, transitions, and governance.

#### Lifecycle Diagram

```
┌────────────┐
│  PROPOSED  │  → Proposal submitted to Capability Engine
└──────┬─────┘
       │ Approved by CTO
       ▼
┌────────────┐
│ REGISTERED │  → Registered in Capability Catalog, not yet active
└──────┬─────┘
       │ Provider ready
       ▼
┌────────────┐
│   ACTIVE   │  → Available for resolution, fully operational
└──────┬─────┘
       │
       ├──→ Superseded by new version
       │
       ▼
┌────────────┐
│ DEPRECATED │  → Still available, migration notice, sunset date set
└──────┬─────┘
       │ After sunset date
       ▼
┌────────────┐
│  RETIRED   │  → Removed from active registry, archived
└────────────┘
```

#### Lifecycle State Definitions

| State | Meaning | Resolution Behavior | Provider Responsibility |
|-------|---------|-------------------|------------------------|
| **PROPOSED** | Under review by CTO | Not resolved | Complete contract template |
| **REGISTERED** | Approved, provider not ready | Not resolved | Implement provider |
| **ACTIVE** | Fully operational | Normal resolution | Maintain quality & SLOs |
| **DEPRECATED** | Replacement available | Resolved with warning | Support migration, minimum 30 days notice |
| **RETIRED** | Archived | Never resolved | Archive capability record |

#### Lifecycle Transition Rules

| Transition | Trigger | Required Approvals | Artifact |
|-----------|---------|-------------------|----------|
| PROPOSED → REGISTERED | CTO approval | CTO | Approved proposal ADR |
| REGISTERED → ACTIVE | Provider ready + Gate 1 | Architecture Chief | Provider verification report |
| ACTIVE → DEPRECATED | New capability supersedes | CTO + Provider Chief | Deprecation notice + Migration path |
| DEPRECATED → RETIRED | Sunset date passed (min 30 days) | CTO | Retirement record in CHANGELOG |
| ACTIVE → RETIRED | Emergency (security/breaking) | CEO + CTO | Emergency ADR |

#### Versioning Policy

Capabilities follow SemVer independently of the Cosca platform version:

```
MAJOR: Breaking change to input/output contract
MINOR: New capability features, backward compatible
PATCH: Quality improvements, bug fixes, SLO adjustments
```

| Change Type | Version Bump | Approval | Affected Consumers |
|-------------|-------------|----------|-------------------|
| Input contract change | MAJOR | CTO + affected Chiefs | All dependents |
| Output contract change | MAJOR | CTO + affected Chiefs | All dependents |
| New optional input | MINOR | Provider Chief | None (backward compatible) |
| Quality criteria change | MINOR | QA Chief | Quality Engine |
| SLO change | MINOR | CTO | Monitoring Chief |
| Provider change | MAJOR | CTO + Provider Chief | All dependents |
| Bug fix | PATCH | Provider Chief | None |
| Documentation update | PATCH | Provider Chief | None |

---

### 2.3 Capability Resolution Algorithm

The Capability Engine resolves every request to a set of required capabilities using a formal 6-step algorithm.

#### Algorithm Specification

```
Algorithm: RESOLVE_CAPABILITIES
Input:  request { type, payload, metadata }
        context { session, project, environment }
        registry [Capability Catalog]
Output: resolved [Ordered List of Capability IDs]
        plan    [Execution Plan with dependencies]

Step 1 — CLASSIFY
  Determine request taxonomy:
  ┌──────────────────────────────────────────────────────┐
  │ type = classify(request.payload)                     │
  │   → type: feature | bug | refactor | architecture    │
  │          | documentation | deployment | research      │
  │          | review | maintenance | incident            │
  │   → subtype: (type-specific classification)          │
  │   → priority: critical | high | medium | low         │
  │   → complexity: trivial | simple | medium | complex   │
  └──────────────────────────────────────────────────────┘

Step 2 — MATCH
  Match request type to capability candidates:
  ┌──────────────────────────────────────────────────────┐
  │ candidates = capability_registry.match(              │
  │   type = request.type,                               │
  │   subtype = request.subtype,                         │
  │   context = context                                  │
  │ )                                                    │
  │   → Returns list of matching capability IDs          │
  │   → Each match has confidence score (0.0-1.0)        │
  └──────────────────────────────────────────────────────┘

Step 3 — RESOLVE DEPENDENCIES
  Resolve transitive capability dependencies:
  ┌──────────────────────────────────────────────────────┐
  │ resolved = {}                                        │
  │ queue = candidates                                   │
  │ while queue is not empty:                            │
  │   cap = queue.pop()                                  │
  │   if cap in resolved: continue                       │
  │   resolved.add(cap)                                  │
  │   for dep in cap.dependencies.requires:              │
  │     if dep not in resolved:                          │
  │       queue.add(dep)                                 │
  │   # Detect circular dependencies                     │
  │   if has_cycle(resolved):                            │
  │     abort with CIRCULAR_DEPENDENCY_ERROR             │
  └──────────────────────────────────────────────────────┘

Step 4 — COMPOSE
  Determine composition pattern for resolved capabilities:
  ┌──────────────────────────────────────────────────────┐
  │ composition = analyze_composition(resolved)          │
  │   → AND: all required capabilities                   │
  │   → OR: choose best among alternatives               │
  │   → SEQUENCE: ordered execution with handoffs        │
  │   → PARALLEL: independent concurrent execution       │
  │   → MIXED: hybrid composition pattern                │
  └──────────────────────────────────────────────────────┘

Step 5 — RANK
  Rank capabilities by priority, quality, and availability:
  ┌──────────────────────────────────────────────────────┐
  │ ranked = sort(resolved, key=lambda cap: (            │
  │   cap.priority * 0.4 +                               │
  │   cap.quality_score * 0.3 +                          │
  │   cap.provider.availability * 0.2 +                  │
  │   cap.contextual_relevance * 0.1                     │
  │ ), reverse=True)                                     │
  └──────────────────────────────────────────────────────┘

Step 6 — SELECT & BIND
  Select final capabilities and bind to providers:
  ┌──────────────────────────────────────────────────────┐
  │ selected = []                                        │
  │ for cap in ranked:                                   │
  │   provider = resolve_provider(cap)                   │
  │   if provider is available:                          │
  │     selected.append({cap, provider})                 │
  │   elif cap.dependencies.optional_with exists:        │
  │     alternative = find_alternative(cap)              │
  │     selected.append({alternative, provider})         │
  │   else:                                              │
  │     escalate(UNAVAILABLE_CAPABILITY, cap)            │
  │                                                      │
  │ return selected                                      │
  └──────────────────────────────────────────────────────┘
```

#### Algorithm Performance Characteristics

| Step | Complexity | Typical Duration | Caching |
|------|-----------|-----------------|---------|
| CLASSIFY | O(1) | < 100ms | Request type cache |
| MATCH | O(n) where n = categories | < 500ms | Category index |
| RESOLVE DEPENDENCIES | O(v + e) where v = capabilities, e = dependency edges | < 1000ms | Dependency graph cache |
| COMPOSE | O(k log k) where k = resolved capabilities | < 200ms | Composition pattern cache |
| RANK | O(k log k) | < 100ms | Quality score index |
| SELECT & BIND | O(k × p) where p = providers per capability | < 500ms | Provider availability cache |

---

### 2.4 Request → Capability Resolution Matrix

This is the **complete resolution matrix** for every supported request type. The Capability Engine uses this matrix as its primary matching rule set.

#### Feature Requests

| Subtype | Primary Capabilities | Supporting Capabilities | Workflow |
|---------|---------------------|------------------------|----------|
| New feature | CAP-PROD-001, CAP-ARCH-001 | CAP-ENG-*, CAP-DATA-*, CAP-QUAL-* | feature-development |
| Enhancement | CAP-PROD-001, CAP-ENG-* | CAP-ARCH-005, CAP-QUAL-* | feature-development |
| Integration | CAP-INT-001, CAP-ARCH-004 | CAP-ENG-001, CAP-SEC-003 | feature-development |
| UI change | CAP-PROD-002, CAP-ENG-002 | CAP-ARCH-001, CAP-QUAL-002 | feature-development |
| API change | CAP-ENG-001, CAP-ARCH-005 | CAP-ENG-004, CAP-SEC-003 | feature-development |
| Database change | CAP-ENG-004, CAP-DATA-001 | CAP-ARCH-002, CAP-QUAL-003 | feature-development |
| Mobile feature | CAP-ENG-003, CAP-ARCH-001 | CAP-PROD-002, CAP-QUAL-004 | feature-development |

#### Bug Fixes

| Subtype | Primary Capabilities | Supporting Capabilities | Workflow |
|---------|---------------------|------------------------|----------|
| Critical bug | CAP-ENG-*, CAP-DATA-* | CAP-QUAL-*, CAP-SEC-* | bug-fix |
| Security bug | CAP-SEC-001, CAP-SEC-006 | CAP-ENG-*, CAP-QUAL-002 | bug-fix (emergency) |
| Performance bug | CAP-ENG-*, CAP-DATA-005 | CAP-QUAL-006, CAP-INFRA-005 | bug-fix |
| UI bug | CAP-ENG-002, CAP-PROD-002 | CAP-QUAL-002, CAP-QUAL-004 | bug-fix |
| Regression | CAP-ENG-*, CAP-QUAL-* | CAP-ARCH-007, CAP-SEC-006 | bug-fix |
| Data integrity | CAP-DATA-005, CAP-ENG-004 | CAP-SEC-001, CAP-QUAL-003 | bug-fix |

#### Refactoring

| Subtype | Primary Capabilities | Supporting Capabilities | Workflow |
|---------|---------------------|------------------------|----------|
| Code refactor | CAP-ENG-*, CAP-ARCH-002 | CAP-QUAL-001, CAP-QUAL-002 | refactoring |
| Architecture refactor | CAP-ARCH-001, CAP-ARCH-002 | CAP-ENG-*, CAP-QUAL-001 | refactoring |
| Database refactor | CAP-ENG-004, CAP-DATA-004 | CAP-ARCH-002, CAP-QUAL-003 | refactoring |
| API refactor | CAP-ENG-001, CAP-ARCH-005 | CAP-SEC-003, CAP-QUAL-001 | refactoring |
| Migration | CAP-DATA-004, CAP-ENG-* | CAP-ARCH-001, CAP-OPS-001 | refactoring |

#### Architecture

| Subtype | Primary Capabilities | Supporting Capabilities | Workflow |
|---------|---------------------|------------------------|----------|
| System design | CAP-ARCH-001, CAP-ARCH-005 | CAP-ENG-*, CAP-SEC-001 | planning |
| ADR creation | CAP-ARCH-003, CAP-GOV-002 | CAP-DOC-* | documentation |
| Stack evaluation | CAP-ARCH-006, CAP-ARCH-001 | CAP-INFRA-*, CAP-AI-005 | planning |
| Integration design | CAP-ARCH-004, CAP-INT-001 | CAP-SEC-001, CAP-ENG-* | planning |
| Standards update | CAP-ARCH-005, CAP-GOV-001 | CAP-ARCH-007 | planning |

#### Documentation

| Subtype | Primary Capabilities | Supporting Capabilities | Workflow |
|---------|---------------------|------------------------|----------|
| API docs | CAP-GOV-002, CAP-ENG-001 | CAP-QUAL-* | documentation |
| README update | CAP-GOV-002 | CAP-DOC-* | documentation |
| ADR | CAP-ARCH-003, CAP-GOV-002 | CAP-ARCH-* | documentation |
| Changelog | CAP-GOV-002 | CAP-OPS-004 | documentation |
| Architecture docs | CAP-ARCH-003, CAP-GOV-002 | CAP-ARCH-001 | documentation |

#### Deployment

| Subtype | Primary Capabilities | Supporting Capabilities | Workflow |
|---------|---------------------|------------------------|----------|
| Standard deploy | CAP-OPS-001, CAP-OPS-004 | CAP-INFRA-002, CAP-SEC-006 | deployment |
| Rollback | CAP-OPS-004, CAP-DATA-005 | CAP-INFRA-006, CAP-INFRA-002 | deployment |
| Hotfix | CAP-OPS-001, CAP-SEC-* | CAP-ENG-*, CAP-QUAL-* | deployment (emergency) |
| Canary | CAP-OPS-001, CAP-GOV-004 | CAP-INFRA-002, CAP-OPS-002 | deployment |

#### Incident

| Subtype | Primary Capabilities | Supporting Capabilities | Workflow |
|---------|---------------------|------------------------|----------|
| Production outage | CAP-OPS-002, CAP-OPS-003 | CAP-ENG-*, CAP-INFRA-* | incident |
| Security incident | CAP-SEC-002, CAP-SEC-008 | CAP-OPS-003, CAP-INFRA-* | security-audit |
| Performance degradation | CAP-QUAL-006, CAP-OPS-002 | CAP-INFRA-005, CAP-ENG-* | performance-audit |
| Data loss | CAP-DATA-005, CAP-INFRA-006 | CAP-SEC-001, CAP-ENG-004 | incident |

#### Research

| Subtype | Primary Capabilities | Supporting Capabilities | Workflow |
|---------|---------------------|------------------------|----------|
| Technology research | CAP-ARCH-006, CAP-AI-* | CAP-ARCH-001, CAP-ENG-* | planning |
| Architecture research | CAP-ARCH-001, CAP-ARCH-006 | CAP-SEC-001, CAP-INFRA-* | planning |
| Security research | CAP-SEC-008, CAP-SEC-002 | CAP-ARCH-006, CAP-ENG-* | security-audit |

#### Review

| Subtype | Primary Capabilities | Supporting Capabilities | Workflow |
|---------|---------------------|------------------------|----------|
| Code review | CAP-QUAL-001 | CAP-ARCH-007, CAP-SEC-* | code-review |
| Architecture review | CAP-ARCH-007, CAP-QUAL-001 | CAP-SEC-001, CAP-ENG-* | code-review |
| Security review | CAP-SEC-001, CAP-QUAL-001 | CAP-ARCH-007, CAP-ENG-* | security-audit |
| Performance review | CAP-QUAL-006, CAP-QUAL-001 | CAP-ENG-*, CAP-DATA-005 | performance-audit |

#### Maintenance

| Subtype | Primary Capabilities | Supporting Capabilities | Workflow |
|---------|---------------------|------------------------|----------|
| Dependency update | CAP-ENG-*, CAP-SEC-006 | CAP-QUAL-002, CAP-OPS-001 | dependency-update |
| Certificate rotation | CAP-SEC-005, CAP-SEC-003 | CAP-OPS-001 | maintenance |
| Backup verification | CAP-DATA-005, CAP-INFRA-006 | CAP-OPS-002 | maintenance |
| Log rotation | CAP-OPS-002, CAP-ENG-006 | CAP-INFRA-* | maintenance |

---

### 2.5 Capability Composition Rules

When multiple capabilities are required for a single request, they compose through one of six formal composition patterns.

#### Composition Pattern Catalog

| Pattern | Symbol | Semantics | Execution Model | Example |
|---------|--------|-----------|----------------|---------|
| **AND** | ∧ | All capabilities required | Parallel or sequence | Feature = PROD-001 ∧ ARCH-001 ∧ ENG-001 |
| **OR** | ∨ | Any capability suffices | First available wins | Auth = SEC-003 ∨ INT-003 |
| **SEQUENCE** | → | Ordered, each feeds next | Pipeline | Feature → Review → QA → Deploy |
| **PARALLEL** | ∥ | Independent, concurrent | Parallel DAG | Frontend ∥ Backend ∥ Database |
| **CONDITIONAL** | ? | Only if condition met | Branch | Security audit ? SEC-* : skip |
| **PREFERRED** | ≻ | Primary with fallback | Primary → Fallback | Provider ≻ Alternative Provider |

#### Composition Resolution Algorithm

```
Step 1: Identify composition pattern from request type
Step 2: Evaluate AND groups (all must resolve)
Step 3: Evaluate OR groups (select best available)
Step 4: Order SEQUENCE groups by dependency chain
Step 5: Group PARALLEL capabilities by independence
Step 6: Evaluate CONDITIONAL branches
Step 7: Apply PREFERRED fallbacks
Step 8: Generate final execution DAG

Output: DAG with composition-annotated edges
  - AND edges: must-execute, no alternative
  - OR edges: labeled with alternatives
  - SEQUENCE edges: ordered, labeled with data flow
  - PARALLEL edges: no dependency
  - CONDITIONAL edges: labeled with guard condition
  - PREFERRED edges: labeled with fallback
```

#### Composition Constraints

| Constraint | Description | Violation Handling |
|-----------|-------------|-------------------|
| No circular composition | A → B → A is invalid | Detection → Resolution abort |
| Consistent OR alternatives | OR alternatives must produce same output type | Validation → Provider mismatch error |
| Sequence data compatibility | SEQUENCE steps must have compatible input/output | Contract validation error |
| Parallel resource limits | PARALLEL capabilities must not exceed concurrency limits | Queue or degrade |
| Conditional determinism | CONDITIONAL guards must be deterministic | Non-deterministic → warning |

---

### 2.6 Capability Dependency Resolution

The Capability Engine resolves capability dependencies transitively, producing a complete dependency graph.

#### Dependency Graph Construction

```
Registry:                         Resolved Graph:
┌──────────────┐                 ┌────────────────────────────────┐
│ CAP-PROD-001 │                 │            Request             │
│ depends: []  │                 │                                │
└──────────────┘                 │  ┌──────────┐                  │
        │                        │  │PROD-001  │                  │
        ▼                        │  └────┬─────┘                  │
┌──────────────┐                 │       │                        │
│ CAP-ARCH-001 │                 │       ▼                        │
│ depends: []  │                 │  ┌──────────┐  ┌──────────┐   │
└──────────────┘                 │  │ARCH-001  │  │ARCH-005  │   │
        │                        │  └────┬─────┘  └────┬─────┘   │
        ▼                        │       │             │          │
┌──────────────┐                 │       ├─────────────┘          │
│ CAP-ARCH-005 │                 │       ▼                        │
│ depends: []  │                 │  ┌──────────┐                  │
└──────────────┘                 │  │ENG-001   │                  │
        │                        │  └────┬─────┘                  │
        ▼                        │       │                        │
┌──────────────┐                 │       ▼                        │
│ CAP-ENG-001  │                 │  ┌──────────┐  ┌──────────┐   │
│ depends:     │                 │  │QUAL-002  │  │SEC-003   │   │
│   CAP-QUAL-00│                 │  └──────────┘  └──────────┘   │
│   CAP-SEC-003│                 └────────────────────────────────┘
└──────────────┘
```

#### Dependency Types

| Type | Description | Resolution Behavior |
|------|-------------|-------------------|
| **requires** | Hard dependency, must resolve | If unavailable → capability unavailable |
| **optional_with** | Alternative capability acceptable | If unavailable → substitute with alternative |
| **enhances** | Improves quality but not required | If unavailable → degraded quality |
| **conflicts_with** | Cannot coexist | If both requested → abort or select one |

#### Circular Dependency Detection

```yaml
circular_detection_algorithm:
  method: "DFS with back-edge detection"
  complexity: O(V + E)
  action_on_detect:
    - Log CIRCULAR_DEPENDENCY_ERROR
    - Include cycle path in error details
    - Abort resolution
    - Escalate to Architecture Chief
    - Flag for Capability Registry audit
```

---

### 2.7 Capability Quality & SLOs

Every capability has a quality score and SLOs that the Capability Engine uses for ranking and provider selection.

#### Quality Score Formula

```
CAPABILITY_QUALITY = 0.30 × delivery_quality
                   + 0.25 × provider_reliability
                   + 0.20 × contract_compliance
                   + 0.15 × dependency_health
                   + 0.10 × freshness

Where:
  delivery_quality      = Average quality score from last 10 deliveries (0-10)
  provider_reliability  = Provider success rate over last 100 resolutions (0-1) × 10
  contract_compliance   = Contract violation score (0-10, 10 = no violations)
  dependency_health     = Average quality of all dependencies (0-10)
  freshness             = Recency weight: 10 if updated <30 days, decays to 5 over 180 days
```

#### SLO Catalog by Category

| Category | p50 Duration | p95 Duration | p99 Duration | Availability | Max Failure Rate |
|----------|-------------|-------------|-------------|-------------|-----------------|
| Architecture | 15s | 60s | 120s | 99.9% | 0.1% |
| Engineering | 120s | 600s | 1800s | 99.5% | 1.0% |
| Quality | 60s | 300s | 900s | 99.8% | 0.5% |
| Security | 30s | 120s | 300s | 99.95% | 0.05% |
| Infrastructure | 60s | 300s | 600s | 99.9% | 0.1% |
| AI | 30s | 120s | 300s | 99.5% | 1.0% |
| Data | 30s | 120s | 300s | 99.8% | 0.5% |
| Platform | 15s | 60s | 120s | 99.95% | 0.05% |
| Governance | 10s | 30s | 60s | 99.99% | 0.01% |
| Product | 20s | 60s | 120s | 99.8% | 0.5% |
| Operations | 30s | 120s | 300s | 99.95% | 0.05% |
| Integration | 30s | 120s | 300s | 99.5% | 1.0% |

#### Quality Score Interpretation

| Score | Grade | Meaning | Provider Action |
|-------|-------|---------|-----------------|
| 9.0–10.0 | A | Excellent | No action needed |
| 7.0–8.9 | B | Good | Minor improvements suggested |
| 5.0–6.9 | C | Acceptable | Improvement plan required |
| 3.0–4.9 | D | Poor | Provider review, possible replacement |
| 0.0–2.9 | F | Critical | Immediate escalation to CTO |

---

### 2.8 Capability Orchestration Protocol

The Runtime orchestrates capabilities through a formal protocol with defined handoffs, synchronization points, and quality gates.

#### Orchestration Flow

```
REQUEST
  │
  ▼
┌──────────────────────────────────────────────────────────────────┐
│                   CAPABILITY ENGINE                              │
│                                                                  │
│  Resolve Capabilities → Bind Providers → Generate Orchestration  │
│                                                                  │
│  Output: OrchestrationPlan { capabilities[], composition,        │
│           providers[], handoffs[], quality_gates[] }             │
└──────────────────────────────┬───────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│                   ORCHESTRATION EXECUTOR                         │
│                                                                  │
│  For each capability in plan:                                    │
│    1. PREPARE: Validate provider readiness                       │
│    2. EXECUTE: Invoke provider with capability contract          │
│    3. VERIFY: Check output against contract                      │
│    4. HANDOFF: Pass output to next capability in sequence        │
│    5. GATE: Apply quality gate if configured                     │
│                                                                  │
│  Composition patterns determine execution order:                 │
│    AND:      Execute all, aggregate results                      │
│    OR:       Execute first available, skip others                │
│    SEQUENCE: Execute in order, pipe output to next               │
│    PARALLEL: Execute concurrently, merge when all complete       │
│    CONDITIONAL: Evaluate guard, execute or skip branch           │
│    PREFERRED: Try primary, fall back to alternative on failure   │
└──────────────────────────────────────────────────────────────────┘
```

#### Handoff Protocol

Between capabilities in a SEQUENCE composition, data is passed through a formal handoff:

```yaml
handoff:
  source_capability: "CAP-XXX-XXX"
  target_capability: "CAP-YYY-YYY"

  contract:
    output_of_source_matches_input_of_target: true

  data:
    passed_directly: []      # Fields passed as-is
    transformed: []           # Fields with transformation rules
    enriched: []              # Fields enriched by source

  quality_gate:
    required_before_handoff: true
    gate_type: "contract_validation | quality_check"

  error_handling:
    on_mismatch: "reject | transform | escalate"
```

---

### 2.9 Capability Availability & Fallback

When a capability is unavailable, the Capability Engine follows a formal fallback chain.

#### Availability States

| State | Definition | Resolution Behavior |
|-------|-----------|-------------------|
| **AVAILABLE** | Provider ready, within SLO | Normal resolution |
| **DEGRADED** | Provider responding, SLO exceeded | Resolve with quality penalty |
| **LIMITED** | Provider at concurrency limit | Queue or use alternative |
| **UNAVAILABLE** | Provider not responding | Trigger fallback |
| **MAINTENANCE** | Scheduled downtime | Reschedule or use alternative |

#### Fallback Chain

```
Capability Unavailable
  │
  ▼
┌──────────────────────────────────┐
│ Level 1: Alternative Capability  │
│ (cap.dependencies.optional_with) │
└──────────┬───────────────────────┘
           │ Available?
           ├── YES → Use alternative
           │
           ▼ NO
┌──────────────────────────────────┐
│ Level 2: Degraded Mode           │
│ (Capability without non-critical │
│  features)                       │
└──────────┬───────────────────────┘
           │ Available?
           ├── YES → Use degraded, log warning
           │
           ▼ NO
┌──────────────────────────────────┐
│ Level 3: Manual Provider         │
│ (Fallback provider or human)     │
└──────────┬───────────────────────┘
           │ Available?
           ├── YES → Use manual, flag for review
           │
           ▼ NO
┌──────────────────────────────────┐
│ Level 4: Escalate                │
│ (Escalate to CTO/CEO)           │
└──────────────────────────────────┘
```

---

### 2.10 Capability Discovery & Registration

Capabilities are discovered and registered through an automated protocol.

#### Registration Protocol

```yaml
registration:
  trigger: "Capability Engine startup | Provider deployment | Manual request"

  steps:
    1. submit_proposal:
        artifact: "Capability Contract (YAML)"
        target: "Capability Engine"

    2. validate_contract:
        checks:
          - All required fields present
          - ID does not conflict
          - Provider exists in ORGCHART.md
          - Dependencies reference existing capabilities
          - No circular dependencies

    3. approve:
        authority: "CTO (for new) | Provider Chief (for updates)"
        artifact: "Approval ADR"

    4. register:
        action: "Add to CAPABILITY_CATALOG.md"
        state: "REGISTERED"

    5. activate:
        precondition: "Provider ready + Gate 1 passed"
        state: "ACTIVE"
        event: "CapabilityRegistered"

  auto_discovery:
    enabled: true
    scan: "engines/*/SKILL.md and departments/*/SKILL.md"
    pattern: "capabilities: [CAP-XXX-*]"
```

#### Registry Synchronization

```
Capability Registry (CAPABILITY_CATALOG.md)
  │
  ├──→ Capability Engine (in-memory cache)
  │     │
  │     ├──→ Resolution queries (real-time)
  │     └──→ Metrics collection (periodic)
  │
  ├──→ Runtime (session start sync)
  │
  ├──→ Dashboard (API queries)
  │
  └──→ COSCA_INDEX.md (file inventory reference)
```

---

### 2.11 Capability Isolation & Conflict Resolution

Capabilities must not conflict. The Capability Engine detects and resolves conflicts automatically.

#### Conflict Types

| Type | Description | Detection | Resolution |
|------|-------------|-----------|------------|
| **Resource** | Two capabilities need same resource | Resource lock detection | Queue, sequentialize |
| **Dependency** | Circular dependency between capabilities | DFS cycle detection | Abort, escalate |
| **Side-effect** | Capabilities modify same state | Write-set intersection | Sequentialize, warn |
| **Priority** | Both claim high priority for different work | Priority comparison | Higher priority wins |
| **Exclusive** | Capabilities declared as mutually exclusive | conflicts_with field | Select one, reject other |

#### Isolation Rules

```
Rule CI-01: Capabilities MUST NOT modify each other's contracts
Rule CI-02: Capabilities MUST NOT access each other's provider directly
Rule CI-03: Capabilities MUST declare all resource requirements
Rule CI-04: Capabilities MUST release resources after completion
Rule CI-05: Capabilities MUST NOT bypass quality gates of other capabilities
Rule CI-06: Capabilities MUST log all cross-capability interactions
Rule CI-07: Capabilities MUST respect isolation boundaries defined in ARCH-002
```

---

### 2.12 Provider → Capability Binding

Every capability MUST be bound to at least one provider. Providers claim capabilities through a formal binding protocol.

#### Binding Types

| Binding | Description | Example |
|---------|-------------|---------|
| **Primary** | Chief/Engine owns this capability | Architecture Chief → CAP-ARCH-001 |
| **Shared** | Multiple providers co-own | Architecture Chief + Documentation Chief → CAP-ARCH-003 |
| **Delegated** | Provider delegates to sub-provider | QA Chief → Testing Chief → CAP-QUAL-002 |
| **Engine** | Engine provides automatically | Benchmark Engine → CAP-QUAL-007 |
| **External** | External plugin/SDK provides | Plugin System → CAP-PLAT-002 |

#### Provider Capability Limits

Each provider has defined limits on how many capabilities they can serve simultaneously:

| Provider Type | Max Concurrent Capabilities | Max Queue Depth | Degradation Threshold |
|--------------|---------------------------|-----------------|----------------------|
| Chief | 10 | 50 | >8 concurrent |
| Specialist | 3 | 20 | >2 concurrent |
| Engine | 100 | 500 | >80 concurrent |
| External | 50 | 200 | >40 concurrent |

---

### 2.13 Capability Catalog Reference

The complete inventory of all 64 capabilities across 12 categories is maintained in:

> **[capabilities/CAPABILITY_CATALOG.md](../capabilities/CAPABILITY_CATALOG.md)**

This section defines the **architecture** of how capabilities work. The catalog defines the **inventory** of what capabilities exist. Both are binding.

| Artifact | Location | Purpose |
|----------|----------|---------|
| Capability Architecture | This section (KERNEL.md §2) | How capabilities work |
| Capability Contract Template | [capabilities/CAPABILITY_TEMPLATE.md](../capabilities/CAPABILITY_TEMPLATE.md) | Standard contract format |
| Capability Inventory | [capabilities/CAPABILITY_CATALOG.md](../capabilities/CAPABILITY_CATALOG.md) | Complete registry of 64 capabilities |
| Capability Engine | [engines/capability/SKILL.md](../engines/capability/SKILL.md) | Resolution and orchestration implementation |


