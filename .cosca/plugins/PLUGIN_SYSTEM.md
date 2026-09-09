# Cosca Plugin System

> **Version**: 1.0.0 | **Status**: active | **Owner**: Plugin Chief | **Last Updated**: 2026-07-23

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    HOST APPLICATION                       │
│  ┌─────────────────────────────────────────────────────┐ │
│  │                 PLUGIN RUNTIME                       │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐          │ │
│  │  │ Registry  │  │ Sandbox  │  │ Lifecycle │          │ │
│  │  │  (plugins)│  │ (isolated)│  │ (hooks)   │          │ │
│  │  └────┬─────┘  └────┬─────┘  └────┬──────┘          │ │
│  │       │              │             │                  │ │
│  │  ┌────▼──────────────▼─────────────▼──────┐          │ │
│  │  │          PLUGIN API LAYER              │          │ │
│  │  │  (config, events, resources, logging)  │          │ │
│  │  └────────────────┬───────────────────────┘          │ │
│  └───────────────────┼───────────────────────────────────┘ │
│                      │                                     │
│  ┌───────────────────┼───────────────────────────────────┐ │
│  │  ┌────────────────▼──────────────────────────────┐    │ │
│  │  │              PLUGIN INSTANCES                   │    │ │
│  │  │  ┌─────────┐  ┌─────────┐  ┌─────────┐        │    │ │
│  │  │  │ Plugin A│  │ Plugin B│  │ Plugin C│  ...    │    │ │
│  │  │  └─────────┘  └─────────┘  └─────────┘        │    │ │
│  │  └────────────────────────────────────────────────┘    │ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Plugin Manifest

Every plugin must have a `plugin.json` manifest:

```json
{
  "id": "my-plugin",
  "version": "1.0.0",
  "name": "My Plugin",
  "description": "Description of what the plugin does",
  "author": "Author Name",
  "license": "MIT",
  "entry": "dist/index.js",
  "runtime": "nodejs",
  "permissions": ["api:read", "events:publish", "files:read"],
  "dependencies": {
    "cosca": ">=1.0.0"
  },
  "lifecycle": {
    "init": true,
    "start": true,
    "stop": true,
    "configChange": true
  },
  "config": {
    "properties": {
      "apiKey": { "type": "string", "required": true },
      "debug": { "type": "boolean", "default": false }
    }
  }
}
```

## Lifecycle Hooks

| Hook | Trigger | Description |
|------|---------|-------------|
| `init(config)` | Plugin loaded | Initialize plugin with configuration |
| `start()` | Plugin activated | Start background tasks, connections |
| `stop()` | Plugin deactivated | Graceful shutdown, cleanup |
| `configChange(newConfig)` | Config updated | Handle configuration changes at runtime |
| `event(event)` | Event published | Handle events from event bus |

## Plugin SDK

The Plugin SDK provides a base class for plugin development:

```typescript
import { PluginBase, PluginConfig, PluginContext } from '@cosca/plugin-sdk';

export class MyPlugin extends PluginBase {
  async init(config: PluginConfig): Promise<void> {
    // Initialize with configuration
    this.logger.info('Plugin initialized');
  }

  async start(): Promise<void> {
    // Start plugin operations
  }

  async stop(): Promise<void> {
    // Graceful shutdown
  }

  async handleEvent(event: any): Promise<void> {
    // Handle events
  }
}
```

## Security Model

- **Sandboxing**: Plugins run in isolated context (via `isolated-vm` or `vm2`)
- **Permissions**: Manifest declares required permissions (principle of least privilege)
- **Resource Limits**: CPU, memory, and execution time limits enforced
- **No Filesystem Access**: Unless explicitly granted via `files:*` permission
- **No Network Access**: Unless explicitly granted via `network:*` permission
- **Audit Trail**: All plugin actions logged

## Registry

Plugins are registered in the Plugin Registry:

| Plugin | Version | Status | Author | Permissions |
|--------|---------|--------|--------|-------------|
| analytics-dashboard | 1.2.0 | active | Cosca Core | api:read, events:subscribe |
| slack-notifier | 2.0.1 | active | Community | api:read, events:subscribe, network:webhook |
| custom-auth | 0.5.0 | draft | Internal | api:write, files:read |

## Related
- [Plugin Chief](../departments/plugin/SKILL.md)
- [SDK SDK](../sdk/typescript/README.md)
- [Plugin Template](../templates/plugin/TEMPLATE.md)
- [Security Chief](../departments/security/SKILL.md)
