---
name: validation
description: Provides comprehensive input/output/contract validation across all Cosca components.
level: 3
---

# VALIDATION ENGINE

> **Version**: 1.0.0 | **Status**: active | **Owner**: Validation Engine | **Last Updated**: 2026-07-12

## PURPOSE
The Validation Engine provides comprehensive input/output/contract validation across all Cosca components.

## SCOPE
- Agent output validation against contracts
- Capability contract validation
- Cross-reference integrity validation
- CONVENTIONS.md compliance checking
- AGENT_DNA.md compliance checking
- Schema validation for memory records

## VALIDATION TYPES
| Type | Checks | Trigger |
|------|--------|---------|
| Contract | Output matches capability contract | After agent execution |
| Format | File follows CONVENTIONS.md | On file change |
| DNA | Agent has all 23 fields | On agent registration |
| Reference | All links resolve | Bootstrap Phase 9 |
| Schema | Memory records match schema | On memory write |

## DEPENDENCIES
- Cross-Reference Validator — Link integrity
- CONVENTIONS.md — Format standards
- AGENT_DNA.md — Agent contract standards

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Initial Validation Engine |
