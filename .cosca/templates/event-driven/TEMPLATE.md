# TEMPLATE: Event-Driven Architecture

> **Version**: 1.0.0 | **Status**: active | **Owner**: Messaging Chief | **Last Updated**: 2026-07-23

## DOMAIN
Event-driven, message-based, and streaming architectures using message brokers and event buses.

## RECOMMENDED STACK
| Layer | Primary | Alternative |
|-------|---------|-------------|
| Message Broker | Apache Kafka | RabbitMQ, AWS SQS/SNS, Google Pub/Sub |
| Schema Registry | Confluent Schema Registry | Apicurio, AWS Glue Schema Registry |
| Event Format | Avro | Protobuf, JSON Schema |
| Stream Processing | Kafka Streams | Apache Flink, Apache Spark |
| Async API | AsyncAPI Specification | — |
| Service Framework | NestJS / Spring Boot | FastAPI, Go Kit |
| Database | PostgreSQL (event store) | MongoDB, EventStoreDB |
| Monitoring | Prometheus + Grafana | Datadog, New Relic |

## MODULE STRUCTURE
```
project-name/
├── events/                    # Event definitions and schemas
│   ├── schemas/               # Avro/Protobuf schema files
│   ├── registry/              # Schema registry config
│   └── catalog/               # Event catalog documentation
├── producers/                 # Event producer services
│   └── service-name/
│       ├── src/
│       └── tests/
├── consumers/                 # Event consumer services
│   └── service-name/
│       ├── src/
│       └── tests/
├── streams/                   # Stream processing jobs
│   └── job-name/
│       ├── src/
│       └── tests/
├── infra/                     # Infrastructure as code
│   ├── kafka/                 # Kafka cluster config
│   └── monitoring/            # Monitoring dashboards
├── docs/
│   ├── ARCHITECTURE.md
│   ├── EVENT_CATALOG.md
│   └── ASYNCAPI.md
├── docker-compose.yml
└── README.md
```

## KEY FEATURES
- Event schema management with Schema Registry
- AsyncAPI specification for event contracts
- Event sourcing and CQRS patterns
- Dead letter queues and error handling
- At-least-once or exactly-once delivery guarantees
- Message tracing and observability (OpenTelemetry)
- Idempotent event processing
- Event versioning and compatibility
- Stream processing for real-time analytics
- Event replay and recovery capabilities

## ARCHITECTURE NOTES
- Events are immutable facts — never modify historical events
- Schemas must be backward compatible within major version
- Each event has a single producer, multiple consumers
- Consumers must be idempotent for at-least-once delivery
- Dead letter queue must be monitored and alerted
- Event size should be under 1MB (prefer references over payload)
- Use event sourcing for audit-critical data
- Schema registry enforces compatibility on produce/consume

## RELATED
- [Messaging Chief](../../departments/messaging/SKILL.md)
- [API Template](../api/TEMPLATE.md)
- [Microservices Template](../microservices/TEMPLATE.md)
- [Skills Catalog](../../skills/SKILLS_CATALOG.md)
