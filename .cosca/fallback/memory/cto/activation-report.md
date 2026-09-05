# CTO Activation Report — 2026-07-28

**Agent:** cosca-cto (Level 1→2)  
**Scope:** Stack Review, Architecture Scan, Tech Debt Assessment, P0/P1 Recommendations  
**Confidence:** 0.72  

---

## 1. Stack Review

### go.mod — Go 1.25 with clean dependency profile
- **Core stack:** `spf13/cobra` (CLI), `spf13/viper` (config), `rs/zerolog` (logging), `modernc.org/sqlite` (pure-Go SQLite, no CGO), `tetratelabs/wazero` (WASM), `google.golang.org/grpc` (gRPC), `nhooyr.io/websocket`
- **Strengths:** Zero CGO dependencies, modern Go version, curated dependency set (no bloat), WASM runtime using wazero (no host dependency)
- **Gaps:** No OpenTelemetry SDK (observability is zerolog-only), no circuit-breaker library, no caching library (Redis driver absent — acceptable for embedded)

### Dockerfile — Multi-stage, minimal scratch
- Builder: `golang:1.25-alpine` → Runtime: `scratch` (6.8 MB binary)
- Exposes 14120 (REST/WS) and 14121 (needs verification — MCP?). gRPC port 14122 is NOT exposed.
- **Issues:** Runs as root (no `USER` directive), no `HEALTHCHECK` in Dockerfile (only in docker-compose), no labels

### docker-compose.yml — Local dev only
- Cosca binary + Next.js frontend with health-dependent startup
- **Missing:** Reverse proxy (nginx/Caddy), TLS termination, proper env var documentation

### Makefile — Enterprise-grade but gap in coverage threshold
- 48 targets including build, cross-compile, test (unit/integration/e2e/bench/race), lint, proto, dev, embed-sync
- **Coverage threshold inconsistency:** Makefile checks 40%, CI docs say 70% — process gap

### .goreleaser.yaml — Solid but no Docker publishing
- Cross-platform: linux/darwin/windows × amd64/arm64
- No Homebrew/Scoop release automation, no Docker image in goreleaser (handled separately in CD workflow)

---

## 2. Architecture Scan

### Binary Architecture
```
cmd/cosca/main.go
  └─ internal/cli/        (65 files — CLI commands)
       ├─ root.go          (Cobra root)
       ├─ serve.go         (HTTP/gRPC server startup)
       ├─ agent.go/.go     (agent subcommands)
       ├─ ... 60+ command files
  └─ internal/            (43 packages, ~117K lines)
       ├─ runtime/         (lifecycle, daemon, state, eventbus)
       ├─ sqlite/          (DB, schema, migrations, FTS)
       ├─ knowledge/       (search engine)
       ├─ memory/          (multi-layer memory system)
       ├─ plugins/         (WASM plugin host + sandbox)
       ├─ workflows/       (workflow engine)
       ├─ agents/          (agent registry + manager)
       ├─ providers/       (LLM providers — OpenAI, Anthropic, Ollama, etc.)
       ├─ embeddings/      (embedding providers)
       ├─ vector/          (vector search with sqlite-vec)
       ├─ search/          (hybrid search — FTS + vector + graph)
       └─ ... (30+ more)
  ├─ api/                  (3 API surfaces)
  │    ├─ rest/            (52 endpoints, 16 domains)
  │    ├─ grpc/ + grpcserver/ (3 services: Runtime, Knowledge, Memory)
  │    ├─ mcp/             (14 JSON-RPC tools, stdin/stdout)
  │    ├─ auth/            (JWT + API key middleware)
  │    ├─ middleware/       (rate limit, CSRF, security headers)
  │    └─ stream/          (WebSocket hub)
  ├─ pkg/cosca/            (shared version/build info)
  ├─ web/                  (Next.js 15, React 19, Tailwind, 27 features)
  ├─ proto/aos/v1/         (3 .proto files)
  ├─ sdk/typescript/       (TypeScript SDK)
  └─ deploy/               (Helm chart, Terraform AWS, Prometheus config)
```

### Strengths
- Clean separation of concerns: `internal/` packages are well-bounded by domain
- Hexagonal-like architecture with `api/` as port layer and `internal/` as domain
- REST API uses Go 1.22+ routing patterns (`"POST /v1/knowledge/search"`)
- gRPC services map directly to proto definitions
- MCP server exposes same capabilities via JSON-RPC
- Web frontend is mature: 27 feature modules, Storybook, PWA, Playwright E2E

### Concerns
- **Triple API surface syndrome:** REST + gRPC + MCP each duplicate business logic wiring. Changes must be hand-replicated.
- **gRPC maturity gap:** Marked FASE 1/2/3, missing auth interceptors, no TLS by default, only basic recovery+logging interceptors
- **Monolith risk:** Single binary means any subsystem crash (plugins, knowledge, runtime) can take down everything

---

## 3. Tech Debt Assessment

### T1 — Plugin Sandbox Incomplete [RISK: P0] 🔴
| Aspect | Detail |
|--------|--------|
| **File** | `internal/plugins/sandbox_linux.go`, `sandbox_other.go` |
| **Issue** | Linux sandbox uses `Setrlimit` which affects the *parent* process (not just child). RLIMIT_AS (memory) is explicitly NOT set because of this. Non-Linux platforms have a NO-OP sandbox (no isolation at all). |
| **Impact** | A malicious or buggy WASM plugin can exhaust host memory/CPU, affecting the entire Cosca process. No isolation on macOS/Windows. |
| **Mitigation** | cgroups v2 for Linux, sandbox-exec for macOS, Job Objects for Windows. The code comment acknowledges this ("consider using cgroups v2 or a separate wrapper in a future release"). |
| **Cost if deferred** | Growing as more plugins are installed. Currently only WASM plugins exist, but the plugin system is expanding. |

### T2 — gRPC Security Asymmetry [RISK: P0] 🔴
| Aspect | Detail |
|--------|--------|
| **File** | `api/grpcserver/server.go` |
| **Issue** | REST API has full security stack: JWT middleware, API key auth, CSRF protection, rate limiting, security headers. gRPC only has recovery + logging interceptors. No auth, no rate limiting, no TLS. |
| **Impact** | gRPC port 14122 is an unauthenticated backdoor to Memory/Knowledge/Runtime services. |
| **Mitigation** | Add auth interceptor using same `internal/auth` package, implement TLS via `grpc.Creds()`, add rate limiting interceptor. |
| **Cost if deferred** | Production deployment with gRPC exposed creates a severe security gap. |

### T3 — Triple API Surface Maintainability [RISK: P1] 🟡
| Aspect | Detail |
|--------|--------|
| **Files** | `api/rest/server.go` (455 lines), `api/grpcserver/` (9 files), `api/mcp/server.go` (1360 lines) |
| **Issue** | Three independent implementations of similar logic. Adding a new capability (e.g., workflow execution) requires implementing it 3× with different frameworks. No shared handler/tool generation. |
| **Impact** | Each new feature costs 2-3× in implementation + maintenance. Inconsistent behavior between API surfaces is likely. |
| **Mitigation** | Generate REST handlers from proto service definitions using protoc plugins, or create a shared handler abstraction that REST/gRPC/MCP wrappers delegate to. |
| **Cost if deferred** | Grows linearly with feature count. Currently at 52 REST endpoints + 3 gRPC services + 14 MCP tools. |

### T4 — Coverage Threshold Inconsistency & Gaps [RISK: P1] 🟡
| Aspect | Detail |
|--------|--------|
| **Files** | `Makefile` (line 343: 40%), `.github/workflows/ci.yml` (line 11: 70%) |
| **Issue** | Makefile enforces 40% coverage threshold; CI documentation says 70%. Actual coverage likely between these values. 116K lines of Go, some packages (runtime, sqlite, plugins, workflows) well-tested, others thin. |
| **Impact** | Refactoring confidence is low for less-tested packages. Coverage gate ambiguity means CI could pass with different standards. |
| **Mitigation** | Align on 70% (CI standard), add package-level coverage reporting, enforce in CI using `go test -cover`. |
| **Cost if deferred** | As the codebase grows, coverage debt compounds. Refactoring becomes increasingly risky. |

### T5 — Monolithic Scaling Ceiling [RISK: P2] 🟢
| Aspect | Detail |
|--------|--------|
| **Scope** | Entire architecture |
| **Issue** | Everything in a single binary: CLI, REST, gRPC, MCP, WASM runtime, knowledge engine, memory engine, vector search, file watcher. No graceful subsystem degradation. |
| **Impact** | Cannot scale subsystems independently (e.g., scale knowledge engine separately from API). A plugin crash can take down the entire server. |
| **Mitigation** | For v1.4.x, acceptable. For v2.0, consider extracting heavy subsystems (knowledge, memory) into sidecar processes with gRPC communication. |
| **Cost if deferred** | Low for current stage. High if product-market fit demands independent scaling. |

---

## 4. Recommendations

### R1 [P0] — Complete Plugin Sandbox with OS-Level Isolation
**Context:** The current sandbox uses `Setrlimit` which contaminates the parent process. Memory isolation is explicitly skipped. Non-Linux platforms have zero sandboxing.

**Action:**
1. Implement cgroups v2 memory/CPU limits for Linux plugin execution (use `[]string{"/sys/fs/cgroup"}` bind mount + cgroup manager)
2. Add `seccomp` profile to restrict WASM plugin syscalls to a minimum set (read/write/exit/nanosleep)
3. On macOS, implement sandbox-exec profiles; on Windows, use Job Objects
4. Add a `SandboxConfig.MemoryLimitMB` field and enforce it

**Verification:** `make test -tags=integration` on Linux passes with cgroups v2 enabled. Plugin stress test (memory bomb) is contained.

### R2 [P0] — Harden gRPC Security to Match REST
**Context:** gRPC server has zero authentication while REST has JWT + API keys + CSRF + rate limiting.

**Action:**
1. Add `AuthInterceptor` in `api/grpcserver/interceptors.go` using JWT validation from `internal/auth`
2. Add TLS support via `grpc.Creds()` — make it configurable (self-signed for dev, cert file for prod)
3. Add rate limiting interceptor matching REST implementation
4. Expose gRPC port 14122 in Dockerfile and document in docker-compose.yml

**Verification:** `grpcurl -insecure localhost:14122 list` fails without valid JWT. Integration test verifies auth flow.

### R3 [P1] — Create Shared API Handler Abstraction
**Context:** REST (52 endpoints), gRPC (3 services), and MCP (14 tools) each independently implement business logic wiring. This is a 3× maintenance burden.

**Action:**
1. Define all capabilities as Go interfaces in `internal/port/` (e.g., `KnowledgeSearcher`, `MemoryStorer`, `RuntimeStatusProvider`)
2. Refactor REST handlers to use interfaces instead of engine pointers directly
3. Refactor gRPC service implementations to use same interfaces
4. Refactor MCP handlers to use same interfaces
5. *(Optional)* Generate REST handler code from proto annotations

**Verification:** All 3 API surfaces pass same integration test suite for each capability.

---

## 5. Summary

| Dimension | Verdict | Confidence |
|-----------|---------|------------|
| **Stack Health** | ✅ Modern, lean, well-chosen | 0.75 |
| **Architecture Cohesion** | ✅ Clean domain boundaries, hexagonal ports | 0.72 |
| **Security Posture** | ⚠️ Asymmetric (REST hardened, gRPC open) | 0.55 |
| **Plugin Isolation** | ❌ Known documented gap | 0.35 |
| **Test Coverage** | ⚠️ Threshold ambiguity (40-70%) | 0.50 |
| **Production Readiness** | ✅ Good for dev/pre-prod, needs hardening for prod | 0.60 |

**Next CTO check-in:** Post-remediation of R1 (Sandbox) and R2 (gRPC auth), or in 2 weeks, whichever comes first.
