# COGNITIVE GRAVITY ENGINE — Atração de Conhecimento Afim (F2.4)

> **Versão**: 2.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
>
> **Autoridade constitucional**: [CONSTITUTION.md](../../CONSTITUTION.md) — Implementa P1 (memória acima de tudo), P5 (aprender com erros).
>
> **Dependências**: [F1.3 Gap Detection](../gap-detection/SKILL.md) · [F1.4 Wisdom Decay](../wisdom-decay/WISDOM_DECAY.md) · [F1.6 Cognitive Entropy](../cognitive-entropy/ENTROPY.md) · [F9.1 Experience Compiler](../experience-compiler/SKILL.md) · [Semantic Memory](../semantic-memory/SKILL.md)
>
> **Lema**: *"Assim como a gravidade atrai massa, o Cognitive Gravity atrai conhecimento afim. Quando um agente trabalha num domínio, o conhecimento que ele precisa orbita até ele."* — O Don

---

## SUMÁRIO

1. [Definição](#1-definição)
2. [Fórmula de Gravidade](#2-fórmula-de-gravidade)
3. [Pipeline de Atração](#3-pipeline-de-atração)
4. [Órbitas de Conhecimento](#4-órbitas-de-conhecimento)
5. [Integrações](#5-integrações)
6. [Exemplo Real](#6-exemplo-real)
7. [Formato de Dados](#7-formato-de-dados)
8. [CLI e Automação](#8-cli-e-automação)
9. [Configuração](#9-configuração)
10. [Métricas do Engine](#10-métricas-do-engine)
11. [Restrições e Governança](#11-restrições-e-governança)
12. [Anti-Padrões e Vieses](#12-anti-padrões-e-vieses)
13. [Complemento: GRAVITY_ENTRY.md (v1.0.0)](#13-complemento-gravity_entrymd-v100)
14. [Histórico](#14-histórico)

---

## 1. DEFINIÇÃO

### 1.1 O que é

O **Cognitive Gravity Engine (F2.4)** é o sistema de **atração de conhecimento afim**. Na física, a gravidade atrai massa; no Cosca, a gravidade cognitiva atrai **learnings, patterns e DDNA relacionados** para o contexto do agente sempre que uma task chega.

**Massa cognitiva de um domínio = volume de conhecimento × relevância.**

Conhecimento afim não precisa ser buscado ativamente — ele **orbita** a task atual. Quando um agente recebe "corrigir race condition no registry", o engine calcula quais domínios conhecidos (concurrency, testing, registry) têm mais massa gravitacional e estão mais próximos da task, e injeta o conhecimento relevante **antes** do agente começar a trabalhar.

```
METÁFORA FÍSICA ←→ COGNITIVA

  Massa estelar            ←→  knowledge_mass(domínio) — volume de conhecimento
  Distância até o corpo    ←→  proximity(domínio, task) — similaridade semântica
  Força gravitacional      ←→  gravity_force = massa × proximidade
  Órbita interna (fresco)  ←→  knowledge recente, perto do núcleo do domínio
  Órbita externa (gelado)  ←→  knowledge velho, distante, alimentando Wisdom Decay
  Evento horizonte         ←→  limite de 3 domínios × 3 learnings injetados
```

### 1.2 Problema que resolve

| Antes (sem Gravity) | Depois (com Gravity) |
|---------------------|----------------------|
| Agente inicia task sem contexto de conhecimento acumulado | Agente recebe até 9 learnings/patterns/DDNA afins pré-carregados |
| Conhecimento valioso (ex: L9 — fix de race no CI) fica enterrado em `learnings.md` | O engine atrai L9 automaticamente para tasks de concurrency |
| Cada task repete erros que já foram aprendidos | O agente trabalha **com conhecimento pré-carregado** desde o passo 1 |
| Busca semântica depende de o agente lembrar de consultar | Injeção é automática, determinística e < 100ms |

### 1.3 Gravidade vs Confiança vs Entropia

| Dimensão | Pergunta | Escopo |
|----------|----------|--------|
| **Confiança** (Evidence/Trust) | "Quão certo estou desta entrada?" | Intra-entrada |
| **Gravidade** (F2.4) | "Quanto este domínio atrai conhecimento para a task?" | Domínio ↔ Task |
| **Entropia** (F1.6) | "Quão desorganizado está o conhecimento?" | Base inteira / domínio |

A gravidade **usa** confiança e freshness como insumos (via proximidade) e é **reduzida** por entropia (Seção 5.3).

---

## 2. FÓRMULA DE GRAVIDADE

### 2.1 Equação Principal

```
gravity_force(domínio_X, task_atual) = knowledge_mass(X) × proximity(X, task)
```

### 2.2 Massa Cognitiva (`knowledge_mass`)

```
knowledge_mass(X) = count_learnings(X) + count_patterns(X) × 2 + count_ddna(X) × 3
```

| Componente | Peso | Justificativa |
|------------|------|---------------|
| `count_learnings(X)` | ×1 | Experiência bruta — cada learning vale sua massa |
| `count_patterns(X)` | ×2 | Padrão compilado = learning validado e reutilizado (F9.1) — vale o dobro |
| `count_ddna(X)` | ×3 | Decisão registrada com contexto/riscos/resultado (F1.1) — vale o triplo |

A ponderação reflete a **densidade de conhecimento**: 1 DDNA (decisão documentada, com consequências) vale mais que 1 learning bruto. A massa cresce com **volume × densidade** — exatamente como um corpo celeste.

**Fontes de contagem** (por domínio):
- `learnings`: tags `#domain` em `internal/embed/cosca/memory/agent/*/learnings.md`
- `patterns`: tags `#domain` em `patterns.md` e `engines/pattern-evolution/patterns/`
- `ddna`: campo `Domínio:` em `internal/embed/cosca/memory/decisions/ddna/*.md`

### 2.3 Proximidade (`proximity`)

```
proximity(X, task) = semantic_similarity(tags_X, task_tags) + recency_factor
```

| Componente | Fonte | Faixa |
|------------|-------|-------|
| `semantic_similarity(tags_X, task_tags)` | Semantic Memory — vetores do domínio vs vetores da task (cosine em `.cosca/memory/vectors.db`) | 0.0 – 0.8 |
| `recency_factor` | Freshness médio dos itens do domínio (F1.4 Wisdom Decay) | 0.0 – 0.2 |

`recency_factor` escala com o freshness: `recency_factor = avg_freshness(X) × 0.2`. Conhecimento fresco (órbita interna) aproxima o domínio da task; conhecimento gelado (órbita externa) afasta.

### 2.4 Fórmula Efetiva (com Entropia)

Aplicando a integração com F1.6 (Seção 5.3):

```
gravity_force_final(X, task) = knowledge_mass(X) × proximity(X, task) × (1 − entropy_penalty)
```

Onde `entropy_penalty` = entropia do domínio (0.0–1.0) se disponível; senão, entropia global da base (F1.6). Conhecimento desorganizado **atrai menos** — como matéria que se desintegra.

### 2.5 Thresholds de Seleção

| Parâmetro | Valor Default | Significado |
|-----------|---------------|-------------|
| `min_mass` | 3 | Domínio com massa < 3 não atrai nada (muito pouco conhecimento para valer a injeção) |
| `min_proximity` | 0.50 | Domínio com proximidade < 0.50 está longe demais da task |
| `max_domains` | 3 | Máximo de domínios atraídos por task |
| `max_learnings_per_domain` | 3 | Máximo de itens injetados por domínio |
| `top_n` | 9 | Teto absoluto de injeção (3 × 3) |

---

## 3. PIPELINE DE ATRAÇÃO

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    F2.4 COGNITIVE GRAVITY PIPELINE                      │
└─────────────────────────────────────────────────────────────────────────┘

  1. TASK CHEGA
     └─ "corrigir race condition no registry" (com tags: #race-condition
        #concurrency #registry #go)

                              ▼
  2. GRAVITY ENGINE CALCULA MASSA DE CADA DOMÍNIO CONHECIDO
     └─ knowledge_mass(X) para todos os domínios no registry
        (cache de massa por domínio, TTL 300s → custo ~2ms)

                              ▼
  3. ATRAI TOP 3 DOMÍNIOS MAIS PRÓXIMOS
     └─ proximity(X, task) via Semantic Memory (tags_X × task_tags)
        ├── testing     (massa 25, proximidade 0.82) → 3 learnings relevantes
        ├── concurrency (massa 15, proximidade 0.75) → 2 learnings
        └── registry    (massa  8, proximidade 0.70) → 1 learning
        Ranking: gravity_force = massa × proximidade
        testing 25×0.82=20.5 ████████████████████ 1º
        conc.   15×0.75=11.3 ████████████         2º
        registry 8×0.70= 5.6 ██████                3º

                              ▼
  4. INJETA CONHECIMENTO ATRAÍDO NO CONTEXTO DO AGENTE
     └─ Bloco estruturado "KNOWLEDGE IN ORBIT" anexado ao contexto:
        fonte, domínio, gravity_force, snippet de cada item.
        Rankeamento interno: gravity_force × relevance do item.

                              ▼
  5. AGENTE TRABALHA COM CONHECIMENTO PRÉ-CARREGADO
     └─ Não precisa buscar; usa L9 + L21 + L29 já no contexto.
        Ao final da task, Stage 7-8 do metacognition pipeline
        alimenta F9.1 → massa recalcula (Seção 5.1).
```

### 3.1 Orçamento de Tempo

| Etapa | Operação | Custo |
|-------|----------|-------|
| 1 | Extrair tags da task | ~1ms |
| 2 | Ler massa por domínio (cache) | ~2ms |
| 3 | Similaridade semântica (3–15 domínios × 1 query vetorial) | ~40ms |
| 4 | Seleção top-3 + montagem do bloco de contexto | ~5ms |
| 5 | (injeção — custo do agente, fora do engine) | — |
| **Total** | | **< 50ms** (garantia < 100ms) |

**Estratégia de timeout**: se > 100ms, injeta com o cache de massa existente e sem a busca vetorial completa (modo degradado). Nunca bloqueia a task.

### 3.2 Modo Degradado

Se o Semantic Memory estiver indisponível:
- `proximity = recency_factor` apenas (sem similaridade semântica)
- Seleção por massa pura (domínios com maior `knowledge_mass`)
- Injeção limitada a `max_learnings_per_domain` itens por massa

---

## 4. ÓRBITAS DE CONHECIMENTO

### 4.1 Conceito

Cada domínio é um **núcleo gravitacional**. Seu conhecimento orbita em camadas determinadas pelo **freshness** (F1.4):

```
NÚCLEO DO DOMÍNIO (massa)
   │
   ├── ÓRBITA INTERNA — fresco, perto do núcleo
   │     freshness ≥ 0.70  →  atração plena, injetado com prioridade
   │     "recentemente usado/validado — ainda quente"
   │
   ├── ÓRBITA MÉDIA
   │     0.30 ≤ freshness < 0.70  →  atração moderada, injetado se houver vaga
   │     "conhecido, mas esfriando"
   │
   └── ÓRBITA EXTERNA — velho, gelado, distante do núcleo
         freshness < 0.30  →  atração mínima, NÃO injetado
         "alimenta Wisdom Decay (F1.4) para revalidação ou depreciação"
```

### 4.2 Orbital Radius

```
orbit_radius(item) = 1 − freshness(item)
radius < 0.30  →  órbita interna
0.30–0.70      →  órbita média
radius ≥ 0.70  →  órbita externa (gelado)
```

- **Uso recente** puxa o item para perto do núcleo (freshness sobe, F1.4).
- **Desuso** empurra o item para fora (freshness cai).
- **Depreciação** (F1.4) remove o item da órbita — ele não atrai mais nada.

### 4.3 Integração com Wisdom Decay (F1.4)

| Evento F1.4 | Efeito na Órbita | Efeito na Gravidade |
|-------------|------------------|---------------------|
| Freshness sobe (revalidação ok) | Item migra para órbita interna | `recency_factor` sobe → proximidade sobe |
| Freshness cai (desuso) | Item migra para órbita externa | `recency_factor` cai → proximidade cai |
| Revalidação disparada (gatilho T1-T5) | Item em trânsito, injeção suspensa | Massa congelada até revalidação |
| Depreciação (F9.4) | Item **sai da órbita** | Massa do domínio cai (count − 1) |

A órbita é a **interface natural** entre atração (F2.4) e envelhecimento (F1.4): o que esfria deixa de ser atraído, e o que deixa de ser atraído esfria mais rápido.

---

## 5. INTEGRAÇÕES

### 5.1 Integração com F9.1 Experience Compiler

**Gatilho**: após cada compilação (pipeline F9.1, agendada ou pós-onda).

**Efeito**: a massa dos domínios **recalcula** — conhecimento novo aumenta a gravidade.

```
F9.1 compila N learnings de um domínio → 1 padrão novo
  count_learnings:  ↓ N (learnings consumidos)
  count_patterns:   ↑ 1 (padrão criado)  ×2
  count_ddna:       ↑ (se princípio virar emenda constitucional) ×3

  knowledge_mass ANTES:  10 learnings + 2 patterns×2 + 1 ddna×3 = 17
  knowledge_mass DEPOIS:  6 learnings + 3 patterns×2 + 1 ddna×3 = 18
  → massa cresce (padrão compilado vale mais que o learning bruto)
  → gravity_force do domínio cresce para TODAS as tasks futuras
```

- Massa compilada é **mais densa**: o mesmo volume passa a atrair mais.
- Emenda constitucional (F9.2) incrementa `count_ddna` — conhecimento fundacional exerce gravidade máxima.
- Recalculo: `cosca gravity recalc` ou automático via evento `knowledge.compressed` / `pattern.evolved`.

### 5.2 Integração com F1.3 Gap Detection

- **GAP_MEMORY** (agente sem learnings recentes no domínio) → domínio perde massa (volume estagnado) e o agente não é atraído.
- **GAP_COVERAGE/DOCS/ARCH** no domínio → entropia do domínio sobe (F1.6) → gravidade efetiva cai (Seção 5.3).
- Domínio com gaps persistentes recebe alerta: "massa alta mas desorganizada — curadoria antes de atração".
- Quando o gap é resolvido (novo learning validado), massa sobe — a atração recompensa a correção.

### 5.3 Integração com F1.6 Cognitive Entropy

**Regra**: entropia alta **reduz** a gravidade — conhecimento desorganizado atrai menos.

```
gravity_force_final = gravity_force × (1 − entropy_penalty)
```

| Entropia (F1.6) | Penalty | Efeito |
|-----------------|---------|--------|
| 0.00 (🟢 organizado) | 0.0 | Gravidade plena |
| 0.25 (🟡) | 0.25 | Gravidade 75% |
| 0.433 (🔴 atual) | 0.433 | Gravidade 57% — metade da atração é perdida |
| 0.75 (🔴 crítico) | 0.75 | Gravidade 25% — quase não atrai |

**Justificativa**: injetar conhecimento contraditório ou stale no contexto do agente é pior que não injetar — polui a decisão. A entropia é o "vento solar" que desvia a gravidade.

**Ciclo virtuoso**: F9.4 deprecia stale → entropia cai (F1.6) → gravidade sobe → mais atração de conhecimento limpo → decisões melhores → menos contradições.

### 5.4 Integração com Semantic Memory

| Operação | Papel |
|----------|-------|
| `semantic_similarity(tags_X, task_tags)` | Cálculo de proximidade (busca vetorial em `.cosca/memory/vectors.db`) |
| `relevance` do item | Ranking interno dos itens a injetar dentro de cada domínio |
| Reindexação incremental | Mantém tags dos novos learnings/patterns/DDNA consultáveis |

**Contrato de performance**: a consulta vetorial para o cálculo de proximidade deve retornar em < 40ms (índice FTS5 + vector, cache LRU de embeddings 1000 entradas, TTL 300s).

### 5.5 Integração com o Metacognition Pipeline

| Stage | Ponto de Gravidade |
|-------|--------------------|
| Stage 1 (SELF-ASSESS) | Engine calcula `gravity_force` para o domínio da task |
| Stage 2 (RETRIEVE MEMORY) | Conhecimento atraído entra como bloco "KNOWLEDGE IN ORBIT" |
| Stage 3 (PLAN STRATEGY) | Agente planeja com conhecimento pré-carregado |
| Stage 7 (EXTRACT PATTERN) | Novo learning/pattern alimenta `count_learnings/patterns` |
| Stage 8 (UPDATE CAPABILITY) | Massa do domínio recalcula (futuras tasks atraem mais) |

---

## 6. EXEMPLO REAL

### 6.1 Cenário

Task: **"corrigir race condition no registry"** (tags extraídas: `#race-condition`, `#concurrency`, `#registry`).

### 6.2 Cálculo de Massa (passo 2 do pipeline)

| Domínio | learnings | patterns ×2 | ddna ×3 | **massa** |
|---------|-----------|-------------|---------|-----------|
| testing | 10 | 6 | 1 | 10+12+3 = **25** |
| concurrency | 6 | 3 | 1 | 6+6+3 = **15** |
| registry | 3 | 1 | 1 | 3+2+3 = **8** |

### 6.3 Cálculo de Proximidade (passo 3)

| Domínio | semantic_similarity | recency_factor (freshness) | **proximidade** |
|---------|---------------------|----------------------------|-----------------|
| testing | 0.62 (tags #go #race compartilhadas) | 0.20 (fresco — auditado ontem) | **0.82** |
| concurrency | 0.60 (tags #concurrency diretas) | 0.15 (uso recente) | **0.75** |
| registry | 0.58 (tag #registry na task) | 0.12 | **0.70** |
| frontend | 0.10 | 0.05 | 0.15 ✗ (< min_proximity) |

### 6.4 Força Gravitacional e Seleção

```
gravity_force(testing)     = 25 × 0.82 = 20.5  → 1º — injeta 3 itens
gravity_force(concurrency) = 15 × 0.75 = 11.3  → 2º — injeta 2 itens
gravity_force(registry)    =  8 × 0.70 =  5.6  → 3º — injeta 1 item
total = 6 itens ≤ 9 (limite) ✓
```

### 6.5 Injeção no Contexto

```
📦 KNOWLEDGE IN ORBIT — task "corrigir race condition no registry"
   fonte: cognitive-gravity (F2.4) | overhead: 48ms | 6 itens

   ▸ testing (massa 25, proximidade 0.82, force 20.5)
     • L9  [cosca-kernel] Parallel CI Fix Orchestration —
         "Race condition pattern: global state + goroutine = mutex obrigatório.
          Emit() capturava globalTelemetry em closure sem lock.
          Fix: sync.RWMutex + snapshot local."        (relevance 0.91)
     • L21 [cosca-kernel] Coverage Audit — "20 race conditions em 5 pacotes:
          Chat (12 races Registry/Message), CLI (4 races Serve/Shutdown),
          circuitbreaker, google, mistral. -race flag obrigatório no CI." (0.87)
     • L29 [cosca-kernel] Onda F0 — "21 race conditions resolvidas. Padrão:
          2 ondas paralelas (diagnose → fix)."         (relevance 0.84)

   ▸ concurrency (massa 15, proximidade 0.75, force 11.3)
     • [cosca-testing] Race Condition Corrections — "atomic.Pointer para
        estado de servidor; syncWriter para I/O compartilhado." (0.89)
     • [cosca-testing] 20 races em 5+ pacotes — "registry, output,
        embeddings providers."                          (relevance 0.82)

   ▸ registry (massa 8, proximidade 0.70, force 5.6)
     • [cosca-testing] Registry race — "chat/registry.go ~4 races:
        singleton init global, provider caching."       (relevance 0.80)
```

### 6.6 Resultado

O agente recebe **L9 + L21 + L29** (todos sobre races — exatamente o exemplo do Don) **mais** o conhecimento específico de registry, sem nenhuma busca manual. O trabalho começa com o conhecimento acumulado de 3 auditorias e 21 correções de race já no contexto.

---

## 7. FORMATO DE DADOS

### 7.1 SQLite (`.cosca/memory/vectors.db` — mesmo DB do Semantic Memory)

```sql
-- Massa por domínio (cache com TTL)
CREATE TABLE IF NOT EXISTS gravity_domains (
    domain TEXT PRIMARY KEY,
    count_learnings INTEGER DEFAULT 0,
    count_patterns  INTEGER DEFAULT 0,
    count_ddna      INTEGER DEFAULT 0,
    knowledge_mass  REAL DEFAULT 0,
    avg_freshness   REAL DEFAULT 0,
    entropy_penalty REAL DEFAULT 0,       -- F1.6
    mass_recalculated_at TEXT,            -- ISO8601 (após F9.1)
    cache_ttl_s     INTEGER DEFAULT 300
);

-- Órbita de cada item de conhecimento
CREATE TABLE IF NOT EXISTS gravity_orbits (
    knowledge_id TEXT PRIMARY KEY,
    domain TEXT NOT NULL,
    knowledge_type TEXT NOT NULL,          -- learning | pattern | ddna
    source TEXT NOT NULL,                  -- internal/embed/cosca/memory/...
    orbit_radius REAL DEFAULT 0,           -- 1 - freshness (F1.4)
    freshness REAL DEFAULT 0,
    last_attracted_at TEXT,
    times_injected INTEGER DEFAULT 0,
    FOREIGN KEY (domain) REFERENCES gravity_domains(domain)
);

-- Log de injeções (audit)
CREATE TABLE IF NOT EXISTS gravity_injections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    task_tags TEXT NOT NULL,               -- JSON array
    domain TEXT NOT NULL,
    gravity_force REAL NOT NULL,
    injected_count INTEGER NOT NULL,
    overhead_ms INTEGER NOT NULL,
    timestamp TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_gravity_domains_mass ON gravity_domains(knowledge_mass DESC);
CREATE INDEX IF NOT EXISTS idx_gravity_orbits_domain ON gravity_orbits(domain, orbit_radius);
CREATE INDEX IF NOT EXISTS idx_gravity_injections_task ON gravity_injections(task_id, timestamp);
```

### 7.2 Payload de Injeção (contexto do agente)

```yaml
gravity_injection:
  engine: "cognitive-gravity"
  version: "2.0.0"
  task_id: "T-2026-07-30-042"
  task_tags: ["#race-condition", "#concurrency", "#registry"]
  overhead_ms: 48
  domains:
    - domain: "testing"
      mass: 25
      proximity: 0.82
      gravity_force: 20.5
      items:
        - {id: "L9",  type: "learning", source: "cosca-kernel/learnings.md", relevance: 0.91, freshness: 0.85}
        - {id: "L21", type: "learning", source: "cosca-kernel/learnings.md", relevance: 0.87, freshness: 0.90}
        - {id: "L29", type: "learning", source: "cosca-kernel/learnings.md", relevance: 0.84, freshness: 0.80}
```

---

## 8. CLI E AUTOMAÇÃO

```bash
# Atrair conhecimento para uma task (modo inspeção, não injeta)
cosca gravity field "corrigir race condition no registry"

# Desligar/ligar o engine (Don control)
cosca gravity off          # desliga — nenhuma task recebe injeção
cosca gravity on           # religa
cosca gravity status       # enabled/disabled + última injeção

# Massa por domínio
cosca gravity mass --domain testing
cosca gravity mass --top 10

# Recalcular massa (após F9.1 compilation)
cosca gravity recalc

# Ver órbita de um domínio (fresco → gelado)
cosca gravity orbits --domain testing

# Histórico de injeções
cosca gravity injections --task T-2026-07-30-042
cosca gravity injections --since 24h

# Simular (dry-run, sem tocar contexto)
cosca gravity simulate "corrigir race condition no registry"
```

### Config (`internal/embed/cosca/engines/cognitive-gravity/config.yaml`)

```yaml
cognitive_gravity:
  enabled: true                  # cosca gravity off → false
  overhead_budget_ms: 100
  max_domains: 3
  max_learnings_per_domain: 3
  min_mass: 3
  min_proximity: 0.50
  weights:
    learning: 1
    pattern: 2
    ddna: 3
  recency_factor_max: 0.2
  entropy_weight: 1.0           # multiplicador da penalty F1.6
  cache:
    mass_ttl_s: 300
    embedding_cache: 1000
```

---

## 9. CONFIGURAÇÃO

### 9.1 Thresholds

| Chave | Default | Regra |
|-------|---------|-------|
| `enabled` | `true` | Don desliga com `cosca gravity off` |
| `max_domains` | `3` | Nunca exceder 3 domínios por task |
| `max_learnings_per_domain` | `3` | Nunca exceder 3 itens por domínio |
| `min_mass` | `3` | Abaixo disso, domínio não atrai |
| `min_proximity` | `0.50` | Abaixo disso, domínio está longe demais |
| `overhead_budget_ms` | `100` | Orçamento rígido de injeção |

### 9.2 Invariantes (imutáveis)

1. **Injeção máxima = 9 itens** (3 domínios × 3 itens). Contexto é recurso finito — sobrecarregar mata o benefício.
2. **Overhead < 100ms por task**. A injeção é leve por design; se estourar, degrada (Seção 3.2).
3. **Don control absoluto**: `cosca gravity off` desliga instantaneamente, sem perda de dados.
4. **Nunca injeta conhecimento depreciado** (órbita externa + deprecado = fora da atração).
5. **Atribuição obrigatória**: todo item injetado traz fonte (arquivo + agente). Conhecimento sem fonte não atrai.

---

## 10. MÉTRICAS DO ENGINE

| Métrica | Tipo | Descrição | Meta |
|---------|------|-----------|------|
| `f24.overhead_ms` | Histogram | Latência de injeção por task | p99 < 100ms |
| `f24.injections` | Counter | Itens injetados (total) | — |
| `f24.tasks_with_injection` | Counter | Tasks que receberam ≥ 1 item | — |
| `f24.injection_rate` | Gauge | Tasks com injeção / total de tasks | > 60% |
| `f24.adoption_rate` | Gauge | Itens injetados efetivamente usados no plano/task | > 0.50 |
| `f24.avg_mass` | Gauge | Massa média dos domínios | crescente |
| `f24.avg_proximity` | Gauge | Proximidade média dos domínios atraídos | > 0.60 |
| `f24.entropy_penalty` | Gauge | Penalty médio aplicado (F1.6) | < 0.30 |
| `f24.orbits_external` | Gauge | Itens em órbita externa (alimentando F1.4) | < 20% |
| `f24.mass_recalc_count` | Counter | Recalques pós-F9.1 | — |

### Relatório de 1 parágrafo (formato executivo)

> "Gravity injetou conhecimento em **X** tasks (Y% de adoção). Domínios mais massivos: testing (25), concurrency (15). Entropia reduz gravidade em **Z%** — curadoria em `registry` destravaria atração. 4 itens em órbita externa aguardam revalidação (F1.4)."

---

## 11. RESTRIÇÕES E GOVERNANÇA

### 11.1 Restrições

1. **Performance**: overhead por task NUNCA excede 100ms. Se exceder, degrada sem bloquear.
2. **Contexto**: máximo 3 domínios × 3 itens = 9 learnings injetados. Não sobrecarregar.
3. **Qualidade**: entropia alta (F1.6) reduz a gravidade — nunca injetar conhecimento de domínio desorganizado como se fosse limpo.
4. **Controle**: Don pode desligar (`cosca gravity off`). Zero surpresas.
5. **Transparência**: toda injeção é logada com fonte, força e overhead.
6. **Complementaridade**: o micro-modelo de entrada (GRAVITY_ENTRY.md v1.0.0) é o refinamento opcional do ranking interno; não substitui o macro-modelo de domínio.

### 11.2 Governança

| Papel | Responsabilidade |
|-------|------------------|
| **Architecture Chief** | Dono do engine, thresholds, fórmula, integrações |
| **CTO** | Aprova mudanças de pesos (`learning:pattern:ddna`) |
| **Memory Chief** | Curadoria dos domínios (órbitas, depreciação F1.4) |
| **Semantic Memory Chief** | Disponibilidade e latência da busca vetorial |
| **Kernel** | Invoca o engine no Stage 1-2 e orquestra recalque pós-F9.1 |

---

## 12. ANTI-PADRÕES E VIESES

| Anti-padrão | Risco | Mitigação |
|-------------|-------|-----------|
| **Injeção excessiva** | Contexto poluído, agente ignora tudo | Teto rígido de 9 itens + `min_proximity` |
| **Rich-get-richer** | Domínio massivo atrai sempre, domínio novo nunca | `min_mass` baixo + entropia reduz domínios desorganizados |
| **Recency bias** | Conhecimento novo não validado domina a atração | `recency_factor` limitado a 0.2 (20% da proximidade) |
| **Entropy blindness** | Atrair contradições/stale como se fossem limpos | `gravity_force × (1 − entropy_penalty)` obrigatório |
| **Compilation lag** | Massa desatualizada após F9.1 | `gravity recalc` automático no evento `knowledge.compressed` |
| **Cold-start** | Base nova sem massa em nenhum domínio | Nenhuma injeção até `min_mass` (comportamento esperado) |
| **Tag pollution** | Tags genéricas (#go, #fix) inflam a massa | Contagem por tags canônicas de domínio (registry de domínios) |

---

## 13. COMPLEMENTO: GRAVITY_ENTRY.MD (V1.0.0)

O arquivo **[GRAVITY_ENTRY.md](./GRAVITY_ENTRY.md)** preserva o modelo v1.0.0 (1595 linhas, incluindo a nota de complemento no cabeçalho): a **gravidade por entrada de conhecimento** (validation_count, diversity, cross_project, time_factor, evidence_strength, níveis Dust→Planet). Ele é o **micro-modelo complementar**:

| Modelo | Arquivo | Unidade | Pergunta |
|--------|---------|---------|----------|
| **F2.4 (v2.0.0)** | `SKILL.md` | Domínio ↔ Task | "Qual conhecimento orbita esta task?" |
| **v1.0.0** | `GRAVITY_ENTRY.md` | Entrada individual | "Quanto esta entrada influencia decisões?" |

**Uso integrado**: dentro de um domínio atraído, o ranking dos itens a injetar pode ser ponderado pelo `gravity_score` do item (GRAVITY_ENTRY.md), refinando o `relevance` do Semantic Memory. Configuração:

```yaml
cognitive_gravity:
  entry_level_weight: 0.3   # 0 = apenas relevance semântica
                            # 0.3 = 30% do ranking vem da gravidade da entrada
```

A fórmula canônica do Don (`knowledge_mass × proximity`) permanece intocada — o micro-modelo apenas **ordena** o que já foi atraído.

---

## 14. HISTÓRICO

| Versão | Data | Autor | Alterações |
|--------|------|-------|------------|
| 1.0.0 | 2026-07-30 | Cosca Memory Chief | Modelo de gravidade por entrada de conhecimento (validation_count, diversity, cross_project, time_factor, evidence_strength; níveis Dust→Planet). 1594 linhas. (Preservado em GRAVITY_ENTRY.md) |
| 2.0.0 | 2026-07-30 | Architecture Chief | Reformulação para o modelo canônico do Don: **atração de conhecimento afim por domínio**. `gravity_force = knowledge_mass × proximity`, com `knowledge_mass = learnings + patterns×2 + ddna×3` e `proximity = similarity + recency`. Pipeline de 5 passos com injeção de top-3 domínios × 3 itens (máx 9). Órbitas de conhecimento alimentando Wisdom Decay (F1.4). Integrações F9.1 (recalque de massa pós-compilação), F1.6 (entropia reduz gravidade), F1.3 (gaps), Semantic Memory. Exemplo real: race condition → L9+L21+L29. CLI `cosca gravity [field|off|on|recalc|orbits]`. Overhead < 100ms. |

---

> **"Cada ideia atrai outras relacionadas. Quanto mais evidências uma decisão recebe, maior sua gravidade."**
>
> — O Don, definindo o conceito de Gravidade Cognitiva

> **"O conhecimento que você precisa não precisa ser procurado. Ele orbita até você."**
>
> — Architecture Chief, 2026-07-30
