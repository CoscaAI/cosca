# Cosca Semantic Memory — Index v2.0.0

> **Cycle**: C2 (Second Indexing Cycle)  
> **Generated**: 2026-08-29  
> **Indexed By**: cosca-semantic-memory  
> **Files Analyzed**: 494  
> **Status**: active (replaces C1)

---

## Executive Summary

Second-cycle semantic re-index of the full memory tree (494 files, up from 426 in C1). Reconstructed the derived index from the canonical source; no source-knowledge files were modified. The ecosystem has densified dramatically since C1: what was a "sharply stratified" ecosystem (10/55 agents with substance) is now a broadly-active workforce (16 agents at L3+, 28 at L2, driven by the Onda 2 / Onda 3 activation waves on 2026-07-28). A substantive new per-domain artifact layer emerged (quality-gates, SLOs, performance baseline, technical-debt scorecard, audit/activation reports) that did not exist at C1. A new project direction — the **Cosca "Living World"** (generative mining, Unreal integration, VFX/audio/spatial/vision layers, ADR-011 Mega Brain, ADR-012 2-zone air-gap) — represents a major new semantic domain absent from C1.

### Key Metrics

| Metric | Value (C1) | Value (C2) |
|--------|-----------|-----------|
| Total files indexed | 426 | **494** |
| Agent directories | 55 | **54** |
| Agents with substantive evidence (≥L2) | 10 (18%) | **44 (81%)** |
| Agents at L3+ | 4 | **16** |
| Agents at L1 seed only | 41 (75%) | **10 (19%)** |
| Cross-agent clusters identified | 8 | **10** |
| Semantic topic groups | 24 | **27** |
| ADRs | 14 | **15** |
| Bug registry entries | 5 | **8** (5 fixed, 3 open) |
| Knowledge density (substantive) | ~15% | **~55%** |

---

## 1. Semantic Topic Map

Each topic maps to all relevant memory files, regardless of directory or agent. Search by meaning, not path.

### 1.1 Architecture & System Design

**Coverage**: HIGH (26 files). 5-layer model intact. New subsystem plans (gRPC, streaming, MCP tools) added since C1.

| Sub-topic | Files |
|-----------|-------|
| System architecture (5 layers) | `architecture/system-architecture-overview.md`, `codebase/overview.md`, `architecture/event-architecture.md`, `architecture/database-architecture.md` |
| ADRs (cross-cutting) | `architecture/adr/adr-0001-ai-architecture.md`, `architecture/adr/adr-1322-compliance.md`, `architecture/adr/adr-1430-plugin.md`, `architecture/adr/adr-1825-performance.md`, `architecture/adr/adr-2000-platform.md`, `architecture/adr/adr-2412-governance.md`, `architecture/adr/adr-2745-sdk.md`, `architecture/adr/adr-2958-provider.md`, `architecture/adr/adr-3159-cache.md`, `architecture/adr/adr-3428-messaging.md`, `architecture/adr/adr-3584-migration.md`, `architecture/adr/adr-3692-technical-debt.md`, `architecture/adr/adr-3929-cli.md`, `architecture/adr/adr-5567-discovery.md`, `architecture/adr/adr-7116-api.md` |
| New subsystem plans | `architecture/grpc-server-plan.md`, `architecture/streaming-plan.md`, `architecture/mcp-tools-plan.md` |
| Architecture patterns | `pattern/architecture-patterns.md`, `pattern/go-provider-pattern.md`, `pattern/go-editor-adapter-pattern.md`, `pattern/go-plugin-wasm-pattern.md`, `pattern/pattern-enterprise-gaps.md` |
| Agent capability | `agent/cosca-architecture/capability-profile.md`, `agent/cosca-architecture/learnings.md` |

**Agents involved**: cosca-architecture (L3), cosca-cto (L2), cosca-kernel (L3), cosca-documentation (L3), cosca-backend (L3)  
**Keywords**: `layered-monolith`, `single-binary`, `sqlite`, `interface-driven`, `editor-agnostic`, `provider-agnostic`, `plugin-extensible`, `5-layer-model`, `gRPC-3-services-12-RPCs`, `WebSocket/SSE-streaming`, `MCP-tools`, `ADR`, `design-patterns`

---

### 1.2 Database & Storage

**Coverage**: HIGH. "PostgreSQL fantasy" stayed corrected. FTS5 schema mismatch surfaced by perf baseline as a correctness bug (documents_fts).

| Sub-topic | Files |
|-----------|-------|
| SQLite stack reality | `agent/cosca-database/learnings.md`, `long/go-sqlite-stack.md`, `architecture/database-architecture.md` |
| Schema mapping | `agent/cosca-database/learnings.md`, `codebase/go-packages.md` |
| FTS5 & Vector search | `agent/cosca-database/learnings.md`, `agent/cosca-performance/learnings.md`, `performance/baseline-report.md` |
| Migration, WAL, tuning | `agent/cosca-database/learnings.md`, `agent/cosca-performance/learnings.md`, `agent/cosca-migration/learnings.md` |
| **FTS5 schema bug (documents_fts)** | `performance/baseline-report.md` §2.2, `agent/cosca-performance/learnings.md` |
| Agent capability | `agent/cosca-database/capability-profile.md`, `agent/cosca-specialist-database-sql/capability-profile.md`, `agent/cosca-migration/capability-profile.md` |

**Agents involved**: cosca-database (L2), cosca-performance (L3), cosca-specialist-database-sql (L1), cosca-migration (L2)  
**Keywords**: `sqlite`, `modernc.org/sqlite`, `WAL-mode`, `FTS5`, `sqlite-vec`, `BM25`, `vector-fusion`, `single-writer`, `embedded`, `schema-migration`, `EXPLAIN-QUERY-PLAN`, `documents_fts-content-column-mismatch`

---

### 1.3 API, Backend & Middleware

**Coverage**: HIGH. 36-endpoint REST surface intact. gRPC and streaming plans now added.

| Sub-topic | Files |
|-----------|-------|
| REST API surface | `agent/cosca-backend/learnings.md`, `pattern/api-patterns.md`, `architecture/adr/adr-7116-api.md` |
| Middleware chain | `agent/cosca-backend/learnings.md`, `agent/cosca-security/learnings.md` |
| Error handling patterns | `agent/cosca-backend/failures.md` (inconsistency failure) |
| Abstraction pitfalls | `agent/cosca-backend/failures.md` (CRUDHandler over-abstraction, caching failure) |
| Agent capability | `agent/cosca-backend/capability-profile.md` (L3), `agent/cosca-specialist-backend-api/capability-profile.md` (L2), `agent/cosca-specialist-backend-service/capability-profile.md` (L2) |

**Agents involved**: cosca-backend (L3), cosca-specialist-backend-api (L2), cosca-specialist-backend-service (L2), cosca-security (L3)  
**Keywords**: `REST`, `36-endpoints`, `10-domains`, `middleware-chain`, `SecurityHeaders→Auth→CSRF→RateLimit→CORS→Logging`, `RBAC`, `typed-contextKey`, `CRUD-coverage`, `gRPC-unary`, `SSE-sendSSE`

---

### 1.4 Auth, Security & Compliance

**Coverage**: HIGH. Auth audit complete (JWT/bcrypt/CSRF/rate-limit verified). Compliance self-assessment now exists (tied to security's 5 steps).

| Sub-topic | Files |
|-----------|-------|
| Auth architecture (3-tier) | `agent/cosca-security/learnings.md`, `agent/cosca-backend/learnings.md`, `agent/cosca-frontend/learnings.md` |
| JWT, bcrypt, CSRF, rate limiting | `agent/cosca-security/learnings.md` |
| Security headers (CSP, HSTS) | `agent/cosca-security/learnings.md` |
| Compliance (GDPR/SOC2 — aspirational) | `agent/cosca-security/learnings.md`, `compliance/gdpr-lgpd-assessment.md`, `long/compliance-framework.md`, `architecture/adr/adr-1322-compliance.md` |
| RBAC & middleware | `agent/cosca-security/capability-profile.md`, `agent/cosca-backend/capability-profile.md` |
| Bug references | `bug/bug-003-race-conditions.md`, `bug/bug-004-provider-caching.md` |
| Agent capability | `agent/cosca-security/capability-profile.md` (L3), `agent/cosca-compliance/capability-profile.md` (L2) |

**Agents involved**: cosca-security (L3), cosca-backend (L3), cosca-frontend (L3), cosca-compliance (L2), cosca-critic (L3)  
**Keywords**: `3-tier-auth`, `API-Key→Cookie-JWT→Bearer`, `HMAC-SHA256`, `bcrypt-cost-12`, `brute-force-lockout`, `double-submit-CSRF`, `ConstantTimeCompare`, `token-bucket-rate-limit`, `GDPR-20-controls`, `LGPD-15-controls`, `OWASP-Top-10`, `govulncheck`, `STRIDE`

---

### 1.5 Frontend & UI/UX

**Coverage**: MEDIUM-HIGH. Next.js 15 architecture intact. UX audit performed (uiux at L2).

| Sub-topic | Files |
|-----------|-------|
| Next.js 15 architecture | `agent/cosca-frontend/learnings.md`, `long/nextjs-frontend-stack.md`, `codebase/web-frontend.md` |
| Feature-slice pattern | `agent/cosca-frontend/learnings.md`, `agent/cosca-frontend/capability-profile.md` |
| State management | `agent/cosca-frontend/learnings.md` (Context + TanStack Query) |
| Testing pyramid (frontend) | `agent/cosca-frontend/learnings.md`, `testing/strategy.md` |
| Agent capability | `agent/cosca-frontend/capability-profile.md` (L3), `agent/cosca-uiux/capability-profile.md` (L2), `agent/cosca-specialist-frontend-component/capability-profile.md` (L2) |

**Agents involved**: cosca-frontend (L3), cosca-uiux (L2), cosca-specialist-frontend-component (L2), cosca-product (L1)  
**Keywords**: `Next.js-15`, `App-Router`, `26-routes`, `27-feature-modules`, `14-Radix-primitives`, `shadcn/ui`, `TanStack-Query`, `staleTime-60s`, `httpOnly-cookies`, `sentinel-cookie`, `PWA`, `MSW-mocking`, `WCAG-2.1-AA`

---

### 1.6 Runtime, State Machine & Lifecycle

**Coverage**: HIGH. State machine (8 states/20 transitions) audited. Bugs 006/007 now formally registered (still open).

| Sub-topic | Files |
|-----------|-------|
| State machine (8 states, 20 transitions) | `agent/cosca-runtime/learnings.md`, `agent/cosca-runtime/capability-profile.md` |
| Daemon & watchdog | `agent/cosca-runtime/learnings.md` |
| Metrics system (12 real metrics) | `agent/cosca-runtime/learnings.md`, `agent/cosca-performance/learnings.md`, `agent/cosca-monitoring/learnings.md` |
| Known bugs (Restart, EventStartupComplete) | `bug/bug-006-restart-broken.md`, `bug/bug-007-startup-event-timing.md`, `bug/bug-008-metrics-misdocumented.md` |
| Agent capability | `agent/cosca-runtime/capability-profile.md` (L2) |

**Agents involved**: cosca-runtime (L2), cosca-performance (L3), cosca-monitoring (L2), cosca-backend (L3)  
**Keywords**: `state-machine`, `8-states-20-transitions`, `Restart-broken`, `EventStartupComplete-premature`, `daemon-watchdog`, `pid-file`, `signal-handling`, `atomic-counters`, `durationHistogram`, `sync.Map`, `hot-reload-missing`

---

### 1.7 Performance & Observability

**Coverage**: HIGH (this is the biggest jump vs C1; was "Low" at C1). First quantified performance baseline established 2026-07-28. SLOs defined.

| Sub-topic | Files |
|-----------|-------|
| Performance baseline (search/vector/hot-paths) | `performance/baseline-report.md`, `agent/cosca-performance/learnings.md` |
| SQLite performance characteristics | `agent/cosca-performance/learnings.md`, `agent/cosca-database/learnings.md` |
| Vector search bottleneck (brute-force O(n)) | `performance/baseline-report.md` §1.3 |
| SLOs / Prometheus metrics | `monitoring/slos.md`, `agent/cosca-monitoring/learnings.md` |
| Bug-005 root cause (FTS5 correctness) | `performance/baseline-report.md` §2.2, `bug/bug-005-sqlite-first-run.md` |
| Agent capability | `agent/cosca-performance/capability-profile.md` (L3), `agent/cosca-monitoring/capability-profile.md` (L2) |

**Agents involved**: cosca-performance (L3), cosca-monitoring (L2), cosca-database (L2), cosca-runtime (L2)  
**Keywords**: `vector-search-bruteforce`, `WAL-mode`, `single-writer`, `goroutine-leak`, `pprof`, `benchstat`, `EXPLAIN-QUERY-PLAN`, `go-benchmark`, `SLO-5`, `SLI`, `Prometheus-60-metrics`, `OpenTelemetry`, `FAISS/ANN-index`

---

### 1.8 Documentation & Knowledge Management

**Coverage**: HIGH. 887-asset knowledge base confirmed; doc-code validation gate (G6) defined.

| Sub-topic | Files |
|-----------|-------|
| Documentation audit (Phases 1-3) | `agent/cosca-documentation/learnings.md`, `agent/cosca-kernel/learnings.md` |
| Doc-code validator / G6 gate | `qa/quality-gates.md`, `agent/cosca-documentation/learnings.md`, `agent/cosca-qa/learnings.md` |
| Memory health (494 files) | `memory-chief/activation-report.md`, `agent/cosca-memory-chief/learnings.md`, `agent/cosca-documentation/learnings.md` |
| Agent capability | `agent/cosca-documentation/capability-profile.md` (L3), `agent/cosca-memory-chief/capability-profile.md` (L2), `agent/cosca-specialist-documentation-writer/capability-profile.md` (L2) |

**Agents involved**: cosca-documentation (L3), cosca-memory-chief (L2), cosca-specialist-documentation-writer (L2), cosca-kernel (L3)  
**Keywords**: `887-doc-assets`, `badge-drift`, `version-inconsistency`, `postgresql-fantasy`, `doc-code-validator`, `frontmatter-coverage`, `cross-reference-integrity`, `stale-content`, `number-verification`

---

### 1.9 Testing & Quality Assurance

**Coverage**: HIGH (was Medium at C1). QA Chief at L3. G0–G9 quality gates ratified. Integration test suite for runtime produced.

| Sub-topic | Files |
|-----------|-------|
| Quality gates G0–G9 | `qa/quality-gates.md`, `agent/cosca-qa/learnings.md` |
| Test strategy (pyramid) | `testing/strategy.md`, `testing/coverage.md`, `testing/patterns.md`, `testing/integration-report.md`, `testing/e2e-scenarios.md` |
| Go test patterns | `pattern/testing-patterns.md`, `testing/patterns.md` |
| Frontend testing | `agent/cosca-frontend/learnings.md` |
| Bug registry | `bug/bug-001-tmp-path.md` through `bug-008-metrics-misdocumented.md` |
| Specialist wave review | `review/onda-3-specialist-review.md` |
| Agent capability | `agent/cosca-qa/capability-profile.md` (L3), `agent/cosca-testing/capability-profile.md` (L3), `agent/cosca-specialist-testing-unit/capability-profile.md` (L1), `agent/cosca-specialist-testing-integration/capability-profile.md` (L2), `agent/cosca-specialist-testing-e2e/capability-profile.md` (L2) |

**Agents involved**: cosca-qa (L3), cosca-testing (L3), cosca-specialist-testing-unit (L1), cosca-specialist-testing-integration (L2), cosca-specialist-testing-e2e (L2)  
**Keywords**: `quality-gates-G0-G9`, `testing-pyramid`, `AAA-pattern`, `go-test`, `-race`, `Vitest`, `Playwright`, `Storybook`, `MSW`, `coverage-70%`, `flaky-tests`, `integration-suite`

---

### 1.10 Governance, Evolution & Metacognition

**Coverage**: HIGH (was Medium). Governance audit executed (98.1% DNA compliance). Critic/paradigm ran first reviews.

| Sub-topic | Files |
|-----------|-------|
| Constitution (7 principles) | `agent/cosca-kernel/learnings.md` |
| Confidence Model (6 levels) | `agent/cosca-kernel/learnings.md` |
| Curation Engine (5 rules) | `agent/cosca-kernel/learnings.md` |
| Agent DNA v3.0 (28 fields) | `agent/cosca-kernel/learnings.md`, `agent/cosca-kernel/evolution.md` |
| Governance compliance audit | `governance/audit-report.md`, `agent/cosca-governance/learnings.md` |
| Critic review (Onda 2) | `critic/onda-2-review.md`, `agent/cosca-critic/learnings.md` |
| Framework decisions | `decision/decision-enterprise-evolution-2026-07-23.md`, `decision/decision-enterprise-prompt-2026-07-12.md`, `decision/decision-audit-2026-07-12.md` |
| Agent capability | `agent/cosca-kernel/capability-profile.md` (L3), `agent/cosca-critic/capability-profile.md` (L3), `agent/cosca-evolution/capability-profile.md` (L2), `agent/cosca-governance/capability-profile.md` (L1), `agent/cosca-paradigm/capability-profile.md` (L1) |

**Agents involved**: cosca-kernel (L3), cosca-critic (L3), cosca-governance (L1), cosca-evolution (L2), cosca-paradigm (L1)  
**Keywords**: `constitution`, `confidence-model`, `curation-engine`, `metacognition-pipeline`, `DNA-v3.0`, `28-fields`, `quality-gates-G0-G9`, `evidence-weights`, `curation-score`, `paradigm-shift`, `decision-critique`, `99.1%-compliance`

---

### 1.11 Platform, DevOps & Infrastructure

**Coverage**: HIGH (was Low-Medium at C1). DevOps pipeline + CI/CD defined, audit reports for platform/infra/cache.

| Sub-topic | Files |
|-----------|-------|
| CI/CD pipeline + G0-G6 automation | `agent/cosca-devops/learnings.md`, `qa/quality-gates.md` |
| Infrastructure audit | `agent/cosca-infrastructure/audit-report-2026-07-28.md`, `agent/cosca-infrastructure/learnings.md` |
| Platform audit | `agent/cosca-platform/audit-report-2026-07-28.md`, `agent/cosca-platform/learnings.md` |
| Cache audit | `cache/audit-report.md`, `agent/cosca-cache/learnings.md` |
| ADR (platform, cache) | `architecture/adr/adr-2000-platform.md`, `architecture/adr/adr-3159-cache.md` |
| Agent capability | `agent/cosca-devops/capability-profile.md` (L3), `agent/cosca-platform/capability-profile.md` (L2), `agent/cosca-infrastructure/capability-profile.md` (L2), `agent/cosca-cache/capability-profile.md` (L3) |

**Agents involved**: cosca-devops (L3), cosca-platform (L2), cosca-infrastructure (L2), cosca-cache (L3)  
**Keywords**: `CI/CD`, `Helm`, `Kubernetes`, `Docker`, `IaC`, `Terraform`, `blue-green-deploy`, `immutable-infrastructure`, `secrets-management`, `redis`, `CDN`, `multi-tier-cache`

---

### 1.12 Provider, AI & ML

**Coverage**: MEDIUM (was Low). Provider at L2 (audit done). AI at L2 (metacognition/deliberation mining). New "Living World" mining domain.

| Sub-topic | Files |
|-----------|-------|
| LLM providers (11) | `architecture/system-architecture-overview.md`, `pattern/go-provider-pattern.md`, `architecture/adr/adr-2958-provider.md`, `agent/cosca-provider/learnings.md` |
| RAG & embeddings | `agent/cosca-ai/learnings.md`, `agent/cosca-provider/learnings.md` |
| AI deliberation / orchestration mining | `agent/cosca-ai/learnings.md`, `agent/cosca-architecture/learnings.md` (ADR-011 Mega Brain) |
| Agent capability | `agent/cosca-provider/capability-profile.md` (L2), `agent/cosca-ai/capability-profile.md` (L2) |

**Agents involved**: cosca-provider (L2), cosca-ai (L2), cosca-architecture (L3), cosca-kernel (L3)  
**Keywords**: `OpenAI`, `Anthropic`, `Ollama`, `LLM-providers`, `RAG`, `embeddings`, `prompt-engineering`, `injection-attacks`, `provider-failover`, `cost-optimization`, `deliberation`, `convergence-calculated`

---

### 1.13 SDKs, CLI & Developer Experience

**Coverage**: MEDIUM. TypeScript SDK at L2. CLI at L1 (maxLv 1). Audit reports present.

| Sub-topic | Files |
|-----------|-------|
| TypeScript SDK | `agent/cosca-sdk/learnings.md`, `sdk/audit-report.md`, `architecture/adr/adr-2745-sdk.md` |
| CLI design | `agent/cosca-cli/capability-profile.md`, `cli/audit-report.md`, `architecture/adr/adr-3929-cli.md`, `decisions/cosca-cli/adr-001.md` through `adr-007.md` |
| Project overview | `project/cosca-cli-overview.md` |
| Agent capability | `agent/cosca-sdk/capability-profile.md` (L2), `agent/cosca-cli/capability-profile.md` (L1) |

**Agents involved**: cosca-sdk (L2), cosca-cli (L1)  
**Keywords**: `TypeScript-SDK`, `@cosca/sdk`, `Vitest`, `Cobra-CLI`, `POSIX`, `shell-completion`, `code-generators`, `NPM-publication`

---

### 1.14 Plugins & WASM

**Coverage**: MEDIUM (was Low). Plugin audit performed.

| Sub-topic | Files |
|-----------|-------|
| Plugin pattern (Go/WASM/External) | `pattern/go-plugin-wasm-pattern.md`, `architecture/adr/adr-1430-plugin.md`, `plugin/audit-report.md` |
| Agent capability | `agent/cosca-plugin/capability-profile.md` (L2) |

**Agents involved**: cosca-plugin (L2)  
**Keywords**: `WASM`, `wazero`, `plugin-runtime`, `sandboxing`, `hot-reload`, `SDK-contracts`, `plugin-registry`

---

### 1.15 Bootstrap, Discovery & Context

**Coverage**: MEDIUM. Bootstrap optimized. Discovery engine used. Context at L1.

| Sub-topic | Files |
|-----------|-------|
| Bootstrap optimization | `agent/cosca-automation/learnings.md`, `agent/cosca-kernel/learnings.md` |
| Codebase discovery | `agent/cosca-discovery/capability-profile.md`, `agent/cosca-kernel/learnings.md` |
| Session context | `context/session.md`, `context/cognitive-state.md` |
| Agent capability | `agent/cosca-bootstrap/capability-profile.md` (L1), `agent/cosca-discovery/capability-profile.md` (L1), `agent/cosca-context/capability-profile.md` (L1) |

**Agents involved**: cosca-bootstrap (L1), cosca-discovery (L1), cosca-context (L1), cosca-automation (L3)  
**Keywords**: `bootstrap`, `startup-optimization`, `cognitive-state`, `fast-path`, `Phase-0`, `codebase-scan`, `glob-grep`, `stack-detection`, `module-boundaries`

---

### 1.16 Sessions, Evolution & Roadmap

| Sub-topic | Files |
|-----------|-------|
| Active sessions | `sessions/active/current.md`, `session/INDEX.md` |
| Session archives | `sessions/archive/`, `short/session-*.md` |
| Evolution tracking | `evolution/learnings.md`, `agent/*/evolution.md` |
| Roadmap & milestones | `roadmap/INDEX.md`, `roadmap/milestones.md`, `roadmap/platform-evolution-v1.4.0.md`, `roadmap/onda-2-plan.md` |
| Risk registry | `risk/RISK_REGISTRY.md` |

**Keywords**: `sessions`, `evolution-timeline`, `confidence-trajectory`, `milestones`, `risk-registry`, `auto-evolution`

---

### 1.17 Living World & Generative AI (NEW domain since C1)

**Coverage**: EMERGING. New project direction revealed 2026-08-23 — Cosca as agent living in an Unreal world. Born from professor-guided mining.

| Sub-topic | Files |
|-----------|-------|
| World-model mining (10 layers) | `agent/cosca-kernel/learnings.md` (Sessions 2026-08-23), `agent/cosca-architecture/learnings.md` |
| Unreal integration | `agent/cosca-kernel/learnings.md` (Unreal UE5 mining, ACoscaAgentPawn) |
| Generative media stack | `agent/cosca-kernel/learnings.md` (Sceelix, Gaussian Splatting, ComfyUI, AudioCraft) |
| Deliberation & plan-only (ADR-011) | `agent/cosca-architecture/learnings.md`, `agent/cosca-kernel/evolution.md` |
| 2-zone air-gap architecture (ADR-012) | `agent/cosca-architecture/learnings.md`, `agent/cosca-security/learnings.md`, `agent/cosca-infrastructure/learnings.md` |
| Project goal | `project/architecture-initiative.md`, `project/current-projects.md` |

**Agents involved**: cosca-kernel (L3), cosca-architecture (L3), cosca-security (L3), cosca-infrastructure (L2), cosca-ai (L2)  
**Keywords**: `living-world`, `unreal-UE5`, `PCG`, `vision/spatial/VFX/audio layers`, `Sceelix`, `Gaussian-Splatting`, `ComfyUI`, `ACoscaAgentPawn`, `plan-only`, `boardroom-2-zone`, `air-gap`, `ADR-011`, `ADR-012`

---

## 2. Cross-Agent Knowledge Clusters

Clusters where knowledge from ≥3 agents intersects on the same domain concern. Regenerated from C2 content.

### Cluster A: 🔐 Auth & Security Stack
**Density**: HIGH (5 agents, 13+ files, real audit data)

**Agents**: cosca-security (L3), cosca-backend (L3), cosca-frontend (L3), cosca-runtime (L2), cosca-compliance (L2)

**Shared knowledge**:
- 3-tier auth chain (API Key → Cookie JWT → Bearer) — audited by security, mapped by backend, consumed by frontend
- CSRF double-submit pattern — implemented in backend middleware, auto-injected by frontend fetch wrapper, audited by security
- RBAC (admin/editor/viewer) — enforced in backend middleware, reflected in frontend route guards, verified by security audit
- Rate limiting — token bucket in backend, per-IP and per-endpoint, documented by security
- GDPR/LGPD controls — security 5-step framework, compliance 20-control GDPR / 15-control LGPD mapping

**Core files**: `agent/cosca-security/learnings.md`, `agent/cosca-backend/learnings.md`, `agent/cosca-frontend/learnings.md`, `compliance/gdpr-lgpd-assessment.md`

**Keywords**: `jwt`, `hs256`, `bcrypt`, `csrf`, `httpOnly-cookies`, `rate-limiting`, `rbac`, `middleware-chain`, `sentinel-cookie`, `double-submit`, `gdpr-20-controls`

---

### Cluster B: 📦 Data & Storage Reality
**Density**: HIGH (4 agents, 10+ files, corrected fantasy + FTS5 bug)

**Agents**: cosca-database (L2), cosca-backend (L3), cosca-performance (L3), cosca-documentation (L3)

**Shared knowledge**:
- SQLite embedded reality (NOT PostgreSQL RDS) — discovered by database, verified by documentation, consumed by backend and performance
- FTS5 + sqlite-vec search pipeline — mapped by database, performance characteristics by performance baseline
- Schema: agents, memory, knowledge, providers, skills, workflows, plugins, sessions, config
- WAL mode: concurrent reads, single writer — known bottleneck
- FTS5 `documents_fts` content-column mismatch — surfaced by perf baseline, a correctness bug

**Core files**: `agent/cosca-database/learnings.md`, `agent/cosca-performance/learnings.md`, `performance/baseline-report.md`, `agent/cosca-documentation/learnings.md`

**Keywords**: `sqlite`, `fts5`, `sqlite-vec`, `wal-mode`, `embedded-database`, `no-network-latency`, `single-writer`, `postgresql-fantasy`, `documents_fts-content-column-mismatch`

---

### Cluster C: ⚡ Runtime Lifecycle & Observability
**Density**: HIGH (4 agents, real metrics + bugs formalized)

**Agents**: cosca-runtime (L2), cosca-performance (L3), cosca-monitoring (L2), cosca-backend (L3)

**Shared knowledge**:
- 8-state machine with 20 transitions — mapped by runtime, consumed by all agents
- 3 critical bugs now formalized as bug-006/007/008 (Restart broken, EventStartupComplete premature, metrics misdocumented) — still open
- Daemon watchdog: auto-restart, stale PID detection, sync loop
- Metrics: 8 atomic counters + 4 duration histograms + 60+ Prometheus metrics via /metrics (14121)

**Core files**: `agent/cosca-runtime/learnings.md`, `agent/cosca-monitoring/learnings.md`, `agent/cosca-performance/learnings.md`, `monitoring/slos.md`

**Keywords**: `state-machine`, `lifecycle`, `daemon`, `watchdog`, `metrics`, `atomic-counters`, `signal-handling`, `graceful-shutdown`, `Restart-broken`, `Prometheus-60-metrics`, `SLO`

---

### Cluster D: 📋 Documentation Integrity & Knowledge Management
**Density**: HIGH (4 agents, 887 docs audited)

**Agents**: cosca-documentation (L3), cosca-kernel (L3), cosca-qa (L3), cosca-memory-chief (L2)

**Shared knowledge**:
- Multi-source audit pattern (README claims vs codebase scan vs memory)
- Doc-code validator (G6 gate) defined in quality-gates, enforced by QA
- Version drift: docs/README.md vs CHANGELOG
- Memory health: 494 files currently indexed (up from 309 at C1)

**Core files**: `agent/cosca-documentation/learnings.md`, `agent/cosca-kernel/learnings.md`, `qa/quality-gates.md`, `agent/cosca-qa/learnings.md`

**Keywords**: `cross-source-audit`, `badge-drift`, `version-inconsistency`, `number-verification`, `frontmatter-compliance`, `stale-content`, `doc-code-validator`, `aspirational-drift`

---

### Cluster E: 🎨 Frontend Architecture & State
**Density**: MEDIUM (3 agents, 5+ files)

**Agents**: cosca-frontend (L3), cosca-uiux (L2), cosca-specialist-frontend-component (L2)

**Shared knowledge**:
- Next.js 15 App Router, 26 routes, 27 feature modules
- Feature-slice pattern: components/hooks/types per feature, barrel exports
- State: React Context + TanStack Query, NO Zustand/Redux
- Auth: httpOnly cookies, sentinel cookie pattern
- Testing: Vitest, Playwright, Storybook, MSW

**Core files**: `agent/cosca-frontend/learnings.md`, `agent/cosca-frontend/capability-profile.md`, `agent/cosca-uiux/activation-report.md`

**Keywords**: `Next.js-15`, `feature-slice`, `Radix-UI`, `TanStack-Query`, `MSW`, `PWA`, `httpOnly-cookies`

---

### Cluster F: 🧠 Metacognition & Framework Evolution
**Density**: HIGH (5 agents, executed reviews now)

**Agents**: cosca-kernel (L3), cosca-critic (L3), cosca-paradigm (L1), cosca-evolution (L2), cosca-governance (L1)

**Shared knowledge**:
- Constitution: 7 immutable principles, chain of command, conflict resolution
- Confidence Model: 6 evidence levels, 7 modifiers, 0.30 threshold
- Curation Engine: 5 rules, CurationScore, auto-cycle
- Agent DNA v3.0: 28 fields, metacognition pipeline (8 stages)
- Critic framework: now executed — 5-question adversarial challenge applied to Onda 2 plan
- Governance audit: 98.1% DNA compliance verified (2026-07-28)

**Core files**: `agent/cosca-kernel/learnings.md`, `agent/cosca-critic/learnings.md`, `critic/onda-2-review.md`, `governance/audit-report.md`, `agent/cosca-paradigm/learnings.md`

**Keywords**: `constitution`, `confidence-model`, `curation-engine`, `metacognition`, `DNA-v3.0`, `quality-gates`, `evidence-weighted`, `paradigm-shift`, `adversarial-critique`

---

### Cluster G: 🚀 Delivery Pipeline & Infrastructure
**Density**: MEDIUM (4 agents, audit reports present, up from LOW at C1)

**Agents**: cosca-devops (L3), cosca-infrastructure (L2), cosca-platform (L2), cosca-cache (L3)

**Shared knowledge**:
- CI/CD pipeline (G0-G6 automation) defined by devops
- Infrastructure/Platform/Cache audit reports created 2026-07-28
- IaC principles, secrets management
- Cache architecture (multi-tier)

**Core files**: `agent/cosca-devops/learnings.md`, `agent/cosca-infrastructure/audit-report-2026-07-28.md`, `agent/cosca-platform/audit-report-2026-07-28.md`, `cache/audit-report.md`, `qa/quality-gates.md`

**Keywords**: `CI/CD`, `Kubernetes`, `Docker`, `IaC`, `HPA`, `NetworkPolicy`, `probes`, `auto-scaling`, `G0-G6-gates`, `audit-report`

---

### Cluster H: 🔌 Extensibility (SDK + CLI + Plugins)
**Density**: MEDIUM (3 agents, audit reports present)

**Agents**: cosca-sdk (L2), cosca-cli (L1), cosca-plugin (L2)

**Shared knowledge**:
- TypeScript SDK: test suite, audit report
- CLI: Cobra-based, audit report
- Plugins: Go/WASM/External, wazero runtime, sandboxing, audit report
- Workflow templates

**Core files**: `agent/cosca-sdk/learnings.md`, `sdk/audit-report.md`, `cli/audit-report.md`, `plugin/audit-report.md`, `pattern/go-plugin-wasm-pattern.md`

**Keywords**: `TypeScript-SDK`, `Cobra-CLI`, `WASM-wazero`, `workflow-templates`, `plugin-sandbox`, `POSIX-conventions`

---

### Cluster I: 🏭 Quality & Test Engineering (NEW since C1)
**Density**: MEDIUM (3 agents, solidified at L3)

**Agents**: cosca-qa (L3), cosca-testing (L3), cosca-performance (L3)

**Shared knowledge**:
- Quality gates G0–G9 defined/ratified and applied to Onda 2 sign-off
- Integration test suite for runtime state machine produced by testing
- Performance baseline (G7) established by performance, consumed as reference by QA and monitoring
- Bug severity classification (QA audit of 8 bugs)

**Core files**: `qa/quality-gates.md`, `agent/cosca-qa/learnings.md`, `agent/cosca-testing/learnings.md`, `agent/cosca-performance/learnings.md`, `testing/integration-report.md`

**Keywords**: `quality-gates-G0-G9`, `AAA-pattern`, `-race`, `coverage-70%`, `baseline-G7`, `integration-suite`, `severity-classification`

---

### Cluster J: 🔮 Living World & Generative AI (NEW since C1)
**Density**: EMERGING (4 agents, born from professor-guided mining)

**Agents**: cosca-kernel (L3), cosca-architecture (L3), cosca-security (L3), cosca-infrastructure (L2)

**Shared knowledge**:
- Professor's 10-layer world mining map (procedural, vision, spatial AI, VFX, audio, destruction, simulation)
- Unreal UE5 runtime (ACoscaAgentPawn, GAS, Behavior Tree, WebSocket bridge to Cosca port 14120)
- Generative media stack (Sceelix PCG, Gaussian Splatting, ComfyUI, AudioCraft)
- ADR-011 (Mega Brain deliberative/plan-only) + ADR-012 (2-zone air-gap vault architecture)

**Core files**: `agent/cosca-kernel/learnings.md`, `agent/cosca-architecture/learnings.md`, `agent/cosca-security/learnings.md`, `agent/cosca-infrastructure/learnings.md`, `architecture/adr/adr-0001-ai-architecture.md`

**Keywords**: `living-world`, `unreal-UE5`, `PCG`, `Sceelix`, `Gaussian-Splatting`, `ComfyUI`, `plan-only`, `2-zone-air-gap`, `ADR-011`, `ADR-012`

---

## 3. Agent Capability Matrix

Capability map for all 54 agents. Level (L) is the highest **Level** recorded in an agent's learnings.md (evidence of real task execution). Confidence (C) is the per-domain figure in capability-profile.md. Capability-profile `Current Level` may lag behind the learnings-derived level for agents activated in the Onda 2/3 waves; where the two disagree, the learnings evidence is authoritative for execution level.

### Level 3+ (Proven — real tasks, verified outcomes) — 16 agents

| Agent | L | Conf | Primary Domain | Evidence |
|-------|---|------|----------------|----------|
| cosca-kernel | 5 | 0.95 | Agent orchestration, framework design, Living World mining | `agent/cosca-kernel/` (40 learning entries) |
| cosca-architecture | 3 | 0.85 | System design, ADRs, gRPC/streaming planning | `agent/cosca-architecture/` (13 entries) |
| cosca-backend | 3 | 0.92 | REST API architecture, middleware, auth | `agent/cosca-backend/` (14 entries, failures) |
| cosca-security | 3 | 0.90 | Auth audit, OWASP, compliance, threat model | `agent/cosca-security/` (13 entries) |
| cosca-documentation | 3 | 0.95 | Enterprise-scale doc audit, doc-code validator | `agent/cosca-documentation/` (3 entries) |
| cosca-qa | 3 | 0.80 | Quality gates G0–G9, severity classification | `agent/cosca-qa/` + `qa/quality-gates.md` |
| cosca-frontend | 3 | 0.90 | Next.js architecture, feature-slice | `agent/cosca-frontend/` (4 entries) |
| cosca-performance | 3 | 0.70 | Performance baseline, vector bottleneck, G7 | `agent/cosca-performance/` (9 entries) + `performance/baseline-report.md` |
| cosca-testing | 3 | 0.25* | Integration test suite, runtime state machine | `agent/cosca-testing/` (3 entries, 69-line failures) |
| cosca-review | 3 | 0.25* | Code review, Onda 2/3 review reports | `agent/cosca-review/` (6 entries) + `review/` |
| cosca-critic | 3 | 0.40 | Adversarial decision review, Onda 2 critique | `agent/cosca-critic/` (3 entries) + `critic/onda-2-review.md` |
| cosca-integrations | 3 | 0.75 | Integrations audit | `agent/cosca-integrations/` (4 entries) + `integrations/audit-report.md` |
| cosca-devops | 3 | 0.25* | CI/CD pipeline, G0–G6 automation | `agent/cosca-devops/` (4 entries) |
| cosca-cache | 3 | 0.25* | Cache infra audit, TTL patterns | `agent/cosca-cache/` (4 entries) + `cache/audit-report.md` |
| cosca-automation | 3 | 0.25* | Bootstrap optimization, scripting | `agent/cosca-automation/` (3 entries) |
| cosca-platform | 3 | 0.85 | Platform audit, extensibility | `agent/cosca-platform/` (8 entries) |

> *Confidence 0.25 = capability-profile still at seed baseline; this agent has real execution evidence in learnings (Onda activation wave) but its capability-profile was not re-scored. This is known version-drift; flagged in §6.

### Level 2 (Active — real tasks) — 28 agents

| Agent | L | Conf | Primary Domain |
|-------|---|------|----------------|
| cosca-runtime | 2 | 0.95 | State machine, lifecycle, daemon, metrics |
| cosca-database | 2 | 0.90 | SQLite reality, schema mapping |
| cosca-memory-chief | 2 | 0.85 | Memory health, activation report |
| cosca-monitoring | 2 | 0.72 | SLOs, Prometheus metrics, observability |
| cosca-ai | 2 | 0.65 | RAG, AI deliberation, metacognition |
| cosca-provider | 2 | 0.85 | LLM providers, cache leak fix co-owner |
| cosca-compliance | 2 | 0.55 | GDPR/LGPD self-assessment |
| cosca-uiux | 2 | 0.50 | UX audit |
| cosca-analytics | 2 | 0.30 | Metrics, analytics |
| cosca-sdk | 2 | 0.25* | TypeScript SDK, audit report |
| cosca-integrations (see L3 above) | — | — | — |
| cosca-infrastructure | 2 | 0.35 | Infrastructure audit |
| cosca-messaging | 2 | 0.82 | Event bus architecture |
| cosca-workflow-chief | 2 | 0.85 | Workflow templates/audit |
| cosca-technical-debt | 2 | 0.25* | Tech-debt scorecard (1,375) |
| cosca-cto | 2 | 0.25* | CTO activation report |
| cosca-ceo | 2 | 0.25* | CEO activation report |
| cosca-mobile | 2 | 0.50 | Mobile activation analysis |
| cosca-release | 2 | 0.75 | Release activation report |
| cosca-evolution | 2 | 0.25* | Evolution/self-improvement |
| cosca-migration | 2 | 0.25* | Migration audit |
| cosca-specialist-backend-api | 2 | 0.25* | Backend API specialist |
| cosca-specialist-backend-service | 2 | 0.25* | Backend service specialist |
| cosca-specialist-documentation-writer | 2 | 0.25* | Doc writer specialist |
| cosca-specialist-frontend-component | 2 | 0.25* | Frontend component specialist |
| cosca-specialist-review-code | 2 | 0.25* | Code reviewer specialist |
| cosca-specialist-testing-integration | 2 | 0.25* | Integration test specialist |
| cosca-specialist-testing-e2e | 2 | 0.25* | E2E test specialist |

### Level 1 — Seed Data Only (no substantive task execution) — 10 agents

| Agent | Conf | Notes |
|-------|------|-------|
| cosca-cli | 0.40 | 1/5 tasks to L2; CLI audit report exists |
| cosca-governance | 0.45 | Governance audit executed, transitioning to L2 |
| cosca-product | 0.25 | Product activation report exists but no execution level |
| cosca-context | 0.25 | Seed only |
| cosca-bootstrap | 0.25 | Seed only |
| cosca-discovery | 0.80 | Seed — used as engine in audits |
| cosca-specialist-database-sql | 0.25 | Seed only |
| cosca-specialist-testing-unit | 0.25 | Seed only |
| cosca-paradigm | 0.25 | Gated — requires 3 months confidence data |
| cosca-monitoring | — | Listed L2 above via SLO evidence (capability-profile shows L1) |

---

## 4. Knowledge Gaps (Density Analysis) — Re-evaluated for C2

### 4.1 Previously-Critical Gaps (P0) — Status Change since C1

| Gap | C1 Status | C2 Status | Evidence |
|-----|-----------|-----------|----------|
| **No automated security scanning** | Critical open | **PARTIALLY CLOSED** | G4 gate (govulncheck + gosec) defined in `qa/quality-gates.md`; enforcement pending |
| **No runtime integration tests** | Critical open | **PARTIALLY CLOSED** | Integration suite produced by `agent/cosca-testing/`; not all 20 transitions covered |
| **No performance profiling** | Critical open | **CLOSED** | `performance/baseline-report.md` establishes first quantified baseline (vector/bench/hot-paths) |
| **No observability (OpenTelemetry)** | Critical open | **PARTIALLY CLOSED** | Prometheus /metrics documented (60+ metrics), 5 SLOs defined; OTel still aspirational |
| **No CI/CD automation** | Critical open | **PARTIALLY CLOSED** | G0–G6 gates defined; pipeline implementation pending |
| **Bug-005 uninvestigated** | Critical open | **CLOSED** | Root-caused in `performance/baseline-report.md` §2.2 (FTS5 schema, not perf) |
| **Restart() broken** | Critical open | **REGISTERED, still open** | `bug/bug-006-restart-broken.md` (formalized, unfixed) |
| **EventStartupComplete premature** | Critical open | **REGISTERED, still open** | `bug/bug-007-startup-event-timing.md` (formalized, unfixed) |
| **Metrics misdocumented** | (known in runtime) | **REGISTERED, still open** | `bug/bug-008-metrics-misdocumented.md` (added to registry) |

### 4.2 Remaining Critical Gaps (still open)

| Gap | Affected Agents | Impact |
|-----|-----------------|--------|
| **Restart() broken (bug-006)** | cosca-runtime | Blocker — daemon self-healing non-functional |
| **EventStartupComplete premature (bug-007)** | cosca-runtime | Critical — subscribers get false startup state |
| **Metrics misdocumented (bug-008)** | cosca-documentation, cosca-runtime | Major — docs claim 7 metrics, code has 12 |
| **Hot reload missing (BUG-U03)** | cosca-runtime, cosca-provider | Major — all 11 providers need daemon restart on reconfig |
| **FTS5 documents_fts content-column bug** | cosca-database, cosca-performance | Correctness — document-level search silently broken |
| **Vector search brute-force O(n)** | cosca-performance, cosca-database | Perf — 64ms @ 10K vectors, 46.8MB/query; no ANN |

### 4.3 Major Gaps (P1) — Status Change since C1

| Gap | C1 Status | C2 Status |
|-----|-----------|-----------|
| No accessibility audit (WCAG 2.1 AA) | Open | Open (UX audit executed, a11y still pending) |
| No visual regression testing | Open | Open |
| No feature flags | Open | Open |
| No STRIDE threat model | Open | Open (2-zone air-gap ADR-012 partially addresses) |
| No token revocation | Open | Open |
| No dependency graph generation | Open | Open |
| No indexed memory retrieval | Open | Open (semantic engine deployed, retrieval still path-first) |

### 4.4 Structural Gaps — Status Change since C1

| Gap | C1 Status | C2 Status | Evidence |
|-----|-----------|-----------|----------|
| **Agent-to-agent knowledge transfer** | Open (10/55 had learnings) | **IMPROVED** | Now 44 agents with substantive evidence (L≥2); negative memory now in kernel/testing/backend |
| **Negative memory underutilization** | Open (3/55 had failures) | **IMPROVED** | Substantive failures now in cosca-kernel (127 ln), cosca-testing (69 ln), cosca-backend (50 ln) |
| **Pattern memory is thin** | Open (2/55 had patterns) | **IMPROVED** | Substantive patterns in cosca-kernel (66 ln), cosca-review (55 ln), cosca-ai (24 ln), cosca-mobile (24 ln), cosca-infrastructure (23 ln), cosca-cli (20 ln), cosca-uiux (18 ln), cosca-memory-chief (16 ln) |
| **Evolution tracking incomplete** | Open (15/55) | **IMPROVED** | All 54 agents now have evolution.md (though many still seed-level) |
| **Leadership agents unexercised** | Open | **IMPROVED** | ceil/cto/product activation reports exist (L2 evidence) |
| **Specialist agents never used** | Open | **IMPROVED** | 7 specialists at L2 (Onda 3) — delegation pyramid tested |
| **Version drift between capability-profiles and learnings** | Not identified at C1 | **NEW STRUCTURAL GAP** | Many agents (qa, testing, devops, cache, automation, etc.) show L3 evidence in learnings but capability-profile still at seed 0.25 — profile re-scoring backlog |

---

## 5. Metrics Summary

### Coverage by Memory Type (C2)

| Memory Type | Files | Substantive Content | Density |
|-------------|-------|---------------------|---------|
| Agent memory | 339 | ~44 agents substantive (82%) | **HIGH (was LOW)** |
| Architecture | 24 | 24 files (100%) | HIGH |
| Pattern | 8 | 8 files (100%) | HIGH |
| Bug | 8 | 8 files (100%) | HIGH |
| Per-domain audit/report layer | ~22 | 22 files (100%) | **HIGH (NEW)** |
| Long/Knowledge | 7 | 5 files (71%) | MEDIUM |
| Codebase | 4 | 4 files (100%) | HIGH |
| Session/Short | 11 | 11 files (100%) | HIGH |
| Decision/Decisions | 12 | 12 files (100%) | HIGH |
| Testing | 6 | 6 files (100%) | HIGH |
| Roadmap | 4 | 4 files (100%) | HIGH |
| Dependencies | 3 | 3 files (100%) | HIGH |
| Context | 3 | 3 files (100%) | HIGH |
| Risk | 1 | 1 file (100%) | HIGH |
| Project | 5 | 5 files (100%) | HIGH |
| Evolution | 3 | 3 files (100%) | HIGH |

**Key insight (C2)**: The agent-memory density gap — C1's primary concern (12% density) — has been substantially resolved. From 10/55 (18%) to 44/54 (81%) agents with real execution evidence. The new structural concern is **version drift**: execution levels have outrun capability-profile re-scoring for ~16 agents, creating a risk where the capability matrix under-reports actual capability.

### Topic Coverage Heatmap (C2)

| Topic | Files | Depth | Freshness | Gaps |
|-------|-------|-------|-----------|------|
| Architecture | 26+ | Deep | Fresh (Aug) | None (ADR-011/012 emerging) |
| Documentation | 12+ | Deep | Fresh (Jul 28) | Doc-code validator enforcement pending |
| Auth/Security | 14+ | Deep | Fresh (Jul 28) | No STRIDE, token revocation; G4 enforcement pending |
| Database | 10+ | Deep | Fresh (Jul 28) | FTS5 schema bug, no ANN |
| Runtime | 8+ | Deep | Fresh (Jul 28) | 3 bugs open, no integration coverage for all transitions |
| Frontend | 7+ | Medium | Fresh (Jul 28) | No a11y, visual regression, feature flags |
| Performance | 8+ | Deep (quantified) | Fresh (Jul 28) | Vector brute-force, no ANN |
| Observability | 6+ | Deep (SLOs) | Fresh (Jul 28) | OTel aspirational |
| Governance | 8+ | Deep (executed) | Fresh (Jul 28) | Paradigm gated; profile drift |
| SDK/CLI/Plugins | 12+ | Medium | Fresh (Jul 28) | CLI at L1 |
| DevOps/Infra | 9+ | Medium (audits) | Fresh (Jul 28) | CI execution pending |
| Testing/QA | 12+ | Deep (gates) | Fresh (Jul 28) | E2E zero; coverage gaps remain |
| AI/ML | 3+ | Medium (deliberation) | Fresh (Aug) | RAG implementation detail |
| Mobile | 3+ | Shallow | Fresh | Activation analysis only |
| **Living World / Generative** | **EMERGING** | New | Fresh (Aug 23) | Whole new domain; not yet artifacts beyond kernel/architecture learnings |

---

## 6. Version-Drift Note (Structural)

A systematic finding of C2: the Onda 2 / Onda 3 activation waves (2026-07-28) gave ~16 agents real execution evidence (Level 3 in learnings), but their **capability-profiles were not re-scored** and still show the seed baseline (L1, conf 0.25). Agents affected include: cosca-qa, cosca-testing, cosca-review, cosca-critic, cosca-integrations, cosca-devops, cosca-cache, cosca-automation, cosca-platform, cosca-monitoring, cosca-semantic-memory, and most specialists. This is a maintenance target for C3, not a knowledge discrepancy.

---

## 7. Recommendations for Cycle 3

### Priority 1: Close the 6 remaining open bugs (highest ROI)
- Fix bug-006 (Restart() broken) — 4h, unblocks daemon self-healing
- Fix bug-007 (EventStartupComplete premature) — 6h
- Fix bug-008 (metrics misdocumented) — sync docs to 12 real metrics
- Fix FTS5 `documents_fts` content-column mismatch — restores document-level search
- Implement ANN index for vector search (16h, 100× speedup)
- Implement hot reload for provider/plugin reconfig

### Priority 2: Sync capability-profiles with execution evidence
- Re-score the ~16 agents whose learnings show L3 but profiles show seed (0.25 → real confidence)
- This aligns the capability matrix with the actual evolved ecosystem

### Priority 3: Densify emerging domains
- Capture Living World / generative (Unreal, vision, spatial AI, VFX, audio, destruction, simulation) as first-class semantic topics once the ADR-011/012 direction is codified
- Record ADR-011 (deliberation/plan-only) and ADR-012 (2-zone air-gap) as architecture entries in the index

### Priority 4: Complete the structural gates
- Enforce G0-G4 (build/lint/vet/test/security) in actual CI runs (currently defined, execution pending)
- Enforce G6 doc-code validator in CI

### Measurable Targets for Cycle 3

| Metric | C1 | C2 (now) | Target C3 |
|--------|-----|----------|-----------|
| Total files indexed | 426 | 494 | 500+ |
| Agents L3+ | 4 | 16 | 20+ |
| Agents with real execution | 10 | 44 | 50 |
| Critical bugs open | 3+ | 3 (006/007/008) | 0 |
| Capability-profile drift | — | ~16 agents | 0 |
| Security automation | 0 | G4 defined | enforced in CI |
| CI gates enforced | 0 | 0 | G0-G6 enforced |
| Living World topics indexed | 0 | 1 | 4+ |

---

## 8. Index Maintenance

This index is a living derived document. It is regenerated from the canonical source (the `.md` files); source files are never modified during re-indexing. Process:
1. Re-scan all memory files (inventory + count)
2. Read substantive learnings/capability-profiles/patterns/failures/evolution
3. Regenerate topic map, clusters, capability matrix, gaps
4. Version: C2 → INDEX-v2.md snapshot, create new INDEX.md for C3

**Scheduled refresh**: After every agent evolution cycle, or every 30 days, whichever comes first.

---

> **Generated by**: cosca-semantic-memory (Semantic Memory Chief)  
> **Based on**: 494 files across 27 topic groups, 54 agents  
> **Cycle**: C2 — Re-index; canonical source unchanged, derived index regenerated  
> **Next cycle**: C3 — Bug-fix wave + capability-profile sync + Living World codification
