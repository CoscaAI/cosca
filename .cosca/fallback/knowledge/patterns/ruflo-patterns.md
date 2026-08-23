# Ruflo Patterns — Swarm, Memória Self-Learning, Federação, Meta-Harness, Capability Inventory

> **Version**: 1.0.0 | **Confidence**: 0.90 | **Category**: Enterprise Platform Patterns | **Created**: 2026-08-23 | **Source**: https://github.com/ruvnet/ruflo (MIT)

> **Mined by**: cosca-kernel (ordem do Don). Extraído de 5 batedores paralelos sobre o checkout local (v3/src, v3/@claude-flow/*, plugins/*, docs/, verification/). O **meta-harness de agentes** — o axioma "Agent = Model + Harness": o modelo escreve, o harness dá ferramentas/memória/loops/sandboxes/controles. Alvo direto dos gaps do Cosca: orquestração, memória durável valorada, federação segura, camada de harness, inventário de capabilities.

## Purpose

O Ruflo resolve o problema de **envolver um harness base (Claude Code/Codex) numa camada de execução que dá ao agente ferramentas, memória auto-aprendida, swarm, federação segura e controles** — e de **medir/evoluir o próprio harness**. O Cosca tem os blocos, mas o Ruflo mostra 5 lições estruturais: (1) **a memória é o único estado durável**, e router/swarm/loop são funções aprendidas sobre ela; (2) **memória imutável e auditável** (invalida, não sobrescreve); (3) **federação zero-trust com handshake + PII transformation**; (4) **harness degradável (removable, graceful)**; (5) **capability inventory data-only + baseline monotônico no CI**.

---

## A. Swarm Orchestration

### A1. Closed-Loop Learned Routing (route → outcome → learn)
- **O que resolve**: decidir quem resolve a task não é one-shot; precisa **aprender** com cada resultado.
- **Como funciona**: `hooks_route` retorna `{recommended, confidence, reasoning}`; agente spawnado no tier recomendado; após completar, `hooks_model-outcome {success:false, reason}` realimenta o router (feedback **mandatório** — "skipping = no learning"). O autopilot grava em `autopilot-patterns`. Gate de promoção: bandit pick vs neural-augmented pick com AND-gate `quality>2% AND cost<1% AND latency<5%`. Todo caminho tem fallback degradado (`degraded:true`).
- **Onde**: `plugins/ruflo-intelligence/skills/intelligence-route/SKILL.md`, `commands/neural.md`, `plugins/ruflo-autopilot/README.md`.
- **Aplicação no Cosca**: conectar escolha de modelo/agente a um `cosca-memory-chief` que grava outcomes, com gate de critérios antes de promover um router de produção.

### A2. Multi-Factor Neural Semantic Matching (confidence gate + alternates + fallback)
- **O que resolve**: rotear task→agente combinando semântica, skills, carga e histórico — com degradação elegante.
- **Como funciona**: `SemanticRouter` pontua `score = skillWeight*skillMatch + semanticWeight*semanticScore + loadWeight*loadFactor + performanceWeight*performance`. Gating por `minConfidence` (0.5), lista de **alternates** (top 3, >80% threshold) para reassign sob carga, `updatePerformance` ajusta média móvel. Sem WASM/embedding → fallback cosine (routing nunca quebra).
- **Onde**: `v3/plugins/teammate-plugin/src/semantic-router.ts`.
- **Aplicação no Cosca**: agente de roteamento `cosca-*` como weighted-scoring sobre embeddings de capabilities, com confidence gate + alternates de backup + fallback determinístico.

### A3. Topology-Driven Swarm + Anti-Drift Defaults
- **O que resolve**: swarms divergem sem estrutura explícita; a topologia é o contrato social (quem é leader, quem comunica com quem).
- **Como funciona**: `SwarmCoordinator.updateConnections` constrói edges por topologia (`mesh`=todos peers, `hierarchical`=workers→leader). Anti-drift defaults: `topology:hierarchical`, `maxAgents:6-8`, `strategy:specialized`, `consensus:raft`, `memory:hybrid`. `scaleAgents` trata count como target absoluto (não delta), escala-down do mais antigo primeiro. Restauração via `reconfigure({topology})`.
- **Onde**: `v3/src/coordination/application/SwarmCoordinator.ts`, `plugins/ruflo-swarm/README.md`.
- **Aplicação no Cosca**: hierarquia `cosca-ceo/cto` como topologia (líder que capta divergência) + workers; fixar limites e roles especializadas para conter drift.

### A4. Load-Balanced Priority + Capability/Dependency Scheduling com Rollback
- **O que resolve**: distribuir N tasks entre M agentes respeitando prioridade, capability e DAG de dependências — revertendo em falha.
- **Como funciona**: `distributeTasks` ordena por prioridade, filtra `agent.canExecute(type) && active`, faz balance grasping (menor carga). `resolveExecutionOrder` faz topological sort (ciclo → erro). Tasks nested workflows. Falha com `rollbackOnFailure` → `rollbackWorkflow` executa `onRollback` em ordem reversa.
- **Onde**: `v3/src/coordination/application/SwarmCoordinator.ts`, `v3/src/task-execution/domain/Task.ts`, `.../WorkflowEngine.ts`.
- **Aplicação no Cosca**: `cosca-workflow-chief` com Gantt de prioridade + DAG + load-balance por carga, com compensação reversa em workflows multi-agente.

### A5. Graceful Task/Agent State Machine + Structured Failure Contract
- **O que resolve**: agentes/redes falham; a orquestração precisa de falha estruturada, não exceções que derrubam o swarm.
- **Como funciona**: `Task` é máquina de estados (`pending→in-progress→completed/failed/cancelled`) com callbacks `onExecute`/`onRollback` injetados. `Agent.executeTask` retorna sempre `TaskResult{status:'failed', error}`; todo `await agent.executeTask()` é envolvido em try/catch para converter crash em `status:'failed'`. `Agent.status` (active/busy/idle/terminated) é gate de admissão.
- **Onde**: `v3/src/task-execution/domain/Task.ts`, `v3/src/agent-lifecycle/domain/Agent.ts`.
- **Aplicação no Cosca**: todo `cosca-*` expõe `executeTask→TaskResult{status,error}`, nunca lança para o kernel; máquina de estados no Task como único contrato de ciclo de vida.

### A6. Cache-Aware Autonomous Loop Heartbeat (270s wake + Monitor stream)
- **O que resolve**: swarm agindo por horas sem input, sem cache-miss de prompt e sem polling cego.
- **Como funciona**: loop dirigido por `/loop` + `ScheduleWakeup` com **heartbeat de 270s** (abaixo do TTL de 5min do prompt cache). Wake primário é `Monitor` (stream NDJSON de eventos); 270s é rede de segurança. Cada iteração lê estado de `AgentDB`/namespace próprio — a **memória é o registro durável** que o loop reconsidera. `HybridBackend` combina SQLite (estruturado) + AgentDB (HNSW vector, filtro SQL + kNN).
- **Onde**: `plugins/ruflo-autopilot/README.md`, `v3/src/memory/infrastructure/HybridBackend.ts`.
- **Aplicação no Cosca**: unir loop + memória como único estado durável; heartbeat < TTL do provider + Monitor como sinal de evento, com fallback agendado.

### A7. Declarative Skills/Agents como Superfície de Orquestração + Smoke-as-Contract
- **O que resolve**: expor swarm/tools de forma que agentes escolham sem ler fonte; garantir que o contrato não regride.
- **Como funciona**: orquestração exposta por `SKILL.md` (com `description` + `allowed-tools` sem wildcard + `argument-hint`) e `agents/*.md` (roles com tool-routing matrix). Cada plugin declara a superfície MCP. Contrato vira teste (`scripts/smoke.sh`, gate de CI). Namespaces de memória **reservados** (`pattern`, `claude-memories`, `default`) e cada plugin afirma ownership.
- **Onde**: `plugins/ruflo-swarm/README.md`, `scripts/smoke.sh`, `plugins/ruflo-intelligence/docs/adrs/0001-...`.
- **Aplicação no Cosca**: agentes `cosca-*` como arquivos declarativos (descrição+tools+rolê), cada um afirmando namespace de memória; gerar smoke/contrato no CI para travar a superfície.

---

## B. Memória Self-Learning + AgentDB

### B1. Memória hierárquica com validade temporal — "invalida, não sobrescreve"
- **O que resolve**: overwrite de memória apagando o histórico; mentir que sucesso quando a escrita não persistiu.
- **Como funciona**: `TieredMemoryStore` (tiers `working|episodic|semantic`) modela fatos com `validFrom/validUntil` + `supersedes`. Fato novo que contradiz antigo **invalida** o outro (`validUntil=now`, `supersededBy=<novo>`, archive FIFO) em vez de apagar — histórico consultável via `recall(..., {includeExpired:true})`. `isDurable()===false` se store volátil. `store()` persiste **antes** de publicar no map (falha de I/O não deixa valor visível).
- **Onde**: `v3/@claude-flow/memory/src/tiered-memory.ts`.
- **Aplicação no Cosca**: memória de agente **imutável e auditável**, não UPDATE destrutivo. Plug no `cosca-semantic-memory` (rejeitar overwrite de embeddings conflitantes) + `cosca-runtime` (health check real de persistência).

### B2. SmartRetrieval — pipeline de 5 fases sobre HNSW (expansão→RRF→recency→MMR→round-robin)
- **O que resolve**: recall de alta qualidade entre sessões — busca vetorial crua devolve o próximo, não o mais útil.
- **Como funciona**: `smartSearch()`: (1) expansão de query sem LLM (≤3 variantes); (2) fan-out multi-variante + **RRF** (`Σ 1/(k+rank)`, k=60); (3) recency boost com decaimento exponencial (half-life 30d, máx 0.2); (4) **MMR** rerank com proxy token-Jaccard (λ=0.7); (5) round-robin por `session_id`. Cada fase é toggle, devolve `SmartSearchStats` (auditável).
- **Onde**: `v3/@claude-flow/memory/src/smart-retrieval.ts`, `plugins/ruflo-rag-memory/README.md`.
- **Aplicação no Cosca**: blueprint do `cosca-semantic-memory` — o fator `sessionDiversity` + recency separa "banco de embedding" de "memória de agente".

### B3. Hybrid search de 3 braços com proveniência de sinal (dense + sparse + entity)
- **O que resolve**: nenhum arm recupera tudo; fusão com rastreabilidade.
- **Como funciona**: `hybridSearch` roda em paralelo com `.catch` per-arm (falha não zera os outros): `semanticSearch` (dense), `searchKeyword` (FTS5 sparse), braço de entidades. Funde com `applyRRF` + `applyMMR`. Campo `signals: ('vector'|'bm25'|'entity')[]` diz **quais braços** sinalizaram — proveniência para debug/re-rankers.
- **Onde**: `v3/@claude-flow/memory/src/controller-registry.ts`, `smart-retrieval.ts`.
- **Aplicação no Cosca**: `cosca-semantic-memory` + RAG corporativo. O `signals[]` desambiguar "mesmo termo, contexto diferente" e alimentar `cosca-critic` com justificativa de recuperação.

### B4. Memória com reforço por confiança (LearningBridge) — boost, decay, EWC
- **O que resolve**: o agente evoluir do histórico sem virar saco de fatos sem peso.
- **Como funciona**: `LearningBridge` conecta cada insight a uma trajetória (`beginTask→recordStep→completeTask`). Confiança sobe `+0.03` a cada acesso, decai `-0.005/hora` após >1h sem uso, piso 0.1. `consolidate()` só dispara com ≥10 trajetórias ativas. EWC (`ewcLambda=2000`) protege contra esquecimento catastrófico.
- **Onde**: `v3/@claude-flow/memory/src/learning-bridge.ts`, `auto-memory-bridge.ts`.
- **Aplicação no Cosca**: o coração do "self-optimizing" — `cosca-memory-chief` com boost/decay (caminho pavimentado) e EWC (consolidação sem esquecimento).

### B5. SONA auto-aprendizado: pattern mining + EMA + pruning + persistência RVF
- **O que resolve**: transformar trajetórias em padrões reutilizáveis — o loop de aprendizado concretizado.
- **Como funciona**: `PersistentSonaCoordinator` acumula trajetórias (máx 1000); `runBackgroundLoop()` minera só `success|partial`; para cada passo > `patternThreshold` (0.85), gera embedding e só cria padrão se não achar similar (dedup). Padrão carrega `successRate` com **EMA (α=0.1)** + `useCount`; `prunePatterns()` remove baixa performance (useCount≥5, successRate<0.3). Persistência RVF, auto-persist 30s.
- **Onde**: `v3/@claude-flow/memory/src/persistent-sona.ts`, `rvf-learning-store.ts`.
- **Aplicação no Cosca**: o trio **mineração→dedup→pruning** com EMA é como uma biblioteca de padrões de agente deve se comportar. Gotcha: `pattern`/`patterns` (namespaces distintos).

### B6. Registry de controllers com degradação graciosa + fallback-first routing
- **O que resolve**: 15+ subsistemas de memória; se um backend faltar, o harness não morre nem mente.
- **Como funciona**: `ControllerRegistry` inicializa 29 controllers por `INIT_LEVELS` (0-6) com `Promise.allSettled` por nível; `closePriorIfAny` antes de substituir. Fellbacks são **contratuais** (`agentdb_pattern-store` sem registry → `controller:'memory-store-fallback'`). Stubs retornam `source:'stub', note:'...não rodou'` em vez de `{promoted:0}`. Convenção de namespace `<plugin-stem>-<intent>`, reservados (`pattern`, `claude-memories`, `default`).
- **Onde**: `v3/@claude-flow/memory/src/controller-registry.ts`, ADR-0001 ruflo-agentdb.
- **Aplicação no Cosca**: melhor padrão para a camada de plugins de memória do `cosca-memory-chief` — init em níveis, registry como fonte de verdade, fallback é resultado válido (não soft-failure), disciplina de namespace.

### B7. Ponte bidirecional de memória (legível ⇄ vetorial) + memória escopada por agente
- **O que resolve**: dois mundos — Claude/markdown legível vs embeddings — sincronizados entre sessões, memória isolada por agente.
- **Como funciona**: `AutoMemoryBridge` sync bidirecional: `SessionStart` importa `~/.claude/projects/*/memory/*.md` → AgentDB (384-dim, namespace `claude-memories`); `SessionEnd` devolve insights ao `MEMORY.md`. Cada insight tem `category` + `confidence`; MEMORY.md podado por `confidence-weighted|fifo|lru`. O `agent-memory-scope.ts` adiciona **3 escopos por agente** (`project|local|user`), sanitização anti path-traversal, `transferKnowledge()` (filtra por minConfidence + category).
- **Onde**: `v3/@claude-flow/memory/src/auto-memory-bridge.ts`, `agent-memory-scope.ts`.
- **Aplicação no Cosca**: memória com **face legível** (ADRs/markdown) sincronizada com a face vetorial (inspecionável); escopo triplo project/local/user como isolamento anti-vazamento de contexto.

---

## C. Federação (comunicação segura entre máquinas)

### C1. Trust-Graded Agent-Card Gateway (A2A Agent Card)
- **O que resolve**: uma máquina expor seus agentes para outra sem inventar protocolo, com bind inseguro bloqueado por padrão.
- **Como funciona**: máquina publica `GET /.well-known/agent-card.json` (spec A2A 1.0) com `A2AAgentCard`; identity nativa (nodeId, publicKey, signature, complianceModes) viaja em extension `urn:ruflo:federation:manifest:v1`. Do lado consumidor, `fromAgentCard()` converte em `FederationNode` **sempre em `UNTRUSTED`**. Defesa: bind `127.0.0.1` default, recusa non-loopback sem `allowNonLoopback:true`, endpoint read-only, ETag/304.
- **Onde**: `v3/@claude-flow/plugin-agent-federation/src/a2a/well-known.ts`, `a2a/agent-card.ts`.
- **Aplicação no Cosca**: gateway de agentes cross-machine — expor endpoint discoverable, tratar todo peer como não-confiável até provar identidade.

### C2. Signed Manifest + Multi-Backend Peer Discovery
- **O que resolve**: descoberta de peers em máquinas distintas com identidade verificável e expurgo de mortos.
- **Como funciona**: nó publica `FederationManifest` (nodeId, publicKey ed25519, endpoint, capabilities, signature). Descoberta: `static`, `dns-sd`, `ipfs-registry`, `a2a-card`. `addStaticPeer` valida assinatura do manifest (endpoint deve bater); `registerExternalPeer` refresca sem substituir (preserva trust). `pruneStale()` remove os que não responderam.
- **Onde**: `src/domain/services/discovery-service.ts`, `domain/value-objects/wg-config.ts`.
- **Aplicação no Cosca**: descoberta de peers multi-máquina com assinatura de identidade; registry assinado + TTL de staleness.

### C3. Capability-Negotiated Challenge-Response Handshake
- **O que resolve**: autenticar um peer (demonstra possuir a chave) e estabelecer sessão com escopo/prazo.
- **Como funciona**: `initiateHandshake` emite challenge com `nonce` + timeout (10s); peer responde assinando. `verifyChallenge` checa validade + assinatura, calcula **interseção de capabilities** e cria `FederationSession` (sessionToken, TTL 1h, maxSessionTtl 24h, heartbeat 30s, trust level). Manifest assinado provoca só `VERIFIED` (nunca ATTESTED). Re-renovação respeita maxSessionTtl.
- **Onde**: `src/domain/services/handshake-service.ts`, `domain/entities/federation-session.ts`.
- **Aplicação no Cosca**: handshake criptográfico com escopo negociado e TTL — "quem pode o quê" por sessão, não por identidade global.

### C4. JCS-Canonical Signed Envelope + Signature-Version Negotiation
- **O que resolve**: transporte autenticado anti-replay e anti-deserialização — payload assinado sobre bytes canônicos idênticos.
- **Como funciona**: `FederationEnvelope` (sourceNodeId, targetNodeId, sessionId, payload, nonce, hmacSignature, timestamp, piiScanResult). Série canônica RFC-8785/JCS (`canonicalizeJcsValue`: ordena chaves, rejeita -0/bigint/sparse/ciclos). Inbound reconstrói os **mesmos bytes** e verifica com `@noble/ed25519`. Negociação `legacy-v1` vs `jcs-v1`; `prefer-jcs` default, `require-jcs` fail-closed. Replays barrados por nonce + isExpired.
- **Onde**: `src/domain/entities/federation-envelope.ts`, `application/inbound-dispatcher.ts`.
- **Aplicação no Cosca**: canonicalização JCS + negociação de versão é o guarda-corpo contra "dados que mudam de shape". Use em qualquer integração verificada por chave pública.

### C5. Trust-Graded PII Data-Flow Control (transformação em vez de isolamento)
- **O que resolve**: "não vazar dados" controlado por transformação do dado ao atravessar trust boundary.
- **Como funciona**: `PIIPipelineService` detecta 14 tipos (email, ssn, credit_card, api_key, private_key, github_token...). Para cada, matriz de política por trust-level (`block`/`redact`/`hash`/`pass`). `block` falha a mensagem; `redact` substitui por `[REDACTED:{type}]`; `hash` por `[HASH:{type}:...]` com salt. Confiança adaptativa recalibra por false-positives.
- **Onde**: `src/domain/services/pii-pipeline-service.ts`, `application/policy-engine.ts`.
- **Aplicação no Cosca**: tratar dado como vazável até o peer provar trust, e transformá-lo no limite — o mecanismo certo para `integrations`/data-sharing.

### C6. Zero-Trust Trust Ladder + Behavioral Scoring + Fail-Closed Autorização (PEP/PDP)
- **O que resolve**: "quem tem o quê" em 3 gates ortogonais (confiança, claims, modo) com default-deny.
- **Como funciona**: 5 níveis UNTRUSTED→VERIFIED→ATTESTED→TRUSTED→PRIVILEGED, cada um com `CAPABILITY_GATES` acumulativos. Score = `0.4*successRate + 0.2*uptime + 0.2*(1-threat) + 0.2*data_integrity`. Upgrade exige score + minInteractions (50/500/5000); **PRIVILEGED exige hasInstitutionalAttestation + requiresHumanApproval**. Downgrade imediato. PDP/PEP com modo `observe`/`enforce`; enforce fail-closed.
- **Onde**: `src/domain/entities/trust-level.ts`, `application/trust-evaluator.ts`, `application/policy-engine.ts`.
- **Aplicação no Cosca**: blueprint de autorização do kernel — trust ladder como estado + PEP/PDP default-deny. "Elevação exige aprovação humana" torna federado seguro para spawn remoto.

### C7. Federated Swarm Circuit Breaker + Peer State Machine
- **O que resolve**: delegacao recursiva A→B→A, cascata de custo, peer hostil, oráculo de orçamento.
- **Como funciona**: (a) envelope de orçamento + hop counter (`maxHops` default 8, `maxTokens`/`maxUsd` com teto; erro é **string constante** — anti-oráculo, sem eco do limite); (b) state machine por peer ACTIVE→SUSPENDED (custo 24h >$5 ou failure-ratio >50% em ≥10 sends)→ACTIVE (cooldown 30min + probe) ou EVICTED; `canTransition` é fonte de verdade; EVICTED só reativa manual. (c) `reportSpend`/`SpendReporter`.
- **Onde**: `src/domain/value-objects/federation-budget.ts`, `application/federation-breaker-service.ts`, `application/federation-coordinator.ts`.
- **Aplicação no Cosca**: resiliência para swarm multi-máquina — ciclo de vida de peer com auto-recuperação, orçamento propagado. O **anti-oráculo** (erro constante) é reutilizável em gateways de billing/quota.

---

## D. Meta-Harness & Hooks

### D1. Agent = Model + Harness (o axioma)
- **O que resolve**: define o que é um agente — o modelo escreve; o harness dá tools/memória/loops/sandboxes/controles para o agente trabalhar.
- **Onde**: `README.md`, `v3/@claude-flow/cli/README.md`.
- **Aplicação no Cosca**: axioma que o kernel deve declarar. Cosca é o harness; os `cosca-*` são "model + harness" — devem chamar MCP, hibernar contexto, rodar loops.

### D2. Harness-on-harness via Removable Augmentation + Graceful Degradation
- **O que resolve**: um harness se encaixar por cima de outro sem se tornar runtime obrigatório.
- **Como funciona**: 4 constraints do ADR-150: (1) **Removable** (sem os pacotes externos, CLI funciona); (2) `optionalDependencies`, nunca `dependencies`; (3) **Graceful degradation** — todo caminho captura `MODULE_NOT_FOUND`/rede e emite `{degraded:true, reason, hint, generatedAt}` e `exit 0`; (4) **CI gate** com `npm install --no-optional` + smoke verde.
- **Onde**: `plugins/ruflo-metaharness/scripts/_harness.mjs`, `_invoke.mjs`, `.harness/README.md`.
- **Aplicação no Cosca**: contratos com agentes/ferramentas externas como **opcionais e degradáveis** — todo adaptador retorna `{degraded, reason}`; teste CI "Cosca roda sem X".

### D3. Hooks como membrana de interceptação/roteamento
- **O que resolve**: o harness reagir automaticamente a todo evento do harness base sem o usuário chamar nada.
- **Como funciona**: `hooks.json` declara matchers por tool (`PreToolUse`, `PostToolUse`, `UserPromptSubmit → route`, `SessionStart`, `Stop`, `PermissionRequest`). A membrana é o `v3HookMapping` (eventos internos V3 → hooks oficiais do Claude Code). Tudo usa **stdin→jq→xargs-0** ou bootstrap `node -e` — **nunca interpolar `$TOOL_INPUT` em string shell** (anti shell-injection #1747).
- **Onde**: `plugin/hooks/hooks.json`, `plugins/ruflo-core/hooks/hooks.json`, `scripts/ruflo-hook.cjs`.
- **Aplicação no Cosca**: interceptar o harness do usuário com matcher por tool + stdin JSON + argv single (nunca interpolação shell); "chiefs" de observabilidade/memória/rotação escutam `PostToolUse`, não poll.

### D4. Skill-as-Tool + Ponte de Subprocesso com pin-version e cache
- **O que resolve**: transformar CLI/binário externo em capacidades invocáveis de forma hermética, versionada, com timeout, sem import de biblioteca.
- **Como funciona**: `SKILL.md` → `scripts/X.mjs` → `_harness.mjs`/`_invoke.mjs` → `spawnSync('node', [binAbs])`. **pin-version** `~0.3.0` nunca `@latest`; resolução em cascata (local node_modules → versão única `~/.ruflo/<pkg>-cache-<pin>`); timeout hard 60s; força `--json` + `parseTrailingJson` (último bloco `{...}`); distinção `-timeout` vs `-not-available`.
- **Onde**: `scripts/_harness.mjs`, `scripts/_invoke.mjs`.
- **Aplicação no Cosca**: camada única tipo `_invoke` (runner de subagente/binário) com pin-version, cache versionado, timeout, contrato unificado — evita "cada plugin faz seu próprio spawn".

### D5. Harness como Objeto Descritível (self-introspection via .harness/)
- **O que resolve**: auditar/caracterizar outro harness lendo só os arquivos declarativos, sem executar código.
- **Como funciona**: repo-alvo declara `.harness/`: `manifest.json` (identidade: host, template, sha256) e `mcp-policy.json`. O meta-harness lê e produz `score` (5 dims numéricas: harnessFit/compileConfidence/taskCoverage/toolSafety/memoryUsefulness + estCostPerRunUsd) e `genome` (7 seções: repo_type/agent_topology/risk_score/mcp_surface/test_confidence/publish_readiness). `similarity` (cosine 0.4 + categorical 0.3 + jaccard 0.3).
- **Onde**: `.harness/manifest.json`, `plugins/ruflo-metaharness/scripts/score.mjs`, `genome.mjs`.
- **Aplicação no Cosca**: declarar `.cosca/harness.json` + `claims.json` descrevendo topologia de agentes, superfície de MCP, postura de segurança — "Cosca Genome Score" para avaliar maturidade/drift.

### D6. Superfície MCP declarada + Envelope uniforme {success,data,degraded,exitCode}
- **O que resolve**: converter as ferramentas num conjunto grande/categorizado com semântica processável por modelo + segurança via policy.
- **Como funciona**: um servidor MCP registra ~314 tools; cada handler retorna envelope `{success, data, degraded, exitCode}` onde `success===false` é a fonte de verdade (derivado de `exitCode===0`). Descrições com "Use when...". Segurança: `PermissionRequest` auto-aprova `mcp__claude-flow__.*`, `mcp-policy.json` default-deny (shell/network/file-write=false), `dangerousPatterns` (rm -rf, sudo, curl|sh, ssh, git push --force), `maxToolCallsPerTurn:200`, `auditLog:true`.
- **Onde**: `v3/@claude-flow/cli/src/mcp-tools/`, `mcp-policy.json`, `plugin/hooks/hooks.json`.
- **Aplicação no Cosca**: expor capacidades como MCP server único/categorizado; **padronizar TODO resultado no envelope `{success,data,degraded,exitCode}`**; modelar `mcp-policy.json` (default-deny + dangerousPatterns + budget) como layer de controles.

### D7. Meta-harness como Gerador/Evoluidor de Harnesses (mint + darwin evolve + flywheel)
- **O que resolve**: gerar e evoluir harnesses a partir de templates/hosts, com promoção governada por evidência.
- **Como funciona**: **`mint`** gera harness (`metaharness new <name> --template <id> --host <id>`), dry-run por default, recusa escrever na raiz. **`evolve`** (Darwin) muta 7 superfícies de policy, pontua em **sandbox**, promove só ganhos medidos. **`bench`** cria suites `{input, expectedOutput, weight}` (baseline estável vs npm test ruidoso). **`flywheel`/ADR-322** promoção por *receipts* imutáveis com assinatura Ed25519 + ledger append-only + promoção CAS com `confirm=true`.
- **Onde**: `scripts/mint.mjs`, `scripts/evolve.mjs`, `scripts/bench.mjs`, `mcp-tools/metaharness-tools.ts`, `v3/@claude-flow/cli/src/commands/eject.ts`.
- **Aplicação no Cosca**: codegen de harness; **evolução por mutação de política + sandbox** (o kernel evolui prompts/pesos fazendo single-DOF mutations, promovendo só ganhos medidos); **promoção por receita-assinada/ledger** (CAS + chave aprovada, nunca implícito).

---

## E. Plugin / Capability Inventory / Verification

### E1. Baseline Monotone-Decrescente (multi-checkpoint "só pode diminuir")
- **O que resolve**: proteger superfícies de capabilities (285 MCP tools, 40+ CLI) contra regressão de cobertura/signatura sem um teste por tool.
- **Como funciona**: JSON versionado em `verification/` guarda o piso; script de auditoria recomputa a métrica e `exit(1)` se **subir** acima do baseline. `--update-baseline` só re-trava um piso **menor** após correções reais (nunca mascarar). Ex: `mcp-tool-baseline.json` (withoutGuidance/tooShort/duplicateDescriptions = 0/0/0 sobre 285 tools).
- **Onde**: `scripts/audit-tool-descriptions.mjs`, `verification/mcp-tool-baseline.json`.
- **Aplicação no Cosca**: `verification/inventory-baseline.json` por domínio (nº de tools/skills com cobertura baixa, plugins sem ADR) que só decresce — portão de release do ecossistema de plugins.

### E2. Manifesto de Testemunha Criptográfico (presença de código-carga + assinatura reprodutível)
- **O que resolve**: atestar que o código-carga de cada fix ainda existe, mesmo sem teste de fumaça.
- **Como funciona**: SHA-256 por arquivo + **marker** (substring única do fix) que deve permanecer; assinatura Ed25519 sobre manifest canônico com seed determinística `sha256(gitCommit + ':ruflo-witness/v1')` (qualquer um re-deriva a mesma pubkey). Distingue **drift** (hash mudou, marker presente → falso alarme) de **regressão** (marker sumiu → bloqueia). Por-OS (hash drift com LF/CRLF).
- **Onde**: `verification/README.md`, `plugins/ruflo-core/scripts/witness/*.mjs`.
- **Aplicação no Cosca**: cada capacidade/skill/agent carrega marker de "código-carga" atestado por assinatura — evita skills-fantasma/features descontinuadas.

### E3. Capability Brain "data-only" + injeção da registry
- **O que resolve**: o harness saber tudo sobre as ~314 tools sem dependência circular; catálogo ≠ disponibilidade ≠ autorização.
- **Como funciona**: `capability-brain.ts` é **só dados** (sem import da registry); o `mcp-client.ts` injeta a lista viva via `configureGuidanceToolProvider`. O brain classifica por prefixo (domínios), cada domain declara `maturity`, `authority` (advisory/capability-plane/control-plane), `risk` (read-only/reversible/privileged), `availabilityMode`, `verifyBeforeUse`. Health reporta 5 fatos independentes: `registered | configured | reachable | healthy | authorized` = `unknown` até sondados.
- **Onde**: `v3/@claude-flow/cli/src/mcp-tools/capability-brain.ts`, `guidance-tools.ts`.
- **Aplicação no Cosca**: o modelo exato do **capability-inventory** — brain data-only por plugin, com authority/risk/availabilityMode e os 5 fatos independentes.

### E4. Auditoria estática que "espelha o loader"
- **O que resolve**: pegar tool definida mas nunca wired (bug #1916 — tool em arquivo não importado).
- **Como funciona**: `audit-cli-mcp-tools.mjs` re-implementa a resolução do `mcp-client.ts`: extrai os spreads `...getX()` dentro de `registerTools([...])`, resolve cada `import {...} from '<path>'`, escaneia só esses arquivos. Tool em arquivo não importado **não conta** como registered. Âncora única = `mcp-client.ts`.
- **Onde**: `scripts/audit-cli-mcp-tools.mjs`, `v3/@claude-flow/cli/src/mcp-client.ts`.
- **Aplicação no Cosca**: o verificação do inventário precisa "ler o mesmo código que registra" em vez de grep — detecta capability fantasma.

### E5. Catálogo de "drift de MCP" embutido no scaffolder (conhecimento injetado para frente)
- **O que resolve**: erros de semântica de capability que se repetem na família (referenciar tool inexistente, confundir namespace).
- **Como funciona**: `ruflo-plugin-creator` mantém tabela **"MCP-tool drift to avoid"** que vira (a) seção no `create-plugin/SKILL.md` e (b) checks no `smoke.sh`. Ex: `embeddings_embed`→`embeddings_generate`; `agentdb_pattern-*` rota via ReasoningBank; `pattern`≠`patterns`; "19 controllers" era mito. Todo plugin já nasce carregando as correções.
- **Onde**: `plugins/ruflo-plugin-creator/skills/create-plugin/SKILL.md`, `scripts/smoke.sh`.
- **Aplicação no Cosca**: catálogo de anti-padrões de capability versionado que o `cosca-plugin`/skill-creator injeta no template de cada novo plugin.

### E6. Contrato de plugin como ADR scaffolded + smoke.sh "o contrato é o portão"
- **O que resolve**: padronizar/provar que 40+ plugins seguem a mesma forma.
- **Como funciona**: shape canônico: `.claude-plugin/plugin.json` (sem arrays skills/commands/agents — auto-discovery por diretório), `skills/<n>/SKILL.md` (name/description/allowed-tools sem wildcard), `commands/*.md`, `agents/*.md`, `docs/adrs/0001-*.md`, `scripts/smoke.sh` (≥8 checks), README com seções. Namespace `<plugin-stem>-<intent>`. `validate-marketplace.yml` roda o contrato em CI.
- **Onde**: `plugin/.claude-plugin/plugin.json`, `plugins/<n>/.claude-plugin/plugin.json`, `.github/workflows/validate-marketplace.yml`.
- **Aplicação no Cosca**: cada plugin/skill carrega ADR+smoke+README-de-contrato, auto-descoberto, validado num job único de marketplace/smoke.

### E7. Descrições "Use when native X is wrong" (ADR-112)
- **O que resolve**: 285 tools não servem se o LLM sempre escolhe as nativas (Read/Write/Bash).
- **Como funciona**: toda descrição de MCP tool responde (1) "Use when `<nativo>` is wrong porque `<valor concreto>`"; (2) nomear o nativo; (3) ser honesta quando o nativo é melhor. Anti-padrão ("Spawn an agent", "Store data") rejeitado. `audit-tool-descriptions.mjs` cobra `/Use when|Prefer...over|fall back|use over native/i` + comprimento ≥80 + unicidade.
- **Onde**: `v3/docs/adr/ADR-112-mcp-tool-discoverability.md`, `scripts/audit-tool-descriptions.mjs`, `verification/mcp-tool-baseline.json`.
- **Aplicação no Cosca**: regra de ouro para skill/tool discoverability — cada capability tem "Use quando X é errado porque..." + nativo equivalente, com audit em CI que bloqueia release se a cobertura subir.

---

## Synthesis — o que o Cosca deveria copiar (priorizado)

| # | Padrão | Gap | Aplicação no Cosca |
|---|--------|-----|--------------------|
| 1 | B1/B7 — memória imutável (invalida, não sobrescreve) + face legível↔vetorial | memória durável | `cosca-memory-chief`/`semantic-memory` |
| 2 | B4/B5 — reforço de confiança (boost/decay/EWC) + pattern mining/EMA/pruning | self-evolving real | memória valorada, consolidação sem esquecimento |
| 3 | C3/C5/C6 — handshake capability-negotiated + PII transform + trust ladder PEP/PDP | federação segura | `integrations`/`messaging` multi-máquina |
| 4 | D2 — harness degradável (removable, graceful, envelope degraded) | camada de harness | `cosca-runtime`/adaptadores opcionais |
| 5 | E1/E3 — baseline monotônico + capability brain data-only (5 fatos) | inventário de capabilities | portão de release do ecossistema de plugins |
| 6 | A2/A4 — weighted semantic routing + DAG scheduling com rollback | orquestração | `cosca-*` routing + workflow |
| 7 | D6 — envelope uniforme {success,data,degraded,exitCode} + mcp-policy | controles do agente | padronizar resultado de todo tool |
| 8 | E5/E7 — catálogo de drift + "Use when native X is wrong" | discoverability | skills que os agentes realmente chamam |

## Known Uses (referência)

- `ruvnet/ruflo` — meta-harness de agentes (Claude Code/Codex), 8.1M+ downloads, 106k clones/14d.

## Related Patterns

- [`hermes-self-evolution-patterns.md`](hermes-self-evolution-patterns.md) — memória tvalueada + EWC + self-evolution
- [`deepseek-harness-patterns.md`](deepseek-harness-patterns.md) — plugin/session/gates
- [`anthropics-skills-patterns.md`](anthropics-skills-patterns.md) — skill specification + meta-loop A/B
- [`kubernetes-org-patterns.md`](kubernetes-org-patterns.md) — observabilidade de estado + sharding
