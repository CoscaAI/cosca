---
name: discovery
description: Automatically discovers the technology landscape of any workspace without manual configuration.
level: 2
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Discovery Engine | **Last Updated**: 2026-07-10

# DISCOVERY ENGINE

## PURPOSE
The Discovery Engine automatically discovers the technology landscape of any workspace. It identifies frameworks, languages, tools, patterns, and structure without manual configuration.

## DISCOVERY PROCESS

### Step 1: Language Detection
```
Check for language-specific files:
- package.json → JavaScript/TypeScript (Node.js)
- tsconfig.json → TypeScript
- requirements.txt, pyproject.toml, setup.py → Python
- Cargo.toml → Rust
- go.mod → Go
- Gemfile → Ruby
- pom.xml, build.gradle → Java/Kotlin
- composer.json → PHP
- Mix.exs → Elixir
- CMakeLists.txt → C/C++
```

### Step 2: Framework Detection
```
Check dependencies in package/project files:
- next, nuxt, sveltekit, remix → Meta-framework
- react, vue, svelte, angular → Frontend framework
- express, fastify, nest, koa → Backend framework (Node)
- django, flask, fastapi → Backend framework (Python)
- rails, sinatra → Backend framework (Ruby)
- spring, quarkus, micronaut → Backend framework (Java)
- gin, echo, fiber → Backend framework (Go)
- actix, axum, rocket → Backend framework (Rust)
- phoenix → Backend framework (Elixir)
```

### Step 3: Database Detection
```
Check dependencies and config files:
- pg, postgres, psycopg2 → PostgreSQL
- mysql, mysql2 → MySQL
- sqlite3, better-sqlite3 → SQLite
- mongodb, mongoose → MongoDB
- redis, ioredis → Redis
- prisma, typeorm, sequelize, knex → ORM
- alembic, flyway → Migration tool
```

### Step 4: Build System Detection
```
Check for:
- package.json scripts → npm/yarn/pnpm
- Makefile → make
- Dockerfile → Docker
- docker-compose.yml → Docker Compose
- .github/workflows/ → GitHub Actions
- .gitlab-ci.yml → GitLab CI
- Jenkinsfile → Jenkins
```

### Step 5: Test Framework Detection
```
Check dependencies:
- jest, vitest, mocha → Test framework (JS/TS)
- pytest → Test framework (Python)
- rspec → Test framework (Ruby)
- junit, testng → Test framework (Java)
- cypress, playwright, selenium → E2E framework
- supertest, pytest-httpx → API testing
```

### Step 6: Linting/Formatting Detection
```
Check config files:
- .eslintrc.*, eslint.config.* → ESLint
- .prettierrc.* → Prettier
- biome.json → Biome
- .rubocop.yml → Rubocop
- .pylintrc → Pylint
- ruff.toml → Ruff
- checkstyle.xml → Checkstyle
- .golangci.yml → golangci-lint
```

### Step 7: Architecture Pattern Detection
```
Analyze directory structure:
- src/controllers, src/models, src/views → MVC
- src/domain, src/application, src/infrastructure → DDD/Clean
- src/modules/ → Modular Monolith
- services/ directory per service → Microservices
- src/components/, src/pages/ → Component-based (React/Vue)
- app/, domain/, infra/ → Hexagonal
- context/ per bounded context → DDD
```

### Step 8: Monorepo Detection
```
Check for:
- Workspace configuration (pnpm-workspace.yaml, lerna.json, nx.json, turbo.json)
- Multiple package.json files
- Gradle multi-project
- Cargo workspace
```

### Step 9: API Style Detection
```
Analyze code for:
- app.get/post/put/delete → Express-style REST
- @Get/@Post decorators → Nest/Spring decorators
- GraphQL schema files → GraphQL
- .proto files → gRPC
- TRPC routers → tRPC
- OpenAPI/Swagger files → REST with OpenAPI
```

### Step 10: Infrastructure Detection
```
Check for:
- Dockerfile, Dockerfile.* → Docker
- docker-compose.yml → Docker Compose
- k8s/, kubernetes/, deploy/ → Kubernetes
- terraform/, *.tf → Terraform
- Pulumi.yaml → Pulumi
- helm/ → Helm
- .env, .env.example → Environment variables
```

## DISCOVERY OUTPUT

```json
{
  "project": {
    "name": "string",
    "type": "web|api|mobile|cli|library|microservice",
    "monorepo": true|false
  },
  "languages": ["typescript", "python"],
  "frameworks": {
    "frontend": "next.js",
    "backend": "fastapi",
    "meta": null
  },
  "runtime": "node|python|jvm|go|rust",
  "build_system": "npm|pnpm|yarn|cargo|gradle",
  "package_manager": "npm|pnpm|yarn",
  "database": {
    "primary": "postgresql",
    "secondary": ["redis"],
    "orm": "prisma",
    "migrations": "prisma migrate"
  },
  "testing": {
    "unit": "vitest",
    "integration": "vitest",
    "e2e": "playwright"
  },
  "linting": {
    "linter": "eslint",
    "formatter": "prettier"
  },
  "architecture": {
    "pattern": "modular-monolith|microservices|ddd|mvc|clean",
    "modules": ["auth", "users", "products", "orders"]
  },
  "infrastructure": {
    "container": "docker",
    "orchestration": "docker-compose",
    "ci": "github-actions",
    "cloud": "aws|gcp|azure|none"
  },
  "documentation": {
    "readme": true,
    "contributing": false,
    "adr": false,
    "api_docs": false
  }
}
```

## DEPENDENCIES
- Called by Kernel at session start
- Called by Context Engine
- Feeds into Memory Engine for project context
- Used by Planning Engine for technology-aware planning

## RELATED
- [Planning Engine](../planning/SKILL.md) — Consumes tech landscape for informed plans
- [Wizard Engine](../wizard/SKILL.md) — Discovery may supplement feature intake

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Added metadata, HISTORY, and cross-references |
