---
name: evolution
description: Continuously analyzes the codebase for improvement opportunities - debt, refactors, and quality.
level: 2
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Evolution Engine | **Last Updated**: 2026-07-10

# EVOLUTION ENGINE

## PURPOSE
The Evolution Engine continuously analyzes the codebase and Cosca itself for improvement opportunities. It identifies technical debt, suggests refactorings, and drives the system toward higher quality.

## EVOLUTION ANALYSIS

### 1. Code Smell Detection
- Long methods (> 50 lines)
- Long parameter lists (> 5 params)
- Large classes/modules (> 300 lines)
- Duplicate code (> 10 lines similar)
- Dead code (unused exports, unreachable code)
- Commented-out code
- Magic numbers and strings
- Feature envy (method using another class's data excessively)
- Data clumps (same group of params appearing together)
- Primitive obsession (using primitives instead of value objects)
- Switch statements (polymorphism opportunity)
- Temporary fields
- Refused bequest (subclass not using inherited methods)

### 2. Architecture Smell Detection
- Circular dependencies between modules
- Violated layer boundaries
- Package/module coupling too high
- God modules (too many responsibilities)
- Scattered functionality (same concern across many modules)
- Architectural drift (code not following defined patterns)
- Missing abstractions
- Wrong abstractions

### 3. Performance Smell Detection
- N+1 query patterns
- Missing database indexes
- Unnecessary object allocations in hot paths
- Missing caching opportunities
- Synchronous operations that could be async
- Large bundle sizes
- Unoptimized images
- Render-blocking resources

### 4. Security Smell Detection
- Hardcoded secrets
- Missing input validation
- Missing output encoding
- Weak cryptography
- Outdated dependencies with vulnerabilities
- Overly permissive CORS
- Missing security headers
- Debug mode enabled in production

### 5. Test Smell Detection
- Tests without assertions
- Overly mocked tests
- Fragile tests (dependent on implementation details)
- Slow tests
- Missing edge case tests
- Missing error case tests
- Test file not near source file

### 6. Documentation Smell Detection
- Undocumented public APIs
- Outdated README
- Missing ADR for architectural decisions
- Comments that explain "what" instead of "why"
- TODOs and FIXMEs older than 30 days

## EVOLUTION REPORT

```markdown
# EVOLUTION REPORT — [Project]

## HEALTH SCORE: [A|B|C|D|F]

## CRITICAL ISSUES (Must Fix)
1. [Issue] — [Impact] — [Fix complexity: S/M/L]

## HIGH PRIORITY (Should Fix)
1. [Issue] — [Impact] — [Fix complexity]

## MEDIUM PRIORITY (Consider Fixing)
1. [Issue] — [Impact] — [Fix complexity]

## LOW PRIORITY (Nice to Have)
1. [Issue] — [Impact] — [Fix complexity]

## REFACTORING CANDIDATES
1. [Module/Class] — [Why] — [Suggested approach]

## ARCHITECTURE IMPROVEMENTS
1. [Suggestion] — [Rationale]

## Cosca SELF-IMPROVEMENT
1. [Process improvement] — [Expected benefit]

## TREND ANALYSIS
| Metric | Last Month | This Month | Trend |
|--------|-----------|------------|-------|
| Code Smells | N | N | ↑↓→ |
| Test Coverage | X% | X% | ↑↓→ |
| Duplication | X% | X% | ↑↓→ |
| Tech Debt Ratio | X% | X% | ↑↓→ |
```

## AUTO-FIX CAPABILITIES
The Evolution Engine can automatically fix:
- Unused imports
- Missing type annotations
- Simple code formatting
- Obvious dead code removal
- Simple naming improvements
- TODO-to-issue conversion

For complex refactorings, it generates a plan and routes to Refactoring Workflow.

## SCHEDULE
- Quick scan: After every task completion
- Deep scan: Weekly
- Full audit: Monthly
- Cosca self-review: Monthly

## DEPENDENCIES
- Triggered by Kernel periodically
- Feeds into Planning Engine for improvement stories
- Stores results in Memory Engine
- Coordinates with Review Chief for validation

## RELATED
- [Planning Engine](../planning/SKILL.md) — Receives improvement stories
- [Learning Engine](../learning/SKILL.md) — Feeds recommendations for self-improvement
- [Observability Engine](../observability/SKILL.md) — Provides trend data

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Added metadata, HISTORY, and cross-references |
