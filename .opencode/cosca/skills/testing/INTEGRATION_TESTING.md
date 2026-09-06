---
name: integration-testing
description: Use when the user asks to write integration tests validating interactions between components, services, databases, and external systems.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Testing Chief | **Last Updated**: 2026-07-23

# INTEGRATION TESTING SKILL

## Description
Plan, implement, and execute integration tests that validate interactions between system components, services, databases, and external dependencies in realistic environments.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| components | Yes | Components/services to test integration between |
| test_scope | Yes | `internal` (internal services), `external` (3rd party), `all` |
| environment | No | `dev`, `staging`, `ci` (default: ci) |
| test_framework | No | `jest`, `pytest`, `go-test`, `junit` (auto-detected) |

## Outputs
| Output | Description |
|--------|-------------|
| Integration tests | Test implementations for service interactions |
| Test results | Pass/fail with diagnostics per test |
| Coverage report | Integration coverage (which service pairs are tested) |
| Performance metrics | Response times and throughput for integrations |

## Test Categories

### Service-to-Service
- REST API calls between services
- gRPC service calls
- Event/message queue interactions
- Database read/write through repositories
- Cache interaction patterns

### External Integration
- Third-party API calls (with test doubles or sandbox)
- Webhook delivery and reception
- Authentication provider integration
- Payment gateway integration
- Email/SMS delivery verification

### Data Integration
- Database schema compatibility
- Data format transformation
- Event schema compatibility
- File/S3 storage integration
- Stream processing integration

## Process
1. Identify integration points between components
2. Define integration test scenarios
3. Set up test infrastructure (testcontainers, localstack)
4. Configure service dependencies in test environment
5. Implement service-to-service tests
6. Implement external integration tests (with mocks/sandbox)
7. Implement database integration tests
8. Execute tests in CI/CD environment
9. Measure integration coverage
10. Generate report with diagnostics

## Success Criteria
- [ ] All integration points covered
- [ ] Tests use realistic test data
- [ ] External services mocked or sandboxed
- [ ] CI/CD integration configured
- [ ] Integration coverage reported
- [ ] Performance baselines established

## Related
- [Testing Chief](../../departments/testing/SKILL.md)
- [Unit Testing](./UNIT_TESTING.md)
- [E2E Testing](./E2E_TESTING.md)
- [Contract Testing](./CONTRACT_TESTING.md)
- [DevOps Chief](../../departments/devops/SKILL.md)
