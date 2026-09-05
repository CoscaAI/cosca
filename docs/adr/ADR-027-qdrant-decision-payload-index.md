# ADR-027: NÃO adotar o Qdrant como engine vetorial — absorver o payload index + facets SQL no `internal/knowledge`

> **Status:** Accepted ✅ | **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-08-29
> **Revisão:** formalizado por ordem do Don ("criar o ADR" pós-mineração). **Design aditivo — não introduz infra,
> não quebra o kernel.** É uma decisão de **manter a arquitetura atual** e **absorver boas práticas** — não implementa Qdrant.
> **Referência (base):** mineração de **código** (não README) de `qdrant/qdrant` (v1.19.0, Apache-2.0, ~34k⭐, clone em
> `Temp/opencode/qdrant`) + `ADR-002` (Knowledge Engine) + `ADR-013` (Bancos de Dados Modulares) + `ADR-017` (Borrowing
> Protocol) + auditoria de `internal/knowledge` (Search, filtro por payload) e `internal/vector` (índice in-memória).

---

## 0. Contexto — a dúvida do Don

O Don ficou curioso sobre o **Qdrant** — o vector database Rust mais popular ("o vetor-first, payload como metadado de
filtro, standalone ou edge"). A pergunta implícita: *"estamos deixando performance/de capacidade na mesa ao não usar um
vector DB dedicado?"*

A mineração de código (três capos: Architecture, Backend, Database) responde com um **mapa honesto** que muda a forma da
dúvida:

| Frente | Estado medido hoje (Cosca) | Par no Qdrant |
|---|---|---|
| **Busca híbrida** | `internal/knowledge` = FTS5 (BM25) + vetorial + grafo, numa query | hybrid search (RRF/DBSF, `prefetch`) |
| **Índice vetorial** | `internal/vector` = índice in-memória **quantizado** (int8/int16, AVX2, SoA, snapshots imutáveis, recall@10 **0.99**) | HNSW + quantização (`quantized_vectors.rs`) |
| **Latência atual** | **28.888 vetores → ~10ms p50** (dentro do alvo `<20ms` da ADR-002) | sub-ms em corpo pequeno |
| **Filtro por payload** | `json_extract(metadata_json,'$.epistemic')` **SEM índice** → scan completo | **payload index** (`Keyword/Integer/...` 3 backends) |
| **Leitura unificada** | grafo + FTS5 + epistemologia (CKL) + proveniência **na mesma query** | NÃO faz (vetor-first; payload é filtro, não grafo/epistemia) |

**A conclusão da mineração:** o Cosca **já faz o núcleo** (híbrido + índice quantizado com recall 0.99), e está em
**~2,8% do joelho de performance** (28k vetores de ~1M) — onde **ANN/HNSW sequer importa**. O **déficit real** não é
ANN, é **filtro por payload sem índice** — exatamente o que o *payload index* do Qdrant resolve.

---

## 1. Decisão

**NÃO adotar o Qdrant como engine vetorial agora.** Manter o `internal/knowledge` (FTS5 + vetor + grafo, SQLite
através de `modernc.org` — Go sem CGO, single-binary) como a **fonte única** da busca, e **absorver como melhorias
aditivas** as duas boas práticas que a mineração provou serem os "ativos reais" para o nosso desenho:

1. **Payload index (SQLite)** — colunas **materializadas** + **B-tree** para `epistemic`, `tier`, `document_id`,
   `entity_type` (e o estimador de cardinalidade que decidir a ordem **filter→vector vs vector→filter**).
2. **Facets via SQL** — `GROUP BY`/`COUNT` para clusterizar/contar por dimensão, em vez de **pós-corte sobre o top-N**
   (hoje os facets são estimados de um subconjunto já ordenado, o que distorce a contagem).
3. **Fusão determinística ajustável** — **RRF** (default `k=2`, weights por fonte) e **DBSF** (`distr_norm` Welford 3σ /
   `min_max_norm`, Agregação `Sum`), + `score_threshold`/`offset` consistentes por fonte. Zero LLM, ~60 linhas, **I1**.
4. **Rescore em 2 estágios** (opcional) — um cross-encoder via seam `internal/embeddings`, **default off** (preserva I1).

**A decisão pode ser REABERTA** se os gates abaixo forem atingidos (§5).

### 1.1 Por que "manter" é a decisão de arquitetura (e não a ausência de decisão)

A mineração encontrou a **arquitetura do Qdrant oposta à do Cosca** — então a resposta à dúvida do Don não é "qual
vector DB instalar", é **"qual dos dois modelos de dados é o nosso?"**. Qdrant é **storage-first de vetores** (o payload
é metadado de filtro **colado**, escrito em volta do vetor). O Cosca é **document-first** (o conteúdo é a fonte; o vetor é
um índice derivado, reindexável, e o payload é **epistemologia real** — CKL/`EvidenceClass`/proveniência — não uma tag
de filtro). Trocar de modelo de dados **quebraria** a vantagem da **leitura unificada** (§2.3).

---

## 2. Racional

### 2.1 — O joelho de performance: por que ANN/HNSW **não é** o problema hoje

O argumento de "precisamos de um vector DB porque o index vetorial vai ser lento" só se sustenta **acima do joelho**:

| Cenário | Nº de vetores | Slab | Latência no Cosca | ANN/HNSW importa? |
|---|---|---|---|---|
| **Hoje** | 28.888 | ~32 MB (768 dims × 4 B) | **~10ms p50** (≈28k itens, full-scan + dot) | **Não** — full-scan resolve |
| **Joelho** | ~1M | **~768 MB — sai do L3** | **~10×** a latência (cache miss + I/O de página) | **Sim** — é aqui que HNSW entra |
| Alvo ADR-002 | — | — | `<20ms p50` | — |

Somos **~2,8% do joelho** (28k/1M). A **autópsia** (`internal/vector`) já provou que o gargalo atual **não é o dot**, é o
**re-decode** do store via SQLite; e isso **já foi corrigido** com o índice quantizado in-memória (recall@10 **0.99**,
int8/int16 AVX2). **Adicionar Qdrant agora resolveria um problema que não temos** — e criaria o problema de infra.

### 2.2 — O déficit real: filtro por payload **sem índice**

O único ganho concreto que a mineração isolou está no **filtro por payload**. Hoje, filtrar por epistemologia/tier é um
**scan**, sem índice:

```sql
-- internal/knowledge/knowledge.go (Search → enrichEpistemic)
SELECT id, path, json_extract(metadata_json, '$.epistemic') FROM documents WHERE ...
-- internal/vector/sqlite_vec.go (filterClause, aplicado a CADA row candidata)
AND json_extract(metadata, '$.epistemic') = ?          -- SEM índice → full scan
AND json_extract(metadata, '$.tier')     = ?           -- idem
```

O Qdrant, nesse ponto, faz algo que não fazemos: mantém um **índice por campo de payload** (`Keyword/Integer/Float/Text/
Bool/Datetime/Uuid`, cada um em 3 backends RAM/mmap) e um **query planner por cardinalidade** (`query_estimator.rs`) que
estima quantos pontos um filtro seleciona (`must=produto`, `should=prob-complementar`, `min_should=combinações`) e decide
**filter→vector vs vector→filter** conforme a seletividade da condição primária. **Isso não exige infra nova**: é uma
**coluna materializada + B-tree** num SQLite que **já** temos.

### 2.3 — Por que NÃO adotar o Qdrant **agora**

| Motivo | Detalhe do conflito | ADR afetada |
|---|---|---|
| **Quebra local-first / zero-external-dep** | O cliente Go do Qdrant é maduro (gRPC, sem CGO), mas **exige um servidor separado** → quebra a "zero-infra" e a leitura unificada num arquivo. | **ADR-002** (local-first, zero dependency) |
| **Quebra single-binary / <100MB** | Standalone = processo à parte + Raft/sharding. `vector.db` já está ~88% do teto de 100MB (Decisão 1 do Don); somar um serviço vetorial **inverte** a modularização. | **ADR-013** (módulos <100MB, single-binary) |
| **Perde a leitura unificada** | Grafo + FTS5 + epistemologia (CKL) + proveniência na **mesma query** é o **valor central** do Cosca. Qdrant é **vetor-first** — o payload é tag de filtro, **não** grafo/epistemia/proveniência. | **ADR-002 / ADR-013** |
| **Qdrant Edge é inviável** | O "SQLite de vectors" embutido (`lib/edge`, `EdgeShard`) só existe em **Rust/Python**, **sem binding Go**, e está **beta**. Inviável para um Cosca **Go sem CGO**. | — |
| **Over-engineering** | Portar Qdrant (default) → **Go** (Rust → Go) é reescrita maciça de anéis de implementação para um problema que não temos (estamos a ~2,8% do joelho). | — |

### 2.4 — O que absorver (sem infra nova)

Tudo o que absorvemos é **aditivo** e fica dentro de `internal/knowledge` + `internal/vector`:

1. **Colunas materializadas + B-tree** em `documents`/`chunks`/`vectors` para `epistemic`, `tier`, `document_id`,
   `entity_type` (derivadas de `metadata_json`, preenchidas por **backfill idempotente** — o que já existe para o
   `epistemic` na Fase 4). Índice `CREATE INDEX ... ON ... (epistemic)` — **filtrar deixa de ser scan**.
2. **Estimador de cardinalidade + plano de ordem** — replicar a heurística do `query_estimator.rs` (~60 linhas,
   puro `math`, **I1**): se `filter` for muito seletivo → **filter→vector** (reduz o pool antes do dot); senão →
   **vector→filter**. Hoje o filtro é aplicado **a cada row candidata** (pior caso).
3. **Facets via SQL** — em vez de pós-corte sobre o top-N (que distorce a contagem), `SELECT epistemic, COUNT(*) FROM
   documents WHERE <filtro> GROUP BY epistemic`. Correto e barato com o índice.
4. **Fusão determinística** — **RRF** com `k=2` e weights por fonte; **DBSF** com normalização por distribuição
   (`distr_norm` Welford 3σ / `min_max`) e `Aggregation::Sum`; `score_threshold`/`offset` consistentes por fonte.
5. **Rescore em 2 estágios** (opcional, adaptador de `internal/embeddings`, **default off**) — cross-encoder só se
   o usuário habilitar; preserva **I1** (o caminho default é determinístico).

---

## 3. Alternativas rejeitadas

| Alternativa | Veredito |
|---|---|
| **(a) Qdrant standalone** (servidor gRPC/REST + Raft/sharding) | 🔴 **Rejeitado** — quebra zero-infra e a leitura unificada; viola ADR-002 (local-first) e ADR-013 (<100MB, single-binary). Só se os gates do §5 forem atingidos e houver design de "serviço de busca" separado aprovado pelo Don. |
| **(b) Qdrant Edge** (embutido, "SQLite de vectors") | 🔴 **Rejeitado** — o ideal conceitualmente, mas é **Rust/Python**, **sem binding Go**, e **beta**. Inviável para o Cosca Go-sem-CGO e para o requisito de single-binary. |
| **(c) Manter exatamente como está** (não absorver nada) | 🟡 **Parcialmente** — rejeitado como "não fazer nada": perderíamos o **payload index** (o único ganho real isolado) e os **facets corretos**. Mantemos a arquitetura, mas **absorvemos** as melhorias (a decisão é "mesma arquitetura + melhorias", não "tudo igual"). |
| **(d) Portar Rust → Go** (reimplementar Qdrant como lib) | 🔴 **Rejeitado** — over-engineering. Reescrita de um sistema de vector-first para o nosso modelo document-first, a 2,8% do joelho. O ativo real (query planner + fusão) é **adaptável em ~60–200 linhas**, não portável. |
| (incidental) HNSW/quantização on-disk/GPU WGPU/io_uring do Qdrant | 🔴 **Rejeitado** — são otimizações **acima do joelho** (≥1M vetores). Não tocamos até os gates. `io_uring` é Linux-only; `gpu` é feature flag. |

---

## 4. Consequências

### Positivas

- **Elimina o pior caso de filtro** — `epistemic`/`tier`/`document_id`/`entity_type` deixam de ser full-scan; o estimador
  de cardinalidade escolhe a ordem certa (filter→vector vs vector→filter).
- **Facets corretos** — contagem por dimensão via `GROUP BY`/`COUNT`, não pós-corte sobre o top-N.
- **Sem infra nova** — zero servidor, zero dependência externa, zero processo; tudo em SQLite (binário único). Mantém
  **ADR-002** e **ADR-013** intactos.
- **Mantém a leitura unificada** — grafo + FTS5 + epistemologia + proveniência na **mesma query** (nossa vantagem, que um
  vector-first não dá).
- **Fusão determinística** (RRF/DBSF) melhora o *ranking* da busca híbrida existente, sem LLM, **I1**.

### Negativas

- **Continua fazendo scan em filtro pesado até materializar as colunas** — o ganho só aparece após o **backfill
  idempotente** criar as colunas + B-tree; antes disso, filtro denso é o comportamento atual.
- **Não temos ANN/HNSW além de ~1M vetores** — o full-scan quantizado é ótimo **abaixo do joelho**, mas acima dele a
  latência cresce ~10×. Aceito conscientemente porque estamos a 2,8% do joelho; o gate §5 reabre a discussão.
- **A leitura unificada continua sendo nossa vantagem, mas é a razão de não adotar Qdrant** — se o Don esperava um
  vector-first com payload como "tag", isso **não** será entregue; entregamos a mesma cobertura de filtro via SQLite.

### O que NÃO muda

- **`internal/oracle`** (semântica + gate fail-closed) — intocado; é quem valida contra a âncora.
- **`internal/vector`** (índice quantizado in-memória, recall@10 0.99) — intocado na essência; só recebe o plano de
  ordem (2.4.2).
- **Modelo de dados document-first** — o conteúdo continua sendo a fonte; o vetor continua sendo índice derivado.
- **Invariantes I1–I8** — nenhum item absorvido introduz LLM em caminho default (I1), externo como autoridade (I8) ou
  runtime novo no núcleo (I7).

---

## 5. Gates para REABRIR a decisão (regra de reavaliação)

A decisão é **`Accepted`** e **não** é definitiva. Ela é reaberta (e aí o Qdrant standalone/edge merece um ADR de design
próprio) se **qualquer** dos gates for atingido:

| Gate | Métrica | Sinal |
|---|---|---|
| **G1 — Volume** | vetores indexados **> 500k** | full-scan quantizado degrada; hora de avaliar HNSW/ANN |
| **G2 — Latência com filtro** | query de busca **com filtro `epistemic=`/`tier=`** com p95 **> 20ms** | o payload index + B-tree não bastou; avaliar serviço vetorial |
| **G3 — Facets sobre corpus** | facets exigidos como **requisito funcional** sobre o corpus inteiro (não sobre top-N) | exige índice de payload dedicado num volume grande |
| **G4 — Multivetor / multitenancy física** | necessidade de **multi-vector** por item ou **isolamento físico** por tenant | recomputar para um modelo vetorial dedicado |
| **G5 — Joelho** | atingir **~1M vetores** (slab saindo do L3) | ANN/HNSW deixa de ser opcional |

---

*Autor: cosca-architecture (Architecture Chief). Decisão fundamentada em mineração de código de `qdrant/qdrant` +
auditoria de `internal/knowledge`/`internal/vector` + invariantes ADR-002/ADR-013/ADR-017. O Cosca **não precisa** de um
vector DB agora — precisa de **índice de payload (colunas materializadas + B-tree), facets SQL e fusão determinística
(RRF/DBSF)**, tudo dentro do SQLite que já temos. O Qdrant é um **modelo de dados oposto** (vetor-first) que quebraria a
leitura unificada; a decisão é **manter a arquitetura e absorver as boas práticas**, reabrindo a discussão só se os gates
G1–G5 forem atingidos.*
