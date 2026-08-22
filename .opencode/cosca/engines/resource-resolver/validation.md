# PATH VALIDATION

> **Version**: 1.0.0 | **Status**: active | **Owner**: Resource Resolver Engine | **Last Updated**: 2026-07-11

## PURPOSE
Validate that all resolved Virtual Paths point to existing, accessible locations with correct structure.

---

## VALIDATION RULES

### `${COSCA_HOME}` Validation
| Check | Requirement | Severity |
|-------|-------------|----------|
| Directory exists | `${COSCA_HOME}/` is a directory | Critical |
| KERNEL.md present | `${COSCA_HOME}/KERNEL.md` exists | Critical |
| Engines directory | `${COSCA_HOME}/engines/` exists and has files | Critical |
| Departments directory | `${COSCA_HOME}/departments/` exists and has files | Critical |
| COSCA_INDEX.md present | `${COSCA_HOME}/COSCA_INDEX.md` exists | High |
| GOVERNANCE.md present | `${COSCA_HOME}/GOVERNANCE.md` exists | Medium |

### `${PROJECT_ROOT}` Validation
| Check | Requirement | Severity |
|-------|-------------|----------|
| Directory exists | `${PROJECT_ROOT}/` is a directory | Critical |
| Write permission | Can create files in `${PROJECT_ROOT}/` | Critical |
| Not system directory | Not `/`, `/etc`, `/usr`, `C:\Windows` | Critical |
| Has project files | At least 1 source file present | Warn |

### `${MEMORY_GLOBAL}` Validation
| Check | Requirement | Severity |
|-------|-------------|----------|
| Directory exists | `${MEMORY_GLOBAL}/` is a directory | Critical |
| Pattern memory | `${MEMORY_GLOBAL}/pattern/INDEX.md` exists | Medium |
| Bug memory | `${MEMORY_GLOBAL}/bug/INDEX.md` exists | Medium |
| Agent memory | `${MEMORY_GLOBAL}/agent/INDEX.md` exists | Medium |
| Write permission | Can create files | High |

### `${MEMORY_PROJECT}` Validation
| Check | Requirement | Severity |
|-------|-------------|----------|
| `.cosca/` exists | `${PROJECT_ROOT}/.cosca/` directory | Warn |
| `.cosca/memory/` exists | Subdirectory | Warn |
| `.cosca/config.yml` valid | YAML parseable | Medium |
| `.cosca/state.yml` valid | YAML parseable | Medium |

---

## DIAGNOSTIC OUTPUT

### On Validation Success
```
✅ All Virtual Paths validated
   COSCA_HOME: /home/user/.config/opencode/cosca/ [OK]
   PROJECT_ROOT: /home/user/projects/app/ [OK]
   MEMORY_GLOBAL: ${COSCA_HOME}/memory/ [OK]
   MEMORY_PROJECT: .cosca/memory/ [OK]
   SKILLS_HOME: 80 skills loaded [OK]
```

### On Validation Failure
```
❌ Path validation failed

Critical:
  - COSCA_HOME not found at /home/user/.config/opencode/cosca/
    → KERNEL.md missing. Cosca may not be installed.
    → Run: cosca install

Warnings:
  - MEMORY_PROJECT not initialized (first session?)
    → Will be created by Bootstrap Phase 2

Suggestions:
  - Set COSCA_HOME=/correct/path if Cosca is installed elsewhere
  - Verify cosca-loader plugin is installed in node_modules
```

---

## AUTO-REPAIR

The Resource Resolver can attempt to repair certain validation failures:

| Failure | Auto-Repair |
|---------|-------------|
| `.cosca/` missing | Create from scaffold |
| `.cosca/memory/` missing | Create empty directory |
| `.cosca/config.yml` missing | Copy from scaffold template |
| `.cosca/state.yml` missing | Copy from scaffold template |
| Memory index missing | Create empty INDEX.md |

Auto-repair is logged and reported to user.

---

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Resource Resolver | Initial validation rules |
