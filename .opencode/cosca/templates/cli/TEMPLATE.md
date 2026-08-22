# TEMPLATE: CLI Tool

> **Version**: 1.0.0 | **Status**: active | **Owner**: CLI Chief | **Last Updated**: 2026-07-23

## DOMAIN
Command-line interface tools for developer tooling, automation, platform self-service, and operational tasks.

## RECOMMENDED STACK
| Layer | Primary | Alternative |
|-------|---------|-------------|
| Language | TypeScript (Node.js) | Go, Python, Rust |
| CLI Framework | Commander / oclif | Cobra (Go), Click (Python), clap (Rust) |
| Shell Completions | oclif | Fig, custom generation |
| Testing | Vitest | Jest, pytest, Go test |
| Distribution | npm / brew | GoReleaser, PyPI, cargo |
| Documentation | TypeDoc | MkDocs, GoDoc |
| CI/CD | GitHub Actions | GitLab CI, CircleCI |

## MODULE STRUCTURE
```
cli-name/
├── src/
│   ├── commands/              # CLI command implementations
│   │   ├── init.ts
│   │   ├── deploy.ts
│   │   └── config.ts
│   ├── services/              # Business logic services
│   ├── api/                   # API client
│   ├── utils/                 # Utilities and helpers
│   └── types/                 # Type definitions
├── tests/
│   ├── unit/
│   └── integration/
├── docs/
│   ├── README.md
│   ├── COMMANDS.md            # Command reference
│   └── CONTRIBUTING.md
├── scripts/
│   ├── build.sh
│   └── release.sh
├── .github/
│   └── workflows/
│       ├── test.yml
│       └── release.yml
├── package.json
├── tsconfig.json
└── README.md
```

## KEY FEATURES
- Subcommand support with nested commands
- Shell completions (bash, zsh, fish)
- Configuration file support (JSON, YAML, TOML)
- JSON and table output formats
- Interactive prompts for required inputs
- Progress indicators for long operations
- Automatic update checking
- Error handling with user-friendly messages
- Telemetry (opt-in)
- Plugin system for extensibility

## ARCHITECTURE NOTES
- Single command entry point, subcommand dispatch
- Configuration loaded from XDG standard paths
- All external calls have timeout and retry
- Output formatted consistently (JSON, table, plain)
- Errors include actionable messages and exit codes
- Sensitive data never logged or displayed
- CLI follows POSIX conventions
- Support for CI mode (no interactive prompts)

## RELATED
- [CLI Chief](../../departments/cli/SKILL.md)
- [SDK Template](../sdk/TEMPLATE.md)
- [Platform Chief](../../departments/platform/SKILL.md)
- [Project Bootstrap skill](../../skills/platform/PROJECT_BOOTSTRAP.md)
