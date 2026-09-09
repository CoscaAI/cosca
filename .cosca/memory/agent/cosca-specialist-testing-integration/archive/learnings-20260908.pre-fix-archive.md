# cosca-specialist-testing-integration - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-specialist-testing-integration — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-testing-integration |
| **Task** | Initial capability establishment |
| **Technique** | Standard testing-integration patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #testing-integration #baseline #initialization |
| **Related** | .cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core testing-integration patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — gRPC Integration Test Suite Extension
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-testing-integration |
| **Task** | Write integration tests for gRPC services (Knowledge, Memory, Runtime) |
| **Technique** | bufconn integration testing, concurrent gRPC calls with sync.WaitGroup, table-driven sub-tests, nil-engine edge cases, real SQLite FileStore |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #testing-integration #grpc #bufconn #concurrency #knowledge #memory #runtime |
| **Related** | api/grpcserver/server_test.go, internal/memory/store.go, internal/knowledge/knowledge.go, internal/runtime/runtime.go |
| **Learned** | 1) `newBufconnServer` helper creates all 3 services with real engines in TempDir. 2) Knowledge engine init is heavy (~200ms per test); use `initKnowledge=false` when not testing knowledge. 3) `engineInitMu` sync.Mutex serializes engine creation to prevent SQLite "database locked" across parallel packages. 4) Memory FileStore uses SQLite FTS5 index internally — tests exercise real FTS queries. 5) Concurrent gRPC calls against bufconn work correctly with shared connection; no races detected. 6) nil-engine tests verify gRPC status codes (FailedPrecondition) without needing real engines. |
| **Next** | Level 3: End-to-end tests with real TCP gRPC server, streaming RPCs, auth middleware integration |

