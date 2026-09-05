# Runtime State Machine

> **Extracted from**: KERNEL.md §3 | **Lines**: ~750 | **Date**: 2026-07-28

## States

The Cosca Runtime operates as a finite state machine with the following states:

```
                    ┌──────────────┐
                    │   CREATED    │
                    └──────┬───────┘
                           │ Init()
                           ▼
                    ┌──────────────┐
         ┌─────────│  INITIALIZING │─────────┐
         │         └──────┬───────┘          │
         │                │ Ready()          │
         │                ▼                  │
         │         ┌──────────────┐          │
         │         │    READY     │          │
         │         └──────┬───────┘          │
         │                │                  │
         │      ┌─────────┼─────────┐        │
         │      ▼         ▼         ▼        │
         │ ┌────────┐ ┌────────┐ ┌────────┐  │
         │ │BUSY    │ │PAUSED  │ │ERROR   │  │
         │ └───┬────┘ └───┬────┘ └───┬────┘  │
         │     │          │          │        │
         │     └──────────┼──────────┘        │
         │                │                  │
         │                ▼                  │
         │         ┌──────────────┐          │
         └────────▶│  SHUTTING    │◀─────────┘
                   │    DOWN      │
                   └──────┬───────┘
                          │ Stopped()
                          ▼
                   ┌──────────────┐
                   │   STOPPED    │
                   └──────────────┘
```

## State Transitions

| From | To | Trigger | Guard |
|------|----|---------|-------|
| CREATED | INITIALIZING | `Init()` | Config loaded |
| INITIALIZING | READY | `Ready()` | All services started |
| READY | BUSY | `Execute()` | Request received |
| READY | PAUSED | `Pause()` | Admin command |
| READY | SHUTTING_DOWN | `Shutdown()` | Signal received |
| BUSY | READY | `Complete()` | Task finished |
| BUSY | ERROR | `Fail()` | Unrecoverable error |
| ERROR | READY | `Recover()` | Error handled |
| PAUSED | READY | `Resume()` | Admin command |
| SHUTTING_DOWN | STOPPED | `Stopped()` | Cleanup done |

## State Properties

| State | Accepts Requests | Emits Events | Background Tasks |
|-------|-----------------|--------------|------------------|
| CREATED | No | No | No |
| INITIALIZING | No | Yes (InitProgress) | Yes |
| READY | Yes | Yes (Ready) | Yes |
| BUSY | Yes (queued) | Yes (TaskProgress) | Yes |
| PAUSED | No | Yes (Paused) | No |
| ERROR | No | Yes (Error) | No |
| SHUTTING_DOWN | No | Yes (ShuttingDown) | Draining |
| STOPPED | No | No | No |

## Health Checks

Each state has associated health checks:

- **READY**: All dependencies connected, knowledge.db accessible
- **BUSY**: Task progress being reported, no deadlocks
- **ERROR**: Error details available, recovery possible
- **SHUTTING_DOWN**: Graceful drain in progress

## Recovery from ERROR

1. Log error details to trace store
2. Emit `Error` event with context
3. Attempt automatic recovery if configured
4. If recovery fails, require manual intervention
5. On recovery, transition to READY
