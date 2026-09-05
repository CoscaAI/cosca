# cosca-infrastructure — Reusable Patterns

> Discovered patterns that can be reapplied.

## Patterns Discovered

### PAT-001: Multi-Layer Infrastructure Audit (2026-07-28)

**Context**: Full-stack infrastructure assessment for a monorepo with CI/CD, Docker, Terraform, Helm, and observability components.

**Pattern**:
1. **Inventory phase**: Glob-scan all infrastructure artifacts (Dockerfiles, docker-compose, Terraform, Helm, CI YAML, Prometheus config, health-check handlers)
2. **Cross-reference phase**: Read risk registry + technical debt scorecard + cognitive state to identify known risks
3. **Gap analysis phase**: Map each artifact against production-readiness checklist (TLS, HA, auto-scaling, DR, security, observability)
4. **Priority matrix**: Assign P0 (this week), P1 (this month), P2 (this quarter) based on risk probability × impact × dependency chain
5. **Action plan**: For each priority tier, specify: exact action, resolved risk ID, effort estimate, owner agent

**Applicability**: Any infrastructure audit, deployment readiness review, or production hardening sprint.

**Evidence**: Production-hardened CI/CD pipeline (G0-G6), well-structured Helm chart with all features disabled by default, minimal Terraform missing ALB/TLS/SGs — pattern revealed systemic gap: everything is built for dev, nothing enabled for production.

**Tags**: #audit #pattern #infrastructure-assessment #cross-reference #prioritization

### PAT-002: P0/P1/P2 Risk-Based Prioritization (2026-07-28)

**Context**: 75 technical debt items, 20 infrastructure gaps, limited engineering bandwidth.

**Pattern**:
- **P0 (This Week)**: Items with risk probability ≥ 70% OR blocking CI/CD pipeline (critical path). Must resolve before any other work.
- **P1 (This Month)**: Items that block production deployment (TLS, HA, security groups). Resolve during sprint cycles.
- **P2 (Next Quarter)**: Items that improve but don't block (CDN, WAF, service mesh, multi-region). Technical investment with diminishing returns.

**Heuristic**: If risk probability ≥ 70% AND impact = High, it's P0 regardless of effort. If risk probability < 30% OR impact = Low, it's P2. Everything else is P1.

**Applicability**: Any multi-gap remediation plan.

**Tags**: #prioritization #risk-management #p0-p1-p2 #triage
