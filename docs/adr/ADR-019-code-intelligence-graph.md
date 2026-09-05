# ADR-019: Code-Intelligence Graph — busca semântica de código determinística (compor, não duplicar)

> **Status:** Proposed | **Owner:** cosca-kernel + cosca-architecture | **Last Updated:** 2026-08-28
> **Revisão:** aguardando Don + cosca-cto. **Design aditivo — não quebra o Root.**
> **Referência (base):** mineração `DeusData/codebase-memory-mcp` (~40,9k ⭐, MIT) + `ADR-017` (Capability Borrowing Protocol) + `ADR-016` (Evolution Engine).

---

## 0. Contexto — a mineração que motivou isto

Ao minerar `codebase-memory-mcp` (C puro, binário único, 100% local, sem LLM/API key) surgiu um
insight que **valida a tese I1 do Cosca** com um projeto de ~40,9k estrelas:

> **"Você pode ter busca semântica REAL sobre código com custo ZERO-LLM e zero rede"** — um
> vetor de código int8 (~30 MB) embutido no binário + fallback determinístico (Random Indexing via
> xxHash) + quantização, sem chamar modelo nenhum. E o agente é o **tradutor**: o grafo responde em
> <1ms; quem entende é o LLM (o cliente).

O projeto resolve exatamente o que o Cosca chama de fronteira **`Domain Adapter → Generic Control
Plane`** e reforça: *o cérebro não precisa acreditar livremente; o grafo é determinístico, o agente
traduz.* O que muda a forma da solução é que **o Cosca JÁ TEM o substrato** — então isto é **reuso
por composição**, não construção do zero.

## 1. O que o Cosca JÁ tem (não reinventar)

| Capacidade | Onde está | Observação |
|---|---|---|
| **Vetor int8 SIMD** | `internal/vector` | `dot8.go`/`dot8_amd64.s`/`dot8_pure.go`, `dot16_*.go`, `sqlite_vec.go`, `int8_recall_test.go` — já é int8 + SIMD (memory-bandwidth-bound). **Mesma decisão de design do CBM.** |
| **Busca em camadas** | `internal/search` | `layered.go`, `scope.go`, `meaning_first_test.go` — layered/meaning-first. |
| **Grafo** | `internal/graph` | `graph.go`, `builder.go` — grafo de memória/conhecimento. |
| **Code graph (heurístico)** | `internal/codegraph` | `lang.go` (13 langs por extensão), `extract.go`, `build.go`. **"Heuristic v1 — structural precision, not AST"** (Go puro, zero deps). |
| **Embedding** | `internal/embed` | `embed.go` — camada de embeddings. |
| **SQLite + FTS5 + migrações** | `internal/sqlite` | `db.go`, `fts.go` (FTS5), `schema.go`, `migrations.go`. |
| **Integridade + publish** | `internal/integrity`, `internal/dbhealth` | verificação de integridade. |
| **MCP nativo** | `internal/chat/mcp` + `internal/editors/generic_mcp` + `internal/policy/mcp_policy.go` | cliente + servidor + default-deny. |

**Conclusão:** o Cosca tem o substrato (vetor int8, grafo, busca, FTS5, integridade). O que **falta**
é o **embedding de CÓDIGO determinístico** (sem LLM/rede) e a **fusão de sinais estruturais** — é aqui
que o CBM é ouro.

## 2. A DECISÃO — como fazer corretamente (o ponto do Don)

Dado que "temos muita coisa", o caminho correto é **COMPOR, não duplicar**:

- **Preservar binário único + SoT único + propriedade do diferencial** (o Cosca entende o próprio
  código). Não criar uma **camada semântica paralela** nem um **store paralelo**.
- **Não vendorar o CBM**: é C, binário externo, com *próprio* daemon/cache/embedding/grafo — seria um
  **segundo cérebro** para a mesma função cognitiva. Delega o que o Cosca deveria crescer.
- **Não copiar a arquitetura** (nem o C, nem o daemon/admission-barrier/cache-root, nem o blob nomic).
  Minerar a **IDEIA** (*busca semântica de código determinística, sem LLM*).

### 2.1 O que ADOTAR / ADAPTAR / COMPARAR / REJEITAR

| Ideia (do CBM) | Class. | Como entra no Cosca |
|---|---|---|
| **Embedding de código int8 + fallback RI determinístico** | 🟢 **ADAPTAR** (o ouro) | Construir o **próprio** vocab de código int8 (não o blob nomic), compondo sobre `internal/vector` (dot8/dot16 SIMD + `sqlite_vec`). Runtime determinístico (I1), zero LLM/rede. Carimbo `INFERRED` na saída (I4). |
| **Fusão de sinais estruturais** (TF-IDF, RRI, API/Type, AST-profile, dataflow, MinHash, proximidade, difusão) | 🟢 **ADAPTAR** | Portar a técnica de **similaridade sem LLM** para `internal/search`/`internal/graph`. Custo é por sinal; barato conceitualmente. |
| **Pipeline RAM-first multi-pass + publish atômico** | 🟡 **ADAPTAR** | Encapsular em `internal/codegraph`/`internal/indexer`: pipeline em memória → dump único com `staging + seal + rename` (fail-closed I2). |
| **Publish atômico: distinguir CORRUPT vs TRANSIENT** | 🟢 **ADOTAR** (lição) | Não quarentenar por `SQLITE_BUSY`/lock transitório; distinguir corrupção real de lock. Baixo esforço, anti-regressão. |
| **Coverage/metadata SEPARADA do grafo** ("não registrado ≠ completo") | 🟢 **ADOTAR** (epistêmico) | Métricas SOBRE o grafo nunca misturadas aos fatos; best-effort. **I3/I4 puro.** |
| **FQN + registry com confiança priorizada** (callback) | 🟡 **ADAPTAR** (princípio) | Ponte para o `internal/codegraph` evoluir de heurístico para type-aware, SEM reescrever do zero. |
| **Subconjunto openCypher read-only** | 🟡 **ADAPTAR** (prioridade menor) | Superfície Cypher-like sobre o grafo; a lição "fora do subset falha ALTO (nunca vazio silencioso)" é a joia. |
| **Daemon/admission-barrier/cache-root** | 🔴 **REJEITAR** | Máquina paralela que conflita com o lifecycle/daemon do Cosca. |
| **Fluxo `install`/44-surfaces de agente** | 🔴 **REJEITAR / nunca rodar** | Escreve config de 44 clientes de agente — I7/I8 proíbem em capacidade externa. |
| **Hybrid LSP (12 langs)** / cross-service | ⚪ **REFERÊNCIA** | Meses de trabalho; rota longa. Anotar como eixo futuro. |
| **Tool tiers Scout/Verify/Auditor** | 🔵 **COMPARAR** | Cosca já tem default-deny por capability-class (`internal/policy`); mais forte. Não duplicar. |

## 3. Fronteira de autoridade (ADR-017 §3 aplicado)

Se o CBM for usado **alguma vez** (prova de fronteira), entra **apenas** pelo Capability Adapter:

```
 KERNEL → decide/authorizes → MCP Adapter → CBM (subprocesso sandbox) → resultado → provenance → ledger
```

- **Nunca** ganha autoridade, confiança automática, acesso à memória, ou bypass de proposal/gate/sandbox.
- Rodar **sandboxed (I7)**: subprocesso na auto-jail/sandbox do Cosca, nunca root, escopo `realpath` a um
  **root de projeto**, **rede desligada**, cap de memória, escopo de dump-verify.
- **Nunca** executar `install`/`update`/`uninstall` do CBM.
- Saída = conteúdo externo → `contenttrust`, carimbo `INFERRED` (I4), registrada no ledger (I5),
  quarentenada até validação (I6).
- **Tratado como capacidade de INSPEÇÃO**, nunca como fonte de verdade do conhecimento.

## 4. Plano faseado (compõe sobre o que existe)

| Fase | Entrega | Compõe sobre | Ix |
|---|---|---|---|
| **F0** | Ratificar este ADR. | — | — |
| **F1** | **Embedding de código int8 determinístico** + fallback RI + quantização. Novo artefato (ex. estender `internal/embed` ou criar `internal/codeembed`). Vocab PRÓPRIO. | `internal/vector` (dot8/dot16 SQL int8) + `internal/sqlite` (FTS5) | I1/I2 |
| **F2** | **Fusão de sinais** para similaridade estrutural sem LLM. | `internal/search` (layered) + `internal/graph` | I4 |
| **F3** | **RAM-first multi-pass + publish atômico** em `internal/codegraph`/`indexer`. | `internal/sqlite` + `internal/integrity` | I2/I5 |
| **F4** | **Coverage separada do grafo** (I3/I4) + carimbo epistêmico por resultado. | `internal/provenance` + `internal/knowledge/epistemic*` | I3/I4 |
| **F5** *(opcional)* | CBM via MCP Capability Adapter como **prova de fronteira escopada e sandboxed**. | `internal/chat/mcp` + `internal/policy` + jail | I3–I8 |

> Cada fase é **aditiva** e **avaliada** (benchmark antes/depois — a régua de cold-start/perf do
> `internal/performance` já existe). Nada promove sem passar pelos gates (I1).

## 5. O que NÃO fazer (anti-roadmap)

1. **NÃO vendorar o CBM** (código C) nem o blob nomic no binário Go.
2. **NÃO criar camada semântica paralela** ao `internal/search`/`internal/graph` existente.
3. **NÃO** adotar o daemon/admission-barrier/cache-root do CBM.
4. **NÃO** rodar `install`/`update`/44-surfaces do CBM (I7/I8).
5. **NÃO** tratar a saída do grafo/similaridade como **FACT** — é `INFERRED` (I4).
6. **NÃO** construir Hybrid LSP agora (meses) nem cross-service (rota longa).
7. **NÃO** duplicar o mecanismo de permissão (o default-deny do Cosca já é mais forte).

## 6. Recomendação

1. **ADOTAR este ADR como o guia** para o code-intelligence do Cosca: **compor sobre o substrato**,
   construir o **embedding de código determinístico** (F1) — o máximo valor, alinhado a I1.
2. **Rejeitar o CBM como backbone**; usá-lo, no máximo, como **prova de fronteira** (F5) para
   exercitar o Capability Adapter (ADR-017) sob o regime I3–I8.
3. **Sequenciar F1→F4** com evidência (benchmark) em cada passo; F5 é opcional e só se o Don quiser
   stress-testar a fronteira.

---

*O Cosca aprende a **ideia** do CBM (busca semântica de código determinística, sem LLM — que alinha I1
como quase nada) e não a **arquitetura**. Compõe sobre o que já existe, preservando binário único, SoT
único e a soberania do cérebro (gate ZERO-LLM) sobre tudo que vem de fora.*
