# Plugin Runtime Specification

> **Version**: 1.0.0 | **Status**: active | **Owner**: Plugin Chief | **Last Updated**: 2026-07-23

## Purpose
Define the official Plugin Runtime that loads, sandboxes, and manages plugin execution within the Cosca framework.

## Components

### 1. PluginRegistry
- **Purpose**: Discover, load, and index plugins
- **Scan paths**: `~/.cosca/plugins/`, `./plugins/`, `COSCA_PLUGIN_PATH` env
- **Caching**: Manifest cache with TTL
- **Validation**: Validates manifest against schema on load

### 2. PluginSandbox
- **Purpose**: Isolate plugin execution from host
- **Isolation modes**: Thread (Node.js Worker), Process, VM
- **Resource limits**: CPU share, memory max, execution timeout
- **Capabilities**: Permissions-based API access

### 3. PluginLifecycle
- **Purpose**: Manage plugin lifecycle states
- **States**: loaded → configured → started → running → stopped → unloaded
- **Error handling**: Retry on transient failures, quarantine on permanent
- **Hot-reload**: Stop, update, start without affecting other plugins

### 4. PluginAPI
- **Purpose**: API surface available to plugins
- **Services**: Config, Events, Logging, HTTP, Storage, Cache
- **Rate limiting**: Per-plugin API call limits
- **Authentication**: Plugin identity tokens

## Runtime API

| Method | Description | Permission Required |
|--------|-------------|-------------------|
| `config.get(key)` | Get configuration value | — (always) |
| `config.set(key, value)` | Set configuration value | config:write |
| `events.publish(name, payload)` | Publish event | events:publish |
| `events.subscribe(pattern, handler)` | Subscribe to events | events:subscribe |
| `http.request(options)` | Make HTTP request | network:http |
| `storage.get(key)` | Read from plugin storage | storage:read |
| `storage.set(key, value)` | Write to plugin storage | storage:write |
| `logger.info(msg)` | Log info message | — (always) |
| `logger.error(msg)` | Log error message | — (always) |

## Error Handling

| Error | Action | Retry |
|-------|--------|-------|
| Plugin init failure | Log, disable plugin | 0 |
| Plugin crash | Log, restart with backoff | 3 (exponential) |
| Resource limit exceeded | Kill plugin, notify admin | 0 |
| Permission violation | Log, disable feature | 0 |

## Related
- [PLUGIN_SYSTEM.md](../PLUGIN_SYSTEM.md)
- [Plugin Chief](../../departments/plugin/SKILL.md)
- [Plugin Template](../../templates/plugin/TEMPLATE.md)
