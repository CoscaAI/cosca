# Cosca Semantic Memory — Index v1.0.0

> **Cycle**: C1 (First Indexing Cycle)  
> **Generated**: 2026-07-28  
> **Indexed By**: cosca-semantic-memory  
> **Files Analyzed**: 426  
> **Status**: active

---

## Executive Summary

First-cycle semantic indexing of all 426 memory files across 19 categories and 55 agents. The index maps knowledge by meaning (topic → related files) rather than by path hierarchy alone, enabling cross-agent semantic search. Analysis reveals a sharply stratified memory ecosystem: ~10 agents (18%) are Level 2-3 with substantive learnings, while ~41 agents (75%) are Level 1 seed data only. Four robust cross-agent knowledge clusters were identified. Major gaps exist in: runtime observability, security automation, testing coverage, mobile/plugin/SDK execution, and agent-to-agent knowledge transfer.

### Key Metrics

| Metric | Value |
|--------|-------|
| Total files indexed | 426 |
| Directories | 76 |
| Agent directories | 55 |
| Agents with substantive content (≥L2) | 10 (18%) |
| Agents at L1 seed only | 41 (75%) |
| Agents at L3 | 4 (cosca-kernel, cosca-backend, cosca-documentation, cosca-runtime) |
| Cross-agent clusters identified | 8 |
| Semantic topic groups | 24 |
| Gaps identified | 17 |
| Knowledge density (substantive files / total) | ~15% |

---

## 1. Semantic Topic Map

Each topic maps to all relevant memory files, regardless of directory or agent. Search by meaning, not path.

### 1.1 Architecture & System Design

**Coverage**: High (19 files). Solid architectural documentation with accurate 5-layer model.

| Sub-topic | Files |
|-----------|-------|
| System architecture (5 layers) | `architecture/system-architecture-overview.md`, `codebase/overview.md`, `architecture/event-architecture.md`, `architecture/database-architecture.md` |
| ADRs (cross-cutting) | `architecture/adr/adr-1825-performance.md`, `adr-1322-compliance.md`, `adr-3159-cache.md`, `adr-2412-governance.md`, `adr-3692-technical-debt.md`, `adr-1430-plugin.md`, `adr-3584-migration.md`, `adr-2958-provider.md`, `adr-2745-sdk.md`, `adr-5567-discovery.md`, `adr-2000-platform.md`, `adr-7116-api.md`, `adr-3929-cli.md`, `adr-3428-messaging.md` |
| Architecture patterns | `pattern/architecture-patterns.md`, `pattern/go-provider-pattern.md`, `pattern/go-editor-adapter-pattern.md`, `pattern/go-plugin-wasm-pattern.md` |
| Agent capability (Architecture Chief) | `agent/cosca-architecture/capability-profile.md` |

**Agents involved**: cosca-architecture, cosca-cto, cosca-kernel, cosca-documentation  
**Keywords**: `layered-monolith`, `single-binary`, `sqlite`, `interface-driven`, `editor-agnostic`, `provider-agnostic`, `plugin-extensible`, `5-layer-model`, `ADR`, `design-patterns`

---

### 1.2 Database & Storage

**Coverage**: High (corrected PostgreSQL→SQLite fantasy). Strong schema mapping, missing performance profiling.

| Sub-topic | Files |
|-----------|-------|
| SQLite stack reality | `agent/cosca-database/learnings.md` (PostgreSQL fantasy correction), `long/go-sqlite-stack.md`, `architecture/database-architecture.md` |
| Schema mapping | `agent/cosca-database/learnings.md` (schema inventory), `codebase/go-packages.md` |
| FTS5 & Vector search | `agent/cosca-database/learnings.md`, `agent/cosca-performance/learnings.md` |
| Migration, WAL, tuning | `agent/cosca-database/learnings.md`, `agent/cosca-performance/learnings.md` |
| Agent capability | `agent/cosca-database/capability-profile.md`, `agent/cosca-specialist-database-sql/capability-profile.md` |

**Agents involved**: cosca-database (L2), cosca-specialist-database-sql (L1), cosca-performance (L1), cosca-migration (L1)  
**Keywords**: `sqlite`, `modernc.org/sqlite`, `WAL-mode`, `FTS5`, `sqlite-vec`, `BM25`, `vector-fusion`, `single-writer`, `embedded`, `schema-migration`, `EXPLAIN-QUERY-PLAN`

---

### 1.3 API, Backend & Middleware

**Coverage**: High. Full API surface mapped (36 endpoints, 19 handlers). Negative memory active (3 failures recorded).

| Sub-topic | Files |
|-----------|-------|
| REST API surface | `agent/cosca-backend/learnings.md`, `pattern/api-patterns.md`, `architecture/adr/adr-7116-api.md` |
| Middleware chain | `agent/cosca-backend/learnings.md`, `agent/cosca-security/learnings.md` |
| Error handling patterns | `agent/cosca-backend/failures.md` (inconsistency failure) |
| Abstraction pitfalls | `agent/cosca-backend/failures.md` (CRUDHandler over-abstraction, caching failure) |
| Agent capability | `agent/cosca-backend/capability-profile.md` (L3, 0.59), `agent/cosca-specialist-backend-api/capability-profile.md` (L1), `agent/cosca-specialist-backend-service/capability-profile.md` (L1) |

**Agents involved**: cosca-backend (L3), cosca-specialist-backend-api (L1), cosca-specialist-backend-service (L1), cosca-security (L2), cosca-api-chief (L1)  
**Keywords**: `REST`, `36-endpoints`, `10-domains`, `middleware-chain`, `SecurityHeaders→Auth→CSRF→RateLimit→CORS→Logging`, `RBAC`, `admin/editor/viewer`, `typed-contextKey`, `CRUD-coverage`, `response-envelope-standardization`

---

### 1.4 Auth, Security & Compliance

**Coverage**: Medium-High. Strong auth audit complete. Compliance aspirational. Missing automated scanning, STRIDE, token revocation.

| Sub-topic | Files |
|-----------|-------|
| Auth architecture (3-tier) | `agent/cosca-security/learnings.md`, `agent/cosca-backend/learnings.md`, `agent/cosca-frontend/learnings.md` |
| JWT, bcrypt, CSRF, rate limiting | `agent/cosca-security/learnings.md` |
| Security headers (CSP, HSTS) | `agent/cosca-security/learnings.md` |
| Compliance (GDPR/SOC2 — aspirational) | `agent/cosca-security/learnings.md` (compliance fix), `long/compliance-framework.md`, `architecture/adr/adr-1322-compliance.md` |
| RBAC & middleware | `agent/cosca-security/capability-profile.md`, `agent/cosca-backend/capability-profile.md` |
| Bug references | `bug/bug-003-race-conditions.md`, `bug/bug-004-provider-caching.md` |
| Agent capability | `agent/cosca-security/capability-profile.md` (L2), `agent/cosca-compliance/capability-profile.md` (L1) |

**Agents involved**: cosca-security (L2), cosca-backend (L3), cosca-frontend (L2), cosca-compliance (L1), cosca-critic (L1)  
**Keywords**: `3-tier-auth`, `API-Key→Cookie-JWT→Bearer`, `HMAC-SHA256`, `bcrypt-cost-12`, `brute-force-lockout`, `double-submit-CSRF`, `ConstantTimeCompare`, `token-bucket-rate-limit`, `aspirational-GDPR`, `OWASP-Top-10`, `govulncheck`, `STRIDE`

---

### 1.5 Frontend & UI/UX

**Coverage**: Medium. Complete architecture extraction from 240+ TSX files. Missing a11y audit, visual regression, feature flags.

| Sub-topic | Files |
|-----------|-------|
| Next.js 15 architecture | `agent/cosca-frontend/learnings.md`, `long/nextjs-frontend-stack.md`, `codebase/web-frontend.md` |
| Feature-slice pattern | `agent/cosca-frontend/learnings.md`, `agent/cosca-frontend/capability-profile.md` |
| State management | `agent/cosca-frontend/learnings.md` (Context + TanStack Query, no Zustand/Redux) |
| Testing pyramid (frontend) | `agent/cosca-frontend/learnings.md`, `testing/strategy.md` |
| Agent capability | `agent/cosca-frontend/capability-profile.md` (L2), `agent/cosca-uiux/capability-profile.md` (L1), `agent/cosca-specialist-frontend-component/capability-profile.md` (L1) |

**Agents involved**: cosca-frontend (L2), cosca-uiux (L1), cosca-specialist-frontend-component (L1), cosca-product (L1)  
**Keywords**: `Next.js-15`, `App-Router`, `26-routes`, `27-feature-modules`, `14-Radix-primitives`, `shadcn/ui`, `TanStack-Query`, `staleTime-60s`, `httpOnly-cookies`, `sentinel-cookie`, `PWA`, `MSW-mocking`, `WCAG-2.1-AA`

---

### 1.6 Runtime, State Machine & Lifecycle

**Coverage**: Medium-High. Complete runtime audit done. 3 critical bugs found (not fixed). Missing integration tests, hot reload, OpenTelemetry.

| Sub-topic | Files |
|-----------|-------|
| State machine (8 states, 20 transitions) | `agent/cosca-runtime/learnings.md`, `agent/cosca-runtime/capability-profile.md` |
| Daemon & watchdog | `agent/cosca-runtime/learnings.md` |
| Metrics system (8 counters, 4 histograms) | `agent/cosca-runtime/learnings.md`, `agent/cosca-performance/learnings.md` |
| Known bugs (Restart, EventStartupComplete) | `agent/cosca-runtime/learnings.md` |
| Agent capability | `agent/cosca-runtime/capability-profile.md` (L3) |

**Agents involved**: cosca-runtime (L3), cosca-performance (L1), cosca-monitoring (L1)  
**Keywords**: `state-machine`, `8-states-20-transitions`, `Restart-broken`, `EventStartupComplete-premature`, `daemon-watchdog`, `pid-file`, `signal-handling`, `atomic-counters`, `durationHistogram`, `sync.Map`, `hot-reload-missing`

---

### 1.7 Performance & Observability

**Coverage**: Low. Static analysis only (L1). No actual profiling, benchmarking, EXPLAIN, or load testing done.

| Sub-topic | Files |
|-----------|-------|
| SQLite performance characteristics | `agent/cosca-performance/learnings.md`, `agent/cosca-database/learnings.md` |
| Go concurrency patterns | `agent/cosca-performance/learnings.md` |
| Bug-005 (slow dashboard query) | `bug/bug-005-sqlite-first-run.md`, `agent/cosca-performance/learnings.md` |
| ADR | `architecture/adr/adr-1825-performance.md` |
| Agent capability | `agent/cosca-performance/capability-profile.md` (L1), `agent/cosca-monitoring/capability-profile.md` (L1) |

**Agents involved**: cosca-performance (L1), cosca-database (L2), cosca-runtime (L3), cosca-monitoring (L1)  
**Keywords**: `WAL-mode`, `single-writer`, `goroutine-leak`, `pprof`, `benchstat`, `EXPLAIN-QUERY-PLAN`, `go-benchmark`, `SLO`, `SLI`, `OpenTelemetry`, `Prometheus`

---

### 1.8 Documentation & Knowledge Management

**Coverage**: High. 887-asset audit completed. 3 critically stale files found. Version drift detected.

| Sub-topic | Files |
|-----------|-------|
| Documentation audit (Phases 1-3) | `agent/cosca-documentation/learnings.md`, `agent/cosca-kernel/learnings.md` |
| New documentation creation | `agent/cosca-documentation/learnings.md` |
| Memory health (309 files) | `agent/cosca-memory-chief/capability-profile.md`, `agent/cosca-documentation/learnings.md` |
| Agent capability | `agent/cosca-documentation/capability-profile.md` (L3), `agent/cosca-memory-chief/capability-profile.md` (L1), `agent/cosca-specialist-documentation-writer/capability-profile.md` (L1) |

**Agents involved**: cosca-documentation (L3), cosca-memory-chief (L1), cosca-specialist-documentation-writer (L1), cosca-kernel (L3)  
**Keywords**: `887-doc-assets`, `badge-drift`, `version-inconsistency`, `postgresql-fantasy`, `go-sdk-fiction`, `compliance-fabrication`, `frontmatter-coverage`, `cross-reference-integrity`, `stale-content`, `number-verification`

---

### 1.9 Testing & Quality Assurance

**Coverage**: Medium. Strategy defined. Testing Chief and QA Chief at L1 (no execution). Specialist agents seeded.

| Sub-topic | Files |
|-----------|-------|
| Test strategy (pyramid) | `testing/strategy.md`, `testing/coverage.md`, `testing/patterns.md` |
| Go test patterns | `pattern/testing-patterns.md`, `testing/patterns.md` |
| Frontend testing (Vitest, Playwright, Storybook) | `agent/cosca-frontend/learnings.md` |
| Bug registry | `bug/bug-001-tmp-path.md` through `bug-005-sqlite-first-run.md` |
| Agent capability | `agent/cosca-qa/capability-profile.md` (L1), `agent/cosca-testing/capability-profile.md` (L1), `agent/cosca-specialist-testing-unit/capability-profile.md` (L1), `agent/cosca-specialist-testing-integration/capability-profile.md` (L1), `agent/cosca-specialist-testing-e2e/capability-profile.md` (L1) |

**Agents involved**: cosca-testing (L1), cosca-qa (L1), cosca-specialist-testing-unit (L1), cosca-specialist-testing-integration (L1), cosca-specialist-testing-e2e (L1)  
**Keywords**: `testing-pyramid`, `AAA-pattern`, `go-test`, `-race`, `Vitest`, `Playwright`, `Storybook`, `MSW`, `coverage`, `quality-gates`, `flaky-tests`

---

### 1.10 Governance, Evolution & Metacognition

**Coverage**: Medium. Framework artifacts exist (Constitution, DNA v3.0, Confidence Model, Curation Engine). Execution unverified.

| Sub-topic | Files |
|-----------|-------|
| Constitution (7 principles) | `agent/cosca-kernel/learnings.md` |
| Confidence Model (6 levels) | `agent/cosca-kernel/learnings.md` |
| Curation Engine (5 rules) | `agent/cosca-kernel/learnings.md` |
| Agent DNA v3.0 (28 fields) | `agent/cosca-kernel/learnings.md`, `agent/cosca-kernel/evolution.md` |
| Framework decisions | `decision/decision-enterprise-evolution-2026-07-23.md`, `decision/decision-enterprise-prompt-2026-07-12.md`, `decision/decision-audit-2026-07-12.md` |
| Agent capability | `agent/cosca-kernel/capability-profile.md` (L3, 0.88), `agent/cosca-evolution/capability-profile.md` (L1), `agent/cosca-governance/capability-profile.md` (L1), `agent/cosca-critic/capability-profile.md` (L1), `agent/cosca-paradigm/capability-profile.md` (L1) |

**Agents involved**: cosca-kernel (L3), cosca-evolution (L1), cosca-governance (L1), cosca-critic (L1), cosca-paradigm (L1)  
**Keywords**: `constitution`, `confidence-model`, `curation-engine`, `metacognition-pipeline`, `DNA-v3.0`, `28-fields`, `quality-gates-G0-G9`, `evidence-weights`, `curation-score`, `paradigm-shift`, `decision-critique`

---

### 1.11 Platform, DevOps & Infrastructure

**Coverage**: Low-Medium. Helm chart exists (devops L2). Platform, infrastructure, CI/CD at L1 only.

| Sub-topic | Files |
|-----------|-------|
| Helm chart (K8s) | `agent/cosca-devops/learnings.md` |
| ADR (platform, cache) | `architecture/adr/adr-2000-platform.md`, `adr-3159-cache.md` |
| Agent capability | `agent/cosca-devops/capability-profile.md` (L1), `agent/cosca-platform/capability-profile.md` (L1), `agent/cosca-infrastructure/capability-profile.md` (L1), `agent/cosca-cache/capability-profile.md` (L1) |

**Agents involved**: cosca-devops (L1), cosca-platform (L1), cosca-infrastructure (L1), cosca-cache (L1)  
**Keywords**: `CI/CD`, `Helm`, `Kubernetes`, `Docker`, `IaC`, `Terraform`, `blue-green-deploy`, `immutable-infrastructure`, `secrets-management`, `redis`, `CDN`, `multi-tier-cache`

---

### 1.12 Provider, AI & ML

**Coverage**: Low. Provider profile describes 10+ LLM providers. AI/ML at L1 seed only. No embeddings/RAG implementation detail.

| Sub-topic | Files |
|-----------|-------|
| LLM providers (11) | `architecture/system-architecture-overview.md`, `pattern/go-provider-pattern.md`, `architecture/adr/adr-2958-provider.md` |
| RAG & embeddings (aspirational) | `agent/cosca-ai/capability-profile.md` |
| Agent capability | `agent/cosca-provider/capability-profile.md` (L1), `agent/cosca-ai/capability-profile.md` (L1) |

**Agents involved**: cosca-provider (L1), cosca-ai (L1)  
**Keywords**: `OpenAI`, `Anthropic`, `Ollama`, `LLM-providers`, `RAG`, `embeddings`, `prompt-engineering`, `injection-attacks`, `provider-failover`, `cost-optimization`

---

### 1.13 SDKs, CLI & Developer Experience

**Coverage**: Low-Medium. TypeScript SDK test suite exists (186 tests). CLI at L1. SDK L1.

| Sub-topic | Files |
|-----------|-------|
| TypeScript SDK | `agent/cosca-sdk/learnings.md`, `architecture/adr/adr-2745-sdk.md` |
| CLI design | `agent/cosca-cli/capability-profile.md`, `architecture/adr/adr-3929-cli.md`, `decisions/cosca-cli/adr-001.md` through `adr-007.md` |
| Project overview | `project/cosca-cli-overview.md` |
| Agent capability | `agent/cosca-sdk/capability-profile.md` (L1), `agent/cosca-cli/capability-profile.md` (L1) |

**Agents involved**: cosca-sdk (L1), cosca-cli (L1)  
**Keywords**: `TypeScript-SDK`, `@cosca/sdk`, `Vitest`, `186-tests`, `Cobra-CLI`, `POSIX`, `shell-completion`, `code-generators`, `NPM-publication`

---

### 1.14 Plugins & WASM

**Coverage**: Low. Pattern documented. ADR exists. Plugin Chief at L1 seed only. No execution history.

| Sub-topic | Files |
|-----------|-------|
| Plugin pattern (Go/WASM/External) | `pattern/go-plugin-wasm-pattern.md`, `architecture/adr/adr-1430-plugin.md` |
| Agent capability | `agent/cosca-plugin/capability-profile.md` (L1) |

**Agents involved**: cosca-plugin (L1)  
**Keywords**: `WASM`, `wazero`, `plugin-runtime`, `sandboxing`, `hot-reload`, `SDK-contracts`, `plugin-registry`

---

### 1.15 Bootstrap, Discovery & Context

**Coverage**: Medium. Bootstrap optimized (57% faster). Discovery engine used for codebase scans. Context at L1.

| Sub-topic | Files |
|-----------|-------|
| Bootstrap optimization | `agent/cosca-automation/learnings.md`, `agent/cosca-kernel/learnings.md` |
| Codebase discovery | `agent/cosca-discovery/capability-profile.md`, `agent/cosca-kernel/learnings.md` |
| Session context | `context/session.md`, `context/cognitive-state.md` |
| Agent capability | `agent/cosca-bootstrap/capability-profile.md` (L1), `agent/cosca-discovery/capability-profile.md` (L1), `agent/cosca-context/capability-profile.md` (L1) |

**Agents involved**: cosca-bootstrap (L1), cosca-discovery (L1), cosca-context (L1), cosca-automation (L1)  
**Keywords**: `bootstrap`, `startup-optimization`, `cognitive-state`, `fast-path`, `Phase-0`, `codebase-scan`, `glob-grep`, `stack-detection`, `module-boundaries`

---

### 1.16 Sessions, Evolution & Roadmap

| Sub-topic | Files |
|-----------|-------|
| Active sessions | `sessions/active/current.md`, `session/INDEX.md` |
| Session archives | `sessions/archive/`, `short/session-*.md` |
| Evolution tracking | `evolution/learnings.md`, `agent/*/evolution.md` |
| Roadmap & milestones | `roadmap/INDEX.md`, `roadmap/milestones.md`, `roadmap/platform-evolution-v1.4.0.md` |
| Risk registry | `risk/RISK_REGISTRY.md` |

**Keywords**: `sessions`, `evolution-timeline`, `confidence-trajectory`, `milestones`, `risk-registry`, `auto-evolution`

---

## 2. Cross-Agent Knowledge Clusters

These are clusters where knowledge from ≥3 different agents intersects on the same domain concern, enabling rich cross-agent semantic search.

### Cluster A: 🔐 Auth & Security Stack
**Density**: HIGH (5 agents, 12+ files, real audit data)

**Agents**: cosca-security (L2), cosca-backend (L3), cosca-frontend (L2), cosca-runtime (L3), cosca-compliance (L1)

**Shared knowledge**:
- 3-tier auth chain (API Key → Cookie JWT → Bearer) — audited by security, mapped by backend, consumed by frontend
- CSRF double-submit pattern — implemented in backend middleware, auto-injected by frontend fetch wrapper, audited by security
- RBAC (admin/editor/viewer) — enforced in backend middleware, reflected in frontend route guards, verified by security audit
- Rate limiting — token bucket in backend, per-IP and per-endpoint, documented by security

**Core files**: `agent/cosca-security/learnings.md`, `agent/cosca-backend/learnings.md`, `agent/cosca-frontend/learnings.md`, `agent/cosca-runtime/learnings.md`

**Keywords**: `jwt`, `hs256`, `bcrypt`, `csrf`, `httpOnly-cookies`, `rate-limiting`, `rbac`, `middleware-chain`, `sentinel-cookie`, `double-submit`

---

### Cluster B: 📦 Data & Storage Reality
**Density**: MEDIUM-HIGH (4 agents, 8+ files, corrected fantasy)

**Agents**: cosca-database (L2), cosca-backend (L3), cosca-performance (L1), cosca-documentation (L3)

**Shared knowledge**:
- SQLite embedded reality (NOT PostgreSQL RDS) — discovered by database, verified by documentation, consumed by backend and performance
- FTS5 + sqlite-vec search pipeline — mapped by database, performance characteristics by performance
- Schema: agents, memory, knowledge, providers, skills, workflows, plugins, sessions, config
- WAL mode: concurrent reads, single writer — known bottleneck

**Core files**: `agent/cosca-database/learnings.md`, `agent/cosca-performance/learnings.md`, `agent/cosca-documentation/learnings.md`

**Keywords**: `sqlite`, `fts5`, `sqlite-vec`, `wal-mode`, `embedded-database`, `no-network-latency`, `single-writer`, `postgresql-fantasy`

---

### Cluster C: ⚡ Runtime Lifecycle & Observability
**Density**: MEDIUM (4 agents, 6+ files, critical bugs found)

**Agents**: cosca-runtime (L3), cosca-performance (L1), cosca-monitoring (L1), cosca-backend (L3)

**Shared knowledge**:
- 8-state machine with 20 transitions — fully mapped by runtime, consumed by all other agents
- 3 critical bugs: Restart() broken, EventStartupComplete premature, metrics misdocumented
- Daemon watchdog: auto-restart, stale PID detection, sync loop
- Metrics: 8 atomic counters + 4 sorted-slice histograms — no OpenTelemetry/Prometheus

**Core files**: `agent/cosca-runtime/learnings.md`, `agent/cosca-runtime/capability-profile.md`, `agent/cosca-performance/learnings.md`

**Keywords**: `state-machine`, `lifecycle`, `daemon`, `watchdog`, `metrics`, `atomic-counters`, `signal-handling`, `graceful-shutdown`, `Restart-broken`

---

### Cluster D: 📋 Documentation Integrity & Knowledge Management
**Density**: HIGH (5 agents, 10+ files, 887 docs audited)

**Agents**: cosca-documentation (L3), cosca-kernel (L3), cosca-discovery (L1), cosca-memory-chief (L1), cosca-security (L2)

**Shared knowledge**:
- Multi-source audit pattern: cross-reference README claims vs codebase scan vs memory files
- 3 critically stale files found: PostgreSQL fantasy, Go SDK fiction, compliance fabrication
- Version drift: docs/README.md v1.3.0 vs CHANGELOG v1.4.0-dev
- Memory health: 309 files, zero broken links, 95% frontmatter coverage

**Core files**: `agent/cosca-documentation/learnings.md`, `agent/cosca-kernel/learnings.md`, `agent/cosca-security/learnings.md`

**Keywords**: `cross-source-audit`, `badge-drift`, `version-inconsistency`, `number-verification`, `frontmatter-compliance`, `stale-content`, `aspirational-drift`

---

### Cluster E: 🎨 Frontend Architecture & State
**Density**: MEDIUM (3 agents, 5+ files)

**Agents**: cosca-frontend (L2), cosca-uiux (L1), cosca-specialist-frontend-component (L1)

**Shared knowledge**:
- Next.js 15 App Router, 26 routes, 2 route groups, 27 feature modules
- Feature-slice pattern: components/hooks/types per feature, barrel exports
- State: React Context + TanStack Query, NO Zustand/Redux
- Auth: httpOnly cookies (tokens never touch JS), sentinel cookie pattern
- Testing: Vitest (43 files), Playwright (6 specs), Storybook (21 stories), MSW (30+ endpoints)

**Core files**: `agent/cosca-frontend/learnings.md`, `agent/cosca-frontend/capability-profile.md`

**Keywords**: `Next.js-15`, `feature-slice`, `Radix-UI`, `TanStack-Query`, `MSW`, `PWA`, `httpOnly-cookies`

---

### Cluster F: 🧠 Metacognition & Framework Evolution
**Density**: MEDIUM (5 agents, 8+ files, framework-designed, unvalidated)

**Agents**: cosca-kernel (L3), cosca-critic (L1), cosca-paradigm (L1), cosca-evolution (L1), cosca-governance (L1)

**Shared knowledge**:
- Constitution: 7 immutable principles, chain of command, conflict resolution
- Confidence Model: 6 evidence levels, 7 modifiers, 0.30 threshold
- Curation Engine: 5 rules, CurationScore, auto-cycle
- Agent DNA v3.0: 23→28 fields, metacognition pipeline (8 stages)
- Critic framework: 5-question adversarial challenge (P0/P1 only)
- Paradigm framework: 3-question challenge (≥3 months data required)

**Core files**: `agent/cosca-kernel/learnings.md`, `agent/cosca-critic/learnings.md`, `agent/cosca-paradigm/learnings.md`

**Keywords**: `constitution`, `confidence-model`, `curation-engine`, `metacognition`, `DNA-v3.0`, `quality-gates`, `evidence-weighted`, `paradigm-shift`, `adversarial-critique`

---

### Cluster G: 🚀 Delivery Pipeline & Infrastructure
**Density**: LOW (4 agents, 4+ files, mostly L1 seed)

**Agents**: cosca-devops (L1), cosca-infrastructure (L1), cosca-platform (L1), cosca-cache (L1)

**Shared knowledge**:
- Helm chart with production-grade patterns (ConfigMap/Secret separation, layered probes, HPA v2, NetworkPolicy)
- Docker container strategy
- IaC principles (Terraform/Pulumi aspirational)
- Cache architecture (multi-tier, Redis aspirational)

**Core files**: `agent/cosca-devops/learnings.md`

**Keywords**: `Helm`, `Kubernetes`, `Docker`, `CI/CD`, `IaC`, `HPA`, `NetworkPolicy`, `probes`, `auto-scaling`

---

### Cluster H: 🔌 Extensibility (SDK + CLI + Plugins)
**Density**: LOW (4 agents, 6+ files, TypeScript SDK tested, others L1)

**Agents**: cosca-sdk (L1), cosca-cli (L1), cosca-plugin (L1), cosca-workflow-chief (L1)

**Shared knowledge**:
- TypeScript SDK: 12 test files, 186 tests, 0 failing
- CLI: Cobra-based, 37 root commands (123 with subcommands)
- Plugins: Go/WASM/External, wazero runtime, sandboxing
- Workflows: template-driven (project-init, feature-development, bug-fix, release, deployment)

**Core files**: `agent/cosca-sdk/learnings.md`, `decisions/cosca-cli/adr-001.md` through `adr-007.md`, `pattern/go-plugin-wasm-pattern.md`

**Keywords**: `TypeScript-SDK`, `Cobra-CLI`, `WASM-wazero`, `workflow-templates`, `plugin-sandbox`, `POSIX-conventions`

---

## 3. Agent Capability Matrix

Complete capability map for all 55 agents. L = Level, C = Confidence.

### Level 3+ (Proven — real tasks, verified outcomes)

| Agent | L | Conf | Primary Domain | Files |
|-------|---|------|----------------|-------|
| cosca-kernel | 3 | 0.88 | Agent orchestration, framework design, metacognition | `agent/cosca-kernel/` (6 files, 96 learnings lines) |
| cosca-documentation | 3 | 0.95 | Enterprise-scale doc audit (887 assets) | `agent/cosca-documentation/` (6 files, 45 learnings lines) |
| cosca-backend | 3 | 0.59 | REST API architecture, middleware, auth | `agent/cosca-backend/` (6 files, 31 learnings + 63 failures lines) |
| cosca-runtime | 2 | 0.95 | State machine, lifecycle, daemon, metrics | `agent/cosca-runtime/` (6 files, 31 learnings lines) |

### Level 2 (Active — real tasks, some areas unverified)

| Agent | L | Conf | Primary Domain | Files |
|-------|---|------|----------------|-------|
| cosca-security | 2 | 0.90 | Auth audit, OWASP, compliance correction | `agent/cosca-security/` (6 files, 59 learnings lines) |
| cosca-frontend | 2 | 0.85 | Next.js architecture extraction | `agent/cosca-frontend/` (6 files, 31 learnings lines) |
| cosca-database | 2 | 0.85 | SQLite reality correction, schema mapping | `agent/cosca-database/` (6 files, 31 learnings lines) |
| cosca-discovery | 1 | 0.75 | Codebase scanning (used in Phases 1-3) | `agent/cosca-discovery/` (6 files) |
| cosca-memory-chief | 1 | 0.75 | Memory health reporting (309 files) | `agent/cosca-memory-chief/` (6 files) |
| cosca-automation | 1 | — | Bootstrap optimization (Phase 0.5) | `agent/cosca-automation/` (6 files, 31 learnings lines) |

### Level 1 — Seed Data Only (no real task execution)

**41 agents** have zero real task execution. Their capability profiles are template-based (0.25 baseline). These include:

| Category | Agents |
|----------|--------|
| **Leadership** | cosca-ceo, cosca-cto, cosca-product |
| **Architecture** | cosca-architecture |
| **Development** | cosca-mobile, cosca-specialist-backend-api, cosca-specialist-backend-service |
| **Infrastructure** | cosca-devops, cosca-infrastructure, cosca-platform, cosca-migration |
| **Quality** | cosca-qa, cosca-testing, cosca-specialist-testing-unit, cosca-specialist-testing-integration, cosca-specialist-testing-e2e |
| **Specialists** | cosca-specialist-database-sql, cosca-specialist-documentation-writer, cosca-specialist-frontend-component, cosca-specialist-review-code |
| **Platform** | cosca-sdk, cosca-cli, cosca-plugin, cosca-cache, cosca-messaging, cosca-workflow-chief |
| **Operations** | cosca-monitoring, cosca-release, cosca-review, cosca-technical-debt |
| **Governance** | cosca-critic, cosca-paradigm, cosca-compliance, cosca-governance, cosca-evolution |
| **Integration** | cosca-integrations, cosca-provider, cosca-ai, cosca-analytics |
| **Foundation** | cosca-bootstrap, cosca-context, cosca-uiux, cosca-semantic-memory |
| **Extras** | agent-platform-chief.md, agent-performance-chief.md, agent-review-performance.md, agent-api-chief.md, agent-cosca-security.md, agent-cosca-architecture.md, agent-architecture-performance.md, agent-compliance-chief.md, agent-kernel-performance.md |

---

## 4. Knowledge Gaps (Density Analysis)

### 4.1 Critical Gaps (P0 — Missing foundational knowledge)

| Gap | Affected Agents | Impact |
|-----|-----------------|--------|
| **No automated security scanning** | cosca-security (govulncheck/gosec/semgrep at 0.10-0.20 confidence) | Manual audits only. CI has no security gate. |
| **No runtime integration tests** | cosca-runtime (0.05), cosca-testing | 20 state transitions untested. Restart() bug unfixed. |
| **No performance profiling** | cosca-performance (all profiling at 0.10-0.15) | Bottlenecks are architectural hypotheses only. No data. |
| **No observability (OpenTelemetry)** | cosca-runtime (0.10), cosca-monitoring | No distributed tracing, no Prometheus export, no SLOs. |
| **No CI/CD automation** | cosca-devops (seed), cosca-discovery (no CI integration) | 100% manual deployment. No automated code quality gates. |
| **Bug-005 uninvestigated** | cosca-database, cosca-performance | Slow dashboard query root cause unknown. No EXPLAIN run. |
| **Restart() broken** | cosca-runtime | Critical lifecycle bug. Found but not fixed. |
| **EventStartupComplete premature** | cosca-runtime | Subscribers receive startup event before init hooks. |

### 4.2 Major Gaps (P1 — Important but not blocking)

| Gap | Affected Agents | Impact |
|-----|-----------------|--------|
| **No accessibility audit (WCAG 2.1 AA)** | cosca-frontend (0.20), cosca-uiux | Cannot ship accessible product. |
| **No visual regression testing** | cosca-frontend (0.10), cosca-qa | UI regressions undetected. |
| **No feature flags** | cosca-frontend (0.10), cosca-product | Cannot do gradual rollouts. |
| **No STRIDE threat model** | cosca-security (0.15) | Security posture is checklist-based only. |
| **No token revocation** | cosca-security (0.10) | No way to revoke compromised tokens. |
| **No hot reload** | cosca-runtime (0.10) | Documented but not implemented. |
| **No dependency graph generation** | cosca-discovery (0.30) | Import cycle detection impossible. |
| **No indexed memory retrieval** | cosca-memory-chief (0.60) | Memory loading may be PATH-based, not indexed. |

### 4.3 Structural Gaps (Cross-cutting)

| Gap | Description |
|-----|-------------|
| **Agent-to-agent knowledge transfer** | Only 10 of 55 agents have recorded learnings. 82% are seed-only. No mechanism for agents to learn from each other's failures. |
| **Negative memory underutilization** | Only 3 of 55 agents have failure entries. The "avoided failures" pattern (cosca-backend) is exemplary but unique. |
| **Pattern memory is thin** | Only 3 of 55 agents have patterns.md content (cosca-critic, cosca-paradigm with 1 each). No agent has reusable solution patterns documented. |
| **Evolution tracking incomplete** | Only 15 of 55 agents have evolution.md content. 40 agents have no evolution timeline at all. |
| **Leadership agents unexercised** | ceo, cto, product, architecture chiefs have never executed a real task. Chain of command exists only on paper. |
| **Specialist agents never used** | All 10 specialist agents are L1 seed. The delegation pyramid (Chief → Specialist) is untested. |
| **No real execution for 75% of agents** | 41 of 55 agents are template-only. The agent ecosystem exists on paper but has not been battle-tested at scale. |

---

## 5. Metrics Summary

### Coverage by Memory Type

| Memory Type | Files | Substantive Content | Density |
|-------------|-------|---------------------|---------|
| Agent memory | 334 | ~40 files (12%) | LOW |
| Architecture | 19 | 19 files (100%) | HIGH |
| Pattern | 8 | 8 files (100%) | HIGH |
| Bug | 7 | 7 files (100%) | HIGH |
| Long/Knowledge | 7 | 4 files (57%) | MEDIUM |
| Codebase | 4 | 4 files (100%) | HIGH |
| Session/Short | 6 | 6 files (100%) | HIGH |
| Decision | 12 | 12 files (100%) | HIGH |
| Testing | 4 | 4 files (100%) | HIGH |
| Roadmap | 3 | 3 files (100%) | HIGH |
| Dependencies | 3 | 3 files (100%) | HIGH |
| Context | 3 | 3 files (100%) | HIGH |
| Risk | 1 | 1 file (100%) | HIGH |
| Project | 5 | 5 files (100%) | HIGH |
| Evolution | 2 | 2 files (100%) | HIGH |

**Key insight**: Non-agent memory types have near-100% density of substantive content. Agent memory, which represents 78% of all files, has only ~12% density. This is the primary knowledge gap.

### Topic Coverage Heatmap

| Topic | Files | Depth | Freshness | Gaps |
|-------|-------|-------|-----------|------|
| Architecture | 19+ | Deep | Fresh (Jul 28) | None |
| Documentation | 10+ | Deep | Fresh (Jul 28) | No CI automation |
| Auth/Security | 12+ | Deep | Fresh (Jul 28) | No automated scanning, STRIDE, token revocation |
| Database | 8+ | Medium | Fresh (Jul 28) | No WAL profiling, bug-005 |
| Runtime | 6+ | Deep | Fresh (Jul 28) | 3 bugs unfixed, no integration tests |
| Frontend | 5+ | Medium | Fresh (Jul 28) | No a11y, visual regression, feature flags |
| Performance | 4+ | Shallow | Fresh (Jul 28) | No profiling/benchmarking data |
| Governance | 8+ | Medium (designed) | Fresh (Jul 28) | Unvalidated (no execution history) |
| SDK/CLI/Plugins | 10+ | Shallow | Fresh | Mostly seed/L1 |
| DevOps/Infra | 4+ | Shallow | Fresh | Mostly seed/L1 |
| Testing/QA | 9+ | Medium (strategy) | Fresh | Chiefs at L1, specialists unused |
| AI/ML | 2+ | Shallow | Fresh | Seed only |
| Mobile | 0+ | None | Fresh | Seed only, no project relevance |
| Compliance | 3+ | Shallow | Fresh | Aspirational, no real implementation |

---

## 6. Recommendations for Cycle 2

Based on the semantic analysis, Cycle 2 should focus on densifying the agent memory layer and closing the most critical gaps.

### Priority 1: Activate Dormant Agents (highest ROI)
- **cosca-qa** + **cosca-testing**: Write tests for runtime state machine (20 transitions). This validates 5+ agents simultaneously.
- **cosca-ci/cd**: Design and implement CI pipeline with automated security scanning (govulncheck/gosec) and code quality gates.
- **cosca-monitoring**: Implement OpenTelemetry export for runtime metrics and Prometheus /metrics endpoint.
- **cosca-performance**: Run EXPLAIN QUERY PLAN on bug-005, Go benchmarks on search pipeline.

### Priority 2: Fix Known Critical Issues
- Fix Restart() bug (Stopped→Uninitialized transition)
- Fix EventStartupComplete timing (move after init hooks)
- Root-cause bug-005 with EXPLAIN + pprof
- Implement hot reload trigger

### Priority 3: Densify Agent Memory (record learnings)
- Bootstrap learnings.md for all 41 L1 agents from their SKILL.md definitions
- Have active agents (cosca-backend, cosca-security, cosca-frontend) record patterns from their successes
- Have cosca-critic and cosca-paradigm perform their first decision/paradigm reviews
- Record failures for all agents that attempt tasks (not just cosca-backend)

### Priority 4: Enable Cross-Agent Knowledge Transfer
- Implement semantic search (keyword + vector) across all memory files
- Create agent "avoided failures" cross-reference system (like cosca-backend's pattern)
- Build cross-agent cluster queries: "show me all knowledge about auth from any agent"
- Link agent failures to capability profile confidence adjustments automatically

### Priority 5: Validate Governance Framework
- Run first Curation Engine cycle on real memory data
- Apply Constitution compliance check to at least one agent
- Have cosca-critic review one real decision with its 5-question framework
- Begin accumulating Confidence Model data (prerequisite for cosca-paradigm activation)

### Measurable Targets for Cycle 2

| Metric | Current (C1) | Target (C2) |
|--------|-------------|-------------|
| Agents with substantive learnings | 10 (18%) | 20 (36%) |
| Agents with failure records | 3 (5%) | 10 (18%) |
| Agents with evolution timelines | 15 (27%) | 30 (55%) |
| Agents with pattern records | 2 (4%) | 10 (18%) |
| Critical bugs fixed | 0 of 3 | 3 of 3 |
| Automated CI checks | 0 | 3+ (security, docs, coverage) |
| Cross-agent semantic queries | 0 | Implemented (keyword + vector) |
| Agent L3+ count | 4 | 6 |

---

## 7. Index Maintenance

This index is a living document. After Cycle 2 completes:
1. Re-run full semantic scan of all memory files
2. Update topic coverage heatmap with new density scores
3. Recalculate cross-agent clusters
4. Update gap analysis with closed/found gaps
5. Version: INDEX.md → INDEX-v1.md, create new INDEX-v2.md

**Scheduled refresh**: After every agent evolution cycle or every 30 days, whichever comes first.

---

> **Generated by**: cosca-semantic-memory (Semantic Memory Chief)  
> **Based on**: 426 files across 19 categories, 55 agents  
> **Next cycle**: C2 — Agent Activation & Knowledge Densification
