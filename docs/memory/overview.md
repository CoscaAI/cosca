# Memory Engine

> **Status**: active | **Owner**: Memory Chief | **Last Updated**: 2026-07-23

## Overview

The Memory Engine provides multi-layer persistent memory for the Cosca platform. It enables storing, retrieving, searching, and promoting knowledge across sessions, projects, and workspaces.

```
┌──────────────────────────────────────────────────────────────────┐
│                      MEMORY ENGINE                                │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                    MEMORY LAYERS                          │    │
│  │                                                           │    │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │    │
│  │  │  GLOBAL  │  │WORKSPACE │  │ PROJECT  │  │SESSION  │ │    │
│  │  │ Permanent│  │Permanent │  │Persistent│  │Ephemeral│ │    │
│  │  └──────────┘  └──────────┘  └──────────┘  └─────────┘ │    │
│  │  ┌──────────┐                                            │    │
│  │  │   TEMP   │  ← Short TTL, auto-pruned                  │    │
│  │  └──────────┘                                            │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                    MEMORY TYPES                            │    │
│  │                                                           │    │
│  │  Decision | Pattern | Bug | Agent | Project              │    │
│  │  Architecture | Session                                   │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                    STORAGE BACKENDS                        │    │
│  │                                                           │    │
│  │  ┌──────────────────────┐  ┌──────────────────────────┐  │    │
│  │  │  FileStore           │  │  SQLiteIndex (FTS5)      │  │    │
│  │  │  • Markdown files    │  │  • Full-text search      │  │    │
│  │  │  • YAML frontmatter  │  │  • Type/layer filtering  │  │    │
│  │  │  • Human-readable    │  │  • Fast lookup           │  │    │
│  │  └──────────────────────┘  └──────────────────────────┘  │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                    FEATURES                                │    │
│  │                                                           │    │
│  │  • TTL-based auto pruning  • Layer promotion              │    │
│  │  • Priority sorting        • Snapshot on promote          │    │
│  │  • Multi-criteria search   • Layer statistics             │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

---

## Memory Layers

| Layer | Scope | Persistence | TTL | Typical Use |
|-------|-------|-------------|-----|-------------|
| **Global** | All workspaces | Permanent | — | Cross-project patterns, agent preferences |
| **Workspace** | Current workspace | Permanent | — | Workspace conventions, shared decisions |
| **Project** | Current project | Persistent | Configurable | Features, modules, architecture |
| **Session** | Current session | Ephemeral | Session end | Active context, current decisions |
| **Temp** | Temporary | Ephemeral | Short (default 24h) | Scratch data, temporary notes |

### Layer Priority

When searching across layers, results are ordered by layer priority:

| Layer | Priority |
|-------|----------|
| Session | 100 (highest) |
| Project | 75 |
| Workspace | 50 |
| Global | 25 |
| Temp | 10 (lowest) |

---

## Memory Types

| Type | Content | Example |
|------|---------|---------|
| **Decision** | Architecture decisions and rationale | "ADR-005: Use SQLite as primary store" |
| **Pattern** | Working patterns and anti-patterns | "Repository pattern works well for this domain" |
| **Bug** | Bug reports with root cause and fix | "Bug #42: Null pointer in user service" |
| **Agent** | Agent performance and preferences | "Backend Specialist preferred for Go tasks" |
| **Project** | Features, modules, releases | "Feature: User authentication completed in v2.1" |
| **Architecture** | ADRs, patterns, contracts | "System uses layered architecture with Go core" |
| **Session** | Active context, current decisions | "Current task: implementing search filters" |

---

## Storage Format

Each memory record is stored as a Markdown file with YAML frontmatter:

```markdown
---
id: "abc123-def456"
type: "decision"
layer: "project"
scope: "backend"
created_at: "2026-07-23T10:00:00Z"
updated_at: "2026-07-23T10:00:00Z"
ttl: "720h"
priority: 5
version: 1
metadata:
  author: "developer"
  tags: "architecture,backend"
---

# Decision: Use SQLite for Local Storage

We decided to use SQLite as the primary local storage engine because:

- Zero configuration required
- Portable (single file)
- Supports FTS5 for full-text search
- Supports vector extension for embeddings
- Well-tested in Go ecosystem
```

This format is:
- **Human-readable** — Can be viewed and edited directly
- **Git-friendly** — Tracks changes via version control
- **Searchable** — SQLite FTS index enables fast full-text search
- **Self-describing** — YAML frontmatter contains all metadata

---

## Snapshot System

Snapshots capture the state of memory at a point in time:

### Creating Snapshots

```bash
# Manual snapshot
cosca memory snapshot "pre-refactor-backup"

# Automatic snapshots (on promote)
cosca memory promote <id> session project
# → Auto-creates snapshot: "promote-<id>-session-project"
```

### Snapshot Features

| Feature | Description |
|---------|-------------|
| Creation | Manual or automatic on promote |
| Storage | Snapshot metadata in SQLite |
| Export | Graph state serialized as JSON |
| Restore | Via database backup |

---

## CLI Commands

```bash
# Store a memory record
cosca memory store --type decision --layer project --content "..."

# Store with metadata
cosca memory store \
    --type pattern \
    --layer project \
    --scope frontend \
    --priority 8 \
    --metadata "author=developer,tags=react,testing" \
    --content "..."

# Search memory
cosca memory search "authentication flow"

# Search with filters
cosca memory search "authentication" --type decision --layer project

# Retrieve by ID
cosca memory get <id>

# Delete a record
cosca memory delete <id>

# Promote a record to a higher layer
cosca memory promote <id> session project

# Prune expired records
cosca memory prune

# Show layer statistics
cosca memory stats

# Create a snapshot
cosca memory snapshot "backup-name"
```

---

## Best Practices

### Layer Strategy

1. **Use Session layer** for current task context (auto-cleaned)
2. **Promote to Project** for decisions that should persist across sessions
3. **Promote to Global** for patterns that apply across all projects
4. **Use Temp layer** for scratch data (auto-pruned)

### Memory Hygiene

- Set appropriate TTLs — don't keep temporary data forever
- Use metadata tags for better filtering
- Set priority levels to control search ranking
- Regularly prune expired records
- Use snapshots before significant changes

### Search Tips

```bash
# Find all decisions about authentication
cosca memory search "authentication" --type decision

# Find project-level patterns
cosca memory search "" --type pattern --layer project

# Find recent session context
cosca memory search "" --layer session --limit 10
```

---

## Architecture

```go
// Core interfaces
type Store interface {
    Save(ctx context.Context, record MemoryRecord) (*MemoryRecord, error)
    Get(ctx context.Context, id string) (*MemoryRecord, error)
    Delete(ctx context.Context, id string) error
    Search(ctx context.Context, query string, opts SearchOptions) ([]MemoryRecord, error)
    Index(ctx context.Context, record MemoryRecord) error
    Prune(ctx context.Context) (int, error)
    Stats(ctx context.Context) (LayerStats, error)
    Close() error
}

// Memory engine
type MemoryEngine struct {
    stores map[MemoryLayer]Store
    layers *LayerManager
    config EngineConfig
}

// Record structure
type MemoryRecord struct {
    ID        string
    Type      MemoryType      // decision, pattern, bug, etc.
    Layer     MemoryLayer     // global, workspace, project, session, temp
    Scope     string          // optional scope identifier
    Content   string          // markdown content
    Metadata  map[string]string
    CreatedAt time.Time
    UpdatedAt time.Time
    TTL       time.Duration
    Priority  int
    Version   int
}
```

---

> **Related**: [Knowledge Engine](../knowledge/overview.md) | [Runtime Configuration](../runtime/configuration.md)
