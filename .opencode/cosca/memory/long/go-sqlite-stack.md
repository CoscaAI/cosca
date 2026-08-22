---
type: long
key: go-sqlite-stack
tags: [go, sqlite, fts5, stack, cosca]
timestamp: 2026-07-26T00:00:00Z
status: active
agent: Memory Chief
---

# Go + SQLite Knowledge (Cosca Stack)

## Why Go
- Single static binary (no runtime deps)
- Cross-compilation (GOOS/GOARCH)
- Strong stdlib (no framework needed for core)
- Excellent concurrency (goroutines, channels)
- Interface-based design (perfect for provider/adapter pattern)

## Why SQLite
- Embedded (zero config, no server process)
- FTS5 full-text search (built-in, no external service)
- Single file database (easy backup/portability)
- WAL mode for concurrent reads
- Pure Go driver (modernc.org/sqlite — no CGO)

## Go Patterns Used in Cosca

### Interface-Driven Design
```go
// Define interface, implement per-provider/adapter
type Engine interface {
    Start(ctx context.Context) error
    Stop() error
    Health() HealthStatus
}
```

### Registry Pattern
```go
// Auto-discover implementations, lazy-load on demand
type Registry[T any] struct {
    mu      sync.RWMutex
    items   map[string]T
    factory Factory[T]
}
```

### Event Bus
```go
// Internal communication without tight coupling
type EventBus struct {
    subscribers map[string][]chan Event
}
```

### File Watcher (fsnotify)
```go
// Hot-reload without polling
watcher, _ := fsnotify.NewWatcher()
go func() {
    for event := range watcher.Events {
        // Handle create/write/remove
    }
}()
```

## SQLite FTS5 Setup
```sql
CREATE VIRTUAL TABLE knowledge_fts USING fts5(
    title, content, path, language,
    tokenize='porter unicode61'
);
```

## Key Go Dependencies
| Package | Usage |
|---------|-------|
| `spf13/cobra` | CLI framework |
| `spf13/viper` | Config management |
| `modernc.org/sqlite` | Pure Go SQLite |
| `rs/zerolog` | Structured logging |
| `fsnotify` | File watching |
| `wazero` | WASM runtime |
| `google/uuid` | ID generation |
