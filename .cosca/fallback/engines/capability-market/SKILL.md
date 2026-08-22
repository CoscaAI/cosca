# CAPABILITY MARKET ENGINE ★ F8.1 — Motor de Mercado de Capacidades

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-capability-market`
> **Conceito**: ★ ESTRELA DA FASE 8 — Capability Market
> **Referências**: next-evolution-phases.md §F8 | TRUST_REGISTRY.md | DECISION_DNA.md | contrafactual-gate.md
> **Dependências**: F7.1 (Prediction Engine) | F7.2 (Trust Registry) | F1.1 (DDNA) | F1.2 (Contrafactual Gate) | F2.1 (Cognitive Economy)
> **CMI Impact**: Julgamento +8, Planejamento +6, Aprendizado +5

---

## Índice

1. [O que é o Capability Market](#1-o-que-é-o-capability-market)
2. [Protocolo de Anúncio](#2-protocolo-de-anúncio)
3. [Mecanismo de Leilão](#3-mecanismo-de-leilão)
4. [Shadow Bids](#4-shadow-bids)
5. [Ranking e Pontuação](#5-ranking-e-pontuação)
6. [Integração com F7.1 — Prediction Engine](#6-integração-com-f71--prediction-engine)
7. [Integração com F1.2 — Contrafactual Gate](#7-integração-com-f12--contrafactual-gate)
8. [Integração com F2.1 — Cognitive Economy](#8-integração-com-f21--cognitive-economy)
9. [Integração com F1.1 — Decision DNA](#9-integração-com-f11--decision-dna)
10. [Aprendizado do Mercado](#10-aprendizado-do-mercado)
11. [Regras de Operação](#11-regras-de-operação)
12. [Exemplo com Dados Reais do Trust Registry](#12-exemplo-com-dados-reais-do-trust-registry)
13. [Diagrama de Arquitetura](#13-diagrama-de-arquitetura)
14. [Métricas do Engine](#14-métricas-do-engine)
15. [Relacionados](#15-relacionados)
16. [Histórico](#16-histórico)

---

## 1. O que é o Capability Market

### Definição

O **Capability Market** é o motor de **roteamento dinâmico de tasks** do Cosca. Cada agente **anuncia publicamente suas capacidades** (como ações em uma bolsa de valores), e o Kernel **leiloa cada task** para o melhor executor disponível no momento.

Não é mais o Kernel escalando manualmente — **o mercado decide**.

### Analogia: Bolsa de Valores

```
Bolsa de Valores (Financeira)   →   Capability Market (Cosca)
──────────────────────────────────────────────────────────────
Empresas listadas               →   Agentes
Ações (ticker + preço)          →   Capacidades (nome + confiança + custo)
Corretora                       →   Kernel
Investidor comprando            →   Task sendo delegada
Preço da ação (demanda/oferta)  →   Score do agente para aquela task
Dividendos                      →   Aprendizados e calibragem
IPO de nova empresa             →   Novo agente anunciando capacidades
Mercado futuro                  →   Shadow bids (simulação)
```

### Filosofia

```
"O Kernel não escala mais — o mercado escala.
 O Kernel não escolhe mais — o mercado ranqueia.
 O Kernel não confia mais — o mercado aprende."
 — Cosca Architecture Chief, 2026-07-30
```

### Por que um Mercado?

| Abordagem | Problema | Solução do Market |
|-----------|----------|-------------------|
| Kernel escala manualmente | Viés de confirmação, cansaço, repetição | Leilão impessoal, baseado em dados |
| Delegar sempre para o mesmo agente | Agente especialista superlotado, outros subutilizados | Distribuição por confiança × disponibilidade |
| Regra fixa de roteamento | Não adapta a mudanças de performance | Aprendizado contínuo do mercado |
| Don escolhe manualmente | Gargalo humano, decisão sem dados | Override permitido mas documentado |

### O que o Market NÃO é

- **Não é um escalonador de filas** — não gerencia concorrência ou prioridade de execução
- **Não é um orquestrador de workflows** — não decide a ordem de execução de tarefas
- **Não substitui o Kernel** — o Kernel continua sendo a autoridade final
- **Não substitui o Trust Registry** — o Market CONSULTA o Trust Registry, não substitui
- **Não é um sistema de "quem grita mais alto"** — agentes não competem por preço, competem por score

---

## 2. Protocolo de Anúncio

### 2.1 Formato do Anúncio

Cada agente anuncia suas capacidades no formato YAML. O anúncio é **público** e **versionado**:

```yaml
agent: cosca-testing
version: 1.0.0
last_active: 2026-07-30T15:30:00Z
announced_at: 2026-07-30T15:00:00Z

capabilities:
  - name: race-detection
    confidence: 0.94          # Auto-avaliado (lastreado no Trust Registry)
    avg_latency: 1.2s         # Tempo médio histórico
    avg_cost: $0.004          # Custo médio histórico
    success_rate: 98%         # Taxa de sucesso no domínio
    domain: testing
    max_complexity: 4         # Complexidade máxima que aceita (1-5)
    available_slots: 3        # Tasks simultâneas que pode aceitar
    current_load: 1           # Tasks em execução no momento

  - name: coverage-audit
    confidence: 0.88
    avg_latency: 3.0s
    avg_cost: $0.010
    success_rate: 92%
    domain: testing
    max_complexity: 3
    available_slots: 2
    current_load: 0

  - name: bug-reproduction
    confidence: 0.82
    avg_latency: 4.5s
    avg_cost: $0.015
    success_rate: 85%
    domain: testing
    max_complexity: 2
    available_slots: 1
    current_load: 1

availability:
  status: available            # available | busy | degraded | offline
  next_available_at: null      # Se busy, quando libera
  reason: null                 # Se degraded, qual motivo
```

### 2.2 Ciclo de Vida do Anúncio

```
BOOT → ANUNCIAR (registro inicial)
         │
         ├── ATIVO (anúncio válido, agente disponível)
         │     ├── A cada task completa → atualizar scores
         │     ├── A cada 15 minutos → heartbeat (last_active)
         │     └── Se confidence mudar > 0.05 → re-anúncio
         │
         ├── DEGRADADO (anúncio válido, disponibilidade reduzida)
         │     └── agent informa: reason, estimated_recovery
         │
         ├── BUSY (anúncio válido, agente ocupado)
         │     └── available_slots = 0
         │
         └── OFFLINE (anúncio removido ou expirado)
               └── Após 30 min sem heartbeat → removido do market
```

### 2.3 Heartbeat

Os agentes enviam heartbeat periódico para manter o anúncio ativo:

```yaml
heartbeat:
  agent: cosca-testing
  timestamp: 2026-07-30T15:30:00Z
  current_load: 1
  available_slots: 2
  status: available
  last_task_id: race-condition-fixes-2026-07-30-002
  last_task_outcome: success
```

### 2.4 Validação do Anúncio

O Market **valida** cada anúncio contra o Trust Registry:

| Campo do Anúncio | Validação Contra Trust Registry | Regra |
|-------------------|-------------------------------|-------|
| `confidence` | `confidence_after` do agente | Não pode diferir > 0.10 do TR |
| `success_rate` | `success_rate.total` do agente | Não pode diferir > 5% do TR |
| `avg_latency` | `avg_latency.overall` do agente | Não pode ser < 50% do TR (subestimativa) |
| `avg_cost` | `avg_cost.overall` do agente | Não pode ser < 50% do TR (subestimativa) |

Se o anúncio estiver fora dos limites, o Market:
1. **Rejeita** o anúncio com erro específico
2. **Notifica** o agente para corrigir
3. **Registra** tentativa de overstatement (para calibragem futura)

### 2.5 Overstatement Detectado

Se um agente consistentemente anuncia valores melhores que seus dados históricos:

```
1a tentativa: aviso ao agente
2a tentativa: anúncio rejeitado + confiança penalizada em -0.05
3a tentativa: agente marcado como "unreliable" — anúncios ignorados
             até correção manual do Don
```

---

## 3. Mecanismo de Leilão

### 3.1 Visão Geral do Leilão

Quando uma task chega no Kernel, o Capability Market executa um **leilão de primeiro-score**:

```
┌──────────────────────────────────────────────────────────────────┐
│                      LEILÃO DE CAPACIDADES                        │
│                                                                  │
│  1. Kernel define: task_type, complexity, domain, max_cost       │
│     └── Envia requisição ao Market                               │
│                                                                  │
│  2. Market identifica agentes com capability compatível          │
│     └── Filtro por: domain match + max_complexity >= complexity  │
│                                                                  │
│  3. Market consulta Prediction Engine para cada candidato        │
│     └── P(success), tempo_est, custo_est, risco_score            │
│                                                                  │
│  4. Cada agente retorna bid (se opt-in)                          │
│     └── Confirma disponibilidade + estimativa                    │
│                                                                  │
│  5. Market calcula score composto para cada candidato            │
│     └── Fórmula: confidence × efficiency × cost_factor × risk    │
│                                                                  │
│  6. Market ranqueia top 3                                        │
│     └── Recomendação ao Kernel                                   │
│                                                                  │
│  7. Kernel escolhe (ou Gate abre para P0)                        │
│     └── Decisão final registrada                                 │
│                                                                  │
│  8. Perdedores registram shadow bids                             │
│     └── "Eu teria feito assim" — dados para calibragem           │
└──────────────────────────────────────────────────────────────────┘
```

### 3.2 Passo a Passo Detalhado

#### Passo 1: Kernel Define Parâmetros

O Kernel prepara os parâmetros da task:

```yaml
task:
  id: "audit-runtime-security-2026-07-30-001"
  task_type: "security-audit"
  complexity: 3
  domain: "security-architecture"
  max_cost_usd: 0.050
  max_latency: "30m"
  priority: "P1"
  description: "Auditar segurança do runtime — analisar threat model, dependências e configurações"
```

#### Passo 2: Market Filtra Candidatos

O Market consulta o catálogo de capacidades ativas para encontrar agentes compatíveis:

```yaml
filtro_aplicado:
  domínio_match: "security-audit"
  max_complexity >= 3
  status: available
  available_slots > 0
  confiança_mínima: 0.50

candidatos_encontrados: 3
  - cosca-security    (domain_strength: 0.88, confidence: 0.88)
  - cosca-architecture (domain_strength: 0.93, confidence: 0.93)
  - cosca-devops       (domain_strength: 0.72, confidence: 0.72)
```

#### Passo 3: Market Consulta Prediction Engine

Para cada candidato, o Market consulta o Prediction Engine (F7.1):

```yaml
predicoes:
  cosca-security:
    p_success: 0.94
    tempo_estimado: "12m30s"
    custo_estimado: "$0.028"
    risco: "🟢 BAIXO (score: 0.06)"
    confianca_predicao: 0.85

  cosca-architecture:
    p_success: 0.90
    tempo_estimado: "18m45s"
    custo_estimado: "$0.035"
    risco: "🟢 BAIXO (score: 0.10)"
    confianca_predicao: 0.82

  cosca-devops:
    p_success: 0.72
    tempo_estimado: "22m00s"
    custo_estimado: "$0.042"
    risco: "🟡 MODERADO (score: 0.28)"
    confianca_predicao: 0.65
```

#### Passo 4: Agentes Retornam Bid

Os agentes (via opt-in automático) confirmam disponibilidade:

```yaml
bids:
  cosca-security:
    confirmed: true
    estimated_time: "12m30s"
    estimated_cost: "$0.028"
    current_load: 1
    notes: "Última auditoria similar teve 100% de sucesso"

  cosca-architecture:
    confirmed: true
    estimated_time: "18m45s"
    estimated_cost: "$0.035"
    current_load: 2
    notes: "Disponível, mas com 2 tasks em andamento"

  cosca-devops:
    confirmed: true
    estimated_time: "22m00s"
    estimated_cost: "$0.042"
    current_load: 0
    notes: null
```

#### Passo 5: Market Calcula Score

O Market aplica a **fórmula de pontuação composta** (detalhada na Seção 5) para cada candidato:

```yaml
scores:
  cosca-security:
    score: 0.91
    breakdown:
      confidence_factor:     0.94 × 0.30 = 0.282
      efficiency_factor:     12m30s/18m45s_avg = 0.67 × 0.25 = 0.167
      cost_factor:           (1 - 0.028/0.050) = 0.44 × 0.20 = 0.088
      risk_factor:           0.94 × 0.15 = 0.141
      load_factor:           1/3 = 0.67 × 0.10 = 0.067

  cosca-architecture:
    score: 0.78
    breakdown:
      confidence_factor:     0.90 × 0.30 = 0.270
      efficiency_factor:     18m45s/18m45s_avg = 1.00 × 0.25 = 0.250
      cost_factor:           (1 - 0.035/0.050) = 0.30 × 0.20 = 0.060
      risk_factor:           0.90 × 0.15 = 0.135
      load_factor:           2/3 = 0.33 × 0.10 = 0.033

  cosca-devops:
    score: 0.45
    breakdown:
      confidence_factor:     0.72 × 0.30 = 0.216
      efficiency_factor:     22m00s/18m45s_avg = 0.85 × 0.25 = 0.213
      cost_factor:           (1 - 0.042/0.050) = 0.16 × 0.20 = 0.032
      risk_factor:           0.72 × 0.15 = 0.108
      load_factor:           0/3 = 1.00 × 0.10 = 0.100
```

#### Passo 6: Market Ranqueia Top 3

```yaml
ranking:
  1º: cosca-security   (score: 0.91) ← RECOMENDADO
  2º: cosca-architecture (score: 0.78)
  3º: cosca-devops      (score: 0.45)

rationale:
  winner: "cosca-security vence por:
    1. Maior confidence no domínio (0.94 vs 0.90)
    2. Menor tempo estimado (12m30s vs 18m45s)
    3. Menor custo ($0.028 vs $0.035)
    4. Carga moderada (1/3 tasks)"
```

#### Passo 7: Kernel Escolhe

O Kernel recebe o ranking e decide:

```yaml
kernel_decision:
  task_id: "audit-runtime-security-2026-07-30-001"
  selected_agent: "cosca-security"
  rationale: "Market recommendation accepted — highest score (0.91)"
  override: false
  gate_triggered: false          # P1, confidence > 0.70, sem Gate
  delegated_at: 2026-07-30T16:00:00Z
```

#### Passo 8: Shadow Bids Registrados

Agentes perdedores registram como teriam abordado a task (detalhado na Seção 4).

### 3.3 Tempo Limite do Leilão

O leilão completo deve executar em **< 100ms**:

| Fase | Tempo Máximo | Descrição |
|------|-------------|-----------|
| Filtro de candidatos | 10ms | Consulta ao índice de capacidades em cache |
| Consulta Prediction Engine | 40ms | 3 consultas paralelas (cada uma < 100ms) |
| Coleta de bids | 20ms | Opt-in automático (não requer LLM) |
| Cálculo de scores | 10ms | Aritmética simples |
| Formatação do ranking | 20ms | Geração do YAML de saída |
| **Total** | **100ms** | Limite máximo |

### 3.4 Fast Track (Leilão Simplificado)

Para tasks de baixa complexidade (complexity 1-2) com baixo custo estimado (< $0.005), o Market pode executar um **leilão simplificado**:

```
Fast Track:
  1. Kernel define parâmetros
  2. Market consulta cache de top-3 do domínio
  3. Seleciona #1 do cache (sem consulta Prediction Engine)
  4. Kernel delega diretamente
  5. Shadow bid registrado para #2 e #3 apenas

  Tempo total: < 20ms
```

---

## 4. Shadow Bids

### 4.1 O que São Shadow Bids

**Shadow bids** são registros de "como o agente teria executado a task" — mesmo que o agente não tenha vencido o leilão. É um **mecanismo de simulação e aprendizado**.

### 4.2 Propósito

```
Shadow Bids respondem a 3 perguntas:
  1. "E se tivéssemos escolhido outro agente? Qual seria o resultado?"
  2. "O segundo colocado teria sido significativamente pior?"
  3. "Como calibrar o modelo de pontuação com dados de 'quase-acertos'?"
```

### 4.3 Formato do Shadow Bid

Após cada leilão, os agentes perdedores (top 2) registram:

```yaml
shadow_bid:
  auction_id: "AUC-2026-07-30-001"
  task_id: "audit-runtime-security-2026-07-30-001"
  agent: "cosca-architecture"
  rank: 2
  score: 0.78
  would_have:
    approach: "Análise de threat model seguida de auditoria de dependências"
    estimated_time: "18m45s"
    estimated_cost: "$0.035"
    key_steps:
      - "Mapear superfície de ataque do runtime"
      - "Auditar configurações de segurança"
      - "Verificar dependências contra CVE database"
      - "Gerar relatório de risco"
    risk_areas:
      - "Complexidade de integração com compute fabric"
      - "Possível falso positivo em dependências indiretas"
  actual_winner_outcome: null   # Preenchido após task
  would_have_different: false   # Se teria feito diferente do winner
  learning: null                # Preenchido após task se abordagem diferente
```

### 4.4 Calibragem via Shadow Bids

Após a execução da task, o Market compara:

```yaml
calibragem_shadow:
  auction_id: "AUC-2026-07-30-001"
  task_id: "audit-runtime-security-2026-07-30-001"

  winner_real:
    agent: "cosca-security"
    outcome: success
    actual_time: "11m00s"
    actual_cost: "$0.025"

  shadow_comparison:
    - agent: "cosca-architecture"
      would_have_time: "18m45s"
      would_have_cost: "$0.035"
      delta_vs_winner:
        time: "+7m45s"      # 70% mais lento
        cost: "+$0.010"     # 40% mais caro
      veredito: "✅ Escolha correta — shadow confirma que security era melhor opção"

  market_learning:
    - "security-architecture domain: cosca-security consistentemente mais rápido que cosca-architecture"
    - "Peso do fator domain_strength deve ser aumentado de 0.25 para 0.30"
    - "Nenhum ajuste no score — shadow valida o ranking"
```

### 4.5 Shadow como Dados de Treino

Shadow bids são **dados de treino gratuitos** para o próprio Market:

| Uso | Descrição |
|-----|-----------|
| **Validar ranking** | Se shadow bids consistentemente mostram que o segundo colocado teria sido pior, o ranking está funcionando |
| **Detectar underconfidence** | Se shadow bids mostram que o perdedor teria sido tão bom quanto o winner, o score está subestimando o perdedor |
| **Calibrar pesos** | Se um fator do score consistentemente erra, seus pesos são ajustados |
| **Identificar gaps** | Se nenhum shadow bid é registrado (porque ninguém além do winner tem a capability), o domínio tem gap de cobertura |

### 4.6 Shadow Lock

Em situações específicas, shadow bids podem ser **promovidos a bids reais**:

```
Quando shadow lock está ativo:
  1. Market encerra leilão normalmente
  2. Winner executa
  3. Se winner falhar (outcome = failure ou partial):
     a. Shadow bid do segundo colocado é automaticamente promovido
     b. Segundo colocado executa sem novo leilão
     c. Custo é contabilizado como retrabalho
  4. Se winner suceder: shadow bid arquivado como aprendizado
```

---

## 5. Ranking e Pontuação

### 5.1 Fórmula do Score Composto

```
Score_COMPOSTO = Σ(peso_i × fator_i)

Onde:
  ┌── confidence_factor   (peso: 0.30) — P(success) estimado
  ├── efficiency_factor   (peso: 0.25) — Tempo relativo vs média do domínio
  ├── cost_factor         (peso: 0.20) — (1 - custo_est / max_cost)
  ├── risk_factor         (peso: 0.15) — (1 - risco_score)
  └── load_factor         (peso: 0.10) — (available_slots - current_load) / available_slots
```

### 5.2 Cálculo de Cada Fator

#### Confidence Factor

```
confidence_factor = P(success) da Prediction Engine
                  × trust_multiplier

Onde:
  trust_multiplier = 1.0 se reliability_score ≥ 0.80
                   = 0.95 se ≥ 0.70
                   = 0.85 se ≥ 0.50
                   = 0.70 se < 0.50
```

#### Efficiency Factor

```
efficiency_factor = min(avg_latency_domain / estimated_time, 1.5)

Onde:
  avg_latency_domain = Latência média para este domínio (Trust Registry)
  estimated_time     = Tempo estimado pelo Prediction Engine
  Cap: 1.5 (não pode ser mais de 50% mais rápido que a média)

Interpretação:
  > 1.0: Agente é mais rápido que a média do domínio  (bônus)
  = 1.0: Agente está na média                          (neutro)
  < 1.0: Agente é mais lento que a média               (penalidade)
```

#### Cost Factor

```
cost_factor = 1 - (estimated_cost / max_cost)

Onde:
  estimated_cost = Custo estimado pelo Prediction Engine
  max_cost       = Orçamento máximo definido pelo Kernel

Interpretação:
  Próximo de 1.0: Custo muito abaixo do orçamento  (bônus máximo)
  Próximo de 0.0: Custo próximo ao orçamento        (neutro)
  Negativo:        Custo excede o orçamento          (penalidade, score negativo)

Regra:
  Se cost_factor < 0:
    → Agente é removido do leilão (estoura orçamento)
    → A menos que não haja nenhum agente dentro do orçamento
    → Nesse caso, o menor custo vence com penalidade de -0.10 no score final
```

#### Risk Factor

```
risk_factor = 1 - risco_score

Onde:
  risco_score = Score de risco do Prediction Engine (0.0-1.0)

Interpretação:
  risco_score baixo  → risk_factor alto  → score melhor
  risco_score alto   → risk_factor baixo → score pior
```

#### Load Factor

```
load_factor = (available_slots - current_load) / available_slots

Onde:
  available_slots = Tasks simultâneas máximas do agente
  current_load    = Tasks em execução no momento

Interpretação:
  1.0: Agente ocioso (sem carga)
  0.5: Agente com metade da capacidade ocupada
  0.0: Agente completamente ocupado (não participa do leilão)
```

### 5.3 Normalização Final

O score final é normalizado para 0.0-1.0:

```
Score_FINAL = max(min(Score_COMPOSTO, 1.0), 0.0)

Regras:
  - Se Score_FINAL < 0.30: agente não é recomendado (risco alto demais)
  - Se Score_FINAL = 0.00: agente removido da lista de candidatos
  - Se único candidato com Score_FINAL > 0: recomendado com aviso
```

### 5.4 Exemplo de Cálculo Completo

```yaml
candidato: cosca-security
task: auditing-security-runtime

fatores:
  confidence_factor:
    p_success: 0.94
    trust_multiplier: 1.0 (reliability_score = 0.90 >= 0.80)
    resultado: 0.94 × 1.0 × 0.30 peso = 0.282

  efficiency_factor:
    avg_latency_security-audit: 15m00s (Trust Registry)
    estimated_time: 12m30s
    resultado: min(15m00s/12m30s, 1.5) = min(1.2, 1.5) = 1.2
    ponderado: 1.2 × 0.25 = 0.300

  cost_factor:
    estimated_cost: $0.028
    max_cost: $0.050
    resultado: 1 - (0.028/0.050) = 1 - 0.56 = 0.44
    ponderado: 0.44 × 0.20 = 0.088

  risk_factor:
    risco_score: 0.06
    resultado: 1 - 0.06 = 0.94
    ponderado: 0.94 × 0.15 = 0.141

  load_factor:
    available_slots: 3
    current_load: 1
    resultado: (3-1)/3 = 0.67
    ponderado: 0.67 × 0.10 = 0.067

score_composto: 0.282 + 0.300 + 0.088 + 0.141 + 0.067 = 0.878
score_final: 0.878 (normalizado, já dentro de 0-1)
```

### 5.5 Pesos por Prioridade

Os pesos dos fatores se ajustam conforme a prioridade da task:

| Prioridade | Confidence | Efficiency | Cost | Risk | Load |
|:----------:|:----------:|:----------:|:----:|:----:|:----:|
| **P0** (crítico) | 0.35 | 0.15 | 0.15 | 0.25 | 0.10 |
| **P1** (alto) | 0.30 | 0.25 | 0.20 | 0.15 | 0.10 |
| **P2** (médio) | 0.25 | 0.25 | 0.25 | 0.15 | 0.10 |
| **P3** (baixo) | 0.20 | 0.25 | 0.30 | 0.10 | 0.15 |

**Interpretação dos pesos por prioridade:**

- **P0**: Peso maior em **confidence** e **risk** — segurança é prioridade
- **P1**: Equilíbrio entre **confidence** e **efficiency** — qualidade com velocidade
- **P2**: Equilíbrio entre **confidence**, **efficiency** e **cost** — trade-off padrão
- **P3**: Peso maior em **cost** e **load** — eficiência e distribuição de carga

---

## 6. Integração com F7.1 — Prediction Engine

### 6.1 Prediction Engine Alimenta o Leilão

O Prediction Engine (F7.1) é a principal fonte de dados do leilão. O Market consulta o Prediction Engine para cada candidato **antes** de calcular o score:

```
CAPABILITY MARKET                     PREDICTION ENGINE (F7.1)
┌─────────────────────┐              ┌──────────────────────────┐
│ Consulta para cada  │──request──▶ │ Cálculo completo:        │
│ candidato:          │              │  ├── P(success)          │
│  ├── agent          │              │  ├── Tempo estimado      │
│  ├── task_type      │              │  ├── Custo estimado      │
│  ├── complexity     │              │  ├── Risco score         │
│  ├── domain         │              │  └── Confiança da pred.  │
│  └── max_cost       │              │                          │
│                     │◀──response── │                          │
│  Market usa output  │              │  output:                 │
│  para calcular      │              │  p_success: 0.94         │
│  score composto     │              │  tempo_est: 12m30s       │
│                     │              │  custo_est: $0.028       │
│                     │              │  risco: 🟢 BAIXO         │
└─────────────────────┘              └──────────────────────────┘
```

### 6.2 Cache de Predições

Para otimizar os < 100ms de leilão, o Market mantém um **cache de predições**:

```yaml
prediction_cache:
  ttl: 300                       # 5 minutos
  max_entries: 100               # Limite de memória
  hit_rate_target: 0.80          # 80% dos acessos devem ser cache hit

  keys:
    - agent: cosca-security
      task_type: security-audit
      complexity: 3
      domain: security-architecture
```

### 6.3 Feedback Loop

Pós-task, o Market envia feedback ao Prediction Engine:

```yaml
market_feedback:
  auction_id: "AUC-2026-07-30-001"
  task_id: "audit-runtime-security-2026-07-30-001"
  winner: "cosca-security"
  winner_score: 0.91
  runner_up: "cosca-architecture"
  runner_up_score: 0.78

  prediction_accuracy:
    p_success_previsto: 0.94
    p_success_real: 1.00
    erro: 0.06              # < 20% → OK

    tempo_previsto: "12m30s"
    tempo_real: "11m00s"
    erro_tempo: 12%         # < 20% → OK

    custo_previsto: "$0.028"
    custo_real: "$0.025"
    erro_custo: 10.7%       # < 20% → OK

  market_learning:
    - "cosca-security consistentemente abaixo do previsto em tempo e custo"
    - "Sugerir recalibragem dos predictions para este agente em security-audit"
```

---

## 7. Integração com F1.2 — Contrafactual Gate

### 7.1 Gatilho do Gate

O Capability Market pode **abrir o Contrafactual Gate** quando:

```
SE o leilão não encontrar um candidato com confidence > 0.70:
  ├── Abrir Contrafactual Gate
  ├── Gate analisa: "E se delegarmos para o melhor disponível mesmo assim?"
  ├── Alternativas:
  │    ├── Cenário A: Delegar para o melhor score (mesmo < 0.70)
  │    ├── Cenário B: Kernel executa diretamente
  │    └── Cenário C: Escalonar para Don
  └── Gate retorna: recomendação final para o Kernel

SE o leilão encontrar candidato com confidence >= 0.70:
  └── Contrafactual Gate NÃO é ativado (seguir ranking do Market)
```

**Exceção P0**: Se a task for P0, o Gate é ativado **independentemente do score**:

```
Task P0 → Market executa leilão normalmente
        → Gate é ativado (regra G1 do Contrafactual Gate)
        → Gate analisa top 3 do ranking como alternativas A, B, C
        → Gate recomenda ou questiona
        → Kernel decide
```

### 7.2 Pipeline Market → Gate

```
CAPABILITY MARKET (F8.1)               CONTRAFACTUAL GATE (F1.2)
┌─────────────────────────┐            ┌──────────────────────────┐
│ Leilão executado        │            │                          │
│                         │            │ Analisa cenários:        │
│ SE score_melhor < 0.70: │──abre──▶  │  A: Delegar #1 (0.65)   │
│   melhor_score: 0.65    │   gate     │  B: Kernel executa (0.78)│
│   agente: cosca-devops  │            │  C: Escalonar p/ Don    │
│                         │            │                          │
│ SE task for P0:         │──abre──▶  │ Gate recebe top 3:      │
│   ranking:              │   gate     │  A: cosca-security (0.91)│
│   #1 security (0.91)    │            │  B: cosca-arch (0.78)   │
│   #2 arch (0.78)        │            │  C: cosca-devops (0.45) │
│   #3 devops (0.45)      │            │  Gate recomenda:        │
│                         │            │  "A é claramente        │
│                         │            │   superior. Proceed."   │
└─────────────────────────┘            └──────────────────────────┘
```

### 7.3 Dados Transmitidos ao Gate

```yaml
contrafactual_input:
  origem: "capability_market"
  auction_id: "AUC-2026-07-30-001"
  task:
    id: "audit-runtime-security-2026-07-30-001"
    task_type: "security-audit"
    complexity: 3
    domain: "security-architecture"
    priority: "P0"
    max_cost: "$0.050"

  market_result:
    status: "completed"
    gate_trigger: "P0"                     # Ou "low_confidence" se score < 0.70
    melhor_score: 0.91
    media_scores: 0.71
    desvio_padrao: 0.19

  alternativas:
    - agent: "cosca-security"
      score: 0.91
      p_success: 0.94
      estimated_time: "12m30s"
      estimated_cost: "$0.028"
      reliability: 0.90

    - agent: "cosca-architecture"
      score: 0.78
      p_success: 0.90
      estimated_time: "18m45s"
      estimated_cost: "$0.035"
      reliability: 0.93

    - agent: "cosca-devops"
      score: 0.45
      p_success: 0.72
      estimated_time: "22m00s"
      estimated_cost: "$0.042"
      reliability: 0.72
```

### 7.4 Regra de Override do Gate

```
Se Gate recomendar ESCALONAR (score < 0.70 + sem boa alternativa):
  ├── Kernel acata a menos que:
  │    ├── Don explicitamente escolheu este agente
  │    └── Única opção disponível no momento
  └── Se Don override: registrado em DDNA

Se Gate recomendar PROCEED (score >= 0.70 ou top 1 claramente superior):
  ├── Kernel delega para o #1 do ranking
  └── Shadow bids dos perdedores são registrados
```

---

## 8. Integração com F2.1 — Cognitive Economy

### 8.1 Market Alimenta a Cognitive Economy

O resultado do leilão fornece dados cruciais para o cálculo de ROI cognitivo:

```
CAPABILITY MARKET (F8.1)               COGNITIVE ECONOMY (F2.1)
┌─────────────────────────┐            ┌──────────────────────────┐
│ Resultado do leilão:    │──custo──▶ │ Cost Vector:             │
│  ├── winner: security   │            │  ├── tokens: $0.028      │
│  ├── custo: $0.028      │            │  ├── time: 12m30s        │
│  ├── tempo: 12m30s      │            │  ├── compute: estimado   │
│  ├── confidence: 0.91   │            │  ├── attention: 2        │
│  └── p_success: 0.94    │            │  └── storage: 1          │
│                         │──valor──▶ │                          │
│  Shadow bids:           │            │ Value Vector:            │
│  ├── arch: $0.035       │            │  ├── alternative_cost    │
│  ├── devops: $0.042     │            │  ├── {compared to}      │
│  └── economia: $0.007   │            │  ├── savings: $0.007    │
│                         │            │  └── ROI: 6.2×          │
└─────────────────────────┘            └──────────────────────────┘
```

### 8.2 Custo Evitado pelo Market

Uma das métricas mais importantes que o Market fornece para a Cognitive Economy é o **custo evitado** — a diferença entre o pior e o melhor candidato:

```yaml
cost_avoided:
  auction_id: "AUC-2026-07-30-001"
  winner_cost: "$0.028"
  worst_candidate_cost: "$0.042"
  cost_avoided: "$0.014"              # 50% de economia
  time_avoided: "9m30s"               # Diferença entre winner e pior
  rationale: "O Market economizou $0.014 ao escolher security em vez de devops"
```

### 8.3 ROI do Próprio Leilão

O custo do leilão (< 100ms, ~$0.0005 em tokens) é comparado ao benefício:

```yaml
leilao_roi:
  auction_id: "AUC-2026-07-30-001"
  custo_leilao: "$0.0005"            # < 0.1% do custo da task
  economia_gerada: "$0.014"           # 28× o custo do leilão
  roi_leilao: "2800%"                 # (0.014 - 0.0005) / 0.0005
  veredito: "🔥 Leilão com ROI altíssimo — cada $1 investido gerou $28 em economia"
```

### 8.4 Modos de Operação Conjunta

| Modo | Capability Market | Cognitive Economy | Custo do Leilão |
|:----:|:-----------------:|:-----------------:|:---------------:|
| **Full** | Leilão completo com Prediction + Shadow | Full ROI tracking | ~$0.0005 |
| **Standard** | Leilão completo sem Shadow | Apenas cost tracking | ~$0.0003 |
| **Fast** | Leilão simplificado (cache) | Apenas winner cost | ~$0.0001 |
| **Direct** | Kernel delega diretamente (Don override) | Skip | $0 |

---

## 9. Integração com F1.1 — Decision DNA

### 9.1 Leilões Viram DDNA

Decisões de roteamento do Market que envolvem **trade-offs significativos** (P0, score apertado, ou gate ativado) são registradas como DDNA:

```
Quando registrar DDNA do Market:
  ├── Leilão P0: SEMPRE registrar DDNA
  ├── Score gap < 0.05 entre #1 e #2: registrar DDNA
  ├── Gate foi ativado: registrar DDNA
  ├── Don fez override: registrar DDNA
  └── Shadow lock promoveu perdedor: registrar DDNA
```

### 9.2 Formato do DDNA de Roteamento

```yaml
---
id: DDNA-2026-07-30-003
title: "Roteamento: auditoria de segurança delegada para cosca-security via Market"
status: accepted
date: 2026-07-30
agents:
  - cosca-kernel
  - cosca-capability-market
  - cosca-security
domain: orchestration
decision_level: 2
confidence: 0.91
revisit: 2026-08-30
tags:
  - dna
  - market-routing
  - security-audit
  - auction
supersedes:
superseded_by:
---

## Context

Task de auditoria de segurança do runtime chegou ao Kernel.
O Capability Market executou leilão com 3 candidatos.

## Options

### Opção A: cosca-security (score: 0.91) — DECISÃO

**Descrição:** Agente especialista em segurança, domínio security-architecture forte.

| Critério | Avaliação |
|----------|-----------|
| P(success) | 0.94 |
| Custo | $0.028 |
| Tempo | 12m30s |
| Risco | 🟢 BAIXO |

### Opção B: cosca-architecture (score: 0.78)

**Descrição:** Agente de arquitetura, pode fazer auditoria mas não é especialista.

| Critério | Avaliação |
|----------|-----------|
| P(success) | 0.90 |
| Custo | $0.035 |
| Tempo | 18m45s |
| Risco | 🟢 BAIXO |

### Opção C: cosca-devops (score: 0.45)

**Descrição:** Agente de devops, sem histórico forte em security-audit.

| Critério | Avaliação |
|----------|-----------|
| P(success) | 0.72 |
| Custo | $0.042 |
| Tempo | 22m00s |
| Risco | 🟡 MODERADO |

## Decision

**Decisão final:** Opção A — cosca-security.

**Justificativa:** Score 0.91 vs 0.78 do segundo colocado. Gap de 0.13 é significativo.
Security tem melhor P(success), menor custo, menor tempo, e menor risco.
Shadow bid de architecture confirma que security era a melhor escolha.

## Consequences

### Positivas
- Economia estimada de $0.007 vs segunda melhor opção
- Task executada por especialista com 94% de chance de sucesso
- Shadow bids fornecem baseline para calibragem futura

### Negativas
- architecture perde oportunidade de aprender security-audit
- Se security falhar, não há fallback quente (shadow lock só na segunda tentativa)

### Mitigações
- Shadow lock configurado: se security falhar, architecture assume automaticamente
- Shadow bids registrados para calibragem do modelo

## Evidence

- [Capability Market Auction Result](internal/embed/cosca/engines/capability-market/auctions/AUC-2026-07-30-001.yaml)
- [Prediction Engine Output](internal/embed/cosca/engines/prediction/predictions/PRED-2026-07-30-SEC-001.yaml)
- [Trust Registry — cosca-security](internal/embed/cosca/memory/trust/TRUST_REGISTRY.md#6-cosca-security)
```

---

## 10. Aprendizado do Mercado

### 10.1 O Mercado Aprende

O Capability Market não é estático — ele **aprende com cada leilão** e calibra seus próprios pesos e regras:

```
Ciclo de Aprendizado:
  ┌────────────────────────────────────────────────────────┐
  │ 1. EXECUTAR leilão                                     │
  │ 2. REGISTRAR resultado (winner + shadow bids)          │
  │ 3. COMPARAR previsto vs real (pós-task)                │
  │ 4. CALIBRAR pesos dos fatores                          │
  │ 5. AJUSTAR ranking para próximos leilões               │
  │ 6. REPETIR                                             │
  └────────────────────────────────────────────────────────┘
```

### 10.2 Regras de Calibragem

| Condição | Ação | Gatilho |
|----------|------|---------|
| Agente consistentemente superestima capacidade | Penalidade de -0.05 no score | 3 overstatements consecutivos |
| Agente consistentemente subestima capacidade | Bônus de +0.03 no score | 3 underpromise consecutivos |
| Shadow bid mostra que perdedor teria sido melhor | Ajustar peso do factor que errou | 2 shadow wins consecutivos |
| Um fator do score nunca é decisivo | Reduzir peso do factor em 0.02 | 10 leilões sem impacto do fator |
| Um fator do score sempre decide | Aumentar peso do fator em 0.01 | 5 leilões onde fator foi decisivo |
| Agente vence mas falha na execução | Reduzir confidence do agente em 0.10 | Imediato (pós-task) |
| Agente perde mas shadow mostra que venceria | Aumentar confidence do agente em 0.05 | Imediato (pós-task) |

### 10.3 Overstatement — Penalidade Progressiva

Se um agente consistentemente anuncia capacidades melhores que sua performance real:

```
1º overstatement:  Aviso ao agente    → confiança não alterada
2º overstatement:  Penalidade -0.05   → score reduzido em 5%
3º overstatement:  Penalidade -0.10   → score reduzido em 10%
4º overstatement:  Marcado unreliable → anúncio ignorado até revisão do Don

O overstatement é detectado quando:
  confidence_anunciada - confidence_real > 0.15
  OU
  success_rate_anunciado - success_rate_real > 10%
  OU
  avg_cost_anunciado < avg_cost_real × 0.50 (subestimativa de custo)
```

### 10.4 Underconfidence — Bônus

Se um agente consistentemente anuncia capacidades piores que sua performance real, ele recebe bônus:

```
1º underpromise: Sem ação (pode ser estratégia conservadora)
2º underpromise: Bônus de +0.03 (recompensa por conservadorismo)
3º underpromise: Confiança do agente aumentada em 0.05 no Trust Registry
```

### 10.5 Versionamento do Modelo de Scores

Os pesos do market são versionados:

```yaml
market_model_version:
  id: "CM-MODEL-v1.0.0"
  data: "2026-07-30"
  pesos_default:
    confidence: 0.30
    efficiency: 0.25
    cost: 0.20
    risk: 0.15
    load: 0.10

  pesos_por_prioridade:
    P0: { confidence: 0.35, efficiency: 0.15, cost: 0.15, risk: 0.25, load: 0.10 }
    P1: { confidence: 0.30, efficiency: 0.25, cost: 0.20, risk: 0.15, load: 0.10 }
    P2: { confidence: 0.25, efficiency: 0.25, cost: 0.25, risk: 0.15, load: 0.10 }
    P3: { confidence: 0.20, efficiency: 0.25, cost: 0.30, risk: 0.10, load: 0.15 }

  overstatement_threshold: 0.15
  min_score_to_recommend: 0.30
  auction_timeout_ms: 100
  max_candidates: 5

  calibration_history:
    - version: "v1.0.0"
      data: "2026-07-30"
      razao: "Versão inicial do Capability Market"
      leiloes: 0
```

---

## 11. Regras de Operação

### 11.1 Regras Imutáveis

| # | Regra | Descrição | Consequência |
|---|-------|-----------|--------------|
| **R1** | **Leilão < 100ms** | O leilão completo (filtro → score → ranking) não pode exceder 100ms | Se exceder, fallback para modo Fast (cache) |
| **R2** | **Agentes não mentem** | Confidence do anúncio é lastreada no Trust Registry real. Overstatement é detectado e penalizado | Ver Seção 10.3 — overstatement progressivo |
| **R3** | **Override do Don sempre vence** | Don pode escolher qualquer agente, ignorando o ranking | Override é registrado em DDNA com rationale |
| **R4** | **Mercado aprende** | Se um agente consistentemente superestima sua capacidade, perde ranking | Ver Seção 10.3 — calibragem contínua |
| **R5** | **Shadow bids são obrigatórios** | Top 2 perdedores DEVEM registrar shadow bid | Se não registrar, penalidade no próximo leilão |
| **R6** | **Transparência total** | Todo leilão gera registro completo auditável | Auction ID linkado a DDNA se P0 ou gate ativado |
| **R7** | **P0 sempre abre Gate** | Decisões P0 passam pelo Contrafactual Gate mesmo se score > 0.70 | Ver Seção 7.1 |

### 11.2 Don Override

O Don pode fazer override do Market de duas formas:

**Override Explícito:**
```yaml
don_override:
  task_id: "audit-runtime-security-2026-07-30-001"
  override_to: "cosca-architecture"
  rationale: "Don quer validação arquitetural além da auditoria de segurança"
  override_type: "explicit"           # explicit | emergency
  market_recommendation: "cosca-security (score: 0.91)"
  market_respected: false
  registered_in_ddna: "DDNA-2026-07-30-003"
```

**Override de Emergência:**
```yaml
don_emergency:
  task_id: "critical-hotfix-2026-07-30-001"
  override_to: "cosca-kernel"
  rationale: "Hotfix crítico — Kernel executa diretamente sem leilão"
  override_type: "emergency"
  market_skipped: true
  post_action: "Registrar DDNA + Shadow bids retroativos"
```

### 11.3 Tratamento de Erros

| Falha | Ação | Impacto |
|-------|------|---------|
| Nenhum agente com capability compatível | Abrir Contrafactual Gate (Cenário C — escalar para Don) | Task bloqueada até decisão |
| Prediction Engine indisponível | Usar confidence do anúncio (sem ajuste do Prediction) | Score menos preciso |
| Trust Registry indisponível | Rejeitar todos os anúncios — mercado suspenso | Kernel volta a escalar manualmente |
| Agente não responde bid em 50ms | Assumir busy (available_slots = 0) | Agente não participa do leilão |
| Cache de predições expirado | Consulta síncrona ao Prediction Engine | +40ms no leilão (ainda dentro do limite) |
| Dois agentes com score idêntico | Desempate por: maior reliability_score → menor custo → menor tempo | Desempate determinístico |
| Leilão excede 100ms | Fallback para Fast Track (cache, sem shadow) | Score menos preciso, shadow perdido |

---

## 12. Exemplo com Dados Reais do Trust Registry

### 12.1 Task: "Auditar Segurança do Runtime"

**Contexto real** (extraído de `internal/embed/cosca/memory/trust/TRUST_REGISTRY.md`):

```yaml
task:
  id: "audit-runtime-security-2026-07-30-001"
  task_type: "security-audit"
  complexity: 3
  domain: "security-architecture"
  max_cost: "$0.050"
  max_latency: "30m"
  priority: "P1"
  description: "Auditar segurança do runtime — analisar threat model,
    dependências e configurações de segurança"
```

### 12.2 Agentes Candidatos (do Trust Registry)

**cosca-security:**
```yaml
success_rate:
  last_20: 1.00    # 3/3 tasks
  total: 1.00
  total_tasks: 3
avg_latency:
  by_task_type:
    specification: 16m30s
avg_cost:
  by_task_type:
    specification: $0.030
domain_strength:
  cognitive-immune-system: 0.88
  auth-architecture: 0.82
  compliance-audit: 0.78
  security-documentation: 0.80
reliability_score: 0.90
confidence_after: 0.88
```

**cosca-architecture:**
```yaml
success_rate:
  last_20: 1.00    # 5/5 tasks
  total: 1.00
  total_tasks: 5
avg_latency:
  by_task_type:
    specification: 18m33s
avg_cost:
  by_task_type:
    specification: $0.034
domain_strength:
  specification: 0.93
  architecture-design: 0.93
  cognitive-architecture: 0.93
reliability_score: 0.93
confidence_after: 0.93
```

**cosca-devops:** (Dados inferidos — não registrado no Trust Registry atual, usado como placeholder)
```yaml
success_rate:
  total: 0.95
  total_tasks: 20
domain_strength:
  security-architecture: 0.72
  devops: 0.90
reliability_score: 0.72
confidence_after: 0.72
```

### 12.3 Anúncios no Market

```yaml
market_announcements:
  - agent: cosca-security
    capabilities:
      - name: security-audit
        confidence: 0.94    # Trust Registry: 0.88 + bônus de transferência
        avg_latency: 1.2s
        avg_cost: $0.004
        success_rate: 98%
        domain: security-architecture
        max_complexity: 5
        available_slots: 3
        current_load: 1

  - agent: cosca-architecture
    capabilities:
      - name: security-review
        confidence: 0.90
        avg_latency: 2.1s
        avg_cost: $0.006
        success_rate: 95%
        domain: architecture
        max_complexity: 5
        available_slots: 2
        current_load: 2

  - agent: cosca-devops
    capabilities:
      - name: security-hardening
        confidence: 0.85
        avg_latency: 1.8s
        avg_cost: $0.003
        success_rate: 90%
        domain: devops
        max_complexity: 4
        available_slots: 3
        current_load: 0
```

### 12.4 Leilão Executado

**Passo 1-2: Filtro de Candidatos**

```yaml
filtro:
  domínio_match: security-audit → cosca-security (domain: security-architecture) ✓
                                → cosca-architecture (domain: architecture)       ✓ (parcial)
                                → cosca-devops (domain: devops)                  ✓ (parcial)
```

**Passo 3: Prediction Engine consultado**

```yaml
predicoes:
  cosca-security:
    p_success: 0.94
    tempo_estimado: "12m30s"
    custo_estimado: "$0.028"
    risco: "🟢 BAIXO (0.06)"

  cosca-architecture:
    p_success: 0.90
    tempo_estimado: "18m45s"
    custo_estimado: "$0.035"
    risco: "🟢 BAIXO (0.10)"

  cosca-devops:
    p_success: 0.72
    tempo_estimado: "22m00s"
    custo_estimado: "$0.042"
    risco: "🟡 MODERADO (0.28)"
```

**Passo 4: Bids recebidos**

```yaml
bids:
  cosca-security: confirmed
  cosca-architecture: confirmed
  cosca-devops: confirmed
```

**Passo 5-6: Score calculado**

```yaml
ranking:
  1º: cosca-security    (score: 0.91)  ← RECOMENDADO
  2º: cosca-architecture (score: 0.78)
  3º: cosca-devops       (score: 0.45)

breakdown:
  cosca-security:        cosca-architecture:   cosca-devops:
    confidence: 0.282      confidence: 0.270      confidence: 0.216
    efficiency: 0.300      efficiency: 0.250      efficiency: 0.213
    cost:        0.088      cost:        0.060      cost:        0.032
    risk:        0.141      risk:        0.135      risk:        0.108
    load:        0.067      load:        0.033      load:        0.100
    ──────────────────     ──────────────────      ──────────────────
    TOTAL: 0.878 → 0.91    TOTAL: 0.748 → 0.78     TOTAL: 0.669 → 0.45
```

**Passo 7: Kernel decide**

```yaml
kernel_decision:
  selected: cosca-security
  rationale: "Market recommendation accepted. Score 0.91 é claramente superior.
              Security é especialista no domínio, com melhor P(success),
              menor custo e menor tempo."
  gate_triggered: false    # P1, confidence > 0.70
```

**Passo 8: Shadow bids registrados**

```yaml
shadow_bids:
  - agent: cosca-architecture
    rank: 2
    score: 0.78
    would_have:
      approach: "Análise de threat model seguida de auditoria de dependências"
      estimated_time: "18m45s"
      estimated_cost: "$0.035"

  - agent: cosca-devops
    rank: 3
    score: 0.45
    would_have:
      approach: "Auditoria automatizada de segurança"
      estimated_time: "22m00s"
      estimated_cost: "$0.042"
```

### 12.5 Resultado Real (Pós-Task)

```yaml
outcome:
  task_id: "audit-runtime-security-2026-07-30-001"
  winner: cosca-security
  outcome: success
  actual_time: "11m00s"          # Previsto: 12m30s
  actual_cost: "$0.025"          # Previsto: $0.028
  confidence_delta: +0.04
  files_changed: 4
  loc_delta: +250

market_validation:
  score_accuracy: 0.87           # Score previsto (0.91) - erro (0.04)
  ranking_correct: true          # #1 venceu e entregou
  shadow_validation:
    cosca-architecture:
      time_if_selected: "~18m"   # 64% mais lento que security
      cost_if_selected: "$0.033" # 32% mais caro que security
      veredito: "✅ Ranking correto — security era a melhor escolha"
  economia_gerada:
    vs_architecture: "$0.008"    # (0.033 - 0.025)
    vs_devops: "$0.017"          # (0.042 - 0.025)
    total_evitado: "$0.017"      # Economia total do leilão

market_learning:
  - "cosca-security é consistentemente subestimado em tempo (-12%) e custo (-10.7%)"
  - "Sugerir recalibragem do domain_strength de cosca-security para security-audit"
  - "Peso do fator efficiency (0.25) funcionou bem — security venceu com folga"
```

---

## 13. Diagrama de Arquitetura

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                          CAPABILITY MARKET ENGINE (F8.1)                              │
│                          ═══════════════════════════                                   │
│                                                                                        │
│  ┌────────────────────────────────────────────────────────────────────────────────┐   │
│  │                              INPUT LAYER                                         │   │
│  │                                                                                  │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │   │
│  │  │  Kernel      │  │  Anúncios    │  │  Trust        │  │  Prediction      │   │   │
│  │  │  (task def)  │  │  (agentes)   │  │  Registry     │  │  Engine (F7.1)   │   │   │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘   │   │
│  │         └──────────────────┴─────────────────┴───────────────────┘              │   │
│  └────────────────────────────────────┬────────────────────────────────────────────┘   │
│                                       │                                                 │
│                                       ▼                                                 │
│  ┌────────────────────────────────────────────────────────────────────────────────┐   │
│  │                            LEILÃO ENGINE                                         │   │
│  │                                                                                  │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐    │   │
│  │  │  Filtro  │  │ Consulta │  │  Coleta  │  │  Score   │  │  Ranking     │    │   │
│  │  │ candidatos│  │ F7.1     │  │  bids    │  │  cálculo │  │  + Top 3     │    │   │
│  │  │ domain   │→ │ P(succ)  │→ │ confirm  │→ │ 5 fatores│→ │ recomendar   │    │   │
│  │  │ + avail  │  │ time,cost│  │ 20ms     │  │ 10ms     │  │ 20ms         │    │   │
│  │  │ 10ms     │  │ 40ms     │  │          │  │          │  │              │    │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────┬───────┘    │   │
│  └────────────────────────────────────┬──────────────────────────────┼──────────────┘   │
│                                       │                              │                   │
│                                       ▼                              ▼                   │
│  ┌────────────────────────────┐  ┌─────────────────────────────────────────┐          │
│  │     OUTPUT LAYER           │  │     SHADOW LAYER                          │          │
│  │                            │  │                                            │          │
│  │  ┌────────────────────┐   │  │  ┌──────────────────┐  ┌──────────────┐  │          │
│  │  │ Ranking + Score    │   │  │  │ Shadow bids      │  │ Calibragem   │  │          │
│  │  │ → Kernel decide    │   │  │  │ (#2 e #3 perdem) │  │ (após task)  │  │          │
│  │  └─────────┬──────────┘   │  │  └────────┬─────────┘  └──────┬───────┘  │          │
│  │            │              │  │           │                   │           │          │
│  │            ▼              │  │           ▼                   ▼           │          │
│  │  ┌────────────────────┐   │  │  ┌──────────────────────────────────┐    │          │
│  │  │ Kernel delega      │   │  │  │ DDNA (se P0 ou score < 0.70)   │    │          │
│  │  │ → Task executa     │   │  │  └──────────────────────────────────┘    │          │
│  │  └────────────────────┘   │  └─────────────────────────────────────────┘          │
│  └────────────────────────────┘                                                       │
│                                                                                        │
│  ┌────────────────────────────────────────────────────────────────────────────────┐   │
│  │                           INTEGRAÇÕES                                           │   │
│  │                                                                                  │   │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────────┐               │   │
│  │  │ F7.1 Prediction │  │ F1.2 Gate       │  │ F2.1 Cognitive   │               │   │
│  │  │ Engine          │  │ (se score<0.70  │  │ Economy          │               │   │
│  │  │ → estimativas   │  │  ou P0)         │  │ → ROI do leilão  │               │   │
│  │  └─────────────────┘  └─────────────────┘  └──────────────────┘               │   │
│  └────────────────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────────────────┘
```

### Pipeline de Integração no Kernel

```
KERNEL.md PIPELINE (com Capability Market):

  Step 5: Request Analysis → task_type, complexity, domain, priority
       │
       ▼
  ┌──────────────────────────────────────────────────────────────────┐
  │ ★ CAPABILITY MARKET ENGINE (F8.1) ★                              │
  │                                                                  │
  │  Fast Path (complexity 1-2, baixo custo):                       │
  │    ├── 1. Consulta cache de top-3 do domínio                     │
  │    ├── 2. Seleciona #1 (sem consulta F7.1)                       │
  │    └── 3. Kernel delega diretamente                             │
  │                                                                  │
  │  Full Path (complexity 3+, P0/P1, custo > $0.01):               │
  │    ├── 1. Filtra candidatos por domain + availability            │
  │    ├── 2. Consulta Prediction Engine (F7.1) para cada candidato  │
  │    ├── 3. Coleta bids dos candidatos                             │
  │    ├── 4. Calcula score composto (5 fatores)                     │
  │    ├── 5. Gera ranking Top 3                                     │
  │    └── 6. Registra shadow bids                                   │
  │                                                                  │
  │  Gate Check:                                                     │
  │    ├── SE score #1 >= 0.70 E task não é P0:                     │
  │    │     → Kernel delega para #1                                 │
  │    ├── SE score #1 < 0.70 OU task é P0:                         │
  │    │     → Abre Contrafactual Gate (F1.2)                        │
  │    └── SE Don override:                                          │
  │          → Kernel delega para o agente escolhido pelo Don        │
  │                                                                  │
  │  Pós-Task:                                                       │
  │    ├── Kernel registra outcome real                              │
  │    ├── Shadow bids calibrados                                    │
  │    ├── Market aprende (ajusta pesos se necessário)               │
  │    └── DDNA criado (se aplicável)                                │
  └──────────────────────────────────────────────────────────────────┘
       │
       ▼
  Step 7: Execution (agente selecionado executa)
```

---

## 14. Métricas do Engine

### 14.1 Métricas do Próprio Leilão

| Métrica | Tipo | Descrição | Alerta |
|---------|------|-----------|--------|
| `market.auctions.total` | Counter | Total de leilões executados | — |
| `market.auctions.duration_ms` | Histogram | Tempo de execução do leilão | Se > 100ms, investigar |
| `market.auctions.by_priority` | Counter | Leilões por prioridade (P0/P1/P2/P3) | — |
| `market.auctions.by_fast_path` | Counter | Leilões que usaram Fast Track | Se < 30%, configurar Fast Track |
| `market.bids.per_auction` | Histogram | Média de bids por leilão | Se < 2, poucos candidatos |
| `market.ranking.gap` | Histogram | Gap de score entre #1 e #2 | Se < 0.05, leilão apertado |
| `market.shadow.registered` | Counter | Shadow bids registrados | Se < 50% dos leilões, problema |
| `market.shadow.wins` | Counter | Shadow bids que teriam sido melhores | Se > 10%, ranking descalibrado |
| `market.overstatement.detected` | Counter | Overstatements detectados | Se > 5/dia, revisar validação |
| `market.don.override` | Counter | Don overrides no ranking | Se > 20%, market desalinhado |

### 14.2 Métricas de Acerto do Market

| Métrica | Tipo | Descrição | Meta |
|---------|------|-----------|:----:|
| `market.accuracy.ranking_correct` | Gauge | % de vezes que #1 era realmente o melhor | > 90% |
| `market.accuracy.score_error` | Histogram | |Score previsto - score real| | < 0.10 |
| `market.accuracy.time_error` | Histogram | Erro de tempo previsto vs real | < 20% |
| `market.accuracy.cost_error` | Histogram | Erro de custo previsto vs real | < 20% |
| `market.economy.savings` | Gauge | Economia total gerada pelo market | Tracking |
| `market.economy.roi` | Gauge | ROI acumulado do próprio market | > 500% |

### 14.3 Dashboard

```yaml
market_dashboard:
  widgets:
    - "Gauge: acurácia do ranking (% de #1 corretos)"
    - "Time series: leilões/dia por prioridade"
    - "Bar: tempo médio do leilão (ms)"
    - "Table: top 5 agentes por score médio"
    - "Alert: quando shadow win rate > 10%"
    - "Sparkline: economia acumulada ($)"
```

---

## 15. Relacionados

| Documento | Relação | Localização |
|-----------|---------|-------------|
| **Prediction Engine (F7.1)** | Fonte de estimativas para o leilão | `engines/prediction/SKILL.md` |
| **Trust Registry (F7.2)** | Base de reputação para validação de anúncios | `memory/trust/TRUST_REGISTRY.md` |
| **Contrafactual Gate (F1.2)** | Gate ativado se score < 0.70 ou task P0 | `workflows/contrafactual-gate.md` |
| **Cognitive Economy (F2.1)** | ROI do leilão e custo evitado | `engines/cognitive-economy/SKILL.md` |
| **Decision DNA (F1.1)** | Registro de decisões de roteamento | `knowledge/architecture/DECISION_DNA.md` |
| **Capability Engine** | Registro estático de capacidades (catálogo) | `engines/capability/SKILL.md` |
| **Next Evolution Phases** | Visão geral da Fase 8 | `knowledge/architecture/next-evolution-phases.md` |
| **Kernel** | Pipeline de execução com Market integrado | `KERNEL.md` |
| **Capability Catalog** | Catálogo de capacidades do sistema | `capabilities/CAPABILITY_CATALOG.md` |
| **Cognitive Maturity Implementation** | Pipeline CMI com Market | `workflows/cognitive-maturity-implementation.md` |

---

## 16. Histórico

| Versão | Data | Autor | Alterações |
|--------|------|-------|------------|
| 1.0.0 | 2026-07-30 | cosca-architecture | Criação inicial — Capability Market Engine ★ F8.1: definição, protocolo de anúncio, mecanismo de leilão, shadow bids, ranking com 5 fatores, integrações com F7.1, F1.2, F2.1, F1.1, aprendizado do mercado, regras de operação, exemplo real com Trust Registry |

---

> **Owner**: cosca-architecture | **Invocado por**: Kernel (entre Step 5 e Step 7) | **Mandatório para**: Toda task com complexity ≥ 3 ou P0/P1
> **Engine path**: `engines/capability-market/SKILL.md` | **Workflow associado**: `workflows/market-routing.md`
>
> *"O Kernel não escala mais — o mercado escala."*
> — Cosca Capability Market, 2026-07-30
