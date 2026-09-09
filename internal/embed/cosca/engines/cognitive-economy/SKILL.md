# COGNITIVE ECONOMY ENGINE ★ F2.1 — Motor de Economia Cognitiva

> **Versão**: 2.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30 | **Atualizado**: 2026-07-30
> **Workflow**: `cosca-cognitive-economy`
> **Conceito**: C14 ★ — Conceito-Estrela da Arquitetura de Maturidade Cognitiva — **F2.1 é a ESTRELA da Fase 2**
> **Referências**: COGNITIVE_MATURITY.md §5 C14 | cognitive-maturity-implementation.md F2.1
> **Dependências**: F1.5 (cognitive-metrics.md) | F7.1 (Prediction Engine) | F7.2 (Trust Registry) | F1.6 (Cognitive Entropy)
> **CMI Impact**: Julgamento +5, Planejamento +8

---

## Índice

1. [Propósito e Definição](#1-propósito-e-definição)
2. [Fundamentos Teóricos](#2-fundamentos-teóricos)
3. [Componentes de CUSTO](#3-componentes-de-custo)
4. [Componentes de VALOR](#4-componentes-de-valor)
5. [ROI Cognitivo — A Equação ★](#5-roi-cognitivo--a-equação-)
6. [ROI Ajustado por Risco (com F1.6 Entropy)](#6-roi-ajustado-por-risco-com-f16-entropy)
7. [Pipeline Pós-Task](#7-pipeline-pós-task)
8. [Dashboard de ROI Cognitivo](#8-dashboard-de-roi-cognitivo)
9. [Exemplo com Dados Reais da Sessão](#9-exemplo-com-dados-reais-da-sessão)
10. [Integração com a Arquitetura Cosca](#10-integração-com-a-arquitetura-cosca)
11. [Configuração de Thresholds](#11-configuração-de-thresholds)
12. [Métricas do Próprio Engine](#12-métricas-do-próprio-engine)
13. [Automação e Performance](#13-automação-e-performance)
14. [Implementação](#14-implementação)
15. [Referências Cruzadas](#15-referências-cruzadas)
16. [Histórico](#16-histórico)

---

## 1. Propósito e Definição

### 1.1 O que é Cognitive Economy

O **Cognitive Economy Engine** é o motor que **mede o ROI cognitivo de cada ação** no Cosca Runtime. Ele transforma um sistema que apenas executa em um sistema que **calcula se valeu a pena executar**.

A equação fundamental:

```
ROI cognitivo = (valor_gerado - custo_total) / custo_total
```

### 1.2 Propósito

Cada tarefa no Cosca custa **tokens, tempo e atenção** — recursos escassos e não-renováveis. Cada tarefa gera **valor** — aprendizados, reuso de padrões, prevenção de erros. O Cognitive Economy Engine quantifica ambos os lados da equação para responder:

> **"Esta tarefa valeu o que custou?"**

O Kernel já demonstrou consciência implícita de custo em decisões anteriores — L17 (Token Bloat Audit), decisões de routing (delegar vs agir), awareness que spawnar 5 agentes custa 5× mais que 1. O Cognitive Economy Engine substitui a intuição por um **modelo matemático de ROI** que pesa sistematicamente:

- **O que esta task consumiu?** (custo_total)
- **O que esta task produziu?** (valor_total)
- **O custo justifica o valor?** (ROI)
- **O risco de conhecimento desorganizado reduziu o ROI real?** (entropy adjustment)

### 1.3 Filosofia

```
"Se você não pode medir o ROI de uma task,
 você não pode saber se ela valeu a pena.
 Se você não sabe se valeu a pena,
 você não pode decidir se deve repeti-la.

 ★ F2.1 — Cognitive Economy é a ESTRELA da Fase 2.
   Ela transforma o Cosca de um sistema que gasta
   em um sistema que investe."
   — Cosca Architecture Chief, 2026-07-30
```

---

## 2. Fundamentos Teóricos

### 2.1 Por que Economia Cognitiva?

O Cosca Runtime opera com 3 recursos escassos e não-renováveis (ou de renovação lenta) que formam a base do custo:

| Recurso | Natureza | Limite Típico | Custo do Excesso |
|---------|----------|---------------|------------------|
| **Tokens** | Cada token consumido por LLM custa dinheiro real ($) e latência | ~50K/sessão | Custo financeiro + timeout de contexto |
| **Tempo** | Tempo do Don é finito e não-recuperável | ~30 min/sessão | Fadiga de decisão, abandono da ferramenta |
| **Atenção** | Interrupções ao Don consomem o recurso mais valioso do sistema | 3 por sessão | Perda de confiança, microgerenciamento |

E 3 componentes de valor que justificam o investimento:

| Valor | Natureza | Exemplo |
|-------|----------|---------|
| **Aprendizado** | Conhecimento novo registrado que não existia antes | "Descoberta de padrão de race condition" |
| **Reuso** | Padrão já existente que foi reaplicado | "Cross-agent audit pattern reusado em 3 domínios" |
| **Prevenção** | Erro que foi evitado por ação proativa | "Dead code removal que preveniria bug futuro" |

### 2.2 A Equação de ROI Cognitivo — Visão Geral

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                  │
│    CUSTO                          VALOR                          │
│    ─────                          ─────                          │
│                                                                  │
│  custo_tokens      ◄─── token_price × (input + output)          │
│  custo_tempo       ◄─── time_seconds × opportunity_cost         │
│  custo_atencao     ◄─── num_agents × attention_cost             │
│        │                                │                       │
│        ▼                                ▼                       │
│  ┌──────────┐                   ┌──────────────┐                │
│  │custo_total│                   │ valor_total   │                │
│  └─────┬────┘                   └──────┬───────┘                │
│        │                               │                        │
│        └──────────┬────────────────────┘                        │
│                   ▼                                              │
│        ┌─────────────────────┐                                   │
│        │  ROI = (V - C) / C  │                                   │
│        └─────────────────────┘                                   │
│                   │                                              │
│                   ▼                                              │
│        ┌─────────────────────┐                                   │
│        │ ROI_ajustado = ROI  │                                   │
│        │ × (1 - entropy)     │  ◄─── F1.6 Cognitive Entropy     │
│        └─────────────────────┘                                   │
│                                                                  │
│                   ▼                                              │
│        ┌─────────────────────┐                                   │
│        │     Alerta se       │                                   │
│        │    ROI < 0%         │                                   │
│        └─────────────────────┘                                   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. Componentes de CUSTO

### 3.1 Definição

Para cada task, o custo total é calculado como:

```
custo_total = custo_tokens + custo_tempo + custo_atencao
```

### 3.2 custo_tokens — Custo de LLM

**O que mede**: Tokens consumidos em input (contexto carregado) + output (resposta gerada) para todas as chamadas de LLM envolvidas na task.

```
custo_tokens = (input_tokens + output_tokens) × token_price

Onde:
  input_tokens   = tokens de contexto carregado (system prompt + KERNEL.md + memory + task)
  output_tokens  = tokens de resposta gerada
  token_price    = $0.00001 por token ($0.01/K tokens) — preço médio ponderado entre providers
```

**Fontes de dados** (da F1.5 Cognitive Metrics — B1 Cognitive Load):

| Campo | Fonte | Momento |
|-------|-------|---------|
| `input_tokens` | Kernel — contexto carregado | Pós-task |
| `output_tokens` | Kernel — resposta gerada | Pós-task |
| `token_price` | Config — provider pricing table | Configurável |

**Valores de referência** (baseline da sessão atual):

| Tipo de Task | Input Médio | Output Médio | Custo Token Estimado |
|-------------|-------------|--------------|---------------------|
| **simple** (CLI, config) | ~4.500 | ~2.000 | $0.065 |
| **moderate** (backend, testing) | ~12.000 | ~6.000 | $0.180 |
| **complex** (engine, multi-agent) | ~25.000 | ~12.000 | $0.370 |

### 3.3 custo_tempo — Custo de Tempo

**O que mede**: Wall-clock time consumido pela task. Tempo do Don + tempo de sistema.

```
custo_tempo = time_seconds × opportunity_cost_per_second

Onde:
  time_seconds              = tempo total decorrido (task_start → task_end)
  opportunity_cost_per_second = $0.0005 — custo de oportunidade por segundo
                               (≈ $1.80/hora — tempo do Don + compute)
```

**Fontes de dados** (da F1.5 — B2 Decision Velocity):

| Campo | Fonte | Momento |
|-------|-------|---------|
| `time_seconds` | Kernel — diff entre task_start e task_end | Pós-task |
| `opportunity_cost` | Config — custo de oportunidade | Configurável |

**Valores de referência**:

| Tipo de Task | Tempo Médio | Custo Tempo Estimado |
|-------------|-------------|---------------------|
| **simple** | ~120s (2min) | $0.060 |
| **moderate** | ~380s (6min) | $0.190 |
| **complex** | ~510s (8.5min) | $0.255 |

### 3.4 custo_atencao — Custo de Atenção

**O que mede**: Quantos agentes foram mobilizados. Cada agente adicional consome atenção do sistema (coordenação, contexto, comunicação).

```
custo_atencao = num_agents × attention_cost_per_agent

Onde:
  num_agents               = número de agentes primários + subagentes acionados
  attention_cost_per_agent = $0.050 — custo de coordenação por agente
```

**Rationale**: O custo de atenção não é linear apenas em interrupções ao Don (que são raras). Ele mede o custo de **coordenação multi-agente**: cada agente extra adiciona overhead de contexto compartilhado, sincronização de estado, e potencial de conflito.

**Fontes de dados** (da F1.5 — B1 Cognitive Load):

| Campo | Fonte | Momento |
|-------|-------|---------|
| `num_agents` | Kernel — agentes primários + subagentes | Pós-task |

**Valores de referência**:

| Agentes Mobilizados | Custo Atenção |
|-------------------|---------------|
| 1 (único agente) | $0.050 |
| 2-3 (time pequeno) | $0.100 - $0.150 |
| 4-5 (time médio) | $0.200 - $0.250 |
| 6+ (time grande) | $0.300+ |

### 3.5 Cálculo do Custo Total — Exemplo

**Task F7-memorize-workflow** (dados reais da sessão):
- Input tokens: 38.400
- Output tokens: 12.800
- Tempo: 620s
- Agentes: 5

```
custo_tokens  = (38.400 + 12.800) × $0.00001 = $0.512
custo_tempo   = 620 × $0.0005 = $0.310
custo_atencao = 5 × $0.050 = $0.250

custo_total   = $0.512 + $0.310 + $0.250 = $1.072
```

---

## 4. Componentes de VALOR

### 4.1 Definição

Para cada task, o valor total é calculado como:

```
valor_total = valor_aprendizado + valor_reuso + valor_prevencao
```

### 4.2 valor_aprendizado — Valor do Conhecimento Gerado

**O que mede**: Novos learnings registrados que não existiam antes da task. Cada learning registrado tem valor porque **não ter aprendido teria um custo** (retrabalho, erro repetido, decisão sem contexto).

```
valor_aprendizado = num_learnings_gerados × avg_value_per_learning

Onde:
  num_learnings_gerados   = quantidade de learnings registrados no pós-task
  avg_value_per_learning  = $0.01 — valor médio de cada learning
                           (custo de não ter aprendido)
```

**Valor por nível de learning**:

| Nível | Descrição | Valor Unitário |
|-------|-----------|----------------|
| 1 | Observação simples | $0.005 |
| 2 | Heurística reutilizável | $0.010 |
| 3 | Padrão cross-domain | $0.025 |
| 4 | Princípio universal | $0.050 |
| 5 | Framework contribution | $0.100 |

**Default**: Usar `avg_value_per_learning = $0.01` para cálculo rápido (configurável).

**Fontes de dados**:

| Campo | Fonte | Momento |
|-------|-------|---------|
| `num_learnings_gerados` | Auto-Evolution Protocol — Stage 7 | Pós-task |
| `nivel_medio_learnings` | Learning entries — campo `level` | Pós-task |

### 4.3 valor_reuso — Valor do Reuso de Padrões

**O que mede**: Padrões já existentes que foram reutilizados nesta task. Reuso é o indicador mais forte de eficiência cognitiva: conhecimento que já existe não precisa ser recriado.

```
valor_reuso = num_patterns_reusados × avg_value_per_reuse

Onde:
  num_patterns_reusados   = quantidade de patterns existentes consultados/reutilizados
  avg_value_per_reuse     = $0.02 — valor de não ter que redescobrir o padrão
```

**Classificação de reuso** (da F1.5 — B4 Cross-agent Reuse):

| Taxa de Reuso | Classificação | Interpretação |
|--------------|---------------|---------------|
| > 40% | 🟢 Alto | Forte colaboração cross-agent |
| 20-40% | 🟢 Médio | Reuso saudável |
| 10-20% | 🟡 Baixo | Pouca colaboração |
| < 10% | 🔴 Crítico | Silos cognitivos |

**Fontes de dados**:

| Campo | Fonte | Momento |
|-------|-------|---------|
| `num_patterns_reusados` | Kernel — patterns carregados no contexto | Pós-task |
| `cross_agent_count` | F1.5 — B4 Cross-agent Reuse | Pós-task |

### 4.4 valor_prevencao — Valor da Prevenção de Erros

**O que mede**: Erros que foram evitados graças a esta task. Inclui dead code removido, bugs corrigidos, vulnerabilidades fechadas, inconsistências resolvidas.

```
valor_prevencao = estimated_bug_cost × prob_bug_prevented

Onde:
  estimated_bug_cost    = custo estimado do bug que foi prevenido
  prob_bug_prevented    = probabilidade de que o bug realmente ocorreria (0.0-1.0)
```

**Tabela de custo de bug por severidade**:

| Severidade | estimated_bug_cost | Exemplo |
|-----------|-------------------|---------|
| **trivial** | $0.05 | Warning de compilação, formatação |
| **minor** | $0.20 | Bug de UI que não bloqueia fluxo |
| **moderate** | $0.50 | Bug funcional com workaround |
| **major** | $1.00 | Bug que bloqueia funcionalidade |
| **critical** | $2.00 | Vulnerabilidade de segurança, data loss |
| **blocker** | $5.00 | Sistema inteiro comprometido |

**prob_bug_prevented por tipo de task**:

| Tipo de Task | prob_bug_prevented | Rationale |
|-------------|-------------------|-----------|
| **dead code removal** | 0.30 | Dead code pode causar confusão mas raramente bug direto |
| **bug fix** | 0.95 | Bug já confirmado — prevenção é quase certa |
| **test addition** | 0.40 | Testes previnem regressão |
| **security audit** | 0.60 | Vulnerabilidade tem alta chance de ser explorada |
| **refactoring** | 0.25 | Refatoração reduz complexidade mas risco é indireto |
| **documentation** | 0.15 | Documentação melhora mas não previne bugs diretamente |

### 4.5 Cálculo do Valor Total — Exemplo

**Task F7-memorize-workflow** (dados reais da sessão):
- Learnings gerados: 4 (nível 3-4)
- Patterns reusados: 4 (cross-agent: architecture, documentation, evolution, workflows)
- Bugs prevenidos: N/A (task de infraestrutura, não de bug fix)

```
valor_aprendizado = 4 × $0.01 = $0.040
valor_reuso       = 4 × $0.02 = $0.080
valor_prevencao   = 0

valor_total       = $0.040 + $0.080 + $0 = $0.120
```

---

## 5. ROI Cognitivo — A Equação ★

### 5.1 Fórmula

```
ROI = ((valor_total - custo_total) / custo_total) × 100
```

O ROI é expresso em **percentual**. Ele responde: **"Quanto retorno (em valor) cada $1 de custo gerou?"**

### 5.2 Interpretação

```
ROI > 200%   → 🔥 Excelente   (cada $1 gerou $3+ de valor)
ROI 100-200% → 🟢 Bom         (cada $1 gerou $2 de valor)
ROI 0-100%   → 🟡 Médio       (valeu a pena mas podia ser melhor)
ROI < 0%     → 🔴 Prejuízo    (task consumiu mais que gerou)
```

### 5.3 Matriz de Decisão por ROI

| Faixa de ROI | Decisão | Ação |
|-------------|---------|------|
| **ROI > 200%** | 🔥 Excelente | Reforço positivo no Confidence Model. Registrar como benchmark. |
| **ROI 100-200%** | 🟢 Bom | Manter abordagem. Task saudável. |
| **ROI 0-100%** | 🟡 Médio | Revisar se pode ser otimizada. Verificar se o custo pode ser reduzido. |
| **ROI -50% a 0%** | 🟠 Ruim | Alerta: "Task consumiu mais que gerou". Investigar causa. |
| **ROI < -50%** | 🔴 Crítico | Alerta P0: "Task com prejuízo significativo". Revisar decisão de execução futura. |

### 5.4 Exemplo de Cálculo — Task F7-memorize-workflow

**Custo** (da Seção 3.5):
```
custo_total = $1.072
```

**Valor** (da Seção 4.5):
```
valor_total = $0.120
```

**ROI**:
```
ROI = (($0.120 - $1.072) / $1.072) × 100
ROI = (-$0.952 / $1.072) × 100
ROI = -88.8%
```

**Interpretação**: 🔴 **Prejuízo**. Esta task consumiu mais que gerou em valor imediato.

**Análise qualitativa**: Task F7-memorize-workflow é uma **task de infraestrutura cognitiva** — ela cria ativos (workflow, timeline) que serão reutilizados em tasks futuras. O ROI negativo é **esperado para tarefas de capital intelectual**: o retorno vem do reuso futuro, não do valor imediato.

**ROI projetado com reuso futuro**:
```
Se o workflow for reutilizado 10× em tasks futuras:
  valor_reuso_futuro = 10 × $0.02 = $0.200
  valor_total_projetado = $0.120 + $0.200 = $0.320
  ROI_projetado = (($0.320 - $1.072) / $1.072) × 100 = -70.1%

Se o workflow gerar 3 learnings adicionais no futuro:
  ROI_projetado_total = ainda negativo mas a tendência é de melhora
```

> **Nota**: Tasks de infraestrutura (memorize, workflows, timelines) tendem a ter ROI imediato baixo mas ROI acumulado alto. O Cognitive Economy Engine **não bloqueia** tasks com ROI negativo — ele **alerta** e **monitora o ROI acumulado ao longo do tempo**.

---

## 6. ROI Ajustado por Risco (com F1.6 Entropy)

### 6.1 Definição

Conhecimento desorganizado (entropia alta) reduz o valor real do aprendizado gerado. Se a base de conhecimento tem alta entropia, os learnings registrados têm menor valor porque:

1. **Duplicam conhecimento existente** (já existe algo similar)
2. **Contradizem outras fontes** (criam mais entropia)
3. **Estão em local errado** (dificultam retrieval futuro)

### 6.2 Fórmula

```
ROI_ajustado = ROI × (1 - entropy_score)

Onde:
  entropy_score = F1.6 Cognitive Entropy Score (normalizado 0.0-1.0)
                  0.0 = ordem perfeita
                  1.0 = caos total
```

### 6.3 Exemplo de Ajuste

**Cenário A — Entropia baixa (15%)**:
```
ROI = 50%
entropy_score = 0.15
ROI_ajustado = 50% × (1 - 0.15) = 50% × 0.85 = 42.5%
```

**Cenário B — Entropia alta (70% — baseline atual)**:
```
ROI = 50%
entropy_score = 0.70
ROI_ajustado = 50% × (1 - 0.70) = 50% × 0.30 = 15.0%

Perda de ROI: 50% → 15% (-70% do valor nominal)
```

**Cenário C — Task F7-memorize-workflow com entropia atual**:
```
ROI = -88.8%
entropy_score = 0.645 (entropia atual de 64.5%)
ROI_ajustado = -88.8% × (1 - 0.645) = -88.8% × 0.355 = -31.5%
```

### 6.4 Integração com F1.6

| Fase | Ação | Responsável |
|------|------|-------------|
| **Pós-task** | Economy Engine consulta entropy_score do Cognitive Entropy Engine | Economy Engine |
| **Cálculo** | Aplica `ROI × (1 - entropy_score)` | Economy Engine |
| **Alerta** | Se entropy > 60%, adiciona nota: "ROI reduzido por alta entropia. Considere Cognitive Compression." | Economy Engine |
| **Correção** | Após Cognitive Compression, entropy cai → ROI_ajustado sobe automaticamente | Entropy Engine |

### 6.5 Mapa de Efeito da Entropia no ROI

```
Entropy │ ROI Ajustado (para ROI base = 100%)
────────┼─────────────────────────────────
   0%   │ 100%  (ROI_ajustado = ROI base)
  20%   │  80%
  40%   │  60%
  60%   │  40%  ← Alerta: entropia alta reduzindo significativamente o ROI
  80%   │  20%
 100%   │   0%  (Todo conhecimento gerado é ruído)
```

---

## 7. Pipeline Pós-Task

### 7.1 Ciclo de Execução

Após cada task, o Kernel executa o pipeline abaixo. O tempo total deve ser **< 100ms** (operação leve, apenas aritmética + consulta a dados locais).

```
┌─────────────────────────────────────────────────────────────────────┐
│                  PIPELINE PÓS-TASK — Cognitive Economy               │
│                                                                      │
│  ┌──────────┐                                                        │
│  │ 1. Coleta│  Kernel coleta dados da task:                         │
│  │  Dados   │  ├── tokens: input_tokens + output_tokens             │
│  │          │  ├── tempo: task_end - task_start (segundos)          │
│  │          │  ├── agentes: num_agents mobilizados                  │
│  │          │  ├── learnings: num_learnings gerados (Stage 7)       │
│  │          │  └── patterns: num_patterns_reusados                  │
│  └────┬─────┘                                                        │
│       │                                                              │
│       ▼                                                              │
│  ┌──────────┐                                                        │
│  │ 2. Custo │  Economy Engine calcula custo_total:                  │
│  │  Total   │  ├── custo_tokens  = (input + output) × token_price   │
│  │          │  ├── custo_tempo   = time_seconds × opp_cost          │
│  │          │  ├── custo_atencao = num_agents × attention_cost      │
│  │          │  └── custo_total   = soma das 3 dimensões             │
│  └────┬─────┘                                                        │
│       │                                                              │
│       ▼                                                              │
│  ┌──────────┐                                                        │
│  │ 3. Valor │  Economy Engine calcula valor_total:                  │
│  │  Total   │  ├── valor_aprendizado = num_learnings × $0.01        │
│  │          │  ├── valor_reuso       = num_patterns × $0.02         │
│  │          │  ├── valor_prevencao   = bug_cost × prob_prevented   │
│  │          │  └── valor_total       = soma das 3 dimensões         │
│  └────┬─────┘                                                        │
│       │                                                              │
│       ▼                                                              │
│  ┌──────────┐                                                        │
│  │ 4. ROI   │  Economy Engine calcula ROI:                          │
│  │          │  └── ROI = ((valor_total - custo_total) /             │
│  │          │               custo_total) × 100                       │
│  └────┬─────┘                                                        │
│       │                                                              │
│       ▼                                                              │
│  ┌──────────┐                                                        │
│  │ 5. Risco │  Economy Engine consulta F1.6 Cognitive Entropy:     │
│  │          │  └── entropy_score = CognitiveEntropyEngine.consulta() │
│  └────┬─────┘                                                        │
│       │                                                              │
│       ▼                                                              │
│  ┌──────────┐                                                        │
│  │ 6. Ajuste│  ROI_ajustado = ROI × (1 - entropy_score)            │
│  └────┬─────┘                                                        │
│       │                                                              │
│       ▼                                                              │
│  ┌──────────┐                                                        │
│  │ 7. Regis-│  Registra no Trust Registry e na Timeline:            │
│  │  trar    │  ├── TRUST_REGISTRY.md — entrada de reputação          │
│  │          │  ├── memory/timeline/roi-cognitivo.csv                │
│  │          │  └── Decision DNA (se ROI < 0%)                       │
│  └────┬─────┘                                                        │
│       │                                                              │
│       ▼                                                              │
│  ┌──────────┐                                                        │
│  │ 8. Alerta│  Se ROI < 0%:                                         │
│  │          │  └── Alerta: "⚠️ Task {id} não valeu o custo.         │
│  │          │         ROI: {roi}%. Investigar."                      │
│  └──────────┘                                                        │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 7.2 Formato de Registro — CSV Timeline

```csv
timestamp,task_id,agent,roi_pct,roi_ajustado_pct,custo_total,valor_total,entropy_score,alert,task_type
2026-07-30T20:33:38Z,F7-memorize-workflow,cosca-kernel,-88.8,-31.5,1.072,0.120,0.645,true,complex
```

**Schema**:
- `timestamp`: ISO 8601
- `task_id`: Identificador único da task
- `agent`: Agente primário executor
- `roi_pct`: ROI calculado (percentual)
- `roi_ajustado_pct`: ROI ajustado por entropia (percentual)
- `custo_total`: Custo total em dólares
- `valor_total`: Valor total em dólares
- `entropy_score`: Score de entropia cognitiva (0.0-1.0)
- `alert`: `true` se ROI < 0%
- `task_type`: simple / moderate / complex

### 7.3 Integração com Trust Registry (F7.2)

O ROI é registrado como campo adicional em cada entrada do Trust Registry:

```yaml
- agent: cosca-kernel
  task_type: orchestration
  task_id: F7-memorize-workflow
  outcome: success
  confidence_before: 0.90
  confidence_after: 0.93
  confidence_delta: +0.03
  latency: 620s
  cost: $1.072
  cognitive_roi: -88.8%          # ★ NOVO — ROI cognitivo
  cognitive_roi_adjusted: -31.5% # ★ ROI ajustado por entropia
  files_changed: 8
  loc_delta: +2193
  tags:
    - cognitive-economy
    - roi-negative
    - infrastructure-task
```

---

## 8. Dashboard de ROI Cognitivo

### 8.1 Dashboard Principal

```
┌──────────────────────────────────────────────────────────────────────────┐
│                      COGNITIVE ECONOMY DASHBOARD                          │
│                          2026-07-30 — Sessão Atual                        │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│  RESUMO DA SESSÃO                                                         │
│  ┌──────────────────────┐  ┌──────────────────────┐                      │
│  │  CUSTO TOTAL         │  │  VALOR TOTAL          │                      │
│  │  $XX.XX               │  │  $X.XX                │                      │
│  │  14 tasks            │  │  32 learnings gerados  │                      │
│  │                       │  │  18 patterns reusados  │                      │
│  └──────────────────────┘  └──────────────────────┘                      │
│                                                                           │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  ROI MÉDIO DA SESSÃO: -12.3% (ajustado: -4.4%)                    │   │
│  │                                                                   │   │
│  │  ████████████████████░░░░░░░░░░░░░░░░░░░░░░  🟡 ROI médio        │   │
│  │                                                                   │   │
│  │  Entropia atual: 64.5% 🟠 ALTA — reduzindo ROI em 64.5%          │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                           │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  TOP 5 AGENTES POR ROI                                            │   │
│  │                                                                   │   │
│  │  🥇 cosca-cli          ROI: +245% 🔥   custo médio: $0.15       │   │
│  │  🥈 cosca-testing      ROI: +180% 🟢   custo médio: $0.22       │   │
│  │  🥉 cosca-backend      ROI: +120% 🟢   custo médio: $0.30       │   │
│  │  4.  cosca-review      ROI: +85%  🟡   custo médio: $0.08       │   │
│  │  5.  cosca-qa          ROI: +62%  🟡   custo médio: $0.12       │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                           │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  BOTTOM 5 AGENTES POR ROI                                         │   │
│  │                                                                   │   │
│  │  10. cosca-kernel       ROI: -88%  🔴   custo médio: $1.07      │   │
│  │   9. cosca-architecture ROI: -35%  🔴   custo médio: $0.68      │   │
│  │   8. cosca-documentation ROI: -12% 🟠   custo médio: $0.45      │   │
│  │   7. cosca-security     ROI: +15%  🟡   custo médio: $0.38      │   │
│  │   6. cosca-evolution    ROI: +42%  🟡   custo médio: $0.18      │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                           │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  ALERTAS ATIVOS                                                    │   │
│  │                                                                   │   │
│  │  🔴 F7-memorize-workflow    ROI: -88.8%  — Task de infraestrutura│   │
│  │  🔴 F3-agent-engine         ROI: -42.5%  — Custo alto de tokens  │   │
│  │  🟡 F6-config-tests         ROI: +12.3%  — ROI baixo, reuso zero │   │
│  │                                                                   │   │
│  │  3 tarefas com ROI < 0% (21% da sessão)                           │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

### 8.2 Dashboard por Agente

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    cosca-cli — Detalhamento de ROI                        │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│  ┌────────┬────────┬────────┬────────┬────────┬────────┬────────┐       │
│  │ Task   │ Custo  │ Valor  │ ROI    │ Ajust. │ Entrop │ Alerta │       │
│  ├────────┼────────┼────────┼────────┼────────┼────────┼────────┤       │
│  │ F6-cli1│ $0.08  │ $0.25  │+212%   │+75%    │ 0.645  │ 🟢     │       │
│  │ F6-cli2│ $0.07  │ $0.22  │+214%   │+76%    │ 0.645  │ 🟢     │       │
│  │ F6-cli3│ $0.06  │ $0.18  │+200%   │+71%    │ 0.645  │ 🟢     │       │
│  │ F6-cli4│ $0.05  │ $0.20  │+300%   │+107%   │ 0.645  │ 🔥     │       │
│  │ F6-cli5│ $0.04  │ $0.15  │+275%   │+98%    │ 0.645  │ 🔥     │       │
│  ├────────┼────────┼────────┼────────┼────────┼────────┼────────┤       │
│  │ MÉDIA  │ $0.06  │ $0.20  │+240%   │+85%    │ 0.645  │ 🔥     │       │
│  └────────┴────────┴────────┴────────┴────────┴────────┴────────┘       │
│                                                                           │
│  INSIGHT: cosca-cli domina ROI porque tasks CLI são curtas,              │
│  consomem poucos tokens (6-8K vs 50K+ de tasks complexas) e              │
│  geram padrões reutilizáveis (comandos seguem templates existentes).     │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

### 8.3 Dashboard de Tendência

```
┌──────────────────────────────────────────────────────────────────────────┐
│                   TENDÊNCIA DE ROI (últimas 10 tasks)                     │
│                                                                           │
│                  ┌──────┐                                                 │
│                  │      │ ROI médio                                       │
│   +100% ────────│▄▄▄▄▄▄│────────────────────────────────────             │
│                  │ ██   │                                                 │
│      0% ────────│ ██ ▄▄│────────────────────────────────────             │
│                  │ ██ ██│▄▄                                               │
│   -100% ────────│ ██ ██│██▄▄────────────────────────────                 │
│                  └──────┘                                                 │
│                1  2  3  4  5  6  7  8  9  10                              │
│                                                                           │
│  Tendência: ROI médio +12.3% por task → melhorando                       │
│  Tasks de infraestrutura (negativas) estão concentradas no início        │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 9. Exemplo com Dados Reais da Sessão

### 9.1 Task Selecionada: Remoção de Dead Code (L22 — Triple Offensive)

**Task**: F6-mcp-deadlock-fix — Corrigir deadlock no `Connect()` do MCP
**Agente**: cosca-backend
**Complexidade**: simple
**Tempo**: 120s
**Tokens**: 8.100 (input: 6.200, output: 1.900)
**Agentes**: 2 (cosca-backend, cosca-chat)
**Learnings gerados**: 1 (padrão de deadlock prevention)
**Patterns reusados**: 2 (MCP connection pattern, lock pattern)
**Bug prevenido**: moderate — deadlock em produção poderia travar conexões MCP

#### Passo 1: Coletar Dados

```yaml
task_data:
  id: "F6-mcp-deadlock-fix"
  agent: "cosca-backend"
  type: "simple"
  tokens:
    input: 6200
    output: 1900
  time_seconds: 120
  num_agents: 2
  num_learnings: 1
  num_patterns: 2
  bug_severity: "moderate"
```

#### Passo 2: Calcular Custo Total

```
custo_tokens  = (6.200 + 1.900) × $0.00001 = $0.081
custo_tempo   = 120 × $0.0005 = $0.060
custo_atencao = 2 × $0.050 = $0.100

custo_total   = $0.081 + $0.060 + $0.100 = $0.241
```

#### Passo 3: Calcular Valor Total

```
valor_aprendizado = 1 × $0.01 = $0.010
valor_reuso       = 2 × $0.02 = $0.040
valor_prevencao   = $0.50 × 0.95 = $0.475
  # bug moderate: $0.50
  # prob_prevented: 0.95 (bug fix confirmado — deadlock reproduzido e corrigido)

valor_total       = $0.010 + $0.040 + $0.475 = $0.525
```

#### Passo 4: Calcular ROI

```
ROI = (($0.525 - $0.241) / $0.241) × 100
ROI = ($0.284 / $0.241) × 100
ROI = +117.8%
```

#### Passo 5: Ajustar por Risco (Entropia)

```
entropy_score = 0.645 (entropia atual da base de conhecimento: 64.5%)

ROI_ajustado = 117.8% × (1 - 0.645)
ROI_ajustado = 117.8% × 0.355
ROI_ajustado = +41.8%
```

#### Passo 6: Resultado

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                  │
│   📊 RELATÓRIO DE ROI COGNITIVO                                 │
│   ─────────────────────────────                                  │
│                                                                  │
│   Task:         F6-mcp-deadlock-fix                              │
│   Agente:       cosca-backend                                    │
│   Tipo:         simple (bug fix)                                 │
│                                                                  │
│   ┌────────────────────┬────────────┬─────────────┬───────────┐  │
│   │ Componente         │ Custo      │ Valor       │ Detalhe   │  │
│   ├────────────────────┼────────────┼─────────────┼───────────┤  │
│   │ Tokens             │ $0.081     │ —           │ 8.1K tok  │  │
│   │ Tempo              │ $0.060     │ —           │ 120s      │  │
│   │ Atenção            │ $0.100     │ —           │ 2 agents  │  │
│   │ Aprendizado        │ —          │ $0.010      │ 1 learn   │  │
│   │ Reuso              │ —          │ $0.040      │ 2 patterns│  │
│   │ Prevenção          │ —          │ $0.475      │ Moderado  │  │
│   ├────────────────────┼────────────┼─────────────┼───────────┤  │
│   │ TOTAL              │ $0.241     │ $0.525      │           │  │
│   └────────────────────┴────────────┴─────────────┴───────────┘  │
│                                                                  │
│   ROI:    +117.8%  🟢 BOM                                       │
│   Ajustado: +41.8%  🟡 (entropia 64.5% reduziu ROI em 64.5%)   │
│                                                                  │
│   INTERPRETAÇÃO:                                                 │
│   ├── Cada $1 gasto gerou $2.18 de valor (ROI de 117.8%)        │
│   ├── O valor de prevenção de bug ($0.475) domina o retorno     │
│   ├── Custo baixo (task simples, 2 agentes, 2 min)              │
│   └── Entropia alta reduz o ROI ajustado para 41.8%             │
│                                                                  │
│   VEREDITO: ✅ TASK VALEU A PENA                                 │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 9.2 Tabela Comparativa — Todas as Tasks da Sessão

| Task | Agente | Tipo | Custo | Valor | ROI | ROI Ajustado | Alerta |
|------|--------|------|-------|-------|-----|-------------|--------|
| F6-config-tests | cosca-testing | simple | $0.065 | $0.080 | +23.1% | +8.2% | 🟡 |
| F6-mcp-deadlock-fix | cosca-backend | simple | $0.241 | $0.525 | **+117.8%** | +41.8% | 🟢 |
| F6-mcp-plugin-cli | cosca-cli | simple | $0.082 | $0.220 | **+168.3%** | +59.7% | 🟢 |
| F6-agent-skill-cli | cosca-cli | simple | $0.096 | $0.250 | **+160.4%** | +56.9% | 🟢 |
| F6-session-cli | cosca-cli | simple | $0.080 | $0.200 | **+150.0%** | +53.3% | 🟢 |
| F6-completion-config-cli | cosca-cli | simple | $0.073 | $0.180 | **+146.6%** | +52.0% | 🟢 |
| F3-agent-engine | cosca-backend | complex | $0.438 | $0.320 | -26.9% | -9.6% | 🔴 |
| F3.5-4-engine-tests | cosca-testing | complex | $0.522 | $0.380 | -27.2% | -9.7% | 🔴 |
| F7-memorize-workflow | cosca-kernel | complex | $1.072 | $0.120 | **-88.8%** | -31.5% | 🔴 |

### 9.3 Insights da Sessão

1. **Tasks CLI dominam ROI**: As 5 tasks do cosca-cli estão no topo com ROI > 140%. São tasks curtas, baratas e que geram padrões reutilizáveis.

2. **Bug fix é o melhor investimento**: F6-mcp-deadlock-fix tem ROI de 117.8% porque o valor de prevenção de bug é alto e o custo é baixo.

3. **Tasks de engine (complexas) têm ROI negativo imediato**: F3, F3.5, F7 são tasks de infraestrutura/investimento. ROI negativo é esperado — o retorno vem do reuso futuro.

4. **Entropia de 64.5% corta o ROI em 64.5%**: Todos os ROIs ajustados são significativamente menores. Reduzir a entropia para <30% aumentaria o ROI ajustado em ~2.8×.

5. **Três tasks com ROI negativo (21% da sessão)**: Duas são tasks de engine (investimento), uma é task de memorização (infraestrutura). Nenhuma é task de entrega direta ao Don.

---

## 10. Integração com a Arquitetura Cosca

### 10.1 Posição no Pipeline do KERNEL.md

O Cognitive Economy Engine opera em **dois momentos**:

1. **PRÉ-TASK**: Como advisor de eficiência (efficiency_score — modelo conceitual completo no Apêndice A)
2. **PÓS-TASK**: Como contador de ROI (★ F2.1 — este documento)

```
KERNEL.md DECISION PIPELINE
═══════════════════════════════════════════════════════════════

PRÉ-TASK (Advisor):
───────────────────

Step 5: Request Analysis
  │
  ▼
Step 6: Capability Resolution
  │
  ▼
┌─────────────────────────────────────────────────────────────┐
│ ★ COGNITIVE ECONOMY ENGINE (PRÉ-TASK) ★                     │
│  ├── Consulta modelo de eficiência                           │
│  ├── Estima custo × valor                                    │
│  └── Recomenda: Full / Standard / Light / Defer / Skip      │
└──────────────────────────────────────────────────────────────┘
  │
  ▼
Step 7: Planning & DAG Generation
  │
  ▼
Steps 8-13: Execution → Review → Quality → Documentation → Storage
  │
  ▼
  ┌─────────────────────────────────────────────────────┐
  │             PÓS-TASK (★ ESTE DOCUMENTO)              │
  │                                                     │
  │ ★ COGNITIVE ECONOMY ENGINE (PÓS-TASK) ★              │
  │  ├── 1. Coleta dados da task                        │
  │  ├── 2. Calcula custo_total                         │
  │  ├── 3. Calcula valor_total                         │
  │  ├── 4. Calcula ROI                                 │
  │  ├── 5. Consulta F1.6 Entropy                       │
  │  ├── 6. ROI_ajustado = ROI × (1 - entropy)          │
  │  ├── 7. Registra no Trust Registry e Timeline        │
  │  └── 8. Alerta se ROI < 0%                          │
  └─────────────────────────────────────────────────────┘
```

### 10.2 Integração com F1.5 — Cognitive Metrics (B1-B5)

| Métrica F1.5 | Uso no F2.1 | Fonte |
|-------------|-------------|-------|
| **B1 — Cognitive Load** | `input_tokens`, `output_tokens`, `num_agents` → custo_tokens + custo_atencao | CSV: cognitive-load.csv |
| **B2 — Decision Velocity** | `time_seconds` → custo_tempo | CSV: decision-velocity.csv |
| **B3 — Knowledge Freshness** | Qualificador: freshness < 0.50 reduz avg_value_per_learning em 50% | CSV: knowledge-freshness.csv |
| **B4 — Cross-agent Reuse** | `num_patterns_reusados` → valor_reuso | CSV: cross-agent-reuse.csv |
| **B5 — Cognitive Debt** | `debt_contribution` → se > 0, reduz valor_aprendizado em debt% | CSV: cognitive-debt.csv |

### 10.3 Integração com F7.1 — Prediction Engine

| Dado | Fornecido Por | Usado Em |
|------|---------------|----------|
| `custo_estimado` | Prediction Engine (pré-task) | Comparação: custo estimado vs custo real (pós-task) |
| `tempo_estimado` | Prediction Engine (pré-task) | delta: estimado vs real → calibração |
| `P(success)` | Prediction Engine (pré-task) | Qualificador: se P < 0.70, ROI com desconto de 20% |

### 10.4 Integração com F7.2 — Trust Registry

O ROI é armazenado como campo adicional no Trust Registry (ver Seção 7.3). O Trust Registry então pode consultar:

- `cognitive_roi` médio por agente (para ranking de ROI)
- `cognitive_roi` por task_type (para identificar tipos de task mais eficientes)
- `cognitive_roi` trend (para monitorar melhoria/piora ao longo do tempo)

### 10.5 Integração com Confidence Model

O ROI alimenta o Confidence Model:

```
Se ROI > 100%:
  → confidence_delta += 0.02 (reforço positivo para o agente)

Se ROI < 0%:
  → confidence_delta -= 0.01 (alerta: agente está gerando prejuízo)
  → Se ROI < -50% por 3+ tasks consecutivas:
      → Confidence Model reduz confidence_score do agente em -0.05
```

---

## 11. Configuração de Thresholds

### 11.1 Thresholds Padrão

```yaml
cognitive_economy:
  # Thresholds de ROI
  roi:
    excelente: 200     # ROI > 200% → 🔥
    bom: 100           # ROI 100-200% → 🟢
    medio: 0           # ROI 0-100% → 🟡
    prejuizo: -50      # ROI -50% a 0% → 🟠
    critico: -50       # ROI < -50% → 🔴

  # Thresholds de alerta
  alertas:
    roi_negativo: true               # Alerta se ROI < 0%
    roi_critico_consecutivo: 3       # N tasks consecutivas com ROI < 0% antes de escalar
    entropy_max: 0.60                # Alerta se entropy > 60%

  # Preços (configuráveis por provider)
  precos:
    token_price: 0.00001             # $ por token
    opportunity_cost_per_second: 0.0005  # $ por segundo
    attention_cost_per_agent: 0.050  # $ por agente
    avg_value_per_learning: 0.01     # $ por learning
    avg_value_per_reuse: 0.02        # $ por pattern reusado

  # Pesos das dimensões (para composição de custo/valor)
  pesos:
    custo:
      tokens: 0.40
      tempo: 0.35
      atencao: 0.25
    valor:
      aprendizado: 0.30
      reuso: 0.25
      prevencao: 0.45  # Prevenção de bug tem maior peso porque evita custo real

  # Performance
  performance:
    max_calculation_time_ms: 100     # Pipeline completo deve executar em < 100ms
    cache_ttl_seconds: 300           # Cache de entropy_score por 5 minutos
```

### 11.2 Personalização por Projeto

Os thresholds podem ser sobrescritos no `cosca.config.yaml`:

```yaml
extensions:
  cognitive_economy:
    precos:
      token_price: 0.00002  # Provider mais caro
    pesos:
      custo:
        atencao: 0.35       # Atenção do Don vale mais neste projeto
```

---

## 12. Métricas do Próprio Engine

### 12.1 Métricas de Saúde do Engine

```yaml
engine_self_metrics:
  # Métricas de operação
  tasks_processed: 0
  tasks_with_alerts: 0
  avg_calculation_time_ms: 0

  # Métricas de acurácia (comparação com previsão do Prediction Engine)
  cost_accuracy:
    formula: "1 - |custo_real - custo_estimado| / custo_real"
    target: "> 0.80"
    current: 0.0

  # Distribuição de ROI
  roi_distribution:
    excelente: 0    # ROI > 200%
    bom: 0          # ROI 100-200%
    medio: 0        # ROI 0-100%
    prejuizo: 0     # ROI < 0%

  # Impacto da entropia
  entropy_impact_avg: 0.0  # Média de redução de ROI por entropia

  # Alertas
  active_alerts: 0
```

### 12.2 Formato de Armazenamento

```
internal/embed/cosca/memory/timeline/
├── cognitive-load.csv              # F1.5 — B1
├── decision-velocity.csv           # F1.5 — B2
├── knowledge-freshness.csv         # F1.5 — B3
├── cross-agent-reuse.csv           # F1.5 — B4
├── cognitive-debt.csv              # F1.5 — B5
├── roi-cognitivo.csv               # ★ F2.1 — ROI Cognitivo (NOVO)
└── cognitive-health-report.md      # Relatório consolidado
```

---

## 13. Automação e Performance

### 13.1 Requisitos de Performance

| Operação | Tempo Máximo | Descrição |
|----------|-------------|-----------|
| Coleta de dados | 10ms | Kernel já tem os dados em memória |
| Cálculo de custo | 5ms | 3 multiplicações + 2 somas |
| Cálculo de valor | 5ms | 3 multiplicações + 2 somas |
| Cálculo de ROI | 2ms | 1 divisão + 1 multiplicação |
| Consulta entropy | 50ms | Cache hit (TTL 300s) |
| Registro em CSV | 10ms | Append to file |
| **Total** | **82ms** | **< 100ms ✓** |

### 13.2 Gatilho Automático

O Kernel DEVE executar o hook pós-task automaticamente:

```
hook_post_task_cognitive_economy:
  trigger: "Kernel.task_complete"
  steps:
    1. Coletar dados (tokens, tempo, agentes, learnings, patterns)
    2. Calcular custo_total
    3. Calcular valor_total
    4. Calcular ROI
    5. Consultar Cognitive Entropy (cache 300s)
    6. Calcular ROI_ajustado
    7. Registrar em memory/timeline/roi-cognitivo.csv
    8. Se ROI < 0%: log alerta
    9. Atualizar dashboard (a cada 10 tasks)
```

### 13.3 Cache de Entropia

Para garantir < 100ms, o score de entropia é cachead por 300 segundos:

```yaml
entropy_cache:
  ttl: 300              # 5 minutos
  key: "cognitive_entropy_score"
  refresh: "on_demand"  # Refresh ao solicitar
  stale_if_error: true  # Se falhar, usar último valor conhecido
```

---

## 14. Implementação

### 14.1 Artefatos do Engine

```
internal/embed/cosca/engines/cognitive-economy/
├── SKILL.md                    ← ESTE ARQUIVO (especificação completa)
├── roi-calculator.go           ← Calculadora de ROI (fórmulas Seção 5)
├── cost-calculator.go          ← Cálculo de custo (Seção 3)
├── value-calculator.go         ← Cálculo de valor (Seção 4)
├── entropy-adjuster.go         ← Ajuste por entropia (Seção 6)
├── pipeline.go                 ← Pipeline pós-task (Seção 7)
├── types.go                    ← Tipos: RoiResult, CostVector, ValueVector
├── config.go                   ← Thresholds e preços configuráveis
├── dashboard.go                ← Geração de dashboard
├── logs/
│   └── roi-history.jsonl       ← Histórico de ROI (para dashboard e tendência)
└── tests/
    ├── roi-calculator_test.go
    ├── cost-calculator_test.go
    └── value-calculator_test.go
```

### 14.2 Interface Go (types.go)

```go
package cognitiveconomy

// CostVector representa os 3 componentes de custo
type CostVector struct {
    Tokens   float64 `json:"custo_tokens"`   // $ (input + output) × price
    Time     float64 `json:"custo_tempo"`    // $ time_seconds × opp_cost
    Attention float64 `json:"custo_atencao"` // $ num_agents × attention_cost
}

func (c CostVector) Total() float64 {
    return c.Tokens + c.Time + c.Attention
}

// ValueVector representa os 3 componentes de valor
type ValueVector struct {
    Learning  float64 `json:"valor_aprendizado"` // $ num_learnings × value_per_learning
    Reuse     float64 `json:"valor_reuso"`       // $ num_patterns × value_per_reuse
    Prevention float64 `json:"valor_prevencao"`  // $ bug_cost × prob_prevented
}

func (v ValueVector) Total() float64 {
    return v.Learning + v.Reuse + v.Prevention
}

// RoiResult representa o resultado completo do cálculo
type RoiResult struct {
    TaskID        string  `json:"task_id"`
    Agent         string  `json:"agent"`
    Cost          float64 `json:"custo_total"`        // $
    Value         float64 `json:"valor_total"`        // $
    ROI           float64 `json:"roi_pct"`            // %
    EntropyScore  float64 `json:"entropy_score"`      // 0.0-1.0
    RoiAdjusted   float64 `json:"roi_ajustado_pct"`   // %
    Alert         bool    `json:"alert"`               // ROI < 0%
    TaskType      string  `json:"task_type"`
    Timestamp     string  `json:"timestamp"`
}
```

### 14.3 Pseudocódigo do Pipeline

```python
def calcular_roi(task_data):
    """
    Pipeline completo de cálculo de ROI.
    Tempo estimado: < 100ms
    """
    # Passo 1: Coleta (dados já fornecidos pelo Kernel)
    tokens_input = task_data['input_tokens']
    tokens_output = task_data['output_tokens']
    time_seconds = task_data['time_seconds']
    num_agents = task_data['num_agents']
    num_learnings = task_data['num_learnings']
    num_patterns = task_data['num_patterns']
    bug_cost = task_data.get('bug_cost', 0)
    prob_prevented = task_data.get('prob_prevented', 0)

    # Passo 2: Custo Total
    custo_tokens = (tokens_input + tokens_output) * TOKEN_PRICE
    custo_tempo = time_seconds * OPPORTUNITY_COST
    custo_atencao = num_agents * ATTENTION_COST
    custo_total = custo_tokens + custo_tempo + custo_atencao

    # Passo 3: Valor Total
    valor_aprendizado = num_learnings * LEARNING_VALUE
    valor_reuso = num_patterns * REUSE_VALUE
    valor_prevencao = bug_cost * prob_prevented
    valor_total = valor_aprendizado + valor_reuso + valor_prevencao

    # Passo 4: ROI
    if custo_total > 0:
        roi = ((valor_total - custo_total) / custo_total) * 100
    else:
        roi = 0  # Sem custo → ROI indefinido (considera 0)

    # Passo 5: Entropy
    entropy_score = get_cached_entropy()  # Cache de 300s

    # Passo 6: ROI Ajustado
    roi_ajustado = roi * (1 - entropy_score)

    # Passo 7: Alerta
    alert = roi < 0

    # Passo 8: Registro
    registrar_roi(task_data['task_id'], task_data['agent'],
                  roi, roi_ajustado, custo_total, valor_total,
                  entropy_score, alert)

    return RoiResult(
        task_id=task_data['task_id'],
        agent=task_data['agent'],
        cost=custo_total,
        value=valor_total,
        roi=roi,
        entropy_score=entropy_score,
        roi_adjusted=roi_ajustado,
        alert=alert,
        task_type=task_data['task_type'],
    )
```

### 14.4 Passos de Implementação

```yaml
implementation_steps:
  step_1:
    title: "Definir tipos Go (types.go)"
    effort: "1 hora"
    description: |
      CostVector, ValueVector, RoiResult.
      Constantes de preço e thresholds.

  step_2:
    title: "Implementar Cost Calculator (cost-calculator.go)"
    effort: "1 hora"
    description: |
      3 funções: calcular_custo_tokens, calcular_custo_tempo,
      calcular_custo_atencao. Aritmética pura.

  step_3:
    title: "Implementar Value Calculator (value-calculator.go)"
    effort: "1 hora"
    description: |
      3 funções: calcular_valor_aprendizado, calcular_valor_reuso,
      calcular_valor_prevencao. Tabelas de classificação.

  step_4:
    title: "Implementar ROI Calculator (roi-calculator.go)"
    effort: "1 hora"
    description: |
      Função principal calcular_roi().
      Integração com entropy cache.
      Geração de alerta.

  step_5:
    title: "Implementar Pipeline (pipeline.go)"
    effort: "2 horas"
    description: |
      Hook pós-task.
      Registro em CSV (roi-cognitivo.csv).
      Integração com Trust Registry.

  step_6:
    title: "Implementar Dashboard (dashboard.go)"
    effort: "2 horas"
    description: |
      Geração de dashboard ASCII.
      Ranking top 5 / bottom 5.
      Tendência de ROI.

  step_7:
    title: "Integrar com Kernel"
    effort: "2 horas"
    description: |
      Hook pós-task no KERNEL.md.
      Pipeline automático < 100ms.
      Fallback: se engine falhar, continuar sem ROI.

  step_8:
    title: "Testes"
    effort: "2 horas"
    description: |
      Testes unitários para cada calculadora.
      Testes de integração com dados da sessão.
      Teste de performance (< 100ms).
```

---

## 15. Referências Cruzadas

| Documento | Seção | Relação |
|-----------|-------|---------|
| **F1.5 — Cognitive Metrics** | [cognitive-metrics.md](../../analytics/cognitive-metrics.md) | B1-B5 fornecem dados de custo e reuso |
| **F1.6 — Cognitive Entropy** | [COGNITIVE_ENTROPY.md](../../analytics/COGNITIVE_ENTROPY.md) | Entropy score para ajuste de ROI |
| **F7.1 — Prediction Engine** | [engines/prediction/SKILL.md](prediction/SKILL.md) | Custo estimado vs custo real |
| **F7.2 — Trust Registry** | [memory/trust/TRUST_REGISTRY.md](../../memory/trust/TRUST_REGISTRY.md) | Registro de ROI por task |
| **KERNEL.md** | §10 Steps 5-7 | Pipeline de decisão |
| **CONFIDENCE_MODEL.md** | Evidence Engine | Feedback de ROI → confidence |
| **AUTO_EVOLUTION_PROTOCOL.md** | Post-task checklist | Fonte de num_learnings |
| **COGNITIVE_MATURITY.md** | §5 C14 | Conceito original |
| **cognitive-maturity-implementation.md** | F2.1 | Tarefa de implementação |

---

## 16. Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Architecture Chief | Criação inicial. Especificação completa: 5 dimensões de custo, 5 de valor, efficiency_score, decision ladder, learning loop, 3 exemplos práticos, integração com KERNEL.md pipeline, Confidence Model, CMI Julgamento, plano de implementação em 10 passos. |
| **2.0.0** | **2026-07-30** | **Cosca Architecture Chief** | **★ F2.1 — Reformulação completa como STAR da Fase 2.** Novo modelo de ROI cognitivo (3 custos × 3 valores). Equação fundamental ROI = (V-C)/C. Ajuste por entropia (F1.6). Pipeline pós-task em 8 passos (< 100ms). Dashboard com top 5/bottom 5 agentes. Exemplo com dados reais da sessão (F6-mcp-deadlock-fix). Thresholds configuráveis. Integração com F1.5, F7.1, F7.2, Confidence Model. Pseudocódigo do pipeline. |

---

> **Enforced by**: Cosca Architecture Chief | **Próxima calibração**: Após 20 tasks com ROI registrado
> **Kernel instruction**: `cosca economy roi --task <task_id> --report`
