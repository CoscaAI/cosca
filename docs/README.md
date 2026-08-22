# Cosca Documentation Index

> **Version**: 1.4.0-dev | **Last Updated**: 2026-07-29

Welcome to the Cosca documentation. This index provides quick navigation to all documentation resources.

---

## Quick Navigation

| I want to... | Start here |
|-------------|------------|
| Install Cosca | [CLI Overview](cli/overview.md) |
| Run my first `cosca install` | [CLI Examples](cli/examples.md) |
| Understand the architecture | [Architecture Overview](architecture/overview.md) |
| Understand the frontend | [Frontend Architecture](frontend/architecture.md) |
| Search my project knowledge | [Search Guide](knowledge/search.md) |
| Set up editor integration | [Editor Overview](editors/overview.md) |
| Write a plugin | [Plugin Development](plugins/development.md) |
| Use the Go SDK | [Go SDK Reference](sdk/go.md) |
| Use the TypeScript SDK | [TypeScript SDK Reference](sdk/typescript.md) |
| Set up authentication | [Auth Reference](api-reference/auth.md) |
| Understand middleware | [Middleware Reference](api-reference/middleware.md) |
| Contribute to Cosca | [Developer Getting Started](developer-guide/getting-started.md) |
| Troubleshoot issues | [Common Issues](troubleshooting/common-issues.md) |
| Resolver dúvidas e decidir com segurança | [Manual de Autoajuda](MANUAL_AUTOAJUDA_COSCA.md) |

---

## Documentation Map

### 📖 Architecture
| Document | Description |
|----------|-------------|
| [Architecture Overview](architecture/overview.md) | System architecture, layers, design principles |
| [Layer Architecture](architecture/layers.md) | Detailed layer descriptions and interfaces |

### ⚙️ CLI
| Document | Description |
|----------|-------------|
| [CLI Overview](cli/overview.md) | Installation, command structure, global flags |
| [Command Reference](cli/commands.md) | Complete command reference with examples |
| [CLI Examples](cli/examples.md) | Practical usage examples |

### 🏃 Runtime
| Document | Description |
|----------|-------------|
| [Runtime Overview](runtime/overview.md) | Runtime architecture, state machine, lifecycle |
| [Configuration Reference](runtime/configuration.md) | All config options, env vars, YAML format |

### 🧠 Knowledge Engine
| Document | Description |
|----------|-------------|
| [Knowledge Overview](knowledge/overview.md) | Architecture, indexing pipeline, search pipeline |
| [Search Guide](knowledge/search.md) | Hybrid search, query syntax, filters, examples |
| [Knowledge Graph](knowledge/graph.md) | Entity types, relationships, graph queries |

### 🔎 Discovery Engine
| Document | Description |
|----------|-------------|
| [Discovery Overview](discovery/overview.md) | What is discovered, how it works, extending |

### 💾 Memory Engine
| Document | Description |
|----------|-------------|
| [Memory Overview](memory/overview.md) | Memory layers, types, snapshot system, best practices |

### 🔌 Plugin System
| Document | Description |
|----------|-------------|
| [Plugin Overview](plugins/overview.md) | Architecture, lifecycle, manifest format |
| [Plugin Development](plugins/development.md) | Getting started, interface, hooks, examples |

### 🎮 Editor Integration
| Document | Description |
|----------|-------------|
| [Editor Overview](editors/overview.md) | Supported editors, setup, MCP integration |
| [OpenCode Integration](editors/opencode.md) | OpenCode-specific setup and features |
| [Claude Code Integration](editors/claude.md) | Claude Code-specific setup |
| [VS Code Integration](editors/vscode.md) | VS Code-specific setup |

### 📦 SDK
| Document | Description |
|----------|-------------|
| [Go SDK Reference](sdk/go.md) | Go SDK installation, client, API reference |
| [TypeScript SDK Reference](sdk/typescript.md) | TypeScript SDK installation, API reference |

### 📐 ADRs (Architecture Decision Records)
| Document | Description |
|----------|-------------|
| [ADR-001: CLI Architecture](adr/ADR-001-cosca-cli-architecture.md) | Layered architecture with Go core |
| [ADR-002: Knowledge Engine](adr/ADR-002-knowledge-engine.md) | Hybrid search design |
| [ADR-003: Plugin System](adr/ADR-003-plugin-system.md) | Multi-runtime plugin architecture |
| [ADR-004: Editor Adapters](adr/ADR-004-editor-adapters.md) | Editor integration design |
| [ADR-005: AI Orchestration](adr/ADR-005-ai-orchestration.md) | AI orchestration engine design |
| [ADR-006: AI Orchestration Implementation](adr/ADR-006-ai-orchestration-implementation.md) | Orchestration implementation details |
| [ADR-007: Frontend Architecture](adr/ADR-007-frontend-architecture.md) | Next.js 15 frontend architecture |

### 👨‍💻 Developer Guide
| Document | Description |
|----------|-------------|
| [Getting Started](developer-guide/getting-started.md) | Prerequisites, building, workflow |
| [Architecture Guide](developer-guide/architecture.md) | Code organization, packages, patterns |
| [Testing Guide](developer-guide/testing.md) | Unit, integration, E2E, benchmarks |
| [Agent Skills Standard](developer-guide/agent-skills.md) | Anthropic Agent Skills support, validation, migration |

### 🔧 Troubleshooting
| Document | Description |
|----------|-------------|
| [Common Issues](troubleshooting/common-issues.md) | Installation, editor, knowledge engine issues |

### 📡 API Reference
| Document | Description |
|----------|-------------|
| [API Overview](api-reference/overview.md) | gRPC, REST, MCP, SDK APIs |
| [Authentication](api-reference/auth.md) | JWT, API Keys, RBAC, cookie-based auth |
| [Middleware](api-reference/middleware.md) | CORS, CSRF, Rate Limiting, Security Headers |

### 🎯 Orchestration
| Document | Description |
|----------|-------------|
| [Orchestration Overview](orchestration/README.md) | AI orchestration engine, pipelines, execution |

### 🏗️ Platform
| Document | Description |
|----------|-------------|
| [Platform Overview](platform/COSCA-CLI-PLATFORM-OVERVIEW.md) | Cosca CLI platform architecture (Portuguese) |

### 🗺️ Roadmap
| Document | Description |
|----------|-------------|
| [Strategic Roadmap](roadmap/README.md) | Vision, milestones, and implementation phases |
| [Phase 1 MVP Backlog](roadmap/phase-1-mvp-backlog.md) | Web Console backlog |
| [Phase 4 Implementation](roadmap/phase-4-implementation-workflow.md) | Phase 4 workflow and progress |
| [State Audit 2026-07-25](roadmap/state-audit-2026-07-25.md) | Post-Phase 0-3 audit |
| [Pre-Flight Audit](roadmap/audit-pre-flight-2026-07-25.md) | Pre-flight audit report |
| [Accessibility Audit](roadmap/accessibility-audit.md) | WCAG 2.1 AA audit |
| [OWASP Top 10 Review](roadmap/owasp-top10-review.md) | Security review |
| [Lighthouse Baseline](roadmap/lighthouse-baseline.md) | Performance baseline |

### 🖥️ Frontend
| Document | Description |
|----------|-------------|
| [Frontend Architecture](frontend/architecture.md) | Next.js 15, React 19, state management, components |

### 📁 Examples
| Document | Description |
|----------|-------------|
| [Project Init Example](../examples/init/example.md) | Example: project initialization |
| [Hello World Plugin](../examples/plugins/hello-world.md) | Example: plugin development |
| [MCP Server Setup](../examples/editors/mcp-server.md) | Example: MCP server configuration |
| [Custom Workflow](../examples/workflows/custom-workflow.md) | Example: custom workflow |

---

## File Structure

```
cosca/
├── README.md                   ← Main project README
├── docs/                       ← All documentation
│   ├── README.md               ← This index
│   ├── architecture/           ← Architecture docs
│   ├── cli/                    ← CLI reference
│   ├── runtime/                ← Runtime docs
│   ├── knowledge/              ← Knowledge Engine docs
│   ├── discovery/              ← Discovery Engine docs
│   ├── memory/                 ← Memory Engine docs
│   ├── plugins/                ← Plugin system docs
│   ├── editors/                ← Editor integration docs
│   ├── frontend/               ← Frontend architecture docs
│   ├── sdk/                    ← SDK references
│   ├── adr/                    ← Architecture Decision Records
│   ├── developer-guide/        ← Developer docs
│   ├── troubleshooting/        ← Troubleshooting guides
│   └── api-reference/          ← API references (auth, middleware, overview)
├── cmd/                        ← CLI entry point
├── internal/                   ← Internal packages
│   ├── cli/                    ← Command implementations
│   ├── runtime/                ← Runtime engine
│   ├── knowledge/              ← Knowledge Engine
│   ├── discovery/              ← Discovery Engine
│   ├── memory/                 ← Memory Engine
│   ├── plugins/                ← Plugin system
│   ├── editors/                ← Editor adapters
│   ├── search/                 ← Search engine
│   └── ...                     ← Other packages
├── pkg/                        ← Public packages
├── api/                        ← API definitions
├── sdk/                        ← SDK implementations
└── examples/                   ← Usage examples
```

---

## Conventions

- All commands are shown with `$` prefix for shell commands
- Code blocks use language-specific syntax highlighting
- Architecture diagrams use Mermaid or ASCII art
- Configuration examples show YAML format
- API references show Go and TypeScript examples

---

> **Maintained by**: Cosca Documentation Chief  
> **Questions?**: Open an issue on GitHub
