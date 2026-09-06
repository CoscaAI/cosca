> **Version**: 1.0.0 | **Status**: active | **Owner**: Technical Debt Chief | **Last Updated**: 2026-07-23
> 
> # COMPLEXITY ANALYSIS SKILL
> 
> ## Description
> Use this skill to analyze code complexity metrics. Calculates cyclomatic complexity, cognitive complexity, maintainability index, and provides recommendations to reduce complexity.
> 
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | code_path | Yes | Path to code to analyze |
> | complexity_threshold | No | Maximum allowed cyclomatic complexity (default: 10) |
> | cognitive_threshold | No | Maximum allowed cognitive complexity (default: 15) |
> 
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Complexity report | Per-function complexity metrics |
> | Hotspots | Functions exceeding thresholds |
> | Recommendations | Specific refactoring suggestions |
> | Trend data | Complexity changes over time |
> 
> ## Metrics Calculated
> - Cyclomatic complexity per function
> - Cognitive complexity per function
> - Maintainability index per module
> - Function length in lines
> - Parameter count per function
> - Nesting depth
> 
> ## Success Criteria
> - [ ] All functions analyzed
> - [ ] Hotspots identified with locations
> - [ ] Recommendations provided
> - [ ] Trend data available for comparison
> 
> ## Related
> - [Technical Debt Chief](../../departments/technical-debt/SKILL.md)
> - [Refactoring](./REFACTORING.md)
> - [QUALITY_GATES.md](../../QUALITY_GATES.md) — Gate 2.2 quality checks

## Process
1. **Select Target**: Identify the file or package to analyze (Go: internal/*, TypeScript: web/src/*).
2. **Parse Source**: Load source files and generate AST (Go: `go/parser`, TS: use TypeScript compiler API or `eslint` with complexity plugin).
3. **Calculate Metrics**: Compute cyclomatic complexity per function, cognitive complexity, lines of code, depth of nesting.
4. **Classify Severity**: Flag functions exceeding thresholds (>10 cyclomatic, >15 cognitive, >50 LOC).
5. **Generate Report**: Output structured report with file:line references, severity levels, and refactoring suggestions.
6. **Track Baseline**: Store results for trend comparison in next analysis.
