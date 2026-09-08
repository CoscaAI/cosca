# ADR-045: Task-Aware Search — busca orientada ao estado de implementação em curso

> **Status:** IMPLEMENTADO — F1-F4 entregues + F5 benchmarks (2026-09-08) | **Owner:** cosca-kernel | **Last Updated:** 2026-09-08
> **Natureza:** ADR de decisão. Cria o desenho do **Task-Aware Search (TAS)** — uma camada que conecta a busca ao **estado de implementação em curso**, re-ponderando resultados por afinidade com a tarefa e decidindo **quando** aplicar entendimento profundo conforme a fase da implementação.
> **Relação:** complementa **ADR-035** (Context Compiler — o `task_affinity` do §2.1 nunca foi implementado); reusa **ADR-013 §3.2** (modlink scope), **ADR-031** (token efficiency) e o **search/ranking** híbrido existente.

---

## 1. Contexto

O ADR-035 (Context Compiler, F1-F6) entregou: Context Router (L0/L1/L2 por confiança), Context Compiler (TASK/STATE/FACTS/EVIDENCE) e a integração no pipeline. Mas o **`task_affinity`** — mencionado no §2.1 como "novo" — **nunca foi implementado** (verificado por busca no código: zero ocorrências).

O problema em duas frases:

1. **A busca não sabe o que está sendo implementado.** Ela recebe a query crua e busca por domínio semântico (modlink scope). Se o Don está implementando um endpoint REST em Go, a busca deveria priorizar: padrões de API do próprio projeto, arquivos relacionados àquele módulo, exemplos do mesmo stack — **não** um resultado genérico sobre "REST" de qualquer lugar.

2. **A busca não decide QUANDO usar entendimento profundo.** No início de uma tarefa (exploração) o sistema precisa de recall amplo e rápido. No meio (implementação) precisa de entendimento profundo do padrão do projeto. No fim (verificação) precisa de precisão cirúrgica. Hoje busca sempre igual.

**Direção do Don (2026-09-08):** "criar algo que melhore ainda mais a busca entendimento e quando usar esse entendimento conforme o que está sendo usado durante a implementação."

---

## 2. Decisão

Criar o **Task-Aware Search (TAS)** — uma camada que:

1. Deriva um **perfil de tarefa** do estado de implementação em curso (stack, alvo, fase, afinidade).
2. **Re-pondera** os resultados de busca por afinidade com o perfil (não só por domínio semântico).
3. Detecta a **fase** da implementação (exploração → implementação → verificação) e ajusta o modo de busca (recall amplo vs. entendimento profundo vs. precisão cirúrgica).

### 2.1 Componentes

```
                    COSCA
                      │
              ┌───────▼────────┐
              │  Task Profile   │  ← NOVO: deriva perfil da tarefa em curso
              │  (taskaffinity) │      (stack, alvo, fase, afinidade)
              └───────┬────────┘
                      │
              ┌───────▼────────┐
              │  Task Phase    │  ← NOVO: detecta fase (exploração/implementação/
              │  (taskphase)   │      verificação) → decide modo de busca
              └───────┬────────┘
                      │
              ┌───────▼────────┐
              │  Search Engine │  ← EXISTENTE: re-ponderado por afinidade
              │  (search)      │      (reusa modlink scope + ranking)
              └───────┬────────┘
                      │
              ┌───────▼────────┐
              │ Context Compiler│  ← EXISTENTE: FACTS/EVIDENCE já orientados
              │  (contextcompile)│     pela tarefa, não só pela query
              └────────────────┘
```

### 2.2 Task Profile (taskaffinity)

Perfil de tarefa derivado do estado de implementação real:

```
TASK:    implementar endpoint REST em Go
STACK:   go + chi + sqlite
TARGET:  internal/api/handlers.go
PHASE:   implementação (meio)
AFFINITY: [api, handlers, rest, chi, sqlite, project-patterns]
```

A busca **re-pondera** os resultados: o que casa com o perfil da tarefa sobe; o que é genérico desce. Reusa o `search.Scope` do modlink (não reinventa roteamento) — **adiciona** a dimensão de afinidade.

### 2.3 Task Phase (taskphase)

Detecta a fase da implementação e ajusta o modo de busca:

| Fase | Modo de busca | Foco |
|---|---|---|
| **Exploração** (início) | recall amplo, entendimento leve | FTS + vetor, mais resultados |
| **Implementação** (meio) | entendimento profundo | vetor + grafo + afinidade; prioriza padrões do projeto e arquivos relacionados |
| **Verificação** (fim) | precisão cirúrgica | scope + epistemic + afinidade; confina ao alvo |

### 2.4 Integração no pipeline

O `contextpipeline` (F6) passa a alimentar a busca com o **perfil de tarefa** antes de compilar o contexto. A busca retorna FACTS/EVIDENCE **já orientados pela tarefa**, não só pela query.

---

## 3. Consequências

### Positivas
- **Entendimento orientado à tarefa**: a busca prioriza o que é relevante para o que está sendo implementado, não só para a query.
- **Custo adaptativo por fase**: exploração é barata (recall), implementação é profunda (entendimento), verificação é cirúrgica (precisão) — sem desperdício.
- **Reusa o existente**: modlink scope, ranking, context compiler — não reinventa.
- **Medível**: recall/precisão/custo (ADR-031) verificáveis.

### Negativas / Riscos
- **Perfil mal derivado** pode enviesar a busca (afinidade errada piora em vez de melhorar) → validação por recall/precisão nos testes.
- **Fase mal detectada** pode mudar o comportamento indevidamente → regras determinísticas conservadoras.
- **Escopo**: NÃO é um passo da Etapa 3 — é uma **fase própria** com gate do Don (já aprovado).

### Não-faz (limites)
- NÃO substitui o search/ranking híbrido — **re-pondera** sobre ele.
- NÃO substitui o modlink scope — **adiciona** a dimensão de afinidade.
- NÃO substitui o Context Compiler — **alimenta** FACTS/EVIDENCE já orientados.
- NÃO reabre o replay (já corrigido) nem o Context Router (já entregue).

---

## 4. Fases de implementação (incremental, cada uma com gate de testes)

| Fase | O que entrega | Gate |
|---|---|---|
| **F1 — taskaffinity: perfil** | Deriva o perfil de tarefa (stack, alvo, fase, afinidade) | testes de derivação; recall não regride |
| **F2 — taskaffinity: re-ponderação** | Re-pondera os resultados de busca por afinidade | recall/precisão sobem; custo não sobe |
| **F3 — taskphase: detecção de fase** | Detecta a fase (exploração/implementação/verificação) | regras determinísticas testadas |
| **F4 — Integração no pipeline** | Alimenta a busca com o perfil antes de compilar | E2E: FACTS/EVIDENCE orientados à tarefa |
| **F5 — Medição** | Recall/precisão/custo por fase | observabilidade (ADR-031) |

---

## 5. Critérios de aceite

1. Busca de uma tarefa de implementação retorna **mais resultados do projeto** (padrões/arquivos relacionados) e menos ruído genérico.
2. Custo por decisão **não sobe** (ADR-031 mede).
3. Recall/precisão **não regride** nos testes existentes.
4. A busca **muda de comportamento por fase** (verificável).

---

## 6. Roteamento

| Componente | Capo | Por quê |
|---|---|---|
| **taskaffinity** (perfil + re-ponderação) | `cosca-architecture` + `cosca-ai` | Design do perfil + integração com search/ranking |
| **taskphase** (detecção de fase) | `cosca-architecture` | Regras determinísticas de fase |
| **Integração pipeline** | `cosca-backend` | Conectar ao contextpipeline/executor |
| **Testes + benchmark** | `cosca-qa` + `cosca-performance` | Recall/precisão não regride; custo não sobe |

---

## 7. Prova de que as peças existem (para não reinventar)

- `go test ./internal/search/...` + `./internal/ranking/...` → retrieval por relevância ✅
- `go test ./internal/contextrouter/...` → L0/L1/L2 por confiança ✅
- `go test ./internal/contextcompile/...` → estado operacional compilado ✅
- `go test ./internal/contextpipeline/...` → integração F6 ✅
- `go test ./internal/modlink/...` → roteamento por domínio (scope) ✅

O Task-Aware Search **não parte do zero**: parte do `search/ranking` (relevância), do `modlink` (scope), do `contextcompile` (estado operacional) e do `contextpipeline` (integração) — todos canônicos, unificados e testados. Falta a **camada de afinidade e fase**.

---

## 8. Status de implementação (2026-09-08)

> **Fase:** F1-F4 entregues + F5 benchmarks. Design técnico: `docs/design/DESIGN-001-task-aware-search.md`.

### Entregue

| Fase | Pacote/Arquivo | Status |
|---|---|---|
| **F1 — taskaffinity: perfil** | `internal/taskaffinity/taskaffinity.go` | ✅ 6 testes PASS |
| **F2 — taskaffinity: re-ponderação** | `internal/taskaffinity/rerank.go` | ✅ 6 testes PASS |
| **F3 — taskphase: detecção de fase** | `internal/taskphase/taskphase.go` | ✅ 12 testes PASS |
| **F4 — Integração no pipeline** | `internal/taskaffinity/buildcontext.go`, `internal/orchestration/types.go`, `internal/contextpipeline`, `internal/contextcompile` | ✅ 7 testes PASS |
| **F5 — Benchmarks** | `internal/taskaffinity/bench_test.go` | ✅ critério atingido |

### Performance (F5 — overhead total < 2ms)

| Benchmark | Resultado | Critério | Status |
|---|---|---|---|
| `DeriveProfile` | 3.0 μs | < 1ms | ✅ |
| `AffinityRerank` (30 resultados) | 86 μs | < 500μs | ✅ |
| `DetectPhase` | 7.6 ns | < 100μs | ✅ |
| `TAS_Overhead` (total) | **65.8 μs** | < 2ms | ✅ 30x abaixo |

### Correções durante a implementação

1. **BUG do `EnrichWithProfile`**: modificar uma seção do `CompiledContext` sem remontar o texto (`renderText`) deixava o contexto entregue ao LLM desatualizado — a seção mudava mas o texto não. Corrigido adicionando `renderText()` ao final do `EnrichWithProfile`.
2. **ERRO do DESIGN-001 §3.3**: o exemplo numérico mostrava projeto 0.757 "superando" genérico 0.836 (matematicamente falso, 0.757 < 0.836). O boost de afinidade (teto 0.15) **consolida** e resolve empates, mas **não reordena** resultados com grande diferença de score inicial. O DESIGN-001 §3.3 deve ser corrigido para refletir isso.

### Pendências (evolução futura)

- **Medição no contextmetrics**: métricas de TAS (overhead real por fase, boost médio aplicado) ainda não integradas ao `contextmetrics`/`cost` (ADR-031). Recomendado para observabilidade contínua.
- **Correção do DESIGN-001 §3.3**: o exemplo numérico foi corrigido (2026-09-08) com a nota de calibração sobre o boost de 0.15.
- **Injeção de arquivos abertos pelo editor**: o executor só consegue alimentar o perfil com o que está no `PipelineData` (Prompt + metadados via `Request.Context`). Para afinidade plena (com arquivos abertos), o opencode/editor deve injetar `tas.open_files`/`tas.recent_files`/`tas.working_dir`/`tas.go_mod` via `Request.Context`.

### Wiring no executor (2026-09-08)

O TAS foi conectado de ponta a ponta no executor:

1. **Feature flag `EnableTAS`** no `ExecutorConfig` (default false, opt-in, fail-closed). Quando false → comportamento idêntico ao atual (zero regressão).
2. **Wiring no `Execute`**: quando `EnableTAS && pc.Data.TaskContext == nil`, o executor deriva o perfil via `tasTaskContextFrom(data)` (taskaffinity.BuildTaskContext) e injeta no `PipelineData.TaskContext` ANTES do `BuildContext`.
3. **`tasTaskContextFrom`**: constrói o `ImplementaçãoState` a partir do Prompt + metadados do `Request.Context` (via `PipelineData.Extra` sob chaves `tas.*`). Fail-closed: sem prompt e sem sinal de stack/alvo → retorna nil (pipeline segue como antes).
4. **Testes**: `TestTasTaskContextFrom_*` (fail-closed, prompt-only, com metadados) — 4 testes PASS. Build completo OK, orchestration sem regressão.

O overhead do TAS (65.8μs) é desprezível frente à latência real da busca (média 2.46s no benchmark). O wiring não degrada a performance.
