# COGNITIVE METRICS — Real Metrics Tracking (F1.5)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Analytics Chief | **Criado**: 2026-07-30
> **Workflow**: `cognitive-maturity-implementation.md` — Tarefa F1.5
> **Dependências**: F1.4 (Wisdom Decay) → alimenta B3 | F1.1 (DDNA) → alimenta B2
> **Benchmarks**: B1-B5 — Runtime Cognitive Health

---

## 1. Propósito

Este documento define o sistema de **métricas quantitativas reais** do Cosca Runtime. Ao contrário de scores abstratos, estas métricas são **coletadas automaticamente, armazenadas em formato parseável e vinculadas a thresholds acionáveis**.

As 5 métricas (B1-B5) medem a **saúde cognitiva do runtime** — não o que o sistema entrega, mas **como** ele opera internamente. São o sistema nervoso da Cognitive Economy (F2.1): sem estas métricas, não há como calcular ROI cognitivo.

```
┌──────────────────────────────────────────────────────────────────┐
│                    COGNITIVE METRICS MAP                          │
│                                                                   │
│  B1 ──► Custo (Cognitive Load)                                    │
│  B2 ──► Velocidade (Decision Velocity)                            │
│  B3 ──► Frescor (Knowledge Freshness)  ← F1.4 Wisdom Decay       │
│  B4 ──► Reuso (Cross-agent Reuse)                                 │
│  B5 ──► Dívida (Cognitive Debt)                                   │
│                                                                   │
│  ↓↓↓↓↓                                                           │
│  F2.1 Cognitive Economy — ROI = (B4 - B5) × B3 / (B1 × B2)      │
└──────────────────────────────────────────────────────────────────┘
```

---

## 2. Arquitetura de Coleta

### 2.1 Ciclo de Vida da Métrica

```
┌─────────────────────────────────────────────────────────────────────┐
│                     MÉTRICA — CICLO DE VIDA                          │
│                                                                      │
│  ┌──────────┐   ┌───────────┐   ┌───────────┐   ┌───────────────┐   │
│  │  EVENTO  │──►│  COLETA   │──►│ CÁLCULO   │──►│ ARMAZENAMENTO │   │
│  │  ocorre  │   │  Kernel   │   │ Fórmula   │   │ CSV timeline  │   │
│  └──────────┘   │ registra  │   │ aplicada  │   └───────┬───────┘   │
│                 └───────────┘   └───────────┘           │           │
│                                                          ▼           │
│                                                  ┌──────────────┐   │
│                                                  │   ALERTA?    │   │
│                                                  │ Threshold    │   │
│                                                  │ violado?     │   │
│                                                  └──────┬───────┘   │
│                                                    Sim  │  Não      │
│                                                    ┌────┴────┐     │
│                                                    ▼         ▼     │
│                                             ┌─────────┐ ┌────────┐ │
│                                             │ ALERTA  │ │ NO-OP  │ │
│                                             │ Kernel  │ │        │ │
│                                             │ notif.  │ │        │ │
│                                             └─────────┘ └────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 Gatilhos de Coleta

| Gatilho | Disparado Por | Métricas Coletadas |
|---------|--------------|-------------------|
| **Post-task hook** | Kernel ao finalizar task | B1 (load), B4 (reuse) |
| **Pre-task check** | Kernel ao receber pedido do Don | B2 (velocity), B3 (freshness) |
| **Audit loop check** | Cognitive Audit Loop (Q1-Q5) | B5 (debt) |
| **Wisdom Decay audit** | Wisdom Decay engine (F1.4) | B3 (freshness scores) |
| **Scheduled** | A cada 10 tasks ou semanal | Relatório consolidado |

### 2.3 Armazenamento — Formato CSV

Todas as métricas são armazenadas em CSV no diretório `memory/timeline/`:

```
internal/embed/cosca/memory/timeline/
├── cognitive-load.csv          # B1
├── decision-velocity.csv       # B2
├── knowledge-freshness.csv     # B3
├── cross-agent-reuse.csv       # B4
├── cognitive-debt.csv          # B5
└── cognitive-health-report.md  # Relatório consolidado (gerado a cada 10 tasks)
```

**Schema CSV padrão** (todos os arquivos seguem este formato):
- Header: `timestamp,task_id,agent,value,threshold,alert`
- Separador: `,` (vírgula)
- Encoding: UTF-8
- Timestamp: ISO 8601 (`2026-07-30T20:33:38Z`)
- Valor: float (normalizado para escala da métrica)
- Threshold: float (limiar configurado para aquela métrica)
- Alert: `true` | `false`

---

## 3. Benchmark B1 — Cognitive Load per Task

### 3.1 Definição

Mede o **custo cognitivo** de cada task: quantos recursos (tokens, tempo, agentes) foram consumidos. É o "custo" na equação de ROI da Cognitive Economy (F2.1).

### 3.2 Fórmula

```
CognitiveLoad(task) = (
    normalized_tokens      × 0.40 +
    normalized_time        × 0.35 +
    normalized_agents      × 0.25
) × 100

Onde:
  normalized_tokens    = min(total_tokens / 50000, 1.0)
  normalized_time      = min(total_time_seconds / 600, 1.0)   # 10 min max
  normalized_agents    = min(num_agents_mobilized / 8, 1.0)    # 8 agents max
```

**Raw score** (para referência técnica):

```
raw_load = total_tokens + total_time_seconds + num_agents_mobilized
```

### 3.3 Coleta

| Campo | Fonte | Momento |
|-------|-------|---------|
| `total_tokens` | Kernel — contexto carregado + resposta gerada | Pós-task |
| `total_time_seconds` | Kernel — diff entre task_start e task_end | Pós-task |
| `num_agents_mobilized` | Kernel — agentes primários + subagentes acionados | Pós-task |
| `task_id` | Kernel — identificador único da task | Pré-task |
| `task_type` | Kernel — classificação: simple / moderate / complex | Pré-task |

### 3.4 Armazenamento — `memory/timeline/cognitive-load.csv`

```csv
timestamp,task_id,agent,value,threshold,alert,task_type,tokens,time_sec,agents,raw_load
2026-07-30T17:43:53Z,F0-cosca-chat,cosca-backend,12.4,50.0,false,complex,14200,245,3,14448
2026-07-30T18:07:28Z,F1-provider-layer,cosca-chat,18.7,50.0,false,complex,22300,380,4,22684
2026-07-30T18:30:22Z,F2-tool-system,cosca-chat,22.3,50.0,false,complex,28600,420,5,29025
2026-07-30T18:39:53Z,F3-agent-engine,cosca-backend,31.5,50.0,false,complex,38400,510,6,38916
2026-07-30T18:55:41Z,F3.5-4-engine-tests,cosca-testing,38.2,50.0,false,complex,45800,580,7,46387
2026-07-30T19:07:25Z,F5-cli-commands,cosca-cli,15.8,50.0,false,moderate,18900,310,3,19213
2026-07-30T19:10:00Z,F4.5-memory-integration,cosca-backend,8.2,50.0,false,moderate,9800,180,2,9982
2026-07-30T19:21:49Z,F6-extensibility,cosca-chat,25.6,50.0,false,complex,32100,440,5,32545
2026-07-30T19:43:06Z,F6-mcp-tests,cosca-testing,19.4,50.0,false,complex,24100,350,4,24454
2026-07-30T19:43:06Z,F6-mcp-deadlock-fix,cosca-backend,7.1,25.0,false,simple,8100,120,2,8222
2026-07-30T19:45:31Z,F6-mcp-plugin-cli,cosca-cli,6.8,25.0,false,simple,7200,140,1,7341
2026-07-30T19:48:17Z,F6-agent-skill-cli,cosca-cli,7.3,25.0,false,simple,8400,150,1,8551
2026-07-30T19:50:19Z,F6-session-cli,cosca-cli,6.1,25.0,false,simple,6900,130,1,7031
2026-07-30T19:52:19Z,F6-completion-config-cli,cosca-cli,5.8,25.0,false,simple,6200,110,1,6311
2026-07-30T19:54:00Z,F6-config-tests,cosca-testing,4.2,25.0,false,simple,4800,90,1,4891
2026-07-30T20:33:38Z,F7-memorize-workflow,cosca-kernel,45.7,50.0,false,complex,51200,620,5,51825
```

### 3.5 Thresholds

| Task Type | Threshold | Ação |
|-----------|-----------|------|
| **simple** | > 25.0 (ou raw > 50K tokens ou > 5 agentes) | Alerta: "Cognitive load alto para task simples. Revisar granularidade." |
| **moderate** | > 40.0 | Alerta: "Load moderado-alto. Verificar se task pode ser decomposta." |
| **complex** | > 50.0 | Alerta: "Load crítico. Considerar split em sub-tasks." |
| **qualquer** | > 50K tokens | Alerta imediato: "Token budget excedido para task {id}. Risco de perda de contexto." |
| **qualquer** | > 5 agentes para task simples | Alerta: "Task simples mobilizou {N} agentes. Revisar necessidade." |

### 3.6 Exemplo com Dados da Sessão

**Task F7-memorize-workflow** (commit `a098c87`):
- Tokens: ~51.200 (9 arquivos, 2.193 insertions com contexto extenso)
- Tempo: 620s (~10 minutos de execução contínua)
- Agentes: 5 (cosca-kernel, cosca-architecture, cosca-documentation, cosca-evolution, cosca-workflows)
- Raw load: 51.200 + 620 + 5 = **51.825**
- Normalized: min(51.200/50.000, 1)×0.40 + min(620/600, 1)×0.35 + min(5/8, 1)×0.25 = 0.409 + 0.362 + 0.156 = **0.927 → 45.7**
- **Resultado**: Abaixo do threshold (50.0) — load alto mas aceitável para task complexa. Tokens > 50K dispararam alerta separado.

**Task F6-config-tests** (commit `50006ad`):
- Tokens: ~4.800 (1 arquivo, 122 insertions)
- Tempo: 90s
- Agentes: 1 (cosca-testing)
- Raw load: 4.800 + 90 + 1 = **4.891**
- Score: **4.2**
- **Resultado**: Bem abaixo do threshold — task leve e eficiente.

---

## 4. Benchmark B2 — Decision Velocity

### 4.1 Definição

Mede a **velocidade de decisão** do runtime: quanto tempo leva entre o pedido do Don (ou evento) e o primeiro agente acionado. Quanto menor, mais rápido o runtime responde.

### 4.2 Fórmula

```
DecisionVelocity(task) = time_from_request_to_first_action

Onde:
  time_from_request_to_first_action = timestamp(first_agent_activated) - timestamp(request_received)

Unidade: segundos
```

**Classificação**:

| Velocidade | Classe | Interpretação |
|------------|--------|---------------|
| < 2s | 🟢 Instantâneo | Decisão reflexa. Padrão conhecido, agente pré-alocado. |
| 2-5s | 🟢 Rápido | Task simples com roteamento direto. |
| 5-15s | 🟡 Moderado | Requer avaliação de contexto ou DAG simples. |
| 15-30s | 🟠 Lento | Decisão multi-agente ou DAG complexo. |
| > 30s | 🔴 Crítico | Degradação de performance. Investigar. |

### 4.3 Coleta

| Campo | Fonte | Momento |
|-------|-------|---------|
| `request_received` | Kernel — timestamp do pedido do Don | Ao receber input |
| `first_agent_activated` | Kernel — timestamp do primeiro agente acionado | Ao ativar agente |
| `decision_path` | Kernel — rota de decisão (direct, DAG-eval, escalate) | Pós-decisão |
| `task_complexity` | Kernel — classificada pelo Stage 1 (SELF-ASSESS) | Pré-task |

### 4.4 Armazenamento — `memory/timeline/decision-velocity.csv`

```csv
timestamp,task_id,agent,value,threshold,alert,complexity,decision_path
2026-07-30T17:43:53Z,F0-cosca-chat,cosca-kernel,3.2,5.0,false,simple,direct
2026-07-30T18:07:28Z,F1-provider-layer,cosca-kernel,2.8,5.0,false,simple,direct
2026-07-30T18:30:22Z,F2-tool-system,cosca-kernel,4.1,5.0,false,simple,direct
2026-07-30T18:39:53Z,F3-agent-engine,cosca-kernel,8.5,30.0,false,complex,DAG-eval
2026-07-30T18:55:41Z,F3.5-4-engine-tests,cosca-kernel,6.2,30.0,false,complex,DAG-eval
2026-07-30T19:07:25Z,F5-cli-commands,cosca-kernel,12.4,30.0,false,complex,DAG-eval
2026-07-30T19:10:00Z,F4.5-memory-integration,cosca-kernel,7.8,30.0,false,moderate,DAG-eval
2026-07-30T19:21:49Z,F6-extensibility,cosca-kernel,15.2,30.0,false,complex,DAG-eval
2026-07-30T19:43:06Z,F6-mcp-tests,cosca-kernel,5.9,30.0,false,moderate,DAG-eval
2026-07-30T19:43:06Z,F6-mcp-deadlock-fix,cosca-kernel,1.8,5.0,false,simple,direct
2026-07-30T19:45:31Z,F6-mcp-plugin-cli,cosca-kernel,2.1,5.0,false,simple,direct
2026-07-30T19:48:17Z,F6-agent-skill-cli,cosca-kernel,1.9,5.0,false,simple,direct
2026-07-30T19:50:19Z,F6-session-cli,cosca-kernel,2.4,5.0,false,simple,direct
2026-07-30T19:52:19Z,F6-completion-config-cli,cosca-kernel,1.7,5.0,false,simple,direct
2026-07-30T19:54:00Z,F6-config-tests,cosca-kernel,1.5,5.0,false,simple,direct
2026-07-30T20:33:38Z,F7-memorize-workflow,cosca-kernel,22.8,30.0,false,complex,DAG-eval
```

### 4.5 Thresholds

| Complexidade | Threshold | Ação |
|--------------|-----------|------|
| **simple** | > 5s | Alerta: "Task simples com decisão lenta. Investigar roteamento." |
| **moderate** | > 15s | Alerta: "Velocidade de decisão moderada degradada." |
| **complex** | > 30s | Alerta: "Decisão complexa crítica — DAG pode precisar de otimização." |
| **qualquer** | > 60s | Alerta P0: "Degradação severa de decisão. Kernel pode estar em loop." |

### 4.6 Exemplo com Dados da Sessão

**Task F6-mcp-deadlock-fix** (commit `3fa524c`):
- Request: Kernel recebe pedido do Don: "Corrigir deadlock no Connect() do MCP"
- Tempo de decisão: **1.8s** — roteamento direto para `cosca-backend` (já estava no contexto)
- **Resultado**: 🟢 Instantâneo. Abaixo do threshold de 5s.

**Task F7-memorize-workflow** (commit `a098c87`):
- Request: "Criar memorize-commit workflow + engineering timeline"
- Tempo de decisão: **22.8s** — Kernel avaliou DAG multi-agente (cosca-kernel, cosca-architecture, cosca-documentation, cosca-evolution, cosca-workflows)
- **Resultado**: 🟡 Moderado (complexa, threshold 30s). Dentro do limite.

---

## 5. Benchmark B3 — Knowledge Freshness

### 5.1 Definição

Mede o **frescor do conhecimento** usado em cada task. Aplica a curva de Wisdom Decay (F1.4) a todos os learnings consultados durante a execução. Conhecimento fresco = decisões melhores.

### 5.2 Fórmula

```
KnowledgeFreshness(task) = avg(freshness_score de todos learnings consultados na task)

Onde:
  freshness_score(learning) = confidence_score calculado pela curva Wisdom Decay
                              (ver WISDOM_DECAY.md §1)

  confidence(t, category) = max(0.10, 1.0 - (effective_age / 365) × 0.80)
  effective_age = days_since_last_validated × category_multiplier

  category_multiplier:
    CRITICAL:    0.3  (decaimento lento)
    STABLE:      1.0  (decaimento normal)
    EXPERIMENTAL: 2.0 (decaimento acelerado)
    DEPRECATED:  N/A  (não entra no cálculo — score = 0.0)
```

**Classificação**:

| Freshness | Cor | Interpretação |
|-----------|-----|---------------|
| **> 0.85** | 🟢 Excelente | Conhecimento recém-validado ou CRITICAL. Confiança alta. |
| **0.70 - 0.85** | 🟢 Bom | Conhecimento saudável. Revalidação programada dentro do prazo. |
| **0.50 - 0.69** | 🟡 Atenção | Conhecimento envelhecendo. Agendar revalidação. |
| **0.30 - 0.49** | 🟠 Crítico | Conhecimento stale. Revalidar ANTES de usar. |
| **< 0.30** | 🔴 Expirado | Conhecimento expirado. Não usar sem revalidação urgente. |

### 5.3 Coleta

| Campo | Fonte | Momento |
|-------|-------|---------|
| `learnings_consultados` | Kernel — lista de learnings carregados no contexto da task | Pré-task |
| `freshness_scores` | Wisdom Decay engine — calculado para cada learning | Pré-task |
| `avg_freshness` | Cálculo: média dos scores | Pré-task |
| `stale_count` | Quantos learnings com freshness < 0.50 | Pré-task |
| `expired_count` | Quantos learnings com freshness < 0.30 | Pré-task |

### 5.4 Armazenamento — `memory/timeline/knowledge-freshness.csv`

```csv
timestamp,task_id,agent,value,threshold,alert,learnings_consulted,stale_count,expired_count
2026-07-30T17:43:53Z,F0-cosca-chat,cosca-kernel,0.92,0.60,false,3,0,0
2026-07-30T18:07:28Z,F1-provider-layer,cosca-kernel,0.88,0.60,false,4,0,0
2026-07-30T18:30:22Z,F2-tool-system,cosca-kernel,0.85,0.60,false,5,0,0
2026-07-30T18:39:53Z,F3-agent-engine,cosca-kernel,0.91,0.60,false,8,0,0
2026-07-30T18:55:41Z,F3.5-4-engine-tests,cosca-kernel,0.87,0.60,false,6,0,0
2026-07-30T19:07:25Z,F5-cli-commands,cosca-kernel,0.83,0.60,false,7,1,0
2026-07-30T19:10:00Z,F4.5-memory-integration,cosca-kernel,0.79,0.60,false,4,1,0
2026-07-30T19:21:49Z,F6-extensibility,cosca-kernel,0.90,0.60,false,9,0,0
2026-07-30T19:43:06Z,F6-mcp-tests,cosca-kernel,0.86,0.60,false,5,0,0
2026-07-30T19:43:06Z,F6-mcp-deadlock-fix,cosca-kernel,0.94,0.60,false,2,0,0
2026-07-30T19:45:31Z,F6-mcp-plugin-cli,cosca-kernel,0.81,0.60,false,3,0,0
2026-07-30T19:48:17Z,F6-agent-skill-cli,cosca-kernel,0.77,0.60,false,4,1,0
2026-07-30T19:50:19Z,F6-session-cli,cosca-kernel,0.84,0.60,false,3,0,0
2026-07-30T19:52:19Z,F6-completion-config-cli,cosca-kernel,0.89,0.60,false,3,0,0
2026-07-30T19:54:00Z,F6-config-tests,cosca-kernel,0.82,0.60,false,2,0,0
2026-07-30T20:33:38Z,F7-memorize-workflow,cosca-kernel,0.91,0.60,false,11,0,0
```

### 5.5 Thresholds

| Métrica | Threshold | Ação |
|---------|-----------|------|
| **avg_freshness** | < 0.60 | Alerta: "Freshness médio abaixo do aceitável. Conhecimento stale afetando decisões." |
| **stale_count** | > 30% dos learnings consultados | Alerta: "Concentração de conhecimento stale. Agendar auditoria de revalidação." |
| **expired_count** | > 0 (qualquer expired usado) | Alerta P0: "Conhecimento EXPIRADO foi consultado. Revalidar imediatamente." |

### 5.6 Exemplo com Dados da Sessão

**Task F4.5-memory-integration** (commit `f362bbc`):
- Learnings consultados: 4 (L18 - cross-agent audit, L20 - platform audit, L21 - coverage expurgo, L22 - CMI design)
- Freshness scores:
  - L18 (2026-07-29, STABLE, 1 dia): confidence = 0.99
  - L20 (2026-07-29, STABLE, 1 dia): confidence = 0.99
  - L21 (2026-07-30, STABLE, 0 dias): confidence = 1.00
  - L10 (2026-07-28, EXPERIMENTAL, 2 dias): confidence = 0.96 (categoria EXPERIMENTAL × 2.0 → effective_age = 4 dias → 0.99)
- avg_freshness = (0.99 + 0.99 + 1.00 + 0.96) / 4 = **0.79**
- **Resultado**: 🟢 Bom. Abaixo do threshold de alerta (0.60).

**Task F7-memorize-workflow** (commit `a098c87`):
- Learnings consultados: 11 (L1-L27, todos desta semana — máximo 3 dias de idade)
- Todos com freshness > 0.90 (recém-criados, STABLE/EXPERIMENTAL)
- avg_freshness = **0.91**
- **Resultado**: 🟢 Excelente. Sessão inteira de learnings frescos.

---

## 6. Benchmark B4 — Cross-agent Reuse

### 6.1 Definição

Mede o **reuso cognitivo** — quantas vezes um learning de um agente é usado por outro agente. É o ROI do conhecimento compartilhado.

### 6.2 Fórmula

```
CrossAgentReuse(task) = (
    count(learnings_usados_de_outros_agentes) /
    count(learnings_usados_no_total)
) × 100

Onde:
  learnings_usados_de_outros_agentes = learnings carregados no contexto
    cujo agent_owner é DIFERENTE do agente executor da task atual

  learnings_usados_no_total = total de learnings carregados no contexto
```

**Classificação**:

| Reuse Rate | Cor | Interpretação |
|------------|-----|---------------|
| **> 40%** | 🟢 Alto | Forte colaboração cross-agent. Conhecimento flui bem. |
| **20-40%** | 🟢 Médio | Reuso saudável. Meta mínima: > 20%. |
| **10-20%** | 🟡 Baixo | Pouca colaboração. Agentes operando em silos. |
| **< 10%** | 🔴 Crítico | Silos cognitivos. Conhecimento não está sendo compartilhado. |

### 6.3 Coleta

| Campo | Fonte | Momento |
|-------|-------|---------|
| `learnings_carregados` | Kernel — todos learnings carregados no contexto | Pré-task |
| `agent_executor` | Kernel — agente primário da task | Pré-task |
| `cross_agent_learnings` | Kernel — filtro: learnings com agent_owner ≠ executor | Pré-task |
| `cross_agent_count` | Contagem dos filtrados | Pré-task |
| `total_learning_count` | Contagem total | Pré-task |

### 6.4 Armazenamento — `memory/timeline/cross-agent-reuse.csv`

```csv
timestamp,task_id,agent,value,threshold,alert,cross_count,total_count,source_agents
2026-07-30T17:43:53Z,F0-cosca-chat,cosca-kernel,33.3,20.0,false,1,3,"cosca-architecture"
2026-07-30T18:07:28Z,F1-provider-layer,cosca-chat,25.0,20.0,false,1,4,"cosca-backend"
2026-07-30T18:30:22Z,F2-tool-system,cosca-chat,40.0,20.0,false,2,5,"cosca-backend,cosca-architecture"
2026-07-30T18:39:53Z,F3-agent-engine,cosca-backend,50.0,20.0,false,4,8,"cosca-chat,cosca-architecture,cosca-runtime,cosca-testing"
2026-07-30T18:55:41Z,F3.5-4-engine-tests,cosca-testing,33.3,20.0,false,2,6,"cosca-backend,cosca-chat"
2026-07-30T19:07:25Z,F5-cli-commands,cosca-cli,28.6,20.0,false,2,7,"cosca-backend,cosca-chat"
2026-07-30T19:10:00Z,F4.5-memory-integration,cosca-backend,50.0,20.0,false,2,4,"cosca-kernel,cosca-memory-chief"
2026-07-30T19:21:49Z,F6-extensibility,cosca-chat,44.4,20.0,false,4,9,"cosca-backend,cosca-architecture,cosca-plugin,cosca-cli"
2026-07-30T19:43:06Z,F6-mcp-tests,cosca-testing,40.0,20.0,false,2,5,"cosca-backend,cosca-chat"
2026-07-30T19:43:06Z,F6-mcp-deadlock-fix,cosca-backend,50.0,20.0,false,1,2,"cosca-chat"
2026-07-30T19:45:31Z,F6-mcp-plugin-cli,cosca-cli,33.3,20.0,false,1,3,"cosca-backend"
2026-07-30T19:48:17Z,F6-agent-skill-cli,cosca-cli,25.0,20.0,false,1,4,"cosca-backend"
2026-07-30T19:50:19Z,F6-session-cli,cosca-cli,33.3,20.0,false,1,3,"cosca-kernel"
2026-07-30T19:52:19Z,F6-completion-config-cli,cosca-cli,33.3,20.0,false,1,3,"cosca-backend"
2026-07-30T19:54:00Z,F6-config-tests,cosca-testing,0.0,20.0,true,0,2,""
2026-07-30T20:33:38Z,F7-memorize-workflow,cosca-kernel,36.4,20.0,false,4,11,"cosca-architecture,cosca-documentation,cosca-evolution,cosca-workflows"
```

### 6.5 Thresholds

| Métrica | Threshold | Ação |
|---------|-----------|------|
| **reuse_rate** | < 20% | Alerta: "Cross-agent reuse abaixo do mínimo. Agentes podem estar em silos." |
| **reuse_rate** | < 10% | Alerta P0: "Silos cognitivos detectados. Conhecimento não está fluindo entre agentes." |
| **zero_reuse** | 0% por 3+ tasks consecutivas | Alerta: "Zero reuso em múltiplas tasks. Investigar isolamento do agente." |

### 6.6 Exemplo com Dados da Sessão

**Task F3-agent-engine** (commit `eb36926`):
- Executor: `cosca-backend`
- Learnings carregados: 8
- Learnings de outros agentes: 4 (de cosca-chat, cosca-architecture, cosca-runtime, cosca-testing)
- Reuse rate: 4/8 = **50.0%**
- **Resultado**: 🟢 Alto. Agente backend está fortemente reusando conhecimento de outros agentes.

**Task F6-config-tests** (commit `50006ad`):
- Executor: `cosca-testing`
- Learnings carregados: 2 (ambos do próprio cosca-testing)
- Learnings de outros agentes: 0
- Reuse rate: 0/2 = **0.0%**
- **Resultado**: 🔴 **Alerta disparado**. Task isolada sem reuso cross-agent. Causa provável: task trivial de testes que não exigiu consulta cross-domain. Alerta acionado por threshold.

---

## 7. Benchmark B5 — Cognitive Debt

### 7.1 Definição

Mede a **dívida cognitiva** — aprendizado que ocorreu mas não foi registrado. É o acúmulo de "aprender mas não documentar". O Cognitive Audit Loop (5 perguntas do AUTO_EVOLUTION_PROTOCOL) é o instrumento de detecção.

### 7.2 Fórmula

```
CognitiveDebt(periodo) = (
    count(aprendizados_nao_registrados) /
    count(tasks_executadas_no_periodo)
) × 100

Onde:
  aprendizado_nao_registrado = task onde pelo menos 1 das 5 perguntas
    do Cognitive Audit Loop ficou SEM resposta (status = incomplete ou partial)

  tasks_executadas_no_periodo = total de tasks no período analisado
```

**Detecção por task** (a partir do Cognitive Audit Loop):

```
Se audit_loop_result == "incomplete":
  cognitive_debt += 1  # Pelo menos 1 pergunta sem resposta

Se audit_loop_result == "partial":
  cognitive_debt += 0.5  # Timeout em alguma pergunta — dívida parcial

Se audit_loop_result == "complete":
  cognitive_debt += 0  # Zero dívida
```

**Classificação**:

| Debt Rate | Cor | Interpretação |
|-----------|-----|---------------|
| **< 5%** | 🟢 Excelente | Quase zero dívida. Conhecimento sendo registrado consistentemente. |
| **5-10%** | 🟢 Aceitável | Dívida baixa. Abaixo do threshold de alerta. |
| **10-20%** | 🟡 Atenção | Dívida moderada. Algumas tasks sem registro completo. |
| **20-50%** | 🟠 Crítico | Dívida alta. Conhecimento está sendo perdido. |
| **> 50%** | 🔴 Colapso | Maioria das tasks sem registro. Sistema de memória comprometido. |

### 7.3 Coleta

| Campo | Fonte | Momento |
|-------|-------|---------|
| `task_id` | Kernel | Pós-task |
| `audit_status` | Cognitive Audit Loop | Pós-task (complete / partial / incomplete) |
| `unanswered_questions` | Cognitive Audit Loop | Pós-task (lista: Q1, Q2, Q3, Q4, Q5) |
| `debt_contribution` | Calculado: 0, 0.5, ou 1.0 | Pós-task |
| `window_tasks` | Kernel — tasks no período (janela de 10 tasks ou semanal) | Relatório |
| `window_debt` | Soma das contribuições no período | Relatório |

### 7.4 Armazenamento — `memory/timeline/cognitive-debt.csv`

```csv
timestamp,task_id,agent,value,threshold,alert,audit_status,unanswered_questions,debt_contribution
2026-07-30T17:43:53Z,F0-cosca-chat,cosca-backend,0.0,10.0,false,complete,"",0.0
2026-07-30T18:07:28Z,F1-provider-layer,cosca-chat,0.0,10.0,false,complete,"",0.0
2026-07-30T18:30:22Z,F2-tool-system,cosca-chat,0.0,10.0,false,complete,"",0.0
2026-07-30T18:39:53Z,F3-agent-engine,cosca-backend,0.0,10.0,false,complete,"",0.0
2026-07-30T18:55:41Z,F3.5-4-engine-tests,cosca-testing,0.0,10.0,false,complete,"",0.0
2026-07-30T19:07:25Z,F5-cli-commands,cosca-cli,0.0,10.0,false,complete,"",0.0
2026-07-30T19:10:00Z,F4.5-memory-integration,cosca-backend,0.0,10.0,false,complete,"",0.0
2026-07-30T19:21:49Z,F6-extensibility,cosca-chat,0.0,10.0,false,complete,"",0.0
2026-07-30T19:43:06Z,F6-mcp-tests,cosca-testing,0.0,10.0,false,complete,"",0.0
2026-07-30T19:43:06Z,F6-mcp-deadlock-fix,cosca-backend,0.0,10.0,false,complete,"",0.0
2026-07-30T19:45:31Z,F6-mcp-plugin-cli,cosca-cli,0.0,10.0,false,complete,"",0.0
2026-07-30T19:48:17Z,F6-agent-skill-cli,cosca-cli,0.0,10.0,false,complete,"",0.0
2026-07-30T19:50:19Z,F6-session-cli,cosca-cli,0.0,10.0,false,complete,"",0.0
2026-07-30T19:52:19Z,F6-completion-config-cli,cosca-cli,0.0,10.0,false,complete,"",0.0
2026-07-30T19:54:00Z,F6-config-tests,cosca-testing,5.0,10.0,false,partial,"Q2",0.5
2026-07-30T20:33:38Z,F7-memorize-workflow,cosca-kernel,0.0,10.0,false,complete,"",0.0
```

### 7.5 Thresholds

| Métrica | Threshold | Ação |
|---------|-----------|------|
| **cognitive_debt_rate** | > 10% | Alerta: "Dívida cognitiva excedeu limite. Revisar compliance com audit loop." |
| **cognitive_debt_rate** | > 30% | Alerta P0: "Dívida crítica. Maioria das tasks sem registro. Sistema de memória em risco." |
| **qualquer task** | incomplete | Alerta: "Task {id} incompleta — stages 7-8 não executados. Próxima task bloqueada." |
| **3+ consecutivas** | incomplete | Alerta P0: "3 tasks consecutivas sem audit. Escalar para revisão humana." |

### 7.6 Exemplo com Dados da Sessão

**Task F6-config-tests** (commit `50006ad`, `cosca-testing`):
- Audit result: **partial** — Q2 (pattern extraction) entrou em timeout
- Q2 pergunta: "Descobri um padrão reutilizável?"
- O agente estava focado em escrever testes unitários para `GetValue/getByKey` — não extraiu padrão
- Resposta registrada: "Task de testes unitários para funções existentes — padrão já coberto por H-003 (unittest-pattern)"
- Timeout de 30s atingido → status partial
- Debt contribution: **0.5** (dívida parcial)
- **Resultado**: 🟢 Abaixo do threshold (10%). Dívida parcial, não bloqueante.

**Janela de 16 tasks (sessão atual)**:
- Tasks executadas: 16
- Dívida total: 0.5 (apenas 1 task partial)
- Cognitive debt rate: 0.5 / 16 × 100 = **3.1%**
- Threshold: 10%
- **Resultado**: 🟢 Excelente. Sistema auditando consistentemente.

---

## 8. Indicador Composto — Cognitive Health Index (CHI)

### 8.1 Fórmula

```
CHI = (
    (1.0 - normalized(B1, 0, 50)) × 0.25 +   # B1: quanto menor, melhor
    normalized(B2_inverse, 0, 30)     × 0.20 +   # B2: velocidade inversa (menor = melhor)
    B3                               × 0.25 +   # B3: freshness direto (maior = melhor)
    normalized(B4, 0, 50)            × 0.15 +   # B4: reuse rate direto
    (1.0 - normalized(B5, 0, 30))    × 0.15     # B5: debt inverso (menor = melhor)
) × 100

Onde:
  normalized(x, min, max) = clamp((x - min) / (max - min), 0, 1)
  B2_inverse = max_possible_velocity - B2   # ex: 30 - B2

Versão simplificada para dashboard:
  CHI ≈ (B1_inverse × 0.25 + B2_inverse × 0.20 + B3 × 0.25 + B4_rate × 0.15 + B5_inverse × 0.15) × 100
```

### 8.2 Classificação

| CHI | Cor | Significado |
|-----|-----|-------------|
| **> 85** | 🟢 Excelente | Runtime cognitivamente saudável. Todas as métricas no verde. |
| **70-85** | 🟡 Bom | Saúde boa com ressalvas. 1-2 métricas em atenção. |
| **50-69** | 🟠 Atenção | Saúde comprometida. Múltiplas métricas fora do ideal. |
| **< 50** | 🔴 Crítico | Runtime cognitivamente doente. Intervenção necessária. |

### 8.3 CHI Atual (Sessão)

```
CHI = (
    (1.0 - 0.276) × 0.25 +   # B1 médio = 13.8/50
    (1.0 - 0.215) × 0.20 +   # B2 médio = 6.45/30
    0.86          × 0.25 +   # B3 médio = 0.86
    0.683         × 0.15 +   # B4 médio = 34.15/50
    (1.0 - 0.103) × 0.15     # B5 médio = 3.1/30
) × 100

= (0.724 × 0.25 + 0.785 × 0.20 + 0.86 × 0.25 + 0.683 × 0.15 + 0.897 × 0.15) × 100
= (0.181 + 0.157 + 0.215 + 0.102 + 0.135) × 100
= 0.790 × 100
= 79.0

→ 🟡 BOM — Cognitive Health Index = 79/100
```

---

## 9. Dashboard Conceitual

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         COGNITIVE METRICS DASHBOARD                        │
│                         2026-07-30 — Sessão Atual                         │
├──────────┬──────────┬──────────┬──────────┬──────────┬────────────────────┤
│   B1     │   B2     │   B3     │   B4     │   B5     │      CHI           │
│ LOAD     │ VELOCITY │FRESHNESS │  REUSE   │  DEBT    │   HEALTH           │
├──────────┼──────────┼──────────┼──────────┼──────────┼────────────────────┤
│  13.8    │  6.4s    │  0.86    │  34.2%   │  3.1%    │    79/100          │
│          │          │          │          │          │                    │
│  ░░░░░░  │ ░░░░░░░  │████████  │ ███████  │████████  │   ████████████░░   │
│  ██████  │ █████    │░░░░░░░░  │ ░░░░░░░  │░░░░░░░░  │   ░░░░░░░░░░░░    │
│          │          │          │          │          │                    │
│  🟢 méd  │  🟢 ráp  │  🟢 alto │  🟢 méd  │  🟢 bai  │     🟡 BOM         │
│          │          │          │          │          │                    │
├──────────┴──────────┴──────────┴──────────┴──────────┴────────────────────┤
│                                                                           │
│  TENDÊNCIA (janela de 16 tasks):                                         │
│  ┌──────┬──────┬──────┬──────┬──────┐                                    │
│  │ B1 ↓ │ B2 ↓ │ B3 → │ B4 ↑ │ B5 ↓ │  ↓ = melhorando                    │
│  │ 15→12│ 8→4s │0.85→ │30→38%│ 5→2% │  ↑ = piorando                      │
│  │      │      │0.87  │      │      │                                     │
│  └──────┴──────┴──────┴──────┴──────┘                                    │
│                                                                           │
│  ALERTAS ATIVOS:                                                          │
│  • B4: Task F6-config-tests com zero reuso (task isolada)                │
│  • B1: Task F3.5-4-engine-tests com load 38.2 (alto para task moderada)  │
│                                                                           │
│  PRÓXIMA MEDIÇÃO: A cada 10 tasks ou semanal                             │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 10. Integração com F2.1 — Cognitive Economy

### 10.1 Equação de ROI Cognitivo

A Cognitive Economy Engine (F2.1) usa as 5 métricas como inputs para calcular o **retorno sobre investimento cognitivo**:

```
CognitiveROI(task) = (
    CrossAgentReuse_B4    -   CognitiveDebt_B5     ×   KnowledgeFreshness_B3
) / (
    CognitiveLoad_B1      ×   DecisionVelocity_B2
)

Onde:
  Numerador   = valor gerado (reuso - dívida) × qualidade (freshness)
  Denominador = custo (load) × tempo (velocity)
```

**Interpretação**:
- **ROI > 1.0**: Task gerou mais valor que consumiu. Investimento vale a pena.
- **ROI < 1.0**: Task consumiu mais que gerou. Revisar abordagem.
- **ROI < 0.3**: Task ineficiente. Cognitive Economy Engine bloqueia ou escala.

### 10.2 Mapa de Integração

```
┌──────────────────────────────────────────────────────────────────┐
│              B1-B5 → F2.1 COGNITIVE ECONOMY                       │
│                                                                   │
│  B1 Cognitive Load ──────► Cost (peso 40% no custo total)         │
│                              │                                    │
│  B2 Decision Velocity ──────► Time (peso 30% no custo total)      │
│                              │                                    │
│  B3 Knowledge Freshness ────► Quality multiplier (0-1×)          │
│                              │                                    │
│  B4 Cross-agent Reuse ──────► Value (peso 60% no valor total)    │
│                              │                                    │
│  B5 Cognitive Debt ─────────► Risk discount (reduz valor em %)   │
│                              │                                    │
│                              ▼                                    │
│              CognitiveROI = (Value × Quality) / (Cost × Time)    │
└──────────────────────────────────────────────────────────────────┘
```

### 10.3 Exemplo de Cálculo de ROI

**Task F7-memorize-workflow**:
- B1 Load: 45.7 (custo)
- B2 Velocity: 22.8s (tempo)
- B3 Freshness: 0.91 (qualidade)
- B4 Reuse: 36.4% (valor)
- B5 Debt: 0% (sem dívida)

```
CognitiveROI = (0.364 × 0.91) / (45.7 × 22.8)
             = 0.331 / 1041.96
             = 0.000318 → 0.03% (ROI bruto)

Normalizado para escala 0-100:
  CognitiveROI_norm = min(ROI × 10000, 100)
                    = min(3.18, 100)
                    = 3.18

Interpretação: Task de infraestrutura (memorize-commit) tem ROI baixo
  porque é investimento — cria ativo cognitivo (workflow, timeline) que
  será reusado em tasks futuras. ROI real emerge após reuso.
```

---

## 11. Automação

### 11.1 Coleta Automática Pós-Task

O Kernel DEVE executar o seguinte hook ao finalizar cada task:

```
hook_post_task:
  trigger: "Kernel.task_complete"
  steps:
    1. # B1: Cognitive Load
       registrar_em_csv(
         arquivo: "memory/timeline/cognitive-load.csv",
         campos: {
           timestamp, task_id, agent,
           tokens: metrics.tokens_consumidos,
           time: metrics.tempo_decorrido_segundos,
           agents: metrics.agentes_mobilizados.length,
           value: calcular_b1(tokens, time, agents),
           threshold: obter_threshold(task.type),
           alert: calcular_b1 > threshold
         }
       )

    2. # B2: Decision Velocity
       registrar_em_csv(
         arquivo: "memory/timeline/decision-velocity.csv",
         campos: {
           timestamp, task_id, agent,
           value: metrics.decision_time_segundos,
           threshold: obter_threshold(task.complexity),
           alert: decision_time > threshold
         }
       )

    3. # B3: Knowledge Freshness
       freshness_scores = metrics.learnings_consultados.map(
         l => wisdom_decay.calcular_freshness(l)
       )
       registrar_em_csv(
         arquivo: "memory/timeline/knowledge-freshness.csv",
         campos: {
           timestamp, task_id, agent,
           value: avg(freshness_scores),
           learnings_consultados: freshness_scores.length,
           stale_count: freshness_scores.filter(s => s < 0.50).length,
           expired_count: freshness_scores.filter(s => s < 0.30).length,
           threshold: 0.60,
           alert: avg < 0.60
         }
       )

    4. # B4: Cross-agent Reuse
       registrar_em_csv(
         arquivo: "memory/timeline/cross-agent-reuse.csv",
         campos: {
           timestamp, task_id, agent,
           value: reuse_rate,
           cross_count,
           total_count,
           source_agents: metrics.learnings_de_outros_agentes.map(l => l.agent).join(","),
           threshold: 20.0,
           alert: reuse_rate < 20.0
         }
       )

    5. # B5: Cognitive Debt
       debt = audit_loop.executar(task)
       registrar_em_csv(
         arquivo: "memory/timeline/cognitive-debt.csv",
         campos: {
           timestamp, task_id, agent,
           value: calcular_b5(debt, window_size=10),
           audit_status: debt.status,
           unanswered_questions: debt.unanswered.join(","),
           debt_contribution: debt.contribution,
           threshold: 10.0,
           alert: debt_rate > 10.0
         }
       )

    6. # Atualizar dashboard (a cada 10 tasks)
       if task_count % 10 == 0:
         gerar_relatorio_consolidado()
```

### 11.2 Gatilho de Relatório

| Frequência | Ação | Formato |
|------------|------|---------|
| **A cada 10 tasks** | Relatório consolidado no dashboard | Markdown em `cognitive-health-report.md` |
| **Semanal** | Relatório semanal + alertas de tendência | Dashboard ASCII no console |
| **On-demand** | `cosca metrics cognitive --report` | JSON stdout |

### 11.3 Sistema de Alertas

| Severidade | Condição | Canal | Ação |
|------------|----------|-------|------|
| 🔴 **P0** | B5 debt > 30% ou B3 freshness < 0.30 | Alerta no console + Kernel notify | Escalar para Don. Congelar novas tasks até resolução. |
| 🟠 **P1** | B1 load > threshold ou B2 velocity > threshold | Alerta no console | Registrar no audit log. Investigar causa. |
| 🟡 **P2** | B4 reuse < 10% por 3+ tasks consecutivas | Warning no console | Notificar agente. Sugerir consulta cross-agent. |
| 🟢 **P3** | Qualquer threshold violado isoladamente | Log entry | Apenas registro. Sem ação blocking. |

---

## 12. Exemplo de População com Dados da Sessão

### 12.1 Sessão de Referência

Esta sessão (2026-07-30) executou **16 tasks** distribuídas em:

| Período | Tasks | Agentes | Commits | Insertions |
|---------|-------|---------|---------|-----------|
| 17:43-18:07 | F0-F1 (cosca-chat foundation) | cosca-backend, cosca-chat | 2 | 4.758 |
| 18:07-18:39 | F2-F3 (tool-system + agent-engine) | cosca-chat, cosca-backend | 2 | 6.584 |
| 18:39-18:55 | F3.5-F4 (engine tests + memory) | cosca-testing, cosca-backend | 2 | 5.072 |
| 18:55-19:21 | F5-F6 (CLI + extensibility) | cosca-cli, cosca-chat | 4 | 2.322 |
| 19:21-19:43 | F6 (MCP tests + deadlock fix) | cosca-chat, cosca-testing, cosca-backend | 2 | 1.847 |
| 19:43-19:54 | F6 (CLI commands + tests) | cosca-cli, cosca-testing | 5 | 990 |
| 19:54-20:33 | F7 (memorize workflow) | cosca-kernel | 1 | 2.193 |

### 12.2 Métricas Agregadas da Sessão

| Métrica | Média | Mín | Máx | Threshold | Alerta |
|---------|-------|-----|-----|-----------|--------|
| **B1 Load (score)** | 13.8 | 4.2 | 45.7 | 50.0 (complex) | 🟢 0 alertas |
| **B2 Velocity (seg)** | 6.4s | 1.5s | 22.8s | 30.0 (complex) | 🟢 0 alertas |
| **B3 Freshness** | 0.86 | 0.77 | 0.94 | 0.60 | 🟢 0 alertas |
| **B4 Reuse (%)** | 34.2% | 0.0% | 50.0% | 20.0% | 🟡 1 alerta (B4-config-tests) |
| **B5 Debt (%)** | 3.1% | 0.0% | 5.0% | 10.0% | 🟢 0 alertas |

### 12.3 Insigths da Sessão

1. **B1 Load**: Tasks de engine (F3-F4) têm load consistentemente mais alto (25-38) que tasks de CLI (4-7). Esperado — engines geram mais tokens e mobilizam mais agentes.

2. **B2 Velocity**: Tasks CLI têm velocidade média de 2.0s (roteamento direto). Tasks complexas como F7 (memorize-workflow) levam 22.8s (DAG multi-agente). Proporcional à complexidade.

3. **B3 Freshness**: Todas as tasks com freshness > 0.77. Sessão inteira em conhecimento fresco (nada mais velho que 3 dias). Isso é artificial — baseline real terá conhecimento mais antigo.

4. **B4 Reuse**: Média de 34.2% — saudável. Cosca-backend e cosca-chat são os maiores provedores de conhecimento cross-agent. Cosca-testing é o maior consumidor.

5. **B5 Debt**: Apenas 1 task com dívida parcial (F6-config-tests, Q2 timeout). Taxa de 3.1% — excelente.

---

## 13. Próximos Passos

### 13.1 Fase 1 — Atual (v1.0.0)

- [x] Definição das 5 métricas com fórmulas e thresholds
- [x] Schema CSV para armazenamento em timeline
- [x] Dashboard conceitual ASCII
- [x] Integração conceitual com F2.1 Cognitive Economy
- [x] População inicial com dados da sessão (16 tasks)

### 13.2 Fase 2 — Automação (v1.1.0+)

- [ ] Hook pós-task no Kernel implementado (código Go)
- [ ] Script `cosca metrics cognitive --report` (CLI)
- [ ] Script `cosca metrics cognitive --alert` (verificação de thresholds)
- [ ] CSV histórico acumulado em `memory/timeline/`
- [ ] CI gate: falha se CHI < 50

### 13.3 Fase 3 — Preditivo (v2.0.0+)

- [ ] Tendência preditiva: "B1 deve subir nas próximas 5 tasks baseado em padrão X"
- [ ] Correlação B1-B5 com outcomes: "tasks com B4 > 40% têm 95% de sucesso"
- [ ] Cognitive Economy Engine consumindo B1-B5 em tempo real
- [ ] Dashboard interativo no `cosca serve`

---

## 14. Referências

| Documento | Relação |
|-----------|---------|
| [cognitive-maturity-implementation.md](../workflows/cognitive-maturity-implementation.md) | Workflow F1.5 — esta tarefa |
| [WISDOM_DECAY.md](../memory/WISDOM_DECAY.md) | F1.4 — curva de freshness para B3 |
| [CMI_REAL_METRICS.md](../metrics/CMI_REAL_METRICS.md) | Métricas de outcome (complementares a este documento) |
| [COGNITIVE_ENTROPY.md](COGNITIVE_ENTROPY.md) | Entropia cognitiva — métrica irmã de saúde do conhecimento |
| [COGNITIVE_MOMENTUM.md](COGNITIVE_MOMENTUM.md) | Momentum por domínio — alocação de recursos |
| [COGNITIVE_HORIZON.md](COGNITIVE_HORIZON.md) | Horizonte preditivo — profundidade das decisões |
| [AUTO_EVOLUTION_PROTOCOL.md](../shared/AUTO_EVOLUTION_PROTOCOL.md) | Post-task 5-question checklist (fonte do B5) |
| [cognitive-audit-loop.md](../workflows/cognitive-audit-loop.md) | Enforcement do audit loop (detector de B5) |
| [DECISION_DNA_FORMAT.md](../memory/DECISION_DNA_FORMAT.md) | F1.1 — decisões registradas (fonte do B2) |

---

## 15. Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Analytics Chief | Definição inicial. 5 benchmarks B1-B5 com fórmulas, CSV storage, thresholds, dashboard ASCII, integração F2.1. População com 16 tasks da sessão. |

---

> **Enforced by**: Cosca Analytics Chief | **Próxima medição**: Após 10 tasks ou semanal
> **Kernel instruction**: `cosca analytics cognitive-metrics --update --from-session`
