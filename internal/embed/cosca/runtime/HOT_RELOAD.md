# Hot Reload

> **Extracted from**: KERNEL.md section 14 | **Lines**: ~631 | **Date**: 2026-07-28

## Overview

Hot Reload enables live-updating Markdown files without restarting the Runtime. Changes to embeds, agents, skills, and workflows are detected and applied automatically.

## Reload Sources

| Source | Detection | Reload Method |
|--------|-----------|---------------|
| internal/embed/cosca/ | File watcher | Parse + index |
| .cosca/framework/ | File watcher | Re-sync from embed |
| .cosca/fallback/ | Polling (30s) | Re-index |
| knowledge.db | Direct edit | Validate + cache |

## Reload Process

```
File Changed
    │
    ▼
┌─────────────┐
│  Validate   │  Check format, schema
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Diff       │  Compare with current
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Apply      │  Update in-memory state
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Notify     │  Event: FileReloaded
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Cache      │  Invalidate affected cache
└─────────────┘
```

## What Can Be Hot-Reloaded

| Type | Reloadable | Requires Restart |
|------|------------|------------------|
| Agent prompts | Yes | No |
| Skill definitions | Yes | No |
| Workflow configs | Yes | No |
| Knowledge docs | Yes | No |
| Go code changes | No | Yes |
| Config (config.yaml) | No | Yes |

## Safety

- Invalid files are rejected with error log
- Previous version kept in memory until new version validated
- Maximum 100 reloads per hour (rate limit)
