> **Version**: 1.0.0 | **Status**: active | **Owner**: Technical Debt Chief | **Last Updated**: 2026-07-23
>
> # REFACTORING SKILL
>
> ## Description
> Use this skill for systematic code refactoring. Analyzes code for improvement opportunities, plans refactoring steps, ensures behavior preservation through tests, and validates the result.
>
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | code_path | Yes | Path to code to refactor |
> | refactoring_type | Yes | `extract-method`, `rename`, `move-class`, `extract-class`, `introduce-parameter`, `replace-conditional`, `compose-method`, `extract-module` |
> | preserve_tests | No | Whether existing tests must pass (default: true) |
>
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Refactored code | Clean, improved code |
> | Refactoring report | Changes made and rationale |
> | Test results | Verification that behavior is preserved |
>
> ## Process
> 1. Analyze current code structure
> 2. Identify refactoring opportunities
> 3. Plan refactoring steps (small, safe increments)
> 4. Ensure test coverage exists before changes
> 5. Execute each refactoring step
> 6. Run tests after each step (red-green-refactor)
> 7. Validate no behavior change
> 8. Update documentation if API changed
>
> ## Refactoring Patterns
> - Extract Method: Break down long methods
> - Rename: Improve naming clarity
> - Move Class: Relocate to correct module
> - Extract Class: Split large classes
> - Introduce Parameter: Reduce hardcoding
> - Replace Conditional with Strategy: Simplify logic
> - Compose Method: Improve method flow
> - Extract Module: Create cohesive modules
>
> ## Success Criteria
> - [ ] All existing tests pass after refactoring
> - [ ] Code complexity reduced (cyclomatic < 10)
> - [ ] No behavior change introduced
> - [ ] Duplication reduced
> - [ ] Naming improved and consistent
> - [ ] Documentation updated
>
> ## Related
> - [Technical Debt Chief](../../departments/technical-debt/SKILL.md)
> - [Code Review](./CODE_REVIEW.md)
> - [Complexity Analysis](./COMPLEXITY_ANALYSIS.md)
> - [workflows/refactoring.md](../../workflows/refactoring.md)
