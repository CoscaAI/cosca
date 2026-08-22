# CAPABILITY ENGINE

> **Version**: 1.0.0 | **Status**: active | **Owner**: Capability Engine | **Last Updated**: 2026-07-12

## PURPOSE
The Capability Engine manages the complete lifecycle of all capabilities in the Cosca ecosystem. It discovers, registers, validates, resolves, and composes capabilities. Every responsibility in the platform is expressed as a Capability — this engine is the registry and resolver for all of them.

## ACTIVATION
- Bootstrap Phase 4: Discover all capabilities from departments and engines
- On capability request: Resolve which provider can fulfill a capability
- On capability change: Re-validate dependencies and consumers

## SCOPE
- Capability discovery from department and engine definitions
- Capability registration and versioning
- Capability resolution (which provider for which capability)
- Capability composition (combining capabilities for complex tasks)
- Capability dependency graph maintenance
- Capability health monitoring (is every capability covered?)

## PROCESS

### Discovery
```
Scan all departments/*/SKILL.md → extract RESPONSIBILITIES → register as capabilities
Scan all engines/*/SKILL.md → extract capabilities → register
Validate against CAPABILITY_CATALOG.md
Report gaps (capabilities without providers)
```

### Resolution
```
Request: "I need code review"
  → Query capability catalog: CAP-QUAL-001
  → Resolve providers: Review Chief (primary), Architecture Chief (architecture review)
  → Return best provider based on context and availability
```

### Composition
```
Request: "Build a new feature"
  → Required capabilities: [CAP-PROD-001, CAP-ARCH-001, CAP-ENG-001, CAP-QUAL-001, ...]
  → Build dependency graph
  → Determine execution order
  → Assign providers
```

## DEPENDENCIES
- CAPABILITY_CATALOG.md — Capability registry
- Skills Engine — Skill-to-capability mapping
- Planning Engine — Capability composition for plans

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Initial Capability Engine |
