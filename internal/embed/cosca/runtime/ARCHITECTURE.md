# Runtime Architecture

> **Extracted from**: KERNEL.md §1 | **Lines**: ~1,006 | **Date**: 2026-07-28

## Overview

The Cosca Runtime is a single Go binary that orchestrates AI agents, skills, workflows, and knowledge stores. It operates as a state machine with explicit lifecycle phases.

## Core Components

```
┌─────────────────────────────────────────────────────────┐
│                     THE DON (User)                      │
│                         │                               │
│                  ┌──────┴──────┐                        │
│                  │   KERNEL    │  Consigliere           │
│                  │  (Primary)  │  Orchestrator          │
│                  └──────┬──────┘                        │
│                         │                               │
│              ┌──────────┼──────────┐                    │
│              ▼          ▼          ▼                    │
│         ┌────────┐ ┌────────┐ ┌────────┐                │
│         │  CEO   │ │  CTO   │ │Product │  Strategic     │
│         │ Chief  │ │ Chief  │ │ Chief  │  Command       │
│         └───┬────┘ └───┬────┘ └────────┘                │
│             │          │                                │
│      ┌──────┘    ┌─────┴──────────────────┐             │
│      ▼           ▼                        ▼             │
│  ┌────────┐ ┌─────────┐ ┌──────────┐ ┌──────────┐      │
│  │Backend │ │Frontend │ │  DevOps  │ │Security  │      │
│  │ Chief  │ │ Chief   │ │  Chief   │ │  Chief   │      │
│  └────────┘ └─────────┘ └──────────┘ └──────────┘      │
│                                                         │
│              ┌──────────────────────────────┐           │
│              │      SPECIALISTS (55)        │           │
│              │   Workers / Individual       │           │
│              │      Contributors            │           │
│              └──────────────────────────────┘           │
└─────────────────────────────────────────────────────────┘
```

## Runtime Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| **Development** | Full logging, hot reload, debug tools | Local development |
| **Production** | Optimized, minimal logging, health checks | Deployed environments |
| **Test** | Mocked providers, deterministic output | CI/CD pipelines |

## Binary Structure

```
cosca.exe (98MB)
├── cmd/          → CLI entry points
├── api/          → REST, gRPC, SSE/WebSocket
├── internal/     → Core packages (109 packages)
├── pkg/          → Public SDK
├── proto/        → Protobuf definitions
└── internal/embed/ → Embedded assets (knowledge, agents, workflows)
```

## Execution Flow

1. **Bootstrap** → Discover workspace, load config
2. **Initialize** → Start services, connect to DB
3. **Load Context** → Read embeds, populate knowledge.db
4. **Serve** → Accept API requests, route to agents
5. **Execute** → Run workflows, manage state
6. **Observe** → Metrics, logs, traces
7. **Recover** → Retry, checkpoint, rollback
8. **Evolve** → Learn from sessions, update knowledge

## Design Principles

- **Single Binary**: Everything compiled into one executable
- **Zero External Dependencies**: SQLite embedded, no Redis/Postgres required
- **Fail-Closed**: Default to deny, require explicit permission
- **Event-Driven**: Every behavior published as event
- **Capability-First**: Resolve capabilities, not departments
