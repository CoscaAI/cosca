> **Version**: 1.0.0 | **Status**: active | **Owner**: Testing Chief | **Last Updated**: 2026-07-23
>
> # UNIT TESTING SKILL
>
> ## Description
> Use this skill to write unit tests following the AAA (Arrange-Act-Assert) pattern. Ensures tests are independent, deterministic, and provide meaningful coverage.
>
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | source_path | Yes | Path to source file(s) to test |
> | test_framework | Yes | `jest`, `vitest`, `pytest`, `junit`, `mocha`, `go-test` |
> | coverage_target | No | Target coverage percentage (default: 80%) |
>
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Test files | Unit test implementations |
> | Coverage report | Code coverage results |
> | Test run results | All tests passing |
>
> ## Test Structure
>
> ### AAA Pattern
> ```typescript
> describe('ComponentName', () => {
>   describe('methodName', () => {
>     it('should [expected behavior] when [condition]', () => {
>       // Arrange — setup test data and mocks
>       // Act — execute the method under test
>       // Assert — verify the expected outcome
>     });
>   });
> });
> ```
>
> ### Coverage Requirements
> - Line coverage > 80%
> - Branch coverage > 70%
> - Happy path tested
> - At least 2 edge cases
> - At least 1 error path
>
> ### Test Quality Rules
> - Tests must be independent (no shared state)
> - Tests must be deterministic (same result every run)
> - No test depends on another test
> - Mock external dependencies
> - Test one behavior per test
> - Descriptive test names (behavior, not implementation)
>
> ## Success Criteria
> - [ ] All testable functions covered
> - [ ] Coverage target met
> - [ ] All tests passing
> - [ ] Tests follow AAA pattern
> - [ ] Tests are independent and deterministic
> - [ ] Edge and error cases covered
>
> ## Related
> - [Testing Chief](../../departments/testing/SKILL.md)
> - [QA Chief](../../departments/qa/SKILL.md)
> - [QUALITY_GATES.md](../../QUALITY_GATES.md) — Gate 2.5 Testing
> - [Integration Testing](./INTEGRATION_TESTING.md)
