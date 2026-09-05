# ADR-023: Control Loop & Engine-gated Execution — o sistema decide a convergência e a execução (I1 mecânico)

> **Status:** PARTIAL — Fase 1 implementada (2026-09-01, verificado em código) | **Owner:** cosca-kernel + cosca-architecture | **Last Updated:** 2026-09-01
> **Revisão:** itens 1 (workqueue k8s-style em `internal/pipeline/workqueue.go` + `control_loop.go`) e 4 (argument-aware deny em `internal/policy/mcp_policy.go`) implementados; item 2 (`internal/workflow/execgate.go`) presente. Fase 2 parcial. O header anterior "Proposed" estava STALE.
> **Referência (base):** big-tech mining (`kubernetes`, `sample-controller`, `vercel/workflow`, `n8n`,
> `aws/agent-toolkit`, `google/agents-cli`) + `ADR-017` (Crystallization — ratificado) + `ADR-016/018/019/020/021/022`.

---

## 0. Contexto — a mina-mãe valida a tese, e o Cosca JÁ fala a língua

Mineramos 6 projetos de classe mundial. **A tese do Crystallization (ADR-017 §1) foi confirmada em escala**:
k8s, n8n, vercel resolvem orquestração não com "IA mais esperta", mas com **mecanismo verificável** (loop de
reconcile, gate no engine, replay determinístico). E um achado muda o FIT: **o Cosca já tem muito disso**.

**O achado-âncora (auditoria):**
- `internal/pipeline/workqueue.go` é **cópia funcional** do `client-go/util/workqueue` (dedup + rate-limit +
  `AddRateLimited`/`Forget`/`NumRequeues`), **mas não é usado por nenhum prod** (só testes).
- `internal/pipeline/reconciler.go` já implementa **desired vs actual → drift → convergência** (padrão k8s/ArgoCD).
- `internal/dflow` já declara **workflow puro + replay determinístico sobre histórico** (mas *por comentário*).
- `internal/skills` já fala o **formato Agent Skills** (SKILL.md frontmatter, AllowedTools string, Plugin/
  Marketplace, quarentena I6) — idêntico ao `aws/agent-toolkit`.

**Ou seja:** não é "aprender k8s/n8n". É **cristalizar os mecanismos que o Cosca já tem embrionários** — o
padrão do ADR-017 §1: *capacidade da IA = matéria-prima; mecanismo verificável = produto.*

## 1. O que o Cosca JÁ tem (COMPARAR, não recriar)

| Big-tech | Cosca equivalente | Estado |
|---|---|---|
| k8s `util/workqueue` | `internal/pipeline/workqueue.go` | ✅ clone, falta bucket global |
| k8s reconciler | `internal/pipeline/reconciler.go` (`detectDrift`, `maxCycles=5`) | ✅ presente |
| k8s `input_hash`/fencing | `internal/durable/ledger.go` (`Generation`, fencing, `input_hash`) | ✅ presente |
| Agent Skills format | `internal/skills` (Skill/Marketplace/Plugin/quarantine) | ✅ idêntico |
| k8s controller loop contínuo | — | 🔴 **gaps** (reconcile é one-shot via REST, `workqueue` sem worker) |
| vercel deterministic replay | `internal/dflow` (contrato) | 🔵 conceito, falta mecanismo |

## 2. O que ADAPTAR (priorizado) — "Crystallization" em ação

| # | Ideia | Fonte | Vai para | Ix |
|---|---|---|---|---|
| **1** | **Control Loop contínuo**: `workqueue → worker → processNext → reconcile → AddRateLimited/Forget` + espera de cache sincronizado + skip por **revision-hash** (`input_hash` já calculado) | k8s + sample-controller | `internal/pipeline` | I1/I2 |
| **2** | **Engine-gated execution**: nó de IA devolve `EngineRequest` (proposta tipada) → **gate no engine** (não um nó) → executa → **retoma o nó**. É o I1 *dentro* do workflow. | n8n | `internal/workflow` + `internal/gate` | I1/I2 |
| **3** | **Determinismo mecânico + replay fail-closed**: congelar clock/RNG no corpo do workflow (ctx vedado), log denso invariante-por-prefixo, **divergência → fresh replay** (nunca "conserta"), **single-flight once-per-step** | vercel/workflow | `internal/dflow` | I1/I2 |
| **4** | **Argument-aware deny**: `mcp_policy.Evaluate(tool, _)` hoje **ignora `tool_input`** — denegar inspecionando argumentos (exfiltração de segredo) | aws/agent-toolkit | `internal/policy/mcp_policy.go` | I8 |
| **5** | **Resync + lister p/ World**: cache de leitura com selo epistemológico por entidade + resync (auto-cura de eventos perdidos) | k8s | `internal/worldmodel` / `ADR-021` | I4/I8 |
| **6** | **Eval-as-flywheel p/ AGENTES**: generate→grade→**cluster de falhas**→gate (comportamento de agente) | google/agents-cli | novo `internal/evalgo` | I1/I4 |
| **7** | **Nó = capability governada** (execute/supplyData/poll/trigger) + `allowed-tools` é **advisory** (gate no código) | n8n/aws | `internal/skills`/`internal/workflow` | I8 |

## 3. Invariantes (I1–I8) — veredito por ideia

- **I1**: os itens 1–7 são **determinísticos por construção** — o LLM PROPÕE (args tipados, plano, destino);
  o **sistema decide** (gate, reconcile, replay). O controller põe o "system decides" no lugar do "model
  proposes" da convergência. **I1 reforçado.**
- **I2**: item 3 (divergência → fresh replay, nunca conserta) e item 1 (backoff per-item + `Forget` no
  sucesso) são **fail-closed** de coração. **I2 reforçado.**
- **I3/I4**: item 5 (resync com stamp por entidade) e o reconciler lendo do cache (nunca da request) =
  I3/I4 por construção. Item 6: judge = medição (I4), nunca autoridade (I1).
- **I5**: o reconciler/executor deve commitar via `internal/ledger`; **não** confundir com o log de replay
  (separado). **I5 preservado.**
- **I7/I8**: nada entra como runtime novo (I7); item 4 (argument-aware deny) e item 7 (advisory tools)
  reforçam **I8** (externo nunca autoridade). Nada de sidecar/TS/Python (ADR-017 §7).

## 4. O que REJEITAR (com convicção)

- **API server / etcd / watch-RV / fila-3-níveis do scheduler / owner-refs / code-generator** — infra k8s;
  o Cosca é monólito Go local, não um cluster.
- **Runtime TS/Python/VM QuickJS** (`vercel/workflow`, `google-adk`, `n8n` nodes) — I7 (monólito binário único).
- **Licenças não-abertas**: `n8n` (Sustainable Use + Enterprise) = **zero cópia** de código.
- **`executionOrder v1` (posição no canvas)** — anti-I1.
- **Harness google-adk** — LLM decide a ação **sem gate** (I1 violado); o Cosca tem kernel melhor.
- **HITL como NÓ no grafo** — o gate no Cosca deve ser **decisão do engine**, não um nó contornável.
- **`eval run` exit-0-whatever-the-scores** — sem gate de promoção (anti ADR-016).

## 5. Recomendação (faseado, com evidência)

1. **Fase 1 (XS+S):** **Argument-aware deny** (item 4 — o único upgrade real de capacidade hoje) + compor
   o **control loop** (item 1) sobre o `workqueue`/`reconciler` já existentes (soldar peças, ~1-2 dias).
2. **Fase 2 (M):** **Engine-gated execution** (item 2) no `internal/workflow` + **determinismo mecânico +
   replay fail-closed** (item 3) no `internal/dflow`.
3. **Fase 3 (M):** **resync/lister World** (item 5) + **eval-as-flywheel p/ agentes** (item 6).
4. Cada fase: **aditiva + avaliada** (benchmark antes/depois — régua do `internal/performance`), com o
   **gate do Cosca** (ADR-016/017) — evidência antes de promover.

---

*O big-tech confirma: o Cosca não precisa de "IA mais esperta" — precisa cristalizar os mecanismos que já
tem embrionário (workqueue, reconciler, dflow, skills). Nós governamos; a IA propõe. Mecanismo verificável
= produto. Fica mais Cosca, não mais parecido com k8s/n8n/vercel.*
