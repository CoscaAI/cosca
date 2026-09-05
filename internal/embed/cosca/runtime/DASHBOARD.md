# Dashboard Integration

> **Extracted from**: KERNEL.md section 18 | **Lines**: ~495 | **Date**: 2026-07-28

## Overview

The Dashboard provides real-time visibility into Runtime state via REST API, SSE, and WebSocket.

## Dashboard Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| /api/dashboard/stats | GET | Aggregate statistics |
| /api/dashboard/agents | GET | Agent status |
| /api/dashboard/tasks | GET | Task queue |
| /api/dashboard/health | GET | Component health |
| /api/dashboard/events | SSE | Real-time events |
| /api/dashboard/ws | WS | Bidirectional updates |

## Dashboard Modules

| Module | Description | Refresh |
|--------|-------------|---------|
| Overview | Key metrics at a glance | 30s |
| Agents | Agent status and history | 10s |
| Tasks | Task queue and execution | 5s |
| Knowledge | Knowledge store stats | 60s |
| Memory | Memory usage and tiers | 60s |
| Events | Live event stream | Real-time |
| Logs | Structured log viewer | Real-time |

## SSE Event Stream

```javascript
const eventSource = new EventSource('/api/dashboard/events');
eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data);
  // Update UI based on event type
};
```

## WebSocket Protocol

```json
{
  "type": "subscribe",
  "channel": "tasks"
}

{
  "type": "update",
  "channel": "tasks",
  "data": {
    "taskId": "abc",
    "status": "running"
  }
}
```

## Security

- Dashboard requires authentication (JWT or API key)
- Read-only by default (no mutations via dashboard)
- Rate limiting: 100 requests/minute per client
