# Cosca Messaging Audit Report

> **Date**: 2026-07-28 | **Auditor**: cosca-messaging | **Version**: 1.0.0

## Executive Summary

The Cosca platform has **5 independent event/streaming systems**, none of which communicate with each other. The WebSocket Hub is the only client-facing real-time channel. Internal event buses exist for runtime lifecycle and plugin communication but are completely isolated. There is zero integration with external message brokers, no event persistence, and no formal pub/sub contract.

**Maturity Level**: Level 1 — Ad-hoc, scattered, no unified strategy.

---

## 1. Event Systems Inventory

### 1.1 WebSocket Hub (`api/stream/websocket.go`)

| Attribute | Value |
|-----------|-------|
| **Type** | Topic-based pub/sub over WebSocket |
| **Architecture** | Single event-loop goroutine (select-based) |
| **Delivery** | Best-effort, fire-and-forget |
| **Backpressure** | Channel buffer = 64 per connection, drops on overflow |
| **Retry** | None |
| **Persistence** | None (in-memory only) |
| **Topics Observed** | `chat`, `sync`, `workflow` |
| **Protocol** | Client sends: subscribe, unsubscribe, ping; Server sends: subscribed, unsubscribed, pong, error, + typed events |
| **Shutdown** | Graceful with 30s drain window |

**Strengths**:
- Clean single-goroutine event-loop design — no mutex contention on hot path
- Slow consumer isolation (channel overflow → drop, not block)
- Per-connection JWT auth
- Graceful shutdown with drain

### 1.2 SSE Writer (`api/stream/sse.go`)

| Attribute | Value |
|-----------|-------|
| **Type** | HTTP Server-Sent Events (request-scoped) |
| **Delivery** | Synchronous, one-to-one |
| **Concurrency** | NOT safe — single goroutine per stream |
| **Events** | `thinking`, `response`/`token`, `progress`, `done`, `error` |
| **Headers** | Content-Type: text/event-stream, Cache-Control: no-cache, X-Accel-Buffering: no |

**Strengths**:
- Zero-dependency (stdlib only)
- Legacy format compatibility with `formatSSEEvent` dual-mode (string/map)
- Close-after-write guard

### 1.3 Runtime EventBus (`internal/runtime/runtime.go`)

| Attribute | Value |
|-----------|-------|
| **Type** | Synchronous in-process event bus |
| **Scope** | Application lifecycle |
| **Events** | `state_change`, `subsystem_started`, `subsystem_stopped`, `subsystem_error`, `health_change`, `startup_complete`, `shutdown_initiated`, `shutdown_complete`, `config_reload` |
| **Delivery** | Synchronous, sequential, 10s timeout per handler |
| **Subscription** | Per-event-type, multiple handlers |
| **Persistence** | None |
| **Retry** | None |
| **Handler Errors** | Logged as warning, does not abort remaining handlers |

**Strengths**:
- UUID event IDs, timestamped
- Clean Subscribe/Unsubscribe/Publish API
- Context-aware with timeout per handler
- Good test coverage (eventbus_test.go, lifecycle tests)

**Gaps**:
- Unsubscribe removes ALL handlers for an event type (too coarse)
- No individual handler unsubscribe by ID
- Synchronous delivery blocks the event loop if a handler stalls

### 1.4 Plugin EventBus (`internal/plugins/events.go`)

| Attribute | Value |
|-----------|-------|
| **Type** | Asynchronous in-process pub/sub |
| **Scope** | Inter-plugin and system-wide |
| **Events** | `index.started/completed/error`, `search.started/completed/error`, `context.built`, `memory.stored/retrieved`, `plugin.started/stopped/error`, `system.error`, `config.changed`, `system.shutdown` |
| **Delivery** | Async (goroutine per handler) + Sync mode available |
| **Subscription** | Individual subscriber IDs, filterable by Type + Source |
| **Persistence** | None |
| **Retry** | None |
| **Buffer** | Configurable (default 100, singleton uses 200) |
| **Singleton** | `DefaultEventBus()` — package-level global |

**Strengths**:
- Best design among all event systems: filterable subscriptions, subscriber IDs, sync+async modes
- Panic isolation per handler (recover in goroutine)
- Plugin lifecycle integration: ClearPluginSubscribers on shutdown
- Global helpers: `PublishGlobalEvent`, `SubscribeGlobal`

**Gaps**:
- Async delivery loses ordering
- No delivery confirmation / acknowledgment
- Global singleton is an anti-pattern for testing

### 1.5 Orchestration Stream Events (`internal/orchestration/types.go`)

| Attribute | Value |
|-----------|-------|
| **Type** | Go channel-based streaming |
| **Scope** | AI orchestration pipeline |
| **Events** | `progress`, `chunk`, `stage_transition`, `error` |
| **Pattern** | `ExecuteStream() → <-chan StreamEvent` |

**Notes**: This is a streaming protocol, not an event bus. It is the internal mechanism by which the orchestration engine feeds tokens and progress to the SSE handler.

### 1.6 Telemetry Events (`internal/telemetry/`)

| Attribute | Value |
|-----------|-------|
| **Type** | SQLite-backed analytics |
| **Scope** | Anonymous usage reporting |
| **Events** | `command_executed`, `index_completed`, `search_performed`, `context_built`, `memory_stored`, `plugin_installed`, `error_occurred`, `runtime_started/stopped`, `provider_called`, `config_changed`, `update_checked`, `sync_completed` |
| **Persistence** | SQLite (`telemetry.db`) |
| **Privacy** | No PII, no file contents |

### 1.7 Plugin Hook Registry (`internal/plugins/hooks.go`)

| Attribute | Value |
|-----------|-------|
| **Type** | Lifecycle interception hooks |
| **Hook Points** | `before_index`, `after_index`, `before_search`, `after_search`, `before_context`, `after_context`, `before_execute`, `after_execute` |
| **Execution** | Priority-ordered, 30s timeout per hook |
| **Error Handling** | Collects errors, continues execution |

---

## 2. Event Flow Map

```
                    ┌─────────────────────────────────────────────────┐
                    │                 CLIENT (Browser)                 │
                    └──────┬──────────────────────────────────┬───────┘
                           │ WebSocket                        │ SSE (HTTP)
                           ▼                                  ▼
┌──────────────┐   ┌───────────────┐                  ┌──────────────┐
│  WebSocket   │   │  WebSocket    │                  │  SSE Writer  │
│  Handler     │──▶│  Hub          │                  │              │
│  (upgrade)   │   │  (event loop) │                  │  (thinking,  │
└──────────────┘   └──────┬────────┘                  │   response,  │
                           │                           │   done,      │
          ┌────────────────┼─────────────────┐         │   error)     │
          │                │                  │         └──────────────┘
          ▼                ▼                  ▼               ▲
  ┌───────────┐   ┌───────────┐      ┌───────────┐          │
  │  Handler  │   │  Handler  │      │  Handler  │          │
  │  run.go   │   │ knowledge │      │ workflows │          │
  │           │   │   .go     │      │   .go     │          │
  │ Broadcast:│   │ Broadcast:│      │ Broadcast:│          │
  │ chat_*    │   │ sync_*    │      │ step_*,   │          │
  │           │   │           │      │ workflow_*│          │
  └───────────┘   └───────────┘      └───────────┘          │
                                                             │
  ┌──────────────────────────────────────────────────────────┤
  │                   Orchestration Engine                   │
  │  ExecuteStream() → <-chan StreamEvent ──────────────────┘
  │  Events: progress, chunk, stage_transition, error       │
  └──────────────────────────────────────────────────────────┘
                                                             
  ┌──────────────────────────────────────────────────────────┐
  │         Runtime EventBus (separate, isolated)            │
  │  Events: state_change, startup_complete, shutdown, etc. │
  │  Consumers: Only in tests — no production subscribers    │
  └──────────────────────────────────────────────────────────┘
                                                             
  ┌──────────────────────────────────────────────────────────┐
  │         Plugin EventBus (separate, isolated)             │
  │  Events: index.*, search.*, memory.*, plugin.*, etc.    │
  │  Consumers: Plugin lifecycle manager only               │
  └──────────────────────────────────────────────────────────┘
                                                             
  ┌──────────────────────────────────────────────────────────┐
  │              Telemetry (separate, isolated)               │
  │  Events → SQLite → Batch Report → External Server        │
  └──────────────────────────────────────────────────────────┘
```

**Critical observation**: The WebSocket Hub (`api/stream/`) and the Plugin EventBus (`internal/plugins/`) are COMPLETELY DISCONNECTED. Events published to one never reach the other. A plugin that emits `index.completed` via `PublishGlobalEvent` will NOT notify WebSocket subscribers.

---

## 3. Gap Analysis

### GAP-01: No Unified Event Bus
**Severity**: Critical

Three separate in-process event buses (Runtime, Plugin, WebSocket Hub) with zero integration. Events cannot flow between domains. A runtime `startup_complete` event should trigger a WebSocket broadcast to `system` topic, but there's no bridge.

### GAP-02: No Event Persistence / Event Sourcing
**Severity**: High

All events are ephemeral. If the process crashes:
- All in-flight WebSocket messages are lost
- No event replay for new subscribers
- No audit trail of domain events
- No temporal query support ("what happened between 10:00 and 10:05?")

The telemetry system does persist to SQLite, but it's analytics-only, not an event store.

### GAP-03: No External Message Queue Integration
**Severity**: Medium (current scale), High (future)

Zero integration with Kafka, RabbitMQ, NATS, or Redis pub/sub. For a single-process architecture this is acceptable, but:
- No horizontal scaling path for WebSocket (need Redis pub/sub for multi-instance)
- No durable messaging for critical workflows
- No integration with external systems

### GAP-04: No Retry / Dead Letter Queue
**Severity**: High

- WebSocket Hub: drops messages on slow consumer channels (explicit design choice)
- Plugin EventBus: async goroutine dispatch with no retry
- Runtime EventBus: logs handler errors but doesn't retry
- No dead letter queue anywhere in the system

### GAP-05: No Event Schema Versioning
**Severity**: Medium

Events are ad-hoc `map[string]interface{}` or `interface{}` payloads. No schema registry, no versioning, no backward compatibility guarantees. Changing a payload structure silently breaks consumers.

### GAP-06: No Idempotency / Deduplication
**Severity**: Medium

No event ID-based deduplication. If a handler is called twice with the same event (e.g., due to retry), it will process the event twice. The Plugin EventBus generates UUIDs but doesn't check for duplicates.

### GAP-07: No Ordering Guarantees
**Severity**: Medium

Plugin EventBus dispatches asynchronously via goroutines — ordering is non-deterministic. Runtime EventBus is synchronous/ordered. WebSocket Hub is ordered per-connection but not across connections.

### GAP-08: No Observability Instrumentation
**Severity**: High

- No event tracing (no correlation IDs across event boundaries)
- No metrics on event throughput, latency, or error rates
- No monitoring of WebSocket Hub backpressure (drop rate unknown)
- No alert on event bus saturation

### GAP-09: WebSocket Hub Backpressure is Silent
**Severity**: Medium

When `conn.send` channel is full (buffer=64), messages are silently dropped. The sender (`BroadcastEvent` caller) has no way to know the message wasn't delivered. The consumer gets no indication of data loss.

### GAP-10: No At-Least-Once Delivery Guarantee
**Severity**: Medium

Nowhere in the system is there at-least-once delivery. WebSocket Hub drops on backpressure; Plugin EventBus has no ack; Runtime EventBus errors are swallowed. The agent's prompt itself declares "At-least-once delivery" as a STANDARD but the codebase does not implement it.

### GAP-11: Singleton Anti-patterns
**Severity**: Low

Both `DefaultEventBus()` and `DefaultHookRegistry()` use package-level singletons. This makes testing harder (shared mutable state) and prevents runtime reconfiguration.

---

## 4. Recommendations

### Quick Wins (Level 1 → 2)

#### QW-01: Add Event Metrics to WebSocket Hub
Add prometheus counters for:
- `cosca_ws_messages_published_total{topic}`
- `cosca_ws_messages_dropped_total{topic}`
- `cosca_ws_connections_active`
- `cosca_ws_subscriptions_active{topic}`

**Effort**: 2 hours

#### QW-02: Add Correlation IDs Across Boundaries
Inject a `trace_id` / `correlation_id` into all event payloads. Generate at the SSE stream entry point and propagate through WebSocket broadcasts.

**Effort**: 3 hours

#### QW-03: WebSocket Hub — Notify Sender on Drop
Change `Broadcast` to return a `BroadcastResult` with per-topic delivery stats so callers know when messages are dropped.

**Effort**: 1 hour

#### QW-04: Event Schema Constants
Extract all event type strings into a single `pkg/events/types.go` package:
```go
package events

// WebSocket broadcast event types (server → client)
const (
    WsChatStarted    = "chat_started"
    WsChatEnded      = "chat_ended"
    WsSyncStarted    = "sync_started"
    WsSyncProgress   = "sync_progress"
    WsSyncCompleted  = "sync_completed"
    WsWorkflowFailed = "workflow_failed"
    // ...
)
```

**Effort**: 1 hour

### Medium Term (Level 2 → 3)

#### MT-01: Unify Internal Event Bus
Create `internal/eventbus/` that consolidates:
- Plugin EventBus (best design — keep its API)
- Runtime EventBus (migrate to use unified bus)
- WebSocket Hub integration (bridge)

New design:
```go
// internal/eventbus/bus.go
type Bus struct { /* ... */ }

// Bridge: plugin events → WebSocket topics
bus.Bridge("index.completed", wsHub, []string{"sync"})
bus.Bridge("memory.stored",   wsHub, []string{"memory"})
```

**Effort**: 3 days

#### MT-02: Event Persistence with SQLite Event Store
Add `internal/eventstore/` backed by SQLite (reuse the existing telemetry SQLite pattern):
- Append-only event log
- Per-event-type TTL (e.g., keep 7 days of chat events, 30 days of sync events)
- Replay capability for late-joining WebSocket subscribers
- Use existing `uuid` for idempotency check

**Effort**: 3 days

#### MT-03: Dead Letter Queue Pattern
Add DLQ to the unified event bus:
```go
type DeadLetter struct {
    Event     Event
    Error     error
    Attempts  int
    Timestamp time.Time
}
bus.OnDeadLetter(func(dl DeadLetter) { /* log, metric, alert */ })
```

**Effort**: 2 days

#### MT-04: At-Least-Once for Critical Events
For WebSocket Hub broadcasts of critical events (workflow completion, sync status):
- Add acknowledgment protocol: client sends `{"type":"ack","id":123}` after receiving an event
- Server retries unacknowledged events up to 3 times
- Requires event IDs in all server→client messages

**Effort**: 3 days

### Long Term (Level 3 → 4)

#### LT-01: External Message Broker Integration
If the platform scales beyond single-process:
- **Redis Pub/Sub** for WebSocket Hub multi-instance fanout (lowest friction)
- **NATS** for internal event bus distributed to multiple services (or **Kafka** for durability)
- Abstract behind `Broker` interface for swappable backends

#### LT-02: Full CQRS / Event Sourcing
If domain complexity warrants it:
- Separate write model (command handlers) from read model (projections)
- Event store as source of truth
- Materialized views for query performance
- Event replay for debugging and auditing

#### LT-03: Schema Registry
If event schemas stabilize:
- Protobuf or JSON Schema for event contracts
- Generated code for type-safe event publishing/handling
- Versioned events with backward compatibility checks

---

## 5. Decision Framework

| If you need... | Implement |
|---------------|-----------|
| Observability right now | QW-01, QW-02 |
| Safer WebSocket delivery | QW-03, MT-04 |
| Cleaner codebase | QW-04, MT-01 |
| Data durability | MT-02 |
| Production reliability | MT-03, MT-04 |
| Multi-instance scaling | LT-01 |
| Full audit trail | LT-02 |

---

## 6. Current State vs. Standards

The agent PROMPT declares these standards:
> **STANDARDS**: At-least-once delivery. Idempotent handlers. Events versioned.

| Standard | Current State | Gap |
|----------|--------------|-----|
| At-least-once delivery | Not implemented anywhere | MT-03, MT-04 |
| Idempotent handlers | Not implemented | MT-02 (event ID dedup) |
| Events versioned | Not implemented | QW-04 (types), LT-03 (schemas) |

---

## Appendix A: File Inventory

| File | System | Lines | Purpose |
|------|--------|-------|---------|
| `api/stream/websocket.go` | WebSocket Hub | 554 | Real-time topic pub/sub |
| `api/stream/sse.go` | SSE Writer | 202 | Server-Sent Events output |
| `api/stream/types.go` | SSE Types | 23 | SSE event type constants |
| `api/stream/auth.go` | WS Auth | 47 | JWT auth for WebSocket upgrade |
| `api/rest/handler/websocket.go` | WS Handler | 101 | HTTP → WebSocket upgrade endpoint |
| `api/rest/handler/run.go` | Run Handler | 358 | Chat streaming + WS broadcasts |
| `api/rest/handler/knowledge.go` | Knowledge Handler | 356 | Sync streaming + WS broadcasts |
| `api/rest/handler/workflows.go` | Workflows Handler | 184 | Workflow streaming + WS broadcasts |
| `api/rest/server.go` | Server | 450 | Hub bootstrap and wiring |
| `internal/runtime/runtime.go` | Runtime EventBus | 682 | Lifecycle events + state machine |
| `internal/runtime/eventbus_test.go` | Runtime EB Tests | 243 | EventBus unit tests |
| `internal/plugins/events.go` | Plugin EventBus | 412 | Inter-plugin pub/sub |
| `internal/plugins/hooks.go` | Hook Registry | 373 | Lifecycle hook interception |
| `internal/plugins/lifecycle.go` | Plugin Lifecycle | 604 | Plugin init/start/stop with events |
| `internal/telemetry/events.go` | Telemetry Events | 270 | Analytics event definitions |
| `internal/telemetry/telemetry.go` | Telemetry Engine | 508+ | SQLite-backed telemetry |
| `internal/orchestration/types.go` | Stream Events | 495 | Pipeline streaming events |

---

## Appendix B: Topic/Event Cross-Reference

| Domain | WebSocket Topic | WebSocket Events | Plugin Events | Runtime Events |
|--------|----------------|-----------------|---------------|----------------|
| Chat | `chat` | `chat_started`, `chat_ended` | — | — |
| Sync | `sync` | `sync_started`, `sync_progress`, `sync_completed` | `index.*` | — |
| Workflow | `workflow` | `step_*`, `workflow_failed`, `workflow_completed` | — | — |
| Lifecycle | — | — | `plugin.*`, `system.shutdown` | `startup_complete`, `shutdown_*`, `state_change` |
| Search | — | — | `search.*` | — |
| Memory | — | — | `memory.*` | — |
| Config | — | — | `config.changed` | `config_reload` |
| Health | — | — | — | `health_change` |
| Subsystems | — | — | — | `subsystem_*` |
