# ADR-032: Kernel-First Deliberation — o Kernel pensa primeiro; a LLM é especialista quando o Kernel reconhece os próprios limites

> **Status:** Proposed (aguardando cosca-cto + cosca-architecture + Don) | **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-08-31
> **Revisão:** decisão de DESIGN — NÃO implementada. Define o norte e o plano; a implementação é incremental em fases, cada uma com gate de testes.
> **Fonte (ordem do Don, 2026-08-31):** **"o Kernel pensa primeiro; a LLM é chamada como especialista quando o Kernel reconhece os próprios limites."** O professor: `INPUT → PERCEPÇÃO/EMBED → EVIDÊNCIAS → 🧠 DELIBERAÇÃO (determinística) → confiança suficiente? RESPONDE SEM LLM | incerto? chama LLM com contexto LIMPO`.
> **Relação:** CONECTA o **ADR-011** (deliberação determinística zero-LLM, `internal/deliberate`, hoje ISOLADO) ao **`Engine.Execute`** (`internal/orchestration`). É a ponte que o ADR-011 previu (`DecisionDeliberator` port + `DeliberateConfig`) mas **nunca foi implementada**. Também eleva o caminho determinístico já existente no Executor (`deterministicResponse`, executor.go:1229) de uma heurística de snippet para uma **decisão aritmética evidência-gated**.

---

## 0. Mapa honesto do estado atual (o que JÁ EXISTE — crítica antes do gap)

> **Nota de veracidade (auditada em código):** o COSCA **já tem** a camada determinística (`internal/deliberate`) e **já tem** um caminho "sem LLM" no Executor. A proposta do Don NÃO é do zero — a fundação existe. O que falta é **conectar** as duas e **elevar** o gate de "responder sem LLM" de heurística para decisão aritmética.

### 0.1 `internal/deliberate` — a camada determinística EXISTE e está ISOLADA

| Peça | Onde | Estado |
|---|---|---|
| `ComputeConvergence(positions, weights) (float64, bool)` | `convergence.go:72` | ✅ Σ(peso×concordância), threshold 0.70, zero-LLM |
| `ComputeConfidence(base, adjustments) ConfidenceBreakdown` | `confidence.go:79` | ✅ breakdown aritmético auditável (L0-L5 + M1-M7) |
| `EvaluateEmit(breakdown) Emit` | `confidence.go:113` | ✅ ≥0.70 EmitOK / 0.50-0.69 EmitWithReservations / <0.50 Escalate |
| Evidence-gating ("zero achismo") | `evidence.go`, `types.go:56` | ✅ Position só Substantiated com EvidenceIDs; sem evidência é desponderada |
| `ValidateSpec(spec) error` | `spec.go:24` | ✅ SPEC acionável (FINDINGS/CROSS-AGENT/SPEC/ACTIONS/CONFIDENCE) |
| `VoteCross(reviews, owner)` | `pool.go:29` | ✅ rejeita self-vote |
| `DetectLoop(hashes)`, `RoundHash(positions)` | `convergence.go:151,166` | ✅ circuit breaker anti-loop |
| `DefaultConvergenceWeights()` | `types.go:128` | ✅ A3 defaults (0.30/0.25/0.25/0.20) |
| Testes | `deliberate_test.go` | ✅ `go test ./internal/deliberate/...` verde, sem LLM |

**Conclusão:** o `deliberate` está **completo e testado**, mas **não é consumido por nenhum pacote** (grep por `Deliberate|DecisionDeliberator|Deliberation` em `internal/orchestration` → zero ocorrências reais). É a "engrenagem pronta, sem eixo".

### 0.2 O Executor JÁ tem um caminho determinístico — mas é heurístico

| Peça | Onde | Estado |
|---|---|---|
| `deterministicResponse(data PipelineData) string` | `executor.go:1229` | ✅ responde SEM LLM quando o knowledge.db tem um resultado `Score>=0.6` com snippet |
| `pc.WithExecutorDeterministic(true)` | `executor.go:234` | ✅ sinaliza que respondeu sem LLM |
| `PipelineData.ExecutorDeterministic` | `types.go:90` | ✅ flag já existe |

**Conclusão:** o Executor **já decide** "responder sem LLM" — mas por uma **heurística de snippet** (1 resultado com score≥0.6), **sem** convergência multi-dimensão, **sem** breakdown de confiança, **sem** evidência-gating por posição, e **sem** trilha aritmética auditável. A proposta do Don **eleva** isso para a deliberação do `deliberate`.

### 0.3 O ADR-011 previu a ponte — mas ela nunca foi construída

O ADR-011 §3.1 especificou:
- Port `type DecisionDeliberator interface { Deliberate(ctx, pc PipelineContext) (deliberate.Decision, error) }` em `ports.go`.
- `OrchestratorConfig.DeliberateConfig { Enabled bool; P0P1Only bool; Weights; EmitThreshold }`.
- `Engine.Deliberate(ctx, req)` rodando **depois do Router, antes do Executor**.

**Nada disso existe no código hoje.** Este ADR **retoma** essa especificação e a **adapta** ao objetivo do Don (Kernel-first em TODO request, não só P0/P1, com gate de confiança determinístico).

---

## 1. Contexto / Problema

Hoje, no `Engine.Execute` (orchestrator.go:223), o fluxo é:

```
MAG → ContextBuilder → Router → Executor(LLM) → Pipeline
```

O Executor chama a LLM **diretamente** (executor.go:310-318). A única exceção é a heurística `deterministicResponse` (snippet único). Ou seja:

- **O Kernel não delibera antes de chamar a LLM.** Ele delega tudo ao modelo, mesmo quando já tem evidência suficiente na casa (knowledge.db, memória, codegraph).
- **Quando chama a LLM, joga o contexto INTEIRO** (`buildSystemPrompt` injeta todo o knowledge/memory) — o que estoura o contexto do modelo local (o problema que o Don apontou).
- **Não há trilha de "por que o Kernel decidiu"** — não se sabe se respondeu por confiança alta ou por falta de alternativa.

**Problema em uma frase:** o Kernel não reconhece os próprios limites — nem quando tem certeza (responde sem LLM) nem quando não tem (chama a LLM com contexto limpo). Ele sempre delega, sempre com contexto inteiro, sem trilha.

---

## 2. Decisão — Kernel-First Deliberation como etapa determinística ANTES do Executor

**Adotar** uma etapa de **deliberação determinística** (`internal/deliberate`) entre o **Router** e o **Executor** no `Engine.Execute`, seguindo a arquitetura do professor:

```
INPUT → PERCEPÇÃO/EMBED → EVIDÊNCIAS → 🧠 DELIBERAÇÃO (determinística)
   ├─ convergence + confidence + evidence-gating
   ├─ confiança suficiente? → RESPONDE SEM LLM (determinístico)
   └─ incerto ("não sei")? → chama LLM com contexto LIMPO (positions + evidências + hipóteses)
```

O **Kernel pensa primeiro** (P9 "a IA propõe, o sistema decide"): a aritmética e os gates são **externos à LLM**. A LLM é **especialista** — chamada **apenas** quando o Kernel reconhece que a evidência da casa não converge para uma resposta confiante, e recebe **só o que falta** (contexto limpo), não o contexto inteiro.

### 2.0 Princípios vinculantes (Mandamentos + LEI DO COFRE)

- **Fail-closed (LEI DO COFRE):** se a deliberação falhar ou estiver desligada, o fluxo **cai para o comportamento atual** (chamar a LLM como hoje). A deliberação **nunca bloqueia** uma resposta que hoje funcionaria; ela **adiciona** um caminho determinístico e **enriquece** o contexto da LLM. Nenhuma regressão no caminho legado.
- **Zero-LLM na deliberação:** a etapa de deliberação usa **apenas** `internal/deliberate` (aritmética pura). A LLM só entra no Executor, e só quando o gate decide "incerto".
- **Não tocar `internal/embed`** (cérebro ancestral, P8): a deliberação **consome** evidências (knowledge/memory/codegraph) via ports já existentes; não reescreve o cérebro.
- **Reuso, não duplicação:** `deliberate` (gates), `knowledge`/`memory` (evidências), `ranking` (scores), `confidence` (tracker de agente). Nada novo do zero.
- **Trilha verificável (P3):** toda decisão do Kernel (responder sem LLM / chamar LLM, e por quê) é registrada com a breakdown aritmética.

---

## 3. Desenho Técnico

### 3.1 Onde encaixa — novo estágio `deliberation` no `Engine.Execute`

No `Engine.Execute` (orchestrator.go), entre o **Router** (etapa 4, linha ~307) e o **Executor** (etapa 5, linha ~310), insere-se a etapa **4.5 Deliberation**:

```
4. Router (agent selection)
4.5 DELIBERATION (novo, determinístico)  ←── Kernel pensa primeiro
    ├─ 4.5.1 Coleta evidências/positions (de pc.Data: KnowledgeResults, MemoryResults, codegraph)
    ├─ 4.5.2 ComputeConvergence + ComputeConfidence + EvaluateEmit
    ├─ 4.5.3 Gate:
    │    ├─ EmitOK (confiança ≥0.70) → RESPONDE SEM LLM (determinístico)
    │    ├─ EmitWithReservations (0.50-0.69) → chama LLM com contexto limpo + mitigação
    │    └─ Escalate (<0.50) → chama LLM com contexto limpo (Kernel reconhece limite)
    └─ 4.5.4 Registra trilha (DeliberationTrace) em pc.Data
5. Executor (LLM) — só se o gate decidiu "incerto"
```

**Interação com o Executor (sem quebrar o fluxo):**
- O Executor **permanece intacto** como o único ponto de chamada à LLM.
- A deliberação **precede** o Executor e **decide**:
  - **EmitOK** → a deliberação **preenche `pc.Data.LLMResponse`** com a resposta determinística (montada das evidências) e marca `pc.WithExecutorDeterministic(true)`. O Executor, ao ver `LLMResponse` já preenchido + `ExecutorDeterministic=true`, **pula a chamada à LLM** (o mesmo mecanismo que `deterministicResponse` já usa hoje — ver executor.go:231-236). **Zero mudança no Executor.**
  - **EmitWithReservations / Escalate** → a deliberação **reescreve `pc.Data.AugmentedPrompt`** com o **contexto limpo** (ver §3.4) e o Executor segue o caminho normal de LLM. **Zero mudança no Executor.**
- **Fail-closed:** se a deliberação estiver desligada (`DeliberateConfig.Enabled=false`) ou falhar, o `pc` passa **inalterado** para o Executor → comportamento atual preservado.

> **Por que NÃO mudar o Executor:** o Executor já tem o mecanismo de "resposta determinística sem LLM" (executor.go:231). A deliberação **reusa** esse mecanismo em vez de duplicá-lo. Isso mantém a mudança **aditiva** e **reversível** — o Executor não sabe nem precisa saber que existe deliberação.

### 3.2 Como o Kernel obtém evidências/positions (de onde vêm)

A deliberação **não busca nada novo** — ela **consome** o que o pipeline já coletou (reuso, não duplicação). As evidências vêm de `pc.Data` (PipelineData), já populadas pelas etapas anteriores:

| Fonte | Campo em `pc.Data` | Vira | Como vira Position |
|---|---|---|---|
| **Knowledge search** (knowledge.db) | `KnowledgeResults.Results[]` | `Position` (DimensionRecommendation/Premises) | Cada resultado com `Score>=MinScore` vira uma Position com `EvidenceIDs=[result.ID]`, `Substantiated=true` |
| **Memory** (MAG + ContextBuilder) | `MemoryResults[]`, `RetrievedMemories[]` | `Position` (DimensionPremises) | Cada record vira Position com `EvidenceIDs=[record.ID]`, `Substantiated=true` |
| **Codegraph / codeindex** (ADR-019) | via `KnowledgeSearcher.Search` com `Types=["code_block","symbol"]` | `Position` (DimensionPremises) | Resultados de código viram Positions com `EvidenceIDs=[codegraph node ID]` |
| **Ranking** (`internal/ranking`) | `Ranker.Explain` sobre os results | `Adjustment` (corroboração) | `CorroborationAdjustment(sourceCount)` quando ≥3 fontes independentes |
| **Confidence tracker** (`internal/confidence`) | `Tracker` por agente | `Adjustment` (critic) | `CriticAdjustment(criticScore)` quando o agente resolvido tem confiança <0.70 |
| **Contradições** | resultados com scores conflitantes | `Adjustment` (contradição) | `UnresolvedContradictionAdjustment()` quando duas positions se contradizem |

**Mapeamento para `deliberate.Position`** (via `NewPosition`):
```go
pos := deliberate.NewPosition(
    id,            // "ev:" + result.ID
    result.Snippet, // claim (a evidência textual)
    owner,          // o agente resolvido pelo Router
    dimension,      // DimensionRecommendation | DimensionPremises | ...
    []string{result.ID}, // EvidenceIDs rastreáveis
)
```

**Regra "zero achismo":** uma Position **só** é criada com `EvidenceIDs` não-vazio. Se não há evidência, **não há Position** — e a deliberação conclui "incerto" (chama a LLM). Isso é exatamente o reconhecimento de limite do Kernel.

### 3.3 O gate de confiança (suficiente vs incerto)

O gate usa as três funções do `deliberate` em cascata:

```go
// 1. Convergência multi-dimensão
convergence, converged := deliberate.ComputeConvergence(positions, weights)

// 2. Confiança aritmética com breakdown
breakdown := deliberate.ComputeConfidence(convergence, adjustments)

// 3. Decisão de emissão (o sistema decide, P9)
verdict := deliberate.EvaluateEmit(breakdown)
```

| Verdict | Condição | Ação do Kernel |
|---|---|---|
| **EmitOK** | `breakdown.Final >= 0.70` | **RESPONDE SEM LLM** — monta resposta determinística das evidências, preenche `LLMResponse`, marca `ExecutorDeterministic=true` |
| **EmitWithReservations** | `0.50 <= Final < 0.70` | **chama LLM** com contexto limpo + **mitigação** (anexa o plano de reservas ao prompt) |
| **Escalate** | `Final < 0.50` | **chama LLM** com contexto limpo — o Kernel **reconhece o limite** e delega ao especialista |

**Anti-loop (A4):** `Escalate` **não re-roda** a deliberação. Uma vez "incerto", vai para a LLM. `DetectLoop`/`RoundHash` protegem contra convergência cíclica se a deliberação for iterada (reservado para fases futuras).

**Thresholds configuráveis:** `EmitThreshold` (default 0.70) e `ReservationThreshold` (default 0.50) vêm de `DeliberateConfig` (ver §3.5).

### 3.4 O contrato do contexto LIMPO pra LLM

Quando o gate decide "incerto" (EmitWithReservations/Escalate), a deliberação **reescreve `pc.Data.AugmentedPrompt`** com um **contexto limpo e estruturado** — NÃO o contexto inteiro. Isso resolve o problema do contexto que estoura no modelo local.

O contexto limpo é um bloco determinístico com **apenas o essencial**:

```
── DELIBERAÇÃO DO KERNEL (determinístico, zero-LLM) ──
Kernel verdict: escalate (confiança 0.42 < 0.50)

POSITIONS (evidências da casa):
  [P1] recommendation: "adotar X" — evidência: ev:abc123 (score 0.82)
  [P2] premises:       "X requer Y" — evidência: ev:def456 (score 0.74)
  [P3] risks:          "Y é instável" — evidência: ev:ghi789 (score 0.61)

EVIDÊNCIAS (traceáveis, top-N):
  ev:abc123 — knowledge.db — "adotar X porque..." (score 0.82)
  ev:def456 — memory — "X requer Y" (score 0.74)

HIPÓTESES (scores):
  H1: "adotar X" — convergência 0.42 — baixa
  H2: "rejeitar X" — convergência 0.31 — baixa

CONTRADIÇÕES (não resolvidas):
  P2 vs P3: "X requer Y" contradiz "Y é instável"

PERGUNTA AO ESPECIALISTA:
  O Kernel não converge (confiança 0.42). Resolva a contradição P2↔P3
  e recomende entre H1/H2, citando as evidências acima.
── FIM DA DELIBERAÇÃO ──
```

**Regras do contexto limpo:**
- **Só o que falta:** positions + evidências + hipóteses com scores + contradições. **Nunca** o contexto inteiro (knowledge/memory bruto).
- **Top-N limitado:** evidências truncadas a `MaxEvidence` (default 5) e `MaxCharsPerEvidence` (default 300) — o mesmo truncamento que `deterministicResponse` já usa.
- **Estrutura determinística:** o bloco é montado por uma função pura (`BuildCleanContext`), testável sem LLM.
- **A LLM recebe o problema, não o dump:** a pergunta ao especialista é explícita ("resolva a contradição, recomende entre hipóteses"), o que reduz o trabalho do modelo e o contexto.

### 3.5 Configuração (feature flag + thresholds)

Novo campo em `OrchestratorConfig` (orchestrator.go:16), **opcional e zero-flag por padrão** (fail-closed):

```go
// DeliberateConfig configura a etapa Kernel-First Deliberation.
// Quando Enabled=false (default), o fluxo é EXATAMENTE o atual (sem deliberação).
type DeliberateConfig struct {
    Enabled bool // feature flag; default false (fail-closed)

    // Thresholds (defaults do deliberate: 0.70 / 0.50)
    EmitThreshold         float64 // >= este → responde sem LLM
    ReservationThreshold  float64 // >= este e < EmitThreshold → LLM + mitigação

    // Evidências
    MaxEvidence           int     // top-N evidências no contexto limpo (default 5)
    MaxCharsPerEvidence   int     // truncamento por evidência (default 300)
    MinScore              float64 // score mínimo para virar Position (default 0.50)

    // Peso das dimensões (defaults A3)
    Weights deliberate.ConvergenceWeights
}
```

**Onde é lida:** `Engine.Execute` checa `e.config.DeliberateConfig.Enabled` antes da etapa 4.5. Se `false` ou `nil`, pula direto para o Executor (comportamento atual).

**Como ativar:** via `DefaultOrchestratorConfig()` (default desligado) ou explicitamente no wiring (`factory.go`/`NewEngine`). A ativação é **por request/ambiente**, não global obrigatória — respeita o modo `SIMPLE` do ADR-011 (baixo risco pula deliberação).

### 3.6 Trilha verificável (auditoria)

A deliberação registra uma **trilha aritmética** em `pc.Data` (novo campo `DeliberationTrace`), auditável e determinística:

```go
type DeliberationTrace struct {
    Verdict      deliberate.Emit
    Convergence  float64
    Confidence   deliberate.ConfidenceBreakdown // breakdown auditável (base + ajustes + final)
    Positions    []deliberate.Position
    Adjustments  []deliberate.Adjustment
    EvidenceIDs  []string
    Deterministic bool // true = respondeu sem LLM
    Timestamp    time.Time
}
```

- **Onde vive:** `PipelineData.DeliberationTrace` (types.go) + serializado em `Extra["deliberation"]` para compatibilidade com o pipeline (pipeline.go:759 já serializa `Extra`).
- **O que responde:** "por que o Kernel decidiu?" — o `ConfidenceBreakdown.Breakdown` string já é a trilha aritmética (`base=0.42 critic_below_threshold=-0.20 final=0.22`).
- **Para auditoria:** o trace é logado (zerolog) e pode ser persistido no `execution_store.go` / memória (fase futura). Satisfaz P3 ("nenhum agente age sem rastro").

### 3.7 Métricas

Adicionar ao `OrchestrationMetrics` (metrics.go):
- `deliberation_verdict` — contador por verdict (emit_ok / emit_with_reservations / escalate).
- `deliberation_deterministic` — contador de respostas sem LLM (vs. com LLM).
- `deliberation_skipped` — quando desligado/falhou (fail-closed).
- `deliberation_confidence` — histograma do `Final` (calibração de thresholds).

---

## 4. Implementação — fases e ordem (Mandamento III: incremental)

### Fase 0 — Port + Config + wiring vazio (barato, não toca o fluxo)

- Adicionar `DeliberateConfig` a `OrchestratorConfig` (default `Enabled=false`).
- Adicionar `PipelineData.DeliberationTrace` (types.go).
- Adicionar a etapa 4.5 em `Engine.Execute` **guardada por `Enabled`** — quando false, no-op (fail-closed).
- **Gate:** `go test ./internal/orchestration/...` verde; comportamento atual 100% preservado.

### Fase 1 — Deliberador determinístico (núcleo)

- Novo `internal/orchestration/deliberation.go`:
  - `Deliberator` struct (constrói positions a partir de `pc.Data`).
  - `Deliberator.Deliberate(ctx, pc) (DeliberationTrace, error)` — usa `deliberate.ComputeConvergence` + `ComputeConfidence` + `EvaluateEmit`.
  - `BuildCleanContext(pc, trace, cfg) string` — monta o contexto limpo (§3.4).
  - `BuildDeterministicResponse(pc, trace) string` — monta a resposta sem LLM (reusa o formato de `deterministicResponse`).
- Wire no `Engine.Execute` (etapa 4.5): EmitOK → preenche `LLMResponse` + `ExecutorDeterministic`; senão → reescreve `AugmentedPrompt` com contexto limpo.
- **Gate:** `go test ./internal/deliberate/...` + novos testes de `deliberation.go` (sem LLM).

### Fase 2 — Evidências ricas (codegraph + ranking + confidence)

- Estender a coleta de positions para `codegraph`/`codeindex` (via `KnowledgeSearcher` com `Types=["code_block","symbol"]`).
- Aplicar `CorroborationAdjustment` (via `ranking.Explain`) e `CriticAdjustment` (via `confidence.Tracker`).
- **Gate:** testes de integração com mocks de knowledge/memory/codegraph.

### Fase 3 — Trilha persistida + métricas + calibração

- Persistir `DeliberationTrace` no `execution_store.go` / memória.
- Adicionar métricas (§3.7).
- Calibrar thresholds com dados reais (ADR-031: medir antes de otimizar).

> **Nota de rigor:** as Fases 1-3 **não devem** ser implementadas antes da Fase 0 — sem o wiring fail-closed, qualquer bug na deliberação derruba o fluxo. A Fase 0 é o pré-requisito de segurança.

---

## 5. Consequências

**Positivas:**
- **Kernel pensa primeiro:** responde deterministicamente quando tem evidência suficiente — menos chamadas à LLM, menos custo, menos latência (alinhado ao ADR-031 token efficiency).
- **Contexto limpo:** quando chama a LLM, recebe só o essencial (positions + evidências + hipóteses + contradições) — resolve o estouro de contexto do modelo local.
- **Trilha aritmética:** toda decisão do Kernel é auditável (P3), com breakdown de confiança.
- **Reuso total:** `deliberate` (gates), `knowledge`/`memory` (evidências), `ranking` (scores), `confidence` (tracker). Nada duplicado.
- **Fail-closed:** desligado por padrão; nunca bloqueia o fluxo atual.

**Negativas / trade-offs:**
- **Latência de deliberação:** a etapa 4.5 adiciona computação determinística (barata, sem rede) antes do Executor.
- **Risco de falso EmitOK:** responder sem LLM com evidência insuficiente pode dar resposta incompleta. Mitigado por thresholds conservadores (0.70) e pelo `EmitWithReservations` (0.50-0.69) que ainda chama a LLM.
- **Complexidade de config:** novos campos em `OrchestratorConfig` — mitigado por serem opcionais e zero-flag.
- **Qualidade da resposta determinística:** depende da qualidade das evidências (knowledge.db/codegraph). Se a casa não sabe, o Kernel reconhece e delega — é o comportamento correto.

### O que NÃO muda

- **`internal/embed`** intocado (P8) — o cérebro ancestral não é reescrito.
- **O Executor** — permanece o único ponto de chamada à LLM; a deliberação reusa o mecanismo `ExecutorDeterministic` já existente.
- **O caminho legado** — com `Enabled=false`, o fluxo é exatamente o atual.
- **A cadeia de comando e a guarda do Don** — a deliberação **nunca autoriza**; apenas recomenda emitir/escalar (P9).

---

## 6. Testes (deliberação determinística sem LLM)

A deliberação é **100% testável** com `go test`, sem LLM — o mesmo princípio do `deliberate`:

1. **Gate de confiança** (unit):
   - Evidências suficientes (convergência ≥0.70) → `EmitOK` → responde sem LLM.
   - Evidências parciais (0.50-0.69) → `EmitWithReservations` → chama LLM com mitigação.
   - Sem evidências (convergência <0.50) → `Escalate` → chama LLM com contexto limpo.
2. **Zero achismo** (unit): position sem `EvidenceIDs` é desponderada; não vira Position.
3. **Contexto limpo** (unit): `BuildCleanContext` produz o bloco estruturado com top-N, truncamento, contradições — sem vazar contexto inteiro.
4. **Fail-closed** (unit): `Enabled=false` → `pc` inalterado → Executor chamado como hoje.
5. **Trilha** (unit): `DeliberationTrace` registra verdict, convergence, breakdown, evidenceIDs.
6. **Integração** (mock): `Engine.Execute` com mocks de knowledge/memory → EmitOK responde sem chamar o provider (mock de `ChatProvider` que falha se chamado).
7. **Regressão** (existente): `go test ./internal/orchestration/...` e `./internal/deliberate/...` verdes.

---

## 7. Alternativas consideradas

| Alternativa | Veredito |
|---|---|
| **A. Não fazer nada** (manter o Executor chamando a LLM sempre) | ❌ Mantém o Kernel sem reconhecer limites — delega tudo, contexto inteiro, sem trilha. É exatamente o problema do Don. |
| **B. Só melhorar `deterministicResponse`** (mais heurísticas de snippet) | ⚠️ Melhora o "responder sem LLM" mas não o "contexto limpo" nem a trilha; heurística não é decisão aritmética. |
| **C. Kernel-First Deliberation (ESTE ADR)** | ✅ Ataca os três: responde sem LLM quando confiante, contexto limpo quando incerto, trilha aritmética sempre. Reusa `deliberate` + mecanismo existente. |
| **D. Reescrever o Executor para deliberar inline** | ❌ Duplica `deliberate` e quebra o Executor; a deliberação deve ser uma etapa separada, aditiva e reversível. |

---

## 8. Verificação (como saber que funciona)

1. **Fase 0:** `go test ./internal/orchestration/...` verde; `Enabled=false` preserva o comportamento atual.
2. **Fase 1:** com `Enabled=true` e evidências suficientes, o provider de LLM **não é chamado** (mock que falha se chamado); com evidências insuficientes, o `AugmentedPrompt` contém o contexto limpo (não o dump inteiro).
3. **Fase 2:** positions de codegraph/ranking/confidence aparecem no trace com `EvidenceIDs` rastreáveis.
4. **Fase 3:** métricas `deliberation_verdict`/`deliberation_deterministic` mostram a taxa de respostas sem LLM; thresholds calibrados com dados reais (ADR-031).
5. **Trilha:** `DeliberationTrace` auditável por request — "por que o Kernel decidiu" é respondível.

---

## Referências

- **ADR-011** — deliberação determinística zero-LLM (`internal/deliberate`); a ponte `DecisionDeliberator`/`DeliberateConfig` que este ADR retoma.
- **ADR-019** — code-intelligence graph (codegraph/codeindex como fonte de evidência).
- **ADR-031** — token efficiency (o contexto limpo reduz tokens desperdiçados; a deliberação responde sem LLM quando confiante).
- **`internal/deliberate/*.go`** — a camada determinística pronta e isolada.
- **`internal/orchestration/orchestrator.go`** (etapa 4.5), **`executor.go`** (`deterministicResponse`, executor.go:1229), **`types.go`** (`PipelineData`), **`ports.go`** (ports de knowledge/memory).
- **Ordem do Don + professor (2026-08-31)** — "o Kernel pensa primeiro; a LLM é especialista quando o Kernel reconhece os próprios limites."

---

> **Autor:** Ordem do Don + orientação do professor (2026-08-31) | **Formalizado por:** cosca-architecture | **Revisão pendente:** cosca-cto + Don | **Status:** Proposed — Fase 0 (wiring fail-closed) recomendada primeiro.
