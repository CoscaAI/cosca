# ADR-011: Gates de Fidelidade Anti-Alucinação do RAG (Mega Brain C4–C7)

> **Status**: Proposed | **Owner**: cosca-qa (QA Chief) | **Last Updated**: 2026-08-23
> **Revisão**: aguardando cosca-cto + Don. **Decisão de design — NÃO implementada.**
> **Fonte**: `.cosca/fallback/knowledge/patterns/mega-brain-patterns.md` (seções C1–C7, RAG grounded)

---

## 0. Mapa objetivo do que o Cosca JÁ tem (o "estado atual" para o gap)

Antes de declarar o gap, o mapeamento real (evitando achar lacuna onde não existe):

| Bloco real hoje | Onde | O que cobre |
|---|---|---|
| **Motor de busca híbrida** | `internal/search` (`search.Engine.Search`) | FTS5 + vetor + grafo + re-rank via `internal/ranking` (`ranking.Ranker`). **Sem** gate de fidelidade. |
| **Knowledge Engine** | `internal/knowledge` (`knowledge.Engine`) | Orquestra indexação/busca/cache/verify. É o "RAG" do Cosca hoje. `Verify()` checa integridade (vetores órfãos, dangling, dimensão). |
| **Vetores** | `internal/vector` (`vector.Store`, `vector.NewSQLiteVec`) | sqlite-vec; `VectorRecord{ID, Vector, DocumentID, ChunkID, EntityID, Content, Metadata}`. **Sem** assinatura `model:dim` no record. |
| **Embeddings** | `internal/embeddings` (`ProviderRegistry`) | `Select(cfg)` (Primary/Fallback/AutoDetect/`Digest`/`Dimensions`); `GenerateEmbedding(s)`; `Model()`/`Dimensions()`. Já tem **fail-closed de identidade** (`ErrEmbeddingIdentityMismatch`, digest do modelo — L376). Cache chaveado por `sha256(text)` apenas. |
| **Atribuição por claim (já existe!)** | `internal/knowledge/thinking.go` | `ClaimKind` (FACT/EVIDENCE/INFERENCE/ASSUMPTION/HYPOTHESIS/UNKNOWN), `ClaimStore` (`.cosca/claims.db`), `ClaimRecord{Kind, Statement, Supports, ContradictedBy, Confidence}`, `IsTrustworthy()`, `Classify()` determinístico. CLI `cosca knowledge claim`. |
| **Evidência + proveniência** | `internal/knowledge/evidence.go`, `provenance.go` | `Evidence` com P0–P5, `IsReproducible()`, `KnowledgeItem` (escada observation→law), `LevelRegression`/`ValidateLevels`. |
| **Oráculo (evals)** | `internal/evals` (`oracle.go`, `metrics.go`, `suite.go`, `runner.go`, `canary.go`) | `SubmitToOracle` (verificação black-box), `computeMetrics` (Accuracy/Recall/F1/MCC/BootstrapStd), `Report`, `StripCanary`. Foco em cases de orquestração (verify commands), **não** em recall de retrieval. |
| **Meta-loop A/B + RegressionGate** | `internal/skilleval` (`ab.go`, `regression.go`, `guardrail.go`) | `RegressionGate.CheckRegression` (holdout, tolerância 0.02, determinístico) + guardrails (invariante B estrutural). É o análogo estrutural do gate de recall. |
| **Gate determinístico fail-on-invariant** | `internal/catalog` + `internal/cli/catalog_gate.go` | `cosca gate catalog --audit --strict`; invariantes A (INDEX), B (frontmatter), C (cross-refs), **D (mojibake)**; generate-and-diff (`catalog.manifest`). Precedente exato de gate bloqueante no CI. |
| **Gate plan-approval** | `internal/gate` | Máquina de estados Gate 0–4 (aprov. de planos). Fora do escopo. |

> Conclusão do mapa: **não existe** `internal/rag`, nem `internal/semantic-memory`, nem `internal/evals` de recall de retrieval. O "RAG" é `internal/knowledge`+`internal/search`+`internal/vector`; a "memória semântica" (embedding) é `internal/embeddings`. A **busca não tem gate de fidelidade**, não há **atribuição por claim de resposta**, não há **quarentena de espaço de embedding** e não há **gabarito congelado** para não-regressão de recall. O Cosca **já tem** a máquina de claims e a escada de evidência — o ganho é conectar isso à resposta do RAG, não criar do zero.

---

## 1. Contexto / Problema (o gap de anti-alucinação)

A busca semântica do Cosca (`internal/search` + `internal/knowledge`) devolve um ranking de chunks via FTS+vétor+grafo, mas:

1. **Sem gate de fidelidade.** O ranking é por similaridade (BM25/cosíno/grafo). Não existe verificação de que a *resposta gerada* é sustentada pelos chunks recuperados. Um chamador consome `search.SearchResults` e monta contexto/answer com o que vier — sem distinguir fato de alucinação.

2. **Sem atribuição por claim.** Cada `SearchResult` carrega `ID`, `Score`, `Snippet`, `DocumentID`, `Source` — mas **não** um vínculo `claim → source_chunk` + `matching_terms`. Não é possível auditar, por afirmação da resposta, *de qual chunk ela veio*. O modelo de claims existe (`ClaimStore`), mas é usado em separado (`cosca knowledge claim`), não acoplado à resposta do RAG.

3. **Sem invariante de espaço de embedding.** `VectorRecord` não guarda assinatura `model:dim`; o cache de embedding chaveia só por `sha256(text)`. Trocar de modelo/provedor pode **silenciosamente misturar** vetores de espaços divergentes (dimen. A vs B) — exatamente o "achismo" que o C2 (Mega Brain) trata. O Cosca já tem fail-closed de **identidade** (digest) no `Select`, mas não o de **compatibilidade/espaço** no reuso de vetores persistidos.

4. **Sem gabarito congelado p/ não-regressão de recall.** Muda-se o `ranking.Ranker`, o `chunker`, o provedor de embedding, o `reranker` — e o recall pode regredir imperceptivelmente. Não há piso de `recall@K` / `first_relevant_hit` / `nDCG@k` bloqueando o CI. O `internal/catalog` mostra que o Cosca *sabe* fazer gate determinístico — mas não aplica isso ao retrieval.

5. **"Sem evidência não gera" não existe no chamador.** Não há um `answer_builder` que **recuse gerar** quando não há evidência. `adapters_context_builder_adapter.go` é um placeholder (`Build()` retorna `nil`). Vale o risco de o LLM inventar a partir de conhecimento paramétrico.

---

## 2. Decisão (2 blocos)

**Adotar** (com adaptação honesta) os padrões C3/C4/C5 e C2/C6/C7 do Mega Brain, em **dois blocos** separados, priorizados por custo-valor:

### Bloco 1 — Cascata de gates de fidelidade + atribuição por claim (C4 + C5 + C3)
- **C4/self-RAG heurístico** primeiro, **zero-LLM, ~1ms**: extrair claims da resposta → verificar cada claim contra os chunks recuperados (token-overlap + bônus n-grama/número) → `faithfulness = supported/total`.
- **C4/block vs flag** no **chamador**: `0 ≤ faithfulness < threshold (default 0.60)` → **BLOCK** (`delivered:false`, placeholder seguro, resposta original preservada em `blocked_response` p/ auditoria); senão **FLAG**. **`faithfulness == -1` nunca bloqueia** (fail-open no scorer).
- **C5/atribuição por claim**: cada claim anexa `source_chunk` (`chunk_id`) + `matching_terms`. **"Sem evidência não gera"**: `len(chunks) == 0` → resposta **extrativa/degradada**, nunca chamada generativa ao LLM.
- **C3/fail-open como invariante**: a expansão/verificação **só soma recall, nunca derruba a entrega**; kill-switch por env com precedência (`COSCA_GROUNDING_*`) paridade com `MCE_RERANKER_ENABLED`.

### Bloco 2 — Gabarito congelado + invariante de espaço de embedding (C6 + C2 + C7)
- **C6/qrels congelado**: `qrels-baseline.json` commitado (query, `relevant_chunk_ids`, `first_relevant_chunk_id`) + gate fail-closed no CI (`cosca gate recall --audit --strict`): `recall@K`, `first_relevant_hit`, `nDCG@k`; **`queries_errored > 0 ⇒ fail`** (postura fail-closed do Mega Brain — descartar query fácil inflaria a nota).
- **C2/invariante de espaço único**: assinatura `model:dim` propagada no cache de embedding + no `VectorRecord`/store; **quarentena por assinatura** no reuso de vetores persistidos (assinatura on-disk ≠ ativa ⇒ re-embedd) + gate `embedding_space_invariant` no `Verify()`.
- **C7/evidência atômica + BrainHealth**: **fase posterior**, monitor-only (fail-warn). Cosca tem grafo (`internal/graph`) mas edges NÃO carregam `atomic_facts` (frases-fonte verbatim). Estado atual: gap real, mas custo/escopo alto — fora da fatia 1.

### Honestidade sobre over-engineering (o "gate de custo")
- **HHEM NLI local (C4, fase 2) é PESADO**: ~400MB de modelo ONNX cross-encoder, inferência em CPU, tokenizer extra — custo de latência/implantacção em um bitol local-first com provedor de embedding leve (default nomic-embed-text 768d via "local"). **Não vale começar por ele.** O heurístico (token-overlap + n-gram/número) já cobre a maioria dos casos de resposta não-sustentada. HHEM só entra se **o dado mostrar resíduo** (ver Fatia 1 / Alternativas).
- **Cross-encoder reranker (C1/hybrid_query)** e **RRF+cosine re-score**: Cosca já faz merge híbrido via `ranking.Ranker`; um re-ranker cross-encoder é a **mesma classe de custo** do HHEM → diferido pelo mesmo argumento. C1 é considerado **já satisfeito** (fora de escopo).

---

## 3. Desenho Técnico

### 3.1 Pacote novo: `internal/grounding` (o "RAG grounded" — separado do engine)

Motivo do nome: evita colidir com `internal/rag` (inexistente) e com o fato de `internal/knowledge` ser o *engine*. `grounding` pega as `search.SearchResults` e devolve um veredito de fidelidade + atribuição por claim. **Não re-implementa busca** — consume `search.Engine.Search`.

```
internal/grounding/
  claim.go        // Claim, SourceChunk, ClaimVerdict
  extract.go      // ExtractClaims(text, citePatterns) []Claim   (split sentenças + marcas [RAG:]/[Chunk N]/[K-xxxx])
  verify.go       // VerifyClaim(query, claim, chunks, opts) ClaimVerdict  (token-overlap + bônus n-gram/número; best_chunk_idx)
  fidelity.go     // Verify(ctx, query, answer, chunks, opts) FidelityReport; Faithfulness(); Verdict(); DefaultFaithfulnessThreshold=0.60
  cascade.go      // Gater interface + HHEMGate (phase 2, opcional); Choose() retorna heuristic-only por default
  answer.go       // BuildAnswer(ctx, query, chunks, llmFn, opts)  ("sem evidência não gera"; degrade p/ extrativo)
  recall_gate.go  // RecallGate.Run(ctx, retriever, baseline, floor) — recall@K, first_relevant_hit, nDCG@k, queries_errored
  qrels.go        // Qrels struct + LoadQrels(path) + Validation
```

**Tipos centrais (esboço — não implementar, decisão/design):**

```go
// claim.go
type SourceChunk struct {
    ChunkID        string
    DocumentID     string
    DocumentPath   string
    MatchingTerms  []string
    Score          float64
}
type Claim struct {
    Text          string
    CitationMarker string          // "[RAG:chunk_id]" | "[Chunk N]" | "[K-xxxx]"
    HasCitation   bool
    Kind          knowledge.ClaimKind  // reusa ClaimStore determinístico
    Confidence    float64
    SourceChunk   *SourceChunk
    Supported     bool
}

// fidelity.go
type Verdict string // "block" | "flag" | "deliver"
type FidelityReport struct {
    Faithfulness   float64  // supported/total; -1 = scorer indisponível (fail-open)
    Supported      int
    Total          int
    Claims         []Claim
    Blocked        bool
    Flagged        bool
    Delivered      bool
    BlockedResponse string // resposta original preservada p/ auditoria quando blocked
}
func DefaultVerdict() Verdict // threshold 0.60; faithfulness == -1 nunca block
```

### 3.2 Reuso concreto (P8 — não duplicar o que existe)

| Preciso | Reuso existente | Adaptação mínima |
|---|---|---|
| Classificação determinística de claims | `knowledge.Classifier`/`Classify()` e `ClaimStore` | O `Claim` do grounding **embute** `Kind`+`Confidence` do `ClaimStore`; ao persistir, chama `ClaimStore.ClassifyAndAdd` e guarda o `CL-xxxx` na resposta. **Não recriar** a máquina de tipos. |
| Evidência/auditoria | `knowledge.Evidence` + `ProvenanceLevel` | `source_chunk` da resposta alimenta um `Evidence{Kind:"test", Source:"rag-answer", Provenance, SHA256}` — trilha "zero achismo" já existente. |
| Custo da "não-regressão" | `internal/skilleval.RegressionGate` (holdout, tolerância) | Análogo: o **RecallGate** usa a MESMA filosofia (delta/floor determinístico, holdout de qrels), mas mede recall de retrieval, não score de skill. Não acoplar os dois. |
| Gate determinístico no CI | `internal/catalog` (`cosca gate catalog --audit --strict`), invariante D | **Paridade de forma**: `cosca gate recall --audit --strict` com o mesmo contrato `--check`/`--audit`/`--strict`/`--summary` e `--json`, e o mesmo padrão de "fail apenas em --strict, débito reportado sem --strict". |
| Métricas puras | `internal/evals/metrics.go` (`Recall`, `FScore`, `computeMetrics`) | `RecallGate` reusa `Recall`/padrão de report; a métrica de relacionamento é nova (recall@K, nDCG@k por interseção de `chunk_id`) — colocar no `internal/grounding/recall_gate.go` para não encher `evals` (que é de orquestração). |
| Fail-open da busca | `search.Engine.Search` (fases FTS/vector/graph degradam com `log.Warn` e continuam) | Já é o C3 no nível de retrieval. O **novo** fail-open é no *scorer* (`faithfulness == -1 nunca block`) e no `BuildAnswer` ("sem evidência → extrativo"). |
| Fuga do modelo paramétrico | `search.Engine` (`embedFunc`) | `BuildAnswer` só chama `llmFn` quando `len(chunks) > 0`; senão `Answer{Delivered:false, Reason:"no_evidence"}` ou resposta extrativa. |

### 3.3 Integração (ponto de inserção no chamador)

- **Não** reescrever `search.Engine.Search`. Adicionar um método de fachada no `knowledge.Engine` que é *opcional* e *sem custo* quando desligado:

```go
// knowledge.go (facada aditiva)
type GroundedResult struct {
    Results  *search.SearchResults
    Fidelity *grounding.FidelityReport
}
func (e *Engine) SearchGrounded(ctx context.Context, params search.SearchParams, g *grounding.Config) (*GroundedResult, error)
// g == nil ⇒ retorna e.Search(ctx, params) com Fidelity nil (comportamento atual, zero regressão).
```

- **Onde o chamador aplica o veredito**: no ponto em que as `search.SearchResults` viram contexto/resposta de LLM. Hoje isso **não existe em Go** (`adapters_context_builder_adapter.go` é placeholder) — está no layer de orquestração/agente. A integração é **aditiva**: quem monta contexto passa a consultar `FidelityReport.Verdict()`:
  - `block` → não entregar a resposta generativa; entregar placeholder seguro + `blocked_response` p/ auditoria.
  - `flag` → entregar com aviso/encaixa de baixa confiança por claim.
  - `deliver` → entregar.

- **Estado de fidelidade por causa**: para **atrever `blocked`** é só não emitir; para **auditar**, persistir `FidelityReport` junto ao `ClaimStore` (claims com `CL-xxxx`) e ao `Evidence` — trilha reversível até os chunks.

### 3.4 Bloco 2 — Gabarito congelado + invariante de embedding

**C6 — qrels congelado + gate fail-closed no CI:**

```
internal/grounding/
  qrels.go          // Qrels{Version, Embedding{Model, Dim}, Queries[]{ID, Query, RelevantChunkIDs, FirstRelevantChunkID}}
  recall_gate.go    // RecallGate{FloorRecall, FloorFirst, ...}.Run(ctx, retriever, qrels, opts) RecallReport
testdata/qrels-baseline.json   // gabarito commitado (schema v1, assinatura model:dim)
```

- **Contrato da CLI** (paridade total com `cosca gate catalog`): `cosca gate recall --audit --strict`, com `--check` (default, drift do baseline, não roda recall), `--generate` (regenera se o baseline mudar), `--summary`, `--json`.
- **Fail-closed**: `queries_errored > 0 ⇒ Passed=false`. Query que lança é `errored` e vira breach — nunca descartar para inflar a nota (postura do Mega Brain C6).
- **Métricas zero-LLM**: `recall@K` (chunk_id interseção), `first_relevant_hit`, `nDCG@k` (premia posição), `id_based_context_precision`.
- **Piso default proposto**: `recall@5 >= 0.60`, `first_relevant_hit >= 0.80`, `queries_errored == 0` (env `COSCA_RECALL_FLOOR_*` para override).
- **Quem executa o retrieval**: `Retriever func(ctx, query string, k int) ([]string, error)` retornando `chunk_id` ordenados — o CLI injeta `knowledge.Engine.Search` (ou `vector.Store.Search`), o gate permanece puro/determinístico e testável.

**C2 — invariante de espaço único + quarentena por assinatura:**

```
internal/embeddings/
  signature.go      // func Signature(model string, dim int) string  → "nomic-embed-text:768"
```

- **Cache**: `embeddingCache.key(text)` deixa de ser `sha256(text)` e passa a `sha256(sig + "|" + text)` — **evita servir embedding de outro modelo no mesmo cache** (`sig + model + dim`). Mudou modelo ⇒ bake-up no cache (fail-seguro, não mistura).
- **VectorRecord / store**: registrar a assinatura. `vector.SQLiteVecConfig` ganha um campo `EmbeddingSignature`; `knowledge.Init` passa `embRegistry.Model()+":"+embRegistry.Dimensions()`.
- **Quarentena**: `knowledge.Verify()` (que já checa dimensão/órfãos/dangling) ganha `embedding_space_invariant`: se um vetor persistido tem assinatura ≠ ativa ⇒ **mismatch** reportado e **reuso vetado** (como `_load_prior_vector_map` do C2). Reuso exige `content_sha` + assinatura (o Cosca já tem `cachefingerprint.go` + `campaign_reembed.go` — estender, não criar).
- Reuso do re-embed: `RepairOrphanVectors()` já existe; criar `ReembedMismatchedSignature()` análogo (fase 2, sob `cosca knowledge verify --fix`).

**C7 — evidência atômica nos edges + BrainHealth (fase posterior):**
- `internal/graph` edges hoje não têm `atomic_facts`. Ação bounded futura: adicionar `AtomicFacts []string` (frases-fonte verbatim) na `Edge`; `BrainHealth` 6-métrica (staleness, chunks órfãos, dead_links, consistência de dimensão, invalidação de cache de RRF) como `cosca monitor` / fail-**warn** (não block). **Não entra na fatia 1.**

### 3.5 Config / Feature-gates (kill-switch com precedência — C3)

```
COSCA_GROUNDING_ENABLED         (default: false — opt-in; 0 = comportamento atual)
COSCA_GROUNDING_THRESHOLD       (default: 0.60)
COSCA_GROUNDING_HHEM_ENABLED    (default: false — fase 2; 0 = heuristic-only)
COSCA_RECALL_FLOOR_RECALL       (default: 0.60)
COSCA_RECALL_FLOOR_FIRST        (default: 0.80)
```

Precedência: env > config > default (paridade `MCE_RERANKER_ENABLED`).

---

## 4. Impacto e o que NÃO muda

**Muda (aditivo):**
- Novo pacote `internal/grounding` + `internal/embeddings/signature.go` + `internal/cli/recall_gate.go` (+ subcomando `cosca gate recall`).
- `knowledge.Engine` ganha **fachadas aditivas** (`SearchGrounded`, `ReembedMismatchedSignature`) — o caminho existente (`Search`, `Verify`, `Init`) permanece.
- `VectorRecord`/`SQLiteVecConfig`/cache de embedding ganham campo de **assinatura** (desativável/compatível: assinatura vazia = "sem invariante", modo atual).

**NÃO muda (para não inflar o incremento):**
- `internal/search.Search` (retrieval) — intocado.
- `internal/ranking.Ranker` (merge híbrido/C1) — intocado, já satisfeito.
- `internal/skilleval` e `internal/evals` (orquestração) — intocados; o RecallGate é **paralelo**, não acoplado.
- `internal/catalog` — intocado; usado só como **precedente** de forma do novo `cosca gate recall`.
- `internal/gate` (plan-approval) — intocado.
- `ClaimStore`/`thinking.go`/`evidence.go` — intocados; **reusados** como building blocks.

**Impacto operacional:**
- CI ganha um second gate determinístico (`cosca gate recall --audit --strict`), espelhando o `catalog`.
- A busca híbrida continua **com o mesmo custo** quando `COSCA_GROUNDING_ENABLED=false` (zero regressão de latência/arquitetura).

---

## 5. Trade-offs / Riscos

| Risco | Mitigação |
|---|---|
| **Falso-positivo de block** (heurístico marca "não-sustentado" o que é legítimo). | Threshold 0.60 é conservador; o veredito padrão é **FLAG** (entrega com aviso), BLOCK só abaixo do threshold; `blocked_response` preservada p/ auditoria. Calibrar o threshold contra `qrels-baseline` na fatia 1. |
| **Falso-negativo** (alucinação passa no token-overlap). | É o limite do heurístico — por isso o **HHEM condicional** é fase 2, gated por evidência de resíduo. Não esconder o limite: documentar. |
| **Qrels-baseline vira fonte de overfit** (tunagem para "ganhar no teste"). | `qrels` é **congelado/commitado**; mudanças exigem `--generate` + PR + justificativa. `first_relevant_hit` + `queries_errored=0` punem "vender" recall fácil. |
| **Embedding mudou silenciosamente** (modelo novo, mesma dimensão). | Assinatura `model:dim` no cache + `embedding_space_invariant` (mismatch reportado, reuso vetado). **Custo**: re-embed quando a assinatura muda (uma vez, operacional). |
| **HHEM local adiciona 400MB + CPU** a um pbitol local-first. | **Não entra na fatia 1.** Só se `COSCA_GROUNDING_HHEM_ENABLED=true` E o A/B em `qrels` mostrar resíduo real. Honestidade: o argumento do Mega Brain é "zero custo API" — para o Cosca, o custo é **implantação + latência**, e isso é o que evitamos. |
| **Over-engineering dos dois blocos de uma vez** (escopo). | Fatia 1 = heurístico + atribuição por claim + qrels simples + assinatura no cache. HHEM, atomic_facts/BrainHealth, re-ranker = fase 2+. |
| **Complexidade na integração do chamador** (ainda não há answer builder). | A facada `SearchGrounded` + `Verdict()` isola o chamador; quem monta contexto decide aplicar `block`/`flag`. Sem mudar o engine. |

---

## 6. Fronteira do incremento — Fatialh 1 recomendada (valor isolado primeiro)

**Escopo bounded** (o que entrega anti-alucinação IMEDIATA, sem peso):

1. **`internal/grounding` heurístico** (C4): `ExtractClaims` + `VerifyClaim` (token-overlap + bônus n-grama/número) + `FidelityReport` + `Verdict()` com `DefaultFaithfulnessThreshold=0.60`, **`faithfulness == -1 nunca block`** (fail-open C3), BLOCK/FLAG no chamador.
2. **Atribuição por claim** (C5): cada claim com `source_chunk` (`chunk_id`) + `matching_terms`; **"sem evidência não gera"** no `BuildAnswer` (`len(chunks)==0` ⇒ extrativo/degradado, nunca paramétrico).
3. **`qrels-baseline.json` simples** (C6): ~20–30 queries autorais sobre a base real do Cosca; `cosca gate recall --audit --strict` (fail-closed `queries_errored>0`, piso `recall@5>=0.60`, `first_relevant_hit>=0.80`), determinístico, espelhando `cosca gate catalog`.
4. **Assinatura de embedding** (C2): `internal/embeddings/signature.go` + cache chaveado por `sig|text` + `embedding_space_invariant` no `Verify()` (fail-warn na fatia 1; fail-block na fatia 2 se desejado).

**Critério de saída da fatia 1:** `go test ./internal/grounding/... ./internal/embeddings/...` + `cosca gate recall --audit --strict` verde no CI, sem regressão em `go test ./...` e **zero mudança** em `search`/`ranking`/`knowledge.Search` (exceto a facada aditiva).

**Gate de custo explícito (o que está FORA):**
- **HHEM NLI local** — fase 2, condicionado a `COSCA_GROUNDING_HHEM_ENABLED=true` + A/B em `qrels` mostrando resíduo de infidelidade maior que o tolerado. ~400MB + tokenizer ONNX + inferência CPU; não vale começar por ele.
- **Cross-encoder reranker** + RRF/cosine re-score (C1/hybrid) — **já coberto** pelo `ranking.Ranker`; diferido.
- **atomic_facts nos edges + BrainHealth 6-métrica (C7)** — fase 3, monitor-only (fail-warn), pois o grafo não tem `atomic_facts` hoje.
- **Re-embed automático de assinatura divergente** — fase 2 / `cosca knowledge verify --fix`, operacional.

---

## 7. Alternativas Consideradas

| Alternativa | Veredito |
|---|---|
| **HHEM NLI local como fase 1** (cascata direta self-RAG→HHEM). | **Rejeitado para fatia 1.** Custo de implantação/latência (400MB/CPU) para um bitol local-first. Adotado só como fase 2 condicional. |
| **LLM como juiz de fidelidade** (um LLM classifica se a resposta é sustentada). | **Rejeitado.** Custo de API/latência, não-determinístico, anti-pattern "usar LLM para validar LLM" sem âncora. Coincide com o postura "zero achismo" — prefere-se gate determinístico + heurístico zero-LLM. |
| **Apenas atribuição por claim, sem gate** (só citar, não bloquear). | **Rejeitado como suficiente.** Só citar não impede alucinação; o bloco 1 tem BLOCK/FLAG. |
| **Só gabarito congelado, sem gate (baseline passivo)**. | **Rejeitado.** Sem `queries_errored>0 ⇒ fail` e piso de recall, o baseline não protege — vira um relatório morto. Fail-closed é o que dá valor. |
| **Copiar RRF+cosine re-score + cross-encoder (C1) junto.** | **Adiado.** Cosca já tem merge híbrido em `ranking.Ranker`; o cross-encoder é a mesma classe de custo do HHEM. Só entra se recall de `qrels` indicar. |
| **Recriar máquina de claims** (novo tipo em `grounding` em vez de reusar `knowledgement.ClaimStore`). | **Rejeitado.** O `ClaimKind`/`Classify`/`ClaimStore` já existem e são determinísticos; reusá-los é P8 (economia, consistência, auditoria unificada). |
| **Acoplar RecallGate ao `internal/evals` (harness de orquestração)**. | **Rejeitado.** `evals` é para cases de pipeline/verify; recall de retrieval é outra métrica. Colocar em `internal/grounding` evita embaralhar dois conceitos. |
| **Derrubar busca quando uma fase falha** (fail-closed no retrieval). | **Rejeitado.** Violaria o C3 (fail-open da busca). O fail-closed é **só** no gate de fidelidade (score real baixo) e no gabarito (query erroda), nunca no caminho de retrieval em si. |

---

## 8. Related

- `.cosca/fallback/knowledge/patterns/mega-brain-patterns.md` — C1–C7 RAG grounded (fonte).
- `docs/adr/ADR-002-knowledge-engine.md` — Knowledge Engine + hybrid search (baseline).
- `internal/search/search.go`, `internal/vector/vector.go`, `internal/embeddings/embeddings.go` + `provider.go` — stack que o ADR envolve.
- `internal/knowledge/thinking.go` — `ClaimStore`/`ClaimKind`/`IsTrustworthy` (reuso).
- `internal/knowledge/evidence.go` + `provenance.go` — trilha "zero achismo".
- `internal/catalog/catalog.go` + `internal/cli/catalog_gate.go` — precedente de gate determinístico fail-on-invariant (`cosca gate catalog --audit --strict`, invariante D mojibake).
- `internal/skilleval/regression.go` — `RegressionGate` (holdout), análogo ao `RecallGate`.
- `internal/evals/metrics.go` + `oracle.go` — métricas puras + oráculo.
- `.opencode/cosca/workflows/skill-evaluate.md` — meta-loop A/B que o ADR espelha para justificar gate por evidência.

---

> **Decisão pendente de revisão (cosca-cto + Don).** A fatia 1 é bounded e dá valor isolado imediato: anti-alucinação heurística zero-LLM + atribuição por claim + gabarito congelado + assinatura de embedding, sem HHEM pesado (fase 2 condicionada).
