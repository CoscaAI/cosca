# TEMPLATE: Plugin

> **Version**: 1.0.0 | **Status**: active | **Owner**: Plugin Chief | **Last Updated**: 2026-07-23

## DOMAIN
Extensible plugin modules for platform extensions, third-party integrations, and custom functionality additions.

## RECOMMENDED STACK
| Layer | Technology |
|-------|-----------|
| Language | TypeScript / Go / Python |
| Plugin SDK | @platform/plugin-sdk |
| Sandbox | isolated-vm / WebAssembly |
| Registry | npm / GitHub Packages |
| Documentation | TypeDoc / MkDocs |
| Testing | Vitest / Go test / pytest |

## MODULE STRUCTURE
```
plugin-name/
├── src/
│   ├── index.ts               # Plugin entry point
│   ├── plugin.ts              # Plugin class implementation
│   ├── hooks/                 # Lifecycle hooks
│   │   ├── init.ts
│   │   ├── start.ts
│   │   └── stop.ts
│   ├── api/                   # Plugin API handlers
│   ├── config/                # Plugin configuration
│   │   └── schema.ts          # Config schema definition
│   ├── ui/                    # Plugin UI components (if applicable)
│   └── types/                 # Type definitions
├── tests/
│   ├── unit/
│   └── integration/
├── examples/
│   └── basic-usage.ts
├── plugin.json                # Plugin manifest
├── docs/
│   ├── README.md
│   ├── API.md
│   └── CONFIGURATION.md
├── package.json
├── tsconfig.json
└── README.md
```

## KEY FEATURES
- Plugin manifest with metadata, permissions, dependencies
- Lifecycle hooks (init, start, stop, config-change)
- Configuration schema with validation
- Resource limits and sandboxing
- API extensions through plugin endpoints
- UI extensions through plugin components
- Event subscription and publishing
- Hot-reload support
- Version compatibility declaration
- Dependency declaration and resolution

## ARCHITECTURE NOTES
- Plugin runs in sandboxed environment with resource limits
- Plugin communicates through well-defined plugin API
- Plugin manifest declares required permissions (principle of least privilege)
- Plugin configuration validated against schema on load
- Plugin lifecycle hooks are async with timeout
- Plugin errors are isolated — cannot crash host system
- Plugin hot-reload does not affect running plugins
- Plugin version declared in manifest (SemVer)
- Plugin dependencies must be explicit and versioned

## RELATED
- [Plugin Chief](../../departments/plugin/SKILL.md)
- [SDK Template](../sdk/TEMPLATE.md)
- [Platform Chief](../../departments/platform/SKILL.md)
- [Architecture Chief](../../departments/architecture/SKILL.md)
