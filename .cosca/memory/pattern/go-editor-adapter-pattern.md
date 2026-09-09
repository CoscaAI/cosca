---
type: pattern
key: go-editor-adapter-pattern
tags: [go, editor, adapter, mcp, cosca]
timestamp: 2026-07-26T00:00:00Z
status: active
agent: Architecture Chief
category: design
confidence: 0.95
times_used: 9
---

# Pattern: Go Editor Adapter

## Context
Cosca must integrate with 9 different AI code editors (OpenCode, Claude Code, VS Code, Cursor, Neovim, Zed, Windsurf, Codex, MCP Generic) — each with different context injection mechanisms.

## Solution
Define an `Editor` interface with context injection:

```go
// internal/editors/editor.go
type Editor interface {
    Name() string
    InjectContext(ctx context.Context, data ContextData) error
    RemoveContext(ctx context.Context) error
    IsActive(ctx context.Context) bool
}

type ContextData struct {
    Knowledge []KnowledgeResult
    Memory    []MemoryRecord
    Skills    []SkillDefinition
    Workflows []WorkflowDefinition
    Project   ProjectContext
}
```

## Structure
```
internal/editors/
├── editor.go           ← Interface + manager
├── editor_test.go      ← Interface conformance tests
├── types/              ← Shared types
├── opencode/           ← OpenCode adapter
├── claude/             ← Claude Code adapter
├── vscode/             ← VS Code adapter
├── cursor/             ← Cursor adapter
├── neovim/             ← Neovim adapter
├── zed/                ← Zed adapter
├── windsurf/           ← Windsurf adapter
├── codex/              ← OpenAI Codex adapter
└── generic_mcp/        ← Generic MCP adapter
```

## Key Pattern: MCP Transport
All editors ultimately use MCP (Model Context Protocol) for context injection:
1. Editor adapter writes context files to editor-specific location
2. Editor reads context on next AI interaction
3. MCP server provides real-time context via protocol

## Benefits
- Add new editor without touching existing code
- Each adapter handles editor-specific file formats
- MCP server provides real-time alternative
- Editor auto-detection on `cosca install`

## Application in Cosca
All 9 editors implement this interface. The `cosca context build` command generates context for the active editor. The runtime daemon watches for editor changes.
