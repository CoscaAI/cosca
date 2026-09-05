# FASE B — Mapa de Integração vectoragg × search (P0: MAPEAR, sem código)

> Documento de **mapeamento** (só leitura) · 2026-08-24 · **NENHUM código
> alterado**. Objetivo: provar se o `search` consegue fornecer **exatamente** o
> que o contrato novo do `vectoragg` exige, antes de IMPLEMENTAR. Ordem do
> professor: **MAPEAR → PROVAR → IMPLEMENTAR**. O `search` é a parte sensível —
> não se toca até o mapa estar aprovado.

---

## 1. O contrato novo do `vectoragg` (o que a Fase B vai consumir)

```go
type SearchRequest struct {
    Query        string               // conteúdo pesquisado (o que) → chega ao FTS MATCH
    Scope        *modlink.SearchScope // onde pode pesquisar (opcional) — confinamento físico
    CandidateIDs []string             // ids permitidos (fonte de candidatos) — NUNCA "ler tudo"
    TopK         int                  // top-K explícito; 0 = SEM resultados, nunca "sem limite"
}
```

**O que ele exige do `search`:**
1. `Query` — a string original da pergunta.
2. `Scope` — o `*modlink.SearchScope` roteado.
3. `CandidateIDs` — a lista de candidatos permitidos (a fonte).
4. `TopK` — o teto de resultados.

---

## 2. O que o `search` já fornece (mapeado no código real)

| Campo exigido pelo contrato | Existe no `search`? | Onde | Observação |
|---|---|---|---|
| `Query` | ✅ | `SearchParams.Query` (L73) | já é a query original |
| `Scope` | ✅ | `SearchParams.Scope *modlink.SearchScope` (L130) | já é o escopo roteado |
| `CandidateIDs` | ✅ | `SearchParams.CandidateIDs []string` (L108-112) | JÁ EXISTE e é usado (L409) |
| `TopK` | ✅ (parcial) | `SearchParams.Limit` (L76) | precisa ser convertido/mapeado |

**Conclusão do mapeamento: o `search` JÁ fornece os 4 itens.** Não há lacuna de
dados — há lacuna de **fiação** (o `CandidateIDs` já é usado, mas não vem do
`vectoragg`; o `vectoragg` não está conectado a lugar nenhum).

### Como o `CandidateIDs` funciona hoje (o ponto-chave da Fase B)

- `CandidateIDs` (L108) = IDs estilo FTS (`"chunks_fts_<rowid>"`).
- L409: `candidateIDs := e.resolveChunkCandidates(params.CandidateIDs)` —
  traduz IDs FTS em IDs de vetor (chunk).
- L411-434: se há candidatos, a busca vetorial é **confinada** a eles
  (`SearchWithMetrics`/`SearchWithCandidates`) + um `CandidatePool` — em vez do
  full-scan O(N) de 28.888.
- Se **não** há candidatos → **full-scan** (L436+), o "palheiro" que o Don
  quer eliminar.

**Ou seja: o mecanismo de candidate retrieval JÁ EXISTE no `search`** (Fase do
"hybrid-first L3 bounded"). O que falta é: **gerar** o `CandidateIDs` roteado
(letra do `vectoragg`) e **alimentar** o laço. O `vectoragg` provou (Fase A) que
com `CandidateIDs+TopK` ele decodifica SÓ 10, não 28.888.

---

## 3. O fluxo-alvo de integração (desenho proposto)

```
QUERY original
   │
   ├───────────────┐
   │               │
   ▼               ▼
modlink.Router   SearchParams.Query        ← Router resolve o espaço (onde)
   │               │
   ▼               │
SearchScope        │                        ← o scope (onde pode buscar)
   │               │
   └───────┬───────┘
           ▼
  (etapa lexical FTS produz CandidateIDs)  ← o "palheiro" é reduzido primeiro
           │
           ▼
  vectoragg.RetrieveCandidates{Query, Scope, CandidateIDs, TopK}
           │
           ▼
  decodifica SÓ os candidatos (TopK)        ← NUNCA 28.888 (invariante da Fase A)
           │
           ▼
       rerank → resultado
```

**Os dois "miolos" do mesmo pipeline:**
- O `search` já tem o **laço de busca** (FTS → vetor → grafo → rerank) e o
  **candidate retrieval** (L409-433).
- O `vectoragg` (Fase A) tem o **contrato correto** (não materializar tudo).
- **A Fase B fiou esses dois**: o `search` passa a **gerar `CandidateIDs`
  roteado** (do `Scope`) e o delega ao `vectoragg` para o retriveal confinado.

---

## 4. Alterações previstas na Fase B (para aprovação, NÃO para implementar agora)

| Arquivo | Mudança | Risco |
|---|---|---|
| `internal/search/scope.go` | `SearchWithRoute` passa a gerar `CandidateIDs` a partir do `Scope` e chamar o retriever do `vectoragg` | Médio |
| `internal/search/scope.go` | Conectar `vectoragg` como read-model confinado | Médio |
| `internal/search/search.go` | `Search()` passa a delegar ao `vectoragg` quando houver `Scope` roteado | **Sensível** (suíte gigante) |
| `internal/search/*_test.go` | Testes de integração (Fase B) provando que o retriever confinado é usado (e que o full-scan 28.888 **não** acontece quando há scope) | — |

**NÃO mexer (intocável):** `internal/modlink`, `internal/oracle`, `internal/sqlite/migrations.go`, `internal/embed/cosca` (chain), `.cosca/*.db`.

---

## 5. A decisão antes de IMPLEMENTAR (PROVAR, na linguagem do professor)

**PROVA a fazer na Fase B (teste que falha se o full-scan voltar):**
> Um teste de integração: busca com `Scope={vegetation}` (ou módulo existente)
> gera `CandidateIDs` → retriever do `vectoragg` → **decodifica SÓ os candidatos**,
> e **NÃO** 28.888. Se o `search` voltar ao `full-scan` (sem candidatos) quando há
> scope, o teste falha.

**Racional (por que essa prova importa):** sem ela, a Fase B "conecta" mas o
`search` pode continuar caindo no full-scan (L436) e o ganho da Fase A se perde
no pipeline. A prova garante que o **invariante da Fase A** (não materializar
tudo) sobrevive à integração.

---

## 6. Veredito do MAPEAMENTO (o que o professor pediu primeiro)

- ✅ O `search` **consegue fornecer** os 4 itens do contrato do `vectoragg`
  (Query, Scope, CandidateIDs, TopK≈Limit).
- ✅ O `search` **já tem** o candidate retrieval (L409-433) — não é para inventar.
- ⚠️ A lacuna real é **fiação**: gerar `CandidateIDs` roteado do `Scope` e
  delegar ao `vectoragg`, garantindo que NÃO caia no full-scan.
- 🔒 O `search` é **sensível** (suíte gigante) → IMPLEMENTAR em fase separada,
  isolada, com teste de prova do invariante.

**Este documento NÃO altera código.** Aprovação do Don + teste de prova antes de
IMPLEMENTAR. O `879eb2f` (Fase A) é a **âncora arquitetural**: qualquer regressão
de materialização (voltar a 28.888) é detectável imediatamente.
