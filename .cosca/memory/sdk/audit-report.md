# Cosca SDK Audit Report

> **Date**: 2026-07-28
> **Auditor**: cosca-sdk (SDK Chief, Level 1)
> **Scope**: Go SDK (`pkg/cosca/`) and TypeScript SDK (`sdk/typescript/`)

---

## Executive Summary

| Dimension | Go SDK | TypeScript SDK |
|-----------|--------|----------------|
| **Location** | `pkg/cosca/` | `sdk/typescript/` |
| **Version** | (part of main module, v1.0.0-rc.1) | `@cosca/sdk` v1.1.0 |
| **Source files** | 12 Go files | 11 TS files + 1 generated |
| **Tests** | 272 passing | 186 passing |
| **API coverage** | ~60% (12 of ~20 domains) | ~55% (11 of ~20 domains) |
| **Overall completeness** | **70%** | **65%** |
| **Production readiness** | Beta — usable but path mismatches | Beta — usable but 3 modules are stubs |

**Veredict**: Both SDKs are in **beta/working** state with real value but significant gaps. Neither is production-grade. The Go SDK is slightly ahead in completeness (extra types, connection management, heartbeat), but uses inconsistent endpoint paths vs the OpenAPI spec. The TypeScript SDK is well-tested but has 3 stub modules (Context, Plugins, Discovery) and its generated types file is stale and unused.

---

## 1. SDK Architecture Comparison

### 1.1 Go SDK (`pkg/cosca/`)

**Package structure**:
```
pkg/cosca/
├── cosca.go          — Version info, platform info, mode
├── sdk.go            — Client, ClientConfig, CoscaError, retry/HTTP helpers
├── knowledge.go      — KnowledgeSDK (index, search, stats, sync)
├── memory.go         — MemorySDK (store, retrieve, search, snapshots, promote, delete, stats)
├── context.go        — ContextSDK (get, set, delete, list)
├── runtime.go        — RuntimeSDK (start, stop, status, health)
├── plugins.go        — PluginSDK (install, uninstall, list, get)
├── discovery.go      — DiscoverySDK (project, workspace, editor, discover)
├── graph.go          — GraphSDK (relations, stats, export)
├── agents.go         — AgentsSDK (list, search, get)
├── skills.go         — SkillsSDK (list, search, get, install)
├── providers.go      — ProvidersSDK (list, get, test, setActive)
├── workflows.go      — WorkflowsSDK (list, search, get, run)
└── orchestration.go  — OrchestrationSDK (run, stream)
```

**Design strengths**:
- Full `Client` lifecycle management (Connect, Reconnect, Close, heartbeat)
- Structured `CoscaError` with status code, code, and details
- Exponential backoff retry on 5xx
- All sub-SDKs hold a pointer to the parent Client
- TLS support
- `RunOption` functional options pattern for orchestration
- 272 tests with `httptest` server mocking — solid coverage

**Design weaknesses**:
- Single Go module — not separable as a library
- Uses plain `http.Client` — no typed client interface
- No context propagation in most methods (hardcoded `context.Background()`)
- Inconsistent: WorkflowsSDK.Run accepts context, but most other methods don't

### 1.2 TypeScript SDK (`sdk/typescript/`)

**Package structure**:
```
sdk/typescript/src/
├── index.ts          — Public exports (CoscaClient, all APIs, all types)
├── client.ts         — AosClient (CoscaClient), CoscaError, retry logic
├── types.ts          — All TypeScript types and interfaces
├── knowledge.ts      — KnowledgeAPI
├── memory.ts         — MemoryAPI
├── context.ts        — ContextAPI (STUB)
├── runtime.ts        — RuntimeAPI
├── plugins.ts        — PluginsAPI (STUB)
├── discovery.ts      — DiscoveryAPI (STUB)
├── agents.ts         — AgentsAPI
├── skills.ts         — SkillsAPI
├── providers.ts      — ProvidersAPI
├── workflows.ts      — WorkflowsAPI
├── orchestration.ts  — OrchestrationAPI
└── generated/
    └── api.ts        — Auto-generated from openapi-typescript (STALE)
```

**Design strengths**:
- Clean Axios-based HTTP client with retry interceptor
- `CoscaError` class with statusCode, code, details
- Well-documented README with examples
- Async generator for SSE streaming
- `COSCA_API_URL` env var support
- 186 tests — all passing
- Separate `package.json` — publishable to NPM

**Design weaknesses**:
- `generated/api.ts` is **stale** (12 of 40+ endpoints) and **not imported** by any code
- Three modules (Context, Plugins, Discovery) are explicit stubs waiting for backend handlers
- `AosClient` class name leaked in code (historical name)
- No JWT/auth token management — relies on user-passed apiKey
- Types use both camelCase and snake_case (mismatch between handwritten types and OpenAPI-generated types)

---

## 2. API Endpoint Coverage

### 2.1 OpenAPI Spec Endpoints (40+ total across 14 tags)

| Tag | Endpoints | Go SDK | TS SDK | Notes |
|-----|-----------|--------|--------|-------|
| System | `/health`, `/ready` | ✗ | ✗ | System probes — not SDK concern |
| Runtime | `/v1/status`, `/v1/health` | ✓ (wrong path*) | ✓ | Go uses `/v1/runtime/*`, spec uses `/v1/*` |
| Knowledge | 4 endpoints | ✓ | ✓ | Both match spec |
| Memory | 6 endpoints | ✓ (diff path) | ✓ | Go: POST for search; spec/TS use GET |
| Agents | 3 endpoints | ✓ | ✓ | Both match spec |
| Skills | 4 endpoints | ✓ | ✓ | Both match spec |
| Providers | 4 endpoints | ✓ | ✓ | Both match spec |
| Workflows | 4 endpoints | ✓ | ✓ | Both match spec |
| Orchestration | 2 endpoints | ✓ | ✓ | Both match spec |
| Auth | 5 endpoints | ✗ | ✗ | Login, refresh, me, logout, CSRF |
| Users | 4 endpoints | ✗ | ✗ | Admin-only CRUD |
| API Keys | 3 endpoints | ✗ | ✗ | Admin-only key management |
| Executions | 2 endpoints | ✗ | ✗ | Execution history |
| Audit | 3 endpoints | ✗ | ✗ | Admin-only audit logs |
| Secrets | 3 endpoints | ✗ | ✗ | Admin-only secrets vault |

**\*Path mismatch details**:
- Go SDK: `/v1/runtime/status` → OpenAPI spec: `/v1/status`
- Go SDK: `/v1/runtime/health` → OpenAPI spec: `/v1/health`
- Go SDK: `/v1/memory/{id}` → OpenAPI spec: `/v1/memory/get?id=...`
- Go SDK: POST `/v1/memory/search` → OpenAPI spec: GET with query params

### 2.2 Sub-API module status by SDK

| Module | Go SDK | TS SDK | Go Status | TS Status |
|--------|--------|--------|-----------|-----------|
| Knowledge | `KnowledgeSDK` | `KnowledgeAPI` | ✅ Complete | ✅ Complete |
| Memory | `MemorySDK` | `MemoryAPI` | ✅ Complete | ✅ Complete |
| Context | `ContextSDK` | `ContextAPI` | ✅ Has impl | ⚠️ STUB |
| Runtime | `RuntimeSDK` | `RuntimeAPI` | ✅ Complete | ✅ Complete |
| Plugins | `PluginSDK` | `PluginsAPI` | ✅ Has impl | ⚠️ STUB |
| Discovery | `DiscoverySDK` | `DiscoveryAPI` | ✅ Has impl | ⚠️ STUB |
| Graph | `GraphSDK` | (N/A in TS) | ✅ Complete | ❌ Missing |
| Agents | `AgentsSDK` | `AgentsAPI` | ✅ Complete | ✅ Complete |
| Skills | `SkillsSDK` | `SkillsAPI` | ✅ Complete | ✅ Complete |
| Providers | `ProvidersSDK` | `ProvidersAPI` | ✅ Complete | ✅ Complete |
| Workflows | `WorkflowsSDK` | `WorkflowsAPI` | ✅ Complete | ✅ Complete |
| Orchestration | `OrchestrationSDK` | `OrchestrationAPI` | ✅ Complete | ✅ Complete |

---

## 3. Generated Code Analysis (TypeScript)

### 3.1 Current state

The file `src/generated/api.ts` was generated via `npm run generate:types` which runs:
```
openapi-typescript ../../api/rest/openapi.yaml -o ./src/generated/api.ts
```

**What it covers**: Only 12 of 40+ endpoints in the full OpenAPI spec:
- `/v1/knowledge/search`, `/v1/knowledge/index`, `/v1/knowledge/stats`, `/v1/knowledge/sync`
- `/v1/memory/store`, `/v1/memory/search`, `/v1/memory/get`, `/v1/memory/delete`, `/v1/memory/promote`, `/v1/memory/stats`
- `/v1/status`, `/v1/health`

**What's missing from generated types**: Agents, Skills, Providers, Workflows, Orchestration, Auth, Users, API Keys, Executions, Audit, Secrets, Context, Plugins, Discovery — plus all system probes.

**Why?** The generated file was apparently produced from an earlier/smaller version of the OpenAPI spec (see `openapi.yaml` dates vs `api.ts` comment "Do not make direct changes"). The full spec at `api/rest/openapi.yaml` (v1.3.0, 2738 lines) is significantly larger.

### 3.2 Blockage assessment

The generated file is **stale but NOT blocking** the SDK. The SDK does not import from it — all types are defined manually in `src/types.ts`. The SDK functions normally without the generated types. However:

- **No build-time type safety** from OpenAPI spec
- **No automatic regeneration** when spec changes
- **Types are out of sync** — the generated types use snake_case (from spec) while SDK types use camelCase (from TypeScript conventions)

---

## 4. Gaps Prioritized

### 🔴 Critical (blocking production use)

| ID | Gap | Impact | Effort |
|----|-----|--------|--------|
| G-01 | **Go SDK path mismatch** — `/v1/runtime/*` vs `/v1/*` | Runtime, Status, Health calls 404 on real API | 1h |
| G-02 | **No Auth/Token management** in either SDK | Users/clients can't login, refresh, or manage tokens | 8h (both) |
| G-03 | **TS SDK 3 stub modules** (Context, Plugins, Discovery) | 27% of TS API surface unusable | 2h (types + wiring) |

### 🟡 High (significant quality improvement)

| ID | Gap | Impact | Effort |
|----|-----|--------|--------|
| G-04 | **Stale generated types** in TS SDK | No build-time API contract validation | 30m (regenerate) |
| G-05 | **Go SDK hardcoded context.Background()** | No timeout/cancellation control for callers | 4h |
| G-06 | **No Go SDK module separation** — part of main module | Can't `go get` SDK independently | 16h (module split) |
| G-07 | **Type name inconsistency** — `AosClient` vs `CoscaClient` | Confusing; `AosClient` is legacy name | 1h |
| G-08 | **No integration tests** — both SDKs only have unit tests | Can't verify against real API | 4h |

### 🟢 Medium (nice to have)

| ID | Gap | Impact | Effort |
|----|-----|--------|--------|
| G-09 | **Missing Admin APIs** — Users, API Keys, Audit, Secrets, Executions | Only admins can manage users/keys; separate admin client needed | 12h |
| G-10 | **Go SDK missing Graph SDK in TS** | TS users have no knowledge graph access | 2h |
| G-11 | **No NPM/Central Registry publication** for `@cosca/sdk` | Not installable without git clone | 4h |
| G-12 | **No gRPC support** in either SDK (protos exist) | High-perf streaming unavailable | 20h+ |

---

## 5. Recommendations

### 5.1 Quick Wins (1–2h each)

1. **Fix Go SDK paths** (G-01): Change `/v1/runtime/status` → `/v1/status`, `/v1/runtime/health` → `/v1/health`, `/v1/memory/{id}` → `/v1/memory/get?id=...`, and POST→GET for memory search. ~1h.

2. **Regenerate TS types** (G-04): Run `npm run generate:types` against the **full** `api/rest/openapi.yaml` (v1.3.0, 2738 lines). The command already exists — just needs execution. ~30m.

3. **Fix `AosClient` naming** (G-07): The `index.ts` already exports `AosClient as CoscaClient`. Internal references use `AosClient`. Just rename everywhere. ~1h.

4. **Add Context/Plugins/Discovery to TS** (G-03 part): The types already exist in `types.ts` and the code exists in the source files but docs mark them as stubs. Verify if the backend has handlers yet; if so, remove stub labels. ~1h.

5. **Run integration smoke test**: Point both SDKs at a running Cosca Runtime and verify connectivity, knowledge search, and health check. ~2h.

### 5.2 Unblocking TypeScript SDK (if blocked as reported)

The Risk Registry mentions the TS SDK is "blocked pending OpenAPI spec generation." **Assessment: Not actually blocked.** The SDK is fully functional with its manually-written types. The generated file is unused. However:

- **If "blocked" means "can't auto-generate":** Fix is trivial — the script is in place (`npm run generate:types`). Just run it. If it fails on the full spec, it's likely a version compatibility issue with `openapi-typescript` v7.x and OpenAPI 3.0.3.
- **If "blocked" means "spec is incomplete":** The spec at `api/rest/openapi.yaml` is very complete (2738 lines, 14 tags). No blockers there.

### 5.3 Medium-term Roadmap

1. **Auth module** — Add `AuthAPI`/`AuthSDK` to both SDKs (login, refresh, me, logout). Highest-value missing feature.
2. **Admin modules** — Add Users, API Keys, Secrets, Audit, Executions for admin tooling.
3. **Go SDK module separation** — Extract `pkg/cosca/` into `sdk/go/` with its own `go.mod` for `go get` support.
4. **gRPC transports** — Leverage existing protos for high-performance streaming.
5. **E2E/Integration tests** — Test both SDKs against a real Cosca Runtime instance.

---

## 6. Completeness Scores

| SDK | API Coverage | Test Coverage | Documentation | Code Quality | **Overall** |
|-----|-------------|---------------|---------------|--------------|-------------|
| **Go SDK** | 60% (12/20 domains) | 80% (272 tests) | 60% (godoc + README) | 75% | **70%** |
| **TypeScript SDK** | 55% (11/20 domains, 3 stubs) | 85% (186 tests all passing) | 80% (excellent README) | 80% | **65%** |

---

## 7. Methodology

This audit was performed by:
1. Reading every source file in both SDKs
2. Cross-referencing API endpoints against `api/rest/openapi.yaml` (v1.3.0, 2738 lines)
3. Running the full test suites:
   - Go: `go test ./pkg/cosca/...` — 272 tests, all passing
   - TS: `vitest run` — 186 tests across 12 files, all passing
4. Analyzing generated code coverage vs full spec
5. Comparing type definitions, patterns, and design decisions between SDKs

---

*Report generated by cosca-sdk (SDK Chief). Next review: when Auth module is added or OpenAPI spec reaches v1.4.0.*
