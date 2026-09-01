---
type: codebase
key: codebase-overview
tags: [structure, directories, map]
timestamp: 2026-07-28T00:00:00Z
status: active
audited: 2026-07-28
---

# Cosca — Codebase Structure Map

## Top-Level Layout

```
cosca/
├── cmd/cosca/           → Entry point (main.go)
├── pkg/cosca/           → Public Go SDK (24 files)
├── internal/            → Core implementation (41 top-level dirs, 291 Go files)
├── api/                 → API layer (REST + MCP + Registry)
├── web/                 → Next.js 15 Web Console (386 source files)
├── proto/               → gRPC Protobuf definitions (aos/v1/)
├── deploy/              → Deployment (Helm, Terraform, Prometheus)
├── sdk/                 → TypeScript SDK (@cosca/sdk v1.1.0)
├── docs/                → Documentation (ADRs, guides, roadmap, 48 files)
├── test/                → Integration & E2E tests
├── build/               → CI/CD build artifacts
├── examples/            → Example configs
└── .opencode/cosca/     → Cosca Framework (versioned: agents, skills, workflows, engines)
```

## Key Numbers (verified 2026-07-28)

| Metric | Value |
|--------|-------|
| **Go files** | 357 |
| **Internal packages** | 41 top-level, 63 Go packages total |
| **REST endpoints** | 52 (16 domains) |
| **Web routes** | 34 (Next.js App Router page.tsx) |
| **TSX files** | 273 |
| **TS files** | 125 |
| **Total frontend files** | 398 |
| **CLI commands** | 37 root (123 total with subcommands) |
| **LLM providers** | 11 (OpenAI, Anthropic, Ollama, Mistral, Groq, DeepSeek, Google, Azure, Bedrock, Local, OpenAICompat) |
| **Editor adapters** | 10 (OpenCode, Claude, VS Code, Cursor, Neovim, Zed, Windsurf, Codex, IntelliJ, Generic MCP) |
| **Go dependencies** | 12 direct (stdlib-heavy) |

## Entry Points

| Entry | File | Purpose |
|-------|------|---------|
| CLI binary | `cmd/cosca/main.go` | Main CLI entry, signal handling, root command |
| REST server | `api/rest/server.go` | HTTP server on port 14120 |
| MCP server | `api/mcp/server.go` | MCP protocol server (stdin/stdout) |
| Plugin registry | `api/registry/server.go` | Plugin metadata HTTP server |
| Web Console | `web/` | Next.js 15 (pnpm dev, port 3000) |

## Architecture Layers (5)

1. **CLI Layer** — `internal/cli/` (62 Go files, Cobra commands)
2. **Runtime API Layer** — `internal/runtime/` (18 files: state machine, event bus, lifecycle)
3. **Subsystem Layer** — Knowledge, Memory, Discovery, Plugins, Editors, Agents, Workflows, Skills
4. **Cross-Cutting Infrastructure** — SQLite (8 files), Cache, File Watcher, Logging, Metrics, Context
5. **Providers Layer** — 11 LLM/Embedding providers, MCP transport

## Internal Packages (41 top-level)

| Package | Files | Purpose |
|---------|-------|---------|
| `cli` | 62 | Cobra CLI commands (root, serve, run, agent, knowledge, memory, skill, plugin, provider, workflow, index, graph, config, init, health, search, pipeline, chat, metrics) |
| `orchestration` | 22 | AI orchestration engine: chain execution, semantic router, pipeline, tool execution |
| `runtime` | 18 | Runtime daemon, lifecycle, event bus, metrics, state machine |
| `memory` | 15 | Memory engine: store, search, promote, delete, snapshots |
| `plugins` | 12 | Plugin system: manager, loader, lifecycle, hooks, WASM sandbox |
| `discovery` | 10 | Project discovery: editor detection, environment scanning |
| `sqlite` | 8 | SQLite database layer (modernc.org/sqlite) |
| `config` | 7 | Configuration loading/validation (Viper-based) |
| `context` | 6 | Context building, intent detection, optimization |
| `providers` | 5 + 11 subdirs | Provider manager + 11 LLM sub-providers |
| `chat` | 5 | Chat types, hot-reload registry |
| `telemetry` | 5 | Event recording, reporting |
| `search` | 4 | Hybrid search engine (FTS + vector + graph) |
| `vector` | 4 | Vector embeddings storage |
| `editors` | 4 | Editor adapter interfaces |
| `auth` | 4 | JWT auth, user store, API keys |
| `knowledge` | 3 | Knowledge engine (graph RAG, hybrid search) |
| `graph` | 3 | Knowledge graph engine |
| `filesystem` | 3 | Filesystem operations |
| `diagnostics` | 3 | System diagnostics |
| `watcher` | 3 | File system watcher |
| `updater` | 3 | Self-update mechanism |
| `registry` | 3 | Plugin registry |
| Other (≤2 files) | 18 pkgs | agents, skills, workflows, templates, parser, markdown, embeddings, indexer, chunker, cache, audit, ranking, prompts, secrets, safe, embed, adapter, installers |
