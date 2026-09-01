# cosca-documentation — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 3

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| GRAND_SCALE Doc Audit (800+ assets) | 0.95 | 1 | success | ↑ |
| README/CHANGELOG Accuracy | 0.90 | 1 | success | ↑ |
| API Documentation | 0.85 | 1 | success | ↑ |
| Architecture Documentation | 0.85 | 1 | success | ↑ |
| Middleware Documentation | 0.80 | 1 | success | → |
| Frontend Architecture Docs | 0.80 | 1 | success | ↑ |
| Runtime Documentation | 0.75 | 1 | success | → |
| Diagram Creation (Mermaid) | 0.50 | 0 | — | → |
| CI Automation for Doc Health | 0.15 | 0 | — | → |
| Contribution Guides | 0.10 | 0 | — | → |

## Strengths
- **Enterprise-scale multi-source documentation audit**: Audited 887 documentation assets against ground-truth codebase scan (357 Go files, 240+ TSX files), caught 3 critically stale files, verified memory health (309 files, zero broken links, 95% YAML frontmatter coverage) — this is the highest-complexity task any Cosca agent has performed.
- **Automated number verification**: Pattern of cross-referencing README badges and stats tables against actual code counts — caught drift in agent count (52→51), CLI commands (34→39), Go files (336→357), workflows (25→26).
- **Delegate-to-specialists pipeline**: Recognizes that effective documentation requires input from specialized agents (Discovery for codebase scans, Memory Chief for memory health) — never tries to do it all alone.
- **Honest limitation documentation**: Documents gaps transparently (Restart() broke due to state machine bug, EventStartupComplete fires too early, hot reload not implemented) rather than papering over issues.
- **Cross-file consistency verification**: Detected version discrepancy (docs/README.md=v1.3.0 vs CHANGELOG=v1.4.0-dev), MEMORY_MODEL.md 44-line sync gap between .opencode/ and internal/embed/, and SDK docs describing a non-existent Go client library.

## Weaknesses
- **No automated documentation health CI**: Has identified the problem (badges drift, version inconsistency, SDK doc fiction) but has not implemented automated checks.
- **No documentation contribution guide**: Has not created a contributor's guide for documentation standards.
- **No automated link checking**: Cross-reference links exist but validity is manual.

## Preferred Strategies
- **Multi-source cross-referencing**: Always cross-references README badges, docs/ structure, memory files, and actual codebase counts before claiming documentation accuracy.
- **Delegate then aggregate**: Launches specialized sub-agents (Discovery, Memory Chief, Security, Runtime) for deep codebase scans, then aggregates structured findings into consistent documentation with uniform headers (Status, Owner, Last Updated, Version).
- **Gap-first documentation**: Prioritizes filling documented gaps (missing frontend architecture, missing auth reference, missing middleware reference) over polishing existing docs — ensures no "ghost docs" remain.

## Known Failure Modes
- None recorded — both learning entries show successful outcomes.

## Evolution Goal
Reach Level 4:
*"Create a fully automated doc-health CI check that validates README numbers, version consistency across all files, SDK doc accuracy against go.mod imports, and cross-reference link integrity on every commit — graduating from manual audit excellence to continuous documentation quality enforcement."*
