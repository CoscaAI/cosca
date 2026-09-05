# Multi Runtime

> **Extracted from**: KERNEL.md section 17 | **Lines**: ~559 | **Date**: 2026-07-28

## Overview

Cosca supports multiple runtime environments: Go Runtime, OpenCode, Claude Code, Codex, ADK-Go, and future integrations.

## Runtime Environments

| Runtime | Type | Capabilities |
|---------|------|--------------|
| Go Runtime | Native | Full (CLI, API, embedded) |
| OpenCode | Editor | Agent, skills, memory |
| Claude Code | Editor | Agent, skills |
| Codex | CLI | Agent, execution |
| ADK-Go | Framework | Agent, orchestration |

## Runtime Abstraction

```go
type Runtime interface {
    Name() string
    Capabilities() []Capability
    Execute(ctx context.Context, req Request) (Response, error)
    Health() HealthStatus
}
```

## Capability Matrix

| Capability | Go | OpenCode | Claude | Codex | ADK |
|------------|----| --------|--------|-------|-----|
| CLI | Yes | No | No | Yes | No |
| REST API | Yes | No | No | No | No |
| gRPC | Yes | No | No | No | Yes |
| SSE | Yes | Yes | No | No | No |
| Memory | Yes | Yes | Yes | Limited | Yes |
| Knowledge | Yes | Yes | No | No | Yes |
| Plugins | Yes | No | No | No | No |

## Cross-Runtime Communication

```
Go Runtime ←──gRPC──→ OpenCode
     │                    │
     └──────SSE───────────┘
```

## Deployment Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| Standalone | Single binary | Development |
| Server + Client | API server + thin client | Production |
| Distributed | Multiple runtimes | Enterprise |
