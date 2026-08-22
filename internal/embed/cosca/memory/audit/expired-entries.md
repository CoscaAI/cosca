# Audit — Expired Entries Registry

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Memory Chief | **Criado**: 2026-07-30

## Purpose
Registro de entradas de aprendizado que atingiram confiança abaixo do limiar crítico (confidence < 0.30). Estas entradas requerem ação imediata: revalidação ou depreciação.

## Current Expired Entries

*Nenhuma entrada expirada. O sistema Wisdom Decay foi ativado em 2026-07-30. Entradas migradas receberão confidence calculada retroativamente — as que já ultrapassarem 365 dias sem revalidação aparecerão aqui.*

---

## Entry Format

```markdown
### {original-timestamp} — {technique-name}
| Field | Value |
|-------|-------|
| **Agent** | cosca-{name} |
| **Category** | CRITICAL / STABLE / EXPERIMENTAL / DEPRECATED |
| **Last Validated** | YYYY-MM-DD |
| **Current Confidence** | 0.XX |
| **Days Without Validation** | N |
| **Recommended Action** | revalidate / deprecate / promote |
| **Reason** | Why this entry is expired and what should be done |
| **Status** | pending / resolved |
```

## Resolution Log

Entradas resolvidas são movidas para esta seção com nota de resolução.

---

## Action Policy

| Confidence Range | Action Required | Deadline |
|-----------------|----------------|----------|
| < 0.10 | Deprecate immediately | 24h |
| 0.10–0.19 | Revalidate or deprecate | 48h |
| 0.20–0.29 | Revalidate | 7 dias |
