# Recovery Engine

> **Extracted from**: KERNEL.md section 15 | **Lines**: ~691 | **Date**: 2026-07-28

## Overview

The Recovery Engine handles crashes, timeouts, and failures with automatic retry, checkpointing, and rollback.

## Recovery Strategies

| Strategy | When | How |
|----------|------|-----|
| Retry | Transient error | Exponential backoff |
| Checkpoint | Long task | Save progress, resume later |
| Rollback | Partial failure | Undo completed steps |
| Failover | Provider down | Switch to backup provider |
| Resume | Crash recovery | Load last checkpoint |

## Retry Policy

```go
type RetryPolicy struct {
    MaxAttempts  int           // default: 3
    InitialDelay time.Duration // default: 100ms
    MaxDelay     time.Duration // default: 30s
    Multiplier   float64       // default: 2.0
    Jitter       float64       // default: 0.1
}
```

## Checkpoint System

```go
type Checkpoint struct {
    ID        string
    TaskID    string
    Step      int
    State     []byte
    CreatedAt time.Time
}
```

Checkpoints are stored in SQLite with WAL mode for crash safety.

## Recovery Flow

```
Crash Detected
    │
    ▼
┌─────────────┐
│  Load State │  Read last checkpoint
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Validate   │  Check integrity
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Resume     │  Continue from checkpoint
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Complete   │  Finish remaining steps
└─────────────┘
```

## Rollback

If recovery fails:
1. Execute rollback handlers in reverse order
2. Clean up partial artifacts
3. Log failure details
4. Emit RecoveryFailed event
