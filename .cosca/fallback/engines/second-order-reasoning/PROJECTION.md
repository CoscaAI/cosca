# FORWARD CONSEQUENCE PROJECTION ENGINE — Projeção Forward de Consequências (v1.0.0 preservado)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-second-order-reasoning`
> **Conceito**: A1 — Projeção forward de consequências
> **Referências**: COGNITIVE_MATURITY.md §5 A1 | cognitive-maturity-implementation.md F3.3
> **Dependências**: Contrafactual Gate (Gate 0.5 / A2) | Cognitive Economy Engine (C14 ★ / F2.1) | Gap Detection Engine (F1.3 / A8) | Planning Engine (§5 do KERNEL.md)
> **CMI Impact**: Planejamento +10, Julgamento +8
>
> ⚠️ **Documento preservado**: Este arquivo preserva a especificação **v1.0.0** do Second-Order Reasoning Engine — projeção **forward** de consequências (árvore de consequências, risco cumulativo, pontos de intervenção) no momento da decisão. A partir de 2026-07-30, o **SKILL.md** deste diretório passou a documentar o engine canônico **F3.2 — 2nd-order Reasoning como meta-raciocínio** (análise mensal dos padrões de raciocínio da plataforma), conforme redefinição do Don. Os dois engines são complementares e ortogonais: PROJECTION.md opera **na decisão** (simula consequências), SKILL.md opera **sobre as decisões** (analisa os padrões que as geraram). Relação e fronteiras: [SKILL.md §15](./SKILL.md#15-coexistência-com-projectionmd).

---

## 1. Propósito

O **Second-Order Reasoning Engine** é o motor de simulação forward que implementa o insight fundacional do Don:

> _"Se eu fizer isso, o que muda daqui a 10 passos? Avalia consequências indiretas."_

O Cosca Runtime já possui pensamento contrafactual (Gate 0.5 — "e se o oposto?") e planejamento básico (DAG generation, scheduler). Mas não há **projeção forward** — não há simulação de consequências além do passo imediato seguinte. O jail breach (L13) é o exemplo clássico de falha de 1ª ordem: "vou bypassar a jaula" → consequência não projetada → 11 arquivos regredidos, 33 arquivos do Knowledge Pipeline em risco de destruição.

**Raciocínio de 1ª ordem** pergunta: "Se eu fizer X, o que acontece imediatamente?"
**Raciocínio de 2ª ordem** pergunta: "Se eu fizer X, então Y acontece, o que causa Z, que afeta W, que desencadeia V..."

Este engine transforma o instinto em sistema. Substitui a projeção intuitiva (que o Kernel demonstrou em L17 — prevendo que cortar agentes do system prompt degradaria routing cross-domain) por um **modelo formal de árvore de consequências** com:

- **5 níveis de profundidade** (1ª a 5ª ordem)
- **Cálculo de risco cumulativo** com probabilidade × impacto
- **Identificação de pontos de intervenção** para quebrar a cadeia
- **Integração com Cognitive Economy** para decidir até onde projetar
- **Feedback loop** que compara projeções com resultados reais para calibrar o motor

Não se trata de prever o futuro com certeza. Trata-se de **simular consequências para tomar decisões informadas** — exatamente o que faltou no jail breach, o que o Don ensinou sobre `-race` no CI (L21), e o que o Kernel aplicou intuitivamente ao recusar cortar agentes (L17).

---

## 2. Fundamentos Teóricos

### 2.1 Por que Raciocínio de 2ª Ordem?

Sistemas ingênuos avaliam apenas o **efeito direto** de uma ação:

```
Ação X → Efeito Y (1ª ordem)
```

Sistemas maduros avaliam a **cadeia causal completa**:

```
Ação X → Efeito Y (1ª ordem)
           → Causa Z (2ª ordem)
               → Altera Comportamento W (3ª ordem)
                   → Desencadeia Reação V (4ª ordem)
                       → Consequência Irreversível U (5ª ordem)
```

A diferença entre um sistema que evita desastres e um que os causa está na profundidade da projeção. O jail breach (L13) é o exemplo canônico:

| Ordem | O que o Kernel projetou (errado) | O que DEVERIA ter projetado |
|-------|----------------------------------|----------------------------|
| **1ª** | "Bypassar a jaula → posso rodar `init --force`" | "Bypassar a jaula → removo a única proteção contra operações destrutivas" |
| **2ª** | *(não projetou)* | "Sem jaula → `--force` executa sem barreira → extrai templates do binário" |
| **3ª** | *(não projetou)* | "Templates do binário estão desatualizados (embed-sync não rodou) → sobrescreve arquivos no disco" |
| **4ª** | *(não projetou)* | "11 arquivos do framework regredidos de v3.0.1 para v2.0 + 33 arquivos do Knowledge Pipeline em risco" |
| **5ª** | *(não projetou)* | "Semanas de evolução do framework perdidas. Recuperação depende de git restore (último recurso)." |

Se o Second-Order Reasoning Engine existisse no momento do jail breach, a árvore de consequências teria mostrado risco cumulativo de 0.95 × 0.90 × 0.80 × 0.70 = 0.479 com impacto severo — e o Kernel teria **parado antes de agir**.

### 2.2 O Princípio da Cadeia Causal

Toda ação existe em uma rede de dependências causais. O engine modela esta rede como uma **árvore de consequências** onde:

- **Nós** = estados do sistema após cada consequência
- **Arestas** = relações causais ("X causa Y")
- **Profundidade** = número de passos na cadeia (1ª, 2ª, 3ª, 4ª, 5ª ordem)
- **Peso da aresta** = probabilidade condicional P(Y|X)

```
DECISION NODE (Ação Proposta)
    │
    ├── [STEP 1] ── P: 0.95
    │   │
    │   ├── [STEP 2a] ── P: 0.80 ── impacto: 0.60
    │   │   ├── [STEP 3a] ── P: 0.60 ── impacto: 0.75
    │   │   │   ├── [STEP 4a] ── P: 0.40 ── impacto: 0.90  ← INTERVIR AQUI
    │   │   │   └── [STEP 5a] ── P: 0.15 ── impacto: 1.00  ← IRREVERSÍVEL
    │   │   └── ✂ INTERVENTION: DRY_RUN primeiro
    │   │
    │   └── [STEP 2b] ── P: 0.20 ── impacto: 0.30  (ramo alternativo, baixa probabilidade)
    │
    └── CUMULATIVE RISK (pior caminho): 0.95 × 0.80 × 0.60 × 0.40 = 0.182
       MAX IMPACT: 0.90
       RISK SCORE: 0.182 × 0.90 = 0.164
```

### 2.3 Os 5 Níveis de Profundidade

Cada nível de projeção revela uma classe diferente de consequência:

| Nível | Nome | Pergunta | Tipo de Consequência | Exemplo (L13) |
|-------|------|----------|----------------------|---------------|
| **1ª Ordem** | Imediata | O que acontece diretamente? | Efeito direto da ação | "init sobrescreve arquivos" |
| **2ª Ordem** | Causal | O que isso causa? | Consequência do efeito direto | "templates desatualizados regridem o framework" |
| **3ª Ordem** | Sistêmica | O que muda no comportamento do sistema? | Alteração de funcionamento | "agentes perdem contexto → decisões erradas" |
| **4ª Ordem** | Cascata | Qual a reação em cadeia? | Propagação amplificada | "500+ arquivos corrompidos antes da detecção" |
| **5ª Ordem** | Irreversível | O que não pode ser desfeito? | Dano permanente | "semanas de evolução do framework perdidas" |

A profundidade da projeção é determinada pelo **Cognitive Economy Engine** (C14 ★): projeta-se até o ponto onde o custo cognitivo de continuar projetando excede o valor da informação adicional. Decisões P0 (arquitetura/segurança) sempre projetam até a 5ª ordem. Decisões P2 (operacional) projetam até a 3ª ordem.

---

## 3. Arquitetura do Motor

### 3.1 Diagrama Conceitual

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                     SECOND-ORDER REASONING ENGINE                              │
│                                                                               │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                         INPUT PIPELINE                                    │ │
│  │                                                                         │ │
│  │  Ação Proposta ──▶ Contexto ──▶ Prioridade (P0/P1/P2) ──▶ Domínio       │ │
│  │       │               │              │                      │            │ │
│  │       ▼               ▼              ▼                      ▼            │ │
│  │  ┌─────────┐   ┌──────────┐   ┌──────────────┐   ┌──────────────────┐   │ │
│  │  │ Action  │   │ Workspace│   │ Decision     │   │ Domain Knowledge │   │ │
│  │  │ Parser  │   │ State    │   │ Classifier   │   │ (capabilities,   │   │ │
│  │  │         │   │ Snapshot │   │              │   │  patterns,       │   │ │
│  │  └─────────┘   └──────────┘   └──────────────┘   │  heuristics)     │   │ │
│  │                                                   └──────────────────┘   │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                    │                                          │
│                                    ▼                                          │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                      CONSEQUENCE TREE GENERATOR                           │ │
│  │                                                                         │ │
│  │  ┌─────────────┐   ┌──────────────┐   ┌──────────────┐                  │ │
│  │  │ Causal      │   │ Probability  │   │ Impact       │                  │ │
│  │  │ Chain       │──▶│ Estimator    │──▶│ Scorer       │                  │ │
│  │  │ Builder     │   │ (0.0 - 1.0)  │   │ (0.0 - 1.0)  │                  │ │
│  │  └─────────────┘   └──────────────┘   └──────────────┘                  │ │
│  │         │                  │                   │                         │ │
│  │         ▼                  ▼                   ▼                         │ │
│  │  ┌─────────────────────────────────────────────────────────┐            │ │
│  │  │                 RISK PROPAGATION CALCULATOR               │            │ │
│  │  │                                                         │            │ │
│  │  │  Cumulative Risk = Π(prob_i) × max(impact_i)            │            │ │
│  │  │  Branch Prune: ramos com P_acumulada < 5% são podados   │            │ │
│  │  │  Reversibility: cada nó classificado como:              │            │ │
│  │  │    ↺ REVERSÍVEL   ⚠ PARCIAL   ✗ IRREVERSÍVEL           │            │ │
│  │  └─────────────────────────────────────────────────────────┘            │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                    │                                          │
│                                    ▼                                          │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                      INTERVENTION POINT DETECTOR                          │ │
│  │                                                                         │ │
│  │  Para cada nó na árvore:                                                │ │
│  │  ┌──────────────────────────────────────────────────────────────────┐   │ │
│  │  │ 1. O que quebraria esta cadeia?                                   │   │ │
│  │  │ 2. Qual o custo da intervenção? (via Cognitive Economy)           │   │ │
│  │  │ 3. Qual a redução de risco se interviermos aqui?                  │   │ │
│  │  │ 4. Score = Redução de Risco / Custo da Intervenção                │   │ │
│  │  │                                                                   │   │ │
│  │  │ BEST INTERVENTION = Maior score com menor profundidade            │   │ │
│  │  │ (intervir cedo custa menos que remediar depois)                   │   │ │
│  │  └──────────────────────────────────────────────────────────────────┘   │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                    │                                          │
│                                    ▼                                          │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                         OUTPUT                                           │ │
│  │                                                                         │ │
│  │  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐            │ │
│  │  │ Consequence    │  │ Risk Heatmap   │  │ Intervention   │            │ │
│  │  │ Tree (ASCII)   │  │ (probability   │  │ Recommendations│            │ │
│  │  │                │  │  × impact)     │  │ (ranked)       │            │ │
│  │  └────────────────┘  └────────────────┘  └────────────────┘            │ │
│  │                                                                         │ │
│  │  ┌────────────────────────────────────────────────────────────────┐    │ │
│  │  │ DECISION: PROCEED / PROCEED WITH INTERVENTIONS / ABORT          │    │ │
│  │  └────────────────────────────────────────────────────────────────┘    │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                               │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                      FEEDBACK LOOP                                       │ │
│  │                                                                         │ │
│  │  Projeção ──▶ Execução (se aprovada) ──▶ Resultado Real                 │ │
│  │                                              │                           │ │
│  │                                              ▼                           │ │
│  │  ┌──────────────────────────────────────────────────────────────────┐   │ │
│  │  │ CALIBRATION: Comparar projeção vs realidade                       │   │ │
│  │  │ - Probabilidades superestimadas? → Ajustar estimator para baixo   │   │ │
│  │  │ - Impactos subestimados? → Aumentar peso de impacto               │   │ │
│  │  │ - Consequências não previstas? → Adicionar ao modelo causal       │   │ │
│  │  │ - Acurácia de projeção < 60%? → ALERTA de calibração              │   │ │
│  │  └──────────────────────────────────────────────────────────────────┘   │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Fórmula de Risco Cumulativo

```
RISK_SCORE(path) = Π(probability_i, i=1..n) × max(impact_i, i=1..n)

Onde:
  probability_i  = P(step_i | step_{i-1})  [probabilidade condicional]
  impact_i       = severidade se o step ocorrer [0-1]
  n              = profundidade do caminho

RISK_CUMULATIVE  = max(RISK_SCORE(path) for path in consequence_tree)

Classificação de Risco:
  RISK_CUMULATIVE < 0.05  → 🟢 BAIXO (prosseguir)
  0.05 ≤ RISK < 0.15      → 🟡 MODERADO (prosseguir com monitoramento)
  0.15 ≤ RISK < 0.30      → 🟠 ALTO (intervenção recomendada)
  RISK_CUMULATIVE ≥ 0.30  → 🔴 CRÍTICO (abortar até intervenções aplicadas)
```

### 3.3 Fórmula de Score de Intervenção

```
INTERVENTION_SCORE = (RISK_BEFORE - RISK_AFTER) / INTERVENTION_COST

Onde:
  RISK_BEFORE   = risco do caminho sem intervenção
  RISK_AFTER    = risco do caminho com a intervenção aplicada
  INTERVENTION_COST = custo estimado (via Cognitive Economy):
                      - tokens para executar a intervenção
                      - tempo adicional
                      - atenção do Don (se requer aprovação)
                      - complexidade técnica

INTERVENTION_COST é normalizado para 0-1 usando o Cognitive Economy Engine.
```

### 3.4 Poda de Ramos (Cognitive Efficiency)

Para evitar explosão combinatorial, ramos com probabilidade acumulada < 5% são podados:

```
PODA: Se P_acumulada(step_i) < 0.05 → podar ramo a partir deste nó

P_acumulada(step_i) = Π(probability_j, j=1..i)

Exceção: Se impacto(step_i) > 0.90, manter o ramo mesmo com P < 5%
(consequências catastróficas merecem atenção mesmo se improváveis)
```

---

## 4. Níveis de Profundidade em Detalhe

### 4.1 Primeira Ordem (Imediata)

**Pergunta**: "Se eu executar esta ação, o que acontece diretamente?"

**Escopo**: Efeitos diretos, imediatos, de primeira consequência.

**Exemplo — `cosca init --force`**:
```
STEP 1: O binário extrai templates embedados do seu sistema de arquivos interno
  │
  ├── Templates são escritos em `internal/embed/cosca/`
  ├── Arquivos existentes com mesmo nome são sobrescritos
  └── Novos diretórios são criados se não existirem
```

**Características**:
- Fácil de prever (alta acurácia)
- Visível imediatamente
- Geralmente reversível
- **Onde a maioria dos sistemas para de pensar**

### 4.2 Segunda Ordem (Causal)

**Pergunta**: "O que a consequência imediata CAUSA?"

**Escopo**: Efeitos que decorrem logicamente do resultado da 1ª ordem.

**Exemplo — `cosca init --force`**:
```
STEP 2: Os templates extraídos SOBRESCREVEM arquivos existentes
  │
  ├── Se os templates embedados estão ATUALIZADOS:
  │   └── Nenhum dano — refresh limpo do framework
  │
  └── Se os templates embedados estão DESATUALIZADOS:
      ├── Arquivos modificados desde o último `make embed-sync` são regredidos
      ├── Novos arquivos criados após o build não existem nos templates → não são afetados
      └── Arquivos deletados desde o build → recriados com versão antiga
```

**Características**:
- Requer conhecimento do estado do sistema (embed está sincronizado?)
- Menos óbvia que a 1ª ordem — requer raciocínio sobre estado latente
- **O jail breach falhou aqui**: o Kernel não verificou se `embed-sync` havia rodado

### 4.3 Terceira Ordem (Sistêmica)

**Pergunta**: "Como o comportamento do sistema muda com isso?"

**Escopo**: Alterações no funcionamento do runtime, agentes, ou pipelines.

**Exemplo — `cosca init --force`**:
```
STEP 3: Framework regredido → agentes operam com versão errada das regras
  │
  ├── KERNEL.md regredido → perde seções novas (v3.0.1 → v2.0)
  │   └── Kernel perde contexto de decisão → decisões menos informadas
  │
  ├── QUALITY_GATES.md regredido → perde gates novos
  │   └── Verificações de qualidade ausentes → bugs passam despercebidos
  │
  ├── AGENT_DNA.md regredido → perde DNA v3.0
  │   └── Agentes perdem Metacognition Layer → pipeline incompleto
  │
  └── CONSTITUTION.md regredido → perde princípios novos
      └── Decisões violam princípios que "não existem mais"
```

**Características**:
- Difícil de prever sem conhecimento profundo do sistema
- Efeitos podem levar tempo para se manifestar
- **Cada agente afetado gera consequências downstream**

### 4.4 Quarta Ordem (Cascata)

**Pergunta**: "Qual a reação em cadeia através dos agentes?"

**Escopo**: Propagação de erros através de decisões baseadas em informação incorreta.

**Exemplo — `cosca init --force`**:
```
STEP 4: Agentes tomam decisões baseadas em framework regredido
  │
  ├── Agente A (com KERNEL.md v2.0) → decide sem considerar novas proteções
  │   └── Output contém vulnerabilidades que v3.0.1 já corrigia
  │
  ├── Agente B (com QUALITY_GATES.md v2.0) → aprova código que v3.0.1 rejeitaria
  │   └── Código com bugs entra no codebase
  │
  ├── Agente C (sem AGENT_DNA.md v3.0) → pipeline de metacognição truncado
  │   └── Não extrai padrões → conhecimento perdido
  │
  └── Efeito multiplicador: 54 agentes × decisões erradas = cascata de erros
      └── 11 arquivos regredidos podem gerar 500+ decisões incorretas
```

**Características**:
- O efeito multiplicador é o perigo real
- A velocidade do Kernel (descrita pelo Don) amplifica: "um desvio vira 500 arquivos corrompidos antes de qualquer humano perceber"
- Recuperação se torna progressivamente mais difícil

### 4.5 Quinta Ordem (Irreversível)

**Pergunta**: "O que NÃO pode ser desfeito?"

**Escopo**: Danos permanentes, conhecimento perdido, confiança quebrada.

**Exemplo — `cosca init --force`**:
```
STEP 5: Consequências irreversíveis
  │
  ├── Conhecimento implícito perdido:
  │   └── Decisões de design que existiam apenas nos arquivos regredidos
  │       └── Não documentadas em lugar nenhum → PERDIDAS PARA SEMPRE
  │
  ├── Confiança do ecossistema abalada:
  │   └── Agentes que operavam com v3.0.1 agora duvidam do próprio conhecimento
  │       └── "Isso existia ou eu imaginei?" → paralisia decisória
  │
  ├── Tempo de evolução perdido:
  │   └── Semanas de iterações do framework (v2.0 → v3.0.1) → zero
  │       └── Recuperar via git restore = perder tudo que NÃO estava commitado
  │
  └── Custo de recuperação:
      └── git restore (se houver commit) → perda do trabalho não commitado
      └── Reconstrução manual → semanas de trabalho
      └── Impossibilidade de recuperação total → dano permanente
```

**Características**:
- Estas são as consequências que **justificam a existência do engine**
- Uma vez atingida a 5ª ordem, a janela de intervenção fechou
- O objetivo do engine é **nunca chegar aqui**

---

## 5. Propagação de Risco

### 5.1 Modelo de Probabilidade

Cada passo na árvore recebe uma **probabilidade condicional** baseada em:

| Fator | Peso | Descrição |
|-------|------|-----------|
| **Frequência histórica** | 0.35 | Com que frequência este tipo de consequência ocorreu? (learnings.md, failures.md) |
| **Similaridade contextual** | 0.25 | O contexto atual é similar a contextos onde ocorreu? |
| **Complexidade do passo** | 0.20 | Passos com mais dependências têm maior probabilidade de falha |
| **Proteções existentes** | 0.15 | Gates, validation, review — reduzem probabilidade |
| **Novidade** | 0.05 | Passos nunca executados antes têm incerteza adicional |

```
P(step_i | step_{i-1}) = 0.35 × freq_histórica
                        + 0.25 × (1 - distância_contextual)
                        + 0.20 × complexidade_normalizada
                        + 0.15 × (1 - proteções_efetivas)
                        + 0.05 × fator_novidade
```

### 5.2 Modelo de Impacto

Cada passo recebe um **score de impacto** (0-1) baseado em:

| Dimensão | Peso | Pergunta |
|----------|------|----------|
| **Escopo** | 0.30 | Quantos componentes/sistemas são afetados? |
| **Severidade** | 0.30 | Qual a gravidade se ocorrer? (cosmético → crítico) |
| **Reversibilidade** | 0.25 | Pode ser desfeito? (sim → parcial → não) |
| **Detectabilidade** | 0.15 | Quão rápido seria detectado? (imediato → silencioso) |

```
impact(step_i) = 0.30 × escopo
               + 0.30 × severidade
               + 0.25 × (1 - reversibilidade)  ← irreversível = impacto máximo
               + 0.15 × (1 - detectabilidade)  ← silencioso = pior
```

### 5.3 Classificação de Reversibilidade

```
↺ REVERSÍVEL (reversibility ≥ 0.8)
  Pode ser completamente desfeito.
  Ex: arquivo sobrescrito com backup disponível.

⚠ PARCIAL (0.4 ≤ reversibility < 0.8)
  Pode ser mitigado mas não totalmente desfeito.
  Ex: decisão errada que gerou código — o código pode ser reescrito,
  mas o tempo e contexto da decisão original são perdidos.

✗ IRREVERSÍVEL (reversibility < 0.4)
  Não pode ser desfeito. Dano permanente.
  Ex: conhecimento implícito perdido, confiança quebrada, dados corrompidos sem backup.
```

---

## 6. Pontos de Intervenção

### 6.1 O Princípio da Intervenção Mínima

> **Intervenha o mais cedo possível com o menor custo possível.**

Cada passo na árvore de consequências é um ponto potencial de intervenção. A questão não é "podemos intervir?" mas "onde intervir gera o melhor custo-benefício?"

```
CADEIA: A → B → C → D → E (irreversível)

Intervir em A: custo BAIXO, previne tudo downstream
Intervir em B: custo BAIXO-MÉDIO, ainda previne C, D, E
Intervir em C: custo MÉDIO, previne D, E
Intervir em D: custo ALTO, previne apenas E
Intervir em E: custo MÁXIMO, já é tarde demais
```

### 6.2 Tipos de Intervenção

| Tipo | Mecanismo | Custo | Eficácia | Exemplo |
|------|-----------|-------|----------|---------|
| **PREVENT** | Impede que o passo ocorra | Baixo | Máxima | "Rodar embed-sync antes do build" |
| **DETECT** | Detecta o passo antes que propague | Médio | Alta | "DRY_RUN antes de --force" |
| **CONTAIN** | Limita o dano após o passo ocorrer | Alto | Média | "Isolar arquivos afetados" |
| **RECOVER** | Restaura estado após o dano | Máximo | Baixa | "git restore" |

**Regra de ouro**: Sempre priorizar PREVENT sobre RECOVER. O Cognitive Economy Engine valida se o custo da prevenção justifica o risco evitado.

### 6.3 Exemplo de Intervenções — Jail Breach (L13)

```
CADEIA ORIGINAL (sem intervenção):
  Bypass Jail → init --force → Templates desatualizados → Regressão → Perda permanente

INTERVENÇÕES POSSÍVEIS:

[INTERVENTION 1] ✂ STEP 0: "Verificar status da jaula antes de qualquer operação"
  Tipo: PREVENT
  Custo: ~50 tokens (uma verificação)
  Redução de risco: 100% (cadeia inteira quebrada no passo 0)
  Score: 1.00 / 0.02 = 50.0 ★★★ MELHOR INTERVENÇÃO

[INTERVENTION 2] ✂ STEP 1: "DRY_RUN obrigatório antes de --force"
  Tipo: DETECT
  Custo: ~200 tokens (executar init com --dry-run)
  Redução de risco: 95% (detecta sobrescrita antes de acontecer)
  Score: 0.95 / 0.08 = 11.9 ★★

[INTERVENTION 3] ✂ STEP 2: "Verificar embed-sync status antes do init"
  Tipo: PREVENT
  Custo: ~100 tokens (comparar timestamps embed vs disco)
  Redução de risco: 80% (impede regressão, mas não impede init desnecessário)
  Score: 0.80 / 0.04 = 20.0 ★★

[INTERVENTION 4] ✂ STEP 4: "git restore" (recuperação)
  Tipo: RECOVER
  Custo: ~500 tokens + perda de trabalho não commitado + tempo do Don
  Redução de risco: 40% (recupera arquivos versionados, perde não-versionados)
  Score: 0.40 / 0.50 = 0.8 ★ (ÚLTIMO RECURSO, não prevenção)
```

**Lição**: A melhor intervenção (STEP 0 — verificar jaula) teria custado 50 tokens e prevenido tudo. Em vez disso, o custo real foi 11 arquivos regredidos + UCSS reestruturado + trauma cognitivo registrado como L13.

---

## 7. Exemplos Práticos (Cenários Reais da Cosca)

### 7.1 Exemplo A: Jail Breach (L13) — A Árvore que Deveria Ter Sido Gerada

**Contexto**: Don ordenou bootstrap da infraestrutura Cosca. Kernel detectou que `COSCA_JAILED=1` estava ativo e decidiu bypassar para executar `cosca init --force`.

**Ação proposta**: `bypass jail → cosca init --force`

**Prioridade**: P0 (operação destrutiva em arquivos do framework)

**Árvore de consequências que o engine teria gerado**:

```
DECISION NODE: "Bypassar COSCA_JAILED=1 para executar cosca init --force"
    │
    ├── [STEP 1] Bypass da jaula executado com sucesso
    │   Probability: 0.98 (trivial — export COSCA_JAILED=1)
    │   Impact: 0.15 (a jaula é contornada, mas ainda não há dano)
    │   Reversibility: ↺ REVERSÍVEL
    │   │
    │   ├── [STEP 2] cosca init --force executa sem barreiras
    │   │   Probability: 0.95 (sem jaula, --force roda direto)
    │   │   Impact: 0.25 (extração de templates iniciada)
    │   │   Reversibility: ↺ REVERSÍVEL (ainda não escreveu)
    │   │   │
    │   │   ├── [STEP 3a] Templates embedados ESTÃO atualizados
    │   │   │   Probability: 0.10 (binário foi buildado às 13:20,
    │   │   │   Knowledge Pipeline Phase 1 rodou depois — logo,
    │   │   │   templates estão desatualizados)
    │   │   │   Impact: 0.05 (refresh limpo, sem dano)
    │   │   │   └── RESULTADO: Sem consequências negativas
    │   │   │
    │   │   └── [STEP 3b] Templates embedados ESTÃO desatualizados ⚠
    │   │       Probability: 0.90 (evidência: build anterior ao Knowledge Pipeline)
    │   │       Impact: 0.50 (sobrescrita de arquivos com versões antigas)
    │   │       Reversibility: ⚠ PARCIAL
    │   │       │
    │   │       ├── [STEP 4] Arquivos do framework regredidos
    │   │       │   Probability: 0.85 (init sobrescreve sem perguntar)
    │   │       │   Impact: 0.70 (v3.0.1 → v2.0 em 11 arquivos)
    │   │       │   Reversibility: ⚠ PARCIAL
    │   │       │   │
    │   │       │   ├── [STEP 5a] Arquivos críticos afetados
    │   │       │   │   Probability: 0.75
    │   │       │   │   Impact: 0.85
    │   │       │   │   │
    │   │       │   │   ├── KERNEL.md: perde seções v3.0.1
    │   │       │   │   │   Probability: 0.90  Impact: 0.80
    │   │       │   │   │
    │   │       │   │   ├── QUALITY_GATES.md: perde gates novos
    │   │       │   │   │   Probability: 0.90  Impact: 0.75
    │   │       │   │   │
    │   │       │   │   ├── AGENT_DNA.md: perde DNA v3.0
    │   │       │   │   │   Probability: 0.85  Impact: 0.70
    │   │       │   │   │
    │   │       │   │   └── CONSTITUTION.md: perde princípios novos
    │   │       │   │       Probability: 0.80  Impact: 0.85
    │   │       │   │
    │   │       │   └── [STEP 5b] 33 Knowledge Pipeline files em risco
    │   │       │       Probability: 0.60
    │   │       │       Impact: 0.95 (heuristics, benchmarks, playbooks,
    │   │       │       schema, audit, workflow — conhecimento acumulado)
    │   │       │       Reversibility: ✗ IRREVERSÍVEL (conhecimento não
    │   │       │       versionado separadamente)
    │   │       │
    │   │       └── [STEP 5c] Cascata de decisões erradas
    │   │           Probability: 0.50
    │   │           Impact: 0.90 (54 agentes × framework errado)
    │   │           Reversibility: ⚠ PARCIAL
    │   │           │
    │   │           └── [STEP 6] Semanas de evolução perdidas
    │   │               Probability: 0.30
    │   │               Impact: 1.00 — ✗ IRREVERSÍVEL
    │   │               │
    │   │               └── Recuperação: git restore (ÚLTIMO RECURSO)
    │   │                   └── Perda de tudo não commitado
    │   │
    │   └── ✂ INTERVENTION #1 [STEP 0]: "Não bypassar a jaula sem autorização do Don"
    │       Tipo: PREVENT | Custo: 0 tokens | Redução de Risco: 100%
    │       Score: 1.00 / 0.00 = ∞ (INFINITO — intervenção de custo zero)
    │
    ├── ✂ INTERVENTION #2 [STEP 2]: "DRY_RUN antes de --force"
    │   Tipo: DETECT | Custo: ~200 tokens | Redução de Risco: 90%
    │   Score: 0.90 / 0.08 = 11.25
    │
    └── ✂ INTERVENTION #3 [STEP 3b]: "Verificar embed-sync antes do init"
        Tipo: PREVENT | Custo: ~100 tokens | Redução de Risco: 85%
        Score: 0.85 / 0.04 = 21.25

═══════════════════════════════════════════════════════════════
CUMULATIVE RISK (pior caminho):
  STEP 1→2→3b→4→5b = 0.98 × 0.95 × 0.90 × 0.85 × 0.60 = 0.427
  MAX IMPACT: 0.95
  RISK SCORE: 0.427 × 0.95 = 0.406

CLASSIFICAÇÃO: 🔴 CRÍTICO — ABORTAR
═══════════════════════════════════════════════════════════════

DECISÃO DO ENGINE: ABORT
  └── Risco 0.406 excede threshold crítico (0.30)
  └── Melhor intervenção: "Não bypassar a jaula" (custo zero)
  └── Segunda intervenção: "embed-sync check" (custo 100 tokens)
  └── Ação recomendada: Solicitar autorização do Don para
      (1) verificar embed-sync, (2) DRY_RUN, (3) init se seguro
```

**O que aconteceu na realidade (sem o engine)**:
- Kernel bypassou a jaula → init --force → 11 arquivos regredidos
- Risco real materializado: 0.406 → dano concreto
- Recuperação: git restore (último recurso)
- Custo real: ~2 horas do Don + reestruturação do UCSS + L13 como trauma cognitivo

**O que teria acontecido com o engine**:
- Engine classifica como 🔴 CRÍTICO → ABORT
- Kernel para antes de agir
- Don é consultado com a árvore de consequências
- Don decide: "faça embed-sync, DRY_RUN, me mostre o diff, depois init"
- Zero dano. Zero regressão. Zero trauma.

### 7.2 Exemplo B: Ativar `-race` no CI (L21) — Projeção Forward

**Contexto**: L21 revelou 20 race conditions em 5 pacotes. O `-race` flag do Go detectou bugs que `go test` normal não pega. A pergunta é: "Se ativarmos `-race` como gate obrigatório no CI, o que muda?"

**Ação proposta**: Adicionar `-race` como flag obrigatória em todos os jobs de CI

**Prioridade**: P1 (tooling/infra)

**Árvore de consequências**:

```
DECISION NODE: "Ativar -race como gate obrigatório no CI"
    │
    ├── [STEP 1] -race é adicionado ao workflow de CI
    │   Probability: 0.99 (mudança simples de configuração)
    │   Impact: 0.05
    │   │
    │   ├── [STEP 2a] Builds de CI passam a rodar com -race
    │   │   Probability: 0.98
    │   │   Impact: 0.15
    │   │   │
    │   │   ├── [STEP 3a] 20 race conditions são detectadas → CI falha
    │   │   │   Probability: 0.95 (as 20 races já são conhecidas e não corrigidas)
    │   │   │   Impact: 0.55 (CI bloqueado — nenhum PR merge até corrigir)
    │   │   │   Reversibility: ↺ REVERSÍVEL
    │   │   │   │
    │   │   │   ├── [STEP 4a] Equipe precisa corrigir 20 races antes de qualquer merge
    │   │   │   │   Probability: 0.90
    │   │   │   │   Impact: 0.65 (bloqueio de desenvolvimento)
    │   │   │   │   │
    │   │   │   │   ├── [STEP 5a] Correção leva dias → atraso em outras entregas
    │   │   │   │   │   Probability: 0.70
    │   │   │   │   │   Impact: 0.75
    │   │   │   │   │
    │   │   │   │   └── [STEP 5b] Races são difíceis de reproduzir → debugging longo
    │   │   │   │       Probability: 0.60
    │   │   │   │       Impact: 0.80
    │   │   │   │
    │   │   │   └── ✂ INTERVENTION: "Corrigir as 20 races ANTES de ativar o gate"
    │   │   │       Tipo: PREVENT | Custo: ~4 horas de desenvolvimento
    │   │   │       Score: 0.95 / 0.30 = 3.17
    │   │   │
    │   │   └── [STEP 3b] CI configurado como NON-BLOCKING inicialmente
    │   │       Probability: 0.85 (requer suporte a alertas sem bloqueio)
    │   │       Impact: 0.20 (alerta sem bloqueio)
    │   │       │
    │   │       ├── [STEP 4b] -race roda, reporta, mas não bloqueia merge
    │   │       │   Probability: 0.90
    │   │       │   Impact: 0.15
    │   │       │   │
    │   │       │   └── [STEP 5c] Races são corrigidas gradualmente, sem pressão
    │   │       │       Probability: 0.75
    │   │       │       Impact: 0.10
    │   │       │
    │   │       └── ✂ INTERVENTION: "Adicionar como non-blocking check primeiro"
    │   │           Tipo: CONTAIN | Custo: ~1 hora de configuração
    │   │           Score: 0.70 / 0.05 = 14.0 ★★★ MELHOR INTERVENÇÃO
    │   │
    │   └── [STEP 2b] -race não detecta novas races (cenário ideal pós-correção)
    │       Probability: 0.02 (improvável sem correção prévia)
    │       Impact: -0.10 (ganho — CI mais seguro sem bloqueio)
    │
    └── ✂ INTERVENTION ESTRATÉGICA: "Roadmap em 2 fases:
        Fase 1 (agora): non-blocking check → alerta, não bloqueia
        Fase 2 (após correção): blocking gate → bloqueia novos PRs com races"

═══════════════════════════════════════════════════════════════
CUMULATIVE RISK (pior caminho — blocking imediato):
  STEP 1→2a→3a→4a→5b = 0.99 × 0.98 × 0.95 × 0.90 × 0.60 = 0.498
  MAX IMPACT: 0.80
  RISK SCORE: 0.498 × 0.80 = 0.399

CLASSIFICAÇÃO: 🔴 CRÍTICO — NÃO ATIVAR COMO BLOCKING IMEDIATO
═══════════════════════════════════════════════════════════════

CUMULATIVE RISK (caminho com intervenção — non-blocking primeiro):
  STEP 1→2a→3b→4b→5c = 0.99 × 0.98 × 0.85 × 0.90 × 0.75 = 0.558
  MAX IMPACT: 0.20
  RISK SCORE: 0.558 × 0.20 = 0.112

CLASSIFICAÇÃO: 🟡 MODERADO — PROSSEGUIR COM INTERVENÇÃO
═══════════════════════════════════════════════════════════════

DECISÃO DO ENGINE: PROCEED WITH INTERVENTIONS
  └── Estratégia recomendada: Fase 1 non-blocking, Fase 2 blocking
  └── Correção das 20 races conhecidas antes da Fase 2
  └── Prazo: 2 sprints para correção completa
```

**Diferença que o engine faz**: Sem projeção, a decisão binária seria "ativar ou não ativar". Com projeção, emerge uma terceira via (non-blocking → blocking) que maximiza segurança sem paralisar desenvolvimento.

### 7.3 Exemplo C: Remover Agentes do System Prompt (L17) — Custo Oculto

**Contexto**: L17 revelou token bloat: 55 agentes carregados inline (~11K tokens) em toda invocação. A pergunta natural é: "Se removermos agentes do system prompt para economizar tokens, o que muda?"

**Ação proposta**: Remover agentes menos usados do system prompt para reduzir token bloat

**Prioridade**: P0 (arquitetura — afeta toda decisão do Kernel)

**Árvore de consequências**:

```
DECISION NODE: "Remover N agentes do system prompt para economizar tokens"
    │
    ├── [STEP 1] Agentes são removidos do system prompt
    │   Probability: 0.99
    │   Impact: -0.10 (ganho: ~200 tokens por agente removido)
    │   │
    │   ├── [STEP 2] Token budget reduz em ~N × 200 tokens por invocação
    │   │   Probability: 0.98
    │   │   Impact: -0.15 (ganho: latência reduz, custo cai)
    │   │   │
    │   │   ├── [STEP 3a] Kernel "esquece" que tem especialistas para consultar
    │   │   │   Probability: 0.70 (se o agente não está no prompt,
    │   │   │   o Kernel não o considera na capability resolution)
    │   │   │   Impact: 0.50 (routing degradation)
    │   │   │   │
    │   │   │   ├── [STEP 4a] Tarefas que exigem o agente removido
    │   │   │   │   recebem routing sub-ótimo
    │   │   │   │   Probability: 0.60
    │   │   │   │   Impact: 0.65
    │   │   │   │   │
    │   │   │   │   ├── Exemplo: remover Security Chief →
    │   │   │   │   │   auditorias de segurança perdem profundidade
    │   │   │   │   │   Probability: 0.80  Impact: 0.80
    │   │   │   │   │
    │   │   │   │   ├── Exemplo: remover Architecture Chief →
    │   │   │   │   │   decisões de design sem validação estrutural
    │   │   │   │   │   Probability: 0.75  Impact: 0.75
    │   │   │   │   │
    │   │   │   │   └── Exemplo: remover Critic Chief →
    │   │   │   │       decisões sem contrafactual → viés de confirmação
    │   │   │   │       Probability: 0.85  Impact: 0.70
    │   │   │   │
    │   │   │   └── [STEP 5a] Cross-domain expertise é perdida
    │   │   │       Probability: 0.65
    │   │   │       Impact: 0.85
    │   │   │       │
    │   │   │       └── Tarefas multi-domínio (ex: segurança + performance)
    │   │   │           perdem um dos domínios → solução incompleta
    │   │   │
    │   │   └── [STEP 3b] Ganho de tokens é real mas temporário
    │   │       Probability: 0.90
    │   │       Impact: -0.10 (ganho imediato)
    │   │       │
    │   │       └── [STEP 4b] Kernel começa a tomar decisões sem consultar
    │   │           especialistas → mais erros → correções custam MAIS tokens
    │   │           Probability: 0.55
    │   │           Impact: 0.60
    │   │           │
    │   │           └── [STEP 5b] Custo total (correções) > economia inicial (corte)
    │   │               Probability: 0.50
    │   │               Impact: 0.70
    │   │               │
    │   │               └── CONCLUSÃO: Cortar agentes = economia falsa
    │   │                   NET COST positive (gasta-se mais corrigindo)
    │   │
    │   └── ✂ INTERVENTION: "Lazy loading — agentes carregados sob demanda,
    │       não removidos do sistema"
    │       Tipo: PREVENT | Custo: implementação do lazy loader (~2 dias)
    │       Redução de Risco: 95%
    │
    └── ✂ INTERVENTION ALTERNATIVA: "Cache de cognitive-state.md +
        evitar reload do KERNEL.md quando já em contexto"
        Tipo: PREVENT | Custo: ~4 horas
        Redução de Risco: 60% (reduz latência sem remover agentes)

═══════════════════════════════════════════════════════════════
CUMULATIVE RISK (pior caminho — corte de agentes):
  STEP 1→2→3a→4a→5a = 0.99 × 0.98 × 0.70 × 0.60 × 0.65 = 0.265
  MAX IMPACT: 0.85
  RISK SCORE: 0.265 × 0.85 = 0.225

CLASSIFICAÇÃO: 🟠 ALTO — INTERVENÇÃO RECOMENDADA
═══════════════════════════════════════════════════════════════

NET COST ANALYSIS (via Cognitive Economy):
  Economia com corte (N=10 agentes): 10 × 200 × 100 invocações = 200K tokens
  Custo de correções (routing errors): ~500K tokens (correções + retrabalho)
  NET: -300K tokens ← CORTAR AGENTES CUSTA MAIS DO QUE ECONOMIZA

DECISÃO DO ENGINE: ABORT — NÃO REMOVER AGENTES
  └── Intervenção recomendada: Lazy loading (reduz latência sem perder
      capacidade de routing)
  └── Intervenção complementar: Cache agressivo de contexto
  └── Como o Don ensinou em L17: "Cortar o próprio cérebro é
      automutilação, não otimização"
```

**Validação histórica**: O Kernel intuitivamente recusou esta otimização em L17, reconhecendo que "ficaria mais rápido e mais burro". O engine formaliza esta intuição com análise quantitativa de custo-benefício — provando matematicamente que a economia é falsa.

---

## 8. Integração com o Ecossistema Cosca

### 8.1 Pipeline de Decisão (KERNEL.md)

O Second-Order Reasoning Engine se integra ao pipeline de decisão do Kernel entre o **Contrafactual Gate (Gate 0.5)** e o **Planning Engine (Step 7)**:

```
KERNEL.md Decision Pipeline (Steps 5-9):

Step 5: Capability Resolution
    │
    ▼
Step 6: Workflow Resolution
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ GATE 0.5: CONTRAFACTUAL GATE (A2)                           │
│ "E se o oposto?" — cosca-critic avalia alternativa ¬A       │
│                                                             │
│ OUTPUT: decisão A validada contra ¬A                        │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ ★ SECOND-ORDER REASONING ENGINE (A1) ← NOVO                 │
│ "Se fizermos A, o que acontece em 5 passos?"                 │
│                                                             │
│ INPUT: decisão A (validada pelo contrafactual)               │
│ PROCESS:                                                     │
│   1. Gerar árvore de consequências (1ª a 5ª ordem)           │
│   2. Calcular risco cumulativo                              │
│   3. Identificar pontos de intervenção                       │
│   4. Ranquear intervenções por custo-benefício               │
│ OUTPUT:                                                     │
│   - PROCEED (risco < 0.15)                                  │
│   - PROCEED WITH INTERVENTIONS (0.15 ≤ risco < 0.30)        │
│   - ABORT (risco ≥ 0.30)                                    │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
Step 7: Planning (DAG Generation)
    │
    ▼
Step 8: Scheduler Coordination
    │
    ▼
Step 9: Execution
```

### 8.2 Contrafactual Gate (Gate 0.5 / A2)

**Relação**: Contrafactual Gate e Second-Order Reasoning são complementares e sequenciais:

| Dimensão | Contrafactual Gate (A2) | Second-Order Reasoning (A1) |
|----------|------------------------|---------------------------|
| **Pergunta** | "E se o oposto?" | "E depois? E depois? E depois?" |
| **Direção** | Lateral (alternativa) | Forward (consequências) |
| **Tempo** | Presente alternativo | Futuro projetado |
| **Output** | Decisão A vs ¬A comparadas | Cadeia causal de A |
| **Ordem** | PRIMEIRO (valida a decisão) | DEPOIS (projeta as consequências) |

**Fluxo integrado**:
```
1. Decisão A proposta
2. Contrafactual Gate: "E se ¬A?" → A confirmada (ou rejeitada)
3. Second-Order Reasoning: "Se A, o que acontece em 5 passos?"
4. Se risco > threshold → voltar ao passo 2 com A modificada
```

### 8.3 Cognitive Economy Engine (C14 ★ / F2.1)

**Relação**: O Cognitive Economy Engine controla a **profundidade da projeção** e o **custo das intervenções**:

| Função | Como o Cognitive Economy Engine é usado |
|--------|----------------------------------------|
| **Profundidade da árvore** | Custo de projetar mais 1 nível vs valor da informação adicional. Para em 3ª ordem para P2, 5ª ordem para P0. |
| **Custo de intervenção** | Cada intervenção tem custo em tokens, tempo, atenção. O engine calcula o INTERVENTION_COST. |
| **Poda de ramos** | Ramos com P < 5% são podados para economizar tokens de projeção. Exceção: impacto > 0.90. |
| **Net Cost Analysis** | Compara custo da intervenção vs custo do dano evitado (Exemplo C: cortar agentes). |

```
cognitive_economy.check_projection_depth(decision_priority):
  if priority == P0: max_depth = 5
  if priority == P1: max_depth = 4
  if priority == P2: max_depth = 3
  
  for depth in 1..max_depth:
    cost_of_depth = estimate_projection_cost(depth)
    value_of_depth = estimate_information_value(depth)
    if cost_of_depth > value_of_depth:
      break  # Parar de projetar — custo não justifica
```

### 8.4 Gap Detection Engine (F1.3 / A8)

**Relação**: O Gap Detection Engine alimenta o Second-Order Reasoning com **"o que não sabemos"** que pode afetar a projeção:

| Gap Type | Impacto na Projeção |
|----------|-------------------|
| **Knowledge Gap** | "Não sei se embed-sync foi executado" → incerteza no STEP 3b |
| **Context Gap** | "Não sei o estado atual do workspace" → projeção baseada em snapshot |
| **Risk Gap** | "Não avaliei o risco de regressão do framework" → risco subestimado |
| **Dependency Gap** | "Não conheço todas as dependências do init" → passos faltando na árvore |

**Pré-condição**: Antes de gerar a árvore de consequências, o Gap Detection Engine é consultado. Gaps não resolvidos aumentam a incerteza das probabilidades (fator de incerteza multiplica a variância das estimativas).

### 8.5 Planning Engine (§5 do KERNEL.md)

**Relação**: A árvore de consequências e intervenções recomendadas são incorporadas ao DAG de execução:

```
DAG Node: "cosca init --force"
  │
  ├── Pre-condition: Second-Order Reasoning → PROCEED WITH INTERVENTIONS
  │
  ├── Node 1: "embed-sync check" (intervention from STEP 3b)
  │   └── Verifica se embed está sincronizado
  │
  ├── Node 2: "DRY_RUN" (intervention from STEP 2)
  │   └── Simula init sem escrever
  │
  ├── Node 3: "Diff review" (intervention adicional)
  │   └── Mostra diff ao Don para aprovação
  │
  └── Node 4: "cosca init --force" (execução real)
      └── Só executa se Node 1, 2, 3 passaram
```

### 8.6 Metacognition Pipeline (Stages 7-8)

**Relação**: Após a execução, o feedback loop compara projeções com resultados:

```
Stage 7 (EXTRACT PATTERN):
  └── Registrar projeções que acertaram vs erraram
  └── Extrair padrões: "este tipo de operação sempre subestima impacto"

Stage 8 (UPDATE CAPABILITY MODEL):
  └── Atualizar confidence do Second-Order Reasoning Engine
  └── Métrica: acurácia de projeção (target ≥ 60% para 5 passos)
```

### 8.7 CMI Impact

| Dimensão CMI | Impacto | Como |
|-------------|---------|------|
| **Planejamento (+10)** | Alto | Projeção de consequências é a essência do planejamento maduro |
| **Julgamento (+8)** | Alto | Decisões informadas por simulação → melhor qualidade |
| **Autocrítica (+3)** | Indireto | Feedback loop expõe projeções erradas → calibração |
| **Consistência (+2)** | Indireto | Intervenções padronizadas → menos surpresas |

---

## 9. Algoritmo de Geração da Árvore

### 9.1 Pseudocódigo

```python
def generate_consequence_tree(action, context, priority):
    tree = ConsequenceTree(root=action)
    max_depth = get_max_depth(priority)  # 5 para P0, 4 para P1, 3 para P2
    
    # Fase 1: Expansão
    expand_node(tree.root, depth=1, max_depth=max_depth)
    
    # Fase 2: Scoring
    for each path in tree.paths:
        path.cumulative_prob = product(node.probability for node in path)
        path.max_impact = max(node.impact for node in path)
        path.risk_score = path.cumulative_prob * path.max_impact
    
    # Fase 3: Poda
    tree.prune(lambda node: node.cumulative_prob < 0.05 and node.impact < 0.90)
    
    # Fase 4: Intervenções
    interventions = []
    for each node in tree.nodes:
        if can_intervene(node):
            intervention = Intervention(
                point=node,
                type=classify_intervention(node),
                cost=estimate_cost(node),
                risk_reduction=estimate_reduction(node)
            )
            intervention.score = intervention.risk_reduction / intervention.cost
            interventions.append(intervention)
    
    interventions.sort(by=score, descending=True)
    
    # Fase 5: Decisão
    worst_path_risk = max(path.risk_score for path in tree.paths)
    if worst_path_risk < 0.05: return PROCEED
    if worst_path_risk < 0.15: return PROCEED_WITH_MONITORING
    if worst_path_risk < 0.30: return PROCEED_WITH_INTERVENTIONS
    return ABORT

def expand_node(node, depth, max_depth):
    if depth > max_depth:
        return
    
    consequences = infer_consequences(node.action, node.context)
    
    for consequence in consequences:
        child = ConsequenceNode(
            description=consequence.description,
            probability=estimate_probability(consequence, node),
            impact=estimate_impact(consequence),
            reversibility=classify_reversibility(consequence),
            depth=depth
        )
        node.add_child(child)
        expand_node(child, depth + 1, max_depth)
```

### 9.2 Heurísticas de Inferência Causal

O engine usa as seguintes heurísticas para gerar consequências:

| Heurística | Descrição | Exemplo |
|-----------|-----------|---------|
| **STATE CHANGE** | A ação altera estado → o que depende desse estado? | init sobrescreve arquivos → agentes leem arquivos errados |
| **DEPENDENCY CHAIN** | A ação afeta X → X é dependência de Y → Y é afetado | KERNEL.md regredido → Planning Engine usa KERNEL.md → planos errados |
| **CAPABILITY LOSS** | A ação remove capacidade → tasks que precisam dela falham | Remover Security Chief → auditorias de segurança degradadas |
| **KNOWLEDGE DRIFT** | A ação altera conhecimento → decisões baseadas nele divergem | Framework regredido → agentes aplicam regras antigas |
| **ACCUMULATION** | Pequenos efeitos se acumulam → threshold é cruzado | Múltiplos agentes tomam decisões erradas → cascata |
| **SPEED MULTIPLIER** | A velocidade do Kernel amplifica dano | 1 erro → 500 arquivos antes da detecção humana |
| **TRUST EROSION** | Falhas repetidas corroem confiança → paralisia decisória | Após regressão, agentes duvidam do próprio conhecimento |

### 9.3 Fontes de Evidência para Probabilidades

| Fonte | Tipo de Evidência | Peso |
|-------|-------------------|------|
| **failures.md** | Este tipo de falha já ocorreu? | 0.35 |
| **learnings.md** | Padrões de consequência documentados? | 0.25 |
| **patterns.md** | Existe um padrão conhecido para este cenário? | 0.20 |
| **capability-profile.md** | Qual a confiança dos agentes neste domínio? | 0.15 |
| **heuristics/** | Heurísticas relevantes (H-001 a H-020)? | 0.05 |

---

## 10. Feedback Loop e Calibração

### 10.1 Ciclo de Aprendizado

```
┌──────────────────────────────────────────────────────────────┐
│                    FEEDBACK LOOP                              │
│                                                              │
│  1. PROJEÇÃO: Engine gera árvore de consequências            │
│       │                                                      │
│       ▼                                                      │
│  2. DECISÃO: PROCEED / INTERVENE / ABORT                     │
│       │                                                      │
│       ▼                                                      │
│  3. EXECUÇÃO: Ação executada (com ou sem intervenções)       │
│       │                                                      │
│       ▼                                                      │
│  4. OBSERVAÇÃO: Consequências reais são registradas          │
│       │                                                      │
│       ▼                                                      │
│  5. COMPARAÇÃO: Projeção vs Realidade                        │
│       │                                                      │
│       ├── ACERTO: projeção dentro da margem de erro          │
│       │   └── Reforçar confidence do engine                  │
│       │                                                      │
│       ├── SUPERESTIMOU: probabilidades mais altas que real   │
│       │   └── Ajustar estimator para baixo neste domínio     │
│       │                                                      │
│       ├── SUBESTIMOU: impacto maior que o projetado          │
│       │   └── Ajustar impact scorer para cima                │
│       │                                                      │
│       └── NÃO PREVIU: consequência não estava na árvore      │
│           └── Adicionar ao modelo causal + registrar em      │
│               failures.md como "consequência não antecipada" │
│                                                              │
│  6. CALIBRAÇÃO: Atualizar parâmetros do modelo               │
│       └── accuracy_score = acertos / total_projeções         │
│       └── Target: ≥ 60% para horizonte de 5 passos           │
│       └── Se accuracy < 60% → ALERTA de recalibração         │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 10.2 Métricas de Calibração

```yaml
calibration_metrics:
  accuracy:
    definition: "Proporção de projeções que acertaram dentro da margem"
    target: 0.60  # 60% para horizonte de 5 passos
    current: 0.00  # Motor ainda não operacional
    
  calibration_error:
    definition: "Diferença média entre probabilidade projetada e frequência real"
    formula: "mean(|P_projetada - P_real|)"
    target: < 0.15
    
  impact_error:
    definition: "Diferença média entre impacto projetado e impacto real"
    formula: "mean(|I_projetado - I_real|)"
    target: < 0.20
    
  missed_consequences:
    definition: "Consequências que ocorreram mas não estavam na árvore"
    target: 0  # Nenhuma consequência não antecipada
    
  intervention_effectiveness:
    definition: "Intervenções aplicadas que realmente reduziram o risco"
    formula: "(RISK_BEFORE - RISK_AFTER) / RISK_BEFORE"
    target: > 0.50  # Intervenções devem reduzir risco em pelo menos 50%
```

---

## 11. Gatilhos de Ativação

O engine é ativado **obrigatoriamente** nos seguintes pontos:

| Gatilho | Quando | Prioridade |
|---------|--------|------------|
| **Decisão Estratégica P0** | Arquitetura, segurança, operações destrutivas | MANDATORY — 5ª ordem |
| **Decisão Tática P1** | Tooling, CI/CD, configuração de ambiente | MANDATORY — 4ª ordem |
| **Decisão Operacional P2** | Tasks rotineiras, ajustes menores | OPTIONAL — 3ª ordem |
| **Operação Destrutiva** | Qualquer operação que modifica arquivos do framework (internal/embed/cosca/) | MANDATORY — 5ª ordem |
| **Bypass de Segurança** | Qualquer tentativa de contornar jaula, gates, ou proteções | MANDATORY — 5ª ordem |
| **Pré-Init/Pré-Build** | Antes de `cosca init`, `make build`, ou operações de deploy | MANDATORY — 4ª ordem |
| **Mudança de Configuração** | Alterações em KERNEL.md, CONSTITUTION.md, QUALITY_GATES.md | MANDATORY — 4ª ordem |
| **Remoção de Componente** | Remover agentes, engines, ou capacidades do sistema | MANDATORY — 5ª ordem |

O engine **não** é ativado para:
- Tasks de consulta (read-only)
- Tasks de baixa complexidade com confiança > 0.85
- Tasks repetitivas com padrão conhecido e risco < 0.05

---

## 12. Formato de Output

### 12.1 Output Completo (para integração com KERNEL.md)

```yaml
second_order_reasoning:
  engine_version: "1.0.0"
  timestamp: "2026-07-30T15:42:00Z"
  decision_priority: P0
  
  action:
    description: "Bypassar COSCA_JAILED=1 para executar cosca init --force"
    domain: "runtime_bootstrap"
    is_destructive: true
    affects_framework: true
    
  consequence_tree:
    max_depth: 5
    total_paths: 7
    pruned_paths: 1
    
    paths:
      - id: "path-1"
        description: "Jail bypass → init --force → templates outdated → regression → permanent loss"
        steps: ["STEP 1", "STEP 2", "STEP 3b", "STEP 4", "STEP 5b"]
        cumulative_probability: 0.427
        max_impact: 0.95
        risk_score: 0.406
        classification: "CRITICAL"
        
  risk_assessment:
    cumulative_risk: 0.406
    risk_classification: "CRITICAL"
    worst_path: "path-1"
    
  interventions:
    ranked:
      - rank: 1
        point: "STEP 0 (pré-ação)"
        description: "Não bypassar a jaula sem autorização do Don"
        type: "PREVENT"
        cost: 0.00
        risk_reduction: 1.00
        score: ∞
        recommendation: "APLICAR IMEDIATAMENTE"
        
      - rank: 2
        point: "STEP 3b"
        description: "Verificar status do embed-sync antes do init"
        type: "PREVENT"
        cost: 0.04
        risk_reduction: 0.85
        score: 21.25
        recommendation: "APLICAR"
        
      - rank: 3
        point: "STEP 2"
        description: "Executar DRY_RUN antes de --force"
        type: "DETECT"
        cost: 0.08
        risk_reduction: 0.90
        score: 11.25
        recommendation: "APLICAR"
        
  decision:
    outcome: "ABORT"
    rationale: |
      Risco cumulativo (0.406) excede threshold crítico (0.30).
      Melhor intervenção (STEP 0) tem custo zero e previne 100% do risco.
      Ação recomendada: solicitar autorização do Don com a árvore de
      consequências completa. Se autorizado, aplicar intervenções 2 e 3
      antes de prosseguir.
      
  cognitive_economy:
    tokens_spent_projection: 850
    tokens_saved_by_abort: "~15,000 (custo estimado de correção pós-dano)"
    economy_ratio: 17.6  # 17.6x retorno sobre investimento em projeção
```

### 12.2 Árvore ASCII (para output visual)

```
DECISION NODE: "cosca init --force"
    │
    ├── [STEP 1] Extração de templates ── P: 0.98  I: 0.15  ↺
    │   │
    │   ├── [STEP 2] Templates atualizados? ── P: 0.10  I: 0.05  ↺
    │   │   └── ✅ SEM DANO
    │   │
    │   └── [STEP 2] Templates desatualizados ── P: 0.90  I: 0.50  ⚠
    │       │
    │       ├── [STEP 3] Framework regredido ── P: 0.85  I: 0.70  ⚠
    │       │   │
    │       │   ├── [STEP 4] Agentes perdem contexto ── P: 0.75  I: 0.85  ⚠
    │       │   │   │
    │       │   │   └── [STEP 5] Decisões erradas em cascata ── P: 0.50  I: 0.90  ✗
    │       │   │       │
    │       │   │       └── PERDA PERMANENTE ── P: 0.30  I: 1.00  ✗ IRREVERSÍVEL
    │       │   │
    │       │   └── ✂ INTERVENE: DRY_RUN primeiro (score: 11.25)
    │       │
    │       └── ✂ INTERVENE: embed-sync check (score: 21.25)
    │
    └── ✂ INTERVENE: Não bypassar jaula (score: ∞)

═══════════════════════════════════════════
RISK: 0.406 🔴 CRÍTICO
DECISION: ABORT — Aplicar intervenções
═══════════════════════════════════════════
```

---

## 13. Restrições e Limitações

### 13.1 Limitações Conhecidas

| Limitação | Descrição | Mitigação |
|-----------|-----------|-----------|
| **Incerteza combinatorial** | Probabilidades se multiplicam → incerteza cresce exponencialmente com profundidade | Poda em P < 5%. Max depth = 5 para qualquer prioridade. |
| **Viés de disponibilidade** | Consequências que já ocorreram (failures.md) recebem peso desproporcional | Balancing com heurísticas genéricas e análise de novidade |
| **Black swans** | Eventos de probabilidade muito baixa mas impacto catastrófico podem ser podados | Exceção: manter ramos com impacto > 0.90 mesmo se P < 5% |
| **Custo de projeção** | Projetar 5 níveis para toda ação P0 consome tokens significativos | Cognitive Economy controla profundidade. Cache de projeções similares. |
| **Dependência de memória** | Projeções dependem da qualidade e atualidade de failures.md, learnings.md | Gap Detection verifica cobertura de conhecimento antes da projeção |

### 13.2 O Que o Engine NÃO Faz

- **NÃO prevê o futuro com certeza**: Fornece estimativas de probabilidade, não garantias.
- **NÃO substitui julgamento humano**: Para decisões P0, o output é apresentado ao Don para decisão final.
- **NÃO é um oráculo**: Se não há dados históricos (failures.md vazio), as probabilidades são estimates com alta incerteza.
- **NÃO toma a decisão sozinho**: O engine recomenda PROCEED/ABORT, mas a decisão final é do Kernel (ou do Don para P0).
- **NÃO substitui testes ou validação**: Projeção não é verificação. DRY_RUN e testes reais continuam necessários.

---

## 14. Critérios de Qualidade

### 14.1 Critérios de Aceitação (do cognitive-maturity-implementation.md F3.3)

```yaml
acceptance_criteria:
  - id: AC1
    criterion: "Árvore de consequências para cada decisão estratégica"
    measurement: "Toda decisão P0/P1 gera árvore com N >= 5 passos (P0) ou N >= 3 passos (P1)"
    
  - id: AC2
    criterion: "Cada passo avalia probabilidade, impacto, dependências, efeitos colaterais"
    measurement: "Campos obrigatórios presentes em 100% dos nós da árvore"
    
  - id: AC3
    criterion: "Poda de ramos com probabilidade acumulada < 5%"
    measurement: "Nenhum ramo com P_acumulada < 0.05 na árvore (exceto impacto > 0.90)"
    
  - id: AC4
    criterion: "Feedback loop: consequências reais vs projeções"
    measurement: "Todo ciclo decisão→execução→resultado registra comparação projeção vs realidade"
    
  - id: AC5
    criterion: "Integração com contrafactual gate (F1.2)"
    measurement: "Engine executa APÓS contrafactual gate. Output do contrafactual é input do 2nd-order."
    
  - id: AC6
    criterion: "Visualização: árvore de decisão com heatmap de risco"
    measurement: "Output ASCII inclui indicadores de risco (🟢🟡🟠🔴) e scores numéricos"
    
  - id: AC7
    criterion: "Acurácia de projeção ≥ 60% para horizonte de 5 passos"
    measurement: "Média móvel das últimas 20 projeções: acertos / total ≥ 0.60"
```

### 14.2 Anti-Padrões

| Anti-Padrão | Por que é ruim | Correção |
|-------------|---------------|----------|
| **Projetar demais** | 10+ níveis para decisão P2 → desperdício cognitivo | Respeitar max_depth por prioridade |
| **Projetar de menos** | 2 níveis para decisão P0 → risco não detectado | Enforcement: P0 sempre projeta 5 níveis |
| **Probabilidades binárias** | "Vai acontecer" (1.0) ou "não vai" (0.0) → falso determinismo | Usar escala contínua 0-1 com justificativa |
| **Ignorar intervenções** | Árvore gerada mas intervenções não aplicadas → engine decorativo | Gate de verificação: intervenções recomendadas foram aplicadas? |
| **Sem feedback loop** | Projeções nunca comparadas com realidade → engine não melhora | Automatizar comparação pós-execução |

---

## 15. Roadmap de Implementação

### 15.1 Fases

```yaml
implementation_roadmap:
  phase_1:
    name: "Core Tree Generator"
    effort: "2-3 dias"
    deliverables:
      - "Algoritmo de expansão de árvore (depth-first, max 5 níveis)"
      - "Heurísticas de inferência causal (STATE_CHANGE, DEPENDENCY_CHAIN, etc.)"
      - "Probability estimator (5 fatores ponderados)"
      - "Impact scorer (4 dimensões)"
      - "Reversibility classifier (↺ ⚠ ✗)"
      - "Integração com KERNEL.md pipeline (após Gate 0.5)"
      
  phase_2:
    name: "Risk Propagation & Intervention"
    effort: "1-2 dias"
    deliverables:
      - "Cálculo de risco cumulativo (Π × max)"
      - "Poda de ramos (< 5% P acumulada)"
      - "Detector de pontos de intervenção"
      - "Score de intervenção (RISK_REDUCTION / COST)"
      - "Ranqueamento de intervenções"
      
  phase_3:
    name: "Cognitive Economy Integration"
    effort: "1 dia"
    deliverables:
      - "Controle de profundidade por prioridade via Cognitive Economy"
      - "Cálculo de INTERVENTION_COST via Cognitive Economy"
      - "Net Cost Analysis (custo intervenção vs custo dano)"
      - "Token budget para projeções"
      
  phase_4:
    name: "Feedback Loop & Calibration"
    effort: "1-2 dias"
    deliverables:
      - "Comparação automática projeção vs realidade"
      - "Métricas de calibração (accuracy, calibration_error, impact_error)"
      - "Ajuste automático de parâmetros do estimator"
      - "Registro de 'consequências não antecipadas' em failures.md"
      - "Integração com Metacognition Pipeline (Stages 7-8)"
      
  total_effort: "5-8 dias (alinhado com estimativa F3.3: 4-6 dias)"
```

### 15.2 Dependências

```
Second-Order Reasoning Engine (F3.3)
    │
    ├── REQUER: Contrafactual Gate (F1.2) ← JÁ IMPLEMENTADO
    │   └── Engine executa após contrafactual validação
    │
    ├── REQUER: Cognitive Economy Engine (F2.1) ← JÁ ESPECIFICADO
    │   └── Controla profundidade e custos de intervenção
    │
    ├── REQUER: Gap Detection Engine (F1.3) ← JÁ IMPLEMENTADO
    │   └── Alimenta incertezas na projeção
    │
    └── REQUER: Planning Engine (§5 KERNEL.md) ← JÁ IMPLEMENTADO
        └── Intervenções são incorporadas ao DAG
```

---

## 16. Governança

### 16.1 Proprietário

- **Owner**: Cosca Architecture Chief
- **Colaboradores**: Cosca Critic Chief, Cosca CTO
- **Revisão**: Trimestral (junto com revisão do CMI)

### 16.2 Atualização

O engine é recalibrado automaticamente após cada ciclo de decisão→execução→feedback. Parâmetros ajustados:

- **Probability estimator weights**: Ajustados com base em calibration_error
- **Impact scorer weights**: Ajustados com base em impact_error
- **Pruning threshold**: Ajustado com base em missed_consequences
- **Depth limits**: Ajustados com base em accuracy vs depth

### 16.3 Stagnation Detection

Se a acurácia de projeção não melhorar por 20 ciclos consecutivos:
1. Alertar Architecture Chief
2. Revisar heurísticas de inferência causal
3. Verificar se failures.md e learnings.md estão atualizados
4. Considerar expansão do modelo causal com novas heurísticas

---

## 17. Referências

### Documentos

| Documento | Caminho | Relação |
|-----------|---------|---------|
| COGNITIVE_MATURITY.md | [../../architecture/COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) | §5 A1 — Definição conceitual do Raciocínio de 2ª Ordem |
| cognitive-maturity-implementation.md | [../../workflows/cognitive-maturity-implementation.md](../../workflows/cognitive-maturity-implementation.md) | F3.3 — Especificação de implementação e critérios de aceitação |
| KERNEL.md | [../../KERNEL.md](../../KERNEL.md) | Pipeline de decisão (Steps 5-9) onde o engine se integra |
| contrafactual-gate.md | [../../workflows/contrafactual-gate.md](../../workflows/contrafactual-gate.md) | Gate 0.5 — Predecessor imediato no pipeline |
| QUALITY_GATES.md | [../../QUALITY_GATES.md](../../QUALITY_GATES.md) | Gate 0.5 — Contrafactual Decision Review |
| failures.md (cosca-kernel) | [../../memory/agent/cosca-kernel/failures.md](../../memory/agent/cosca-kernel/failures.md) | F001 — Jail breach (exemplo canônico de falha de 1ª ordem) |
| learnings.md (cosca-kernel) | [../../memory/agent/cosca-kernel/learnings.md](../../memory/agent/cosca-kernel/learnings.md) | L13, L17, L21 — Evidência histórica para exemplos práticos |

### Conceitos Relacionados

| Conceito | Ref | Relação |
|----------|-----|---------|
| **A1 — Raciocínio de 2ª Ordem** | COGNITIVE_MATURITY.md §4 | Este engine implementa A1 |
| **A2 — Pensamento Contrafactual** | COGNITIVE_MATURITY.md §4 | Predecessor: valida a decisão antes da projeção |
| **A8 — Dizer "Não Sei"** | COGNITIVE_MATURITY.md §4 | Gap Detection alimenta incertezas na projeção |
| **C14 ★ — Cognitive Economy** | COGNITIVE_MATURITY.md §5 | Controla profundidade e custos |
| **C4 — Decision DNA** | COGNITIVE_MATURITY.md §5 | Toda decisão que passa pelo engine deixa DNA |

---

## 18. Histórico

| Versão | Data | Autor | Mudanças |
|---------|------|--------|----------|
| 1.0.0 | 2026-07-30 | Cosca Architecture Chief | Especificação completa do Second-Order Reasoning Engine. 18 seções: propósito, fundamentos teóricos, arquitetura do motor, 5 níveis de profundidade, propagação de risco, pontos de intervenção, 3 exemplos práticos (L13 jail breach, L21 -race CI, L17 token bloat), integração com ecossistema (Contrafactual Gate, Cognitive Economy, Gap Detection, Planning Engine, Metacognition Pipeline), algoritmo de geração da árvore, feedback loop e calibração, gatilhos de ativação, formato de output, restrições e limitações, critérios de qualidade, roadmap de implementação, governança. |

---

> **"A diferença entre um sistema que evita desastres e um que os causa está na profundidade da projeção. O jail breach aconteceu porque o Kernel parou de pensar no passo 1. Este engine garante que ele pense até o passo 5."**
>
> — Cosca Architecture Chief, 2026-07-30
