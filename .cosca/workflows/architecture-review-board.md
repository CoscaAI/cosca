# WORKFLOW: architecture-review-board

> **Version**: 1.0.0 | **Category**: review | **Estimated Duration**: 1-3 days | **Status**: active | **Owner**: Architecture Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Formal architecture review process for significant system changes. Ensures all major architecture decisions are reviewed by the Architecture Council, documented as ADRs, and aligned with organizational standards.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| proposal | Document | Yes | Architecture proposal with context, options, recommendation |
| change_type | String | Yes | `new-service`, `breaking-api`, `new-technology`, `migration`, `deprecation`, `architecture-change` |
| impact_analysis | Document | Yes | Cross-department impact assessment |
| affected_departments | String[] | Yes | Departments affected by the change |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| ARB decision | String | Approved, Changes-Required, Rejected, Deferred |
| Review minutes | Document | ARB discussion summary and rationale |
| ADR | Document | Architecture Decision Record (if approved) |
| Action items | Document | Required changes before implementation |

## STEPS
### Step 1: Proposal Preparation
- **Chief**: Proposing Chief
- **Specialists**: Architecture Analyst
- **Task**: Prepare architecture proposal with context, options, recommendation
- **Output**: Architecture proposal document

### Step 2: Pre-Review
- **Chief**: Architecture Chief
- **Specialists**: Solutions Architect
- **Task**: Initial review for completeness, identify missing information
- **Output**: Pre-review feedback

### Step 3: ARB Meeting
- **Chief**: Architecture Chief (Chair)
- **Specialists**: Architecture Council members
- **Task**: Present proposal, discuss alternatives, assess impact
- **Output**: Meeting minutes

### Step 4: Decision
- **Chief**: Architecture Chief
- **Specialists**: Architecture Council
- **Task**: Vote and document decision
- **Output**: ARB decision document

### Step 5: ADR Creation
- **Chief**: Architecture Chief
- **Specialists**: Technical Writer
- **Task**: Create ADR documenting the approved decision
- **Output**: ADR document

### Step 6: Communication
- **Chief**: Architecture Chief
- **Specialists**: All affected Chiefs
- **Task**: Communicate decision and action items to all stakeholders
- **Output**: Communication summary

## SUCCESS CRITERIA
- [ ] Proposal submitted with complete context and options
- [ ] Architecture Council reviewed and discussed
- [ ] Decision documented with rationale
- [ ] ADR created and stored in architecture memory
- [ ] Action items assigned with owners and deadlines

## RELATED
- [Architecture Chief](../departments/architecture/SKILL.md)
- [councils/COUNCILS.md](../councils/COUNCILS.md) — Architecture Council
- [skills/architecture/ADR_GENERATION.md](../skills/architecture/ADR_GENERATION.md)
- [skills/architecture/ARCHITECTURE_VALIDATION.md](../skills/architecture/ARCHITECTURE_VALIDATION.md)

## PRECONDITIONS
1. Architecture proposal prepared with context and options
2. Impact analysis completed for all affected departments
3. Required stakeholders identified and available for review

## POSTCONDITIONS
1. Decision documented in ADR format
2. Action items assigned with owners and deadlines
3. Decision communicated to all affected teams

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Incomplete proposal | Return to proposing chief with gaps list |
| No consensus | Escalate to Executive Council |
| Missing stakeholders | Reschedule with complete attendance |

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |
