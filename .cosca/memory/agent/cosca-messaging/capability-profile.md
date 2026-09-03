# cosca-messaging — Capability Profile

> **DNA Version**: 3.0.0 | **Last Updated**: 2026-07-28

## Current Level: 2

## Per-Domain Confidence

| Domain | Confidence | Successful Tasks | Last Outcome | Trend |
|--------|-----------|-----------------|-------------|-------|
| Event system discovery | 0.82 | 2 | success | ↑ |
| Topology mapping | 0.78 | 1 | success | ↑ |
| Pub/sub design | 0.75 | 1 | success | ↑ |
| WebSocket architecture | 0.70 | 1 | success | ↑ |
| Dead letter queues | 0.50 | 0 | — | → |
| External broker integration | 0.45 | 0 | — | → |
| Event sourcing | 0.45 | 0 | — | → |

## Strengths
- Systematic codebase scanning — ability to discover all event systems in a large codebase via grep, glob, and call-site tracing
- Topology mapping — can build complete producer/consumer maps across package boundaries
- Gap analysis against messaging standards (at-least-once, idempotency, DLQ, schema, persistence)

## Weaknesses
- No hands-on experience implementing DLQ or retry patterns in Go
- No experience with external message brokers (Kafka, NATS, RabbitMQ) in Cosca context
- No experience with event sourcing or CQRS implementation

## Preferred Strategies
- Discovery-first: scan all packages before making recommendations
- Standard-driven: evaluate against proven messaging patterns rather than ad-hoc solutions
- Incremental evolution: propose quick wins first, then medium-term architectural changes

## Known Failure Modes
- Overengineering: tendency to propose full event sourcing when simpler solutions suffice — guard by matching solution to actual scale
- Analysis paralysis: can spend too long cataloging vs. implementing — time-box audit phase

## Evolution Goal
Reach Level 3:
"Implement MT-01 unified event bus with Bridge pattern, proving ability to design and execute cross-system messaging infrastructure. Unlock advanced pattern recognition (threat modeling for messaging failures)."
