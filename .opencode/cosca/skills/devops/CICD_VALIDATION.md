---
name: cicd-validation
description: Use when the user asks to validate a CI/CD pipeline configuration, including workflow stages, secrets, caching, and gate ordering.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: DevOps Chief | **Last Updated**: 2026-07-23
> 
> # CI/CD VALIDATION SKILL
> 
> ## Description
> Use this skill to validate CI/CD pipeline configurations. Ensures pipelines follow best practices, are secure, efficient, and produce consistent results.
> 
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | pipeline_file | Yes | Path to CI/CD configuration file |
> | platform | Yes | `github-actions`, `gitlab-ci`, `jenkins`, `circle-ci`, `argo-workflows` |
> 
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Validation report | Pipeline compliance report |
> | Issues list | Issues found with severity |
> | Recommendations | Pipeline optimization suggestions |
> 
> ## Validation Checks
> 
> ### Pipeline Structure
> - Stages defined logically
> - Dependencies between stages correct
> - No unnecessary sequential stages
> - Caching configured for dependencies
> 
> ### Security
> - No secrets exposed in configuration
> - Least privilege for CI/CD tokens
> - SAST/DAST scanning integrated
> - Dependency scanning configured
> - Container image scanning
> 
> ### Efficiency
> - Parallel job execution where possible
> - Build caching configured
> - Artifact retention policies set
> - Timeouts configured
> - Resource limits defined
> 
> ### Quality Gates
> - Test suite runs automatically
> - Code quality checks enforced
> - Security scan passes before deploy
> - Performance benchmarks checked
> - Approval gate for production
> 
> ## Success Criteria
> - [ ] Pipeline structure validated
> - [ ] Security checks passed
> - [ ] Efficiency optimizations identified
> - [ ] Quality gates properly configured
> - [ ] Recommendations provided
> 
> ## Related
> - [DevOps Chief](../../departments/devops/SKILL.md)
> - [Platform Chief](../../departments/platform/SKILL.md)
> - [Security Chief](../../departments/security/SKILL.md)
> - [QUALITY_GATES.md](../../QUALITY_GATES.md) — Quality gates

## Process
1. **Pipeline Discovery**: Identify CI/CD configuration files (.github/workflows/, Makefile, Dockerfile).
2. **Build Verification**: Confirm build command (`make build`, `go build`) succeeds in CI environment.
3. **Test Execution**: Verify all test suites run in CI (`make test`, `go test ./...`).
4. **Lint & Vet**: Check that `golangci-lint` and `go vet` run without errors.
5. **Security Scanning**: Confirm `govulncheck` or equivalent runs in pipeline.
6. **Artifact Generation**: Verify binary/release artifacts are correctly produced and versioned.
7. **Deploy Configuration**: Check that deploy steps (Docker push, Helm install, terraform apply) are correctly configured.
8. **Report**: Document any pipeline gaps, flaky tests, or missing validations.
