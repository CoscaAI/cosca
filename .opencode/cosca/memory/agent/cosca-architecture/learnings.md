# cosca-architecture — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-architecture |
| **Task** | Initial capability establishment |
| **Technique** | Standard architecture patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #architecture #baseline #initialization |
| **Related** | See .opencode/cosca/memory/codebase/overview.md, .opencode/cosca/memory/pattern/ |
| **Learned** | Project established. Core architecture patterns documented. Ready for level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — gRPC Server Planning
| Field | Value |
|-------|-------|
| **Agent** | cosca-architecture + cosca-backend |
| **Task** | Architectural planning for gRPC server (3 services, 12 RPCs) |
| **Technique** | Full-stack analysis: proto files → engine interfaces → handler patterns → startup lifecycle. DRY via engine-sharing. |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #grpc #architecture #planning #adr #protobuf #server |
| **Related** | `.opencode/cosca/memory/architecture/grpc-server-plan.md`, `api/rest/server.go`, `internal/runtime/runtime.go`, `internal/cli/serve.go` |
| **Learned** | 1) Proto files at `proto/aos/v1/` with go_package `api/grpc/pb` — mismatch requires move. 2) REST handlers call engines directly (no service layer) — gRPC follows same pattern. 3) `memory.MemoryEngine.Delete()` exists and is functional (not placeholder). 4) Start gRPC in `internal/cli/serve.go` alongside REST/metrics servers with shared engine instances. 5) 12 RPCs confirmed (4 Knowledge + 6 Memory + 2 Runtime), not 15 as initially stated. 6) gRPC deps already in go.mod (grpc v1.64.0, protobuf v1.33.0). 7) All RPCs are unary — no streaming needed. 8) Server reflection should be opt-in flag for production safety. 9) Graceful shutdown: gRPC first (GracefulStop), then REST, then engines Close. 10) Mapping layer (pb ↔ domain) in separate files keeps service implementations clean. |
| **Next** | Execute Phase 0: move .pb.go to api/grpc/pb/, verify compilation. Then Phase 1: RuntimeService as template. |

### 2026-07-28 — Streaming Architecture Planning
| Field | Value |
|-------|-------|
| **Agent** | cosca-architecture + cosca-backend |
| **Task** | Discovery & planning for WebSocket/SSE streaming in Cosca serve.go |
| **Technique** | Full-stack discovery: go.mod deps → grep for stream/websocket/sse → chat/types.go interface → handler/run.go SSE impl → provider ChatStream → knowledge Sync → workflow Run → frontend hooks. Decision matrix SSE vs WS. |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #streaming #sse #websocket #architecture #planning #adr |
| **Related** | `.opencode/cosca/memory/architecture/streaming-plan.md`, `api/rest/handler/run.go`, `internal/chat/types.go`, `api/middleware/security.go`, `internal/knowledge/knowledge.go`, `internal/workflows/workflows.go` |
| **Learned** | 1) SSE already mature for chat — `POST /v1/run/stream` uses `sendSSE()` with `text/event-stream` + `http.Flusher`. 2) Zero WebSocket code or deps — no gorilla, nhooyr, or gobwas. 3) All 10+ providers implement `ChatStream` via `chat.ChatStream` interface. 4) Knowledge `Sync()` and Workflow `Run()` are fully sync — no progress callbacks exist yet. 5) CSP already has `ws://` in connect-src (line 46 of security.go). 6) Frontend `use-provider-stream.ts` is a placeholder waiting for backend wiring. 7) `sendSSE()` is trapped in handler/run.go — needs extraction to shared package. 8) Auth middleware uses JWT via cookie/Bearer header — WebSocket needs query param pattern. 9) Recommended lib: `nhooyr.io/websocket` (context-aware, Go std interfaces, pure Go). 10) 4 SSE + 1 WS endpoints planned. Total effort 40-55h across 5 phases. |
| **Next** | Phase 0: Extract SSE utilities to `api/stream/sse.go`. Phase 1: Add progress callbacks to knowledge Sync. |
