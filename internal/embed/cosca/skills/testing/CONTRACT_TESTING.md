---
name: contract-testing
description: Use when the user asks to implement API contract tests (Pact, Spring Cloud Contract) between a provider and its consumers.
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Testing Chief | **Last Updated**: 2026-07-23

# CONTRACT TESTING SKILL

## Description
Implement contract tests to validate API interactions between services. Ensures provider and consumer agreements are maintained, detects breaking changes early, and enables safe independent deployments.

## Inputs
| Input | Required | Description |
|-------|----------|-------------|
| contract_type | Yes | `pact` (consumer-driven), `spring-cloud-contract`, `openapi` |
| provider_service | Yes | Service providing the API |
| consumer_service | Yes | Service consuming the API |
| contract_path | No | Path to existing contract files |
| provider_base_url | No | Provider base URL for verification |

## Outputs
| Output | Description |
|--------|-------------|
| Contract tests | Consumer and provider contract test implementations |
| Verification report | Contract compatibility verification results |
| Breaking changes | Identified contract incompatibilities |
| Compatible deployments | Verified safe deployment combinations |

## Contract Types

### Consumer-Driven (Pact)
- Consumer defines expected interactions
- Provider verifies it can satisfy all consumer pacts
- Supports HTTP and message-based interactions
- Can be integrated into CI/CD pipeline

### Provider-Driven (Spring Cloud Contract)
- Provider defines the contract
- Consumers use generated stubs
- Tight integration with Spring ecosystem

### OpenAPI Contract Testing
- Contract defined in OpenAPI specification
- Both sides validate against spec
- Schema validation on request/response

## Testing Process
1. Identify service pairs with API dependencies
2. Choose contract testing approach
3. Write contract definitions (pacts, stubs, or specs)
4. Implement consumer tests against contract
5. Implement provider verification tests
6. Run contract verification in CI/CD
7. Detect breaking changes automatically
8. Generate compatibility matrix
9. Archive contracts with version history

## Success Criteria
- [ ] Contracts defined for all service pairs
- [ ] Consumer tests passing against contracts
- [ ] Provider verification passing CI/CD
- [ ] Breaking changes detected and reported
- [ ] Compatibility matrix maintained
- [ ] Contracts versioned and archived

## Related
- [Testing Chief](../../departments/testing/SKILL.md)
- [Unit Testing](./UNIT_TESTING.md)
- [Integration Testing](./INTEGRATION_TESTING.md)
- [E2E Testing](./E2E_TESTING.md)
- [API Chief](../../departments/api/SKILL.md)
- [Backend Chief](../../departments/backend/SKILL.md)
