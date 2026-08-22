# COGNITIVE ECONOMICS ENGINE ★ F10.1 — Economia do Conhecimento da Plataforma

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-cognitive-economics`
> **Conceito**: ★ ESTRELA DA FASE 10 — Cognitive Economics — **O ÁPICE da arquitetura cognitiva**
> **Referências**: next-evolution-phases.md §F10.1 | COGNITIVE_MATURITY.md §5 | cognitive-maturity-implementation.md F10.1
> **Dependências**: F2.1 (Cognitive Economy) | F1.5 (B1-B5 Metrics) | F7.1 (Prediction Engine) | F7.2 (Trust Registry) | F7.3 (Engineering Score) | F9.1 (Experience Compiler) | F8.1 (Capability Market)
> **CMI Impact**: Julgamento +10, Planejamento +8, Aprendizado +6

---

## Índice

1. [Definição — O que é Cognitive Economics](#1-definição--o-que-é-cognitive-economics)
2. [Equação Mestra — Platform ROI](#2-equação-mestra--platform-roi)
3. [Dashboard Econômico Executivo](#3-dashboard-econômico-executivo)
4. [Pipeline de Coleta e Cálculo](#4-pipeline-de-coleta-e-cálculo)
5. [Tabela de Preços — Valor de Cada Ativo Cognitivo](#5-tabela-de-preços--valor-de-cada-ativo-cognitivo)
6. [Integração com F2.1 — Cognitive Economy](#6-integração-com-f21--cognitive-economy)
7. [Integração com F1.5 — Métricas B1-B5](#7-integração-com-f15--métricas-b1-b5)
8. [Integração com F7.1 — Prediction Engine](#8-integração-com-f71--prediction-engine)
9. [Integração com F7.2 — Trust Registry](#9-integração-com-f72--trust-registry)
10. [Integração com F7.3 — Engineering Score](#10-integração-com-f73--engineering-score)
11. [Integração com F9.1 — Experience Compiler](#11-integração-com-f91--experience-compiler)
12. [Integração com F8.1 — Capability Market](#12-integração-com-f81--capability-market)
13. [Exemplo com Dados Reais da Sessão](#13-exemplo-com-dados-reais-da-sessão)
14. [Alertas Configuráveis](#14-alertas-configuráveis)
15. [Arquitetura do Motor](#15-arquitetura-do-motor)
16. [Métricas do Próprio Engine](#16-métricas-do-próprio-engine)
17. [Implementação e Automação](#17-implementação-e-automação)
18. [Referências Cruzadas](#18-referências-cruzadas)
19. [Histórico](#19-histórico)

---

## 1. Definição — O que é Cognitive Economics

### 1.1 A ECONOMIA do Conhecimento

O **Cognitive Economics Engine** é o **ápice da arquitetura cognitiva do Cosca**. Ele não calcula o ROI de uma task isolada (isso é responsabilidade do F2.1 — Cognitive Economy Engine). Ele calcula o **ROI da PLATAFORMA INTEIRA**.

A pergunta que este engine responde:

> **"Quanto valor o Cosca gerou hoje?"**
> **"O ecossistema como um todo está mais eficiente do que ontem?"**
> **"Cada $1 gasto na plataforma está gerando quanto em retorno cognitivo?"**

Enquanto o F2.1 responde "esta task valeu a pena?", o F10.1 responde "o **sistema inteiro** valeu a pena hoje?".

### 1.2 Filosofia

```
"Se F2.1 é o extrato bancário de cada transação,
 F10.1 é o balanço patrimonial da empresa inteira.
 
 Não pergunte 'este investimento foi bom?'
 Pergunte 'nossa carteira de investimentos está saudável?'
 
 ★ F10.1 — Cognitive Economics é o CFO da plataforma.
   Ela responde se o Cosca, como um todo,
   está gerando mais valor do que consumindo."
   — Cosca Architecture Chief, 2026-07-30
```

### 1.3 Diferença Fundamental: F2.1 vs F10.1

| Aspecto | F2.1 — Cognitive Economy | F10.1 — Cognitive Economics |
|---------|-------------------------|----------------------------|
| **Escopo** | Uma task / uma ação | A plataforma inteira |
| **Pergunta** | "Valeu a pena executar?" | "Valeu a pena existir?" |
| **Período** | Por execução | Diário / semanal / mensal |
| **Inputs** | 5 dimensões custo × 5 valor | Agregação de TODAS as engines |
| **Output** | Efficiency score, decisão | Platform ROI, dashboard executivo |
| **Quem vê** | Kernel (decisão de execução) | Don (decisão de investimento) |
| **Alerta** | Efficiency < threshold | ROI caindo por período |
| **Complexidade** | < 1s (tempo real) | < 5s (batch diário) |

### 1.4 Princípios Imutáveis

1. **O ROI da plataforma é a métrica final.** Tudo o mais é sub-métrica.
2. **Transparência total.** Don vê de onde vem cada centavo de valor e custo.
3. **Cálculo < 5s.** Agrega dados já existentes — não cria nova carga computacional.
4. **Dashboard executivo.** Don entende em 30s se a plataforma está saudável.
5. **Alertas acionáveis.** Se ROI cair X%, o engine diz por quê e sugere correções.

---

## 2. Equação Mestra — Platform ROI

### 2.1 Fórmula Fundamental

```
                          Σ(valor_gerado)
Platform ROI = ────────────────────────────── - 1
                          Σ(custo_total)
```

Onde:

**Σ(valor_gerado)** = Receita Cognitiva Total (em $)
```
  Σ(valor_gerado) =
    aprendizados_gerados    × $1.00  (de F9.1, F2.1)
  + bugs_prevenidos         × $5.00  (de F1.5 B5, F2.1)
  + reuso_de_padroes        × $0.50  (de F1.5 B4, F2.1, F9.1)
  + decisoes_melhores       × $5.00  (de F1.5 B2/B3, F7.3, DDNA)
  + tempo_economizado       × $10.00 (de F1.5 B1, F8.1)
  + conhecimento_destilado  × $3.00  (de F9.1 Experience Compiler)
  + capacidade_alocada      × $2.00  (de F8.1 Capability Market)
```

**Σ(custo_total)** = Custos Totais da Plataforma (em $)
```
  Σ(custo_total) =
    tokens_gastos     × token_price   (de F2.1, F7.1, logs de runtime)
  + tempo_gasto       × $10.00/hora   (de F2.1, métricas de execução)
  + atencao_gasta     × $5.00/int     (de F2.1, contagem de interrupções)
  + custo_provedor    × preço real    (de PROVIDER_INTERFACE.md, logs de LLM)
  + custo_infra       × $0.10/hora    (de métricas de infraestrutura)
```

### 2.2 Platform ROI Interpretado

| Valor de Platform ROI | Interpretação | Ação |
|-----------------------|---------------|------|
| **> +100%** | **Excelente.** Cada $1 gera > $2 de retorno | Manter estratégia. Dashboard verde. |
| **+50% a +100%** | **Bom.** Plataforma gera valor consistente | Monitorar tendência semanal. |
| **+10% a +50%** | **Aceitável.** Custo cobre valor com margem | Investigar otimizações de custo. |
| **-10% a +10%** | **Atenção.** Plataforma no break-even | ⚠️ Revisar alocação de recursos. |
| **-50% a -10%** | **Ruim.** Custos superando valor gerado | 🔴 Alerta. Necessária intervenção. |
| **< -50%** | **Crítico.** Plataforma queimando recursos | 🛑 Pausar expansão. Auditoria completa. |

### 2.3 Fórmula de Tendência

O engine não calcula apenas o ROI pontual — ele calcula a **tendência**:

```
tendencia = (Platform_ROI_hoje - Platform_ROI_ontem) / Platform_ROI_ontem
```

| Tendência | Significado |
|-----------|-------------|
| **> +10%** | Plataforma acelerando em eficiência 🟢 |
| **+2% a +10%** | Melhora gradual 🟢 |
| **-2% a +2%** | Estável ⚪ |
| **-10% a -2%** | Declínio lento 🟡 |
| **< -10%** | Declínio acelerado 🔴 |

---

## 3. Dashboard Econômico Executivo

### 3.1 Dashboard Principal (Don vê em 30s)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        COSCA COGNITIVE ECONOMICS                              │
│                        ═══════════════════════════                              │
│                                                                            │
│  Período: 2026-07-30 │ Ciclo: Diário │ Engine: ★ F10.1                     │
│                                                                            │
├─────────────────────────────┬───────────────────────────────────────────────┤
│  RECEITA COGNITIVA          │  CUSTOS                                        │
│  ────────────────────       │  ──────                                        │
│                              │                                               │
│  Aprendizados:    47 ($47)  │  Tokens:   1.2M ($2.40)                       │
│  Bugs prevenidos:   3 ($15) │  Tempo:      4.2h ($42.00)                    │
│  Reuso:           12 ($6)   │  Atenção:   2 int ($10.00)                    │
│  Decisões:          8 ($40) │  Provedor:           ($0.90)                   │
│  Conh. Destilado:   5 ($15) │  Infra:              ($0.10)                   │
│  Capacidade:       18 ($36) │                                                 │
│                              │                                               │
├─────────────────────────────┼───────────────────────────────────────────────┤
│  TOTAL RECEITA:    $169     │  TOTAL CUSTOS:       $55.40                    │
│                              │                                               │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │  PLATFORM ROI:  +205%          HEALTH: 🟢 EXCELENTE                 │     │
│  │  Cada $1 gera $3.05           Tendência: ▲ +12% vs ontem           │     │
│  │  Break-even em: 0.49 dias     Sequência: 5 dias positivos           │     │
│  └────────────────────────────────────────────────────────────────────┘     │
│                                                                            │
├─────────────────────────────┬───────────────────────────────────────────────┤
│  TOP CONTRIBUIDORES         │  BOTTOM CUSTOS                                │
│  ────────────────────       │  ────────────                                │
│                              │                                               │
│  1. Autonomia (B1)    $40   │  1. Tokens LLM        $2.40                  │
│  2. Decisões (B2/B3)  $35   │  2. Tempo Don          $42.00                │
│  3. Reuso (B4)        $30   │  3. Atenção Don        $10.00                │
│  4. Prevenção (B5)    $25   │  4. Provedor           $0.90                  │
│  5. Destilação (F9.1) $15   │  5. Infra              $0.10                  │
│                              │                                               │
└─────────────────────────────┴───────────────────────────────────────────────┘
```

### 3.2 Dashboard de Tendência (Don vê em 15s)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  TENDÊNCIA ECONÔMICA — Últimos 7 Dias                                       │
│                                                                            │
│  Platform ROI ▲                                                            │
│  +250% ┤                    ╭──╮                                           │
│  +200% ┤         ╭──────────╯  ╰──╮                                        │
│  +150% ┤   ╭─────╯                 ╰──╮                                     │
│  +100% ┤──╯                           ╰──                                  │
│   +50% ┤                                                                   │
│       └───────────────────────────────────────────                         │
│         24  25  26  27  28  29  30  Julho                                  │
│                                                                            │
│  RECEITA: ▲ ▲ ▲ ▲ ▼ ▲ ▲      CUSTO: ▼ ▼ ▲ ▼ ▼ ▼ ▼                         │
│  MÉDIA 7d: +168%              RECORDE: +205% (hoje)                       │
│  PIOR DIA: +82% (24/07)       MELHORANDO: ▲ 7/7 dias                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.3 Relatório Executivo (1-parágrafo para o Don)

```
📊 RELATÓRIO COGNITIVE ECONOMICS — 2026-07-30

Plataforma gerou ROI de +205% hoje — cada $1 investido retornou $3.05 em valor cognitivo.
5º dia consecutivo de crescimento (▲ +12% vs ontem). 

🔹 Destaques positivos:
  - 47 novos aprendizados registrados (receita: $47)
  - 3 bugs prevenidos por detecção proativa (economia estimada: $15)
  - 18 alocações eficientes via Capability Market ($36)
  - 8 decisões com evidência DDNA ($40)

🔸 Principal custo:
  - Tempo do Don: $42 (76% dos custos) — dentro do esperado para sessão complexa
  - Tokens LLM: $2.40 (4% dos custos) — eficiência de provider excelente

⚡ Recomendação: Manter estratégia atual. Plataforma operando na fronteira de eficiência.
```

---

## 4. Pipeline de Coleta e Cálculo

### 4.1 Pipeline Diário

```
                          PIPELINE DIÁRIO DE COGNITIVE ECONOMICS
  ═══════════════════════════════════════════════════════════════════════════════

  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
  │  07:00 UTC   │    │  07:05 UTC   │    │  07:10 UTC   │    │  07:15 UTC   │
  │              │    │              │    │              │    │              │
  │  FASE 1      │───▶│  FASE 2      │───▶│  FASE 3      │───▶│  FASE 4      │
  │  Coleta      │    │  Cálculo     │    │  Comparação  │    │  Relatório   │
  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘    └──────┬───────┘
         │                   │                   │                   │
         ▼                   ▼                   ▼                   ▼
  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
  │ F2.1: ROI/task  │ │ Calcular        │ │ Comparar com    │ │ Gerar dashboard │
  │ F1.5: B1-B5     │ │ Σ(valor_gerado) │ │ período anter.  │ │ Gerar relatório │
  │ F7.2: Trust     │ │ Σ(custo_total)  │ │ Calcular        │ │ Disparar        │
  │ F7.3: Eng Score │ │ Platform ROI    │ │ tendência       │ │ alertas se ROI  │
  │ F9.1: Conh.     │ │                 │ │ Detectar        │ │ caindo          │
  │ F8.1: Alocações │ │                 │ │ anomalias       │ │ Arquivar no     │
  │ Provider logs   │ │                 │ │                 │ │ Decision DNA    │
  └─────────────────┘ └─────────────────┘ └─────────────────┘ └─────────────────┘
```

### 4.2 Pipeline Semanal

```
  ┌───────────────────────────────────────────────────────────────────────────┐
  │  PIPELINE SEMANAL (Cada segunda-feira 08:00 UTC)                           │
  │                                                                           │
  │ 1. Agrega 7 relatórios diários em um semanal                              │
  │ 2. Calcula média móvel de Platform ROI (7 dias)                           │
  │ 3. Identifica padrões: dias mais/menos eficientes                         │
  │ 4. Gera relatório executivo semanal para o Don                            │
  │ 5. Recomenda ajustes de budget e alocação                                 │
  │ 6. Arquivado em internal/embed/cosca/engines/cognitive-economics/reports/      │
  └───────────────────────────────────────────────────────────────────────────┘
```

### 4.3 Coleta de Dados — Fontes Específicas

| Engine / Fonte | Arquivo | O que coleta | Formato |
|----------------|---------|--------------|---------|
| **F2.1 Cognitive Economy** | `engines/cognitive-economy/SKILL.md` | ROI de cada task executada | `efficiency_score`, `was_worth_it` |
| **F1.5 B1** | `metrics/B1_autonomous_problems.md` | Tasks autônomas (sem intervenção) | `autonomous: true/false`, `agents_mobilized` |
| **F1.5 B2** | `metrics/B2_predictions.md` | Previsões corretas | `accuracy_rate`, `total_predictions` |
| **F1.5 B3** | `metrics/B3_reverted_decisions.md` | Decisões revertidas | `reversion_count`, `cause_category` |
| **F1.5 B4** | `metrics/B4_reused_knowledge.md` | Reuso de conhecimento | `reuse_count`, `pattern_id` |
| **F1.5 B5** | `metrics/B5_prefailure_detection.md` | Detecção proativa de falhas | `detection_count`, `severity`, `lead_time` |
| **F7.2 Trust Registry** | `memory/trust/TRUST_REGISTRY.md` | Tasks executadas por agente | `outcome`, `latency`, `cost`, `confidence_delta` |
| **F7.3 Engineering Score** | `analytics/engineering-score.md` | Qualidade dos commits | `engineering_score`, `dimension_scores` |
| **F9.1 Experience Compiler** | `engines/experience-compiler/SKILL.md` | Conhecimento destilado | `patterns_generated`, `principles`, `maturity_score` |
| **F8.1 Capability Market** | `engines/capability-market/SKILL.md` | Alocações do mercado | `bids_won`, `avg_score`, `allocation_efficiency` |
| **Provider Logs** | `PROVIDER_INTERFACE.md` + runtime logs | Custo real de LLM | `tokens_used`, `cost_per_call`, `model_used` |

### 4.4 Performance — Cálculo < 5s

O engine não executa novas queries pesadas. Ele lê agregados já calculados:

```
⏱️  Budget de tempo: < 5 segundos para cálculo completo

   Leitura de agregados:    ~1.5s  (7 arquivos, todos em cache)
   Cálculo das fórmulas:    ~0.5s  (aritmética simples, sem loops)
   Comparação com período:  ~0.5s  (delta simples)
   Geração de dashboard:    ~1.0s  (template pre-compilado)
   Geração de relatório:    ~1.0s  (template pre-compilado)
   Total:                   ~4.5s  (dentro do budget)
```

---

## 5. Tabela de Preços — Valor de Cada Ativo Cognitivo

### 5.1 Tabela de Precificação (em USD)

Cada ativo cognitivo gerado pela plataforma tem um **valor econômico** atribuído. Estes preços são usado no cálculo da Receita Cognitiva Total.

| # | Ativo Cognitivo | Preço Unitário | Fonte | Justificativa |
|---|-----------------|:--------------:|-------|---------------|
| 1 | **Learning registrado** | **$1.00** | F9.1, F2.1 | Custo de descoberta. Cada learning é um micro-insight que economiza tempo futuro. |
| 2 | **Bug prevenido (proativo)** | **$5.00** | F1.5 B5, F2.1 | Custo médio de bug em produção evitado (base: 5h ~ $50 + impacto reputacional, amortizado por probabilidade de ocorrência). |
| 3 | **Padrão reusado** | **$0.50** | F1.5 B4, F9.1 | Cada reuso de padrão economiza ~30min de redescoberta. $0.50 = valor marginal por reaplicação. |
| 4 | **Decisão melhor (com evidência DDNA)** | **$5.00** | F7.3, DDNA, F1.5 B2/B3 | Decisão com evidência rastreável evita retrabalho. Custo de uma reversão evitada. |
| 5 | **Hora economizada** | **$10.00** | F1.5 B1, F8.1 | Tempo do Don é o recurso mais caro. Cada hora de autonomia = $10 de custo evitado. |
| 6 | **Conhecimento destilado (padrão compilado)** | **$3.00** | F9.1 | Padrão compilado pelo Experience Compiler, pronto para reuso cross-agent. |
| 7 | **Alocação eficiente de capacidade** | **$2.00** | F8.1 | Quando o Capability Market aloca o agente ideal para a task, economizando custo de tentativa-e-erro. |
| 8 | **Previsão correta** | **$1.00** | F7.1, F1.5 B2 | Previsão acurada permite planejamento melhor e evita over-provisioning de recursos. |
| 9 | **Commit com Engineering Score > 80** | **$2.00** | F7.3 | Commit de alta qualidade reduz dívida técnica futura. Bônus por qualidade. |
| 10 | **Melhoria de confiança (confidence_delta > 0.05)** | **$1.00** | F7.2 | Agente que aprendeu e melhorou sua confiança no domínio. |

### 5.2 Fatores de Ajuste de Preço

Os preços base podem ser ajustados por contexto:

```
adjustment_factors:
  domínio_crítico:
    security:            1.5×    # Bugs de segurança valem 50% mais
    data_integrity:      1.3×    # Problemas de dados valem 30% mais
    governance:          1.2×    # Conformidade vale 20% mais
  
  momentum:
    domínio_com_momentum_baixo:  1.2×    # Aprendizado em área negligenciada vale mais
    domínio_com_momentum_alto:   0.8×    # Aprendizado em área saturada vale menos
  
  cross_domain:
    padrão_cross_domain:         2.0×    # Padrão que conecta 2+ domínios vale o dobro
    learning_cross_agent:        1.5×    # Learning validado por múltiplos agentes
  
  raridade:
    primeira_descoberta_domínio: 3.0×    # Pioneirismo em domínio inexplorado
    padrão_novo:                 2.0×    # Primeira ocorrência de um padrão
```

### 5.3 Precificação de Custos (em USD)

| Componente de Custo | Preço Unitário | Fonte | Notas |
|---------------------|:--------------:|-------|-------|
| **Token LLM (provider premium)** | $15.00/1M tokens | PROVIDER_INTERFACE.md | GPT-4, Claude Opus, DeepSeek Pro |
| **Token LLM (provider standard)** | $3.00/1M tokens | PROVIDER_INTERFACE.md | Claude Sonnet, GPT-4o-mini |
| **Token LLM (provider budget)** | $0.15/1M tokens | PROVIDER_INTERFACE.md | Haiku, Flash, Llama |
| **Tempo do Don** | $10.00/hora | Estimativa | Custo de oportunidade do Don |
| **Atenção do Don (interrupção)** | $5.00/unidade | Estimativa | Cada interrupção quebra o fluxo |
| **Infraestrutura (compute)** | $0.10/hora | Métricas de infra | CPU, RAM, disco |

---

## 6. Integração com F2.1 — Cognitive Economy

### 6.1 F10.1 Agrega os ROIs Individuais do F2.1

O F2.1 calcula o ROI de **cada task** individualmente. O F10.1 pega todos esses ROIs e os agrega em um **Platform ROI**:

```
F10.1_aggregation = {
  total_tasks_executed:    contar_tasks(F2.1_logs),
  avg_efficiency_score:    media(efficiency_score de todas tasks),
  tasks_worth_it:          contar(was_worth_it == true),
  total_tokens_consumed:   sum(tokens_gastos),
  total_learnings:         sum(learnings_gerados),
  total_interruptions:     sum(interrupcoes),
  execution_time_total:    sum(tempo_execucao),
  
  platform_roi:            calcular_platform_roi(todos_acima)
}
```

### 6.2 Mapa de Dados F2.1 → F10.1

| Dado do F2.1 | Uso no F10.1 | Conversão |
|--------------|--------------|-----------|
| `custo_total` (5 dimensões) | Componente de `Σ(custo_total)` | Converter para $ via tabela de preços |
| `valor_total` (5 dimensões) | Componente de `Σ(valor_gerado)` | Converter para $ via tabela de preços |
| `efficiency_score` | Métrica de saúde do portfólio | Média do período |
| `was_worth_it` | Taxa de acerto das decisões | `tasks_worth_it / total_tasks` |
| `action.type` | Distribuição por tipo de ação | Categorizar receita/custo por tipo |
| `context.session_token_budget_remaining` | Saúde orçamentária | Budget restante da sessão |

### 6.3 Exemplo de Agregação

```
Tasks executadas no período: 14
├── 11 com was_worth_it = true  (78.6% de acerto)
├──  3 com was_worth_it = false (21.4% de erro)

Total de tokens consumidos: 187K
Total de tempo de execução: 4.2h
Total de interrupções: 2
Total de aprendizados: 47
Total de reusos: 12

Média efficiency_score: 2.14
Maior efficiency: 3.51 (cross-audit security)
Menor efficiency: 0.15 (lint de código legacy)
```

---

## 7. Integração com F1.5 — Métricas B1-B5

### 7.1 B1-B5 como Indicadores de Saúde Econômica

As 5 métricas reais (B1-B5) são os **indicadores vitais** da saúde econômica da plataforma. Cada uma alimenta um componente específico do Platform ROI:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  B1-B5 → PLATFORM ROI MAP                                                    │
│                                                                             │
│  B1 (Autonomia) ────────────→ tempo_economizado ($10/hora economizada)       │
│                                  ↓                                          │
│                          Σ(valor_gerado) + $X                               │
│                                                                             │
│  B2 (Previsões) ────────────→ decisoes_melhores ($5.00 por previsão certa)  │
│                                  ↓                                          │
│                          Σ(valor_gerado) + $X                               │
│                                                                             │
│  B3 (Reversões) ────────────→ decisoes_melhores ($5.00 por reversão evitada)│
│                                  ↓                                          │
│                          Σ(valor_gerado) + $X                               │
│                                                                             │
│  B4 (Reuso) ────────────────→ reuso_de_padroes ($0.50 por reuso)            │
│                                  ↓                                          │
│                          Σ(valor_gerado) + $X                               │
│                                                                             │
│  B5 (Prefailure) ───────────→ bugs_prevenidos ($5.00 por detecção proativa) │
│                                  ↓                                          │
│                          Σ(valor_gerado) + $X                               │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 7.2 Cálculo de Valor a Partir de B1-B5

```yaml
b1_valor:
  metrica: "Tasks autônomas sem intervenção"
  formula: "total_autonomous_tasks × 0.5h_economizada × $10.00/h"
  atual: "20 tasks autônomas → 20 × 0.5 × $10 = $100.00 em tempo economizado"
  fonte: "metrics/B1_autonomous_problems.md"

b2_valor:
  metrica: "Previsões corretas"
  formula: "total_correct_predictions × $1.00"
  atual: "7 previsões corretas → $7.00"
  fonte: "metrics/B2_predictions.md"

b3_valor:
  metrica: "Decisões não revertidas (economia de retrabalho)"
  formula: "reversoes_evitadas × $5.00"
  atual: "1 reversão evitada (vs baseline) → $5.00"
  fonte: "metrics/B3_reverted_decisions.md"

b4_valor:
  metrica: "Conhecimento reutilizado"
  formula: "total_reuses × $0.50"
  atual: "12 reusos → $6.00"
  fonte: "metrics/B4_reused_knowledge.md"

b5_valor:
  metrica: "Problemas detectados antes da falha"
  formula: "total_detections × $5.00"
  atual: "5 detecções proativas → $25.00"
  fonte: "metrics/B5_prefailure_detection.md"
```

### 7.3 B1-B5 como Sinais de Alerta Econômico

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  ALERTAS BASEADOS EM B1-B5                                                   │
│                                                                             │
│  🟡 B1 caindo   → Autonomia diminuindo → mais interrupções ao Don          │
│                    → Custo de atenção sobe → Platform ROI cai              │
│                                                                             │
│  🟡 B2 caindo   → Previsões menos acuradas → planejamento pior            │
│                    → Mais retrabalho → Custo de execução sobe              │
│                                                                             │
│  🔴 B3 subindo  → Mais decisões revertidas → qualidade caindo              │
│                    → Custo de retrabalho → Platform ROI negativo            │
│                                                                             │
│  🟢 B4 subindo  → Mais reuso de conhecimento → eficiência cognitiva        │
│                    → Valor sem custo adicional → Platform ROI sobe          │
│                                                                             │
│  🟢 B5 subindo  → Mais detecção proativa → bugs evitados                   │
│                    → Custo de bug em produção evitado → Platform ROI sobe   │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 8. Integração com F7.1 — Prediction Engine

### 8.1 Previsão de Custo

O Prediction Engine (F7.1) fornece ao F10.1 **previsões de custo** para o período seguinte:

```yaml
prediction_input:
  engine: "F7.1 — Prediction Engine"
  arquivo: "engines/prediction/SKILL.md"
  
  dados_fornecidos:
    - "Previsão de tokens a consumir no próximo período"
    - "Previsão de tempo de execução das tasks planejadas"
    - "Previsão de taxa de sucesso (confidence)"
    - "Previsão de risco de falha"
  
  uso_no_F10.1:
    - "Platform ROI previsto para amanhã (projeção)"
    - "Orçamento preditivo: 'com este budget, ROI esperado é X%'"
    - "Detecção antecipada: 'se seguir este padrão, ROI cairá Y%'"
```

### 8.2 Platform ROI Preditivo

O F10.1 pode calcular um **Platform ROI previsto** para o próximo período:

```
Platform_ROI_previsto = (Σ(valor_previsto) / Σ(custo_previsto)) - 1

Onde:
  Σ(valor_previsto) = Baseado na média de valor gerado por task × tasks planejadas
  Σ(custo_previsto) = Previsão do F7.1 para tokens + tempo + atenção
```

### 8.3 Ciclo de Calibração

```
F7.1 prevê custo → F10.1 calcula ROI previsto → Período executa →
F2.1 mede ROI real de cada task → F10.1 calcula ROI real →
Compara ROI previsto vs real → Feedback para F7.1 recalibrar previsões
```

---

## 9. Integração com F7.2 — Trust Registry

### 9.1 Tasks Executadas e Reputação

O Trust Registry (F7.2) fornece o **histórico de execução** que alimenta o cálculo de custo real:

```yaml
trust_registry_input:
  engine: "F7.2 — Trust Registry"
  arquivo: "memory/trust/TRUST_REGISTRY.md"
  
  dados_fornecidos:
    - "Total de tasks executadas por período"
    - "Taxa de sucesso por agente (success_rate)"
    - "Custo médio por task (cost)"
    - "Tempo médio de execução (avg_latency)"
    - "Confidence delta médio (confiança ganha/perdida)"
  
  uso_no_F10.1:
    - "Σ(task_cost) para cálculo de custo total de execução"
    - "Custo médio como baseline para previsão de custo futuro"
    - "Agentes com baixa reputação → custo mais alto (retrabalho provável)"
```

### 9.2 Ajuste de Custo por Reputação

```
Se Trust Registry indica que um agente tem:
  success_rate < 0.70 → custo_real = custo_nominal × 1.3
  success_rate < 0.50 → custo_real = custo_nominal × 1.5
  confidence_delta < 0 → custo_real = custo_nominal × 1.2
  avg_latency > 2× baseline → custo_real = custo_nominal × 1.4
```

---

## 10. Integração com F7.3 — Engineering Score

### 10.1 Qualidade dos Commits como Valor

O Engineering Score (F7.3) mede a **qualidade do código produzido**. Esta qualidade é um ativo cognitivo com valor econômico:

```yaml
engineering_score_input:
  engine: "F7.3 — Engineering Score"
  arquivo: "analytics/engineering-score.md"
  
  dados_fornecidos:
    - "engineering_score médio do período"
    - "Commit count no período"
    - "Dimensões com maior/menor score"
    - "Tendência semanal do score"
  
  uso_no_F10.1:
    - "Valor por commit de alta qualidade: commits × $2.00 (se score > 80)"
    - "Desconto por commit de baixa qualidade: commits × $1.00 (se score < 50)"
    - "Dívida técnica acumulada como custo futuro (provisionado)"
```

### 10.2 Engenharia de Qualidade → ROI

```
Qualidade do código afeta o Platform ROI de 3 formas:

1. Commit de alta qualidade (score > 80):
   → Gera ativo cognitivo: +$2.00/commit na receita

2. Commit de baixa qualidade (score < 50):
   → Gera dívida técnica: -$1.00/commit na receita (provisionado)

3. Engineering Score médio > 75:
   → Multiplicador de confiança: +5% no valor dos aprendizados gerados
     (código de qualidade tem learnings mais confiáveis)
```

---

## 11. Integração com F9.1 — Experience Compiler

### 11.1 Conhecimento Destilado como Receita

O Experience Compiler (F9.1) transforma learnings brutos em **padrões reutilizáveis e princípios**. Este é um dos maiores geradores de valor da plataforma:

```yaml
experience_compiler_input:
  engine: "F9.1 — Experience Compiler"
  arquivo: "engines/experience-compiler/SKILL.md"
  
  dados_fornecidos:
    - "Padrões compilados no período"
    - "Princípios extraídos"
    - "Maturidade média dos padrões (maturity_score)"
    - "Emendas constitucionais propostas"
  
  valor_economico:
    learning_bruto:          $1.00  # Cada learning registrado
    padrão_compilado:        $3.00  # Padrão destilado e validado (5+ learnings → 1 padrão)
    princípio_estabelecido:  $10.00 # Princípio cross-domain (10+ padrões → 1 princípio)
    emenda_constitucional:   $50.00 # Mudança na CONSTITUIÇÃO (impacto sistêmico)
```

### 11.2 Maturidade do Conhecimento → Valor

```
Maturidade do padrão (F9.1) afeta o multiplicador de valor:

  maturity_score < 0.3:  × 1.0 (padrão imaturo, valor base)
  maturity_score 0.3-0.6: × 1.5 (padrão em desenvolvimento)
  maturity_score 0.6-0.8: × 2.0 (padrão maduro, alta confiabilidade)
  maturity_score > 0.8:   × 3.0 (padrão consolidado, referência)
```

---

## 12. Integração com F8.1 — Capability Market

### 12.1 Eficiência de Alocação como Valor

O Capability Market (F8.1) aloca agentes para tasks via leilão. Uma alocação eficiente **economiza recursos** e **maximiza valor**:

```yaml
capability_market_input:
  engine: "F8.1 — Capability Market"
  arquivo: "engines/capability-market/SKILL.md"
  
  dados_fornecidos:
    - "Total de leilões realizados"
    - "Bids vencedores (avg_score, avg_cost)"
    - "Shadow bids (alocações que seriam piores)"
    - "Eficiência de alocação (score_vencedor / score_médio)"
  
  valor_economico:
    alocacao_eficiente:         $2.00  # Quando o melhor agente vence o leilão
    economia_vs_baseline:       $1.00  # Quando custo é menor que a média histórica
    shadow_bid_evitado:         $0.50  # Quando alocação sub-ótima foi evitada
  
  ajuste_por_eficiencia:
    allocation_efficiency > 0.8:  × 1.5  # Mercado funcionando bem
    allocation_efficiency < 0.5:  × 0.5  # Mercado ineficiente, alocações erradas
```

### 12.2 Custo de Oportunidade

O Capability Market também expõe o **custo de oportunidade** de alocações sub-ótimas. O F10.1 registra:

```yaml
opportunity_cost:
  definicao: "Valor perdido quando uma task foi alocada para um agente sub-ótimo"
  formula: "avg_score_vencedor - score_melhor_agente_disponivel × task_value"
  tracking: "Registrado como custo implícito no relatório econômico"
  alerta: "Se opportunity_cost acumulado > $10/dia → revisar mercado"
```

---

## 13. Exemplo com Dados Reais da Sessão

### 13.1 Contexto da Sessão

Esta sessão (Evolution Marathon, iniciada em 2026-07-28) envolveu:

| Parâmetro | Valor |
|-----------|-------|
| **Agentes ativos** | 27 (de 51 totais, 8 ondas de ativação) |
| **Ondas de ativação** | 8 (da Onda 1 de arquitetura à Onda 8 de liderança) |
| **Dias de sessão** | 3 (28, 29, 30 de julho de 2026) |
| **Commits** | 10 commits principais |
| **Arquivos alterados** | 150+ |
| **Learnings registrados** | De L1 a L47 (47 learnings totais) |
| **Plataforma** | Evoluída de v1.3.0 para v1.4.0-dev |

### 13.2 Coleta de Dados — Período: 2026-07-30

#### Receita Cognitiva (Valor Gerado)

| Ativo | Quantidade | Preço Unit. | Receita | Fonte |
|------|:----------:|:-----------:|:-------:|-------|
| **Aprendizados** (learnings L1-L47) | 47 | $1.00 | $47.00 | F9.1, F2.1 |
| **Bugs prevenidos** (B5: detecções proativas) | 5 | $5.00 | $25.00 | F1.5 B5 |
| **Reuso de padrões** (B4: 12 reusos em 3 padrões) | 12 | $0.50 | $6.00 | F1.5 B4 |
| **Decisões melhores** (B2: 7 previsões corretas + B3: 1 reversão evitada + commits de qualidade) | 8 | $5.00 | $40.00 | F1.5 B2/B3, F7.3 |
| **Conhecimento destilado** (padrões compilados do F9.1) | 5 | $3.00 | $15.00 | F9.1 |
| **Alocações eficientes** (F8.1 Capability Market) | 18 | $2.00 | $36.00 | F8.1 |
| **Tempo economizado** (B1: 20 tasks autônomas × 0.5h = 10h) | 10h | $10.00 | $100.00 | F1.5 B1 |
| **Total Receita** | | | **$269.00** | |

#### Custos Totais

| Componente | Quantidade | Preço Unit. | Custo | Fonte |
|-----------|:----------:|:-----------:|:-----:|-------|
| **Tokens LLM** | ~187K tokens | $2.00/1M (média) | $0.37 | F2.1, PROVIDER |
| **Tempo do Don** | ~4.2h de sessão | $10.00/h | $42.00 | F2.1, runtime logs |
| **Atenção do Don** | 2 interrupções | $5.00/cada | $10.00 | F2.1 |
| **Custo Provedor** | Mistura premium/standard | — | $0.90 | PROVIDER_INTERFACE.md |
| **Custo Infra** | 3 dias × ~$0.10/h | $0.10/h | $7.20 | Métricas de infra |
| **Dívida técnica (commits < 50 score)** | 1 commit | $1.00 | $1.00 | F7.3 (provisionado) |
| **Total Custos** | | | **$61.47** | |

### 13.3 Cálculo do Platform ROI

```
Σ(valor_gerado) = $269.00
Σ(custo_total)  = $61.47

Platform ROI = ($269.00 / $61.47) - 1
             = 4.376 - 1
             = +337.6%

Interpretação: Cada $1 gasto na plataforma gerou $4.38 em valor cognitivo.
               ROI de +337.6% → 🟢 EXCELENTE
```

### 13.4 Detalhamento

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  PLATFORM ROI — SESSÃO EVOLUTION MARATHON (2026-07-28 a 2026-07-30)          │
│                                                                             │
│  RECEITA COGNITIVA                                                           │
│  ────────────────                                                           │
│  Aprendizados (L1-L47)          47 × $1.00  =   $47.00   ████████████░░  17%│
│  Bugs prevenidos                 5 × $5.00  =   $25.00   ██████░░░░░░░░   9%│
│  Reuso de padrões               12 × $0.50  =    $6.00   █░░░░░░░░░░░░░   2%│
│  Decisões melhores               8 × $5.00  =   $40.00   █████████░░░░░  15%│
│  Conhecimento destilado          5 × $3.00  =   $15.00   ███░░░░░░░░░░░   6%│
│  Alocações eficientes           18 × $2.00  =   $36.00   ████████░░░░░░  13%│
│  Tempo economizado             10h × $10.00 =  $100.00   ██████████████  37%│
│                                   ─────────                                    │
│                                   $269.00                      100%          │
│                                                                             │
│  CUSTOS                                                                      │
│  ──────                                                                      │
│  Tokens LLM                      187K       =    $0.37   ░░░░░░░░░░░░░   1%│
│  Tempo do Don                    4.2h       =   $42.00   ██████████████  68%│
│  Atenção do Don                  2 int      =   $10.00   ███░░░░░░░░░░░  16%│
│  Custo Provedor                  misto      =    $0.90   ░░░░░░░░░░░░░   1%│
│  Custo Infra                     72h        =    $7.20   ██░░░░░░░░░░░░  12%│
│  Dívida técnica                  1 commit   =    $1.00   ░░░░░░░░░░░░░   2%│
│                                   ─────────                                    │
│                                   $61.47                       100%          │
│                                                                             │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │  PLATFORM ROI: +337.6%              HEALTH: 🟢 EXCELENTE            │     │
│  │  Cada $1 gera $4.38                ⚡ RECORDE DA SESSÃO              │     │
│  └────────────────────────────────────────────────────────────────────┘     │
│                                                                             │
│  TOP 3 CONTRIBUIDORES:                                                      │
│  1. Tempo economizado (B1): $100.00 — 37% da receita                        │
│  2. Aprendizados: $47.00 — 17% da receita                                   │
│  3. Decisões melhores: $40.00 — 15% da receita                             │
│                                                                             │
│  PRINCIPAL CUSTO:                                                           │
│  Tempo do Don: $42.00 — 68% dos custos. Gestão de atenção é prioridade.    │
│                                                                             │
│  RECOMENDAÇÃO: Plataforma operando acima da fronteira de eficiência.        │
│  Manter estratégia de autonomia e investimento em capacidade cognitiva.     │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 13.5 Análise de Tendência (3 dias)

```
Dia 1 (28/07): Platform ROI +168%   — Ativação de agentes, custo alto de setup
Dia 2 (29/07): Platform ROI +245%   — Produção de conhecimento acelera
Dia 3 (30/07): Platform ROI +338%   — Maturidade, reuso e autonomia dominam

Tendência: ▲ +101% em 3 dias
           ▲ +38% vs ontem
           Sequência: 3 dias positivos consecutivos
```

---

## 14. Alertas Configuráveis

### 14.1 Tabela de Alertas

| # | Alerta | Condição | Severidade | Canais |
|---|--------|----------|:----------:|--------|
| 1 | **ROI Crítico** | Platform ROI < -50% | 🔴 P0 | Don notificado imediatamente + dashboard + log |
| 2 | **ROI em Declínio** | Platform ROI caindo > 10% em 1 dia | 🟡 P1 | Dashboard + relatório + log |
| 3 | **ROI Negativo** | Platform ROI < 0% | 🔴 P1 | Don notificado + dashboard + log |
| 4 | **Custo de Atenção Alto** | Atenção Don > $20/dia (> 4 interrupções) | 🟡 P2 | Dashboard + log |
| 5 | **Custo de Token Alto** | Tokens > 2× baseline do período | 🟡 P2 | Dashboard + log |
| 6 | **Sequência Negativa** | 3 dias consecutivos de ROI caindo | 🟠 P1 | Don notificado + sugestão de ação |
| 7 | **Eficiência de Alocação Baixa** | Capability Market efficiency < 0.5 | 🟡 P2 | Dashboard + log |
| 8 | **B3 Reversões Acima do Normal** | > 1 reversão no período | 🔴 P1 | Don notificado + auditoria |
| 9 | **B5 Zero por 7 Dias** | Nenhuma detecção proativa em 7 dias | 🟡 P3 | Dashboard + sugestão de foco |
| 10 | **B1 Queda > 20%** | Autonomia caindo mais de 20% em 1 semana | 🟡 P2 | Dashboard + log |

### 14.2 Configuração de Thresholds

```yaml
alerts:
  platform_roi:
    critical_threshold: -0.50     # -50%
    warning_threshold: 0.0        # 0%
    decline_rate_alert: -0.10     # -10% em 1 dia
    consecutive_decline_alert: 3  # 3 dias seguidos de queda
  
  costs:
    attention_daily_limit: 20.0   # $20/dia de atenção
    token_baseline_multiplier: 2.0 # 2× a média histórica
    provider_cost_spike: 1.5      # 1.5× o custo médio do provedor
  
  metrics:
    b3_max_reversions: 1          # Máximo de reversões por período
    b5_zero_days: 7               # Dias sem detecção proativa antes do alerta
    b1_autonomy_decline: 0.20     # 20% de queda na autonomia
    
  notification:
    p0: ["don_immediate", "dashboard_red", "executive_report"]
    p1: ["don_notification", "dashboard_orange", "executive_report"]
    p2: ["dashboard_yellow", "log"]
    p3: ["dashboard_info", "log"]
```

### 14.3 Exemplo de Alerta Disparado

```
═══════════════════════════════════════════════════════════════════
  🚨 ALERTA COGNITIVE ECONOMICS — P0
═══════════════════════════════════════════════════════════════════

Timestamp:  2026-07-30T14:00:00Z
Severidade: P0 — CRÍTICO
Alerta:     Platform ROI < -50%
Atual:      -52.3% (últimas 24h)
Período:    2026-07-29T14:00 → 2026-07-30T14:00

Causas identificadas:
  🔴 Custo de tokens: 3.2× acima da média (570K tokens vs 180K baseline)
  🔴 Custo de provedor: 4.1× acima da média ($3.69 vs $0.90)
  🟡 B5 (detecção proativa): 0 detecções nas últimas 24h
  🟢 B1 (autonomia): estável em 75%

Causa raiz provável:
  Sessão com 5 agentes em paralelo usando provider premium (GPT-4) 
  para tarefas que poderiam usar provider budget (Haiku).
  Recommendation: revisar provider routing no Capability Market.

Ação recomendada:
  1. Reconfigurar F8.1 para priorizar provider budget em tasks não-críticas
  2. Ativar modo econômico (budget_mode: true)
  3. Revisar B5 — domínios sem detecção proativa podem ter gaps de monitoramento

═══════════════════════════════════════════════════════════════════
```

---

## 15. Arquitetura do Motor

### 15.1 Diagrama de Arquitetura

```
┌───────────────────────────────────────────────────────────────────────────────────┐
│                        COGNITIVE ECONOMICS ENGINE ★ F10.1                          │
│                        ════════════════════════════════════                          │
│                                                                                   │
│  ┌───────────────────────────────────────────────────────────────────────────┐   │
│  │                         INPUT LAYER                                         │   │
│  │                                                                             │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐         │   │
│  │  │  F2.1    │ │  F1.5    │ │  F7.1    │ │  F7.2    │ │  F7.3    │         │   │
│  │  │ROI/task  │ │ B1-B5    │ │Prediction│ │  Trust   │ │  Eng.    │         │   │
│  │  │          │ │ Metrics  │ │ Engine   │ │ Registry │ │  Score   │         │   │
│  │  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘         │   │
│  │       └────────────┴──────────┬─┴─────────────┴────────────┘               │   │
│  │                               │                                            │   │
│  │  ┌──────────┐ ┌──────────────┐┘ ┌──────────────┐ ┌──────────────────┐    │   │
│  │  │  F9.1    │ │  F8.1        │  │  PROVIDER    │ │  RUNTIME LOGS    │    │   │
│  │  │Experience│ │  Capability  │  │  INTERFACE   │ │  (tokens/infra)  │    │   │
│  │  │ Compiler │ │  Market      │  │              │ │                  │    │   │
│  │  └────┬─────┘ └────┬────────┘  └──────┬───────┘ └────────┬─────────┘    │   │
│  │       └────────────┴──────────────────┴──────────────────┘               │   │
│  └──────────────────────────────────┬───────────────────────────────────────┘   │
│                                     │                                           │
│                                     ▼                                           │
│  ┌───────────────────────────────────────────────────────────────────────────┐   │
│  │                      AGGREGATION LAYER                                     │   │
│  │                                                                             │   │
│  │  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────────┐ │   │
│  │  │ Σ(valor_gerado)  │  │ Σ(custo_total)   │  │ Métricas de Tendência    │ │   │
│  │  │                  │  │                  │  │                          │ │   │
│  │  │ • Aprendizados   │  │ • Tokens         │  │ • ROI hoje vs ontem      │ │   │
│  │  │ • Bugs prev.     │  │ • Tempo          │  │ • Média 7 dias           │ │   │
│  │  │ • Reuso          │  │ • Atenção        │  │ • Desvio padrão          │ │   │
│  │  │ • Decisões       │  │ • Provedor       │  │ • Sequência              │ │   │
│  │  │ • Conh. destil.  │  │ • Infra          │  │ • Recorde                │ │   │
│  │  │ • Alocações      │  │ • Dívida técnica │  │                          │ │   │
│  │  │ • Tempo econom.  │  │                  │  │                          │ │   │
│  │  └────────┬─────────┘  └────────┬─────────┘  └───────────┬──────────────┘ │   │
│  │           └─────────────────────┴────────────────────────┘                 │   │
│  └──────────────────────────────────┬───────────────────────────────────────┘   │
│                                     │                                           │
│                                     ▼                                           │
│  ┌───────────────────────────────────────────────────────────────────────────┐   │
│  │                      CALCULATION LAYER                                     │   │
│  │                                                                             │   │
│  │  ┌──────────────────────────────────────────────────────────────────────┐  │   │
│  │  │  Platform ROI = (Σ(valor_gerado) / Σ(custo_total)) - 1               │  │   │
│  │  │                                                                       │  │   │
│  │  │  Tendência = (ROI_hoje - ROI_ontem) / ROI_ontem                      │  │   │
│  │  │                                                                       │  │   │
│  │  │  ROI_Preditivo = (valor_previsto / custo_previsto) - 1               │  │   │
│  │  └──────────────────────────────────────────────────────────────────────┘  │   │
│  └──────────────────────────────────┬───────────────────────────────────────┘   │
│                                     │                                           │
│                                     ▼                                           │
│  ┌───────────────────────────────────────────────────────────────────────────┐   │
│  │                      OUTPUT LAYER                                          │   │
│  │                                                                             │   │
│  │  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────────┐ │   │
│  │  │   Dashboard      │  │  Relatório       │  │  Alertas                 │ │   │
│  │  │   Executivo      │  │  Executivo       │  │  Configuráveis           │ │   │
│  │  │   (Don vê 30s)   │  │  (1 parágrafo)   │  │  (P0/P1/P2/P3)          │ │   │
│  │  └──────────────────┘  └──────────────────┘  └──────────────────────────┘ │   │
│  │                                                                             │   │
│  │  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────────────┐ │   │
│  │  │   DDNA Record    │  │  Archive         │  │  Recomendações           │ │   │
│  │  │   (decisão econ.)│  │  (reports/)      │  │  (próximo período)       │ │   │
│  │  └──────────────────┘  └──────────────────┘  └──────────────────────────┘ │   │
│  └───────────────────────────────────────────────────────────────────────────┘   │
│                                                                                   │
└───────────────────────────────────────────────────────────────────────────────────┘
```

### 15.2 Artefatos do Engine

```
internal/embed/cosca/engines/cognitive-economics/
├── SKILL.md                    ← ESTE ARQUIVO (especificação completa)
├── platform-roi-calculator.go  ← Implementação Go do Platform ROI
├── aggregator.go              ← Agrega dados de F2.1, F1.5, F7.1-F9.1, F8.1
├── dashboard-generator.go     ← Gera dashboard executivo Markdown
├── alert-engine.go            ← Engine de alertas configuráveis
├── report-generator.go        ← Gera relatório executivo
├── types.go                   ← Tipos: PlatformROI, RevenueVector, CostVector, Alert
├── config.go                  ← Configuração: thresholds, pricing table, weights
├── logs/
│   ├── daily-reports/         ← Relatórios diários (YYYY-MM-DD.md)
│   ├── weekly-reports/        ← Relatórios semanais (YYYY-WW.md)
│   └── alerts.log             ← Histórico de alertas disparados
└── tests/
    ├── platform-roi-calculator_test.go
    ├── aggregator_test.go
    └── alert-engine_test.go
```

---

## 16. Métricas do Próprio Engine

### 16.1 Métricas de Saúde do Cognitive Economics Engine

```yaml
engine_self_metrics:
  execution:
    total_calculations: 0           # Total de cálculos de Platform ROI
    avg_calculation_time_ms: 0      # Tempo médio de cálculo (< 5000ms target)
    last_calculation: null          # Timestamp da última execução
  
  data_collection:
    sources_connected: 0/7          # Quantas fontes de dados estão conectadas
    last_successful_collection: null
    collection_errors: 0            # Erros de coleta de dados
    stale_data_sources: []          # Fontes com dados desatualizados
  
  alerts:
    total_alerts_triggered: 0       # Total de alertas disparados
    p0_count: 0                     # Alertas P0
    p1_count: 0                     # Alertas P1
    p2_count: 0                     # Alertas P2
    p3_count: 0                     # Alertas P3
    false_positives: 0              # Alertas incorretos
    alert_accuracy: 0.0             # Taxa de acerto dos alertas
  
  accuracy:
    platform_roi_variance: 0.0      # Variação do ROI calculado vs ROI real auditado
    prediction_accuracy: 0.0        # Acurácia do ROI previsto vs ROI realizado
    last_calibration: null          # Última calibração de thresholds
```

### 16.2 Critérios de Sucesso

```yaml
success_criteria:
  - "Cálculo de Platform ROI em < 5s para qualquer período"
  - "Dashboard executivo legível em 30s pelo Don"
  - "7/7 fontes de dados conectadas e atualizadas"
  - "Alertas com acurácia > 90% (false positive rate < 10%)"
  - "ROI calculado automaticamente 1×/dia (07:00 UTC)"
  - "Relatório executivo gerado e arquivado diariamente"
  - "Nenhuma interrupção ao Don para coleta de dados econômicos"
  - "Arquivo de relatório acessível em < 2s via comando CLI"
```

---

## 17. Implementação e Automação

### 17.1 Comando CLI

```bash
# Calcular Platform ROI do dia atual
cosca economics roi

# Calcular Platform ROI de um período específico
cosca economics roi --from 2026-07-28 --to 2026-07-30

# Ver dashboard executivo
cosca economics dashboard

# Ver relatório executivo (1 parágrafo)
cosca economics report

# Configurar alertas
cosca economics alerts set --roi-critical -0.50 --roi-decline -0.10

# Ver histórico de alertas
cosca economics alerts history

# Ver tabela de preços atual
cosca economics pricing

# Ver tendência dos últimos 7 dias
cosca economics trend
```

### 17.2 Automação (Cron / Scheduler)

```yaml
automation:
  daily:
    schedule: "0 7 * * *"                       # 07:00 UTC todos os dias
    actions:
      - "Coletar dados de todas as 7 fontes"
      - "Calcular Platform ROI do dia anterior"
      - "Comparar com período anterior"
      - "Gerar dashboard executivo"
      - "Gerar relatório executivo (1 parágrafo)"
      - "Verificar alertas e disparar se necessário"
      - "Arquivar relatório em logs/daily-reports/"
  
  weekly:
    schedule: "0 8 * * 1"                       # 08:00 UTC toda segunda-feira
    actions:
      - "Agregar 7 relatórios diários em semanal"
      - "Calcular média móvel de 7 dias"
      - "Gerar relatório semanal para o Don"
      - "Recomendar ajustes de alocação"
  
  on_demand:
    triggers:
      - "Comando explícito do Don (cosca economics roi)"
      - "Alerta P0 disparado (cálculo automático adicional)"
      - "Fim de sessão (calcular ROI da sessão)"
```

### 17.3 Passos de Implementação

```yaml
implementation_steps:
  step_1:
    title: "Definir tipos e interfaces Go"
    effort: "2 horas"
    description: |
      Criar types.go com PlatformROI, RevenueVector, CostVector, AlertConfig.
      Interfaces: DataAggregator, ROICalculator, DashboardGenerator, AlertEngine.
  
  step_2:
    title: "Implementar Aggregator"
    effort: "4 horas"
    description: |
      aggregator.go: coleta dados de 7 fontes (F2.1, F1.5 B1-B5, F7.2, F7.3, F9.1, F8.1, runtime logs).
      Parse de arquivos markdown/YAML existentes. Cache de dados para performance.
  
  step_3:
    title: "Implementar Platform ROI Calculator"
    effort: "2 horas"
    description: |
      platform-roi-calculator.go: fórmula principal (Σvalor / Σcusto - 1).
      Cálculo de tendência, média móvel, ROI preditivo.
  
  step_4:
    title: "Implementar Dashboard Generator"
    effort: "3 horas"
    description: |
      dashboard-generator.go: geração do dashboard executivo em Markdown.
      Templates pré-compilados para performance.
  
  step_5:
    title: "Implementar Alert Engine"
    effort: "3 horas"
    description: |
      alert-engine.go: 10 alertas configuráveis com 4 níveis de severidade.
      Notificação ao Don via dashboard + log + (futuro) webhook.
  
  step_6:
    title: "Implementar Report Generator"
    effort: "2 horas"
    description: |
      report-generator.go: relatório executivo de 1 parágrafo.
      Suporte a períodos: diário, semanal, custom.
  
  step_7:
    title: "Integrar com Cognitive Economy (F2.1)"
    effort: "2 horas"
    description: |
      Integração com F2.1 para dados de ROI por task.
      Parse dos logs de predição/outcome do F2.1.
  
  step_8:
    title: "Integrar com B1-B5 Metrics (F1.5)"
    effort: "2 horas"
    description: |
      Parse dos arquivos B1-B5 em metrics/.
      Mapeamento B1-B5 → componentes de receita.
  
  step_9:
    title: "Integrar com demais engines (F7.1-F9.1, F8.1)"
    effort: "3 horas"
    description: |
      Conexão com Trust Registry, Engineering Score, Experience Compiler,
      Capability Market. Parse de arquivos estruturados.
  
  step_10:
    title: "Testes e Calibração Inicial"
    effort: "4 horas"
    description: |
      Testes unitários para cada componente.
      Teste de integração com dados reais da Evolution Marathon.
      Calibração de thresholds baseada em dados históricos.
```

---

## 18. Referências Cruzadas

| Documento | Seção | Relação |
|-----------|-------|---------|
| **F2.1 Cognitive Economy** | `engines/cognitive-economy/SKILL.md` | F10.1 agrega os ROIs individuais do F2.1 |
| **F1.5 B1-B5 Metrics** | `metrics/B1_*` a `B5_*` | Indicadores de saúde econômica da plataforma |
| **F7.1 Prediction Engine** | `engines/prediction/SKILL.md` | Previsão de custo para ROI preditivo |
| **F7.2 Trust Registry** | `memory/trust/TRUST_REGISTRY.md` | Histórico de execução e reputação de agentes |
| **F7.3 Engineering Score** | `analytics/engineering-score.md` | Qualidade dos commits como ativo cognitivo |
| **F9.1 Experience Compiler** | `engines/experience-compiler/SKILL.md` | Conhecimento destilado como receita |
| **F8.1 Capability Market** | `engines/capability-market/SKILL.md` | Alocações eficientes como valor |
| **PROVIDER_INTERFACE.md** | — | Precificação de tokens por provider |
| **CONSTITUTION.md** | Art. P2 | Código executado é a verdade absoluta (origem dos dados) |
| **COGNITIVE_MATURITY.md** | §5 C14 | Conceito original de Cognitive Economics |
| **CMI_REAL_METRICS.md** | §3 | Schema das 5 métricas B1-B5 |
| **DECISION_DNA.md** | — | Registro de decisões econômicas da plataforma |
| **KERNEL.md** | — | Pipeline de decisão onde F10.1 se integra como camada de reporting |

---

## 19. Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Architecture Chief | Criação inicial. Definição do Cognitive Economics Engine (F10.1): equação mestra Platform ROI, dashboard executivo, pipeline diário/semanal, tabela de precificação de ativos cognitivos, integração com 7 engines (F2.1, F1.5 B1-B5, F7.1, F7.2, F7.3, F9.1, F8.1), exemplo com dados reais da Evolution Marathon (27 agentes, 8 ondas, +337.6% Platform ROI), 10 alertas configuráveis, arquitetura do motor, implementação em 10 passos. |
