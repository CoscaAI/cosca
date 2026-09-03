# cosca-cli — Semantic Learnings

> Auto-evolution memory.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-cli |
| **Task** | Initial capability establishment |
| **Technique** | Standard cli patterns |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #cli #baseline |
| **Learned** | Ready for Level 2. |
| **Next** | Identify first advanced technique |

### 2026-07-28 — Full CLI Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-cli |
| **Task** | Comprehensive CLI audit: coverage, UX, gap analysis of 37 top-level + 105 leaf commands |
| **Technique** | Systematic audit via build-verify-test-analyze pipeline |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #cli #audit #cobra #ux #completion #coverage |
| **Related** | root.go, adapters, output.go, Cobra CLI |
| **Learned** | 37/37 top-level commands confirmed functional. 105 leaf commands total. Adapter pattern bridges CLI to internal engines. Key UX gaps: no aliases, -v/-V flag swap, inconsistent --dry-run shorthand. Quick wins identified: aliases, grouping, completion hints. Build compiles, tests pass. |
| **Next** | Implement quick wins (aliases, flags, grouping). Then explore deeper CLI patterns: custom completions, command suggestions, middleware. |
