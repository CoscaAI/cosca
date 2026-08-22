# COSCA ARCHITECTURAL HEALTH REVIEW

> Revisão arquitetural · 2026-08-17 (L367) · READ-ONLY — nenhuma
> implementação, nenhuma migração, nenhuma alteração de produção.
> Régua de ouro: *"antes de perguntar como tornar um módulo mais rápido,
> pergunte se ele deveria estar fazendo aquele trabalho."*

## A) Mapa arquitetural

```
140 pacotes em internal/ — áreas principais por peso:
knowledge 19.8k LOC (motor de conhecimento) · runtime 12.1k (daemon/serve)
memory 8.0k (chain, blocks, merkle) · vector 7.0k (índice in-memory)
chat 4.7k (executor, ferramentas, sandbox) · search 4.6k (híbrido FTS+vetor)
chunker 2.2k · markdown 1.2k · supervisor 2.3k · policy 0.3k (novo, L366)
+ providers (12), editors, agentes, gate, integrity, metrics, etc.
```

## B) Mapa de dependências (crítico)

```
knowledge → chunker → markdown           (ingestão — CORRIGIDA L356)
knowledge → vector (índice int16)        (busca — OTIMIZADA L339-L363)
knowledge → embeddings (nomic/ollama)    (chamada externa — 1 por busca)
search → FTS + vector + ranking          (caminho híbrido)
chat → executor → policy (L366) + sandbox (execução — guard novo)
runtime → knowledge + providers + índice (daemon — índice em memória por processo)
memória → blocks + chain + merkle        (imutável, ORDEM SAGRADA)
```

## C) Mapa de fluxo (caminhos críticos)

```
BUSCA:  query → embed (nomic, 1 chamada) → FTS → candidates → índice int16
        (scan ~600µs) → materialização (fetch colunas) → ranking → resposta
INGESTÃO: arquivo → parser (1 nó/parágrafo ✓ L356) → filtro T1-T4 ✓ L353
        → chunks → embeddings → store
MEMÓRIA: decisão → bloco → sha256 → chain → merkle → commit → sign
EXECUÇÃO (chat): proposta → validação → POLICY (L366, disponível) → sandbox
        → tool
```

## D) Gargalos medidos (EVIDENCE da campanha)

```
1. BUSCA: 2.8ms pós-corpus-51k (L364) — o scan int16 ~600µs; o RESTANTE é
   materialização/fetch de colunas + FTS + ranking. O gargalo de produção é
   PÓS-scan (apontado na FASE 1, L343; nunca instrumentado por fase).
2. ÍNDICE: int16 a 51.122 vetores = 78.5MB ≈ 82% do L3 96MB — zona de
   transição LRU (FASE 3 mapeou); latência pode subir (monitorar).
3. RE-EMBED: ~133 emb/s (nomic) — o throughput do provider é o teto para
   ingestão em lote.
4. FTS por busca: custo NUNCA isolado (UNKNOWN → medir).
```

## E) Desperdícios prováveis (a régua: "deveria fazer esse trabalho?")

```
1. MATERIALIZAÇÃO PÓS-SCAN: o Search busca colunas completas (content,
   metadata) dos top-K — necessário, mas domina a latência vs o scan.
   (O kernel não é o problema — o fetch é.)
2. DEDUP NÃO-PREVENTIVO: o dedup_of foi uma LIMPEZA ÚNICA (L360), não um
   mecanismo de ingestão. Um documento NOVO com conteúdo duplicado ganha
   vetor sem dedup → a duplicação pode REAPARECER. O filtro T1-T4 é
   preventivo; o dedup não é. (Risco real — ver F.)
3. MARCAÇÕES ÓRFÃS: 8.734 is_trivial + 3.248 dedup_of permanecem no db
   (peso morto ~12k chunks que nunca terão vetor) — limpeza é campanha
   própria (pendente).
4. WATCHER DESATIVADO: o serve tem watcher mas watch:false — o
   conhecimento só re-indexa manualmente; os órfãos se acumulam entre
   indexações (o re-embed L364 foi manual).
5. PROVIDER "local" test-deterministic: registrado (testes) — risco de
   confusão de configuração; produção usa nomic (correto).
```

## F) Riscos

```
1. DEDUP REAPARECE (médio-alto): sem dedup preventivo na ingestão, o
   estoque limpo (L360) volta a sujar com o tempo.
2. LATÊNCIA PÓS-51k (médio): int16 ~82% L3 — monitorar; se degradar,
   reavaliar int8 (39MB, folga) ou o regime.
3. GUARD NÃO-ATIVO (médio): o policy (L366) está pronto/testado mas o
   runtime não o anexa por padrão — a proteção depende de ativação.
4. ÍNDICE POR PROCESSO (baixo): o serve mantém o índice em memória —
   mudanças de db exigem reinício para refletir (a limpeza exigiu).
5. CONHECIMENTO PARCIAL (informativo): 51.122 de 62.954 chunks têm vetor —
   os ~11.8k marcados (triviais/duplicados) nunca terão (por design).
```

## G) Oportunidades classificadas por ROI

| # | Oportunidade | ROI | Confiança | Risco |
|---|---|---|---|---|
| 1 | Instrumentar a busca por fase (FTS vs scan vs materialização) | alto | alta | baixo |
| 2 | Dedup PREVENTIVO na ingestão (evitar re-sujar) | alto | alta | médio |
| 3 | Guard ativo por padrão no runtime | médio | alta | baixo |
| 4 | Auto-index (watcher ligado) | médio-alto | média | médio |
| 5 | Formalizar REJECTED no fluxo de memória | médio | média | baixo |
| 6 | Isolar custo do FTS por busca | médio | média | baixo |

## H) Experimentos recomendados (read-only, na ordem)

```
1. pprof/instrumentação do Search por fase (FTS, scan, materialização) —
   o breakdown da latência 2.8ms. (A "FASE 6" da campanha, nunca feita.)
2. Teste: documento novo com conteúdo duplicado → o vetor é criado sem
   dedup? (confirma o risco de reaparecimento da duplicação).
3. Medir o FTS isolado (custo por busca) e o fetch de colunas.
4. Snapshot de latência pós-51k vs pós-9.7k (L357 vs agora).
```

## Resultado final

**Estado atual**: sólido. O núcleo (busca, ingestão, memória) foi
corrigido/otimizado com evidência; a governança tem os 3 níveis (texto,
código, sandbox); o conhecimento está ~completo e buscável com int16
quase-oracle.

**Pontos fortes**: epistemologia praticada (errata, UNKNOWN, evidência
antes de decisão) · fábrica corrigida (parser/filtro) · estoque limpo ·
int16 default com recall ~0.995 · tie-breaker determinístico · guard
disponível.

**Dívida técnica**: dedup não-preventivo · guard não-ativo · materialização
não-instrumentada · watcher desligado · marcações sem limpeza futura.

### ONDE NÃO MEXER (sem evidência de problema)

```
✗ Kernel AVX2/int16 (quase-oracle, testado) — não tocar.
✗ Parser/chunker (corrigido L356, testado) — não tocar.
✗ Seleção de representação int16 > int8 > float32 — estável.
✗ Memória/chain (imutável, funcional) — não tocar.
✗ Policy (novo, testado) — só ATIVAR quando decidido.
```

**Regra**: nenhum módulo funcionando é alterado sem problema provado
(UNKNOWN → medir primeiro). A revisão não propõe nenhuma implementação —
apenas os experimentos read-only acima para reduzir os UNKNOWNs.
