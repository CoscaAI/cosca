# ADR-001: Cosca Enterprise Architecture

> **Status**: Accepted | **Owner**: Architecture Chief | **Last Updated**: 2026-07-23

## Context

The Cosca platform requires a universal runtime that works seamlessly across any editor, programming language, and project type. The runtime must be:

- **Editor-agnostic**: Integrate with OpenCode, Claude Code, VS Code, Cursor, Neovim, Zed, Windsurf, and any MCP-compatible editor
- **Language-agnostic**: Work with Go, TypeScript, Python, Rust, Java, and any other language projects
- **Project-agnostic**: Support single-file scripts to multi-module monorepos
- **Self-contained**: No external database, search service, or runtime dependency
- **Cross-platform**: Run on Linux, macOS, and Windows (amd64 and arm64)
- **Extensible**: Allow third-party plugins without compromising stability

The architecture must support multiple subsystems — Knowledge Engine, Memory Engine, Discovery Engine, Plugin System, Editor Adapters — that communicate and coordinate without tight coupling.

## Decision

We adopt a **layered Go architecture** with independent subsystems communicating through formal interfaces.

### Architecture Layers

```
┌─────────────────────────────────────────────────────────────────────┐
│                         CLI LAYER (Cobra)                            │
│  Command definitions, flag parsing, output formatting, shell comp   │
├─────────────────────────────────────────────────────────────────────┤
│                       RUNTIME API LAYER                              │
│  Event bus, state machine, lifecycle manager, health checks         │
├─────────────────────────────────────────────────────────────────────┤
│                      SUBSYSTEM LAYER                                 │
│  Knowledge  │  Memory  │  Discovery  │  Plugins  │  Editors        │
├─────────────────────────────────────────────────────────────────────┤
│                    CROSS-CUTTING INFRASTRUCTURE                      │
│  SQLite  │  Cache  │  File Watcher  │  Logging  │  Metrics         │
├─────────────────────────────────────────────────────────────────────┤
│                      PROVIDERS LAYER                                 │
│  Embedding providers  │  LLM providers  │  MCP transport           │
└─────────────────────────────────────────────────────────────────────┘
```

### Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go 1.22+ | Single binary, cross-compilation, strong concurrency, excellent stdlib |
| CLI framework | Cobra | De facto standard, plugin support, auto-completion, wide adoption |
| Database | SQLite with FTS5 + vector | Embedded, zero-config, no external service |
| Inter-subsystem comms | Interface contracts + Event bus | Loose coupling, testability, swappable implementations |
| Configuration | YAML + environment variables | Human-readable, version-controllable, 12-factor compliant |
| Logging | zerolog | Zero-allocation, structured, fast |
| Output formats | Text, JSON, YAML, Table | Machine and human readable |

### Interface Contract Pattern

Every subsystem defines a public Go interface. Implementations are injected via dependency injection:

```go
// Example: Knowledge Engine interface
type KnowledgeEngine interface {
    Search(ctx context.Context, query Query) (*SearchResult, error)
    Index(ctx context.Context, path string) error
    Sync(ctx context.Context) (*SyncResult, error)
    Stats(ctx context.Context) (*Stats, error)
}
```

## Rationale

### Why Go?

1. **Single binary deployment** — No JVM, no Node.js runtime, no Python interpreter required
2. **Cross-platform compilation** — `GOOS=linux GOARCH=arm64 go build` produces a native binary
3. **Excellent performance** — Compiled, garbage-collected, with great concurrency primitives
4. **Strong standard library** — HTTP, JSON, file I/O, testing, profiling built-in
5. **Large ecosystem** — Cobra, SQLite drivers, zerolog, Viper for config, testify for testing
6. **Easy to learn** — Shallow learning curve compared to Rust or C++
7. **Excellent tooling** — `go fmt`, `go vet`, `go test -race`, `pprof`, `delve` debugger

### Why Layered Architecture?

1. **Separation of concerns** — CLI parsing is isolated from business logic
2. **Testability** — Each layer can be tested independently with mocked dependencies
3. **Swappable implementations** — The Knowledge Engine can use different backends
4. **Independent development** — Teams can work on different layers simultaneously
5. **Graceful degradation** — A subsystem failure does not crash the entire application

## Alternatives Considered

### TypeScript / Node.js

| Aspect | Assessment |
|--------|------------|
| Pros | Rich ecosystem, familiar to many developers, excellent package management |
| Cons | Requires Node.js runtime, npm/node_modules overhead, single-threaded, slower startup |
| Verdict | Rejected due to runtime dependency and startup latency requirements |

### Rust

| Aspect | Assessment |
|--------|------------|
| Pros | Maximum performance, memory safety, single binary |
| Cons | Steep learning curve, slower development velocity, complex ownership model |
| Verdict | Rejected — team productivity and ecosystem maturity did not justify the performance gain |

### Python

| Aspect | Assessment |
|--------|------------|
| Pros | Rapid prototyping, extensive AI/ML libraries, large community |
| Cons | Poor performance, GIL limitations, no single binary, dependency hell |
| Verdict | Rejected — performance and distribution requirements ruled out interpreted languages |

## Consequences

### Positive

- **Single binary distribution** — Users install one file, no dependencies
- **Modular development** — Teams can develop subsystems independently
- **Excellent testability** — Interface-based design enables comprehensive mocking
- **Cross-platform by default** — Go compilation targets all major OS/arch combinations
- **Fast startup** — Compiled binary starts in milliseconds
- **Extensible via plugins** — Plugin interface allows third-party extensions

### Negative

- **Go ecosystem limitations** — Some libraries (e.g., NLP, advanced AI) have fewer Go bindings
- **Build complexity** — WASM plugin compilation requires additional tooling
- **Plugin isolation** — Go native plugins run in-process; full isolation requires WASM or external processes

### Neutral

- **SQLite dependency** — Embedded database is a file; backups require standard file operations
- **YAML configuration** — Human-friendly but requires validation; mitigated by JSON Schema validation

## Compliance

All layers and subsystems must adhere to the following interface contracts:

| Contract | Layer | File |
|----------|-------|------|
| `KnowledgeEngine` | Subsystem | `internal/knowledge/knowledge.go` |
| `MemoryEngine` | Subsystem | `internal/memory/memory.go` |
| `DiscoveryEngine` | Subsystem | `internal/discovery/discovery.go` |
| `PluginManager` | Subsystem | `internal/plugins/manager.go` |
| `EditorAdapter` | Subsystem | `internal/editors/editor.go` |
| `Runtime` | Runtime API | `internal/runtime/runtime.go` |
| `EventBus` | Runtime API | `internal/runtime/runtime.go` (event bus) |
| `Cache` | Infrastructure | `internal/cache/cache.go` |

Any code that violates these contracts will fail CI/CD quality gates.

---

**Related**: [Architecture Overview](../architecture/overview.md) | [Runtime Overview](../runtime/overview.md) | [ADR-002](ADR-002-knowledge-engine.md) | [ADR-003](ADR-003-plugin-system.md) | [ADR-004](ADR-004-editor-adapters.md)
