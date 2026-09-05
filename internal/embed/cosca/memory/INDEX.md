# Cosca Kernel — Memory System

> **Version**: 3.0.1 | **Status**: active | **Last Updated**: 2026-07-29

## Purpose
The Kernel's memory system. Stores everything the Kernel needs to be intelligent, efficient, and context-aware about the Cosca project.

## Documentation
| Key | Description |
|-----|-------------|
| [MEMORY_SYSTEM](MEMORY_SYSTEM.md) | How the memory system works, evolves, and is maintained |

## Memory Categories
| Directory | Purpose | Files |
|-----------|---------|-------|
| [context/](context/INDEX.md) | Quick-load session context | 2 |
| [sessions/](sessions/active/INDEX.md) | Active session tracking | 2 |
| [codebase/](codebase/INDEX.md) | Project structure map | 4 |
| [project/](project/INDEX.md) | Project identity and status | 5 |
| [architecture/](architecture/INDEX.md) | System design knowledge | 19 |
| [decisions/](decisions/cosca-cli/INDEX.md) | Architecture Decision Records | 8 |
| [roadmap/](roadmap/INDEX.md) | Development tracking | 2 |
| [dependencies/](dependencies/INDEX.md) | Module dependency graph | 3 |
| [testing/](testing/INDEX.md) | Test strategy and coverage | 4 |
| [bug/](bug/INDEX.md) | Bug registry (active + archived) | 13 |
| [pattern/](pattern/INDEX.md) | Design patterns | 8 |
| [long/](long/INDEX.md) | Cross-session stack knowledge | 7 |
| [agent/](agent/INDEX.md) | Agent performance records | 10 |
| [evolution/](evolution/INDEX.md) | Kernel self-improvement log | 2 |
| [decision/](decision/INDEX.md) | Framework-level decisions | 4 |
| [session/](session/INDEX.md) | Bootstrap session history | 3 |
| [short/](short/INDEX.md) | Recent session notes | 5 |

## Load Order
1. `context/cognitive-state.md` — Single source of truth (~400 tokens, contains architecture, state, decisions, pending, risks)
2. `sessions/active/current.md` — Current session (only if needed)
3. On-demand: all other categories

## Health
- Files: 421 | Directories: 76 | INDEX.md: 72
- Agent directories: 54 | Engine directories: 34 | Workflow files: 28
- Embed files: 289 (synced via make embed-sync)
- Broken links: 0 | Orphans: 0
- Last verified: 2026-07-29
- Semantic Memory Engine deployed: 2026-07-29 (Don's order, Fase C)
