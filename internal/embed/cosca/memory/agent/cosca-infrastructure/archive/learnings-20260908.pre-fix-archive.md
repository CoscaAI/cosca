# cosca-infrastructure - learnings.md PRE-FIX (conteudo nao-registrado na chain)

> Arquivo gerado em 20260908 antes da reconstrucao do indice de gatilhos.
> Conteudo preservado - leia por grep, nunca inteiro.

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
| **Related** | internal/embed/cosca/memory/codebase/overview.md |
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

### 2026-08-24 — Cofre Vault Blinding (WSL2 + bubblewrap jail)

| Field | Value |
|-------|-------|
| **Agent** | cosca-infrastructure |
| **Task** | Establish WSL2 Linux distro to host the Cofre bwrap jail (isolated, air-gap sandbox) |
| **Technique** | Non-interactive WSL2 provisioning — `wsl --install -d <distro> --no-launch` to bypass OOBE; diagnosing via `wsl -d <distro> -- bash -lc "..."` as root |
| **Level** | 2 (provisioning + verification) |
| **Outcome** | success (partial — distro ready, bwrap not yet installed) |
| **Tags** | #infrastructure #wsl2 #ubuntu #bubblewrap #bwrap #sandbox #cofre #vault #air-gap #security |
| **Related** | learnings.md (this), capability-profile.md |
| **Learned** | 1) WSL core was already enabled — `wsl --version` = 2.7.12, kernel 6.18.33.2-2, `wsl --status` shows "WSL1 não é compatível" (default version 2). 2) `wsl --list --online` shows Ubuntu-24.04 LTS available. 3) `wsl --install -d Ubuntu-24.04 --no-launch` installs fully NON-interactively, NO admin, NO reboot required (VirtualMachinePlatform already enabled). 4) Distro registered as `Stopped`, version 2. 5) CRITICAL: executing `wsl -d Ubuntu-24.04 -- bash -lc "cmd"` runs as ROOT (uid=0) WITHOUIT triggering the interactive OOBE user-creation prompt. 6) Kernel confirmed: `6.18.33.2-microsoft-standard-WSL2` (true WSL2). 7) `bwrap`/bubblewrap NOT installed by default on Ubuntu 24.04.4 LTS — must `sudo apt-get update && sudo apt-get install -y bubblewrap`. 8) Network within WSL resolves OK (DNS_OK, HTTP_OK to archive.ubuntu.com) so apt install will work when authorized. 9) Do NOT elevate or `sudo` on the Don's behalf — authorize only. |
| **Next** | On Don authorization: install bubblewrap inside the distro; harden the bwrap jail (user namespaces, non-root user, bind-mount air-gap for Cofre). Coordinate with cosca-security on jail policy. |

