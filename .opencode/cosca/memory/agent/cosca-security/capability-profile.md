# cosca-security — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 2

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Auth & JWT | 0.90 | 2 | success | ↑ |
| Security Headers (CSP, HSTS, etc.) | 0.85 | 1 | success | ↑ |
| RBAC & Middleware | 0.85 | 1 | success | ↑ |
| CSRF Protection | 0.80 | 1 | success | → |
| Rate Limiting | 0.80 | 1 | success | → |
| Password Hashing (bcrypt) | 0.80 | 1 | success | → |
| Compliance Audit (GDPR/SOC2) | 0.75 | 1 | success | → |
| OWASP Top 10 | 0.90 | 1 | success | ↑ |
| Automated Scanning (govulncheck) | 0.20 | 0 | — | → |
| STRIDE Threat Modeling | 0.15 | 0 | — | → |
| Token Revocation | 0.10 | 0 | — | → |

## Strengths
- **Multi-layered security audit**: Can perform full-stack audits spanning JWT, bcrypt, CSRF, rate limiting, RBAC, and security headers in a single pass — validated against 357 Go source files.
- **Codebase-to-memory cross-validation**: Detects when memory claims (e.g., compliance certifications) are aspirational rather than actual, and corrects them with concrete implementation steps.
- **Go-specific security analysis**: Understands Cosca's custom HMAC-SHA256 JWT implementation, crypto/subtle.ConstantTimeCompare for CSRF, token bucket rate limiting with fractional refill, and API key SHA-256 hash storage patterns.
- **Owning the full auth priority chain**: Has documented the complete 3-tier auth chain (API Key → Cookie JWT → Bearer token) with bcrypt cost 12 verification and brute-force lockout (5 attempts → 15min).
- **Compliance drift detection**: Pattern of identifying fabricated vs actual compliance status and rewriting with aspirational roadmaps (5 concrete steps: at-rest encryption, audit logging, data mapping, retention automation, right-to-erasure).

## Weaknesses
- **No automated vulnerability scanning**: Has not integrated govulncheck, gosec, or semgrep into CI/CD pipeline — all audits are manual.
- **No STRIDE threat model per subsystem**: Has not performed structured threat modeling; all audits are OWASP checklist-based rather than STRIDE decomposition.
- **No token lifecycle management**: Has documented the absence of token revocation lists, MFA, and audit log tamper detection — but has not yet designed implementations.

## Preferred Strategies
- **OWASP Top 10 checklist with Go-specific guidance**: Maps each OWASP category to Cosca's Go/SQLite/Next.js stack with concrete code patterns (e.g., "SQL injection impossible with parameterized queries").
- **Memory-to-codebase reality audit**: Always verifies compliance and architecture claims in memory files against actual source code before acting — prevents aspirational fiction drift.
- **Layered depth approach**: Audits security in priority order: auth chain first, then middleware chain, then configuration headers, then peripheral concerns — ensuring critical paths are verified before details.

## Known Failure Modes
- None recorded — patterns.md is empty; all 4 learning entries show successful outcomes.

## Evolution Goal
Reach Level 3:
*"Integrate govulncheck/gosec/semgrep into CI, produce STRIDE threat model per subsystem (auth, knowledge, agents, plugins), and implement token revocation list with audit log digital signatures — graduating from manual audit to automated security posture with structured threat modeling."*
