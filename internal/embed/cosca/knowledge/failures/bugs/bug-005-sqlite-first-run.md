---
type: bug
key: bug-005-sqlite-first-run
tags: [sqlite, migration, first-run, panic]
severity: high
timestamp: 2026-07-24T00:00:00Z
detected_in: cosca
fixed_in: commit 0ffb4da + c30fac3
confidence: 1.0
times_seen: 2
---

# Bug-005: SQLite Migration Panics on First Run

## Symptoms
- Fresh project: `panic()` in `getCurrentVersion()` when `schema_version` table doesn't exist
- New users unable to initialize project
- `panic()` instead of graceful error

## Causality Tree

### N1 — Causa Direta
`getCurrentVersion()` chamava `panic()` quando a tabela `schema_version` não existia (first-run). Não tratava o erro "no such table" como estado válido (versão 0).

### N2 — Causa Arquitetural
O sistema de migração não tinha um conceito de "estado inicial" (version 0). Assumia que o banco sempre existia com schema prévio. A arquitetura de inicialização não separava "zero state" (nada existe) de "error state" (algo deu errado).

### N3 — Causa de Processo
- Nenhum teste de "fresh install" — CI sempre rodava com estado pré-existente
- `panic()` usado em código de biblioteca (deveria ser `error` ou `log.Fatal` apenas em `main`)
- Review não questionou o tratamento de erro da migration

### N4 — Prevenção Sistêmica
- **CI gate:** teste de "clean state" obrigatório — rodar em diretório sem `.cosca/`
- **Padrão de erro:** `panic()` proibido em `internal/` — apenas `main()` pode usar; library code retorna `error`
- **Lint rule:** `forbidigo` configurado para bloquear `panic(` em todos os pacotes exceto `main`
- **Template de migration:** sempre iniciar tratando versão 0 como estado válido
