# Vector Recall — Baseline do Índice (estado REAL, pré-transformação)

> Relatório técnico · 2026-08-24 · **READ-ONLY** — nenhum dado alterado, nenhum
> DELETE, nenhum re-embed, nenhuma reindexação. O `knowledge.db` de produção foi
> aberto em `mode=ro`; nenhuma escrita foi emitida.
>
> Objetivo (Don): **"deixar perfeito"** o ranking. Este relatório mede o
> **estado ATUAL** do índice vetorial de busca semântica como linha-de-base
> imutável, ANTES de qualquer transformação. Complementa (e usa os mesmos dados)
> o diagnóstico paralelo de pipeline/Ranking:
> `docs/reports/vector-ranking-diagnosis-2026-08-24.md`.

---

## 0. Método e honestidade (o que foi, e o que NÃO foi, medido)

Instrumento: `internal/vectorbaseline/baseline_test.go` (novo, test-only).
Abre o `knowledge.db` com `modernc.org/sqlite` em `?mode=ro` e **replica em Go
puro** os três caminhos de score do `SQLiteVec`:

| Caminho | Fórmula | Papel |
|---|---|---|
| float32 | `cos = dot/(‖q‖·‖r‖)` | **ORACLE exato** |
| int16 | `dot16/(1024²·‖q‖·‖r‖)` | **default de produção** |
| int8 | `dot8/(127²·‖q‖·‖r‖)` | fast path antigo |

A replicação em inteiro **i64** é bit-idêntica ao kernel AVX2 (`dot16Pure` é a
referência portátil; o acumulador não estoura em dim 768; o bias `128·Σr` do
int8 é reproduzido). Assim medimos recall/NDCG/distribuição **sem** abrir o banco
em modo de escrita e **sem** confiar no provider de embedding para a métrica de
recall.

- Embeddings de **queries reais** (self-match de texto) foram gerados com o
  provider local `nomic-embed-text` em `localhost:11434` — **disponível** durante
  a medição (0 erros). Não foram usados mocks.
- **NÃO** foi medido: recall de um conjunto de consultas humanas rotuladas (não
  existe ground-truth anotado no repo); impacto pós-deduplicação (demandaria
  mutação). Fronteira rígida respeitada.

---

## 1. Estado do índice

```
vetores (dim válidos)      = 13.217
dimensão                   = 768 (float32 BLOB, LittleEndian)
linhas com dim-mismatch     = 0   (nenhuma é pulada no loadGeneration)
tipo de índice             = flat_bruteforce  (VectorStats().IndexType)
path do DB                 = .cosca/knowledge.db
```

- `VectorStats()` devolve `IndexType: "flat_bruteforce"` (hardcoded em
  `internal/vector/sqlite_vec.go:827`) — confirmado por leitura de código. Não há
  índice ANN (HNSW/usearch/faiss): a busca é full-scan (in-memory, otimizado).
- **Caminho de produção:** `internal/vector/index.go` `search()` roteia
  `int16 (default) → int8 → float32`. O `Config` do engine
  (`knowledge.go:272-275`, flags default) deixa `DisableInt8=false` e
  `DisableInt16=false` ⇒ **int16 é o caminho ativo**, com int8 como fallback e
  float32 como oracle. FASE A/L361 confirma: int16 = default de produção.

### Footprint estimado do índice in-memory (relevante p/ regime L3)

| Slab | bytes/elem | tamanho (@13.217×768) |
|---|---|---|
| vecs (float32) | 4 | 40.6 MB |
| vecs16 (int16, pad 32) | 2 | 21.1 MB |
| vecs8 (int8, pad 16) | 1 | 10.4 MB |
| nb + sums8 + ids | ~4 | ~1.2 MB |
| **total** | | **≈ 73 MB** |

Está **abaixo** do budget L3 de 80 MB (regime cache) — coerente com a conclusão
da FASE 1 (13.8k vetores cabe no V-Cache). O full-scan não é o gargalo hoje; o
gargalo é **qualidade** (rank/duplicatas), não throughput.

---

## 2. Frequência de duplicatas

Método: `sha256` do BLOB (vetor) e hash do `content` (texto), read-only.

### Duplicatas por vetor (cosseno exato 1.0 ⇒ BLOB idêntico)

```
distintos      = 10.510
duplicados     =  3.039   (23.0% do índice)
MAX cópias     =  61  (um único BLOB tem 61 cópias — hash 92f85b6a…)
```

### Duplicatas por conteúdo (cross-documento)

```
content vazio                    =    0   (todos os vetores têm texto)
conteúdos distintos (com texto)  = 10.525
linhas duplicadas (2ª+ ocorrência)=  2.692   (20.4%)
grupos cross-documento (>1 doc)  =    239
MAX cópias de UM conteúdo        =   61
MAX documentos de UM conteúdo    =   61
linhas em grupos >10 cópias      =  2.168   (16.4% do índice)
mediana de cópias por conteúdo   =    1
```

**Exemplo de boilerplate duplicado (61 cópias, 61 documentos):**
> `**📖 Leia o [AGENT_PRIMER.md](../AGENT_PRIMER.md) antes de agir.**`

**Leitura:** o corpus tem **~20–23% de duplicatas**, e a duplicação **não é
aleatória** — concentra-se em bloilerplate transversal (preâmbulos/instruções
compartilhados por muitos documentos) e em 239 grupos que atravessam **vários**
documentos. O dedup **não está ativo** neste DB: as colunas `is_trivial`/
`dedup_of` **não existem** (confirmado em `schema.go:62-73` e por PRAGMA no
diagnóstico paralelo), então o fail-safe do indexer (`indexer.go:735-736`)
desliga o dedup preventivo de ingestão. O índice atual está **menos** duplicado
que em 2026-08-17 (34% → 23%), mas ainda carrega o problema estrutural.

---

## 3. Recall@K e NDCG@K vs. ORACLE float32

Método (o mesmo da FASE 8/L346): **self-match** — query = vetor real de um chunk
do corpus; oracle = top-K float32 exato; comparação determinística com desempate
`(score desc, id asc)`. 30 queries de conteúdo **distinto**, K=50, dim 768.

| Caminho | set-recall@1 | @5 | @10 | @50 | ranking-recall@10 | NDCG@10 | jaccard@50 | distintos@50 |
|---|---|---|---|---|---|---|---|---|
| **int16 (produção)** | **1.0000** | 1.0000 | 1.0000 | 0.9960 | **1.0000** | **1.0000** | **0.9922** | 0.8626 |
| int8 (fast path antigo) | 1.0000 | 1.0000 | 1.0000 | 0.9633 | 0.9700 | 1.0000 | 0.9311 | 0.8276 |

```
top-1 mudou:            int16=0/30 (0)   int8=0/30 (0)     ← o top-1 é sempre preservado
MAX abs score err:      int16=0.001528 (mean 0.000376)
                        int8 =0.015625 (mean 0.002766)
quase-empates (≥0.99) no oracle top-50 = 21.5% do total
diversidade do top-10 (oracle): 7.33 conteúdos distintos / 7.57 documentos (de 10)
```

**Leitura — a quantização NÃO é o problema:**

- **int16 (produção) é praticamente bit-fiel ao float32**: `ranking-recall@10=1.0`,
  `NDCG@10=1.0`, `jaccard@50=0.992`, `top-1 nunca muda`, erro médio 0.0004. A
  busca atual, como **índice**, é excelente.
- **int8** perde um pouco no @50 (`0.9633`, ranking-recall@10 `0.97`) e erra mais
  (0.0156/0.0028), mas preserva o top-1. Consistente com os valors históricos
  (0.8840 sintético / 0.978 determinístico real); a diferença para o "0.934" citado
  advém de métrica/corpus distintos (o corpus aqui está menos duplicado).
- **O score não é o que degrada.** `distintos@50` (0.86) é O sinal fraco: quando
  você **exclui a loteria das duplicatas** (itens ≥0.99), a concordância int16 cai
  de jaccard 0.99 para **0.86** — porque o conjunto de vizinhos **distintos** é
  minado pelas duplicatas.

---

## 4. Distribuição de scores (o "ranking achatado")

Distribuição do **top-10** (oracle float32), 30 queries de conteúdo distinto:

```
score top-10:  min=0.6189   max=1.0000   mean=0.8461   med=0.8031   sd=0.1254
GAP top1→top10: min=0.0000  max=0.3811   mean=0.1904   med=0.2476   sd=0.1230
ranks 2..10 do top-10 com score ≥0.99 do top-1: 69 de 270 (25.6%)
quase-empates (≥0.99) em todo o oracle top-50: 21.5%
```

**Leitura — achatamento é real, porém BIMODAL:**

- Para a maioria das queries (conteúdo distinto) há **espaço**: `med=0.80`, gap
  mediano `0.25`. O top-10 não é todo 1.0.
- Mas há uma **segunda população**: **25.6%** dos ranks 2–10 estão a ≥0.99 do
  top-1, e o **gap mínimo = 0.0** — existem queries cujo top-10 inteiro colapsa
  em ~1.0 (as que batiram em conteúdo duplicado/boilerplate). Nelas o ranking é
  uma **loteria** (o desempate `id asc` decide, deterministicamente, mas sem
  significado semântico).

---

## 5. Real-text self-match (nomic-embed-text via 11434)

Re-embedei o **texto real** de 25 chunks e busquei; mede se o próprio chunk sobe
a top-1:

```
self-match top-1 (próprio chunk = top-1): 76.0%
score top-1 médio:                        1.0000
score do PRÓPRIO chunk (médio):           0.8400
falhas top-1:
   colisão de duplicata (score próprio ≥0.99) = 8.0%
   score próprio < 0.9 (vetor de OUTRO provider) = 16.0%
   outras = 0.0%
tempo médio por busca (inclui embed):   ~27.5 ms
```

**Leitura — a falha de self-match é ⅔ vetor de outro provider:**

Só **76%** dos chunks reencontram a si mesmos — apesar de o top-1 ter *sempre*
score 1.0. A falha se decompõe em:

- **8%** — colisão de duplicata: o próprio chunk tem score ≥0.99, mas um vizinho
  quase-idêntico ganha o desempate (`id asc`). Sem impacto semântico real.
- **16%** — o **score do próprio chunk é <0.9**: o vetor armazenado daquele chunk
  **não foi gerado pelo nomic** (provider/normalização diferentes de uma ingestão
  anterior). Re-embedando com nomic, o texto não encontra a si mesmo. **Este é o
  sinal de contaminação de provider** (hipótese L368 confirmada neste corpus).

---

## 6. Assinatura do score atual

**O que o usuário vê:** `r.Score` = **cosseno bruto** da camada vetorial, em
[0,1] (nomic normaliza), sem calibração, sem normalização entre fontes e **sem**
o score multi-fator. O diagnóstico paralelo confirma: no caminho `layered`, o
ranker multi-fator nunca roda e o score final (se calculado) é **descartado**;
`mergeRanked` ordena por `Score` cru.

**Por que o ranking está achatado (4 sinais, todos medidos):**

1. **Corpus homogêneo + duplicado.** Embeddings de documentação/confs são
   próximos; com ~20% de conteúdo duplicado, muitas embeddings colidem em ~1.0.
2. **Score não-calibrado por query.** O cosseno cru não é transformado por
   query (sem `temperature`/`softmax`/contrastivo), então a banda do top-k se
   comprime: `max=1.0`, `mean=0.85`, mas **25.6%** dos ranks 2–10 a ≥0.99 do
   top-1 e **gap mínimo = 0.0**.
3. **Escalas misturadas sem normalização** (BM25 pode passar de 1.0; cosseno é
   [0,1]) — `mergeRanked` ordena o cru → a ordem entre fontes é arbitrária.
4. **Desempate por `id asc`** entre scores ≥0.99: deterministicamente estável,
   semanticamente **sem significado**. É o que decide a ordem nos quase-empates.

**Resultado:** o número dito ao usuário ("similaridade 0.9981") é informativamente
**pobre** — para as sub-populações duplicadas, a diferença entre o certo e o
quase-certo é ~0.01, invisível para julgamento humano.

---

## 7. Três causas-raiz (identificadas)

### CAUSA 1 — Duplicatas de conteúdo no corpus (dedup de ingestão INATIVO)
- **Evidência:** 20.4% linhas duplicadas por conteúdo; 23% BLOBs duplicados;
  **61 cópias** de um boilerplate ("Leia o AGENT_PRIMER.md…") em **61 documentos**;
  16.4% do índice em grupos >10; 239 grupos cross-documento.
- **Mecanismo:** dedup preventivo desligado (colunas `is_trivial`/`dedup_of`
  ausentes → fail-safe do indexer ativa comportamento histórico sem dedup).
- **Impacto:** top-k inundado de hits redundantes; **diversidade top-10 = 7.33
  conteúdos / 10 ids**; loteria de desempate nos ~1.0; `distintos@50` cai a 0.86.

### CAUSA 2 — Vetores de providers/normalização heterogêneos (sub-espaço impuro)
- **Evidência:** real-text self-match: **16%** dos chunks têm score próprio
  <0.9 (vetor não-nomic); top-1 é sempre 1.0 (os demais são nomic-alinhados).
- **Mecanismo:** uma fração do índice foi embebedada por outro provider (ou
  normalização) numa ingestão anterior — esses vetores vivem em coordenadas
  **diferentes** do espaço nomic.
- **Impacto:** queries nomic **rebaixam** esses vetores; a distância no espaço
  deixa de ser comparável; recall para o sub-corpus contaminado é fraco.

### CAUSA 3 — Score final é cosseno cru (não-calibrado, não-rankado)
- **Evidência:** `layered.go` não invoca o ranker multi-fator (sub-queries de fonte
  única → `sourceCount>1` nunca verdadeiro); `mergeRanked` ordena `Score` cru;
  score multi-fator é descartado; sinais de grafo/frescor/popularidade são 0.
- **Mecanismo:** **nenhum** re-ranking de múltiplos sinais; score cru em banda alta;
  escalas misturadas sem normalização; desempate por `id asc`.
- **Impacto:** a informação mais relevante para o usuário (a *ordem* entre o que
  importa) é decidida por `id asc` nos quase-empates; o número exibido não separa
  relevância.

> **Síntese:** o índice (int16) é **fiél** ao oracle — recall/NDCG ≈ 1.0. O que
> degrada a qualidade percebida é **dado** (duplicatas + providers misturados) e
> **ausência de ranking/calibração**, não a quantização.

---

## 8. Confirmações

```
go build ./...                    → OK (exit 0)
go test ./internal/vectorbaseline/ → PASS (instrumento read-only, 5.5s)
go vet  ./internal/vectorbaseline/ → OK
nenhuma escrita no knowledge.db   → o .db foi aberto em mode=ro (0 writes)
```

> ⚠️ O `go test` do instrumento **não** modifica dados: só `SELECT` em `mode=ro` e
> replicação de score em memória. Não rodei `cosca index rebuild`, `re-embed` ou
> `verify --fix`.

## 9. Cruzamento com o diagnóstico paralelo

`docs/reports/vector-ranking-diagnosis-2026-08-24.md` (mesmos dados) classifica,
em P1/P2/P3, exatamente a CAUSA 3 acima: o ranker multi-fator é **fiação morta**, a
mistura de escalas sem normalização distorce a ordem, e grafo/frescor/popularidade
são sinais mortos (grafo vazio: 0 entidades, 0 relações). Endereçamento coordenado:
**CAUSA 1** (dedup) e **CAUSA 2** (provider) são de **qualidade de dados**;
**CAUSA 3** é de **pipeline**.

---

## Anexo — arquivos de evidência (read-only)

- `internal/vectorbaseline/baseline_test.go` — instrumento (replica float32/int16/int8).
- Legado reutilizado: `internal/knowledge/campaign_recall_test.go` (FASE 8),
  `internal/vector/int8_recall_test.go` (recall quantização).
- Relatório paralelo: `docs/reports/vector-ranking-diagnosis-2026-08-24.md`.
