# cosca-testing — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-testing |
| **Task** | Initial capability establishment |
| **Technique** | Standard testing patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #testing #baseline #initialization |
| **Related** | See .opencode/cosca/memory/codebase/overview.md, .opencode/cosca/memory/pattern/ |
| **Learned** | Project established. Core testing patterns documented. Ready for level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

---

## Real Execution Learnings

### 2026-07-28 — Runtime Integration Suite (Level 3)

| Field | Value |
|-------|-------|
| **Agent** | cosca-testing |
| **Task** | Write integration suite for runtime state machine (20 transitions) + validate 3 bugs |
| **Technique** | AAA pattern, table-driven transition catalog, concurrent stress testing, bug reproduction |
| **Level** | 3 |
| **Outcome** | success |
| **Confidence Score** | **≥ 0.40** (up from 0.25 baseline) — primary domain: integration testing |
| **Tags** | #testing #integration #runtime #state-machine #bug-reproduction #race-detection |
| **Related** | `internal/runtime/state_integration_test.go`, `internal/runtime/runtime_lifecycle_test.go`, `.opencode/cosca/memory/testing/integration-report.md` |

**Key Learnings**:

1. **State machine testing requires transition graph coverage, not just happy paths.** The `validTransitions` map is the source of truth — every edge must be tested. Missing even one transition (like `Stopped → Uninitialized`) creates hidden bugs because the code path that should trigger it doesn't exist.

2. **Bug reproduction is the most valuable testing activity.** Reproducing BUG-U01 (Restart broken) revealed that the state machine's `validTransitions` map is correct but the code never executes the `Stopped → Uninitialized` transition. Without an integration test, this would have remained invisible.

3. **Event timing bugs require explicit subscriber-based verification.** BUG-U02 (EventStartupComplete premature) was confirmed by a test that subscribes to the event AND registers an init hook. The test proves the event fires before the hook executes — something unit tests can't detect.

4. **Mutex ordering in callback chains.** The `TransitionTo` method holds `rs.mu` while firing the `onChange` callback. If the callback tries to acquire a lock that the test already holds, deadlock occurs. Pattern: **never hold a lock across a transition call** when testing callbacks.

5. **Parallel testing with Runtime needs goroutine cleanup.** The `healthCheckLoop` goroutine created by `Runtime.Start()` persists until `Stop()` is called. Tests must pair every `Start()` with a `Stop()`. In parallel test suites, leaked goroutines from one test can interfere with others under `-race`.

6. **Metrics enumeration as test data.** BUG-U03 (metrics doc gap) was validated by enumerating all `MetricsSnapshot` fields in the test itself — the test serves as a living specification of what metrics exist.

**Technique Mastery**: Successfully applied AAA pattern across 30 test functions with 0 shared mutable state. The table-driven transition catalog pattern (22 sub-tests in a single table) proved highly maintainable and complete.

**Confidence Model Update**:
- **Primary Domain** (Integration Testing): confidence **0.40** (up from 0.25)
- **Secondary Domain** (Race Detection): confidence **0.35** (newly acquired)
- **Secondary Domain** (Bug Reproduction): confidence **0.45** (newly acquired)
- **Growth Area** (E2E Testing): still 0.05 — no E2E work done

**Next**: Level 4 — E2E test suite for critical user journeys, soak tests (>1h), CI pipeline integration with `-race` gate.
