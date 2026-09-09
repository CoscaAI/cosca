# cosca-messaging - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-messaging — Semantic Learnings

> Auto-evolution memory.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-messaging |
| **Task** | Initial capability establishment |
| **Technique** | Standard messaging patterns |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #messaging #baseline |
| **Learned** | Ready for Level 2. |
| **Next** | Identify first advanced technique |

### 2026-07-28 — Full-Platform Messaging Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-messaging |
| **Task** | Audit messaging patterns across Cosca platform (WebSocket Hub, SSE, event buses, hooks) |
| **Technique** | Comprehensive event system discovery — scanning all Go packages for event/publish/subscribe/channel patterns, mapping topology, identifying gaps against messaging standards (at-least-once delivery, idempotency, dead letter queues, event persistence, schema versioning) |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #messaging #audit #events #pubsub #websocket #sse #eventbus |
| **Learned** | Cosca has 5 independent event systems (WebSocket Hub, SSE Writer, Runtime EventBus, Plugin EventBus, Orchestration StreamEvents) plus Hook Registry and Telemetry — all completely isolated with zero bridges between them. No event persistence, no retry/DLQ, no schema versioning, no external broker integration. The Plugin EventBus has the best API design (filterable subscriptions, sync+async modes, panic isolation) and should serve as the foundation for unification. WebSocket Hub silently drops messages on backpressure (buffer=64). The declared standard "at-least-once delivery" is not implemented anywhere. |
| **Next** | Implement Quick Win QW-01 (WebSocket Hub prometheus metrics) then bridge Plugin EventBus to WebSocket Hub via MT-01 unified bus design |
| **Confidence** | 0.82 |

### 2026-07-28 — Multi-System Event Topology Mapping
| Field | Value |
|-------|-------|
| **Agent** | cosca-messaging |
| **Task** | Map complete event flow topology across WebSocket, SSE, Runtime, Plugin, and Orchestration boundaries |
| **Technique** | Call-site tracing — followed every BroadcastEvent, Publish, Subscribe, ExecuteHooks call site to build a complete directed graph of event producers and consumers |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #messaging #topology #audit #event-flow |
| **Learned** | Three critical disconnects: (1) Plugin events (index.completed, memory.stored) never reach WebSocket clients, (2) Runtime lifecycle events have zero production subscribers (all Subscribe calls are only in test files), (3) The WebSocket Hub is the ONLY client-facing real-time channel but has no integration with internal event buses. Handlers (run.go, knowledge.go, workflows.go) manually call both SSE WriteEvent AND hub.BroadcastEvent in the same code block — duplication that signal the need for a bridge. |
| **Next** | Design the Bridge abstraction: `bus.Bridge(eventType, wsHub, topics)` that auto-forwards matching internal events to WebSocket topics |
| **Confidence** | 0.78 |

