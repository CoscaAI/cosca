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

### 2026-08-23 — Mega Brain: deliberação evidência-gated + plan-only
| Field | Value |
|-------|-------|
| **Agent** | cosca-architecture |
| **Task** | ADR-011 — adotar padrões-ouro (A1-A7, D1-D6) do Mega Brain no Cosca, sem implementar |
| **Technique** | Mapa gap: o Cosca TEM RAG/critic/gates/DAG/Ciclo-de-Decisão, mas é 1-agente auto-avaliação (passo 8), confiança intuitiva, sem convergência calculada, sem plan-only nem gate 3-estados. Solução: `internal/deliberate` determinístico (Convergence/Confidence/Emit/VoteCross/ValidateSpec) para P0/P1 + Plan-only no `Engine` reusando `internal/gate` e `internal/workflow`. Régua P9: modelo propõe, sistema decide. |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #mega-brain #deliberation #plan-only #adr #convergence #confidence #cpu #over-engineering |
| **Related** | `docs/adr/ADR-011-mega-brain-deliberation-and-plan-only.md`, `.cosca/fallback/knowledge/patterns/mega-brain-patterns.md`, `internal/orchestration/orchestrator.go`, `internal/orchestration/router.go`, `internal/gate/gate.go`, `internal/workflow/workflow.go`, `internal/confidence/tracker.go`, `internal/embed/cosca/{CONSTITUTION,QUALITY_GATES,DECISION_PROTOCOL,CONFIDENCE_MODEL}.md` |
| **Learned** | 1) Não existe `internal/review` (cosca-review/qa são agentes-texto) — deliberação deve viver num pacote determinístico `internal/deliberate`. 2) `Engine.Execute` roda inline (sem plan-only); a guarda já está em `internal/gate.GateStore` (plan→approving→approved→executed) — estender p/ 3-estados, não recriar. 3) `internal/workflow` já é DAG typed-routing com JoinNode paralelo + ErrCycle — usar como substrato, só somar CPM/FMEA. 4) Reuso é a régua: NÃO tocar `internal/embed/cosca/*` (P8). 5) Over-engineering honesto: batalha adversarial completa (A5), arquétipos D1-D10 (A6), D7 token-fencing, e todo o bloco C (RAG) ficam de fora — RAG merece ADR próprio. 6) Fatia 1 = plan-only + gate 3-estados (bloco 2) e `internal/deliberate` só P0/P1 (bloco 1). |
| **Next** | Se aprovado: fatiar 1 dos 2 blocos; considerar ADR-012 para RAG "zero achismo" (C2/C4/C5/C6). |
