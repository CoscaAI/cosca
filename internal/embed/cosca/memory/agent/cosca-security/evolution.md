# cosca-security — Evolution Timeline

> Auto-evolution tracking. Records capability level progression.

## Current Level: 3

## Evolution History

| Date | Level | Capability | Trigger |
|------|-------|------------|---------|
| 2026-07-27 | 1 | Baseline capabilities established | Initial audit |
| 2026-07-27 | 1 | OWASP Top 10 + JWT security baseline documented | Auth implementation review |
| 2026-07-28 | 2 | Multi-layered security audit: JWT, bcrypt, CSRF, rate limiting, RBAC, security headers | Documentation sync — Phase 1+2 |
| 2026-07-28 | 2 | Aspirational vs actual compliance audit | Memory compliance correction |
| 2026-08-22 | 3 | CLI semantic manipulation attack surface (V1–V13: prompt injection, trust boundary, content trust gaps) | Semantic manipulation security audit |
| 2026-08-24 | 3 | Windows jail fail-closed vs fail-open analysis + threat model + blindagem do Cofre (opt-in→default bug) | Windows jail audit |
| 2026-08-27 | 3 | Context7/Upstash deep security mining (multi-issuer JWT, AES fail-closed, out-of-band nudge) | Security mining (repo externo) |
| 2026-08-28 | 3 | codesight-mcp (MIT) + cie (AGPL) mining — data-plane security model, spotlighting, byte-offset, license verdict | Security/license mining |
| 2026-08-29 | 3 | /brain public endpoint audit: data exposure + traversal + CSP threat model (A1–A8) | Observatorio /brain audit |
| 2026-08-29 | 3 | P1 fixes applied: `Prompt`→`Action` redaction (prompt-leak fix) in graph.go + server.go + root.go readActivityLog/recordCommandActivity | Don-approved P1 implementation |
