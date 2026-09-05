# Example: Project Initialization

> **Status**: active | **Owner**: CLI Chief | **Last Updated**: 2026-07-23

This example walks through initializing Cosca in a new project, explaining what happens at each step, how to verify the installation, and how to customize the configuration.

---

## Step-by-Step: `cosca install`

### Step 1: Create a New Project

```bash
mkdir my-awesome-project
cd my-awesome-project
git init
echo "# My Awesome Project" > README.md
```

### Step 2: Run Installation

```bash
cosca install
```

**Expected output:**

```
╭─────────────────────────────────────────────────────╮
│            Cosca Enterprise Installation           │
├─────────────────────────────────────────────────────┤
│ Project:      /home/user/my-awesome-project          │
│ Detected:     opencode v1.5.0                        │
│ Language:     Go 1.22                                 │
│ Framework:    none                                    │
├─────────────────────────────────────────────────────┤
│ ✅ Editor integration configured                     │
│ ✅ Plugin directory created                           │
│ ✅ Runtime initialized                                │
│ ✅ Cache directory created                            │
│ ✅ File index created                                 │
│ ✅ SQLite database created                            │
│ ✅ Knowledge graph initialized                        │
│ ✅ Embeddings generated                               │
│ ✅ Health check passed                                │
├─────────────────────────────────────────────────────┤
│ System: healthy                                      │
│ Database: .cosca/knowledge.db (1.2 MB)                 │
│ Indexed: 12 files                                    │
│ Plugins: 0 installed                                 │
│ Editor: opencode (configured)                        │
├─────────────────────────────────────────────────────┤
│ Next steps:                                          │
│ • cosca knowledge search "query"  — Search knowledge   │
│ • cosca plugin list               — List plugins       │
│ • cosca status                    — Check system health │
│ • cosca doctor                    — Full diagnostics   │
╰─────────────────────────────────────────────────────╯
```

### Step 3: Verify Installation

```bash
# Check system health
cosca status
```

**Expected output:**

```
System Status: healthy
  Uptime: 15s
  Version: 0.1.0-dev
  Components:
    knowledge: healthy
    discovery: healthy
    memory: healthy
    cache: healthy
    plugins: healthy
    editors: healthy
  Database: .cosca/knowledge.db (1.2 MB)
  Indexed: 12 files
```

```bash
# Run diagnostics
cosca doctor
```

**Expected output:**

```
Cosca Diagnostics
===================
✅ Go version: go1.22.5
✅ SQLite version: 3.45.1
✅ Config file: .cosca/config.yaml (valid)
✅ Database: .cosca/knowledge.db (consistent)
✅ Runtime state: running
✅ Editor: opencode (configured)
✅ Permissions: all checks passed
✅ Disk space: 45 GB available
```

### Step 4: Test Knowledge Search

```bash
# Index the project (if you have files)
cosca knowledge index .

# Search for content
cosca knowledge search "project"
```

**Expected output:**

```
Found 1 result (0.002s)

 1. README.md — My Awesome Project
    ───────────────────────────────────────────────
    # My Awesome Project
    Score: 0.92 | Type: document
```

---

## What Happens During Installation

`cosca install` performs the following operations in order:

```
1. Detect Project Environment
   ├── Language detection (Go, TypeScript, Python, etc.)
   ├── Framework detection (React, Django, Spring, etc.)
   └── Editor detection (OpenCode, VS Code, etc.)

2. Create Directory Structure
   ├── .cosca/                    ← Root Cosca directory
   │   ├── config.yaml          ← Cosca configuration
   │   ├── knowledge.db         ← SQLite database
   │   ├── plugins/             ← Plugin installations
   │   ├── cache/               ← Cache directory
   │   └── logs/                ← Runtime logs

3. Initialize Runtime
   ├── Create event bus
   ├── Initialize state machine
   ├── Register subsystems
   └── Start health check loop

4. Build Knowledge Base
   ├── Scan project files
   ├── Parse documents (Markdown, code, config)
   ├── Create FTS5 full-text index
   ├── Generate vector embeddings
   ├── Build knowledge graph
   └── Cache results

5. Configure Editor Integration
   ├── Detect active editor
   ├── Create/update configuration files
   ├── Register MCP server (if supported)
   └── Validate configuration

6. Final Validation
   ├── Health check all components
   ├── Verify database integrity
   └── Print installation summary
```

### Files Created by Installation

```
my-awesome-project/
├── .cosca/
│   ├── config.yaml              ← Main configuration file
│   ├── knowledge.db             ← SQLite database (FTS5 + vectors + graph)
│   ├── plugins/
│   │   └── .gitkeep             ← Plugin directory placeholder
│   ├── cache/
│   │   └── .gitkeep             ← Cache directory placeholder
│   └── logs/
│       └── cosca.log              ← Runtime log file
├── .opencode/                    (if OpenCode detected)
│   └── AGENTS.md                ← Custom instructions with Cosca tools
├── CLAUDE.md                     (if Claude Code detected)
└── .coscaignore                   ← File exclusion patterns
```

---

## Custom Configuration

### Installation with Options

```bash
# Install with custom data directory
cosca install --data-dir /path/to/data

# Install without editor integration
cosca install --no-editor

# Install with verbose output
cosca --verbose install

# Install in quiet mode
cosca --quiet install

# Install with JSON output (for scripting)
cosca --json install
```

### Configuration File

The generated `.cosca/config.yaml` can be customized:

```yaml
# .cosca/config.yaml

# Project configuration
project:
  name: "my-awesome-project"
  version: "0.1.0"

# Runtime settings
runtime:
  mode: "development"             # development | staging | production
  log_level: "info"               # trace | debug | info | warn | error
  health_check_interval: "30s"
  shutdown_timeout: "60s"

# Knowledge Engine settings
knowledge:
  auto_index: true                 # Auto-index on file changes
  exclude_patterns:
    - "node_modules/**"
    - ".git/**"
    - "vendor/**"
    - "dist/**"
    - "build/**"
  embedding_provider: "local"      # local | openai | azure
  chunk_size: 1000                 # Max characters per chunk
  chunk_overlap: 100               # Overlap between chunks

# Cache settings
cache:
  memory_size: 256                 # MB
  ttl: "24h"

# Editor settings
editor:
  auto_detect: true
  mcp_port: 0                      # 0 = auto-assign

# Plugin settings
plugins:
  allowed_runtimes:
    - go
    - wasm
    - external
  max_plugins: 50
  timeout: "30s"
```

### Manual Configuration After Install

```bash
# Edit the configuration file
nano .cosca/config.yaml

# Validate the configuration
cosca config validate

# Apply configuration changes
cosca sync
```

---

## Installing with Different Project Types

### TypeScript/Node.js Project

```bash
# Create project
mkdir my-node-app && cd my-node-app
npm init -y

# Install Cosca
cosca install

# Cosca automatically detects TypeScript and npm
```

### Python Project

```bash
# Create project
mkdir my-python-app && cd my-python-app
python -m venv venv

# Install Cosca
cosca install

# Cosca automatically detects Python and venv
```

### Existing Project

```bash
# Already have a project with existing files?
cd /path/to/existing/project

# Cosca will index all existing files
cosca install
```

---

## Verification Checklist

After installation, verify:

- [ ] `cosca status` reports **healthy**
- [ ] `cosca doctor` reports **all checks passed**
- [ ] `cosca knowledge stats` shows indexed files
- [ ] Editor integration is working (tools accessible in editor)
- [ ] `cosca knowledge search "your query"` returns results
- [ ] `.cosca/` directory structure is created
- [ ] Configuration file is valid YAML

---

## Troubleshooting Installation

| Issue | Solution |
|-------|----------|
| `Permission denied` when creating `.cosca/` | Ensure write permissions in the project directory |
| `Editor not detected` | Run `cosca editor setup <editor-name>` manually |
| `Knowledge index failed` | Check file permissions and disk space |
| `Database error` | Run `cosca knowledge rebuild` |
| `Slow installation` | First-time indexing of large projects takes longer |

---

**Related**: [Getting Started Guide](../../docs/developer-guide/getting-started.md) | [CLI Overview](../../docs/cli/overview.md) | [Configuration Reference](../../docs/runtime/configuration.md)
