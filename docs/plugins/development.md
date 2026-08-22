# Plugin Development Guide

> **Status**: active | **Owner**: Plugin Chief | **Last Updated**: 2026-07-23

## Getting Started

This guide walks you through creating a complete Cosca plugin.

### Prerequisites

- Go 1.22+ installed
- Cosca installed
- Understanding of the Go plugin system

### Project Structure

```
my-plugin/
├── plugin.yaml          # Plugin manifest
├── main.go              # Plugin implementation
├── go.mod               # Go module
└── README.md            # Plugin documentation
```

---

## Plugin Interface

Every plugin must implement the `Plugin` interface:

```go
// Plugin defines the interface that all plugins must implement.
type Plugin interface {
    ID() string
    Name() string
    Version() string
    Description() string
    Author() string
    Init(ctx PluginContext) error
    Start() error
    Stop() error
    Health() (PluginHealth, error)
}
```

### Using BasePlugin

The `BasePlugin` provides default implementations for all interface methods:

```go
type MyPlugin struct {
    plugins.BasePlugin
}

func NewMyPlugin() *MyPlugin {
    return &MyPlugin{
        BasePlugin: plugins.BasePlugin{
            IDValue:      "my-plugin",
            NameValue:    "My Plugin",
            VersionValue: "1.0.0",
            DescValue:    "Does awesome things",
            AuthorValue:  "My Company",
        },
    }
}
```

You only need to override the methods you need.

---

## Step-by-Step: Hello World Plugin

### 1. Create the project

```bash
mkdir hello-cosca
cd hello-cosca
go mod init hello-cosca
```

### 2. Create the manifest

**plugin.yaml:**
```yaml
id: hello-cosca
name: Hello Cosca
version: 1.0.0
description: A simple hello world plugin for Cosca
author: Cosca Developer
runtime: go
permissions:
  - filesystem
hooks:
  - on_startup
  - on_shutdown
```

### 3. Implement the plugin

**main.go:**
```go
package main

import (
    "fmt"
    "time"

    "github.com/CoscaAI/cosca/pkg/plugins"
)

type HelloPlugin struct {
    plugins.BasePlugin
    ctx       plugins.PluginContext
    startTime time.Time
}

func NewHelloPlugin() *HelloPlugin {
    return &HelloPlugin{
        BasePlugin: plugins.BasePlugin{
            IDValue:      "hello-cosca",
            NameValue:    "Hello Cosca",
            VersionValue: "1.0.0",
            DescValue:    "A simple hello world plugin for Cosca",
            AuthorValue:  "Cosca Developer",
        },
    }
}

func (p *HelloPlugin) Init(ctx plugins.PluginContext) error {
    p.ctx = ctx
    ctx.Logger.Info("Hello Cosca plugin initialized!")
    
    // Read configuration
    if greeting, err := ctx.GetConfig("greeting"); err == nil {
        ctx.Logger.Info(fmt.Sprintf("Custom greeting: %v", greeting))
    }
    
    return nil
}

func (p *HelloPlugin) Start() error {
    p.startTime = time.Now()
    p.ctx.Logger.Info("Hello Cosca plugin started!")
    
    // Register a hook
    _, err := p.ctx.RuntimeAPI.RegisterHook("on_startup", func(args interface{}) error {
        p.ctx.Logger.Info("Startup hook triggered!")
        return nil
    })
    if err != nil {
        return fmt.Errorf("register hook: %w", err)
    }
    
    return nil
}

func (p *HelloPlugin) Stop() error {
    uptime := time.Since(p.startTime)
    p.ctx.Logger.Info(fmt.Sprintf("Hello Cosca plugin stopped (uptime: %s)", uptime))
    
    // Emit shutdown event
    p.ctx.RuntimeAPI.EmitEvent("plugin_shutdown", map[string]interface{}{
        "plugin": p.ID(),
        "uptime": uptime.String(),
    })
    
    return nil
}

func (p *HelloPlugin) Health() (plugins.PluginHealth, error) {
    return plugins.PluginHealth{
        PluginID:  p.IDValue,
        Status:    "healthy",
        LastCheck: time.Now(),
        Uptime:    time.Since(p.startTime),
        State:     plugins.PluginStateStarted,
    }, nil
}

// Export the plugin (required for Go native plugins)
var Plugin HelloPlugin
```

### 4. Build the plugin

```bash
go build -buildmode=plugin -o hello-cosca.so .
```

### 5. Install the plugin

```bash
# Copy to plugins directory
cp hello-cosca.so ~/.config/cosca/plugins/hello-cosca/
cp plugin.yaml ~/.config/cosca/plugins/hello-cosca/

# Install via CLI
cosca plugin install ./hello-cosca
```

### 6. Verify installation

```bash
cosca plugin list
# Should show:
#   hello-cosca   v1.0.0   Hello Cosca   [started]

cosca plugin info hello-cosca
# Shows detailed plugin information
```

---

## Hook Integration

### Available Hook Points

```go
const (
    // Lifecycle hooks
    HookOnStartup  HookPoint = "on_startup"   // Runtime started
    HookOnShutdown HookPoint = "on_shutdown"  // Runtime shutting down
    
    // Knowledge hooks
    HookOnSearch   HookPoint = "on_search"    // Search executed
    HookOnIndex    HookPoint = "on_index"     // Document indexed
    HookOnSync     HookPoint = "on_sync"      // Filesystem sync
    
    // Plugin hooks
    HookOnInstall  HookPoint = "on_install"   // Plugin installed
    HookOnRemove   HookPoint = "on_remove"    // Plugin removed
    
    // Config hooks
    HookOnConfigChange HookPoint = "on_config_change" // Config changed
    
    // Error hooks
    HookOnError    HookPoint = "on_error"     // Error occurred
)
```

### Registering Hooks

```go
func (p *MyPlugin) Start() error {
    // Register with default priority (100)
    id, err := p.ctx.RuntimeAPI.RegisterHook("on_search", func(args interface{}) error {
        params := args.(map[string]interface{})
        p.ctx.Logger.Info(fmt.Sprintf("Search query: %v", params["query"]))
        return nil
    })
    
    // Store hook ID for later unregistration
    p.hookID = id
    return nil
}

func (p *MyPlugin) Stop() error {
    // Unregister hook on shutdown
    p.ctx.RuntimeAPI.UnregisterHook(p.hookID)
    return nil
}
```

---

## Event Subscription

Plugins can emit and receive events through the event bus:

```go
// Emit custom events
func (p *MyPlugin) DoSomething() {
    p.ctx.RuntimeAPI.EmitEvent("my_plugin.action", map[string]interface{}{
        "action": "something_happened",
        "timestamp": time.Now(),
    })
}
```

---

## Configuration

### Plugin-Specific Configuration

Plugins receive configuration from the `PluginContext.Config` map. Configuration can be provided in the user's `config.yaml`:

```yaml
# In ~/.config/cosca/config.yaml
plugins:
  config:
    hello-cosca:
      greeting: "Hello from Cosca!"
      max_items: 50
```

### Reading Configuration

```go
func (p *MyPlugin) Init(ctx plugins.PluginContext) error {
    // Read config values
    greeting, _ := ctx.GetConfig("greeting")
    maxItems, _ := ctx.GetConfig("max_items")
    
    ctx.Logger.Info(fmt.Sprintf("Greeting: %v, Max items: %v", greeting, maxItems))
    return nil
}
```

---

## Best Practices

### 1. Handle Errors Gracefully
```go
func (p *MyPlugin) Init(ctx plugins.PluginContext) error {
    // If optional dependency is missing, log and continue
    if err := p.setupOptionalFeature(); err != nil {
        ctx.Logger.Warn("optional feature unavailable", "error", err)
    }
    return nil
}
```

### 2. Clean Up Resources
```go
func (p *MyPlugin) Stop() error {
    // Close connections, release resources
    if p.db != nil {
        p.db.Close()
    }
    if p.fileWatcher != nil {
        p.fileWatcher.Close()
    }
    return nil
}
```

### 3. Use Structured Logging
```go
ctx.Logger.Debug("processing item", "item_id", id, "count", count)
ctx.Logger.Info("operation completed", "duration", elapsed)
ctx.Logger.Warn("resource low", "memory", usage, "limit", limit)
ctx.Logger.Error("operation failed", "error", err, "retry", attempt)
```

### 4. Respect Timeouts
```go
func (p *MyPlugin) Start() error {
    // Use context for long operations
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    return p.initializeWithContext(ctx)
}
```

### 5. Validate on Install
```go
func (p *MyPlugin) Validate() error {
    if p.ctx.Config["api_key"] == "" {
        return fmt.Errorf("api_key is required")
    }
    return nil
}
```

### 6. Version Your Plugin
Follow semantic versioning for your plugin:
- `1.0.0` — First stable release
- `1.1.0` — New feature (backward compatible)
- `2.0.0` — Breaking changes

---

## Testing Plugins

```go
package main

import (
    "testing"
    "github.com/CoscaAI/cosca/pkg/plugins"
)

func TestHelloPlugin_Init(t *testing.T) {
    plugin := NewHelloPlugin()
    ctx := plugins.NewPluginContext(
        map[string]interface{}{"greeting": "test"},
        &mockLogger{},
        "./tmp/test-data",
        &mockRuntimeAPI{},
    )
    
    err := plugin.Init(*ctx)
    if err != nil {
        t.Fatalf("Init failed: %v", err)
    }
}

type mockLogger struct{}
func (l *mockLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *mockLogger) Info(msg string, keysAndValues ...interface{}) {}
func (l *mockLogger) Warn(msg string, keysAndValues ...interface{}) {}
func (l *mockLogger) Error(msg string, keysAndValues ...interface{}) {}

type mockRuntimeAPI struct{}
func (a *mockRuntimeAPI) GetConfig(key string) (interface{}, error) { return nil, nil }
func (a *mockRuntimeAPI) SetConfig(key string, value interface{}) error { return nil }
func (a *mockRuntimeAPI) EmitEvent(eventType string, data interface{}) error { return nil }
func (a *mockRuntimeAPI) RegisterHook(hookPoint string, handler func(args interface{}) error) (string, error) {
    return "hook-1", nil
}
func (a *mockRuntimeAPI) UnregisterHook(hookID string) error { return nil }
```

---

## Example Plugin List

| Plugin | Description | Links |
|--------|-------------|-------|
| `hello-cosca` | Hello World example | [Source](../../examples/plugins/hello-world.md) |
| `knowledge-enhancer` | Custom search ranking | — |
| `git-integration` | Git-aware features | — |
| `code-analyzer` | Static code analysis | — |

---

> **Related**: [Plugin System Overview](overview.md) | [ADR-003](../adr/ADR-003-plugin-system.md)
