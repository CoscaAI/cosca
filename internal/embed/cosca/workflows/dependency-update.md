# WORKFLOW: dependency-update

> **Version**: 1.0.0 | **Status**: active | **Category**: maintenance | **Last Updated**: 2026-07-10

## OBJECTIVE
Safely update project dependencies with automated testing, security scanning, and rollback capability.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| packages | array | No | Specific packages to update; all if empty |
| update_type | string | No | patch, minor, major, all |
| auto_merge | boolean | No | Auto-merge if all checks pass (default: false) |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| updated_deps | array | List of updated packages with old/new versions |
| audit_report | object | Security audit results |
| test_results | object | Test suite results |
| breaking_changes | array | Breaking changes detected |

## PRECONDITIONS
1. All tests passing on main branch
2. Git state clean (no uncommitted changes)
3. Lock file present (package-lock.json, yarn.lock, etc.)

## POSTCONDITIONS
1. Dependencies updated
2. Lock file regenerated
3. Tests passing with updated deps
4. Security audit clean
5. Branch created or changes committed

## DEPENDENCIES
None

## STEPS

### Step 1: Dependency Audit
- **Chief**: Security
- **Specialists**: Security Engineer
- **Task**: Run security audit on current and proposed dependencies
- **Output**: Current vulnerability report

### Step 2: Update Execution
- **Chief**: DevOps
- **Specialists**: CI/CD Engineer
- **Task**: Run package manager update, generate lock file diff
- **Output**: Updated lock file, version diff report

### Step 3: Breaking Change Detection
- **Chief**: Architecture
- **Specialists**: Solutions Architect
- **Task**: Analyze changelogs for breaking changes in major version bumps
- **Output**: Breaking change impact report

### Step 4: Test Suite Execution
- **Chief**: Testing
- **Specialists**: Unit Test Engineer, Integration Test Engineer
- **Task**: Run full test suite with updated dependencies
- **Output**: Test results and coverage report

### Step 5: Build Verification
- **Chief**: DevOps
- **Specialists**: CI/CD Engineer
- **Task**: Verify build succeeds with updated dependencies
- **Output**: Build status

### Step 6: Security Re-Audit
- **Chief**: Security
- **Specialists**: Security Engineer
- **Task**: Run security audit on updated dependency tree
- **Output**: Post-update vulnerability report

### Step 7: Merge or Report
- **Chief**: DevOps
- **Specialists**: Release Engineer
- **Task**: Create PR with changes or auto-merge if enabled and all checks pass
- **Output**: PR created or changes merged

## VALIDATION
1. All tests pass with updated dependencies
2. Build succeeds
3. No new critical/high vulnerabilities introduced
4. No breaking changes in patch/minor updates
5. Breaking changes documented for major updates

## SUCCESS CRITERIA
- [ ] Dependencies updated successfully
- [ ] All tests pass
- [ ] Build succeeds
- [ ] Security audit clean
- [ ] Breaking changes documented (if any)

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Test failure | Report failed tests, suggest rollback |
| Security vulnerability | Block update, report vulnerability |
| Build failure | Report build errors, suggest fixes |
| Breaking change (unexpected) | Block update, create migration guide |

## RELATED
- [Security Chief](../departments/security/SKILL.md)
- [DevOps Chief](../departments/devops/SKILL.md)
- [Architecture Chief](../departments/architecture/SKILL.md)

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Initial dependency-update workflow |
