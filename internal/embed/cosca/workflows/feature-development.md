# WORKFLOW: feature-development

> **Version**: 1.1.0 | **Status**: active | **Category**: feature | **Last Updated**: 2026-07-12

## OBJECTIVE
Full lifecycle of a feature from requirement to delivery — wizard intake, planning, architecture, implementation, testing, review, QA, documentation, and release.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| feature_name | string | Yes | Feature name/identifier |
| feature_description | string | Yes | What the feature should do |
| priority | string | Yes | critical, high, medium, low |
| constraints | object | No | Technical or business constraints |
| deadline | string | No | Target completion date |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| executive_plan | object | Complete execution plan |
| implementation | object | Implemented code |
| tests | object | Test suite |
| documentation | object | Updated documentation |
| review_report | object | Review results (Gate 2) |
| qa_report | object | QA test results (Gate 3) |

## PRECONDITIONS
1. Project exists and is Cosca-managed
2. Current codebase is in a stable state
3. All tests passing on main branch

## POSTCONDITIONS
1. Feature is implemented
2. All tests pass (including new tests)
3. Code review approved
4. QA sign-off obtained
5. Documentation updated
6. Feature branch merged to main

## DEPENDENCIES
None (but may depend on other features based on plan)

## STEPS

### Phase 1: REQUIREMENTS
#### Step 1.1: Requirement Analysis
- **Chief**: Product
- **Specialists**: Business Analyst
- **Task**: Analyze feature request, define requirements
- **Output**: Requirements document

#### Step 1.2: Wizard Intake
- **Chief**: Product
- **Engine**: Wizard Engine
- **Task**: Complete 14-phase wizard for detailed specification
- **Output**: Wizard output document

### Phase 2: PLANNING
#### Step 2.1: Technical Feasibility
- **Chief**: CTO
- **Specialists**: Tech Lead
- **Task**: Assess technical feasibility and constraints
- **Output**: Feasibility report

#### Step 2.2: Executive Plan
- **Chief**: CTO
- **Engine**: Planning Engine
- **Task**: Generate complete executive plan with dependency graph
- **Output**: Executive Plan document

#### Step 2.3: Plan Review & Approval
- **Chief**: CEO
- **Task**: Review and approve plan (Gate 1)
- **Output**: Approved plan

### Phase 3: ARCHITECTURE
#### Step 3.1: Solution Design
- **Chief**: Architecture
- **Specialists**: Solutions Architect
- **Task**: Design technical solution
- **Output**: Architecture design, ADR

#### Step 3.2: Architecture Review
- **Chief**: Review
- **Specialists**: Architecture Reviewer
- **Task**: Review architecture for compliance
- **Output**: Architecture review report

### Phase 4: IMPLEMENTATION
#### Step 4.1: Database Changes
- **Chief**: Database
- **Specialists**: SQL Developer, Data Migration Engineer
- **Task**: Design and implement schema changes
- **Output**: Migration files
- **Depends On**: Step 3.1

#### Step 4.2: Backend Implementation
- **Chief**: Backend
- **Specialists**: API Developer, Service Developer
- **Task**: Implement backend changes
- **Output**: Backend code
- **Depends On**: Step 3.1, Step 4.1

#### Step 4.3: Frontend Implementation
- **Chief**: Frontend
- **Specialists**: Component Developer
- **Task**: Implement frontend changes
- **Output**: Frontend code
- **Depends On**: Step 4.2

#### Step 4.4: UI/UX Design (parallel with 4.2)
- **Chief**: UI/UX
- **Specialists**: UI Designer
- **Task**: Design UI components and flows
- **Output**: Design specs
- **Depends On**: Step 1.2

### Phase 5: QUALITY
#### Step 5.1: Unit & Integration Tests
- **Chief**: Testing
- **Specialists**: Unit Test Engineer, Integration Test Engineer
- **Task**: Write and run tests
- **Output**: Test suite, coverage report
- **Depends On**: Step 4.2, Step 4.3

#### Step 5.2: Code Review
- **Chief**: Review
- **Specialists**: Code Reviewer, Security Reviewer
- **Task**: Review all code changes (Gate 2)
- **Output**: Review report
- **Depends On**: Step 4.2, Step 4.3, Step 5.1

#### Step 5.3: QA Testing
- **Chief**: QA
- **Specialists**: Test Automation Engineer, Manual QA
- **Task**: Run QA test suite
- **Output**: QA report
- **Depends On**: Step 5.2

#### Step 5.4: Security Review
- **Chief**: Security
- **Specialists**: Security Engineer
- **Task**: Security audit of all changes
- **Output**: Security report
- **Depends On**: Step 4.2

### Phase 6: DOCUMENTATION
#### Step 6.1: Documentation Update
- **Chief**: Documentation
- **Specialists**: Technical Writer, API Documenter, Diagram Creator
- **Task**: Update all affected documentation
- **Output**: Updated documentation
- **Depends On**: Step 5.3

### Phase 7: DELIVERY
#### Step 7.1: Release Preparation
- **Chief**: Release
- **Specialists**: Release Manager
- **Task**: Prepare release, update changelog
- **Output**: Release notes, changelog
- **Depends On**: Step 6.1

#### Step 7.2: Final Approval
- **Chief**: CEO
- **Task**: Final review and approval
- **Output**: Approval
- **Depends On**: Step 7.1

#### Step 7.3: Merge
- **Chief**: DevOps
- **Specialists**: Release Engineer
- **Task**: Merge feature branch to main
- **Output**: Merged code
- **Depends On**: Step 7.2

## VALIDATION
1. All acceptance criteria met
2. All quality gates passed (Gate 0-3)
3. All documentation updated
4. No regressions in existing functionality
5. Code coverage ≥ 80% on changed code
6. Security scan clean (0 critical/high)

## SUCCESS CRITERIA
- [ ] Feature works as specified
- [ ] All tests pass
- [ ] Code review approved (Gate 2)
- [ ] QA sign-off obtained (Gate 3)
- [ ] Security review passed
- [ ] Performance benchmarks met
- [ ] Documentation updated
- [ ] Merged to main

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Wizard incomplete | Re-run wizard phases, request clarification from user |
| Architecture rejected | Return to Architecture Chief with feedback |
| Implementation bugs | Route to bug-fix workflow |
| Test failures | Return to implementing chief with test report |
| Review rejection | Address issues, re-submit for review |
| QA failure | Fix issues, re-run QA suite |
| Security vulnerability | Escalate to Security Chief, block merge |
| Merge conflict | Resolve conflicts, re-run tests |

## RELATED
- [Wizard Engine](../engines/wizard/SKILL.md) — Feature intake (14 phases)
- [Planning Engine](../engines/planning/SKILL.md) — Executive plan generation
- [Review Engine](../engines/review/SKILL.md) — Gate 2 enforcement
- [Quality Engine](../engines/quality/SKILL.md) — Gate 3 enforcement
- [QUALITY_GATES.md](../QUALITY_GATES.md) — Canonical gate definitions
- [Release Workflow](release.md) — Post-feature release process

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Kernel | Initial workflow definition (16 steps, 7 phases) |
| 1.1.0 | 2026-07-12 | Cosca Kernel | Updated to canonical format: added History, Error Handling, Related, Status/Category, quality gate references |
