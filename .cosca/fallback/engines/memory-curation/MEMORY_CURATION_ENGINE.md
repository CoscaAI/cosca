# MEMORY CURATION ENGINE — Self-Cleaning Knowledge System

> **Version**: 1.0.0 | **Status**: active | **Owner**: Memory Chief | **Created**: 2026-07-28
>
> **Upgrade**: [MEMORY_DECAY_ENGINE.md](MEMORY_DECAY_ENGINE.md) v1.0.0 — CurationScore v2.0 with frequency density, success rate, age decay, and conflict detection.
> **Constitutional authority**: [CONSTITUTION.md](../../CONSTITUTION.md) — Implements P7 (Memória sem poluição).
> **Depends on**: [CONFIDENCE_MODEL.md](../evidence/CONFIDENCE_MODEL.md) — Uses EvidenceConfidence to evaluate memory entries.

---

## Purpose

The Memory Curation Engine prevents the "1000 aprendizados → memória gigante → recuperação pior → contexto poluído" problem. It automatically evaluates, condenses, promotes, demotes, and removes memory entries based on their quality, usage, and relevance.

Without curation, every agent's learnings.md becomes an ever-growing append-only journal. With curation, the memory system becomes self-cleaning — more useful with time, not less.

---

## Architecture

```
                        ┌──────────────────────────────────┐
                        │     MEMORY CURATION ENGINE        │
                        │                                   │
                        │  Triggers:                        │
                        │  • Every 50 new entries           │
                        │  • Every 7 days (cron)            │
                        │  • Manual: cosca memory curate    │
                        └───────────────┬───────────────────┘
                                        │
            ┌───────────────────────────┼───────────────────────────┐
            ▼                           ▼                           ▼
   ┌─────────────────┐       ┌─────────────────┐       ┌─────────────────┐
   │   R1: AVALIAR   │       │  R2: CONDENSAR  │       │   R3: PODAR     │
   │   Classificar    │       │   Remover        │       │   Depreciar     │
   │   cada entrada   │       │   duplicatas     │       │   obsoletas     │
   │   (CurationScore)│       │   (similarity    │       │   (level N-2)   │
   └────────┬────────┘       │    > 80%)        │       └────────┬────────┘
            │                └────────┬─────────┘                │
            │                         │                          │
            └─────────────────────────┼──────────────────────────┘
                                      │
                                      ▼
                           ┌─────────────────────┐
                           │  R4: PROPAGAR       │
                           │  Promover entradas  │
                           │  validadas a         │
                           │  conhecimento global │
                           │  (CurationScore      │
                           │   ≥ 0.90)           │
                           └──────────┬──────────┘
                                      │
                                      ▼
                           ┌─────────────────────┐
                           │  R5: FALLBACK       │
                           │  Processar failures │
                           │  (resolver, escalar │
                           │   não resolvidos)   │
                           └──────────┬──────────┘
                                      │
                                      ▼
                           ┌─────────────────────┐
                           │  RELATÓRIO          │
                           │  + atualizar         │
                           │  INDEX.md            │
                           └─────────────────────┘
```

---

## R1 — AVALIAR: Classification by CurationScore

### Purpose
Assign a quality score to every memory entry across all agents. The score determines the entry's fate in subsequent rules.

### CurationScore Formula

```
CurationScore = (outcome × 0.35) + (usage × 0.25) + (recency × 0.20) + (evidence × 0.20)

Where each component is normalized to [0.0, 1.0]:

1. OUTCOME (weight: 0.35)
   success           → 1.00
   partial           → 0.55
   failure           → 0.30
   unknown/seed      → 0.15
   
2. USAGE (weight: 0.25)
   usage_count = number of times this entry was retrieved for a task
   normalized: min(usage_count, 10) / 10
   Entry nunca usada → 0.00
   Entry usada 10+ vezes → 1.00

3. RECENCY (weight: 0.20)
   days_since = days since last update or retrieval
   recency_score:
     < 7 dias    → 1.00
     7-30 dias   → 0.70
     30-90 dias  → 0.40
     > 90 dias   → 0.10

4. EVIDENCE (weight: 0.20)
   Uses EvidenceConfidence from CONFIDENCE_MODEL.md
   Entry backed by code evidence (level 5)      → 1.00
   Entry backed by test evidence (level 4)       → 0.90
   Entry from documentation (level 3)            → 0.60
   Entry from memory/opinion (level 2-1)         → 0.40
   Entry from LLM (level 0)                     → 0.15
```

### Classification

| CurationScore | Tier | Label | Action |
|---------------|------|-------|--------|
| **≥ 0.85** | ⭐ Elite | Validated knowledge | Candidate for R4 (promotion to global) |
| **0.70–0.84** | 📋 Active | Reliable entry | Keep active, re-evaluate next cycle |
| **0.50–0.69** | 📋 Active | Adequate entry | Keep active, flag for improvement |
| **0.30–0.49** | ⏳ Pending | Low quality | Move to `pending/`, re-evaluate in 30 days |
| **< 0.30** | 🗑️ Deprecated | Unreliable/obsolete | Move to `deprecated/`, auto-remove in 90 days |

### Scoring Examples

```
Entry: "Complete API Surface Mapping" (cosca-backend, 2026-07-28)
  Outcome: success                 → 1.00 × 0.35 = 0.350
  Usage: 3 retrievals              → 0.30 × 0.25 = 0.075
  Recency: 0 days                  → 1.00 × 0.20 = 0.200
  Evidence: backed by code audit   → 0.90 × 0.20 = 0.180
  CURATIONSCORE: 0.805 → Tier: 📋 Active (Reliable)

Entry: "Slow Dashboard Query" (bug-005, 2026-07-22)
  Outcome: failure (template bug)  → 0.30 × 0.35 = 0.105
  Usage: 0 retrievals              → 0.00 × 0.25 = 0.000
  Recency: 6 days                  → 1.00 × 0.20 = 0.200
  Evidence: fake commit hash       → 0.15 × 0.20 = 0.030
  CURATIONSCORE: 0.335 → Tier: ⏳ Pending (would be deprecated)
  STATUS: Already removed in Fase 3 cleanup ✅

Entry: "PostgreSQL Architecture" (memory/architecture, 2026-07-23)
  Outcome: factual error           → 0.30 × 0.35 = 0.105
  Usage: 2 retrievals              → 0.20 × 0.25 = 0.050
  Recency: 5 days                  → 1.00 × 0.20 = 0.200
  Evidence: contradicted by code   → 0.15 × 0.20 = 0.030
  CURATIONSCORE: 0.385 → Tier: ⏳ Pending
  ACTION: Move to pending/, flag for correction
  RESULT: Already corrected in Fase 1 ✅
```

---

## R2 — CONDENSAR: Duplicate Removal

### Purpose
Detect and merge entries that say essentially the same thing, preventing redundant memory.

### Detection

Two entries are candidates for condensation when:

1. **Same tags** (≥ 2 matching tags)
2. **Same agent** or **same domain** agents
3. **Similarity > 80%** — measured by:
   - Embedding cosine similarity (via vector store)
   - OR FTS5 keyword overlap (fallback)

### Condensation Algorithm

```
function condense_duplicates(entry_a, entry_b):
    // Keep the entry with higher CurationScore
    IF entry_a.curation_score >= entry_b.curation_score:
        KEEPER = entry_a
        REMOVED = entry_b
    ELSE:
        KEEPER = entry_b
        REMOVED = entry_a
    
    // Merge metadata
    KEEPER.tags = UNION(KEEPER.tags, REMOVED.tags)
    KEEPER.related = UNION(KEEPER.related, REMOVED.related)
    KEEPER.times_consolidated += 1
    KEEPER.consolidated_from = APPEND(KEEPER.consolidated_from, REMOVED.key)
    
    // Archive removed entry
    REMOVED.status = "consolidated"
    REMOVED.consolidated_into = KEEPER.key
    MOVE(REMOVED → memory/agent/{agent}/deprecated/)
    
    RETURN KEEPER
```

### Example

```
Entry A: "API Surface Mapping" (cosca-backend, CS=0.805)
  Tags: #api #rest #handlers #middleware #auth
  Learned: "36 endpoints across 10 domains..."

Entry B: "Endpoint Coverage Audit" (cosca-backend, CS=0.650)
  Tags: #api #rest #endpoints #coverage
  Learned: "Complete coverage: agents, skills, providers..."

Similarity: 82% (both about API endpoint mapping)
Action: Merge B into A. A gains tags #endpoints #coverage.
Result: One entry instead of two. Context saved: ~120 tokens.
```

---

## R3 — PODAR: Obsolescence Removal

### Purpose
Remove or archive entries that are no longer relevant because the agent has evolved beyond them.

### Level-Based Obsolescence

```
For each agent at current_level = N:

  Entries at level N-2 (two levels below current):
    → Mark as "superseded"
    → Move to deprecated/
    → Reason: Agent has mastered techniques 2 levels above this
    
  Entries at level N-1:
    → Keep active (historical reference)
    → Add note: "Superseded by Level {N} technique: {technique_name}"
    
  Exception: FAILURE entries are NEVER removed by level obsolescence
    → Failures have preventive value regardless of agent level
    → See R5 instead
```

### Example

```
cosca-backend at Level 3:

  Level 1 entries (N-2):
    "Standard backend patterns — project conventions" [seed data]
    → SUPERSEDED. Move to deprecated/.
    
  Level 2 entries (N-1):
    "Complete API Surface Mapping" [useful history]
    → KEEP. Add reference to Level 3 techniques.
    
  Level 3 entries (current):
    "Metacognition Layer implementation"
    → ACTIVE.
```

---

## R4 — PROPAGAR: Knowledge Promotion

### Purpose
Entries that demonstrate exceptional quality and cross-agent validation are promoted from individual agent memory to global knowledge.

### Promotion Criteria

An entry is promoted when ALL of these are true:

1. **CurationScore ≥ 0.90** (elite tier)
2. **Validated by ≥ 2 different agents** (cross-agent corroboration via M2 modifier)
3. **Backed by level 4-5 evidence** (code or tests)
4. **Applied successfully ≥ 5 times** (proven pattern)

### Promotion Process

```
1. Entry meets all 4 criteria
   ↓
2. Extract generic pattern from entry:
   - Remove project-specific details
   - Keep technique, approach, and lessons
   - Add "Applicable to" section (stacks, domains)
   ↓
3. Create pattern entry in patterns.md:
   Pattern: {name}
   Domain: {domain}
   Confidence: {EvidenceConfidence}
   Times Applied: {count}
   Times Succeeded: {count}
   Source Agent: {agent}
   Validated By: [{agent1}, {agent2}]
   ↓
4. Index in knowledge engine with tag #validated-knowledge
   ↓
5. Notify agents in same domain via event bus
   ↓
6. Original entry in learnings.md marked: "promoted_to_pattern: {pattern_key}"
```

### Example

```
Entry: "Auth Middleware Chain Audit" (cosca-security, CS=0.92)
  Applied: 7 times, 100% success
  Validated by: cosca-backend, cosca-documentation
  
  PROMOTED TO PATTERN:
  Pattern: "middleware-chain-audit"
  Domain: security, backend
  Technique: "Trace request lifecycle: outermost→innermost middleware.
              Verify auth bypasses, CSRF gaps, rate limit bypasses.
              Document chain order in buildHandler()."
  Confidence: 0.94
```

---

## R5 — FALLBACK: Failure Memory Management

### Purpose
Failures are the most valuable learning resource — they are NEVER automatically removed. But they are managed: resolved when a solution is found, escalated when they remain unresolved.

### Failure Lifecycle

```
ACTIVE FAILURE (no solution yet)
  │
  ├── Solution found → RESOLVED
  │     │
  │     └── Related Success link added
  │         → "Related Success: learnings.md#2026-07-20-database-optimization"
  │
  └── 90 days without solution → UNRESOLVED
        │
        └── Escalate to Chief for review
            → Should this failure mode be:
              a) Investigated (assign task)
              b) Accepted as known limitation
              c) Deprioritized (low impact)
```

### Repeated Failure Detection

When the SAME failure pattern occurs across DIFFERENT agents:

```
Agent A: "Aggressive caching on auth endpoints" (2026-06-15)
Agent B: "Cache invalidation race condition" (2026-07-10)
Agent C: "Stale permission data in cache" (2026-07-28)

→ Pattern detected: "Cache + Auth = Danger"
→ Promoted to ORGANIZATIONAL FAILURE
→ All agents receive avoidance pattern
→ Added to AGENT_DNA.md Known Failure Modes for backend, security
```

### Failure Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Total failures recorded | 3 (cosca-backend) | Tracked per agent |
| Resolved failures (%) | 100% (3/3 têm Related Success) | > 80% |
| Repeated failures (same agent) | 0 | 0 |
| Cross-agent failures (same pattern) | 0 | < 2 per quarter |
| Unresolved failures | 0 | < 3 at any time |

---

## Execution Cycle

### Triggers

| Trigger | Description |
|---------|-------------|
| **Volume** | Every 50 new entries across all agents (learnings.md + failures.md + patterns.md) |
| **Time** | Every 7 days (scheduled) |
| **Manual** | `cosca memory curate` (Don or Kernel initiated) |

### Cycle Execution

```
CURATION CYCLE:

1. SCAN
   ├── Collect all learnings.md entries from all agents
   ├── Collect all failures.md entries
   └── Count: total entries before curation

2. SCORE (R1)
   ├── Calculate CurationScore for each entry
   ├── Classify into tiers (Elite, Active, Pending, Deprecated)
   └── Flag entries with CurationScore < 0.30

3. CONDENSE (R2)
   ├── Group entries by agent + domain
   ├── Detect duplicates (similarity > 80%)
   ├── Merge duplicates
   └── Count: entries consolidated

4. PRUNE (R3)
   ├── For each agent at Level N:
   │   └── Mark Level N-2 entries as superseded
   └── Count: entries deprecated

5. PROMOTE (R4)
   ├── Find entries with CurationScore ≥ 0.90
   ├── Check cross-agent validation
   ├── Extract patterns
   └── Count: entries promoted to global

6. FAILURE REVIEW (R5)
   ├── Check for resolved failures (Related Success exists)
   ├── Flag unresolved > 90 days
   ├── Detect cross-agent failure patterns
   └── Count: failures resolved, escalated

7. REPORT
   ├── Generate CurationReport
   ├── Update INDEX.md for affected agents
   └── Publish EventMemoryCurationComplete
```

---

## Curation Report

After each cycle, the engine generates a report:

```json
{
  "cycle_id": "cur-2026-07-28-001",
  "timestamp": "2026-07-28T10:00:00Z",
  "trigger": "manual",
  "pre_curation": {
    "total_entries": 312,
    "total_agents": 7,
    "avg_curation_score": 0.48
  },
  "actions": {
    "R1_scored": 312,
    "R2_condensed": 3,
    "R3_deprecated": 5,
    "R4_promoted_to_global": 1,
    "R5_failures_resolved": 1,
    "R5_failures_escalated": 0
  },
  "post_curation": {
    "total_entries": 303,
    "active_entries": 250,
    "pending_entries": 12,
    "deprecated_entries": 41,
    "avg_curation_score": 0.56
  },
  "impact": {
    "entries_removed": 9,
    "context_freed_tokens": 2400,
    "health_improvement": "+0.08 avg CurationScore",
    "duplicates_eliminated": 3
  },
  "warnings": [
    "cosca-mobile: 0 active entries, agent never executed",
    "cosca-performance: 3 entries with CurationScore < 0.40"
  ]
}
```

---

## Integration Points

| System | How it integrates |
|--------|------------------|
| **CONFIDENCE_MODEL.md** | Evidence component of CurationScore uses EvidenceConfidence |
| **LEARNING_PROTOCOL.md** | Defines entry format that curation engine evaluates |
| **AGENT_DNA.md** | Failure modes propagated to capability profiles |
| **Metacognition Pipeline** | UPDATE CAPABILITY MODEL stage triggers curation check |
| **Knowledge Engine** | Promoted patterns indexed via FTS5 + vector for cross-agent search |
| **Event Bus** | `EventMemoryCurationComplete` notifies affected agents |

---

> **Related**: [CONSTITUTION.md](../../CONSTITUTION.md) P7 | [CONFIDENCE_MODEL.md](../evidence/CONFIDENCE_MODEL.md) | [LEARNING_PROTOCOL.md](../../memory/LEARNING_PROTOCOL.md) | [AGENT_DNA.md](../../AGENT_DNA.md)
