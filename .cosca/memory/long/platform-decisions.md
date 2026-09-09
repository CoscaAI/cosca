---
type: long
key: platform-decisions
tags: [platform, decisions, architecture]
timestamp: 2026-07-23T00:00:00Z
status: active
agent: Platform Chief
---

# Platform Technical Decisions

## Internal Developer Platform
- Backstage as developer portal (CNCF graduated)
- Spotify's platform model with golden paths
- Self-service actions via Backstage Scaffolder
- TechDocs for documentation-as-code

## CI/CD Standards
- GitHub Actions as primary CI/CD platform
- Trunk-based development with short-lived feature branches
- Automatic preview environments for every PR
- Quality gates before merge (Gate 2 checks)
- Deployment approval for production (Gate 3 check)

## Observability Stack
- OpenTelemetry for instrumentation (vendor-neutral)
- Prometheus + Grafana for metrics
- Loki for log aggregation
- Tempo for distributed tracing
- PagerDuty for alerting
