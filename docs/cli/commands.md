# Command Reference

> **Status**: active | **Owner**: CLI Chief | **Last Updated**: 2026-07-23

This document provides the complete reference for all Cosca commands, organized by category.

---

## Core Commands

### `cosca install`

Perform a full auto-install of Cosca in the current project.

```
Usage:
  cosca install [flags]

Flags:
  --global     Install globally instead of locally
  --json       Output in JSON format

Description:
  Runs the complete 14-step installation pipeline:
  1. Discover editor
  2. Discover project environment
  3. Discover Cosca Global configuration
  4. Install editor integration
  5. Create plugins directory
  6. Create runtime configuration
  7. Create cache directory
  8. Create file index
  9. Create SQLite database
  10. Build knowledge graph
  11. Generate embeddings
  12. Configure context
  13. Validate installation
  14. Run health check

Example:
  cosca install              # Install in current project
  cosca install --global     # Install globally for all projects
  cosca install --json       # JSON output for scripting
```

### `cosca init`

Initialize Cosca in the current project (lighter than install).

```
Usage:
  cosca init [flags]

Example:
  cosca init
```

### `cosca status`

Show system status and health information.

```
Usage:
  cosca status [flags]

Flags:
  --watch     Continuously watch status (refresh every 5s)

Example:
  cosca status              # Show status
  cosca status --json       # JSON output
  cosca status --watch      # Live status updates
```

### `cosca doctor`

Run full system diagnostics.

```
Usage:
  cosca doctor [flags]

Flags:
  --fix       Attempt to fix detected issues

Example:
  cosca doctor              # Run diagnostics
  cosca doctor --fix        # Run diagnostics and fix issues
  cosca doctor --json       # JSON output
```

### `cosca version`

Show version information.

```
Usage:
  cosca version [flags]

Flags:
  --full      Show full version details

Example:
  cosca version             # Short version
  cosca version --full      # Full version with build info
  cosca version --json      # JSON output
```

### `cosca sync`

Synchronize all subsystems with the filesystem.

```
Usage:
  cosca sync [flags]

Description:
  Detects added, modified, and removed files, then updates
  the knowledge index, memory, and graph accordingly.

Example:
  cosca sync
  cosca sync --verbose
```

### `cosca update`

Update Cosca to the latest version.

```
Usage:
  cosca update [flags]

Example:
  cosca update
  cosca update --channel beta
```

### `cosca upgrade`

Upgrade project configuration to the latest schema version.

```
Usage:
  cosca upgrade [flags]

Example:
  cosca upgrade
```

### `cosca uninstall`

Remove Cosca from the current project.

```
Usage:
  cosca uninstall [flags]

Flags:
  --global    Remove global installation
  --force     Skip confirmation prompts

Example:
  cosca uninstall
  cosca uninstall --global
  cosca uninstall --force
```

---

## Knowledge Commands

### `cosca knowledge search`

Search the knowledge base using hybrid search.

```
Usage:
  cosca knowledge search <query> [flags]

Flags:
  --limit int         Max results (default: 20)
  --offset int        Results to skip
  --type strings      Filter by type (document, chunk, entity, code)
  --path string       Filter by path prefix
  --tag strings       Filter by metadata tags (key=value)
  --since duration    Filter by recency (e.g., 7d, 24h)
  --min-score float   Minimum score threshold
  --graph             Enable graph search
  --facets            Show faceted counts
  --json              JSON output

Example:
  cosca knowledge search "authentication"
  cosca knowledge search "database" --type document
  cosca knowledge search "API" --type code --tag language=go
  cosca knowledge search "query" --graph --facets
  cosca knowledge search "query" --limit 50 --min-score 0.8
```

### `cosca knowledge index`

Index files into the knowledge base.

```
Usage:
  cosca knowledge index [path...] [flags]

Flags:
  --recursive     Index directories recursively
  --watch         Watch for changes after indexing

Example:
  cosca knowledge index file.md
  cosca knowledge index ./docs --recursive
  cosca knowledge index ./src --watch
```

### `cosca knowledge graph`

Manage the knowledge graph.

```
Usage:
  cosca knowledge graph [command] [flags]

Commands:
  stats           Show graph statistics
  node <id>       Show entity details
  neighbors <id>  Show entity neighbors
  viz <id>        Visualize graph (text-based)
  export          Export graph (json, dot)

Example:
  cosca knowledge graph stats
  cosca knowledge graph node "node-abc123"
  cosca knowledge graph viz "Runtime" --depth 2
  cosca knowledge graph export --format dot > graph.dot
```

### `cosca knowledge stats`

Show knowledge engine statistics.

```
Usage:
  cosca knowledge stats [flags]

Example:
  cosca knowledge stats
  cosca knowledge stats --json
```

### `cosca knowledge explain`

Explain why a search result was returned.

```
Usage:
  cosca knowledge explain <result-id>

Example:
  cosca knowledge explain "abc123"
```

### `cosca knowledge sync`

Synchronize the knowledge index with the filesystem.

```
Usage:
  cosca knowledge sync [flags]

Example:
  cosca knowledge sync
  cosca knowledge sync --verbose
```

### `cosca knowledge rebuild`

Rebuild all knowledge indexes from scratch.

```
Usage:
  cosca knowledge rebuild [flags]

Example:
  cosca knowledge rebuild
```

### `cosca knowledge snapshot`

Create a point-in-time snapshot of the knowledge engine.

```
Usage:
  cosca knowledge snapshot <name> [flags]

Example:
  cosca knowledge snapshot "pre-refactor-backup"
```

### `cosca knowledge verify`

Verify the integrity of the knowledge engine.

```
Usage:
  cosca knowledge verify [flags]

Example:
  cosca knowledge verify
```

### `cosca knowledge vacuum`

Clean stale data and reclaim database space.

```
Usage:
  cosca knowledge vacuum

Example:
  cosca knowledge vacuum
```

---

## Memory Commands

### `cosca memory store`

Store a memory record.

```
Usage:
  cosca memory store [flags]

Flags:
  --type string       Memory type: decision, pattern, bug, agent, project, architecture, session
  --layer string      Memory layer: global, workspace, project, session, temp
  --scope string      Optional scope identifier
  --content string    Memory content (markdown)
  --priority int      Priority for search ranking (higher = more important)
  --ttl duration      Time-to-live (e.g., 24h, 7d)
  --metadata string   Comma-separated key=value pairs

Example:
  cosca memory store \
    --type decision \
    --layer project \
    --scope backend \
    --priority 8 \
    --content "Use PostgreSQL with connection pooling" \
    --metadata "author=developer,tags=database"
```

### `cosca memory search`

Search memory records.

```
Usage:
  cosca memory search <query> [flags]

Flags:
  --type strings      Filter by memory type
  --layer strings     Filter by memory layer
  --limit int         Max results (default: 20)

Example:
  cosca memory search "authentication"
  cosca memory search "database" --type decision
  cosca memory search "" --layer session --limit 10
```

### `cosca memory get`

Retrieve a memory record by ID.

```
Usage:
  cosca memory get <id>

Example:
  cosca memory get "abc123-def456"
```

### `cosca memory delete`

Delete a memory record.

```
Usage:
  cosca memory delete <id>

Example:
  cosca memory delete "abc123-def456"
```

### `cosca memory promote`

Promote a memory record to a higher layer.

```
Usage:
  cosca memory promote <id> <from-layer> <to-layer>

Example:
  cosca memory promote "abc123" session project
  cosca memory promote "def456" project global
```

### `cosca memory prune`

Remove expired memory records.

```
Usage:
  cosca memory prune [flags]

Flags:
  --layer string     Prune specific layer (default: all)

Example:
  cosca memory prune
  cosca memory prune --layer session
```

### `cosca memory stats`

Show memory layer statistics.

```
Usage:
  cosca memory stats [flags]

Example:
  cosca memory stats
  cosca memory stats --json
```

### `cosca memory snapshot`

Create a memory snapshot.

```
Usage:
  cosca memory snapshot <name>

Example:
  cosca memory snapshot "weekly-backup"
```

---

## Runtime Commands

### `cosca runtime start`

Start the runtime daemon.

```
Usage:
  cosca runtime start [flags]

Flags:
  --daemon     Run as background daemon (default: foreground)

Example:
  cosca runtime start              # Foreground
  cosca runtime start --daemon     # Background daemon
```

### `cosca runtime stop`

Stop the runtime daemon.

```
Usage:
  cosca runtime stop [flags]

Flags:
  --force      Force stop (SIGTERM instead of SIGINT)

Example:
  cosca runtime stop
  cosca runtime stop --force
```

### `cosca runtime status`

Show runtime daemon status.

```
Usage:
  cosca runtime status [flags]

Example:
  cosca runtime status
  cosca runtime status --json
```

### `cosca runtime restart`

Restart the runtime daemon.

```
Usage:
  cosca runtime restart [flags]

Example:
  cosca runtime restart
```

---

## Plugin Commands

### `cosca plugin install`

Install a plugin from a path or URL.

```
Usage:
  cosca plugin install <source> [flags]

Flags:
  --file       Install from local file path
  --url        Install from remote URL
  --registry   Install from plugin registry

Example:
  cosca plugin install ./my-plugin/
  cosca plugin install https://plugins.cosca.dev/my-plugin.tar.gz
  cosca plugin install my-plugin --registry
```

### `cosca plugin list`

List installed plugins.

```
Usage:
  cosca plugin list [flags]

Example:
  cosca plugin list
  cosca plugin list --json
  cosca plugin list --format table
```

### `cosca plugin info`

Show detailed information about a plugin.

```
Usage:
  cosca plugin info <id>

Example:
  cosca plugin info my-plugin
```

### `cosca plugin remove`

Remove an installed plugin.

```
Usage:
  cosca plugin remove <id>

Example:
  cosca plugin remove my-plugin
```

### `cosca plugin enable`

Enable a disabled plugin.

```
Usage:
  cosca plugin enable <id>

Example:
  cosca plugin enable my-plugin
```

### `cosca plugin disable`

Disable an enabled plugin.

```
Usage:
  cosca plugin disable <id>

Example:
  cosca plugin disable my-plugin
```

### `cosca plugin update`

Update a plugin to the latest version.

```
Usage:
  cosca plugin update <id>

Example:
  cosca plugin update my-plugin
```

### `cosca plugin validate`

Validate a plugin manifest.

```
Usage:
  cosca plugin validate <path>

Example:
  cosca plugin validate ./my-plugin/plugin.yaml
```

---

## Editor Commands

### `cosca editor detect`

Detect the active editor.

```
Usage:
  cosca editor detect [flags]

Example:
  cosca editor detect
  cosca editor detect --verbose
```

### `cosca editor setup`

Setup Cosca integration for a specific editor.

```
Usage:
  cosca editor setup [editor-name] [flags]

Flags:
  --mcp-only    Only configure MCP server

Example:
  cosca editor setup                    # Auto-detect and setup
  cosca editor setup opencode           # Setup for OpenCode
  cosca editor setup vscode --mcp-only  # MCP only for VS Code
```

### `cosca editor validate`

Validate Cosca integration for an editor.

```
Usage:
  cosca editor validate [editor-name]

Example:
  cosca editor validate opencode
  cosca editor validate vscode
```

### `cosca editor teardown`

Remove Cosca integration from an editor.

```
Usage:
  cosca editor teardown [editor-name]

Example:
  cosca editor teardown opencode
```

### `cosca editor list`

List all supported editors and their detection status.

```
Usage:
  cosca editor list [flags]

Example:
  cosca editor list
  cosca editor list --json
```

---

## Config Commands

### `cosca config show`

Show the current configuration.

```
Usage:
  cosca config show [flags]

Example:
  cosca config show
  cosca config show --json
  cosca config show --format yaml
```

### `cosca config init`

Initialize the default configuration.

```
Usage:
  cosca config init [flags]

Example:
  cosca config init
```

### `cosca config validate`

Validate the current configuration.

```
Usage:
  cosca config validate

Example:
  cosca config validate
```

---

## Other Commands

### `cosca search`

Quick search alias for `cosca knowledge search`.

```
Usage:
  cosca search <query> [flags]

Example:
  cosca search "authentication"
```

### `cosca cache clear`

Clear cached data.

```
Usage:
  cosca cache clear [flags]

Example:
  cosca cache clear
```

### `cosca cache stats`

Show cache statistics.

```
Usage:
  cosca cache stats [flags]

Example:
  cosca cache stats
```

### `cosca context show`

Show the current context.

```
Usage:
  cosca context show [flags]

Example:
  cosca context show
```

### `cosca context build`

Rebuild the context.

```
Usage:
  cosca context build

Example:
  cosca context build
```

### `cosca provider list`

List available AI providers.

```
Usage:
  cosca provider list [flags]

Example:
  cosca provider list
```

### `cosca provider set`

Set the active provider.

```
Usage:
  cosca provider set <name>

Example:
  cosca provider set openai
```

### `cosca benchmark`

Run performance benchmarks.

```
Usage:
  cosca benchmark [flags]

Example:
  cosca benchmark
  cosca benchmark --json
```

### `cosca validate`

Validate the project configuration.

```
Usage:
  cosca validate [flags]

Example:
  cosca validate
```

### `cosca completion`

Generate shell completion scripts.

```
Usage:
  cosca completion <shell>

Shells:
  bash          Bash completion
  zsh           Zsh completion
  fish          Fish completion
  powershell    PowerShell completion

Example:
  cosca completion bash
  cosca completion zsh | sudo tee /usr/share/zsh/site-functions/_aos
```

### `cosca health`

Run health checks.

```
Usage:
  cosca health [flags]

Example:
  cosca health
  cosca health --json
```

### `cosca docs`

Open the Cosca documentation.

```
Usage:
  cosca docs [flags]

Example:
  cosca docs
```

### `cosca workflow list`

List available workflows.

```
Usage:
  cosca workflow list [flags]

Example:
  cosca workflow list
```

### `cosca template list`

List available templates.

```
Usage:
  cosca template list [flags]

Example:
  cosca template list
```

### `cosca agent`

Manage agents.

```
Usage:
  cosca agent [command] [flags]

Commands:
  list    List agents
  info    Show agent details

Example:
  cosca agent list
```

### `cosca skill`

Manage skills.

```
Usage:
  cosca skill [command] [flags]

Commands:
  list        List skills
  show        Show skill details
  search      Search skills
  install     Install a skill
  use         Register skill usage
  status      Skill usage/state table
  validate    Validate skills against the Agent Skills standard
  migrate     Generate Agent Skills frontmatter for legacy skills
  curator     List (--dry-run) or archive (--archive) stale skills
  restore     Restore an archived skill

Example:
  cosca skill list
  cosca skill validate
  cosca skill migrate --dry-run
```

### `cosca prompt`

Manage prompts.

```
Usage:
  cosca prompt [command] [flags]

Commands:
  list        List prompts
  info        Show prompt details

Example:
  cosca prompt list
```

### `cosca bootstrap`

Run the bootstrap process.

```
Usage:
  cosca bootstrap [flags]

Description:
  Initializes all subsystems needed for Cosca to function:
  - Load configuration
  - Initialize memory stores
  - Build context
  - Prepare runtime
  - Run workspace discovery

Example:
  cosca bootstrap
  cosca bootstrap --json
```

---

## AI Orchestration Commands (NEW in v1.3.0)

### `cosca run`

Execute a prompt through the AI Orchestration Engine.

```
Usage:
  cosca run <prompt> [flags]

Flags:
  --agent string       Explicitly specify the agent (e.g. "Backend Chief")
  --provider string    Chat provider to use (default: auto-detect)
  --model string       Model name to use
  --stream             Enable streaming output
  --no-mag             Disable Memory-Augmented Generation
  --semantic           Enable semantic (embedding-based) agent routing
  --metrics            Show execution metrics after completion
  --dry-run            Show pipeline plan without executing LLM calls

Description:
  The engine: (1) searches knowledge and memory for context,
  (2) routes to the best agent, (3) executes via LLM provider,
  (4) returns the agent's response.

Example:
  cosca run "build an authentication API"
  cosca run --agent "Backend Chief" "design a database schema"
  cosca run --provider openai --model gpt-4 "explain microservices"
  cosca run --stream "tell me a story"
  cosca run --semantic "modelar as tabelas"
```

### `cosca chat`

Start an interactive AI chat session.

```
Usage:
  cosca chat [flags]

Flags:
  --agent string       Initial agent for the session
  --provider string    Initial chat provider
  --model string       Initial model name
  --stream             Enable streaming by default
  --metrics            Show metrics after each response

Session Commands:
  /agent <name>        Switch to a specific agent
  /provider <name>     Switch LLM provider
  /model <name>        Switch model
  /stream              Toggle streaming mode
  /metrics             Show session metrics
  /history             Show conversation history
  /clear               Clear conversation history
  /help                Show available commands
  /exit, /quit         End the session

Example:
  cosca chat
  cosca chat --agent "Backend Chief"
  cosca chat --stream --metrics
```

### `cosca exec`

Execute a non-interactive prompt (CI/CD).

> **AVISO**: este comando vive no binário único `cosca` e está
> **DESATIVADO POR PADRÃO** (feature gate). Sem a env var
> `COSCA_ENABLE_EXEC=1` (ou `true`) o comando falha de forma fail-closed e não
> executa nenhum trabalho.

```
Usage:
  cosca exec <prompt> [flags]

Flags:
  --model string         LLM model (default "deepseek")
  --max-turns int        Maximum agent turns (default 10)
  --json                 Output as JSON
  --session string       Session ID to persist conversation history ({id}.jsonl under .cosca/sessions/)
  --clear-session        Delete the session before creating a new one
  --agent string         Agent to use (default "cosca-kernel")

Example:
  COSCA_ENABLE_EXEC=1 cosca exec "explique o que é cosca"
  COSCA_ENABLE_EXEC=1 cosca exec "oi" --model none        # modo determinístico
```

### `cosca mcp`

Manage MCP servers.

> **AVISO**: este comando vive no binário único `cosca` e está
> **DESATIVADO POR PADRÃO** (feature gate). Sem a env var
> `COSCA_ENABLE_MCP=1` (ou `true`) o comando falha de forma fail-closed e não
> executa nenhum trabalho.

```
Usage:
  cosca mcp [command] [flags]

Commands:
  list                    List configured MCP servers
  add <name> <command> [args...]   Add an MCP server (stdio transport)
  remove <name>           Remove an MCP server

Example:
  COSCA_ENABLE_MCP=1 cosca mcp list
  COSCA_ENABLE_MCP=1 cosca mcp add my-server npx @modelcontextprotocol/server-filesystem .
  COSCA_ENABLE_MCP=1 cosca mcp remove my-server
```

### `cosca pipeline`

Manage and run orchestration pipelines.

```
Usage:
  cosca pipeline [command] [flags]

Commands:
  list          List available pipelines
  run <name>    Execute a pipeline

Example:
  cosca pipeline list
  cosca pipeline run code-review
  cosca pipeline run code-review --prompt "Review the auth module"
```

### `cosca metrics`

Show orchestration engine metrics.

```
Usage:
  cosca metrics [flags]

Description:
  Displays aggregated statistics:
  - Request counts and success rates
  - LLM call statistics (calls, tokens, errors, fallbacks)
  - Router decision breakdown
  - Memory-Augmented Generation (MAG) statistics
  - Per-stage timing breakdown

Example:
  cosca metrics
  cosca metrics --json
```

### `cosca serve`

Start the Cosca REST API server.

```
Usage:
  cosca serve [flags]

Flags:
  --host string          Host address to listen on (default: "0.0.0.0")
  --port int             Port for the REST API server (default: 14120)
  --metrics-port int     Port for the Prometheus metrics server (default: 14121)
  --cors-origins string  Comma-separated CORS origins (default: "*")
  --data-dir string      Data directory for engines (default: .cosca)

Description:
  Initializes all engines (Knowledge, Memory, Runtime, Agents, Skills,
  Providers, Workflows) and starts an HTTP REST API server with 36 endpoints.
  A separate metrics server exports Prometheus-compatible metrics on /metrics.
  Supports graceful shutdown on SIGINT/SIGTERM (30s timeout).

Endpoints Include:
  - Health: /health, /ready
  - Knowledge: /v1/knowledge/search, /v1/knowledge/index, /v1/knowledge/stats, /v1/knowledge/sync
  - Memory: /v1/memory/store, /v1/memory/search, /v1/memory/get, /v1/memory/delete, /v1/memory/promote, /v1/memory/stats
  - Runtime: /v1/status, /v1/health
  - Agents: /v1/agents, /v1/agents/search, /v1/agents/{name}
  - Skills: /v1/skills, /v1/skills/search, /v1/skills/{name}
  - Providers: /v1/providers, /v1/providers/{name}, /v1/providers/{name}/test, /v1/providers/active
  - Workflows: /v1/workflows, /v1/workflows/search, /v1/workflows/{name}, /v1/workflows/{name}/run
  - Auth: /v1/auth/login, /v1/auth/refresh, /v1/auth/me
  - Users: /v1/users, /v1/users/{id}, /v1/users/{id}/role

Example:
  cosca serve
  cosca serve --port 14120 --cors-origins "http://localhost:3000"
  cosca serve --host 0.0.0.0 --port 14120 --metrics-port 14121
```

---

### `cosca index`

Index files into the Cosca project (standalone, legacy command).

```
Usage:
  cosca index [flags]

Description:
  Quick file indexing command. For advanced indexing options,
  use `cosca knowledge index` instead.

Example:
  cosca index
  cosca index --verbose
```

### `cosca graph`

Access the knowledge graph (standalone, legacy command).

```
Usage:
  cosca graph [flags]

Description:
  Quick graph access. For advanced graph operations,
  use `cosca knowledge graph` instead.

Example:
  cosca graph
  cosca graph --json
```

### `cosca security scan`

Scan project dependencies for known vulnerabilities using the OSV.dev database
(via Google's osv-scanner).

```
Usage:
  cosca security scan [dir]

Flags:
  --severity string   Minimum severity to report (low, medium, high, critical)
  --recursive         Recursively scan subdirectories (default true)
  --exit-zero         Always exit with status 0 (CI friendly)
  --json              Machine-readable JSON output (global flag)

Exit codes:
  0  scan completed, no critical/high vulnerabilities (or --exit-zero)
  1  critical or high vulnerabilities found
  2  scan error (network failure, invalid arguments, ...)
```

Description:
  Scans the given directory (default: current dir) for dependency files
  (go.mod, go.sum, package-lock.json, pnpm-lock.yaml, requirements.txt, etc.)
  and reports known vulnerabilities from the OSV.dev database.

  For each vulnerable package it reports the advisory identifiers (CVE /
  GHSA / OSV), the severity (CRITICAL/HIGH/MEDIUM/LOW/UNKNOWN), a short
  summary and the OSV advisory URL. A summary of vulnerabilities by severity
  is printed at the end.

  Advisory URLs follow the form https://osv.dev/vulnerability/<id>.

Examples:
  cosca security scan               # Scan the current directory
  cosca security scan ./services    # Scan a specific directory
  cosca security scan --json        # Machine-readable JSON output
  cosca security scan --severity high   # Only HIGH/CRITICAL advisories
  cosca security scan --exit-zero   # Always exit 0 (CI pipelines)
```

> **Related**: [Dependency Security Scan](../../security/dependency-scanning.md) | [CLI Overview](overview.md) | [CLI Examples](examples.md)
