# Dashboard Integration — Extracted from KERNEL.md §18

> **Source**: KERNEL.md v3.0.1 §18 | **Extracted**: 2026-07-28 | **Status**: active
>
> This document was extracted from the monolithic KERNEL.md to improve maintainability.
> The authoritative specification remains in KERNEL.md. This extraction is a
> readability aid. In case of discrepancy, KERNEL.md takes precedence.

## 18. DASHBOARD INTEGRATION

The Dashboard **never** reads Markdown directly. All information comes from the Runtime via the Event Bus and REST API. The Dashboard is the **visual command center** for the Cosca platform — providing real-time visibility, control, and insight into every aspect of the Runtime.

---

### 18.1 Dashboard Architecture

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           DASHBOARD SYSTEM                                   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                       BACKEND SERVICES                                │   │
│  │                                                                       │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌──────────────┐   │   │
│  │  │ API Server │  │  SSE       │  │ WebSocket  │  │  Auth        │   │   │
│  │  │ (REST)     │  │  Stream    │  │  Server    │  │  Service     │   │   │
│  │  └────────────┘  └────────────┘  └────────────┘  └──────────────┘   │   │
│  │                                                                       │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌──────────────┐   │   │
│  │  │ Event Bus  │  │  Metrics   │  │  State     │  │  Command     │   │   │
│  │  │ Consumer   │  │  Aggregator│  │  Cache     │  │  Handler     │   │   │
│  │  └────────────┘  └────────────┘  └────────────┘  └──────────────┘   │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│                                    ▼                                        │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                       FRONTEND (Web UI)                               │   │
│  │                                                                       │   │
│  │  ┌──────────────────────────────────────────────────────────────┐   │   │
│  │  │                    PAGE LAYOUT                                  │   │   │
│  │  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐  │   │   │
│  │  │  │ Overview │  │ Runtime  │  │ Health   │  │  Capabilities │  │   │   │
│  │  │  │ Page     │  │ State    │  │ Page     │  │  Page         │  │   │   │
│  │  │  └──────────┘  └──────────┘  └──────────┘  └──────────────┘  │   │   │
│  │  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐  │   │   │
│  │  │  │ Workflows│  │ Memory   │  │ Events   │  │  Command     │  │   │   │
│  │  │  │ Page     │  │ Page     │  │ Timeline │  │  Center      │  │   │   │
│  │  │  └──────────┘  └──────────┘  └──────────┘  └──────────────┘  │   │   │
│  │  └──────────────────────────────────────────────────────────────┘   │   │
│  │                                                                       │   │
│  │  ┌──────────────────────────────────────────────────────────────┐   │   │
│  │  │                    WIDGET SYSTEM                                │   │   │
│  │  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐  │   │   │
│  │  │  │ Status   │  │ Gauges   │  │ Graphs   │  │  Tables      │  │   │   │
│  │  │  │ Badge    │  │ & Meters │  │ & Charts │  │  & Grids     │  │   │   │
│  │  │  └──────────┘  └──────────┘  └──────────┘  └──────────────┘  │   │   │
│  │  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐  │   │   │
│  │  │  │ Timelines│  │ Progress │  │ Alerts   │  │  Command     │  │   │   │
│  │  │  │          │  │  Bars    │  │ & Notifs │  │  Palette     │  │   │   │
│  │  │  └──────────┘  └──────────┘  └──────────┘  └──────────────┘  │   │   │
│  │  └──────────────────────────────────────────────────────────────┘   │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

### 18.2 Dashboard Pages

#### Page 1 — Overview

```yaml
dashboard_page_overview:
  name: "Overview"
  route: "/"
  refresh: "5s"

  widgets:
    - name: "Runtime Status"
      type: "status_badge"
      data_source: "GET /api/v1/status"
      values: ["running", "degraded", "stopped"]

    - name: "Health Score"
      type: "gauge"
      data_source: "GET /api/v1/health"
      min: 0
      max: 100
      thresholds: { warning: 70, critical: 50 }

    - name: "Active Sessions"
      type: "counter"
      data_source: "GET /api/v1/metrics?name=session.active"

    - name: "Queue Depth"
      type: "graph"
      data_source: "SSE: scheduler.queue.depth"
      time_range: "1h"

    - name: "Recent Events"
      type: "timeline"
      data_source: "SSE: all events"
      max_items: 20

    - name: "Quality Score"
      type: "gauge"
      data_source: "GET /api/v1/metrics?name=quality.score.overall"
      min: 0
      max: 10
```

#### Page 2 — Runtime State

```yaml
dashboard_page_runtime:
  name: "Runtime State"
  route: "/runtime"
  refresh: "2s"

  widgets:
    - name: "State Machine"
      type: "state_diagram"
      data_source: "GET /api/v1/state"
      highlight_current: true

    - name: "State Transitions"
      type: "timeline"
      data_source: "SSE: state.transition.*"
      max_items: 50

    - name: "Current State Details"
      type: "property_table"
      data_source: "GET /api/v1/state"
      fields: [state, duration_ms, started_at, last_transition]

    - name: "Session Activity"
      type: "graph"
      data_source: "SSE: session.*"
      time_range: "1h"
```

#### Page 3 — Health

```yaml
dashboard_page_health:
  name: "Health"
  route: "/health"
  refresh: "10s"

  widgets:
    - name: "Component Health"
      type: "health_table"
      data_source: "GET /api/v1/health"
      columns: [component, status, latency_ms, uptime, cb_state]

    - name: "Circuit Breakers"
      type: "status_table"
      data_source: "GET /api/v1/health/circuit-breakers"
      columns: [breaker, state, tripped, last_opened]

    - name: "Resource Usage"
      type: "gauges"
      data_source: "GET /api/v1/health/resources"
      metrics: [cpu, memory, disk]

    - name: "Recovery Status"
      type: "status_card"
      data_source: "GET /api/v1/health/recovery"

    - name: "Failover Status"
      type: "status_card"
      data_source: "GET /api/v1/health/failover"
```

#### Page 4 — Capabilities

```yaml
dashboard_page_capabilities:
  name: "Capabilities"
  route: "/capabilities"
  refresh: "30s"

  widgets:
    - name: "Capability Registry"
      type: "searchable_table"
      data_source: "GET /api/v1/capabilities"
      columns: [id, name, category, status, provider, quality_score]
      searchable: true
      filterable: ["category", "status"]

    - name: "Capability Quality"
      type: "bar_chart"
      data_source: "GET /api/v1/metrics?name=capability.quality.score"

    - name: "Capability Usage"
      type: "heatmap"
      data_source: "GET /api/v1/metrics?name=capability.resolution.count"
      time_range: "7d"
```

#### Page 5 — Workflows

```yaml
dashboard_page_workflows:
  name: "Workflows"
  route: "/workflows"
  refresh: "10s"

  widgets:
    - name: "Active Workflows"
      type: "table"
      data_source: "GET /api/v1/workflows?status=active"
      columns: [id, name, progress, duration, status]

    - name: "DAG Visualization"
      type: "graph_view"
      data_source: "GET /api/v1/workflows/{id}/dag"
      interactive: true

    - name: "Workflow Duration"
      type: "graph"
      data_source: "SSE: workflow.duration_ms"
      time_range: "24h"

    - name: "Workflow Success Rate"
      type: "gauge"
      data_source: "GET /api/v1/metrics?name=workflow.failure.count"
```

#### Page 6 — Memory

```yaml
dashboard_page_memory:
  name: "Memory"
  route: "/memory"
  refresh: "30s"

  widgets:
    - name: "Memory Stores"
      type: "table"
      data_source: "GET /api/v1/memory"
      columns: [store, entries, size, last_updated]

    - name: "Memory Operations"
      type: "graph"
      data_source: "SSE: memory.*"
      time_range: "1h"

    - name: "Knowledge Graph"
      type: "graph_view"
      data_source: "GET /api/v1/knowledge/graph"
      interactive: true
```

#### Page 7 — Events Timeline

```yaml
dashboard_page_events:
  name: "Events"
  route: "/events"
  refresh: "realtime"

  widgets:
    - name: "Live Event Stream"
      type: "infinite_scroll"
      data_source: "SSE: all events"
      max_items: 200
      filterable: ["event_type", "publisher", "severity"]

    - name: "Event Volume"
      type: "graph"
      data_source: "SSE: eventbus.published.total"
      time_range: "1h"

    - name: "Event Correlation"
      type: "graph_view"
      data_source: "GET /api/v1/events/trace?correlation_id={id}"
```

#### Page 8 — Command Center

```yaml
dashboard_page_command:
  name: "Command Center"
  route: "/commands"
  refresh: "manual"

  widgets:
    - name: "Command Palette"
      type: "command_input"
      data_source: "POST /api/v1/command"
      suggestions: ["/status", "/health", "/metrics", "/sync", "/evolve"]
      history: true

    - name: "Command History"
      type: "table"
      data_source: "GET /api/v1/commands/history"
      columns: [command, status, timestamp, duration_ms]

    - name: "Quick Actions"
      type: "action_buttons"
      actions:
        - label: "Force Sync"
          command: "/sync"
          confirm: true
        - label: "Run Evolution"
          command: "/evolve"
          confirm: true
        - label: "Create Snapshot"
          command: "/snapshot"
          confirm: false
        - label: "Health Check"
          command: "/health"
          confirm: false
```

---

### 18.3 Real-Time Update Protocol

#### SSE Stream Specification

```yaml
sse_stream:
  endpoint: "/api/v1/events/stream"
  protocol: "Server-Sent Events (SSE, text/event-stream)"

  event_format:
    event: "event_name"
    data: "{ json payload }"
    id: "event-uuid"
    retry: 3000  # ms before reconnection

  streams:
    - name: "all"
      path: "/api/v1/events/stream"
      events: "all"

    - name: "state"
      path: "/api/v1/events/stream?filter=state"
      events: ["state.*", "session.*"]

    - name: "health"
      path: "/api/v1/events/stream?filter=health"
      events: ["health.*", "circuit.*"]

    - name: "metrics"
      path: "/api/v1/events/stream?filter=metrics"
      events: ["metric.*", "scheduler.*"]

    - name: "workflows"
      path: "/api/v1/events/stream?filter=workflows"
      events: ["workflow.*", "execution.*"]

  reconnection:
    strategy: "exponential backoff"
    initial_delay_ms: 1000
    max_delay_ms: 30000
    max_attempts: 10
    last_event_id: "Send on reconnect to resume from last event"
```

#### WebSocket Specification

```yaml
websocket:
  endpoint: "/api/v1/ws"
  protocol: "WebSocket (RFC 6455)"

  messages:
    client_to_server:
      subscribe: { type: "subscribe", channels: ["state", "health"] }
      unsubscribe: { type: "unsubscribe", channels: ["metrics"] }
      command: { type: "command", command: "/status", id: "req-1" }
      ping: { type: "ping" }

    server_to_client:
      event: { type: "event", channel: "state", data: {} }
      command_result: { type: "command_result", id: "req-1", status: "ok", data: {} }
      pong: { type: "pong" }
      error: { type: "error", message: "..." }

  channels:
    - "state"
    - "health"
    - "events"
    - "metrics"
    - "workflows"
    - "commands"
    - "alerts"
```

---

### 18.4 Dashboard Authentication

```yaml
dashboard_auth:
  authentication:
    methods:
      - "API Key (header: X-API-Key)"
      - "JWT Bearer Token"
      - "OAuth2 (future)"

  authorization:
    roles:
      admin:
        description: "Full access to all pages and commands"
        pages: ["*"]
        commands: ["*"]

      operator:
        description: "View all pages, execute non-destructive commands"
        pages: ["*"]
        commands: ["/status", "/health", "/metrics", "/events"]

      viewer:
        description: "Read-only access to all pages"
        pages: ["*"]
        commands: []

  session:
    duration: "8 hours"
    refresh: "Silent refresh via refresh token"
    logout: "On session end or explicit logout"
```

---

### 18.5 Dashboard Events

| Event | Trigger | Payload | Consumers |
|-------|---------|---------|-----------|
| `DashboardClientConnected` | SSE/WS client connects | client_id, type, user_agent | Observability |
| `DashboardClientDisconnected` | SSE/WS client disconnects | client_id, duration_ms | Observability |
| `DashboardCommandExecuted` | Command from Command Center | command, status, duration_ms | Audit, History |
| `DashboardCommandFailed` | Command execution failed | command, error | Audit, Alerting |
| `DashboardPageViewed` | User navigates to page | page_name, duration_ms | Analytics |
| `DashboardWidgetRefreshed` | Widget data updated | widget_name, duration_ms | Performance |

---

### 18.6 Dashboard Metrics

| Metric | Type | Tags | Description |
|--------|------|------|-------------|
| `dashboard.clients.connected` | Gauge | type | Connected clients |
| `dashboard.api.latency_ms` | Histogram | endpoint | API response time |
| `dashboard.sse.events_pushed` | Counter | channel | Events pushed via SSE |
| `dashboard.sse.latency_ms` | Histogram | channel | SSE event latency |
| `dashboard.ws.messages` | Counter | direction | WebSocket messages |
| `dashboard.commands.executed` | Counter | command | Commands executed |
| `dashboard.commands.failed` | Counter | command | Commands failed |
| `dashboard.page.views` | Counter | page | Page views |
| `dashboard.error.count` | Counter | error_type | Dashboard errors |

---

### 18.7 Dashboard Configuration

```yaml
dashboard_config:
  enabled: true
  port: 8080

  authentication:
    enabled: true
    api_key: "${DASHBOARD_API_KEY}"
    jwt_secret: "${JWT_SECRET}"

  sse:
    enabled: true
    max_clients: 100
    ping_interval_ms: 30000

  websocket:
    enabled: true
    max_clients: 50
    message_size_limit: 65536

  api:
    rate_limit: 100  # requests per minute per client
    timeout_ms: 30000

  pages:
    - "overview"
    - "runtime"
    - "health"
    - "capabilities"
    - "workflows"
    - "memory"
    - "events"
    - "commands"

  default_page: "overview"
  refresh_intervals:
    fast_ms: 2000
    normal_ms: 10000
    slow_ms: 30000
```


