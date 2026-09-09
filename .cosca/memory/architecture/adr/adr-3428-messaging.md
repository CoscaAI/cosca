# ADR-3428: Create Messaging Chief

**Status**: Accepted
**Deciders**: Chief AI Platform Architect
**Date**: 2026-07-23
**Tags**: organization, chief, v3.0, enterprise

## Context
The Cosca framework needed to expand its organizational coverage to include enterprise domains that were previously uncovered or inadequately covered by existing chiefs.

## Decision
Create the Messaging Chief role as a first-class department in the Cosca organizational structure, with dedicated SKILL.md, specialists, and council representation.

## Rationale
Event-driven architecture is a fundamental architectural pattern requiring dedicated ownership. Previously split between Backend and Architecture Chiefs. A dedicated Messaging Chief owns message brokers, schema registry, event catalog, and async communication standards.

## Alternatives Considered
1. Extend Backend Chief scope: Backend focuses on request-response, not async. 2. Extend Integration Chief scope: Integration focuses on external, not internal. 3. Dedicated Messaging Chief: Correct for event-driven architecture.

## Decision Outcome
Create dedicated Messaging Chief with:
- Reports to: as defined in ORGCHART.md
- Specialists: as defined in departments/messaging/SKILL.md
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
- [departments/messaging/SKILL.md](../../../departments/messaging/SKILL.md)
- [company/ORGCHART.md](../../../company/ORGCHART.md)
- [councils/COUNCILS.md](../../../councils/COUNCILS.md)
