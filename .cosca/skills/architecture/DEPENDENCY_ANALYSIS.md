> **Version**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Last Updated**: 2026-07-23
> 
> # DEPENDENCY ANALYSIS SKILL
> 
> ## Description
> Use this skill to analyze dependency graphs in the codebase. Detects circular dependencies, excessive coupling, dependency direction violations, and provides recommendations for dependency management.
> 
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | dependency_manifest | Yes | Package manifest (package.json, pom.xml, etc.) |
> | codebase_path | Yes | Path to source code |
> | depth | No | Analysis depth: `basic`, `deep` |
> 
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Dependency graph | Complete dependency map |
> | Circular dependencies | List of circular dependency chains |
> | Coupling metrics | Afferent/efferent coupling per module |
> | Recommendations | Dependency optimization suggestions |
> 
> ## Process
> 1. Resolve all dependency manifests
> 2. Map internal module dependencies
> 3. Detect circular dependency chains
> 4. Measure coupling metrics (Ca, Ce, Instability)
> 5. Analyze dependency direction against architecture rules
> 6. Check for duplicate or conflicting dependencies
> 7. Generate dependency report with visual graph
> 
> ## Success Criteria
> - [ ] All dependencies resolved and mapped
> - [ ] Circular dependencies detected and reported
> - [ ] Coupling metrics calculated
> - [ ] Dependency direction validated
> - [ ] Recommendations provided
> 
> ## Related
> - [Architecture Chief](../../departments/architecture/SKILL.md)
> - [Architecture Validation](./ARCHITECTURE_VALIDATION.md)
> - [Technical Debt Chief](../../departments/technical-debt/SKILL.md)
