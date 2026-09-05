# WORKFLOW: skill-evaluate

> **Version**: 1.0.0 | **Status**: active | **Owner**: Documentation Chief | **Category**: evolution | **Last Updated**: 2026-08-22

## OBJECTIVE

Run the A/B meta-loop for a skill (ADR-8101, incremento 1) and promote a candidate skill body **only with evidence** — never by opinion or auto-deploy. Measured via `cosca skill benchmark`, compared robustly (median + IQR, never mean), and gated by a regression gate before any promotion.

## INPUTS

| Name | Type | Required | Description |
|------|------|----------|-------------|
| skill_name | string | Yes | Name of the `.eval.yaml` to evaluate (e.g. `adr-generation`) |
| eval_def | file | Yes | `.cosca/evals/skills/<skill_name>.eval.yaml` — with/without bodies + cases |
| scorer | string | No | `static` (fixture, default) or `llm` (judge, later increment) |
| trials | int | No | Trials per arm (default `MinTrials` = 5; below this a candidate is never flagged) |

## OUTPUTS

| Name | Type | Description |
|------|------|-------------|
| benchmark | object | `<skill_name>.benchmark.json` — pass-rate, median/IQR per arm, delta, candidate flag, gate |
| history | object | `<skill_name>.history.json` — append-only rows (version, parent, pass-rate, best ✓) |
| candidate | boolean | Whether the A/B signal (plus gate) marks the arm as promotion-worthy |
| decision | enum | `promote` / `keep-baseline` / `inconclusive` (evidence-driven) |

## PRECONDITIONS

1. A `.eval.yaml` exists for the skill under `.cosca/evals/skills/`.
2. The `with` body is the candidate revision, the `without` body the baseline (empty `""` = degraded).
3. `cosca skill benchmark` runs against the real workspace (outside the cage) — authorized.

## POSTCONDITIONS

1. Benchmark snapshot + immutable history persisted (0600).
2. One of: promotion PR opened (`evolve/<skill>-<timestamp>`) OR baseline kept / inconclusive recorded.
3. Documentation updated to reflect the promoted skill body / the evidence used.

## DEPENDENCIES

- `internal/skilleval` — harness `RunAB`, robust summary (median+IQR, bootstrap std).
- `ci/gate` — `cosca gate catalog` (catalog invariants) for the regression gate.

## STEPS

### Step 0: Define / Refresh the Evaluation
- **Chief**: Documentation
- **Task**: Author or update `.cosca/evals/skills/<skill_name>.eval.yaml` — the candidate `with` body, the baseline `without` body, and 2+ real cases. Each case has `{id, task, rubric[], weight}`; `rubric` encodes `expected_behavior` (what the output **does**, never the exact text).
- **Output**: Valid `.eval.yaml` (verifiable via `cosca skill eval list`).

### Step 1: Dry-run (no side effects)
- **Chief**: Documentation
- **Tool**: `cosca skill benchmark <skill_name> --dry-run`
- **Task**: Outline the run — description, version, case count, trials/arm, scorer, body sizes — without grading or persisting anything.
- **Output**: Dry-run outline; confirms the harness reads the `with`/`without` bodies and cases correctly.

### Step 2: Run the A/B
- **Chief**: Documentation
- **Tool**: `cosca skill benchmark <skill_name>` (`--dry-run` absent; `--trials <N>` optional; default `MinTrials` = 5)
- **Task**: Execute `RunAB` — arm A (`with`) vs arm B (`without`) — and persist `<skill_name>.benchmark.json` + append-only `<skill_name>.history.json`.
- **Output**: Benchmark snapshot + history row. Use `--all` to sweep every `.eval.yaml`; `--no-gate` only for diagnostic (never for a promotion).

### Step 3: Compare With/Without (robust, not mean)
- **Chief**: Review
- **Task**: Read the candidate signal from the summary stats. **Use median + IQR, never the mean.** The candidate flag is true only when: `trials >= MinTrials`, `median(with) >= median(without)`, and the IQRs are non-overlapping or overlap ≤ 25% of the smaller IQR width. A large common IQR region is inconclusive — do not promote.
- **Output**: Verdict on A/B signal (`signal = candidate` / `signal = inconclusive`).

### Step 4: Regression Gate (promote only if it passes)
- **Chief**: Review / QA
- **Tool**: `cosca gate catalog --audit --strict` (invariants A–D, blocking) + `go test ./...`
- **Task**: Run the deterministic regression gate. `RunAB` vetos the candidate if the gate verdict is `Passed == false` (i.e. `IsCandidate = candidate && gate.Passed`). A candidate without a green gate is **not** promoted.
- **Output**: Gate verdict (catalog audit + regression + delta tolerance).

### Step 5: Decision — Promote Only on Evidence
- **Chief**: Documentation
- **Task**: Promote **only if** `signal = candidate` **and** the gate passed. Otherwise keep the baseline and record the run as `keep-baseline` / `inconclusive` (it still lands in history).
- **Rule**: never auto-deploy, never promote on a single thin sample, never promote with `--no-gate`.
- **Output**: Decision.

### Step 6: Open the Promotion PR (never auto-deploy)
- **Chief**: Documentation
- **Task**: When promoting, open a PR **`evolve/<skill>-<timestamp>`** carrying: the winning `with` body, the benchmark + history evidence (pass-rate, median/IQR), and the updated docs (README/changelog/ADR where the skill changed). Merge only after human / Review Chief approval — promotion is a reviewed, evidence-backed merge, not an automatic deploy.
- **Output**: `evolve/<skill>-<timestamp>` PR → reviewed merge → baseline advanced to the promoted body.

### Step 7: Cadence & Inspection
- **Chief**: Documentation
- **Task**: Re-run the meta-loop for the skill family weekly. Inspect with `cosca skill history <skill_name>` (append-only). Continue only when a candidate demonstrates evidence; the weekly cadence guards against regression.
- **Output**: Weekly evidence review; the next A/B starts from the new baseline.

## ARTIFACTS

| Artifact | Path | Purpose |
|----------|------|---------|
| Eval definition (source) | `.cosca/evals/skills/<skill_name>.eval.yaml` | The evaluable spec (with/without bodies + rubric cases) |
| Benchmark snapshot | `.cosca/evals/skills/<skill_name>.benchmark.json` | Latest run result (overwritten each run) |
| Immutable history | `.cosca/evals/skills/<skill_name>.history.json` | Append-only evidence chain (version, parent, pass-rate) |
| Promotion PR | `evolve/<skill>-<timestamp>` | Evidence-backed, reviewed merge (never auto-deploy) |

## GATE

| Gate | Command | Enforced? | Blocks promotion? |
|------|---------|-----------|-------------------|
| A/B signal | `cosca skill benchmark <skill_name>` | `trials >= MinTrials` + median/IQR separation | Yes (if not a candidate) |
| Catalog audit | `cosca gate catalog --audit --strict` | Invariants A–D (index, frontmatter, cross-refs, mojibake) | Yes |
| Regression | `go test ./...` | Test suite green | Yes |
| Veto | YAML `gate` block (catalog_audit/reg_tests/reg_delta) | `RunAB` sets `IsCandidate = candidate && gate.Passed` | Yes |

> **Promotion requires both**: a positive A/B **candidate** signal **and** a green regression gate. `--no-gate` is diagnostic only and never authorizes a promotion.

## VALIDATION

1. `cosca skill eval list` shows the skill.
2. `cosca skill benchmark <skill_name> --dry-run` reads `with`/`without` + cases.
3. `cosca skill benchmark <skill_name>` produces a valid snapshot + an append-only history row.
4. Candidate decisions rest on median/IQR (never the mean) and `trials >= MinTrials`.
5. A promotion is gated by `cosca gate catalog --audit --strict` + `go test ./...`.

## SUCCESS CRITERIA

- [ ] `.eval.yaml` exists with 2+ real cases and semantic `rubric` (expected_behavior, never exact text)
- [ ] Dry-run confirmed before any persisted run
- [ ] A/B compared with median + IQR, not the mean
- [ ] Candidate promoted **only** with a green regression gate and sufficient trials
- [ ] Promotion shipped as a reviewed PR `evolve/<skill>-<timestamp>`, never auto-deployed
- [ ] Evidence (benchmark + history) committed alongside the promoted body

## ERROR HANDLING

| Failure | Action |
|---------|--------|
| `.eval.yaml` not found | Ensure it exists at `.cosca/evals/skills/<name>.eval.yaml`; `skill eval list` to confirm |
| Invalid YAML / empty rubric | `cosca skill benchmark` fails to parse; fix the definition, re-run |
| Fewer trials than MinTrials | Bump `--trials`; below `MinTrials` a candidate is never flagged |
| Inconclusive IQR overlap | Keep baseline; record `inconclusive`; re-test next cadence |
| Gate fails | Do **not** promote; fix the catalog/regression debt; re-run gate then the A/B |
| Scorer error / runner failure | Degrades to a fail or neutral per arm and continues; no abort |

## RELATED

- [Meta-loop A/B (internal/skilleval)](../../internal/skilleval) — `RunAB`, robust summary, candidate + gate semantics
- [skill-eval CLI](../../internal/cli/skill_eval.go) — `cosca skill benchmark | history | eval`
- [Catalog gate](../QUALITY_GATES.md) — gate de catálogo `cosca gate catalog`
- [AUTO-EVOLUTION PROTOCOL](../shared/AUTO_EVOLUTION_PROTOCOL.md) — stages 7-8 evidence-based promotion
- [ADR-001](docs/adr/ADR-001-cosca-cli-architecture.md) — canonical ADR template the skill generates

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-08-22 | Documentation Chief | Initial workflow — A/B meta-loop with evidence-gated promotion (ADR-8101 incremento 1) |

## Gate de Regressao (Fase 3 - benchmarks-as-gates)

- **SplitEval 50/25/25** (treino/valida��o/holdout, deterministico, anti-overfit)
- **RegressionGate**: avalia baseline vs candidato **so no holdout**; 	hreshold 0.02; candidato que melhora no treino mas regride >2% no holdout => REJEITADO
- **So promove se**: guardrails ok E RegressionGate.Passed E A/B IsCandidate (PR evolve/<skill>-<ts>, nunca auto-deploy)
