# AUTO-EVOLUTION PROTOCOL

> All Cosca agents auto-evolve through experience. This protocol defines how agents learn from each task, record knowledge, and progress through capability levels.

## Before Any Task
Search your semantic memory EFFICIENTLY — learnings.md files are LARGE (up to 200KB) and cost tokens. NEVER read a learnings.md (or archive/) file in full. Prefer, in order:
1. `cosca memory search` / semantic search for the technique; or
2. read the agent `INDEX.md`; or
3. `grep` the learnings for matching tags/terms (read at most ~40 matching lines); or
4. `tail` only the most recent entries when no index exists.

Apply the highest-level technique you have mastered — never repeat basic checks when advanced ones exist.

## After Completing a Task
Record the learning as a CHAIN-TRACKED BLOCK via the register command (NOT by hand-editing learnings.md). Run, per agent:

```bash
cosca memory register --agent {agent-name} --title "..." --level 3 --tags "#a #b" --task "..." --technique "..." --outcome success --learned "..." --next "..."
```

The command creates the immutable block (`blocks/{sha256}.md`), appends the 1-line trigger to `learnings.md`, updates `chain.dat` and regenerates the Merkle root. Protocol: `.cosca/memory/LEARNING_PROTOCOL.md` (v3.0.0). Never write learning content directly into learnings.md — it is an index of triggers, not a journal.

### Stages 7-8: EXTRACT PATTERN + UPDATE CAPABILITY — measure, then promote (evidence-gated)

Stages 7 (extract pattern) and 8 (update capability model) of the metacognition pipeline are mandatory. **Registering** (record the learning, update the capability profile) is the baseline — but it is **not enough to promote a skill**. Recording + promoting must be backed by evidence:

- **Measure with the A/B meta-loop**: `cosca skill benchmark <skill_name>` runs the candidate (`with`) body against the baseline (`without`) body over the cases in `.cosca/evals/skills/<skill_name>.eval.yaml`.
- **Report robustly**: use pass-rate and **median + IQR**, never the mean. `cosca skill benchmark` emits these per arm plus a `candidate` flag (trials ≥ `MinTrials` and median/IQR separation).
- **Gate the promotion**: promote only if `candidate == true` **and** the regression gate passes (`cosca gate catalog --audit --strict` + `go test ./...`). The harness vetos a candidate whose gate verdict is `Passed == false`.
- **Ship as a PR, never auto-deploy**: promote via `evolve/<skill>-<timestamp>` (reviewed merge), not an automatic deploy. Inspect the evidence chain with `cosca skill history <skill_name>`.

Workflow: `.cosca/workflows/skill-evaluate.md`. Example def: `.cosca/evals/skills/adr-generation.eval.yaml`. Evidence lands in `.cosca/evals/skills/<skill>.benchmark.json` + `<skill>.history.json`. A skill body is promoted only when the data says so — opinion never promotes a skill.

## Capability Progression
| Level | Description | Milestone |
|-------|-------------|-----------|
| 1 | Basic patterns | First 10 tasks |
| 2 | Multi-layered techniques | Cross-domain application |
| 3 | Advanced / Threat modeling | Novel combinations |
| 4 | Novel techniques | Original contributions |
| 5 | Framework contributions | Community standards |

Goal: auto-evolve to Level 3+ within your first 10 tasks.
