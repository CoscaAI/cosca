# FEDERATION ENGINE — Cross-project Knowledge Transfer (F2.3)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
> **Código**: F2.3 | **Bloco Cognitivo**: Bloco 1 — Memória & Conhecimento | **Fase CMI**: Fase 2
> **Dependências**: F7.2 Trust Registry → `memory/trust/TRUST_REGISTRY.md` | F9.1 Experience Compiler → `engines/experience-compiler/SKILL.md` | Semantic Memory Engine → `engines/semantic-memory/SKILL.md`
>
> Consulte também:
> - [KNOWLEDGE-FEDERATION SKILL.md](../knowledge-federation/SKILL.md) — especificação completa da federação semântica (Hub, assinaturas, dedup, 3 mecanismos de transferência). Este engine é a **camada operacional leve** focada no modelo publish/subscribe de princípios compilados.
> - [EXPERIENCE COMPILER](../experience-compiler/SKILL.md) — F9.1: destilação de learnings brutos em padrões e princípios (pré-requisito para publicação global)
> - [TRUST REGISTRY](../../memory/trust/TRUST_REGISTRY.md) — F7.2: reputação histórica de agentes (fonte de autoridade da atribuição)
> - [SEMANTIC MEMORY](../semantic-memory/SKILL.md) — indexação vetorial local (consulta local antes da consulta global)
> - [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) — formato de learning entries (o que NÃO pode ser publicado global)

---

## ÍNDICE

1. [Definição](#1-definição)
2. [Arquitetura](#2-arquitetura)
3. [Mecanismo Publish / Subscribe](#3-mecanismo-publish--subscribe)
4. [Pipeline de Publicação (5 Passos)](#4-pipeline-de-publicação-5-passos)
5. [Consulta Global (Subscribe)](#5-consulta-global-subscribe)
6. [Segurança e Privacidade](#6-segurança-e-privacidade)
7. [Exemplo Real](#7-exemplo-real)
8. [Performance](#8-performance)
9. [CLI e Automação](#9-cli-e-automação)
10. [Métricas do Engine](#10-métricas-do-engine)
11. [Casos de Borda e Anti-Padrões](#11-casos-de-borda-e-anti-padrões)

---

## 1. DEFINIÇÃO

O **Federation Engine** implementa a **transferência de conhecimento cross-project**: um princípio compilado no projeto A pode ser consultado e reutilizado pelo projeto B quando relevante.

A ideia central é uma **federação semântica entre projetos**: conhecimento aprendido com dor (e validado por evidência) em UM projeto não precisa ser redescoberto em OUTROS projetos.

### 1.1 Escopo — o que esta engine faz e o que não faz

| Fator | Incluído (F2.3 — este engine) | Excluído (delegado) |
|-------|------------------------------|---------------------|
| **Tipo de conhecimento** | Apenas **princípios compilados** pelo F9.1 | Learnings brutos, heurísticas não validadas, failures, DDNAs |
| **Mecanismo** | Publish/Subscribe com store global (`~/.config/cosca/global-knowledge/`) | Matching vetorial completo, assinaturas KSIG, dedup complexo |
| **Controle** | Don decide TODA publicação (nada automático) | Publicação automática, auto-propagação |
| **Fonte de verdade** | `PRINCIPLES.md` (output F9.1) | `learnings.md`, `failures.md`, `patterns/` |
| **Consulta** | Busca semântica no store global (extensão do Semantic Memory) | Hub remoto, federação multi-máquina |

### 1.2 Relação com o Knowledge Federation Engine

O `knowledge-federation/SKILL.md` (semantic-memory) especifica a arquitetura completa de federação com Federation Hub, Knowledge Signatures, 3 mecanismos de transferência (Pattern Matching, Failure Avoidance, Heuristic Transfer) e deduplicação.

Este engine **não substitui** aquela especificação — é a **camada operacional mínima** que torna F2.3 funcional hoje:

```
┌─────────────────────────────────────────────────────────────┐
│  ECOSSISTEMA DE FEDERAÇÃO CROSS-PROJECT                      │
│                                                              │
│  ┌──────────────────────┐   ┌────────────────────────────┐  │
│  │ FEDERATION ENGINE     │   │ KNOWLEDGE FEDERATION       │  │
│  │ (F2.3 — este arquivo) │   │ (spec completa, existente)  │  │
│  │ ───────────────────   │   │ ─────────────────────────   │  │
│  │ Publica PRINCÍPIOS    │   │ Patterns + Failures +      │  │
│  │ compilados (F9.1)     │   │ Heurísticas (assinaturas)  │  │
│  │ Store: global-knowledge│   │ Hub: $HOME/.cosca/federation│ │
│  │ Don-controlled        │   │ 3 mecanismos de matching   │  │
│  │ < 200ms de overhead   │   │ Embeddings + dedup         │  │
│  └──────────────────────┘   └────────────────────────────┘  │
│              │                            │                  │
│              └──────────┬─────────────────┘                  │
│                         ▼                                    │
│            ┌───────────────────────────┐                    │
│            │ SEMANTIC MEMORY ENGINE    │  índice vetorial   │
│            │ (local + federado)        │  local estendido   │
│            └───────────────────────────┘                    │
└─────────────────────────────────────────────────────────────┘
```

**Regra de coexistência**: este engine é o "portão" (gate) que decide O QUE pode deixar o projeto; o Knowledge Federation Engine é o "transporte" que distribui. Nada entra no Hub sem antes passar por este portão.

---

## 2. ARQUITETURA

```
┌──────────────────────────────────────────────────────────────────┐
│                  FEDERATION — ARQUITETURA                          │
│                                                                    │
│  ┌────────────────────────────┐                                   │
│  │   PROJETO A (cosca-test)    │                                   │
│  │  ────────────────────────  │                                   │
│  │  internal/embed/cosca/           │                                   │
│  │  ├── knowledge/             │                                   │
│  │  │   └── PRINCIPLES.md  ◄───┼─── F9.1 compila (semanal)        │
│  │  │                          │                                   │
│  │  │   (opcional: publish)    │                                   │
│  │  │   Don marca #global-ready│                                   │
│  │  └── engines/federation/    │                                   │
│  │      └── SKILL.md (engine)  │                                   │
│  └───────────┬────────────────┘                                   │
│              │  publish: copia princípio compilado                │
│              ▼                                                     │
│  ┌──────────────────────────────────────────┐                     │
│  │   MEMORY_GLOBAL                          │                     │
│  │   ~/.config/cosca/global-knowledge/      │                     │
│  │  ──────────────────────────────────────  │                     │
│  │   ├── principles/                        │  princípios .md     │
│  │   │   └── P-{id}-{slug}.md               │                     │
│  │   ├── index.yaml                         │  índice FTS + tags  │
│  │   ├── provenance.jsonl                   │  trilha de origem   │
│  │   └── config.yaml                        │  namespaces, don    │
│  └───────────┬──────────────────────────────┘                     │
│              │  subscribe: consulta semântica                     │
│              ▼                                                     │
│  ┌────────────────────────────┐                                   │
│  │   PROJETO B (cosca-web)     │                                   │
│  │  ────────────────────────  │                                   │
│  │  internal/embed/cosca/           │                                   │
│  │  ├── knowledge/             │                                   │
│  │  │   └── (consulta global   │                                   │
│  │  │       quando não acha    │                                   │
│  │  │       resultado local)   │                                   │
│  │  └── engines/semantic-memory│                                   │
│  │      └── (index estendido   │                                   │
│  │          com source=global) │                                   │
│  └────────────────────────────┘                                   │
│                                                                    │
│  ATRIBUIÇÃO: "aprendido em cosca-test por cosca-architecture"     │
│  ──────────────────────────────────────────────────────────────────┘
```

### 2.1 Componentes

| Componente | Localização | Responsabilidade |
|------------|-------------|------------------|
| **Publicador** (`FederationPublisher`) | `engines/federation/` (por projeto) | Detecta princípios `#global-ready`, copia para MEMORY_GLOBAL, registra proveniência |
| **MEMORY_GLOBAL** | `~/.config/cosca/global-knowledge/` | Store central project-agnostic. Única fonte de conhecimento federado |
| **Assinante** (`FederationSubscriber`) | Integrado ao Semantic Memory | Consulta o store global quando a busca local não retorna resultados suficientes |
| **Índice Global** | `global-knowledge/index.yaml` + FTS5 | Indexação semântica do store global para consulta < 200ms |

### 2.2 Estrutura do MEMORY_GLOBAL

```
~/.config/cosca/global-knowledge/
├── config.yaml              # Namespaces, Don policy, thresholds
├── index.yaml               # Índice semântico (tags, domínios, embeddings refs)
├── provenance.jsonl         # Trilha de origem: quem publicou, quando, de onde
├── principles/              # Princípios compilados publicados
│   ├── P-2026-0001-dead-code-deps.md
│   ├── P-2026-0002-race-flag-ci.md
│   └── ...
└── versions/                # Histórico de versões (supersedes)
    └── P-2026-0001/         # v1, v2, ...
```

---

## 3. MECANISMO PUBLISH / SUBSCRIBE

### 3.1 Publish

**Precondições (TODAS obrigatórias)**:

```
1. ✅ Princípio compilado pelo F9.1 (registrado em PRINCIPLES.md do projeto de origem)
2. ✅ Maturity ≥ 0.7 (threshold de princípio do F9.1)
3. ✅ Freshness > 0.5 (validado pelo F1.4 Wisdom Decay)
4. ✅ Don marca o princípio com a tag #global-ready (ação EXPLÍCITA e MANUAL)
5. ✅ Autoridade da atribuição confirmada no TRUST_REGISTRY (agente com reliability ≥ 0.70)
```

**O que NUNCA é publicado** (ver §6 — Segurança):

- Learnings brutos (`learnings.md`) — independente de qualidade
- Padrões com maturity < 0.7 (que ainda estão em `knowledge/patterns/`)
- Failures, DDNAs, decisões específicas do projeto
- Qualquer conteúdo que o Don não marcou explicitamente

### 3.2 Store

Quando um princípio é marcado `#global-ready`:

1. O Publicador lê o princípio de `PRINCIPLES.md`
2. Gera o arquivo `principles/P-{id}-{slug}.md` com frontmatter canônico (ver §4.3)
3. Registra em `provenance.jsonl`: `{project, agent, timestamp, commit_hash, principle_id}`
4. Atualiza `index.yaml` (tags, domínio, embedding ref)
5. NUNCA altera o princípio original — o store global é cópia imutável em versão

### 3.3 Subscribe

O Assinante (via Semantic Memory) consulta o store global **somente quando**:

```
1. Busca local retorna 0 resultados relevantes (relevance < 0.30)
   OU
2. Busca local retorna resultados, mas o top-1 tem score < threshold_configurável
   (default: < 0.50)
```

O fluxo de consulta:

```
consulta_semantica(query):
    resultados_locais = semantic_memory.search(query)       # < 100ms
    if resultados_locais.relevance_max >= 0.50:
        return resultados_locais                            # local basta
    else:
        resultados_globais = federation.search(query)       # < 200ms
        return merge(resultados_locais, resultados_globais) # globais marcados 🏷️
```

Resultados globais são **sempre marcados** com:

- 🏷️ `source: global`
- 🏷️ `origin_project: cosca-test`
- 🏷️ `learned_by: cosca-architecture`
- 🏷️ `validated_in: [cosca-test, ...]`

### 3.4 Privacy — Controle do Don

| Regra | Descrição |
|-------|-----------|
| **Nada automático** | Nenhum agente publica sem ordem do Don. A tag `#global-ready` só pode ser adicionada pelo Don (ou pelo Kernel sob ordem explícita do Don) |
| **Revogação** | Don pode revogar a qualquer momento: `cosca federation unpublish P-2026-0001` → princípio removido do index (o arquivo original no projeto permanece intacto) |
| **Namespace** | `config.yaml` define namespaces (ex: `personal`, `acme-corp`). Projetos só consultam o namespace que o Don autorizou |
| **Auditoria** | Toda publicação/revogação gera entrada em `provenance.jsonl` — nada é apagado do log |

---

## 4. PIPELINE DE PUBLICAÇÃO (5 PASSOS)

```
┌──────────────────────────────────────────────────────────────────────┐
│            PIPELINE DE PUBLICAÇÃO GLOBAL — 5 PASSOS                   │
│                                                                       │
│  PASSO 1 — COMPILAR (F9.1)                                            │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │ Experience Compiler destila learnings → padrões → princípios │     │
│  │ Output: PRINCIPLES.md (maturity ≥ 0.7, freshness > 0.5)      │     │
│  └───────────────────────────────┬─────────────────────────────┘     │
│                                  ▼                                    │
│  PASSO 2 — MARCAR (DON)                                               │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │ Don revisa o princípio e decide: "este é global"             │     │
│  │ → adiciona tag #global-ready no frontmatter do princípio     │     │
│  │ → NENHUM agente pode fazer isso por conta própria            │     │
│  └───────────────────────────────┬─────────────────────────────┘     │
│                                  ▼                                    │
│  PASSO 3 — PUBLICAR (ENGINE)                                          │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │ Federation Engine copia o princípio para MEMORY_GLOBAL       │     │
│  │ → principles/P-{id}-{slug}.md (frontmatter canônico)         │     │
│  │ → provenance.jsonl: {origem, agente, timestamp, commit}      │     │
│  │ → index.yaml atualizado                                      │     │
│  └───────────────────────────────┬─────────────────────────────┘     │
│                                  ▼                                    │
│  PASSO 4 — CONSULTAR (OUTRO PROJETO)                                  │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │ Projeto B faz consulta semântica (local-first, §3.3)         │     │
│  │ → se local insuficiente, consulta MEMORY_GLOBAL              │     │
│  │ → encontra o princípio global com score ≥ 0.60               │     │
│  └───────────────────────────────┬─────────────────────────────┘     │
│                                  ▼                                    │
│  PASSO 5 — ATRIBUIR                                                  │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │ Todo uso registra atribuição:                                │     │
│  │ "princípio aprendido em cosca-test por cosca-architecture"   │     │
│  │ → proveniência na resposta (nunca se apresenta como próprio) │     │
│  │ → sucesso/falha retroalimenta validated_in do princípio      │     │
│  └─────────────────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────────────────┘
```

### 4.1 Ponto de Início — Como um princípio fica `#global-ready`

Exemplo de frontmatter em `PRINCIPLES.md` do projeto de origem (após Don marcar):

```yaml
---
id: P-2026-0001
title: "Dead Code Removal Requer go list -deps"
status: principle
maturity: 0.759
freshness: 0.87
agents:
  - cosca-kernel
  - cosca-architecture
  - cosca-performance
global-ready: true        # ← MARCAÇÃO DO DON (passo 2)
global-since: 2026-07-30
namespace: personal
---
```

### 4.2 Condições de Publicação (checklist do Publicador)

```
PUBLISH_CHECKLIST:
  [ ] frontmatter contém global-ready: true
  [ ] status == principle (NÃO pattern, NÃO learning)
  [ ] maturity >= 0.7
  [ ] freshness > 0.5
  [ ] agents[].reliability >= 0.70 (TRUST_REGISTRY)
  [ ] namespace autorizado no config.yaml do MEMORY_GLOBAL
  [ ] princípio NÃO contradiz CONSTITUIÇÃO (Gate 4 do F2.3 já validou)
SE TODOS ✅ → publish()
SE QUALQUER ❌ → rejeitar com log (motivo registrado em provenance.jsonl)
```

### 4.3 Formato Canônico no MEMORY_GLOBAL

```yaml
# principles/P-2026-0001-dead-code-deps.md
---
id: P-2026-0001
slug: dead-code-deps
title: "Dead Code Removal Requer go list -deps"
status: active                    # active | superseded | revoked
version: 1
namespace: personal
domain: architecture
tags:
  - go
  - dead-code
  - dependencies
  - refactoring
origin:
  project: cosca-test
  project_id: <uuid>
  agent: cosca-architecture        # agente que compilou
  compiled_by: experience-compiler # F9.1
  marked_by: don                  # quem autorizou global
  timestamp: 2026-07-30T14:00:00Z
  commit_hash: 90e864f
maturity: 0.759
freshness: 0.87
validated_in:
  - project: cosca-test
    count: 3
    last: 2026-07-30
    success_rate: 1.0
---

# DEAD CODE REMOVAL REQUER go list -deps

**Regra:** Nenhum bloco de código pode ser removido como "dead code" sem
verificação de grafo de dependências via `go list -deps`.

**Aplicação prática:**
- Executar `go list -f '{{join .Deps}}' ./cmd/<target>/ | grep <pkg>` antes de remover
- `go build` NÃO prova que código não é usado
- Classificar: Tipo A (inatingível → remove), Tipo B (edge → testa), Tipo C (defensivo → documenta)

**Aprendido em:** cosca-test · **Compilado por:** cosca-architecture (F9.1)
```

---

## 5. CONSULTA GLOBAL (SUBSCRIBE)

### 5.1 Regras de Consulta

```
SEARCH_GLOBAL(query, topK=3, namespace=default):
  1. Verificar config.yaml do MEMORY_GLOBAL (namespace autorizado?)
  2. Embed query (mesmo modelo do Semantic Memory)
  3. Cosine similarity contra index.yaml (embeddings dos princípios globais)
  4. Ranking: similarity × freshness × authority
     authority = 1.0 (validated_in ≥ 3) | 0.8 (1-2) | 0.6 (0 validações)
  5. Retornar top-K com proveniência completa
  ⏱ Alvo: < 200ms adicional sobre a consulta local
```

### 5.2 Exemplo de Resposta (Projeto B consultando)

```
Consulta: "como remover código morto em Go com segurança"

Resultado local: 0 resultados com relevance ≥ 0.30
→ fallback para MEMORY_GLOBAL

🏷️ [GLOBAL] P-2026-0001 — Dead Code Removal Requer go list -deps
   relevance: 0.83 | validated: 3× em cosca-test (100% sucesso)
   learned_by: cosca-architecture | origin: cosca-test
   aplicação: executar go list -deps antes de qualquer remoção
   ⚠️ atenuação cross-stack: verificar ferramenta equivalente se não-Go
```

### 5.3 Retroalimentação (Feedback Loop)

Quando o Projeto B **aplica** o princípio global:

| Outcome | Ação |
|---------|------|
| Sucesso | `validated_in` ganha entrada (project B, count +1, success) |
| Falha | `validated_in` ganha entrada (project B, count +1, failure) → `success_rate` recalculado |
| Contradição | Princípio marcado como `contested` → notifica Don do projeto de origem |

A retroalimentação alimenta a **Cognitive Gravity (F2.4)** — cada projeto que valida o conhecimento incrementa `cross_project_count` (+20 por projeto, o ganho mais valioso de gravidade).

---

## 6. SEGURANÇA E PRIVACIDADE

### 6.1 Regra de Ouro

> **Apenas princípios compilados pelo F9.1 podem ser globais. Learnings brutos NÃO.**

| Conhecimento | Pode ser global? | Por quê |
|--------------|:----------------:|---------|
| Princípio compilado (F9.1, maturity ≥ 0.7) | ✅ | Destilado de múltiplas evidências, acionável, validado |
| Padrão (F9.1, maturity 0.4–0.7) | ❌ | Ainda não consolidado — evidência insuficiente |
| Learning bruto | ❌ | Ruidoso, específico do contexto, sem validação cross-agent |
| Failure | ❌ | Contém contexto sensível do projeto de origem |
| Decisão (DDNA) | ❌ | Específica do projeto, pode conter lógica proprietária |
| Secrets / credenciais | ❌ | Nunca, em nenhuma hipótese (regex de bloqueio no Publicador) |

### 6.2 Salvaguardas

```
🛡️ GATE DE COMPILAÇÃO: nada entra no MEMORY_GLOBAL sem passar pelo F9.1.
   O Publicador REJEITA qualquer entrada sem `compiled_by: experience-compiler`.

🛡️ GATE DO DON: a tag #global-ready só pode ser escrita pelo Don.
   O Publicador verifica a autorização (marca d'água de autorização no config).

🛡️ NUNCA código proprietário: o store global contém APENAS princípios
   (regras abstratas), nunca código-fonte, lógica de negócio ou arquitetura interna.

🛡️ Namespace isolation: projetos consultam apenas namespaces autorizados.

🛡️ Log imutável: provenance.jsonl é append-only. Revogação nunca apaga o log.

🛡️ Atribuição obrigatória: nenhum resultado global é apresentado sem
   "aprendido em {project} por {agent}". Reuso sem atribuição é violação.
```

### 6.3 Controle de Acesso (arquivo `config.yaml` do MEMORY_GLOBAL)

```yaml
# ~/.config/cosca/global-knowledge/config.yaml
namespace: personal
owner: don
subscribers:                          # projetos autorizados a consultar
  - cosca-test
  - cosca-web
  - cosca-cli
publish_policy:
  require_global_ready_tag: true     # sempre
  require_maturity_min: 0.7
  require_reliability_min: 0.70      # F7.2 TRUST_REGISTRY
  allow_agents: [don, kernel]        # quem pode disparar publish
blocklist:
  - "sk-"
  - "api_key"
  - "token"
  - "password"
  - "secret"
```

---

## 7. EXEMPLO REAL

### 7.1 Cenário: Princípio "Dead Code Removal Requer `go list -deps`"

**Projeto A (cosca-test)** — pipeline de publicação:

```
1. COMPILAR (F9.1):
   → 5 learnings sobre dead code (L20, L22, L35, L52, L71)
   → maturity 0.759 → PRINCIPLE registrado em PRINCIPLES.md
   → "Dead Code Removal Requer go list -deps"

2. MARCAR (Don):
   → Don revisa o princípio no relatório semanal do F9.1
   → Don adiciona #global-ready: true no frontmatter
   → Don: "qualquer projeto Go que remover código morto precisa disso"

3. PUBLICAR (Engine):
   → Federation Engine copia para ~/.config/cosca/global-knowledge/
   → principles/P-2026-0001-dead-code-deps.md criado
   → provenance.jsonl: {origin: cosca-test, agent: cosca-architecture,
                        timestamp: 2026-07-30T14:00:00Z}
   → index.yaml atualizado com embedding + tags
```

**Projeto B (cosca-web)** — consulta:

```
4. CONSULTAR:
   → Task: "Remover 2.000 linhas de handlers não utilizados"
   → Busca local: 0 resultados com relevance ≥ 0.30
   → Fallback MEMORY_GLOBAL: query "dead code removal go"
   → Encontra P-2026-0001 com relevance 0.83

5. ATRIBUIR:
   → Agente do cosca-web executa `go list -deps` antes de remover
   → Evita remover 3 pacotes que ainda eram importados indiretamente
   → Resultado registrado: validated_in += cosca-web
   → Resposta ao Don do cosca-web:
     "Apliquei princípio global P-2026-0001 aprendido em cosca-test
      por cosca-architecture (3 validações anteriores). Preveniu
      quebra de build em 3 pacotes."
```

**Resultado**: o princípio foi validado em 2 projetos → `validated_in: 2`, `success_rate: 1.0`, e a Cognitive Gravity (F2.4) registrou +20 de `cross_project_count`.

### 7.2 Exemplo de REJEIÇÃO (segurança em ação)

```
Um agente do cosca-test tenta publicar o learning bruto:
  "L22: removi código morto usando grep"

PUBLISHER REJEITA:
  ✗ status = learning (não principle)
  ✗ maturity = N/A (não compilado)
  ✗ global-ready = ausente (Don não marcou)
  → registrado em provenance.jsonl como rejected_attempt
  → nenhuma cópia é feita
```

---

## 8. PERFORMANCE

### 8.1 Orçamento de Latência

| Operação | Orçamento | Observação |
|----------|:---------:|------------|
| Consulta local (Semantic Memory) | < 100ms | Existente, sem alteração |
| Consulta global (MEMORY_GLOBAL) | < 150ms | FTS5 + índice de tags + cache de embeddings |
| **Overhead adicional total** | **< 200ms** | Regra F2.3: a federação NUNCA pode dobrar o tempo de consulta |
| Publicação (publish) | < 500ms | Copy + index + provenance (assíncrono, não bloqueia task) |
| Sync de proveniência | < 1s | Append-only log, sem lock |

### 8.2 Estratégia de Cache

```
CACHE_STRATEGY:
  embeddings: LRU (1.000 entradas, TTL 300s)
  index.yaml: cache de 60s (re-parse raro)
  resultados_globais: cache por query hash, TTL 300s
  fallback: consulta global SÓ dispara quando local < threshold
  (evita consulta global desnecessária em tasks com memória local rica)
```

### 8.3 Métrica de Performance

```
fed_perf_query_ms = p95(consulta_global_duration)
fed_perf_overhead = p95(consulta_total - consulta_local)
fed_cache_hit_rate = cache_hits / total_consultas_globais

Alvos:
  fed_perf_query_ms < 150ms
  fed_perf_overhead < 200ms
  fed_cache_hit_rate > 0.70
```

---

## 9. CLI E AUTOMAÇÃO

### 9.1 Comandos

```bash
# === PUBLICAÇÃO ===
# Listar princípios candidatos (compilados, sem #global-ready ainda)
cosca federation candidates

# Publicar um princípio (requer marcação do Don no PRINCIPLES.md)
cosca federation publish P-2026-0001

# Revogar um princípio global (Don)
cosca federation unpublish P-2026-0001

# === CONSULTA ===
# Buscar princípios globais
cosca federation search "dead code removal go" --namespace default

# Ver princípio global com proveniência completa
cosca federation show P-2026-0001

# === ADMINISTRAÇÃO ===
# Status da federação (conexão, cache, últimas publicações)
cosca federation status

# Log de proveniência (append-only)
cosca federation log --origin cosca-test

# Validar princípio após reuso bem-sucedido
cosca federation validate P-2026-0001 --project cosca-web --success

# Configurar namespace/autorizações
cosca federation config --set namespace.default=acme-corp
```

### 9.2 Trigger de Publicação

A publicação NUNCA é automática. O gatilho é sempre humano:

```
GATILHO 1 (Don ativo):
  Don edita PRINCIPLES.md → adiciona #global-ready → roda
  cosca federation publish P-2026-0001

GATILHO 2 (Don via Kernel):
  Don instrui: "publique o princípio P-2026-0001"
  Kernel verifica autorização e executa publish

NUNCA:
  ✗ post-commit hook que publica
  ✗ Stage 7 do pipeline que publica
  ✗ Experiência Compiler que publica
```

### 9.3 Scheduler (não-publicação)

```bash
# Sync de índice e cache (a cada 6h) — NUNCA publica
0 */6 * * * cosca federation sync-index

# Verificação de proveniência pendente (apenas alerta)
0 9 * * * cosca federation status --alert-pending
```

---

## 10. MÉTRICAS DO ENGINE

### 10.1 Métricas Internas

| Métrica | Definição | Alvo |
|---------|-----------|:----:|
| **Principles Published** | Princípios globais ativos | > 3 após 1 mês |
| **Publish Rejection Rate** | Tentativas rejeitadas / total | < 20% |
| **Cross-Project Reuse** | Vezes que princípios globais foram aplicados por outros projetos | > 5/mês |
| **Success Rate Federado** | Média de success_rate dos princípios globais | > 0.85 |
| **Global Query p95** | Latência da consulta global | < 150ms |
| **Overhead Adicional** | Latência total - latência local | < 200ms |
| **Attribution Compliance** | % de usos com atribuição correta | 100% |
| **Zero Leakage** | Publicações rejeitadas de learnings brutos/secrets | 0 vazamentos |

### 10.2 Alertas

| Condição | Severidade | Ação |
|----------|------------|------|
| Overhead > 200ms | 🔴 ALTO | Otimizar cache/índice — regra de performance violada |
| Publish sem marcação do Don | 🔴 ALTO | Revisar permissões do Publicador (possível bypass) |
| Tentativa de publicar learning bruto | 🟡 MÉDIO | Registrar em provenance, revisar workflow do agente |
| Princípio global com success_rate < 0.5 | 🟡 MÉDIO | Sinalizar para o projeto de origem revalidar |
| 0 publicações em 60 dias | 🟢 INFO | Federação pode estar ociosa — verificar F9.1 rodando |

---

## 11. CASOS DE BORDA E ANTI-PADRÕES

### 11.1 Casos de Borda

| Caso | Comportamento |
|------|---------------|
| **Dois projetos publicam o mesmo princípio** | `index.yaml` usa hash de conteúdo → detecta duplicata → mantém o de maior maturity, registra ambos na proveniência |
| **Princípio global contradiz princípio local** | Resultado global é apresentado com warning; o princípio local prevalece (local-first). Conflito registrado para o Cognitive Immune System (F2.2) |
| **Projeto de origem é deletado** | MEMORY_GLOBAL é project-agnostic — o princípio permanece com proveniência intacta |
| **MEMORY_GLOBAL inacessível** | Projeto opera normalmente com memória local (offline-first). Consulta global é pulada silenciosamente |
| **Princípio global superseded** | `status: superseded` no frontmatter → removido do index de busca ativa, mantido em versions/ |
| **Don muda de ideia** | `unpublish` → princípio sai do índice, mas NUNCA é apagado do log |

### 11.2 Anti-Padrões

| Anti-Padrão | Por que evitar |
|-------------|----------------|
| **Publicar learning bruto** | Ruído, contexto específico, sem validação — polui o store global |
| **Publicação automática** | Remove o controle do Don — o bem mais importante da federação é a confiança |
| **Consultar global antes do local** | Dobra latência sem necessidade — local-first é regra de performance |
| **Apresentar conhecimento global como próprio** | Destrói a rastreabilidade (proveniência é inegociável) |
| **Store global versionado no git do projeto** | O MEMORY_GLOBAL NÃO pertence a nenhum projeto — deve ficar em `~/.config/cosca/` |
| **Publicar padrões com maturity < 0.7** | O princípio ainda não é consolidado — pode propagar erro |

### 11.3 Regras de Resiliência

```
1. F9.1 não rodou → não há candidatos → publish lista vazia (sem erro)
2. Don não marcou nada → nada é publicado (federação é passiva por design)
3. MEMORY_GLOBAL corrompido → rebuild do índice a partir de principles/ (nunca
   re-publica: source of truth é o PRINCIPLES.md do projeto de origem)
4. Consulta global falha → retorna apenas resultados locais (nunca bloqueia task)
5. Conflito de namespace → bloqueia consulta e alerta Don (nunca vaza entre namespaces)
```

---

## 12. CRITÉRIOS DE QUALIDADE (F2.3)

- [ ] Princípio compilado (F9.1, maturity ≥ 0.7) com `#global-ready` é publicado em `~/.config/cosca/global-knowledge/`
- [ ] Learning bruto NUNCA é publicado (teste adversarial: 100% de rejeição)
- [ ] Publicação ocorre apenas com marcação do Don (0 publicações automáticas)
- [ ] Projeto B consulta e recebe princípio global com atribuição completa ("aprendido em cosca-test por cosca-architecture")
- [ ] Overhead de consulta global < 200ms (p95)
- [ ] Local-first: consulta global só dispara quando local é insuficiente
- [ ] Proveniência rastreável: toda publicação tem entry em `provenance.jsonl`
- [ ] Reuso cross-project ≥ 2 projetos (ex: cosca-test → cosca-web)
- [ ] Atribuição 100% presente nos resultados globais
- [ ] Revogação pelo Don funciona sem apagar o log

---

## 13. DEPENDÊNCIAS

| Engine / Sistema | Propósito | Tipo |
|------------------|-----------|------|
| Experience Compiler (F9.1) | Compila learnings → princípios (pré-requisito do publish) | Forte |
| Trust Registry (F7.2) | Autoridade de atribuição (reliability ≥ 0.70) | Forte |
| Semantic Memory Engine | Consulta local + extensão para consulta federada | Forte |
| Wisdom Decay (F1.4) | Freshness dos princípios publicados | Média |
| Cognitive Gravity (F2.4) | cross_project_count alimentado pela retroalimentação | Média |
| Cognitive Immune System (F2.2) | Detecção de contradição entre princípios globais e locais | Média |
| Knowledge Federation (spec) | Hub/assinaturas/matching avançado (consumidor deste portão) | Fraca |

### Infraestrutura

| Componente | Propósito |
|------------|-----------|
| `~/.config/cosca/global-knowledge/` | Store global (project-agnostic) |
| `~/.config/cosca/global-knowledge/index.yaml` | Índice semântico |
| `~/.config/cosca/global-knowledge/provenance.jsonl` | Trilha de origem |
| Provider API (embeddings) | Indexação semântica dos princípios |

---

## HISTÓRICO

| Versão | Data | Autor | Mudanças |
|--------|------|-------|---------|
| 1.0.0 | 2026-07-30 | Architecture Chief | Criação inicial — modelo publish/subscribe de princípios compilados (F9.1), MEMORY_GLOBAL em `~/.config/cosca/global-knowledge/`, Don-controlled publishing, pipeline de 5 passos, segurança (apenas F9.1 compilado), exemplo do dead code removal, orçamento de performance < 200ms |

---

> *"Aprender em um projeto é bom. Aprender em um projeto e ensinar os outros é federação."*
> — Cosca Architecture Chief, 2026-07-30
