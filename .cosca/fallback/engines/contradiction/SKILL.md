# CONTRADICTION ENGINE v2 — Mineração Ativa de Evidências Opostas (F8.3)

> **Versão**: 2.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
> **CMI Dimensão**: Decision Quality (Julgamento) | **Bloco Cognitivo**: Bloco 2 — Decisão & Raciocínio
> **Fase CMI**: Fase 1 — Foundation | **Código**: F8.3 | **Quality Gate**: G0.5 (complementar)
>
> **Dependências**:
> - F1.2 [Contrafactual Gate](../../workflows/contrafactual-gate.md) — gate de decisão; este engine alimenta e ativa o gate
> - F1.4 [Wisdom Decay](../wisdom-decay/WISDOM_DECAY.md) — contradições alimentam freshness score
> - F1.6 [Cognitive Entropy](../cognitive-entropy/ENTROPY.md) — contradições como componente de entropia
> - F7.1 [Prediction](../prediction/SKILL.md) — contradições reduzem confiança preditiva
> - F7.2 [Trust Registry](../../memory/trust/TRUST_REGISTRY.md) — reputação histórica para calibrar contradições
> - F1.1 [Decision DNA](../../knowledge/architecture/DECISION_DNA.md) — contradições geram DDNA de conflito
>
> Consulte também:
> - [F1.6 Cognitive Entropy — §3.1 Contradictions](../cognitive-entropy/ENTROPY.md#31-contradictions-peso-3)
> - [F1.4 Wisdom Decay — §2.3 Contradiction Decay](../wisdom-decay/WISDOM_DECAY.md#23-contradiction-decay)
> - [Immune System — §2 Antígenos Cognitivos](../immune-system/SKILL.md#2-antígenos-cognitivos--catálogo-de-patologias)
> - [Forward Consequence Projection — §2.3 Árvore de Consequências](../second-order-reasoning/PROJECTION.md#23-os-5-níveis-de-profundidade)
> - [CONFIDENCE_MODEL.md](../evidence/CONFIDENCE_MODEL.md) — M5: Contradita por fonte superior (-0.40)

---

## ÍNDICE

1. [O que é a Contradiction Engine v2](#1-o-que-é-a-contradiction-engine-v2)
2. [Pipeline Completo](#2-pipeline-completo)
3. [Fontes de Contradição](#3-fontes-de-contradição)
4. [Score de Contradição](#4-score-de-contradição)
5. [Detecção e Validação de Falsos Positivos](#5-detecção-e-validação-de-falsos-positivos)
6. [Integração com F1.2 — Contrafactual Gate](#6-integração-com-f12--contrafactual-gate)
7. [Integração com F1.6 — Cognitive Entropy](#7-integração-com-f16--cognitive-entropy)
8. [Integração com F7.1 — Prediction Engine](#8-integração-com-f71--prediction-engine)
9. [Regras de Override (Don)](#9-regras-de-override-don)
10. [Exemplo Completo](#10-exemplo-completo)
11. [Métricas do Engine](#11-métricas-do-engine)
12. [CLI e Automação](#12-cli-e-automação)
13. [Relacionados](#13-relacionados)

---

## 1. O QUE É A CONTRADICTION ENGINE V2

### 1.1 Definição

A **Contradiction Engine v2** é o motor do Cosca que **caça ativamente evidências CONTRA a decisão proposta**. Diferente do Contrafactual Gate (F1.2), que gera alternativas e compara (A vs ¬A), este engine **mina o conhecimento existente** em busca de evidências que contradizem diretamente a premissa ou a execução da decisão.

Onde o Gate é **gerativo** (cria cenários alternativos), a Contradiction Engine é **investigativa** (escava conhecimento já registrado).

### 1.2 Filosofia

```
Decisão proposta: "vamos remover o provider X"
Contradiction Engine pergunta:
  ├── "Alguém já tentou remover X antes? O que aconteceu?"
  ├── "Há um learning que diz 'sempre verificar dependências antes de remover'?"
  ├── "Há um failure registrado que ocorreu na última remoção?"
  ├── "Há uma decisão anterior (DDNA) que escolheu manter X?"
  ├── "O agente que propõs tem baixa confiança neste domínio?"
  └── "O histórico mostra que remover X sempre quebra Y?"

Se encontrar N evidências CONTRA > evidências A FAVOR → abre Gate
Se confiança das evidências CONTRA > 0.7 → bloqueia e escalona ao Don
```

### 1.3 Diferença do Contrafactual Gate (F1.2)

| Dimensão | Contrafactual Gate (F1.2) | Contradiction Engine v2 (F8.3) |
|----------|--------------------------|-------------------------------|
| **O que faz** | Gera alternativas (A, ¬A, C) e compara | Mina evidências existentes que contradizem a decisão |
| **Natureza** | **Gerativa** — cria cenários "e se fosse diferente?" | **Investigativa** — escava memória em busca de contradições |
| **Quando atua** | Antes da decisão P0/P1 (mandatório) | Disparado pela decisão proposta ou por detecção passiva |
| **Fonte principal** | Raciocínio adversarial do cosca-critic | Learnings + Failures + DDNA + Trust Registry + Timeline |
| **Output** | Matriz de comparação A/B/C + recomendação | Score de contradição + evidências encontradas |
| **Complementaridade** | Gera o que PODERIA ser diferente | Encontra o que JÁ FOI diferente e deu errado |

### 1.4 Analogia: O Advogado do Diabo

```
Contrafactual Gate: O juiz que pede "e se a testemunha estiver mentindo?"
Contradiction Engine: O investigador que vasculha arquivos em busca de
                      "esta testemunha já foi pega mentindo antes"

Gate pergunta: "e se?"
Engine responde: "já aconteceu. Eis a prova."
```

---

## 2. PIPELINE COMPLETO

### 2.1 Visão Geral

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                     CONTRADICTION ENGINE v2 — PIPELINE                        │
│                                                                              │
│  TRIGGERS:                                                                   │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐           │
│  │ Decisão Proposta │  │ Wisdom Decay     │  │ Scheduled Scan   │           │
│  │ (pré-delegação)  │  │ contradição nova │  │ (diário)         │           │
│  └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘           │
│           │                     │                      │                      │
│           └─────────────────────┼──────────────────────┘                      │
│                                 │                                              │
│                                 ▼                                              │
│  ┌────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 1: CONSULTA (5 fontes de contradição)                            │  │
│  │                                                                        │  │
│  │  ├── [1] learnings.md de todos os 54 agentes                          │  │
│  │  │     → "alguém já tentou isso e falhou?"                            │  │
│  │  │     → "alguém já documentou que isso é perigoso?"                  │  │
│  │  │                                                                     │  │
│  │  ├── [2] failures.md de todos os 54 agentes                           │  │
│  │  │     → "alguém já quebrou algo parecido?"                           │  │
│  │  │     → "falha passada no mesmo domínio?"                            │  │
│  │  │                                                                     │  │
│  │  ├── [3] DDNAs existentes                                              │  │
│  │  │     → "decisão anterior escolheu o oposto?"                        │  │
│  │  │     → "decisão anterior já reverteu isso?"                         │  │
│  │  │                                                                     │  │
│  │  ├── [4] Trust Registry                                                │  │
│  │  │     → "agente proponente já errou nesse domínio?"                  │  │
│  │  │     → "qual a confiança histórica do agente aqui?"                 │  │
│  │  │                                                                     │  │
│  │  └── [5] Engineering Timeline                                          │  │
│  │        → "já removemos X antes? O que aconteceu?"                     │  │
│  │        → "tendência histórica: toda vez que fazemos X, Y quebra?"     │  │
│  │                                                                        │  │
│  │  ⏱ Tempo alvo: < 400ms (consultas paralelas + cache)                 │  │
│  └────────────────────────────────┬───────────────────────────────────────┘  │
│                                   │                                           │
│                                   ▼                                           │
│  ┌────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 2: PONTUAÇÃO                                                      │  │
│  │                                                                        │  │
│  │  ├── Para cada evidência encontrada:                                   │  │
│  │  │   ├── classificar tipo (learning/failure/ddna/trust/timeline)       │  │
│  │  │   ├── extrair confidence da evidência (0.0-1.0)                    │  │
│  │  │   ├── calcular recency (dias desde registro)                        │  │
│  │  │   └── classificar severity (baixa/média/alta/crítica)               │  │
│  │  │                                                                     │  │
│  │  ├── Calcular contradiction_score (fórmula §4)                        │  │
│  │  │                                                                     │  │
│  │  └── Contar evidências A FAVOR vs CONTRA                               │  │
│  │                                                                        │  │
│  │  ⏱ Tempo alvo: < 50ms (cálculo O(n) com n = evidências)              │  │
│  └────────────────────────────────┬───────────────────────────────────────┘  │
│                                   │                                           │
│                                   ▼                                           │
│  ┌────────────────────────────────────────────────────────────────────────┐  │
│  │ FASE 3: AÇÃO                                                           │  │
│  │                                                                        │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │                    contradiction_score                            │  │  │
│  │  │                                                                   │  │  │
│  │  │  score > 0.7 ────────────► BLOQUEIA                               │  │  │
│  │  │  │                         ├── Escalona ao Don                    │  │  │
│  │  │  │                         ├── Don decide: override ou mantém     │  │  │
│  │  │  │                         └── Cria DDNA de conflito              │  │  │
│  │  │  │                                                                 │  │  │
│  │  │  0.4 ≤ score ≤ 0.7 ──────► ABRE GATE                               │  │  │
│  │  │  │                         ├── Ativa Contrafactual Gate (F1.2)    │  │  │
│  │  │  │                         └── Gate reavalia com contradições     │  │  │
│  │  │  │                                                                 │  │  │
│  │  │  score < 0.4 ────────────► APENAS DOCUMENTA                        │  │  │
│  │  │                            ├── Registra no log de contradições    │  │  │
│  │  │                            └── Alimenta Cognitive Entropy (F1.6)  │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  │                                                                        │  │
│  │  ⏱ Tempo alvo: < 50ms (decisão baseada em threshold)                 │  │
│  └────────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  TOTAL: < 500ms                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Gatilhos de Ativação

| Gatilho | Disparado por | Quando | Ação |
|---------|--------------|--------|------|
| **Decisão Proposta** | Kernel (pré-delegação) | Antes de toda task P0/P1 | Pipeline completo (F1-F3) |
| **Nova Contradição no Wisdom Decay** | F1.4 Wisdom Decay | Quando F1.4 detecta contradição entre aprendizados | Engine recalcula score e verifica se afeta decisões pendentes |
| **Escaneamento Programado** | Scheduler diário | Diariamente à 00:00 | Varredura passiva de contradições latentes (não vinculadas a uma decisão específica) |
| **Manual** | CLI `cosca contradiction mine` | On-demand | Pipeline completo com decisão fornecida |

### 2.3 Algoritmo

```
function mine_contradictions(proposed_decision):
    // Fase 1: Consulta paralela às 5 fontes
    learnings_contra = search_learnings_contra(proposed_decision)     // [1]
    failures_contra   = search_failures_contra(proposed_decision)     // [2]
    ddnas_contra      = search_ddnas_contra(proposed_decision)        // [3]
    trust_contra      = search_trust_contra(proposed_decision)        // [4]
    timeline_contra   = search_timeline_contra(proposed_decision)     // [5]

    all_evidence_contra = merge(learnings_contra, failures_contra,
                                ddnas_contra, trust_contra, timeline_contra)

    // Fase 2: Pontuação
    contradiction_score = calculate_score(all_evidence_contra)
    count_contra = len(all_evidence_contra)
    count_favor = count_supporting_evidence(proposed_decision)

    // Fase 3: Ação
    if contradiction_score > 0.7:
        return BLOCK(contradiction_score, all_evidence_contra)
    elif contradiction_score >= 0.4:
        atividade = open_contrafactual_gate(proposed_decision, all_evidence_contra)
        return ESCALATE(contradiction_score, gate_id)
    else:
        register_contradiction_log(proposed_decision, all_evidence_contra)
        return DOCUMENT(contradiction_score, all_evidence_contra)
```

### 2.4 Performance

| Operação | Tempo Máximo | Estratégia |
|----------|-------------|------------|
| Consulta learnings (54 agentes) | < 150ms | Cache de índices FTS5, busca paralela por tags |
| Consulta failures (54 agentes) | < 100ms | Cache de índices FTS5, busca paralela |
| Consulta DDNAs | < 50ms | Índice de domínio em DECISION_DNA_INDEX.md |
| Consulta Trust Registry | < 50ms | Já carregado em memória no runtime |
| Consulta Timeline | < 50ms | Timeline CSV parseado uma vez, cacheado |
| Cálculo do score | < 50ms | O(n) em memória |
| Decisão e ação | < 100ms | Thresholds pré-computados |
| **Total** | **< 500ms** | Limite rigoroso |

---

## 3. FONTES DE CONTRADIÇÃO

### 3.1 Learnings — Memória Positiva dos Agentes

**O que consulta**: `learnings.md` de cada um dos 54 agentes.

**Mecanismo**: Matching semântico entre o domínio da decisão proposta e as tags/conteúdo dos aprendizados. Uma contradição existe quando um learning afirma A e a decisão propõe não-A.

**Formato da consulta**:

```yaml
contradiction_query:
  source: "learnings"
  decision_domain: "provider-removal"
  decision_tags: ["provider", "migration", "removal"]
  matching_tags: ["provider", "removal", "migration", "downtime"]
  min_confidence: 0.40        # Só considera learnings com confidence ≥ 0.40
  lookback_days: 365           # Considera learnings de até 1 ano atrás
```

**Formato da resposta**:

```yaml
contradiction_evidence:
  - source: "learning"
    agent: "cosca-backend"
    learning_id: "L29"                           # Formato: timestamp ou ID
    title: "Sempre verificar go list -deps antes de remover provider"
    domain: "provider-management"
    evidence: "go list -deps mostra dependências ocultas que não aparecem em go.mod"
    tags: ["#provider", "#deps", "#removal", "#go"]
    confidence: 0.92                              # Confidence original do learning
    recency_days: 3                               # Dias desde o último uso
    contradiction_type: "dependence_violation"     # Tipo de contradição (ver §5.1)
    matched_on: "decision propõe remover sem verificar dependências"
```

**Regras**:
- Só considera learnings com `confidence >= 0.40` (aprendizados com confiança muito baixa não geram contradição forte)
- Só considera learnings com recência < 365 dias (aprendizados muito antigos têm peso reduzido)
- Learnings marcados como `#deprecated` são ignorados

### 3.2 Failures — Memória Negativa dos Agentes

**O que consulta**: `failures.md` de cada um dos 54 agentes.

**Mecanismo**: Matching por domínio + tags. Uma failure no mesmo domínio da decisão proposta é uma evidência CONTRA forte, especialmente se a falha foi causada por ação similar.

**Formato da consulta**:

```yaml
contradiction_query:
  source: "failures"
  decision_domain: "provider-removal"
  decision_tags: ["provider", "removal", "migration", "downtime"]
  matching_tags: ["provider", "removal", "migration", "downtime", "outage"]
  require_same_domain: true     # Failure precisa ser do mesmo domínio
  severity_min: "medium"        # Só considera severidade média+
```

**Formato da resposta**:

```yaml
contradiction_evidence:
  - source: "failure"
    agent: "cosca-database"
    failure_id: "F005"
    title: "Remoção do provider SQLite causou downtime de 45min"
    domain: "provider-management"
    description: "Removemos o provider SQLite sem verificar dependências em lote. 3 serviços ficaram sem acesso a BD por 45 minutos."
    severity: "critical"
    tags: ["#provider", "#removal", "#downtime", "#sqlite"]
    recency_days: 45
    contradiction_type: "past_failure_repeat"
    matched_on: "decision propõe remover provider sem verificar dependências"
```

**Regras**:
- Só considera failures com `severity >= medium` (failures triviais não geram contradição)
- Failures com `status: resolved` contam mas com peso reduzido em 30%
- Múltiplas failures no mesmo domínio aumentam o score cumulativamente

### 3.3 DDNAs — Decisões Anteriores

**O que consulta**: Arquivos DDNA em `memory/decision-dna/` ou `knowledge/architecture/DECISION_DNA.md`.

**Mecanismo**: Matching por domínio + comparação de decisão. Um DDNA que decidiu o oposto da decisão proposta é uma contradição direta. Um DDNA que decidiu a mesma coisa mas foi revertido depois também é evidência contra.

**Formato da consulta**:

```yaml
contradiction_query:
  source: "ddna"
  decision_domain: "provider-removal"
  decision_tags: ["provider", "removal", "migration"]
  decision_action: "remove"        # O que a decisão propõe fazer
  compare_action: "keep"           # Ação oposta a buscar nos DDNAs
  status_filter: ["accepted", "active"]   # Só DDNAs válidos
```

**Formato da resposta**:

```yaml
contradiction_evidence:
  - source: "ddna"
    ddna_id: "DDNA-2026-07-15-003"
    title: "Decisão de manter provider X por dependências críticas"
    domain: "provider-management"
    decision: "keep"                         # A decisão tomada no DDNA
    proposed_decision: "remove"              # A decisão proposta atual
    rationale: "Provider X é dependência de 3 serviços core. Remoção requer migração de 6 meses."
    status: "accepted"
    date: "2026-07-15"
    confidence: 0.85
    contradiction_type: "ddna_opposite_decision"
    matched_on: "DDNA anterior decidiu MANTER o mesmo provider que agora querem REMOVER"
```

**Regras**:
- DDNAs com `status: superseded` são ignorados (já foram substituídos)
- DDNAs com `status: deprecated` contam com peso 0.5
- A data do DDNA influencia a recência (DDNAs mais recentes têm mais peso)

### 3.4 Trust Registry — Reputação Histórica do Agente

**O que consulta**: `memory/trust/TRUST_REGISTRY.md`.

**Mecanismo**: Verifica a confiança do agente proponente no domínio da decisão. Se o agente tem baixa taxa de sucesso ou alta taxa de erro no domínio, isso é uma contradição indireta — não ao conteúdo da decisão, mas à credibilidade de quem propõe.

**Formato da consulta**:

```yaml
contradiction_query:
  source: "trust_registry"
  agent: "cosca-backend"                # Agente que propôs a decisão
  domain: "provider-management"         # Domínio da decisão
  min_samples: 2                        # Mínimo de amostras para considerar
  thresholds:
    success_rate_below: 0.70            # Se success_rate < 0.70, alerta
    domain_strength_below: 0.50         # Se domain_strength < 0.50, alerta
    confidence_below: 0.50              # Se confidence < 0.50, alerta
```

**Formato da resposta**:

```yaml
contradiction_evidence:
  - source: "trust_registry"
    agent: "cosca-backend"
    domain: "provider-management"
    total_decisions: 5
    success_rate: 0.40                  # 2/5 sucessos — BAIXO
    domain_strength: 0.35               # Fraco no domínio
    avg_confidence: 0.45
    recency_days: 12
    contradiction_type: "low_agent_credibility"
    matched_on: "agente cosca-backend tem apenas 40% de sucesso em provider-management"
    flags:
      - "success_rate_below_threshold"
      - "domain_strength_below_threshold"
      - "confidence_below_threshold"
```

**Regras**:
- Só gera contradição se PELO MENOS UM dos thresholds for violado
- Se múltiplos thresholds violados, a evidência conta como 1 unidade com peso aumentado
- Agentes sem histórico no domínio (`total_decisions = 0`) geram aviso neutro, não contradição

### 3.5 Engineering Timeline — Histórico de Eventos

**O que consulta**: Timeline de eventos de engenharia em `memory/timeline/`.

**Mecanismo**: Busca padrões históricos no CSV de timeline. A timeline registra decisões anteriores, reversões, incidentes e outcomes. O engine busca sequências como "toda vez que removemos X, Y quebrou".

**Formato da consulta**:

```yaml
contradiction_query:
  source: "timeline"
  decision_domain: "provider-removal"
  decision_action: "remove"
  search_pattern: "remoção de provider"    # Palavra-chave na timeline
  pattern_window_days: 30                  # Dias após o evento para observar consequências
  min_occurrences: 2                       # Mínimo de ocorrências para considerar padrão
```

**Formato da resposta**:

```yaml
contradiction_evidence:
  - source: "timeline"
    pattern_id: "T-007"
    pattern: "remoção de provider → incidente de downtime"
    occurrences:
      - date: "2026-06-15"
        event: "Removido provider SQLite"
        consequence: "Downtime de 45min em 3 serviços"
        severity: "critical"
      - date: "2026-05-20"
        event: "Removido provider Redis cache layer"
        consequence: "Degradação de performance por 2h"
        severity: "high"
      - date: "2026-04-10"
        event: "Removido provider Auth0"
        consequence: "Auth quebrado por 20min (fallback não configurado)"
        severity: "high"
    frequency: 3                           # 3 ocorrências no período
    confidence: 0.88                       # Padrão forte, 3/3 vezes
    recency_days: 15
    contradiction_type: "historical_pattern"
    matched_on: "historicamente, remoção de provider SEMPRE causa incidente"
```

**Regras**:
- Só gera contradição se houver `>= 2` ocorrências do padrão
- Padrões com `confidence > 0.80` são considerados fortes
- Padrões com recência > 180 dias são ignorados (contexto pode ter mudado)

---

## 4. SCORE DE CONTRADIÇÃO

### 4.1 Fórmula

```
contradiction_score = Σ(
    count_evidencias_contra × 0.40 +
    avg_confidence_contra   × 0.30 +
    recency_contra          × 0.20 +
    severity_contra         × 0.10
)
```

### 4.2 Componentes

| Componente | Peso | Cálculo | Faixa | Descrição |
|------------|------|---------|-------|-----------|
| **count_evidencias_contra** | 0.40 | `min(total_evidencias / 10, 1.0)` | 0.0–1.0 | Quantas evidências contra foram encontradas. Saturou em 10+ |
| **avg_confidence_contra** | 0.30 | `média(confidence de cada evidência)` | 0.0–1.0 | Confiança média das evidências. Quanto maior, mais forte a contradição |
| **recency_contra** | 0.20 | `1 - min(avg_days_since / 365, 1.0)` | 0.0–1.0 | Quão recentes são as evidências. Quanto menor avg_days_since, maior o score |
| **severity_contra** | 0.10 | `max(severity_score de cada evidência)` | 0.0–1.0 | Pior severidade entre as evidências. A mais grave dita o tom |

### 4.3 Mapa de Severidade

| Severidade | Score | Descrição | Exemplo |
|------------|-------|-----------|---------|
| **Crítica** | 1.00 | Evidência de dano catastrófico | Failure que derrubou produção por horas |
| **Alta** | 0.75 | Evidência de dano significativo | Learning que alerta sobre perigo grave |
| **Média** | 0.50 | Evidência de risco moderado | DDNA que decidiu o oposto com rationale sólido |
| **Baixa** | 0.25 | Evidência de risco menor | Trust Registry com confiança baixa mas sem falhas |

### 4.4 Thresholds

```
contradiction_score > 0.7 ──────► BLOQUEIA (escalona ao Don)
contradiction_score 0.4–0.7 ────► ABRE GATE (ativa Contrafactual Gate)
contradiction_score < 0.4 ──────► APENAS DOCUMENTA (alimenta entropia)
```

### 4.5 Exemplo de Cálculo

**Cenário**: Decisão proposta "remover provider X sem verificar dependências".
Engine encontra:

| # | Fonte | Evidência | Confidence | Recência (dias) | Severidade |
|---|-------|-----------|:----------:|:---------------:|:----------:|
| 1 | Learning (L29) | "sempre verificar go list -deps antes" | 0.92 | 3 | Alta (0.75) |
| 2 | Failure (F005) | "remoção SQLite causou downtime 45min" | 0.88 | 45 | Crítica (1.00) |
| 3 | DDNA (D-003) | "decidiu manter provider X" | 0.85 | 30 | Alta (0.75) |
| 4 | Trust | agente tem 40% sucesso no domínio | 0.60 | 12 | Média (0.50) |

```
count_evidencias_contra = min(4 / 10, 1.0) = 0.400  × 0.40 = 0.160
avg_confidence_contra   = (0.92 + 0.88 + 0.85 + 0.60) / 4 = 0.813  × 0.30 = 0.244
recency_contra          = 1 - min((3 + 45 + 30 + 12) / 4 / 365, 1.0)
                        = 1 - min(22.5 / 365, 1.0)
                        = 1 - 0.062 = 0.938  × 0.20 = 0.188
severity_contra         = max(0.75, 1.00, 0.75, 0.50) = 1.00  × 0.10 = 0.100

contradiction_score = 0.160 + 0.244 + 0.188 + 0.100 = 0.692

Resultado: 0.692 → 0.4 ≤ score ≤ 0.7 → ABRE GATE
```

### 4.6 Fórmula de Qualificação "Evidências CONTRA vs A FAVOR"

Além do score numérico, o engine faz uma contagem simples:

```
Se count_evidencias_contra > count_evidencias_favor:
    → ativa Gate mesmo se score < 0.4 (overriding parcial)
Se count_evidencias_contra <= count_evidencias_favor:
    → segue apenas o score
```

**Motivação**: Se há mais evidências contra do que a favor, mesmo que cada uma individualmente seja fraca, o volume merece atenção.

---

## 5. DETECÇÃO E VALIDAÇÃO DE FALSOS POSITIVOS

### 5.1 Tipos de Contradição

Nem toda oposição é uma contradição real. O engine classifica cada evidência em um tipo para evitar falsos positivos:

| Tipo | Descrição | Exemplo | É contradição real? |
|------|-----------|---------|:-------------------:|
| **direct_opposition** | Afirmação A vs decisão não-A | Learning: "sempre usar transações" / Decisão: "remover transações" | ✅ Sim |
| **dependence_violation** | Decisão ignora dependência documentada | Learning: "verificar deps antes de remover" / Decisão: "remover sem verificar" | ✅ Sim |
| **past_failure_repeat** | Decisão repete ação que já falhou | Failure: "remoção causou downtime" / Decisão: "remover sem testes" | ✅ Sim |
| **ddna_opposite_decision** | DDNA anterior decidiu o oposto | DDNA: "manter provider X" / Decisão: "remover provider X" | ✅ Sim |
| **low_agent_credibility** | Agente proponente não é confiável no domínio | Trust: 40% sucesso / Decisão: proposta por este agente | ⚠️ Parcial |
| **historical_pattern** | Padrão histórico de consequências negativas | Timeline: "toda remoção causa incidente" | ✅ Sim |
| **apparent_contradiction** | Parece contradição mas não é | Learning de contexto diferente aplicado fora de contexto | ❌ Falso positivo |
| **superseded_evidence** | Evidência foi substituída por decisão posterior | DDNA superseded / Learning deprecated | ❌ Ignorado |

### 5.2 Filtros Anti-Falso-Positivo

| Filtro | O que faz | Disparo |
|--------|-----------|---------|
| **Context Mismatch** | Verifica se o contexto do learning/failure é o MESMO da decisão. Se tags divergirem > 40%, a evidência é descartada | Matching de tags < 60% |
| **Superseded Check** | Verifica se o DDNA ou learning foi superseded por outro | `status: superseded` |
| **Deprecated Filter** | Ignora learnings/failures marcados como deprecated | Tag `#deprecated` |
| **Recency Gate** | Ignora evidências com mais de 365 dias | `recency_days > 365` |
| **Confidence Floor** | Ignora evidências com confidence < 0.30 | `confidence < 0.30` |
| **Cross-Agent Validation** | Se a mesma evidência aparece em apenas 1 agente, reduz peso em 20% (pode ser viés local) | Evidência única |
| **Don Override Check** | Se o Don já fez override desta contradição antes, ignorar (ver §9) | Don override registry |

### 5.3 Matriz de Decisão

```
                                  EVIDÊNCIA É CONTRADIÇÃO REAL?
                                ┌────────────────────┬──────────────────┐
                                │        SIM         │       NÃO        │
        ┌───────────────────────┼────────────────────┼──────────────────┤
        │ MÚLTIPLAS FONTES      │ Contradição FORTE  │ Falso positivo   │
        │ (3+ agentes)          │ Score +0.2 bonus    │ improvável       │
        ├───────────────────────┼────────────────────┼──────────────────┤
        │ FONTE ÚNICA           │ Contradição MÉDIA  │ Possível viés    │
        │ (1 agente)            │ Score normal        │ local. Reduzir   │
        │                       │                    │ peso em 20%      │
        ├───────────────────────┼────────────────────┼──────────────────┤
        │ CONFLITO DE TAGS      │ Contradição FRACA  │ Muito provável   │
        │ (tags divergem)       │ Score -0.3 penalty  │ falso positivo   │
        └───────────────────────┴────────────────────┴──────────────────┘
```

---

## 6. INTEGRAÇÃO COM F1.2 — CONTRAFACTUAL GATE

### 6.1 Como a Contradiction Engine Ativa o Gate

A Contradiction Engine NÃO substitui o Contrafactual Gate. Ela o **alimenta e ativa**:

```
DECISÃO PROPOSTA
        │
        ▼
┌────────────────────────────────┐
│ CONTRADICTION ENGINE v2        │
│                                │
│  Score 0.4–0.7 → "abre gate"  │
│  ───────────────────────────   │
│  Output para o Gate:           │
│  ├── Evidências CONTRA         │
│  ├── Score de contradição      │
│  └── Recomendação inicial      │
└────────────┬───────────────────┘
             │
             ▼
┌────────────────────────────────┐
│ CONTRAFACTUAL GATE (F1.2)      │
│                                │
│  Recebe contradições como      │
│  input para análise:           │
│  ├── Alternativa A: proposta   │
│  │   (com contradições)        │
│  ├── Alternativa ¬A: oposto    │
│  └── Gate decide:              │
│      proceed | escalate | reject│
└────────────────────────────────┘
```

### 6.2 Formato dos Dados Transmitidos

Quando ativa o Gate (score 0.4–0.7), a Contradiction Engine envia:

```yaml
contradiction_gate_input:
  engine: "contradiction-v2"
  decision_id: "DEC-2026-07-30-001"
  
  contradiction_score: 0.692
  count_contra: 4
  count_favor: 1
  
  evidence_contra:
    - source: "learning"
      agent: "cosca-backend"
      id: "L29"
      title: "Sempre verificar go list -deps antes de remover provider"
      confidence: 0.92
      contradiction_type: "dependence_violation"
    
    - source: "failure"
      agent: "cosca-database"
      id: "F005"
      title: "Remoção do provider SQLite causou downtime de 45min"
      confidence: 0.88
      contradiction_type: "past_failure_repeat"
    
    - source: "ddna"
      ddna_id: "DDNA-2026-07-15-003"
      title: "Decisão de manter provider X por dependências críticas"
      confidence: 0.85
      contradiction_type: "ddna_opposite_decision"
    
    - source: "trust_registry"
      agent: "cosca-backend"
      metric: "success_rate"
      value: 0.40
      contradiction_type: "low_agent_credibility"
  
  recommendation: "Gate recomenda ESCALATE — 4 evidências contra vs 1 a favor"
  override_possible: true
  don_override_if_approved: true
```

### 6.3 Regras de Ativação do Gate

| Condição da Contradiction Engine | Ação no Gate |
|---------------------------------|--------------|
| `score > 0.7` | **Não ativa Gate** — vai direto para Don (bypassa Gate) |
| `score 0.4–0.7` | **Ativa Gate** — passa contradições como input |
| `score < 0.4` | **Não ativa Gate** — apenas documenta |
| `count_contra > count_favor` mesmo com score < 0.4 | **Ativa Gate** — override do threshold por volume |
| Don override ativo para este domínio | **Não ativa Gate** — respeita override |

### 6.4 Integração Bidirecional

Após o Gate decidir, a Contradiction Engine é atualizada:

```yaml
contradiction_gate_feedback:
  decision_id: "DEC-2026-07-30-001"
  gate_decision: "proceed | escalate | reject"
  gate_confidence: 0.85
  contradictions_used: true/false
  contradiction_score_pre_gate: 0.692
  contradiction_score_post_gate: 0.692  # Pode ser ajustado pelo Gate
  learning_generated: "Gate validou/rejeitou contradições. Novo learning registrado."
```

---

## 7. INTEGRAÇÃO COM F1.6 — COGNITIVE ENTROPY

### 7.1 Contradições como Alimento da Entropia

A Contradiction Engine v2 **alimenta diretamente** o componente de Contradições do Cognitive Entropy (F1.6, §3.1, peso 3):

```
F1.6 CognitiveEntropy = Σ(contradictions × 3 + stale_knowledge × 2 + gaps × 1)
                                           / total_knowledge
                           ▲
                           │
              ┌────────────────────┐
              │ Contradiction      │
              │ Engine v2 (F8.3)   │
              │                    │
              │ Output:            │
              │ ├── contradições   │
              │ │   ativas         │
              │ ├── contradições   │
              │ │   resolvidas     │
              │ └── score por      │
              │     contradição    │
              └────────────────────┘
```

### 7.2 Formato dos Dados para Entropia

```yaml
contradiction_entropy_feed:
  timestamp: "2026-07-30T00:00:00Z"
  total_contradictions_active: 2
  contradictions_detail:
    - pair_id: "C-004"
      evidence_a:
        source: "learning"
        agent: "cosca-backend"
        id: "L29"
        claim: "Sempre verificar dependências antes de remover provider"
      evidence_b:
        source: "decision_proposal"
        agent: "cosca-backend"
        claim: "Remover provider X sem verificação de dependências"
      domain: "provider-management"
      contradiction_score: 0.692
      status: "active"           # active | resolved | dismissed
      resolution: null
    
    - pair_id: "C-005"
      evidence_a:
        source: "ddna"
        ddna_id: "DDNA-2026-07-15-003"
        claim: "Manter provider X por dependências críticas"
      evidence_b:
        source: "decision_proposal"
        claim: "Remover provider X"
      domain: "provider-management"
      contradiction_score: 0.55
      status: "active"
      resolution: null
```

### 7.3 Impacto na Entropia

| Ação da Contradiction Engine | Efeito na Entropia | Quando |
|------------------------------|-------------------|--------|
| Nova contradição detectada | `contradictions += 1` → entropia aumenta | Toda detecção |
| Contradição resolvida (Don override) | `contradictions -= 1` → entropia diminui | Após override |
| Contradição expirada (> 180 dias sem reativação) | `contradictions -= 1` → entropia diminui | Scan semanal |
| Múltiplas contradições no mesmo domínio | Ponderação especial: peso 4 em vez de 3 | Quando 3+ contradições no mesmo domínio |

---

## 8. INTEGRAÇÃO COM F7.1 — PREDICTION ENGINE

### 8.1 Contradições como Modificador de Confiança Preditiva

A existência de contradições ativas no mesmo domínio de uma task reduz a confiança da predição:

```
P(success)_adjusted = P(success)_base × (1 - contradiction_penalty)

Onde:
  contradiction_penalty = min(
    max_contradiction_score_in_domain × 0.3,
    0.25  # Cap máximo de 25% de penalidade
  )
```

### 8.2 Tabela de Penalidade

| Max Contradiction Score no Domínio | Penalidade na P(success) | Exemplo |
|:----------------------------------:|:------------------------:|---------|
| 0.0 (sem contradições) | 0% | P(success) = 0.80 → 0.80 |
| 0.3 | 9% | P(success) = 0.80 → 0.73 |
| 0.5 | 15% | P(success) = 0.80 → 0.68 |
| 0.7 | 21% | P(success) = 0.80 → 0.63 |
| 0.9+ | 25% (cap) | P(success) = 0.80 → 0.60 |

### 8.3 Quando o Prediction Engine Consulta a Contradiction Engine

| Gatilho | Consulta | Impacto |
|---------|----------|---------|
| Task em domínio com contradição ativa | "Qual o max_contradiction_score neste domínio?" | Penalidade na P(success) |
| Task proposta por agente com contradição ativa | "Este agente tem contradições ativas?" | Penalidade adicional se o agente for parte da contradição |
| Task em domínio com padrão histórico negativo | "Há timeline pattern para este domínio?" | Penalidade se pattern confirmado |

---

## 9. REGRAS DE OVERRIDE (DON)

### 9.1 Don Override Registry

O Don pode fazer override de qualquer contradição detectada. Overrides são registrados em `memory/contradiction/override-registry.md`:

```yaml
contradiction_override:
  id: "OVR-2026-07-30-001"
  contradiction_id: "C-004"
  decision_id: "DEC-2026-07-30-001"
  overridden_by: "don"
  override_reason: "Contexto mudou desde o learning L29. Provider X foi substituído internamente e não tem mais dependências."
  override_date: "2026-07-30"
  expires: "2026-10-30"           # Override expira em 90 dias
  scope: "single_decision"        # single_decision | domain | permanent
```

### 9.2 Tipos de Override

| Tipo | Escopo | Duração | Quando usar |
|------|--------|---------|-------------|
| **single_decision** | Apenas esta decisão | 90 dias | "Sei que há contradição, mas decido prosseguir mesmo assim" |
| **domain** | Todas as decisões neste domínio | 180 dias | "A contradição não se aplica mais a este domínio devido a mudança de contexto" |
| **permanent** | Indefinido | Indefinido | "Esta contradição foi resolvida estruturalmente. Não deve mais ser detectada." |

### 9.3 Regras de Override

```
1. Override NÃO apaga a contradição — apenas a suspende
2. Override expira automaticamente (default: 90 dias)
3. Após expirar, a contradição é reavaliada
4. Don pode REVOGAR um override a qualquer momento
5. Override é registrado em DDNA de decisão com rationale completo
6. Override permanente requer validação de 2ª opinião (CTO ou outro Chief)
```

### 9.4 Hierarquia de Decisão

```
Contradiction Engine detecta contradição
    │
    ├── score > 0.7 ──────────► Don decide
    │                            ├── Override (com rationale)
    │                            └── Confirma bloqueio
    │
    ├── score 0.4–0.7 ────────► Gate decide
    │                            ├── Proceed (Gate aprovou)
    │                            ├── Escalate (Gate pede revisão)
    │                            │   └── Don decide
    │                            └── Don override (se Gate reject)
    │
    └── score < 0.4 ──────────► Kernel decide (sem bloqueio)
                                 └── Documenta contradição
```

---

## 10. EXEMPLO COMPLETO

### 10.1 Contexto

**Decisão proposta**: "Remover o provider X do código sem verificar dependências primeiro."
**Proponente**: cosca-backend
**Domínio**: provider-management
**Prioridade**: P1

### 10.2 Execução da Contradiction Engine

```yaml
---
contradiction_engine:
  version: "2.0.0"
  executed_at: "2026-07-30T14:00:00Z"
  executed_by: "cosca-architecture"
  decision_id: "DEC-2026-07-30-001"
  pipeline_duration_ms: 340

  decision:
    title: "Remover provider X sem verificar dependências"
    description: "Proposta de remover o provider X do código-fonte por não ser mais utilizado."
    domain: "provider-management"
    priority: "P1"
    proposed_by: "cosca-backend"

  # ── CONSULTAS ──────────────────────────────────────────────────────────

  queries:
    learnings:
      agents_scanned: 54
      matching_learnings: 3
      contradictions_found: 1

    failures:
      agents_scanned: 54
      matching_failures: 2
      contradictions_found: 1

    ddnas:
      ddnas_scanned: 15
      matching_ddnas: 1
      contradictions_found: 1

    trust_registry:
      agent: "cosca-backend"
      domain: "provider-management"
      total_decisions: 5
      success_rate: 0.40
      contradictions_found: 1  # confidence baixa

    timeline:
      patterns_scanned: 12
      matching_patterns: 1
      contradictions_found: 1

  # ── EVIDÊNCIAS ENCONTRADAS ────────────────────────────────────────────

  evidence:
    contra:
      - id: "E-001"
        source: "learning"
        agent: "cosca-backend"
        learning_id: "L29"
        title: "Sempre verificar go list -deps antes de remover provider"
        confidence: 0.92
        recency_days: 3
        severity: "alta"
        severity_score: 0.75
        contradiction_type: "dependence_violation"
        detail: "Learning L29 do próprio cosca-backend documenta a necessidade de verificar dependências ocultas antes de remover qualquer provider. A decisão proposta ignora esta verificação."

      - id: "E-002"
        source: "failure"
        agent: "cosca-database"
        failure_id: "F005"
        title: "Remoção do provider SQLite causou downtime de 45min"
        confidence: 0.88
        recency_days: 45
        severity: "crítica"
        severity_score: 1.00
        contradiction_type: "past_failure_repeat"
        detail: "Falha F005 documenta que a remoção do provider SQLite sem verificação de dependências causou downtime em 3 serviços por 45 minutos. Mesmo padrão da decisão proposta."

      - id: "E-003"
        source: "ddna"
        ddna_id: "DDNA-2026-07-15-003"
        title: "Decisão de manter provider X por dependências críticas"
        confidence: 0.85
        recency_days: 15
        severity: "alta"
        severity_score: 0.75
        contradiction_type: "ddna_opposite_decision"
        detail: "DDNA-2026-07-15-003 documenta decisão de MANTER o provider X porque ele é dependência crítica de 3 serviços. A decisão proposta quer REMOVER o mesmo provider."

      - id: "E-004"
        source: "trust_registry"
        agent: "cosca-backend"
        metric: "success_rate"
        value: 0.40
        threshold: 0.70
        confidence: 0.60
        recency_days: 12
        severity: "média"
        severity_score: 0.50
        contradiction_type: "low_agent_credibility"
        detail: "cosca-backend tem apenas 40% de sucesso (2/5) em decisões de provider-management. 3 thresholds violados: success_rate < 0.70, domain_strength < 0.50, confidence < 0.50."

      - id: "E-005"
        source: "timeline"
        pattern_id: "T-007"
        pattern: "remoção de provider → incidente de downtime"
        confidence: 0.88
        recency_days: 15
        severity: "crítica"
        severity_score: 1.00
        contradiction_type: "historical_pattern"
        detail: "Padrão histórico T-007: 3 ocorrências de remoção de provider seguidas de incidente. 100% de taxa de correlação. A última foi há 15 dias."

    favor:
      - id: "F-001"
        source: "learning"
        agent: "cosca-backend"
        learning_id: "L31"
        title: "Provider X não é mais utilizado por nenhum serviço core"
        confidence: 0.75
        recency_days: 7
        severity: "média"
        detail: "Learning do próprio cosca-backend afirma que provider X não tem mais usuários conhecidos."

  # ── SCORE ──────────────────────────────────────────────────────────────

  contradiction_score:
    count_evidencias_contra: 5
    count_normalized: min(5 / 10, 1.0) = 0.500
    count_term: 0.500 × 0.40 = 0.200

    avg_confidence_contra: (0.92 + 0.88 + 0.85 + 0.60 + 0.88) / 5 = 0.826
    confidence_term: 0.826 × 0.30 = 0.248

    avg_recency_days: (3 + 45 + 15 + 12 + 15) / 5 = 18
    recency_normalized: 1 - min(18 / 365, 1.0) = 0.951
    recency_term: 0.951 × 0.20 = 0.190

    max_severity: 1.00 (crítica)
    severity_term: 1.00 × 0.10 = 0.100

    final_score: 0.200 + 0.248 + 0.190 + 0.100 = 0.738

  # ── AÇÃO ────────────────────────────────────────────────────────────────

  action:
    threshold_result: "score > 0.7 → BLOQUEIA"
    action_taken: "block_and_escalate"
    escalation_to: "don"
    summary: |
      5 evidências CONTRA encontradas (score 0.738):
      E-001: Learning L29 exige verificação de dependências (conf: 0.92)
      E-002: Failure F005: remoção similar causou downtime 45min (conf: 0.88)
      E-003: DDNA anterior decidiu MANTER provider X (conf: 0.85)
      E-004: Agente cosca-backend tem 40% de sucesso no domínio (conf: 0.60)
      E-005: Padrão histórico: remoção de provider → incidente (conf: 0.88)

      1 evidência A FAVOR:
      F-001: Learning L31 diz que provider não é mais usado (conf: 0.75)

      RECOMENDAÇÃO: BLOQUEAR. Contradição forte (score 0.738 > 0.7).
      Escalonar ao Don para decisão final.
      
    recommendation: "BLOCK — 5 evidências CONTRA vs 1 a FAVOR. Score 0.738 > 0.7 threshold."

  # ── DON RESPONSE (preenchido após decisão do Don) ──────────────────────

  don_response:
    decided_by: "don"
    decision: "override"            # Don fez override
    rationale: "Provider X foi substituído internamente na última semana. As dependências documentadas no DDNA-2026-07-15-003 foram migradas. O padrão T-007 não se aplica porque as remoções anteriores eram de providers com dependências ativas."
    override_id: "OVR-2026-07-30-001"
    override_type: "single_decision"
    override_expires: "2026-10-30"
    condition: "Executar go list -deps antes de remover (conforme L29)"
    timestamp: "2026-07-30T14:05:00Z"

  # ── RASTREABILIDADE ────────────────────────────────────────────────────

  traceability:
    gate_activated: false           # Não ativou Gate (foi direto ao Don)
    entropy_updated: true           # Entropia alimentada
    ddna_created: "DDNA-CONTRADICTION-2026-07-30-001"
    override_registered: true
    evidence_archived: true
```

### 10.3 Resultado Pós-Contradiction Engine

| Aspecto | Sem Engine | Com Engine v2 |
|---------|-----------|---------------|
| Decisão | "Remover provider X" (sem questionamento) | "Remover MAS com `go list -deps` antes (condição do Don)" |
| Risco identificado | Nenhum | 5 evidências contra (learning, failure, ddna, trust, timeline) |
| Contradições expostas | 0 | 5 evidências de contradição |
| Don envolvido? | Não (decisão do Kernel) | Sim (escalado por score > 0.7) |
| Condição de segurança | Nenhuma | `go list -deps` mandatório |
| Rastreabilidade | Nenhuma | DDNA de contradição criado, override registrado |

---

## 11. MÉTRICAS DO ENGINE

### 11.1 Métricas Internas

| Métrica | Tipo | Descrição | Alerta |
|---------|------|-----------|--------|
| `contradiction.queries.total` | Counter | Total de consultas (minerações) realizadas | — |
| `contradiction.queries.by_trigger` | Counter | Consultas por gatilho (decision/wisdom_decay/scheduled) | — |
| `contradiction.evidence.found` | Counter | Total de evidências de contradição encontradas | — |
| `contradiction.evidence.by_source` | Counter | Evidências por fonte (learning/failure/ddna/trust/timeline) | — |
| `contradiction.score.avg` | Histogram | Score médio de contradição por consulta | Se > 0.5, sistema pode estar com muitas contradições |
| `contradiction.score.max` | Gauge | Maior score ativo no momento | Se > 0.7, há bloqueio pendente |
| `contradiction.actions` | Counter | Ações tomadas (block/escalate/document) | Se block > 10% das consultas, revisar qualidade das decisões |
| `contradiction.false_positives` | Counter | Falsos positivos detectados (Don override por "não é contradição") | Se > 20%, revisar filtros |
| `contradiction.duration_ms` | Histogram | Tempo de execução do pipeline | Se > 500ms, otimizar |
| `contradiction.override.count` | Counter | Total de overrides do Don | Se > 5/domínio, contradição pode ser inválida |
| `contradiction.entropy.feed` | Counter | Alimentações para Cognitive Entropy | Deve ser ≈ total de contradições ativas |

### 11.2 Thresholds de Alerta

| Condição | Severidade | Ação |
|----------|------------|------|
| Média de score > 0.5 por 3 dias consecutivos | 🟡 MÉDIO | Revisar fonte das contradições |
| > 30% das consultas resultam em block | 🔴 ALTO | Qualidade das decisões proposta está baixa |
| Pipeline > 500ms | 🔴 ALTO | Otimizar consultas |
| Falsos positivos > 20% | 🟡 MÉDIO | Revisar filtros anti-falso-positivo |
| Override count por domínio > 5 | 🟡 MÉDIO | Contradição pode ser inválida, revisar |

---

## 12. CLI E AUTOMAÇÃO

### 12.1 Comandos

```bash
# Minerar contradições para uma decisão específica
cosca contradiction mine "remover provider X sem verificar dependências"

# Minerar contradições para uma decisão com domínio explícito
cosca contradiction mine "remover provider X" --domain provider-management

# Ver score de contradição atual para um domínio
cosca contradiction score --domain provider-management

# Ver todas as contradições ativas
cosca contradiction list --status active

# Ver detalhes de uma contradição específica
cosca contradiction inspect C-004

# Registrar Don override
cosca contradiction override C-004 --reason "Contexto mudou" --scope single_decision

# Ver histórico de overrides
cosca contradiction overrides

# Executar scan programado manualmente
cosca contradiction scan

# Ver estatísticas do engine
cosca contradiction stats

# Executar em modo dry-run (não alimenta entropia, não ativa gate)
cosca contradiction mine "decisão" --dry-run

# Output JSON para integração
cosca contradiction mine "decisão" --format json
```

### 12.2 Opções

| Flag | Descrição | Default |
|------|-----------|---------|
| `--domain` | Domínio da decisão (para matching preciso) | Detectado automaticamente |
| `--dry-run` | Não alimenta entropia, não ativa gate | `false` |
| `--format` | Formato de output: `table`, `json`, `yaml` | `table` |
| `--threshold-block` | Threshold de bloqueio customizado | `0.7` |
| `--threshold-gate` | Threshold de abertura de gate customizado | `0.4` |
| `--sources` | Fontes a consultar (ex: `--sources learnings,failures`) | `all` |
| `--min-confidence` | Confidence mínimo para considerar evidência | `0.30` |
| `--max-recency` | Recência máxima em dias | `365` |

### 12.3 Scheduler

```bash
# Executar scan programado diariamente à meia-noite
0 0 * * * cosca contradiction scan --format json >> /var/log/contradiction.log

# Verificar contradições antes de cada commit (pre-commit hook)
cosca contradiction mine "diff --cached" --domain code-quality --dry-run
```

### 12.4 CI Gate

```yaml
# .github/workflows/contradiction-check.yml
name: Contradiction Check
on:
  pull_request:
    types: [opened, synchronize]
  workflow_dispatch:

jobs:
  contradiction-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Run Contradiction Scan
        run: |
          cosca contradiction scan --format json --output contradiction-report.json
      
      - name: Check for Blocking Contradictions
        run: |
          SCORE=$(jq '.contradiction_score' contradiction-report.json)
          if [ "$(echo "$SCORE > 0.7" | bc)" -eq 1 ]; then
            echo "❌ Contradiction score $SCORE exceeds block threshold 0.7"
            echo "📋 Blocking contradictions found:"
            jq -r '.evidence.contra[] | "  • \(.title) (conf: \(.confidence))"' contradiction-report.json
            exit 1
          fi
          echo "✅ Contradiction score $SCORE is within safe range"
```

---

## 13. RELACIONADOS

| Documento | Relação | Localização |
|-----------|---------|-------------|
| **Contrafactual Gate (F1.2)** | Gate ativado por este engine quando score 0.4–0.7 | `workflows/contrafactual-gate.md` |
| **Wisdom Decay (F1.4)** | Fonte de contradições entre aprendizados; contradição decay | `engines/wisdom-decay/WISDOM_DECAY.md` |
| **Cognitive Entropy (F1.6)** | Consome contradições como componente de entropia (peso 3) | `engines/cognitive-entropy/ENTROPY.md` |
| **Prediction Engine (F7.1)** | Usa contradições como penalidade na P(success) | `engines/prediction/SKILL.md` |
| **Trust Registry (F7.2)** | Fonte de confiança histórica do agente proponente | `memory/trust/TRUST_REGISTRY.md` |
| **Decision DNA (F1.1)** | Formato canônico de decisão; contradições geram DDNA de conflito | `knowledge/architecture/DECISION_DNA.md` |
| **Confidence Model** | M5: Contradita por fonte superior (-0.40) | `engines/evidence/CONFIDENCE_MODEL.md` |
| **Immune System (F2.2)** | Antígenos cognitivos — contradição como patologia | `engines/immune-system/SKILL.md` |
| **Forward Projection (F3.3)** | Árvore de consequências — contradições como nós de risco | `engines/second-order-reasoning/PROJECTION.md` |
| **Second-Order Reasoning (F3.2)** | Meta-análise dos padrões de decisão — contradições como dado de viés de confirmação | `engines/second-order-reasoning/SKILL.md` |
| **Cognitive Economy (F2.1)** | Entropia (alimentada por contradições) como risk_penalty | `engines/cognitive-economy/SKILL.md` |
| **DECISION_DNA_FORMAT.md** | Versão agent-facing do DDNA | `memory/DECISION_DNA_FORMAT.md` |
| **CONSTITUTION.md** | P5 — A família aprende com erros (failures alimentam contradições) | `CONSTITUTION.md` |

---

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|--------|------|-------|------------|
| 2.0.0 | 2026-07-30 | Architecture Chief | Criação da Contradiction Engine v2 (F8.3): pipeline completo, 5 fontes de contradição (learnings, failures, DDNAs, Trust Registry, Timeline), score de contradição com 4 componentes, integração com Gate (F1.2), Entropy (F1.6), Prediction (F7.1), Don Override, exemplo completo, métricas, CLI |

---

> **Owner**: Architecture Chief | **Invoked by**: Kernel (pré-delegação de task P0/P1) | **Complementar a**: Contrafactual Gate (F1.2)
>
> *"Não espere a contradição aparecer — vá caçá-la."*
