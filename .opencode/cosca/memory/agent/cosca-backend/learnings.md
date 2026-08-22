# cosca-backend — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Session: 2026-07-28 — API Architecture Audit

### 2026-07-28 — Complete API Surface Mapping
| Field | Value |
|-------|-------|
| **Agent** | cosca-backend |
| **Task** | Map entire REST API surface: endpoints, handlers, middleware chain, auth flow |
| **Technique** | Level 2 — API audit: traced all handler files (19 handlers in api/rest/handler/), middleware chain order (SecurityHeaders → Auth → CSRF → RateLimit → CORS → Logging), auth flow (3 methods: API Key → Cookie → Bearer), RBAC enforcement points |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #api #rest #handlers #middleware #documentation #auth |
| **Related** | api/rest/handler/, api/rest/server.go, docs/api-reference/auth.md |
| **Learned** | 36 registered REST endpoints across 10 domains: knowledge, memory, agents, skills, providers, workflows, run, executions, analytics, plugins. Auth endpoints: login/refresh/logout/me. Admin-only routes (RequireRole): /v1/users, /v1/api-keys, /v1/audit/logs, /v1/secrets, /v1/skills/{name}/install. Middleware chain built in buildHandler(): innermost is mux, outermost is SecurityHeaders. RBAC: admin bypasses all checks (rank 3), editor(2) and viewer(1) checked by integer comparison. Context claims stored via typed contextKey (not string). Auth middleware in api/auth/oidc.go — misnamed (no OIDC implementation). API key auth builds synthetic JWT claims from API key metadata. |
| **Next** | Level 3: Audit handler error consistency, check for missing input validation patterns, propose handler response envelope standardization |

### 2026-07-28 — Endpoint Coverage Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-backend |
| **Task** | Audit which domains have complete CRUD coverage and which have gaps |
| **Technique** | Level 1 — Endpoint inventory by domain: counted methods per resource, identified missing operations |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #api #rest #endpoints #coverage |
| **Related** | api/rest/handler/, docs/api-reference/overview.md |
| **Learned** | Complete coverage: agents, skills, providers, workflows, api-keys, users (full CRUD). Partial: knowledge (search/index/rebuild — no single-document get/delete), memory (search/store/get/delete/promote — full), plugins (install/list/get/remove/enable/disable — full), executions (list/get — no delete/export), runtime (GET only — no config update endpoint). Missing: agent invocation logging endpoint, bulk operations, export endpoints. |
| **Next** | Level 2: Design consistent error response format, add missing GET single document endpoint for knowledge |

## Session: 2026-07-28 — FASE 3 gRPC MemoryService

### 2026-07-28 — MemoryService gRPC implementation (6 RPCs)
| Field | Value |
|-------|-------|
| **Agent** | cosca-backend |
| **Task** | Implement MemoryServiceServer gRPC: Store, Search, Get, Delete, Promote, Stats |
| **Technique** | Level 3 — Full gRPC service implementation following the RuntimeService template pattern. Created mapping layer (pb↔domain), service layer (RPC handlers), and wired into GRPCServer |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #grpc #memory #service #fase3 |
| **Related** | api/grpcserver/mapping_memory.go, api/grpcserver/memory_service.go, api/grpcserver/server.go |
| **Learned** | 1) MemoryEngine.Delete (via FileStore.Delete) returns nil for non-existent records — MUST Retrieve before Delete to get proper NotFound gRPC status. 2) The gRPC MemoryServiceServer struct MUST embed `aospb.UnimplementedMemoryServiceServer` by value (not pointer) to satisfy forward compatibility. 3) TTL proto field is string parsed with `time.ParseDuration()` — invalid strings produce zero TTL, which the engine replaces with its configured DefaultTTL. 4) `GetLayerStats` returns `map[MemoryLayer]LayerStats` where LayerStats.Count maps to RecordCount and LayerStats.TotalSize maps to SizeBytes. 5) The server registration pattern: only register MemoryServiceServer if `mem != nil` (unlike Runtime which always registers with nil handling). 6) All CreatedAt timestamps use `time.RFC3339` format for consistency across gRPC services. |
| **Next** | FASE 4: Server + Interceptors integration testing. FASE 5: Integration tests with bufconn.

## Session: 2026-07-28 — FASE 0 — Streaming Plan: Extract SSE Infrastructure

### 2026-07-28 — Shared SSE package extraction (api/stream/)
| Field | Value |
|-------|-------|
| **Agent** | cosca-backend |
| **Task** | Extract sendSSE() from handler/run.go into shared api/stream/ package per streaming plan |
| **Technique** | Level 3 — Refactoring: created SSEWriter abstraction with full test coverage (19 tests), migrated Stream() handler to use it, ensured zero behavior change |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #streaming #sse #refactoring #fase0 #api |
| **Related** | api/stream/sse.go, api/stream/types.go, api/stream/test_writer.go, api/stream/sse_test.go, api/rest/handler/run.go |
| **Learned** | 1) Legacy sendSSE() format: `data: {"type":"<type>","content":<JSON-escaped string>}\n\n` — strictly maintained via formatSSEEvent(). 2) Done event had inline format at line 328 of run.go with `duration_ms` field — extracted to WriteDoneWithDuration(). 3) Variable name collision: local `stream` (chat.ChatStream) shadowed package `stream` import — renamed to `chatStream`. 4) SSETestWriter implements both http.ResponseWriter AND http.Flusher for handler test compatibility. 5) SSEWriter is NOT thread-safe (documented) — all writes from single goroutine. 6) Close() uses sync.Mutex only for the `closed` flag — not for write serialization. 7) formatSSEEvent handles strings (legacy "content" wrapper) and structured data (field merge at top level) via type switch. 8) Headers set by NewSSEWriter: Content-Type text/event-stream, Cache-Control no-cache, Connection keep-alive, X-Accel-Buffering no. 9) WriteError(nil) is a no-op — safe to call with nil errors. 10) All 33 existing handler tests pass unchanged — confirmed zero regression. |
| **Next** | Phase 1: Knowledge sync streaming (add SyncProgressFn callback, create POST /v1/knowledge/sync/stream, instrument Sync() phases) | |

## Session: 2026-07-28 — MCP Tools Discovery & Planning

### 2026-07-28 — MCP Server Architecture Discovery
| Field | Value |
|-------|-------|
| **Agent** | cosca-backend |
| **Task** | Discovery phase: map existing MCP tools, identify implementation pattern, plan 14 new tools across 4 domains |
| **Technique** | Level 2 — Full codebase discovery: grep for MCP patterns, read all manager APIs, trace server wiring, document architecture |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #mcp #tools #discovery #planning #agents #skills #workflows #providers |
| **Related** | api/mcp/server.go, internal/agents/agents.go, internal/skills/skills.go, internal/workflows/workflows.go, internal/providers/providers.go |
| **Learned** | 1) Single MCP server lives at api/mcp/server.go — JSON-RPC 2.0 over stdin/stdout. Currently 3 tools: cosca_knowledge_search, cosca_memory_store, cosca_runtime_status. 2) A separate schema-only definition exists at internal/editors/generic_mcp/generic_mcp.go — generates static MCP config files, NOT the runtime server. These two sets are out of sync (5 tools in generic_mcp vs 3 in runtime). 3) Implementation pattern: (a) json.RawMessage for InputSchema, (b) register in global listToolsResponse, (c) add case in handleToolCall switch, (d) dedicated handler method on *Server struct. 4) Server struct takes nil-safe manager dependencies — each handler checks for nil and returns -32000 "not available". 5) All 4 managers expose List/Get/Search APIs — agents lacks mutex but is read-only after startup; skills and workflows have sync.RWMutex; providers mutates active/model fields. 6) Workflow execution (Run) needs context.WithTimeout protection — long-running operations could hang MCP connection. 7) The orchestrator ToolExecutor (internal/orchestration/tool_exec.go) is a separate system for LLM agent tool loops, NOT MCP. 8) MCP server is NOT yet wired into runtime entrypoint — static reference in generic_mcp adapter only. |
| **Next** | Begin Phase 1 implementation: cosca_agent_list, cosca_agent_inspect, cosca_agent_search (3 tools ~150 lines in api/mcp/server.go) |
