# Queue Patterns

> **Version**: 1.0.0 | **Status**: active | **Owner**: Messaging Chief | **Last Updated**: 2026-07-27

## Purpose
Implement pub/sub, dead letter queues, retry logic, and message ordering guarantees.

## Process
1. Choose pattern: pub/sub (fan-out), point-to-point (worker queue), request/reply (RPC).
2. Implement publisher: message serialization, routing key, idempotency key.
3. Implement consumer: idempotent handler, acknowledgment, retry with backoff.
4. Configure dead letter queue: max retries → DLQ for manual inspection.
5. Add observability: messages published/consumed/failed metrics, queue depth gauge.
6. Test: duplicate messages (idempotency), network failures (retry), consumer crash (ack recovery).

## Success Criteria
- Zero duplicate processing with idempotency keys
- Failed messages reach DLQ after max retries
- Queue depth remains bounded under sustained load
