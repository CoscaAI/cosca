# Workflow: Cosca Chat — Next-Gen CLI Implementation

> **Status**: Plan | **Author**: Cosca Kernel | **Date**: 2026-07-30
> **Depends on**: `internal/embed/cosca/knowledge/architecture/next-gen-cli-design.md`
> **Don's Order**: "Criar workflow pra implementação e commit"

---

## Visão Geral

Este workflow implementa o Cosca Chat CLI em 6 fases, cada uma com deliverables concretos e verificáveis. O objetivo é ter um CLI funcional, autêntico, e superior ao opencode atual.

**Princípios:**
1. Cada fase produz código testável e commitável
2. Zero regressão no código existente
3. Testes escritos antes ou junto com implementação
4. 100% statement coverage desde a Fase 1

---

## Fase 0: Fundação — Estrutura e Build System

**Duração estimada:** 1 sessão
**Objetivo:** Criar a estrutura de diretórios, build system, e o esqueleto do CLI.

### Tasks

| # | Task | Agent | Deliverable |
|---|------|-------|-------------|
| 0.1 | Criar `cmd/cosca-chat/main.go` com entry point mínimo | cosca-backend | `go build` funciona |
| 0.2 | Criar `internal/chat/` com interfaces core (Agent, Tool, Provider, Memory, Sandbox) | cosca-architecture | `ports.go` com interfaces limpas |
| 0.3 | Criar estrutura de diretórios completa alinhada com design doc | cosca-backend | Tree de diretórios criada |
| 0.4 | Configurar Makefile com targets: build, test, lint, install | cosca-devops | `make build` compila |
| 0.5 | Criar `.cosca/` scaffolding (config.yaml template, AGENTS.md template) | cosca-automation | `cosca init` funcional |
| 0.6 | Escrever testes unitários para o entry point e interfaces | cosca-testing | 100% coverage na Fase 0 |

### Verification
```bash
make build && ./bin/cosca-chat --version
make test && make lint
```

---

## Fase 1: Provider Layer — Multi-Model com Fallback

**Duração estimada:** 1-2 sessões
**Objetivo:** Implementar o sistema de providers com registry, fallback chain, e streaming.

### Tasks

| # | Task | Agent | Deliverable |
|---|------|-------|-------------|
| 1.1 | Implementar `Provider` interface e `ChatProvider` abstraction | cosca-backend | `provider.go` |
| 1.2 | Implementar OpenAI provider (native Go client, streaming) | cosca-provider | `openai/provider.go` |
| 1.3 | Implementar Anthropic provider (native Go client, streaming) | cosca-provider | `anthropic/provider.go` |
| 1.4 | Implementar DeepSeek provider (OpenAI-compatible) | cosca-provider | `deepseek/provider.go` |
| 1.5 | Implementar Ollama provider (local, OpenAI-compatible) | cosca-provider | `ollama/provider.go` |
| 1.6 | Implementar `ChatRegistry` com primary + fallback + hot-reload | cosca-backend | `registry.go` |
| 1.7 | Implementar config loading via YAML (`.cosca/config.yaml`) | cosca-backend | `config.go` |
| 1.8 | Implementar AES-256-GCM crypto para API keys (port do existente) | cosca-security | `crypto.go` |
| 1.9 | Testes unitários para cada provider + registry + config | cosca-testing | 100% coverage |

### Verification
```bash
cosca chat --model deepseek "Hello world"
cosca chat --model ollama/llama3 "Hello world"
# Fallback test: desconecta primary, verifica fallback funciona
```

---

## Fase 2: Tool System — MCP-First com Sandbox

**Duração estimada:** 2-3 sessões
**Objetivo:** Implementar o sistema de tools MCP-first com sandbox OS-enforced.

### Tasks

| # | Task | Agent | Deliverable |
|---|------|-------|-------------|
| 2.1 | Implementar `Tool` interface e `ToolRegistry` | cosca-backend | `tool/registry.go` |
| 2.2 | Implementar MCP client (conexão, descoberta, invocação) | cosca-integrations | `mcp/client.go` |
| 2.3 | Implementar built-in tools como MCP servers internos | cosca-backend | `builtin/` |
| 2.3a | - Filesystem tools (read, write, edit, glob) | cosca-backend | `builtin/filesystem/` |
| 2.3b | - Shell tool (bash, com sandbox) | cosca-backend | `builtin/shell/` |
| 2.3c | - Search tools (grep, semantic search) | cosca-backend | `builtin/search/` |
| 2.3d | - Web tools (fetch, search) | cosca-integrations | `builtin/web/` |
| 2.3e | - Agent tool (spawn subagent) | cosca-backend | `builtin/agent/` |
| 2.3f | - Git tools (commit, diff, branch, PR) | cosca-backend | `builtin/git/` |
| 2.3g | - Memory tools (store, retrieve, forget) | cosca-backend | `builtin/memory/` |
| 2.4 | Implementar Sandbox Gate (bwrap + seccomp) | cosca-security | `sandbox/gate.go` |
| 2.5 | Implementar Permission Classifier (AI-powered) | cosca-security | `sandbox/classifier.go` |
| 2.6 | Implementar Workspace Rails (path validation) | cosca-security | `sandbox/rails.go` |
| 2.7 | Implementar Tool Executor com sandbox enforcement | cosca-backend | `tool/executor.go` |
| 2.8 | Testes unitários + integração para cada tool + sandbox | cosca-testing | 100% coverage |

### Verification
```bash
# Testa cada tool isoladamente
go test ./internal/tool/builtin/... -v
# Testa sandbox (tenta escapar do workspace)
go test ./internal/sandbox/... -v -run TestEscape
# Testa MCP client com um server dummy
go test ./internal/mcp/... -v
```

---

## Fase 2.5: Compute Fabric — Multi-Core Adaptive Execution

**Duração estimada:** 1-2 sessões
**Objetivo:** Implementar o sistema de execução multi-core que detecta hardware, escala workers, e previne gargalos.
**Design doc:** `knowledge/architecture/compute-fabric-design.md`

### Tasks

| # | Task | Agent | Deliverable |
|---|------|-------|-------------|
| 2.5.1 | Implementar `HardwareProbe` — CPU, RAM, GPU, IOPS, load em tempo real | cosca-backend | `internal/compute/hardware.go` |
| 2.5.2 | Implementar `WorkerPool` — pool por tipo (agent, tool, index, io, sandbox) com spawn/kill | cosca-backend | `internal/compute/pool.go` |
| 2.5.3 | Implementar `WorkStealing` — workers ociosos roubam tasks de outras filas | cosca-backend | `internal/compute/steal.go` |
| 2.5.4 | Implementar `AdaptiveScheduler` — escala workers baseado em CPU load, queue depth, latência | cosca-backend | `internal/compute/scheduler.go` |
| 2.5.5 | Implementar `CircuitBreaker` — corta pool após falhas consecutivas, recovery automático | cosca-backend | `internal/compute/breaker.go` |
| 2.5.6 | Implementar `RateLimiter` — token bucket por categoria (LLM calls, tool calls) | cosca-backend | `internal/compute/limiter.go` |
| 2.5.7 | Implementar `MemoryBudget` — alocação/release com limite por tipo de agente | cosca-backend | `internal/compute/budget.go` |
| 2.5.8 | Implementar `Fabric` — integração de todos os componentes como Subsystem do Runtime | cosca-backend | `internal/compute/fabric.go` |
| 2.5.9 | Implementar `ExecutionOrchestrator` — FanOut, Pipeline, Map-Reduce patterns | cosca-backend | `internal/compute/orchestrator.go` |
| 2.5.10 | Integrar no Runtime lifecycle (init/start/stop como Subsystem) | cosca-architecture | hooks no lifecycle |
| 2.5.11 | Testes unitários para cada componente | cosca-testing | 100% coverage |
| 2.5.12 | Testes de concorrência com `-race -count=100` | cosca-testing | Zero race conditions |
| 2.5.13 | Métricas exportáveis + comando `cosca fabric status` | cosca-cli | Tabela ASCII com status |

### Verification
```bash
cosca fabric status  # Mostra hardware, pools, backpressure
go test -race -count=100 ./internal/compute/...  # Zero races
# Stress test: 1000 tasks em 100ms, verificar distribuição entre cores
```

---

## Fase 4: Agent Engine — Core Loop + Subagents

**Duração estimada:** 2-3 sessões
**Objetivo:** Implementar o agent loop principal e o sistema de subagents.

### Tasks

| # | Task | Agent | Deliverable |
|---|------|-------|-------------|
| 3.1 | Implementar `AgentEngine` com o core loop (context→route→call→tools→verify) | cosca-backend | `engine/engine.go` |
| 3.2 | Implementar Context Builder (AGENTS.md chain + MAG + knowledge + skills + tools) | cosca-backend | `engine/context.go` |
| 3.3 | Implementar Router (keyword → semantic search → fallback) | cosca-backend | `engine/router.go` |
| 3.4 | Implementar Response Parser (content + tool_calls streaming) | cosca-backend | `engine/parser.go` |
| 3.5 | Implementar Subagent Spawner (contexto isolado, summary return) | cosca-backend | `engine/subagent.go` |
| 3.6 | Implementar Agent Registry (carrega de `internal/embed/cosca/` + `.cosca/agents/`) | cosca-backend | `engine/registry.go` |
| 3.7 | Implementar Auto-Compaction (context > 80% → summariza) | cosca-backend | `engine/compaction.go` |
| 3.8 | Implementar Session Manager (save/load/resume JSONL) | cosca-backend | `engine/session.go` |
| 3.9 | Testes unitários + integração para o engine completo | cosca-testing | 100% coverage |

### Verification
```bash
cosca exec "Liste os arquivos .go no diretório atual"
cosca exec "Crie um arquivo hello.go com uma função main"
cosca exec --max-turns 5 "Explique o padrão de arquitetura deste projeto"
# Testa subagent spawn
cosca exec "Analise o schema do banco e sugira otimizações" --agent cosca-database
```

---

## Fase 5: TUI & CLI — Interface do Usuário

**Duração estimada:** 1-2 sessões
**Objetivo:** Implementar o Terminal UI com Bubble Tea e todos os comandos CLI.

### Tasks

| # | Task | Agent | Deliverable |
|---|------|-------|-------------|
| 4.1 | Implementar TUI com Bubble Tea (chat view, streaming, scroll) | cosca-frontend | `tui/` |
| 4.2 | Implementar slash commands (/model, /agent, /clear, /compact, /session) | cosca-cli | `tui/commands.go` |
| 4.3 | Implementar input handling (multiline, history, @mentions, !shell) | cosca-cli | `tui/input.go` |
| 4.4 | Implementar comando `cosca chat` (TUI + one-shot mode) | cosca-cli | `cmd/cosca-chat/chat.go` |
| 4.5 | Implementar comando `cosca exec` (non-interactive, CI/CD) | cosca-cli | `cmd/cosca-chat/exec.go` |
| 4.6 | Implementar comando `cosca review` (code review mode) | cosca-cli | `cmd/cosca-chat/review.go` |
| 4.7 | Implementar comando `cosca serve` (API server REST + gRPC) | cosca-cli | `cmd/cosca-chat/serve.go` |
| 4.8 | Implementar `cosca config` (get/set/list) | cosca-cli | `cmd/cosca-chat/config.go` |
| 4.9 | Implementar `cosca doctor` (diagnóstico de ambiente) | cosca-cli | `cmd/cosca-chat/doctor.go` |
| 4.10 | Implementar `cosca init` (scaffolding do projeto) | cosca-cli | `cmd/cosca-chat/init.go` |
| 4.11 | Implementar `cosca mcp` (gerenciar MCP servers) | cosca-cli | `cmd/cosca-chat/mcp.go` |
| 4.12 | Implementar `cosca plugin` (gerenciar plugins WASM) | cosca-cli | `cmd/cosca-chat/plugin.go` |
| 4.13 | Testes de integração para todos os comandos | cosca-testing | 100% coverage |

### Verification
```bash
cosca chat "Explain this codebase"  # TUI abre, streaming funciona
cosca exec "Create a test file" --json  # Saída JSONL correta
cosca review --uncommitted  # Review do código
cosca serve --port 8080 &  # API server sobe
curl localhost:8080/health  # Health check OK
```

---

## Fase 6: Extensibilidade — MCP, Plugins, Hooks

**Duração estimada:** 1-2 sessões
**Objetivo:** Implementar o sistema de extensibilidade completo.

### Tasks

| # | Task | Agent | Deliverable |
|---|------|-------|-------------|
| 5.1 | Implementar MCP Server Manager (add, remove, list, start/stop) | cosca-integrations | `mcp/manager.go` |
| 5.2 | Implementar Plugin System WASM (Wazero runtime) | cosca-plugin | `plugin/` |
| 5.3 | Implementar Skills System (Markdown, progressive disclosure) | cosca-backend | `skills/` |
| 5.4 | Implementar Hooks System (PreToolUse, PostToolUse, etc.) | cosca-backend | `hooks/` |
| 5.5 | Implementar Custom Agents (`.cosca/agents/` directory) | cosca-backend | `agents/custom.go` |
| 5.6 | Implementar Plugin Marketplace client | cosca-plugin | `plugin/marketplace.go` |
| 5.7 | Testes de integração para todo o sistema de extensibilidade | cosca-testing | 100% coverage |

### Verification
```bash
cosca mcp add postgres --command "mcp-server-postgres"
cosca plugin install cosca-linter
cosca exec "Liste as tabelas do banco"  # Usa MCP postgres
```

---

## Fase 7: Polish — Documentação, CI/CD, Release

**Duração estimada:** 1 sessão
**Objetivo:** Documentação completa, CI/CD pipeline, release artifacts.

### Tasks

| # | Task | Agent | Deliverable |
|---|------|-------|-------------|
| 6.1 | Escrever README.md completo (instalação, quickstart, comandos) | cosca-documentation | `README.md` |
| 6.2 | Escrever AGENTS.md template (para `cosca init`) | cosca-documentation | Template |
| 6.3 | Criar CI/CD pipeline (GitHub Actions: test, lint, build, release) | cosca-devops | `.github/workflows/` |
| 6.4 | Criar script de instalação (curl | sh) | cosca-devops | `install.sh` |
| 6.5 | Criar Homebrew formula | cosca-devops | Formula |
| 6.6 | Configurar GoReleaser para multi-platform builds | cosca-devops | `.goreleaser.yaml` |
| 6.7 | Auditoria final de segurança | cosca-security | Relatório |
| 6.8 | Performance benchmark | cosca-performance | Relatório |

### Verification
```bash
# Instalação limpa
curl -fsSL https://cosca.ai/install.sh | sh
cosca --version
cosca doctor  # Tudo verde
```

---

## Resumo de Dependências entre Fases

```
Fase 0 (Fundação)
  └── Fase 1 (Providers)
        └── Fase 2 (Tools + Sandbox)
              ├── Fase 2.5 (Compute Fabric)
              └── Fase 4 (Agent Engine)
                    ├── Fase 5 (TUI + CLI)
                    └── Fase 6 (Extensibilidade)
                          └── Fase 7 (Polish)
```

Nota: Fase 2.5 (Compute Fabric) e Fase 4 (Agent Engine) podem ser feitas em paralelo após Fase 2 — o Fabric é um Subsystem independente do Runtime.

---

## Estrutura de Diretórios (Target)

```
cmd/cosca-chat/
  main.go              # Entry point
  chat.go              # cosca chat
  exec.go              # cosca exec
  review.go            # cosca review
  serve.go             # cosca serve
  config.go            # cosca config
  doctor.go            # cosca doctor
  init.go              # cosca init
  mcp.go               # cosca mcp
  plugin.go            # cosca plugin

internal/
  chat/
    ports.go           # Interfaces: Agent, Tool, Provider, Memory, Sandbox
    config.go          # Config loading + crypto
    registry.go        # ChatRegistry (primary + fallback)
    provider/
      openai/          # OpenAI provider
      anthropic/       # Anthropic provider
      deepseek/        # DeepSeek provider
      ollama/          # Ollama provider
  engine/
    engine.go          # Core agent loop
    context.go         # Context builder
    router.go          # Agent router
    parser.go          # Response parser (streaming)
    subagent.go        # Subagent spawner
    registry.go        # Agent registry (Markdown-native)
    compaction.go      # Auto-compaction
    session.go         # Session save/load/resume
  tool/
    registry.go        # Tool registry (MCP-first)
    executor.go        # Tool executor com sandbox gate
    builtin/
      filesystem/      # read, write, edit, glob
      shell/           # bash com sandbox
      search/          # grep, semantic search
      web/             # fetch, search
      agent/           # spawn subagent
      git/             # commit, diff, branch, PR
      memory/          # store, retrieve, forget
  mcp/
    client.go          # MCP client (connect, discover, invoke)
    manager.go         # MCP server manager
  sandbox/
    gate.go            # Sandbox enforcement gate
    classifier.go      # AI-powered permission classifier
    rails.go           # Workspace path validation
  compute/
    hardware.go        # HardwareProbe (CPU, RAM, GPU, IOPS, load)
    pool.go            # WorkerPool (por tipo: agent, tool, index, io)
    steal.go           # Work stealing entre pools
    scheduler.go       # AdaptiveScheduler (scale up/down)
    breaker.go         # CircuitBreaker por pool
    limiter.go         # RateLimiter token bucket
    budget.go          # MemoryBudget allocate/release
    fabric.go          # Fabric (Subsystem interface)
    orchestrator.go    # FanOut, Pipeline, Map-Reduce patterns
  skills/
    loader.go          # Skills loader (progressive disclosure)
  hooks/
    engine.go          # Hooks engine (lifecycle events)
  plugin/
    runtime.go         # WASM plugin runtime (Wazero)
    marketplace.go     # Plugin marketplace client
  memory/
    mag.go             # Memory-Augmented Generation
    store.go           # Session memory store

tui/
  app.go               # Bubble Tea app
  chat.go              # Chat view
  input.go             # Input handling
  commands.go          # Slash commands

.cosca/
  config.yaml          # Project config
  AGENTS.md            # Project instructions
  sessions/            # Session transcripts (JSONL)
  agents/              # Custom agents
  skills/              # Custom skills
```

---

## Estimativa Total

| Fase | Sessões | Agentes Mobilizados |
|------|---------|---------------------|
| 0: Fundação | 1 | 4 (architecture, backend, devops, testing) |
| 1: Providers | 1-2 | 5 (backend, provider, security, testing) |
| 2: Tools + Sandbox | 2-3 | 6 (backend, integrations, security, testing) |
| 2.5: Compute Fabric | 1-2 | 5 (backend, architecture, cli, testing) |
| 4: Agent Engine | 2-3 | 5 (backend, testing) |
| 5: TUI + CLI | 1-2 | 6 (frontend, cli, backend, testing) |
| 6: Extensibilidade | 1-2 | 5 (integrations, plugin, backend, testing) |
| 7: Polish | 1 | 6 (documentation, devops, security, performance) |
| **Total** | **10-16 sessões** | **13 agentes únicos** |

---

## Go/No-Go Gates

| Gate | Após Fase | Critério |
|------|-----------|----------|
| G1 | Fase 1 | `cosca exec "hello"` funciona com 3 providers |
| G2 | Fase 2 | Tools executam dentro do sandbox, zero escapes |
| G3 | Fase 3 | Subagent spawn funciona, auto-compaction não perde contexto |
| G4 | Fase 4 | TUI responsivo, streaming funciona, todos comandos executam |
| G5 | Fase 5 | MCP server externo funciona, plugin WASM carrega |
| G6 | Fase 6 | `curl install.sh | sh` produz binário funcional |
