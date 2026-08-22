---
type: project
key: cosca-overview
tags: [overview, cosca, reference]
timestamp: 2026-07-26T00:00:00Z
status: active
agent: Kernel
---

# Cosca — Complete Project Reference

## Identity
- **Name**: Cosca (AI Orchestration System)
- **Binary**: `cosca`
- **Module**: `github.com/CoscaAI/cosca`
- **Language**: Go 1.22
- **License**: MIT
- **SLOC**: ~15K Go + ~11K TypeScript

## Core Capabilities

### 1. Knowledge Engine
- FTS5 full-text search (SQLite)
- Vector similarity search (embeddings)
- Knowledge graph (entity relationships)
- File indexer with hot-reload watcher
- Hybrid search ranking

### 2. Memory Engine
- 5 layers: short, long, project, pattern, bug
- 7 memory types: agent, architecture, bug, decision, pattern, project, session
- File-based storage (markdown + SQLite)
- Cross-session persistence

### 3. Discovery Engine
- 8 discovery areas (framework, language, database, etc.)
- Automatic project classification
- Stack detection from filesystem

### 4. Plugin System
- 3 runtimes: Go native, WASM (wazero), External process
- Plugin SDK specification
- Registry server

### 5. Editor Adapters
- 9 supported editors: OpenCode, Claude Code, VS Code, Cursor, Neovim, Zed, Windsurf, Codex, MCP Generic
- MCP protocol support
- Automatic context injection

### 6. LLM Providers
- 10+ providers: OpenAI, Anthropic, Ollama, Mistral, Groq, DeepSeek, Google, Azure, Bedrock, Local
- Rate limiting, retry, fallback
- Streaming support

### 7. REST API (36 endpoints)
- `/v1/knowledge/*` — search, index, stats, sync
- `/v1/memory/*` — CRUD across layers, search
- `/v1/runtime/*` — state, health, metrics
- `/v1/agents/*` — list, get, capabilities
- `/v1/skills/*` — list, get, validate
- `/v1/providers/*` — list, get, test, config
- `/v1/workflows/*` — list, get, run
- `/v1/auth/*` — login, refresh, me
- `/v1/users/*` — CRUD (admin)
- `/v1/apikeys/*` — generate, list, revoke (admin)

### 8. Web Console (Next.js 15)
- 17 routes, 12 feature modules
- JWT auth with refresh tokens
- RBAC (admin, editor, viewer)
- Shadcn/UI + Tailwind
- WCAG AA+ accessibility
- CSRF protection, ISR

### 9. CLI (34+ commands)
- `cosca install`, `cosca search`, `cosca doctor`
- `cosca runtime`, `cosca plugin`, `cosca serve`
- `cosca workflow`, `cosca skill`, `cosca agent`
- `cosca knowledge`, `cosca memory`, `cosca context`

### 10. Deployment
- Docker (multi-stage, scratch-based Go image)
- Docker Compose (Go API + Next.js frontend)
- Helm chart (Kubernetes)
- Terraform (AWS ECS)
- GitHub Actions CI/CD
