> **Version**: 1.0.0 | **Status**: active | **Owner**: Review Chief | **Last Updated**: 2026-07-23
>
> # CODE REVIEW SKILL
>
> ## Description
> Use this skill to perform comprehensive code reviews. Covers architecture compliance, code quality, security, performance, testing, and documentation aspects. Follows the quality gates defined in QUALITY_GATES.md.
>
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | code_diff | Yes | Code changes to review |
> | review_depth | Yes | `quick`, `standard`, `full` |
> | focus_areas | No | `architecture`, `security`, `performance`, `testing`, `docs` |
>
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Review report | Comprehensive review findings |
> | Issues list | Issues found with severity and location |
> | Score | Quality score (0-10) per QUALITY_GATES.md |
> | Pass/fail | Overall review decision |
>
> ## Review Dimensions
>
> ### Architecture (Gate 2.1)
> - Module boundaries respected
> - Dependency direction correct
> - ADR compliance
> - Pattern consistency
>
> ### Code Quality (Gate 2.2)
> - SOLID principles
> - DRY (duplication < 5%)
> - Function length < 50 lines
> - File length < 300 lines
> - Cyclomatic complexity < 10
> - No dead code
> - No magic numbers
>
> ### Security (Gate 2.3)
> - OWASP Top 10 check
> - No hardcoded secrets
> - Input validation present
> - Output encoding correct
> - Parameterized queries
> - Authentication/authorization checks
>
> ### Performance (Gate 2.4)
> - N+1 queries check
> - Missing indexes
> - Lazy/eager loading review
> - Synchronous blocking in async contexts
>
> ### Testing (Gate 2.5)
> - Line coverage > 80%
> - Happy path tested
> - Edge cases tested
> - Error paths tested
> - Test independence
>
> ### Documentation (Gate 2.6)
> - API docs updated
> - ADR created if needed
> - Changelog updated
> - Code comments meaningful
> - No TODOs without issue reference
>
> ## Success Criteria
> - [ ] All relevant dimensions reviewed
> - [ ] Issues categorized by severity
> - [ ] Score calculated per QUALITY_GATES.md
> - [ ] Actionable feedback provided
> - [ ] Pass/fail recommendation clear
>
> ## Related
> - [QUALITY_GATES.md](../../QUALITY_GATES.md) — Quality gate definitions
> - [Review Chief](../../departments/review/SKILL.md) — Review ownership
> - [Security Chief](../../departments/security/SKILL.md) — Security review
> - [Performance Chief](../../departments/performance/SKILL.md) — Performance review

## Process
1. **Load Diff**: Obtain the git diff or PR changeset for review.
2. **Dimension Gate 1 — Security**: Check for hardcoded secrets, missing input validation, SQL injection, broken auth (OWASP Top 10).
3. **Dimension Gate 2 — Correctness**: Verify error handling, nil checks, concurrency safety, context propagation.
4. **Dimension Gate 3 — Architecture**: Check for circular deps, layer violations, interface contracts, SOLID principles.
5. **Dimension Gate 4 — Performance**: Flag N+1 queries, missing indexes, unnecessary allocations, blocking operations.
6. **Dimension Gate 5 — Style**: Verify naming conventions, Go idioms, DRY principle, dead code.
7. **Dimension Gate 6 — Testing**: Confirm test coverage for changed code, table-driven tests, edge cases tested.
8. **Compile Report**: Output structured review with severity (CRITICAL/HIGH/MEDIUM/LOW) per finding.
