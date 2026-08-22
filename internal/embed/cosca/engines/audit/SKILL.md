> **Version**: 1.0.0 | **Status**: active | **Owner**: Audit Engine | **Last Updated**: 2026-07-10

# AUDIT ENGINE

## PURPOSE
The Audit Engine records every action, decision, and change in the system. It provides a complete, tamper-evident audit trail for compliance and debugging.

## AUDIT EVENTS

### Decision Audit
Every decision is logged:
- Timestamp
- Agent who made it
- Decision type (architecture, product, technical, resource)
- Context
- Alternatives considered
- Rationale
- Outcome

### Action Audit
Every action is logged:
- Timestamp
- Agent who performed it
- Action type (file_write, file_edit, command_execute, git_commit, etc.)
- Target (file path, command, etc.)
- Before state (for edits)
- After state (for edits)
- Duration
- Success/failure

### Workflow Audit
Every workflow event is logged:
- Timestamp
- Workflow ID
- Step ID
- Event type (started, completed, failed, retried)
- Agent assigned
- Output/error
- Duration

### Review Audit
Every review is logged:
- Timestamp
- Reviewer
- Deliverable reviewed
- Issues found
- Decision (approved, changes_requested, rejected)
- Compliance scores

### Quality Audit
Every quality check is logged:
- Timestamp
- Quality gate
- Result (passed, failed, warning)
- Metrics
- Recommendations

### Communication Audit
Every user interaction is logged:
- Timestamp
- User request
- Cosca response summary
- Decisions made
- Actions taken

## AUDIT STORAGE

```
.cosca/audit/
├── decisions/
│   └── YYYY-MM-DD/
│       └── decision-[id].json
├── actions/
│   └── YYYY-MM-DD/
│       └── action-[id].json
├── workflows/
│   └── YYYY-MM-DD/
│       └── workflow-[id].json
├── reviews/
│   └── YYYY-MM-DD/
│       └── review-[id].json
├── quality/
│   └── YYYY-MM-DD/
│       └── quality-[id].json
├── communications/
│   └── YYYY-MM-DD/
│       └── comm-[id].json
└── summary/
    └── YYYY-MM.md
```

## AUDIT EVENT FORMAT

```json
{
  "id": "uuid",
  "timestamp": "ISO8601",
  "session_id": "string",
  "event_type": "decision|action|workflow|review|quality|communication",
  "agent": {
    "name": "string",
    "type": "chief|specialist|engine",
    "department": "string"
  },
  "details": {
    // Event-specific data
  },
  "outcome": "success|failure|partial",
  "duration_ms": 0,
  "related_events": ["uuid"],
  "metadata": {}
}
```

## COMPLIANCE REPORTS

The Audit Engine can generate:
- Decision log (all architecture/product decisions)
- Change log (all file changes)
- Review log (all reviews performed)
- Security log (all security-related events)
- Access log (all user interactions)
- Performance report (workflow durations and bottlenecks)

## DEPENDENCIES
- Every action in Cosca triggers an audit event
- Memory Engine stores audit data for long-term retention
- Monitoring Engine can alert on audit event patterns
- Compliance reports feed into Documentation Engine

## RELATED
- [Observability Engine](../observability/SKILL.md) — Monitors audit event patterns and triggers alerts
- [Documentation Engine](../documentation/SKILL.md) — Consumes compliance reports
- [Tools Engine](../tools/SKILL.md) — Tool usage is audited by this engine

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Added metadata, HISTORY, and cross-references |
