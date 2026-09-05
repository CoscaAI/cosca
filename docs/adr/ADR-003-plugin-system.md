# ADR-003: Multi-Runtime Plugin Architecture

> **Status**: Accepted | **Owner**: Plugin Chief | **Last Updated**: 2026-07-23

## Context

The Cosca platform must support extensibility without compromising stability, security, or performance. Third-party developers need to extend Cosca capabilities, but the system must protect against:

- Malicious plugins accessing unauthorized resources
- Buggy plugins crashing the host process
- Performance degradation from poorly written plugins
- Dependency conflicts between plugins

Different use cases demand different trade-offs between performance, isolation, and language flexibility.

### Requirements

1. Plugin developers can use any language (Go, TypeScript, Python, Rust, etc.)
2. Plugins have a sandboxed execution environment with configurable permissions
3. Plugin lifecycle is managed (install, init, start, stop, remove)
4. Plugins can register hooks and communicate via events
5. Plugin manifests declare dependencies, permissions, and configuration schema
6. Plugin integrity is verified via checksums

## Decision

We adopt a **multi-runtime plugin architecture** supporting four execution runtimes, each optimized for a specific use case.

### Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                         PLUGIN MANAGER                                │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                     REGISTRY                                    │    │
│  │  Plugin manifests  │  Version resolution  │  Dependency graph  │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                    LIFECYCLE MANAGER                            │    │
│  │  Install → Init → Start → Stop → Remove                        │    │
│  │  Dependency-ordered initialization and shutdown                │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                    SECURITY LAYER                               │    │
│  │  Permission enforcement  │  Checksum verification             │    │
│  │  Timeout control  │  Memory limits  │  Audit logging          │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                    RUNTIME ADAPTERS                             │    │
│  │                                                                   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐    │    │
│  │  │   Go     │  │   WASM   │  │ External │  │  Shared Lib   │    │    │
│  │  │  Native  │  │  Sandbox │  │ Process  │  │  .so/.dll     │    │    │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────────┘    │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                    COMMUNICATION LAYER                          │    │
│  │  Hook system (typed callback points)                          │    │
│  │  Event bus (pub/sub for cross-plugin communication)           │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

### Runtime Comparison

| Runtime | Technology | Isolation | Performance | Language Support | Use Case |
|---------|-----------|-----------|-------------|-----------------|----------|
| **Go Native** | `plugin.Open` | Low (same process) | Best | Go only | First-party, trusted plugins |
| **WASM** | WebAssembly | High (sandboxed) | Good | Go, Rust, C, AssemblyScript | Third-party plugins |
| **External** | Child process / gRPC | Highest (separate process) | Moderate | Any language | Untrusted or polyglot plugins |
| **SharedLib** | `.so` / `.dll` | Medium | Good | C, C++, Rust | Performance-critical, trusted |

### Plugin Manifest

```yaml
# plugin.yaml — Required manifest for all runtimes
id: "my-plugin"              # Unique identifier
name: "My Plugin"            # Human-readable name
version: "1.0.0"             # Semantic version
runtime: "wasm"              # go | wasm | external | sharedlib
author: "Developer <dev@example.com>"
description: "Extends Cosca with custom capabilities"

permissions:                 # Declared at install time
  - filesystem               # Read/write files in plugin data directory
  - network                  # Make HTTP requests

dependencies:                # Plugin dependencies
  - plugin_id: "cosca-core"
    version: ">=1.0.0"
    optional: false

hooks:                       # Hook points subscribed to
  - on_search
  - on_index

config_schema:               # JSON Schema for plugin configuration
  type: object
  properties:
    api_key:
      type: string

entry_point: "./bin/plugin.wasm"  # For wasm/external/sharedlib
checksum: "sha256-abc123..."       # Integrity verification
```

## Rationale

### Why Multiple Runtimes?

| Concern | Single Runtime Solution | Multi-Runtime Solution |
|---------|------------------------|----------------------|
| Performance | Go native only | Go native for perf-critical + WASM for safe |
| Language choice | Go only | Any language via external process |
| Security | In-process, no isolation | WASM sandbox + external process isolation |
| Complexity | Simple | More complex but flexible |

No single runtime satisfies all requirements. Go native is fast but limited to Go and offers no isolation. WASM provides safety but has overhead for certain operations. External processes allow any language but incur latency. Supporting all four gives plugin authors the right tool for each job.

### Why WASM over other sandbox technologies?

| Technology | Verdict |
|------------|---------|
| WASM | Chosen — Sandboxed by design, portable, growing ecosystem, multiple language targets |
| gVisor | Too heavy for a CLI tool, requires kernel support |
| Docker containers | Overkill for individual plugins, high overhead |
| Deno/isolates | Tied to JavaScript/TypeScript ecosystem |
| NaCL/PNaCl | Deprecated, Chrome-only |

### Why Go plugin.Open for native plugins?

- **Simple** — No IPC, no serialization, direct function calls
- **Fast** — Function calls are nanoseconds vs. microseconds for IPC
- **Go-native** — Shares the same runtime, GC, and type system
- **Mature** — Supported since Go 1.8

The risk (in-process crashes) is acceptable for first-party and curated plugins with strict code review.

## Alternatives Considered

### Go Native Only (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Maximum performance, simplest implementation |
| Cons | Limited to Go, no isolation, same-process crashes |
| Verdict | Rejected — language lock-in and security concerns |

### WASM Only (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Sandboxed, multi-language, portable |
| Cons | Performance overhead, limited system access, immature ecosystem |
| Verdict | Rejected — performance-critical plugins need native execution |

### External Subprocess Only (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Maximum isolation, any language |
| Cons | Serialization overhead, process management complexity, higher latency |
| Verdict | Rejected — latency too high for hook-based callbacks |

## Consequences

### Positive

- **Language independence** — Plugin authors can use Go, Rust, TypeScript, Python, or any language
- **Appropriate isolation** — Untrusted plugins run in WASM sandbox; trusted plugins run natively
- **Versioned manifests** — Clear dependency management with semantic versioning
- **Hooks + Events** — Rich integration model for plugins to extend Cosca behavior
- **Checksum verification** — Ensures plugin integrity at install and load time

### Negative

- **Four runtimes to maintain** — Each runtime adapter has different implementation requirements
- **WASM compilation toolchain** — Plugin authors need `tinygo` or `wasm-pack` for WASM plugins
- **SharedLib portability** — `.so`/`.dll` files are platform-specific; must be built per target
- **Error handling complexity** — Each runtime has different error propagation semantics

### Neutral

- **Plugin manifest format** — YAML is human-readable but requires schema validation
- **Runtime negotiation** — The system selects the appropriate adapter based on plugin manifest

---

**Related**: [Plugin Overview](../plugins/overview.md) | [Plugin Development](../plugins/development.md) | [ADR-001](ADR-001-cosca-cli-architecture.md)
