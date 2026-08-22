# CHANGELOG

> **Format**: Based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
> **Versioning**: [SemVer](https://semver.org/spec/v2.0.0.html)

## [3.0.7] — 2026-07-31 — MEMÓRIA RESTAURADA (Backup Recovery — Ordem do Don)

### Fixed
- **Memória de agentes restaurada** a partir de `/home/cosca/Documents/cosca-test-bk--noop/` (fonte autorizada pelo Don — snapshot NÃO usado): 54 diretórios de agentes (learnings/patterns/failures/capability-profile/evolution), incluindo `cosca-kernel/learnings.md` (142KB, 37 learnings L9-L40) que estavam ausentes do workspace
- **13 diretórios de memória restaurados**: context, codebase, decisions, roadmap, risk, evolution, review, compliance, cli, sdk, session, testing, dependencies + sessions/archive
- **3 flat files de performance restaurados**: agent-kernel-performance.md, agent-architecture-performance.md, agent-review-performance.md
- **TRUST_REGISTRY.md + ENGINEERING_TIMELINE.md** atualizados para a versão do backup (mais recente)
- **knowledge.db populado**: 459 documentos + 8.370 chunks indexados (memória semântica ativa; antes vazia no workspace)
- **Diretórios extras restaurados**: cli/ (35), diagrams/ (4), scripts/ (7), sdk/ (7), skills/cli, skills/sdk, .cosca-scaffold

### Changed
- **opencode.json**: permissões edit/write/read/task/glob/grep corrigidas de `/home/cosca/Documents/cosca-test/` (inexistente) para `/home/cosca/Documents/cosca/`
- **Links relativos corrigidos**: 179 → 4 (restantes são artefatos de runtime intencionais — auctions, predictions, gate-results)
- **Inventários sincronizados**: HELP.md (41 departments, 64 engines), COSCA_INDEX.md (41 departments, 64 engines, 38 workflows, 14 templates, 990+ files)

### Not Changed (intencional)
- `sync.go`/`sync_test.go` mantidos como versão do workspace (anti-loop guard — mais nova que a do backup)

## [3.0.6] — 2026-07-30 — ADAPTIVE PERSONALITY ENGINE v2.0.0 (F3.4 REDESIGN)

### Changed
- **Adaptive Personality Engine** (`engines/adaptive-personality/SKILL.md`): redesigned from 6-dimension/6-profile context detection (v1.0.0) to 5-axis/5-mode pre-task state selection (v2.0.0)
  - 5 axes: Velocidade, Risco, Profundidade, Tom, Autonomia (0-100 continuous spectrum)
  - 5 modes per task type (Don's literal table): Cirúrgico, Exploratório, Sprint, Cauteloso, Zen
  - Pre-task pipeline < 50ms: tipo×risco×urgência×custo → mode selection (no LLM in hot path)
  - Don adaptation: preference memory ("continue" → Sprint, "travou?" → Cauteloso, "mostre tabela" → tables) with moving average of 5 tasks
  - Post-task recalibration: every Don reaction adjusts axis deltas on the default mode
  - F3.1 integration: don-style insights feed preference memory (evidence-required)
  - F2.5 integration: momentum high → Exploratório favored (exploratory_bias)
  - F3.5 integration: low energy → Cauteloso/Zen axis deltas (mapping for 5 axes)
  - F1.2/F7.2 integration: Cauteloso forces Contrafactual Gate; Trust Registry adjusts effective risk
  - Safety rule: **P0 always Cauteloso** (immutable — replaces v1.0.0 EMERGENCY max-autonomy)
  - CLI: `cosca personality --mode <modo>` (Don override total)
  - Real example: "remover dead code" (F0 Wave 1) → Cirúrgico (2m30s, $0.002, 6 agents parallel, L22 "remoção cirúrgica")
- **COSCA_INDEX.md**: Adaptive Personality engine entry updated to ★ F3.4 v2.0.0

### Philosophy
- Personality is an operating state, not a voice: the same Cosca corrects surgically, designs exploratorily, decides cautiously, and delegates in sprints
- Safety over speed: no mode, override, or bias ever reduces verification on a P0 decision

## [3.0.5] — 2026-07-30 — MENTAL ENERGY ENGINE v2.0.0 (F3.5 REDESIGN)

### Changed
- **Mental Energy Engine** (`engines/mental-energy/SKILL.md`): redesigned from 5-pool model (v1.0.0) to unified energy model (v2.0.0)
  - Energy as a single managed resource: `energy = 100` max
  - Task cost formula (Don's literal spec): `task_cost = complexity × 15 + agents_mobilized × 10 + tokens/10K × 5`
  - Regen formula: `10/hora idle + 5/hora leve + 0/hora intensa`
  - 4 energy states renamed: FULL (> 70), FOCUSED (40-70), CONSERVATIVE (15-40), RECOVERY (< 15)
  - Continuous 6-step pipeline (< 10ms) with session-end projection ("energia 62/100 — 3 tasks de ~12 = fim de sessão em 2 tasks")
  - 4 fatigue indicators (B1 load sustained, F7.2 success_rate dropping, F2.5 momentum dropping, avg latency rising) → fadiga_score modulating the admission gate
  - F2.1 integration: maintenance tasks (F9.1, F1.6) as recovery — cheap, context-regenerating, virtuous cycle
  - F3.4 integration: low energy → Cauteloso/Zen personality deltas (more rigor, less speed)
  - 3 laws: P0 never blocked, RECOVERY mandatory < 15, Don override `cosca energy --refill`
  - Real example: 12-wave session (30+ agents, ~50 tasks) → 750 gasto vs 80 regen → deficit 670 → RECOVERY mandatory at end
- **COSCA_INDEX.md**: Mental Energy engine entry updated to ★ F3.5 v2.0.0

### Philosophy
- Simplification is architectural: 5 pools of budgets did not translate into better decisions; one number, one formula, four states do
- Fatigue is the symptom that appears before the number shows it — fadiga_score reads B1/F7.2/F2.5/latency to catch exhaustion early

## [3.0.4] — 2026-07-30 — MENTAL ENERGY ENGINE

### Added
- **Mental Energy Engine** (`engines/mental-energy/SKILL.md`): Dynamic cognitive resource budgeting for the Cosca Runtime (Fase 3, C7)
  - 5 energy pools: TOKENS (35%), TIME (25%), ATTENTION (25%), DEPTH (10%), PARALLEL (5%)
  - 4 energy states: HIGH (80-100%, explore), MEDIUM (40-79%, standard), LOW (10-39%, conserve), CRITICAL (0-9%, essential only)
  - Energy Gate: pre-action cost estimation and budget validation algorithm
  - Energy Debt: borrow future energy at 20% interest with Don approval
  - Joint decision matrix: Mental Energy × Cognitive Economy (4 quadrants)
  - Recovery mechanisms: session reset, mid-session break, efficiency bonus, cooldown
  - 4 budget profiles: exploratory, standard, economical, critical_incident
  - Dashboard ASCII with CLI commands, session energy reports, metrics
  - Integrates with KERNEL.md pipeline (Step 6.5), Cognitive Economy, Cognitive Momentum, Cognitive Gravity, UCSS Cognitive State
  - Practical session example: F0+F1+F2+F3 trajectory (100% → 95% → 72% → 48% → ~20%)
  - CMI Impact: Planejamento +8, Julgamento +4, Eficiência +6
- **COSCA_INDEX.md**: Mental Energy engine added to engines inventory
- **cosca-runtime learnings**: F3.5 specification registered as Level 4 learning

### Philosophy
- Implements "Cada investigação consome energia" as described by the Don: high energy → explore, low energy → reuse
- Mental Energy is the runtime's awareness of its own resource consumption — the "budget consciousness" the Kernel lacks today

## [3.0.3] — 2026-07-30 — CONTRAFACTUAL GATE

### Added
- **Contrafactual Gate** (`workflows/contrafactual-gate.md`): Mandatory pre-decision gate for P0/P1 strategic decisions
  - Forces "What if the opposite decision had been made?" before committing
  - 5-dimension comparison: Risk, Cost, Time, Knowledge Gain, Reversibility
  - Structured YAML output with `proceed | escalate | reject` outcomes
  - Includes full example: memfd_create vs script-based for auto-jail
- **Gate 0.5** in `QUALITY_GATES.md`: Contrafactual Decision Review integrated into quality gate architecture
- **KERNEL.md §10**: Gate 0.5 added to quality gate enforcement sequence
  - Invoked between Step 6 (Capability Resolution) and Step 7 (Planning)

### Changed
- `QUALITY_GATES.md` v1.0.0 → v1.1.0: New Gate 0.5 section with checks, outcome rules, and integration diagram
- Gate architecture updated: 5 gates → 6 gates (Gate 0.5 inserted between Gate 0 and Gate 1)

### Philosophy
- Implements "pensamento contrafactual" as described by the Don: before crystallizing any strategic decision, actively seek evidence supporting the opposite
- cosca-critic assigned as gate owner — natural extension of adversarial review responsibilities

## [3.0.3.1] — 2026-07-30 — CONTRAFACTUAL GATE v2 (ENGINE UPGRADE)

### Added
- **Contrafactual Gate v2.0.0** (`workflows/contrafactual-gate.md`): Upgrade from workflow to full decision engine
  - **6 activation triggers**: G1 (P0), G2 (multi-module), G3 (cost > $0.01), G4 (P1 confidence < 0.7), G5 (DDNA pending), G6 (Don override)
  - **Gate Open/Gate Closed pipeline**: 7-step formal process with state machine (IDLE → OPENING → ANALYZING → VOTING → ACCEPT/ESCALATE/REJECT → CLOSING → IDLE)
  - **DDNA integration (F1.1)**: `Options` populated by Gate, `evidence` links to Gate analysis, `confidence` calculated by Gate
  - **Trust Registry integration (F7.2)**: Gate queries agent reputation (domain success rate, bias profile) before scoring risk
  - **5 escalation rules (E1-E5)**: all-high-risk → Don, cost > decision → fast-track skip, Don offline → autonomous mode with documentation
  - **6 design principles (D1-D6)**: lightweight, skippable, recommend-not-decide, automatable, auditable, calibratable
  - **Cost-based activation**: configurable $0.01 threshold (G3) with `max_analysis_cost_ratio` guard
  - **Fast-track mode**: automatic skip when analysis cost > 50% of decision cost
  - **Don offline mode**: autonomous execution with `status: proposed` and `don_review_required: true`
  - **Metrics**: 11 gate metrics with alert thresholds (cost ratio, confidence, skip rate, escalations)
  - **YAML output v2**: expanded with trigger metadata, trust registry results, per-alternative impact matrix, final decision block
  - Enhanced example: auto-jail decision with all new fields populated (Trust Registry consulted, impacts computed, escalations evaluated)

### Changed
- `workflows/contrafactual-gate.md` v1.0.0 → v2.0.0: Full engine redesign with pipeline metaphor, activation matrix, and cross-system integrations
- Engine path established: `engines/decision/contrafactual-gate.md` (mirror of workflow file for F1.2 compliance)

### Philosophy
- Upgrades contrafactual from "review step" to "decision motor" — the Gate generates alternatives, doesn't just validate them
- Cost-awareness: the Gate must be cheaper than the decision it evaluates — otherwise it's skipped
- Layered escalation: Gate recommends → Kernel decides → Don (if escalated) — clear chain of command

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
- **Shared References** (`internal/embed/cosca/shared/`): Eliminated ~23KB of duplicated text
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

## [3.0.2] — 2026-07-29 — KNOWLEDGE PIPELINE + COVERAGE OFFENSIVE

### Added
- **Knowledge Pipeline Phase 1**: 20 heuristics (YAML), 3 playbooks, 3 benchmarks, KB schema (V001)
- **CI G5b**: Branch coverage gate (≥ 60%, basic-block approximation)
- **CI G6b**: Memory validation gate (learnings.md integrity check)
- **CI web-test**: Frontend test job (lint + typecheck + vitest ≥ 80%)
- **Heuristics directory**: `internal/embed/cosca/knowledge/heuristics/` — 20 rules across 8 domains
- **DB Schema**: `schema/V001__initial.sql` — heuristics, patterns, playbooks tables + FTS5

### Changed
- CI coverage threshold: 55% → 70% (now matches qa/quality-gates.md G5)
- Makefile `coverage-check`: 40% → 70% (+ `./pkg/...` packages)
- `runServe()` refactored: extracted `loadDotEnv()`, `resolveDataDir()`, `configureCORSFromEnv()`

### Fixed
- Threshold crisis: 4 conflicting values (40%/55%/70%/80%) unified to 70% operational
- `jail.go` coverage: 0% → ~70% (6 security functions now tested)
- `api/rest/handler` runtime.go: 14.5% → 95%
- `pkg/cosca` SDK heartbeat: 21% → 96%
- `internal/cli` coverage: 46.9% → 71.5%
- `state-audit-2026-07-25.md`: 4 resolved stubs still marked as active

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
