# SECOND-ORDER REASONING ENGINE — Meta-Análise dos Padrões de Raciocínio (F3.2)

> **Versão**: 2.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30 | **Atualizado**: 2026-07-30
>
> **Workflow**: `cosca-second-order-reasoning` | **Fase CMI**: Fase 3 — Cognição Avançada | **Código**: F3.2
> **Dependências**: F1.1 (DDNA) | F1.2 (Contrafactual Gate) | F2.3 (Federation) | F2.6 (Pattern Evolution) | F7.2 (Trust Registry) | F7.3 (Engineering Score) | F8.1 (Capability Market) | F8.3 (Contradiction Engine)
>
> **Autoridade constitucional**: [CONSTITUTION.md](../../CONSTITUTION.md) — Implementa P5 (a família aprende com erros) e P4 (o Don tem veto absoluto). Estende [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) §4 A6 (Metacognição) no nível da plataforma inteira.
>
> **Lema**: *"Não pense apenas melhor — pense sobre como você pensa. O 1º raciocínio resolve problemas. O 2º raciocínio resolve os padrões que criam os problemas."*
>
> **Consulte também**:
> - [PROJECTION.md](./PROJECTION.md) — engine complementar de projeção forward de consequências (preservado da v1.0.0)
> - [Contrafactual Gate (F1.2)](../../workflows/contrafactual-gate.md) — gate de decisão; fonte de dados e alvo de calibração
> - [DECISION_DNA.md (F1.1)](../../knowledge/architecture/DECISION_DNA.md) — fonte primária de decisões
> - [Pattern Evolution (F2.6)](../pattern-evolution/SKILL.md) — ciclo de vida dos padrões de raciocínio
> - [Federation (F2.3)](../federation/SKILL.md) — publicação cross-project de meta-learnings
> - [Contradiction Engine (F8.3)](../contradiction/SKILL.md) — evidências contra como dado de viés de confirmação
> - [Capability Market (F8.1)](../capability-market/SKILL.md) — leilão; alvo de detecção de viés de reputação
> - [TRUST_REGISTRY.md (F7.2)](../../memory/trust/TRUST_REGISTRY.md) — reputação histórica de agentes
> - [AUTO_EVOLUTION_PROTOCOL.md](../../shared/AUTO_EVOLUTION_PROTOCOL.md) — Stages 7-8, checklist Q1-Q5

---

## SUMÁRIO

1. [Definição — Meta-Raciocínio](#1-definição--meta-raciocínio)
2. [Níveis de Raciocínio — 1ª, 2ª e 3ª Ordem](#2-níveis-de-raciocínio)
3. [Pipeline Mensal (< 5s)](#3-pipeline-mensal--5s)
4. [Métricas de Padrões de Decisão](#4-métricas-de-padrões-de-decisão)
5. [Detecção de Vieses Sistêmicos](#5-detecção-de-vieses-sistêmicos)
6. [Correções de Meta-Melhoria (Propostas, Nunca Regras)](#6-correções-de-meta-melhoria)
7. [Integração com F1.2 — Contrafactual Gate](#7-integração-com-f12--contrafactual-gate)
8. [Integração com F8.1 — Capability Market](#8-integração-com-f81--capability-market)
9. [Integração com F2.6, F2.3 e F8.3 — Padrões de Raciocínio como Conhecimento](#9-integração-com-f26-f23-e-f83)
10. [Formato do Relatório Mensal](#10-formato-do-relatório-mensal)
11. [Exemplo Real — Sessão 2026-07-30 (12 ondas)](#11-exemplo-real--sessão-2026-07-30-12-ondas)
12. [Regras Non-Negotiable](#12-regras-non-negotiable)
13. [Governança](#13-governança)
14. [CLI e Automação](#14-cli-e-automação)
15. [Coexistência com PROJECTION.md](#15-coexistência-com-projectionmd)
16. [Referências](#16-referências)

---

## 1. DEFINIÇÃO — META-RACIOCÍNIO

### 1.1 O que é o Second-Order Reasoning Engine

O **Second-Order Reasoning Engine** (F3.2) é o motor de **raciocínio de segunda ordem** do Cosca: pensamento sobre o pensamento. Ele não pergunta *"qual é a melhor solução para X?"* — ele pergunta *"como decidimos sobre X? Que padrões de raciocínio usamos? Esses padrões estão funcionando?"*.

Enquanto as engines de decisão (Gate, Contrafactual, Projection, Contradiction) operam **dentro** de cada decisão, o F3.2 opera **acima** de todas elas: analisa os META-PADRÕES — os padrões que o próprio Cosca exibe ao decidir — e detecta vieses sistêmicos que nenhuma engine individual consegue enxergar porque estão distribuídos entre decisões.

Uma decisão individual nunca revela um viés. **Um viés é uma propriedade estatística de um conjunto de decisões.** O F3.2 é a única engine que olha para o conjunto.

### 1.2 As Três Perguntas do Meta-Raciocínio

```
┌─────────────────────────────────────────────────────────────────────┐
│                    AS 3 PERGUNTAS DO F3.2                            │
│                                                                      │
│  1. "POR QUE pensamos assim?"                                        │
│     → Que premissas, heurísticas e vieses moldam nossas decisões?    │
│                                                                      │
│  2. "QUE VIÉSES influenciam nossas decisões?"                        │
│     → Ancoragem? Autoridade? Disponibilidade? Confirmação? Custo?   │
│     → Onde a fricção de qualidade está faltando?                    │
│                                                                      │
│  3. "NOSSOS PADRÕES DE RACIOCÍNIO estão evoluindo ou estagnados?"    │
│     → A cada mês decidimos melhor que no anterior? Ou repetimos?    │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.3 Analogia: O Observador da Fábrica

Uma fábrica tem operadores (agentes), ferramentas (engines) e produtos (decisões). O F3.2 não é mais um operador — é o **engenheiro de processos** que observa a linha de produção inteira:

- Não inspeciona um produto ("esta decisão está boa?") — isso é o Gate (F1.2).
- Não prevê o que o produto causa — isso é o PROJECTION.md.
- Ele pergunta: *"por que 80% dos produtos saem da mesma estação? Por que a inspeção só roda no turno da manhã? A linha está melhorando ou repetindo os mesmos defeitos?"*

O output do F3.2 nunca é uma decisão — é uma **mudança de processo**.

### 1.4 Estado Atual vs Estado Alvo

**Hoje**: O Cosca decide muito (12+ ondas em uma sessão), documenta pouco (1 DDNA formal), e questiona pouco (0 outputs de Gate no `gate-results/`). Não existe medição de vieses sistêmicos nem comparação mês-a-mês dos padrões de decisão. A qualidade do raciocínio é intuída, não medida.

**Amanhã**:
- Todo mês, o pipeline calcula as métricas de padrão de decisão em < 5s
- Vieses sistêmicos são detectados por fórmula (não por intuição)
- Cada viés vira uma **proposta ao Don** — nunca uma regra automática
- A evolução mês-a-mês dos padrões responde objetivamente: "estamos raciocinando melhor?"

### 1.5 Escopo

**Cobre**:
- Análise de meta-padrões: como a plataforma decide (não o que decide)
- Detecção estatística de 6 vieses sistêmicos (ancoragem, autoridade, disponibilidade, confirmação, custo, reputação)
- Medição da evolução dos padrões de raciocínio (evoluindo / estagnado / regredindo)
- Geração de propostas de meta-melhoria para o Don
- Calibração *proposta* dos gatilhos do Contrafactual Gate (F1.2)
- Detecção de viés de reputação no Capability Market (F8.1)

**NÃO cobre**:
- Decidir (a decisão é sempre do Kernel/Don — este engine recomenda processos)
- Projetar consequências forward de uma decisão (isso é o [PROJECTION.md](./PROJECTION.md))
- Gerar alternativas para uma decisão (isso é o Contrafactual Gate, F1.2)
- Minerar evidências contra uma decisão (isso é o Contradiction Engine, F8.3)
- Aplicar correções automaticamente (toda correção exige aprovação explícita do Don)

---

## 2. NÍVEIS DE RACIOCÍNIO

### 2.1 Os Três Níveis

```
┌─────────────────────────────────────────────────────────────────────┐
│                    NÍVEIS DE RACIOCÍNIO                              │
│                                                                      │
│  1ª ORDEM (normal)        "Qual é a melhor solução para X?"         │
│  ─────────────────────────────────────────────────────────           │
│  É o raciocínio comum de toda decisão: analisa o problema,          │
│  gera opções, escolhe. O Cosca já faz isso em cada task.            │
│                                                                      │
│  2ª ORDEM (meta)          "Como decidimos sobre X? Que padrões      │
│                           usamos? Esses padrões estão funcionando?" │
│  ─────────────────────────────────────────────────────────           │
│  É o raciocínio SOBRE o raciocínio: observa o processo de           │
│  decisão da plataforma como um todo, detecta vieses e               │
│  recomenda melhorias de processo. É o F3.2 — ESTE ENGINE.           │
│                                                                      │
│  3ª ORDEM (meta-meta)     "Como melhoramos a forma como             │
│                           decidimos sobre como decidimos?"          │
│  ─────────────────────────────────────────────────────────           │
│  É a evolução da própria meta-análise: as melhorias de processo     │
│  melhorando a si mesmas. É o escopo do F3.7 (Cognitive Ecosystem    │
│  Dynamics), que integra TODOS os engines F3 em um sistema vivo.     │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 Tabela Comparativa

| Dimensão | 1ª Ordem (operacional) | 2ª Ordem (meta — F3.2) | 3ª Ordem (meta-meta — F3.7) |
|----------|------------------------|------------------------|------------------------------|
| **Pergunta** | "Qual a melhor solução para X?" | "Como decidimos sobre X?" | "Como melhoramos como decidimos sobre como decidimos?" |
| **Objeto** | O problema | O processo de decisão | O processo de melhorar o processo |
| **Unidade** | 1 decisão | N decisões (conjunto) | N ciclos de meta-análise |
| **Frequência** | Contínuo (cada task) | Mensal | Trimestral/anual |
| **Output** | Decisão | Propostas de processo | Emendas ao meta-processo |
| **Exemplo** | "Use memfd_create" | "80% das decisões confirmam a 1ª opção — ativar 2ª opinião" | "A regra de 2ª opinião melhorou o acerto em 12% — elevá-la a princípio" |
| **Engine responsável** | Todas as engines de decisão | **F3.2 (este arquivo)** | F3.7 Ecosystem |

### 2.3 A Pergunta que Esta Engine Responde

> **"A plataforma está raciocinando melhor do que raciocinava no mês passado?"**
> **"Quais vieses sistêmicos estão distorcendo nossas decisões sem que ninguém perceba?"**
> **"Onde devemos adicionar fricção de qualidade ao nosso processo de decisão?"**

---

## 3. PIPELINE MENSAL (< 5S)

O pipeline roda **mensalmente** (1º dia do mês, 07:00 UTC) ou sob demanda via CLI. Tempo alvo: **< 5s** para 30+ decisões.

### 3.1 Pipeline

```
┌──────────────────────────────────────────────────────────────────────┐
│                PIPELINE MENSAL DO F3.2 (5 PASSOS)                    │
│                                                                      │
│  1. COLETA — todas as decisões do mês                                │
│     ├── DDNA (F1.1): decision_level, Options, Evidence, confidence  │
│     ├── Gate (F1.2): outputs em gate-results/, ativação por gatilho │
│     ├── Trust Registry (F7.2): outcome, latency, cost por task      │
│     ├── Engineering Score (F7.3): qualidade do commit associado     │
│     └── Market (F8.1): leilões, win streaks, bids                   │
│         │                                                            │
│         ▼                                                            │
│  2. ANALISA PADRÕES de decisão                                       │
│     ├── % decisões com análise de alternativas  (Gate usado?)       │
│     ├── % decisões com evidências               (DDNA completo?)    │
│     ├── Viés dominante (confirmatório? autoridade? disponibilidade?)│
│     ├── Tempo médio de decisão por tipo                             │
│     └── Correlação: decisões rápidas × qualidade (F7.3)             │
│         │                                                            │
│         ▼                                                            │
│  3. DETECTA vieses sistêmicos                                        │
│     ├── "80% das decisões confirmam a 1ª opção"  → ANCORAGEM        │
│     ├── "Gate só usado em P0, nunca em P1"        → CUSTO           │
│     └── "agente X sempre vence o leilão"          → REPUTAÇÃO       │
│         │                                                            │
│         ▼                                                            │
│  4. GERA recomendações de meta-melhoria                              │
│     ├── "Ativar Gate em P1 quando risco > 30%"                      │
│     ├── "Sortear 2ª opinião quando o mesmo agente vence 5x"         │
│     └── "Revisar DDNA template — campo alternatives vazio em 60%"   │
│         │                                                            │
│         ▼                                                            │
│  5. PROPOSTAS ao Don (nunca regras automáticas)                     │
│     └── Relatório mensal + propostas aguardando aprovação           │
└──────────────────────────────────────────────────────────────────────┘
```

### 3.2 Algoritmo

```
function run_meta_analysis():
    // Passo 1: Coleta (todas as fontes já indexadas — FTS5, YAML parse)
    decisions = collect_decisions(month)          # DDNA + Gate + Trust + Score + Market

    // Passo 2: Métricas de padrão
    metrics = {
        gate_usage_rate:     gate_usage_rate(decisions),          # §4.1
        evidence_rate:       evidence_rate(decisions),            # §4.2
        alternatives_rate:   alternatives_rate(decisions),        # §4.2
        latency_by_type:     latency_by_type(decisions),          # §4.3
        speed_quality_corr:  speed_quality_correlation(decisions) # §4.4
    }

    // Passo 3: Detecção de vieses (§5)
    biases = detect_biases(decisions, metrics)    # 6 detectores, cada um com threshold

    // Passo 4: Recomendações de meta-melhoria (§6)
    recommendations = generate_recommendations(metrics, biases)

    // Passo 5: Relatório + propostas ao Don
    report = build_report(month, metrics, biases, recommendations)
    append_history(report)                          # CSV append-only (imutável)
    notify_don(report)                              # propostas, nunca ações
    return report
```

### 3.3 Performance

| Operação | Complexidade | Tempo Estimado |
|----------|--------------|----------------|
| Coleta de DDNAs + Gate + Trust + Score + Market | O(D), índices FTS5/YAML | < 2.0s |
| Cálculo de métricas (aritmética) | O(D) | < 0.5s |
| Detecção de vieses (6 detectores) | O(D) | < 0.5s |
| Recomendações + relatório | O(B) | < 1.5s |
| **Total** | | **< 5s** ✅ |

> **D = decisões do mês, B = vieses detectados**. O segredo de performance: as fontes (DDNA, gate-results, Trust) já são arquivos estruturados com índices FTS5 — nenhuma leitura de learnings.md completa é necessária. A regra de mínimo de 30 decisões (§12) protege o engine de rodar com amostras insignificantes.

### 3.4 Trigger Mensal

```
Trigger: cron mensal (1º dia do mês, 07:00 UTC)
  ├── Após F7.4 (Impact Reports do último commit do mês)
  ├── Antes do F10.2 (Engineering Evolution Score, que consome saúde mensal)
  └── Ordem de execução: F7.4 → F3.2 → F10.2
```

---

## 4. MÉTRICAS DE PADRÕES DE DECISÃO

### 4.1 Análise de Alternativas (Gate Usado?)

```
gate_usage_rate = decisões com análise de alternativas (Gate executado)
                  / decisões elegíveis (P0/P1/multi-módulo)

Interpretação:
  ≥ 0.80  → fricção saudável: alternativas são consideradas por padrão
  0.40–0.80 → fricção parcial: algumas classes de decisão passam sem gate
  < 0.40  → fricção insuficiente: decisões raramente desafiadas
```

### 4.2 Evidências (DDNA Completo?)

```
evidence_rate     = decisões com DDNA completo (Options + Evidence + Decision preenchidos)
                    / decisões P0/P1

alternatives_rate = decisões com ≥ 2 Options no DDNA / decisões com DDNA

Interpretação:
  evidence_rate ≥ 0.80  → decisões rastreáveis
  evidence_rate < 0.50  → decisões "esquecidas" (verdadeira raiz do viés:
                          o que não é documentado não pode ser analisado)
  alternatives_rate < 0.40 → Options frequentemente vazias → revisar template
```

### 4.3 Tempo Médio de Decisão por Tipo

```
latency_avg(task_type) = média(latência das decisões do tipo no mês)
latency_p50 / latency_p90 = percentis (robustos a outliers)

Fonte: TRUST_REGISTRY (campo latency) + ENGINEERING_TIMELINE

Interpretação (comparada ao baseline histórico):
  Aumento > 50% em um tipo → engarrafamento ou fricção excessiva
  Queda > 50% em um tipo → possível precipitação (cruza com §4.4)
```

### 4.4 Correlação: Decisões Rápidas × Qualidade (F7.3)

```
speed_quality_correlation =
    r = Σ(xᵢ − x̄)(yᵢ − ȳ) / √(Σ(xᵢ − x̄)² · Σ(yᵢ − ȳ)²)

Onde:
  xᵢ = latência da decisão i (horas, do Trust Registry)
  yᵢ = F7.3 engineering score do commit associado à decisão i
  n ≥ 10 (amostra mínima para confiança estatística)

Interpretação:
  r < −0.40  → 🔴 PRECIPITAÇÃO: decisões rápidas geram commits de baixa
               qualidade (F7.3). Recomendação: adicionar fricção mínima
               a decisões deste tipo.
  |r| < 0.20 → 🟢 NEUTRO: velocidade não compromete qualidade.
  r > +0.30  → 🟡 EXCESSO DE ANÁLISE: decisões lentas não produzem mais
               qualidade. Recomendação: reduzir fricção (fast-track).
  sem dados  → ⚠️ Requer F7.3 + latency registrados na mesma decisão.
               Se ausente, o relatório marca "dado ausente" (nunca inventa).
```

### 4.5 Evolução do Raciocínio (Estagnado vs Evoluindo)

```
Δ_metric(m) = métrica(mês m) − métrica(mês m−1)   # para as 4 métricas §4.1-§4.4

ESTADO DO RACIOCÍNIO:
  EVOLUINDO  → ≥ 3 métricas com Δ favorável ≥ 0.05
  ESTAGNADO  → ≥ 3 métricas com |Δ| < 0.02 por 3 meses consecutivos
  REGREDINDO → ≥ 3 métricas com Δ desfavorável ≤ −0.05
  INSIGNIFICANTE → < 30 decisões no mês (nenhuma conclusão emitida)
```

> **Regra de ouro**: evolução é medida contra o histórico imutável (CSV append-only, mesma regra constitucional do F7.3/F10.2). Recalcular meses passados destruiria a capacidade de ver tendência.

---

## 5. DETECÇÃO DE VIÉSES SISTÊMICOS

### 5.1 Tabela de Vieses

| Viés | Detecção | Correção |
|------|----------|----------|
| **Ancoragem** | 1ª opção vence > 70% | Forçar 2ª opinião |
| **Autoridade** | mesmo agente vence sempre | Rotação de experts |
| **Disponibilidade** | decisões usam só learnings recentes | Ponderar por relevância |
| **Confirmação** | evidências contra ignoradas | Contradiction Engine |
| **Custo** | Gate ativo em P0, zero em P1 | Calibrar gatilho G4 (risco) |
| **Reputação** | mesmo agente vence o leilão sempre | Sortear 2ª opinião |

### 5.2 Detectores (Fórmulas e Thresholds)

#### B1 — Ancoragem: a primeira opção domina

```
first_option_win_rate = decisões onde a Opção A (1ª gerada) venceu
                        / decisões com ≥ 2 Options no DDNA

DETECTADO: first_option_win_rate > 0.70
GRAVIDADE: escalada por confidence média — ancoragem com alta confiança
           é o viés mais perigoso (parece convicção, é preguiça cognitiva)
```

**Assinatura sistêmica**: "80% das decisões confirmam a primeira opção gerada."

#### B2 — Autoridade: um agente decide sempre

```
agent_share(i)  = decisões decididas pelo agente i / total de decisões no mês
authority_streak = maior sequência de decisões vencidas pelo mesmo agente

DETECTADO: max(agent_share) > 0.60  OU  authority_streak ≥ 5
```

**Assinatura sistêmica**: "O mesmo agente decide tudo neste domínio."

#### B3 — Disponibilidade: só o que é recente é lembrado

```
recent_evidence_rate = evidências citadas com idade < 30 dias
                       / total de evidências citadas nas decisões do mês

DETECTADO: recent_evidence_rate > 0.80
           E existem evidências relevantes (mesmos tags) com idade > 90 dias
             que NÃO foram citadas
```

**Assinatura sistêmica**: "As decisões citam só learnings da última semana."

#### B4 — Confirmação: evidências contra são ignoradas

```
contradiction_considered_rate = decisões onde evidências contra foram
                                explicitamente avaliadas (Gate ou F8.3 executado)
                                / decisões com contradições registradas (F8.3)

DETECTADO: contradiction_considered_rate < 0.30
```

**Assinatura sistêmica**: "Há contradições registradas no domínio, mas nenhuma decisão as consultou."

#### B5 — Custo: o gate só é ativado onde é obrigatório

```
gate_usage_by_priority(p) = decisões de prioridade p com Gate / decisões de prioridade p

DETECTADO: gate_usage(P0) ≥ 0.80  E  gate_usage(P1) < 0.10
```

**Assinatura sistêmica**: "Gate só usado em P0, nunca em P1" — a fricção é evitada onde é opcional.

#### B6 — Reputação: o leilão vira monopólio

```
market_concentration = share de leilões vencidos pelo top-1 agente no mês (F8.1)
market_win_streak    = maior sequência de leilões vencidos pelo mesmo agente

DETECTADO: market_concentration > 0.60  OU  market_win_streak ≥ 5
```

**Assinatura sistêmica**: "agente X sempre vence o leilão" — viés de reputação no market.

### 5.3 Índice de Viés

```
bias_index = max(bias_score_i, i ∈ {B1..B6})
viés dominante = argmax(bias_score_i)

ALERTAS:
  ≥ 1 viés ativo     → 🟡 MÉDIO — incluir no relatório mensal
  ≥ 2 vieses ativos  → 🟠 ALTO — escalar para revisão com o Don
  viés + regredindo (§4.5) → 🔴 CRÍTICO — combinação de degeneração
```

---

## 6. CORREÇÕES DE META-MELHORIA

### 6.1 Princípio: Recomenda Processos, Nunca Decide

O F3.2 **não decide nada**. Todo viés detectado vira uma **proposta ao Don**. A cadeia é sempre:

```
VIÉS DETECTADO (fato estatístico)
    ↓
RECOMENDAÇÃO (mudança de processo sugerida)
    ↓
PROPOSTA AO DON (aprovada → vira DDNA → muda processo)
    ↓
REGIÃO PROIBIDA: regra automática sem aprovação
```

### 6.2 Catálogo de Recomendações (por viés)

| Viés | Recomendação típica | Forma de aplicação |
|------|--------------------|--------------------|
| **Ancoragem** | "Forçar 2ª opinião independente quando a 1ª opção tiver confiança > 0.90" | Proposta de política de decisão (Gate) |
| **Autoridade** | "Rotacionar o decisor primário em domínios com agent_share > 0.60" | Proposta de roteamento (Kernel/Market) |
| **Disponibilidade** | "Ponderar evidências por relevância semântica, não por idade" | Proposta de melhoria no Confidence Model |
| **Confirmação** | "Ativar Contradiction Engine (F8.3) para domínios com contradições não consultadas" | Proposta de ativação de engine |
| **Custo** | "Ativar Gate em P1 quando risco > 30%" | Proposta de calibração do Gate (G4) |
| **Reputação** | "Sortear 2ª opinião quando o mesmo agente vence 5x seguidas" | Proposta de política do Market (F8.1) |

### 6.3 Exemplos do Don (recomendações canônicas)

O Don especificou três recomendações canônicas que o engine deve ser capaz de gerar:

```
├── "Ativar Gate em P1 quando risco > 30%"
│     → Dispara quando B5 (custo) é detectado
│
├── "Sortear 2ª opinião quando o mesmo agente vence 5x seguidas"
│     → Dispara quando B2 (autoridade) ou B6 (reputação) é detectado
│
└── "Revisar DDNA template — campo alternatives vazio em 60%"
      → Dispara quando alternatives_rate < 0.40 (métrica §4.2)
```

### 6.4 Formato da Proposta

```yaml
proposal:
  id: "META-2026-08-001"
  date: 2026-08-01
  bias: "B5-cost"                  # viés que motivou
  evidence: {                       # o fato estatístico (não a opinião)
    gate_usage_P0: 1.0,
    gate_usage_P1: 0.02,
    decisions_analyzed: 42
  }
  recommendation: "Ativar Gate em P1 quando risco > 30%"
  target: "contrafactual-gate G4 threshold"   # o que muda, se aprovado
  status: pending_don_approval      # pending | approved | rejected
  approved_by: null
  ddna_id: null                     # preenchido quando o Don aprova
```

---

## 7. INTEGRAÇÃO COM F1.2 — CONTRAFACTUAL GATE

A integração com o Gate é **bidirecional e assimétrica**: o F3.2 consome dados do Gate e devolve calibração — mas **nunca** mexe nos gatilhos por conta própria.

### 7.1 Fluxo de Dados

```
F1.2 GATE (toda decisão P0/P1)            F3.2 META-ANÁLISE (mensal)
┌────────────────────────────┐           ┌────────────────────────────┐
│  gate_usage por gatilho    │──────────▶│  B5 (viés de custo):       │
│  (G1-G6)                   │  dados    │  "P0 sempre, P1 nunca"     │
│  first_option_win_rate     │──────────▶│  B1 (ancoragem):           │
│  (alternativa vencedora)   │  dados    │  "A vence sempre"          │
│  confidence do Gate        │──────────▶│  calibração de thresholds │
└────────────────────────────┘           └────────────────────────────┘
        ▲                                          │
        │        proposta (via Don, DDNA)          │
        └──────────────────────────────────────────┘
        "Ativar G4 em P1 quando risco > 30%"
```

### 7.2 Calibração dos Gatilhos (Sempre Proposta)

| Observação do F3.2 | Proposta de calibração | Destino |
|--------------------|------------------------|---------|
| `gate_usage(P1) < 0.10` com P1s de alto risco | Reduzir `min_confidence_for_p1` (G4) de 0.70 para 0.70 − Δ | `cosca.config.yaml` → `contrafactual_gate` |
| `first_option_win_rate > 0.70` | Exigir alternativa ¬A real quando a 1ª opção tiver confidence > 0.90 | Gate pipeline, Passo 2 |
| `gate_usage(P2) alto` (over-friction) | Elevar `cost_threshold_usd` (G3) | `cosca.config.yaml` |
| Contradições não consultadas (B4) | Ativar G5 (DDNA pendente) automaticamente para o domínio | Gate pipeline |

> **Regra absoluta**: Nenhuma linha da tabela acima é executada pelo F3.2. Cada uma vira proposta (formato §6.4). O Don aprova → vira DDNA → o ajuste é aplicado ao Gate. O F3.2 apenas **recomenda calibração**; é o contraponto sistêmico ao viés de custo que o próprio Gate herdaria.

---

## 8. INTEGRAÇÃO COM F8.1 — CAPABILITY MARKET

O Capability Market (F8.1) roda leilões para rotear tasks. O F3.2 observa o **comportamento agregado do leilão** — algo que o Market individualmente não percebe.

### 8.1 Detecção de Viés de Reputação no Leilão

```
F8.1 MARKET (cada task)                F3.2 META-ANÁLISE (mensal)
┌────────────────────────────┐        ┌────────────────────────────┐
│  bids por agente           │───────▶│  market_concentration      │
│  vencedores por leilão     │───────▶│  (share do top-1)          │
│  win streaks               │───────▶│  market_win_streak         │
│  shadow bids (perdedores)  │───────▶│  diversidade de vencedores │
└────────────────────────────┘        └────────────────────────────┘
       ▲                                          │
       │   proposta (via Don)                     │
       └──────────────────────────────────────────┘
       "Sortear 2ª opinião quando o mesmo agente
        vence 5x seguidas" (shadow bid obrigatório)
```

### 8.2 Por que isso importa

O leilão de primeiro-score (F8.1) é **endógeno ao viés**: um agente que vence sempre acumula mais sucessos no Trust Registry, o que aumenta sua confidence predita (F7.1), o que o faz vencer mais — um **ciclo de reforço de reputação**. O F3.2 é o único ponto do sistema que mede a concentração do mercado e alerta:

- `market_concentration > 0.60` → o mercado virou monopólio cognitivo
- `market_win_streak ≥ 5` → mesmo resultado, mesmo ator, sem contraponto
- Perdedores sistemáticos com shadow bids bons → talento subutilizado (econômico, além de cognitivo)

### 8.3 Correção proposta (sempre via Don)

```
PROPOSTA: "Rotação de experts" ou "Sortear 2ª opinião"
IMPLEMENTAÇÃO POSSÍVEL (se o Don aprovar):
  - Shadow bid do 2º colocado vira execução paralela em modo audit
  - 1 em cada 5 leilões com streak ≥ 5 roda em double-blind
  - F7.1 predição recebe fator anti-concentração
```

---

## 9. INTEGRAÇÃO COM F2.6, F2.3 E F8.3

### 9.1 F2.6 — Padrões de Raciocínio Têm Ciclo de Vida

Os padrões de raciocínio descobertos pelo F3.2 seguem o ciclo de vida do Pattern Evolution (F2.6): nascem como observação meta-analítica, viram `#candidate` quando se repetem em 3 meses, `#validated` quando a correção melhora a métrica, e podem virar **princípio** (via F9.1) quando transcendem o processo:

```
F3.2 observa (mês 1):   "sempre rodar validação pós-onda"  → repetido
F2.6 registra (mês 3):  padrão #candidate
F3.2 mede (mês 6):      métrica melhorou após a correção  → #validated
F9.1 promove:           princípio de operação → Don aprova → emenda
```

> **Fronteira**: o F3.2 detecta a repetição; o F2.6 gerencia o ciclo de vida. O F3.2 nunca promove padrão — alimenta o F2.6 com a evidência meta-analítica.

### 9.2 F2.3 — Meta-Leanings São Federação-Valiosos

Aprendizados sobre **como a plataforma raciocina** (não sobre o domínio do projeto) são candidatos naturais a publicação cross-project (F2.3): outro projeto Cosca herda a lição "decisões rápidas com F7.3 baixo correlacionam" sem precisar repetir o mês de dados. Regra do F2.3 aplicada: só o que o F9.1 compilou e o Don marcou `#global-ready` sai do projeto — o F3.2 apenas sinaliza o candidato no relatório.

### 9.3 F8.3 — Contradição Como Dado de Viés de Confirmação

O Contradiction Engine (F8.3) minera evidências contra decisões. O F3.2 consome o **output agregado** do F8.3 para medir o viés de confirmação (B4):

- `contradictions_registered(domain)` — contradições que existem
- `contradictions_consulted(domain)` — contradições que as decisões avaliaram
- `contradiction_considered_rate = consulted / registered`

Se o F8.3 registra contradições e as decisões do mês não as consultaram → o F3.2 detecta confirmação **sistêmica** (não é um caso isolado — é um padrão do processo).

---

## 10. FORMATO DO RELATÓRIO MENSAL

O relatório é salvo em `internal/embed/cosca/analytics/reports/meta-reasoning-{YYYY-MM}.md` e registrado no CSV histórico `meta-reasoning-history.csv` (append-only).

```markdown
# META-REASONING REPORT — {Mês}/{Ano}

## Resumo (1 parágrafo)
{Como a plataforma decidiu neste mês, viés dominante, estado de evolução}

## Métricas de Padrão
| Métrica | Valor | Baseline | Δ | Status |
|---------|-------|----------|-----|--------|
| gate_usage_rate | 0.35 | 0.30 | +0.05 | 🟡 |
| evidence_rate | 0.72 | 0.55 | +0.17 | 🟢 |
| alternatives_rate | 0.48 | 0.60 | −0.12 | 🟠 |
| latency_avg (architecture) | 12m | 10m | +2m | 🟡 |
| speed_quality_corr | −0.31 | −0.10 | −0.21 | 🟠 |

## Vieses Detectados
| Viés | Score | Evidência | Recomendação |
|------|-------|-----------|--------------|
| B5 Custo | ATIVO | P0: 100%, P1: 2% | Ativar Gate em P1 quando risco > 30% |
| B1 Ancoragem | ATIVO | 1ª opção vence 82% | Forçar 2ª opinião em confidence > 0.90 |

## Estado do Raciocínio
**EVOLUINDO** — 3 métricas com Δ favorável (evidências ↑, gate ↑, tempo ↓)

## Propostas ao Don (aguardando aprovação)
- [ ] META-{mês}-001: Ativar G4 em P1 quando risco > 30% [B5]
- [ ] META-{mês}-002: Sortear 2ª opinião após 5 wins consecutivos [B2/B6]

## Candidatos a Padrão (F2.6) e Federação (F2.3)
- #candidate → F2.6: "validação pós-onda obrigatória" (repetiu 3 meses)
- global-ready candidato → F2.3: "lição de correlação velocidade×qualidade"

## Dados Ausentes (transparência)
- speed_quality_corr não computável: 6/42 decisões sem latency registrado
```

---

## 11. EXEMPLO REAL — SESSÃO 2026-07-30 (12 ONDAS)

> Este exemplo usa dados reais da Evolution Marathon de 2026-07-30 — a sessão em que este engine foi projetado. A meta-análise abaixo é exatamente o que o F3.2 faria ao final do mês.

### 11.1 Coleta (dados reais da sessão)

| Fonte | Dado real |
|-------|-----------|
| **Ondas** | 12 ondas de agentes paralelos (F0 → F1 → F2 → F3 → F7 → F8 → F9 → F10) |
| **Origem das decisões** | 100% iniciadas por "continue" do Don ao Kernel |
| **Confiança do Kernel** | 0.97 (L31), 0.98 (L32), 0.99 (L33) — crescente |
| **DDNA formal** | 1 (DDNA-2026-07-30-003 — Decision Replay) |
| **Gate outputs** | 0 (diretório `gate-results/` vazio) |
| **Trust Registry** | 46+ tasks registradas com outcome/latency/cost |
| **Learnings** | 33 entries do Kernel (L1-L33), todos da sessão |

### 11.2 Métricas de Padrão

```
decisões analisadas: 12 ondas (≥ 30? NÃO — mas exemplo didático da sessão)

gate_usage_rate      = 0/12 = 0%     → 🔴 nenhuma decisão foi contrafactual
evidence_rate        = 1/12 ≈ 8%    → 🔴 1 DDNA formal em 12 ondas P0/P1
alternatives_rate    = 1/1  = 100%  (único DDNA tem Options)
latency_by_type      = n/d (latência registrada por task no Trust, não por onda)
speed_quality_corr   = n/d (F7.3 presente por commit; latência por decisão não)
estado               = EVOLUINDO (confiança crescente 0.97→0.99, L33 adicionou fricção)
```

### 11.3 Detecção de Vieses

```
B2 AUTORIDADE  → DETECTADO (agent_share Kernel = 1.0; streak = 12)
                 "100% das ondas decididas pelo mesmo agente com confiança > 0.95"

B5 CUSTO       → DETECTADO (gate_usage P0 = 0/12 = 0% apesar de P0s existirem)
                 "Nenhuma decisão P0 passou pelo Gate nesta sessão"

B1 ANCORAGEM   → SUSPEITO (sem DDNAs suficientes para estatística;
                 mas com 0 Gates, a 1ª opção nunca foi desafiada)

B3/B4/B6       → INSUFICIENTE (amostra: dados ausentes — transparente no relatório)

viés dominante = AUTORIDADE
padrão identificado = "alta confiança Kernel, baixa fricção"
```

### 11.4 Recomendações de Meta-Melhoria

```
R1. "Adicionar checkpoint de validação pós-onda"
    → Contra o viés de autoridade: mesmo com alta confiança, toda onda
      precisa de verificação independente.
    → STATUS: JÁ IMPLEMENTADO como L33 — "Limite de Passos ≠ Task Concluída".
      O Don criou o padrão de validação pós-onda ANTES deste engine existir.
      A meta-análise teria recomendado exatamente o que o Don já fez —
      validação cruzada: o Don é o meta-raciocinador humano.

R2. "Formalizar DDNA por onda P0/P1"
    → evidence_rate de 8% é o maior gap: sem DDNA não há dado para
      meta-análise. Proposta: template DDNA obrigatório ao fechar onda.

R3. "Ativar Gate G4 (P1 com risco) nas próximas ondas"
    → Contra o viés de custo: gate_usage 0% em decisões P0/P1.

R4. "Registrar latência por decisão"
    → speed_quality_corr ficou "dado ausente". Sem a instrumentação
      (F1.5/F7.4), a meta-análise não pode correlacionar velocidade × qualidade.
```

### 11.5 Lição do Exemplo

O exemplo valida o design: com 12 decisões reais, o F3.2 detectou o viés de autoridade (estatisticamente inegável: agent_share = 1.0) e o viés de custo (gate_usage = 0%), e suas recomendações coincidiram com o que o Don já implementou (L33) — prova de que o engine formaliza o meta-raciocínio que o Don exercita intuitivamente. Também revelou a dependência de instrumentação: **meta-análise sem dados é especulação; o F3.2 registra "dado ausente" em vez de inventar**.

---

## 12. REGRAS NON-NEGOTIABLE

1. **Nunca decide — recomenda processos.** O F3.2 não executa correção alguma. Viés detectado = proposta ao Don.
2. **Vieses viram propostas, nunca regras automáticas.** Nenhuma correção pode ser autoaplicada. O Don aprova via DDNA.
3. **Meta-análise mensal, mínimo 30 decisões.** Abaixo de 30, o relatório emite "amostra insuficiente" e não conclui vieses.
4. **Nunca inventa dado.** Métricas sem fonte registram `dado ausente` (ex: speed_quality_corr sem latency). Insigth sem evidência é alucinação.
5. **Relatório imutável.** CSV append-only; mudança de regras vale apenas para meses futuros (mesma regra constitucional do F7.3/F10.2).
6. **Fronteiras com engines vizinhos preservadas.** Projeção forward → PROJECTION.md. Alternativas por decisão → Gate (F1.2). Evidências contra → Contradiction (F8.3). O F3.2 só agrega.
7. **Transparência de vieses, inclusive próprios.** Se a meta-análise deste engine estiver estagnada (mesma recomendação 3 meses seguidos sem efeito), o relatório reporta o próprio viés do engine.

---

## 13. GOVERNANÇA

### 13.1 Propriedade

| Papel | Quem | Responsabilidade |
|-------|------|------------------|
| **Owner do Engine** | Architecture Chief | Design, fórmulas, pipeline, relatório |
| **Executor do Pipeline** | Analytics Chief | Coleta mensal, cálculo, geração do relatório |
| **Validador de Vieses** | Critic Chief | Revisão adversarial dos vieses detectados |
| **Fonte de Dados** | Kernel + Gate + Market + Trust | Registro fiel das decisões (F1.1, F1.2, F7.2, F8.1) |
| **Aprovador de Propostas** | **Don** | Toda correção de processo (via DDNA) |
| **Padrões Derivados** | Evolution Chief (F2.6) | Ciclo de vida dos padrões de raciocínio |

### 13.2 Direitos e Responsabilidades

| Ação | Quem Pode | Condições |
|------|-----------|-----------|
| Rodar meta-análise | Analytics Chief | Mensal (cron) ou `--on-demand` |
| Emitir relatório | Analytics Chief | ≥ 30 decisões; senão "amostra insuficiente" |
| Gerar proposta | Engine (automático) | Todo viés detectado com evidência |
| Aprovar proposta | **Don** | Veto absoluto (P4) |
| Aplicar correção aprovada | Don + Engine alvo (Gate/Market) | DDNA de aprovação criado |
| Alterar thresholds do F3.2 | Architecture Chief | `cosca.config.yaml`, nunca em runtime |

### 13.3 Atualização e Calibração

- Thresholds de vieses (B1-B6) ajustáveis em `cosca.config.yaml` sem mudança de código
- A eficácia de uma proposta aprovada é medida no mês seguinte (a métrica melhorou?)
- Proposta aprovada sem efeito em 2 meses → relatório sinaliza e propõe alternativa

---

## 14. CLI E AUTOMAÇÃO

```bash
# Rodar meta-análise do mês
cosca second-order-reasoning run

# Rodar sob demanda (fora do ciclo mensal)
cosca second-order-reasoning run --month 2026-07

# Ver relatório do mês
cosca second-order-reasoning report --month 2026-07

# Listar vieses detectados
cosca second-order-reasoning biases --month 2026-07

# Listar propostas pendentes de aprovação
cosca second-order-reasoning proposals --status pending

# Registrar aprovação do Don (gera DDNA)
cosca second-order-reasoning approve META-2026-08-001 --by don

# Ver histórico de evolução dos padrões (CSV imutável)
cosca second-order-reasoning history
```

---

## 15. COEXISTÊNCIA COM PROJECTION.MD

O diretório `engines/second-order-reasoning/` abriga **duas engines complementares** (mesmo padrão de `gap-detection/COGNITIVE.md + SKILL.md` e `discovery/WORKSPACE.md + SKILL.md`):

| Dimensão | **SKILL.md (F3.2 — este)** | **PROJECTION.md (preservado)** |
|----------|----------------------------|-------------------------------|
| **Pergunta** | "Como decidimos?" (meta) | "O que acontece se agirmos?" (forward) |
| **Unidade de análise** | Conjunto de decisões (mensal) | 1 decisão (no momento da decisão) |
| **Temporalidade** | Retrospectiva (mês fechado) | Prospectiva (antes de agir) |
| **Output** | Propostas de processo ao Don | Árvore de consequências + intervenções |
| **Integração** | F1.2, F8.1, F2.6, F2.3 | F1.2, F2.1, F1.3, Planning |
| **Trigger** | Cron mensal | Pipeline de decisão (após Gate) |

**Regra de ortogonalidade**: fontes disjuntas (decisões do mês vs decisão corrente), triggers disjuntos (mensal vs decisão), outputs disjuntos (proposta de processo vs árvore de consequências). Nenhuma chama a outra — mas ambas consomem o Contrafactual Gate (F1.2) sem conflito: o Gate alimenta o PROJECTION.md na decisão e alimenta o F3.2 no fechamento do mês.

---

## 16. REFERÊNCIAS

| Documento | Relação | Localização |
|-----------|---------|-------------|
| **DECISION_DNA.md (F1.1)** | Fonte primária de decisões (Options, Evidence, confidence) | `knowledge/architecture/DECISION_DNA.md` |
| **Contrafactual Gate (F1.2)** | Fonte de dados + alvo de calibração de gatilhos | `workflows/contrafactual-gate.md` |
| **Pattern Evolution (F2.6)** | Ciclo de vida dos padrões de raciocínio | `engines/pattern-evolution/SKILL.md` |
| **Federation (F2.3)** | Publicação cross-project de meta-learnings | `engines/federation/SKILL.md` |
| **Contradiction Engine (F8.3)** | Viés de confirmação (B4) como dado agregado | `engines/contradiction/SKILL.md` |
| **Capability Market (F8.1)** | Viés de reputação (B6) no leilão | `engines/capability-market/SKILL.md` |
| **TRUST_REGISTRY.md (F7.2)** | Outcome/latency/cost por decisão | `memory/trust/TRUST_REGISTRY.md` |
| **Engineering Score (F7.3)** | Correlação velocidade × qualidade | `analytics/engineering-score.md` |
| **FORWARD PROJECTION (PROJECTION.md)** | Engine complementar preservado da v1.0.0 | `engines/second-order-reasoning/PROJECTION.md` |
| **COGNITIVE_MATURITY.md** | §4 A6 (Metacognição) — base conceitual | `architecture/COGNITIVE_MATURITY.md` |
| **COGNITIVE_ECOSYSTEM.md** | F3.7 — a 3ª ordem (meta-meta) | `architecture/COGNITIVE_ECOSYSTEM.md` |
| **AUTO_EVOLUTION_PROTOCOL.md** | Stages 7-8, checklist Q1-Q5 | `shared/AUTO_EVOLUTION_PROTOCOL.md` |
| **CONSTITUTION.md** | P4 (veto do Don), P5 (aprender com erros) | `CONSTITUTION.md` |

---

## CHANGELOG

> - **v2.0.0** (2026-07-30): Redefinição do Second-Order Reasoning Engine pelo Architecture Chief a pedido do Don. A v2.0.0 transforma o conceito de "2nd-order" de **projeção forward de consequências** (modelo v1.0.0, preservado em `PROJECTION.md`) para **meta-raciocínio** (F3.2): análise mensal dos padrões de decisão da plataforma. Novos: 3 níveis de raciocínio (1ª/2ª/3ª ordem, 3ª = F3.7 Ecosystem), pipeline mensal de 5 passos em < 5s, 6 detectores de viés sistêmico com fórmulas e thresholds (ancoragem, autoridade, disponibilidade, confirmação, custo, reputação), métricas de padrão de decisão (gate_usage_rate, evidence_rate, speed_quality_correlation via F7.3, estado evoluindo/estagnado/regredindo), integração com F1.2 (calibração de gatilhos sempre via proposta), F8.1 (viés de reputação no leilão), F2.6/F2.3/F8.3 (padrões de raciocínio como conhecimento). Regras: não decide nada, vieses viram propostas ao Don, mínimo 30 decisões, relatório imutável. Exemplo real validado com a sessão de 12 ondas (autoridade detectada: agent_share 1.0; recomendação R1 coincidiu com o L33 que o Don já havia criado).
> - **v1.0.0** (2026-07-30): Especificação original — motor de projeção forward de consequências (árvore de consequências, risco cumulativo, intervenções). **Preservada integralmente em [PROJECTION.md](./PROJECTION.md).**

---

> **"O 1º raciocínio resolve o problema. O 2º raciocínio resolve o raciocínio. O Cosca só para de repetir os próprios erros quando olha para si mesmo."**
