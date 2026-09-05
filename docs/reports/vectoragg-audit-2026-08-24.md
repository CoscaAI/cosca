# ADR-013 AUDIT — vectoragg

> Auditoria com **PROVA DE TESTE** (não só leitura estática) · 2026-08-24 ·
> **NO CODE CHANGED** — nenhum arquivo de código alterado, nenhum banco migrado.
> O `vectoragg` foi usado como **instrumento de diagnóstico**.
>
> **Método (professor/advisor técnico):** para cada ponto, distinguir
> `PASS` (comprovado), `FAIL` (comprovadamente errado), `AMBÍGUO` (contrato não
> define), `NÃO PROVADO` (código parece correto, mas falta teste). "*Não
> encontrei bug*" ≠ "*provei que não existe bug*".
>
> **Ordem:** AUDIT → EVIDENCE → VERDICT → ADR ACTION. Nada implementado. Testes
> read-only executados contra `.cosca/knowledge.db` (`?mode=ro`) e código real.

---

## PONTO 1 — QueryScope: Isolamento físico real

```
CONTRATO (package, L51):  "reads ONLY the modules named in a *SearchScope"
EVIDÊNCIA NO CÓDIGO:      selectModules(L683) filtra corretamente (nil→todos,
                          NoRoute→nada, senão interseção).
CAMINHO DE EXECUÇÃO:      QueryScope(L584) → selectModules → Vectors(0)/Entities(0)
                          /Relationships(0)/FTSBM25(q, 0) → Counts().
TESTE/PROVA:              selectModules escolhe os módulos certos (lógico OK).
                          MAS todos os readers são chamados com `limit=0`:
                            • Vectors(0)     → LIMIT ausente (L377-378) → lê tudo
                            • Entities(0)    → idem (L432-433)
                            • Relationships(0)→ idem (L473-474)
                            • FTSBM25(q, 0)  → LIMIT -1 = "no limit" (L831-836)
                          Counts() (L333) é incondicionalmente GLOBAL.
RESULTADO:                Isolamento LÓGICO ok; isolamento FÍSICO NÃO.
RISCO:                    module ≠ candidate set. Se um módulo (ex. vegetation)
                          contém subdomínios (tree/grass/shrub/forest), o router
                          escolhe vegetation mas a busca lê TODO o vegetation —
                          precisa de uma 2ª dimensão de filtragem por candidato.
VERDICT:                  **FAIL** (isolamento físico — materializa tudo no módulo)
```

**Sub-verificação (a armadilha do professor):** `SELECT ... FROM "vector".vectors`
— se o arquivo `vector.db` corresponder 1:1 ao módulo roteado, ok; mas o módulo
`vegetation` (lógico) pode mapear a **vários** subdomínios, e o scope atual
**não tem** dimensão abaixo do módulo. É a "2ª dimensão de filtragem" que falta.

---

## PONTO 2 — Counts(): semântica (global vs scoped)

```
CONTRATO (L51):           "reads ONLY the modules named in a scope" → sugere scoped
EVIDÊNCIA NO CÓDIGO:      Counts()(L333-363) conta SEMPRE vector+entities+
                          relationships+chunks_fts+entities_fts, sem olhar `selected`.
CAMINHO DE EXECUÇÃO:      QueryScope(L616): proj.Counts = a.Counts() — global,
                          mesmo quando leu um subconjunto de módulos.
TESTE/PROVA:              QueryScope lê SÓ os módulos selecionados, mas reporta
                          Counts GLOBAL (todas as tabelas do catálogo).
RESULTADO:                Nome/contrato sugere "scoped"; implementação é "global".
RISCO:                    Alguém vê "Scope: vegetation / Counts: 82.431 vectors"
                          e conclui "pesquisou tudo", quando internamente não.
VERDICT:                  **AMBÍGUO** — precisa decidir A(global) ou B(scoped)
                          e tornar o contrato EXPLÍCITO e coerente com o nome.
```

---

## PONTO 3 — Query original vs RouteID (o mais delicado)

```
CONTRATO (ADR §3.2):      Router decide ONDE; a query original carrega O QUE.
EVIDÊNCIA NO CÓDIGO:      scopeQuery(L743-751) retorna scope.RouteID (não a query).
                          SearchScope.RouteID (modlink L80) = triggers unidos por '|'
CAMINHO DE EXECUÇÃO:      SearchRequest → modlink → SearchScope → QueryScope
                          → scopeQuery → FTSBM25(q, 0) → SQL "MATCH ?"
TESTE/PROVA (executado, read-only):
  Query original: "árvore urbana no terreno"  → MATCH 'arvore urbana no terreno'  → 0 hits
  scopeQuery devolve (RouteID): "vegetation.world.materials"
      MATCH 'vegetation.world.materials'  → **FTS5 SYNTAX ERROR near "."**
      MATCH 'vegetation|world|materials'  → **FTS5 SYNTAX ERROR near "|"**
      MATCH 'vegetation world materials'  → 2 hits
RESULTADO:                O RouteID NÃO é o conteúdo e, pior, **quebra o FTS5**:
                          os separadores '.' e '|' do RouteID causam "syntax error" no
                          MATCH. Ou seja: FTSBM25(scopeQuery(scope), 0) SEMPRE falha
                          quando RouteID ≠ vazio — nem chega a buscar.
RISCO:                    Não é só "busca o rótulo errado"; é que **não busca**.
VERDICT:                  **FAIL** (conceitual + erro de execução real no MATCH)
```

**Decisão arquitetural a tomar (não é necessariamente código errado):** o
`SearchRequest` deve carregar `OriginalQuery` **separada** do `SearchScope`.
O router decide o espaço; a **query original** alimenta o BM25. `RouteID` é
metadado de rota, nunca o texto a buscar. Se mantiver `RouteID`, precisa
sanitizar sinais de FTS e redefinir a semântica.

---

## PONTO 4 — Vector materialization (o fantasma dos 80k)

```
CONTRATO (ADR §3.2/§6):   Buscar SÓ os candidatos do espaço roteado (top-K).
                          Router → module → candidate retrieval → top-K → decode
                          SÓ candidatos → rerank.
EVIDÊNCIA NO CÓDIGO:      Vectors(lim) (L368-418): lim<=0 → SEM LIMIT (L377-378);
                          para cada row chama decodeVector(blob) (L412).
                          QueryScope chama Vectors(0) (L593).
CADEIA MEDIDA (read-only, .cosca/knowledge.db?mode=ro):
  1. Rows lidas (SELECT sem LIMIT):            28.888
  2. BLOBs de vetor recebidos (not null):      28.888
  3. BLOBs decodificados (decodeVector):       28.888  (todos)
  4. Dim:                                       768 (float32)
  5. float32 alocados (todos):                  28.888 × 768 × 4B = ~84,6 MB
  6. Memória aproximada (peak allocation):     ~84,6 MB
RESULTADO:                Nenhuma camada reduz o conjunto antes da decodificação.
                          Router eliminou o ruído SEMÂNTICO, mas NÃO o custo FÍSICO
                          (memória + latência): ainda materializa ~84,6 MB na RAM.
RISCO:                    Alta — o caminho de execução continua pesado mesmo com a
                          arquitetura modular correta. NÃO resolve "80k na RAM".
VERDICT:                  **FAIL** (o maior risco — 84,6 MB decodificados)
```

---

## GLOBAL VERDICT — ADR-013 AUDIT (vectoragg)

| Ponto | Argumento | Vertict |
|---|---|---|
| **P1** QueryScope isolation | Seleciona módulos certos, mas materializa tudo no módulo (`limit 0`) | **FAIL** |
| **P2** Counts semantics | Global implementado, contrato sugere scoped — nome ≠ impl | **AMBÍGUO** |
| **P3** Original query vs RouteID | Busca o rótulo (RouteID), e ainda **quebra o FTS5 MATCH** | **FAIL** |
| **P4** Vector materialization | 28.888 emb. = **84,6 MB** decodificados na RAM (medido) | **FAIL** |

### Por eixo
- **Architecture:** ✅ sólida — espelho lógico antes do físico, `Aggregator ≠
  database ≠ source of truth`, contrato read-only por construção (`mode=ro`,
  `SetMaxOpenConns(1)`).
- **Runtime:** ⚠️ `QueryScope` pode materializar 84,6 MB e o `FTSBM25` com
  `RouteID` lança syntax error.
- **Performance:** ❌ top-K/candidate-retrieval ausente; `Vectors(0)` decodifica
  o índice inteiro.
- **Contract gaps:** (1) isolamento físico; (2) `Counts` global vs scoped;
  (3) `Query` vs `RouteID`; (4) materialização sem top-K.

---

## ADR ACTION

**Documento** (o esqueleto do vectoragg é correto e deve ser **KEEP** mantido)
**+ resolver os gaps antes de declarar "busca semântica modular funcionando".**

| Ponto | Ação |
|---|---|
| **P4** | **CHANGE** — `candidate retrieval + top-K` (decodificar SÓ candidatos). Ataca ruído + espaço + memória + latência juntos. |
| **P3** | **CHANGE** — `SearchRequest{OriginalQuery, SearchScope}`; query original → BM25; RouteID = metadado. Sanitizar se mantiver. |
| **P1** | **CLARIFY** — confinamento físico (não só lógico) + 2ª dimensão (candidate set dentro do módulo). |
| **P2** | **CLARIFY** — decidir global vs scoped e documentar; ou separar `catalog_totals` de `scoped_counts`. |

**NO CODE CHANGED** — este é o registro de proveniência. Nenhum arquivo de
código alterado, nenhum banco migrado/indexado/duplicado. O `vectoragg` foi
apenas **auditado e medido** (read-only). A correção (TEST + IMPLEMENT) fica
para a próxima fase, após o Don validar o veredicto.

---

## Anexo — evidência de teste (read-only)

- `.cosca/knowledge.db` aberto em `?mode=ro` (SQLite).
- `vectors`: 28.888 rows, dim 768, 84,6 MB de BLOB, decodificação = 84,6 MB.
- FTS5: `MATCH 'vegetation.world.materials'` → **syntax error**; `'vegetation|world|materials'`
  → **syntax error**; `'arvore urbana no terreno'` → 0 hits.
- Arquivos: `internal/vectoragg/vectoragg.go`, `internal/modlink/modlink.go`.
