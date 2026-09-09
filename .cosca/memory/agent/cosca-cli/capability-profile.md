# cosca-cli — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (1/5 tasks to L2)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| CLI tooling (Cobra CLI, shell completion, developer tools) | 0.40 | 1 | success | ↑ |

## Strengths
- CLI framework and architecture design with multi-platform support (Linux, macOS, Windows)
- Code generators, scaffolding tools, and shell completion implementation (bash, zsh, fish)
- CLI authentication, authorization, plugin architecture, and developer experience optimization

## Weaknesses
- No execution history — capabilities unverified
- Profile based on agent definition only, not practical experience

## Preferred Strategies
- Follow POSIX conventions with comprehensive help and man pages; support JSON and table output
- Never break CLI backward compatibility without migration; require opt-in for telemetry
- Deploy via package managers (brew, apt, npm); implement automatic update checking
- Keep CLI startup under 500ms; collect usage analytics with consent

## Known Failure Modes
- None recorded — agent has no execution history

## Evolution Goal
Reach Level 2:
"Complete first 5 real tasks and establish baseline confidence in primary domain"
