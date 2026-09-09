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
