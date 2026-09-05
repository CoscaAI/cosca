# ADR-011: Padrões-ouro do Mega Brain — Deliberação Meta-Cognitiva Evidência-Gated + Orquestração Plan-Only

> **Status:** Proposto (para revisão) | **Owner:** Architecture Chief | **Pauta:** CTO → Kernel → Don
> **Fonte minerada:** `.cosca/fallback/knowledge/patterns/mega-brain-patterns.md` (seções A1–A7, D1–D7)
> **Escopo:** ADR de DECISÃO/DESIGN. NÃO implementa — aprova revisão antes de qualquer código.

---

## 1. Contexto / Problema

O Cosca **tem** RAG, critic, gates de qualidade, engine de orquestração e um Ciclo de Decisão de 10 passos
(`internal/embed/cosca/CONSTITUTION.md` PARTE IV). Mas, frente aos padrões-ouro do Mega Brain, há **gaps
estruturais** em duas áreas: **deliberação** e **orquestração**.

### 1.1 Deliberação — o Ciclo de Decisão não delibera, ele auto-avalia

| Existe hoje | Gap vs Mega Brain | Ref. |
|---|---|---|
| Passo 8 CRÍTICA é **auto-avaliação de 1 agente** ("honesta — sem ego") | Não é deliberação **multi-agente**: ninguém desafia o dono da resposta com uma camada **sem DNA de domínio** | A1 |
| `CONFIDENCE_MODEL.md` pondera evidência (L0–L5, M1–M7) e resolve conflito (≥0.30 vence) | **Nenhuma posição** exige ID de evidência rastreável; "zero achismo" não é aplicado ao pronunciamento do agente | A2 |
| `cosca-critic` emite `RECOMMENDATION: PROCEED/REVISE/REJECT + CONFIDENCE: 0.0-1.0` | Confiança é **intuitiva**, sem breakdown aritmético; **sem thresholds EMITIR/COM-RESSALVAS/ESCALAR**; **sem convergência calculada**, **sem circuit breaker** e **sem detecção de loop** por hash | A3, A4 |
| `ChainExecutor.ExecuteParallel` (chain.go) dispara agentes em paralelo | **Sem votação cruzada** com auto-voto PROIBIDO; ninguém escora os outros | A5 |
| `cosca-critic`/`cosca-review` produzem **prosa** (Risks/Alternatives/Verdict) | **Sem síntese como SPEC acionável** (file + Action + Metric + Acceptance) que vira trabalho auditável | A7 |

**Verdicto do gap:** a Constituição manda (P2, P13, G4) que o sistema seja evidência-vestido e nunca afirme
certeza absoluta. Mas **não existe camada determinística** que transforme "achismo do agente" em
`(confiança aritmética, emitir ou escalar)`. O agente é juiz e parte da própria decisão.

### 1.2 Orquestração — o orchestrator executa sem plan-only

| Existe hoje | Gap vs Mega Brain | Ref. |
|---|---|---|
| `internal/orchestration.Engine.Execute` roda **inline**: context → router → executor(LLM) → pipeline | **sem separação plan-only**: o plano nunca é um artefato aprovável antes de executar; não há `plan.yaml`/`success_criteria`/`falsifiable_assumptions` | D1 |
| `internal/gate.GateStore` (plan→approving→approved→executed) é um FSM de aprovação com guarda de papel | Existe o **silogismo certo** (a guarda do Don), mas o gate não consome um plano com critérios; `plan_ref` é um ID vago | D1, D6 |
| `Router` (keyword) + `SemanticRouter` (embedding) escolhem o melhor agente, fallback → CEO | **sem confidence de roteamento**, **sem multi-banda** (≥0.80 / 0.60–0.79 / <0.60), **sem elicitation gate** | D3 |
| `internal/workflow` (DAG typed-routing, parallel fan-out, cycle detection) | **sem CPM** (caminho crítico/slack) e **sem FMEA RPN** por nó no planejamento | D4 |
| KERNEL marca feature-flags (runtime.parallel-execution STABLE, dag.optimization EXPERIMENTAL) | **sem trajetória explícita de maturidade** 0→1→10→100 nem "modo por complexidade" | D5 |
| `QUALITY_GATES.md` (Gates 0–4 + 0.5) pontua em **grau A–F** de média ponderada | **sem gate de 3 estados** por fase (APPROVE/REVIEW/VETO) nem `veto_conditions` hard-stop | D6 |

**Verdicto do gap:** existe um executor de DAG (`internal/workflow`) e um FSM de aprovação (`internal/gate`),
mas **nenhum elo** que produza um plano primeiro (D1) e o submeta à guarda determinística antes de executar —
exatamente o que P9 ("IA propõe, o sistema decide") exige. Hoje, no `Engine.Execute`, a proposição e a
decisão acontecem no mesmo passo acoplado à chamada de LLM.

---

## 2. Decisão

Adotar, com **adaptação de escopo** (postura do Don: **segurança sem sobre-engenharia**), dois blocos:

1. **Bloco 1 — Deliberação meta-cognitiva evidência-gated** (A1, A2, A3, A4, A5, A7).
2. **Bloco 2 — Orquestração plan-only + maturidade** (D1, D3, D4, D5, D6).

### 2.1 O que adoptamos fielmente

- **A2** — Toda posição no conselho carrega IDs de evidência rastreáveis. Sem evidência → rebaixa a confiança
  daquela posição (reuso do RAG já em `PipelineData.KnowledgeResults`/`MemoryResults`).
- **A3** — Convergência **calculada** (`Σ(peso × concordância)`), threshold 0.70, com **circuit breaker** claro
  (max rounds + **detecção de loop por hash** de posições). Divergência não-resolvida vira tensão registrada.
- **A4** — **Confiança aritmética** com breakdown auditável (base convergência ± ajustes tipados) e **thresholds**
  EMITIR (≥0.70) / EMITIR-COM-RESSALVAS (0.50–0.69 com mitigação) / ESCALAR (<0.50). Proibido "Confiança: Alta".
  O sistema, não o modelo, decide a emissão (P9).
- **A5 (núcleo)** — **Auto-voto PROIBIDO**: cada revisador escora os outros, nunca a si mesmo. (A panóplia de
  batalha — juiz-relay, 5 fases, tiebreaker — é descartada como sobre-engenharia; ver §6.)
- **A7** — Síntese como **SPEC acionável** (FINDINGS / CROSS-AGENT / SPEC / ACTIONS / CONFIDENCE), validada por
  `ValidateSpec()`; nunca "resumo narrativo".
- **D1** — **Plan-only** separado da execução: o arquiteto de plano **nunca executa**; emite
  plan.yaml/md/json + audit.jsonl + success_criteria + falsifiable_assumptions.
- **D3 (núcleo)** — Roteamento por intenção com **confidence + multi-banda + elicitation limitada** (máx 3 perguntas).
- **D6** — Quality gate de **3 estados** (APPROVE/REVIEW/VETO) com `veto_conditions` hard-stop, por fase.

### 2.2 O que adaptamos (D1 "gate humano obrigatório" e A1 — sem criar agentes novos)

- **A1** ("conselho sem DNA de domínio"): NÃO criamos o trio Crítico Metodológico/Advogado do Diabo/Sintetizador.
  Reusamos o `cosca-critic` (já é meta-cognitivo, escora *processo* e é adversarial) + **um** revisor de domínio
  **não-dono** (ex.: `cosca-qa` p/ verificação, ou `cosca-review` p/ processo). A régua é: **quem propõe não revisa
  a própria decisão**; quem é dono do tema não é o único a validar o raciocínio.
- **D4**: o CPM/FMEA fica como **analisador de planejamento** que consome o DAG já existente de
  `internal/workflow` (não reescrevemos o engine de execução). Validação de ciclo já existe (Kahn-like via
  `ErrCycle`), o que adicionamos é **caminho crítico** e **RPN** por nó.
- **D5**: a trajetória 0→1→10→100 é **documentada + virada em flag de maturidade/modo**, não um engine novo.

### 2.3 Decisões NEGATIVAS (o que NÃO entramos agora — honestidade)

Deferimos explicitamente por **sobre-engenharia para o Cosca de hoje** (detalhe e motivo em §5):

- A5 completo (batalha adversarial com juiz-relay, 5 fases, margem ≤5%) — teatro sobre-engenharia.
- A6 (3 adversários arquetípicos + taxonomia universal de defeitos D1–D10) — mantemos só a **FMEA RPN**.
- B1–B7 (DNA cognitivo / pipeline MCE) — preocupação separada (extração de conhecimento), **fora deste ADR**.
- C2/C4/C5/C6/C7 (RAG híbrido, invariante de embedding, cascata self-RAG→HHEM→block/flag, atribuição por claim,
  gabarito congelado, graph/ontology, BrainHealth) — **fora do escopo**; é um ADR próprio de RAG "zero achismo".
- D7 (token-fencing + fila durável Postgres + FSM de 18 estados + reaper) — sobre-engenharia para orquestração
  **in-process** SQLite; o `internal/workflow` já garante concorrência e detecção de ciclo em processo.

---

## 3. Desenho Técnico

### 3.1 Bloco 1 — `internal/deliberate` (novo, determinístico, zero-LLM)

O **núcleo é determinístico**; a LLM só preenche as *posições* (claim + evidência). A **aritmética e os gates são
externos ao modelo** — isso satisfaz P9 e P2. Tudo testável por `go test` sem rede.

**Novos arquivos em `internal/deliberate/`:**

- `types.go`
  - `Position` — `{ID, Claim, EvidenceIDs []string, Owner string, Substantiated bool}`. `Substantiated=false`
    quando `len(EvidenceIDs)==0` (rebaixa confiança — A2).
  - `Evidence` — `{ID, Level int /*0–5*/, BaseWeight float64, Source string, Concrete bool /*M1*/, ...}`.
  - `Review` — `{Reviewer string, Target string /*não == Reviewer*/, Score float64}`. Flag `SelfVote` inválida
    quando `Target == Reviewer` (A5).
  - `Decision` — `{Positions []Position, Reviews []Review, Convergence float64, Confidence ConfidenceBreakdown,
    Emit Emit, Spec SynthesisSpec}`.
  - `Emit` — `EmitOK | EmitWithReservations | Escalate`.
  - `ConvergenceWeights` — `{Recommendation:0.30, Premises:0.25, Risks:0.25, Timing:0.20}` (defaults A3).
  - `ConfidenceBreakdown` — `{Base float64, Adjustments []float64, Final float64}`.
  - `SynthesisSpec` — `{Findings []string, CrossAgent CrossAgent, Spec []SpecItem, Actions []Action,
    Confidence float64}`.
  - `SpecItem` — `{File string, Action string, Metric string, Acceptance string}` (A7).
- `convergence.go`
  - `ComputeConvergence(positions []Position, w ConvergenceWeights) (float64, bool)` — Σ(peso × concordância) e
    deu ≥0.70? Threshold parametrizável.
  - `DetectLoop(roundHashes []string) bool` — mesmo hash 2× = loop (A3 circuit breaker). Constantes:
    `MaxRounds=3`, `MaxIterations=5`, `Timeout=300s`.
- `confidence.go`
  - `ComputeConfidence(base float64, adj ConfidenceAdjustments) ConfidenceBreakdown` — aplica ajustes tipados de
    A4 (Critic<0.70 → −0.20; risco Alta/Catastrófico → −0.15; Alta → −0.10; Média → −0.05; contradição
    não-resolvida → −0.10; evidência em 3+ fontes → +0.10), clamp [0,1].
  - `Emit(breakdown ConfidenceBreakdown) Emit` — ≥0.70 → EmitOK; 0.50–0.69 → EmitWithReservations;
    <0.50 → Escalate. `Escalate` **não re-roda** (anti-loop A4).
- `evidence.go`
  - `EvidenceConfidence(ev Evidence) float64` — espelha `CONFIDENCE_MODEL.md` (base + M1–M7), clamp [0,1],
    revertendo à fonte. Reusa conceitualmente `internal/confidence` (tracker de agente) mas é **evidência x claim**.
- `pool.go`
  - `VoteCross(reviews []Review, owner string) (ReviewAggregate, error)` — valida que nenhum
    `Target == Reviewer` (auto-voto proibido) e retorna `Σ(score×peso)` sem incluir o owner.
- `spec.go`
  - `ValidateSpec(spec SynthesisSpec) error` — exige FINDINGS/CROSS-AGENT/SPEC/ACTIONS/CONFIDENCE presentes e
    cada `SpecItem` com File+Action+Metric+Acceptance. Rejeita anti-padrões (refs vagas, cross-agent ausente,
    confiança intuitiva, ação vaga, sem critérios de reversão) (A7).

**Integração (reuso, não duplicação):**

- `internal/orchestration/ports.go`: adicionar port
  `type DecisionDeliberator interface { Deliberate(ctx, pc PipelineContext) (deliberate.Decision, error) }`.
- `internal/orchestration/orchestrator.go`:
  - `OrchestratorConfig` ganha `DeliberateConfig { Enabled bool; P0P1Only bool; Weights ConvergenceWeights;
    EmitThreshold float64 }`.
  - Novo método `Deliberate(ctx, req) (*deliberate.Decision, error)` — roda **depois do Router, antes do Executor**,
    **apenas** se `Deliberate.Enabled` e a decisão for estratégica (P0/P1, via prioridade do request/flag).
  - `Engine.Execute` consulta o resultado em `pc.Data` (`pc.WithExtra("deliberation", decision)`). Se `Emit==Escalate`,
    retorna erro `ErrEscalateHuman` **sem executar** (o sistema decide). Se `EmitWithReservations`, anexa o plano
    de mitigação ao plano (não bloqueia, sinaliza).
  - `internal/orchestration/factory.go`: injeta o `DecisionDeliberator` (construído sobre o `cosca-critic` — um
    adapter `criticAdapter` que chama o agente via `AgentNode`/chat — e um `reviewAdapter` de domínio não-dono).
- Evidência rastreável: as posições são povoadas com `chunk_id`/`source:chunk` vindos de
  `PipelineData.KnowledgeResults` / `MemoryResults` (já disponíveis no pipeline — A2 sem infra nova).

### 3.2 Bloco 2 — `internal/orchestration` (plan-only) + `internal/gate` (3 estados)

**Arquivos que mudam/estendem (sem derrubar o existente):**

- `internal/orchestration/plan.go` (novo)
  - `Plan` — `{ID, PlanOnly bool /*true*/, DAG *workflow.Workflow, Cost Estimate, Risks []Risk,
    SuccessCriteria []string, FalsifiableAssumptions []string, AuditPath string, GateID string}`.
  - `PlanArchitect.Build(ctx, req) (*Plan, error)` — único ponto de entrada do plano; **nunca executa** (D1).
    Decompõe em nós de capability, arestas output→input, `parallelizable_with`, e emite o plano.
- `internal/orchestration/dag.go` (novo)
  - Reusa `internal/workflow` como substrato de DAG (já tem `JoinNode` paralelo + `ErrCycle`).
  - `CriticalPath(w *workflow.Workflow) ([]string, float64)` — CPM forward/backward, slack, caminho crítico (D4).
  - `FMEANodeRPN(node Risk) float64` — `RPN = Severity × Occurrence × Detectability` (D4/FMEA; mantemos só o RPN).
- `internal/orchestration/orchestrator.go`
  - `Engine.Plan(ctx, req) (*Plan, error)` → chama `PlanArchitect.Build` e **NÃO executa** (flag `plan_only`).
  - `Engine.ExecutePlan(ctx, plan) (*Result, error)` → **valida** via `internal/gate` que `plan.GateID` está em
    `StateApproved` (papel `don`); só então executa. `Engine.Execute` permanece como caminho legado.
- `internal/orchestration/router.go` + `semantic_router.go`
  - `Route` passa a retornar também `confidence float64`.
  - `ShouldConfirm(c float64) bool` → 0.60–0.79 (D3); `ShouldEscalate(c float64) bool` → <0.60.
  - `Elicit(ctx, pc, maxQuestions=3) ([]Question, error)` — elicitation por **info-gain**, limitada (D3).
- `internal/gate/gate.go`
  - Adiciona `QualityVerdict {VerdictApprove, VerdictReview, VerdictVeto}` (D6).
  - `EvaluateScore(metric float64, approveScore, reviewScore float64) QualityVerdict` — 3 score-boards.
  - `veto_conditions = ["critical_error","schema_violation","pii_leakage","authorized_mutation"]` — hard-stops.
  - `Veto(conditions []string) bool` — qualquer condição presente → VETO (block/revert), não request_rework.
  - `GateStore` continua guardando a trilha imutável; apenas o **verdict** é novo.

**Fluxo resultante (mapeia DECISION_PROTOCOL.md e P9):**

```
Request → Router (confidence multi-banda) ──[<0.60]─→ ESCALAR
        │        │ ──[0.60–0.79]─→ confirmação humana (elicitation ≤3)
        │        └──[≥0.80]→ PlanArchitect (plan_only, nunca executa)
        → Plan (plan.yaml/md/json + audit.jsonl + success_criteria + falsifiable_assumptions)
        → [P0/P1?] Deliberate gate (Bloco 1): EMIT / COM-RESSALVAS / ESCALAR
        → GateStore: plan → approving → approved (só don/admin)
        → ExecutePlan (executa o DAG aprovado)
        → Gate 3 estados (APPROVE/REVIEW/VETO) por fase
```

**Modo/maturidade (D5) como flag, não engine:**

- Flag `orchestration.mode` com valores `SIMPLE | STANDARD | COMPLEX | CRITICAL`. `SIMPLE` pula gates e
  deliberação; `CRITICAL` exige roundtable + require_human_signoff (D5). Default: `STANDARD`.
- Trajetória de maturidade documentada: **0→1** router único (hoje, routing accuracy >0.80) → **1→10** este ADR
  (plan-only + gate 3 estados) → **10→100** engine autônomo com auto-cura (futuro, fora do escopo). Métrica de
  progresso: `routing_accuracy`.

### 3.3 Reuso mínimo (nada duplicado)

| Precisa | Já existe | Ação |
|---|---|---|
| FSM de aprovação com guarda de papel | `internal/gate.GateStore` | **estender** (3 estados), não recriar |
| DAG com paralelismo + detecção de ciclo | `internal/workflow` | **consumir** como substrato |
| Evidência ponderada + resolução de conflito | `CONFIDENCE_MODEL.md` + `internal/confidence` | **reusar** conceitualmente |
| Gate de qualidade canônico (grade A–F) | `QUALITY_GATES.md` (embed) | **referenciar**; 3-estados vive em `internal/gate` (código), **sem tocar no embed** |
| Meta-cognição adversarial | `cosca-critic` (agente) | **reusar** como revisor 1; 1 revisor não-dono |
| Critérios de aceite / gating de release | `QUALITY_GATES` (G2, G3) | **reusar** |

---

## 4. Impacto

### O que MUDA

- Decisões estratégicas (P0/P1) passam por uma **camada determinística** que decide emitir/reavaliar/escalar —
  o modelo apenas **propõe**; o sistema **decide** (P9, G4).
- O plano deixa de ser implícito: vira artefato aprovável com `success_criteria` e `falsifiable_assumptions`,
  com trilha em `audit.jsonl` (P3 "nenhum agente age sem rastro").
- O `internal/gate` ganha o poder de **hard-stop** (veto) e reverte/reworka por fase, em vez de só aprovar.
- Roteamento perde a arbitrariedade: passa a ter banda de confiança e escalada humana como padrão.

### O que NÃO muda

- `internal/embed/cosca/**` **intocado** (P8): `CONSTITUTION.md`, `QUALITY_GATES.md`, `DECISION_PROTOCOL.md`,
  `metacognition-pipeline.md`, `CONFIDENCE_MODEL.md` são **só referenciados/ler**, nunca editados por código.
- O **Ciclo de Decisão de 10 passos** permanece — a deliberação **aumenta** os passos 2/3/8, não os substitui.
- A **tabela keyword de roteamento** (`router.go`) continua como base; o semantic router é opt-in e é o fallback.
- O **`Engine.Execute`** legado continua compilando e rodando (caminho de compatibilidade); o novo caminho
  é `Route → Plan → Gate → ExecutePlan`.
- A existência e o contrato dos agentes `cosca-critic`/`cosca-review`/`cosca-qa` não mudam — só ganham a
  **saída estruturada** (verdict + SPEC).
- A **cadeia de comando** (PARTE II) e a **guarda de papel** (quem aprova) são preservadas integralmente.

---

## 5. Trade-offs / Riscos

| Trade-off | Risco | Mitigação |
|---|---|---|
| Deliberação só em P0/P1 (slicing) | Deixa escapar decisão P2/P3 com viés | P2/P3 seguem no contrafactual leve / critic brief já existente |
| Reuso de `cosca-critic` como único revisor | Saturação de um só agente → viés único | Pool mínimo = revisor meta + 1 revisor não-dono; auto-voto proibido |
| `Plan`-first adiciona latência a todo request estratégico | Privação de "responta rápida" | Modo `SIMPLE` pula plan-only/gates p/ demanda de baixo risco |
| Aritmética de confiança pode rebaixar decisões corretas | False-negative / escalada excessiva ao Don | Thresholds calibrados (0.50/0.70) endereçam só o <0.50 p/ escalar |
| CPM/FMEA sobre `internal/workflow` é análise estática | Estimativas envelhecem | Recomputar na construção do plano; nunca persistir como verdade (P2) |
| `veto_conditions` hard-stop pode bloquear trabalho útil | Falso positivo de veto | Condições restritas a safety/authority (pii, mutation), owner=security |
| Contrato novo em `OrchestratorConfig` | Quebra de compatibilidade de config | Campos novos `DeliberateConfig`/`mode` opcionais, zero-flag por padrão |

**Risco constitucional (não ignorar):** qualquer mudança que toque o fluxo de aprovação de decisão estratégica
**precisa** confirmar P9 ("IA propõe, o sistema decide") e a guarda de papel do Don. A autorização da execução
**continua** do `internal/gate` + Don; a deliberação **nunca autoriza**, apenas recomenda emitir/escalar.

---

## 6. Fronteira do Incremento (fatia bounded primeiro)

### Bloco 1 — Fatia 1 (recommendada)

`internal/deliberate` com: `ComputeConvergence` + `DetectLoop` + `EvidenceConfidence` + `ComputeConfidence` +
`Emit` + `VoteCross` + `ValidateSpec` + **evidência rastreável obrigatória**. Integração **mínima**: deliberação
**só P0/P1**, invocada no `Engine` entre Router e Executor. Reuso: `cosca-critic` (revisor meta) + 1 revisor
não-dono; síntese em SPEC estruturada (não prosa).

- **Não entra na fatia 1**: batalha adversarial completa, juiz-relay, múltiplos arquétipos (A5/A6).
- **Critério de aceite (fatia 1)**: `go test ./internal/deliberate/` verde; `Emit` decide ≥0.70/0.50–0.69/<0.50;
  posição sem evidência rebaixa confiança; auto-voto é rejeitado.

### Bloco 2 — Fatia 1 (recommendada)

`Engine.Plan` + `internal/gate` com 3-estados + `confidence multi-banda` no `Router`. É a fatia de **maior valor
e menor risco**: entrega "IA propõe (plano), sistema decide (gate)" imediatamente, sem reescrever o executor.

- **Não entra na fatia 1**: CPM/FMEA (fatia 2), modo/maturidade (fatia 3), D7 (token-fencing — descartado).
- **Critério de aceite (fatia 1)**: plano emite `plan_only:true` e **não executa**; o executor só roda com gate
  `approved` + papel don; roteamento <0.60 escala humano.

### Sequência de fatias (se aprovado)

| Fatia | Bloco | Entrega | Risco |
|---|---|---|---|
| 1 | 2 | Plan-only + Gate 3 estados + confiança de roteamento | Baixo |
| 1 | 1 | Deliberação determinística (P0/P1) + SPEC | Médio |
| 2 | 2 | CPM + FMEA RPN no planejamento | Baixo |
| 2 | 1 | Votação cruzada multi-revisor (sem auto-voto) | Médio |
| 3 | 2 | Modo por complexidade + trajetória de maturidade | Baixo |

---

## 7. Alternativas Consideradas

| Alternativa | Por que não |
|---|---|
| **Não fazer nada** (deixar o Ciclo de Decisão como está) | Mantém o viés de confirmação da auto-avaliação e o "achismo" nos pronunciamentos — exatamente o que a Constituição (P2/P13/G4) proíbe |
| **Copiar o Conclave completo** (3 agentes: Crítico Metodológico, Advogado, Sintetizador) | Sobre-engenharia: o Cosca tem um critic e uma cadeia de comando; criar mais 2 agentes por decisão só aumenta custo de contexto sem ganho determinístico |
| **Reescrever o engine de orquestração** (DAG próprio com FSM + fila durável) | Duplica `internal/workflow` e `internal/gate`; o substrato já cobre DAG + guarda. Criar seria P8-corromper e anti-reuso |
| **Confiança por intuição do agente** (mantém `CONFIDENCE: 0.0–1.0` do critic) | Viola A4 (anti-padrão AP-4 "Confiança: Alta") e G4 (nunca 100%); sem break-down auditável não há P3 |
| **Gate de qualidade só com grau A–F** (como hoje) | Não distingue "revisar" de "vetar"; não tem hard-stop de segurança. O 3-estados é o mínimo necessário |
| **RAG "zero achismo" (C2/C4/C5/C6)** | Problema **real** e **importante**, mas **separado** da deliberação. Merece ADR próprio; introduzi-lo aqui explode o escopo |

---

## 8. Conclusão

O Cosca já tem os **ossos** (RAG, critic, gates, DAG, guarda do Don, Ciclo de Decisão). Faltam **dois
ligamentos determinísticos**: **(1)** transformar "achismo do agente" em decisão **aritmética** com evidence-track
e threshold de emissão, e **(2)** separar **planejar** de **executar**, submetendo o plano à guarda do Don antes
de rodar. Ambos são exatamente a régua da Constituição (P9 "IA propõe, o sistema decide"; P3 rastro; P2 evidência;
G4 nunca 100%). A disciplina do Don (segurança **sem** sobre-engenharia) recomenda **fatiar**: entregar a
deliberação determinística em P0/P1 e o plan-only + gate 3-estados na fatia 1, **deferindo** a batalha
adversarial completa, o DNA cognitivo e a cascata de fidelidade de RAG para contexto próprio.

> **Próximo passo (se aprovado):** fatiar 1 dos dois blocos, com ADR-012 para o RAG "zero achismo" caso o Don
> priorize a fidelidade de recuperação antes da deliberação.
