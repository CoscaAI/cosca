# CHANGELOG

> **Format**: Based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
> **Versioning**: [SemVer](https://semver.org/spec/v2.0.0.html)

## [3.0.1] — 2026-07-29 — SEMANTIC MEMORY KERNEL

### Added
- **Semantic Memory Chief** (`cosca-semantic-memory`): Vector-based knowledge retrieval agent
  - Semantic indexing of all 421+ memory files
  - Cross-agent knowledge discovery (Agent A's patterns found by Agent B)
  - Relevance-ranked search with `Similarity × Freshness × Authority` scoring
  - Auto-reindex on file changes
- **Semantic Memory Engine** (`engines/semantic-memory/`): Vector embedding pipeline
  - Cosine similarity search across all memory
  - SQLite FTS5 + vector store at `.cosca/memory/vectors.db`
  - Integration with Go runtime (`internal/embeddings/`, `internal/search/`)
- **Shared References** (`.opencode/cosca/shared/`): Eliminated ~23KB of duplicated text
  - `AUTO_EVOLUTION_PROTOCOL.md` — canonical reference for all 44 agents
  - `PROJECT_CONTEXT.md` — canonical reference for all 15 agents

### Changed
- `opencode.json`: Reduced from 108KB to ~84KB (21.5% reduction)
- `BOOTSTRAP.md`: Phase 0 health check reduced from 7 to 3 components
- `memory/INDEX.md`: Startup load order reduced from 4 files to 1 (cognitive-state)
- Kernel instructions: Fast path prioritized over full bootstrap

### Fixed
- Kernel startup freeze caused by loading all 54 agent prompts eagerly
- Redundant memory scanning at runtime (cognitive-state fast path now default)

---

## [3.0.0] — 2026-07-23 — ENTERPRISE PLATFORM EVOLUTION

### Added (Major)

#### New Departments (14 Enterprise Chiefs)
- **API Chief**: API lifecycle, contracts, gateways, versioning
- **Performance Chief**: System performance, benchmarking, load testing
- **Platform Chief**: Internal Developer Platform, golden paths, DX
- **Compliance Chief**: Regulatory compliance (GDPR, SOC2, HIPAA, PCI-DSS)
- **Plugin Chief**: Plugin architecture, SDK, registry, marketplace
- **Migration Chief**: Data/system/cloud migrations
- **Provider Chief**: AI/cloud provider management, failover
- **Governance Chief**: Framework governance, convention compliance
- **Cache Chief**: Caching strategy, Redis, CDN, invalidation
- **Messaging Chief**: Event-driven architecture, message brokers
- **CLI Chief**: CLI tools, code generators, developer tooling
- **SDK Chief**: Multi-language SDK/client library development
- **Discovery Chief**: Codebase analysis, architecture discovery
- **Technical Debt Chief**: Technical debt tracking, quality gates

#### New Skills Framework (43 Skills)
- Architecture: Architecture Analysis, Validation, Dependency Analysis, ADR Generation, Architecture Documentation
- Code Quality: Code Review, Refactoring, Technical Debt Analysis, Complexity Analysis
- Security: Security Audit, Vulnerability Assessment, Secrets Audit, Compliance Validation
- Performance: Performance Audit, Load Testing, Database Performance
- Testing: Unit Testing, Integration Testing, E2E Testing, Contract Testing
- Documentation: Documentation Update, API Documentation, ADR Creation
- DevOps: CI/CD Validation, Docker Validation, Kubernetes Validation
- Data: Database Audit, Data Migration Planning, Query Optimization
- AI: Prompt Engineering, Provider Discovery, Embedding Pipeline
- Governance: Convention Validation, Quality Gate, Memory Synchronization
- API: API Audit, OpenAPI Validation, API Design Review
- Platform: Project Bootstrap, Provider Integration, Configuration Validation
- Reliability: Disaster Recovery Planning, Incident Response

#### New Workflows (10)
- API Design Review, Migration Execution, Compliance Audit, Disaster Recovery
- Technical Debt Paydown, Secrets Rotation, Provider Migration
- Performance Optimization, Incident Response, Platform Bootstrap

#### New Templates (5)
- Event-Driven Architecture, AI Platform, CLI Tool, SDK/Library, Plugin Module

### Changed
- **ORGCHART.md**: Expanded from 26 to 40 Chiefs, updated chain of command, redundancy matrix, decision authority
- **COSCA_INDEX.md**: Updated with 280+ files, new index sections for v3.0
- **Framework scope**: Now includes Skills framework as first-class entity

### Summary
- **Total files**: 155 → 280+
- **Departments**: 26 → 40
- **Skills**: 0 → 43 (new category)
- **Workflows**: 10 → 20
- **Templates**: 9 → 14
- **New categories**: Skills, Migration, Incident Response, Platform Engineering

---

Previous releases:
- [2.0.0] — 2026-07-12 — Enterprise Cognitive Platform (155+ files, 64 capabilities, 12 councils, 29 engines)
- [1.2.0] — 2026-07-11 — Capability First Architecture
- [1.1.0] — 2026-07-10 — Resource Resolver, Virtual Paths
- [1.0.0] — 2026-07-10 — Initial Cosca Framework (94 files)
