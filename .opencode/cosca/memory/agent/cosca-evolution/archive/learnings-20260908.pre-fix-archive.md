# cosca-evolution - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-evolution — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-evolution |
| **Task** | Initial capability establishment |
| **Technique** | Standard evolution patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #evolution #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core evolution patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — Activation-Level2

| Field | Value |
|-------|-------|
| **Agent** | cosca-evolution |
| **Task** | First activation — codebase evolution analysis across internal/ and pkg/ |
| **Technique** | Multi-package maturity scanning — interface density, god-file detection, test-gap triangulation, scorecard cross-validation |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #evolution #codebase-scan #interface-density #god-files #test-gaps #scorecard-crossref |
| **Related** | .opencode/cosca/memory/technical-debt/scorecard.md, .opencode/cosca/memory/agent/cosca-evolution/evolution.md |
| **Learned** | 63% of packages (27/43) define zero interfaces — dominant structural pattern. CLI is the largest monolith (52 files, 19,550 LOC, 0 interfaces). 4 critical-path packages have 0 tests (adapter, diagnostics, embed, embeddings). Scorecard COV-01 claim of "11 handlers at 0%" is stale — actually 19 handler files, still at 0% coverage. API middleware package has partial test coverage (cors, csrf, security have tests), contradicting the scorecard's "0%" claim. The `Restart()` implementation actually handles Stopped→Uninitialized transition correctly — scorecard BUG-U01 may be outdated or refers to a different code path. |
| **Next** | Deeper coupling analysis using import-graph tooling; track interface-addition velocity over next 3 months |

### 2026-07-28 — Activation-RefactoringCandidates

| Field | Value |
|-------|-------|
| **Agent** | cosca-evolution |
| **Task** | Identify top 3 refactoring candidates across the entire codebase |
| **Technique** | Impact-effort matrix — coupling depth × testability gap × user-facing surface area |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #evolution #refactoring #interfaces #god-files #api-coverage |
| **Related** | .opencode/cosca/memory/technical-debt/scorecard.md §3 Priorities |
| **Learned** | Top 3 refactoring targets: (1) **api/rest/handler/** — 19 concrete-coupled handlers with 0% tests, user-facing, highest blast radius; (2) **internal/cache/** — 583-line file, 0 interfaces, multi-level cache with SQLite/FS fallback but no mockable contract; (3) **internal/cli/** — 52 files, 19,550 LOC, 0 interfaces, coupling to nearly every internal package, hardest to test (currently 13.7% coverage). These three alone account for ~22,000 LOC with zero interface contracts. |
| **Next** | Profile import coupling chains: which packages have the highest fan-out and fan-in? |

