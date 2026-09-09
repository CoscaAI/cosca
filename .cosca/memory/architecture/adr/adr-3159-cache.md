# ADR-3159: Create Cache Chief

**Status**: Accepted
**Deciders**: Chief AI Platform Architect
**Date**: 2026-07-23
**Tags**: organization, chief, v3.0, enterprise

## Context
The Cosca framework needed to expand its organizational coverage to include enterprise domains that were previously uncovered or inadequately covered by existing chiefs.

## Decision
Create the Cache Chief role as a first-class department in the Cosca organizational structure, with dedicated SKILL.md, specialists, and council representation.

## Rationale
Caching strategy impacts performance, cost, and data consistency across all services. Previously fragmented across Backend and Performance Chiefs. A dedicated Cache Chief owns multi-tier caching architecture, CDN strategy, and cache invalidation policies.

## Alternatives Considered
1. Extend Performance Chief scope: Performance is measurement, not infrastructure. 2. Extend Infrastructure Chief scope: Too operational. 3. Dedicated Cache Chief: Covers strategy, infrastructure, and patterns.

## Decision Outcome
Create dedicated Cache Chief with:
- Reports to: as defined in ORGCHART.md
- Specialists: as defined in departments/cache/SKILL.md
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
- [departments/cache/SKILL.md](../../../departments/cache/SKILL.md)
- [company/ORGCHART.md](../../../company/ORGCHART.md)
- [councils/COUNCILS.md](../../../councils/COUNCILS.md)
