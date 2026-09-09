# Editor Integration

> **Status**: active | **Owner**: CLI Chief | **Last Updated**: 2026-07-23

## Overview

The Editor Integration system enables Cosca to seamlessly work with your preferred AI-assisted development environment. It automatically detects which editor you're using, sets up the necessary configuration, and integrates Cosca capabilities into your editor's workflow.

```
┌──────────────────────────────────────────────────────────────────┐
│                      EDITOR INTEGRATION                           │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                     EDITOR MANAGER                         │    │
│  │                                                           │    │
│  │  • Detect editors  • Setup integration                   │    │
│  │  • Validate config  • Teardown                           │    │
│  │  • Auto configuration                                    │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                    EDITOR ADAPTERS                         │    │
│  │                                                           │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────┐  │    │
│  │  │ OpenCode │ │ClaudeCode│ │  Codex   │ │  Cursor    │  │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └────────────┘  │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────┐  │    │
│  │  │ VS Code  │ │ Neovim   │ │ Windsurf │ │    Zed     │  │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └────────────┘  │    │
│  │  ┌──────────────┐                                        │    │
│  │  │ Generic MCP  │  ← Any MCP-compatible editor           │    │
│  │  └──────────────┘                                        │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                  INTEGRATION METHODS                       │    │
│  │                                                           │    │
│  │  • Config file setup     • MCP server registration       │    │
│  │  • Custom instructions   • Tool definitions               │    │
│  │  • Runtime API bindings                                   │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

---

## Supported Editors

| Editor | Adapter Package | Detection | Features |
|--------|----------------|-----------|----------|
| **OpenCode** | `editors/opencode` | Config file check | Full Cosca integration via MCP |
| **Claude Code** | `editors/claude` | Config file check | MCP server, custom instructions |
| **Codex** | `editors/codex` | Config file check | Custom instructions |
| **Cursor** | `editors/cursor` | Config file check | Rules configuration |
| **VS Code** | `editors/vscode` | Settings check | Settings, tasks, MCP |
| **Neovim** | `editors/neovim` | Plugin check | Lua configuration |
| **Windsurf** | `editors/windsurf` | Config check | Custom rules |
| **Zed** | `editors/zed` | Settings check | Settings configuration |
| **Generic MCP** | `editors/generic_mcp` | File detection | MCP-compatible setup |

---

## Editor Interface

Each adapter implements the `Editor` interface:

```go
type Editor interface {
    Name() string              // Editor name
    Detect() (bool, error)     // Is this editor present?
    Info() (EditorInfo, error) // Get editor info
    Setup(EditorConfig) error  // Configure Cosca integration
    Validate() error           // Check if configured
    Teardown() error           // Remove Cosca integration
}
```

---

## Setup Guide

### Automatic Setup

The easiest way to set up editor integration is via `cosca install`:

```bash
# Full auto-install (detects editor and configures everything)
cosca install

# Or detect and setup editor separately
cosca editor detect          # Shows detected editor
cosca editor setup           # Sets up the detected editor
```

### Manual Setup by Editor

| Editor | Setup Command |
|--------|---------------|
| OpenCode | `cosca editor setup opencode` |
| Claude Code | `cosca editor setup claude` |
| Codex | `cosca editor setup codex` |
| Cursor | `cosca editor setup cursor` |
| VS Code | `cosca editor setup vscode` |
| Neovim | `cosca editor setup neovim` |
| Windsurf | `cosca editor setup windsurf` |
| Zed | `cosca editor setup zed` |

### Detection Priority

When multiple editors are available, the detection follows this priority:

```
1. OpenCode  (highest priority)
2. Codex
3. Cursor
4. Claude Code
5. VS Code
6. Neovim
7. Windsurf
8. Generic MCP
```

---

## MCP Integration

Many editors support the **Model Context Protocol (MCP)**, which enables Cosca to provide search, memory, and knowledge capabilities directly within the editor.

### What MCP Provides

- **Knowledge search** — Search project knowledge without leaving the editor
- **Memory access** — Store and retrieve context across sessions
- **Runtime commands** — Execute Cosca commands from within the editor
- **Real-time updates** — Receive indexing and sync notifications

### MCP Server Setup

```bash
# Automatic setup (recommended)
cosca editor setup

# Manual MCP configuration
cosca editor setup generic-mcp --mcp-only
```

See [MCP Server Example](../../examples/editors/mcp-server.md) for detailed setup.

---

## CLI Commands

```bash
# Detect the active editor
cosca editor detect

# List all supported editors
cosca editor list

# Setup Cosca integration for an editor
cosca editor setup [editor-name]

# Validate Cosca integration
cosca editor validate [editor-name]

# Remove Cosca integration
cosca editor teardown [editor-name]

# Auto-detect and setup
cosca install
```

### Examples

```bash
$ cosca editor detect
Detected editor: opencode (version 1.5.0)
  Setup required: false (already configured)

$ cosca editor list
  Editor         Detected  Configured
  opencode       yes       yes
  claude         no        no
  codex          no        no
  cursor         no        no
  vscode         yes       no
  neovim         no        no
  windsurf       no        no
  zed            no        no

$ cosca editor setup vscode
Setting up Cosca for VS Code...
✅ VS Code integration configured
✅ MCP server registered
✅ Workspace settings updated
```

---

## Editor-Specific Features

Each editor adapter provides editor-specific integration:

### OpenCode
- Custom instructions via `.opencode/AGENTS.md`
- MCP server for knowledge search
- Skill and tool definitions
- See [OpenCode Integration](opencode.md)

### Claude Code
- Custom instructions via `CLAUDE.md`
- MCP server setup
- Tool definitions
- See [Claude Code Integration](claude.md)

### VS Code
- Workspace settings (`.vscode/settings.json`)
- MCP configuration
- Task definitions
- See [VS Code Integration](vscode.md)

---

## Validation

After setup, you can validate the integration:

```bash
# Validate specific editor
cosca editor validate opencode

# Validation checks
cosca doctor                    # Full system diagnostics
```

---

## Troubleshooting

```bash
# Editor not detected
cosca editor detect --verbose

# Reset editor configuration
cosca editor teardown
cosca editor setup

# Check logs
cosca doctor
```

---

> **Related**: [OpenCode Integration](opencode.md) | [Claude Code Integration](claude.md) | [VS Code Integration](vscode.md)
