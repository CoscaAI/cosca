# cosca-cto — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Seed Knowledge

### 2026-07-27 — Baseline
| Field | Value |
|-------|-------|
| **Agent** | cosca-cto |
| **Task** | Initial capability establishment |
| **Technique** | Standard cto patterns — project conventions |
| **Level** | 1 |
| **Outcome** | success |
| **Tags** | #cto #baseline #initialization |
| **Related** | .opencode/cosca/memory/codebase/overview.md |
| **Learned** | Project established. Core cto patterns documented. Ready for Level 2 techniques. |
| **Next** | Level 2: Identify first advanced technique to master |

### 2026-07-28 — Activation Audit: Stack, Architecture, Tech Debt
| Field | Value |
|-------|-------|
| **Agent** | cosca-cto |
| **Task** | Full technical audit: Stack Review, Architecture Scan, Tech Debt Assessment |
| **Technique** | Multi-dimensional audit: dependency analysis × codebase traversal × risk scoring |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #cto #audit #activation #tech-debt #architecture #stack-review |
| **Related** | /home/henrique/documents/projects/CoscaAI/.opencode/cosca/memory/cto/activation-report.md |
| **Learned** | (1) Project has strong foundation — Go 1.25, pure-SQLite, WASM runtime, Next.js 15 web console. REST/gRPC/MCP triple API surface is a maintainability multiplier. (2) Plugin sandbox has known gaps on Linux (Setrlimit affects parent, no cgroups) and is a no-op on non-Linux — documented gap requiring cgroups v2. (3) gRPC server lacks auth interceptors (JWT/API key/CSRF) present in REST, creating asymmetric security. (4) Coverage threshold at 40% in Makefile vs 70% in CI docs indicates process gap. |
| **Next** | Drive P0 remediation: (1) Plugin sandbox cgroups v2, (2) gRPC auth interceptors, (3) API handler abstraction layer |

### 2026-09-05 — Google TESTE 2: Durable Execution & Kernel Routing (audit real)
| Field | Value |
|-------|-------|
| **Agent** | cosca-cto |
| **Task** | Provar enfileiramento compute fabric + stallwatch em tempo real num workflow multi-agente; medir durabilidade/memória. Auditou código e executou comandos reais (read-only + execução, sem commit). |
| **Technique** | Code-graph audit (run.go/pipeline.go/workflow.go/orchestration/compute/stallwatch) × live CLI runs (medido tempo + pico WS efêmero). |
| **Level** | 3 |
| **Outcome** | success (conclusão honesta) |
| **CRITICAL FINDING** | **O compute fabric NÃO está no caminho de delegação multi-agente.** `fabric.Submit` só existe em `internal/chat/ui/terminal/model.go` e `internal/chat/provider/register_chat.go` (TUI/terminal + bootstrap server) e `cosca fabric status`. O caminho durável (`cosca run`/`pipeline run`/`workflow run`) usa `pipeline.StepRunner`→`orchestration.Engine`→`Executor`, sem tocar o fabric. **Stallwatch**: no `Executor` é só `Collector` (grava eventos no loop de retry próprio); o `Watchdog.Watch` (backoff+fallback) roda APENAS em testes (`stallwatch_test.go`, hardening) — não é instanciado em produção. `cosca metrics` é por-processo (var package `engineHolder`) → não enxerga run de outro processo. `ChainExecutor.ExecuteDynamic` (Don→CEO→Specialists) exige `ChainConfig.Enabled=true` (default false) e não tem comando CLI exposto. |
| **Evidence** | `cosca run --metrics "design a db schema..."` → 35092ms, pico WS **62.5MB**, exit 0, routado **DATABASE CHIEF** (keyword hit:1), LLM 2587 tokens, 0 erros/0 fallbacks (0 stalls), MAG store=1. `cosca fabric` pós-run segue **done=0 queue=0** (fabric intocado). `cosca run` NÃO criou .jsonl em durable-events (só MAG/cost). `cosca flow --history ... --fail 2` → **Retries(whisper)=2, executed=3, 3356ms, 56.7MB**; replay (sem --fail) → **replayed=3, executed=0, 413ms, 56.2MB** (determinístico, sem perda). `cosca pipeline-test --json` **NÃO completou em 60s** (suspeita de lock SQLite do servidor cosca.exe PID 12708 rodando). 39 workflows/pipelines presentes. |
| **Matter-of-fact** | Durabilidade REAL no path workflow/pipeline (DurableStepRunner + append-only JSONL em `.cosca/durable-events/`, resume/replay); `flow` usa pacote `dflow` separado. "Enfileiramento via fabric" só é observável no TUI `cosca chat` ou bootstrap, não no run durável. Caminho mínimo p/ provar fabric+stallwatch: dirigir `cosca chat`/TUI (fabric.Submit) e ligar stallwatch.Watch no executor (hoje só collector). |
| **Tags** | #cto #teste2 #durability #fabric #stallwatch #routing #honest-audit #google |
| **Related** | PROJECT_CONTEXT.md, internal/compute/fabric.go, internal/compute/pool.go, internal/stallwatch/stallwatch.go, internal/on/executor.go, internal/cli/{run.go,pipeline.go,workflow.go,durable_cli.go,flow.go,metrics.go}, internal/terminal.go, internal/chat/ui/terminal/model.go |
| **Next** | Se o Google exigir fabric real no run multi-agente, wiring `fabric.Submit` no StepRunner/Executor e `SetStallCollector`→`Watchdog.Watch` no `Executor.chatWithRetry`; hoje o Watchdog ativo é test-only. |
