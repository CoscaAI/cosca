# WORKFLOW: code-review

> **Version**: 1.1.0 | **Status**: active | **Category**: review | **Last Updated**: 2026-07-12

## OBJECTIVE
Comprehensive multi-dimensional review of code changes — automated checks, architecture, code quality, security, performance, testing, and documentation.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| target | string | Yes | Branch, commit range, or files to review |
| review_type | string | Yes | full (all dimensions), architecture, security, performance, quick (high-level) |
| context | string | No | What is this change supposed to do? |
| previous_review_id | string | No | If re-reviewing after changes |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| review_report | object | Comprehensive review report with scores |
| issues | array | Issues found (critical, warning, suggestion) |
| approval | boolean | Whether the change is approved |
| compliance_score | object | Scores per dimension (0-10) |

## PRECONDITIONS
1. Code changes are submitted for review
2. Automated checks have run (linting, typing, tests)

## POSTCONDITIONS
1. All review dimensions evaluated
2. Issues documented with severity and recommendations
3. Approval decision rendered
4. Review stored in memory for trend analysis

## DEPENDENCIES
None

## STEPS

### Step 1: Automated Checks
- **Chief**: Review
- **Tool**: Automated (linter, type checker, formatter, test runner)
- **Task**: Run all automated quality checks
- **Output**: Automated check results (pass/fail per tool)

### Step 2: Architecture Review
- **Chief**: Review
- **Specialists**: Architecture Reviewer
- **Task**: Check architecture compliance per QUALITY_GATES.md Gate 2.1
- **Output**: Architecture review notes with compliance score

### Step 3: Code Quality Review
- **Chief**: Review
- **Specialists**: Code Reviewer
- **Task**: Review code quality per QUALITY_GATES.md Gate 2.2
- **Output**: Code quality review notes with quality score

### Step 4: Security Review
- **Chief**: Review
- **Specialists**: Security Reviewer
- **Task**: Check for security issues per QUALITY_GATES.md Gate 2.3
- **Output**: Security review notes with security score

### Step 5: Performance Review
- **Chief**: Review
- **Specialists**: Code Reviewer
- **Task**: Check for performance issues per QUALITY_GATES.md Gate 2.4
- **Output**: Performance review notes with performance score

### Step 6: Test Review
- **Chief**: Review
- **Specialists**: Code Reviewer
- **Task**: Review test quality and coverage per QUALITY_GATES.md Gate 2.5
- **Output**: Test review notes with test score

### Step 7: Documentation Review
- **Chief**: Review
- **Specialists**: Standards Reviewer
- **Task**: Check documentation completeness per QUALITY_GATES.md Gate 2.6
- **Output**: Documentation review notes with documentation score

### Step 8: Report Generation
- **Chief**: Review
- **Task**: Compile all reviews into final report with overall score
- **Output**: Review report with APPROVED / CHANGES REQUESTED / REJECTED

## REVIEW DIMENSIONS (Detailed)

### Architecture Compliance
- Module boundaries respected
- Dependency direction correct (→ stable abstractions)
- Patterns applied correctly
- Single Responsibility Principle
- Interface segregation
- ADR compliance

### Code Quality
- Naming: Clear, consistent, conventional
- Functions: Small (< 50 lines), focused, pure when possible
- Classes: Single responsibility, proper encapsulation
- Error handling: Comprehensive, appropriate
- Comments: Explain "why", not "what"
- No dead code, no commented-out code
- No magic numbers (use named constants)

### Security (OWASP Top 10)
- Injection prevention (SQL, XSS, command)
- Authentication and session management
- Sensitive data exposure
- Broken access control
- Security misconfiguration
- Vulnerable components
- Insufficient logging and monitoring

### Performance
- N+1 queries: 0 instances
- Missing indexes: 0 tables without proper indexes
- Unnecessary allocations: No large objects in hot paths
- Blocking operations: No sync ops in async contexts
- Missing caching opportunities
- Large payloads without pagination

### Testing
- Happy path covered
- Edge cases covered (≥ 2)
- Error paths covered (≥ 1)
- Tests are independent (no test depends on another)
- Tests are deterministic (0 flaky tests)
- Appropriate use of mocks (external deps only)

### Documentation
- API docs: All new/changed endpoints documented
- ADR: Created if architecture decision made
- README: Updated if project structure changed
- Changelog: Entry added for the change
- Code comments: Explain "why", not "what"

## VALIDATION
1. All automated checks pass
2. All 6 review dimensions evaluated
3. Issues categorized by severity
4. Overall compliance score calculated per QUALITY_GATES.md formula

## SUCCESS CRITERIA
- [ ] All automated checks pass
- [ ] Overall score ≥ 7.0 (B or higher)
- [ ] No critical issues unresolved
- [ ] All warnings have documented acceptance or fix plan

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Automated checks fail | Block review, return to author for fixes |
| Critical security issue | Immediate rejection, escalate to Security Chief |
| Architecture violation | Request changes, escalate to Architecture Chief if pattern persists |
| Insufficient test coverage | Request additional tests, block approval |
| Review timeout | Generate partial report, flag unreviewed dimensions |
| Reviewer agent failure | Assign secondary reviewer, continue from last completed step |

## RELATED
- [QUALITY_GATES.md](../QUALITY_GATES.md) — Gate 2 canonical definitions, thresholds, and scoring formula
- [Review Engine](../engines/review/SKILL.md) — Automated review enforcement
- [Review Chief](../departments/review/SKILL.md) — Review orchestration
- [Security Chief](../departments/security/SKILL.md) — Security review escalation
- [Architecture Chief](../departments/architecture/SKILL.md) — Architecture compliance standards

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Kernel | Initial workflow definition (8 steps, 6 review dimensions) |
| 1.1.0 | 2026-07-12 | Cosca Kernel | Updated to canonical format: added History, Error Handling, Related, Status/Category, PRECONDITIONS, POSTCONDITIONS, references to QUALITY_GATES.md |
