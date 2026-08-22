# cosca-semantic-memory — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Session: 2026-07-28 — First Indexing Cycle (C1)

### 2026-07-28 — Complete Semantic Indexing of 426 Memory Files

| Field | Value |
|-------|-------|
| **Agent** | cosca-semantic-memory |
| **Task** | First-cycle semantic indexing of all 426 memory files (Don's order, Fase C) |
| **Technique** | Level 2 — Multi-phase analysis: 1) inventory scan of 426 files across 19 categories/55 agents, 2) deep reading of all substantive content (capability profiles, learnings with substance, failures, evolution timelines, non-agent architecture/patterns/bugs/decisions), 3) topic classification into 16 semantic groups, 4) cross-agent cluster detection (8 clusters identified), 5) gap analysis (17 gaps across critical/major/structural tiers), 6) density heatmap generation |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #semantic-indexing #memory-analysis #cross-agent #clustering #gap-analysis #cycle-1 |
| **Related** | .opencode/cosca/memory/semantic/INDEX.md, .opencode/cosca/memory/MEMORY_SYSTEM.md, .opencode/cosca/memory/INDEX.md |
| **Learned** | 1) The memory ecosystem is sharply stratified: 78% of files are agent memory but only ~12% have substantive content (334 agent files, ~40 with real learnings). Non-agent memory types (architecture, patterns, bugs, decisions) have near-100% substantive density. 2) Only 3 of 55 agents have recorded failures — negative memory is severely underutilized despite being the most valuable learning resource per the AUTO_EVOLUTION protocol. 3) Four agents reached Level 3 (kernel, documentation, backend, runtime) through real task execution; 75% of agents (41/55) are template-only seeds with no execution history. 4) Cross-agent clusters naturally form around shared domain concerns: Auth/Security (5 agents), Documentation Integrity (5 agents), Runtime Lifecycle (4 agents), and Database Reality (4 agents) are the densest clusters. 5) The "PostgreSQL fantasy" pattern recurred across agents — memory can drift into aspirational/fictitious claims without go.mod + source code verification. 6) The agent "avoided failures" pattern (cosca-backend — 2 avoided failures from past negative memory) is an exemplary cross-agent learning mechanism that should be replicated. 7) Critical gaps: no automated security scanning, no runtime integration tests, no performance profiling data, no observability, no CI/CD automation, and 3 known bugs (Restart, EventStartupComplete, bug-005) found but unfixed. 8) Indexing approach: reading capability profiles for agent domains, learnings for execution history, failures for negative memory, evolution for confidence trajectories proved effective for mapping the full ecosystem without needing to read every single file. |
| **Next** | Level 3: Cycle 2 — implement semantic search with keyword + vector retrieval, auto-update index on memory changes, create agent learning feedback loop, enable cross-agent query: "show me all knowledge about X from any agent" |

### 2026-07-28 — Semantic Index Architecture Design

| Field | Value |
|-------|-------|
| **Agent** | cosca-semantic-memory |
| **Task** | Design the semantic index structure (INDEX.md) for Cosca memory system |
| **Technique** | Level 2 — Multi-dimensional index design: 1) Topic map (16 semantic groups with sub-topic tables linking to files), 2) Cross-agent cluster analysis (shared knowledge across ≥3 agents), 3) Capability matrix (55 agents with Level/Confidence/Primary Domain), 4) Gap analysis (3 tiers: P0 critical, P1 major, structural), 5) Coverage heatmap by memory type, 6) Measurable Cycle 2 targets |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #index-design #topic-map #capability-matrix #coverage-heatmap #cycle-planning |
| **Related** | .opencode/cosca/memory/semantic/INDEX.md |
| **Learned** | 1) Effective semantic index must support multiple access patterns: topic-first (what do we know about X?), agent-first (what does agent Y know?), cluster-first (which agents share knowledge about Z?), and gap-first (what's missing?). 2) Density is the key metric: agent memory is high-volume but low-density (12%); non-agent memory is low-volume but high-density (100%). 3) Clustering threshold of ≥3 agents sharing knowledge about the same domain produces meaningful clusters — 2-agent overlaps are too common to be useful. 4) Gap classification into P0 (blocking), P1 (important), and structural (cross-cutting) enables prioritization for Cycle 2. 5) The capability matrix should distinguish between designed capability (seed L1) and proven capability (L2+ with execution history) — confidence scores alone don't tell the full story. 6) Measurable targets for the next cycle make the index actionable: from 10→20 agents with substantive learnings, from 4→6 agents at L3+, from 0→3+ automated CI checks. |
| **Next** | Level 3: Build the auto-update mechanism — detect new/modified memory files, re-index affected clusters, update coverage metrics |

