---
name: execution
description: Runs workflows and tasks - coordinates agents, parallel execution, failures, and deliverables.
level: 3
---

> **Version**: 1.0.0 | **Status**: active | **Owner**: Execution Engine | **Last Updated**: 2026-07-10

# EXECUTION ENGINE

## PURPOSE
The Execution Engine runs workflows and tasks. It coordinates agents, manages parallel execution, handles failures, and ensures deliverables are produced.

## EXECUTION MODEL

### Task Execution Loop
```
1. Receive task from Planning Engine or Chief
2. Validate task definition (inputs, preconditions, acceptance criteria)
3. Select primary agent (specialist type)
4. Assign secondary agent for redundancy
5. Execute task via primary agent
6. Monitor progress
7. On completion → Validate output against acceptance criteria
8. On failure → Activate secondary agent or escalate
9. Send to Review Chief
10. Track in execution log
```

### Parallel Execution Rules
- Tasks with no mutual dependencies can run in parallel
- Max parallel tasks per department: 3
- Max total parallel tasks: 10
- Resource contention: Serialize tasks on same file/module

### Failure Handling

| Failure Type | Action | Escalation |
|-------------|--------|------------|
| Agent timeout | Retry (max 3) | Secondary agent |
| Agent error | Secondary agent | Chief |
| Validation failure | Return to agent with feedback | Chief |
| Secondary agent failure | Escalate to Chief | CTO |
| Chief failure | Escalate to CTO | CEO |
| Dependency failure | Block dependent tasks, retry dependency | CTO |

### Progress Tracking

Each task tracks:
- Status: pending | running | reviewing | qa | completed | failed
- Started at
- Completed at
- Agent assigned
- Attempts
- Output
- Review status
- QA status

## EXECUTION LOG FORMAT
```json
{
  "task_id": "uuid",
  "workflow_id": "uuid",
  "step_id": "string",
  "status": "string",
  "agent_primary": "string",
  "agent_secondary": "string",
  "started_at": "ISO8601",
  "completed_at": "ISO8601",
  "attempts": 0,
  "output": {},
  "review": {},
  "qa": {},
  "errors": []
}
```

## DEPENDENCIES
- Receives tasks from Planning Engine
- Delegates to department chiefs via Kernel
- Reports status to Workflow Engine
- Stores execution logs in Memory Engine
- Triggers Review Engine on completion
- Triggers QA Engine after review

## RELATED
- [Planning Engine](../planning/SKILL.md) — Provides task plans to execute
- [Review Engine](../review/SKILL.md) — Receives completed work for review

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Added metadata, HISTORY, and cross-references |
