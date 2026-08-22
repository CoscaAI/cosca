# Infrastructure Audit Report — cosca-infrastructure

> **Date**: 2026-07-28 | **Auditor**: cosca-infrastructure (Activation Wave 5)
> **Scope**: CI/CD, containers, cloud, networking, scaling, security, DR
> **Version**: 1.0.0

---

## 1. Current State Assessment

### 1.1 CI/CD Pipeline (`.github/workflows/ci.yml`)

**Status**: 🟢 Functional, multi-gate pipeline v1.4.0-dev

| Gate | Description | Status | Notes |
|------|-------------|--------|-------|
| G0 | Build (`go build ./...`) | ✅ Passing | Ubuntu latest, Go 1.25 |
| G1 | Lint (golangci-lint v2) | ✅ Passing | v2.1 config; format check via gofmt |
| G2 | Vet (`go vet ./...`) | ✅ Passing | Runs in parallel with G0 |
| G3 | Test (`go test -race`) | ✅ Passing | Race detector enabled, 20min timeout |
| G4 | Security (govulncheck + gosec) | ✅ Passing | gosec JSON report uploaded as artifact |
| G5 | Coverage (threshold ≥55%) | ✅ Passing | Ratcheted down from 70% (TODO tracked) |
| G6 | Docs Validator | ✅ Passing | Cross-references doc paths with code |

**Strengths**:
- Concurrency control (`cancel-in-progress: true`)
- Artifact upload on failure for debugging
- Docker build smoke test with version output
- Multi-object cache for Docker builds (gha cache)

**Gaps Identified**:
1. No deployment stage — CI stops at build verification
2. No soak test gate — coverage validated but memory leaks not (R3)
3. No clean-state test — fresh install with empty `.cosca/` not validated (bug-005 regression risk)
4. No multi-platform matrix (Linux-only; Mac/Windows uncovered)
5. Coverage threshold at 55% (target was 70%, downgraded as acknowledged debt)
6. No Dependabot/Renovate for automated dependency updates

### 1.2 Container Strategy (Docker)

**Status**: 🟢 Good multi-stage builds

**Backend Dockerfile** (`Dockerfile`):
- Two-stage: `golang:1.25-alpine` builder → `scratch` runtime
- Static binary (`CGO_ENABLED=0`), stripped (`-s -w`)
- LD flags for version, commit, build date
- Exposes ports 14120 (API) and 14121 (metrics)
- **Score**: Excellent. Scratch base = minimal attack surface.

**Web Dockerfile** (`web/Dockerfile`):
- Three-stage: deps → builder → runner (node:22-alpine)
- Non-root user (`nextjs:1001`)
- Standalone Next.js output
- **Score**: Good. Follows Next.js best practices.

**docker-compose.yml**:
- Two services: cosca + web
- Health check on cosca (30s interval, 3 retries)
- Web depends on cosca with `service_healthy` condition
- Named volume for data persistence
- **Score**: Adequate for local dev. Missing production features (resource limits, restart policies, logging driver).

### 1.3 Terraform IaC (`deploy/terraform/aws/`)

**Status**: 🟡 Minimal, production-incomplete

Components:
- ECS Fargate cluster + task definition + service
- IAM execution role (basic)
- CloudWatch log group (30-day retention)

**Critical Gaps**:
| # | Gap | Impact |
|---|-----|--------|
| TF-01 | No Application Load Balancer | Public IP on ECS task directly — no TLS, no routing, single point of failure |
| TF-02 | No HTTPS/SSL termination | No TLS for API (14120) or web (3000) |
| TF-03 | Default VPC/subnets used | No network isolation, no private subnets |
| TF-04 | No security groups configured | All traffic open to container ports |
| TF-05 | No auto-scaling on ECS service | `desired_count = 1`, no scaling policies |
| TF-06 | No RDS or replicated storage | SQLite single-node only |
| TF-07 | No Route53 / DNS management | No domain configuration |
| TF-08 | No CloudFront CDN | No static asset caching or edge distribution |
| TF-09 | No WAF | No web application firewall |
| TF-10 | Log retention 30 days | Insufficient for compliance/audit |

### 1.4 Kubernetes Helm Chart (`deploy/helm/cosca/`)

**Status**: 🟢 Well-structured, feature-complete templates, production-incomplete defaults

**Chart Quality**: Professional. Templates for: deployment, service, ingress, HPA, network policy, configmap, secret, PVC, service account, helpers.

**Strengths**:
- Startup probe with 300s grace period (WASM/plugin loading)
- Proper liveness + readiness probes
- Security context: non-root user (1000), no privilege escalation, drop ALL capabilities
- Resource requests/limits defined (cosca: 250m-2000m CPU, 256Mi-2Gi RAM; web: 100m-500m CPU, 256Mi-512Mi RAM)
- ConfigMap/Secret separation

**Production Gaps**:
| # | Gap | Impact |
|---|-----|--------|
| K8S-01 | `replicaCount: 1` | No high availability — single pod |
| K8S-02 | `autoscaling.enabled: false` | No dynamic scaling |
| K8S-03 | `networkPolicy.enabled: false` | No pod-to-pod network restrictions |
| K8S-04 | `ingress.enabled: false` | No external traffic routing configured |
| K8S-05 | TLS commented out in ingress | No HTTPS termination |
| K8S-06 | `persistence.accessModes: [ReadWriteOnce]` | SQLite can't scale beyond 1 pod |
| K8S-07 | No PodDisruptionBudget | Voluntary disruptions can cause downtime |
| K8S-08 | No topologySpreadConstraints | No multi-zone distribution |
| K8S-09 | No ServiceMonitor (Prometheus Operator) | No automated metric scraping in K8s |
| K8S-10 | Secrets in plaintext values.yaml | Empty by default but pattern encourages plaintext |

### 1.5 Networking

**Status**: 🔴 Minimal

- Docker: Port mapping only (14120, 14121, 3000)
- Terraform: Public IP on ECS task. No ALB, no private subnets, no security groups.
- Helm: ClusterIP service + optional ingress (disabled). NetworkPolicy optional (disabled).
- No CDN, no WAF, no DDoS protection.
- No service mesh (Istio/Linkerd/Consul).

### 1.6 Observability

**Status**: 🟡 Basic

- Prometheus `/metrics` endpoint exists (port 14121)
- Prometheus config file at `deploy/prometheus.yml` (scrapes localhost)
- Health endpoint `/v1/health` with component health reporting
- Status endpoint `/v1/status` with detailed component status
- SSE streaming endpoint `/v1/status/stream`

**Gaps**:
- No Grafana dashboards committed to repo (mentioned in cognitive state: "33 panels, 25 alerts" but not in deploy/)
- No Alertmanager config
- No distributed tracing (OpenTelemetry)
- No SLO definitions committed as code
- No log aggregation beyond CloudWatch (no Loki/ELK)

### 1.7 Health Checks & Self-Healing

**Status**: 🟡 Partially functional

- Runtime `healthCheckLoop()` polls component health at configured interval
- `/v1/health` returns healthy/degraded status with warnings
- `/v1/status` returns full component breakdown
- **Bug**: Restart() broken (BUG-U01) — daemon watchdog auto-restart non-functional
- **Bug**: EventStartupComplete fires prematurely (BUG-U02) — subscribers get nil state

### 1.8 Disaster Recovery

**Status**: 🔴 None

- No backup strategy for SQLite data or memory files (R11)
- No automated git commit of memory state
- No off-site replication
- No DR runbook
- No RTO/RPO defined

---

## 2. Risk Analysis

### 2.1 R3: No Soak Test (>24h) — 🔴 Critical

**Current State**: Bug-004 (cache leak) fixed in code but never validated with long-running test. Memory leaks, cache leaks, race conditions only manifest in extended sessions.

**Mitigation Plan**:
1. **Immediate (Week 1)**: Add 1-hour soak test to CI pipeline
   - Run server in daemon mode, simulate traffic for 1h
   - Monitor RSS memory every 5 minutes
   - Alert if growth > 10% from baseline
   - Estimated effort: 8h (cosca-devops + cosca-testing)
2. **Short-term (Week 2-4)**: Extend to 24-hour soak test
   - Run overnight on schedule (not per-PR — too slow)
   - Full traffic simulation with multiple concurrent agents
   - Memory + goroutine leak detection
   - Estimated effort: 4h (extend existing test)
3. **Long-term**: Continuous memory profiling in production via pprof endpoint

### 2.2 R9: Kernel Single Point of Failure — 🟡 Medium

**Current State**: Cosca Kernel routes all agent tasks. If kernel routing fails, entire chain breaks. No fallback router.

**Mitigation Plan**:
1. **Short-term**: cosca-critic as secondary opinion on kernel routing decisions (already registered in roadmap B)
2. **Medium-term**: Route health monitoring — detect kernel degradation early
3. **Long-term**: Distributed routing with consensus among multiple chiefs OR simple round-robin fallback

### 2.3 Additional Infrastructure Risks

| # | Risk | Severity | 
|---|------|----------|
| INF-01 | No HA deployment path — single replica everywhere | 🔴 High |
| INF-02 | No TLS termination in any config | 🟠 High |
| INF-03 | SQLite + ReadWriteOnce blocks scale-out | 🟠 High |
| INF-04 | No DR for memory files (R11) | 🟠 High |
| INF-05 | No CI deployment stage (manual deploys only) | 🟡 Medium |
| INF-06 | Secrets management immature (plaintext pattern) | 🟡 Medium |
| INF-07 | Log retention only 30 days (compliance gap) | 🟡 Medium |
| INF-08 | No CDN for static assets | ⚪ Low |
| INF-09 | No WAF | ⚪ Low |
| INF-10 | No multi-platform CI (Linux only) | ⚪ Low |

---

## 3. Recommendations — Prioritized Action Plan

### P0: Immediate (This Week)

| # | Action | Resolves | Effort | Owner |
|---|--------|----------|--------|-------|
| **P0-1** | Add soak test (1h) to CI with memory monitoring | R3, VAL-04 | 8h | cosca-devops + cosca-testing |
| **P0-2** | Add `-race` gate + clean-state test to CI | bug-003, bug-005, VAL-03, VAL-05 | 6h | cosca-devops |
| **P0-3** | Create DR backup: auto-commit memory after curation + weekly `.cosca/` backup | R11, RSK-11 | 4h | cosca-devops + cosca-memory-chief |
| **P0-4** | Enable TLS in Helm ingress (add cert-manager annotation example) | INF-02 | 1h | cosca-infrastructure |
| **P0-5** | Add PodDisruptionBudget to Helm chart | K8S-07 | 1h | cosca-infrastructure |

**Total P0 effort**: ~20h

### P1: Short-Term (This Month)

| # | Action | Resolves | Effort | Owner |
|---|--------|----------|--------|-------|
| **P1-1** | Extend soak test to 24h (scheduled, not per-PR) | R3 (full) | 4h | cosca-devops |
| **P1-2** | Add ALB + TLS to Terraform config | TF-01, TF-02 | 8h | cosca-infrastructure |
| **P1-3** | Add security groups + proper VPC to Terraform | TF-03, TF-04 | 4h | cosca-infrastructure |
| **P1-4** | Enable HPA + multi-replica defaults in Helm (≥2) | K8S-01, K8S-02 | 2h | cosca-infrastructure |
| **P1-5** | Add ServiceMonitor for Prometheus Operator | K8S-09 | 2h | cosca-infrastructure |
| **P1-6** | Add Grafana dashboard JSON to deploy/ | Observability | 4h | cosca-monitoring |
| **P1-7** | Implement external secrets (Sealed Secrets or Vault) | INF-06 | 8h | cosca-security |
| **P1-8** | Add CD stage to CI (push to registry on main tag) | INF-05 | 4h | cosca-devops |
| **P1-9** | Add Dependabot/Renovate config for Go + Docker | Supply chain | 2h | cosca-devops |

**Total P1 effort**: ~38h

### P2: Medium-Term (Next Quarter)

| # | Action | Resolves | Effort | Owner |
|---|--------|----------|--------|-------|
| **P2-1** | Evaluate SQLite → Postgres migration for HA (or Litestream for SQLite replication) | INF-03, K8S-06 | 40h | cosca-database + cosca-infrastructure |
| **P2-2** | Add CloudFront CDN to Terraform | INF-08 | 4h | cosca-infrastructure |
| **P2-3** | Add WAF rules to Terraform | INF-09 | 4h | cosca-security |
| **P2-4** | Multi-AZ deployment in Terraform | High availability | 8h | cosca-infrastructure |
| **P2-5** | Implement OpenTelemetry distributed tracing | Observability | 16h | cosca-monitoring + cosca-infrastructure |
| **P2-6** | Multi-region DR plan with RTO/RPO targets | DR | 24h | cosca-infrastructure |
| **P2-7** | Cost optimization review (right-sizing, reserved instances) | Cost | 4h | cosca-infrastructure |
| **P2-8** | Service mesh evaluation (Istio/Linkerd) | Service-to-service | 8h | cosca-infrastructure |
| **P2-9** | Kernel routing redundancy (R9 mitigation) | R9 | 16h | cosca-kernel + cosca-infrastructure |

**Total P2 effort**: ~124h

---

## 4. R3 & R9 Action Plans

### R3 Mitigation Plan: Soak Test Pipeline

```
Week 1:  CI 1-hour soak test
         ├── Script: scripts/soak-test.sh
         │   ├── Start cosca server with known config
         │   ├── Baseline RSS memory after 30s warm-up
         │   ├── Run traffic simulation (10 req/s, mixed endpoints)
         │   ├── Sample RSS every 5 minutes for 60 minutes
         │   ├── Alert if RSS > baseline × 1.10 (10% growth threshold)
         │   └── Alert if goroutine count grows > 20%
         └── Gate: Fail PR if soak test fails

Week 2-4:  Scheduled 24-hour soak test
         ├── GitHub Actions scheduled workflow (daily at 2am UTC)
         ├── Extended traffic simulation (concurrent agents, provider calls)
         ├── Metrics: memory, goroutines, file descriptors, DB connections
         └── Alert: Slack/email on failure

Ongoing:  Production memory profiling
         ├── pprof endpoint (enabled in debug mode)
         └── Continuous profiling via Parca/Pyroscope
```

### R9 Mitigation Plan: Kernel SPOF

```
Phase 1: Monitoring
         ├── Kernel routing success/failure metrics exported to Prometheus
         ├── Route latency histogram (p50, p95, p99)
         └── Alert on route failure rate > 1%

Phase 2: Critic Oversight (Short-term)
         ├── cosca-critic validates kernel routing decisions
         ├── Sampling rate: 20% of routing decisions
         └── Escalation on disagreement

Phase 3: Routing Redundancy (Long-term)
         ├── Option A: Dual-kernel with consensus
         │   └── Two kernels, quorum-based routing
         ├── Option B: Agent self-routing fallback
         │   └── Agents cache their own capability registry
         └── Option C: Leader election with failover
             └── Multiple kernel instances, leader elected via Raft/etcd
```

---

## 5. Architecture Decision Records (ADRs)

**Status**: No ADRs found in `internal/embed/cosca/memory/architecture/adr/` — directory does not exist.

**Recommendation**: Create ADRs for the following infrastructure decisions:
1. ADR-INF-001: Choice of ECS Fargate over EKS/Kubernetes
2. ADR-INF-002: SQLite as primary database (single-node architecture)
3. ADR-INF-003: Scratch-based Docker images for backend
4. ADR-INF-004: Helm chart as primary Kubernetes deployment method
5. ADR-INF-005: Health check design (startup/liveness/readiness probe strategy)

---

## 6. Summary Dashboard

| Area | Status | Maturity (1-5) | Critical Actions |
|------|--------|---------------|------------------|
| CI/CD Pipeline | 🟢 Functional | 3 | Add soak test, clean-state test, CD stage |
| Docker Strategy | 🟢 Good | 4 | Minor: add SBOM generation |
| Terraform (AWS) | 🟡 Basic | 1 | Add ALB, TLS, security groups, auto-scaling |
| Helm Chart (K8s) | 🟢 Good | 3 | Enable production defaults, PDB, ServiceMonitor |
| Networking | 🔴 Minimal | 1 | ALB, VPC, CDN, WAF, DNS |
| Observability | 🟡 Basic | 2 | Grafana dashboards, Alertmanager, tracing |
| Health Checks | 🟡 Partial | 3 | Fix BUG-U01 (Restart), BUG-U02 (StartupComplete) |
| Disaster Recovery | 🔴 None | 0 | Memory backup, DR runbook, RTO/RPO |
| Security (Infra) | 🔴 Minimal | 1 | TLS, WAF, secrets management, security groups |
| Auto-Scaling | 🟡 Defined/Disabled | 1 | Enable HPA, ECS scaling, multi-replica |
| Cost Optimization | ⚪ None | 0 | Right-sizing review, reserved instances |

**Overall Infrastructure Maturity Score**: **2.2 / 5** (Early-stage — functional for dev, not production-ready)

---

## 7. References

| Document | Path |
|----------|------|
| CI/CD Workflow | `.github/workflows/ci.yml` |
| Backend Dockerfile | `Dockerfile` |
| Web Dockerfile | `web/Dockerfile` |
| Docker Compose | `docker-compose.yml` |
| Terraform (AWS) | `deploy/terraform/aws/main.tf` |
| Helm Chart | `deploy/helm/cosca/` |
| Prometheus Config | `deploy/prometheus.yml` |
| Risk Registry | `internal/embed/cosca/memory/risk/RISK_REGISTRY.md` |
| Technical Debt Scorecard | `internal/embed/cosca/memory/technical-debt/scorecard.md` |
| Cognitive State | `internal/embed/cosca/memory/context/cognitive-state.md` |
| Deployment Workflow | `internal/embed/cosca/workflows/deployment.md` |
| Runtime Health Handler | `api/rest/handler/runtime.go` |

---

*Generated by cosca-infrastructure (Activation Wave 5) — 2026-07-28*
