# cosca-critic - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

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

---

## TASK-002 — Benchmark Design Audit (FULL-SCAN vs ROTEADO) (2026-08-24)

| Field | Value |
|-------|-------|
| **Agent** | cosca-critic |
| **Task** | Auditar adversarialmente o design do benchmark vectoragg (docs/reports/vectoragg-benchmark-design-2026-08-24.md) antes de executar. Papel: quebrar o experimento — garantir que mede o que alega e que o ground-truth não é inventado. |
| **Technique** | Level 2 — Cross-reference design-vs-code-vs-corpus: tese do design, implementação real, e dados reais indexados (3-vias). |
| **Level** | 2 |
| **Outcome** | success (identifiquei falha estrutural ANTES de existir número bonito) |
| **Tags** | #benchmark-audit #vectoragg #ground-truth #recall #modlink #routing #adversarial #design-review |
| **Related** | vectoragg-benchmark-design-2026-08-24.md, internal/search/scope.go, internal/search/search.go, internal/modlink/modlink.go, internal/vector/vector.go, internal/vector/sqlite_vec.go, .cosca/knowledge.db, ADR-013 |
| **Learned** | 1) **Ground-truth honesty is not a checkbox — it's a queryable invariant.** A design that says "cada query terá a resposta-relevante conhecida" is empty; the professor demanded per-query pre-registration of (a)-(e). Two independent verifications are mandatory: (i) the ground-truth doc actually EXISTS in the indexed corpus (verified, not assumed), and (ii) the domain module has ≥1 indexed item (path segment OR entity_type). 2) **Routing domains must exist in the corpus.** The proposed modules vector/unreal/world/vegetation/materials have **0 docs, 0 chunks** in knowledge.db (verified via read-only SQL); queries 4,5,6,7 therefore route to empty scopes → routed=empty → the "recall 0% / router destroys evidence" conclusion is an artifact of a misconfigured route, NOT an architectural finding. 3) **Route→module mapping must follow the evidence, not the question.** Query 4 (cosine ranking) ground-truth lives in docs/knowledge/search.md + docs/architecture/PIPELINE_TECNICO.md → route to "vector" drops it. Map module = the path segment where the evidence actually lives. 4) **Scope alone does NOT reduce work.** confineToScope (search.go L283-285) filters the RESULTS after the vector phase. Only CandidateIDs (vectoragg vocabulary) + scopeRouted triggers the bounded scan (ScannedVectors < TotalVectors). Measure ScannedVectors/MetadataCandidates from vector.SearchMetrics, never the post-confine result count (false-positive reduction). 5) **Default store uses in-memory int8 index** (NewSQLiteVec indexEnabled=true) → full-scan baseline decodes 0 BLOBs (ScannedVectors=TotalVectors, UsedFloat32=false). The "28.888 BLOBs decodificados" narrative only holds on the SQL-degrade path. Use ScannedVectors as the comparable metric. 6) **Guard against false-positive**: assert scopeRouted=true AND NoRoute=false AND ScannedVectors<TotalVectors for every routed query; else mark test INVALID, not "pass". |
| **Next** | Level 3: checar, antes de um re-audit, se o relatório de PROVENANCE final do benchmark registra o ground-truth pré-registrado com ID/domínio/rota esperada; verificar se o benchmark mediu ScannedVectors (não resultado pós-confine) e se declarou o caminho de index (in-memory int8 vs SQL BLOB). |

## TASK-003 — Re-audit Benchmark Design v2 (FULL-SCAN vs ROTEADO) (2026-08-24)

| Field | Value |
|-------|-------|
| **Agent** | cosca-critic |
| **Task** | Re-auditar adversarialmente o design v2 do benchmark (docs/reports/vectoragg-benchmark-design-2026-08-24.md) depois que a v1 foi reprovada e corrigida. Papel: tentar quebrar de novo, buscando falhas NOVAS ou residuais. |
| **Technique** | Level 3 — Cross-reference design-vs-code: validar cada correção v2 contra o caminho real (SearchMetrics/scopeRouted/resolveRouteCandidates/scanCandidates) e testar a regra de validade contra cenários de "passar de mentira". |
| **Level** | 3 |
| **Outcome** | success (encontrei uma lacuna de validade que a v2 NÃO fechou — candidato não-ser-exaustivo passa na regra) |
| **Tags** | #benchmark-audit #vectoragg #validity-rule #candidate-set #false-positive #adversarial #re-audit #SearchMetrics |
| **Related** | vectoragg-benchmark-design-2026-08-24.md, internal/search/search.go, internal/search/scope.go, internal/vector/vector.go, internal/vector/sqlite_vec.go, internal/modlink/modlink.go |
| **Learned** | 1) **As correções v2 são majoritariamente CORRETAS e verificáveis**: (a) SearchMetrics em vector.go L142-150 expõe ScannedVectors; SearchWithMetrics (sqlite_vec.go L308-380) PREENCHE ScannedVectors=len(rows) no caminho candidato via scanCandidates (L457-526 lê SÓ os candidateIDs + recent pool); search.go L429 chama SearchWithMetrics com candidateIDs → a métrica de trabalho REAL é honesta (PASS). (b) scopeRouted (scope.go L48) = scope!=nil && !NoRoute && len(Modules)>0 — correta e observável. (c) confineToScope roda em search.go L283-285 DEPOIS da fase vetorial (L406+), confirmando o DIFF v1 (contagem pós-confine enganosa). (d) Queries v2 só em módulos com conteúdo; cosseno→knowledge está certo (docs/knowledge/search.md L10 fala de "cosine similarity" — path segment "knowledge"; vector-index.md é FILENAME, não segmento=="vector", por isso vector=0). 2) **A LACUNA que a v2 NÃO fechou**: a regra de validade (§2.1) NÃO exige que o `CandidateIDs` do caminho roteado seja o VOCABULÁRIO EXAUSTIVO do módulo. `scopeRouted=true` só exige Scope com Modules; `ScannedVectors<TotalVectors` só exige que o set seja MENOR que o total. Logo um set NÃO-exaustivo — (i) derivado de FTS (candidatos lexicais query-dependentes → redução é do LÉXICO, não do router, e passa na validade) ou (ii) subset de mão (ex.: só os chunks do ground-truth → recall 100% trivial + ScannedVectors minúsculo) — PASSARIA na regra e viraria falso-positivo de "router preserva recall com ~0 trabalho". 3) **Mecanismo de derivação não especificado**: não há helper (grep de IDsForModule/ModuleVectors/IDsByScope → 0) que enumere vector-IDs por módulo/path. O design manda "passar CandidateIDs" mas não define o JOIN (documents.path → document_id → vector.id); o gate de pré-execução conta DOCUMENTOS por path-segment, não VETORES/CHUNKS — então |candidatos do módulo| ≠ o que o gate mede. 4) **Confinde méntrico**: o caminho roteado confina a fase vetorial aos candidatos MAS a fase FTS roda sem escopo (search.go L229+), só sendo pós-cortada por confineToScope — a "redução de trabalho" é só do vetorial; a narrativa "99,9% do trabalho" precisa ser escopada. 5) Query ambígua (memory+runtime) só exercita "amplia sem estreitar" SE o resolver devolver BOTH módulos para a query exata (whole-word); requer rotas test-only cujos triggers batam na query inteira — pre-verificar via Resolve na tabela. Próximo: **antes de executar, validar com um teste que ASSERTE len(CandidateIDs)==COUNT(vetores do módulo) e que CandidateIDs==vocabulário do escopo (não FTS-hits)**, senão a tabela de métricas pode reportar otimização que não veio do router. |
| **Next** | Level 4: se for feita a correção v3, re-auditar a regra de validade com um CASO CONCRETO (query real + rotas test-only + enumeração do vocabulário do módulo) e checar que o run reporta |CandidateIDs| == COUNT(vectors) do módulo para CADA query antes de entrar na tabela principal. |

