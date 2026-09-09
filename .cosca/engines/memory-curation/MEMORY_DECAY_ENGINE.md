# MEMORY DECAY ENGINE — Neural Forgetting for Agent Memory

> **Version**: 1.0.0 | **Status**: active | **Owner**: Memory Chief | **Created**: 2026-07-28
>
> **Extends**: [MEMORY_CURATION_ENGINE.md](MEMORY_CURATION_ENGINE.md) v1.0.0 — Upgrades CurationScore from v1.0 → v2.0
> **Constitutional authority**: [CONSTITUTION.md](../../identidade/CONSTITUTION.md) — Implements P7 (Memória sem poluição).
> **Approved by**: Don — "um cérebro eficiente também esquece"

---

## Purpose

The Memory Decay Engine prevents the "crescimento infinito" problem. The Curation Engine (v1.0) evaluates entries — but it evaluates them at a point in time with a static formula. The Decay Engine adds the **temporal dimension**: entries naturally lose weight over time unless reinforced by usage, and conflicting patterns trigger automatic weight reduction.

**The core insight:** Memory should not be append-only. It should breathe — strengthen useful memories, weaken obsolete ones, forget what no longer serves.

---

## What Changes from CurationScore v1.0

### v1.0 Formula (Static)

```
CurationScore = (outcome × 0.35) + (usage × 0.25) + (recency × 0.20) + (evidence × 0.20)
```

Problems with v1.0:
- **Usage** counts total retrievals but doesn't distinguish "used 150x in 1 month" (hot) from "used 10x in 12 months" (cold)
- **Outcome** is binary per-entry — doesn't capture success rate over time (a pattern that worked once but fails now)
- **Recency** only scores freshness — doesn't model natural decay
- **No conflict detection** — new knowledge doesn't challenge old knowledge

### v2.0 Formula (Dynamic with Decay)

```
CurationScore_v2 = CurationScore_v1 × DecayFactor + ConflictPenalty

Where:
  CurationScore_v1 = same as before (outcome × 0.35 + usage × 0.25 + recency × 0.20 + evidence × 0.20)

  DecayFactor = function of (frequency_density, success_rate, age)

  ConflictPenalty = negative adjustment when entry conflicts with newer, higher-confidence patterns
```

---

## Component 1 — Frequency Density

### Problem
v1.0 usage counting is naive: 10 retrievals in 1 week is very different from 10 retrievals in 1 year. The first is "hot knowledge," the second is "occasionally useful."

### Formula

```
frequency_density = total_retrievals / max(weeks_since_creation, 1)

Normalized:
  density ≥ 5.0/week  → score 1.00  (HOT — used multiple times per week)
  density 2.0–4.9/week → score 0.85  (WARM — used a few times per week)
  density 1.0–1.9/week → score 0.70  (ACTIVE — used weekly)
  density 0.3–0.9/week → score 0.50  (LUKEWARM — used monthly)
  density 0.1–0.2/week → score 0.30  (COLD — used quarterly)
  density < 0.1/week   → score 0.10  (FROZEN — barely used)

frequency_score = density_score × 0.20  (weight in DecayFactor)
```

### Examples

```
Entry A: "API Surface Mapping"
  Created: 28 days ago (4 weeks)
  Retrievals: 32
  Density: 32/4 = 8.0/week → HOT → 1.00

Entry B: "Legacy Auth Pattern v1"
  Created: 180 days ago (~26 weeks)
  Retrievals: 5
  Density: 5/26 = 0.19/week → COLD → 0.30
```

---

## Component 2 — Success Rate

### Problem
v1.0 `outcome` is a single value per entry. But knowledge evolves: a pattern that worked 10 times might fail the 11th time because the codebase changed. We need a **rolling success rate** that decays when failures accumulate.

### Formula

```
success_rate = successful_applications / total_applications

Tracked per entry over time:
  - Each time entry is retrieved and applied → record outcome (success/failure)
  - Rolling window: last 20 applications (or all if < 20)
  - Minimum 3 applications before rate is considered reliable

success_rate_score:
  rate ≥ 0.95  → 1.00  (proven pattern)
  rate 0.85–0.94 → 0.85  (reliable)
  rate 0.70–0.84 → 0.60  (adequate, declining)
  rate 0.50–0.69 → 0.35  (unreliable)
  rate < 0.50   → 0.10  (mostly fails — candidate for deprecation)
  rate N/A (< 3 apps) → 0.50 (neutral, insufficient data)

success_rate_score_weight = 0.20  (weight in DecayFactor)
```

### Examples from the Don

```
Pattern A: "Auth middleware chain with JWT validation"
  Applied: 150 times
  Succeeded: 147 times
  Success Rate: 147/150 = 0.98 → 1.00 (PROVEN)
  Weight: HIGH — Don's intuition confirmed

Pattern B: "Direct SQL for reporting queries"
  Applied: 5 times
  Failed: 3 times
  Success Rate: 2/5 = 0.40 → 0.10 (MOSTLY FAILS)
  Weight: REDUZIR — Don's intuition confirmed
```

---

## Component 3 — Age Decay

### Problem
v1.0 `recency` is a snapshot score. But natural memory decay is continuous: knowledge fades gradually, not in steps.

### Formula

```
age_in_days = days since entry was created (or last updated)

Natural Decay Curve (half-life model):
  base_decay = 0.5 ^ (age_in_days / half_life_days)

  half_life_days depends on entry type:
    Code patterns (técnicas):      half_life = 180 days (codebases change slowly)
    Project facts (estado):        half_life = 90 days  (project state evolves)
    Process/tooling (processos):   half_life = 60 days  (tools, workflows change faster)
    Bug fixes (correções):         half_life = 365 days (bugs teach forever)
    Architecture decisions (ADRs): half_life = 270 days (architecture is relatively stable)

age_score = base_decay  (already in [0.0, 1.0])

Reinforcement: Each retrieval RESETS the age clock for that entry
  → age_in_days = days since LAST RETRIEVAL (not since creation)
  → This is the "use it or lose it" mechanism

age_score_weight = 0.10  (weight in DecayFactor)
```

### Visual Decay Curves

```
Score
1.0 ┤╲
    ┤ ╲________ (code patterns: 180d half-life)
0.5 ┤     ╲
    ┤      ╲___ (process: 60d half-life)
0.25┤        ╲
    ┤_________╲___
0.0 └──────────────► Days
    0   60  180  365

Reinforcement: each retrieval resets to 1.0
```

### Examples

```
Entry A: "Race condition fix — sync.Mutex pattern"
  Type: bug fix → half_life = 365 days
  Created: 8 days ago, retrieved 3 days ago
  age_in_days = 3 (since last retrieval)
  base_decay = 0.5 ^ (3/365) = 0.994 → barely decayed
  Age Score: 0.994

Entry B: "Old deployment script using Makefile v1"
  Type: process → half_life = 60 days
  Created: 120 days ago, last retrieved 90 days ago
  age_in_days = 90
  base_decay = 0.5 ^ (90/60) = 0.5 ^ 1.5 = 0.354
  Age Score: 0.354 → significantly decayed
```

---

## DecayFactor Assembly

```
DecayFactor = (frequency_score × 0.40) + (success_rate_score × 0.40) + (age_score × 0.20)

Weights rationale:
  - frequency (0.40): usage is the strongest signal of value
  - success_rate (0.40): equally strong — a frequently used but failing pattern is dangerous
  - age (0.20): natural decay is real but should not dominate; reinforcement resets it

DecayFactor range: [0.0, 1.0]
  → DecayFactor = 1.00 : entry is actively used, highly successful, recently accessed
  → DecayFactor = 0.00 : entry is unused, failing, and ancient
```

---

## CurationScore v2.0 — Complete Formula

```
CurationScore_v2 = (CurationScore_v1 × DecayFactor) - ConflictPenalty

Expanded:
  CurationScore_v2 = (outcome × 0.35 + usage × 0.25 + recency × 0.20 + evidence × 0.20)
                     × (frequency × 0.40 + success_rate × 0.40 + age × 0.20)
                     - conflict_penalty

Where:
  - All components normalized to [0.0, 1.0]
  - CurationScore_v2 clamped to [0.0, 1.0] (never negative)
  - DecayFactor < 0.30 triggers automatic review
```

### Worked Example

```
Entry: "Auth Middleware Chain" (cosca-security)
  Created: 60 days ago | Last retrieved: 2 days ago
  Type: code pattern → half_life = 180 days
  Applications: 150 total, 147 succeeded

v1.0 Components:
  outcome (success)      → 1.00 × 0.35 = 0.350
  usage (150, capped 10) → 1.00 × 0.25 = 0.250
  recency (2 days)       → 1.00 × 0.20 = 0.200
  evidence (code)        → 1.00 × 0.20 = 0.200
  CurationScore_v1 = 1.000

Decay Components:
  frequency_density = 150/8.57 = 17.5/week → 1.00 × 0.40 = 0.400
  success_rate = 147/150 = 0.98 → 1.00 × 0.40 = 0.400
  age = 0.5^(2/180) = 0.992 × 0.20 = 0.198
  DecayFactor = 0.998

CurationScore_v2 = 1.000 × 0.998 - 0 = 0.998

Tier: ⭐ Elite (≥ 0.85) — actively used, proven, recently accessed
```

```
Entry: "Old Deployment Script" (cosca-devops)
  Created: 120 days ago | Last retrieved: 90 days ago
  Type: process → half_life = 60 days
  Applications: 5 total, 2 succeeded

v1.0 Components:
  outcome (partial)      → 0.55 × 0.35 = 0.193
  usage (5, capped 10)   → 0.50 × 0.25 = 0.125
  recency (90 days)      → 0.40 × 0.20 = 0.080
  evidence (opinion)     → 0.40 × 0.20 = 0.080
  CurationScore_v1 = 0.478

Decay Components:
  frequency_density = 5/17.1 = 0.29/week → 0.30 × 0.40 = 0.120
  success_rate = 2/5 = 0.40 → 0.10 × 0.40 = 0.040
  age = 0.5^(90/60) = 0.354 × 0.20 = 0.071
  DecayFactor = 0.231

CurationScore_v2 = 0.478 × 0.231 = 0.110

Tier: 🗑️ Deprecated (< 0.30) — move to deprecated/, auto-remove in 90 days
```

---

## New Rule — R6: CONFLICT DETECTION

### Purpose

When a new learning contradicts an existing pattern, the older entry's weight is automatically reduced. This prevents the system from being "locked in" to outdated knowledge.

### Detection Algorithm

```
function detect_conflicts(new_entry, existing_entries):
    conflicts = []

    FOR each existing_entry in same_domain(new_entry):
        similarity = cosine_similarity(new_entry.embedding, existing_entry.embedding)

        IF similarity > 0.60 AND // Same topic
           new_entry.conclusion != existing_entry.conclusion AND // Different conclusion
           new_entry.evidence_level > existing_entry.evidence_level: // Newer, better evidence

            conflicts.append({
                old_entry: existing_entry.key,
                new_entry: new_entry.key,
                similarity: similarity,
                evidence_delta: new_entry.evidence_level - existing_entry.evidence_level
            })

    RETURN conflicts
```

### Conflict Resolution

```
FOR each conflict:

    // Calculate ConflictPenalty for the OLDER entry
    evidence_gap = conflict.evidence_delta  // how much better is the new evidence?

    IF evidence_gap >= 3:
        // New entry has MUCH better evidence (e.g., code vs. opinion)
        penalty = 0.40  // Heavy penalty — old pattern is likely wrong

    ELSE IF evidence_gap >= 2:
        penalty = 0.25  // Moderate penalty

    ELSE IF evidence_gap >= 1:
        penalty = 0.10  // Light penalty — may still be valid in some contexts

    ELSE:
        penalty = 0.00  // Similar evidence level — let the scores compete naturally

    // Apply penalty to old entry's CurationScore_v2
    old_entry.curation_score_v2 = max(0, old_entry.curation_score_v2 - penalty)
    old_entry.conflicts_with = APPEND(old_entry.conflicts_with, new_entry.key)
    new_entry.supersedes = APPEND(new_entry.supersedes, old_entry.key)

    // Log the paradigm shift
    EVENT "memory.conflict_detected" {
        old: old_entry.key (score was: X, now: X - penalty),
        new: new_entry.key (score: Y),
        reason: "New pattern with stronger evidence contradicts old pattern"
    }
```

### Example

```
Entry A (OLD): "Use REST for all service-to-service communication"
  Evidence: opinion (level 2) — "this is the standard approach"
  CurationScore_v2: 0.75 (Active)

Entry B (NEW): "gRPC is better for internal service communication"
  Evidence: code + benchmark (level 5) — "measured 40% latency reduction"
  CurationScore_v2: 0.88 (Elite)

Similarity: 0.72 (same topic: service communication)
Conclusion conflict: REST vs gRPC
Evidence delta: 5 - 2 = 3

ConflictPenalty for Entry A: 0.40
Entry A new score: 0.75 - 0.40 = 0.35 → ⏳ Pending

Entry B gains supersedes: [Entry A]
```

---

## Tier Thresholds (Updated for v2.0)

With DecayFactor and ConflictPenalty, scores can drop faster. Adjusted thresholds:

| CurationScore_v2 | Tier | Label | Action |
|---|---|---|---|
| **≥ 0.85** | ⭐ Elite | Validated knowledge | Candidate for R4 (promotion) |
| **0.65–0.84** | 📋 Active | Reliable entry | Keep active |
| **0.45–0.64** | 📋 Active | Adequate entry | Keep active, flag for improvement |
| **0.25–0.44** | ⏳ Pending | Low quality/decaying | Move to `pending/`, re-evaluate in 14 days |
| **< 0.25** | 🗑️ Deprecated | Unreliable/obsolete | Move to `deprecated/`, auto-remove in 60 days |

**Key changes from v1.0:**
- Elite threshold unchanged (0.85)
- Active lower bound dropped from 0.70 → 0.65 (decay is natural, not a failure signal)
- Pending lower bound dropped from 0.30 → 0.25 (decayed entries may still have value)
- Pending re-evaluation shortened from 30 → 14 days (decaying entries should be resolved faster)
- Deprecated auto-removal shortened from 90 → 60 days (decayed entries add noise faster)

---

## Anti-Decay: Reinforcement Mechanisms

Entries can RESIST decay through reinforcement:

| Mechanism | Effect | Trigger |
|---|---|---|
| **Retrieval** | Resets age clock | Entry is retrieved for a task |
| **Successful application** | +1 to success counter | Entry is applied and task succeeds |
| **Cross-agent validation** | +0.05 to DecayFactor | Another agent independently confirms the pattern |
| **Code evidence upgrade** | +0.10 to DecayFactor | Entry is backed by new code/test evidence |
| **Manual pin** | Immune to R3 pruning | Don or Chief marks entry as `pinned: true` |

### Pinned Entries

Some knowledge is eternal:
- Constitutional principles
- ADRs that define architecture
- Security-critical patterns
- Bug fixes with prevention value

Pinned entries:
- Still tracked for frequency, success_rate, age
- Still subject to DecayFactor (for metrics visibility)
- But NEVER moved to deprecated/ by R3 or R6
- Manual unpin required to allow natural decay

```
Entry frontmatter:
  pinned: true
  pinned_by: "Don"
  pinned_reason: "Security-critical auth pattern — must never be forgotten"
```

---

## Memory Health Metrics

| Metric | Formula | Target |
|---|---|---|
| **Memory Density** | active_entries / total_entries | > 70% (less than 30% deprecated) |
| **Avg DecayFactor** | mean(DecayFactor) across all entries | > 0.60 |
| **Conflict Rate** | conflicts_detected / new_entries_per_cycle | < 5% (healthy: few conflicts) |
| **Forgetting Rate** | entries_deprecated_per_cycle / total_entries | 2-5% (too low = stagnation, too high = amnesia) |
| **Reinforcement Rate** | entries_retrieved_per_cycle / total_entries | > 30% (active use of memory) |
| **Pinned Ratio** | pinned_entries / active_entries | < 10% (too many pins = manual curation overload) |

---

## Integration with Curation Engine

The Decay Engine runs as a **pre-pass** before each Curation Cycle:

```
CURATION CYCLE v2.0:

0. DECAY PRE-PASS (NEW)
   ├── Calculate frequency_density for all entries
   ├── Calculate success_rate for all entries
   ├── Calculate age_score for all entries
   ├── Compute DecayFactor for all entries
   ├── Detect conflicts (R6) — apply ConflictPenalties
   └── Compute CurationScore_v2 for all entries

1. SCAN (unchanged)
2. SCORE (R1) — uses CurationScore_v2 instead of v1
3. CONDENSE (R2) — unchanged
4. PRUNE (R3) — uses v2.0 thresholds
5. PROMOTE (R4) — unchanged
6. FAILURE REVIEW (R5) — unchanged
7. REPORT — now includes Decay metrics
```

---

## Required Data per Entry

Each memory entry now tracks:

```yaml
# Existing (v1.0)
outcome: success | partial | failure
usage_count: int
last_retrieved: timestamp
evidence_level: 0-5

# New (v2.0 — Decay Engine)
total_applications: int          # How many times applied (for success_rate)
successful_applications: int     # How many times succeeded
frequency_density: float         # retrievals/week
success_rate: float              # rolling rate (last 20 apps)
age_score: float                 # half-life decay
decay_factor: float              # composite 0-1
curation_score_v2: float         # final score with decay
conflicts_with: [string]         # Keys of conflicting entries
supersedes: [string]             # Keys of entries this supersedes
pinned: bool                     # Immune to pruning
half_life_days: int              # Based on entry type
last_decay_calc: timestamp       # When DecayFactor was last computed
```

---

## Fallback: When Decay Goes Wrong

### Over-Decay (Amnesia)

**Symptom:** Forgetting Rate > 10% per cycle. Valuable knowledge being lost.

**Recovery:**
1. Pause auto-deprecation
2. Increase half-life values by 50%
3. Require manual review of deprecated entries before deletion
4. Restore any entry cited in last 7 days

### Under-Decay (Stagnation)

**Symptom:** Forgetting Rate < 1% per cycle. Memory growing without pruning.

**Recovery:**
1. Decrease half-life values by 30%
2. Lower deprecated auto-removal from 60 → 30 days
3. Increase conflict detection sensitivity (similarity threshold 0.60 → 0.50)

---

> **Related**: [MEMORY_CURATION_ENGINE.md](MEMORY_CURATION_ENGINE.md) — the engine this extends | [CONFIDENCE_MODEL.md](../evidence/CONFIDENCE_MODEL.md) — evidence levels | [platform-evolution-v1.4.0.md](../../memory/roadmap/platform-evolution-v1.4.0.md) — roadmap item #2
