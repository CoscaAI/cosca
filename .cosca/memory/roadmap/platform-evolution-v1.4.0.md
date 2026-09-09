---
type: roadmap
key: platform-evolution-v1.4.0
tags: [roadmap, evolution, improvements, cosca-kernel]
timestamp: 2026-07-28T00:00:00Z
status: active
author: Cosca Kernel
approved_by: Don
---

# Platform Evolution — 10 Melhorias Analisadas

> **Contexto:** O Don apresentou 10 propostas de melhoria para a plataforma. O Kernel analisou cada uma contra os 32 engines e 27 workflows existentes, avaliando risco, valor, complexidade de runtime, e risco de regressão.

---

## Categoria A — ⭐ Já (Após Commit)

Baixo risco, complementam engines existentes, retorno imediato.

| # | Melhoria | Esforço | O que já temos | O que falta |
|---|---|---|---|---|
| **2** | **Memory Decay Engine** | 1-2 dias | Memory Curation Engine (5 regras, CurationScore) | ✅ **Concluído 2026-07-28.** [MEMORY_DECAY_ENGINE.md](../../engines/memory-curation/MEMORY_DECAY_ENGINE.md) — CurationScore v2.0 com frequency_density, success_rate, age_decay (half-life model), e R6 Conflict Detection. |
| **4** | **Árvore de Causalidade** | 1 hora | Bug registry (13 arquivos, flat) | Template com 4 níveis (causa direta → arquitetural → processo → prevenção). Atualizar bugs existentes. |
| **6** | **Intelligence Score** | 1-2 dias | Platform Health Dashboard (5 métricas-chave + 10 secundárias) | ✅ **Concluído 2026-07-28.** Dashboard v2.0.0 — Cosca Intelligence Score (CIS): 5 dimensões (Autonomia, Precisão, Auto-correção, Reutilização, Evolução) com score composto 0-100. |
| **9** | **Context Compression** | 2-3 dias | Session context (quick-load manual) | ✅ **Concluído 2026-07-28.** [CONTEXT_COMPRESSION_ENGINE.md](../../engines/context-compression/CONTEXT_COMPRESSION_ENGINE.md) — cognitive-state.md (~400 tokens, formato YAML compacto), Rule of 5, pipeline COLLECT→COMPRESS→VALIDATE. Target: 9.7k → 2k tokens (79% redução). |

**Total esforço:** ~5 dias | **Risco de regressão:** Zero (são extensões, não substituições).

---

## Categoria B — 📋 Plano (Risco Controlado, Próximo Mês)

Novas capacidades com risco gerenciável. Precisam de calibração com dados reais.

| # | Melhoria | Esforço | Dependência | Risco |
|---|---|---|---|---|
| **3** | **Paradigm Shift Detection (cosca-paradigm)** | 1-2 semanas | Confidence Model com ≥3 meses de dados | ✅ **Criado 2026-07-28.** Agent definition + DNA v3.0 + 6 memory files. Activation gate: 3 meses de Confidence Model data. Observation mode until ~2026-10-28. |
| **5** | **Devil's Advocate (cosca-critic)** | 1-2 semanas | Bug registry + ADRs históricos como base de treino | ✅ **Criado 2026-07-28.** Agent definition + DNA v3.0 + 6 memory files. 5-Question Challenge, evidence-weighted critique, severity-gated. Integrated: opencode.json, bootstrap (always_active). |

**Total esforço:** ~3 semanas | **Risco de regressão:** Baixo (agentes novos, sem alteração de engines existentes).

---

## Categoria C — 🔮 Futuro (v2.0, Depende de Infra)

Alto valor, mas dependem de Shadow Execution (#1) como pré-requisito.

| # | Melhoria | Esforço | Pré-requisito | Por que esperar |
|---|---|---|---|---|
| **1** | **Shadow Execution** | 3-4 sprints | — (é o pré-requisito dos outros) | Sandbox por agente, diff engine, rollback atômico, análise de impacto cross-module. Custo alto, justifica quando tivermos 100+ agentes. |
| **7** | **Knowledge Replay** | 2-3 sprints | Shadow Execution (#1) | Precisa de sandbox isolado pra re-executar tarefas antigas com segurança. |
| **10** | **Experiment-Based Evolution** | 4-6 sprints | Shadow Execution (#1) + Knowledge Replay (#7) | É o Santo Graal: agente vira cientista (hipótese → experimento → aprendizado). Mas sem sandbox, é perigoso. |

**Total esforço:** ~3 meses (após v1.5.0) | **Risco de regressão:** Alto se implementado sem a infra de sandbox.

---

## Categoria D — ⚠️ Postergar

Problema real, mas solução generalista complexa demais para o retorno.

| # | Melhoria | Motivo |
|---|---|---|
| **8** | **Controle de Deriva (Evolution Consistency Check)** | Detectar conflito semântico entre domínios diferentes (backend vs frontend) requer NLP cross-domain pesado. Alternativa mais barata: quando um agente toma decisão que afeta outro domínio, notificar o Chief do outro domínio para revisar. Abordagem social, não técnica. |

---

## Ordem de Execução

```
HOJE ──────────────────────────────────────────────► v2.0
│
├─ ⭐ #4  Causalidade (1h)         ← já
├─ ⭐ #2  Memory Decay (2d)        ← já
├─ ⭐ #6  Intelligence Score (2d)  ← já
├─ ⭐ #9  Context Compression (3d) ← já
│
├─ 📋 #5  cosca-critic (2 sem)     ← próximo mês
├─ 📋 #3  cosca-paradigm (2 sem)   ← após 3 meses de dados do Confidence Model
│
├─ 🔮 #1  Shadow Execution         ← v2.0 (pré-requisito)
├─ 🔮 #7  Knowledge Replay         ← v2.0 (depende de #1)
├─ 🔮 #10 Experiment Evolution     ← v2.0 (depende de #1 + #7)
│
└─ ⚠️ #8  Drift Control           ← postergado (alternativa social primeiro)
```

---

## Métricas de Sucesso por Categoria

| Categoria | Métrica | Baseline | Target |
|---|---|---|---|
| ⭐ Já | CurationScore médio | 0.72 (estimado) | ≥0.80 |
| ⭐ Já | Tokens/sessão | ~9.7k | ≤2k |
| ⭐ Já | Intelligence Score composto | — | ≥80/100 |
| 📋 Plano | Decisões com revisão crítica | 0% | 100% |
| 📋 Plano | Paradigmas questionados/mês | 0 | 2-5 |
| 🔮 Futuro | Erros prevenidos por Shadow Execution | 0 | ≥30% dos erros |
| 🔮 Futuro | Tempo de replay vs original | — | ≤50% do tempo original |
