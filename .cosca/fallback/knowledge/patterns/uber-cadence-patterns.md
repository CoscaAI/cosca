# Uber Cadence Patterns — Execução Durável, Decisor, Replay, Sharding/N_DC

> **Version**: 1.0.0 | **Confidence**: 0.90 | **Category**: Enterprise Platform Patterns | **Created**: 2026-08-22 | **Source**: https://github.com/uber/cadence (MIT, agora maintained by Temporal)

> **Mined by**: cosca-kernel (ordem do Don). Extraído de `service/history/`, `service/matching/` e `docs/`. O antecessor do Temporal — **event-sourcing de workflow durável**. O que é **distinto do Temporal** (e vale reaproveitar): modelo **NDC-AP** com version history em árvore, sub-árvore de partição do matching (forwarding) e o "transient started" com retry.

## Purpose

O Cadence resolve o problema de **execução durável de workflows** — um "processo distribuído" cujo estado é do servidor, não do worker. O servidor não executa o código do workflow; o worker (decider) roda o código e devolve **decisões**; o servidor persiste mutações como eventos e reconstrói o estado re-executando-os. É o paradigma que o Cosca precisa para execução confiável de pipelines de agentes de longa duração.

---

## 1. Decisor como fonte única de progresso + event-sourcing por mutable state
- **O que resolve**: como um workflow de longa duração sobrevive a crashes de host. O servidor apenas persiste as mutações como eventos e reconstrói o estado re-executando-os.
- **Como funciona**: cada execução tem um histórico append-only de eventos com `eventID` monotônico. O SDK recebe uma *decision task*, re-executa o código e devolve **decisões** (comandos: `ScheduleActivityTask`, `StartTimer`, `CompleteWorkflow`...). O servidor valida cada decisão e chama `mutableState.AddXxxEvent(...)`. O histórico é re-projetado em memória via `mutableStateBuilder` (mapas `pendingActivityInfoIDs`, `pendingTimerInfoIDs`, `pendingChildExecutionInfoIDs`, `bufferedEvents`). O contrato de máquina de estados é explícito (decider: Scheduled→Started→Closed; activity: idem; child: Initiated→Started→Closed; timer: Scheduled→Fired/Canceled).
- **Onde**: `service/history/decision/handler.go`, `.../task_handler.go:124`, `.../execution/mutable_state_builder.go`, `docs/non-deterministic-error.md`.
- **Aplicação no Cosca**: se o Cosca tiver execução durável, modele o núcleo como projeção de um log de eventos (não tabelas de estado "vivas"). A unidade de progresso é o agente/decisor que emite comandos; o log é a fonte da verdade e o estado é sempre derivado — isso dá recovery, replay e auditabilidade de graça.

## 2. Replay determinístico (por que o código do workflow é determinístico)
- **O que resolve**: workflows podem migrar entre workers (e o stack vivo fica em cache LRU por pressão de memória). Ao voltar, o estado precisa ser re-derivado identicamente — qualquer divergência é falha de deploy e não pode ser ignorada silenciosamente.
- **Como funciona**: o histórico é re-executado do zero no novo worker até o ponto atual; atividades/timers/child-workflows **não são re-executados durante replay** — apenas o código de decisão. Para isso, o código cliente é obrigatoriamente determinístico: nada de `time.Now()` (usar `workflow.Now()`), `goroutine` nativa (usar `workflow.Go`), canal nativo (usar `workflow.Channel`), `time.Sleep`, nem reordenação de chamadas. O SDK compara decisões do replay vs eventos do histórico e falha com erros categorizados ("missing decision", "extra decision", "decision mismatch"). Versionamento via `GetVersion()`/`SideEffect()` e `BinaryChecksum` + `workflow.Now()`. O servidor guarda **checksum do mutable state** para detectar corrupção e auto-reparo.
- **Onde**: `docs/non-deterministic-error.md`, `service/history/execution/{state_rebuilder,workflow_repairer,checksum}.go`.
- **Aplicação no Cosca**: toda linguagem de workflow durável precisa de um **juramento de determinismo** (clock injetado, concorrência via abstração própria, IDs via sequência determinística). Grave o `BinaryChecksum` do agente por decisão para viabilizar rollback seguro e resets sem quebrar execuções em andamento.

## 3. Serviço de matching = fila durável + matcher em memória + long-poll sem busy-wait
- **O que resolve**: como produtores (history) e consumidores (workers) se acoplam sem polling agressivo, com durabilidade do backlog e distribuição entre múltiplos hosts de matching.
- **Como funciona**: cada tasklist tem um `taskListManager` com (a) `taskListDB` (índice em Cassandra/SQL — tasks persistidas como fonte durável), (b) `taskWriter`, (c) `taskReader` (drena backlog do DB), (d) um **`taskMatcher`** que faz match produtor↔consumidor em **canais síncronos** (taskC, `isolatedTaskC`, `queryTaskC`) com rate limiter — o poller fica *bloqueado* num canal, sem busy-loop; (e) `taskAckManager` (ack/retry por mensagem), `taskGC`, `liveness`, `taskCompleter`. Long-poll com timeout de `returnEmptyTaskTimeBudget` (~1s) para devolver "no task". Sincronização orientada a eventos (notifier) para acordar o poller.
- **Onde**: `service/matching/tasklist/{task_list_manager,matcher,task_reader,task_writer,task_completer,task_gc,db}.go`, `service/matching/poller/manager.go`.
- **Aplicação no Cosca**: para execução durável + jobs, separe "filas de trabalho" como durabilidade (o que está pendente não se perde) de "matcher" (matching eficiente em memória, com ack/retry e long-poll). Evite fila em memória pura; a combinação *persistência + canais bloqueantes de match* é o padrão para escala sem perder load.

## 4. Sticky task lists (pin de workflow no worker) — distinto do Temporal no grau
- **O que resolve**: evitar re-entregar o histórico inteiro a cada decision task, que é caro para workflows longos. Depois que um worker executa uma execução, ela fica "pinada" nele.
- **Como funciona**: a execução passa a usar um tasklist especial (sticky), e o worker cacheia o stack vivo em sua *sticky cache* (LRU). Na próxima decision task, o history envia para o tasklist sticky, permitindo entrega incremental (sem replay completo). O estado ficará **transient** enquanto o worker segurar o stack. Se o worker morrer ou o `StickyScheduleToStartTimeout`/`StickyTTL` expirar, a stickyness é limpa e a próxima task volta ao tasklist normal, forçando replay. Desabilitar sticky é usado para diagnosticar erros de determinismo.
- **Onde**: `service/history/execution/mutable_state_decision_task_manager.go`, `.../mutable_state_builder.go:812`, `docs/non-deterministic-error.md`.
- **Aplicação no Cosca**: é um padrão de *cache de sessão de agente*: uma "hot path" de entrega incremental para o agente que recentemente executou a workflow, com TTL e fallback para replay completo. Modelar como otimização de latência (sacrificando algum replay) e manter a opção de desligar para depurar.

## 5. Escala horizontal de tasklist: árvore de partições + forwarding + adaptive scaler
- **O que resolve**: uma tasklist única vira gargalo, pois o Matching é "sharded por tasklist". Para escalar, a tasklist é particionada sem mudar o contrato do cliente.
- **Como funciona**: as partições formam uma **árvore**; uma requisição pode ser **encaminhada** recursivamente ao pai até a raiz, para que um poller ocioso numa partição seja aproveitado por uma task de outra (o forwarder tem `MaxOutstandingPolls`, `MaxOutstandingTasks`, `MaxRatePerSecond`, `MaxChildrenPerNode`). O cliente escolhe a partição via LoadBalancer: **random**, **round-robin**, ou **weighted** (escolhe pelo backlog se > 100, senão round-robin — melhor matematicamente). Um **`adaptiveScaler`** monitora QPS por partição/isolation group e redimensiona, sincronizando topologia para os pollers.
- **Onde**: `service/matching/tasklist/{adaptive_scaler,forwarder,shard_processor}.go`, `client/matching/loadbalancer.go`, `docs/scalable_tasklist.md`.
- **Aplicação no Cosca**: quando um tópico/fila único saturar, use *root partitions + forwarding* em vez de rebalanceamento complexo: mantenha o contrato de "enfileirar/consumir" estável e mova a decisão de partição para um LoadBalancer que leia backlog. O forwarder é o truque de não desperdiçar pollers ociosos.

## 6. Separação Activity vs Workflow + heartbeats/timeouts como eventos de 1ª classe
- **O que resolve**: distinguir código "de fio" (longo, com efeitos colaterais, possivelmente não-determinístico — Activity) do código de orquestração determinístico (Workflow). O workflow nunca faz I/O direto; delega a activities.
- **Como funciona**: Activity tem máquina de estados própria (`Scheduled→Started→Closed`). Elas são executadas por workers e reportam progresso via **heartbeat** (`RespondActivityTaskHeartbeat`), que o servidor grava como `LastHeartBeatUpdatedTime`. Há timeouts independentes: `ScheduleToStart`, `StartToClose`, `ScheduleToClose`, `Heartbeat`. **Caso especial com `RetryPolicy`**: Cadence NÃO grava `ActivityTaskStarted` imediatamente; marca `StartedID = TransientEventID` e só materializa `Started` quando a atividade fecha (para não inflar o histórico em retries). Decisões também têm timeout (`WorkflowTaskStartToCloseTimeout`).
- **Onde**: `service/history/execution/mutable_state_builder_methods_activity.go:361`, `service/history/engine/engineimpl/respond_activity_task_heartbeat.go`, `docs/non-deterministic-error.md`.
- **Aplicação no Cosca**: separe estritamente o motor de orquestração determinístico do executor de efeitos (I/O, LLM, chamadas externas). Heartbeat é o mecanismo de *liveness* do executor; timeouts de schedule-to-start/start-to-heartbeat/start-to-close devem ser propagados como eventos de domínio para a workflow reagir explicitamente. O padrão "transient started com retry" evita escrever eventos repetidos em retries.

## 7. History sharding + multi-tenancy por domínio + NDC (multi-DC AP)
- **O que resolve**: particionar a carga de histórico entre hosts e isolar múltiplos tenants, mantendo replicação cross-cluster para failover/HA.
- **Como funciona**: cada **domínio** é a fronteira de tenant e de consistência/replicação. Shards determinísticos: `WorkflowIDToHistoryShard = farm.Fingerprint32(workflowID) % numberOfShards`. Os shards são distribuídos via **anel de membership** (hash ring); o `ShardController` sobe/derruba engines conforme mudanças. Tudo que um workflow faz acontece dentro de um único shard (ordenação garantida). Para multi-DC, o design **NDC é AP**: gera um **`version`** por domínio, cada evento carrega a versão; há **version history** (resumo comprimido), **histórico em árvore** com **conflict resolution** (branch de maior version vira a corrente), **zombie workflows**, e **replicação assíncrona** (`service/history/replication/`, `ndc/`). A replicação gera DLQ por `ShardID`; `queuev2` é a versão nova do processador de transfer/timer com virtual queues.
- **Onde**: `service/history/shard/controller.go`, `common/util.go:367`, `common/domain/`, `service/history/ndc/{conflict_resolver,branch_manager}.go`, `docs/design/{2290-cadence-ndc,active-active}.md`.
- **Aplicação no Cosca**: use sharding determinístico (`hash(status de execução) % N`) para garantir ordenação por execução — é a régua que torna o event-sourcing correto sem locks globais. Trate tenant como unidade de isolamento E de consistência. Para HA cross-região, adote NDC: versão por tenant + histórico em árvore + conflict resolution por "maior versão vence" + estado zombie, em vez de consistência forte global (AP).

---

## O que é distinto do Temporal (e vale reaproveitar)

1. **NDC-AP** com *version history em árvore + conflict resolution + zombie workflows* — o Temporal moderno simplificou isso.
2. **Sub-árvore de partição do matching (forwarding)** — para escala de tasklist.
3. **"Transient started" com retry** — não materializar eventos de tentativa.
4. **Separação explícita persistência ↔ matcher em memória** na camada de matching.

## Synthesis — o que o Cosca deveria copiar

| # | Padrão | Ganho |
|---|--------|-------|
| 1 | Decisor + event-sourcing (mutable state) | recovery/replay/audit de graça |
| 2 | Replay determinístico + juramento de determinismo | migração de worker segura, rollback por checksum |
| 3 | Matching: persistência + canais bloqueantes | fila durável sem busy-wait |
| 4 | Sticky tasklists (cache de sessão) | latência baixa para execuções ativas |
| 5 | Partições em árvore + forwarding | escala horizontal sem mudar contrato |
| 6 | Activity vs Workflow + heartbeats | separar orquestração de efeitos colaterais |

## Related Patterns

- [`temporal-workflow-engine-patterns.md`](temporal-workflow-engine-patterns.md) — o sucessor (12 patterns de config e cores)
- [`n8n-workflow-patterns.md`](n8n-workflow-patterns.md) — engine de DAG (paradigma gráfico)
- [`openai-symphony-patterns.md`](openai-symphony-patterns.md) — scheduler/claims
