# ADR-2745: Create SDK Chief

**Status**: Accepted
**Deciders**: Chief AI Platform Architect
**Date**: 2026-07-23
**Tags**: organization, chief, v3.0, enterprise

## Context
The Cosca framework needed to expand its organizational coverage to include enterprise domains that were previously uncovered or inadequately covered by existing chiefs.

## Decision
Create the SDK Chief role as a first-class department in the Cosca organizational structure, with dedicated SKILL.md, specialists, and council representation.

## Rationale
Multi-language SDKs require consistent design, documentation, and release processes. Previously ad-hoc across language-specific teams. A dedicated SDK Chief ensures consistent developer experience across TypeScript, Python, Go, Java and other languages.

## Alternatives Considered
1. Single-language SDKs only: Limits platform adoption. 2. Auto-generate SDKs from OpenAPI: Lacks language-specific optimization. 3. Dedicated SDK Chief: Multi-language with consistent quality.

## Decision Outcome
Create dedicated SDK Chief with:
- Reports to: as defined in ORGCHART.md
- Specialists: as defined in departments/sdk/SKILL.md
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
- [departments/sdk/SKILL.md](../../../departments/sdk/SKILL.md)
- [company/ORGCHART.md](../../../company/ORGCHART.md)
- [councils/COUNCILS.md](../../../councils/COUNCILS.md)
