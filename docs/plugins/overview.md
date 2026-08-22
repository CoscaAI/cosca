# Plugin System

> **Status**: active | **Owner**: Plugin Chief | **Last Updated**: 2026-07-23

## Architecture

The Plugin System provides a powerful extension mechanism for Cosca. Plugins can be written in Go (native), WebAssembly (WASM), as external processes, or as shared libraries. The system manages the full plugin lifecycle, handles dependencies, and provides communication via hooks and events.

```
┌──────────────────────────────────────────────────────────────────┐
│                       PLUGIN SYSTEM                               │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                    PLUGIN MANAGER                          │    │
│  │                                                           │    │
│  │  • Register plugins  • Resolve dependencies              │    │
│  │  • Lifecycle control  • List/query plugins               │    │
│  │  • Security enforcement                                  │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                  LIFECYCLE MANAGER                         │    │
│  │                                                           │    │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │    │
│  │  │ INSTALL  │──▶│  INIT   │──▶│  START   │──▶│  STOP   │ │    │
│  │  │          │  │          │  │          │  │          │ │    │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │    │
│  │       │              │             │             │        │    │
│  │       ▼              ▼             ▼             ▼        │    │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │    │
│  │  │  ERROR   │  │  ERROR   │  │  ERROR   │  │  ERROR   │ │    │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                 RUNTIME ADAPTERS                           │    │
│  │                                                           │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌──────────────┐ │    │
│  │  │   Go    │  │  WASM   │  │External │  │  Shared Lib   │ │    │
│  │  │ Native  │  │ Sandbox │  │ Process │  │  .so/.dll     │ │    │
│  │  └─────────┘  └─────────┘  └─────────┘  └──────────────┘ │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │              COMMUNICATION LAYER                           │    │
│  │                                                           │    │
│  │  ┌──────────────────┐  ┌──────────────────────────────┐  │    │
│  │  │    HOOK SYSTEM   │  │        EVENT BUS             │  │    │
│  │  │  (typed hooks)   │  │  (publish/subscribe)         │  │    │
│  │  └──────────────────┘  └──────────────────────────────┘  │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

---

## Plugin Lifecycle

```
                    ┌────────────┐
                    │  INSTALLED │  ← Plugin is installed on filesystem
                    └──────┬─────┘
                           │ Init() called
                           ▼
                    ┌────────────┐
              ┌────▶│INITIALIZED │  ← Plugin context created
              │     └──────┬─────┘
              │            │ Start() called
              │            ▼
              │     ┌────────────┐
              │     │  STARTED   │  ← Plugin actively running
              │     └──────┬─────┘
              │            │ Stop() called
              │            ▼
              │     ┌────────────┐
              │     │  STOPPED   │  ← Gracefully stopped
              │     └────────────┘
              │
              │  Error states:
              └──── ERROR ← Any phase can transition here
```

| State | Description |
|-------|-------------|
| `Installed` | Plugin is on filesystem but not loaded |
| `Initialized` | Plugin loaded, context created, `Init()` succeeded |
| `Started` | Plugin actively running, `Start()` succeeded |
| `Stopped` | Plugin gracefully stopped |
| `Error` | Plugin encountered an unrecoverable error |

### Lifecycle Ordering

Plugins are initialized and started in **dependency order** (dependencies first) and stopped in **reverse dependency order**.

---

## Plugin Runtimes

| Runtime | Technology | Isolation | Performance | Use Case |
|---------|-----------|-----------|-------------|----------|
| **Go** | `plugin.Open` | Low (same process) | Best | First-party, trusted plugins |
| **WASM** | WebAssembly | High (sandboxed) | Good | Third-party plugins |
| **External** | Child process/gRPC | Highest (separate process) | Moderate | Untrusted or language-agnostic |
| **SharedLib** | `.so`/`.dll` | Medium | Good | Performance-critical, trusted |

---

## Plugin Manifest Format

Each plugin requires a `plugin.yaml` or `plugin.json` manifest file:

```yaml
# plugin.yaml

# Unique plugin identifier
id: "my-plugin"

# Human-readable name
name: "My Awesome Plugin"

# Semantic version
version: "1.0.0"

# Short description
description: "Extends Cosca with awesome capabilities"

# Plugin author
author: "My Company <dev@company.com>"

# Runtime type: go, wasm, external, sharedlib
runtime: "go"

# Required permissions
permissions:
  - filesystem     # Read/write files
  - network        # Make network requests
  - exec           # Execute commands
  - environment     # Read environment variables

# Plugin dependencies
dependencies:
  - plugin_id: "cosca-core"
    version: ">=1.0.0"
    optional: false
  - plugin_id: "helper-lib"
    version: ">=0.5.0"
    optional: true

# Hook points this plugin registers for
hooks:
  - on_search
  - on_index
  - on_startup

# JSON Schema for plugin configuration
config_schema:
  type: object
  properties:
    api_key:
      type: string
    max_results:
      type: integer
      default: 10

# Entry point (for external/sharedlib runtimes)
entry_point: "./bin/my-plugin"

# SHA-256 checksum (for integrity verification)
checksum: "sha256-abc123..."
```

### Permissions

| Permission | Description |
|------------|-------------|
| `filesystem` | Read and write files |
| `network` | Make network/HTTP requests |
| `exec` | Execute system commands |
| `environment` | Read environment variables |
| `all` | All permissions (not recommended) |

---

## Hooks and Events

### Hook System

Hooks allow plugins to intercept and extend Cosca operations:

| Hook Point | Trigger | Payload |
|------------|---------|---------|
| `on_startup` | Runtime startup completes | Runtime info |
| `on_shutdown` | Runtime begins shutdown | — |
| `on_search` | Search query executed | SearchParams, results |
| `on_index` | Document indexed | Document metadata |
| `on_sync` | Filesystem sync completes | SyncResult |
| `on_install` | Plugin installed | Plugin manifest |
| `on_config_change` | Configuration changed | Config diff |
| `on_error` | Error occurred | Error details |

### Event Bus

Plugins can publish and subscribe to events:

```go
// Publish an event
ctx.RuntimeAPI.EmitEvent("my_custom_event", map[string]interface{}{
    "key": "value",
})

// Register a hook handler
handlerID, err := ctx.RuntimeAPI.RegisterHook("on_search", func(args interface{}) error {
    // Handle search event
    return nil
})
```

---

## Creating Plugins

### Minimal Go Plugin

```go
package main

import "github.com/CoscaAI/cosca/pkg/plugins"

// MyPlugin implements the Plugin interface
type MyPlugin struct {
    plugins.BasePlugin
    ctx plugins.PluginContext
}

func (p *MyPlugin) Init(ctx plugins.PluginContext) error {
    p.ctx = ctx
    ctx.Logger.Info("initialized my plugin")
    return nil
}

func (p *MyPlugin) Start() error {
    p.ctx.Logger.Info("started my plugin")
    return nil
}

func (p *MyPlugin) Stop() error {
    p.ctx.Logger.Info("stopped my plugin")
    return nil
}

// Export as plugin (required for Go native plugins)
var Plugin MyPlugin
```

### WASM Plugin

```go
//go:build wasm
package main

import "github.com/CoscaAI/cosca/pkg/plugins"

// Export WASM functions
//export init
func init() { /* ... */ }

//export start
func start() int { return 0 }

//export stop
func stop() int { return 0 }
```

---

## CLI Commands

```bash
# Install a plugin
cosca plugin install <path-or-url>

# List installed plugins
cosca plugin list

# Show plugin details
cosca plugin info <id>

# Remove a plugin
cosca plugin remove <id>

# Enable/disable a plugin
cosca plugin enable <id>
cosca plugin disable <id>

# Update a plugin
cosca plugin update <id>

# Validate plugin manifest
cosca plugin validate <path>
```

---

## Security

The plugin system enforces security through:

1. **Permission model** — Plugins declare required permissions at install time
2. **Runtime isolation** — WASM and external runtimes provide sandboxing
3. **Dependency validation** — All plugin dependencies must be satisfied
4. **Checksum verification** — Plugin integrity verified via SHA-256
5. **Timeout enforcement** — Plugins cannot exceed configured timeouts
6. **Memory limits** — Plugins constrained to configured memory budgets

---

## Plugin Context API

Plugins receive a `PluginContext` during initialization:

```go
type PluginContext struct {
    Config     map[string]interface{}   // Plugin-specific configuration
    Logger     PluginLogger             // Structured logger
    DataDir    string                   // Persistent data directory
    TempDir    string                   // Temporary files directory
    RuntimeAPI RuntimeAPI              // Limited runtime access
}

type RuntimeAPI interface {
    GetConfig(key string) (interface{}, error)
    SetConfig(key string, value interface{}) error
    EmitEvent(eventType string, data interface{}) error
    RegisterHook(hookPoint string, handler func(args interface{}) error) (string, error)
    UnregisterHook(hookID string) error
}
```

---

> **Related**: [Plugin Development Guide](development.md) | [ADR-003](../adr/ADR-003-plugin-system.md)
