# Índice Vetorial In-Memory (Full-Scan Otimizado)

> Otimização #1 da rodada de performance do vector search (autópsia L311).
> Endereça o gargalo ARQUITETURAL provado: não existe índice vetorial; cada
> busca sem candidatos lexicais faz full-scan O(N) re-decodificando 100% do
> store via SQLite (modernc) a cada query (~94% da latência).

## O problema

O caminho quente antigo (`Search` → `SearchWithFilter` → `scanAll` →
`scoreParallel`) fazia, por query:

1. `SELECT id, vector FROM vectors` — full-scan do SQLite.
2. `database/sql` clonava cada BLOB (≈ 6.5 KB por vetor em dim 768) —
   ~600 MB de alocação por varredura de 100K vetores.
3. Decodificação float32 + dot — só ~5–10% do tempo total.

Resultado medido (BEFORE): 100K dim768 ≈ 402 ms; 1M ≈ 4.1 s.

## O que é o índice

Um snapshot **derivado** do store (nunca fonte de verdade), com layout
**Structure-of-Arrays (SoA)**:

```
indexGeneration:
  ids  []string            // id[i] == id da row i
  vecs []float32           // slab CONTÍGUO: row i vive em vecs[i*dim:(i+1)*dim]
  dim  int
  n    int
```

- Nenhum per-row BLOB, nenhuma alocação na busca quente (o score só aloca o
  resultado top-K).
- O score roda dot puro sobre `float64(vecs[k])`, com a **mesma aritmética** do
  `dotFromBytes` original (acumulação float64 na mesma ordem) → scores
  **bit-idênticos** ao caminho antigo (paridade funcional garantida por teste).

## Onde vive / como sincroniza

- `internal/vector/index.go` — tipo `InMemoryIndex` + snapshot imutável.
- `internal/vector/sqlite_vec.go` — `SQLiteVec` ganhou um contador de versão
  (`version`, bumpado em toda escrita **committed**) e um ponteiro atômico para
  a geração atual.

### Invalidação

- `Store` / `Delete*` / `Rebuild` — bumpam a versão após o commit.
- `StoreTx` — bumpa na invalidação conservadora; o indexer chama
  `StoreTxCommitted()` **após** o `tx.Commit()` (nova interface
  `vector.StoreTxCommitter`), garantindo que o snapshot nunca bake uma escrita
  não-commitada como final (mesmo que uma busca recarregue no meio da janela).
- A busca lê o ponteiro atômico UMA vez e compara `gen.version == store.version`;
  se divergir, recarrega sob um mutex de recarga (um construtor por vez).
- Leituras paralelas: **zero lock na busca quente** (snapshot imutável, swap
  atômico de ponteiro — padrão Go ideal). `reloadMu` só é disputado quando há
  recarga.
- Fail-safe: se o load falhar ou o store mutar durante o load (versão instável,
  tentativas limitadas), a busca cai no caminho SQL original — o índice nunca
  quebra uma busca.

## Quando usar / QUANDO NÃO usar

**Usar** quando: store 100% em memória é aceitável (1M×768 ≈ 3 GB), buscas
full-scan sem filtro, serve 24/7 com queries repetidas.

**NÃO usar** (cai no SQL) quando: há filtro de metadata (`SearchWithFilter` com
`filter` não-vazio — o índice não guarda metadata), busca bounded por candidatos
lexicais (`SearchWithCandidates` — já é rápida e limitada), query com dimensão
diferente da dimensão do store, ou o store está vazio.

## Resultado medido (BEFORE vs AFTER) — dim 768, limit 10, Ryzen 7 5700X3D

| Dataset | BEFORE (SQL) | AFTER (índice) | Speedup |
|--------:|-------------:|---------------:|--------:|
| 10K     | 43.0 ms  / 0.23 Mvec/s / 64.6 MB / 60,375 allocs | 0.92 ms / 10.8 Mvec/s / 0.28 MB / 322 allocs | **~46×** |
| 100K    | 401.7 ms / 0.25 Mvec/s / 649.5 MB / 600,386 allocs | 10.6 ms / 9.4 Mvec/s / 2.5 MB / 326 allocs | **~38×** |
| 1M      | 4,076 ms / 0.25 Mvec/s / 6,488 MB / 6,000,406 allocs | 106 ms / 9.4 Mvec/s / 24.1 MB / 328 allocs | **~38×** |

Custo de carga (uma vez, 100K): ≈ 626 ms / ~958 MB alocados — amortizado na
primeira busca (cold). Cold (load + search) ≈ 624 ms; warm ≈ 10.6 ms.

Paralelismo (score 100K, workers explícitos): W1 77 ms → W4 20 ms → W8 12 ms →
W16 8.4 ms (~9× em 16 núcleos).

## Próximo experimento

ANN (HNSW) é o próximo passo recomendado pela L311 (250 ms → <5 ms): o índice
flat ainda é O(N) por query (106 ms a 1M). HNSW troca exatidão por latência
sub-milissegundo; o índice flat aqui serve de baseline exato e fonte de
validação de recall para o ANN.
