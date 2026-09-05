---
type: bug
key: bug-006-restart-broken
tags: [runtime, restart, state-machine, lifecycle, blocker]
severity: blocker
timestamp: 2026-07-28T00:00:00Z
detected_in: cosca
detected_by: cosca-runtime (code audit) + cosca-qa (classification)
confidence: 1.0
times_seen: 1
---

# Bug-006: Restart() Funcionalmente Quebrado

## Symptoms
- `cosca runtime restart` sempre falha
- `Restart()` retorna erro: "runtime already started (state: stopped)"
- CLI restart command é inutilizável

## Causality Tree

### N1 — Causa Direta
`Restart()` chama `Stop()` (runtime.go:415) → state = `StateStopped`. Depois chama `Start()` (runtime.go:425). `Start()` verifica `state == StateUninitialized` (runtime.go:320) — não é, é `StateStopped` — retorna erro.

A transição `Stopped→Uninitialized` está DEFINIDA no transition map (state.go:106, linha `StateStopped: {StateUninitialized}`) mas **nunca é executada** dentro da função `Restart()`.

### N2 — Causa Arquitetural
A state machine tem o transition map declarativo correto, mas a função `Restart()` não o utiliza. O contrato de `Start()` exige `StateUninitialized`, e `Stop()` deixa `StateStopped`. Falta uma camada de "reset" entre Stop e Start que execute a transição permitida.

### N3 — Causa de Processo
- Nenhum integration test cobre o fluxo completo Start→Stop→Restart→Stop
- O teste `TestRestartReportsStateIssue` (runtime_extended_test.go:449) documenta o bug mas não falha (usa `_ = err`)
- Review do código de Restart não questionou o gap entre transition map e código real

### N4 — Prevenção Sistêmica
- **CI gate:** Integration test obrigatório para todos os fluxos de lifecycle (Start, Stop, Restart, signal handling)
- **Test pattern:** Testes de lifecycle devem ASSERT sucesso, não apenas logar o erro
- **Code review checklist:** Verificar que funções que invocam state transitions realmente executam as transições definidas no mapa

## Detection Pattern
```bash
grep -A10 "func.*Restart" internal/runtime/runtime.go
```

Teste existente que documenta (mas não valida) o bug:
```go
// runtime_extended_test.go:449-464
func TestRestartReportsStateIssue(t *testing.T) { ... }
```

## Fix Expected
Entre `Stop()` e `Start()` dentro de `Restart()`, executar transição `Stopped→Uninitialized`:
```go
// After Stop(), before Start():
if err := r.state.TransitionTo(StateUninitialized, "restart reset"); err != nil {
    return fmt.Errorf("reset state for restart: %w", err)
}
```

## Affected Files
- `internal/runtime/runtime.go:408-431` (Restart function)
- `internal/runtime/runtime_extended_test.go:449-464` (test que documenta o bug)
- `internal/cli/runtime.go:101-113` (CLI restart command)
