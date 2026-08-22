# cosca-testing — Negative Memory (Failures)

> Auto-evolution memory. Failures are the most valuable teachers. Search before acting.

## Active Failures

### 2026-07-28 — Deadlock: Callback Mutex Ordering in Transition Tests

| Field | Value |
|-------|-------|
| **Agent** | cosca-testing |
| **Task** | Write `TestStateMachine_OnChangeCallback` — state change event validation |
| **Severity** | Minor (test infrastructure only, not production) |
| **Outcome** | Fixed |
| **Tags** | #deadlock #mutex #callback #testing |
| **Related File** | `internal/runtime/state_integration_test.go:561-622` |

**What Happened**:

The test registered an `OnStateChange` callback that acquired a local `sync.Mutex`. After three successful transitions, the test held the mutex (`mu.Lock()` / `defer mu.Unlock()`) while calling a fourth `TransitionTo`. The `TransitionTo` method fires the callback synchronously while holding `rs.mu`. The callback tries to acquire the same `mu` the test already holds → deadlock.

**Root Cause**:
```go
// BUG: mu is held here via defer mu.Unlock()
mu.Lock()
defer mu.Unlock()
// ... assertions ...
// BANG: TransitionTo fires callback which tries mu.Lock() → deadlock
_ = rs.TransitionTo(StateInitializing, "invalid back")
```

**Fix Applied**:
- Release `mu` before the fourth `TransitionTo` call
- Re-acquire after to read results
- Document lock ordering constraint in comments

**Lesson Learned**:
`TransitionTo()` holds an internal mutex (`rs.mu`) and synchronously invokes `onChange` callback. The callback runs while `rs.mu` is held. If the callback needs its own lock and the test also needs that lock, the test must **release its lock before calling TransitionTo**. Pattern: never hold a lock across a function that fires callbacks.

**Prevention**: Added comment in test code documenting the constraint. All future callback-based tests should follow the pattern: `UnlockBeforeTransition → Transition → LockAfterTransition`.

---

### 2026-07-28 — Bug Confirmed: Restart() Self-Healing Broken (BUG-U01)

| Field | Value |
|-------|-------|
| **Agent** | cosca-testing |
| **Task** | Reproduce Restart() bug |
| **Severity** | Blocker (production path) |
| **Outcome** | Confirmed — not fixed (documenting as expected failure) |
| **Tags** | #bug #runtime #restart #state-machine #production |
| **Related** | BUG-U01 in `internal/embed/cosca/memory/technical-debt/scorecard.md` |

**Reproduction**: `TestBugU01_RestartBroken` in `internal/runtime/runtime_lifecycle_test.go`

**Confirmed Behavior**:
- `Restart()` calls `Stop()` → state = `Stopped`
- `Restart()` calls `Start()` → `Start()` requires `state == Uninitialized`
- `Start()` returns: `"runtime already started (state: stopped)"`
- Self-healing completely non-functional

**Why This Wasn't Caught Before**: The `validTransitions` map correctly defines `Stopped → Uninitialized`, creating a false sense of correctness. Unit tests validated transitions *in isolation* but never tested the actual `Restart()` code path that orchestrates multiple transitions.

---

### 2026-07-28 — Bug Confirmed: EventStartupComplete Fires Before Init (BUG-U02)

| Field | Value |
|-------|-------|
| **Agent** | cosca-testing |
| **Task** | Reproduce EventStartupComplete timing bug |
| **Severity** | Critical (cascading startup failures) |
| **Outcome** | Confirmed — not fixed (documenting as expected failure) |
| **Tags** | #bug #runtime #event #startup #timing |
| **Related** | BUG-U02 in `internal/embed/cosca/memory/technical-debt/scorecard.md` |

**Reproduction**: `TestBugU02_EventStartupCompletePremature` + `TestBugU02_SubscribersGetNilSubsystems`

**Confirmed Behavior**:
- `EventStartupComplete` published at `runtime.go:333` during `StateInitializing`
- `lifecycle.ExecuteInit()` runs at `runtime.go:336` — *after* the event
- Subscribers receive "startup complete" before subsystems are initialized
- Subscribers accessing subsystems during the event get `nil`

**Why This Wasn't Caught Before**: The bug is a simple line-ordering issue (3 lines apart). Unit tests of `Start()` pass because they don't check event timing relative to init hooks. Only an integration test with subscribed handlers can detect the ordering inversion.

---

## Historical Failures

*Execution history: 2 tasks completed (1 seed baseline, 1 integration suite). 1 test infrastructure bug self-fixed. 2 production bugs confirmed.*

### 2026-08-21 — Frente B (12 pacotes) — Task completed without failures

| Field | Value |
|-------|-------|
| **Agent** | cosca-testing |
| **Task** | Portar 12 pacotes de filesystem/path Ubuntu→Windows (testes passando ou skip legítimo) |
| **Outcome** | Nenhuma falha do agente — 2 bugs reais de produção corrigidos, 8 testes corrigidos, 5 skips justificados |
| **Tags** | #failure #learned #windows #port |
| **Related** | learnings.md L010 ba74120e |

**Observação**: nenhuma abordagem falhou nesta task. O triage (produção → teste → skip) funcionou; os únicos riscos eram mudar comportamento Linux (mitigado com validação `GOOS=linux go build` + testes) e skip em massa (evitado — apenas 5 skips pontuais justificados de 31 falhas).

---

> **Protocol**: [LEARNING_PROTOCOL.md](../../LEARNING_PROTOCOL.md) | **Constitution**: P5 — A família aprende com erros
