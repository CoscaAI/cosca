# cosca-specialist-backend-service — Negative Memory (Failures)

> Auto-evolution memory. Failures are the most valuable teachers. Search before acting.

## Active Failures

### F001 | 2026-08-04 | Non-idempotent memfd cleanup

The jail binary tests called the returned cleanup explicitly and through a
deferred cleanup. The second raw `close(2)` could close a newly allocated
descriptor (including one used by temporary-directory cleanup), producing
intermittent `bad file descriptor` errors under `-race`. Fix by making the
returned cleanup callback `sync.Once`-guarded; retain the expected ReadFile
failure after the first cleanup.

---
> **Protocol**: [LEARNING_PROTOCOL.md](../../LEARNING_PROTOCOL.md) | **Constitution**: P5 — A família aprende com erros
