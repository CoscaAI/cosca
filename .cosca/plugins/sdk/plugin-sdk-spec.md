# Plugin SDK Specification

> **Version**: 1.0.0 | **Status**: active | **Owner**: Plugin Chief | **Last Updated**: 2026-07-23

## Purpose
Define the standard Plugin SDK that plugin developers use to build Cosca-compatible plugins.

## Base Class

```typescript
abstract class PluginBase {
  // ------- Properties -------
  protected id: string;
  protected name: string;
  protected version: string;
  protected config: PluginConfig;
  protected logger: PluginLogger;
  protected api: PluginAPI;
  protected context: PluginContext;

  // ------- Lifecycle -------
  abstract init(config: PluginConfig): Promise<void>;
  abstract start(): Promise<void>;
  abstract stop(): Promise<void>;
  
  // Optional hooks
  onConfigChange?(config: PluginConfig): Promise<void>;
  onEvent?(event: PluginEvent): Promise<void>;

  // ------- Built-in methods -------
  protected async publishEvent(name: string, payload: any): Promise<void>;
  protected async subscribeEvent(pattern: string, handler: Function): Promise<void>;
  protected async httpRequest(options: HttpOptions): Promise<HttpResponse>;
  protected async storageGet(key: string): Promise<any>;
  protected async storageSet(key: string, value: any): Promise<void>;
}
```

## Plugin Manifest Schema

```json
{
  "$schema": "https://cosca.dev/plugin-manifest-schema.json",
  "type": "object",
  "required": ["id", "version", "name", "entry"],
  "properties": {
    "id": { "type": "string", "pattern": "^[a-z0-9-]+$" },
    "version": { "type": "string", "pattern": "^\\d+\\.\\d+\\.\\d+$" },
    "name": { "type": "string" },
    "description": { "type": "string" },
    "author": { "type": "string" },
    "license": { "type": "string" },
    "entry": { "type": "string" },
    "runtime": { "type": "string", "enum": ["nodejs", "python", "wasm"] },
    "permissions": { 
      "type": "array",
      "items": { 
        "type": "string",
        "enum": ["api:read", "api:write", "events:publish", "events:subscribe",
                 "files:read", "files:write", "network:http", "network:websocket",
                 "storage:read", "storage:write", "config:write", "admin"]
      }
    },
    "dependencies": {
      "type": "object",
      "properties": {
        "cosca": { "type": "string" }
      }
    },
    "lifecycle": {
      "type": "object",
      "properties": {
        "init": { "type": "boolean" },
        "start": { "type": "boolean" },
        "stop": { "type": "boolean" },
        "configChange": { "type": "boolean" }
      }
    }
  }
}
```

## Built-in Plugins

| Plugin | Description | Status |
|--------|-------------|--------|
| analytics-dashboard | Real-time Cosca metrics dashboard | planned |
| slack-notifier | Send Cosca events to Slack | planned |
| github-integration | Sync Cosca workflows with GitHub | planned |
| custom-auth | Custom authentication provider | planned |

## Related
- [PLUGIN_SYSTEM.md](../PLUGIN_SYSTEM.md)
- [Plugin Runtime](../runtime/plugin-runtime-spec.md)
- [Plugin Template](../../templates/plugin/TEMPLATE.md)
