---
type: dependency
key: external-dependencies
tags: [dependencies, go-mod, external]
timestamp: 2026-07-26T00:00:00Z
status: active
---

# External Go Module Dependencies

## Direct Dependencies (go.mod)
| Module | Version | Purpose | Risk |
|--------|---------|---------|------|
| spf13/cobra | v1.8.0 | CLI framework | Low (stable, widely used) |
| spf13/viper | v1.19.0 | Config management | Low |
| rs/zerolog | v1.33.0 | Structured logging | Low |
| modernc.org/sqlite | v1.29.0 | Pure Go SQLite | Medium (CGO-free but complex) |
| fsnotify/fsnotify | v1.7.0 | File system watcher | Low |
| tetratelabs/wazero | v1.7.0 | WASM runtime | Medium (evolving API) |
| google/uuid | v1.6.0 | UUID generation | Low |
| sergi/go-diff | v1.3.1 | Diff engine | Low |
| mitchellh/go-homedir | v1.1.0 | Home directory | Low |
| golang.org/x/term | v0.28.0 | Terminal I/O | Low |
| golang.org/x/mod | v0.17.0 | Module versioning | Low |
| gopkg.in/yaml.v3 | v3.0.1 | YAML parsing | Low |

## Key Design Principle
**Zero CGO dependencies** — `modernc.org/sqlite` is a pure Go SQLite implementation (no CGO required). This enables:
- Trivial cross-compilation (GOOS/GOARCH)
- Smaller binary size
- No libc dependency
- Faster build times
