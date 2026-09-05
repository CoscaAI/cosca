# cosca-product — Product Audit & Activation Report

> **Agente**: cosca-product (Product Chief)  
> **Data**: 2026-07-28  
> **Versão**: 1.4.0-dev  
> **Missão**: Auditoria de produto na ativação Onda 6

---

## 1. README & Docs Scan

### Proposta de Valor

A frase de abertura — *"Cosca is not a tool. It is an organization"* — é memorável e diferencia o produto, mas tem riscos:

| Aspecto | Avaliação |
|---------|-----------|
| **Clareza** | 🟡 Média. A metáfora da máfia é forte mas pode confundir usuários empresariais. O README compensa com dados concretos (53 agents, 71 skills, etc.) mas a proposta de valor central ainda não está cristalizada em uma frase de elevador. |
| **Público-alvo** | 🟢 Claro. Desenvolvedores e engenheiros que precisam de orquestração de IA. |
| **Diferenciação** | 🟢 Forte. Auto-evolution memory, 55 agentes especializados, architecture hierarchical. |
| **Quick Start** | 🟡 Funcional mas incompleto. Mostra `make build` e `cosca serve` mas **não mostra um agente rodando**. Usuário termina o Quick Start sem ter visto o valor principal do produto. |
| **CHANGELOG** | 🟢 Excelente. Extremamente detalhado, mostra evolução rápida (v1.0.0-rc.1 → v1.4.0-dev em 4 dias). |
| **Docs estrutura** | 🟢 48 arquivos de documentação, 7 ADRs, guias dedicados. |

### Documentação Faltante (Confirmada pelo Backlog)

- ❌ **CONTRIBUTING.md** — PLAT-001 (P0, 2h)
- ❌ **ARCHITECTURE.md** — PLAT-002 (P0, 3h)
- ❌ **CODEOWNERS** — PLAT-003 (P0, 1h)
- ❌ **Issue/PR templates** — PLAT-004 (P0, 1h)
- ❌ **Devcontainer** — PLAT-005 (P0, 3h)

---

## 2. Feature Inventory

### Produtos/Subprodutos da Cosca

| Produto | Descrição | Maturidade | Usuário-alvo |
|---------|-----------|-----------|--------------|
| **CLI (`cosca`)** | 37 comandos raiz (123 total), gerenciamento de agentes, knowledge search, memória, pipelines, chat | 🟢 Produção | Desenvolvedores |
| **Web Console** | Next.js 15, 27 módulos, 21+ páginas, autenticação JWT, PWA, Design System | 🟢 Produção | Desenvolvedores + Admins |
| **REST API** | 52 endpoints, 16 domínios, OpenAPI 3.0, porta 14120 | 🟢 Produção | Integradores |
| **Agentes Cosca** | 55 agentes especializados com memória semântica auto-evolutiva | 🟢 Produção | Usuários finais |
| **Orchestration Engine** | Roteamento semântico, DAG multi-agente, execução de ferramentas, MAG | 🟢 Produção | Desenvolvedores |
| **Knowledge Engine** | Busca híbrida FTS5 + vector, grafo de conhecimento, facets | 🟢 Produção | Todos |
| **Memory Engine** | 5 camadas, 7 tipos, evolução automática, poda TTL | 🟢 Produção | Todos |
| **Plugin System** | 4 runtimes (Go, WASM, External, SharedLib), sandbox, permissões | 🟡 Beta | Desenvolvedores |
| **TypeScript SDK** | `@cosca/sdk` — cliente API, tipos OpenAPI | 🟡 Beta (axios→fetch pendente) | Desenvolvedores web |
| **Go SDK** | `pkg/cosca/` — cliente Go | 🔴 MVP inicial | Desenvolvedores Go |
| **gRPC Server** | Proto definido, servidor pendente | 🔴 Stub | Integradores |
| **Editor Adapters** | 10 adaptadores (VS Code, Cursor, Claude, Neovim, etc.) | 🟢 Produção | Editores |
| **AI Providers** | 11 integrados (OpenAI, Anthropic, Google, DeepSeek, etc.) | 🟢 Produção | Todos |
| **Pipelines/Workflows** | 28 workflows, pipelines DAG configuráveis | 🟢 Produção | Desenvolvedores |

### Resumo por Tipo de Produto

```
Produtos Entregues (🟢):   8 — CLI, Web Console, REST API, Agents, Engines, Providers, Editor Adapters, Pipelines
Produtos Beta (🟡):        3 — Plugin System, TypeScript SDK, gRPC proto
Produtos MVP/Stub (🔴):    2 — Go SDK, gRPC server
```

---

## 3. User Journey

### Fluxo Atual (Clone → Rodar Agente)

```
┌─────────────────┐
│ 1. README        │  ← Lê que Cosca é "not a tool, an organization"
│ 2. git clone     │  ← Precisa ter Go 1.25+ instalado
│ 3. make build    │  ← Compila (~30s-2min dependendo da máquina)
│ 4. ./bin/cosca   │  ← Vê 37 comandos... por onde começar?
│ 5. cosca init    │  ← Cria .cosca/ com contexto
│ 6. cosca serve   │  ← Sobe servidor (mas sem agente configurado)
│ 7. ???           │  ← Precisa configurar provider de IA (API key)
│ 8. cosca agent run| ← Finalmente roda um agente
└─────────────────┘
```

### Problemas Identificados

| Passo | Problema | Impacto |
|-------|----------|---------|
| **2** | Precisa compilar do source — sem binário pré-compilado, sem `docker pull` | ❌ Alto — barreira de entrada |
| **3** | Go 1.25+ é recente, nem todos têm | 🟡 Médio |
| **4-5** | Sem tutorial interativo, sem "exemplo hello world" | ❌ Alto — abandono aqui |
| **6** | `cosca serve` não mostra o valor real do produto | 🟡 Médio |
| **7** | **Gap crítico**: usuário precisa de API key de LLM para fazer QUALQUER coisa útil. Sem modo demo/offline. | ❌❌ Crítico — bloqueante |
| **8** | Não há exemplo canônico documentado no README | ❌ Alto |

### O Que Deveria Ser

```
┌──────────────────────────────┐
│ 1. docker run cosca/cosca    │  ← 1 comando, sem instalação
│ 2. cosca demo                │  ← Modo demo com Ollama local ou simulação
│ 3. cosca agent run hello     │  ← "Hello World" de agentes
│      "Explain this project"  │
│ 4. cosca serve --open        │  ← Web Console com dados reais
└──────────────────────────────┘
```

---

## 4. Gap Analysis

### "5-minute to value" Gaps

| # | Gap | Severidade | Esforço | Item no Backlog? |
|---|-----|-----------|---------|-----------------|
| 1 | **Sem modo demo/zero-config** — usuário precisa de API key externa para testar | 🔴 Crítico | Média (4-8h) | ❌ Não identificado |
| 2 | **Sem binário pré-compilado** — `make build` é o único caminho | 🔴 Crítico | Baixo (1h) | ❌ Parcial (CI-002 CD stage) |
| 3 | **Sem tutorial 5-min** — README não mostra agente rodando | 🟠 Alto | Baixo (2h) | ❌ Não identificado |
| 4 | **Sem `make setup` one-command** — precisa instalar Go, Node, pnpm manualmente | 🟠 Alto | Baixo (3h) | ✅ PLAT-006 (P1) |
| 5 | **Sem glossário de termos** — agent vs skill vs workflow vs pipeline vs engine | 🟡 Médio | Baixo (1h) | ❌ Não identificado |
| 6 | **Terminologia da máfia pode ser polarizante** — "Don", "capos", "soldiers" em contexto enterprise | 🟡 Médio | Baixo (1h) | ❌ Não identificado |
| 7 | **Zero-result onboarding** — `cosca init` cria diretório mas não gera saída visível | 🟡 Médio | Baixo (1h) | ❌ Não identificado |
| 8 | **Sem devcontainer** — ambiente reproduzível para contribuidores | 🟡 Médio | Baixo (3h) | ✅ PLAT-005 (P0) |
| 9 | **Sem CONTRIBUTING.md** — como contribuir não está documentado | 🟡 Médio | Baixo (2h) | ✅ PLAT-001 (P0) |
| 10 | **Sem ARCHITECTURE.md** — visão geral da arquitetura para novos devs | 🟡 Médio | Baixo (3h) | ✅ PLAT-002 (P0) |

### Matriz de Priorização (Impacto × Esforço)

```
          ALTO     │ Gap 1 (demo)         │ Gap 2 (binário)    │ Gap 3 (tutorial)
                   │ Gap 4 (make setup)   │                     │
                   │                       │                     │
Impacto   MÉDIO    │ Gap 5 (glossário)    │ Gap 7 (zero-result) │ Gap 10 (ARCHITECTURE)
                   │ Gap 6 (terminologia) │ Gap 8 (devcontainer)│
                   │                       │                     │
          BAIXO    │ Gap 9 (CONTRIBUTING)  │                     │
                   ├───────────────────────┼─────────────────────┼─────────────────────
                   │       BAIXO           │      MÉDIO          │        ALTO
                   │                       │                     │
                   │               Esforço
```

---

## 5. Recommendations (Top 3)

### 🔴 R1: Zero-Config Demo Mode (P0 — Crítico)

**Problema**: Usuário precisa de API key externa para ver valor. Cosca morre na primeira impressão.

**Solução**: Adicionar `cosca demo` ou `cosca run --demo` que:
1. Detecta Ollama local (se instalado) — 0 config
2. Se não, usa um modo simulado com respostas pré-definidas para demonstração
3. Executa `cosca agent run cosca-demo "Explain this project"` com output real
4. Mostra métricas, memória, evolução

**Critério de sucesso**: `docker run cosca/cosca demo` → 5 segundos → agente rodando. 0 configuração.

**Esforço estimado**: 4-8h (detecção Ollama + modo simulado + comando demo)

---

### 🟠 R2: "5-Minute Tutorial" no README (P0 — Alto)

**Problema**: README Quick Start termina em `cosca serve` — o usuário nunca vê o valor principal do produto.

**Solução**: Substituir o Quick Start atual por um tutorial que, em 10 passos, leva o usuário do zero a rodar um agente:
```bash
# 1. Build
make build

# 2. Init
./bin/cosca init

# 3. Run an agent (demo mode)
./bin/cosca agent run cosca-demo "Analyze this project"

# 4. See what happened
./bin/cosca memory list
./bin/cosca knowledge stats

# 5. Start the web UI
./bin/cosca serve --open
```

**Critério de sucesso**: Novo usuário consegue rodar um agente em <5 minutos seguindo o README.

**Esforço estimado**: 2h (edição README + criar agente cosca-demo)

---

### 🟡 R3: Pre-Built Distribution Pipeline (P1 — Alto)

**Problema**: Único caminho é `make build`. Sem binário pré-compilado, sem `brew install`, sem Docker Hub.

**Solução**:
1. Configurar GoReleaser para publicar assets na GitHub Releases (já configurado parcialmente)
2. Adicionar `docker push` no CI (CI-002, já no backlog)
3. Adicionar instrução `docker run ghcr.io/coscaai/cosca:latest` no README
4. Adicionar badge de "latest release" + "docker pulls"

**Critério de sucesso**: `docker run cosca/cosca --help` funciona sem qualquer instalação.

**Esforço estimado**: 4h (CI-002 já contempla parte disso)

---

## 6. Métricas de Produto Propostas

| Métrica | Baseline | Target 30 dias | Target 90 dias |
|---------|----------|---------------|----------------|
| **Time-to-Value** (clone → agente rodando) | ~15 min | <5 min | <2 min |
| **Onboarding completion rate** | Desconhecido | >50% | >80% |
| **Docker pulls / semana** | 0 | >100 | >1000 |
| **GitHub stars** | ~0 | >50 | >200 |
| **Issues abertas por novos usuários** | 0 | >5 | >20 |
| **Pre-built downloads / semana** | 0 | >50 | >500 |

---

## 7. Alinhamento com o Backlog Existente

O [Product Backlog Consolidado](../agent/cosca-product/product-backlog-2026-07-28.md) já identifica corretamente o **EPIC-C (Developer Experience & Community Readiness)** como prioridade. Este relatório adiciona 3 recomendações que **complementam** o EPIC-C:

| Recomendação | Novo? | Relação com Backlog |
|-------------|-------|---------------------|
| R1: Zero-config demo | **Novo** | Não está no backlog. Deve ser adicionado como P0 no EPIC-C. |
| R2: 5-minute tutorial | **Novo** | Não está no backlog. Deve ser adicionado como P0 no EPIC-C. |
| R3: Pre-built distribution | Parcial | CI-002 (CD stage) já está no backlog como P0. Este R3 adiciona a perspectiva de produto (Docker Hub, brew, UX). |

**Ação**: Adicionar R1 e R2 ao backlog como P0 itens no EPIC-C.

---

## 8. Conclusão

**Pontos fortes**: Cosca tem um produto tecnicamente impressionante — 55 agentes, 71 skills, memória auto-evolutiva, 52 endpoints, web console completo. A engenharia está à frente do produto.

**Gap principal**: **"Ativação"** — o usuário investe 15+ minutos e esforço (instalar Go, compilar, configurar API key) antes de ver qualquer valor. A Cosca precisa de um caminho de zero-config para a primeira execução.

**Recomendação central**: Antes de adicionar mais features, invista 10-16h nas 3 recomendações acima para reduzir o time-to-value de 15min para <2min. Um produto que ninguém consegue testar não importa quantos agentes tem.

---

*Relatório gerado por cosca-product — Activation Audit, 2026-07-28*
