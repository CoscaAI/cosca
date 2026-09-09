---
name: resource-resolver
description: Eliminates hardcoded paths by resolving Virtual Paths at runtime based on the environment.
level: 2
---

# RESOURCE RESOLVER ENGINE

> **Version**: 1.0.0 | **Status**: active | **Owner**: Resource Resolver Engine | **Last Updated**: 2026-07-11

## PURPOSE
The Resource Resolver Engine eliminates all hardcoded paths from the Cosca. Instead of physical paths like `/home/user/.config/opencode/cosca/`, all components reference **Virtual Paths** that are resolved at runtime based on the environment.

**No component shall ever use a physical path. Only Virtual Paths.**

## ACTIVATION
- On Bootstrap (Phase 0, before any other engine)
- On any path resolution request
- On environment change detection

## SCOPE
- Virtual Path definition and registration
- Runtime path resolution (OS-aware)
- Environment variable override support
- Path validation and existence checking
- Path caching for performance

## OUT OF SCOPE
- File I/O (delegate to Memory Engine / filesystem tools)
- Project scaffolding (delegate to Template Engine)
- Build configuration (delegate to DevOps Chief)

---

## VIRTUAL PATH SYSTEM

### Path Variables

| Variable | Resolves To | Scope |
|----------|-------------|-------|
| `${COSCA_HOME}` | Cosca installation root directory | Global |
| `${PROJECT_ROOT}` | Current workspace root | Session |
| `${MEMORY_GLOBAL}` | Cross-project memory (patterns, bugs, agents) | Global |
| `${MEMORY_PROJECT}` | Project-local memory (`.cosca/memory/`) | Project |
| `${SKILLS_HOME}` | Skills directory (`${COSCA_HOME}/departments`, `${COSCA_HOME}/engines`, etc.) | Global |
| `${ENGINES_HOME}` | Engines directory (`${COSCA_HOME}/engines`) | Global |
| `${DEPARTMENTS_HOME}` | Departments directory (`${COSCA_HOME}/departments`) | Global |
| `${WORKFLOWS_HOME}` | Workflows directory (`${COSCA_HOME}/workflows`) | Global |
| `${TEMPLATES_HOME}` | Templates directory (`${COSCA_HOME}/templates`) | Global |
| `${CONFIG_HOME}` | Cosca config files (`${COSCA_HOME}/`) | Global |
| `${BOOTSTRAP_HOME}` | Bootstrap directory (`${COSCA_HOME}/bootstrap`) | Global |
| `${SCAFFOLD_HOME}` | Project scaffold (`${COSCA_HOME}/.cosca-scaffold`) | Global |
| `${SESSIONS_HOME}` | Session storage (`${MEMORY_PROJECT}/session`) | Project |
| `${AUDIT_HOME}` | Audit trail (`.cosca/audit/`) | Project |

### Resolution Strategy
See [path-strategy.md](path-strategy.md) for OS-specific resolution rules and fallback chain.

### Environment Discovery
See [environment.md](environment.md) for how the resolver detects OS, user home, and config directories.

---

## API (Logical)

```
resolve("COSCA_HOME")       → /home/user/.config/opencode/cosca/
resolve("PROJECT_ROOT")    → /home/user/projects/my-app/
resolve("MEMORY_GLOBAL")   → ${COSCA_HOME}/memory/
resolve("MEMORY_PROJECT")  → ${PROJECT_ROOT}/.cosca/memory/
resolve("SKILLS_HOME")     → ${COSCA_HOME}/departments, ${COSCA_HOME}/engines, ...
resolve("ENGINES_HOME")    → ${COSCA_HOME}/engines
```

---

## RESOLUTION ALGORITHM

```
1. Check environment variable (e.g., COSCA_HOME env var)
   ↓ found? → use it
2. Check cosca.config.yaml override
   ↓ found? → use it
3. Auto-discover based on OS conventions
   ↓ found? → use it
4. Fallback: search upward from workspace for .cosca-marker
   ↓ found? → use it
5. Error: Cannot resolve COSCA_HOME. Provide diagnostic.
```

---

## PROCESS

### On Bootstrap Start
1. Resolve `${COSCA_HOME}` (where am I installed?)
2. Resolve `${PROJECT_ROOT}` (what am I managing?)
3. Validate all Virtual Path directories exist
4. Build path cache
5. Register with Memory Engine

### On Path Request
1. Check cache (fast path)
2. If cache miss: run resolution algorithm
3. Validate resolved path exists (optional, per caller)
4. Return resolved path or diagnostic error

### On Environment Change
1. Detect OS/user change
2. Invalidate cache
3. Re-resolve all paths
4. Notify dependent engines

---

## INPUTS

| Input | From | Format |
|-------|------|--------|
| Environment variables | OS | `COSCA_HOME`, `HOME`, `USERPROFILE` |
| cosca.config.yaml | ${COSCA_HOME} | YAML path overrides |
| Workspace path | OpenCode | Current directory |
| OS type | Node.js `process.platform` | linux, darwin, win32 |

## OUTPUTS

| Output | To | Format |
|--------|-----|--------|
| Resolved paths | All engines | String paths |
| Path cache | Memory Engine | JSON map |
| Validation report | Bootstrap | Gate 0 check |
| Diagnostic errors | Kernel → User | Structured message |

## DEPENDENCIES

| Component | Why |
|-----------|-----|
| Memory Engine | Cache resolved paths |
| Bootstrap Engine | Initialize before other engines |
| Kernel | Diagnostic error reporting |

## CONSTRAINTS
- Never use hardcoded paths (enforced by audit)
- Resolution must complete in < 100ms
- Cache hit rate must be > 95%
- Must work on Linux, macOS, Windows
- Must support environment variable overrides

## QUALITY CRITERIA
- [ ] All internal paths use Virtual Path notation
- [ ] Resolution works on all 3 OS families
- [ ] Fallback chain produces valid path or clear error
- [ ] Cache reduces resolution time by > 90%
- [ ] No physical paths in any Cosca .md file

## RELATED
- [resolver.md](resolver.md) — Detailed resolution rules
- [path-strategy.md](path-strategy.md) — OS-specific strategies
- [environment.md](environment.md) — Environment detection
- [validation.md](validation.md) — Path validation rules
- [../../bootstrap/BOOTSTRAP.md](../../bootstrap/BOOTSTRAP.md) — Bootstrap (consumer)
- [../../KERNEL.md](../../identidade/KERNEL.md) — Kernel (consumer)
- [../../MEMORY_MODEL.md](../../identidade/MEMORY_MODEL.md) — Memory taxonomy (path references)

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Resource Resolver | Initial implementation |
