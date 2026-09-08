# cosca-specialist-backend-api - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-specialist-backend-api — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-api |
| **Task** | Initial capability establishment |
| **Technique** | Standard backend-api patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #backend-api #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core backend-api patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — gRPC Auth Interceptor (P0)
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-api |
| **Task** | Implement gRPC Auth Interceptor |
| **Technique** | gRPC unary interceptor pattern with JWT validation |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #backend-api #grpc #auth #interceptor #jwt |
| **Related** | api/grpcserver/interceptors.go, api/grpcserver/server.go, api/auth/oidc.go, internal/auth/jwt.go |
| **Learned** | 1. `New()` interceptor chain order: Recovery → Auth → Logging → extras. 2. `Config.JWTSecret []byte` drives auth enabling/disabling (nil/empty = dev mode passthrough). 3. `auth.ContextKeyClaims` (type `contextKey`) stores parsed `*internalauth.Claims` in gRPC context — compatible with `auth.ClaimsFromContext()` for downstream handlers. 4. gRPC metadata key is lowercased "authorization" automatically by the framework. 5. Tests use `metadata.NewIncomingContext` and `metadata.AppendToOutgoingContext` for auth header injection; `grpc.UnaryServerInfo` provides the method name to the interceptor. 6. Integration test pattern: bufconn server with full interceptor chain, status call with/without auth header proves end-to-end auth enforcement. |
| **Next** | Extend with rate-limiting or per-method RBAC gRPC interceptor |

