# ARCHITECTURE DETECTOR

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Bootstrap

## PURPOSE
Detect the architectural pattern of the workspace by analyzing directory structure, module organization, and configuration.

## DETECTION RULES

### Pattern Detection by Structure

#### Monorepo
```
Detection: pnpm-workspace.yaml OR lerna.json OR nx.json OR turbo.json
           OR multiple package.json in subdirectories
Pattern: monorepo
Tool: turbo / nx / lerna / pnpm workspaces
```

#### Microservices
```
Detection: Multiple directories with independent package.json +
           docker-compose.yml with multiple services +
           OR message queue (kafka/rabbitmq) in dependencies
Pattern: microservices
Indicators: services/, apps/ with independent builds
```

#### Modular Monolith
```
Detection: src/modules/ OR src/domain/ organized by feature
           Single package.json, no workspace config
Pattern: modular-monolith
Indicators: feature-based folder structure, shared kernel
```

#### MVC
```
Detection: src/controllers/ + src/models/ + src/views/
Pattern: mvc
Common in: Express, Django, Rails, Spring MVC
```

#### Clean / Hexagonal
```
Detection: src/domain/ + src/application/ + src/infrastructure/
           OR src/core/ + src/adapters/ + src/ports/
Pattern: clean-architecture
Common in: NestJS, FastAPI (advanced), Spring (advanced)
```

#### DDD (Domain-Driven Design)
```
Detection: domain/ OR bounded contexts with separate aggregates
           + domain events, value objects patterns in code
Pattern: ddd
Indicators: context/, domain/, aggregate roots
```

#### Component-Based
```
Detection: src/components/ + src/pages/ (Next.js pages/app router)
Pattern: component-based
Common in: React, Vue, Next.js
```

#### Serverless
```
Detection: serverless.yml OR SAM template OR @aws-cdk OR vercel.json functions
Pattern: serverless
Indicators: lambda/ functions/, vercel functions
```

### Entry Point Detection
| Framework | Entry Point |
|-----------|------------|
| Next.js | pages/ or app/ directory |
| NestJS | src/main.ts |
| Express | src/index.ts, app.ts, server.ts |
| FastAPI | main.py |
| Django | manage.py |
| Spring | Application.java |

## OUTPUT
```yaml
detection:
  pattern: modular-monolith
  confidence: high
  indicatos:
    - "src/modules/ organized by feature"
    - "Single package.json"
  monorepo: false
  monorepo_tool: null
  entry_point: src/main.ts
  modules:
    - auth
    - users
    - orders
    - notifications
```

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Cosca Bootstrap | Initial detector |
