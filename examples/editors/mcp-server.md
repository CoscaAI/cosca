# Example: MCP Server Setup

> **Status**: active | **Owner**: CLI Chief | **Last Updated**: 2026-07-23

This example demonstrates setting up the Cosca MCP (Model Context Protocol) server and connecting from different AI-powered editors.

---

## Overview

The MCP server enables Cosca to provide knowledge search, memory access, and runtime commands directly within your editor. It exposes Cosca capabilities as **tools** (callable functions) and **resources** (data endpoints) that the editor's AI agent can use.

### Supported MCP Transports

| Transport | Mode | When to Use |
|-----------|------|-------------|
| **STDIO** | CLI-integrated | Default for `cosca editor setup` |
| **HTTP/SSE** | Daemon mode | When runtime is running as a background daemon |

---

## Automatic Setup (Recommended)

The easiest way to configure MCP is through the editor integration:

```bash
# Full automatic setup (detects editor, configures MCP)
cosca install

# Or configure MCP only for existing Cosca installations
cosca editor setup

# For OpenCode specifically
cosca editor setup opencode

# For Claude Code
cosca editor setup claude

# For VS Code
cosca editor setup vscode

# For any MCP-compatible editor (generic)
cosca editor setup generic-mcp --mcp-only
```

---

## Manual Configuration by Editor

### OpenCode

Add the MCP server to `.opencode/AGENTS.md`:

```markdown
# Cosca Integration

## MCP Servers

Use the following MCP server to search project knowledge and manage memory:

```json
{
  "mcpServers": {
    "cosca": {
      "command": "cosca",
      "args": ["mcp", "--stdio"],
      "env": {
        "COSCA_HOME": "${workspaceFolder}/.cosca"
      }
    }
  }
}
```

The MCP server provides these tools:

- `cosca_knowledge_search` — Search project knowledge base
- `cosca_knowledge_index` — Index files for knowledge search
- `cosca_memory_store` — Store context in memory
- `cosca_memory_search` — Search stored memory
- `cosca_runtime_status` — Check runtime health
- `cosca_plugin_list` — List installed plugins

Use these tools to help me understand the project context and find relevant information.
```

### Claude Code

Add to `CLAUDE.md`:

```markdown
# Cosca Integration

## MCP Server Configuration

Cosca provides knowledge and memory capabilities through MCP.

```json
{
  "mcpServers": {
    "cosca": {
      "command": "cosca",
      "args": ["mcp", "--stdio"],
      "env": {
        "COSCA_HOME": "${workspaceFolder}/.cosca"
      }
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `cosca_knowledge_search` | Search project documentation and code |
| `cosca_memory_store` | Store important context for future sessions |
| `cosca_memory_search` | Retrieve stored context |
| `cosca_runtime_status` | Check Cosca runtime health |
```

### VS Code

Add to `.vscode/settings.json`:

```json
{
  "mcp": {
    "servers": {
      "cosca": {
        "command": "cosca",
        "args": ["mcp", "--stdio"],
        "env": {
          "COSCA_HOME": "${workspaceFolder}/.cosca"
        }
      }
    }
  }
}
```

### Cursor

Add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "cosca": {
      "command": "cosca",
      "args": ["mcp", "--stdio"],
      "env": {
        "COSCA_HOME": "${workspaceFolder}/.cosca"
      }
    }
  }
}
```

### Zed

Add to `.zed/settings.json`:

```json
{
  "mcp_servers": {
    "cosca": {
      "command": "cosca",
      "args": ["mcp", "--stdio"],
      "env": {
        "COSCA_HOME": "${workspaceFolder}/.cosca"
      }
    }
  }
}
```

### Windsurf

Add to `.windsurf/mcp_config.json`:

```json
{
  "mcpServers": {
    "cosca": {
      "command": "cosca",
      "args": ["mcp", "--stdio"],
      "env": {
        "COSCA_HOME": "${workspaceFolder}/.cosca"
      }
    }
  }
}
```

### Neovim

Add to your Neovim configuration:

```lua
-- init.lua
vim.g.mcp_servers = {
  ["cosca"] = {
    command = "cosca",
    args = { "mcp", "--stdio" },
    env = {
      COSCA_HOME = vim.fn.getcwd() .. "/.cosca",
    },
  },
}
```

---

## Daemon Mode Configuration

For HTTP/SSE transport (daemon mode):

### Start the Daemon

```bash
# Start runtime in daemon mode
cosca runtime start

# Verify the MCP server is running
curl http://127.0.0.1:9123/mcp/health
```

### Configure for HTTP/SSE

**OpenCode:**
```json
{
  "mcpServers": {
    "cosca": {
      "url": "http://127.0.0.1:9123/mcp",
      "type": "sse"
    }
  }
}
```

**VS Code:**
```json
{
  "mcp": {
    "servers": {
      "cosca": {
        "type": "sse",
        "url": "http://127.0.0.1:9123/mcp"
      }
    }
  }
}
```

---

## Available MCP Tools

### Tool: `cosca_knowledge_search`

Search the project knowledge base.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `query` | string | Yes | — | Search query |
| `type` | string | No | `all` | Filter by type: `document`, `chunk`, `entity`, `code_block` |
| `path` | string | No | — | Filter by file path prefix |
| `limit` | number | No | `10` | Maximum results to return |

**Example:**

```json
{
  "name": "cosca_knowledge_search",
  "arguments": {
    "query": "authentication flow",
    "type": "document",
    "limit": 5
  }
}
```

**Response:**

```json
{
  "content": [
    {
      "type": "text",
      "text": "Found 3 results:\n\n1. Authentication Flow (score: 0.95)\n   docs/auth/overview.md\n   The authentication flow uses OAuth 2.0 with JWT tokens...\n\n2. API Authentication (score: 0.87)\n   docs/api/authentication.md\n   All API endpoints require a valid JWT in the Authorization header..."
    }
  ]
}
```

### Tool: `cosca_knowledge_index`

Index a file or directory for knowledge search.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path` | string | Yes | — | File or directory path to index |

### Tool: `cosca_memory_store`

Store important context in memory for future sessions.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `key` | string | Yes | — | Unique memory identifier |
| `content` | string | Yes | — | Content to store |
| `type` | string | No | `short` | Memory type: `short`, `long`, `project` |
| `tags` | string[] | No | `[]` | Tags for categorizing |

**Example:**

```json
{
  "name": "cosca_memory_store",
  "arguments": {
    "key": "architecture-decision-20260723",
    "content": "We decided to use PostgreSQL for the user database because of complex query requirements.",
    "type": "decision",
    "tags": ["database", "architecture", "decision"]
  }
}
```

### Tool: `cosca_memory_search`

Search stored memory records.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `query` | string | Yes | — | Search query |
| `type` | string | No | `all` | Filter by memory type |

### Tool: `cosca_runtime_status`

Check the current runtime status.

**Parameters:** None

**Response:**

```json
{
  "content": [
    {
      "type": "text",
      "text": "Runtime Status: running (healthy)\nUptime: 2h 15m\nVersion: 0.1.0-dev\nComponents:\n  knowledge: healthy\n  discovery: healthy\n  memory: healthy\n  cache: healthy\n  plugins: healthy"
    }
  ]
}
```

### Tool: `cosca_plugin_list`

List all installed plugins.

**Parameters:** None

---

## Available MCP Resources

| Resource URI | Description | Example Response |
|-------------|-------------|-----------------|
| `cosca://knowledge/stats` | Knowledge engine statistics | `{"total_docs": 1250, "total_chunks": 4520, ...}` |
| `cosca://memory/stats` | Memory engine statistics | `{"total_records": 47, "types": {"short": 12, "long": 25, ...}}` |
| `cosca://runtime/status` | Current runtime status | `{"state": "running", "health": "healthy", ...}` |

---

## Verification

After configuring the MCP server, verify the integration:

```bash
# List MCP tools
cosca mcp list-tools
```

**Expected output:**

```
Available MCP Tools:
  cosca_knowledge_search  — Search the project knowledge base
  cosca_knowledge_index   — Index files for knowledge search
  cosca_memory_store      — Store context in memory
  cosca_memory_search     — Search stored memory
  cosca_runtime_status    — Check runtime health
  cosca_plugin_list       — List installed plugins
```

```bash
# Test the MCP server directly
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | cosca mcp --stdio
```

**Expected output:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [
      {
        "name": "cosca_knowledge_search",
        "description": "Search the project knowledge base",
        "inputSchema": {
          "type": "object",
          "properties": {
            "query": { "type": "string", "description": "Search query" },
            "type": { "type": "string", "description": "Filter by type" },
            "limit": { "type": "number", "description": "Max results" }
          },
          "required": ["query"]
        }
      }
    ]
  }
}
```

---

## Troubleshooting

### MCP Tool Not Found

**Issue:** Editor reports `Tool not found: cosca_knowledge_search`

**Solutions:**
```bash
# Check if runtime is running
cosca runtime status

# Verify MCP server is responding
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | cosca mcp --stdio

# Restart runtime
cosca runtime restart
```

### Connection Refused

**Issue:** `MCP server connection failed: connection refused`

**Solutions:**
```bash
# For HTTP/SSE mode, ensure daemon is running
cosca runtime start

# Check the configured port
cosca config show | grep mcp_port

# Try STDIO mode instead (no daemon needed)
# Update your editor config to use stdio transport
```

### No Results from Search

**Issue:** `cosca_knowledge_search` returns no results

**Solutions:**
```bash
# Index the project first
cosca knowledge index .

# Check knowledge base stats
cosca knowledge stats

# Verify files are indexed
cosca knowledge search "test" --limit 5
```

---

**Related**: [Editor Overview](../../docs/editors/overview.md) | [Editor Integration](../../docs/editors/overview.md) | [API Reference](../../docs/api-reference/overview.md) | [ADR-004](../../docs/adr/ADR-004-editor-adapters.md)
