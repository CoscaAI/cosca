# AI Orchestration Engine

> **Status:** implemented | **Owner:** AI Chief | **Last Updated:** 2026-07-24
> **Dependencies:** Knowledge Engine ✅, Memory Engine ✅, Plugin System ✅, Agents ✅, Skills ✅, Workflows ✅, Providers ✅
> **Architecture:** [ADR-006](../adr/ADR-006-ai-orchestration-implementation.md)

## Overview

The AI Orchestration Engine is the brain of the Cosca platform. It connects agents, skills, memory, knowledge, and AI providers into an intelligent pipeline that executes complex software development tasks.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                     AI ORCHESTRATION ENGINE                        │
│                                                                   │
│  User Request                                                      │
│       │                                                           │
│       ▼                                                           │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────────┐  │
│  │   CONTEXT    │────▶│   ROUTER     │────▶│   EXECUTOR       │  │
│  │   BUILDER    │     │   (who?)     │     │   (how?)         │  │
│  └──────┬───────┘     └──────┬───────┘     └────────┬─────────┘  │
│         │                    │                       │            │
│         ▼                    ▼                       ▼            │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────────┐  │
│  │  KNOWLEDGE   │     │    AGENT     │     │    SKILL         │  │
│  │  ENGINE      │     │   SELECTION  │     │    PIPELINE      │  │
│  └──────────────┘     └──────────────┘     └──────────────────┘  │
│         │                    │                       │            │
│         ▼                    ▼                       ▼            │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────────┐  │
│  │   MEMORY     │     │   PROVIDER   │     │   WORKFLOW       │  │
│  │   ENGINE     │     │   (LLM)      │     │   EXECUTOR       │  │
│  └──────────────┘     └──────────────┘     └──────────────────┘  │
│                                                                   │
│  Result → Memory Store → Response                                 │
└──────────────────────────────────────────────────────────────────┘
```

## Components

### 1. Context Builder
- Fetches relevant context from the Knowledge Engine
- Injects recent memory from the Memory Engine
- Builds prompt with context for the LLM

### 2. Router
- Analyzes the user request
- Selects the most appropriate agent (Backend Chief, Frontend Chief, etc.)
- Fallback to generic agent when no match is found

### 3. Executor (Agent Runner)
- Runs the selected agent
- Manages the cycle: Prompt → LLM → Parse → Action
- Supports sequential and parallel skill execution

### 4. Skill Pipeline
- Chains skills in sequence
- Output of one skill becomes input of the next
- Pipeline steps: transform, filter, enrich, validate

### 5. Memory-Augmented Generation (MAG)
- Before execution: retrieves relevant memories
- During execution: saves partial decisions
- After execution: stores result as memory

## Complete Flow

```
1. User: "implement authentication flow"
2. Context Builder → searches Knowledge Engine + Memory Engine
3. Router → selects Backend Chief
4. Agent Runner → Backend Chief receives context
5. Skill Pipeline:
   a. skill:analyze-requirements
   b. skill:design-architecture
   c. skill:generate-code
   d. skill:review-code
6. Memory Engine → saves decisions and results
7. Formatted response → user
```

## Directory

```
internal/
├── chat/                         # LLM Chat Provider Interface
│   ├── types.go                  # ChatProvider, Message, ChatOptions, Stream
│   └── registry.go               # ChatRegistry with fallback chains (singleton)
│
├── orchestration/                # AI Orchestration Engine
│   ├── ports.go                  # 6 port interfaces (consumer)
│   ├── types.go                  # Request, Result, PipelineContext, StreamEvent
│   ├── context.go                # Context Builder (Knowledge + Memory)
│   ├── router.go                 # Agent Router (18 categories by keyword)
│   ├── executor.go               # Agent Runner (sync + streaming + retry)
│   ├── pipeline.go               # Skill Pipeline (sequential + parallel)
│   ├── mag.go                    # Memory-Augmented Generation
│   ├── orchestrator.go           # Concrete Engine (wiring of all ports)
│   ├── executor_test.go          # 57 tests
│   ├── router_test.go            # 27 tests
│   └── pipeline_test.go          # 39 tests
│
├── adapter/                      # Adapters (subsystem -> port)
│   ├── knowledge.go              # knowledge.Engine -> KnowledgeSearcher
│   └── memory.go                 # memory.Engine -> Retriever + Storer
│
└── providers/                    # 9 LLM Chat Providers
    ├── openai/chat.go            # GPT-4o (725 lines + 41 tests)
    ├── anthropic/chat.go         # Claude Messages API (800 lines + 37 tests)
    ├── azure/chat.go             # Azure OpenAI (700 lines)
    ├── bedrock/chat.go           # AWS Bedrock SigV4 (650 lines)
    ├── deepseek/chat.go          # DeepSeek Chat (350 lines + 22 tests)
    ├── google/chat.go            # Gemini (450 lines + 30 tests)
    ├── groq/chat.go              # Groq (340 lines + 21 tests)
    ├── mistral/chat.go           # Mistral (340 lines + 21 tests)
    └── ollama/chat.go            # Ollama local (370 lines + 26 tests)
```

## Dependencies

| Component | Depends on |
|-----------|-----------|
| Context Builder | `internal/knowledge/`, `internal/memory/` |
| Router | `internal/agents/` |
| Executor | Providers (LLM), Skills |
| Pipeline | Skills, Workflows |
| MAG | Memory Engine, Knowledge Engine |

## Implementation Status

| Component | Status | File | Lines |
|-----------|--------|---------|--------|
| Core Engine (Orchestrator) | OK | orchestrator.go | 320 |
| Context Builder | OK | context.go | 176 |
| Router (18 categories) | OK | router.go | 244 |
| Executor (sync + streaming) | OK | executor.go | 706 |
| Skill Pipeline | OK | pipeline.go | 672 |
| MAG | OK | mag.go | 542 |
| Port Interfaces | OK | ports.go | 174 |
| Core Types | OK | types.go | 233 |
| Knowledge Adapter | OK | adapter/knowledge.go | 116 |
| Memory Adapter | OK | adapter/memory.go | 143 |
| Chat Providers (9) | OK | providers/*/chat.go | ~5K |
| Provider Tests (199) | OK | providers/*/chat_test.go | ~8K |
| CLI: cosca run | OK | cli/run.go | 250 |
| CLI: cosca pipeline | OK | cli/pipeline.go | 230 |
| ADR-006 | OK | docs/adr/ | 648 |
| Tool Executor (6 tools) | OK | tool_exec.go | 709 |
| Embedding Cache | OK | embed_cache.go | 229 |
| Multi-modal (images) | OK | chat/types.go | 78 |
| Pipeline Metrics | OK | metrics.go | 150 |
| Integration Tests (15) | OK | integration_test.go | 850 |
| Azure Tests (47) | OK | azure/chat_test.go | 1463 |
| Bedrock Tests (41) | OK | bedrock/chat_test.go | 1478 |
| Tool Exec Tests (55+) | OK | tool_exec_test.go | 1568 |
| Total Tests | OK | ~400+ across 44 packages | ~28K lines total |

## Next Steps

1. End-to-end integration tests
2. Multi-modal support (images) across providers
3. Embedding cache for the Context Builder
4. Pipeline metrics and observability
5. Web dashboard for monitoring
