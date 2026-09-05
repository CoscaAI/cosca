# MICROSERVICES TEMPLATE

## Domain
Microservices architecture with API Gateway. Event-driven communication.

## Recommended Stack
- Gateway: Kong, Traefik, or custom (FastAPI/NestJS)
- Services: Any language per service (polyglot)
- Message Broker: RabbitMQ, Kafka, or NATS
- Database: One database per service
- Service Mesh: Istio or Linkerd (optional)
- Observability: OpenTelemetry, Jaeger, Prometheus, Grafana
- Container Orchestration: Kubernetes or Docker Compose (dev)

## Module Structure
```
services/
├── gateway/           # API Gateway (routing, auth, rate limiting)
├── auth/              # Authentication service
├── users/             # User management service
├── products/          # Product catalog service
├── orders/            # Order management service
├── payments/          # Payment processing service
├── notifications/     # Notification service (email, push, SMS)
├── search/            # Search indexing service
└── analytics/         # Analytics and reporting service
packages/
├── contracts/         # Service contracts (protobuf, OpenAPI, JSON Schema)
├── shared/            # Shared utilities (logging, types)
└── testing/           # Integration test utilities
infrastructure/
├── docker/
├── k8s/               # Kubernetes manifests
├── terraform/         # Infrastructure as Code
└── helm/              # Helm charts
```

## Key Features
- API Gateway with routing and auth
- Service discovery
- Distributed tracing
- Circuit breakers
- Event-driven communication
- Saga pattern for distributed transactions
- CQRS where needed
- Centralized logging
- Health checks per service
- Graceful degradation

## Architecture Patterns
- Database per service
- Event sourcing (where applicable)
- CQRS (for read-heavy services)
- Saga for distributed transactions
- Outbox pattern for reliable events
- API Gateway / BFF pattern
- Service mesh for observability
- Sidecar pattern for cross-cutting concerns

## Communication Patterns
| Pattern | Use Case | Technology |
|---------|----------|------------|
| Synchronous REST/gRPC | Request-response | Gateway → Services |
| Asynchronous Events | State changes | RabbitMQ / Kafka |
| Event Sourcing | Audit trail | Event store |
| CQRS | Read optimization | Read model DB |
| Saga | Distributed transactions | Orchestrator |
