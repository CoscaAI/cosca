---
type: roadmap
key: cosca-milestones
tags: [roadmap, milestones, epics, cosca]
timestamp: 2026-07-26T00:00:00Z
status: active
---

# Cosca — Milestone Status

## Current Score: ~83/100 (exceeded v1.0 targets, approaching v2.0)

## Epic Progress

| Epic | Name | Original Target | Actual | Status |
|------|------|----------------|--------|--------|
| EPIC-001 | Foundation (tests, CI/CD, Docker) | 60% | ✅ ~90% | Complete |
| EPIC-002 | Core Stabilization (WASM, providers, editors) | 80% | 🔄 ~75% | In Progress |
| EPIC-003 | Subsystem Completion (agent/skill/prompt CLIs) | 100% | 🔄 ~60% | In Progress |
| EPIC-004 | Quality & Documentation | 100% | 🕐 ~50% | Pending |
| EPIC-005 | Release v1.0 | Release | 🕐 Not started | Pending |

## v2.0 Epics (Accelerated — partially delivered)

| Epic | Name | Status |
|------|------|--------|
| EPIC-006 | API Layer (REST + gRPC) | ✅ REST done, 🔄 gRPC started |
| EPIC-007 | SDK Ecosystem | 🔄 TS SDK draft, 🔄 Go SDK started |
| EPIC-008 | Security & Governance | ✅ JWT, RBAC, API Keys done |
| EPIC-009 | Enterprise Infrastructure | 🔄 Docker, Helm, Terraform started |
| EPIC-010 | Platform Ecosystem | ✅ Web Console Phase 0-4 done |
| EPIC-011 | Release v2.0 | 🕐 Future |

## Active Work Items

### Categoria A — ⭐ Imediato (após commit 2026-07-28)

| Priority | Item | Status | Esforço |
|----------|------|--------|---------|
| P0 | #4 — Árvore de Causalidade (Bug Registry upgrade) | 🕐 Pending | 1h |
| P0 | #2 — Memory Decay Engine (estender Curation Engine) | 🕐 Pending | 2d |
| P0 | #6 — Intelligence Score (estender Health Dashboard) | 🕐 Pending | 2d |
| P0 | #9 — Context Compression (session context → 2k tokens) | 🕐 Pending | 3d |

### Categoria B — 📋 Risco Controlado (próximo mês)

| Priority | Item | Status | Blocked By |
|----------|------|--------|------------|
| P1 | #5 — cosca-critic (advogado do diabo) | 🔄 Agent created | Dados reais de decisão para calibrar |
| P1 | #3 — cosca-paradigm (detecção de mudança de paradigma) | 🔄 Agent created | 3 meses de dados do Confidence Model |

### Categoria C — 🔮 v2.0 (depende de infra)

| Priority | Item | Status | Pré-requisito |
|----------|------|--------|---------------|
| P2 | #1 — Shadow Execution (sandbox + diff engine) | 🕐 Future | — |
| P2 | #7 — Knowledge Replay | 🕐 Future | Shadow Execution |
| P2 | #10 — Experiment-Based Evolution | 🕐 Future | Shadow Execution + Knowledge Replay |

### Categoria D — ⚠️ Postergado

| Priority | Item | Status | Motivo |
|----------|------|--------|--------|
| P3 | #8 — Controle de Deriva | 🕐 Deferred | Complexidade NLP cross-domain. Alternativa social primeiro. |

### Itens Anteriores (mantidos)

| Priority | Item | Status | Blocked By |
|----------|------|--------|------------|
| P0 | gRPC server implementation | 🔄 proto done, server pending | — |
| P1 | Helm chart productionization | 🔄 Chart created | CI/CD integration |
| P1 | TypeScript SDK completion | 🕐 Pending | OpenAPI spec generation |
| P2 | Performance benchmarking suite | 🕐 Pending | — |
| P2 | Provider coverage (6 more) | 🕐 Pending | — |

## Recent Completions
- ✅ **Platform Evolution Analysis** (2026-07-28): 10 melhorias analisadas contra 32 engines + 27 workflows. Roadmap definido: 4 imediatas, 2 planejadas, 3 pós-v2.0, 1 postergada.
- ✅ **Evolution Marathon** (2026-07-28): 10 commits, 150+ arquivos. Agent DNA v3.0, Metacognition Pipeline, Confidence Model, Memory Curation Engine, Platform Health Dashboard, 51 Capability Profiles.
- ✅ v1.3.0 Web Console (Phases 0-4): 17 routes, 36 endpoints, JWT, RBAC
- ✅ Linter cleanup: 959 issues resolved
- ✅ Pre-flight audit workflow: 20-step read-only audit
- ✅ Embedded Cosca resources: 258 files
- ✅ Self-contained .opencode/ configuration
- ✅ Memory Evolution Option B: 82 files, 388K

## Quality Gates
| Gate | Status |
|------|--------|
| 🏗️ Architecture | ✅ Pass |
| 🔒 Security | ✅ Pass (JWT, RBAC, CSRF) |
| ⚡ Performance | ⚠️ Need benchmarks |
| 🧪 Testing | ✅ ~78% coverage |
| 📚 Documentation | ✅ Pass |
| 🚀 Release | 🕐 Pre-release pending |

## Technology Radar — Post v2.0

Items to implement after Cosca reaches stable v2.0:

| # | Item | Trigger | Priority | Why wait |
|---|------|---------|----------|----------|
| 1 | **Memória semântica do Kernel** | Memória passar de 300+ arquivos | P2 | INDEX hierárquico atual é suficiente. Infra já existe (sqlite-vec, embeddings). |
| 2 | **Gatilho semântico de checkpoint** | Integrar com checkpoint de sessão | P1 | Firewall automático contra truncamento de contexto. Essencial pra sessões longas. |
| 3 | **Sessão semântica (busca)** | Se busca em sessões antigas for necessária | P3 | Memória já indexa tudo que importa. Sessão tem 70% de ruído. |
| 4 | **Shadow Execution (#1)** | v2.0 estável | P2 | Sandbox por agente, diff engine, rollback atômico. Custo alto, justifica com escala. |
| 5 | **Knowledge Replay (#7)** | Shadow Execution implementado | P2 | Depende de sandbox isolado pra re-execução segura. |
| 6 | **Experiment-Based Evolution (#10)** | Shadow Execution + Knowledge Replay | P2 | Agente como cientista: hipótese → experimento → aprendizado. |

### Detalhes

**Item 1 — Memória semântica do Kernel**
- Objetivo: Buscar nos 103+ arquivos de memória por similaridade de significado
- Infraestrutura: Já existe (internal/vector/, internal/embeddings/, internal/search/)
- Risco: Dependência circular (Kernel usa Cosca pra indexar a si mesmo)
- Decisão do chef: "Fica no radar, revisita depois da v2.0 estável" (2026-07-26)

**Item 2 — Gatilho semântico de checkpoint**
- Objetivo: Quando contexto da sessão atinge ~70%, analisar e extrair automaticamente apenas o essencial (decisões, estado atual) antes do truncamento
- Funciona como: Firewall/airbag — não depende de chamada manual
- Complementa: Comando `cosca session checkpoint` (manual)
- Decisão do chef: "Coloca no radar, implementa depois da v2.0" (2026-07-26)

**Item 3 — Sessão semântica (busca)**
- Objetivo: Buscar sessões antigas por significado
- Baixa prioridade: Memória já contém o destilado de cada sessão
- Decisão do chef: "Radar, prioridade zero agora" (2026-07-26)
