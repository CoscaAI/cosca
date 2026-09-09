---
type: architecture
key: event-architecture
tags: [events, messaging, kafka]
timestamp: 2026-07-23T00:00:00Z
status: active
agent: Messaging Chief
---

# Event-Driven Architecture

## Message Broker
- Apache Kafka 3.5 (Confluent Cloud)
- 6 partitions per topic (default)
- Replication factor: 3
- Retention: 7 days (compacted for keyed topics)

## Event Schema Registry
- Avro serialization with Schema Registry
- Backward compatibility enforced
- 42 registered schemas (and growing)
- Schema evolution: FULL_TRANSITIVE compatibility

## Event Categories
| Domain | Topics | Producer | Consumers |
|--------|--------|----------|-----------|
| User | user.created, user.updated, user.deleted | auth-service | billing, notification, analytics |
| Billing | invoice.created, payment.received, subscription.changed | billing-service | notification, analytics |
| Orders | order.placed, order.fulfilled, order.cancelled | order-service | billing, notification, inventory |

## Dead Letter Queue
- All topics have DLQ with same partition count
- DLQ retention: 30 days
- Alert on any message in DLQ within 5 minutes
- Automatic retry: 3 attempts with exponential backoff
