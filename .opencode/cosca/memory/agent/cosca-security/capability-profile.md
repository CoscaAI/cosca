# cosca-security — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-08-29

## Current Level: 3

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Auth & JWT | 0.90 | 2 | success | ↑ |
| Security Headers (CSP, HSTS, etc.) | 0.88 | 2 | success | ↑ |
| RBAC & Middleware | 0.85 | 1 | success | ↑ |
| CSRF Protection | 0.80 | 1 | success | → |
| Rate Limiting | 0.80 | 1 | success | → |
| Password Hashing (bcrypt) | 0.80 | 1 | success | → |
| Compliance Audit (GDPR/SOC2) | 0.75 | 1 | success | → |
| OWASP Top 10 | 0.90 | 2 | success | ↑ |
| Public endpoint / data exposure audit | 0.88 | /brain audit (A1–A8) | success | ↑ |
| Prompt injection / trust boundary | 0.85 | CLI V1–V13 + /brain CSP | success | ↑ |
| Threat modeling (Windows/jail, air-gap) | 0.78 | Cofre threat model | success | ↑ |
| Automated Scanning (govulncheck) | 0.20 | 0 | — | → |
| Token Revocation | 0.10 | 0 | — | → |

## Strengths
- **Multi-layered security audit**: Can perform full-stack audits spanning JWT, bcrypt, CSRF, rate limiting, RBAC, security headers, CSP, public-endpoint exposure, and prompt-injection/trust-boundary analysis in a single pass — validated against 357 Go source files.
- **Threat modeling in practice**: Has produced structured threat models for the Windows jail (fail-closed vs fail-open), the "Cofre" air-gap (2-zone), and the /brain public endpoints (A1–A8 with priority + recommendations) — moving beyond checklist-only to attack-chain reasoning.
- **Prompt-leak / data-exposure fixing**: Diagnosed and implemented the P1 fix that removed user-prompt leakage — renaming `Prompt` → safe `Action` label in the activity log so sensitive args never persist/vazam to the public /brain.
- **Integrity-plus-security audits**: Uses `PRAGMA integrity_check`/`foreign_key_check` (via DB chief) and code review to separate real corruption from by-design artifacts (NULL embeddings, dedup hashes).
- **Mining with a security lens**: Extracts config patterns with an explicit I7/I8 + license verdict (codesight-mcp MIT vs cie AGPL), and flags fail-open anti-patterns (AES-CBC zero-key, XFF trust) to be avoided in Cosca.
- **Honest finding prioritization**: Always distinguishes P0/P1/P2 and, when a fix isn't authorized (Doc-only), reports cleanly instead of "fixing" others' concurrent refactors.

## Weaknesses
- **No automated vulnerability scanning**: Has not integrated govulncheck, gosec, or semgrep into CI/CD pipeline — all audits are manual.
- **No token lifecycle management**: Has documented the absence of token revocation lists and MFA — but has not yet designed implementations (Token Revocation confidence 0.10).
- **Windows native isolator unbuilt**: The AppContainer + Job Object + Low-IL confinement and execpolicy shell-operator parser were recommended but are P1/P2 and not yet implemented (threat model is complete, enforcement pending).

## Preferred Strategies
- **OWASP Top 10 checklist with Go-specific guidance**: Maps each OWASP category to Cosca's Go/SQLite/Next.js stack with concrete code patterns.
- **Memory-to-codebase reality audit**: Always verifies compliance and architecture claims in memory against actual source code before acting — prevents aspirational fiction drift.
- **Attack-chain (STRIDE-style) decomposition**: Decomposes a subsystem by trust boundary and data flow (recon → traversal → CSP → exposure), produces a numbered finding list with priority, then a separate recommendations list.
- **Fix = minimal surface, verified by the real path**: Implement the minimal change (`Prompt`→`Action`), confirm the consumer (app.js only reads `a.agent`), validate via `go build`+`go test`, and never "fix" a concurrent agent's refactor.

## Known Failure Modes
- **Concurrent refactor races**: multiple agents editing the same file (server.go readActivityLog) introduced compile errors that were not mine — lesson: never assume file stability, don't fix others' bugs, just report test results honestly.
- **Trusting the README's security claims**: docs claimed "fail-closed preserved" while launchers injected `COSCA_ALLOW_NO_ROOT=1` by default — mitigated by verifying the actual launcher/opt-in path (install-service.ps1, cosca-serve.bat).

## Evolution Goal
Reach Level 4:
*"Integrate govulncheck/gosec/semgrep into CI, implement token revocation list with audit log signatures, and prototype a Windows native isolator (AppContainer + Job Object + Low-IL + execpolicy shell-operator parser) — graduating from manual audit to an automated, always-on security posture plus a real Windows sandbox."*
