# ADR-028: Servidor MCP do COSCA — o "sistema nervoso" consultável via tools cognitivas (não funções cruas)

> **Status:** Accepted ✅ (aprovado pelo Don) | **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-08-29
> **Revisão:** aprovado — design aditivo, não quebra o Kernel.
> **A visão que este ADR formaliza (do Don):** o COSCA é um **cérebro consultável via MCP server** — NÃO um "runtime
> dentro do OpenCode". O OpenCode enxerga o COSCA como um **serviço cognitivo externo**; o MCP é o **sistema nervoso**,
> o Runtime é o **corpo**, o Kernel é o **cérebro**. **Não criar segunda implementação das regras do COSCA dentro do
> OpenCode.**
> **Nota de honestidade (auditada em código, não suposição):** o que já existe no repo é um **CLIENTE** MCP
> (`internal/chat/mcp`) e um **definidor estático de tools** (`internal/editors/generic_mcp`). O `cosca mcp serve`
> **AINDA NÃO EXISTE** — só `list/add/remove` (`internal/cli/mcp.go`). Este ADR decide como **construir o SERVIDOR** e
> as **tools cognitivas**, não descreve código pronto.

---

## 0. Contexto — o objetivo real (cérebro consultável, não runtime dentro do OpenCode)

O OpenCode hoje roda o COSCA como **ferramenta/agente embutido** — o que acopla o "corpo" e o "cérebro" a um único
host e impede o reuso. O desenho do Don inverte isso: o COSCA passa a ser um **serviço cognitivo externo**, exposto
por um protocolo aberto (MCP), de modo que **qualquer** agente/IDE (OpenCode, Cursor, Zed, Claude, um CLI próprio)
possa *consultar o cérebro* sem duplicar as regras.

```
   OpenCode  →  MCP COSCA Server  (sistema nervoso — interface FINA)   →  COSCA Runtime  (corpo)
              ────── pedir ──────                                       →  COSCA Kernel   (cérebro / governança)
                                                                        →  Memory | Knowledge | State
```

- **O OpenCode pede → o Runtime decide → o Kernel governa.** O MCP é **interface fina (nervos), NÃO lógica**.
- **NÃO acoplar o COSCA ao OpenCode.** O mesmo servidor MCP serve depois a outros agentes/IDEs.

### 0.1 O que JÁ EXISTE no repo (auditado — não duplicar)

| Peça | Onde hoje | Papel | Estado |
|---|---|---|---|
| **CLIENTE MCP** | `internal/chat/mcp` (`registry.go` + `client.go`) | O Cosca **consome** outros servers via stdio. `Registry` (Register/Remove/Servers/ToolNames/ResolveCall/ResolveTool + `ToolInfo`), `NewStdioClient` (JSON-RPC 2.0, NDJSON, sandbox `sbGate`, Command/Args/Env). | ✅ implementado |
| **Knowledge Engine** | `internal/knowledge` | `Engine.Search(ctx, search.SearchParams) (*search.SearchResults, error)` (FTS5+vetor+grafo, híbrido), `SearchSymbols`, `GetStats`. `SearchParams` tem `Query/Limit/Path/Tags/Epistemic[]/Scope/MinScore`; `SearchResult` tem `Type/Score/Content/Snippet/DocumentPath/Metadata[epistemic]/Source`. | ✅ implementado |
| **Epistemologia (classes)** | `internal/knowledge/epistemic_class.go` | `KnowledgeEpistemic` = **FACT / MEASURED / EVIDENCE / INFERRED / RULE / DECISION / PROFILE** (a *natureza* do que se sabe). Ortogonal ao `EpistemicStatus` (`epistemic.go`: KNOWN/SUPPORTED/UNCERTAIN/CONFLICTING/UNKNOWN/STALE, a *maturidade*). | ✅ implementado |
| **Proveniência (níveis)** | `internal/knowledge/provenance.go` | `ProvenanceLevel` **P0–P5** + `SourceRef` (repository/commit/path/sha256/retrieved) — a origem auditável e reproduzível. | ✅ implementado |
| **Memory** | `internal/memory` | `MemoryEngine.Store/Retrieve/Search(ctx, query, SearchOptions)`; `MemoryType` (decision/pattern/bug/agent/project/architecture/session), `MemoryLayer` (global/workspace/project/session/temp/long). | ✅ implementado |
| **Runtime (corpo)** | `internal/runtime` | `Start/Stop/Health/State/Metrics/Events/Lifecycle` + `Subsystem` (Name/Start/Stop/Health). Orquestra knowledge/discovery/memory/cache/compute/plugins/editors/watcher. | ✅ implementado |
| **Kernel (cérebro)** | `internal/kernel` | `EmergencyManager` (kill-switch `none/stop/halted`, `TriggerStop/IsHalted/SetShutdownFn/Clear`), `donauth`, `identity`, memória do Kernel. | ✅ implementado |
| **Percepção determinística** | `internal/vision` (`observation.go` + `pipeline.go`) | `Observation` (OBSERVED/TRACKED/PREDICTED/UNKNOWN + `Corroborate` + `EvidenceLevel`) e `AnalyzeVideo` (frame-a-frame, ffprobe/ffmpeg/OCR/pixel-diff, sem VLM). | ✅ implementado |
| **Trace / causal** | `internal/trace` | `TraceID` (TRACE-YYYYMMDD-XXXX), `Event`, `Sequence`, `DetectDivergence` (heurística determinística, sem LLM). | ✅ implementado |
| **Definição estática de tools** | `internal/editors/generic_mcp` | `CoscaToolDefinitions()` (cosca_search, cosca_index, cosca_context, cosca_status, cosca_memory, cosca_kernel_identity), `Setup` escreve `.mcp/cosca-server.json` + `.mcp/mcp.json`, `GenerateMCPConfig` aponta pra `cosca mcp serve`. | ✅ **só declarativo** — NÃO é um server real |
| **CLI do servidor** | `internal/cli/mcp.go` | `cosca mcp list/add/remove`, **feature gate** `COSCA_ENABLE_MCP` (fail-closed). **`serve` NÃO existe.** | 🔴 **lacuna** |
| **Registro de tools do consumidor** | `internal/cli/engine_builder.go` | O Cosca (como cliente) registra tools de `cfg.MCP.Servers` via `mcp.NewStdioClient` + `DiscoverTools`. | ✅ implementado |

**Conclusão do gap:** o Cosca sabe **consumir** MCP e sabe **declarar** (em JSON) que "tem tools". O que **falta** é o
**servidor MCP real** (o nervo) que **implementa** essas tools chamando o Runtime → Kernel → Memory/Knowledge, e que
devolva context packets com a **epistemologia e proveniência** que o Cosca já produz — não uma exposição crua de funções.

---

## 1. Decisão — SERVIDOR MCP do COSCA, interface fina, tools cognitivas

**Implementar um SERVIDOR MCP do COSCA** (JSON-RPC 2.0, stdio por padrão) que expõe **tools cognitivas** — cada uma
representa uma **CAPACIDADE COGNITIVA** (o que o cérebro faz), não uma função crua (banco/HTTP/shell). O servidor é
uma **camada fina de tradução**: recebe `tools/call`, roteia para o **Runtime** (corpo), que delega ao **Kernel**
(cérebro) para **governar** (kill-switch, RBAC, permissão de tool), e então acessa **Memory/Knowledge/State**.

**Princípio estrutural (a assinatura desta decisão):** *a tool não é a função; a tool é a **intenção cognitiva**.* O
`cosca.recall` não é "chama o Search do knowledge" — é "**o cérebro lembra** sobre X, com o nível de certeza que ele
tem". A implementação em Go **pode** delegar ao `knowledge.Engine.Search`, mas a semântica exposta, o filtro
epistemológico, a proveniência e o **context packet** são do COSCA — nunca um passthrough cru.

### 1.1 As tools cognitivas (superfície da capacidade)

| Tool | Capacidade cognitiva | Fonte real (não duplicar) | Semântica / retorno |
|---|---|---|---|
| `cosca.recall` | **Lembrar** | `knowledge.Engine.Search` + `SearchParams.Epistemic[]` + `provenance` | Busca semântica híbrida no conhecimento; devolve **context packet** com `source∈{FACT, MEASURED, INFERRED,...}`, `relevance`, `confidence`, `trace_id`. |
| `cosca.context` | **[tool central] Contextualizar** | `knowledge.Engine.Search`/`SearchSymbols` + `codeindex`/`codegraph` + `memory.Search` + `project/manifest` | Dado **arquivo/projeto/query** → packet do contexto **relevante para o agente usar** (como o Don descreveu). É a tool "o que o COSCA sabe sobre X?". |
| `cosca.learn` | **Aprender** | `memory.Store` + `internal/proposal`/`quarantine`/`gate` (ver tensionamento §1.2) | Registrar aprendizado com **proveniência**. **Resolve a tensão read-only-first**: escrita **controlada** — ver §1.2. |
| `cosca.observe` | **Perceber** | `vision.AnalyzeVideo` + `vision.Observation` | Percepção **determinística frame-a-frame** (OCR/pixel-diff) → eventos com estado epistêmico (OBSERVED/TRACKED). Sem VLM. |
| `cosca.reason` | **Raciocinar (cadeia causal)** | `internal/deliberate`/`proposal`/`decision` + `trace.causal`/`propagation` + `trace.DetectDivergence` | Cadeia causal / replay de raciocínio sobre um trace — **heurística determinística** (sem LLM como autoridade), apontando divergências. |
| `cosca.trace` | **Rastrear** | `internal/trace` (`trace.db`, append-only) → `trace.causal`/`store` | Traces/execuções/grafo causal de uma operação (`TRACE-...`): eventos, sequência, divergência. |
| `cosca.project` | **Orientar** | `internal/project/manifest` + `internal/bootstrap` + `internal/discovery` (stack) | Estado do projeto (bootstrap, stack detectada, contexto, manifest) — "quem está sendo observado?". |

> **Nota de naming:** usamos o **namespacing `cosca.<tool>`** — a ferramenta que o registry de MCP do Cosca já exige para
> resolver chamadas (`server.tool`, `registry.ResolveCall`, I8 default-deny). O servidor **anuncia** tools como
> `cosca.recall` etc.; o cliente (OpenCode) as enxerga agrupadas sob o namespace `cosca`.

### 1.2 A tensão read-only-first → `cosca.learn` como escrita **controlada**

O Cosca é **"read-only-first"** (ADR-009) — mas `cosca.learn` é, por natureza, uma **escrita**. Resolver a tensão
**explicitamente**, com a regra de ouro da casa (ADR-017 §4: *a saída externa é conteúdo, nunca autoridade; a escrita
em memória/conhecimento passa pelo caminho `proposal → quarantine → validation`*):

- **A tool NÃO grava direto na memória assinada.** O `cosca.learn` **submete um `proposal`** — a escrita real passa
  pelo `quarantine`/`gate`/`validation` normais do Kernel (ADR-012 §3.4, ADR-017 §4).
- **Duas modalidades explicitadas:**
  1. **`cosca.learn` (write-only via gate)** — se o chamador for **autorizado pelo Kernel** (RBAC/tool-permission §7),
     a proposta é **aceita e commitada** idempotentemente (ledger, I5). Read-only-first **não é violado**: é uma
     **escrita provada**, com `provenance ≥ P3/P4` e classe epistêmica atribuída (nunca `FACT` sem evidência).
  2. **`cosca.learn` (propose)** — modo **default fail-closed**: a tool apenas **propõe**; a consolidação só acontece
     se o gate do Kernel aprovar. Isto permite usar `learn` sem escrever, preservando a invariante I1/I2.
- **Proveniência obrigatória:** todo `learn` carrega `source`/`class`/`commit`/`sha256` (leitura `internal/provenance`).
  Sem ela → `P0` + status `UNCERTAIN` — **nunca** `FACT`.

---

## 2. Racional — por que tools cognitivas, por que MCP server fino, por que não acoplar

### 2.1 Tools cognitivas, não exposição crua

Se o MCP expusesse `knowledge.Search` / `memory.Store` / `runtime.Health` como ferramentas genéricas, o OpenCode
receberia **técnica** (uma API Go), não **conhecimento**. As regras de decisão — "INFERRED nunca vira FACT", "o
determinístico decide, a IA compreende", "falta de evidência não é evidência de ausência" — são **do COSCA**. Se o
OpenCode as reimplementasse, teríamos **duas versões das regras**, com o risco de divergência epistêmica.

**Tools cognitivas** (recall/context/learn/observe/reason/trace/project) expõem a **intenção** e **embutem todas as
regras do COSCA dentro do COSCA**. O OpenCode apenas **pergunta e usa** — nunca **re-decide**. É exatamente a
tradução do princípio do professor (ADR-017 §1): *"toda capacidade que a IA demonstra, o Cosca cristaliza em mecanismo
verificável"* — aqui, cristalizada como **superfície MCP**.

### 2.2 MCP server é interface fina (nervos), não lógica (cérebro)

| Camada | O que faz | NÃO faz |
|---|---|---|
| **MCP Server** (nervo) | Traduz JSON-RPC 2.0 (`initialize`/`tools/list`/`tools/call`), valida input contra o schema da tool, monta o **context packet**, injeta `trace_id`. | Não decide, não autoriza, não conhece as regras epistêmicas. |
| **Runtime** (corpo) | Orquestra os subsistemas; decide **que órgão** atenderá (knowledge vs memory vs vision vs trace). | Não governa autoridade (isso é do Kernel). |
| **Kernel** (cérebro) | **Governa** — kill-switch (`EmergencyManager`), RBAC, permissão de tool, gate de escrita. | Não executa a busca linear (é do corpo). |

A separação **nervo ↔ corpo ↔ cérebro** é o que dá a testabilidade: o nervo é fino e testável isoladamente; o corpo
(motor de busca, memória, visão) é determinístico; o cérebro é soberano (fail-closed).

### 2.3 Por que NÃO acoplar o COSCA ao OpenCode (reuso multi-agente)

Acoplar = o COSCA como plugin/skill embutida do OpenCode. Consequências:

- **Duplicação de regras** — o OpenCode reimplementa filtros/proveniência/políticas (anti-identidade).
- **Single-point-of-failure** — o COSCA só é consultável a partir do OpenCode.
- **Perde a visão do Don** — o mesmo COSCA deve servir **a outros agentes/IDEs** (Cursor, Zed, Claude, CLI próprio).
- **Quebra o mental model corpo/cérebro** — um runtime embutido não é um corpo externo consultável.

**A escolha a favor de MCP como fronteira** é também o que o ADR-017 §4 já definiu: *capacidade externa (aqui, a
*interface*) entra por um **adapter na borda** — nunca no Kernel*. O MCP server é exatamente esse adapter: a borda.
O cérebro permanece soberano.

### 2.4 Honestidade técnica: o que já existe vs o que é decisão nova

- O que é **reuso direto**: conhecimento/memória/runtime/kernel/vision/trace (tabela §0.1). Nada disso será duplicado.
- O que é **decisão nova** (e o "gap" real): (a) o **servidor MCP** (stdin/stdout stdio — hoje só existe o cliente),
  (b) o **dispatcher de `tools/call` → órgão**, (c) o **context packet** (schema + epistemologia + confidence),
  (d) a **camada de governança** (Kernel gate + RBAC de tool).
- O `internal/editors/generic_mcp` **continua válido como declarador de schema** (`.mcp/cosca-server.json`), mas a
  **execução real** delega ao novo servidor. O ADR mantém a compatibilidade (o `Setup` já gera `cosca mcp serve`).

---

## 3. Formato do context packet (o contrato do "cérebro consultável")

### 3.1 O schema (do Don) — o que o OpenCode recebe

```json
{
  "query": "como o Cosca lida com proveniência de evidência?",
  "context": [
    { "content": "FACT ...", "source": "FACT",     "relevance": 0.94 },
    { "content": "MEASURED ...", "source": "MEASURED", "relevance": 0.89 },
    { "content": "INFERRED ...", "source": "INFERRED", "relevance": 0.71 }
  ],
  "confidence": 0.91,
  "trace_id": "TRACE-20260829-7F92A1B3"
}
```

### 3.2 Como `source` (a epistemologia) alimenta o packet

O campo `source` do packet **é** a `KnowledgeEpistemic` (`internal/knowledge/epistemic_class.go`) — **não** um rótulo
de adivinhação. Mapa:

| `source` no packet | Classe epistêmica (code) | Pode virar "fato"? | Regra de confiança |
|---|---|---|---|
| `FACT` | `EpistemicFACT` | ✅ sim | Estabelecido com evidência validada. |
| `MEASURED` | `EpistemicMEASURED` | ✅ como medição (com margem) | Observação quantitativa; **nunca** vira `FACT`. |
| `EVIDENCE` | `EpistemicEVIDENCE` | ⛔ provisório | Observação registrada, ainda não validada. |
| `INFERRED` | `EpistemicINFERRED` | ⛔ **nunca** | Derivado; a regra de ouro da Fase 4 — nunca apresentado como fato. |
| `RULE` / `DECISION` / `PROFILE` | `EpistemicRULE`/`DECISION`/`PROFILE` | ⚠️ autoritativo na própria natureza | Não é medição-fato; rotulado como é. |
| `UNKNOWN` | (ausência/fallback) | ⛔ | "Sem evidência" — honesto, não especulativo. |

**A regra dura:** todo item de contexto carrega **uma** classe; um item que não pode ser classificado nas 7 classes
válidas (`Valid()==false`) é **descartado** (fail-closed, I2) — nunca incluído por engano. Isto espelha o fail-closed
de `SearchParams.Epistemic[]` já implementado no `knowledge.Engine`.

### 3.3 Como o `confidence` do packet é calculado (epistemologia + proveniência)

O `confidence` **não é um número mágico** — é derivado deterministicamente do que o Cosca já "sabe que sabe":

1. **Confiança por classe** (`KnowledgeEpistemic.Trustworthy()`): `FACT`/`MEASURED` → alta; `EVIDENCE`/`INFERRED` →
   baixa; `RULE`/`DECISION`/`PROFILE` → valor próprio (não é medição).
2. **Confiança por maturidade** (`EpistemicStatus`, `epistemic.go`): `KNOWN`/`SUPPORTED` → consistente; `UNCERTAIN`/
   `CONFLICTING`/`STALE` → rebaixa (um `FACT` com status `STALE` **não pesa** como `KNOWN`).
3. **Confiança por proveniência** (`ProvenanceLevel`, `provenance.go`): `P5` (múltiplas independentes) > `P4`
   (reproduzível) > `P3` (oficial) > `P2` > `P1` > `P0`.
4. **Confiança por relevância** — o `relevance` (0..1) vindo do score híbrido (FTS5+vetor+grafo) entra como peso.
5. **Agregação** (determinística): média ponderada dos itens retornados → **confidence** do packet. É **zero-LLM**
   (I1): a confiança é honesta por construção, não opinada.

**Convenção do `confidence`:** `>= 0.85` → "o cérebro afirma"; `0.55..0.85` → "o cérebro sugere"; `< 0.55` → "o
cérebro não sabe / especula" (o agente deve tratar como `INFERRED`-like, nunca como fato).

### 3.4 Como a percepção (`internal/vision`) alimenta o packet em `cosca.observe`

Em `cosca.observe`, o packet usa a primitiva de `observation.go`: cada evento derivado é uma `Observation` com
`EpistemicState` (OBSERVED/TRACKED/PREDICTED/UNKNOWN) e `EvidenceLevel` (NONE/LOW/MEDIUM/HIGH) via `Corroborate`.
Ex.: um evento `currency_changed` corroborado por **OCR dos números + pixel-diff** (`Corroborate("ocr_numbers")` +
`Corroborate("pixel_diff")`) sobe para **EvidenceHigh** → aparece no packet como `source: FACT` *da percepção* (não do
conhecimento). O `confidence` segue a mesma regra agrega → "o olho viu, com este grau de certeza".

---

## 4. Transporte / interface — MCP stdio (default) e a borda de registro

### 4.1 MCP stdio (mesmo transporte que o cliente já usa)

O cliente do Cosca (`internal/chat/mcp/client.go`) fala com servers externos via **stdio + NDJSON (JSON-RPC 2.0)**.
O servidor do Cosca deve **espelhar exatamente esse contrato** — mesmo formato de mensagens, mesmos métodos
(`initialize`, `tools/list`, `tools/call`), mesmos tipos (`ToolInfo`, `toolCallResult`). Simetria cliente↔servidor =
o Cosca é **um MCP citizen completo** (consome e é consumido), sem stack nova.

### 4.2 SDD vs SSE

- **MCP stdio (padrão do ADR)** — o servidor sobe como **subprocesso** do cliente. Vantagens: zero porta exposta,
  mesma auto-jail/sandbox (`sbGate`) que já protege o cliente, isolamento por processo. Custo: **um processo por
  cliente conectado**.
- **MCP SSE/HTTP (opcional, fase posterior)** — útil para **servir vários agentes** de uma vez (o "mesmo COSCA para
  outros agentes" do Don). Já existe a noção no `generic_mcp` (`GenerateMCPHTTPServerConfig` → `URL
  http://host:port/mcp`). **Não é o caminho default** para não introduzir superfície de rede (lei do Cofre, ADR-012).

### 4.3 Registro no cliente (o OpenCode enxerga o COSCA)

O `generic_mcp.Setup` já gera `.mcp/cosca-server.json` (schema) e `.mcp/mcp.json` (config do cliente com
`command: cosca, args: [mcp, serve], env: COSCA_MCP_TRANSPORT=stdio`). O `cli_editor_test.go` cobra esse caminho
(`.mcp/cosca-server.json` + `.mcp/mcp.json`). O registro no OpenCode segue o mesmo padrão de MCP, apontando para o
binário único `cosca` com `mcp serve` — o que aproveita o `cosca mcp` que já tem **feature gate fail-closed**
(`COSCA_ENABLE_MCP`, I2) e o **single-binary** (sem porta, sem sidecar).

> **Gap explícito a fechar (implementação):** o subcomando `cosca mcp serve` **ainda não existe** em
> `internal/cli/mcp.go` (só `list/add/remove`). Este ADR define que ele será o **lançador do servidor MCP** — binding
> do `internal/chat/mcp` (ou um `internal/mcpsrv`) + dispatcher de `tools/call` + assembler de context packet.

---

## 5. Alternativas rejeitadas

| Alternativa | Veredito | Motivo |
|---|---|---|
| **(a) Runtime dentro do OpenCode** (COSCA como plugin/skill embutida) | 🔴 **Rejeitado** | Acopla corpo+cérebro a um host, duplica regras, impede reuso multi-agente, quebra o mental model corpo/cérebro. É a **anti-tese** do desenho do Don. |
| **(b) RAG genérico via MCP** ("search this codebase" como tool) | 🔴 **Rejeitado** | Perde **epistemologia** (FACT/MEASURED/INFERRED), **proveniência** (P0–P5), **fail-closed epistêmico** (INFERRED nunca vira FACT) e o confinamento por `scope`/`class`. Devolve "pedaços" sem **dizer o que o COSCA sabe — e o que ele não sabe**. |
| **(c) Expor o REST existente diretamente** (knowledge/memory/runtime handlers) | 🟡 **Parcialmente rejeitado** | O REST (`api/rest`) já existe e pode ser **backing interno**, mas expô-lo cru via MCP dá **semântica de HTTP**, não **cognitiva** (sem context packet, sem confidence, sem trace). O MCP server **encapsula** o REST por trás das tools cognitivas, não o expõe. |
| **(d) Duplicar as regras do COSCA dentro do OpenCode** | 🔴 **Rejeitado** | Duas implementações dos filtros/proveniência/políticas → divergência epistêmica e risco de a "regra local" contradizer o cérebro. Regra de ouro: **as regras vivem no COSCA**, o agente só pergunta. |
| (incidental) **SSE/HTTP como único transporte** | 🟡 **Adiado** | Só para "servir vários agentes"; introduz superfície de rede (lei do Cofre). **Não** é o default. |
| (incidental) **Expor tools genéricas tipo `run_shell`/`read_file`** | 🔴 **Rejeitado** | São capacidade de **execução/arquivo** — não cognitiva. Fora do escopo "cérebro consultável"; pertence a outra fronteira. |

---

## 6. Consequências

### Positivas

- **Cérebro consultável** — o OpenCode (e qualquer agente) pergunta; o COSCA responde com **conhecimento + confiança +
  trace**, não com "código retornado".
- **Reuso multi-agente** — o mesmo servidor serve OpenCode, Cursor, Zed, Claude, CLI próprio (o "um COSCA para todos"
  do Don).
- **Governança via Kernel** — kill-switch (`EmergencyManager`), RBAC e permissão de tool **no caminho da tool**;
  a escrita de `learn` é **provada** (gate), não aberta.
- **Epistemologia preservada** — o context packet carrega a natureza e a maturidade do conhecimento; o agente **sabe
  quando o COSCA não sabe** (postura "incomplete, not evidence of absence", ADR-017/ADR-026 §2.4).
- **Zero-infra, single-binary** — stdio = subprocesso do cliente, sem porta, sem sidecar, Go puro sem CGO (ADR-002/013).
- **Simetria cliente↔servidor** — o Cosca consome MCP (`internal/chat/mcp/client.go`) e é consumido pelo MCP, com o
  mesmo JSON-RPC 2.0 (baixa curva, alta coerência).

### Negativas / trade-offs explícitos

- **MCP `tools/call` é síncrono/blocking para `recall`/`context`** — a busca híbrida (FTS5+vetor+grafo) roda no
  request, bloquando o chamador por dezenas de ms (o joelho de performance já medido na ADR-027: ~10ms p50 em 28k
  vetores). Para consultas pesadas, o cliente deve **usar `limit`/`scope`** e tratar o packet como sugestão, não como
  stream.
- **`cosca.learn` é uma escrita — precisa de gate.** O read-only-first **não** é violado, mas a comodidade de "aprender
  direto" é **limitada** pela governança (propose-first, default fail-closed). Isto é **intencional** (ADR-017 §4).
- **Disponibilidade depende do runtime estar no ar.** Se o `runtime` (corpo) não estiver `running`, o servidor MCP
  devolve **erro/health baixo** — o agente deve tratar como "cérebro indisponível", não como resposta vazia.
- **Um processo stdio por cliente** — N clientes conectados = N subprocessos do servidor (custo de boot/cold-start;
  o cold-path é o mesmo gap que a ADR-017 mediou no LLRT). Mitigável depois com o modo SSE/HTTP.
- **O `serve` ainda não existe** — até implementar, o `generic_mcp` gera a definição mas a execução real das tools
  estará **incompleta** (o "nervo" ainda não transmite). Este ADR define o destino; a implementação é faseada.

### O que NÃO muda

- **`internal/chat/mcp`** (cliente) — intocado; é a simetria que este servidor espelha.
- **`internal/knowledge`** (motor, epistemologia, proveniência) — intocado; é a fonte consultada, não duplicada.
- **`internal/runtime` / `internal/kernel`** — intocados; o servidor apenas **delega** para eles.
- **`internal/vision` / `internal/trace` / `internal/memory`** — intocados; são órgãos consultados.
- **Invariantes I1–I8** — nenhuma tool introduz LLM em caminho default (I1: o packet é zero-LLM), externo como
  autoridade (I8), ou runtime novo no núcleo (I7). O servidor é **aditivo** na borda.

---

## 7. Camadas e gates (a inteligência em camadas)

### 7.1 A cadeia de autoridade de uma `tools/call`

```
  tools/call (cosca.recall, args {query, limit, ep, scope})
       │
       ▼
  [MCP Server — nervo]  valida JSON-RPC + schema da tool, gera trace_id (internal/trace.NewID)
       │
       ▼
  [Kernel Gate — cérebro]  EmergencyManager.IsHalted() == none?  RBAC/permissão da tool OK?
       │  (se halted ou não-autorizado → erro fail-closed, NUNCA executa)
       ▼
  [Runtime — corpo]  decide o órgão (knowledge/memory/vision/trace) e delega
       │
       ▼
  [Órgão]  knowledge.Engine.Search / memory.Search / vision.AnalyzeVideo / trace...
       │
       ▼
  [Assembler do packet]  mapeia source←KnowledgeEpistemic, confidence←epistemologia+proveniência+relevance, monta context packet
       │
       ▼
  tools/call → { content: context packet, isError: false }
```

### 7.2 O que o Kernel governa (fail-closed)

| Gate | Fonte real | Comportamento |
|---|---|---|
| **Kill-switch** | `kernel.EmergencyManager` (`IsHalted()`) | Se `halted/stop` → a tool **não executa**; devolve erro. O Don pode **derrubar o cérebro** remotamente. |
| **RBAC / permissão de tool** | `kernel.donauth` + policy | Default-deny (I8): tools **não-listadas** ou de um **papel não-autorizado** → erro. Ex.: `cosca.learn` exige papel com permissão de **escrita**; `cosca.recall`/`context` são leitura (mais acessíveis). |
| **Escrita provada** | `internal/proposal`/`quarantine`/`gate` (ADR-012/017) | `cosca.learn` **não grava direto** — submete `proposal`; commit **idempotente** com ledger (I5). Read-only-first preservado. |
| **Epistêmico (na camada de dados)** | `knowledge` (`Trustworthy()`, `Valid()`, `SearchParams.Epistemic[]`) | Um item sem classe válida é **descartado**; `INFERRED` nunca é apresentado como `FACT`. |

### 7.3 A epistemologia como espinha dorsal do packet

As **duas** dimensões epistêmicas do COSCA **não podem** se perder na interface:

- **`EpistemicStatus`** (`epistemic.go`) — *maturidade* (KNOWN/SUPPORTED/UNCERTAIN/CONFLICTING/UNKNOWN/STALE). Alimenta a
  confiança (um `FACT` `STALE` não pesa como `KNOWN`).
- **`KnowledgeEpistemic`** (`epistemic_class.go`) — *natureza* (FACT/MEASURED/EVIDENCE/INFERRED/RULE/DECISION/PROFILE).
  Alimenta o `source` do contexto e a regra "INFERRED nunca vira FACT".

Juntas com a **proveniência** (`provenance.go`, P0–P5) e a **percepção** (`vision.observation.go,
OBSERVED/TRACKED/PREDICTED/UNKNOWN` + corroboração), elas tornam o **context packet** um artefato *epistemicamente
honesto*: o agente recebe não só **o que** o COSCA acha, mas **como** e **por que** (e quando não tem certeza).

---

## Referências

- **ADR-017** — Capability Borrowing Protocol / Capability Crystallization: por que a capacidade externa entra por
  **adapter na borda**, nunca no Kernel; e por que a escrita em memória passa por `proposal → quarantine → validation`.
- **ADR-012** — Arquitetura de 2 Zonas (Cofre/Kernel): lei do Cofre (NUNCA o contexto do cérebro à nuvem) + o
  `oracleGate` como ingress point, que este servidor **não** substitui (é fronteira externa, não o Cofre).
- **ADR-022** — Verificação & Proof: MCP Registry/Router → skill-surface (gema #6); aqui aplicada ao **servidor**.
- **ADR-009** — Change Safety Level (read-only): a base da tensão que `cosca.learn` resolve via gate.
- **ADR-002 / ADR-013** — Local-first, zero-infra, single-binary <100MB: por que **stdio + subprocesso** (não um
  serviço com porta) é o transporte coerente.
- **`internal/chat/mcp/client.go`** + **`registry.go`** — o contrato JSON-RPC 2.0 (NDJSON stdio) que o servidor
  **espelha**; `ResolveCall`/`ToolNames` (I8 default-deny).
- **`internal/knowledge/epistemic_class.go`**, **`epistemic.go`**, **`provenance.go`** — `source` e `confidence` do
  packet (epistemologia + proveniência).
- **`internal/search/search.go`** — `SearchParams` (incl. `Epistemic[]`/`Scope`) e `SearchResult` (score, path,
  metadata[epistemic]).
- **`internal/vision/observation.go`** + **`pipeline.go`** — percepção determinística e o estado epistêmico da
  observação (`cosca.observe`).
- **`internal/trace/trace.go`** — `TraceID`, `Event`, `DetectDivergence` (`cosca.trace`/`cosca.reason`).
- **`internal/editors/generic_mcp/generic_mcp.go`** — declarador de schema já existente (`.mcp/cosca-server.json`,
  `GenerateMCPConfig` → `cosca mcp serve`) que este ADR dá **execução real**.
- **`internal/cli/engine_builder.go`** — como o Cosca (cliente) já registra tools de `cfg.MCP.Servers`.

---

*Autor: cosca-architecture. Decisão fundamentada em auditoria de código (cliente MCP, knowledge/memory/runtime/kernel,
vision, trace, generic_mcp, CLI mcp) e nos invariantes I1–I8. O COSCA **não vira** um runtime dentro do OpenCode —
vira um **serviço cognitivo externo** (MCP server) que expõe **capacidades cognitivas** (recall/context/learn/observe/
reason/trace/project), preservando a epistemologia (source), a proveniência (P0–P5) e a governança (Kernel kill-switch/
RBAC). O `context packet` (query/context[{content,source,relevance}]/confidence/trace_id) é o artefato honesto: o
agente sabe **o que** o cérebro acha, **como** e **por quê** — e quando ele **não sabe**. A lacuna real a fechar é o
subcomando `cosca mcp serve` (hoje só `list/add/remove`), que será o lançador deste servidor. Proposta aguardando o Don.*
