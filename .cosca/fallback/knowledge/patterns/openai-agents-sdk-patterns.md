# OpenAI Agents SDK Patterns — Agente Declarativo, Handoffs, Guardrails

> **Version**: 1.0.0 | **Confidence**: 0.90 | **Category**: AI Agent Patterns | **Created**: 2026-08-22 | **Source**: https://github.com/openai/openai-agents-python (MIT)

> **Mined by**: cosca-kernel (ordem do Don). Extraído de `src/agents/`. O SDK multi-agente da OpenAI — o modelo de referência para orquestração de agentes em Python.

## Purpose

O OpenAI Agents SDK resolve o problema de **compor agentes multi-agente com delegação de 1ª classe** (handoffs), guardrails como portões tripwire e um loop de orquestrador com estado serializável. É o modelo mental mais próximo do que o Cosca quer para os seus chefes (cosca-*) e specialists.

---

## 1. Agent como config declarativa composta (não classe "ativa")
- **O que resolve**: modelar um agente para ser fácil de declarar, compor, derivar e testar, sem acoplar lógica de runtime.
- **Como funciona**: `Agent[TContext]` é um `@dataclass` genérico com TODAS as possibilidades como campos opcionais com defaults: `instructions` (string **ou** callable `(ctx, agent) -> str`), `prompt`, `model`, `tools`, `handoffs`, `input/output_guardrails`, `output_type`, `hooks`, `tool_use_behavior`. Validação em `__post_init__`. `clone(**kwargs)` usa `dataclasses.replace` para derivar um novo agente (ex. `agent.clone(instructions=...)`). Memória não vive no agente — vem de fora via `context` param + `RunConfig`.
- **Onde**: `src/agents/agent.py` (`AgentBase:182`, `Agent:295`, `clone:548`, `get_system_prompt:1042`).
- **Aplicação no Cosca**: cada `cosca-*` vira um `Agent` declarativo; `instructions` dinâmico é o gancho do Memory/Context Chief injetar o contexto do projeto no run. `clone()` deriva variantes (ex. cosca-cto focado só em ADR). `TContext` = um `SessionContext` compartilhado do Cosca.

## 2. Handoff como ferramenta roteável de 1ª classe (+ compressão de histórico)
- **O que resolve**: delegação entre agentes — transferir controle de forma modular, com controle de quais inputs o próximo agente vê, sem explodir contexto.
- **Como funciona**: `handoff(agent, ...)` fabrica um `Handoff` registrado como tool de nome `transfer_to_<agent>`. No `process_model_response`, a tool de handoff é roteada para `execute_handoffs`, emite `HandoffOutputItem`, dispara `hooks.on_handoff` e decide `NextStepHandoff(new_agent)` — o descendente **assume a conversação** (diferente de `agent.as_tool()` que continua o pai). Pontos-chave: `input_filter` (filtra o `HandoffInputData` só para o modelo), `is_enabled` (esconde o handoff do LLM em runtime), `nest_handoff_history` que colapsa o transcript anterior em UMA mensagem `assistant` com marcadores `<CONVERSATION HISTORY>`.
- **Onde**: `src/agents/handoffs/__init__.py:126`, `src/agents/handoffs/history.py:83`, `src/agents/run_internal/turn_resolution.py:527`.
- **Aplicação no Cosca**: cada chefão (cosca-ceo/cto) orquestra chefes de domínio como handoffs. `is_enabled` liga/desliga chefes por fase sem recompilar o grafo. `nest_handoff_history` é o mecanismo para os chefes de doc/display resumirem a jornada anterior — essencial para poupar tokens em runs longos.

## 3. Loop orquestrador como máquina de estados de turnos (Runner)
- **O que resolve**: orquestrar a iteração agent↔model↔tools até um resultado final, com limites e pontos de pausa/resumo.
- **Como funciona**: `Runner.run(...)` → `while True:` chama `run_single_turn`, que produz um `SingleStepResult` com um `next_step`. Os tipos de passo são um enum explícito (`run_steps.py`): `NextStepFinalOutput`, `NextStepHandoff(new_agent)`, `NextStepRunAgain`, `NextStepInterruption` (aprovação humana pendente). O loop decide: final? → termina; handoff? → roda o novo agente; tools? → executa e re-roda. Um "turno" = 1 invocação de IA. `max_turns` cancela; exceções `MaxTurnsExceeded`/`InputGuardrailTripwireTriggered`/`OutputGuardrailTripwireTriggered` e handlers resolvem sem quebrar. `RunState` permite serializar e **resumir** um run interrompido.
- **Onde**: `src/agents/run.py:248`, `src/agents/run_internal/run_steps.py:155`.
- **Aplicação no Cosca**: o orquestrador central implementa exatamente esse loop. `max_turns` protege contra loops de agentes chamando skills uns dos outros. `RunState`/`to_state()` dá **resumibilidade** — um pipeline que pausou numa aprovação pode retomar do mesmo ponto. `error_handlers` = política de degradação por chefe.

## 4. RunContextWrapper: memória compartilhada + ledger de efeitos colaterais
- **O que resolve**: passar um estado mutável único e tipado a todas as tools/handoffs/guardrails de um run, rastreando uso, histórico do turno, aprovações e dedup de invocações — sem passar os dados ao LLM.
- **Como funciona**: o `context` é embrulhado num `RunContextWrapper[TContext]` que carrega `context` (dados do app), `usage` (acumula tokens), `turn_input`, `_approvals` (mapa de decisões de aprovação), `_tool_invocations` (dedup de `call_id`). O wrapper é **injetado** em toda assinatura de tool/guardrail/hook. Aprovações têm "sticky" (bool, vale sempre) vs "per-call" (lista de call ids), e sobrevivem a handoffs via `_share_tool_state_with`. `_copy_for_run_state` clona usage/approvals para um checkpoint resumível sem vazar tokens.
- **Onde**: `src/agents/run_context.py:72`, `src/agents/tool_context.py:42`.
- **Aplicação no Cosca**: um `SessionContext` (stack detectada, memória do projeto, objetivos) é o `TContext` — todas as skills/chefes leem/escrevem pagas via wrapper. O ledger de invocações+aprovações é perfeito para as skills que precisam de aprovação humana (release, migration) com decisão "sempre" vs "só essa". Isso é a "memória do agente": contexto de app + estado do run, não histórico de LLM.

## 5. Declaração de tool via `@function_tool` (+ composição `agent.as_tool`)
- **O que resolve**: transformar funções Python em tools seguras e auto-descritas, e compor agentes como ferramentas.
- **Como funciona**: `function_tool` introspecta a assinatura para gerar o JSON schema (modo **strict** por padrão), usa o docstring para descrição, detecta sync/async. Se o param for `RunContextWrapper`/`ToolContext`, é injetado. `FunctionTool` carrega política: `is_enabled`, `needs_approval`, `timeout_seconds`/`timeout_behavior`, `tool_input/output_guardrails`, `failure_error_function` (erro→mensagem visível ao modelo em vez de exceção). `agent.as_tool()` roda o agente aninhado como tool (o pai continua a conversa — contraste com handoff). Erros de tool são normalizados em outputs re-renderizáveis.
- **Onde**: `src/agents/tool.py:2509`, `src/agents/agent.py:583`.
- **Aplicação no Cosca**: cada skill/tool de chefe é um `@function_tool`: schema vem do tipo (Cosca é tipado), `is_enabled` liga/desliga por contexto, `needs_approval` gateia ações destrutivas. Erros virarem mensagem (em vez de exceção) mantém o run produzindo. **Composição em camadas**: `agent.as_tool()` mantém um sub-agente como "especialista" dentro de um chefe maior; `handoff` é para troca de dono da conversa.

## 6. Guardrails como portões paralelos tipo tripwire
- **O que resolve**: validar entrada e saída, abortar com segurança e impedir que um agente produza/consuma algo indesejado, sem colapsar o run.
- **Como funciona**: `@input_guardrail`/`@output_guardrail` retornam `GuardrailFunctionOutput{output_info, tripwire_triggered}`. Se `tripwire_triggered=True`, o run lança `InputGuardrailTripwireTriggered`/`OutputGuardrailTripwireTriggered`. Semântica pontual: **input guardrails rodam só no primeiro agente**, `run_in_parallel` (default True) define se corre em paralelo à geração — e quando um tripwire de input dispara em paralelo, **cancela a task de LLM em voo**. Output guardrails rodam no output final (depois dos tools). Há guardrails por-tool também.
- **Onde**: `src/agents/guardrail.py:19`, `src/agents/run_internal/guardrails.py:67`, `src/agents/tool_guardrails.py`.
- **Aplicação no Cosca**: guardrails como gates de governança. Input guardrail = "este request é arquitetura ou bug?" que pode **cancelar** geração e rotear (combinando com handoff). Output guardrail em doc/security = valida compliance/estilo antes de aceitar a resposta. O padrão "tripwire cancela tarefa em voo" é ideal para o Critic Chief abortar antes de gastar o run inteiro.

## 7. Observabilidade em árvore de spans (contextvar) + execução paralela via `gather_with_cancel`
- **O que resolve**: rastrear o que cada agente/tool/handoff fez num run multi-agente, e rodar ferramentas/checks em paralelo com descarte limpo de irmãos em falha.
- **Como funciona**: tracing é hierárquico via contextvars: `TraceCtxManager` cria a trace, `agent_span`/`handoff_span`/`tool_span`/`guardrail_span`/`generation_span` abrem spans aninhados, cada span carrega `SpanData`. O provider é plugável por `TracingProcessor` (troca o exporter sem tocar nos agentes). `gather_with_cancel(*awaitables, on_child_failure=...)` roda vários awaitables juntos e, se UM falhar, cancela e drena os irmãos — usado no plano de tools (executa function/computer/custom/shell em paralelo). `run_producer_consumer` dá o padrão produtor-consumidor para streaming.
- **Onde**: `src/agents/tracing/__init__.py:94`, `src/agents/util/_asyncio_tasks.py:93`, `src/agents/run_internal/tool_planning.py:944`.
- **Aplicação no Cosca**: a árvore de spans dá "observe o pipeline inteiro" (qual chefe chamou qual skill, quantos tokens, onde quebrou). `gather_with_cancel` + plano de tools paralelo é o modelo para rodar vários chefes/skills em paralelo num turno e abortar todos coerentes se um falhar.

---

## Synthesis — o que o Cosca deveria copiar

| # | Padrão | Ganho |
|---|--------|-------|
| 1 | Agent declarativo + `clone()` | chefes compostos e deriváveis sem classe imperativa |
| 2 | Handoff de 1ª classe + `nest_handoff_history` | delegação sem explodir contexto |
| 3 | Runner com `max_turns` + `RunState` | loop seguro + resumibilidade de pipelines |
| 4 | `RunContextWrapper` (memória + ledger de aprovação) | estado de run compartilhado sem vazar ao LLM |
| 5 | Guardrails tripwire paralelos | gates de governança que abortam cedo |
| 6 | `gather_with_cancel` | paralelismo coerente com abort de irmãos |

## Related Patterns

- [`deepseek-harness-patterns.md`](deepseek-harness-patterns.md) — plugin/agent-loop/session (C1-C6)
- [`hermes-agent-patterns.md`](hermes-agent-patterns.md) — memória/SessionDB/trajectory
- [`temporal-workflow-engine-patterns.md`](temporal-workflow-engine-patterns.md) — execução durável
