# Architecture Overview

> **Status**: active | **Owner**: Architecture Chief | **Last Updated**: 2026-07-31

## System Architecture

Cosca follows a **layered architecture** with a Go core. Each layer has well-defined responsibilities and communicates through formal interfaces. The design emphasizes modularity, testability, and runtime independence.

```
┌──────────────────────────────────────────────────────────────────────────┐
│                     Cosca — SYSTEM ARCHITECTURE                         │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    WEB CONSOLE LAYER (Next.js 15)                │    │
│  │                                                                  │    │
│  │  Dashboard · Knowledge Explorer · Memory Viewer · Agents        │    │
│  │  Skills · Providers · Workflows · Runtime · Settings            │    │
│  │  Admin (Users + API Keys) · Login · Command Palette             │    │
│  └────────────────────────────┬────────────────────────────────────┘    │
│                               │ REST API (JSON, port 14120)             │
│  ┌────────────────────────────▼────────────────────────────────────┐    │
│  │                    REST API LAYER (60 endpoints)                  │    │
│  │                                                                  │    │
│  │  Auth (internal/auth: apikeys·jwt·users) · RBAC · JWT HS256     │    │
│  │  Prometheus Metrics (:14121) · gRPC (:14122) · Rate Limiting    │    │
│  │  Domains: Knowledge · Memory · Runtime · Agents · Skills ·      │    │
│  │  Providers · Workflows · Auth · Users · Run · Secrets · Audit   │    │
│  └────────────────────────────┬────────────────────────────────────┘    │
│                               │                                         │
│  ┌────────────────────────────▼────────────────────────────────────┐    │
│  │                       CLI LAYER (2 binaries)                      │    │
│  │                                                                  │    │
│  │  cosca (main — incl. chat/exec/mcp/terminal) · cosca-indexer     │    │
│  │                                                                  │    │
│  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────────┐  │    │
│  │  │install │ │ search │ │doctor  │ │ status │ │ knowledge    │  │    │
│  │  │ init   │ │ config │ │runtime │ │ plugin │ │ memory       │  │    │
│  │  │ sync   │ │ cache  │ │editor  │ │update  │ │ serve run    │  │    │
│  │  │ run    │ │ chat   │ │pipeline│ │metrics │ │ benchmark    │  │    │
│  │  └────────┘ └────────┘ └────────┘ └────────┘ └──────────────┘  │    │
│  │                                                                  │    │
│  │  Cobra Commands · Global Flags · Output Formatting              │    │
│  └────────────────────────────┬────────────────────────────────────┘    │
│                               │                                         │
│  ┌────────────────────────────▼────────────────────────────────────┐    │
│  │                     RUNTIME API LAYER                             │    │
│  │                                                                  │    │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────────┐ │    │
│  │  │  Event Bus   │ │State Machine │ │     Lifecycle Manager    │ │    │
│  │  │  Pub/Sub     │ │8 States      │ │Init → Start → Stop hooks │ │    │
│  │  └──────────────┘ └──────────────┘ └──────────────────────────┘ │    │
│  │                                                                  │    │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────────┐ │    │
│  │  │   Metrics    │ │Health Checks │ │   Subsystem Registry    │ │    │
│  │  │  Collectors  │ │Periodic/OnDem│ │8 registered (Knowledge → │ │    │
│  │  └──────────────┘ └──────────────┘ └──────────────────────────┘ │    │
│  └────────────────────────────┬────────────────────────────────────┘    │
│                               │                                         │
│  ┌────────────────────────────▼────────────────────────────────────┐    │
│  │                  AGENT ENGINE (internal/engine)                  │    │
│  │                                                                  │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │  Context │ │ Registry │ │  Router  │ │     Session      │   │    │
│  │  │  Builder │ │Agnt+Skill│ │ (routing)│ │     Manager      │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  │                                                                  │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │ Subagent │ │Knowledge │ │  Memory  │ │   Parser ·       │   │    │
│  │  │ Spawner  │ │ Adapter  │ │  Adapter │ │   Compaction     │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  │                                                                  │    │
│  │  Tool loop: route → build context → LLM → tools → repeat        │    │
│  └────────────────────────────┬────────────────────────────────────┘    │
│                               │                                         │
│  ┌────────────────────────────▼────────────────────────────────────┐    │
│  │                  CHAT SUBSYSTEM (internal/chat)                  │    │
│  │                                                                  │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │ Provider │ │  Tool    │ │   MCP    │ │     Sandbox      │   │    │
│  │  │ Registry │ │ Executor │ │  Client  │ │  (OS-enforced)   │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  │                                                                  │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │Failover  │ │ Hot-     │ │ Chat     │ │  Config ·        │   │    │
│  │  │ Stream   │ │ Reload   │ │ Registry │ │  Types · Ports   │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └────────────────────────────┬────────────────────────────────────┘    │
│                               │                                         │
│  ┌────────────────────────────▼────────────────────────────────────┐    │
│  │                COMPUTE FABRIC (internal/compute)                 │    │
│  │                                                                  │    │
│  │  HardwareProbe (@5s) · Adaptive Scheduler · Work Stealing      │    │
│  │                                                                  │    │
│  │  5 pools: agent 50% · tool 25% · index 12.5% · io 12.5%         │    │
│  │  sandbox 12.5% · CircuitBreaker · RateLimiter · MemoryBudget   │    │
│  │                                                                  │    │
│  │  implements runtime.Subsystem (name: compute-fabric)           │    │
│  └────────────────────────────┬────────────────────────────────────┘    │
│                               │                                         │
│  ┌────────────────────────────▼────────────────────────────────────┐    │
│  │                    KNOWLEDGE ENGINE                               │    │
│  │                                                                  │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │ FTS5     │ │ Vector   │ │Knowledge │ │    Ranking       │   │    │
│  │  │ Full-Text│ │ Similar  │ │  Graph   │ │  Re-ranking      │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  │                                                                  │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │ Indexer  │ │ Chunker  │ │  Parser  │ │  Embeddings      │   │    │
│  │  │ Pipeline │ │Document  │ │Markdown +│ │  Provider        │   │    │
│  │  │          │ │Splitting │ │Code+Ent. │ │  Registry        │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └────────────────────────────┬────────────────────────────────────┘    │
│                               │                                         │
│  ┌────────────────────────────▼────────────────────────────────────┐    │
│  │                   CROSS-CUTTING SUBSYSTEMS                       │    │
│  │                                                                  │    │
│  │  ┌────────────────┐ ┌────────────────┐ ┌────────────────────┐   │    │
│  │  │ DISCOVERY      │ │   MEMORY       │ │     PLUGINS       │   │    │
│  │  │  Engine        │ │   Engine       │ │     System        │   │    │
│  │  │                │ │                │ │                    │   │    │
│  │  │ • Project scan │ │ • 5 layers     │ │ • 4 runtimes      │   │    │
│  │  │ • Editor detect│ │ • 7 types      │ │ • Lifecycle mgmt  │   │    │
│  │  │ • Provider scan│ │ • FTS indexing │ │ • Hook system     │   │    │
│  │  │ • Env discovery│ │ • TTL pruning  │ │ • Event comms     │   │    │
│  │  └────────────────┘ └────────────────┘ └────────────────────┘   │    │
│  │                                                                  │    │
│  │  ┌────────────────┐ ┌────────────────┐ ┌────────────────────┐   │    │
│  │  │ EDITOR ADAPTERS│ │    CACHE       │ │    FILE WATCHER   │   │    │
│  │  │                │ │   Subsystem    │ │    Subsystem      │   │    │
│  │  │ • 8 adapters   │ │ • Multi-level  │ │ • fsnotify        │   │    │
│  │  │ • Auto-detect  │ │ • Memory+SQLite│ │ • Debounce        │   │    │
│  │  │ • Auto-setup   │ │ • TTL eviction │ │ • Recursive       │   │    │
│  │  └────────────────┘ └────────────────┘ └────────────────────┘   │    │
│  └────────────────────────────┬────────────────────────────────────┘    │
│                               │                                         │
│  ┌────────────────────────────▼────────────────────────────────────┐    │
│  │                  SECURITY & SANDBOX (auto-jail)                  │    │
│  │                                                                  │    │
│  │  Self-jailing binary: memfd_create (RAM copy) + Bubblewrap     │    │
│  │  Namespaces: user · pid · uts (+net per constraints)            │    │
│  │  COSCA_JAILED=1 · admin cmds (init/config/version) outside      │    │
│  │  AES-256-GCM config crypto (key from COSCA_REAL_HOSTNAME)       │    │
│  └────────────────────────────┬────────────────────────────────────┘    │
│                               │                                         │
│  ┌────────────────────────────▼────────────────────────────────────┐    │
│  │                    PROVIDERS & INFRASTRUCTURE                     │    │
│  │                                                                  │    │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌─────────────┐  │    │
│  │  │  SQLite    │ │Embeddings  │ │  File      │ │    gRPC     │  │    │
│  │  │  FTS5+vec  │ │Providers   │ │  System    │ │    API      │  │    │
│  │  └────────────┘ └────────────┘ └────────────┘ └─────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## Data Flow

### Installation Flow
```
User runs `cosca install`
  → CLI layer parses flags (--global, --json)
  → Discover editor (OpenCode, Claude Code, etc.)
  → Discover project (language, framework, dependencies)
  → Discover Cosca Global config (~/.config/opencode/cosca)
  → Install editor integration
  → Create plugin directory
  → Initialize runtime
  → Create cache directory
  → Create file index
  → Create SQLite database
  → Build knowledge graph
  → Generate embeddings
  → Configure context
  → Validate installation
  → Run health check
  → Return summary to user
```

### Search Flow
```
User runs `cosca knowledge search "query"`
  → CLI layer formats output
  → Knowledge Engine receives query
  → Phase 1: FTS5 full-text search
  → Phase 2: Vector similarity search
  → Phase 3: Knowledge graph traversal
  → Phase 4: Re-ranking (or score sort)
  → Phase 5: Facet computation
  → Phase 6: Suggestion generation
  → Results returned and formatted
```

### Runtime Startup Flow
```
Runtime.Start() called
  → Transition: UNINITIALIZED → INITIALIZING
  → Init hooks execute (in order):
      1. Knowledge subsystem
      2. Discovery subsystem
      3. Memory subsystem
      4. Cache subsystem
      5. Compute Fabric subsystem
      6. Plugins subsystem
      7. Editors subsystem
      8. Watcher subsystem
  → Transition: INITIALIZING → READY
  → Start hooks execute
  → Transition: READY → RUNNING
  → Health check loop starts
  → Runtime ready for commands
```

### Agent Execution Flow
```
User sends message to Agent Engine
  → Router routes to the appropriate agent (registry lookup)
  → ContextBuilder assembles system prompt + history + tool definitions
  → Engine calls LLM via chat provider (failover across providers)
  → ResponseParser extracts content and tool calls
  → Tool calls execute (ToolExecutor / Compute Fabric pool) or
    SubagentSpawner spawns a subagent for delegated tasks
  → Optional pre-turn memory/knowledge retrieval (adapters)
  → Optional post-turn memory persistence + session save
  → AutoCompaction kicks in when context exceeds threshold
  → Loop repeats until no tool calls, max turns, or error
```

### Auto-Jail Flow
```
User runs `cosca <cmd>` (binary on disk is 644 / not executable)
  → main() detects COSCA_JAILED is unset
  → Reads own binary (/proc/self/exe)
  → Creates executable copy in RAM (memfd_create, fallback /tmp)
  → Re-executes via Bubblewrap with COSCA_JAILED=1
       --ro-bind /usr /lib /lib64 /etc /run
       --bind workspace (ro if constraints.ReadOnly)
       --tmpfs /tmp · --proc /proc · --unshare-user --unshare-pid
       --unshare-uts --hostname cosca-jail (+net per constraints)
  → CLI runs normally inside the jail
  → RAM copy is deleted when the process exits
  → Admin commands (init, config, version) run OUTSIDE the jail
```

---

## Component Interaction

```
┌─────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│   CLI   │────▶│ Runtime  │────▶│Knowledge │────▶│  SQLite  │
│ Command │     │  Engine  │     │  Engine  │     │    DB    │
└─────────┘     └──────────┘     └──────────┘     └──────────┘
                     │                 │
                     ▼                 ▼
               ┌──────────┐     ┌──────────┐
               │  Event   │     │  Vector  │
               │   Bus    │     │  Store   │
               └──────────┘     └──────────┘
                     │
                     ▼
               ┌──────────┐     ┌──────────┐
               │ Compute  │     │  Memory  │
               │  Fabric  │     │  Engine  │
               └──────────┘     └──────────┘
                     │
                     ▼
               ┌──────────┐     ┌──────────┐
               │ Agent    │────▶│   Chat   │
               │  Engine  │     │Subsystem │
               └──────────┘     └──────────┘
                     │                 │
                     ▼                 ▼
               ┌──────────┐     ┌──────────┐
               │Plugins   │     │ Editors  │
               │System    │     │ Adapters │
               └──────────┘     └──────────┘
```

### Key Interaction Patterns

| Pattern | Description | Example |
|---------|-------------|---------|
| **Command → Runtime** | CLI commands invoke runtime methods | `cosca status` → `Runtime.HealthReport()` |
| **Runtime → Subsystem** | Runtime delegates to registered subsystems | Runtime calls `knowledge.Search()` |
| **Event-driven** | State changes publish events | `Runtime.Start()` publishes `EventStartupComplete` |
| **Subsystem → Subsystem** | Subsystems can call each other via runtime API | Knowledge Engine uses Cache subsystem |
| **Plugin → Runtime** | Plugins communicate via PluginContext | Plugin calls `EmitEvent()`, `RegisterHook()` |
| **Engine → Chat** | Agent Engine consumes chat providers/tools via ports | `AgentEngine.Run()` → `chat.GetRegistry().Chat()` |
| **Engine → Memory/Knowledge** | Adapters wire engines to agent context | Pre-turn retrieval via `MemoryRetriever` / `KnowledgeSearcher` |
| **Engine → Compute** | Heavy tool/subagent work enqueued to pools | `Fabric.Submit(ctx, "tool", task)` with backpressure |

---

## Design Principles

| # | Principle | Description |
|---|-----------|-------------|
| 1 | **Modularity** | Every subsystem has a clean interface; implementations are swappable |
| 2 | **Local-first** | All data stored locally in SQLite; zero external service dependencies |
| 3 | **Event-driven** | All state transitions and actions publish events to the event bus |
| 4 | **Observability** | Every operation emits metrics, logs, and health status |
| 5 | **Graceful degradation** | Optional subsystems fail independently; runtime continues |
| 6 | **Security by design** | Self-jailing binary (Bubblewrap + memfd) plus plugin permission model; no arbitrary code execution |
| 7 | **Deterministic state** | Formal state machine with explicit, validated transitions |
| 8 | **Self-contained** | 2 Go binaries (cosca, cosca-indexer); zero runtime deps beyond the OS + bubblewrap |
| 9 | **Cross-platform** | Linux, macOS, Windows; amd64, arm64 |
| 10 | **Testability** | Interface-based design enables mocking and unit testing |

---

## Key Architectural Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | Performance, cross-compilation, single binary, strong concurrency |
| Database | SQLite | Embedded, zero-config, FTS5, vector extension, portable |
| CLI framework | Cobra | De facto standard for Go CLIs, plugin support, auto-completion |
| Plugin runtimes | Go + WASM + External | Flexibility vs. isolation vs. performance trade-off |
| Search strategy | Hybrid (FTS5 + Vector + Graph) | Best accuracy across document types |
| Memory storage | Markdown files + FTS index | Human-readable, git-friendly, searchable |
| Editor integration | Adapter pattern | Consistent API across 8+ editors |
| Event system | In-process pub/sub | Simple, fast, no external dependency needed |
| Agent execution | `internal/engine` (AgentEngine loop) | Core loop: route → context → LLM → tools/subagents → compaction; DI-friendly |
| Chat domain | `internal/chat` (provider/tool/mcp/sandbox) | Provider registry with failover stream; hot-reloadable config |
| Execution resources | `internal/compute` (Compute Fabric) | Adaptive multi-core pools + circuit breaker/rate limit/memory budget |
| Sandboxing | Auto-jail (memfd_create + Bubblewrap) | Self-jailing binary; RAM copy; namespaced user/pid/uts/net |

---

## Performance Characteristics

> **Note**: The latency/throughput figures below are **historical estimates** — they predate the current codebase and re-benchmarking is pending. Recent micro-benchmarks (2026-07-31) measured: runtime state-machine startup ~12µs/op, vector search (10K vectors) ~55ms/op, memory search ~2.7ms/op.

| Operation | Latency (p50) | Latency (p95) | Throughput |
|-----------|--------------|--------------|------------|
| FTS5 search (100K docs) | 5ms | 20ms | 10,000 qps |
| Vector search (100K vectors) | 15ms | 50ms | 2,000 qps |
| Document indexing | 10ms/doc | 50ms/doc | 100 docs/s |
| Full re-index (10K files) | 30s | 60s | — |
| Runtime startup | 200ms | 500ms | — |
| Plugin init (10 plugins) | 100ms | 300ms | — |
| Editor detection | 50ms | 200ms | — |

### Test Coverage (measured 2026-07-31)

Total statement coverage: **76.7%** (`go test ./... -coverprofile`). CI gates: statements ≥70%, branch ≥60%.

| Package | Coverage |
|---------|----------|
| `internal/compute` | 98.7% |
| `internal/chat/executor` | 96.4% |
| `internal/runtime` | 97.4% |
| `internal/registry` | 95.7% |
| `internal/telemetry` | 94.7% |
| `internal/ranking` | 92.6% |
| `internal/chat` | 91.0% |
| `internal/engine` | 83.9% |

---

> **Related**: [Layer Architecture](layers.md) | [Runtime Overview](../runtime/overview.md) | [ADR-001](../adr/ADR-001-cosca-cli-architecture.md)
