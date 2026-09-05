# Temporal Workflow Engine Patterns

> **Source**: https://github.com/temporalio/temporal — MIT License, Go 1.26, monorepo  
> **Analyzed**: 2026-08-09 — Análise profunda cross-agent (3 agentes em paralelo)  
> **Confidence**: 0.95 (validado por leitura direta do código fonte: ~300+ arquivos-chave, 30+ interfaces críticas)

## Intent

Extrair padrões reutilizáveis do motor de orquestração de workflows mais avançado do mundo — event sourcing, durable execution, task pipeline, shard-based partitioning, HSM, DI, multi-tenancy. Aplicação direta na esteira autônoma Cosca (L153).

## Context

O Temporal é o padrão ouro de orquestração de workflows duráveis — usado por Uber, Netflix, Stripe, Snap, Coinbase, Datadog. É a evolução do Cadence (Uber) com:
- **Go 1.26** — server monorepo, ~25 diretórios de domínio
- **4 serviços internos**: Frontend (gRPC API), History (execução + event sourcing), Matching (dispatch de tasks), Worker (background processing)
- **Persistência**: Cassandra, MySQL, PostgreSQL — event sourcing com blob storage
- **gRPC**: API pública estável (`temporal.api.workflowservice.v1`) + APIs internas entre serviços
- **Multi-cluster**: replicação cross-DC com version history e LCA (Lowest Common Ancestor)
- **HSM/CHASM**: sistema de componentes com state machines hierárquicas para extensão da plataforma

## Patterns Extraídos (Top 12 para Cosca)

### 1. Event Sourcing + Replay (Fundação da Execução Durável)

**O que é**: Todo estado de workflow é derivado de uma sequência imutável de `HistoryEvent` protobuf. O estado atual (`MutableStateImpl`) é uma **view materializada** construída aplicando cada evento via métodos `Apply*`. Se o servidor cair, o novo host faz replay dos eventos do banco e reconstrói o estado idêntico.

**Eventos principais** (30+ tipos):
```
WORKFLOW_EXECUTION_STARTED  → cria execution info, configura timeouts
WORKFLOW_TASK_SCHEDULED     → workflow task entra na fila
WORKFLOW_TASK_STARTED       → worker recebeu a task
WORKFLOW_TASK_COMPLETED     → worker devolveu comandos
ACTIVITY_TASK_SCHEDULED     → activity entra na fila
ACTIVITY_TASK_STARTED       → worker iniciou activity
ACTIVITY_TASK_COMPLETED     → activity concluída com resultado
TIMER_STARTED/FIRED/CANCELED → timers duráveis que sobrevivem a crash
CHILD_WORKFLOW_EXECUTION_STARTED/COMPLETED/FAILED
WORKFLOW_EXECUTION_COMPLETED/FAILED/CANCELED/TERMINATED/TIMED_OUT/CONTINUED_AS_NEW
```

**Replay**: `MutableStateRebuilder.ApplyEvents()` itera lotes de eventos e chama `Apply*` para cada tipo. Após replay, `taskGenerator.Generate*Tasks()` reconstrói as tasks pendentes a partir do estado reconstruído.

**Por que importa para Cosca**: Nossa esteira (L153) tem TaskPlanner → StepRunner → RecoveryLoop, mas NÃO TEM persistência de estado entre passos. Se o runtime cair no meio de um workflow, perde-se tudo. O padrão event sourcing resolve: cada passo gera um evento imutável, o estado é replay, e a execução sobrevive a qualquer crash.

**Arquivos-chave**: `service/history/workflow/mutable_state_impl.go` (9.930 linhas), `service/history/workflow/mutable_state_rebuilder.go`

---

### 2. Staged Execution (Decisão → Evento → Task → Side Effect)

**O que é**: O workflow NÃO executa side effects diretamente. Em vez disso: (1) o worker toma decisões e emite **comandos** (`ScheduleActivity`, `StartTimer`, `CompleteWorkflow`), (2) o history engine transforma comandos em **eventos** imutáveis, (3) o `TaskGenerator` deriva **tasks** dos eventos, (4) as tasks são processadas **assincronamente** pelos queue processors. Isso significa que a decisão termina rápido (só escreve eventos), e os efeitos colaterais acontecem depois.

**Pipeline completo**:
```
Worker completa WorkflowTask → commands[]
  → CommandHandlerRegistry processa cada command
    → MutableState.Add*Event() gera eventos
    → TaskGenerator.Generate*Tasks() deriva tasks
  → CloseTransactionAsMutation() persiste eventos + estado + tasks atomicamente
  → engine.NotifyNewTasks() envia tasks aos queue processors
    → TransferQueue: envia WorkflowTask/ActivityTask ao Matching
    → TimerQueue: agenda UserTimerTask para disparar no futuro
    → VisibilityQueue: escreve search attributes
```

**Por que importa para Cosca**: Nosso StepRunner atual é síncrono — o step executa, gera resultado, próximo step. O padrão staged execution desacopla decisão de execução: o planner decide o que fazer, gera eventos, e tasks derivadas executam assincronamente. Permite paralelismo real entre steps independentes.

**Arquivos-chave**: `service/history/api/respondworkflowtaskcompleted/workflow_task_completed_handler.go`, `service/history/workflow/task_generator.go`, `service/history/workflow/command_handler.go`

---

### 3. Task Queue como Buffer Durável (Matching System)

**O que é**: Tasks não são entregues diretamente aos workers. O Matching Service as **persiste no banco** quando não há worker disponível (`SpoolTask`). Quando um worker faz poll, as tasks são lidas do banco (`getTasksPump`) e entregues. O `taskAckManager` rastreia quais tasks foram entregues mas não completadas. Se o worker cair, a task volta pra fila após timeout.

**Arquitetura de Partições**:
```
TaskQueueFamily (namespace + nome)
  ├── Partition 0 (root) — recebe AddTask, gerencia UserData
  ├── Partition 1..N (children) — criadas dinamicamente para scale-out
  │     └── Physical Queues (unversioned + per-build-id versioned)
  └── Forwarder — child→parent para tasks/polls excedentes
```

**Sticky Execution**: Workers mantêm um cache de workflow state. Tasks são roteadas para o mesmo worker via `TASK_QUEUE_KIND_STICKY` por 10s após a última task — latência mínima para workflows com múltiplas decisões.

**Por que importa para Cosca**: Nosso workflow engine não tem buffer durável — tasks são criadas em memória e perdidas no crash. O padrão Matching + SpoolTask + AckManager garante entrega exatamente-once com persistência. As partições dinâmicas permitem scale-out horizontal dos workers.

**Arquivos-chave**: `service/matching/matching_engine.go`, `service/matching/backlog_manager.go`, `service/matching/task_reader.go`, `service/matching/task_writer.go`

---

### 4. Optimistic Concurrency com Conditional Updates

**O que é**: Toda escrita no banco é condicional em `NextEventID` + `DBRecordVersion`. Se outro writer modificou o workflow desde que o carregamos, a condição falha → `WorkflowConditionFailedError` → reload + retry. Sem locks distribuídos, sem 2PC.

```go
// MutableStateImpl carrega o estado "último visto"
nextEventIDInDB int64   // nextEventID que estava no DB quando carregou
dbRecordVersion int64   // versão do registro no DB

// Na persistência, estes viram a condição:
Condition:       ms.nextEventIDInDB,
DBRecordVersion: ms.dbRecordVersion,
```

**Multi-camada de proteção**:
1. **Workflow Cache Lock**: lock exclusivo por workflow execution (in-memory)
2. **Optimistic Condition**: NextEventID + DBRecordVersion como CAS
3. **Dedup por RequestID**: `MutableStateImpl.appliedEvents` rastreia quais requestIDs já foram processados
4. **Vector Clock**: `WorkflowConsistencyChecker` compara o vector clock do request com o do shard

**Por que importa para Cosca**: Nosso `bootstrap.Compose()` cria engines compartilhadas sem controle de concorrência. Se dois workflows tentarem modificar o mesmo conhecimento simultaneamente, temos race condition. O padrão de conditional update com optimistic locking + dedup por request ID garante exactly-once sem locks pesados.

**Arquivos-chave**: `service/history/workflow/mutable_state_impl.go`, `service/history/api/consistency_checker.go`, `common/persistence/data_interfaces.go`

---

### 5. Shard-Based Partitioning com RangeID Fencing

**O que é**: Workflows são hasheados para shards numerados 1..N (tipicamente 512-4096). Cada shard é owned por **exatamente um history host** por vez. O ownership é protegido por um `RangeID` — inteiro monotônico que funciona como fencing token. Quando um host assume um shard, incrementa o RangeID. Todas as escritas do host antigo falham porque o RangeID está stale → `ShardOwnershipLostError`.

**Graceful Degradation (Shard Linger)**: Quando um host perde ownership, ele NÃO fecha o shard imediatamente. Entra em "linger" por ~1 minuto, continuando a servir requests enquanto verifica se o novo owner já assumiu. Isso evita dropping de trabalho em voo durante rebalance.

**Membership via Ringpop**: Hash ring consistente com SWIM gossip protocol. Heartbeat timeout de 20s. Quando um host cai, os sobreviventes detectam e rebalanceiam shards automaticamente.

**Por que importa para Cosca**: Nossa arquitetura atual é single-node — runtime é um processo, serve é outro. Para scale-out horizontal, o padrão shard + RangeID fencing é o modelo canônico: cada instância de runtime é dona de um subconjunto de workflows, RangeID previne split-brain, ringpop gerencia membership.

**Arquivos-chave**: `service/history/shard/controller_impl.go`, `service/history/shard/context_impl.go`, `common/persistence/shard_manager.go`, `common/membership/ringpop/monitor.go`

---

### 6. Checksum-Based Nondeterminism Detection

**O que é**: A cada `CloseTransactionAsMutation`, um CRC32 é computado sobre o mutable state completo (nextEventID, activity count, timer IDs, version histories, CHASM paths). O checksum é persistido com o registro. No `Load()`, o checksum é recalculado e comparado. Divergência → `MutableStateChecksumMismatch` metric + log warning.

**Por que isso é crítico**: Workflows determinísticos são a base do replay. Se um bug faz o estado divergir entre execução original e replay, o workflow corrompe. O checksum pega: bugs no Apply*, corrupção de dados, updates parciais.

**Por que importa para Cosca**: Nosso Recovery Loop (L153) não tem detecção de divergência de estado. Se um step falha e o retry produz um estado diferente, não sabemos. Checksum sobre o estado do workflow + validação no reload fecha essa brecha.

**Arquivos-chave**: `service/history/workflow/checksum.go`

---

### 7. Hierarchical State Machine (HSM) — Sistema de Componentes Universal

**O que é**: Sistema de extensão da plataforma onde componentes registram: (1) `StateMachineDefinition` (tipo string + estados + transições), (2) `TaskSerializer`, (3) `ImmediateExecutor` ou `TimerExecutor`, (4) `EventDefinition`. Tasks carregam um `StateMachineRef` (path na árvore + versioned transitions) para detecção de staleness.

**Contrato**:
```go
type StateMachine[S comparable] interface {
    Type() string
    State() S
    SetState(S)
    TransitionCount() int32
    AddVersionedTransitionCount(int32)
}

type Transition[S, SM, E any] struct {
    Source S
    Machine SM
    Action E
    Destination S
}
```

**CHASM (Component Handler Abstraction State Machine)**: Sistema paralelo mais novo com `chasm.Engine`, `chasm.Component`, `chasm.Registry`. Libraries (coleções de componentes) registram-se por nome. CHASM tem seu próprio sistema de visibility, search attributes, e Nexus operation processor.

**Por que importa para Cosca**: Nossos 55+ agentes + skills + workflows são componentes, mas sem contrato uniforme. O padrão HSM dá a cada agente uma state machine explícita com transições validadas + tasks serializáveis + execução assíncrona. O `StateMachineRef` com versioned transitions garante que tasks stale (de versões antigas do estado) sejam detectadas e descartadas.

**Arquivos-chave**: `service/history/hsm/sm.go`, `service/history/hsm/registry.go`, `service/history/hsm/executor.go`, `chasm/registry.go`, `chasm/component.go`

---

### 8. fx Dependency Injection para Serviços Componíveis

**O que é**: Uber's `fx` library fornece late-binding de interfaces com lifecycle management. Cada serviço é um `fx.Module`. O top-level `TopLevelModule` compõe tudo:

```go
TopLevelModule = fx.Options(
    fx.Provide(NewServerFxImpl, ServerOptionsProvider, ...),
    dynamicconfig.Module,
    chasm.Module,
    serialization.Module,
    fx.Provide(HistoryServiceProvider),
    fx.Provide(MatchingServiceProvider),
    fx.Provide(FrontendServiceProvider),
    fx.Invoke(ServiceLifetimeHooks),
)
```

**Group Injection**: Múltiplas implementações de uma interface são coletadas via `group:"queueFactory"`:
```go
type HistoryEngineFactoryParams struct {
    fx.In
    QueueFactories []QueueFactory `group:"queueFactory"`
}
```

**Por que importa para Cosca**: Já identificamos esse padrão no Backstage (Pattern 2 do L164). O Temporal mostra a versão Go desse padrão com fx. Nosso `bootstrap.Compose()` (L154) poderia ser refatorado para um container fx onde engines são `fx.Provide()` e agentes são `fx.Invoke()`.

**Arquivos-chave**: `temporal/fx.go` (1.294 linhas), `service/history/fx.go`, `service/fx.go`

---

### 9. Dynamic Configuration com Constraint Precedence

**O que é**: Sistema de configuração tipada onde cada setting tem um tipo (`BoolPropertyFn`, `IntPropertyFn`, `DurationPropertyFn`) e uma precedência de constraints: `TaskQueue+Namespace → TaskQueue → Namespace → Global`. Settings podem mudar em runtime sem restart.

**Contrato**:
```go
type Client interface {
    GetValue(key Key) []ConstrainedValue
}
// ConstrainedValue: {Value, Constraints{Namespace, TaskQueueName, ShardID}}
```

**3.650+ settings** definidas em `common/dynamicconfig/constants.go`. Clientes implementam `NotifyingClient` com `Subscribe(ClientUpdateFunc)` para push-based updates.

**Por que importa para Cosca**: Nosso `config.yaml` é estático — qualquer mudança exige restart. O padrão DynamicConfig permitiria ajustar parâmetros de agentes, timeouts de workflow, limites de rate, tudo em runtime. O modelo de constraints (global → namespace → task queue) é o padrão para scoping de configuração.

**Arquivos-chave**: `common/dynamicconfig/client.go`, `common/dynamicconfig/setting.go`, `common/dynamicconfig/constants.go`

---

### 10. Timer Durável (Event Sourcing, Não Goroutine)

**O que é**: Timers NÃO são `time.After()` em goroutines. São `UserTimerTask` com `VisibilityTimestamp` persistidos no banco. O `TimerQueueProcessor` ordena tasks por timestamp e só processa quando o tempo chega. Se o servidor cair, o novo host carrega os timers do banco e continua de onde parou.

**TimerSequence**: Gerencia a relação entre eventos (`TIMER_STARTED`, `TIMER_FIRED`, `TIMER_CANCELED`) e tasks (`UserTimerTask`), rastreando quais timers foram criados/disparados/cancelados para cada workflow execution.

**Por que importa para Cosca**: Nossos workflows (L153) não têm timers duráveis. Um step que depende de timeout (ex: "esperar 5min e retentar") morre com o processo. O padrão de timer como task persistida com VisibilityTimestamp garante que timeouts sobrevivam a qualquer crash.

**Arquivos-chave**: `service/history/tasks/user_timer.go`, `service/history/workflow/timer_sequence.go`, `service/history/timer_queue_active_task_executor.go`

---

### 11. Multi-Tenancy via Namespace com Replicação Cross-DC

**O que é**: Namespaces são limites de isolamento com: `NamespaceInfo` (nome, ID, estado), `NamespaceConfig` (retention, archival), `NamespaceReplicationConfig` (clusters, failover). Cada namespace tem seu próprio conjunto de search attributes, visibility store, e archival config.

**Replicação**: Namespaces globais (`failover_version != 0`) replicam entre clusters. `ReplicationResolver.ActiveClusterName(RoutingKey)` determina qual cluster processa. `VersionHistory` rastreia ranges de eventos por cluster version. O **LCA (Lowest Common Ancestor)** entre duas histórias divergentes resolve conflitos.

**Por que importa para Cosca**: Hoje somos single-tenant — um runtime, um knowledge.db, um Don. Para SaaS multi-cliente, o padrão namespace isola workloads, dados, e configuração. O VersionHistory + LCA é o padrão para replicação multi-cluster sem conflito.

**Arquivos-chave**: `common/namespace/registry.go`, `common/namespace/replication_resolver.go`, `common/persistence/versionhistory/version_history.go`

---

### 12. Callback & Nexus Component System (Extensibilidade de Plataforma)

**O que é**: Dois sistemas de componentes que estendem a plataforma sem modificar o core:

**Callbacks**: State machine `"callbacks.Callback"` com estados STANDBY → SCHEDULED → BACKING_OFF → terminal. Triggers: `WorkflowClosed`. Tasks: `InvocationTask` (HTTP call), `BackoffTask` (retry timer). Executor usa `HTTPCallerProvider` para rotear HTTP calls.

**Nexus Operations**: State machines `"nexusoperations.Operation"` + `"nexusoperations.Cancelation"`. Tasks: `InvocationTask`, `CancelationTask`, `BackoffTask`, `TimeoutTask`. Permite que workflows chamem serviços externos via protocolo Nexus.

**Por que importa para Cosca**: Nossos agentes são hardcoded — adicionar um novo tipo de agente exige código no core. O padrão de componente registrável (callback, nexus operation) mostra como agentes poderiam registrar-se em runtime: definem sua state machine, tasks, e executor, e o motor de workflow os invoca automaticamente.

**Arquivos-chave**: `components/callbacks/fx.go`, `components/nexusoperations/fx.go`, `chasm/lib/nexusoperation/`

---

## Patterns Não Copiados

| Pattern | Razão |
|---------|-------|
| **Cassandra LWT** | Não precisamos de Cassandra — SQLite/WAL + PostgreSQL bastam |
| **Ringpop SWIM gossip** | Overkill para single-node; membership pode ser mais simples (Raft, etcd) |
| **Dual HSM/CHASM** | Ter dois sistemas de componentes paralelos é dívida técnica de migração — ficamos com um só |
| **Nexus protocol** | Protocolo específico do Temporal para comunicação inter-service |
| **Archival S3/GCS** | Arquivamento é importante mas não prioritário para MVP da esteira |
| **Visibility Store separado** | Nossa busca já tem FTS5 + vector — não precisamos de Elasticsearch separado |

---

## Confidence Tracking

| # | Pattern | Confidence | Validated By |
|---|---------|:----------:|--------------|
| 1 | Event Sourcing + Replay | 0.96 | Fundação de toda plataforma, 9.930 linhas MutableStateImpl |
| 2 | Staged Execution (Decisão→Evento→Task) | 0.95 | Pipeline completo em produção Netflix/Uber |
| 3 | Task Queue como Buffer Durável | 0.93 | Matching engine com partições + backlog |
| 4 | Optimistic Concurrency com Conditional Updates | 0.95 | Multi-camada de proteção em toda API de persistência |
| 5 | Shard Partitioning + RangeID Fencing | 0.94 | Shard controller + membership ring |
| 6 | Checksum Nondeterminism Detection | 0.90 | Presente em toda mutation |
| 7 | Hierarchical State Machine (HSM) | 0.91 | Sistema de componentes universal |
| 8 | fx Dependency Injection | 0.93 | Top-level + todos serviços |
| 9 | Dynamic Configuration | 0.92 | 3.650+ settings, runtime reload |
| 10 | Timer Durável via Event Sourcing | 0.93 | TimerQueue + TimerSequence |
| 11 | Multi-Tenancy + Replicação | 0.91 | Namespace registry + VersionHistory |
| 12 | Callback/Nexus Components | 0.87 | Extensibilidade de plataforma |

**Average confidence**: ~0.92

---

## Top 5 Para Implementação Imediata na Cosca

1. **Event Sourcing + Replay** (Pattern 1) — Persistir cada passo do workflow como evento imutável
2. **Staged Execution** (Pattern 2) — Desacoplar decisão (planner) de execução (side effects)
3. **Task Queue Durável** (Pattern 3) — Tasks persistidas, entregues com ack, re-entregues em crash
4. **Optimistic Concurrency** (Pattern 4) — NextEventID como CAS token nas engines compartilhadas
5. **Timer Durável** (Pattern 10) — Timeouts de workflow como tasks persistidas, não goroutines

---
