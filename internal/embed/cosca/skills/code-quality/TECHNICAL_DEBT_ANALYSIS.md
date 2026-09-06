---
name: technical-debt-analysis
description: Use when the user asks to identify, measure, categorize, and prioritize technical debt across a codebase.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Technical Debt Chief | **Last Updated**: 2026-07-23

# TECHNICAL DEBT ANALYSIS SKILL

## Description
Identify, measure, categorize, and prioritize technical debt across the codebase. Provides quantitative debt scores, actionable remediation plans, and trend tracking to systematically reduce debt over time.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| codebase_path | Yes | Path to codebase to analyze |
| analysis_depth | No | `quick` (fast, surface-level), `standard` (default), `deep` (thorough) |
| debt_categories | No | `complexity`, `coverage`, `duplication`, `dependency`, `documentation`, `style` |

## Outputs
| Output | Description |
|--------|-------------|
| Debt inventory | Cataloged technical debt items with locations |
| Debt score | Quantitative score (0-100, lower is better) |
| Prioritized backlog | Debt items ranked by effort vs impact |
| Trend report | Debt score changes over time |
| Remediation plan | Concrete steps to reduce debt |

## Debt Categories
### Complexity Debt
- Functions exceeding cyclomatic complexity threshold (> 10)
- Functions exceeding cognitive complexity (> 15)
- Deeply nested conditionals
- Long methods (> 50 lines)
- God classes (> 300 lines)

### Coverage Debt
- Modules below coverage threshold (< 80%)
- Untested error paths
- Missing edge case tests
- Integration test gaps

### Duplication Debt
- Code duplication > 5% in changed files
- Repeated logic across modules
- Copy-pasted API clients
- Duplicate configuration

### Dependency Debt
- Outdated major version dependencies
- Deprecated packages in use
- Excessive transitive dependencies
- Circular dependencies

### Documentation Debt
- Public APIs without documentation
- Missing ADRs for architecture decisions
- Stale README files
- TODO/FIXME without issue references

### Style Debt
- Violations of coding standards
- Inconsistent naming conventions
- Mixed formatting across codebase
- Dead/commented-out code

## Process
1. Scan codebase for complexity metrics
2. Analyze test coverage data
3. Detect code duplication patterns
4. Audit dependency health and freshness
5. Check documentation coverage
6. Evaluate coding style consistency
7. Calculate overall debt score
8. Prioritize items by effort/impact ratio
9. Generate debt inventory with location links
10. Create remediation plan with estimated effort

## Success Criteria
- [ ] All debt categories analyzed
- [ ] Debt score calculated via golangci-lint + gocyclo + deadcode analysis. Baseline recorded in .opencode/cosca/memory/evolution/learnings.md. Score must decrease or remain stable in subsequent analyses.
- [ ] Prioritized backlog generated
- [ ] Remediation plan with effort estimates
- [ ] Trend tracking established for future comparison

## Tools
| Tool | Purpose | Command |
|------|---------|---------|
| golangci-lint | Multi-linter debt scan | `golangci-lint run --enable-all --max-issues-per-linter 0` |
| gocyclo | Complexity hotspots | `gocyclo -over 15 .` |
| deadcode | Unused code detection | `deadcode ./...` |
| go vet -shadow | Variable shadowing | `go vet -shadow ./...` |
| git log | Change frequency (hot files) | `git log --format=oneline -- "*.go" \| cut -d" " -f1 \| sort \| uniq -c` |

## Related
- [Technical Debt Chief](../../departments/technical-debt/SKILL.md)
- [Complexity Analysis](./COMPLEXITY_ANALYSIS.md)
- [Refactoring](./REFACTORING.md)
- [workflows/technical-debt-paydown.md](../../workflows/technical-debt-paydown.md)
- [Code Review](./CODE_REVIEW.md)
