# cosca-governance - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-governance — Semantic Learnings

> Auto-evolution memory.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-governance |
| **Task** | Initial capability establishment |
| **Technique** | Standard governance patterns |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #governance #baseline |
| **Learned** | Ready for Level 2. |
| **Next** | Identify first advanced technique |

---

## Task History

### 2026-07-28 — Task #1: DNA v3.0 Full Compliance Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-governance |
| **Task** | Auditar conformidade de todos os 54 agentes Cosca com CONVENTIONS e DNA v3.0 |
| **Technique** | Bulk structural scan (bash/glob) + targeted content analysis + cross-reference verification |
| **Level** | 1 → transitioning to 2 |
| **Outcome** | success |
| **Tags** | #governance #audit #dna-v3 #compliance #first-task |
| **Duration** | Single session |
| **Artifacts Produced** | `memory/governance/audit-report.md` (full report), updated `learnings.md` |

### What Worked Well
- **Parallel bulk scanning**: Using bash glob + grep to extract structural compliance data from all 54 profiles simultaneously was efficient — identified the pattern that 53/54 are template-identical within seconds
- **Template pattern recognition**: Quickly identified that Level 1 seed profiles follow a 33-line template; exceptions (cosca-semantic-memory) stood out immediately
- **Two-tier analysis**: Fast structural scan (section presence) followed by deep content analysis (domain overlaps, naming consistency) was the right approach
- **Reference document first**: Reading CONVENTIONS.md, AGENT_DNA.md, and GOVERNANCE.md upfront established the audit baseline before touching any agent file

### What Was Difficult
- **Subagent depth limits**: Attempted to parallelize content analysis via explore subagents but hit depth limit (1). Had to run bash/grep scans directly instead
- **Confidence score extraction**: Multi-line tables in capability profiles made automated extraction fragile — grep patterns captured header rows instead of data rows
- **Domain overlap assessment**: Without reading every PROMPT.md in full, had to infer domain boundaries from capability profile primary domains — moderate-risk pairs need deeper analysis
- **Kernel asymmetry**: cosca-kernel has memory but no agents/ directory — required special-case handling throughout

### Patterns Identified
1. **Two-template system**: Capability profiles use exactly 2 templates — Level 1 seed (33 lines, 0.25 confidence) and Level 3+ detailed (varies by agent). Level 2 agents inherit the Level 3+ template with fewer domains.
2. **PROMPT.md = simpler format**: PROMPT.md files follow version 1.0.0 system-prompt format, NOT DNA v3.0 28-field contract. DNA version is tracked in capability-profile.md only.
3. **Specialist gap**: All 9 specialists lack AUTO-EVOLUTION directive — suggests a systematic omission in the specialist template generation
4. **Orphaned files are pre-v3.0 artifacts**: 9 agent-*.md files use naming conventions from DNA v2.0 era (flat files, agent-* prefix, -chief suffix)
5. **INDEX.md is stale**: References orphaned files and doesn't reflect current 54-agent structure

### Confidence Update
- **Primary domain** (Framework governance): 0.25 → **0.45**
- **Evidence**: Successfully completed first real task — full audit of 54 agents against DNA v3.0, produced comprehensive report (12 sections, actionable findings), identified 1 blocker + 10 warnings, maintained cross-reference integrity
- **New secondary domain** (Audit methodology): 0.40 — demonstrated systematic audit approach combining structural scans + content analysis
- **Trend**: ↑ (first real execution validates agent definition)

### Evolution Goal Status
- Target: Reach Level 2 ("Complete first 5 real tasks and establish baseline confidence in primary domain")
- Progress: 1/5 tasks complete
- Next: Schedule quarterly re-audit (2026-10-28), fix blocker items (cosca-semantic-memory)

