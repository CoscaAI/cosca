# PROJECT VALIDATOR

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Bootstrap

## PURPOSE
Validate that the workspace contains a valid project structure and is ready for Cosca initialization.

## VALIDATION CHECKS

### Structure Checks
| Check | Requirement | Severity |
|-------|-------------|----------|
| Not empty directory | At least 1 file present | Error |
| Config file exists | package.json / pyproject.toml / Cargo.toml / go.mod / etc. | Warn |
| Not node_modules only | Has source files beyond dependencies | Warn |
| Has README | README.md or equivalent | Info |
| Has .gitignore | .gitignore present | Info |

### Project Integrity
| Check | Requirement | Severity |
|-------|-------------|----------|
| Config file parseable | JSON/YAML/TOML valid | Error |
| Lock file matches config | Lock file present and consistent | Warn |
| No duplicate configs | Single config per type | Warn |

### Cosca Compatibility
| Check | Requirement | Severity |
|-------|-------------|----------|
| `.cosca/` not conflicting | No existing `.cosca/` with corruption | Error |
| Write permissions | Can create files in directory | Error |
| No name conflict | Workspace name valid for Cosca | Info |

## OUTPUT
```yaml
validation:
  status: pass | warn | fail
  checks:
    passed: 7
    warnings: 2
    failed: 0
  issues: []
```

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Cosca Bootstrap | Initial validator |
