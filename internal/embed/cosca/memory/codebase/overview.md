# Cosca Codebase Overview

> Auto-generated navigation map. Referenced by all 55 agents as their primary codebase orientation.
> Last updated: 2026-08-09

## Entry Points

| File | Purpose |
|------|---------|
| [KERNEL.md](../../KERNEL.md) | Kernel entity definition — 24 responsibilities, state machine, capability resolver |
| [CONSTITUTION.md](../../CONSTITUTION.md) | 8 immutable principles (v1.1.0) |
| [AGENT_DNA.md](../../AGENT_DNA.md) | Agent identity + capability model (v3.0) |
| [COSCA_INDEX.md](../../COSCA_INDEX.md) | Master index — all agents/skills/engines/departments |
| [QUALITY_GATES.md](../../QUALITY_GATES.md) | Gate 0-4 quality enforcement |
| [SECURITY_ARCHITECTURE.md](../../SECURITY_ARCHITECTURE.md) | Security model — jail, JWT, sandbox |
| [RUNTIME_CONTRACT.md](../../RUNTIME_CONTRACT.md) | Runtime gRPC contract |
| [MEMORY_MODEL.md](../../MEMORY_MODEL.md) | Memory architecture — MAG, EmbedCache, vector search |
| [GOVERNANCE.md](../../GOVERNANCE.md) | Governance model — councils, auditing |
| [PROVIDER_INTERFACE.md](../../PROVIDER_INTERFACE.md) | LLM provider interface — 11 providers |

## Directory Map

```
internal/embed/cosca/
├── agents/          — 55 agent PROMPT.md + INDEX.md files
├── architecture/    — COGNITIVE_MATURITY.md, COGNITIVE_ECOSYSTEM.md
├── analytics/       — cognitive-entropy, evolution-score, cognitive-metrics
├── departments/     — 55 department SKILL.md files
├── engines/         — 40+ cognitive engines (semantic-memory, wisdom-decay, etc.)
├── knowledge/       — patterns/, best-practices/, laws.json
├── memory/          — agent learnings, failures, patterns, capability profiles
│   ├── agent/       — per-agent semantic memory (learnings.md, failures.md, patterns.md)
│   ├── codebase/    — this file
│   └── governance/  — audit reports
├── shared/          — AUTO_EVOLUTION_PROTOCOL.md, KNOWLEDGE_PROTOCOL.md, PROJECT_CONTEXT.md
├── skills/          — 71+ skill definitions
├── workflows/       — metacognition pipeline, cognitive-audit-loop
├── councils/        — council definitions
├── plugins/         — plugin contracts
├── runtime/         — runtime specifications
├── capabilities/    — capability definitions
├── bootstrap/       — bootstrap configuration
├── company/         — company/organization context
├── prompts/         — shared prompt templates
├── templates/       — code templates
└── metrics/         — metric definitions
```

## Agent Architecture

- **55 agents** organized in hierarchy: Kernel → CEO → CTO → Chiefs → Specialists
- Each agent has: `PROMPT.md` (identity + instructions), `INDEX.md` (capability index)
- Memory per agent: `memory/agent/{name}/learnings.md`, `failures.md`, `patterns.md`, `capability-profile.md`
- Chain of command: Don → Kernel → CEO → CTO → Chiefs → Specialists
- Delegation protocol: `cosca plan --target --type --agent` before any task

## Key Protocols

| Protocol | Path | Purpose |
|----------|------|---------|
| Auto-Evolution | [shared/AUTO_EVOLUTION_PROTOCOL.md](../../shared/AUTO_EVOLUTION_PROTOCOL.md) | Stages 7-8 mandatory, post-task checklist |
| Knowledge | [shared/KNOWLEDGE_PROTOCOL.md](../../shared/KNOWLEDGE_PROTOCOL.md) | Anti-hallucination: verify tools before delegating |
| Learning Entry | [memory/LEARNING_PROTOCOL.md](../LEARNING_PROTOCOL.md) | Learning entry format |
| Metacognition | [workflows/metacognition-pipeline.md](../../workflows/metacognition-pipeline.md) | 9-stage pipeline |

## Runtime Stack

| Component | Technology | Location |
|-----------|-----------|----------|
| Backend | Go 1.25 | `cmd/cosca/`, `internal/`, `pkg/` |
| Frontend | Next.js 15 | `web/` |
| Database | SQLite (WAL, FTS5, vector) | `.cosca/knowledge.db` |
| API | REST :14120, gRPC :14123 | `api/rest/`, `proto/` |
| CLI | Cobra (39 commands) | `cmd/cosca/` |
| Auth | JWT HS256, RBAC | `internal/auth/` |
| Sandbox | bubblewrap | `/usr/bin/bwrap` |
| LLM | 11 providers | `internal/providers/` |
| Embedding | Ollama (nomic-embed-text 768d) | `internal/providers/ollama/` |

## Build & Install

```bash
make install          # Build + install to ~/.cosca/bin/
make test             # Run all tests
cosca serve           # Start API server
cosca runtime start   # Start runtime daemon
cosca-chat exec "..." # Direct chat
```
