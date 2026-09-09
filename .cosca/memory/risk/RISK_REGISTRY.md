# RISK REGISTRY — Cosca v1.4.0-dev

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Created**: 2026-07-28
>
> **Scope**: Riscos de plataforma, agentes, memória, processo e roadmap.
> **Update cadence**: Revisão semanal no dashboard ou após qualquer incidente.

---

## 🔴 Críticos (agir agora ou perde o controle)

| # | Risco | Prob. | Impacto | Mitigação | Depende |
|---|---|---|---|---|---|
| **R1** | **41/51 agentes nunca executaram** — Capability Profiles baseados em seed data. Se um agente falhar na primeira task real, o Don perde confiança na plataforma. | 90% | Alto | Onda 2 de ativação: 1 task real para cada agente inativo. Priorizar 10 mais críticos (security, database, devops, testing). | — |
| **R2** | **Confiança média 0.48** — Meta é 0.70. Plataforma não confia em si mesma. Metacognition pipeline sem dados reais de validação cruzada. | 80% | Alto | Ativar Onda 2 + 30 dias de dados no Confidence Model. Cross-agent validation (M2) precisa de ≥2 agentes validando o mesmo padrão. | R1 + 30d |
| **R3** | **Sem soak test (>24h)** — Memory leaks, cache leaks, race conditions só aparecem em sessões longas. Bug-004 corrigido mas nunca validado em 24h+. | 70% | Alto | CI gate: soak test 1h com monitoramento de memória. Estender para 24h. Alerta se crescimento > 10%. | CI |

---

## 🟠 Altos (resolver este mês)

| # | Risco | Prob. | Impacto | Mitigação | Depende |
|---|---|---|---|---|---|
| **R4** | **Sem baseline de performance** — Não sabemos se a plataforma está ficando mais rápida ou mais lenta. Otimização é tiro no escuro. | 60% | Médio | Performance benchmarking suite (P2 no roadmap). Medir: latência de roteamento, tempo/task, tokens/task. | — |
| **R5** | **gRPC server incompleto** — Proto definido mas servidor pendente. Sem gRPC não tem streaming, performance, ou contrato forte entre serviços. | 50% | Médio | Completar implementação do gRPC server (handler + integração com orchestration engine). | — |
| **R6** | **Memory Decay Engine não calibrado** — Half-life values (180d/90d/60d) são teóricos. Pode over-decay (amnésia) ou under-decay (estagnação). | 55% | Médio | Aguardar 2 ciclos de curadoria (14 dias). Monitorar Forgetting Rate (target: 2-5%). Ajustar half-lives. | 14d |
| **R7** | **Context Compression é lossy — sem validação** — 79% de compressão teórica. Estado comprimido pode não ser suficiente para todos os tipos de sessão. | 40% | Médio | Testar em 3 tipos de sessão: feature dev, debugging, architecture review. Adicionar seção condicional se necessário. | 3 sessões |
| **R8** | **Documentation drift** — Docs sincronizados manualmente (Fase 1-2). Sem validação automática, vão divergir do código novamente. | 75% | Médio | Doc-code validator: script que cruza referências em docs com paths reais. Rodar no CI. | — |

---

## 🟡 Médios (planejar, não urgente)

| # | Risco | Prob. | Impacto | Mitigação | Depende |
|---|---|---|---|---|---|
| **R9** | **Kernel single point of failure** — Se o Kernel rotear errado, toda a cadeia falha. Sem fallback. | 30% | Alto | cosca-critic (#5, Categoria B) como segundo opinador. | Roadmap B |
| **R10** | **Sem E2E tests** — Cobertura ~78% mas é unit + integration. Nenhum teste de jornada completa. | 45% | Médio | E2E suite: 5 cenários críticos (criar projeto, provider, task, review, deploy). | — |
| **R11** | **Sem disaster recovery** — 408 arquivos de memória sem backup automático. Se perder, meses de evolução perdidos. | 25% | Alto | Backup: git auto-commit da memória a cada curadoria + backup semanal do .cosca/. | — |
| **R12** | **Zero cross-agent validation data** — Confidence Model M2 modifier sem dados reais. 41 agentes nunca trocaram conhecimento. | 60% | Baixo | Onda 2 + promover padrões cross-agent via R4 do Curation Engine. | R1 |
| **R13** | **Evolução sem supervisão** — Metacognition Pipeline permite auto-evolução. Sem gate, agente pode evoluir para pior (confirmation bias). | 35% | Médio | Level-up requer: ≥5 tasks com sucesso + cross-agent validation + review do Chief. | — |
| **R14** | **Sem semantic memory** — INDEX hierárquico funciona com 408 arquivos. Com 1000+, busca linear degrada. | 40% | Baixo | Memória semântica (Tech Radar #1). Postergado para pós-v2.0. Reavaliar quando >600 arquivos. | v2.0 |
| **R15** | **Helm chart incompleto** — Sem deploy production-grade. Docker funciona mas sem scaling, rollback, health checks K8s. | 40% | Baixo | Productionizar Helm chart. Depende de CI/CD. | CI/CD |

---

## ⚪ Baixos (monitorar)

| # | Risco | Prob. | Impacto | Mitigação | Depende |
|---|---|---|---|---|---|
| **R16** | **TypeScript SDK bloqueado** — Pendente de OpenAPI spec generation. | 30% | Baixo | Gerar OpenAPI spec do código Go → gerar TS SDK. | OpenAPI |
| **R17** | **Provider coverage (6 faltando)** — Nem todos os providers de LLM integrados. | 20% | Baixo | Integrar conforme demanda. Providers existentes cobrem 80%. | — |
| **R18** | **WASM runtime incompleto** — EPIC-002 em 75%. Plugin system parcial. | 25% | Baixo | Completar após v1.0. | v1.0 |
| **R19** | **Drift entre agentes** — Backend Chief aprende regra que Frontend contradiz. Sem detecção (#8 postergado). | 20% | Baixo | Alternativa social: notificar Chief do outro domínio em decisões cross-domain. | — |
| **R20** | **CIS baseline é estimado** — 42.80 com componentes aspiracionais. Sem dados reais, score pode estar errado. | 50% | Baixo | Primeira medição real do CIS após 30 dias de Onda 2. | R1 + 30d |

---

## Resumo por Severidade

| Severidade | Quantidade | Ação |
|---|---|---|
| 🔴 Crítico | 3 | **Esta semana** — R1 (Onda 2), R2 (Confiança), R3 (Soak test) |
| 🟠 Alto | 5 | **Este mês** — R4 (Benchmark), R5 (gRPC), R6 (Calibrar Decay), R7 (Validar Compressão), R8 (Doc validator) |
| 🟡 Médio | 7 | **Planejar** — R9-R15, maioria depende de infra ou dados |
| ⚪ Baixo | 5 | **Monitorar** — R16-R20, risco cresce com escala |

---

## Top 3 Ações Imediatas

| # | Ação | Resolve | Esforço |
|---|---|---|---|
| 1 | Ativar Onda 2 — 1 task real para cada um dos 10 agentes mais críticos | R1 + R2 + R12 | 2-3 dias |
| 2 | Soak test de 1h no CI com monitoramento de memória | R3 | 1 dia |
| 3 | Doc-code validator script (cruzar referências de docs com paths reais) | R8 | 1 dia |

---

> **Related**: [platform-evolution-v1.4.0.md](../roadmap/platform-evolution-v1.4.0.md) | [milestones.md](../roadmap/milestones.md) | [cognitive-state.md](../context/cognitive-state.md) | [bug/INDEX.md](../bug/INDEX.md)
