---
type: pattern
key: go-plugin-wasm-pattern
tags: [go, plugin, wasm, wazero, cosca]
timestamp: 2026-07-26T00:00:00Z
status: active
agent: Architecture Chief
category: design
confidence: 0.90
times_used: 3
---

# Pattern: Go Plugin System with WASM

## Context
Cosca needs a plugin system that supports 3 runtime modes: Go native (compile-time), WASM (sandboxed), and External process (any language).

## Solution
Define a `Plugin` interface and runtime-specific loaders:

```go
// internal/plugins/
type Plugin interface {
    Name() string
    Version() string
    Runtime() RuntimeType  // Go, WASM, External
    Execute(ctx context.Context, input PluginInput) (*PluginOutput, error)
    Validate() error
    Manifest() PluginManifest
}
```

## WASM Runtime (wazero)
```
wazero (tetratelabs/wazero v1.7.0)
  ├── Pure Go, no CGO required
  ├── Cross-platform (Linux, macOS, Windows)
  ├── Sandboxed execution (no filesystem access by default)
  └── Module caching for fast reload
```

## Runtime Comparison
| Feature | Go Native | WASM | External |
|---------|-----------|------|----------|
| Performance | Fastest | Near-native | Slow (IPC) |
| Safety | No sandbox | Full sandbox | OS-level |
| Cross-language | Go only | Any→WASM | Any |
| Hot reload | No (recompile) | Yes | Yes |
| Debug | Full | Limited | Full |

## Benefits
- Users choose safety vs performance
- WASM plugins are cross-platform
- Go native for trusted internal plugins
- External for prototyping in any language
