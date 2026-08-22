# Campanha de Performance — Dry-run dos filtros T1-T4 (L352)

> Relatório técnico · 2026-08-17 · READ-ONLY (nenhuma escrita, nenhuma
> alteração de corpus/índice/produção) · Filtros NÃO implementados — apenas
> a simulação de classificação.

## 1. Resultado do dry-run (MEASURED)

```
corpus: 62.740 chunks (com vetor 14.119, órfãos 48.621)   ← db mutante: 14.006→14.119
                                                            entre medições (serve ativo)

por regra:        T1 vazio = 3.465 · T2 separador = 4.092 · T3 zero-alnum = 8.730 · T4 estrutura = 387
interseções:      T2 ⊆ T3 (4.092) · T4 ⊆ T3 (387) · T2 sem T3 = 0 · T4 sem T3 = 0
                  → regra efetiva = T1 ∪ T3

CANDIDATOS ÚNICOS = 8.730 (13.91% do corpus)
  com vetor = 1.491 (10.56% dos 14.119)   ← removíveis do índice
  órfãos   = 7.239 (14.89% dos 48.621)    ← evitáveis no re-embed

KEEP pequenos preservados (≤12 chars com alfanuméricos): 4.556
distribuição por documento (top): 279 · 218 · 87 · 75 · 74 · 68 · 58 · 54 candidatos
  (documentos grandes — JSON/config — chunkados em fragmentos)
```

## 2. Auditoria (100 candidatos ESPALHADOS — a amostra inicial era enviesada)

```
SAFE_TO_FILTER  = 92/100  (vazio, separadores puros, estruturas vazias, símbolos sem estrutura)
REQUIRES_REVIEW = 8/100   (fragmentos de símbolos de código/JSON: "{", "}", "\": {",
                           "|-------|" separador de tabela — sem alfanuméricos, sem valor de busca)
DO_NOT_FILTER   = 0/100   ← NENHUM falso positivo estrutural

Exemplos críticos PRESERVADOS (proibido filtrar por tamanho):
  "Go 1.26" ✓  "v1.2.3" ✓  "RAG" ✓  "API" ✓  "SQL" ✓  "404" ✓  "C++17" ✓
Exemplos KEEP na fronteira (preservados — têm alfanuméricos):
  "\"scripts\": {"  "\"engines\": {"  "\"cpu\": ["  "\"arm64\""  "\"os\": ["  "\"darwin\""
```

## 3. Falsos positivos encontrados

**NENHUM estrutural.** Os 8 REQUIRES_REVIEW são fragmentos de símbolos sem
alfanuméricos (`{`, `}`, `": {`, separador de tabela) — o embedding deles é
ruído (mesmo vetor para qualquer fragmento), e o conteúdo real do documento
está nos chunks vizinhos KEEP. O padrão revela um problema do CHUNKER (quebra
de JSON em fragmentos de 1 char), não do filtro.

## 4. Regras ajustadas

Nenhuma regra precisa mudar. Observação registrada:
- T2 e T4 são subconjuntos de T3 por construção → a regra efetiva é **T1 ∪ T3**;
  as regras separadas ficam para o relatório analítico e os unit tests.
- O T3 é a regra dominante (8.730) — cobre separadores, estruturas e símbolos.
- NENHUMA regra usa comprimento mínimo (verificado: 4.556 KEEP pequenos + os
  7 exemplos críticos preservados).

## 5. Estimativa de impacto (INFERRED — validar na implementação)

```
ÍNDICE:    14.119 → ~12.628 vetores pós-filtro (-10.6%) → slab int8 ~9.7MB
           (a dedup futura leva a ~9.273 / 7.1MB — L349)
ÓRFÃOS:    48.621 → ~41.382 candidatos a re-embed (-14.9% de lixo evitado)
LATÊNCIA:  ∝ N (bandwidth-bound) → ~89% do scan atual pós-filtro (ganho
           marginal sozinho; o ganho grande vem da dedup)
RAG:       remoção de ruído do índice (1.491 vetores de lixo) — diversidade
           e recall pós-filtro a medir na fase de validação (HYPOTHESIS)
```

## 6. Classificação

```
MEASURED    contagens do dry-run (escopo: db em 2026-08-17 — db MUTANTE entre
            medições: 14.006→14.119 vetores; EVIDENCE de instabilidade do instrumento)
EVIDENCE    0/100 DO_NOT_FILTER na auditoria espalhada; 7 exemplos críticos preservados
INFERRED    T2/T4 ⊆ T3; fragmentos JSON = problema do chunker, não do filtro;
            impacto no índice/latência (∝ N)
HYPOTHESIS  ganho de diversidade/recall pós-filtro (a medir); o re-embed dos
            órfãos restantes rende mais após filtro+dedup
UNKNOWN     causa da leva de 08-16; se o chunker de JSON deveria não produzir
            fragmentos; NDCG pós-filtro em produção
DECISION    filtros T1-T4 SEGUROS (critério de parada: sem FP estrutural →
            pode avançar) — mas a IMPLEMENTAÇÃO ainda requer aprovação do Don
```

## 7. Próxima fase (proposta)

**L353 — Implementação dos filtros T1-T4 no pipeline de chunkização/ingestão**
(com flag de ativação, nada deletado, testes unit por regra + re-auditoria).
Requisitos: aprovação do Don · backup · código novo (não altera o existente) ·
dry-run re-executável como teste de regressão.

## Anexo — evidência

- `internal/knowledge/campaign_dryrun_test.go` — harness read-only (L352)