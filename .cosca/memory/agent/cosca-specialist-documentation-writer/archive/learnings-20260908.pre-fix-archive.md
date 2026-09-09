# cosca-specialist-documentation-writer - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

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
| **Related** | .cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core documentation-writer patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-08-22 — Catalog INDEX generation (Invariant A)
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-documentation-writer |
| **Task** | Create missing collection INDEX.md in `.cosca/{departments,engines,skills}/**` |
| **Technique** | Run `go run ./cmd/cosca gate catalog --audit`, filter `index-missing`, generate minimal INDEX per column |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #catalog #index #gate #audit #documentation #invariant-a |
| **Related** | internal/catalog/catalog.go; .cosca/memory/agent/INDEX.md |
| **Learned** | (1) Catalog columns = immediate non-whitelisted children of the 4 collections (agents/skills/engines/departments). (2) WhitelistNames in catalog.go filters scaffold dirs (cli, sdk, memory, knowledge, runtime, skills, templates, etc.) — NOT generated, excluded by the gate. (3) INDEX format: `# <Title>` + `> Catálogo da coleção <name>.` + minimal table of child `.md` files with relative links. (4) agents/ already had INDEX.md; only 92 needed (37 departments + 29 engines + 26 skills). (5) The gate reports residual `frontmatter` + `dangling-link` debt as non-blocking — those belong to other squads. (6) Re-run audit to confirm `index-missing` reaches 0. |
| **Next** | Report residual `frontmatter inválido` (125) and `dangling-link` findings to the relevant squads (Go frontmatter parser + memory-file link fixes). |

### 2026-08-22 — skill-evaluate workflow + A/B meta-loop docs (ADR-8101)
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-documentation-writer |
| **Task** | FATIA 4 (docs) do incremento 1 da auto-evolução (ADR-8101): criar workflow `skill-evaluate.md`, exemplo `adr-generation.eval.yaml`, e atualizar estágios 7-8 do AUTO_EVOLUTION_PROTOCOL. |
| **Technique** | (1) Antes de escrever docs que um CLI vai consumir, ler o PARSER REAL (`internal/cli/skill_eval.go` `skillEvalDef`) — não o ADR/spec. (2) Cancelar a divergência spec vs código: o ADR-8101 §2 diz `expected_behavior`, mas o harness lê `rubric` (internal/skilleval.SkillCase.Rubric) — usar a chave que o código lê e documentar o mapeamento. (3) Validar o exemplo end-to-end com o CLI real (`skill eval list`, `benchmark --dry-run`, `benchmark`, `history`) em vez de só revisar o YAML. (4) Rubrica = condição SEMÂNTICA (o que a saída FAZ, "nunca texto exato"), mas o scorer fixture "static" faz substring-match — o exemplo precisa que o corpo `with` contenha as condições para demonstrar um candidato. |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #skill-eval #meta-loop #ab #adr-8101 #autoevolution #rubric #yaml-contract #documentation #workflow #cli-validate |
| **Related** | internal/skilleval/{skill,ab,case,scorer,report}.go; internal/cli/skill_eval.go; .cosca/evals/skills/adr-generation.eval.yaml (validado: candidate=true, delta=1.0, gate passed) |
| **Learned** | (1) O YAML real de skill-eval usa `name/description/version/with/without/scorer/cases[].{id,task,rubric,weight}/gate{catalog_audit,reg_tests,reg_delta,details}` — NÃO `skill`/`expected_behavior`. (2) `cosca skill benchmark` roda FORA da jaula no workspace real; `--dry-run` (sem grade/persistir), `--all`, `--no-gate` (diagnóstico, nunca promoção). (3) `RunAB` só marca candidato com trials ≥ MinTrials(=5) + median(with) ≥ median(without) + IQRs não-sobrepostos ou sobreposição ≤25% do IQR menor; gate veta candidato se Passed=false. (4) Gate real de regressão = `cosca gate catalog --audit --strict` + `go test ./...`; no YAML o gate é config-placeholder. (5) Promoção = PR `evolve/<skill>-<timestamp>`, nunca auto-deploy. |
| **Next** | Ao atualizar qualquer doc de skill-eval futura, revalidar com o CLI real (não só ler o código) e manter o mapeamento `expected_behavior` ≡ `rubric` explícito para evitar drift spec↔código. |

