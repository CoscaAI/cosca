# Cosca MASTER INDEX

> **Version**: 1.4.0-dev | **Status**: active | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-29

## Purpose
Single source of truth for all Cosca ecosystem files. Navigate, discover, and understand dependencies.

---

## File Inventory (990+ files)

### KERNEL (1)
| File | Purpose | Dependencies |
|------|---------|-------------|
| [KERNEL.md](KERNEL.md) | Bootstrap orchestrator — discovers context, loads memory, routes work | ORGCHART, QUALITY_GATES, MEMORY_MODEL, all engines |

### GOVERNANCE (7)
| File | Purpose | Dependencies |
|------|---------|-------------|
| [CONSTITUTION.md](CONSTITUTION.md) | **Supreme authority** — 8 immutable principles, chain of command, conflict rules, decision cycle, Don's guarantees | All — referenced by every agent and engine |
| [CONVENTIONS.md](CONVENTIONS.md) | Standard skill contract — every SKILL.md must follow this format | None |
| [GOVERNANCE.md](GOVERNANCE.md) | Versioning, lifecycle, deprecation policies | CONVENTIONS |
| [QUALITY_GATES.md](QUALITY_GATES.md) | Canonical quality gate definitions | None |
| [MEMORY_MODEL.md](MEMORY_MODEL.md) | Canonical memory taxonomy and schema (v4.0.0) | None |
| [HELP.md](HELP.md) | User-facing usage guide | All |
| [COSCA_ENTERPRISE_ARCHITECTURE_AUDIT.md](COSCA_ENTERPRISE_ARCHITECTURE_AUDIT.md) | Complete enterprise architecture audit (2026-07-12) | All |

### COMPANY (1)
| File | Purpose | Dependencies |
|------|---------|-------------|
| [company/ORGCHART.md](company/ORGCHART.md) | Organizational structure, chain of command, decision authority | None |

### DEPARTMENTS (26)
| Department | File | Reports To | Type |
|-----------|------|------------|------|
| CEO | [departments/ceo/SKILL.md](departments/ceo/SKILL.md) | Kernel | Executive |
| CTO | [departments/cto/SKILL.md](departments/cto/SKILL.md) | CEO | Executive |
| Kernel | [departments/kernel/SKILL.md](departments/kernel/SKILL.md) | CEO | Executive |
| Product | [departments/product/SKILL.md](departments/product/SKILL.md) | CEO | Product |
| Architecture | [departments/architecture/SKILL.md](departments/architecture/SKILL.md) | CTO | Architecture |
| Backend | [departments/backend/SKILL.md](departments/backend/SKILL.md) | CTO, Architecture | Engineering |
| Frontend | [departments/frontend/SKILL.md](departments/frontend/SKILL.md) | CTO, Architecture | Engineering |
| Mobile | [departments/mobile/SKILL.md](departments/mobile/SKILL.md) | CTO, Architecture | Engineering |
| Database | [departments/database/SKILL.md](departments/database/SKILL.md) | CTO, Architecture | Engineering |
| DevOps | [departments/devops/SKILL.md](departments/devops/SKILL.md) | CTO | Infrastructure |
| Security | [departments/security/SKILL.md](departments/security/SKILL.md) | CTO | Security |
| Semantic Memory | [departments/semantic-memory/SKILL.md](departments/semantic-memory/SKILL.md) | CTO | Knowledge |
| QA | [departments/qa/SKILL.md](departments/qa/SKILL.md) | CTO | Quality |
| Documentation | [departments/documentation/SKILL.md](departments/documentation/SKILL.md) | CTO | Documentation |
| Review | [departments/review/SKILL.md](departments/review/SKILL.md) | CTO | Quality |
| Testing | [departments/testing/SKILL.md](departments/testing/SKILL.md) | QA | Quality |
| UI/UX | [departments/uiux/SKILL.md](departments/uiux/SKILL.md) | Product | Product |
| Runtime | [departments/runtime/SKILL.md](departments/runtime/SKILL.md) | CTO | Engineering |
| Workflow | [departments/workflow/SKILL.md](departments/workflow/SKILL.md) | CTO | Engineering |
| Analytics | [departments/analytics/SKILL.md](departments/analytics/SKILL.md) | CTO | Specialized |
| AI | [departments/ai/SKILL.md](departments/ai/SKILL.md) | CTO | Specialized |
| Integrations | [departments/integrations/SKILL.md](departments/integrations/SKILL.md) | CTO | Specialized |
| Monitoring | [departments/monitoring/SKILL.md](departments/monitoring/SKILL.md) | CTO | Specialized |
| Release | [departments/release/SKILL.md](departments/release/SKILL.md) | CTO | Specialized |
| Infrastructure | [departments/infrastructure/SKILL.md](departments/infrastructure/SKILL.md) | CTO | Infrastructure |
| Automation | [departments/automation/SKILL.md](departments/automation/SKILL.md) | CTO | Specialized |
| Memory | [departments/memory/SKILL.md](departments/memory/SKILL.md) | CTO | Specialized |
| Context | [departments/context/SKILL.md](departments/context/SKILL.md) | CTO | Specialized |
| Bootstrap | [departments/bootstrap/SKILL.md](departments/bootstrap/SKILL.md) | CTO | Infrastructure |
| Critic | [departments/critic/SKILL.md](departments/critic/SKILL.md) | CEO | Quality |
| Evolution | [departments/evolution/SKILL.md](departments/evolution/SKILL.md) | CTO | Specialized |
| Paradigm | [departments/paradigm/SKILL.md](departments/paradigm/SKILL.md) | CTO, Architecture | Specialized |

### ENGINES (64)
| Engine | File | Activates On |
|--------|------|-------------|
| Context | [engines/context/SKILL.md](engines/context/SKILL.md) | session_start, context_refresh |
| Memory | [engines/memory/SKILL.md](engines/memory/SKILL.md) | session_start, session_end, decision_made |
| Workflow | [engines/workflow/SKILL.md](engines/workflow/SKILL.md) | task_routing, planning |
| Planning | [engines/planning/SKILL.md](engines/planning/SKILL.md) | new_feature, new_bug, refactoring |
| Execution | [engines/execution/SKILL.md](engines/execution/SKILL.md) | task_execution |
| Review | [engines/review/SKILL.md](engines/review/SKILL.md) | task_completed |
| Quality | [engines/quality/SKILL.md](engines/quality/SKILL.md) | review_completed, pre_release |
| Documentation | [engines/documentation/SKILL.md](engines/documentation/SKILL.md) | task_completed, feature_completed, release |
| Wizard | [engines/wizard/SKILL.md](engines/wizard/SKILL.md) | new_feature |
| Discovery | [engines/discovery/WORKSPACE.md](engines/discovery/WORKSPACE.md) | session_start |
| Evolution | [engines/evolution/SKILL.md](engines/evolution/SKILL.md) | periodic, on_demand |
| Templates | [engines/templates/SKILL.md](engines/templates/SKILL.md) | project_init |
| Runtime | [engines/runtime/SKILL.md](engines/runtime/SKILL.md) | application_lifecycle |
| Skills | [engines/skills/SKILL.md](engines/skills/SKILL.md) | skill_loading, skill_routing |
| Tools | [engines/tools/SKILL.md](engines/tools/SKILL.md) | agent_execution |
| Audit | [engines/audit/SKILL.md](engines/audit/SKILL.md) | all_actions |
| Observability | [engines/observability/SKILL.md](engines/observability/SKILL.md) | continuous |
| Learning | [engines/learning/SKILL.md](engines/learning/SKILL.md) | session_end, periodic |
| Capability | [engines/capability/SKILL.md](engines/capability/SKILL.md) | agent_spawn, level_up, on_demand |
| Compliance | [engines/compliance/SKILL.md](engines/compliance/SKILL.md) | audit_request, policy_check |
| Feature Flags | [engines/feature-flags/SKILL.md](engines/feature-flags/SKILL.md) | deploy, config_change |
| Identity | [engines/identity/SKILL.md](engines/identity/SKILL.md) | agent_spawn, session_start |
| Knowledge | [engines/knowledge/SKILL.md](engines/knowledge/SKILL.md) | search_request, index_update |
| Recovery | [engines/recovery/SKILL.md](engines/recovery/SKILL.md) | error_detected, health_check_fail |
| Scheduler | [engines/scheduler/SKILL.md](engines/scheduler/SKILL.md) | periodic, cron_trigger |
| Validation | [engines/validation/SKILL.md](engines/validation/SKILL.md) | pre_commit, pre_deploy, quality_gate |
| Secrets | [engines/secrets/SKILL.md](engines/secrets/SKILL.md) | agent_spawn, pre_commit, on_rotation |
| Semantic Memory | [engines/semantic-memory/SKILL.md](engines/semantic-memory/SKILL.md) | semantic_search, cross_agent_discovery, embedding_index |
| Policy | [engines/policy/SKILL.md](engines/policy/SKILL.md) | decision_point, quality_gate, agent_action |
| Benchmark | [engines/benchmark/SKILL.md](engines/benchmark/SKILL.md) | scheduled, on_demand, before_release |
| **Cognitive Economy ★** | [engines/cognitive-economy/SKILL.md](engines/cognitive-economy/SKILL.md) | **★ F2.1 STAR**: post_task, roi_calculation, pre_task_efficiency |
| **Evidence** | [engines/evidence/CONFIDENCE_MODEL.md](engines/evidence/CONFIDENCE_MODEL.md) | **NEW v3.0**: information_conflict, decision_point |
| **Memory Curation** | [engines/memory-curation/MEMORY_CURATION_ENGINE.md](engines/memory-curation/MEMORY_CURATION_ENGINE.md) | **NEW v3.0**: every_50_entries, weekly, on_demand |
| **Context Compression** | [engines/context-compression/CONTEXT_COMPRESSION_ENGINE.md](engines/context-compression/CONTEXT_COMPRESSION_ENGINE.md) | **NEW v1.4.0**: session_end, pre_commit, weekly |
| Resource Resolver | [engines/resource-resolver/SKILL.md](engines/resource-resolver/SKILL.md) | bootstrap, path_request |
| **Mental Energy ★** | [engines/mental-energy/SKILL.md](engines/mental-energy/SKILL.md) | **★ F3.5 v2.0.0**: continuous_pipeline (< 10ms), task_cost, hourly_regen, fadiga_score, recovery_trigger |
| **Decision Replay** | [engines/decision-replay/SKILL.md](engines/decision-replay/SKILL.md) | **NEW F7.5**: don_query, decision_review, post_action_audit |
| **Contradiction ★** | [engines/contradiction/SKILL.md](engines/contradiction/SKILL.md) | **NEW F8.3**: pre_decision, wisdom_decay_contradiction, scheduled_daily |
| **Wisdom Distillation** | [engines/wisdom-distillation/SKILL.md](engines/wisdom-distillation/SKILL.md) | **NEW F9.2**: post_experience_compile, weekly, on_demand, pre_constitutional_amendment |
| **Federation** | [engines/federation/SKILL.md](engines/federation/SKILL.md) | **NEW F2.3**: publish (Don marcou #global-ready), subscribe (busca local insuficiente), semantic_search_fallback |
| **Discovery ★** | [engines/discovery/SKILL.md](engines/discovery/SKILL.md) | **NEW F10.3**: sprint_end, post_N_tasks, on_demand (serendipity mining) |
| **Org Memory** | [engines/org-memory/SKILL.md](engines/org-memory/SKILL.md) | **NEW F10.4**: post_sprint, weekly, collaboration_pattern_detection, process_rule_recommendation |
| **Insight Generator ★** | [engines/insight-generator/SKILL.md](engines/insight-generator/SKILL.md) | **NEW F3.1**: weekly (segunda 07:00), post_N_tasks, on_demand, post_sprint (correlação de séries temporais) |
| **Cognitive Compression** | [engines/cognitive-compression/SKILL.md](engines/cognitive-compression/SKILL.md) | **NEW F3.3**: weekly (segunda 08:00), on_demand, entropy_spike, don_command |
| **Adaptive Personality ★** | [engines/adaptive-personality/SKILL.md](engines/adaptive-personality/SKILL.md) | **★ F3.4 v2.0.0**: pre_task (< 50ms), 5 eixos, 5 modos (Cirúrgico/Exploratório/Sprint/Cauteloso/Zen), don_preference, moving_average_5, P0_always_cauteloso |

### BOOTSTRAP (1)
| File | Purpose | Dependencies |
|------|---------|-------------|
| [bootstrap/BOOTSTRAP.md](bootstrap/BOOTSTRAP.md) | 10-phase automatic workspace initialization | bootstrap-config.yaml, detectors/, validators/, QUALITY_GATES |

### WORKFLOWS (39)
| Workflow | File | Category | Steps | Status |
|----------|------|----------|-------|--------|
| project-init | [workflows/project-init.md](workflows/project-init.md) | init | 9 | active |
| feature-development | [workflows/feature-development.md](workflows/feature-development.md) | feature | 16 | active |
| bug-fix | [workflows/bug-fix.md](workflows/bug-fix.md) | bug | 8 | active |
| refactoring | [workflows/refactoring.md](workflows/refactoring.md) | refactor | 7 | active |
| code-review | [workflows/code-review.md](workflows/code-review.md) | review | 8 | active |
| release | [workflows/release.md](workflows/release.md) | deploy | 8 | active |
| deployment | [workflows/deployment.md](workflows/deployment.md) | deploy | 7 | active |
| dependency-update | [workflows/dependency-update.md](workflows/dependency-update.md) | maintenance | 7 | active |
| architecture-review-board | [workflows/architecture-review-board.md](workflows/architecture-review-board.md) | review | 6 | active |
| capacity-planning | [workflows/capacity-planning.md](workflows/capacity-planning.md) | planning | 7 | active |
| chaos-testing | [workflows/chaos-testing.md](workflows/chaos-testing.md) | testing | 7 | active |
| compliance-audit | [workflows/compliance-audit.md](workflows/compliance-audit.md) | governance | 6 | active |
| data-privacy-impact | [workflows/data-privacy-impact.md](workflows/data-privacy-impact.md) | governance | 6 | active |
| dependency-upgrade | [workflows/dependency-upgrade.md](workflows/dependency-upgrade.md) | maintenance | 7 | active |
| disaster-recovery | [workflows/disaster-recovery.md](workflows/disaster-recovery.md) | resilience | 7 | active |
| incident-response | [workflows/incident-response.md](workflows/incident-response.md) | resilience | 6 | active |
| migration-execution | [workflows/migration-execution.md](workflows/migration-execution.md) | data | 7 | active |
| performance-optimization | [workflows/performance-optimization.md](workflows/performance-optimization.md) | performance | 7 | active |
| platform-bootstrap | [workflows/platform-bootstrap.md](workflows/platform-bootstrap.md) | platform | 7 | active |
| provider-migration | [workflows/provider-migration.md](workflows/provider-migration.md) | provider | 7 | active |
| secrets-rotation | [workflows/secrets-rotation.md](workflows/secrets-rotation.md) | security | 5 | active |
| technical-debt-paydown | [workflows/technical-debt-paydown.md](workflows/technical-debt-paydown.md) | maintenance | 6 | active |
| security-audit | [workflows/security-audit.md](workflows/security-audit.md) | security | 8 | active |
| performance-audit | [workflows/performance-audit.md](workflows/performance-audit.md) | performance | 8 | active |
| **metacognition-pipeline** | [workflows/metacognition-pipeline.md](workflows/metacognition-pipeline.md) | **meta** | **8** | **NEW v3.0** |
| **cosca-evolution-autonomy** | [workflows/cosca-evolution-autonomy.md](workflows/cosca-evolution-autonomy.md) | **evolution** | **5 phases** | **NEW v3.0** |
| **api-design-review** | [workflows/api-design-review.md](workflows/api-design-review.md) | review | 6 | active |
| **audit-runtime-sync** | [workflows/audit-runtime-sync.md](workflows/audit-runtime-sync.md) | **audit** | **6** | **NEW v1.4.0** |
| cognitive-audit-loop | [workflows/cognitive-audit-loop.md](workflows/cognitive-audit-loop.md) | cognition | 8 | active |
| cognitive-maturity-implementation | [workflows/cognitive-maturity-implementation.md](workflows/cognitive-maturity-implementation.md) | cognition | 7 | active |
| cognitive-phylogeny | [workflows/cognitive-phylogeny.md](workflows/cognitive-phylogeny.md) | cognition | 6 | active |
| contrafactual-gate | [workflows/contrafactual-gate.md](workflows/contrafactual-gate.md) | cognition | 5 | active |
| cosca-jail-autoexec | [workflows/cosca-jail-autoexec.md](workflows/cosca-jail-autoexec.md) | runtime | 6 | active |
| knowledge-pipeline | [workflows/knowledge-pipeline.md](workflows/knowledge-pipeline.md) | knowledge | 7 | active |
| knowledge-pipeline-f2-f3 | [workflows/knowledge-pipeline-f2-f3.md](workflows/knowledge-pipeline-f2-f3.md) | knowledge | 6 | active |
| memorize-commit | [workflows/memorize-commit.md](workflows/memorize-commit.md) | memory | 4 | active |
| next-gen-cli-implementation | [workflows/next-gen-cli-implementation.md](workflows/next-gen-cli-implementation.md) | cli | 7 | active |
| org-memory | [workflows/org-memory.md](workflows/org-memory.md) | memory | 5 | active |
| session-reception | [workflows/session-reception.md](workflows/session-reception.md) | session | 4 | active |

### METRICS (1)
| File | Purpose | Dependencies |
|------|---------|-------------|
| [metrics/platform-health-dashboard.md](metrics/platform-health-dashboard.md) | **NEW v3.0** — Platform self-measurement: 5 key metrics + 10 secondary, weekly dashboard, auto-alerts | CONFIDENCE_MODEL, MEMORY_CURATION_ENGINE, capability profiles |

### MEMORY STORES (9)
| Store | Index File | Scope | Location |
|-------|-----------|-------|----------|
| Short | [memory/short/INDEX.md](memory/short/INDEX.md) | Session | `.cosca/memory/short/` |
| Long | [memory/long/INDEX.md](memory/long/INDEX.md) | Permanent | `.cosca/memory/long/` |
| Project | [memory/project/INDEX.md](memory/project/INDEX.md) | Project | `.cosca/memory/project/` |
| Architecture | [memory/architecture/INDEX.md](memory/architecture/INDEX.md) | System | `.cosca/memory/architecture/` |
| Decision | [memory/decision/INDEX.md](memory/decision/INDEX.md) | Permanent | `.cosca/memory/decision/` |
| Pattern | [memory/pattern/INDEX.md](memory/pattern/INDEX.md) | Cross-project | `${MEMORY_GLOBAL}/pattern/` |
| Bug | [knowledge/failures/bugs/INDEX.md](knowledge/failures/bugs/INDEX.md) | Cross-project | `${MEMORY_GLOBAL}/bug/` |
| Agent | [memory/agent/INDEX.md](memory/agent/INDEX.md) | Cross-project | `${MEMORY_GLOBAL}/agent/` |
| Audit | [memory/audit/](memory/audit/) | Permanent | `internal/embed/cosca/memory/audit/` |

### CONTRACTS (2)
| File | Purpose |
|------|---------|
| [RUNTIME_CONTRACT.md](RUNTIME_CONTRACT.md) | Formal interface between Cosca Kernel and Runtime execution environment |
| [PROVIDER_INTERFACE.md](PROVIDER_INTERFACE.md) | Standardized interface for AI model providers with failover |

### TEMPLATES (14)
| Template | File | Recommended Stack |
|----------|------|------------------|
| ERP | [templates/erp/TEMPLATE.md](templates/erp/TEMPLATE.md) | NestJS/Django + React/Angular + PostgreSQL |
| CRM | [templates/crm/TEMPLATE.md](templates/crm/TEMPLATE.md) | FastAPI/Express + React/Vue + PostgreSQL |
| SaaS | [templates/saas/TEMPLATE.md](templates/saas/TEMPLATE.md) | Next.js + NestJS + PostgreSQL + Stripe |
| Marketplace | [templates/marketplace/TEMPLATE.md](templates/marketplace/TEMPLATE.md) | NestJS/Django + Next.js + Elasticsearch |
| API | [templates/api/TEMPLATE.md](templates/api/TEMPLATE.md) | NestJS/FastAPI/Go/Rust |
| Microservices | [templates/microservices/TEMPLATE.md](templates/microservices/TEMPLATE.md) | Polyglot + RabbitMQ/Kafka + K8s |
| Landing | [templates/landing/TEMPLATE.md](templates/landing/TEMPLATE.md) | Next.js/Astro + Tailwind |
| Admin | [templates/admin/TEMPLATE.md](templates/admin/TEMPLATE.md) | React + Ant Design/MUI |

### CONFIG (3)
| File | Purpose |
|------|---------|
| [cosca.config.yaml](cosca.config.yaml) | Centralized path overrides, resolver settings, bootstrap flags |
| [templates/scaffold/config.yml](templates/scaffold/config.yml) | Project scaffold configuration template |
| [templates/scaffold/state.yml](templates/scaffold/state.yml) | Project state tracking template |

---

## Cross-Reference Map

### Kernel Initialization Flow
```
KERNEL.md
  → company/ORGCHART.md           (org structure)
  → engines/discovery/WORKSPACE.md (workspace scan)
  → engines/resource-resolver/SKILL.md (path resolution)
  → engines/context/SKILL.md       (build context)
  → engines/memory/SKILL.md        (load memories)
  → QUALITY_GATES.md               (enforce gates)
  → MEMORY_MODEL.md                (memory taxonomy)
```

### Feature Development Flow
```
KERNEL.md
  → departments/ceo/SKILL.md
  → departments/product/SKILL.md
    → engines/wizard/SKILL.md
  → departments/cto/SKILL.md
    → engines/planning/SKILL.md
      → engines/workflow/SKILL.md
        → workflows/feature-development.md
          → departments/architecture/SKILL.md
          → departments/backend/SKILL.md
          → departments/frontend/SKILL.md
          → departments/mobile/SKILL.md
          → departments/database/SKILL.md
          → departments/testing/SKILL.md
          → departments/review/SKILL.md
            → engines/review/SKILL.md
          → departments/qa/SKILL.md
            → engines/quality/SKILL.md
          → departments/documentation/SKILL.md
            → engines/documentation/SKILL.md
```

### Quality Gate Flow
```
QUALITY_GATES.md (canonical definition)
  → engines/quality/SKILL.md       (enforcement)
  → engines/review/SKILL.md        (review checks)
  → engines/audit/SKILL.md         (audit trail)
  → engines/observability/SKILL.md (metrics)
```

### Memory Flow
```
MEMORY_MODEL.md (canonical definition)
  → engines/memory/SKILL.md        (operations)
  → departments/memory/SKILL.md    (management)
  → departments/context/SKILL.md   (context integration)
  → engines/learning/SKILL.md      (learning storage)
  → engines/evolution/SKILL.md     (pattern storage)
```

---

## Quick Reference

### /commands Index
| Command | Target |
|---------|--------|
| `/help-cosca` | HELP.md |
| `/init` | KERNEL.md + workflows/project-init.md |
| `/feature` | KERNEL.md + engines/wizard/SKILL.md |
| `/fix` | KERNEL.md + workflows/bug-fix.md |
| `/refactor` | KERNEL.md + workflows/refactoring.md |
| `/review` | KERNEL.md + workflows/code-review.md |
| `/deploy` | KERNEL.md + departments/release/SKILL.md |
| `/plan` | KERNEL.md + engines/planning/SKILL.md |
| `/docs` | KERNEL.md + engines/documentation/SKILL.md |
| `/status` | KERNEL.md (summary) |
| `/evolve` | KERNEL.md + engines/evolution/SKILL.md |

### Governance Documents
| File | Covers |
|------|--------|
| [CONVENTIONS.md](CONVENTIONS.md) | Skill file format, sections, naming |
| [GOVERNANCE.md](GOVERNANCE.md) | Versions, lifecycle, deprecation, ownership |
| [QUALITY_GATES.md](QUALITY_GATES.md) | Gate definitions, metrics, thresholds |
| [MEMORY_MODEL.md](MEMORY_MODEL.md) | Memory types, schemas, storage locations |
| [COSCA_ENTERPRISE_ARCHITECTURE_AUDIT.md](COSCA_ENTERPRISE_ARCHITECTURE_AUDIT.md) | Enterprise readiness audit and roadmap |

### Enterprise Contracts
| File | Covers |
|------|--------|
| [RUNTIME_CONTRACT.md](RUNTIME_CONTRACT.md) | Kernel ↔ Runtime interface |
| [PROVIDER_INTERFACE.md](PROVIDER_INTERFACE.md) | AI provider abstraction with failover |

---

> **Maintained by**: Cosca Kernel | **Regenerate on**: Every skill addition/removal

### COUNCILS (12)
| Council | File | Chair | Meets |
|---------|------|-------|-------|
| All Councils | [councils/COUNCILS.md](councils/COUNCILS.md) | — | — |
| Executive | — | CEO | Weekly |
| Architecture | — | Architecture Chief | Bi-weekly |
| Security | — | Security Chief | Weekly |
| Quality | — | QA Chief | Weekly |
| AI | — | AI Chief | Bi-weekly |
| Infrastructure | — | Infrastructure Chief | Bi-weekly |
| Platform | — | CTO | Monthly |
| Data | — | Database Chief | Monthly |
| Product | — | Product Chief | Weekly |
| Governance | — | CEO | Monthly |
| Innovation | — | CTO | Monthly |
| Research | — | AI Chief | Quarterly |

### CAPABILITIES (64)
| File | Purpose |
|------|---------|
| [capabilities/CAPABILITY_CATALOG.md](capabilities/CAPABILITY_CATALOG.md) | Complete catalog — 64 capabilities in 12 categories |
| [capabilities/CAPABILITY_TEMPLATE.md](capabilities/CAPABILITY_TEMPLATE.md) | Standard capability contract (12 sections) |

### KNOWLEDGE DOMAIN (8 stores)
| Store | Index | Purpose |
|-------|-------|---------|
| Heuristics | [knowledge/heuristics/INDEX.yaml](knowledge/heuristics/INDEX.yaml) | 20 heuristics across 8 domains — extracted from agent learnings |
| Patterns | [knowledge/patterns/INDEX.md](knowledge/patterns/INDEX.md) | Architecture, design, code, testing, security patterns |
| Playbooks | [knowledge/best-practices/playbooks/INDEX.md](knowledge/best-practices/playbooks/INDEX.md) | Step-by-step guides for common scenarios |
| Runbooks | [knowledge/best-practices/runbooks/INDEX.md](knowledge/best-practices/runbooks/INDEX.md) | Operational procedures |
| Incidents | [knowledge/failures/incidents/INDEX.md](knowledge/failures/incidents/INDEX.md) | Incident reports and post-mortems |
| Benchmarks | [knowledge/best-practices/benchmarks/INDEX.md](knowledge/best-practices/benchmarks/INDEX.md) | Agent and provider performance data |
| Schema | [knowledge/schema/INDEX.md](knowledge/schema/INDEX.md) | KB versioning: V001 migration, schema_version, migrate.sh |
| Reference Architectures | [knowledge/architecture/adr/](knowledge/architecture/adr/) | Architecture Decision Records |

### ENTERPRISE DOCUMENTS (5)
| File | Purpose |
|------|---------|
| [SECURITY_ARCHITECTURE.md](SECURITY_ARCHITECTURE.md) | Complete cybersecurity framework — 8 domains, Zero Trust, OWASP, supply chain, AI security, incident response |
| [AGENT_DNA.md](AGENT_DNA.md) | Standardized 28-field agent contract (v3.0) |
| [ENTERPRISE_REDUNDANCY.md](ENTERPRISE_REDUNDANCY.md) | 6-layer redundancy matrix with RTO/RPO |
| [COSCA_ENTERPRISE_ARCHITECTURE_AUDIT.md](COSCA_ENTERPRISE_ARCHITECTURE_AUDIT.md) | Complete architecture audit (v1.0 baseline) |
| [COSCA_ENTERPRISE_EVOLUTION.md](COSCA_ENTERPRISE_EVOLUTION.md) | Full evolution roadmap and platform spec (v2.0) |

---

## v3.0 — ENTERPRISE EVOLUTION (2026-07-23)

### NEW DEPARTMENTS (14)
| Department | File | Reports To | Type |
|-----------|------|------------|------|
| API Chief | [departments/api/SKILL.md](departments/api/SKILL.md) | CTO, Architecture | Engineering |
| Performance Chief | [departments/performance/SKILL.md](departments/performance/SKILL.md) | CTO | Engineering |
| Platform Chief | [departments/platform/SKILL.md](departments/platform/SKILL.md) | CTO | Platform |
| Compliance Chief | [departments/compliance/SKILL.md](departments/compliance/SKILL.md) | CTO, CEO | Governance |
| Plugin Chief | [departments/plugin/SKILL.md](departments/plugin/SKILL.md) | CTO, Platform | Platform |
| Migration Chief | [departments/migration/SKILL.md](departments/migration/SKILL.md) | CTO, Architecture | Engineering |
| Provider Chief | [departments/provider/SKILL.md](departments/provider/SKILL.md) | CTO | Infrastructure |
| Governance Chief | [departments/governance/SKILL.md](departments/governance/SKILL.md) | CEO, CTO | Governance |
| Cache Chief | [departments/cache/SKILL.md](departments/cache/SKILL.md) | CTO, Architecture | Engineering |
| Messaging Chief | [departments/messaging/SKILL.md](departments/messaging/SKILL.md) | CTO, Architecture | Engineering |
| CLI Chief | [departments/cli/SKILL.md](departments/cli/SKILL.md) | CTO, Platform | Platform |
| SDK Chief | [departments/sdk/SKILL.md](departments/sdk/SKILL.md) | CTO, Platform | Platform |
| Discovery Chief | [departments/discovery/SKILL.md](departments/discovery/SKILL.md) | CTO, Architecture | Engineering |
| Technical Debt Chief | [departments/technical-debt/SKILL.md](departments/technical-debt/SKILL.md) | CTO | Quality |

### SPECIALIST DEPARTMENTS (9)
| Department | File | Reports To | Type |
|-----------|------|------------|------|
| Specialist Backend API | [departments/specialist-backend-api/SKILL.md](departments/specialist-backend-api/SKILL.md) | Backend | Specialist |
| Specialist Backend Service | [departments/specialist-backend-service/SKILL.md](departments/specialist-backend-service/SKILL.md) | Backend | Specialist |
| Specialist Database SQL | [departments/specialist-database-sql/SKILL.md](departments/specialist-database-sql/SKILL.md) | Database | Specialist |
| Specialist Documentation Writer | [departments/specialist-documentation-writer/SKILL.md](departments/specialist-documentation-writer/SKILL.md) | Documentation | Specialist |
| Specialist Frontend Component | [departments/specialist-frontend-component/SKILL.md](departments/specialist-frontend-component/SKILL.md) | Frontend | Specialist |
| Specialist Review Code | [departments/specialist-review-code/SKILL.md](departments/specialist-review-code/SKILL.md) | Review | Specialist |
| Specialist Testing E2E | [departments/specialist-testing-e2e/SKILL.md](departments/specialist-testing-e2e/SKILL.md) | Testing | Specialist |
| Specialist Testing Integration | [departments/specialist-testing-integration/SKILL.md](departments/specialist-testing-integration/SKILL.md) | Testing | Specialist |
| Specialist Testing Unit | [departments/specialist-testing-unit/SKILL.md](departments/specialist-testing-unit/SKILL.md) | Testing | Specialist |

### NEW SKILLS (43)
| Category | Count | File |
|----------|-------|------|
| Architecture | 5 | [skills/architecture/](skills/SKILLS_CATALOG.md) |
| Code Quality | 4 | [skills/code-quality/](skills/SKILLS_CATALOG.md) |
| Security | 4 | [skills/security/](skills/SKILLS_CATALOG.md) |
| Performance | 3 | [skills/performance/](skills/SKILLS_CATALOG.md) |
| Testing | 4 | [skills/testing/](skills/SKILLS_CATALOG.md) |
| Documentation | 3 | [skills/documentation/](skills/SKILLS_CATALOG.md) |
| DevOps | 3 | [skills/devops/](skills/SKILLS_CATALOG.md) |
| Data | 3 | [skills/data/](skills/SKILLS_CATALOG.md) |
| AI | 3 | [skills/ai/](skills/SKILLS_CATALOG.md) |
| Governance | 3 | [skills/governance/](skills/SKILLS_CATALOG.md) |
| API | 3 | [skills/api/](skills/SKILLS_CATALOG.md) |
| Platform | 3 | [skills/platform/](skills/SKILLS_CATALOG.md) |
| Reliability | 2 | [skills/reliability/](skills/SKILLS_CATALOG.md) |

### NEW WORKFLOWS (10)
| Workflow | File | Category |
|----------|------|----------|
| API Design Review | [workflows/api-design-review.md](workflows/api-design-review.md) | review |
| Migration Execution | [workflows/migration-execution.md](workflows/migration-execution.md) | migration |
| Compliance Audit | [workflows/compliance-audit.md](workflows/compliance-audit.md) | audit |
| Disaster Recovery | [workflows/disaster-recovery.md](workflows/disaster-recovery.md) | deploy |
| Technical Debt Paydown | [workflows/technical-debt-paydown.md](workflows/technical-debt-paydown.md) | refactor |
| Secrets Rotation | [workflows/secrets-rotation.md](workflows/secrets-rotation.md) | security |
| Provider Migration | [workflows/provider-migration.md](workflows/provider-migration.md) | migration |
| Performance Optimization | [workflows/performance-optimization.md](workflows/performance-optimization.md) | refactor |
| Incident Response | [workflows/incident-response.md](workflows/incident-response.md) | ops |
| Platform Bootstrap | [workflows/platform-bootstrap.md](workflows/platform-bootstrap.md) | init |

### NEW TEMPLATES (5)
| Template | File | Domain |
|----------|------|--------|
| Event-Driven | [templates/event-driven/TEMPLATE.md](templates/event-driven/TEMPLATE.md) | Event-Driven Architecture |
| AI Platform | [templates/ai-platform/TEMPLATE.md](templates/ai-platform/TEMPLATE.md) | AI/ML Platform |
| CLI | [templates/cli/TEMPLATE.md](templates/cli/TEMPLATE.md) | CLI Tool |
| SDK | [templates/sdk/TEMPLATE.md](templates/sdk/TEMPLATE.md) | Client Library |
| Plugin | [templates/plugin/TEMPLATE.md](templates/plugin/TEMPLATE.md) | Plugin Module |

### UPDATED ORGCHART
| File | Change |
|------|--------|
| [company/ORGCHART.md](company/ORGCHART.md) | +14 chiefs, updated chain of command, redundancy matrix, decision authority |

### SUMMARY v3.0
- **Total files**: 155 → **280+**
- **Departments**: 26 → **40**
- **New Skills**: 0 → **43**
- **Workflows**: 10 → **28**
- **Templates**: 9 → **14**
- **New Categories**: Skills framework, Migration workflows, Incident Response, Platform Engineering, Audit

---

### SHARED REFERENCES (2)
| File | Purpose |
|------|---------|
| [shared/AUTO_EVOLUTION_PROTOCOL.md](shared/AUTO_EVOLUTION_PROTOCOL.md) | Canonical auto-evolution protocol for all agents |
| [shared/PROJECT_CONTEXT.md](shared/PROJECT_CONTEXT.md) | Canonical project context for all agents |
