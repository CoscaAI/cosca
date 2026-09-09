# WORKFLOW: bug-fix

> **Version**: 1.1.0 | **Status**: active | **Category**: bug | **Last Updated**: 2026-07-12

## OBJECTIVE
Fix a reported bug with full quality assurance — root cause analysis, fix implementation, regression tests, and pattern documentation for future prevention.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| bug_description | string | Yes | Description of the bug |
| steps_to_reproduce | string | Yes | How to reproduce |
| expected_behavior | string | Yes | What should happen |
| actual_behavior | string | Yes | What actually happens |
| severity | string | Yes | critical, high, medium, low |
| affected_components | array | No | Which components are affected |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| root_cause | string | Root cause analysis |
| fix | object | Code changes |
| tests | object | Regression tests |
| review_report | object | Review results |
| bug_pattern | object | Documented pattern for future reference |

## PRECONDITIONS
1. Bug is reproducible
2. Current tests passing

## POSTCONDITIONS
1. Bug is fixed
2. Regression tests added
3. No new issues introduced
4. Bug pattern documented (if novel)

## DEPENDENCIES
None

## STEPS

### Step 1: Bug Analysis
- **Chief**: CTO
- **Specialists**: Tech Lead
- **Task**: Understand the bug, identify affected area, classify severity
- **Output**: Bug analysis document

### Step 2: Root Cause Investigation
- **Chief**: Review (or Backend/Frontend based on affected area)
- **Specialists**: Code Reviewer
- **Task**: Find the root cause through code analysis
- **Output**: Root cause analysis

### Step 3: Fix Design
- **Chief**: Architecture
- **Specialists**: Solutions Architect
- **Task**: Design the fix approach (if significant)
- **Output**: Fix design document (or skip for trivial fixes)

### Step 4: Fix Implementation
- **Chief**: Backend or Frontend (based on affected area)
- **Specialists**: Relevant specialist
- **Task**: Implement the fix
- **Output**: Fixed code

### Step 5: Regression Tests
- **Chief**: Testing
- **Specialists**: Unit Test Engineer
- **Task**: Add regression tests that reproduce the bug and verify the fix
- **Output**: Test cases (at least 1 reproducing bug + 1 edge case)

### Step 6: Code Review
- **Chief**: Review
- **Specialists**: Code Reviewer
- **Task**: Review the fix for correctness and side effects
- **Output**: Review report

### Step 7: Impact Analysis
- **Chief**: QA
- **Specialists**: Manual QA
- **Task**: Verify no regression in related functionality
- **Output**: Impact analysis report

### Step 8: Bug Pattern Documentation
- **Chief**: Memory
- **Specialists**: Knowledge Base Engineer
- **Task**: Document bug pattern for future reference (if novel)
- **Output**: Bug memory entry in ${MEMORY_GLOBAL}/bug/

## VALIDATION
1. Bug is no longer reproducible
2. Regression tests pass
3. Existing tests still pass
4. No side effects detected in related areas

## SUCCESS CRITERIA
- [ ] Bug is fixed
- [ ] Regression test added
- [ ] All existing tests pass
- [ ] Code review approved
- [ ] Bug pattern documented (if novel)

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Bug not reproducible | Request more details from user, log as unconfirmed |
| Root cause unclear | Escalate to Architecture Chief for deeper analysis |
| Fix introduces new bug | Revert fix, re-analyze, implement alternative |
| Test regression | Identify affected tests, fix or update |
| Review rejection | Address feedback, re-submit |
| Pattern already exists | Link to existing pattern, skip new entry |

## RELATED
- [MEMORY_MODEL.md](../MEMORY_MODEL.md) — Bug memory store schema
- [Workflow Engine](../engines/workflow/SKILL.md) — Workflow lifecycle
- [Review Engine](../engines/review/SKILL.md) — Code review enforcement
- [Architecture Chief](../departments/architecture/SKILL.md) — Fix design for complex bugs
- [Memory Chief](../departments/memory/SKILL.md) — Bug pattern storage

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Kernel | Initial workflow definition (8 steps) |
| 1.1.0 | 2026-07-12 | Cosca Kernel | Updated to canonical format: added History, Error Handling, Related, Status/Category |
