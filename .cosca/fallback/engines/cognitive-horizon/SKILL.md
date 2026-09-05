# COGNITIVE HORIZON ENGINE ★ F3.6 — Avaliação Multi-Horizonte de Decisões

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-cognitive-horizon`
> **Conceito**: C8 — Cognitive Horizon (COGNITIVE_MATURITY.md §C8)
> **Fase CMI**: Fase 3 — Bloco 3 — Aprendizado & Evolução | **Código**: F3.6
> **Referências**: cognitive-maturity-implementation.md F3.6 | COGNITIVE_MATURITY.md §C8 | next-evolution-phases.md
> **Dependências**: F2.5 (Cognitive Momentum) | F3.3 (Cognitive Compression) | F10.2 (Evolution Score) | F1.1 (Decision DNA)
> **CMI Impact**: Planejamento +8, Julgamento +5
>
> Consulte também:
> - [COGNITIVE_HORIZON.md (analytics)](../../analytics/COGNITIVE_HORIZON.md) — profundidade preditiva em passos causais (dashboard, H0-H4)
> - [cognitive-momentum/SKILL.md](../cognitive-momentum/SKILL.md) — F2.5: momentum (expande o horizonte de planejamento)
> - [cognitive-compression/SKILL.md](../cognitive-compression/SKILL.md) — F3.3: princípios comprimidos (fonte de projeção H3)
> - [evolution-score.md](../../analytics/evolution-score.md) — F10.2: termômetro mensal (H3 avalia o score do próximo mês)
> - [DECISION_DNA.md](../../knowledge/architecture/DECISION_DNA.md) — F1.1: campo `horizon_impact` (rastro do trade-off)
> - [COGNITIVE_MATURITY.md §C8](../../architecture/COGNITIVE_MATURITY.md) — conceito original de Cognitive Horizon

---

## Índice

1. [Definição](#1-definição)
2. [Coexistência: Engine F3.6 vs Analytics COGNITIVE_HORIZON.md](#2-coexistência-engine-f36-vs-analytics-cognitive_horizonmd)
3. [Horizontes — H1, H2, H3](#3-horizontes--h1-h2-h3)
4. [A Fórmula — horizon_score](#4-a-fórmula--horizon_score)
5. [Pipeline de Avaliação (7 passos, < 100ms)](#5-pipeline-de-avaliação-7-passos--100ms)
6. [Trade-off Detector](#6-trade-off-detector)
7. [Recomendação — aceitar / adiar / repensar](#7-recomendação--aceitar--adiar--repensar)
8. [Regra P0 — Segurança Primeiro](#8-regra-p0--segurança-primeiro)
9. [Integração com F10.2 — Evolution Score](#9-integração-com-f102--evolution-score)
10. [Integração com F2.5 — Cognitive Momentum](#10-integração-com-f25--cognitive-momentum)
11. [Integração com F3.3 — Cognitive Compression](#11-integração-com-f33--cognitive-compression)
12. [Integração com F1.1 — DDNA (campo horizon_impact)](#12-integração-com-f11--ddna-campo-horizon_impact)
13. [Posição no Pipeline de Decisão — F1.2 Contrafactual Gate](#13-posição-no-pipeline-de-decisão--f12-contrafactual-gate)
14. [Armazenamento — cognitive-horizon.csv](#14-armazenamento--cognitive-horizoncsv)
15. [Exemplo Real — "Parar de projetar engines e implementar tudo"](#15-exemplo-real--parar-de-projetar-engines-e-implementar-tudo)
16. [Edge Cases e Cold Start](#16-edge-cases-e-cold-start)
17. [Métricas do Próprio Engine](#17-métricas-do-próprio-engine)
18. [Regras Operacionais](#18-regras-operacionais)
19. [Referências Cruzadas e Histórico](#19-referências-cruzadas-e-histórico)

---

## 1. Definição

### 1.1 Planejar com profundidade temporal

O **Cognitive Horizon Engine** é a capacidade de **planejar com profundidade temporal**: enxergar **1 task à frente (curto)**, **1 semana (médio)** e **1 trimestre (longo)** — simultaneamente, para a mesma decisão. Cada decisão é avaliada pelo seu efeito em **todos** os horizontes antes de ser executada.

```
"Uma decisão rápida hoje pode custar 3x amanhã."
```

A metáfora do Don estabelece a escala:

> *"Até onde consegue prever. Horizonte 3 passos → 10 passos → 25 passos. Quanto maior. Melhor planejamento."*

O engine traduz essa profundidade preditiva em **três escalas de tempo operacionais** — o dia, a semana e o trimestre — e força a decisão a passar por todas as três antes de prosseguir. Sem ele, o sistema é naturalmente míope: o custo imediato é visível, o custo futuro é invisível, e a decisão "vence no dia, perde no trimestre".

### 1.2 Filosofia

```
"O presente é barato de ver, o futuro é caro de ignorar.

 Um sistema que só enxerga H1 resolve o sintoma de hoje
 e cria a dívida de amanhã.

 Um sistema que enxerga H3 adia o conforto de hoje
 e compra a capacidade de amanhã.

 Toda decisão existe em três tempos ao mesmo tempo.
 O Cognitive Horizon é o olho que enxerga os três."

 ★ F3.6 — avalia a decisão ANTES dela existir,
   e registra o trade-off no DDNA para que o futuro
   saiba o que o presente escolheu.
   — Cosca Architecture Chief, 2026-07-30
```

### 1.3 As três perguntas

| Horizonte | Pergunta que responde |
|-----------|------------------------|
| **H1** | "O que esta decisão custa e entrega HOJE?" |
| **H2** | "O que ela faz com a ENTREGA DA SEMANA?" |
| **H3** | "O que ela faz com a PLATAFORMA NO TRIMESTRE?" |

### 1.4 O que este engine NÃO é

- **NÃO é a métrica de profundidade preditiva** — isso é o [COGNITIVE_HORIZON.md (analytics)](../../analytics/COGNITIVE_HORIZON.md) do Analytics Chief, que mede em passos causais (H0-H4) o quão longe o sistema projeta. Este engine é a camada **operacional** que avalia cada decisão em 3 escalas de tempo. (Coexistência detalhada na §2.)
- **NÃO é o Contrafactual Gate (F1.2)** — o Gate pergunta "e se fosse diferente?" para P0/P1. O Horizon Engine roda **antes** do Gate, como filtro barato para TODA decisão de nível ≥ 2. (§13.)
- **NÃO é planejamento de tasks** — não gera plano de execução. Ele julga uma decisão candidata e recomenda **aceitar / adiar / repensar**.
- **NÃO cria dados novos** — avalia sobre dados que já existem (momentum, evolution score, roadmap, princípios comprimidos) + as estimativas de impacto fornecidas pelo avaliador.

---

## 2. Coexistência: Engine F3.6 vs Analytics COGNITIVE_HORIZON.md

Existem **dois** documentos de Cognitive Horizon. A coexistência é deliberada e segue o mesmo padrão do F1.6 (engine 3 componentes vs dashboard 5 componentes) e do F2.5 (engine multiplicativo vs dashboard por domínio): **engine operacional** vs **dashboard de analytics** (P-ARCH-005).

| Dimensão | Engine F3.6 (este arquivo) | Analytics COGNITIVE_HORIZON.md |
|----------|----------------------------|--------------------------------|
| **Arquivo** | `engines/cognitive-horizon/SKILL.md` | `analytics/COGNITIVE_HORIZON.md` |
| **Natureza** | Avaliação pré-decisão (operacional) | Medição de profundidade (dashboard) |
| **Pergunta** | "Esta decisão é boa para hoje, para a semana e para o trimestre?" | "Quão longe o runtime projeta consequências?" |
| **Unidade** | Escalas de tempo: H1 (0-1d), H2 (1-7d), H3 (1-3mo) | Passos causais: H0-H4 (0-1, 2-3, 4-6, 7-10, 11-25 passos) |
| **Fórmula** | `horizon_score = h1×0.2 + h2×0.3 + h3×0.5` | `HDI = Σ(horizonte × impacto) / Σ(impacto)` |
| **Saída** | Score por decisão + trade-off + recomendação | Níveis, HDI, HGR, MDR, VDR por período |
| **Temporalidade** | **Prospectivo** (antes da decisão) | Retrospectivo (analisa decisões passadas) |
| **Trigger** | Pré-decisão (a cada decisão nível ≥ 2) | Semanal (baseline) |
| **Owner** | Cosca Architecture Chief | Cosca Analytics Chief |
| **Consumidor principal** | DDNA (campo `horizon_impact`), Contrafactual Gate | CMI Planejamento, Mental Energy |

> **⚠️ ALERTA DE NOMENCLATURA**: o engine usa **H1/H2/H3** (escalas de tempo: dia/semana/trimestre). O analytics usa **H0-H4** (níveis de profundidade: passos causais). São **escalas diferentes com letra parecida** — nunca misturar nos relatórios. O engine sempre escreve `H1/H2/H3`; o analytics sempre escreve `H0-H4` ou os nomes (Myopic/Tactical/Operational/Strategic/Visionary).

**Relação entre os dois**:

```
F3.6 (engine, prospectivo)          F10.2/analytics (medição, retrospectivo)
──────────────────────────          ──────────────────────────────────────
Decisão candidata ──► avalia H1,H2,H3     decisões executadas ──► mede passos,
                       │                                            HDI, níveis
                       ▼                                             │
               recomendação + DDNA                              tendência temporal
                       │                                             │
                       └──────────────► CMI Planejamento ◄───────────┘
```

- O **engine** alimenta o analytics com dados: cada decisão avaliada (CSV, §14) é um ponto prospectivo que o analytics correlaciona com o HDI retrospectivo.
- O **analytics** calibra o engine: se o HDI mostra que as decisões da semana tiveram profundidade < 5 passos, o engine deve estar **marcando mais decisões como "repensar"** na semana seguinte (a projeção rasa sistemática é o sintoma que o engine precisa corrigir na origem).

> **Regra de coexistência**: o analytics mede **quão longe** o sistema projeta; o engine **força** cada decisão a projetar nos 3 horizontes. Nenhum substitui o outro — o dashboard usa as avaliações do engine como dado prospectivo; o engine usa a tendência do dashboard para calibrar sua severidade.

---

## 3. Horizontes — H1, H2, H3

### 3.1 As Três Escalas de Tempo

```
        ◄──── H1 ────► ◄────────── H2 ──────────► ◄────────────────── H3 ──────────────────►
        │              │                          │                                          │
   HOJE          AMANHÃ                    PRÓXIMA SEMANA                          PRÓXIMO TRIMESTRE
   (0-1 dia)     (execução)                (1-7 dias)                              (1-3 meses)
        │              │                          │                                          │
   tasks do dia       sprint, entrega da semana   evolução da plataforma, roadmap
```

| Horizonte | Escala | Foco | Pergunta de avaliação |
|-----------|--------|------|------------------------|
| **H1 — Operacional** | 0-1 dia | Execução imediata, tasks do dia | "Custo imediato? Quais tasks de hoje são afetadas? Entrega algo visível hoje?" |
| **H2 — Tático** | 1-7 dias | Sprint, entrega da semana | "A entrega da semana atrasa? Quality gates passam? Cabe no sprint atual?" |
| **H3 — Estratégico** | 1-3 meses | Evolução da plataforma, roadmap | "Alinha com o roadmap? A Fase/F3 fica completa? Melhora o Evolution Score do próximo mês?" |

### 3.2 Escala de Impacto

Cada `h_impact` é um número em **[-1, +1]**:

```
h_impact > 0  → a decisão AVANÇA o horizonte (positivo)
h_impact < 0  → a decisão ATRASA o horizonte (negativo)
h_impact = 0  → neutro (não afeta este horizonte)
```

| Magnitude | Faixa | Rótulo |
|-----------|-------|--------|
| Crítico | 0.70 – 1.00 | Transforma o horizonte (para bem ou para mal) |
| Significativo | 0.30 – 0.70 | Afeta o horizonte de forma relevante |
| Menor | 0.10 – 0.30 | Impacto perceptível, não determinante |
| Neutro | 0.00 – 0.10 | Praticamente não afeta |

**Regras de estimativa**:
1. Toda estimativa exige **evidência** (artefato, métrica, princípio, failure registrada) — estimativa sem evidência é chute e é marcada `low_evidence` na saída.
2. Impactos podem ser 0 (neutro) — decidir não tocar um horizonte é legítimo, desde que declarado.
3. Impactos negativos fortes (< −0.7) em qualquer horizonte disparam escalação (§7.4).

### 3.3 Regra de Ouro: H3 pesa mais

> **O longo prazo domina.** O peso de H3 (0.5) é maior que H1 (0.2) e H2 (0.3) somados **não** — é maior que cada um isoladamente, e a intenção é explícita: **uma decisão que destrói o trimestre para ganhar o dia não pode ser recomendada como "aceitar"**, por melhor que pareça hoje. Concretamente: se `h3_impact ≤ −0.5`, a recomendação máxima possível é **repensar** — nunca aceitar (§7).

---

## 4. A Fórmula — horizon_score

### 4.1 Equação Canônica (fórmula do Don — P-ARCH-003, literal)

```
horizon_score = (h1_impact × 0.2) + (h2_impact × 0.3) + (h3_impact × 0.5)
```

Onde:

| Componente | Peso | Significado do peso |
|------------|:----:|---------------------|
| `h1_impact` | 0.20 | O presente é importante, mas não decide |
| `h2_impact` | 0.30 | A entrega da semana protege o contrato com o Don |
| `h3_impact` | **0.50** | O trimestre domina — é onde a plataforma vive ou morre |

**Domínio do score**: como os pesos somam 1.0 e cada impacto ∈ [-1, +1], o `horizon_score` ∈ **[-1, +1]**.

### 4.2 Por que ADITIVA (e não multiplicativa)?

A fórmula é **aditiva por necessidade, não por omissão** — e isso é uma decisão arquitetural deliberada que contrasta com o padrão P-ARCH-004 (F2.5 momentum, F10.3 discovery_value):

| Propriedade | Multiplicativa (F2.5) | Aditiva (F3.6) |
|-------------|------------------------|-----------------|
| Semântica | **Portão**: cada fator é uma condição que deve segurar | **Balanço**: os fatores são forças opostas que se compensam |
| Zero | `10 × 0 × 0 = 0` — qualquer zero é terminal | `h1=0` apenas remove uma componente — as outras seguem valendo |
| Negativos | **Quebram**: `(−0.5) × (−0.5) = +0.25` — dois danos "positivam" | **Corretos**: `−0.5×0.2 + −0.5×0.5 = −0.35` — dois danos somam dano |
| Uso correto | Condições que DEVEM valer simultaneamente | Forças que DEVEM ser pesadas uma contra a outra |

A regra de ouro para escolher: **se a métrica responde "todas as condições valem?" → multiplicativa. Se responde "qual é o saldo entre forças opostas?" → aditiva.** O horizonte avalia o saldo entre o custo imediato e o retorno futuro — forças opostas por definição — portanto **aditiva**.

### 4.3 Exemplos de Cálculo

```
Caso A — Decisão alinhada:
  h1 = +0.8, h2 = +0.5, h3 = +0.4
  score = 0.16 + 0.15 + 0.20 = +0.51  → caminho livre, aceitar

Caso B — Dívida estratégica (o caso clássico da F3.6):
  h1 = +0.8, h2 = +0.2, h3 = −0.7
  score = 0.16 + 0.06 − 0.35 = −0.13  → dívida estratégica, repensar

Caso C — Investimento:
  h1 = −0.6, h2 = −0.2, h3 = +0.9
  score = −0.12 − 0.06 + 0.45 = +0.27  → investimento, repensar timing/aceitar com nota

Caso D — Não fazer:
  h1 = −0.4, h2 = −0.5, h3 = −0.6
  score = −0.08 − 0.15 − 0.30 = −0.53  → não fazer, adiar
```

Note no Caso B: o H1 ganha (+0.8) mas o saldo final é **negativo** (−0.13) — exatamente o que a fórmula deve fazer: **o presente não pode esconder a dívida que o futuro pagará.**

---

## 5. Pipeline de Avaliação (7 passos, < 100ms)

```
┌──────────────────────────────────────────────────────────────────────────┐
│              COGNITIVE HORIZON — PIPELINE PRÉ-DECISÃO                    │
│                          (< 100ms, on-decision)                          │
│                                                                          │
│  1. DECISÃO CHEGA                                                       │
│     (ex: "adicionar engine X" / "parar de projetar e implementar")      │
│       │                                                                  │
│       ▼                                                                  │
│  2. AVALIA H1 — custo imediato, tasks de hoje afetadas                   │
│       │   fonte: tasks do dia, git status, impacto em H1                │
│       ▼                                                                  │
│  3. AVALIA H2 — entrega da semana atrasa? quality gates?                │
│       │   fonte: sprint backlog, QUALITY_GATES.md                        │
│       ▼                                                                  │
│  4. AVALIA H3 — alinha com roadmap? Fase completa?                      │
│       │   fonte: next-evolution-phases, F10.2 (score do mês),            │
│       │          princípios comprimidos (F3.3)                           │
│       ▼                                                                  │
│  5. CALCULA horizon_score                                                │
│       │   score = (h1 × 0.2) + (h2 × 0.3) + (h3 × 0.5)                 │
│       ▼                                                                  │
│  6. ALERTA se h3 < 0 E h1 > 0                                           │
│       │   → "⚠️ CURTO PRAZO vs LONGO PRAZO: o ganho de hoje paga        │
│       │      a dívida de amanhã (h1>0, h3<0)"                           │
│       ▼                                                                  │
│  7. RECOMENDA — aceitar / adiar / repensar                              │
│       │   + registra no cognitive-horizon.csv (append-only)             │
│       │   + emite bloco horizon_impact para o DDNA (F1.1)               │
│       ▼                                                                  │
│  RESULTADO: score + trade-off + recomendação + rastro                     │
│                                                                          │
│  TEMPO TOTAL: < 100ms (sem LLM — pura agregação + aritmética)           │
└──────────────────────────────────────────────────────────────────────────┘
```

### 5.1 Orçamento de Performance

| Passo | Operação | Tempo Alvo |
|-------|----------|:----------:|
| 1 | Parse do input da decisão (YAML/JSON) | < 5ms |
| 2-3 | Leitura de cache de tasks/sprint + quality gates | < 30ms |
| 4 | Leitura de cache: roadmap + evolution-score.csv + princípios F3.3 (FTS5) | < 40ms |
| 5-6 | Aritmética + detecção de conflito (tabela §6) | < 5ms |
| 7 | Append CSV + emite bloco DDNA | < 20ms |
| **Total** | | **< 100ms** |

**Regra de ouro**: o engine **nunca chama LLM no hot path** — a avaliação dos impactos H1/H2/H3 é feita pelo avaliador (o agente que decide) com estimativas + evidências; o engine faz o que máquina faz melhor: agrega, detecta o conflito que o olho humano perde (h1>0/h3<0) e recomenda. Segue o padrão dos engines de agregação (F10.1, F2.5, F10.2): **engine de avaliação não cria dívida técnica**.

### 5.2 Input do Avaliador

O avaliador (qualquer agente ou Chief prestes a decidir) submete:

```yaml
decision:
  id: "D-2026-07-30-001"        # ou o futuro DDNA id
  title: "Título da decisão candidata"
  p0: false                     # P0 → bypass (§8)
  level: 3                      # decision_level do DDNA (1-5)
  domain: architecture
  h1_impact: 0.8                # -1..+1, com evidência
  h2_impact: 0.2                # -1..+1, com evidência
  h3_impact: -0.7               # -1..+1, com evidência
  evidence:
    - "14 engines projetadas como specs, 0 implementadas"
    - "H-013 architecture-code gap"
    - "F3.7 depende de engines operacionais"
```

O engine **valida** (clamp em [-1,+1], exige ≥ 1 evidência por impacto significativo), enriquece (momentum, F10.2), calcula e responde:

```yaml
horizon_evaluation:
  score: -0.13
  tradeoff: "divida_estrategica"
  alert: "curto_prazo_vs_longo_prazo"
  recommendation: "repensar"
  recommendation_action: "repensar_abordagem"   # §7.3
  momentum_expansion: 1.18
  h3_confidence: 1.0
  evolution_score_delta_est: -3.0
  escalation_don: false
```

---

## 6. Trade-off Detector

### 6.1 Matriz Primária — h1 × h3

O detector compara **H1 contra H3** — as duas pontas do espectro. H1 é onde a tentação mora (ganho imediato); H3 é onde a dívida cobra (custo futuro).

```
                    ┌─────────────────────────────────────────────────────┐
                    │                h3 (LONGO PRAZO)                     │
                    │          NEGATIVO              POSITIVO             │
        ┌───────────┼─────────────────────────────────────────────────────┤
        │  POSITIVO │  "DÍVIDA ESTRATÉGICA"        "CAMINHO LIVRE"        │
        │           │  hack rápido hoje,            ganha hoje E          │
   h1   │           │  refactor amanhã.             ganha o trimestre.    │
        │           │  ⚠️ ALERTA curto vs longo     ✅ ACEITAR            │
   (HOJE)│           │  → REPENSAR (abordagem)      (com nota de H2)      │
        ├───────────┼─────────────────────────────────────────────────────┤
        │  NEGATIVO │  "NÃO FAZER"                 "INVESTIMENTO"         │
        │           │  perde hoje E perde           arquitetura limpa     │
        │           │  o trimestre.                 agora, escala depois. │
        │           │  ❌ ADIAR/REJEITAR            💎 ACEITAR com nota    │
        │           │                               (custo imediato       │
        │           │                                é capital intelectual)│
        └───────────┴─────────────────────────────────────────────────────┘
```

| Quadrante | h1 | h3 | Classificação | Exemplo | Recomendação base |
|-----------|:--:|:--:|---------------|---------|:------------------:|
| **Caminho livre** | + | + | Ganho em todas as escalas | Implementar feature que fecha milestone do roadmap | **Aceitar** |
| **Dívida estratégica** | + | − | Ganho hoje, dívida amanhã | Hack rápido hoje, refactor amanhã | **Repensar** |
| **Investimento** | − | + | Custo hoje, retorno no trimestre | Arquitetura limpa agora, escala depois | **Aceitar com nota** (ou repensar timing) |
| **Não fazer** | − | − | Perde em todas as escalas | Feature que não alinha com nada e atrasa a sprint | **Adiar/Rejeitar** |

### 6.2 Modificador H2 — o Sprint

H2 não define o quadrante (a matriz primária é H1×H3), mas **modifica a recomendação**:

| h2 | Efeito na recomendação |
|:--:|------------------------|
| **+** | Reforça: a decisão também protege a entrega da semana. Aceitar fica mais confortável. |
| **0** | Neutro: não pesa na recomendação. |
| **−** | **Atrito de sprint**: mesmo em "caminho livre" ou "investimento", a decisão deve ser **adiada ou fatiada** — não no meio da semana de entrega. Nota de scheduling obrigatória. |

### 6.3 Alertas do Detector

| Condição | Alerta | Severidade | Canal |
|----------|--------|:----------:|-------|
| `h1 > 0` E `h3 < 0` | ⚠️ **"Curto prazo vs longo prazo"** — o ganho de hoje paga a dívida de amanhã | 🟠 P1 | Avaliador + DDNA |
| `h2 < 0` E score ≥ +0.3 | ⚠️ "Atrito de sprint" — não executar no meio da semana | 🟡 P2 | Avaliador |
| `h3 ≤ −0.5` (qualquer h1/h2) | 🔴 "Longo prazo em risco crítico" — o peso 0.5 de H3 bloqueia aceitar | 🔴 P0* | Don (se decisão P1+) |
| `h1 < 0` E `h3 > 0` | 💎 "Investimento detectado" — custo imediato é capital intelectual (nota F2.1: ROI emerge do reuso) | 🟢 P3 | Log + DDNA |

*\*A decisão em si continua executável — o alerta é a escalação da recomendação, não um bloqueio (bloqueio só existe no Contrafactual Gate para P0/P1, §13).*

---

## 7. Recomendação — aceitar / adiar / repensar

### 7.1 Matriz de Recomendação

```
┌────────────────────────────────────────────────────────────────────────┐
│                                                                        │
│  horizon_score ≥ +0.3   E   h3 > 0    →  ✅ ACEITAR                   │
│                                                                        │
│  horizon_score ≥ +0.3   E   h3 < 0    →  ⚠️ REPENSAR (dívida)         │
│                                                                        │
│  +0.3 > score > −0.3                  →  🔍 REPENSAR (equilíbrio)     │
│                                                                        │
│  horizon_score ≤ −0.3                  →  🕓 ADIAR                     │
│                                                                        │
│  h3 ≤ −0.5  (sobrepõe qualquer score) →  ⚠️ REPENSAR — mínimo          │
│                                           (longo prazo domina, §3.3)   │
│                                                                        │
│  P0                                    →  ⚡ ACEITAR IMEDIATO (bypass, §8)│
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### 7.2 Interpretação dos Três Vereditos

| Veredito | Significado | O que acontece em seguida |
|----------|-------------|---------------------------|
| **✅ Aceitar** | O saldo entre as 3 escalas é positivo e sem conflito | Executa. Registra `horizon_impact` no DDNA. Sem gate adicional. |
| **🔍 Repensar** | Há conflito entre horizontes ou o saldo é ambíguo | Retorna ao avaliador com o **fator dominante** (qual horizonte pesou) e uma **ação de repensar** (§7.3). |
| **🕓 Adiar** | O saldo é negativo em mais de um horizonte, ou H3 crítico | Não executa agora. Reavalia no próximo ciclo/sprint. Registra motivo + gatilho de reavaliação. |

### 7.3 As Três Ações de "Repensar"

"Repensar" sem ação é burocracia. O engine sempre emite **uma** das três:

| Ação | Quando | Exemplo de execução |
|------|--------|---------------------|
| **Repensar escopo** | O conflito está na magnitude (h3 −0.5, mas o objetivo é certo) | Reduzir o escopo da decisão até o impacto em H3 ficar ≥ −0.3. |
| **Repensar timing** | O conflito está na janela (h2 < 0, ou h3 só é ruim AGORA) | Adiar a execução para o próximo ciclo/sprint — o problema não é a decisão, é o momento. |
| **Repensar abordagem** | O conflito está no método (h3 < 0 por como se faz, não pelo que se faz) | Mudar a forma de executar — ex: "implementar em paralelo com projetar" em vez de "parar de projetar". |

### 7.4 Escalação ao Don

| Situação | Ação |
|----------|------|
| Decisão P1+ com recomendação "repensar" após 2 ciclos de avaliação | Escalar ao Don com o histórico das 2 avaliações |
| Decisão com `h3 ≤ −0.5` que o avaliador insiste em executar | Escalar ao Don (nunca silenciar — registrar divergência no DDNA) |
| Recomendação "aceitar" com `h3_confidence < 0.5` (§10) | Escalar ao Don — aceitar sem confiança no longo prazo é aposta |

---

## 8. Regra P0 — Segurança Primeiro

> **Decisão P0 ignora o horizonte. Segurança primeiro.**

Incidentes de segurança, dados em risco ou dano ativo **não esperam** avaliação multi-horizonte. A regra é constitucional (P2 — código executado é a verdade absoluta; proteção de dados e segurança precedem planejamento):

```
SE p0 = true:
  1. BYPASS — a decisão executa imediatamente, sem recomendação
  2. SHADOW — o engine avalia mesmo assim, EM SEGUNDO PLANO:
     - registra horizon_impact no DDNA pós-fato
     - documenta o custo multi-horizonte da emergência
     - alimenta o CSV para o analytics (lição pós-incidente)
  3. Revisão — após a emergência, o DDNA é revisado:
     "O que a avaliação teria dito? A P0 era real ou evitável?"
     (alimenta o F9.4 gatilhos de evento e o F8.3 contradiction)
```

**Por que shadow, não ignorar**: a P0 é executada sem bloqueio (ninguém espera), mas o **custo** da emergência no horizonte longo é registrado — é o que transforma cada incidente em aprendizado (P5 — a família aprende com erros). Métrica de saúde: **taxa de bypass P0 < 5% das decisões** — acima disso, o sistema está operando em modo emergência crônico (§17).

---

## 9. Integração com F10.2 — Evolution Score

### 9.1 H3 consulta o termômetro

A avaliação de H3 (1-3 meses) não é vaga — ela consulta o [Evolution Score (F10.2)](../../analytics/evolution-score.md), o termômetro mensal do Don:

```
FONTES CONSULTADAS NA AVALIAÇÃO H3:
  • evolution-score.csv            → score atual + Δ vs mês anterior + projeção
  • evolution-score-report.md      → breakdown dos 5 componentes
  • breakdown do mês               → qual componente está MAIS LONGE do teto
```

### 9.2 Estimativa de Δ do score

A pergunta que H3 responde com o F10.2: **"Esta decisão melhora o evolution_score do próximo mês?"**

```
evolution_score_delta_est = f(componentes afetados pela decisão)

  Exemplo: decisão "criar DDNAs para os 4 gaps sem DDNA"
    → entropia 0.433 → 0.367  (componente mais longe do teto: contribui 8.5/15)
    → evolution_score: 79.4 → 80.4  → delta_est = +1.0 ✅
```

| Resultado do delta_est | Efeito na avaliação H3 |
|------------------------|------------------------|
| `delta_est > 0` | Reforça `h3_impact` positivo (decisão compra saúde mensal) |
| `delta_est ≈ 0` | Neutro — decisão não mexe no termômetro |
| `delta_est < 0` | Reforça `h3_impact` negativo (decisão degrada a saúde mensal) |

### 9.3 Alinhamento com o eixo de maior alavancagem

O breakdown do F10.2 identifica o componente mais distante do teto (no mês real de julho/2026: **entropia 0.433 → 8.5/15**, a alavanca de maior ROI por esforço). A regra de alinhamento:

```
SE a decisão ataca o componente mais distante do teto:
    → h3_impact += 0.2  (alignment bonus, documentado na saída)

SE a decisão IGNORA um componente em faixa crítica (< 50 no score):
    → nota obrigatória na avaliação H3
```

> **Relação bidirecional**: a F3.6 lê o F10.2 (componente do H3) e o F10.2 consome as avaliações da F3.6 (as decisões do mês com `h3_impact` negativo explicam quedas futuras do score — causalidade prospectiva). O F10.2 NUNCA recalcula o passado; a F3.6 é uma das fontes que explicam o presente.

---

## 10. Integração com F2.5 — Cognitive Momentum

### 10.1 Momentum alto amplia o horizonte

> **Momentum alto amplia o horizonte: consegue planejar mais longe.**

A integração com o [Cognitive Momentum (F2.5)](../cognitive-momentum/SKILL.md) usa o mesmo mapeamento já estabelecido do `momentum_multiplier` do F2.5 (P-ARCH-003 — componente derivado com a forma do Don preservada):

```
momentum_expansion = clamp(0.5 + momentum / 10, 0.5, 2.0)

efetiva_H3 = 90 dias × momentum_expansion
```

| momentum (F2.5) | estado | expansion | janela H3 efetiva | interpretação |
|:---------------:|:------:|:---------:|:-----------------:|---------------|
| **6.8** (sessão atual) | 🔥 aceleração | **1.18** | ~106 dias | Plataforma aprende rápido e na direção certa → projeta além do trimestre |
| 5.0 | 🔥 | 1.00 | 90 dias | Baseline (janela nominal) |
| 3.2 | 🟢 saudável | 0.82 | ~74 dias | Projeção ligeiramente encurtada |
| 1.2 | 🟡 desaceleração | 0.62 | ~56 dias | O futuro fica turvo |
| **0.25** | 🔴 estagnação | **0.53** | ~47 dias | H3 colapsa em direção a H2 — plataforma estagnada não sustenta planos de trimestre |

**Semântica**: a janela do calendário não muda (H3 continua sendo "1-3 meses" na escala nominal), mas a **janela de confiança** da projeção encolhe com a estagnação — o engine passa a enxergar "longo prazo" cada vez mais curto, porque uma plataforma que não aprende não tem trajetória para projetar.

### 10.2 Confiança da projeção H3

```
h3_confidence = clamp(0.5 + momentum / 10, 0.3, 1.0)
```

| momentum | h3_confidence | efeito |
|:--------:|:-------------:|--------|
| 6.8 | 1.0 | Projeção H3 confiável — roadmap com tração real |
| 3.2 | 0.82 | Projeção razoável |
| 1.2 | 0.62 | Projeção incerta — recomendações H3 marcadas |
| 0.25 | 0.53 | Projeção fraca |

Quando `h3_confidence < 0.5`:
- A avaliação H3 é marcada `low_confidence` na saída.
- A recomendação "aceitar" com H3 positivo é escalada ao Don (§7.4) — **aceitar sem confiança no longo prazo é aposta**.

### 10.3 Aviso de Estagnação

Quando o F2.5 reporta **estagnação** (`momentum < 0.5`, alerta P0 do próprio F2.5), o engine emite um aviso global em todas as avaliações:

```
"⚠️ Momentum estagnado (< 0.5). Projeções H3 com confiança reduzida.
 Decisões de longo prazo devem ser ADIADAS até a recuperação do momentum —
 primeiro restaurar a capacidade de aprender, depois planejar o trimestre."
```

**Racional**: decisões estratégicas tomadas durante estagnação acumulam erro porque a premissa ("a plataforma vai evoluir") não se sustenta. O F2.5 diz *onde* a plataforma está; o F3.6 ajusta *até onde* ela consegue enxergar a partir daí.

---

## 11. Integração com F3.3 — Cognitive Compression

### 11.1 Princípios comprimidos projetam mais longe

O [Cognitive Compression (F3.3)](../cognitive-compression/SKILL.md) é a **fonte de projeção H3**: princípios comprimidos (fidelity ≥ 0.90, 10:1 de compressão) capturam a **estrutura** da cadeia causal em vez de casos individuais — e estruturas projetam mais longe que exemplos.

```
AVALIAÇÃO H3 — HIERARQUIA DE FONTES:

  1º  Princípios comprimidos (knowledge/principles/CCP-NNN)   ← F3.3
  2º  Padrões compilados (patterns.md, F9.1)                  ← F9.1
  3º  Learnings brutos do domínio (learnings.md)              ← F1.4
  4º  Sem fonte → avaliação H3 marcada low_evidence
```

### 11.2 Por que a ordem importa

| Fonte | Custo | Alcance de projeção | Risco |
|-------|:-----:|:-------------------:|-------|
| Princípio comprimido (F3.3) | ~0.5KB | Projeta semanas-meses (estrutura causal) | Baixo (fidelity ≥ 90% garantida) |
| Padrão compilado (F9.1) | ~1KB | Projeta semanas | Médio |
| Learning bruto | ~1KB cada | Projeta dias (caso específico) | Alto (caso não generaliza) |

**Regra operacional**: se o domínio da decisão tem princípio comprimido disponível, a avaliação H3 **deve** usá-lo como base — é o conhecimento de maior densidade que a plataforma possui. Se não tem, a avaliação H3 fica mais cara e menos confiável (marcada). Isso cria o ciclo virtuoso: **comprimir hoje (F3.3) = decidir melhor no trimestre (F3.6)**.

### 11.3 Exemplo

Decisão "remover código de um provider". O princípio CCP-101 ("Sempre verificar `go list -deps` antes de remover código" — fidelity 0.93) projeta automaticamente a cadeia H3: remover sem verificar → código morto permanece → custo de manutenção cresce → entropia sobe → evolution_score cai no próximo mês. O learning bruto L29 ("`go build` não detecta código não-linkado") projeta apenas o Step 1. **O princípio é a fonte H3; o learning é a evidência.**

---

## 12. Integração com F1.1 — DDNA (campo horizon_impact)

### 12.1 O rastro do trade-off

Toda decisão avaliada pela F3.6 registra o resultado no [DDNA (F1.1)](../../knowledge/architecture/DECISION_DNA.md) — o campo **`horizon_impact`** adicionado ao frontmatter:

```yaml
---
id: DDNA-2026-07-30-003
title: "Decisão com trade-off multi-horizonte"
status: accepted
date: 2026-07-30
agents:
  - cosca-architecture
domain: architecture
decision_level: 3
confidence: 0.80
revisit: 2026-10-30
tags:
  - dna
  - cognitive-horizon
  - trade-off
supersedes:
superseded_by:
horizon_impact:                    # ★ F3.6 — avaliação multi-horizonte
  h1_impact: 0.8                   # -1..+1: efeito no horizonte operacional (0-1d)
  h2_impact: 0.2                   # -1..+1: efeito no horizonte tático (1-7d)
  h3_impact: -0.7                  # -1..+1: efeito no horizonte estratégico (1-3mo)
  horizon_score: -0.13             # (0.8×0.2) + (0.2×0.3) + (-0.7×0.5)
  tradeoff: "divida_estrategica"   # caminho_livre | divida_estrategica | investimento | nao_fazer
  recommendation: "repensar"       # aceitar | adiar | repensar
  recommendation_action: "repensar_abordagem"
  alert: "curto_prazo_vs_longo_prazo"
  momentum_expansion: 1.18         # F2.5 (momentum 6.8)
  h3_confidence: 1.0               # F2.5
  evolution_score_delta_est: -3.0  # F10.2 (impacto estimado no score do mês)
  p0: false                        # true = bypass (§8)
  shadow: false                    # true = avaliação pós-fato (P0 shadow)
---
```

### 12.2 Quando o campo é obrigatório

| Regra | Detalhe |
|-------|---------|
| **Obrigatório** | Decisão nível ≥ 2 **OU** trade-off explícito entre alternativas (DDNA já obrigatório nestes casos) |
| **Obrigatório** | Qualquer decisão em que o engine emitiu recomendação ≠ "aceitar" |
| **Recomendado** | Decisão nível 1 com impacto cruzado em 2+ horizontes |
| **Dispensado** | P0 em emergência ativa (o shadow avalia e preenche pós-fato) |

### 12.3 Por que registrar no DDNA

1. **Auditabilidade** (P3/P4): 6 meses depois, "por que aceitamos dívida estratégica em julho?" tem resposta com números.
2. **Revalidação** (revisit): na data de revalidação, o resultado real é comparado com a projeção — **a calibragem da F3.6 vem daqui** (recomendou "repensar" e a decisão foi aceita mesmo assim → deu certo ou errado?).
3. **Entropia**: decisões sem rastro são gaps (F1.6). O `horizon_impact` fecha o gap de decisão.
4. **F8.3 Contradiction**: uma decisão com `horizon_impact.tradeoff = "nao_fazer"` que foi executada de qualquer forma é evidência de contradição futura.

---

## 13. Posição no Pipeline de Decisão — F1.2 Contrafactual Gate

A F3.6 não substitui o [Contrafactual Gate (F1.2)](../../workflows/contrafactual-gate.md) — ela o **antecede**:

```
DECISÃO CANDIDATA (nível ≥ 2)
        │
        ▼
  ┌─────────────┐
  │  F3.6 HORIZON │  < 100ms — barato, roda SEMPRE
  │  avaliação    │
  └──────┬──────┘
         │
         ├── recomendação = "aceitar" (score ≥ +0.3, h3 > 0)
         │      └──► executa (com DDNA + horizon_impact)
         │
         ├── recomendação = "repensar"/"adiar" E decisão é P0/P1
         │      └──► F1.2 CONTRAFACTUAL GATE  (geração de alternativas)
         │              └──► deliberação completa antes de executar
         │
         └── recomendação = "repensar"/"adiar" E decisão é P2+
                 └──► retorna ao avaliador (repensar escopo/timing/abordagem)
                        └──► sem escalação ao gate — o filtro barato resolveu
```

**Divisão de trabalho**: o Horizon Engine é o **filtro barato universal** (todo nível ≥ 2, < 100ms, detecta o conflito de escalas). O Contrafactual Gate é a **deliberação cara seletiva** (P0/P1 com conflito ou incerteza, gera o "e se fosse diferente?"). Se o Horizon Engine resolveu com "repensar abordagem", o Gate nem precisa ser acionado — economia real no pipeline de decisão.

---

## 14. Armazenamento — cognitive-horizon.csv

### 14.1 Arquivo

```
internal/embed/cosca/memory/timeline/cognitive-horizon.csv
```

Mesma família de timeline dos CSVs do F1.5, F1.6 e F2.5 — permitindo análise temporal cruzada ("quando a entropia subiu, as decisões passaram a ter h3 mais negativo?").

### 14.2 Schema

```
timestamp,decision_id,title,h1,h2,h3,horizon_score,tradeoff,recommendation,alert,momentum_expansion,h3_confidence,evolution_delta_est,p0,shadow
```

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `timestamp` | ISO 8601 | Momento da avaliação |
| `decision_id` | string | ID da decisão (ou DDNA id futuro) |
| `title` | string | Título da decisão |
| `h1`, `h2`, `h3` | float | Impactos (-1..+1) |
| `horizon_score` | float | Score composto (-1..+1) |
| `tradeoff` | string | `caminho_livre` / `divida_estrategica` / `investimento` / `nao_fazer` |
| `recommendation` | string | `aceitar` / `adiar` / `repensar` |
| `alert` | string | Alerta emitido (ou `none`) |
| `momentum_expansion` | float | Expansão do F2.5 na avaliação |
| `h3_confidence` | float | Confiança da projeção H3 (F2.5) |
| `evolution_delta_est` | float | Δ estimado do evolution score (F10.2) |
| `p0` | bool | Decisão P0 (bypass) |
| `shadow` | bool | Avaliação shadow (P0 pós-fato) |

### 14.3 Linha de Exemplo (sessão atual)

```csv
timestamp,decision_id,title,h1,h2,h3,horizon_score,tradeoff,recommendation,alert,momentum_expansion,h3_confidence,evolution_delta_est,p0,shadow
2026-07-30T23:00:00Z,D-2026-07-30-001,parar de projetar engines e implementar tudo,0.8,0.2,-0.7,-0.13,divida_estrategica,repensar,curto_prazo_vs_longo_prazo,1.18,1.0,-3.0,false,false
```

### 14.4 Imutabilidade

O CSV é **append-only** — avaliações passadas nunca são regravadas (regra constitucional P4, mesmo contrato dos CSVs do F7.3/F2.5/F10.2). Correções de recomendação (o avaliador ignorou o engine) são registradas como **linha nova** com o outcome real, nunca como edição.

---

## 15. Exemplo Real — "Parar de projetar engines e implementar tudo"

### 15.1 Contexto (dados reais da sessão de 2026-07-30)

A sessão de arquitetura cognitiva projetou **14+ engines como specs** (F2.1-F2.6, F3.1-F3.3, F10.1-F10.4 em `engines/*/SKILL.md`) em um único dia, com **zero implementação em código Go** até aquele momento. Surge a decisão candidata:

> **"Parar de projetar engines e implementar tudo."**

O atrativo é real: o diretório `engines/` cresce, o código Go não muda, e a tentação de "chega de spec, vamos codar" é forte.

### 15.2 Avaliação Passo a Passo

```
PASSO 1 — DECISÃO: "Parar de projetar engines e implementar tudo" (p0=false, level 3)

PASSO 2 — H1 (0-1d): +0.8
  Tasks do dia andam mais rápido: código concreto entrega valor visível hoje.
  Evidência: diretório engines/ com specs-only; git status sem mudança em Go.

PASSO 3 — H2 (1-7d): +0.2
  A semana entrega mais código. Mas quality gates (cobertura, testes) não
  acompanham specs — risco de entregar código sem a rede de segurança.

PASSO 4 — H3 (1-3mo): −0.7
  • Engines projetadas e não implementadas = ESPECIFICAÇÃO ÓRFÃ
    (H-013 architecture-code gap: spec sem código = dívida de reconciliação)
  • F3.7 Ecosystem Dynamics depende de TODAS as engines operacionais —
    specs-only não acionam o ecossistema
  • Roadmap F3 (next-evolution-phases) ficaria com entregáveis não-operacionais
  • Refactor futuro para reconciliar spec × código custa ~3× o que custaria
    implementar agora em paralelo → "uma decisão rápida hoje custa 3x amanhã"
  • F10.2: parar de projetar deixa o roadmap (direction do F2.5) sem avanço →
    momentum cairia → evolution_score do próximo mês cai. delta_est = −3.0

PASSO 5 — horizon_score:
  0.8 × 0.2 = 0.16
  0.2 × 0.3 = 0.06
  −0.7 × 0.5 = −0.35
  ─────────────────
  horizon_score = −0.13

PASSO 6 — ALERTA:
  h1 > 0 (+0.8) E h3 < 0 (−0.7) → "⚠️ CURTO PRAZO vs LONGO PRAZO"

PASSO 7 — RECOMENDAÇÃO:
  score −0.13 (entre −0.3 e +0.3) E h3 < 0 → REPENSAR
  Ação: repensar_abordagem (o conflito está no MÉTODO, não no objetivo)
  → "implementar em paralelo com projetar"
```

### 15.3 Resultado

```
┌──────────────────────────────────────────────────────────────────────┐
│                                                                      │
│   COGNITIVE HORIZON — AVALIAÇÃO PRÉ-DECISÃO                          │
│   ═══════════════════════════════════                                 │
│                                                                      │
│   DECISÃO:  "Parar de projetar engines e implementar tudo"           │
│                                                                      │
│   H1 (0-1d):   +0.8   ██████████  (velocidade imediata)             │
│   H2 (1-7d):   +0.2   ███░░░░░░░  (mais código, gates defasados)     │
│   H3 (1-3mo):  −0.7   ░░░░░░░░░█  (specs órfãs, roadmap para,        │
│                                   F3.7 inoperante, refactor 3×)      │
│                                                                      │
│   HORIZON_SCORE = −0.13  🔍 REPENSAR                                 │
│   TRADE-OFF:     dívida estratégica  (h1 > 0, h3 < 0)                │
│   ALERTA:        ⚠️ CURTO PRAZO vs LONGO PRAZO                       │
│   AÇÃO:          repensar_abordagem                                  │
│   F2.5:          momentum 6.8 → expansion 1.18, h3_confidence 1.0    │
│   F10.2:         evolution_score 79.4 → delta_est −3.0 se aceita     │
│                                                                      │
│   VEREDITO: NÃO parar de projetar. IMPLEMENTAR EM PARALELO.          │
│   "Cada engine projetada é implementada no mesmo ciclo —             │
│    spec viva, não spec órfã."                                        │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### 15.4 Por que o exemplo é fiel à sessão

A recomendação **não é teórica — é a estratégia que a própria sessão adotou**: as engines F2.x e F10.x foram projetadas E seus pipelines especificados com CLI, config e armazenamento prontos para implementação imediata (SKILL.md operacional, não apenas conceitual). O `horizon_score` negativo em H3 (−0.35 de contribuição) é o que separa "especificação viva" (a caminho de virar código) de "especificação órfã" (que vira o architecture-code gap do H-013).

---

## 16. Edge Cases e Cold Start

| Cenário | Comportamento |
|---------|---------------|
| **Sem momentum medido (F2.5 cold start)** | `momentum_expansion = 1.0` (neutral, janela nominal 90 dias) e `h3_confidence = 0.5` — sem flag de estagnação. Avaliação prossegue com nota `momentum: unavailable`. |
| **Sem evolution score (F10.2 cold start)** | `evolution_delta_est = 0` (neutral) + nota. O H3 usa roadmap + princípios como fontes. |
| **Sem princípios comprimidos no domínio (F3.3 cold start)** | Avaliação H3 usa padrões F9.1 → learnings brutos; marcada `low_evidence` se nem padrões existirem. |
| **Avaliador sem evidência para um impacto ≥ 0.3** | Engine rejeita o input com pedido de evidência (estimativa sem evidência é chute — mesma postura do F10.3: "insight sem evidência é alucinação"). |
| **h1 = 0 e h3 = 0 (decisão neutra)** | Score = h2 × 0.3. Se h2 ≈ 0 → score ≈ 0 → "repensar" por indiferença: decisão sem impacto em nenhum horizonte provavelmente não precisa existir. |
| **Duas decisões em conflito no mesmo dia** | Cada uma avaliada independentemente; o CSV permite detectar que a soma das decisões do dia é negativa no H3 (alerta agregado diário). |
| **Decisão revertida após execução** | Nova linha no CSV com `recommendation=aceitar` + outcome real; a calibragem compara recomendação × resultado (§17). |
| **Recomendação ignorada pelo avaliador** | Executa mesmo assim — o engine nunca bloqueia (só o Contrafactual Gate bloqueia P0/P1). A divergência é registrada; se o resultado real der certo, o engine recalibra; se der errado, o DDNA tem o registro ("o engine avisou"). |
| **Burst de decisões (10+ no dia)** | Avaliações são independentes e < 100ms cada — sem gargalo. O relatório diário agrega a distribuição de trade-offs (quantas "dívidas estratégicas" o dia acumulou). |

---

## 17. Métricas do Próprio Engine

| Métrica | Definição | Alvo | Alerta |
|---------|-----------|:----:|--------|
| **Evaluation Latency** | Tempo total da avaliação | < 100ms | > 100ms = otimizar pipeline |
| **Evaluation Coverage** | Decisões nível ≥ 2 avaliadas / decisões totais | > 95% | < 80% = engine sendo ignorado |
| **Calibration Accuracy** | Recomendações corretas (outcome real confirma) / totais | > 80% | < 60% = recalibrar thresholds |
| **Dívida estratégica acumulada** | Σ de decisões `divida_estrategica` executadas | Decrescente | Crescente = sistema trocando futuro por presente |
| **False positive (repensar errado)** | Decisões "repensar" que deram certo como estavam | < 20% | > 30% = engine pessimista demais |
| **P0 bypass rate** | Bypasses P0 / decisões totais | < 5% | > 5% = modo emergência crônico |
| **DDNA coverage** | Decisões avaliadas com `horizon_impact` no DDNA | > 90% | < 70% = rastro quebrado |

### 17.1 Ciclo de Calibragem (mensal, junto com F10.2)

```
1. Para cada decisão do mês com recommendation registrada:
     comparar recommendation × outcome real (DDNA Outcome Validation)
2. Calcular acurácia por trade-off class:
     "repensar" acertou? "aceitar" acertou?
3. Ajustar thresholds (config, §18) — mudança vale para o futuro,
   NUNCA para o histórico (imutabilidade, §14.4)
4. Alimentar o analytics COGNITIVE_HORIZON.md:
     decisões do mês com h3 < 0 → esperado: HDI futuro menor
```

---

## 18. Regras Operacionais

1. **Toda decisão nível ≥ 2 passa pela avaliação** — o engine é o filtro barato universal do pipeline de decisão.
2. **H3 pesa mais (0.5)** — o longo prazo domina; `h3 ≤ −0.5` impede recomendação "aceitar".
3. **P0 ignora o horizonte** — segurança primeiro; avaliação roda em shadow pós-fato.
4. **Fórmula do Don é fixa** — `horizon_score = (h1×0.2) + (h2×0.3) + (h3×0.5)`. Thresholds e pesos de calibragem são config; a fórmula é lei.
5. **Estimativa sem evidência é chute** — impactos ≥ 0.3 exigem ≥ 1 evidência concreta.
6. **Nunca chama LLM no hot path** — avaliação < 100ms, pura agregação + aritmética.
7. **Trade-offs vão para o DDNA** — campo `horizon_impact` obrigatório quando recomendação ≠ "aceitar".
8. **O engine recomenda, não bloqueia** — execução da decisão permanece com o avaliador/Don; bloqueio é prerrogativa do Contrafactual Gate (P0/P1).
9. **Divergência é dado** — recomendação ignorada é registrada e alimenta a calibragem (se o avaliador acertou contra o engine, o engine aprende).
10. **Histórico imutável** — CSV append-only; recalibragem vale apenas para avaliações futuras.
11. **Coexistência respeitada** — este engine não duplica o analytics COGNITIVE_HORIZON.md (H0-H4, passos causais); o engine é prospectivo (H1-H3, escalas de tempo), o analytics é retrospectivo.
12. **Momentum modula, não substitui** — F2.5 expande a janela de confiança de H3; nunca altera os pesos da fórmula.

### 18.1 Configuração

```yaml
# cosca.config.yaml
cognitive_horizon:
  weights: { h1: 0.2, h2: 0.3, h3: 0.5 }      # fórmula do Don — fixa
  impact_scale: [-1.0, 1.0]
  momentum:
    window_days: 7                            # janela do F2.5 consultado
    expansion: { min: 0.5, neutral_at: 5.0, max: 2.0 }
    stagnation_threshold: 0.5                 # abaixo → aviso global (§10.3)
  f10_2:
    consult: true                             # lê evolution-score.csv + report
    alignment_bonus: 0.2                      # bonus do eixo de maior alavancagem
  recommendations:
    accept_at: 0.3
    defer_at: -0.3
    h3_block_threshold: -0.5                  # h3 ≤ este valor → nunca "aceitar"
  evaluation: on-decision                      # pré-decisão, não scheduled
  csv: "internal/embed/cosca/memory/timeline/cognitive-horizon.csv"
```

### 18.2 CLI

```bash
# Avaliar uma decisão (pré-decisão, < 100ms)
cosca cognitive-horizon evaluate --decision "adicionar engine X" --h1 0.5 --h2 0.3 --h3 -0.4

# Avaliar a partir de um DDNA proposto (lê o frontmatter)
cosca cognitive-horizon evaluate --ddna memory/decisions/ddna/DDNA-2026-07-30-003.md

# Ver status do engine (métricas §17)
cosca cognitive-horizon status

# Relatório diário/semanal de trade-offs (agregação do CSV)
cosca cognitive-horizon report --period week

# Calibragem mensal (compara recomendação × outcome real)
cosca cognitive-horizon calibrate --month 2026-07
```

---

## 19. Referências Cruzadas e Histórico

### 19.1 Referências

| Documento | Relação |
|-----------|---------|
| [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) §C8 | Conceito original de Cognitive Horizon |
| [COGNITIVE_HORIZON.md (analytics)](../../analytics/COGNITIVE_HORIZON.md) | Profundidade preditiva H0-H4 — coexistência §2 |
| [cognitive-momentum/SKILL.md](../cognitive-momentum/SKILL.md) | F2.5 — momentum expande a janela H3 (§10) |
| [cognitive-compression/SKILL.md](../cognitive-compression/SKILL.md) | F3.3 — princípios comprimidos como fonte de projeção H3 (§11) |
| [evolution-score.md](../../analytics/evolution-score.md) | F10.2 — H3 avalia o score do próximo mês (§9) |
| [DECISION_DNA.md](../../knowledge/architecture/DECISION_DNA.md) | F1.1 — campo `horizon_impact` (§12) |
| [contrafactual-gate.md](../../workflows/contrafactual-gate.md) | F1.2 — gate pós-horizon para P0/P1 (§13) |
| [cognitive-maturity-implementation.md](../../workflows/cognitive-maturity-implementation.md) F3.6 | Roadmap da tarefa |
| [next-evolution-phases.md](../../knowledge/architecture/next-evolution-phases.md) | Roadmap F3 (fonte do H3) |
| [H-013-architecture-code-gap.yaml](../../knowledge/heuristics/H-013-architecture-code-gap.yaml) | Heurística do exemplo real (§15) |
| [CONSTITUTION.md](../../CONSTITUTION.md) | P2 (código é a verdade), P3 (rastro), P4 (imutabilidade), P5 (aprender com erros) |
| [COGNITIVE_ECOSYSTEM.md](../../architecture/COGNITIVE_ECOSYSTEM.md) | Mapa do ecossistema (C8 → F3.6) |

### 19.2 Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Architecture Chief | Criação inicial — engine de avaliação multi-horizonte (H1 0-1d / H2 1-7d / H3 1-3mo), fórmula do Don `horizon_score = h1×0.2 + h2×0.3 + h3×0.5` (aditiva — contraste com P-ARCH-004), pipeline de 7 passos < 100ms, trade-off detector (4 quadrantes h1×h3 + modificador h2), recomendação aceitar/adiar/repensar com 3 ações, regra P0 (bypass + shadow), integrações F10.2 (delta do score mensal), F2.5 (momentum_expansion e h3_confidence), F3.3 (princípios como fonte H3), F1.1 (campo horizon_impact no DDNA), coexistência documentada com analytics COGNITIVE_HORIZON.md (P-ARCH-005), CSV append-only, exemplo real da sessão ("parar de projetar engines e implementar tudo" → dívida estratégica, score −0.13, recomendação: implementar em paralelo). |

---

> *"O presente vence no placar de hoje. O futuro vence no placar de sempre. O Cognitive Horizon é o árbitro que olha os dois placares ao mesmo tempo — e não deixa o jogo ser decidido só pelo primeiro tempo."*
>
> — Cosca Architecture Chief, 2026-07-30

> **Enforced by**: Cosca Architecture Chief | **Próxima calibragem**: 2026-08-30 (mensal, junto com F10.2)
> **Kernel instruction**: `cosca cognitive-horizon evaluate`
