---
name: architecture-documentation
description: Use when the user asks to generate architecture documentation (overviews, module diagrams, dependency maps) from codebase analysis.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Last Updated**: 2026-07-23

# ARCHITECTURE DOCUMENTATION SKILL

## Description
Generate comprehensive architecture documentation from codebase analysis. Creates architecture overviews, module diagrams, dependency graphs, data flow diagrams, and deployment architecture documentation.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| codebase_path | Yes | Path to codebase to document |
| doc_format | Yes | `markdown`, `adoc`, `mermaid`, `plantuml` |
| sections | No | `overview`, `modules`, `dependencies`, `data-flow`, `deployment`, `all` |
| include_diagrams | No | Generate Mermaid/PlantUML diagrams (default: true) |

## Outputs
| Output | Description |
|--------|-------------|
| Architecture document | Complete architecture documentation |
| Module diagrams | Component and module diagrams |
| Dependency graphs | Dependency relationship graphs |
| Data flow diagrams | Data flow through the system |
| Deployment diagrams | Infrastructure and deployment view |

## Documentation Sections

### Architecture Overview
- System purpose and scope
- Architecture style (layered, hexagonal, microservices, etc.)
- Key design decisions and ADRs
- Technology stack overview

### Module Architecture
- Module boundaries and responsibilities
- Module dependency graph
- Interface contracts between modules
- Module ownership

### Data Flow
- Data flow diagrams (logical and physical)
- Data stores and their purposes
- Data transformation pipelines
- Event flows and message routing

### Deployment Architecture
- Infrastructure components
- Network topology
- Deployment environments
- Scaling strategy

### Security Architecture
- Authentication and authorization flow
- Network security boundaries
- Data encryption (at rest and in transit)
- Secrets management

## Process
1. Scan codebase structure and module organization
2. Extract module boundaries and dependencies
3. Analyze data flow through the system
4. Map infrastructure and deployment configuration
5. Generate architecture documentation sections
6. Create diagrams for visual representation
7. Cross-reference with existing ADRs
8. Validate documentation for completeness

## Success Criteria
- [ ] All modules documented with responsibilities
- [ ] Dependency graphs generated and accurate
- [ ] Data flow documented end-to-end
- [ ] Deployment architecture mapped
- [ ] Diagrams included for complex relationships
- [ ] ADRs referenced where applicable

## Related
- [Architecture Chief](../../departments/architecture/SKILL.md)
- [Architecture Analysis](./ARCHITECTURE_ANALYSIS.md)
- [ADR Generation](./ADR_GENERATION.md)
- [Documentation Chief](../../departments/documentation/SKILL.md)
