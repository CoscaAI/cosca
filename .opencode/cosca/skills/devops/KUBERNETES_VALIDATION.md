---
name: kubernetes-validation
description: Use when the user asks to validate Kubernetes manifests (Deployment, Service, Ingress, ConfigMap) for best practices and security.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: DevOps Chief | **Last Updated**: 2026-07-23

# KUBERNETES VALIDATION SKILL

## Description
Validate Kubernetes manifests (Deployment, Service, Ingress, ConfigMap, etc.) against best practices, security standards, resource efficiency, and organizational conventions.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| manifest_path | Yes | Path to Kubernetes manifest(s) |
| cluster_version | No | Target cluster version (default: 1.28) |
| strict_mode | No | Enable strict validation (default: true) |

## Outputs
| Output | Description |
|--------|-------------|
| Validation report | Pass/fail per validation rule |
| Issues list | Issues with severity (error, warning, info) |
| Recommendations | Fix suggestions and optimization tips |

## Validation Categories

### Security
- [ ] Run as non-root user (securityContext)
- [ ] Read-only root filesystem
- [ ] No privileged containers
- [ ] Seccomp profile configured
- [ ] Network policies defined
- [ ] Pod Security Standards enforced
- [ ] Secrets, not ConfigMaps, for sensitive data
- [ ] ServiceAccount with least privilege

### Resource Management
- [ ] CPU requests and limits set
- [ ] Memory requests and limits set
- [ ] HPA configured for scalable workloads
- [ ] PodDisruptionBudget configured
- [ ] Resource quotas defined per namespace
- [ ] LimitRanges configured

### Reliability
- [ ] Readiness probe configured
- [ ] Liveness probe configured
- [ ] Startup probe for slow-starting apps
- [ ] Replicas > 1 for stateless services
- [ ] Anti-affinity rules for HA
- [ ] Graceful shutdown (preStop hook)

### Configuration
- [ ] Image tag pinned (no `latest`)
- [ ] ImagePullPolicy set appropriately
- [ ] ConfigMaps used for configuration
- [ ] Environment variables from ConfigMaps/Secrets
- [ ] Labels and selectors consistent
- [ ] Rolling update strategy configured

## Process
1. Parse all Kubernetes manifests
2. Validate API version compatibility
3. Check security configurations
4. Verify resource management settings
5. Assess reliability configurations
6. Validate naming and labeling conventions
7. Generate report with prioritized findings

## Success Criteria
- [ ] All manifests validated
- [ ] Security issues flagged as errors
- [ ] Resource limits verified
- [ ] Probe configuration checked
- [ ] Naming conventions validated
- [ ] Recommendations prioritized

## Related
- [DevOps Chief](../../departments/devops/SKILL.md)
- [Docker Validation](./DOCKER_VALIDATION.md)
- [CI/CD Validation](./CICD_VALIDATION.md)
- [Security Chief](../../departments/security/SKILL.md)
- [Infrastructure Chief](../../departments/infrastructure/SKILL.md)
