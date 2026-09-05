# cosca-specialist-backend-service — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-08-04

## Current Level: 2 (structured trust and immutable cache boundaries established)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Backend service implementation (business logic, domain services) | 0.86 | 9 | success | ↑ |

## Strengths
- Manager pattern implementation (Manager structs with injected dependencies and constructor wiring)
- Business logic encapsulation with data access in the same layer (no separate service/repository)
- Concurrency-safe code using sync.RWMutex, channels, and context.Context propagation

## Weaknesses
- Race-safe mutable statistics with JSON/API compatibility and concurrent test coverage
- Descriptor-level TOCTOU reduction remains a follow-up opportunity
- Raw file-descriptor ownership and idempotent cleanup now covered by a tested pattern
- Opt-in durable lifecycle integration with fencing, cancellation cleanup, and sensitive-data boundaries
- Conditional semantic routing integration across synchronous and streaming execution
- Threat-aware context construction with structured provenance and safe tool-output envelopes
- Immutable cache ownership boundaries and privacy-preserving diagnostics

## Preferred Strategies
- Follow the manager/store pattern: Managers own both business logic and data access
- Inject dependencies via constructor; wrap errors with context using fmt.Errorf("context: %w", err)
- Never panic() — return errors; use table-driven tests with testify (require/assert)
- Mock at interface boundaries or use in-memory SQLite for testing

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Maintain Level 2:
"Apply multi-layered threat and concurrency boundaries while preserving legacy contracts"
