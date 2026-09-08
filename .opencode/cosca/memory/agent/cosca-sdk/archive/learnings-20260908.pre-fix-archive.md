# cosca-sdk - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-sdk — Semantic Learnings

> Auto-evolution memory.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-sdk |
| **Task** | Initial capability establishment |
| **Technique** | Standard sdk patterns |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #sdk #baseline |
| **Learned** | Ready for Level 2. |
| **Next** | Identify first advanced technique |

### 2026-07-27 — Test Suite & Production Readiness
| Field | Value |
|-------|-------|
| **Agent** | cosca-sdk |
| **Task** | Fill gaps: tests, README, config — prepare for NPM publication |
| **Technique** | Vitest with manually-mocked AxiosInstances (no MSW/axios-mock-adapter needed); tests inject mock http directly into API constructors which take AxiosInstance + config. For client tests, mock http is injected via `(client as any).http = http` after construction. |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #sdk #testing #vitest #npm |
| **Learned** | 1) `index.ts` had a bug: exported `CoscaClient` but client.ts class was `AosClient` — fixed with `as` alias. 2) Types in spec (context, plugins, discovery) marked as stubs but still fully testable via mock http. 3) The `axios` interceptor-based retry logic is clean and testable via public `get/post/put/delete` methods once http is swapped. 4) `tsconfig.json` excludes `**/*.test.ts` from build — vitest handles TS natively. 5) `files` field references `LICENSE` which must exist in package dir (copied from repo root). |
| **Tests** | 12 test files, 186 tests, 0 failing |
| **Next** | Real HTTP integration tests against a running Cosca Runtime instance |

### 2026-07-28 — Full SDK Audit (Go + TypeScript)
| Field | Value |
|-------|-------|
| **Agent** | cosca-sdk |
| **Task** | Auditar SDKs Go e TypeScript — documentar estado atual, gaps, recomendações |
| **Technique** | Full-package read of all 12 Go source files + 11 TS source files, cross-referencing every endpoint against `api/rest/openapi.yaml` (v1.3.0, 2738 lines, 40+ endpoints), running both test suites, analyzing generated code coverage |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #sdk #audit #go #typescript #openapi |
| **Confidence** | 0.92 |
| **Learned** | 1) Go SDK lives at `pkg/cosca/` (not `sdk/go/`) — 12 modules, 272 tests, 70% complete. 2) Go SDK has critical path mismatches: uses `/v1/runtime/status` and `/v1/runtime/health` but OpenAPI spec defines `/v1/status` and `/v1/health` — these would 404 against the real API. Also uses POST for memory search but spec uses GET. 3) TS SDK is at `sdk/typescript/` — 11 modules, 186 tests, 65% complete. Three modules (Context, Plugins, Discovery) are documented stubs waiting for backend handlers. 4) The generated `api.ts` covers only 12 of 40+ endpoints and is NOT imported by any code — it's stale but not blocking. 5) `AosClient` is a legacy class name leaked throughout the TS SDK; `CoscaClient` is just an alias. 6) Neither SDK covers Auth, Users, API Keys, Secrets, Audit, or Executions. 7) The Risk Registry's "TS SDK blocked by OpenAPI spec" claim is inaccurate — the SDK is fully functional with manual types, and regeneration takes 30m. 8) Both SDKs lack integration tests against a real Runtime instance. |
| **Report** | `.opencode/cosca/memory/sdk/audit-report.md` |
| **Next** | Fix Go SDK path mismatches (G-01), regenerate TS types (G-04), verify stub module backend handlers, add Auth module to both |

