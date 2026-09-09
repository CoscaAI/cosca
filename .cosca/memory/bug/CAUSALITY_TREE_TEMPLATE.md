---
type: template
key: causality-tree-template
tags: [bug, causality, root-cause, template, v1.4.0]
version: 2.0.0
created: 2026-07-28
---

# Causality Tree — Bug Analysis Template

> **Version 2.0.0** — Upgrade do formato flat (v1.0) para árvore de causalidade em 4 níveis.
> Aplicado a partir de 2026-07-28. Bugs existentes serão retroativamente atualizados.

---

## Estrutura da Árvore de Causalidade

```
BUG (sintoma)
 │
 ├── Nível 1: CAUSA DIRETA — o que quebrou no código
 │    └── "nil pointer dereference", "race condition no map X", etc.
 │
 ├── Nível 2: CAUSA ARQUITETURAL — por que o sistema permitiu que isso acontecesse
 │    └── "falta validação de boundary", "sem interface de sincronização", etc.
 │
 ├── Nível 3: CAUSA DE PROCESSO — qual falha no processo de desenvolvimento permitiu
 │    └── "sem review de PR", "sem teste de first-run", "sem CI com race detector", etc.
 │
 └── Nível 4: PREVENÇÃO SISTÊMICA — o que impede que bugs desta classe voltem a ocorrer
      └── "lint rule", "CI gate", "template de código", "checklist de review", etc.
```

---

## Template de Seções (para novos bugs)

```markdown
## Causality Tree

### N1 — Causa Direta
[O que quebrou no código. Fato técnico específico.]

### N2 — Causa Arquitetural
[Por que o sistema/arquitetura permitiu que o N1 acontecesse.
Ausência de abstração? Boundary não validado? Módulo sem contrato?]

### N3 — Causa de Processo
[Qual falha no processo de desenvolvimento permitiu N1+N2 chegarem à main.
Falta de teste? Review ausente? CI incompleto?]

### N4 — Prevenção Sistêmica
[O que foi implementado para impedir que bugs desta CLASSE voltem.
Não é "arrumar o código" — é "mudar o sistema pra detectar antes".]
```

---

## Critérios de Qualidade

| Nível | Critério |
|-------|----------|
| N1 | Deve citar linha, função, ou estrutura específica. Não pode ser vago ("problema de concorrência"). |
| N2 | Deve questionar a arquitetura. "Por que o sistema não me protegeu disso?" |
| N3 | Deve expor o processo. Sem autopiedade, sem culpa — fato. "Faltou X no pipeline." |
| N4 | Deve ser acionável e mensurável. "Adicionar lint rule Y" ou "Adicionar gate Z no CI". |
