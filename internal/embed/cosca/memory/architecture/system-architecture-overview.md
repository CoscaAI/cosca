---
type: architecture
key: system-architecture-overview
tags: [architecture, overview, system-design]
timestamp: 2026-07-23T00:00:00Z
status: active
agent: Architecture Chief
---

# System Architecture Overview

## Architecture Style
Event-driven microservices with API gateway, following Clean Architecture principles.

## Core Components
```
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│   Gateway   │──▶│   BFF API   │──▶│   Services   │
│  (Kong)     │   │  (NestJS)   │   │  (Multiple)  │
└─────────────┘   └─────────────┘   └──────┬──────┘
                                           │
┌─────────────┐   ┌─────────────┐   ┌──────▼──────┐
│   Frontend  │   │   Mobile    │   │   Message    │
│  (Next.js)  │   │ (React Nat.)│   │    Broker    │
└─────────────┘   └─────────────┘   │   (Kafka)    │
                                     └──────┬──────┘
                                            │
┌─────────────┐   ┌─────────────┐   ┌──────▼──────┐
│PostgreSQL   │   │   Redis     │   │   Workers    │
│(Primary DB) │   │  (Cache)    │   │  (Microsvc)  │
└─────────────┘   └─────────────┘   └─────────────┘
```

## Integration Points
- REST APIs (internal) via API Gateway
- Events via Apache Kafka (asynchronous)
- gRPC for service-to-service (high throughput)
- WebSocket for real-time updates

## Deployment Architecture
- Kubernetes on AWS EKS
- Helm charts for service deployment
- ArgoCD for GitOps deployment
- Istio for service mesh

## Security Architecture
- Zero Trust Network Architecture
- JWT-based authentication (RS256)
- OAuth2 + OIDC for SSO
- mTLS for service-to-service
- Network policies per namespace
