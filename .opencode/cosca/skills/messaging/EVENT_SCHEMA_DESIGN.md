# Event Schema Design

> **Version**: 1.0.0 | **Status**: active | **Owner**: Messaging Chief | **Last Updated**: 2026-07-27

## Purpose
Design event contracts with versioning, serialization formats, and compatibility guarantees.

## Process
1. Define event envelope: event_id (UUID), event_type, timestamp, version, payload.
2. Choose serialization: JSON for debugging, MsgPack for performance, Protobuf for gRPC.
3. Design payload schema per event type with required/optional fields.
4. Implement versioning: add fields (forward compatible), never remove fields (breaking).
5. Document event catalog: all event types, schemas, producers, consumers.
6. Test compatibility: old consumer reads new event, new consumer reads old event.

## Success Criteria
- All events versioned with semantic versioning
- Forward and backward compatibility verified
- Event catalog documented and searchable
