# cosca-qa — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-08-29

## Current Level: 3

Achieved via: quality gates G0–G9 definition + bug-registry cross-validation, RAG fidelity gates ADR (reuse-first), and concrete regression test implementation for the read-only brain module.

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Quality gates G0–G9 definition | 0.80 | 1 | success | ↑ |
| Bug registry cross-validation (docs↔code) | 0.78 | 1 | success | ↑ |
| RAG fidelity / anti-hallucination gates (claims, grounding, qrels) | 0.75 | 1 | success | ↑ |
| Regression test authoring (table-driven AAA, go test) | 0.80 | 6 tests (brain regression) | success | ↑ |
| Security regression assertion (reflect guard against field reintroduction) | 0.78 | TestActivity_SemCampoPrompt | success | ↑ |
| E2E automated gate execution (CI) | 0.35 | 0 | — | → |

## Strengths
- **Verifiable acceptance criteria**: Replaces subjective criteria with objective thresholds — a core discipline applied to quality gates (G0–G9) and RAG fidelity gates.
- **Code-to-docs cross-validation**: Found 3 undocumented bugs by cross-referencing the bug registry with actual source (runtime.go, state.go, metrics.go) — not just trusting the registry.
- **Reuse-first test design**: Before writing, reads the real signatures (getActivityLog, Activity struct, handler mount, embed, observatory builder) and adapts defensively to concurrent production changes instead of asserting blindly.
- **Security regression + reflect guard**: Wrote `TestActivity_SemCampoPrompt` to catch reintroduction of the sensitive `Prompt` field — turning a past vulnerability into a durable guard.
- **Distinguishing production errors from test errors**: When `go build` failed due to a concurrent refactor (unused `io`, old Prompt field), correctly identified it as a production change, not a test failure, and did NOT "fix" prod code.

## Weaknesses
- **Cannot author production code**: QA observes/reports but does not modify production source — fixing bugs found is delegated (Coding cap delegated to Testing Chief / specialist).
- **No E2E automated gate execution yet**: Has defined G0–G5 CI gates + G6–G9 semi-auto, but hasn't validated them running through a real CI pipeline (confidence 0.35, no task).
- **Gap: `limit<=0`/fallback test in `readActivityLog`** — the `Recent` fallback to 30 is untested; acknowledged open item.

## Preferred Strategies
- **Read real signatures before writing tests**: Glob/read the actual structs/builders to author table-driven AAA tests that match the real behavior, not a guessed contract.
- **Guard regression with reflect**: For public payload projections, assert the field set is exactly the minimal safe projection (e.g., no `Prompt`).
- **Validate the real gate form (--check/--audit/--strict/--summary/--json)**: Mirrors the catalog gate contract for any new gate (e.g., `cosca gate recall`).
- **Separate build/lint errors from test errors**: Run `go build` on the package first before `go test` to isolate production-change failures from test-authoring failures.

## Known Failure Modes
- **Asserting a stale contract**: When production is refactored concurrently (Prompt→Action), a blind assert fails; lesson — verify the CURRENT struct/behavior, then adapt the assert to the correct post-fix behavior, per the task clause.
- **Production blockages interpreted as test failures**: compile errors from concurrent changes can read as "my tests broke" — mitigate by building the package in isolation and `-run`/`-count=1` per subtest.

## Evolution Goal
Reach Level 4:
*"Automate a QA bash/gate validator that cross-validates the bug registry against the codebase, integrate G0–G5 CI execution, and add flaky-test detection (1 fail in 10 runs) + coverage-diff enforcement — graduating from authored test cases to continuous, automated quality enforcement across the pipeline."*
