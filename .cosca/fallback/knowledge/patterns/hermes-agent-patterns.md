# Hermes Agent Patterns — Self-Evolution, Memory, Trajectory

> **Version**: 1.0.0 | **Confidence**: 0.93 | **Category**: Enterprise Platform Patterns | **Created**: 2026-08-13 | **Source**: https://github.com/NousResearch (hermes-agent + hermes-agent-self-evolution, MIT)

> **Mined by**: cosca-kernel. Extraído de `hermes-agent-self-evolution/` (PLAN.md, README.md) e `hermes-agent/` (hermes_state*.py, trajectory_compressor.py). O "agente que cresce com você" (230k stars) — a referência canônica de **self-improvement real** (otimizar, não só registrar).

## Purpose

O Hermes Agent resolve o problema que o Cosca ainda não resolveu: **self-improvement como OTIMIZAÇÃO, não só REGISTRO**. O Cosca registra learnings (stages 7-8); o Hermes EVOLUI o texto (skills/prompts/código) via DSPy+GEPA, medindo melhoria com eval. Este é o gap nº1 identificado na comparação Cosca vs Hermes.

---

## A. Self-Evolution (DSPy + GEPA) — o ouro

### A1. GEPA — evolução reflexiva de prompts (lê o PORQUÊ da falha)
- **O que resolve**: otimizar texto (prompt/skill) programaticamente, entendendo a CAUSA da falha, não só o fato dela.
- **Como funciona**: GEPA (Genetic-Pareto Prompt Evolution) lê **execution traces** (trajetórias) para entender *por que* algo falha, e propõe **mutações direcionadas** — não mutação cega. Funciona com 3 exemplos. Integrado ao DSPy. Supera RL e otimizadores anteriores. Sem GPU — opera via API (mutar string → avaliar → selecionar), ~$2-10 por run.
- **Onde**: `hermes-agent-self-evolution/README.md`, PLAN.md (linhas 15, 62-66).
- **Cosca**: o Cosca tem `learnings.md` (registra) mas NÃO otimiza o texto das skills/prompts. Aplicar GEPA-equivalente: tratar cada SKILL.md/prompt de agente como módulo otimizável, com fitness = desempenho no `cosca eval` (oráculo que já construímos). O oráculo (L242) É o fitness.

### A2. Skill-as-DSPy-Module (texto vira módulo otimizável)
- **O que resolve**: tornar um artefato de texto (skill/prompt/descrição) um objeto que o otimizador pode mutar e avaliar.
- **Como funciona**: wrap o texto (SKILL.md) como módulo DSPy: injeta o texto como system prompt, roda o agente numa task de teste, retorna o resultado para scoring. O texto é a única variável mutável.
- **Onde**: PLAN.md (linhas 66-70, 294-299).
- **Cosca**: cada skill/prompt do Cosca poderia ser embrulhado do mesmo jeito — o `cosca eval oracle` já fornece o mecanismo de scoring.

### A3. Tier de otimização por risco (skills → tools → prompt → código → loop)
- **O que resolve**: ordem de evolução que respeita risco×valor.
- **Como funciona**: Tier 1 = skills (maior valor, menor risco — texto puro); Tier 2 = descrições de tools; Tier 3 = seções do system prompt; Tier 4 = código (maior risco, Darwinian Evolver); Tier 5 = loop contínuo. Cada fase tem um **validation gate** ("melhorou de verdade? sem regressão?") antes da próxima.
- **Onde**: PLAN.md (linhas 21-48, 222-258).
- **Cosca**: mapear para skills → descrições de tools (bindings) → prompt do kernel → código da esteira → loop contínuo.

### A4. Benchmarks como GATES, não fitness (a lição mais profunda)
- **O que resolve**: separar "melhorou a tarefa-alvo" de "não quebrou o resto".
- **Como funciona**: a **fitness** é task-específica (a skill fez seu trabalho melhor?); os **benchmarks são GATES** (TBLite/YC-Bench) que garantem NÃO regressão. "Uma variante que melhora a skill 20% mas cai 5% no benchmark é REJEITADA." Filtro em cascata: pytest (100%) → TBLite rápido → eval task-específica → top-3 → benchmark completo.
- **Onde**: PLAN.md (linhas 630-663).
- **Cosca**: o Cosca tem o eval oracle (fitness) mas NÃO tem o gate de regressão amplo. Separar fitness (caso específico) de gate (regressão geral) é o padrão que falta.

### A5. Guardrails de evolução (5 barreiras obrigatórias)
- **O que resolve**: evolução em produção sem quebrar nada.
- **Como funciona**: (1) suite de testes completa 100% (zero tolerância); (2) limites de tamanho (skills ≤15KB, descrições ≤500 chars — penalidade de comprimento na fitness); (3) compatibilidade de caching (nada muda mid-conversation, só em sessão nova); (4) preservação semântica (evoluiu não pode derivar do propósito); (5) **deploy via PR, nunca commit direto** (review humano).
- **Onde**: PLAN.md (linhas 666-727).
- **Cosca**: o Cosca já tem integridade (family chain) e review (cosca-review), mas não tem o "limite de tamanho + preservação semântica + caching-compat" para evolução.

### A6. Fontes de dataset de eval (sintético + SessionDB + golden + auto-eval)
- **O que resolve**: criar dataset de avaliação sem esforço manual massivo.
- **Como funciona**: (A) **sintético** — modelo forte gera (task, comportamento-esperado) a partir da skill; (B) **SessionDB mining** — minera uso real, LLM-as-judge pontua cada (task, resposta) numa rubrica; (C) **golden sets** — curados à mão para skills críticas; (D) **auto-eval** — onde há verificador natural (ex.: plantou bug, checa se o agente achou). Rubrica não é texto exato ("deve achar o SQL injection na linha 42"), não "output idêntico".
- **Onde**: PLAN.md (linhas 302-327).
- **Cosca**: o Cosca já tem o eval oracle + canary; a fonte "SessionDB mining + LLM-as-judge" (minerar o próprio histórico de uso para gerar dataset) é o que falta — o Cosca TEM o histórico (learnings/knowledge.db), só não o usa como dataset de eval.

### A7. Auto-triage + loop contínuo (detectar → otimizar → PR, sem intervenção)
- **O que resolve**: automatizar a DETECÇÃO do que otimizar, não só a otimização.
- **Como funciona**: performance monitor (taxa de sucesso por skill, acurácia de seleção de tool, scores ao longo do tempo) → auto-triage (rank por impacto × frequência) → threshold-triggered optimization (falha > X% → dispara GEPA) → PR para review humano. **Automatiza detecção + otimização, NÃO deploy.**
- **Onde**: PLAN.md (linhas 586-627).
- **Cosca**: o Cosca tem o CMITracker (métricas) mas não o auto-triage. Conectar CMITracker → rank de skills por taxa de falha → trigger de otimização.

### A8. "Operates ON, not inside" (evolução desacoplada)
- **O que resolve**: evoluir o agente sem tocar no agente.
- **Como funciona**: o self-evolution vive num repo SEPARADO, lê o hermes-agent (read-only), escreve variantes em branches git, abre PRs. Zero mudanças no repo do agente.
- **Onde**: PLAN.md (linhas 135-136, 206-218).
- **Cosca**: o padrão "Neural Link desacoplado" (L192) É isso — confirmou a decisão. A evolução do Cosca deveria ser outro módulo desacoplado que abre PRs contra o kernel.

---

## B. Memória & Estado (o "cresce com você")

### B1. SessionDB — SQLite + FTS5 + WAL + sessão comprimida em cadeia
- **O que resolve**: armazenamento persistente de sessões com busca full-text e compressão.
- **Como funciona**: SQLite com WAL (leitores concorrentes + 1 escritor — gateway multi-plataforma), tabela virtual FTS5 para busca rápida em todas as mensagens, **sessão split por compressão** via cadeia `parent_session_id` (sessão longa → comprime → filha continua). Source tagging ('cli'/'telegram'/'discord') para filtrar.
- **Onde**: `hermes_state.py` (docstring), `hermes_state_schema.py`.
- **Cosca**: o Cosca tem knowledge.db (FTS5+vetor+grafo) mas NÃO tem: compressão de sessão em cadeia (parent_id), nem source tagging. O `internal/ledger` (WAL+MVCC) é complementar.

### B2. Memory nudges (auto-persistência periódica)
- **O que resolve**: o agente NÃO esquece de memorizar (o problema do "aprendeu mas não registrou").
- **Como funciona**: "agent-curated memory with periodic nudges" — o agente é cutucado periodicamente para persistir conhecimento (o mecanismo de nudge, não só confiar que o LLM lembra).
- **Onde**: README.md (linha "A closed learning loop").
- **Cosca**: o Cosca tem auto-evolution stages 7-8 (registra), mas depende do LLM LEMBRAR de registrar. O nudge (gatilho periódico de auto-persistência) é o que falta para fechar o loop.

### B3. Honcho — user modeling dialético
- **O que resolve**: construir um modelo do usuário (quem é, preferências) ao longo de sessões.
- **Como funciona**: modelagem "dialética" do usuário — o agente constrói um perfil que se refina via interação.
- **Onde**: README.md.
- **Cosca**: o Cosca não tem user modeling (só memória de tarefa). O "quem é o Don" (preferências, estilo) poderia ser um perfil persistente que se refina.

---

## C. Trajetória & Tooling

### C1. Trajectory compression (proteger pontas, comprimir o meio)
- **O que resolve**: comprimir trajetórias para treinar a próxima geração sem perder sinal.
- **Como funciona**: (1) protege as PRIMEIRAS turns (system, human, 1º gpt, 1º tool); (2) protege as ÚLTIMAS N turns (ações/conclusões finais); (3) comprime SÓ o meio (a partir do 2º tool response); (4) comprime só o necessário para caber no orçamento; (5) substitui a região comprimida por UM resumo; (6) mantém tool calls intactas (o modelo continua após o resumo).
- **Onde**: `trajectory_compressor.py` (docstring).
- **Cosca**: o Cosca tem condensation (internal/pipeline/condensation.go) mas o padrão "proteger pontas + comprimir meio + resumo único" é mais preciso — preserva o sinal de início/fim.

### C2. Toolsets + backends de terminal + gateway (deploy/interface)
- **O que resolve**: rodar em qualquer lugar e falar em qualquer plataforma.
- **Como funciona**: toolsets (40+ tools agrupadas por domínio, distribuição por contexto); 7 backends de terminal (local/Docker/SSH/Singularity/Modal/Daytona/Vercel) com hibernação serverless (custo ~zero quando idle); gateway único → 6 plataformas (Telegram/Discord/Slack/WhatsApp/Signal/Email) com continuidade cross-platform.
- **Onde**: README.md, `toolsets.py`, `gateway/`.
- **Cosca**: o Cosca tem serve/runtime (local) + desktop Wails. Os gaps: múltiplos backends de deploy com hibernação, e gateway de mensageria. O padrão "toolset por domínio + distribuição" mapeia para o roteamento de skills do Cosca.

---

## D. Deploy, Execução, Hibernação & Self-Improvement (ângulos novos — 2026-08-22)

### D1. Execution-Environment Abstraction (sandbox plugável local/Docker/SSH/Modal/Daytona/Vercel)
- **O que resolve**: executar código/terminal do agente em qualquer backend sem tocar no loop do agente e sem duplicar lógica de IO/estado, degradando graciosamente quando o backend está fora do ar.
- **Como funciona**: um `BaseEnvironment` (ABC) impõe *spawn-per-call*: cada comando sobe um `bash -c` fresco; snapshot de sessão capturado no `init` e re-source antes de cada comando; CWD preservado via marcadores in-band no stdout (remoto) ou arquivo temporário (local). `TERMINAL_ENV` seleciona o backend; `tools/env_probe.py` detecta qual capacidade está realmente disponível. `credential_files.py` faz sync/bind-mount de credenciais por tipo de backend. Falhas de infra viram `EnvironmentConnectionError` → resultado `status:"degraded"` com `retry_hint`, nunca um traceback; backend falho nunca é cacheado.
- **Onde**: `tools/environments/{base,local,docker,ssh,modal,daytona,vercel_sandbox}.py`, `tools/env_probe.py`.
- **Cosca**: um `CoscaExecutionEnvironment` por backend (local/Docker/SSH), habilitando "mesmo agente, sandbox diferente". Reaproveite o par check_fn passivo vs ensure_deps_fn ativo (D4) e o `EnvironmentConnectionError`→degraded como contrato de resiliência.

### D2. Scale-to-Zero com auto-suspend via socket Flaps (hibernação serverless)
- **O que resolve**: a economia de custo do serverless sem o race condition do autostop do proxy, que não enxerga tráfego outbound-only e suspenderia a máquina no meio de um job.
- **Como funciona**: o gateway **possui** a decisão de idle (não o proxy). `is_idle()` compõe três conjuncts: zero trabalho ativo *agregado* (turns de agente + cron + runs), sem inbound no timeout, e sem background work vivo — quem não consegue ler uma fonte de trabalho deve falhar AWAKE (sentinel positivo), nunca pra 0. Ao esvaziar, roda `go_dormant()` (fecha socket, preserva supervisor — nunca stop/drain) e então **suspende a própria máquina** via POST ao unix-socket local (`/v1/apps/{app}/machines/{id}/suspend`) — o socket é a credencial. Sempre *fail-awake, nunca fail-frozen*.
- **Onde**: `gateway/scale_to_zero.py`.
- **Cosca**: backbone da hibernação serverless. Padrão-chave: inverta o "quem decide idle" — o runtime deve receber um sinal de inatividade que não depende de tráfego de entrada. Encapsule o suspend numa interface `SelfSuspendBackend`.

### D3. Gateway Split-Edge (connector/relay com capacidade vault e buffered flip)
- **O que resolve**: rodar um gateway "hosted" **sem porta pública de entrada**, sem vazar secrets da plataforma, e sem perder eventos quando o gateway hiberna a zero.
- **Como funciona**: o gateway diala **para fora** até um connector na edge. A edge responde o ACK do provider, **remove** os tokens da plataforma e os liga a um *capability vault* da sessão — nunca chegam ao gateway (`send_follow_up` emite ação **semântica** por `session_key`, sem nomear/tocar token). O transporte oferece `go_idle()` (*buffered flip*): envia `going_idle` e aguarda o ack confirmando que o inbound agora bufferiza duravelmente para replay no reconnect — é o que torna o scale-to-zero seguro.
- **Onde**: `gateway/relay/{transport,ws_transport,adapter,descriptor}.py`, `docs/relay-connector-contract.md`.
- **Cosca**: arquitetura ideal de gateway multi-plataforma quando o deploy é remoto/serverless: separe "edge autenticada" do "núcleo do agente", mantenha tokens no tenant certo e garanta que escala-zero não perca mensagens.

### D4. Platform Registry → toolset `hermes-<platform>` auto-derivado + split passivo/ativo de dependências
- **O que resolve**: adicionar plataforma nova sem if/elif e sem que um status-display dispare `pip install` (boot-loop) nem que o connect fique preso (deadlock).
- **Como funciona**: `PlatformEntry` com `adapter_factory`, `validate_config`, e **dois** probes separados: `check_fn` PASSIVO (sem efeito colateral, usado em status/setup/readiness) vs `ensure_deps_fn` ATIVO (instala via pip/lazy_deps no momento exato de `create_adapter()`). Um toolset `hermes-<platform>` ausente é **auto-gerado** durante `resolve_toolset()`: core + tools que o registry registrou naquele nome de plataforma. `bundle_non_core_tools()` subtrai só o delta. `resolve_toolset()` memoiza por chave `(nome, registry_id, generation)` e invalida só quando o registry muda, com teto de 256 entradas.
- **Onde**: `gateway/platform_registry.py`, `toolsets.py`, `gateway/platforms/*.py`.
- **Cosca**: modelo de extensão de plataforma e de toolset: registry de adapters + derivação automática de toolset por plataforma. Reproduza o split passivo/ativo (o mesmo bug de boot-loop/deadlock existe em qualquer sistema com "auto-install").

### D5. Distribuição de contexto: orçamentação por categoria + UI de atribuição
- **O que resolve**: dar visibilidade de *onde* a janela está indo (system prompt tiers, tool schemas, regras, skills, MCP, subagentes, memória, conversa) para decidir compressão/melhoria.
- **Como funciona**: `compute_session_context_breakdown()` recompõe o system prompt em tiers (`stable`/`context`/`volatile`) e atribui tokens por categoria usando a **mesma** heurística char/4 de `estimate_request_tokens_rough` (para alinhar com thresholds de compressão). Slots renderizados como grid de glifos no CLI e tabela no gateway. `/context all` faz atribuição **por unidade**. O dado real (`last_prompt_tokens`) tem prioridade sobre a estimativa quando disponível.
- **Onde**: `agent/context_breakdown.py`, `agent/model_metadata.py`, `hermes_cli/prompt_size.py`.
- **Cosca**: ferramenta de "distribuição de contexto" — um `/context`-like que mostra o orçamento por toolset/skill para otimizar quais bundles entram no prompt. Alinhar a estimativa com os thresholds de compressão evita "contas diferentes" entre UI e compressor.

### D6. Self-improvement loop: pós-turn review em fork + `/learn` guiado por padrões
- **O que resolve**: fazer o agente melhorar a si mesmo (gravar memória/skills) sem corromper a sessão ativa, o prompt cache nem o custo da conversa.
- **Como funciona**: após cada turno, `spawn_background_review()` sobe um **fork em daemon** de um `AIAgent` que re-planeja o snapshot da conversa e se pergunta "devo salvar/atualizar algum skill/memória?". O fork **herda o runtime vivo** (provider, model, credenciais, prompt cache) → bate no mesmo prefix cache, mas roda com **whitelist de tools limitada a memória + skill** (todo o resto negado). Escritas vão direto aos stores; main loop e prompt cache intocados. `/learn` é o caminho insumo→skill com regras HARDLINE de authoring.
- **Onde**: `agent/background_review.py`, `agent/learn_prompt.py`, `agent/learning_graph.py`.
- **Cosca**: o loop de self-improvement *real*: um "revisor de fundo" isolado por perfil/sessão que feeda memória/skills, mais um `/learn` que transforma descrição do usuário em ativo reaproveitável sob regras de casa.

### D7. Memória dual-store + provider único ativo + snapshot congelado (user modeling)
- **O que resolve**: modelar o usuário e persistir conhecimento entre sessões de forma determinística e estável, sem inchar o schema de tools nem invalidar o prefix cache.
- **Como funciona**: dois stores em texto: `MEMORY.md` (fatos do agente) e `USER.md` (perfil do usuário: preferências, estilo, expectativas) — *user modeling*. Snapshot injetado **congelado** no system prompt no início da sessão; escritas no meio persistem em disco imediatamente (duráveis) mas **não alteram** o prompt atual (preserva o prefix cache); só refresca na próxima sessão. Entries delimitadas por `§`, limites em caracteres. `MemoryManager` é o **ponto único de integração**: permite **exatamente UMA** provider externa ativa (rejeita a segunda), ciclo `build_system_prompt / prefetch_all / sync_all`. `normalize_tool_schema` desembrulha um tool dict duplo-embrulhado (DeepSeek HTTP 400 derruba o toolset inteiro). Discovery roda **precedência invertida** (bundled > user > project) para não fazer shadow do provedor.
- **Onde**: `tools/memory_tool.py`, `agent/memory_manager.py`, `plugins/memory/*`.
- **Cosca**: modelo de memória dual (conhecimento do agente vs modelo do usuário), snapshot congelado para estabilidade de cache, e uma Máquina de Memória com "um provider ativo". O `USER.md` é a base direta do user modeling.

---

## Synthesis — o que o Cosca deveria copiar (priorizado)

| # | Padrão | Aplicação no Cosca | Ganho |
|---|--------|-------------------|-------|
| 1 | GEPA (evolução reflexiva) | otimizar skills/prompts via eval oracle | self-improvement REAL (otimiza, não registra) |
| 2 | Benchmarks como GATES | separar fitness (oráculo) de gate (regressão) | evolução sem quebrar nada |
| 3 | Guardrails (5 barreiras) | tamanho + semântica + caching + PR | evolução segura em produção |
| 4 | SessionDB mining → dataset | minerar knowledge.db/learnings → eval dataset | dataset de eval orgânico |
| 5 | Memory nudges | gatilho periódico de auto-persistência | fecha o loop de aprendizado |
| 6 | Auto-triage + loop contínuo | CMITracker → rank por falha → trigger | automatiza DETECÇÃO |
| 7 | "Operates ON, not inside" | evolução como módulo desacoplado | confirma o Neural Link |
| 8 | Trajectory compression | melhorar o condensation | preserva sinal de pontas |
| 9 | SessionDB (FTS5+compressão+tagging) | melhorar o knowledge.db | busca + compressão de sessão |

## Known Uses (referência)

- `NousResearch/hermes-agent` (230k stars) + `hermes-agent-self-evolution` (DSPy+GEPA) — o agente self-improving em produção.

## Related Patterns

- [`cosca-product-pattern.md`](cosca-product-pattern.md) — o padrão de produto (desacoplamento)
- [`kubernetes-core-patterns.md`](kubernetes-core-patterns.md) — workqueue/reconciler (o fitness/loop)
- [`vscode-go-extension-patterns.md`](vscode-go-extension-patterns.md) — lifecycle de processo (os backends/gateway)
- `internal/evals/ORACLE_SPEC.md` — o oráculo que serve de fitness
- `internal/evals/ABLATION_PROTOCOL.md` — a medição de metacognição
