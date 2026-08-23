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
