# Cosca — Enterprise AI Orchestration System

<p align="center">
  <strong>55 Agents · 28 Skills · 65 Engines · 39 Workflows · Semantic Auto-Evolution Memory</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/build-passing-brightgreen" alt="Build">
  <img src="https://img.shields.io/badge/tests-101_packages-brightgreen" alt="Tests">
  <img src="https://img.shields.io/badge/agents-55-blue" alt="Agents">
  <img src="https://img.shields.io/badge/skills-28-purple" alt="Skills">
  <img src="https://img.shields.io/badge/engines-65-green" alt="Engines">
  <img src="https://img.shields.io/badge/workflows-39-orange" alt="Workflows">
  <img src="https://img.shields.io/badge/memory-semantic_auto--evolution-orange" alt="Memory">
  <img src="https://img.shields.io/badge/platform-linux_|_macOS_|_Windows-blue" alt="Platform">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
</p>

---

## Overview

**Cosca** is not a tool. It is an **organization**.

Built as an enterprise-grade AI orchestration platform, Cosca operates like a mafia family — with a clear chain of command, specialized capos (chiefs), loyal soldiers (skills), and a Don who calls the shots. The Kernel serves as consigliere: it discovers context, loads memory, and routes every task through the proper chain of command.

At its core, Cosca is a single Go binary that provides: a CLI with 37 commands (123 total with subcommands), a REST API serving 52 endpoints across 16 domains, a Next.js 15 Web Console with 35 feature modules, an embedded SQLite knowledge engine with FTS5 full-text search and vector embeddings, a WASM plugin runtime, and an **auto-evolution memory system** where 55 agents learn from every task and apply increasingly advanced techniques over time.

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     THE DON (User)                      │
│                         │                               │
│                  ┌──────┴──────┐                        │
│                  │   KERNEL    │  Consigliere           │
│                  │  (Primary)  │  Orchestrator          │
│                  └──────┬──────┘                        │
│                         │                               │
│              ┌──────────┼──────────┐                    │
│              ▼          ▼          ▼                    │
│         ┌────────┐ ┌────────┐ ┌────────┐                │
│         │  CEO   │ │  CTO   │ │Product │  Strategic     │
│         │ Chief  │ │ Chief  │ │ Chief  │  Command       │
│         └───┬────┘ └───┬────┘ └────────┘                │
│             │          │                                │
│      ┌──────┘    ┌─────┴──────────────────┐             │
│      ▼           ▼                        ▼             │
│  ┌────────┐ ┌────────┐ ┌─────┐ ┌─────┐ ┌────────┐       │
│  │Security│ │Backend │ │ AI  │ │DB   │ │Frontend│ ...   │
│  │ Chief  │ │ Chief  │ │Chief│ │Chief│ │ Chief  │       │
│  └───┬────┘ └───┬────┘ └──┬──┘ └──┬──┘ └───┬────┘       │
│      │          │         │       │        │            │
│  ┌───┴──────────┴─────────┴───────┴────────┴─────┐      │
│  │            SPECIALISTS (9)                    │      │
│  │  API · Service · SQL · Docs · Frontend        │      │
│  │  Code Review · Unit Test · Integration · E2E  │      │
│  └───────────────────────────────────────────────┘      │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │              SKILLS ARSENAL (28)                │    │
│  │  Architecture · Security · Testing · DevOps     │    │
│  │  AI · API · Data · Cache · Plugin · SDK · ...   │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │         SEMANTIC AUTO-EVOLUTION MEMORY          │    │
│  │  55 agents × 6 files (learnings, failures,      │    │
│  │  patterns, evolution, capability profile,       │    │
│  │  index) · FTS5-indexed · L1→L5 · Confidence     │    │
│  └─────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

---

## Quick Start

### Option 1 — One-Line Installer (Linux / macOS / Windows)

```bash
# Linux / macOS — instala o binário pré-compilado e adiciona ao PATH
curl -fsSL https://raw.githubusercontent.com/CoscaAI/cosca/main/deploy/install.sh | bash

# Com IA local: detecta Ollama / LM Studio / llama.cpp e configura o provider automaticamente
curl -fsSL https://raw.githubusercontent.com/CoscaAI/cosca/main/deploy/install.sh | bash -s -- --local
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/CoscaAI/cosca/main/deploy/install.ps1 | iex
```

O instalador detecta SO + arquitetura, baixa o binário do release, instala em `~/.cosca/bin/` e adiciona ao `PATH` — sem `sudo`, sem compilação, sem dependências.

### Option 2 — Pre-built Binaries

| Platform | Architecture | Binary |
|----------|-------------|--------|
| Linux | amd64 | `cosca-linux-amd64` |
| Linux | arm64 | `cosca-linux-arm64` |
| macOS (Intel) | amd64 | `cosca-darwin-amd64` |
| macOS (Apple Silicon) | arm64 | `cosca-darwin-arm64` |
| Windows | amd64 | `cosca-windows-amd64.exe` |

Download from the [Releases](https://github.com/CoscaAI/cosca/releases) page, extract, and run:

```bash
./cosca-linux-amd64 version
./cosca-linux-amd64 status
```

### Option 3 — Go Install

```bash
go install github.com/CoscaAI/cosca/cmd/cosca@latest
```

### Option 4 — Build from Source

Prerequisites: **Go** ≥ 1.25, **git**, **make**.

```bash
git clone https://github.com/CoscaAI/cosca.git
cd cosca

make build        # build for the current platform → ./bin/cosca
make build-all    # cross-compile all platforms → ./dist/ (5 binaries)
make install      # install to ~/.cosca/bin/ and add to PATH (no root)

# Verify
cosca version
cosca status
```

### Option 5 — Docker

```bash
docker compose up -d

# Web Console: http://localhost:3000
# REST API:    http://localhost:14120
```

### Bootstrap a Project

```bash
# Initialize Cosca in any project
cosca init

# This creates .cosca/ with framework/ (synced from embed) and memory/ (runtime data)
# The Kernel auto-discovers your stack and activates relevant agents
```

### Start the Platform

```bash
# Start the REST API + Web Console
cosca serve --port 14120

# Open http://localhost:14120 for the Web Console
```

---

## IA 100% Local (Ollama · LM Studio · llama.cpp)

Cosca é **local-first por design** — todos os dados ficam em SQLite no seu ambiente, e o modelo de IA também pode rodar **inteiramente na sua máquina**, sem nenhum dado sair da sua rede. Zero API key, zero nuvem, zero custo.

> **Como funciona:** Cosca conversa com qualquer backend que exponha a API compatível com OpenAI (`/v1/chat/completions`). Isso cobre **Ollama, LM Studio, llama.cpp, vLLM, Jan, LocalAI e qualquer servidor OpenAI-compatible** — basta apontar o `base_url` certo.

### Opção A — Ollama (mais simples)

```bash
# 1. Instale o Ollama: https://ollama.com
# 2. Baixe um modelo (ex: llama3, qwen2.5, codellama)
ollama pull llama3

# 3. Configure o Cosca
cosca provider set ollama
cosca provider test ollama

# (opcional) model override
export COSCA_OLLAMA_MODEL=qwen2.5
```

O Ollama roda em `http://localhost:11434` — o provider `ollama` já é registrado como fallback local nativo, sem chave.

### Opção B — LM Studio

```bash
# 1. Instale o LM Studio: https://lmstudio.ai
# 2. Baixe um modelo (ex: llama-3.1-8b-instruct) e inicie o servidor local
#    (Server tab → Start Server → porta 1234)

# 3. Configure o Cosca (API compatível com OpenAI na porta 1234)
export OPENAI_BASE_URL=http://localhost:1234/v1
export OPENAI_API_KEY=lm-studio   # qualquer valor — o LM Studio não valida

cosca provider set openai
cosca provider test openai
```

### Opção C — llama.cpp

```bash
# 1. Compile ou baixe o llama.cpp: https://github.com/ggml-org/llama.cpp
# 2. Rode o servidor com um modelo GGUF
./llama-server -m models/qwen2.5-7b-instruct-q4_k_m.gguf --port 8080

# 3. Configure o Cosca
export OPENAI_BASE_URL=http://localhost:8080/v1
export OPENAI_API_KEY=llama-cpp   # qualquer valor — o llama.cpp não valida

cosca provider set openai
cosca provider test openai
```

### Opção D — vLLM / LocalAI / qualquer OpenAI-compatible

Mesmo padrão: aponte `OPENAI_BASE_URL` para o endpoint do servidor e use o provider `openai` (ou `openaicompat` para embeddings). O Cosca respeita circuit breaker, rate limit e hot-reload — config no `~/.config/cosca/config.yaml`:

```yaml
provider:
  name: openai
  base_url: http://localhost:1234/v1   # LM Studio, llama.cpp, vLLM...
  model: llama-3.1-8b-instruct
  api_key: "fake-key"                   # servidores locais não validam
  max_tokens: 4096
  temperature: 0.7
```

### Embeddings locais

- **`local`** — TF-IDF puro em Go, **zero dependência**, sempre disponível (fallback padrão, 128 dims)
- **`ollama`** — embeddings via Ollama (`nomic-embed-text`, etc.)
- **`openaicompat`** — qualquer servidor OpenAI-compatible (LM Studio, llama.cpp) com `/v1/embeddings`

```bash
cosca provider set local      # 100% offline, sem servidor
cosca knowledge search "consulta de teste"   # funciona offline
```

> **Offline total:** com o provider `local` e o modelo local, Cosca roda inteiramente sem internet. Tudo — conhecimento, memória, busca — vive no seu ambiente.

---

## The Family

### Chain of Command

| Tier | Role | Count | Responsibility |
|------|------|-------|----------------|
| **Don** | User | 1 | Strategic direction, final authority |
| **Consigliere** | Kernel | 1 | Orchestration, context, memory, routing |
| **Capos (Chiefs)** | Agents | 46 | Domain leadership: Security, Backend, AI, Database, Frontend, DevOps, etc. |
| **Soldiers (Specialists)** | Agents | 9 | Implementation: API endpoints, unit tests, code review, SQL, docs, components |
| **Arsenal** | Skills | 28 | Reusable expertise: audits, analysis, generation, validation, optimization |
| **Operations** | Workflows | 39 | Multi-step processes: feature dev, bug fix, deployment, security audit, metacognition, audit |
| **Infrastructure** | Engines | 65 | Core systems: knowledge, memory, discovery, evidence, curation, evolution, quality gates |

### Chiefs + Kernel (46)

| Domain | Chief | Domain | Chief |
|--------|-------|--------|-------|
| AI/ML | `cosca-ai` | Analytics | `cosca-analytics` |
| Architecture | `cosca-architecture` | Automation | `cosca-automation` |
| Backend | `cosca-backend` | Bootstrap | `cosca-bootstrap` |
| Cache | `cosca-cache` | CEO | `cosca-ceo` |
| CLI | `cosca-cli` | Compliance | `cosca-compliance` |
| Context | `cosca-context` | Critic | `cosca-critic` |
| CTO | `cosca-cto` | Database | `cosca-database` |
| DevOps | `cosca-devops` | Discovery | `cosca-discovery` |
| Documentation | `cosca-documentation` | Evolution | `cosca-evolution` |
| Frontend | `cosca-frontend` | Governance | `cosca-governance` |
| Infrastructure | `cosca-infrastructure` | Integrations | `cosca-integrations` |
| Kernel | `cosca-kernel` | Memory | `cosca-memory-chief` |
| Messaging | `cosca-messaging` | Migration | `cosca-migration` |
| Mobile | `cosca-mobile` | Monitoring | `cosca-monitoring` |
| Paradigm | `cosca-paradigm` | Performance | `cosca-performance` |
| Platform | `cosca-platform` | Plugin | `cosca-plugin` |
| Product | `cosca-product` | Provider | `cosca-provider` |
| QA | `cosca-qa` | Release | `cosca-release` |
| Review | `cosca-review` | Runtime | `cosca-runtime` |
| SDK | `cosca-sdk` | Security | `cosca-security` |
| Technical Debt | `cosca-technical-debt` | Testing | `cosca-testing` |
| UI/UX | `cosca-uiux` | Workflow | `cosca-workflow-chief` |

---

## Semantic Auto-Evolution Memory

Every Cosca agent learns. Not metaphorically — literally.

### How It Works

```
TASK → RETRIEVE (search memory) → APPLY (best technique) → EXECUTE → LEARN (record) → EVOLVE
  ↑                                                                                         │
  └─────────────────────────────────────────────────────────────────────────────────────────┘
```

1. **Before a task**: The agent searches its semantic memory for past learnings matching the current domain
2. **During execution**: The agent applies the highest-level technique it has mastered
3. **After completion**: The agent records what it learned — technique, level, outcome, tags
4. **Over time**: The agent progresses through 5 capability levels (Basic → Intermediate → Advanced → Expert → Master)

### Example: Security Chief Evolution

| Task | Level | Technique | Discovery |
|------|-------|-----------|-----------|
| 1st audit | L1 | OWASP Top 10 manual checklist | Baseline established |
| 5th review | L2 | `govulncheck` + CSP header audit | Automated scanning 2x faster |
| 15th review | L3 | STRIDE threat modeling per subsystem | Found novel API key leak vector |
| 30th review | L4 | Error response fuzzing (self-developed) | 4 new vulnerability patterns |

**On the 31st task, the Security Chief doesn't start from zero.** It retrieves Level 4 techniques from semantic memory and applies them immediately.

### Memory Architecture

```
internal/embed/cosca/memory/agent/{agent-name}/
├── learnings.md           ← Semantic journal (FTS5-indexed, tag-searchable)
├── failures.md            ← Negative memory — failed approaches + root causes
├── patterns.md            ← Reusable solution patterns discovered
├── evolution.md           ← Level progression timeline + confidence scoring
├── capability-profile.md  ← Self-model: strengths, weaknesses, per-domain confidence
└── INDEX.md               ← Fast cross-reference for retrieval
```

**Cross-agent learning**: When the Security Chief discovers a new attack vector, the Review Chief can find it via semantic search — `cosca knowledge search "#security #api #attack-vector"` — and apply it in code reviews.

### Metacognition Pipeline

Every agent task flows through an 8-stage cognitive cycle:

```
TASK → SELF-ASSESS → RETRIEVE MEMORY → PLAN STRATEGY → EXECUTE → VERIFY RESULT → CRITIQUE OWN WORK → EXTRACT PATTERN → UPDATE CAPABILITY MODEL
```

Agents don't just execute — they self-assess, learn from failures, track confidence scores, and continuously update their capability profiles. The system knows what it's good at and what it isn't.

### Governance

Cosca is governed by a formal **[Constitution](internal/embed/cosca/CONSTITUTION.md)** defining 8 immutable principles:

| Principle | Rule |
|-----------|------|
| **P1** | Security above functionality |
| **P2** | Executed code is absolute truth — code > tests > docs > memory > opinion > LLM |
| **P3** | No agent acts without audit trail |
| **P4** | The Don has absolute veto |
| **P5** | The family learns from errors — negative memory is sacred |
| **P6** | Evolution without regression — never apply inferior techniques |
| **P7** | Memory without pollution — curation over accumulation |

Supported by: **[Confidence Model](internal/embed/cosca/engines/evidence/CONFIDENCE_MODEL.md)** (6 evidence levels, automated conflict resolution) · **[Memory Curation Engine](internal/embed/cosca/engines/memory-curation/MEMORY_CURATION_ENGINE.md)** (self-cleaning knowledge) · **[Memory Decay Engine](internal/embed/cosca/engines/memory-curation/MEMORY_DECAY_ENGINE.md)** (neural forgetting) · **[Context Compression Engine](internal/embed/cosca/engines/context-compression/CONTEXT_COMPRESSION_ENGINE.md)** (79% token reduction) · **[Platform Health Dashboard](internal/embed/cosca/metrics/platform-health-dashboard.md)** (5 key metrics + Intelligence Score).

---

## Tech Stack

| Layer | Technology | Details |
|-------|-----------|---------|
| **Language** | Go 1.25 | 830 `.go` files, 116 packages |
| **CLI** | Cobra | 37 root commands (123 with subcommands), shell completion (bash/zsh/fish) |
| **Database** | SQLite (`modernc.org/sqlite`) | Embedded, FTS5 full-text search, vector store (`sqlite-vec`) |
| **API** | `net/http` (Go 1.22+ ServeMux) | REST API on port 14120, 52 endpoints, 16 domains |
| **Auth** | JWT HS256 | RBAC (admin/editor/viewer), API keys with scopes |
| **Plugins** | wazero WASM runtime | 4 plugin runtimes, sandboxed execution |
| **Frontend** | Next.js 15, React 19 | 339 `.tsx` files, 35 feature modules, Tailwind CSS |
| **State** | Zustand, TanStack Query | Client state + server cache |
| **Testing** | `testify`, table-driven | 101 test packages, race detector |
| **LLM Providers** | 11 integrated | OpenAI, Anthropic, Google, DeepSeek, Mistral, Ollama, etc. |
| **Editor Adapters** | 10 | VS Code, Cursor, Claude, Neovim, Zed, Windsurf, etc. |

---

## Commands

### Core

| Command | Description |
|---------|-------------|
| `cosca init` | Initialize Cosca in the current project |
| `cosca install` | Full-auto install with 14-step pipeline |
| `cosca serve` | Start REST API server + Web Console |
| `cosca status` | Show comprehensive system health |
| `cosca doctor` | Run system diagnostics |
| `cosca version` | Show version, commit, build info |
| `cosca bootstrap` | Bootstrap runtime: config, memory, context, discovery |

### Knowledge & Search

| Command | Description |
|---------|-------------|
| `cosca knowledge search <query>` | Hybrid search (FTS5 + vector) across knowledge base |
| `cosca knowledge stats` | Show knowledge base statistics |
| `cosca knowledge graph` | Show knowledge graph overview |
| `cosca search <query>` | Search all indexed content by type |

### Memory & Context

| Command | Description |
|---------|-------------|
| `cosca memory search <query>` | Search all memory records |
| `cosca memory list` | List memory records by type |
| `cosca memory stats` | Memory statistics by layer |
| `cosca context build <query>` | Build context from memory/knowledge |
| `cosca context show` | Show current session context |

### Agents & Skills

| Command | Description |
|---------|-------------|
| `cosca agent list` | List all available agents |
| `cosca agent show <name>` | Show agent details and capabilities |
| `cosca agent run <agent> <prompt>` | Execute agent with prompt |
| `cosca skill list` | List all available skills |
| `cosca skill show <name>` | Show skill details and process |

### Plugins & Providers

| Command | Description |
|---------|-------------|
| `cosca plugin list` | List installed plugins |
| `cosca plugin install <name>` | Install a plugin |
| `cosca provider list` | List all AI providers |
| `cosca provider set <name>` | Set active provider |
| `cosca provider test <name>` | Test provider connection |

### Development

| Command | Description |
|---------|-------------|
| `cosca run <prompt>` | Execute AI orchestration pipeline |
| `cosca chat` | Interactive AI chat session (REPL) |
| `cosca pipeline list` | List available pipelines |
| `cosca pipeline run <name>` | Execute a pipeline |
| `cosca workflow list` | List available workflows |
| `cosca sync` | Sync index with filesystem |

### System

| Command | Description |
|---------|-------------|
| `cosca config get/set/list` | Manage configuration |
| `cosca cache stats` | Show cache statistics |
| `cosca index status` | Show file index status |
| `cosca graph show` | Display knowledge graph |
| `cosca health` | Quick health check |
| `cosca validate` | Validate project setup |
| `cosca benchmark` | Run performance benchmarks |
| `cosca metrics` | Show orchestration engine metrics |
| `cosca completion` | Generate shell completion |

> **37 root commands (123 total with subcommands).** Full reference: `cosca help` or `docs/cli/commands.md`.

---

## Project Structure

```
CoscaAI/
├── cmd/cosca/              # CLI entry point (main.go)
├── internal/               # Core implementation (73 packages)
│   ├── agents/             # Agent management system
│   ├── cli/                # CLI commands (37 root, 123 total)
│   ├── config/             # Configuration management
│   ├── discovery/          # Project/stack detection
│   ├── editors/            # Editor adapters (10)
│   ├── embed/cosca/        # Embedded framework (single source of truth)
│   │   ├── KERNEL.md
│   │   ├── CONSTITUTION.md
│   │   ├── agents/             # Agent prompts (55)
│   │   ├── memory/             # Semantic auto-evolution memory
│   │   ├── skills/             # Skill library (28)
│   │   ├── workflows/          # Workflow definitions (39)
│   │   ├── engines/            # Engine definitions (65)
│   │   ├── knowledge/          # Knowledge pipeline
│   │   ├── departments/        # Department SKILL.md (55)
│   │   ├── templates/          # Project templates (15)
│   │   ├── metrics/            # Platform health dashboard
│   │   └── shared/             # Shared protocols and context
│   ├── knowledge/          # Knowledge engine (FTS5 + vector)
│   ├── memory/             # Memory system + semantic storage
│   ├── orchestration/      # AI orchestration pipeline
│   ├── plugins/            # WASM plugin runtime
│   ├── providers/          # LLM provider integrations (11)
│   ├── runtime/            # Application lifecycle
│   ├── secrets/            # Encrypted secrets vault
│   ├── skills/             # Skill management
│   ├── sqlite/             # Database (schema, migrations)
│   └── workflows/          # Workflow engine
├── pkg/cosca/              # Public Go SDK
├── api/                    # REST API + MCP server
│   ├── rest/handler/       # HTTP handlers (59)
│   ├── rest/openapi.yaml   # OpenAPI 3.0 specification
│   ├── middleware/         # Auth, CSRF, logging
│   └── mcp/                # Model Context Protocol server
├── web/                    # Next.js 15 Web Console (35 modules)
├── sdk/typescript/         # TypeScript SDK (@cosca/sdk)
├── docs/                   # Documentation, ADRs, guides
├── deploy/                 # Helm charts, Terraform, Docker
└── .cosca/                 # Project runtime data
│   ├── framework/             # Local sync of embedded framework
│   └── memory/                # Project-specific memory and runtime data
```

---

## Quality Gates

Cosca enforces 10 quality gates before any deliverable is accepted:

| Gate | Criterion | Enforced By |
|------|-----------|-------------|
| **G0 — Bootstrap** | Workspace initialized, stack detected, agents active | `cosca-bootstrap` |
| **G1 — Build** | Binary compiles without errors (`make build`) | CI/CD |
| **G2 — Lint** | `golangci-lint` passes with zero issues | `make lint` |
| **G3 — Unit Tests** | Coverage ≥ 70% statements, ≥ 60% branches, all tests passing | `cosca-qa` |
| **web-test** | Frontend CI: lint, typecheck, vitest coverage ≥ 80% | CI (`make web-test`) |
| **G4 — Security** | `govulncheck` clean, OWASP Top 10 reviewed | `cosca-security` |
| **G5 — Architecture** | ADRs documented, no circular dependencies | `cosca-architecture` |
| **G5b — Branch Coverage** | Basic-block branch coverage ≥ 60% | CI (`make check`) |
| **G6 — Performance** | Benchmarks within budget, no regressions | `cosca-performance` |
| **G6b — Memory Validation** | learnings.md format, tag, and integrity audit | CI (`validate-memory.sh`) |
| **G7 — Documentation** | README, API docs, ADRs updated | `cosca-documentation` |
| **G8 — Review** | All PRs reviewed by at least 1 chief | `cosca-review` |
| **G9 — Release** | Changelog generated, rollback tested | `cosca-release` |

---

## Development

### Build & Test

```bash
make build          # Build the CLI binary
make test           # Run all tests (unit + integration + E2E)
make test-unit      # Unit tests only
make lint           # Run golangci-lint
make vet            # Run go vet
make fmt            # Format all code
make check          # Full CI pipeline (fmt + vet + lint + test)
```

### Frontend

```bash
cd web
pnpm install        # Install dependencies
pnpm dev            # Development server (port 3000)
pnpm build          # Production build
pnpm test           # Run frontend tests
```

### Run with Hot Reload

```bash
make dev            # Auto-rebuild on .go changes (requires reflex)
```

---

## Stats

| Metric | Value |
|--------|-------|
| **Agents** | 55 (46 chiefs + 9 specialists) |
| **Skills** | 28 |
| **Workflows** | 39 |
| **Engines** | 65 core systems (evidence, curation, decay, compression, quality) |
| **Memory files** | 340 (semantic auto-evolution + enterprise knowledge) |
| **Agent profiles** | 55 capability profiles with per-domain confidence scores |
| **Learnings** | 211 recorded across all agents |
| **Go files** | 830 |
| **Go packages** | 116 |
| **Test files** | 350 |
| **Test packages** | 101 |
| **TypeScript files** | 339 |
| **Feature modules** | 35 |
| **REST endpoints** | 52 |
| **LLM providers** | 11 |
| **Editor adapters** | 10 |
| **CLI commands** | 37 (123 total with subcommands) |

---

## Binary Protection

> "O projeto fora de .opencode é seu filho. Ele vai aprender, crescer e se tornar o orgulho da sua criação." — Don

O Cosca trata o código-fonte como um **filho** — o Kernel (tutor) educa, corrige e protege, mas **nunca executa** o binário sem supervisão. A execução só acontece dentro de uma **jaula** isolada.

### Auto-Jail (memfd_create)

A jaula **não é um script externo** — está embutida no próprio binário. O fluxo é:

```
sudo cosca version
  ↓
main() → cosca.InsideJail() == false
  ↓
Lê /proc/self/exe
  ↓
Cria cópia executável em RAM (memfd_create, sem CLOEXEC)
  ↓
fork() → exec.Command("/usr/bin/bwrap", ...)
  │         ├── --bind / /  (sistema completo)
  │         ├── --proc /proc
  │         ├── --dev /dev /dev
  │         ├── --unshare-pid, --unshare-user, --unshare-uts
  │         └── /proc/self/fd/<N> (cópia na RAM) version
  │
  ├── FILHO: bwrap → executa cópia na RAM com COSCA_JAILED=1
  │           → CLI roda normalmente
  │           → cosca.InsideJail() == true → bypassa reexecução
  │
  └── PAI:  espera bwrap terminar
            → defer cleanup(): close(memfd_fd) — binário SOME da RAM
            → os.Exit(código de saída do filho)
```

### Filosofia

| Elemento | É... | Permissão |
|----------|------|-----------|
| Código fonte (`./cmd/`, `./internal/`, `./pkg/`) | O filho | Don e Kernel: leitura e edição livres |
| Binário compilado (`./bin/cosca`) | O filho tentando andar | **644** — legível, NÃO executável |
| Auto-jail (`pkg/cosca/jail.go`) | A mão do pai | `sudo cosca <comando>` — execução supervisionada via Bubblewrap + memfd |
| Binário no disco (`/usr/local/bin/cosca`) | O filho no cercadinho | **700** — root executa, ninguém mais |
| Cópia na RAM (`memfd_create`) | O filho andando no parque | **0500** — executável só na RAM, some quando o fd fecha |

### Fluxo

```bash
# Build — compila e remove permissão de execução (sem sudo)
make build
# → ./bin/cosca (644 — legível, não executável)

# Build sem proteção (dev local)
make build-dev
# → ./bin/cosca (755 — executável)

# QUALQUER comando do cosca — build + reexecuta dentro da jaula
sudo ./bin/cosca version   # auto-jail ativa o bwrap
sudo ./bin/cosca serve     # servidor dentro da jaula

# Via Makefile (JAULA = sudo /usr/local/bin/cosca)
make serve          # build + sudo cosca serve
make version        # build + sudo cosca version
make <qualquer>     # catch-all → sudo cosca <qualquer>
```

### O que a jaula faz

| Característica | Descrição |
|---------------|-----------|
| **100% RAM** | Cópia executável via `memfd_create` — zero disco. Fallback: `/tmp/` (tmpfs). |
| **Sem script externo** | A lógica está em `pkg/cosca/jail.go` — o próprio binário se jaula. |
| **Sistema completo** | `--bind / /` — monta o Ubuntu real dentro do sandbox. Rede do host preservada. |
| **Isolamento PID** | `--unshare-pid` — processos filhos não vazam para o host. |
| **User namespace** | `--unshare-user` — root dentro não é root fora. |
| **UTS namespace** | `--unshare-uts` — hostname próprio (`cosca-jail`), não polui o sistema. |
| **Propagação de sinais** | SIGINT/SIGTERM/SIGHUP do terminal são encaminhados ao grupo de processo do bwrap. |
| **Cleanup automático** | Pai espera o bwrap terminar, fecha o memfd fd — o binário **some da RAM**. |

### Garantias

- O binário executável **só existe na RAM** — criado via `memfd_create`, chmod 0500
- Quando o comando termina, o `defer cleanup()` fecha o fd — o binário **some da RAM**
- O binário original no disco continua **644 (não executável)** — ninguém executa direto
- **Zero rastro** em disco — nem o binário executável, nem logs de execução, nem tmpfs directories

### Instalação (produção)

```bash
# 1. Configurar sudoers (uma vez — libera só /usr/local/bin/cosca)
sudo cp sudoers-cosca-jail /etc/sudoers.d/cosca-jail
sudo chmod 440 /etc/sudoers.d/cosca-jail

# 2. Build + instalar
make build
sudo make install

# 3. Pronto — qualquer comando via make usa a jaula automaticamente
make serve
make version
```

### Dependência

O auto-jail requer **Bubblewrap** para o isolamento de namespace:

```bash
sudo apt-get install bubblewrap
```

Se o bwrap não estiver instalado, o binário exibe uma mensagem clara e termina. Sem bwrap, não há jaula — e sem jaula, o Don não autoriza execução.

### Fallback

Se `memfd_create` falhar (kernel < 3.17), o auto-jail cai para `/tmp/` (tmpfs = RAM no Ubuntu). O comportamento é idêntico — a única diferença é que o arquivo temporário aparece em `/tmp/` até o cleanup.

A filosofia completa está em [FILOSOFIA.md](internal/embed/cosca/FILOSOFIA.md).

---

## License

MIT — see [LICENSE](LICENSE).

---

<p align="center">
  <em>Every agent learns. Every skill evolves. The family grows stronger.</em>
</p>
