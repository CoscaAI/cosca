# ENVIRONMENT DETECTION

> **Version**: 1.0.0 | **Status**: active | **Owner**: Resource Resolver Engine | **Last Updated**: 2026-07-11

## PURPOSE
Detect the runtime environment (OS, user, paths, capabilities) to inform the Resource Resolver's path resolution strategy.

---

## DETECTION CHECKLIST

### 1. Operating System
| Method | Output |
|--------|--------|
| `process.platform` | `linux`, `darwin`, `win32` |
| `os.type()` | OS kernel name |
| `os.release()` | OS version |

### 2. User Identity
| Method | Output |
|--------|--------|
| `os.homedir()` | User home directory |
| `os.userInfo()` | Username, UID, GID, shell |
| `process.env.USER` / `USERNAME` | Current user |

### 3. Config Directories
| OS | Path |
|----|------|
| Linux | `$XDG_CONFIG_HOME` → `~/.config/` |
| macOS | `~/Library/Application Support/` |
| Windows | `%APPDATA%` |

### 4. Data Directories
| OS | Path |
|----|------|
| Linux | `$XDG_DATA_HOME` → `~/.local/share/` |
| macOS | `~/Library/Application Support/` |
| Windows | `%LOCALAPPDATA%` |

### 5. Cache Directories
| OS | Path |
|----|------|
| Linux | `$XDG_CACHE_HOME` → `~/.cache/` |
| macOS | `~/Library/Caches/` |
| Windows | `%TEMP%` |

### 6. Temp Directories
| All | `os.tmpdir()` |

---

## Cosca INSTALLATION DETECTION

### Detect if Cosca is installed
1. Look for `${COSCA_HOME}/KERNEL.md`
2. Look for `${COSCA_HOME}/engines/` directory
3. Look for `${COSCA_HOME}/departments/` directory
4. Check if `cosca-loader` plugin is in node_modules

### Detect Cosca version
1. Read `${COSCA_HOME}/CHANGELOG.md` for latest version
2. Read `${COSCA_HOME}/KERNEL.md` metadata block

---

## PROJECT DETECTION

### Detect project from workspace
1. Walk up from workspace looking for project markers
2. Read project name from package.json / pyproject.toml / etc.
3. Detect `.cosca/` directory (previously bootstrapped)
4. Check `state.yml` for last session data

---

## CAPABILITY DETECTION

| Capability | Detection Method |
|-----------|-----------------|
| Filesystem write | Try creating temp file in target dir |
| Git available | `which git` / `where git` |
| Node.js version | `process.version` |
| Package managers | `which npm`, `which pnpm`, `which yarn` |
| Docker available | `which docker` |

---

## OUTPUT

```yaml
environment:
  os: linux
  os_version: "6.8.0"
  user: henrique
  home: <user-home>
  config_dir: <user-config>/opencode
  aos_home: <user-config>/opencode/cosca
  aos_version: 1.0.0
  project:
    name: from workspace
    root: <workspace-path>
    has_aos: true | false
  capabilities:
    filesystem_write: true
    git: true
    node: "v22.0.0"
    pnpm: true
    docker: true
```

---

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-11 | Resource Resolver | Initial environment detection |
