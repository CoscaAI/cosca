---
type: bug
key: bug-007-startup-event-timing
tags: [runtime, eventbus, startup, lifecycle, critical]
severity: critical
timestamp: 2026-07-28T00:00:00Z
detected_in: cosca
detected_by: cosca-runtime (code audit) + cosca-qa (classification)
confidence: 1.0
times_seen: 1
---

# Bug-007: EventStartupComplete Prematuro

## Symptoms
- Subscribers de `EventStartupComplete` recebem o evento quando o runtime ainda está em `StateInitializing`
- Init hooks (`lifecycle.ExecuteInit`) ainda não foram executados
- Subsystems ainda não foram iniciados (`lifecycle.ExecuteStart`)
- Qualquer subscriber que dependa de runtime pronto opera com estado falso

## Causality Tree

### N1 — Causa Direta
`EventStartupComplete` é publicado em runtime.go:333, imediatamente após a transição para `StateInitializing`, mas ANTES de:
- `lifecycle.ExecuteInit()` (linha 336) — init hooks
- `lifecycle.ExecuteStart()` (linha 347) — start subsystems
- Health check loop (linha 359)

O evento semanticamente significa "startup completo" mas é publicado no início do startup.

### N2 — Causa Arquitetural
O event bus não tem contrato de "ready state". Eventos são publicados sem garantia de que o estado alvo foi atingido. A arquitetura de eventos permite que publishers emitam eventos em qualquer ponto do ciclo de vida, sem validação de pré-condições.

### N3 — Causa de Processo
- Nenhum teste verifica o estado do runtime no momento em que `EventStartupComplete` é recebido
- O teste `TestPublishAllEventTypes` (eventbus_test.go:117) verifica apenas que o evento é publicável, não que é publicado no momento correto
- Review de código do `Start()` não questionou o posicionamento do Publish

### N4 — Prevenção Sistêmica
- **CI gate:** Integration test que subscreve a eventos de lifecycle e verifica que o estado do runtime é o esperado no momento do evento
- **Event contract:** Documentar para cada evento qual o estado esperado do runtime (ex: `EventStartupComplete` → runtime deve estar em `StateRunning`)
- **Code review checklist:** Verificar posicionamento de `Publish()` relativo a transições de estado

## Detection Pattern
```bash
grep -B5 -A5 "EventStartupComplete" internal/runtime/runtime.go
```
O Publish está na linha 333, entre `TransitionTo(StateInitializing)` (329) e `ExecuteInit` (336).

## Fix Expected
Mover o `Publish(EventStartupComplete)` para depois de `ExecuteStart()` e health check loop, quando o runtime está efetivamente em `StateRunning`:
```go
// After ExecuteStart and health check loop start:
r.events.Publish(ctx, EventStartupComplete, "runtime", nil)
```

## Affected Files
- `internal/runtime/runtime.go:317-365` (Start function)
