# Strategic Roadmap — Cosca

> **Status**: active | **Owner**: Cosca Kernel / PMO | **Last Updated**: 2026-07-29
> **Plan Version**: 1.0 | **Type**: Strategic

This document is the **official source** for all future Cosca development.

---

## Table of Contents

- [Overview](#overview)
- [Current State](#current-state)
- [v1.0 Epics](#v10-epics)
- [v2.0 Epics](#v20-epics)
- [Dependency Graph](#dependency-graph)
- [Quality Gates](#quality-gates)
- [Golden Rules](#golden-rules)
- [Detailed Files](#detailed-files)

---

## Overview

```
╔══════════════════════════════════════════════════════════════╗
║              Cosca — EVOLUTION ROADMAP                     ║
║                                                              ║
║  CURRENT STATE:    78/100 — Enterprise Platform Alpha      ║
║  SCORE v1.0:       80/100 — Production Ready                ║
║  SCORE v2.0:       98/100 — Enterprise Platform             ║
║                                                              ║
║  EPICS: 11                                                   ║
║  MILESTONES: 32                                              ║
║  FEATURES: ~120                                              ║
║                                                              ║
║  ESTIMATE v1.0: 4-5 months                                   ║
║  ESTIMATE v2.0: 12 months                                    ║
║                                                              ║
║  FUNDAMENTAL RULE:                                           ║
║  Never implement out of order.                               ║
║  Never skip quality gates.                                   ║
║  Never commit without tests.                                 ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
```

---

## Current State

| Category | % | Details |
|-----------|---|----------|
| Architecture | 95% | 6 layers (CLI + REST API + Web Console + Orchestration + Subsystems + Infra), ADRs. Excellent |
| CLI Layer | 90% | 37 commands. Serve, run, chat, pipeline functional |
| Runtime Engine | 90% | State machine, event bus, lifecycle. Complete |
| Knowledge Engine | 85% | FTS5+Vector+Graph+Indexer. Real via REST |
| Memory Engine | 85% | 5 layers, 7 types. Real via REST |
| Discovery Engine | 85% | 8 areas. Complete |
| Plugin System | 50% | Structure OK. WASM/External/SharedLib runtimes partially implemented |
| Editor Adapters | 70% | 9/9 defined. Functional |
| Providers | 60% | 10 embedding providers + 10 chat providers. Shared transport |
| API Layer | 90% | 36 REST endpoints, JWT auth, RBAC, OpenAPI 3.0 spec, CORS, logging middleware |
| Web Console | 85% | 17 routes, JWT auth, RBAC, dark theme, command palette. Fully functional |
| SDKs | 40% | TS SDK functional. Go SDK in progress |
| Tests | 40% | 500+ Go tests. 354 Vitest tests. Playwright E2E. Storybook |
| CI/CD | 60% | GitHub Actions: lint, vet, test, race, coverage, build. GoReleaser |
| Docker/Infra | 60% | Multi-container Docker, docker-compose, Helm charts, Terraform, Prometheus |
| Security | 85% | JWT HS256, RBAC, rate limiting, CSP, CSRF, bcrypt, httpOnly cookies |
| Documentation | 90% | Excellent. ADRs, guides, troubleshooting |
| **Overall** | **74/100** | Enterprise platform alpha — web console, REST API, security, tests operational |

---

## v1.0 Epics

### EPIC-001: Foundation (3-4 weeks)
**Objective:** Quality infrastructure, testing, and CI/CD

| Milestone | Features | DoD |
|-----------|----------|-----|
| M1.1 Test Infrastructure | Unit tests, mocks, all packages | 200+ tests, coverage ≥40% |
| M1.2 CI/CD Pipeline | GitHub Actions, lint, test, build, release | Every PR runs CI |
| M1.3 Docker | Multi-stage Dockerfile, docker-compose | Image <50MB |

### EPIC-002: Core Stabilization (6-8 weeks)
**Objective:** Complete critical subsystems

| Milestone | Features | DoD |
|-----------|----------|-----|
| M2.1 Plugin Runtime | WASM (wazero), External, SharedLib, git clone | Go+WASM plugins functional |
| M2.2 Local Embedding | ONNX model, automatic fallback | Knowledge engine offline |
| M2.3 Providers | Anthropic, Azure, Mistral, Groq, DeepSeek, Bedrock | 6 new providers |
| M2.4 Editor Adapters | Windsurf, Zed, tests | 9 functional editors |

### EPIC-003: Subsystem Completion (4-5 weeks)
**Objective:** Complete all stubs

| Milestone | Commands to implement |
|-----------|----------------------|
| M3.1 Agents | `cosca agent list`, `cosca agent info`, `cosca agent run` |
| M3.2 Skills | `cosca skill list`, `cosca skill info`, `cosca skill validate` |
| M3.3 Prompts | `cosca prompt list`, `cosca prompt info`, `cosca prompt create` |
| M3.4 Workflows | `cosca workflow list`, `cosca workflow run` |
| M3.5 Templates | `cosca template list`, `cosca template render` |
| M3.6 Context Builder | `cosca context build` generates full context |

### EPIC-004: Quality & Documentation (3-4 weeks)
**Objective:** Performance, security, documentation

- Benchmarks for search, index, startup
- Security audit (permissions, path traversal, SQL injection)
- Documentation sync with real code
- Integration tests (20+ critical flows)

### EPIC-005: Release v1.0 (2-3 weeks)
**Objective:** Official launch

- Feature freeze, release candidates
- 7 quality gates (architecture, security, performance, testing, docs, UX, release)
- Coverage ≥70%, 0 critical vulns, green CI/CD
- Tag `v1.0.0`, cross-platform builds, Docker image

---

## v2.0 Epics

### EPIC-006: API Layer (6-8 weeks) — **MOSTLY COMPLETE (90%)**
- ✅ gRPC API (proto definitions, server, client, auth interceptor)
- ✅ REST API (OpenAPI 3.0, 36 endpoints /v1/*)
- ✅ MCP Server enhancement (standalone mode, tools, resources)
- ✅ JWT auth, RBAC, middleware chain
- ⚠ WebSocket real-time events (in progress)

### EPIC-007: SDK Ecosystem
- Go SDK (client, knowledge, memory, runtime methods)
- TypeScript SDK (NPM package, AOSClient, types, vitest)
- SDK documentation and examples

### EPIC-008: Security & Governance — **PARTIALLY COMPLETE (50%)**
- ✅ JWT Authentication (HS256, stdlib-only)
- ✅ RBAC (admin, editor, viewer)
- ⚠ Audit logging (structured, immutable) — in progress
- OIDC/OAuth2 authentication (planned)
- Multi-tenancy (directory isolation) (planned)

### EPIC-009: Enterprise Infrastructure
- Helm charts (cosca-server, values dev/staging/prod)
- Terraform modules (AWS, GCP, Azure)
- Observability (Prometheus, Grafana, OpenTelemetry)
- Server mode (`cosca server start`, headless)

### EPIC-010: Platform Ecosystem
- Plugin Registry (server, submission, versioning)
- Web Dashboard (knowledge explorer, memory viewer, health)
- Cloud Sync (optional, team sharing)

### EPIC-011: Release v2.0
- Penetration testing, load testing, disaster recovery
- OWASP, SCA, SAST
- Tag `v2.0.0`, enterprise documentation

---

## Dependency Graph

```
EPIC-001 (Foundation)
    │
    ▼
EPIC-002 (Core Stabilization)
    │
    ▼
EPIC-003 (Subsystem Completion)
    │
    ▼
EPIC-004 (Quality & Documentation)
    │
    ▼
EPIC-005 (Release v1.0)  ←  🚀 v1.0 MILESTONE
    │
    ├──────────────────────────────┐
    ▼                              ▼
EPIC-006 (API Layer)       FUTURE ROADMAP
    │
    ▼
EPIC-007 (SDK Ecosystem)
    │
    ▼
EPIC-008 (Security & Governance)
    │
    ▼
EPIC-009 (Enterprise Infrastructure)
    │
    ▼
EPIC-010 (Platform Ecosystem)
    │
    ▼
EPIC-011 (Release v2.0)  ←  🚀 v2.0 MILESTONE
```

**Parallelizable:**
- M3.1 through M3.5 (among themselves)
- M2.1, M2.2, M2.3 (among themselves)
- M6.1, M6.2 (among themselves)

---

## Quality Gates

| Gate | When | Blocking |
|------|--------|-----------|
| 🏗️ Architecture | End of each Epic | ✅ Yes |
| 🔒 Security | Before release | ✅ Yes |
| ⚡ Performance | Before release | ✅ Yes (regression >20%) |
| 🧪 Testing | Every PR | ✅ Yes |
| 📚 Documentation | End of each Milestone | ❌ Warning |
| 🚀 Release | Before each release | ✅ Yes |

---

## Golden Rules

```
R-001: Do not implement EPIC-002 before EPIC-001 reaches 60%
R-002: Do not implement EPIC-003 before EPIC-002 reaches 80%
R-003: Do not implement EPIC-005 before EPIC-001 through EPIC-004 reach 100%
R-004: Do not start v2.0 before v1.0 is released
R-005: Every PR must pass CI/CD (after EPIC-001)
R-006: All new code must have tests
R-007: Every new feature must have documentation
R-008: No stub may be removed without a functional replacement
R-009: No existing interface may be broken without an ADR
R-010: No existing CLI command may change signature without deprecation
```

---

## Detailed Files

For the complete plan with all tasks, subtasks, detailed features, acceptance criteria, and risk matrix, see the epic descriptions above. Detailed per-epic files are planned for creation as each epic is initiated:

- `EPIC-001-foundation.md` (planned)
- `EPIC-002-core-stabilization.md` (planned)
- `EPIC-003-subsystem-completion.md` (planned)
- `EPIC-004-quality-documentation.md` (planned)
- `EPIC-005-release-v1.md` (planned)
- `EPIC-006-api-layer.md` (planned)
- `EPIC-007-sdk-ecosystem.md` (planned)
- `EPIC-008-security-governance.md` (planned)
- `EPIC-009-enterprise-infrastructure.md` (planned)
- `EPIC-010-platform-ecosystem.md` (planned)
- `EPIC-011-release-v2.md` (planned)

---

> **Related**: [Architecture Overview](../architecture/overview.md) | [ADR-001](../adr/ADR-001-cosca-cli-architecture.md) | [CLI Reference](../cli/commands.md)
