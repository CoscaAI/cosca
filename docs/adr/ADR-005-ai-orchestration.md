# ADR-005: AI Orchestration Engine

> **Status:** Active | **Owner:** AI Chief | **Last Updated:** 2026-07-24

## Context

The Cosca platform has all the building blocks for AI-powered development assistance:
- Knowledge Engine (search, indexing)
- Memory Engine (persistent context)
- Agent system (specialized assistants)
- Skills system (capabilities)
- Workflow system (processes)
- LLM Providers (GPT, Claude, etc.)

But there is no **orchestrator** that connects them into a cohesive experience. Users must manually invoke each subsystem. We need an engine that:
1. Understands the user's intent from natural language
2. Routes to the right agent
3. Chains skills in the right order
4. Augments with relevant knowledge and memory
5. Produces structured, actionable results

## Decision

We adopt a **pipeline-based orchestration architecture** with four core stages: Context → Router → Execute → Memorize.

### Architecture

```
Request → [Context Builder] → [Router] → [Executor] → [Memory] → Response
                │                  │            │            │
                ▼                  ▼            ▼            ▼
          Knowledge Engine    Agent Mgmt    LLM Prov.   Memory Engine
          Memory Engine       Skill Mgmt    Skill Exec.
```

### Core Interfaces

```go
// Orchestrator is the main entry point.
type Orchestrator interface {
    Execute(ctx context.Context, req Request) (*Result, error)
}

// Request represents a user request.
type Request struct {
    Prompt  string
    Context map[string]interface{}
}

// Result represents the orchestration result.
type Result struct {
    Response    string
    Agent       string
    SkillsUsed  []string
    MemoryID    string
    Duration    time.Duration
}

// Router selects the best agent for a request.
type Router interface {
    Route(ctx context.Context, req Request) (*Agent, error)
}

// Pipeline executes a sequence of skills.
type Pipeline interface {
    Execute(ctx context.Context, skills []Skill, input interface{}) (interface{}, error)
}
```

### Pipeline Steps

1. **Context Building** — Searches relevant knowledge + memory
2. **Routing** — Selects agent based on semantic analysis
3. **Execution** — Runs agent with augmented context
4. **Skill Pipeline** — Chains skills in sequence
5. **Memory Storage** — Saves result as memory

## Rationale

### Why Pipeline Architecture?

| Approach | Pros | Cons |
|----------|------|------|
| Pipeline (chosen) | Predictable, testable, extensible | More verbose |
| Event-driven | Flexible, loose | Hard to debug |
| Monolithic | Simple | Not scalable |

### Why Four Stages?

Each stage maps to an existing Cosca subsystem:
1. **Context** → Knowledge + Memory Engines (already built)
2. **Router** → Agent system (already built)
3. **Executor** → Providers + Skills (already built)
4. **Memory** → Memory Engine (already built)

No new infrastructure needed — only the orchestration logic.

## Consequences

### Positive
- Reuses all existing subsystems
- Clear separation of concerns
- Each stage independently testable
- Easy to add new stages (e.g., Validation, Review)

### Negative
- Pipeline overhead for simple requests
- Requires all subsystems to be healthy

## Implementation Plan

### Phase 1 (core)
- Types, interfaces, Orchestrator struct
- Context Builder (knowledge + memory search)
- Router (agent selection by keywords + embeddings)

### Phase 2 (execution)
- Agent Runner (LLM call with context)
- Pipeline executor (skill chaining)

### Phase 3 (memory)
- MAG (auto-store results)
- CLI commands (`cosca run`)

