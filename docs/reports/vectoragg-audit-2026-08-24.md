# vectoragg — Auditoria do Contrato vs Implementação (4 pontos do professor)

> Relatório de auditoria · 2026-08-24 · **READ-ONLY** — nenhum código alterado,
> nenhum dado migrado. O `vectoragg` foi usado como **instrumento de diagnóstico**
> da arquitetura modular (ADR-013), não como código de produção.
>
> **Ordem do Don + professor/advisor técnico:** AUDIT → EVIDENCE → VERDICT →
> ADR decision → TEST → IMPLEMENT. Este documento cobre os 3 primeiros (audit,
> evidência, veredicto). Nada foi corrigido — só auditado e documentado.

---

## 0. Escopo e contexto

O professor destacou o `internal/vectoragg/vectoragg.go` como um **espelho de
leitura** entre o legado (`knowledge.db`) e a arquitetura modular (ADR-013):
cria a **fronteira lógica** (module vector/graph/fts/projects → mesmo
`knowledge.db`) **antes** da **fronteira física**, com contrato **read-only** e
`ATTACH` `mode=ro`. O desenho foi **elogiado** (melhor que um `vector_rag.go`
tradicional). Mas o professor apontou **4 pontos** que exigem confirmação antes
de declarar a busca semântica modular como "funcionando".

O objetivo desta auditoria: **confirmar cada ponto contra o código real**, com
evidência, e registrar o veredicto — para que a correção (se houver) seja
cirúrgica, não uma nova cirurgia de 80k registros no escuro.

---

## 1. PONTO 1 — Isolamento real do QueryScope

**Alegação do professor:** existe `SearchScope` para confinar, mas é preciso
provar que `scope=vegetation → somente vegetation`, e que **não existe caminho
indireto** para materializar todos os módulos.

**Evidência (código real):**

- `selectModules(scope)` (L683) **filtra corretamente**: `scope==nil` → todos;
  `scope.NoRoute || len(Modules)==0` → nada; senão interseção com o catálogo. ✅
- MAS as leituras dentro de `QueryScope` (L584-620) chamam:
  - `a.Vectors(0)` (L593) — `limit=0` = **SEM LIMIT** (L377-378)
  - `a.Entities(0)` (L599) — idem
  - `a.Relationships(0)` (L603) — idem
  - `a.FTSBM25(q, 0)` (L610) — `limit=0` = **LIMIT -1** (L831-836, "no limit")
- Portanto, embora a **seleção** de módulos seja correta, o `QueryScope`
  **materializa TODOS os vetores/entidades/relações do módulo selecionado**
  (ex.: `scope={vector}` → `Vectors(0)` lê as 28.888 linhas), e `Counts()`
  (L616) é **incondicionalmente global**.

**Verdicto PONTO 1: CONFIRMADO (falha de isolamento físico).** O scope escolhe
os módulos certos, mas cada módulo é lido por inteiro (`limit 0`). O ruído
semântico é eliminado (só os módulos certos), porém **todo o peso físico** do
módulo é materializado. Isolamento **lógico** ✅, isolamento **físico** ❌.

---

## 2. PONTO 2 — Semântica do Counts()

**Alegação do professor:** o `Counts()` reporta contagem global, mas o nome/
contrato pode sugerir que é scoped.

**Evidência (código real):**

- `Counts()` (L333-363) conta **sempre e incondicionalmente** `vectors`,
  `entities`, `relationships`, `chunks_fts`, `entities_fts` — **sem** olhar o
  `selected`/`scope`.
- `QueryScope` chama `proj.Counts, err = a.Counts()` (L616) **após** já ter
  lido só os módulos selecionados. Ou seja: a projeção relata um `Counts`
  **global** mesmo quando leu um **subconjunto**.
- O cabeçalho do package (L51) diz: *"reads ONLY the modules named in a
  scope"* — o que o `Counts` **viola** (relata o global).

**Verdicto PONTO 2: CONFIRMADO (ambiguidade de semântica).** O contrato do
package sugere "scoped", mas `Counts()` implementa "global". Ambas são válidas,
mas o **nome/contrato não pode sugerir B enquanto implementa A**. Precisa
decidir: (A) `counts` = catálogo total, ou (B) `counts` = só o lido — e
documentar explicitamente.

---

## 3. PONTO 3 — Query original vs RouteID

**Alegação do professor (o mais delicado):** o pipeline ideal é
`QUERY → ROUTER → {modules} → busca restringida`, onde a **query original**
carrega o **conteúdo**, e o **router** determina o **espaço**. NÃO pode ser
`QUERY → ROUTER → RouteID → BM25`, porque o gatilho (RouteID) é o **rótulo da
rota**, não o conteúdo procurado.

**Evidência (código real):**

- `scopeQuery(scope)` (L743-751) retorna **`scope.RouteID`** quando não vazio.
- `QueryScope` usa `proj.FTSHits, err = a.FTSBM25(q, 0)` (L610), onde
  `q = scopeQuery(scope)` = **`RouteID`**.
- O `SearchScope.RouteID` (modlink L80) é *"deterministically identifies the
  matched route set (sorted, unique triggers joined by '|')"* — ou seja, é um
  **identificador da rota**, não a consulta.
- **Exemplo concreto:** query *"árvore urbana no terreno"* → router →
  `{vegetation, world, materials}` → `RouteID ≈ "vegetation.world.materials"`.
  O BM25 procuraria a string **"vegetation.world.materials"** em `chunks_fts`/
  `entities_fts`, em vez do conteúdo **"árvore urbana no terreno"**. Isso é
  procurar o rótulo em vez do texto.

**Verdicto PONTO 3: CONFIRMADO (falha conceitual grave).** O router deve
decidir **ONDE** (o scope), a query original deve carregar **O QUE** (o
conteúdo). Usar `RouteID` como BM25 **substitui** a intenção da pergunta pelo
rótulo da rota. O `SearchRequest` ideal precisa carregar `OriginalQuery`
**separada** do `SearchScope` (o router decide o espaço; a query decide o
conteúdo).

---

## 4. PONTO 4 — O fantasma dos 80k (custo de decodificação)

**Alegação do professor:** existe diferença entre "80k registros no banco" e
"80k embeddings decodificados". Se `Vectors(0)` decodificar todos os BLOBs, o
caminho de execução continua pesado, mesmo com a arquitetura modular correta.

**Evidência (medição read-only — `knowledge.db` aberto em `mode=ro`):**

```
TOTAL vetores:            28.888
Dim média:                768 (float32, little-endian)
Bytes totais de BLOB:     84,6 MB
RAM p/ decodificar todos: ~84,6 MB (float32)
```

- `Vectors(0)` (L368-418): `limit==0` → **sem LIMIT**; para cada linha, faz
  `decodeVector(blob)` (L412) → aloca `[]float32` de 768 dims.
- `QueryScope` chama exatamente `Vectors(0)` (L593). Portanto, uma busca
  roteada ainda pode materializar **84,6 MB de embeddings** na RAM.
- (Nota: o valor de produção é ~28.888 vetores válidos — o "80k" é o total de
  linhas do índice; o que importa é que **todos são decodificados** no `Vectors(0)`.)

**Verdicto PONTO 4: CONFIRMADO (o maior risco físico).** A arquitetura modular
está certa, mas o **caminho de execução** materializa o índice inteiro. O
roteamento eliminou o **ruído semântico**, mas **não** eliminou o **custo
físico** (memória + latência). O alvo é:
`Router → module → candidate retrieval → top-K → decode SÓ candidatos → rerank`.

---

## 5. Síntese dos veredictos

| Ponto | Confirmação | Severidade | Evidência |
|---|---|---|---|
| **1. Isolamento físico do QueryScope** | ✅ **Confirmado** | Alta | Choose módulos certos, mas `Vectors/Entities/Relationships(0)` materializam tudo |
| **2. Counts() semântica** | ✅ **Confirmado** | Média | `Counts()` é global (L333), contrato sugere scoped (L51); nome ≠ implementação |
| **3. Query original vs RouteID** | ✅ **Confirmado** | **Alta (conceitual)** | `scopeQuery`= `RouteID`; BM25 busca rótulo, não conteúdo (L743-751, L610) |
| **4. Fantasma dos 80k** | ✅ **Confirmado** | **Crítica** | 28.888 vetores × 768 = **84,6 MB** decodificados em `Vectors(0)` |

---

## 6. DECISÃO DO ADR (revisão pendente — NADA corrigido)

O `vectoragg.go` é um **excelente espelho de leitura** (contrato read-only por
construção, `ATTACH` `mode=ro`, fronteira lógica antes da física, camada
`Aggregator ≠ database ≠ source of truth`). Mas **não pode ser declarado como
"busca semântica modular funcionando"** até os 4 pontos serem resolvidos.
Nenhum deles desmonta o esqueleto — são **refinamentos de contrato e de
execução**.

### Correções recomendadas (nesta ordem — para a próxima fase)
1. **Ponto 4 (crítico):** `QueryScope` deve usar **candidate retrieval + Top-K**
   (`Vectors(limit>0)`, `FTSBM25(q, limit>0)`, etc.), e **decodificar SÓ os
   candidatos** — nunca o índice inteiro. Isso ataca simultaneamente ruído +
   espaço + memória + latência.
2. **Ponto 3 (conceitual):** `SearchRequest{OriginalQuery, SearchScope}` — o
   router decide o espaço; a **query original** carrega o conteúdo pro BM25.
   `RouteID` é metadado de rota, **não** o texto a buscar.
3. **Ponto 1:** após o Top-K, garantir que NENHUMA leitura materialize módulo
   fora do scope (confinamento físico, não só lógico).
4. **Ponto 2:** separar `Counts` global (`catalog_totals`) de `Counts` scoped
   (`scoped_counts`) — ou declarar explicitamente o contrato.

### NÃO corrigido nesta sessão (por ordem do professor)
> "Não mandaria ele corrigir nada ainda. Primeiro: AUDIT → EVIDENCE → VERDICT →
> ADR decision → TEST → IMPLEMENT."

Este documento é a etapa **AUDIT + EVIDENCE + VERDICT**. A correção (TEST +
IMPLEMENT) fica para a próxima fase, **após** o Don validar o veredicto.

## Anexo — arquivos auditados

- `internal/vectoragg/vectoragg.go` (L51, L333, L368-418, L584-620, L743-751)
- `internal/modlink/modlink.go` (struct `SearchScope`: Modules/RouteID/NoRoute)
- `.cosca/knowledge.db` (medição read-only, `mode=ro`)

Nenhum arquivo foi alterado. Nenhum banco foi migrado. O `knowledge.db` foi
apenas lido (`?mode=ro`).
