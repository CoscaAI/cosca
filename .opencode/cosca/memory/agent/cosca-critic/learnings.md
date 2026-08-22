# cosca-critic — Semantic Learnings

> **Agent**: cosca-critic | **Type**: decision-critique | **DNA**: v3.0
>
> **Level**: 1 (seed → first real task) | **Confidence**: 0.40 | **Last Task**: 2026-07-28

---

## L1-SEED-001 — Decision Critique Framework (2026-07-28)

### Technique
5-Question Adversarial Challenge

### Context
Agent initialization. The cosca-critic role is to question decisions before they become locked-in architecture. Seed learning defines the baseline framework for evaluating any decision.

### Level
1 — Basic framework (seed data, no real execution)

### Outcome
seed

### Tags
#decision-critique #adversarial #risk-assessment #seed

### What Was Learned
The 5-question framework establishes a minimum bar for decision quality:
1. **Risks** — what can go wrong? (cross-reference bug registry)
2. **Alternatives** — what else could work? (minimum 2 alternatives)
3. **Scale** — what breaks at 10x load? (database, network, agent count)
4. **Assumptions** — what are we taking for granted? (are assumptions still valid?)
5. **Future-proof** — what would invalidate this decision in 6 months?

Decisions should NOT be critiqued indiscriminately — only P0/P1 decisions. Lower priority decisions get a lighter touch to avoid analysis paralysis.

### What to Try Next
- Apply the 5-question framework to a real decision
- Cross-reference bug registry for similar failure patterns
- Test whether critique leads to better outcomes (measure: decisions changed after critique)

---

## TASK-001 — Onda 2 Plan Review (2026-07-28)

| Field | Value |
|-------|-------|
| **Agent** | cosca-critic |
| **Task** | Aplicar o 5-Question Challenge ao plano de ativação Onda 2 da plataforma Cosca |
| **Technique** | Level 1 — Full 5-Question framework application with cross-reference analysis |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #decision-critique #onda-2 #agent-activation #risk-assessment #confidence-model #adversarial-review |
| **Related** | onda-2-plan.md, RISK_REGISTRY.md, bug/INDEX.md, cognitive-state.md, semantic/INDEX.md, LEARNING_PROTOCOL.md, ADR-005, ADR-006 |
| **Learned** | 1) **Cross-reference is the critic's superpower**: The most impactful finding (confidence math doesn't close) came from cross-referencing the plan's target (0.55) against the Semantic Index's raw confidence data. Without cross-reference, this would be invisible. 2) **Foundational document inconsistency is a critical risk vector**: LEARNING_PROTOCOL (+0.05/task) vs plano Onda 2 (1 task → 0.40) are incompatible. When foundational documents contradict each other, all decisions built on them are suspect. 3) **Confirmation bias is structural in sequential waves — not accidental**: The plan's 3-phase design (define → apply → review) creates a validation pipeline with no external checkpoint. The critic must flag structural bias, not just individual decision bias. 4) **Bug registry staleness creates a hidden single-point-of-failure**: All 5 bugs marked "✅ Fixed" but Semantic Index describes 3 as open. If agents trust the bug registry, they make decisions on wrong assumptions. The critic's role includes detecting stale/contradictory data sources. 5) **"Confidence theatre" is the dominant long-term risk**: The plan's architecture incentivizes reporting success over actual improvement. The metric (confidence ≥ 0.40) is gamed if the gate is "1 task completed" without quality validation. |
| **Next** | Level 2: Apply 5-question framework to a decision WITH code evidence (Level 4-5 evidence for stronger baseline). Measure: did the critique change the decision? Also: develop a cross-reference checklist template for future reviews. |

### Technique Evolution

- **Cross-reference density mapping**: The most effective technique in this review was layering multiple data sources against each claim in the plan. For each claim ("confiança sobe para 0.55"), cross-reference against: (a) raw data (Semantic Index), (b) foundational docs (LEARNING_PROTOCOL), (c) risk registry (R2), (d) bug patterns (bug-003). This 4-way cross-reference revealed gaps invisible to single-source analysis.
- **Assumption stress-testing**: Q4 (premissas) proved highest-yield — finding that the math doesn't close was the single most impactful result. Future critiques should allocate disproportionate time to assumption validation.
- **Structural bias detection**: Q5 (6-month failure scenarios) revealed that the plan's structure (sequential waves) creates confirmation bias regardless of agent intent. Structural bias is harder to detect than individual bias — requires analyzing the system design, not just agent behavior.

### Meta-Learning (Critic's Self-Critique)

What I did well:
- Cross-referenced 6 documents to find inconsistencies
- Identified a critical math error that no other agent flagged
- Provided 4 concrete alternatives with explicit trade-offs
- Proposed 6 actionable conditions (not just criticism)

What was difficult:
- Confidence math triangulation: the Semantic Index reports 0.48 platform confidence but individual agent data doesn't add up to 0.48. This required inferring that partial confidences (cosca-performance: 0.70 in subdomains) contribute to the average — but the methodology isn't documented.
- Distinguishing "real risk" from "theoretical risk": many things COULD go wrong. Prioritizing which risks are likely and consequential (vs possible but improbable) was the hardest judgment call.
- Balancing adversarial rigor with constructive recommendation: the critic must find problems without paralyzing the decision. The 6 conditions strike this balance — 2 críticas (must-fix), 4 important but non-blocking.

What I'd do differently:
- Request access to the raw confidence calculation data before the review (would have saved triangulation time)
- Interview the CEO about the confidence math methodology (was it calculated or estimated?)
- Check if the bug registry fix commits (c30fac3, 9a950ff, f3dbdc2, 03860c2) actually resolved the bugs or just marked them fixed
