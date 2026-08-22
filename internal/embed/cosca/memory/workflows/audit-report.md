# Cosca Workflow Audit Report

> **Auditor**: cosca-workflow-chief | **Date**: 2026-07-28 | **Version**: 1.0.0
>
> **Scope**: Full audit of all 28 platform workflows — catalog, quality, gaps, and recommendations.

---

## 1. EXECUTIVE SUMMARY

The Cosca platform has **28 workflow definitions** stored in two mirrored locations:
- `internal/embed/cosca/workflows/` (28 `.md` files)
- `internal/embed/cosca/workflows/` (28 `.md` files, embedded via Go `embed.FS`)

Workflows are parsed by the `internal/workflows/workflows.go` engine from Markdown files with optional YAML frontmatter. The engine supports two execution paths: a **fallback** (sequential step-by-step simulation) and a **pipeline** path (delegates to `orchestration.Pipeline`).

**4 MCP tools** expose workflows to external clients: `cosca_workflow_list`, `cosca_workflow_inspect`, `cosca_workflow_search`, `cosca_workflow_run`. SSE streaming for step-by-step progress is supported via `POST /v1/workflows/{name}/run/stream`. CLI access via `cosca workflow {list,show,run,search}`.

**Overall health**: All 28 workflows load and are discoverable. 1 is **draft** (cosca-evolution-autonomy), 27 are **active**. Several quality issues exist (format inconsistencies, limited engine support for schema fields, missing metrics).

---

## 2. WORKFLOW CATALOG

### 2.1 Classification Matrix

| # | Workflow | Version | Status | Steps | Category | Created |
|---|----------|---------|--------|-------|----------|---------|
| 1 | feature-development | 1.1.0 | active | 16 | Development | 2026-07-10 |
| 2 | bug-fix | 1.1.0 | active | 8 | Development | 2026-07-10 |
| 3 | code-review | 1.1.0 | active | 8 | Development | 2026-07-10 |
| 4 | refactoring | 1.1.0 | active | 7 | Development | 2026-07-10 |
| 5 | project-init | 2.0.0 | active | 12 | Development | 2026-07-10 |
| 6 | api-design-review | 1.0.0 | active | 5 | Development | 2026-07-23 |
| 7 | architecture-review-board | 1.0.0 | active | 6 | Development | 2026-07-23 |
| 8 | deployment | 1.0.0 | active | 7 | Operations | 2026-07-10 |
| 9 | release | 1.0.0 | active | 8 | Operations | 2026-07-10 |
| 10 | incident-response | 1.0.0 | active | 6 | Operations | 2026-07-23 |
| 11 | disaster-recovery | 1.0.0 | active | 7 | Operations | 2026-07-23 |
| 12 | platform-bootstrap | 1.0.0 | active | 8 | Operations | 2026-07-23 |
| 13 | capacity-planning | 1.0.0 | active | 7 | Operations | 2026-07-23 |
| 14 | secrets-rotation | 1.0.0 | active | 7 | Operations | 2026-07-23 |
| 15 | security-audit | 1.0.0 | active | 8 | Quality | 2026-07-10 |
| 16 | performance-audit | 1.0.0 | active | 8 | Quality | 2026-07-10 |
| 17 | compliance-audit | 1.0.0 | active | 6 | Quality | 2026-07-23 |
| 18 | data-privacy-impact | 1.0.0 | active | 7 | Quality | 2026-07-23 |
| 19 | audit-runtime-sync | 1.0.0 | active | 6 | Quality | 2026-07-28 |
| 20 | chaos-testing | 1.0.0 | active | 7 | Quality | 2026-07-23 |
| 21 | metacognition-pipeline | 1.0.0 | active | 8 | Quality | 2026-07-28 |
| 22 | dependency-update | 1.0.0 | active | 7 | Maintenance | 2026-07-10 |
| 23 | dependency-upgrade | 1.0.0 | active | 7 | Maintenance | 2026-07-23 |
| 24 | migration-execution | 1.0.0 | active | 7 | Maintenance | 2026-07-23 |
| 25 | provider-migration | 1.0.0 | active | 7 | Maintenance | 2026-07-23 |
| 26 | technical-debt-paydown | 1.0.0 | active | 7 | Maintenance | 2026-07-23 |
| 27 | performance-optimization | 1.0.0 | active | 7 | Maintenance | 2026-07-23 |
| 28 | cosca-evolution-autonomy | 1.0.0 | **draft** | 5 | Maintenance | 2026-07-28 |

### 2.2 Distribution by Category

```
Development  (7):  25.0%  ████████████████████████
Operations   (7):  25.0%  ████████████████████████
Quality      (7):  25.0%  ████████████████████████
Maintenance  (7):  25.0%  ████████████████████████
```
Perfectly balanced 4-ways.

### 2.3 Distribution by Creation Era

| Era | Count | Workflows |
|-----|-------|-----------|
| Wave 1 (Jul 10-12, "Cosca Kernel") | 11 | feature-development, bug-fix, code-review, refactoring, project-init, deployment, release, security-audit, performance-audit, dependency-update, metacognition-pipeline |
| Wave 2 (Jul 23, "Enterprise Evolution") | 15 | incident-response, disaster-recovery, platform-bootstrap, capacity-planning, secrets-rotation, compliance-audit, data-privacy-impact, chaos-testing, dependency-upgrade, migration-execution, provider-migration, technical-debt-paydown, performance-optimization, api-design-review, architecture-review-board |
| Wave 3 (Jul 28, "Autonomy") | 2 | audit-runtime-sync, cosca-evolution-autonomy |

### 2.4 Depth Analysis (Steps per Workflow)

| Complexity Tier | Steps | Count | Workflows |
|-----------------|-------|-------|-----------|
| Deep (>10) | 12-16 | 2 | feature-development (16), project-init (12) |
| Medium (7-8) | 7-8 | 16 | bug-fix (8), code-review (8), deployment (7), release (8), security-audit (8), performance-audit (8), platform-bootstrap (8), chaos-testing (7), dependency-update (7), dependency-upgrade (7), migration-execution (7), provider-migration (7), technical-debt-paydown (7), performance-optimization (7), data-privacy-impact (7), secrets-rotation (7) |
| Shallow (5-6) | 5-6 | 6 | refactoring (7), api-design-review (5), architecture-review-board (6), compliance-audit (6), audit-runtime-sync (6), incident-response (6), capacity-planning (7), cosca-evolution-autonomy (5) |
| (Meta) | 8 stages | 1 | metacognition-pipeline |

---

## 3. ENGINE & INFRASTRUCTURE AUDIT

### 3.1 Workflow Engine (`internal/workflows/workflows.go`)

| Component | Status | Notes |
|-----------|--------|-------|
| **Parser** | Working | Supports YAML frontmatter + inline Markdown parsing |
| **Manager** | Working | Loads from embedded + local `.cosca/workflows/` + global `~/.config/opencode/cosca/workflows/` |
| **Run (fallback)** | Working | Sequential step-by-step execution with StepRunner callback |
| **Run (pipeline)** | Working | Delegates to `orchestration.Pipeline.Execute` when configured |
| **RunWithProgress** | Working | SSE-compatible step-by-step progress reporting |
| **Search** | Working | Case-insensitive substring search across name + description |
| **List** | Working | Returns all registered workflows |

### 3.2 Schema vs. Implementation Gap

The `workflow/SKILL.md` defines a **rich schema** with fields not yet implemented in the parser or engine:

| Field | Schema (SKILL.md) | Parser (workflows.go) | Engine |
|-------|-------------------|----------------------|--------|
| `name` | ✅ | ✅ | ✅ |
| `version` | ✅ | ✅ | ✅ |
| `description` | ✅ | ✅ | ✅ |
| `category` | ✅ (info line) | ❌ | ❌ |
| `inputs` | ✅ | ✅ | ⚠️ (parsed, not validated) |
| `outputs` | ✅ | ✅ | ⚠️ (parsed, not validated) |
| `preconditions` | ✅ | ❌ | ❌ |
| `postconditions` | ✅ | ❌ | ❌ |
| `dependencies` | ✅ | ❌ | ❌ |
| `steps[].retry` | ✅ | ❌ | ❌ |
| `steps[].depends_on` | ✅ | ❌ | ❌ |
| `steps[].parallel` | ✅ | ❌ | ❌ |
| `steps[].review_required` | ✅ | ❌ | ❌ |
| `steps[].qa_required` | ✅ | ❌ | ❌ |
| `validation` | ✅ | ❌ | ❌ |
| `success_criteria` | ✅ | ❌ | ❌ |
| `failure_handlers` | ✅ | ❌ | ❌ |
| `retry` | ✅ | ❌ | ❌ |
| `error_handling` | ✅ (table) | ❌ | ❌ |

**Implementation coverage**: ~40% of schema fields are parsed and usable at runtime.

### 3.3 MCP Tools Coverage

| Tool | Schema | Handler | Tests |
|------|--------|---------|-------|
| `cosca_workflow_list` | ✅ (`enabled_only` filter) | ✅ | ✅ (2 tests) |
| `cosca_workflow_inspect` | ✅ (`name` required) | ✅ | ✅ (3 tests) |
| `cosca_workflow_search` | ✅ (`query` required) | ✅ | ✅ (1 test) |
| `cosca_workflow_run` | ✅ (`name` + `timeout_seconds`) | ✅ | ✅ (3 tests) |

All 4 tools are operational and tested. The `cosca_workflow_run` tool has an intentional limitation: workflow definitions with `StepList` but no `PipelineExecutor` will simulate execution (10ms per step) rather than performing real work.

### 3.4 SSE Streaming

The `POST /v1/workflows/{name}/run/stream` endpoint emits:
- `step` events: `StepProgress` per step (started/completed/failed) with step_num, total, duration_ms
- `done` event: final `Result` on success
- `error` event: error message on failure
- WebSocket broadcast: step status and workflow completion/failure forwarded to subscribers on `workflow` topic

---

## 4. QUALITY AUDIT

### 4.1 Format Consistency

| Section | Wave 1 (11) | Wave 2 (15) | Wave 3 (2) |
|---------|-------------|-------------|------------|
| Version + Status info line | ✅ All | ✅ All | ⚠️ Mixed |
| OBJECTIVE | ✅ All | ✅ All | ✅ All |
| INPUTS table | ✅ 10/11 | ✅ 15/15 | ⚠️ 1/2 |
| OUTPUTS table | ✅ 10/11 | ✅ 15/15 | ⚠️ 1/2 |
| PRECONDITIONS | ✅ 8/11 | ✅ 12/15 | ❌ 0/2 |
| POSTCONDITIONS | ✅ 8/11 | ✅ 10/15 | ❌ 0/2 |
| DEPENDENCIES | ✅ 5/11 | ✅ 8/15 | ❌ 0/2 |
| STEPS (numbered) | ✅ All | ✅ All | ✅ All |
| VALIDATION | ✅ 8/11 | ✅ 8/15 | ❌ 0/2 |
| SUCCESS CRITERIA | ✅ All | ✅ All | ✅ 1/2 |
| ERROR HANDLING | ✅ 8/11 | ✅ 12/15 | ❌ 0/2 |
| RELATED | ✅ All | ✅ All | ✅ All |
| HISTORY | ✅ All | ✅ All | ❌ 1/2 |

**Legacy-grade** (full format): feature-development, bug-fix, code-review, refactoring, project-init
**Enterprise-grade** (good format): All 15 Wave 2 workflows, deployment, release, security-audit, performance-audit, dependency-update
**Inconsistent**: audit-runtime-sync (non-standard heading format, no PRECONDITIONS/POSTCONDITIONS), cosca-evolution-autonomy (draft, uses Brazilian Portuguese, very long — 663 lines)

### 4.2 Step Atomicity & Idempotency

| Dimension | Score | Notes |
|-----------|-------|-------|
| Step atomicity | ⚠️ Mixed | Some steps are atomic ("Run dependency scan"), others are composite ("Implement backend changes" spans multiple sub-tasks) |
| Step idempotency | ❌ Not defined | No workflow specifies idempotency guarantees. No `idempotency_key` support in engine |
| Step ordering | ✅ Clear | Steps use sequential numbering (Step 1, Step 2...) or phase/sub-step (Step 1.1, Step 1.2...) |
| Step dependencies | ⚠️ Partial | feature-development and project-init document `Depends On` within step details, but these are NOT parsed by the engine |

### 4.3 Error Handling & Retry

| Feature | Implementation |
|---------|---------------|
| Error handling definitions | ✅ 20/28 workflows have ERROR HANDLING tables |
| Retry logic | ❌ No retry implemented in engine. `runFallback` breaks on first failure. `runWithPipeline` reports single pipeline error |
| Rollback | ✅ deployment, migration-execution, secrets-rotation, provider-migration, disaster-recovery have rollback steps |
| Escalation paths | ✅ Most ERROR HANDLING tables specify escalation targets |
| Timeout handling | ✅ Engine supports per-step timeout via `time.Timeout` and `context.WithTimeout` |

### 4.4 Documentation Completeness

| Metric | Value |
|--------|-------|
| Workflows with HISTORY section | 27/28 |
| Workflows with VALIDATION section | 16/28 |
| Workflows with PRECONDITIONS | 20/28 |
| Workflows with POSTCONDITIONS | 18/28 |
| Workflows with DEPENDENCIES | 13/28 |
| Workflows with ERROR HANDLING | 20/28 |
| Workflows with SUCCESS CRITERIA | 27/28 |
| Workflows with RELATED links | 28/28 |
| Cross-referencing between workflows | ✅ Good — feature-development → release, release → deployment, etc. |

### 4.5 Testing Coverage

| Test Type | Location | Coverage |
|-----------|----------|----------|
| Unit tests (engine) | `internal/workflows/workflows_test.go` | ✅ 1590 lines, tests: parseTimeout, containsFold, Manager construction, Run fallback (6 tests), Run pipeline (4 tests), RunWithProgress (4 tests), Search, loadFromDir, parsing (7 tests), etc. |
| Unit tests (parser) | `internal/parser/parser_test.go` | ✅ Workflow entity parsing tested |
| MCP tests | `api/mcp/server_test.go` | ✅ 4 workflow tools tested with 10+ cases |
| Integration tests | None | ❌ No end-to-end workflow execution test |
| Workflow-specific tests | None | ❌ No test validates that all 28 workflows load and parse correctly |
| Workflow execution tests | None | ❌ No test runs a workflow with a real StepRunner |

---

## 5. ISSUES FOUND

### 5.1 Critical

| ID | Issue | File | Impact |
|----|-------|------|--------|
| **BUG-01** | **Typo in workflow name**: `# WORKFLOW: chcosca-testing` should be `# WORKFLOW: chaos-testing` | `internal/embed/cosca/workflows/chaos-testing.md:1` | Workflow registers as `chcosca-testing` instead of `chaos-testing` — search by correct name fails |
| **GAP-01** | **No retry support in engine** — step failure immediately breaks execution. Neither `runFallback` nor `runWithPipeline` retries failed steps. Schema defines retry but it is not implemented | `internal/workflows/workflows.go` | Workflows cannot self-heal from transient errors |
| **GAP-02** | **No parallel execution** — engine runs all steps sequentially even when `Depends On` allows parallelism. No DAG-based execution model | `internal/workflows/workflows.go` | Workflows like feature-development (where Step 4.4 is parallel with 4.2) cannot exploit parallelism |

### 5.2 High

| ID | Issue | File | Impact |
|----|-------|------|--------|
| **FMT-01** | **cosca-evolution-autonomy is draft** — only 1 of 28 workflows not in "active" status | `cosca-evolution-autonomy.md` | Workflow not discoverable by default if filtering for `status:active` |
| **FMT-02** | **audit-runtime-sync uses non-standard format** — top-level heading is `# AUDIT WORKFLOW — ...` instead of `# WORKFLOW: audit-runtime-sync`. Steps are written as prose with bash code blocks instead of structured step definitions | `audit-runtime-sync.md` | Parser cannot extract structured StepList — workflow runs with 0 parsed steps |
| **FMT-03** | **cosca-evolution-autonomy is 663 lines** — 5.9x the average workflow size (112 lines). Contains full implementation specifications for 5 phases (Constitution, Confidence Model, Memory Curation, Capability Profiles, Auto-Medication) | `cosca-evolution-autonomy.md` | Not a typical workflow — more of an implementation plan document |
| **PARSER-01** | **Parser only extracts 6 fields from steps** (Name, Description, Agent, Timeout). Ignores: Chief, Specialists, Task, Output, Depends On, On Failure | `internal/workflows/workflows.go:parseStepDetailLine` | Runtime cannot validate Chief assignments or enforce Depends On ordering |

### 5.3 Medium

| ID | Issue | Impact |
|----|-------|--------|
| **OBS-01** | **No workflow execution metrics** — `workflow.duration_ms`, `workflow.step.duration_ms`, `workflow.count`, `workflow.failure.count` defined in KERNEL.md contract but not implemented | No observability into workflow performance |
| **CI-01** | **No CI/CD integration** — workflows are not triggered by Git events, PR creation, or scheduled cron jobs. No webhook integration | Workflows are manual-only; cannot be part of automated pipelines |
| **DOC-01** | **Wave 1 workflows missing ERROR HANDLING**: deployment, release, security-audit, performance-audit, dependency-update (5 of 11) | These 5 have less error recovery documentation |
| **DOC-02** | **13 workflows lack DEPENDENCIES section** | Inter-workflow dependencies not formally tracked |
| **DOC-03** | **12 workflows lack PRECONDITIONS section** | Pre-condition verification not enforced |
| **DUPE-01** | **dependency-update vs dependency-upgrade** — two workflows with overlapping scope. `dependency-update` focuses on single-package updates; `dependency-upgrade` is broader (multi-manager, risk-tier) | Potential user confusion on which to use |

### 5.4 Low

| ID | Issue | Impact |
|----|-------|--------|
| **FMT-04** | Some workflows use `String` (capitalized) and others `string` (lowercase) for input types. No consistency | Minor inconsistency |
| **FMT-05** | `architecture-review-board.md` has PRECONDITIONS and POSTCONDITIONS after RELATED (non-standard order) | Minimal readability impact |
| **FMT-06** | `chaos-testing.md` and `secrets-rotation.md` have SUCCESS CRITERIA before RELATED (non-standard order) | Minimal readability impact |

---

## 6. MISSING WORKFLOWS

These workflows do not exist but should, based on the platform's capabilities and common development patterns:

| # | Workflow | Rationale | Priority |
|---|----------|-----------|----------|
| 1 | **workflow-audit** | Self-audit — who audits the workflow definitions? Needed for this exact report to be automatable | High |
| 2 | **rollback** | Generic rollback workflow reused by deployment, release, migration-execution | High |
| 3 | **database-backup** | Scheduled + on-demand database backup with verification. Referenced by disaster-recovery but not a standalone workflow | Medium |
| 4 | **monitoring-setup** | Bootstrap monitoring for a new service: dashboards, alerts, SLOs | Medium |
| 5 | **hotfix** | Documented in `docs/roadmap/hotfix-workflow.md` but no markdown definition exists in either workflows directory | Medium |
| 6 | **canary-deployment** | Canary-specific deployment variant (deployment.md mentions canary strategy but no dedicated workflow) | Medium |
| 7 | **ab-testing** | A/B testing workflow for feature flags and experiments | Low |
| 8 | **onboarding** | New team member onboarding workflow — access, tools, orientation | Low |
| 9 | **deprecation** | API/feature deprecation workflow with sunset timeline, deprecation notices, migration guides | Low |
| 10 | **license-audit** | Audit of all dependency licenses for compliance (complements compliance-audit) | Low |

---

## 7. OBSERVABILITY GAPS

| Gap | Current State | Recommended |
|-----|--------------|-------------|
| **Workflow execution metrics** | Defined in KERNEL.md contract (workflow.duration_ms, workflow.step.duration_ms, workflow.count, workflow.failure.count) but NOT implemented | Implement Prometheus metrics in the workflow engine |
| **Workflow audit log** | AuditStore records `workflow.run` and `workflow.run.stream` events | Add structured logging per step with context propagation |
| **Workflow health dashboard** | No dashboard | Add to platform-health-dashboard as planned in cosca-evolution-autonomy Phase E |
| **WebSocket events** | ✅ Implemented: step status, workflow completed, workflow failed broadcasted to `workflow` topic via Hub | Good — maintain |
| **Step-level tracing** | Not implemented | Add OpenTelemetry spans per step for distributed tracing |
| **Workflow success rate tracking** | Not implemented | Track per-workflow success/failure rates over time |

---

## 8. RECOMMENDATIONS

### 8.1 Immediate (this sprint)

| # | Action | Effort | Impact |
|---|--------|--------|--------|
| **R1** | **Fix BUG-01**: Rename `chcosca-testing` → `chaos-testing` in chaos-testing.md:1 | 1 min | Critical — unblocks workflow discovery |
| **R2** | **Promote cosca-evolution-autonomy** from draft to active, or explicitly document why it's draft | 5 min | Visibility |
| **R3** | **Normalize audit-runtime-sync.md** to standard format with `# WORKFLOW: audit-runtime-sync` and structured steps | 15 min | Parser compatibility |
| **R4** | **Add missing ERROR HANDLING** to deployment, release, security-audit, performance-audit, dependency-update | 20 min | Resilience documentation |

### 8.2 Short-term (next 2 sprints)

| # | Action | Effort | Impact |
|---|--------|--------|--------|
| **R5** | **Implement retry in engine**: Add `Step.Retry` field to struct, `maxRetries` loop in `runFallback` and `runWithPipeline` | 4h | Critical — workflows become resilient |
| **R6** | **Extend parser**: Parse `depends_on`, `retry`, `parallel`, `review_required`, `qa_required` from step details | 3h | Enables DAG-based execution |
| **R7** | **Add PRECONDITIONS/DEPENDENCIES** to all 13 workflows missing them | 2h | Completeness |
| **R8** | **Implement workflow metrics**: Export Prometheus counters/histograms for workflow.duration_ms, workflow.count, workflow.failure.count per KERNEL.md | 4h | Observability |
| **R9** | **Create workflow-audit workflow**: Automate this exact audit so it can run weekly | 2h | Self-maintenance |

### 8.3 Medium-term (next quarter)

| # | Action | Effort | Impact |
|---|--------|--------|--------|
| **R10** | **Implement DAG-based parallel execution**: Build dependency graph from `depends_on`, run independent steps in parallel | 8h | Performance — feature-development could run ~30% faster |
| **R11** | **Add CI/CD triggers**: Webhook handler for Git events → automatic workflow execution (e.g., code-review on PR, deploy on merge) | 6h | Automation |
| **R12** | **Create 10 missing workflows**: rollback, database-backup, monitoring-setup, hotfix, canary-deployment, ab-testing, onboarding, deprecation, license-audit | 10h | Coverage |
| **R13** | **Idempotency support**: Add `IdempotencyKey` to workflow execution; prevent duplicate runs | 4h | Safety |
| **R14** | **Workflow versioning**: Track workflow definition versions; detect when a running workflow's definition has changed | 4h | Reliability |

### 8.4 Long-term (next 6 months)

| # | Action | Effort | Impact |
|---|--------|--------|--------|
| **R15** | **Workflow composition**: Allow workflows to invoke sub-workflows (e.g., feature-development → code-review, bug-fix → release) | 12h | Reusability |
| **R16** | **Visual workflow editor**: Web UI with drag-and-drop step ordering and DAG visualization | 40h | UX |
| **R17** | **Workflow templates from CLI**: `cosca workflow create --template feature-development` | 8h | Developer experience |
| **R18** | **Multi-tenancy workflows**: Per-project workflow overrides; workflow inheritance across projects | 16h | Scale |

---

## 9. APPENDIX: Workflow Interconnection Map

```
project-init ─────────────────────────────────────────────────────┐
                                                                  │
feature-development ──→ code-review ──→ release ──→ deployment   │
     │                      ↑               │           │         │
     │                      │               │           │         │
     └──→ api-design-review─┘               │     disaster-recovery
            architecture-review-board       │           │         │
                                            │     incident-response
bug-fix ──→ code-review ────────────────────┘           │         │
                                                         │         │
refactoring ──→ code-review                     chaos-testing      │
                                                         │         │
security-audit ←── secrets-rotation ←────────────────────┘         │
     │                                                              │
     ├──→ dependency-update → dependency-upgrade                   │
     │                                                              │
     ├──→ compliance-audit → data-privacy-impact                   │
     │                                                              │
     └──→ migration-execution → provider-migration                 │
                                                                    │
performance-audit → performance-optimization → capacity-planning   │
                                                                    │
technical-debt-paydown ←──→ refactoring                            │
                                                                    │
platform-bootstrap ─────────────────────────────────────────────────┤
                                                                    │
metacognition-pipeline (meta — applies to all agent tasks)          │
                                                                    │
cosca-evolution-autonomy (meta — platform maturity roadmap)         │
                                                                    │
audit-runtime-sync (meta — cross-cuts all docs and code)            │
```

---

## 10. SIGN-OFF

| Role | Name | Date | Status |
|------|------|------|--------|
| Auditor | cosca-workflow-chief | 2026-07-28 | ✅ Complete |
| Reviewed by | (pending) | — | ⬜ |
| Approved by | Cosca Kernel | — | ⬜ |

> **Next audit**: Scheduled after R1-R9 are implemented (estimated: 2 sprints).
>
> **Related**: [workflow/SKILL.md](../../engines/tools/SKILL.md) | [KERNEL.md](../../KERNEL.md) | [MEMORY_MODEL.md](../MEMORY_MODEL.md) | [learnings.md](../evolution/learnings.md)
