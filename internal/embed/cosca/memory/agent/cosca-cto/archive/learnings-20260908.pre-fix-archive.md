# cosca-cto - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

# cosca-cto — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-cto |
| **Task** | Initial capability establishment |
| **Technique** | Standard cto patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #cto #baseline #initialization |
| **Related** | internal/embed/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core cto patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — Activation Audit: Stack, Architecture, Tech Debt
| Field | Value |
|-------|-------|
| **Agent** | cosca-cto |
| **Task** | Full technical audit: Stack Review, Architecture Scan, Tech Debt Assessment |
| **Technique** | Multi-dimensional audit: dependency analysis × codebase traversal × risk scoring |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #cto #audit #activation #tech-debt #architecture #stack-review |
| **Related** | /home/henrique/documents/projects/CoscaAI/internal/embed/cosca/memory/cto/activation-report.md |
| **Learned** | (1) Project has strong foundation — Go 1.25, pure-SQLite, WASM runtime, Next.js 15 web console. REST/gRPC/MCP triple API surface is a maintainability multiplier. (2) Plugin sandbox has known gaps on Linux (Setrlimit affects parent, no cgroups) and is a no-op on non-Linux — documented gap requiring cgroups v2. (3) gRPC server lacks auth interceptors (JWT/API key/CSRF) present in REST, creating asymmetric security. (4) Coverage threshold at 40% in Makefile vs 70% in CI docs indicates process gap. |
| **Next** | Drive P0 remediation: (1) Plugin sandbox cgroups v2, (2) gRPC auth interceptors, (3) API handler abstraction layer |

