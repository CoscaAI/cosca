# cosca-security — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — OWASP Top 10 Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Security audit baseline |
| **Technique** | OWASP Top 10 checklist — manual code review |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #security #owasp #baseline #code-review |
| **Related** | Broken Access Control, Cryptographic Failures, Injection, Insecure Design |
| **Learned** | All 10 categories mapped to Cosca codebase patterns. Go-specific: SQL injection impossible with parameterized queries. JWT HS256 baseline. |
| **Next** | Level 2: Integrate govulncheck automated scanning |

### 2026-07-27 — JWT Security Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Auth implementation review |
| **Technique** | JWT best practices — algorithm check, expiry validation, refresh rotation |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #jwt #auth #hs256 #token-rotation |
| **Related** | OWASP #2 Cryptographic Failures, OWASP #7 Auth Failures |
| **Learned** | Project uses HS256 with refresh tokens. Tokens stored in httpOnly cookies. RBAC enforced at middleware. |
| **Next** | Level 2: Add token revocation list, implement rate limiting on auth endpoints |

## Session: 2026-07-28 — Documentation Audit (Fase 1+2)

### 2026-07-28 — Full Auth Architecture Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Comprehensive auth + middleware security documentation |
| **Technique** | Level 2 — Multi-layered security audit: JWT implementation (custom HMAC-SHA256, no lib), bcrypt cost verification (12), brute-force analysis (5 attempts → 15min lockout), CSRF double-submit cookie audit (constant-time compare), rate limiting audit (token bucket: 5 RPM login / 100 RPM general), security headers audit (CSP, HSTS, X-Frame-Options), RBAC tier analysis (admin bypass pattern) |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #auth #jwt #middleware #csrf #rate-limiting #rbac #bcrypt #security-headers #documentation |
| **Related** | OWASP #2, #4, #7; docs/api-reference/auth.md, docs/api-reference/middleware.md |
| **Learned** | Full 3-tier auth priority chain documented: API Key (SHA-256 hash lookup) → Cookie (HttpOnly JWT) → Bearer token. Password hashing bcrypt cost 12 confirmed secure. CSRF uses crypto/subtle.ConstantTimeCompare — timing-attack resistant. Rate limiting uses per-IP token bucket with fractional refill. Security headers: CSP restricts connect-src, HSTS only on TLS. No OIDC/OAuth2 integration exists — documented as aspirational. API keys: 32-byte random → SHA-256 storage (plaintext shown once). Account lockout: check before bcrypt (CPU-saving). Known gaps: no token revocation list, no MFA, no audit log tamper detection. |
| **Next** | Level 3: STRIDE threat model per subsystem, implement govulncheck in CI, add token revocation list |

### 2026-07-28 — Memory Compliance Fix
| Field | Value |
|-------|-------|
| **Agent** | cosca-security |
| **Task** | Correct false compliance claims in memory |
| **Technique** | Level 2 — Aspirational vs actual audit: identified fabricated GDPR/SOC2/ISO 27001 compliance dates in memory, rewrote as aspirational with clear next steps |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #compliance #gdpr #soc2 #memory-audit #documentation |
| **Related** | memory/long/compliance-framework.md |
| **Learned** | Memory can drift into aspirational/fictitious claims. Pattern: always verify memory claims against codebase reality. Compliance framework marked as aspirational with 5 concrete implementation steps (at-rest encryption, audit logging, data mapping, retention automation, right-to-erasure). |
| **Next** | Level 3: Implement at-rest encryption for secrets, add audit log digital signatures |
