# Runtime Overview

> **Status**: active | **Owner**: Runtime Chief | **Last Updated**: 2026-07-28 | **Version**: 1.4.0-dev

## Runtime Architecture

The Cosca Runtime is the central orchestrator that manages the application lifecycle, coordinates all subsystems, and provides health reporting, metrics, and event-driven communication.

```
┌──────────────────────────────────────────────────────────────────┐
│                      RUNTIME ENGINE                               │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                    EVENT BUS                              │    │
│  │  Publish / Subscribe — All state changes → events        │    │
│  │  10 event types, 10s handler timeout per subscriber      │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│  │  STATE       │  │  LIFECYCLE   │  │      METRICS         │   │
│  │  MACHINE     │  │  MANAGER     │  │    COLLECTOR         │   │
│  │  8 states    │  │  Init/Start  │  │  8 counters, 4       │   │
│  │  Transitions │  │  Stop/Restart│  │  duration histograms │   │
│  └──────────────┘  └──────────────┘  └──────────────────────┘   │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                 SUBSYSTEM REGISTRY                        │    │
│  │                                                           │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │    │
│  │  │Knowledge │ │Discovery │ │  Memory  │ │  Cache   │   │    │
│  │  │(required)│ │(optional)│ │(optional)│ │(required)│   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐                │    │
│  │  │ Plugins  │ │ Editors  │ │ Watcher  │                │    │
│  │  │(optional)│ │(optional)│ │(optional)│                │    │
│  │  └──────────┘ └──────────┘ └──────────┘                │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                 HEALTH CHECK LOOP                         │    │
│  │  Periodic (30s) → Check all subsystems → Update state    │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                 SIGNAL HANDLER                            │    │
│  │  SIGINT → Graceful stop  |  SIGTERM → Force stop        │    │
│  │  SIGHUP → Config reload event                            │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

---

## State Machine

The runtime follows a formal deterministic state machine with 8 states and validated transitions.

### States

| State | Description |
|-------|-------------|
| `uninitialized` | Initial state before any initialization |
| `initializing` | Runtime is starting up, subsystems initializing |
| `ready` | All subsystems initialized, ready for commands |
| `running` | Runtime is actively running, health checks active |
| `stopping` | Runtime is shutting down gracefully |
| `stopped` | Runtime has shut down (restartable) |
| `error` | Non-recoverable error encountered |
| `recovering` | Runtime attempting to recover from error |

### State Transition Diagram

```
UNINITIALIZED ──→ INITIALIZING ──→ READY ──→ RUNNING ──→ STOPPING ──→ STOPPED
       │                │             │          │            │
       ▼                ▼             ▼          ▼            ▼
     ERROR ←────── RECOVERING ←───── ERROR ←── ERROR ←───── ERROR
       │                │
       ▼                ▼
  UNINITIALIZED       READY
```

### Transition Rules

| From | To | Condition |
|------|----|-----------|
| `uninitialized` | `initializing` | `Start()` called |
| `uninitialized` | `error` | Init failed before start |
| `initializing` | `ready` | All init hooks succeeded |
| `initializing` | `error` | Critical init hook failed |
| `initializing` | `stopping` | Interrupt during init |
| `ready` | `running` | Start hooks succeeded |
| `ready` | `stopping` | Stop requested |
| `ready` | `error` | Startup failure |
| `running` | `stopping` | Stop requested |
| `running` | `error` | Critical subsystem failure |
| `running` | `recovering` | Auto-recovery triggered |
| `running` | `ready` | Hot reload |
| `stopping` | `stopped` | All subsystems stopped |
| `stopping` | `error` | Force stop |
| `stopped` | `uninitialized` | Full reset (re-initialization) |
| `error` | `recovering` | Recovery attempt |
| `error` | `stopping` | Cleanup on unrecoverable |
| `error` | `uninitialized` | Full reset |
| `recovering` | `ready` | Recovery succeeded |
| `recovering` | `error` | Recovery failed |
| `recovering` | `stopping` | Interrupt during recovery |

---

## Component Lifecycle

### Startup Sequence

```
Runtime.Start()
  │
  ├── Lock state mutex
  ├── Verify state is Uninitialized (else error)
  ├── Transition to Initializing
  ├── Publish EventStartupComplete
  ├── Execute Init hooks (sequential, each 30s timeout):
  │   ├── 1. Knowledge (required) — fails → abort
  │   ├── 2. Discovery (optional) — fails → warn, continue
  │   ├── 3. Memory (optional)
  │   ├── 4. Cache (required)
  │   ├── 5. Plugins (optional)
  │   ├── 6. Editors (optional)
  │   └── 7. Watcher (optional)
  ├── Transition to Ready
  ├── Execute Start hooks (same order, publish events)
  ├── Transition to Running
  ├── Start health check loop (goroutine, periodic)
  └── Return success
```

### Shutdown Sequence

```
Runtime.Stop()
  │
  ├── Transition to Stopping
  ├── Create timeout context (ShutdownTimeout, default 60s)
  ├── Execute Stop hooks (reverse order, each 30s timeout):
  │   ├── 1. Watcher
  │   ├── 2. Editors
  │   ├── 3. Plugins
  │   ├── 4. Cache (required)
  │   ├── 5. Memory (optional)
  │   ├── 6. Discovery (optional)
  │   └── 7. Knowledge (required)
  ├── Cancel runtime context
  ├── Wait for goroutines (sync.WaitGroup)
  ├── Transition to Stopped
  ├── Close shutdown channel
  ├── Publish EventShutdownComplete
  └── Return success
```

### Component Health Status

| Status | Meaning |
|--------|---------|
| `unknown` | Status not yet determined |
| `starting` | Component initializing |
| `healthy` | Component operating normally |
| `degraded` | Reduced capabilities |
| `unhealthy` | Component not functioning |
| `stopped` | Component shut down |

The aggregate health is computed from component statuses:
- Any component `unhealthy` → overall `unhealthy`
- Any component `degraded` → overall `degraded`
- All `stopped` or `unknown` → overall `unknown`
- Otherwise → overall `healthy`

---

## Deployment Modes

### CLI Mode (default)
- Runtime starts and stops with each command
- No background process
- Suitable for interactive use and scripting

```bash
cosca status      # Runtime starts → runs command → stops
cosca doctor      # Runtime starts → runs diagnostics → stops
```

### Daemon Mode
- Runtime runs as a background process
- Continuous file watching and indexing
- Faster command execution (no startup cost)
- PID file at `./tmp/cosca.pid`
- Log rotation: 100MB max, 5 backups

```bash
cosca runtime start     # Start daemon
cosca runtime status    # Check daemon status
cosca runtime stop      # Stop daemon
```

**Daemon features:**
- **Watchdog:** Auto-restarts unhealthy subsystems (Stop + Start, 30s timeout each)
- **Sync loop:** Periodic sync (default: every 5 minutes, initial 10s delay)
- **Stale PID detection:** Checks if existing PID is alive (signal 0 probe), auto-removes stale files

### Embedded Mode
- Runtime embedded in another application via Go types
- Full lifecycle managed by host application

```go
import (
    "github.com/CoscaAI/cosca/pkg/cosca"
)

// Use the public SDK (HTTP client to running Cosca)
runtime := cosca.NewRuntimeSDK(client)
status, _ := runtime.Status(ctx)
health, _ := runtime.Health(ctx)
```

> **Note:** Direct embedding of `internal/runtime` types is not supported outside the Go module due to Go's `internal/` package restrictions. Use the HTTP API or the public `pkg/cosca` types for integration.

---

## Metrics

### Counters

| Metric | Description |
|--------|-------------|
| `index_count` | Total index operations |
| `search_count` | Total search operations |
| `context_builds` | Total context builds |
| `memory_stores` | Total memory store operations |
| `plugin_calls` | Total plugin invocations |
| `error_count` | Total errors recorded |
| `sync_count` | Total sync operations |
| `event_count` | Total events published |

### Duration Histograms (p50/p95/p99)

| Histogram | Operation |
|-----------|-----------|
| Index duration | Document indexing time |
| Search duration | Search query time |
| Context build duration | Context construction time |
| Memory store duration | Memory record write time |

### System Metrics

| Metric | Description |
|--------|-------------|
| `goroutines` | Current goroutine count + min/max/avg samples |
| `memory_alloc` | Current heap memory allocation |
| `total_alloc` | Cumulative memory allocated |

### Runtime State Metrics

| Metric | Description |
|--------|-------------|
| `uptime` | Runtime uptime in seconds |
| `state` | Current state string |
| `version` | Runtime version |

---

## Event Bus

The event bus provides publish-subscribe communication between subsystems:

```go
// Publish an event
runtime.Events().Publish(ctx, EventSubsystemError, "knowledge", err)

// Subscribe to events
runtime.Events().Subscribe(EventStateChange, func(ctx context.Context, event Event) error {
    log.Info().Str("from", event.Data.(StateChangeEvent).From.String()).Msg("state changed")
    return nil
})
```

### Event Types

| Event | Description |
|-------|-------------|
| `state_change` | State machine transition |
| `subsystem_started` | Subsystem initialized |
| `subsystem_stopped` | Subsystem shut down |
| `subsystem_error` | Subsystem error |
| `health_change` | Health status changed |
| `startup_complete` | Startup sequence finished |
| `shutdown_initiated` | Shutdown started |
| `shutdown_complete` | Shutdown finished |
| `config_reload` | Configuration reloaded |

### Event Delivery
- Synchronous within publishing goroutine
- 10-second handler timeout per subscriber
- Errors logged but do not block other subscribers

---

## Signal Handling

| Signal | Action |
|--------|--------|
| `SIGINT` | Graceful shutdown (ShutdownTimeout, default 60s) |
| `SIGTERM` | Force cancel (all contexts cancelled immediately) |
| `SIGHUP` | Config reload event published (no automatic config re-read) |

---

## Configuration

```go
type RuntimeConfig struct {
    Name                string        // default: "cosca"
    Version             string        // build version
    DataDir             string        // data directory
    RuntimeDir          string        // runtime working directory
    ComponentTimeout    time.Duration // default: 30s
    ShutdownTimeout     time.Duration // default: 60s
    HealthCheckInterval time.Duration // default: 30s
    EnableMetrics       bool          // default: true
    EnableDaemon        bool          // default: false
    LogLevel            string        // default: "info"
    PidFile             string
}
```

### Daemon Configuration

```go
type DaemonConfig struct {
    PIDPath           string        // default: "./tmp/cosca.pid"
    LogMaxSize        int64         // default: 100 MB
    LogMaxBackups     int           // default: 5
    WatchdogInterval  time.Duration // default: 30s
    SyncInterval      time.Duration // default: 5 min
    SyncFunc          func(ctx context.Context) error
    OnReload          func() error
}
```

---

## Health Report

```json
{
  "state": "running",
  "health": "healthy",
  "uptime": 332000000000,
  "started_at": "2026-07-28T10:00:00Z",
  "version": "1.4.0-dev",
  "components": {
    "knowledge": {"name": "knowledge", "status": "healthy", "message": "", "error": ""},
    "discovery": {"name": "discovery", "status": "healthy", "message": "", "error": ""},
    "memory":    {"name": "memory",    "status": "healthy", "message": "", "error": ""},
    "cache":     {"name": "cache",     "status": "healthy", "message": "", "error": ""}
  }
}
```

---

## Known Limitations

1. **`Restart()` requires `Uninitialized` state.** A successful `Restart()` sequence is: `Stop()` → transition to `uninitialized` → `Start()`. The `stopped → uninitialized` transition is defined but must be executed manually between Stop and Start.

2. **`EventStartupComplete` fires early.** Published during `initializing` state, before init hooks execute. Subscribers should not assume subsystems are ready when they receive this event.

3. **Double signal handling in daemon mode.** When both `Runtime.HandleSignals()` and `Daemon.handleSignals()` are active, signals trigger both handlers concurrently. This is functionally safe (second `Stop()` call hits early-return) but worth noting.

4. **Hot reload (`running → ready`)** is defined in the state machine but has no triggering code path. SIGHUP publishes a config reload event but does not execute any state transition.

---

> **Related**: [Configuration Reference](configuration.md) | [Architecture Overview](../architecture/overview.md) | [Frontend Architecture](../frontend/architecture.md)
