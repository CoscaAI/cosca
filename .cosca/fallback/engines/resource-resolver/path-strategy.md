# PATH STRATEGY — OS-Specific Resolution

> **Version**: 1.0.0 | **Status**: active | **Owner**: Resource Resolver Engine | **Last Updated**: 2026-07-11

## PURPOSE
Define OS-specific directory conventions and fallback strategies for resolving Cosca paths on Linux, macOS, and Windows.

---

## LINUX

### Config Directory Priority
```
1. $XDG_CONFIG_HOME/opencode/        (typically ~/.config/opencode/)
2. ~/.config/opencode/                (XDG default)
3. ~/.opencode/                       (legacy fallback)
4. $COSCA_HOME                          (env var override)
```

### COSCA_HOME Resolution
```
$XDG_CONFIG_HOME/opencode/cosca/
  → ~/.config/opencode/cosca/
    → $COSCA_HOME (env var)
```

### Project Root Detection
```
1. Walk up from CWD:
   - package.json → root
   - .git/ → root
   - go.mod → root
   - Cargo.toml → root
   - pyproject.toml → root
   - Makefile → root
2. CWD if no marker found
```

---

## macOS

### Config Directory Priority
```
1. ~/Library/Application Support/opencode/
2. ~/.config/opencode/                (XDG-compatible)
3. ~/.opencode/                       (legacy)
4. $COSCA_HOME                          (env var)
```

### COSCA_HOME Resolution
```
~/Library/Application Support/opencode/cosca/
  → ~/.config/opencode/cosca/
    → $COSCA_HOME (env var)
```

### macOS-Specific Paths
- Config: `~/Library/Application Support/`
- Caches: `~/Library/Caches/`
- Logs: `~/Library/Logs/`

---

## WINDOWS

### Config Directory Priority
```
1. %APPDATA%/opencode/                (typically C:\Users\<user>\AppData\Roaming\opencode\)
2. %LOCALAPPDATA%/opencode/           (C:\Users\<user>\AppData\Local\opencode\)
3. %COSCA_HOME%                         (env var)
```

### COSCA_HOME Resolution
```
%APPDATA%/opencode/cosca/
  → %LOCALAPPDATA%/opencode/cosca/
    → %COSCA_HOME% (env var)
```

### Windows-Specific Conventions
- Use forward slashes in Virtual Paths: `${COSCA_HOME}/engines`
- Resolved paths use OS-native separators
- Drive letters supported: `C:/Users/...`

---

## CROSS-PLATFORM GUARANTEES

| Aspect | Guarantee |
|--------|-----------|
| Path separators | Virtual Paths always use `/` |
| Case sensitivity | Virtual Path names are case-insensitive on Windows |
| Home directory | Always resolved via `$HOME` / `%USERPROFILE%` / `os.homedir()` |
| Temp directory | OS-specific temp via `os.tmpdir()` |
| Config directory | OS-specific via resolution chain |
| Data directory | OS-specific via resolution chain |

---

## FALLBACK BEHAVIOR

If the primary config directory doesn't exist:
1. Create it (if write permission)
2. Use next fallback (if cannot create)
3. Use temp directory (last resort)

If COSCA_HOME doesn't exist:
1. Search for `KERNEL.md` in config directories
2. Search for `cosca/` directory in config directories
3. Error with diagnostic message listing searched paths

---

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Resource Resolver | Initial path strategy |
