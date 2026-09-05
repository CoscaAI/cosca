# Organizational Memory Workflow

> **Workflow**: `cosca-org-memory`
> **Versão**: 1.0.0 | **Status**: active
> **Owner**: cosca-architecture | **Criado**: 2026-07-30
> **Engine**: F10.4 — Organizational Memory

---

## Objetivo

Detectar, registrar e promover padrões organizacionais do ecossistema Cosca: pares de agentes que funcionam bem juntos, sequências eficientes, conflitos recorrentes, timing de execução. Cada padrão vira um registro em `ORG_MEMORY.md` e, quando confiável, uma recomendação ao Don.

---

## Pré-condições

- Trust Registry populado (`memory/trust/TRUST_REGISTRY.md`)
- Engineering Timeline com dados recentes
- Pelo menos 3 unidades de trabalho analisadas (sprints/sessões)

---

## Workflow

### Step 1 — Coleta de dados

Coletar de todas as fontes:
- Trust Registry: quem executou o quê, com que outcome
- Engineering Timeline: ordem cronológica de commits
- git: conflitos em arquivos (mesma janela de edição)
- Metrics B1-B5: latência, reuso, débito

### Step 2 — Cálculo de métricas por par

Para cada par de agentes que co-executou tasks:
- `success_rate_em_conjunto`
- `conflitos` (mesmo arquivo, janela de conflito)
- `tempo_médio_em_conjunto`
- `resultado_combinado`

### Step 3 — Identificação de padrões

Padrão identificado com **3+ ocorrências** em unidades de trabalho distintas.

### Step 4 — Cálculo de confiança

```
org_pattern_confidence = occurrences × 0.4 +
                         consistency × 0.3 +
                         data_quality × 0.2 +
                         recency × 0.1
```

### Step 5 — Recomendação (≥ 0.70)

Padrões com confiança ≥ 0.70 viram recomendação ao Don.

### Step 6 — Decisão do Don

- **Aprova** → padrão vira regra de processo (DDNA de processo) + registra em `ORG_MEMORY.md`
- **Rejeita** → padrão fica como candidato, reavaliado no próximo ciclo

---

## Regras

- Padrão exige 3+ ocorrências (não é anedota)
- Confiança mínima 0.70 para recomendação
- Recomendação ≠ regra — Don aprova
- Correlação ≠ causalidade — nunca atribuir causa sem evidência
- Padrão obsoleto → move para Archived (não exclui)

---

## Post-Task (AUTO-EVOLUTION)

- [ ] Q1: learning registrado em `learnings.md`
- [ ] Q2: padrão extraído em `patterns.md`
- [ ] Q3: falha? registrada em `failures.md`
- [ ] Q4: confiança atualizada em `capability-profile.md`
- [ ] Q5: level-up? registrado em `evolution.md`

---

## Related

- [Engine SKILL](../engines/org-memory/SKILL.md)
- [Registro de Padrões](../engines/org-memory/ORG_MEMORY.md)
- [Trust Registry](../memory/trust/TRUST_REGISTRY.md)
- [Cognitive Maturity Implementation](cognitive-maturity-implementation.md)
