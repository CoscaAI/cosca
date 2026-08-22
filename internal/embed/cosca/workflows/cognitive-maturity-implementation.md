# Cognitive Maturity Implementation — Pipeline de Implementação

> **Workflow**: `cosca-cognitive-maturity-implementation`
> **Versão**: 1.0.0 | **Status**: active
> **Owner**: Cosca Kernel
> **Criado**: 2026-07-30
> **Dependências**: Nenhuma — workflow FUNDACIONAL para a plataforma
> **CMI Target**: +1.71 cumulative (baseline 0.24 → target 1.95 / 95%)

---

## Objetivo

Implementar a arquitetura de Cognitive Maturity do Cosca Kernel — 21 capacidades distribuídas em 4 blocos cognitivos, organizadas em 4 fases de execução sequencial. Cada fase entrega um ganho mensurável de CMI (Cognitive Maturity Index) e habilita a fase seguinte.

---

## Visão Geral da Arquitetura Cognitiva

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        COSCA COGNITIVE ARCHITECTURE                      │
│                                                                          │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  ┌──────┐│
│  │  BLOCO 1        │  │  BLOCO 2        │  │  BLOCO 3        │  │BLOCO ││
│  │  Memória &      │  │  Decisão &      │  │  Aprendizado &  │  │  4   ││
│  │  Conhecimento   │  │  Raciocínio     │  │  Evolução       │  │Ética ││
│  │                 │  │                 │  │                 │  │  &   ││
│  │ F0.1-F0.2       │  │ F0.6            │  │ F0.3-F0.5       │  │Gov.  ││
│  │ F1.1,F1.4       │  │ F1.2-F1.3       │  │ F1.5-F1.6       │  │      ││
│  │ F2.3-F2.4,F2.6  │  │ F2.1-F2.2       │  │ F2.5            │  │      ││
│  │ F3.2,F3.4       │  │ F3.1,F3.3       │  │ F3.5-F3.7       │  │      ││
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  └──────┘│
│                                                                          │
│  4 Blocos × 4 Fases = pipeline de maturação cognitiva completa           │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## Dependências entre Fases

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        FASE DEPENDENCY GRAPH                            │
│                                                                         │
│   FASE 0: Foundation Fix                                                │
│   (Imediata — CMI gain +0.38)                                           │
│   ════════════════════════════                                          │
│   │                                                                     │
│   │  Corrige o que está quebrado HOJE.                                  │
│   │  Popula memória real (failures, patterns, evolution).               │
│   │  Sem isso, nada funciona.                                           │
│   │                                                                     │
│   ├──────────── BLOQUEIA ───────────────────────────────────────────►  │
│   │                                                                     │
│   ▼                                                                     │
│   FASE 1: Foundation                                                    │
│   (Curto prazo — CMI gain +0.22)                                        │
│   ═══════════════════════════════                                       │
│   │                                                                     │
│   │  Constrói as fundações: Decision DNA, métricas reais,               │
│   │  contrafactual gate, gap detection, Wisdom Decay.                   │
│   │                                                                     │
│   ├──────────── BLOQUEIA ───────────────────────────────────────────►  │
│   │                                                                     │
│   ▼                                                                     │
│   FASE 2: Core Engines                                                  │
│   (Médio prazo — CMI gain +0.43)                                        │
│   ════════════════════════════════                                      │
│   │                                                                     │
│   │  Motores centrais: Cognitive Economy, Immune System,                │
│   │  Semantic Federation, Gravity, Momentum, Pattern Evolution.         │
│   │  ★ Cognitive Economy é a STAR desta fase.                           │
│   │                                                                     │
│   ├──────────── BLOQUEIA ───────────────────────────────────────────►  │
│   │                                                                     │
│   ▼                                                                     │
│   FASE 3: Advanced Cognition                                            │
│   (Longo prazo — CMI gain +0.68)                                        │
│   ════════════════════════════════                                      │
│                                                                         │
│   Cognição avançada: Insight Generator, 2nd-order reasoning,            │
│   Cognitive Compression, Adaptive Personality, Mental Energy,           │
│   Cognitive Horizon, Ecosystem Dynamics.                                │
│   Requer TODOS os motores da Fase 2 em operação.                        │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘

LEGENDA:
  ───►  Bloqueante (fase anterior DEVE estar completa)
  - -►  Parcialmente bloqueante (motores específicos necessários)
```

---

## Cumulative CMI Trajectory

```
CMI
1.00 ┤                                                          ┌── F3: +0.68
     │                                                          │
0.80 ┤                                                          │
     │                                                          │
0.60 ┤                                          ┌── F2: +0.43   │
     │                                          │               │
0.40 ┤                      ┌── F1: +0.22       │               │
     │                      │                   │               │
0.20 ┤  ┌── F0: +0.38       │                   │               │
     │  │                   │                   │               │
0.00 ┼──┴───────────────────┴───────────────────┴───────────────┴─────►
     Fase 0              Fase 1              Fase 2           Fase 3
     (1-2h)             (2-3d)             (1-2sem)         (2-4sem)

Baseline CMI: 0.24  →  F0: 0.62  →  F1: 0.84  →  F2: 1.27  →  F3: 1.95 (>=95%)
```

---

## Estrutura de Cada Tarefa

Cada tarefa segue formato estruturado com propriedades padronizadas:

```yaml
task:
  id: F{N}.{n}
  phase: Fase {N}
  block: Bloco {1-4} — {nome}
  title: Descrição curta da tarefa
  owner: agente-primário
  collaborators: [agente-1, agente-2]
  cmi_gain: +0.XX
  depends_on: [task_id, ...]
  artifacts:
    - path/relativo/ao/artefato.md
  acceptance:
    - Critério de aceitação 1
    - Critério de aceitação 2
  estimated_time: X horas/dias
```

---

## Fase 0 — Foundation Fix (Imediata)

> **Meta**: Corrigir o que está quebrado HOJE. Populate failures.md, patterns.md, evolution.md, capability-profile.md, INDEX.md. Corrigir estágios 7-8 do metacognition pipeline.
> **CMI Gain**: +0.38 (baseline: 0.24 → 0.62)
> **Timeline**: 1-2 horas
> **Pré-requisito**: Nenhum (fase inicial)

```
┌──────────────────────────────────────────────────────────────────┐
│                    FASE 0 — DEPENDENCY MAP                       │
│                                                                  │
│  F0.1 ─────────────┐                                             │
│  Populate          │                                             │
│  failures.md       │                                             │
│                    ├──► F0.3 ──► F0.5                           │
│  F0.2 ─────────────┘    Update     Update                       │
│  Extract                evolution  INDEX.md                     │
│  patterns.md            .md                                     │
│                             │                                    │
│                             ▼                                    │
│                          F0.4                                    │
│                          Update                                  │
│                          capability-profile.md                   │
│                                                                  │
│  F0.6 (paralelo com todos acima)                                │
│  Fix metacognition pipeline stages 7-8                           │
│  ════════════════════════════════════                            │
│  Esta correção roda em paralelo porque afeta o runtime,         │
│  não os artefatos de memória. Mas deve completar antes           │
│  da Fase 1 (que depende do pipeline corrigido).                  │
└──────────────────────────────────────────────────────────────────┘
```

### Tarefas

```yaml
- id: F0.1
  phase: Fase 0
  block: Bloco 1 — Memória & Conhecimento
  title: "Popular failures.md com incidentes reais"
  owner: cosca-kernel
  collaborators: []
  cmi_gain: +0.08
  depends_on: []
  artifacts:
    - internal/embed/cosca/memory/failures.md
  acceptance:
    - Jail breach incident (2026-07) documentado com: contexto, causa raiz, impacto, lição, mitigação
    - Mínimo 3 incidentes/near-misses registrados
    - Cada entrada com #tags semânticas para busca cross-agent
    - Formato padronizado: título, data, severidade, causa, lição
  estimated_time: 20-30 min
```

```yaml
- id: F0.2
  phase: Fase 0
  block: Bloco 1 — Memória & Conhecimento
  title: "Extrair 3 patterns.md de experiências cross-agent"
  owner: cosca-kernel
  collaborators: [cosca-semantic-memory]
  cmi_gain: +0.08
  depends_on: []
  artifacts:
    - internal/embed/cosca/memory/patterns.md
  acceptance:
    - cross-agent-audit: padrão documentado (contexto, solução, confiança, vezes aplicado)
    - extract-then-test: padrão documentado para extração de conhecimento pré-validação
    - 3-wave-activation: padrão documentado para ativação progressiva de agentes
    - Cada padrão com #tags semânticas e metadados de aplicabilidade
  estimated_time: 20-30 min
```

```yaml
- id: F0.3
  phase: Fase 0
  block: Bloco 3 — Aprendizado & Evolução
  title: "Atualizar evolution.md — registrar L13-L21 como conquistas Level 4"
  owner: cosca-kernel
  collaborators: []
  cmi_gain: +0.06
  depends_on: [F0.1, F0.2]
  artifacts:
    - internal/embed/cosca/memory/evolution.md
  acceptance:
    - Entradas L13-L21 registradas como Level 4 achievements
    - Cada entrada com: técnica aplicada, domínio, outcome, confiança pós-task
    - Progressão documentada: Level 1→2→3→4 visível
    - Próximo objetivo (Level 5) definido com critérios claros
  estimated_time: 15-20 min
```

```yaml
- id: F0.4
  phase: Fase 0
  block: Bloco 3 — Aprendizado & Evolução
  title: "Atualizar capability-profile.md — Level 4, recalibrar confidence, adicionar dimensões CMI"
  owner: cosca-kernel
  collaborators: []
  cmi_gain: +0.06
  depends_on: [F0.3]
  artifacts:
    - internal/embed/cosca/memory/agent/cosca-kernel/capability-profile.md
  acceptance:
    - Current Level atualizado para 4
    - Confidence scores recalibrados com base em dados reais (não seed)
    - Dimensões CMI adicionadas: memory_health, decision_quality, learning_velocity, ethical_alignment
    - Per-domain confidence com trend indicators (↑↓→)
    - Strengths e Weaknesses atualizados com base nas 21 tarefas executadas
  estimated_time: 15-20 min
```

```yaml
- id: F0.5
  phase: Fase 0
  block: Bloco 3 — Aprendizado & Evolução
  title: "Atualizar INDEX.md com contagens reais"
  owner: cosca-kernel
  collaborators: []
  cmi_gain: +0.04
  depends_on: [F0.3]
  artifacts:
    - internal/embed/cosca/memory/INDEX.md
  acceptance:
    - Contagem real de learnings (não seed data estimada)
    - Contagem real de failures (≥ 3)
    - Contagem real de patterns (≥ 3)
    - Cross-references atualizadas entre arquivos de memória
  estimated_time: 10 min
```

```yaml
- id: F0.6
  phase: Fase 0
  block: Bloco 2 — Decisão & Raciocínio
  title: "Corrigir metacognition pipeline — estágios 7-8 DEVEM executar após cada task"
  owner: cosca-memory-chief
  collaborators: [cosca-evolution]
  cmi_gain: +0.06
  depends_on: []
  artifacts:
    - internal/embed/cosca/workflows/metacognition-pipeline.md
    - internal/embed/cosca/memory/LEARNING_PROTOCOL.md
  acceptance:
    - Stage 7 (EXTRACT PATTERN) disparado automaticamente ao final de cada task — sem exceção
    - Stage 8 (UPDATE CAPABILITY MODEL) disparado automaticamente ao final de cada task — sem exceção
    - Pipeline hook integrado no runtime (não depende de ação manual do agente)
    - Verificação: após qualquer task de qualquer agente, capability-profile.md é atualizado
    - Timeout máximo de 30s para stages 7-8 (não pode bloquear execução)
  estimated_time: 20-30 min
```

---

### Tabela Resumo — Fase 0

| # | Tarefa | Owner | Colaboradores | CMI | Artefato Principal | Tempo |
|---|--------|-------|---------------|-----|--------------------|-------|
| F0.1 | Popular failures.md | cosca-kernel | — | +0.08 | failures.md | 20-30min |
| F0.2 | Extrair patterns.md | cosca-kernel | cosca-semantic-memory | +0.08 | patterns.md | 20-30min |
| F0.3 | Atualizar evolution.md | cosca-kernel | — | +0.06 | evolution.md | 15-20min |
| F0.4 | Atualizar capability-profile.md | cosca-kernel | — | +0.06 | capability-profile.md | 15-20min |
| F0.5 | Atualizar INDEX.md | cosca-kernel | — | +0.04 | INDEX.md | 10min |
| F0.6 | Fix metacognition stages 7-8 | cosca-memory-chief | cosca-evolution | +0.06 | metacognition-pipeline.md | 20-30min |
| **Total** | **6 tarefas** | | | **+0.38** | | **~2h** |

---

### Critérios de Sucesso — Fase 0

- [ ] evolution.md mostra Level 4 com entradas L13-L21 documentadas
- [ ] failures.md tem ≥ 3 entradas (jail breach + 2 outros incidentes/near-misses)
- [ ] patterns.md tem ≥ 3 entradas (cross-agent-audit, extract-then-test, 3-wave-activation)
- [ ] capability-profile.md mostra Level 4 com confidence scores recalibrados
- [ ] INDEX.md reflete contagens reais (não estimativas)
- [ ] Stages 7-8 do metacognition pipeline executam automaticamente após cada task

---

## Fase 1 — Foundation (Curto Prazo)

> **Meta**: Construir as fundações da cognição: formatos de decisão, gates de raciocínio, métricas reais, detecção de gaps.
> **CMI Gain**: +0.22 (0.62 → 0.84)
> **Timeline**: 2-3 dias
> **Pré-requisito**: Fase 0 COMPLETA (especialmente F0.6 — pipeline stages 7-8 corrigidos)

```
┌──────────────────────────────────────────────────────────────────┐
│                    FASE 1 — DEPENDENCY MAP                       │
│                                                                  │
│  F1.1 ─────────────┐                                             │
│  Decision DNA      │                                             │
│  format            ├──►  F1.2 ──►  F1.3                         │
│                    │     Contrafactual  Proactive Gap            │
│  F1.4 ─────────────┘     Gate           Detection                │
│  Wisdom Decay                                                    │
│                                                                  │
│  F1.5 ────────────────────────────────────────────►  F1.6        │
│  Real Metrics Tracking (B1-B5)                      Cognitive    │
│  ════════════════════════════                      Entropy       │
│  (roda em paralelo com F1.1-F1.4)                  Metric        │
│                                                                  │
│  NOTA: F1.5 e F1.6 podem iniciar em paralelo com                 │
│  F1.1-F1.4 pois tocam sistemas diferentes                        │
│  (monitoring/analytics vs memory/decision).                      │
│  Mas F1.6 depende de F1.5 para ter dados.                        │
└──────────────────────────────────────────────────────────────────┘
```

### Tarefas

```yaml
- id: F1.1
  phase: Fase 1
  block: Bloco 1 — Memória & Conhecimento
  title: "Implementar formato Decision DNA (Decisão→Evidências→Riscos→Alternativas→Resultado)"
  owner: cosca-memory-chief
  collaborators: [cosca-critic]
  cmi_gain: +0.04
  depends_on: [F0.6]
  artifacts:
    - internal/embed/cosca/memory/DECISION_DNA.md
    - internal/embed/cosca/memory/decisions/*.md
  acceptance:
    - Template Decision DNA documentado com campos obrigatórios e opcionais
    - Todo agente registra decisões não-triviais no formato Decision DNA
    - Campos: Decisão, Evidências (com fontes), Riscos (probabilidade×impacto), Alternativas (consideradas e descartadas), Resultado (validado ou refutado)
    - Integration hook: após cada decisão, Decision DNA é persistido
    - Mínimo 3 decisões históricas registradas como exemplo
  estimated_time: 4-6 horas
```

```yaml
- id: F1.2
  phase: Fase 1
  block: Bloco 2 — Decisão & Raciocínio
  title: "Adicionar contrafactual gate ao pipeline de decisão"
  owner: cosca-critic
  collaborators: [cosca-architecture]
  cmi_gain: +0.04
  depends_on: [F1.1]
  artifacts:
    - internal/embed/cosca/engines/decision/contrafactual-gate.md
  acceptance:
    - Gate "what if the opposite?" implementado como passo obrigatório em decisões nível ≥ 3
    - Para cada decisão: agente deve articular explicitamente por que a alternativa OPOSTA foi descartada
    - Score de qualidade contrafactual: 0-1 baseado em profundidade da análise da alternativa oposta
    - Integrado ao metacognition pipeline Stage 3 (PLAN STRATEGY)
    - Exceção documentada: decisões operacionais simples (nível 1) não exigem gate
  estimated_time: 3-4 horas
```

```yaml
- id: F1.3
  phase: Fase 1
  block: Bloco 2 — Decisão & Raciocínio
  title: "Implementar proactive gap detection"
  owner: cosca-context
  collaborators: [cosca-discovery]
  cmi_gain: +0.04
  depends_on: [F1.2]
  artifacts:
    - internal/embed/cosca/engines/decision/gap-detection.md
  acceptance:
    - Sistema identifica gaps de conhecimento ANTES de agir (não após falha)
    - "Não sei" triggers: missing_dependency, unknown_technology, untested_assumption, conflicting_memory
    - Gap report gerado automaticamente antes de tarefas complexas (complexity ≥ 0.7)
    - Resolução de gap: escalate, research, delegate ou assume_risk (documentado)
    - Integrado ao metacognition pipeline Stage 1 (SELF-ASSESS)
  estimated_time: 4-6 horas
```

```yaml
- id: F1.4
  phase: Fase 1
  block: Bloco 1 — Memória & Conhecimento
  title: "Implementar Wisdom Decay — expiração de conhecimento + triggers de revalidação"
  owner: cosca-memory-chief
  collaborators: [cosca-semantic-memory]
  cmi_gain: +0.04
  depends_on: [F0.6]
  artifacts:
    - internal/embed/cosca/engines/wisdom-decay/WISDOM_DECAY.md
  acceptance:
    - Todo conhecimento tem TTL (time-to-live) baseado no domínio
    - TTL padrões: código (30d), tecnologia (90d), arquitetura (180d), segurança (30d), processo (365d)
    - Trigger de revalidação ao atingir 80% do TTL
    - Conhecimento expirado: marcado stale, não removido (referência histórica)
    - Revalidação: agente reexecuta a técnica/afirmação e atualiza ou deprecia
    - Métrica: staleness_score por domínio (0-1, onde 1 = tudo atualizado)
  estimated_time: 4-6 horas
```

```yaml
- id: F1.5
  phase: Fase 1
  block: Bloco 3 — Aprendizado & Evolução
  title: "Implementar tracking de métricas reais (B1-B5)"
  owner: cosca-monitoring
  collaborators: [cosca-analytics]
  cmi_gain: +0.04
  depends_on: [F0.6]
  artifacts:
    - internal/embed/cosca/metrics/cognitive-metrics.md
    - internal/embed/cosca/metrics/data/*.json
  acceptance:
    - B1 — Autonomous Problems Resolved: tracking operacional (tasks completadas sem intervenção humana)
    - B2 — Correct Predictions: tracking de predições validadas (agent previu outcome → outcome confirmado)
    - B3 — Reverted Decisions: tracking de decisões revertidas (decisão tomada → posteriormente revertida)
    - B4 — Reused Knowledge: tracking de conhecimento reutilizado cross-agent
    - B5 — Pre-failure Detection: tracking de falhas antecipadas pelo gap detection
    - Cada métrica com coleta automática (hook no metacognition pipeline Stage 8)
    - Dashboard mínimo: tabela semanal com tendências
  estimated_time: 5-8 horas
```

```yaml
- id: F1.6
  phase: Fase 1
  block: Bloco 3 — Aprendizado & Evolução
  title: "Criar métrica Cognitive Entropy (knowledge health dashboard)"
  owner: cosca-analytics
  collaborators: [cosca-memory-chief]
  cmi_gain: +0.02
  depends_on: [F1.5]
  artifacts:
    - internal/embed/cosca/engines/cognitive-entropy/ENTROPY.md
  acceptance:
    - Fórmula Cognitive Entropy definida: staleness × duplication × contradiction × gap_ratio
    - staleness: % de conhecimento expirado (Wisdom Decay)
    - duplication: entradas similares > 80% não condensadas
    - contradiction: afirmações conflitantes no knowledge graph
    - gap_ratio: domínios sem cobertura / total de domínios
    - Health score: 0 (entropia máxima, conhecimento caótico) a 1 (conhecimento curado, consistente)
    - Dashboard visual: score atual, tendência semanal, breakdown por domínio
    - Alerta automático quando entropy > 0.40
  estimated_time: 3-4 horas
```


---

### Tabela Resumo — Fase 1

| # | Tarefa | Owner | Colaboradores | CMI | Artefato Principal | Tempo |
|---|--------|-------|---------------|-----|--------------------|-------|
| F1.1 | Decision DNA format | cosca-memory-chief | cosca-critic | +0.04 | DECISION_DNA.md | 4-6h |
| F1.2 | Contrafactual gate | cosca-critic | cosca-architecture | +0.04 | contrafactual-gate.md | 3-4h |
| F1.3 | Proactive gap detection | cosca-context | cosca-discovery | +0.04 | gap-detection.md | 4-6h |
| F1.4 | Wisdom Decay | cosca-memory-chief | cosca-semantic-memory | +0.04 | engines/wisdom-decay/WISDOM_DECAY.md | 4-6h |
| F1.5 | Real metrics tracking (B1-B5) | cosca-monitoring | cosca-analytics | +0.04 | cognitive-metrics.md | 5-8h |
| F1.6 | Cognitive Entropy metric | cosca-analytics | cosca-memory-chief | +0.02 | engines/cognitive-entropy/ENTROPY.md | 3-4h |
| **Total** | **6 tarefas** | | | **+0.22** | | **~2-3d** |

---

### Critérios de Sucesso — Fase 1

- [ ] Decision DNA format implementado e integrado ao pipeline de decisão
- [ ] Contrafactual gate ativo para decisões nível ≥ 3
- [ ] Proactive gap detection funcional — identifica gaps ANTES da execução
- [ ] Wisdom Decay operacional: TTLs definidos, revalidação automática
- [ ] Todas as 5 métricas reais (B1-B5) com tracking operacional
- [ ] Acurácia de predições (B2) ≥ 50%
- [ ] Cognitive Entropy dashboard funcional com alertas
- [ ] Nenhum agente executa sem passar pelo gap detection (tasks complexidade ≥ 0.7)

---

## Fase 2 — Core Engines (Médio Prazo)

> **Meta**: Construir os 6 motores cognitivos centrais. ★ Cognitive Economy é o motor estrela.
> **CMI Gain**: +0.43 (0.84 → 1.27)
> **Timeline**: 1-2 semanas
> **Pré-requisito**: Fase 1 COMPLETA (métricas operacionais, Decision DNA, gates de raciocínio ativos)

```
┌──────────────────────────────────────────────────────────────────┐
│                    FASE 2 — DEPENDENCY MAP                       │
│                                                                  │
│  ┌─────────────────────────────────────────────────┐             │
│  │              PARALELO (Wave 1)                   │             │
│  │                                                  │             │
│  │  F2.1 ★ STAR                                    │             │
│  │  Cognitive Economy Engine                        │             │
│  │  ─────────────────────                           │             │
│  │  F2.2                                           │             │
│  │  Cognitive Immune System                         │             │
│  │  ─────────────────────                           │             │
│  │  F2.3                                           │             │
│  │  Cross-project Knowledge Transfer                │             │
│  │                                                  │             │
│  └───────────────────┬──────────────────────────────┘             │
│                      │                                            │
│                      ▼                                            │
│  ┌─────────────────────────────────────────────────┐             │
│  │              PARALELO (Wave 2)                   │             │
│  │                                                  │             │
│  │  F2.4                                           │             │
│  │  Cognitive Gravity System                        │             │
│  │  ─────────────────────                           │             │
│  │  F2.5                                           │             │
│  │  Cognitive Momentum Tracking                     │             │
│  │  ─────────────────────                           │             │
│  │  F2.6                                           │             │
│  │  Pattern Evolution System                        │             │
│  │                                                  │             │
│  └──────────────────────────────────────────────────┘             │
│                                                                  │
│  NOTA: Wave 1 (F2.1-F2.3) são motores independentes.            │
│  Wave 2 (F2.4-F2.6) estendem os motores da Wave 1.              │
│  Cognitive Economy (F2.1) é a STAR — maior impacto isolado.      │
└──────────────────────────────────────────────────────────────────┘
```

### Tarefas

```yaml
- id: F2.1
  phase: Fase 2
  block: Bloco 2 — Decisão & Raciocínio
  title: "★ STAR: Construir Cognitive Economy Engine (otimização cost×value para cada ação)"
  owner: cosca-architecture
  collaborators: [cosca-cto, cosca-performance, cosca-analytics]
  cmi_gain: +0.10
  depends_on: [F1.5]
  artifacts:
    - internal/embed/cosca/engines/cognitive-economy/SKILL.md
    - internal/embed/cosca/engines/cognitive-economy/cost-calculator.md
  acceptance:
    - Métrica de custo definida: tokens consumidos + tempo de execução + carga computacional + risco de falha
    - Métrica de valor definida: impacto no CMI + valor entregue ao Don + desbloqueio de tarefas dependentes
    - Fórmula: Efficiency = (Value / Cost). Ações com Efficiency < 0.3 são bloqueadas ou escaladas
    - Todo agente consulta o economy engine ANTES de executar — estimativa de custo obrigatória
    - Post-execução: custo real vs estimado registrado para calibração contínua
    - Dashboard de economia cognitiva: top 5 ações mais eficientes, bottom 5 menos eficientes
    - Integração com metacognition pipeline Stage 1 (SELF-ASSESS) e Stage 3 (PLAN STRATEGY)
  estimated_time: 3-4 dias
```

```yaml
- id: F2.2
  phase: Fase 2
  block: Bloco 2 — Decisão & Raciocínio
  title: "Construir Cognitive Immune System (validação proativa de conhecimento antes da ingestão)"
  owner: cosca-security
  collaborators: [cosca-qa, cosca-memory-chief]
  cmi_gain: +0.08
  depends_on: [F1.4]
  artifacts:
    - internal/embed/cosca/engines/cognitive-immune/IMMUNE_SYSTEM.md
  acceptance:
    - Validação de 3 camadas antes da ingestão de conhecimento: source_check → consistency_check → contradiction_check
    - source_check: a fonte é confiável? (Evidence Confidence Model da Fase B)
    - consistency_check: o conhecimento é consistente com o código executado?
    - contradiction_check: o conhecimento contradiz conhecimento existente de confiança superior?
    - Modo de quarentena: conhecimento que falha validação é isolado em quarantine/ para revisão humana
    - Detecção de contamination attempt: sistema detecta e rejeita pelo menos 1 tentativa de contaminação
    - Log de rejeições: o que foi rejeitado, por que, quando
    - Integração com metacognition pipeline Stage 2 (RETRIEVE MEMORY) — conhecimento em quarentena NÃO é carregado
  estimated_time: 2-3 dias
```

```yaml
- id: F2.3
  phase: Fase 2
  block: Bloco 1 — Memória & Conhecimento
  title: "Implementar cross-project knowledge transfer (federação de memória semântica)"
  owner: cosca-semantic-memory
  collaborators: [cosca-memory-chief, cosca-integrations]
  cmi_gain: +0.07
  depends_on: [F1.1]
  artifacts:
    - internal/embed/cosca/engines/semantic-federation/FEDERATION.md
  acceptance:
    - Federação de memória entre projetos Cosca ativos
    - Índice compartilhado: patterns.md e learnings.md cross-project indexados
    - Busca semântica cross-project: agentes podem consultar conhecimento de outros projetos
    - Namespace por projeto: conhecimento mantém origem (projeto de origem rastreável)
    - Privacidade: projetos podem marcar conhecimento como private (não federado)
    - Deduplicação cross-project: padrões idênticos em múltiplos projetos são consolidados
    - Métrica: cross-project knowledge reuse rate (B4 expandido)
  estimated_time: 2-3 dias
```

```yaml
- id: F2.4
  phase: Fase 2
  block: Bloco 1 — Memória & Conhecimento
  title: "Construir Cognitive Gravity system (influência de decisão ponderada por validação)"
  owner: cosca-memory-chief
  collaborators: [cosca-semantic-memory, cosca-critic]
  cmi_gain: +0.06
  depends_on: [F2.1, F2.3]
  artifacts:
    - internal/embed/cosca/engines/cognitive-gravity/SKILL.md
  acceptance:
    - Cada conhecimento/padrão tem massa gravitacional: validation_count × success_rate × recency
    - Quanto maior a massa gravitacional, mais influência o conhecimento exerce nas decisões
    - Gravidade decai com distância semântica: conhecimento de domínio próximo tem mais peso
    - Orbital decay: conhecimento não utilizado perde massa gravitacional gradualmente
    - Colisão de conhecimento: quando duas afirmações conflitam, vence a de maior massa gravitacional
    - Visualização: gravity map mostrando clusters de conhecimento e sua influência relativa
  estimated_time: 2-3 dias
```

```yaml
- id: F2.5
  phase: Fase 2
  block: Bloco 3 — Aprendizado & Evolução
  title: "Implementar Cognitive Momentum tracking (métricas de tração por domínio)"
  owner: cosca-analytics
  collaborators: [cosca-monitoring]
  cmi_gain: +0.06
  depends_on: [F2.1]
  artifacts:
    - internal/embed/cosca/engines/cognitive-momentum/MOMENTUM.md
  acceptance:
    - Momentum por domínio: tasks_completed × avg_complexity × success_rate / time_window
    - Aceleração: delta de momentum entre janelas consecutivas (positiva = acelerando, negativa = desacelerando)
    - Inércia cognitiva: domínios com alto momentum tendem a manter momentum (auto-reforço)
    - Friction detection: identifica domínios onde o momentum está caindo (alerta precoce)
    - Dashboard de momentum: mapa de calor dos domínios (verde = acelerando, vermelho = desacelerando)
    - Integração com Cognitive Economy: domínios com momentum positivo recebem prioridade de alocação
  estimated_time: 1-2 dias
```

```yaml
- id: F2.6
  phase: Fase 2
  block: Bloco 1 — Memória & Conhecimento
  title: "Construir Pattern Evolution system (padrões versionados com ciclo de vida de depreciação)"
  owner: cosca-memory-chief
  collaborators: [cosca-evolution]
  cmi_gain: +0.06
  depends_on: [F2.3, F2.4]
  artifacts:
    - internal/embed/cosca/engines/pattern-evolution/PATTERN_LIFECYCLE.md
  acceptance:
    - Padrões versionados: v1, v2, v3 com changelog de evolução
    - Ciclo de vida: proposed → validated → stable → deprecated → retired
    - Depreciação automática: padrão não utilizado por 90 dias entra em deprecated
    - Substituição: quando um padrão vN+1 é validado, vN é marcado como superseded (não removido)
    - Métrica de health do padrão: usage_count × avg_success_rate × recency
    - Breaking change detection: padrão que causa regressão é revertido e marcado para revisão
    - Catálogo de padrões: visualização da árvore de evolução de cada padrão
  estimated_time: 2-3 dias
```


---

### Tabela Resumo — Fase 2

| # | Tarefa | Owner | Colaboradores | CMI | Artefato Principal | Tempo |
|---|--------|-------|---------------|-----|--------------------|-------|
| F2.1 ★ | Cognitive Economy Engine | cosca-architecture | cosca-cto, cosca-performance, cosca-analytics | +0.10 | SKILL.md | 3-4d |
| F2.2 | Cognitive Immune System | cosca-security | cosca-qa, cosca-memory-chief | +0.08 | IMMUNE_SYSTEM.md | 2-3d |
| F2.3 | Cross-project Knowledge Transfer | cosca-semantic-memory | cosca-memory-chief, cosca-integrations | +0.07 | FEDERATION.md | 2-3d |
| F2.4 | Cognitive Gravity System | cosca-architecture | cosca-semantic-memory, cosca-critic | +0.06 | SKILL.md | 2-3d |
| F2.5 | Cognitive Momentum Tracking | cosca-analytics | cosca-monitoring | +0.06 | MOMENTUM.md | 1-2d |
| F2.6 | Pattern Evolution System | cosca-memory-chief | cosca-evolution | +0.06 | PATTERN_LIFECYCLE.md | 2-3d |
| **Total** | **6 tarefas** | | | **+0.43** | | **~1-2sem** |

---

### Critérios de Sucesso — Fase 2

- [ ] ★ Cognitive Economy Engine operacional: toda ação tem estimativa de custo×valor
- [ ] Cognitive Immune System ativo: validação de 3 camadas antes de ingestão de conhecimento
- [ ] Immune System detecta e rejeita ≥ 1 contamination attempt (teste com dados contraditórios)
- [ ] Cross-project knowledge transfer funcional entre ≥ 2 projetos
- [ ] Cognitive Gravity System influencia decisões com pesos baseados em validação
- [ ] Cognitive Momentum dashboard mostra aceleração/desaceleração por domínio
- [ ] Pattern Evolution System gerencia ciclo de vida completo dos padrões
- [ ] Métrica B4 (Reused Knowledge) expandida para cross-project

---

## Fase 3 — Advanced Cognition (Longo Prazo)

> **Meta**: Cognição avançada — Insight Generator, 2nd-order reasoning, Cognitive Compression, Adaptive Personality, Mental Energy, Cognitive Horizon, Ecosystem Dynamics.
> **CMI Gain**: +0.68 (1.27 → 1.95)
> **Timeline**: 2-4 semanas
> **Pré-requisito**: Fase 2 COMPLETA (todos os 6 motores centrais operacionais)

```
┌──────────────────────────────────────────────────────────────────┐
│                    FASE 3 — DEPENDENCY MAP                       │
│                                                                  │
│  ┌─────────────────────────────────────────────────┐             │
│  │              PARALELO (Wave 1)                   │             │
│  │                                                  │             │
│  │  F3.1                                           │             │
│  │  Insight Generator                               │             │
│  │  (depende: F2.3 Federation + F2.6 Patterns)      │             │
│  │  ─────────────────────                           │             │
│  │  F3.2                                           │             │
│  │  Cognitive Compression                           │             │
│  │  (depende: F2.4 Gravity)                         │             │
│  │  ─────────────────────                           │             │
│  │  F3.3                                           │             │
│  │  2nd-Order Reasoning Engine                      │             │
│  │  (depende: F2.1 Economy + F1.2 Contrafactual)    │             │
│  │                                                  │             │
│  └───────────────────┬──────────────────────────────┘             │
│                      │                                            │
│                      ▼                                            │
│  ┌─────────────────────────────────────────────────┐             │
│  │              PARALELO (Wave 2)                   │             │
│  │                                                  │             │
│  │  F3.4                                           │             │
│  │  Adaptive Personality                            │             │
│  │  (depende: F2.5 Momentum + F3.1 Insights)        │             │
│  │  ─────────────────────                           │             │
│  │  F3.5                                           │             │
│  │  Mental Energy Allocation                        │             │
│  │  (depende: F2.1 Economy)                         │             │
│  │  ─────────────────────                           │             │
│  │  F3.6                                           │             │
│  │  Cognitive Horizon Metrics                       │             │
│  │  (depende: F2.5 Momentum + F3.3 Reasoning)       │             │
│  │                                                  │             │
│  └───────────────────┬──────────────────────────────┘             │
│                      │                                            │
│                      ▼                                            │
│  ┌─────────────────────────────────────────────────┐             │
│  │              FINAL (Wave 3)                      │             │
│  │                                                  │             │
│  │  F3.7                                           │             │
│  │  Cognitive Ecosystem Dynamics                    │             │
│  │  (depende: TODOS F3.1-F3.6)                      │             │
│  │                                                  │             │
│  └──────────────────────────────────────────────────┘             │
│                                                                  │
│  NOTA: Wave 1 são motores de descoberta (independem entre si).   │
│  Wave 2 usa os outputs da Wave 1.                                │
│  Wave 3 (Ecosystem) é o integrador final — só funciona           │
│  quando todos os outros motores estão operando em harmonia.       │
└──────────────────────────────────────────────────────────────────┘
```

### Tarefas

```yaml
- id: F3.1
  phase: Fase 3
  block: Bloco 3 — Aprendizado & Evolução
  title: "Construir Insight Generator (motor de descoberta proativa de padrões)"
  owner: cosca-ai
  collaborators: [cosca-evolution, cosca-semantic-memory]
  cmi_gain: +0.12
  depends_on: [F2.3, F2.6]
  artifacts:
    - internal/embed/cosca/engines/insight-generator/SKILL.md
  acceptance:
    - Motor proativo: não espera tasks — analisa continuamente o knowledge graph em busca de padrões emergentes
    - Técnicas: clustering semântico, detecção de anomalias, análise de correlação cross-domain
    - Output: insight reports com: descoberta, evidências, confiança, ações sugeridas
    - Meta: produzir ≥ 1 descoberta/semana que gere ação (otimização, correção, nova capacidade)
    - Validação de insight: todo insight gerado é testado contra dados históricos antes de ser aceito
    - Integração com Decision DNA: insights que geram decisões são rastreados
    - Dashboard de descobertas: insights ativos, validados, rejeitados, pendentes
  estimated_time: 3-5 dias
```

```yaml
- id: F3.2
  phase: Fase 3
  block: Bloco 1 — Memória & Conhecimento
  title: "Construir Cognitive Compression (N casos → 1 princípio universal)"
  owner: cosca-ai
  collaborators: [cosca-memory-chief, cosca-evolution]
  cmi_gain: +0.10
  depends_on: [F2.4]
  artifacts:
    - internal/embed/cosca/engines/cognitive-compression/SKILL.md
  acceptance:
    - Algoritmo de compressão: identifica N casos similares e extrai 1 princípio universal
    - Princípio extraído é testado contra TODOS os casos originais (deve explicar todos)
    - Métrica de compressão: compression_ratio = (N casos → 1 princípio) / N
    - Princípios são versionados e evoluem (integrado com Pattern Evolution F2.6)
    - Compressão reversível: é possível expandir o princípio de volta aos casos originais
    - Gatilho automático: quando ≥ 5 padrões similares existem, compression engine é acionado
    - Redução de entropia: Cognitive Entropy (F1.6) deve cair após compressão
  estimated_time: 3-4 dias
```

```yaml
- id: F3.3
  phase: Fase 3
  block: Bloco 2 — Decisão & Raciocínio
  title: "Implementar 2nd-order reasoning engine (projeção forward 10+ passos)"
  owner: cosca-architecture
  collaborators: [cosca-critic, cosca-cto]
  cmi_gain: +0.12
  depends_on: [F2.1, F1.2]
  artifacts:
    - internal/embed/cosca/engines/second-order-reasoning/REASONING.md
  acceptance:
    - Árvore de consequências: para cada decisão, projeta consequências N passos à frente (N >= 10)
    - Cada passo avalia: probabilidade, impacto, dependências, efeitos colaterais
    - Poda de ramos: ramos com probabilidade acumulada < 5% são podados (eficiência cognitiva)
    - Feedback loop: consequências reais são comparadas com projeções para calibrar o motor
    - Integração com contrafactual gate (F1.2): análise contrafactual estendida para N passos
    - Visualização: árvore de decisão com heatmap de risco
    - Validação: acurácia de projeção ≥ 60% para horizonte de 5 passos
  estimated_time: 4-6 dias
```

```yaml
- id: F3.4
  phase: Fase 3
  block: Bloco 1 — Memória & Conhecimento
  title: "Implementar Adaptive Personality (ajuste de estratégia sensível ao contexto)"
  owner: cosca-context
  collaborators: [cosca-runtime, cosca-ai]
  cmi_gain: +0.10
  depends_on: [F2.5, F3.1]
  artifacts:
    - internal/embed/cosca/engines/adaptive-personality/SKILL.md
  acceptance:
    - Perfis de personalidade adaptativa: cautious, balanced, aggressive, experimental
    - Seleção automática baseada em: criticidade da tarefa, confiança do domínio, momentum, histórico de falhas
    - cautious: tarefas de segurança, confiança < 0.6, domínio com falhas recentes
    - aggressive: tarefas de alta confiança (>0.85), domínio com momentum positivo, baixo risco
    - experimental: domínios novos, exploração de padrões, Insight Generator ativo
    - Transições suaves: personalidade não muda abruptamente entre tarefas similares
    - Métrica de efetividade: taxa de sucesso por personalidade × domínio
    - Feedback loop: personalidade é ajustada com base em outcomes (reforço positivo/negativo)
  estimated_time: 3-4 dias
```

```yaml
- id: F3.5
  phase: Fase 3
  block: Bloco 3 — Aprendizado & Evolução
  title: "Construir Mental Energy allocation model (orçamento dinâmico de recursos)"
  owner: cosca-runtime
  collaborators: [cosca-cto, cosca-analytics]
  cmi_gain: +0.08
  depends_on: [F2.1]
  artifacts:
    - internal/embed/cosca/engines/mental-energy/ENERGY_MODEL.md
  acceptance:
    - Energy pool global: orçamento total de tokens/tempo por ciclo (hora/dia)
    - Alocação dinâmica: tarefas críticas recebem mais energia, tarefas de baixo valor recebem menos
    - Energy cost por ação: tokens estimados × complexidade × risco
    - Energy recovery: após tarefas intensas, sistema reduz alocação temporariamente (cooldown)
    - Energy debt: tarefas podem "pegar emprestado" energia futura com juros (prioridade justificada)
    - Métrica de eficiência energética: valor_entregue / energia_consumida
    - Integração com Cognitive Economy (F2.1): energy é uma dimensão do cost
    - Alerta de exaustão: quando energy pool < 20%, escala para revisão humana
  estimated_time: 3-4 dias
```

```yaml
- id: F3.6
  phase: Fase 3
  block: Bloco 3 — Aprendizado & Evolução
  title: "Implementar Cognitive Horizon metrics (medição de profundidade preditiva)"
  owner: cosca-analytics
  collaborators: [cosca-ai]
  cmi_gain: +0.08
  depends_on: [F2.5, F3.3]
  artifacts:
    - internal/embed/cosca/metrics/cognitive-horizon.md
  acceptance:
    - Horizon depth: quantos passos à frente o sistema consegue prever com acurácia ≥ 60%
    - Horizon breadth: quantos domínios diferentes são cobertos pelas projeções
    - Horizon clarity: quão precisa é a projeção (desvio entre previsto e real)
    - Horizon velocity: taxa de expansão do horizonte (passos/dia)
    - Métrica composta: HorizonScore = depth × breadth × clarity × velocity
    - Dashboard: horizon radar mostrando depth, breadth, clarity, velocity em eixos
    - Meta: horizon depth ≥ 10 passos com clarity ≥ 0.6
  estimated_time: 2-3 dias
```

```yaml
- id: F3.7
  phase: Fase 3
  block: Bloco 4 — Ética & Governança
  title: "Ativar Cognitive Ecosystem dynamics (influência mútua entre todos os componentes)"
  owner: cosca-architecture
  collaborators: [cosca-cto, cosca-kernel, cosca-governance]
  cmi_gain: +0.08
  depends_on: [F3.1, F3.2, F3.3, F3.4, F3.5, F3.6]
  artifacts:
    - internal/embed/cosca/engines/ecosystem-dynamics/SKILL.md
  acceptance:
    - Mapa de interdependências: todos os 21 motores com conexões de influência documentadas
    - Feedback loops conscientes: o sistema reconhece e monitora loops de feedback entre motores
    - Homeostase cognitiva: perturbações em um motor são compensadas por ajustes em motores conectados
    - Emergência monitorada: propriedades emergentes (não previstas) são detectadas e avaliadas
    - Ecosystem health score: métrica holística que agrega saúde de todos os motores
    - Governança adaptativa: regras de operação evoluem com base no estado do ecossistema
    - Kill switch: capacidade de desligar motores individuais sem colapsar o ecossistema
    - Documentação do ecossistema: diagrama de influências com pesos e direções
  estimated_time: 3-5 dias
```


---

### Tabela Resumo — Fase 3

| # | Tarefa | Owner | Colaboradores | CMI | Artefato Principal | Tempo |
|---|--------|-------|---------------|-----|--------------------|-------|
| F3.1 | Insight Generator | cosca-ai | cosca-evolution, cosca-semantic-memory | +0.12 | SKILL.md | 3-5d |
| F3.2 | Cognitive Compression | cosca-ai | cosca-memory-chief, cosca-evolution | +0.10 | SKILL.md | 3-4d |
| F3.3 | 2nd-Order Reasoning Engine | cosca-architecture | cosca-critic, cosca-cto | +0.12 | REASONING.md | 4-6d |
| F3.4 | Adaptive Personality | cosca-architecture | cosca-runtime, cosca-ai | +0.10 | SKILL.md | 3-4d |
| F3.5 | Mental Energy Allocation | cosca-runtime | cosca-cto, cosca-analytics | +0.08 | ENERGY_MODEL.md | 3-4d |
| F3.6 | Cognitive Horizon Metrics | cosca-analytics | cosca-ai | +0.08 | cognitive-horizon.md | 2-3d |
| F3.7 | Cognitive Ecosystem Dynamics | cosca-architecture | cosca-cto, cosca-kernel, cosca-governance | +0.08 | ECOSYSTEM.md | 3-5d |
| **Total** | **7 tarefas** | | | **+0.68** | | **~2-4sem** |

---

### Critérios de Sucesso — Fase 3

- [ ] Insight Generator produzindo ≥ 1 descoberta/semana que gera ação concreta
- [ ] Cognitive Compression reduzindo ≥ 5 padrões similares em 1 princípio universal
- [ ] 2nd-order reasoning validado: acurácia ≥ 60% para horizonte de 5 passos
- [ ] Adaptive Personality ajustando estratégia automaticamente por contexto
- [ ] Mental Energy model funcional: alocação dinâmica com recovery e debt management
- [ ] Cognitive Horizon metrics operacionais: depth ≥ 10 passos com clarity ≥ 0.6
- [ ] Ecosystem Dynamics ativo: homeostase cognitiva com feedback loops conscientes
- [ ] CMI ≥ 95% (1.95 no índice)
- [ ] Todos os 21 motores operacionais e integrados

---

## Agentes Envolvidos — Alocação Completa

### Agentes Primários (por frequência de ownership)

| Agente | Tarefas como Owner | Fases | Carga Total Estimada |
|--------|-------------------|-------|---------------------|
| **cosca-kernel** | 5 (F0.1-F0.5) | Fase 0 | ~2h |
| **cosca-memory-chief** | 3 (F0.6, F1.1, F1.4, F2.4, F2.6) | Fases 0-2 | ~3-4d |
| **cosca-architecture** | 2 (F2.1, F3.3, F3.7) | Fases 2-3 | ~7-10d |
| **cosca-analytics** | 3 (F1.6, F2.5, F3.6) | Fases 1-3 | ~4-6d |
| **cosca-ai** | 2 (F3.1, F3.2) | Fase 3 | ~6-9d |
| **cosca-critic** | 1 (F1.2) | Fase 1 | ~3-4h |
| **cosca-context** | 2 (F1.3, F3.4) | Fases 1, 3 | ~3-5d |
| **cosca-monitoring** | 1 (F1.5) | Fase 1 | ~5-8h |
| **cosca-security** | 1 (F2.2) | Fase 2 | ~2-3d |
| **cosca-semantic-memory** | 1 (F2.3) | Fase 2 | ~2-3d |
| **cosca-runtime** | 1 (F3.5) | Fase 3 | ~3-4d |

### Agentes Colaboradores (compartilham execução)

| Agente | Tarefas | Tipo de Colaboração |
|--------|---------|---------------------|
| cosca-cto | F2.1, F3.3, F3.5, F3.7 | Revisão técnica e aprovação de arquitetura |
| cosca-evolution | F0.6, F2.6, F3.1, F3.2 | Evolução de padrões e aprendizado |
| cosca-semantic-memory | F0.2, F1.4, F2.4, F3.1 | Memória semântica e federação |
| cosca-performance | F2.1 | Otimização de custo computacional |
| cosca-qa | F2.2 | Validação de qualidade do conhecimento |
| cosca-integrations | F2.3 | Integração cross-project |
| cosca-governance | F3.7 | Regras de ecossistema |
| cosca-discovery | F1.3 | Detecção de gaps |
| cosca-runtime | F3.4 | Execução adaptativa |

---

## Timeline Completa

```
Semana 1        Semana 2        Semana 3        Semana 4        Semana 5+
─────┬─────────────┬─────────────┬─────────────┬─────────────┬─────────────►

Fase 0 ██
(1-2h)

Fase 1 ████████████
(2-3d)

Fase 2 ████████████████████████████████
(1-2sem)                                           (pode estender)

Fase 3 ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
(2-4sem)                                           (inicia após Fase 2)

LEGENDA:
  ██  Fase concluída
  ░░  Fase em andamento
  ──  Buffer/margem
```

### Marcos Principais (Milestones)

| Marco | Quando | O que verifica |
|-------|--------|---------------|
| **M1: Foundation Fixed** | Fim da Fase 0 (Dia 1, Hora 2) | evolution.md Level 4, failures.md ≥ 3, patterns.md ≥ 3, pipeline corrigido |
| **M2: Cognitive Baseline** | Fim da Fase 1 (Dia 3-4) | B1-B5 operacionais, Decision DNA ativo, contrafactual gate funcional, gap detection ativo |
| **M3: Engines Online** | Fim da Fase 2 (Semana 2-3) | Cognitive Economy rodando, Immune System ativo, Gravity influenciando decisões |
| **M4: Advanced Cognition** | Fim da Fase 3 (Semana 4-6) | Insight Generator produzindo, 2nd-order reasoning validado, Ecosystem integrado |
| **M5: CMI Target** | Fim da Fase 3 | CMI ≥ 95% (1.95), todos os 21 motores operacionais |

---

## Matriz de Dependências Cross-Phase

```
┌─────────────────────────────────────────────────────────────────────┐
│              CROSS-PHASE DEPENDENCY MATRIX                          │
│                                                                     │
│  Legenda: ■ = bloqueia diretamente, □ = habilita (soft dependency)  │
│                                                                     │
│  Fase 0 ──────► Fase 1:  F0.6 ■ F1.1, F1.4, F1.5                  │
│                         F0.1-F0.2 □ F1.1 (memória populada)        │
│                                                                     │
│  Fase 1 ──────► Fase 2:  F1.1 ■ F2.3 (Decision DNA → Federation)   │
│                         F1.2 ■ F3.3 (Contrafactual → 2nd-order)    │
│                         F1.4 ■ F2.2 (Wisdom Decay → Immune)        │
│                         F1.5 ■ F2.1, F2.5 (Métricas → Economy,     │
│                                                Momentum)           │
│                                                                     │
│  Fase 2 ──────► Fase 3:  F2.1 ■ F3.3, F3.5 (Economy → Reasoning,  │
│                                                Energy)             │
│                         F2.3 ■ F3.1 (Federation → Insights)        │
│                         F2.4 ■ F3.2 (Gravity → Compression)        │
│                         F2.5 ■ F3.4, F3.6 (Momentum → Personality, │
│                                              Horizon)              │
│                         F2.6 ■ F3.1 (Patterns → Insights)          │
│                                                                     │
│  Fase 3 ──────► F3.7:   F3.1-F3.6 ■ F3.7 (Todos → Ecosystem)      │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Protocolo de Memória e Aprendizado

### O que registrar após cada tarefa concluída

```yaml
memory_entry:
  agent: "{agent_name}"
  task_id: "{Fase.N}"
  outcome: success | partial | failure
  confidence_delta: +0.XX (ou -0.XX)
  technique_level: N
  cmi_contribution: "+0.XX"
  lessons:
    - "O que funcionou bem"
    - "O que poderia ser melhor"
    - "Surpresas encontradas"
  tags: ["#cognitive-maturity", "#fase-N", "#{block}"]
  artifacts_created:
    - path/para/artefato
  next_actions:
    - "Próxima tarefa dependente"
```

### Atualizações automáticas ao concluir cada tarefa

1. **learnings.md** do agente executor: nova entrada com tags semânticas
2. **capability-profile.md** do agente executor: recalibrar confidence score
3. **INDEX.md** global: atualizar contagem de artefatos
4. **evolution.md** (a partir da Fase 1): registrar milestone
5. **cognitive-metrics.md** (a partir da Fase 1): atualizar métricas B1-B5
6. **engines/cognitive-entropy/ENTROPY.md** (a partir da Fase 1): recalcular entropy score

### Anti-padrões a EVITAR

| Anti-padrão | Por que evitar | O que fazer em vez disso |
|-------------|---------------|-------------------------|
| Pular Fase 0 | Memória vazia = pipeline cego | Completar F0 integralmente antes de qualquer F1 |
| Implementar Fase 2 sem métricas (F1.5) | Sem medição = sem calibração | F1.5 é pré-requisito para F2.1, F2.5 |
| Ignorar contrafactual gate (F1.2) | Decisões sem contraponto = viés de confirmação | Gate integrado ao pipeline, sem bypass |
| Cognitive Economy sem cost tracking real | Otimização cega = pior que nenhuma otimização | F1.5 fornece dados reais para F2.1 calibrar |
| Pattern Evolution sem versionamento (F2.6) | Padrões evoluem sem rastro = regressão silenciosa | Versionamento desde o primeiro padrão |
| Insight Generator sem validação (F3.1) | Insights falsos poluem o knowledge graph | Todo insight é validado contra dados históricos |
| Ecosystem Dynamics sem kill switch (F3.7) | Sistema integrado sem controle de danos | Cada motor tem kill switch independente |

---

## Glossário de Termos

| Termo | Definição |
|-------|-----------|
| **CMI** | Cognitive Maturity Index — índice composto de maturidade cognitiva (0.0 a 2.0+) |
| **Bloco Cognitivo** | Agrupamento lógico de capacidades: Memória, Decisão, Aprendizado, Ética |
| **Decision DNA** | Formato estruturado para registro de decisões (Decisão→Evidências→Riscos→Alternativas→Resultado) |
| **Contrafactual Gate** | Verificação obrigatória: "e se o oposto for verdade?" antes de decisões importantes |
| **Wisdom Decay** | Sistema de expiração de conhecimento com TTL por domínio e revalidação |
| **Cognitive Economy** | Motor de otimização cost×value para cada ação do sistema |
| **Cognitive Immune System** | Validação de 3 camadas antes da ingestão de novo conhecimento |
| **Cognitive Gravity** | Sistema de pesos onde conhecimento validado exerce mais influência em decisões |
| **Cognitive Momentum** | Métrica de tração por domínio (tasks × complexidade × sucesso / tempo) |
| **Cognitive Compression** | Algoritmo que extrai 1 princípio universal de N casos similares |
| **Cognitive Horizon** | Profundidade preditiva do sistema (quantos passos à frente com acurácia) |
| **Cognitive Entropy** | Métrica de saúde do conhecimento (staleness × duplication × contradiction × gaps) |
| **Mental Energy** | Modelo de orçamento dinâmico de recursos computacionais/cognitivos |
| **Ecosystem Dynamics** | Sistema de influência mútua e homeostase entre todos os motores cognitivos |

---

## Referências

### Documentos Relacionados

| Documento | Relação com este Workflow |
|-----------|--------------------------|
| `internal/embed/cosca/workflows/metacognition-pipeline.md` | Pipeline que as tarefas F0.6 e F1.x integram |
| `internal/embed/cosca/memory/agent/cosca-kernel/evolution.md` | Destino das atualizações F0.3 |
| `internal/embed/cosca/memory/agent/cosca-kernel/capability-profile.md` | Destino das atualizações F0.4 |
| `internal/embed/cosca/memory/failures.md` | Populado por F0.1 |
| `internal/embed/cosca/memory/patterns.md` | Populado por F0.2 |
| `internal/embed/cosca/memory/INDEX.md` | Atualizado por F0.5 |
| `internal/embed/cosca/memory/LEARNING_PROTOCOL.md` | Protocolo de aprendizado referenciado por F0.6 |
| `internal/embed/cosca/CONSTITUTION.md` | Princípios imutáveis que governam todas as fases |
| `internal/embed/cosca/AGENT_DNA.md` | DNA dos agentes — referência para capability profiles |
| `internal/embed/cosca/workflows/cosca-evolution-autonomy.md` | Workflow predecessor — Fases A-E que inspiram este pipeline |

### Engines a Serem Criadas

| Engine | Fase | Path |
|--------|------|------|
| Decision DNA | F1.1 | `internal/embed/cosca/engines/decision/DECISION_DNA.md` |
| Contrafactual Gate | F1.2 | `internal/embed/cosca/engines/decision/contrafactual-gate.md` |
| Gap Detection | F1.3 | `internal/embed/cosca/engines/decision/gap-detection.md` |
| Wisdom Decay | F1.4 | `internal/embed/cosca/engines/wisdom-decay/WISDOM_DECAY.md` |
| Cognitive Economy ★ | F2.1 | `internal/embed/cosca/engines/cognitive-economy/SKILL.md` |
| Cognitive Immune System | F2.2 | `internal/embed/cosca/engines/cognitive-immune/IMMUNE_SYSTEM.md` |
| Semantic Federation | F2.3 | `internal/embed/cosca/engines/semantic-federation/FEDERATION.md` |
| Cognitive Gravity | F2.4 | `internal/embed/cosca/engines/cognitive-gravity/SKILL.md` |
| Cognitive Momentum | F2.5 | `internal/embed/cosca/engines/cognitive-momentum/MOMENTUM.md` |
| Pattern Evolution | F2.6 | `internal/embed/cosca/engines/pattern-evolution/PATTERN_LIFECYCLE.md` |
| Insight Generator | F3.1 | `internal/embed/cosca/engines/insight-generator/SKILL.md` |
| Cognitive Compression | F3.2 | `internal/embed/cosca/engines/cognitive-compression/SKILL.md` |
| 2nd-Order Reasoning | F3.3 | `internal/embed/cosca/engines/second-order-reasoning/REASONING.md` |
| Adaptive Personality | F3.4 | `internal/embed/cosca/engines/adaptive-personality/SKILL.md` |
| Mental Energy | F3.5 | `internal/embed/cosca/engines/mental-energy/ENERGY_MODEL.md` |
| Ecosystem Dynamics | F3.7 | `internal/embed/cosca/engines/ecosystem-dynamics/SKILL.md` |

---

## Histórico de Versões

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-30 | Cosca Kernel (via cosca-workflow-chief) | Workflow inicial: 4 fases, 21 capacidades, 4 blocos cognitivos. CMI pipeline completo documentado. |

---

> **Próximo passo após aprovação**: Iniciar Fase 0 imediatamente. F0.1 e F0.2 podem ser executados em paralelo.
> **Kernel instruction**: `cosca-kernel execute workflow cognitive-maturity-implementation --phase F0`
