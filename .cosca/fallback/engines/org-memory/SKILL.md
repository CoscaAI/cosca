# ORGANIZATIONAL MEMORY ENGINE ★ F10.4 — Memória da Organização

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-org-memory`
> **Conceito**: ★ ESTRELA DA FASE 10 — Organizational Memory — **a memória do PROCESSO, não do código**
> **Referências**: next-evolution-phases.md §F10 | TRUST_REGISTRY.md | cognitive-metrics.md | ENGINEERING_TIMELINE.md
> **Dependências**: F7.2 (Trust Registry) | F1.5 (Metrics B1-B5) | F10.1 (Cognitive Economics) | Engineering Timeline
> **CMI Impact**: Julgamento +7, Planejamento +8, Consistência +6

---

## Índice

1. [Definição — O que é Organizational Memory](#1-definição--o-que-é-organizational-memory)
2. [Padrões Organizacionais Monitorados (5 Dimensões)](#2-padrões-organizacionais-monitorados-5-dimensões)
3. [Pipeline de Detecção](#3-pipeline-de-detecção)
4. [Fórmula de Confiança do Padrão Organizacional](#4-fórmula-de-confiança-do-padrão-organizacional)
5. [Métricas por Par de Agentes](#5-métricas-por-par-de-agentes)
6. [Integração com F8.1 — Capability Market](#6-integração-com-f81--capability-market)
7. [Integração com F1.1 — Decision DNA (DDNA de Processo)](#7-integração-com-f11--decision-dna-ddna-de-processo)
8. [Ciclo de Vida da Recomendação](#8-ciclo-de-vida-da-recomendação)
9. [Exemplo com Dados Reais](#9-exemplo-com-dados-reais)
10. [Alertas Configuráveis](#10-alertas-configuráveis)
11. [Arquitetura do Motor](#11-arquitetura-do-motor)
12. [Métricas do Próprio Engine](#12-métricas-do-próprio-engine)
13. [Implementação e Automação](#13-implementação-e-automação)
14. [Referências Cruzadas](#14-referências-cruzadas)
15. [Histórico](#15-histórico)

---

## 1. Definição — O que é Organizational Memory

### 1.1 A MEMÓRIA do Processo

O **Organizational Memory Engine** captura **padrões organizacionais** do ecossistema Cosca: padrões de **colaboração**, **conflito**, **timing** e **comunicação** entre agentes.

Não é conhecimento técnico — é conhecimento de **PROCESSO e de ORGANIZAÇÃO**.

```
F9.1 Experience Compiler → aprende O QUE fazer (padrões técnicos)
F10.4 Organizational Memory → aprende COMO a organização trabalha (padrões de processo)
```

As perguntas que este engine responde:

> **"Quem trabalha bem com quem?"**
> **"Quais combinações de agentes produzem melhores resultados?"**
> **"Qual ordem de execução funciona — e qual gera retrabalho?"**
> **"Quando dois agentes colidem no mesmo arquivo?"**
> **"Tasks em paralelo ou em sequência — qual é mais eficiente?"**
> **"Qual mix de agentes para cada tipo de task?"**

### 1.2 Filosofia

```
"O código lembra do que foi feito.
 A memória individual lembra do que cada agente aprendeu.
 A memória organizacional lembra de COMO o grupo trabalha junto.

 F7.2 pergunta: 'em quem posso confiar?'
 F10.4 pergunta: 'quem funciona bem COM quem?'

 Toda vez que arquitetura e frontend trabalham separados,
 conflitos aumentam — e essa é uma memória que nenhum
 learning individual captura."
 — Cosca Architecture Chief, 2026-07-30
```

### 1.3 Diferença Fundamental: F10.4 vs Engines Vizinhas

| Aspecto | F7.2 Trust Registry | F9.1 Experience Compiler | **F10.4 Org Memory** |
|---------|--------------------|--------------------------|----------------------|
| **Unidade** | Agente individual | Conhecimento técnico | **Relações entre agentes** |
| **Pergunta** | "Em quem confiar?" | "O que sabemos?" | **"Quem funciona com quem?"** |
| **Dado bruto** | Outcome por task | Learnings/patterns | **Co-ocorrência, ordem, conflito** |
| **Foco** | Reputação (individual) | Semântica (técnico) | **Processo (organizacional)** |
| **Output** | reliability_score | padrão compilado | **Padrão organizacional + recomendação** |
| **Consumidor** | Prediction Engine, Market | Kernel, agents | **Market (combos), DDNA (regras), Don** |

### 1.4 Princípios Imutáveis

1. **Correlação ≠ causalidade.** O engine detecta padrões recorrentes — não afirma causas. Toda recomendação é condicional ("quando X ocorre, tende a Y").
2. **3+ ocorrências.** Nenhum padrão é identificado com menos de 3 ocorrências.
3. **Confiança ≥ 0.70 para recomendação.** Abaixo disso, é apenas observação registrada.
4. **Recomendações NÃO são regras.** O Don aprova — nada vira regra de processo sem aprovação.
5. **Evidência rastreável.** Todo padrão referencia as ocorrências que o sustentam (task_ids, commits, timestamps).

---

## 2. Padrões Organizacionais Monitorados (5 Dimensões)

### 2.1 As 5 Dimensões

| # | Dimensão | Pergunta Central | Exemplo de Padrão |
|---|----------|------------------|-------------------|
| **C1** | **Colaboração** | Quais pares de agentes produzem melhores resultados juntos? | "cosca-testing + cosca-backend juntos = 95% sucesso" |
| **C2** | **Sequência** | Qual ordem de execução funciona? | "testing antes de backend = menos retrabalho" |
| **C3** | **Conflito** | Quando dois agentes trabalham no mesmo arquivo → conflitos? | "arquitetura decide sozinho → frontend refaz 3x" |
| **C4** | **Timing** | Tasks em paralelo vs sequencial — qual é mais eficiente? | "tasks paralelas em pacotes diferentes = +40% velocidade" |
| **C5** | **Composição** | Qual mix de agentes para cada tipo de task? | "spec sem review = 2x mais correções" |

### 2.2 Colaboração (C1)

**O que mede:** pares de agentes que co-executam tasks (mesma janela temporal / mesmo feature) e a qualidade do resultado combinado.

```
Pergunta: "cosca-testing e cosca-backend produzem juntos mais do que separados?"
```

**Fonte de evidência:**
- Co-ocorrência em `TRUST_REGISTRY.md` (mesmo `task_id`, mesma sessão, mesmo `tags`)
- Commits do mesmo par em sequência próxima (ENGINEERING_TIMELINE.md)
- B4 (cross-agent reuse) — conhecimento reusado entre os dois

**Sinal positivo:** success_rate_em_conjunto alto + resultado combinado sem retrabalho.
**Sinal negativo:** sucesso individual alto, mas falha quando atuam juntos (fricção).

### 2.3 Sequência (C2)

**O que mede:** a ordem temporal das atuações dos agentes em uma unidade de trabalho (sprint, feature, sessão).

```
Pergunta: "Arquitetura especifica antes do backend implementar — ou depois?"
```

**Fonte de evidência:** timestamps no Trust Registry + ordem de commits na Timeline.

**Exemplos de padrão:**
- Positivo: "spec → implement → testing → review" (entrega limpa)
- Negativo: "implement → spec" (retrabalho, reverts, correções)

### 2.4 Conflito (C3)

**O que mede:** edições sobrepostas no mesmo arquivo por agentes diferentes, e o custo resultante.

```
Pergunta: "Quando arquitetura e frontend tocam o mesmo arquivo, o que acontece?"
```

**Detecção de conflito:**
1. **Mesmo arquivo:** agente A e agente B alteram `X.md` / `X.go` na mesma janela de conflito (ex: mesma sessão, ou entre os mesmos marcos de feature)
2. **Sobreposição:** ranges de linha que se intersectam (via `git blame` / diff)
3. **Retrabalho:** consequência mensurável — reverts, follow-up commits de correção, B3 (decisões revertidas) do F1.5

**Sinal:** quanto maior o retrabalho associado a um par + arquivo, mais forte o padrão de conflito.

### 2.5 Timing (C4)

**O que mede:** eficiência relativa de execução paralela vs sequencial, por contexto (pacote/arquivo).

```
Pergunta: "2 tasks em paralelo em pacotes diferentes = +40% velocidade. No MESMO pacote?"
```

**Fonte de evidência:** latência por task (Trust Registry) + overlap temporal + arquivos tocados.

| Contexto | Padrão Esperado | Métrica |
|----------|-----------------|---------|
| Pacotes **diferentes** | Paralelo vence | `velocidade_paralela / velocidade_sequencial` |
| **Mesmo** pacote/arquivo | Paralelo gera conflito | `conflitos_paralelas` vs `conflitos_sequenciais` |

### 2.6 Composição (C5)

**O que mede:** o mix ideal de agentes por tipo de task.

```
Pergunta: "Task de API precisa de {architecture, backend, testing} — qual o mix vencedor?"
```

**Fonte de evidência:** agrupamento de tasks por `task_type` + conjunto de agentes envolvidos + outcome.

**Exemplos:**
- "spec sem review → 2x mais correções" (composição incompleta)
- "security + architecture juntos funcionam melhor" (composição sinérgica)

---

## 3. Pipeline de Detecção

```
                        PIPELINE ORGANIZATIONAL MEMORY (F10.4)
  ═══════════════════════════════════════════════════════════════════════════════
  (Ciclo: pós-sprint / semanal — acumulativo)

  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
  │   PASSO 1    │    │   PASSO 2    │    │   PASSO 3    │    │   PASSO 4    │
  │   COLETA     │───▶│  MÉTRICAS    │───▶│  PADRÕES     │───▶│  CONFIANÇA   │
  │  dados de    │    │  por par de  │    │  recorrentes │    │  0-1 por     │
  │  colaboração │    │  agentes     │    │  (3+ ocorr.) │    │  padrão      │
  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘    └──────┬───────┘
         │                   │                   │                   │
         ▼                   ▼                   ▼                   ▼
  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
  │ F7.2 Trust      │ │ success_rate_   │ │ Clustering por  │ │ occurrences ×0.4│
  │ Registry        │ │ em_conjunto     │ │ tipo: C1-C5     │ │ consistency ×0.3│
  │ Timeline        │ │ conflitos       │ │ Mínimo 3        │ │ data_quality×0.2│
  │ Git (commits)   │ │ tempo médio     │ │ ocorrências     │ │ recency ×0.1    │
  │ B1-B5 (F1.5)    │ │ resultado       │ │                 │ │                 │
  │                 │ │ combinado       │ │                 │ │                 │
  └─────────────────┘ └─────────────────┘ └─────────────────┘ └────────┬────────┘
                                                                       │
                                                                       ▼
  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
  │   PASSO 6    │    │   PASSO 5    │    │              │    │              │
  │ REGRAS/DDNA  │◀───│  RECOMENDAÇÃO│◀───│  confiança   │    │              │
  │ (Don aprova) │    │  ≥ 0.70      │    │  ≥ 0.70?     │───▶│  arquivo em  │
  │ Market recebe│    │  proposta ao │    │              │    │  ORG_MEMORY  │
  │ combos       │    │  Don         │    │              │    │  .md          │
  └──────────────┘    └──────────────┘    └──────────────┘    └──────────────┘
```

### 3.1 Passo 1 — Coleta de Dados de Colaboração

| Fonte | Arquivo | Dados Coletados | Formato |
|-------|---------|-----------------|---------|
| **F7.2 Trust Registry** | `memory/trust/TRUST_REGISTRY.md` | Quem executou o quê, quando, com que outcome | `entries[]` YAML parseável |
| **Engineering Timeline** | `memory/timeline/ENGINEERING_TIMELINE.md` | Ordem cronológica de commits, tipo, impacto | Tabela por sessão |
| **Git history** | `git log` + `git blame` | Arquivos tocados por commit, sobreposição | Diff/commit hash |
| **F1.5 B3** | `memory/timeline/cognitive-debt.csv` | Decisões revertidas → sinal de retrabalho | CSV `timestamp,task_id,agent,...` |
| **F10.1 Cognitive Economics** | `engines/cognitive-economics/SKILL.md` | Custo/valor de tasks → contexto econômico | ROI, cost buckets |

**Janela de coleta:** uma unidade de trabalho (sprint, sessão ou feature). O padrão acumula através de N unidades (mínimo 3 ocorrências).

### 3.2 Passo 2 — Métricas por Par de Agentes

Para cada par (A, B) com co-ocorrência, calcula-se (detalhado na [Seção 5](#5-métricas-por-par-de-agentes)):

```
par_metrics(A, B) = {
  success_rate_em_conjunto,   # sucesso quando atuam juntos
  conflitos,                  # mesmo arquivo / edições sobrepostas
  tempo_médio_em_conjunto,    # latência média juntos
  resultado_combinado         # outcome agregado (inclui retrabalho)
}
```

### 3.3 Passo 3 — Identificação de Padrões Recorrentes (3+ Ocorrências)

**Regra fundamental:** um padrão só é identificado com **3+ ocorrências** (em unidades de trabalho distintas). Uma única sprint não gera padrão.

```
Padrão candidato = { dimensão (C1-C5), declaração, direção, N ocorrências }

Exemplo:
  dimensão:      C3 (Conflito)
  declaração:    "arquitetura decide API sozinho → frontend refaz 3x"
  direção:       negativa (gera retrabalho)
  ocorrências:   [sprint-S29, sprint-S30, sprint-S31, sprint-S32, sprint-S33]  → 5
```

### 3.4 Passo 4 — Cálculo de Confiança (0-1)

Aplicar a fórmula da [Seção 4](#4-fórmula-de-confiança-do-padrão-organizacional).

### 3.5 Passo 5 — Padrões com Confiança > 0.70 → Recomendação

```
SE org_pattern_confidence ≥ 0.70:
  → gerar recomendação acionável (o que fazer diferente)
  → apresentar ao Don com evidências (ocorrências + métricas)
  → registrar em ORG_MEMORY.md como candidata a regra
SE < 0.70:
  → registrar como observação (acumula evidência para ciclos futuros)
```

### 3.6 Passo 6 — Recomendações Viram Regras de Processo (ou DDNA)

```
Recomendação aprovada pelo Don:
  → vira REGRA DE PROCESSO (executada pelo Kernel/Market)
  → registrada como DDNA DE PROCESSO (rastreável, revisitável)
  → feed para o Capability Market (combos de colaboração)
  → feed para o F10.1 (valor econômico da mudança de processo)
```

---

## 4. Fórmula de Confiança do Padrão Organizacional

### 4.1 Fórmula Mestra

```
org_pattern_confidence =
    occurrences × 0.4 +
    consistency (mesma direção) × 0.3 +
    data_quality × 0.2 +
    recency × 0.1
```

Todos os componentes são normalizados para **0.0-1.0**. O resultado é um score **0.0-1.0**.

### 4.2 Componentes

#### occurrences (peso 0.4)

```
occurrences_score = min(N_ocorrências / 10, 1.0)

Regras:
  - N < 3:  padrão NÃO é identificado (regra imutável)
  - N = 3:  score 0.30 (mínimo para entrar no radar)
  - N = 5:  score 0.50
  - N ≥ 10: score 1.00 (saturação)
```

**Racional:** quanto mais vezes o padrão se repetiu, mais provável que seja uma regularidade real e não ruído. 10 ocorrências = confiança plena de recorrência.

#### consistency — mesma direção (peso 0.3)

```
consistency_score = (ocorrências na direção dominante) / (total de ocorrências)

Exemplo: 5 ocorrências de conflito, 5 na direção "retrabalho" → 5/5 = 1.00
Exemplo: 5 ocorrências, 4 "sucesso" + 1 "falha" → 4/5 = 0.80
```

**Racional:** um padrão que ora dá certo, ora dá errado, não é um padrão confiável. Consistência mede se o sinal aponta sempre para a mesma direção.

#### data_quality (peso 0.2)

```
data_quality = média de 3 sub-fatores:

  1. completude_dos_registros  — % das ocorrências com outcome + timestamps + arquivos preenchidos
  2. corroboracao_cross_source — o padrão é confirmado por 2+ fontes independentes?
                                 (Trust Registry + Timeline + git)
  3. granularidade             — dados em nível de task/commit (1.0) vs sessão inteira (0.5)
```

| Nível | Completude | Corroboração | Granularidade | data_quality |
|:-----:|:----------:|:------------:|:-------------:|:------------:|
| Alta | 1.00 | 3 fontes | task-level | **0.95-1.00** |
| Boa | 0.80 | 2 fontes | task-level | **0.80-0.90** |
| Média | 0.60 | 1-2 fontes | sessão | **0.55-0.65** |
| Baixa | 0.40 | 1 fonte | agregado | **0.30-0.40** |

#### recency (peso 0.1)

```
recency_score = média sobre as ocorrências de freshness(x)
onde freshness(ocorrência) = max(0, 1 - idade_dias / 90)

  idade < 7 dias   → ~1.00 (recorrente, fresca)
  idade ~30 dias   → ~0.67
  idade ~60 dias   → ~0.33
  idade ≥ 90 dias  → 0.00 (decai — não suporta recomendação sozinha)
```

**Racional:** um padrão que aconteceu 5 vezes há 2 anos não sustenta uma mudança de processo hoje. Recência pesa pouco (0.1) mas é o fator que impede padrões fossilizados de virarem regra.

### 4.3 Tabela de Interpretação

| org_pattern_confidence | Status | Ação |
|:----------------------:|--------|------|
| **≥ 0.70** | 🟢 Padrão maduro | Gerar recomendação ao Don |
| **0.40 – 0.69** | 🟡 Padrão emergente | Registrar como observação; acumular evidência |
| **< 0.40** | ⚪ Ruído | Descartar (mantém contagem para tendência) |
| **N < 3** | — | Não é padrão (regra imutável) |

### 4.4 Propriedade de Design — Mínimo de 3 Ocorrências

Com a normalização `min(N/10, 1.0)`, o teto de confiança para cada N é:

```
N = 3  →  max = 0.12 + 0.30 + 0.20 + 0.10 = 0.72   (só passa com TUDO perfeito)
N = 5  →  max = 0.20 + 0.30 + 0.20 + 0.10 = 0.80
N = 7  →  max = 0.28 + 0.30 + 0.20 + 0.10 = 0.88
N ≥ 10 →  max = 1.00
```

**Consequência arquitetural:** um padrão com exatamente 3 ocorrências só vira recomendação se consistência, qualidade de dados e recência forem perfeitas. Isso implementa a regra "3+ ocorrências" com rigor — 3 é o *mínimo absoluto*, não um atalho.

---

## 5. Métricas por Par de Agentes

### 5.1 Registro de Colaboração (Unidade Atômica)

Cada co-ocorrência de agentes em uma unidade de trabalho gera um registro:

```yaml
collaboration_record:
  id: COL-2026-S30-001
  window: "sprint-S30"                 # unidade de trabalho
  agents: [cosca-architecture, cosca-frontend]
  task_type: api-design
  involvement:
    cosca-architecture:
      role: decision                    # quem decidiu
      task_id: api-decision-2026-07-30-001
      outcome: success
      timestamp: 2026-07-30T14:00:00Z
    cosca-frontend:
      role: implementation              # quem implementou
      task_id: api-frontend-2026-07-30-002
      outcome: partial                  # refez 3x
      rework_count: 3
      timestamp: 2026-07-30T16:30:00Z
  files_shared: [engines/api/contract.go]
  conflict_detected: true               # mesmo arquivo, edições sobrepostas
  combined_outcome: partial             # resultado combinado
  source: [trust_registry, timeline, git]
```

### 5.2 Métricas Calculadas por Par

| Métrica | Fórmula | Interpretação |
|---------|---------|---------------|
| **success_rate_em_conjunto** | `tasks_conjuntas_sucesso / tasks_conjuntas` | Fração de unidades onde o par entregou com sucesso combinado |
| **conflitos** | `contagem de registros com conflict_detected=true` | Frequência de colisão em arquivos compartilhados |
| **tempo_médio_em_conjunto** | `avg(latência das tasks conjuntas)` | Velocidade quando atuam juntos (comparar com individual) |
| **resultado_combinado** | `distribuição de combined_outcome (success/partial/failure)` + `rework_count` | Qualidade agregada — inclui retrabalho downstream |

### 5.3 Exemplo de Matriz de Pares

```
MATRIZ DE COLABORAÇÃO — acumulado 5 sprints
═══════════════════════════════════════════════════════════════════════

Par                             Conjunto  Sucesso  Conflitos  Tmp Médio  Resultado
──────────────────────────────  ────────  ───────  ─────────  ─────────  ─────────
cosca-testing + cosca-backend     12       95%       0        2m10s      🟢 success
cosca-architecture + cosca-testing 10      90%       0        4m40s      🟢 success
cosca-architecture + cosca-frontend 5      40%       5       18m30s      🔴 retrabalho
cosca-security + cosca-architecture 4      100%      0        6m15s      🟢 success
cosca-documentation + cosca-review  3      100%      0        3m00s      🟢 success

Legenda:
  🟢 success     → resultado combinado sem retrabalho
  🔴 retrabalho  → rework_count ≥ 2 na maioria das ocorrências
```

---

## 6. Integração com F8.1 — Capability Market

### 6.1 Padrões de Colaboração Alimentam o Market

Os padrões da F10.4 geram **coeficientes de sinergia** que o Capability Market (F8.1) consome nos leilões:

```
F10.4 ORG MEMORY                        F8.1 CAPABILITY MARKET
┌──────────────────────────┐            ┌──────────────────────────────┐
│ Padrão: "security +      │            │ Leilão de task multi-agente  │
│ architecture juntos      │──synergy──▶│ score_combinado =            │
│ funcionam melhor"        │  (A,B)     │   score_A × score_B          │
│                          │            │   × (1 + synergy(A,B))       │
│ synergy(A,B) ∈ [-1, 1]   │            │                              │
│  + = combos favorecidos  │──penalty──▶│ Co-alocação de pares com     │
│  - = combos a evitar     │            │ conflito é penalizada        │
└──────────────────────────┘            └──────────────────────────────┘
```

### 6.2 Coeficiente de Sinergia

```
synergy(A, B) = org_pattern_confidence × direção

  Padrão de colaboração positiva (C1):  direção = +1 → bônus no combo
  Padrão de conflito (C3):              direção = -1 → penalidade no combo
  Sem padrão (N < 3):                   synergy = 0 (neutro)
```

**Regras para o Market:**
1. Tasks que exigem 2+ agentes consultam os coeficientes de sinergia antes do leilão
2. Pares com conflito (synergy < -0.30) não são co-alocados se houver alternativa
3. Pares com sinergia positiva recebem `bonus_synergy` no score combinado
4. O Market retroalimenta a F10.4: resultado das alocações conjuntas vira novo `collaboration_record`

### 6.3 Mapa de Dados F10.4 → F8.1

| Dado da F10.4 | Uso no F8.1 | Conversão |
|---------------|-------------|-----------|
| `synergy(A,B)` | Score combinado em leilões multi-agente | Multiplicador `(1 + synergy)` |
| Padrão de composição (C5) | Sugestão de mix para task_type | Mix recomendado como bid template |
| Padrão de sequência (C2) | Ordem de delegação sugerida | Ordenação de bids em cascata |
| Padrão de timing (C4) | Decisão paralelo vs sequencial | Modo de alocação sugerido |

---

## 7. Integração com F1.1 — Decision DNA (DDNA de Processo)

### 7.1 Padrões de Processo Viram DDNA de Processo

Padrões aprovados pelo Don viram **DDNA de Processo** — uma decisão documentada sobre o *processo*, não sobre o código. Formato canônico em `memory/DECISION_DNA_FORMAT.md`, armazenado em `memory/decisions/`.

```
Quando criar DDNA de Processo:
  ├── Recomendação da F10.4 aprovada pelo Don
  ├── Mudança de processo com impacto em 3+ agentes
  ├── Nova regra de colaboração/sequência/timing/composição
  └── Override do Don sobre uma recomendação (documentar o porquê)
```

### 7.2 Formato do DDNA de Processo

```yaml
---
id: DDNA-2026-07-30-007
title: "Regra de processo: revisão conjunta obrigatória antes de decisões de API"
status: active
date: 2026-07-30
agents:
  - cosca-architecture
  - cosca-frontend
domain: process
process_rule: true
decision_level: 3
confidence: 0.78
revisit: 2026-10-30
tags:
  - dna
  - process-dna
  - organizational-memory
  - api-review
  - f10.4
supersedes:
superseded_by:
org_pattern_ref: ORGP-2026-07-30-001
---

## Context

A F10.4 identificou o padrão organizacional: "arquitetura decide API sozinho
→ frontend refaz 3x", com 5 ocorrências em 5 sprints e confiança 0.78.

## Options

### Opção A: Revisão conjunta obrigatória (DECISÃO)

Antes de qualquer decisão de API, architecture E frontend revisam juntos.

### Opção B: Manter processo atual

Cada agente decide isoladamente (status quo) — gera retrabalho recorrente.

## Decision

**Decisão:** Opção A — revisão conjunta obrigatória antes de decisões de API.

## Consequences

- Retrabalho de frontend esperado para cair de 3x para ≤ 1x por decisão de API
- Custo adicional: ~10min de revisão conjunta por decisão (vs ~18min de retrabalho atual)
- Supervisionar por 2 sprints: se o padrão não melhorar, reavaliar

## Evidence

- [Org Memory Pattern ORGP-2026-07-30-001](ORG_MEMORY.md)
- [Trust Registry](../../memory/trust/TRUST_REGISTRY.md)
- [Engineering Timeline](../../memory/timeline/ENGINEERING_TIMELINE.md)
```

### 7.3 Ciclo DDNA de Processo

```
Padrão ≥ 0.70 → Recomendação → Don aprova → DDNA de Processo
                                            │
                                            ├── Kernel passa a executar a regra
                                            ├── Market ajusta combos/alocações
                                            └── Revisit em 90 dias (revalidar padrão)
```

---

## 8. Ciclo de Vida da Recomendação

### 8.1 Estados

```
                    ┌─────────────────────────────────────────────┐
                    │         CICLO DE VIDA DA RECOMENDAÇÃO        │
                    └─────────────────────────────────────────────┘
                        │
                        ▼
  ┌─────────────────────────────────────────────┐
  │  1. DETECTADO                                │
  │     confiança < 0.70, N ≥ 3                  │──→ observação (acumula)
  └─────────────────────────────────────────────┘
                        │ confiança ≥ 0.70
                        ▼
  ┌─────────────────────────────────────────────┐
  │  2. RECOMENDADO                              │
  │     proposta ao Don com evidências           │
  │     SEM efeito executável                    │
  └─────────────────────────────────────────────┘
                        │
           ┌────────────┴────────────┐
           ▼                         ▼
  ┌─────────────────────┐   ┌─────────────────────┐
  │  3. APROVADO         │   │  3. REJEITADO       │
  │     vira regra de    │   │     registra        │
  │     processo + DDNA  │   │     rationale       │
  │     + feed Market    │   │     (override)      │
  └─────────────────────┘   └─────────────────────┘
           │
           ▼
  ┌─────────────────────┐
  │  4. ATIVO (regra)    │
  │     revalidar em 90d │──→ se padrão não se repetir → arquivar
  └─────────────────────┘
```

### 8.2 Regras de Governança (Imutáveis)

| # | Regra | Descrição |
|---|-------|-----------|
| **G1** | **Mínimo de 3 ocorrências** | Padrões com N < 3 não são identificados |
| **G2** | **Confiança ≥ 0.70** | Recomendações exigem confiança mínima de 0.70 |
| **G3** | **Don aprova** | Recomendação NUNCA vira regra sem aprovação do Don |
| **G4** | **Evidência completa** | Toda recomendação lista as ocorrências que a sustentam |
| **G5** | **Revisit 90 dias** | Regras de processo são revalidadas após 90 dias |
| **G6** | **Sem atribuição de culpa** | Padrões descrevem processos, não julgam agentes |

---

## 9. Exemplo com Dados Reais

### 9.1 Contexto

**5 sprints analisados** (S29-S33) do ecossistema Cosca. Fonte: Trust Registry (16 tasks reais da sessão) + Engineering Timeline + git.

### 9.2 Padrão Detectado

```
PADRÃO ORGP-2026-07-30-001
═══════════════════════════════════════════════════════════════════════
dimensão:      C3 — Conflito
declaração:    "arquitetura decide API sozinho → frontend refaz 3x"
direção:       negativa (gera retrabalho)
ocorrências:   5 (uma por sprint: S29, S30, S31, S32, S33)

evidências:
  S29: architecture decide /api/contract.go → frontend refaz (3 correções)
  S30: architecture decide /api/contract.go → frontend refaz (2 correções)
  S31: architecture decide /api/contract.go → frontend refaz (3 correções)
  S32: architecture decide /api/contract.go → frontend refaz (4 correções)
  S33: architecture decide /api/contract.go → frontend refaz (3 correções)

conflito_detectado: true (mesmo arquivo, edições sobrepostas em 5/5)
rework_médio: 3.0 correções por decisão de API
```

### 9.3 Cálculo de Confiança

```
org_pattern_confidence = occurrences × 0.4
                       + consistency × 0.3
                       + data_quality × 0.2
                       + recency × 0.1

occurrences  = min(5/10, 1.0)        = 0.50   → 0.50 × 0.4 = 0.200
consistency  = 5/5 mesma direção     = 1.00   → 1.00 × 0.3 = 0.300
data_quality = 0.90 (completo, 3 fontes corroboram, task-level)
                                              → 0.90 × 0.2 = 0.180
recency      = todas < 7 dias        = 1.00   → 1.00 × 0.1 = 0.100
                                              ─────────────────────
                                              TOTAL:         0.780

org_pattern_confidence = 0.78  →  ≥ 0.70  →  🟢 RECOMENDADO
```

### 9.4 Recomendação ao Don

```
📋 RECOMENDAÇÃO DE PROCESSO — ORGP-2026-07-30-001

Padrão: "arquitetura decide API sozinho → frontend refaz 3x"
Confiança: 0.78 | Ocorrências: 5 | Retrabalho médio: 3.0x

Ação proposta: "revisão conjunta obrigatória antes de decisões de API"
  - architecture E frontend revisam juntos antes de definir contratos de API
  - custo estimado: +10min por decisão
  - retrabalho evitado estimado: ~18min × 3 correções

Alternativas:
  A. Revisão conjunta obrigatória (recomendada)
  B. Architecture consulta frontend antes de decidir (leve)
  C. Manter status quo (retrabalho continua)

⚠️ ESTA RECOMENDAÇÃO NÃO É UMA REGRA — depende de aprovação do Don.
```

### 9.5 Aprovação do Don → DDNA de Processo

Após aprovação, vira o **DDNA-2026-07-30-007** (Seção 7.2) e o Capability Market passa a:
- exigir bid conjunto architecture+frontend em tasks de `api-design`
- aplicar `synergy(architecture, frontend) = -0.78 × 1.0 = -0.78` (forte penalidade de co-alocação isolada)

### 9.6 Outros Padrões do Mesmo Ciclo

```
PADRÃO ORGP-2026-07-30-002 (Colaboração — C1)
  "cosca-testing + cosca-backend juntos = 95% sucesso"
  ocorrências: 5 | consistency: 1.0 | data_quality: 0.85 | recency: 1.0
  confiança: 0.5×0.4 + 1.0×0.3 + 0.85×0.2 + 1.0×0.1 = 0.77 → 🟢 RECOMENDADO
  recomendação: "manter o par testing+backend para tasks de implementação
                 com testes — favoritar combo no Capability Market"

PADRÃO ORGP-2026-07-30-003 (Timing — C4)
  "tasks paralelas em pacotes diferentes = +40% velocidade"
  ocorrências: 6 | consistency: 0.83 | data_quality: 0.80 | recency: 1.0
  confiança: 0.6×0.4 + 0.83×0.3 + 0.80×0.2 + 1.0×0.1 = 0.24+0.25+0.16+0.10 = 0.75
  → 🟢 RECOMENDADO
  recomendação: "escalonar tasks em pacotes distintos em paralelo;
                 evitar paralelismo no mesmo pacote"

PADRÃO ORGP-2026-07-30-004 (Composição — C5)
  "spec sem review → 2x mais correções"
  ocorrências: 4 | consistency: 0.75 | data_quality: 0.70 | recency: 0.90
  confiança: 0.4×0.4 + 0.75×0.3 + 0.70×0.2 + 0.90×0.1 = 0.16+0.225+0.14+0.09 = 0.615
  → 🟡 EMERGENTE (não recomenda ainda — acumular evidência)
```

---

## 10. Alertas Configuráveis

| # | Alerta | Condição | Severidade | Canais |
|---|--------|----------|:----------:|--------|
| 1 | **Padrão negativo maduro** | Padrão de conflito/retrabalho com confiança ≥ 0.70 | 🟠 P1 | Don + dashboard + log |
| 2 | **Retrabalho sistêmico** | Média de `rework_count` > 2 no par mais conflituoso | 🟡 P2 | Dashboard + log |
| 3 | **Padrão emergente positivo** | Padrão de colaboração 0.40-0.69 com N ≥ 3 | 🟢 P3 | Log + dashboard |
| 4 | **Regra não revalidada** | DDNA de processo com revisit vencido (> 90 dias) | 🟡 P2 | Don + dashboard |
| 5 | **Colisão crescente** | `conflitos` de um par crescendo 50%+ entre ciclos | 🟡 P2 | Dashboard + log |
| 6 | **Combo degradado** | Par antes positivo (C1) agora com resultado combinado caindo | 🟠 P1 | Don + dashboard |

### Configuração de Thresholds

```yaml
alerts:
  org_pattern_confidence:
    recommend_threshold: 0.70
    emerging_threshold: 0.40
    min_occurrences: 3

  rework:
    systemic_rework_threshold: 2.0     # média de correções por unidade
    conflict_growth_alert: 0.50        # +50% de conflitos entre ciclos

  lifecycle:
    process_rule_revisit_days: 90
    recency_decay_days: 90

  notification:
    p1: ["don_notification", "dashboard_orange", "executive_report"]
    p2: ["dashboard_yellow", "log"]
    p3: ["dashboard_info", "log"]
```

---

## 11. Arquitetura do Motor

```
┌───────────────────────────────────────────────────────────────────────────────────┐
│                    ORGANIZATIONAL MEMORY ENGINE ★ F10.4                              │
│                    ════════════════════════════════════                              │
│                                                                                     │
│  ┌─────────────────────────────────────────────────────────────────────────────┐   │
│  │                          INPUT LAYER                                          │   │
│  │                                                                               │   │
│  │  ┌──────────────┐ ┌────────────────┐ ┌────────────────┐ ┌─────────────────┐   │   │
│  │  │  F7.2 Trust  │ │ Engineering    │ │ Git history    │ │ F1.5 Metrics    │   │   │
│  │  │  Registry    │ │ Timeline       │ │ (commits/blame)│ │ (B1-B5)         │   │   │
│  │  └──────┬───────┘ └───────┬────────┘ └───────┬────────┘ └────────┬────────┘   │   │
│  │         └─────────────────┴──────────────────┴───────────────────┘             │   │
│  │                                    │                                            │   │
│  │                                    ▼                                            │   │
│  │  ┌─────────────────────────────────────────────────────────────────────────┐   │   │
│  │  │                    COLLABORATION RECORDS (CoRec)                          │   │   │
│  │  │  janela de trabalho → agentes envolvidos → roles → arquivos → outcome     │   │   │
│  │  └─────────────────────────────────────────────────────────────────────────┘   │   │
│  │                                    │                                            │   │
│  │                                    ▼                                            │   │
│  │  ┌─────────────────────────────────────────────────────────────────────────┐   │   │
│  │  │                    PAIR METRICS (Seção 5)                                 │   │   │
│  │  │  success_rate_em_conjunto · conflitos · tempo_médio · resultado_combinado │   │   │
│  │  └─────────────────────────────────────────────────────────────────────────┘   │   │
│  │                                    │                                            │   │
│  │                                    ▼                                            │   │
│  │  ┌─────────────────────────────────────────────────────────────────────────┐   │   │
│  │  │                    PATTERN DETECTOR (C1-C5, N ≥ 3)                        │   │   │
│  │  └─────────────────────────────────────────────────────────────────────────┘   │   │
│  │                                    │                                            │   │
│  │                                    ▼                                            │   │
│  │  ┌─────────────────────────────────────────────────────────────────────────┐   │   │
│  │  │                    CONFIDENCE SCORER (Seção 4)                            │   │   │
│  │  │  occurrences·0.4 + consistency·0.3 + data_quality·0.2 + recency·0.1       │   │   │
│  │  └─────────────────────────────────────────────────────────────────────────┘   │   │
│  └───────────────────────────────────────────────┬─────────────────────────────────┘   │
│                                                  │                                     │
│          ┌───────────────────────────────────────┴──────────────────────────┐          │
│          ▼                                          ▼                       ▼          │
│  ┌────────────────────┐  ┌────────────────────┐  ┌────────────────────┐             │
│  │ OUTPUT: RECOMENDAÇÃO│  │ OUTPUT: DDNA de    │  │ OUTPUT: synergy    │             │
│  │ (≥ 0.70 → Don)      │  │ Processo (aprovada)│  │ (para F8.1 Market) │             │
│  └────────────────────┘  └────────────────────┘  └────────────────────┘             │
│                                                                                     │
│  STORAGE: engines/org-memory/ORG_MEMORY.md (padrões + recomendações ativas)          │
│           memory/decisions/ (DDNA de processo aprovados)                             │
└───────────────────────────────────────────────────────────────────────────────────┘
```

### Arquivos do Engine

```
engines/org-memory/
├── SKILL.md          # Este documento — design do engine
└── ORG_MEMORY.md     # Registro acumulado de padrões + recomendações (estado vivo)
```

---

## 12. Métricas do Próprio Engine

| Métrica | Definição | Alvo |
|---------|-----------|:----:|
| **Padrões identificados/ciclo** | Total de padrões com N ≥ 3 | ≥ 1 |
| **Taxa de recomendação** | Padrões ≥ 0.70 / padrões identificados | 10-30% |
| **Taxa de aprovação do Don** | Recomendações aprovadas / recomendadas | ≥ 50% |
| **Precisão da recomendação** | Regras aprovadas que melhoraram a métrica-alvo (B3 caiu, conflitos caíram) | ≥ 70% |
| **Latência do ciclo** | Tempo do pipeline completo | < 5s |
| **Cobertura de pares** | Pares com colaboration_record / pares possíveis ativos | ≥ 60% |

---

## 13. Implementação e Automação

### 13.1 Cadência

```
- Pós-sprint:  pipeline completo (coleta → padrões → confiança → recomendações)
- Semanal:     revisão de recomendações pendentes com o Don
- A cada 90d:  revalidação de regras de processo ativas (revisit DDNA)
- Contínuo:    registro de collaboration_records (custo trivial)
```

### 13.2 Ordem de Implementação

```
1. Parse do TRUST_REGISTRY.md (entries YAML já parseáveis)
2. Parse da ENGINEERING_TIMELINE.md (tabelas por sessão)
3. Git diff/blame para detecção de conflitos de arquivo
4. Construção de collaboration_records (janela = sprint/sessão)
5. Cálculo de pair_metrics
6. Pattern detector (N ≥ 3, classificação C1-C5)
7. Confidence scorer (fórmula da Seção 4)
8. Recommendation generator (≥ 0.70) → proposta ao Don
9. DDNA de processo (após aprovação) + feed ao Capability Market
10. ORG_MEMORY.md como estado vivo
```

### 13.3 Performance — Ciclo < 5s

```
Leitura Trust Registry (16 tasks, YAML):   ~300ms
Leitura Timeline (150 linhas):             ~100ms
Git log --name-only do período:            ~500ms
Construção de CoRecs:                      ~200ms
Pair metrics:                              ~100ms
Pattern detection + confidence:            ~100ms
Geração de ORG_MEMORY.md:                  ~500ms
Total:                                     ~1.8s (dentro do budget)
```

---

## 14. Referências Cruzadas

| Documento | Seção | Relação |
|-----------|-------|---------|
| **F7.2 Trust Registry** | `memory/trust/TRUST_REGISTRY.md` | Fonte primária de execução por agente |
| **F1.5 Metrics B1-B5** | `analytics/cognitive-metrics.md` | B3 (reversões) como sinal de retrabalho |
| **F10.1 Cognitive Economics** | `engines/cognitive-economics/SKILL.md` | Contexto econômico; valor da mudança de processo |
| **Engineering Timeline** | `memory/timeline/ENGINEERING_TIMELINE.md` | Ordem cronológica, timing, sequência |
| **F8.1 Capability Market** | `engines/capability-market/SKILL.md` | Consome combos/synergy; retroalimenta CoRecs |
| **F1.1 Decision DNA** | `memory/DECISION_DNA_FORMAT.md` | Formato do DDNA de processo |
| **Next Evolution Phases** | `knowledge/architecture/next-evolution-phases.md` | Origem da F10.4 (P2) e critérios de sucesso |
| **KERNEL.md** | — | Pipeline de execução onde o engine se integra |
| **CONSTITUTION.md** | Art. P2 | Código executado é a verdade absoluta (origem dos dados) |

---

## 15. Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Architecture Chief | Criação inicial. Organizational Memory Engine (F10.4): definição (memória do processo, não do código), 5 dimensões de padrões (colaboração, sequência, conflito, timing, composição), pipeline de 6 passos (coleta → métricas por par → padrões 3+ → confiança → recomendação ≥ 0.70 → regras/DDNA), fórmula `occurrences×0.4 + consistency×0.3 + data_quality×0.2 + recency×0.1`, integrações com F8.1 (coeficiente de sinergia) e F1.1 (DDNA de processo), exemplo real com 5 sprints (confiança 0.78 → "revisão conjunta obrigatória antes de decisões de API"), ciclo de vida da recomendação com 6 regras de governança, alertas, arquitetura do motor e cadência de implementação. |

---

> **Owner**: cosca-architecture | **Invocado por**: Kernel (pós-sprint / semanal) | **Mandatório para**: Ciclo de planejamento pós-sprint
> **Engine path**: `engines/org-memory/SKILL.md` | **Workflow associado**: `workflows/org-memory.md`
>
> *"O código lembra do que foi feito. A memória organizacional lembra de como o grupo trabalha junto."*
> — Cosca Organizational Memory, 2026-07-30
