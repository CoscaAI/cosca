---
type: bug
key: bug-003-race-conditions
tags: [concurrency, race-condition, goroutine]
severity: high
timestamp: 2026-07-20T00:00:00Z
detected_in: cosca
fixed_in: commit 9a950ff
confidence: 0.9
times_seen: 1
---

# Bug-003: Race Conditions in Parallel Execution

## Symptoms
- Intermittent test failures
- `go test -race` detected data races
- Parallel provider calls competing for shared state

## Causality Tree

### N1 — Causa Direta
Goroutines no pipeline de execução paralela acessando maps compartilhados (`providerInstances`, `activeChats`) sem sincronização. Data races detectadas pelo `go test -race`.

### N2 — Causa Arquitetural
O pipeline de execução paralela não definia contratos de thread-safety para estruturas compartilhadas. Maps eram tratados como seguros por padrão (não são em Go). A arquitetura não separava estado mutável de estado imutável.

### N3 — Causa de Processo
- `go test -race` não era executado em CI — rodava apenas `go test` simples
- Review de PR não questionou segurança de concorrência nos maps
- Documentação de thread-safety ausente nos pacotes

### N4 — Prevenção Sistêmica
- **CI gate:** `go test -race` obrigatório em todo PR e na main
- **Documentação:** todo pacote com estado compartilhado deve documentar thread-safety guarantees
- **Padrão de código:** preferir `sync.RWMutex` + channels; evitar maps compartilhados sem wrapper sincronizado
- **Lint rule:** `go vet` + `staticcheck` para detectar possíveis race conditions estaticamente
