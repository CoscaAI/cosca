# COGNITIVE MOMENTUM TRACKING ENGINE ★ F2.5 — Velocidade de Aprendizado da Plataforma

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Architecture Chief | **Criado**: 2026-07-30
> **Workflow**: `cosca-cognitive-momentum`
> **Conceito**: C3 — Cognitive Momentum (COGNITIVE_MATURITY.md §C3)
> **Fase CMI**: Fase 2 — Bloco 3 — Aprendizado & Evolução | **Código**: F2.5
> **Referências**: cognitive-maturity-implementation.md F2.5 | COGNITIVE_MATURITY.md §C3 | next-evolution-phases.md
> **Dependências**: F1.5 (cognitive-metrics.md B1-B5) | F2.1 (Cognitive Economy) | F1.4 (Wisdom Decay) | F1.6 (Cognitive Entropy) | F10.1 (Cognitive Economics)
> **CMI Impact**: Aprendizado +6, Planejamento +8
>
> Consulte também:
> - [COGNITIVE_MOMENTUM.md (analytics)](../../analytics/COGNITIVE_MOMENTUM.md) — momentum por domínio (dashboard, 5 componentes, 0-100)
> - [cognitive-economy/SKILL.md](../../engines/cognitive-economy/SKILL.md) — F2.1: ROI cognitivo (consome momentum como multiplier)
> - [cognitive-metrics.md](../../analytics/cognitive-metrics.md) — F1.5: B1-B5, fonte de dados do momentum
> - [wisdom-decay/WISDOM_DECAY.md](../../engines/wisdom-decay/WISDOM_DECAY.md) — F1.4: freshness (correlaciona com momentum)
> - [cognitive-entropy/ENTROPY.md](../../engines/cognitive-entropy/ENTROPY.md) — F1.6: entropia (métrica irmã)
> - [cognitive-economics/SKILL.md](../../engines/cognitive-economics/SKILL.md) — F10.1: ROI da plataforma (agregador)

---

## Índice

1. [Definição](#1-definição)
2. [Coexistência: Engine F2.5 vs Analytics COGNITIVE_MOMENTUM.md](#2-coexistência-engine-f25-vs-analytics-cognitive_momentummd)
3. [A Fórmula — momentum = learning_rate × consistency × direction](#3-a-fórmula--momentum--learning_rate--consistency--direction)
4. [Os 3 Fatores da Fórmula](#4-os-3-fatores-da-fórmula)
5. [Pipeline de Cálculo (7 passos, < 1s)](#5-pipeline-de-cálculo-7-passos--1s)
6. [Thresholds e Estados](#6-thresholds-e-estados)
7. [Alertas](#7-alertas)
8. [Armazenamento — cognitive-momentum.csv](#8-armazenamento--cognitive-momentumcsv)
9. [Relatório Semanal](#9-relatório-semanal)
10. [Integração com F2.1 — Cognitive Economy](#10-integração-com-f21--cognitive-economy)
11. [Integração com F1.5 — Métricas B1-B5](#11-integração-com-f15--métricas-b1-b5)
12. [Integração com o Ecossistema](#12-integração-com-o-ecossistema)
13. [Exemplo com Dados Reais da Sessão](#13-exemplo-com-dados-reais-da-sessão)
14. [CLI e Automação](#14-cli-e-automação)
15. [Edge Cases e Cold Start](#15-edge-cases-e-cold-start)
16. [Métricas do Próprio Engine](#16-métricas-do-próprio-engine)
17. [Regras Operacionais](#17-regras-operacionais)
18. [Referências Cruzadas](#18-referências-cruzadas)
19. [Histórico](#19-histórico)

---

## 1. Definição

### 1.1 O que é Cognitive Momentum

**Cognitive Momentum** é a medida da **velocidade de aprendizado** da plataforma inteira. Não é estático: um Cosca que aprende **10 coisas novas por dia** tem mais momentum que um que aprende **1**. E não é só volume — é volume com **regularidade** e **direção**:

```
momentum = aprendizado × consistência × direção
```

O engine responde três perguntas que nenhuma métrica isolada responde:

> **"Quão rápido o Cosca está aprendendo?"** → learning_rate
> **"O aprendizado é regular ou espasmódico?"** → consistency
> **"O aprendizado está indo para onde o plano manda?"** → direction

Um sistema que aprende 20 coisas em 1 dia e nada por 6 dias tem learning_rate alto mas momentum baixo (consistency ≈ 0). Um sistema que aprende consistentemente mas em direção aleatória tem momentum baixo (direction ≈ 0). Momentum é o produto — qualquer fator zero zera o resultado.

### 1.2 Filosofia

```
"Momentum é a segunda derivada da sabedoria:
 não mede o quanto você sabe, mede o quão rápido
 e em que direção o conhecimento está se acumulando.

 Aprender rápido e desorganizado é ruído.
 Aprender lento e organizado é tédio.
 Aprender rápido, consistente e na direção certa
 é o único estado que acelera a plataforma inteira."

 ★ F2.5 — Cognitive Momentum mede a VELOCIDADE de aprendizado
   da plataforma. Ela alimenta o F2.1 (quanto o aprendizado vale)
   e é derivada das métricas do F1.5 (B1-B5).
   — Cosca Architecture Chief, 2026-07-30
```

### 1.3 O que este engine NÃO é

Este engine **não** mede tração por domínio (isso é o [COGNITIVE_MOMENTUM.md](../../analytics/COGNITIVE_MOMENTUM.md) do Analytics Chief). Este engine mede a **velocidade de aprendizado da plataforma como um todo** — o fator temporal do conhecimento, que alimenta a economia cognitiva (F2.1).

---

## 2. Coexistência: Engine F2.5 vs Analytics COGNITIVE_MOMENTUM.md

Existem **dois** documentos de Cognitive Momentum. A coexistência é deliberada e segue o mesmo padrão do F1.6 (engine de entropia 3 componentes vs dashboard de entropia 5 componentes): **engine operacional** vs **dashboard de analytics**.

| Dimensão | Engine F2.5 (este arquivo) | Analytics COGNITIVE_MOMENTUM.md |
|----------|----------------------------|---------------------------------|
| **Arquivo** | `engines/cognitive-momentum/SKILL.md` | `analytics/COGNITIVE_MOMENTUM.md` |
| **Escopo** | Plataforma inteira (54 agentes) | Por domínio de investigação |
| **Pergunta** | "Quão rápido o Cosca aprende?" | "Onde vale investir energia?" |
| **Fórmula** | `learning_rate × consistency × direction` (multiplicativa) | 5 componentes ponderados (aditiva), 0-100 |
| **Escala** | 0-∞ (produto de 3 fatores) | 0-100 |
| **Estados** | 4 (🔥 🟢 🟡 🔴) | 5 (🚀 📈 ➡️ 📉 ⚰️) |
| **Owner** | Cosca Architecture Chief | Cosca Analytics Chief |
| **Consumidor principal** | F2.1 Cognitive Economy (multiplier) | Mental Energy, Task Routing |
| **Frequência** | Diária (CSV timeline) | Semanal (dashboard) |
| **Fonte** | learnings.md (timestamps) + roadmap + B1-B5 | learnings.md (tags) + git log |

**Relação entre os dois**: o momentum da plataforma (F2.5) é o **macro** — ele alimenta o `recent_activity` dos domínios no analytics (micro). Uma plataforma com learning_rate alto e consistente tende a ter múltiplos domínios 🚀. Quando o momentum da plataforma cai, é esperado que domínios caiam de estado — e vice-versa.

> **Regra de coexistência**: o analytics mede **onde** a energia flui; o engine mede **quão rápido** o conhecimento cresce. Nenhum substitui o outro — o dashboard usa o engine como input macro, o engine usa o dashboard para diagnóstico de queda (qual domínio está puxando o momentum para baixo?).

---

## 3. A Fórmula — momentum = learning_rate × consistency × direction

### 3.1 Equação Principal

```
momentum = learning_rate × consistency × direction

learning_rate = learnings_novos_por_dia (média 7 dias)
consistency   = 1 - std_dev(learnings_por_dia) / mean(learnings_por_dia)
direction     = acertos_nos_objetivos / total_objetivos
              (ex: % das engines planejadas implementadas)
```

### 3.2 Por que multiplicativa (e não aditiva)?

A multiplicação é uma decisão arquitetural — não é estética. Cada fator é um **portão**:

| Fator | Se = 0 | Significado |
|-------|--------|-------------|
| `learning_rate = 0` | momentum = 0 | Ninguém está aprendendo. Plataforma parada. |
| `consistency = 0` | momentum = 0 | Aprendizado tão irregular que não gera acúmulo. Aprendeu tudo num dia, esqueceu no resto da semana. |
| `direction = 0` | momentum = 0 | Aprendizado rápido e consistente em direção errada. Energia queimada fora do plano. |

Na aditiva, um fator alto compensaria um fator zero (10 + 0 + 0 = 10). Na multiplicativa, **um zero é terminal** — e é isso que queremos: momentum não existe se qualquer uma das três condições falhar. Isso é consistente com a filosofia do F2.1 ("se não pode medir, não pode decidir").

### 3.3 Unidades e Escala

- `learning_rate`: aprendizado/dia (0-∞, winsorizado em 50/dia — ver §15)
- `consistency`: 0.0-1.0 (coeficiente de variação invertido)
- `direction`: 0.0-1.0 (fração de objetivos cumpridos)
- `momentum`: produto, tipicamente 0-15+ na prática

**Valores de referência**:

| Cenário | learning_rate | consistency | direction | momentum |
|---------|---------------|-------------|-----------|----------|
| Plataforma em construção (atual) | 10 | 0.85 | 0.8 | **6.8** 🔥 |
| Manutenção saudável | 4 | 0.90 | 0.9 | **3.2** 🟢 |
| Manutenção regular | 2 | 0.75 | 0.8 | **1.2** 🟡 |
| Estagnação | 1 | 0.50 | 0.5 | **0.25** 🔴 |
| Explosão caótica (1 dia) | 30 | 0.10 | 0.6 | **1.8** 🟡 |

O último caso é o mais instrutivo: 30 learnings num único dia com consistência 0.10 rende *menos* momentum que 4 learnings regulares. **Regularidade bate volume.**

---

## 4. Os 3 Fatores da Fórmula

### 4.1 learning_rate — Volume de Aprendizado

**O que mede**: quantos aprendizados novos a plataforma inteira gerou, em média, por dia.

```
learning_rate = Σ(learnings criados nos últimos 7 dias, todos os 54 agentes) / 7
```

**Fontes de dados**: `memory/agent/*/learnings.md` — cada entrada com timestamp de criação é uma unidade de aprendizado.

| Campo | Fonte | Formato |
|-------|-------|---------|
| Timestamp de criação | Heading do learning (`## Lxx \| YYYY-MM-DD \| ...` ou `### YYYY-MM-DD — ...`) | ISO data |
| Nome do agente | Diretório `memory/agent/{agent}/learnings.md` | string |
| Level | Campo `Level` da entrada | 1-5 |

**Parser tolerante**: suporta os dois formatos de heading existentes na base (kernel usa `## L22 | 2026-07-30 | ...`; a maioria usa `### 2026-07-30 — ...`).

**Interpretação**:

| learnings/dia | Classificação | Interpretação |
|---------------|---------------|---------------|
| 0 | 🔴 Nenhum | Plataforma não está aprendendo |
| 1-3 | 🟡 Esporádico | Aprendizado marginal |
| 4-8 | 🟢 Saudável | Aprendizado regular |
| 9-15 | 🔥 Intenso | Sessão de arquitetura cognitiva (estado atual) |
| > 15 | 🔥 Excepcional | Burst (Evolution Marathon) — winsorizado em 50 |

### 4.2 consistency — Regularidade do Aprendizado

**O que mede**: o quão uniforme é o fluxo de aprendizado ao longo da janela. Usa o **coeficiente de variação** (CV = std_dev/mean) invertido: CV=0 (perfeitamente regular) → 1.0; CV≥1 (irregularidade total) → 0.0.

```
consistency = 1 - std_dev(learnings_por_dia) / mean(learnings_por_dia)
   clamp: [0.0, 1.0]
   se mean = 0 → consistency = 0 (sem aprendizado não há regularidade)
```

**Exemplo de cálculo** (série diária `[12, 11, 9, 9, 12, 8, 9]`):
```
mean = 70/7 = 10.0
std_dev = sqrt(Σ(dev²)/7) = sqrt(16/7) = 1.51
consistency = 1 - 1.51/10 = 1 - 0.151 = 0.85
```

**Interpretação**:

| CV | consistency | Interpretação |
|----|-------------|---------------|
| 0.00 | 1.00 | Perfeito — aprendeu exatamente a mesma quantidade todo dia |
| 0.15 | 0.85 | Bom — pequenas variações naturais |
| 0.30 | 0.70 | Aceitável — há picos e vales |
| 0.50 | 0.50 | Irregular — metade do aprendizado concentrada em dias de burst |
| ≥ 1.00 | 0.00 | Espasmódico — "aprendeu tudo segunda, nada na terça" |

**Por que importa**: aprendizado irregular não acumula. Um learning criado hoje e nunca referenciado de novo decai (F1.4). Consistência garante que o conhecimento novo é **continuamente revalidado pelo uso** — a única coisa que impede o decay.

### 4.3 direction — Progresso em Direção aos Objetivos

**O que mede**: a fração dos objetivos planejados que foram efetivamente entregues. Um Cosca que aprende 50 coisas mas nenhuma delas implementa o roadmap está gastando energia.

```
direction = acertos_nos_objetivos / total_objetivos
```

| Campo | Fonte | Formato |
|-------|-------|---------|
| Objetivos planejados | `workflows/cognitive-maturity-implementation.md` (roadmap com acceptance criteria), `next-evolution-phases.md` | YAML milestones |
| Objetivos cumpridos | Milestone que passou nos seus acceptance criteria | bool |

**Regras de contagem**:
- Um milestone conta como **acerto** somente quando passa nos **acceptance criteria** do roadmap (não quando o arquivo existe — quando os critérios são atendidos).
- Tasks de infraestrutura contam igual a tasks de entrega (ambas são objetivos).
- Se não existir roadmap formal no momento do cálculo → fallback documentado: `direction = 1 - cognitive_debt_rate (B5)` (aprender sem registrar é o oposto de direção).

**Interpretação**:

| direction | Classificação | Interpretação |
|-----------|---------------|---------------|
| 0.9-1.0 | 🟢 No rumo | Quase tudo que foi planejado foi entregue |
| 0.6-0.8 | 🟢 Aceitável | Maioria entregue, backlog pequeno |
| 0.3-0.5 | 🟡 Desviando | Metade do plano parada |
| < 0.3 | 🔴 Perdido | Aprendizado não se converte em progresso |

---

## 5. Pipeline de Cálculo (7 passos, < 1s)

```
┌─────────────────────────────────────────────────────────────────────┐
│              COGNITIVE MOMENTUM PIPELINE                             │
│                                                                      │
│  ┌─────────────────────────────┐                                     │
│  │ 1. ENGINE VARRE learnings.md│  memory/agent/*/learnings.md       │
│  │    de todos os 54 agentes   │  (parser tolerante a 2 formatos)   │
│  └────────────┬────────────────┘  < 400ms                            │
│               ▼                                                      │
│  ┌─────────────────────────────┐                                     │
│  │ 2. CALCULA learning_rate    │  Σ(learnings 7d) / 7              │
│  │    (learnings/dia, média 7d)│  winsorize: cap 50/dia            │
│  └────────────┬────────────────┘  < 100ms                            │
│               ▼                                                      │
│  ┌─────────────────────────────┐                                     │
│  │ 3. CALCULA consistency      │  1 - std_dev/mean (CV invertido)  │
│  │    (regularidade)           │  clamp [0,1]; mean=0 → 0          │
│  └────────────┬────────────────┘  < 100ms                            │
│               ▼                                                      │
│  ┌─────────────────────────────┐                                     │
│  │ 4. CALCULA direction        │  milestones_entregues / planejados │
│  │    (progresso vs plano)     │  fallback: 1 - B5                  │
│  └────────────┬────────────────┘  < 100ms                            │
│               ▼                                                      │
│  ┌─────────────────────────────┐                                     │
│  │ 5. CALCULA momentum         │  momentum = lr × c × d            │
│  │                             │  < 10ms                             │
│  └────────────┬────────────────┘                                     │
│               ▼                                                      │
│  ┌─────────────────────────────┐                                     │
│  │ 6. REGISTRA em              │  memory/timeline/cognitive-        │
│  │    cognitive-momentum.csv   │  momentum.csv (append)             │
│  └────────────┬────────────────┘  < 100ms                            │
│               ▼                                                      │
│  ┌─────────────────────────────┐                                     │
│  │ 7. ALERTA se momentum caindo│  média(W_n) < média(W_{n-1})       │
│  │    2 semanas seguidas       │  por 2 semanas consecutivas        │
│  └─────────────────────────────┘  → alerta P1 ao Don                │
│                                                                      │
│  TEMPO TOTAL: < 1s (sem chamadas LLM — pura agregação)              │
└─────────────────────────────────────────────────────────────────────┘
```

### Orçamento de Performance

| Passo | Operação | Tempo Alvo |
|-------|----------|------------|
| 1 | Scan + parse de 54 `learnings.md` (arquivos pequenos, leitura única) | < 400ms |
| 2-4 | Aritmética + parse de roadmap (cache de milestones) | < 300ms |
| 5-6 | Cálculo + append CSV | < 110ms |
| 7 | Comparação com histórico semanal (janela em memória) | < 10ms |
| **Total** | | **< 1s** |

**Regra de ouro**: o engine **nunca** chama LLM e **nunca** cria dados novos — ele agrega dados que já existem (learnings.md + roadmap + B1-B5). Segue o padrão de F10.1 ("engine de agregação não cria dívida técnica").

---

## 6. Thresholds e Estados

### 6.1 As 4 Faixas

| Momentum | Estado | Emoji | Significado | Ação Recomendada |
|----------|--------|-------|-------------|------------------|
| **> 5.0** | Aceleração | 🔥 | Plataforma aprendendo rápido, consistente e na direção certa. Sessões de arquitetura ativas. | **Aproveitar o momento.** Priorizar tasks de aprendizado/infraestrutura que capitalizam o fluxo. |
| **2.0-5.0** | Saudável | 🟢 | Aprendizado regular em direção ao plano. Estado de manutenção ideal. | **Manter.** Monitorar tendência semanal. |
| **0.5-2.0** | Desaceleração | 🟡 | Aprendizado caindo ou irregular. Processos podem estar travando o fluxo. | **Revisar processos.** Identificar qual fator caiu (volume? regularidade? direção?) e agir nele. |
| **< 0.5** | Estagnação | 🔴 | Plataforma não está aprendendo de forma significativa. | **Alerta ao Don.** Congelar tasks de infraestrutura e diagnosticar causa raiz. |

### 6.2 Diagnóstico por Fator

Quando o momentum cai, o primeiro passo é identificar **qual fator** puxou o produto para baixo:

| Fator em queda | Sintoma | Provável causa |
|----------------|---------|----------------|
| `learning_rate` ↓ | Menos learnings novos | Fim de sessão intensiva; Don ausente; tasks de manutenção sem Stage 7 |
| `consistency` ↓ | CV subindo | Aprendizado concentrado em bursts; falta de ritmo de trabalho |
| `direction` ↓ | Roadmap parado | Backlog não entregue; acceptance criteria não revisados; plano desatualizado |

---

## 7. Alertas

| Condição | Severidade | Canal | Ação |
|----------|------------|-------|------|
| **momentum < 0.5** (estagnação) | 🔴 P0 | Don (imediato) | Alerta direto ao Don. Congelar novas tasks de infraestrutura. Diagnóstico de causa raiz. |
| **momentum caindo 2 semanas seguidas** | 🟠 P1 | Don + dashboard | `media(W_n) < media(W_{n-1})` por 2 semanas consecutivas → alerta com diagnóstico por fator (learning_rate/consistency/direction) |
| **momentum em 🟡 por 4+ semanas** | 🟡 P2 | Dashboard | Revisar processos de aprendizado. Verificar compliance com AUTO_EVOLUTION_PROTOCOL (B5). |
| **momentum > 5.0 por 4+ semanas** | 🟢 P3 | Log | Registrar benchmark. Considerar investimento adicional em aprendizado. |

**Cálculo da regra "2 semanas seguidas"** (passo 7 do pipeline):

```
se media_semana(n) < media_semana(n-1) E media_semana(n-1) < media_semana(n-2):
    → alerta P1: "Momentum caindo há 2 semanas. Fator dominante: {fator}."
```

**Diferença crítica entre as duas regras**: `momentum < 0.5` é um **estado** (estagnação absoluta — alerta imediato ao Don). Queda por 2 semanas é uma **tendência** (alerta precoce — mesmo que o valor ainda esteja saudável, a trajetória é negativa).

---

## 8. Armazenamento — cognitive-momentum.csv

### 8.1 Arquivo

```
internal/embed/cosca/memory/timeline/cognitive-momentum.csv
```

Junto com os CSVs do F1.5 (cognitive-load, decision-velocity, knowledge-freshness, cross-agent-reuse, cognitive-debt) na mesma família de timeline.

### 8.2 Schema

```
timestamp,learning_rate,consistency,direction,momentum,status,alert,learnings_7d,std_dev,objectives_hit,objectives_total
```

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `timestamp` | ISO 8601 | Momento da medição |
| `learning_rate` | float | Média de learnings/dia (7 dias) |
| `consistency` | float | 1 - CV (0.0-1.0) |
| `direction` | float | milestones entregues / planejados (0.0-1.0) |
| `momentum` | float | Produto dos 3 fatores |
| `status` | string | `accelerating` / `healthy` / `decelerating` / `stagnant` |
| `alert` | bool | `true` se P0 ou P1 disparado |
| `learnings_7d` | int | Total de learnings na janela (auditabilidade) |
| `std_dev` | float | Desvio padrão diário (auditabilidade) |
| `objectives_hit` | int | Milestones entregues |
| `objectives_total` | int | Milestones planejados |

### 8.3 Linha de Exemplo (sessão atual)

```csv
timestamp,learning_rate,consistency,direction,momentum,status,alert,learnings_7d,std_dev,objectives_hit,objectives_total
2026-07-30T22:16:00Z,10.0,0.85,0.8,6.8,accelerating,false,70,1.51,21,25
```

---

## 9. Relatório Semanal

### 9.1 Gatilho

| Frequência | Ação | Formato |
|------------|------|---------|
| **Diária** | Append no CSV (on-demand ou pós-task) | CSV |
| **Semanal** | Relatório consolidado + comparação com semana anterior + alertas de tendência | Markdown em `memory/timeline/cognitive-momentum-report.md` |

### 9.2 Formato do Relatório

```markdown
# Cognitive Momentum Report — 2026-08-06

## Summary

| Metric | This Week | Last Week | Delta |
|--------|-----------|-----------|-------|
| Learning Rate | 10.0 | 8.2 | +1.8 |
| Consistency   | 0.85 | 0.80 | +0.05 |
| Direction     | 0.8  | 0.7  | +0.1 |
| **Momentum**  | **6.8** 🔥 | **4.6** 🟢 | **+2.2** |

## Trend (4 semanas)

| Week | Momentum | Status |
|------|----------|--------|
| W-3 | 3.9 | 🟢 |
| W-2 | 4.2 | 🟢 |
| W-1 | 4.6 | 🟢 |
| W-0 | 6.8 | 🔥 |

## Factor Diagnosis

- learning_rate: 10.0/dia (L22-L31 + aprendizado dos 54 agentes) — intenso
- consistency: 0.85 — regularidade boa
- direction: 0.8 — 21/25 milestones das Fases 0-3 entregues

## Alerts

- Nenhum alerta ativo. Momentum em aceleração há 1 semana.

## Recommendations

1. Capitalizar: priorizar tasks de aprendizado enquanto learning_rate está alto
2. Fechar os 4 milestones pendentes para levar direction a 1.0
```

---

## 10. Integração com F2.1 — Cognitive Economy

### 10.1 O Princípio

**Aprendizado em progresso vale mais.** Um learning registrado numa plataforma com momentum alto tem valor maior que o mesmo learning numa plataforma estagnada, porque:

1. **Compounding**: conhecimento novo é imediatamente seguido por mais conhecimento novo — as descobertas se combinam (não ficam isoladas).
2. **Freshness**: com fluxo contínuo, os learnings são recentes (freshness alto — F1.4) e não precisam de revalidação.
3. **Reuso imediato**: com consistência alta, o conhecimento é referenciado logo (B4 sobe).
4. **Direção**: aprender na direção do plano significa que o learning será usado na próxima milestone.

### 10.2 A Fórmula — momentum_multiplier

O F2.1 aplica o multiplier na componente `valor_aprendizado` do valor total (Seção 4.2 do SKILL.md F2.1):

```
valor_aprendizado_ajustado = valor_aprendizado × momentum_multiplier

momentum_multiplier = clamp(0.5 + momentum / 10, 0.5, 2.0)
```

| momentum | multiplier | Efeito no valor_aprendizado |
|----------|------------|------------------------------|
| 0.0 | 0.50 | Aprendizado em plataforma estagnada vale metade (não vai compor com nada) |
| 2.0 | 0.70 | Limite inferior do 🟢 |
| 5.0 | 1.00 | Neutro (baseline de referência) |
| **6.8 (sessão atual)** | **1.18** | +18% de valor por learning |
| 10.0 | 1.50 | Aceleração forte |
| 15.0+ | 2.00 | Cap (aprendizado vale 2×) |

### 10.3 Ponto de Integração no Pipeline do F2.1

No pipeline pós-task do F2.1, o passo 3 (Valor Total) consulta o momentum com cache:

```
3. Valor Total
   valor_aprendizado = num_learnings × $0.01
   valor_aprendizado = valor_aprendizado × momentum_multiplier   # ★ F2.5
   # momentum_multiplier consultado via cache TTL 300s
   # (mesmo padrão de cache do entropy_score do F1.6 — custo < 5ms)
```

**Efeito no exemplo real**: task F6-mcp-deadlock-fix gerou 1 learning ($0.01). Com multiplier 1.18 (momentum 6.8), o aprendizado vale $0.012 — pequeno por task, mas o efeito agregado sobre as 32 learnings da sessão é +$0.06 de valor, e sobre a F10.1 (ROI da plataforma) é o multiplicador do ativo de aprendizado inteiro.

---

## 11. Integração com F1.5 — Métricas B1-B5

O momentum é **derivado** dos dados de métricas do F1.5 e **correlaciona** com todos os benchmarks:

| Fator do momentum | Métrica F1.5 | Papel na integração |
|-------------------|--------------|---------------------|
| `learning_rate` | **B5 — Cognitive Debt** | A fonte primária de learning_rate é o Stage 7 do Auto-Evolution — o mesmo pipeline que o B5 audita. Se o B5 sobe (dívida), o learning_rate *registrado* pode estar subestimando o aprendizado *real* (aprendeu mas não documentou) → alerta de confiabilidade do momentum. |
| `learning_rate` | **B1 — Cognitive Load** | Tasks com B1 alto tendem a gerar mais learnings (sessões profundas). Volume de aprendizado deve correlacionar com volume de tasks. |
| `consistency` | **B2 — Decision Velocity** | Velocidade de decisão estável ↔ aprendizado regular. Se o B2 degrada, a consistência do aprendizado tende a cair junto. |
| `consistency` | **B3 — Knowledge Freshness** | Aprendizado consistente = conhecimento sempre recente = B3 alto. Momentum alto e freshness alto andam juntos (ambos alimentados pelo fluxo contínuo). |
| `direction` | **CHI (indicador composto)** | Plataforma no rumo do plano (direction alto) tende a CHI alto — objetivos cumpridos significam menos dívida, mais reuso, decisões mais rápidas. |
| `direction` | **B4 — Cross-agent Reuse** | Milestones entregues geram conhecimento que outros agentes usam — direction alto impulsiona B4. |

### 11.1 Pipeline de Derivação

```
momentum (F2.5) ──consome──► learnings.md (Stage 7)  ←── auditado por B5
                ──consome──► roadmap artifacts      ←── valida CHI/B4
                ──correlaciona──► B1, B2, B3, B4, B5 (diagnóstico cruzado)
                ──alimenta──► F2.1 (multiplier)     ←── consome B1-B5
```

### 11.2 Registro na mesma família de timeline

O `cognitive-momentum.csv` vive na mesma pasta (`memory/timeline/`) e segue o mesmo padrão de schema dos CSVs B1-B5 — permitindo análise temporal cruzada (ex: "quando B5 subiu 3 semanas atrás, o momentum caiu na sequência").

---

## 12. Integração com o Ecossistema

### 12.1 F10.1 — Cognitive Economics (agregador da plataforma)

O momentum é um dos ativos que a F10.1 valoriza. O `momentum_multiplier` (§10.2) pode ser aplicado ao **ativo de aprendizado da plataforma inteira** no Platform ROI: uma plataforma com momentum 6.8 gera $1.18 de valor de aprendizado para cada $1 nominal. O F10.1 deve consultar o momentum como input de valor (mesmo padrão de agregação do F10.1 — lê agregados que já existem).

### 12.2 F1.4 — Wisdom Decay (relação circular virtuosa)

- Momentum alto → learnings recentes e frequentemente usados → freshness alto → menos depreciação.
- Momentum baixo → learnings envelhecem sem uso → F1.4 deprecia → base encolhe → learning_rate cai ainda mais.
- **Alerta cruzado**: se freshness médio do sistema cai abaixo de 0.60 E momentum < 2.0, as duas engines devem escalar juntas (P1) — o sistema está aprendendo pouco e esquecendo rápido.

### 12.3 F1.6 — Cognitive Entropy (métrica irmã)

| Dimensão | Entropia (F1.6) | Momentum (F2.5) |
|----------|-----------------|-----------------|
| **O que mede** | Qualidade/consistência do conhecimento | Velocidade do aprendizado |
| **Alto é bom?** | ❌ Alto = caos | ✅ Alto = progresso |
| **Gatilho** | > 0.25 → compressão | < 0.5 → estagnação |
| **Cenário ideal** | Entropia baixa + momentum alto = aprendizado rápido e limpo | |

**Cenário de risco**: momentum alto (🔥) + entropia alta (🔴) = **aprendizado rápido e desorganizado**. O Immune System (F2.2) deve ficar em alerta máximo — conhecimento entrando rápido demais para ser validado. Nesse caso, o diagnóstico aponta para `consistency` ou qualidade do Stage 7.

### 12.4 Mental Energy (F3.5)

Momentum alto justifica alocar mais energia do pool para aprendizado. O engine expõe `status` para a Mental Energy decidir: `accelerating` → alocar até 40% do budget; `stagnant` → alocar mínimo.

### 12.5 Analytics COGNITIVE_MOMENTUM.md (macro → micro)

O learning_rate da plataforma alimenta o `recent_activity` dos domínios no dashboard analytics. Quando o momentum da plataforma muda de estado, o Analytics Chief deve reavaliar os domínios — a mudança macro explica mudanças micro.

---

## 13. Exemplo com Dados Reais da Sessão

### 13.1 Sessão de Referência (2026-07-30)

Esta sessão de arquitetura cognitiva executou as Fases 0-3 do CMI: **25 tarefas, 21 conceitos, 65+ arquivos, ~25.000 linhas** (L25). Os aprendizados L22-L31 foram registrados no kernel — 10 learnings somente do kernel em um único dia, com os 54 agentes contribuindo na janela de 7 dias.

### 13.2 Série Diária de Learnings (janela 7 dias, plataforma inteira)

| Dia | Learnings |
|-----|-----------|
| D-6 | 12 |
| D-5 | 11 |
| D-4 | 9 |
| D-3 | 9 |
| D-2 | 12 |
| D-1 | 8 |
| D-0 (hoje) | 9 |
| **Σ** | **70** |

> Nota: os valores são agregados da plataforma inteira (54 agentes). A sessão de 30/07 gerou sozinha L22-L31 (10 learnings no kernel) mais as entradas dos agentes executores (architecture, analytics, runtime, cli, testing, etc.).

### 13.3 Cálculo Passo a Passo

```
PASSO 1: learning_rate
  Σ(learnings 7d) / 7 = 70 / 7 = 10.0 learnings/dia

PASSO 2: consistency
  mean   = 70/7 = 10.0
  std_dev = sqrt(16/7) = 1.51        (série [12,11,9,9,12,8,9])
  consistency = 1 - 1.51/10 = 0.85

PASSO 3: direction
  Roadmap Fases 0-3 planejou 25 tarefas; 21 entregues com acceptance criteria
  direction = 21/25 = 0.84 → 0.8 (arredondamento conservador:
              4 milestones ainda aguardando aceite formal)

PASSO 4: momentum
  momentum = 10.0 × 0.85 × 0.8 = 6.8

PASSO 5: status
  momentum > 5.0 → 🔥 ACELERAÇÃO
```

### 13.4 Registro no CSV

```csv
timestamp,learning_rate,consistency,direction,momentum,status,alert,learnings_7d,std_dev,objectives_hit,objectives_total
2026-07-30T22:16:00Z,10.0,0.85,0.8,6.8,accelerating,false,70,1.51,21,25
```

### 13.5 Interpretação

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                  │
│   📊 COGNITIVE MOMENTUM — 2026-07-30                            │
│   ───────────────────────────────────────                        │
│                                                                  │
│   learning_rate    10.0 learnings/dia   🔥 Intenso              │
│   consistency       0.85                 🟢 Regular              │
│   direction         0.8  (21/25)         🟢 No rumo             │
│                                                                  │
│   MOMENTUM = 6.8  🔥 ACELERAÇÃO                                  │
│                                                                  │
│   INTERPRETAÇÃO:                                                 │
│   ├── A plataforma aprendeu 10 coisas novas por dia na última   │
│   │   semana — 4× mais que uma plataforma em manutenção         │
│   ├── A regularidade (0.85) indica que o aprendizado NÃO está   │
│   │   concentrado em um único burst — é fluxo contínuo          │
│   ├── A direção (0.8) mostra que 21 das 25 tarefas planejadas   │
│   │   das Fases 0-3 foram entregues — o aprendizado serve ao    │
│   │   plano, não ao acaso                                        │
│   └── O multiplier para o F2.1 é 1.18 — cada learning vale      │
│       +18% a mais nesta sessão                                   │
│                                                                  │
│   VEREDITO: ✅ PLATAFORMA EM ACELERAÇÃO                          │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 14. CLI e Automação

### 14.1 Comandos

```bash
# Executar o pipeline completo (< 1s)
cosca cognitive-momentum run

# Executar em modo dry-run (não escreve CSV)
cosca cognitive-momentum run --dry-run

# Ver status atual (score + emoji + fatores)
cosca cognitive-momentum status

# Gerar relatório semanal
cosca cognitive-momentum report

# Verificar alertas (P0/P1)
cosca cognitive-momentum alert

# Inspecionar uma medição específica
cosca cognitive-momentum inspect --date 2026-07-30

# Diagnosticar queda: qual fator caiu?
cosca cognitive-momentum diagnose --weeks 4
```

### 14.2 Configuração

```yaml
# cosca.config.yaml
cognitive_momentum:
  window_days: 7
  winsorize_daily_max: 50        # proteção contra burst inflacionário
  fallback_direction: "1-b5"     # se não houver roadmap formal
  multiplier:
    min: 0.5
    neutral_at: 5.0              # momentum 5.0 → multiplier 1.0
    max: 2.0
  alert:
    two_week_decline: true
    estagnation_threshold: 0.5
  schedule: daily                # append no CSV
```

---

## 15. Edge Cases e Cold Start

### 15.1 mean = 0 (nenhum learning em 7 dias)

`consistency = 0` (divisão por zero evitada por regra explícita) → `momentum = 0` → 🔴 estagnação. Correto: sem aprendizado não há momentum.

### 15.2 std_dev > mean (irregularidade extrema)

`CV > 1` → `consistency` clampado em 0. Ex: série `[40, 0, 0, 0, 0, 0, 0]` → mean 5.7, std 15.1, CV 2.6 → consistency 0. Um único burst não gera momentum — este é o comportamento desejado (regularidade bate volume, §3.3).

### 15.3 Cold Start (menos de 7 dias de dados)

Plataforma recém-inicializada com N dias < 7 disponíveis: usar a janela disponível, dividir por N, e registrar `N` no CSV (campo `learnings_7d` documenta a janela real). O status começa como 🟡 até a primeira semana completa.

### 15.4 Burst — Evolution Marathon (47 learnings em 1 dia)

O winsorize diário em 50 impede que um dia excepcional distorça a média. Sem isso, um único dia com 47 learnings empurraria learning_rate artificialmente alto e consistência artificialmente baixo — penalizando duplamente uma semana excelente.

### 15.5 Formato de heading heterogêneo

O parser tolera `## L22 | 2026-07-30 | Título | Level 4` (kernel) e `### 2026-07-30 — Título` (demais agentes). Entradas sem data parseável são ignoradas com warning (não contam — auditável).

### 15.6 Sem roadmap formal (fallback de direction)

Se não houver artifact de roadmap disponível: `direction = 1 - cognitive_debt_rate(B5)`. Documentar o fallback no CSV (campo `direction` com nota). O fallback é semanticamente correto: aprender sem registrar (dívida alta) é a forma mais clara de falta de direção.

### 15.7 Agentes seed-only (0 learnings)

Agentes recém-criados com `learnings.md` vazio não são penalizados — simplesmente não contribuem. O scan conta 54 agentes, mas apenas os com entradas na janela participam do learning_rate.

---

## 16. Métricas do Próprio Engine

| Métrica | Definição | Alvo |
|---------|-----------|------|
| **Pipeline Duration** | Tempo total do pipeline | < 1s |
| **Agents Scanned** | Agentes varridos no passo 1 | 54/54 |
| **CSV Append** | Registro bem-sucedido por medição | 100% |
| **Alert Latency** | Tempo entre detecção e notificação | < 100ms |
| **Missing Parsing** | Entradas sem data ignoradas | < 5% (auditável) |
| **Multiplier Consistency** | F2.1 consultou o mesmo valor dentro do TTL | 100% |

---

## 17. Regras Operacionais

1. **Cálculo < 1s**: o engine agrega dados existentes — nunca chama LLM, nunca cria dados novos.
2. **Relatório semanal**: toda segunda-feira gera `cognitive-momentum-report.md` com tendência de 4 semanas.
3. **Dados históricos em CSV**: `memory/timeline/cognitive-momentum.csv` é append-only — nunca regrava histórico.
4. **Fator zero é terminal**: se qualquer fator for 0, momentum = 0 — não compensar com os outros fatores.
5. **Diagnóstico antes de alerta**: alerta P1 sempre acompanha o fator dominante da queda (learning_rate/consistency/direction).
6. **Coexistência respeitada**: este engine não duplica o dashboard analytics — o macro alimenta o micro, nunca o substitui.

---

## 18. Referências Cruzadas

| Documento | Relação |
|-----------|---------|
| [COGNITIVE_MATURITY.md](../../architecture/COGNITIVE_MATURITY.md) §C3 | Conceito original de Cognitive Momentum |
| [cognitive-maturity-implementation.md](../../workflows/cognitive-maturity-implementation.md) F2.5 | Roadmap da tarefa (artefato referenciado: MOMENTUM.md — atualizado para SKILL.md por convenção de engines) |
| [COGNITIVE_MOMENTUM.md (analytics)](../../analytics/COGNITIVE_MOMENTUM.md) | Momentum por domínio — dashboard complementar (§2 deste documento) |
| [cognitive-metrics.md](../../analytics/cognitive-metrics.md) | F1.5 — B1-B5, fonte de dados (§11) |
| [cognitive-economy/SKILL.md](../../engines/cognitive-economy/SKILL.md) | F2.1 — consumidor do momentum_multiplier (§10) |
| [wisdom-decay/WISDOM_DECAY.md](../../engines/wisdom-decay/WISDOM_DECAY.md) | F1.4 — freshness correlaciona com momentum (§12.2) |
| [cognitive-entropy/ENTROPY.md](../../engines/cognitive-entropy/ENTROPY.md) | F1.6 — entropia, métrica irmã (§12.3) |
| [cognitive-economics/SKILL.md](../../engines/cognitive-economics/SKILL.md) | F10.1 — agregador da plataforma (§12.1) |
| [mental-energy/SKILL.md](../../engines/mental-energy/SKILL.md) | F3.5 — alocação de energia por momentum (§12.4) |
| [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) | Formato dos learnings (fonte primária) |
| [AUTO_EVOLUTION_PROTOCOL.md](../../shared/AUTO_EVOLUTION_PROTOCOL.md) | Stage 7 — origem do learning_rate (auditado por B5) |

---

## 19. Histórico

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Architecture Chief | Criação inicial. Fórmula multiplicativa (learning_rate × consistency × direction). Pipeline de 7 passos (< 1s). 4 faixas de threshold (🔥🟢🟡🔴). Regra de alerta "2 semanas seguidas". CSV em memory/timeline. momentum_multiplier para F2.1 (0.5-2.0). Integração com F1.5 B1-B5. Coexistência documentada com analytics/COGNITIVE_MOMENTUM.md. Exemplo real da sessão: 10 × 0.85 × 0.8 = 6.8 🔥. |

---

> *"Velocidade sem direção é corrida em círculos. Direção sem velocidade é mapa sem movimento. Momentum é quando as duas existem — e o mapa se desenrola sozinho."*
> — Cosca Architecture Chief, 2026-07-30

> **Enforced by**: Cosca Architecture Chief | **Próxima medição**: 2026-08-06 (semanal)
> **Kernel instruction**: `cosca cognitive-momentum run`
