# DECISION PROTOCOL — O fluxo de aprovação

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — o trilho da autoridade
> **Propósito**: referência operacional ÚNICA para como uma decisão nasce, é
> desafiada, sobe ao Don, é aprovada e registrada. Complementa o
> `GOVERNANCE.md` (conselhos) — isto é o operacional.

---

## 1. A CADEIA de decisão

```
IDEIA → KERNEL classifica → (estratégica?) → CONTRAFACTUAL (Gate 0.5) → DON aprova → EXECUTA → REGISTRA
```

| Decisão | Quem decide |
|---------|-------------|
| Estratégica (P0/P1) | **Don** (só ele) |
| Arquitetura / stack | CTO (com aval do Don se for grande) |
| Tática (P2/P3) | Kernel/Chief (registrada) |
| Trivial (sem oposição real) | Kernel (executa) |

---

## 2. O CONTRAFACTUAL — desafiar antes de cristalizar

Para decisão estratégica, o gate 0.5 pergunta: **"E se a decisão oposta fosse
tomada?"** (previne viés de confirmação).

- ¬A (oposto) formulado + evidência.
- 5 dimensões: risco, custo, tempo, conhecimento, reversibilidade.
- Outcome: **proceed** (segue) · **escalate** (revisa) · **reject** (sobe ao Don).

---

## 3. O REGISTRO — decisão sem trilha é decisão perdida

```bash
cosca decision   # trilha de decisão (Decision Trace) — explicabilidade auditável
```

- Toda decisão não-trivial registra: o quê, por quê, alternativas, evidência,
  confiança, quem decidiu.
- **P2 (imutabilidade histórica)**: o registro original nunca é alterado — só
  resultado/lições atualizáveis (ver LEARNING_PROTOCOL §Decision DNA).

---

## 4. REGRAS

1. **Nunca agir sem aprovação do Don** em decisão estratégica.
2. **Confirmar antes de destrutivo** (git reset, rm, branch delete).
3. **Contrafactual antes de P0/P1** — não cristalizar sem desafiar.
4. **Registrar toda decisão não-trivial** — o Don pode perguntar "por quê?" meses
   depois; a resposta tem que estar no DNA.

---

## 5. GOTCHAS

1. **Decidir sem contrafactual = viés** — o oposto não examinado é risco cego.
2. **Decisão sem registro = re-decisão** — o Don pergunta de novo e ninguém sabe.
3. **Urgência não é licença** — P0/P1 tem que subir ao Don, não ser decidido no atropelo.

---

## 6. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — o fluxo de aprovação |
