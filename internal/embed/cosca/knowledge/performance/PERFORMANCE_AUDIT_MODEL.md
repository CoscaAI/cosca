# 14 — PERFORMANCE AUDIT MODEL

> Stack 14 da Cosca Engineering Intelligence Matrix.
> O modelo matemático de auditoria (GoogleChrome/lighthouse): como cada check é pontuado,
> como o score agregado nasce e por que "medido" nunca pode ser "simulado".

## MISSÃO
Formalizar o modelo de auditoria de performance para que todo check tenha score rastreável,
threshold verificável e savings honestos — o auditor nunca troca medição por simulação.

## PRINCÍPIOS CORE
1. **Todo check tem um meta**: `id`/`title`/`description`/`scoreDisplayMode`/`requiredArtifacts` — sem meta não é check · UNIVERSAL
2. **Score individual é log-normal de 2 pontos**: `median` → score 0.5, `p10` → score 0.9; a curva mapeia valor medido → score 0-1 · UNIVERSAL
3. **Bandas estáveis**: pass ≥0.9, avg 0.5-0.89, fail <0.5 — a banda diz "percentil de sites piores que você" · UNIVERSAL
4. **Score agregado = média aritmética ponderada**; N/A/informative/manual têm peso forçado a 0 · UNIVERSAL
5. **Savings nunca é medição**: a estimativa Lantern re-simula o dependency graph e produz `metricSavings` (FCP/LCP em ms) — sempre marcado como SIMULADO · UNIVERSAL

## O MODELO

### Anatomia de um check
| Componente | Papel |
|---|---|
| `meta.id` | identidade estável do check |
| `meta.scoreDisplayMode` | como o score deve ser lido (numeric/binary/metricSavings/manual/informative/notApplicable/error) |
| `meta.requiredArtifacts` | quais artefatos o check precisa antes de rodar |
| `score` (0-1) | resultado normalizado do check |
| `numericValue` + `numericUnit` | o valor bruto medido com sua unidade |
| `details` | breakdown por item (subpartes do check) |
| `metricSavings` | economia estimada em ms por métrica (FCP/LCP) |

### SCORING_MODES
| Mode | Comportamento |
|---|---|
| `numeric` | pontua 0-1 pela curva log-normal |
| `binary` | 0 ou 1 (passou/não passou) |
| `metricSavings` | carrega `metricSavings`; não entra como score tradicional |
| `manual` | sem valor automático — requer auditor humano |
| `informative` | informa, não pontua |
| `notApplicable` | não se aplica ao contexto; NÃO pontua |
| `error` | falhou ao coletar; NÃO pontua |

### Score individual — log-normal de 2 pontos
A curva log-normal é ancorada em **dois control points**: `median` (→ score 0.5) e `p10` (→ score 0.9). Entre eles o mapeamento é log-linear.

| Banda | Score | Significado |
|---|---|---|
| pass | ≥0.9 | performance melhor que ~90% dos sites |
| avg | 0.5–0.89 | na mediana dos sites |
| fail | <0.5 | pior que a maioria |

### Score agregado
Média aritmética ponderada dos scores individuais. `notApplicable`, `informative` e `manual` têm peso **forçado a 0**.

| Métrica | Peso |
|---|---|
| FCP | 10 |
| LCP | 25 |
| TBT | 30 |
| CLS | 25 |
| SI | 10 |

### Thresholds (mobile)
| Métrica | Bom | Alvo |
|---|---|---|
| LCP | ≤2500ms | — |
| FCP | ≤1800ms | — |
| TBT | ≤200ms | — |
| CLS | ≤0.1 | — |
| SI | ≤3387ms | — |
| INP | ≤200ms (≤500ms precisa melhoria) | — |
| TTFB | — | 100ms |

Desktop: LCP ≤1200ms · FCP ≤934ms · TBT ≤150ms.

## REGRAS DE DECISÃO
- **Savings = simulação**: a Lantern re-simula o dependency graph com os bytes removidos e estima `metricSavings` (FCP/LCP em ms). Reportar como *estimado/simulado* — nunca como medido.
- **CI usa median-run**: de N runs, escolha a run mais próxima da mediana — nunca a média do score. Média esconde cauda e vibra com outliers.
- **Insights são declarações tipadas**: `cache`, `render-blocking`, `lcp-discovery`, `inp-breakdown` com `state` (fail/pass), `category`, `metricSavings` e `guidanceLevel` — para priorizar a ação.
- Score de check com `requiredArtifacts` ausentes = `error`, não `fail` — erro de coleta nunca vira falha de performance.
- `manual`/`informative` não entram no agregado; seu peso é zero por construção, não por esquecimento.

## ANTI-PATTERNS
`tratar savings simulado como dado medido` · `média do score em CI em vez de median-run` · `mudar threshold p/ parecer bom` · `ignorar requiredArtifacts e pontuar check no escuro` · `N/A com peso 1 "pra equilibrar"` · `reportar p10/p50 sem control points` · `FID como métrica viva (morta desde 2024; usar INP)`

## CHECKLIST
- [ ] Todo check tem `meta` completo (id/title/description/scoreDisplayMode/requiredArtifacts)
- [ ] Curva log-normal ancorada em `median` (0.5) e `p10` (0.9)
- [ ] Agregado = média ponderada com pesos FCP 10 / LCP 25 / TBT 30 / CLS 25 / SI 10
- [ ] N/A/informative/manual com peso 0 no agregado
- [ ] Thresholds mobile (LCP 2500 / FCP 1800 / TBT 200 / CLS 0.1 / SI 3387 / INP 200) respeitados
- [ ] Savings Lantern rotulado como SIMULADO, nunca como medido
- [ ] CI com median-run (run mais próxima da mediana, não média)
- [ ] Insights tipados: cache / render-blocking / lcp-discovery / inp-breakdown com state + category

## A REGRA
Auditoria é ciência: o score nasce de uma curva com control points explícitos, o agregado só soma quem pontua, e a estimativa de ganho é sempre simulação honesta — quem confunde simulação com medição mente para si mesmo.

## REFERÊNCIAS
GoogleChrome/lighthouse (scoring, median-run, Lantern) · web.dev/vitals (INP, thresholds) · Stack 14 (performance/) · performance/README.md · PERFORMANCE_AUDIT.md (skill)

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: performance-001
DOMAIN: performance
TITLE: Score individual log-normal de 2 pontos
PROBLEM: Valores medidos não se traduzem em "bom ou ruim" sem uma régua de comparação com a web real
CONTEXT: Lighthouse audit, qualquer métrica contínua (LCP, FCP, TBT, CLS)
PRINCIPLE: Uma curva log-normal ancorada em control points traduz valor medido → score 0-1; median → 0.5, p10 → 0.9
RECOMMENDATION: Mapear numericValue pela curva log-normal e reportar a banda (pass ≥0.9 / avg 0.5-0.89 / fail <0.5) como percentil de sites piores que você
WHEN_TO_USE: Toda métrica contínua com distribuição na web real
WHEN_NOT_TO_USE: Checks binários (passou/não) ou manuais, sem curva
TRADE_OFFS: Curva calibrada em uma amostra vs comparação estável ao longo do tempo
EXAMPLE: LCP 2500ms caindo em score ~0.9 (p10); LCP 4000ms abaixo de 0.5
COUNTER_EXAMPLE: Pontuar por fórmula linear fixa (100 - valor/10) sem base estatística
FAILURE_MODES: Curva recalibrada silenciosamente muda scores sem nenhuma mudança real na página
REFERENCES: GoogleChrome/lighthouse scoring docs
CONFIDENCE: UNIVERSAL
SOURCE: GoogleChrome/lighthouse (verificado) + Stack 14
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: performance-002
DOMAIN: performance
TITLE: Score agregado ponderado com peso zero para N/A/informative/manual
PROBLEM: Soma cega de scores distorce o resultado quando metade dos checks não se aplica
CONTEXT: Performance score total do Lighthouse (FCP 10 / LCP 25 / TBT 30 / CLS 25 / SI 10)
PRINCIPLE: Média aritmética ponderada; checks sem valor (notApplicable/informative/manual) têm peso forçado a 0
RECOMMENDATION: Normalizar o agregado apenas sobre pesos positivos; nunca dar peso 1 a N/A para "equilibrar"
WHEN_TO_USE: Agregação de múltiplos checks num score único
WHEN_NOT_TO_USE: Comparação item-a-item de checks individuais
TRADE_OFFS: Um número único legível vs granularidade perdida no agregado
EXAMPLE: Só LCP+CLS aplicáveis → score = (LCP·25 + CLS·25) / 50
COUNTER_EXAMPLE: Incluir informative no denominador e derrubar o score por checks que nem pontuam
FAILURE_MODES: Pesos editados à mão para inflar o score em dashboards
REFERENCES: GoogleChrome/lighthouse weight docs
CONFIDENCE: UNIVERSAL
SOURCE: GoogleChrome/lighthouse (verificado)
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: performance-003
DOMAIN: performance
TITLE: Thresholds móveis dos Core Web Vitals e métricas de lab
PROBLEM: Sem números fixos, "está rápido" vira opinião e a regressão passa despercebida
CONTEXT: Auditoria mobile-first de páginas web
PRINCIPLE: Thresholds verificáveis definem bom/melhorável: LCP 2500ms, FCP 1800ms, TBT 200ms, CLS 0.1, SI 3387ms, INP 200ms, TTFB alvo 100ms
RECOMMENDATION: Falhar fora dos thresholds; tratar INP ≤200ms como bom e ≤500ms como "precisa melhoria"
WHEN_TO_USE: Toda auditoria de frontend/web
WHEN_NOT_TO_USE: Aplicações sem interação relevante ou sem render web (backend puro)
TRADE_OFFS: Números fixos fáceis de auditar vs contexto específico de cada produto (ex.: streaming)
EXAMPLE: TBT 180ms = pass; TBT 350ms = fail/needs improvement
COUNTER_EXAMPLE: "Melhorou 20%" sem comparar com o threshold absoluto
FAILURE_MODES: Ambientes lentos (CI sobrecarregado) gerando falsos fails por throttling
REFERENCES: web.dev/vitals thresholds (mobile/desktop)
CONFIDENCE: UNIVERSAL
SOURCE: GoogleChrome/lighthouse + web.dev (verificado)
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: performance-004
DOMAIN: performance
TITLE: Savings da Lantern é simulação, nunca medição
PROBLEM: Estimativas de ganho reportadas como "medidas" geram promessas falsas e decisão errada
CONTEXT: Audit de oportunidades (byte remoção, render-blocking, defer de script)
PRINCIPLE: A Lantern re-simula o dependency graph com bytes removidos e estima metricSavings (FCP/LCP em ms) — resultado é simulado
RECOMMENDATION: Rotular todo savings como estimado/simulado e validar com medição real pós-deploy (A/B)
WHEN_TO_USE: Priorizar oportunidades de otimização por impacto estimado
WHEN_NOT_TO_USE: Reporte pós-otimização — ali o impacto deve ser MEDIDO em campo
TRADE_OFFS: Velocidade da simulação vs fidelidade do navegador real
EXAMPLE: Remover render-blocking CSS estima -600ms de FCP (simulado) → validar no RUM depois
COUNTER_EXAMPLE: Reportar "FCP melhorou 600ms" baseado só na Lantern, sem medição
FAILURE_MODES: Simulação otimista em graph complexo com cache/CDN diverge do mundo real
REFERENCES: GoogleChrome/lighthouse Lantern docs
CONFIDENCE: UNIVERSAL
SOURCE: GoogleChrome/lighthouse (verificado)
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: performance-005
DOMAIN: performance
TITLE: CI com median-run em vez de média do score
PROBLEM: A média de scores em N runs vibra com outliers e mascara regressões reais
CONTEXT: Pipeline de CI rodando Lighthouse/audit em N execuções
PRINCIPLE: De N runs, escolhe-se a run mais próxima da mediana — não a média dos scores
RECOMMENDATION: Reportar e fazer gate no score da run mediana; fixar o ambiente (throttle, CPU, rede) para reduzir ruído
WHEN_TO_USE: Qualquer gate automatizado de performance
WHEN_NOT_TO_USE: Diagnóstico pontual de uma única execução local
TRADE_OFFS: Estabilidade da decisão vs custo de N runs por commit
EXAMPLE: 3 runs com scores 0.82/0.91/0.93 → reporta a run ~0.91 (mediana), não 0.886 (média)
COUNTER_EXAMPLE: Gate reprovando/reprovando alternadamente porque compara a média
FAILURE_MODES: N pequeno + ambiente ruidoso → mediana ainda instável (aumentar N)
REFERENCES: GoogleChrome/lighthouse CI (median-run)
CONFIDENCE: UNIVERSAL
SOURCE: GoogleChrome/lighthouse (verificado) + Stack 14
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
