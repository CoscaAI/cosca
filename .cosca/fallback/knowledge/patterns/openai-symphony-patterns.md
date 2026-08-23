# OpenAI Symphony Patterns — Work Manager, Runs Isoladas, Orquestrador

> **Version**: 1.0.0 | **Confidence**: 0.88 | **Category**: Enterprise Platform Patterns | **Created**: 2026-08-22 | **Source**: https://github.com/openai/symphony

> **Mined by**: cosca-kernel (ordem do Don). Extraído de `elixir/` (implementação de referência, OTP) e `SPEC.md`. Filosofia central: **"manage work, not supervise agents"** — gerenciar o *trabalho*, não inspecionar a conversa do agente.

## Purpose

Symphony resolve o problema de **transformar issues de projeto em "implementation runs" autônomas e isoladas**, com um orquestrador que gerencia *claims* de trabalho (não a conversa do agente). É o modelo anti-microgestão: o orquestrador só decide *o quê* roda e *quando*; o agente roda a sessão inteira.

---

## 1. WORKFLOW.md como contrato de política executável versionável
- **O que resolve**: parametrizar um agente autônomo sem espalhar config por env/flags/fora do repo. Toda a semântica de "como um issue é executado" vive no repo e é versionada junto.
- **Como funciona**: um único markdown com front matter YAML (config tipada) + corpo markdown (prompt template Liquid). O loader separa front matter do corpo. Um `WorkflowStore` (GenServer) faz polling do `mtime/size/hash` e, ao detectar mudança, re-parseia e re-aplica tudo. **Regra de ouro**: reload inválido NUNCA derruba o serviço — mantém a última config boa ("last known good") e loga para o operador.
- **Onde**: `elixir/lib/symphony_elixir/{workflow,workflow_store,config}.ex`.
- **Aplicação no Cosca**: um "job contract" por run (ex. `JOBS.md`) com front matter (timeouts, concurrency, sandbox) + corpo como prompt do agente, carregado por um store com hot-reload e "last-known-good". É o mecanismo mais direto para o agendador/orquestrador ser dirigido por política versionada no repo.

## 2. Orquestrador de autoridade única com estado interno de claim (não é o estado do tracker)
- **O que resolve**: evitar dispatch duplicado e manter uma única fonte de verdade mutável de agendamento, desacoplada do estado do tracker ("Todo/In Progress").
- **Como funciona**: um único `GenServer` (Orchestrator) é o `SoleMutator`: todo estado de scheduling (`running`, `claimed`, `retry_attempts`, `completed`, `blocked`) só é alterado por ele. Máquina de estados interna separada do tracker: `Unclaimed → Claimed → Running | RetryQueued → Released`. Antes de lançar worker, valida `claimed` e `running` (idempotência). "Sucesso" do worker ≠ issue pronto (pode ter terminado em `Human Review`). O tick executa `reconcile_running → reconcile_blocked → validate → fetch → sort → dispatch`.
- **Onde**: `elixir/lib/symphony_elixir/orchestrator.ex`.
- **Aplicação no Cosca**: o orquestrador deve ter um state machine de job separado do estado da tarefa externa, com um map `claimed` para idempotência entre múltiplos agentes do mesmo work. Tudo centralizado num orchestrator; workers reportam de volta. É o coração do "manage work": supervisa o *claim*, não o agente.

## 3. Runs isoladas por issue: workspace determinístico com invariantes de path-safety
- **O que resolve**: isolar execuções de agentes em paralelo sem que comandos vazem para fora de uma área controlada.
- **Como funciona**: cada issue mapeia para um workspace determinístico `<root>/<workspace_key>`. O `workspace_key` sanitiza o identifier para `[A-Za-z0-9._-]` e, se houve alteração, anexa um hash SHA256 de 64 bits (colisão-resistente). `PathSafety.canonicalize` resolve symlinks reais antes do comparativo. **Três invariantes** checados no launcher: (a) `cwd == workspace_path`; (b) `workspace_path` sob o `workspace_root` (rejeita symlink-escape); (c) todo workspace dentro do root. Workspaces reutilizados entre runs (preservados após sucesso), com hooks de lifecycle: `after_create` (fatal), `before_run` (fatal), `after_run` (log-ignore), `before_remove` (log-ignore).
- **Onde**: `elixir/lib/symphony_elixir/{workspace,path_safety}.ex`, `.../codex/app_server.ex`.
- **Aplicação no Cosca**: cada job id gera um workspace determinístico sob um root, com sanitização + hash, e o processo do agente tem `cwd` forçado e validado contra escape de symlink. É o padrão que garante que execuções paralelas de agentes não pisem umas nas outras nem toquem caminhos fora do root.

## 4. "Implementation run" autônoma: worker como sessão contínua com turn-loop + re-checagem do tracker
- **O que resolve**: materializa literalmente "trabalho de projeto vira runs isolados e autônomos" — o orquestrador gerencia o *trabalho*, não o *agente*. Ele não inspeciona a conversa; recebe apenas eventos upstream.
- **Como funciona**: `AgentRunner.run` cria/reusa o workspace, roda `before_run`, abre UMA sessão app-server persistente (`thread_id` fixo) e entra num loop de turns. Turn 1 recebe o prompt completo; turns seguintes apenas "continuation guidance" (não reenvia o original, já no histórico). Entre cada turn, re-busca o issue: se ainda ativo, continua na MESMA thread; se terminal ou atingiu `max_turns`, encerra e devolve o controle. O AppServer emite eventos estruturados (`session_started`, `turn_completed`, `approval_required`) via callback; nunca há comando/controle do agente vindo do orquestrador.
- **Onde**: `elixir/lib/symphony_elixir/agent_runner.ex`, `.../codex/app_server.ex`.
- **Aplicação no Cosca**: modelar cada job como uma **sessão de execução com loop de continuação**, re-checando a fonte de verdade externa entre passos, e retornando o controle ao orquestrador via eventos estruturados. O orquestrador só gerencia estado/claim/política — nunca supervisa a conversa.

## 5. Fila de retry de dois níveis sem banco: continuação (1s) vs falha (backoff exponencial)
- **O que resolve**: diferenciar "preciso continuar o trabalho" de "precisei recuperar de uma falha", com fila de retry em memória e recuperação de restart dirigida pelo tracker.
- **Como funciona**: `schedule_issue_retry` grava em `retry_attempts` (issue_id → `%{attempt, timer_ref, retry_token, due_at_ms, error, worker_host, workspace_path}`) e usa `Process.send_after`. Duas famílias: saída normal → `@continuation_retry_delay_ms = 1000`; falha/timeout/stall → `min(10_000 * 2^(attempt-1), max_retry_backoff_ms)`. Um token de retry (`make_ref`) garante que timers obsoletos não disparem re-dispatch. **Restart recovery**: o estado do scheduler é só em memória — recupera por startup-cleanup de workspaces terminados + re-poll + re-dispatch.
- **Onde**: `elixir/lib/symphony_elixir/orchestrator.ex`.
- **Aplicação no Cosca**: a fila/job deve ter **duas classes de retry** (continuação curta vs falha com backoff exponencial com teto), timers com token anti-stale, e requeue explícito por esgotamento de slots. Resiliência: scheduler em memória + recuperação dirigida pela fonte externa (re-fetch + re-dispatch + sweep de workspaces órfãos).

## 6. Concorrência limitada em três níveis (global / por-estado / por worker-host)
- **O que resolve**: controlar saturação do executor de forma gradável, sem estourar recursos e sem preempção arbitrária.
- **Como funciona**: no dispatch, `available_slots = max(max_concurrent_agents - map_size(running), 0)` (global). Por estado: `max_concurrent_agents_by_state[state]` sobrepõe o global. Se `worker.ssh_hosts` existe, `max_concurrent_agents_per_host` por host; `select_worker_host` escolhe host least-loaded, dá preferência ao host anterior do mesmo issue em retries (workspace locality) e retorna `:no_worker_capacity` em vez de cair em modo errado. A seleção ordena por `priority (1..4)` → `created_at` → `identifier`.
- **Onde**: `elixir/lib/symphony_elixir/orchestrator.ex`, `.../config.ex`.
- **Aplicação no Cosca**: agendador com limites em três camadas (global, por-metadado/estado, por-recurso). Ordenar fila por prioridade+senioridade+tie-breaker e, em vez de falhar, "esperar" (requeue) quando não há slot. Para execução distribuída, pool com least-loaded + preferência de host anterior e nunca failover silencioso após efeitos colaterais.

## 7. Adapter de tracker como "kernel de leitura" + agent tools host-side; segredos no host, fora do processo filho
- **O que resolve**: atender os boundares de segurança/integração — o orquestrador só lê o tracker; as escritas ficam em tools nativas do provedor executadas no host, mantendo o orchestrator agnóstico de provedor e o segredo fora do processo do agente.
- **Como funciona**: o módulo `Tracker` é um behavior (`fetch_issues_by_states`, `fetch_issues_by_ids`, `agent_tool_specs`, `execute_agent_tool`, `secret_environment_names`) com adapters plugáveis por `tracker.kind` (linear, github, jira, asana, gitlab, memory). O orquestrador usa só os callbacks de leitura e normaliza para um `Issue` estável. As tools de agente são executadas host-side com credencial do adapter; `secret_environment_names` remove as variáveis de token do ambiente do processo filho (unset no launch, `env: [{name,false}]` no Port). Tracker escritas => via tools, não via API de escrita do orquestrador.
- **Onde**: `elixir/lib/symphony_elixir/{tracker,tracker/issue,codex/dynamic_tool}.ex`, adapters em `{linear,github,jira,asana,gitlab}/`.
- **Aplicação no Cosca**: o núcleo do orquestrador expõe apenas um "kernel de leitura" da fonte de verdade, e as ações mutadoras são tools nativas do provedor executadas host-side (credencial confinada ao processo host, nunca no agente filho). Cada integração vira um adapter com tool-specs + `secret_environment_names`, e o estado de uma execução "binda" um snapshot adapter+settings por sessão para impedir que um reload mude o provedor no meio de um run.

---

## Synthesis — o que o Cosca deveria copiar

| # | Padrão | Ganho |
|---|--------|-------|
| 1 | Orquestrador de autoridade única (claim states) | dispatch idempotente sem dispatcher duplicado |
| 2 | Runs isoladas via workspace determinístico + path-safety | execuções paralelas sem pisar umas nas outras |
| 3 | Worker como sessão contínua + re-checagem do tracker | agenda o trabalho, não a conversa (anti-microgestão) |
| 4 | Fila de retry de 2 níveis (continuação vs falha) | diferencia "continuar" de "recuperar de erro" |
| 5 | Concorrência em 3 níveis | saturação gradável sem preempção arbitrária |
| 6 | Tracker como kernel de leitura + tools host-side | segredo fora do processo do agente; integrações plugáveis |
| 7 | `WORKFLOW.md` como política versionada | agendador dirigido por config no repo, com hot-reload seguro |

## Related Patterns

- [`openai-agents-sdk-patterns.md`](openai-agents-sdk-patterns.md) — o outro lado (loop do agente)
- [`deepseek-harness-patterns.md`](deepseek-harness-patterns.md) — sandbox process-tree (B3)
- [`temporal-workflow-engine-patterns.md`](temporal-workflow-engine-patterns.md) — execução durável
