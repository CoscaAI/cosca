---
type: architecture
key: database-architecture
tags: [database, architecture, data-model]
timestamp: 2026-07-23T00:00:00Z
status: active
agent: Database Chief
---

# Database Architecture

## Primary Database
- PostgreSQL 16 (Amazon RDS Multi-AZ)
- 32 vCPU, 256GB RAM, 4TB SSD
- Point-in-time recovery enabled (7-day window)
- Read replicas in each availability zone

## Database per Service Pattern
Each service owns its database schema:
- auth-service: users, roles, permissions
- billing-service: invoices, payments, subscriptions
- notification-service: templates, delivery_logs
- analytics-service: events, aggregations

## Caching Layer
- Redis 7 (ElastiCache, cluster mode)
- Primary cache: API response cache (TTL: 60s)
- Session store: User sessions (TTL: 24h)
- Rate limiter: Sliding window counters
- Cache hit rate target: > 85%

## Vector Database
- pgvector extension in PostgreSQL
- 1536-dimensional embeddings
- IVFFlat index with 100 lists
- Used for: semantic search, RAG retrieval
