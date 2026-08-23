# n8n Workflow Engine Patterns — Grafo, Execução, Credenciais, Scaling

> **Version**: 1.0.0 | **Confidence**: 0.90 | **Category**: Enterprise Platform Patterns | **Created**: 2026-08-22 | **Source**: https://github.com/n8n-io/n8n (200k★, Sustainable Use License)

> **Mined by**: cosca-kernel (ordem do Don). Extraído de `packages/workflow`, `packages/core` (execution-engine) e `packages/cli` (scaling). O motor de automação de workflow mais adotado — o modelo de referência para **engine de execução de DAG com join/fan-in** e **data lineage**.

## Purpose

O n8n resolve o problema de **executar workflows grafo-based (branch/join/loop) com um engine síncrono-por-nó mas sem deadlock**, e de manter **lineage de dados** e **segredos criptografados por instância**. É o que falta para o cosca-workflow-chief: um engine que realmente roda DAG não-trivial com join e retomada parcial.

---

## 1. Workflow = Nós + Mapa de Conexões (3 níveis), não lista de edges
- **O que resolve**: representar um workflow arbitrário (multi-thread, multi-input, self-join, branches) sem perder a semântica de "qual output → qual input".
- **Como funciona**: um workflow é um par `nodes: INode[]` + `connections: IConnections`. `IConnections` é um mapa aninhado por 3 chaves: `connections[sourceNodeName][connectionType]['main'][outputIndex] = IConnection[]`. A `IConnection` é `{node, type, index}`. Em vez de edges soltos, guarda-se um índice *por origem* (`connectionsBySourceNode`) e *por destino* (`connectionsByDestinationNode`), ambos derivados do mesmo `connections` no construtor. Isso permite `getParentNodes`/`getChildNodes` (BFS sobre o mapa reverso), `queryNodes` e `getStartNode` sem recalcular o grafo. `INode` carrega `type`, `typeVersion`, `parameters`, `credentials`, `position`, `disabled`, retry settings e `onError` — todo o comportamento de execução é declarado no nó. `NodeConnectionTypes` generaliza portas: `main` + `ai_*` permitem grafos não-main.
- **Onde**: `packages/workflow/src/interfaces.ts` (IWorkflowBase:3493, INode:1608, IConnections:471, IConnection:93), `packages/workflow/src/workflow.ts` (índices bidirecionais, getStartNode:871, getParentNodes:595).
- **Aplicação no Cosca**: portar o *índice bidirecional por origem+destino* em vez de grafo de edges. É o que torna trivial "rodar a partir do nó X" (basta pegar `connectionsByDestinationNode[X]`), computar pais de um nó para reproduzir um braço, e suportar join de múltiplos inputs.

## 2. Engine de execução = pilha de execução + acumulador de joins (`waitingExecution`)
- **O que resolve**: executar um DAG com branching, multi-input joins e loops de forma síncrona-por-nó mas sem deadlock, permitindo pausar/retomar e execução parcial.
- **Como funciona**: `WorkflowExecute.run()` inicializa `nodeExecutionStack` com o start node e roda um loop `while (nodeExecutionStack.length)`: dá `shift()` no topo, executa o nó e faz `push`/`unshift` dos nós downstream — `executionOrder` decide push (BFS) vs unshift (DFS). O branching é trivial: cada output connection vira um item separado na pilha. O **join** é o padrão mais profundo: `waitingExecution[nodeName][runIndex].main[inputIndex]` acumula dados por input; um nó com N inputs só é enfileirado quando todos os inputs têm `data !== null` — é um "barrier" por nó. Quando a pilha esvazia mas sobra `waitingExecution`, faz-se um *drain* final que executa nós cujos inputs não são todos obrigatórios. Dados de saída por nó viram `resultData.runData[nodeName]`. Execução parcial reusa um `DirectedGraph.fromWorkflow()` + `findSubgraph` + `findStartNodes` + `handleCycles` + `recreateNodeExecutionStack`. Cancelamento: `run()` retorna um `PCancelable` (não async!) ligado a um `AbortController`.
- **Onde**: `packages/core/src/execution-engine/workflow-execute.ts` (run:141, runNode:1311, checkReadyForExecution:859, processRunExecutionData:1586), `.../partial-execution-utils/*`.
- **Aplicação no Cosca**: copiar o par *fila-de-escopo (stack) + acumulador de inputs por runIndex* para suportar fan-in. É um barrier leve (sem counters globais), usa os dados do próprio nó como estado. Ponto ideal para expor "resume a partir do nó N preservando dados anteriores".

## 3. Item + `pairedItem` — a linhagem do dado entre nós
- **O que resolve**: saber, para cada item de saída, de que item de entrada ele veio — essencial para `$json` e para ramos/loops, onde a relação 1:1 se perde.
- **Como funciona**: `INodeExecutionData` = `{ json, binary?, pairedItem?, error?, metadata?, redaction? }`. `pairedItem: { item: number; input?: number; sourceOverwrite? }` aponta para o item de origem. Antes de cada node execution, o engine reescreve os itens de entrada com `pairedItem` correto; `resolveSourceOverwrite` preserva para nodes-tool. Depois da execução, `assignPairedItems` propaga a linhagem de volta aos outputs. É isso que funda o "data lineage" usado pelo proxy para responder `$('Node').item` mesmo em branches.
- **Onde**: `packages/workflow/src/interfaces.ts` (INodeExecutionData:1722, IPairedItemData:1659), `packages/core/src/execution-engine/workflow-execute.ts` (:1713, :2838).
- **Aplicação no Cosca**: se o Cosca precisa de data-mapping/transform entre estágios, `pairedItem` é o modelo para rastreabilidade e para o sandbox de expressões. Também habilita "pular nós que não mudaram".

## 4. Expressões resolvidas via `WorkflowDataProxy` lazy (Proxy JS) + sandbox por isolate
- **O que resolve**: avaliar `${json}`/`$node`/`$items`/`$input`/`$env` dentro de parâmetros do nó, com resolução adiada e sem expor internals.
- **Como funciona**: `WorkflowExpression.getParameterValue` detecta string de expressão, cria um `WorkflowDataProxy` e chama `getDataProxy()`. `getDataProxy` constrói um objeto com todos os `$` helpers, mas `$json`/`$data`/`$binary` são placeholders: o `get` trap do Proxy intercepta e só então busca o item da corrente via `nodeDataGetter`. A leitura é *lazy* e *contextual*: resolve contra o `runIndex`/`itemIndex`/`executeData` capturados no momento. `$items()`/`$input.first()` re-criam o proxy com itemIndex diferente. A execução da string é sandboxed por um isolate; código do nó Code roda num runner separado.
- **Onde**: `packages/workflow/src/workflow-data-proxy.ts` (getDataProxy:818, get trap:1674), `packages/workflow/src/workflow-expression.ts`, `packages/cli/src/task-runners/*`.
- **Aplicação no Cosca**: implementar "mappings" como um Proxy lazy sobre um "staged data context". Dá custo proporcional ao que é lido, resolução contextual por item/run, e um ponto único para sandbox/whitelist de helpers. O isolate separando o runner de código é o padrão que o Cosca deve replicar para rodar funções do usuário com segurança.

## 5. Credenciais: resolução lazy por nó + encryption-at-rest com modelo envelope-key
- **O que resolve**: guardar segredos criptografados por instância, entregar ao nó só na execução, respeitar escopo por nó e permitir secrets externos.
- **Como funciona**: `_getCredentials(type)` resolve type→decrypt **na hora** e no contexto do nó: valida o tipo, checa restrição `isCredentialUsableByNode`/`restrictToSupportedNodes` e `FULL_ACCESS_NODE_TYPES`. `Credentials.getData()` faz `cipher.decryptV2(this.data)`. `Cipher` com AES-256-CBC usando `instanceSettings.encryptionKey`; **envelope-key**: `encryptV2`/`decryptV2` guardam os bytes como `${keyId}:${ciphertext}`, em que o data-encryption-key é gerado por instância, wrappeado com o instance key via AES-256-GCM — permite rotação de chave sem re-encryptar tudo. Secrets externos: `getSecretsProxy` é um Proxy aninhado `$secrets[{provider}][{secret}]` que lança `ExpressionError` ao acessar secret inexistente e faz gating por `externalSecretProviderKeysAccessibleByCredential` (a credencial só vê providers permitidos).
- **Onde**: `packages/core/src/credentials.ts`, `packages/core/src/encryption/cipher.ts` (:32-104), `.../execution-engine/node-execution-context/node-execution-context.ts` (_getCredentials:318), `.../utils/get-secrets-proxy.ts`.
- **Aplicação no Cosca**: (a) resolver credencial por "referência declarada no nó" e nunca embutir segredo no workflow; (b) cifrar com envelope-key (DEK + key-manager) para permitir rotação; (c) expor secrets via Proxy lazy; (d) manter allowlist de "quais credenciais esse nó pode ver" (escopo) — padrão direto para RBAC em secrets.

## 6. Queue/Scaling: Bull jobs + throttle por fila de concurrency + leader election + queue recovery
- **O que resolve**: rodar o engine em instâncias separadas (main/webhook/worker), com controle de concorrência por modo, sem execução duplicada de triggers, recuperando jobs órfãos.
- **Como funciona**: `ScalingService` cria uma Bull queue (`queue.add(JOB_TYPE_NAME, jobData)`) e processa com `queue.process(JOB_TYPE_NAME, concurrency)`. Dois níveis de controle: (1) concorrência do Bull por worker; (2) throttle **intencional** por modo via `ConcurrencyControlService` (`throttle`, `release`, `ConcurrencyQueue` por production/evaluation) que decide deixar ou bloquear antes de enfileirar/desenfileirar. Para não duplicar ativação de triggers em multi-main: `LeaderElectionClient` elege um líder; comandos são broadcast via pub/sub. Recuperação de jobs pendurados: `scheduleQueueRecovery` compara execuções no DB vs jobs no queue e re-enfileira as órfãs.
- **Onde**: `packages/cli/src/scaling/scaling.service.ts` (:60,114,236,582), `job-processor.ts`, `leader-election-client.ts`, `concurrency-control.service.ts`.
- **Aplicação no Cosca**: reproduzir a separação *enqueue* (produtor) vs *work* (worker) com uma fila (Bull/Redis), cap por `concurrency`, e camada extra de throttle por "tipo de carga". Isso dá priorização (por fila/modo) e escala horizontal; recovery de órfãos e leader election são imprescindíveis para idempotência de triggers.

## 7. Observabilidade: `runExecutionData` versionado + lifecycle hooks + execution-context propagado
- **O que resolve**: registrar cada execução (per-node timing, index, branch source, erro) de forma migrável, emitir eventos para UI/telemetria e carregar contexto (modo, parent execution, redaction, user) através de sub-workflows.
- **Como funciona**: a run inteira vive em `IRunExecutionData` com **schema versionado** (`run-execution-data.v0.ts` → `.v1.ts` com migração). `ITaskStartedData` guarda `startTime`, `executionIndex` (ordem global), `source` (array de `ISourceData` por input). `ITaskData` adiciona `executionTime`, `executionStatus`, `data`, `error`, `tracing`. Os hooks `ExecutionLifecycleHooks` disparam `workflowExecuteBefore/After`, `nodeExecuteBefore/After`, `sendChunk` — o canal que alimenta push, persistência e telemetria. O `execution-context.ts` carrega `mode`, `parentExecutionId`, `triggerNode`, `redaction`, `executedByUserId`, e os `credentials` **sempre criptografados** quando persistidos (`PlaintextExecutionContext` é só runtime). Em cima há OTEL com span por node e `span-sampling`.
- **Onde**: `packages/workflow/src/run-execution-data/run-execution-data.v1.ts`, `packages/workflow/src/interfaces.ts` (ITaskData:3417), `packages/core/src/execution-engine/execution-lifecycle-hooks.ts`, `packages/cli/src/events/event.service.ts`.
- **Aplicação no Cosca**: (a) guardar o run inteiro num blob versionado (migração sem quebrar runs antigos); (b) emitir lifecycle hooks por nó para alimentar UI/logs/métricas sem acoplar o engine; (c) carregar "execution context" (modo, parent id, identidade, policy de redaction) e propagá-lo a sub-contextos — a chave para rastrear execução pai→filho e aplicar redaction de segredos automaticamente.

---

## Synthesis — o que o Cosca deveria copiar

| # | Padrão | Ganho |
|---|--------|-------|
| 1 | Índice bidirecional origem+destino | branch/join trivial, "rodar do nó X", resume parcial |
| 2 | Stack + `waitingExecution` barrier | execução de DAG com fan-in sem deadlock |
| 3 | `pairedItem` (linhagem de dados) | rastreabilidade e data-mapping em branches |
| 4 | `WorkflowDataProxy` lazy + isolate | expressões sandboxed, custo proporcional |
| 5 | Credenciais envelope-key + Proxy lazy | rotação de chave, RBAC em secrets |
| 6 | Queue + leader election + recovery | escala horizontal, idempotência, órfãos recuperados |

## Related Patterns

- [`uber-cadence-patterns.md`](uber-cadence-patterns.md) — execução durável/replay (o outro paradigma de orquestração)
- [`deepseek-harness-patterns.md`](deepseek-harness-patterns.md) — session durável (C2-C4), gate de sistemas (E5)
- [`openai-symphony-patterns.md`](openai-symphony-patterns.md) — scheduler/claims
