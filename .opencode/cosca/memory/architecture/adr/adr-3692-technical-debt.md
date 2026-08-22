# ADR-3692: Create Technical Debt Chief

**Status**: Accepted
**Deciders**: Chief AI Platform Architect
**Date**: 2026-07-23
**Tags**: organization, chief, v3.0, enterprise

## Context
The Cosca framework needed to expand its organizational coverage to include enterprise domains that were previously uncovered or inadequately covered by existing chiefs.

## Decision
Create the Technical Debt Chief role as a first-class department in the Cosca organizational structure, with dedicated SKILL.md, specialists, and council representation.

## Rationale
Technical debt is a systemic problem requiring dedicated measurement, tracking, and reduction programs. Previously implicit in QA and Review Chiefs' responsibilities. A dedicated Technical Debt Chief establishes debt budgets, quality gates, and systematic reduction programs.

## Alternatives Considered
1. Extend QA Chief scope: QA focuses on testing, not debt. 2. Extend Review Chief scope: Review focuses on code, not system. 3. Dedicated Technical Debt Chief: Correct for systematic debt management.

## Decision Outcome
Create dedicated Technical Debt Chief with:
- Reports to: as defined in ORGCHART.md
- Specialists: as defined in departments/technical-debt/SKILL.md
- Council representation: as defined in COUNCILS.md

## Consequences
- Clear ownership for the domain
- Dedicated focus and accountability
- Better cross-department coordination through council representation
- Increased framework size (14 new departments)

## Compliance
- All new departments follow CONVENTIONS.md format
- All departments have AGENT_DNA.md-compliant contracts
- All departments have council representation

## Related
- [departments/technical-debt/SKILL.md](../../../departments/technical-debt/SKILL.md)
- [company/ORGCHART.md](../../../company/ORGCHART.md)
- [councils/COUNCILS.md](../../../councils/COUNCILS.md)
