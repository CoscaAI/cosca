# RUNTIME CONTRACT — Cosca Kernel ↔ Runtime Interface

> **Version**: 2.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Last Updated**: 2026-08-01
> **Decision**: ADR-7423 — versionamento de contratos por método

## PURPOSE
This document defines the formal interface between the Cosca Kernel and any Runtime execution environment. The Runtime is responsible for executing the behaviors defined by the Cosca Kernel. This contract ensures that any conforming Runtime (CLI, API server, Dashboard, IDE plugin, headless agent) can consume Cosca directives consistently.

## ARCHITECTURE

```
┌──────────────────────────────────────┐
│              Cosca KERNEL              │
│  (defines behavior, routes work,     │
│   enforces quality gates)            │
└──────────────┬───────────────────────┘
               │ RUNTIME CONTRACT
               │ (this document)
               ▼
┌──────────────────────────────────────┐
│             RUNTIME LAYER            │
│  ┌────────┐ ┌────────┐ ┌─────────┐  │
│  │  CLI   │ │  API   │ │Dashboard│  │
│  └────────┘ └────────┘ └─────────┘  │
│  ┌────────┐ ┌────────┐              │
│  │  SDK   │ │  IDE   │  ...         │
│  └────────┘ └────────┘              │
└──────────────────────────────────────┘
```

## 1. KERNEL INTERFACE

### 1.1 Session Lifecycle

| Operation | Direction | Description |
|-----------|-----------|-------------|
| `session.init` | Runtime → Kernel | Initialize a new Cosca session |
| `session.discover` | Kernel → Runtime | Request workspace discovery |
| `session.context` | Kernel → Runtime | Request context loading |
| `session.route` | Kernel → Runtime | Route a user request |
| `session.execute` | Kernel → Runtime | Execute a workflow step |
| `session.teardown` | Runtime → Kernel | End session, persist memory |

### 1.2 Request/Response Protocol

```
Request:  { type, payload, metadata }
Response: { type, payload, status, errors }
```

### 1.3 Supported Request Types

| Type | Payload | Response |
|------|---------|----------|
| `feature` | Feature description, constraints | Executive Plan + Implementation |
| `bug` | Bug description, steps, expected behavior | Fix + Regression tests + Pattern |
| `refactor` | Target, reason, scope | Refactored code + Quality comparison |
| `review` | Target, review_type | Review report + Approval |
| `deploy` | Environment, version, strategy | Deployment status + Verification |
| `docs` | Target docs, scope | Updated documentation |
| `status` | — | Project health + Quality metrics |
| `evolve` | — | Evolution report + Recommendations |

## 2. RUNTIME RESPONSIBILITIES

### 2.1 The Runtime MUST:

1. **Load Cosca Kernel** — Parse and execute KERNEL.md directives
2. **Invoke engines** — Load engine skills when triggered by Kernel
3. **Spawn agents** — Create subagent sessions for chiefs and specialists
4. **Enforce quality gates** — Apply QUALITY_GATES.md checks
5. **Manage memory** — Read/write to memory stores per MEMORY_MODEL.md
6. **Track state** — Maintain workflow state, task status, session context
7. **Report errors** — Escalate failures per KERNEL.md error handling table
8. **Provide tools** — Expose file I/O, git, shell, search via Tools Engine

### 2.2 The Runtime MAY:

1. **Parallelize** — Execute independent tasks concurrently
2. **Cache** — Cache context, memory queries, discovery results
3. **Optimize** — Choose optimal AI model per task type
4. **Extend** — Add custom tools beyond Tools Engine catalog
5. **Persist** — Store session snapshots for resume capability

### 2.3 The Runtime MUST NOT:

1. **Bypass chain of command** — Never route work directly to specialists
2. **Skip quality gates** — Never deliver without passing Gate 2+
3. **Modify skills** — Never alter SKILL.md or workflow definitions
4. **Override decisions** — Never override CEO/CTO/Chief decisions
5. **Hardcode paths** — Always use Virtual Paths from Resource Resolver

## 3. AGENT SPAWNING INTERFACE

### 3.1 Spawn Request

```json
{
  "type": "spawn_agent",
  "agent_type": "chief | specialist | engine",
  "department": "backend | frontend | security | ...",
  "task": "Task description with context",
  "context": { "workflow_id": "...", "step_id": "..." },
  "tools": ["read_file", "write_file", "execute_command"],
  "timeout_ms": 300000,
  "retry": { "max": 3, "backoff_ms": 5000 }
}
```

### 3.2 Spawn Response

```json
{
  "agent_id": "uuid",
  "status": "running | completed | failed",
  "output": {},
  "review": { "score": 8.5, "issues": [] },
  "duration_ms": 45000,
  "attempts": 1,
  "errors": []
}
```

## 4. TOOL INTERFACE

### 4.1 Tools Available to All Agents

| Category | Tools |
|----------|-------|
| File I/O | read_file, write_file, edit_file, list_directory, find_files, search_content |
| Code | execute_command, run_tests, run_linter, run_typecheck, run_build |
| Git | git_status, git_diff, git_log, git_branch, git_checkout, git_commit |
| Memory | store_memory, retrieve_memory, search_memory |
| Docs | generate_readme, generate_api_docs, generate_adr, update_changelog |
| Quality | run_security_scan, check_test_coverage, run_complexity_analysis |
| Workflow | get_workflow_status, create_workflow, execute_workflow |

### 4.2 Tool Permissions

| Level | Agents | Tools |
|-------|--------|-------|
| Read | All | read_file, list_directory, find_files, search_content, git_status, git_diff, git_log |
| Write | Chiefs | write_file, edit_file, git_commit, store_memory |
| Execute | Chiefs | execute_command, run_tests, run_linter, run_build |
| Admin | Kernel, CEO, CTO | spawn_agent, kill_agent, install_dependencies |

## 5. EVENT BUS

### 5.1 Events Emitted by Kernel

| Event | Payload | Consumers |
|-------|---------|-----------|
| `session.started` | session_id, timestamp | Observability, Audit |
| `workflow.created` | workflow_id, type | Workflow Engine, Audit |
| `task.assigned` | task_id, agent, department | Execution Engine, Audit |
| `task.completed` | task_id, output, duration | Review Engine, Audit |
| `task.failed` | task_id, error, attempts | Execution Engine, Audit |
| `review.completed` | review_id, score, issues | Quality Engine, Audit |
| `qa.completed` | qa_id, passed, issues | Release Chief, Audit |
| `decision.made` | decision_id, type, rationale | Memory Engine, Audit |
| `session.ended` | session_id, summary, learnings | Memory Engine, Learning Engine |

### 5.2 Events Consumed by Runtime

| Event | Action |
|-------|--------|
| `session.started` | Initialize observability, load context |
| `task.assigned` | Spawn agent, monitor progress |
| `task.completed` | Store output, trigger next step |
| `task.failed` | Retry or escalate per error table |
| `decision.made` | Store in memory |
| `session.ended` | Persist memory, generate session report |

## 6. QUALITY GATE INTEGRATION

| Gate | When | Runtime Action |
|------|------|---------------|
| Gate 0 | Before any work | Validate request type, scope, departments |
| Gate 1 | After plan generation | Validate architecture, security, dependencies |
| Gate 2 | After implementation | Run automated checks + review + QA |
| Gate 3 | Before release | Full test suite + security scan + docs check |
| Gate 4 | After release | Health checks + error monitoring + user feedback |

## 7. ERROR HANDLING CONTRACT

| Error Type | Retry | Escalate To | Runtime Action |
|------------|-------|-------------|----------------|
| Agent timeout | 3x, exponential backoff | Secondary agent | Respawn with different agent |
| Agent failure | 1x | Department Chief | Log error, escalate |
| Validation failure | 0 | Chief | Return to agent with feedback |
| Dependency failure | 3x | CTO | Block dependent tasks |
| All paths exhausted | — | User | Notify with diagnosis |

## 8. MEMORY INTERFACE

| Operation | Runtime Implementation |
|-----------|----------------------|
| Store | Write markdown file with YAML frontmatter per MEMORY_MODEL.md |
| Retrieve | Read by key or query by tags |
| Search | Full-text search across memory stores |
| Index | Rebuild search metadata |
| Prune | Archive records older than retention period |

## 9. VERSIONED CONTRACTS (v2.0.0)

> Decisão: [ADR-7423](knowledge/architecture/adr/adr-7423-versioned-contracts.md). Padrão
> adaptado do framework `versioned-rpc` do Traycer (open-source, MIT) — ver
> `.cosca/memory/project/traycer-analysis.md`.

Todo método RPC do contrato declara uma versão `{ major, minor }` própria, com schemas
de request/response. As invariantes abaixo são verificadas **em tempo de carga do
registry** (CI obrigatória, `make contract-validate`), falhando o build se violadas.

### 9.1 Regra de ouro — minor é somente aditivo

Entre minors do mesmo major, mudanças de schema devem ser **apenas aditivas**:
campos novos obrigatoriamente opcionais; nada removido, renomeado ou alterado em tipo.
Violação → build falha com a mensagem exata do campo.

### 9.2 Major é obrigatoriamente breaking

Um bump de major sem mudança real de schema (request E response) falha com
"could have shipped as a minor". Proíbe major de mentira e força disciplina.

### 9.3 Downgrade explícito + floor methods

- Cada major declara paths de downgrade a partir do seu latest minor.
- Métodos fora do floor declararam `degrade`: `unsupported` ou `fallback`
  (adapta request/response para um método floor).
- Garante que cliente novo ↔ host antigo conversam sem derrubar a conexão.

### 9.4 Negociação de manifesto

O handshake troca manifesto de capacidades (`method → {major, minor}`) com mirror
check em ambos os lados. Incompatibilidade → erro tipado com guidance de upgrade
(client-missing-method / host-missing-method / no-bridge).

### 9.5 Registry central

`internal/contracts/` é a única fonte de verdade dos contratos, carregada no boot
de todo runtime (CLI, API, dashboard, plugins). Nenhum método fora do registry.

## 10. COMPATIBILITY

| Version | Runtime Requirement | Breaking Changes |
|---------|-------------------|-----------------|
| 1.0.0 | Floor inicial — todos os métodos migram como major 1, baseline de compatibilidade | — |
| 2.0.0 | Versionamento por método (seção 9) ativo; métodos v1 formam o floor | Sem mudança de contrato para runtimes v1 (compatibilidade preservada via floor) |

## RELATED
- [KERNEL.md](KERNEL.md) — Kernel orchestration
- [MEMORY_MODEL.md](MEMORY_MODEL.md) — Memory taxonomy
- [QUALITY_GATES.md](QUALITY_GATES.md) — Quality gate definitions
- [engines/resource-resolver/SKILL.md](engines/resource-resolver/SKILL.md) — Virtual Path resolution
- [ADR-7423](knowledge/architecture/adr/adr-7423-versioned-contracts.md) — Decisão de versionamento
- [BRIDGE_ARCHITECTURE.md](knowledge/architecture/BRIDGE_ARCHITECTURE.md) — Referência: ponte Host↔OpenCode (padrão Vercel AI SDK) para futuro bridge Cosca

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Initial runtime contract — formalized Kernel↔Runtime interface |
| 2.0.0 | 2026-08-01 | Cosca Kernel | Versionamento por método (ADR-7423) — minor aditivo, major breaking, floor methods, manifesto |
