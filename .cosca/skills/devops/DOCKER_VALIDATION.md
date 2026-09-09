> **Version**: 1.0.0 | **Status**: active | **Owner**: DevOps Chief | **Last Updated**: 2026-07-23

# DOCKER VALIDATION SKILL

## Description
Validate Dockerfiles and Docker Compose configurations against best practices, security guidelines, efficiency standards, and organizational conventions.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| dockerfile_path | Yes | Path to Dockerfile(s) to validate |
| compose_path | No | Path to docker-compose file(s) |
| strict_mode | No | Enable strict validation (default: false) |

## Outputs
| Output | Description |
|--------|-------------|
| Validation report | Pass/fail per validation rule |
| Issues list | Issues found with severity (error, warning, info) |
| Recommendations | Fix suggestions with priority |

## Validation Categories

### Security
- [ ] Base image pinned to specific version (no `latest` tag)
- [ ] Non-root user configured (`USER` instruction)
- [ ] No secrets in build args or env
- [ ] Minimal package installations (no dev packages in prod)
- [ ] No unnecessary packages or tools
- [ ] Regular security scanning of base images
- [ ] HEALTHCHECK instruction present

### Efficiency
- [ ] Layer count minimized (combine RUN commands)
- [ ] Layer ordering optimized (least-changing layers first)
- [ ] Multi-stage build for smaller images
- [ ] `.dockerignore` present and excludes unnecessary files
- [ ] Cache mounts used for package managers (where supported)
- [ ] Appropriate base image size (alpine vs full distro)

### Configuration
- [ ] EXPOSE for application ports
- [ ] WORKDIR set appropriately
- [ ] COPY vs ADD (prefer COPY for local files)
- [ ] Entrypoint vs CMD (prefer ENTRYPOINT + CMD)
- [ ] Environment variables documented in README
- [ ] Labels set for metadata (maintainer, version)

### Docker Compose
- [ ] Service names follow conventions
- [ ] Resource limits defined (memory, CPU)
- [ ] Restart policy configured
- [ ] Health checks defined for services
- [ ] Volume mounts configured correctly
- [ ] Networks configured with appropriate driver

## Process
1. Parse Dockerfile(s) into instruction sequence
2. Validate security rules against each instruction
3. Check efficiency best practices
4. Verify configuration completeness
5. Parse compose file (if provided) and validate
6. Generate report with issues and fix suggestions

## Success Criteria
- [ ] All validation categories checked
- [ ] Security issues flagged as errors
- [ ] Efficiency improvements identified
- [ ] Image size reduction opportunities documented
- [ ] Compose configuration validated

## Related
- [DevOps Chief](../../departments/devops/SKILL.md)
- [Kubernetes Validation](./KUBERNETES_VALIDATION.md)
- [CI/CD Validation](./CICD_VALIDATION.md)
- [Security Chief](../../departments/security/SKILL.md)
