# cosca-discovery — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-08-04

## Current Level: 1

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Codebase Scanning (glob/grep/walk) | 0.85 | 3* | success | ↑ |
| Stack Detection (Go, SQLite, Next.js) | 0.82 | 3* | success | ↑ |
| File Count & Structure Inventory | 0.86 | 3* | success | ↑ |
| Import/Dependency Detection | 0.70 | 2* | success | → |
| Architecture Pattern Detection | 0.62 | 1 | success | ↑ |
| Dependency Graph Generation | 0.30 | 0 | — | → |
| CI/CD Integration | 0.10 | 0 | — | → |
| Module Boundary Detection | 0.40 | 2* | success | → |
| Security Vulnerability Discovery | 0.15 | 0 | — | → |
| Documentation Gap Analysis | 0.70 | 2* | success | ↑ |

> *Tasks inferred from Phase 1+2 documentation sync where cosca-discovery was used for codebase scanning. Learnings.md contains only seed data; capabilities are inferred from SKILL.md scope and delegate context from other agents' learnings.

## Strengths
- **Full-codebase structural scanning**: Proven ability to scan large codebases (357 Go files, 240+ TSX files, 887 doc assets) and produce accurate file counts, module inventories, and technology stack profiles — used as the primary scanner by cosca-documentation in Phase 1+2.
- **Technology stack profiling**: Can detect the project's technology stack (Go backend, Next.js 15 frontend, SQLite database, TypeScript, Tailwind CSS) from go.mod, package.json, and directory structure — accurate enough to catch PostgreSQL fantasy in database docs.
- **Module boundary identification**: Can distinguish internal packages (internal/runtime/, internal/sqlite/, internal/search/) from public API surface (pkg/cosca/) and third-party dependencies — enabling accurate architecture maps.
- **Documentation gap detection**: Used by cosca-documentation to identify areas where docs were missing or stale — contributed to surfacing 3 critically stale documentation files.
- **Evidence-tiered reality classification**: Separates executable source, test/contract evidence, live artifacts, archived content, and claims; caught current README/roadmap count drift and inactive Python voice artifacts.

## Weaknesses
- **No automated CI integration**: All discovery has been manual/ad-hoc; has not designed a CI pipeline step that runs on every commit.
- **No dependency graph generation**: Can list imports and modules but has not produced formal dependency graphs (no import cycle detection, no coupling metrics).
- **No security discovery**: Has not scanned for vulnerability patterns (hardcoded secrets, vulnerable imports, misconfigurations) — this remains entirely with cosca-security.
- **Limited runtime validation**: Full Go execution was validated, but frontend/provider/jail/browser/voice execution remains environment-dependent.

## Preferred Strategies
- **Scanner-first architecture mapping**: Uses glob patterns, grep for imports, and directory walks to build complete file/package inventories before any analysis begins — ensures no blind spots.
- **Stack detection from package manifests**: Reads go.mod, package.json, and config files to determine the actual technology stack rather than trusting documentation claims — prevented PostgreSQL fantasy from persisting.
- **Module boundary discovery**: Traces import paths to identify internal packages, external dependencies, and module groups — enabling architecture agents to understand the codebase topology.

## Known Failure Modes
- None recorded — patterns.md is empty; learnings.md contains only seed data. No execution failures have been captured. Caution: level 1 only; no automated or structured discovery processes have been validated.

## Evolution Goal
Reach Level 2:
*"Integrate automated codebase scanning into CI/CD (run on every commit), generate formal dependency graphs with import cycle detection and coupling metrics, produce versioned discovery reports, and add security vulnerability pattern scanning — graduating from ad-hoc manual scans to continuous automated architecture intelligence."*
