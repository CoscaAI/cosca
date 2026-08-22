---
type: bug
key: bug-008-metrics-misdocumented
tags: [documentation, metrics, runtime, code-docs-sync, major]
severity: major
timestamp: 2026-07-28T00:00:00Z
detected_in: cosca
detected_by: cosca-runtime (code audit) + cosca-qa (classification)
confidence: 1.0
times_seen: 1
---

# Bug-008: Métricas Mal Documentadas

## Symptoms
- Documentação (`docs/runtime/overview.md`) descreve 7 métricas que não existem no código:
  - `runtime.state gauge`
  - `runtime.health gauge`
  - `runtime.start.duration histogram`
  - `runtime.stop.duration histogram`
  - `runtime.request.duration histogram`
  - `runtime.memory.usage gauge`
  - `runtime.goroutine.count gauge`
- O código (`internal/runtime/metrics.go`) contém 12 métricas reais NÃO documentadas:
  - 8 atomic counters: indexCount, searchCount, contextBuilds, memoryStores, pluginCalls, errorCount, syncCount, eventCount
  - 4 duration histograms: indexDurations, searchDurations, contextDurations, memoryDurations
- Usuários/agentes que consultam a documentação tomam decisões baseadas em informação falsa

## Causality Tree

### N1 — Causa Direta
Documentação e código divergiram. As 7 métricas documentadas foram provavelmente planejadas mas nunca implementadas. As 12 métricas reais foram implementadas mas nunca documentadas. Sem mecanismo de sincronização docs↔código.

### N2 — Causa Arquitetural
Não existe ferramenta ou processo de cross-reference entre claims de documentação e símbolos reais do código. A arquitetura de documentação é baseada em "escrever uma vez e esperar que continue verdade", sem validação contínua.

### N3 — Causa de Processo
- Doc-code validator não existe (planejado no CI do cosca-devops, mas não implementado)
- Atualização de documentação não é parte do definition of done
- Review de PR não verifica se documentação foi atualizada para refletir mudanças de código

### N4 — Prevenção Sistêmica
- **CI gate:** Doc-code validator que cruza claims em docs/ com símbolos exportados no código
- **Definition of done:** Documentação atualizada é parte obrigatória de qualquer mudança de código
- **Code review checklist:** "A documentação relevante foi atualizada?" como item obrigatório

## Detection Pattern
Comparar claims de documentação com símbolos reais:
```bash
# Métricas documentadas (docs/runtime/overview.md)
grep -i "metric\|gauge\|counter\|histogram" docs/runtime/overview.md

# Métricas reais (código)
grep -E "Int64|durationHistogram|atomic\." internal/runtime/metrics.go
```

## Fix Expected
1. Atualizar `docs/runtime/overview.md` para documentar as 12 métricas reais
2. Remover as 7 métricas fictícias da documentação
3. Implementar doc-code validator no CI (planejado pelo cosca-devops na Onda 2)

## Affected Files
- `docs/runtime/overview.md` (fonte da documentação incorreta)
- `internal/runtime/metrics.go` (fonte da verdade — 12 métricas reais)
