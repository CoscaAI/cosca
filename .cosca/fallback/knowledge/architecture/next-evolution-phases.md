# Next Evolution Phases — Engineering Intelligence Platform

> **Status**: Proposed | **Owner**: Cosca Kernel | **Criado**: 2026-07-30
>
> Quatro fases para transformar o Cosca de plataforma cognitiva em **plataforma de inteligência de engenharia** — onde cada ação gera dados, cada dado gera aprendizado, e cada aprendizado refina o runtime.
>
> Baseado na conversa do Don com arquiteto externo — 30+ conceitos avaliados, destilados em 4 fases acionáveis.

---

## Visão Geral

```
F7 — Engineering Intelligence
  ↓ (dados históricos)
F8 — Capability Market  
  ↓ (roteamento inteligente)
F9 — Experience Compiler
  ↓ (conhecimento → princípios)
F10 — Cognitive Economics
  ↓ (ROI cognitivo)
Runtime Autônomo
```

---

## F7 — Engineering Intelligence

> **Foco**: Dados históricos, predição, reputação, score por commit.
> **Conceitos da conversa**: Prediction Engine, Trust Engine, Engineering Score, Impact Report (base já criada), Decision Replay.
> **Dependências**: Timeline + Impact Reports (criados hoje no `memorize-commit`).
> **Esforço estimado**: 2-3 dias.

### Mapa de Conceitos

| Conceito Original | O que é | Entregável | Prioridade |
|-------------------|---------|------------|:----------:|
| **Prediction Engine** | Antes de cada task, prever: P(sucesso), tempo, custo, risco. Comparar previsão × realidade pós-task. | Engine `prediction/SKILL.md` + integração no Quality Gate G0 | 🔥 P0 |
| **Trust Registry** | Reputação histórica de cada agente e tool: tasks executadas, sucessos, falhas, tempo médio, custo médio. | `memory/trust/TRUST_REGISTRY.md` + consulta no Kernel antes de delegar | 🔥 P0 |
| **Engineering Score por Commit** | Score técnico (não só cognitivo) por commit: arquitetura, código, testes, segurança, perf, docs. | Estender `ENGINEERING_TIMELINE.md` com coluna de score | 🔥 P1 |
| **Impact Report Automático** | Já temos o workflow manual. Automatizar via **git hook pós-commit**. | Script `hooks/post-commit` + integração com `memorize-commit` | 🔥 P1 |
| **Decision Replay** | Reproduzir decisões passadas: "Por que escolhemos Bubblewrap?" → abre DDNA com contexto, alternativas, evidências. | Engine `decision-replay/SKILL.md` + UI via `cosca decision replay <id>` | 🔥 P2 |

### Pipeline F7

```
Pré-Task:
  Prediction Engine calcula:
    ├── P(sucesso) = 96%
    ├── Tempo est. = 1m42s
    ├── Custo est. = $0.008
    └── Risco = baixo

  Trust Registry consulta:
    ├── Agent Runtime: 99% (45/46 tasks)
    ├── Tool Bash: 100% (120/120)
    └── Agent Security: 91% (10/11)

Pós-Task:
  Impact Report gera:
    ├── Arquivos: 3
    ├── LOC: +84/-2
    ├── Cobertura: 98.6% → 99.1%
    ├── Tempo real: 2m01s (vs 1m42s previsto)
    ├── Custo real: $0.009 (vs $0.008 previsto)
    └── Score: 94.2 (+0.3)

  Trust Registry atualiza:
    ├── Agent X: success (46/47)
    ├── Tool Y: success (121/121)
    └── Prediction Engine aprende: erro médio ↓ 4%
```

### Arquivos a Criar

| Arquivo | Descrição |
|---------|-----------|
| `engines/prediction/SKILL.md` | Prediction Engine — fórmula, inputs, outputs, calibragem |
| `memory/trust/TRUST_REGISTRY.md` | Trust Registry — formato de entrada, weights, decay |
| `memory/trust/INDEX.md` | Índice do Trust Registry |
| `workflows/prediction-flow.md` | Workflow de previsão pré-task |
| `engines/decision-replay/SKILL.md` | Decision Replay engine |
| `scripts/hooks/post-commit` | Git hook para automatizar Impact Report |

### Critério de Sucesso

- [ ] Prediction Engine com acurácia ≥ 85% em 20 tasks (comparar previsão vs realidade)
- [ ] Trust Registry populado com dados reais dos 54 agentes e 15+ tools
- [ ] Engineering Score por commit propagado na Timeline
- [ ] Git hook pós-commit funcional em pelo menos 5 commits consecutivos
- [ ] Decision Replay funcional para 3 decisões passadas (DDNA existentes)

---

## F8 — Capability Market

> **Foco**: Roteamento dinâmico de tasks. Agentes anunciam capacidades, Kernel escolhe o melhor executor no momento.
> **Conceitos da conversa**: Capability Market ★, Shadow Mode, Contradiction Engine (evoluído), Multi Provider Consensus (casos específicos).
> **Dependências**: F7 (Trust Registry + Prediction Engine são os dados de entrada do market).
> **Esforço estimado**: 1 semana.

### Mapa de Conceitos

| Conceito Original | O que é | Entregável | Prioridade |
|-------------------|---------|------------|:----------:|
| **Capability Market** ★ | Agentes anunciam: `capability: {name: architecture, confidence: 0.94, avg_latency: 1.2s, success_rate: 98%, cost: 0.004}`. Kernel escolhe dinamicamente. | Engine `capability-market/SKILL.md` + protocolo de anúncio | 🔥 P0 |
| **Shadow Mode** | Antes de alterar arquitetura, simular Plano A vs Plano B. "Ensaio" antes de agir. | Engine `shadow-mode/SKILL.md` + integração Gate G0.5 | 🔥 P1 |
| **Contradiction Engine v2** | Busca proativa por evidências contra a decisão atual. Não apenas contrafactual (G0.5) — mineração ativa. | Evoluir `cosca-critic` + engine `contradiction/SKILL.md` | 🔥 P1 |
| **Multi Provider Consensus** | Decisões P0/P1 consultam 2-3 modelos e Kernel sintetiza. | Engine `consensus/SKILL.md` + regras de ativação | 🔥 P2 |

### Pipeline F8

```
Task chega no Kernel:
  "Preciso de uma auditoria de segurança no runtime."

Kernel consulta Capability Market:
  ├── Agent Security:    94% confidence, 1.2s avg, $0.004
  ├── Agent Architecture: 90% confidence, 2.1s avg, $0.006
  ├── Agent DevOps:       85% confidence, 1.8s avg, $0.003
  └── Agent QA:           82% confidence, 3.0s avg, $0.005

Kernel seleciona: Agent Security
  └── Motivo: maior confidence + menor latência + custo compatível

Shadow Mode (se P0):
  ├── Plano A: Security Chief isolado → 98% coverage, 2min
  ├── Plano B: Security + Architecture → 99% coverage, 4min, $0.012
  └── Escolha: Plano A (risco compatível, custo 50% menor)

Contradiction Engine:
  ├── Evidência a favor: Security Chief acertou 45/46 tasks
  ├── Evidência contra: última auditoria complexa teve 1 regressão
  └── Decisão: Plano A, com ponto de atenção documentado
```

### Arquivos a Criar

| Arquivo | Descrição |
|---------|-----------|
| `engines/capability-market/SKILL.md` | Capability Market — protocolo de anúncio, leilão, seleção |
| `engines/shadow-mode/SKILL.md` | Shadow Mode — simulação de cenários alternativos |
| `engines/contradiction/SKILL.md` | Contradiction Engine — mineração ativa de evidências opostas |
| `engines/consensus/SKILL.md` | Multi Provider Consensus — síntese cross-modelo |
| `memory/market/CAPABILITY_CATALOG.md` | (expandir o atual — 64 caps → 64+ com dados de runtime) |
| `workflows/market-routing.md` | Workflow de roteamento via Capability Market |

### Critério de Sucesso

- [ ] 10+ agentes com anúncios ativos no Capability Market
- [ ] 5 tasks roteadas via market com acerto ≥ 90%
- [ ] Shadow Mode executado em 2 decisões P0 com simulação validada
- [ ] Contradiction Engine encontra pelo menos 1 evidência oposta relevante em 5 tasks
- [ ] Multi Provider Consensus usado em ≥ 1 decisão P0 (custo justificado)

---

## F9 — Experience Compiler

> **Foco**: Pipeline de destilação de conhecimento. Centenas de learnings → padrões → princípios → CONSTITUIÇÃO.
> **Conceitos da conversa**: Experience Compiler ★, Knowledge Compiler (evoluído), Wisdom Distillation, Engineering DNA, Cognitive Time Machine, Knowledge Aging (gatilhos).
> **Dependências**: F7 (dados históricos para destilar).
> **Esforço estimado**: 1 semana.

### Mapa de Conceitos

| Conceito Original | O que é | Entregável | Prioridade |
|-------------------|---------|------------|:----------:|
| **Experience Compiler** ★ | Experiência → Aprendizado → Benchmark → Princípio → Constituição. O runtime refatora a própria experiência. | Pipeline F2-F3-F4 completo + engine `experience-compiler/SKILL.md` | 🔥 P0 |
| **Wisdom Distillation** | 500 learnings → agrupar → eliminar duplicados → gerar princípios → atualizar CONSTITUIÇÃO. | Engine `wisdom-distillation/SKILL.md` + ciclo automático | 🔥 P0 |
| **Knowledge Pipeline F2-F3** | F2: Source/Parse/Validate + hot reload. F3: auto-commit + review automático. | Completar `knowledge-pipeline.md` F2 + F3 | 🔥 P1 |
| **Knowledge Aging Gatilhos** | Quando confiança cai abaixo do threshold, disparar revalidação automática. | Gatilhos no `WISDOM_DECAY.md` + integração com scheduler | 🔥 P1 |
| **Engineering DNA** | Projetos novos (`cosca init`) herdam DNA de arquitetura, testing, docs. | Engine `engineering-dna/SKILL.md` + template no init | 🔥 P2 |
| **Cognitive Time Machine** | Revisitar decisões: "Volte para 12 de março. Veja discussão, alternativas, benchmark, decisão." | Engine `time-machine/SKILL.md` + `cosca decision replay <id> --full` | 🔥 P2 |

### Pipeline F9

```
A cada N tarefas (ex: 50):
  Experience Compiler acorda:
    1. Coleta todos os learnings novos (L28, L29, L30...)
    2. Agrupa por domínio (testing, architecture, security...)
    3. Identifica padrões: "3 agentes diferentes encontraram o mesmo bug FTS5"
    4. Gera princípio: "Sempre validar schema FTS5 após migration"
    5. Propõe atualização na CONSTITUIÇÃO
    6. Don aprova (ou rejeita)
    7. Princípio vira regra no runtime

  Wisdom Distillation roda:
    200 learnings atuais
    ↓ Agrupar (similarity score ≥ 0.85)
    42 grupos
    ↓ Eliminar duplicados
    31 grupos únicos
    ↓ Extrair princípios
    7 princípios candidatos
    ↓ Validar contra CONSTITUIÇÃO
    4 princípios aprovados
    ↓ Atualizar CONSTITUIÇÃO.md

  Knowledge Pipeline executa:
    F2: Source → parse learnings.md de todos os 54 agentes
    F2: Validate → checar consistência, tags, referências
    F2: Hot reload → atualizar vector store sem restart
    F3: Auto-commit → git commit com mensagem padrão
    F3: Review → cosca-review valida antes de merge
```

### Arquivos a Criar

| Arquivo | Descrição |
|---------|-----------|
| `engines/experience-compiler/SKILL.md` | Experience Compiler — pipeline completo de destilação |
| `engines/wisdom-distillation/SKILL.md` | Wisdom Distillation — agrupamento, dedup, extração de princípios |
| `engines/engineering-dna/SKILL.md` | Engineering DNA — herança de comportamento entre projetos |
| `engines/time-machine/SKILL.md` | Cognitive Time Machine — reconstrução de contexto de decisão |
| `workflows/experience-compilation.md` | Workflow do ciclo de compilação |
| `memory/knowledge/PRINCIPLES.md` | Registro de princípios destilados (candidatos + aprovados) |

### Critério de Sucesso

- [ ] Experience Compiler roda ciclo completo (coleta → agrupa → princípio → CONSTITUIÇÃO) ao menos 1 vez
- [ ] Wisdom Distillation processa 200+ learnings com taxa de dedup ≥ 40%
- [ ] Pelo menos 1 princípio gerado é aprovado pelo Don e incorporado à CONSTITUIÇÃO
- [ ] F2-F3 do Knowledge Pipeline completos com auto-commit funcional
- [ ] Engineering DNA testado em 1 `cosca init` com herança de configuração

---

## F10 — Cognitive Economics

> **Foco**: Medir e otimizar o ROI de cada ação no runtime. Conhecimento como ativo econômico.
> **Conceitos da conversa**: Cognitive Economics, Engineering Evolution Score (evoluído), Discovery Engine, Organizational Memory.
> **Dependências**: F7 + F8 + F9 (dados históricos + market + princípios).
> **Esforço estimado**: 2-3 dias.

### Mapa de Conceitos

| Conceito Original | O que é | Entregável | Prioridade |
|-------------------|---------|------------|:----------:|
| **Cognitive Economics** ★★ | Cada ação custa (tokens, tempo, atenção) e gera valor (aprendizados, reuso, prevenção de erro). Medir o ROI cognitivo. | Engine `cognitive-economics/SKILL.md` + dashboard | 🔥 P0 |
| **Engineering Evolution Score** | Janeiro 72 → Fevereiro 81 → Março 89. Score composto mensal de saúde do projeto. | Estender CMI + Engineering Score por período | 🔥 P1 |
| **Discovery Engine** | Ao final da sprint: "O que descobrimos que não procurávamos?" — mineração de serendipidade. | Engine `discovery/SKILL.md` + relatório pós-sprint | 🔥 P1 |
| **Organizational Memory** | Padrões organizacionais: "Toda vez que arquitetura e frontend trabalham separados, conflitos aumentam." | Engine `org-memory/SKILL.md` + correlação de eventos | 🔥 P2 |

### Pipeline F10

```
A cada task, Cognitive Economics contabiliza:
  CUSTO:
    ├── Tokens: 12,450 (input) + 3,200 (output) = 15,650
    ├── Tempo: 1m42s
    ├── Atenção: 2 agentes mobilizados
    └── Custo financeiro: $0.008

  VALOR:
    ├── Aprendizados: 2 (reutilizáveis)
    ├── Impacto futuro: 1 bug prevenido (est. $0.05)
    ├── Reuso: 3 padrões aplicados
    └── Prevenção: 0 regressões

  ROI COGNITIVO:
    ├── Custo total: $0.008 + 2 agentes × $0.001
    ├── Valor estimado: $0.05 (bug prevenido) + 2 aprendizados × $0.01
    ├── ROI: ($0.07 - $0.01) / $0.01 = 600%
    └── Eficiência: 92.3% (melhor que a mediana de 87%)

Discovery Engine (fim da sprint):
  ├── O que procurávamos: "aumentar cobertura de testes"
  ├── O que encontramos: "padrão de race condition em 3 pacotes diferentes"
  └── Aprendizado: "race condition é sistêmico, não local — precisa de policy global"

Organizational Memory (acumulativo):
  ├── 5 sprints analisadas
  ├── Padrão: "arquitetura decide isolado → frontend refaz 3x"
  ├── Confiança: 0.78 (5 ocorrências)
  └── Recomendação: "revisão conjunta obrigatória antes de decisões de API"
```

### Arquivos a Criar

| Arquivo | Descrição |
|---------|-----------|
| `engines/cognitive-economics/SKILL.md` | Cognitive Economics ★ — ROI tracking, dashboard value/cost |
| `engines/discovery/SKILL.md` | Discovery Engine — serendipity mining |
| `engines/org-memory/SKILL.md` | Organizational Memory — padrões de processo |
| `analytics/evolution-score.md` | Engineering Evolution Score — score mensal composto |
| `workflows/cognitive-economics.md` | Ciclo de report e otimização |

### Critério de Sucesso

- [ ] 10 tasks com ROI cognitivo calculado (custo × valor)
- [ ] Dashboard funcional mostrando top 5 agentes por ROI
- [ ] Discovery Engine gera pelo menos 1 insight não-planejado em 3 sprints
- [ ] Engineering Evolution Score calculado com baseline e projeção
- [ ] Organizational Memory identifica 1 padrão organizacional com confiança ≥ 0.70

---

## Dependências entre Fases

```
F7 ─────────────────────────────────────────────►
  Engineering Intelligence (dados históricos)
  │
  ├── F8 depende de F7 ────────────────────────►
  │   Capability Market (precisa de Trust + Prediction)
  │   │
  │   ├── F9 depende de F7+F8 ────────────────►
  │   │   Experience Compiler (precisa de dados + roteamento)
  │   │   │
  │   │   ├── F10 depende de F7+F8+F9 ────────►
  │   │   │   Cognitive Economics (precisa de tudo acima)
  │   │   │   │
  │   │   │   ▼
  │   │   │   RUNTIME AUTÔNOMO
```

---

## Resumo de Esforço

| Fase | Conceitos | Arquivos | Esforço | Impacto |
|------|-----------|:--------:|:-------:|:-------:|
| **F7 — Engineering Intelligence** | 5 (Prediction, Trust, Score, Impact, Replay) | ~7 | 🟡 2-3 dias | 🔥🔥🔥🔥🔥 |
| **F8 — Capability Market** | 4 (Market, Shadow, Contradiction, Consensus) | ~7 | 🔴 1 semana | 🔥🔥🔥🔥🔥 |
| **F9 — Experience Compiler** | 6 (Compiler, Distillation, Pipeline, Aging, DNA, Time Machine) | ~7 | 🔴 1 semana | 🔥🔥🔥🔥🔥 |
| **F10 — Cognitive Economics** | 4 (Economics, Evolution, Discovery, Org Memory) | ~5 | 🟡 2-3 dias | 🔥🔥🔥🔥🔥 |
| **Total** | **19 conceitos** | **~26 arquivos** | **~3 semanas** | **🔥 Revolucionário** |

---

## O Que Fica de Fora (Por Enquanto)

| Conceito | Motivo | Possível Futuro |
|----------|--------|-----------------|
| Engineering Theory | Pesquisa, não engenharia. Exigiria correlação estatística complexa. | Pós-F10 |
| Evolution Simulator | Extremamente complexo. Exigiria modelo preditivo de acoplamento. | Pós-F10 |
| Universal Pattern Engine | Exigiria parser cross-linguagem. Só faz sentido com múltiplas stacks. | Pós-F10 |
| Engineering Instinct | ML preditivo sobre métricas históricas. Pesquisa acadêmica. | Pós-F10 |
| Engenharia Evolutiva | Seleção natural de arquiteturas. Exigiria dezenas de projetos. | Visão 2027 |
| Cognitive Genetics | DNA entre projetos — depende do Engineering DNA (F9) amadurecer. | Pós-F10 |

---

## Related

- [Workflow: memorize-commit](../../workflows/memorize-commit.md) — Base da F7 (Impact Report + Timeline)
- [Cognitive Maturity Implementation](../../workflows/cognitive-maturity-implementation.md) — F0-F3 atuais
- [Engineering Timeline](../../memory/timeline/ENGINEERING_TIMELINE.md) — Dados históricos para F7
- [CONSTITUTION](../../CONSTITUTION.md) — Alvo do Experience Compiler (F9)
- [QUALITY_GATES](../../QUALITY_GATES.md) — Gates que serão expandidos com Prediction (F7) e Shadow Mode (F8)
- [KERNEL.md](../../KERNEL.md) — Kernel como orquestrador do market (F8)
