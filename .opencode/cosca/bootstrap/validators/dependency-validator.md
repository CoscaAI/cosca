# DEPENDENCY VALIDATOR

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Bootstrap

## PURPOSE
Validate project dependencies for health, security, and compatibility before Cosca initialization.

## VALIDATION CHECKS

### Dependency Health
| Check | Requirement | Severity |
|-------|-------------|----------|
| Lock file present | package-lock.json / yarn.lock / pnpm-lock.yaml | Warn |
| Lock file up to date | Hash matches package.json | Warn |
| No missing peer deps | Peer dependency warnings == 0 | Warn |
| No deprecated packages | Deprecation warnings == 0 (or flagged) | Info |

### Security
| Check | Requirement | Severity |
|-------|-------------|----------|
| No critical CVEs | 0 critical vulnerabilities | Error |
| No high CVEs | 0 high vulnerabilities | Warn |
| Dependency audit passing | npm audit / pip audit clean | Warn |

### Version Consistency
| Check | Requirement | Severity |
|-------|-------------|----------|
| Framework version stable | Not on alpha/beta/rc (unless explicit) | Info |
| TypeScript version compatible | ^4.9+ or ^5.0+ | Info |
| Node version compatible | Matches engines field in package.json | Warn |

### Outdated Dependencies
| Check | Threshold | Severity |
|-------|-----------|----------|
| Major version behind | < 3 packages | Info |
| Minor version behind | < 10 packages | Info |
| Patch version behind | Any count | Info |

## OUTPUT
```yaml
validation:
  status: pass | warn | fail
  dependency_count: 42
  outdated:
    major: 2
    minor: 7
    patch: 12
  vulnerabilities:
    critical: 0
    high: 0
    medium: 1
    low: 3
  recommendations:
    - "Update react from 18.2.0 to 18.3.1 (patch)"
    - "Review medium CVE in axios@1.6.0"
```

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Cosca Bootstrap | Initial validator |
