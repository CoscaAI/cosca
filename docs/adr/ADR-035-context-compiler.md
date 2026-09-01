# ADR-035: Context Compiler — o menor contexto suficiente para cada decisão (e a resposta mínima para executar)

> **Status:** Proposed (aguardando aprovação do Don) | **Owner:** cosca-architecture + cosca-kernel | **Last Updated:** 2026-09-01
> **Revisão:** decisão de DESIGN — NÃO implementada. Define o norte e o plano; a implementação é incremental, cada fase com gate de testes.
> **Fonte (ordem do Don, 2026-09-01, conversa com o professor):** "como criar algo eficiente pra chegar o contexto no llm compactado e eficiente e trazer a resposta curta e necessária pra execução". O professor: *"Dado tudo que o COSCA sabe, qual é o menor contexto suficiente para esta decisão?"* — e *"não construir um 'resumidor de contexto'. Construir um compilador de contexto orientado à tarefa."*
> **Relação:** promove a descoberta da sessão (o gasto era replay MCP + contexto empilhado) a **mecanismo de eficiência estrutural**: o COSCA já tem retrieval por relevância, epistemologia por fragmento, deliberação determinística e budget de custo — falta o **orquestrador** que seleciona, compila, dá budget por seção, exige resposta estruturada e escala L0→L2 conforme a confiança.

---

## 0. Mapa honesto do estado atual (o que JÁ EXISTE — crítica antes do gap)

> **Nota de veracidade (auditada em código, 2026-09-01):** o COSCA **já tem ~70% das peças** do Context Compiler. O que falta NÃO é criar do zero — é **orquestrar** o que existe na ordem certa.

| Peça do Context Compiler | COSCA hoje | Onde | Estado |
|---|---|---|---|
| **Seleção por relevância** (não comprimir tudo) | Retrieval híbrido FTS+vetor+grafo+re-rank com score por item | `internal/search`, `internal/ranking` | ✅ canônico e testado |
| **Proveniência por fragmento** | Classes epistêmicas FACT/MEASURED/EVIDENCE/INFERRED prefixadas no contexto (`concatEpistemic`) | `internal/knowledge/evidence.go`, `internal/engine/concat_epistemic.go` | ✅ já exibe `[INFERRED] ...` ao agente |
| **"Estado de execução" estruturado** | `BuildCleanContext` (Deliberator, ADR-032) — bloco determinístico pré-LLM | `internal/orchestration/deliberation.go` | ✅ ativo no serve (ADR-032 implementado) |
| **Decisão trivial sem LLM** | Modo determinístico — "a IA é o último recurso" (responde SEM LLM quando knowledge.db tem score≥0.6) | `orchestration/executor.go` | ✅ já decide "responder sem LLM" (heurística de snippet) |
| **Confiança → decisão/escalada** | `deliberate` (convergência A3, confiança A4, spec A7) + `confidence.Tracker` | `internal/deliberate/*`, `internal/confidence` | ✅ pronto, parcialmente conectado (ADR-032) |
| **Budget de custo** | `CognitiveBudget` (US$0.05/8000 tokens/20s) + UsefulWork/Efficiency (ADR-031) | `internal/cost`, `orchestration/executor.go` | ✅ em todos os caminhos LLM |
| **Resposta estruturada (tool calls)** | ResponseParser extrai function calls do LLM | `internal/engine/parser.go` | ✅ para tools; ❌ para DECISÃO estruturada |
| **Trace/identidade de execução** | Trace ID universal + flight recorder | `internal/trace` | ✅ `trace_id` em todo packet MCP |
| **Proteção de replay** (custo duplicado) | replayStore (hash name+args, TTL 5min) no MCP | `internal/mcpserver/server.go` | ✅ implementado e validado (2026-09-01, commit `ab2c162`) |

**Conclusão honesta:** o COSCA **retrieva com relevância**, **classifica por epistemologia**, **decide sem LLM quando pode**, **mede custo** e **protege contra replay** — mas o contexto entregue ao LLM é uma **concatenação de seções** (system prompt + memória + knowledge + history), não um **estado operacional compilado** com orçamento por seção. E a resposta é **prosa + tool calls**, não um **instruction packet** (DECISION/TARGET/ACTION/ARGUMENTS/CONFIDENCE/VERIFICATION).

---

## 1. Contexto / Problema

O custo observado pelo Don ("estava gastando horrores", "vejo o pensamento dele e ele repete") teve DUAS causas, ambas agora endereçáveis:

1. **Custo de replay** (JÁ CORRIGIDO): o MCP re-executava a mesma tool a cada reenvio do cliente. Corrigido com timeout + replayStore (commit `ab2c162`). **Este ADR NÃO reabre isso.**

2. **Custo de contexto** (ESTE ADR): o LLM recebe **o cérebro inteiro** (dentro do possível) em vez do **menor contexto suficiente para a decisão atual**. O professor formalizou:

```
RAW KNOWLEDGE (429MB) → RETRIEVAL (2.4MB) → RELEVANCE FILTER (180KB) → CONTEXT COMPILER (11KB) → LLM (~1-3K tokens) → ACTION PACKET (~200 tokens) → RUNTIME
```

**Problema em uma frase:** o COSCA sabe muito, mas **não sabe qual pequena parte do próprio conhecimento precisa pensar** para executar a ação atual — e o LLM devolve prosa que o runtime precisa interpretar, em vez de um instruction packet que o runtime executa.

**Decisão do Don/professor:** construir um **compilador de contexto orientado à tarefa** (não um resumidor): seleção → compilação em estado operacional → budget por seção → LLM → resposta estruturada → execução. Com **contexto progressivo** (L0 → L1 → L2) para custo adaptativo.

---

## 2. Decisão — Context Compiler em 4 estágios, reusando o que existe

```
                   COSCA
                     │
             ┌───────▼────────┐
             │ Context Router  │  ← NOVO: seleção por score (similaridade + dependência + frescor + autoridade + task_affinity)
             └───────┬────────┘
                     │
         ┌───────────▼───────────┐
         │ Context Compiler      │  ← NOVO: monta o bloco estruturado com BUDGET por seção
         │                       │
         │ TASK (5%)             │
         │ STATE (15%)           │
         │ CONSTRAINTS (10%)     │
         │ FACTS (25%)           │  ← usa search/ranking (relevância) + evidence (epistemologia)
         │ EVIDENCE (25%)        │
         │ MEMORY (10%)          │
         │ OUTPUT_CONTRACT (10%) │
         └───────────┬───────────┘
                     │
               compact context
                     │
             ┌───────▼───────┐
             │      LLM      │
             └───────┬───────┘
                     │
               structured output (instruction packet)
                     │
             ┌───────▼───────┐
             │ Action Decoder│  ← NOVO: DECISION/TARGET/ACTION/ARGS/CONFIDENCE/VERIFICATION
             └───────┬───────┘
                     │
                Runtime/Tool
```

### 2.1 Context Router (seleção orientada à tarefa)

Em vez de "resuma 20.000 tokens", pergunta **"para executar ESTA tarefa, quais informações são necessárias?"**. Score por fragmento:

```
relevância = semantic_similarity + dependency_score + recency + authority + task_affinity
```

Reusa: `internal/search` (similaridade), `internal/ranking` (re-rank multi-fator). Novo: `task_affinity` (o estado da tarefa decide quais seções importam).

### 2.2 Context Compiler (estado operacional, não documentos empilhados)

Em vez de `DOCUMENTO:... MEMÓRIA:... HISTÓRICO:...`, entregar:

```
TASK:        fix MCP endpoint mismatch
STATE:       server=internal/mcpserver · tools_expected=11 · tools_advertised=6
CONSTRAINTS: - não criar contratos duplicados · - preservar API · - testes verdes
FACTS:       [F:FACT] server expõe 11 tools · [M:MEASURED] go test ./... = PASS
EVIDENCE:    - generic_mcp anuncia 6 tools obsoletas · - registry.go importado por serve.go
TARGET:      make adapter reflect server registry
ALLOWED:     editar arquivos existentes · rodar testes
FORBIDDEN:   criar implementação paralela · deletar dados legados
```

Reusa: `BuildCleanContext` (ADR-032) como base — **evolui**, não substitui. Novos: o template estruturado + o **Context Budget** (alocação de tokens por seção, realimentando o `cost.UsefulWork`).

### 2.3 Resposta estruturada (Action Decoder)

O LLM devolve um **instruction packet** (JSON), não prosa:

```json
{
  "decision": "EDIT",
  "target": "internal/mcpserver/registry.go",
  "action": "align_registry",
  "confidence": 0.94,
  "reason": "single source of truth is internal/mcpserver",
  "verification": ["go test ./internal/mcpserver/..."]
}
```

Reusa: `ResponseParser` (engine) e `parseToolArgs` (orchestration) como base. Novo: `ActionDecoder` com **fallback fail-open para prosa** (se o LLM não devolver JSON válido, trata como resposta textual — NUNCA quebra).

### 2.4 Contexto progressivo (custo adaptativo)

```
L0 → estado mínimo (TASK + STATE + OUTPUT_CONTRACT)
     ↓
     resolve? (deliberate: confiança ≥ threshold)
     ├── SIM → executa SEM LLM (modo determinístico já existe!)
     └── NÃO → L1 (FACTS + CONSTRAINTS) → resolve? → L2 (EVIDENCE + MEMORY) → LLM
```

Reusa: `deliberate` (confiança) + o **modo determinístico já existente** (executor.go). Novo: a **escada de camadas** (escalar só quando a confiança cai).

---

## 3. Consequências

### Positivas
- **Custo adaptativo**: LLM recebe ~1-3K tokens compilados, não o cérebro inteiro; resposta ~200 tokens, não prosa.
- **Runtime cognitivo**: o LLM vira um **coprocessador** — o runtime executa DECISION/TARGET/ACTION, não interpreta texto.
- **Epistemologia em uso**: `[F:FACT]` vs `[I:INFERENCE]` no contexto reduz o problema clássico (modelo confundiu hipótese antiga com estado atual).
- **Realimenta o custo**: o Context Budget por seção alimenta `cost.UsefulWork` (ADR-031) — a medição existe.

### Negativas / Riscos
- **Mudança de contrato LLM**: exigir JSON estruturado pode **degradar qualidade** se o prompt não for perfeito → **fallback fail-open obrigatório** (prosa aceita).
- **Budget por seção é arte**: alocar mal (STATE demais) piora em vez de melhorar → medir com `cost` e ajustar.
- **Escopo**: NÃO é um passo da Etapa 3 — é uma **fase própria** com gate do Don.

### Não-faz (limites)
- NÃO cria um "resumidor" genérico — seleciona primeiro, compila depois.
- NÃO substitui `internal/deliberate` nem o modo determinístico — REUSA.
- NÃO é um terceiro cérebro — é uma **etapa no caminho existente** (Router → Compiler → LLM → Decoder), nos dois motores via contratos já unificados.
- NÃO reabre o replay (já corrigido).

---

## 4. Fases de implementação (incremental, cada uma com gate de testes)

| Fase | O que entrega | Gate |
|---|---|---|
| **F1 — Action Decoder** (resposta estruturada) | Novo contrato de resposta + fallback prosa; validar que os caminhos existentes continuam funcionando | `go test ./internal/orchestration/...` + benchmark de custo |
| **F2 — Context Compiler** (estado operacional) | Template TASK/STATE/CONSTRAINTS/FACTS/EVIDENCE em cima do `BuildCleanContext` | recall baseline não regride + testes de formato |
| **F3 — Context Budget** | Alocação de tokens por seção, realimentando `cost` | medição: UsefulWork/tokens sobe |
| **F4 — Context Router + progressivo L0→L2** | Seleção por score + escada de camadas com confiança | E2E: custo por decisão cai; qualidade não regride |
| **F5 — Medição** | Dashboard de custo por decisão (já existem métricas) | observabilidade |

---

## 5. Critérios de aceite

1. Um pedido que hoje custa N tokens no LLM passa a custar **< N com qualidade igual ou melhor** (medido por `cosca cost`).
2. O LLM devolve instruction packet JSON válido na maioria dos casos; quando devolve prosa, o sistema NÃO quebra (fail-open).
3. O modo determinístico (responder sem LLM) passa a ser acionado **por confiança** (deliberate), não só por heurística de snippet.
4. Nenhum teste existente regride (a mudança é aditiva sobre o caminho atual).
5. O contexto entregue ao LLM **nunca** contém informação sem proveniência (epistemologia presente ou ausente de forma explícita).

---

## 6. Prova de que as peças existem (para não reinventar)

- `go test ./internal/search/...` + `./internal/ranking/...` → retrieval por relevância ✅
- `go test ./internal/deliberate/...` → convergência/confiança determinística ✅
- `go test ./internal/orchestration/ -run Deliberation` → BuildCleanContext estruturado ✅
- `go test ./internal/mcpserver/ -run Replay` → proteção de replay ✅ (2026-09-01)
- `cosca cost` → UsefulWork/Efficiency por task ✅ (ADR-031)

O Context Compiler **não parte do zero**: parte do `BuildCleanContext` (estado operacional), do `deliberate` (confiança), do `search/ranking` (relevância), do `cost` (budget) e do `trace` (identidade) — todos canônicos, unificados e testados. Falta o **orquestrador**.
