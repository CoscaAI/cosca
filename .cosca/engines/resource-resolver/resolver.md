# RESOLUTION RULES

> **Version**: 1.0.0 | **Status**: active | **Owner**: Resource Resolver Engine | **Last Updated**: 2026-07-11

## PURPOSE
Defines the exact resolution order and fallback chain for every Virtual Path in the Cosca.

## CORE VIRTUAL PATHS

### `${COSCA_HOME}` — Cosca Installation Root

The directory containing KERNEL.md, engines/, departments/, workflows/, etc.

**Resolution Order:**
1. `$COSCA_HOME` environment variable (explicit override)
2. `cosca.config.yaml` → `paths.COSCA_HOME` (config override)
3. **Linux**: `$XDG_CONFIG_HOME/opencode/cosca/` → `~/.config/opencode/cosca/`
4. **macOS**: `~/Library/Application Support/opencode/cosca/`
5. **Windows**: `%APPDATA%/opencode/cosca/` → `%LOCALAPPDATA%/opencode/cosca/`
6. **Auto-discovery**: Walk up from config file location (`opencode.jsonc` directory + `/cosca/`)
7. **Error**: `COSCA_HOME could not be resolved. Set COSCA_HOME environment variable or ensure opencode.jsonc is in ~/.config/opencode/.`

### `${PROJECT_ROOT}` — Current Workspace

The root directory of the project being managed.

**Resolution Order:**
1. OpenCode workspace directory (provided by runtime)
2. `$PROJECT_ROOT` environment variable
3. **Auto-detect**: Walk up from CWD looking for:
   - `package.json` → project root
   - `.git/` → project root
   - `go.mod` → project root
   - `Cargo.toml` → project root
   - `pyproject.toml` → project root
4. **Fallback**: Current working directory
5. **Error**: `PROJECT_ROOT could not be determined. Ensure you are in a project directory.`

### `${MEMORY_GLOBAL}` — Cross-Project Memory

Stores patterns, bugs, and agent performance data shared across all projects.

**Resolution Order:**
1. `$COSCA_MEMORY_GLOBAL` environment variable
2. `cosca.config.yaml` → `paths.MEMORY_GLOBAL`
3. **Default**: `${COSCA_HOME}/memory/`
4. **Error**: `Global memory path could not be resolved. COSCA_HOME must be resolved first.`

### `${MEMORY_PROJECT}` — Project-Local Memory

Stores project-specific memory, decisions, and architecture records.

**Resolution Order:**
1. `${PROJECT_ROOT}/.cosca/memory/` (always — project-local)
2. **Error**: `Project memory path could not be resolved. PROJECT_ROOT must be determined first.`

---

## DERIVED VIRTUAL PATHS

These are computed from the core paths above:

| Variable | Formula |
|----------|---------|
| `${SKILLS_HOME}` | `${COSCA_HOME}/departments`, `${COSCA_HOME}/engines`, `${COSCA_HOME}/workflows`, `${COSCA_HOME}/company`, `${COSCA_HOME}/memory`, `${COSCA_HOME}/templates`, `${COSCA_HOME}/bootstrap` |
| `${ENGINES_HOME}` | `${COSCA_HOME}/engines` |
| `${DEPARTMENTS_HOME}` | `${COSCA_HOME}/departments` |
| `${WORKFLOWS_HOME}` | `${COSCA_HOME}/workflows` |
| `${TEMPLATES_HOME}` | `${COSCA_HOME}/templates` |
| `${CONFIG_HOME}` | `${COSCA_HOME}` (root level config files) |
| `${BOOTSTRAP_HOME}` | `${COSCA_HOME}/bootstrap` |
| `${SCAFFOLD_HOME}` | `${COSCA_HOME}/.cosca-scaffold` |
| `${SESSIONS_HOME}` | `${MEMORY_PROJECT}/session` |
| `${AUDIT_HOME}` | `${PROJECT_ROOT}/.cosca/audit` |

---

## RESOLUTION CACHING

| Cache Key | TTL | Refresh Trigger |
|-----------|-----|----------------|
| `${COSCA_HOME}` | Session | Never (constant per install) |
| `${PROJECT_ROOT}` | Session | Workspace change |
| `${MEMORY_GLOBAL}` | Session | COSCA_HOME change |
| Derived paths | Lazy | Source path change |

---

## ENVIRONMENT VARIABLE OVERRIDES

All paths can be overridden via environment variables:

```bash
export COSCA_HOME=/custom/path/to/cosca
export PROJECT_ROOT=/custom/project/path
export COSCA_MEMORY_GLOBAL=/shared/memory
export COSCA_CONFIG=/custom/cosca.config.yaml
```

Environment variables take **highest priority** in all resolution chains.

---

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Resource Resolver | Initial resolution rules |
