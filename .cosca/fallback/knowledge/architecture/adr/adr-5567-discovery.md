# ADR-5567: Create Discovery Chief

**Status**: Accepted
**Deciders**: Chief AI Platform Architect
**Date**: 2026-07-23
**Tags**: organization, chief, v3.0, enterprise

## Context
The Cosca framework needed to expand its organizational coverage to include enterprise domains that were previously uncovered or inadequately covered by existing chiefs.

## Decision
Create the Discovery Chief role as a first-class department in the Cosca organizational structure, with dedicated SKILL.md, specialists, and council representation.

## Rationale
Automated codebase analysis and architecture discovery are essential for technical debt management and architecture governance. Previously no ownership. A dedicated Discovery Chief builds systems that automatically analyze codebases, detect patterns, and provide actionable intelligence.

## Alternatives Considered
1. Extend Architecture Chief scope: Architecture focuses on design, not analysis. 2. Use open-source tools: Lacks integration with Cosca. 3. Dedicated Discovery Chief: Automated intelligence integrated with Cosca.

## Decision Outcome
Create dedicated Discovery Chief with:
- Reports to: as defined in ORGCHART.md
- Specialists: as defined in departments/discovery/SKILL.md
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
- [departments/discovery/SKILL.md](../../../departments/discovery/SKILL.md)
- [company/ORGCHART.md](../../../company/ORGCHART.md)
- [councils/COUNCILS.md](../../../councils/COUNCILS.md)
