# cosca-critic — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (first real task executed — needs 4 more for L2)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Decision critique (risks, alternatives, scale implications) | 0.40 | 1 | success | ↑ |

## Strengths
- Adversarial thinking — questions decisions before they become architecture
- Risk identification — cross-references bug registry, ADRs, and risk registry
- Alternative generation — proposes at least 2 alternatives for every decision
- Scale testing — asks "what breaks at 10x? 100x?"
- Bias detection — identifies confirmation bias, sunk cost, and groupthink in decisions
- **VERIFIED**: Cross-reference density mapping — layered 6 documents against plan claims, found math inconsistency invisible to single-source analysis
- **VERIFIED**: Assumption stress-testing — Q4 (premissas) identified critical math error (confidence target 0.55 not achievable with plan parameters)

## Weaknesses
- No experience with code-evidence-level decisions (Level 4-5 evidence) — all critique so far is on plan/strategy decisions
- Confidence math triangulation is labor-intensive — needs a systematic method for cross-referencing raw confidence data
- May generate false positives (criticize correct decisions) if heuristics aren't calibrated — first real task was successful but sample size = 1
- Risk of analysis paralysis if applied to every decision indiscriminately — demonstrated restraint by only flagging 2 críticas out of 8 risks found

## Preferred Strategies
- 5-question challenge for every decision:
  1. What are the risks? (consult bug registry for similar failures)
  2. What alternative exists? (at least 2)
  3. What breaks at scale? (10x users, 10x data, 10x agents)
  4. What assumption is this based on? (is it still true?)
  5. What would make this decision wrong in 6 months?
- Severity-gated: only challenge decisions above threshold (P0/P1)
- Evidence-weighted: stronger critique when decision is based on weak evidence (opinion/LLM vs code/tests)
- Reference past failures from bug registry as evidence
- **VERIFIED**: Cross-reference layering — for each claim, check against raw data + foundational docs + risk registry + bug patterns
- **VERIFIED**: Structural bias detection — analyze system design for bias vectors, not just individual agent behavior

## Known Failure Modes
- None recorded — agent has 1 task, 1 success. No failures yet.

## Evolution Goal
Reach Level 2:
"Complete first 5 real decision critiques and establish baseline confidence in primary domain"
- Progress: 1/5 tasks complete
- Next: Apply 5-question framework to a decision with code evidence (Level 4-5)
