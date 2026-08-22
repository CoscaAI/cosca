# COSCA — Auditoria Quantitativa (2026-08-14)

> Modo 100% leitura. Medido no código real. Nada estimado quando mensurável. Rótulos: [MEASURED] / [DERIVED] / [UNKNOWN].
> Padrão de cobertura da casa: **70%** (Makefile `coverage-check` falha se total < 70%).

## 1. Produção por linguagem (sem testes, sem markdown, sem generated)

| Linguagem | Arquivos | LOC reais | Branco | Comentário | % coment |
|-----------|---------:|----------:|-------:|-----------:|---------:|
| **Go (manual)** | 594 | **131.495** | 45.743 | 37.425 | 10,6% |
| TypeScript (manual) | 15 | 1.020 | 587 | 1.096 | — |
| Python | 2 | 577 | 57 | 22 | — |
| Shell | 2 | 285 | 45 | 49 | — |
| Proto | 3 | 142 | 24 | 0 | — |
| SQL | 2 | 9 | 23 | 120 | — |
| JavaScript | 0 | 0 | — | — | (todo vendored) |
| **TOTAL produção manual** | **618** | **133.528** | | | |

Go total (prod+test): 600+416 arquivos, 271.666 LOC code, 46.704 blank, 37.738 comment (76,3%/13,1%/10,6%).

## 2. Testes

| Grupo | Arquivos | LOC |
|-------|---------:|----:|
| Go unit/integration (`*_test.go`) | 416 | 137.834 |
| TypeScript (vitest, `__tests__`) | 12 | 1.841 |
| — integração (`test/integration`, tag `integration`) | 7 | 1.667 |
| — soak (`test/soak`, tag `soak`) | 1 | 160 |
| E2E (Playwright) | 0 | 0 |

- **Fuzz/propriedade**: 0 funções Fuzz
- **Concorrente**: 111 arquivos usam `t.Parallel()`; 5 `TestMain`
- **-race**: habilitado no Makefile (`test-unit`, `test-integration`, `test-race`, `coverage-check`) e no CI (ci.yml); 71 arquivos citam race
- **E2E**: DECLARADO no Makefile (`test-e2e` → `pnpm --dir web test:e2e`), **mas `web/` não existe** e não há Playwright → **não implementado**
- **Flaky sob carga** [MEASURED]: `TestTaskQueueDeduplicates` (internal/pipeline) e teste em `pkg/cosca` falham na suíte completa, passam isolados

**PRODUÇÃO: 618 arquivos / 133.528 LOC — TESTES: 428 arquivos / 139.675 LOC — RATIO: 1,046**

## 3. Cobertura [MEASURED — executada nesta auditoria]

- **Agregado ponderado (statements): 62,9%** — abaixo do padrão de 70%
- 122 pacotes ok, 3 falham na suíte completa (pipeline, pkg/cosca flaky; `internal/sandbox` sem setup no ambiente)
- **41 pacotes abaixo de 70%**
- Sem relatório de coverage versionado (`build/` no .gitignore)

Forte (≥80%): context 98,6, supervisor 97,3, runtime 96,6, agents 96,3, capability 95,5, embeddings 94,9, telemetry 94,7, chat 91,3, search 87,0, vector 87,6, evals 87,4, contenttrust 86,7, ledger 85,9, engine 84,3, circadian 84,3, execpolicy 83,8, proposal 83,1, diagnostics 83,0, workflow 82,8, gate 82,5, kernel 82,2, config 82,2, sessionindex 81,3, memory 80,8, secrets 79,5, scheduler 79,7, audit 79,2, contracts 78,2, orchestration 77,5, sciengine 76,9, skills 75,3, providers 74,7, knowledge 73,4, sqlite 71,8, auth 70,5.

Fraco (<70%): **pipeline 18,7** (9.786 LOC prod!), chat/ui/theme 7,0, chat/ui/terminal 35,7, chat/tool 30,6, cmd/cosca 22,7, adapter 24,4, grpcclient 36,7, updater 42,4, **security 42,7**, **indexer 43,1**, templates 45,7, generic_mcp 47,7, windsurf 47,9, **discovery 48,8**, zed 49,2, cmd/cosca-chat 50,1, chat/provider 51,3, bootstrap 52,3, editors 52,3, watcher 53,3, opencode 53,6, cursor 53,8, api/stream 56,1, chat/config 56,1, **plugins 57,6**, claude 58,2, **graph 58,3**, **integrity 59,7**, **cli 59,8** (27.425 LOC!), trace 61,0, media 61,8, api/rest 64,3, prompts 65,1, api/auth 67,1, metrics 67,1, durable 67,2, memoryintegrity 67,5, vscode 68,7, codex 69,3, api/rest/handler 69,7, chat/ui 69,9.

**21 pacotes SEM nenhum teste** (5.498 LOC; 3.161 manuais): providers bedrock/azure/ollama/google/groq/mistral (1.263), benchmark (393), cmd/cosca-indexer/acquireall/merkle/check (734), sessionstatus (189), pkg/engine (175), embed (176), safeerror/safe (127), tools (43), env (33) — +2.337 gerado (pb).

## 4. Testes vs Produção por área

| Área | Prod LOC | Test LOC | Ratio | Race | Integração | E2E | Status |
|------|---------:|---------:|------:|:----:|:----:|:----:|--------|
| kernel | 1.048 | 944 | 0,90 | sim | — | — | 82,2% ok |
| engine | 2.504 | 4.717 | 1,88 | sim | — | — | 84,3% ok |
| context | 1.377 | 2.089 | 1,52 | sim | — | — | 98,6% ok |
| memory | 1.914 | 3.697 | 1,93 | sim | sim | — | 80,8% ok |
| knowledge | 4.745 | 6.517 | 1,37 | sim | sim | — | 73,4% ok |
| search | 832 | 2.466 | 2,96 | sim | sim | — | 87,0% ok |
| embeddings | 632 | 849 | 1,34 | sim | — | — | 94,9% ok |
| vector | 423 | 1.141 | 2,70 | sim | — | — | 87,6% ok |
| pipeline | 9.786 | 1.768 | 0,18 | sim | — | — | **18,7% fraco + flaky** |
| orchestration | 4.799 | 6.494 | 1,35 | sim | — | — | 77,5% ok |
| runtime | 2.201 | 6.701 | 3,04 | sim | sim | — | 96,6% ok |
| provenance | 173 | 115 | 0,66 | sim | — | — | 71,9% ok |
| integrity | 1.154 | 566 | 0,49 | sim | — | — | 59,7% fraco |
| security | 204 | 100 | 0,49 | sim | — | — | 42,7% fraco |
| execpolicy | 253 | 285 | 1,13 | sim | — | — | 83,8% ok |
| sandbox | 426 | 740 | 1,74 | sim | — | — | setup falha |
| secrets | 284 | 447 | 1,57 | sim | — | — | 79,5% ok |
| auth | 856 | 1.415 | 1,65 | sim | — | — | 70,5% ok |
| audit | 474 | 785 | 1,66 | sim | — | — | 79,2% ok |
| trace | 481 | 474 | 0,99 | sim | — | — | 61,0% fraco |
| ledger | 638 | 395 | 0,62 | sim | — | — | 85,9% ok |
| session (sessionindex) | 297 | 275 | 0,93 | sim | — | — | 81,3% ok |
| scheduler | 921 | 635 | 0,69 | sim | — | — | 79,7% ok |
| circadian | 1.076 | 973 | 0,90 | sim | — | — | 84,3% ok |
| proposal | 914 | 1.081 | 1,18 | sim | — | — | 83,1% ok |
| gate | 367 | 397 | 1,08 | sim | — | — | 82,5% ok |
| sciengine | 147 | 108 | 0,73 | sim | — | — | 76,9% ok |
| providers | 1.365 | 812 | 0,59 | sim | — | — | 51,3% fraco |
| cli | 27.425 | 22.618 | 0,82 | sim | — | — | 59,8% fraco |
| chat (core) | 814 | 1.980 | 2,43 | sim | — | — | 91,3% ok |
| chat/ui+tool+provider | 7.109 | 2.340 | 0,33 | sim | — | — | 7–51% fraco |
| api/rest+handler+stream | 6.102 | 9.813 | 1,61 | sim | — | — | 56–70% parcial |
| pkg/cosca | 3.410 | 5.988 | 1,76 | sim | — | — | 75,5% ok |
| test/integration | 0 | 1.667 | — | sim | sim | — | 7 áreas |
| test/soak | 0 | 160 | — | sim | — | — | 1 área |

## 5. Implementação recente

- **Git: 156 commits, TODOS entre 12 e 14/08** — baseline a321eb5 = 2.376 arquivos (re-âncora pós-incidente). [MEASURED] Não é possível derivar "antigo vs recente" do git; a árvore é uma re-âncora de código com linhagem mais antiga (memória desde 28/07).
- **Mtimes**: 531 Go em 08-12 (baseline), 46 em 08-13, 23 em 08-14.
- **Últimos 12 commits**: 100% memória/documentação (L300-L309, ADRs, doutrina).
- **Áreas mais tocadas pós-baseline**: cli (217), chat (92), pipeline (61), knowledge (35), evals (31), orchestration (28), runtime (26), engine (26), editors (25), providers (22), memory (19).
- **Correlação recência × cobertura**: as áreas mais ativas com MENOR cobertura são pipeline (18,7%), cli (59,8%) e a camada UI (chat/ui 7-70%, editors 47-69%) — coincidem com as recentemente ativas. O core de execução (kernel/engine/orchestration/runtime) foi bem coberto. Hipótese suportada com nuance: a maior dívida de cobertura está na camada de interface (UI/editors/pipeline), não no core.

## 6. Código comentado

- Doc comments (// + Capital): 18.598 linhas [MEASURED]
- Comentários Go totais: 37.738 (10,6%)
- TODO/FIXME/XXX/HACK (não-teste): 154 — topo: pipeline/gate.go (7), cli/knowledge.go (4), search/layered.go (2)
- Código comentado aparente: ~146 linhas (heurística) — **insignificante**, não há código morto relevante

## 7. Generated / artefatos

- Go: 6 `*.pb.go` (api/grpc/pb) = **2.337 LOC**
- TS: `sdk/typescript/src/generated/api.ts` (openapi-typescript) = **786 linhas**
- Vendored (não mantido): node_modules visual-media = 1.340 arquivos, **119.293 linhas JS, 31MB**, 150 sourcemaps
- Duplicação: **`internal/bootstrap/.cosca/` = 925 arquivos, 9,5MB** (882 md, 107.569 linhas) — espelho materializado do fallback commitado na árvore fonte [FINDING]

## 8. Documentação (nunca contada como código)

| Categoria | Arquivos | Linhas |
|-----------|---------:|-------:|
| internal/embed/cosca (specs, skills, agentes, protocolos, memória, conhecimento) | 1.285 | 170.050 |
| docs/ | 60 | 23.539 |
| outros (README etc.) | 9 | 4.353 |
| **Total fonte** | **1.354** | **191.942** |
| Duplicado (bootstrap/.cosca) | 882 | 107.569 |
| Runtime memory (.cosca/memory) | 945 | — |

## 9. Memória e Knowledge (dados, não código)

- knowledge.db: **112** knowledge_entries (architecture 28, patterns 25, heuristics 22, failures 19, best-practices 15, cognitive 3); **556** documents; **15.841** chunks; **15.841** vetores 768-dim; 3 snapshots; relationships 0; symbols 0
- Learnings: **222** entradas `## L` (kernel 212 + 10 outros agentes); failures/patterns: 86
- chain.dat kernel: **221 blocos**
- provenance.yaml: 1 generation (qwen2.5-coder:14b), **0 claims**
- .cosca/memory runtime: 8 .md de sessão + index.db

## 10. Quadro final

```
┌──────────────────────────────┐
│ COSCA — DIMENSÃO REAL        │
├──────────────────────────────┤
│ Production LOC (manual)      │  133.528
│ Test LOC                     │  139.675
│ Documentation (fonte)        │  191.942  (+107.569 duplicada)
│ Knowledge (entries)          │  112 (+15.841 vetores)
│ Memory (learnings)           │  222 (+86 failures/patterns)
│ Generated                    │  3.123 (+119.293 vendored)
│ Total files (fonte)          │  2.491 (3.416 com duplicação)
│ Production/Test ratio        │  1,046 (testes superam produção)
└──────────────────────────────┘
```

### A–J

- **A)** Linhas reais sem comentários/blanks: **276.326** (Go 271.666 + TS 3.647 + Shell 285 + Python 577 + SQL 9 + Proto 142; inclui testes)
- **B)** Produção: **133.528**
- **C)** Testes: **139.675**
- **D)** Ratio: **1,046** — há mais LOC de teste que de produção
- **E)** 428 arquivos de teste (416 Go + 12 TS); **125/146 pacotes** têm ≥1 teste (85,6%); **95,9%** do LOC de produção Go vive em pacotes testados; 21 pacotes sem teste
- **F)** -race: Makefile (test-unit, test-integration, test-race, coverage-check) + ci.yml — toda a suíte roda -race no CI
- **G)** Integração: test/integration (editors, knowledge, memory, plugins, runtime, search) + soak. **E2E: inexistente** (web/ ausente, sem Playwright)
- **H)** Científica/3D/vídeo/áudio: em grande parte **spec** (engines cognitive-gravity/entropy, tdengine, gameengine, visual-media); Go real pequeno (tdengine 145, gameengine 144, media 253 com 61,8%, compute 1.853) e scripts Python visual-media **sem testes**
- **I)** Recentes fechando cobertura: pipeline (18,7%), cli (59,8%), chat/ui+tools (7-51%), editors (47-69%), indexer (43,1%), discovery (48,8%), security (42,7%)
- **J)** Maior concentração: **internal/cli = 27.425 LOC (20,5% de todo Go de produção)**; depois pipeline 9.786 (7,3%), api/rest/handler 4.951, orchestration 4.799, knowledge 4.745

## 11. Qualidade da medição

- [MEASURED]: todos os LOC, counts de DB, coverage (execução real `go test -cover`/`-coverprofile`), git, mtimes, TODO, generated
- [DERIVED]: agregado de coverage (coverprofile), ratio, percentuais
- [UNKNOWN]: cobertura de `internal/sandbox` (setup falha fora da jaula); total exato de statements de pacotes que flakearam (pipeline/pkg/cosca — parcial)

## 12. Conclusão — o tamanho é código ou mecanismos?

**O tamanho é explicado primariamente por MECANISMOS INTERLIGADOS sobre uma camada documental desproporcional, não por volume de código de features.**

- **LOC**: 131 mil LOC de Go manual é um sistema médio-grande — **abaixo** da percepção que a árvore transmite.
- **Documentação/spec**: 170 mil linhas de spec/contrato markdown (1,3× o código Go!) + 107 mil duplicadas + 119 mil vendored. Grande parte do "tamanho percebido" é essa camada.
- **Complexidade arquitetural**: 146 pacotes, 58+ engines de spec, dois subsistemas de memória, chains+merkle, provenance, approval fail-closed, 8+ DBs, jail. A complexidade é real e incomum — mas parte dela é **spec não implementada** (consensus, shadow-mode, wizard, identity, M1-M7: DOCUMENTADO, sem Go), o que infla a percepção.
- **Dívidas técnicas concretas**: pipeline com 18,7% de cobertura e teste flaky (coração da orquestração); E2E declarado e ausente; 21 pacotes sem teste; sandbox não testável fora da jaula; 9,5MB de documentação duplicada na árvore; git re-âncorado (sem história evolutiva auditável); `web/` referenciado e inexistente.
- **Forças medidas**: ratio teste/produção > 1, suíte -race no CI, core de execução (engine/context/runtime/orchestration) com cobertura ≥77%, segurança (secrets/auth/proposal/gate) ≥70%.
- **Engenharia incomum (explicação técnica)**: test LOC > prod LOC em 20 das 30 áreas-chave não é acidente — o sistema é orientado a *validação* (escada CKL, evidence, gate fail-closed), e a camada markdown embutida funciona como **código de contrato** executado pelo agente (o LLM), não pelo runtime — por isso a proporção docs/código é anômala para um repo Go típico.
