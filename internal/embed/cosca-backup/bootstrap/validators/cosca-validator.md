# Cosca VALIDATOR

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Bootstrap

## PURPOSE
Validate that all Cosca components are functioning correctly before attempting workspace initialization.

## VALIDATION CHECKS

### Core Components
| Component | Check | Severity |
|-----------|-------|----------|
| KERNEL.md | File exists and has PURPOSE section | Critical |
| Skills Engine | File exists and has SKILL LIFECYCLE section | Critical |
| Cosca Loader Plugin | Plugin registered, skills loaded | Critical |

### Engine Components
| Engine | File | Severity |
|--------|------|----------|
| Context Engine | engines/context/SKILL.md | High |
| Memory Engine | engines/memory/SKILL.md | High |
| Workflow Engine | engines/workflow/SKILL.md | Medium |
| Discovery Engine | engines/discovery/WORKSPACE.md | High |
| Observability Engine | engines/observability/SKILL.md | Low |

### Skill Registry
| Check | Requirement | Severity |
|-------|-------------|----------|
| Departments | 26 SKILL.md files present | Medium |
| Engines | 18 SKILL.md files present | Medium |
| Workflows | 10 workflow files present | Medium |
| Templates | 9 TEMPLATE.md files present | Low |
| Governance | 7 governance files present | Medium |

### Configuration
| Check | Requirement | Severity |
|-------|-------------|----------|
| opencode.jsonc | Valid JSONC, schema-compliant | Critical |
| Agent configs | At least Kernel + 5 chiefs defined | High |
| Plugin config | cosca-loader in plugin array | Critical |
| Permissions | Read/write allowed for .cosca/ | High |

## OUTPUT
```yaml
validation:
  aos_status: healthy | degraded | critical
  core_components: 3/3
  engine_components: 5/5
  skills_available: 80/80
  agents_defined: 35/35
  config_valid: true
```

## HISTORY
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Cosca Bootstrap | Initial validator |
