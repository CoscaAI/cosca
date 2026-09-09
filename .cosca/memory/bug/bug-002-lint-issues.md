---
type: bug
key: bug-002-lint-issues
tags: [lint, quality, golangci-lint]
severity: low
timestamp: 2026-07-24T00:00:00Z
detected_in: cosca
fixed_in: commit 03860c2
confidence: 1.0
times_seen: 1
---

# Bug-002: 959 Linter Warnings Across Codebase

## Symptoms
- `golangci-lint run` reported 959 issues
- Mixed code styles, unused imports, formatting inconsistencies
- gofmt/goimports violations

## Causality Tree

### N1 — Causa Direta
959 issues acumuladas: imports não usados, formatação inconsistente, misspell, gofmt/goimports violations. Débito técnico de estilo acumulado em ~350 arquivos Go.

### N2 — Causa Arquitetural
O pipeline de build não tinha gate de qualidade estática. `golangci-lint` existia como ferramenta mas não era executado em CI nem como pre-commit hook. A arquitetura de CI permitia que código não-conforme chegasse à main.

### N3 — Causa de Processo
- CI configurado apenas para `go build` + `go test` — sem análise estática
- Sem pre-commit hooks configurados no repositório
- Desenvolvimento rápido sem tempo alocado para housekeeping
- Nenhum owner definido para qualidade de código

### N4 — Prevenção Sistêmica
- **CI gate:** `golangci-lint run` como required check em todo PR (não permite merge com warnings)
- **Pre-commit hook:** gofmt + goimports automáticos
- **Makefile target:** `make lint` documentado no CONTRIBUTING.md
- **Política:** zero warnings policy — qualquer warning bloqueia o merge
