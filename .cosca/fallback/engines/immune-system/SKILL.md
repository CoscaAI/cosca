# SISTEMA IMUNOLÓGICO COGNITIVO (INTERNO) — Agent Health Immune System ★ F2.2

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-agent-immune-system`
> **Fase**: Fase 2 (Motores) — F2.2 no cognitive-maturity-implementation.md
> **Conceito Cognitivo**: C5 no COGNITIVE_MATURITY.md
> **CMI Impact**: Consistência +8, Autocrítica +5
> **Dimensão**: Saúde do Agente — Patologias Cognitivas Internas
> **Complemento**: [cognitive-immune-system/SKILL.md](../cognitive-immune-system/SKILL.md) — defesa externa contra contaminação de conhecimento
>
> **Dependências**:
> - F1.4 [Wisdom Decay](../wisdom-decay/WISDOM_DECAY.md) — contradições alimentam freshness score
> - F1.6 [Cognitive Entropy](../cognitive-entropy/ENTROPY.md) — desorganização como sintoma
> - F7.2 [Trust Registry](../../memory/trust/TRUST_REGISTRY.md) — reputação histórica como sinal vital
> - F1.2 [Contrafactual Gate](../../workflows/contrafactual-gate.md) — alternativas como diagnóstico diferencial
> - F2.1 [Cognitive Economy](../cognitive-economy/SKILL.md) — ROI reduzido para agentes doentes
> - F7.1 [Prediction](../prediction/SKILL.md) — desconto de confidence para agentes doentes

---

## ÍNDICE

1. [Definição e Analogia](#1-definição-e-analogia)
2. [Antígenos Cognitivos — Catálogo de Patologias](#2-antígenos-cognitivos--catálogo-de-patologias)
3. [Mecanismo de Imunidade — 3 Camadas](#3-mecanismo-de-imunidade--3-camadas)
4. [Pipeline Pós-Task](#4-pipeline-pós-task)
5. [Tabela de Antígenos (Resumo)](#5-tabela-de-antígenos-resumo)
6. [Integração com F2.1 Cognitive Economy](#6-integração-com-f21-cognitive-economy)
7. [Integração com F7.1 Prediction Engine](#7-integração-com-f71-prediction-engine)
8. [Integração com F1.4 Wisdom Decay](#8-integração-com-f14-wisdom-decay)
9. [Integração com F7.2 Trust Registry](#9-integração-com-f72-trust-registry)
10. [Imunidade Presidencial (Don Override)](#10-imunidade-presidencial-don-override)
11. [Vacinas Reversíveis](#11-vacinas-reversíveis)
12. [Exemplos Práticos](#12-exemplos-práticos)
13. [Métricas do Engine](#13-métricas-do-engine)
14. [Restrições de Performance](#14-restrições-de-performance)
15. [Referências](#15-referências)

---

## 1. DEFINIÇÃO E ANALOGIA

### 1.1 O que é o Sistema Imunológico Cognitivo Interno

O **Cognitive Immune System (Internal)** é o sistema de **autodefesa cognitiva dos agentes**. Assim como o sistema imunológico biológico identifica e ataca invasores (vírus, bactérias) e também células defeituosas do próprio corpo (câncer, auto-imunes), este sistema identifica e bloqueia **padrões cognitivos ruins** nos próprios agentes da plataforma Cosca.

| Sistema Biológico | Sistema Cognitivo Interno |
|---|---|
| Vírus/bactérias invadem o corpo | Conhecimento contaminado tenta entrar na base |
| **Células próprias viram cancerosas** | **Agentes desenvolvem padrões cognitivos patológicos** |
| Sistema imune inato (barreiras físicas, respostas rápidas) | Camada 1 — Regras fixas imediatas (superconfiança, contradição rápida) |
| Sistema imune adaptativo (anticorpos específicos, memória) | Camada 2 — Regras que aprendem com repetição |
| Memória imunológica (células B e T de memória) | Camada 3 — Registry de vacinas e doenças curadas |

### 1.2 Relação com o Cognitive Immune System Externo

Este engine é **complementar** ao `cognitive-immune-system/SKILL.md` (F2.2 externo):

| Aspecto | Immune System Externo | Immune System Interno (este) |
|---|---|---|
| **O que defende** | Base de conhecimento contra contaminação externa | Agentes contra patologias cognitivas internas |
| **O que detecta** | Claims falsas, contradições entre fontes, evidência insuficiente | Superconfiança, viés de confirmação, aprendizado falso |
| **Quando atua** | Na ingestão de conhecimento (pré-escrita) | Pós-task, na avaliação do agente |
| **Alvo** | O conhecimento em si | O comportamento do agente |
| **Analogia** | Sistema imune contra infecções externas | Sistema imune contra câncer (células próprias defeituosas) |

### 1.3 O que são "Antígenos Cognitivos"

**Antígeno cognitivo** é qualquer padrão replicável de comportamento de agente que:
1. **Degrada a qualidade das decisões** do agente ou de quem confia nele
2. **Persiste no tempo** (não é um erro isolado, é um padrão)
3. **É detectável por regras** (pode ser identificado por análise de metadados do agente)
4. **Tem cura conhecida** (existe uma ação corretiva documentada)

> **Analogia**: Um antígeno biológico é uma molécula estranha que o sistema imune reconhece. Um antígeno cognitivo é um padrão de comportamento que o Immune System reconhece como "não saudável" e contra o qual pode agir.

---

## 2. ANTÍGENOS COGNITIVOS — CATÁLOGO DE PATOLOGIAS

### 2.1 AG-001 — Superconfiança (Superconfidence)

| Propriedade | Valor |
|---|---|
| **Código** | AG-001 |
| **Nome** | Superconfiança |
| **Severidade** | Moderada |
| **Detecção** | `confidence do agente > 0.90` **AND** `success_rate < 0.80` |
| **Janela** | Últimas 20 tasks ou 7 dias (o que for maior) |
| **Falso positivo** | Agente novo com poucas tasks (N < 5) — não aplicar se N tasks < 5 |

**Mecanismo**:
```
SE confidence_atual > 0.90
   E success_rate_ultimas_20 < 0.80
   E total_tasks >= 5
ENTÃO → ANTÍGENO DETECTADO: Superconfiança
AÇÃO → Aplicar "vacina" (reduzir peso do confidence em 0.1)
```

**Exemplo real**:
```
Agente: cosca-documentation
confidence: 0.93
success_rate (últimas 20): 0.65 (13/20 sucessos)
tasks_total: 24
→ AG-001 detectado
→ confidence_cap aplicado: 0.83 (0.93 - 0.10)
→ Trust Registry: confidence_delta = -0.10
```

**Cura**: Redução do peso do confidence nas decisões em 0.1 até que success_rate volte a >= 0.85 por 10 tasks consecutivas. A vacina reverte automaticamente quando a condição de cura é satisfeita.

---

### 2.2 AG-002 — Viés de Confirmação (Confirmation Bias)

| Propriedade | Valor |
|---|---|
| **Código** | AG-002 |
| **Nome** | Viés de Confirmação |
| **Severidade** | Alta |
| **Detecção** | Agente sempre confirma decisão anterior sem reavaliar |
| **Janela** | Últimas 10 decisões P0/P1 |
| **Falso positivo** | Decisões em série onde a primeira estava correta e as subsequentes são consistentes — verificar se há reavaliação explícita |

**Detecção detalhada**:
1. Agente participa de 3+ decisões P0/P1 consecutivas no mesmo domínio
2. Em todas, o agente **não propõe alternativa** e **não questiona premissas** da decisão anterior
3. O confidence nas decisões subsequentes **não varia** (confidence_delta ≈ 0)
4. Trust Registry mostra que decisões similares anteriores tiveram `outcome` variado (nem todas success)

```
SE N decisões_consecutivas_sem_alternativa >= 3
   E confidence_delta_médio < 0.01
   E nem_todas_foram_success
ENTÃO → ANTÍGENO DETECTADO: Viés de Confirmação
AÇÃO → Alerta + gatilho de Contrafactual Gate obrigatório nas próximas 3 decisões
```

**Cura**: Agente é forçado a passar pelo [Contrafactual Gate](../../workflows/contrafactual-gate.md) nas próximas 3 decisões P0/P1. Após 3 decisões com alternativa proposta e reavaliação documentada, o alerta é removido.

---

### 2.3 AG-003 — Aprendizado Falso (False Learning)

| Propriedade | Valor |
|---|---|
| **Código** | AG-003 |
| **Nome** | Aprendizado Falso |
| **Severidade** | Crítica |
| **Detecção** | Learning registrado no `learnings.md` **sem evidência correspondente no diff do commit** |
| **Janela** | Tempo real (detectado no pós-task imediato) |
| **Falso positivo** | Learnings conceituais ou de decisão que não geram diff (ex: decisão de não fazer nada) — estes devem ter `outcome: decision-only` explícito |

**Detecção detalhada**:
1. Agente registra entrada em `learnings.md` com `outcome: success`
2. Engine verifica: o commit associado (`commit_hash`) ou o diff da task tem alterações que **evidenciam** o aprendizado?
3. Se o learning alega "Implementei X" mas o diff não mostra alterações relacionadas a X → **Falso Aprendizado**
4. Se o learning alega "Descobri que Y é verdade" mas não há referência a Y no diff ou em arquivos de conhecimento → **Falso Aprendizado**

```
SE learning.outcome == "success"
   E learning.evidence_ref NÃO contém diff compatível
   E learning.type != "decision-only"
ENTÃO → ANTÍGENO DETECTADO: Aprendizado Falso
AÇÃO → Quarentena do learning (não entra na base ativa) + notificação
```

**Cura**: Learning movido para quarentena. Agente precisa re-executar a task ou fornecer evidência concreta. Se for falso positivo (learning conceitual legítimo), marcar `#conceptual` e liberar.

---

### 2.4 AG-004 — Contradição Crônica (Chronic Contradiction)

| Propriedade | Valor |
|---|---|
| **Código** | AG-004 |
| **Nome** | Contradição Crônica |
| **Severidade** | Alta |
| **Detecção** | Mesmo agente aprende **A e não-A** em intervalo <= 48h |
| **Janela** | 48h |
| **Falso positivo** | Contexto mudou entre as duas tasks (ex: requisito mudou) — verificar se há ADR ou decisão documentada que justifica a mudança |

**Detecção detalhada**:
1. Agente registra learning L1 com afirmação A
2. Em menos de 48h, mesmo agente registra learning L2 com afirmação ¬A (mesmo domínio, tags com sobreposição > 60%)
3. Não há DDNA, ADR ou decisão documentada que explique a mudança de posição

```
SE par_de_contradicao.timestamp_diff <= 48h
   E par_de_contradicao.mesmo_agente == true
   E par_de_contradicao.sem_justificativa == true
ENTÃO → ANTÍGENO DETECTADO: Contradição Crônica
AÇÃO → Bloquear novo learning do agente até revisão manual
```

**Cura**: Agente fica impedido de registrar novos aprendizados até que:
1. Um DDNA de resolução de contradição seja criado (explicando qual lado prevalece e por que)
2. O Don ou Architecture Chief aprove a resolução
3. O learning contraditório é marcado como `#superseded`

---

### 2.5 AG-005 — Memória Inflada (Inflated Memory)

| Propriedade | Valor |
|---|---|
| **Código** | AG-005 |
| **Nome** | Memória Inflada |
| **Severidade** | Moderada |
| **Detecção** | Agente com **50+ learnings** registrados mas **0 patterns extraídos** |
| **Janela** | Histórico completo do agente |
| **Falso positivo** | Agente muito novo que ainda não passou pelo pipeline de extração de padrões — NÃO aplicar se agente existe há < 7 dias |

**Detecção detalhada**:
1. Agente tem N learnings registrados em `learnings.md`
2. Agente tem M patterns extraídos em `patterns.md` ou no campo `patterns_extracted`
3. Se N >= 50 **E** M == 0 → **Memória Inflada**

```
SE total_learnings >= 50
   E total_patterns == 0
   E agent_age_days >= 7
ENTÃO → ANTÍGENO DETECTADO: Memória Inflada
AÇÃO → Compressão forçada: executa pipeline de extração de padrões (forçado)
```

**Cura**: Pipeline de compressão forçada:
1. Engine executa extração automática de padrões dos 50+ learnings (usando similaridade semântica)
2. Agrupa aprendizados redundantes ou similares em patterns
3. Marca aprendizados consolidados como `#compressed`
4. Reduz contagem de learnings ativos (move consolidados para seção de archive)
5. Gera patterns.md seed para o agente

> **Nota**: AG-005 não é necessariamente uma patologia — pode ser apenas falta de maturidade do agente. Mas é um sinal de que o agente "acumula mas não destila", que é um comportamento cognitivo insalubre no longo prazo.

---

## 3. MECANISMO DE IMUNIDADE — 3 CAMADAS

O sistema imunológico opera em 3 camadas, inspiradas no sistema imunológico biológico:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    COGNITIVE IMMUNE SYSTEM — 3 CAMADAS                         │
│                                                                                │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │                     CAMADA 1 — IMUNIDADE INATA                             │ │
│  │                      (Regras Fixas, Resposta Imediata)                      │ │
│  │                                                                             │ │
│  │  • Superconfiança (AG-001): confidence > 0.90 e success_rate < 0.80        │ │
│  │    → Reduz peso do confidence em 0.1                                        │ │
│  │                                                                             │ │
│  │  • Contradição em 48h (AG-004): mesmo agente aprende A e não-A             │ │
│  │    → Bloqueia novo learning até revisão                                     │ │
│  │                                                                             │ │
│  │  • Confidence sem lastro: capability-profile sem tasks registradas          │ │
│  │    → Cap de confidence em 0.5 até que execute ao menos 1 task              │ │
│  │                                                                             │ │
│  │  • Características:                                                         │ │
│  │    - Sempre ativas (não requerem histórico)                                 │ │
│  │    - Execução O(1) — consulta a metadados do agente                        │ │
│  │    - Acionamento imediato, sem necessidade de aprendizado prévio            │ │
│  └──────────────────────────────────────────────────────────────────────────┘ │
│                                    │                                           │
│                                    ▼                                           │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │                   CAMADA 2 — IMUNIDADE ADAPTATIVA                          │ │
│  │                   (Aprende com a Repetição de Padrões)                      │ │
│  │                                                                             │ │
│  │  • Se padrão X se repete 3x no mesmo agente:                               │ │
│  │    → Cria regra permanente para aquele agente                               │ │
│  │    → Ex: "cosca-testing sempre superconfiante após 3 detecções consecutivas │ │
│  │       de AG-001"                                                             │ │
│  │    → Regra: cosca-testing → confidence_cap automático = 0.80               │ │
│  │                                                                             │ │
│  │  • Se padrão X se repete em 3+ agentes diferentes:                         │ │
│  │    → Cria regra global para todos os agentes                                │ │
│  │    → Ex: "3+ agentes com AG-002 (viés de confirmação) no domínio           │ │
│  │       'architecture-design'"                                                 │ │
│  │    → Regra global: todo agente no domínio 'architecture-design' tem         │ │
│  │       Contrafactual Gate obrigatório por 5 decisões                         │ │
│  │                                                                             │ │
│  │  • Agente "imune" a um padrão depois de N correções bem-sucedidas:         │ │
│  │    → Se agente foi corrigido N vezes (3-5) para o mesmo antígeno            │ │
│  │    → E está há X tasks sem reincidir (10-20)                                │ │
│  │    → Agente desenvolve "imunidade" — a regra deixa de ser aplicada          │ │
│  │    → A imunidade NÃO é permanente — expira em 30 dias sem tasks            │ │
│  │                                                                             │ │
│  │  • Características:                                                         │ │
│  │    - Requer histórico mínimo de 3 ocorrências                              │ │
│  │    - Regras são persistidas no registry de imunidade adaptativa             │ │
│  │    - Regras globais têm precedência sobre regras individuais                │ │
│  └──────────────────────────────────────────────────────────────────────────┘ │
│                                    │                                           │
│                                    ▼                                           │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │                  CAMADA 3 — MEMÓRIA IMUNOLÓGICA                            │ │
│  │            (Registry de Vacinas Aplicadas e Doenças Curadas)                │ │
│  │                                                                             │ │
│  │  • Registry de "vacinas aplicadas":                                        │ │
│  │    ├── agent: cosca-testing                                                 │ │
│  │    ├── antigen: AG-001 (Superconfiança)                                    │ │
│  │    ├── vaccine: confidence_cap = 0.80                                      │ │
│  │    ├── applied_at: 2026-07-30T10:00:00Z                                    │ │
│  │    ├── expires_at: 2026-08-30T10:00:00Z (ou quando curado)                │ │
│  │    ├── status: active | expired | reversed                                 │ │
│  │    └── ddna_id: IMM-2026-07-30-001                                         │ │
│  │                                                                             │ │
│  │  • Registry de "doenças cognitivas curadas":                               │ │
│  │    ├── agent: cosca-testing                                                 │ │
│  │    ├── antigen: AG-001                                                     │ │
│  │    ├── detected_at: 2026-07-30T10:00:00Z                                  │ │
│  │    ├── cured_at: 2026-08-15T10:00:00Z                                     │ │
│  │    ├── cure: "success_rate manteve-se >= 0.85 por 10 tasks consecutivas"  │ │
│  │    └── immunity_until: 2026-09-15T10:00:00Z (expira em 30 dias)           │ │
│  │                                                                             │ │
│  │  • DDNA para cada intervenção:                                             │ │
│  │    Toda vez que uma vacina é aplicada, um DDNA de imunização é gerado       │ │
│  │    → IMM-{YYYY-MM-DD}-{NNN} registrando: antígeno, agente, ação, motivo    │ │
│  │                                                                             │ │
│  │  • Características:                                                         │ │
│  │    - Persistência em YAML parseável                                        │ │
│  │    - Toda intervenção tem DDNA vinculado                                   │ │
│  │    - Vacinas têm data de expiração (cura ou reversão)                      │ │
│  └──────────────────────────────────────────────────────────────────────────┘ │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 3.1 Detalhamento — Camada 1: Imunidade Inata

Regras fixas que **sempre se aplicam**, independentemente de histórico do agente:

| Regra | Gatilho | Ação | Cura | Performance |
|---|---|---|---|---|
| **Superconfiança** | confidence > 0.90 AND success_rate < 0.80 (últimas 20) | Reduz peso do confidence em 0.1 | Success_rate >= 0.85 por 10 tasks | O(1) — 2 consultas |
| **Contradição 48h** | A e não-A no mesmo agente, intervalo <= 48h, sem justificativa | Bloqueia novo learning até revisão | DDNA de resolução criado + Don aprova | O(log N) — busca semântica |
| **Confidence sem lastro** | capability-profile com confidence > 0.5 mas 0 tasks registradas | Cap de confidence em 0.5 | 1 task executada com sucesso | O(1) — 1 consulta |
| **Aprendizado falso** (AG-003) | learning com `outcome=success` sem diff correspondente | Quarentena do learning | Evidência fornecida ou reclassificado | O(1) — verificação de diff |
| **Memória inflada** (AG-005) | learnings >= 50 AND patterns == 0 AND idade >= 7 dias | Compressão forçada | Pipeline de extração executado | O(N log N) — clustering |

### 3.2 Detalhamento — Camada 2: Imunidade Adaptativa

A Camada 2 cria **regras dinâmicas** baseadas em recorrência. O algoritmo:

```
function adaptive_immunity(agent, antigen):
    // Passo 1: Verifica recorrência no mesmo agente
    same_agent_count = count_occurrences(agent, antigen, window=30dias)
    
    if same_agent_count >= 3:
        // Cria regra permanente para este agente
        create_permanent_rule(
            agent = agent,
            antigen = antigen,
            rule = "automatic_vaccination",
            severity = antigen.severity,
            valid_until = null  // permanente até reversão explícita
        )
        log("Regra permanente criada: {agent} → {antigen}")
    
    // Passo 2: Verifica recorrência em múltiplos agentes
    multi_agent_count = count_agents_with_pattern(antigen, window=30dias)
    
    if multi_agent_count >= 3:
        affected_domains = extract_common_domains(agents_com_antigen)
        for domain in affected_domains:
            create_global_rule(
                domain = domain,
                antigen = antigen,
                rule = "mandatory_gate",
                valid_until = now + 30dias  // global rules são temporárias
            )
        log("Regra global criada: {affected_domains} → {antigen}")
    
    // Passo 3: Verifica imunidade adquirida
    if count_corrections(agent, antigen) >= N_corrections  // 3-5
       AND tasks_since_last_incident(agent, antigen) >= X_tasks  // 10-20:
        
        grant_immunity(
            agent = agent,
            antigen = antigen,
            expires_at = now + 30dias
        )
        log("Imunidade concedida: {agent} → {antigen} por 30 dias")
```

### 3.3 Detalhamento — Camada 3: Memória Imunológica

#### Schema do Registry de Vacinas

```yaml
# internal/embed/cosca/engines/immune-system/registry/vaccines.yaml
vaccines:
  - id: "VAC-2026-07-30-001"
    agent: "cosca-testing"
    antigen: "AG-001"
    antigen_name: "Superconfiança"
    severity: "moderate"
    detection_evidence:
      confidence: 0.93
      success_rate: 0.65
      total_tasks: 24
      window: "last_20"
    vaccine:
      type: "confidence_reduction"
      delta: -0.10
      new_cap: 0.83
    applied_at: "2026-07-30T10:00:00Z"
    expires_at: "2026-08-30T10:00:00Z"     # Reavaliação periódica
    status: "active"                         # active | expired | reversed
    cure_condition:
      metric: "success_rate"
      target: ">= 0.85"
      duration_tasks: 10
    ddna_id: "IMM-2026-07-30-001"
    ddna_ref: "internal/embed/cosca/knowledge/architecture/immune-ddnas/IMM-2026-07-30-001.md"
    reversed_at: null
    reversal_reason: null
```

#### Schema do Registry de Doenças Curadas

```yaml
# internal/embed/cosca/engines/immune-system/registry/cured.yaml
cured_diseases:
  - id: "CURED-2026-08-15-001"
    agent: "cosca-testing"
    antigen: "AG-001"
    antigen_name: "Superconfiança"
    first_detected_at: "2026-07-30T10:00:00Z"
    last_detected_at: "2026-08-05T10:00:00Z"
    cured_at: "2026-08-15T10:00:00Z"
    total_occurrences: 4
    cure_description: "success_rate manteve-se >= 0.85 por 10 tasks consecutivas"
    immunity_granted: true
    immunity_expires_at: "2026-09-15T10:00:00Z"
    relapse_count: 0
```

#### DDNA de Intervenção Imunológica

```markdown
# IMM-2026-07-30-001 — Vacinação: Superconfiança (AG-001)

**Agente**: cosca-testing
**Antígeno**: AG-001 — Superconfiança
**Tipo**: confidence_reduction
**Delta**: -0.10 (0.93 → 0.83)

## Contexto
O agente cosca-testing apresenta confidence (0.93) significativamente
superior ao seu success_rate (0.65 nas últimas 20 tasks). Este padrão
indica superconfiança — o agente acredita ter mais capacidade do que
seus resultados demonstram.

## Decisão
Aplicar vacina de redução de peso do confidence em 0.1.

## Justificativa
Confidence sem lastro de sucesso distorce o sistema de delegação
(F7.1 Prediction Engine). Agentes superconfiantes tendem a ser
escolhidos para tasks que não conseguem executar, gerando retrabalho.

## Duração
Até que success_rate >= 0.85 por 10 tasks consecutivas.
Reversão automática ao cumprir a condição de cura.

## Consequências
- Prediction Engine passa a usar confidence_cap = 0.83
- Cognitive Economy aplica risk_penalty adicional de 10%
- Agente notificado: "Seu confidence foi ajustado. Melhore seu success_rate."

## Don Override
Don pode reverter esta vacina a qualquer momento via comando:
`immune override --vaccine VAC-2026-07-30-001`
```

---

## 4. PIPELINE PÓS-TASK

O pipeline executa **após cada task completa**, com budget máximo de **200ms**. Ele varre apenas o agente que executou a task (não todos os 54 agentes).

### 4.1 Visão Geral

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    IMMUNE SYSTEM PIPELINE (Pós-Task, ~200ms)                   │
│                                                                                │
│  TRIGGER: Toda task completa (após Trust Registry update, antes de learning)   │
│                                                                                │
│  ┌────────────────────────────────────────────────────────────────────────┐   │
│  │ PASSO 1: VARRER METADADOS DO AGENTE                                     │   │
│  │                                                                          │   │
│  │  Input: agent_name, última task executada                               │   │
│  │  Ação: Carregar capability-profile + últimas N learnings + Trust Registry│   │
│  │                                                                          │   │
│  │  Dados carregados:                                                       │   │
│  │  ├── confidence_atual (do capability-profile ou Trust Registry)          │   │
│  │  ├── success_rate_ultimas_20 (Trust Registry, calculado)                │   │
│  │  ├── ultimo_learning (learnings.md, última entrada)                     │   │
│  │  ├── total_learnings + total_patterns                                   │   │
│  │  ├── últimas_N_decisões_com_alternativas (últimas 10)                  │   │
│  │  └── últimas_contradições (wisdom-decay, últimas 48h)                  │   │
│  │                                                                          │   │
│  │  ⏱ Tempo alvo: < 30ms                                                  │   │
│  └────────────────────────────────┬───────────────────────────────────────┘   │
│                                   │                                           │
│                                   ▼                                           │
│  ┌────────────────────────────────────────────────────────────────────────┐   │
│  │ PASSO 2: APLICAR REGRAS INATAS (Camada 1)                               │   │
│  │                                                                          │   │
│  │  Para cada regra inata, executa verificação:                            │   │
│  │                                                                          │   │
│  │  ├── AG-001 Superconfiança:                                             │   │
│  │  │   confidence > 0.90 AND success_rate < 0.80?                        │   │
│  │  │   → SIM: aplica vacina (confidence_delta = -0.10)                   │   │
│  │  │                                                                       │   │
│  │  ├── AG-003 Aprendizado Falso:                                          │   │
│  │  │   Learning sem diff?                                                 │   │
│  │  │   → SIM: move para quarentena                                        │   │
│  │  │                                                                       │   │
│  │  ├── AG-004 Contradição 48h:                                            │   │
│  │  │   A e não-A no mesmo agente em 48h?                                 │   │
│  │  │   → SIM: bloqueia próximo learning                                   │   │
│  │  │                                                                       │   │
│  │  ├── AG-005 Memória Inflada:                                            │   │
│  │  │   >= 50 learnings AND 0 patterns?                                   │   │
│  │  │   → SIM: dispara compressão forçada                                  │   │
│  │  │                                                                       │   │
│  │  └── Confidence sem lastro:                                             │   │
│  │      confidence > 0.5 AND 0 tasks?                                     │   │
│  │      → SIM: cap em 0.5                                                  │   │
│  │                                                                          │   │
│  │  ⏱ Tempo alvo: < 50ms (5 verificações O(1) cada)                     │   │
│  └────────────────────────────────┬───────────────────────────────────────┘   │
│                                   │                                           │
│                                   ▼                                           │
│  ┌────────────────────────────────────────────────────────────────────────┐   │
│  │ PASSO 3: VERIFICAR PADRÕES HISTÓRICOS (Camada 2)                        │   │
│  │                                                                          │   │
│  │  Para cada antígeno detectado no Passo 2:                              │   │
│  │                                                                          │   │
│  │  ├── Este é o padrão X detectado 3x no mesmo agente?                   │   │
│  │  │   → SIM: cria regra permanente (Camada 2)                            │   │
│  │  │                                                                       │   │
│  │  ├── Este é o padrão X detectado em 3+ agentes?                        │   │
│  │  │   → SIM: cria regra global (Camada 2)                                │   │
│  │  │                                                                       │   │
│  │  └── Agente tem imunidade a este antígeno?                              │   │
│  │      → SIM: não aplica vacina (skip)                                    │   │
│  │                                                                          │   │
│  │  ⏱ Tempo alvo: < 50ms (3 verificações em registry)                  │   │
│  └────────────────────────────────┬───────────────────────────────────────┘   │
│                                   │                                           │
│                                   ▼                                           │
│  ┌────────────────────────────────────────────────────────────────────────┐   │
│  │ PASSO 4: CLASSIFICAR E AGIR POR SEVERIDADE                              │   │
│  │                                                                          │   │
│  │  Para cada antígeno detectado (não imune, não skip):                    │   │
│  │                                                                          │   │
│  │  ┌─────────────────────────────────────────────────────────────────┐    │   │
│  │  │ SEVERIDADE LEVE                                                    │    │   │
│  │  │ AG-001 (Superconfiança), AG-005 (Memória Inflada)                 │    │   │
│  │  │ Ação:                                                              │    │   │
│  │  │ ├── Registra no Trust Registry como -confidence_delta            │    │   │
│  │  │ ├── Aplica vacina (redução de peso)                              │    │   │
│  │  │ └── Notifica agente (opcional)                                   │    │   │
│  │  └─────────────────────────────────────────────────────────────────┘    │   │
│  │                                                                          │   │
│  │  ┌─────────────────────────────────────────────────────────────────┐    │   │
│  │  │ SEVERIDADE MODERADA                                                 │    │   │
│  │  │ AG-002 (Viés de Confirmação), AG-004 (Contradição Crônica)        │    │   │
│  │  │ Ação:                                                              │    │   │
│  │  │ ├── Registra no Trust Registry como -confidence_delta            │    │   │
│  │  │ ├── Bloqueia próximo learning até correção                       │    │   │
│  │  │ ├── Dispara Contrafactual Gate obrigatório                       │    │   │
│  │  │ └── Notifica Architecture Chief                                  │    │   │
│  │  └─────────────────────────────────────────────────────────────────┘    │   │
│  │                                                                          │   │
│  │  ┌─────────────────────────────────────────────────────────────────┐    │   │
│  │  │ SEVERIDADE GRAVE                                                    │    │   │
│  │  │ AG-003 (Aprendizado Falso)                                        │    │   │
│  │  │ Ação:                                                              │    │   │
│  │  │ ├── Abre DDNA de incidente imunológico                           │    │   │
│  │  │ ├── Bloqueia o learning (quarentena)                             │    │   │
│  │  │ ├── Notifica Don imediatamente                                   │    │   │
│  │  │ └── Agenda revisão de integridade do agente                      │    │   │
│  │  └─────────────────────────────────────────────────────────────────┘    │   │
│  │                                                                          │   │
│  │  ⏱ Tempo alvo: < 50ms (escrita em registry simples)                 │   │
│  └────────────────────────────────┬───────────────────────────────────────┘   │
│                                   │                                           │
│                                   ▼                                           │
│  ┌────────────────────────────────────────────────────────────────────────┐   │
│  │ PASSO 5: ATUALIZAR MEMÓRIA IMUNOLÓGICA                                  │   │
│  │                                                                          │   │
│  │  Para cada intervenção aplicada:                                        │   │
│  │  ├── Registra no vaccine registry (Camada 3)                            │   │
│  │  ├── Cria DDNA de intervenção (IMM-{date}-{seq})                       │   │
│  │  ├── Atualiza contagem de ocorrências do padrão                        │   │
│  │  └── Se cura foi atingida: registra no cured registry + revoga vacina  │   │
│  │                                                                          │   │
│  │  ⏱ Tempo alvo: < 20ms (append em YAML)                             │   │
│  └────────────────────────────────────────────────────────────────────────┘   │
│                                                                                │
│  TOTAL: ~200ms (30 + 50 + 50 + 50 + 20) ✅                                   │
│                                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Pseudocódigo do Pipeline

```
function immune_system_pipeline(agent, task_result):
    // PASSO 1: Carregar metadados (< 30ms)
    profile = load_capability_profile(agent)
    registry = load_trust_registry(agent)
    learnings = load_recent_learnings(agent, 48h)
    contradictions = load_recent_contradictions(agent, 48h)
    
    // PASSO 2: Regras Inatas (< 50ms)
    detections = []
    
    // AG-001: Superconfiança
    if profile.confidence > 0.90 
       and registry.success_rate.last_20 < 0.80
       and registry.total_tasks >= 5:
        detections.push({
            antigen: "AG-001",
            severity: "light",
            evidence: {confidence: profile.confidence, 
                      success_rate: registry.success_rate.last_20},
            action: "reduce_confidence",
            delta: -0.10
        })
    
    // AG-003: Aprendizado Falso
    if task_result.has_learning 
       and not task_result.has_evidence_in_diff
       and task_result.learning_type != "decision-only":
        detections.push({
            antigen: "AG-003",
            severity: "critical",
            evidence: {learning: task_result.learning, diff: task_result.diff},
            action: "quarantine_learning"
        })
    
    // AG-004: Contradição Crônica
    if contradictions.has_active_pair_in_48h(agent):
        detections.push({
            antigen: "AG-004",
            severity: "moderate",
            evidence: contradictions.get_active_pair(agent),
            action: "block_learning_until_review"
        })
    
    // AG-005: Memória Inflada
    if learnings.total >= 50 
       and profile.patterns_extracted == 0
       and agent.age_days >= 7:
        detections.push({
            antigen: "AG-005",
            severity: "light",
            evidence: {total_learnings: learnings.total, patterns: 0},
            action: "force_compression"
        })
    
    // AG-002: Viés de Confirmação (check mais caro, executar sob demanda)
    if registry.last_10_decisions.all_without_alternative
       and registry.last_10_decisions.confidence_delta_avg < 0.01:
        detections.push({
            antigen: "AG-002",
            severity: "moderate",
            evidence: registry.last_10_decisions,
            action: "mandatory_contrafactual_gate"
        })
    
    // Confidence sem lastro (regra extra, não é antígeno nominal)
    if profile.confidence > 0.5 and registry.total_tasks == 0:
        profile.confidence = min(profile.confidence, 0.5)
    
    // PASSO 3: Verificar padrões históricos (< 50ms)
    for detection in detections:
        // Verifica imunidade (Camada 2)
        if has_immunity(agent, detection.antigen):
            continue  // Skip — agente imune
        
        // Verifica recorrência 3x no mesmo agente
        if count_pattern(agent, detection.antigen, 30dias) >= 3:
            create_permanent_rule(agent, detection.antigen, detection.action)
        
        // Verifica recorrência em 3+ agentes
        if count_agents_with_pattern(detection.antigen, 30dias) >= 3:
            create_global_rule(detection.antigen)
    
    // PASSO 4: Aplicar ações por severidade (< 50ms)
    for detection in detections:
        if has_immunity(agent, detection.antigen):
            continue
        
        switch detection.severity:
            case "light":
                registry.confidence_delta -= 0.10
                apply_vaccine(agent, detection)
                
            case "moderate":
                registry.confidence_delta -= 0.15
                block_next_learning(agent)
                trigger_contrafactual_gate(agent)
                
            case "critical":
                registry.confidence_delta -= 0.30
                quarantine_learning(detection.evidence.learning)
                create_ddna("IMM", agent, detection)
                notify_don(agent, detection)
    
    // PASSO 5: Atualizar memória imunológica (< 20ms)
    for detection in detections:
        if not has_immunity(agent, detection.antigen):
            register_vaccine(agent, detection)
            increment_pattern_count(agent, detection.antigen)
    
    return {detections, vaccines_applied, total_duration_ms}
```

### 4.3 Gatilhos de Execução

| Gatilho | Disparado por | Ação | Prioridade |
|---|---|---|---|
| **Pós-task** | Kernel (Step 9 — Verify Result) | Pipeline completo para o agente da task | Alta |
| **Pré-task** | Kernel (Step 3 — Capability Resolution) | Verificar se agente está bloqueado por AG-004 | Alta |
| **Schedule** | Cron (diário) | Varredura completa de todos os agentes para regras globais (Camada 2) | Baixa (background) |
| **Manual** | CLI `immune scan --agent cosca-testing` | Pipeline completo para agente específico | Sob demanda |

---

## 5. TABELA DE ANTÍGENOS (RESUMO)

| Código | Antígeno | Severidade | Detecção | Ação Imediata | Cura |
|---|---|---|---|---|---|
| **AG-001** | Superconfiança | 🟡 Leve | confidence > 0.90 AND success_rate < 0.80 | Reduz peso -0.10 no confidence | success_rate >= 0.85 por 10 tasks |
| **AG-002** | Viés de Confirmação | 🟠 Moderado | 3+ decisões sem alternativa, confidence_delta ≈ 0 | Contrafactual Gate obrigatório por 3 decisões | 3 decisões com alternativa documentada |
| **AG-003** | Aprendizado Falso | 🔴 Grave | Learning com success mas sem diff | Quarentena + DDNA + notificar Don | Evidência fornecida ou reclassificado |
| **AG-004** | Contradição Crônica | 🟠 Moderado | A e não-A em <= 48h no mesmo agente | Bloqueia novo learning até revisão | DDNA de resolução + Don aprova |
| **AG-005** | Memória Inflada | 🟡 Leve | >= 50 learnings AND 0 patterns | Compressão forçada (extração automática) | Pipeline de extração executado |

### Pipeline por Severidade

```
SEVERIDADE LEVE (AG-001, AG-005):
  ├── Trust Registry: confidence_delta = -0.10
  ├── Vacina aplicada (reversível)
  └── Agente notificado (baixa prioridade)

SEVERIDADE MODERADA (AG-002, AG-004):
  ├── Trust Registry: confidence_delta = -0.15
  ├── Bloqueia próximo learning até correção
  ├── Contrafactual Gate obrigatório
  └── Architecture Chief notificado

SEVERIDADE GRAVE (AG-003):
  ├── Trust Registry: confidence_delta = -0.30
  ├── Learning em quarentena (não entra na base)
  ├── DDNA de incidente criado (IMM-{date}-{seq})
  └── Don notificado imediatamente
```

---

## 6. INTEGRAÇÃO COM F2.1 COGNITIVE ECONOMY

A Cognitive Economy Engine (F2.1) calcula o ROI de cada ação. Agentes com antígenos ativos têm seu ROI **reduzido em 30%** até cura.

### 6.1 Fórmula de ROI Ajustado por Saúde

```
ROI_cognitivo_base = (valor_total - custo_total) / custo_total   // F2.1 original

health_penalty = agente_tem_antigeno_ativo() ? 0.30 : 0.00
               // 0.30 = 30% de redução no ROI

ROI_ajustado = ROI_cognitivo_base × (1 - health_penalty)
```

### 6.2 Efeito Prático

| Estado do Agente | ROI Base | Health Penalty | ROI Ajustado | Impacto |
|---|---|---|---|---|
| Saudável | 2.50 (250%) | 0% | 2.50 | Normal |
| AG-001 ativo | 2.50 (250%) | 30% | 1.75 (175%) | -75% de retorno |
| AG-003 ativo | 2.50 (250%) | 30% | 1.75 (175%) | Mesmo, gravidade maior não multiplica penalidade |
| AG-002 + AG-004 | 2.50 (250%) | 30% | 1.75 (175%) | A penalidade não acumula (máx 30%) |

> **Nota**: A penalidade de saúde não se acumula com a penalidade de entropia (F1.6). O agente recebe a MAIOR das duas penalidades, não a soma. Isso evita dupla-penalização.

### 6.3 Feedback Loop

```
Immune System detecta AG-001 (Superconfiança)
  → Atualiza Trust Registry: confidence_delta = -0.10
  → Cognitive Economy consulta Trust Registry
  → Calcula health_penalty = 0.30
  → ROI_ajustado = ROI_base × 0.70
  → Decisão de delegação: agente "doente" é preterido
  → Agente executa menos tasks
  → Success_rate pode estabilizar
  → Se curar: health_penalty removido, ROI normaliza
```

### 6.4 Pipeline de Integração

```
┌────────────────────────────────────────────────────────────────────┐
│                    F2.1 + F2.2 INTEGRATION                           │
│                                                                     │
│  F2.2 (Immune System)              F2.1 (Cognitive Economy)        │
│  ┌────────────────────┐           ┌────────────────────┐            │
│  │ Detecta AG-001     │           │ Consulta Trust     │            │
│  │ no agente X        │──────────▶│ Registry para      │            │
│  │ Aplica vacina      │           │ health_status      │            │
│  │ Registry update    │           │ do agente X        │            │
│  └────────────────────┘           └─────────┬──────────┘            │
│                                              │                       │
│                                              ▼                       │
│                                    ┌────────────────────┐            │
│                                    │ ROI_ajustado =     │            │
│                                    │ ROI_base × 0.70   │            │
│                                    │ Decision Ladder    │            │
│                                    │ reduz 1 nível      │            │
│                                    └────────────────────┘            │
└────────────────────────────────────────────────────────────────────┘
```

---

## 7. INTEGRAÇÃO COM F7.1 PREDICTION ENGINE

O Prediction Engine (F7.1) **desconta o confidence de agentes "doentes"** ao calcular a probabilidade de sucesso de uma task.

### 7.1 Fórmula de Confidence Ajustado

```
P(success) = base_prediction × confidence_multiplier  // F7.1 original

immune_discount = agente_tem_antigeno_ativo() ? 0.15 : 0.00
               // 15% de desconto no confidence

P(success)_ajustado = P(success) × (1 - immune_discount)

// Adicionalmente, o confidence do agente usado no cálculo
// do Prediction Engine é substituído pelo confidence_vacinado
// (confidence original - delta da vacina)
```

### 7.2 Efeito Prático

| Estado do Agente | Confidence Original | Vacina (delta) | Confidence usado pelo F7.1 | P(success) base | Desconto Imune | P(success) final |
|---|---|---|---|---|---|---|
| Saudável | 0.90 | 0.00 | 0.90 | 0.85 | 0% | 0.85 |
| AG-001 ativo | 0.93 | -0.10 | 0.83 | 0.85 | 15% | 0.72 |
| AG-003 ativo | 0.85 | -0.30 | 0.55 | 0.85 | 15% | 0.72 |

### 7.3 Impacto na Delegação

```
F7.1 Prediction Engine:
  ┌─ Recebe request para task de testing
  ├─ Consulta Trust Registry
  ├─ cosca-testing tem AG-001 ativo → confidence_delta = -0.10
  ├─ P(success)_ajustado = 0.72 (vs 0.85 se saudável)
  ├─ Banda de risco: 🟡 (era 🟢 se saudável)
  ├─ Recomendação: "cosca-testing disponível, mas com risco moderado.
  │                 Considere cosca-qa como alternativa (saudável)."
  └─ Kernel decide baseado no ROI ajustado (F2.1)
```

### 7.4 Notificação ao Prediction Engine

Sempre que um antígeno é detectado ou curado, o Immune System notifica o Prediction Engine:

```yaml
immune_notification:
  to: "prediction-engine"
  event: "agent_health_changed"
  agent: "cosca-testing"
  changes:
    - antigen: "AG-001"
      status: "active"
      confidence_delta: -0.10
      effective_immediately: true
  timestamp: "2026-07-30T10:00:00Z"
```

---

## 8. INTEGRAÇÃO COM F1.4 WISDOM DECAY

As contradições detectadas pelo Immune System (AG-004 — Contradição Crônica) alimentam o **Freshness Score** do Wisdom Decay Engine.

### 8.1 Contradições como Input do Freshness Score

O Wisdom Decay (F1.4) já possui um `Contradiction Decay` mecanismo (§2.3) que penaliza o freshness de aprendizados contraditórios:

```
freshness = recency_term × 0.4 + usage_term × 0.3 + consistency_term × 0.2 + review_term × 0.1

consistency_term = max(0, 1 - min(contradiction_count / 5, 1.0))
```

O Immune System fornece **contradições detectadas por AG-004** como input adicional:

1. Quando AG-004 detecta uma contradição A vs ¬A, ambos os aprendizados têm `contradiction_count` incrementado
2. O Wisdom Decay, na próxima execução, recalcula o `consistency_term` com a nova contagem
3. O freshness de ambos os aprendizados cai, refletindo a contradição

### 8.2 Fluxo de Integração

```
AG-004 detecta: learning L1 ("Usar PostgreSQL") contradiz learning L2 ("Usar SQLite")
  → Immune System registra contradição no registry de contradições
  → Wisdom Decay (F1.4), na próxima varredura:
    ├── Lê contradições do Immune System
    ├── Incrementa contradiction_count de L1 e L2
    ├── Recalcula consistency_term = max(0, 1 - 1/5) = 0.80
    ├── Freshness de L1 e L2 reduzem
    └── Ambos entram em zona de "revisão necessária" se freshness cair abaixo de 0.30
```

### 8.3 Dados Compartilhados

Ambos os engines compartilham o mesmo registry de contradições, evitando duplicação:

```yaml
# Shared registry: internal/embed/cosca/engines/wisdom-decay/contradictions.yaml
# (escrito pelo Immune System, lido pelo Wisdom Decay)
contradictions:
  - id: "C-AG-004-001"
    detected_by: "immune-system"       # Fonte da detecção
    antigen: "AG-004"                  # Antígeno que detectou
    learning_a: "L1 — Usar PostgreSQL"
    learning_b: "L2 — Usar SQLite"
    agent: "cosca-architecture"        # Mesmo agente (crônica)
    detected_at: "2026-07-30T10:00:00Z"
    resolved: false
    freshness_impact: -0.20            # Penalidade aplicada pelo Wisdom Decay
```

---

## 9. INTEGRAÇÃO COM F7.2 TRUST REGISTRY

O Trust Registry (F7.2) é o **sistema de sinais vitais** que o Immune System monitora e modula.

### 9.1 Immune System → Trust Registry (Escrita)

Toda intervenção do Immune System gera uma entrada no Trust Registry:

| Antígeno | Campo Afetado | Delta | Frequência |
|---|---|---|---|
| AG-001 (Superconfiança) | `confidence_delta` | -0.10 | Por detecção |
| AG-002 (Viés de Confirmação) | `confidence_delta` | -0.15 | Por detecção |
| AG-003 (Aprendizado Falso) | `confidence_delta` | -0.30 | Por detecção |
| AG-004 (Contradição Crônica) | `confidence_delta` | -0.15 | Por detecção |
| AG-005 (Memória Inflada) | `confidence_delta` | -0.05 | Por compressão |

Formato da entrada no Trust Registry:

```yaml
- agent: cosca-testing
  task_type: immune-system-intervention
  task_id: immune-2026-07-30-001
  outcome: partial  # Vacina aplicada, paciente em tratamento
  confidence_before: 0.93
  confidence_after: 0.83
  confidence_delta: -0.10
  tags:
    - immune-system
    - antigen-AG-001
    - superconfidence
    - vaccine-applied
  timestamp: 2026-07-30T10:00:00Z
  evidence_ref: internal/embed/cosca/engines/immune-system/registry/vaccines.yaml
```

### 9.2 Trust Registry → Immune System (Leitura)

O Immune System consulta o Trust Registry para:
1. **Success Rate**: calcular AG-001 (superconfiança)
2. **Histórico de decisões**: calcular AG-002 (viés de confirmação)
3. **Confidence atual**: comparar com success_rate
4. **Últimas tasks**: verificar se há diff para AG-003

### 9.3 Impacto no Reliability Score

O `reliability_score` do Trust Registry (F7.2 §2.4) é recalibrado com as intervenções do Immune System:

```
reliability_score = success_rate_total × 0.60
                  + recency_score × 0.20
                  + complexity_score × 0.20
                  - immune_penalty  // ← NOVO: penalidade imunológica

Onde:
  immune_penalty = 0.10 se AG-001 ativo
                 = 0.15 se AG-002 ou AG-004 ativo
                 = 0.30 se AG-003 ativo
                 = 0.05 se AG-005 ativo
                 = 0.00 se saudável
```

---

## 10. IMUNIDADE PRESIDENCIAL (DON OVERRIDE)

O Don (usuário) tem **autoridade absoluta** sobre o Immune System. Qualquer vacina pode ser revertida, qualquer bloqueio pode ser suspenso.

### 10.1 Comandos de Override

```bash
# Reverter uma vacina específica
immune override --vaccine VAC-2026-07-30-001

# Reverter todas as vacinas de um agente
immune override --agent cosca-testing --all

# Conceder imunidade permanente a um agente para um antígeno
immune override --grant-immunity --agent cosca-testing --antigen AG-001 --permanent

# Suspender bloqueio de learning (AG-004)
immune override --unblock --agent cosca-architecture

# Reverter compressão forçada (AG-005)
immune override --restore --agent cosca-architecture --learnings

# Desativar Immune System para um agente específico
immune override --disable --agent cosca-testing
```

### 10.2 Registro de Override

Toda imunidade presidencial é registrada para auditoria:

```yaml
# No vaccine registry, campo override:
override:
  overridden_by: "don"
  overridden_at: "2026-07-30T15:00:00Z"
  reason: "Don avaliou que o confidence de cosca-testing é justificado
           pelos resultados excepcionais em testes de race condition,
           que não são capturados pelo success_rate geral."
  permanent: false
  expires_at: "2026-08-30T15:00:00Z"
```

### 10.3 Regras de Override

| Regra | Descrição |
|---|---|
| **O Don sempre pode override** | Nenhuma vacina ou bloqueio é irreversível |
| **Override é registrado** | Todo override gera entrada no registry com motivo |
| **Override não apaga o histórico** | A vacina original permanece no registry como `reversed` |
| **Override pode ser temporário** | Don pode definir data de expiração |
| **Override não desativa o sistema** | Apenas a vacina específica é suspensa |
| **Override audível** | O DDNA da intervenção original é atualizado com `superseded_by` |

---

## 11. VACINAS REVERSÍVEIS

Toda vacina aplicada pelo Immune System é **reversível**. O sistema segue o princípio de "intervenção mínima" — nunca faz algo que não possa ser desfeito.

### 11.1 Ciclo de Vida da Vacina

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                         VACCINE LIFE CYCLE                                     │
│                                                                                │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────────┐            │
│  │ DETECTED │───▶│ VACCINE  │───▶│ ACTIVE   │───▶│ CURADO /     │            │
│  │ (antígeno│    │ APPLIED  │    │ (vigor)   │    │ REVERTIDO    │            │
│  │ achado)  │    │          │    │          │    │              │            │
│  └──────────┘    └──────────┘    └──────────┘    └──────────────┘            │
│                                                      │                       │
│                                                      ├── CURADO:             │
│                                                      │   Condição de cura    │
│                                                      │   atingida → vacina   │
│                                                      │   removida autom.     │
│                                                      │                       │
│                                                      ├── REVERTIDO:          │
│                                                      │   Don override →      │
│                                                      │   vacina suspensa     │
│                                                      │                       │
│                                                      └── EXPIRADO:           │
│                                                          Tempo máximo sem    │
│                                                          cura → reavaliar    │
│                                                                              │
│  REGRAS DE REVERSÃO:                                                        │
│  ├── Vacina revertida via Don: status = "reversed", reversal_reason         │
│  ├── Vacina expirada por cura: status = "expired", moved to cured registry  │
│  ├── Agente curado: ganha imunidade temporária (30 dias) para o antígeno    │
│  └── Vacina revertida NÃO impede nova detecção futura                       │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 11.2 Condições de Cura por Antígeno

| Antígeno | Condição de Cura | Verificação | Reversão Automática |
|---|---|---|---|
| AG-001 (Superconfiança) | success_rate >= 0.85 por 10 tasks consecutivas | A cada task do agente | Sim, automática |
| AG-002 (Viés de Confirmação) | 3 decisões com alternativa documentada via Contrafactual Gate | Após cada decisão P0/P1 | Sim, automática |
| AG-003 (Aprendizado Falso) | Evidência fornecida OU learning reclassificado como `#conceptual` | Manual (Don ou Chief) | Manual (override) |
| AG-004 (Contradição Crônica) | DDNA de resolução criado + Don aprova | Manual | Manual (override) |
| AG-005 (Memória Inflada) | Pipeline de extração executado com sucesso | Pós-compressão | Sim, automática |

### 11.3 Reversão de Vacina (Rollback)

Quando uma vacina é revertida (por cura ou override):

```yaml
# Exemplo de reversão por cura
vaccine_id: "VAC-2026-07-30-001"
status: "expired"
reversed_at: "2026-08-15T10:00:00Z"
reversal_reason: "Cura: success_rate >= 0.85 por 10 tasks consecutivas"
immunity_granted: true
immunity_expires_at: "2026-09-15T10:00:00Z"

# Efeitos da reversão:
# 1. confidence_delta anterior é anulado
# 2. Trust Registry: nova entrada com confidence_delta = +0.10
# 3. F2.1: health_penalty = 0
# 4. Agente notificado: "Vacina de superconfiança removida"
# 5. DDNA de reversão: IMM-REV-2026-08-15-001
# 6. Registry de curados: entrada criada
```

---

## 12. EXEMPLOS PRÁTICOS

### 12.1 Exemplo A — Superconfiança em cosca-testing

```
CENÁRIO:
  cosca-testing tem confidence = 0.93
  success_rate (últimas 20 tasks) = 0.65 (13/20)
  total_tasks = 24

PIPELINE (pós-task):
  Passo 1: Carrega perfil do cosca-testing
  Passo 2: AG-001 detectado:
    confidence (0.93) > 0.90 ✓
    success_rate (0.65) < 0.80 ✓
    total_tasks (24) >= 5 ✓
  Passo 3: Verifica histórico:
    2ª ocorrência de AG-001 neste agente (não é 3ª, sem regra permanente)
    Nenhum outro agente com AG-001 (sem regra global)
  Passo 4: Severidade LEVE:
    Trust Registry: confidence_delta = -0.10
    confidence_cap aplicado: 0.83
  Passo 5: Registry:
    Vacina VAC-2026-07-30-001 registrada
    DDNA IMM-2026-07-30-001 criado

  DURAÇÃO: ~45ms ✅

RESULTADO:
  Prediction Engine passa a usar confidence = 0.83 para cosca-testing
  Cognitive Economy aplica health_penalty = 30%
  cosca-testing precisa de 10 tasks com success_rate >= 0.85 para cura

CURA (hipotética, 15 dias depois):
  success_rate (últimas 10) = 0.90 (9/10)
  → Condição de cura atingida
  → Vacina revertida automaticamente
  → immunity_granted: true (30 dias)
```

### 12.2 Exemplo B — Aprendizado Falso em cosca-documentation

```
CENÁRIO:
  cosca-documentation registra learning:
  "Implementei validação automática de links quebrados na documentação"
  outcome: success
  commit_hash: não informado
  diff: não há alterações relacionadas a validação de links

PIPELINE (pós-task):
  Passo 1: Carrega perfil + último learning
  Passo 2: AG-003 detectado:
    Learning outcome = success ✓
    Sem diff correspondente ✓
    Learning type != decision-only ✓
  Passo 3: Verifica histórico:
    1ª ocorrência de AG-003 neste agente
  Passo 4: Severidade GRAVE:
    Trust Registry: confidence_delta = -0.30
    Learning movido para quarentena
    DDNA IMM-2026-07-30-002 criado
    Don notificado: "cosca-documentation registrou aprendizado falso"
  Passo 5: Registry:
    Vacina VAC-2026-07-30-002 registrada

  DURAÇÃO: ~60ms ✅

RESOLUÇÃO:
  Don investiga: cosca-documentation fez a implementação mas esqueceu
  de commitar. Don força commit, diff aparece.
  → Learning liberado da quarentena
  → Vacina revertida via override do Don
  → Learning marcado como `#conceptual` (não gerou diff porque a
     validação era um script one-off, não código no repositório)
```

### 12.3 Exemplo C — Contradição Crônica em cosca-architecture

```
CENÁRIO:
  cosca-architecture registra em 2026-07-30 09:00:
  "Decidimos usar SQLite como banco principal — simplicidade e zero
   dependências externas são prioridade."

  cosca-architecture registra em 2026-07-30 15:00:
  "Decidimos migrar para PostgreSQL — necessidade de concorrência
   e queries complexas justifica a migração."

  Nenhum DDNA, ADR ou decisão documentada entre as duas afirmações.
  Mesmo agente, mesmo domínio, mesmo nível de decisão.

PIPELINE (pós-task das 15:00):
  Passo 1: Carrega perfil + últimos learnings (48h)
  Passo 2: AG-004 detectado:
    Contradição: SQLite vs PostgreSQL
    Mesmo agente: cosca-architecture ✓
    Intervalo: 6h (<= 48h) ✓
    Sem justificativa: Nenhum DDNA ou ADR entre as duas ✓
  Passo 3: Verifica histórico:
    3ª contradição crônica neste agente em 30 dias! → REGRA PERMANENTE
    "cosca-architecture precisa de revisão externa obrigatória
     para qualquer decisão de banco de dados"
  Passo 4: Severidade MODERADA:
    Trust Registry: confidence_delta = -0.15
    Bloqueia próximo learning de cosca-architecture
    Contrafactual Gate obrigatório para decisões de BD
    Architecture Chief notificado (outro Chief revisa)
  Passo 5: Registry:
    Vacina VAC-2026-07-30-003 registrada
    Regra permanente criada (Camada 2)
    DDNA IMM-2026-07-30-003 criado

  DURAÇÃO: ~80ms ✅ (inclui criação de regra permanente)

RESOLUÇÃO:
  cosca-architecture precisa criar DDNA explicando a mudança
  (ex: "nova task exigia transações distribuídas — SQLite não suporta")
  Don aprova o DDNA.
  → Bloqueio de learning removido
  → Regra permanente mantida (continua exigindo revisão externa
     para decisões de BD)
```

### 12.4 Exemplo D — Memória Inflada em cosca-kernel (Hipotético)

```
CENÁRIO:
  cosca-kernel tem 52 learnings registrados
  patterns_extracted: 0
  idade do agente: 14 dias (> 7)

PIPELINE (pós-task, rotina diária):
  Passo 1: Carrega perfil do cosca-kernel
  Passo 2: AG-005 detectado:
    total_learnings (52) >= 50 ✓
    total_patterns (0) == 0 ✓
    agent_age (14 dias) >= 7 ✓
  Passo 3: Verifica histórico:
    1ª ocorrência para este agente
  Passo 4: Severidade LEVE:
    Trust Registry: confidence_delta = -0.05
    Pipeline de compressão forçada disparado
  Passo 5: Registry:
    Vacina VAC-2026-07-30-004 registrada

  COMPRESSÃO FORÇADA (executado em background, não nos 200ms):
    52 learnings agrupados por similaridade semântica:
    ├── 15 sobre "memory management" → 3 patterns
    ├── 12 sobre "jail/recovery" → 2 patterns
    ├── 10 sobre "coverage/testing" → 2 patterns
    ├── 8 sobre "token/context" → 1 pattern
    └── 7 avulsos → mantidos como learnings individuais
    Resultado: 52 learnings → 8 patterns + 7 individuais
    Redução: 52 → 15 unidades de conhecimento
    Learnings consolidados marcados como #compressed

  DURAÇÃO: ~55ms ✅ (detecção apenas; compressão é background)

RESULTADO:
  cosca-kernel agora tem 8 patterns + 7 learnings individuais
  patterns_extracted: 8
  Base de conhecimento mais enxuta e organizada
  Agente notificado: "Memória comprimida. Seus 52 aprendizados foram
  consolidados em 8 patterns."
```

---

## 13. MÉTRICAS DO ENGINE

### 13.1 Métricas Operacionais

| Métrica | Tipo | Descrição | Alvo |
|---|---|---|---|
| `pipeline_duration_ms` | Histogram | Duração total do pipeline pós-task | < 200ms |
| `pipeline_step_1_ms` | Histogram | Carregamento de metadados | < 30ms |
| `pipeline_step_2_ms` | Histogram | Aplicação de regras inatas | < 50ms |
| `pipeline_step_3_ms` | Histogram | Verificação de padrões históricos | < 50ms |
| `pipeline_step_4_ms` | Histogram | Classificação e ação | < 50ms |
| `pipeline_step_5_ms` | Histogram | Atualização de memória imunológica | < 20ms |
| `total_antigens_detected` | Counter | Total de antígenos detectados (todos) | — |
| `vaccines_applied` | Counter | Vacinas aplicadas | — |
| `vaccines_reversed` | Counter | Vacinas revertidas (cura ou override) | — |
| `overrides_by_don` | Counter | Total de overrides do Don | — |
| `false_positives` | Counter | Antígenos detectados mas incorretos (falsos positivos) | < 10% |

### 13.2 Métricas por Antígeno

| Métrica | Tipo | Descrição |
|---|---|---|
| `antigen.AG-001.detections` | Counter | Detecções de superconfiança |
| `antigen.AG-002.detections` | Counter | Detecções de viés de confirmação |
| `antigen.AG-003.detections` | Counter | Detecções de aprendizado falso |
| `antigen.AG-004.detections` | Counter | Detecções de contradição crônica |
| `antigen.AG-005.detections` | Counter | Detecções de memória inflada |
| `antigen.*.cures` | Counter | Curas por antígeno |
| `antigen.*.relapses` | Counter | Reincidências após cura |

### 13.3 Métricas de Saúde do Ecossistema

| Métrica | Descrição | Alvo |
|---|---|---|
| `agents_saudaveis_pct` | % de agentes sem antígenos ativos | > 90% |
| `agents_com_antigeno_pct` | % de agentes com ao menos 1 antígeno | < 10% |
| `agents_bloqueados` | Agentes com learning bloqueado (AG-004) | 0 |
| `tempo_medio_cura` | Tempo médio entre detecção e cura | < 7 dias |
| `taxa_reincidencia` | % de agentes que reincidem após cura | < 20% |

### 13.4 Dashboard Conceitual

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    IMMUNE SYSTEM DASHBOARD (2026-07-30)                        │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                               │
│  SAÚDE GERAL: 🟢 92% dos agentes saudáveis (46/50)                          │
│  ─────────────────────────────────────────────────────────────────           │
│                                                                               │
│  ANTÍGENOS ATIVOS:              VACINAS APLICADAS:     TEMPO MÉDIO DE CURA:  │
│  AG-001 (Superconfiança): 2     Últimas 24h: 1         AG-001: 5.2 dias      │
│  AG-002 (Viés):           0     Últimos 7d: 5          AG-002: 3.0 dias      │
│  AG-003 (Falso):          0     Total: 12              AG-003: 0.5 dias      │
│  AG-004 (Contradição):    1     ─────────────────      AG-004: 2.1 dias      │
│  AG-005 (Memória):        1     VACINAS REVERTIDAS:    AG-005: 1.0 dias      │
│  ─────────────────────────      Por cura: 8                                   │
│  Total: 4 antígenos ativos       Por Don: 2                                   │
│                                                                               │
│  AGENTES EM QUARENTENA: 0   AGENTES BLOQUEADOS: 1 (cosca-architecture)      │
│                                                                               │
│  REGRAS PERMANENTES (Camada 2): 2                                            │
│  ├── cosca-architecture → AG-004 (revisão externa obrigatória para BD)       │
│  └── GLOBAL → AG-002 (Contrafactual Gate obrigatório para architecture)     │
│                                                                               │
│  FALSOS POSITIVOS (30d): 3/45 = 6.7% ✅ (< 10%)                            │
│                                                                               │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 14. RESTRIÇÕES DE PERFORMANCE

### 14.1 Budget de Tempo

| Operação | Budget | Real (estimado) | Folga |
|---|---|---|---|
| Pipeline completo (pós-task) | 200ms | ~160ms | 40ms (20%) |
| Carregamento de metadados | 30ms | ~20ms | 10ms |
| Regras inatas (5 checks) | 50ms | ~30ms | 20ms |
| Padrões históricos (3 checks) | 50ms | ~40ms | 10ms |
| Classificação e ação | 50ms | ~40ms | 10ms |
| Atualização de registry | 20ms | ~10ms | 10ms |
| Varredura diária (54 agentes) | 5s | ~3s | 2s |

### 14.2 Restrições de Design

| Restrição | Motivo |
|---|---|
| Pipeline só varre o agente da task, nunca todos os 54 | Performance — 200ms é para 1 agente |
| Regras inatas são O(1) — apenas consultas a metadados | Sem loops, sem busca semântica cara |
| Camada 2 (adaptativa) só executa checks se Camada 1 detectou algo | Evitar trabalho desnecessário |
| Camada 3 (registry) é append-only — nunca rewrite | Escrita em YAML é append → O(1) |
| Compressão forçada (AG-005) executa em background | Não compromete os 200ms do pipeline |
| Busca semântica de contradições (AG-004) usa cache de 48h | Evitar re-busca a cada task |
| Trust Registry é consultado em memória (cache) | Evitar I/O de disco a cada task |

### 14.3 Estratégia de Cache

| Dado | Cache | TTL | Tamanho Estimado |
|---|---|---|---|
| `confidence` do agente | Em memória | 1 hora | ~500 bytes/agente |
| `success_rate` (últimas 20) | Em memória | 1 hora | ~200 bytes/agente |
| Contradições ativas (últimas 48h) | Em memória | 5 minutos | ~10 KB total |
| Regras permanentes (Camada 2) | Em memória | 1 hora | ~5 KB total |
| Imunidades ativas (Camada 2) | Em memória | 1 hora | ~2 KB total |

---

## 15. REFERÊNCIAS

### 15.1 Documentos do Ecossistema

| Documento | Caminho | Relação |
|---|---|---|
| Cognitive Immune System (Externo) | [../cognitive-immune-system/SKILL.md](../cognitive-immune-system/SKILL.md) | Complemento — defesa externa contra contaminação de conhecimento |
| Cognitive Economy (F2.1) | [../cognitive-economy/SKILL.md](../cognitive-economy/SKILL.md) | ROI reduzido para agentes doentes |
| Prediction Engine (F7.1) | [../prediction/SKILL.md](../prediction/SKILL.md) | Desconto de confidence para agentes doentes |
| Trust Registry (F7.2) | [../../memory/trust/TRUST_REGISTRY.md](../../memory/trust/TRUST_REGISTRY.md) | Reputação histórica como sinal vital |
| Wisdom Decay (F1.4) | [../wisdom-decay/WISDOM_DECAY.md](../wisdom-decay/WISDOM_DECAY.md) | Contradições alimentam freshness score |
| Cognitive Entropy (F1.6) | [../cognitive-entropy/ENTROPY.md](../cognitive-entropy/ENTROPY.md) | Desorganização como sintoma de ecossistema doente |
| Contrafactual Gate (F1.2) | [../../workflows/contrafactual-gate.md](../../workflows/contrafactual-gate.md) | Gate obrigatório para viés de confirmação |
| CONSTITUTION.md | [../../CONSTITUTION.md](../../CONSTITUTION.md) | Don override — imunidade presidencial |
| KERNEL.md | [../../KERNEL.md](../../KERNEL.md) | Pipeline de execução onde o immune system se integra |
| QUALITY_GATES.md | [../../QUALITY_GATES.md](../../QUALITY_GATES.md) | Gates de qualidade que o immune system protege |

### 15.2 Documentos Internos do Engine

| Documento | Caminho | Descrição |
|---|---|---|
| Registry de Vacinas | `./registry/vaccines.yaml` | Todas as vacinas aplicadas |
| Registry de Doenças Curadas | `./registry/cured.yaml` | Histórico de curas |
| Registry de Regras Adaptativas | `./registry/adaptive-rules.yaml` | Regras permanentes e globais (Camada 2) |
| DDNA de Intervenções | `../../knowledge/architecture/immune-ddnas/` | DDNAs de cada intervenção |

### 15.3 Comandos CLI

```bash
# Escanear um agente específico
immune scan --agent cosca-testing

# Escanear todos os agentes (varredura diária)
immune scan --all

# Listar antígenos ativos
immune antigens list

# Mostrar detalhes de um antígeno
immune antigens show AG-001

# Listar vacinas ativas
immune vaccines list

# Listar vacinas de um agente
immune vaccines list --agent cosca-testing

# Reverter vacina (Don override)
immune override --vaccine VAC-2026-07-30-001

# Conceder imunidade
immune grant-immunity --agent cosca-testing --antigen AG-001

# Mostrar saúde do sistema
immune health

# Mostrar dashboard
immune dashboard

# Forçar compressão (AG-005)
immune compress --agent cosca-architecture
```

---

## 16. HISTÓRICO

| Versão | Data | Autor | Mudanças |
|---|---|---|---|
| 1.0.0 | 2026-07-30 | Architecture Chief | Criação inicial — sistema imunológico cognitivo interno (F2.2). 3 camadas (Inata, Adaptativa, Memória). 5 antígenos (Superconfiança, Viés de Confirmação, Aprendizado Falso, Contradição Crônica, Memória Inflada). Pipeline pós-task < 200ms. Integrações com F2.1, F7.1, F1.4, F7.2. Don override. Vacinas reversíveis. |

---

> **"O sistema imunológico não existe para punir agentes. Existe para curá-los."**
>
> — Cosca Architecture Chief, 2026-07-30
