---
name: architecture-analysis
description: Use when the user asks to analyze system architecture, detect patterns, and find violations or architectural drift.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Last Updated**: 2026-07-23
> 
> # ARCHITECTURE ANALYSIS SKILL
> 
> ## Description
> Use this skill when you need to analyze the system architecture of a codebase. This includes detecting architecture patterns, identifying modular boundaries, mapping dependencies, finding architecture violations, and generating architecture documentation.
> 
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | workspace_path | Yes | Path to the codebase to analyze |
> | analysis_depth | No | `quick` (default), `standard`, `deep` |
> | focus_areas | No | Comma-separated: `boundaries`, `dependencies`, `patterns`, `violations` |
> 
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Architecture report | Complete architecture analysis |
> | Dependency graph | Module dependency relationships |
> | Violations list | Architecture violations detected |
> | Pattern inventory | Architecture patterns in use |
> 
> ## Process
> 1. Scan project structure for module organization
> 2. Detect architecture patterns (layered, hexagonal, clean, etc.)
> 3. Analyze dependency direction between modules
> 4. Identify circular dependencies and boundary violations
> 5. Map data flow through the system
> 6. Detect technology choices and framework usage
> 7. Generate architecture report with findings
> 8. Prioritize violations by severity
> 
> ## Success Criteria
> - [ ] All architecture patterns identified (verified against .opencode/cosca/memory/pattern/). At least 90% pattern coverage — fewer than 10% of modules unclassified.
> - [ ] Module boundaries mapped accurately
> - [ ] Circular dependencies detected
> - [ ] Dependency direction validated
> - [ ] Report generated with actionable findings
> 
> ## Related
> - [Architecture Chief](../../departments/architecture/SKILL.md)
> - [Architecture Validation](./ARCHITECTURE_VALIDATION.md)
> - [ADR Generation](./ADR_GENERATION.md)
> - [Discovery Chief](../../departments/discovery/SKILL.md)
