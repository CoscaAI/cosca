# ADR-004: Editor Adapter Pattern

> **Status**: Accepted | **Owner**: CLI Chief | **Last Updated**: 2026-07-23

## Context

The Cosca platform must integrate with multiple AI-powered editors and development environments. Each editor has a unique configuration format, file structure, and integration mechanism. The supported editors include:

- **OpenCode** — `.opencode/AGENTS.md`, MCP server, custom instructions
- **Claude Code** — `CLAUDE.md`, MCP server, custom instructions
- **Codex** — Custom instructions configuration
- **Cursor** — `.cursor/rules/`, rules configuration
- **VS Code** — `.vscode/settings.json`, MCP configuration, tasks
- **Neovim** — Lua configuration files, plugins
- **Windsurf** — `.windsurf/`, custom rules
- **Zed** — `settings.json`, task definitions
- **Generic MCP** — Any editor supporting the Model Context Protocol

Without a unified integration pattern, adding support for each new editor would require significant duplicated effort and result in inconsistent behavior.

## Decision

We adopt the **Adapter Pattern** with a common `Editor` interface that all editor integrations implement.

### Editor Interface

```go
// Editor defines the contract every editor adapter must satisfy.
type Editor interface {
    // Name returns the editor identifier (e.g., "opencode", "vscode").
    Name() string

    // Detect checks whether this editor is present in the current environment.
    Detect() (bool, error)

    // Info returns metadata about the detected editor installation.
    Info() (EditorInfo, error)

    // Setup configures the editor for Cosca integration.
    Setup(EditorConfig) error

    // Validate checks whether Cosca integration is properly configured.
    Validate() error

    // Teardown removes all Cosca integration artifacts from the editor.
    Teardown() error
}

// EditorInfo contains metadata about an editor installation.
type EditorInfo struct {
    Name      string // Editor name
    Version   string // Detected version
    Path      string // Installation path
    ConfigDir string // Configuration directory
}

// EditorConfig contains parameters for editor setup.
type EditorConfig struct {
    MCPPort     int    // Port for MCP server (0 = auto)
    InstallDir  string // Cosca installation directory
    Verbose     bool   // Enable verbose output
    AutoDetect  bool   // Auto-detect configuration
}
```

### Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                        EDITOR MANAGER                                 │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                    ADAPTER REGISTRY                             │    │
│  │  Maps editor names → adapter implementations                  │    │
│  │  Maintains detection priority order                           │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                     EDITOR ADAPTERS                             │    │
│  │                                                                   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────┐      │    │
│  │  │ OpenCode │  │ClaudeCode│  │  Codex   │  │  Cursor    │      │    │
│  │  └──────────┘  └──────────┘  └──────────┘  └────────────┘      │    │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────┐      │    │
│  │  │ VS Code  │  │ Neovim   │  │ Windsurf │  │    Zed     │      │    │
│  │  └──────────┘  └──────────┘  └──────────┘  └────────────┘      │    │
│  │  ┌────────────────┐                                              │    │
│  │  │ Generic MCP    │  ← Fallback for any MCP-compatible editor    │    │
│  │  └────────────────┘                                              │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                    INTEGRATION METHODS                          │    │
│  │                                                                   │    │
│  │  • Config file setup       • MCP server registration            │    │
│  │  • Custom instructions     • Tool definitions                   │    │
│  │  • Task definitions        • Workspace settings                 │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                    MCP TRANSPORT LAYER                          │    │
│  │  STDIO  │  HTTP/SSE  │  WebSocket                              │    │
│  │  Tools: knowledge_search, memory_store, runtime_status         │    │
│  │  Resources: cosca://knowledge, cosca://memory, cosca://runtime       │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

### Detection Strategy

Each editor adapter implements a `Detect()` method that checks for editor-specific markers:

```go
// Example: OpenCode detection
func (a *OpenCodeAdapter) Detect() (bool, error) {
    // Check for .opencode directory or config file
    paths := []string{
        filepath.Join(projectDir, ".opencode"),
        filepath.Join(homeDir, ".config", "opencode"),
    }
    for _, p := range paths {
        if _, err := os.Stat(p); err == nil {
            return true, nil
        }
    }
    return false, nil
}
```

### Detection Priority

When multiple editors are detected, the following priority determines which adapter is used:

| Priority | Editor | Detection Marker |
|----------|--------|-----------------|
| 1 | OpenCode | `.opencode/` directory |
| 2 | Codex | `.codex/` directory |
| 3 | Cursor | `.cursor/` directory |
| 4 | Claude Code | `CLAUDE.md` file |
| 5 | VS Code | `.vscode/` directory |
| 6 | Neovim | `init.lua` or `init.vim` |
| 7 | Windsurf | `.windsurf/` directory |
| 8 | Zed | `settings.json` patterns |
| 9 | Generic MCP | Always available fallback |

## Rationale

### Why the Adapter Pattern?

1. **Single interface, multiple implementations** — Every editor follows the same contract
2. **Easy to add new editors** — Implement 5 methods and register the adapter
3. **Consistent behavior** — Users get the same experience regardless of editor
4. **Independent testing** — Each adapter can be tested in isolation
5. **Graceful degradation** — If one editor's setup fails, others are unaffected

### Why an MCP Transport Layer?

The Model Context Protocol (MCP) provides a universal integration mechanism. By implementing MCP, Cosca can integrate with any MCP-compatible editor without a custom adapter. This future-proofs the integration strategy and reduces the maintenance burden.

### Core vs. MCP-only editors

| Editor Type | Integration Method |
|-------------|-------------------|
| Core adapters (8 editors) | Direct config file manipulation + MCP |
| Generic MCP | MCP protocol only |

Core adapters provide richer integration by directly editing editor configuration files. The Generic MCP adapter serves as a fallback for any editor that supports the MCP standard.

## Alternatives Considered

### Monolithic Editor Support (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Single code path, no abstraction overhead |
| Cons | Tight coupling, difficult to extend, high risk of regression |
| Verdict | Rejected — violates Open/Closed Principle |

### Separate Tools per Editor (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Editor-specific optimization, independent tooling |
| Cons | Fragmentation, inconsistent UX, duplicated configuration logic |
| Verdict | Rejected — inconsistent user experience across editors |

### Single MCP-only Approach (Rejected)

| Aspect | Assessment |
|--------|------------|
| Pros | Universal, editor-agnostic |
| Cons | Cannot manipulate editor-specific config files, limited to MCP capabilities |
| Verdict | Rejected — cannot set up custom instructions, rules, or workspace settings |

## Consequences

### Positive

- **Consistent API** — All editors share the same `Editor` interface
- **Easy extensibility** — Adding a new editor requires ~100 lines of Go code
- **MCP universality** — Any MCP-compatible editor is supported out of the box
- **Auto-detection** — Users run `cosca install` and their editor is configured automatically
- **Isolated testing** — Each adapter can be tested independently

### Negative

- **Adapter maintenance** — Each editor adapter must be updated when the editor changes its config format
- **Detection complexity** — Some editors have overlapping detection markers requiring priority logic
- **MCP versioning** — MCP specification updates require coordinated adapter updates

### Neutral

- **Generic MCP fallback** — Serves as both a safety net and the integration path for unsupported editors
- **Config file ownership** — Adapters modify editor config files; conflicts with manual edits are possible

---

**Related**: [Editor Overview](../editors/overview.md) | [OpenCode Integration](../editors/opencode.md) | [Claude Code Integration](../editors/claude.md) | [VS Code Integration](../editors/vscode.md) | [ADR-001](ADR-001-cosca-cli-architecture.md)
