# L347 — Campanha FASE 7: int16 (kernel + matriz de representacoes)

## Kernel dot16AVX2 (dot16_amd64.s)
- 32 elems/iter, YMM: VMOVDQU + VPMADDWD (16 pares -> 8 i32) + VPADDD.
- SEM bias (diferente do int8): produto i16xi16 cabe em i32 com escala 2^10.
- Escala 2^10 (1024): SOMA TOTAL dim x 1023^2 <= 2^31 exige dim <= 2048
  (embeddings reais 768/1536 ✓). Resolucao 10 bits = 8x melhor que int8.
- Licao: estouro nao e por-lane, e na SOMA FINAL (reducao das 8 lanes) —
  teto = dim x escala^2 <= 2^31. Escala 2^11 estourava em dim 768.

## Matriz (mesmo harness 15M x 768 = 23GB RAM, 16 workers)
Representacao  Mvec/s    GB/s     recall@10  NDCG@10  maxerr   bytes/elem
float32       12.07     37.0     1.0000     1.0000   0        4
int16         26.28     40.37    0.9840     0.9898   0.001183 2
int8          52.66     40.44    0.8840     0.9215   0.011690 1

Respostas as 3 perguntas do professor:
1. Mais rapido? int8 (52.66) > int16 (26.28) > float32 (12.07)
2. Maior recall? float32=oracle > int16 (0.984 @10) > int8 (0.884)
3. Menos memoria? int8 (1B) < int16 (2B) < float32 (4B)

## Observacoes
- int16 satura a bandwidth (40.37 GB/s) — 2.18x float32 com recall ~0.98.
- int16 NDCG 0.99 vs int8 0.92: a ORDEM fica quase exata com 8x resolucao.
- Benchmarks antigos intocados (oraculos 48.63/80.82 preservados).

## DECISION (para o Don)
int16 = candidato a melhor DEFAULT de producao: 2.18x float32 + recall
~0.98 (vs 0.88 do int8) — o meio-termo entre velocidade e contrato.
int8 = quando velocidade maxima e o contrato tolera ~0.88 (jaccard 0.82 real).
Implementacao no indice (slab vecs16) fica como proximo passo apos decisao.
