# Metacognition Pipeline — Cognitive Self-Improvement Workflow

> **Version**: 1.1.0 | **Status**: active | **Owner**: Cosca Kernel | **DNA Version**: 3.0.0 | **Created**: 2026-07-28 | **Updated**: 2026-07-30
> 
> **Change 1.1.0**: Stages 7-8 marcados como MANDATORY. Post-task checklist adicionado. Referência ao cognitive-audit-loop como enforcement mechanism.

## Purpose

Every agent task in the Cosca ecosystem now flows through a metacognition pipeline. This is not a simple execute loop — it's a self-reflective cognitive cycle where agents assess their own capabilities, learn from both success and failure, and continuously update their internal model of what they can and cannot do.

---

## Pipeline Stages

```
                            ┌─────────────────────────┐
                            │         TASK             │
                            │  (received from Kernel)  │
                            └────────────┬────────────┘
                                         │
                                         ▼
                            ┌─────────────────────────┐
                            │    1. SELF-ASSESS       │
                            │  • Load capability       │
                            │    profile               │
                            │  • Check confidence      │
                            │    score for domain      │
                            │  • Identify gaps         │
                            │  • Determine if task      │
                            │    is within capability  │
                            └────────────┬────────────┘
                                         │
                            ┌────────────▼────────────┐
                            │    2. RETRIEVE MEMORY   │
                            │  • Search learnings.md   │
                            │    for #tags matching    │
                            │    task domain           │
                            │  • Check failures.md     │
                            │    for known pitfalls    │
                            │  • Load patterns.md      │
                            │    for reusable solutions │
                            │  • Retrieve highest-     │
                            │    level technique       │
                            └────────────┬────────────┘
                                         │
                            ┌────────────▼────────────┐
                            │    3. PLAN STRATEGY     │
                            │  • Select technique      │
                            │    (never regress below  │
                            │    highest mastered)     │
                            │  • Apply failure mode    │
                            │    avoidance from        │
                            │    negative memory       │
                            │  • Choose approach based │
                            │    on preferred          │
                            │    strategies            │
                            │  • If confidence < 0.7:  │
                            │    request review        │
                            └────────────┬────────────┘
                                         │
                            ┌────────────▼────────────┐
                            │    4. EXECUTE           │
                            │  • Perform the task      │
                            │  • Use selected          │
                            │    technique             │
                            │  • Record metrics:       │
                            │    duration, complexity  │
                            └────────────┬────────────┘
                                         │
                            ┌────────────▼────────────┐
                            │    5. VERIFY RESULT     │
                            │  • Run quality gates     │
                            │  • Check against review  │
                            │    criteria              │
                            │  • Validate security     │
                            │    boundaries            │
                            │  • Confirm no regression │
                            └────────────┬────────────┘
                                         │
                            ┌────────────▼────────────┐
                            │    6. CRITIQUE OWN WORK │
                            │  • Compare result vs     │
                            │    expected outcome      │
                            │  • Identify what worked  │
                            │    and what didn't       │
                            │  • Check for known       │
                            │    failure modes         │
                            │  • Be honest about       │
                            │    quality — no ego      │
                            └────────────┬────────────┘
                                         │
                       ┌─────────────────────────┐
                       │  ╔═══════════════════╗  │
                       │  ║ MANDATORY GATE    ║  │
                       │  ║ Task NÃO está     ║  │
                       │  ║ completa sem      ║  │
                       │  ║ stages 7 e 8.     ║  │
                       │  ╚═══════╤═══════════╝  │
                       └──────────┼──────────────┘
                                  │
                     ┌────────────▼────────────┐
                     │  ★ 7. EXTRACT PATTERN   │
                     │     MANDATORY           │
                     │  • Genericize the        │
                     │    approach into a       │
                     │    reusable pattern      │
                     │  • If novel → create     │
                     │    pattern entry         │
                     │  • If failed → create    │
                     │    negative memory       │
                     │    entry                 │
                     │  • Tag semantically      │
                     │    for cross-agent       │
                     │    retrieval             │
                     │  • MAX 30s — não         │
                     │    bloqueia execução     │
                     └────────────┬────────────┘
                                  │
                     ┌────────────▼────────────┐
                     │  ★ 8. UPDATE CAPABILITY │
                     │     MODEL — MANDATORY   │
                     │  • Recalculate           │
                     │    confidence score      │
                     │  • Update strengths/     │
                     │    weaknesses            │
                     │  • Check if level-up     │
                     │    threshold reached     │
                     │  • Update evolution.md   │
                     │  • Propagate lessons     │
                     │    to knowledge engine   │
                     │  • MAX 30s — não         │
                     │    bloqueia execução     │
                     └────────────┬────────────┘
                                  │
                  ┌───────────────▼───────────────┐
                  │   ⚡ COGNITIVE AUDIT LOOP     │
                  │   (post-task enforcement)     │
                  │   • Verify stages 7-8         │
                  │     executed                  │
                  │   • If skipped → flag task    │
                  │     as INCOMPLETE             │
                  │   • Log audit trail           │
                  │   → cognitive-audit-loop.md   │
                  └───────────────┬───────────────┘
                                  │
                                  ▼
                             ┌─────────────────────────┐
                             │      DONE                │
                             │  Agent is now smarter    │
                             │  than before the task    │
                             │  (audit trail verified)  │
                             └─────────────────────────┘
```

---

## Stage Details

### 1. SELF-ASSESS

**Purpose:** Before touching any code, the agent honestly evaluates whether this task is within its current capability.

**Inputs:**
- `capability-profile.md` — strengths, weaknesses, confidence scores per domain
- `evolution.md` — current level and progression history
- `failures.md` — known failure modes relevant to this task domain

**Decision matrix:**
| Confidence | Action |
|-----------|--------|
| ≥ 0.85 | Proceed autonomously |
| 0.70–0.84 | Proceed, flag for post-execution review |
| 0.50–0.69 | Proceed, request second opinion on plan |
| < 0.50 | Escalate — task exceeds current capability |

**Questions the agent must answer:**
- Do I have the right techniques for this task?
- Have I failed at similar tasks before?
- What's my confidence score for this task domain?
- What gaps do I need to fill before execution?

---

### 2. RETRIEVE MEMORY

**Purpose:** Load ALL relevant past experience — positive AND negative — before executing.

**Search order:**
1. `learnings.md` — semantic search by #tags matching task domain
2. `failures.md` — check for known pitfalls with same #tags
3. `patterns.md` — load reusable solution templates
4. Cross-agent memory — search other agents' learnings for #tags in this domain

**Anti-pattern to avoid:** Only searching positive learnings. Negative memory is equally valuable.

---

### 3. PLAN STRATEGY

**Purpose:** Design the execution approach using the best technique available.

**Rules:**
- **Never regress**: If you've mastered Level 3 techniques for this domain, do NOT apply Level 2 or Level 1
- **Avoid known failures**: If failures.md contains a pitfall matching this scenario, actively avoid that approach
- **Prefer proven strategies**: Use preferred strategies from capability profile unless they match a known failure mode
- **Confidence gate**: If confidence < 0.7, the plan MUST be reviewed before execution

---

### 4. EXECUTE

**Purpose:** Perform the task with full instrumentation.

**During execution:**
- Record start time
- Track decision points (why was X chosen over Y?)
- Log any unexpected behavior immediately
- If a failure mode is encountered, abort and go directly to CRITIQUE

---

### 5. VERIFY RESULT

**Purpose:** Objectively validate the output before claiming success.

**Checklist:**
- Quality gates pass? (reference QUALITY_GATES.md)
- Security boundaries intact? (no auth bypass, no injection)
- Backward compatibility preserved? (existing tests still pass)
- No new technical debt introduced?

**If verification fails:** Do NOT proceed to CRITIQUE. Fix the issue, re-execute, re-verify.

---

### 6. CRITIQUE OWN WORK

**Purpose:** Honest self-evaluation. This is where agents actually learn.

**Questions to answer:**
- What did I do well? (technique applied correctly)
- What was suboptimal? (could have done better)
- Was my confidence score accurate? (overconfident or underconfident?)
- Did I encounter any of my known failure modes?
- What surprised me? (unexpected behavior)

**Rule:** Be brutally honest. Ego has no place here. The Kernel trusts agents that can accurately self-critique.

---

### 7. EXTRACT PATTERN ★ MANDATORY

**Purpose:** Convert this experience into reusable knowledge. THIS STAGE CANNOT BE SKIPPED.

**Enforcement:** If this stage is skipped, the task is INCOMPLETE. The cognitive-audit-loop (see `cognitive-audit-loop.md`) will flag the task and escalate to the Kernel.

**If successful:**
- Genericize the approach into a pattern (patterns.md)
- Record the technique in learnings.md with outcome=success
- Tag semantically for cross-agent discovery

**If failed:**
- Record in failures.md with: what was attempted, why it failed, what should be done instead
- Update learnings.md with outcome=failure and the lesson
- Tag with #failure #learned for cross-agent avoidance

**If nothing new was learned:** That IS a finding. Record: "No new pattern extracted — task was routine application of existing technique {name}." This prevents silent skipping.

**Pattern format:**
```markdown
### Pattern: {name}
**Domain**: {security|api|database|frontend|runtime|...}
**Context**: When does this pattern apply?
**Solution**: The reusable approach
**Confidence**: {score based on success rate}
**Times Applied**: N
**Times Succeeded**: N
**Known Pitfalls**: What to watch out for
```

**Timeout:** Maximum 30 seconds. If extraction takes longer, record partial findings and flag for async processing. Never block task completion on extraction delay.

---

### 8. UPDATE CAPABILITY MODEL ★ MANDATORY

**Purpose:** The agent's self-model evolves based on this experience. THIS STAGE CANNOT BE SKIPPED.

**Enforcement:** If this stage is skipped, the task is INCOMPLETE. The cognitive-audit-loop will flag the task and escalate to the Kernel.

**Updates to capability-profile.md:**
- Recalculate confidence score for the task domain
- Add newly discovered strengths
- Add newly discovered weaknesses
- Update preferred strategies if this approach proved better

**Level-up check:**
| Current Level | Tasks at this level needed | With confidence ≥ 0.8 |
|--------------|---------------------------|----------------------|
| 1 → 2 | 5 successful L1 tasks | Required |
| 2 → 3 | 10 successful L2 tasks | Required |
| 3 → 4 | 15 successful L3 tasks | Required |
| 4 → 5 | 20 successful L4 tasks + 1 novel contribution | Required |

**Update evolution.md:**
- Record level change if threshold reached
- Log confidence score delta (+0.05 for success, -0.10 for failure)

**Timeout:** Maximum 30 seconds. If model update takes longer, record partial changes and flag for async reconciliation. Never block task completion on model update delay.

---

## Confidence Scoring Model

### Per-Domain Confidence

Each agent maintains confidence scores per task domain:

```
Confidence = (SuccessfulTasks * 0.6 + LevelFactor * 0.3 + RecencyFactor * 0.1) / MaxScore

Where:
  SuccessfulTasks = count of successes in this domain
  LevelFactor = current_level / 5 (maps 1-5 to 0.2-1.0)
  RecencyFactor = 1.0 if last task in domain was successful, 0.3 if failed, 0.0 if no recent tasks
```

### Confidence Adjustment Rules

| Event | Adjustment |
|-------|-----------|
| Successful task | +0.05 (max 1.0) |
| Failed task | -0.10 |
| Novel technique discovered | +0.08 |
| Repeated same failure mode | -0.15 (compounding penalty) |
| Pattern contributed to framework | +0.10 |
| Regression (broke existing functionality) | -0.20 |

---

## ★ MANDATORY Post-Task Checklist

> **REGRA**: Nenhuma task está completa até que TODAS as 5 perguntas abaixo sejam respondidas.  
> **ENFORCEMENT**: O cognitive-audit-loop (`cognitive-audit-loop.md`) verifica cada resposta.  
> **SANÇÃO**: Task marcada como INCOMPLETE se qualquer pergunta ficar sem resposta.  
> **TIMEOUT**: 30 segundos por pergunta. Timeout não é desculpa — é falha de extração.

Após CADA task, o agente DEVE responder:

### Q1: "O que eu aprendi que eu não sabia antes?"
→ **Extrair para `learnings.md`** (agente ou cross-agent se aplicável)
- Técnica aplicada, nível, outcome, tags semânticas, próximo passo
- Se nada foi aprendido: registrar "Task foi aplicação rotineira de técnica já dominada: {nome}"
- Formato: Learning Entry Format (ver `LEARNING_PROTOCOL.md`)

### Q2: "Descobri um padrão reutilizável?"
→ **Extrair para `patterns.md`**
- Se sim: nome do padrão, domínio, contexto, solução, confiança, pitfalls
- Se não: registrar "Nenhum padrão novo — abordagem já coberta pelo padrão existente: {nome}"
- Se falhou: registrar em `failures.md` com o que foi tentado, por que falhou, o que fazer diferente

### Q3: "Algo falhou?"
→ **Extrair para `failures.md`**
- Se sim: o que foi tentado, causa raiz, consequência, lição, padrão de evitação
- Se não: registrar "Task concluída sem falhas — sem novo negative memory"
- Verificar se a falha corresponde a um failure mode já conhecido (atualizar contagem se sim)

### Q4: "Minha confiança / habilidades mudaram?"
→ **Atualizar `capability-profile.md`**
- Recalcular confidence score para o domínio da task
- Atualizar strengths e weaknesses
- Registrar delta de confiança (+0.05 sucesso, -0.10 falha, etc.)
- Se confiança não mudou: registrar "Confidence estável para o domínio {nome}"

### Q5: "Alcancei um threshold de level-up?"
→ **Atualizar `evolution.md`**
- Verificar thresholds: L1→L2 (5 tasks), L2→L3 (10 tasks), L3→L4 (15 tasks), L4→L5 (20 tasks + 1 novel)
- Se houve level-up: registrar data, nível alcançado, tarefas acumuladas, próxima meta
- Se não houve: registrar contagem atual: "X/Y tasks completadas para próximo nível"

### Audit Trail

Cada execução do checklist gera um registro de auditoria:

```yaml
audit_entry:
  task_id: "{task identifier}"
  agent: "{agent-name}"
  timestamp: "{ISO 8601}"
  q1_learnings_extracted: true/false
  q2_pattern_extracted: true/false
  q3_failure_recorded: true/false
  q4_capability_updated: true/false
  q5_evolution_checked: true/false
  stages_7_8_complete: true/false
  task_completion_status: complete|incomplete
  timeout_triggered: true/false
```

---

## Integration Points

| System | How It Integrates |
|--------|------------------|
| **Knowledge Engine** | Patterns indexed via FTS5 + vector for cross-agent search |
| **Memory Engine** | learnings.md, failures.md, patterns.md all auto-indexed |
| **Evolution Engine** | Monitors confidence scores, triggers level-up reviews |
| **Quality Gates** | Stage 5 VERIFY enforces G0-G9 gates |
| **Review Engine** | Triggered when confidence < 0.7 or on level-up |
| **Kernel** | Receives capability model updates, adjusts task routing |
| **★ Cognitive Audit Loop** | Post-task enforcement — verifies stages 7-8 completed; see `cognitive-audit-loop.md` |
| **★ Post-Task Checklist** | 5 mandatory questions answered after EVERY task; see § MANDATORY Post-Task Checklist |

---

## Example: Backend Agent Through the Pipeline

```
TASK: "Optimize API performance for knowledge search endpoint"

1. SELF-ASSESS
   → Load capability profile: API Architecture (0.92), Performance (0.45)
   → Performance confidence 0.45 < 0.50 → ESCALATE
   → Kernel: "cosca-backend requests cosca-performance collaboration"
   
2. RETRIEVE MEMORY
   → Search learnings.md for #api #performance
   → Check failures.md: "aggressive-caching-2026-07-15" — DON'T cache auth endpoints!
   → Load patterns.md: "query-optimization-indexing"
   
3. PLAN STRATEGY
   → Technique: Level 2 — Add database indexes + FTS5 query optimization
   → Avoid: aggressive caching layer (known failure)
   → Collaborate: cosca-performance to run EXPLAIN QUERY PLAN
   
4. EXECUTE
   → Add composite index on (type, created_at)
   → Optimize FTS5 query with content= option
   → Run benchmarks: 340ms → 12ms (28x improvement)
   
5. VERIFY
   → Quality gates: G1 (build), G2 (lint), G3 (tests) ✓
   → Security: no auth bypass introduced ✓
   → Backward compatibility: all existing tests pass ✓
   
6. CRITIQUE
   → Good: collaboration with cosca-performance prevented over-engineering
   → Suboptimal: didn't check for existing indexes first (one was redundant)
   → Confidence accuracy: was right to escalate (0.45 performance confidence)
   
7. EXTRACT PATTERN
   → New pattern: "sqlite-query-optimization" — EXPLAIN before indexing
   → Learning entry: Level 2 technique for performance optimization
   → Did NOT trigger known failure mode "aggressive-caching"
   
8. UPDATE CAPABILITY MODEL
   → Performance confidence: 0.45 → 0.50 (+0.05 success)
   → Backend confidence: 0.92 → 0.92 (stable, domain well-mastered)
   → Strengths: added "SQLite query optimization"
```

---

> **Related**: [AGENT_DNA.md](../AGENT_DNA.md) | [LEARNING_PROTOCOL.md](../memory/LEARNING_PROTOCOL.md) | [QUALITY_GATES.md](../QUALITY_GATES.md) | [cognitive-audit-loop.md](cognitive-audit-loop.md) | [AUTO_EVOLUTION_PROTOCOL.md](../shared/AUTO_EVOLUTION_PROTOCOL.md)

---

## ★ Enforcement Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                    HOW STAGES 7-8 ARE ENFORCED                       │
│                                                                      │
│  1. DESIGN-TIME: metacognition-pipeline.md marca stages 7-8 como    │
│     MANDATORY com gate explícito no diagrama e ★ nas seções.        │
│                                                                      │
│  2. RUN-TIME: cognitive-audit-loop.md executa após cada task e      │
│     verifica se as 5 perguntas do checklist foram respondidas.       │
│                                                                      │
│  3. PROTOCOL-LEVEL: AUTO_EVOLUTION_PROTOCOL.md define que stages     │
│     7-8 são NON-NEGOTIABLE e tasks sem eles são INCOMPLETE.         │
│                                                                      │
│  4. KERNEL: Recebe flag de task incompleta e rejeita conclusão      │
│     até que o audit trail esteja completo.                           │
│                                                                      │
│  5. TIMEOUT: 30s máximo para stages 7-8. Se exceder, registra       │
│     partial e agenda async reconciliation (nunca bloqueia execução). │
│                                                                      │
│  Esta arquitetura de 3 camadas (design + runtime + protocolo)        │
│  elimina o risco de drift de documentação que causou as falhas       │
│  F002 documentadas em failures.md.                                   │
└──────────────────────────────────────────────────────────────────────┘
```
