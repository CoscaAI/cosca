# Cosca Bootstrap Lifecycle

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Bootstrap

## Lifecycle States

```
INACTIVE
    │
    ▼ (OpenCode starts)
INITIALIZING
    │
    ├── Phase 0: System Check
    ├── Phase 1: Discovery
    ├── Phase 2: Context
    ├── Phase 3: Memory
    ├── Phase 4: Skills
    ├── Phase 5: Agents
    ├── Phase 6: Classification
    ├── Phase 7: Activation
    ├── Phase 8: Report
    ├── Phase 9: Validation
    │
    ├──► READY (all phases pass)
    │
    ├──► DEGRADED (non-critical failures)
    │
    └──► FAILED (critical failures)
            │
            ▼
         RETRY (max 3)
            │
            ├──► READY
            └──► FAILED → REPORT TO USER
```

## State Transitions

| From | To | Trigger |
|------|-----|---------|
| INACTIVE | INITIALIZING | OpenCode session started |
| INITIALIZING | READY | All phases completed successfully |
| INITIALIZING | DEGRADED | Non-critical component unavailable |
| INITIALIZING | FAILED | Critical component (Kernel, Skills Engine) unavailable |
| FAILED | INITIALIZING | Retry triggered (max 3 attempts) |
| FAILED | INACTIVE | Max retries exhausted, report to user |
| DEGRADED | READY | Degraded — continue with available components |

## Phase Durations (expected)

| Phase | Duration | Description |
|-------|----------|-------------|
| 0 — Health Check | < 1s | File existence checks |
| 1 — Discovery | 1-5s | Filesystem scan for tech stack |
| 2 — Context | < 2s | Generate .cosca/ files |
| 3 — Memory | < 3s | Load/initialize memories |
| 4 — Skills | < 1s | Verify skill availability |
| 5 — Agents | < 1s | Map agent registry |
| 6 — Classification | < 1s | Classify project type |
| 7 — Activation | < 1s | Select active chiefs |
| 8 — Report | < 2s | Generate bootstrap report |
| 9 — Validation | < 1s | Run Gate 0 checks |
| 10 — Handover | < 1s | Transfer to Kernel |
| **TOTAL** | **< 18s** | Full bootstrap cycle |

## Session Types

| Type | Bootstrap Behavior |
|------|-------------------|
| **First Session** | Full bootstrap: create .cosca/, generate all context, initialize memory |
| **Subsequent Session** | Quick bootstrap: verify .cosca/ exists, update context, load memories |
| **New Project (empty dir)** | Minimal bootstrap: create .cosca/, mark as uninitialized, suggest /init |
| **Non-Cosca Project** | Discovery-only: detect stack, report, do not create .cosca/ unless user confirms |

## Recovery Procedures

### Health Check Failure
1. Identify failed component
2. If critical (Kernel, Skills Engine): abort, report to user
3. If non-critical: log warning, continue in DEGRADED mode
4. Components recoverable: retry once after 1s delay

### Discovery Failure
1. If no language detected: mark as "unknown"
2. If partial detection: flag missing categories
3. Continue with available data — agents can still operate

### Context Creation Failure
1. If `.cosca/` creation fails (permissions): report to user
2. If file write fails (disk full): abort, report to user
3. If existing `.cosca/` corrupted: backup and recreate

### Memory Load Failure
1. If global memory unavailable: continue with empty memory
2. If project memory corrupted: recreate from context
3. Log all load failures for Learning Engine

## Cleanup

### On Session End
- Save bootstrap session data to `.cosca/memory/sessions/`
- Update `state.yml` with session stats
- Clean temporary detection files
- Log session duration and metrics

### On Bootstrap Failure
- Preserve partial `.cosca/` structure
- Log detailed failure report
- Do not delete existing project files
- Suggest manual recovery commands

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Cosca Bootstrap | Initial lifecycle definition |
