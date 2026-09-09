# ADR-3929: Create CLI Chief

**Status**: Accepted
**Deciders**: Chief AI Platform Architect
**Date**: 2026-07-23
**Tags**: organization, chief, v3.0, enterprise

## Context
The Cosca framework needed to expand its organizational coverage to include enterprise domains that were previously uncovered or inadequately covered by existing chiefs.

## Decision
Create the CLI Chief role as a first-class department in the Cosca organizational structure, with dedicated SKILL.md, specialists, and council representation.

## Rationale
CLI tools are essential for developer productivity and platform self-service. Previously part of Automation Chief's scope. A dedicated CLI Chief owns CLI design, code generators, shell completions, and multi-platform support.

## Alternatives Considered
1. Extend Automation Chief scope: Too broad. 2. Extend Platform Chief scope: Platform focuses on portal, not CLI. 3. Dedicated CLI Chief: Correct for developer tooling.

## Decision Outcome
Create dedicated CLI Chief with:
- Reports to: as defined in ORGCHART.md
- Specialists: as defined in departments/cli/SKILL.md
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
- [departments/cli/SKILL.md](../../../departments/cli/SKILL.md)
- [company/ORGCHART.md](../../../company/ORGCHART.md)
- [councils/COUNCILS.md](../../../councils/COUNCILS.md)
