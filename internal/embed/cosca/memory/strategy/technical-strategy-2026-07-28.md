# CoscaAI Technical Strategy — v1.4.0-dev

> **Author**: cosca-cto (CTO Agent) | **Date**: 2026-07-28 | **Wave**: 6 — Activation & Synthesis
> **Sources**: Audits W1-W5, 16 ADRs, 4 roadmap docs, Risk Registry, Technical Debt Scorecard, Cognitive State
> **Status**: Approved baseline | **Review cadence**: Bi-weekly (next: 2026-08-11)

---

## Executive Summary

CoscaAI is a **55-agent enterprise AI platform** at v1.4.0-dev, built on Go 1.25 + Next.js 15 + SQLite. Across Waves 1-5, 47 of 55 agents (85%) have been activated, producing 4 comprehensive domain audits, 16 ADRs, a quantified risk registry (20 risks), and a technical debt scorecard (75 items, 1,375/3,200). The platform demonstrates **strong architectural foundation** with **production-critical gaps** in infrastructure reliability, provider resilience, AI completeness, and developer experience.

**Overall Technical Maturity**: **2.7 / 5** — Solid dev platform, not yet production-grade.

**Composite Intelligence Score (CIS)**: 84-86/100 (estimated, needs real calibration data).

**Confidence Average**: 0.55 (target: 0.70).

**Strategic Imperative**: Close the gap between architecture documentation and code implementation across 3 key domains: provider resilience (circuit breaker, failover), infrastructure productionization (TLS, DR, scaling), and AI pipeline hardening (ANN index, RAG pipeline, safety).

---

## 1. Technical Maturity Assessment

### 1.1 Domain-by-Domain Maturity Scores

| Domain | Maturity (0-5) | Assessment | Owner (Chief) |
|--------|---------------|------------|---------------|
| **Core Runtime & Kernel** | 4 | Stable state machine, gRPC defined, health checks functional. Restart() fix pending (BUG-U01). | cosca-runtime |
| **CI/CD Pipeline** | 3 | 7-gate functional. Missing CD stage, soak test, clean-state test, multi-platform matrix. | cosca-devops |
| **Container Strategy** | 4 | Multi-stage Docker (scratch/Alpine), non-root, static binary. Best-in-class. | cosca-devops |
| **Infrastructure (AWS + K8s)** | 2 | Helm well-structured but production defaults disabled. Terraform minimal (no ALB, no TLS). No DR. | cosca-infrastructure |
| **Networking & Security Ops** | 1 | No TLS anywhere, no WAF, no CDN, no security groups. Secrets in plaintext pattern. | cosca-security + cosca-infrastructure |
| **Observability** | 2 | Prometheus endpoint + 33 Grafana panels documented. No distributed tracing, no alerting rules, orchestration metrics not exposed. | cosca-monitoring + cosca-analytics |
| **AI Subsystem** | 3 | 6-layer pipeline (embedding → vector → graph → hybrid search → re-ranking → engine). No ANN/HNSW, no RAG pipeline, no AI safety, 5/9 providers missing embedding registration. | cosca-ai |
| **Provider Layer** | 3 | 10 LLM providers. OpenAIC Compat pattern reduces duplication 80%. Missing: circuit breaker, multi-provider failover, rate limiting in 3 providers. | cosca-provider |
| **API Design** | 4 | REST (50 ops) + gRPC (12 RPCs). OpenAPI spec. JWT + RBAC. | cosca-api + cosca-backend |
| **CLI** | 4 | 46 commands, Cobra, multi-format output, shell completion. | cosca-cli |
| **TypeScript SDK** | 4 | v1.1.0, 12 domain modules, typed, published npm. | cosca-sdk |
| **Go SDK** | 1 | Aspirational — types only. No HTTP client, no streaming, no typed methods. | cosca-sdk |
| **Web Console** | 4 | Next.js 15, 21+ pages, PWA, Lighthouse 100%, WCAG 2.1 AA+. No CI integration. | cosca-frontend |
| **Developer Experience** | 3 | Fast onboarding (~15 min), comprehensive Makefile. Missing: devcontainer, pre-commit hooks, CONTRIBUTING.md, ARCHITECTURE.md. | cosca-platform |
| **Documentation** | 3 | Deep sub-docs, 7 ADRs in docs/adr/. Missing root-level community files. | cosca-documentation |
| **Testing** | 3 | 500+ Go tests, 354 frontend tests, 46.5% coverage. 0% runtime integration, 0% E2E. | cosca-testing + cosca-qa |
| **Security** | 2 | JWT, RBAC, API keys, encryption at rest. No STRIDE model, no token revocation, no automated scanning in CI. | cosca-security |
| **Compliance** | 1 | GDPR/LGPD aspirational. No real self-assessment performed. No retention policy, no right-to-erasure endpoint. | cosca-compliance |
| **Disaster Recovery** | 0 | No backup strategy, no off-site replication, no DR runbook, no RTO/RPO. | cosca-infrastructure |
| **Agent Ecosystem** | 2 | 47/55 activated but 70% on seed data. 94% without failure records. Cross-agent validation at 0. | cosca-governance + cosca-kernel |
| **Technical Debt Management** | 3 | First scorecard completed (1,375/3,200). Debt quantified, prioritized, but reduction not started. | cosca-technical-debt |
| **Plugin System** | 2 | WASM runtime defined, provider registered. Incomplete — host functions missing. | cosca-plugin |
| **Release Engineering** | 3 | GoReleaser (6 platforms), manual tagging. No automated release pipeline. | cosca-release |

### 1.2 Maturity Heatmap

```
Domain                    | 0  1  2  3  4  5
--------------------------|----------------
Core Runtime & Kernel     |         ████████
CI/CD Pipeline            |      ██████
Container Strategy        |         ████████
Infrastructure (AWS+K8s)  |   ████
Networking & Security Ops | ██
Observability             |   ████
AI Subsystem              |      ██████
Provider Layer            |      ██████
API Design                |         ████████
CLI                       |         ████████
TypeScript SDK            |         ████████
Go SDK                    | ██
Web Console               |         ████████
Developer Experience      |      ██████
Documentation             |      ██████
Testing                   |      ██████
Security                  |   ████
Compliance                | ██
Disaster Recovery         | 
Agent Ecosystem           |   ████
Tech Debt Management      |      ██████
Plugin System             |   ████
Release Engineering       |      ██████

MEAN: 2.7/5
```

---

## 2. Top 5 Technical Risks with Mitigation

### Risk 1: Provider Resilience Gap — Circuit Breaker & Failover Missing
**Severity**: 🔴 Critical | **Probability**: 90% | **Impact**: High

**Description**: PROVIDER_INTERFACE.md §2.2-2.3 defines circuit breaker (3-state: CLOSED→OPEN→HALF_OPEN) and 3-tier failover chain (Primary→Secondary→Fallback). Neither exists in code. The executor binds to a single ChatProvider. If a provider fails, the entire agent chain breaks — no retry escalation, no fallback routing, no circuit opening to prevent cascading failures.

**Mitigation Plan**:
- **Week 1-2**: Implement circuit breaker at `internal/executor/` level (5 failures in 60s → OPEN for 30s). Integrate with existing retry logic. Export circuit state to Prometheus. **Effort**: 12h. **Owner**: cosca-provider + cosca-runtime.
- **Week 2-3**: Design and implement `ProviderChain` abstraction supporting priority-ordered provider list. Multi-provider executor that tries providers in sequence. Respect circuit breaker state when selecting. **Effort**: 16h. **Owner**: cosca-provider.
- **Week 3-4**: Add failover metrics (attempts, successes, failures per provider). Alert on failover activation. **Effort**: 4h. **Owner**: cosca-monitoring + cosca-provider.

### Risk 2: No High-Availability Deployment Path
**Severity**: 🟠 High | **Probability**: 85% | **Impact**: High

**Description**: Single replica everywhere (ECS `desired_count=1`, Helm `replicaCount=1`, HPA disabled). No TLS anywhere. SQLite with `ReadWriteOnce` blocks horizontal scaling. No DR backup for 408+ memory files — months of agent evolution at risk.

**Mitigation Plan**:
- **Week 1**: Enable TLS in Helm ingress (cert-manager annotation) + add PodDisruptionBudget. **Effort**: 2h. **Owner**: cosca-infrastructure.
- **Week 1-2**: Add ALB + TLS termination + security groups + proper VPC to Terraform. **Effort**: 12h. **Owner**: cosca-infrastructure.
- **Week 2**: Enable HPA + multi-replica defaults in Helm (≥2) + ServiceMonitor for Prometheus Operator. **Effort**: 4h. **Owner**: cosca-infrastructure.
- **Week 2**: DR backup — auto-commit memory after curation + weekly `.cosca/` backup. **Effort**: 4h. **Owner**: cosca-devops + cosca-memory-chief.
- **Month 2**: Evaluate SQLite→Postgres migration (or Litestream for SQLite replication) for HA. **Effort**: 40h. **Owner**: cosca-database + cosca-infrastructure.

### Risk 3: 94% of Agent Ecosystem Has No Failure Records
**Severity**: 🟠 High | **Probability**: 80% | **Impact**: High (compounding)

**Description**: 51 of 54 agents have no substantive `failures.md`. Only cosca-backend (63 lines), cosca-critic (11), cosca-paradigm (11) record failures. The platform cannot learn from mistakes because mistakes aren't recorded — same errors will recur across agents. Metacognition pipeline operates on empty data.

**Mitigation Plan**:
- **Ongoing**: Require failure recording as part of every agent task completion. Gate task acceptance on `failures.md` update when appropriate.
- **Wave 6-7**: Activate remaining 8 agents with real tasks. Target: 15+ agents with failure records (28% → up from 6%).
- **Process**: cosca-governance to audit failure recording compliance monthly.

### Risk 4: AI Pipeline Hard Ceiling at ~100K Vectors
**Severity**: 🟡 Medium | **Probability**: 65% | **Impact**: High (at scale)

**Description**: SQLiteVec uses brute-force cosine similarity — degrades linearly beyond ~100K vectors. No HNSW/FAISS/IVF index implemented. The monitoring threshold (trigger migration warning at 50K+) is defined but not active. When scale is reached, search latency will exceed acceptable SLOs.

**Mitigation Plan**:
- **Month 1**: Implement vector count monitoring + alert at 50K vectors. **Effort**: 2h. **Owner**: cosca-ai + cosca-monitoring.
- **Month 1-2**: Evaluate ANN library options (HNSW via Go, FAISS via CGo, pgvector via Postgres migration). Produce ADR-AI-001 for ANN selection. **Effort**: 8h. **Owner**: cosca-ai + cosca-performance.
- **Month 2-3**: Implement selected ANN index as `VectorStore` implementation behind existing interface. Zero API changes. **Effort**: 24h. **Owner**: cosca-ai.

### Risk 5: No Observability for Production Decision-Making
**Severity**: 🟡 Medium | **Probability**: 70% | **Impact**: High

**Description**: Prometheus `/metrics` endpoint exists (port 8371) with rich metrics. But: orchestration metrics (LLM costs, router decisions, cache hit rates) are in-memory only — not exposed. No alerting rules configured. No Grafana dashboards committed to repo. No distributed tracing. Platform operates blind when it matters most.

**Mitigation Plan**:
- **Week 1**: Expose orchestration metrics via Prometheus (add `writeOrchestrationMetrics()` to prometheus.go). **Effort**: 4h. **Owner**: cosca-analytics + cosca-monitoring.
- **Week 1**: Define SLOs for 3 key signals (error rate <1%, P99 latency <2s search/<5s context, uptime 99.9%). **Effort**: 2h. **Owner**: cosca-monitoring.
- **Week 2**: Commit Grafana dashboard JSON to `deploy/grafana/` with Executive Overview (5 rows, 20 panels). **Effort**: 4h. **Owner**: cosca-analytics + cosca-monitoring.
- **Week 3**: Add Alertmanager rules for error spikes, latency degradation, component health degradation. **Effort**: 3h. **Owner**: cosca-monitoring.
- **Month 2**: Add cost tracking ($/token per provider config → `cosca_llm_cost_dollars_total`). **Effort**: 6h. **Owner**: cosca-analytics + cosca-provider.

---

## 3. Top 5 Architecture Opportunities

### Opportunity 1: Unified Provider Resilience Layer
**Value**: Very High | **Effort**: 32h | **Impact**: All 10 LLM providers, all 55 agents

Build a `ProviderExecutor` abstraction layer between the executor and individual ChatProviders that provides: circuit breaker (3-state), rate limiting (token bucket, consistent across all providers), retry with exponential backoff (configurable per provider), multi-provider failover (priority chain), and cost tracking (per-call $ estimation). This converts documented architecture into working code and protects every agent that uses LLM providers.

**Key Decision**: Implement at the executor level (not per-provider) to ensure consistent behavior. Use decorator pattern to wrap existing ChatProvider interface without breaking provider implementations.

### Opportunity 2: SQLite → Postgres Migration Path (HA Enablement)
**Value**: High | **Effort**: 40h (migration) + 24h (infrastructure) | **Impact**: Enables horizontal scaling, multi-replica, read replicas

Current SQLite + `ReadWriteOnce` PVC is the single biggest blocker to production HA. Two paths:
- **Path A (Litestream)**: SQLite replication to S3 — minimal code changes, keeps single-node simplicity, 2h effort. Enables DR but not HA.
- **Path B (Postgres + pgvector)**: Full RDBMS migration — enables multi-replica, read replicas, connection pooling, native vector search (pgvector). 40h migration effort. Future-proof for scale.

**Recommendation**: Start with Litestream (Path A) for immediate DR. Evaluate Postgres migration at 50K+ vector threshold or production deployment gates. This is a reversible decision — the VectorStore interface abstracts the backend.

### Opportunity 3: Go SDK Client Library (Platform Completeness)
**Value**: High | **Effort**: 24h (MVP) + 16h (completion) | **Impact**: Go-native consumers, CLI internal usage, plugin system

The TypeScript SDK v1.1.0 is production-ready with 12 domain modules. The Go SDK is aspirational (types only, `docs/sdk/go.md` says "aspirational"). Building the Go client ($sdk/go/$) enables: CLI internals to use typed clients instead of raw HTTP, Go-native integrations, the plugin system to call Cosca APIs, and platform completeness (both main languages covered).

**Key Decision**: Use `oapi-codegen` to generate Go client from the existing OpenAPI spec (`api/rest/openapi.yaml`) rather than hand-writing. This ensures type safety, reduces maintenance, and auto-updates with API changes.

### Opportunity 4: RAG Pipeline — End-to-End Retrieval-Augmented Generation
**Value**: High | **Effort**: 40h | **Impact**: Unlocks AI features for all agents

All 6 layers exist (embedding → vector → graph → search → re-ranking → knowledge engine). But no end-to-end retrieval-augmented generation flow wires them together. Building the RAG pipeline creates a reusable capability that every agent can use for context-aware LLM calls.

**Architecture**: 
```
Agent Query → Semantic Router (selects agent) 
→ Knowledge Engine (searches knowledge.db)
→ Re-Ranker (scores results by relevance)
→ Context Builder (constructs prompt with top-K results)
→ Provider Call (LLM with retrieved context)
→ Response Validator (checks output quality, safety)
→ Agent Response
```

### Opportunity 5: Automated Code Generation Pipeline
**Value**: Medium-High | **Effort**: 20h | **Impact**: Developer productivity, type safety, maintenance reduction

Current code generation is ad-hoc (manual proto compilation, manual OpenAPI→TS types, manual mock writing). Building an automated pipeline provides: mock generation from Go interfaces (`mockgen` via `//go:generate`), OpenAPI spec drift detection in CI (fail if spec doesn't match code), OpenAPI→Go client generation (for SDK), OpenAPI→TS types automated in CI, and protobuf compilation gated in CI. This reduces boilerplate, prevents drift, and accelerates development.

---

## 4. Technology Decisions Pending

### 4.1 Make vs Buy Decisions

| # | Decision | Context | Recommendation | Urgency |
|---|----------|---------|----------------|----------|
| D-01 | **Vector Search at Scale** | SQLiteVec brute-force degrades at >100K vectors | **Make**: Implement HNSW or IVF via Go library behind existing VectorStore interface. Reject: external vector DB (Pinecone/Weaviate) — adds dependency, breaks self-contained architecture (ADR-0001). | P1 (within 3 months) |
| D-02 | **Database for HA** | SQLite blocks multi-replica | **Staged**: Litestream for DR now (2h), Postgres+pgvector evaluated at 50K vector threshold or production gate | P2 (within 6 months) |
| D-03 | **Go SDK Generation** | Go HTTP client needed | **Buy (generate)**: Use `oapi-codegen` from OpenAPI spec. Reject: hand-writing (maintenance burden). | P1 (this sprint) |
| D-04 | **Message Queue for Async** | Event-driven architecture needed for scale | **Make (internal)**: Start with in-process channel-based pub/sub. External broker (NATS/Kafka) when throughput exceeds single-node. ADR-MESSAGING-001 needed. | P3 (post v2.0) |
| D-05 | **Distributed Tracing** | OpenTelemetry vs vendor-specific | **Buy (standard)**: OpenTelemetry (vendor-neutral). Reject: Datadog/Honeycomb SDK lock-in. | P2 (next quarter) |
| D-06 | **Secrets Management** | Plaintext secrets in values.yaml pattern | **Buy (integrate)**: External Secrets Operator for K8s, AWS Secrets Manager for ECS. Reject: building custom vault. | P1 (this month) |
| D-07 | **CDN** | Static asset delivery | **Buy (CloudFront)**: AWS native, minimal config. Included in Terraform module. | P2 (next quarter) |
| D-08 | **WAF** | Web application firewall | **Buy (AWS WAF)**: AWS native, managed rules. Included in Terraform module. | P2 (next quarter) |
| D-09 | **Service Mesh** | Istio vs Linkerd vs none | **Defer**: Not needed at current scale. Re-evaluate when >10 services or multi-cluster. | P3 (post v2.0) |
| D-10 | **Animation/Motion Library** | UI polish | **Defer to cosca-uiux**: Evaluate framer-motion vs react-spring in isolation. | P3 (post v2.0) |

### 4.2 Architecture Decisions Pending

| # | ADR Needed | Context | Urgency |
|---|-----------|---------|----------|
| ADR-INF-001 | Choice of ECS Fargate over EKS/Kubernetes for AWS deployment | Terraform uses ECS Fargate, Helm chart assumes K8s. Two deployment paths need justification. | P1 |
| ADR-INF-002 | SQLite as primary database (single-node architecture) — with migration plan | Documents rationale for SQLite, Litestream DR, and Postgres evaluation criteria. | P1 |
| ADR-AI-001 | ANN/HNSW index selection for vector store >100K | Vector search scaling strategy. Build vs buy for ANN. | P2 |
| ADR-PLAT-001 | CLI Framework Choice (Cobra with custom output formatting) | Documents Cobra rationale, output formatter design, viper integration. | P2 |
| ADR-PLAT-002 | Monorepo Structure (Go + TS SDK + Web Console) | Package boundaries, shared types, cross-language coordination. | P2 |
| ADR-PLAT-003 | SDK Strategy (TypeScript-first, Go aspirational → full) | Documents why TS SDK is complete, Go SDK path, API versioning. | P1 |
| ADR-PLAT-004 | Developer Environment Strategy | Devcontainer, Docker Compose, hot reload, tooling choices. | P2 |
| ADR-PROV-001 | Provider Resilience Architecture (circuit breaker + failover) | Design of ProviderExecutor, ProviderChain, backoff strategy, cost tracking. | P1 |
| ADR-SEC-001 | Secrets Management Strategy (plaintext→Vault/SealedSecrets/ASM) | Production secrets rotation, audit, access control. | P1 |
| ADR-DR-001 | Disaster Recovery Strategy (RTO/RPO targets, backup cadence) | Memory backup, database backup, off-site replication plan. | P1 |

---

## 5. Technical Roadmap — 3 Sprints Prioritized

### Sprint 1: "Production Foundation" (Week 1-2, ~80h)

**Theme**: Close the P0 gaps that block production deployment.

```
SPRINT 1 GOAL: Maturity 2.7 → 3.2

┌─────────────────────────────────────────────────────────┐
│ PROVIDER RESILIENCE                                      │
│ ├─ [P0] Circuit breaker in executor (12h)                │
│ ├─ [P0] Multi-provider failover chain (16h)              │
│ └─ [P0] Rate limiting for openaicompat layer (4h)        │
│                                                          │
│ INFRASTRUCTURE                                           │
│ ├─ [P0] TLS in Helm ingress + PDB (2h)                   │
│ ├─ [P0] DR backup: auto-commit memory + weekly backup (4h)│
│ └─ [P0] Add ALB + TLS + security groups to Terraform (12h)│
│                                                          │
│ CI/CD + TESTING                                          │
│ ├─ [P0] Soak test 1h in CI with memory monitoring (8h)   │
│ ├─ [P0] Add -race gate + clean-state test to CI (6h)     │
│ └─ [P0] CD stage: push to registry on tag (4h)           │
│                                                          │
│ OBSERVABILITY                                            │
│ ├─ [P0] Expose orchestration metrics via Prometheus (4h) │
│ └─ [P0] Define 3 SLOs (error rate, latency, uptime) (2h) │
│                                                          │
│ PLATFORM                                                 │
│ ├─ [P0] CONTRIBUTING.md + ARCHITECTURE.md (5h)           │
│ └─ [P0] Devcontainer config (3h)                         │
│                                                          │
│ AI                                                       │
│ └─ [P1] Vector count monitoring + 50K alert (2h)         │
└─────────────────────────────────────────────────────────┘

Total: ~80h | Parallel tracks: 5 | Expected maturity gain: +0.5
```

### Sprint 2: "Platform Hardening" (Week 3-4, ~80h)

**Theme**: Close P1 gaps, complete platform foundations.

```
SPRINT 2 GOAL: Maturity 3.2 → 3.7

┌─────────────────────────────────────────────────────────┐
│ PROVIDER                                                 │
│ ├─ [P1] Bedrock batch embedding fix (4h)                  │
│ ├─ [P1] Remove duplicate utility functions (3h)          │
│ └─ [P1] Failover metrics + alerting (4h)                 │
│                                                          │
│ INFRASTRUCTURE                                           │
│ ├─ [P1] HPA + multi-replica in Helm (2h)                 │
│ ├─ [P1] ServiceMonitor for Prometheus Operator (2h)      │
│ ├─ [P1] External secrets (Sealed Secrets or Vault) (8h)  │
│ └─ [P1] Extend soak test to 24h (4h)                     │
│                                                          │
│ OBSERVABILITY                                            │
│ ├─ [P1] Grafana dashboards JSON to deploy/grafana/ (4h)  │
│ ├─ [P1] Alertmanager rules (error, latency, health) (3h) │
│ └─ [P1] Add cost tracking ($/token) (6h)                 │
│                                                          │
│ SDK                                                      │
│ ├─ [P1] Go SDK HTTP client MVP (oapi-codegen) (8h)       │
│ └─ [P1] Automate OpenAPI→TS SDK types in CI (2h)         │
│                                                          │
│ PLATFORM                                                 │
│ ├─ [P1] make setup + pre-commit hooks (6h)               │
│ ├─ [P1] go generate directives + make generate (4h)      │
│ └─ [P1] Dependabot/Renovate config (2h)                  │
│                                                          │
│ AI                                                       │
│ ├─ [P1] Register missing embedding providers (8h)        │
│ └─ [P1] Prompt safety scanner prototype (8h)             │
│                                                          │
│ TESTING                                                  │
│ ├─ [P1] Runtime integration tests (20 transitions) (16h) │
│ └─ [P1] API handler tests (top 5 handlers) (8h)          │
│                                                          │
│ SECURITY                                                 │
│ ├─ [P1] Token revocation mechanism (8h)                  │
│ └─ [P1] Automated security scanning in CI (8h)           │
└─────────────────────────────────────────────────────────┘

Total: ~80h | Parallel tracks: 8 | Expected maturity gain: +0.5
```

### Sprint 3: "Scale & Ecosystem" (Week 5-6, ~80h)

**Theme**: Prepare for scale, complete ecosystem.

```
SPRINT 3 GOAL: Maturity 3.7 → 4.2

┌─────────────────────────────────────────────────────────┐
│ AI — SCALE                                              │
│ ├─ [P2] Evaluate HNSW/FAISS ANN libraries (8h)          │
│ └─ [P2] ADR-AI-001 for ANN selection (2h)               │
│                                                          │
│ AI — RAG                                                │
│ ├─ [P2] End-to-end RAG pipeline (24h)                   │
│ └─ [P2] AI safety: content filtering + prompt injection (8h)│
│                                                          │
│ INFRASTRUCTURE                                           │
│ ├─ [P2] Litestream for SQLite DR (2h)                   │
│ ├─ [P2] CloudFront CDN + WAF in Terraform (8h)          │
│ ├─ [P2] Multi-AZ deployment in Terraform (8h)           │
│ └─ [P2] DR runbook + RTO/RPO targets (4h)               │
│                                                          │
│ OBSERVABILITY                                            │
│ └─ [P2] OpenTelemetry distributed tracing (16h)         │
│                                                          │
│ SDK                                                      │
│ ├─ [P2] Go SDK completion (streaming, retry, all APIs) (16h)│
│ └─ [P2] Go SDK docs with real examples (4h)             │
│                                                          │
│ PLATFORM                                                 │
│ ├─ [P2] CLI progress bars for long ops (4h)             │
│ ├─ [P2] Code generation pipeline (mockgen, openapi) (4h)│
│ └─ [P2] Benchmark regression detection in CI (6h)       │
│                                                          │
│ ECOSYSTEM                                               │
│ ├─ [P2] Activate remaining 8 agents with real tasks (16h)│
│ └─ [P2] CODEOWNERS + issue templates (2h)               │
│                                                          │
│ COMPLIANCE                                              │
│ ├─ [P2] GDPR/LGPD self-assessment (8h)                  │
│ └─ [P2] Data retention policy + right-to-erasure (8h)   │
└─────────────────────────────────────────────────────────┘

Total: ~80h | Parallel tracks: 8 | Expected maturity gain: +0.5
```

### Post-Sprint 3 Target State

| Metric | Baseline (W6) | After Sprint 3 | Delta |
|--------|--------------|----------------|-------|
| Overall Maturity | 2.7 | 4.2 | +1.5 |
| Infrastructure Maturity | 2.2 | 3.8 | +1.6 |
| Platform Maturity | 3.3 | 4.2 | +0.9 |
| AI Maturity (cosca-ai level) | 2 | 3 | +1.0 |
| Provider Maturity | 3 | 4 | +1.0 |
| Test Coverage | 46.5% | 65% | +18.5% |
| Tech Debt Score | 1,375 | 720 | -655 (-47.6%) |
| Agents with Learnings | 16/54 (30%) | 30/54 (55%) | +14 |
| Agents with Failures | 3/54 (6%) | 15/54 (28%) | +12 |
| P0 Gaps Closed | 0/8 | 8/8 | All |
| CIS (measured) | 84-86 (estimated) | ≥80 (calibrated) | — |
| Production Readiness | Pre-production | Production-ready for single-AZ | — |

---

## 6. Technical Governance Recommendations

### 6.1 Architecture Decision Records (ADR) Cadence

All significant technical decisions MUST have an ADR before implementation begins. The current ADR directory has 16 entries, mostly organizational (creating Chief roles). Technical ADRs are needed for the pending decisions in §4.2.

**Process**:
1. **Propose**: Any Chief may propose an ADR. Submit as PR to `internal/embed/cosca/memory/architecture/adr/`.
2. **Review**: cosca-architecture + cosca-cto review within 48h. cosca-critic applies 5-Question Challenge to P0/P1 decisions.
3. **Decide**: cosca-cto approves/rejects. cosca-ceo escalates if resource/budget impact >10% sprint capacity.
4. **Record**: Accepted ADRs stored in adr/ directory. Rejected ADRs documented with rationale.

### 6.2 Quality Gate Enforcement

From the cosca-qa Quality Gate Standard (Wave 2 deliverable), extend CI with:

| Gate | Description | Threshold | Blocks |
|------|-------------|-----------|--------|
| G0 | Build | `go build ./...` pass | Merge |
| G1 | Lint | golangci-lint v2 (0 warnings) | Merge |
| G2 | Vet | `go vet ./...` pass | Merge |
| G3 | Test | `go test -race ./...` pass (20min timeout) | Merge |
| G4 | Security | govulncheck + gosec pass | Merge |
| G5 | Coverage | ≥55% (→70% target) | Merge |
| G6 | Docs | Cross-reference validator pass | Merge |
| **G7** | **Soak** | **1h memory test (RSS ≤ baseline × 1.10)** | **Merge** |
| **G8** | **Integration** | **Runtime state machine tests pass** | **Merge** |
| **G9** | **Benchmark** | **No >10% regression in p95 search latency** | **Warning** |

Gates G7-G9 are new additions identified in Wave 5 audits.

### 6.3 Technical Debt Budget

Based on the Technical Debt Scorecard (1,375/3,200):

- **Per-sprint allocation**: Minimum 20% of sprint capacity dedicated to debt reduction.
- **Target reduction**: -215 points per sprint (tracked in scorecard).
- **Blockers**: Any blocker-severity item (BUG-U01 at 130) suspends feature work until resolved.
- **Review**: Monthly scorecard review by cosca-technical-debt. Scorecard published in memory/technical-debt/.

### 6.4 Technology Selection Principles

1. **Self-contained first**: Prefer in-process, no-external-dependency solutions (ADR-0001). Cosca runs as a single Go binary whenever possible.
2. **Open standards**: Prefer OpenTelemetry over vendor-specific, OpenAPI over proprietary, Prometheus over SaaS-only.
3. **Reversible decisions**: Choose architectures where backend decisions (database, vector store, cache) are behind Go interfaces — swap implementations without API changes.
4. **Build what differentiates**: Cosca's agents, metacognition, memory curation, and AI pipeline are the moat. Buy/integrate standard infrastructure (CDN, WAF, secrets management).
5. **TypeScript-first SDK, Go parity**: TypeScript SDK is the reference implementation. Go SDK follows. Both auto-generated from OpenAPI where possible.
6. **Scale when needed**: SQLite is fine for 0→100K vectors. Migrate only when monitoring shows approaching thresholds. Don't optimize prematurely.

### 6.5 Cross-Department Coordination

| Council | Members | Cadence | Purpose |
|---------|---------|---------|---------|
| **Architecture Council** | cosca-cto, cosca-architecture, cosca-backend, cosca-frontend, cosca-database | Bi-weekly | Review ADRs, cross-domain architecture decisions |
| **Infrastructure Council** | cosca-infrastructure, cosca-devops, cosca-security, cosca-monitoring | Weekly | Production readiness, incidents, capacity planning |
| **Quality Council** | cosca-qa, cosca-testing, cosca-review, cosca-technical-debt | Bi-weekly | Quality metrics, debt budget, testing strategy |
| **AI Council** | cosca-ai, cosca-provider, cosca-semantic-memory, cosca-plugin | Bi-weekly | AI pipeline, provider strategy, model management |

### 6.6 Agent Activation Cadence

Remaining 8 agents to activate (55 total → 47 active → 8 to go):

**Wave 6 (this wave)**:
- cosca-cto (this activation — Technical Strategy document)

**Wave 7 — Specialists**:
- cosca-specialist-testing-unit, cosca-specialist-testing-integration, cosca-specialist-testing-e2e
- cosca-specialist-backend-api, cosca-specialist-backend-service
- cosca-specialist-database-sql, cosca-specialist-frontend-component
- cosca-specialist-documentation-writer, cosca-specialist-review-code

Target: 55/55 activated by end of Sprint 2.

### 6.7 Memory & Learning Governance

- **Failure recording**: Mandatory for every task that encounters an error, unexpected behavior, or approach that didn't work. Tracked in compliance audit by cosca-governance.
- **Pattern extraction**: Every 5 tasks, extract reusable patterns into `patterns.md`. Cross-reference with other agents' patterns.
- **Capability profile updates**: After every task, update `capability-profile.md` with new confidence scores, strengths, weaknesses learned.
- **Cross-agent validation**: Agents must validate outputs of at least 1 other agent per sprint cycle to build the M2 modifier in the Confidence Model.

---

## 7. References

| Document | Path |
|----------|------|
| Cognitive State (current) | `internal/embed/cosca/memory/context/cognitive-state.md` |
| Risk Registry | `internal/embed/cosca/memory/risk/RISK_REGISTRY.md` |
| Technical Debt Scorecard | `internal/embed/cosca/memory/technical-debt/scorecard.md` |
| AI Capability Profile (Wave 5) | `internal/embed/cosca/memory/agent/cosca-ai/capability-profile.md` |
| Provider Learnings (Wave 5) | `internal/embed/cosca/memory/agent/cosca-provider/learnings.md` |
| Infrastructure Audit (Wave 5) | `internal/embed/cosca/memory/agent/cosca-infrastructure/audit-report-2026-07-28.md` |
| Platform Audit (Wave 5) | `internal/embed/cosca/memory/agent/cosca-platform/audit-report-2026-07-28.md` |
| Analytics & Observability Audit | `internal/embed/cosca/memory/architecture/adr/analytics-observability-audit-2026-07-28.md` |
| AI Architecture (ADR-0001) | `internal/embed/cosca/memory/architecture/adr/adr-0001-ai-architecture.md` |
| ADR Directory (16 files) | `internal/embed/cosca/memory/architecture/adr/` |
| Roadmap Index | `internal/embed/cosca/memory/roadmap/INDEX.md` |
| Milestones | `internal/embed/cosca/memory/roadmap/milestones.md` |
| Platform Evolution v1.4.0 | `internal/embed/cosca/memory/roadmap/platform-evolution-v1.4.0.md` |
| Wave 2 Plan | `internal/embed/cosca/memory/roadmap/onda-2-plan.md` |
| Quality Gates | `internal/embed/cosca/QUALITY_GATES.md` |
| Cosca Kernel (bootstrap) | `internal/embed/cosca/agents/cosca-kernel/` |

---

## Appendix A: Sprint Dependency Graph

```
Sprint 1 dependencies:
  circuit_breaker ───────────────────────┐
  failover ─────────── depends on ───────┤
  rate_limiting ─────────────────────────┤
  tls_helm_pdb ──────────────────────────┤
  dr_backup ─────────────────────────────┤
  alb_tls_terraform ─────────────────────┤ ALL INDEPENDENT
  soak_test ─────────────────────────────┤ (parallel tracks)
  race_clean_state_test ─────────────────┤
  cd_stage ──────────────────────────────┤
  orchestration_metrics ─────────────────┤
  slo_definitions ───────────────────────┤
  contributing_architecture ─────────────┤
  devcontainer ──────────────────────────┘

Sprint 2 dependencies:
  Some items depend on Sprint 1 outputs:
    grafana_dashboards ← orchestration_metrics (S1)
    go_sdk_client ← openapi_types_automation (S2, parallel)
    integration_tests ← race_clean_state_test (S1)

Sprint 3 dependencies:
  Some items depend on Sprint 2 outputs:
    ann_evaluation ← vector_count_monitoring (S1 data)
    rag_pipeline ← embedding_providers (S2)
    litestream ← dr_backup (S1)
    go_sdk_completion ← go_sdk_mvp (S2)
```

---

*Generated by cosca-cto (CTO Agent) — Wave 6 Activation — 2026-07-28*
*Next review: 2026-08-11 (bi-weekly)*
