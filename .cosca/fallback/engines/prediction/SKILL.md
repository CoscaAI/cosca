# PREDICTION ENGINE — Motor de Predição de Tasks (F7.1)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-prediction`
> **Conceito**: Engineering Intelligence — Prediction Engine
> **Referências**: next-evolution-phases.md §F7.1 | TRUST_REGISTRY.md | DECISION_DNA.md | cognitive-maturity-implementation.md F1.2/F2.1
> **Dependências**: Trust Registry (F7.2) | Contrafactual Gate (F1.2 / G0.5) | Cognitive Economy Engine (F2.1) | Meta-cognition Pipeline (F0.6)
> **CMI Impact**: Julgamento +5, Planejamento +5, Aprendizado +3

---

## Índice

1. [O que é o Prediction Engine](#1-o-que-é-o-prediction-engine)
2. [Ciclo de Vida da Predição](#2-ciclo-de-vida-da-predição)
3. [Pipeline de Predição — PRÉ-TASK](#3-pipeline-de-predição--pré-task)
4. [Pipeline de Predição — PÓS-TASK](#4-pipeline-de-predição--pós-task)
5. [Fórmulas do Motor](#5-fórmulas-do-motor)
6. [Cálculo de Risco e Bandas](#6-cálculo-de-risco-e-bandas)
7. [Calibragem Contínua](#7-calibragem-contínua)
8. [Integração com F1.2 — Contrafactual Gate](#8-integração-com-f12--contrafactual-gate)
9. [Integração com F2.1 — Cognitive Economy](#9-integração-com-f21--cognitive-economy)
10. [Integração com Meta-cognition Pipeline (F0.6)](#10-integração-com-meta-cognition-pipeline-f06)
11. [Exemplo com Dados Reais do Trust Registry](#11-exemplo-com-dados-reais-do-trust-registry)
12. [Diagrama de Arquitetura](#12-diagrama-de-arquitetura)
13. [Gatilhos de Ativação](#13-gatilhos-de-ativação)
14. [Regras de Decisão](#14-regras-de-decisão)
15. [Métricas do Próprio Engine](#15-métricas-do-próprio-engine)
16. [Relacionados](#16-relacionados)
17. [HISTÓRICO](#17-histórico)

---

## 1. O que é o Prediction Engine

### Definição

O **Prediction Engine** é o motor do Cosca que **prevê o resultado de cada task antes de executá-la**: probabilidade de sucesso, tempo estimado, custo estimado e nível de risco. Após a execução, ele compara previsão vs realidade e recalibra seu modelo preditivo.

Ele transforma o Cosca de um sistema que **age e depois aprende** em um sistema que **calcula antes de agir** — implementando o ciclo completo de **prever → executar → medir → calibrar**.

### Filosofia

```
"Se você não pode prever, você não pode melhorar.
 Se você não pode medir, você não pode gerenciar.
 Se você não pode calibrar, você não pode confiar."
 — Cosca Kernel, 2026-07-30
```

### Inputs

| Input | Fonte | Descrição |
|-------|-------|-----------|
| `agent` | Kernel | Nome do agente candidato à execução |
| `task_type` | Kernel (Request Analysis) | Classificação da task: `testing`, `specification`, `architecture-design`, etc. |
| `complexity` | Kernel (Request Analysis) | Nível de complexidade 1-5 |
| `files_estimated` | Kernel (Request Analysis) | Número estimado de arquivos que serão alterados |
| `domain` | Kernel (Capability Resolution) | Domínio primário da task: `race-detection`, `rest-api`, `security-audit`, etc. |
| `confidence_current` | Confidence Model | Confidence score atual do agente candidato |
| `urgency` | Kernel | Prioridade da task: `critical`, `high`, `medium`, `normal`, `low` |

### Outputs

| Output | Formato | Descrição |
|--------|---------|-----------|
| `P(success)` | float 0.0-1.0 | Probabilidade estimada de sucesso |
| `tempo_estimado` | string (XdXmXs) | Tempo estimado de execução |
| `custo_estimado` | string ($0.000) | Custo estimado em tokens LLM |
| `risco` | string (🟢/🟡/🔴) | Banda de risco calculada |
| `risco_score` | float 0.0-1.0 | Score numérico de risco (1 - P(success)) |
| `confianca_predicao` | float 0.0-1.0 | Confiança do próprio engine na predição (baseada em N amostras históricas) |
| `fatores_contribuintes` | map | Decomposição dos fatores que mais influenciaram a predição |
| `recomendacao` | string | Sugestão ao Kernel: `delegar`, `revisar`, `escalonar`, `substituir_agente` |

### Objeto de Predição (Formato YAML)

```yaml
prediction:
  id: "PRED-2026-07-30-001"
  timestamp: "2026-07-30T14:30:00Z"
  task:
    agent: "cosca-testing"
    task_type: "testing"
    complexity: 3
    files_estimated: 5
    domain: "race-detection"
  inputs:
    success_rate: 1.00
    domain_strength: 0.48
    recency_weight: 0.95
    avg_latency_by_type: "13m45s"
    avg_cost_by_type: "$0.025"
    confidence_current: 0.60
  outputs:
    p_success: 0.85
    tempo_estimado: "16m30s"
    custo_estimado: "$0.033"
    risco: "🟡 moderado"
    risco_score: 0.15
    confianca_predicao: 0.72
    fatores_contribuintes:
      success_rate: 0.40    # 40% do peso
      domain_strength: 0.12 # 12% do peso (penalizado — domain_strength baixo)
      recency: 0.19         # 19% do peso
      complexity: 0.08      # 8% do peso
      confidence: 0.06      # 6% do peso (ajustado pela baixa confiança)
    recomendacao: "delegar_com_revisao"
```

---

## 2. Ciclo de Vida da Predição

```
┌────────────────────────────────────────────────────────────────────────┐
│                    CICLO DE VIDA DA PREDIÇÃO                          │
│                                                                        │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐           │
│  │ 1. INPUT │──▶│ 2. CALC  │──▶│ 3. OUTPUT│──▶│ 4. EXEC  │           │
│  │ Kernel   │   │ Engine   │   │ Engine   │   │ Task     │           │
│  │ fornece  │   │ consulta │   │ retorna  │   │ (Kernel  │           │
│  │ agent +  │   │ Trust    │   │ predição │   │ decide)  │           │
│  │ params   │   │ Registry │   │ + reco   │   │          │           │
│  └──────────┘   + calcula  │   └────┬─────┘   └────┬─────┘           │
│                  └──────────┘        │              │                  │
│                                      │              ▼                  │
│                                      │    ┌──────────────────┐        │
│                                      │    │ 5. REGISTRO      │        │
│                                      │    │ Kernel registra  │        │
│                                      │    │ outcome real no  │        │
│                                      │    │ Trust Registry   │        │
│                                      │    └────────┬─────────┘        │
│                                      │             │                   │
│                                      │    ┌────────▼─────────┐        │
│                                      │    │ 6. CALIBRAÇÃO    │        │
│                                      │    │ Engine compara   │        │
│                                      │    │ previsto vs real │        │
│                                      │    │ Ajusta pesos     │        │
│                                      │    │ Aprende padrões  │        │
│                                      │    └──────────────────┘        │
│                                      │                                 │
│                                      ▼                                 │
│                          ┌──────────────────────┐                     │
│                          │ 7. FEEDBACK          │                     │
│                          │ Próxima predição     │                     │
│                          │ será mais precisa    │                     │
│                          └──────────────────────┘                     │
└────────────────────────────────────────────────────────────────────────┘
```

### Tempo de Execução

O motor completo (consulta + cálculo) deve executar em **< 500ms**. É uma operação síncrona e leve — não envolve chamadas de LLM, apenas consulta a dados locais e aritmética.

| Fase | Tempo Máximo | Descrição |
|------|-------------|-----------|
| Consulta ao Trust Registry | 200ms | Leitura do YAML parseado em cache |
| Cálculo das fórmulas | 50ms | Aritmética simples em memória |
| Formatação do output | 50ms | Montagem do objeto de predição |
| **Total** | **300ms** | Bem abaixo do limite de 500ms |

---

## 3. Pipeline de Predição — PRÉ-TASK

### Passo 1: Kernel Define Parâmetros

O Kernel, após a **Request Analysis (Step 5)** e antes da delegação, prepara os parâmetros de predição:

```
Kernel define:
  ┌── agent: cosca-testing              # Candidato primário
  ├── task_type: testing                 # Classificação da task
  ├── complexity: 3                      # 1-5 (definido na Request Analysis)
  ├── files_estimated: 5                 # Estimativa do Kernel
  ├── domain: race-detection             # Domínio específico (do capability-profile)
  ├── confidence_current: 0.60           # Do Confidence Model
  └── urgency: high                      # P1 — prioridade elevada
```

### Passo 2: Prediction Engine Consulta o Trust Registry

O engine consulta o Trust Registry (F7.2) no arquivo `internal/embed/cosca/memory/trust/TRUST_REGISTRY.md`:

```
Consulta:
  ├── success_rate do agente para este task_type
  │     └── cosca-testing success_rate total: 1.00 (4/4 tasks)
  │
  ├── avg_latency para este task_type
  │     └── cosca-testing avg_latency testing: 13m45s
  │
  ├── avg_cost para este task_type
  │     └── cosca-testing avg_cost testing: $0.025
  │
  ├── recency_weight (tasks recentes pesam mais)
  │     └── Decay exponencial: weight = 0.95 ^ (dias_desde_ultima_task / 30)
  │
  ├── domain_strength para o domínio
  │     └── cosca-testing race-detection domain_strength: 0.48
  │
  ├── reliability_score do agente
  │     └── cosca-testing reliability_score: 0.84
  │
  └── confidence_current do agente
        └── cosca-testing confidence_after: 0.60
```

### Passo 3: Prediction Engine Calcula

```
Cálculo:
  ├── P(success) = weighted_success_rate × domain_strength × recency_factor
  │     = 0.92 × 0.48 × 0.95
  │     = 0.42  (antes dos weights)
  │     → aplica fórmula completa com weights (Seção 5)
  │     → P(success) = 0.85 (após pesos combinados)
  │
  ├── Tempo est. = avg_latency × complexity_multiplier
  │     = 13m45s × (1 + 0.2 × (3 - 1))
  │     = 13m45s × 1.4
  │     = 16m30s (19m15s × 1.4 = ~19m, ajustado para 16m30s com fator adicional)
  │
  ├── Custo est. = avg_cost × complexity_multiplier
  │     = $0.025 × (1 + 0.3 × (3 - 1))
  │     = $0.025 × 1.6
  │     = $0.033 (arredondado)
  │
  └── Risco = 1 - P(success) com banda:
        └── risco_score = 1 - 0.85 = 0.15
        └── banda: 🟢 >85% → 🟡 70-85% → 🔴 <70%
        └── resultado: 🟡 moderado (85% está no limite verde/amarelo)
```

### Passo 4: Kernel Decide

O Kernel recebe a predição e decide como proceder:

```
Predição recebida:
  ├── P(success): 85%        ──┐
  ├── Tempo est.: 16m30s       ├── Recomendação: delegar_com_revisão
  ├── Custo est.: $0.033       │
  ├── Risco: 🟡 moderado    ──┘
  └── Fatores críticos: domain_strength baixo (0.48) está puxando P(success) para baixo

Kernel decide:
  ├── Se P(success) ≥ 85%:    Delegação autônoma
  ├── Se P(success) ≥ 70%:    Delegação com supervisão leve
  ├── Se P(success) ≥ 50%:    Delegação com supervisão do Kernel
  ├── Se P(success) < 70%:    Abre Contrafactual Gate (F1.2) → decide se delega ou escalona
  └── Se P(success) < 50%:    Não delegar. Kernel executa ou escalona para o Don

  Decisão: ✅ DELEGAR com supervisão leve (P(success)=85%, dentro do aceitável)
```

---

## 4. Pipeline de Predição — PÓS-TASK

### Passo 1: Kernel Registra Resultado no Trust Registry

Após a execução da task, o Kernel adiciona uma entrada no Trust Registry com o resultado real:

```yaml
- agent: cosca-testing
  task_type: testing
  task_id: race-condition-predicted-2026-07-30-001
  outcome: success
  confidence_before: 0.60
  confidence_after: 0.65
  confidence_delta: +0.05
  latency: 18m00s
  cost: $0.030
  files_changed: 5
  loc_delta: +320
  tags:
    - testing
    - race-condition
    - prediction-validation
    - level-3
  timestamp: 2026-07-30T17:00:00Z
```

### Passo 2: Prediction Engine Compara Previsto vs Real

```yaml
comparacao:
  predicao_id: "PRED-2026-07-30-001"
  task_id: "race-condition-predicted-2026-07-30-001"

  previsto:
    p_success: 0.85
    tempo: "16m30s"
    custo: "$0.033"
    risco: "🟡 moderado"

  real:
    outcome: success
    tempo: "18m00s"
    custo: "$0.030"

  delta:
    p_success_delta: +0.15    # Real foi melhor que previsto
    tempo_delta: +1m30s       # 9% acima do previsto
    custo_delta: -$0.003      # 9% abaixo do previsto
    erro_tempo: 9.1%          # |18m00s - 16m30s| / 16m30s = 0.091
    erro_custo: 9.1%          # |$0.030 - $0.033| / $0.033 = 0.091

  veredito: "✅ DENTRO DO ACEITÁVEL — ambos os erros < 20%"
```

### Passo 3: Calibra

```yaml
calibracao:
  acao: "NENHUMA — erro < 20% para ambos os eixos"
  regra: "Se erro > 20%, ajusta peso do agente. Se erro < 10%, reforço positivo."
  status: "Modelo mantido. Nenhum ajuste necessário."
```

### Passo 4: Aprende

O engine registra um aprendizado estrutural:

```yaml
aprendizado:
  padrao: "Para task_type=testing, complexity=3, domain=race-detection:"
  observacao: "O modelo subestima levemente o tempo (+9.1%) e superestima o custo (-9.1%)"
  acao: "Ajustar complexity_multiplier de tempo de 1.4 para 1.45 para este perfil"
  impacto: "Redução esperada de erro: ~2-3% na próxima predição similar"
  tipo: "calibration_learning"
```

---

## 5. Fórmulas do Motor

### 5.1 Probabilidade de Sucesso — P(success)

```
P(success) = min(Σ(weight_i × factor_i), 1.0)

Onde os fatores são normalizados para 0.0-1.0:

Fatores (weight):
  ├── success_rate    (0.40) — Taxa histórica do agente no task_type
  │     Normalização: success_rate_total do Trust Registry
  │     (Se sem dados, usar 0.50 — confiança neutra)
  │
  ├── domain_strength (0.25) — Força do agente no domínio específico
  │     Normalização: domain_strength[domain] do Trust Registry
  │     (Se sem dados, usar avg(domain_strength) do agente)
  │
  ├── recency         (0.20) — Tasks recentes pesam mais que tasks antigas
  │     Cálculo:   recency_factor = 0.95 ^ (dias_desde_ultima_task / 30)
  │     Se última task foi hoje:  0.95^0 = 1.0
  │     Se última task foi há 30d: 0.95^1 = 0.95
  │     Se última task foi há 90d: 0.95^3 = 0.857
  │     Se última task foi há 365d: 0.95^12 = 0.540
  │     (Decay follow Trust Registry §3.3)
  │
  ├── complexity      (0.10) — Quão bem o agente lida com complexidade
  │     Cálculo:   complexity_score = avg(level) / 5.0
  │     Onde avg(level) é a média dos níveis (1-5) das tasks do agente
  │     Ex: cosca-testing avg level = 3.0 → 3.0/5.0 = 0.60
  │
  └── confidence      (0.05) — Confidence score atual do agente
        Normalização: confidence_current do Confidence Model
        (Valor direto, já está 0.0-1.0)

EXEMPLO — cosca-testing (race-detection, complexity=3):

  success_rate    = 1.00  (4/4 tasks, last_20: 1.00)
  domain_strength = 0.48  (race-detection domain_strength)
  recency         = 0.95  (última task hoje, mas decay leve)
  complexity      = 0.60  (avg level 3.0 / 5.0)
  confidence      = 0.60  (confidence_after atual)

  P(success) = min(
      0.40 × 1.00
    + 0.25 × 0.48
    + 0.20 × 0.95
    + 0.10 × 0.60
    + 0.05 × 0.60
  , 1.0)

  P(success) = min(0.400 + 0.120 + 0.190 + 0.060 + 0.030, 1.0)
  P(success) = min(0.800, 1.0)
  P(success) = 0.80

  Nota: O domain_strength baixo (0.48) para race-detection penalizou a predição.
  Se domain_strength fosse 0.88, P(success) subiria para:
  0.400 + 0.220 + 0.190 + 0.060 + 0.030 = 0.90
```

### 5.2 Tempo Estimado

```
Tempo est. = avg_latency × (1 + 0.2 × (complexity - 1))

Onde:
  avg_latency     = avg_latency.by_task_type[task_type] do Trust Registry
  complexity      = 1-5 (definido pelo Kernel)

EXEMPLO — cosca-testing testing, complexity=3:

  avg_latency_testing = 13m45s = 825s
  complexity          = 3

  Tempo est. = 825s × (1 + 0.2 × (3 - 1))
             = 825s × (1 + 0.2 × 2)
             = 825s × (1 + 0.4)
             = 825s × 1.4
             = 1.155s
             = 19m15s

  Ajuste fino por domain_strength:
    Se domain_strength < 0.50: tempo_est += tempo_est × 0.10 (10% de incerteza)
    Se domain_strength > 0.85: tempo_est -= tempo_est × 0.05 (5% de confiança)

  Tempo est. final = 19m15s + 1m55s = ~21m (domain_strength baixo = incerteza)
```

### 5.3 Custo Estimado

```
Custo est. = avg_cost × (1 + 0.3 × (complexity - 1))

Onde:
  avg_cost        = avg_cost.by_task_type[task_type] do Trust Registry
  complexity      = 1-5 (definido pelo Kernel)

EXEMPLO — cosca-testing testing, complexity=3:

  avg_cost_testing = $0.025
  complexity       = 3

  Custo est. = $0.025 × (1 + 0.3 × (3 - 1))
             = $0.025 × (1 + 0.3 × 2)
             = $0.025 × (1 + 0.6)
             = $0.025 × 1.6
             = $0.040

  Ajuste fino por domain_strength:
    Se domain_strength < 0.50: custo_est += custo_est × 0.15 (15% de incerteza)
    Se domain_strength > 0.85: custo_est -= custo_est × 0.05 (5% de otimismo)

  Custo est. final = $0.040 + $0.006 = ~$0.046 (domain_strength baixo = mais iterações)
```

### 5.4 Fórmulas de Ajuste Fino (Domain Strength)

O domain_strength é o fator de maior variança. Quando o agente é forte no domínio, a predição é mais otimista (confiante). Quando é fraco, a predição é mais pessimista (precavida):

```
Ajustes por Domain Strength:

┌───────────────────┬──────────────┬──────────────┬─────────────────┐
│ Domain Strength   │ Tempo (bônus)│ Custo (bônus)│ P(success)      │
├───────────────────┼──────────────┼──────────────┼─────────────────┤
│ ≥ 0.90            │ -10%         │ -10%         │ +0.05 direct    │
│ 0.80 - 0.89       │ -5%          │ -5%          │ +0.03 direct    │
│ 0.60 - 0.79       │ 0%           │ 0%           │ 0%              │
│ 0.40 - 0.59       │ +10%         │ +15%         │ -0.03 direct    │
│ < 0.40            │ +20%         │ +25%         │ -0.05 direct    │
└───────────────────┴──────────────┴──────────────┴─────────────────┘
```

### 5.5 Confiança da Predição

A confiança do próprio engine na predição é calculada com base no volume de dados históricos disponíveis:

```
confianca_predicao = min(N_amostras / 20, 1.0) × 0.60
                   + (1 - variancia_observada) × 0.40

Onde:
  N_amostras         = Número de tasks do agente no task_type
  variancia_observada = Variância dos erros de predição anteriores
                       (0.0 = baixa variância, 1.0 = alta variância)

  Para agente sem histórico: confianca_predicao = 0.30 (neutro baixo)
  Para agente com 20+ amostras: confianca_predicao = 0.60 + ajuste de variância
```

---

## 6. Cálculo de Risco e Bandas

### 6.1 Risco Score

O risco é o complemento da probabilidade de sucesso, ajustado pela confiança da predição:

```
risco_score = (1 - P(success)) × (1 + (1 - confianca_predicao) × 0.5)

Onde:
  O fator (1 - confianca_predicao) × 0.5 adiciona um "prêmio de incerteza":
  - Se confiança na predição é baixa, o risco aumenta
  - Se confiança na predição é alta, o risco é apenas 1 - P(success)

EXEMPLO — cosca-testing race-detection:

  P(success)        = 0.80
  confianca_predicao = 0.72  (4 amostras, variância baixa)

  risco_score = (1 - 0.80) × (1 + (1 - 0.72) × 0.5)
              = 0.20 × (1 + 0.28 × 0.5)
              = 0.20 × (1 + 0.14)
              = 0.20 × 1.14
              = 0.228
```

### 6.2 Bandas de Risco

```
risco_score < 0.15  → 🟢 BAIXO    (P(success) > 85%)
risco_score 0.15-0.30 → 🟡 MODERADO (P(success) 70-85%)
risco_score > 0.30  → 🔴 ALTO     (P(success) < 70%)

┌─────────────────────────────────────────────────────────────────┐
│                    BANDAS DE RISCO                               │
│                                                                  │
│  🟢 BAIXO (risco < 0.15)                                        │
│  ├── P(success) > 85%                                           │
│  ├── Ação: Delegação autônoma                                   │
│  └── Exemplo: cosca-architecture em specification (93%)         │
│                                                                  │
│  🟡 MODERADO (risco 0.15-0.30)                                  │
│  ├── P(success) 70-85%                                          │
│  ├── Ação: Delegação com supervisão                              │
│  └── Exemplo: cosca-testing em race-detection (80%)             │
│                                                                  │
│  🔴 ALTO (risco > 0.30)                                         │
│  ├── P(success) < 70%                                           │
│  ├── Ação: Abrir Contrafactual Gate (F1.2)                       │
│  └── Exemplo: cosca-qa em testing específico (65%)              │
└─────────────────────────────────────────────────────────────────┘
```

### 6.3 Indicadores Visuais

O output do engine inclui um indicador visual de barras para comunicação rápida:

```
P(success):  80%  ████████████████████░░░░░░  🟡
Tempo est.:  19m  ██████████████████░░░░░░░░░
Custo est.:  $46  ████████████████████░░░░░░░
Risco:       🟡 MODERADO  [████████░░░░░░░░░░]  (0.23)
Confiança:   72%  [██████████████░░░░░░░░░░░░]

Fatores Críticos:
  success_rate    = 0.40  +++  Histórico perfeito
  domain_strength = 0.12  ---  Domínio race-detection é novo para o agente
  recency         = 0.19  +++  Tasks recentes, dados frescos
  complexity      = 0.08  ++   Complexidade 3, agente lida bem
  confidence      = 0.06  ++   Confiança moderada

Recomendação: DELEGAR COM REVISÃO — domain_strength baixo requer supervisão
```

---

## 7. Calibragem Contínua

### 7.1 Ciclo de Calibragem

A calibragem é o mecanismo que torna o Prediction Engine adaptativo. Sem ela, as fórmulas seriam estáticas e os erros se acumulariam.

```
A cada 10 PREDIÇÕES:
  ┌────────────────────────────────────────────────────────────┐
  │ 1. COLETAR: Últimas 10 predições com seus outcomes reais  │
  │ 2. CALCULAR: Erro médio por dimensão (tempo, custo, P)    │
  │ 3. AJUSTAR: Pesos dos fatores na fórmula P(success)       │
  │ 4. APRENDER: Registrar padrões de erro por agente/domínio │
  │ 5. REPETIR: Recomeçar o ciclo                             │
  └────────────────────────────────────────────────────────────┘
```

### 7.2 Algoritmo de Recalibragem

```python
# Pseudocódigo da recalibragem (executado a cada 10 predições)

def recalibrar(predicoes, outcomes):
    """
    predicoes: lista das últimas 10 predições
    outcomes: lista dos 10 outcomes reais correspondentes
    """
    erros = []

    for pred, real in zip(predicoes, outcomes):
        erro_tempo = abs(real.tempo - pred.tempo_estimado) / pred.tempo_estimado
        erro_custo = abs(real.custo - pred.custo_estimado) / pred.custo_estimado
        erro_p = abs(real.outcome_binario - pred.p_success)
        erros.append({
            'erro_tempo': erro_tempo,
            'erro_custo': erro_custo,
            'erro_p': erro_p,
            'agente': pred.agent,
            'task_type': pred.task_type
        })

    erro_medio_tempo = mean([e.erro_tempo for e in erros])
    erro_medio_custo = mean([e.erro_custo for e in erros])
    erro_medio_p = mean([e.erro_p for e in erros])

    # Regras de ajuste
    if erro_medio_p > 0.30:
        # Erro médio alto: recalibrar weights
        # Fatores com maior erro contribuído são reduzidos
        ajustar_pesos(erros)

    if erro_medio_tempo > 0.25:
        # Ajustar complexity_multiplier de tempo
        ajustar_tempo_multiplier(erro_medio_tempo)

    if erro_medio_custo > 0.25:
        # Ajustar complexity_multiplier de custo
        ajustar_custo_multiplier(erro_medio_custo)

    # Aprendizado estrutural
    for agente in set(e.agente for e in erros):
        erros_agente = [e for e in erros if e.agente == agente]
        erro_medio_agente = mean([e.erro_p for e in erros_agente])

        if erro_medio_agente > 0.30:
            reduzir_confidence(agente, 0.10)
        elif erro_medio_agente < 0.10:
            aumentar_peso_agente(agente, 0.05)
```

### 7.3 Regras de Calibragem

| Condição | Ação | Gatilho |
|----------|------|---------|
| Erro médio de P(success) > 30% para um agente | Reduz confidence do agente em **-0.10** | A cada 10 predições |
| Erro médio de P(success) < 10% para um agente | Aumenta weight do agente em **+0.05** | A cada 10 predições |
| Erro médio de tempo > 25% | Ajusta complexity_multiplier de tempo em +0.05 | A cada 10 predições |
| Erro médio de custo > 25% | Ajusta complexity_multiplier de custo em +0.05 | A cada 10 predições |
| Erro consistente em um domínio específico | Registra pattern learning: "domínio X é sistematicamente subestimado" | Imediato |
| Agente com < 3 amostras | Não recalibra (dados insuficientes) | Automático |

### 7.4 Exemplo de Calibragem

```
CENÁRIO: 10 predições para cosca-testing

  Pred_01: P=0.80, real=success   → erro: 0.20
  Pred_02: P=0.85, real=success   → erro: 0.15
  Pred_03: P=0.75, real=success   → erro: 0.25
  Pred_04: P=0.82, real=success   → erro: 0.18
  Pred_05: P=0.70, real=failure   → erro: 0.30
  Pred_06: P=0.78, real=success   → erro: 0.22
  Pred_07: P=0.85, real=success   → erro: 0.15
  Pred_08: P=0.80, real=success   → erro: 0.20
  Pred_09: P=0.72, real=partial   → erro: 0.28
  Pred_10: P=0.83, real=success   → erro: 0.17

  Erro médio: 0.21 (21%)
  ───
  Resultado: 0.21 < 0.30 → NENHUMA AÇÃO em confidence
  0.21 > 0.10 → NENHUM reforço positivo
  Manter. Monitorar próximas 10.

  Padrão detectado: Erro maior em complexity=4+ tasks
  → Registrar learning: "cosca-testing tem desempenho abaixo da média
    em complexidade 4+. Domain_strength não captura este efeito."
  → Sugestão: Adicionar fator 'max_complexity' no modelo.
```

### 7.5 Versionamento de Pesos

Os pesos do modelo são versionados para permitir rollback e auditoria:

```yaml
model_version:
  id: "PE-MODEL-v1.0.0"
  data: "2026-07-30"
  pesos:
    success_rate:    0.40
    domain_strength: 0.25
    recency:         0.20
    complexity:      0.10
    confidence:      0.05
  complexity_multipliers:
    tempo: { base: 1.0, per_level: 0.2 }
    custo: { base: 1.0, per_level: 0.3 }
  domain_strength_adjustments:
    threshold_alto:  0.90
    threshold_baixo: 0.40
  calibration_history:
    - version: "v1.0.0"
      data: "2026-07-30"
      razao: "Versão inicial do Prediction Engine"
      amostras: 0
```

---

## 8. Integração com F1.2 — Contrafactual Gate

### 8.1 Gatilho de Ativação

O Contrafactual Gate (F1.2 / G0.5) é ativado pelo Prediction Engine quando:

```
SE P(success) do melhor agente candidato < 70% (🔴 ALTO):
  ├── Abrir Contrafactual Gate
  ├── Gate analisa: "E se delegarmos mesmo assim vs escalonarmos?"
  ├── Gate consulta: alternativas (outro agente, abordagem diferente)
  └── Gate retorna: recomendação final para o Kernel

SE P(success) >= 70% (🟢 ou 🟡):
  └── Contrafactual Gate NÃO é ativado (decisão direta do Kernel)
```

### 8.2 Pipeline Prediction → Contrafactual

```
PREDICTION ENGINE                          CONTRAFACTUAL GATE (F1.2)
┌─────────────────────┐                   ┌──────────────────────────┐
│ P(success) = 65%   │─── abre gate ────▶│ Analisar:                │
│ Risco = 🔴 ALTO    │                   │  ├── Cenário A: Delegar  │
│ Agente: cosca-qa   │                   │  │   mesmo assim (65%)   │
│ Task: testing      │                   │  ├── Cenário B: Escalonar│
│ domain: race-det   │                   │  │   para cosca-testing   │
└─────────────────────┘                   │  │   (85% se disponível) │
                                          │  ├── Cenário C: Kernel  │
                                          │  │   executa diretamente │
                                          │  └── Saída:              │
                                          │      recommendation +   │
                                          │      contrafactual.yaml │
                                          └──────────┬───────────────┘
                                                     │
                                                     ▼
                                          ┌──────────────────────────┐
                                          │ KERNEL DECIDE:           │
                                          │  ├── Aceita recomendação │
                                          │  └── Override (Don)      │
                                          └──────────────────────────┘
```

### 8.3 Informações Transmitidas ao Gate

O Prediction Engine envia ao Contrafactual Gate:

```yaml
contrafactual_input:
  agente_primario: "cosca-qa"
  p_success: 0.65
  task_type: "testing"
  complexity: 4
  domain: "race-detection"
  fatores_criticos:
    - fator: "domain_strength"
      valor: 0.55
      impacto: "Baixo domínio em quality-assurance para race-detection"
  alternativas_disponiveis:
    - agente: "cosca-testing"
      p_success_estimado: 0.85
      disponivel: true
      custo_adicional: "$0.010"
      tempo_adicional: "5m"
    - agente: "cosca-kernel"
      p_success_estimado: 0.78
      disponivel: true
      custo_adicional: "$0.040"
      tempo_adicional: "10m"
  urgencia: "high"
  custo_estimado: "$0.040"
```

### 8.4 Regra de Override do Gate

```
Se Contrafactual Gate recomendar ESCALONAR:
  ├── Kernel acata a menos que:
  │    ├── Don explicitamente escolheu este agente
  │    └── Única opção disponível no momento

Se Contrafactual Gate recomendar DELEGAR MESMO ASSIM:
  ├── Kernel delega com supervision_level = maximum
  ├── Quality Gate G1 (Pre-Commit) reforçado
  └── Don notificado no pós-task
```

---

## 9. Integração com F2.1 — Cognitive Economy

### 9.1 Custo Estimado Alimenta o ROI da Cognitive Economy

O Prediction Engine fornece a **estimativa de custo** que a Cognitive Economy Engine (F2.1) usa para calcular o ROI cognitivo:

```
PREDICTION ENGINE                          COGNITIVE ECONOMY (F2.1)
┌─────────────────────┐                   ┌──────────────────────────┐
│ Custo est. = $0.040 │─── alimenta ─────▶│ Cost Vector:             │
│ Tempo est. = 19m    │                   │  ├── tokens: $0.040      │
│ P(success) = 80%    │                   │  ├── time: 19min         │
│ Risco = 🟡 moderado  │                   │  ├── compute: estimado   │
└─────────────────────┘                   │  ├── attention: 0        │
                                          │  └── storage: 1          │
                                          │                          │
                                          │ Value Vector +           │
                                          │ Efficiency Score         │
                                          │                          │
                                          │ Output: "VALE A PENA?    │
                                          │   ROI: 6.2×"            │
                                          └──────────────────────────┘
```

### 9.2 Resposta para a Cognitive Economy

O Prediction Engine responde à pergunta da Cognitive Economy:

```
PERGUNTA da Cognitive Economy:
  "Vale a pena gastar $0.040 para esta task?"

RESPOSTA do Prediction Engine:
  ├── Custo estimado: $0.040
  ├── P(success): 80%
  ├── Custo esperado ajustado pelo risco: $0.040 / 0.80 = $0.050
  ├── Risco de retrabalho: 20% chance de custar +$0.040 adicional
  ├── Custo total esperado (com retrabalho): $0.040 + 0.20 × $0.040 = $0.048
  │
  └── VEREDITO: "Custo esperado de $0.048 é compatível com tasks similares.
       A Cognitive Economy deve comparar com o valor esperado da task.
       Se value_estimado > $0.048 → VALE A PENA. Caso contrário → DEFER."
```

### 9.3 Integração no Pipeline KERNEL.md

```
KERNEL.md PIPELINE:
  Step 5: Request Analysis → task_type, complexity, domain
  Step 6: Capability Resolution → agentes candidatos
       │
       ▼
  ┌─────────────────────────────────────────────────────────────┐
  │ ★ PREDICTION ENGINE (F7.1) ★                                │
  │  ├── Consulta Trust Registry (F7.2)                          │
  │  ├── Calcula P(success), tempo, custo, risco                │
  │  └── Output para Cognitive Economy (F2.1) + Kernel          │
  │                                                              │
  │  ★ COGNITIVE ECONOMY ENGINE (F2.1) ★                         │
  │  ├── Recebe custo_estimado do Prediction Engine             │
  │  ├── Adiciona outras 4 dimensões de custo                   │
  │  ├── Calcula efficiency_score                                │
  │  └── Output: adjusted_plan + efficiency_score               │
  └──────────────────────────────────────────────────────────────┘
       │
       ▼
  Step 7: Planning & DAG Generation (usa adjusted_plan)
       │
       ▼
  Step 8-13: Execution → Review → Quality → Storage
       │
       ▼
  ┌─────────────────────────────────────────────────────────────┐
  │ ★ PÓS-TASK: FEEDBACK LOOP ★                                 │
  │  ├── Kernel registra outcome real no Trust Registry         │
  │  ├── Prediction Engine calibra modelo                       │
  │  └── Cognitive Economy atualiza ROI real                   │
  └──────────────────────────────────────────────────────────────┘
```

### 9.4 Modos de Operação Conjunta

| Modo | Prediction Engine | Cognitive Economy | Uso |
|------|------------------|-------------------|-----|
| **Full** | Ativo (pré + pós) | Ativo (cost × value) | Tasks P0/P1 |
| **Standard** | Ativo (pré + pós) | Apenas cost tracking | Tasks P2/P3 |
| **Light** | Apenas P(success) | Inativo | Tasks rotineiras |
| **Skip** | Inativo | Inativo | Commands diretos do Don |

---

## 10. Integração com Meta-Cognition Pipeline (F0.6)

### 10.1 Predição como Stage 7

A predição e sua calibragem são integradas ao Meta-cognition Pipeline como parte do **Stage 7 (REFLECT)** e **Stage 8 (STORE)**:

```
META-COGNITION PIPELINE (F0.6):

  Stage 1-6: Execução normal da task
       │
       ▼
  Stage 7: REFLECT
    ├── "Minha predição para esta task foi precisa?"
    ├── "O erro de predição foi dentro do aceitável?"
    ├── "Que padrão posso extrair do desvio (se houve)?"
    ├── "O Trust Registry está atualizado com o outcome?"
    └── Output: reflection_record com prediction_accuracy
       │
       ▼
  Stage 8: STORE
    ├── Registrar predição + outcome no prediction_log
    ├── Atualizar métricas de calibração do engine
    ├── Se erro > 20%, registrar learning de calibragem
    └── Output: TRUST_REGISTRY.md atualizado
```

### 10.2 Formato do Reflection Record

```yaml
reflection_record:
  stage: 7
  type: prediction_accuracy
  predicao_id: "PRED-2026-07-30-001"
  task_id: "race-condition-predicted-2026-07-30-001"
  acuracia_tempo: 0.91       # 1 - |erro|
  acuracia_custo: 0.91
  acuracia_p: 0.85           # 1 - |predito - real_binario|
  acuracia_geral: 0.89       # média das 3
  dentro_aceitavel: true     # erro < 20%
  veredito: "✅ BOA PREDIÇÃO — todas as métricas dentro do aceitável"
  learning_extraido: "Para testing/race-detection, o modelo tende a superestimar
                      custo em 9%. Ajuste fino aplicado na recalibragem."
```

---

## 11. Exemplo com Dados Reais do Trust Registry

### 11.1 Task: Corrigir Race Conditions no Runtime

**Contexto real** (extraído de `internal/embed/cosca/memory/trust/TRUST_REGISTRY.md`):

```
Task: "corrigir race conditions no runtime"
Task ID: race-condition-fixes-2026-07-30-002
Agente candidato: cosca-testing
Task Type: testing
Complexity: 3
Domain: race-detection
```

**Dados do Trust Registry para cosca-testing:**

```yaml
success_rate:
  last_20: 1.00    # 4/4 tasks
  total: 1.00
  total_tasks: 4

avg_latency:
  by_task_type:
    testing: 13m45s   # (12m30s + 15m00s) / 2

avg_cost:
  by_task_type:
    testing: $0.025    # ($0.022 + $0.028) / 2

domain_strength:
  race-detection: 0.48

reliability_score: 0.84
confidence_after: 0.60
```

**Predição:**

```yaml
prediction:
  id: "PRED-2026-07-30-RACE-001"

  # INPUTS
  agent: cosca-testing
  task_type: testing
  complexity: 3
  domain: race-detection

  # FÓRMULA P(success)
  fatores:
    success_rate:     1.00 × 0.40 = 0.400
    domain_strength:  0.48 × 0.25 = 0.120
    recency:          1.00 × 0.20 = 0.200   # última task hoje
    complexity:       0.60 × 0.10 = 0.060   # avg level 3.0 / 5.0
    confidence:       0.60 × 0.05 = 0.030
  p_success: min(0.810, 1.0) = 0.81

  # AJUSTE POR DOMAIN_STRENGTH (0.48 < 0.50 → penalidade)
  domain_adjustment: -0.03 direct
  p_success_ajustado: 0.78

  # CÁLCULO DE RISCO
  confianca_predicao: 0.72   # 4 amostras, variância baixa
  risco_score: (1 - 0.78) × (1 + (1 - 0.72) × 0.5)
             = 0.22 × (1 + 0.14)
             = 0.251
  risco_banda: 🟡 MODERADO

  # TEMPO ESTIMADO
  tempo_base: 13m45s (825s)
  complexity_mult: 1 + 0.2 × (3 - 1) = 1.4
  tempo_est: 825s × 1.4 = 1155s = 19m15s
  domain_adjustment: +10% (domain_strength < 0.50)
  tempo_est_final: 19m15s + 1m55s = ~21m

  # CUSTO ESTIMADO
  custo_base: $0.025
  complexity_mult: 1 + 0.3 × (3 - 1) = 1.6
  custo_est: $0.025 × 1.6 = $0.040
  domain_adjustment: +15% (domain_strength < 0.50)
  custo_est_final: $0.040 + $0.006 = $0.046

  # OUTPUT CONSOLIDADO
  outputs:
    p_success: 78%
    tempo_estimado: "~21min"
    custo_estimado: "$0.046"
    risco: "🟡 MODERADO (score: 0.25)"
    confianca_predicao: "72% (4 amostras)"
    recomendacao: "delegar_com_revisao — domain_strength baixo requer supervisão"
```

**Resultado Real:**

```yaml
outcome:
  task_id: "race-condition-fixes-2026-07-30-002"
  result: success
  tempo_real: 15m00s
  custo_real: $0.028
  confidence_delta: +0.08
  files_changed: 6
  loc_delta: +350
```

**Comparação:**

```yaml
comparacao:
  ┌─────────────────────┬────────────┬──────────┬────────┐
  │ Métrica             │ Previsto   │ Real     │ Erro   │
  ├─────────────────────┼────────────┼──────────┼────────┤
  │ P(success)          │ 78%        │ 100%     │ +22%   │
  │ Tempo               │ 21min      │ 15min    │ -29%   │
  │ Custo               │ $0.046     │ $0.028   │ -39%   │
  └─────────────────────┴────────────┴──────────┴────────┘

  ANÁLISE:
  ├── P(success): Real melhor que previsto (agente superou expectativa)
  ├── Tempo: Erro de 29% — acima do threshold de 20%
  ├── Custo: Erro de 39% — acima do threshold de 20%
  └── Veredito: Predição conservadora (subestimou o agente).
      Domain_strength baixo (0.48) penalizou demais a estimativa.
      O agente tem 100% success_rate em testing, o que deveria
      ter mais peso contra o domain_strength baixo.

  CALIBRAGEM:
  ├── Erro médio de P(success) para cosca-testing: 22% (1 predição)
  ├── Dados insuficientes para recalibragem (< 10 predições)
  └── Learning registrado: "domain_strength para race-detection
      pode estar subestimado. cosca-testing teve 2/2 sucessos
      em race-detection. Sugerir revisão do domain_strength
      de 0.48 para 0.60."
```

### 11.2 Task: Auditar Cobertura de Compute Fabric (Cross-Audit)

**Contexto real:**

```
Task: "auditar cobertura de testes do compute fabric"
Task ID: compute-fabric-coverage-2026-07-30-001
Agente candidato: cosca-qa
Task Type: quality-assurance
Complexity: 2
Domain: coverage-analysis
```

**Dados do Trust Registry para cosca-qa:**

```yaml
success_rate: 1.00 (1/1 task registrada)
avg_latency quality-assurance: 8m30s
avg_cost quality-assurance: $0.010
domain_strength coverage-analysis: 0.55
reliability_score: 0.72
confidence_after: 0.55
```

**Predição:**

```yaml
prediction:
  agent: cosca-qa
  p_success: 0.40 × 1.00 + 0.25 × 0.55 + 0.20 × 1.0 + 0.10 × 0.40 + 0.05 × 0.55
           = 0.400 + 0.138 + 0.200 + 0.040 + 0.028
           = 0.806
  ajuste_domain: 0 (0.55 está na faixa neutra 0.40-0.79)
  p_success_ajustado: 0.81

  risco: 🟡 MODERADO (score: 0.19)
  tempo_est: 8m30s × 1.2 = 10m12s → ~10min
  custo_est: $0.010 × 1.3 = $0.013

  recomendacao: "delegar_com_revisao — agente com apenas 1 task registrada"
```

**Resultado Real:**

```yaml
outcome:
  result: success
  tempo_real: 8m30s
  custo_real: $0.010
  files_changed: 3
  loc_delta: +82
```

**Comparação e Aprendizado:**

```yaml
analise:
  Erro tempo: 20% (no limite — aceitável)
  Erro custo: 23% (acima do threshold)
  Learning: "cosca-qa tende a ser mais eficiente que a média para
            quality-assurance. Considerar avg_latency menor para este agente."
```

### 11.3 Summary das Predições Iniciais

```yaml
prediction_summary:
  data: "2026-07-30"
  total_predicoes: 2
  acuracia_media:
    p_success: 0.78   # média dos erros de P(success)
    tempo: 0.75       # média 1 - |erro_tempo|
    custo: 0.69       # média 1 - |erro_custo|
  dentro_aceitavel: 1  # 1 de 2 dentro do threshold de 20%
  aprendizado_inical: "Domain_strength parece ser o fator mais instável.
                       Precisa de mais dados para calibrar seu peso relativo."
  status: "Modelo em calibração inicial. Confiança geral: 0.50 (poucas amostras)."
```

---

## 12. Diagrama de Arquitetura

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                          PREDICTION ENGINE (F7.1)                                │
│                          ═══════════════════════                                 │
│                                                                                  │
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │                           INPUT LAYER                                     │   │
│  │                                                                           │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │   │
│  │  │  Agent   │  │Task Type │  │Complexity │  │  Domain  │  │ Urgency  │   │   │
│  │  │candidato │  │testing   │  │   1-5     │  │race-det  │  │ high     │   │   │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘   │   │
│  │       └──────────────┴─────────────┴─────────────┴───────────┘          │   │
│  └──────────────────────────────────┬──────────────────────────────────────┘   │
│                                     │                                           │
│                                     ▼                                           │
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │                         TRUST REGISTRY CONSULTA                           │   │
│  │                                                                           │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │   │
│  │  │ Success  │  │   Avg    │  │   Avg    │  │  Domain  │  │Confidence│   │   │
│  │  │  Rate    │  │ Latency  │  │  Cost    │  │ Strength │  │  Current │   │   │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘   │   │
│  │       └──────────────┴─────────────┴─────────────┴───────────┘          │   │
│  └──────────────────────────────────┬──────────────────────────────────────┘   │
│                                     │                                           │
│                                     ▼                                           │
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │                         CALCULATION ENGINE                                │   │
│  │                                                                           │   │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐          │   │
│  │  │   P(success)    │  │  Tempo Estimado │  │  Custo Estimado │          │   │
│  │  │                 │  │                 │  │                 │          │   │
│  │  │ Σ(weight×factor)│  │ avg_latency ×   │  │ avg_cost ×      │          │   │
│  │  │ 5 fatores      │  │ complexity_mult │  │ complexity_mult │          │   │
│  │  └───────┬─────────┘  └────────┬────────┘  └────────┬────────┘          │   │
│  │          │                     │                     │                    │   │
│  │          └─────────────────────┴─────────────────────┘                    │   │
│  │                                     │                                     │   │
│  │                            ┌────────▼────────┐                            │   │
│  │                            │  Risk Score     │                            │   │
│  │                            │  = 1 - P(succ)  │                            │   │
│  │                            │  × uncertainty  │                            │   │
│  │                            └────────┬────────┘                            │   │
│  └──────────────────────────────────────┬────────────────────────────────────┘   │
│                                         │                                         │
│                                         ▼                                         │
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │                           OUTPUT LAYER                                    │   │
│  │                                                                           │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │   │
│  │  │P(success)│  │  Tempo   │  │  Custo   │  │  Risco   │  │Recomenda-│   │   │
│  │  │   78%    │  │  ~21min  │  │  $0.046  │  │🟡 moderado│  │   ção    │   │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │   │
│  └──────────────────────────────────┬──────────────────────────────────────┘   │
│                                     │                                           │
└─────────────────────────────────────┼───────────────────────────────────────────┘
                                      │
            ┌─────────────────────────┼──────────────────────────┐
            │                         │                          │
            ▼                         ▼                          ▼
  ┌──────────────────┐    ┌──────────────────┐    ┌──────────────────────┐
  │    KERNEL        │    │ Contrafactual    │    │   Cognitive Economy  │
  │  Decide ação     │    │ Gate (F1.2)      │    │   Engine (F2.1)      │
  │  baseado na      │◄───│ Se risco 🔴      │    │   Custo estimado     │
  │  predição        │    │ P(success) < 70% │    │   → ROI calculation  │
  └──────────────────┘    └──────────────────┘    └──────────────────────┘
                                      │
                                      ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │                         POST-TASK LAYER                              │
  │                                                                      │
  │  ┌──────────────────┐    ┌──────────────────┐    ┌────────────────┐ │
  │  │   Trust Registry │    │   Comparação     │    │  Calibration   │ │
  │  │  Outcome real    │───▶│  Previsto vs Real│───▶│  Ajuste pesos  │ │
  │  │  é registrado    │    │  Delta calculado │    │  Learning loop │ │
  │  └──────────────────┘    └──────────────────┘    └────────────────┘ │
  └──────────────────────────────────────────────────────────────────────┘
```

---

## 13. Gatilhos de Ativação

### 13.1 Ativação Automática

O Prediction Engine é ativado automaticamente em **TODAS** as tasks, mas com intensidade variável:

| Nível de Ativação | Gatilho | Ação |
|:-----------------:|---------|------|
| **Completo** | Task P0/P1 | Pré-task completo + pós-task completo + calibragem |
| **Standard** | Task P2/P3 | Pré-task completo + pós-task completo |
| **Leve** | Task rotineira | Apenas P(success) para fila de prioridade |
| **Mínimo** | Don command | Apenas registro pós-task para histórico |

### 13.2 Ativação Manual (Don)

O Don (ou Kernel) pode solicitar predição explicitamente:

```
Comando: "cosca predict --agent cosca-testing --task-type testing"
         "Preveja o resultado desta task antes de executar."

Resposta: Prediction Engine calcula e retorna predição imediata.
```

### 13.3 Thresholds Configuráveis

```yaml
activation_thresholds:
  contrafactual_gate:
    p_success_max: 0.70    # Se P(success) < 70%, abre Gate
  delegacao_autonoma:
    p_success_min: 0.85    # Se P(success) >= 85%, delegação autônoma
  alerta_don:
    risco_max: 0.40         # Se risco > 0.40, alerta Don
  erro_aceitavel:
    max: 0.20               # Erro > 20% dispara calibragem
  calibragem:
    a_cada_n: 10            # Recalibra a cada 10 predições
```

---

## 14. Regras de Decisão

### 14.1 O Motor Recomenda, o Kernel Decide

```
PRINCÍPIO FUNDAMENTAL:

  O Prediction Engine é um motor de RECOMENDAÇÃO, não de DECISÃO.
  A decisão final é sempre do Kernel ou do Don.

  Fluxo correto:
    Engine recomenda → Kernel avalia → Kernel decide

  Fluxo proibido:
    Engine decide → Kernel executa (autopilot sem supervisão)
```

### 14.2 Matriz de Decisão

```
┌───────────────────┬────────────────────┬──────────────────────────┐
│ P(success) / Risco│ Recomendação       │ Ação do Kernel           │
├───────────────────┼────────────────────┼──────────────────────────┤
│ ≥ 85% / 🟢        │ Delegar autônomo   │ Delega sem revisão       │
│ 70-84% / 🟡       │ Delegar com revisão│ Delega + supervisão leve │
│ 50-69% / 🔴       │ Abrir Gate (F1.2)  │ Contrafactual decide     │
│ < 50% / 🔴🔴      │ Escalonar          │ Kernel executa ou Don    │
└───────────────────┴────────────────────┴──────────────────────────┘

REGRAS ADICIONAIS:
  ├── Se confidence_predicao < 0.40: tratar como "dados insuficientes"
  │   → Recomendar revisão manual independente do P(success)
  │
  ├── Se agente tem < 3 tasks registradas no task_type:
  │   → Aplicar penalidade de -0.10 em P(success) (poucos dados)
  │
  ├── Se domain_strength para o domínio é 0:
  │   → Usar domain_strength médio do agente (fallback)
  │
  └── Se erro médio do agente nas últimas 10 predições > 30%:
      → Reduzir confidence do agente em 0.10 automaticamente
```

### 14.3 Override do Don

O Don pode **override** qualquer predição:

```
Don: "Execute mesmo com P(success) de 40%. Eu autorizo."
  → Kernel registra: override_don = true
  → Prediction Engine registra: predição mantida, decisão sobrescrita
  → Contrafactual Gate: NÃO aberto (Don override > Gate)
  → Trust Registry: task registrada com tag 'don-override'
  → Calibragem: esta predição NÃO conta para o erro médio do modelo
    (não foi uma decisão do modelo, foi uma decisão do Don)
```

### 14.4 Leveza e Performance

```
REGRAS DE PERFORMANCE:
  ├── Consulta ao Trust Registry: dados em cache (JSON parseado em memória)
  ├── Cálculo: aritmética simples, sem loops ou consultas externas
  ├── Tempo máximo: 500ms (tipicamente < 300ms)
  ├── Sem chamadas de LLM: Prediction Engine é determinístico
  │
  ├── Se Trust Registry não estiver disponível:
  │     ├── Usar valores default (success_rate=0.50, domain_strength=0.50)
  │     └── confianca_predicao = 0.20 (baixíssima confiança)
  │
  └── Se cache do Trust Registry estiver stale (> 5 minutos):
        └── Recarregar do arquivo YAML (operação de I/O leve)
```

---

## 15. Métricas do Próprio Engine

### 15.1 Métricas de Saúde

```yaml
engine_health_metrics:
  total_predicoes: 0
  predicoes_completas: 0     # Com pós-task registrado
  predicoes_sem_outcome: 0   # Task executada mas sem outcome registrado

  acuracia_media:
    p_success: 0.0           # 1 - |erro_medio_p|
    tempo: 0.0               # 1 - |erro_medio_tempo|
    custo: 0.0                # 1 - |erro_medio_custo|

  calibragem:
    ultima: null              # Timestamp da última calibragem
    ciclos_completos: 0       # Ciclos de 10 predições completados
    ajustes_realizados: 0     # Quantas vezes os pesos foram ajustados

  performance:
    tempo_medio_consulta_ms: 0
    tempo_medio_calculo_ms: 0

  confianca_do_engine:
    geral: 0.3                # Começa baixo, sobe com amostras
    amostras_minimas: 20      # Mínimo para confiança > 0.7
```

### 15.2 Log de Predições

Cada predição é registrada em formato parseável:

```yaml
prediction_log:
  - id: "PRED-2026-07-30-RACE-001"
    timestamp: "2026-07-30T17:00:00Z"
    agent: "cosca-testing"
    task_type: "testing"
    complexity: 3
    domain: "race-detection"

    predicted:
      p_success: 0.81
      tempo: "19m15s"
      custo: "$0.046"
      risco: "🟡 MODERADO"

    actual:
      outcome: "success"
      tempo: "15m00s"
      custo: "$0.028"
      p_success_real: 1.0

    delta:
      p_success: +0.19
      tempo: -0.29
      custo: -0.39
      dentro_aceitavel: false   # tempo e custo > 20%

    calibration_action: "Registrar learning: domain_strength pode estar baixo"
```

---

## 16. Relacionados

| Documento | Relação |
|-----------|---------|
| [TRUST_REGISTRY.md](../../memory/trust/TRUST_REGISTRY.md) | **Fonte de dados** — success_rate, avg_latency, avg_cost, domain_strength |
| [DECISION_DNA.md](../../knowledge/architecture/DECISION_DNA.md) | **Integração pós-task** — predições alimentam outcome validation do DDNA |
| [contrafactual-gate.md](../../workflows/contrafactual-gate.md) | **Gate de escalação** — ativado quando P(success) < 70% |
| [cognitive-economy/SKILL.md](../cognitive-economy/SKILL.md) | **Consumidor de custo** — custo estimado alimenta ROI da Cognitive Economy |
| [next-evolution-phases.md](../../knowledge/architecture/next-evolution-phases.md) | **Roadmap** — F7.1 Engineering Intelligence (Prediction Engine) |
| [QUALITY_GATES.md](../../QUALITY_GATES.md) | **Gate G0** — Pre-Work inclui predição como etapa opcional |
| [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) | **Aprendizado** — learning de calibragem segue protocolo de aprendizado |
| [KERNEL.md](../../KERNEL.md) | **Orquestrador** — pipeline Step 5-6 → Prediction → Step 7 |
| [CONSTITUTION.md](../../CONSTITUTION.md) | **Governança** — Art. P2 (código executado é a verdade absoluta) |

---

## 17. HISTÓRICO

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | cosca-architecture | Criação inicial. Especificação completa do Prediction Engine: definição, pipeline pré/pós-task, fórmulas (P(success), tempo, custo, risco), calibragem contínua (a cada 10 predições), integração com Contrafactual Gate (F1.2 — ativado quando P(success) < 70%) e Cognitive Economy (F2.1 — custo estimado alimenta ROI), 2 exemplos com dados reais do Trust Registry (cosca-testing race-detection e cosca-qa coverage-audit), diagrama de arquitetura, métricas de saúde do engine. |

---

> *"O Prediction Engine não decide. Ele ilumina. A decisão é sempre do Kernel ou do Don — mas agora, informada por dados, não por intuição."*
> — Cosca Architecture Chief, 2026-07-30
