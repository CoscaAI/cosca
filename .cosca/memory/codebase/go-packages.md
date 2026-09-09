---
type: codebase
key: go-packages
tags: [go, packages, internal]
timestamp: 2026-07-26T00:00:00Z
status: active
---

# Go Internal Packages Reference

## API Layer (`api/`)
| Package | Files | Purpose |
|---------|-------|---------|
| `api/rest/handler/` | 11 | REST endpoint handlers (agents, skills, providers, workflows, knowledge, memory, auth, users, apikeys, runtime, run) |
| `api/rest/server.go` | 1 | HTTP server setup, routing, middleware chain |
| `api/middleware/` | 4 | CORS, CSRF, logging, rate limiting |
| `api/mcp/server.go` | 1 | MCP protocol server |
| `api/auth/` | 2 | OIDC + RBAC |
| `api/registry/server.go` | 1 | Plugin registry server |

## Core Internal (`internal/`)

### Runtime & Lifecycle
| Package | Files | Purpose |
|---------|-------|---------|
| `internal/runtime/` | 13 | State machine (7 states), event bus, lifecycle manager, daemon, metrics |
| `internal/execution/` | - | Task execution pipeline |

### Knowledge & Search
| Package | Files | Purpose |
|---------|-------|---------|
| `internal/knowledge/` | 3+ | FTS5 full-text search engine |
| `internal/search/` | 4 | Hybrid search (FTS5 + vector) |
| `internal/vector/` | 4 | Vector embeddings |
| `internal/embeddings/` | - | Embedding generation |
| `internal/indexer/` | 2 | File indexer/watcher |
| `internal/parser/` | 2 | File parser |
| `internal/chunker/` | - | Text chunking |
| `internal/graph/` | - | Knowledge graph |
| `internal/ranking/` | - | Search result ranking |

### Memory
| Package | Files | Purpose |
|---------|-------|---------|
| `internal/memory/` | 9 | 5-layer memory engine (short, long, project, pattern, bug) |
| `internal/markdown/` | - | Markdown processing for memory records |
| `internal/sqlite/` | 6 | SQLite driver, migrations, FTS5 |

### Plugins & Extensibility
| Package | Files | Purpose |
|---------|-------|---------|
| `internal/plugins/` | 11 | Plugin system (Go, WASM/wazero, external, shared lib) |
| `internal/providers/` | 16 | LLM/embedding providers (10+ implementations) |
| `internal/editors/` | 11 | Editor adapters (9 editors) |
| `internal/skills/` | - | Skills engine |
| `internal/templates/` | 2 | Template engine |
| `internal/workflows/` | 2 | Workflow engine |
| `internal/prompts/` | - | Prompt management |

### Infrastructure
| Package | Files | Purpose |
|---------|-------|---------|
| `internal/config/` | 5 | Configuration management (Viper-based) |
| `internal/filesystem/` | 3 | File system utilities |
| `internal/telemetry/` | 4 | Metrics, events, reporting |
| `internal/watcher/` | 3 | File system watcher (fsnotify) |
| `internal/cache/` | - | Caching layer |
| `internal/safe/` | - | Thread-safe primitives |
| `internal/updater/` | 3 | Self-update mechanism |
| `internal/diagnostics/` | 3 | System diagnostics (`cosca doctor`) |
| `internal/installers/` | - | Installation helpers |

### CLI
| Package | Files | Purpose |
|---------|-------|---------|
| `internal/cli/` | 54 | All CLI command implementations |
| `internal/discovery/` | 10 | Project/workspace discovery engine |
| `internal/context/` | 5 | Context builder |
| `internal/registry/` | 3 | Plugin/Skill registry |

### AI & Orchestration
| Package | Files | Purpose |
|---------|-------|---------|
| `internal/chat/` | 4 | Chat session management, hot reload |
| `internal/orchestration/` | 21 | Multi-agent orchestration pipeline |
| `internal/agents/` | - | Agent definitions/runtime |
| `internal/adapter/` | - | Provider abstraction adapter |

## Dependencies (go.mod)
| Dependency | Purpose |
|------------|---------|
| `spf13/cobra` | CLI framework |
| `spf13/viper` | Configuration management |
| `rs/zerolog` | Structured logging |
| `modernc.org/sqlite` | Pure Go SQLite (no CGO) |
| `fsnotify/fsnotify` | File system watcher |
| `tetratelabs/wazero` | WASM runtime for plugins |
| `google/uuid` | UUID generation |
| `sergi/go-diff` | Diff engine |
| `mitchellh/go-homedir` | Home directory resolution |
| `golang.org/x/term` | Terminal I/O |
