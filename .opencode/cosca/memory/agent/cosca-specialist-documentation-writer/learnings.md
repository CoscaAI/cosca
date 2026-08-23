# cosca-specialist-documentation-writer — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-documentation-writer |
| **Task** | Initial capability establishment |
| **Technique** | Standard documentation-writer patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #documentation-writer #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core documentation-writer patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-08-22 — Catalog INDEX generation (Invariant A)
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-documentation-writer |
| **Task** | Create missing collection INDEX.md in `.opencode/cosca/{departments,engines,skills}/**` |
| **Technique** | Run `go run ./cmd/cosca gate catalog --audit`, filter `index-missing`, generate minimal INDEX per column |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #catalog #index #gate #audit #documentation #invariant-a |
| **Related** | internal/catalog/catalog.go; .opencode/cosca/memory/agent/INDEX.md |
| **Learned** | (1) Catalog columns = immediate non-whitelisted children of the 4 collections (agents/skills/engines/departments). (2) WhitelistNames in catalog.go filters scaffold dirs (cli, sdk, memory, knowledge, runtime, skills, templates, etc.) — NOT generated, excluded by the gate. (3) INDEX format: `# <Title>` + `> Catálogo da coleção <name>.` + minimal table of child `.md` files with relative links. (4) agents/ already had INDEX.md; only 92 needed (37 departments + 29 engines + 26 skills). (5) The gate reports residual `frontmatter` + `dangling-link` debt as non-blocking — those belong to other squads. (6) Re-run audit to confirm `index-missing` reaches 0. |
| **Next** | Report residual `frontmatter inválido` (125) and `dangling-link` findings to the relevant squads (Go frontmatter parser + memory-file link fixes). |
