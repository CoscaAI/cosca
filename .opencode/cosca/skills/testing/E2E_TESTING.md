---
name: e2e-testing
description: Use when the user asks to plan, implement, and execute end-to-end tests validating complete user workflows across all system components.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Testing Chief | **Last Updated**: 2026-07-23

# E2E TESTING SKILL

## Description
Plan, implement, and execute end-to-end tests that validate complete user workflows across all system components, from UI to database, including external integrations.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| workflows | Yes | List of critical user workflows to test |
| environment | No | `dev`, `staging`, `production` (default: staging) |
| browser | No | `chromium`, `firefox`, `webkit` (default: chromium) |
| test_framework | No | `playwright`, `cypress`, `selenium`, `testcafe` (default: playwright) |

## Outputs
| Output | Description |
|--------|-------------|
| E2E test suite | Comprehensive end-to-end tests |
| Test results | Pass/fail per workflow with trace |
| Visual evidence | Screenshots, videos, and traces for failures |
| Coverage report | User workflow coverage matrix |

## Test Design Principles
- Test critical user journeys (happy paths) first
- Each test validates one complete workflow
- Tests are independent (no shared state)
- Test data is created and cleaned up per test
- Flaky tests are immediately quarantined
- Tests run in CI/CD on every deployment

## Process
1. Identify critical user workflows with Product Chief
2. Define test scenarios for each workflow
3. Set up test infrastructure (browser automation)
4. Implement page objects for UI components
5. Create test data fixtures
6. Implement workflow tests
7. Configure CI/CD integration
8. Run tests in parallel across browsers
9. Handle flaky tests with retry mechanism
10. Generate report with screenshots and traces

## Success Criteria
- [ ] All critical workflows covered
- [ ] Tests independent and deterministic
- [ ] CI/CD integration configured
- [ ] Visual evidence on failure
- [ ] Test execution time within budget
- [ ] Flaky tests handled with auto-retry

## Related
- [Testing Chief](../../departments/testing/SKILL.md)
- [Unit Testing](./UNIT_TESTING.md)
- [Integration Testing](./INTEGRATION_TESTING.md)
- [Contract Testing](./CONTRACT_TESTING.md)
- [QA Chief](../../departments/qa/SKILL.md)
