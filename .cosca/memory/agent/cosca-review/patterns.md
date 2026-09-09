# cosca-review — Reusable Patterns

> Discovered patterns that can be reapplied. Grows with agent experience.

## Patterns Discovered

### P1: Bug Reproduction with Non-Blocking Tests (PATTERN TO ADOPT)
**Discovered**: 2026-07-28 | **Source**: Onda 2 Integration Tests
**Type**: Test strategy

**Problem**: Bugs known at review time should not break CI, but must be documented and tracked until fixed.

**Solution**: Write bug reproduction tests using `t.Logf()` instead of `t.Errorf()` / `t.Fatalf()`. The test documents: (a) the root cause, (b) exact code location with line numbers, (c) blast radius / impact, (d) recommended fix. The test name is prefixed with `TestBugU0X_` for easy grep. CI stays green. When the bug is fixed, the test is updated to assert success (or removed if the fix renders it obsolete).

**Example**: `TestBugU01_RestartBroken` in `runtime_lifecycle_test.go:130-163`. Confirms the bug with `t.Logf` and documents state after failure.

**Applicability**: Any project with known bugs that can't be fixed immediately but must not be forgotten.

---

### P2: Fire-and-Forget Event Before State Readiness (PATTERN TO AVOID)
**Discovered**: 2026-07-28 | **Source**: BUG-U02 (EventStartupComplete)
**Type**: Antipattern — Event Timing

**Problem**: Emitting lifecycle "completion" events BEFORE the corresponding lifecycle step executes causes subscribers to operate with inconsistent state.

**How to spot**:
1. Event publish (`Publish(EventXxxComplete)`) appears before the function that makes Xxx complete (`ExecuteInit()`, `ExecuteStart()`)
2. Subscribers receive event with `nil` dependencies (subsystems not yet initialized)
3. State at event time != expected "complete" state

**Root cause**: Inverted order — should be: Execute step → Transition state → Emit event. The bug has: Transition state → Emit event → Execute step.

**Fix pattern**: Move event emission to AFTER the lifecycle step completes AND after the state transition to the target state. In `runtime.go`, move `r.events.Publish(ctx, EventStartupComplete, ...)` from line 333 to after line 355 (after `ExecuteStart` and `StateRunning` transition).

**Prevention**: Audit all events with regex `Publish.*Complete` and verify they appear AFTER the corresponding execution step.

---

### P3: State Machine Definition Without Contract Enforcement (PATTERN TO AVOID)
**Discovered**: 2026-07-28 | **Source**: BUG-U01 (Restart broken)
**Type**: Antipattern — State Machine Design

**Problem**: A well-defined `validTransitions` map (declarative contract) is not enforced at the sequence level. Individual `TransitionTo()` calls validate single transitions, but multi-step sequences (like `Restart()`) bypass intermediate transitions that are required by the contract.

**How to spot**:
1. A state machine with `validTransitions` map exists.
2. A public method composes multiple transitions manually.
3. One of the required intermediate transitions (defined in the map) is never called within the method.
4. No compile-time or runtime check for sequence validity.

**Root cause**: The state machine is "declarative" (map defines valid edges) but the usage is "imperative" (callers compose transitions manually). The gap = no enforcement at the sequence level.

**Fix pattern**: Create `TransitionSequence` methods that encapsulate multi-step sequences:
```go
func (r *Runtime) RestartSequence(ctx context.Context) error {
    if err := r.Stop(ctx); err != nil { return err }
    if err := r.state.TransitionTo(StateUninitialized, "restart"); err != nil { return err }
    return r.Start(ctx)
}
```

**Prevention**: For every multi-step method that composes transitions, verify that each intermediate transition in the validTransitions graph is executed. Consider adding a linter rule or runtime assertion.

---

### P4: CI Gate Without Enforcement (PATTERN TO AVOID)
**Discovered**: 2026-07-28 | **Source**: CI-001 (coverage gate)
**Type**: Antipattern — CI Configuration

**Problem**: A quality gate is defined in documentation and implemented in CI, but the CI step uses `continue-on-error: true` or `|| true`, rendering the gate a no-op.

**How to spot**:
1. Quality gate defined with threshold (e.g., "coverage ≥ 70%")
2. CI step that checks the gate
3. `continue-on-error: true` or `|| true` in the CI step
4. Gate never actually fails the build

**Impact**: False confidence. Teams see "G5: PASS" in CI but the actual metric is below threshold. Regressions go undetected.

**Fix pattern**: Either: (a) make the gate truly blocking and lower the threshold to the actual baseline, or (b) create a technical debt ticket with a deadline to reach the threshold and document that the gate is non-blocking temporarily.

**Prevention**: CI review checklist: for every gate step, verify `continue-on-error` is false or explicitly justified. Verify that the step can actually fail by temporarily lowering the threshold below the actual value and confirming it fails.
