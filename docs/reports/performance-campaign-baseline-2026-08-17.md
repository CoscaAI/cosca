# Campanha de Performance — Baseline Imutável

> Registro canônico da campanha · criado 2026-08-17 · **NÃO EDITAR** — se um
> número mudar, crie um novo registro com referência a este (epistemologia:
> nunca reescrever a evidência, apenas acrescentar).

## Hardware (fixo — o perfil da máquina do chef)

```
CPU  = AMD Ryzen 7 5700X3D (Zen 3)
cores = 8
threads = 16
L3   = 96 MB (V-Cache)
RAM  = DDR4-3200 dual-channel (teórico 51.2 GB/s)
```

## Dataset (benchmark canônico)

```
N    = 1,000,000 vetores
dim  = 768
representation = int8 (slab vecs8, pad 16)  [+ float32 exato no mesmo snapshot]
bytes/vetor = 768 B (int8) / 3072 B (float32)
```

## Query

```
K = 10
workers = 1..16 (default 16 = NumCPU)
páginas SEMPRE aquecidas (2+ iterações antes de medir — lição L340)
```

## Métricas

```
latency p50 / p95
Mvec/s
GB/s
recall@1 / recall@5 / recall@10   (float32 = ORACLE, nunca a aproximação)
```

## Oráculos de performance (NÃO alterar — novos benchmarks por hipótese)

| Cenário | Mvec/s | GB/s | Regime |
|---|---|---|---|
| int8 / 1M / 768 (RAM) | **48.63** | ~40.4 | RAM-bound (slab 768MB > L3 96MB) |
| int8 / 100k / 768 (L3) | **80.82** | — | cache (slab 76.8MB < L3 96MB) |

Fonte: BenchmarkIndexScoreOnly8_N1000000_Dim768 / _N100000_Dim768 (commit b963a4d, L341).

## Estado da evidência (FACT / MEASURED / UNKNOWN — atualizado por fase)

```
FACT    hardware L3 = 96 MB
FACT    DDR4 prática saturada ≈ 40.44 GB/s (L340)
MEASURED 100k int8 = 80.82 Mvec/s (cache)
MEASURED 1M int8   = 48.63 Mvec/s (RAM)
MEASURED int8 recall@10 = 0.884 (sintético gaussiano)  ← validar com dados reais (FASE 8)
MEASURED caminho real (FASE 1, 400 queries reais): 100% full-scan do índice int8,
        13.801 vetores/query (== TotalVectors), latency média 1.02ms / p95 615µs,
        fast path int8 atingido SEMPRE; caminho híbrido (CandidateIDs) NUNCA usado
        pelo knowledge.Search — é uma capacidade latente (LayeredSearch), não o caminho
        de produção do `cosca knowledge search`
UNKNOWN joelho cache↔RAM (FASE 3)
UNKNOWN int16 velocidade/recall/memória (FASE 7)
```

## Consequência da FASE 1 (mudança de alvo)

O problema da produção HOJE **não é** o scan de 1M — é 13.8k vetores (10.6MB int8,
cabe no L3). A otimização relevante é o que está ANTES/DEPOIS do kernel (FTS +
embedding + materialização + ranking), não o kernel. O índice int8 já opera em
regime cache na produção atual. O scan de 1M é o cenário de CRESCIMENTO (62k após
re-embed ainda cabe no L3: 48MB).

## FASE 3 — Curva do V-Cache (2026-08-17, L345)

Generation sintético mínimo (só slabs do score8), dim 768, 16 workers,
benchtime 1s/run, páginas quentes (3 iterações pré-timer). Slab = N×768 bytes.

| N | MB slab | ns/op | Mvec/s | GB/s |
|---|---|---|---|---|
| 10k | 7.3 | 150µs | 66.7 | 51.2 |
| 20k | 14.7 | 241µs | 82.9 | 63.7 |
| 40k | 29.3 | 399µs | 100.4 | 77.1 |
| 60k | 44.0 | 568µs | 105.6 | 81.1 |
| 80k | 58.6 | 720µs | 111.1 | 85.3 |
| 100k | 73.2 | 903µs | 110.7 | 85.0 |
| 120k | 87.9 | 1.18ms | 102.1 | 78.4 |
| 150k | 109.9 | 1.64ms | 91.6 | 70.3 |
| 200k | 146.5 | 2.46ms | 81.2 | 62.4 |
| 250k | 183.1 | 3.51ms | 71.3 | 54.8 |
| 500k | 366.2 | 8.62ms | 58.0 | 44.6 |
| 1M | 732.4 | 19.06ms | 52.5 | 40.3 |

```
PROFILE (5700X3D, dim 768, int8):
  regime L3 pleno:   ≤ ~100k (73MB)  → 85 GB/s, ~110 Mvec/s
  transição (LRU):   ~120k–200k      → 78 → 62 GB/s (cache parcial)
  regime RAM:        ≥ ~250k         → 40–45 GB/s (teto DDR4)
DECISION cache_budget (conservador, 15% de margem): 80MB de slab
  → RegimeL3 abaixo de 80MB (produção hoje: 13.8k = 10.6MB ✓)
  → RegimeRAM acima (escala: 62k re-embed = 48MB ainda L3 ✓)
Oráculos preservados (benchmarks antigos intocados): 1M int8 = 48.63 Mvec/s
(índice real) vs 52.5 (sintético) — consistente; 100k = 80.82 (índice real)
vs 110.7 (sintético) — a diferença é o overhead do índice real (slab float32
presente + materialização).
```

## FASE 8 — Recall real no corpus (2026-08-17, L346)

200 queries = vetores reais de chunks (13.914 vetores, dim 768), float32 como
ORACLE, K=50. Condição: query = vetor de um chunk do corpus.

```
conteúdo@50 = 0.8666   jaccard50 = 0.8205 (conjunto de vizinhos — PASS ≥ 0.80)
NDCG@10 = 0.9010       max err = 0.0147   mean err = 0.0028
top-1 mudou 48% — TODOS em empates (gap < 0.02), nunca inversão com gap real
DESCOBERTA do corpus: 34% de embeddings DUPLICADOS (13.914 vetores, 9.184
distintos) + 44.7% de quase-empates (score ≥ 0.99) no top-50 → o ranking é
estruturalmente instável ATÉ no oracle; recall de ranking mede a loteria.
Ação recomendada: deduplicação do corpus (independente do int8).
```

## FASE 7 — Matriz de representações (2026-08-17, L347)

Mesmo harness (15M×768 = 23GB RAM, 16 workers, bandwidth-saturado):

| Representação | Mvec/s | GB/s | recall@10* | NDCG@10 | maxerr | bytes/elem |
|---|---|---|---|---|---|---|
| float32 (oracle) | 12.07 | 37.0 | 1.0000 | 1.0000 | 0 | 4 |
| **int16** | **26.28** | 40.37 | **0.9840** | **0.9898** | 0.0012 | 2 |
| int8 | 52.66 | 40.44 | 0.8840 | 0.9215 | 0.0117 | 1 |

\* recall@10 sintético (gaussiano L2, 50 queries row+ruído).

```
RESPOSTAS às 3 perguntas do professor:
1. mais rápido?  int8 (52.66) > int16 (26.28) > float32 (12.07)
2. maior recall? float32 (oracle) > int16 (0.984) > int8 (0.884)
3. menos memória? int8 (1B) < int16 (2B) < float32 (4B)

DECISION (proposta): int16 = melhor DEFAULT de produção (2.18x float32 com
recall ~0.98 e NDCG 0.99 — ordem quase exata); int8 = velocidade máxima
quando o contrato tolera ~0.88; float32 = oracle/fallback (flag OFF).
Slab vecs16 no índice = próximo passo após decisão do Don.
```

## Regras da campanha

1. **Nunca alterar o benchmark antigo** — cada hipótese ganha um benchmark novo.
2. **FLOAT32 = ORACLE** — a aproximação nunca é o padrão de verdade.
3. **Regimes decididos por medição** — orçamento de cache vem do joelho da curva,
   não do datasheet (FACT L3=96MB ≠ cache_budget).
4. **Bloqueio/qualquer técnica = hipótese testada, não dogma.**
5. **Toda decisão nasce de FACT/MEASURED → DECISION → REQUIRED EVIDENCE.**