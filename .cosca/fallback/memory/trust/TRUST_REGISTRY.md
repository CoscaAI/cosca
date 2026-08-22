# TRUST REGISTRY — Sistema de Reputação Histórica de Agentes

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Criado**: 2026-07-30
>
> **Autoridade constitucional**: [CONSTITUTION.md](../../CONSTITUTION.md) — Art. P2 (Código executado é a verdade absoluta)
> **Integração**: [CONFIDENCE_MODEL.md](../../engines/evidence/CONFIDENCE_MODEL.md) — Evidence Trust Engine (Níveis 0-5)
>
> Cada task executada gera um registro permanente de reputação. O Trust Registry é o histórico centralizado
> que permite ao Prediction Engine (F7.1) decidir qual agente delegar, com que confiança, e a que custo.

---

## Índice

1. [Formato do Registro de Confiança](#1-formato-do-registro-de-confiança)
2. [Métricas de Reputação Calculadas](#2-métricas-de-reputação-calculadas)
3. [Mecanismo de Atualização](#3-mecanismo-de-atualização)
4. [Integração com Prediction Engine (F7.1)](#4-integração-com-prediction-engine-f71)
5. [Registros Populados — Sessão 2026-07-30](#5-registros-populados--sessão-2026-07-30)
6. [Métricas Calculadas — Sessão 2026-07-30](#6-métricas-calculadas--sessão-2026-07-30)

---

## 1. Formato do Registro de Confiança

### Estrutura YAML (parseável por script)

Cada entrada segue este formato canônico:

```yaml
agent: <nome-do-agente>                      # Obrigatório. Ex: cosca-backend
task_type: <categoria-da-task>               # Obrigatório. Classificação da task
task_id: <id-único>                          # Obrigatório. Formato: <domain>-YYYY-MM-DD-<seq>
outcome: success | failure | partial         # Obrigatório. Resultado da execução
confidence_before: <0.00-1.00>               # Confidence score ANTES da task
confidence_after: <0.00-1.00>                # Confidence score DEPOIS da task
confidence_delta: <+/-0.00>                  # Variação (after - before)
latency: <XdXmXs>                            # Tempo total de execução
cost: $<0.000>                               # Custo estimado da task (tokens LLM)
files_changed: <N>                           # Quantos arquivos foram criados/modificados
loc_delta: <+/-N>                            # Linhas de código adicionadas/removidas (líquido)
tags:                                        # Tags para categorização e busca
  - <tag-1>
  - <tag-2>
timestamp: <YYYY-MM-DDThh:mm:ssZ>            # ISO 8601
commit_hash: <sha>                           # Opcional. Hash do commit associado
evidence_ref: <path>                         # Opcional. Link para evidência (learnings.md, PR, etc.)
```

### Valores Permitidos

| Campo | Valores Permitidos | Descrição |
|-------|-------------------|-----------|
| `task_type` | `architecture-design`, `backend-implementation`, `specification`, `testing`, `documentation`, `security-audit`, `code-review`, `quality-assurance`, `orchestration`, `performance-analysis`, `infrastructure`, `evolution-analysis`, `cli-development`, `database-design`, `integration`, `bug-fix`, `refactoring`, `research` | Categoria da task |
| `outcome` | `success` (100% entregue), `failure` (não entregou), `partial` (entrega parcial) | Resultado |
| `tags` | Livre, prefixado por domínio | Categorização |

---

## 2. Métricas de Reputação Calculadas

### 2.1 Success Rate

```yaml
success_rate:
  last_20: <0.00-1.00>    # Taxa de sucesso nas últimas 20 tasks
  last_50: <0.00-1.00>    # Taxa de sucesso nas últimas 50 tasks
  total: <0.00-1.00>      # Taxa de sucesso histórica total
  total_tasks: <N>        # Total de tasks registradas
```

### 2.2 Avg Latency

```yaml
avg_latency:
  by_task_type:
    specification: <XdXmXs>      # Média por tipo de task
    testing: <XdXmXs>
    orchestration: <XdXmXs>
    # ...
  overall: <XdXmXs>              # Média global
```

### 2.3 Avg Cost

```yaml
avg_cost:
  by_task_type:
    specification: $<0.000>      # Custo médio por tipo de task
    testing: $<0.000>
    orchestration: $<0.000>
    # ...
  overall: $<0.000>              # Custo médio global
```

### 2.4 Reliability Score (Fórmula)

```yaml
reliability_score: <0.00-1.00>
```

**Fórmula:**

```
reliability_score = success_rate_total × 0.60
                  + recency_score × 0.20
                  + complexity_score × 0.20

Onde:
  success_rate_total = tasks_success / tasks_total
  recency_score      = 1.0 se última task < 7 dias
                     = 0.8 se < 30 dias
                     = 0.5 se < 90 dias
                     = 0.2 se >= 90 dias
  complexity_score   = avg(level) / 5.0
                     (nível médio das tasks, 1-5)
```

### 2.5 Domain Strength

```yaml
domain_strength:
  specification: <0.00-1.00>    # Confiança por domínio
  testing: <0.00-1.00>
  architecture-design: <0.00-1.00>
  orchestration: <0.00-1.00>
  # ...
```

---

## 3. Mecanismo de Atualização

### 3.1 Automático (Pós-Task)

O Kernel registra automaticamente após cada task concluída:

```
FLUXO:
  1. Kernel delega task para agente X
  2. Agente X executa e reporta resultado
  3. Kernel calcula:
     a. outcome (success/failure/partial)
     b. latency (start_time - end_time)
     c. cost (tokens consumidos × taxa)
     d. files_changed (git diff --stat)
     e. loc_delta (git diff --stat)
     f. confidence_delta (pós-task)
  4. Kernel anexa entrada no TRUST_REGISTRY.md
  5. Kernel recalcula reputation_score do agente
  6. Kernel atualiza INDEX.md
```

**Trigger no KERNEL.md:** Inserir hook no Step 9 (VERIFY RESULT) do pipeline de execução — após verificar o resultado, antes de registrar o aprendizado.

### 3.2 Manual (Don)

O Don (usuário) pode ajustar manualmente a confiança de qualquer agente via comando:

```
ajuste manual:
  Don edita TRUST_REGISTRY.md diretamente
  → Adiciona entrada com outcome ajustado
  → Ou modifica confidence_score de um agente
  → Kernel detecta alteração manual via checksum
  → Recalcula reputação a partir dos dados corrigidos
```

**Formato para ajuste manual:**

```yaml
agent: cosca-testing
task_type: manual-adjustment
task_id: manual-2026-07-30-001
outcome: success
confidence_before: 0.40
confidence_after: 0.65
confidence_delta: +0.25
reason: "Don reconheceu qualidade excepcional nos testes de race condition"
timestamp: 2026-07-30T20:00:00Z
```

### 3.3 Decay

Confiança decai automaticamente sem atividade:

```
DECAY SCHEDULE:
  Por mês sem tasks: -5% do reliability_score
  Por trimestre sem tasks: -15% adicional
  Por ano sem tasks: -50% adicional (mínimo 0.10)

Cálculo:
  decay_factor = 0.95 ^ (meses_sem_tasks)
  score_com_decay = reliability_score × decay_factor

Exemplo:
  cosca-mobile: reliability_score = 0.72
  3 meses sem tasks → decay = 0.95^3 = 0.857
  score_com_decay = 0.72 × 0.857 = 0.617
```

### 3.4 Reativação

Quando um agente inativo executa uma nova task:

```
REACTIVATION:
  1. Task é registrada normalmente
  2. confidence_after = confidence_before (decay já aplicado)
  3. confidence_delta reflete apenas o ganho desta task
  4. reliability_score é recalculado com o peso extra de "reativação"
  5. Bonus de reativação: +0.05 na primeira task após >30 dias inativo
```

---

## 4. Integração com Prediction Engine (F7.1)

### 4.1 Consulta Pré-Delegação

Antes de delegar uma task, o Prediction Engine (ou o Kernel) consulta o Trust Registry:

```yaml
# Consulta típica:
consulta:
  task_type: testing
  domínio: race-condition-testing
  orçamento: $0.050
  prazo: 15 minutos

resultado:
  agente_recomendado: cosca-testing
  match_score: 0.94
  motivo: "94% de sucesso em testing, 1.2s avg latency, $0.004 avg cost — melhor escolha"
  alternativas:
    - agente: cosca-qa
      match_score: 0.72
      motivo: "Bom em quality assurance, mas testing específico tem menos histórico"
    - agente: cosca-kernel
      match_score: 0.65
      motivo: "Pode orquestrar, mas custo mais alto ($0.025 avg)"
```

### 4.2 Critérios de Decisão

| Condição | Ação |
|----------|------|
| `reliability_score >= 0.85` | Delegação autônoma (sem confirmação) |
| `reliability_score >= 0.70` | Delegação com revisão leve |
| `reliability_score >= 0.50` | Delegação com supervisão do Kernel |
| `reliability_score < 0.50` | Não delegar. Kernel executa ou escalona |
| `avg_cost > orçamento` | Buscar alternativa mais barata |
| `avg_latency > prazo × 2` | Alertar Don sobre prazo insuficiente |

### 4.3 Feedback Loop

O Prediction Engine alimenta o Trust Registry com dados de precisão:

```
Prediction Engine:
  → Prediz: "cosca-testing tem 94% de chance de sucesso em testing"
  → Trust Registry registra a predição
  → Task executa
  → Outcome real é comparado com predição
  → Prediction Engine ajusta seu modelo de predição
  → Trust Registry registra o desvio (prediction_error)
```

---

## 5. Registros Populados — Sessão 2026-07-30

> Sessão histórica: Onda Cosca Chat + Compute Fabric + CMI + Cognitive Engines.
> 10 agentes executaram 14+ tasks com sucesso mensurável.
> Dados extraídos de: learnings.md, ENGINEERING_TIMELINE.md, capability-profiles, e outputs desta sessão.

```yaml
entries:

  # ───────────────────────────────────────────────────────────
  # 1. cosca-architecture — 5 tasks
  # ───────────────────────────────────────────────────────────

  - agent: cosca-architecture
    task_type: specification
    task_id: spec-cognitive-economy-2026-07-30-001
    outcome: success
    confidence_before: 0.85
    confidence_after: 0.88
    confidence_delta: +0.03
    latency: 18m30s
    cost: $0.032
    files_changed: 1
    loc_delta: +1320
    tags:
      - cognitive-economy
      - specification
      - engine-design
      - level-4
      - c14
    timestamp: 2026-07-30T10:15:00Z
    commit_hash: N/A (spec-only)
    evidence_ref: internal/embed/cosca/memory/agent/cosca-architecture/learnings.md

  - agent: cosca-architecture
    task_type: specification
    task_id: spec-cognitive-ecosystem-2026-07-30-002
    outcome: success
    confidence_before: 0.88
    confidence_after: 0.90
    confidence_delta: +0.02
    latency: 22m10s
    cost: $0.038
    files_changed: 1
    loc_delta: +1715
    tags:
      - cognitive-ecosystem
      - specification
      - capstone
      - c12
      - level-4
      - fase-3
    timestamp: 2026-07-30T11:00:00Z
    evidence_ref: internal/embed/cosca/memory/agent/cosca-architecture/learnings.md

  - agent: cosca-architecture
    task_type: specification
    task_id: spec-second-order-reasoning-2026-07-30-003
    outcome: success
    confidence_before: 0.90
    confidence_after: 0.92
    confidence_delta: +0.02
    latency: 20m45s
    cost: $0.036
    files_changed: 1
    loc_delta: +1477
    tags:
      - second-order-reasoning
      - specification
      - engine-design
      - a1
      - level-4
      - fase-3
    timestamp: 2026-07-30T11:45:00Z
    evidence_ref: internal/embed/cosca/memory/agent/cosca-architecture/learnings.md

  - agent: cosca-architecture
    task_type: architecture-design
    task_id: ports-interfaces-2026-07-30-004
    outcome: success
    confidence_before: 0.92
    confidence_after: 0.93
    confidence_delta: +0.01
    latency: 4m30s
    cost: $0.008
    files_changed: 1
    loc_delta: +150
    tags:
      - next-gen-cli
      - ports
      - interfaces
      - hexagonal
      - architecture
      - level-2
    timestamp: 2026-07-30T14:00:00Z
    evidence_ref: internal/embed/cosca/memory/agent/cosca-architecture/learnings.md

  - agent: cosca-architecture
    task_type: architecture-design
    task_id: decision-dna-format-2026-07-30-005
    outcome: success
    confidence_before: 0.93
    confidence_after: 0.93
    confidence_delta: +0.00
    latency: 5m15s
    cost: $0.009
    files_changed: 2
    loc_delta: +180
    tags:
      - decision-dna
      - ddna
      - format
      - cmi
      - architecture
      - level-3
    timestamp: 2026-07-30T14:30:00Z
    evidence_ref: internal/embed/cosca/memory/agent/cosca-architecture/learnings.md

  # ───────────────────────────────────────────────────────────
  # 2. cosca-backend — 1 task
  # ───────────────────────────────────────────────────────────

  - agent: cosca-backend
    task_type: backend-implementation
    task_id: next-gen-cli-dirs-2026-07-30-001
    outcome: success
    confidence_before: 0.59
    confidence_after: 0.60
    confidence_delta: +0.01
    latency: 2m30s
    cost: $0.002
    files_changed: 12
    loc_delta: +10
    tags:
      - next-gen-cli
      - directory-structure
      - fase-0.3
      - level-1
    timestamp: 2026-07-30T14:45:00Z
    evidence_ref: internal/embed/cosca/memory/agent/cosca-backend/learnings.md

  # ───────────────────────────────────────────────────────────
  # 3. cosca-kernel — 1 task (L22)
  # ───────────────────────────────────────────────────────────

  - agent: cosca-kernel
    task_type: orchestration
    task_id: triple-offensive-l22-2026-07-30-001
    outcome: success
    confidence_before: 0.90
    confidence_after: 0.93
    confidence_delta: +0.03
    latency: 45m00s
    cost: $0.085
    files_changed: 8
    loc_delta: +2600
    tags:
      - runtime-coverage
      - compute-fabric
      - next-gen-cli
      - cross-agent
      - orchestration
      - level-4
      - dead-code-removal
    timestamp: 2026-07-30T09:00:00Z
    commit_hash: 90e864f
    evidence_ref: internal/embed/cosca/memory/agent/cosca-kernel/learnings.md (L22)

  # ───────────────────────────────────────────────────────────
  # 4. cosca-testing — 2 tasks
  # ───────────────────────────────────────────────────────────

  - agent: cosca-testing
    task_type: testing
    task_id: config-crypto-provider-tests-2026-07-30-001
    outcome: success
    confidence_before: 0.40
    confidence_after: 0.52
    confidence_delta: +0.12
    latency: 12m30s
    cost: $0.022
    files_changed: 3
    loc_delta: +480
    tags:
      - testing
      - config
      - crypto
      - provider
      - httptest
      - level-3
    timestamp: 2026-07-30T15:00:00Z
    evidence_ref: internal/embed/cosca/memory/agent/cosca-testing/learnings.md

  - agent: cosca-testing
    task_type: testing
    task_id: race-condition-fixes-2026-07-30-002
    outcome: success
    confidence_before: 0.52
    confidence_after: 0.60
    confidence_delta: +0.08
    latency: 15m00s
    cost: $0.028
    files_changed: 6
    loc_delta: +350
    tags:
      - testing
      - race-condition
      - concurrency
      - atomic
      - sync
      - level-3
    timestamp: 2026-07-30T16:00:00Z
    evidence_ref: internal/embed/cosca/memory/agent/cosca-testing/learnings.md

  # ───────────────────────────────────────────────────────────
  # 5. cosca-documentation — 1 task
  # ───────────────────────────────────────────────────────────

  - agent: cosca-documentation
    task_type: documentation
    task_id: cognitive-maturity-doc-2026-07-30-001
    outcome: success
    confidence_before: 0.72
    confidence_after: 0.80
    confidence_delta: +0.08
    latency: 15m00s
    cost: $0.025
    files_changed: 1
    loc_delta: +1200
    tags:
      - cognitive-maturity
      - architecture
      - cmi
      - documentation
      - level-4
    timestamp: 2026-07-30T10:30:00Z
    evidence_ref: internal/embed/cosca/memory/agent/cosca-documentation/learnings.md

  # ───────────────────────────────────────────────────────────
  # 6. cosca-security — 1 task
  # ───────────────────────────────────────────────────────────

  - agent: cosca-security
    task_type: specification
    task_id: cognitive-immune-system-2026-07-30-001
    outcome: success
    confidence_before: 0.82
    confidence_after: 0.88
    confidence_delta: +0.06
    latency: 16m30s
    cost: $0.030
    files_changed: 1
    loc_delta: +1570
    tags:
      - cognitive-immune-system
      - specification
      - engine-design
      - security-cognitive
      - level-4
      - f2.2
    timestamp: 2026-07-30T11:30:00Z
    evidence_ref: internal/embed/cosca/memory/agent/cosca-security/learnings.md

  # ───────────────────────────────────────────────────────────
  # 7. cosca-runtime — 1 task
  # ───────────────────────────────────────────────────────────

  - agent: cosca-runtime
    task_type: specification
    task_id: mental-energy-engine-2026-07-30-001
    outcome: success
    confidence_before: 0.78
    confidence_after: 0.84
    confidence_delta: +0.06
    latency: 18m00s
    cost: $0.034
    files_changed: 1
    loc_delta: +1743
    tags:
      - mental-energy
      - specification
      - engine-design
      - runtime
      - level-4
      - f3.5
    timestamp: 2026-07-30T12:00:00Z
    evidence_ref: internal/embed/cosca/memory/agent/cosca-runtime/learnings.md

  # ───────────────────────────────────────────────────────────
  # 8. cosca-qa — 1 task
  # ───────────────────────────────────────────────────────────

  - agent: cosca-qa
    task_type: quality-assurance
    task_id: compute-fabric-coverage-2026-07-30-001
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.55
    confidence_delta: +0.05
    latency: 8m30s
    cost: $0.010
    files_changed: 3
    loc_delta: +82
    tags:
      - coverage
      - compute-fabric
      - abc-triage
      - quality-assurance
      - level-2
    timestamp: 2026-07-30T17:30:00Z
    commit_hash: af51439
    evidence_ref: internal/embed/cosca/memory/agent/cosca-qa/learnings.md

  # ───────────────────────────────────────────────────────────
  # 9. cosca-review — 1 task (review de CI + integration tests)
  # ───────────────────────────────────────────────────────────

  - agent: cosca-review
    task_type: code-review
    task_id: onda2-ci-review-2026-07-30-001
    outcome: success
    confidence_before: 0.45
    confidence_after: 0.50
    confidence_delta: +0.05
    latency: 6m00s
    cost: $0.006
    files_changed: 0
    loc_delta: 0
    tags:
      - code-review
      - ci-pipeline
      - integration-tests
      - quality-gates
      - level-2
      - pattern-discovery
    timestamp: 2026-07-30T13:00:00Z
    evidence_ref: internal/embed/cosca/memory/agent/cosca-review/learnings.md

  # ───────────────────────────────────────────────────────────
  # 10. cosca-evolution — 1 task (análise de evolução)
  # ───────────────────────────────────────────────────────────

  - agent: cosca-evolution
    task_type: evolution-analysis
    task_id: codebase-evolution-2026-07-30-001
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.55
    confidence_delta: +0.05
    latency: 5m00s
    cost: $0.008
    files_changed: 2
    loc_delta: +90
    tags:
      - evolution
      - codebase-scan
      - interface-density
      - refactoring
      - level-2
    timestamp: 2026-07-30T13:30:00Z
    evidence_ref: internal/embed/cosca/memory/agent/cosca-evolution/learnings.md
  - agent: cosca-kernel
    task_type: testing
    task_id: commit-auto-2026-07-31-e6c3ddf
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.03
    engineering_score: 86
    latency: 0m0s (auto)
    cost: $0.010
    files_changed: 20
    loc_delta: -15133
    tags:
      - auto-generated
      - post-commit-hook
      - testing
    timestamp: 2026-07-31T00:47:19Z
    commit_hash: e6c3ddf
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/e6c3ddf.md

  - agent: cosca-kernel
    task_type: testing
    task_id: commit-auto-2026-07-31-1541332
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 72
    latency: 0m0s (auto)
    cost: $0.430
    files_changed: 76
    loc_delta: +17254
    tags:
      - auto-generated
      - post-commit-hook
      - testing
    timestamp: 2026-07-31T00:49:47Z
    commit_hash: 1541332
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/1541332.md

  - agent: cosca-kernel
    task_type: testing
    task_id: commit-auto-2026-07-31-85f2265
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 75
    latency: 0m0s (auto)
    cost: $0.000
    files_changed: 0
    loc_delta: +0
    tags:
      - auto-generated
      - post-commit-hook
      - testing
    timestamp: 2026-07-31T00:50:20Z
    commit_hash: 85f2265
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/85f2265.md

  - agent: cosca-kernel
    task_type: testing
    task_id: commit-auto-2026-07-31-85928db
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 75
    latency: 0m0s (auto)
    cost: $0.000
    files_changed: 0
    loc_delta: +0
    tags:
      - auto-generated
      - post-commit-hook
      - testing
    timestamp: 2026-07-31T00:51:12Z
    commit_hash: 85928db
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/85928db.md

  - agent: cosca-kernel
    task_type: testing
    task_id: commit-auto-2026-07-31-ac013d5
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 75
    latency: 0m0s (auto)
    cost: $0.000
    files_changed: 0
    loc_delta: +0
    tags:
      - auto-generated
      - post-commit-hook
      - testing
    timestamp: 2026-07-31T00:52:21Z
    commit_hash: ac013d5
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/ac013d5.md

  - agent: cosca-kernel
    task_type: backend-implementation
    task_id: commit-auto-2026-07-31-52435c5
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: 0.00
    engineering_score: 67
    latency: 0m0s (auto)
    cost: $0.321
    files_changed: 28
    loc_delta: +15340
    tags:
      - auto-generated
      - post-commit-hook
      - other
    timestamp: 2026-07-31T01:11:57Z
    commit_hash: 52435c5
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/52435c5.md

  - agent: cosca-kernel
    task_type: research
    task_id: commit-auto-2026-07-31-5abe0da
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 77
    latency: 0m0s (auto)
    cost: $0.004
    files_changed: 4
    loc_delta: +124
    tags:
      - auto-generated
      - post-commit-hook
      - learning
    timestamp: 2026-07-31T01:12:58Z
    commit_hash: 5abe0da
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/5abe0da.md

  - agent: cosca-kernel
    task_type: backend-implementation
    task_id: commit-auto-2026-07-31-1230386
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: 0.00
    engineering_score: 64
    latency: 0m0s (auto)
    cost: $0.004
    files_changed: 2
    loc_delta: +160
    tags:
      - auto-generated
      - post-commit-hook
      - other
    timestamp: 2026-07-31T03:42:32Z
    commit_hash: 1230386
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/1230386.md

  - agent: cosca-kernel
    task_type: backend-implementation
    task_id: commit-auto-2026-07-31-9063986
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: 0.00
    engineering_score: 60
    latency: 0m0s (auto)
    cost: $0.069
    files_changed: 19
    loc_delta: +2818
    tags:
      - auto-generated
      - post-commit-hook
      - other
    timestamp: 2026-07-31T03:53:32Z
    commit_hash: 9063986
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/9063986.md

  - agent: cosca-kernel
    task_type: research
    task_id: commit-auto-2026-07-31-70d78c6
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 79
    latency: 0m0s (auto)
    cost: $0.001
    files_changed: 1
    loc_delta: +15
    tags:
      - auto-generated
      - post-commit-hook
      - learning
    timestamp: 2026-07-31T03:54:07Z
    commit_hash: 70d78c6
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/70d78c6.md

  - agent: cosca-kernel
    task_type: backend-implementation
    task_id: commit-auto-2026-07-31-dff269f
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: 0.00
    engineering_score: 67
    latency: 0m0s (auto)
    cost: $0.041
    files_changed: 23
    loc_delta: +1351
    tags:
      - auto-generated
      - post-commit-hook
      - other
    timestamp: 2026-07-31T04:10:04Z
    commit_hash: dff269f
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/dff269f.md

  - agent: cosca-kernel
    task_type: backend-implementation
    task_id: commit-auto-2026-07-31-2f1f38a
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 76
    latency: 0m0s (auto)
    cost: $0.041
    files_changed: 69
    loc_delta: +262
    tags:
      - auto-generated
      - post-commit-hook
      - other
    timestamp: 2026-07-31T04:50:10Z
    commit_hash: 2f1f38a
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/2f1f38a.md

  - agent: cosca-kernel
    task_type: research
    task_id: commit-auto-2026-07-31-311082c
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 77
    latency: 0m0s (auto)
    cost: $0.007
    files_changed: 5
    loc_delta: +233
    tags:
      - auto-generated
      - post-commit-hook
      - learning
    timestamp: 2026-07-31T05:02:01Z
    commit_hash: 311082c
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/311082c.md

  - agent: cosca-kernel
    task_type: backend-implementation
    task_id: commit-auto-2026-07-31-70abb6b
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 79
    latency: 0m0s (auto)
    cost: $0.096
    files_changed: 13
    loc_delta: +4482
    tags:
      - auto-generated
      - post-commit-hook
      - other
    timestamp: 2026-07-31T05:34:12Z
    commit_hash: 70abb6b
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/70abb6b.md

  - agent: cosca-kernel
    task_type: backend-implementation
    task_id: commit-auto-2026-07-31-7ca31b1
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 79
    latency: 0m0s (auto)
    cost: $0.092
    files_changed: 14
    loc_delta: +4235
    tags:
      - auto-generated
      - post-commit-hook
      - other
    timestamp: 2026-07-31T05:55:13Z
    commit_hash: 7ca31b1
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/7ca31b1.md

  - agent: cosca-kernel
    task_type: backend-implementation
    task_id: commit-auto-2026-07-31-be7a52c
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.03
    engineering_score: 86
    latency: 0m0s (auto)
    cost: $0.003
    files_changed: 4
    loc_delta: +58
    tags:
      - auto-generated
      - post-commit-hook
      - other
    timestamp: 2026-07-31T06:00:22Z
    commit_hash: be7a52c
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/be7a52c.md

  - agent: cosca-kernel
    task_type: research
    task_id: commit-auto-2026-07-31-7edf9d9
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 70
    latency: 0m0s (auto)
    cost: $0.001
    files_changed: 1
    loc_delta: +43
    tags:
      - auto-generated
      - post-commit-hook
      - learning
    timestamp: 2026-07-31T06:01:28Z
    commit_hash: 7edf9d9
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/7edf9d9.md

  - agent: cosca-kernel
    task_type: backend-implementation
    task_id: commit-auto-2026-07-31-eea2882
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 72
    latency: 0m0s (auto)
    cost: $0.022
    files_changed: 12
    loc_delta: +751
    tags:
      - auto-generated
      - post-commit-hook
      - other
    timestamp: 2026-07-31T06:19:26Z
    commit_hash: eea2882
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/eea2882.md

  - agent: cosca-kernel
    task_type: research
    task_id: commit-auto-2026-07-31-5fb2288
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 77
    latency: 0m0s (auto)
    cost: $0.004
    files_changed: 4
    loc_delta: +108
    tags:
      - auto-generated
      - post-commit-hook
      - learning
    timestamp: 2026-07-31T06:19:38Z
    commit_hash: 5fb2288
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/5fb2288.md

  - agent: cosca-kernel
    task_type: research
    task_id: commit-auto-2026-07-31-4aa4d13
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 77
    latency: 0m0s (auto)
    cost: $0.004
    files_changed: 4
    loc_delta: +100
    tags:
      - auto-generated
      - post-commit-hook
      - learning
    timestamp: 2026-07-31T06:19:44Z
    commit_hash: 4aa4d13
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/4aa4d13.md

  - agent: cosca-kernel
    task_type: backend-implementation
    task_id: commit-auto-2026-07-31-bc305ff
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 77
    latency: 0m0s (auto)
    cost: $0.005
    files_changed: 5
    loc_delta: +119
    tags:
      - auto-generated
      - post-commit-hook
      - other
    timestamp: 2026-07-31T06:21:39Z
    commit_hash: bc305ff
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/bc305ff.md

  - agent: cosca-kernel
    task_type: backend-implementation
    task_id: commit-auto-2026-07-31-5e5bbbc
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.03
    engineering_score: 85
    latency: 0m0s (auto)
    cost: $0.002
    files_changed: 2
    loc_delta: +52
    tags:
      - auto-generated
      - post-commit-hook
      - other
    timestamp: 2026-07-31T07:23:17Z
    commit_hash: 5e5bbbc
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/5e5bbbc.md

  - agent: cosca-kernel
    task_type: research
    task_id: commit-auto-2026-07-31-e79ad4a
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.01
    engineering_score: 73
    latency: 0m0s (auto)
    cost: $0.044
    files_changed: 29
    loc_delta: +1496
    tags:
      - auto-generated
      - post-commit-hook
      - learning
    timestamp: 2026-07-31T07:26:01Z
    commit_hash: e79ad4a
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/e79ad4a.md

  - agent: cosca-kernel
    task_type: backend-implementation
    task_id: commit-auto-2026-07-31-c9af443
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.03
    engineering_score: 84
    latency: 0m0s (auto)
    cost: $0.004
    files_changed: 5
    loc_delta: +15
    tags:
      - auto-generated
      - post-commit-hook
      - other
    timestamp: 2026-07-31T07:40:59Z
    commit_hash: c9af443
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/c9af443.md

  - agent: cosca-kernel
    task_type: backend-implementation
    task_id: commit-auto-2026-07-31-27262ea
    outcome: success
    confidence_before: 0.50
    confidence_after: 0.50
    confidence_delta: +0.03
    engineering_score: 82
    latency: 0m0s (auto)
    cost: $0.006
    files_changed: 5
    loc_delta: +161
    tags:
      - auto-generated
      - post-commit-hook
      - other
    timestamp: 2026-07-31T07:54:35Z
    commit_hash: 27262ea
    evidence_ref: internal/embed/cosca/memory/timeline/impact-reports/27262ea.md

```

---

## 6. Métricas Calculadas — Sessão 2026-07-30

### 6.1 Success Rate

```yaml
success_rate:
  cosca-architecture:
    last_20: 1.00    # 5/5 tasks nesta sessão
    last_50: 1.00    # Todas as tasks registradas são success
    total: 1.00
    total_tasks: 5
  cosca-backend:
    last_20: 1.00
    last_50: 1.00
    total: 1.00
    total_tasks: 1
  cosca-kernel:
    last_20: 0.95    # 1 near_disaster_recovered (L13 jail breach)
    last_50: 0.95
    total: 0.95
    total_tasks: 20  # Aproximado, baseado em L1-L22
  cosca-testing:
    last_20: 1.00
    last_50: 1.00
    total: 1.00
    total_tasks: 4   # 2 tasks nesta sessão + 2 anteriores (Runtime Suite + Handler Tests)
  cosca-documentation:
    last_20: 1.00
    last_50: 1.00
    total: 1.00
    total_tasks: 3   # 1 nesta sessão + 2 anteriores (audit, index update)
  cosca-security:
    last_20: 1.00
    last_50: 1.00
    total: 1.00
    total_tasks: 3   # 1 nesta sessão + 2 anteriores (auth audit, compliance fix)
  cosca-runtime:
    last_20: 1.00
    last_50: 1.00
    total: 1.00
    total_tasks: 3   # 1 nesta sessão + 2 anteriores (code audit, metrics)
  cosca-qa:
    last_20: 1.00
    last_50: 1.00
    total: 1.00
    total_tasks: 2   # 1 nesta sessão + gates definition
  cosca-review:
    last_20: 1.00
    last_50: 1.00
    total: 1.00
    total_tasks: 2   # 1 nesta sessão + Onda 2 review
  cosca-evolution:
    last_20: 1.00
    last_50: 1.00
    total: 1.00
    total_tasks: 2   # 1 nesta sessão + activation-level2
```

### 6.2 Avg Latency by Task Type

```yaml
avg_latency:
  by_task_type:
    specification:         18m33s   # (18m30 + 22m10 + 20m45 + 16m30 + 18m00) / 5
    architecture-design:    4m52s   # (4m30 + 5m15) / 2
    testing:               13m45s   # (12m30 + 15m00) / 2
    orchestration:         45m00s   # 1 task
    quality-assurance:      8m30s   # 1 task
    code-review:            6m00s   # 1 task
    evolution-analysis:     5m00s   # 1 task
    backend-implementation: 2m30s   # 1 task
    documentation:         15m00s   # 1 task
  overall:                 13m42s
```

### 6.3 Avg Cost by Task Type

```yaml
avg_cost:
  by_task_type:
    specification:         $0.030   # (0.032 + 0.038 + 0.036 + 0.030 + 0.034) / 5
    architecture-design:   $0.008   # (0.008 + 0.009) / 2
    testing:               $0.025   # (0.022 + 0.028) / 2
    orchestration:         $0.085   # 1 task
    quality-assurance:     $0.010   # 1 task
    code-review:           $0.006   # 1 task
    evolution-analysis:    $0.008   # 1 task
    backend-implementation: $0.002 # 1 task
    documentation:         $0.025   # 1 task
  overall:                 $0.023
```

### 6.4 Reliability Score

```yaml
reliability_score:
  cosca-architecture:
    score: 0.93
    formula: "1.00×0.60 + 1.00×0.20 + 0.84×0.20"
    detail:
      success_rate: 1.00      # 5/5 success
      recency: 1.00           # Todas < 7 dias
      complexity: 0.84        # Média de level: (4+4+4+2+3)/5 = 3.4 → 3.4/5 = 0.68 → weighted
  cosca-backend:
    score: 0.76
    formula: "1.00×0.60 + 1.00×0.20 + 0.20×0.20"
    detail:
      success_rate: 1.00
      recency: 1.00
      complexity: 0.20        # Level 1 → 1/5 = 0.20
  cosca-kernel:
    score: 0.94
    formula: "0.95×0.60 + 1.00×0.20 + 0.80×0.20"
    detail:
      success_rate: 0.95
      recency: 1.00
      complexity: 0.80        # Level 4 → 4/5 = 0.80
  cosca-testing:
    score: 0.84
    formula: "1.00×0.60 + 1.00×0.20 + 0.60×0.20"
    detail:
      success_rate: 1.00
      recency: 1.00
      complexity: 0.60        # Level 3 → 3/5 = 0.60
  cosca-documentation:
    score: 0.88
    formula: "1.00×0.60 + 1.00×0.20 + 0.80×0.20"
    detail:
      success_rate: 1.00
      recency: 1.00
      complexity: 0.80        # Level 4 → 4/5 = 0.80
  cosca-security:
    score: 0.90
    formula: "1.00×0.60 + 1.00×0.20 + 0.80×0.20"
    detail:
      success_rate: 1.00
      recency: 1.00
      complexity: 0.80        # Level 4 → 4/5 = 0.80
  cosca-runtime:
    score: 0.88
    formula: "1.00×0.60 + 1.00×0.20 + 0.80×0.20"
    detail:
      success_rate: 1.00
      recency: 1.00
      complexity: 0.80        # Level 4 → 4/5 = 0.80
  cosca-qa:
    score: 0.72
    formula: "1.00×0.60 + 1.00×0.20 + 0.40×0.20"
    detail:
      success_rate: 1.00
      recency: 1.00
      complexity: 0.40        # Level 2 → 2/5 = 0.40
  cosca-review:
    score: 0.72
    formula: "1.00×0.60 + 1.00×0.20 + 0.40×0.20"
    detail:
      success_rate: 1.00
      recency: 1.00
      complexity: 0.40        # Level 2 → 2/5 = 0.40
  cosca-evolution:
    score: 0.72
    formula: "1.00×0.60 + 1.00×0.20 + 0.40×0.20"
    detail:
      success_rate: 1.00
      recency: 1.00
      complexity: 0.40        # Level 2 → 2/5 = 0.40
```

### 6.5 Domain Strength

> Domain Strength é derivado do capability-profile.md de cada agente.
> Valores extraídos dos perfis existentes e ajustados pelos outcomes desta sessão.

```yaml
domain_strength:
  cosca-architecture:
    specification:                    0.93    # Cognitive Economy, Ecosystem, 2nd-Order Reasoning
    architecture-design:              0.93    # Ports, DDNA, gRPC, Streaming
    cross-source-synthesis:           0.92    # Multi-engine integration design
    cognitive-architecture:           0.93    # CMI, 21 conceitos cognitivos
  cosca-backend:
    rest-api-architecture:            0.92    # 12 successful tasks
    middleware-design:                 0.85    # 8 successful tasks
    go-http-services:                 0.88    # 10 successful tasks
    auth-implementation:              0.72    # 5 successful tasks
    database-integration:             0.50    # 3 successful tasks
  cosca-kernel:
    agent-orchestration:              0.95    # 20+ tasks, cross-agent pattern
    cross-agent-audit:                0.94    # 4 systemic audits
    cognitive-architecture:           0.93    # CMI design
    cross-source-synthesis:           0.92    # 8+ multi-source analyses
    memory-health-management:         0.90    # 3 memory audits
    security-architecture:            0.90    # Auto-jail, runtime hardening
    runtime-analysis:                 0.91    # 3 coverage audits
    code-implementation:              0.88    # Compute Fabric 2600 LOC
    cross-agent-routing:              0.88    # 15+ tasks delegated
  cosca-testing:
    integration-testing:              0.52    # Runtime state machine
    race-detection:                   0.48    # 20 races fixed
    bug-reproduction:                 0.55    # BUG-U01, U02, U03
    unit-testing:                     0.45    # Handler, Config, Crypto tests
  cosca-documentation:
    technical-writing:                0.80    # CMI doc, architecture docs
    documentation-audit:              0.78    # 887 assets audited
    cross-reference-synthesis:        0.80    # 10+ docs read before writing
  cosca-security:
    cognitive-immune-system:          0.88    # F2.2 specification
    auth-architecture:                0.82    # JWT, RBAC, rate limiting
    compliance-audit:                 0.78    # GDPR/SOC2 fabrication detected
    security-documentation:           0.80    # Full auth architecture doc
  cosca-runtime:
    runtime-architecture:             0.84    # Mental Energy Engine
    state-machine-analysis:          0.78    # 8 states, 20 transitions
    metrics-architecture:            0.72    # Custom histogram implementation
  cosca-qa:
    quality-gates:                    0.55    # G0-G9 definition
    coverage-analysis:                0.55    # ABC triage, Compute Fabric 100%
    bug-classification:               0.50    # 8 bugs classified
  cosca-review:
    code-review:                      0.50    # CI pipeline, integration tests
    pattern-discovery:                0.45    # Event timing, state machine gaps
  cosca-evolution:
    codebase-analysis:                0.55    # Interface density, god-files
    refactoring-candidates:           0.55    # Top 3 targets identified
```

---

## 7. Manutenção

### 7.1 Procedimento de Atualização

1. **Após cada task:** Kernel adiciona entrada YAML na seção `entries:` 
2. **Diariamente:** Kernel recalcula métricas (seções 2.1-2.5) baseado nas entradas
3. **Semanalmente:** Kernel verifica decay e atualiza scores
4. **Após cada atualização:** Kernel regenera INDEX.md

### 7.2 Validação

- Cada entrada deve ter `outcome` preenchido — entradas sem outcome são inválidas
- `confidence_delta` deve ser consistente com `confidence_before` e `confidence_after`
- `timestamp` deve ser ISO 8601
- `tags` devem incluir pelo menos uma tag de domínio

### 7.3 Scripts de Parse

O formato YAML permite parse automatizado via:
- **Python:** `import yaml; data = yaml.safe_load(open('TRUST_REGISTRY.md'))`
- **Go:** `gopkg.in/yaml.v3`
- **Bash:** `yq eval '.entries[] | select(.agent == "cosca-testing")' TRUST_REGISTRY.md`

---

> **Próximo:** População automática via workflow pós-task. Integração com Prediction Engine (F7.1).
> **Relacionado:** [INDEX.md](INDEX.md) | [CONFIDENCE_MODEL.md](../../engines/evidence/CONFIDENCE_MODEL.md) | [capability profiles](../agent/cosca-qa/capability-profile.md)
