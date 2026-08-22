# Bug Registry

> **Category**: Failures → Bugs | **Version**: 1.0.0 | **Owner**: Cosca QA Chief | **Last Updated**: 2026-07-29

## Purpose

Systematic bug registry with 4-level causality trees. Every bug documents not just what broke (N1), but why the system allowed it (N2), what process failed (N3), and what prevents recurrence (N4).

## Causality Summary

| Bug | Severity | N1 — Direct Cause | N2 — Architectural | N3 — Process | N4 — Prevention |
|-----|:--------:|-------------------|--------------------|--------------|-----------------|
| [bug-001](bug-001-tmp-path.md) | medium | Hardcoded `/tmp/` paths in 3 files | No project temp directory abstraction | CI only tested Linux | CI matrix Linux+macOS+Windows; `forbidigo` lint rule |
| [bug-002](bug-002-lint-issues.md) | low | 959 accumulated lint issues across ~350 files | No static quality gate in build pipeline | `golangci-lint` existed but not in CI | `golangci-lint run` as required CI check; zero warnings policy |
| [bug-003](bug-003-race-conditions.md) | high | Goroutines accessing shared maps without synchronization | No thread-safety contracts for shared state | `go test -race` not in CI | `go test -race` mandatory in CI; document thread-safety |
| [bug-004](bug-004-provider-caching.md) | medium | Provider cache without TTL, invalidation, or health check | No Cache-Aside pattern with TTL; no config/instance separation | Tests only covered short sessions (<1 min) | 1h soak test with memory monitoring; TTL on all caches |
| [bug-005](bug-005-sqlite-first-run.md) | high | `panic()` when `schema_version` table doesn't exist on first run | No "zero state" concept in migration system | No fresh-install test; `panic()` in library code | Clean-state CI test; `panic()` banned in `internal/` |

## Active Bugs

| Bug | Severity | Status | Detected | Fix Commit |
|-----|:--------:|:------:|----------|------------|
| [bug-001](bug-001-tmp-path.md) | medium | ✅ Fixed | 2026-07-25 | `c30fac3` |
| [bug-002](bug-002-lint-issues.md) | low | ✅ Fixed | 2026-07-24 | `03860c2` |
| [bug-003](bug-003-race-conditions.md) | high | ✅ Fixed | 2026-07-20 | `9a950ff` |
| [bug-004](bug-004-provider-caching.md) | medium | ✅ Fixed | 2026-07-21 | `f3dbdc2` |
| [bug-005](bug-005-sqlite-first-run.md) | high | ✅ Fixed | 2026-07-24 | `0ffb4da` + `c30fac3` |
| [bug-006](bug-006-restart-broken.md) | blocker | 🔴 Open | 2026-07-28 | — |
| [bug-007](bug-007-startup-event-timing.md) | critical | 🔴 Open | 2026-07-28 | — |
| [bug-008](bug-008-metrics-misdocumented.md) | major | 🔴 Open | 2026-07-28 | — |

## Severity Distribution

| Severity | Count | Bugs |
|:--------:|------:|------|
| blocker | 1 | bug-006 |
| critical | 1 | bug-007 |
| major | 1 | bug-008 |
| high | 2 | bug-003, bug-005 |
| medium | 2 | bug-001, bug-004 |
| low | 1 | bug-002 |

## Template

New bugs must follow the Causality Tree format defined in [`CAUSALITY_TREE_TEMPLATE.md`](CAUSALITY_TREE_TEMPLATE.md) (v2.0.0). The template specifies:
- YAML frontmatter with type, key, severity, timestamp, fix commit
- 4-level causality tree (N1–N4) with quality criteria per level
- Detection pattern (commands to reproduce)
- Fix expected / fix applied
- Affected files

---

*"Formal bug registry underestimates real bugs — audit source code for completeness." — Heuristic H-012. Bugs 006–008 were discovered via code audit, not reported.*
