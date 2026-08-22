# CLI Usage Examples

> **Status**: active | **Owner**: CLI Chief | **Last Updated**: 2026-07-23

This document provides practical examples of using Cosca in real-world scenarios.

---

## Installation

### Quick Install in Any Project

```bash
$ cd my-project
$ cosca install

Starting Cosca installation...

[████████████████████████████] 100% Installation Progress

✅ Installation complete!
  Project:  /home/user/my-project
  Editor:   opencode
  Duration: 3.2s
  Install ID: abc123-def456

Next Steps:
  • Run 'cosca status' to see system status
  • Run 'cosca doctor' for full diagnostics
  • Run 'cosca sync' to index your project files
  • Run 'cosca knowledge search <query>' to test the knowledge base
```

### Global Installation

```bash
$ cosca install --global

Starting global Cosca installation...
✅ Installed to /home/user/.cosca/
✅ Editor integration configured for opencode
✅ Runtime configuration created
✅ Global installation complete
```

### JSON Output for Scripting

```bash
$ cosca install --json
{
  "project_dir": "/home/user/my-project",
  "aos_dir": "/home/user/my-project/.cosca",
  "editor": "opencode",
  "install_id": "abc123-def456",
  "duration": "3.2s",
  "completed_steps": 14,
  "total_steps": 14
}
```

---

## Status and Diagnostics

### System Status

```bash
$ cosca status
System Status: healthy
  Uptime: 5m 32s
  Version: 1.4.0-dev (abc1234)
  Codename: Nova

  Components:
    knowledge:  healthy
    discovery:  healthy
    memory:     healthy
    cache:      healthy

  Knowledge Base:
    Documents:  1,247
    Chunks:     8,934
    Entities:   3,456
    Vectors:    8,934

  Memory:
    Records:    42
    Layers:     5
```

### Full Diagnostics

```bash
$ cosca doctor

🔍 Cosca System Diagnostics
══════════════════════════

✅ Configuration        Valid
✅ SQLite Database      Connected (3.2 MB)
✅ FTS5 Engine          Available
❌ Vector Engine        Not available (no embedding provider configured)
✅ Knowledge Graph      Loaded (1,247 nodes, 3,892 edges)
✅ Memory Engine        Running (5 layers)
✅ Plugin System        Active (2 plugins loaded)
✅ Editor Integration   Configured (opencode)
✅ File Watcher         Active
✅ Event Bus            Running

⚠️  Issues Found:
  1. Vector search unavailable: no embedding provider configured
     Suggestion: Set OPENAI_API_KEY or configure embedding provider

  2. Plugin 'legacy-plugin' is using deprecated API
     Suggestion: Run 'cosca plugin update legacy-plugin'
```

---

## Knowledge Search

### Basic Search

```bash
$ cosca knowledge search "authentication flow"

Results (15 found, 1.2ms):
  Rank  Score  Type      Title
     1  0.92   document  Authentication Architecture — docs/auth/overview.md
     2  0.85   chunk     JWT Token Flow — docs/auth/jwt.md
     3  0.78   chunk     OAuth2 Integration — docs/auth/oauth.md
     4  0.72   code      auth_middleware.go — internal/api/middleware/auth.go
     5  0.65   entity    AuthService — internal/api/service/auth.go
     6  0.60   code      auth_test.go — internal/api/service/auth_test.go
     7  0.55   document  Security Best Practices — docs/security/README.md
```

### Type-Specific Search

```bash
$ cosca knowledge search "database" --type document --limit 5

Results (5 found, 0.8ms):
  Rank  Score  Title
     1  0.91   Database Schema — docs/database/schema.md
     2  0.85   Migration Guide — docs/database/migrations.md
     3  0.78   Connection Pooling — docs/database/pooling.md
     4  0.72   Backup and Recovery — docs/database/backup.md
     5  0.65   Query Optimization — docs/database/queries.md
```

### Code Search

```bash
$ cosca knowledge search "gRPC client" --type code --tag language=go

Results (4 found):
  Rank  Score  Language  File
     1  0.94   go        internal/api/grpc/client.go (lines 42-78)
     2  0.88   go        internal/api/grpc/connection.go (lines 15-45)
     3  0.76   go        internal/editors/claude/adapter.go (lines 120-135)
     4  0.70   go        internal/editors/opencode/adapter.go (lines 85-92)
```

### Search with Facets

```bash
$ cosca knowledge search "plugin" --facets

Results (23 found, 2.1ms):
  Rank  Score  Type      Title
     1  0.89   document  Plugin System — docs/plugins/overview.md
     ...

Facets:
  type:       document=5, chunk=12, entity=4, code=2
  source:     fts=18, vector=5
  entity_type: component=3, interface=1
```

### Graph-Enabled Search

```bash
$ cosca knowledge search "Runtime" --graph --type entity

Results (5 found):
  Rank  Score  Entity          Type          Connections
     1  0.95   Runtime         Component     8 connections
     2  0.80   RuntimeState    StateMachine  3 connections
     3  0.75   RuntimeConfig    Config        5 connections
     4  0.70   EventBus        EventSystem   4 connections
     5  0.65   Lifecycle       Component     3 connections
```

### Search with Explanation

```bash
$ cosca knowledge explain "result-abc123"

Explanation for result-abc123:
  Type: chunk
  Name: "JWT Token Flow"
  Factors:
    fts_match:          0.82
    vector_similarity:  0.71
    graph_presence:     0.00
  Connections:
    - connected to AuthService (component)
    - connected to JWT (concept)
```

---

## Knowledge Graph

### Graph Statistics

```bash
$ cosca knowledge graph stats

Knowledge Graph Statistics:
  Nodes:              1,247
  Edges:              3,892
  Entity Types:       10
  Relationship Types: 10
  Avg Connections/Node: 3.1
  Most Connected:     "Runtime" (24 connections)
```

### Entity Details

```bash
$ cosca knowledge graph node "node-runtime"

Entity: "Runtime"
  Type: Component
  Metadata:
    description: "Central orchestrator for Cosca platform"
    package: "internal/runtime"
    file: "internal/runtime/runtime.go"
  Connections (24):
    → "RuntimeState"    (contains)
    → "EventBus"         (contains)
    → "Lifecycle"        (contains)
    → "Metrics"          (contains)
    → "KnowledgeEngine"  (depends_on)
    → "DiscoveryEngine"  (depends_on)
    → "MemoryEngine"     (depends_on)
    ...

### Graph Visualization

```bash
$ cosca knowledge graph viz "Runtime" --depth 1

Runtime (Component)
├── contains → RuntimeState (StateMachine)
├── contains → EventBus (EventSystem)
├── contains → Metrics (MetricsCollector)
├── contains → Lifecycle (LifecycleManager)
├── configures → RuntimeConfig (Config)
└── depends_on → KnowledgeEngine (Component)
```

---

## Memory Operations

### Storing a Decision

```bash
$ cosca memory store \
    --type decision \
    --layer project \
    --scope backend \
    --priority 8 \
    --content "Use PostgreSQL with PgBouncer for connection pooling" \
    --metadata "author=developer,date=2026-07-23"

✅ Memory stored: abc123-def456
```

### Searching Memory

```bash
$ cosca memory search "database" --type decision

Results (3 found):
  Rank  Layer     Priority  Content
     1   project   8         Use PostgreSQL with PgBouncer for connection pooling
     2   project   5         Enable WAL mode for better write concurrency
     3   global    3         Always use parameterized queries
```

### Promoting Memory

```bash
$ cosca memory promote "abc123" session project
✅ Memory abc123 promoted from session → project
📸 Snapshot created: promote-abc123-session-project
```

---

## Plugin Management

### Listing Plugins

```bash
$ cosca plugin list --format table

PLUGIN          VERSION  STATE     HEALTH
hello-cosca       1.0.0    started   healthy
search-enhancer 2.1.0    started   healthy
legacy-plugin   0.5.0    stopped   unhealthy
```

### Installing a Plugin

```bash
$ cosca plugin install ./my-plugin/

📦 Installing plugin: my-plugin
  ✅ Manifest validated
  ✅ Dependencies satisfied (cosca-core >=1.0.0)
  ✅ Plugin initialized
  ✅ Plugin started
  ✅ my-plugin v1.0.0 installed and active
```

### Plugin Details

```bash
$ cosca plugin info hello-cosca

Plugin: hello-cosca
  Name:         Hello Cosca
  Version:      1.0.0
  Author:       Cosca Developer
  Runtime:      go
  State:        started (uptime: 2h 15m)
  Health:       healthy
  Permissions:  filesystem
  Hooks:        on_startup, on_shutdown
  Dependencies: none
```

---

## Editor Integration

### Detecting Editors

```bash
$ cosca editor detect
✅ Detected editor: opencode (v1.5.0)
  Setup required: no (already configured)

$ cosca editor list
  Editor      Detected  Configured
  opencode    yes       yes
  claude      no        no
  codex       no        no
  cursor      no        no
  vscode      yes       no
  neovim      no        no
```

### Setting Up an Editor

```bash
$ cosca editor setup vscode

Setting up Cosca integration for VS Code...
✅ VS Code detected at /usr/bin/code
✅ Workspace settings updated (.vscode/settings.json)
✅ MCP server registered
✅ Task definitions created (.vscode/tasks.json)

VS Code integration complete!
  • Use Ctrl+Shift+P → Tasks → Cosca: Search Knowledge
  • Use integrated terminal: cosca knowledge search "query"
```

---

## Runtime Management

### Starting the Daemon

```bash
$ cosca runtime start --daemon
✅ Runtime daemon started (PID: 12345)
  Health: healthy
  Components:
    knowledge: starting
    discovery: started
    memory: started
    cache: started
```

### Checking Runtime Status

```bash
$ cosca runtime status

Runtime Status:
  State:   running
  Health:  healthy
  Uptime:  2h 15m 32s
  Version: 1.4.0-dev

  Components:
    knowledge:  healthy (uptime: 2h 15m)
    discovery:  healthy (uptime: 2h 15m)
    memory:     healthy (uptime: 2h 15m)
    cache:      healthy (uptime: 2h 15m)
    plugins:    healthy (uptime: 2h 15m)
    editors:    healthy (uptime: 2h 15m)
    watcher:    healthy (uptime: 2h 15m)
```

### Stopping the Daemon

```bash
$ cosca runtime stop
✅ Runtime daemon stopped gracefully
```

---

## Configuration

### Viewing Configuration

```bash
$ cosca config show --format yaml

version: "1.0"
mode: development
paths:
  home: /home/user/.config/cosca
  project: /home/user/my-project
db:
  path: /home/user/.config/cosca/cosca.db
  wal_mode: true
```

### Initializing Configuration

```bash
$ cosca config init
✅ Default configuration created at /home/user/.config/cosca/config.yaml
```

---

## Shell Completion

### Installing Bash Completion

```bash
$ cosca completion bash | sudo tee /etc/bash_completion.d/cosca
$ source ~/.bashrc

# Now tab-complete works:
$ cosca [TAB][TAB]
install     init        status      doctor      version
knowledge   memory      runtime     plugin      editor
...
```

---

## Advanced Examples

### Piped JSON Output

```bash
$ cosca --json status | jq '.components'
{
  "knowledge": { "status": "healthy" },
  "discovery": { "status": "healthy" },
  "memory": { "status": "healthy" }
}
```

### Batch Indexing

```bash
$ find docs -name "*.md" | xargs cosca knowledge index
✅ Indexed: docs/architecture/overview.md
✅ Indexed: docs/architecture/layers.md
✅ Indexed: docs/runtime/overview.md
...
```

### Automated Diagnostics in CI

```bash
$ if ! cosca doctor --json | jq -e '.issues | length == 0' > /dev/null; then
    echo "Cosca issues found!"
    cosca doctor
    exit 1
  fi
```

---

> **Related**: [CLI Overview](overview.md) | [Command Reference](commands.md)
