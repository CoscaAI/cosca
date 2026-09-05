---
type: pattern
key: architecture-patterns
tags: [pattern, architecture, microservices]
timestamp: 2026-07-23T00:00:00Z
status: active
agent: Architecture Chief
category: architecture
confidence: 0.9
times_used: 15
times_succeeded: 14
---

# Architecture Patterns Catalog

## Pattern: API Gateway
- **Context**: Multiple microservices with different APIs
- **Solution**: Single entry point (Kong/APIGW) routing to services
- **Benefits**: Unified auth, rate limiting, routing, monitoring
- **Success Rate**: 93% (14/15 projects)

## Pattern: Saga (Choreography)
- **Context**: Distributed transaction across microservices
- **Solution**: Each service publishes events, compensating actions on failure
- **Benefits**: No coordinator, eventual consistency, high availability
- **Success Rate**: 87% (13/15 projects)

## Pattern: CQRS
- **Context**: High read/write disparity in same data store
- **Solution**: Separate read models (materialized views) from write models
- **Benefits**: Optimized read performance, independent scaling
- **Success Rate**: 100% (3/3 projects)
