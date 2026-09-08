# cosca-semantic-memory - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

# cosca-semantic-memory — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Session: 2026-08-29 — Re-indexing Cycle C2

### 2026-08-29 — Complete Semantic Re-index of 494 Memory Files (C2)

| Field | Value |
|-------|-------|
| **Agent** | cosca-semantic-memory |
| **Task** | Cycle C2 re-index of all 494 memory files (Don's order). Reconstruct derived index from canonical source; source files NOT modified. |
| **Technique** | Level 2 — Same multi-phase method as C1: 1) full recursive inventory scan (494 .md, 54 agent dirs, 9 flat agent files, 8 bugs, 15 ADRs, new per-domain audit/activation-report layer), 2) deep reading of capability-profiles, learnings with content, failures/patterns/evolution substance, non-agent artifacts, 3) topic classification into 27 semantic groups (added Living World/generative + Quality Engineering as new), 4) cross-agent cluster detection (10 clusters, +2 new: Quality/Test Engineering, Living World), 5) capability matrix regeneration (16 L3+, 28 L2, 10 L1), 6) gap re-evaluation vs C1, 7) cycle bump C1→C2. |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #semantic-indexing #memory-analysis #cross-agent #clustering #gap-analysis #cycle-2 #reindex |
| **Related** | internal/embed/cosca/memory/semantic/INDEX.md, internal/embed/cosca/memory/INDEX.md, internal/embed/cosca/memory/MEMORY_SYSTEM.md |
| **Learned** | 1) **Re-index ≠ modify knowledge**: canonical source (.md files) untouched; only the derived semantic/INDEX.md regenerated — the "canonical source / derived index" separation held fully. 2) **Massive densification since C1**: substantive agent evidence went from 10/55 (18%) to 44/54 (81% with L≥2); L3+ agents 4→16. Root cause: Onda 2 + Onda 3 activation waves (2026-07-28) gave ~28 agents real tasks. 3) **KEY NEW FINDING — version drift**: ~16 agents show L3 evidence in learnings.md but capability-profile still at seed baseline (L1, conf 0.25). The capability matrix under-reports actual capability. This is a NEW structural gap that C1 never surfaced. 4) **New project domain "Living World"**: kernel/architecture/security/infrastructure evolved into generative-AI mining (Unreal UE5, 10-layer vision/spatial/VFX/audio/destruction/simulation, Sceelix PCG, Gaussian Splatting, ComfyUI, ADR-011 Mega Brain plan-only, ADR-012 2-zone air-gap). Absent entirely at C1. 5) **Bug registry grew 5→8**: bugs 006/007/008 (Restart, EventStartupComplete, metrics misdocumented) that C1 listed as "found but unfound/unregistered" are now formalized as files, though still open. 6) **P0 gaps mostly partially-closed**: perf profiling fully closed (baseline-report.md); CI/sec-ops/observability/integration-tests moved to "defined but not enforced". 7) **Density inversion**: the low-density agent-memory concern (12%) that dominated C1 is now resolved (82%), displaced by version-drift as the new structural risk. |
| **Next** | Level 3 targets: (a) automated index refresh on memory change, (b) build capability-profile↔learnings sync tool to close the version-drift gap, (c) codify Living World / generative ADRs (011/012) as first-class topics, (d) semantic search query "show all knowledge about X across agents". |

### 2026-08-29 — Semantic Index Cycle 2 Version-Drift Discovery

| Field | Value |
|-------|-------|
| **Agent** | cosca-semantic-memory |
| **Task** | Investigate why capability-profiles disagree with learnings-derived levels during C2 |
| **Technique** | Level 2 — Cross-reference: compare `**Level**` max in each learnings.md against `Current Level` in each capability-profile.md |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #version-drift #capability-matrix #data-quality #reindex #cycle-2 |
| **Related** | internal/embed/cosca/memory/semantic/INDEX.md §6 |
| **Learned** | 1) Agreement: capability-profile is the authoritative "current level", but it lags behind evidence. 2) The learnings `**Level**` field is a reliable proxy for real task execution (each entry records a task's technique level). 3) Root cause of drift: Onda waves scored learnings but did not re-run capability-profile updates. 4) Recommendation: for C3, a sync tool should propagate the max learnings level into capability-profile when capability-profile is stale (seed 0.25). |
| **Next** | Fix drift as Priority 2 of Cycle 3 recommendations. |

---

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
| **Related** | internal/embed/cosca/memory/semantic/INDEX.md, internal/embed/cosca/memory/MEMORY_SYSTEM.md, internal/embed/cosca/memory/INDEX.md |
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
| **Related** | internal/embed/cosca/memory/semantic/INDEX.md |
| **Learned** | 1) Effective semantic index must support multiple access patterns: topic-first (what do we know about X?), agent-first (what does agent Y know?), cluster-first (which agents share knowledge about Z?), and gap-first (what's missing?). 2) Density is the key metric: agent memory is high-volume but low-density (12%); non-agent memory is low-volume but high-density (100%). 3) Clustering threshold of ≥3 agents sharing knowledge about the same domain produces meaningful clusters — 2-agent overlaps are too common to be useful. 4) Gap classification into P0 (blocking), P1 (important), and structural (cross-cutting) enables prioritization for Cycle 2. 5) The capability matrix should distinguish between designed capability (seed L1) and proven capability (L2+ with execution history) — confidence scores alone don't tell the full story. 6) Measurable targets for the next cycle make the index actionable: from 10→20 agents with substantive learnings, from 4→6 agents at L3+, from 0→3+ automated CI checks. |
| **Next** | Level 3: Build the auto-update mechanism — detect new/modified memory files, re-index affected clusters, update coverage metrics |


