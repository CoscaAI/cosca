# OWASP Top 10 (2021) Security Review

> **Date:** 2026-07-27
> **Reviewer:** Security Chief
> **Methodology:** STRIDE per subsystem, CVSS scoring for findings

---

## A01: Broken Access Control
- **Status:** ✅ Mitigated
- JWT authentication enforced on all protected endpoints via `api/auth.Middleware`
- RBAC middleware at `api/auth.RequireRole()` — three roles: admin, editor, viewer
- httpOnly cookies prevent token theft via XSS (access/refresh tokens)
- API key authentication for programmatic access with scoped roles
- CSRF double-submit cookie pattern on all mutating requests
- No Insecure Direct Object Reference (IDOR) — user IDs come from JWT claims, not URL params
- **CVSS:** N/A (no findings)

## A02: Cryptographic Failures
- **Status:** ✅ Mitigated
- bcrypt cost factor 12 for password hashing (`internal/auth/users.go`)
- AES-256-GCM for secrets vault encryption (`internal/secrets/vault.go`)
- JWT HS256 with enforced strong secret — no hardcoded fallback, panics if env var missing
- `go.sum` integrity verification for all Go modules
- **CVSS:** N/A (no findings)

## A03: Injection
- **Status:** ✅ Mitigated
- SQLite parameterized queries throughout — no string concatenation of user input in SQL
  - Internal packages use `fmt.Sprintf` for table *names*, but these come from hardcoded app config, not user input
  - All user-supplied values go through `?` placeholders
- Input length limits enforced on all user-facing fields:
  - Prompt: max 10,000 characters (`api/rest/handler/run.go`)
  - Username: max 50 characters (`api/rest/handler/auth.go`, `api/rest/handler/users.go`)
  - API key name: max 100 characters (`api/rest/handler/apikeys.go`)
  - Secret key: max 100 characters (`api/rest/handler/secrets.go`)
- HTML escaping (`html.EscapeString`) applied to user-provided content before API response
- No command injection — no shell execution with user input
- FTS5 queries sanitized via `internal/sqlite.SanitizeFTSQuery()`
- **CVSS:** N/A (no findings)

## A04: Insecure Design
- **Status:** ✅ Mitigated
- Rate limiting on all endpoints (100 req/min default, 5 req/min for login)
- CSRF protection on all mutating requests (double-submit cookie pattern)
- Audit logging for all sensitive operations (login, user CRUD, API key ops, secrets)
- Health/readiness probes with subsystem checks for Kubernetes
- Graceful shutdown with context cancellation
- **CVSS:** N/A (no findings)

## A05: Security Misconfiguration
- **Status:** ✅ Mitigated
- CSP headers configured on both Next.js frontend and Go API middleware
  - Known tradeoff: `'unsafe-inline'` and `'unsafe-eval'` required for Next.js development
- Security headers: X-Frame-Options: DENY, X-Content-Type-Options: nosniff, Referrer-Policy: strict-origin-when-cross-origin, Permissions-Policy: camera=(), microphone=(), geolocation=()
- HSTS: max-age=31536000; includeSubDomains (conditional on HTTPS)
- CORS origins explicit — no wildcard `*` in production
- No default credentials — admin user removed, configuration requires env vars
- **CVSS:** N/A (no findings)

## A06: Vulnerable Components
- **Status:** ⚠️ Monitor
- Go dependencies checked via `go vet` and `govulncheck`
- NPM dependencies checked via `npm audit`
- Go 1.22 — current stable release, receives security patches
- Next.js 15 — current stable release
- **Action items:**
  - [ ] Set up automated `govulncheck` in CI pipeline
  - [ ] Set up Dependabot or Renovate for automated dependency updates
  - [ ] Review all dependencies quarterly
- **CVSS:** N/A (monitoring)

## A07: Authentication Failures
- **Status:** ✅ Mitigated
- bcrypt cost 12 password hashing
- Rate limiting on login endpoint: 5 requests per minute per IP (brute force protection)
- No hardcoded credentials — admin init requires explicit `--admin-password` flag or env var
- JWT secrets enforced — app panics on startup if `COSCA_JWT_SECRET` is missing
- Token expiry: 24h access, 7d refresh with rotation
- httpOnly cookies with SameSite=Strict prevent token theft and CSRF-based auth attacks
- **CVSS:** N/A (no findings)

## A08: Software and Data Integrity Failures
- **Status:** ✅ Mitigated
- Go modules with `go.sum` checksum verification
- `pnpm-lock.yaml` for frontend dependency locking
- Atomic writes for SQLite databases (WAL mode)
- WASM plugin runtime uses checksum verification (SHA-256) before execution
- **CVSS:** N/A (no findings)

## A09: Security Logging and Monitoring Failures
- **Status:** ✅ Mitigated
- Audit logs for all auth events: login (success/denied), user CRUD, API key operations, secret vault access
- Structured logging via `zerolog` with request context (method, path, status, duration, remote_addr)
- Execution history persisted for AI orchestration runs
- Log levels: server errors (500), client errors (400), success (200) all logged
- No secrets or passwords in log output
- **CVSS:** N/A (no findings)

## A10: Server-Side Request Forgery (SSRF)
- **Status:** ✅ Mitigated
- No user-controlled URLs in backend HTTP requests
- Provider endpoints (LLM APIs) are predefined in `internal/providers/` — not user-supplied
- Chat registry validates model names against known providers
- **CVSS:** N/A (no findings)

---

## Summary

| Category | Status | Action Required |
|---|---|---|
| A01: Broken Access Control | ✅ Mitigated | None |
| A02: Cryptographic Failures | ✅ Mitigated | None |
| A03: Injection | ✅ Mitigated | Monitor input validation |
| A04: Insecure Design | ✅ Mitigated | None |
| A05: Security Misconfiguration | ✅ Mitigated | Review CSP for production |
| A06: Vulnerable Components | ⚠️ Monitor | Set up automated scanning |
| A07: Authentication Failures | ✅ Mitigated | None |
| A08: Software & Data Integrity | ✅ Mitigated | None |
| A09: Security Logging | ✅ Mitigated | None |
| A10: SSRF | ✅ Mitigated | None |

**Overall Score: 9/10 mitigated, 1 monitored**

**Next Review Due:** 2026-10-27 (quarterly)
