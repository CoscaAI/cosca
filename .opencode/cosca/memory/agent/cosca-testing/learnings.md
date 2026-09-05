# cosca-testing — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-testing |
| **Task** | Initial capability establishment |
| **Technique** | Standard testing patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #testing #baseline #initialization |
| **Related** | See .opencode/cosca/memory/codebase/overview.md, .opencode/cosca/memory/pattern/ |
| **Learned** | Project established. Core testing patterns documented. Ready for level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

---

## Real Execution Learnings

### 2026-07-28 — Runtime Integration Suite (Level 3)

| Field | Value |
|-------|-------|
| **Agent** | cosca-testing |
| **Task** | Write integration suite for runtime state machine (20 transitions) + validate 3 bugs |
| **Technique** | AAA pattern, table-driven transition catalog, concurrent stress testing, bug reproduction |
| **Level** | 3 |
| **Outcome** | success |
| **Confidence Score** | **≥ 0.40** (up from 0.25 baseline) — primary domain: integration testing |
| **Tags** | #testing #integration #runtime #state-machine #bug-reproduction #race-detection |
| **Related** | `internal/runtime/state_integration_test.go`, `internal/runtime/runtime_lifecycle_test.go`, `.opencode/cosca/memory/testing/integration-report.md` |

**Key Learnings**:

1. **State machine testing requires transition graph coverage, not just happy paths.** The `validTransitions` map is the source of truth — every edge must be tested. Missing even one transition (like `Stopped → Uninitialized`) creates hidden bugs because the code path that should trigger it doesn't exist.

2. **Bug reproduction is the most valuable testing activity.** Reproducing BUG-U01 (Restart broken) revealed that the state machine's `validTransitions` map is correct but the code never executes the `Stopped → Uninitialized` transition. Without an integration test, this would have remained invisible.

3. **Event timing bugs require explicit subscriber-based verification.** BUG-U02 (EventStartupComplete premature) was confirmed by a test that subscribes to the event AND registers an init hook. The test proves the event fires before the hook executes — something unit tests can't detect.

4. **Mutex ordering in callback chains.** The `TransitionTo` method holds `rs.mu` while firing the `onChange` callback. If the callback tries to acquire a lock that the test already holds, deadlock occurs. Pattern: **never hold a lock across a transition call** when testing callbacks.

5. **Parallel testing with Runtime needs goroutine cleanup.** The `healthCheckLoop` goroutine created by `Runtime.Start()` persists until `Stop()` is called. Tests must pair every `Start()` with a `Stop()`. In parallel test suites, leaked goroutines from one test can interfere with others under `-race`.

6. **Metrics enumeration as test data.** BUG-U03 (metrics doc gap) was validated by enumerating all `MetricsSnapshot` fields in the test itself — the test serves as a living specification of what metrics exist.

**Technique Mastery**: Successfully applied AAA pattern across 30 test functions with 0 shared mutable state. The table-driven transition catalog pattern (22 sub-tests in a single table) proved highly maintainable and complete.

**Confidence Model Update**:
- **Primary Domain** (Integration Testing): confidence **0.40** (up from 0.25)
- **Secondary Domain** (Race Detection): confidence **0.35** (newly acquired)
- **Secondary Domain** (Bug Reproduction): confidence **0.45** (newly acquired)
- **Growth Area** (E2E Testing): still 0.05 — no E2E work done

**Next**: Level 4 — E2E test suite for critical user journeys, soak tests (>1h), CI pipeline integration with `-race` gate.

---

### 2026-08-24 — Fase B vectoragg×search (PROVA / TDD RED, contratos de confinamento)

| Field | Value |
|-------|-------|
| **Agent** | cosca-testing |
| **Task** | Escrever a PROVA (teste RED) da Fase B — search não pode materializar 28.888 embeddings quando há Scope roteado |
| **Technique** | TDD RED contratual; mock vector.Store que implementa `MetricsSearcher` e grava o conjunto de candidatos; previsibilidade RED/GREEN por contraste |
| **Level** | 3 |
| **Outcome** | success (1 RED esperado, 1 GREEN de contrapeso; 0 regressão na suíte) |
| **Tags** | #testing #tdd-red #vectoragg #search #faseB #integração #contrato |
| **Related** | `internal/search/faseb_pipeline_test.go`, `internal/search/scope.go`, `internal/search/search.go`, `internal/vectoragg/vectoragg.go` |

**Key Learnings**:

1. **PROVA contratual usa um "seam" observável, não um campo inexistente.** Para provar que `search` NÃO delega ao `vectoragg`, escrevi um `vector.Store` que implementa a superfície opcional `MetricsSearcher` e REGISTRA o slice de candidatos que a fase vetorial recebeu. Candidato `nil` ⇒ full-scan (`ScannedVectors == TotalVectors`); candidato não-vazio ⇒ caminho confinado. É o único jeito de provar o invariante SEM implementar a fiação (que é a fase IMPLEMENTAR, separada).

2. **O par RED/GREEN é o que define a fronteira do contrato.** Teste 1 (Scope roteado + CandidateIDs ⇒ DEVE confinar) é RED hoje; Teste 2 (sem Scope ⇒ full-scan é legítimo) é GREEN hoje. Juntos documentam que o confinamento é exigido SÓ quando há scope — nunca como regra universal. Usar `SearchWithRoute` com rota CONHECIDA (RED) vs rota DESCONHECIDA/NoRoute (GREEN) para variar só a presença do scope.

3. **TDD RED exige que o teste compile e rode** — não deve quebrar o `go build ./...`. A falha deve ser de ASSERT (runtime), não de compilação/import-ciclo. Evitei importar `vectoragg` no teste (seria um ciclo de intenção / impl); referencio o contrato apenas em comentários.

4. **Mecanismo interno de candidatos ≠ contrato do vectoragg.** O `resolveChunkCandidates` do `search` só entende `chunks_fts_<rowid>` (exige cliente FTS) e viaja para `SearchWithCandidates`/`SearchWithMetrics`. O `vectoragg.RetrieveCandidates` aceita IDs de vetor permitidos diretamente e decodifica SÓ os TopK. A lacuna de fiação é exatamente essa: passar o conjunto roteado ao vectoragg (que o testa com IDs de vetor simples), em vez de depender da tradução via FTS.

5. **Verificação final de "não quebrei a suíte":** rodar a suíte do pacote inteira e grep por `--- FAIL` deve listar APENAS o novo teste RED. `go build ./...` sem saída = build ok. `git status --short` deve mostrar apenas o arquivo de teste novo como artefato meu (o resto do workspace sujo é ruído pré-existente de outros agentes/runtime).

**Next**: Fase IMPLEMENTAR — conectar `search ↔ vectoragg` e tornar o Teste 1 verde; depois, testar que o novo caminho NÃO introduz regressão de materialização.

---

### 2026-09-03 — Caso D: Continuidade semântica de capacidades (invetariante anti-regressão + doutrina persistida)

| Field | Value |
|-------|-------|
| **Agent** | cosca-testing |
| **Task** | Caso D (restart): provar que o COSCA não regride para "visão = VLM externo". Regressão de continuidade da RELAÇÃO arquitetural (capacidade→sensores→Observation→fusão→gate→(resolve\|escala VLM)). |
| **Technique** | (1) Teste de INTEGRAÇÃO anti-regressão: caminho canônico em versão de teste (fusion.Fuse→gate.Decide) com um `vlmSet` de providers registrados como fato externo — provar que o provider é INERTE ao veredito; (2) Teste DETERMINÍSTICO de doutrina persistida: ler ADR-036 + provenance.yaml + learnings do kernel do disco (via repoRoot/go.mod walk) e verificar que a RELAÇÃO ressurge e que NÃO é "visão=VLM" (cláusula "VLM é escalada, nunca identidade"). |
| **Level** | 3 → 4 |
| **Outcome** | success — 2 arquivos de teste novos, todos PASS; vet/build limpos; 0 regressão na suíte sensor. Nenhum `.go` funcional tocado. |
| **Confianca** | 0.94 |
| **Tags** | #testing #caso-D #continuidade-semantica #anti-regressao #capacidade-provider #gate #fusion #ADR-036 #integração #test-deterministico |
| **Related** | `internal/sensor/gate/continuity_test.go`, `internal/sensor/semantic_continuity_test.go`, `docs/adr/ADR-036-continuidade-semantica-de-capacidades.md`, `internal/sensor/gate/gate.go`, `internal/sensor/fusion/fusion.go` |

**Key Learnings**:

1. **A anti-regressão é testável SEM tocar código funcional — via o invariante de "provider inerte".** O gate (`Policy.Decide`) NÃO recebe um parâmetro provider, então a prova não é "o gate ignora provider" — é que, no caminho canônico, o veredito é função PURA da evidência. Gravei um `route(obs, providers, p)` (helper de teste) que roda `fusion.Fuse→gate.Decide` e outro `assertVerdictInertToProviders` que compara o resultado com `providers=nil` vs `providers={vlm}`. Se o veredito muda, o caminho primário quebrou. As duas invariantes do ADR-036 §2.4 (#2, #3) viram asserções concretas.

2. **Separar "veredito do gate" de "consultar VLM" é essencial — senão o teste fica falso.** `vlm` só é `true` quando o gate diz `Escalate` E há um alvo registrado. Então, ao comparar "sem provider" vs "com provider", o `vlm` DIFERE legitimamente (sem alvo, nunca consulta). A invariante é sobre o VEREDITO (`v`), não sobre `vlm`; `vlm` deve espelhar o `Escalate` quando há alvo. Confundi-los teria gerado erro no caso "contradição" (escalada sem alvo). A lição: **a igualdade entre cenários é sobre a DECISÃO do gate, não sobre a existência do alvo.**

3. **Teste de "restart" não precisa de índice semântico — usa as fontes persistidas em disco.** O knowledge.db pode estar sem reindex (ADR-036 não indexado), então a busca semântica é instável. O Caso D é validável de forma DETERMINÍSTICA lendo as fontes que SOBREVIVEM ao restart e que a busca indexaria: `docs/adr/ADR-036...`, `.cosca/provenance.yaml`, `.opencode/cosca/memory/agent/cosca-kernel/learnings.md`. Encontrei o repo root subindo do CWD do pacote até `go.mod` (padrão `baseline_recall_test.go`). Normalizei o texto (lowercase + strip acentos PT + `≠`→`!=`) pra casar formas acentuadas/não-acentuadas e evitar falsos negativos.

4. **Verificação-negativa robusta (não matchear a frase proibida literalmente).** O texto das fontes CONTÉM a frase "visão = VLM externo" dentro do OBJETIVO ("impedir que o despertar regrida para..."). Logo um teste de "não deve conter 'visão=VLM'" falharia. A saída é asserção-POSITIVA: a doutrina declara explicitamente "VLM é escalada, NUNCA identidade da capacidade" e "capacidade != provider". Presença dessas cláusulas prova a antítese da regressão, sem depender de string proibida.

5. **Não criar diretório novo só de `_test.go`** — quebraria `go build ./...` ("no buildable Go files"). Coloquei os testes em pacotes existentes (`gate` e `sensor`) que já têm código de produção. O teste de doutrina vai em `sensor` porque a doutrina governa exatamente o `sensor.Observation`, mas valida documentação/proveniência, não código.

**Gap comprovado (registrado, NÃO resolvido — exige mudança funcional)**: a continuidade de RELAÇÃO é testada, mas a continuidade de RUNTIME de ponta-a-ponta (o `cosca despertar` carregar a doutrina + pipeline `screen.Observations→fusion.Fuse→gate.Decide` estar ligado + destino de escalada) continua sendo o GAP do ADR-036 §6. Testar isso exigiria ligar o pipeline e/ou carregar a doutrina no despertar — mudança em `.go` funcional, fora do escopo (não fiz nenhuma).

**Next**: (1) Quando o pipeline for ligado em runtime (ADR-036 §6), escrever o teste E2E que fecha `screen→fusion→gate→escalada` de verdade. (2) Após reindex, validar `cosca knowledge search "capacidade visao sensores gate escalacao VLM"` devolve a doutrina. (3) NÃO duplicar: qualquer novo código consome `sensor.Observation`+`fusion.Fuse`+`gate.Policy.Decide`.
