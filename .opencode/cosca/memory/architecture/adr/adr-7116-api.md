# ADR-7116: Create API Chief

**Status**: Accepted
**Deciders**: Chief AI Platform Architect
**Date**: 2026-07-23
**Tags**: organization, chief, v3.0, enterprise

## Context
The Cosca framework needed to expand its organizational coverage to include enterprise domains that were previously uncovered or inadequately covered by existing chiefs.

## Decision
Create the API Chief role as a first-class department in the Cosca organizational structure, with dedicated SKILL.md, specialists, and council representation.

## Rationale
API governance is a cross-cutting concern that touches all services. Previously split between Backend and Architecture Chiefs, causing inconsistent API design. A dedicated API Chief ensures consistent API contracts, versioning, and gateway management across the organization.

## Alternatives Considered
1. Extend Backend Chief scope: Overloaded Backend Chief, API governance would be secondary priority. 2. Create API Guild without formal authority: Lacks decision power for enforcement. 3. Dedicated API Chief: Correct level of authority and focus.

## Decision Outcome
Create dedicated API Chief with:
- Reports to: as defined in ORGCHART.md
- Specialists: as defined in departments/api/SKILL.md
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
- [departments/api/SKILL.md](../../../departments/api/SKILL.md)
- [company/ORGCHART.md](../../../company/ORGCHART.md)
- [councils/COUNCILS.md](../../../councils/COUNCILS.md)
