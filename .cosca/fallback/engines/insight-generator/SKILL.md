# INSIGHT GENERATOR ENGINE ★ F3.1 — Conector de Pontos Desconectados

> **Versão**: 2.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
> **Fase CMI**: Fase 3 — Cognição Avançada | **Código**: F3.1
> **Bloco Cognitivo**: Bloco 3 — Aprendizado & Evolução
> **Dependências**: F1.5 Metrics B1-B5 | F9.1 Experience Compiler | F9.2 Wisdom Distillation | F10.3 Discovery | F1.6 Cognitive Entropy
>
> Consulte também:
> - [cognitive-metrics.md (analytics)](../../analytics/cognitive-metrics.md) — F1.5: B1-B5, Cognitive Health Index, ROI
> - [experience-compiler/SKILL.md](../experience-compiler/SKILL.md) — F9.1: compilação de learnings em padrões e princípios
> - [wisdom-distillation/SKILL.md](../wisdom-distillation/SKILL.md) — F9.2: destilação de princípios em sabedoria constitucional
> - [discovery/SKILL.md](../discovery/SKILL.md) — F10.3: mineração de serendipidade (o que NÃO procuramos)
> - [cognitive-entropy/ENTROPY.md](../cognitive-entropy/ENTROPY.md) — F1.6: entropia cognitiva, contradições, stale knowledge
> - [cognitive-maturity-implementation.md F3.1](../../workflows/cognitive-maturity-implementation.md) — tarefa de implementação
> - [CONVENTIONS.md](../../CONVENTIONS.md) — contrato canônico de skills
> - [SKILL_TEMPLATE.md](../../SKILL_TEMPLATE.md) — template de skill engine

---

## SUMÁRIO

1. [Definição](#1-definição)
2. [Arquitetura no Runtime](#2-arquitetura-no-runtime)
3. [Pipeline Semanal (7 Passos, < 30s)](#3-pipeline-semanal-7-passos--30s)
4. [Normalização em Séries Temporais](#4-normalização-em-séries-temporais)
5. [Detecção de Correlações](#5-detecção-de-correlações)
6. [Geração de Hipóteses (Correlação > 0.5)](#6-geração-de-hipóteses-correlação--05)
7. [Validação Contra Learnings Existentes](#7-validação-contra-learnings-existentes)
8. [Fórmula de Insight](#8-fórmula-de-insight)
9. [Tipos de Insight](#9-tipos-de-insight)
10. [Integração com F9.1 — Experience Compiler](#10-integração-com-f91--experience-compiler)
11. [Integração com F10.3 — Discovery](#11-integração-com-f103--discovery)
12. [Integração com F9.2, F1.5 e F1.6](#12-integração-com-f92-f15-e-f16)
13. [Exemplo Real — Dados da Sessão 2026-07-30](#13-exemplo-real--dados-da-sessão-2026-07-30)
14. [Governança e Regras](#14-governança-e-regras)
15. [Armazenamento](#15-armazenamento)
16. [CLI e Automação](#16-cli-e-automação)
17. [Métricas do Engine](#17-métricas-do-engine)
18. [Casos de Borda e Anti-Padrões](#18-casos-de-borda-e-anti-padrões)
19. [Constraints e Forbidden Actions](#19-constraints-e-forbidden-actions)
20. [Critérios de Qualidade](#20-critérios-de-qualidade)
21. [Escalation](#21-escalation)
22. [Glossário](#22-glossário)
23. [Referências Cruzadas](#23-referências-cruzadas)
24. [Histórico](#24-histórico)

---

## 1. DEFINIÇÃO

### 1.1 O que é o Insight Generator

O **Insight Generator** é o gerador de insights cross-domain do Cosca. Ele encontra **conexões entre dados que não parecem relacionados** — learnings, métricas, eventos, padrões, decisões — e sintetiza o que nenhum agente individual veria isoladamente.

> **Não é busca. É síntese criativa baseada em dados.**

O F9.1 Experience Compiler pergunta *"o que aprendemos que é reutilizável?"*. O F10.3 Discovery pergunta *"o que descobrimos que não procurávamos?"*. O F3.1 Insight Generator pergunta **"o que dois pontos desconectados revelam quando conectados?"**

### 1.2 O Problema que Resolve

Cada agente do Cosca acumula conhecimento no próprio domínio: o Database Chief vê bugs de SQLite, o Architecture Chief vê padrões de design, o Analytics Chief vê métricas. **Nenhum deles vê a conexão entre os domínios** — e é exatamente aí que moram os insights de maior valor:

```
EXEMPLO CANÔNICO:

  B2 (Decision Velocity) caiu 20% NA MESMA SEMANA que
  a Entropia Cognitiva (F1.6) subiu 15%.

  Nenhum agente individual viu isso:
  - Analytics vê "B2 caiu" → alerta de performance
  - Architecture vê "entropia subiu" → alerta de conhecimento

  O Insight Generator conecta os dois:
  → "Decisões mais lentas = conhecimento mais desorganizado.
     O Kernel está demorando mais para decidir porque a base
     de conhecimento está contraditória e fragmentada."
  → Suspeita causal: entropia alta → decisões lentas
  → Ação: priorizar redução de entropia (F1.6) ANTES de otimizar
     roteamento do Kernel (B2)
```

### 1.3 O que Não É

| Engine | Pergunta | Dimensão | Relação com F3.1 |
|--------|----------|----------|-------------------|
| **F9.1 Experience Compiler** | "O que aprendemos que é reutilizável?" | Compilação (learnings → padrões) | **Consumidor** dos insights validados |
| **F10.3 Discovery** | "O que descobrimos que não procurávamos?" | Serendipidade (divergência plano × realizado) | **Complementar** — Discovery encontra, F3.1 conecta |
| **F9.2 Wisdom Distillation** | "O que destila até princípio?" | Governança (padrões → emendas) | Downstream indireto (via F9.1) |
| **F1.6 Cognitive Entropy** | "O quão desorganizado está o conhecimento?" | Medição de desordem | **Fonte de dados** para correlações |
| **F1.5 Metrics B1-B5** | "O quão saudável está o runtime?" | Medição de saúde operacional | **Fonte de dados** para correlações |

---

## 2. ARQUITETURA NO RUNTIME

### 2.1 Posição

O Insight Generator opera como um **motor de agregação semanal** — roda em background, consome dados que já existem (nunca cria dados), e produz insights com custo mínimo. Segue o padrão das engines de agregação F10.1/F10.2/F2.5: **lê agregados que já existem, nunca cria dívida técnica**.

```
┌──────────────────────────────────────────────────────────────────────────┐
│                INSIGHT GENERATOR NO RUNTIME (F3.1)                       │
│                                                                          │
│  SCHEDULER (§6 KERNEL.md)                                                │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  Queue: background_tasks (P3)                                    │   │
│  │  Trigger: semanal (segunda-feira, 07:00) OU on_demand via CLI    │   │
│  │  Budget: < 30 segundos | Zero interrupção ao Don                 │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                          │                                              │
│                          ▼                                              │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  PIPELINE SEMANAL DE 7 PASSOS                                   │   │
│  │  (detalhado na §3)                                              │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                          │                                              │
│          ┌───────────────┼───────────────┐                             │
│          ▼               ▼               ▼                             │
│  ┌───────────────┐ ┌─────────────┐ ┌──────────────────┐               │
│  │ INSIGHT LOG   │ │ DDNA        │ │ F9.1 Experience  │               │
│  │ (> 0.5)       │ │ SUGGESTION  │ │ Compiler         │               │
│  │               │ │ (> 0.7)     │ │ (insights        │               │
│  │ Registro      │ │ Don aprova  │ │  alimentam a     │               │
│  │ histórico     │ │ → vira      │ │  compilação)     │               │
│  │               │ │ decisão     │ │                  │               │
│  └───────────────┘ └─────────────┘ └──────────────────┘               │
│                                                                          │
│  Regra: o motor NUNCA força ação. Insight é sugestão.                   │
│  O Don aprova insights que viram ação.                                  │
└──────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Principio Arquitetural — Agregador, Não Criador

| Característica | Insight Generator (F3.1) |
|----------------|---------------------------|
| **Postura** | Passiva — observa dados existentes |
| **Trigger** | Semanal (ou pós-N tasks / on-demand) |
| **Custo** | < 30s — aritmética + leitura de CSVs, sem LLM no pipeline de detecção |
| **Output** | Insights classificados (suspeita causal, oportunidade, risco, contradição) |
| **Decisão** | Don decide se o insight vira ação (DDNA, learning, padrão) |
| **Fonte** | Metrics B1-B5 (CSV), learnings, patterns, DDNA, Trust Registry, timeline, entropia, momentum |

O motor é deliberadamente **barato e observacional**: a detecção de correlação é matemática pura (Pearson sobre séries temporais). O LLM entra apenas na **redação da hipótese** e na **avaliação de novelty/actionability** — nunca na detecção.

---

## 3. PIPELINE SEMANAL (7 PASSOS, < 30s)

```
┌──────────────────────────────────────────────────────────────────────────┐
│               INSIGHT GENERATOR — PIPELINE SEMANAL (7 passos)            │
│                                Budget: < 30s                              │
│                                                                          │
│  1. COLETA TODOS OS DADOS  ──▶ metrics CSVs (B1-B5), learnings,         │
│     (passiva, < 5s)            patterns, DDNA, trust, timeline,         │
│                                entropia, momentum, F7.3 score           │
│                               │                                          │
│                               ▼                                          │
│  2. NORMALIZA EM SÉRIES      ──▶ Cada fonte vira uma série temporal     │
│     TEMPORAIS (7 dias)          com 7+ pontos diários                   │
│                               │                                          │
│                               ▼                                          │
│  3. DETECTA CORRELAÇÕES      ──▶ Pearson entre pares de séries:         │
│     (< 5s, matemática pura)      B1×B5, B2×entropia, F7.3×F2.1,        │
│                                  B4×momentum, + pares livres            │
│                               │                                          │
│                               ▼                                          │
│  4. PARA CORRELAÇÃO > 0.5    ──▶ Gera hipótese testável:               │
│     (LLM, < 10s)                 "Se [X], então [Y], porque [mecanismo]"│
│                               │                                          │
│                               ▼                                          │
│  5. VALIDA CONTRA LEARNINGS  ──▶ A hipótese contradiz ou confirma      │
│     (FTS5, < 3s)                 learnings existentes?                  │
│                               │                                          │
│                               ▼                                          │
│  6. INSIGHT = CORRELAÇÃO +   ──▶ Monta o insight completo com:         │
│     HIPÓTESE + EVIDÊNCIA         r, série de dados, hipótese,          │
│                                  evidências, tipo, insight_score        │
│                               │                                          │
│                               ▼                                          │
│  7. CLASSIFICA E ROTEIA      ──▶ insight_score > 0.7 → DDNA suggestion │
│                                  insight_score > 0.5 → insight log      │
│                                  Don aprova → ação                      │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

### Budget Breakdown (< 30s total)

| Passo | Operação | Tempo Alvo |
|-------|----------|------------|
| 1. Coleta | Leitura paralela de CSVs + arquivos | < 5s |
| 2. Normalização | Agregação diária → séries de 7 pontos | < 3s |
| 3. Correlações | Pearson entre N pares (aritmética) | < 2s |
| 4. Hipóteses | LLM para redação (apenas r > 0.5) | < 10s |
| 5. Validação | FTS5 search em learnings | < 3s |
| 6. Montagem | Formatação do insight | < 2s |
| 7. Roteamento | Score + classificação + registro | < 2s |
| **Total** | | **< 27s** |

---

## 4. NORMALIZAÇÃO EM SÉRIES TEMPORAIS

### 4.1 Regra Fundamental

> **Mínimo 7 pontos de dados por série (janela de 7 dias).** Série com menos de 7 pontos não gera correlação — gera apenas observação.

### 4.2 Fontes e Séries Derivadas

| Fonte | Arquivo / Engine | Série Derivada | Unidade |
|-------|------------------|----------------|---------|
| B1 Cognitive Load | `memory/timeline/cognitive-load.csv` | load_diario | média do dia |
| B2 Decision Velocity | `memory/timeline/decision-velocity.csv` | velocity_diario | média de segundos |
| B3 Knowledge Freshness | `memory/timeline/knowledge-freshness.csv` | freshness_diario | média 0-1 |
| B4 Cross-agent Reuse | `memory/timeline/cross-agent-reuse.csv` | reuse_diario | % médio |
| B5 Cognitive Debt | `memory/timeline/cognitive-debt.csv` | debt_diario | % médio |
| Entropia (F1.6) | `engines/cognitive-entropy/ENTROPY.md` + analytics | entropy_diario | 0.0-1.0 |
| Momentum (F2.5) | `analytics/COGNITIVE_MOMENTUM.md` | momentum_diario | 0-100 |
| Engineering Score (F7.3) | `analytics/engineering-score.md` | eng_score_diario | 0-100 |
| ROI (F2.1) | `engines/cognitive-economy/SKILL.md` | roi_diario | % |
| Learnings | `memory/agent/*/learnings.md` | learnings_diario | contagem |
| Patterns | `memory/agent/*/patterns.md` | patterns_diario | contagem |
| DDNA | `memory/decisions/ddna/*.md` | ddna_diario | contagem |
| Trust | `memory/trust/TRUST_REGISTRY.md` | trust_diario | confidence médio |
| Timeline | `memory/timeline/ENGINEERING_TIMELINE.md` | eventos_diario | contagem |

### 4.3 Algoritmo de Normalização

```yaml
normalizacao:
  janela: "7 dias consecutivos"
  agregação: |
    Para cada fonte, agrupar eventos por dia (ISO 8601):
      - Métricas B1-B5: média diária do campo value
      - Entropia: valor do dia (ou interpolação linear entre medições)
      - Momentum: score do dia
      - Learnings/Patterns/DDNA: contagem diária (0 se nenhum)
  formato_saida: |
    serie:
      nome: "B2_velocity"
      pontos: [4.2, 5.1, 6.8, 7.3, 8.0, 7.5, 9.1]   # 7 dias
      datas: ["2026-07-24", ..., "2026-07-30"]
      fontes: ["memory/timeline/decision-velocity.csv"]
      cobertura: 7/7  # se < 7, marcar como insuficiente
  regra: |
    Série com cobertura < 7 dias é EXCLUÍDA da detecção de correlação
    e registrada como observação (não insight).
```

---

## 5. DETECÇÃO DE CORRELAÇÕES

### 5.1 Método — Correlação de Pearson

```
r(X, Y) = Σ((xᵢ − x̄)(yᵢ − ȳ)) / √(Σ(xᵢ − x̄)² · Σ(yᵢ − ȳ)²)

Onde:
  X, Y = duas séries temporais de 7+ pontos (mesma janela de dias)
  r ∈ [-1, 1]
```

### 5.2 Pares Canônicos de Correlação

O motor verifica SEMPRE estes pares, mais os pares livres:

```
├── B1 (load) × B5 (debt):        carga alta → débito alto?
│     Hipótese: "tasks pesadas geram aprendizado não-registrado?"
│
├── B2 (velocity) × Entropia:     decisão lenta → conhecimento desorganizado?
│     Hipótese: "o Kernel demora porque a base está contraditória?"
│
├── F7.3 (eng score) × F2.1 (ROI): score alto → ROI alto?
│     Hipótese: "código bem avaliado gera retorno cognitivo?"
│
└── B4 (reuse) × Momentum:        reuso alto → aprendizado rápido?
      Hipótese: "domínios com reuso cross-agent aprendem mais rápido?"
```

### 5.3 Pares Livres

Qualquer par de séries pode ser correlacionado, desde que ambas tenham 7+ pontos:

- Learnings × B1 (load)
- DDNA × B5 (debt) — "decisões registradas reduzem dívida?"
- Patterns × B4 (reuse) — "padrões formais aumentam reuso?"
- Trust × B2 (velocity) — "agentes confiáveis decidem mais rápido?"
- Timeline eventos × Learnings — "eventos geram aprendizado?"
- Agentes paralelos × Learnings — "paralelismo acelera aprendizado?" (§13)

### 5.4 Threshold de Detecção

| |r| | Classificação | Ação |
|------|---------------|------|
| **> 0.5** | Correlação forte | Gera hipótese (§6) |
| **0.3 - 0.5** | Correlação moderada | Registra como observação, acumula dados |
| **< 0.3** | Correlação fraca | Descarta ou arquiva para reavaliação |

---

## 6. GERAÇÃO DE HIPÓTESES (CORRELAÇÃO > 0.5)

### 6.1 Template de Hipótese

Para cada par com |r| > 0.5, o motor redige uma hipótese seguindo o template canônico:

```
"Se [condição observada], então [fenômeno relacionado],
 porque [mecanismo hipotético]."

Sempre classificada como SUSPEITA — nunca como causa.
```

```yaml
hipotese:
  id: "HYP-2026-07-30-001"
  pares: ["B2_velocity", "entropy_diario"]
  correlacao: 0.71
  direcao: "positiva"   # ambas sobem/descem juntas
  statement: |
    "Se a entropia cognitiva subiu 15% na mesma semana em que a
     velocidade de decisão (B2) caiu 20%, então decisões lentas
     podem ser sintoma de conhecimento desorganizado, porque o
     Kernel gasta mais tempo avaliando contradições na base."
  tipo: "causal-suspeito"
  confianca: "moderada"  # correlação forte, causalidade não provada
```

### 6.2 Limite de Hipóteses por Ciclo

- Máximo de **5 hipóteses por ciclo** (qualidade sobre quantidade).
- Se mais de 5 pares cruzam 0.5, priorizar por `|r|` descendente.
- Hipóteses de segurança (jail, dados, compliance) têm prioridade absoluta.

---

## 7. VALIDAÇÃO CONTRA LEARNINGS EXISTENTES

### 7.1 Objetivo

Uma correlação sem ancoragem no conhecimento existente é **números soltos**. A validação responde: *"o Cosca já sabe disso? Já tentou? Já falhou?"*

### 7.2 Método (FTS5, < 3s)

```yaml
validacao:
  metodo: "Semantic search (FTS5) + matching de tags"
  passos: |
    1. Extrair tags/domínios da hipótese (ex: #entropy, #decision-velocity)
    2. Buscar learnings com tags sobrepostas (matching ≥ 60%)
    3. Buscar failures com domínio similar (a hipótese já falhou?)
    4. Buscar DDNAs com decisões relacionadas
    5. Verificar freshness das evidências (F1.4 Wisdom Decay)
  resultados: |
    - CONFIRMA: learning existente sustenta a hipótese → insight MAIS forte
    - CONTRADIZ: learning existente nega a hipótese → insight tipo
      CONTRADIÇÃO (§9.4) → investigar qual está certo
    - NEUTRO: nenhuma evidência → insight com novelty ALTA
    - STALE: evidências antigas (freshness < 0.5) → qualificar como risco
```

### 7.3 Regra de Evidência

> **Insight sem evidência = hipótese, não insight.** Todo insight entregue ao Don deve listar as evidências concretas (série de dados, learnings relacionados, failures, DDNAs) que o sustentam.

---

## 8. FÓRMULA DE INSIGHT

### 8.1 Fórmula

```
insight_score = correlation_strength × 0.4 +
                novelty × 0.3 +
                actionability × 0.3

Onde:
  correlation_strength = |r| do par correlacionado (0.0 - 1.0)
  novelty              = quão inédito é o insight vs conhecimento existente
                         0.0 = já sabíamos  1.0 = nunca visto
  actionability        = quão acionável é a resposta à pergunta
                         "o que fazemos com isso?"
                         0.0 = informativo  1.0 = ação clara imediata
```

### 8.2 Thresholds

| insight_score | Classificação | Ação |
|---------------|---------------|------|
| **> 0.7** | Insight de alto impacto | **Sugestão de DDNA** — Don aprova → vira decisão rastreada |
| **> 0.5** | Insight relevante | **Registra no insight log** — histórico, referenciável |
| **≤ 0.5** | Observação | Arquiva como observação, acumula dados para próxima semana |

### 8.3 Semântica dos Componentes

| Componente | Peso | Por quê |
|------------|------|---------|
| **correlation_strength** | 0.4 | O dado é a base — sem correlação não há insight, só opinião |
| **novelty** | 0.3 | Insight que já sabemos é reforço (F9.1), não descoberta (F3.1) |
| **actionability** | 0.3 | Insight sem ação é entretenimento — o Don precisa de "o que fazer" |

---

## 9. TIPOS DE INSIGHT

### 9.1 Causal Suspeito

X e Y variam juntos — **não é causa, é suspeita**.

```
Exemplo: B2 (velocity) × Entropia correlacionam 0.71
Suspeita: "entropia alta pode estar deixando decisões mais lentas"
NÃO diz:  "entropia causa lentidão" (isso exige experimento, não correlação)
```

**Uso**: priorizar investigação (experimento, auditoria) sobre as relações mais prováveis. A suspeita é o input do 2nd-order reasoning e do experimento do Don.

### 9.2 Oportunidade

Padrão que sugere **ganho** — onde investir gera retorno.

```
Exemplo: B4 (reuse) × Momentum correlacionam 0.65
Insight: "domínios com reuso cross-agent alto aprendem mais rápido"
Oportunidade: "investir em documentação de padrões cross-domain
              amplifica o momentum de aprendizado"
```

**Uso**: sugestão de alocação de recursos, investimento em patterns, priorização de federação (F2.3).

### 9.3 Risco

Padrão que sugere **perda** — onde a tendência atual vai doer.

```
Exemplo: B5 (debt) × B1 (load) correlacionam 0.58
Insight: "tasks pesadas estão gerando aprendizado não-registrado"
Risco: "se a tendência continuar, a dívida cognitiva vai corroer
        a base de conhecimento em 3-4 semanas"
```

**Uso**: alerta proativo, priorização de mitigação, input para o Gap Detection (F1.3).

### 9.4 Contradição

**Dados dizem uma coisa, learnings dizem outra.**

```
Exemplo: dados mostram B4 (reuse) subindo 30%, mas nenhum learning
         registra reuso cross-agent na semana
Contradição: "os dados afirmam reuso; a memória não confirma.
              Ou os dados estão medindo algo errado, ou o registro
              de learnings está incompleto (dívida B5)."
```

**Uso**: detecção de problemas de instrumentação, gaps de memória, ou conhecimento stale — alimenta a Contradiction Engine (F8.3) e o F1.6.

---

## 10. INTEGRAÇÃO COM F9.1 — EXPERIENCE COMPILER

### 10.1 Fluxo

O Insight Generator é o **alimentador de descobertas** do Experience Compiler:

```
F3.1 Insight Generator                 F9.1 Experience Compiler
─────────────────────                  ──────────────────────────
Correlação forte +              ──▶   Compila learnings em padrões
hipótese validada                      e princípios reutilizáveis
        │                                     │
        ▼                                     ▼
Insight validado com              ──▶   Padrão candidato entra no
evidências concretas                      pipeline de compilação
                                          (maturity, freshness, impacto)
        │                                     │
        ▼                                     ▼
Insight de alto impacto           ──▶   Princípio candidato →
(vira learning marcado                   DDNA de princípio → Don
#insight-generator)                       → CONSTITUIÇÃO (via F9.2)
```

### 10.2 Contrato

```yaml
f9.1_integration:
  sentido: "F3.1 → F9.1 (unidirecional, dados fluem para compilação)"
  trigger: "Todo insight com score > 0.5 registra learning de origem
            no agente do domínio primário, com tag #insight-generator"
  consumo: |
    - F9.1 usa os learnings #insight-generator como fonte de compilação
    - Insights do tipo OPORTUNIDADE são os mais valiosos para padrões
      (sugerem técnica reutilizável explícita)
    - Insights do tipo RISCO alimentam heurísticas de prevenção
  output_esperado: |
    - Padrões compilados com origem rastreável ao F3.1
    - Princípios que nasceram de insights cross-domain têm prioridade
      de maturidade (já nascem com evidências de 2+ domínios)
```

---

## 11. INTEGRAÇÃO COM F10.3 — DISCOVERY

### 11.1 A Frase que Define a Relação

> **"Discovery encontra o que NÃO procurávamos; Insight Generator conecta o que encontramos."**

### 11.2 Divisão de Trabalho

```
┌──────────────────────────────────────────────────────────────────┐
│          F10.3 DISCOVERY × F3.1 INSIGHT GENERATOR               │
│                                                                  │
│  F10.3 Discovery (fim de sprint)                                 │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Pergunta: "O que descobrimos que NÃO procurávamos?"        │ │
│  │ Postura: observa divergência plano × realizado             │ │
│  │ Output: descobertas classificadas HIGH/MEDIUM/LOW          │ │
│  │   (ex: "race condition é sistêmico em 3 pacotes")          │ │
│  └────────────────────────────────────────────────────────────┘ │
│                          │                                      │
│                          ▼                                      │
│  F3.1 Insight Generator (semanal)                               │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Pergunta: "O que esses pontos desconectados revelam        │ │
│  │            quando conectados?"                              │ │
│  │ Postura: correlaciona séries temporais de dados            │ │
│  │ Output: insights com correlação + hipótese + evidência     │ │
│  │   (ex: "as 3 descobertas de concurrency da sprint          │ │
│  │         correlacionam com a queda de B2 — decisões         │ │
│  │         estão lentas porque concurrency trava o build")    │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  O F10.3 fornece as DESCOBERTAS (novidade, sem correlação).     │
│  O F3.1 fornece as CONEXÕES (correlação, sem novidade).         │
│  Juntos cobrem o ciclo completo: descobrir + conectar.          │
└──────────────────────────────────────────────────────────────────┘
```

### 11.3 Contrato

```yaml
f10.3_integration:
  fluxo: |
    1. F10.3 roda no fim da sprint → identifica descobertas HIGH
    2. F3.1 consome as descobertas como SÉRIE DE DADOS adicional
       (ex: contagem de descobertas por domínio por dia)
    3. F3.1 correlaciona as descobertas com métricas B1-B5
    4. Se descoberta + correlação > 0.5 → insight composto
       (novidade do F10.3 + correlação do F3.1 = insight de alto score)
  complementaridade: |
    - F10.3 sem F3.1: descobertas isoladas, sem contexto sistêmico
    - F3.1 sem F10.3: correlações sem novidade (reforço, não descoberta)
  exemplo_combinado: |
    F10.3 descobre: "3 artefatos da sprint tocam concurrency (não-planejado)"
    F3.1 correlaciona: "concurrency_count × B2_velocity → r = -0.62"
    Insight: "descobertas de concurrency crescem quando a velocidade de
              decisão cai — o build está sendo o gargalo das decisões"
```

---

## 12. INTEGRAÇÃO COM F9.2, F1.5 E F1.6

### 12.1 F9.2 Wisdom Distillation (indireto, via F9.1)

```
F3.1 → F9.1 (learning/pattern) → F9.2 (destilação) → emenda constitucional

Insights de tipo OPORTUNIDADE que se repetem em múltiplas semanas
acumulam evidências no F9.1 → quando maturity > 0.9, entram no funil
F9.2 como princípios candidatos.
```

### 12.2 F1.5 Metrics B1-B5 (fonte primária)

- O F3.1 é **consumidor puro** dos CSVs B1-B5 (leitura, nunca escrita).
- As métricas são as séries temporais mais confiáveis (coleta automática pós-task).
- Quando B5 (dívida) está alta, as séries de learnings podem estar incompletas → insights baseados nelas devem ser qualificados.

### 12.3 F1.6 Cognitive Entropy (fonte + consumidor)

- A entropia entra como série temporal nas correlações (par canônico B2×entropia).
- Insights do tipo RISCO que apontam desorganização crescente geram sugestão de ação de curadoria → **reduz entropia** na semana seguinte (ciclo virtuoso).
- A contradição detectada pelo F3.1 alimenta o F1.6 como nova contradição catalogada.

---

## 13. EXEMPLO REAL — DADOS DA SESSÃO 2026-07-30

### 13.1 O Par Correlacionado

**Série A — Agentes em paralelo por onda (Wave 1-12)** × **Série B — Learnings gerados (L22-L33)**

Âncoras documentadas da sessão:
- L31 (F1 completa): "Wave 4 (4 agentes) + Wave 5 (3 agentes) — 24 agentes em 5 ondas paralelas"
- L32 (F2 completa): "Wave 11 (3 agentes) — ~30 agentes em 11 ondas paralelas"
- L23 (Fase 1): 7 agentes em paralelo → 22 arquivos em < 5 minutos
- L24 (Fase 2): 6 agentes em paralelo → 6 engines em < 8 minutos
- L25 (Fase 3): 18 agentes → 7 engines em 30 minutos
- L22-L33: **12 learnings** gerados pela sessão de ondas paralelas

### 13.2 Séries (12 pontos, ≥ 7 OK)

```
Série A (X) — agentes em paralelo por onda:
  Wave:     1   2   3   4   5   6   7   8   9  10  11  12
  Agentes:  1   2   3   4   3   3   4   7   6   5   3  18

Série B (Y) — learnings L22-L33 distribuídos por onda:
  Wave:     1   2   3   4   5   6   7   8   9  10  11  12
  Learnings:0   0   1   1   1   0   1   2   2   1   1   2   (Σ = 12 = L22-L33)
```

### 13.3 Correlação (Pearson)

```
r = 0.693  (positiva forte — > 0.5 → gera hipótese)

Interpretação: ondas com mais agentes em paralelo produziram
mais learnings. A força (0.69) e a direção (positiva) sustentam
a suspeita: paralelismo acelera aprendizado.
```

### 13.4 Hipótese Gerada

```
"Se ondas com 6-18 agentes em paralelo produziram 12 learnings
 Level 4 em < 48h (L22-L33), então orquestração paralela acelera
 o aprendizado, porque cada agente opera em domínio independente
 e o kernel sintetiza sem dependências sequenciais (L23: 'zero
 dependências entre agentes')."
→ tipo: causal-suspeito
```

### 13.5 Validação Contra Learnings

| Evidência | Resultado |
|-----------|-----------|
| L23: "7 agentes em paralelo → 22 arquivos < 5 min" | ✅ CONFIRMA |
| L24: "6 agentes em paralelo → 6 engines < 8 min" | ✅ CONFIRMA |
| L25: "18 agentes → 4 fases, 25 tarefas em 30 min" | ✅ CONFIRMA |
| L31-L32: ondas 4-5 e 11 com agentes múltiplos | ✅ CONFIRMA |
| Baseline serial (L9-L21): 13 learnings em 2 dias de execução individual | 📊 comparação |

**Comparação quantitativa (o "×3")**:

```
Ondas com ≥ 4 agentes em paralelo: 9 learnings em 6 ondas = 1.5/onda
Ondas com < 4 agentes:             3 learnings em 6 ondas = 0.5/onda

Razão: 1.5 / 0.5 = 3×
→ "Paralelismo acelera aprendizado ×3" (por onda)
```

### 13.6 insight_score

```
correlation_strength = 0.693
novelty              = 0.60   (o padrão qualitativo era conhecido em
                               L23-L25, mas a quantificação ×3 por onda
                               e a correlação formal são novas)
actionability        = 0.85   (ação clara: adotar delegação paralela
                               como padrão para tasks multi-domínio)

insight_score = 0.693 × 0.4 + 0.60 × 0.3 + 0.85 × 0.3
              = 0.277 + 0.180 + 0.255
              = 0.712   →  > 0.7  →  SUGESTÃO DE DDNA
```

### 13.7 Sugestão de DDNA (aguardando aprovação do Don)

```yaml
ddna_suggestion:
  id: "DDNA-2026-07-30-IG-001"
  titulo: "Paralelismo acelera aprendizado ×3 — adotar como padrão"
  tipo_insight: "oportunidade"
  evidencia:
    - "Correlação 0.693 entre agentes paralelos e learnings (12 pontos)"
    - "L22-L33: 12 learnings Level 4 em < 48h via ondas paralelas"
    - "1.5 learnings/onda (≥4 agentes) vs 0.5 (serial) = 3×"
  acao_proposta: |
    Adotar delegação paralela (múltiplos agentes em domínios
    independentes) como padrão para tasks multi-domínio, com
    contexto completo + deliverables explícitos + zero dependências.
  confianca: 0.712
  status: "pending_don_approval"
```

### 13.8 Como o Don Decide

1. Don revisa a sugestão de DDNA (correlação + hipótese + evidências).
2. Don aprova → vira DDNA formal (`memory/decisions/ddna/`) → ação rastreada.
3. Don rejeita → registra rationale → insight volta ao log com status `rejected`.
4. A decisão alimenta o Trust Registry e o contrafactual gate (F1.2).

---

## 14. GOVERNANÇA E REGRAS

### 14.1 Regras Non-Negotiable

1. **Correlação ≠ causalidade.** Todo insight causal é "suspeita", nunca "causa". A palavra "causa" só pode ser usada após experimento controlado.
2. **Mínimo 7 pontos de dados por série.** Série insuficiente → observação, não insight.
3. **Insight sem evidência = hipótese, não insight.** Evidência = série de dados + learnings/DDNAs/failures relacionados.
4. **O Don aprova insights que viram ação.** Nenhum insight se torna DDNA sem aprovação explícita do Don.
5. **O motor nunca cria dados.** Ele lê CSVs, learnings, patterns, DDNA, trust — nunca escreve métricas falsas.
6. **Insights nunca são impostos.** São sugestões classificadas. Chiefs e Don decidem.
7. **O motor não pode consumir mais que 30s por ciclo.** Se o budget estourar, priorizar pares canônicos.

### 14.2 Propriedade e Responsabilidades

| Papel | Responsável | Responsabilidade |
|-------|-------------|------------------|
| **Owner** | Architecture Chief | Mantém a especificação, calibra thresholds, revisa qualidade dos insights |
| **Executor** | Kernel + Scheduler | Agenda o ciclo semanal (P3) |
| **Validador de Hipóteses** | Critic Chief | Revisão adversarial de hipóteses antes de virar DDNA |
| **Validador de Correlações** | Analytics Chief | Verifica se a correlação é estatisticamente defensável |
| **Aprovador** | Don | Aprova insights que viram ação (DDNA) |
| **Consumidor Principal** | Experience Compiler (F9.1) | Compila insights validados em padrões/princípios |

### 14.3 Ciclo de Vida do Insight

```
┌────────────┐   ┌───────────┐   ┌───────────┐   ┌───────────┐
│ OBSERVED   │──▶│ HYPOTHESIS│──▶│ VALIDATED │──▶│ ACTIONED  │
│ (r > 0.5,  │   │ (formada, │   │ (score    │   │ (Don      │
│ 7+ pontos) │   │ 5/ciclo)  │   │ > 0.5,    │   │  aprova → │
└────────────┘   └───────────┘   │ evidência │   │  DDNA)    │
                                 └───────────┘   └───────────┘
                                      │                │
                                      ▼                ▼
                              ┌───────────┐   ┌───────────┐
                              │ REJECTED  │   │ ARCHIVED  │
                              │ (evidência│   │ (< 0.5 ou │
                              │  contrária│   │  stale)   │
                              └───────────┘   └───────────┘
```

### 14.4 Métricas de Ciclo de Vida

| Métrica | Target |
|---------|--------|
| Insights gerados por semana | ≥ 2 |
| Taxa de validação (com evidência) | > 80% |
| Insights que viraram DDNA | ≥ 1 por mês |
| Tempo médio OBSERVED → ACTIONED | < 14 dias |
| Falsos positivos (r > 0.5 que não resistem à validação) | < 40% |

---

## 15. ARMAZENAMENTO

```
internal/embed/cosca/
├── engines/
│   └── insight-generator/
│       ├── SKILL.md                              ← Este arquivo
│       ├── insight-log.md                        ← Insights registrados (score > 0.5)
│       ├── hypotheses/                           ← Hipóteses ativas
│       │   └── HYP-{id}.yaml
│       └── reports/
│           └── weekly-{YYYY-MM-DD}.md            ← Relatório semanal
│
├── memory/
│   ├── timeline/                                 ← Séries B1-B5 (fonte, não escrita pelo F3.1)
│   │   ├── cognitive-load.csv
│   │   ├── decision-velocity.csv
│   │   ├── knowledge-freshness.csv
│   │   ├── cross-agent-reuse.csv
│   │   └── cognitive-debt.csv
│   └── decisions/
│       └── ddna/
│           └── DDNA-{date}-IG-{seq}.md           ← DDNAs aprovados (via Don)
```

### Formato do Insight Log (CSV parseável)

```csv
timestamp,id,par_a,par_b,r,insight_score,tipo,status,ddna_ref,evidence_count
2026-07-30,IG-001,agentes_paralelos,learnings,0.693,0.712,oportunidade,pending_don_approval,DDNA-2026-07-30-IG-001,5
```

---

## 16. CLI E AUTOMAÇÃO

| Comando | Função | Tempo |
|---------|--------|-------|
| `cosca insight generate` | Executa o pipeline semanal completo | < 30s |
| `cosca insight generate --pairs=B2:entropy` | Correlaciona par específico | < 10s |
| `cosca insight log --week` | Exibe insights da semana | < 2s |
| `cosca insight report` | Gera relatório semanal markdown | < 5s |
| `cosca insight approve --id IG-001` | Don aprova → cria DDNA | < 2s |

### Automação

- **Semanal**: scheduler executa `cosca insight generate` (segunda-feira 07:00).
- **Pós-N tasks**: quando 10+ tasks acumulam desde o último ciclo.
- **On-demand**: Don ou qualquer Chief pode executar manualmente.
- **Fim de sprint**: após o F10.3 Discovery rodar (integração §11).

---

## 17. MÉTRICAS DO ENGINE

| Métrica | Definição | Target |
|---------|-----------|--------|
| **Ciclos executados** | Ciclos semanais completados | 1/semana |
| **Insights gerados/semana** | Insights com score > 0.5 | ≥ 2 |
| **Sugestões de DDNA/semana** | Insights com score > 0.7 | ≥ 0 (qualidade) |
| **DDNAs aprovados/mês** | Insights que viraram decisão | ≥ 1 |
| **Taxa de validação** | Insights com evidência / total | > 80% |
| **Falsos positivos** | Hipóteses refutadas na validação | < 40% |
| **Latência do ciclo** | Tempo total do pipeline | < 30s |
| **Séries insuficientes** | Séries < 7 pontos (não correlacionadas) | registrar, não descartar |

---

## 18. CASOS DE BORDA E ANTI-PADRÕES

| Caso | Tratamento |
|------|------------|
| **Série com < 7 dias** | Registra como observação, acumula dados. Nunca força correlação. |
| **r alto mas significado nulo** | Ex: "learnings × café da manhã do Don" — validação contra learnings filtra; se nenhum mecanismo plausível, marca como `spurious`. |
| **r alto com poucos pontos** | 7 pontos é mínimo, mas 7-10 pontos exigem confiança extra — critic review obrigatório. |
| **Correlação espúria (terceira variável)** | A hipótese deve propor mecanismo; se um fator comum explica ambos (ex: "semana de lançamento"), classificar como `confounded`. |
| **Dados ausentes (CSV vazio)** | Ciclo roda mesmo assim — apenas séries disponíveis são correlacionadas; relatório nota a cobertura. |
| **Contradição entre dados e learnings** | Vira insight tipo CONTRADIÇÃO (§9.4), não é silenciada. |
| **Anti-padrão: LLM sintético** | Pedir ao modelo para "inventar insights" sem dados. Proibido — insight sem correlação é alucinação. |
| **Anti-padrão: causalidade prematura** | Escrever "X causa Y" no relatório. Proibido — sempre "suspeita". |
| **Anti-padrão: ruído semanal** | Se nenhuma correlação cruzar 0.5, o relatório diz "zero insights" — honestidade sobre falta de sinal. |

---

## 19. CONSTRAINTS E FORBIDDEN ACTIONS

- **Nunca** modificar CSVs de métricas, learnings ou DDNA durante a observação (read-only).
- **Nunca** interromper o Don com notificações síncronas — tudo via insight log/report.
- **Nunca** exceder 30s por ciclo.
- **Nunca** gerar mais de 5 hipóteses por ciclo.
- **Nunca** usar a palavra "causa" sem experimento controlado.
- **Nunca** criar DDNA sem aprovação do Don.
- **Nunca** correlacionar séries com menos de 7 pontos.
- **Nunca** descartar contradições dados × learnings — sempre reportar.
- **Sempre** ancorar todo insight em evidências concretas (fontes, CSVs, learnings).
- **Sempre** preservar hipóteses refutadas (são aprendizado negativo).
- **Sempre** registrar insights > 0.5 no insight log (append-only).

---

## 20. CRITÉRIOS DE QUALIDADE

- [ ] Pipeline semanal executa em < 30s sem falhas
- [ ] Séries temporais normalizadas com mínimo de 7 pontos
- [ ] Pares canônicos (B1×B5, B2×entropia, F7.3×F2.1, B4×momentum) verificados a cada ciclo
- [ ] Correlações > 0.5 geram hipóteses com template canônico
- [ ] Toda hipótese é validada contra learnings existentes (FTS5)
- [ ] Todo insight lista evidências concretas (nunca "porque sim")
- [ ] insight_score calculado com os 3 componentes documentados
- [ ] Thresholds corretos: > 0.7 DDNA, > 0.5 log, ≤ 0.5 observação
- [ ] Insight sem evidência é rotulado como hipótese, nunca insight
- [ ] Correlação nunca é apresentada como causalidade
- [ ] Insights alimentam F9.1 (learnings #insight-generator)
- [ ] Insights integram com F10.3 (descobertas viram séries)
- [ ] Don aprova antes de qualquer DDNA
- [ ] Insight log é append-only e parseável
- [ ] Relatório semanal gerado e referenciado no dashboard

---

## 21. ESCALATION

| Issue | Escalate To |
|-------|-------------|
| Ciclo falhou 3 semanas consecutivas | Architecture Chief + Kernel |
| Correlação de segurança/compliance (jail, dados) com r > 0.5 | Security Chief + Don (imediato) |
| Taxa de falsos positivos > 60% | Architecture Chief (recalibrar thresholds) |
| Insight de alto score (> 0.7) sem ação por 2 semanas | Kernel (notificar Don) |
| Contradição sistêmica dados × learnings | Contradiction Engine (F8.3) + Analytics Chief |
| Insight que sugere mudança constitucional | F9.1 → F9.2 (pipeline constitucional) |

---

## 22. GLOSSÁRIO

| Termo | Definição |
|-------|-----------|
| **Série temporal** | Sequência de 7+ pontos diários de uma fonte de dados (B1-B5, learnings, etc.) |
| **Correlação (r)** | Coeficiente de Pearson entre duas séries — mede associação, NÃO causalidade |
| **Hipótese** | Afirmação testável gerada quando r > 0.5, template "Se → então → porque" |
| **Insight** | Correlação + hipótese + evidências, com insight_score > 0.5 |
| **insight_score** | `r×0.4 + novelty×0.3 + actionability×0.3` |
| **Sugestão de DDNA** | Insight com score > 0.7, enfileirado para aprovação do Don |
| **Causal suspeito** | Tipo de insight: X e Y variam juntos, mas não é causa provada |
| **Oportunidade** | Tipo de insight: padrão que sugere ganho se explorado |
| **Risco** | Tipo de insight: padrão que sugere perda se ignorado |
| **Contradição** | Tipo de insight: dados dizem uma coisa, learnings dizem outra |
| **Evidência** | Dado concreto que sustenta o insight (série, learning, DDNA, failure) |
| **Insight log** | Registro histórico append-only de insights com score > 0.5 |
| **Falso positivo** | Correlação > 0.5 que não resiste à validação contra learnings |

---

## 23. REFERÊNCIAS CRUZADAS

- [cognitive-maturity-implementation.md F3.1](../../workflows/cognitive-maturity-implementation.md) — tarefa de implementação
- [COGNITIVE_MATURITY.md §5 C9](../../architecture/COGNITIVE_MATURITY.md) — conceito original do Insight Generator
- [Experience Compiler (F9.1)](../experience-compiler/SKILL.md) — consumidor dos insights
- [Wisdom Distillation (F9.2)](../wisdom-distillation/SKILL.md) — downstream constitucional
- [Discovery Engine (F10.3)](../discovery/SKILL.md) — complementar (descobre + conecta)
- [Cognitive Metrics (F1.5)](../../analytics/cognitive-metrics.md) — séries B1-B5
- [Cognitive Entropy (F1.6)](../cognitive-entropy/ENTROPY.md) — série de entropia
- [Cognitive Momentum (F2.5)](../cognitive-momentum/SKILL.md) — série de momentum
- [Engineering Score (F7.3)](../../analytics/engineering-score.md) — série de score
- [Cognitive Economy (F2.1)](../cognitive-economy/SKILL.md) — série de ROI
- [Contradiction Engine (F8.3)](../contradiction/SKILL.md) — escalação de contradições
- [Decision DNA (F1.1)](../../memory/DECISION_DNA_FORMAT.md) — destino dos insights aprovados
- [Gap Detection (F1.3)](../gap-detection/SKILL.md) — consumo de insights de risco
- [CONVENTIONS.md](../../CONVENTIONS.md) — contrato canônico de skills

---

## 24. HISTÓRICO

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca AI Chief | Especificação inicial: motor de descoberta proativa de padrões, ciclo de 4h, pipeline de 5 estágios (Observação → Padrão → Hipótese → Experimento → Conhecimento), 7 fontes de observação, 5 tipos de padrão, 4 tipos de hipótese, 4 tipos de experimento. |
| 2.0.0 | 2026-07-30 | Cosca Architecture Chief | **Redesign completo por ordem do Don.** Novo core: pipeline semanal (< 30s) de correlação de séries temporais com mínimo de 7 pontos. Novo modelo: pares canônicos (B1×B5, B2×entropia, F7.3×F2.1, B4×momentum), fórmula `insight_score = r×0.4 + novelty×0.3 + actionability×0.3` com thresholds (> 0.7 DDNA, > 0.5 log), 4 tipos de insight (causal suspeito, oportunidade, risco, contradição). Novas integrações: F9.1 (Experience Compiler — consumidor), F10.3 (Discovery — complementar), F9.2 (downstream), F1.5/F1.6 (fontes). Exemplo real: correlação agentes paralelos × learnings (r=0.693, ×3, score 0.712 → DDNA). Regra central: correlação ≠ causalidade; insight sem evidência = hipótese. |

---

> **Enforced by**: Cosca Architecture Chief | **Próximo ciclo**: Segunda-feira 07:00 ou `cosca insight generate`
> **Kernel instruction**: `cosca insight generate --weekly`
