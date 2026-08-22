# Example: Hello World Plugin

> **Status**: active | **Owner**: Plugin Chief | **Last Updated**: 2026-07-23

This example demonstrates creating a simple "Hello World" plugin for Cosca. The plugin registers a command, hooks into the startup event, and provides a custom `/hello` tool via the MCP server.

---

## Plugin Overview

We'll build a Go native plugin that:

1. Registers the `cosca hello` command
2. Logs a greeting on startup
3. Exposes the MCP tool `hello_world`
4. Responds to the `on_search` hook to add a custom result

---

## Prerequisites

- Cosca installed and initialized in a project
- Go 1.22 or later
- Basic familiarity with Go programming

---

## Step 1: Create Plugin Directory

```bash
mkdir -p hello-cosca-plugin
cd hello-cosca-plugin
go mod init hello-cosca-plugin
```

---

## Step 2: Create Plugin Manifest

Create `plugin.yaml` in the plugin root:

```yaml
# plugin.yaml
id: "hello-cosca"
name: "Hello Cosca"
version: "1.0.0"
description: "A simple hello world plugin for Cosca"
author: "Developer <dev@example.com>"
runtime: "go"

# Required permissions
permissions:
  - filesystem

# Hook points
hooks:
  - on_startup
  - on_search

# Configuration schema
config_schema:
  type: object
  properties:
    greeting:
      type: string
      default: "Hello from Cosca Plugin!"
    target:
      type: string
      default: "World"
```

---

## Step 3: Implement the Plugin

Create `main.go`:

```go
package main

import (
    "fmt"

    "github.com/CoscaAI/cosca/pkg/plugins"
)

// HelloPlugin implements the Plugin interface.
type HelloPlugin struct {
    plugins.BasePlugin
    ctx plugins.PluginContext
}

// Init is called when the plugin is loaded.
// Use this to initialize resources, parse config, and register hooks.
func (p *HelloPlugin) Init(ctx plugins.PluginContext) error {
    p.ctx = ctx

    // Read plugin configuration
    greeting, _ := ctx.Config["greeting"].(string)
    if greeting == "" {
        greeting = "Hello from Cosca Plugin!"
    }

    target, _ := ctx.Config["target"].(string)
    if target == "" {
        target = "World"
    }

    ctx.Logger.Info(fmt.Sprintf("Plugin initialized with greeting: %s, target: %s", greeting, target))

    // Register the plugin's custom command
    ctx.RuntimeAPI.RegisterCommand("hello", func(args []string) (string, error) {
        return fmt.Sprintf("%s, %s!", greeting, target), nil
    })

    // Register a hook handler for on_startup
    _, err := ctx.RuntimeAPI.RegisterHook("on_startup", func(data interface{}) error {
        ctx.Logger.Info(fmt.Sprintf("Cosca Runtime started! %s, %s!", greeting, target))
        return nil
    })
    if err != nil {
        return fmt.Errorf("failed to register startup hook: %w", err)
    }

    // Register a hook handler for on_search to inject a custom result
    _, err = ctx.RuntimeAPI.RegisterHook("on_search", func(data interface{}) error {
        searchData, ok := data.(map[string]interface{})
        if !ok {
            return nil
        }
        query, _ := searchData["query"].(string)
        ctx.Logger.Debug(fmt.Sprintf("Search query received: %s", query))
        return nil
    })
    if err != nil {
        return fmt.Errorf("failed to register search hook: %w", err)
    }

    return nil
}

// Start is called when the runtime transitions to RUNNING state.
func (p *HelloPlugin) Start() error {
    p.ctx.Logger.Info("HelloPlugin started — ready to greet!")
    return nil
}

// Stop is called when the runtime is shutting down.
func (p *HelloPlugin) Stop() error {
    p.ctx.Logger.Info("HelloPlugin stopped — goodbye!")
    return nil
}

// Export the plugin (required for Go native plugins)
var Plugin HelloPlugin
```

---

## Step 4: Build the Plugin

Compile the plugin as a shared library:

```go
//go:build plugin
// +build plugin

package main

// Build with: go build -buildmode=plugin -o hello-cosca.so .
```

```bash
# Build the plugin (must be built with the same Go version as Cosca)
go build -buildmode=plugin -o hello-cosca.so .

# Verify the plugin was built
ls -la hello-cosca.so
# -rwxr-xr-x 1 user user 4.2M Jul 23 10:00 hello-cosca.so
```

> **Important:** Go native plugins must be compiled with the same Go version as the Cosca binary. Use `go version` to check your Cosca build version.

---

## Step 5: Install the Plugin

```bash
# Install the plugin from the build directory
cosca plugin install ./hello-cosca.so

# Or install from a directory containing plugin.yaml
cosca plugin install ./hello-cosca-plugin

# Verify the plugin is installed
cosca plugin list
```

**Expected output:**

```
PLUGIN         VERSION  STATUS   RUNTIME  PERMISSIONS
hello-cosca      1.0.0    started  go       filesystem
```

### Plugin Details

```bash
cosca plugin info hello-cosca
```

**Expected output:**

```
Plugin: hello-cosca
  Name: Hello Cosca
  Version: 1.0.0
  Author: Developer <dev@example.com>
  Runtime: go
  Status: started
  Permissions: filesystem
  Hooks: on_startup, on_search
  Commands: hello
```

---

## Step 6: Use the Plugin

### Run the Custom Command

```bash
cosca hello
```

**Expected output:**

```
Hello from Cosca Plugin!, World!
```

### With Custom Configuration

Create a plugin configuration file at `.cosca/plugins/hello-cosca.yaml`:

```yaml
# .cosca/plugins/hello-cosca.yaml
greeting: "Hi there"
target: "Cosca Community"
```

```bash
# Restart runtime to pick up config changes
cosca runtime restart

# Run the command again
cosca hello
```

**Expected output:**

```
Hi there, Cosca Community!
```

### Check Plugin Logs

```bash
cosca doctor --plugin hello-cosca
```

The plugin logs messages during init, start, and stop:

```
[INFO]  hello-cosca: Plugin initialized with greeting: Hi there, target: Cosca Community
[INFO]  hello-cosca: HelloPlugin started — ready to greet!
[INFO]  hello-cosca: Cosca Runtime started! Hi there, Cosca Community!
```

---

## Step 7: MCP Tool Registration

When the plugin registers a command via `RegisterCommand`, Cosca automatically exposes it as an MCP tool if the editor integration supports MCP.

The `hello_cosca` MCP tool becomes available in your editor:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "hello_cosca",
    "arguments": {}
  }
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Hi there, Cosca Community!"
      }
    ]
  }
}
```

---

## Step 8: Remove the Plugin (Cleanup)

```bash
# Disable the plugin (keeps it installed but inactive)
cosca plugin disable hello-cosca

# Re-enable the plugin
cosca plugin enable hello-cosca

# Remove the plugin completely
cosca plugin remove hello-cosca

# Verify removal
cosca plugin list
# (hello-cosca should no longer appear)
```

---

## Complete File Listing

```
hello-cosca-plugin/
├── plugin.yaml            ← Plugin manifest
├── main.go                ← Plugin implementation
├── go.mod                 ← Go module file
├── go.sum                 ← Go module checksums
└── hello-cosca.so           ← Built plugin binary (output)
```

---

## Extending the Plugin

### Add a Custom Hook

```go
// Trigger a custom event from your plugin
err := p.ctx.RuntimeAPI.EmitEvent("hello:greeted", map[string]interface{}{
    "greeting": greeting,
    "target":   target,
    "timestamp": time.Now().Unix(),
})
```

### Add Memory Features

```go
// Store a memory from the plugin
err := p.ctx.RuntimeAPI.SetConfig("hello-cosca:last-greeting", "Hello at "+time.Now().String())
```

### Add Search Results

```go
// Inject results into search queries (via on_search hook)
func injectResult(searchData interface{}) error {
    results, ok := searchData.(map[string]interface{})["results"].(*[]plugins.SearchResult)
    if !ok {
        return nil
    }
    *results = append(*results, plugins.SearchResult{
        Title:   "Hello Cosca Plugin",
        Snippet: "This is a custom result from the Hello Cosca plugin!",
        Score:   0.5,
        Source:  "hello-cosca",
    })
    return nil
}
```

---

**Related**: [Plugin System Overview](../../docs/plugins/overview.md) | [Plugin Development Guide](../../docs/plugins/development.md) | [ADR-003](../../docs/adr/ADR-003-plugin-system.md)
