# Event-Driven Architecture

> **Extracted from**: KERNEL.md section 4 | **Lines**: ~813 | **Date**: 2026-07-28

## Overview

Every behavior in the Cosca Runtime is published as an event on the Event Bus. This provides full observability, audit trails, and enables reactive programming patterns.

## Event Types

| Category | Events | Description |
|----------|--------|-------------|
| **Lifecycle** | Started, Stopped, Paused, Resumed | Runtime state changes |
| **Task** | TaskQueued, TaskStarted, TaskCompleted, TaskFailed | Workflow execution |
| **Memory** | MemoryStored, MemoryPromoted, MemoryPruned | Memory operations |
| **Knowledge** | KnowledgeIndexed, KnowledgeSynced, ConflictDetected | Knowledge store |
| **Provider** | ProviderCalled, ProviderFailed, ProviderRecovered | AI provider |
| **Security** | AuthSuccess, AuthFailed, SecretAccessed, JailActivated | Security events |

## Event Schema

```json
{
  "id": "evt_abc123",
  "type": "TaskCompleted",
  "timestamp": "2026-07-28T10:30:00Z",
  "source": "pipeline/steprunner",
  "data": {
    "taskId": "task_xyz",
    "duration": 1523,
    "status": "success"
  },
  "metadata": {
    "correlationId": "corr_123",
    "agent": "cosca-backend"
  }
}
```

## Event Bus Implementation

- **In-process**: Channel-based for single binary
- **Optional Redis**: For multi-instance deployments
- **Buffered**: 10,000 events in memory, persisted to SQLite

## Event Consumers

| Consumer | Purpose | Replay |
|----------|---------|--------|
| **Trace Store** | Audit trail | Yes |
| **Metrics Collector** | Aggregation | No |
| **Dashboard SSE** | Real-time UI | No |
| **Knowledge Sync** | Learning extraction | Yes |
| **Recovery Engine** | Crash recovery | Yes |

## Event Ordering

Events are **totally ordered** within a single Runtime instance. Cross-instance ordering uses vector clocks.

## Event Retention

- **Hot**: Last 1 hour in memory
- **Warm**: Last 24 hours in SQLite
- **Cold**: Archived after 7 days
