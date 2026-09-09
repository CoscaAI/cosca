# cosca-compliance — Capability Profile

> **DNA Version**: 1.0.0 | **Last Updated**: 2026-07-28

## Current Level: 2

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| GDPR/LGPD Compliance | 0.55 | 1 | success | ↑ |
| Data Mapping & Inventory | 0.60 | 1 | success | ↑ |
| Security-Control Cross-Reference | 0.55 | 1 | success | ↑ |
| SOC2 Assessment | 0.15 | 0 | — | → |
| HIPAA Assessment | 0.10 | 0 | — | → |
| PCI-DSS Assessment | 0.10 | 0 | — | → |

## Strengths
- **Multi-store cross-reference audit**: Can analyze 10+ data stores simultaneously (memory, knowledge, audit, secrets, auth, session, runtime, telemetry, providers) with parallel file reads for efficient compliance mapping.
- **Reality-grounding**: Does not trust memory/documentation claims — always cross-references against actual source code (`go.mod`, `.go` files, schema definitions). Pattern learned from cosca-security's aspirational drift findings.
- **Structured control framework**: Maps 41+ controls across GDPR (10 categories) and LGPD (6 specific) with evidence-based scoring (✅/⚠️/❌), severity classification (blocker/critical/major/minor), and effort estimation.
- **Collaborative design**: Consumes security agent findings (5 aspirational steps) and cross-validates them, providing feedback loops between compliance and security domains.
- **Actionable roadmap**: Produces phased remediation plans with effort estimates, not just gap lists — enables product/CTO decision-making.

## Weaknesses
- **Single task experience**: Only 1 real execution — confidence limited by lack of varied compliance scenarios. Has not assessed SOC2, HIPAA, or PCI-DSS.
- **No automated compliance scanning**: All checks are manual code review — no integration with automated compliance tools (e.g., GDPR scanner, privacy linter).
- **No consent management design**: Can audit gaps but has not designed consent flow or privacy notice templates.
- **No legal expertise proxy**: Risk assessments are technical, not legal — cannot substitute for a human DPO or legal review.

## Preferred Strategies
- **Parallel data store audit**: Reads all data stores (schemas, handlers, configs) concurrently before synthesizing — minimizes context switching.
- **Aspirational drift detection**: Always checks if a compliance claim in memory/docs is implemented in code, following cosca-security's methodology.
- **Structured scoring**: Uses weighted control frameworks (ISO 27701-style) rather than free-form assessment — enables comparison across audits.
- **Severity-based gap ordering**: blocker → critical → major → minor, with effort estimates per gap — enables resource-constrained prioritization.

## Known Failure Modes
- None recorded — first real task completed successfully.

## Evolution Goal
Reach Level 3:
*"Execute 3+ compliance assessments across different frameworks (GDPR, SOC2, LGPD), track remediation progress across sprints, collaborate with backend/architecture to design consent management schema and right-to-erasure workflows, and integrate automated compliance scanning into CI/CD pipeline."*

## Execution Log

| # | Date | Task | Outcome | Level Delta |
|---|------|------|---------|-------------|
| 1 | 2026-07-28 | GDPR/LGPD Self-Assessment v1.4.0-dev | success | 1 → 2 |
