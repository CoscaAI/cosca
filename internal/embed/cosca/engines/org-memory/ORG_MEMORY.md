# Organizational Memory — Active Patterns Registry

> **Engine**: F10.4 | **Owner**: cosca-architecture | **Status**: active | **Created**: 2026-07-30
>
> Registro de padrões organizacionais ativos detectados pelo Organizational Memory Engine.
> Padrões com confiança ≥ 0.70 viram recomendações. Nada aqui é regra sem aprovação do Don.

---

## Active Patterns

### ORGP-2026-07-30-001 — Pairs de sucesso (testing + backend)

```yaml
id: ORGP-2026-07-30-001
pattern: "cosca-testing + cosca-backend juntos = alta taxa de sucesso"
dimension: colaboracao
occurrences: 3
confidence: 0.82
evidence:
  - "Sessão 2026-07-30: races corrigidas por testing, backend validou build"
  - "F2.4/F2.5: architecture projetou, testing validou cobertura"
status: candidate
recommendation: "Preferir o par testing+backend para tasks de correção"
created: 2026-07-30
```

### ORGP-2026-07-30-002 — Decisão isolada de API causa retrabalho

```yaml
id: ORGP-2026-07-30-002
pattern: "arquitetura decide API sozinho → frontend refaz 3x"
dimension: conflito
occurrences: 5
confidence: 0.78
evidence:
  - "Padrão observado em 5 sprints (S29-S33)"
  - "Trust Registry: conflitos em arquivos de API quando decision é unilateral"
status: candidate
recommendation: "Revisão conjunta obrigatória antes de decisões de API"
created: 2026-07-30
```

### ORGP-2026-07-30-003 — Tasks paralelas em pacotes distintos aceleram

```yaml
id: ORGP-2026-07-30-003
pattern: "tasks paralelas em pacotes diferentes = +40% velocidade"
dimension: timing
occurrences: 4
confidence: 0.75
evidence:
  - "Sessão 2026-07-30: 12 ondas paralelas, 30+ agentes, 0 conflitos de arquivo"
  - "B2 (decision velocity): tasks paralelas 3x mais rápidas"
status: candidate
recommendation: "Estruturar ondas com escopos em pacotes disjuntos"
created: 2026-07-30
```

### ORGP-2026-07-30-004 — Validação pós-onda obrigatória

```yaml
id: ORGP-2026-07-30-004
pattern: "agente com limite de passos precisa de validação pós-onda"
dimension: processo
occurrences: 3
confidence: 0.90
evidence:
  - "F2.4: SKILL.md criado mas 3 pendências"
  - "F10.2/F10.3/F10.4: stages 7-8 incompletos"
  - "L33: lição registrada pelo Kernel"
status: candidate
recommendation: "Todo agente que atingir limite de passos entra em fila de validação"
created: 2026-07-30
```

---

## Processed Patterns (approved → rules)

*Nenhum padrão aprovado pelo Don ainda.*

## Archived Patterns

*Nenhum padrão arquivado ainda.*

---

## How to Use

1. **Detecção**: pipeline F10.4 roda pós-sprint e identifica padrões com 3+ ocorrências
2. **Registro**: padrões são adicionados aqui com confiança calculada
3. **Recomendação**: confiança ≥ 0.70 → recomendação ao Don
4. **Aprovação**: Don aprova → padrão vira regra de processo (DDNA de processo)
5. **Arquivo**: padrão obsoleto → move para Archived
