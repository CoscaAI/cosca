# LEARNING PROTOCOL — Auto-Evolution Memory System

> **Version**: 3.1.0 | **Status**: active | **Owner**: Cosca Memory Chief | **Last Updated**: 2026-07-30

## Purpose
Every agent in the Cosca ecosystem auto-evolves. This protocol defines how agents learn from experience, store knowledge semantically, learn from failures, track confidence, and apply increasingly advanced techniques over time. v2.0.0 added **Negative Memory** and **Confidence Scoring**. v3.0.0 adds **Wisdom Decay** — knowledge aging, confidence decay over time, and revalidation triggers. v3.1.0 adds **Decision DNA** — structured, queryable records of every non-trivial decision (CMI Fase 1, Bloco 2).

## The Metacognition Loop

```
TASK
 ↓
SELF-ASSESS (load capability profile, check confidence)
 ↓
RETRIEVE MEMORY (search learnings + failures + patterns)
 ↓
PLAN STRATEGY (select technique, avoid known pitfalls)
 ↓
EXECUTE (with full instrumentation)
 ↓
VERIFY RESULT (quality gates, security, regression)
 ↓
CRITIQUE OWN WORK (honest self-evaluation)
 ↓
EXTRACT PATTERN (success → pattern, failure → negative memory)
 ↓
UPDATE CAPABILITY MODEL (recalc confidence, check level-up)
```

Full pipeline specification: [workflows/metacognition-pipeline.md](../workflows/metacognition-pipeline.md)

---

## Memory Structure Per Agent

Each agent has its own memory directory: `internal/embed/cosca/memory/agent/{agent-name}/`

| File | Purpose |
|------|---------|
| `learnings.md` | Semantic learning journal — each entry is a discrete technique or discovery |
| `failures.md` | **NEW v2.0**: Negative memory — catalog of failed approaches and root causes |
| `patterns.md` | Reusable solution patterns discovered by this agent |
| `evolution.md` | Agent capability evolution timeline — tracks level + confidence progression |
| `capability-profile.md` | **NEW v2.0**: Self-model with strengths, weaknesses, confidence scores, evolution goal |
| `INDEX.md` | Cross-reference index of all learnings (for fast retrieval) |

## Learning Entry Format

Todo aprendizado é registrado em **três camadas** (P15 — MEMÓRIA ESTRUTURADA EM GATILHOS). O conteúdo completo vive no **block assinado**; o `learnings.md` guarda só o **gatilho**.

**1. BLOCK (conteúdo completo, imutável)** — `memory/agent/{agente}/blocks/<sha256>.md`:

```markdown
PREV: <hash do block anterior>
ID: LXXX
TIME: YYYY-MM-DD
LEVEL: 1-5
TAGS: #tag1 #tag2
---
## LXXX — YYYY-MM-DD — {título} | Level N

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | What was being done (context) |
| **Technique** | The specific technique applied |
| **Level** | 1-5 (1=basic, 5=expert) |
| **Outcome** | success / partial / failure |
| **Tags** | #tag1 #tag2 |
| **Related** | referências |
| **Learned** | What was discovered or confirmed |
| **Next** | What to try next time |
| **Wisdom Decay Category** | CRITICAL / STABLE / EXPERIMENTAL / DEPRECATED |
| **Last Validated** | YYYY-MM-DD |
| **Confidence** | 0.00–1.00 |
| **Expires At** | YYYY-MM-DD |
```

O **nome do arquivo** é o `sha256` do **CONTEÚDO COMPLETO do arquivo** (header
`PREV/ID/TIME/LEVEL/TAGS` + `---` + título + tabela), não apenas "título + tabela".
O `PREV` encadeia ao block anterior (ver `MEMORY_ACCESS_PROTOCOL.md` §3 e §5).

**2. GATILHO (índice, 1 linha)** — no `learnings.md`:

```markdown
## LXXX | YYYY-MM-DD | {título curto} | L{nível} | #tags | {sha256[:16]}
```

**3. CHAIN (integridade)** — após o commit: `cosca-check --sign-auto`.

**Regra inegociável:** o conteúdo NÃO aparece no índice — só no block. O índice guarda apenas o gatilho + o hash. Duplicar conteúdo entre índice e block viola a P15.

### Technique Evolution

- **Level 1**: Basic patterns (standard checks, common vulnerabilities)
- **Level 2**: Intermediate (multi-layered checks, tool integration)
- **Level 3**: Advanced (threat modeling, attack chain analysis)
- **Level 4**: Expert (zero-day patterns, novel attack vectors, research-level)
- **Level 5**: Master (contributing new techniques back to the framework)
```

### Wisdom Decay Integration

Knowledge ages. Confidence decreases over time unless revalidated. See [WISDOM_DECAY.md](WISDOM_DECAY.md) for the full specification.

**Quick reference:**
- `wisdom_decay_category`: CRITICAL (security, constitutional → ×0.3), STABLE (proven patterns → ×1.0), EXPERIMENTAL (hypotheses → ×2.0), DEPRECATED (archived → ×0)
- `last_validated`: Updated on creation and every revalidation. Drives the decay curve.
- `confidence`: Auto-calculated: `max(0.10, 1.0 - (days_since_last_validated × category_multiplier / 365) × 0.80)`
- `expires_at`: When confidence is projected to reach 0.20

**Before applying any learning with confidence < 0.70, revalidate it first.**

### Semantic Retrieval

Before starting any task, the agent MUST:
1. Search `learnings.md` for tags matching the current task domain
2. Load the highest-level techniques matching the task
3. Apply the best known approach (not repeating basic checks when advanced ones exist)

### Cross-Agent Learning

Learnings are indexed globally via the knowledge engine (FTS5 full-text search + vector embeddings). When agent A discovers a pattern, agent B can find it via semantic search.

### Evolution Tracking

`evolution.md` records capability milestones with confidence scores:
```markdown
## Evolution Timeline

| Date | Level | Confidence | Capability | Trigger |
|------|-------|-----------|------------|---------|
| 2026-07-27 | 2 | 0.78 | XSS detection via CSP header analysis | Security audit finding #47 |
| 2026-08-01 | 3 | 0.85 | Automated threat modeling with STRIDE | Architecture review feedback |
```

---

## Negative Memory Format (NEW v2.0)

### Purpose
Failures are the most valuable learning resource. Every approach that failed must be recorded so the agent — and all other agents — can avoid repeating the same mistake.

### Entry Format

```markdown
### {timestamp} — {failure-name}

| Field | Value |
|-------|-------|
| **Agent** | cosca-{name} |
| **Task** | What was being attempted (context) |
| **Failed Approach** | The specific approach that did NOT work |
| **Root Cause** | Why it failed (technical reason, not blame) |
| **Consequence** | What broke, what was impacted |
| **Lesson** | What should be done instead |
| **Confidence Impact** | How much confidence dropped (-0.10, -0.15, -0.20) |
| **Tags** | #failure #learned #{domain} #{failure-type} |
| **Related Success** | Link to the learnings.md entry that eventually solved this |
| **Avoidance Pattern** | How to recognize this situation and avoid the same failure |
```

### Cross-Agent Avoidance

Failures are indexed globally with `#failure #learned` tags. Before executing in a domain, agents MUST:
1. Search failures.md across ALL agents for `#tags` matching the task
2. Check if any known failure mode matches the current approach
3. If a match is found, explicitly document why the approach differs or abort

### Failure → Pattern Conversion

When a failure leads to a successful alternative approach, the cycle is:
```
FAILURE (failures.md) → LEARNING (learnings.md) → PATTERN (patterns.md)
```

---

## Confidence Scoring Model (NEW v2.0)

### Purpose
Numerical self-assessment of agent reliability. Drives the SELF-ASSESS stage — agents must know when they're out of their depth.

### Calculation

```
Per-Domain Confidence = (SuccessCount × 0.6 + LevelFactor × 0.3 + RecencyFactor × 0.1) / MaxScore

Where:
  SuccessCount  = min(successes in domain, 20) / 20
  LevelFactor   = current_level / 5  (maps 1-5 to 0.2-1.0)
  RecencyFactor = 1.0 (last task succeeded), 0.3 (last task failed), 0.0 (no recent tasks)
```

### Confidence Adjustment Rules

| Event | Delta | Cap |
|-------|-------|-----|
| Successful task | +0.05 | max 1.00 |
| Failed task | -0.10 | min 0.10 |
| Novel technique discovered | +0.08 | max 1.00 |
| Repeated same failure mode | -0.15 | compounding per repeat |
| Pattern contributed to framework | +0.10 | max 1.00 |
| Regression (broke existing) | -0.20 | min 0.10 |
| First success after failure | +0.08 | recovery bonus |
| Collaboration success | +0.03 | team bonus |

### Decision Thresholds

| Confidence | Action |
|-----------|--------|
| ≥ 0.85 | Proceed autonomously |
| 0.70–0.84 | Proceed, flag for post-execution review |
| 0.50–0.69 | Proceed, request second opinion on plan |
| < 0.50 | Escalate — task exceeds current capability |

### Level-Up Thresholds

| From | To | Successful Tasks Required | Min Confidence |
|------|----|--------------------------|----------------|
| L1 | L2 | 5 at L1 | ≥ 0.80 |
| L2 | L3 | 10 at L2 | ≥ 0.80 |
| L3 | L4 | 15 at L3 + 1 novel contribution | ≥ 0.85 |
| L4 | L5 | 20 at L4 + 3 novel contributions | ≥ 0.90 |

---

## Capability Profile Format (NEW v2.0)

Stored in `capability-profile.md`:

```markdown
# {agent-name} — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: YYYY-MM-DD

## Current Level: N

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| {domain} | 0.XX | N | success/failure | ↑ ↓ → |

## Strengths
- [Capability the agent excels at — specific, not vague]
- [Capability the agent excels at]

## Weaknesses
- [Known gap — honest admission]
- [Known gap]

## Preferred Strategies
- [Go-to approach for common scenarios]
- [Go-to approach for common scenarios]

## Known Failure Modes
- [Pattern: what goes wrong, why, how to detect]
- [Pattern: what goes wrong, why, how to detect]

## Evolution Goal
Reach Level {N+1}:
"{What capability would this unlock?}"
```

---

## Decision DNA Format (NEW v3.1.0 — CMI Fase 1 / Bloco 2)

### Purpose

Nem toda experiência é um aprendizado. Algumas são **decisões** — escolhas entre alternativas com riscos, evidências e trade-offs. O **Decision DNA** captura o ciclo completo de uma decisão não-trivial em formato estruturado e queryable, permitindo que qualquer agente — ou o Don — possa responder, meses depois:

> *"Por que decidimos X em Janeiro de 2026?"*
> *"Quais decisões dos últimos 6 meses foram revertidas?"*
> *"Quais decisões tinham confidence < 0.70 e mesmo assim foram tomadas?"*

### DNA vs Learning: Quando Usar Cada Um

| Critério | Learning Entry (`###`) | Decision DNA (`## DNA`) |
|----------|------------------------|--------------------------|
| **Natureza** | Descoberta, técnica, habilidade adquirida | Escolha entre alternativas com trade-offs |
| **Pergunta respondida** | "O que eu aprendi?" | "Por que escolhi X em vez de Y?" |
| **Reversível?** | Não (aprendizado é cumulativo) | Sim (decisões podem ser revertidas) |
| **Estrutura** | Leve (~15 campos) | Completa (~25 campos com evidências, riscos, alternativas) |
| **Prefixo no arquivo** | `### {date} — {name}` | `## DNA — {id}: {summary}` |

**Regra de ouro**: Se a tarefa envolveu ESCOLHER entre duas ou mais alternativas com impacto cross-domain, é Decision DNA. Se envolveu DESCOBRIR ou APLICAR uma técnica, é Learning Entry.

### Entry Format

```markdown
## DNA — DDNA-{YYYY-MM-DD}-{NNN}: {resumo-da-decisão}

> **DNA ID**: DDNA-YYYY-MM-DD-NNN
> **Status**: active | revisado | revertido | obsoleto
> **Versão do registro**: 1.0.0

### Decisão
{O que foi decidido — uma frase, concisa e assertiva.}

### Contexto
{Por que esta decisão era necessária. O problema ANTES da decisão.}

### Metadados

| Campo | Valor |
|-------|-------|
| **Data da decisão** | YYYY-MM-DD |
| **Agente decisor** | cosca-{nome} |
| **Domínio** | security, architecture, testing, devops, performance, ... |
| **Confiança na decisão** | 0.XX |
| **Nível da decisão** | 1=tático, 2=design local, 3=arquitetura, 4=estratégico, 5=fundacional |
| **CMI impact** | Aprendizado: ±X, Julgamento: ±X, Planejamento: ±X, Autocrítica: ±X, Transferência: ±X, Consistência: ±X |

### Evidências

#### A favor
| # | Evidência | Fonte | Peso (1-5) |
|---|-----------|-------|------------|
| 1 | {Descrição da evidência} | {código, benchmark, doc, auditoria, experimento} | 5 |

#### Contra
| # | Evidência | Fonte | Peso (1-5) |
|---|-----------|-------|------------|
| 1 | {Evidência contrária} | {fonte} | 3 |

### Riscos

| # | Risco | P | I | Severidade | Mitigação |
|---|-------|---|---|------------|-----------|
| 1 | {O que pode dar errado} | 0.X | 0.X | P×I | {Como mitigamos} |

### Alternativas Consideradas

| # | Alternativa | Prós | Contras | Por que rejeitada |
|---|-------------|------|---------|-------------------|
| 1 | {Alternativa A} | {Vantagens} | {Desvantagens} | {Razão específica} |

### Gatilhos de Reconsideração
- [ ] Se {condição X} acontecer, reavaliar esta decisão
- [ ] Revisão programada: {data}

### Resultado

| Campo | Valor |
|-------|-------|
| **Resultado observado** | success / partial / failure / mixed / pendente |
| **Data da validação** | YYYY-MM-DD |
| **Validador** | {agente ou Don} |
| **Evidência do resultado** | {O que prova o resultado} |

### Lições Aprendidas
- {Lição 1}
- {Lição 2}

### Tags
`#dna` `#{domain}` `#{tag-1}` `#{tag-2}`

### Relacionado
- **Learnings**: [{agent}/learnings.md#L{num}](agent/{agent}/learnings.md)
- **Patterns**: [{agent}/patterns.md](agent/{agent}/patterns.md)
- **Failures**: [{agent}/failures.md](agent/{agent}/failures.md)
- **Heurísticas**: [{path}](../knowledge/heuristics/H-{num}.yaml)
```

### Obrigatoriedade

Uma decisão DEVE ser registrada como DNA quando atende a PELO MENOS UM destes critérios:

1. **Confiança < 0.95**: Há incerteza real na decisão
2. **Impacto cross-domain**: Afeta 2+ domínios do CMI
3. **Irreversível ou custosa de reverter**: Mudar depois é caro
4. **Nível ≥ 3**: Decisão de arquitetura, estratégica ou fundacional
5. **Envolve trade-off explícito**: Duas ou mais alternativas razoáveis competindo
6. **O Don pediu**: Se o Don perguntar "por que fizemos isso?", a resposta deve estar no DNA

### Princípios do DNA

| # | Princípio | Descrição |
|---|-----------|-----------|
| **P1** | **Rastreabilidade total** | Toda decisão não-trivial DEVE ter um registro DNA |
| **P2** | **Imutabilidade histórica** | O registro original NUNCA é alterado — apenas campos de resultado/lições são atualizáveis |
| **P3** | **Honestidade radical** | Riscos e evidências contrárias DEVEM ser registrados com o mesmo rigor das evidências favoráveis |
| **P4** | **Queryabilidade** | Todo campo é indexável por tag, domínio, data, agente, outcome, confidence |
| **P5** | **Compatibilidade** | Decisões DNA coexistem com learnings no mesmo arquivo — diferenciadas pelo prefixo `## DNA` |

### Coexistência com Learnings

Decisões DNA podem ser armazenadas de duas formas:

1. **Embedded em `learnings.md`** (recomendado): prefixadas com `## DNA`, coexistem com learnings (`###`) no mesmo arquivo
2. **Arquivo separado** em `internal/embed/cosca/memory/decisions/`: para decisões cross-agent (3+ agentes) ou fundacionais

A distinção por prefixo (`## DNA` vs `###`) permite grep independente:

```bash
# Apenas decisões DNA
rg "^## DNA" agent/cosca-kernel/learnings.md

# Apenas learnings operacionais
rg "^### 2026" agent/cosca-kernel/learnings.md
```

### Ciclo de Vida

```
GATILHO → DECISÃO → REGISTRO DNA → VALIDAÇÃO → CONFIRMA ou REAVALIA
                                                    │              │
                                                    ▼              ▼
                                              Status: revisado  Status: revertido
                                              Lições positivas  Lições corretivas
```

O registro original é IMUTÁVEL. Apenas `Status`, `Resultado`, `Lições` e `Gatilhos` são atualizáveis.

### Full Specification

Para a especificação completa — incluindo regras de validação de campos, exemplos de query, integração FTS5/vector search, e o exemplo canônico da decisão auto-jail com memfd_create — consulte:

| Documento | Conteúdo |
|-----------|----------|
| [DECISION_DNA_FORMAT.md](DECISION_DNA_FORMAT.md) | Especificação canônica completa (v1.0.0) |
| [DECISION_DNA_EXAMPLE.md](DECISION_DNA_EXAMPLE.md) | Exemplo preenchido com decisão real (DDNA-2026-07-29-001) |
| [../architecture/COGNITIVE_MATURITY.md](../architecture/COGNITIVE_MATURITY.md) | Arquitetura CMI — Conceito C4 (Decision DNA) |
| [../workflows/cognitive-maturity-implementation.md](../workflows/cognitive-maturity-implementation.md) | Workflow F1.1 — Implementação do Decision DNA |

