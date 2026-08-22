# Cosca Kernel — Memory Model

> **Version**: 4.0.0 | **Status**: active | **Last Updated**: 2026-07-28

## Purpose
This document explains how the Kernel's memory system works, how it evolves, and how to use it effectively. It is the meta-documentation of the memory system itself.

---

## Architecture

```
.opencode/cosca/memory/
├── MEMORY_SYSTEM.md        ← This file (self-documentation)
│
├── context/               ← Quick-load session context (loaded FIRST)
│   └── session.md         ← Stack, versions, state, active files
│
├── sessions/active/       ← Hot-reload current session
│   └── current.md         ← What we're doing right now
│
├── codebase/              ← Project structure map (navigation)
│   ├── overview.md        ← Full directory tree
│   ├── go-packages.md     ← All 42 internal packages
│   └── web-frontend.md    ← Next.js 15 structure
│
├── project/               ← Project identity and status
│   ├── cosca-overview.md← Complete project reference
│   └── current-projects.md← Active project: Cosca
│
├── architecture/          ← System design knowledge
│   ├── system-architecture-overview.md  ← 5-layer architecture
│   └── event-architecture.md           ← Internal event bus
│
├── decisions/cosca/     ← Architecture Decision Records
│   └── adr-001.md ... adr-007.md
│
├── roadmap/               ← Development tracking
│   └── milestones.md      ← Epic status, priorities
│
├── dependencies/          ← Module dependency graph
│   ├── go-modules.md      ← Internal import graph
│   └── external-deps.md   ← go.mod dependencies
│
├── testing/               ← Test strategy and coverage
│   ├── strategy.md        ← Test pyramid, commands
│   ├── coverage.md        ← Coverage per package
│   └── patterns.md        ← Go test patterns
│
├── bug/                   ← Bug registry (active + archived)
│   ├── bug-001-*.md       ← Real Cosca bugs (active)
│   └── bug-001-*.md       ← Archived generic bugs
│
├── pattern/               ← Design patterns (active + reference)
│   ├── go-provider-pattern.md
│   ├── go-editor-adapter-pattern.md
│   └── go-plugin-wasm-pattern.md
│
├── long/                  ← Cross-session stack knowledge
│   ├── go-sqlite-stack.md
│   └── nextjs-frontend-stack.md
│
├── agent/                 ← Agent self-evolving memory (DNA v3.0)
│   ├── {agent-name}/
│   │   ├── learnings.md           ← Semantic learning journal
│   │   ├── failures.md            ← Negative memory (NEW v3.0)
│   │   ├── patterns.md            ← Reusable solution patterns
│   │   ├── evolution.md           ← Capability timeline + confidence
│   │   ├── capability-profile.md  ← Self-model with per-domain scores (NEW v3.0)
│   │   └── INDEX.md               ← Fast cross-reference
│   └── ... (51 agent directories)
│
├── evolution/             ← Kernel self-improvement log
│   └── learnings.md       ← Cumulative lessons learned
│
├── decision/              ← Framework-level decisions (Cosca evolution)
├── session/               ← Bootstrap session history
└── short/                 ← Recent session notes
```

---

## Load Order (Session Startup)

1. `context/session.md` — Essential context (<1s)
2. `sessions/active/current.md` — What we're doing
3. `codebase/overview.md` — Where files are
4. `project/cosca-overview.md` — What we're building
5. On-demand: specific files as needed

---

## Memory Types

| Type | Directory | Scope | Retention |
|------|-----------|-------|-----------|
| **Context** | context/ | Session-level | Reloaded each session |
| **Active** | sessions/active/ | Current session | Hot-reload |
| **Codebase** | codebase/ | Cross-session | Updated on major changes |
| **Project** | project/ | Project lifespan | Versioned |
| **Architecture** | architecture/ | Cross-session | Updated on design changes |
| **Decision** | decisions/, decision/ | Permanent | Immutable after acceptance |
| **Roadmap** | roadmap/ | Quarterly | Updated each sprint |
| **Dependency** | dependencies/ | Cross-session | Updated on go.mod changes |
| **Testing** | testing/ | Cross-session | Updated on coverage changes |
| **Bug** | bug/ | Permanent | Active or archived |
| **Pattern** | pattern/ | Permanent | Accreting |
| **Knowledge** | long/ | Cross-session | Updated on stack changes |
| **Agent** | agent/ | Rolling 12 months | Updated each session |
| **Evolution** | evolution/ | Permanent | Accreting (never deleted) |
| **Session** | session/, short/ | Rolling 30 days | Rotates |

---

## Evolution Principles

### How Memory Grows
1. **Discover** — Identify what knowledge the Kernel needs but doesn't have
2. **Propose** — Present options to the chef for approval
3. **Create** — Write memory files (markdown only, never code)
4. **Audit** — Validate links, consistency, no orphans
5. **Commit** — Version alongside the project

### Quality Rules
- Every directory has an INDEX.md
- Every INDEX.md link points to an existing file
- No orphaned files (all files referenced from an INDEX)
- Active files contain project-specific knowledge (not generic templates)
- Archived files are clearly marked with `status: archived` in frontmatter
- Files use YAML frontmatter + markdown body

### Anti-Patterns
- ❌ Memory copied from templates without validation
- ❌ Active files with knowledge about other projects
- ❌ INDEX.md with broken links
- ❌ Orphaned files with no navigation path
- ❌ Contradictory information across files

---

## Current State
- **Files**: 82+ (growing with Option C)
- **Directories**: 12 (growing with Option C)
- **INDEX.md files**: 12 (growing with Option C)
- **Coverage**: All Cosca subsystems documented
- **Health**: 0 broken links, 0 orphans, 0 stale content

---

## Related
- [AGENTS.md](../../AGENTS.md) — Discovery policy
- [KERNEL.md](../../KERNEL.md) — Kernel instructions
- [CONVENTIONS.md](../../CONVENTIONS.md) — Framework conventions
