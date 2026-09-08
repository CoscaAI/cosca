# cosca-platform - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-platform — Semantic Learnings

> Auto-evolution memory.

## Seed Knowledge

### 2026-07-28 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-platform |
| **Task** | Initial capability establishment |
| **Technique** | Standard platform engineering audit patterns |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #platform #baseline #dx |
| **Learned** | Ready for Level 2 techniques. |
| **Next** | Identify first advanced technique to master |

## Audit Learnings (Onda 5 — 2026-07-28)

### 2026-07-28 — Comprehensive Platform Engineering Audit
| Field | Value |
|-------|-------|
| **Agent** | cosca-platform |
| **Task** | Full platform engineering + developer experience audit: project structure, tooling, CLI, SDK, documentation, CI/CD, dev environment |
| **Technique** | Multi-dimensional DX assessment: reviewed 40+ doc files, Makefile, CI config, CLI commands, SDK source, web console, ADRs, and cross-referenced findings from cosca-infrastructure and cosca-provider audits |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #platform #audit #dx #tooling #cli #sdk #documentation #devops |
| **Related** | cosca-infrastructure audit (W5), cosca-provider learnings (W5), docs/, Makefile, .golangci.yml, .goreleaser.yaml, .github/workflows/ |
| **Learned** | 1) Cosca has a strong multi-interface platform (CLI 46 commands, TypeScript SDK v1.1.0, Web Console 21+ pages, REST API 36 endpoints, gRPC). 2) Platform Eng maturity score: 3.3/5 — moderate with critical gaps in community readiness, Go SDK, deployment automation, and dev environment commodity. 3) Documentation depth is excellent (40+ doc files, 7 ADRs) but root-level CONTRIBUTING.md and ARCHITECTURE.md are missing — a critical gap for open-source. 4) CI/CD pipeline has 7 functional CI gates but zero CD — no automated deployment, no push to registry, no soak testing. 5) Go SDK is only type exports (pkg/cosca/) with no HTTP client — aspirational status confirmed by docs/sdk/go.md. 6) devcontainer and pre-commit hooks are completely absent — developers must manually install Go, Node, pnpm. 7) TypeScript SDK is production-ready but type regeneration is manual. 8) Makefile is comprehensive (354 lines, 25 targets) but missing setup, security-check, and docker-dev targets. 9) Coverage at 46.5% with CI threshold at 55% — documentation states 80%+ but this is aspirational. 10) No Dependabot/Renovate, no CODEOWNERS, no issue/PR templates. |
| **Next** | Address P0 items: CONTRIBUTING.md, ARCHITECTURE.md, devcontainer, CODEOWNERS, issue templates; then P1: make setup, pre-commit hooks, Dependabot, CI gates for TS SDK and Web |

### 2026-07-28 — Gap: DX Scorecard Metrics Not Instrumented
| Field | Value |
|-------|-------|
| **Agent** | cosca-platform |
| **Task** | Attempted to measure onboarding time, build time, test run time with empirical precision |
| **Technique** | Instrumentation gap analysis — checked for existing telemetry/metrics that could provide DX baselines |
| **Level** | 2 |
| **Outcome** | partial |
| **Tags** | #dx #metrics #telemetry #gap #instrumentation |
| **Related** | internal/metrics/, internal/telemetry/, Prometheus /metrics endpoint |
| **Learned** | Current Prometheus metrics cover runtime health and HTTP request metrics but do NOT capture DX-specific indicators: no build duration tracking, no test suite timing aggregation, no onboarding step timing, no CLI command latency percentiles. This means DX scorecard was based on manual estimation rather than measured data. The platform needs an "Engineering Metrics" pipeline to track: CI pipeline duration, local build time, test suite runtime trends, time-to-first-build for new contributors. |
| **Next** | Propose DX instrumentation spec: add build-time tracking to Makefile (wrap build with timer), test-suite duration aggregation in CI (store historical data), onboarding-time analytics via optional telemetry during `cosca install` |

### 2026-07-28 — Pattern: Multi-Interface Platform Architecture
| Field | Value |
|-------|-------|
| **Agent** | cosca-platform |
| **Task** | Analyze platform architecture patterns — CLI vs SDK vs REST API vs gRPC vs Web Console |
| **Technique** | Interface coverage matrix — mapped each subsystem's availability across all 5 platform interfaces |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #architecture #multi-interface #cli #sdk #rest #grpc #web |
| **Related** | internal/cli/, sdk/typescript/src/, api/rest/, api/grpc/, web/src/ |
| **Learned** | Cosca exposes its platform through 5 interfaces with varying completeness: CLI (46 commands, most complete — covers all subsystems), REST API (36 endpoints, 10 domains — core subsystems covered), gRPC (3 services — Knowledge, Memory, Runtime — limited subset), TypeScript SDK (12 modules — mirrors REST API coverage), Web Console (21+ pages — mirrors REST API with admin extras). The architecture follows a "build once, expose many" pattern where the internal Go subsystem layer is wrapped by CLI, which then exposes business logic through REST/gRPC handlers, consumed by TS SDK and Web Console. This is a strong architectural pattern. The missing link is: Go SDK (to complete the loop for Go-first consumers) and gRPC completeness (to match REST API coverage). |
| **Next** | Document the multi-interface pattern as ADR-PLAT-003; prioritize Go SDK implementation to complete the 5-interface coverage matrix |

### 2026-07-28 — Inconsistency: Documentation States 80% Coverage, Reality is 46.5%
| Field | Value |
|-------|-------|
| **Agent** | cosca-platform |
| **Task** | Cross-reference documentation claims with actual metrics |
| **Technique** | Document-driven verification — read all developer docs and compared stated thresholds with CI configuration and actual measurements |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #documentation #inconsistency #coverage #qa |
| **Related** | docs/developer-guide/testing.md, docs/developer-guide/getting-started.md, .github/workflows/ci.yml |
| **Learned** | Multiple documentation files state "80%+ line coverage" as requirement (getting-started.md L285, testing.md L36, L427-439). However, CI gate G5 uses threshold of 55% (ratcheted from 70% after acknowledgment that target was unrealistic), and current actual coverage is 46.5% (per platform overview doc). This 80%→46.5% gap is a significant trust issue for developer documentation. Either the docs need updating to reflect reality, or a plan needs to exist to bridge to 60%+ (stated as v2.0 goal in platform overview). The testing.md per-package targets (75-90%) are also aspirational with no enforcement. |
| **Next** | Recommend documentation update to state current reality (46.5%) with roadmap to 60%; add per-package coverage tracking in CI |

### 2026-07-28 — Discovery: Code Generation Pipeline is Incomplete
| Field | Value |
|-------|-------|
| **Agent** | cosca-platform |
| **Task** | Audit code generation and automation infrastructure |
| **Technique** | Generation pipeline analysis — traced all code generation paths: proto → Go, embed sync, OpenAPI → TypeScript, and checked for missing links |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #codegen #automation #protobuf #openapi #mockgen |
| **Related** | Makefile (proto, embed-sync targets), sdk/typescript/package.json (generate:types), api/rest/openapi.yaml |
| **Learned** | Three generation pipelines exist: (1) Proto → Go via protoc (make proto) — functional but fragile (requires protoc + plugins installed). (2) OpenAPI → TypeScript SDK types via openapi-typescript (manual only). (3) Embed sync via rsync (make embed-sync) — functional with DRY_RUN support. Missing pipelines: (a) mock generation from Go interfaces (mockgen is listed in Makefile variables but never used in targets), (b) Go client generation from OpenAPI spec, (c) stringer generation for iota enums, (d) automated sqlc/ent codegen for database layer. The `go generate` directive is not used anywhere in the codebase. |
| **Next** | Add `//go:generate` directives for mockgen, stringer; create `make generate` aggregation target; document the generation pipeline |

### 2026-07-28 — Cross-Agent Insight: Infrastructure + Platform + Provider Gaps Form a Cohesive Story
| Field | Value |
|-------|-------|
| **Agent** | cosca-platform |
| **Task** | Synthesize findings across cosca-infrastructure, cosca-provider, and cosca-platform audits |
| **Technique** | Cross-audit gap correlation — identified overlapping findings that amplify each other |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #synthesis #cross-audit #gap-correlation |
| **Related** | cosca-infrastructure/audit-report-2026-07-28.md, cosca-provider/learnings.md |
| **Learned** | Three audits reveal a cohesive gap story: Infrastructure says CI has no CD stage and no soak testing → Platform says dev environment has no pre-commit hooks and no Dependabot → Provider says circuit breaker and failover are documented but not implemented. Together these mean: (1) Code reaches production without automated guardrails (no CD, no soak, no circuit breaker). (2) Developer workflow has manual friction points (no pre-commit, no automated deps, no devcontainer). (3) Runtime resilience is documented but not real (circuit breaker, failover). The platform layer is the integration point — it must work with infrastructure to automate deployment safety, and with provider to enforce reliability contracts. |
| **Next** | Plan platform-led initiative: "Reliable Delivery Pipeline" — coordinate with cosca-devops, cosca-infrastructure, and cosca-provider to close CD + soak + circuit-breaker gaps in one sprint |

## Context7 DX Mining (2026-08-27) — Level 3
### 2026-08-27 — Pattern: Agent-Aware "One-Command Setup" + Trigger-by-Description Skills
| Field | Value |
|-------|-------|
| **Agent** | cosca-platform |
| **Task** | Deep mine Context7 (Upstash) for DX/CLI/onboarding patterns: `npx ctx7 setup`, per-editor mapping (claude/cursor/opencode/codex/antigravity/gemini), setup/ directory, SKILL.md frontmatter auto-trigger, rules/, OpenCode plugin |
| **Technique** | Federated DX mining — reverse-engineered 5 subsystems: (1) declarative AgentConfig registry in setup/agents.ts mapping each editor to mcp paths + configKey + buildEntry + rule(kind file/append) + skill dir + detect paths; (2) idempotent writers (installRule section-marker replace / mcp-writer mergeServerEntry) so setup is re-runnable; (3) auto-detection of installed editors (detectAgents probes filesystem), zero-config handoff; (4) device-flow OAuth polling w/ RFC8628 backoff (slow_down/transient→+5s) + auto API-key mint via POST /api/dashboard/api-keys; (5) SKILL.md frontmatter `description` written as imperative trigger policy with few-shot named examples (React/Next.js/Prisma) + "use even when you think you know the answer" anti-anchor; rules/ hold the negative space ("Do not use for: ...") in prose, skills/models hold the positive steps |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #dx #cli #onboarding #setup #skills #trigger-by-description #activetrigger #opencode #mcp #oAuth #deviceflow |
| **Related** | packages/cli/src/setup/agents.ts, packages/cli/src/commands/setup.ts, skills/find-docs/SKILL.md, rules/context7-cli.md, packages/opencode/src/index.ts |
| **Learned** | Context7's adoption engine = ONE `npx ctx7 setup`. 6 agents declared once; auto-detected by probing their config dirs. Writes 3 artifact types per agent (MCP entry + rule + skill). Two runtime modes (MCP vs CLI+Skills) chosen by one flag. Idempotent via HTML-comment section markers (`<!-- context7 -->`) in AGENTS.md and `??=`/mergeServerEntry in JSON/TOML. Rule=trigger policy in prose w/ "Do not use" negative space; Skill=procedural steps; for Cursor, rule gets `alwaysApply: true` YAML frontmatter so it kicks in unconditionally. OpenCode: plugin mutates config.mcp + appends skills.path to a bundled SKILL.md (additive, `??=`, user config wins). Cosca gap: agents have role-description (not trigger), skills have NO `name`/`description` YAML frontmatter AT ALL (start at `# TITLE`), and the CLI is manual `cosca skill get` — nothing auto-activates. |
| **Next** | 1) Add YAML frontmatter (name+description) to all ~98 Cosca skills with imperative trigger policy + named-domain few-shots. 2) Add an agent-aware `cosca setup` that discovers `.opencode/` editor dirs, writes an AGENTS.md `<!-- cosca -->` section, and installs a single `cosca-find` skill. 3) Reuse the section-marker idempotent rule-writer. 4) Make MCP/CLI dual-mode via one flag. |


