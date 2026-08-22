> **Version**: 1.0.0 | **Status**: active | **Owner**: Skills Engine | **Last Updated**: 2026-07-10

# SKILLS ENGINE

## PURPOSE
The Skills Engine manages the Cosca skill registry. It discovers, loads, validates, and routes to skills across the entire system.

## SKILL LIFECYCLE

### 1. Discovery
- Scan ${DEPARTMENTS_HOME}/ for department SKILL.md files
- Scan ${ENGINES_HOME}/ for engine SKILL.md files
- Scan ${WORKFLOWS_HOME}/ for workflow definitions
- Scan ${TEMPLATES_HOME}/ for project templates
- Identify all available skills

### 2. Validation
- Every skill must have: ROLE, RESPONSIBILITIES, REVIEW CRITERIA sections
- Every department skill must have: DELEGATION, ESCALATION, FORBIDDEN ACTIONS sections
- Every engine skill must have: PURPOSE, INTEGRATION sections
- No duplicate skill names

### 3. Loading
- Load skill content into context
- Map skill to agent type
- Configure skill parameters

### 4. Routing
- Kernel determines which skill to load based on request type
- Route to department chief skills for domain-specific work
- Route to engine skills for cross-cutting concerns
- Route to workflow skills for structured processes

## SKILL REGISTRY

Complete skill registry is maintained in [COSCA_INDEX.md](../../COSCA_INDEX.md). See [CONVENTIONS.md](../../CONVENTIONS.md) for skill format standards and [GOVERNANCE.md](../../GOVERNANCE.md) for lifecycle policies.

## SKILL COMPOSITION
Skills can compose other skills:
- KERNEL loads → CEO → CTO → Department Chiefs
- KERNEL loads → Context Engine → Memory Engine
- Product Chief loads → Wizard Engine
- CTO loads → Planning Engine → Workflow Engine
- Any Chief loads → Review Engine → QA Engine → Documentation Engine

## DEPENDENCIES

| Kernel | Skill routing and loading |
| Evolution Engine | Self-evolution validation |
| COSCA_INDEX.md | Skill registry index |
| CONVENTIONS.md | Skill format standards |
| GOVERNANCE.md | Lifecycle policies |

## RELATED
- [COSCA_INDEX.md](../../COSCA_INDEX.md)
- [CONVENTIONS.md](../../CONVENTIONS.md)
- [GOVERNANCE.md](../../GOVERNANCE.md)
- [Evolution Engine](../evolution/SKILL.md)

## HISTORY

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-07-10 | Initial version. Skill lifecycle, composition chains, registry pointer. |
