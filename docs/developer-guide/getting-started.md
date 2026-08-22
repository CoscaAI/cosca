# Developer Guide: Getting Started

> **Status**: active | **Owner**: Documentation Chief | **Last Updated**: 2026-07-27

This guide covers how to set up your development environment, build Cosca from source, run tests, and contribute to the project.

---

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| **Go** | 1.22 or later | Primary development language |
| **Make** | 4.x or later | Build automation |
| **Git** | 2.x or later | Version control |
| **Node.js** | 22.x or later | Frontend development (Next.js) |
| **pnpm** | 9.x or later | Frontend package manager (required for `web/`) |
| **npm** | 9.x or later | TypeScript SDK package management (optional) |
| **tinygo** | 0.30+ | WASM plugin development (optional) |
| **golangci-lint** | 1.56+ | Code linting (optional) |

### Verify Installation

```bash
go version          # Expected: go version go1.22.x linux/amd64
make --version      # Expected: GNU Make 4.x
git --version       # Expected: git version 2.x
node --version      # Expected: v22.x (required for web frontend)
pnpm --version      # Expected: 9.x (required for web frontend)
```

### Install Go

Follow the [official Go installation guide](https://go.dev/doc/install) or use your package manager:

```bash
# Linux (apt)
sudo apt install golang-go

# macOS (Homebrew)
brew install go

# Windows (winget)
winget install GoLang.Go
```

---

## Repository Structure

```
cosca/
├── cmd/                    # CLI entry point (main package)
│   └── cosca/               # Main binary entry point
├── internal/               # Internal packages (not for external use)
│   ├── cli/               # CLI command implementations (Cobra)
│   ├── runtime/           # Runtime engine (state machine, lifecycle)
│   ├── knowledge/         # Knowledge Engine (FTS5, vector, graph)
│   ├── discovery/         # Discovery Engine
│   ├── memory/            # Memory Engine
│   ├── plugins/           # Plugin system
│   ├── editors/           # Editor adapters
│   ├── search/            # Hybrid search engine
│   ├── cache/             # Multi-level cache
│   ├── config/            # Configuration management
│   ├── providers/         # AI provider integrations
│   ├── agents/            # Agent management
│   ├── skills/            # Skill catalog
│   ├── workflows/         # Workflow orchestration
│   ├── auth/              # JWT auth, RBAC, user management
├── api/                    # API definitions
│   ├── rest/              # REST API (server, handlers, middleware, openapi.yaml)
│   ├── grpc/              # gRPC proto files and generated code
│   └── mcp/               # MCP protocol definitions
├── web/                    # Next.js 15 frontend
│   ├── src/
│   │   ├── app/           # App Router pages and layouts
│   │   ├── features/      # Feature-based domain modules
│   │   ├── components/    # Shared UI components
│   │   ├── lib/           # Utilities, API client
│   │   └── providers/     # Auth, Query, Theme providers
│   ├── package.json
│   ├── next.config.ts
│   └── vitest.config.ts
├── sdk/                    # SDK implementations
│   ├── go/                # Go SDK
│   └── typescript/        # TypeScript SDK
├── docs/                   # Documentation
├── examples/               # Usage examples
├── deploy/                 # Infrastructure-as-code
│   ├── helm/              # Kubernetes Helm charts
│   ├── terraform/         # AWS Terraform modules
│   └── prometheus.yml     # Monitoring configuration
├── test/                   # Integration and E2E tests
│   ├── integration/       # Integration tests
│   └── e2e/               # End-to-end tests
├── Dockerfile              # Go backend scratch image
├── web/Dockerfile          # Next.js Node Alpine image
├── docker-compose.yml      # Multi-service dev environment
├── Makefile                # Build automation
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
└── .golangci.yml           # Linter configuration
```

---

## Frontend Development

### Quick Start

```bash
cd web

# Install dependencies (requires pnpm)
pnpm install

# Start development server (port 3000)
pnpm dev

# Type check
pnpm typecheck

# Lint
pnpm lint

# Build for production
pnpm build
```

### Full-Stack Development

```bash
# Start the Go backend (port 14120)
go run ./cmd/cosca serve

# In another terminal, start Next.js (port 3000)
cd web && pnpm dev

# Or use Docker Compose for the full stack
docker-compose up
```

### Docker Development Workflow

```bash
# Build and start both services (Go API + Next.js)
docker-compose up --build

# Stop all services
docker-compose down

# Rebuild after code changes
docker-compose up --build
```

---

## Building from Source

### Quick Build

```bash
# Clone the repository
git clone https://github.com/CoscaAI/cosca.git
cd cosca

# Build the binary
make build

# The binary is placed in ./bin/cosca
./bin/cosca version
```

### Build Variants

```bash
make build                 # Standard build (debug symbols included)
make build-release         # Optimized release build (stripped)
make build-static          # Statically linked binary (Linux)
make build-darwin          # macOS build
make build-windows         # Windows cross-compilation
```

### Platform-Specific Builds

```bash
# Cross-compile for different platforms
GOOS=linux GOARCH=amd64 make build
GOOS=linux GOARCH=arm64 make build
GOOS=darwin GOARCH=amd64 make build
GOOS=darwin GOARCH=arm64 make build
GOOS=windows GOARCH=amd64 make build
```

---

## Development Workflow

### Hot Reload Mode

```bash
make dev
```

This runs the application in development mode with:
- File watcher that recompiles on changes
- Debug-level logging
- Profiling endpoints enabled
- No caching for faster iteration

### Code Structure Conventions

```
internal/<subsystem>/
├── engine.go              # Main engine interface and implementation
├── types.go               # Type definitions
├── config.go              # Configuration structs
├── errors.go              # Error definitions
├── <subsystem>_test.go    # Unit tests
├── mock.go                # Mock implementations for testing
└── subpackage/            # Internal subpackages
```

### Adding a New Command

1. Create the command file in `internal/cli/`:
   ```go
   package cli

   import "github.com/spf13/cobra"

   var myCommand = &cobra.Command{
       Use:   "mycommand [flags]",
       Short: "Description of my command",
       RunE:  runMyCommand,
   }

   func init() {
       RootCmd.AddCommand(myCommand)
   }

   func runMyCommand(cmd *cobra.Command, args []string) error {
       // Implementation
       return nil
   }
   ```

2. Register the command in `internal/cli/root.go`

3. Add tests in `internal/cli/cli_test.go`

### Code Quality

```bash
make lint           # Run golangci-lint
make vet            # Run go vet
make fmt            # Format code with gofmt
make check          # Run all static analysis
```

---

## Testing

### Running Tests

```bash
make test                 # Run all tests
make test-unit            # Run unit tests only
make test-integration     # Run integration tests
make test-e2e             # Run end-to-end tests
make test-bench           # Run benchmarks
make test-race            # Run tests with race detector
```

### Test Coverage

```bash
make coverage             # Generate coverage report
make coverage-html        # Generate HTML coverage report
make coverage-func        # Show per-function coverage
```

Coverage requirements: **80%+ line coverage** for all packages.

### Test Categories

| Category | Location | Runtime Dependencies |
|----------|----------|---------------------|
| Unit tests | `*_test.go` alongside source | None |
| Integration tests | `test/integration/` | SQLite (in-memory) |
| E2E tests | `test/e2e/` | Full Cosca runtime |
| Benchmarks | `*_bench_test.go` | Varies |

---

## Contributing Guidelines

### Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`

Examples:
```
feat(knowledge): add facet-based search filtering
fix(plugins): resolve WASM plugin initialization race condition
docs(adr): add ADR-005 for caching strategy
```

### Pull Request Process

1. Create a feature branch from `main`
2. Write tests for new functionality
3. Ensure all tests pass: `make test`
4. Run linters: `make lint`
5. Update documentation if changing behavior
6. Create a pull request with a clear description
7. Request review from the relevant chief

### Branch Naming

```
feature/<short-description>     # New features
fix/<short-description>         # Bug fixes
docs/<short-description>        # Documentation
refactor/<short-description>    # Refactoring
test/<short-description>        # Test additions
```

---

## Code of Conduct

### Our Standards

Examples of behavior that contributes to a positive environment:

- Using welcoming and inclusive language
- Being respectful of differing viewpoints and experiences
- Gracefully accepting constructive criticism
- Focusing on what is best for the community
- Showing empathy towards other community members

Examples of unacceptable behavior:

- The use of sexualized language or imagery
- Trolling, insulting/derogatory comments, and personal attacks
- Public or private harassment
- Publishing others' private information without explicit permission
- Other conduct which could reasonably be considered inappropriate

### Enforcement

Project maintainers are responsible for clarifying the standards of acceptable behavior and are expected to take appropriate and fair corrective action in response to any instances of unacceptable behavior.

Instances of abusive, harassing, or otherwise unacceptable behavior may be reported by opening an issue or contacting the project maintainers.

---

## Additional Resources

| Resource | Location |
|----------|----------|
| Architecture Guide | [developer-guide/architecture.md](architecture.md) |
| Testing Guide | [developer-guide/testing.md](testing.md) |
| Architecture Overview | [architecture/overview.md](../architecture/overview.md) |
| CLI Overview | [cli/overview.md](../cli/overview.md) |
| API Reference | [api-reference/overview.md](../api-reference/overview.md) |

---

**Related**: [Architecture Guide](architecture.md) | [Testing Guide](testing.md) | [ADR-001](../adr/ADR-001-cosca-cli-architecture.md)
