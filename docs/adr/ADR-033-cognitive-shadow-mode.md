# ADR-033: Cognitive Shadow Mode — o cérebro observa o próprio processo antes de ganhar autoridade

> **Status:** IMPLEMENTED (2026-09-01 — verificado em código) | **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-01
> **Revisão:** implementado em `internal/shadow` (JSONL append-only em `.cosca/shadow/records.jsonl`), ativo na config local (`shadow_mode: true`), REST `/v1/shadow/*`, CLI `cosca shadow`. O header anterior "NÃO implementada" estava STALE — o shadow registra deliberação contrafactual sem interferir no caminho principal.
> **Fonte (ordem do Don + professor, 2026-08-31):** Fase 1 da evolução COSCA-centric — o COSCA **pensa, mede e registra**, mas **NÃO interfere na resposta atual**. O request continua para a LLM normalmente. O COSCA apenas observa o próprio processo e registra o que encontrou (percepção), que evidências recuperou (knowledge/memory), qual seria a confiança (deliberação) e se **teria** chamado uma LLM (escalada no Shadow). É a **disciplina da ponte**: o cérebro observa o próprio processo antes de ganhar autoridade sobre ele. **Por quê:** é ouro para descobrir quanto o COSCA já consegue pensar sozinho, SEM risco de responder errado.
> **Relação:** DECORA a deliberação do **ADR-032** (Kernel-First Deliberation, `internal/deliberate` + `Deliberator`). Não cria um sistema paralelo de deliberação — reusa o `Deliberator.Deliberate` e apenas **muda o que se faz com o verdict** (o Shadow NÃO aplica a autoridade; o modo ativo aplica). Também conecta às peças órfãs da porta cognitiva: percepção vision e MCP cognitivo (fase futura).

---

## 0. Mapa honesto do estado atual (o que JÁ EXISTE — crítica antes do gap)

> **Nota de veracidade (auditada em código):** o ADR-032 **já está implementado**. O `Engine.Execute` tem a etapa **4.5 Deliberation** (orchestrator.go:327-366), o `Deliberator` existe (`internal/orchestration/deliberation.go`), e o `deliberate` (gates aritméticos) é puro e testado. O que **falta** é o modo **observacional**: hoje a deliberação, quando ligada, é **autoritativa** (muda a resposta). Não existe nenhum registro de "o que o Kernel **teria** decidido se NÃO tivesse autoridade". Esse é o gap do Shadow.

### 0.1 O ADR-032 já está implementado (autoritativo)

| Peça | Onde | Estado |
|---|---|---|
| `DeliberateConfig.Enabled` (feature flag, fail-closed) | `orchestration/deliberation.go:22` | ✅ default false |
| Etapa 4.5 no `Engine.Execute` | `orchestrator.go:327-366` | ✅ roda entre Router e Executor |
| `Deliberator.Deliberate(ctx, pc) (DeliberationTrace, error)` — **puro, não muta o pc** | `deliberation.go:209` | ✅ retorna trace, sem efeito colateral |
| `DeliberationTrace{Verdict, Convergence, Confidence, Positions, EvidenceIDs}` | `deliberation.go:117` | ✅ auditável |
| Aplicação do verdict (**a autoridade**): `WithLLMResponse`, `WithDeliberationHandled(true)`, `WithAugmentedPrompt` | `orchestrator.go:348-363` | ✅ é o que faz o modo ativo |
| Executor pula LLM quando `pc.Data.DeliberationHandled` | `executor.go:238` | ✅ aditivo e reversível |
| Config YAML `orchestration.deliberation` + validação | `config.go:408,1129` | ✅ |
| Testes | `deliberation_test.go`, `deliberation_config_test.go` | ✅ verdes, sem LLM |

**Conclusão:** a "engrenagem" do Kernel-First **já está no eixo**. O que falta é um **medidor** que a rode SEM aplicar a autoridade — para descobrir, com dados reais, **quanto o COSCA já decide sozinho**.

### 0.2 Os 3 órfãos da porta cognitiva (contexto do professor)

| Peça órfã | Estado | Relação com Shadow |
|---|---|---|
| **Percepção vision** | adormecida (`internal/vision`, ligada ao `cosca mcp` via `internal/vision`) | Fase futura: alimenta os `Positions` (perceber) |
| **Deliberação ADR-032** | implementada mas **desligada** (`enabled=false` no default) | É o motor que o Shadow mede |
| **MCP cognitivo** | órfão (`cosca mcp` com gate `COSCA_ENABLE_MCP=1`, default off) | Fase futura: portar o Shadow como tool MCP |

**Ordem cognitiva alvo:** perceber → lembrar → recuperar → verificar → inferir → decidir → escalar. O Shadow ataca o **medir o decidir** (deliberação) e, por extensão, o **medir o escalar** (se teria escalado), sem autoridade. Perceber/lembrar/recuperar já existem; verificar/inferir são as próximas fases do mapa.

---

## 1. Contexto / Problema

O ADR-032, quando ligado (`enabled=true`), dá autoridade à deliberação: ela **pode** responder sem LLM (EmitOK) e **pode** reescrever o prompt da LLM (contexto limpo). Isso é o alvo final — mas **ligar isso cedo é arriscado**: se a convergência está mal calibrada, o COSCA responde errado com confiança e o Don só descobre no incidente. E `enabled=false` (default) não mede nada — a deliberação simplesmente não roda, e não se sabe quão perto o COSCA está de pensar sozinho.

**Problema em uma frase:** estamos cegos sobre **quanto o COSCA já conseguiria decidir sozinho** e sobre **a calibração dos thresholds** — porque a deliberação só roda quando tem autoridade, e ter autoridade antes de calibrar é a receita do erro confiante.

**Solução do professor (Shadow Mode):** rodar a deliberação em **modo observacional** — o request segue para a LLM **exatamente como hoje** (o Shadow nunca muda a resposta); o Shadow **apenas registra** qual teria sido a decisão do Kernel (percepção + evidências + confiança + teria escalado?). É ouro para (a) descobrir o quanto o COSCA pensa sozinho, (b) calibrar thresholds com dados reais (ADR-031: medir antes de otimizar), (c) validar o ADR-032 antes de dar autoridade — **observar antes de ganhar autoridade**.

---

## 2. Decisão — Shadow Mode como DECORAÇÃO observacional da deliberação ADR-032

**Adotar** um modo **Shadow** que **reusa** o `Deliberator.Deliberate` do ADR-032 (mesmas funções, mesmos gates, zero-LLM) mas que **NUNCA aplica a autoridade** ao pipeline. O Shadow é um **decorator** da etapa 4.5: roda a deliberação, captura o `DeliberationTrace`, e **descarta** o efeito sobre `LLMResponse` / `DeliberationHandled` / `AugmentedPrompt`. O request continua pelo caminho atual (LLM chamada como hoje). Apenas o **registro** do que o Kernel teria decidido é persistido e reportado.

**Regra de ouro (a diferença crítica):**
- **Modo ativo** (`enabled=true`): a deliberação **governa** a resposta. Se EmitOK → responde SEM LLM (`DeliberationHandled=true`); se incerto → reescreve o prompt com contexto limpo. **Muda o output.**
- **Modo Shadow** (`shadow_mode=true`): a deliberação **observa** a resposta. Roda o MESMO `Deliberate()`, produz o MESMO `DeliberationTrace`, mas **NÃO** seta `LLMResponse`, `DeliberationHandled` nem `AugmentedPrompt`. O output é **produzido pela LLM como hoje**. Apenas o `ShadowTrace` é gravado.

**Reuso, não duplicação:** NÃO se cria um segundo sistema de deliberação. O Shadow chama `Deliberator.Deliberate(ctx, pc)` — exatamente o que o modo ativo chama. A única diferença é **o que se faz com o trace**: o ativo aplica; o Shadow registra.

### 2.0 Princípios vinculantes (Mandamentos + LEI DO COFRE)

- **NÃO-interferência absoluta:** o Shadow **nunca** altera `LLMResponse`, `DeliberationHandled`, `ExecutorDeterministic` nem `AugmentedPrompt`. Em nenhuma hipótese. É a "ponte sem autoridade".
- **Fail-closed (LEI DO COFRE):** se o Shadow falhar, travar por timeout ou lançar panic, ele é **descartado silenciosamente** (log em nível debug) — a resposta principal segue intacta. O Shadow **nunca bloqueia** e **nunca falha** o request.
- **Reuso total:** `deliberate` (gates), `Deliberator` (ADR-032), `knowledge`/`memory` (evidências). Nada novo do zero para deliberar.
- **Zero-LLM no Shadow (e na deliberação):** só aritmética determinística. O Shadow não incrementa custo de LLM nem latência de rede.
- **Não tocar `internal/embed`** (cérebro ancestral, P8) — o Shadow consome evidências via ports existentes, não reescreve o cérebro.
- **Independente do modo ativo:** Shadow roda MESMO com `enabled=false`. É o Caso inteligente (medir sem autoridade). Pode rodar junto com o ativo (validação cruzada) — redundante mas inofensivo.
- **Trilha verificável (P3):** o `ShadowTrace` é determinístico, serializável e auditável — "qual teria sido a decisão do Kernel e por quê" é respondível.

---

## 3. Desenho Técnico

### 3.1 Shadow vs modo ativo — a tabela que separa autoridade de observação

| Aspecto | Modo Ativo (`enabled=true`) | Modo Shadow (`shadow_mode=true`) |
|---|---|---|
| **Roda a deliberação** (`Deliberator.Deliberate`) | ✅ sim | ✅ sim (mesma função) |
| **Verdict EmitOK → resposta SEM LLM** | 🟢 **SIM** (`WithLLMResponse` + `WithDeliberationHandled(true)` → executor pula) | 🔴 **NÃO** — executor continua chamando a LLM |
| **Verdict incerto → contexto limpo** | 🟢 reescreve `AugmentedPrompt` | 🔴 NÃO — prompt original intacto |
| **Muda a resposta do executor?** | 🟢 sim (autoridade) | 🔴 não (observação pura) |
| **Escreve `pc.Data.DeliberationTrace`** | 🟢 sim (campo de auditoria do pipeline) | 🟡 sim, como campo passivo de auditoria (não afeta o executor) |
| **Escreve `ShadowTrace` no store** | 🔴 não (não é o propósito) | 🟢 sim (o propósito) |
| **Usado para** | resolver sem LLM / contexto limpo | medir + calibrar + validar antes de dar autoridade |
| **Custos** | menos chamadas LLM, menos latência | nenhuma mudança no custo/latência real |

**A regra de um linha:** o modo ativo **decide** e **aplica**; o modo Shadow **decide** e **registra**. O "decide" (o `Deliberate()`) é **idêntico**; o que muda é o **o que se faz com o verdict**.

### 3.2 Onde encaixa — o Shadow decora a etapa 4.5, NÃO cria uma etapa nova

No `Engine.Execute` (orchestrator.go:327), a etapa 4.5 **já existe** e é gated por `e.config.DeliberateConfig.Enabled`. O Shadow **não** adiciona uma etapa nova — ele **decora** a existente, com um **branch por modo**:

```
4.5 DELIBERATION (ADR-032)
    ├─ Gate: deliberator criado quando (Enabled || ShadowMode)   ← ALTERADO
    │
    ├─ caso SHADOW (shadow_mode=true) ─────────────────────────────
    │   trace, err := deliberator.Deliberate(ctx, pc)   // mesma função
    │   err? → descarta silenciosamente (debug) e segue p/ Executor
    │   OK   → shadowStore.Append(toShadowTrace(trace, pc))  // registra
    │           loga o bloco "COSCA Cognitive Shadow Mode"
    │           pc = pc.WithDeliberationTrace(&trace)  // passivo, não afeta executor
    │           continua → Executor (NUNCA aplica o verdict)   ← o ponto
    │
    ├─ caso ATIVO (enabled=true, não shadow) ──────────────────────
    │   (comportamento atual: aplica o verdict — EmitOK responde sem
    │    LLM; incerto reescreve AugmentedPrompt)
    │
    └─ caso nenhum (default) → no-op (fail-closed, fluxo legado)
```

**Como o Shadow captura sem interferir:** o `Deliberator.Deliberate(ctx, pc)` já é uma função **pura** (deliberation.go:209) — ela retorna o `DeliberationTrace` e **não muta o `pc`** (o `PipelineContext` é imutável por valor, e `collectPositions`/`collectAdjustments` só leem). Portanto, no ramo Shadow, o código **simplesmente não chama** `WithLLMResponse`/`WithDeliberationHandled`/`WithAugmentedPrompt`. O `pc` passa intacto para o Executor.

> **Por que NÃO criar outra etapa:** o Shadow é uma **configuração de comportamento** da etapa existente, não uma etapa nova. Isso mantém (a) um único ponto de deliberação (não risco de divergência de gates), (b) o `Deliberate()` rodando **uma vez** (não duas), (c) o fail-closed do ADR-032 intacto. A mudança no `Engine` é um **branch no switch de aplicação do verdict**, não uma nova chamada.

### 3.3 O que registrar (`ShadowTrace`) e a taxonomia de decisão

O `ShadowTrace` é um subconjunto **orientado à observação** do `DeliberationTrace`, com um rótulo de decisão mais legível e a resposta **contrafactual** (o que o Kernel **teria** dito/feito):

```go
// internal/shadow/trace.go (novo)
type ShadowTrace struct {
    RequestID    string                `json:"request_id"`
    Agent        string                `json:"agent,omitempty"`
    Decision     ShadowDecision        `json:"decision"`
    WouldEscalate bool                 `json:"would_escalate"`   // true = teria chamado a LLM
    WouldRespond string                `json:"would_respond,omitempty"` // resposta EmitOK (só quando DMitOK)
    Confidence   float64               `json:"confidence"`        // breakdown.Final (0..1)
    Convergence  float64               `json:"convergence"`
    EvidenceIDs  []string              `json:"evidence_ids"`
    Positions    int                   `json:"positions"`         // nº de posições substantiadas
    Reason       string                `json:"reason"`            // humanos-legível ("conflicting evidence")
    Breakdown    string                `json:"breakdown,omitempty"` // trilha aritmética (ConfidenceBreakdown.Breakdown)
    DurationMs   int64                 `json:"duration_ms"`
    At           time.Time             `json:"at"`
}
```

**Taxonomia `ShadowDecision`** (mapeia `deliberate.Emit` + o caso "sem evidência"):

| `ShadowDecision` | Condição | `WouldEscalate` | Corresponde a |
|---|---|---|---|
| `RETRIEVAL_INSUFFICIENT` | `len(positions)==0` (nenhuma posição substanciada) | `true` | o Kernel não tem evidência — teria escalado (e é o caso mais comum na pré-calibração) |
| `EMIT_OK` | `verdict == EmitOK` (confiança ≥ threshold) | `false` | teria respondido SEM LLM |
| `EMIT_WITH_RESERVATIONS` | `verdict == EmitWithReservations` | `true` | teria chamado LLM com mitigação |
| `ESCALATE` | `verdict == Escalate` | `true` | teria chamado LLM (Kernel reconhece limite) |

**Regra "zero achismo":** uma posição só é contada se `Position.Effective()` (substanciada + with EvidenceIDs). Se o pipeline coletou 0 posições efetivas, a decisão é `RETRIEVAL_INSUFFICIENT` — o Kernel **no estado actual** não tem o que usar para decidir. Isso mede honestamente "o COSCA não sabe (ainda)".

**Saída-livro (formato do professor) — log estruturado por execução:**

```
COSCA Cognitive Shadow Mode
Kernel decision: RETRIEVAL_INSUFFICIENT
Confidence: 0.42
LLM escalation: YES
Reason: conflicting evidence
```

### 3.4 Onde persistir — `internal/shadow`, JSONL append-only (espelha o store de `cost`)

**Padrão reusado:** o `internal/cost` já persistiu um log append-only em `.cosca/cost/records.jsonl` (runtime, gitignored) e o `cosca cost` o lê e agrega. O Shadow segue **o mesmo padrão**, em um pacote dedicado (padrão, não pacote — o ADR-015 disciplina reuso do *padrão*, não da *implementação* de custo).

```go
// internal/shadow/store.go (novo) — espelha internal/cost
const RecordsDir = "shadow"
const RecordsFile = "records.jsonl"

func ForCoscaDir(dir string) *Store
func (s *Store) Append(r ShadowTrace) error   // append-only, thread-safe, tolera linhas de versões anteriores
func (s *Store) Read() ([]ShadowTrace, error)
```

- **Local:** `.cosca/shadow/records.jsonl` (runtime, **gitignored** — derivado, como `cost/` e `logs/`).
- **Cada linha** = um `ShadowTrace` compacto. A leitura tolera versões anteriores (campos `omitempty`/zero-default).
- **Segurança:** append-only via `os.O_APPEND`; escrita serializada por um mutex no `Store` (o goroutine do Shadow é o único writer, mas a API é thread-safe).
- **Não é o `cost` log nem o `trace` log:** é um log **distinto** porque o Shadow responde uma pergunta **diferente** (contrafactual de deliberação), para um consumidor **diferente** (o Don calibrando a autoridade).

> **Por que não no `DeliberationTrace` do pipeline** (`pc.Data.DeliberationTrace` → `Extra["deliberation"]`): o pipeline já serializa o `DeliberationTrace` em `Extra` (pipeline.go:836). Mas isso é **per-request, em-memory, volátil** — some quando a execução acaba, e não é agregável. O Shadow precisa de **persistência agregável** para o relatório "quanto o COSCA resolve sozinho". O store JSONL resolve os dois: per-request (cada linha é um request) E agregável (todo o arquivo).

### 3.5 Como ativar — feature flag `orchestration.deliberation.shadow_mode` (separada de `enabled`)

Novo campo `ShadowMode bool` em **três** pontos (espelhando `Enabled`):

```yaml
# cosca.yaml
orchestration:
  deliberation:
    enabled: false        # modo ATIVO (autoritativo) — default false (fail-closed)
    shadow_mode: true     # modo SHADOW (observacional)  ← NOVO, independente de enabled
    emit_threshold: 0.70
    reservation_threshold: 0.50
    max_evidence: 5
    max_chars_per_evidence: 300
    min_score: 0.50
```

| Ponto | Mudança |
|---|---|
| `config.DeliberationConfig` (`internal/config/config.go:408`) | + `ShadowMode bool yaml:"shadow_mode" json:"shadowMode"` |
| `orchestration.DeliberateConfig` (`internal/orchestration/deliberation.go:22`) | + `ShadowMode bool` |
| `DeliberateConfigFromConfig` (`deliberation.go:99`) | + `ShadowMode: cfg.ShadowMode` |
| Deputação `orchestration` (`DeliberateConfig` wiring em `chat.go`, `pipeline_wiring.go`, `run.go`, `serve.go`, `runtime.go`, `terminal.go`) | propaga `ShadowMode` junto de `Enabled` |
| Validação (`config.go:1129`) | validar thresholds quando `Enabled \|\| ShadowMode` (o Shadow usa os mesmos thresholds) |

**Semântica do flag** (inédita, a parte importante):
- **`shadow_mode=true` + `enabled=false`** → o **Caso inteligente**: roda a deliberação, registra o Shadow, **NÃO muda a resposta**. É o modo padrão de adoção (medir antes de autoridade).
- **`shadow_mode=true` + `enabled=true`** → coexistência: o Shadow registra E o ativo aplica. Redundante mas inofensivo (a deliberação roda 1x; o trace vai tanto para o pipeline quanto para o store). Útil como validação cruzada shadow==ativo.
- **`shadow_mode=false` + `enabled=true`** → ADR-032 puro (comportamento atual, autoritativo).
- **`shadow_mode=false` + `enabled=false`** → fail-closed (fluxo legado; nada roda).

**A mudança no `NewEngine` (orchestrator.go:196):**
```go
if config.DeliberateConfig.Enabled || config.DeliberateConfig.ShadowMode {
    deliberator = NewDeliberator(config.DeliberateConfig)
}
```
O delibrador é criado em qualquer dos dois modos — o branch no `Execute` decide o que faz com o trace.

### 3.6 Garantia de NÃO-interferência (latência/erro zero na resposta principal)

Esta é a seção mais importante do desenho. O Shadow **nunca** deve afetar a resposta. Quatro garantias:

**G1 — Nunca toca os knobs de autoridade.** O ramo Shadow, por construção, **não chama** `WithLLMResponse`, `WithDeliberationHandled`, `WithExecutorDeterministic` nem `WithAugmentedPrompt`. Só chama `WithDeliberationTrace` (campo passivo que o executor ignora — o executor só reage a `DeliberationHandled`, e ele fica `false`). **Verificado por construção + teste.**

**G2 — Executado em paralelo (fora do caminho crítico) com timeout e descarte.** O `Deliberate()` é determinístico e microsegundos, mas o desenho preserva a receita do professor: o Shadow roda em um **goroutine destacado** com `context.WithTimeout(ctx, ShadowTimeout)` (default `200ms`) e `defer recover()`. Se o timeout disparar, ou a goroutine entrar em panic, ou o `Deliberate()` errar → **descarte silencioso** (log `debug`, contador `shadow_discarded`). O `Engine.Execute` **retorna imediatamente** para o Executor; o goroutine escreve no store em background. **Latência/erro na resposta principal: zero.**

```go
// Shadow sempre fora do caminho crítico: fire-and-forget + bounded + discard.
go func() {
    defer func() { if r := recover(); r != nil { discardShadow("shadow_panic", r) } }()
    sctx, cancel := context.WithTimeout(ctx, shadowTimeout)
    defer cancel()
    trace, err := deliberator.Deliberate(sctx, pcSnapshot) // pcSnapshot = cópia segura
    if err != nil { discardShadow("shadow_error", err); return }
    shadowStore.Append(toShadowTrace(trace, pcSnapshot))
    logShadow(trace) // "COSCA Cognitive Shadow Mode" bloco
}()
```

**G3 — Clone de dados (race-safe).** `PipelineContext` é valor imutável, mas os campos de `pc.Data` apontam para slices/structs compartilhadas (`KnowledgeResults`, `MemoryResults`). O goroutine do Shadow captura **um snapshot** delas via um clone raso determinístico (a `PipelineData.Clone()` já copia slices/maps; para os pointers/slices de evidência, o Shadow apenas **lê**, e nada no pipeline as muta após o Router — documentado e coberto pelo teste de race se `-race`). Se o clone for omitido (otimização), a regra é **ler-sempre, escrever-nunca** nos campos de evidência.

**G4 — Se o Shadow não estiver configurado, nem roda.** `shadow_mode=false` → o branch Shadow não existe; zero goroutine, zero custo. Fail-closed.

**Resumo da garantia:** a resposta do executor é **bit-a-bit idêntica** com Shadow ligado vs desligado, porque o Shadow **nunca toca** o `pc` que o Executor usa, e **roda fora** do caminho crítico. A única diferença observável é o `ShadowTrace` no store + o log.

### 3.7 Como o Don consome — relatório e logs

**1. Log estruturado (por execução, o bloco do professor):** zerolog em modo humano + um bloco `COSCA Cognitive Shadow Mode` no stdout quando `--shadow` (ou `--verbose`). JSON quando `--json`.

**2. `cosca shadow` — grupo CLI (somente leitura, como `cosca cost`)**:
```
cosca shadow                   → resumo agregado
cosca shadow summary           → total, distribuição de decisões, taxa de escalada, confiança média
cosca shadow list --request X  → detalhe de uma execução (o ShadowTrace)
cosca shadow report            → "quanto o COSCA resolve sozinho?" (o relatório do Don)
cosca shadow --json            → saída máquina
```

**O relatório `cosca shadow report`** responde as perguntas do professor:
- **Taxa de auto-resolução:** `% EMIT_OK` (would_escalate=false) — quanto o COSCA **já** responderia sem LLM.
- **Taxa de escalada:** `%` de `RETRIEVAL_INSUFFICIENT` + `ESCALATE` + `EMIT_WITH_RESERVATIONS`.
- **Distribuição de confiança** por decisão (histograma) — calibra `emit_threshold`/`reservation_threshold`.
- **Detecção de casos-ouro:** execuções `EMIT_OK` com `WouldRespond` não-vazio E que a LLM deu resposta diferente → candidatos a "Kernel poderia ter respondido" (o mapa de risco/custo do ADR-032).
- **Confiança × Correção:** (Fase 4) cruza `EMIT_OK` do Shadow com a resposta real da LLM para medir a **calibração** (quando o Shadow disse "sei", a LLM concordou?).

**3. REST** (Fase 2, opcional): `GET /v1/shadow/spectrum` → `ShadowSpectrum` JSON (agregado), análogo ao observatory. Alimentaria o painel/observatório.

**4. MCP cognitivo** (Fase 3, opcional — ao reviver o `cosca mcp`/porta MCP): tool `shadow:observe` e/ou `cognitive:spectrum`. Conecta o Shadow à porta cognitiva que o professor chamou de órfã.

**5. Observatory (`internal/brainweb`)**: adicionar `cognitive: {shadow_selfresolve: float64}` ao `CognitiveSnapshot` (opcional, Fase 4) — o observatório passa a refletir o Shadow.

### 3.8 Métricas

Adicionar ao `OrchestrationMetrics` (metrics.go) — contadores atômicos, zero-LLM:
- `shadow_observations` — nº de Shadows registrados.
- `shadow_discarded` — nº de Shadows descartados (timeout/erro/panic) — a métrica da não-interferência (deve ser ~0).
- `shadow_selfresolve` — nº de `EMIT_OK` (teria respondido sem LLM).
- `shadow_escalate` — nº de `RETRIEVAL_INSUFFICIENT` + `ESCALATE` + `EMIT_WITH_RESERVATIONS`.
- `shadow_confidence_hist` — histograma do `Confidence.Final` (calibração).

---

## 4. Implementação — fases e ordem (Mandamento III: incremental)

### Fase 0 — Config + store + wiring (barato, não toca o fluxo)
- Adicionar `ShadowMode` aos 3 pontos de config (`config.DeliberationConfig`, `orchestration.DeliberateConfig`, `DeliberateConfigFromConfig`) + validação (`Enabled || ShadowMode`).
- Criar `internal/shadow` (store JSONL + `ShadowTrace` + `ForCoscaDir`).
- Usar `cosca shadow --json` com o store vazio → 0 observações.
- **Gate:** `go test ./internal/orchestration/... ./internal/config/... ./internal/shadow/...` verde; comportamento atual 100% preservado.

### Fase 1 — O ramo Shadow no `Engine.Execute` (núcleo da observação)
- Branch na etapa 4.5: `shadow_mode` → roda `Deliberate()` em goroutine destacado (timeout + recover + descarte), grava no store, **não aplica** o verdict.
- `toShadowTrace(trace, pc)` — mapeia `DeliberationTrace` → `ShadowTrace` (taxonomia §3.3).
- Log do bloco "COSCA Cognitive Shadow Mode" (modo humano/JSON).
- **Gate:** o teste de equivalência (§6) passa — resposta IDÊNTICA com Shadow ligado vs desligado.

### Fase 2 — `cosca shadow` + relatórios (consumo)
- `cosca shadow` (summary / list / report / --json) lendo o store.
- REST `GET /v1/shadow/spectrum` (opcional).
- **Gate:** report com dados sintéticos (mock store) → agregados corretos; `go test ./internal/cli/... ./internal/shadow/...`.

### Fase 3 — Percepção vision + MCP cognitivo (porta cognitiva completa)
- Alimentar `Positions` com percepção vision (`internal/vision`) — "perceber" entra na deliberação.
- Portar o Shadow como tool MCP (`shadow:observe`) ao reviver a porta MCP (`COSCA_ENABLE_MCP=1`).
- **Gate:** integração MCP com o Shadow; vision como evidência `EvidenceIDs` (proveniência I4).

### Fase 4 — Calibração + auto-resolução (a autoridade com dados)
- Cruzar Shadow × resposta real da LLM → calibrar `emit_threshold`/`reservation_threshold`/`min_score` com dados reais (ADR-031).
- Quando a taxa de auto-resolução estiver **validada** (Shadow `EMIT_OK` _correto_ em N amostras), promover do Shadow para Ativo (`enabled=true`) — a **ponte ganha autoridade depois de provada**.
- **Gate:** calibração com critério estatístico (bootstrap CI, determinístico — princípio do ruflo), e o Shadow continua rodando como rede de segurança.

> **Nota de rigor:** as Fases 1-4 não devem ser implementadas antes da Fase 0. Sem o wiring fail-closed + o store, qualquer bug do Shadow derruba o fluxo. A Fase 0 é o pré-requisito de segurança. E a Fase 4 **nunca** deve promover a autoridade sem os dados da Fase 1/2 provarem a calibração (medir antes de dar autoridade).

---

## 5. Consequências

**Positivas:**
- **Mede antes de dar autoridade:** descobre quanto o COSCA já decide sozinho, com dados reais, SEM risco de resposta errada.
- **Calibração com evidência:** os thresholds do ADR-032 deixam de ser chute — passam a ser calibrados por observação (ADR-031: medir antes de otimizar).
- **Validação do ADR-032:** o Shadow é a rede de segurança que permite ligar o modo ativo depois que a ponte provou sabedoria.
- **Reuso total:** zero sistema paralelo; um único `Deliberate()`, dois comportamentos de aplicação.
- **NÃO-interferência garantida por construção + teste:** a resposta é bit-a-bit idêntica.
- **Risco reversível:** desligar o Shadow (`shadow_mode=false`) não deixa rastro no fluxo; o store é descartável.

**Negativas / trade-offs:**
- **Overhead de persistência:** cada execução grava uma linha JSONL no store (I/O assíncrono, barato). Mitigado por append-only + goroutine (fora do caminho crítico) e pelo `shadow_discarded` (zero-custo se o store falhar).
- **Goroutine destacada:** há custo de goroutine por execução (mínimo) e a orquestração fica assíncrona em 1 ponto. Mitigado pelo timeout + recover + o fato do `Deliberate()` ser microsegundos (nunca observável na prática).
- **Volume de log:** cada execução gera um bloco + uma linha. Mitigado por: nível `debug` por default + `cosca shadow` agrega (não imprime cada execução).
- **Dados contrafactuais, não factuais:** o `WouldRespond` é o que o Kernel **teria** dito, não o que foi dito. Honesto no ADR (é um indicador, não um alvo).

### O que NÃO muda
- **`internal/embed`** intocado (P8).
- **O Executor** — continua o único ponto de chamada à LLM; o Shadow nunca o faz pular.
- **O caminho legado** — com `shadow_mode=false` e `enabled=false`, o fluxo é exatamente o atual.
- **A cadeia de comando e a guarda do Don** — o Shadow apenas **recomenda** (contrafactual); nunca autoriza (P9).

---

## 6. Testes (Shadow registra SEM interferir)

O teste-espinha é a **equivalência da resposta**. Um mock de `ChatProvider` que **falha se chamado após resolver** — e que, quando chamado, retorna uma resposta fixa. Roda o MESMO request com Shadow ON e Shadow OFF e compara **`result.Response` byte-a-byte**.

1. **Equivalência de resposta (CRÍTICO):** Shadow ON vs OFF → `result.Response` idêntico; `pc.Data.LLMResponse` idêntico; `pc.Data.DeliberationHandled == false` nos dois; `pc.Data.AugmentedPrompt` idêntico. A ÚNICA diferença: o store do Shadow tem 1 registro a mais (e é o propósito).
2. **Não pulou a LLM:** com Shadow ON e evidência que daria `EMIT_OK` no ativo, o mock `ChatProvider` **é chamado** (`called == true`) — prova que o Shadow não curto-circuita.
3. **Não mutou o pc:** com Shadow ON, `pc.Data.DeliberationHandled == false`, `pc.Data.LLMResponse == ""` antes do executor, `pc.Data.AugmentedPrompt` inalterado pelo ramo Shadow.
4. **Descarte em falha (fail-closed):** injeta `Deliberate()` que **panica**/**engasga** → a resposta principal é produzida corretamente (fall-through); `shadow_discarded` incrementa; nenhum erro no request.
5. **Store correto:** com evidência conhecida, o `ShadowTrace` gravado tem `Decision`/`Confidence`/`EvidenceIDs` esperados. Tempo de `shadow_discarded == 0` num caso normal.
6. **Taxonomia:** `len(positions)==0` → `RETRIEVAL_INSUFFICIENT`/`WouldEscalate=true`; `EMIT_OK` → `WouldEscalate=false`/`WouldRespond` não-vazio.
7. **Regressão (existente):** `go test ./internal/orchestration/... ./internal/deliberate/... ./internal/config/... ./internal/shadow/...` verdes; comportamento atual preservado.
8. **Race:** `go test -race ./internal/orchestration/...` — o goroutine do Shadow lê um snapshot (G3) sem data race.

---

## 7. Alternativas consideradas

| Alternativa | Veredito |
|---|---|
| **A. Não fazer nada** (manter ADR-032 desligado, sem medir) | ❌ Fica-se cego sobre "quanto o COSCA resolve sozinho" e sobre a calibração. É o problema do Don. |
| **B. Fazer um Shadow como sistema de deliberação paralelo** (darwin/ruflo replay) | ❌ Duplica `deliberate`/`Deliberator`; dois conjuntos de gates que podem divergir; contradiz ADR-015 (reuso do padrão, não duplicação). |
| **C. Rodar o Shadow INLINE (síncrono)** | ⚠️ Simples, mas contradiz a receita do professor ("em paralelo, com timeout, descartar"). Latência do `Deliberate()` é ~µs, mas a receita pede o desenho paralelo+timeout+descarte. |
| **D. Shadow como DECORAÇÃO da deliberação ADR-032 (ESTE ADR)** | ✅ Reusa `Deliberator.Deliberate`; um único ponto de deliberação; roda em paralelo com timeout+descarte; registra sem aplicar a autoridade. É a "ponte sem autoridade" do professor. |
| **E. Shadow como dumper do `DeliberationTrace` em `Extra`** | ⚠️ Persistência volátil (in-memory, some no fim da execução) + não agregável. Não atende ao "quanto resolve sozinho". Precisa do store JSONL. |

---

## 8. Verificação (como saber que funciona)

1. **Fase 0:** `go test ./internal/{orchestration,config,shadow}/...` verde; `shadow_mode=false` conserva o fluxo atual bit-a-bit.
2. **Fase 1:** o teste de equivalência (§6) passa — resposta IDÊNTICA com Shadow ON vs OFF; o store do Shadow contém a decisão contrafactual.
3. **Fase 2:** `cosca shadow report` mostra a taxa de auto-resolução (EMIT_OK) e a taxa de escalada de forma legível; `--json` é máquina-parseável.
4. **Fase 3:** percepção vision entra como `EvidenceIDs`; tool MCP `shadow:observe` expõe o Shadow.
5. **Fase 4:** a taxa de auto-resolução validada (Shadow EMIT_OK correto em N amostras, bootstrap CI) autoriza promover para o modo ativo — a **ponte ganhou autoridade depois de medida**.
6. **Não-interferência:** `shadow_discarded ~0`; nenhuma execução com Shadow falha por causa do Shadow.

---

## Referências

- **ADR-032** — Kernel-First Deliberation (o motor que o Shadow decora); `internal/orchestration/deliberation.go`, `orchestrator.go:327-366`.
- **ADR-011** — deliberação determinística zero-LLM (`internal/deliberate`).
- **ADR-031** — token efficiency (calibrar thresholds com dados reais antes de promover; medir antes de otimizar).
- **ADR-015** — reuso do padrão (o Shadow espelha o padrão de store do `internal/cost`, não duplica).
- **`internal/cost`** — o padrão JSONL append-only (`.cosca/cost/records.jsonl`, `cosca cost`).
- **`internal/deliberate/**`** — `ComputeConvergence`, `ComputeConfidence`, `EvaluateEmit`, `Position.Effective` (zero achismo).
- **`internal/orchestration/executor.go:238`** — o skip de LLM via `DeliberationHandled` (que o Shadow NUNCA seta).
- **Ordem do Don + professor (2026-08-31)** — "o cérebro observa o próprio processo antes de ganhar autoridade sobre ele"; "o Shadow registra; a resposta continua para a LLM".

---

> **Autor:** Ordem do Don + orientação do professor (2026-08-31) | **Formalizado por:** cosca-architecture | **Revisão pendente:** cosca-cto + Don | **Status:** Proposed — Fase 0 (config + store + wiring) recomendada primeiro; o Shadow é a disciplina da ponte, anterior a dar autoridade à deliberação.
