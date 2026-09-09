# CoscaAI — Product Backlog Consolidado

> **Autor**: cosca-product (Product Chief) | **Data**: 2026-07-28 | **Versão**: 1.0.0
> **Fontes**: Auditorias Onda 5 (cosca-ai, cosca-provider, cosca-infrastructure, cosca-mobile, cosca-platform, cosca-analytics) + Roadmap milestones.md + Cognitive State PENDING section

---

## 1. Sumário Executivo

**Total de recomendações brutas**: 83 (provenientes de 6 auditorias + roadmap)
**Após desduplicação**: 68 recomendações únicas
**Já concluídas**: 4 (Memory Decay Engine, Intelligence Score, Context Compression, P0 CI fixes)
**Backlog ativo**: 64 itens

**Distribuição**:
| Prioridade | Quantidade | Esforço total estimado |
|-----------|-----------|----------------------|
| P0 (Imediato — Crítico) | 17 | ~54h |
| P1 (Curto prazo — Este mês) | 25 | ~112h |
| P2 (Médio prazo — Próximo trimestre) | 22 | ~280h |
| P3 (Longo prazo / Postergado) | 4 | ~120h |

**Maturidade atual da plataforma**: CIS 84-86/100, Maturidade de Infraestrutura 2.2/5, Platform Engineering 3.3/5.

---

## 2. Agrupamento por Tema/Epic

### EPIC-A: Provider Resilience & Reliability
**Goal**: Transformar a camada de provider de "funcional single-provider" para "resiliente com failover". É o maior gap de confiabilidade da plataforma.

**Total**: 8 itens (3 P0, 4 P1, 1 P2)
**Esforço total**: ~38h
**Stakeholders**: All agents usando LLM (55), Runtime, DevOps, SRE
**Success metrics**: Provider uptime >99.9%, failover latency <500ms, circuit breaker prevents cascading failures

### EPIC-B: AI Platform Completeness
**Goal**: Fechar gaps críticos de AI — RAG pipeline, safety filters, embedding coverage, vector store scalability.

**Total**: 9 itens (3 P0, 2 P1, 4 P2)
**Esforço total**: ~96h
**Stakeholders**: cosca-ai, cosca-semantic-memory, cosca-provider, Knowledge Engine, todos agentes que usam search
**Success metrics**: RAG pipeline funcionando end-to-end, prompt injection detection >95% recall, all major embedding providers registered

### EPIC-C: Developer Experience & Community Readiness
**Goal**: Plataforma aberta para contribuidores externos com documentação completa e ambiente dev de 1 comando.

**Total**: 14 itens (5 P0, 7 P1, 2 P2)
**Esforço total**: ~42h
**Stakeholders**: Desenvolvedores externos, novos membros do time, cosca-documentation, cosca-platform
**Success metrics**: Onboarding time 15min→5min, 100% docs completeness, dev environment fully reproducible

### EPIC-D: CI/CD Pipeline Maturity
**Goal**: CI com CD stage, soak tests, multi-platform, gates para TS/Web, automação de dependências.

**Total**: 11 itens (3 P0, 6 P1, 2 P2)
**Esforço total**: ~44h
**Stakeholders**: DevOps, Testing, Security, todos os devs que abrem PRs
**Success metrics**: CD push-to-registry automático, 100% TS/Web CI coverage, 0 dependências desatualizadas

### EPIC-E: SDK Ecosystem
**Goal**: SDK multiplataforma (Node.js, Browser, React Native, Go) com qualidade de produção.

**Total**: 8 itens (1 P0, 3 P1, 4 P2)
**Esforço total**: ~60h
**Stakeholders**: Desenvolvedores mobile, desenvolvedores Go, integradores
**Success metrics**: axios→fetch migration done, Go SDK MVP with 5+ modules, SDK CI gates active

### EPIC-F: Infrastructure Production Readiness
**Goal**: Infraestrutura preparada para produção com TLS, HA, DR, segurança de rede.

**Total**: 16 itens (2 P0, 6 P1, 8 P2)
**Esforço total**: ~146h
**Stakeholders**: SRE, Security, DevOps, FinOps
**Success metrics**: TLS ativado em todos os entrypoints, HA deployment (≥2 replicas), DR runbook testado

### EPIC-G: Observability, SLOs & Alerting
**Goal**: Plataforma observável com SLOs definidos, dashboards Grafana, alertas inteligentes.

**Total**: 7 itens (2 P0, 3 P1, 2 P2)
**Esforço total**: ~38h
**Stakeholders**: SRE, Monitoring, Analytics, Kernel
**Success metrics**: 5 SLOs definidos e monitorados, <5min MTTR, 100% cobertura de alertas críticos

### EPIC-H: Mobile Platform Enablement
**Goal**: Desbloquear desenvolvimento mobile (React Native) da plataforma Cosca.

**Total**: 3 itens (1 P0, 1 P1, 1 P2)
**Esforço total**: ~28h
**Stakeholders**: Desenvolvedores mobile, cosca-sdk, cosca-mobile
**Success metrics**: @cosca/sdk compatível com React Native, Mobile ADR aprovado, Dashboard mobile protótipo funcional

### EPIC-I: Security & Compliance
**Goal**: Fechar gaps de segurança (prompt injection, secrets management, content safety) e compliance.

**Total**: 5 itens (1 P0, 1 P1, 2 P2, 1 P3)
**Esforço total**: ~36h + P3 contínuo
**Stakeholders**: Security, Compliance, Governance, todos agentes
**Success metrics**: 0 vulnerabilidades críticas, secrets externamente gerenciados, compliance auto-assessment ≥80%

### EPIC-J: Platform Evolution (v2.0)
**Goal**: Preparar a plataforma para escala com Kernel HA, WASM plugins, Shadow Execution.

**Total**: 5 itens (0 P0, 0 P1, 3 P2, 2 P3)
**Esforço total**: ~100h + 3-4 sprints
**Stakeholders**: Kernel, CTO, Architecture, toda a plataforma
**Success metrics**: Kernel SPOF mitigado, WASM host functions funcionais, Shadow Execution MVP

---

## 3. Backlog Completo — Priorizado (Impacto × Esforço)

### Matriz de Priorização

```
Impacto
  ▲  ALTO   │ P0 (RÁPIDO)          │ P0 (ESTRATÉGICO)     │ P1 (PLANEJADO)
  │          │ Baixo esforço, alto   │ Alto esforço, alto   │ Alto esforço, alto
  │          │ impacto → FAZER JÁ    │ impacto → PRIORIZAR  │ valor → PLANEJAR
  │          │                       │                       │
  │  MÉDIO   │ P1 (QUICK WIN)       │ P1 (SÓLIDO)          │ P2 (BACKLOG)
  │          │ Baixo esforço, médio  │ Médio esforço, médio │ Baixo esforço, médio
  │          │ impacto → ENCAIXAR    │ impacto → SPRINT     │ impacto → AVALIAR
  │          │                       │                       │
  │  BAIXO   │ P2 (OPORTUNIDADE)    │ P2 (PREENCHE)        │ P3 (POSTERGADO)
  │          │ Baixo esforço, baixo  │ Médio esforço, baixo │ Alto esforço, baixo
  │          │ impacto → SE DER      │ impacto → UM DIA     │ impacto → DEPOIS
  └──────────┴──────────────────────┴──────────────────────┴──────────────────────► Esforço
             BAIXO (<4h)             MÉDIO (4h-16h)         ALTO (>16h)
```

---

### P0 — CRÍTICO: Esta Semana (Sprint 1)

| # | ID | Epic | Descrição | Fonte | Esforço | Impacto | Owner Principal |
|---|-----|------|-----------|-------|---------|---------|-----------------|
| 1 | PROV-001 | EPIC-A | Implement circuit breaker pattern (3-state: CLOSED→OPEN→HALF_OPEN) no executor level | cosca-provider P0 | 8h | 🔴 Critical | cosca-provider |
| 2 | PROV-002 | EPIC-A | Implement multi-provider failover routing (Primary→Secondary→Fallback chain) | cosca-provider P0 | 8h | 🔴 Critical | cosca-provider |
| 3 | PROV-003 | EPIC-A | Add RateLimiter to openaicompat.ChatProvider (DeepSeek, Groq, Mistral) | cosca-provider P1 | 3h | 🟠 High | cosca-provider |
| 4 | AI-001 | EPIC-B | Registrar providers de embedding faltantes (OpenAI nativo, Google, Azure, Ollama, Bedrock) | cosca-ai P0 | 6h | 🔴 Critical | cosca-ai + cosca-provider |
| 5 | AI-002 | EPIC-B | Implement prompt injection safety scanner | cosca-ai P0 | 8h | 🔴 Critical | cosca-ai + cosca-security |
| 6 | AI-003 | EPIC-B | Criar ADR-001 (AI Architecture & RAG Pipeline Decision) | cosca-ai P2 | 3h | 🟠 High | cosca-ai + cosca-architecture |
| 7 | PLAT-001 | EPIC-C | Criar root-level CONTRIBUTING.md (code of conduct, PR process, conventions) | cosca-platform P0-1 | 2h | 🟡 Medium | cosca-platform + cosca-documentation |
| 8 | PLAT-002 | EPIC-C | Criar root-level ARCHITECTURE.md (C4 diagrams, package graph, ADR index) | cosca-platform P0-2 | 3h | 🟡 Medium | cosca-platform + cosca-architecture |
| 9 | PLAT-003 | EPIC-C | Criar CODEOWNERS file | cosca-platform P0-5 | 1h | 🟡 Medium | cosca-platform |
| 10 | PLAT-004 | EPIC-C | Criar GitHub issue templates (.github/ISSUE_TEMPLATE/) + PR template | cosca-platform P0-6 | 1h | 🟡 Medium | cosca-platform + cosca-documentation |
| 11 | PLAT-005 | EPIC-C | Criar devcontainer config (.devcontainer/devcontainer.json + Dockerfile.dev) | cosca-platform P0-3 | 3h | 🟡 Medium | cosca-platform |
| 12 | CI-001 | EPIC-D | Add soak test (1h) to CI pipeline with memory monitoring | cosca-infra P0-1 | 8h | 🔴 Critical | cosca-devops + cosca-testing |
| 13 | CI-002 | EPIC-D | Add CD stage to CI pipeline (push Docker images to registry on tag, Helm chart validation) | cosca-infra P1-8 + cosca-platform P0-4 | 4h | 🟠 High | cosca-devops + cosca-platform |
| 14 | CI-003 | EPIC-D | Add Dependabot/Renovate config for Go modules + Docker + npm | cosca-infra P1-9 + cosca-platform P1-7 | 2h | 🟡 Medium | cosca-devops + cosca-platform |
| 15 | INFRA-001 | EPIC-F | Enable TLS in Helm ingress (cert-manager annotation example) | cosca-infra P0-4 | 1h | 🔴 Critical | cosca-infrastructure |
| 16 | INFRA-002 | EPIC-F | Add PodDisruptionBudget to Helm chart | cosca-infra P0-5 | 1h | 🟠 High | cosca-infrastructure |
| 17 | SDK-001 | EPIC-E | Migrar @cosca/sdk axios→fetch adapter (desbloqueia React Native + browser + Node.js) | cosca-mobile P0 + cognitive-state P1 | 12h | 🔴 Critical | cosca-sdk + cosca-mobile |

**Total P0**: 17 itens | **Esforço**: ~74h | **Stakeholders**: 14+ agentes

---

### P1 — CURTO PRAZO: Este Mês (Sprint 2-3)

| # | ID | Epic | Descrição | Fonte | Esforço | Impacto | Owner Principal |
|---|-----|------|-----------|-------|---------|---------|-----------------|
| 18 | PROV-004 | EPIC-A | Remover funções utilitárias duplicadas (isNonRetryable, truncateBody, estimateTokens) — importar de common.go | cosca-provider P2 | 2h | 🟡 Medium | cosca-provider |
| 19 | PROV-005 | EPIC-A | Corrigir Bedrock embedding batching (N requests → 1 batch request) | cosca-provider P2 | 4h | 🟡 Medium | cosca-provider |
| 20 | PROV-006 | EPIC-A | Standardize retry configuration across all 10 providers | cosca-provider (implied) | 4h | 🟡 Medium | cosca-provider |
| 21 | PROV-007 | EPIC-A | Implement cost tracking per provider/agent (budget alerts) | PROVIDER_INTERFACE.md § gap | 8h | 🟡 Medium | cosca-provider + cosca-analytics |
| 22 | AI-004 | EPIC-B | Implement content safety filters (toxic output detection, content moderation) | cosca-ai P1 | 8h | 🟠 High | cosca-ai + cosca-security |
| 23 | AI-005 | EPIC-B | Criar prompt management system (templates, versioning, A/B testing) | cosca-ai P1 | 12h | 🟡 Medium | cosca-ai + cosca-platform |
| 24 | PLAT-006 | EPIC-C | Add `make setup` target — one-command dev env bootstrap | cosca-platform P1-1 | 3h | 🟡 Medium | cosca-platform |
| 25 | PLAT-007 | EPIC-C | Add `//go:generate` directives + `make generate` aggregation target | cosca-platform P1-2 | 4h | 🟡 Medium | cosca-platform |
| 26 | PLAT-008 | EPIC-C | Add `make security-check` target (govulncheck + gosec) | cosca-platform P1-3 | 2h | 🟡 Medium | cosca-platform + cosca-security |
| 27 | PLAT-009 | EPIC-C | Add pre-commit hook config (.pre-commit-config.yaml + husky/lint-staged) | cosca-platform P1-6 | 3h | 🟡 Medium | cosca-platform |
| 28 | PLAT-010 | EPIC-C | Automate OpenAPI → TypeScript SDK types regeneration in CI (fail on drift) | cosca-platform P1-8 | 2h | 🟡 Medium | cosca-platform + cosca-sdk |
| 29 | PLAT-011 | EPIC-C | Add mock generation from Go interfaces (mockgen integrated into Makefile + CI) | cosca-platform P1-9 | 3h | 🟡 Medium | cosca-platform |
| 30 | PLAT-012 | EPIC-C | Document SDK versioning and compatibility policy | cosca-platform P1-10 | 2h | ⚪ Low | cosca-platform + cosca-sdk |
| 31 | CI-004 | EPIC-D | Add TypeScript SDK CI gate (test + typecheck + build) | cosca-platform P1-4 | 2h | 🟡 Medium | cosca-platform + cosca-devops |
| 32 | CI-005 | EPIC-D | Add Web Console CI gate (lint + typecheck + test) | cosca-platform P1-5 | 2h | 🟡 Medium | cosca-platform + cosca-devops |
| 33 | CI-006 | EPIC-D | Add clean-state test to CI (fresh install with empty .cosca/ validated) | cosca-infra P0-2 (partial) | 3h | 🟠 High | cosca-devops |
| 34 | CI-007 | EPIC-D | Extend soak test to 24h (scheduled, not per-PR) | cosca-infra P1-1 | 4h | 🟠 High | cosca-devops |
| 35 | CI-008 | EPIC-D | Add multi-platform CI matrix (add macOS to CI) | cosca-infra P2-4 | 4h | ⚪ Low | cosca-devops |
| 36 | INFRA-003 | EPIC-F | Enable HPA + multi-replica defaults in Helm (≥2 replicas) | cosca-infra P1-4 | 2h | 🟠 High | cosca-infrastructure |
| 37 | INFRA-004 | EPIC-F | Add ServiceMonitor for Prometheus Operator | cosca-infra P1-5 | 2h | 🟡 Medium | cosca-infrastructure + cosca-monitoring |
| 38 | INFRA-005 | EPIC-F | Add ALB + TLS to Terraform config | cosca-infra P1-2 | 8h | 🟠 High | cosca-infrastructure |
| 39 | INFRA-006 | EPIC-F | Add security groups + proper VPC to Terraform | cosca-infra P1-3 | 4h | 🟠 High | cosca-infrastructure |
| 40 | INFRA-007 | EPIC-F | Implement external secrets management (Sealed Secrets or Vault) | cosca-infra P1-7 | 8h | 🟠 High | cosca-security + cosca-infrastructure |
| 41 | INFRA-008 | EPIC-F | Criar DR backup: auto-commit memory after curation + weekly .cosca/ backup | cosca-infra P0-3 | 4h | 🟠 High | cosca-devops + cosca-memory-chief |
| 42 | OBS-001 | EPIC-G | Implantar Grafana dashboards JSON em deploy/grafana/ (Runtime Health, API Overview, Provider Status) | cosca-infra P1-6 + cosca-platform P1-11 | 4h | 🟠 High | cosca-monitoring + cosca-platform |
| 43 | OBS-002 | EPIC-G | Conectar métricas de orquestração ao Prometheus /metrics endpoint | cosca-analytics P0 + cognitive-state P1 | 6h | 🟠 High | cosca-analytics + cosca-monitoring |
| 44 | OBS-003 | EPIC-G | Definir 5 SLOs com SLI, target, e error budget (task routing, search, bootstrap, API response, memory retrieval) | cosca-analytics + cosca-monitoring (Onda 2 plan) | 4h | 🟠 High | cosca-monitoring + cosca-analytics |
| 45 | SDK-002 | EPIC-E | Start Go SDK HTTP client implementation (sdk/go/ with CoscaClient) | cosca-platform P1-12 | 8h | 🟠 High | cosca-platform + cosca-sdk |
| 46 | SDK-003 | EPIC-E | Criar gRPC server implementation (proto exists, server pending) | milestones.md P0 | 16h | 🟡 Medium | cosca-backend + cosca-cli |
| 47 | SDK-004 | EPIC-E | TypeScript SDK completion (all remaining modules) | milestones.md P1 | 8h | 🟡 Medium | cosca-sdk |
| 48 | MOB-001 | EPIC-H | Draft Mobile ADR (arquitetura, stack, roadmap mobile) | cosca-mobile P1 | 3h | 🟡 Medium | cosca-mobile + cosca-architecture |
| 49 | SEC-001 | EPIC-I | Content safety compliance alignment (alinhar safety filters com GDPR/LGPD) | cross-audit synthesis | 8h | 🟠 High | cosca-security + cosca-compliance + cosca-ai |

**Total P1**: 32 itens | **Esforço**: ~154h | **Stakeholders**: 20+ agentes

**Nota**: P1 items originalmente 32, mas pode ser reajustado conforme os P0 são concluídos.

---

### P2 — MÉDIO PRAZO: Próximo Trimestre (Sprint 4+)

| # | ID | Epic | Descrição | Fonte | Esforço | Impacto | Owner Principal |
|---|-----|------|-----------|-------|---------|---------|-----------------|
| 50 | AI-006 | EPIC-B | Implement HNSW/ANN index opcional para vector store (>100K vetores) | cosca-ai P2 | 24h | 🟡 Medium | cosca-ai + cosca-performance |
| 51 | AI-007 | EPIC-B | Criar pipeline RAG explícito (ingest→chunk→embed→retrieve→generate) | cosca-ai P0 (deferred) | 24h | 🔴 Critical | cosca-ai + cosca-semantic-memory |
| 52 | AI-008 | EPIC-B | Implement multi-language import extraction no Knowledge Graph (Python, TS, Rust, Java) | cosca-ai P2 | 8h | 🟡 Medium | cosca-ai |
| 53 | AI-009 | EPIC-B | Implement model deployment/monitoring para AI pipeline | cosca-ai (implied) | 16h | 🟡 Medium | cosca-ai + cosca-monitoring |
| 54 | PLAT-013 | EPIC-C | Implement CLI dry-run mode for write operations (cosca install, plugin install) | cosca-platform P2-3 | 6h | ⚪ Low | cosca-cli + cosca-platform |
| 55 | PLAT-014 | EPIC-C | Add progress bars to long-running CLI operations (index, sync, plugin install) | cosca-platform P2-4 | 4h | ⚪ Low | cosca-cli + cosca-platform |
| 56 | CI-009 | EPIC-D | Add Benchmark regression detection to CI | cosca-platform P2-9 | 6h | 🟡 Medium | cosca-performance + cosca-devops |
| 57 | CI-010 | EPIC-D | Add FIXME/TODO tracking automation to CI | cosca-platform P2-10 | 2h | ⚪ Low | cosca-platform |
| 58 | INFRA-009 | EPIC-F | Evaluate SQLite → Postgres migration for HA (or Litestream for SQLite replication) | cosca-infra P2-1 | 40h | 🔴 Critical | cosca-database + cosca-infrastructure |
| 59 | INFRA-010 | EPIC-F | Multi-AZ deployment in Terraform | cosca-infra P2-4 | 8h | 🟠 High | cosca-infrastructure |
| 60 | INFRA-011 | EPIC-F | Add CloudFront CDN to Terraform | cosca-infra P2-2 | 4h | ⚪ Low | cosca-infrastructure |
| 61 | INFRA-012 | EPIC-F | Add WAF rules to Terraform | cosca-infra P2-3 | 4h | ⚪ Low | cosca-security |
| 62 | INFRA-013 | EPIC-F | Multi-region DR plan with RTO/RPO targets | cosca-infra P2-6 | 24h | 🟠 High | cosca-infrastructure |
| 63 | INFRA-014 | EPIC-F | Cost optimization review (right-sizing, reserved instances) | cosca-infra P2-7 | 4h | ⚪ Low | cosca-infrastructure + FinOps |
| 64 | INFRA-015 | EPIC-F | Service mesh evaluation (Istio/Linkerd) | cosca-infra P2-8 | 8h | ⚪ Low | cosca-infrastructure |
| 65 | INFRA-016 | EPIC-F | Kernel routing redundancy — R9 mitigation (dual-kernel consensus or agent self-routing) | cosca-infra P2-9 | 16h | 🟡 Medium | cosca-kernel + cosca-infrastructure |
| 66 | OBS-004 | EPIC-G | Implement OpenTelemetry distributed tracing | cosca-infra P2-5 + cosca-analytics P2 | 16h | 🟡 Medium | cosca-monitoring + cosca-infrastructure |
| 67 | OBS-005 | EPIC-G | Implement alerting rules: error rate >5%, p95 latency >2x baseline, agent confidence drop | cosca-monitoring (Onda 2) | 4h | 🟡 Medium | cosca-monitoring |
| 68 | OBS-006 | EPIC-G | Implement cost tracking dashboard (provider costs per agent/operation) | cosca-analytics P1 | 8h | ⚪ Low | cosca-analytics + cosca-provider |
| 69 | SDK-005 | EPIC-E | Complete Go SDK client library (streaming, retry, all API endpoints) | cosca-platform P2-5+P2-7 | 16h | 🟡 Medium | cosca-sdk + cosca-platform |
| 70 | SDK-006 | EPIC-E | API client code generation from OpenAPI spec (Go) | cosca-platform P2-8 | 4h | ⚪ Low | cosca-platform + cosca-sdk |
| 71 | SDK-007 | EPIC-E | Create ESM/CJS dual build for @cosca/sdk | cosca-platform SDK-02 | 4h | ⚪ Low | cosca-sdk + cosca-platform |
| 72 | MOB-002 | EPIC-H | Prototype CoscaAI Mobile Dashboard (Expo + @cosca/sdk com novo fetch adapter) | cosca-mobile P2 | 16h | 🟡 Medium | cosca-mobile + cosca-frontend |
| 73 | SEC-002 | EPIC-I | Implement FOSSA/SPDX license compliance scanning | cosca-platform (implied) | 4h | ⚪ Low | cosca-security + cosca-platform |
| 74 | SEC-003 | EPIC-I | Add SonarQube or static analysis beyond linters | cosca-platform (implied) | 8h | ⚪ Low | cosca-security + cosca-platform |
| 75 | EVO-001 | EPIC-J | WASM host functions implementation (plugin system blocker) | cognitive-state P2 | 24h | 🟡 Medium | cosca-plugin + cosca-runtime |
| 76 | EVO-002 | EPIC-J | Shadow Execution MVP (sandbox + diff engine) | platform-evolution #1 | 40h | 🟡 Medium | cosca-kernel + cosca-runtime |
| 77 | EVO-003 | EPIC-J | Causalidade tree: upgrade Bug Registry com 4 níveis (causa→arquitetural→processo→prevenção) | platform-evolution #4 | 1h | ⚪ Low | cosca-technical-debt |
| 78 | EVO-004 | EPIC-J | Performance benchmarking suite (baselines para todas as operações críticas) | milestones.md P2 | 12h | 🟡 Medium | cosca-performance |
| 79 | EVO-005 | EPIC-J | Provider coverage completion (add 6 more providers) | milestones.md P2 | 12h | ⚪ Low | cosca-provider |

**Total P2**: 30 itens | **Esforço**: ~332h

---

### P3 — LONGO PRAZO / POSTERGADO

| # | ID | Epic | Descrição | Fonte | Esforço | Motivo Postergação |
|---|-----|------|-----------|-------|---------|-------------------|
| 80 | SEC-004 | EPIC-I | Compliance remediation completa (GDPR, LGPD, SOC2) — 12-16 semanas | cognitive-state P3 | 80h | Após base de segurança P0/P1 implementada |
| 81 | EVO-006 | EPIC-J | Knowledge Replay (re-execução de tarefas antigas em sandbox) | platform-evolution #7 | 40h | Depende de Shadow Execution (#1) |
| 82 | EVO-007 | EPIC-J | Experiment-Based Evolution (agente como cientista) | platform-evolution #10 | 80h | Depende de Shadow Execution + Knowledge Replay |
| 83 | EVO-008 | EPIC-J | Controle de Deriva (Evolution Consistency Check) | platform-evolution #8 | 40h | Complexidade NLP cross-domain. Alternativa social primeiro. |

**Total P3**: 4 itens | **Esforço**: ~240h

---

## 4. Sprint Planning Sugerido

### Sprint 1 (Week 1-2): "Resilience & Foundation"
**Theme**: Provider reliability + Community readiness

**Sprint Goal**: Circuit breaker e failover implementados, documentação pública no lugar, dev environment reproduzível.

```
P0 items (17 — executar em paralelo por times):
  ├── Time A: Provider Resilience
  │   ├── PROV-001 Circuit breaker (8h) — cosca-provider
  │   ├── PROV-002 Multi-provider failover (8h) — cosca-provider
  │   └── PROV-003 RateLimiter openaicompat (3h) — cosca-provider
  │
  ├── Time B: AI Platform
  │   ├── AI-001 Embedding providers registration (6h) — cosca-ai
  │   ├── AI-002 Prompt injection safety (8h) — cosca-ai
  │   └── AI-003 ADR-001 AI Architecture (3h) — cosca-ai
  │
  ├── Time C: Developer Experience
  │   ├── PLAT-001 CONTRIBUTING.md (2h)
  │   ├── PLAT-002 ARCHITECTURE.md (3h)
  │   ├── PLAT-003 CODEOWNERS (1h)
  │   ├── PLAT-004 Issue/PR templates (1h)
  │   └── PLAT-005 Devcontainer (3h)
  │
  ├── Time D: CI/CD + Infra
  │   ├── CI-001 Soak test 1h (8h)
  │   ├── CI-002 CD stage (4h)
  │   ├── CI-003 Dependabot/Renovate (2h)
  │   ├── INFRA-001 TLS Helm ingress (1h)
  │   ├── INFRA-002 PodDisruptionBudget (1h)
  │   └── SDK-001 axios→fetch migration (12h) — cosca-sdk
  │
  └── Cross-team: Gate Review
      └── cosca-review validates all P0 deliverables

  Deliverables:
    ✅ Circuit breaker with metrics
    ✅ Multi-provider failover chain
    ✅ All embedding providers registered
    ✅ Prompt injection safety scanner
    ✅ Root-level docs (CONTRIBUTING, ARCHITECTURE, CODEOWNERS, templates)
    ✅ Devcontainer working
    ✅ Soak test in CI
    ✅ CD push-to-registry
    ✅ @cosca/sdk v2.0.0 with fetch adapter (axios-free)
```

---

### Sprint 2 (Week 3-4): "Quality & Observability"
**Theme**: CI maturity + Observability + SDK ecosystem

**Sprint Goal**: Pipeline CI com todos os gates, dashboards Grafana ativos, Go SDK MVP.

```
P1 items (enfoque ~16 itens):
  ├── Time A: Provider Polish
  │   ├── PROV-004 Remove duplicated utils (2h)
  │   ├── PROV-005 Bedrock batching fix (4h)
  │   ├── PROV-006 Standardize retry (4h)
  │   └── PROV-007 Cost tracking (8h)
  │
  ├── Time B: CI/CD Completion
  │   ├── CI-004 TS SDK CI gate (2h)
  │   ├── CI-005 Web CI gate (2h)
  │   ├── CI-006 Clean-state test (3h)
  │   ├── CI-007 24h soak test (4h)
  │   └── CI-008 Multi-platform CI macOS (4h)
  │
  ├── Time C: Observability
  │   ├── OBS-001 Grafana dashboards (4h)
  │   ├── OBS-002 Orchestration metrics → Prometheus (6h)
  │   ├── OBS-003 SLO definitions (4h)
  │   └── PLAT-008 make security-check (2h)
  │
  ├── Time D: Platform Engineering
  │   ├── PLAT-006 make setup (3h)
  │   ├── PLAT-007 go generate + make generate (4h)
  │   ├── PLAT-009 Pre-commit hooks (3h)
  │   ├── PLAT-010 OpenAPI TS types auto (2h)
  │   ├── PLAT-011 Mock generation (3h)
  │   └── PLAT-012 SDK versioning doc (2h)
  │
  └── Time E: SDK + Mobile
      ├── SDK-002 Go SDK HTTP client (8h)
      ├── SDK-003 gRPC server (16h)
      ├── SDK-004 TS SDK completion (8h)
      └── MOB-001 Mobile ADR (3h)

  Deliverables:
    ✅ CI pipeline with TS/Web/clean-state gates
    ✅ Grafana dashboards in repo
    ✅ 5 SLOs defined and monitored
    ✅ Go SDK MVP (CoscaClient, knowledge, memory, runtime)
    ✅ gRPC server functional
    ✅ Mobile ADR approved
```

---

### Sprint 3 (Week 5-8): "Scale & Security"
**Theme**: Infrastructure HA + AI platform depth + Security harden

**Sprint Goal**: Multi-replica deployment, AI pipeline robusto, secrets management production-ready.

```
P1 residual + Early P2 items:
  ├── Infra HA
  │   ├── INFRA-003 HPA + multi-replica (2h)
  │   ├── INFRA-004 ServiceMonitor (2h)
  │   ├── INFRA-005 ALB + TLS Terraform (8h)
  │   ├── INFRA-006 Security groups + VPC (4h)
  │   ├── INFRA-007 External secrets (8h)
  │   └── INFRA-008 DR backup (4h)
  │
  ├── AI Platform
  │   ├── AI-004 Content safety filters (8h)
  │   ├── AI-005 Prompt management (12h)
  │   └── AI-009 Model deployment/monitoring (16h)
  │
  ├── SDK Complete
  │   └── SDK-005 Go SDK complete (16h)
  │
  ├── Security
  │   ├── SEC-001 Content safety compliance (8h)
  │   └── SEC-002 License compliance (4h)
  │
  └── Observability
      ├── OBS-005 Alerting rules (4h)
      └── OBS-006 Cost tracking dashboard (8h)

  Deliverables:
    ✅ Production-grade infra (TLS, VPC, secrets, DR, HA)
    ✅ Content safety + prompt management operational
    ✅ Go SDK feature-complete
```

---

### Sprint 4+ (Sprint 9-12): "Platform Evolution"
**Theme**: Database evolution + AI scalability + v2.0 foundations

```
P2 deep items:
  ├── Database Evolution
  │   └── INFRA-009 SQLite→Postgres/Litestream evaluation (40h)
  │
  ├── AI Scalability
  │   ├── AI-006 HNSW/ANN index (24h)
  │   ├── AI-007 RAG pipeline (24h)
  │   └── AI-008 Multi-language Knowledge Graph (8h)
  │
  ├── Platform Evolution
  │   ├── EVO-001 WASM host functions (24h)
  │   ├── EVO-002 Shadow Execution MVP (40h)
  │   ├── INFRA-016 Kernel routing redundancy (16h)
  │   └── OBS-004 OpenTelemetry tracing (16h)
  │
  └── Production Hardening
      ├── INFRA-010 Multi-AZ deployment (8h)
      ├── INFRA-013 Multi-region DR plan (24h)
      └── EVO-004 Performance benchmarking suite (12h)
```

---

## 5. Métricas de Sucesso por Tema/Epic

| Epic | Métrica | Baseline (2026-07-28) | Target Pós-Sprint 1 | Target Pós-Sprint 3 | Target Final |
|------|---------|----------------------|---------------------|---------------------|-------------|
| **EPIC-A** Provider Resilience | Provider uptime | ~85% (single-provider) | 99.5% | 99.9% | 99.95% |
| | Failover latency | N/A (não implementado) | <2s | <500ms | <200ms |
| | Circuit breaker trips/mês | 0 (não implementado) | ≥1 (validation) | >0 (production usage) | Prevented cascading failures |
| **EPIC-B** AI Platform | Embedding providers registered | 4/10 | 9/10 | 10/10 | 10/10 |
| | Prompt injection detection recall | 0% (não implementado) | >90% | >95% | >98% |
| | Vector store query latency (100K) | ~50ms (brute-force) | ~50ms | ~30ms (HNSW) | ~10ms |
| **EPIC-C** Developer Exp | Onboarding time (clone→running) | 15 min | 10 min | 5 min | 5 min |
| | Docs completeness | 3/5 | 4/5 | 4.5/5 | 5/5 |
| | Dev environment reproducibility | Manual | Devcontainer | 1-command | 1-command |
| **EPIC-D** CI/CD | CI gates active | 6 (Go-only) | 8 (+TS +Web) | 9 (+soak) | 10 (full coverage) |
| | Deployment automation | 0% (manual) | 100% (tag→registry) | 100% (tag→staging) | 100% (tag→prod) |
| | Dependencies auto-updated | 0% | 100% (Dependabot on) | 100% | 100% |
| **EPIC-E** SDK | axios→fetch migration | 0% (axios-only) | 100% (fetch adapter) | 100% | 100% |
| | Go SDK maturity | 1/5 (types-only) | 2/5 (client started) | 3/5 (MVP done) | 4/5 (complete) |
| | SDK cross-platform (Node+Browser+RN) | Node only | Node+Browser+RN | Node+Browser+RN | Node+Browser+RN+Deno |
| **EPIC-F** Infra Production | TLS enabled | 0% (no TLS anywhere) | Helm ingress TLS | Terraform ALB TLS | All entrypoints TLS |
| | HA deployment | 1 replica | 2 replicas | 3 replicas (multi-AZ) | Multi-replica + multi-AZ |
| | DR capability | 0 (no backups) | Weekly .cosca/ backup | DR runbook tested | RTO <1h, RPO <5min |
| **EPIC-G** Observability | SLOs defined and monitored | 0 | 5 SLOs defined | 5 SLOs with dashboards | 5 SLOs with alerting |
| | Grafana dashboards in repo | 0 | 3 dashboards | 5 dashboards | 7 dashboards |
| | Mean time to detect (MTTD) | Unknown | <30 min | <10 min | <5 min |
| **EPIC-H** Mobile | @cosca/sdk React Native compat | 0% (axios blocks) | 100% (fetch adapter) | 100% | 100% |
| | Mobile ADR approved | No | No | Yes | Yes |
| | Mobile dashboard prototype | No | No | No | Yes (Sprint 4+) |
| **EPIC-I** Security | Prompt injection safety | 0% | Basic scanner | Production scanner | ML-based scanner |
| | External secrets management | Plaintext | Vault/SealedSecrets | Full integration | Automated rotation |
| | Compliance auto-assessment | 0% | 50% | 70% | ≥80% |
| **EPIC-J** Evolution | Kernel SPOF mitigated | Single kernel | Monitoring | Critic oversight | Routing redundancy |
| | WASM host functions | 0 | — | — | MVP functional |
| | Shadow Execution | 0 | — | — | MVP functional |

---

## 6. Stakeholders por Epic

| Epic | Primários | Secundários | Impactados indiretamente |
|------|-----------|-------------|------------------------|
| **EPIC-A** Provider Resilience | cosca-provider (owner), cosca-runtime, cosca-security | cosca-ai, cosca-monitoring, cosca-sdk | Todos os 55 agentes (LLM dependency) |
| **EPIC-B** AI Platform | cosca-ai (owner), cosca-semantic-memory, cosca-provider | cosca-security, cosca-database, cosca-monitoring | Knowledge Engine, Search, todos agentes usando search |
| **EPIC-C** Developer Exp | cosca-platform (owner), cosca-documentation, cosca-architecture | cosca-governance, cosca-devops | Comunidade open-source, novos contribuidores |
| **EPIC-D** CI/CD | cosca-devops (owner), cosca-testing, cosca-security | cosca-platform, cosca-sdk, cosca-review | Todos devs abrindo PRs |
| **EPIC-E** SDK | cosca-sdk (owner), cosca-mobile, cosca-frontend | cosca-platform, cosca-backend, cosca-cli | Desenvolvedores mobile, Go devs, integradores |
| **EPIC-F** Infra Production | cosca-infrastructure (owner), cosca-security, cosca-devops | cosca-monitoring, cosca-database, cosca-compliance | SRE, FinOps, operadores de produção |
| **EPIC-G** Observability | cosca-monitoring (owner), cosca-analytics, cosca-runtime | cosca-infrastructure, cosca-performance, cosca-security | SRE, on-call engineers |
| **EPIC-H** Mobile | cosca-mobile (owner), cosca-sdk, cosca-frontend | cosca-uiux, cosca-documentation | Desenvolvedores mobile, usuários mobile |
| **EPIC-I** Security & Comp | cosca-security (owner), cosca-compliance, cosca-ai | cosca-governance, cosca-infrastructure | Compliance officers, auditores externos |
| **EPIC-J** Platform Evolution | cosca-kernel (owner), cosca-cto, cosca-architecture | cosca-runtime, cosca-plugin, cosca-evolution | Toda a plataforma, todos os agentes |

---

## 7. Riscos e Dependências Cross-Epic

| Risco | Impacto | Epics afetados | Mitigação |
|-------|---------|---------------|-----------|
| **Circuit breaker + failover podem quebrar providers existentes** | 🔴 Critical | EPIC-A, EPIC-B | Feature flag para enable/disable por provider. Testes de regressão em todos os 10 providers antes do merge. |
| **axios→fetch migration quebra integrações existentes** | 🔴 Critical | EPIC-E, EPIC-H | Versão major (v2.0.0) com migration guide. Backward compat via adapter opt-in. SemVer estrito. |
| **Soak test pode revelar memory leaks que bloqueiam releases** | 🟠 High | EPIC-D, EPIC-F, EPIC-G | CI gate pode ser "advisory" inicialmente (warning, não bloqueio) até leak ser corrigido. |
| **SQLite→Postgres migration pode causar regressão de performance** | 🟠 High | EPIC-F, EPIC-B | Benchmark comparativo antes de migration. Manter SQLite como opção para single-node. Litestream como alternativa mais leve. |
| **Prompt injection safety scanner pode ter falsos positivos** | 🟡 Medium | EPIC-B, EPIC-I | Scanner configurável com thresholds. Log de decisões para auditoria. Override manual suportado. |
| **RAG pipeline pode ser complexo demais para MVP** | 🟡 Medium | EPIC-B | MVP com pipeline mínimo (ingest→embed→search). Adicionar chunking e generation em iterações posteriores. |

---

## 8. Itens Concluídos (Já Entregues)

Estes itens apareciam nas auditorias mas foram concluídos entre a auditoria e este backlog:

| # | Descrição | Status |
|---|-----------|--------|
| ✅ | Memory Decay Engine (CurationScore v2.0) | Concluído 2026-07-28 |
| ✅ | Intelligence Score (CIS 84-86/100) | Concluído 2026-07-28 |
| ✅ | Context Compression (cognitive-state.md ~400 tokens) | Concluído 2026-07-28 |
| ✅ | P0 CI fixes (race condition, restart test, flaky chunker, coverage gate) | Concluído 2026-07-28 |
| ✅ | P0 fixes (cache connected to Search, embeddings DeepSeek fix, XSS fix, WebSocket Origin check) | Concluído 2026-07-28 |

---

## 9. Referências

| Documento | Path |
|-----------|------|
| Cognitive State (PENDING) | `.cosca/memory/context/cognitive-state.md` |
| AI Audit | `.cosca/memory/agent/cosca-ai/learnings.md` |
| Provider Audit | `.cosca/memory/agent/cosca-provider/learnings.md` |
| Infrastructure Audit | `.cosca/memory/agent/cosca-infrastructure/audit-report-2026-07-28.md` |
| Mobile Audit | `.cosca/memory/agent/cosca-mobile/learnings.md` |
| Platform Audit | `.cosca/memory/agent/cosca-platform/audit-report-2026-07-28.md` |
| Analytics Audit | `.cosca/memory/agent/cosca-analytics/learnings.md` |
| Milestones | `.cosca/memory/roadmap/milestones.md` |
| Platform Evolution | `.cosca/memory/roadmap/platform-evolution-v1.4.0.md` |
| Onda 2 Plan | `.cosca/memory/roadmap/onda-2-plan.md` |
| Risk Registry | `.cosca/memory/risk/RISK_REGISTRY.md` |
| Quality Gates | `.cosca/QUALITY_GATES.md` |

---

*Product Backlog Consolidado gerado por cosca-product (Activation Wave 6) — 2026-07-28*
