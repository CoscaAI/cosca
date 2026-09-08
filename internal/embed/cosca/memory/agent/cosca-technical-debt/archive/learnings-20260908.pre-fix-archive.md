# cosca-technical-debt - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

# cosca-technical-debt — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-technical-debt |
| **Task** | Initial capability establishment |
| **Technique** | Standard technical-debt patterns |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #technical-debt #baseline |
| **Learned** | Ready for Level 2. |
| **Next** | Identify first advanced technique |

## Level 2 — Real Task Execution

### 2026-07-28 — First Technical Debt Scorecard (v1.4.0-dev Baseline)

| Field | Value |
|-------|-------|
| **Agent** | cosca-technical-debt |
| **Task** | Produce first Technical Debt Scorecard for Cosca v1.4.0-dev — catalog all debt sources, classify, calculate composite score, prioritize top 10, recommend review cadence |
| **Technique** | Comprehensive multi-source debt audit: read bug registry (5 bugs), semantic index (17 gaps), agent memory directories (54 agents with learnings/failures/patterns line counts), test coverage report (coverage.md), risk registry (20 risks). Cross-referenced all sources to deduplicate. Applied severity-weighted scoring (Blocker=100, Critical=50, Major=20, Minor=5, Trivial=1) with age multiplier (1.0→1.5) and structural multiplier for process debt. |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #technical-debt #scorecard #quantification #baseline #structural-debt |
| **Related** | `internal/embed/cosca/memory/technical-debt/scorecard.md`, `bug/INDEX.md`, `semantic/INDEX.md`, `risk/RISK_REGISTRY.md`, `agent/cosca-runtime/learnings.md` |
| **Learned** | 75 debt items cataloged across 8 categories. Composite score: **1,375/3,200 (43.0%)** — moderate-high debt load. Debt is predominantly structural (Major tier = 51.3%), not code-level. The platform's codebase is healthy (5/5 bugs fixed, ~78% unit coverage, architecture well-documented), but the agent ecosystem (70% untrained), CI/CD pipeline (missing 5 critical gates), and test pyramid (0% E2E, 0% runtime integration) are significantly underinvested. Key insight: structural debt (process/cultural gaps) dominates — 14 of 75 items are structural with compounding interest. The highest-ROI fix is adding `-race` to CI (2h effort, score 65). The most critical fix is BUG-U01 (Restart() broken, 4h effort, score 130). |

### What Worked Well
- **Multi-source cross-referencing**: Reading bug INDEX, semantic INDEX, risk registry, coverage report, and agent memory line counts in parallel eliminated blind spots. Each source revealed debt the others didn't capture.
- **Severity-weighted scoring**: The Blocker/Critical/Major/Minor/Trivial scale with age multiplier produced intuitively correct rankings. The #1 priority (Restart() broken, 130) is clearly the most impactful item.
- **Agent memory line-count analysis**: Using `wc -l` on learnings/failures/patterns per agent provided a rapid, objective metric for "substantive vs. seed" — no need to read 54+ files individually.
- **Risk registry integration**: Items from RISK_REGISTRY.md mapped cleanly to technical debt categories, confirming the risk framework's technical grounding.

### What Was Difficult
- **Age estimation**: For latent structural debt (e.g., missing CI gates), exact introduction dates are unknown. Used a conservative 60-day estimate based on project timeline. Future scorecards should track discovery dates precisely.
- **Effort estimation**: Effort ranges from 1h (lint fix) to 80h (agent ecosystem activation). Estimates are ±50% and need calibration as tasks are actually executed.
- **Deduplication across sources**: The same gap (e.g., "no runtime integration tests") appeared in semantic index (GAP-02), test coverage (COV-02), and risk registry — had to cross-reference to avoid double-counting. Systematic canonical IDs (BUG-U01, GAP-01, COV-01) proved essential.
- **Semantic index inaccuracies**: GAP-06 references bug-005 as "slow dashboard query" but the actual bug-005 file is about SQLite migration panic. Flagged as a documentation debt item (DOC-01). Cross-source validation is necessary to trust any single source.
- **No prior baseline**: Without a previous scorecard, trend analysis is forward-looking projection only. Cannot measure improvement velocity yet.

### Primary Domain Confidence Suggestion: **0.50** (≥ 0.40 target, baseline was 0.25)

**Justification**:
- Completed first real task (Level 2) with comprehensive scope — 75 items cataloged across 8 categories, scoring model validated, actionable top-10 produced.
- Multi-source methodology worked correctly on first attempt, with no significant errors or omissions.
- Evidence: scorecard saved at `internal/embed/cosca/memory/technical-debt/scorecard.md`, cross-references 5+ source documents, scoring formula produces intuitive rankings.
- Adjustment from baseline (0.25): +0.25 for successful first real task execution with verified, comprehensive output. Reserved 0.50 headroom for future scorecards with trend data (true Level 3 requires ≥2 scorecards to demonstrate trend analysis capability).

| **Next** | Level 3: Produce second monthly scorecard (due 2026-08-28) with trend analysis comparing to this baseline. Track resolution of Top 10 priorities. Validate effort estimates against actuals. Add technical interest rate multiplier to scoring model. |

