# Campanha de Performance — Caracterização da Deduplicação (L349)

> Relatório técnico · 2026-08-17 · READ-ONLY (nenhuma alteração de produção,
> nenhuma remoção, nenhuma escrita no corpus) · Fronteira EXPERIMENTAL rígida.

## 1. Dados brutos (MEASURED, knowledge.db em 2026-08-17)

```
corpus: 62.627 chunks (com vetor 14.006, órfãos 48.621) · 14.006 vetores · 9.273 distintos

CAT1 conteúdo duplicado (sha256 do text):  33.785 chunks = 53.9% dos chunks
      — desses, 12.850 têm vetor (92% dos vetores pertencem a conteúdo duplicado)
CAT2 vetor duplicado (sha256 do blob):     4.733 vetores = 33.8%
CAT2b distribuição de cópias:              MÁX 752 cópias de UM vetor
      83 grupos com >10 cópias = 3.825 vetores = 27.3% do índice
CAT3 intra-documento: 210 (4.4%)  ·  cross-documento: 4.523 (95.6%)
CAT4 mesmo chunk: 0
CAT5 vetor-igual-conteúdo-diferente: 155
CAT6 quase-duplicatas (cosseno ≥ 0.99): 44.7% do oracle top-50 (FASE 8)

órfãos (chunks sem vetor): 48.621 — 23.786 (48.9%) têm conteúdo idêntico a
      chunk JÁ indexado → re-embed parcialmente redundante
```

## 2. Impacto medido (200 queries reais, método determinístico)

```
diversidade do top-10 (oracle float32): 10.00 ids ÚNICOS mas apenas 4.91
      CONTEÚDOS únicos → ~metade das respostas do RAG são cópias do mesmo texto

jaccard@50 int8 (determinístico): 0.9646  ·  pós-dedup (simulação): 0.9270
índice pós-dedup: 9.273 vetores (33.8% menor) → slab int8 7.1MB (vs 10.8MB)
      — folga grande dentro do L3 budget 80MB
latência estimada pós-dedup: 66.2% do atual (bandwidth-bound ⇒ ∝ N)
```

## 3. Descoberta metodológica (SEGUNDA ORDEM — a mais importante)

O índice em produção usa `scoreParallel` (16 workers) com top-K window por
worker. **Entre scores iguais (duplicatas de 1.0), o desempate é a ordem de
chegada dos workers — NÃO-determinístico.** Medições via `Search` variam entre
execuções:

```
jaccard@50 int8 (índice, produção):  0.82–0.96  (varia — loteria das duplicatas)
jaccard@50 int8 (determinístico):    0.9646     (desempate por rowid, estável)
```

**A FASE 8 (L346) reportou 0.82 — CONTAMINADO pelo desempate não-determinístico.
Errata: o jaccard real do int8 é 0.965 (determinístico), e o ranking de produção
para queries com duplicatas massivas é INSTÁVEL entre execuções.** Isto é um
problema de PRODUTO: o Cosca pode devolver resultados diferentes para a mesma
query em momentos diferentes.

## 4. A-sub — recall real do int16 (método determinístico)

```
                recall@1  recall@5  recall@10  jaccard@50  NDCG@10  maxerr
int8  (real)    0.9950    —        0.9780     0.9646      1.0000   0.0147
int16 (real)    1.0000    0.9980   0.9955     0.9922      1.0000   0.0019
float32         oracle    oracle   oracle     oracle      1.0000   —
int16 top-1 mudou: 0/200 (0.0%) — nenhuma mudança relevante (gap ≥ 0.02: 0)
```

## 5. Análise e classificação

```
MEASURED  contagens acima (escopo: este corpus, esta data — não FACT universal)
MEASURED  int8 real jaccard 0.965 / recall@10 0.978 (determinístico)
MEASURED  int16 real jaccard 0.992 / recall@10 0.996 / top-1 nunca muda
MEASURED  diversidade top-10 = 4.91 conteúdos / 10 ids
EVIDENCE  ranking de produção NÃO-determinístico entre duplicatas (0.82–0.96)
INFERRED  48.9% dos órfãos duplicam conteúdo indexado → re-embed parcialmente
          desperdiçado (a confirmar na próxima fase — depende da causa)
HYPOTHESIS CONFIRMADA  duplicatas são a causa estrutural da instabilidade do
          ranking e da baixa diversidade do top-k (evidência: 752 cópias,
          27.3% do índice em grupos >10, top-10 com 4.91 conteúdos)
HYPOTHESIS AJUSTADA    o jaccard do int8 NÃO é 0.82 — é 0.965; o 0.82 era
          artefato do desempate paralelo (não um defeito do int8)
UNKNOWN    a CAUSA da duplicação massiva (752 cópias!) — origem/processo de
          ingestão; impacto real pós-deduplicação no NDCG de produção
```

## 6. Impacto estimado da deduplicação

> Errata P13 (L350, correção do professor): a linha "Qualidade RAG: top-10
> passa de ~4.9 para ~10 conteúdos" era ESTIMATIVA tratada como resultado.
> Classificação correta: MEASURED diversidade atual = 4.91; MEASURED simulação
> reduz o índice para 9.273; HYPOTHESIS dedup melhora diversidade/qualidade;
> UNKNOWN NDCG pós-dedup em produção (a medir com o protocolo final).

| Dimensão | Classificação | Valor |
|---|---|---|
| Qualidade RAG (diversidade top-10) | **MEASURED** (atual) | 4.91 conteúdos / 10 ids |
| Qualidade RAG pós-dedup | **HYPOTHESIS** | ~10 conteúdos (a medir) |
| Estabilidade | MEASURED | ranking determinístico pós-dedup (sem loteria) |
| Índice | **MEASURED** (simulação) | 33.8% menor (9.273 vs 14.006); slab 7.1MB — L3 folgado |
| Latência | INFERRED | ~66% do scan atual (∝ N, bandwidth-bound) |
| Re-embed | INFERRED | até ~48.9% dos órfãos são duplicatas → evita trabalho redundante |
| Recall int8/int16 pós-dedup | **UNKNOWN** | jaccard 0.965→0.927 na simulação; NDCG real a medir |

## 7. Riscos

- **Dedup por conteúdo exato é seguro** (não remove chunks com conteúdo
  diferente — CAT5 = 155 casos, tratar separadamente).
- Remoção em produção exige backup + dry-run + validação (reversível).
- A causa da duplicação (UNKNOWN) pode ser um processo de ingestão ativo —
  dedup sem corrigir a causa = reaparecimento.

## 8. DECISION recomendada

1. **Próxima experiência (antes de qualquer implementação): investigar a CAUSA**
   das 752 cópias — qual documento/arquivo se repete, qual processo o gerou
   (sync/embed-sync/agregação). Determina se a correção é preventiva (ingestão)
   + limpeza, ou só limpeza.
2. **Representação**: com o método correto, int8 real (jaccard 0.965, recall@10
   0.978) é excelente — o int16 (0.992, top-1 nunca muda) é o default para
   fidelidade máxima. A decisão int8/int16 fica APÓS a dedup (o ranking
   pós-dedup é o que vale).
3. **Nenhuma implementação agora** — a caracterização termina aqui. Dedup e
   slab vecs16 só após a causa ser caracterizada (regra: reduzir incerteza
   primeiro).

## 9. Errata da FASE 8 (L346)

O jaccard@50 0.82 e os recall/NDCG medidos via `Search` (índice paralelo) são
afetados pelo desempate não-determinístico entre duplicatas. Valores corretos
(determinísticos, L349): int8 jaccard@50 = 0.965, recall@10 = 0.978, recall@1 =
0.995, NDCG@10 = 1.0. A correção não invalida a descoberta do L346 (34%
duplicados) — apenas recalibra as métricas de qualidade do int8.

## Anexo — arquivos de evidência (experimental, read-only)

- `internal/knowledge/campaign_dedup_test.go` — caracterização B
- `internal/knowledge/campaign_int16_real_test.go` — A-sub (int16 real)
- Harnesses reutilizáveis: `campaign_path_test.go` (FASE 1), `campaign_recall_test.go` (FASE 8)