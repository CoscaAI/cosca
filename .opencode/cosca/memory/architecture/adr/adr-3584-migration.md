# ADR-3584: Create Migration Chief

**Status**: Accepted
**Deciders**: Chief AI Platform Architect
**Date**: 2026-07-23
**Tags**: organization, chief, v3.0, enterprise

## Context
The Cosca framework needed to expand its organizational coverage to include enterprise domains that were previously uncovered or inadequately covered by existing chiefs.

## Decision
Create the Migration Chief role as a first-class department in the Cosca organizational structure, with dedicated SKILL.md, specialists, and council representation.

## Rationale
Migrations (data, schema, cloud, architecture) are high-risk, high-impact projects requiring specialized planning and execution. Previously handled ad-hoc by individual teams. A dedicated Migration Chief ensures safe, reversible, documented migrations.

## Alternatives Considered
1. Extend Database Chief scope: Covers only database migrations. 2. Project-based teams: Lacks standardization. 3. Dedicated Migration Chief: Covers all migration types.

## Decision Outcome
Create dedicated Migration Chief with:
- Reports to: as defined in ORGCHART.md
- Specialists: as defined in departments/migration/SKILL.md
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
- [departments/migration/SKILL.md](../../../departments/migration/SKILL.md)
- [company/ORGCHART.md](../../../company/ORGCHART.md)
- [councils/COUNCILS.md](../../../councils/COUNCILS.md)
