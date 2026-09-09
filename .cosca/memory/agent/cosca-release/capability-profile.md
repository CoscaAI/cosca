# cosca-release — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 2 (first real task completed — activation audit)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Release audit (versioning, CI/CD, changelog, goreleaser) | 0.75 | 1 | success | ↑ |
| Release coordination (tagging, branching, deployment orchestration) | 0.25 | 0 | — | → |
| Rollback procedure management | 0.25 | 0 | — | → |

## Strengths
- Comprehensive release pipeline analysis — cross-referencing source code, build config, CI/CD, and changelog
- Detecting version constant drift and configuration inconsistencies
- CI/CD pipeline auditing (GitHub Actions, GoReleaser, Docker, Makefile)
- Changelog format validation against Keep a Changelog and SemVer standards
- Identifying automation gaps and providing actionable recommendations

## Weaknesses
- No hands-on release execution yet (tag creation, artifact publishing)
- No rollback procedure experience
- No experience with release candidate/pre-release workflows
- No cross-agent coordination (QA, DevOps, Documentation) during releases

## Preferred Strategies
- Cross-reference version constants across all sources (pkg/, CHANGELOG, git tags) before any release
- Validate GoReleaser config for correct repository references before tag push
- Always check that CHANGELOG has an Unreleased section following Keep a Changelog
- Automate via Makefile targets what cannot be automated via CI/CD

## Known Failure Modes
- **Version constant drift**: Hardcoded version in `pkg/cosca/cosca.go` can become stale if ldflags are the only mechanism keeping it current — mitigrate by adding a init-time warning or auto-detection
- **Wrong repo reference in GoReleaser**: If `release.github` points to the wrong owner/name, CD pipeline publishes to wrong repo — always verify before tagging
- **Tagless CHANGELOG**: If git tags don't match CHANGELOG versions, release history is unreproducible — every CHANGELOG entry must have a corresponding tag

## Evolution Goal
Reach Level 3:
"Execute a complete real release end-to-end, including version bump, tag creation, GoReleaser publish, and post-release validation. Automate the release workflow with a `make release` target."
