# cosca-infrastructure — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-infrastructure |
| **Task** | Initial capability establishment |
| **Technique** | Standard infrastructure patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #infrastructure #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core infrastructure patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — Infrastructure Audit (Activation Wave 5)

| Field | Value |
|-------|-------|
| **Agent** | cosca-infrastructure |
| **Task** | Full infrastructure audit: CI/CD, Docker, Terraform, Helm, networking, security, DR, health checks, observability |
| **Technique** | Multi-layer infrastructure audit — cross-referencing CI pipeline gates × deployment artifacts × risk registry × technical debt scorecard |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #infrastructure #audit #ci-cd #docker #terraform #helm #kubernetes #risks #soak-test #dr #security #activation-wave-5 |
| **Related** | RISK_REGISTRY.md, technical-debt/scorecard.md, cognitive-state.md, audit-report-2026-07-28.md |
| **Learned** | 1) Infrastructure maturity is 2.2/5 — functional for dev, not production-ready. 2) CI/CD pipeline is the strongest component (G0-G6 gates, concurrency control, artifact uploads) but missing deployment stage and soak test. 3) Terraform config is minimal — no ALB, TLS, security groups, or auto-scaling. 4) Helm chart is well-structured but all production features disabled by default (ingress, HPA, networkPolicy, TLS). 5) SQLite + ReadWriteOnce PVC blocks horizontal scaling — need Litestream or Postgres migration. 6) R3 (soak test) has 70% probability × high impact — highest infrastructure risk. 7) R9 (Kernel SPOF) is medium-probability but existential if triggered. 8) No DR strategy exists for 408 memory files — a single `rm -rf` could destroy months of agent evolution. 9) Health checks exist in code (startup/liveness/readiness probes in Helm) but Restart() is broken (BUG-U01), making self-healing non-functional. 10) Prometheus /metrics endpoint exists but no Grafana dashboards or Alertmanager config committed. |
| **Next** | Level 3: Implement P0 actions — soak test CI gate (R3), DR backup script (R11), Helm PDB. Cross-reference with cosca-devops (CI/CD) and cosca-security (TLS/secrets) for collaborative implementation. |
