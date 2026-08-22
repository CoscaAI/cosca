# Claude Code Integration

> **Status**: active | **Owner**: CLI Chief | **Last Updated**: 2026-07-23

## Overview

Cosca integrates with **Claude Code** (Anthropic's CLI agent), providing knowledge search, memory management, and runtime capabilities directly within Claude Code sessions.

---

## Setup

```bash
# Auto-detect and setup (part of cosca install)
cosca install

# Manual setup
cosca editor setup claude
```

### What Gets Configured

1. **Custom Instructions** — `CLAUDE.md` created/updated in project root
2. **MCP Server** — Registered for knowledge search and memory tools
3. **Cosca Commands** — Available as shell commands in Claude Code

---

## Integration Details

### CLAUDE.md

Cosca creates or updates `CLAUDE.md` in the project root:

```markdown
# Cosca Integration

This project uses Cosca for knowledge management and persistent memory.

## Commands
- `cosca knowledge search <query>` — Search project knowledge base
- `cosca memory store --type <type> --content "<content>"` — Store memories
- `cosca memory search <query>` — Search memories
- `cosca status` — Check system health

## Knowledge Workflow
1. Before implementing features, search existing knowledge
2. After making decisions, store them as memory
3. Use `cosca knowledge search` to find relevant patterns and ADRs
```

### MCP Tools

When configured, the following MCP tools are available in Claude Code:

| Tool | Description |
|------|-------------|
| `cosca_search` | Full-text + vector search across project knowledge |
| `cosca_memory` | Access persistent memory store |
| `cosca_status` | Get runtime health and metrics |

---

## Features

### Knowledge Search in Claude Code

```bash
# Search for relevant patterns before implementing
> cosca knowledge search "authentication architecture"

# Search with type filter
> cosca knowledge search "database schema" --type document

# Search with graph context
> cosca knowledge search "UserService" --graph
```

### Memory Persistence

Memory stored in Cosca persists across Claude Code sessions:

```bash
# Store a decision
> cosca memory store \
    --type decision \
    --layer project \
    --content "Using PostgreSQL with connection pooling"

# Retrieve next session
> cosca memory search "database" --type decision
```

---

## Validation

```bash
cosca editor validate claude
```

---

> **Related**: [Editor Overview](overview.md) | [OpenCode Integration](opencode.md)
