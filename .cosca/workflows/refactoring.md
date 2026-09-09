# WORKFLOW: refactoring

> **Version**: 1.1.0 | **Status**: active | **Category**: refactor | **Last Updated**: 2026-07-12

## OBJECTIVE
Improve code quality without changing external behavior — analysis, safety net (tests first), incremental refactoring, verification, and quality comparison.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| target | string | Yes | File, module, or area to refactor |
| reason | string | Yes | Why refactoring is needed |
| scope | string | Yes | targeted (single file), module, system-wide |
| type | string | Yes | extract-method, rename, move, redesign, deduplicate, simplify |
| safety_checks | boolean | No | Run extra safety validations (default: true) |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| refactored_code | object | Changed files |
| migration_guide | object | Guide if breaking changes to interfaces |
| quality_report | object | Before/after quality metrics comparison |

## PRECONDITIONS
1. All tests passing before refactoring
2. Comprehensive test coverage exists for target area
3. Git state is clean (no uncommitted changes)

## POSTCONDITIONS
1. All tests still passing
2. Behavior is unchanged
3. Code quality metrics improved
4. No new technical debt introduced

## DEPENDENCIES
None

## STEPS

### Step 1: Analysis
- **Chief**: Architecture
- **Specialists**: Architecture Reviewer
- **Task**: Analyze code to understand structure, dependencies, and impact
- **Output**: Analysis report with refactoring approach

### Step 2: Safety Net
- **Chief**: Testing
- **Specialists**: Unit Test Engineer
- **Task**: Ensure comprehensive test coverage exists for target
- **Output**: Test coverage report and gap analysis
- **Note**: If gaps found, add tests first before refactoring

### Step 3: Refactoring Execution
- **Chief**: Backend or Frontend (based on target)
- **Specialists**: Relevant specialist
- **Task**: Execute refactoring in small, safe steps
- **Output**: Refactored code
- **Rule**: Each step must keep tests passing; commit after each step

### Step 4: Verification
- **Chief**: Testing
- **Specialists**: All test engineers
- **Task**: Run full test suite, verify no regression
- **Output**: Test results

### Step 5: Code Review
- **Chief**: Review
- **Specialists**: Code Reviewer, Architecture Reviewer
- **Task**: Review refactored code for quality and compliance
- **Output**: Review report

### Step 6: Quality Comparison
- **Chief**: QA
- **Specialists**: Test Automation Engineer
- **Task**: Compare before/after quality metrics
- **Output**: Quality comparison report (complexity, duplication, coverage, maintainability)

### Step 7: Documentation
- **Chief**: Documentation
- **Specialists**: Technical Writer
- **Task**: Update documentation if interfaces changed; create migration guide if needed
- **Output**: Updated documentation

## VALIDATION
1. All tests pass (same or better coverage)
2. Behavior is identical
3. Quality metrics improved (complexity ≤, duplication ≤, coverage ≥)
4. Architecture compliance maintained

## SUCCESS CRITERIA
- [ ] All tests pass
- [ ] Behavior unchanged
- [ ] Quality metrics improved:
  - Cyclomatic complexity: decreased or equal
  - Code duplication: decreased
  - Test coverage: maintained or increased
- [ ] Code review approved
- [ ] No new issues detected

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Insufficient test coverage | Add tests first (Safety Net step), block refactoring until coverage adequate |
| Test failure during refactoring | Revert last step, analyze cause, retry with smaller step |
| Behavior change detected | Revert, identify root cause, re-plan approach |
| Review rejection | Address feedback, re-submit |
| Quality metrics worse | Revert or iterate until metrics improve |
| Git conflicts mid-refactoring | Stash changes, resolve conflicts, re-apply refactoring steps |

## RELATED
- [Evolution Engine](../engines/evolution/SKILL.md) — Detects refactoring candidates
- [QUALITY_GATES.md](../identidade/QUALITY_GATES.md) — Quality metrics and thresholds
- [Architecture Chief](../departments/architecture/SKILL.md) — Refactoring approach design
- [Testing Chief](../departments/testing/SKILL.md) — Safety net test coverage

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Kernel | Initial workflow definition (7 steps) |
| 1.1.0 | 2026-07-12 | Cosca Kernel | Updated to canonical format: added History, Error Handling, Related, Status/Category, quality metrics in success criteria |
