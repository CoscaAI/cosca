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

### 2026-09-05 �?" Prova documental de capacidade honesta (verificacao por medicao)
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-documentation-writer |
| **Task** | Redigir docs/reports/cosca-capability-proof-2026-09-05.md (proof doc, PT-BR, sem inflacao) |
| **Technique** | (1) Antes de escrever, VERIFICAR cada numero no CLI real: in\cosca.exe version/capability level/doctor/status/knowledge stats/knowledge search/agent list/skill status/provider list/model list. (2) Benchmarks: rodar go test ./internal/vector/ -bench="Dot" -benchtime=2x -run=\"^$\" para obter o valor verificado HOJE (32.90 Mvec/s) e marcar claims historicas (52.66, 80.82) como [documentado], declarando a discrepancia. (3) Metodo de arbitragem P2: quando README/CHANGELOG/KERNEL/EVOLUTION discordam das contagens, o CLI e a autoridade; usar tabela de reconciliacao com valores adotados. (4) Marcar 3 classes: [medido]/[documentado]/[reconciliacao]. (5) Seccao de limitacoes lista pendencia e lacunas abertas, nunca omitindo. |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #proof-doc #honestidade #lei-da-familia #verificacao #benchmark #inventario #reconciliacao |
| **Related** | bin/cosca.exe; docs/gold-certification-2026-08-21.md; docs/reports/performance-int8-fastpath-2026-08-17.md; docs/COSCA_LEVELS.md; .opencode/cosca/CONSTITUTION.md; .opencode/cosca/KERNEL.md |
| **Learned** | (1) cosca self nao e comando CLI (nao existe; "did you mean perf"); estado dos orgaos vem do MCP server (cosca_cosca_self) - e um conceito distinto do daemon cosca serve PARADO. (2) Dispersao real de inventario: README badge 53/29/30; KERNEL v3.0.1 header 54 agents/43 skills; changelog 1.5.0 (Platform Overview) 40/43/20; CLI mede 61 agents/88 skills; disco 106/98/30. (3) cosca model list = "No models registered" (registro vazio) mesmo com a sessao rodando em deepseek-v4-flash-vision-exp. (4) knowledge search default usa escala de ranking 1.6-2.3, DIFERENTE da claim do changelog (0.72-0.75 semantico) - nao confundir. (5) KERNEL.md "12.000 linhas" e nota historica da revisao v2.0.0; o arquivo v3.0.1 atual = 1.499 linhas. (6) doctor varreu 217 packages vs README 233 vs GOLD 141 packages. (7) Performance tem variancia ~13% entre execucoes: reportar o valor medido no dia + faixa, e a claim historica separada. |
| **Next** | Ao escrever qualquer proof/capability doc futuro, tratar numeros conflitantes como fato a reconciliar (nao um a escolher), medir no CLI real, e marcar claims historicas abertamente. |
