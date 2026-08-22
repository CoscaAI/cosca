# ADR-009: Change Safety Level — Read-Only como Estratégia de Mudança (P14)

> **Status:** Accepted ✅ (Constituição P14) | **Owner:** Cosca Kernel
> **Last Updated:** 2026-08-14
> **Implementation:** L303 (P14), L307 (execGuard read-only total na UI)
> **Fonte:** professor (2026-08-14) — mandamentos 12, 13 da doutrina

## Context

O professor observou que o kernel propôs read-only ESPONTANEAMENTE para o
Trust Center (L302) e recomendou formalizar como disciplina permanente. O Don
aprovou com a ressalva explícita: **"nao eh pra criar api falsa"**.

### Decisão

**P14 — CHANGE SAFETY LEVEL** na Constituição:

```
OBSERVE → READ-ONLY → PREVIEW → APPROVAL → WRITE → VERIFY → COMMIT
```

1. Toda mudança escolhe o MÍNIMO nível de segurança necessário.
2. Transformações visuais operam em **READ-ONLY** por padrão.
3. **NUNCA criar API falsa**: se o backend não suporta, a UI mostra
   "Not available" — não inventa resultado.
4. Subir de nível (WRITE/EXECUTE) exige aprovação explícita do Don.

### Por quê

1. Reduz drasticamente o risco de quebrar o backend durante transformações
   visuais.
2. Permite validar a UX antes de conectar a parte "perigosa".
3. É a estratégia de mudança "primeiro observar → depois projetar → validar →
   modificar" — o comportamento de engenheiro supervisionado.

### Consequências

- **execGuard.ts** no cosca-code (L307): bloqueia POST para agent/team/
  workflow/plan/test/save/engines; leitura (GET) livre.
- Editor não salva, InlineAI só explica, PipelinePanel mostra o bloqueio.
- Backend intocado — o bloqueio é política da UI.
- Base para liberar execução gradualmente: READ → SIMULATE → PREVIEW →
  APPROVAL → EXECUTE.

### Alternativas rejeitadas

- **Liberar tudo de uma vez**: rejeitada — risco alto, sem validação da UX.
- **UI com APIs falsas**: rejeitada — a ordem do Don + mandamento 14
  (rastreabilidade) exigem honestidade visível.
