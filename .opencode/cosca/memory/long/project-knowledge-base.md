---
type: long
key: project-knowledge-base
tags: [knowledge, architecture, patterns, cross-project]
timestamp: 2026-07-23T00:00:00Z
status: active
agent: Memory Chief
---

# Cross-Project Knowledge Base

## Architecture Patterns
- Microservices with event-driven communication (recommended for distributed systems)
- Clean Architecture with domain-driven design (recommended for complex business logic)
- Hexagonal Architecture for service boundaries (recommended for testability)

## Technology Decisions
- TypeScript for backend services (NestJS preferred framework)
- PostgreSQL as primary database (with pgvector for embeddings)
- Redis for caching and session management
- Apache Kafka for event streaming
- React + Next.js for frontend applications

## Lessons Learned
- Always define API contracts before implementation (contract-first)
- Database migrations must be reversible and tested in staging
- Secrets rotation should be automated with dual-lifecycle pattern
- Load testing must be part of CI/CD pipeline

## Anti-Patterns to Avoid
- Shared database between microservices (leads to coupling)
- God classes (> 500 lines) — split by responsibility
- Synchronous communication between critical services
- Hardcoded configuration values
- Mixed error response formats across APIs
