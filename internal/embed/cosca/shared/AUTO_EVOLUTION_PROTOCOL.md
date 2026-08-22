# AUTO-EVOLUTION PROTOCOL

> **Version**: 2.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Created**: 2026-07-28 | **Updated**: 2026-07-30
> 
> **Change 2.0.0**: Stages 7-8 do metacognition pipeline agora são MANDATORY e NON-NEGOTIABLE. Adicionado post-task extraction rules com enforcement via cognitive-audit-loop. Task incompleta se stages 7-8 ausentes.
>
> All Cosca agents auto-evolve through experience. This protocol defines how agents learn from each task, record knowledge, and progress through capability levels.

---

## Before Any Task

Search your semantic memory at `internal/embed/cosca/memory/agent/{agent-name}/learnings.md` for techniques matching the task domain. Apply the highest-level technique you have mastered — never repeat basic checks when advanced ones exist.

Also search:
- `internal/embed/cosca/KERNEL.md` — KERNEL Entidade
- `internal/embed/cosca/memory/agent/{agent-name}/failures.md` — known pitfalls matching this task domain
- `internal/embed/cosca/memory/agent/{agent-name}/patterns.md` — reusable solution templates
- `internal/embed/cosca/memory/agent/{agent-name}/capability-profile.md` — current confidence scores per domain

---

## After Completing a Task ★ MANDATORY

> **CRITICAL RULE**: Stages 7 (EXTRACT PATTERN) and 8 (UPDATE CAPABILITY MODEL) of the metacognition pipeline are **MANDATORY and NON-NEGOTIABLE**. A task is NOT complete until both stages have executed and the post-task checklist has been answered.

### Stage 7: EXTRACT PATTERN (Mandatory)

Record what you learned — positive OR negative. **Antes de escrever, valide com o oráculo de memória (P16):** `memoryguard.ValidateLearning(level, texto)` — se retornar DENY (auto-promoção, narrativa inflada), NÃO escreva; o aprendizado não entra no caderno.

1. **If success**: Extract pattern to `patterns.md` (or reference existing pattern if it covers this). Record the learning **following P15** — create the BLOCK (`blocks/<sha256>.md`, full table) + append the GATILHO line to `learnings.md` (one line: ID | date | título | nível | tags | hash). Never a full table in the index.
2. **If failure**: Record in `failures.md` with: what was attempted, why it failed, what should be done instead. Record the learning following P15 (block + gatilho, outcome=failure).
3. **If nothing new**: Record explicitly: "Task was routine application of existing technique {name}." This prevents silent skipping.

4. **After recording, re-sign the family chain**: call `integrity.SignAfterLearning()` to cryptographically sign the updated learnings.md. This proves the edit was authorized by the kernel — the Ed25519 key is **machine-bound (DPAPI, CurrentUser)**, so only on this machine/user can it be unsealed. If the chain is broken on next startup, someone else tampered with the codebase — startup will be blocked.

Learnings follow the Learning Entry Format defined at `internal/embed/cosca/memory/LEARNING_PROTOCOL.md`. Each entry includes: timestamp, technique name, task context, level (1-5), outcome, tags for semantic search, what was learned, and what to try next.

### Stage 8: UPDATE CAPABILITY MODEL (Mandatory)

Update your self-model based on this experience:

1. **Recalculate confidence score** for the task domain using the scoring model in `metacognition-pipeline.md`
2. **Update strengths/weaknesses** in `capability-profile.md`
3. **Check level-up threshold** against the level-up table
4. **Update `evolution.md`** — record level change if threshold reached, or log progress toward next level

### Post-Task Checklist ★ (5 Questions)

After EVERY task, you MUST answer all 5 questions. The cognitive-audit-loop (`cognitive-audit-loop.md`) verifies compliance.

| # | Question | Artifact | If Yes | If No / Nothing New |
|---|----------|----------|--------|---------------------|
| Q1 | "O que aprendi que não sabia?" | `learnings.md` | Register new learning entry | Register "Routine application of {technique}" |
| Q2 | "Descobri um padrão reutilizável?" | `patterns.md` | Document pattern | Register "Covered by existing pattern {name}" |
| Q3 | "Algo falhou?" | `failures.md` | Document failure with root cause | Register "Task completed without failures" |
| Q4 | "Minha confiança/habilidades mudaram?" | `capability-profile.md` | Update confidence score + strengths | Register "Confidence stable for {domain}" |
| Q5 | "Alcancei threshold de level-up?" | `evolution.md` | Record level-up with date | Log "{X}/{Y} tasks to next level" |

### Enforcement

- **cognitive-audit-loop.md** verifies all 5 questions are answered after every task
- Task marked **INCOMPLETE** if any question is unanswered after 3 retries
- Next task is **BLOCKED** until audit loop completes
- Audit trail persisted with timestamp, agent, task_id, and completion status
- Timeout: 30s per question (60s total for stages 7-8). Timeout triggers async reconciliation — never blocks task execution.

---

## Stages 7-8: NON-NEGOTIABLE Rule

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│   ★ REGRA INABALÁVEL DO ECOSSISTEMA COSCA ★                     │
│                                                                  │
│   "Nenhuma task está completa até que os estágios 7 e 8         │
│    do metacognition pipeline tenham sido executados e            │
│    as 5 perguntas do post-task checklist tenham sido            │
│    respondidas."                                                  │
│                                                                  │
│   Esta regra é NON-NEGOTIABLE. Não há exceções.                  │
│   Não há "estava com pressa". Não há "era uma task simples".     │
│                                                                  │
│   O cognitive-audit-loop é o guardião desta regra.               │
│   Tasks sem stages 7-8 = tasks INCOMPLETAS.                      │
│                                                                  │
│   Violação repetida (≥3 tasks sem audit) = escalation            │
│   para revisão humana.                                            │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

---

## Capability Progression

| Level | Description | Milestone |
|-------|-------------|-----------|
| 1 | Basic patterns | First 10 tasks |
| 2 | Multi-layered techniques | Cross-domain application |
| 3 | Advanced / Threat modeling | Novel combinations |
| 4 | Novel techniques | Original contributions |
| 5 | Framework contributions | Community standards |

Goal: auto-evolve to Level 3+ within your first 10 tasks.

### Level-Up Thresholds

| Transition | Tasks Required | Additional Requirements |
|-----------|---------------|------------------------|
| 1 → 2 | 5 successful L1 tasks | Confidence ≥ 0.8 in primary domain |
| 2 → 3 | 10 successful L2 tasks | Confidence ≥ 0.8 in primary domain |
| 3 → 4 | 15 successful L3 tasks | Confidence ≥ 0.8 in primary domain |
| 4 → 5 | 20 successful L4 tasks | + 1 novel contribution to framework |

---

## Confidence Adjustment Rules

| Event | Adjustment |
|-------|-----------|
| Successful task | +0.05 (max 1.0) |
| Failed task | -0.10 |
| Novel technique discovered | +0.08 |
| Repeated same failure mode | -0.15 (compounding penalty) |
| Pattern contributed to framework | +0.10 |
| Regression (broke existing functionality) | -0.20 |

---

## Prevention of Documentation Drift

The following mechanisms prevent the documentation drift documented in failure F002:

| Mechanism | Layer | What It Prevents |
|-----------|-------|-----------------|
| **MANDATORY flag** (metacognition-pipeline.md) | Design-time | Silent skipping of stages 7-8 |
| **Post-task checklist** (metacognition-pipeline.md) | Runtime | "I forgot to extract" |
| **Cognitive audit loop** (cognitive-audit-loop.md) | Enforcement | Unverified task completion claims |
| **NON-NEGOTIABLE rule** (this protocol) | Governance | Cultural acceptance of incomplete tasks |
| **Task blocking** (cognitive-audit-loop.md) | Consequence | Accumulation of undocumented experience |
| **Async reconciliation** (cognitive-audit-loop.md) | Resilience | Timeout = partial, not skipped |
| **Audit trail** (cognitive-audit-loop.md) | Transparency | Every task has extract/update proof |

Together, these 7 mechanisms form a 3-layer defense (design + runtime + governance) that eliminates the gap between pipeline design and pipeline execution.

---

## Related Documents

| Document | Purpose |
|----------|---------|
| [metacognition-pipeline.md](../workflows/metacognition-pipeline.md) | Full 8-stage pipeline with mandatory stages 7-8 |
| [cognitive-audit-loop.md](../workflows/cognitive-audit-loop.md) | Post-task enforcement workflow (guardian of this protocol) |
| [LEARNING_PROTOCOL.md](../memory/LEARNING_PROTOCOL.md) | Learning entry format and semantic tagging rules |
| [failures.md — F002](../memory/agent/cosca-kernel/failures.md) | The systemic failure this protocol fixes |
| [AGENT_DNA.md](../AGENT_DNA.md) | Agent identity and capability model |

---

> **Protocol Status**: ACTIVE. All agents bound by this protocol. Violation tracking via cognitive-audit-loop.  
> **Last Compliance Check**: 2026-07-30 — F002 mitigation deployed via F0.6.  
> **Next Review**: After 100 tasks with audit loop active (metric revalidation).
