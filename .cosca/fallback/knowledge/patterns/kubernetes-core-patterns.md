# Kubernetes Core Patterns — Controller, Scheduler Framework, API Machinery

> **Version**: 1.0.0 | **Confidence**: 0.95 | **Category**: Enterprise Platform Patterns | **Created**: 2026-08-13 | **Source**: https://github.com/kubernetes/kubernetes.git (Apache-2.0)

> **Mined by**: cosca-kernel (3 agentes explore em paralelo — controller/reconciliation core, scheduler framework, API machinery/apiserver). Clone sparse de `staging/src/k8s.io/` + `pkg/` (5.421 arquivos Go). Padrões descritos em linguagem neutra.

## Purpose

O Kubernetes é a referência canônica de **orquestração em Go em produção**: observação de estado (informer/workqueue), reconciliação declarativa, agendamento extensível (scheduling framework), e API versionada com validação/admissão. Este documento aprofunda o núcleo de implementação (DeltaFIFO, rate limiters, extension points do scheduler, scheme/conversão, admission chain, storage+resourceVersion) — complementar ao [`kubernetes-temporal-ecosystem-patterns.md`](kubernetes-temporal-ecosystem-patterns.md), que cobre o *ecossistema* (informers, workqueue, spec/status, CRD, leader election) em nível mais alto.

---

## A. Controller / Reconciliation Core (client-go + pkg/controller)

### A1. Informer / Reflector / DeltaFIFO (observer-and-queue pipeline)
- **O que resolve**: observar uma fonte de verdade e entregar mudanças a múltiplos consumidores sem acoplamento e sem perda.
- **Como funciona**: pipeline de 4 estágios. **Reflector** é o único que fala com a fonte (API server): faz `List` (snapshot completo → `lastSyncResourceVersion`) e depois `Watch` incremental a partir dessa RV; re-emite `Sync` em `resync` periódico. **DeltaFIFO** é uma fila *keyed por objeto* (não por evento): `items map[string]Deltas` acumula todos os deltas de um objeto até o `Pop`, com `dedupDeltas` e tombstone (`DeletedFinalStateUnknown`) em `Replace` (relist). **Controller.processLoop** faz `Pop` e `processDeltas` (atualiza o store + chama handlers). **SharedInformer** multiplexa handlers com `processorListener` (3 goroutines + ring buffer) para um handler lento nunca bloquear o DeltaFIFO.
- **Onde**: `tools/cache/reflector.go`, `delta_fifo.go`, `controller.go`, `shared_informer.go`.
- **Cosca**: o `internal/watcher` (fsnotify) é o Reflector do Cosca, mas chama o handler **sincronamente na goroutine do fsnotify** — falta a DeltaFIFO keyed por `path` (coalescer `Modify`, emitir tombstone no delete) para desacoplar o watcher do indexador lento. Falta também relist/resync periódico (`IndexDirectory` + `knownObjects`/fingerprint) para recuperar eventos perdidos.

### A2. Workqueue keyed (dedup + delaying + rate-limiting)
- **O que resolve**: deduplicação, reprocessamento adiado e rate-limit por item.
- **Como funciona**: três camadas compostas. **Fila base** mantém 3 conjuntos (`queue` FIFO, `dirty` precisa-processar, `processing` em-voo) — `Add` deduplica por chave; `Done` re-enfileira se o item ficou `dirty` de novo. **Delaying queue** agenda `AddAfter` com min-heap por `readyAt` e coalesce por item. **Rate-limiting queue** faz `AddRateLimited = AddAfter(rateLimiter.When(item))`. Rate limiters: `ItemExponentialFailureRateLimiter` (`baseDelay * 2^failures` com cap), `BucketRateLimiter` (token bucket global), composto por `MaxOf`. Uso: `handleErr` → sucesso `Forget(key)`; falha `AddRateLimited` se `NumRequeues < maxRetries`, senão drop.
- **Onde**: `util/workqueue/{queue,delaying_queue,rate_limiting_queue,default_rate_limiters}.go`, `pkg/controller/deployment/deployment_controller.go:499`.
- **Cosca**: o `TaskQueue` (`internal/pipeline/taskqueue.go`) usa `chan` cru sem dedup/retry/backoff; o `RecoveryLoop` faz `time.Sleep` inline. Adotar workqueue keyed `planID/taskID` com `ItemExponentialFailureRateLimiter` — move o retry para a fila (libera o worker durante o backoff, backoff exponencial, anti hot-loop), mantendo o `history` durável como fonte de verdade e `RecoverPending` para re-popular no boot.

### A3. Reconcile loop level-triggered
- **O que resolve**: convergência idempotente e tolerante a falhas, sem "thundering herd".
- **Como funciona**: o controller processa a **chave**, não o evento, e re-deriva desired vs actual a cada ciclo. Handlers só fazem `enqueue(key)`; workers fazem `Get → sync(key) → handleErr → Done`. Idempotente (roda 1 ou 100 vezes, converge). Evita thundering herd por: dedup na fila, enqueue por owner (`resolveControllerRef` confere Kind+UID), filtro de resync (compara ResourceVersion), e enfileirar só se o que mudou importa.
- **Onde**: `pkg/controller/deployment/deployment_controller.go` (`Run:171`, `syncDeployment:574`, `handleErr:499`, `resolveControllerRef:461`).
- **Cosca**: o `Reconciler` (`internal/pipeline/reconciler.go`) já é level-triggered correto, mas é **pull-based** (roda no boot/após RunPlan). Adicionar **edge-triggered enqueue**: cada `StepEvent*` gravado no history dispara `enqueue(planID)` numa workqueue — combina edge-enqueue + level-sync.

### A4. Lister + Indexer (indexed read model)
- **O que resolve**: leitura O(1) por múltiplos índices sem bater na fonte.
- **Como funciona**: o Reflector escreve no store; controllers leem via `Indexer` (Store + `Index map[string]sets.Set[string]`, `IndexFunc` mapeia objeto→valores indexados) e `Lister` (pele tipada). Índice customizado: `AddPodControllerIndexer` com chave composta `namespace/kind/name/UID`, `ByIndex` acha todos os pods de um dono em O(1). `AddIndexers` re-indexa itens existentes.
- **Onde**: `tools/cache/{index,store,thread_safe_store,listers}.go`, `pkg/controller/controller_utils.go:1150`.
- **Cosca**: o `detectDrift` re-replaya o `history.Load` inteiro a cada ciclo (O(n) por evento). Materializar um Indexer em memória `planID → map[taskID]TaskStatus` incrementado por evento, e `findTask` deixar de ser busca linear (usar índice por status/DependsOn — o `taskMap`/`dependents` do `RunPlanParallel` já é um índice de dependência reversa).

### A5. Parallelizer (chunked fan-out)
- **O que resolve**: paralelizar trabalho independente com cancelamento e isolamento de panic.
- **Como funciona**: `ParallelizeUntil` particiona `pieces` em chunks, cria um canal buffered e **fecha antes** de lançar N workers (`for chunk := range toProcess`), com `select { case <-ctx.Done(): return; default: doWork }` e `WaitGroup`. `chunks < workers` → reduz workers.
- **Onde**: `util/workqueue/parallelizer.go:46`.
- **Cosca**: o `RunPlanParallel` é fan-out artesanal com `ready`/`sem` + `wg.Add` dinâmico (risco de race/early-close no `close(ready)`). Usar `ParallelizeUntil` dentro de cada "nível" de tasks prontas herda cancelamento e isolamento de panic, e o "canal fechado = fim" elimina o early-close.

---

## B. Scheduler Framework (pkg/scheduler)

### B1. Extension points (interfaces parciais)
- **O que resolve**: estender um algoritmo complexo (agendamento) por adição de peças isoladas, sem herança nem `switch`.
- **Como funciona**: o problema é decomposto em ~15 pontos ordenados (`PreEnqueue → QueueSort → PreFilter → Filter → PostFilter → PreScore → Score → Reserve → Permit → PreBind → Bind → PostBind`, + `Unreserve`). Cada ponto é uma **interface pequena**; um plugin implementa só o subconjunto que interessa, provado por asserção `var _ fwk.FilterPlugin = &X{}`. O framework mantém `[]FilterPlugin`, `[]ScorePlugin`, etc., e encadeia em ordem.
- **Onde**: `pkg/scheduler/framework/runtime/framework.go:127-145`, `plugins/nodeunschedulable/node_unschedulable.go:40-43`.
- **Cosca**: decompor a esteira em hooks por etapa (`PreValidateTask`, `ValidateTask`, `ScoreWorker`, `Execute`, `OnFailure`≈`Unreserve`), cada tipo de task registrando só o que precisa com asserção de interface — elimina `if/else` por tipo de task no StepRunner.

### B2. Ciclo de dois estágios (scheduling + binding)
- **O que resolve**: separar decisão (rápida, sem efeitos colaterais) de ação (lenta, com commit).
- **Como funciona**: o **scheduling cycle** (`PreFilter→Filter→Score→Reserve→Permit`) escolhe o nó e faz um *assume* otimista no cache local (sem falar com o API server). O **binding cycle** (`PreBind→Bind→PostBind`) roda em goroutine e faz o commit real; falha → `unreserveAndForget` reverte. O elo é o `CycleState`.
- **Onde**: `pkg/scheduler/schedule_one.go:93-505`.
- **Cosca**: o `Planner` é o scheduling cycle (sem side-effects), o `StepRunner` é o binding cycle (commit + `Unreserve`/compensação). Nunca misturar decisão e mutação no mesmo passo; todo passo com efeito colateral precisa de um `Unreserve` invocável pelo RecoveryLoop.

### B3. Plugin chain com Status tipado + short-circuit
- **O que resolve**: substituir "sucesso/exceção" por um retorno rico e encadear com regras por ponto.
- **Como funciona**: plugins retornam `*Status` (sum type: `Success`, `Unschedulable`, `UnschedulableAndUnresolvable`, `Error`, `Skip`, `Wait`). `Filter` faz short-circuit no primeiro não-`Success`; `PostFilter` continua até `Success`/`Error`; `Skip` declara "não me aplico"; `Wait` (Permit) segura o objeto. `UnschedulableAndUnresolvable` = falha permanente com diagnóstico (`FitError`).
- **Onde**: `framework/runtime/framework.go:1105-1199`, `plugins/nodeunschedulable/node_unschedulable.go:141`.
- **Cosca**: trocar o retorno binário dos steps por `Status` tipado (`OK/RetryLater/Permanent/Skip/Error`), short-circuit na validação pré-execução, e `Permanent` (com diagnóstico estruturado) consumido direto pelo DoD sem passar por backoff.

### B4. Fila multi-estágio (active / backoff / unschedulable) com retry dirigido por eventos
- **O que resolve**: retry com backoff sem polling cego, e distinção entre rejeição de negócio vs erro de infra.
- **Como funciona**: `PriorityQueue` particiona em `activeQ` (heap por prioridade+timestamp), `backoffQ` (heap por expiração de backoff — `1s<<(count-1)` capado), e `unschedulableEntities` (aguardam *evento*). Cada plugin declara `EventsToRegister()` + `QueueingHintFn` que decide `Queue`/`QueueSkip` quando um evento de cluster chega (`MoveAllToActiveOrBackoffQueue`). Separa `UnschedulableCount` (rejeição) de `ConsecutiveErrorsCount` (erro). Flush periódico evita starvation.
- **Onde**: `backend/queue/scheduling_queue.go:207,1073`, `backoff_queue.go:246`, `queuesort/priority_sort.go:43`.
- **Cosca**: o `taskqueue`/`RecoveryLoop` deve replicar a tripartição (`activeQ`/`backoffQ`/parking), com retry **dirigido por eventos+hints** ("que eventos me tornam executável de novo") em vez de `time.Sleep` fixo, e backoff diferente para falha permanente vs erro transitório.

### B5. CycleState (estado efêmero tipado por ciclo)
- **O que resolve**: passar dados entre plugins sem acoplamento de tipos.
- **Como funciona**: `CycleState` é um `sync.Map` `StateKey(string) → StateData(interface)`; cada plugin define suas próprias chaves (`"PreFilter"+Name`) com write-once/read-many (`PreFilter` escreve, `Filter` lê via type-assert). `StateData` exige `Clone()`, permitindo branching/simulação. Também carrega metadados de controle (`skip*Plugins`).
- **Onde**: `framework/cycle_state.go:28-191`, `plugins/noderesources/fit.go:56-62`.
- **Cosca**: um `TaskStepState` (`StateKey→StateData`) transitando Planner→StepRunner→DoD, com chaves por tipo de task e `Clone()` para retry/simulação — elimina acoplamento entre fases e permite plugar novos tipos de task.

### B6. Registry + factory de plugins
- **O que resolve**: instanciar plugins por configuração, não por código hardcoded, e permitir extensão sem recompilar.
- **Como funciona**: `Registry = map[string]PluginFactory` (`func(ctx, config, handle) (Plugin, error)`); instanciação **lazy** dirigida por config (`pluginsNeeded` → chama factory só dos nomes configurados); cada factory decodifica `args` do YAML/JSON e recebe `handle` (acesso a clientes/informers). `FactoryAdapter` injeta feature gates tipadas. `Register/Merge` permitem plugins out-of-tree.
- **Onde**: `framework/runtime/registry.go:30-101`, `framework/plugins/registry.go:51-81`, `runtime/framework.go:338-431`.
- **Cosca**: registrar hooks/tipos de task num `map[TaskType]TaskHookFactory` em vez de `switch taskType` no StepRunner; instanciar lazy via config da esteira; `Register/Merge` para compor hooks padrão com customizados do usuário.

---

## C. API Machinery + Generic Apiserver

### C1. Scheme + registro de tipos versionados (GVK)
- **O que resolve**: uma única fonte de verdade de tipos ↔ identidade versionada (GroupVersionKind).
- **Como funciona**: `Scheme` mantém mapas simétricos `gvkToType` e `typeToGVK`; `AddKnownTypes`/`AddUnversionedTypes`; `SchemeBuilder` (`[]func(*Scheme) error`) compõe registro por pacote. `SetVersionPriority` ordena preferência de versão (negociação de content-type).
- **Onde**: `apimachinery/pkg/runtime/scheme.go`, `scheme_builder.go`, `register.go`.
- **Cosca**: registrar os tipos de memória (`MemoryRecord`, `KnowledgeItem`) num Scheme `cosca.ai/memory/v1`, unindo `MemoryType`/`MemoryLayer` (hoje enums soltos) a uma identidade versionada; o family chain (versionamento por commit) mapeia ao eixo `Version` do GVK.

### C2. Conversão hub-and-spoke + defaulting por versão
- **O que resolve**: evoluir schema (N versões) com 2N conversões, não N², e defaults por versão.
- **Como funciona**: cada grupo tem um tipo **internal** (hub) e N **versioned** (spokes); converte `v1→internal→v1beta1` (2N funções). `GroupVersioner` escolhe o GVK alvo (`InternalGroupVersioner`, `MultiGroupVersioner`). Defaulting por versão (`AddTypeDefaultingFunc`) chaveado pelo type versionado. Distingue conversão safe (copia) de unsafe (compartilha memória).
- **Onde**: `runtime/scheme.go:497`, `runtime/codec.go:280`, `conversion/converter.go:198`.
- **Cosca**: `MemoryRecord.Version` hoje é campo inerte — elevá-lo a versão de schema com hub internal + wrappers `v1`/`v2` e defaulting por versão (registros antigos não quebram ao promover). O `versionFamily` do resolver já faz normalização de versão (papel do `GroupVersioner`).

### C3. Validação declarativa (field.ErrorList / field.Path)
- **O que resolve**: validar tudo e reportar tudo (não parar no primeiro erro).
- **Como funciona**: acumula erros em `field.ErrorList` (slice), cada um com `Type` (machine-readable: Required/Invalid/Forbidden/Duplicate/NotSupported/TooLong...), `Field` (caminho completo via `field.Path` imutável: `spec.containers[0].name`), `BadValue`, `Detail`, `Origin`. `ToAggregate()` deduplica. Validação de update recebe `oldObject` para imutabilidade (`ValidateImmutableField`).
- **Onde**: `apimachinery/pkg/util/validation/field/{errors,path}.go`, `api/validation/objectmeta.go`.
- **Cosca**: handlers REST/gRPC devem acumular erros de um `MemoryRecord` num `ErrorList` (melhor UX); `ValidateImmutableField` é o molde para imutabilidade de `CreatedAt/ID/Owner`; o `gate.TransitionTable` (`gate.go:78`) já é validação declarativa de transição — retornar `ErrorList` em vez de `error` único reporta transição inválida + papel não autorizado juntos.

### C4. Admission chain (mutating → validating)
- **O que resolve**: cadeia de plugins de admissão com fase que muta e fase que só valida, com contrato de entrada.
- **Como funciona**: `chainAdmissionHandler` = `[]Interface`; `Admit` itera (mutating, pode alterar o objeto, retorna no primeiro erro) e depois `Validate` (validating, não pode mutar). Cada plugin declara `Handles(operation)`, `ValidateInitialization()`, `Initialize(deps)`. O contrato é `AdmissionAttributes` (kind/namespace/operation/object/oldObject/userInfo/dryRun).
- **Onde**: `apiserver/pkg/admission/{chain,interfaces,attributes,config}.go`.
- **Cosca**: os gates de qualidade da esteira mapeiam direto — **mutating = correções** (normalizar memória, preencher defaults, canonicalizar), **validating = verificações** (pattern tem evidência mínima, bug tem repro). `AdmissionAttributes` (operation/object/oldObject/owner/tenant) unifica a assinatura de todos os gates; `dryRun` mapeia ao modo de previsão.

### C5. Storage abstraction + Versioner + optimistic concurrency
- **O que resolve**: abstrair o backend, optimistic concurrency multi-instância, e filtragem server-side.
- **Como funciona**: `storage.Interface` expõe `Create/Delete/Get/GetList/Watch/GuaranteedUpdate` por `key`+`Object`. `Versioner` gerencia o `resourceVersion` (monotônico). `Preconditions{UID,ResourceVersion}` + `Check` detectam conflito (lost update). `GuaranteedUpdate` faz o retry loop (recarrega e re-aplica a cada conflito). `SelectionPredicate` (Label/Field selectors + `GetAttrs` + `Limit`/`Continue`) filtra/ordena **no backend** em vez de carregar tudo.
- **Onde**: `apiserver/pkg/storage/{interfaces,api_object_versioner,selection_predicate,errors}.go`.
- **Cosca**: o `knowledge.db`/`memory` usa `sync.RWMutex` (lock em processo) mas **sem optimistic concurrency multi-instância**; `MemoryRecord.Version` não é usado para conflito. Adicionar `resourceVersion` por registro + `Preconditions.Check` (detectar lost update entre agentes) + `GuaranteedUpdate` no `GateStore.Move` (evita dois approvers em paralelo) + `SelectionPredicate` no conhecimento (filtrar por índice SQLite, não carregar todos os .md).

### C6. Watch + Filter (event stream com resourceVersion)
- **O que resolve**: stream de eventos reconectável sem perder/repetir.
- **Como funciona**: `watch.Interface` = `Stop()` + `ResultChan() <-chan Event` (`Added/Modified/Deleted/Bookmark/Error`). `Bookmark` carrega só a RV para reconectar sem reprocessar nem perder. RV `"0"` = começar do estado atual como Added; RV não-zero = a partir do próximo evento. `watch.Filter` transforma o stream (drop/transform, não re-tipa).
- **Onde**: `apimachinery/pkg/watch/{watch,filter}.go`.
- **Cosca**: implementar `watch.Interface` sobre o knowledge.db com `resourceVersion`; emitir bookmarks periódicos no `api/stream` (SSE/WebSocket/gRPC) para reconexão; `watch.Filter` para entregar por `Owner`/`Tenant` (segurança/IDOR) e converter para representação versionada; mas a mudança de *visibilidade* exige filtro server-side (`SelectionPredicate`), não no stream.

---

## Synthesis — o que o Cosca deveria copiar (priorizado)

| # | Padrão | Aplicação no Cosca | Ganho |
|---|--------|-------------------|-------|
| 1 | Workqueue keyed + rate-limiting (A2) | `TaskQueue`/`RecoveryLoop` | dedup, backoff exponencial, worker livre no backoff |
| 2 | DeltaFIFO (A1) | watcher → indexer de conhecimento | desacopla fsnotify do indexador lento |
| 3 | Extension points + factory (B1/B6) | hooks por etapa da esteira | elimina `switch` no StepRunner, extensão sem tocar núcleo |
| 4 | Status tipado + short-circuit (B3) | retorno dos steps | `Permanent` com diagnóstico → DoD direto |
| 5 | Fila multi-estágio + retry por eventos (B4) | `taskqueue`/`RecoveryLoop` | backoff correto, sem polling |
| 6 | CycleState (B5) | `TaskStepState` Planner→StepRunner→DoD | desacopla fases |
| 7 | Indexer read model (A4) | `detectDrift` | para de re-replayar o log |
| 8 | Admission chain (C4) | gates de qualidade (mutating/validating) | correções antes de verificações |
| 9 | Storage + resourceVersion (C5) | `knowledge.db`/memory | optimistic concurrency multi-agente |
| 10 | Validação declarativa (C3) | handlers REST/gRPC + gates | erro estruturado e completo |
| 11 | Scheme/GVK + hub-and-spoke (C1/C2) | versionamento de memória | evoluir schema sem quebrar registros |
| 12 | Watch + Bookmark (C6) | `api/stream` | reconexão sem perder/repetir |

## Known Uses (referência)

- `kubernetes/kubernetes` — produção global, o padrão de orquestração em Go (controller-manager, kube-scheduler, apiserver).

## Related Patterns

- [`kubernetes-temporal-ecosystem-patterns.md`](kubernetes-temporal-ecosystem-patterns.md) — visão de ecossistema (informers, workqueue, spec/status, CRD, watch, leader election) em nível mais alto
- [`temporal-workflow-engine-patterns.md`](temporal-workflow-engine-patterns.md) — durable execution / event sourcing (complementa A2/A3)
- [`argocd-gitops-reconciliation-patterns.md`](argocd-gitops-reconciliation-patterns.md) — reconciliação declarativa (complementa A3)
- [`backstage-plugin-platform-patterns.md`](backstage-plugin-platform-patterns.md) — DI + extension points (complementa B1/B6)
- [`vscode-go-extension-patterns.md`](vscode-go-extension-patterns.md) — lifecycle de processo servidor + helper process (complementa A1/A3)
