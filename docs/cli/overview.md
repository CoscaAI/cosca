# CLI Overview

> **Status**: active | **Owner**: CLI Chief | **Last Updated**: 2026-07-23

## Overview

Cosca provides a comprehensive command-line interface for the Cosca Enterprise Platform. It is built with the Cobra CLI framework and supports multiple output formats, shell completion, and global flags.

---

## Installation Methods

### Go (recommended)
```bash
go install github.com/CoscaAI/cosca/cmd/cosca@latest
```

### npm
```bash
npm install -g cosca
```

### Build from source
```bash
git clone https://github.com/CoscaAI/cosca.git
cd cosca
make build
sudo cp bin/cosca /usr/local/bin/
```

### Binary Release
Download from the [Releases](https://github.com/CoscaAI/cosca/releases) page.

---

## Command Structure

```
cosca [global flags] <command> [subcommand] [flags] [args]

Examples:
  cosca install
  cosca knowledge search "authentication"
  cosca plugin list --json
  cosca runtime start --daemon
  cosca --verbose status
```

### Command Categories

| Category | Commands | Description |
|----------|----------|-------------|
| **Core** | `install`, `init`, `status`, `doctor`, `version`, `sync`, `update`, `upgrade`, `uninstall` | System management |
| **Knowledge** | `knowledge search`, `knowledge index`, `knowledge graph`, `knowledge stats`, `knowledge sync`, `knowledge rebuild`, `knowledge snapshot`, `knowledge verify` | Knowledge engine |
| **Memory** | `memory store`, `memory search`, `memory get`, `memory delete`, `memory promote`, `memory prune`, `memory stats`, `memory snapshot` | Memory engine |
| **Runtime** | `runtime start`, `runtime stop`, `runtime status`, `runtime restart` | Runtime daemon |
| **Plugins** | `plugin install`, `plugin list`, `plugin info`, `plugin remove`, `plugin enable`, `plugin disable`, `plugin update`, `plugin validate` | Plugin system |
| **Editors** | `editor detect`, `editor setup`, `editor validate`, `editor teardown`, `editor list` | Editor integration |
| **Config** | `config show`, `config init`, `config validate` | Configuration |
| **Cache** | `cache clear`, `cache stats` | Cache management |
| **Context** | `context show`, `context build` | Context management |
| **Providers** | `provider list`, `provider set` | Provider management |
| **Search** | `search` | Quick search alias |
| **Additional** | `agent`, `skill`, `prompt`, `template`, `workflow`, `benchmark`, `validate`, `bootstrap`, `completion`, `health`, `docs` | Specialized commands |

---

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | | `""` | Path to config file (default: `.cosca/config.yaml`) |
| `--verbose` | `-V` | `false` | Enable verbose debug output |
| `--quiet` | `-q` | `false` | Suppress non-essential output |
| `--json` | `-j` | `false` | Output in JSON format |
| `--format` | | `text` | Output format (text, json, yaml, table) |
| `--no-color` | | `false` | Disable colored output |
| `--version` | `-v` | | Show version information |

### Usage Examples

```bash
# Verbose output for debugging
cosca --verbose install

# JSON output for scripting
cosca --json status

# Quiet mode for automation
cosca --quiet sync

# Custom output format
cosca --format yaml plugin list

# Use a specific config file
cosca --config /path/to/custom-config.yaml status
```

---

## Output Formats

### Text (default)
```
$ cosca status
System Status: healthy
  Uptime: 5m 32s
  Version: 1.4.0-dev
  Components:
    knowledge: healthy
    discovery: healthy
    memory: healthy
    cache: healthy
```

### JSON
```bash
$ cosca status --json
{
  "state": "running",
  "health": "healthy",
  "uptime": "5m32s",
  "version": "1.4.0-dev",
  "components": {
    "knowledge": { "status": "healthy" },
    "discovery": { "status": "healthy" },
    "memory": { "status": "healthy" }
  }
}
```

### YAML
```bash
$ cosca --format yaml status
state: running
health: healthy
uptime: 5m32s
version: 1.4.0-dev
components:
  knowledge:
    status: healthy
  discovery:
    status: healthy
  memory:
    status: healthy
```

### Table
```bash
$ cosca --format table plugin list
PLUGIN      VERSION  STATUS
hello-cosca   1.0.0    started
my-plugin   2.1.0    stopped
```

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `COSCA_HOME` | Cosca home directory | `~/.config/cosca` |
| `COSCA_MODE` | Runtime mode (dev, staging, production) | `development` |
| `COSCA_LOG_LEVEL` | Log level | `info` |
| `COSCA_DEV` | Enable development mode | `false` |
| `COSCA_NO_COLOR` | Disable colored output | `false` |
| `COSCA_CONFIG_FILE` | Config file path | — |

---

## Shell Completion

Cosca supports shell completion for bash, zsh, fish, and PowerShell:

```bash
# Generate completion script
cosca completion bash    # Bash
cosca completion zsh     # Zsh
cosca completion fish    # Fish
cosca completion powershell  # PowerShell

# Install completion (bash example)
cosca completion bash | sudo tee /etc/bash_completion.d/cosca
source ~/.bashrc
```

### Output Example
```bash
$ cosca [TAB][TAB]
install     init        status      doctor      version
sync        update      upgrade     uninstall   knowledge
memory      runtime     plugin      editor      config
cache       context     provider    search      agent
skill       prompt      template    workflow    benchmark
validate    bootstrap   completion  health      docs
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error |
| `2` | Configuration error |
| `3` | Runtime error |
| `4` | Permission error |
| `5` | Timeout |

---

> **Related**: [Command Reference](commands.md) | [CLI Examples](examples.md)
