# DIAGNÓSTICO DE QUALIDADE — RANKING DA BUSCA SEMÂNTICA (vetor → multi-fator)

> Diagnóstico técnico · 2026-08-24 · **READ-ONLY** — nenhum dado alterado,
> nenhum DELETE, nenhum re-embed, nenhuma reindexação. Fronteira de análise
> estrita: só li código e fiz consultas de leitura (SELECT / PRAGMA) sobre o
> `knowledge.db` atual.
>
> Objetivo (Don): **"deixar perfeito"** o ranking antes de transformar.
> Escopo: qualidade da ordenação da busca semântica — pipeline L1-FTS5 →
> L2-BM25 → L3-Vector em `internal/search/layered.go`, e o re-ranking
> multi-fator em `internal/ranking/ranking.go` + `explain.go`.
>
> Roteiro: (1) resposta direta — o ranking multi-fator está aplicado na saída?
> (2) estado do dedup · (3) estado do grafo node/edge · (4) top-3 problemas.

---

## 1. RESPOSTA DIRETA — O ranking multi-fator está aplicado na saída?

**NÃO.** No caminho principal da busca (`cosca knowledge search` /
`cosca search layered`), o re-ranking multi-fator **nunca é invocado** sobre a
saída, e o `Score` exposto **nunca** é o score multi-fator — é o score cru
(cosseno ou BM25) vindo da camada.

### 1.1 O único call-site do ranker é condicional e o layered nunca o dispara

O `e.ranker.Rank(...)` aparece em **um único** ponto do código:

- `internal/search/search.go:264-267`
  ```go
  if sourceCount > 1 && params.Query != "" && e.ranker != nil {
      rankables := e.toRankables(allResults)
      ranked := e.ranker.Rank(rankables, params.Query)
      allResults = e.fromRankables(ranked)
  } else {
      sort.Slice(allResults, func(i, j int) bool {   // <-- ordena por Score cru
          return allResults[i].Score > allResults[j].Score
      })
  }
  ```

O gate `sourceCount > 1` (contado em `search.go:251-263`) exige que **uma única
chamada** ao `engine.Search` tenha **≥ 2 fontes habilitadas** (FTS + vetor, etc.).
O algoritmo em camadas jamais faz isso — ele sempre usa sub-queries de **fonte
única** e depois funde manualmente:

- **L1 — FTS5** (`internal/search/layered.go:190-197`):
  `EnableFTS: true, EnableVector: false, EnableGraph: false` → `sourceCount = 1`
  → ranker **pulado**.
- **L3 — Vector** (`internal/search/layered.go:368-396`, params em `372-381`):
  `EnableFTS: false, EnableVector: true, EnableGraph: false` → `sourceCount = 1`
  → ranker **pulado**.
- Fusão manual: `internal/search/layered.go:250` `mergeRanked(bm25Hits, vecHits)`
  → `internal/search/layered.go:523-547` ordena por `Score` **cru**, SEM
  normalização.

### 1.2 O `Score` exibido é o cru, não o multi-fator

- O CLI `cosca knowledge search` lê diretamente `r.Score` do resultado em camadas:
  `internal/cli/knowledge.go:182-196` (`res, _ := ls.Search(...)`, e
  `Score: r.Score` em `194`).
- Mesmo **quando** o ranker roda (caso não-layered, ex.: `runGlobalSearch` →
  `ke.Search` com FTS+vetor → `sourceCount=2` em `cli/knowledge.go:256-260`),
  ele reordena mas **não grava** o score final de volta:
  `search.go:641-648` `fromRankables` mantém `r.(*searchResultRankable).result`
  inalterado e só seta `Rank`. Ou seja, o score final multi-fator do `Ranker`
  é **descartado** no campo exposto.

### 1.3 Sinais de grafo/frescor/popularidade são constantes (mortos)

O adaptador que liga `SearchResult` ao `Rankable` **hardcoda zeros**:

- `internal/search/search.go:626-631`
  ```go
  func (r *searchResultRankable) Timestamp() int64    { return 0 }
  func (r *searchResultRankable) ReferenceCount() int { return 0 }
  func (r *searchResultRankable) GraphDistance() int  { return 0 }
  ```

Assim, mesmo no (raro) caminho em que o ranker roda:
- `GraphDistance()=0` → `computeGraphScore(0)=1.0` (ranking.go:240-249) → todos
  recebem `0.20*1.0` idêntico → **não diferencia**;
- `Timestamp()=0` → `computeFreshness=0.5` (ranking.go:252-255) → constante;
- `ReferenceCount()=0` → `computePopularity=0` (ranking.go:267-273) → nulo.

O grafo, o frescor e a popularidade **não participam** da ordenação em nenhum
caminho real.

---

## 2. ESTADO DO DEDUPLICAÇÃO

### 2.1 Não há dedup de conteúdo aplicado à saída da busca

- `mergeRanked` (`layered.go:523-547`) deduplica **por ID** apenas
  (`seen[r.ID]`), nunca por conteúdo.
- `mergeSearchResults` (`cli/knowledge.go:2339-2363`) deduplica **por Título**
  apenas, preservando maior score — também não agrupa duplicatas por conteúdo.
- Não existe campo `DuplicateOf` / grupo de duplicatas no `SearchResult`
  (`search.go:36-53`), e nenhuma lógica marca/agrupa quase-duplicatas no topo.

### 2.2 Colunas `is_trivial` / `dedup_of` NÃO existem no schema base nem no DB atual

- O schema base de `chunks` **não tem** essas colunas:
  `internal/sqlite/schema.go:62-73`.
- Elas só são criadas por uma "migração leve" de campanha (ex.:
  `internal/knowledge/campaign_cleanup_exec_test.go:75-81` —
  `ALTER TABLE chunks ADD COLUMN is_trivial ...`, `ADD COLUMN dedup_of ...`).
- **Verificado no DB atual** (consulta de leitura, sem escrita):
  ```
  chunks.is_trivial exists: false
  chunks.dedup_of   exists: false
  ```

### 2.3 O dedup preventivo na ingestão está DESLIGADO neste DB

- O dedup preventivo (`internal/indexer/indexer.go:729+`, grava `dedup_of` em
  `indexer.go:865`) tem fail-safe via `chunksHaveDedupCols`
  (`indexer.go:735-736`): sem a coluna, usa o comportamento histórico (sem dedup).
- Como a coluna não existe no DB fresco, o "ciclo fechado" do dedup (memória
  L372) está **inativo** aqui.

**Conclusão (dedup):** o corpus atual está **sem dedup ativo** — nem dedup na
saída da busca, nem colunas de marcação, nem dedup preventivo na ingestão. Isso
reexpõe o problema histórico (≈44.7% de quase-empates no top-50, 34% duplicatas).

---

## 3. ESTADO DO GRAFO NODE/EDGE

- `e.graph = graph.New()` em `internal/knowledge/knowledge.go:286`.
- `loadGraph()` (`knowledge.go:1879-1908`) restaura o grafo de
  `cache WHERE key='graph_state'` — se não houver linha, o grafo **permanece
  vazio** (0 nós / 0 arestas).
- **Verificado no DB atual** (leitura):
  ```
  entities         = 0
  relationships    = 0
  graph_state_cache = 0
  ```
  ⇒ O grafo **não está populado** — nenhum nó, nenhuma aresta,
  sem estado serializado.
- **`GraphDistance` não é populado na prática**: mesmo com um grafo cheio, o
  adaptador `searchResultRankable.GraphDistance()` → `0` (`search.go:631`),
  então o sinal de proximidade é um **vale constante** `1.0` para todos.

**Conclusão (grafo):** o sinal de grafo não pode ter efeito algum — o grafo está
vazio e, ainda que não estivesse, a distância não é propagada até o ranker.

---

## 4. TOP-3 PROBLEMAS DE QUALIDADE (com prioridade)

### P1 (CRÍTICA) — Re-ranking multi-fator é "fiação morta": não roda na saída e o score final nunca é exposto

- **Evidência:** `search.go:264-267` (único call-site, gate `sourceCount>1`);
  `layered.go:190-197` e `layered.go:368-396` (sub-queries de fonte única →
  gate nunca verdadeiro); `layered.go:250` + `523-547` (fusão manual por `Score`
  cru); `search.go:641-648` (rank não grava o `final` no resultado);
  `cli/knowledge.go:182-196` (CLI lê `r.Score` bruto).
- **Impacto:** todo o engenho multi-fator (BM25+vetor+grafo+recência+popularidade
  com pesos 0.25/0.35/0.20/0.10/0.10, `ranking.go:67-79`) é **inativo** no
  caminho principal. O usuário recebe uma ordem baseada em escore cru de uma
  única fonte, não a "melhor" ordenação por todos os sinais.

### P2 (ALTA) — Mistura de escalas sem normalização + desempate não-determinístico

- **Evidência:** `layered.go:523-547` ordena por `Score`, fundindo BM25
  (autocontido, `layered.go:463-495`, **pode exceder 1.0** para consultas
  multi-termo) com cosseno (0..1), **sem** min-max normalizar — exatamente o
  tipo de mistura que o `ranking.Config.NormalizeScores` (default `true`,
  `ranking.go:77`) corrigiria. O default atual desativa BM25 (`MinBM25Score=999`
  e `MinFTSCount=0`, `layered.go:115-123`), então a camada quase sempre cai no
  `mergeRanked`.
- **Impacto:** escalas incomparáveis distorcem a ordem; junto com os quase-empates
  (duplicatas) do corpus sem dedup, gera ordenação instável/arbitrária (o relatório
  de dedup 2026-08-17 já documentou instabilidade do desempate entre scores iguais).

### P3 (ALTA) — Sinais mortos + dados ausentes: grafo vazio, recência/popularidade zeradas, dedup inativo

- **Evidência:** `search.go:626-631` (Timestamp/ReferenceCount/GraphDistance = 0);
  DB real: `entities=0`, `relationships=0` (ver §3); `schema.go:62-73` sem as
  colunas de dedup; `knowledge.go:398-433` e `indexer.go:753-736` mostram o
  fail-safe que desliga o dedup sem as colunas.
- **Impacto:** dos 5 sinais, só **BM25 + vetor** podem diferenciar; grafo,
  frescor e popularidade **não contribuem**; e a ausência de dedup mantém o
  corpus cheio de cópias que "roubam" posições e poluem o top-k.

---

## 5. RESUMO EXECUTIVO (para decisão do Don)

| Pergunta | Resposta | Evidência-chave |
|---|---|---|
| Ranking multi-fator aplicado na saída? | **NÃO** (layered nunca chama; e score final não é propagado) | `search.go:264-267`, `layered.go:250/523-547`, `search.go:641-648`, `cli/knowledge.go:182-196` |
| Dedup na saída / colunas? | **Não há** dedup no output; colunas `is_trivial`/`dedup_of` **ausentes** no DB | `schema.go:62-73`, PRAGMA → `false/false` |
| Dedup preventivo de ingestão? | **Inativo** (fail-safe por coluna ausente) | `indexer.go:735-736` |
| Grafo populado? | **Não** (0 entidades, 0 relações) | leitura DB → `0/0`; `knowledge.go:1879-1908` |
| `GraphDistance` usado? | **Não** (hardcoded 0 no adaptador) | `search.go:631` |
| Build | ✅ `go build ./...` OK | — |
| Testes ranking/grafo | ✅ `ok` (ranking 0.946s, graph 0.990s) | `go test ./internal/ranking/... ./internal/graph/...` |
| Colunas dedup (verificadas) | `is_trivial=false`, `dedup_of=false` | PRAGMA read-only |

---

## Anexo — números medidos do `knowledge.db` atual (somente leitura)

```
vectors                       13217
chunks                        13217
distinct_chunks_with_vec      13217   (1:1 — todo chunk tem vetor)
entities                          0
relationships                     0
graph_state_cache                0
chunks.is_trivial exists    : false
chunks.dedup_of   exists    : false
```

> Método: driver `modernc.org/sqlite`, aberto com `?mode=ro` (nenhuma escrita),
> queries `SELECT COUNT(*)` e `PRAGMA table_info(chunks)`.
