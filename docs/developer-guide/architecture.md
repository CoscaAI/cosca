# Developer Guide: Architecture

> **Status**: active | **Owner**: Architecture Chief | **Last Updated**: 2026-07-23

This guide describes the code organization, package structure, design patterns, and coding conventions used in the Cosca project.

---

## Code Organization

```
cosca/
├── cmd/cosca/main.go            # Binary entry point
├── internal/                  # Internal packages
│   ├── cli/                  # Command implementations
│   │   ├── root.go           # Root command
│   │   ├── install.go        # cosca install
│   │   ├── status.go         # cosca status
│   │   ├── doctor.go         # cosca doctor
│   │   ├── knowledge.go      # cosca knowledge subcommands
│   │   ├── memory.go         # cosca memory subcommands
│   │   ├── runtime.go        # cosca runtime subcommands
│   │   ├── plugin.go         # cosca plugin subcommands
│   │   ├── editor.go         # cosca editor subcommands
│   │   └── config.go         # cosca config subcommands
│   ├── runtime/              # Runtime engine
│   │   ├── engine.go         # Runtime interface + implementation
│   │   ├── state.go          # State machine
│   │   ├── events.go         # Event bus
│   │   ├── lifecycle.go      # Lifecycle manager
│   │   ├── health.go         # Health checks
│   │   ├── metrics.go        # Metrics collection
│   │   └── signal.go         # OS signal handling
│   ├── knowledge/            # Knowledge Engine
│   │   ├── engine.go         # KnowledgeEngine interface
│   │   ├── indexer.go        # Indexing pipeline
│   │   ├── searcher.go       # Search pipeline
│   │   ├── chunker.go        # Document chunking
│   │   ├── parser/           # File parsers
│   │   ├── graph/            # Knowledge graph
│   │   └── embeddings/       # Embedding provider abstraction
│   ├── discovery/            # Discovery Engine
│   ├── memory/               # Memory Engine
│   ├── plugins/              # Plugin system
│   │   ├── manager.go        # PluginManager interface
│   │   ├── registry.go       # Plugin registry
│   │   ├── lifecycle.go      # Lifecycle management
│   │   ├── runtimes/         # Runtime adapters
│   │   │   ├── go_runtime.go
│   │   │   ├── wasm_runtime.go
│   │   │   ├── external_runtime.go
│   │   │   └── sharedlib_runtime.go
│   │   └── security.go       # Permission enforcement
│   ├── editors/              # Editor adapters
│   │   ├── manager.go        # EditorManager
│   │   ├── adapter.go        # Editor interface
│   │   └── adapters/         # Per-editor implementations
│   ├── search/               # Hybrid search engine
│   │   ├── engine.go         # SearchEngine interface
│   │   ├── fts5.go           # FTS5 search
│   │   ├── vector.go         # Vector search
│   │   ├── graph_search.go   # Graph traversal
│   │   └── ranker.go         # Result ranking
│   ├── cache/                # Multi-level cache
│   ├── config/               # Configuration management
│   ├── log/                  # Structured logging
│   └── db/                   # Database utilities (SQLite)
├── pkg/                       # Public packages
│   └── plugins/              # Plugin SDK (interfaces for plugin authors)
├── api/                       # API definitions
│   ├── grpc/                 # gRPC proto definitions
│   └── mcp/                  # MCP protocol definitions
└── sdk/                       # SDK implementations
    ├── go/                   # Go SDK
    └── typescript/           # TypeScript SDK
```

---

## Package Dependency Graph

The dependency graph enforces a strict layering to prevent circular dependencies:

```
cmd/cosca
   │
   ▼
internal/cli ──────────────────────────────────────────────┐
   │                                                        │
   ▼                                                        │
internal/runtime                                            │
   │                                                        │
   ├──► internal/knowledge ──► internal/search ──► ...      │
   ├──► internal/memory                                     │
   ├──► internal/discovery                                  │
   ├──► internal/plugins ────► pkg/plugins                  │
   ├──► internal/editors                                    │
   │                                                        │
   ▼                                                        │
internal/{cache, config, log, db}  ◄────────────────────────┘
   │
   ▼
pkg/plugins  (standalone, no internal dependencies)
```

### Dependency Rules

1. **`cmd/`** depends on `internal/cli` only
2. **`internal/cli/`** depends on `internal/runtime` and internal infrastructure
3. **`internal/runtime/`** orchestrates subsystems via interfaces (no direct implementation deps)
4. **Subsystems** depend on infrastructure packages (`cache`, `config`, `log`, `db`) but not on each other
5. **`internal/` never depends on `cmd/`** — the CLI layer is the entry point, never imported
6. **`pkg/`** never depends on `internal/` — public API is fully separated
7. **`api/`** and **`sdk/`** are independent — no dependencies on internal or cmd

> **No circular dependencies are permitted.** CI enforces this with `go vet` and `make check`.

---

## Interface-Based Design

Every subsystem is defined by a Go interface. This enables:
- Mocking for unit tests
- Swappable implementations
- Loose coupling between layers

### Subsystem Interfaces

```go
// Runtime — Central orchestrator
type Runtime interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Restart(ctx context.Context) error
    Status(ctx context.Context) (*HealthReport, error)
    Events() EventBus
}

// KnowledgeEngine — Document indexing and search
type KnowledgeEngine interface {
    Search(ctx context.Context, query Query) (*SearchResult, error)
    Index(ctx context.Context, path string) error
    Sync(ctx context.Context) (*SyncResult, error)
    Stats(ctx context.Context) (*Stats, error)
}

// MemoryEngine — Cross-session context storage
type MemoryEngine interface {
    Store(ctx context.Context, record MemoryRecord) error
    Search(ctx context.Context, query string) ([]MemoryRecord, error)
    Delete(ctx context.Context, key string) error
    Promote(ctx context.Context, key string) error
}

// PluginManager — Plugin lifecycle and management
type PluginManager interface {
    Install(ctx context.Context, source string) (*PluginManifest, error)
    List(ctx context.Context) ([]PluginInfo, error)
    Remove(ctx context.Context, id string) error
    Get(ctx context.Context, id string) (*PluginInfo, error)
}

// EditorAdapter — Editor integration
type EditorAdapter interface {
    Name() string
    Detect() (bool, error)
    Info() (EditorInfo, error)
    Setup(config EditorConfig) error
    Validate() error
    Teardown() error
}
```

### Interface Naming Conventions

| Pattern | Example | When to Use |
|---------|---------|-------------|
| `InterfaceName` | `KnowledgeEngine` | Primary subsystem interfaces |
| `InterfaceNameProvider` | `EmbeddingProvider` | Pluggable service providers |
| `InterfaceNameStore` | `MemoryStore` | Storage backends |
| `InterfaceNameAPI` | `RuntimeAPI` | Limited API exposed to plugins |

---

## Error Handling Conventions

### Sentinel Errors

Define sentinel errors in a dedicated `errors.go` file per package:

```go
package knowledge

import "errors"

var (
    ErrNotFound       = errors.New("knowledge: document not found")
    ErrIndexFailed    = errors.New("knowledge: indexing failed")
    ErrSearchFailed   = errors.New("knowledge: search failed")
    ErrInvalidQuery   = errors.New("knowledge: invalid query")
    ErrEngineStopped  = errors.New("knowledge: engine is stopped")
)
```

### Error Wrapping

Use `fmt.Errorf` with `%w` for wrapping:

```go
func (e *engine) Index(ctx context.Context, path string) error {
    if err := e.indexer.IndexFile(ctx, path); err != nil {
        return fmt.Errorf("knowledge: index %s: %w", path, ErrIndexFailed)
    }
    return nil
}
```

### Error Classification

| Category | Example | Action |
|----------|---------|--------|
| Validation | `ErrInvalidQuery` | Return error to CLI user |
| Transient | `ErrEngineStopped` | Retry after engine starts |
| Permanent | `ErrIndexFailed` | Log and skip |
| Configuration | Missing API key | Suggest setup steps |
| Permission | Plugin denied access | Log security event |

---

## Logging with zerolog

### Configuration

```go
import "github.com/rs/zerolog/log"

// Default: human-readable console output
log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

// JSON output for production
log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
```

### Log Levels

```go
log.Trace().Msg("entering hot path")       // Development only
log.Debug().Str("file", path).Msg("indexing")  // Debug information
log.Info().Int("count", n).Msg("indexed")      // Normal operations
log.Warn().Str("engine", name).Msg("degraded") // Warning conditions
log.Error().Err(err).Msg("index failed")        // Recoverable errors
log.Fatal().Err(err).Msg("cannot start")        // Unrecoverable (os.Exit)
```

### Structured Fields

```go
// Always include relevant context as structured fields
log.Info().
    Str("component", "knowledge").
    Str("operation", "search").
    Str("query", q).
    Int("results", len(results)).
    Dur("duration", elapsed).
    Msg("search completed")
```

---

## Configuration Patterns

### Configuration Loading

Configuration follows a hierarchical loading strategy:

1. Default values (compiled into binary)
2. Config file (`.cosca/config.yaml` or `--config` flag)
3. Environment variables (`COSCA_*` prefix)
4. Command-line flags (highest priority)

```go
type Config struct {
    Runtime RuntimeConfig  `yaml:"runtime" mapstructure:"runtime"`
    Logging LogConfig      `yaml:"logging" mapstructure:"logging"`
    DB      DBConfig       `yaml:"database" mapstructure:"database"`
}

type RuntimeConfig struct {
    Mode       string `yaml:"mode" env:"COSCA_MODE" default:"development"`
    DataDir    string `yaml:"data_dir" env:"COSCA_DATA_DIR"`
    LogLevel   string `yaml:"log_level" env:"COSCA_LOG_LEVEL" default:"info"`
}
```

### Configuration Validation

```go
func (c *Config) Validate() error {
    switch c.Runtime.Mode {
    case "development", "staging", "production":
        // valid
    default:
        return fmt.Errorf("invalid runtime mode: %s", c.Runtime.Mode)
    }
    return nil
}
```

---

## Testing Patterns

### Unit Tests with Mocks

```go
// mock_knowledge.go
type MockKnowledgeEngine struct {
    SearchFunc func(ctx context.Context, query Query) (*SearchResult, error)
}

func (m *MockKnowledgeEngine) Search(ctx context.Context, query Query) (*SearchResult, error) {
    return m.SearchFunc(ctx, query)
}

// Test usage
func TestStatusCommand(t *testing.T) {
    mock := &MockKnowledgeEngine{
        SearchFunc: func(ctx context.Context, query Query) (*SearchResult, error) {
            return &SearchResult{
                Total: 5,
                Hits:  []Hit{{Title: "test", Score: 0.95}},
            }, nil
        },
    }
    // ... test with mock
}
```

### Table-Driven Tests

```go
func TestSearchQueryValidation(t *testing.T) {
    tests := []struct {
        name    string
        query   string
        wantErr bool
    }{
        {"empty query", "", true},
        {"valid query", "authentication", false},
        {"special chars", "auth*test", false},
        {"too long", string(make([]byte, 1001)), true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := engine.Search(context.Background(), Query{Text: tt.query})
            if (err != nil) != tt.wantErr {
                t.Errorf("Search() error = %v, wantErr = %v", err, tt.wantErr)
            }
        })
    }
}
```

---

## Concurrency Model

### Goroutine Safety

All public subsystem methods must be goroutine-safe. Use the following patterns:

```go
type engine struct {
    mu     sync.RWMutex
    state  State
    db     *sql.DB
}

func (e *engine) Search(ctx context.Context, query Query) (*SearchResult, error) {
    e.mu.RLock()
    defer e.mu.RUnlock()
    // ... perform search
}

func (e *engine) Index(ctx context.Context, path string) error {
    e.mu.Lock()
    defer e.mu.Unlock()
    // ... perform indexing
}
```

### Runtime Context

All methods that perform I/O take a `context.Context` as the first parameter. The context carries:

- Cancellation signals (for graceful shutdown)
- Deadlines (for timeouts)
- Request-scoped values (tracing IDs, etc.)

```go
func (e *engine) Search(ctx context.Context, query Query) (*SearchResult, error) {
    // Check context cancellation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    // ... proceed
}
```

### Goroutine Lifecycle

Long-running goroutines should be managed via the runtime:

```go
runtime.Go(func() error {
    // This goroutine is tracked by the runtime
    // It will be cancelled when Stop() is called
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case event := <-eventCh:
            // process event
        }
    }
})
```

---

**Related**: [Getting Started](getting-started.md) | [Testing Guide](testing.md) | [Architecture Overview](../architecture/overview.md) | [ADR-001](../adr/ADR-001-cosca-cli-architecture.md)
