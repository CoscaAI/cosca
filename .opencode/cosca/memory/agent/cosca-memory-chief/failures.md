# cosca-memory-chief — Negative Memory (Failures)

> Auto-evolution memory. Failures are the most valuable teachers. Search before acting.

## Active Failures

### 2026-07-29 — Activation Audit: INDEX.md Claimed 0 Orphans (False)
| Field | Value |
|-------|-------|
| **Task** | Memory system activation audit |
| **Failure** | `memory/INDEX.md` reported "Orphans: 0" but 9 orphan `agent-*.md` flat files exist unindexed |
| **Root Cause** | INDEX.md was last verified when only 10 agent files existed; 9 files from DNA v2.0 era were never indexed or migrated |
| **Impact** | Low (files are not actively used) but misrepresentation erodes trust in health metrics |
| **Recovery** | Report documents all 9 orphans with disposition recommendations. INDEX.md will be updated. |
| **Prevention** | Add automated orphan detection to memory health checks — count files in agent/ that are not in a subdirectory |

---
> **Protocol**: [LEARNING_PROTOCOL.md](../../../LEARNING_PROTOCOL.md) | **Constitution**: P5 — A família aprende com erros
