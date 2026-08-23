# Anthropic/Claude Patterns — claude-code, Skills, Agent SDK

> **Version**: 1.0.0 | **Confidence**: 0.90 | **Category**: AI Agent Patterns | **Created**: 2026-08-22 | **Source**: https://github.com/anthropics (claude-code 142k★, skills 171k★, claude-agent-sdk-python 7.9k★)

> **Mined by**: cosca-kernel (ordem do Don). Extraído de `claude-code`, `skills`, `claude-agent-sdk-python`. Complementa o `deepseek-harness-patterns.md` (plugin/agent-loop) e o `google-agent-patterns.md` (outra spec de skills).

## Purpose

O Claude é o harness de agente de terminal mais maduro. As lições estruturais: (1) **contrato de hooks por eventos** (JSON-in/JSON-out, exit-code como control-flow) que desacopla totalmente o loop do núcleo; (2) **permissões declarativas em 3 estados** imune a override; (3) **memória de sessão com baseline git**; (4) **skills com progressive disclosure + meta-loop A/B**; (5) **meta-loop para modelar a anthropic-skills-progressions**.

---

## A. claude-code — harness do agente

### A1. Contrato de Eventos de Hook: JSON-in / JSON-out + exit-code como control-flow
- **O que resolve**: estender o loop do agente em pontos precisos (antes/depois de cada tool, fim do turno, submit do prompt) sem tocar no núcleo. É a espinha dorsal da extensibilidade.
- **Como funciona**: um `hooks.json` registra handlers por evento (`PreToolUse`, `PostToolUse`, `Stop`, `SessionStart`, `UserPromptSubmit`, `SubagentStop`, `PreCompact`). Cada handler é um subprocesso que lê payload JSON via **stdin** (`tool_name`, `tool_input`, `tool_response`, `cwd`, `session_id`) e escreve JSON em stdout. **Exit code é o control-flow**: `0`=permite, `1`=mostra stderr ao usuário (não ao modelo), `2`=bloqueia a tool e injeta stderr como feedback pro modelo. Em `PreToolUse`/`Stop`, JSON pode trazer `permissionDecision: "deny"`/`decision: "block"` + `reason`.
- **Onde**: `examples/hooks/bash_command_validator_example.py`, `plugins/{security-guidance,hookify}/hooks/*`.
- **Aplicação no Cosca**: é o molde do pipeline de eventos do agent-loop — um entrypoint por ponto de interceptação que lê stdin, devolve `{decision/permissionDecision/systemMessage}` e usa exit code para prosseguir/bloquear/realimentar. A cadência `SessionStart→UserPromptSubmit→PostToolUse→Stop` espelha o que o `cosca-runtime`/`cosca-context` precisa instrumentar.

### A2. Modelo de Permissão em Três Estados + Hierarquia de Config "Managed-settings"
- **O que resolve**: controlar o que o agente PODE fazer de forma determinística e à prova de override malicioso.
- **Como funciona**: `permissions` usa a tríade `allow`/`ask`/`deny` por tool. Hardening: `disableBypassPermissionsMode: "disable"` (mata `--dangerously-skip-permissions`) e `allowManagedPermissionRulesOnly` (bloqueia regras do usuário/projeto, permite só as gerenciadas). Hierarquia: `~/.claude` (user) < `.claude/` (projeto, commitado) < `.claude/*.local.*` (gitignored) < `managed-settings.json` (enterprise/MDM). `sandbox.enabled` força Bash dentro de sandbox com allowlist de rede (`allowedDomains`, `allowUnixSockets`).
- **Onde**: `examples/settings/settings-{strict,lax,bash-sandbox}.json`, `examples/mdm/managed-settings.json`.
- **Aplicação no Cosca**: mapeia para o `cosca-governance`/`cosca-security`. O Cosca tem hierarquia de agentes; o padrão adiciona a camada **declarativa de permissão por ferramenta** com "modo gerenciado" imune a override — essencial para sub-agentes e plugins não escalarem privilégios. `sandbox` + allowlist de rede é o próximo passo para `cosca-infrastructure`/`cosca-runtime`.

### A3. Memória de Sessão como Arquivo de Estado com Lock Atômico e TTL
- **O que resolve**: hooks são processos efêmeros e stateless; este padrão dá estado durável atravessando turnos com concorrência segura (multi-agent / shared-worktree).
- **Como funciona**: um JSON de estado por `session_id`, acessado por `load_state`/`save_state`/`with_locked_state` (lock via flock/arquivo sentinela). Padrões atômicos: `atomic_check_and_mark_warning` (aviso uma vez), `atomic_check_counter` (max firings por turno), `atomic_check_rate_limit` (janela deslizante rolling-hour). TTLs independentes expiram subtotais.
- **Onde**: `plugins/security-guidance/hooks/{session_state,diffstate}.py`.
- **Aplicação no Cosca**: blueprint do `cosca-memory-chief`/`cosca-context` para memória de curto prazo por sessão: blob com lock, chaves transparentes e TTL. Substitui manter estado em RAM do processo; a semântica atômica serve para coordenação entre os muitos `cosca-*` sub-agents concorrentes no mesmo workspace.

### A4. Revisão Incremental por Git-Baseline (diff do que MUDOU, não do que é)
- **O que resolve**: focar o custo de LLM/contexto apenas na mudança real de cada turno — sem re-revisar código pré-existente nem re-flaggar achados já tratados.
- **Como funciona**: em `UserPromptSubmit`, captura baseline via `git stash create` (SHA da working tree) em paralelo com `ls-files` de untracked. No `Stop`, roda `git diff <baseline_sha>` e atualiza o baseline. Preserva o baseline antigo se o turno foi abortado (interrupt/crash/maxTurns). Cap de arquivos por diff (`MAX_DIFF_FILES=30`), dedup por `(filePath, category)`.
- **Onde**: `plugins/security-guidance/hooks/{gitutil,diffstate}.py`, `security_reminder_hook.py`.
- **Aplicação no Cosca**: resolve o problema clássico do agent-loop com muitos sub-agents: só alimentar o contexto com o delta. O `cosca-context`/`cosca-semantic-memory` pode usar snapshot+diff do repo como "o que mudou desde o último turno" — injeta contexto mínimo, detecta mudanças de outros agentes (race detection), atualiza memória só do que mudou.

### A5. Feedback Assíncrono com "Rewake" + Suffix de Continuação (Stop-as-quality-gate)
- **O que resolve**: rodar análise pesada em background (revisão de commit/push) e injetar o resultado sem interromper a intenção original.
- **Como funciona**: o hook de `Stop`/`PostToolUse[Bash]` roda com `asyncRewake: true`. Ao terminar, **exit code 2** (stderr = findings) para o modelo continuar; usa `rewakeMessage` (instrução de ação), `rewakeSummary` (uma linha no terminal) e um `CONTINUATION_SUFFIX` — "...continue with the user's original request... This is supplementary, not a replacement for your previous response". `MAX_STOP_HOOK_FIRINGS` (default 3) permite fix→review→fix sem loop infinito.
- **Onde**: `plugins/security-guidance/hooks/hooks.json`, `security_reminder_hook.py`.
- **Aplicação no Cosca**: mecanismo pelo qual um **crítico/vencedor assíncrono** dá veredito sem travar o loop principal. O `cosca-critic`/`cosca-qa` dispara em background no `Stop` e "rewake" o agente com findings, com invariante: feedback é suplementar, nunca substituto. O teto de firings é o guard-rail anti-oscilação.

### A6. Loop Auto-Referencial via Stop-Hook (plan→feedback→plan até uma "promise" de conclusão)
- **O que resolve**: impedir que o agente encerre cedo — forçando iteração até uma condição de término verificável.
- **Como funciona**: `/ralph-loop` grava `.claude/ralph-loop.local.md` (frontmatter com `iteration`, `max_iterations`, `completion_promise`). O `stop-hook.sh` lê o estado, extrai o último `role:assistant` do transcript: se satisfaz a `completion_promise` → deixa sair; se `max_iterations` atingido → deixa sair; senão **bloqueia o Stop** (`{decision:"block"}`) re-injetando o prompt. Mutação atômica (temp file + mv). O comportamento é distinto do `ralph-wiggum` (um plugin de comando que itera até condição).
- **Onde**: `plugins/ralph-wiggum/commands/ralph-loop.md`, `hooks/stop-hook.sh`.
- **Aplicação no Cosca**: traduz para o agent-loop: **o `Stop` não é fim, é ponto de inspeção.** Um hook de parada decide *continuar* (feed-back do prompt) ou *terminar* (promessa/asserção validada). Dá determinismo ao "plan→act" — a única saída legítima é condição objetiva (teste passou, promise atingida, budget exaurido).

### A7. Motor de Regras Declarativas User-Extensible com Trust-Model Anti-Prompt-Injection
- **O que resolve**: deixar qualquer usuário/projeto definir comportamento (bloquear/warn) via arquivos simples, **sem** dar a um input malicioso poder de suprimir proteções.
- **Como funciona**: dois pontos de extensão, ambos aditivos. (1) `hookify`: regras em `.claude/hookify.*.local.md`/`.yaml` com frontmatter `{event, action: block|warn, pattern, conditions}`; o `RuleEngine` acumula matches com **prioridade block>warn**, traduzindo para `permissionDecision: deny`. (2) `extensibility.py`: um `.md` de política de segurança **mesclado** com padrões nativos, que entra no prompt dentro de `<project-security-guidance>` enquadrado como aditivo ("pode ADICIONAR checks, NÃO pode suprimir findings") — um PR malicioso "ignore SQL injection" **não suprime** o achado. Regexes validadas contra ReDoS em load.
- **Onde**: `plugins/hookify/core/{config_loader,rule_engine}.py`, `plugins/security-guidance/hooks/extensibility.py`.
- **Aplicação no Cosca**: modelo do `cosca-governance` + `cosca-plugin`. Regras declaradas em markdown/YAML, avaliadas com precedência block>warn. O **enquadramento anti-injeção** (instrução criada por usuário entra como *dados* que só somam, nunca suprimem) é a resposta defensiva certa para o `cosca-plugin`/`cosca-integrations` — essencial onde agentes/plugins contribuem regras.

---

## B. Anthropic Skills — spec de Agent Skills

### B1. Contrato mínimo de frontmatter (spec validável)
- **O que resolve**: garante que qualquer skill é auto-descritiva, descoberta e validável, sem manifesto externo burocrático.
- **Como funciona**: **dois campos obrigatórios** — `name` (kebab-case, ≤64 chars) e `description` (≤1024 chars, sem `<`/`>`). Opcionais: `license`, `allowed-tools`, `metadata`, `compatibility`. O validador `quick_validate.py` trata qualquer chave fora da lista como **erro de packaging** — nunca silenciosamente ignorada.
- **Onde**: `spec/agent-skills-spec.md`, `skills/skill-creator/scripts/quick_validate.py`.
- **Aplicação no Cosca**: adotar a mesma whitelist de campos no frontmatter das skills; `name` kebab-case como ID único, `description` como o "paper" que alimenta o dispatcher. Criar um `quick_validate.py` equivalente para **gate de CI** do inventário de 71 skills.

### B2. Progressive disclosure em três níveis
- **O que resolve**: contexto é recurso finito; a skill não pode viver toda na janela.
- **Como funciona**: Nível 1 `name+description` (~100 palavras) sempre em contexto; Nível 2 corpo do `SKILL.md` quando a skill dispara (ideal <500 linhas); Nível 3 recursos empacotados (`scripts/`, `references/`, `assets/`) sob demanda. Regra prática: se o SKILL.md se aproxima de 500 linhas, não engorde — adiciona camada de hierarquia com ponteiros; se um reference >300 linhas, tabela de conteúdo obrigatória.
- **Onde**: `skills/skill-creator/SKILL.md` § "Progressive Disclosure", `skills/webapp-testing/SKILL.md`.
- **Aplicação no Cosca**: modelar skills como 3 camadas: frente enxuta (descrição do agente, sempre visível), corpo (instrução de ativação), diretório de recursos sob demanda. O `cosca-context` orquestra esse carregamento em camadas, mantendo só a "frente" das 71 skills em contexto.

### B3. Organização por domínio: SKILL.md como roteador para `references/`
- **O que resolve**: escalar UMA skill para muitos frameworks/linguagens sem estourar o orçamento de tokens — o agente lê só a variante que serve.
- **Como funciona**: a SKILL.md vira um **seletor** ("workflow + selection"): decide qual variante se aplica, aponta com caminho relativo, e o agente lê **apenas aquele** arquivo. Flagship: `claude-api/` com `python/`, `typescript/`, `go/`, `java/`, `ruby/`, `php/`, `curl/`, `shared/`; a SKILL.md tem "Language Detection" que infere a stack.
- **Onde**: `claude-api/SKILL.md` + subpastas, `mcp-builder/SKILL.md` + `reference/`.
- **Aplicação no Cosca**: é o padrão que resolve a colisão de escopo. Em vez de uma skill monolítica "frontend", ter `cosca-frontend` como roteador e `references/{react,vue,web-components}.md`. O `cosca-architecture` pode decidir quando partir uma skill em variantes.

### B4. Descrição acionável anti-undertrigger (contrato de gatilho)
- **O que resolve**: o maior risco real é **sub-disparo** — a skill existe mas o agente não a chama quando devia. A description é o único mecanismo de gatilho.
- **Como funciona**: a `description` deve conter **o que faz** E **quando usar** (não deixar "quando" no corpo). Deixá-la "pushy" — listar sinônimos, contextos concretos. Incluir **escopo-negativo** ("Do NOT use for PDFs..."). A skill-creator gera ~20 queries de trigger (8-10 should-trigger, 8-10 should-not-trigger), treina/testa 60/40, avalia 3x por descrição, itera ≤5× e elege a `best_description` por score.
- **Onde**: `skills/*/SKILL.md`, `skill-creator` § "Description Optimization" + `scripts/run_loop.py`.
- **Aplicação no Cosca**: auditar as 71 descrições como a "frente de gatilho" — cada uma com "o quê + quando" + escopo-negativo, e o `cosca-provider`/`cosca-context` invoque pelo texto, não por keyword naive. Reusar o harness de `run_loop` para otimizar o trigger dos agentes críticos com evals should-trigger/should-not-trigger.

### B5. Scripts empacotados como caixa-preta + deduplicação de improvisação
- **O que resolve**: (a) tarefas determinísticas/repetitivas não dependem do agente reinventar a roda; (b) código volumoso não polui a janela de contexto.
- **Como funciona**: `scripts/` guarda código executável; a regra é **executar via `--help`/CLI, não ler o fonte** — call-as-black-box. Sinal de refatoração: se subagentes **independentemente** escreveram o mesmo helper, bundlear em `scripts/` e mandar a skill usá-lo.
- **Onde**: `skills/webapp-testing/SKILL.md` (regra da caixa-preta), `skills/docx/scripts/`.
- **Aplicação no Cosca**: cada agente especializado deve empacotar utilitários canonizados como `scripts/` chamáveis via CLI, nunca re-escritos na hora. Convenção: "se N test-runs geram o mesmo helper, promova-o a script".

### B6. Agrupamento em coleções + empacotamento `.skill` validado
- **O que resolve**: skills crescem em catálogo — precisam ser agrupáveis em coleções semânticas, distribuíveis como artefato único e impeditivas de publicar inválido.
- **Como funciona**: agrupamento num `marketplace.json` (listas nomeadas de paths de skill com `description` e `strict`). Empacotamento: `package_skill.py` zipa a pasta num `.skill` (nome = frontmatter), **rodando `quick_validate` antes** e excluindo artefatos. Instalação via plugin marketplace.
- **Onde**: `.claude-plugin/marketplace.json`, `skill-creator/scripts/package_skill.py`.
- **Aplicação no Cosca**: tratar o inventário de 71 skills como coleções versionáveis no `cosca-release`. Definir um `marketplace.json` Cosca que agrupe agentes por domínio, e `package_skill.py` para emitir pacotes `.skill` com validação obrigatória como gate de release.

### B7. Meta-loop de avaliação e versionamento de skills (a skill que escreve skills)
- **O que resolve**: dá ao ciclo de vida da skill rigor científico — prova que a skill melhora o resultado, quantifica o ganho e versiona por evidência.
- **Como funciona**: a skill-creator executa para cada test-case um **par A/B no mesmo turno** (with-skill vs baseline without_skill; para melhorias, baseline = old_skill via snapshot). Escreve asserções objetivas (`grading.json`), captura `timing.json`, agrega em `benchmark.json` (pass_rate/tempo/tokens, mean±stddev, delta), lança viewer para revisão humana. Versiona com `history.json` (v0 → vN com `parent`, `pass_rate`, `is_current_best`).
- **Onde**: `skills/skill-creator/SKILL.md` (loop maior), `references/schemas.md`, `scripts/aggregate_benchmark.py`.
- **Aplicação no Cosca**: instrumentar cada agente com harness A/B (with-skill vs baseline), medir pass-rate/tempo/tokens, versionar via `history.json`. O `cosca-evolution`/`cosca-testing` rodam o loop para justificar (com dados) quando introduzir/melhorar/descontinuar uma skill.

---

## C. claude-agent-sdk-python — tornar o agente programável

### C1. Duas camadas de API para "tornar o agente programável"
- **O que resolve**: a mesma engine com duas ergonomias — um one-shot stateless e um conversacional full-duplex, sem duplicar a lógica interna.
- **Como funciona**: `query()` é um async generator unidirecional (manda o prompt e faz `async for`). `ClaudeSDKClient` é bidirecional e stateful (`connect()` → `query()` → `receive_response()` → `disconnect()`). Ambos convergem para o mesmo `InternalClient._query`, a diferença é `is_streaming_mode`.
- **Onde**: `src/claude_agent_sdk/{query,client}.py`, `src/claude_agent_sdk/_internal/query.py`.
- **Aplicação no Cosca**: expor um `cosca.query(prompt)` para batch/CI e um `CoscaClient` com `connect/query/receive/interrupt` para UI interativa, ambos sobre o mesmo orquestrador interno (`CoscaRuntime`).

### C2. Reverse protocolo de controle bidirecional multiplexado sobre um único stream NDJSON
- **O que resolve**: coordenar requisições que o agente faz de volta ao app (permissões, hooks, MCP) com o streaming de mensagens de saída, tudo num único stream, sem bloquear o `async for`.
- **Como funciona**: a classe `Query` tem dois canais: mensagens normais e mensagens de controle. Um único `_read_messages()` roteia por `type`: `control_response` completa um `Event`, `control_request` dispara handlers, `control_cancel_request` cancela requests em voo. Cada requisição carrega `request_id` (contador + `os.urandom`) para pareamento.
- **Onde**: `src/claude_agent_sdk/_internal/{query,transport/subprocess_cli}.py`, `types.py`.
- **Aplicação no Cosca**: um único canal full-duplex com `request_id` randomizado e `Event` por pendência. Habilita `interrupt()`, trocar modelo/modo em runtime e cancelamento de calls.

### C3. Controle programático de permissões com callback tipado (`can_use_tool`)
- **O que resolve**: substituir o prompt interativo de permissão por lógica em Python, permitindo allow/deny por contexto.
- **Como funciona**: `ClaudeAgentOptions.can_use_tool` é um callback `Callable[[str, dict, ToolPermissionContext], Awaitable[PermissionResult]]`. O `_handle_control_request` monta um `ToolPermissionContext` e o callback retorna `PermissionResultAllow` (com `updated_input`) ou `PermissionResultDeny` (com `message`/`interrupt`). Valida exclusividade com `permission_prompt_tool_name` e avisa via `CanUseToolShadowedWarning` se `allowed_tools`/`bypassPermissions` já auto-aprovam (o callback nunca roda).
- **Onde**: `src/claude_agent_sdk/types.py` (201-259, 1896-1919), `_internal/query.py` (478-530).
- **Aplicação no Cosca**: gatekeeper de ferramentas por agente em Python considerando contexto. O "shadowing advisory" é um padrão útil para alertar quando regras estáticas tornam o callback ineficaz — evita falhas silenciosas de segurança.

### C4. Hooks tipados por união discriminada + adaptação de palavras-chave Python→wire
- **O que resolve**: deixar o app observar/influenciar o ciclo de vida do agente com callbacks fortemente tipados, sem vazar keywords do CLI.
- **Como funciona**: `HookEvent` enumera 10 eventos; cada `HookInput` é um `TypedDict` discriminado por `hook_event_name`, entregue a um `HookCallback`. O retorno `HookJSONOutput` usa `async_`/`continue_` no Python; `_convert_hook_output_for_cli` converte para `async`/`continue` antes de mandar ao CLI.
- **Onde**: `src/claude_agent_sdk/types.py` (262-604), `_internal/query.py`.
- **Aplicação no Cosca**: hooks de `PostToolUse` para logar/validar saídas, `Stop`/`SubagentStop` para stats/cleanup por subagente. O tradeoff "async_/continue_" na borda evita keywords reservadas na API pública.

### C5. Ferramentas customizadas in-process via bridge MCP (`@tool` + `create_sdk_mcp_server`)
- **O que resolve**: adicionar ferramentas ao agente rodando no MESMO processo (zero IPC), com schema derivado de tipos Python e semântica de erro uniforme.
- **Como funciona**: `@tool(name, description, input_schema, annotations)` envolve um handler async que recebe um dict e retorna `{"content": [...], "is_error": bool}`. `create_sdk_mcp_server` monta `McpSdkServerConfig(type="sdk")`; o socket `run_tool` valida com `jsonschema`, converte conteúdo e reporta ferramenta-desconhecida/args-inválidos/exceções como `isError` (nunca erro de protocolo).
- **Onde**: `src/claude_agent_sdk/__init__.py` (251-619), `_internal/sdk_mcp_bridge.py`.
- **Aplicação no Cosca**: ferramentas nativas (busca de memória semântica, consultas a banco, execução de agentes internos) como tools in-process; o padrão "erros viram `is_error`" garante que o modelo leia a falha e se recupere.

### C6. Modelo de sessão plugável (`SessionStore` Protocol) + resume materializado
- **O que resolve**: persistir/retomar sessões em qualquer backend (S3, Redis, Postgres) sem o agente entender onde mora o transcript.
- **Como funciona**: `SessionStore` é um `Protocol` duck-typed com `append`/`load` obrigatórios e métodos opcionais detectados em runtime. Chave é `SessionKey(project_key, session_id, subpath)`; `materialize_resume_session` carrega do store e monta um diretório temp no layout de `~/.claude/`.
- **Onde**: `src/claude_agent_sdk/types.py` (1489-1704), `_internal/{session_resume,session_store}.py`.
- **Aplicação no Cosca**: o memory/session store do Cosca como `Protocol` — memorizar contexto por `project_key` (tenant), retomar sessões de um backend, orquestrar multi-agente por `subpath`. Duck-typing permite adapters novos sem recompilar.

### C7. Batcher de mirror com decoupling da hot path e falhas não-fatais
- **O que resolve**: espelhar o transcript para um store externo sem adicionar latência ao streaming, garantindo durabilidade local e reportando falhas sem derrubar a sessão.
- **Como funciona**: o read loop "descasca" frames `transcript_mirror` e chama `TranscriptMirrorBatcher.enqueue`, que acumula em buffer (500 entries/1MiB) e faz `flush()` no `result`. `_drain()` desanexa o buffer antes do lock, coalesce por `file_path`, retry 3x com backoff mas **não retry em timeout**; após esgotar, descarta e emite `MirrorErrorMessage` não-fatal.
- **Onde**: `src/claude_agent_sdk/_internal/transcript_mirror_batcher.py`.
- **Aplicação no Cosca**: persistir logs/eventos/memória de cada turno fora da hot path de streaming, com buffer coalescido por unidade de trabalho e "florir antes de entregar o resultado". O contrato "Nunca levanta — falha vira notificação" é o padrão para toda persistência auxiliar.

---

## Synthesis — o que o Cosca deveria copiar

| # | Padrão | Ganho |
|---|--------|-------|
| 1 | A1 — Contrato de hooks por eventos + exit-code | estender o loop sem tocar no núcleo |
| 2 | A2 — Permissões 3 estados + managed-settings | segurança imune a override |
| 3 | A7 — Trust-model aditivo anti-prompt-injection | plugins/regras não suprimem proteções |
| 4 | B2/B3 — Progressive disclosure + roteador por domínio | skills escaláveis sem inchar contexto |
| 5 | B7 — Meta-loop A/B de avaliação | justificar skill por dados, não opinião |
| 6 | C2 — Full-duplex multiplexado com request_id | subagentes/tools dentro de um app |

## Related Patterns

- [`google-agent-patterns.md`](google-agent-patterns.md) — a outra spec de skills (Google)
- [`deepseek-harness-patterns.md`](deepseek-harness-patterns.md) — subagent/tool-approval (D3/D4)
- [`aws-agent-toolkit-patterns.md`](aws-agent-toolkit-patterns.md) — controls-in-code / credencial na borda
- [`hermes-self-evolution-patterns.md`](hermes-self-evolution-patterns.md) — meta-loop de avaliação (paralelo ao B7)
