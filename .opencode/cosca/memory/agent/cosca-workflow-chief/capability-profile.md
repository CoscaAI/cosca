# cosca-workflow-chief — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 2 (first real task completed — workflow audit)

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Workflow orchestration (definitions, pipelines, states, dependencies) | 0.85 | 1 | success (2026-07-28) | ↑ |
| Workflow catalog & classification | 0.80 | 1 | success (2026-07-28) | ↑ |
| Workflow quality audit & gap analysis | 0.80 | 1 | success (2026-07-28) | ↑ |

## Strengths
- Workflow definition design with task pipeline orchestration and state/transition management
- Task dependency management with parallel execution path planning
- Workflow error handling and retry strategies with execution monitoring
- ✅ VERIFIED: Cross-system workflow audit across 28 definitions, engine code, MCP tools, REST API, and CLI
- ✅ VERIFIED: Structured gap analysis with bug identification and prioritized recommendations

## Weaknesses
- Limited execution history (1 task) — confidence scores are self-assessed, not cross-validated
- No experience with workflow execution optimization or CI/CD integration yet

## Preferred Strategies
- Define workflow templates for standard patterns (project-init, feature-development, bug-fix, release, deployment, etc.)
- Never implement workflow steps — delegate to respective departments and specialists
- Monitor workflow execution; optimize efficiency; report workflow status regularly
- Handle errors with retries; define clear state transitions for every workflow
- Audit workflows systematically; track schema-to-implementation gaps; produce actionable reports

## Known Failure Modes
- None recorded — insufficient execution history

## Evolution Goal
Reach Level 3:
"Complete 5 successful tasks with cross-domain workflow design; implement at least 1 engine improvement (retry, parallel, or DAG)"
