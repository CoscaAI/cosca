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

### 2026-08-24 — Windows Jail/Sandbox Platform Analysis
| Field | Value |
|-------|-------|
| **Agent** | cosca-devops |
| **Task** | Analyze Windows isolation for Cosca jail/sandbox (bwrap is Linux-only). Advisory only — no implementation. |
| **Technique** | Cross-platform threat-model mapping; Windows native isolation primitives (Job Objects, AppContainer, Integrity Level/Low-IL, ACL, Windows Sandbox, Windows Containers) vs. bwrap namespaces |
| **Level** | 3 |
| **Outcome** | success |
| **Tags** | #windows #sandbox #jail #security #isolation #job-object #appcontainer #integrity-level #devops |
| **Related** | pkg/cosca/jail_windows.go, jail_linux.go, jail.go; cmd/cosca/main.go (isAdminCommand); internal/chat/sandbox/gate_other.go, gate_linux.go; internal/chat/tool/shell.go; cosca-serve.bat; cosca-service.ps1; Dockerfile |
| **Learned** | (1) NO client bwrap exists on Windows; jail_windows.go just falls back to fail-closed + COSCA_ALLOW_NO_ROOT=1, which is BAKED INTO both start scripts (bat + ps1) — making the explicit opt-in the DEFAULT posture, effectively zero isolation. (2) Real risk is NOT the `serve` daemon (doesn't run agent code); it's the PER-COMMAND agent execution via `shell` tool → `gate_other.go` findBwrap()="" → execWithoutSandbox/execDirect → `cmd /c` UNCONFINED as the interactive user (Scheduled Task principal is `$env:USERNAME` LogonType Interactive, NOT SYSTEM). (3) No single Windows primitive = bwrap; the faithful per-process equivalent is AppContainer, but it's HIGH effort + cmd.exe/powershell.exe PATH/compat edges. (4) Job Objects give process + memory/CPU containment (JOB_OBJECT_LIMIT_ACTIVE_PROCESS, KILL_ON_JOB_CLOSE, PROCESS_MEMORY/JOB_MEMORY) = best ROI; Low-IL token + restrictive ACL prevents WRITE to user profile/system but NOT read/exfil; AppContainer = closest to namespace (FS+network capability); Windows Sandbox = VM, infeasible per-command; Windows Containers (Hyper-V) = high cost but reuses "container=jail" shortcut in IsRunningInContainer(). (5) hardening_other.go is a no-op on Windows (no RLIMIT_CORE/PR_SET_DUMPABLE/PR_SET_NO_NEW_PRIVS) — gap vs Linux. (6) golang.org/x/sys/windows is ALREADY a direct dep (go.mod v0.47.0) so no new dep needed for Job Object/AppContainer/Low-IL. (7) Recommendation: Fase 1 = compose Job Object + Low-IL token + workspace ACL + SetProcessMitigationPolicy on the per-command gate; Fase 2 (only if Don wants true FS/read/network parity) = AppContainer or Windows Container — requires Don/Core approval before code. Also: env allowlist (safeEnv) already prevents credential inheritance into agent subprocesses — keep it. |
| **Next** | If approved: implement Windows Job Object + Low-IL spawner in internal/chat/sandbox/gate_other.go; remove forced COSCA_ALLOW_NO_ROOT=1 from start scripts; add Windows-only tests. Await Don/Core gate decision on Fase 2 (AppContainer vs Windows Container) and on removing the default opt-in. |
