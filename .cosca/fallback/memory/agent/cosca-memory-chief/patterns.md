# cosca-memory-chief — Reusable Patterns

> Discovered patterns that can be reapplied.

## Patterns Discovered

### P1 — Orphan Detection via Directory Structure Analysis
**Context**: Memory systems accumulate orphaned files as they evolve across versions (DNA v2.0 → v3.0). These files are invisible to INDEX.md-based navigation.
**Pattern**: Compare `ls` output of a directory against INDEX.md references. Any file not referenced in INDEX.md or any subdirectory is orphaned.
**Example**: 9 `agent-*.md` flat files in `memory/agent/` existed alongside 54 cosca-*/ subdirectories but were listed nowhere.
**Detection command**: `ls -p memory/agent/ | grep -v /` — lists all non-directory entries (orphans).

### P2 — INDEX.md Coverage Ratio as Health Metric
**Context**: An INDEX.md that references fewer than 100% of items in its directory is stale.
**Pattern**: Define coverage = (items referenced in INDEX.md) / (total items in directory). Target: 100%. Alert at <90%.
**Example**: `memory/agent/INDEX.md` covered 11/54+ agents = 20% coverage.

### P3 — Duplicate Directory Detection via Semantic Overlap
**Context**: Memory directories with similar names (`session/` vs `sessions/`, `decision/` vs `decisions/`) cause fragmentation.
**Pattern**: Flag any pair of directories sharing 4+ contiguous characters in name. Review for consolidation.
**Example**: `session/` (vestigial, 1 file) and `sessions/` (active + archive) should be merged.

### P4 — Knowledge Staleness Detection via Confidence Decay
**Context**: Knowledge entries age. A learning recorded months ago may be invalid but still carries equal weight as fresh knowledge. This causes agents to apply outdated techniques.
**Pattern**: Every learning entry carries `last_validated`, `wisdom_decay_category`, and auto-calculated `confidence`. Confidence decays continuously: 1.00 (fresh) → 0.95 (7d) → 0.80 (30d) → 0.60 (90d) → 0.40 (180d) → 0.20 (365d). Categories apply decay multipliers: CRITICAL ×0.3, STABLE ×1.0, EXPERIMENTAL ×2.0.
**Detection**: Weekly audit scans all learnings.md, calculates `confidence = max(0.10, 1.0 - (days_since / 365) × 0.80 × category_multiplier)`, flags entries below 0.70 for revalidation and below 0.30 as expired.
**Example**: A STABLE learning from 2026-07-27 has confidence 0.80 by 2026-10-25 (90 days). An EXPERIMENTAL learning from the same date drops to 0.60 in the same period.
**Reference**: [WISDOM_DECAY.md](../../WISDOM_DECAY.md) — full specification.

### P5 — Restore HEAD, Then Add Canonical Records
**Context**: A post-task record was written to a generated embed mirror instead of its canonical fallback source.
**Pattern**: Preserve the mirror's additive diff, restore the affected mirror files exactly to HEAD, apply only those additions to the canonical files, run the required sync, then audit and revert out-of-scope generated drift.
