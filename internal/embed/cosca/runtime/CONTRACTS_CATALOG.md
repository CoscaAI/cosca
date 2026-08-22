# Runtime Contracts Catalog

> **Extracted from**: KERNEL.md section 7 | **Lines**: ~1,049 | **Date**: 2026-07-28

## Overview

This document catalogs all formal contracts that agents, skills, and plugins must adhere to.

## Agent Contract

Every agent must implement:

```go
type Agent interface {
    Name() string
    Capabilities() []Capability
    Execute(ctx context.Context, req Request) (Response, error)
    Health() HealthStatus
}
```

## Skill Contract

Every skill must provide:

```yaml
skill:
  name: string
  version: string
  capabilities: [string]
  inputs: [PortDefinition]
  outputs: [PortDefinition]
  timeout: duration
```

## Plugin Contract

Every WASM plugin must export:

```wasm
;; Required exports
(export "name" (func $get_name))
(export "version" (func $get_version))
(export "execute" (func $execute))
(export "health" (func $health))
```

## Provider Contract

AI providers must implement:

```go
type Provider interface {
    Name() string
    Models() []Model
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (<-chan Chunk, error)
}
```

## Quality Gates

| Gate | Description | Blocks |
|------|-------------|--------|
| Gate 0 | Config valid | All execution |
| Gate 1 | Security check | Deployment |
| Gate 2 | Tests pass | Release |
| Gate 3 | Performance OK | Production |
| Gate 4 | Documentation complete | Archive |
