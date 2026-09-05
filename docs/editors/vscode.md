# VS Code Integration

> **Status**: active | **Owner**: CLI Chief | **Last Updated**: 2026-07-23

## Overview

Cosca integrates with **Visual Studio Code** through workspace settings, MCP configuration, and task definitions, enabling knowledge search and runtime management directly from VS Code.

---

## Setup

```bash
# Auto-detect and setup
cosca install

# Manual setup
cosca editor setup vscode
```

### What Gets Configured

1. **Workspace Settings** — `.vscode/settings.json` updated
2. **MCP Configuration** — VS Code MCP settings updated
3. **Task Definitions** — `.vscode/tasks.json` for Cosca commands

---

## Integration Details

### VS Code Settings

```json
// .vscode/settings.json
{
  "cosca.enabled": true,
  "cosca.dataDir": "${workspaceFolder}/.cosca",
  "cosca.search.limit": 20,
  "cosca.watch.enabled": true
}
```

### MCP Configuration

Cosca registers an MCP server that VS Code can use for knowledge search:

```json
// .vscode/mcp.json or VS Code MCP settings
{
  "servers": {
    "cosca-knowledge": {
      "command": "cosca",
      "args": ["knowledge", "mcp"],
      "type": "stdio"
    }
  }
}
```

### Tasks

```json
// .vscode/tasks.json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "Cosca: Search Knowledge",
      "type": "shell",
      "command": "cosca knowledge search ${input:query}",
      "problemMatcher": []
    },
    {
      "label": "Cosca: Sync Index",
      "type": "shell",
      "command": "cosca sync",
      "problemMatcher": []
    },
    {
      "label": "Cosca: System Status",
      "type": "shell",
      "command": "cosca status",
      "problemMatcher": []
    }
  ]
}
```

---

## Features

### Knowledge Search

Press `Ctrl+Shift+P` and run:
- `Tasks: Run Task` → `Cosca: Search Knowledge`
- Or use the integrated terminal: `cosca knowledge search "query"`

### Index Management

- Auto-indexing on file save (via file watcher)
- Manual sync via task: `Cosca: Sync Index`

### Status Monitoring

- Check runtime health via `cosca status`
- View metrics and component health

---

## Validation

```bash
cosca editor validate vscode
# Checks:
# - VS Code executable found
# - Workspace settings configured
# - MCP server registered
```

---

> **Related**: [Editor Overview](overview.md) | [OpenCode Integration](opencode.md) | [Claude Code Integration](claude.md)
