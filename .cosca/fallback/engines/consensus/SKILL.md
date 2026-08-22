# MULTI PROVIDER CONSENSUS ENGINE ★ F8.4 — Motor de Consenso Multi-Provedor

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-consensus`
> **Conceito**: ★ ESTRELA DA FASE 8 — Multi Provider Consensus
> **Referências**: next-evolution-phases.md §F8.4 | TRUST_REGISTRY.md | DECISION_DNA.md | cognitive-economy/SKILL.md | capability-market/SKILL.md | prediction/SKILL.md
> **Dependências**: F8.1 (Capability Market) | F7.1 (Prediction Engine) | F2.1 (Cognitive Economy) | F7.2 (Trust Registry) | F1.1 (DDNA)
> **CMI Impact**: Julgamento +8, Confiabilidade +7, Planejamento +4

---

## Índice

1. [O que é o Multi Provider Consensus Engine](#1-o-que-é-o-multi-provider-consensus-engine)
2. [Filosofia](#2-filosofia)
3. [Gatilhos de Ativação](#3-gatilhos-de-ativação)
4. [Pipeline de Consenso](#4-pipeline-de-consenso)
5. [Fórmula de Consenso](#5-fórmula-de-consenso)
6. [Seleção de Agentes](#6-seleção-de-agentes)
7. [Custo e ROI do Consenso](#7-custo-e-roi-do-consenso)
8. [Integração com F2.1 — Cognitive Economy](#8-integração-com-f21--cognitive-economy)
9. [Integração com F8.1 — Capability Market](#9-integração-com-f81--capability-market)
10. [Integração com F7.1 — Prediction Engine](#10-integração-com-f71--prediction-engine)
11. [Feedback no Trust Registry](#11-feedback-no-trust-registry)
12. [Don Override](#12-don-override)
13. [Modos de Operação](#13-modos-de-operação)
14. [Exemplo Completo](#14-exemplo-completo)
15. [Métricas do Engine](#15-métricas-do-engine)
16. [Regras de Operação](#16-regras-de-operação)
17. [Tratamento de Erros](#17-tratamento-de-erros)
18. [Relacionados](#18-relacionados)
19. [Histórico](#19-histórico)

---

## 1. O que é o Multi Provider Consensus Engine

### Definição

O **Multi Provider Consensus Engine** é o motor que **consulta 2-3 agentes/providers independentes** para decisões de alto impacto (P0/P1) e **sintetiza os resultados** em um veredito único ponderado.

Quando o custo da decisão justifica, usar múltiplos provedores **aumenta a confiabilidade** e **reduz o risco de viés** de um único agente.

### Analogia: Conselho de Especialistas

```
Conselho de Especialistas (Humano)   →   Consensus Engine (Cosca)
──────────────────────────────────────────────────────────────
CEO consulta 3 diretores             →   Kernel consulta 3 agentes
Cada diretor analisa independente    →   Cada agente executa isolado
Votação ponderada por expertise      →   Consenso ponderado por confiança
Maioria simples decide               →   agreement_ratio define ação
Dissidente registra voto em separado →   Agente divergente registra rationale
CEO pode ignorar o conselho          →   Don override
```

### Propósito

| Dimensão | Descrição |
|----------|-----------|
| **Confiabilidade** | Decisões críticas são validadas por múltiplas perspectivas antes de executar |
| **Redução de viés** | Um único agente pode ter viés de confirmação, recency bias, ou lacuna de conhecimento. Múltiplos agentes mitigam |
| **Detecção de anomalias** | Se 2/3 agentes concordam e 1 diverge, a divergência é um sinal de que algo merece atenção |
| **Trust Registry calibrado** | O histórico de "quem acertou no consenso" calibra a confiança de cada agente |
| **Rastreabilidade** | Decisões multi-agente geram DDNA mais rico com o rationale de cada perspectiva |

> **"Duas cabeças pensam melhor que uma. Três cabeças, melhor ainda — desde que o custo justifique."**
> — Cosca Architecture Chief, 2026-07-30

---

## 2. Filosofia

### Princípios

```
┌─────────────────────────────────────────────────────────────────────┐
│                                                                      │
│   ★ F8.4 — Multi Provider Consensus                                  │
│                                                                      │
│   1. INDEPENDÊNCIA: Cada agente executa SEM conhecer os outros       │
│      → Sem contaminação, sem viés de grupo, sem ancoragem           │
│                                                                      │
│   2. CUSTO JUSTIFICADO: Consenso só ativa quando ROI > 0            │
│      → 3× mais caro que decisão única → precisa compensar           │
│                                                                      │
│   3. PONDERAÇÃO POR CONFIANÇA: Nem todo voto pesa igual             │
│      → Agente com mais confiança tem mais peso no resultado         │
│                                                                      │
│   4. TRANSPARÊNCIA: Divergência não é erro — é dado                 │
│      → Cada divergência vira aprendizado no Trust Registry          │
│                                                                      │
│   5. DON SEMPRE PODE: Override humano ignora consenso               │
│      → Don é a autoridade final, registrado em DDNA                 │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### O que o Consensus Engine NÃO é

- **Não é um votador binário** — não é "maioria simples vence". A ponderação por confiança e o agreement_ratio criam um espectro de ação
- **Não substitui o Kernel** — o Kernel continua sendo a autoridade final; o consenso é uma recomendação
- **Não é um ensemble de LLMs** — não combina outputs no nível de token; combina DECISÕES no nível de агент
- **Não é um orquestrador de workflow** — não decide a ordem de execução; decide SE e COMO executar uma decisão específica

---

## 3. Gatilhos de Ativação

### Regras de Ativação

O Consensus Engine é ativado quando UMA OU MAIS das seguintes condições é satisfeita:

```
┌──────────────────────────────────────────────────────────────────┐
│                     GATILHOS DE ATIVAÇÃO                          │
│                                                                  │
│  Decisão P0 ──────────────────────────────────────→ SEMPRE ATIVA │
│                                                                  │
│  Decisão P1 + custo_estimado > $0.01 ────────────→ ATIVA         │
│                                                                  │
│  Decisão com risco_score > 30% (🔴 ALTO) ───────→ ATIVA          │
│                                                                  │
│  Don explicitamente força ───────────────────────→ ATIVA         │
│                                                                  │
│  Decisão P1 + custo ≤ $0.01 + risco ≤ 30% ──────→ NÃO ATIVA     │
│                                                                  │
│  Decisão P2/P3 ──────────────────────────────────→ NÃO ATIVA     │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### Matriz de Decisão

| Prioridade | Custo Estimado | Risco | Consenso? | Rationale |
|:----------:|:--------------:|:-----:|:---------:|-----------|
| **P0** | Qualquer | Qualquer | ✅ **Sempre** | Decisão crítica: múltiplas perspectivas são obrigatórias |
| **P1** | > $0.01 | Qualquer | ✅ Sim | Custo alto justifica verificação extra |
| **P1** | ≤ $0.01 | > 30% | ✅ Sim | Risco alto justifica mesmo com custo baixo |
| **P1** | ≤ $0.01 | ≤ 30% | ❌ Não | Custo baixo + risco baixo = decisão única |
| **P2** | Qualquer | Qualquer | ❌ Não | Prioridade baixa não justifica custo 3× |
| **P3** | Qualquer | Qualquer | ❌ Não | Prioridade mínima |

### Cálculo do Risco

O `risco_score` é obtido do Prediction Engine (F7.1):

```
risco_score = (1 - P(success)) × (1 + (1 - confianca_predicao) × 0.5)

Gatilho: risco_score > 0.30  (equivalente a P(success) < 70%)
```

### Don Override Forçado

O Don pode forçar a ativação do consenso **independentemente das regras**:

```yaml
don_consensus_override:
  task_id: "decision-arch-module-x"
  force_consensus: true
  rationale: "Don quer validação multi-agente para esta decisão"
  minimum_agents: 3
  override_type: "explicit"   # explicit | emergency
```

---

## 4. Pipeline de Consenso

### Visão Geral

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        PIPELINE DE CONSENSO (F8.4)                           │
│                                                                              │
│  1. DECISÃO CHEGA                                                            │
│     ├── "Qual arquitetura para o novo módulo?"                               │
│     ├── "Qual a melhor estratégia de testes?"                                │
│     └── "Qual provedor de IA escolher para este caso de uso?"               │
│          │                                                                   │
│          ▼                                                                   │
│  2. CONSENSUS ENGINE VERIFICA GATILHOS                                       │
│     ├── É P0? → sim → ativa                                                 │
│     ├── É P1 + custo > $0.01? → sim → ativa                                │
│     ├── risco > 30%? → sim → ativa                                          │
│     └── Don override? → sim → ativa                                         │
│          │                                                                   │
│          ▼                                                                   │
│  3. ENGINE SELECIONA 3 AGENTES                                               │
│     ├── Primary:   cosca-architecture (melhor no domínio)                    │
│     ├── Secondary: cosca-backend (visão complementar)                        │
│     └── Tertiary:  cosca-critic (visão crítica/adversarial)                 │
│          │                                                                   │
│          ▼                                                                   │
│  4. CADA AGENTE EXECUTA A MESMA TASK DE FORMA INDEPENDENTE                   │
│     ├── Agente A → produz recomendação (sem saber de B e C)                  │
│     ├── Agente B → produz recomendação (sem saber de A e C)                  │
│     └── Agente C → produz recomendação (sem saber de A e B)                  │
│          │                                                                   │
│          ▼                                                                   │
│  5. ENGINE COMPARA RESULTADOS                                                │
│     ├── Se consenso total (3/3) → "Consenso Forte. Executa."                 │
│     ├── Se maioria (2/3) → "Consenso Médio. Executa com ressalvas."          │
│     └── Se divergência (1/3 ou 0/3) → "Sem consenso. Gate + Don."           │
│          │                                                                   │
│          ▼                                                                   │
│  6. ENGINE CALCULA CONSENSUS STRENGTH                                        │
│     ├── Formula: agreement_ratio × confidence_weighted_avg                   │
│     ├── strength > 0.8 → consenso forte                                     │
│     ├── strength 0.5-0.8 → consenso médio                                   │
│     └── strength < 0.5 → sem consenso                                       │
│          │                                                                   │
│          ▼                                                                   │
│  7. ENGINE CALCULA ROI DO CONSENSO                                           │
│     ├── ROI_consenso = (erro_evitado - custo_extra) / custo_extra            │
│     ├── Se ROI_consenso < 0 → alerta: "Consenso não valeu o custo extra"    │
│     └── Registra no Trust Registry + Cognitive Economy                       │
│          │                                                                   │
│          ▼                                                                   │
│  8. ENGINE PERGUNTA: "QUAL AGENTE ACERTOU?" (pós-task)                       │
│     ├── Compara recomendação de cada agente com resultado real               │
│     ├── Atualiza Trust Registry: confidence_delta por agente                 │
│     └── Aprendizado: "Agente X consistentemente divergente em domínio Y"     │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Passo a Passo Detalhado

#### Passo 1: Decisão Chega

O Kernel recebe uma decisão que requer análise. Exemplos:

- "Qual arquitetura para o novo módulo de cache distribuído?"
- "Qual a melhor estratégia de testes para o runtime WASM?"
- "Devemos migrar o provider X para o padrão Y?"
- "Qual abordagem de segurança para o novo endpoint?"

#### Passo 2: Verificação de Gatilhos

O Consensus Engine consulta:
1. Prioridade da decisão (P0/P1/P2/P3) — do Kernel
2. Custo estimado da task — do Prediction Engine (F7.1)
3. Risco score — do Prediction Engine (F7.1)
4. Don override flag — da requisição

**Output**: `{ activate: bool, reason: string, minimum_agents: int }`

#### Passo 3: Seleção de Agentes

O Engine seleciona **3 agentes** com base em:
1. **Domain expertise** — melhor agente para o domínio da decisão (do Capability Market F8.1)
2. **Complementaridade** — perspectivas diferentes (não 3 especialistas iguais)
3. **Disponibilidade** — agentes disponíveis no momento
4. **Custo** — dentro do orçamento da decisão

Ver detalhes na [Seção 6 — Seleção de Agentes](#6-seleção-de-agentes).

#### Passo 4: Execução Independente

**REGRA FUNDAMENTAL**: Cada agente executa a MESMA task de forma **completamente independente**.

```
Isolamento garante:
├── Sem contaminação de opinião
├── Sem viés de ancoragem (não sabem o que os outros disseram)
├── Sem efeito de grupo (não há pressão para concordar)
└── Sem economia de esforço ("o outro já disse, então repito")
```

**Formato do output de cada agente**:

```yaml
agent_response:
  agent: "cosca-architecture"
  task_id: "consensus-arch-module-x"
  timestamp: "2026-07-30T14:30:00Z"

  recommendation:
    choice: "opcao_B"                    # A escolha do agente
    confidence: 0.85                      # Confiança do agente na própria recomendação
    rationale: "Descrição do porquê..."  # Raciocínio completo

  alternatives_considered:
    - name: "opcao_A"
      pros: ["pro1", "pro2"]
      cons: ["con1"]
      discarded_reason: null
    - name: "opcao_B"
      pros: ["pro1", "pro2", "pro3"]
      cons: ["con1"]
      discarded_reason: null             # ← A escolhida
    - name: "opcao_C"
      pros: ["pro1"]
      cons: ["con1", "con2", "con3"]
      discarded_reason: "Risco alto demais para o benefício"

  risk_assessment:
    risk_level: "medium"
    risk_score: 0.25
    main_concern: "Complexidade de integração com sistema legado"

  estimated_effort: "3 semanas"
  estimated_cost: "$0.045"
```

#### Passo 5: Comparação de Resultados

O Engine coleta os 3 outputs e executa a análise de convergência:

```yaml
comparison:
  task_id: "consensus-arch-module-x"
  agents:
    - agent: "cosca-architecture"
      choice: "opcao_B"
      confidence: 0.85
      risk_score: 0.25

    - agent: "cosca-backend"
      choice: "opcao_B"
      confidence: 0.78
      risk_score: 0.30

    - agent: "cosca-critic"
      choice: "opcao_A"
      confidence: 0.65
      risk_score: 0.45

  convergence:
    total_agents: 3
    convergent: 2      # architecture + backend escolheram B
    divergent: 1       # critic escolheu A
    agreement_ratio: 0.667  # 2/3
```

#### Passo 6: Cálculo do Consensus Strength

Ver fórmula completa na [Seção 5](#5-fórmula-de-consenso).

#### Passo 7: Ação Baseada no Resultado

| consensus_strength | Classificação | Ação |
|:------------------:|:------------:|------|
| **> 0.8** | 🟢 Consenso Forte | Executa decisão. Registrar como consenso forte. |
| **0.5 — 0.8** | 🟡 Consenso Médio | Executa com ressalvas. Incluir ponto de atenção da minoria. |
| **< 0.5** | 🔴 Sem Consenso | Aciona Gate. Notifica Don. Não executa automaticamente. |

#### Passo 8: Feedback Pós-Task (Calibragem)

Após a execução da decisão, o Engine **compara quem recomendou o quê** com **o que realmente aconteceu**:

```yaml
post_task_feedback:
  task_id: "consensus-arch-module-x"
  outcome: "success"

  agent_accuracy:
    - agent: "cosca-architecture"
      recommended: "opcao_B"
      confidence: 0.85
      outcome_match: true     # Recomendou a opção que deu certo
      accuracy: 1.0
      confidence_delta: +0.03

    - agent: "cosca-backend"
      recommended: "opcao_B"
      confidence: 0.78
      outcome_match: true
      accuracy: 1.0
      confidence_delta: +0.02

    - agent: "cosca-critic"
      recommended: "opcao_A"
      confidence: 0.65
      outcome_match: false    # Recomendou a opção errada
      accuracy: 0.0
      confidence_delta: -0.05  # Penalizado

  consensus_accuracy:
    consensus_verdict: "medium"   # 2/3 concordaram
    consensus_was_correct: true   # A maioria acertou
    confidence_delta_consensus: +0.02  # Consenso validado
```

---

## 5. Fórmula de Consenso

### Definição

O **consensus_strength** é a métrica que determina o nível de concordância ponderada entre os agentes:

```
consensus_strength = agreement_ratio × confidence_weighted_avg
```

### Componentes

#### agreement_ratio — Taxa de Convergência

```
agreement_ratio = num_agent_convergent / total_agents

Onde:
  num_agent_convergent = Número de agentes que escolheram a OPÇÃO MAIS VOTADA
  total_agents         = Número total de agentes consultados (geralmente 3)

Interpretação:
  1.0   → Consenso unânime (3/3)
  0.667 → Maioria (2/3)
  0.333 → Minoria (1/3)
  0.0   → Nenhum acordo (0/3 — todos escolheram opções diferentes)
```

#### confidence_weighted_avg — Média Ponderada por Confiança

```
confidence_weighted_avg = Σ(agreement × agent_confidence) / Σ(agent_confidence)

Onde:
  agreement           = 1 se o agente está no grupo convergente, 0 se divergente
  agent_confidence    = Confiança do agente na própria recomendação (0.0-1.0)

EXEMPLO — 2 agentes concordam, 1 discorda:

  Agente A: confidence = 0.85, agreement = 1 (convergente)
  Agente B: confidence = 0.78, agreement = 1 (convergente)
  Agente C: confidence = 0.65, agreement = 0 (divergente)

  confidence_weighted_avg = (1 × 0.85 + 1 × 0.78 + 0 × 0.65) / (0.85 + 0.78 + 0.65)
                         = (0.85 + 0.78 + 0) / 2.28
                         = 1.63 / 2.28
                         = 0.715
```

#### consensus_strength — Score Final

```
consensus_strength = agreement_ratio × confidence_weighted_avg

EXEMPLO (mesmo cenário):
  agreement_ratio           = 2/3 = 0.667
  confidence_weighted_avg   = 0.715

  consensus_strength = 0.667 × 0.715 = 0.477

  → Consenso FRACO (< 0.5) → Gate + Don
```

### Thresholds

```
consensus_strength > 0.8  → 🟢 CONSENSO FORTE
   ├── agreement_ratio ≥ 0.667 E confidence_weighted_avg ≥ 0.80
   ├── EXEMPLO: 3/3 convergentes, confiança média 0.85 → 1.0 × 0.85 = 0.85
   └── Ação: Executa decisão automaticamente

consensus_strength 0.5—0.8 → 🟡 CONSENSO MÉDIO
   ├── agreement_ratio ≥ 0.667 MAS confidence_weighted_avg < 0.80
   ├── OU agreement_ratio = 1.0 MAS confiança baixa
   ├── EXEMPLO: 2/3 convergentes, confiança média 0.75 → 0.667 × 0.75 = 0.50
   └── Ação: Executa com ressalvas. Incluir rationale da minoria.

consensus_strength < 0.5 → 🔴 SEM CONSENSO
   ├── agreement_ratio = 0.333 (minoria) OU confidence_weighted_avg muito baixo
   ├── EXEMPLO: 2/3 convergentes mas confiança baixa → 0.667 × 0.60 = 0.40
   ├── OU 1/3 convergente → 0.333 × qualquer = < 0.5
   └── Ação: Gate + Don. Não executa automaticamente.
```

### Mapa de Decisão

```
                    agreement_ratio
              ┌──────────┬──────────┬──────────┐
              │  0.333   │  0.667   │  1.000   │
              │  (1/3)   │  (2/3)   │  (3/3)   │
───┬──────────┼──────────┼──────────┼──────────┤
   │  0.90    │  0.300   │  0.600   │  0.900   │
   │          │  🔴 Gate │  🟡 Médio│  🟢 Forte│
   ├──────────┼──────────┼──────────┼──────────┤
 C │  0.80    │  0.267   │  0.534   │  0.800   │
 o │          │  🔴 Gate │  🟡 Médio│  🟢 Forte│
 n ├──────────┼──────────┼──────────┼──────────┤
 f │  0.70    │  0.233   │  0.467   │  0.700   │
 i │          │  🔴 Gate │  🔴 Gate │  🟡 Médio│
 a ├──────────┼──────────┼──────────┼──────────┤
 n │  0.60    │  0.200   │  0.400   │  0.600   │
 ç │          │  🔴 Gate │  🔴 Gate │  🟡 Médio│
 a ├──────────┼──────────┼──────────┼──────────┤
   │  0.50    │  0.167   │  0.333   │  0.500   │
   │          │  🔴 Gate │  🔴 Gate │  🟡 Médio│
───┴──────────┴──────────┴──────────┴──────────┘

Legenda:
  🟢 Forte → Executa sem ressalvas
  🟡 Médio → Executa com ressalvas
  🔴 Gate  → Gate + Don
```

### Exemplos de Cálculo

#### Exemplo A: Consenso Forte

```
Agente A: architecture (0.85) → opção B
Agente B: backend (0.78)      → opção B
Agente C: critic (0.65)       → opção B  ← todos convergentes

agreement_ratio           = 3/3 = 1.0
confidence_weighted_avg   = (1×0.85 + 1×0.78 + 1×0.65) / (0.85+0.78+0.65)
                          = 2.28 / 2.28 = 1.0
consensus_strength        = 1.0 × 1.0 = 1.0

→ 🟢 CONSENSO FORTE (strength = 1.0 > 0.8)
→ Executa a decisão
```

#### Exemplo B: Consenso Médio

```
Agente A: architecture (0.85) → opção B
Agente B: backend (0.78)      → opção B
Agente C: critic (0.65)       → opção A  ← divergente

agreement_ratio           = 2/3 = 0.667
confidence_weighted_avg   = (1×0.85 + 1×0.78 + 0×0.65) / (0.85+0.78+0.65)
                          = 1.63 / 2.28 = 0.715
consensus_strength        = 0.667 × 0.715 = 0.477

→ 🔴 SEM CONSENSO (strength = 0.477 < 0.5)
→ Gate + Don
```

#### Exemplo C: Consenso Médio (do enunciado)

```
Agente A: architecture (0.80) → "estratégia X"
Agente B: backend (0.75)      → "estratégia X"
Agente C: critic (0.65)       → "estratégia X"  ← todos convergentes

agreement_ratio           = 3/3 = 1.0
confidence_weighted_avg   = (1×0.80 + 1×0.75 + 1×0.65) / (0.80+0.75+0.65)
                          = 2.20 / 2.20 = 1.0
consensus_strength        = 1.0 × 1.0 = 1.0

→ 🟢 CONSENSO FORTE (strength = 1.0 > 0.8)
→ Executa (mesmo com confianças individuais médias, o consenso total é forte)
```

#### Exemplo D: Do enunciado da missão

```
Agente A: architecture (0.80) → "estratégia Y"
Agente B: backend (0.75)      → "estratégia Y"
Agente C: critic (0.65)       → "estratégia Z"  ← divergente

agreement_ratio           = 2/3 = 0.667
confidence_weighted_avg   = (1×0.80 + 1×0.75 + 0×0.65) / (0.80+0.75+0.65)
                          = 1.55 / 2.20 = 0.705
consensus_strength        = 0.667 × 0.705 = 0.470

→ 🔴 SEM CONSENSO (strength = 0.470 < 0.5)
→ Gate + Don

Nota: O enunciado original dizia strength = 0.73 para este cenário
com 2/3 convergentes, mas o cálculo correto com os valores dados
resulta em 0.470. A diferença se deve à ponderação por confiança.
```

---

## 6. Seleção de Agentes

### Critérios de Seleção

O Consensus Engine seleciona **3 agentes** com base em 4 critérios ponderados:

```
Score_SELECAO = domain_expertise × 0.40
              + complementaridade × 0.25
              + disponibilidade × 0.20
              + eficiencia_custo × 0.15
```

### Papéis dos 3 Agentes

| Papel | Descrição | Critério de Seleção | Exemplo |
|-------|-----------|---------------------|---------|
| **Primary** 🥇 | Melhor agente no domínio da decisão. É a referência técnica. | `domain_expertise` mais alto | cosca-architecture para decisões de arquitetura |
| **Secondary** 🥈 | Visão complementar. Domínio adjacente que enriquece a perspectiva. | `domain_expertise` no segundo melhor domínio relacionado | cosca-backend para viabilidade de implementação |
| **Tertiary** 🥉 | Visão crítica/adversarial. Questiona premissas. | Menor `confirmation_bias` ou perfil crítico | cosca-critic para análise adversária |

### Pool de Agentes por Domínio

O Engine mantém um mapeamento de domínios para agentes recomendados:

```yaml
domain_pool:
  architecture:
    primary:   cosca-architecture   # domain_strength: 0.93
    secondary: cosca-infrastructure  # domain_strength: 0.85 (visão de infra)
    tertiary:  cosca-critic          # Perfil crítico

  backend:
    primary:   cosca-backend        # domain_strength: 0.90
    secondary: cosca-database        # domain_strength: 0.82 (visão de dados)
    tertiary:  cosca-critic

  testing:
    primary:   cosca-testing        # domain_strength: 0.88
    secondary: cosca-qa             # domain_strength: 0.80 (visão de qualidade)
    tertiary:  cosca-critic

  security:
    primary:   cosca-security       # domain_strength: 0.88
    secondary: cosca-devops          # domain_strength: 0.72 (visão operacional)
    tertiary:  cosca-critic

  frontend:
    primary:   cosca-frontend       # domain_strength: 0.90
    secondary: cosca-uiux           # domain_strength: 0.85 (visão de UX)
    tertiary:  cosca-critic

  database:
    primary:   cosca-database       # domain_strength: 0.88
    secondary: cosca-backend        # domain_strength: 0.75 (visão de API)
    tertiary:  cosca-critic

  devops:
    primary:   cosca-devops         # domain_strength: 0.90
    secondary: cosca-infrastructure # domain_strength: 0.82 (visão de infra)
    tertiary:  cosca-critic

  cli:
    primary:   cosca-cli            # domain_strength: 0.92
    secondary: cosca-automation     # domain_strength: 0.80 (visão de automação)
    tertiary:  cosca-critic

  # Domínios que usam providers externos (LLM providers)
  ai-provider-selection:
    primary:   cosca-provider       # domain_strength: 0.90
    secondary: cosca-ai             # domain_strength: 0.85 (visão de ML)
    tertiary:  cosca-critic

  decision-strategy:
    primary:   cosca-architecture   # domain_strength: 0.93
    secondary: cosca-cto            # domain_strength: 0.90 (visão estratégica)
    tertiary:  cosca-critic
```

### Regras de Substituição

Se o agente primário do pool não estiver disponível:

```
1. Tentar agente com maior domain_strength no mesmo domínio
2. Se nenhum disponível, escalar para cosca-architecture (default primário)
3. Se cosca-architecture indisponível, escalar para Kernel
4. Registrar substituição no log de consenso
```

---

## 7. Custo e ROI do Consenso

### Custo Base

O consenso custa **3× mais que uma decisão única**:

```
custo_consenso = custo_decisao_unica × num_agentes

Onde:
  custo_decisao_unica = custo médio de uma task de decisão simples
  num_agentes         = 3 (padrão)

EXEMPLO:
  custo_decisao_unica = $0.030  (decisão architecture simples)
  custo_consenso      = $0.030 × 3 = $0.090
```

### Componentes do Custo do Consenso

```
custo_consenso_total = custo_agents + custo_sintese + custo_coordenacao

Onde:
  custo_agents     = Σ(custo_task_individual) para cada um dos 3 agentes
  custo_sintese    = Custo de comparar resultados e calcular consenso (~$0.005)
  custo_coordenacao = Custo de orquestração pelo Kernel (~$0.003)
```

### Quando o Consenso Vale a Pena

O consenso **só deve ser ativado quando o ROI justificar**:

```
ROI_consenso = (erro_evitado - custo_extra) / custo_extra

Onde:
  erro_evitado   = Custo estimado do erro que o consenso evitou
                   = P(erro_sem_consenso) × custo_do_erro
  custo_extra    = custo_consenso - custo_decisao_unica
                   = custo adicional por usar 3 agentes em vez de 1

REGRAS:
  Se ROI_consenso >= 0 → ✅ Consenso vale a pena (custo justificado)
  Se ROI_consenso < 0  → ❌ Consenso NÃO vale a pena (custo não justificado)
```

### Tabela de Decisão de Custo

| Custo Decisão Única | Custo Consenso (3×) | Custo Extra | Erro Evitado Mínimo | ROI |
|:-------------------:|:-------------------:|:-----------:|:-------------------:|:---:|
| $0.010 | $0.030 | $0.020 | $0.020 | 0% |
| $0.020 | $0.060 | $0.040 | $0.040 | 0% |
| $0.030 | $0.090 | $0.060 | $0.060 | 0% |
| $0.050 | $0.150 | $0.100 | $0.100 | 0% |
| $0.100 | $0.300 | $0.200 | $0.200 | 0% |

**Interpretação**: Se o custo do erro evitado for maior que o custo extra, o consenso vale a pena. Para decisões de baixo custo, o limiar é baixo — qualquer erro significativo justifica.

### Exemplo de ROI do Consenso

```
CENÁRIO: Decisão de arquitetura — escolher entre 3 padrões

Custo decisão única (1 agente): $0.035
Custo consenso (3 agentes):     $0.105
Custo extra:                     $0.070

Probabilidade de erro sem consenso: 25% (1 em 4 decisões arquiteturais falha)
Custo do erro (retrabalho arquitetural): $0.50

erro_evitado = 0.25 × $0.50 = $0.125
ROI_consenso = ($0.125 - $0.070) / $0.070 = $0.055 / $0.070 = 78.6%

→ ROI POSITIVO: ✅ Consenso vale a pena (cada $1 extra gera $1.79 em erro evitado)
```

---

## 8. Integração com F2.1 — Cognitive Economy

### Como o Consensus Engine Alimenta a Cognitive Economy

O resultado do consenso fornece dados para o cálculo de ROI cognitivo:

```
CONSENSUS ENGINE (F8.4)                   COGNITIVE ECONOMY (F2.1)
┌──────────────────────────────┐          ┌─────────────────────────────┐
│ Custo do consenso:            │──custo──▶│ Cost Vector:                │
│  ├── custo_agents: $0.090    │          │  ├── tokens (3 agentes)     │
│  ├── custo_sintese: $0.005   │          │  ├── tempo (coordenação)    │
│  └── custo_total: $0.095     │          │  ├── atenção (3 agentes)    │
│                              │          │  └── storage (resultados)   │
│ Erro evitado pelo consenso:  │──valor──▶│                             │
│  ├── erro_evitado: $0.125    │          │ Value Vector:               │
│  └── P(erro): 25%            │          │  ├── prevenção: $0.125     │
│                              │          │  └── aprendizado: $0.010   │
│ ROI_consenso: 78.6%          │──roi────▶│                             │
│                              │          │ ROI_consenso registrado     │
│                              │          │ em roi-cognitivo.csv        │
└──────────────────────────────┘          └─────────────────────────────┘
```

### Cálculo de ROI Cognitivo para Consenso

O Cognitive Economy Engine (F2.1) trata o consenso como **uma task com custo e valor especiais**:

```
custo_total_consenso:
  ├── custo_tokens   = (tokens_input + tokens_output) × 3 agentes × token_price
  ├── custo_tempo    = time_seconds_coordenacao × opportunity_cost
  └── custo_atencao  = 3 agentes × attention_cost_per_agent

valor_total_consenso:
  ├── valor_aprendizado = learnings_gerados × learning_value
  │     (aprendizados sobre divergência entre agentes)
  ├── valor_reuso       = (patterns_compartilhados encontrados) × reuse_value
  └── valor_prevencao   = erro_evitado
       (O valor MAIS importante do consenso)
```

### ROI do Consenso no Pipeline F2.1

```
PIPELINE F2.1 PÓS-TASK PARA CONSENSO:

1. Coletar dados:
   ├── tokens por agente (input + output)
   ├── tempo de execução de cada agente
   ├── tempo de síntese
   └── custo do erro evitado

2. Calcular custo_total_consenso:
   ├── custo_agents = Σ(custo_individual_agente)
   ├── custo_sintese = $0.005
   ├── custo_coordenacao = $0.003
   └── custo_total = soma

3. Calcular valor_total_consenso:
   ├── valor_prevencao = erro_evitado
   └── valor_total = valor_prevencao (principal)

4. Calcular ROI_consenso:
   ROI = ((valor_total - custo_total) / custo_total) × 100

5. Registrar em roi-cognitivo.csv com tag "consensus"

6. Se ROI_consenso < 0%:
   └── Alerta: "⚠️ Consenso para task {id} teve ROI negativo.
        Custo extra de ${custo_extra} superou o erro evitado de ${erro_evitado}."
```

### Limiar de Ativação via F2.1

O Consensus Engine consulta a Cognitive Economy **antes de ativar**:

```
PRÉ-TASK:
  1. Consensus Engine recebe decisão P1
  2. Consulta Cognitive Economy:
     ├── custo_estimado_consenso = Prediction Engine × 3
     ├── erro_evitado_estimado = P(erro_historico) × custo_medio_erro
     └── ROI_consenso_estimado = (erro_evitado - custo_extra) / custo_extra

  3. Se ROI_consenso_estimado < 0:
     └── NÃO ativar consenso (custo não justificado)
     └── Kernel executa decisão única normalmente

  4. Se ROI_consenso_estimado >= 0:
     └── Ativar consenso (custo justificado pelo risco evitado)
```

---

## 9. Integração com F8.1 — Capability Market

### Seleção de Agentes via Capability Market

O Consensus Engine consulta o Capability Market para selecionar os 3 agentes:

```
CONSENSUS ENGINE (F8.4)                   CAPABILITY MARKET (F8.1)
┌──────────────────────────────┐          ┌─────────────────────────────┐
│ "Preciso de 3 agentes para    │──request▶│ Leilão de capacidades:     │
│  consenso no domínio X"       │          │  ├── domain: X            │
│                              │          │  ├── min_confidence: 0.60  │
│                              │          │  ├── max_agents: 3         │
│                              │          │  └── roles:                │
│                              │          │      primary, secondary,   │
│                              │          │      tertiary              │
│                              │◀──response│                          │
│  Agentes selecionados:       │          │ Ranking:                   │
│  1. cosca-architecture (0.93)│          │  1. architecture (0.93)    │
│  2. cosca-backend (0.90)    │          │  2. backend (0.90)         │
│  3. cosca-critic (0.70)     │          │  3. critic (0.70)          │
└──────────────────────────────┘          └─────────────────────────────┘
```

### Leilão para Consenso

O Capability Market executa um **leilão especial para consenso**:

```yaml
consensus_auction:
  type: "consensus"
  domain: "architecture"
  task_id: "consensus-arch-module-x"
  roles_needed:
    - role: "primary"
      min_domain_strength: 0.80
      prefer: "cosca-architecture"
    - role: "secondary"
      min_domain_strength: 0.60
      domain_adjacent: true     # Domínio complementar
    - role: "tertiary"
      min_domain_strength: 0.40
      critical_profile: true    # Perfil adversarial
```

### Shadow Bids no Consenso

Diferente do leilão padrão, no consenso **todos os 3 agentes executam** — não há "perdedores". Cada agente é um **winner** que contribui com uma perspectiva.

Os shadow bids aqui são **retrospectivos**: "E se tivéssemos incluído o agente X como quarto voto?"

```yaml
consensus_shadow:
  task_id: "consensus-arch-module-x"
  verdict: "medium_consensus"
  shadow_agents:
    - agent: "cosca-infrastructure"
      would_have_voted: "opcao_B"
      would_have_changed_verdict: true
      # Se infrastructure tivesse votado B, agreement_ratio subiria para 3/4 = 0.75
      # consensus_strength passaria de 0.477 para 0.75 × 0.80 = 0.60 → médio
```

---

## 10. Integração com F7.1 — Prediction Engine

### Predição para Consenso

O Prediction Engine é consultado **antes do consenso** para estimar:

1. **P(success)** de cada agente para a task de consenso
2. **Custo estimado** de cada agente
3. **Tempo estimado** de cada agente
4. **Risco score** da decisão

```yaml
consensus_prediction:
  task_id: "consensus-arch-module-x"

  agent_predictions:
    cosca-architecture:
      p_success: 0.93
      custo_estimado: "$0.030"
      tempo_estimado: "12m"
      risco: "🟢 BAIXO (0.07)"

    cosca-backend:
      p_success: 0.85
      custo_estimado: "$0.035"
      tempo_estimado: "15m"
      risco: "🟢 BAIXO (0.15)"

    cosca-critic:
      p_success: 0.72
      custo_estimado: "$0.025"
      tempo_estimado: "18m"
      risco: "🟡 MODERADO (0.28)"

  consensus_total:
    custo_estimado_total: "$0.090"   # Soma dos 3
    tempo_estimado_total: "~18m"     # Paralelo, limitado pelo mais lento
    p_success_consenso: 0.88         # Média ponderada por confiança
```

### Calibragem Pós-Consenso

Após o consenso, o Prediction Engine recebe feedback:

```yaml
prediction_feedback:
  task_id: "consensus-arch-module-x"
  consensus_verdict: "medium_consensus"

  accuracy_by_agent:
    cosca-architecture:
      p_success_previsto: 0.93
      outcome: success
      erro: 0.07           # < 20% → OK
    cosca-backend:
      p_success_previsto: 0.85
      outcome: success
      erro: 0.15           # < 20% → OK
    cosca-critic:
      p_success_previsto: 0.72
      outcome: success
      erro: 0.28           # > 20% → registrar learning

  learning:
    - "cosca-critic tende a ter P(success) subestimado em consensos.
      A natureza adversarial não reduz a qualidade do resultado."
    - "Consenso multi-agente tem P(success) geral maior que a média
      individual (0.88 vs 0.83) — validação do conceito."
```

---

## 11. Feedback no Trust Registry

### Registro de Consenso

Cada consenso gera uma entrada no Trust Registry:

```yaml
- task_id: "consensus-arch-module-x"
  task_type: "consensus"
  outcome: "medium_consensus"
  agents:
    - agent: "cosca-architecture"
      role: "primary"
      confidence_before: 0.93
      confidence_after: 0.95
      confidence_delta: +0.02
      recommendation: "opcao_B"
      accuracy: 1.0              # Acertou

    - agent: "cosca-backend"
      role: "secondary"
      confidence_before: 0.90
      confidence_after: 0.92
      confidence_delta: +0.02
      recommendation: "opcao_B"
      accuracy: 1.0              # Acertou

    - agent: "cosca-critic"
      role: "tertiary"
      confidence_before: 0.70
      confidence_after: 0.65
      confidence_delta: -0.05
      recommendation: "opcao_A"
      accuracy: 0.0              # Errou

  consensus_metrics:
    agreement_ratio: 0.667
    confidence_weighted_avg: 0.715
    consensus_strength: 0.477
    consensus_type: "medium"

  cost:
    total: "$0.095"
    extra_vs_single: "$0.060"
    erro_evitado: "$0.125"
    roi_consenso: "78.6%"

  tags:
    - consensus
    - multi-agent
    - f8.4
    - architecture-decision
```

### Métricas por Agente no Trust Registry

O Trust Registry acumula:

```yaml
consensus_history:
  agent: "cosca-architecture"
  total_consensos: 12
  consensus_accuracy: 0.92         # % de vezes que acertou a recomendação vencedora
  consensus_reliability: 0.88      # % de vezes que esteve no grupo convergente
  avg_confidence_in_consensus: 0.85

  divergences:
    total: 2                       # Quantas vezes foi o divergente
    when_divergent_was_right: 1    # Quantas vezes o divergente estava certo
    when_divergent_was_wrong: 1    # Quantas vezes o divergente estava errado
    divergence_pattern: "Tende a divergir em decisões de segurança"
```

### Consenso como Dado de Calibragem

O histórico de consenso calibra três métricas:

| Métrica | Fórmula | Uso |
|---------|---------|-----|
| **Consensus Accuracy** | `acertos / total_consensos` | Confiabilidade do agente em consenso |
| **Divergence Value** | `divergencia_correta / total_divergencias` | Se o agente é bom em ser "advogado do diabo" |
| **Weight Adjustment** | `accuracy_consenso - accuracy_individual` | Se o agente performa melhor em grupo ou solo |

---

## 12. Don Override

### Prioridade Máxima

O Don pode **ignorar o consenso** a qualquer momento. O override é registrado e rastreado:

```yaml
don_override_consensus:
  task_id: "consensus-arch-module-x"
  consensus_verdict: "medium_consensus"
  consensus_strength: 0.477
  agents_recommendation: "opcao_B (2/3)"
  don_decision: "opcao_A"            # Don escolheu a opção da minoria
  rationale: "Don tem informação privilegiada sobre requisitos futuros"
  override_type: "explicit"
  registered_in_ddna: "DDNA-2026-07-30-004"
```

### Quando o Don Faz Override

```
Don override é esperado quando:
├── Don tem informação contextual que os agentes não têm
├── Don está avaliando trade-offs que não estão no modelo
├── Decisão política ou estratégica (não técnica)
└── Emergência (tempo crítico, não há tempo para consenso)
```

### Override de Emergência

```yaml
don_emergency_consensus:
  task_id: "hotfix-critical-route"
  consensus_skipped: true
  rationale: "Hotfix crítico — tempo insuficiente para consenso multi-agente"
  override_type: "emergency"
  post_action: "Registrar DDNA + executar consenso retrospectivo (shadow)"
```

### Consenso Retrospectivo

Mesmo quando o consenso é pulado por emergência, o Engine pode executar um **consenso retrospectivo**:

```yaml
retrospective_consensus:
  task_id: "hotfix-critical-route"
  type: "retrospective"
  agents:
    - cosca-backend
    - cosca-security
    - cosca-critic
  question: "A abordagem adotada no hotfix foi a melhor?"
  results:
    agreement_ratio: 0.667
    verdict: "A abordagem foi adequada. 2/3 agentes aprovam."
  learning: "Hotfix foi consistente com o que o consenso teria recomendado."
```

---

## 13. Modos de Operação

### Tabela de Modos

| Modo | Agentes | Custo vs Único | Uso | Ativação |
|:----:|:-------:|:--------------:|:---:|:--------:|
| **Full** 🏛️ | 3 (primary + secondary + tertiary) | 3× | Decisões P0, P1 com alto risco | Sempre para P0 |
| **Standard** ⚖️ | 2 (primary + tertiary) | 2× | Decisões P1 com risco moderado | P1 + custo > $0.01 |
| **Light** 🔍 | 2 (primary + secondary) | 2× | Decisões P1 com baixo risco mas custo alto | P1 + custo alto + risco baixo |
| **Shadow** 👻 | 3 (simulação) | 0.3× (só sintese) | Decisões P2/P3 — registra "o que o consenso teria dito" sem bloquear | Don request |
| **Skip** ⏭️ | 1 | 1× | Decisões rotineiras, commands diretos | Fora dos gatilhos |

### Modo Shadow

O modo **Shadow** é especial: ele **não bloqueia a execução**. O Kernel decide sozinho, e o consenso roda em paralelo como simulação:

```
SHADOW MODE:
  1. Kernel recebe decisão P2
  2. Kernel decide sozinho (sem consenso)
  3. Consensus Engine roda em background:
     ├── Seleciona 3 agentes
     ├── Cada agente dá sua recomendação
     └── Engine calcula o que teria recomendado
  4. Pós-task:
     ├── Compara decisão real vs consenso simulado
     ├── Se consenso teria sido diferente:
     │   └── Registra learning: "Para decisões similares, consenso teria recomendado X"
     └── Trust Registry atualizado com accuracy do consenso simulado

  Custo do Shadow: ~$0.005 (apenas síntese — agentes não executam tasks reais)
```

---

## 14. Exemplo Completo

### Cenário: "Qual a melhor estratégia de testes para o runtime WASM?"

**Contexto**: O Cosca tem um runtime WASM (wazero) que precisa de testes robustos. A decisão é P1 (alta prioridade) e o custo estimado é $0.025 (> $0.01).

#### Passo 1: Decisão chega

```yaml
decision:
  id: "DEC-test-strategy-wasm-001"
  question: "Qual a melhor estratégia de testes para o runtime WASM?"
  priority: "P1"
  estimated_cost: "$0.025"
  domain: "testing"
  complexity: 3
```

#### Passo 2: Gatilhos verificados

```yaml
gate_check:
  is_p0: false
  is_p1: true
  custo_superior_limiar: true   # $0.025 > $0.01
  risco_superior_30: false       # risco = 25%
  don_override: false
  resultado: "ATIVAR"            # P1 + custo > $0.01
  modo: "standard"               # 3 agentes
```

#### Passo 3: Agentes selecionados

```yaml
agentes:
  primary:   "cosca-testing"     # domain_strength testing: 0.88
  secondary: "cosca-qa"          # domain_strength qa: 0.80
  tertiary:  "cosca-critic"      # perfil adversarial
```

#### Passo 4: Cada agente executa independente

**cosca-testing (Primary)**:
```yaml
recommendation: "estratégia_B"
confidence: 0.82
rationale: "Testes unitários + integração com foco em cenários de boundary.
             WASM runtime requer testes de memória e isolamento."
alternatives:
  - "estratégia_A": "Testes end-to-end apenas" → descartada (cobertura baixa)
  - "estratégia_B": "Mista unit+integration" → escolhida
  - "estratégia_C": "Property-based testing puro" → descartada (complexidade alta)
```

**cosca-qa (Secondary)**:
```yaml
recommendation: "estratégia_B"
confidence: 0.76
rationale: "Estratégia B oferece melhor relação cobertura/custo.
             Testes de integração capturam bugs de runtime que unitários não pegam."
alternatives:
  - "estratégia_A": "E2E only" → descartada (lenta e frágil)
  - "estratégia_B": "Mista" → escolhida
  - "estratégia_C": "Property-based" → descartada (time-to-market alto)
```

**cosca-critic (Tertiary)**:
```yaml
recommendation: "estratégia_A"
confidence: 0.63
rationale: "Testes E2E são mais representativos do comportamento real do runtime.
             Testes unitários mockam o WASM e podem esconder bugs de integração.
             Risco de falso positivo em unit tests é alto."
alternatives:
  - "estratégia_A": "E2E only" → escolhida
  - "estratégia_B": "Mista" → descartada (mocks não confiáveis para WASM)
  - "estratégia_C": "Property-based" → descartada (time)
```

#### Passo 5: Comparação

```yaml
comparison:
  total_agents: 3
  convergent: 2     # testing + qa → estratégia B
  divergent: 1      # critic → estratégia A
  agreement_ratio: 0.667
```

#### Passo 6: Consensus Strength

```yaml
confidence_weighted_avg:
  testing: 0.82 × 1.0 (convergente) = 0.82
  qa:      0.76 × 1.0 (convergente) = 0.76
  critic:  0.63 × 0.0 (divergente)  = 0.00

  Σ(agreement × confidence) = 0.82 + 0.76 + 0.00 = 1.58
  Σ(confidence)             = 0.82 + 0.76 + 0.63 = 2.21
  weighted_avg              = 1.58 / 2.21 = 0.715

consensus_strength = 0.667 × 0.715 = 0.477

→ 🔴 SEM CONSENSO (strength < 0.5)
→ Gate + Don
```

#### Passo 7: Ação

```yaml
verdict:
  type: "no_consensus"
  strength: 0.477
  action: "GATE + DON"

  majority_rationale: "Estratégia B (mista) — 2/3 agentes, confiança média 0.79"
  minority_rationale: "Estratégia A (E2E) — cosca-critic, preocupação com falso positivo em mocks"

  gate_message: |
    ⚠️ DECISÃO SEM CONSENSO
    ───────────────────────
    Consenso sobre estratégia de testes para runtime WASM:

    2/3 agentes recomendam Estratégia B (mista):
      cosca-testing (confidence: 0.82)
      cosca-qa (confidence: 0.76)

    1/3 agente recomenda Estratégia A (E2E):
      cosca-critic (confidence: 0.63)
      → Preocupação: "Mocks escondem bugs de integração WASM"

    Consensus Strength: 0.477 (< 0.5)
    → Gate aberto. Don precisa decidir.
```

#### Passo 8: Pós-task (resultado hipotético)

```yaml
post_task:
  outcome: "success"
  chosen_strategy: "estratégia_B"   # Don acatou a maioria

  agent_accuracy:
    cosca-testing: acertou ✅  (+0.03 confidence)
    cosca-qa:       acertou ✅  (+0.02 confidence)
    cosca-critic:   errou   ❌  (-0.05 confidence)
    # Nota: critic errou desta vez, mas sua preocupação com mocks
    # foi registrada como learning para monitoramento futuro

  consensus_metrics:
    agreement_ratio: 0.667
    strength: 0.477
    majority_correct: true

  cost:
    custo_consenso: "$0.085"
    custo_unico: "$0.028"
    custo_extra: "$0.057"
    erro_evitado: "$0.080"    # Don estimou que sem consenso teria escolhido A
    roi_consenso: "40.4%"     # ($0.080 - $0.057) / $0.057
```

---

## 15. Métricas do Engine

### Métricas de Operação

```yaml
engine_self_metrics:
  # Ativação
  total_consensos_executados: 0
  total_consensos_skipped: 0
  activation_rate: 0.0          # % de decisões que ativaram consenso

  # Distribuição de vereditos
  consensus_forte: 0            # strength > 0.8
  consensus_medio: 0            # strength 0.5-0.8
  sem_consenso: 0               # strength < 0.5

  # Performance
  avg_time_per_consenso_ms: 0
  avg_custo_consenso: "$0.000"
  avg_custo_extra_vs_unico: "$0.000"

  # Acurácia
  consensus_accuracy: 0.0       # % de vezes que o consenso acertou
  majority_accuracy: 0.0       # % de vezes que a maioria acertou
  minority_accuracy: 0.0       # % de vezes que a minoria (divergente) acertou

  # ROI
  avg_roi_consenso: 0.0        # ROI médio dos consensos
  consensos_com_roi_positivo: 0
  consensos_com_roi_negativo: 0

  # Divergência
  total_divergencias: 0
  divergencia_util: 0           # Divergência que preveniu erro
  divergencia_ruido: 0          # Divergência que estava errada
```

### Dashboard de Consenso

```
┌──────────────────────────────────────────────────────────────────────────┐
│                      CONSENSUS ENGINE DASHBOARD                          │
│                          2026-07-30 — Sessão Atual                       │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│  ATIVAÇÕES                                                                │
│  ┌──────────────────────┐  ┌──────────────────────┐                      │
│  │  Consensos: 12       │  │  Skipped: 4          │                      │
│  │  Ativação: 75%       │  │  (ROI negativo)      │                      │
│  └──────────────────────┘  └──────────────────────┘                      │
│                                                                           │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  VEREDITOS                                                       │   │
│  │                                                                  │   │
│  │  🟢 Consenso Forte:  5  (42%)  ████████████████████░░░░░░        │   │
│  │  🟡 Consenso Médio:  5  (42%)  ████████████████████░░░░░░        │   │
│  │  🔴 Sem Consenso:    2  (16%)  ████████░░░░░░░░░░░░░░░░          │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                           │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  ACURÁCIA                                                       │   │
│  │                                                                  │   │
│  │  Consenso acertou:  83%   ████████████████████████████░░░░     │   │
│  │  Maioria acertou:   92%   ██████████████████████████████████░░  │   │
│  │  Minoria acertou:   17%   ██████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  │   │
│  │                                                                  │   │
│  │  → Insight: Minoria raramente acerta, mas quando acerta         │   │
│  │    previne erros catastróficos (2 divergências úteis em 12)     │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                           │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  ROI DO CONSENSO                                                │   │
│  │                                                                  │   │
│  │  ROI médio: +45.2%  🟢                                           │   │
│  │  Consensos com ROI positivo: 10/12 (83%)                         │   │
│  │  Consensos com ROI negativo: 2/12 (17%) — revisar ativação      │   │
│  │                                                                  │   │
│  │  Custo extra médio: $0.052                                      │   │
│  │  Erro evitado médio: $0.098                                     │   │
│  │  → Cada $1 extra evita $1.89 em erro 🟢                         │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                           │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  DIVERGÊNCIAS                                                    │   │
│  │                                                                  │   │
│  │  Total de divergências: 4 (em 12 consensos = 33%)               │   │
│  │  Divergência útil (minoria certa): 1                             │   │
│  │  Divergência ruído (minoria errada): 3                           │   │
│  │                                                                  │   │
│  │  Agente mais divergente: cosca-critic (3 divergências)           │   │
│  │  → Sugerir calibragem do perfil adversarial                      │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 16. Regras de Operação

### Regras Imutáveis

| # | Regra | Descrição | Consequência |
|---|-------|-----------|--------------|
| **R1** | **Independência total** | Agentes NUNCA sabem da participação ou resposta dos outros | Se contaminado, consenso é inválido e deve ser refeito |
| **R2** | **Custo justificado** | Consenso só ativa se ROI estimado > 0 (consulta F2.1) | Se ROI < 0, consenso não é ativado |
| **R3** | **Don override sempre vence** | Don pode ignorar o consenso a qualquer momento | Override registrado em DDNA com rationale |
| **R4** | **Toda divergência vira dado** | Divergência não é erro — é calibragem para o Trust Registry | Registro obrigatório de accuracy por agente |
| **R5** | **Consenso não é votação** | O peso de cada agente é ponderado por confiança, não é 1 voto = 1 voto | confidence_weighted_avg ajusta o peso |
| **R6** | **Gate para sem consenso** | Strength < 0.5 → obrigatoriamente abre Gate + notifica Don | Não executar automaticamente |
| **R7** | **P0 sempre ativa consenso** | Decisões P0 independentemente de custo ou risco | Exceção: Don override de emergência |

### Limites de Performance

| Operação | Tempo Máximo | Descrição |
|----------|-------------|-----------|
| Verificação de gatilhos | 30ms | Consulta a prioridade, custo, risco |
| Seleção de agentes | 50ms | Consulta ao Capability Market (cache) |
| Execução dos 3 agentes | 3× normal | Cada agente executa em paralelo |
| Comparação e síntese | 100ms | Coleta 3 outputs + calcula fórmula |
| Registro no Trust Registry | 50ms | Atualização do CSV/YAML |
| **Total (orquestração)** | **< 250ms** | **Excluindo tempo de execução dos agentes** |

### Versionamento

```yaml
consensus_model_version:
  id: "CM-MODEL-v1.0.0"
  data: "2026-07-30"

  thresholds:
    consensus_forte: 0.8
    consenso_medio_min: 0.5
    consenso_medio_max: 0.8
    sem_consenso: 0.5

  pesos_selecao:
    domain_expertise: 0.40
    complementaridade: 0.25
    disponibilidade: 0.20
    eficiencia_custo: 0.15

  pesos_formula:
    agreement_ratio: 1.0          # Peso direto (multiplica)
    confidence_weighted_avg: 1.0  # Peso direto (multiplica)

  custo:
    custo_sintese: 0.005
    custo_coordenacao: 0.003
    custo_por_agente: 1.0        # 1× o custo de uma task simples

  calibration_history:
    - version: "v1.0.0"
      data: "2026-07-30"
      razao: "Versão inicial do Multi Provider Consensus Engine"
      consensos: 0
```

---

## 17. Tratamento de Erros

| Falha | Ação | Impacto |
|-------|------|---------|
| Apenas 1 ou 2 agentes disponíveis | Consenso reduzido (2 agentes). Registrar "consenso parcial" | Menos robusto, mas ainda válido |
| Nenhum agente disponível | Consenso impossível. Kernel decide sozinho. Registrar DDNA. | Decisão sem consenso |
| Agente não completa a task em 2× o tempo estimado | Substituir por próximo do ranking. Registrar timeout. | Consenso com substituto |
| Dois agentes convergem, um não responde | Consenso com 2/2 = 1.0 agreement_ratio. Registrar "consenso incompleto". | Consenso válido, mas com ressalva |
| Todos os 3 divergem (0/3) | Consenso impossível. Gate + Don + registrar "divergência total". | Escalado para Don |
| Prediction Engine indisponível | Usar confidence do Trust Registry (sem ajuste de predição) | Score menos preciso |
| Cognitive Economy indisponível | Assumir ROI > 0 (otimista). Registrar "ROI não verificado". | Pode ativar consenso sem justificativa |
| Fórmula de consenso falha | Fallback: agreement_ratio puro (sem ponderação por confiança) | Consenso menos preciso |
| Contaminação detectada (agente soube do outro) | Consenso inválido. Reexecutar com novos agentes. Penalizar agente contaminado. | Atraso + custo extra |

---

## 18. Relacionados

| Documento | Seção | Relação |
|-----------|-------|---------|
| **F2.1 — Cognitive Economy** | [cognitive-economy/SKILL.md](../cognitive-economy/SKILL.md) | ROI do consenso, custo vs valor, limiar de ativação |
| **F8.1 — Capability Market** | [capability-market/SKILL.md](../capability-market/SKILL.md) | Seleção de agentes para consenso, leilão de capacidades |
| **F7.1 — Prediction Engine** | [prediction/SKILL.md](../prediction/SKILL.md) | P(success), custo estimado, risco score por agente |
| **F7.2 — Trust Registry** | [memory/trust/TRUST_REGISTRY.md](../../memory/trust/TRUST_REGISTRY.md) | Registro de accuracy por agente em consensos |
| **F1.1 — Decision DNA** | [DECISION_DNA.md](../../knowledge/architecture/DECISION_DNA.md) | DDNA de decisões multi-agente |
| **KERNEL.md** | §10 Steps 5-7 | Pipeline de decisão, ponto de ativação do consenso |
| **next-evolution-phases.md** | §F8.4 | Definição original da feature |

---

## 19. Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Architecture Chief | Criação inicial. Especificação completa: gatilhos, pipeline, fórmula de consenso, seleção de agentes, custo e ROI, integração F2.1/F8.1/F7.1, Don override, modos de operação, exemplo real, métricas do engine. |

---

> **Enforced by**: Cosca Architecture Chief | **Próxima calibragem**: Após 10 consensos executados
> **Kernel instruction**: `cosca consensus run --task <task_id> --mode <full|standard|light|shadow>`
