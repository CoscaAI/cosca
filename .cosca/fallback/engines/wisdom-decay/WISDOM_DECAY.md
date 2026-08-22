# WISDOM DECAY ENGINE — Knowledge Freshness & Deprecation System

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
> **CMI Dimension**: Consistência (+0.04) | **Bloco Cognitivo**: Bloco 1 — Memória & Conhecimento
> **Fase CMI**: Fase 1 — Foundation | **Código**: F1.4
>
> Consulte também:
> - [WISDOM_DECAY.md](../../memory/WISDOM_DECAY.md) — especificação canônica (conceitual, categorias, curva de decaimento)
> - [CONFIDENCE_MODEL.md](../../engines/evidence/CONFIDENCE_MODEL.md) — evidence trust engine (freshness alimenta confidence)
> - [DECISION_DNA.md](../../knowledge/architecture/DECISION_DNA.md) — Decision DNA (depreciação gera DDNA)
> - [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) — formato de aprendizado
> - [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) — C11 Wisdom Decay conceito

---

## Sumário

1. [O que é Wisdom Decay](#1-o-que-é-wisdom-decay)
2. [Mecanismo de Decay (3 tipos)](#2-mecanismo-de-decay-3-tipos)
3. [Pipeline de Execução](#3-pipeline-de-execução)
4. [Fórmula de Freshness Score](#4-fórmula-de-freshness-score)
5. [Gatilhos de Revalidação](#5-gatilhos-de-revalidação)
6. [Integrações](#6-integrações)
7. [Exemplos com Dados Reais](#7-exemplos-com-dados-reais)
8. [CLI e Automação](#8-cli-e-automação)
9. [Relatório de Conhecimento em Risco](#9-relatório-de-conhecimento-em-risco)
10. [Métricas do Engine](#10-métricas-do-engine)

---

## 1. O que é Wisdom Decay

### Definição

**Wisdom Decay** é o mecanismo que gerencia o envelhecimento do conhecimento no ecossistema Cosca. Cada learning registrado em `learnings.md` tem uma **validade intrínseca** — com o tempo, o contexto muda, o código evolui, as tecnologias se atualizam, e o que era verdade ontem pode não ser mais hoje.

O engine calcula um **Freshness Score** (0.0–1.0) para cada learning, baseado em:
- **Recência**: há quanto tempo foi usado pela última vez
- **Frequência**: quantas vezes foi utilizado (aprendizados usados com frequência são mais confiáveis)
- **Consistência**: se contradiz outros aprendizados ou é contradito
- **Revisão**: há quanto tempo foi revisado manualmente

### Propósito

1. **Diferenciar conhecimento vivo de conhecimento morto** — aprendizados recentes e frequentes têm mais peso que antigos e esquecidos
2. **Automatizar depreciação** — conhecimento obsoleto é sinalizado sem intervenção manual
3. **Preservar conhecimento importante** — mesmo que antigo, se usado com frequência, mantém freshness alto
4. **Evitar poluição cognitiva** — aprendizados contraditórios são detectados e resolvidos
5. **Alimentar o Confidence Model** — freshness score é um dos inputs para o EvidenceConfidence de cada learning

### Analogia: Conhecimento como Comida Perecível

```
┌─────────────────────────────────────────────────────────────────┐
│                   KNOWLEDGE FRESHNESS ANALOGY                    │
│                                                                  │
│  Fresco (freshness > 0.7):                                      │
│    🥩 Carne vermelha fresca — pode consumir sem preocupação     │
│                                                                  │
│  Atenção (0.3 ≤ freshness ≤ 0.7):                               │
│    🥩 Carne na geladeira há 5 dias — cheire antes de consumir   │
│    → "Revalidar em 30 dias"                                     │
│                                                                  │
│  Vencendo (0.15 ≤ freshness < 0.3):                             │
│    🥩 Carne na geladeira há 10 dias — provavelmente estragou    │
│    → "Revalidar agora"                                          │
│                                                                  │
│  Estragado (freshness < 0.15):                                  │
│    🥩 Carne com cheiro forte e cor esverdeada — JOGUE FORA     │
│    → "Depreciado — considerar remoção"                          │
│                                                                  │
│  Contaminado (contradição detectada):                           │
│    🥩 Carne crua que tocou frango cru — risco de contaminação  │
│    → "Revisão manual necessária"                                │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Diferença do Esquema de Decaimento Conceitual

O documento [WISDOM_DECAY.md](../../memory/WISDOM_DECAY.md) na pasta `memory/` define a **especificação conceitual** — categorias (CRITICAL/STABLE/EXPERIMENTAL/DEPRECATED), curva de confiança exponencial, e gatilhos de revalidação baseados em `last_validated`.

Este documento na pasta `engines/wisdom-decay/` define a **implementação do engine** — o cálculo de Freshness Score multidimensional, os 3 tipos de decay (time/event/contradiction), o pipeline de varredura, a CLI automatizada, os relatórios, e a integração com outros motores.

---

## 2. Mecanismo de Decay (3 tipos)

O Wisdom Decay Engine opera com 3 mecanismos ortogonais de decaimento. Cada um atua em uma dimensão diferente do knowledge freshness.

### 2.1 Time-Based Decay

**O que é**: Decaimento natural baseado em tempo sem uso. Cada learning perde frescor progressivamente à medida que o tempo desde o último uso aumenta.

**Regra**: A confiança de cada learning decai **10% ao mês** sem uso (a partir de 30 dias de inatividade).

```
decay_rate = 0.10  // 10% por mês
months_since_last_use = days_since_last_use / 30

time_penalty = min(months_since_last_use × decay_rate, 1.0)
```

**Exceções**:
- Aprendizados com `last_used` nos últimos 30 dias: sem penalidade (freshness = 1.0 no componente temporal)
- Aprendizados marcados como `#immutable` ou `#constitutional`: decay rate reduzido para 2% ao mês
- Aprendizados com `usage_count ≥ 50`: decay rate reduzido para 5% ao mês (conhecimento consolidado)

### 2.2 Event-Based Decay

**O que é**: Eventos externos — como lançamento de nova tecnologia, atualização de provider, mudança de stack — disparam **zeramento ou redução drástica** da confiança de aprendizados relacionados.

**Eventos monitorados**:

| Evento | Gatilho | Ação |
|--------|---------|------|
| **Nova tecnologia** | Dependência adicionada em `go.mod` ou `package.json` | Zera freshness de learnings que mencionam tecnologia anterior |
| **Novo provider** | Provider adicionado em `internal/providers/` | Freshness de provider anterior cai 50% |
| **Breaking change** | Release note com `breaking` ou `BREAKING` | Learnings que mencionam API antiga têm freshness zerado |
| **Depreciação externa** | Dependência marcada como deprecated no ecossistema | Learnings sobre aquela dependência: freshness -0.5 |
| **Stack migration** | ADR aprovada que define migração de stack | Learnings da stack antiga: freshness -0.7 |
| **Nova versão do Go/Node** | `go.mod` ou `package.json` atualizado | Learnings que usam recursos deprecated da versão anterior: freshness cai 30% |

**Mecanismo de matching**: tags do learning são comparadas com o evento. Quanto mais tags em comum, maior o impacto.

```python
def event_decay(learning_tags, event_tags):
    overlap = len(set(learning_tags) & set(event_tags))
    relevance = overlap / max(len(event_tags), 1)
    if relevance > 0.5:
        return 0.0  # Zera freshness
    elif relevance > 0.2:
        return 0.5  # Reduz 50%
    else:
        return 1.0  # Sem impacto
```

### 2.3 Contradiction Decay

**O que é**: Quando dois aprendizados se contradizem, ambos perdem confiança. O mecanismo detecta contradições via matching semântico de tags e conteúdo.

**Detecção**:
1. Dois aprendizados têm tags em comum > 60%
2. O campo `Learned` de um contradiz o do outro (detecção por sentimento/afirmação oposta)
3. Ambos têm freshness > 0.3 (se um já está depreciado, o outro não é penalizado)

**Impacto**:

```
contradiction_penalty:
  primeira contradição detectada: -0.20 cada
  segunda contradição: -0.30 cada
  terceira+: -0.50 cada (escalada para revisão manual)
```

**Exemplo real**: Um learning diz "runtime test coverage is 78.3%" e outro diz "runtime is at 97.9%". Eles contradizem -> ambos entram em contradição decay:

```
L18: "runtime está em 97.9%"
L21: "runtime precisava de 78.3%"
→ Contradição detectada (mesmo domínio: runtime coverage)
→ Ambos perdem -0.20 freshness
→ "Revisão manual necessária"
```

---

## 3. Pipeline de Execução

O pipeline roda em duas modalidades:

- **Agendada**: a cada 50 tasks executadas OU semanalmente (o que ocorrer primeiro)
- **Manual**: via CLI `cosca wisdom-decay run`

### Fluxo Completo

```
┌──────────────────────────────────────────────────────────────────┐
│                  WISDOM DECAY PIPELINE                            │
│                                                                  │
│   TRIGGER:                                                        │
│   ┌──────────────┐    ┌──────────────────┐                       │
│   │ Task counter │    │ Scheduler (cron) │                       │
│   │ ≥ 50 tasks   │    │ Semanal          │                       │
│   └──────┬───────┘    └────────┬─────────┘                       │
│          │                     │                                  │
│          └──────────┬──────────┘                                  │
│                     ▼                                             │
│   ┌─────────────────────────────────────┐                        │
│   │ 1. ENGINE VARRE learnings.md       │                        │
│   │    de todos os 54 agentes           │                        │
│   │    + patterns.md e failures.md     │                        │
│   │    Tempo alvo: < 10s               │                        │
│   └────────────────┬────────────────────┘                        │
│                    ▼                                              │
│   ┌─────────────────────────────────────┐                        │
│   │ 2. PARA CADA LEARNING:              │                        │
│   │    a. Carrega metadados             │                        │
│   │    b. Calcula freshness_score       │                        │
│   │    c. Aplica decay types (3)        │                        │
│   │    d. Registra score parcial        │                        │
│   └────────────────┬────────────────────┘                        │
│                    ▼                                              │
│   ┌─────────────────────────────────────┐                        │
│   │ 3. CLASSIFICA POR THRESHOLD        │                        │
│   │                                    │                        │
│   │  freshness ≥ 0.7  → HEALTHY        │                        │
│   │  0.3 ≤ f < 0.7   → REVALIDATE_30   │                        │
│   │  0.15 ≤ f < 0.3  → REVALIDATE_NOW  │                        │
│   │  f < 0.15        → DEPRECATED       │                        │
│   │  contradição     → CONFLICT         │                        │
│   └────────────────┬────────────────────┘                        │
│                    ▼                                              │
│   ┌─────────────────────────────────────┐                        │
│   │ 4. AÇÕES AUTOMÁTICAS:              │                        │
│   │    - REVALIDATE_30: add tag         │                        │
│   │      #revalidate-by-{date}          │                        │
│   │    - REVALIDATE_NOW: agenda task    │                        │
│   │    - DEPRECATED: aplica             │                        │
│   │      #deprecated, move para audit   │                        │
│   │    - CONFLICT: cria DDNA de         │                        │
│   │      contradição, escala p/ Don     │                        │
│   └────────────────┬────────────────────┘                        │
│                    ▼                                              │
│   ┌─────────────────────────────────────┐                        │
│   │ 5. GERA RELATÓRIO                   │                        │
│   │    "Conhecimento em Risco"          │                        │
│   │    → audit/decay-report-{date}.md   │                        │
│   └────────────────┬────────────────────┘                        │
│                    ▼                                              │
│   ┌─────────────────────────────────────┐                        │
│   │ 6. ATUALIZA MÉTRICAS               │                        │
│   │    - Freshness médio do sistema     │                        │
│   │    - Taxa de depreciação            │                        │
│   │    - Learnings em risco             │                        │
│   └─────────────────────────────────────┘                        │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### Algoritmo de Varredura

```
function run_decay_pipeline():
    // Fase 1: Coleta
    all_learnings = []
    for agent_path in glob("memory/agent/*/learnings.md"):
        learnings = parse_learnings_file(agent_path)
        all_learnings.extend(learnings)
    
    // Fase 2: Cálculo
    contradictions = detect_contradictions(all_learnings)
    
    for learning in all_learnings:
        time_score = calc_time_decay(learning)
        event_score = calc_event_decay(learning, recent_events)
        contradiction_penalty = get_contradiction_penalty(learning, contradictions)
        
        learning.freshness = calc_freshness(learning, time_score, event_score, contradiction_penalty)
        learning.decay_type = classify_decay(learning.freshness)
        learning.next_action = get_action(learning.decay_type)
    
    // Fase 3: Relatório
    report = generate_report(all_learnings, contradictions)
    save_decay_report(report)
    update_agent_learnings(all_learnings)
    
    return report
```

### Performance

| Operação | Tempo Estimado |
|----------|----------------|
| Varredura de 54 agentes (parse) | < 2s |
| Cálculo de freshness (500+ learnings) | < 1s |
| Detecção de contradições | < 3s |
| Geração de relatório | < 1s |
| **Total** | **< 7s** |

---

## 4. Fórmula de Freshness Score

### Fórmula Completa

```
freshness = recency_term × 0.4
          + usage_term   × 0.3
          + consistency_term × 0.2
          + review_term  × 0.1
```

### Componentes Detalhados

#### 4.1 Recency Term (peso 0.4)

Mede o quão recentemente o learning foi utilizado.

```
recency_term = 1 - min(days_since_last_use / 365, 1.0)

Onde:
  days_since_last_use = NOW - last_use_date  (data do último uso documentado)
  
  Se last_use_date não existe → usa creation_date
  Se days_since_last_use ≤ 30 → recency_term = 1.0 (sem decay por uso recente)
```

**Comportamento**:
| Dias sem uso | recency_term | Interpretação |
|-------------|-------------|---------------|
| 0-30 | 1.00 | Uso recente, fresco |
| 90 | 0.75 | 3 meses sem usar |
| 180 | 0.51 | 6 meses sem usar |
| 365 | 0.00 | 1 ano sem usar → decay total |

#### 4.2 Usage Term (peso 0.3)

Mede a frequência de uso acumulada. Quanto mais usado, mais confiável.

```
usage_term = min(usage_count / 20, 1.0)

Onde:
  usage_count = número de vezes que o learning foi referenciado/usado
  Cap: 20 usos = 1.0 (satura — usar mais não aumenta)
```

**Comportamento**:
| usages | usage_term | Interpretação |
|--------|-----------|---------------|
| 0 | 0.00 | Nunca usado — sem evidência |
| 1 | 0.05 | Usado 1x |
| 5 | 0.25 | Uso moderado |
| 10 | 0.50 | Uso frequente |
| 20+ | 1.00 | Padrão consolidado |

#### 4.3 Consistency Term (peso 0.2)

Mede o quão consistente o learning é com outros aprendizados.

```
consistency_term = max(0, 1 - min(contradiction_count / 5, 1.0))

Onde:
  contradiction_count = número de contradições ativas envolvendo este learning
  Cap: 5 contradições → termo = 0.0
```

**Comportamento**:
| Contradições | consistency_term | Interpretação |
|-------------|-----------------|---------------|
| 0 | 1.00 | Consistente — sem conflitos |
| 1 | 0.80 | Uma contradição conhecida |
| 3 | 0.40 | Múltiplas contradições |
| 5+ | 0.00 | Completamente inconsistente |

#### 4.4 Review Term (peso 0.1)

Mede o quão recente foi a última revisão manual.

```
review_term = 1 - min(days_since_last_review / 180, 1.0)

Onde:
  days_since_last_review = NOW - last_review_date
  
  Se last_review_date não existe → usa creation_date
  Se days_since_last_review ≤ 30 → review_term = 1.0
```

**Comportamento**:
| Dias sem revisão | review_term | Interpretação |
|-----------------|-------------|---------------|
| 0-30 | 1.00 | Revisão recente |
| 90 | 0.50 | 3 meses sem revisar |
| 180+ | 0.00 | 6+ meses sem revisão |

### Exemplo de Cálculo

```
Learning: "Jail bypass recovery — UCSS protection rules" (L13)
last_use: 2026-07-30 (today: 2026-07-30 → 0 days)
usage_count: 8
contradiction_count: 0
last_review: 2026-07-30

recency_term = 1 - 0/365 = 1.000 × 0.4 = 0.400
usage_term   = min(8/20, 1) = 0.400 × 0.3 = 0.120
consistency_term = max(0, 1 - 0/5) = 1.000 × 0.2 = 0.200
review_term  = 1 - 0/180 = 1.000 × 0.1 = 0.100

freshness = 0.400 + 0.120 + 0.200 + 0.100 = 0.820 → HEALTHY ✅
```

---

## 5. Gatilhos de Revalidação

### Thresholds

| Freshness | Classificação | Ação | Label |
|-----------|--------------|------|-------|
| **≥ 0.70** | HEALTHY | Nenhuma ação necessária | 🟢 Fresco |
| **0.30 ≤ f < 0.70** | REVALIDATE_30 | Marcar para revalidação em 30 dias | 🟡 Atenção |
| **0.15 ≤ f < 0.30** | REVALIDATE_NOW | Revalidar imediatamente | 🟠 Crítico |
| **< 0.15** | DEPRECATED | Depreciar automaticamente | 🔴 Estragado |
| **Contradição detectada** | CONFLICT | Revisão manual necessária | ⚡ Contradição |

### Matriz de Ações

```
┌────────────────────┬──────────────────┬────────────────────┬────────────┐
│ Classificação      │ Tag Adicionada   │ Ação Automática    │ Notifica   │
├────────────────────┼──────────────────┼────────────────────┼────────────┤
│ HEALTHY            │ #fresh           │ Nenhuma            │ Não        │
│ REVALIDATE_30      │ #revalidate-by-  │ Agendar task para  │ Agent      │
│                    │ {date+30d}       │ 30 dias            │ owner      │
│ REVALIDATE_NOW     │ #stale           │ Criar task de      │ Agent +    │
│                    │                  │ revalidação imediata│ Don       │
│ DEPRECATED         │ #deprecated      │ Mover para         │ Don +      │
│                    │                  │ audit/expired-     │ Memory    │
│                    │                  │ entries.md         │ Chief     │
│ CONFLICT           │ #conflict        │ Bloquear uso até   │ Don        │
│                    │                  │ resolução manual   │ urgente    │
└────────────────────┴──────────────────┴────────────────────┴────────────┘
```

### Gatilhos Específicos

#### Freshness < 0.30 → "Revalidar em 30 dias"
- Adicionar tag `#revalidate-by-{YYYY-MM-DD + 30}`
- Agendar task no agente dono do learning
- Prioridade: baixa (executa quando o agente for acionado novamente)
- Se após 30 dias o freshness não melhorou → sobe para REVALIDATE_NOW

#### Freshness < 0.20 → "Revalidar agora"
- Adicionar tag `#stale`
- Criar task de revalidação para o agente dono
- Prioridade: alta (executa na próxima oportunidade)
- Don é notificado via relatório semanal

#### Freshness < 0.15 → "Depreciado — considerar remoção"
- Adicionar tag `#deprecated`
- Mover entrada para `audit/expired-entries.md`
- Criar DDNA de depreciação (ver Integrações §6.1)
- Don é notificado imediatamente
- Learning é excluído do carregamento de contexto

#### Contradição detectada → "Revisão manual necessária"
- Adicionar tag `#conflict`
- Bloquear ambos os aprendizados (não podem ser usados)
- Gerar DDNA de contradição com os dois lados
- Don é notificado para revisão manual
- Após resolução, um é mantido, o outro depreciado

---

## 6. Integrações

### 6.1 F1.1 — Decision DNA (DDNA)

Learning depreciado gera um DDNA de depreciação automaticamente:

```yaml
---
id: DDNA-DECAY-YYYY-MM-DD-NNN
title: "Depreciação automática: {learning_title}"
status: accepted
date: YYYY-MM-DD
agents:
  - cosca-wisdom-decay-engine
domain: knowledge-health
decision_level: 2
confidence: 0.80
revisit: YYYY-MM-DD + 180
tags:
  - dna
  - wisdom-decay
  - deprecation
  - auto-generated
supersedes:
superseded_by: null
---

## Context

O Wisdom Decay Engine detectou que o learning abaixo atingiu freshness < 0.15,
sendo automaticamente depreciado:

- **Agent**: cosca-{agent-name}
- **Learning ID**: {timestamp} — {title}
- **Freshness Score**: {score}
- **Days since last use**: {days}
- **Contradictions**: {count}

## Reason

{componente_que_mais_contribuiu_para_o_decay}

## Decision

Depreciar automaticamente. Learning preservado em audit/expired-entries.md
para referência histórica.

## Consequences

Learning removido do contexto ativo dos agentes.
```

### 6.2 CONFIDENCE_MODEL.md

O **Freshness Score** do Wisdom Decay alimenta o **EvidenceConfidence** do Confidence Model como um modifier adicional:

```
EvidenceConfidence = clamp(base_weight + sum(modifiers) + freshness_modifier, 0.0, 1.0)

Onde:
  freshness_modifier = (freshness - 0.5) × 0.3
  // freshness 0.0 → modifier -0.15, freshness 1.0 → modifier +0.15
```

**Na prática**:

| Freshness | Modifier | Effect on EvidenceConfidence |
|-----------|----------|------------------------------|
| 1.00 (recém usado e muito usado) | +0.15 | Aumenta confiança |
| 0.82 (saudável) | +0.10 | Aumenta ligeiramente |
| 0.50 (neutro) | +0.00 | Neutro |
| 0.20 (stale) | -0.09 | Reduz confiança |
| 0.00 (depreciado) | -0.15 | Reduz fortemente |

**Integração no pipeline de metacognição**:

```
RETRIEVE MEMORY stage:
  → Weight search results by EvidenceConfidence × freshness
  → Learnings com freshness < 0.15 são excluídos dos resultados
  → Learnings com freshness < 0.30 têm peso reduzido em 50%

PLAN STRATEGY stage:
  → Rejeitar planos baseados em aprendizados com freshness < 0.30
  → Alertar se mais de 30% das fontes de um plano têm freshness < 0.70
```

### 6.3 F1.5 — B3 Knowledge Freshness

A métrica **B3 — Reused Knowledge** do Cognitive Maturity (F1.5) é estendida com o Freshness Score médio dos aprendizados consultados.

```
B3_knowledge_freshness = avg(freshness_score for all learnings used in task)

Onde:
  Para cada task executada, calcula-se a média do freshness dos
  aprendizados referenciados.
  
  Thresholds:
    avg_freshness ≥ 0.70 → 🟢 Conhecimento saudável
    0.40 ≤ avg_freshness < 0.70 → 🟡 Conhecimento parcialmente desatualizado
    avg_freshness < 0.40 → 🔴 Conhecimento predominantemente stale
```

**Série temporal**: O B3 Knowledge Freshness é registrado por task e agregado semanalmente, gerando uma curva de saúde do conhecimento:

```
┌─────────────────────────────────────────────────────────────────┐
│  B3 KNOWLEDGE FRESHNESS — SÉRIE HISTÓRICA                       │
│                                                                  │
│  Freshness                                                       │
│  1.00 ┤    ●─●──●                                               │
│  0.80 ┤   /        ●──●──●──●────●──●                           │
│  0.60 ┤  ●                              ●──●────●               │
│  0.40 ┤                                        ●──●             │
│  0.20 ┤                                                         │
│  0.00 ┼─────────────────────────────────────────────────────►  │
│       Semana 1  Semana 2  Semana 3  Semana 4  Semana 5         │
│                                                                  │
│  Insight: Tendência de queda indica que aprendizados            │
│  não estão sendo revalidados. Disparar revalidação em massa.     │
└─────────────────────────────────────────────────────────────────┘
```

### 6.4 Memory Curation Engine

O Freshness Score é o principal input para o **CurationScore**:

```
CurationScore = freshness × 0.5 + importance × 0.3 + cross_references × 0.2
```

Aprendizados com freshness < 0.15 são candidatos prioritários para remoção na curadoria.

---

## 7. Exemplos com Dados Reais

Abaixo, calculamos o Freshness Score hipotético de 3 aprendizados reais do Kernel (L17, L22, L29).

### L17 — Token Bloat Audit (2026-07-29)

**Dados do learning**:

| Campo | Valor |
|-------|-------|
| **Timestamp** | 2026-07-29 |
| **Agent** | cosca-kernel |
| **Level** | 4 |
| **Confidence** | 0.95 |
| **Tags** | #token-bloat #audit #performance #limits #metacognition #jail #the-fix-problem #context-loading |
| **Related** | usado internamente em decisões de arquitetura |
| **Outcome** | success |

**Parâmetros para o cálculo (data base: 2026-07-30)**:

| Parâmetro | Valor | Justificativa |
|-----------|-------|---------------|
| `days_since_last_use` | 1 dia | Usado em L22 e L23 (arquitetura cognitiva) |
| `usage_count` | 5 | Referenciado em L22 (cognitive maturity), L23 (F1 impl), pipeline de metacognição |
| `contradiction_count` | 0 | Nenhuma contradição conhecida |
| `days_since_last_review` | 1 dia | Criado em 2026-07-29, revisado em 2026-07-30 |

**Cálculo**:

```
recency_term = 1 - min(1/365, 1.0) = 1 - 0.003 = 0.997
              × 0.4 = 0.399

usage_term   = min(5/20, 1.0) = 0.250
              × 0.3 = 0.075

consistency_term = max(0, 1 - min(0/5, 1.0)) = 1.000
                  × 0.2 = 0.200

review_term  = 1 - min(1/180, 1.0) = 1 - 0.006 = 0.994
              × 0.1 = 0.099

freshness = 0.399 + 0.075 + 0.200 + 0.099 = 0.773
```

**Resultado: 0.77 → HEALTHY** 🟢

Análise: Aprendizado muito recente, usado com frequência moderada, sem contradições. O freshness alto reflete que L17 é um dos aprendizados mais citados em decisões subsequentes.

---

### L22 — Cognitive Maturity Architecture (2026-07-30)

**Dados do learning**:

| Campo | Valor |
|-------|-------|
| **Timestamp** | 2026-07-30 |
| **Agent** | cosca-kernel |
| **Level** | 4 |
| **Confidence** | 0.93 |
| **Tags** | #cognitive-maturity #cmi #architecture #judgment-engine #cognitive-economy #capability-analysis #workflow #level-4 #metacognition #don-order |
| **Related** | COGNITIVE_MATURITY.md, cognitive-maturity-implementation.md |
| **Outcome** | success |

**Parâmetros para o cálculo (data base: 2026-07-30)**:

| Parâmetro | Valor | Justificativa |
|-----------|-------|---------------|
| `days_since_last_use` | 0 dias | Criado hoje e usado nas fases seguintes |
| `usage_count` | 3 | Citado em L23 (Fase 1 execution), Fase 2, Fase 3 |
| `contradiction_count` | 0 | Nenhuma contradição |
| `days_since_last_review` | 0 dias | Criado hoje |

**Cálculo**:

```
recency_term = 1 - min(0/365, 1.0) = 1.000
              × 0.4 = 0.400

usage_term   = min(3/20, 1.0) = 0.150
              × 0.3 = 0.045

consistency_term = max(0, 1 - min(0/5, 1.0)) = 1.000
                  × 0.2 = 0.200

review_term  = 1 - min(0/180, 1.0) = 1.000
              × 0.1 = 0.100

freshness = 0.400 + 0.045 + 0.200 + 0.100 = 0.745
```

**Resultado: 0.75 → HEALTHY** 🟢

Análise: Extremamente recente (criado hoje), mas ainda pouco usado (só 3 referências). O freshness é dominado pelo componente temporal. Se ficar 3 meses sem ser usado, cairá para ~0.40.

---

### L29 — Onda F0 — Operação Cross-Agent Massiva (2026-07-30)

**Dados do learning**:

| Campo | Valor |
|-------|-------|
| **Timestamp** | 2026-07-30 |
| **Agent** | cosca-kernel |
| **Level** | 4 |
| **Confidence** | 0.94 |
| **Tags** | #f0 #dead-code-removal #race-conditions #vector-store #cors #ci-gate #p0-barriers #cross-agent #level-4 #onda-massiva |
| **Related** | L20, L21, L28, next-evolution-phases.md |
| **Outcome** | success |

**Parâmetros para o cálculo (data base: 2026-07-30)**:

| Parâmetro | Valor | Justificativa |
|-----------|-------|---------------|
| `days_since_last_use` | 0 dias | Criado hoje |
| `usage_count` | 2 | Citado em L30 (continuação) e no relatório F0 |
| `contradiction_count` | 0 | Nenhuma contradição |
| `days_since_last_review` | 0 dias | Criado hoje |

**Cálculo**:

```
recency_term = 1 - min(0/365, 1.0) = 1.000
              × 0.4 = 0.400

usage_term   = min(2/20, 1.0) = 0.100
              × 0.3 = 0.030

consistency_term = max(0, 1 - min(0/5, 1.0)) = 1.000
                  × 0.2 = 0.200

review_term  = 1 - min(0/180, 1.0) = 1.000
              × 0.1 = 0.100

freshness = 0.400 + 0.030 + 0.200 + 0.100 = 0.730
```

**Resultado: 0.73 → HEALTHY** 🟢

Análise: Muito recente, mas usage_count baixo (2 usos, só L30). O freshness é puxado pelo recency term. Sem uso por 30 dias cairia para ~0.55, por 90 dias para ~0.25 (REVALIDATE_NOW).

---

### Tabela Comparativa

| Learning | recency | usage | consistency | review | Freshness | Status |
|----------|---------|-------|-------------|--------|-----------|--------|
| L17 (Token Bloat) | 0.997 | 0.250 | 1.000 | 0.994 | **0.77** | 🟢 HEALTHY |
| L22 (CMI Arch) | 1.000 | 0.150 | 1.000 | 1.000 | **0.75** | 🟢 HEALTHY |
| L29 (Onda F0) | 1.000 | 0.100 | 1.000 | 1.000 | **0.73** | 🟢 HEALTHY |

### Projeção: L17 em 2026-10-30 (90 dias sem uso)

Se L17 não for usado por 90 dias:

```
recency_term = 1 - min(90/365, 1.0) = 1 - 0.247 = 0.753
              × 0.4 = 0.301

usage_term = 0.250 × 0.3 = 0.075 (não muda — uso acumulado não decai)

consistency_term = 1.000 × 0.2 = 0.200

review_term = 1 - min(90/180, 1.0) = 1 - 0.500 = 0.500
              × 0.1 = 0.050

freshness = 0.301 + 0.075 + 0.200 + 0.050 = 0.626
```

**Futuro: 0.63 → REVALIDATE_30** 🟡

A queda de 0.77 para 0.63 em 90 dias mostra que L17 entraria em zona de atenção. Ações:
- Adicionar tag `#revalidate-by-2026-11-29`
- Agendar revalidação

---

## 8. CLI e Automação

O engine expõe uma CLI para operação manual e integração com schedulers.

### Comandos

```bash
# Executar pipeline completo
cosca wisdom-decay run

# Executar apenas para um agente específico
cosca wisdom-decay run --agent cosca-kernel

# Executar em modo dry-run (não modifica arquivos)
cosca wisdom-decay run --dry-run

# Ver relatório do último pipeline
cosca wisdom-decay report

# Ver relatório de conhecimento em risco
cosca wisdom-decay risk-report

# Ver freshness de um learning específico
cosca wisdom-decay inspect <learning-timestamp>

# Forçar revalidação de um learning
cosca wisdom-decay revalidate <learning-timestamp>

# Listar aprendizados por freshness status
cosca wisdom-decay list --status healthy
cosca wisdom-decay list --status stale
cosca wisdom-decay list --status deprecated
cosca wisdom-decay list --status conflict

# Detectar contradições manualmente
cosca wisdom-decay contradictions

# Configurar scheduler
cosca wisdom-decay schedule --interval weekly
cosca wisdom-decay schedule --interval 50-tasks
```

### Opções Globais

| Flag | Descrição |
|------|-----------|
| `--dry-run` | Não modifica arquivos, apenas calcula e reporta |
| `--agent` | Filtra por agente específico |
| `--tag` | Filtra por tag (ex: `--tag #runtime`) |
| `--min-freshness` | Threshold mínimo (default: 0.30) |
| `--output` | Formato do relatório (table/json/yaml) |
| `--verbose` | Log detalhado de cada etapa |

### Scheduler

O pipeline pode ser agendado de duas formas:

#### Via Task Counter (recomendado)

O Memory Chief incrementa um contador a cada task executada. Quando o contador atinge 50, o pipeline é disparado automaticamente.

```yaml
# Configuração no opencode.json
wisdom_decay:
  trigger:
    type: task_counter
    threshold: 50
    cooldown: 3600  # 1 hora entre execuções
```

#### Via Cron (alternativa)

```bash
# Executar semanalmente (domingo à meia-noite)
0 0 * * 0 cosca wisdom-decay run --output json >> /var/log/wisdom-decay.log
```

---

## 9. Relatório de Conhecimento em Risco

### Formato do Relatório

Após cada execução do pipeline, um relatório é gerado em `audit/decay-report-{YYYY-MM-DD}.md`:

```markdown
# Wisdom Decay Report — 2026-07-30

## Summary

| Metric | Value |
|--------|-------|
| Total Learnings Analyzed | 30 |
| Agents Scanned | 54 |
| 🟢 HEALTHY (f ≥ 0.70) | 28 (93.3%) |
| 🟡 REVALIDATE_30 (0.30 ≤ f < 0.70) | 2 (6.7%) |
| 🟠 REVALIDATE_NOW (0.15 ≤ f < 0.30) | 0 (0.0%) |
| 🔴 DEPRECATED (f < 0.15) | 0 (0.0%) |
| ⚡ CONFLICT (contradiction) | 0 (0.0%) |
| System Avg Freshness | 0.89 |

## Knowledge at Risk

### 🟡 Revalidar em 30 dias

| Learning | Agent | Freshness | Days Since Use | Usage Count |
|----------|-------|-----------|----------------|-------------|
| L15 — Auto-Jail | cosca-kernel | 0.52 | 1 | 1 |
| 2026-07-28 — Onda 2 | cosca-kernel | 0.48 | 2 | 1 |

### Freshness Distribution

```
HEALTHY (≥ 0.70)   ████████████████████████████████ 28
REVALIDATE_30      ██                                  2
REVALIDATE_NOW      (0)
DEPRECATED          (0)
CONFLICT            (0)
```

### Breakdown by Agent

| Agent | Learnings | Avg Freshness | Status |
|-------|-----------|---------------|--------|
| cosca-kernel | 30 | 0.89 | 🟢 |
| cosca-architecture | 0 | — | 🟢 |
| ... | | | |

## Recommendations

1. **2 aprendizados em REVALIDATE_30**: agendar revalidação nos respectivos agentes
2. **Nenhum aprendizado em risco imediato**: sistema saudável
3. **Freshness médio 0.89**: excelente — manter padrão de reutilização

## Action Items

- [ ] Revalidar L15 (cosca-kernel) até 2026-08-29
- [ ] Revalidar "Onda 2" (cosca-kernel) até 2026-08-29
```

---

## 10. Métricas do Engine

### Métricas Internas

| Métrica | Definição | Alvo |
|---------|-----------|------|
| **Pipeline Duration** | Tempo total de execução do pipeline | < 10s |
| **Agents Scanned** | Número de agentes varridos | 54/54 |
| **Learnings Processed** | Total de aprendizados analisados | Incremental |
| **Freshness Avg** | Média do freshness de todos os aprendizados | > 0.70 |
| **Stale Ratio** | Aprendizados com freshness < 0.30 / Total | < 5% |
| **Deprecation Rate** | Aprendizados depreciados por execução | Informativo |
| **Contradiction Rate** | Contradições detectadas por execução | < 2% |
| **Auto-revalidation Rate** | Revalidações automáticas bem-sucedidas | > 80% |

### Integração com F1.5 — Real Metrics

| Métrica | Fonte | Frequência |
|---------|-------|------------|
| B3 Knowledge Freshness | Freshness médio dos aprendizados consultados | Por task |
| Stale Ratio | Razão de aprendizados stale | Semanal |
| Deprecation Velocity | Quantos aprendizados depreciados/mês | Mensal |

### Alertas

| Condição | Severidade | Ação |
|----------|------------|------|
| System Avg Freshness < 0.60 | 🔴 ALTO | Auditoria completa de conhecimento |
| Stale Ratio > 15% | 🔴 ALTO | Revalidação em massa de todos os agentes |
| Deprecation Rate > 5%/mês | 🟡 MÉDIO | Revisar qualidade dos aprendizados |
| Contradiction Rate > 5% | 🟡 MÉDIO | Revisão manual necessária |
| Pipeline falha > 2x seguidas | 🔴 ALTO | Investigar engine |

---

## Appendix A: Estrutura de Dados do Learning (Campos Estendidos)

O formato de entrada em `learnings.md` (definido em `LEARNING_PROTOCOL.md`) é estendido com campos do Wisdom Decay:

```markdown
### {timestamp} — {technique-name}

| Field | Value |
|-------|-------|
| ... (campos existentes) |
| **Wisdom Decay Category** | CRITICAL / STABLE / EXPERIMENTAL / DEPRECATED |
| **Last Used** | YYYY-MM-DD (data do último uso/referência) |
| **Usage Count** | N (número de vezes que foi referenciado) |
| **Freshness Score** | 0.00–1.00 (calculado pelo engine) |
| **Contradictions** | [lista de timestamps de aprendizados contraditórios] |
| **Last Review** | YYYY-MM-DD (data da última revisão manual) |
| **Freshness Status** | healthy / revalidate_30 / revalidate_now / deprecated / conflict |
```

## Appendix B: Referências

| Documento | Relação |
|-----------|---------|
| [WISDOM_DECAY.md (memory/)](../../memory/WISDOM_DECAY.md) | Especificação conceitual com categorias e curva de confiança |
| [CONFIDENCE_MODEL.md](../evidence/CONFIDENCE_MODEL.md) | Evidence trust engine — freshness como modifier |
| [DECISION_DNA.md](../../knowledge/architecture/DECISION_DNA.md) | Depreciação via DDNA |
| [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) | Formato de entrada de aprendizado |
| [MEMORY_CURATION_ENGINE.md](../memory-curation/MEMORY_CURATION_ENGINE.md) | Curation engine — freshness como input |
| [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) | C11 — Wisdom Decay conceito |
| [AGENTS.md](../../../AGENTS.md) | Política de descoberta de agentes |

---

## Changelog

| Versão | Data | Autor | Mudança |
|--------|------|-------|---------|
| 1.0.0 | 2026-07-30 | Architecture Chief | Criação inicial — engine spec, 3 decay types, pipeline, fórmula, exemplos reais (L17, L22, L29), CLI, integrações |

---

> *"Conhecimento sem validade é lixo acumulado. Conhecimento com validade é alimento fresco."*
> — Cosca Architecture Chief, 2026-07-30
