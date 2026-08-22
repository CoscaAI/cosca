# Cosca — AI Orchestration System

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Last Updated**: 2026-07-10

## What is Cosca?

Cosca is an enterprise-grade AI agent orchestration framework. It functions as a **virtual software company** — with a CEO, CTO, department chiefs, and specialists — that plans, builds, reviews, tests, and documents software automatically.

**You describe what you want. The framework figures out how and executes.**

---

## Quick Start

### Load the Kernel
```
Carregue o Cosca Kernel
```

The Kernel will:
1. Discover your project's tech stack, framework, language, database
2. Load project memory and context
3. Report status and await your commands

### Initialize a new project
```
/init
```
or
```
Crie um projeto CRM com React, Node e PostgreSQL
```

### Build a feature
```
/feature
```
or
```
Preciso de um dashboard com gráficos de vendas e filtro por período
```

---

## Commands

| Command | Description |
|---------|-------------|
| `/help-cosca` | Show this guide |
| `/init` | Initialize a new Cosca-managed project |
| `/feature` | Start the Wizard Engine for new feature specification |
| `/fix` | Start a bug fix workflow |
| `/refactor` | Start a refactoring workflow |
| `/review` | Start a code review workflow |
| `/deploy` | Start deployment pipeline |
| `/plan` | Generate an executive plan for a task |
| `/docs` | Update all project documentation |
| `/status` | Show project status, quality metrics, active workflows |
| `/evolve` | Analyze codebase for technical debt and improvements |

---

## How It Works

Every request follows this chain automatically:

```
User
  → Kernel (discovers context, loads memory, routes work)
    → CEO (strategic decisions)
      → Product Chief (requirements, scope, wizard)
        → CTO (technical planning)
          → Architecture Chief (system design, ADRs)
            → Department Chiefs (backend, frontend, database...)
              → Specialists (implement code, write tests, docs...)
                → Review Chief (code + architecture + security review)
                  → QA Chief (quality validation)
                    → Documentation Chief (update docs)
                      → Delivery to User
```

**You never choose which agent to use. The Kernel does it automatically.**

---

## Architecture

### Organization (41 Departments)
```
CEO
├── Product Chief
│   └── UI/UX Chief
└── CTO
    ├── Architecture Chief
    ├── Backend Chief
    ├── Frontend Chief
    ├── Database Chief
    ├── DevOps Chief
    ├── Security Chief
    ├── QA Chief
    │   └── Testing Chief
    ├── Review Chief
    ├── Documentation Chief
    ├── Infrastructure Chief
    ├── Runtime Chief
    ├── Workflow Chief
    ├── Analytics Chief
    ├── AI Chief
    ├── Integrations Chief
    ├── Monitoring Chief
    ├── Release Chief
    ├── Automation Chief
    ├── Memory Chief
    └── Context Chief
```

### Engines (64 Cross-Cutting Systems)
| Engine | Purpose |
|--------|---------|
| Context Engine | Discover and maintain project/session context |
| Memory Engine | Store and retrieve project knowledge across sessions |
| Workflow Engine | Define and orchestrate structured workflows |
| Planning Engine | Generate executive plans from requirements |
| Execution Engine | Run tasks with agents, handle failures, track progress |
| Review Engine | Multi-dimension code review (arch, quality, security, perf) |
| Quality Engine | Quality gates, metrics, automated checks |
| Documentation Engine | Auto-generate and update all docs |
| Wizard Engine | Structured new-feature intake questionnaire |
| Discovery Engine | Auto-detect framework, language, DB, architecture |
| Discovery Engine (F10.3) | Mine unplanned insights at sprint end ("o que descobrimos que não procurávamos?") |
| Evolution Engine | Detect technical debt, code smells, improvement opportunities |
| Template Engine | Project scaffolding for 9 application types |
| Runtime Engine | App lifecycle, middleware, health checks, error handling |
| Skills Engine | Skill registry, discovery, validation, routing |
| Tools Engine | Tool catalog with permission levels |
| Audit Engine | Complete audit trail of all actions and decisions |
| Observability Engine | Metrics, tracing, logging, alerting, dashboards |
| Learning Engine | Agent performance tracking, pattern recognition, self-improvement |

### Memory System (8 Types)
| Type | Scope | Purpose |
|------|-------|---------|
| Short Memory | Session | Active context, current decisions |
| Long Memory | Permanent | Cross-session knowledge |
| Project Memory | Project | Features, modules, releases |
| Architecture Memory | System | ADRs, patterns, contracts |
| Decision Memory | Permanent | All decisions and rationale |
| Pattern Memory | Permanent | Patterns that work / anti-patterns |
| Bug Memory | Permanent | Bug catalog with root causes and fixes |
| Agent Memory | Permanent | Agent performance and preferences |

---

## Workflows

### project-init
Initializes a new project: tech selection → architecture → scaffold → config → docs → git init.

### feature-development
Full lifecycle: requirements → wizard → plan → architecture → implementation → tests → review → QA → docs → merge.

### bug-fix
Bug fix pipeline: analysis → root cause → fix → regression tests → review → pattern documentation.

### refactoring
Safe refactoring: analysis → safety net (tests first) → incremental refactor → verify → quality comparison.

### code-review
Multi-dimensional review: architecture → code quality → security → performance → tests → documentation.

---

## Project Templates

| Template | Stack | Use For |
|----------|-------|---------|
| ERP | NestJS/Django + React/Angular + PostgreSQL | Enterprise resource planning |
| CRM | FastAPI/Express + React/Vue + PostgreSQL | Customer relationship management |
| SaaS | Next.js + NestJS + PostgreSQL + Stripe | Multi-tenant subscription apps |
| Marketplace | NestJS/Django + Next.js + Elasticsearch | Buyer/seller platforms |
| Mobile | React Native/Expo + FastAPI/NestJS | Mobile apps with backend |
| API | NestJS/FastAPI/Go/Rust | Pure backend API services |
| Microservices | Polyglot + RabbitMQ/Kafka + K8s | Distributed systems |
| Landing | Next.js/Astro + Tailwind | Marketing sites |
| Admin | React + Ant Design/MUI | Admin dashboards |

---

## Examples

### Example 1: Build a Login System
```
Usuário: Adicione login com email/senha e recuperação de senha

[Kernel → Product Chief → Wizard Engine]

Wizard: Qual o fluxo de cadastro? Confirmação por email?
Usuário: Sim, email de confirmação antes do primeiro login

Wizard: Quais papéis de usuário existem?
Usuário: Admin e Usuário

Wizard: Gerando Executive Plan...

[Plano apresentado e aprovado]

✅ Database: migration users, password_resets, email_verifications
✅ Backend: POST /auth/register, POST /auth/login, POST /auth/forgot-password, POST /auth/reset-password
✅ Backend: JWT access + refresh tokens
✅ Backend: Email service (SendGrid)
✅ Frontend: Login page, Register page, Forgot Password page, Reset Password page
✅ Frontend: Auth context, protected routes, token refresh
✅ Testing: 52 unit/integration tests, 4 E2E tests
✅ Review: APPROVED 9.1/10
✅ QA: All acceptance criteria passed
✅ Security: OWASP Top 10 check passed, rate limiting configured
✅ Documentation: README, ADR-0008, API docs updated
✅ ENTREGUE
```

### Example 2: Fix a Bug
```
Usuário: A listagem de produtos está quebrada quando filtro por categoria

Kernel → CTO → Review Chief → Root cause: query não escapa categoria com caracteres especiais
→ Architecture Chief → Fix: usar parameterized query
→ Backend Chief → Implementa correção
→ Testing Chief → 3 testes de regressão adicionados
→ Review Chief → APPROVED
→ Memory Chief → Bug padrão registrado: SQL injection via filtro não sanitizado
✅ CORRIGIDO
```

### Example 3: Refactor
```
Usuário: O módulo de pagamentos tem muito código duplicado, refatore

Kernel → CTO → Architecture Chief → Analisa 340 linhas, 12 métodos duplicados
→ Testing Chief → Adiciona 8 testes de cobertura antes da refatoração
→ Backend Chief → Extrai PaymentProcessor, PaymentValidator, PaymentGateway
→ QA Chief → Antes: complexidade 18, duplicação 23% → Depois: complexidade 6, duplicação 4%
→ Documentation Chief → ADR-0015 documentando padrão Strategy adotado
✅ REFATORADO
```

---

## Key Principles

1. **Zero manual agent selection** — The Kernel routes work automatically
2. **Chain of command** — Work flows through chiefs, never bypassed
3. **Redundancy** — Every critical task has primary + secondary + reviewer + validator
4. **Self-documenting** — Documentation updates automatically with every change
5. **Self-healing** — Failed agents trigger fallback automatically
6. **Memory-driven** — Project knowledge persists across sessions
7. **Quality gates** — Nothing is delivered without passing review + QA
8. **Enterprise patterns** — SOLID, DDD, Clean Architecture, ADR, event-driven

---

## Getting Help

- `/help-cosca` — Show this guide
- `/status` — Project health and progress
- Ask any question naturally — the Kernel routes it to the right department
