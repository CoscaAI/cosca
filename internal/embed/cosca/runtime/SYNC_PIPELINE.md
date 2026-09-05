# Runtime Synchronization Pipeline

> **Extracted from**: KERNEL.md section 13 | **Lines**: ~668 | **Date**: 2026-07-28

## Overview

The Sync Pipeline keeps all data stores consistent: embed, knowledge.db, memory, cache, and runtime state.

## Sync Stages

```
Embed (.md files)
    │
    ▼
┌─────────────┐
│  Parse      │  Extract metadata, validate format
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Index      │  FTS5 + vector embeddings
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Store      │  Write to knowledge.db
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Cache      │  In-memory cache invalidation
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Notify     │  Event Bus notification
└─────────────┘
```

## Sync Modes

| Mode | Trigger | Scope |
|------|---------|-------|
| Full | `cosca init` | All embeds |
| Incremental | File change | Changed files only |
| On-demand | `cosca sync` | User-specified |

## Conflict Resolution

When embed and DB diverge:
1. Embed is source of truth for definitions
2. DB is source of truth for runtime state
3. Conflicts logged, latest timestamp wins

## Consistency Guarantees

- **Eventual**: All stores converge within 30 seconds
- **Strong**:knowledge.db writes are atomic (WAL mode)
- **Best-effory**: Cache may be stale for up to 5 seconds
