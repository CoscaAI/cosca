---
type: pattern
key: testing-patterns
tags: [pattern, testing, quality]
timestamp: 2026-07-23T00:00:00Z
status: active
agent: QA Chief
category: testing
confidence: 0.9
times_used: 25
times_succeeded: 23
---

# Testing Patterns

## Pattern: Test Pyramid
- **Context**: Balanced test suite
- **Solution**: 70% unit, 20% integration, 10% E2E
- **Benefits**: Fast feedback, high confidence, maintainable

## Pattern: AAA (Arrange-Act-Assert)
- **Context**: Unit test structure
- **Solution**: Three-section test with clear boundaries
- **Benefits**: Readable, maintainable, failure diagnosis

## Pattern: Consumer-Driven Contracts
- **Context**: Service API compatibility
- **Solution**: Consumers define expectations, providers verify
- **Benefits**: Safe independent deployments, early breakage detection
