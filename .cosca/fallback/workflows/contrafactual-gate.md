# CONTRAFACTUAL GATE — Motor de Decisão Contrafactual (F1.2 / G0.5)

> **Versão**: 2.0.0 | **Status**: active | **Categoria**: decision-engine | **Owner**: cosca-critic
> **Criado**: 2026-07-30 | **Atualizado**: 2026-07-30
> **CMI Dimensão**: Decision Quality (Julgamento) | **Bloco Cognitivo**: Bloco 2 — Decisão & Raciocínio
> **Fase CMI**: Fase 1 — Foundation | **Código**: F1.2 | **Quality Gate**: G0.5
>
> Consulte também:
> - [DECISION_DNA.md](../knowledge/architecture/DECISION_DNA.md) — DDNA format (F1.1), as `Options` são populadas por este Gate
> - [QUALITY_GATES.md](../QUALITY_GATES.md) — G0.5: Contrafactual Decision Review (gate specification)
> - [cosca-critic PROMPT.md](../agents/cosca-critic/PROMPT.md) — Gate owner e executor
> - [KERNEL.md](../KERNEL.md) — §10 Initialization Sequence (invocação entre Step 6 e Step 7)
> - [TRUST_REGISTRY.md](../memory/trust/TRUST_REGISTRY.md) — F7.2: reputação histórica de agentes
> - [CONFIDENCE_MODEL.md](../engines/evidence/CONFIDENCE_MODEL.md) — Ponderação de evidências
> - [cognitive-maturity-implementation.md](cognitive-maturity-implementation.md) — Pipeline CMI (F1.2)

---

## Índice

1. [O que é o Contrafactual Gate](#1-o-que-é-o-contrafactual-gate)
2. [Gatilhos de Ativação](#2-gatilhos-de-ativação)
3. [Pipeline do Gate](#3-pipeline-do-gate)
4. [Integração com DDNA (F1.1)](#4-integração-com-ddna-f11)
5. [Integração com Trust Registry (F7.2)](#5-integração-com-trust-registry-f72)
6. [Regras de Escalação](#6-regras-de-escalação)
7. [Regras de Design](#7-regras-de-design)
8. [Formato de Saída (YAML)](#8-formato-de-saída-yaml)
9. [Tratamento de Erros](#9-tratamento-de-erros)
10. [Exemplo: Auto-Jail memfd_create vs Script-Based](#10-exemplo-auto-jail-memfd_create-vs-script-based)
11. [Métricas e Monitoramento](#11-métricas-e-monitoramento)
12. [Relacionados](#12-relacionados)

---

## 1. O que é o Contrafactual Gate

### Definição

O **Contrafactual Gate** é o motor do Cosca que, antes de cada decisão P0/P1, força o sistema a perguntar: **"E se fosse diferente?"** — implementando o Quality Gate G0.5 como um passo mandatório no pipeline de decisão.

Ele é o **motor que gera e avalia alternativas** (A/B/C) antes da decisão ser executada, prevenindo viés de confirmação e garantindo que toda decisão estratégica passe por escrutínio adversarial explícito.

### Propósito

| Dimensão | Descrição |
|----------|-----------|
| **Cognitivo** | Prevenir viés de confirmação — o sistema busca ativamente evidências que sustentem a decisão oposta |
| **Arquitetural** | Garantir que alternativas foram consideradas antes de cristalizar decisões P0/P1 |
| **Auditável** | Produzir registro estruturado da análise contrafactual, linkável ao DDNA |
| **Econômico** | Impedir que decisões caras (> threshold) sejam tomadas sem contraponto |
| **Histórico** | Acumular baseline de "quantas vezes o contrafactual venceu o proposto" para calibração do motor |

### Filosofia

```
Decisão proposta: A
Pergunta mandatória: "E se fizéssemos ¬A? E se existir uma Opção C?"

┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│   A  ◄─── comparação estruturada ───►  ¬A  ◄─── C (opcional)    │
│                                                                  │
│   Evidências a favor      Evidências a favor      Evidências     │
│   Riscos                  Riscos                  Riscos         │
│   Premissas               Premissas               Premissas      │
│   Custo/benefício         Custo/benefício         Custo/benefício│
│                                                                  │
│              ┌──────────────────────────────────┐                │
│              │   O Gate RECOMENDA:              │                │
│              │   proceed | escalate | reject    │                │
│              │                                  │                │
│              │   O Kernel/Don DECIDE            │                │
│              └──────────────────────────────────┘                │
└──────────────────────────────────────────────────────────────────┘
```

### Quando está ativo

O Gate está sempre "armado" no runtime, mas só dispara quando os gatilhos de ativação são satisfeitos. Para decisões triviais (P2/P3/routine), o Gate permanece silencioso — zero custo.

---

## 2. Gatilhos de Ativação

O Contrafactual Gate é ativado automaticamente quando QUALQUER condição abaixo é satisfeita:

### Gatilhos Mandatórios

| # | Gatilho | Descrição | Severidade se ignorado |
|---|---------|-----------|------------------------|
| **G1** | **Toda decisão P0** | Decisões de arquitetura, segurança, dados, infraestrutura, design de runtime | **Crítico** — gate NON-NEGOTIABLE |
| **G2** | **Decisão multi-módulo** | Qualquer decisão que afete 2+ módulos/departamentos/domínios | **Alto** — impacto cross-module requer contraponto |
| **G3** | **Custo estimado > threshold** | Custo estimado da decisão (em tokens + tempo computacional) excede o threshold configurável | **Alto** — decisões caras não podem ser unilaterais |

### Gatilhos Opcionais (Configuráveis)

| # | Gatilho | Threshold Default | Descrição |
|---|---------|-------------------|-----------|
| **G4** | **P1 com alto risco** | confidence < 0.70 | Decisões P1 onde o agente decisor tem baixa confiança |
| **G5** | **DDNA pendente** | — | Se o próprio agente já marcou `status: proposed` no DDNA, o gate é automático |
| **G6** | **Override do Don** | — | Don explicitamente requisita contrafactual para qualquer decisão |

### Threshold de Custo (G3) — Configuração

O threshold é definido em `cosca.config.yaml`:

```yaml
contrafactual_gate:
  enabled: true
  cost_threshold_usd: 0.01        # Ativa se custo > $0.01
  cost_threshold_tokens: 50000    # Alternativo: ativa se > 50K tokens
  min_confidence_for_p1: 0.70     # Abaixo disso, P1 vira mandatório
  max_analysis_cost_ratio: 0.50   # Se análise > 50% do custo da decisão, pula (fast-track)
```

O threshold de **$0.01** foi escolhido porque:
- Abaixo disso: decisões são baratas demais para justificar o overhead do gate
- Acima disso: o custo do gate (~200-500 tokens) é marginal comparado ao custo da decisão
- Ajustável: o Don pode mudar para $0.05 ou $0.001 conforme o perfil do projeto

### Matriz de Ativação

```
                    ┌─────────────────────────────────────────────────────┐
                    │              GATILHOS DE ATIVAÇÃO                    │
                    ├──────────┬──────────┬──────────┬──────────┬─────────┤
                    │    G1    │    G2    │    G3    │    G4    │   G5    │
                    │   P0     │ Multi-   │  Custo   │ P1 conf  │ DDNA    │
                    │          │ módulo   │ > $0.01  │ < 0.70   │pendente │
─────────┬──────────┼──────────┼──────────┼──────────┼──────────┼─────────┤
Decisão  │ P0       │   ■      │   □      │   □      │   □      │   □     │
         │ P1       │   —      │   ■      │   □      │   ■      │   □     │
         │ P2       │   —      │   —      │   ▲      │   ▲      │   ▲     │
         │ P3       │   —      │   —      │   —      │   —      │   —     │
─────────┴──────────┴──────────┴──────────┴──────────┴──────────┴─────────┘

Legenda:
  ■ = Ativa SEMPRE (mandatório)
  □ = Ativa se condição for satisfeita
  ▲ = Ativa somente se configurado explicitamente
  — = Ignora este gatilho
```

---

## 3. Pipeline do Gate

O Contrafactual Gate opera em duas fases: **Gate Aberto** (análise) e **Gate Fechado** (decisão executada).

### Visão Geral

```
┌──────────────────────────────────────────────────────────────────────┐
│                    CONTRAFACTUAL GATE PIPELINE                        │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │                   GATE ABERTO                                 │     │
│  │                                                               │     │
│  │  1. Kernel registra "Decisão pendente: X"                     │     │
│  │     │                                                         │     │
│  │  2. Kernel gera 2-3 alternativas (A/B/C)                      │     │
│  │     │                                                         │     │
│  │  3. Para cada alternativa, calcula:                           │     │
│  │     ├── Impacto técnico (esforço, arquivos afetados)          │     │
│  │     ├── Impacto em segurança                                  │     │
│  │     ├── Impacto em performance                                │     │
│  │     ├── Custo estimado (tokens + tempo)                       │     │
│  │     └── Risco (confidence histórico do agente)                │     │
│  │     │                                                         │     │
│  │  4. Gate compara alternativas (matriz 5 dimensões)            │     │
│  │     │                                                         │     │
│  │  5. Gate recomenda uma (ou "nenhuma das anteriores")          │     │
│  │     │                                                         │     │
│  │  6. Kernel decide: accept | escalate | reject                 │     │
│  │     │                                                         │     │
│  └─────┬───────────────────────────────────────────────────────┘     │
│        │                                                             │
│        ▼                                                             │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │                   GATE FECHADO                                │     │
│  │                                                               │     │
│  │  • Decisão final registrada em DDNA                           │     │
│  │  • Options populadas pelo output do Gate                      │     │
│  │  • Evidence linka para análise do Gate                        │     │
│  │  • Confidence calculado pelo Gate                             │     │
│  │  • Trust Registry atualizado                                  │     │
│  │  • Gate encerrado — execução prossegue                        │     │
│  │                                                               │     │
│  └─────────────────────────────────────────────────────────────┘     │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### Passo a Passo Detalhado

#### GATE ABERTO

##### Passo 1: Kernel registra "Decisão pendente"

- **Executor**: Kernel
- **Ação**: Identifica que uma decisão P0/P1 está prestes a ser tomada e registra no estado do runtime
- **Evento**: `DecisionPending { decision_id, description, domain, priority, estimated_cost }`
- **Output**: `decision_id` (UUID) que rastreará toda a análise do gate

##### Passo 2: Kernel gera 2-3 alternativas (A/B/C)

- **Executor**: cosca-critic (via Kernel)
- **Tarefa**: Gerar explicitamente 2-3 alternativas viáveis para a decisão proposta
- **Regras**:
  - Alternativa A: a decisão proposta originalmente
  - Alternativa ¬A: a decisão oposta (deve ser real, não espantalho)
  - Alternativa C (opcional): uma terceira via que não é nem A nem ¬A
  - Se nenhuma alternativa viável existe além de A, registrar "não há alternativa" e pular para veredito
- **Output**: Lista de alternativas com descrição de 1-2 frases cada

##### Passo 3: Para cada alternativa, calcular impactos

- **Executor**: cosca-critic (pode delegar a especialistas: cosca-security, cosca-performance)
- **Dimensões de impacto**:

| Dimensão | Métrica | Fonte de Dados |
|----------|---------|----------------|
| **Impacto técnico** | Esforço (XS/S/M/L/XL) + arquivos afetados | Análise estrutural do código |
| **Impacto em segurança** | 0 (nenhum) a 10 (crítico) | [RISK_REGISTRY.md](../knowledge/failures/risks/RISK_REGISTRY.md) + análise de threat model |
| **Impacto em performance** | 0 (sem impacto) a 10 (degradação severa) | Benchmarks existentes + análise de complexidade |
| **Custo estimado** | USD + tokens + tempo de execução | [Cognitive Economy](../engines/cognitive-economy/SKILL.md) + [Cost Calculator](../engines/cognitive-economy/SKILL.md) |
| **Risco** | 0 (seguro) a 10 (crítico) | [Trust Registry](#5-integração-com-trust-registry-f72) + confidence histórico do agente |

- **Regra de consistência**: Nenhum impacto pode ser avaliado sem fonte de dados citada
- **Output**: Matriz de impacto preenchida para cada alternativa

##### Passo 4: Gate compara alternativas

- **Executor**: cosca-critic
- **Método**: Matriz de comparação 5 dimensões (legada do v1.0.0, mantida e expandida)

| Dimensão | Descrição | Escala |
|----------|-----------|--------|
| Risco | Probabilidade × Impacto de falha | 1 (baixo) – 10 (crítico) |
| Custo | Esforço de implementação + manutenção | 1 (trivial) – 10 (proibitivo) |
| Tempo | Time-to-value / velocidade de entrega | 1 (imediato) – 10 (muito longo) |
| Ganho de Conhecimento | Quanto aprendemos com a decisão | 1 (nada novo) – 10 (alto aprendizado) |
| Reversibilidade | Facilidade de voltar atrás | 1 (irreversível) – 10 (trivial reverter) |

- **Adicional**: Para decisões arquiteturais (P0), incluir também:
  - **Extreme scenarios**: 10x, 100x, falha catastrófica, 6 meses
  - **Assumptions audit**: Mínimo 3 premissas explícitas desafiadas
- **Output**: Matriz com scores explícitos + rationales

##### Passo 5: Gate recomenda

- **Executor**: cosca-critic
- **Regras de recomendação**:

| Condição | Recomendação | Ação |
|----------|-------------|------|
| Evidências de A são claramente superiores | **proceed** | Decisão confirmada, rationale documentado |
| ¬A ou C revela riscos não examinados | **escalate** | Revisão mais profunda antes de prosseguir |
| ¬A ou C demonstra ser objetivamente melhor | **reject** | Flag para Don review — NÃO executar |
| Premissas críticas de A têm confiança baixa | **escalate** | Validar premissas primeiro |
| Todas as alternativas têm risco alto | **escalate** | [Escalar para Don](#6-regras-de-escalação) |
| Evidências insuficientes para todos os lados | **escalate** | Decisão prematura |

- **Output**: Recomendação com rationale completo e confidence score

##### Passo 6: Kernel decide

- **Executor**: Kernel (ou Don, se escalado)
- **Poderes do Kernel**:
  - **Accept**: Aceita a recomendação do Gate e prossegue
  - **Override**: Rejeita a recomendação do Gate com rationale documentado
  - **Escalate**: Envia para o Don (quando o Gate recomenda `escalate` ou `reject`)
- **Regra**: O Kernel SEMPRE pode fazer override, mas o override DEVE ser documentado
- **Output**: Decisão final registrada

#### GATE FECHADO

##### Passo 7: Registro e encerramento

- **Executor**: Kernel
- **Ações**:
  1. DDNA criado/atualizado com as `Options` populadas pelo Gate
  2. `evidence` do DDNA linka para a análise do Gate (decision_id)
  3. `confidence` do DDNA é o calculado pelo Gate
  4. Trust Registry atualizado com o resultado
  5. Evento `ContrafactualGateClosed` publicado
  6. Gate encerrado — execução prossegue para Step 7 (Planning)
- **Output**: DDNA completo + evento de fechamento

### Diagrama de Estado

```
                    ┌──────────────┐
                    │  IDLE        │  ← Gate armado, esperando gatilho
                    └──────┬───────┘
                           │ Gatilho dispara (P0, multi-módulo, custo > $0.01...)
                           ▼
                    ┌──────────────┐
                    │  OPENING     │  ← Passo 1: Kernel registra decisão pendente
                    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
                    │ ANALYZING    │  ← Passos 2-4: Alternativas geradas e avaliadas
                    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
                    │  VOTING      │  ← Passo 5: Gate recomenda
                    └──────┬───────┘
                           │
               ┌───────────┼────────────┐
               │           │            │
               ▼           ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │ ACCEPT   │ │ESCALATE  │ │ REJECT   │
        │ Kernel   │ │ Kernel→  │ │ Kernel→  │
        │ prossegue│ │ Don      │ │ Don      │
        └────┬─────┘ └────┬─────┘ └────┬─────┘
             │            │            │
             └────────────┼────────────┘
                          ▼
                    ┌──────────────┐
                    │  CLOSING     │  ← Passo 7: DDNA registrado, Trust atualizado
                    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
                    │  IDLE        │  ← Pronto para próxima decisão
                    └──────────────┘
```

---

## 4. Integração com DDNA (F1.1)

O Contrafactual Gate e o Decision DNA (DDNA) são dois lados da mesma moeda. O Gate **gera** as alternativas e análises; o DDNA **armazena** o resultado de forma queryable.

### Mapeamento DDNA ← Gate

| Campo DDNA | Fonte | Populado por |
|------------|-------|-------------|
| `## Options` (seção inteira) | **Output do Passo 3** | O Gate gera as Opções A/B/C com prós/contras/evidências |
| `## Decision` (decisão final) | **Output do Passo 6** | Kernel ou Don decide, baseado na recomendação do Gate |
| `## Evidence` (evidências) | **Output do Passo 3** | Links para as análises do Gate (contrafactual_gate.yaml) |
| `confidence` (frontmatter) | **Output do Passo 5** | Confidence calculado pelo Gate (média ponderada das dimensões) |
| `risks_identified` | **Output do Passo 3** | Riscos identificados em todas as alternativas |
| `revisit` | **Output do Passo 4** | Sugestão de revalidação baseada na estabilidade estimada |

### Fluxo DDNA + Gate

```
1. Decisão necessária
   │
2. Gate aberto: alternativas geradas (A/B/C)
   │
3. Para cada alternativa, impactos calculados
   │
4. Gate compara e recomenda
   │
5. Kernel decide
   │
6. DDNA CRIADO:
   ├── frontmatter: id, confidence, domain (do Gate)
   ├── Context: o problema (da decisão original)
   ├── Options: populado pelo Gate (alternativas, prós, contras, evidências, esforço, risco)
   ├── Decision: a escolha final + justificativa
   ├── Consequences: derivado da matriz de impacto
   └── Evidence: links para o contrafactual_gate.yaml + análises
```

### Exemplo de Linkagem

```yaml
# No DDNA, seção Evidence:
## Evidence

- [Contrafactual Gate Analysis](internal/embed/cosca/gate-results/cfg-2026-07-30-auto-jail-memfd.yaml)
  - decision_id: cfg-2026-07-30-auto-jail-memfd
  - Gate recommendation: proceed (confidence: 0.85)
  - Recommended action: memfd_create + fallback script-based para Darwin
```

### Regra de Consistência

> **TODO DDNA de decisão P0/P1 DEVE ter passado pelo Contrafactual Gate.**
> Se um DDNA é criado sem o decision_id do Gate correspondente, o Quality Engine deve emitir um warning (e eventualmente um error).

---

## 5. Integração com Trust Registry (F7.2)

O **Trust Registry** (F7.2) é o sistema de reputação histórica de agentes no Cosca. O Contrafactual Gate consulta o Trust Registry para calibrar suas análises.

### Consultas do Gate ao Trust Registry

| Consulta | Quando | Como o resultado é usado |
|----------|--------|--------------------------|
| **"Este agente já fez decisões similares antes?"** | Passo 3 (análise de risco) | Se o agente tem histórico de decisões similares com sucesso, o peso da sua confiança aumenta |
| **"Qual a taxa de sucesso do agente neste domínio?"** | Passo 3 (cálculo de confidence) | Confidence do agente no domínio é um fator no score de risco |
| **"A última decisão similar do agente foi validada ou revertida?"** | Passo 5 (recomendação) | Decisões revertidas no passado aumentam o escrutínio |
| **"Qual o viés histórico do agente?"** | Passo 4 (auditoria de premissas) | Agentes com viés de confirmação conhecido recebem auditoria mais rigorosa |
| **"Custo médio das decisões anteriores do agente?"** | Passo 3 (custo estimado) | Se o agente consistentemente subestima custos, aplica-se fator de correção |

### Formato da Consulta

```yaml
trust_registry_query:
  agent: "cosca-critic | cosca-kernel | cosca-architecture"
  domain: "architecture | security | database"
  decision_type: "P0 | P1"
  lookback_period_days: 90
  min_samples: 3

trust_registry_response:
  agent: "cosca-architecture"
  domain: "architecture"
  total_decisions: 12
  success_rate: 0.83          # 10/12 sucessos
  avg_confidence: 0.78
  avg_cost: 0.015             # USD médio por decisão
  bias_profile:               # Vieses conhecidos
    confirmation_bias: 0.3    # 0-1, quanto maior mais propenso
    recency_bias: 0.2
  last_decision:
    id: "DDNA-2026-07-28-003"
    outcome: "success"
    days_ago: 2
```

### Atualização do Trust Registry pelo Gate

Após cada execução do Gate (Gate Fechado, Passo 7), o Trust Registry é atualizado:

```yaml
trust_registry_update:
  decision_id: "cfg-2026-07-30-auto-jail-memfd"
  agent: "cosca-critic"
  domain: "security"
  outcome: "proceed"           # proceed | escalate | reject | override
  confidence: 0.85             # Confidence da recomendação do Gate
  cost: 0.002                  # Custo real da análise do Gate (USD)
  accuracy_later:              # Preenchido quando o resultado da decisão for conhecido
    outcome_validated: null    # success | partial | failure
    confidence_calibration: null  # O Gate superestimou ou subestimou o risco?
```

---

## 6. Regras de Escalação

O Contrafactual Gate opera em uma hierarquia clara de decisão. Quando não consegue resolver, escala.

### Cadeia de Decisão

```
Gate recomenda ──► Kernel decide ──► Don (se escalado)
```

### Regras de Escalação

| # | Condição | Ação | Justificativa |
|---|----------|------|---------------|
| **E1** | Todas as alternativas têm risco alto (score > 7) | **Escalar para o Don** | O Gate não tem informação suficiente para recomendar — risco alto em todas as opções significa que a decisão precisa de julgamento humano |
| **E2** | Custo da análise > custo da decisão | **Pular Gate (fast-track)** | O Gate não pode custar mais que a decisão que avalia — decisões triviais ou urgentes precisam seguir sem gate |
| **E3** | Don offline + decisão P0 | **Executar com melhor alternativa + documentar** | Decisões P0 não podem esperar — o Gate executa, registra no DDNA com `status: proposed` e sinaliza para revisão do Don quando voltar |
| **E4** | Gate recomenda `reject` | **Escalar para o Don** | O Gate não pode rejeitar uma decisão — só o Don pode vetar. O Gate documenta por que rejeita e envia ao Don |
| **E5** | Gate recomenda `escalate` | **Escalar para o Don OU revisão por par** | Se o Don estiver disponível, escala para ele. Se não, uma segunda rodada de revisão por outro Chief (ex: cosca-architecture para decisão de segurança) |

### Fast-Track (E2) — Detalhamento

O fast-track é acionado quando:

```
custo_estimado_da_analise > custo_estimado_da_decisao × max_analysis_cost_ratio
```

Onde `max_analysis_cost_ratio` é configurável (default: 0.50).

**Exemplo**:
- Decisão: "Renomear variável X para Y" (custo estimado: $0.001)
- Análise do Gate: ~500 tokens ($0.002)
- Ratio: 0.002 / 0.001 = 2.0 → **Aciona fast-track** → Gate pula, decisão segue direto

**O que acontece no fast-track**:
1. Gate registra "pulado por custo" com o cálculo
2. Kernel prossegue diretamente para o Step 7 (Planning)
3. DDNA é criado com `confidence: 0.95` (decisão trivial) e nota sobre o fast-track
4. Evento `ContrafactualGateSkipped { reason: "cost_exceeds_decision", decision_id }` publicado

### Don Offline (E3) — Modo Autônomo

Quando o Don está offline e uma decisão P0 é necessária:

```
1. Gate executa normalmente (Passos 1-5)
2. Gate emite recomendação
3. Kernel assume o papel de decisor (normalmente do Don)
4. Kernel registra NO DDNA:
   - confidence: calculado pelo Gate (pode ser menor se Don não revisou)
   - status: "proposed" (não "accepted" — aguardando revisão do Don)
   - campo especial: don_review_required: true
5. Quando Don voltar:
   - Notificação: "Decisão P0 tomada em modo autônomo: DDNA-XXX"
   - Don pode: accept, reject, modify
   - Se reject/modify: DDNA atualizado com `superseded_by`
```

---

## 7. Regras de Design

### Princípios

| # | Princípio | Descrição | Consequência |
|---|-----------|-----------|--------------|
| **D1** | **Leveza** | O Gate NÃO pode custar mais que a decisão que ele avalia | Se custo estimado da análise > threshold, pula (fast-track) |
| **D2** | **Skip permitido** | O Gate PODE ser pulado para decisões triviais (P2, P3, routine) | Gatilhos G1-G3 só disparam para P0/P1 e multi-módulo; P2/P3 passam direto |
| **D3** | **Recomendação, não decisão** | A decisão final é sempre do Kernel ou do Don — o Gate recomenda, nunca decide | Gate outputs `recommendation`, Kernel outputs `decision` |
| **D4** | **Automatizável** | O formato deve ser compatível com execução automatizada (não apenas manual) | YAML output parseável, eventos publicados, hooks no runtime |
| **D5** | **Auditável** | Toda execução do Gate deixa rastro completo | decision_id, timestamps, rationale, confidence — tudo registrado |
| **D6** | **Calibrável** | O Gate melhora com o tempo | Trust Registry retroalimenta as análises; outcomes validados calibram confidence |

### Custo Máximo do Gate

O Gate deve consumir **no máximo 5% do custo da decisão** que avalia:

| Tipo de Decisão | Custo Típico | Custo Máximo do Gate | Tokens Máximos |
|-----------------|-------------|---------------------|----------------|
| P0 — Arquitetural | $0.05 – $0.50 | $0.0025 – $0.025 | 500 – 5.000 |
| P1 — Feature design | $0.01 – $0.10 | $0.0005 – $0.005 | 100 – 1.000 |
| P2 — Operacional | Não passa pelo Gate | — | — |
| P3 — Trivial | Não passa pelo Gate | — | — |

Se o custo real do Gate exceder o máximo em mais de 20%, um alerta é emitido para o Cognitive Economy Engine.

### Skip Conditions (resumo)

O Gate pode ser pulado (com aviso registrado) quando:

1. **Decisão P2/P3**: Operacional, baixo impacto, routine
2. **Decisão sem alternativa real**: "Usar git" não tem ¬A viável
3. **Workflow puramente mecânico**: Rodar testes, formatar código, deploy automático
4. **Custo da análise > custo da decisão**: Fast-track (E2)
5. **Don autorizou skip explícito**: Don disse "vai direto" — registrado em evento

---

## 8. Formato de Saída (YAML)

O output do Contrafactual Gate é um arquivo YAML armazenado em `internal/embed/cosca/gate-results/{decision_id}.yaml`, com o seguinte formato:

```yaml
---
# =============================================================================
# CONTRAFACTUAL GATE — Output Completo
# =============================================================================
contrafactual_gate:
  # ── Metadados ──────────────────────────────────────────────────────────────
  version: "2.0.0"
  executed_at: "2026-07-30T14:00:00Z"
  executed_by: "cosca-critic"
  decision_id: "cfg-{uuid}"
  gate_trigger: "P0 | P1 | multi_module | cost_threshold"  # Qual gatilho ativou
  estimated_decision_cost_usd: 0.05
  actual_gate_cost_usd: 0.002
  gate_cost_ratio: 0.04  # % do custo da decisão

  # ── Decisão sendo avaliada ──────────────────────────────────────────────────
  decision:
    title: "Título curto da decisão"
    description: "Descrição completa do que está sendo decidido"
    context: "Workflow, etapa, restrições relevantes"
    domain: "architecture | security | database | devops | testing | etc"
    priority: "P0 | P1"
    proposed_by: "cosca-agent-name"
    confidence_proposer: 0.0  # Confidence do agente que propôs

  # ── Gatilho ─────────────────────────────────────────────────────────────────
  trigger:
    type: "G1 | G2 | G3 | G4 | G5 | G6"
    reason: "Descrição legível de por que o gate foi ativado"
    threshold_value: 0.01     # Se G3: o threshold atual
    actual_value: 0.05        # Se G3: o valor que excedeu o threshold

  # ── Trust Registry consultado ───────────────────────────────────────────────
  trust_registry:
    consulted: true
    agent: "cosca-agent-name"
    domain_success_rate: 0.83
    total_similar_decisions: 12
    bias_adjustment_applied: 0.95  # Fator de ajuste baseado em viés histórico

  # ── Alternativas ────────────────────────────────────────────────────────────
  alternatives:
    - id: "A"
      description: "A decisão proposta — descrição"
      selected_by: "proposer"
      evidence:
        - evidence: "Descrição da evidência"
          source: "Fonte (ADR, benchmark, bug registry, etc.)"
          strength: "alta | média | baixa"
          type: "empírica | teórica | heurística | anedótica"
          trust_weight: 0.9  # Peso ajustado pelo Trust Registry
      assumptions:
        - premise: "Premissa 1"
          confidence: "alta | média | baixa"
      impact:
        technical: { score: 0, effort: "XS | S | M | L | XL", files_affected: 0 }
        security: { score: 0, description: "..." }
        performance: { score: 0, description: "..." }
        cost: { usd: 0.0, tokens: 0, hours: 0 }
        risk: { score: 0, confidence_historical: 0.0 }
      comparison:
        risk: 0
        cost: 0
        time: 0
        knowledge_gain: 0
        reversibility: 0

    - id: "¬A"
      description: "A decisão oposta — descrição"
      evidence: [ ]
      assumptions: [ ]
      impact:
        technical: { score: 0, effort: "XS | S | M | L | XL", files_affected: 0 }
        security: { score: 0, description: "..." }
        performance: { score: 0, description: "..." }
        cost: { usd: 0.0, tokens: 0, hours: 0 }
        risk: { score: 0, confidence_historical: 0.0 }
      comparison:
        risk: 0
        cost: 0
        time: 0
        knowledge_gain: 0
        reversibility: 0

    - id: "C"  # Opcional
      description: "Terceira via — descrição"
      evidence: [ ]
      assumptions: [ ]
      impact: {}
      comparison: {}

  # ── Premissas desafiadas ────────────────────────────────────────────────────
  assumptions_challenged:
    - premise: "Premissa questionada"
      confidence: "alta | média | baixa"
      challenge: "Por que pode estar errada"
      impact_if_wrong: "O que muda se estiver errada"

  # ── Cenários extremos ───────────────────────────────────────────────────────
  extreme_scenarios:
    at_10x: "Como cada alternativa se comporta com 10x"
    at_100x: "Como cada alternativa se comporta com 100x"
    catastrophic_failure: "Modo de falha de cada alternativa"
    in_6_months: "A decisão ainda faz sentido em 6 meses?"

  # ── Recomendação ────────────────────────────────────────────────────────────
  recommendation:
    outcome: "proceed | escalate | reject"
    rationale: "Justificativa completa da recomendação"
    confidence: 0.0  # Confidence do Gate na recomendação
    recommended_alternative: "A | ¬A | C | none"
    runner_up: "¬A | C | none"  # Segunda melhor opção

  # ── Riscos identificados (todos os lados) ────────────────────────────────────
  risks_identified:
    - risk: "Descrição do risco"
      alternative: "A | ¬A | C"
      severity: "baixa | média | alta | crítica"
      probability: 0.0
      impact: 0.0
      mitigation: "Como mitigar se prosseguir"
      owner: "cosca-agent-responsável"

  # ── Decisão final (preenchido pelo Kernel/Don) ─────────────────────────────
  final_decision:
    decided_by: "kernel | don"
    decision: "A | ¬A | C | modified | override"
    rationale: "Justificativa da decisão final"
    timestamp: "ISO8601"
    ddna_id: "DDNA-2026-07-30-NNN"
    override_gate: false  # true se Kernel/Don rejeitou a recomendação do Gate

  # ── Rastreabilidade ─────────────────────────────────────────────────────────
  traceability:
    related_adrs: [ ]
    related_bugs: [ ]
    related_risks: [ ]
    related_ddnas: [ ]
    trust_registry_updated: true
```

---

## 9. Tratamento de Erros

| Falha | Ação | Impacto |
|-------|------|---------|
| ¬A não pode ser formulada | Registrar como premissa: "não há alternativa viável" — prosseguir com aviso | Confidence reduzido para 0.50 |
| Evidências insuficientes para ambos os lados | Outcome = escalate — decisão prematura | Bloqueia execução até revisão |
| Premissas ocultas não identificadas | Marcar confidence como baixa, sugerir revisão por par | Confidence reduzido |
| cosca-critic indisponível | Delegar a cosca-review com prompt de contrafactual | Pode aumentar custo do gate |
| Trust Registry indisponível | Ignorar consulta, prosseguir sem ajuste de viés | Confidence sem calibração histórica |
| Conflito entre dimensões (A melhor em risco, ¬A melhor em custo) | Documentar trade-off explícito, escalar se P0 | Vai para o Don se P0 |
| Gate excede custo máximo do orçamento | Cortar análise: usar apenas 2 alternativas, sem cenários extremos | Análise reduzida, confidence menor |
| DDNA não pode ser criado (falha de I/O) | Registrar resultado em memória volátil, tentar persistir depois | Risco de perda de auditoria |

---

## 10. Exemplo: Auto-Jail memfd_create vs Script-Based

> Este exemplo foi mantido e expandido da v1.0.0 por ser um caso real de decisão P0 que passou pelo Gate.

### Contexto

Durante o design do sistema de auto-jaula do Cosca (`cosca-jail-autoexec`), a decisão central foi: **como o binário deve ser disponibilizado dentro da jaula Bubblewrap?** O Don propôs usar `memfd_create` — um syscall Linux que cria um arquivo anônimo em memória, sem nunca tocar em disco. A alternativa seria uma abordagem baseada em script: copiar o binário para um local temporário em disco (`/tmp/cosca-jail-<pid>`), torná-lo executável, e limpar após a execução.

### Gatilho

- **Tipo**: G1 (P0 — arquitetura/segurança)
- **Razão**: Decisão de arquitetura de segurança afetando o runtime central
- **Threshold**: N/A (P0 é sempre mandatório)

### Execução do Gate

```yaml
---
contrafactual_gate:
  version: "2.0.0"
  executed_at: "2026-07-30T14:00:00Z"
  executed_by: "cosca-critic"
  decision_id: "cfg-2026-07-30-auto-jail-memfd"
  gate_trigger: "P0"
  estimated_decision_cost_usd: 0.05
  actual_gate_cost_usd: 0.003
  gate_cost_ratio: 0.06

  decision:
    title: "Usar memfd_create para disponibilizar o binário dentro da jaula"
    description: "O binário cosca precisa ser executável dentro da jaula Bubblewrap, idealmente sem tocar em disco."
    context: "Workflow cosca-jail-autoexec — Step 2 (auto-reexecução)"
    domain: "security"
    priority: "P0"
    proposed_by: "cosca-kernel"
    confidence_proposer: 0.85

  trigger:
    type: "G1"
    reason: "Decisão P0 de arquitetura/segurança — mandatório"

  trust_registry:
    consulted: true
    agent: "cosca-kernel"
    domain_success_rate: 0.92
    total_similar_decisions: 8
    bias_adjustment_applied: 1.0  # Sem viés conhecido neste domínio

  alternatives:
    - id: "A"
      description: "memfd_create — arquivo anônimo em memória (Linux 3.17+)"
      selected_by: "proposer"
      evidence:
        - evidence: "Binário nunca toca em disco — zero vestígios forenses"
          source: "Linux man pages — memfd_create(2)"
          strength: "alta"
          type: "empírica"
        - evidence: "Atomicidade: fd existe apenas no espaço de memória do processo"
          source: "LWN.net — memfd_create and sealing"
          strength: "alta"
          type: "empírica"
        - evidence: "Não requer cleanup — quando o processo morre, o memfd desaparece"
          source: "Linux kernel source — mm/shmem.c"
          strength: "alta"
          type: "empírica"
      assumptions:
        - premise: "Sistema sempre terá Linux 3.17+"
          confidence: "média"
        - premise: "Bubblewrap suporta /proc/self/fd/<N> como executável"
          confidence: "alta"
        - premise: "Binário cabe inteiramente em memória"
          confidence: "alta"
      impact:
        technical: { score: 3, effort: "S", files_affected: 2 }
        security: { score: 1, description: "Zero disk footprint — isolamento máximo" }
        performance: { score: 2, description: "Sem I/O de disco, latência mínima" }
        cost: { usd: 0.02, tokens: 5000, hours: 2 }
        risk: { score: 3, confidence_historical: 0.88 }
      comparison:
        risk: 3
        cost: 3
        time: 3
        knowledge_gain: 8
        reversibility: 5

    - id: "¬A"
      description: "Script-based — copiar binário para /tmp/cosca-jail-<pid>, executar, limpar"
      evidence:
        - evidence: "Portabilidade total — funciona em qualquer Unix (Linux, macOS, BSD)"
          source: "POSIX specification"
          strength: "alta"
          type: "empírica"
        - evidence: "Debugging trivial — binário visível no filesystem"
          source: "Experiência operacional"
          strength: "alta"
          type: "heurística"
        - evidence: "Não depende de syscalls Linux específicas"
          source: "Multi-plataforma — Cosca roda em macOS"
          strength: "alta"
          type: "empírica"
      assumptions:
        - premise: "/tmp está disponível e tem espaço suficiente"
          confidence: "alta"
        - premise: "Limpeza via trap é confiável"
          confidence: "média"
      impact:
        technical: { score: 2, effort: "XS", files_affected: 1 }
        security: { score: 4, description: "Vestígios em disco, race condition potencial" }
        performance: { score: 3, description: "I/O de disco na cópia e remoção" }
        cost: { usd: 0.01, tokens: 2000, hours: 1 }
        risk: { score: 4, confidence_historical: 0.75 }
      comparison:
        risk: 4
        cost: 2
        time: 2
        knowledge_gain: 3
        reversibility: 8

  assumptions_challenged:
    - premise: "Sistema sempre terá Linux 3.17+"
      confidence: "média"
      challenge: "Cosca tem ambição multi-plataforma. macOS, WSL e containers mínimos podem não expor memfd_create."
      impact_if_wrong: "Se falso, a jaula inteira falha em plataformas não-Linux. Precisamos de fallback."

    - premise: "Ausência de vestígios em disco é requisito real"
      confidence: "média"
      challenge: "Modelo de ameaça é sobre execução segura de comandos de IA, não anti-forense."
      impact_if_wrong: "Se desnecessário, o custo de debugging e portabilidade do memfd não se justifica."

    - premise: "Binário cabe inteiramente em memória"
      confidence: "alta"
      challenge: "Binários Go podem ser grandes (5-50MB). Edge case: sistemas embarcados."
      impact_if_wrong: "Se >500MB, memfd pode causar pressão de memória."

  extreme_scenarios:
    at_10x: "memfd: 10 jaulas simultâneas criam 10 memfds — sem contenção de disco. | script: 10 jaulas competem por /tmp — possível contenção de I/O."
    at_100x: "memfd: 100 × 20MB = 2GB. Aceitável se binário compartilhado. | script: 100 escritas simultâneas — risco de lentidão."
    catastrophic_failure: "memfd: se memfd_create falhar (kernel antigo), erro claro. | script: se /tmp sem espaço, falha silenciosa."
    in_6_months: "memfd: se expandir para macOS, memfd_create não funciona. Fallback necessário. | script: funciona em qualquer plataforma."

  recommendation:
    outcome: "proceed"
    rationale: |
      A decisão por memfd_create é confirmada, com ressalva crítica:
      precisamos de fallback script-based para plataformas não-Linux.

      RAZÕES PARA PROCEED:
      1. Ganho de conhecimento (score 8) é o fator decisivo. memfd_create é
         subutilizado no ecossistema Go e gera aprendizado transferível.
      2. Risco de portabilidade é real mas mitigável: memfd como caminho
         primário + fallback script-based para Darwin/BSD.
      3. Superioridade técnica (zero disk, atomicidade, sem cleanup) é real.
      4. Custo de implementação é baixo (~40 linhas Go).

      AÇÃO REQUERIDA:
      - Adicionar //go:build linux no jail_linux.go (memfd)
      - Criar jail_darwin.go (fallback script-based)
      - Documentar como ADR
    confidence: 0.85
    recommended_alternative: "A"
    runner_up: "¬A"

  risks_identified:
    - risk: "Falha em plataformas não-Linux (macOS, BSD, WSL)"
      alternative: "A"
      severity: "alta"
      probability: 0.6
      impact: 0.9
      mitigation: "Implementar fallback script-based com build tags"
      owner: "cosca-architecture"
    - risk: "Kernel Linux antigo (<3.17) não suporta memfd_create"
      alternative: "A"
      severity: "baixa"
      probability: 0.1
      impact: 0.8
      mitigation: "Verificar suporte no init, emitir erro claro"
      owner: "cosca-kernel"
    - risk: "Debugging mais difícil — binário não visível no filesystem"
      alternative: "A"
      severity: "média"
      probability: 0.3
      impact: 0.4
      mitigation: "Flag --jail-keep-temp para debugging"
      owner: "cosca-documentation"

  final_decision:
    decided_by: "kernel"
    decision: "A"
    rationale: "Recomendação do Gate aceita integralmente. memfd_create como primário + fallback script-based."
    timestamp: "2026-07-30T14:05:00Z"
    ddna_id: "DDNA-2026-07-30-001"
    override_gate: false

  traceability:
    related_adrs: [ "docs/adr/auto-jail-memfd.md" ]
    related_bugs: [ ]
    related_risks: [ "internal/embed/cosca/memory/risk/RISK_REGISTRY.md#portabilidade-jail" ]
    related_ddnas: [ "DDNA-2026-07-30-001" ]
    trust_registry_updated: true
```

### Resultado Pós-Gate

| Aspecto | Antes do Gate | Depois do Gate |
|---------|---------------|----------------|
| Decisão | "Usar memfd_create" (não questionada) | "Usar memfd_create com fallback script-based para não-Linux" |
| Premissas expostas | 0 explícitas | 4 questionadas (1 com confiança média) |
| Riscos identificados | Nenhum documentado | 3 riscos com mitigação + owner |
| Portabilidade | Assumida Linux-only | Plano explícito para Darwin/BSD |
| Confiança | Implícita | 0.85 (alta, com ressalva documentada) |
| Trust Registry | Não consultado | Histórico do agente verificado (92% sucesso) |
| DDNA | Não existia | DDNA-2026-07-30-001 criado com Options do Gate |

---

## 11. Métricas e Monitoramento

### Métricas do Gate

| Métrica | Tipo | Descrição | Alerta |
|---------|------|-----------|--------|
| `gate.activations.total` | Counter | Total de ativações do Gate | — |
| `gate.activations.by_trigger` | Counter | Ativações por gatilho (G1-G6) | — |
| `gate.outcome` | Counter | proceed / escalate / reject | Se reject > 10% no mês, revisar qualidade das decisões |
| `gate.cost.usd` | Histogram | Custo real de cada execução do Gate | Se > 5% do custo da decisão |
| `gate.cost.ratio` | Histogram | gate_cost / decision_cost | Se > 0.20, fast-track pode estar descalibrado |
| `gate.duration_ms` | Histogram | Tempo de execução do Gate | Se > 30s, otimizar |
| `gate.confidence` | Histogram | Confidence das recomendações | Se média < 0.60, Gate pode estar supercrítico |
| `gate.skip.count` | Counter | Execuções puladas (fast-track) | Se > 50% das ativações, threshold pode estar baixo |
| `gate.trust_registry.hits` | Counter | Consultas ao Trust Registry | Se 0, Trust Registry pode estar offline |
| `gate.escalations.total` | Counter | Escalações para Don | Se > 5/dia, revisar autonomia do Gate |
| `gate.ddnas.created` | Counter | DDNAs criados após Gate | Deve ser ≈ total de ativações (menos skips) |

### Dashboard

```yaml
gate_dashboard:
  widgets:
    - "Gauge: taxa de acerto do Gate (proceed acertou?)"
    - "Time series: ativações/dia por gatilho"
    - "Bar: custo médio do Gate vs custo médio da decisão"
    - "Table: últimas 10 execuções com outcome e confidence"
    - "Alert: quando gate.cost.ratio > 0.20"
```

---

## 12. Relacionados

| Documento | Relação | Localização |
|-----------|---------|-------------|
| **DECISION_DNA.md** | Formato canônico DDNA — Options populadas pelo Gate | `knowledge/architecture/DECISION_DNA.md` |
| **QUALITY_GATES.md** | G0.5 — Contrafactual Decision Review (gate specification) | `QUALITY_GATES.md` |
| **cosca-critic PROMPT.md** | Gate owner e executor | `agents/cosca-critic/PROMPT.md` |
| **KERNEL.md** | §10 Initialization Sequence — invocação entre Step 6 e Step 7 | `KERNEL.md` |
| **TRUST_REGISTRY.md** | F7.2 — Reputação histórica de agentes | `memory/trust/TRUST_REGISTRY.md` |
| **CONFIDENCE_MODEL.md** | Ponderação de evidências | `engines/evidence/CONFIDENCE_MODEL.md` |
| **RISK_REGISTRY.md** | Registro de riscos conhecidos | `memory/risk/RISK_REGISTRY.md` |
| **Cognitive Economy** | Cálculo de custo para G3 | `engines/cognitive-economy/SKILL.md` |
| **Cognitive Maturity** | Pipeline CMI — F1.2 | `workflows/cognitive-maturity-implementation.md` |
| **DECISION_DNA_FORMAT.md** | Versão agent-facing do DDNA | `memory/DECISION_DNA_FORMAT.md` |
| **COSCA_CONFIG** | Threshold configurável do Gate | `cosca.config.yaml` |
| **cognitive-audit-loop.md** | Auditoria cognitiva — verifica se Gate executou | `workflows/cognitive-audit-loop.md` |

---

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|--------|------|-------|------------|
| 2.0.0 | 2026-07-30 | cosca-architecture | Upgrade para engine de decisão: pipeline Gate Aberto/Gate Fechado, gatilhos G1-G6, integração DDNA (Options populadas, evidence linkada, confidence calculado), integração Trust Registry (F7.2), regras de escalação E1-E5, regras de design D1-D6, métricas + dashboard, threshold de custo configurável ($0.01), modo autônomo Don offline, YAML output v2 com todos os novos campos |
| 1.0.0 | 2026-07-30 | cosca-critic | Criação do Portão Contrafactual — gate mandatório para decisões estratégicas P0/P1 |

---

> **Owner**: cosca-critic | **Invoked by**: Kernel (entre Step 6 e Step 7) | **Mandatório para**: Decisões P0/P1 com ativação por gatilho | **Engine path**: `engines/decision/contrafactual-gate.md`
>
> *"Antes de decidir, pergunte: e se fosse diferente?"*
