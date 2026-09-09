---
type: dependency
key: go-module-graph
tags: [dependencies, go, modules, graph]
timestamp: 2026-07-26T00:00:00Z
status: active
---

# Go Internal Package Dependencies

## Core Dependency Flow
```
cmd/cosca → pkg/cosca → internal/* → api/*
                           ↓
                      SQLite (modernc.org/sqlite)
```

## Key Internal Dependencies

### Runtime → Subsystems
```
internal/runtime/
  → internal/knowledge/    (search, index)
  → internal/memory/        (memory engine)
  → internal/discovery/     (project discovery)
  → internal/plugins/       (plugin manager)
  → internal/editors/       (editor manager)
  → internal/config/        (configuration)
  → internal/telemetry/     (metrics, events)
  → internal/sqlite/        (database)
  → internal/filesystem/    (file operations)
  → internal/watcher/       (file watching)
```

### API Layer
```
api/rest/handler/  → internal/knowledge/, internal/memory/, internal/runtime/, ...
api/middleware/     → independent (CORS, CSRF, logging, rate limit)
api/auth/           → independent (JWT, RBAC, OIDC)
api/mcp/            → internal/editors/, internal/runtime/
api/registry/       → internal/plugins/, internal/registry/
```

### Providers
```
internal/providers/
  ├── openai/       → internal/providers/ (common interface)
  ├── anthropic/    → internal/providers/
  ├── ollama/       → internal/providers/
  ├── mistral/      → internal/providers/
  ├── groq/         → internal/providers/
  ├── deepseek/     → internal/providers/
  ├── google/       → internal/providers/
  ├── azure/        → internal/providers/
  ├── bedrock/      → internal/providers/
  └── local/        → internal/providers/
```

### Editor Adapters
```
internal/editors/
  ├── opencode/     → internal/editors/ (common interface)
  ├── claude/       → internal/editors/
  ├── vscode/       → internal/editors/
  ├── cursor/       → internal/editors/
  ├── neovim/       → internal/editors/
  ├── zed/          → internal/editors/
  ├── windsurf/     → internal/editors/
  ├── codex/        → internal/editors/
  └── generic_mcp/  → internal/editors/
```

## Heavy Dependencies (impact analysis)
| Package | Files | Dependents (imported by) |
|---------|-------|--------------------------|
| internal/cli/ | 54 | cmd/cosca |
| internal/orchestration/ | 21 | internal/cli/, api/rest/handler/ |
| internal/runtime/ | 13 | cmd/cosca, api/*, internal/cli/ |
| internal/providers/ | 16 | internal/chat/, internal/orchestration/ |
| internal/plugins/ | 11 | internal/runtime/ |

## Zero-Dependency Packages (leaf nodes)
`internal/safe/`, `internal/embedfs/`, `internal/vector/`, `internal/parser/`, `internal/chunker/`, `api/middleware/*`

## Circular Dependencies
**None detected** — architecture enforces strict layer hierarchy.
