# Layer Architecture

> **Status**: active | **Owner**: Architecture Chief | **Last Updated**: 2026-07-31

This document describes each architectural layer in detail, including its purpose, responsibilities, interfaces, and dependencies.

---

## Layer Overview

```
┌─────────────────────────────────────────────────────────────────┐
│  LAYER 1: CLI Layer            Interface: cobra.Command          │
├─────────────────────────────────────────────────────────────────┤
│  LAYER 2: Runtime API Layer    Interface: runtime.Runtime        │
├─────────────────────────────────────────────────────────────────┤
│  LAYER 3: Agent Engine         Interface: engine.AgentEngine     │
├─────────────────────────────────────────────────────────────────┤
│  LAYER 4: Chat Subsystem       Interface: chat.ChatProvider      │
├─────────────────────────────────────────────────────────────────┤
│  LAYER 5: Compute Fabric       Interface: runtime.Subsystem      │
├─────────────────────────────────────────────────────────────────┤
│  LAYER 6: Knowledge Engine     Interface: knowledge.Engine       │
├─────────────────────────────────────────────────────────────────┤
│  LAYER 7: Cross-Cutting        Interfaces: Subsystem             │
│  (Discovery, Memory, Plugins, Editors, Cache, Watcher)           │
├─────────────────────────────────────────────────────────────────┤
│  LAYER 8: Infrastructure       SQLite, Filesystem, Embeddings,   │
│                                gRPC, REST, Auth (internal/auth)  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Layer 1: CLI Layer

### Purpose
Provide the command-line interface for all Cosca operations. Parses user input, dispatches commands, and formats output.

### Binaries (2)
| Binary | Package | Purpose |
|--------|---------|---------|
| `cosca` | `cmd/cosca` | Main CLI — full command set (incl. `chat`, `exec`, `mcp`, `terminal`), auto-jail entry point |
| `cosca-indexer` | `cmd/cosca-indexer` | Dedicated batch indexer (TF-IDF local embeddings) |

### Package: `internal/cli/`

**39 command files** (52 non-test `.go` files total including root/utils), including:
- `root.go` — Root command setup, global flags, output formatting
- `install.go` — Full auto-install pipeline (14 steps)
- `search.go` — Knowledge base search
- `knowledge.go` — Knowledge management commands
- `runtime.go` — Runtime daemon commands
- `plugin.go` — Plugin management
- `editor.go` — Editor integration commands
- `doctor.go` — System diagnostics
- `agent.go` — Agent management
- `chat.go` — Interactive chat session
- `run.go` — AI orchestration execution
- `skill.go` — Skill management
- `serve.go` — REST/gRPC server (14120/14121/14122)

### Command Structure
```
cosca [global flags] <command> [subcommand] [flags] [args]
```

### Global Flags
| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | | `""` | Path to config file |
| `--verbose` | `-V` | `false` | Enable verbose output |
| `--quiet` | `-q` | `false` | Suppress non-essential output |
| `--json` | `-j` | `false` | JSON output format |
| `--format` | | `text` | Output format (text, json, yaml, table) |
| `--no-color` | | `false` | Disable colored output |
| `--version` | `-v` | | Show version |

### Dependencies
- Cobra (github.com/spf13/cobra)
- All runtime subsystems (via command implementations)
- Output formatting system

---

## Layer 2: Runtime API Layer

### Purpose
Central orchestrator that manages the application lifecycle, coordinates subsystems, provides health reporting, and enables event-driven communication.

### Package: `internal/runtime/`

**10 source files** (plus extensive tests): `runtime.go`, `state.go`, `lifecycle.go`, `daemon.go`, `metrics.go` (+ `daemon_extended.go`, `lifecycle_extended.go`, `metrics_extended.go`, `state_extended.go`, `coverage_gap_test.go`)

### Core Types

| Type | Description |
|------|-------------|
| `Runtime` | Central orchestrator with subsystem registry |
| `RuntimeState` | State machine: 8 states, formal transitions |
| `Lifecycle` | Phased lifecycle with init/start/stop hooks |
| `EventBus` | In-process pub/sub event system |
| `Metrics` | Metrics collection and export |
| `Daemon` | Background runtime mode |

### State Machine

```
                      ┌────────────┐
                      │Uninitialized│
                      └──────┬─────┘
                             │ Start()
                             ▼
                      ┌────────────┐
                 ┌───▶│Initializing│
                 │    └──────┬─────┘
                 │           │ Init complete
                 │           ▼
                 │    ┌────────────┐
                 │    │   Ready    │
                 │    └──────┬─────┘
                 │           │ Start subsystems
                 │           ▼
              ┌──┴──┐  ┌────────────┐
              │Error│◀─▶│  Running   │
              └──┬──┘  └──────┬─────┘
                 │           │ Stop()
                 │           ▼
                 │    ┌────────────┐
                 │    │  Stopping  │
                 │    └──────┬─────┘
                 │           ▼
                 │    ┌────────────┐
                 └────│  Stopped   │
                      └────────────┘


Additional transitions:
  Running ──→ Recovering ──→ Ready
  Error ──→ Recovering ──→ Ready
  Error ──→ Uninitialized (reset)
```

### Event Types

| Event | Description |
|-------|-------------|
| `EventStateChange` | Runtime state transitioned |
| `EventSubsystemStarted` | Subsystem started |
| `EventSubsystemStopped` | Subsystem stopped |
| `EventSubsystemError` | Subsystem error |
| `EventHealthChange` | Health status changed |
| `EventStartupComplete` | Initialization finished |
| `EventShutdownInitiated` | Shutdown began |
| `EventShutdownComplete` | Shutdown finished |
| `EventConfigReload` | Configuration reloaded |

### Subsystem Interface

```go
type Subsystem interface {
    Name() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health() ComponentStatus
}
```

### Registered Subsystems (8)
1. **Knowledge** — Search, indexing, graph
2. **Discovery** — Project/environment detection
3. **Memory** — Persistent memory layers
4. **Cache** — Multi-level caching
5. **Compute Fabric** — Multi-core adaptive execution (`internal/compute`, implements `runtime.Subsystem`)
6. **Plugins** — Plugin lifecycle management
7. **Editors** — Editor integration adapters
8. **Watcher** — File system notifications

### Lifecycle Phases

| Phase | Hooks | Order |
|-------|-------|-------|
| **Init** | Knowledge → Discovery → Memory → Cache → Compute Fabric → Plugins → Editors → Watcher | Sequential |
| **Start** | Knowledge → Discovery → Memory → Cache → Compute Fabric → Plugins → Editors → Watcher | Sequential |
| **Stop** | Watcher → Editors → Plugins → Compute Fabric → Cache → Memory → Discovery → Knowledge | Reverse |

---

## Layer 3: Agent Engine

### Purpose
The core agent execution loop: routes user input to an agent, builds LLM context, executes tool calls, spawns subagents, and manages session/compaction lifecycle.

### Package: `internal/engine/`

### Core Components

| Component | File | Purpose |
|-----------|------|---------|
| `AgentEngine` | `engine.go` | Core loop: route → context → LLM → tools → subagents → repeat |
| `ContextBuilder` | `context.go` | Assembles system prompt + history + tool definitions |
| `AgentRegistry` | `registry.go` | Loads/dispatches agents from agent definitions (frontmatter) |
| `Router` | `router.go` | Routes user input to the appropriate agent |
| `SessionManager` | `session.go` | Session persistence (JSONL) and retrieval |
| `SubagentSpawner` | `subagent.go` | Spawns subagents for delegated tool work |
| `ResponseParser` | `parser.go` | Streaming parse of content + tool-call deltas |
| `AutoCompaction` | `compaction.go` | Long-context compaction (strip tool outputs, summarize old turns) |
| `MemoryRetriever` / `MemoryStorer` / `KnowledgeSearcher` | `integration.go` | Ports matching orchestration interfaces (no circular imports) |
| `knowledgeAdapter` | `knowledge_adapter.go` | Wraps `knowledge.Engine` for pre-turn retrieval |
| `memoryEngineAdapter` | `memory_adapter.go` | Wraps `memory.MemoryEngine` for pre/post-turn memory |

### Agent Loop

```
User input
  → Router (agent registry lookup)
  → ContextBuilder (system prompt + history + tools)
  → LLM via chat provider (failover across providers)
  → ResponseParser (content + tool calls)
  → Tool calls → ToolExecutor | SubagentSpawner (via Compute Fabric)
  → Optional memory/knowledge retrieval (adapters) + session save
  → AutoCompaction on threshold (default 80% of context window)
  → Repeat until no tool calls, max turns, or error
```

---

## Layer 4: Chat Subsystem

### Purpose
Provides the chat domain: provider registry with failover, tool executor, MCP client, OS-enforced sandbox, hot-reloadable config.

### Package: `internal/chat/`

### Core Components

| Component | Package | Purpose |
|-----------|---------|---------|
| Provider Registry | `provider/` | OpenAI, Anthropic, Ollama providers + registration |
| Tool Executor | `tool/`, `executor/` | Tool registry + executor (filesystem, git, search, shell, mcp, plugin) |
| MCP Client | `mcp/` | Model Context Protocol client |
| Sandbox | `sandbox/` | OS-enforced gates + rails |
| Config | `config/` | Chat config + crypto helpers |
| `failoverStream` | `failover_stream.go` | Failover between providers on stream error |
| `HotReload` | `hotreload.go` | Watch config, reload providers on change |
| `ChatRegistry` | `registry.go` | Global provider registry (singleton via `GetRegistry()`) |
| Ports/Types | `ports.go`, `types.go` | Chat interfaces and message types |

> **Note**: `agent/`, `session/`, `router/`, `memory/`, `context/` are currently **scaffolds** (`.gitkeep` placeholders) — the implemented chat domain lives in the packages above.

---

## Layer 5: Compute Fabric

### Purpose
Multi-core adaptive execution engine. Detects hardware capacity and scales worker pools automatically.

### Package: `internal/compute/`

### Core Components

| Component | File | Purpose |
|-----------|------|---------|
| `Fabric` | `fabric.go` | Orchestrator; implements `runtime.Subsystem` (name: `compute-fabric`) |
| `HardwareProbe` | `hardware.go` | Samples CPU/memory every 5s (`/proc/loadavg`, `/proc/meminfo`) |
| `WorkerPool` | `pool.go` | Dynamic pool with **work stealing** + adaptive scaling |
| `CircuitBreaker` | `backpressure.go` | Per-pool open/close on failure rate |
| `RateLimiter` | `backpressure.go` | Token-bucket rate limiting per pool |
| `MemoryBudget` | `backpressure.go` | Weighted memory allocation (~10MB per weight unit) |
| `ExecutionOrchestrator` | `fabric.go` | High-level patterns: FanOut, Pipeline, MapReduce |

### Worker Pools (5)

| Pool | Share of cores | Ceiling (per Start()) |
|------|----------------|-----------------------|
| `agent` | 50% | 75% of cores, clamp [2,12] |
| `tool` | 25% | 37.5% of cores, clamp [1,6] |
| `index` | 12.5% | 25% of cores, clamp [1,4] |
| `io` | 12.5% | 25% of cores, clamp [1,4] |
| `sandbox` | 12.5% | 25% of cores, clamp [1,4] |

Submit path: circuit-breaker check → rate-limiter check → memory-budget allocate → pool submit → success/failure feedback to breaker.

---

## Layer 6: Knowledge Engine

### Purpose
Provides hybrid search (FTS5 + Vector + Graph), document indexing, entity extraction, and knowledge management.

### Package: `internal/knowledge/`

### Core Subsystems

| Subsystem | Package | Purpose |
|-----------|---------|---------|
| FTS5 Search | `internal/sqlite` | Full-text search with Porter tokenizer |
| Vector Search | `internal/vector` | Vector similarity (sqlite-vec) |
| Knowledge Graph | `internal/graph` | Entity relationship graph |
| Ranking | `internal/ranking` | Result re-ranking |
| Indexing | `internal/indexer` | Document indexing pipeline |
| Chunking | `internal/chunker` | Document splitting |
| Parsing | `internal/parser`, `internal/markdown` | Document parsing |
| Embeddings | `internal/embeddings` | Embedding provider registry |
| Caching | `internal/cache` | Multi-level caching |

### Search Pipeline

```
Query
  │
  ├──▶ FTS5 Search ──▶ SQLite FTS5 (full-text)
  │
  ├──▶ Vector Search ──▶ Embedding → sqlite-vec search
  │
  ├──▶ Graph Search ──▶ Knowledge graph traversal
  │
  └──▶ Re-ranking ──▶ Combined scoring → sorted results
       │
       └──▶ Facets ──▶ Type, language, entity, source counts
            │
            └──▶ Suggestions ──▶ Query suggestions
```

### Key Interfaces

```go
// Search engine
type Engine interface {
    Search(ctx context.Context, params SearchParams) (*SearchResults, error)
    IndexDocument(ctx context.Context, path string) error
    IndexDirectory(ctx context.Context, dir string) error
    Explain(ctx context.Context, resultID string) (*Explanation, error)
    GetStats() (*Stats, error)
    Rebuild(ctx context.Context) error
    Sync(ctx context.Context) (*SyncResult, error)
    Snapshot(name string) (*Snapshot, error)
    Close() error
}
```

---

## Layer 7: Cross-Cutting Subsystems

### 7.1 Discovery Engine

**Package**: `internal/discovery/`

**Purpose**: Detect project type, language, framework, database, editor, AI providers, and environment.

**Discovery Areas**:
| Area | What is detected |
|------|-----------------|
| Project | Language, framework, database, build system, test framework |
| Workspace | Git repo, monorepo detection, sub-projects |
| Editor | OpenCode, Claude Code, Codex, Cursor, VS Code, Neovim |
| Cosca Global | Cosca configuration in ~/.config/opencode/cosca |
| Providers | OpenAI, Anthropic, Google, Azure, Ollama, etc. |
| Plugins | Installed plugins and versions |
| Runtime | Mode, log level, data/cache dirs |
| Environment | OS, shell, terminal, system info |

### 7.2 Memory Engine

**Package**: `internal/memory/`

**Purpose**: Multi-layer persistent memory with file-based storage and FTS-indexed search.

**Memory Layers**:
| Layer | Scope | Persistence | TTL |
|-------|-------|-------------|-----|
| Global | All workspaces | Permanent | — |
| Workspace | Current workspace | Permanent | — |
| Project | Current project | Session+ | Configurable |
| Session | Current session | Ephemeral | Session end |
| Temp | Temporary | Ephemeral | Short TTL |

**Memory Types**:
| Type | Content |
|------|---------|
| Decision | Architecture decisions and rationale |
| Pattern | Working patterns and anti-patterns |
| Bug | Bug reports with root cause and fix |
| Agent | Agent performance and preferences |
| Project | Features, modules, releases |
| Architecture | ADRs, patterns, contracts |
| Session | Active context, current decisions |

### 7.3 Plugin System

**Package**: `internal/plugins/`

**Purpose**: Extend Cosca with plugins written in Go, WASM, or external processes.

**Plugin Runtimes**:
| Runtime | Description | Isolation | Performance |
|---------|-------------|-----------|-------------|
| Go | Native Go plugin | Low | Best |
| WASM | WebAssembly | High | Good |
| External | Child process/gRPC | Highest | Moderate |
| SharedLib | .so/.dll library | High | Good |

**Plugin Lifecycle**: `Installed → Initialized → Started → Stopped`

### 7.4 Editor Adapters

**Package**: `internal/editors/`

**Purpose**: Detect, configure, validate, and manage Cosca integration across editors.

**Supported Editors**:
| Editor | Package | Detection Method |
|--------|---------|-----------------|
| OpenCode | `internal/editors/opencode` | Config file check |
| Claude Code | `internal/editors/claude` | Config file check |
| Codex | `internal/editors/codex` | Config file check |
| Cursor | `internal/editors/cursor` | Config file check |
| VS Code | `internal/editors/vscode` | Settings check |
| Neovim | `internal/editors/neovim` | Plugin check |
| Windsurf | `internal/editors/windsurf` | Config check |
| Zed | `internal/editors/zed` | Settings check |
| Generic MCP | `internal/editors/generic_mcp` | File detection |

### 7.5 Cache Subsystem

**Package**: `internal/cache/`

**Purpose**: Multi-level caching with memory and SQLite backends.

### 7.6 File Watcher

**Package**: `internal/watcher/`

**Purpose**: Monitor filesystem for changes and trigger re-indexing.

---

## Layer 8: Infrastructure

### Purpose
Provide the foundational data storage, embedding, and communication infrastructure.

### SQLite Database
- **Package**: `internal/sqlite/`
- **Features**: FTS5 full-text search, sqlite-vec vector extension
- **Access**: via modernc.org/sqlite (pure Go, no CGo)
- **Schema**: Documents, chunks, entities, vectors, cache, sync_log tables

### Embedding Providers
- **Package**: `internal/embeddings/`
- **Providers**: OpenAI, Anthropic, Ollama, local models
- **Auto-detection**: Scans environment for API keys
- **Dimensions**: 128 (configurable)

### File System
- **Package**: `internal/filesystem/`
- **Operations**: Read, write, watch, walk, hash
- **Patterns**: Glob, recursive, filtered

### gRPC API
- **Package**: `api/grpc/` (+ `api/grpcserver/`)
- **Port**: 14122
- **Services**: Knowledge, Memory, Runtime (per `serve.go`)

### REST API
- **Package**: `api/rest/`
- **Port**: 14120 · Metrics: 14121 (Prometheus) · OpenAPI: `api/rest/openapi.yaml`
- **Routes**: 60 registered (`server.go`), 48 paths documented in OpenAPI
- **Auth**: `internal/auth/` (apikeys, jwt, users) — JWT HS256 + RBAC (admin roles)
- **Domains**: Knowledge, Memory, Runtime, Agents, Skills, Providers, Workflows, Auth, Users, Run, Executions, Secrets, API Keys, Audit, Analytics, Stats, Plugins, WebSocket, Health/Ready

---

## Dependency Graph

```
CLI Layer (cosca — incl. chat/exec/mcp —, cosca-indexer)
  └── Runtime API Layer
        ├── Agent Engine (internal/engine)
        │     ├── Chat Subsystem (internal/chat)
        │     │     ├── Provider Registry
        │     │     ├── Tool Executor / MCP Client
        │     │     └── Sandbox (OS-enforced)
        │     ├── Knowledge Adapter → Knowledge Engine
        │     └── Memory Adapter → Memory Engine
        ├── Compute Fabric (internal/compute)
        │     └── runtime.Subsystem (registered 5th)
        ├── Knowledge Engine
        │     ├── SQLite (FTS5 + Vector)
        │     ├── Embedding Providers
        │     ├── Indexer
        │     └── Cache
        ├── Discovery Engine
        ├── Memory Engine
        │     └── SQLite (FTS)
        ├── Plugin System
        ├── Editor Adapters
        ├── Cache Subsystem
        │     └── SQLite
        └── File Watcher
              └── fsnotify
```

---

## Layer Isolation Rules

| Rule | Description | Violation |
|------|-------------|-----------|
| L-001 | CLI layer must not bypass Runtime API | Direct database access |
| L-002 | Subsystems must not depend on each other directly | Use Runtime as mediator |
| L-003 | Infrastructure must not import from higher layers | No circular deps |
| L-004 | Plugins access only via PluginContext | No direct subsystem access |
| L-005 | All state transitions go through state machine | Direct state mutation |

---

> **Related**: [Architecture Overview](overview.md) | [ADR-001](../adr/ADR-001-cosca-cli-architecture.md)
