# cosca-specialist-backend-service — Evolution Timeline

> Auto-evolution tracking.

## Current Level: 2

| Date | Level | Capability | Trigger |
|------|-------|------------|--------|
| 2026-07-27 | 1 | Baseline capabilities established | Initial audit |
| 2026-08-04 | 1 | Race-safe exported statistics with snapshot/JSON boundaries | Concurrent embedding stats hardening; 1/5 tasks toward Level 2 |
| 2026-08-04 | 1 | Sandbox, subprocess diagnostic, and atomic config hardening | P0 review remediation; 2/5 tasks toward Level 2 |
| 2026-08-04 | 1 | Idempotent raw FD cleanup with `sync.Once` | Memfd double-cleanup regression; 3/5 tasks toward Level 2 |
| 2026-08-04 | 1 | Injected opt-in durable workflow lifecycle with fencing and hash-only persistence | Durable ledger second slice; 4/5 tasks toward Level 2 |
| 2026-08-04 | 1 | Conditional semantic routing across Execute/ExecuteStream with CLI provider selection | SemanticRouter integration; 5/5 tasks toward Level 2 |
| 2026-08-04 | 1 | Cancellation-safe heartbeat shutdown and bounded durable finalization | Durable lifecycle cancellation hardening; 6 successful tasks, confidence 0.63 (level-up confidence threshold not yet met) |
| 2026-08-04 | 1 | Provenance-preserving untrusted context envelopes and quarantine exclusion | P1 prompt-injection/confused-deputy defense; 7 successful tasks, confidence 0.68 (threshold not yet met) |
| 2026-08-04 | 2 | Structured delimiter-safe envelopes, immutable embed cache boundaries, and privacy-safe orchestration diagnostics | Contenttrust review remediation; 8 successful tasks, confidence 0.81; Level 2 threshold reached |
| 2026-08-04 | 2 | External-by-default provenance, identity-checked cache expiry, sanitized diagnostics, and nil request guards | Backend hardening follow-up; 9 successful tasks, confidence 0.86; 1/10 successful Level 2 tasks toward Level 3 |
