---
type: architecture
key: streaming-plan
tags: [streaming, sse, websocket, architecture, planning]
timestamp: 2026-07-28T00:00:00Z
status: draft
agent: Architecture Chief + Backend Chief
version: 1.4.0-dev
---

# Cosca — WebSocket/SSE Streaming Plan

> **Status**: Discovery & Planning Complete | **Target**: v1.5.0 | **Effort**: ~40-55 hours

## 1. Discovery Summary

### 1.1 What Exists Today

| Area | Status | Details |
|------|--------|---------|
| **SSE for Chat** | ✅ Fully implemented | `POST /v1/run/stream` in `api/rest/handler/run.go` (L197-330). `sendSSE()` helper at L333-338. Uses `text/event-stream`, `http.Flusher`. |
| **Chat Provider Streaming** | ✅ Mature | All 10+ providers implement `chat.ChatStream` interface (`internal/chat/types.go:224-231`). Registry has fallback for streaming. |
| **SDK Streaming** | ✅ Implemented | `pkg/cosca/orchestration.go:144` — `OrchestrationSDK.Stream()` consumes SSE via `ReadableStream` pattern. |
| **Frontend Streaming** | ✅ Implemented + Placeholder | `use-orchestration-stream.ts` works for POST SSE. `use-provider-stream.ts` is a placeholder for per-provider streaming. |
| **WebSocket** | ❌ None | Zero WebSocket code, zero dependencies, zero infrastructure. |
| **Knowledge Sync Streaming** | ❌ None | `POST /v1/knowledge/sync` is fully synchronous. Engine's `Sync()` method has no progress callback. |
| **Workflow Progress Streaming** | ❌ None | `POST /v1/workflows/{name}/run` returns single result. `Manager.Run()` is synchronous. |
| **Status Streaming** | ❌ None | `GET /v1/status` is a single JSON response. |
| **CSP ws://** | ✅ Already configured | `api/middleware/security.go:46` includes `ws://localhost:14120` in `connect-src`. |
| **Graceful Shutdown** | ✅ Pattern exists | `internal/cli/serve.go:419-481` — signal-based, 30s grace period, proper drain of HTTP servers. |

### 1.2 What's Missing

1. **Shared SSE utilities** — `sendSSE()` is trapped in `handler/run.go`, not reusable by other handlers.
2. **WebSocket library** — No WebSocket dependency in `go.mod`. No hub, no connection manager.
3. **WebSocket auth pattern** — JWT validation exists but only for HTTP headers/cookies, not query params.
4. **Progress callbacks** — Knowledge Engine's `Sync()` and Workflow Manager's `Run()` have no progress reporting hooks.
5. **Connection lifecycle management** — No tracking of active SSE/WS connections for graceful shutdown.

### 1.3 Risk R22 Assessment

The existing risk R22 ("no HTTP streaming") is partially mitigated — chat streaming works via SSE. The remaining gap is:
- No non-chat streaming endpoints (sync, workflows, status)
- No WebSocket for bidirectional/multi-event use cases
- No infrastructure for connection management

---

## 2. Architecture Decision: SSE vs WebSocket

### 2.1 Decision Matrix

| Criterion | SSE (`text/event-stream`) | WebSocket |
|-----------|---------------------------|-----------|
| **Direction** | Server → Client only (unidirectional) | Bidirectional |
| **Protocol** | HTTP/1.1 long-poll (GET or POST) | Upgrade from HTTP → WS protocol |
| **Browser API** | `EventSource` (GET only) or `fetch` + `ReadableStream` (POST) | `WebSocket` API |
| **Reconnection** | Automatic with `EventSource` | Manual (must implement) |
| **Complexity** | Low — stdlib `http.Flusher`, no deps | Medium — needs library + hub pattern |
| **Use in Cosca** | Chat token streaming, progress events | Real-time status, multi-event subscriptions |
| **Auth** | Standard HTTP (cookies, headers, API keys) | Token via query param (no custom headers on upgrade) |
| **Proxy/ALB** | Works natively (HTTP streaming) | May need sticky sessions, headers config |

### 2.2 Recommendation

**Use SSE for all unidirectional streaming** — it leverages the existing pattern, requires zero new dependencies, and works with the current auth infrastructure.

**Add WebSocket for bidirectional/multi-event use cases** — a single `GET /v1/ws` endpoint as a real-time gateway where clients subscribe to event channels.

**Rule of thumb**:
- SSE: server pushes data, client reads (chat tokens, sync progress, workflow progress)
- WebSocket: client subscribes to topics, server pushes multiple event types on one connection (real-time dashboard)

---

## 3. Proposed Architecture

### 3.1 New Package: `api/stream/`

```
api/stream/
├── sse.go           ← Shared SSE utilities (extracted from handler/run.go)
├── websocket.go     ← WebSocket hub + connection manager
├── auth.go          ← WebSocket auth (JWT query param validation)
└── types.go         ← Shared event types
```

**Rationale**: Extracting streaming infrastructure into its own package prevents circular dependencies between handlers and enables reuse across the `handler/` and `server/` packages.

### 3.2 `api/stream/sse.go` — Shared SSE Utilities

Extract and enhance `sendSSE()` from `handler/run.go:333-338`:

```go
// Package stream provides shared streaming infrastructure (SSE + WebSocket).
package stream

// SSEWriter is a thin wrapper around http.ResponseWriter + http.Flusher
// for writing Server-Sent Events.
type SSEWriter struct {
    w       http.ResponseWriter
    flusher http.Flusher
}

// NewSSEWriter sets up SSE headers and returns a writer.
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error)

// WriteEvent writes a typed event with JSON-payload.
func (s *SSEWriter) WriteEvent(eventType string, data any) error

// WriteRaw writes a raw SSE data line.
func (s *SSEWriter) WriteRaw(data string) error

// Flush flushes the underlying writer.
func (s *SSEWriter) Flush()
```

### 3.3 `api/stream/websocket.go` — Hub Pattern

```go
// Hub manages WebSocket connections and broadcasts.
type Hub struct {
    // Registered connections by topic.
    topics map[string]map[*Connection]struct{}
    // Register/unregister channels.
    register   chan *Connection
    unregister chan *Connection
    // Broadcast channel.
    broadcast  chan BroadcastMessage
}

// Connection wraps a WebSocket connection with metadata.
type Connection struct {
    conn     *websocket.Conn  // nhooyr.io/websocket
    ctx      context.Context
    cancel   context.CancelFunc
    claims   *auth.Claims     // JWT claims from connection auth
    topics   []string         // subscribed topics
    send     chan []byte
}

// BroadcastMessage is a message targeted at specific topics.
type BroadcastMessage struct {
    Topics  []string
    Payload []byte
}
```

### 3.4 WebSocket Auth Pattern

WebSocket upgrades cannot carry custom HTTP headers (browser limitation). The auth pattern:

1. Client connects: `ws://host:14120/v1/ws?token=<JWT>`
2. Server's `UpgradeHandler` extracts `token` from query params before upgrading
3. Validates JWT using existing `internalauth.ValidateToken()` (same as HTTP auth)
4. On failure: reject upgrade with `http.StatusUnauthorized` before the WebSocket handshake
5. On success: store claims in `Connection` struct, complete upgrade

**Security considerations**:
- JWT in query params is logged by proxies — mitigated by HTTPS (TLS)
- Token validated once at connection time; server drops connection if token expires mid-session (optional: periodic revalidation)
- Same RBAC as HTTP — claims carry user role, topics can be role-gated

### 3.5 Graceful Shutdown for WebSocket Connections

Add to `api/rest/server.go` and `internal/cli/serve.go`:

```go
// Server gains a WebSocket hub reference.
type Server struct {
    // ... existing fields ...
    wsHub *stream.Hub  // nil if WebSocket disabled
}

// Shutdown closes all WebSocket connections before HTTP shutdown.
func (s *Server) Shutdown(ctx context.Context) error {
    if s.wsHub != nil {
        s.wsHub.Shutdown(ctx)  // sends close frames, drains channels
    }
    // ... existing HTTP shutdown ...
}
```

Hub shutdown sequence:
1. Stop accepting new connections (close `register` channel)
2. Send close frames (1001 Going Away) to all active connections
3. Wait for connections to close or context deadline
4. Close all channels

---

## 4. Planned Endpoints

### 4.1 SSE Endpoints (Enhance Existing)

| # | Method | Path | Purpose | Auth | Priority |
|---|--------|------|---------|------|----------|
| 1 | `POST` | `/v1/run/stream` | Chat token streaming **(exists — refactor to use shared SSEWriter)** | JWT | P0 |
| 2 | `POST` | `/v1/knowledge/sync/stream` | Knowledge sync progress (phase, files processed, errors) | JWT | P1 |
| 3 | `POST` | `/v1/workflows/{name}/run/stream` | Workflow execution progress (step started/completed/failed) | JWT | P1 |
| 4 | `GET` | `/v1/status/stream` | Runtime status change events (state transitions, health changes) | JWT | P2 |

### 4.2 WebSocket Endpoint (New)

| # | Method | Path | Purpose | Auth | Priority |
|---|--------|------|---------|------|----------|
| 5 | `GET` | `/v1/ws` | Real-time event gateway (subscription-based, multi-topic) | JWT (query param) | P2 |

**WebSocket topic protocol**:

Client sends JSON messages to subscribe/unsubscribe:
```json
// → Client to Server
{"type": "subscribe", "topic": "chat", "id": 1}
{"type": "unsubscribe", "topic": "sync", "id": 2}
{"type": "ping", "id": 3}
```

Server sends JSON events on subscribed topics:
```json
// ← Server to Client
{"type": "chat", "data": {"token": "Hello", "agent": "cosca-architecture"}}
{"type": "sync", "data": {"phase": "scanning", "files_found": 42}}
{"type": "status", "data": {"state": "healthy", "uptime": "1h30m"}}
{"type": "pong", "id": 3}
{"type": "error", "data": {"message": "topic not available", "id": 1}}
```

**Available topics**:
- `chat` — Chat streaming tokens (from `POST /v1/run/stream` equivalent)
- `sync` — Knowledge sync progress
- `workflow` — Workflow execution progress
- `status` — Runtime status changes (pushed automatically)
- `executions` — Execution history events (new executions created)

---

## 5. Dependencies

### 5.1 New Dependencies

| Library | Version | Purpose | Size Impact |
|---------|---------|---------|-------------|
| `nhooyr.io/websocket` | latest | WebSocket library (Go std interfaces, context-aware) | ~200KB (minimal) |

**Why nhooyr.io/websocket**:
- Uses `context.Context` and `net.Conn` (standard Go patterns)
- Actively maintained (last release < 6 months)
- No CGO, pure Go
- Dial/Accept pattern matches Go 1.25 style
- Lighter than gorilla/websocket (fewer features, less code)
- Supports compression extension (permessage-deflate)

**Alternatives considered**:
- `gorilla/websocket`: Most popular but maintenance mode since gorilla archived. Still works but no updates.
- `gobwas/ws`: Zero-alloc, extremely fast. Overkill for Cosca's scale, and has a less intuitive API.
- `coder/websocket`: Fast, maintained, but less battle-tested than nhooyr.

### 5.2 Existing Dependencies (Reused)

No new Go dependencies needed for SSE — purely stdlib:
- `net/http` (ResponseWriter, Flusher, Hijacker)
- `fmt` (Fprintf for SSE data lines)
- `encoding/json` (event payload serialization)
- `context` (cancellation propagation)

---

## 6. Implementation Phases

### Phase 0: Extract Shared SSE Infrastructure (4-6 hours)

**Goal**: Refactor existing SSE without changing behavior, create reusable layer.

1. Create `api/stream/` package with `sse.go`, `types.go`
2. Extract `sendSSE()` → `SSEWriter` type with `WriteEvent()`, `WriteRaw()`, `Flush()`
3. Add SSE event types enum: `EventResponse`, `EventError`, `EventProgress`, `EventDone`, `EventThinking`
4. Migrate `handler/run.go:Stream()` to use `SSEWriter` (behavior identical)
5. Add `SSETestWriter` for handler tests
6. Run existing tests — all must pass without modification
7. Update `handler/run_test.go` if needed

**Deliverable**: `api/stream/sse.go`, `api/stream/types.go`, refactored `handler/run.go`

### Phase 1: Knowledge Sync Streaming (6-8 hours)

**Goal**: Add progress streaming to `POST /v1/knowledge/sync`.

1. Add `SyncProgress` callback type to `internal/knowledge/engine.go`:
   ```go
   type SyncProgressFn func(phase string, processed, total int, currentFile string)
   ```
2. Add `SyncWithProgress(ctx, progressFn)` method (or modify `Sync` with optional callback)
3. Instrument `Sync()` phases:
   - `"scanning"` — filepath.Walk progress
   - `"comparing"` — hash comparison progress
   - `"indexing"` — per-file index progress (added/updated/removed)
4. Create `POST /v1/knowledge/sync/stream` handler in `handler/knowledge.go`
5. Register route in `server.go`
6. Add tests for streaming handler
7. Add TypeScript SDK method for sync streaming
8. Add frontend hook (or extend useOrchestrationStream pattern)

**Deliverable**: Streaming sync endpoint, SDK method, tests

### Phase 2: Workflow Execution Streaming (6-8 hours)

**Goal**: Add step-level progress streaming to workflow execution.

1. Add `StepProgress` callback type to `internal/workflows/workflows.go`:
   ```go
   type StepProgressFn func(stepName string, status string, output string, stepNum, totalSteps int)
   ```
2. Add `RunWithProgress(ctx, name, progressFn)` method
3. Instrument `runFallback()` and `runWithPipeline()` to call progressFn per step
4. Create `POST /v1/workflows/{name}/run/stream` handler in `handler/workflows.go`
5. Register route in `server.go`
6. Add tests
7. Add TypeScript SDK method
8. Add frontend hook

**Deliverable**: Streaming workflow endpoint, SDK method, tests

### Phase 3: WebSocket Infrastructure (10-14 hours)

**Goal**: Add WebSocket hub, connection manager, and real-time gateway.

1. **Add dependency**:
   ```bash
   go get nhooyr.io/websocket
   ```

2. **Create `api/stream/websocket.go`** — Hub pattern:
   - `Hub` struct with register/unregister/broadcast channels
   - `Connection` struct wrapping nhooyr websocket
   - `Hub.Run()` goroutine for event loop
   - `Hub.Subscribe(topic, conn)`, `Hub.Unsubscribe(topic, conn)`
   - `Hub.Broadcast(msg BroadcastMessage)`
   - Read/write pump goroutines per connection
   - Ping/pong heartbeat (30s interval, 60s timeout)
   - Graceful shutdown: `Hub.Shutdown(ctx)`

3. **Create `api/stream/auth.go`** — WebSocket auth:
   - `AuthenticateUpgrade(r *http.Request, jwtSecret []byte)` — validates `?token=` query param
   - Returns `*auth.Claims` or error
   - Called before `websocket.Accept()`

4. **Create `GET /v1/ws` handler** in `handler/stream.go`:
   - New `StreamHandler` struct with `*stream.Hub` reference
   - `Upgrade(w, r)` — auth → accept → register connection → read/write pumps
   - Topic subscription protocol (see §4.2)
   - Topic-based access control (admin topics require admin role)

5. **Integrate into `server.go`**:
   - Add `wsHub *stream.Hub` field to `Server`
   - Create Hub in `New()` (or lazily on first connection)
   - Register `GET /v1/ws` route
   - Wire Hub shutdown into `Server.Shutdown()`

6. **Wire broadcasts from existing handlers**:
   - Chat: notify Hub on stream start/end
   - Knowledge: notify Hub on sync start/end
   - Workflows: notify Hub on run start/step/end
   - Runtime: notify Hub on state changes (via event bus subscription)

7. **Integrate into `internal/cli/serve.go`**:
   - No structural changes needed — Server creates Hub internally
   - Ensure Hub is created after auth store is available

8. **Add tests**:
   - Unit: Hub register/unregister/broadcast
   - Unit: Connection read/write pumps
   - Unit: Auth validation
   - Integration: Full upgrade → subscribe → receive → unsubscribe lifecycle

**Deliverable**: WebSocket gateway, hub, connection manager, auth, tests

### Phase 4: Status Streaming + Polish (4-6 hours)

**Goal**: Add status streaming and finalize the streaming infrastructure.

1. **Create `GET /v1/status/stream`** (SSE):
   - Subscribe to runtime event bus for state changes
   - Send `health` events on health transitions
   - Send `state` events on state changes
   - Keep connection open until client disconnects
   - Send periodic heartbeat events

2. **Runtime Event Bus Integration**:
   - Subscribe Hub to `state.changed` and `health.check` events
   - Forward to WebSocket `status` topic

3. **Config & Feature Flags**:
   - `--enable-websocket` flag on serve command (default: true)
   - `--ws-port` flag for separate WebSocket port (optional)
   - `COSCA_WS_DISABLE` env var

4. **Documentation**:
   - Update serve.go help text with streaming endpoints
   - Add streaming section to API docs
   - Document WebSocket protocol in `docs/`

**Deliverable**: Status streaming, feature flags, docs update

---

## 7. Effort Estimation

| Phase | Task | Hours | Skill Required |
|-------|------|-------|----------------|
| 0 | Extract SSE infrastructure | 4-6 | Go, HTTP, refactoring |
| 1 | Knowledge sync streaming | 6-8 | Go, knowledge engine internals |
| 2 | Workflow streaming | 6-8 | Go, workflow engine internals |
| 3 | WebSocket infrastructure | 10-14 | Go, WebSocket, concurrency |
| 4 | Status streaming + polish | 4-6 | Go, events, docs |
| **Total** | | **30-42** | |

Buffer (integration, testing, edge cases): +10h → **~40-55 hours total**

---

## 8. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `nhooyr.io/websocket` compatibility with Go 1.25 | Low | Medium | Test on Go 1.25 early. Fallback: gorilla/websocket (battle-tested even if unmaintained). |
| Knowledge engine Sync() refactoring breaks existing behavior | Medium | Medium | Add `SyncProgressFn` as optional parameter; keep original `Sync()` signature unchanged. |
| WebSocket proxy issues (nginx, ALB) | Medium | Low | Document proxy config. Default to same port as REST. Add `--ws-port` for separate port. |
| JWT in query param for WebSocket auth | Medium | Medium | Mitigated by HTTPS (TLS). Document that production should use HTTPS only. |
| Connection leak on client disconnect | Low | High | Ping/pong heartbeat detects dead connections. Context cancellation on read/write errors. |
| Hub goroutine leak on shutdown | Low | High | Drain channels. 30s shutdown timeout. Log connections that didn't drain. |
| SSEReader read-timeout kills long-lived SSE connections | Medium | High | Client sends periodic SSE comments as keepalive. Or use separate idle timeout for streaming endpoints. |

---

## 9. Open Questions

1. **Separate WebSocket port?** — Currently planned as route on main REST port (14120). A `--ws-port` flag could enable a dedicated port for production isolation. Decision: start with same port, add flag in Phase 4.

2. **Multi-tenant WebSocket?** — Current plan assumes single Cosca instance, single Hub. If multi-tenant needed later, Hub sharding by project/namespace. Out of scope for v1.5.

3. **gRPC streaming?** — The gRPC server already has infrastructure for streaming RPCs. Should we add streaming to Knowledge/Memory/Runtime gRPC services? Decision: defer to Post-v1.5.

4. **Per-provider chat streaming?** — Frontend has `use-provider-stream.ts` placeholder. Should we add `POST /v1/run/stream?provider=X`? Decision: already supported via existing `runRequest.Provider` field in the JSON body. Frontend can wire it today. No backend changes needed.

5. **SSE reconnection?** — Browser `EventSource` auto-reconnects but only for GET. Our chat streaming uses POST (needed for request body). Should we add an `EventSource`-compatible GET endpoint that uses query params? Decision: defer. Current `fetch` + `ReadableStream` pattern works.

---

## 10. Conclusion

The Cosca codebase already has a mature SSE infrastructure for chat streaming through the provider system. The path forward is:

1. **Extract** the existing SSE code into a shared package
2. **Extend** SSE to knowledge sync and workflow execution via progress callbacks
3. **Add** WebSocket as a real-time event gateway using nhooyr.io/websocket
4. **Wire** the event bus to push runtime state changes to connected clients

This approach minimizes scope creep, reuses existing patterns, and adds exactly one new dependency. The result will be a comprehensive streaming layer that covers all critical use cases.
