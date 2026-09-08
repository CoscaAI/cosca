# cosca-specialist-review-code - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-specialist-review-code — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-review-code |
| **Task** | Initial capability establishment |
| **Technique** | Standard review-code patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #review-code #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core review-code patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — Onda 3 Specialist Review
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-review-code |
| **Task** | Review Wave 3 specialist code (gRPC, SSE, WebSocket, Prometheus, Workflows, Confidence Tracker, Frontend) |
| **Technique** | Multi-layer review: security → correctness → Go idioms → architecture → performance |
| **Level** | 2 |
| **Outcome** | 2 critical, 3 high, 5 medium, 4 low issues found |
| **Tags** | #review-code #onda-3 #gRPC #SSE #WebSocket #security #XSS |
| **Related** | .opencode/cosca/memory/review/onda-3-specialist-review.md |
| **Learned** | 1) WebSocket InsecureSkipVerify must be configurable (not hardcoded true). 2) `context.Background()` in HTTP handlers prevents cancellation propagation — always use `r.Context()`. 3) `dangerouslySetInnerHTML` in React without sanitization is XSS — always sanitize HTML from any source (even AI output). 4) TTL parse errors should be surfaced to the client, not silently ignored. 5) `if/else` with identical branches is dead code — remove or fix. |
| **Next** | Level 3: Automated review with custom linter rules for project-specific patterns (context propagation, error silencing) |

