> **Version**: 1.0.0 | **Status**: active | **Owner**: Workflow Engine | **Last Updated**: 2026-07-10

# WORKFLOW ENGINE

## PURPOSE
The Workflow Engine models all work as structured workflows. It defines, executes, monitors, and validates workflows. Every task in the Cosca follows a workflow.

## WORKFLOW SCHEMA

```yaml
name: string                # Unique workflow identifier
version: string             # Semantic version
description: string         # What this workflow does
category: string            # feature|bug|refactor|review|deploy|init|audit
objective: string           # Clear statement of what success looks like

inputs:                     # Required inputs
  - name: string
    type: string
    required: boolean
    description: string

outputs:                    # Expected outputs
  - name: string
    type: string
    description: string

preconditions:              # Must be true before start
  - condition: string
    verify: string          # How to verify

postconditions:             # Must be true after completion
  - condition: string
    verify: string

dependencies:               # Other workflows that must complete first
  - workflow: string
    reason: string

steps:                      # Sequential or parallel steps
  - id: string
    name: string
    description: string
    chief: string           # Department chief responsible
    specialists:            # Required specialist types
      - string
    inputs: object          # Step-specific inputs
    outputs: object         # Step-specific outputs
    timeout: string         # Max duration
    retry: number           # Max retries
    depends_on: [string]    # Step IDs this step depends on
    parallel: boolean       # Can run in parallel with other steps
    review_required: boolean
    qa_required: boolean

validation:                 # How to validate the workflow result
  - check: string
    severity: error|warning|info
    automated: boolean

success_criteria:           # What defines success
  - criterion: string
    metric: string
    threshold: string

failure_handlers:           # What to do on failure
  - on: string              # Error type
    action: retry|skip|abort|escalate
    max_retries: number
    escalation_target: string
```

## WORKFLOW LIFECYCLE

```
CREATED → VALIDATED → READY → RUNNING → REVIEWING → QA → COMPLETED
                                    ↓           ↓       ↓      ↓
                               FAILED     REJECTED  FAILED  ARCHIVED
                                    ↓           ↓       ↓
                               RETRYING   REWORKING  FIXING
```

## EXECUTION ENGINE

1. **Validate**: Check all preconditions are met
2. **Schedule**: Create execution plan (topological sort of steps)
3. **Execute**: Run steps sequentially or in parallel as defined
4. **Monitor**: Track progress, timeouts, failures
5. **Retry**: On failure, retry or escalate based on rules
6. **Validate**: Check postconditions
7. **Review**: Send to Review Chief if review_required
8. **QA**: Send to QA Chief if qa_required
9. **Complete**: Archive and store in memory

## BUILT-IN WORKFLOWS

1. **project-init** — Initialize a new Cosca-managed project
2. **feature-development** — Full feature lifecycle
3. **bug-fix** — Bug report to fix delivery
4. **refactoring** — Code improvement with safety checks
5. **code-review** — Comprehensive code review
6. **release** — Release preparation and execution
7. **deployment** — Deployment pipeline
8. **dependency-update** — Update dependencies safely
9. **security-audit** — Security review and remediation
10. **performance-audit** — Performance analysis and optimization

Workflow definitions are maintained in workflows/*.md. See [COSCA_INDEX.md](../../COSCA_INDEX.md) for complete inventory.

## DEPENDENCIES

| Planning Engine | Pre-execution planning |
| Execution Engine | Step execution runtime |
| Review Engine | Review step enforcement |
| QA Engine | QA step enforcement |
| Memory Engine | Workflow archival |
| Kernel | Workflow routing |
| Workflow Chief | Workflow design |

## RELATED
- [COSCA_INDEX.md](../../COSCA_INDEX.md)
- [Workflow Chief](../../departments/workflow/SKILL.md)
- [Planning Engine](../planning/SKILL.md)
- [Execution Engine](../execution/SKILL.md)

## HISTORY

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-07-10 | Initial version. Workflow schema, lifecycle, execution model, built-in workflows. |
