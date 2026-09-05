# Google Agent Patterns — ADK, Skills Spec, Sub-agentes, Workflow

> **Version**: 1.0.0 | **Confidence**: 0.88 | **Category**: AI Agent Patterns | **Created**: 2026-08-22 | **Source**: https://github.com/google (adk-python 21k★, skills 18k★)

> **Mined by**: cosca-kernel (ordem do Don). Extraído de `adk-python` (Agent Development Kit) e `skills` (spec de Agent Skills). Complementa o `deepseek-harness-patterns.md` e o `kubernetes-org-patterns.md`.

## Purpose

O Google ADK resolve o problema de **compor agentes em árvore de sub-agents com transferência de controle** e de **padronizar skills como contratos ativáveis**. As duas lições estruturais: (1) agente como **estrutura de dados** (Pydantic) com hierarquia declarativa de sub-agents; (2) skills com **frontmatter mínimo + descrição anti-undertrigger + progressive disclosure**.

---

## A. ADK (Agent Development Kit) — orquestração de agentes

### A1. Config-declarativa de Agente (Pydantic + sub-agents) — "Agent as a Data Structure"
- **O que resolve**: definir um agente por configuração (não por loops imperativos) e compor hierarquias de sub-agents declarativamente, com validação automática e herança de runtime.
- **Como funciona**: `BaseAgent` é um `BaseModel` pydantic com `name`, `description`, `sub_agents: list[BaseAgent]`, callbacks. `model_post_init` monta a árvore: cada sub-agente recebe um único `parent_agent` (agente só pode entrar numa árvore uma vez; reusar exige `clone()`). Validadores fortes: nome deve ser identificador Python e `user` é reservado. O runtime resolve config para objetos via `canonical_*`: se o agente não define `model`, herda do ancestor; senão registra no `LLMRegistry`.
- **Onde**: `src/google/adk/agents/{base_agent,llm_agent,sequential_agent,parallel_agent,loop_agent}.py`.
- **Aplicação no Cosca**: o Cosca tem `Agent` struct com `ReportsTo`/`Department`/`Capabilities`. Valor transferível: (1) sub-agents como lista + `parent_agent` único com validação de ciclo/uniqueness; (2) `clone()` para reusar o mesmo agente em dois pontos da árvore sem compartilhar estado; (3) herança de configuração (modelo/instrução) do pai. Cosca hoje lê SKILL.md como flat registry; adotar esses dois dá composição profunda de supervisor↔subespecialistas.

### A2. Routing por "transferência de controle" (tool `transfer_to_agent` com enum constraint)
- **O que resolve**: deixar o LLM decidir a quem entregar o turno sem alucinar nomes de agentes, mantendo um grafo de vizinhança explícito (pai ↔ filho ↔ irmãos).
- **Como funciona**: a cada turno um processor injeta no prompt a lista de alvos válidos e um tool `transfer_to_agent` cujo schema é **enum-restrito** (`TransferToAgentTool` com `agent_name.enum = agent_names`). `_get_transfer_targets` calcula filhos (exceto `single_turn`/`task`), pai (se não `disallow_transfer_to_parent`) e irmãos. O loop resume o sub-agente apontado pelo último `transfer_to_agent`. `mode` (`chat`/`task`/`single_turn`) decide o comportamento.
- **Onde**: `tools/transfer_to_agent_tool.py`, `flows/llm_flows/agent_transfer.py`, `llm_agent.py`.
- **Aplicação no Cosca**: o `Router` é determinístico. O padrão enum-restrito é o upgrade: permitir que o `KERNEL` delegue a um `Chief`/`Specialist` via um tool de "transfer" que recebe um enum dos agentes da registry — elimina "agente não encontrado" e respeita o grafo `ReportsTo`.

### A3. Agent loop como pipeline de processadores (`BaseLlmFlow` = preprocess → LLM → postprocess)
- **O que resolve**: orquestrar a conversa multi-turn como um loop composto de processadores empilháveis, em vez de um while monolítico.
- **Como funciona**: `BaseLlmFlow.run_async` roda até o último evento ser `is_final_response`. Cada passo: (1) `_preprocess_async` executa `request_processors` (instruções dinâmicas, transfer targets, tools), (2) `_call_llm_async`, (3) `_postprocess_async` executa `response_processors` (schema de output, handle de tool calls, transfer). Processadores são classes `BaseLlmRequestProcessor`/`BaseLlmResponseProcessor` que produzem `Event`s — o loop é o mesmo, o comportamento é pluginado.
- **Onde**: `flows/llm_flows/{base_llm_flow,basic,single_flow,auto_flow}.py`.
- **Aplicação no Cosca**: o padrão forte é a **composição por processors por agente**: em vez de estágios fixos do pipeline, cada agente levaria seus próprios "augmentors" de prompt e pós-processadores. Permite variar o fluxo por agente sem multiplicar pipelines globais.

### A4. Event-sourcing de sessão + estado com delta pendente (e rewind)
- **O que resolve**: persistir sessão e estado de forma auditável, derivável e reversível, com serviços de storage intercambiáveis.
- **Como funciona**: `Session` = `id`, `user_id`, `state: dict`, `events: list[Event]`. Toda mudança de estado vira um `Event` com `actions.state_delta`; o estado atual é a reprodução dos deltas. `State` mantém valor atual + delta pendente (`has_delta()`, `to_dict()`), com prefixos `app:`/`user:`/`temp:`. Há múltiplos `*_session_service` e `rewind_async` que computa o delta inverso para reverter estado/artefatos. Eventos `partial` não são persistidos.
- **Onde**: `sessions/{session,state,base_session_service,sqlite_session_service}.py`, `runners.py`.
- **Aplicação no Cosca**: o estado da sessão é um blob; adotar eventos com `state_delta` permite auditoria, replay determinístico e desfazer estados (rewind) em rollback. O `state_schema` pydantic como contrato tipado casa com o `PipelineData` tipado do Cosca.

### A5. Resumabilidade via `InvocationContext` (branch + isolation_scope + agent_state/end_of_agent)
- **O que resolve**: retomar uma execução interrompida (tool longa/HITL) no agente correto, sem duplicar trabalho, isolando conversas paralelas e sub-agentes.
- **Como funciona**: `InvocationContext` carrega `agent`, `session`, `branch`, `agent_states`, `plugin_manager`, `canonical_tools_cache`. Quando resumível, cada agente grava um marcador `end_of_agent` em `agent_states[name]`; o runner escolhe quem continua seguindo o autor do último `function_call`. `branch` isola ramos paralelos; `isolation_scope` isola a conversa de um sub-agente `task`/long-running dos irmãos. Ferramentas long-running pausam a invocação (`should_pause_invocation`) e retomam com `resume_inputs`.
- **Onde**: `agents/invocation_context.py`, `runners.py`, `llm_agent.py`.
- **Aplicação no Cosca**: o `PipelineContext` comporta naturalmente `invocation_id` + `branch`. O ganho: suportar interrupção/retomada no meio do pipeline (tool de sandbox demorada ou HITL de aprovação), com `agent_states` por sub-executor e um seletor que "volta" para o estágio/agente interrompido — hoje o executor percorre estágios em cadeia sem checkpoint de retomada.

### A6. Plugins como corrente de hooks de ciclo de vida (notificação-only de erro)
- **O que resolve**: instrumentação/customização transversal (telemetria, auth, segurança, logs) sem tocar no agente, com contrato claro para erro.
- **Como funciona**: `PluginManager` executa callbacks em todos os pontos (`before/after_run`, `before/after_model`, `before/after_tool`, `on_event`, `on_agent_error`). Ordem: **plugins primeiro, depois callbacks canônicos** (plugin que retornar conteúdo faz short-circuit). Callbacks de erro são **notificação-only e best-effort**: engolem a própria exceção para nunca mascarar a original.
- **Onde**: `plugins/{plugin_manager,base_plugin}.py`, hooks em `base_llm_flow.py`.
- **Aplicação no Cosca**: elevantar gates inline (`execpolicy`/`circuitbreaker`) a plugins de ciclo de vida dá ordem de precedência testável, sem que falhas de instrumentação derrubem a execução real.

### A7. Grafo de Workflow unificado com BaseNode (trigger-buffer + scheduler + replay)
- **O que resolve**: expressar orquestração determinística (bifurcação, join, paralelismo, nós dinâmicos) onde agente = nó, com retomada por replay dos eventos.
- **Como funciona**: `Workflow` é um `BaseNode` cujo `_run_impl` é o loop de orquestração: **SETUP** (constrói grafo via `Graph.from_edge_items`, semeia triggers), **LOOP** (`_run_loop`: `_schedule_ready_nodes` drena `trigger_buffer` → cria `NodeRunner` em `pending_tasks` → `asyncio.wait(FIRST_COMPLETED)` → `_handle_completion` bufferiza triggers dos nós downstream, respeitando `_requires_all_predecessors` para join e `max_concurrency`), **FINALIZE** (coleta outputs ou propaga `interrupt_ids`). Nós dinâmicos via `ctx.run_node`; LLM agents são nós. Um nó pode ser empacotado como tool (`NodeTool`). Retomada por replay: `recovered_executions` + `sequence_barrier`; nós já completos são fast-forwardados.
- **Onde**: `workflow/{_workflow,_base_node,_graph,_node_runner,_dynamic_node_scheduler,_function_node}.py`.
- **Aplicação no Cosca**: o padrão mais valioso é o **orquestrador como loop dirigido por triggers + scheduler de tasks**, em vez de cadeia linear de estágios: permite que o `KERNEL` despache agentes em paralelo com join por "all predecessors", `max_concurrency`, e nós dinâmicos. Com `SequenceBarrier`, o `PipelineContext` ganharia ramos determinísticos e retomagem por replay dos eventos.

---

## B. Google Skills — spec de Agent Skills

### B1. SKILL.md como Contrato Único + Frontmatter Mínimo
- **O que resolve**: dá ao agente um ponto de entrada único, inequívoco e auto-descritivo por skill, sem schema complexo.
- **Como funciona**: cada skill é um diretório cujo coração é `SKILL.md`, que abre com frontmatter de 3 coisas: `name` (slug kebab-case = nome do diretório), `metadata.category` (taxonomia fechada), `description` (começa com verbo de ação e fecha com **`Use when ...`** e **`Don't use when ...`** — a fonte primária de trigger). Corpo com H2 padronizados (Getting Started / Core Principles / Safety / Structured Workflows) e admonições markdown (`> [!CAUTION]`, `> [!IMPORTANT]`).
- **Onde**: `skills/cloud/gcloud/SKILL.md`, `skills/cloud/bigquery-basics/SKILL.md`.
- **Aplicação no Cosca**: padronizar o frontmatter em `name` + `metadata.category` (taxonomia dos departamentos/engines) + `description` com `Use when`/`Don't use when`. Transforma cada engine/departamento em contrato ativável, em vez de descrição solta no tool-loading.

### B2. Progressive Disclosure via `references/` + `scripts/` + `assets/`
- **O que resolve**: o problema de "skill enorme que inunda o contexto" — mantém a carga inicial pequena e puxa profundidade sob demanda.
- **Como funciona**: o `SKILL.md` fica "lean" (high-level instructions). Diretórios opcionais offloadam: `references/` (documentação pesada, specs), `scripts/` (código determinístico — o SKILL.md manda executar, não reproduzir inline), `assets/` (templates/media estáticos).
- **Onde**: `skills/cloud/bigquery-basics/SKILL.md:66-93`, `skills/cloud/agent-platform-skill-registry/SKILL.md:66-80`, `references/generate-skill.md:20-28`.
- **Aplicação no Cosca**: engines como `cognitive-economy`, `wisdom-distillation` devem ter `scripts/` (mover helpers de Python para fora) e `references/` (separar documentos pesados do prompt-ativo).

### B3. Disambiguation por "Use/Don't-use" + Roteamento de Catálogo
- **O que resolve**: quando há dezenas de skills, como o agente escolhe a certa sem ler todas.
- **Como funciona**: dois mecanismos. (1) **Descrição auto-desambiguante**: `Use when` (gatilho positivo) + `Don't use when` (anti-scoping — rejeita o caso onde outra skill é dona). (2) **Regra de roteamento** (`rules/google-cloud-discovery.md`): um "routing map" que lista prefixos de slug, avisa que skills existem mas não estão instaladas, e manda instalar. Regra de ouro: **"Never infer a skill's contents from its name."**
- **Onde**: `skills/cloud/{gcloud,google-cloud-recipe-onboarding}/SKILL.md`, `plugins/cloud/google-cloud-core/rules/google-cloud-discovery.md`.
- **Aplicação no Cosca**: o `discovery` puede ganhar um mapa de roteamento do inventário de 71 skills por prefixo/categoria e, ao faltar uma skill, dizer "você não tem essa engine instalada; ofereça a instalação" em vez de responder de memória.

### B4. Composição e Encadeamento de Skills (orquestrador ⟶ folhas, com fallback)
- **O que resolve**: permitir skills "meta" que orquestram skills menores sem duplicar conhecimento, formando um DAG de composição — com fallback gracioso quando a skill dependente não está disponível.
- **Como funciona**: duas formas. **Chaining bottom-up**: skill termina apontando para downstream skills. **Composição top-down**: skill referencia os pilares dentro de uma task e define fallback: "If any of the specialized skills are not available, derive design guidance directly from the documentation references." Ou seja: referenciar por NOME da skill (não por conteúdo) e ter sempre um caminho offline.
- **Onde**: `skills/cloud/google-cloud-{recipe-onboarding,solution-architecture}/SKILL.md`.
- **Aplicação no Cosca**: modelar um registro de dependências skill→skill: engines "folha" reutilizáveis e engines "orquestradora" que invocam por nome. O fallback evita alucinar se a skill alvo não estiver no inventário.

### B5. Guardrails e Autorização codificados como Conteúdo da Skill
- **O que resolve**: para agentes autônomos/headless, o perigo não é falta de capacidade, é ação destrutiva não autorizada. O Google embute a política dentro da skill.
- **Como funciona**: padrões de "safety contract": **denylist de operações proibidas** (o que NUNCA executar autonomamente); **dry-run/validate-only obrigatório**; **modo não-interativo** (`--quiet`); **redução de dados** (proibido list sem `--limit`/`--filter`); **Consent Gate** (tabela markdown + pergunta EXATA + "strictly stop to wait for positive affirmation"); **Single-Question Policy**.
- **Onde**: `skills/cloud/gcloud/SKILL.md:15-68`, `skills/cloud/google-cloud-recipe-onboarding/SKILL.md:119-131`.
- **Aplicação no Cosca**: cada SKILL.md deve declarar a própria política de autorização (ops proibidas, dry-run mandatório, pergunta única). Co-locar o contrato de segurança na própria skill (o engine que executa é o mesmo que declara o que não pode).

### B6. Anti-Alucinação e Grounding: "Autoridade Exclusiva" + MCP como fonte de fatos
- **O que resolve**: skills de produto envelhecem (flags/SDK mudam) — separa "como proceder" (skill) de "fatos atualizados" (MCP/library), e bane busca web não autorizada.
- **Como funciona**: (1) **autoridade exclusiva** para sintaxe (só validado por `--help`; proibi web search); (2) **grounding externo por MCP** ("use the Developer Knowledge MCP server `search_documents` tool"); (3) "Never infer a skill's contents from its name".
- **Onde**: `skills/cloud/gcloud/SKILL.md:16-68`, `skills/cloud/bigquery-basics/SKILL.md:92-93`, `plugins/cloud/google-cloud-core/{gemini-extension,mcp_config}.json`.
- **Aplicação no Cosca**: as skills não devem ser a fonte de fatos (que envelhecem), e sim o orquestrador que aponta para um repositório de conhecimento/embeddings como autoridade. Mapeia direto ao RAG/semantic-memory: a skill diz `search` na base semântica em vez de embutir respostas. Reforça a política anti-hallucination ("nunca inferir conteúdo de uma engine pelo nome").

### B7. Ecossistema de Skills Público, Versionado e Instalável
- **O que resolve**: escalar um inventário grande de skills e torná-lo distribuível, instalável e versionável em múltiplos harnesses (Claude Code, Codex, Gemini).
- **Como funciona**: três camadas: (1) catálogo raiz `npx skills add google/skills` (instalação seletiva); (2) **plugins** (`plugin.json` com schema versionado `$schema` + `name/version/description/author/keywords`); (3) **marketplace multi-harness** (`.claude-plugin/marketplace.json` e `.agents/plugins/marketplace.json` com `source.github`/`ref` tag versionada e `policy.installation`).
- **Onde**: `plugins/cloud/google-cloud-core/plugin.json`, `.claude-plugin/marketplace.json`, `.agents/plugins/marketplace.json`.
- **Aplicação no Cosca**: um `skill-manifest` (equivalente a `marketplace.json`) com `id`, `version`, `reports-to`, `policy.installation`. Schema `$schema` versionado + `ref` por tag é o padrão para engines/departamentos serem adicionados via "mercado de capacidades".

---

## Synthesis — o que o Cosca deveria copiar

| # | Padrão | Ganho |
|---|--------|-------|
| 1 | A1 — Agente como estrutura de dados + `clone()` | composição profunda supervisor↔subespecialista |
| 2 | A2 — Transfer com enum-restrito | delegação sem "agente não encontrado" |
| 3 | A7 — Workflow com trigger-buffer + scheduler | orquestração paralela/join/retomável |
| 4 | A4 — Event-sourcing + rewind | auditoria, replay, rollback de estado |
| 5 | B1/B3 — Frontmatter mínimo + Use/Don't-use | contratos ativáveis, anti-undertrigger |
| 6 | B2 — Progressive disclosure | skills escaláveis sem inchar contexto |

## Related Patterns

- [`anthropics-skills-patterns.md`](anthropics-skills-patterns.md) — a outra spec de skills (Claude)
- [`deepseek-harness-patterns.md`](deepseek-harness-patterns.md) — skill registry multi-camada (D1/D2)
- [`openai-agents-sdk-patterns.md`](openai-agents-sdk-patterns.md) — agente declarativo/handoff
