# WORKFLOW: technical-debt-paydown

> **Version**: 1.0.0 | **Category**: refactor | **Estimated Duration**: 1-5 days | **Status**: active | **Owner**: Technical Debt Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Systematically identify, prioritize, and reduce technical debt across the codebase. Each paydown cycle targets specific debt categories with measurable improvement goals.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| debt_inventory | Document | Yes | Current technical debt inventory |
| team_capacity | String | Yes | Available capacity: XS/S/M/L |
| focus_areas | String[] | No | Specific debt categories to target |
| improvement_target | Number | No | Target debt score improvement (default: 10%) |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| Debt reduction report | Document | What debt was resolved |
| Updated debt score | Number | New debt measurement |
| New debt items | Document | Any new debt introduced |
| Lessons learned | Document | Patterns to prevent future debt |

## PRECONDITIONS
1. Debt inventory exists and is up to date
2. Team capacity allocated for paydown
3. No conflicting feature deadlines during paydown

## POSTCONDITIONS
1. Debt score reduced by target percentage
2. Debt prevention patterns documented
3. Quality gates updated if needed

## STEPS
### Step 1: Debt Inventory Review
- **Chief**: Technical Debt Chief
- **Specialists**: Code Quality Analyst
- **Task**: Review current debt inventory, prioritize items
- **Output**: Prioritized debt backlog

### Step 2: Plan Selection
- **Chief**: Technical Debt Chief
- **Specialists**: Refactoring Planner
- **Task**: Select debt items for this paydown cycle
- **Output**: Paydown sprint plan

### Step 3: Code Refactoring
- **Chief**: Backend/Frontend Chiefs
- **Specialists**: Service Developers
- **Task**: Execute refactoring per plan
- **Output**: Refactored code

### Step 4: Validation
- **Chief**: Testing Chief
- **Specialists**: Unit/Integration Test Engineers
- **Task**: Verify behavior preservation, update tests
- **Output**: Test results

### Step 5: Code Review
- **Chief**: Review Chief
- **Specialists**: Code Reviewer
- **Task**: Review refactored code for quality
- **Output**: Review approval

### Step 6: Debt Score Update
- **Chief**: Technical Debt Chief
- **Specialists**: Debt Tracker
- **Task**: Re-measure debt score, update inventory
- **Output**: Updated debt metrics

### Step 7: Documentation
- **Chief**: Documentation Chief
- **Specialists**: Technical Writer
- **Task**: Document changes, update ADRs if architecture changed
- **Output**: Documentation updates

## VALIDATION
1. All existing tests pass after refactoring
2. Code complexity reduced (cyclomatic < 10)
3. Deb score improved by target percentage
4. No new debt introduced
5. Architecture integrity maintained

## SUCCESS CRITERIA
- [ ] Debt score reduced by target percentage
- [ ] All selected debt items resolved
- [ ] No behavior changes introduced
- [ ] Tests pass and coverage maintained
- [ ] Debt prevention patterns documented

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Refactoring breaks tests | Rollback change, analyze root cause |
| Debt score miscalculation | Re-audit, adjust measurement methodology |
| Scope creep during paydown | Escalate to Technical Debt Chief |

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |

## RELATED
- [Technical Debt Chief](../departments/technical-debt/SKILL.md)
- [Technical Debt Analysis skill](../skills/code-quality/TECHNICAL_DEBT_ANALYSIS.md)
- [Refactoring skill](../skills/code-quality/REFACTORING.md)
- [refactoring.md](./refactoring.md)
