# PERFORMANCE_BASELINE_L387_L390 — baseline congelado da campanha de performance

> Documento de referência da campanha de performance (Fase 6-8 do professor).
> **CONGELADO — não alterar.** Qualquer comparação de otimização futura deve
> usar ESTE baseline. Criado em 2026-08-17.

## 1. Contexto congelado

| Item | Valor |
|---|---|
| Commit da campanha | `edc853f` (L390 — árvore: L387→L390) |
| Corpus | conhecimento do Cosca (knowledge.db) |
| Documentos | 2.476 |
| Chunks | 62.806 |
| Vetores | 50.969 |
| Triviais (sem vetor, por design L360) | 8.587 |
| Duplicados (dedup_of, por design L360) | 3.250 |
| Provider de embeddings | ollama (nomic-embed-text:latest) |
| Digest pinado | `0a109f422b47e3a30ba2b10eca18548e944e8a23073ee3f3e947efcf3c45e59f` |
| Dimensão | 768 |
| Kernel vetorial (produção) | int16 AVX2 (default L362), fallback int8, float32 = oracle exato |
| DB | SQLite 452M (`.cosca/knowledge.db`) |
| Hardware | AMD Ryzen 7 5700X3D 8C/16T (3D V-Cache), 32GB RAM |
| Binário | ~130MB (`~/.cosca/bin/cosca`) |

## 2. Resultados medidos (L387-L390)

### Baseline de performance (L387)
- Busca real (TestRealBenchmark): **cache quente 0.045ms / frio 0.696ms / 11.058 qps**
- Breakdown da busca (L368): FTS 46% · ranking/merge 29% · vector 25% · embed 0.2%
- Kernel: int8 AVX2 52.66 Mvec/s (bandwidth DDR4 saturada) · int16 26.28 Mvec/s
- Startup do serve: ~0.55s + 206MB
- Indexação: 4.4s/doc SEM embed (FTS rebuild + graph por documento); re-indexação de alterado = o mesmo custo
- Embed real: ~133 chunks/s (nomic via ollama, L364)

### 15 achados de trabalho duplicado (L387) — top:
1. dedup DEPOIS do embed + batch sem cache (indexer.go:240, provider.go:383)
2. FTS rebuild COMPLETO por IndexDirectory (indexer.go:415)
3. busca REST/gRPC sem o caminho bounded (search.go:202)
4. dedup scan full sem índice em hash (indexer.go:720)

### Bounded (#3) — L388/L389 (corrigido)
- recall@10 bounded vs full: **81.0%** (8.1/10) — speedup 2.86x (0.754ms → 0.263ms)
- Causa da regressão: pool de recência (250) quando o FTS vazio (recall 0.0 em 2/20)
- Veredito: **REJECT** (regressão funcional por ganho marginal)

### H1 (merge) — L389: REFUTADA
- merge∩FTS 26% · merge∩vetor 42% · src=vec 4.2 vs fts 3.9 — o vetor domina o merge
- FTS isolado 3.9/10 (fonte fraca) · vetor isolado 7.5/10 (fonte rica)
- O Ranker (BM25 0.25 + vec 0.35 + graph 0.20 + fresh 0.10 + pop 0.10) remixa radicalmente

### H2 (triviais) — L390: DESCARTADA
- Triviais no top-10: 0.10/10 — o Ranker os bloqueia (score vetor 0)
- Overlap limpo∩vetor 4.2 = atual∩vetor 4.2 (sem mudança)

## 3. Queries do benchmark (as 20 da campanha)

vector search performance · hardware brain capability · scene graph procedural ·
memory bandwidth benchmark · security sandbox jail · cosca kernel memory ·
epistemology fact measured · storage nvme knowledge · avx2 int8 embedding kernel ·
knowledge base integrity verify · worker pool fabric concurrency ·
recovery protocol last known good · chain of custody memory blocks ·
tie breaker deterministic ranking · nomic embedding provider digest ·
sqlite vector index query plan · bwrap sandbox jail secrets ·
merkle tree epoch files · graph persistence save entities · fts5 rebuild tokenize

## 4. Oracle (definição + método + limitações)

- **Oracle de recuperação:** top-k do vetor **float32** (índice exato, não
  quantizado) — o ground truth MATEMÁTICO do espaço semântico.
- **Método:** o float32 é o oracle por construção (a quantização int16 é uma
  aproximação — o oracle conhecido: jaccard 0.989 / recall@10 0.995 no corpus
  limpo, L360-L364). O oracle int16 vs float32 mede a FIDELIDADE da recuperação.
- **Limitações:** o oracle é MATEMÁTICO, não humano. "O vetor encontrou o
  esperado" ≠ "o esperado era o correto". Qualidade humana exige um oracle de
  relevância separado (não definido nesta campanha).
- **Identidade do provider:** digest pinado + fail-closed (L376); a identidade
  semântica provada (L375/L379 — 138 pares cosine 1.0000).

## 5. Princípios da campanha (do professor — disciplinas obrigatórias)

1. **Baseline congelado** — nenhuma comparação sem referenciar ESTE documento.
2. **P/Q/C separados** — toda alteração reporta PERFORMANCE, QUALIDADE e
   CORREÇÃO separadamente (uma pode subir e a outra descer).
3. **Oracle registrado** — versão, origem, método, limitações (acima).
4. **Guardião de regressão** — toda transformação nasce com teste ANTES/DEPOIS
   (latência + recall); REJECT se qualquer régua falhar.
5. **Benchmark sério** — warm-up, múltiplas iterações, MEDIANA, p95/p99,
   variabilidade (0.75/0.74/0.76 ≠ 0.20/1.40/0.31).
6. **A régua das 4 réguas:** uma otimização só é GANHO se sobreviver
   simultaneamente a performance, qualidade, correção e invariantes.
7. **Árvore de hipóteses** — conhecimento negativo e positivo rastreável
   (por que NÃO bounded: L388/L389 — speedup 2.86x, recall 81%, causa da
   regressão identificada, rejeitado pelo custo).

## 6. Árvore de hipóteses (estado em L390)

```
PERFORMANCE (L387, baseline + 15 achados)
 ├── BOUNDED (L388/L389) → REJECT (recall 81%, pool de recência, ganho 0.5ms)
 ├── RANKING
 │   ├── H1 FTS domina (L389) → ❌ REFUTADA (vetor domina 4.2 vs 3.9)
 │   └── H2 triviais contaminam (L390) → ❌ DESCARTADA (0.1/10; Ranker bloqueia)
 │   └── H3 recall semântico isolado → ⏳ (próximo)
 └── INGESTÃO
     ├── dedup pré-embed → ⏳ (evidência forte L387#1)
     └── FTS incremental → ⏳ (evidência forte L387#2)
```
