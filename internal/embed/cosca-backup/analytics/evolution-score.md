# ENGINEERING EVOLUTION SCORE (F10.2)

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Analytics Chief | **Criado**: 2026-07-30
> **DNA Version**: 1.0.0 | **Engine**: `cosca-analytics`
>
> **Autoridade constitucional**: [CONSTITUTION.md](../CONSTITUTION.md) — Art. P2 (Código executado é a verdade absoluta) | Art. P4 (Tudo documentado, nada perdido — histórico imutável)
> **Integrações**: [engineering-score.md](engineering-score.md) (F7.3) | [cognitive-metrics.md](cognitive-metrics.md) (F1.5) | [cognitive-momentum/SKILL.md](../engines/cognitive-momentum/SKILL.md) (F2.5) | [cognitive-entropy/ENTROPY.md](../engines/cognitive-entropy/ENTROPY.md) (F1.6) | [cognitive-economics/SKILL.md](../engines/cognitive-economics/SKILL.md) (F10.1)
>
> O **Engineering Evolution Score** é o **termômetro mensal do Don**: um score único (0-100) que resume a saúde e a evolução da plataforma em um mês. Cada mês, cinco engines de saúde (Engenharia, Cobertura, Momentum, Entropia, Economia) são agregadas em **um número** — e a série histórica projeta o próximo mês. Janeiro 72 → Fevereiro 81 → Março 89: a evolução vira um gráfico, não uma sensação.

---

## Índice

1. [Definição](#1-definição)
2. [Fórmula Composta](#2-fórmula-composta)
3. [Normalização dos Componentes](#3-normalização-dos-componentes)
4. [Fontes de Dados](#4-fontes-de-dados)
5. [Pipeline Mensal](#5-pipeline-mensal)
6. [Thresholds e Interpretação](#6-thresholds-e-interpretação)
7. [Projeção — Regressão Linear](#7-projeção--regressão-linear)
8. [Armazenamento e Imutabilidade do Histórico](#8-armazenamento-e-imutabilidade-do-histórico)
9. [Breakdown Transparente](#9-breakdown-transparente)
10. [Integração com F10.1 — Cognitive Economics](#10-integração-com-f101--cognitive-economics)
11. [Integração com F2.5 — Cognitive Momentum](#11-integração-com-f25--cognitive-momentum)
12. [Integração com F7.3, F1.5 e F1.6](#12-integração-com-f73-f15-e-f16)
13. [Alertas](#13-alertas)
14. [Exemplo com Dados Reais — Julho/2026](#14-exemplo-com-dados-reais--julho2026)
15. [Edge Cases e Cold Start](#15-edge-cases-e-cold-start)
16. [Automação e CLI](#16-automação-e-cli)
17. [Regras do Engine](#17-regras-do-engine)
18. [Relacionamentos](#18-relacionamentos)
19. [HISTÓRICO](#19-histórico)

---

## 1. Definição

### 1.1 O que é o Engineering Evolution Score

O **Engineering Evolution Score** é um **score composto mensal (0-100)** que agrega as cinco dimensões de saúde da plataforma em um único número. Ele responde a pergunta que nenhuma métrica isolada responde:

> **"A plataforma está mais saudável este mês do que no mês passado?"**

Cada componente mede um eixo **ortogonal** de saúde:

| Eixo | Pergunta que responde | Engine de origem |
|------|-----------------------|------------------|
| **Engenharia** | "Os commits do mês foram tecnicamente sólidos?" | F7.3 Engineering Score (média do mês) |
| **Cobertura** | "O código crítico está protegido por testes?" | Benchmark de cobertura (`go test -cover`) |
| **Momentum** | "A plataforma está aprendendo rápido, regular e na direção certa?" | F2.5 Cognitive Momentum |
| **Entropia** | "A base de conhecimento está organizada ou caótica?" | F1.6 Cognitive Entropy |
| **Economia** | "Cada $1 investido gerou retorno cognitivo?" | F10.1 Cognitive Economics (Platform ROI) |

### 1.2 Filosofia

```
"Um projeto não tem uma saúde — tem cinco.
 Engenharia sem testes é frágil.
 Testes sem aprendizado é estático.
 Aprendizado sem direção é ruído.
 Direção sem retorno é vaidade.
 Retorno sem organização é dívida.

 O Engineering Evolution Score é o termômetro que junta
 os cinco em um único número — e a série histórica
 transforma o número em trajetória.

 ★ F10.2 — o Don olha uma vez por mês e sabe
   se a plataforma está evoluindo ou girando em falso."
 — Cosca Architecture Chief, 2026-07-30
```

### 1.3 O que este score NÃO é

- **NÃO é o score de um commit** — isso é o F7.3 (engineering-score.md), que roda pós-commit.
- **NÃO é o CMI (Cognitive Maturity Index)** — o CMI mede maturidade cognitiva estrutural; o F10.2 mede saúde de engenharia mensal.
- **NÃO é o Platform ROI** — isso é o F10.1; o ROI é apenas **um componente** (15%) do F10.2.
- **NÃO substitui nenhuma engine** — o F10.2 é pura agregação: lê os agregados que já existem e nunca cria dados novos.

### 1.4 Posição no Ecossistema

```
┌─────────────────────────────────────────────────────────────────────┐
│                    PIRÂMIDE DE SAÚDE DA PLATAFORMA                   │
│                                                                     │
│                        ┌─────────────────┐                          │
│                        │   F10.2          │   O termômetro do Don   │
│                        │ Evolution Score  │   (1 número / mês)      │
│                        │     0-100        │                         │
│                        └────────┬────────┘                          │
│               ┌─────────────────┼─────────────────┐                │
│               ▼                 ▼                 ▼                │
│      ┌───────────────┐ ┌───────────────┐ ┌───────────────┐        │
│      │ F7.3  Eng.    │ │ F2.5 Momentum │ │ F10.1  ROI    │        │
│      │ Score (30%)   │ │ (20%)         │ │ (15%)         │        │
│      └───────────────┘ └───────────────┘ └───────────────┘        │
│               ▲                 ▲                 ▲                │
│               │                 │                 │                │
│      ┌───────────────┐ ┌───────────────┐ ┌───────────────┐        │
│      │ Cobertura     │ │ F1.6 Entropia │ │ F1.5 B1-B5    │        │
│      │ de testes     │ │ (15%)         │ │ (dados-base)  │        │
│      │ (20%)         │ │               │ │               │        │
│      └───────────────┘ └───────────────┘ └───────────────┘        │
│                                                                     │
│  Cada componente é 0-1, multiplicado pelo peso, somado, ×100.       │
│  Nenhum componente sozinho define o mês — os cinco juntos.          │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. Fórmula Composta

### 2.1 Fórmula Canônica

```
evolution_score =
    engineering_score_avg × 0.30 +     (F7.3 — média do mês)
    coverage_score        × 0.20 +     (cobertura de testes)
    momentum_norm         × 0.20 +     (F2.5 — normalizado)
    (1 - entropy)         × 0.15 +     (F1.6 — organizado = 1, caótico = 0)
    roi_norm              × 0.15       (F10.1 — normalizado)
```

Cada componente é normalizado para **0.0 – 1.0**, multiplicado pelo peso, somado, e **×100** para a escala 0-100:

```
evolution_score = (Σ componente × peso) × 100
```

### 2.2 Pesos — Por que esses valores?

| Componente | Peso | Justificativa |
|------------|:----:|---------------|
| **Engineering Score (F7.3)** | 0.30 | **Maior peso** — commits são o artefato primário da plataforma. Um mês de commits ruins produz dívida técnica que contamina tudo o mais. |
| **Cobertura de testes** | 0.20 | Código sem teste é código sem rede de segurança. Cobertura alta sustenta a evolução rápida sem regressões. |
| **Momentum (F2.5)** | 0.20 | Aprendizado rápido, regular e direcionado é o combustível da plataforma. Momentum zero = plataforma parada. |
| **Entropia (F1.6)** | 0.15 | Conhecimento desorganizado reduz o valor de todo o resto. Entropia alta é o freio silencioso. |
| **ROI (F10.1)** | 0.15 | No final, a plataforma precisa gerar mais valor do que consome. O ROI é o balanço econômico do mês. |

**Relação com QUALITY_GATES.md**: a fórmula NOBLEIA (adapta) o `OVERALL` do Quality Gates e a fórmula do F7.3, mas em escala mensal — agrega dimensões de **health** (saúde do sistema), não de **quality** (qualidade de um artefato). Pesos foram calibrados para que nenhum componente sozinho domine: o máximo que um componente pode contribuir é 30 pontos (engineering) — o restante (70 pontos) depende dos outros quatro.

---

## 3. Normalização dos Componentes

Cada componente bruto é normalizado para **0.0 – 1.0** antes de aplicar o peso:

| Componente | Fonte bruta | Normalização | Exemplo |
|------------|-------------|--------------|---------|
| `engineering_score_avg` | F7.3 — média dos scores de todos os commits do mês (0-100) | `÷ 100` | 75.8 → 0.758 |
| `coverage_score` | Cobertura de testes do código crítico (0-100%) | `÷ 100` | 97.9% → 0.979 |
| `momentum_norm` | F2.5 — momentum do mês (produto `learning_rate × consistency × direction`) | `min(momentum / 10, 1.0)` | 6.8 → 0.68 |
| `entropy_term` | F1.6 — entropia do mês (0.0-1.0) | `1 - entropy` | 0.433 → 0.567 |
| `roi_norm` | F10.1 — Platform ROI do mês (percentual, ex: +337.6%) | `clamp(0.5 + roi_pct / 200, 0, 1)` | +337.6% → 1.0 |

### 3.1 Por que essas normalizações?

**`momentum_norm = min(momentum/10, 1.0)`** — O momentum do F2.5 é um produto sem limite superior teórico (típico 0-15+ na prática). O divisor 10 define o platô de saturação: momentum ≥ 10 (aprendizado excepcional sustentado) vale 1.0. Valores de referência do F2.5 §3.3: aceleração 6.8 → 0.68; manutenção saudável 3.2 → 0.32; estagnação 0.25 → 0.025 (≈ zero — correto: estagnação não contribui). O cap é documentado na configuração, não escondido na fórmula.

**`roi_norm = clamp(0.5 + roi_pct/200, 0, 1)`** — O break-even (ROI 0%) vale 0.5 (metade da escala — a plataforma se pagou mas não criou valor). ROI +100% vale 1.0 (saturação, alinhado com a interpretação do F10.1 §2.2: ">+100% = Excelente"). ROI -100% vale 0.0. Qualquer ROI acima de +100% satura em 1.0 — o F10.2 não distingue entre "+100%" e "+337%"; ambos são "excelente".

**`entropy_term = 1 - entropy`** — Inversão direta: entropia 0 (organizado) → 1.0; entropia 1.0 (caótico) → 0.0. Entropia atual 0.433 (🔴 alta) → 0.567 — o freio mais visível do score de julho (ver §14).

**`engineering_score_avg` e `coverage_score`** — Divisão direta por 100: ambos já nascem em escala 0-100.

---

## 4. Fontes de Dados

O F10.2 **não executa queries novas** — lê agregados já calculados pelas engines de origem:

| Componente | Fonte | Arquivo | O que lê | Formato |
|------------|-------|---------|----------|---------|
| **engineering_score_avg** | F7.3 | `analytics/engineering-score.md` + `memory/timeline/impact-reports/*.md` | Score de cada commit do mês | `**Score**: {n}/100` nos impact reports |
| **coverage_score** | Benchmark | `knowledge/best-practices/benchmarks/coverage-evolution.md` | Cobertura do código crítico (`go test -cover`) | `{n}.{n}%` |
| **momentum_norm** | F2.5 | `memory/timeline/cognitive-momentum.csv` | Média dos momentum diários do mês | CSV: `timestamp,momentum` |
| **entropy_term** | F1.6 | `memory/timeline/cognitive-entropy.csv` | Entropia do mês (última medição semanal) | CSV: `timestamp,entropy` |
| **roi_norm** | F10.1 | `engines/cognitive-economics/SKILL.md` §13 + `engines/cognitive-economics/reports/` | Platform ROI do mês | `Platform ROI = +{n}%` |

### 4.1 Agregação mensal

- **engineering_score_avg**: `mean(engineering_score de todos os commits com impact report no mês)`. Commits sem score registrado não entram no cálculo (são minoria — o hook pós-commit cobre a maioria).
- **coverage_score**: usa a medição de `go test -cover` do código crítico (`internal/runtime`), a mais recente do mês. Se o mês não teve medição, usa a última disponível (flag no relatório).
- **momentum_norm**: `mean(momentum diário do mês)` do CSV do F2.5. Se o CSV tem < 7 amostras no mês, usa a média disponível com flag `partial`.
- **entropy_term**: última medição semanal do mês (o F1.6 roda semanalmente). Se o mês não teve medição, usa a última disponível.
- **roi_norm**: ROI agregado do mês calculado pelo F10.1 (pipeline diário/semanal do F10.1 já agrega o mensal).

---

## 5. Pipeline Mensal

```
                      PIPELINE MENSAL — ENGINEERING EVOLUTION SCORE
  ════════════════════════════════════════════════════════════════════════
  Disparo: dia 1 do mês, 09:00 UTC (cron) | On-demand via CLI | < 5s total

  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
  │  1. COLETA   │───▶│  2. CALCULA  │───▶│  3. SCORE    │───▶│  4. COMPARA  │
  │  dados do    │    │  5 comp.     │    │  evolution_  │    │  com mês     │
  │  mês (todas  │    │  normalizados│    │  score       │    │  anterior Δ  │
  │  as fontes)  │    │  (0-1)       │    │  (0-100)     │    │              │
  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘    └──────┬───────┘
         │                   │                   │                   │
         ▼                   ▼                   ▼                   ▼
  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
  │ F7.3 impact     │ │ /100, /100,     │ │ Σ(comp×peso)    │ │ score(n) -      │
  │ reports do mês  │ │ min(m/10,1),    │ │ ×100            │ │ score(n-1)      │
  │ coverage bench  │ │ 1-entropy,      │ │                  │ │ Tendência: ↑ ↓ →│
  │ momentum CSV    │ │ clamp(roi)      │ │                  │ │                  │
  │ entropy CSV     │ │                 │ │                  │ │                  │
  │ F10.1 report    │ │                 │ │                  │ │                  │
  └─────────────────┘ └─────────────────┘ └─────────────────┘ └─────────────────┘
         │                   │                   │                   │
         ▼                   ▼                   ▼                   ▼
  ┌─────────────────┐ ┌─────────────────────────────────────────────────────┐
  │  5. PROJETA     │ │  6. RELATÓRIO EXECUTIVO                             │
  │  regressão      │ │  gera breakdown + Δ + projeção + alertas            │
  │  linear 6 meses │ │  → evolution-score.csv (append, imutável)           │
  │  → próximo mês  │ │  → evolution-score-report.md (regenerável)          │
  └─────────────────┘ │  → F10.1 relatório econômico (seção de evolução)    │
                      └─────────────────────────────────────────────────────┘
```

### Orçamento de Performance (< 5s)

| Passo | Operação | Tempo Alvo |
|-------|----------|------------|
| 1 | Leitura de agregados (impact reports, benchmark, 2 CSVs, report F10.1) | < 2.0s |
| 2 | Normalização dos 5 componentes | < 100ms |
| 3 | Cálculo do score (aritmética simples) | < 10ms |
| 4 | Comparação com mês anterior (Δ) | < 10ms |
| 5 | Regressão linear (6 pontos, least squares) | < 50ms |
| 6 | Geração do relatório executivo (template pré-compilado) | < 1.0s |
| **TOTAL** | | **< 3.2s** |

**Regra de ouro**: o F10.2 nunca chama LLM e nunca cria dados novos — apenas agrega agregados existentes (mesmo padrão do F10.1 e F2.5: "engine de agregação não cria dívida técnica").

---

## 6. Thresholds e Interpretação

| Score | Faixa | Classificação | Cor | Significado | Ação |
|:-----:|:-----:|:-------------:|:---:|-------------|------|
| **> 85** | 🔥 | **Excepcional** | 🟢 | Todos os 5 componentes fortes. Plataforma evoluindo na fronteira. | Aproveitar o momento. Investir em capacidade. |
| **70 – 85** | 🟢 | **Saudável** | 🟢 | Evolução consistente com 1-2 pontos de atenção. | Manter. Monitorar tendência. |
| **50 – 70** | 🟡 | **Atenção** | 🟡 | Um ou mais componentes em declínio. | Revisar. Identificar componente dominante da queda. |
| **< 50** | 🔴 | **Crítica** | 🔴 | Múltiplos componentes em colapso. | Alerta ao Don. Plano de recuperação obrigatório. |

### Interpretação por Faixa

#### 🔥 Excepcional (> 85)
Os 5 componentes convergem: commits de alta qualidade (F7.3 ≥ 85), cobertura forte (≥ 80%), momentum em aceleração (≥ 0.8), entropia baixa (≤ 0.20), ROI excelente (≥ +100%). É raro — exige que **todos** os eixos estejam saudáveis simultaneamente.

#### 🟢 Saudável (70-85)
Evolução consistente com pontos de atenção pontuais. O mês de julho/2026 (79.4) está aqui — cobertura e ROI excepcionais, mas entropia 🔴 puxando o score para baixo.

#### 🟡 Atenção (50-70)
Um eixo dominante está em declínio (ex: entropia alta recorrente, momentum caindo, cobertura em queda). O relatório aponta **qual componente** domina a perda — nunca alerta sem diagnóstico.

#### 🔴 Crítica (< 50)
Múltiplos eixos em colapso. O Don recebe alerta P0 com plano de recuperação e projeção de impacto de cada ação corretiva.

---

## 7. Projeção — Regressão Linear

### 7.1 Método

A projeção do próximo mês usa **regressão linear por mínimos quadrados** sobre os últimos 6 meses da série histórica:

```
y(x) = a + b·x

b = Σ(xᵢ − x̄)(yᵢ − ȳ) / Σ(xᵢ − x̄)²        (inclinação — tendência mensal)
a = ȳ − b·x̄                               (intercepto)
projeção(mês n+1) = a + b·(n+1)
```

Onde `x` é o índice do mês (1..6 para a janela de 6 meses) e `y` é o evolution_score.

### 7.2 Interpretação da Inclinação

| Inclinação `b` | Significado | Ação |
|----------------|-------------|------|
| **b > +1.0** | Aceleração forte (cada mês sobe > 1 ponto) | Investir em capacidade |
| **+0.2 a +1.0** | Crescimento sustentado | Manter estratégia |
| **-0.2 a +0.2** | Platô (estável) | Buscar próximo eixo de alavancagem |
| **-1.0 a -0.2** | Declínio gradual | Revisar componente dominante da queda |
| **b < -1.0** | Declínio acelerado | 🔴 Alerta — intervenção estrutural |

### 7.3 Limitações

- **Projeção ≠ previsão**: a regressão linear captura tendência, não sazonalidade nem eventos disruptivos (ex: uma onda de refatoração pode derrubar o score temporariamente).
- **Janela mínima**: com menos de 6 meses de histórico, a projeção é marcada como `partial` e usa a janela disponível (ver §15 Cold Start).
- **A projeção nunca altera o histórico**: valores passados são imutáveis (ver §8).

---

## 8. Armazenamento e Imutabilidade do Histórico

### 8.1 CSV — Fonte da Verdade (append-only)

```
internal/embed/cosca/memory/timeline/evolution-score.csv
```

**Schema**:

```
month,engineering_avg,coverage,momentum,entropy,roi_pct,evolution_score,delta,projection,status,alert
```

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `month` | `YYYY-MM` | Mês do score |
| `engineering_avg` | float | Média do F7.3 no mês (0-100) |
| `coverage` | float | Cobertura de testes (0.0-1.0) |
| `momentum` | float | Momentum médio do mês (F2.5) |
| `entropy` | float | Entropia do mês (F1.6, 0.0-1.0) |
| `roi_pct` | float | Platform ROI do mês (F10.1, %) |
| `evolution_score` | float | Score composto (0-100) |
| `delta` | float | Δ vs mês anterior |
| `projection` | float | Projeção para o próximo mês (regressão) |
| `status` | string | `exceptional` / `healthy` / `attention` / `critical` |
| `alert` | bool | `true` se P0/P1 disparado |

**Linha de exemplo** (julho/2026 — cálculo real na §14):

```csv
month,engineering_avg,coverage,momentum,entropy,roi_pct,evolution_score,delta,projection,status,alert
2026-07,75.8,0.979,6.8,0.433,337.6,79.4,-2.6,80.5,healthy,false
```

### 8.2 Imutabilidade — Regra Constitucional

> **Nunca recalcula meses passados.** O CSV é **append-only**: cada linha é escrita uma única vez, com a versão das regras usadas (registrada no relatório do mês). Se as regras mudarem (pesos, normalizações), a mudança gera novo DDNA e vale **apenas para meses futuros** — os meses passados permanecem como foram calculados. Isto preserva a comparabilidade da série histórica e a auditoria do "o que sabíamos e quando".

### 8.3 Relatório Mensal (derivado, regenerável)

```
internal/embed/cosca/memory/timeline/evolution-score-report.md
```

Gerado a partir do CSV + breakdown do mês. Regenerável a qualquer momento sem alterar o histórico (derivado ≠ fonte da verdade).

---

## 9. Breakdown Transparente

O Don vê os 5 componentes de cada mês — nenhum número é caixa-preta:

```yaml
evolution_score:
  month: 2026-07
  total: 79.4
  grade: 🟢 Saudável
  delta: -2.6 vs junho
  projection: 80.5 (agosto)
  breakdown:
    engineering:  { raw: 75.8,  norm: 0.758, weight: 0.30, contribution: 22.7 }
    coverage:     { raw: 0.979, norm: 0.979, weight: 0.20, contribution: 19.6 }
    momentum:     { raw: 6.8,   norm: 0.680, weight: 0.20, contribution: 13.6 }
    entropy:      { raw: 0.433, norm: 0.567, weight: 0.15, contribution: 8.5 }
    roi:          { raw: 337.6, norm: 1.000, weight: 0.15, contribution: 15.0 }
  formula: "(0.758×0.30) + (0.979×0.20) + (0.680×0.20) + (0.567×0.15) + (1.000×0.15) = 0.794"
  thresholds:
    is_exceptional: false   # > 85
    is_healthy: true        # 70-85
    is_attention: false     # 50-70
    is_critical: false      # < 50
  alert_don: false
```

### Visualização na Série Histórica

```
EVOLUTION SCORE — Últimos 7 meses
─────────────────────────────────────────────
 🔥 │            ╭──╮
 🟢 │      ╭─────╯  ╰──╮        ╭──╮
 🟡 │ ╭────╯            ╰──────╯  ╰────╮
 🔴 │─╯                                 ╰─────
    └────────────────────────────────────────
      Jan  Fev  Mar  Abr  Mai  Jun  Jul  Ago*
      (72) (81) (89) (86) (84) (82) (79.4) (80.5*)
                                            * projeção
```

---

## 10. Integração com F10.1 — Cognitive Economics

### 10.1 Fluxo F10.2 → F10.1

O `evolution_score` alimenta o **relatório econômico** do F10.1 como indicador de **saúde estrutural**:

- **No relatório mensal do F10.1**, o evolution_score entra como seção "Evolução da Plataforma":
  - Se `evolution_score ≥ 70`: a receita cognitiva do mês é **sustentável** (a plataforma está saudável para continuar gerando valor).
  - Se `evolution_score < 50`: a receita cognitiva do mês é **frágil** (mesmo com ROI alto, a base está deteriorando — o F10.1 recomenda congelar expansão até recuperação).
- **Na projeção econômica** do F10.1, a tendência do evolution_score (inclinação `b` da regressão) calibra o Platform ROI preditivo: plataforma em declínio estrutural tende a ROIs futuros menores.

### 10.2 Fluxo F10.1 → F10.2

O Platform ROI do F10.1 é **um componente direto** do evolution_score (`roi_norm × 0.15`). A relação é bidirecional e documentada nos dois lados:

| Direção | Dado | Uso |
|---------|------|-----|
| F10.1 → F10.2 | Platform ROI (%) | Componente `roi_norm` (15% do score) |
| F10.2 → F10.1 | Evolution Score + tendência | Seção de saúde estrutural no relatório econômico |

> **Padrão**: F10.1 é o CFO (balanço econômico), F10.2 é o termômetro (saúde mensal). O CFO consulta o termômetro para decidir se vale investir mais; o termômetro consulta o CFO para saber se o mês pagou as contas. (P-ARCH-005 — engine operacional + agregador estratégico.)

---

## 11. Integração com F2.5 — Cognitive Momentum

O momentum do F2.5 é **componente direto** do evolution_score:

```
momentum_norm = min(momentum_mensal / 10, 1.0)
```

- `momentum_mensal` = média dos momentum diários do mês (CSV `cognitive-momentum.csv`).
- Se o F2.5 reporta estagnação (< 0.5), o F10.2 penaliza o mês proporcionalmente (`momentum_norm` próximo de 0) — correto: plataforma que não aprende não evolui.
- Se o F2.5 reporta aceleração (≥ 5.0), o F10.2 premia até o teto (`momentum_norm` → 1.0).

**Retroalimentação**: quando o evolution_score cai em 🟡/🔴 por causa do momentum, o relatório F10.2 recomenda o diagnóstico por fator do próprio F2.5 (learning_rate / consistency / direction) — nunca alerta sem causa.

---

## 12. Integração com F7.3, F1.5 e F1.6

| Engine | Papel no F10.2 |
|--------|----------------|
| **F7.3 Engineering Score** | Componente `engineering_score_avg` (30%): média mensal de todos os scores de commit. Fonte: impact reports gerados pelo hook pós-commit. |
| **F1.5 Cognitive Metrics (B1-B5)** | Dados-base indiretos: B3 (freshness) alimenta a entropia (F1.6); B4/B5 (reuso/prevenção) alimentam o ROI (F10.1). O F10.2 não lê B1-B5 diretamente — herda por cadeia. |
| **F1.6 Cognitive Entropy** | Componente `entropy_term` (15%): `1 - entropy`. A entropia é o freio mais sensível do score — reduzir entropia é a alavanca de maior ROI por esforço (ver §14.4). |

---

## 13. Alertas

| Condição | Severidade | Canal | Ação |
|----------|------------|-------|------|
| **score < 50** | 🔴 P0 | Don (imediato) | Plano de recuperação obrigatório. Congelar expansão. |
| **score caindo 2 meses seguidos** | 🟠 P1 | Don + dashboard | Alerta de tendência com componente dominante da queda (sempre com diagnóstico). |
| **score 50-70** | 🟡 P2 | Dashboard | Revisão recomendada. Relatório aponta eixo dominante. |
| **score > 85** | 🟢 P3 | Log | Registrar benchmark. Considerar investimento em capacidade. |
| **entropy > 0.25 no breakdown** | 🟡 P2 | Relatório | Nota automática: "Entropia alta — criar DDNAs para gaps é a alavanca de maior impacto" (ver §14.4). |

**Regra das duas semanas do F2.5 adaptada ao mês**: `score(n) < score(n-1) E score(n-1) < score(n-2)` → alerta P1 de tendência. Um mês ruim é ruído; dois meses seguidos é padrão.

---

## 14. Exemplo com Dados Reais — Julho/2026

### 14.1 Contexto da Sessão

Dados reais da sessão **Evolution Marathon** (28-30 de julho de 2026), coletados das 5 engines de origem:

| Componente | Valor real | Fonte |
|------------|-----------|-------|
| **engineering_score_avg** | **75.8** | Média de 8 commits com score real no mês: `1541332`=72, `52435c5`=67, `5abe0da`=77, `85928db`=75, `85f2265`=75, `ac013d5`=75, `e6c3ddf`=86, `792c000`=79.5 → 606.5/8 = **75.8** |
| **coverage_score** | **97.9%** | Cobertura do core `internal/runtime` (coverage-evolution benchmark 2026-07-29; learning L20 do Kernel) |
| **momentum** | **6.8** | F2.5 — `10.0 × 0.85 × 0.8 = 6.8` 🔥 aceleração |
| **entropy** | **0.433** | F1.6 — `(1×3 + 0×2 + 4×1) / 30 = 7/30` 🔴 alta (1 contradição ativa, 4 gaps sem DDNA) |
| **roi_pct** | **+337.6%** | F10.1 — `($269.00 / $61.47) - 1` 🟢 excelente (cada $1 gerou $4.38) |

### 14.2 Cálculo Passo a Passo

```
PASSO 1 — Normaliza os 5 componentes (0-1):
  engineering_avg_norm = 75.8 / 100            = 0.758
  coverage_norm        = 97.9 / 100            = 0.979
  momentum_norm        = min(6.8 / 10, 1.0)    = 0.680
  entropy_term         = 1 - 0.433             = 0.567
  roi_norm             = clamp(0.5 + 337.6/200, 0, 1) = 1.000

PASSO 2 — Aplica pesos e soma:
  0.758 × 0.30 = 0.2274   (engineering)
  0.979 × 0.20 = 0.1958   (coverage)
  0.680 × 0.20 = 0.1360   (momentum)
  0.567 × 0.15 = 0.0851   (entropy)
  1.000 × 0.15 = 0.1500   (roi)
  ─────────────────────
  Σ            = 0.7943

PASSO 3 — Escala 0-100:
  evolution_score = 0.7943 × 100 = 79.4
```

### 14.3 Resultado

```
┌──────────────────────────────────────────────────────────────────────┐
│                                                                      │
│   ENGINEERING EVOLUTION SCORE — Julho/2026                           │
│   ══════════════════════════════════════                              │
│                                                                      │
│   SCORE:  79.4  🟢 SAUDÁVEL    Δ -2.6 vs junho (82.0)               │
│                                                                      │
│   Breakdown:                                                         │
│   ─────────────────────────────────────────────────────              │
│   Engineering (F7.3):   75.8 → 0.758 × 0.30 =  22.7   ██████████░   │
│   Cobertura:            97.9 → 0.979 × 0.20 =  19.6   ██████████░   │
│   Momentum (F2.5):       6.8 → 0.680 × 0.20 =  13.6   ██████░░░░░   │
│   Entropia (F1.6):      0.433 → 0.567 × 0.15 =   8.5   ████░░░░░░░ ⚠ │
│   ROI (F10.1):        +337.6 → 1.000 × 0.15 =  15.0   ████████████  │
│   ─────────────────────────────────────────────────────              │
│                                        TOTAL =  79.4                 │
│                                                                      │
│   Thresholds:                                                        │
│   ├── > 85  Excepcional?  ❌                                        │
│   ├── 70-85 Saudável?     ✅ (está aqui)                            │
│   ├── 50-70 Atenção?      ❌                                        │
│   └── < 50  Crítica?      ❌                                        │
│                                                                      │
│   Projeção (regressão 6 meses):  Agosto ≈ 80.5 🟢                    │
│   Alerta ao Don? ❌ (score ≥ 50, tendência sem 2 quedas seguidas)    │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### 14.4 Análise do Exemplo — O Freio da Entropia

O score de **79.4 (Saudável)** conta uma história precisa sobre julho:

- **Forças**: Cobertura (19.6/20 — core em 97.9%) e ROI (15/15 — recorde de +337.6%) operam no teto. A média de engenharia (75.8) é sólida para um mês com 22 commits de features pesadas.
- **O freio**: a **entropia 0.433 (🔴)** contribui apenas 8.5 dos 15 pontos possíveis do eixo — é o componente mais distante do teto. O momentum (13.6/20) também tem folga (6.8 é forte, mas abaixo da saturação 10).
- **Alavanca de maior ROI**: criar DDNAs para os 4 gaps sem DDNA (F1.6) reduz a entropia de 0.433 → 0.367, elevando o score para **80.4** (+1.0). Se a entropia cair para o limiar 🟡 (0.25), o score sobe para **82.4**. A ação é documentação (barata) com impacto direto no termômetro.
- **Momentum em 13.6/20**: julho aprendeu 10 coisas/dia com consistência 0.85 e direção 0.8 — forte, mas o teto (10/dia → saturação) deixa espaço. Manter a regularidade é o que impede o decay (F1.4).

### 14.5 Série Histórica e Projeção — Exemplo de Regressão

Série usada para a regressão (meses com `*` são ilustrativos; **julho é real**):

| Mês | Score | Fonte |
|-----|:-----:|-------|
| Fev | 72 | Ilustrativo (missão) |
| Mar | 81 | Ilustrativo (missão) |
| Abr | 89 | Ilustrativo (missão) |
| Mai | 86* | Ilustrativo |
| Jun | 82* | Ilustrativo |
| Jul | **79.4** | **Real (calculado neste documento)** |

```
Regressão linear (x = 2..7, y = score):
  x̄ = 4.5, ȳ = 83.57
  b = Σ(x−x̄)(y−ȳ) / Σ(x−x̄)² = −15.5 / 17.5 = −0.886
  a = ȳ − b·x̄ = 83.57 + 3.99 = 87.55

  Projeção Agosto (x = 8): y = 87.55 + (−0.886 × 8) = 80.5 🟢
```

**Interpretação**: a inclinação `b = −0.886` indica declínio gradual (entre −1.0 e −0.2) — a série saiu do pico de março (89) e está se estabilizando; a projeção para agosto (80.5) sugere estabilização no patamar saudável, **desde que a entropia seja tratada**. Sem a ação da §14.4, a tendência continua negativa.

---

## 15. Edge Cases e Cold Start

| Cenário | Comportamento |
|---------|---------------|
| **Menos de 6 meses de histórico** | Regressão usa a janela disponível; projeção marcada como `partial` no CSV. Score calculado normalmente a partir do 1º mês. |
| **Mês sem commits** | `engineering_score_avg` = último valor disponível (flag `carry_forward` no relatório). |
| **Mês sem medição de cobertura** | `coverage_score` = última medição disponível. |
| **Mês sem medição de momentum** | `momentum_norm` = último valor disponível; se nunca medido, neutral 0.5 (mom. 5.0). |
| **Mês sem medição de entropia** | `entropy_term` = última medição; se nunca medido, neutral 0.5 (entropy 0.5). |
| **Mês sem ROI (F10.1 não rodou)** | `roi_norm` = neutral 0.5 (break-even). |
| **Componente neutro = não medido** | Todo componente ausente usa o **ponto médio da escala** (0.5) para não inflar nem penalizar artificialmente — e é sempre **flagado** no relatório. Preferência de fallback: (1) último valor real, (2) neutral 0.5, (3) flag. |
| **Burst excepcional (47 learnings em 1 dia)** | O winsorize do F2.5 já protege o momentum; o F10.2 herda a proteção sem ação extra. |
| **Mudança de regras (pesos/normalização)** | Gera DDNA, vale apenas para meses futuros. Histórico permanece imutável (ver §8.2). |

---

## 16. Automação e CLI

### 16.1 Comandos

```bash
# Executar o pipeline mensal completo (< 5s)
cosca analytics evolution run [--month 2026-07]

# Ver status do mês atual + breakdown
cosca analytics evolution status

# Ver série histórica (CSV)
cosca analytics evolution history

# Gerar projeção do próximo mês
cosca analytics evolution project [--months 6]

# Gerar relatório executivo
cosca analytics evolution report
```

### 16.2 Configuração

```yaml
# cosca.config.yaml
evolution_score:
  weights: { engineering: 0.30, coverage: 0.20, momentum: 0.20, entropy: 0.15, roi: 0.15 }
  momentum_cap: 10.0            # momentum 10 → momentum_norm 1.0
  roi_break_even_pct: 0.0       # ROI 0% → roi_norm 0.5
  roi_full_pct: 100.0           # ROI +100% → roi_norm 1.0 (saturação)
  regression_window: 6          # meses da regressão linear
  schedule: "0 9 1 * *"         # dia 1 do mês, 09:00 UTC
  alert:
    critical_threshold: 50
    attention_threshold: 70
    two_month_decline: true
```

### 16.3 Cron / Scheduler

```
0 9 1 * * cosca analytics evolution run
```

---

## 17. Regras do Engine

1. **Score mensal, cálculo < 5s**: agrega dados existentes — nunca chama LLM, nunca cria dados novos.
2. **Histórico imutável**: CSV append-only; meses passados nunca são recalculados (regra constitucional — Art. P4).
3. **Breakdown transparente**: o Don vê os 5 componentes, pesos e contribuições de cada mês.
4. **Componente ausente ≠ zero**: usa último valor real, depois neutral 0.5, sempre com flag no relatório.
5. **Projeção ≠ previsão**: a regressão é tendência, marcada como `partial` quando a janela < 6 meses.
6. **Alerta com causa**: qualquer alerta P0/P1 acompanha o componente dominante da queda — nunca alerta sem diagnóstico.
7. **Mudança de regras gera DDNA**: pesos e normalizações são configuração, e mudanças valem apenas para meses futuros.
8. **Don override**: o Don pode ajustar o score de um mês (ex: mês com evento extraordinário) via `cosca analytics evolution run --override {score} --reason "..."`. O valor original é preservado no CSV; o override é registrado com rationale.

---

## 18. Relacionamentos

| Documento | Relação |
|-----------|---------|
| [engineering-score.md](engineering-score.md) | F7.3 — fonte do componente `engineering_score_avg` (30%). |
| [cognitive-metrics.md](cognitive-metrics.md) | F1.5 — B1-B5, dados-base indiretos (via F1.6 e F10.1). |
| [cognitive-momentum/SKILL.md](../engines/cognitive-momentum/SKILL.md) | F2.5 — componente `momentum_norm` (20%). |
| [cognitive-entropy/ENTROPY.md](../engines/cognitive-entropy/ENTROPY.md) | F1.6 — componente `entropy_term` (15%). |
| [cognitive-economics/SKILL.md](../engines/cognitive-economics/SKILL.md) | F10.1 — componente `roi_norm` (15%) e consumidor do score no relatório econômico. |
| [coverage-evolution.md](../knowledge/best-practices/benchmarks/coverage-evolution.md) | Fonte da cobertura de testes (core 97.9%). |
| [next-evolution-phases.md](../knowledge/architecture/next-evolution-phases.md) | Roadmap F10 — artefato `analytics/evolution-score.md` (entregável da fase). |
| [ENGINEERING_TIMELINE.md](../memory/timeline/ENGINEERING_TIMELINE.md) | Registro dos commits que alimentam o `engineering_score_avg`. |
| [QUALITY_GATES.md](../QUALITY_GATES.md) | Base conceitual dos pesos (nobreados, não duplicados). |
| [COGNITIVE_MATURITY.md](../architecture/COGNITIVE_MATURITY.md) | CMI mede maturidade cognitiva estrutural; F10.2 mede saúde mensal — complementares, não redundantes. |
| [DECISION_DNA.md](../knowledge/architecture/DECISION_DNA.md) | Mudanças de regras do F10.2 geram DDNA. |

---

## 19. HISTÓRICO

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Architecture Chief | Criação inicial — score composto mensal (0-100), fórmula com 5 componentes normalizados (F7.3 30%, cobertura 20%, F2.5 20%, F1.6 15%, F10.1 15%), pipeline mensal em 6 passos < 5s, thresholds 🔥🟢🟡🔴, projeção por regressão linear (6 meses), CSV imutável, breakdown transparente, integrações F10.1 (relatório econômico) e F2.5 (componente direto), exemplo real de julho/2026 (score 79.4 🟢, projeção agosto 80.5). |

---

> **Enforced by**: Analytics Chief + scheduler mensal | **Próximo cálculo**: 2026-08-01 (agosto)
> **Dependências**: F7.3 (engineering-score.md), F1.5 (cognitive-metrics.md), F2.5 (cognitive-momentum/SKILL.md), F1.6 (cognitive-entropy/ENTROPY.md), F10.1 (cognitive-economics/SKILL.md) — todas ativas.
> **Kernel instruction**: `cosca analytics evolution run`
