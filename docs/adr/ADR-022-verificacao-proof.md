# ADR-022: Verificação & Proof — evolução só promove com prova estatística + receipt

> **Status:** Proposed | **Owner:** cosca-kernel + cosca-architecture | **Last Updated:** 2026-08-28
> **Revisão:** aguardando Don + cosca-cto. **Design aditivo — não quebra o Root.**
> **Referência (base):** mineração `ruvnet` (ruflo, RuView, RuVector, federated-mcp, metaharness) +
> `ADR-016` (Evolution Engine) + `ADR-017` (Borrowing) + invariantes I1–I8.

---

## 0. Contexto — 5 minas do `ruvnet`, 1 tese comum

Mineramos 5 repositórios de `ruvnet`. **A tese comum é ouro: eles validam a arquitetura do Cosca**
(*"o modelo propõe; o harness decide; os algoritmos verificam"* — I1). E as **desmistificações** são
honestas e relevantes:

- **`federated-mcp` 🔴 FALSO**: não implementa MCP (protocolo custom `{type,content}`, zero métodos MCP,
  `verifyToken` nunca chamado, handler só `console.log`, CORS `*`). **Não adotar nada.** Só o *conceito*
  (Registry/Router de MCP) aponta um gap real do Cosca.
- **`RuVector` ⚠️ "self-learning" exagerado**: **não reweighta vetores** (congelados); "aprende" via
  Q-learning (behavior) + ledger de crenças. **Confirma I1 do Cosca.** int8 SIMD = paridade.
- **`RuView` ⚠️ RF não é mágica**: só presença/movimento/vitais + trilateração; **CRV = pseudo-ciência**
  (ignorar). A joia é a **epistemologia**, não o RF.

## 1. O que ADAPTAR (as gemas, priorizadas)

| # | Gema | Fonte | O que faz | Ix |
|---|---|---|---|---|
| **1** | **Evolution Gate COM PROVA** (bootstrap CI + receipt + promoção transacional) | ruflo | Candidato só promove com **significância estatística** (bootstrap CI low > 0) + receipt imutável — fecha o `stage.go` (que é só projeção). | **✅ IMPLEMENTADO** |
| **2** | **Receipt-chain replayable + 4-gate conjuntivo + envelope kernel-decide/host-execute** | metaharness | "Sistema Decide (I1)" formalizado + auditoria replayable + I7/I8 (externo = executor puro). | 🔵 |
| **3** | **Epistemic stamp em observação espacial** (`trust_state/uncertainty/source`) | RuView | I4 operacionalizado: a observação **sabe o que é** (Known/Degraded/Unknown + covariância + proveniência). | 🔵 |
| **4** | **Conformal prediction + containment de poison transitiva** | RuVector | Confiança **calibrada** + rejeitar crença → **rebaixa dependentes** (sanidade epistemológica reversível). | 🔵 |
| **5** | **Checkpoint/rollback O(1) por tick** | ruflo | Ticks que mutam estado seguros + rollback barato — destrava World Building. | 🔵 |
| **6** | **MCP Registry/Router → skill-surface unificada** | federated-mcp (conceito) | Agregar N servidores MCP, descobrir tools, **namespacing** `serverA.tool`. *(Não está no repo — é gap real do Cosca.)* | 🔵 |

## 2. O que REJEITAR

- `federated-mcp` inteiro (falso MCP, dead-code, auth-teatro, build quebrado).
- **Rust/WASM/TS/Node stacks** (binário único Go; ADR-017 — ideia, não arquitetura).
- **"Self-learning via reweight de vetores"** (não existe; embeddings congelados).
- **CRV** (pseudo-ciência) · **swarm-by-YAML + `Math.random()` consenso** · **substituir o gate HUMANO
  do Don por algoritmo puro** (a autoridade do Don permanece como gate externo).

## 3. Invariantes

A gema #1 (implementada) reforça **I1** (determinístico, zero LLM), **I2** (fail-closed: amostras
insuficientes ou sem melhoria → REJECT — nunca "promove por sorte"), **I5** (receipt hash-chained
tamper-evidente) e **I4** (deltas medidos em held-out, não opinados). As gemas #2–#5 reforçam
I7/I8 (envelope/claims), I4 (epistemic stamp, conformal, containment).

## 4. Recomendação

1. **✅ Gema #1 implementada** (`internal/evolution/proof.go`: `ProofGate` + `BootstrapCILow` +
   `ProofReceipt` hash-chained, `VerdictAccept/Reject`). Fecha o gap do ADR-016.
2. **Seguir:** gema #3 (epistemic stamp — barato, I4) → gema #4 (conformal + containment) →
   gema #5 (checkpoint/rollback, destrava World Building).
3. **Consolidar:** o `ruvnet` é 90% gangue / 10% minério — mineramos a ideia, nunca a pilha. A tese
   comum (LLM propõe, sistema decide, verificação por prova) já é o DNA do Cosca.

---

*O `ruvnet` valida o Cosca: a autoridade é do sistema (gate I1), a melhoria é por PROVA — nunca por
sorte. Compomos sobre `internal/evolution` + `deliberate` + `gate` + `ledger`; selecionamos, não copiamos.*
