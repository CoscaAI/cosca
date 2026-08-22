# cosca-devops — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-devops |
| **Task** | Initial capability establishment |
| **Technique** | Standard devops patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #devops #baseline #initialization |
| **Related** | See .opencode/cosca/memory/codebase/overview.md, .opencode/cosca/memory/pattern/ |
| **Learned** | Project established. Core devops patterns documented. Ready for level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-27 — Multi-Resource Helm Chart Productionization
| Field | Value |
|-------|-------|
| **Agent** | cosca-devops |
| **Task** | Production-grade Helm chart for Cosca Enterprise Platform (v1.3.0) |
| **Technique** | Helm chart best practices — named templates, ConfigMap/Secret separation, layered probes, autoscaling, network policies, security contexts |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #helm #kubernetes #productionization #probes #security #autoscaling #ingress |
| **Related** | deploy/helm/cosca/, _helpers.tpl, Kubernetes Pod Lifecycle, HPA v2, NetworkPolicy, PodSecurityContext |
| **Learned** | Key patterns: (1) ConfigMap for non-sensitive env, Secret for sensitive — never mix. (2) startupProbe with high failureThreshold (30) gives WASM/plugin runtime enough warm-up before liveness kicks in. (3) Helm named templates (_helpers.tpl) prevent label drift between Deployment selector, Service selector, and NetworkPolicy podSelector. (4) HPA v2 supports both CPU and memory metrics in a single resource. (5) ingress.hosts[].paths[] pattern with template-determined backend service name avoids hardcoding service names in values. (6) `helm.sh/chart` label auto-derived via `cosca.chart` helper ensures traceability. |
| **Next** | Level 3: Add Helm test pods (helm test), integrate cert-manager annotations, add PodDisruptionBudget, KEDA-based scaling for WASM queue depth, implement schema validation with values.schema.json, Sealed Secrets integration. |
