# DISCOVERY ENGINE ★ F10.3 — Mineração de Serendipidade

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-discovery-run` | **Código**: F10.3
> **Conceito**: ★ F10.3 — Discovery Engine — **"O que descobrimos que não procurávamos?"**
> **Referências**: next-evolution-phases.md §F10.3 | cognitive-metrics.md (F1.5) | experience-compiler/SKILL.md (F9.1) | ENGINEERING_TIMELINE.md
> **Dependências**: F9.1 (Experience Compiler) | F1.5 (Métricas B1-B5) | F1.4 (Wisdom Decay — freshness das evidências) | memorize-commit workflow (Impact Reports)
> **CMI Impact**: Aprendizado +8, Julgamento +6

---

## Índice

1. [Definição — Caçador de Insights Não-Planejados](#1-definição--caçador-de-insights-não-planejados)
2. [Coexistência no Diretório — Duas Engines, Uma Pasta](#2-coexistência-no-diretório--duas-engines-uma-pasta)
3. [Pipeline de Descoberta (Fim de Sprint)](#3-pipeline-de-descoberta-fim-de-sprint)
4. [Fórmula de Serendipidade](#4-fórmula-de-serendipidade)
5. [Classificação das Descobertas](#5-classificação-das-descobertas)
6. [Relatório de Descobertas](#6-relatório-de-descobertas)
7. [Integração com F9.1 — Experience Compiler](#7-integração-com-f91--experience-compiler)
8. [Integração com F1.5 — Métricas B1-B5](#8-integração-com-f15--métricas-b1-b5)
9. [Exemplo Real — Sprint de Cobertura de Testes](#9-exemplo-real--sprint-de-cobertura-de-testes)
10. [Regras de Operação](#10-regras-de-operação)
11. [Arquitetura do Motor](#11-arquitetura-do-motor)
12. [Métricas do Próprio Engine](#12-métricas-do-próprio-engine)
13. [Implementação e Automação](#13-implementação-e-automação)
14. [Casos de Borda e Anti-Padrões](#14-casos-de-borda-e-anti-padrões)
15. [Referências Cruzadas](#15-referências-cruzadas)
16. [Histórico](#16-histórico)

---

## 1. Definição — Caçador de Insights Não-Planejados

O **Discovery Engine** é o caçador de **serendipidade** do Cosca. Serendipidade é a arte de encontrar **o que não se procurou** — o valor que emerge de um esforço sem estar no plano do esforço.

A pergunta que este engine responde:

> **"O que descobrimos que não procurávamos?"**

Enquanto o F9.1 Experience Compiler pergunta "o que aprendemos que é reutilizável?", o F10.3 pergunta "o que aprendemos **sem ter planejado** aprender?". O F9.1 compila tudo o que existe; o F10.3 isola especificamente o **fator surpresa** — a divergência entre o plano da sprint e o que de fato emergiu.

### 1.1 Filosofia

```
"Uma sprint procurou 'aumentar cobertura de testes'.
 A sprint encontrou 'um padrão de race condition em 3 pacotes'.
 
 A cobertura era o plano. O race condition era a descoberta.
 
 O Discovery Engine não busca — ele observa.
 Ele lê o rastro que a sprint deixou para trás
 e pergunta: 'o que sobrou que ninguém pediu?'
 
 Descoberta não é trabalho extra. É reconhecimento
 de valor que já aconteceu sem ser cobrado."
 — Cosca Architecture Chief, 2026-07-30
```

### 1.2 O Observador, Não o Caçador

| Característica | Discovery Engine (F10.3) |
|----------------|--------------------------|
| **Postura** | Passiva — observa dados existentes |
| **Trigger** | Fim de sprint (ou pós-N tasks) |
| **Custo** | Agregação + heurísticas, **sem LLM**, sem criação de trabalho |
| **Output** | Sugestões classificadas (HIGH/MEDIUM/LOW) |
| **Decisão** | Don decide se a descoberta vira ação (DDNA, learning, padrão) |
| **Fonte** | Exhaust data da sprint: learnings, impact reports, failures, patterns, DDNAs, B1-B5 |

O motor é deliberadamente **barato e observacional**: ele nunca gera insight sintético, nunca cria task nova para "descobrir" algo. Toda descoberta precisa de **rastro de evidência** em artefatos concretos — se não há artefato, não há descoberta.

### 1.3 Diferenciação contra engines irmãs (anti-duplicação)

| Engine | Pergunta | Dimensão | Relação com F10.3 |
|--------|----------|----------|-------------------|
| **F9.1 Experience Compiler** | "O que aprendemos que é reutilizável?" | Compilação (learnings → padrões) | Consumidor das descobertas HIGH |
| **F9.2 Wisdom Distillation** | "O que destila até princípio?" | Governança (padrões → emendas) | Downstream indireto (via F9.1) |
| **F1.3 Gap Detection** | "O que está faltando?" | Ausência | Complementar — gap é ausência, descoberta é presença não-planejada |
| **F8.3 Contradiction Engine** | "Onde isto já falhou?" | Evidência contra uma decisão | Complementar — descoberta não é contra nem a favor, é novidade |
| **F2.5 Cognitive Momentum** | "Estamos aprendendo rápido e na direção certa?" | Velocidade/direção | Descoberta é um tipo de aprendizado que o momentum contabiliza |
| **Workspace Discovery (WORKSPACE.md)** | "Qual é a paisagem do código?" | Paisagem física (stack) | Ortogonal — nunca analisa insights |
| **★ F10.3 Discovery Engine** | "O que surgiu sem planejarmos?" | **Presença não-planejada** | — |

**Regra de ouro**: o F10.3 **não compila** (isso é do F9.1), **não destila** (F9.2), **não procura ausências** (F1.3), **não julga decisões** (F8.3). Ele apenas **sinaliza**: "isto aqui surgiu fora do plano e parece valioso".

---

## 2. Coexistência no Diretório — Duas Engines, Uma Pasta

O diretório `engines/discovery/` abriga **duas engines complementares** — mesmo padrão de coexistência do `gap-detection/` (COGNITIVE.md + SKILL.md):

| Arquivo | Engine | Pergunta | Trigger | Fonte |
|---------|--------|----------|---------|-------|
| [`WORKSPACE.md`](WORKSPACE.md) | Workspace Discovery (v1.0.0, preservado) | "Qual é a stack deste workspace?" | session_start / bootstrap | Arquivos do projeto (package.json, go.mod...) |
| **`SKILL.md`** (este arquivo) | **★ F10.3 Discovery Engine** | "O que descobrimos que não procurávamos?" | Fim de sprint / pós-N tasks | Memória cognitiva (learnings, DDNAs, metrics...) |

**Regra de coexistência**: ortogonalidade dimensional. A engine de workspace varre a **paisagem física** do repositório; a F10.3 analisa a **paisagem cognitiva** da sprint. Fontes disjuntas, triggers disjuntos, outputs disjuntos. Nenhuma substitui a outra — referências cruzadas em ambos os arquivos.

---

## 3. Pipeline de Descoberta (Fim de Sprint)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                  DISCOVERY ENGINE ★ F10.3 — PIPELINE                         │
│                              Fim de Sprint                                   │
│                                                                              │
│  TRIGGERS:                                                                   │
│  ┌──────────────┐  ┌─────────────────┐  ┌───────────────────────────┐       │
│  │ Fim de sprint│  │ Pós-N tasks     │  │ CLI: cosca discovery run  │       │
│  │ (semanas)    │  │ (N=10 default)  │  │ (on-demand, idempotente)  │       │
│  └──────┬───────┘  └────────┬────────┘  └────────────┬──────────────┘       │
│         └───────────────────┼────────────────────────┘                      │
│                             ▼                                               │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │ PASSO 1 — COLETA (todos os dados do sprint)                            │ │
│  │                                                                         │ │
│  │  ├── Learnings registrados   → memory/agent/*/learnings.md             │ │
│  │  │     (tags, domínios, timestamps do período)                          │ │
│  │  ├── Impact Reports          → memory/timeline/impact-reports/*.md     │ │
│  │  │     (learnings extraídos por commit, por memorize-commit)            │ │
│  │  ├── Failures                → memory/agent/*/failures.md              │ │
│  │  │     (erros, causas-raiz, prevenção)                                  │ │
│  │  ├── Patterns criados        → memory/agent/*/patterns.md              │ │
│  │  ├── DDNAs criados           → memory/decisions/ddna/*.md              │ │
│  │  └── Metrics B1-B5           → memory/timeline/*.csv                   │ │
│  │        (cognitive-load, decision-velocity, knowledge-freshness,        │ │
│  │         cross-agent-reuse, cognitive-debt)                              │ │
│  │  ⏱ < 2s — leitura paralela, sem LLM                                    │ │
│  └───────────────────────────────┬────────────────────────────────────────┘ │
│                                  ▼                                          │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │ PASSO 2 — COMPARA COM O PLANO DA SPRINT                                │ │
│  │                                                                         │ │
│  │  1. Extrai o PLANO: temas da sprint                                     │ │
│  │     ├── ENGINEERING_TIMELINE.md (títulos de sessão/tema)                │ │
│  │     ├── next-evolution-phases.md (fase em andamento)                    │ │
│  │     └── memory/roadmap/ (objetivos declarados)                          │ │
│  │  2. Deriva os DOMÍNIOS PLANEJADOS                                       │ │
│  │     ex: "aumentar cobertura de testes" → {testing, quality}            │ │
│  │  3. Para cada artefato coletado, extrai os DOMÍNIOS QUE TOCA (tags)     │ │
│  │  4. Output: plano (domínios) × realizado (domínios tocados)             │ │
│  │  ⏱ < 0.5s                                                               │ │
│  └───────────────────────────────┬────────────────────────────────────────┘ │
│                                  ▼                                          │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │ PASSO 3 — IDENTIFICA DIVERGÊNCIAS (5 sinais)                            │ │
│  │                                                                         │ │
│  │  S1. Learning em domínio NÃO planejado  → descoberta                   │ │
│  │      (tags do learning ∉ domínios do plano)                             │ │
│  │                                                                         │ │
│  │  S2. Pattern que surgiu de erro        → descoberta                   │ │
│  │      (patterns.md cuja aplicação cita failures.md como origem)         │ │
│  │                                                                         │ │
│  │  S3. Insight cross-domain              → descoberta                   │ │
│  │      (1 artefato com tags em ≥ 2 domínios distintos)                    │ │
│  │                                                                         │ │
│  │  S4. B4 anomaly — reuse não-planejado → descoberta                   │ │
│  │      (cross-agent-reuse.csv: source_agents de domínio fora do plano,    │ │
│  │       ou spike de reuse num domínio não-planejado)                      │ │
│  │                                                                         │ │
│  │  S5. Commit não-learning que gerou learnings → descoberta            │ │
│  │      (impact report de commit fix/refactor/test com learnings > 0 —     │ │
│  │       aprendizado não-planejado aconteceu ali)                          │ │
│  │  ⏱ < 1s — heurísticas sobre dados já em memória                         │ │
│  └───────────────────────────────┬────────────────────────────────────────┘ │
│                                  ▼                                          │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │ PASSO 4 — CLASSIFICA (fórmula de serendipidade)                         │ │
│  │                                                                         │ │
│  │  Para cada divergência:                                                 │ │
│  │    discovery_value = novelty × cross_domain × applicability            │ │
│  │                                                                         │ │
│  │  HIGH   → cross_domain ≥ 3 E discovery_value ≥ 1.5                     │ │
│  │  MEDIUM → cross_domain ≥ 2 OU discovery_value ≥ 1.0                    │ │
│  │  LOW    → curiosidade (demais)                                          │ │
│  │  ⏱ < 0.5s                                                               │ │
│  └───────────────────────────────┬────────────────────────────────────────┘ │
│                                  ▼                                          │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │ PASSO 5 — GERA RELATÓRIO DE DESCOBERTAS                                 │ │
│  │                                                                         │ │
│  │  → memory/timeline/discovery-reports/discovery-{sprint}.md             │ │
│  │    (tema, domínios planejados, HIGH/MEDIUM/LOW com evidências e         │ │
│  │     discovery_value)                                                    │ │
│  │  LOW não entra no resumo do Don — apenas log.                           │ │
│  │  ⏱ < 0.2s                                                               │ │
│  └───────────────────────────────┬────────────────────────────────────────┘ │
│                                  ▼                                          │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │ PASSO 6 — DESCOBERTAS HIGH → SUGESTÃO DE AÇÃO                           │ │
│  │                                                                         │ │
│  │  Cada HIGH vira UMA sugestão ao Don:                                    │ │
│  │  ├── Sugestão de DDNA (decisão) — ex: "policy global de locks"         │ │
│  │  └── OU learning prioritário marcado #discovery-high → F9.1             │ │
│  │                                                                         │ │
│  │  Don decide. Sem aprovação, a descoberta permanece no report            │ │
│  │  (nunca é executada automaticamente).                                   │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
│  TEMPO TOTAL ALVO: < 5s (agregação aritmética, zero LLM)                    │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Detalhamento das Fontes (Passo 1)

| Fonte | Caminho | Campo de descoberta |
|-------|---------|---------------------|
| Learnings | `memory/agent/*/learnings.md` | Tags, domínios, contexto — a matéria-prima dos sinais S1/S3 |
| Impact Reports | `memory/timeline/impact-reports/{commit}.md` | `Aprendizados: {n}` por tipo de commit — sinal S5 |
| Failures | `memory/agent/*/failures.md` | Erros que geraram padrão — sinal S2 |
| Patterns | `memory/agent/*/patterns.md` | Padrões novos do período — sinal S2 |
| DDNAs | `memory/decisions/ddna/*.md` | Decisões que não estavam no plano |
| B1-B5 | `memory/timeline/cognitive-*.csv` | B4 (reuse) — sinal S4; B3 (freshness) — qualificador; B5 (debt) — integridade |

---

## 4. Fórmula de Serendipidade

```
discovery_value = novelty × cross_domain × applicability

Onde:
  novelty       = 1 - similaridade com learnings existentes        [0, 1]
  cross_domain  = quantos domínios toca                            [1, 5]
  applicability = probabilidade de uso futuro (estimada)           [0, 1]

Faixa do discovery_value: [0, 5]
```

### 4.1 Componentes derivados

**novelty** — mede o quão inédito é o insight em relação ao que o sistema já sabe:

```
novelty = 1 - max_similarity(insight, learnings_existentes)

max_similarity = max sobre todos os learnings existentes de:
  similarity(insight, L) = 0.7 × Jaccard(tags_insight, tags_L) + 0.3 × domain_match(L)

Onde:
  domain_match  = 1.0 se mesmo domínio, 0.5 se domínio relacionado, 0.0 se distinto
  (mesmo método de similaridade do F9.1 Experience Compiler — consistência entre engines)
```

Um insight cuja **conclusão** já existe (similaridade alta com um learning anterior) tem novelty baixo — não é descoberta, é reforço. Observações individuais não contam: mede-se a similaridade da **conclusão/insight**, não das evidências.

**cross_domain** — número de domínios distintos que o insight toca, derivado das tags dos artefatos de origem, **cap 5** (são contados no máximo 5 domínios; tocar mais de 5 não aumenta o score):

```
cross_domain = min(count(distinct domain_tags(artefatos_fonte)), 5)
```

**applicability** — probabilidade estimada de uso futuro. Estimativa baseada na **natureza da causa-raiz** (não no tamanho do artefato):

| Natureza da causa-raiz | applicability | Exemplo |
|------------------------|:-------------:|---------|
| **Sistêmica / org-wide** — afeta todo o sistema ou processo | 1.0 | "Race condition é sistêmico — precisa de policy global de locks" |
| **Multi-time** — afeta 2+ times/domínios | 0.7 | "Padrão de deadlock se repete em MCP e runtime" |
| **Time único** — afeta 1 time/domínio | 0.4 | "Ferramenta X tem bug específico do nosso setup" |
| **Curiosidade** — interessante, sem uso previsível | 0.1 | "Descobrimos um padrão de nomenclatura histórico" |

### 4.2 Leitura da fórmula

- **Multiplicativa** (zero-terminal — padrão P-ARCH-004): novelty = 0 (já sabíamos) zera a descoberta; cross_domain = 1 (não toca outros domínios) reduz; applicability = 0.1 (curiosidade) derruba.
- **A surpresa é o fator dominante**: sem novelty não há descoberta, por mais útil que o insight seja (insight útil e conhecido = reforço do F9.1, não descoberta do F10.3).
- **cross_domain distingue impacto de curiosidade**: um insight que atravessa domínios tem efeito amplificado — é o "padrão que explica 3 bugs em 3 pacotes".

---

## 5. Classificação das Descobertas

| Classe | Critério | Significado | Destino |
|--------|----------|-------------|---------|
| **HIGH** | `cross_domain ≥ 3` E `discovery_value ≥ 1.5` | Impacto em **múltiplos domínios** | Sugestão de DDNA ou learning prioritário `#discovery-high` → F9.1 |
| **MEDIUM** | `cross_domain ≥ 2` OU `discovery_value ≥ 1.0` | Impacto em **um domínio** | Registro no report; candidato a learning normal |
| **LOW** | demais | **Curiosidade** | Apenas log — não chega ao resumo do Don |

**Racional do requisito duplo no HIGH**: um insight que "toca 5 domínios" mas é trivial (applicability 0.1) não deve inflar — `discovery_value ≥ 1.5` filtra esse caso (5 × 0.1 = 0.5 < 1.5). E um insight com valor alto mas restrito a 1-2 domínios é MEDIUM, não HIGH — impacto concentrado, não multiplicado.

---

## 6. Relatório de Descobertas

Gerado no Passo 5, armazenado em `memory/timeline/discovery-reports/discovery-{sprint}.md`:

```markdown
## Discovery Report — Sprint {id} ({data})

| Campo | Valor |
|-------|-------|
| **Tema da sprint** | "aumentar cobertura de testes" |
| **Domínios planejados** | testing, quality |
| **Descobertas** | 3 (HIGH: 1, MEDIUM: 1, LOW: 1) |
| **Evidências analisadas** | 27 learnings, 9 impact reports, 3 failures, 5 CSVs |

---

### 🔥 HIGH-1: Race condition é sistêmico em 3 pacotes

| Componente | Valor |
|-----------|-------|
| **discovery_value** | 2.08 |
| **novelty** | 0.65 (conclusão sistêmica inédita — evidências individuais existiam) |
| **cross_domain** | 4 (testing, concurrency, architecture, ci) |
| **applicability** | 0.80 (causa-raiz sistêmica) |
| **Evidências** | `learnings.md L9, L21, L29` · `impact-reports/75d4d99.md` |
| **Por que não-planejado** | Sprint era de cobertura; domínio concurrency não constava do plano |
| **Sugestão** | DDNA: "policy global de locks" — ou learning `#discovery-high` → F9.1 |

---

### MEDIUM-1: ...

### (LOW: log apenas — não entra no resumo)
```

O report é **derivado e idempotente**: pode ser regenerado a qualquer momento com os mesmos dados, sem estado próprio.

---

## 7. Integração com F9.1 — Experience Compiler

```
┌────────────┐   HIGH (sugestão aprovada)   ┌──────────────────────────┐
│  F10.3     │ ───────────────────────────► │  F9.1 Experience        │
│  Discovery │   learning #discovery-high   │  Compiler               │
│  Engine    │                              │                         │
│            │                              │  Fase 1 Coleta:         │
│  Sinaliza  │                              │   lê o discovery report │
│  (não      │                              │   como stream adicional │
│   compila) │                              │  Fase 2 Agrupamento:    │
│            │                              │   #discovery-high ganha │
│            │                              │   prioridade de grupo   │
└────────────┘                              └──────────────────────────┘
```

**Contrato de integração** (anti-duplicação — padrão F2.6 ≠ F9.1):

1. O F10.3 **nunca compila**: ele não cria padrão, não agrupa, não promove a princípio. Ele **marca**.
2. A marca é a tag `#discovery-high` no learning sugerido (ou a referência no discovery report).
3. O F9.1, na Fase 1 (Coleta), lê o discovery report como **stream adicional** de entrada — descobertas HIGH são pré-flagadas como candidatas prováveis a padrão (novelty alta + cross_domain alto = grupo com potencial).
4. A decisão de compilar/promover permanece **inteiramente do F9.1** (maturity, thresholds, cross-agent validation).
5. Coerência com o F2.6 Pattern Evolution: uma descoberta HIGH que o Don aprova vira learning → na 3ª ocorrência nasce padrão candidato → o F9.1 decide maturidade. O F10.3 apenas plantou a semente com prioridade.

**Fluxo completo**: descoberta → sugestão ao Don → Don aprova → learning `#discovery-high` (ou DDNA) → F9.1 compila com prioridade → padrão/princípio → (eventualmente) F9.2 destila → emenda.

---

## 8. Integração com F1.5 — Métricas B1-B5

O F10.3 consome 3 das 5 métricas como **sinais e qualificadores**:

| Métrica | Papel no F10.3 | Como |
|---------|----------------|------|
| **B4 — Cross-agent Reuse** | **Sinal de descoberta (S4)** | `cross-agent-reuse.csv` expõe `source_agents` por task. Se um domínio NÃO planejado aparece como fonte de reuse (ex: learnings de `concurrency` reusados por 3 agentes numa sprint de testing), o conhecimento fluiu sem estar no plano → descoberta. Spike de reuse (> 2× a média do período) num domínio fora do plano = sinal forte. |
| **B3 — Knowledge Freshness** | **Qualificador de validade** | Descoberta exige evidências frescas (`freshness > 0.5` via F1.4). Evidência stale não gera descoberta — gera dívida de conhecimento (F1.6). O report lista a freshness média das evidências de cada HIGH. |
| **B5 — Cognitive Debt** | **Integridade do report** | B5 alto (> 10%) = tasks do período com audit incompleto = aprendizado não registrado = **descobertas perdidas**. O report adverte: "B5 elevado — descobertas podem estar subnotificadas". |
| B1 / B2 | Fora de escopo | Custo e velocidade da task não indicam serendipidade. |

**Direção da integração**: o F10.3 é **consumidor** de B1-B5 — não cria métricas novas, apenas lê os CSVs existentes (mesmo padrão de agregação do F10.1: "lê agregados que já existem"). Nada de coleta própria.

---

## 9. Exemplo Real — Sprint de Cobertura de Testes

Dados reais do repositório (sessões 2026-07-29/30):

**Plano da sprint**: "aumentar cobertura de testes" → domínios planejados: `{testing, quality}`.

**Evidências coletadas no período**:

| Artefato | Tipo | Domínios |
|----------|------|----------|
| L21 — Coverage audit + doc expurgo | learning | testing, documentation |
| L19 — 3 meta-aprendizados da cobertura | learning | testing |
| commit `75d4d99` — "Coverage audit + mass test offensive" | impact report | testing (6 aprendizados) |
| commit `82429cf` — "CLI coverage 46.9% → 71.5%" | impact report | testing |
| L9 — Parallel CI fix (race) | learning | **concurrency**, testing, ci |
| L29 — Onda F0 (21 races) | learning | **concurrency**, testing, architecture |
| commit `c96f4ad` — "5 bloqueantes: panic, **signal race**, MCP, docs, API key" | impact report | **concurrency**, runtime |

**Passo 3 — Divergência detectada (S1 + S3)**: três artefatos do período (L9, L29, `c96f4ad`) tocam o domínio **concurrency**, que **não constava do plano**. Eles se conectam num insight cross-domain: race conditions aparecendo simultaneamente em CI, kernel e runtime.

**Passo 4 — Fórmula de serendipidade**:

```
insight: "race condition é sistêmico em 3 pacotes — precisa de policy global de locks, não fix local"

novelty       = 1 - 0.35 = 0.65
  (as evidências individuais existiam — L9, L21, L29 são fixes pontuais;
   a CONCLUSÃO sistêmica "precisa de policy global" não existia em nenhum learning)

cross_domain  = 4   (testing, concurrency, architecture, ci)

applicability = 0.8 (causa-raiz sistêmica — afeta todo código concorrente futuro)

discovery_value = 0.65 × 4 × 0.8 = 2.08
```

**Passo 5 — Classificação**: `cross_domain 4 ≥ 3` E `2.08 ≥ 1.5` → **🔥 HIGH**.

**Passo 6 — Sugestão ao Don**: DDNA "policy global de locks" OU learning prioritário `#discovery-high` → F9.1 (padrão candidato: "locks devem ser decididos por policy global, não por fix local").

**Aprendizado destilado**: *"race condition é sistêmico, não local — precisa de policy global de locks"* — exatamente o que a sprint de cobertura **não procurava** e encontrou.

---

## 10. Regras de Operação

1. **Execução pós-sprint (ou pós-N tasks, N=10 default)** — alinhado ao cadence de relatório do F1.5 (a cada 10 tasks ou semanal). Nunca em tempo real; serendipidade precisa de janela de observação.

2. **Não busca — observa** — o engine lê exclusivamente dados que já existem. **Nunca cria trabalho extra**: não gera task, não aciona agente, não chama LLM para "inventar" insight. Insight sem evidência em artefato = não existe.

3. **Descobertas são sugestões** — o Don decide se a descoberta vira ação (DDNA, learning, padrão). O F10.3 não executa nada automaticamente. Sem aprovação, a descoberta permanece no report (e decai — ver regra 6).

4. **Sem fabricação** — toda descoberta exige rastro de evidência (paths concretos dos artefatos). Mesmo padrão de rastreabilidade do `compression_path` do F9.2: "de onde veio isto?" tem sempre resposta.

5. **Sem duplicação** — não compilar (F9.1), não destilar (F9.2), não medir gaps (F1.3), não julgar decisões (F8.3). Sinalizar é a única responsabilidade.

6. **Freshness das descobertas** — descoberta não agida em **1 sprint** decai (F1.4) e pode ser descartada no report seguinte. Descoberta velha não é descoberta, é histórico.

7. **"Sem descobertas" é resultado válido** — sprint sem divergência gera report honesto com zero HIGH. Forçar descoberta onde não existe é ruído (anti-padrão, §14).

---

## 11. Arquitetura do Motor

```
┌───────────────────────────────────────────────────────────────┐
│                DISCOVERY ENGINE — ARQUITETURA                 │
│                                                               │
│  ┌─────────────────┐   ┌─────────────────┐                    │
│  │  COLETOR (par.) │   │  PLANO          │                    │
│  │  ler learnings  │   │  parse timeline │                    │
│  │  ler impacts    │   │  parse roadmap  │                    │
│  │  ler failures   │   │  extract domains│                    │
│  │  ler patterns   │   └────────┬────────┘                    │
│  │  ler ddnas      │            │                             │
│  │  ler B1-B5 csv  │            ▼                             │
│  └────────┬────────┘   ┌─────────────────┐                    │
│           └──────────► │  COMPARADOR     │  domínios plano    │
│                        │  sinais S1-S5   │  vs tocados        │
│                        └────────┬────────┘                    │
│                                 ▼                             │
│                        ┌─────────────────┐                    │
│                        │  CLASSIFICADOR  │  fórmula           │
│                        │  HIGH/MED/LOW   │  (novelty×cross×app)│
│                        └────────┬────────┘                    │
│                                 ▼                             │
│                        ┌─────────────────┐                    │
│                        │  REPORT + SUG.  │  discovery report  │
│                        │  (Don decide)   │  #discovery-high   │
│                        └─────────────────┘                    │
└───────────────────────────────────────────────────────────────┘
```

| Característica | Valor |
|----------------|-------|
| **Paradigma** | Agregação + heurísticas (sem LLM, sem I/O de rede) |
| **Estado** | Sem estado próprio — report derivado e idempotente |
| **Tempo alvo** | < 5s (coleta paralela ~2s + comparação ~1s + classificação ~0.5s + report ~0.2s) |
| **Escala** | 54 learnings.md + ~30 timeline entries + 9 impact reports + 5 CSVs |
| **Cache** | Nenhum persistente necessário (roda em batch); re-gerável on-demand |
| **Interfaces** | CLI `cosca discovery run --sprint {id}` · relatório em `memory/timeline/discovery-reports/` |

**Similaridade para novelty**: reutiliza o método do F9.1 (Jaccard de tags + domain match) — consistência entre engines, sem implementação duplicada.

---

## 12. Métricas do Próprio Engine

| Métrica | Definição | Alvo |
|---------|-----------|------|
| `discoveries_per_sprint` | Total de descobertas (todas as classes) | ≥ 1 |
| `discovery_rate` | HIGH / total de descobertas | > 20% (senão o report está ruidoso) |
| `high_per_sprint` | Descobertas HIGH por sprint | ≥ 1 a cada 3 sprints (critério de sucesso da F10) |
| `time_to_action` | Dias entre report e aprovação do Don | < 7 dias (senão F1.4 decai a descoberta) |
| `serendipity_hits` | HIGH que virou DDNA/learning E depois teve efeito positivo (validado via Trust Registry ou impact report posterior) | rastreado acumulativo |
| `evidencia_freshness_avg` | Freshness média (F1.4) das evidências dos HIGH | > 0.5 |

**Critério de sucesso da F10 (next-evolution-phases.md)**: *"Discovery Engine gera pelo menos 1 insight não-planejado em 3 sprints"* → mapeia para `high_per_sprint ≥ 1` a cada 3 sprints.

---

## 13. Implementação e Automação

### 13.1 Fase 1 — Atual (v1.0.0)

- [x] Definição do pipeline de 6 passos com fontes concretas
- [x] Fórmula de serendipidade com componentes derivados
- [x] Classificação HIGH/MEDIUM/LOW com thresholds
- [x] Contratos de integração com F9.1 e F1.5 (B1-B5)
- [x] Exemplo real validado com dados da sprint de cobertura
- [x] Coexistência do diretório (WORKSPACE.md preservado)

### 13.2 Fase 2 — Automação (v1.1.0+)

- [ ] Script `cosca discovery run --sprint {id}` (CLI)
- [ ] Hook pós-sprint no Kernel (trigger: `Kernel.sprint_end` ou contagem de 10 tasks)
- [ ] Tag `#discovery-high` automática nos learnings sugeridos
- [ ] `cosca discovery report --last` para reler o último report

### 13.3 Fase 3 — Preditivo (v2.0.0+)

- [ ] Correlação: "sprints com B4 alto têm 2× mais descobertas HIGH" (alimenta F7.1 Prediction)
- [ ] Sugestão proativa de domínio de exploração para a próxima sprint (baseada em descobertas LOW acumuladas)
- [ ] Valor econômico de descoberta no F10.1 (ativo não-planejado: `serendipity_hits × valor de DDNA`)

---

## 14. Casos de Borda e Anti-Padrões

| Caso | Tratamento |
|------|------------|
| **Sprint sem divergência** | Report honesto com zero HIGH. Não forçar. "Sem descobertas" é resultado válido e informativo (a sprint foi previsível). |
| **Insight útil mas já conhecido** | novelty ≈ 0 → não é descoberta, é reforço. O F9.1 compila normalmente como evidência adicional. Não inflar com applicability alta. |
| **Learning de outro domínio mas trivial** | cross_domain ≥ 3 mas discovery_value < 1.5 → MEDIUM/LOW. Volume de domínios sem valor não é HIGH. |
| **B5 alto no período** | Advertência no report: dívida cognitiva pode estar escondendo descobertas (aprendizado não registrado). Ação: não inventar — registrar o aviso e priorizar a redução de B5 na sprint seguinte. |
| **Descoberta antiga** | Freshness das evidências < 0.5 → descartada ou marcada como histórico. Descoberta velha não gera sugestão. |
| **Descoberta contradiz decisão existente** | Não é papel do F10.3 julgar — mas o report sinaliza a existência de DDNA/learning conflitante e recomenda passar pelo F8.3 Contradiction Engine antes de ação. |
| **Ruído de LOWs** | LOWs vão apenas para log. Don nunca recebe resumo com LOWs — preserva atenção (padrão P2 de notificação do F9.4). |
| **Anti-padrão: caçador ativo** | Criar tasks/agentes para "buscar" descobertas. O F10.3 é observador — busca ativa vira F1.3 (gap detection) ou F9.1 (mineração), não F10.3. |
| **Anti-padrão: LLM sintético** | Pedir ao modelo para "imaginar insights" sem evidência. Proibido — descoberta sem rastro é alucinação. |
| **Anti-padrão: execução automática** | Don não viu → nada acontece. Qualquer automação de ação viola a regra "descobertas são sugestões". |

---

## 15. Referências Cruzadas

| Documento | Relação |
|-----------|---------|
| [experience-compiler/SKILL.md](../experience-compiler/SKILL.md) | F9.1 — consumidor das descobertas HIGH (§7) |
| [cognitive-metrics.md](../../analytics/cognitive-metrics.md) | F1.5 — B4 sinal, B3 qualificador, B5 integridade (§8) |
| [ENGINEERING_TIMELINE.md](../../memory/timeline/ENGINEERING_TIMELINE.md) | Fonte do plano da sprint (Passo 2) |
| [memorize-commit.md](../../workflows/memorize-commit.md) | Gera os impact reports que o Passo 1 consome |
| [WISDOM_DECAY.md](../../memory/WISDOM_DECAY.md) | F1.4 — freshness das evidências (regra 6) |
| [next-evolution-phases.md](../../knowledge/architecture/next-evolution-phases.md) | §F10.3 — spec original + critério de sucesso |
| [cognitive-economics/SKILL.md](../cognitive-economics/SKILL.md) | F10.1 — potencial valor econômico de descobertas (§13.3) |
| [WORKSPACE.md](WORKSPACE.md) | Engine irmã no mesmo diretório — landscape discovery |
| [gap-detection/COGNITIVE.md](../gap-detection/COGNITIVE.md) | F1.3 — complementar (ausência vs presença não-planejada) |
| [contradiction/SKILL.md](../contradiction/SKILL.md) | F8.3 — descoberta conflitante passa pelo filtro adversarial |

---

## 16. Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Architecture Chief | Criação inicial. Definição do Discovery Engine (F10.3): mineração de serendipidade, pipeline de 6 passos pós-sprint, fórmula discovery_value = novelty × cross_domain × applicability, classificação HIGH/MEDIUM/LOW, integrações com F9.1 (Experience Compiler) e F1.5 (B1-B5), exemplo real da sprint de cobertura (race condition sistêmico, discovery_value 2.08), coexistência do diretório com WORKSPACE.md preservado. |
