# WORKFLOW: release

> **Version**: 1.0.0 | **Status**: active | **Category**: deploy | **Last Updated**: 2026-07-10

## OBJECTIVE
Prepare and execute a software release, including version bump, changelog generation, QA sign-off, and deployment coordination.

## INPUTS
| Name | Type | Required | Description |
|------|------|----------|-------------|
| version | string | Yes | Semantic version for the release (e.g., 1.2.0) |
| release_type | string | Yes | major, minor, patch |
| release_notes | string | No | Pre-written notes; auto-generated if empty |
| deploy_target | string | No | Environment to deploy (production, staging) |

## OUTPUTS
| Name | Type | Description |
|------|------|-------------|
| version_tag | string | Git tag created |
| changelog | object | Updated CHANGELOG.md |
| release_notes | object | Generated release notes |
| deployment_status | object | Deployment verification report |

## PRECONDITIONS
1. All tests passing on main branch
2. QA sign-off obtained
3. Security review passed
4. Documentation updated
5. Release branch merged to main

## POSTCONDITIONS
1. Version tag created
2. Changelog updated
3. Release notes published
4. Deployment verified (if applicable)
5. Stakeholders notified

## DEPENDENCIES
| Workflow | Reason |
|----------|--------|
| feature-development | Features must be completed before release |

## STEPS

### Step 1: Release Validation
- **Chief**: Release
- **Specialists**: Release Manager
- **Task**: Verify all preconditions met (tests, QA, security, docs)
- **Output**: Validation report

### Step 2: Version Bump
- **Chief**: Release
- **Specialists**: Version Manager
- **Task**: Update version in package files, create version commit
- **Output**: Version bump commit

### Step 3: Changelog Generation
- **Chief**: Documentation
- **Specialists**: Changelog Manager
- **Task**: Generate changelog entry from commits since last release
- **Output**: Updated CHANGELOG.md

### Step 4: Release Notes
- **Chief**: Documentation
- **Specialists**: Technical Writer
- **Task**: Create release notes highlighting new features, fixes, breaking changes
- **Output**: Release notes document

### Step 5: Final QA Sign-off
- **Chief**: QA
- **Specialists**: Test Automation Engineer
- **Task**: Run complete test suite, provide final sign-off
- **Output**: QA sign-off document

### Step 6: Deployment
- **Chief**: DevOps
- **Specialists**: Release Engineer, Deployment Coordinator
- **Task**: Deploy release to target environment
- **Output**: Deployment confirmation

### Step 7: Post-Deployment Verification
- **Chief**: Monitoring
- **Specialists**: Monitoring Engineer
- **Task**: Verify health checks, error rates, performance
- **Output**: Post-deployment verification report

### Step 8: Notification
- **Chief**: Release
- **Specialists**: Release Manager
- **Task**: Notify stakeholders, update status
- **Output**: Notification sent

## VALIDATION
1. All preconditions verified
2. Version bump correct (semantic versioning)
3. Changelog accurate and complete
4. Tests passing on release tag
5. Deployment healthy (Gate 4 from QUALITY_GATES.md)
6. Rollback tested (if applicable)

## SUCCESS CRITERIA
- [ ] Version tag created and pushed
- [ ] CHANGELOG.md updated
- [ ] Release notes published
- [ ] Deployment successful
- [ ] Post-deployment health verified
- [ ] Rollback plan available

## ERROR HANDLING
| Failure | Action |
|---------|--------|
| Test failure | Abort release, fix issues, retry |
| Deployment failure | Execute rollback plan |
| Post-deployment issues | Assess severity; rollback if critical |
| Version conflict | Resolve manually, retry bump |

## RELATED
- [Release Chief](../departments/release/SKILL.md)
- [QA Chief](../departments/qa/SKILL.md)
- [DevOps Chief](../departments/devops/SKILL.md)
- [QUALITY_GATES.md](../QUALITY_GATES.md)

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Initial release workflow |
