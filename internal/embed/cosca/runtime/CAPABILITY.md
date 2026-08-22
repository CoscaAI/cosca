# Capability-First Architecture

> **Extracted from**: KERNEL.md §2 | **Lines**: ~811 | **Date**: 2026-07-28

## Core Principle

**Resolve capabilities, never departments.** Every request is mapped to a set of capabilities, and the Runtime dispatches to the appropriate agents/skills that provide those capabilities.

## Why Capabilities Over Departments

| Approach | Problem |
|----------|---------|
| **Department Routing** | Tight coupling, rigid hierarchy, can't reuse across departments |
| **Capability Routing** | Loose coupling, flexible composition, cross-cutting concerns |

## Capability Definition

A capability is a named unit of functionality that:
- Has a clear interface (inputs/outputs)
- Can be provided by multiple agents
- Can be composed with other capabilities
- Can be discovered at runtime

```yaml
capability:
  name: "code-review"
  inputs:
    - type: "string"      # code to review
    - type: "string"      # language
  outputs:
    - type: "object"      # review findings
  providers:
    - agent: "cosca-review"
    - skill: "specialist-review-code"
```

## Capability Registry

All capabilities are registered in `capabilities/` directory:

```
capabilities/
├── code-review/
│   ├── SKILL.md
│   └── CAPABILITY.yaml
├── security-audit/
│   ├── SKILL.md
│   └── CAPABILITY.yaml
└── ...
```

## Resolution Process

1. **Request** → Parse intent, extract required capabilities
2. **Match** → Find capabilities that satisfy the request
3. **Select** → Choose best provider (availability, load, expertise)
4. **Dispatch** → Send work to selected provider
5. **Collect** → Gather results, merge if needed
6. **Return** → Unified response to caller

## Capability Composition

Capabilities can be composed into complex workflows:

```yaml
workflow:
  name: "full-stack-feature"
  capabilities:
    - backend-api
    - frontend-component
    - database-schema
    - security-review
    - documentation
  execution: "parallel"  # or "sequential"
```

## Fallback Behavior

If no provider is available for a capability:
1. Check if capability can be decomposed into sub-capabilities
2. Check if neighboring capability can handle (with adapter)
3. Return error with suggested alternatives
