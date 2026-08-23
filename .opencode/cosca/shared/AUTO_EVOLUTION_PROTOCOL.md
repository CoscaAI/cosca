# AUTO-EVOLUTION PROTOCOL

> All Cosca agents auto-evolve through experience. This protocol defines how agents learn from each task, record knowledge, and progress through capability levels.

## Before Any Task
Search your semantic memory at `.opencode/cosca/memory/agent/{agent-name}/learnings.md` for techniques matching the task domain. Apply the highest-level technique you have mastered — never repeat basic checks when advanced ones exist.

## After Completing a Task
Record what you learned in `learnings.md` using the Learning Entry Format defined at `.opencode/cosca/memory/LEARNING_PROTOCOL.md`. Include: timestamp, technique name, task context, level (1-5), outcome, tags for semantic search, what was learned, and what to try next.

### Stages 7-8: EXTRACT PATTERN + UPDATE CAPABILITY — measure, then promote (evidence-gated)

Stages 7 (extract pattern) and 8 (update capability model) of the metacognition pipeline are mandatory. **Registering** (record the learning, update the capability profile) is the baseline — but it is **not enough to promote a skill**. Recording + promoting must be backed by evidence:

- **Measure with the A/B meta-loop**: `cosca skill benchmark <skill_name>` runs the candidate (`with`) body against the baseline (`without`) body over the cases in `.cosca/evals/skills/<skill_name>.eval.yaml`.
- **Report robustly**: use pass-rate and **median + IQR**, never the mean. `cosca skill benchmark` emits these per arm plus a `candidate` flag (trials ≥ `MinTrials` and median/IQR separation).
- **Gate the promotion**: promote only if `candidate == true` **and** the regression gate passes (`cosca gate catalog --audit --strict` + `go test ./...`). The harness vetos a candidate whose gate verdict is `Passed == false`.
- **Ship as a PR, never auto-deploy**: promote via `evolve/<skill>-<timestamp>` (reviewed merge), not an automatic deploy. Inspect the evidence chain with `cosca skill history <skill_name>`.

Workflow: `.opencode/cosca/workflows/skill-evaluate.md`. Example def: `.cosca/evals/skills/adr-generation.eval.yaml`. Evidence lands in `.cosca/evals/skills/<skill>.benchmark.json` + `<skill>.history.json`. A skill body is promoted only when the data says so — opinion never promotes a skill.

## Capability Progression
| Level | Description | Milestone |
|-------|-------------|-----------|
| 1 | Basic patterns | First 10 tasks |
| 2 | Multi-layered techniques | Cross-domain application |
| 3 | Advanced / Threat modeling | Novel combinations |
| 4 | Novel techniques | Original contributions |
| 5 | Framework contributions | Community standards |

Goal: auto-evolve to Level 3+ within your first 10 tasks.
