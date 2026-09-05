# ADR-1322: Create Compliance Chief

**Status**: Accepted
**Deciders**: Chief AI Platform Architect
**Date**: 2026-07-23
**Tags**: organization, chief, v3.0, enterprise

## Context
The Cosca framework needed to expand its organizational coverage to include enterprise domains that were previously uncovered or inadequately covered by existing chiefs.

## Decision
Create the Compliance Chief role as a first-class department in the Cosca organizational structure, with dedicated SKILL.md, specialists, and council representation.

## Rationale
Regulatory compliance (GDPR, SOC2, HIPAA, PCI-DSS) is a specialized domain requiring dedicated ownership. Previously part of Security Chief's scope, but compliance is a distinct discipline with different skills, processes, and audit requirements.

## Alternatives Considered
1. Keep within Security Chief: Compliance is broader than security (privacy, risk, audit). 2. Outsource compliance: Cannot outsource accountability. 3. Dedicated Compliance Chief: Correct level of ownership.

## Decision Outcome
Create dedicated Compliance Chief with:
- Reports to: as defined in ORGCHART.md
- Specialists: as defined in departments/compliance/SKILL.md
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
- [departments/compliance/SKILL.md](../../../departments/compliance/SKILL.md)
- [company/ORGCHART.md](../../../company/ORGCHART.md)
- [councils/COUNCILS.md](../../../councils/COUNCILS.md)
