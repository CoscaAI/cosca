# Discovery Engine

> **Status**: active | **Owner**: Discovery Chief | **Last Updated**: 2026-07-23

## Overview

The Discovery Engine automatically detects and reports everything about your project environment: the project type, language, framework, database, build system, editor, AI providers, and system configuration. It enables the zero-configuration experience of Cosca.

```
┌──────────────────────────────────────────────────────────────────┐
│                      DISCOVERY REPORT                             │
│                                                                   │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────────┐  │
│  │   PROJECT   │  │  WORKSPACE   │  │      EDITOR            │  │
│  │             │  │              │  │                        │  │
│  │ • Language  │  │ • Git repo   │  │ • OpenCode             │  │
│  │ • Framework │  │ • Monorepo?  │  │ • Claude Code          │  │
│  │ • Database  │  │ • Remote     │  │ • Codex                │  │
│  │ • Build sys │  │ • Branch     │  │ • Cursor               │  │
│  │ • Test frmk │  │ • Sub-prjs   │  │ • VS Code              │  │
│  └─────────────┘  └──────────────┘  │ • Neovim               │  │
│                                      │ • Windsurf             │  │
│  ┌─────────────┐  ┌──────────────┐  │ • Zed                  │  │
│  │    Cosca      │  │  PROVIDERS   │  └────────────────────────┘  │
│  │   GLOBAL    │  │              │                               │
│  │             │  │ • OpenAI     │  ┌────────────────────────┐  │
│  │ • Config    │  │ • Anthropic  │  │      RUNTIME           │  │
│  │ • Plugins   │  │ • Google     │  │                        │  │
│  │ • Skills    │  │ • Ollama     │  │ • Mode (cli/daemon)    │  │
│  │ • Templates │  │ • Azure      │  │ • Log level            │  │
│  └─────────────┘  │ • More...    │  │ • Cache/data dirs      │  │
│                    └──────────────┘  └────────────────────────┘  │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │                     ENVIRONMENT                               │ │
│  │  OS | Shell | Terminal | CPU | Memory | Container? | WSL?   │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

---

## What Is Discovered

### 1. Project Discovery

Detects the project's technical stack:

| Aspect | Detection Method | Examples |
|--------|-----------------|----------|
| **Language** | File extension analysis | Go, TypeScript, Python, Rust |
| **Framework** | Config file analysis | React, Next.js, Express, Gin |
| **Database** | Dependency/config check | PostgreSQL, MySQL, SQLite, MongoDB |
| **Build System** | Build file detection | Make, npm, cargo, go build |
| **Test Framework** | Config/dep detection | Jest, pytest, go test, Mocha |
| **CI/CD** | Config file detection | GitHub Actions, GitLab CI, Jenkins |
| **Architecture** | Pattern detection | Monolith, Microservices, Clean Arch |

### 2. Workspace Discovery

Detects workspace structure:

```
WorkspaceInfo:
  Root:       /home/user/projects/my-app
  IsGitRepo:  true
  GitRemote:  github.com/user/my-app.git
  GitBranch:  main
  GitCommit:  a1b2c3d4e5f6...
  IsMonorepo: false
  SubProjects: []
```

### 3. Editor Discovery

Detects which AI-assisted editor is being used:

| Editor | Priority | Detection |
|--------|----------|-----------|
| OpenCode | 1 | OpenCode config exists |
| Claude Code | 2 | Claude config exists |
| Codex | 3 | Codex config exists |
| Cursor | 4 | Cursor config exists |
| VS Code | 5 | VS Code settings exist |
| Neovim | 6 | Neovim plugin exists |
| Windsurf | 7 | Windsurf config exists |
| Zed | 8 | Zed settings exist |

### 4. AI Provider Discovery

Detects available AI providers by scanning environment variables:

| Provider | Environment Variable |
|----------|---------------------|
| OpenAI | `OPENAI_API_KEY` |
| Anthropic | `ANTHROPIC_API_KEY` |
| Google | `GOOGLE_API_KEY` |
| Azure | `AZURE_OPENAI_API_KEY` |
| Cohere | `COHERE_API_KEY` |
| Mistral | `MISTRAL_API_KEY` |
| Groq | `GROQ_API_KEY` |
| Together | `TOGETHER_API_KEY` |
| DeepSeek | `DEEPSEEK_API_KEY` |
| OpenRouter | `OPENROUTER_API_KEY` |

Also reads provider configuration from `cosca.config.yaml`.

### 5. Cosca Global Discovery

Detects the Cosca global installation at `~/.config/opencode/cosca`:

- Configuration files
- Installed plugins
- Skill definitions
- Template files

### 6. Runtime Discovery

Detects runtime configuration from environment:

- Mode (`COSCA_MODE`)
- Log level (`COSCA_LOG_LEVEL`)
- Config file (`COSCA_CONFIG_FILE`)
- Cache directory (`COSCA_CACHE_DIR`)
- Data directory (`COSCA_DATA_DIR`)

### 7. Environment Discovery

Detects the system environment:

- Operating system
- Shell (bash, zsh, fish)
- Terminal (xterm, alacritty, iterm2)
- CPU count and architecture
- Memory available
- Container/WSL detection

---

## How Discovery Works

### Architecture

```go
// Parallel discovery with 8 concurrent workers
func (e *Engine) DiscoverAll(ctx context.Context) (*DiscoveryReport, error) {
    // Launches 8 goroutines concurrently:
    // 1. DiscoverProject
    // 2. DiscoverWorkspace
    // 3. DiscoverEditor
    // 4. DiscoverAOSGlobal
    // 5. DiscoverProviders
    // 6. DiscoverPlugins
    // 7. DiscoverRuntime
    // 8. DiscoverEnvironment
    // All errors are non-fatal (partial discovery is OK)
}
```

### Caching

Discovery results are cached with a configurable TTL (default: 30s):

```go
// Cache invalidation
report, found := engine.GetCachedReport()
if found {
    return report, nil
}
// Perform fresh discovery
```

### Error Handling

All discovery operations are non-critical. If a discovery area fails, it is logged and the rest of the report is still returned. The final report may have some fields as `nil`.

---

## Extending Discovery

### Adding a New Provider Detection

```go
// In internal/discovery/discovery.go
func DetectProviders(ctx context.Context, logger zerolog.Logger) ([]ProviderInfo, error) {
    // Add new detection
    providerEnvVars := map[string]string{
        "newprovider": "NEWPROVIDER_API_KEY",
        // ...
    }
    // ...
}
```

### Adding a New Editor

```go
// In internal/editors/manager.go
func (m *Manager) registerBuiltin() {
    adapters := []types.Editor{
        opencode.NewAdapter(),
        claude.NewAdapter(),
        // ...
        myeditor.NewAdapter(),  // Add new adapter
    }
    // ...
}
```

### Adding a New Project Detector

```go
// In internal/discovery/project.go
func DetectProject(ctx context.Context, workDir string, logger zerolog.Logger) (*ProjectInfo, error) {
    // Add new language/framework detection
    // ...
}
```

---

## Troubleshooting

### Discovery fails silently

Discovery is designed to be resilient. Check verbose output:

```bash
cosca status --verbose
```

### Editor not detected

```bash
cosca editor detect        # Manual detection
cosca editor list          # List all supported editors
```

### Provider not found

```bash
# Check environment variables
echo $OPENAI_API_KEY     # Should be set
cosca provider list        # List detected providers
```

### Cache issues

```bash
# Force fresh discovery
cosca doctor               # Full diagnostics forces re-discovery
cosca status               # Shows current discovery state
```

---

## CLI Commands

```bash
# Show discovery results as part of status
cosca status

# Run discovery and diagnostics
cosca doctor

# Detect editor
cosca editor detect

# List all editors
cosca editor list

# List providers
cosca provider list
```

---

## Discovery Report Structure

```json
{
  "project": {
    "language": "go",
    "framework": "gin",
    "database": "postgresql",
    "build_system": "make",
    "test_framework": "go test"
  },
  "workspace": {
    "root": "/home/user/project",
    "is_git_repo": true,
    "git_branch": "main"
  },
  "editor": {
    "name": "opencode",
    "detected": true,
    "setup_complete": true
  },
  "providers": [
    { "name": "openai", "enabled": true }
  ],
  "runtime": {
    "mode": "cli",
    "log_level": "info"
  },
  "discovered_at": "2026-07-23T10:00:00Z",
  "duration": "150ms"
}
```

---

> **Related**: [Editor Overview](../editors/overview.md) | [Runtime Configuration](../runtime/configuration.md)
