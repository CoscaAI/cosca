# cosca-backend — Negative Memory (Failures)

> Auto-evolution memory. Failures are the most valuable teachers. Search before acting.

## Active Failures

### 2026-07-15 — Aggressive Caching on Auth-Gated Endpoints

| Field | Value |
|-------|-------|
| **Agent** | cosca-backend |
| **Task** | Optimize API performance for user-facing dashboard endpoints |
| **Failed Approach** | Added aggressive in-memory caching layer (`internal/cache/cache.go`) with 60s TTL to `/v1/users`, `/v1/api-keys`, `/v1/secrets` — endpoints behind `RequireRole(RoleAdmin)` |
| **Root Cause** | Cached responses included RBAC-filtered data. When admin A's cached response was served to admin B, it contained stale authorization data. Cache key was `request.URL.Path` — did NOT include user identity. |
| **Consequence** | Authorization leak: admin B could see admin A's filtered results. Rolled back in commit `8f3a21c`. |
| **Lesson** | **Never cache permission-sensitive endpoints without identity-aware invalidation strategy.** Cache keys MUST include user identity (claims.Sub). Auth-gated endpoints should use short TTLs (< 5s) or no cache at all. |
| **Confidence Impact** | -0.15 (repeated failure pattern: also attempted in 2026-06-20 with session cache) |
| **Tags** | #failure #learned #caching #auth #rbac #performance #api |
| **Related Success** | learnings.md: 2026-07-20 — "Database-Level Query Optimization" (fixed the real bottleneck with indexes instead of caching) |
| **Avoidance Pattern** | Before adding cache: 1) Check if endpoint has `RequireRole()`, 2) If yes, ensure cache key includes `claims.Sub`, 3) Set TTL ≤ 5s, 4) Add cache-busting on user permission change |

---

### 2026-07-10 — Handler Error Format Inconsistency

| Field | Value |
|-------|-------|
| **Agent** | cosca-backend |
| **Task** | Standardize API error responses across all handlers |
| **Failed Approach** | Refactored error handling by adding a `RespondWithError(w, status, message)` function and migrated handlers one-by-one over multiple PRs |
| **Root Cause** | Migration was incomplete — 12 of 19 handlers used new format, 7 still used old format (`http.Error(w, msg, code)`). API consumers received inconsistent error shapes depending on which endpoint they called. |
| **Consequence** | Frontend error handling had to support BOTH `{"error": "msg"}` and plain text error formats. Doubled error handling code in API client. |
| **Lesson** | **Structural changes to shared contracts must be atomic.** API response format changes must be applied to ALL handlers in a single PR, verified by integration tests that check every endpoint's error response shape. |
| **Confidence Impact** | -0.10 |
| **Tags** | #failure #learned #api #error-handling #migration #consistency |
| **Related Success** | learnings.md: 2026-07-20 — "API Error Response Standardization" (completed atomic migration with contract tests) |
| **Avoidance Pattern** | Before changing response format: 1) Add test that validates response shape for ALL endpoints, 2) Apply change to all handlers in single commit, 3) Verify test passes before merging |

---

### 2026-06-20 — Premature Abstraction of CRUD Handlers

| Field | Value |
|-------|-------|
| **Agent** | cosca-backend |
| **Task** | Reduce code duplication across CRUD handler patterns |
| **Failed Approach** | Created generic `CRUDHandler[T]` interface with `List/Create/Get/Update/Delete` methods, `EntityStore[T]` abstraction, and `EntityValidator[T]` — applied to agents, skills, providers, and workflows |
| **Root Cause** | Each domain had subtly different requirements: agents needed capability listing, skills needed install/uninstall, providers needed test/validate, workflows needed run/cancel. The generic interface couldn't accommodate domain-specific operations. Ended up with `CRUDHandler` + 3-4 extension interfaces per domain — MORE complex than just writing domain-specific handlers. |
| **Consequence** | 847 lines of abstraction code for 4 domains that originally had ~200 lines each of direct handler code. Net increase in complexity. Reverted in commit `4b2e11d`. |
| **Lesson** | **Wait for 3+ instances before abstracting.** Don't generalize from 2-3 examples. CRUD handlers with different domain operations should NOT be forced into a shared interface. Domain-specific handlers with shared utility functions (response formatting, error handling, auth checks) achieve reuse without over-abstraction. |
| **Confidence Impact** | -0.10 |
| **Tags** | #failure #learned #abstraction #over-engineering #code-quality #pattern |
| **Related Success** | learnings.md: 2026-06-28 — "Shared Handler Utilities Pattern" (extracted `respondJSON`, `respondError`, `parseID` without forcing handler interface) |
| **Avoidance Pattern** | Before creating abstraction: 1) Count distinct domain operations per entity, 2) If > 3 entities have identical operation sets → abstract, 3) If operations differ → use shared utility functions instead, 4) Complexity of abstraction must be LESS than complexity of duplication |

---

## Avoided Failures (due to negative memory)

| Date | Pitfall Avoided | Referenced Failure | Task |
|------|----------------|-------------------|------|
| 2026-07-28 | Did NOT add caching to `/v1/secrets` endpoint | aggressive-caching-2026-07-15 | Documentation audit of auth endpoints |
| 2026-07-28 | Used `response.go` helpers consistently | handler-error-inconsistency-2026-07-10 | API surface documentation |
