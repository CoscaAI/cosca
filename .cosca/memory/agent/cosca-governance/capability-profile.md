# cosca-governance — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 1 (first real task executed — transitioning to 2)

First task completed: Full DNA v3.0 compliance audit of 54 agents (2026-07-28).

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Framework governance (standards, conventions, duplication, evolution) | 0.45 | 1 | success | ↑ |
| Audit methodology (structural scan, content analysis, cross-reference) | 0.40 | 1 | success | ↑ |

## Strengths
- Cosca framework standards enforcement and convention compliance auditing
- Bulk structural scanning with automated compliance extraction (bash/glob/grep)
- Duplication detection and elimination across skills and agent contracts
- Framework evolution governance with controlled, auditable change management
- Template pattern recognition — identifying structural consistency across 54 agents

## Weaknesses
- Limited execution history (1 task) — capabilities partially verified
- Multi-line table parsing in capability profiles requires heuristic extraction
- Subagent delegation for parallel content analysis needs improved workflow
- Domain overlap assessment between adjacent agents is qualitative, not quantitative

## Preferred Strategies
- Audit skill and agent contracts for CONVENTIONS compliance, not application-level quality
- Detect and eliminate framework duplication; govern RFC process for framework changes
- Ensure cross-reference integrity and terminology consistency; manage deprecation properly
- Oversee quality gates for framework changes; maintain contributor guidelines
- Use two-tier analysis: fast structural scan (automated) → deep content analysis (manual)
- Always read reference documents (AGENT_DNA.md, CONVENTIONS.md, GOVERNANCE.md) before auditing

## Known Failure Modes
- **Grep fragility on multi-line tables**: Extracting confidence scores and domain names via single-line regex fails on wrapped table cells. Mitigation: read target files directly after structural scan flags anomalies.
- **Depth-limit on subagents**: explore subagents hit depth limit (1) when nesting audits. Mitigation: use direct bash/grep for bulk scans, manual reads for content.

## Evolution Goal
Reach Level 2 (1/5 tasks complete):
"Complete first 5 real tasks and establish baseline confidence in primary domain"
- Next targets: (2) Fix cosca-semantic-memory profile, (3) Clean orphaned files, (4) Update INDEX.md, (5) Quarterly re-audit
