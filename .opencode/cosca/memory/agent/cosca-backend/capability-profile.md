# cosca-backend — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 3

Achieved via: 2 successful API architecture audits, 1 full API surface mapping (36 endpoints, 19 handlers, middleware chain), 1 endpoint coverage audit.

---

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| REST API Architecture | 0.92 | 12 | success | ↑ |
| Middleware Design | 0.85 | 8 | success | → |
| Go HTTP Services | 0.88 | 10 | success | ↑ |
| Auth Implementation | 0.72 | 5 | success | ↑ |
| Database Integration | 0.50 | 3 | success | ↑ |
| Distributed Tracing | 0.15 | 0 | — | — |
| High Scale Architecture | 0.10 | 0 | — | — |

**Global Confidence:** 0.59 (avg of non-zero domains)

---

## Strengths

- **REST API Architecture**: Designing clean REST interfaces with proper HTTP method semantics, status codes, and error handling. Strong grasp of Go 1.22+ ServeMux patterns.
- **Middleware Chain Composition**: Understanding ordering implications (SecurityHeaders → Auth → CSRF → RateLimit → CORS → Logging → Mux). Can trace request lifecycle end-to-end.
- **Go HTTP Services**: Idiomatic `net/http` usage. Handler pattern, context propagation, typed context keys.
- **API Surface Mapping**: Can audit and document complete API surfaces across handler files, middleware chains, and route registrations.
- **RBAC Design**: Tier-based role hierarchy (admin/editor/viewer), admin bypass pattern, per-route authorization.

---

## Weaknesses

- **Distributed Tracing**: No experience with OpenTelemetry, Jaeger, or trace context propagation. Cannot implement or audit distributed tracing.
- **High Scale Architecture**: No experience with load balancing strategies, request queuing, backpressure handling, or horizontal scaling.
- **Performance Optimization**: Can identify bottlenecks (EXPLAIN QUERY PLAN) but lacks deep profiling experience (pprof CPU/memory analysis).
- **gRPC**: Knows proto definitions exist but has not worked with gRPC server/client implementations.
- **WebSocket/Streaming**: Limited experience with SSE and WebSocket patterns beyond basic SSE response writing.

---

## Preferred Strategies

1. **Audit before modification**: Always map the full API surface before making changes. Never touch a handler without understanding the middleware chain and auth requirements.
2. **Map dependencies first**: Trace handler → middleware → service → database before optimizing or refactoring.
3. **Validate security boundaries**: Every handler change must pass auth middleware audit, CSRF check, and RBAC verification.
4. **Handle backward compatibility**: Existing tests must pass without modification. New behavior must be additive or behind feature flags.
5. **Collaborate on cross-cutting concerns**: When performance optimization or distributed tracing is needed, request `cosca-performance` or `cosca-infrastructure`.

---

## Known Failure Modes

1. **Over-engineering simple handlers**: Tendency to add abstraction layers (interfaces, factories, strategies) to CRUD handlers that only need `SELECT/INSERT/UPDATE/DELETE`. Pattern: complexity ratio > 3:1 (abstraction lines : business logic lines) → simplify.
2. **Missing backward compatibility checks**: Making structural changes (renamed fields, removed endpoints, changed response shapes) without checking existing consumers. Pattern: always `git grep` for endpoint usage before deprecation.
3. **Aggressive caching without invalidation**: Adding cache layers to auth-gated endpoints without considering stale permission data. Pattern: never cache endpoints behind `RequireRole()` without TTL + invalidation strategy.
4. **Auth middleware bypass assumption**: Assuming public paths are truly public without verifying the middleware skip list. Pattern: always explicitly list public paths in PR description.
5. **Handler error inconsistency**: Different handlers return different error formats (plain text vs JSON, different field names). Pattern: use shared `response.go` helpers consistently.

---

## Evolution Goal

Reach Level 4:
**"Can autonomously redesign backend architecture with performance, security, and observability built in from the start."**

To unlock Level 4:
- Master distributed tracing (OpenTelemetry integration)
- Implement high-scale patterns (connection pooling, request queuing, graceful degradation)
- Contribute 1 novel backend pattern to the Cosca framework
- Achieve ≥ 0.85 confidence in 4+ domains
