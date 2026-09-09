# cosca-qa - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-qa — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Active Learnings

### 2026-07-28 — Quality Gates Definition (1ª Task Real)

| Field | Value |
|-------|-------|
| **Agent** | cosca-qa |
| **Task** | Definir quality gates G0–G9 para a plataforma Cosca (Onda 2, Fase 1) |
| **Technique** | Level 2 — Multi-layered quality audit: bug registry cross-validation (5 bugs registrados → 8 bugs totais após code-to-docs analysis), gate definition per artifact type, Wave 2 task acceptance criteria validation |
| **Level** | 2 |
| **Outcome** | success |
| **Confidence (domínio primário)** | **≥ 0.50** (sugerido — baseline era 0.25, subida justificada por: auditoria completa de 8 bugs com classificação QA, definição de 10 gates concretos, validação de 9 tasks com critérios verificáveis) |
| **Tags** | #quality #gates #bug-audit #acceptance-criteria #wave-2 #classification #rejection-criteria |
| **Related** | G0–G9 pipeline gates, CONSTITUTION P1/P2/P6, QUALITY_GATES.md (ciclo de vida), onda-2-plan.md, bug INDEX 3.1.0 |
| **Learned** | 1) Bug registry formal subestimava bugs reais: 5 documentados vs 8 existentes (3 encontrados via code-to-docs cross-validation). 2) Acceptance criteria precisam ser verificáveis: critérios subjetivos ("profundidade analítica") devem ser substituídos por thresholds objetivos. 3) G0–G9 devem ser sequenciais e automatizados — G0–G5 no CI, G6–G9 semi-automatizados. 4) Critério de rejeição automática para security scan (G4) é inegociável — P1 manda. 5) Coverage piso de 70% é realista para o baseline atual (~78%); 80% no diff é enforcing progressivo. 6) Doc-code validator é o mecanismo mais crítico para prevenir o drift docs↔código (bug-008). 7) Onda 2: 8 tasks aprovados, 1 com ressalvas (cosca-critic), 1 executando (cosca-qa). |
| **What worked well** | Cross-referencing bug INDEX com código real via `grep` e leitura de source (runtime.go, state.go, metrics.go). Uso do learnings do cosca-runtime como fonte de bugs não registrados. Classificação por severidade QA (blocker/critical/major/minor/trivial) + owner sugerido. Definição de acceptance criteria por tipo de artefato com tabelas de gates aplicáveis. |
| **What was difficult** | Identificar os 3 bugs não registrados exigiu ler o learnings do cosca-runtime (não estavam no INDEX). A definição de thresholds de rejeição automática exigiu balancear rigidez (security) com pragmatismo (coverage pode ter exceção justificada). A validação dos critérios da Onda 2 exigiu ler o documento completo de 702 linhas do plano. |
| **Improvement points** | 1) Criar um script `qa-audit.sh` que automatize a cross-validation bug registry vs código. 2) Integrar o quality gate validator ao CI (cosca-devops deve referenciar este documento). 3) Implementar rastreamento de flaky tests (métrica ainda desconhecida). 4) Auditoria de zero-state safety em todos os initializers (padrão do bug-005 pode se repetir). |
| **Next** | Level 3: Validar execução concreta dos gates após CI implementado pelo cosca-devops. Medir bug density pós-Onda 2. Definir SLAs por categoria de teste (unit < 5min, integration < 15min, E2E < 30min). Implementar flaky test detection (>1 falha em 10 runs = flaky). |

---

### 2026-08-23 — RAG Fidelity Gates ADR (Mega Brain C4–C7)

| Field | Value |
|-------|-------|
| **Agent** | cosca-qa |
| **Task** | Produzir ADR-011 (decisão/design, NÃO implementar) para adotar gates de fidelidade/anti-alucinação do RAG (Mega Brain) no Cosca |
| **Technique** | Level 3 — Mapeamento honesto do estado atual vs gap (reuso-first, P8) antes de sugerir design: confirmar que `internal/rag`/`semantic-memory` NÃO existem e que o "RAG" é `internal/knowledge`+`search`+`vector`+`embeddings`; detectar building blocks prontos (`thinking.go` ClaimStore/ClaimKind/IsTrustworthy, `catalog` invariante D fail-on-invariant, `skilleval.RegressionGate` holdout, fail-open da busca no `search.Engine.Search`); separar fatia bounded (heurístico + atribuição por claim + qrels congelado + assinatura de embedding) de over-engineering (HHEM local 400MB fases 2, atomic_facts/BrainHealth fase 3); espelhar a forma determinística do `cosca gate catalog --audit --strict` para o novo `cosca gate recall` |
| **Level** | 3 |
| **Outcome** | success |
| **Confidence (domínio primário)** | **≥ 0.70** (subida por: leitura direta dos arquivos-chave `search.go`, `vector.go`, `embeddings.go`/`provider.go`, `knowledge/thinking.go`, `evals/oracle+metrics+suite+runner`, `skilleval/regression+guardrail`, `catalog/catalog.go`, `cli/catalog_gate.go`, `knowledge_claim.go`; verificação que já existem ClaimStore e gate catalog — evita duplicação) |
| **Tags** | #rag #fidelity #anti-hallucination #grounding #claims #claim-attribution #self-rag #hhem #qrels #embedding-space #recall-gate #over-engineering #cost-gate #adr #mega-brain #reuse #fail-open #fail-closed |
| **Related** | mega-brain-patterns.md C1–C7, ADR-002, internal/search | internal/vector | internal/embeddings | internal/knowledge | internal/evals | internal/skilleval | internal/catalog | internal/gate |
| **Learned** | 1) Reuso-first poupa muito: o Cosca já tem a máquina de claims (`ClaimStore`, `Classify()` determinístico) e o precedente de gate determinístico fail-on-invariant (`cosca gate catalog --audit --strict`, invariante D mojibake). O design deveria **conectar** essas peças à resposta do RAG, não criar do zero. 2) Honestidade de custo: HHEM NLI local (~400MB, inferência CPU) é over-engineering para um pbitol local-first com provedor de embedding leve; o heurístico de fidelidade (token-overlap + n-gram/número, zero-LLM) dá valor isolado imediato e é a fatia 1. HHEM só entra se o A/B em `qrels` mostrar resíduo real (fase 2, condicionado). 3) Distinguir fail-open do *retrieval* (C3 — busca quebra → degrada) de fail-closed do *gate* (score real baixo bloqueia; `queries_errored>0 ⇒ fail` no gabarito). 4) A busca já é fail-open no `search.Engine.Search` (fases FTS/vector/graph degradam com warn) — o C3 no nível de retrieval está em grande parte satisfeito; o novo fail-open é no scorer (`faithfulness == -1 nunca block`) e no "sem evidência não gera". 5) Não existe answer builder em Go (`contextBuilderAdapter` é placeholder) — o ponto de integração é um chamador novo; por isso a facada `SearchGrounded` aditiva + `Verdict()` isola quem monta contexto. 6) Precedente de forma a espelhar: `cosca gate catalog` tem `--check`/`--audit`/`--strict`/`--summary`/`--json` — o novo `cosca gate recall` deve ter o mesmo contrato. |
| **What worked well** | Ler os arquivos reais em paralelo (glob+read) para ter o mapa exato e desmentir premissas do prompt (não existia internal/rag; mas existia ClaimStore e gate catalog). Ancorar o design em reuso concreto (P8) e separar fatia bounded de over-engineering com "gate de custo" explícito por padrão (env `COSCA_GROUNDING_HHEM_ENABLED`). |
| **What was difficult** | Confirmar sem reuso a real ausência de `internal/rag`/`semantic-memory` (glob vazio) e provar que o ClaimStore já existia (grep levou a `knowledge/thinking.go`). Balancear o que é C1 já-satisfeito (`ranking.Ranker` merge híbrido) do que é gap (cross-encoder/HHEM = mesma classe de custo, diferido). Escolher o caminho de arquivos concretos para os 2 blocos sem colidir com `evals` (orquestração) — daí `internal/grounding/`. |
| **Improvement points** | 1) Validar a fatia 1 com um protótipo mínimo de `ExtractClaims`+`VerifyClaim` (token-overlap) e um `qrels-baseline` de ~20 queries sobre a base real. 2) Medir o "resíduo" do heurístico no `qrels` (infidelidade não captada) para decidir com evidência se o HHEM vale a pena. 3) Acoplar o `RecallGate` ao mesmo padrão `--check/--audit/--strict` do catalog para consistência de UX de gate. |
| **Next** | Level 4: Implementar a fatia 1 (grounding heurístico + claim attribution + qrels gate + assinatura de embedding) e rodar o A/B de resíduo para justificar/adiar o HHEM. |

---

### 2026-08-29 — Regression Tests Cérebro 3D (readActivityLog + brainweb)

| Field | Value |
|-------|-------|
| **Agent** | cosca-qa |
| **Task** | Implementar testes de regressão auditados do cérebro neural 3D (módulo read-only) — API rest + internal/brainweb |
| **Technique** | Level 3 — Implementação AAA + table-driven respeitando a forma do projeto; descoberta de assinatura real via glob/read (getActivityLog, Activity struct, handler mount, embed, observatory builder) ANTES de escrever; adaptação defensiva a mudança concorrente de produção (campo `Prompt`→`Action` no brainweb.Activity) em vez de assert cego |
| **Level** | 3 |
| **Outcome** | success |
| **Confidence (domínio primário)** | **≥ 0.75** (subida por: 6 testes novos todos verdes em `go test -count=1`; build prod resolvido por agente concorrente sem eu tocar o código de produção; assert ajustado para o comportamento correto pós-fix de segurança) |
| **Tags** | #regression #brainweb #activity-log #path-traversal #observatory #prompt-leak #g6 #table-driven #aaa #go-test |
| **Related** | api/rest/activity_log_test.go, internal/brainweb/brainweb_regression_test.go, server.go readActivityLog, graph.go Activity, observatory.go, handler.go mount/contentType, embed.go WebFS |
| **Learned** | 1) O bug G6 foi corrigido por RENOMEAR o campo de atividade: `Prompt` (vazava entrada do usuário) virou `Action` que carrega SÓ o rótulo seguro ("COMMAND_EXECUTED"), nunca args/prompt. `readActivityLog` segue mapeando `Action: ra.Action`. 2) Teste de regressão de security não deve assert cego — precisa se adaptar ao comportamento correto pós-fix (aqui: nenhum campo Prompt, e guarda reflect de que `Activity` NÃO tem campo `Prompt`). 3) `readActivityLog` é testável em branco (package `rest` = white-box), devolve as mais recentes primeiro (at desc), pulom linha malformada (json mismatch) E vazia, usa `roll = agent || actor` para o ID `at-roll`. 4) path traversal: mount faz `ReplaceAll(name,"..","")` + `path.Clean`; variações URL-encoded (`%2f`, `%2e%2e`) e `....//` são normalizadas e caem em 404 (Open de `web/<clean>` inexistente). 5) Assets do brainweb: `contentType` por extensão (.css→text/css, .js→application/javascript, .obj→text/plain) — assert deve usar prefixo pois há `; charset=utf-8`; corpo não-vazio garante que o go:embed realmente embutiu. 6) `statsFn` do observatório popula `Cognitive.Agents`/`.Skills`; item com `Status==""` cai no ramo `st=="UNKNOWN"`. |
| **What worked well** | Ler `handler.go`/`embed.go`/`observatory.go`/`graph.go` e `go.mod` antes de escrever; confirmar colisão de nomes de teste via grep; `go build ./api/rest/` para separar erro de produção (unused `io`, concorrente) dos meus; usar `-run` + `-count=1` para ver cada subteste sem cache. Guarda reflect contra reintrodução do campo `Prompt` recompensa o esforço. |
| **What was difficult** | O build inicial de `go test ./api/rest` falhou 2x por causa de PRODUÇÃO em mudança (unused `io` + campo Prompt antigo), não pelos meus testes — exigiu diferenciar erro de produção de erro de teste e NÃO corrigir prod. Decidir o assert correto após o fix G6 (Action = rótulo seguro, NÃO ação real), pois o prompt do enunciado conflitava com o fix aplicado; segui a cláusula "ajuste o assert para o comportamento correto". |
| **Improvement points** | 1) Para testes de regressão de security, sempre verificar o estado REAL do struct via leitura fresca (o grafo muda por agentes concorrentes) antes de assert cego. 2) Adicionar guarda reflect `TestActivity_SemCampoPrompt` como padrão em projeções de payload público. 3) Rodar `go build` no pacote antes dos testes para separar lint/build de produção dos meus. 4) Para `readActivityLog`, considerar teste de `limit<=0` (fallback p/de 30 no `Recent`) — gap em aberto. |
| **Next** | Level 4: adicionar teste de `limit<=0`/fallback no `executionActivitySource.Recent`; validar que os assets do observatório cobrem todos os 6 estados epistemológicos + STALE; medir coverage diff do brainweb pós-teste. |

---

## Seed Knowledge (Legacy)

### 2026-07-27 — Quality Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-qa |
| **Task** | Quality standards definition |
| **Technique** | Test pyramid validation — unit > integration > E2E |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #quality #test-pyramid #coverage #acceptance |
| **Related** | Test strategies, coverage thresholds, bug density |
| **Learned** | Coverage > 80% target. Bug density tracking needed. Acceptance criteria must be verifiable. |
| **Next** | Level 2: Define SLAs per test category, implement flaky test detection |

