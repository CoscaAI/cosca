# WORKFLOW: dependency-upgrade

> **Version**: 1.0.0 | **Category**: maintenance | **Estimated Duration**: 1-3 days | **Status**: active | **Owner**: Security Chief | **Last Updated**: 2026-07-23

## OBJECTIVE
Upgrade project dependencies safely and systematically. Ensures compatibility, prevents regressions, and maintains security posture by keeping dependencies current.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| project_path | String | Yes | Path to project with dependencies |
| upgrade_scope | String | Yes | `security-patches`, `minor`, `major`, `all` |
| package_managers | String[] | No | `npm`, `pip`, `go-modules`, `maven`, `cargo`, `nuget` |
| exclude_packages | String[] | No | Packages to exclude from upgrade |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| Upgrade report | Document | All changes made and rationale |
| Compatibility report | Document | Breaking changes and migration notes |
| Changelog entries | Document | Updated changelog |
| Rollback plan | Document | Rollback procedure if issues arise |

## STEPS
### Step 1: Dependency Audit
- **Chief**: Security Chief
- **Specialists**: Dependency Analyst
- **Task**: Audit all dependencies for outdated versions and CVEs
- **Output**: Dependency audit report

### Step 2: Upgrade Planning
- **Chief**: Technical Debt Chief
- **Specialists**: Code Quality Analyst
- **Task**: Categorize upgrades by risk (patch-safe, minor-review, major-migration)
- **Output**: Upgrade plan with risk assessment

### Step 3: Patch Upgrades
- **Chief**: Backend/Frontend Chiefs
- **Specialists**: Service Developers
- **Task**: Apply patch-level upgrades (safe, automated)
- **Output**: Patched dependencies

### Step 4: Minor Version Upgrades
- **Chief**: Backend/Frontend Chiefs
- **Specialists**: Service Developers
- **Task**: Apply minor upgrades with compatibility review
- **Output**: Upgraded dependencies with passing tests

### Step 5: Major Version Upgrades
- **Chief**: Migration Chief
- **Specialists**: Data Migration Engineer, Migration Test Engineer
- **Task**: Plan and execute major version migrations with breaking change handling
- **Output**: Major upgrade with migration guide

### Step 6: Validation
- **Chief**: Testing Chief
- **Specialists**: All testing engineers
- **Task**: Run full test suite, verify no regressions
- **Output**: Test results

### Step 7: Rollback Readiness
- **Chief**: DevOps Chief
- **Specialists**: Release Engineer
- **Task**: Document rollback plan if upgrade causes production issues
- **Output**: Rollback plan

## SUCCESS CRITERIA
- [ ] All security patches applied
- [ ] Minor upgrades passing all tests
- [ ] Major upgrades with migration documentation
- [ ] No regressions in test suite
- [ ] Rollback plan documented
- [ ] Changelog updated

## RELATED
- [Security Chief](../departments/security/SKILL.md)
- [Technical Debt Chief](../departments/technical-debt/SKILL.md)
- [Migration Chief](../departments/migration/SKILL.md)
- [workflows/security-audit.md](./security-audit.md)
- [workflows/release.md](./release.md)

## PRECONDITIONS
1. Dependency audit completed with current version inventory
2. Test suite passing before any upgrades
3. Rollback plan documented before major upgrades

## POSTCONDITIONS
1. All security patches applied and verified
2. Test suite passing after all upgrades
3. Changelog updated with upgrade details
4. Rollback plan preserved for 30 days

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Breaking API change in dependency | Check migration guide, may require architecture review |
| Test regression after upgrade | Rollback dependency, investigate root cause |
| Security patch conflicts with other deps | Escalate to Security Chief for risk acceptance |
| Major upgrade requires significant refactor | Plan as separate refactoring workflow |

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-23 | Cosca Enterprise Evolution | Initial workflow creation |
