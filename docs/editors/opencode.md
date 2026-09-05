# OpenCode Integration

> **Status**: active | **Owner**: CLI Chief | **Last Updated**: 2026-07-23

## Overview

Cosca integrates natively with **OpenCode**, providing the full suite of Cosca capabilities — knowledge search, memory, runtime management, and plugin system — directly within the OpenCode environment.

---

## Setup

OpenCode is detected automatically during `cosca install`:

```bash
# Auto-detect and setup
cosca install

# Or manual setup
cosca editor setup opencode
```

### What Gets Configured

1. **MCP Server** — Registered in OpenCode's MCP configuration
2. **Custom Instructions** — Agent instructions added to `.opencode/`
3. **Knowledge Integration** — Cosca knowledge base accessible from OpenCode
4. **Runtime API** — Cosca commands available as tools

---

## How It Works

```
OpenCode Agent
    │
    ├── MCP Protocol ──▶ Cosca Knowledge Search
    │                       │
    │                       └──▶ FTS5 + Vector + Graph
    │
    ├── Custom Instructions ──▶ Cosca Agent Instructions
    │
    └── Shell Commands ──▶ cosca search, cosca memory, etc.
```

### MCP Tools Available

| Tool | Description |
|------|-------------|
| `cosca_knowledge_search` | Search the knowledge base |
| `cosca_memory_store` | Store a memory record |
| `cosca_memory_search` | Search memory records |
| `cosca_status` | Get system status |
| `cosca_doctor` | Run diagnostics |

---

## Configuration Files

When OpenCode integration is set up, Cosca creates/updates:

### `.opencode/AGENTS.md`

This file instructs the OpenCode agent on how to use Cosca:

```markdown
# Cosca Integration

This project uses Cosca for knowledge management, memory, and runtime orchestration.

## Available Commands
- `cosca knowledge search <query>` — Search project knowledge
- `cosca memory store --type decision --content "..."` — Store decisions
- `cosca memory search <query>` — Retrieve past decisions
- `cosca status` — Check system health
- `cosca sync` — Sync knowledge index

## Knowledge Base
The Cosca knowledge base contains indexed documentation, code, and decisions.
Use `cosca knowledge search` before implementing new features to learn from
past architecture decisions and patterns.
```

### MCP Configuration

OpenCode's MCP configuration is updated to include the Cosca MCP server, enabling direct tool access from the agent.

---

## Features

### 1. Knowledge-Aware Development

When implementing features, OpenCode agents can:
- Search the Cosca knowledge base for relevant architecture decisions
- Retrieve past patterns and solutions
- Access indexed documentation without context switching

### 2. Persistent Memory

Decisions and context persist across OpenCode sessions:
- Store decisions with `cosca memory store`
- Retrieve past context with `cosca memory search`
- Patterns accumulate and improve over time

### 3. Runtime Management

Manage Cosca runtime directly from OpenCode:
- Start/stop the runtime daemon
- Check system health
- Run diagnostics

---

## Example Workflow

```
User: "Implement user authentication"

OpenCode Agent:
  1. Runs: cosca knowledge search "authentication architecture"
  2. Finds past ADRs and patterns
  3. Stores session context: cosca memory store --type session --content "Implementing auth"
  4. Implements the feature
  5. Stores decision: cosca memory store --type decision --content "Used JWT with refresh tokens"

Next session, the agent remembers past decisions and patterns.
```

---

## Validation

```bash
# Check if OpenCode integration is configured
cosca editor validate opencode

# Expected output:
✅ OpenCode detected
✅ MCP server configured
✅ Custom instructions installed
```

---

## Teardown

```bash
# Remove Cosca integration from OpenCode
cosca editor teardown opencode
```

---

> **Related**: [Editor Overview](overview.md) | [Claude Code Integration](claude.md) | [VS Code Integration](vscode.md)
