# ADR-2958: Create Provider Chief

**Status**: Accepted
**Deciders**: Chief AI Platform Architect
**Date**: 2026-07-23
**Tags**: organization, chief, v3.0, enterprise

## Context
The Cosca framework needed to expand its organizational coverage to include enterprise domains that were previously uncovered or inadequately covered by existing chiefs.

## Decision
Create the Provider Chief role as a first-class department in the Cosca organizational structure, with dedicated SKILL.md, specialists, and council representation.

## Rationale
Multi-provider strategy (AI, cloud) requires dedicated management of costs, SLAs, failover, and relationships. Previously split between AI Chief and Infrastructure Chief. A dedicated Provider Chief optimizes provider portfolio and ensures business continuity.

## Alternatives Considered
1. Extend AI Chief scope: Only covers AI providers. 2. Extend Infrastructure Chief scope: Covers only cloud. 3. Dedicated Provider Chief: Covers both AI and cloud providers.

## Decision Outcome
Create dedicated Provider Chief with:
- Reports to: as defined in ORGCHART.md
- Specialists: as defined in departments/provider/SKILL.md
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
- [departments/provider/SKILL.md](../../../departments/provider/SKILL.md)
- [company/ORGCHART.md](../../../company/ORGCHART.md)
- [councils/COUNCILS.md](../../../councils/COUNCILS.md)
