---
name: observability
description: Provides visibility into internal state - metrics, traces, and logs from all agents and workflows.
level: 2
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Observability Engine | **Last Updated**: 2026-07-10

# OBSERVABILITY ENGINE

## PURPOSE
The Observability Engine provides visibility into the Cosca internal state. It collects metrics, traces, and logs from all agents and workflows.

## OBSERVABILITY PILLARS

### 1. Logging
- Structured logging (JSON format)
- Log levels: DEBUG, INFO, WARN, ERROR, FATAL
- Contextual logging (agent, workflow, task IDs)
- Redaction of sensitive data
- Log aggregation

### 2. Metrics
- Agent metrics (tasks completed, failure rate, avg duration)
- Workflow metrics (executions, success rate, avg duration, bottlenecks)
- Quality metrics (review scores, test coverage, code quality)
- System metrics (memory usage, context size, token usage)
- User metrics (requests, satisfaction, common patterns)

### 3. Tracing
- Distributed tracing across agents
- Workflow step tracing
- Task dependency tracing
- Performance bottleneck identification

### 4. Alerting
- Agent failure alerts
- Workflow stall alerts
- Quality degradation alerts
- Security violation alerts
- Resource exhaustion alerts

### 5. Dashboards
- System overview (health, active agents, workflows)
- Quality dashboard (scores, trends, debt)
- Performance dashboard (latency, throughput)
- Security dashboard (vulnerabilities, audits)
- Evolution dashboard (improvements, trends)

## METRICS COLLECTION

```yaml
metrics:
  agents:
    - agent_name
    - tasks_total
    - tasks_succeeded
    - tasks_failed
    - avg_task_duration_ms
    - success_rate
    - last_active
    
  workflows:
    - workflow_name
    - executions_total
    - success_rate
    - avg_duration_ms
    - p50_duration_ms
    - p99_duration_ms
    - failure_reasons
    
  quality:
    - overall_score
    - architecture_score
    - code_quality_score
    - security_score
    - test_coverage_pct
    - documentation_score
    
  system:
    - active_agents_count
    - pending_tasks_count
    - memory_usage
    - context_size
    - session_duration
    - token_usage
```

## DASHBOARD FORMAT

```
┌─────────────────────────────────────────────────┐
│              Cosca SYSTEM DASHBOARD                 │
├─────────────────────────────────────────────────┤
│ STATUS: 🟢 Healthy    UPTIME: 2h 34m             │
│                                                   │
│ ACTIVE AGENTS: 5    QUEUED TASKS: 3               │
│ WORKFLOWS: 2 active, 12 completed                 │
│                                                   │
│ ─── QUALITY ───────────────────────────────────  │
│ Overall: A    Code: A    Tests: 87%    Sec: A     │
│                                                   │
│ ─── PERFORMANCE ───────────────────────────────  │
│ Avg task: 45s    P99: 3m 12s                      │
│                                                   │
│ ─── RECENT ────────────────────────────────────  │
│ ✅ feature-development (auth) — 12m ago          │
│ 🔄 bug-fix (login-error) — running               │
│ ⏳ refactoring (user-module) — queued             │
└─────────────────────────────────────────────────┘
```

## DEPENDENCIES
- Receives events from all engines via Kernel
- Stores metrics in Memory Engine
- Triggers alerts via Monitoring Engine
- Feeds into Evolution Engine for trend analysis

## RELATED
- [Evolution Engine](../evolution/SKILL.md) — Consumes metrics for trend analysis
- [Audit Engine](../audit/SKILL.md) — Audit patterns feed into observability alerts
- [Runtime Engine](../runtime/SKILL.md) — Emits runtime health and performance data

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Added metadata, HISTORY, and cross-references |
