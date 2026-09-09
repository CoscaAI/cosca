# COGNITIVE ENTROPY ENGINE — Medidor de Desorganização do Conhecimento (F1.6)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
> **CMI Dimension**: Consistência | **Bloco Cognitivo**: Bloco 1 — Memória & Conhecimento
> **Fase CMI**: Fase 1 — Foundation | **Código**: F1.6
> **Dependências**: F1.1 DDNA | F1.3 Gap Detection | F1.4 Wisdom Decay | F1.5 Metrics B1-B5
>
> Consulte também:
> - [COGNITIVE_ENTROPY.md (analytics)](../../analytics/COGNITIVE_ENTROPY.md) — dashboard de entropia (5 componentes, baseline 64.5%)
> - [entropy-baseline-2026-07-30.md](../../analytics/entropy-baseline-2026-07-30.md) — baseline detalhada com contradições, fragmentações, órfãos
> - [WISDOM_DECAY.md](../../engines/wisdom-decay/WISDOM_DECAY.md) — F1.4: freshness score, contradictions, stale detection
> - [gap-detection/SKILL.md](../../engines/gap-detection/SKILL.md) — F1.3: 6 dimensões de gap, gap score, DDNA automático
> - [cognitive-metrics.md](../../analytics/cognitive-metrics.md) — F1.5: B1-B5, Cognitive Health Index, ROI cognitivo
> - [cognitive-economy/SKILL.md](../../engines/cognitive-economy/SKILL.md) — F2.1: motor de economia cognitiva, efficiency score
> - [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) — C2 Cognitive Entropy conceito original

---

## SUMÁRIO

1. [Definição](#1-definição)
2. [Fórmula de Entropia Cognitiva](#2-fórmula-de-entropia-cognitiva)
3. [Os 3 Componentes de Entropia](#3-os-3-componentes-de-entropia)
4. [Pipeline de Cálculo](#4-pipeline-de-cálculo)
5. [Thresholds e Alertas](#5-thresholds-e-alertas)
6. [Série Histórica e Tendência](#6-série-histórica-e-tendência)
7. [Integração com F2.1 Cognitive Economy](#7-integração-com-f21-cognitive-economy)
8. [Integração com F1.5 Cognitive Metrics](#8-integração-com-f15-cognitive-metrics)
9. [CLI e Automação](#9-cli-e-automação)
10. [Exemplo com Dados Reais](#10-exemplo-com-dados-reais)
11. [Plano de Redução de Entropia](#11-plano-de-redução-de-entropia)
12. [Métricas do Engine](#12-métricas-do-engine)

---

## 1. DEFINIÇÃO

### 1.1 O que é Entropia Cognitiva

**Entropia Cognitiva** é a medida de **desorganização do conhecimento** no ecossistema Cosca. Inspirada na segunda lei da termodinâmica — onde a entropia mede o grau de desordem molecular de um sistema — a entropia cognitiva mede o grau de desordem **informacional**: quão contraditório, obsoleto ou fragmentado está o conhecimento acumulado pelos 54 agentes.

### 1.2 Analogia Termodinâmica

```
┌─────────────────────────────────────────────────────────────────────┐
│               ENTROPIA TERMODINÂMICA ←→ ENTROPIA COGNITIVA           │
│                                                                      │
│  Moléculas desorganizadas          ←→  Learnings contraditórios     │
│  (gás: moléculas colidindo         │   (afirmações opostas sobre    │
│   em direções aleatórias)          │    o mesmo fato)               │
│                                                                      │
│  Moléculas estagnadas              ←→  Conhecimento stale           │
│  (cristal: sem movimento,          │   (aprendizados não usados     │
│   preso no lugar)                  │    há semanas/meses)           │
│                                                                      │
│  Moléculas isoladas                ←→  Gaps sem rastro              │
│  (gás nobre: não interage          │   (problemas detectados mas    │
│   com o resto)                     │    sem DDNA ou decisão)        │
│                                                                      │
│  2ª Lei: entropia sempre           ←→  Sem curadoria ativa,         │
│  aumenta em sistema isolado        │    conhecimento tende ao caos  │
│                                                                      │
│  Trabalho externo (energia)        ←→  Curadoria, compressão,       │
│  reduz entropia (organiza)         │    decisões registradas        │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.3 Propósito

| Por que medir | O que revela | Ação resultante |
|---------------|-------------|-----------------|
| Detectar deterioração do conhecimento | Aviso precoce de que a base de conhecimento está se degradando | Disparar curadoria antes que cause decisões erradas |
| Quantificar confiabilidade das decisões | Quanto mais entropia, menos confiável é o conhecimento que embasa as decisões | Ajustar confidence threshold nas decisões |
| Priorizar ações de organização | Quais componentes mais contribuem para a desordem | Foco em contradições primeiro (maior peso), depois stale knowledge, depois gaps |
| Qualificar ROI cognitivo (F2.1) | Conhecimento desorganizado aumenta o custo de cada decisão | Entropia como risk qualifier no efficiency score |

### 1.4 Diferença da Especificação de Analytics

O documento [`analytics/COGNITIVE_ENTROPY.md`](../../analytics/COGNITIVE_ENTROPY.md) define a **métrica de dashboard** com 5 componentes (Contradiction, Staleness, Fragmentation, Orphan Ratio, Consolidation Gap) e pontuação 0-100%.

**Este documento** define o **engine de cálculo operacional** — uma fórmula simplificada de 3 componentes, projetada para:
- **Cálculo rápido e determinístico** (O(1) após coleta dos inputs de F1.3 e F1.4)
- **Integração direta com F2.1** (Cognitive Economy usa entropia como qualificador de risco)
- **Alerta automático** com thresholds fixos (🟢/🟡/🔴)
- **Série histórica** semanal no timeline CSV

A métrica de analytics (0-100%) é o instrumento de **diagnóstico profundo** (5 componentes, adequado para auditoria mensal). Este engine (0.0-1.0) é o instrumento de **monitoramento contínuo** (3 componentes, adequado para alerta semanal).

---

## 2. FÓRMULA DE ENTROPIA COGNITIVA

### 2.1 Fórmula Principal

```
CognitiveEntropy = Σ(contradictions × 3  +  stale_knowledge × 2  +  gaps × 1)
                   ───────────────────────────────────────────────────────────
                                        total_knowledge
```

**Onde cada termo representa uma dimensão de desorganização:**

| Termo | Variável | Peso | Fonte | Descrição |
|-------|----------|------|-------|-----------|
| Contradições | `contradictions` | **3** | F1.4 Wisdom Decay (§2.3) | Pares de learnings que se contradizem. Peso 3 porque contradições ativas envenenam o conhecimento — agentes tomam decisões opostas baseadas na mesma base. |
| Conhecimento Stale | `stale_knowledge` | **2** | F1.4 Wisdom Decay (§4) | Learnings com freshness < 0.30. Peso 2 porque conhecimento obsoleto é silencioso — não contradiz, mas engana quem confia. |
| Gaps sem DDNA | `gaps` | **1** | F1.3 Gap Detection | Gaps abertos detectados por F1.3 que não possuem DDNA vinculado. Peso 1 porque são problemas conhecidos — pelo menos sabemos que existem. |

### 2.2 Por que Pesos Diferentes?

A ponderação reflete o **impacto na qualidade das decisões**:

```
Peso 3 — Contradições: dano ativo
  Duas fontes dizem o oposto sobre o mesmo fato.
  Agente A decide baseado na fonte X, Agente B na fonte Y.
  Resultado: decisões inconsistentes, retrabalho, conflito.
  Custo: imediato e visível.

Peso 2 — Stale Knowledge: dano silencioso
  O learning diz algo que era verdade há 6 meses mas não é mais.
  Agente consulta, confia (freshness não era zero), age errado.
  Resultado: decisão incorreta baseada em conhecimento vencido.
  Custo: só aparece quando o erro se manifesta.

Peso 1 — Gaps sem DDNA: dano potencial
  Sabemos que existe um gap (F1.3 detectou), mas não há decisão
  registrada sobre o que fazer. O gap está documentado, não ignorado.
  Resultado: risco conhecido e monitorado.
  Custo: baixo enquanto monitorado, alto se ignorado por muito tempo.
```

### 2.3 Interpretação do Score

O resultado é um número entre **0.0 e 1.0** (normalizado por `total_knowledge`):

| Score | Interpretação |
|-------|---------------|
| **0.000** | Conhecimento perfeitamente organizado. Zero contradições, zero stale, zero gaps sem rastro. |
| **0.100** | 1 contradição em ~30 learnings OU 3 gaps sem DDNA. Aceitável, requer atenção. |
| **0.250** | 3 contradições OU 4 stale + 2 gaps. Limiar de alerta. |
| **0.500** | 5 contradições em 30 learnings = metade do conhecimento comprometido. Crítico. |
| **1.000** | Entropia máxima. Cada learning está contradito ou stale, todo gap sem DDNA. |

---

## 3. OS 3 COMPONENTES DE ENTROPIA

### 3.1 Contradictions (Peso 3)

**Fonte**: F1.4 Wisdom Decay — mecanismo de *Contradiction Decay* (§2.3)

**O que conta**: Pares de learnings em que uma afirmação contradiz a outra. Detectado por:
1. **Tags em comum > 60%** — dois aprendizados sobre o mesmo domínio
2. **Conteúdo semanticamente oposto** — afirmação A vs não-A
3. **Freshness de ambos > 0.30** — se um já está depreciado, não há conflito ativo

**Formato de entrada** (alimentado por Wisdom Decay):

```yaml
contradictions_detected:
  - pair_id: "C-001"
    learning_a: "L20 — Runtime coverage é 97.9%"
    learning_b: "L21 — Runtime coverage é 78.3%"
    domain: "runtime-coverage"
    detected_at: "2026-07-30"
    status: "active"       # active | resolved | dismissed
    resolution: "L21 referia-se a estado anterior ao refactor"
```

**Regra de contagem**: Cada par de contradição ativa conta como 1 unidade. Uma contradição resolvida não conta.

**Exemplo no baseline atual**:

| Par | Aprendizado A | Aprendizado B | Domínio | Status |
|-----|---------------|---------------|---------|--------|
| C-001 | PostgreSQL fantasy (`database-architecture.md`) | SQLite real (`go.mod`) | Stack de BD | ✅ Resolvida |
| C-002 | Threshold 40/55/70/80% (4 fontes) | CI executa 55% | Cobertura | ✅ Resolvida |
| C-003 | Level 3 (`capability-profile.md`) | 7 entradas Level 4 (`learnings.md`) | Capacidade | ❌ Ativa |

**Contagem atual**: **1** contradição ativa (C-003).

---

### 3.2 Stale Knowledge (Peso 2)

**Fonte**: F1.4 Wisdom Decay — classe `DEPRECATED` e `REVALIDATE_NOW`

**O que conta**: Learnings com `freshness_score < 0.30`, segundo a fórmula completa do Wisdom Decay:

```
freshness = recency_term × 0.4 + usage_term × 0.3 + consistency_term × 0.2 + review_term × 0.1
```

**Threshold de stale**:

| Freshness | Classificação | Conta como stale? |
|-----------|--------------|-------------------|
| ≥ 0.70 | 🟢 HEALTHY | ❌ |
| 0.30 ≤ f < 0.70 | 🟡 REVALIDATE_30 | ❌ (requer atenção mas não é stale) |
| 0.15 ≤ f < 0.30 | 🟠 REVALIDATE_NOW | ✅ Conta como stale |
| < 0.15 | 🔴 DEPRECATED | ✅ Conta como stale |

**Formato de entrada** (alimentado por Wisdom Decay):

```yaml
stale_learnings:
  - id: "L00"
    agent: "cosca-kernel"
    freshness: 0.12
    classification: "DEPRECATED"
    days_since_last_use: 180
  - id: "L00"
    agent: "cosca-kernel"
    freshness: 0.22
    classification: "REVALIDATE_NOW"
    days_since_last_use: 95
```

**Regra de contagem**: Cada learning com freshness < 0.30 conta como 1 unidade. Se o mesmo learning estiver em contradição ativa, ele conta NOS DOIS — contradição e stale são dimensões ortogonais.

**Contagem atual**: **0** stale (todos os 30 learnings do Kernel têm freshness > 0.70).

> **Por que zero?** O sistema tem apenas ~6 dias de idade (primeiro learning: 2026-07-27). Todos os aprendizados são recentes e foram usados múltiplas vezes. Em um sistema maduro (6+ meses), a contagem de stale será significativa.

---

### 3.3 Gaps sem DDNA (Peso 1)

**Fonte**: F1.3 Proactive Gap Detection Engine

**O que conta**: Gaps detectados pelo F1.3 em qualquer das 6 dimensões (GAP_COVERAGE, GAP_DEBT, GAP_DOCS, GAP_SECURITY, GAP_ARCH, GAP_MEMORY) que:
1. Estão com status `open` ou `in_progress` no Gap Registry
2. **Não** possuem DDNA vinculado (`ddna_id` vazio no registro do gap)

**Regra de contagem**: Cada gap no registry sem DDNA conta como 1 unidade. Gaps com DDNA (mesmo que não resolvidos) não contam — a decisão foi registrada, o rastro existe.

**Formato de entrada** (alimentado por F1.3 Gap Detection):

```yaml
open_gaps_without_ddna:
  - gap_id: "GAP-20260730-0001"
    dimension: "GAP_ARCH"
    description: "Orphan refs: MEMORY_MODEL.md referencia engines não implementados"
    severity: "MEDIUM"
    score: 45
    detected_at: "2026-07-30"
    ddna_id: ""              # ← VAZIO = conta como gap
  - gap_id: "GAP-20260730-0002"
    dimension: "GAP_MEMORY"
    description: "capability-profile.md do kernel desatualizado (Level 3 vs Level 4)"
    severity: "MEDIUM"
    score: 40
    detected_at: "2026-07-30"
    ddna_id: ""              # ← VAZIO = conta como gap
```

**Contagem atual**: **4** gaps sem DDNA (detalhados na seção 10).

---

## 4. PIPELINE DE CÁLCULO

### 4.1 Visão Geral

```
┌──────────────────────────────────────────────────────────────────────────┐
│                      COGNITIVE ENTROPY PIPELINE                           │
│                         Cálculo Semanal                                   │
│                                                                           │
│  TRIGGERS:                                                                │
│  ┌──────────────┐  ┌─────────────────┐  ┌──────────────┐                 │
│  │ Cron: Semanal │  │ Wisdom Decay    │  │ Gap Detection│                 │
│  │ (Dom 00:00)  │  │ execução completa│  │ execução     │                 │
│  └──────┬───────┘  └────────┬────────┘  └──────┬───────┘                 │
│         │                   │                   │                          │
│         └───────────────────┼───────────────────┘                          │
│                             │                                              │
│                             ▼                                              │
│  ┌────────────────────────────────────────────────────────────────────┐   │
│  │ FASE 1: COLETA (de F1.3 + F1.4)                                    │   │
│  │                                                                     │   │
│  │  ├── De F1.4 Wisdom Decay:                                         │   │
│  │  │   ├── contradictions_detected → filtrar active → count          │   │
│  │  │   └── stale_learnings → freshness < 0.30 → count               │   │
│  │  │                                                                  │   │
│  │  ├── De F1.3 Gap Detection:                                        │   │
│  │  │   └── open_gaps_without_ddna → count                            │   │
│  │  │                                                                  │   │
│  │  └── De Knowledge Inventory:                                       │   │
│  │       └── total_knowledge = Σ learnings (todos agentes)            │   │
│  │                                                                     │   │
│  │  ⏱ Tempo alvo: < 2s (coleta de dados já processados)              │   │
│  └────────────────────────────────┬───────────────────────────────────┘   │
│                                   │                                       │
│                                   ▼                                       │
│  ┌────────────────────────────────────────────────────────────────────┐   │
│  │ FASE 2: CÁLCULO                                                     │   │
│  │                                                                     │   │
│  │  raw_entropy = (contradictions × 3) + (stale × 2) + (gaps × 1)    │   │
│  │  entropy = raw_entropy / total_knowledge                             │   │
│  │                                                                     │   │
│  │  classification = classify(entropy)                                  │   │
│  │  trend = compare_with_last_week(entropy)                             │   │
│  │                                                                     │   │
│  │  ⏱ Tempo alvo: < 100ms (cálculo O(1))                             │   │
│  └────────────────────────────────┬───────────────────────────────────┘   │
│                                   │                                       │
│                                   ▼                                       │
│  ┌────────────────────────────────────────────────────────────────────┐   │
│  │ FASE 3: AÇÃO                                                        │   │
│  │                                                                     │   │
│  │  ├── Registrar no timeline CSV (entropy-timeline.csv)               │   │
│  │  ├── Emitir alerta SE threshold violado                             │   │
│  │  ├── Se entropy > 0.25:                                             │   │
│  │  │   ├── 🔴 ALERTA para Architecture Chief + Kernel                 │   │
│  │  │   ├── 🚫 Bloquear decisões automáticas (exigir Don approval)     │   │
│  │  │   └── 📋 Gerar relatório de "Top 3 fontes de entropia"           │   │
│  │  └── Se entropy subiu > 20% em 2 semanas consecutivas:             │   │
│  │       └── 🔴 ALERTA P1: "Entropia acelerando. Curadoria obrigatória"│   │
│  │                                                                     │   │
│  │  ⏱ Tempo alvo: < 1s                                               │   │
│  └────────────────────────────────────────────────────────────────────┘   │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Gatilhos de Execução

| Gatilho | Disparado por | Quando | Ação |
|---------|--------------|--------|------|
| **Semanal** | Cron scheduler | Domingo 00:00 | Pipeline completo (F1-F3) |
| **Pós Wisdom Decay** | F1.4 pipeline | Após cada execução do Wisdom Decay | Recalcular entropia se contradições ou stale mudaram |
| **Pós Gap Detection** | F1.3 pipeline | Após cada detecção de gap P0 | Recalcular entropia se gaps sem DDNA mudaram |
| **Manual** | CLI `cosca entropy calc` | On-demand | Pipeline completo |
| **Pós-auditoria** | Cognitive Audit Loop | Após auditoria de knowledge drift | Recalcular entropia |

### 4.3 Algoritmo

```
function calculate_cognitive_entropy():
    // Fase 1: Coleta de dados
    contradictions = wisdom_decay.get_active_contradictions()     // de F1.4
    stale = wisdom_decay.get_stale_learnings(freshness < 0.30)    // de F1.4
    gaps = gap_detection.get_open_gaps_without_ddna()             // de F1.3
    total = knowledge_inventory.count_total_learnings()

    // Fase 2: Cálculo
    raw = (contradictions.count × 3) + (stale.count × 2) + (gaps.count × 1)
    entropy = raw / max(total, 1)  // evitar divisão por zero

    // Fase 3: Classificação
    if entropy < 0.10:
        classification = "LOW"       // 🟢
    elif entropy <= 0.25:
        classification = "MEDIUM"    // 🟡
    else:
        classification = "HIGH"      // 🔴

    // Fase 4: Tendência
    last = load_last_measurement()
    delta = entropy - last.entropy
    if delta > 0:
        trend = "↑ worsening"
    elif delta < 0:
        trend = "↓ improving"
    else:
        trend = "→ stable"

    // Fase 5: Alerta
    if classification == "HIGH":
        alert("🔴 Cognitive Entropy HIGH: #{entropy}")
        if classification == last.classification:
            consecutive_high_weeks += 1
            if consecutive_high_weeks >= 2:
                alert_p1("Entropy HIGH for 2+ consecutive weeks. Mandatory curation.")

    // Fase 6: Registro
    record = {
        timestamp: now(),
        entropy: entropy,
        contradictions: contradictions.count,
        stale: stale.count,
        gaps: gaps.count,
        total_knowledge: total,
        classification: classification,
        trend: trend,
        alert: classification == "HIGH"
    }
    append_to_timeline(record)

    return record
```

### 4.4 Performance

| Operação | Tempo |
|----------|-------|
| Coleta de contradições (de F1.4) | < 500ms |
| Coleta de stale (de F1.4) | < 300ms |
| Coleta de gaps (de F1.3) | < 500ms |
| Contagem total de knowledge | < 100ms |
| Cálculo | < 50ms |
| Registro em timeline CSV | < 50ms |
| **Total** | **< 1.5s** |

---

## 5. THRESHOLDS E ALERTAS

### 5.1 Thresholds de Classificação

```
               ┌──────────────────────────────────────────────────────────┐
               │                 ENTROPIA COGNITIVA                        │
               │                                                          │
               │    0.00                    0.10            0.25        → │
               │      │──────────────────────│──────────────│            │ │
               │      │      🟢 BAIXA        │  🟡 MÉDIA     │  🔴 ALTA  │ │
               │      │                      │               │           │ │
               │      │ Conhecimento         │ Revisão       │ Revisão   │ │
               │      │ organizado           │ recomendada   │ obrigatória│ │
               │      │                      │               │           │ │
               └──────────────────────────────────────────────────────────┘
```

| Threshold | Classificação | Cor | Significado | Ação |
|-----------|--------------|-----|-------------|------|
| **< 0.10** | 🟢 Baixa | Verde | Conhecimento organizado. Contradições mínimas, conhecimento fresco, gaps com rastro. | Monitoramento passivo. Medir semanalmente. |
| **0.10 – 0.25** | 🟡 Média | Amarelo | Conhecimento com desorganização moderada. Algumas contradições ou stale detectados. | Revisão recomendada. Priorizar top 3 fontes de entropia na próxima auditoria. |
| **> 0.25** | 🔴 Alta | Vermelho | Conhecimento significativamente desorganizado. Contradições ativas comprometem decisões. | **Revisão obrigatória.** Bloquear decisões automáticas baseadas em conhecimento não verificado. Gerar relatório de correção. |

### 5.2 Matriz de Alertas

| Condição | Severidade | Canal | Ação |
|----------|-----------|-------|------|
| `entropy > 0.25` | 🔴 P1 | Console + Kernel notify | Revisão obrigatória. Bloquear decisões automáticas. Gerar relatório. |
| `entropy subiu > 20% em 2 semanas` | 🔴 P1 | Console + Kernel notify | "Entropia acelerando". Curadoria obrigatória. |
| `entropy 0.10–0.25` | 🟡 P2 | Console warning | Revisão recomendada na próxima auditoria. |
| `entropy > 0.25 por 4+ semanas` | 🔴 P0 | Kernel notify + Don | "Entropia crônica". Sistema precisa de intervenção estrutural. |
| `entropy cai > 50%` | 🟢 Informativo | Log | "Melhoria significativa. Ações de curadoria estão funcionando." |

### 5.3 Ações por Threshold

#### 🟢 Baixa (< 0.10)

```yaml
actions:
  - type: "monitor"
    description: "Medir semanalmente. Nenhuma ação corretiva necessária."
  - type: "log"
    description: "Registrar no timeline CSV para tendência histórica."
```

#### 🟡 Média (0.10 – 0.25)

```yaml
actions:
  - type: "review"
    description: "Revisão recomendada. Identificar top 3 contribuidores de entropia."
    triggers:
      - "Agendar task de revisão para Architecture Chief"
      - "Incluir no relatório semanal de saúde cognitiva"
  - type: "warn"
    description: "Notificar agentes afetados (donos de learnings contraditórios/stale)."
```

#### 🔴 Alta (> 0.25)

```yaml
actions:
  - type: "block"
    description: "Bloquear decisões automáticas baseadas em conhecimento não verificado."
    mechanism: |
      Cognitive Economy Engine (F2.1) recebe risk_qualifier = "high_entropy"
      → Decision Ladder reduz em 1 nível qualquer efficiency score
      → Ações com score < 1.0 são automaticamente SKIP ou DEFER
    override:
      - "Comando explícito do Don"
      - "Ações de segurança crítica (P0)"

  - type: "report"
    description: "Gerar relatório de 'Top 3 Fontes de Entropia'."
    format: |
      #1: [componente] — [descrição] — [impacto no score]
      #2: ...
      #3: ...
      Ação corretiva recomendada: [descrição]

  - type: "notify"
    description: "Notificar Architecture Chief + Kernel + Don (se persistente)."
    channels:
      - "Console alert (kernel)"
      - "Relatório semanal de saúde cognitiva"
      - "Log de incidente de entropia"
```

---

## 6. SÉRIE HISTÓRICA E TENDÊNCIA

### 6.1 Armazenamento — Timeline CSV

```
internal/embed/cosca/memory/timeline/
└── cognitive-entropy.csv
```

**Schema**:

```csv
timestamp,entropy,contradictions,stale,gaps,total_knowledge,classification,trend,alert
2026-07-30T00:00:00Z,0.433,1,0,4,30,HIGH,→,true
```

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `timestamp` | ISO 8601 | Data da medição |
| `entropy` | float (0.0-1.0) | Cognitive Entropy calculado |
| `contradictions` | int | Contagem de contradições ativas |
| `stale` | int | Contagem de learnings stale (freshness < 0.30) |
| `gaps` | int | Contagem de gaps sem DDNA |
| `total_knowledge` | int | Total de learnings na base |
| `classification` | string | LOW / MEDIUM / HIGH |
| `trend` | string | → estável / ↑ piorando / ↓ melhorando |
| `alert` | boolean | True se threshold violado |

### 6.2 Indicador de Tendência

Comparação com a medição anterior:

| Δ | Símbolo | Significado | Threshold |
|---|---------|-------------|-----------|
| **Aumento** | ↑ | Entropia aumentou (piorou) | Δ > +0.02 desde última medição |
| **Estável** | → | Entropia não mudou significativamente | -0.02 ≤ Δ ≤ +0.02 |
| **Queda** | ↓ | Entropia diminuiu (melhorou) | Δ < -0.02 desde última medição |

### 6.3 Dashboard Conceitual

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         COGNITIVE ENTROPY DASHBOARD                        │
│                              2026-07-30 — Baseline                        │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│   SCORE:  0.433                      CLASSIFICAÇÃO: 🔴 ALTA              │
│           ████████████████████████████████░░░░░░░░                        │
│                                                                           │
│   COMPONENTES:                        TENDÊNCIA:  → (primeira medição)   │
│   ┌──────────────┬──────┬───────┐                                         │
│   │ Componente   │ Cont │ Peso  │                                         │
│   ├──────────────┼──────┼───────┤                                         │
│   │ Contradictions│  1   │  ×3   │  ███                                   │
│   │ Stale        │  0   │  ×2   │  (vazio)                               │
│   │ Gaps sem DDNA│  4   │  ×1   │  ████████                              │
│   └──────────────┴──────┴───────┘                                         │
│                                                                           │
│   SÉRIE HISTÓRICA:                                                        │
│   ┌────────────────────────────────────────────────────┐                 │
│   │ 0.500┤                                            │                 │
│   │      │         ● (baseline)                        │                 │
│   │ 0.250┤                                            │                 │
│   │      │                                            │                 │
│   │ 0.000┼────┬────┬────┬────┬────┬────┬────┬────┬────│                 │
│   │     30/7 06/08 13/08 20/08 27/08 03/09 10/09     │                 │
│   └────────────────────────────────────────────────────┘                 │
│                                                                           │
│   ALERTAS ATIVOS:                                                         │
│   🔴 [P1] Entropia 0.433 > 0.25 — Revisão obrigatória.                   │
│   🟡 [P2] 4 gaps sem DDNA detectados — F1.3 recomenda criar DDNAs.      │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 7. INTEGRAÇÃO COM F2.1 COGNITIVE ECONOMY

### 7.1 Entropia como Qualificador de Risco

A **Cognitive Economy Engine** (F2.1) calcula o `efficiency_score` de cada ação como:

```
efficiency_score = (valor_total × value_multiplier) / (custo_total × urgency_factor)
```

A **entropia cognitiva** entra como um **qualificador de risco** que reduz o valor esperado e/ou aumenta o custo esperado de ações que dependem de conhecimento:

```
efficiency_score_adjusted = efficiency_score × (1 - risk_penalty)

Onde:
  risk_penalty = entropy × knowledge_dependency_factor

  knowledge_dependency_factor:
    1.0  — Ação depende CRITICAMENTE de conhecimento (ex: decisão arquitetural baseada em learnings)
    0.5  — Ação usa conhecimento moderadamente (ex: auditoria de código com contexto histórico)
    0.0  — Ação independente de conhecimento (ex: rodar `go build`)
```

### 7.2 Efeito Prático

| Entropia | knowledge_dependency | risk_penalty | Impacto no efficiency_score |
|----------|---------------------|--------------|---------------------------|
| **0.433** (🔴 alta) | 1.0 (crítica) | 0.433 | Score reduzido em **43.3%** |
| **0.433** (🔴 alta) | 0.5 (moderada) | 0.217 | Score reduzido em **21.7%** |
| **0.433** (🔴 alta) | 0.0 (independente) | 0.000 | Sem impacto |
| **0.050** (🟢 baixa) | 1.0 (crítica) | 0.050 | Score reduzido em **5%** |

### 7.3 Exemplo: Cross-Audit com Entropia Alta

**Cenário** (do exemplo F2.1 §5.4): Cross-audit de segurança com 5 agentes.

```
efficiency_score_base = 3.51  (cálculo original do F2.1)

Entropia atual: 0.433 (🔴 alta)
knowledge_dependency: 1.0 (auditoria depende criticamente de conhecimento)

risk_penalty = 0.433 × 1.0 = 0.433

efficiency_score_adjusted = 3.51 × (1 - 0.433)
                         = 3.51 × 0.567
                         = 1.99

Resultado: efficiency caiu de 3.51 (FULL EXECUTION) para 1.99 (STANDARD EXECUTION)
           → A auditoria ainda é recomendada, mas com profundidade normal (não máxima)
           → Recomendação: executar com 3 agentes em vez de 5
           → Ação: adiar profundidade máxima até entropia cair
```

### 7.4 Feedback Loop

```
┌─────────────────────────────┐     ┌─────────────────────────────┐
│  Cognitive Entropy Engine   │────▶│  Cognitive Economy Engine   │
│  (F1.6)                     │     │  (F2.1)                     │
│                             │     │                             │
│  Entropy = 0.433 🔴         │     │  risk_penalty = entropy × k │
│  Contradictions: 1          │     │  efficiency_adjusted =      │
│  Stale: 0                   │     │    base × (1 - risk_penalty)│
│  Gaps: 4                    │     │                             │
└─────────────────────────────┘     └─────────────────────────────┘
         ▲                                 │
         │                                 ▼
┌─────────────────────────────┐     ┌─────────────────────────────┐
│  Curadoria / Redução de     │◀────│  Decisão Impactada           │
│  Entropia                   │     │                             │
│                             │     │  Standard execution (antes   │
│  nova_entropy < 0.25        │     │  seria full). 3 agentes em   │
│  → risk_penalty menor       │     │  vez de 5.                  │
└─────────────────────────────┘     └─────────────────────────────┘
```

### 7.5 Regras de Override na Cognitive Economy

| Condição de Entropia | Override no F2.1 | Justificativa |
|---------------------|-------------------|---------------|
| `entropy > 0.25` | Decision Ladder reduz 1 nível | Conhecimento não confiável → ações mais conservadoras |
| `entropy > 0.25` | Ações com `knowledge_dependency > 0.7` exigem Don approval | Decisões baseadas em conhecimento desorganizado precisam de validação humana |
| `entropy > 0.25 por 4+ semanas` | Bloquear TODAS ações com `knowledge_dependency > 0.5` | Entropia crônica → congelar decisões até consolidação |
| `entropy < 0.10` | Knowledge_dependency bonus: +0.1 no efficiency | Conhecimento confiável reduz risco → permite mais autonomia |

---

## 8. INTEGRAÇÃO COM F1.5 COGNITIVE METRICS

A entropia cognitiva complementa o Cognitive Health Index (CHI) como uma **métrica de risco**:

### 8.1 Relação com B5 — Cognitive Debt

| Aspecto | B5 Cognitive Debt | F1.6 Cognitive Entropy |
|---------|------------------|----------------------|
| **O que mede** | Aprendizado não registrado (audit loop incompleto) | Desorganização do conhecimento registrado |
| **Fonte** | Cognitive Audit Loop pós-task | Wisdom Decay + Gap Detection |
| **Escala** | 0-100% (dívida) | 0.0-1.0 (entropia) |
| **Pergunta** | "Estamos registrando?" | "O que registramos está organizado?" |

### 8.2 CHI Corrigido por Entropia

```
CHI_corrigido = CHI × max(0, 1.0 - entropy)

Exemplo:
  CHI = 79.0 (da sessão atual, F1.5 §8.3)
  entropy = 0.433

  CHI_corrigido = 79.0 × max(0, 1.0 - 0.433)
                = 79.0 × 0.567
                = 44.8

  Interpretação: O CHI aparente (79.0) é 🟡 Bom, mas quando corrigido
  pela entropia (44.8), cai para 🔴 Crítico. A diferença (34.2 pontos)
  é o "risco oculto" do conhecimento desorganizado.
```

### 8.3 Componente no Dashboard de Métricas

O dashboard de métricas cognitivas (F1.5) deve incluir entropia como **métrica de alerta**:

```
┌─────────────────────────────────────────────────────────────────────┐
│  B1 LOAD  B2 VELOCITY  B3 FRESHNESS  B4 REUSE  B5 DEBT │ ENTROPIA │
│  ┌──────┐ ┌──────────┐ ┌───────────┐ ┌────────┐ ┌──────┐ │ ┌──────┐ │
│  │ 13.8 │ │  6.4s    │ │   0.86    │ │ 34.2%  │ │ 3.1% │ │ │0.433 │ │
│  │ 🟢   │ │  🟢      │ │  🟢       │ │  🟢    │ │  🟢  │ │ │ 🔴   │ │
│  └──────┘ └──────────┘ └───────────┘ └────────┘ └──────┘ │ └──────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 9. CLI E AUTOMAÇÃO

### 9.1 Comandos

```bash
# Calcular entropia agora
cosca entropy calc

# Ver entropia atual
cosca entropy status

# Ver timeline histórica
cosca entropy timeline --weeks 12

# Ver relatório de fontes de entropia
cosca entropy sources

# Ver tendência
cosca entropy trend

# Executar em modo dry-run (não registra no timeline)
cosca entropy calc --dry-run

# Output JSON para integração
cosca entropy calc --format json
```

### 9.2 Opções

| Flag | Descrição | Default |
|------|-----------|---------|
| `--dry-run` | Calcula mas não registra | `false` |
| `--format` | Formato de output: `table`, `json`, `yaml` | `table` |
| `--since` | Timeline desde data específica (ex: `--since 2026-06-30`) | `4 weeks ago` |
| `--threshold` | Threshold de alerta customizado | `0.25` |
| `--alert` | Forçar verificação de alerta mesmo se entropy não mudou | `false` |

### 9.3 Scheduler

O cálculo semanal é agendado via cron:

```bash
# Executar semanalmente (domingo à meia-noite)
0 0 * * 0 cosca entropy calc --format json >> /var/log/cognitive-entropy.log
```

### 9.4 CI Gate

```yaml
# .github/workflows/entropy-check.yml
name: Cognitive Entropy Check
on:
  schedule:
    - cron: "0 0 * * 0"  # Semanal
  workflow_dispatch:

jobs:
  entropy-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Calculate Entropy
        run: |
          cosca entropy calc --format json --output entropy-report.json

      - name: Check Threshold
        run: |
          ENTROPY=$(jq '.entropy' entropy-report.json)
          if [ "$(echo "$ENTROPY > 0.25" | bc)" -eq 1 ]; then
            echo "❌ Cognitive Entropy $ENTROPY exceeds threshold 0.25"
            echo "📋 Recommended actions:"
            jq -r '.sources[] | "  • \(.description)"' entropy-report.json
            exit 1
          fi
          echo "✅ Cognitive Entropy $ENTROPY is within threshold"
```

---

## 10. EXEMPLO COM DADOS REAIS

### 10.1 Base de Cálculo

Usando os **30 aprendizados do Kernel** (L1-L30, 2026-07-27 a 2026-07-30) como amostra representativa do conhecimento do sistema.

#### Total de Knowledge

```
total_knowledge = 30 (learnings do cosca-kernel)
```

#### Contradictions

Identificadas pelo Wisdom Decay (F1.4) como pares de afirmações mutuamente exclusivas:

| Par | Domínio | Aprendizado A | Aprendizado B | Status |
|-----|---------|---------------|---------------|--------|
| C-003 | Capacidade | `capability-profile.md` diz Level 3 | 7 entries L4 em learnings.md (L13, L15, L17, L18, L19, L20, L21) | ❌ **Ativa** |

**Contagem**: 1 contradição ativa.

> **Nota**: As outras 2 contradições do baseline (PostgreSQL fantasy e threshold crisis) foram resolvidas — PostgreSQL corrigido por cosca-database, thresholds unificados para 70% pelo Don.

#### Stale Knowledge

Calculado pelo Wisdom Decay (F1.4) usando a fórmula completa:

```
freshness = recency_term × 0.4 + usage_term × 0.3 + consistency_term × 0.2 + review_term × 0.1
```

**Amostra de freshness dos 30 learnings** (calculado em 2026-07-30):

| Learning | Data | Dias sem uso | Usage | Freshness | Status |
|----------|------|-------------|-------|-----------|--------|
| L1 | 2026-07-27 | 3 | 2 | 0.68 | 🟡 REVALIDATE_30* |
| L13 (Jail) | 2026-07-29 | 1 | 8 | 0.82 | 🟢 HEALTHY |
| L17 (Token Bloat) | 2026-07-29 | 1 | 5 | 0.77 | 🟢 HEALTHY |
| L22 (CMI Arch) | 2026-07-30 | 0 | 3 | 0.75 | 🟢 HEALTHY |
| L29 (Onda F0) | 2026-07-30 | 0 | 2 | 0.73 | 🟢 HEALTHY |
| L30 (F0 Completa) | 2026-07-30 | 0 | 1 | 0.72 | 🟢 HEALTHY |

> **\*L1**: Primeiro learning do Kernel, 3 dias sem uso. Freshness 0.68 (acima de 0.30). Ainda não stale.

**Nenhum learning com freshness < 0.30** — o sistema tem apenas ~6 dias de idade. Todos os aprendizados são recentes.

**Contagem**: 0 stale.

#### Gaps sem DDNA

Gaps abertos detectados por F1.3 que **não possuem DDNA vinculado**:

| Gap | Dimensão | Descrição | Severidade | Score | DDNA? |
|-----|----------|-----------|------------|-------|-------|
| G-001 | GAP_ARCH | **5 orphan refs**: MEMORY_MODEL.md referencia engines não implementados (memory, context, learning, evolution, cognitive-compression) | MEDIUM | 45 | ❌ |
| G-002 | GAP_MEMORY | **capability-profile.md desatualizado**: Kernel registrado como Level 3 mas opera em Level 4 | MEDIUM | 40 | ❌ |
| G-003 | GAP_MEMORY | **48 agentes seed-only sem learnings**: 41 agentes com confidence 0.25 seed, 7 sem qualquer entrada | LOW-MEDIUM | 35 | ❌ |
| G-004 | GAP_ARCH | **8 auditorias não consolidadas**: L18-L21 e 4 aprendizados relacionados sem princípio extraído | MEDIUM | 30 | ❌ |

**Contagem**: 4 gaps sem DDNA.

### 10.2 Cálculo

```
raw_entropy = (contradictions × 3) + (stale × 2) + (gaps × 1)
            = (1 × 3) + (0 × 2) + (4 × 1)
            = 3 + 0 + 4
            = 7

cognitive_entropy = raw_entropy / total_knowledge
                  = 7 / 30
                  = 0.433 (arredondado para 3 casas)

classificação = 0.433 > 0.25 → 🔴 ALTA
```

### 10.3 Breakdown

```
Cognitive Entropy: 0.433  🔴 ALTA
──────────────────────────────────────────────────
Componente              Contagem  Peso  Contribuição  % do total
──────────────────────────────────────────────────
Contradictions (ativa)       1   × 3         3/30    42.9%  ██████████████████
Stale Knowledge              0   × 2         0/30     0.0%
Gaps sem DDNA                4   × 1         4/30    57.1%  ██████████████████████
──────────────────────────────────────────────────
Total:                                  7/30 = 0.433
──────────────────────────────────────────────────

Interpretação:
  • A maior contribuição (57.1%) vem de gaps sem DDNA — problemas conhecidos
    mas sem decisão registrada. Solução: criar DDNAs para os 4 gaps.

  • Contradições ativas contribuem com 42.9%. A única contradição ativa
    (Level 3 vs Level 4) é relatível de resolver — atualizar capability-profile.md.

  • Stale knowledge não contribui (0%) porque o sistema é muito jovem.
    Em 3 meses, será o componente dominante se não houver revalidação.

  • A entropia de 0.433 reflete um sistema em construção: conhecimento
    recente e fresco (zero stale), mas com gaps de rastreabilidade (4 gaps
    sem DDNA) e uma contradição que precisa de resolução.
```

### 10.4 Projeção Pós-Correção

Se as correções recomendadas forem aplicadas:

```
Após criar DDNA para G-001 (orphan refs):
  gaps = 3
  raw = (1×3)+(0×2)+(3×1) = 6
  entropy = 6/30 = 0.200 → 🟡 MÉDIA

Após criar DDNA para G-002 (capability mismatch):
  gaps = 2
  raw = (1×3)+(0×2)+(2×1) = 5
  entropy = 5/30 = 0.167 → 🟡 MÉDIA

Após resolver C-003 (atualizar capability-profile.md para Level 4):
  contradictions = 0, gaps = 2
  raw = (0×3)+(0×2)+(2×1) = 2
  entropy = 2/30 = 0.067 → 🟢 BAIXA

Após criar DDNA para G-003 e G-004:
  gaps = 0
  raw = 0
  entropy = 0/30 = 0.000 → 🟢 BAIXA (perfeito)
```

### 10.5 Ações Imediatas

Com base no cálculo atual (0.433 🔴), as ações prioritárias são:

| Prioridade | Ação | Impacto na Entropia | Responsável |
|-----------|------|:-------------------:|-------------|
| **P0** | Criar DDNA para G-004 (consolidar 8 auditorias em 1 princípio) | 0.433 → 0.367 | Architecture Chief |
| **P0** | Atualizar capability-profile.md do Kernel para Level 4 (resolve C-003) | 0.367 → 0.267 | Kernel |
| **P1** | Criar DDNA para G-001 (orphan refs — engines não implementados) | 0.267 → 0.233 | Architecture Chief |
| **P1** | Criar DDNA para G-002 (capability mismatch) | 0.233 → 0.200 | Memory Chief |
| **P2** | Criar DDNA para G-003 (seed agents sem learnings) | 0.200 → 0.167 | Memory Chief |

---

## 11. PLANO DE REDUÇÃO DE ENTROPIA

### 11.1 Alvo

| Período | Entropia Alvo | Classificação |
|---------|:------------:|---------------|
| Baseline (2026-07-30) | 0.433 | 🔴 Alta |
| 1 semana | < 0.25 | 🟡 Média |
| 2 semanas | < 0.15 | 🟡 Média |
| 1 mês | < 0.10 | 🟢 Baixa |
| 3 meses | < 0.05 | 🟢 Baixa (excelente) |

### 11.2 Estratégia

```
FASE 1 — Resolver contradições (impacto imediato, maior peso)
  ├── C-003: capability-profile.md → Level 4
  ├── C-001: verificar regressão PostgreSQL (já resolvida, confirmar)
  └── C-002: verificar thresholds unificados (já resolvido, confirmar)

FASE 2 — Criar DDNAs para gaps (impacto médio, peso 1)
  ├── G-001: DDNA orphan refs
  ├── G-002: DDNA capability mismatch
  ├── G-003: DDNA seed agents
  └── G-004: DDNA consolidação de auditorias

FASE 3 — Prevenir stale knowledge (impacto contínuo)
  ├── Garantir que Wisdom Decay rode semanalmente
  ├── Revalidar learnings com freshness < 0.50
  └── Depreciar learnings com freshness < 0.15

FASE 4 — Automação (preventivo)
  ├── Alerta automático quando entropia sobe > 20% em 2 semanas
  ├── Criar DDNA automático para gaps P0
  └── Dashboard de entropia no relatório semanal
```

---

## 12. MÉTRICAS DO ENGINE

### 12.1 Métricas Internas

| Métrica | Definição | Alvo | Fonte |
|---------|-----------|:----:|-------|
| **Entropy Score** | Cognitive Entropy calculado (0.0-1.0) | < 0.10 | Pipeline |
| **Contradiction Count** | Pares de contradição ativos | 0 | F1.4 |
| **Stale Count** | Learnings com freshness < 0.30 | 0 | F1.4 |
| **Gap Count** | Gaps sem DDNA | 0 | F1.3 |
| **Pipeline Duration** | Tempo total de execução | < 2s | Engine |
| **Alert Rate** | Alertas emitidos / medições | < 10% | Engine |
| **Time to Green** | Tempo entre 🔴 e 🟢 após ações corretivas | < 2 semanas | Histórico |
| **Trend Accuracy** | Previsão de tendência confirmada na semana seguinte | > 80% | Histórico |

### 12.2 Alimentação do CMI

| Dimensão CMI | Impacto | Justificativa |
|-------------|---------|---------------|
| **Consistência** | Direto | Entropia é métrica inversa de consistência do conhecimento |
| **Julgamento** | Indireto | Entropia alta reduz confiança nas decisões baseadas em conhecimento |
| **Aprendizado** | Indireto | Entropia alta indica que aprendizado não está sendo consolidado |

---

## 13. RELACIONAMENTOS

| Documento | Relação |
|-----------|---------|
| [COGNITIVE_ENTROPY.md (analytics)](../../analytics/COGNITIVE_ENTROPY.md) | Dashboard de 5 componentes (0-100%). Este engine é o cálculo operacional (3 componentes, 0.0-1.0). |
| [entropy-baseline-2026-07-30.md](../../analytics/entropy-baseline-2026-07-30.md) | Baseline com 3 contradições, 3 fragmentações, 5 órfãos. Fonte dos dados do exemplo. |
| [WISDOM_DECAY.md](../wisdom-decay/WISDOM_DECAY.md) | F1.4: fornece `contradictions` e `stale` para entropia. |
| [gap-detection/SKILL.md](../gap-detection/SKILL.md) | F1.3: fornece `gaps` para entropia (gaps sem DDNA). |
| [cognitive-metrics.md](../../analytics/cognitive-metrics.md) | F1.5: CHI corrigido por entropia, B1-B5. |
| [cognitive-economy/SKILL.md](../cognitive-economy/SKILL.md) | F2.1: entropia como risk_penalty no efficiency_score. |
| [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) | C2: conceito original de Cognitive Entropy. F1.6: implementação do motor. |

---

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|--------|------|-------|------------|
| 1.0.0 | 2026-07-30 | Architecture Chief | Criação inicial do Cognitive Entropy Engine F1.6. Fórmula de 3 componentes (contradictions×3, stale×2, gaps×1) normalizada por total_knowledge. Thresholds 🟢<0.10, 🟡0.10-0.25, 🔴>0.25. Pipeline semanal. Integração F2.1 (risk_penalty). Exemplo com 30 learnings do Kernel: 0.433 🔴. |

---

> *"Entropia não é destino — é diagnóstico. Cada contradição resolvida, cada gap preenchido, cada conhecimento revalidado reduz a desordem e aproxima o sistema da clareza."*
> — Cosca Architecture Chief, 2026-07-30

---

*[EOF — Cognitive Entropy Engine F1.6]*
