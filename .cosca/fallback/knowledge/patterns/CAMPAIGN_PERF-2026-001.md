# CAMPAIGN PERF-2026-001 — "Onde está o maior custo real e quais otimizações são seguras?"

> Registro vivo da campanha de performance (institucionalizada no 29º protocolo).
> A árvore de investigação — preservada para reconstrução futura.

## 1. Contrato da campanha

| Campo | Valor |
|---|---|
| ID | PERF-2026-001 |
| Question | Qual trabalho redundante existe no caminho de indexação e busca, qual impacto mensurável ele possui e quais otimizações preservam todas as invariantes? |
| Scope | Indexação + busca |
| Non-goals | Kernel (bandwidth DDR4 saturada) · GPU · UI · mudanças arquiteturais não relacionadas |
| Baseline | L387 — `PERFORMANCE_BASELINE_L387_L390.md` (CONGELADO — nunca sobrescrever) |
| Invariants | recall (oracle) · ranking (tie-breaker) · provider identity (digest) · fail-closed · provenance (chain/merkle) · determinismo · recovery · memória (ordem sagrada) |
| Mode | READ_ONLY (experimentos na cópia do db) |
| Transformation | FORBIDDEN até autorização explícita do Don |
| Budget | max experiments 20 · max runtime 2h · max corpus mutation 0 (produção) · max external calls limitado (ollama local) |
| STOP IF | evidência contraditória · ambiente mudou · invariante violada · medição inválida · estado desconhecido |

## 1b. Estado formal (CAMPAIGN_PROTOCOL v3 — §4)

**INVESTIGATING → EVIDENCE_REVIEW** (a pergunta original em vias de resposta;
nenhum estado pulado — o histórico abaixo documenta a transição).

| Estado | Evidência |
|---|---|
| PROPOSED | L387 (a pergunta: onde está o trabalho redundante?) |
| SCOPED | L387 (escopo: indexação + busca; non-goals registrados) |
| BASELINED | L387 (PERFORMANCE_BASELINE_L387_L390 — congelado) |
| INVESTIGATING | L387-L393 (7 experimentos/hipóteses executados) |
| EVIDENCE_REVIEW | atual (conclusão parcial: ranking saudável; custo dominante = FTS rebuild + graph) |
| DECIDED | (pendente — as decisões parciais abaixo; a conclusão final após E-006/E-007) |

## 2. A árvore de investigação (estados formais)

```
PERF-2026-001
 ├── E-001 Baseline (L387) → CLOSED (congelado: busca 0.045/0.696ms, 11k qps;
 │     15 achados; indexação 4.4s/doc; kernel 52.66/26.28 Mvec/s)
 │
 ├── E-002 Bounded search (L388) → INCONCLUSIVE → ERRATA E-003 (provider
 │     vetorial desligado — recall 39% INVALID) → E-004 re-run (L389):
 │     recall 81%, speedup 2.86x, pool de recência = causa da regressão
 │     → REFUTED (para substituir o full) → DECIDED: REJECT (ganho 0.5ms
 │     não compensa a regressão) → CLOSED
 │
 ├── H1 FTS domina o ranking (L389) → REFUTED (merge∩FTS 26% vs merge∩vec
 │     42%; o vetor domina o merge 4.2 vs 3.9) → DECIDED → CLOSED
 │
 ├── H2 triviais contaminam o top-10 (L390) → REFUTED (0.10/10 no top-10;
 │     o Ranker os bloqueia — score vetor 0) → DECIDED: não remover
 │     triviais de produção → CLOSED
 │
 ├── H3 recall semântico isolado (L391) → CONFIRMED (recall@10 1.0000 vs
 │     oracle float32; recall@20 0.9875) → DECIDED: índice saudável → CLOSED
 │
 └── E-005 Dedup pré-embed (L393) → CONFIRMED (mecanismo: DELETE antes do
       dedup — indexer.go:664-668; 99.6% do embed da re-indexação redundante;
       impacto ~25% — o FTS rebuild domina ~5s de 7.1s) → DECIDED: melhoria
       real mas não resolve sozinha → OPEN (depende de E-006)
       │
       └── E-006 Decompor o custo da re-indexação (L396) → CONFIRMED:
             perfil de CPU (pprof, 6.52s de re-indexação): SQLite/VDBE ~98%
             cum (FTS rebuild + graph persist + INSERTs); syscalls (I/O) 39%;
             scans das árvores B (BtreeNext 44% / MoveToLeftmost 41%);
             readDbPage 36% — o FTS rebuild (re-tokenização de 62k chunks)
             é o custo DOMINANTE, confirmado com evidência de perfil →
             DECIDED: E-007 (FTS incremental — equivalência) é o próximo
       │
       └── E-007 FTS incremental vs rebuild (L397) → CONFIRMED: equivalência
             provada nas 3 operações (insert/update/delete) — counts iguais,
             token achável igual, ranking 'vector' top-5 5/5; tempo: 701ms →
             10.2ms (insert, 69x) · 676ms → 7.1ms (update, 95x) · 698ms →
             121µs (delete, 5769x); o incremental (funções existentes,
             fts.go:264-310) é EQUIVALENTE ao rebuild → DECIDED: TRANSFORMAÇÃO
             CANDIDATA registrada (FTS incremental no storeDocument, eliminar
             o RebuildIndex do IndexDirectory) — AGUARDANDO autorização do Don
```

## 3. Erratas

| ERRATA | Origem | Problema | Validade | Correção | Re-run | Resultado original |
|---|---|---|---|---|---|---|
| E-003 | E-002 (L388) | provider registration absent | INVALID | registrar providers | E-004 (L389) | superseded (39% → 81%) |

## 4. Conclusão parcial (até L393)

- O ranking está saudável (H1/H2 refutadas; H3 PASS) — o problema NÃO está no
  embedding nem no ranking.
- O custo real da re-indexação é o FTS rebuild + graph (~5s de 7.1s) — o
  dedup pré-embed (~25%) é parte, não a solução.
- Próximos experimentos (uma pergunta cada): E-006 decompor o custo da
  re-indexação (FTS rebuild vs graph); E-007 FTS incremental (equivalência).

## 5. LIMITATIONS

Evidências valem para: corpus do Cosca (62.8k chunks), AMD Ryzen 7 5700X3D,
nomic-embed-text digest `0a109f...`, int16 AVX2. Não generalizar para outros
corpora/hardwares/providers sem novo experimento.

## 6. Provenance

L387 (`f8c1923`) · L388 (`d411798`) · L389 (`a063140`) · L390 (`edc853f`) ·
L391 (`08a3689`) · L392 (`af3887b`) · L393 (`c024836`) — todos assinados na
family chain.

## 7. TRANSFORMAÇÃO E-007 (autorizada em 2026-08-17 — contrato do Don)

### FASE 1 — PRE_STATE (registrado)
- commit: `466c139` · backup: `knowledge-pre-e007-transform-2026-08-17.db` (398MB, VACUUM INTO)
- corpus: 63.100 chunks · 51.250 vetores · 2.486 docs · FTS 63.100/2.486/3.148/9.110
- integridade: verify 1 issue (mismatch conhecido por design) · memory integrity OK · chain 1781
- referências de performance: rebuild ~700ms/operação (E-007) · re-indexação 6.52s (E-006)
- baseline: PERFORMANCE_BASELINE_L387_L390 (congelado) · 20 queries · oracle recall@10 1.0 (H3)

### FASE 2 — IMPLEMENTAÇÃO (em execução)
Escopo: integrar o FTS incremental (IndexDocument/IndexChunk/IndexCodeBlock/
RemoveDocument) no caminho real do indexer (storeDocument) + remover o
RebuildIndex do caminho normal do IndexDirectory.
Fora do escopo (NÃO alterar): ranking, busca, embeddings, provider, digest,
vector store, graph, chunking, dedup, contratos públicos, recovery,
provenance, outras otimizações.

### FASE 2 — IMPLEMENTAÇÃO (CONCLUÍDA — commit `88e593c`)
- storeDocument: RemoveDocument do oldID antes da substituição (update sem stale) + indexDocumentFTS (IndexDocument/IndexChunk/IndexCodeBlock — rowids dos content tables) após o commit
- IndexDirectory: batch RebuildIndex REMOVIDO (o RebuildAll intencional permanece — linha ~510)
- fts.go: IndexDocument → INSERT OR REPLACE; IndexChunk → delete-then-insert (FTS5 'delete' + INSERT) — re-indexação sem colisão nem tokens stale
- NADA fora do escopo alterado (ranking/busca/embeddings/provider/digest/vector/graph/chunking/dedup/contratos/recovery/provenance intactos)

### FASE 3 — GUARDIÃO (CONCLUÍDA — PASS)
- testes existentes: indexer ✓ sqlite ✓ knowledge ✓ (verdes)
- caminho REAL insert/update/delete: (a) INSERT achável sem rebuild ✓ (b) UPDATE antigo sumiu + novo presente ✓ (c) DELETE sumiu ✓
- counts FTS: incremental 63.107 = rebuild 63.107 ✓
- recall@10 (20 queries do baseline): 200/200 — nenhum resultado do rebuild ausente no incremental ✓
- ranking: coberto pelo recall@10 (os top-10 idênticos)
- benchmark: IndexDirectory sem mudanças ~0s (ANTES: ~700ms do rebuild por IndexDirectory) — + o E-007: 69x-5769x por operação
- integridade: verify 1 issue (o mismatch conhecido por design) — inalterado
- PRE→POST: 3 arquivos alterados (indexer 87+, fts 99, campanha) + knowledge.go limpo (lixo do agente removido)

### VERDICT (a entregar ao Don no formato do contrato)
PASS — a integração real reproduz o comportamento anterior sob o contrato
testado (o princípio da autorização cumprido).
