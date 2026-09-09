# Kubernetes + Temporal Ecosystem Patterns

> **Sources**: `kubernetes/kubernetes` (31K+ files, Go), `temporalio/temporal` (Go), SDKs (Go/Python/TypeScript/Java), `kubernetes/test-infra`, `kubernetes/autoscaler`
> **Analyzed**: 2026-08-09 — Análise cross-agent do ecossistema Kubernetes (API server, controller, scheduler, CRDs, client-go, code-generator, test-infra, autoscaler) + ecossistema Temporal (API, SDKs, ai-cookbook)
> **Confidence**: 0.95 (contratos validados via leitura direta: Object, REST Storage, SharedInformer, Workqueue, Scheduler Framework, Leader Election, CRD types, proto contracts, SDK patterns)

## Intent

Extrair os padrões arquiteturais mais influentes do ecossistema Kubernetes — a plataforma que define como sistemas distribuídos são construídos — e cruzar com os padrões do ecossistema Temporal. Foco nos **contratos** que fazem ambos os ecossistemas escalarem para milhares de contribuidores.

## Context

Kubernetes é o sistema operacional da nuvem, com:
- **API Server**: REST storage interface, admission chain, watch/bookmark mechanism
- **Controller Pattern**: SharedInformer + Lister + Workqueue + Rate-limiting
- **Scheduler Framework**: 10+ extension points com plugin architecture
- **CRD/Extensibility**: CustomResourceDefinition, conversion webhooks, validation via CEL
- **Code Generation**: 11 generators produzindo clients, informers, listers, deepcopy, conversions
- **Multi-version API**: Hub-and-spoke conversion, defaulter per version

## Patterns Extraídos (Top 15 — Síntese Cruzada Kubernetes + Temporal)

### 1. Contract-First + Code Generation Ecosystem

**O que é**: Ambos ecossistemas definem um **contrato canônico** (Kubernetes: Go types + marker comments; Temporal: protobuf definitions) e geram tudo a partir dele. Kubernetes tem **11 generators** (deepcopy-gen, client-gen, informer-gen, lister-gen, conversion-gen, defaulter-gen, applyconfiguration-gen, register-gen, prerelease-lifecycle-gen, go-to-protobuf, validation-gen). Temporal gera gRPC stubs para 6+ linguagens.

**O contrato NUNCA contém lógica específica de linguagem.** O contrato é o tipo Go com `+k8s:deepcopy-gen` ou o `.proto` com `service WorkflowService`. SDKs implementam idioms nativos (Go structs + interfaces, Python decorators + asyncio).

**Por que importa para Cosca**: Nossos agentes, skills, e workflows precisam de um contrato canônico. Definir uma spec (YAML/proto) e gerar adapters para Go (runtime) + TypeScript (dashboard) + Python (scripts) elimina duplicação de código e garante consistência.

**Arquivos-chave**: `staging/src/k8s.io/code-generator/cmd/` (11 generators), `temporalio/api` (proto contract)

---

### 2. SharedInformer + Lister + Workqueue Trinity

**O que é**: O padrão de controller Kubernetes mais copiado do mundo. Três componentes cooperam:

- **SharedInformer**: watch no API server → popula cache local (Store) → notifica handlers
- **Lister**: leitura thread-safe do cache local — nunca chama API
- **Workqueue**: fila assíncrona com dedup, rate-limiting, e graceful shutdown

```go
// O controller canônico:
type DeploymentController struct {
    dLister   appslisters.DeploymentLister   // leitura local, zero API calls
    rsLister  appslisters.ReplicaSetLister   // leitura local
    queue     workqueue.TypedRateLimitingInterface[string] // fila com backoff
    syncHandler func(ctx, key string) error  // injetável para teste
}

// Event handler: só enfileira a key
AddFunc: func(obj) { dc.enqueueDeployment(obj) }

// Worker loop: Get → Sync → Done (ou AddRateLimited no erro)
func worker() {
    for { key, shutdown := dc.queue.Get(); ...
          dc.syncHandler(ctx, key); dc.queue.Done(key) }
}
```

**Garantia de consistência**: `cache.WaitForNamedCacheSync()` bloqueia até o cache local refletir o estado inicial do API server. Workers só começam depois.

**Por que importa para Cosca**: Nossa arquitetura atual é o oposto — agentes chamam APIs diretamente, sem cache local, sem fila. O padrão Informer+Lister+Workqueue daria: cache local do knowledge.db para leituras instantâneas, watch para eventos de mudança, workqueue para processamento assíncrono com retry.

**Arquivos-chave**: `staging/src/k8s.io/client-go/tools/cache/shared_informer.go`, `client-go/util/workqueue/`, `pkg/controller/deployment/deployment_controller.go`

---

### 3. Spec/Status Split + Reconciliation Loop

**O que é**: Todo recurso Kubernetes tem dois campos: `spec` (estado desejado, escrito pelo usuário) e `status` (estado observado, escrito pelo controller). O controller roda um loop infinito: lê spec, observa mundo real, escreve status. Isso é o coração do modelo declarativo.

```
spec (user writes)  ←→  controller (reconciles)  ←→  status (controller writes)
     "3 replicas"              diff + actuate              "2 ready, 1 creating"
```

**Por que importa para Cosca**: Nossos workflows e agentes já seguem esse padrão implicitamente (Don define objetivo → agente executa → kernel reporta), mas não temos a separação explícita spec/status. Formalizar isso daria: `WorkflowSpec` (plano) + `WorkflowStatus` (progresso) + reconciliação contínua até convergência.

---

### 4. Interface Segregation by Capability (REST Storage)

**O que é**: O storage layer decompõe CRUD em interfaces granulares de método único:

```go
type Getter interface { Get(ctx, name, opts) (Object, error) }
type Lister interface { List(ctx, opts) (Object, error) }
type Creater interface { Create(ctx, obj, validation, opts) (Object, error) }
type Updater interface { Update(ctx, name, objInfo, ...) (Object, bool, error) }
type GracefulDeleter interface { Delete(ctx, name, validation, opts) (Object, bool, error) }
type Watcher interface { Watch(ctx, opts) (watch.Interface, error) }
type StandardStorage interface { Getter; Lister; CreaterUpdater; GracefulDeleter; CollectionDeleter; Watcher; Destroy() }
```

O API server usa type assertion em runtime para descobrir quais verbos um recurso suporta. Se um recurso não implementa `Creater`, o POST retorna 405 automaticamente.

**Por que importa para Cosca**: Nossos agentes poderiam implementar interfaces granulares (`TaskRunner`, `HealthChecker`, `StatusReporter`) em vez de uma interface monolítica. O kernel descobre capacidades via type assertion, não via config estático.

**Arquivo-chave**: `staging/src/k8s.io/apiserver/pkg/registry/rest/rest.go`

---

### 5. Two-Phase Admission (Mutate → Validate)

**O que é**: Todo request ao API server passa por uma cadeia de plugins em duas fases: (1) **Mutation** — `Admit(ctx, Attributes)` modifica o objeto (defaults, injeção de sidecar), (2) **Validation** — `Validate(ctx, Attributes)` rejeita se inválido. Ambos implementam `Handles(operation) bool` para filtrar por operação.

**Attributes** que cada plugin recebe: `GetName()`, `GetNamespace()`, `GetResource()`, `GetOperation()` (CREATE/UPDATE/DELETE/CONNECT), `GetObject()`, `GetOldObject()`, `GetUserInfo()`, `IsDryRun()`.

**Por que importa para Cosca**: Nosso pipeline de workflow (planner → build → test → review) é um admission chain! Poderia ser formalizado como plugins que implementam `Validate(step) error` e `Mutate(step) step`, com `Handles(operation)` para filtrar.

**Arquivo-chave**: `staging/src/k8s.io/apiserver/pkg/admission/interfaces.go`

---

### 6. Scheduler Framework — Plugin Extension Points

**O que é**: O scheduler Kubernetes expõe **10+ extension points** bem definidos onde plugins de terceiros se encaixam:

| # | Extension Point | Propósito |
|---|----------------|-----------|
| 1 | `PreEnqueue` | Gate — deve entrar na fila? |
| 2 | `QueueSort` | Ordenação de prioridade |
| 3 | `PreFilter` | Dados específicos do pod, rejeitar cedo |
| 4 | `Filter` | Predicados por nó (NodeUnschedulable, Taints) |
| 5 | `PostFilter` | Preemption em falha |
| 6 | `Score` | Ranquear nós (0-100) |
| 7 | `NormalizeScore` | Normalizar scores com pesos |
| 8 | `Permit` | Gate final (gang scheduling all-or-nothing) |
| 9 | `PreBind` | Setup antes do bind (VolumeBinding) |
| 10 | `Bind` | Atribuir pod ao nó no API server |
| 11 | `PostBind` | Notificar pós-bind |

Cada plugin implementa uma interface específica (`FilterPlugin`, `ScorePlugin`, etc.) e registra no profile. O framework chama todos na ordem.

**Por que importa para Cosca**: Isso é o modelo para nosso workflow engine. Cada passo do workflow (validate, build, test, deploy, notify) seria um extension point onde plugins registram comportamento. O framework chama em ordem, com `CycleState` carregando estado entre fases.

**Arquivo-chave**: `pkg/scheduler/framework/interface.go`

---

### 7. Optimistic Concurrency via ResourceVersion

**O que é**: Toda escrita no etcd carrega `Preconditions{UID, ResourceVersion}`. O método `GuaranteedUpdate()` implementa o loop atômico:

```go
// O storage layer faz:
func GuaranteedUpdate(ctx, key, destination, ignoreNotFound, preconditions, tryUpdate) error {
    for {
        obj, rv := get(key)                     // lê estado atual
        newObj, ttl, err := tryUpdate(obj, rv)  // usuário calcula novo estado
        err := put(key, newObj, Preconditions{rv}) // escreve condicional
        if IsConflict(err) { continue }          // retry em conflito
        return err
    }
}
```

**Tipo `UpdateFunc`**: `func(input Object, res ResponseMeta) (output Object, ttl *uint64, err error)` — recebe estado atual, retorna estado desejado.

**Por que importa para Cosca**: Nossas engines compartilhadas (L154) precisam disso. Em vez de locks, `GuaranteedUpdate` com `Preconditions{ResourceVersion}` garante consistência sem bloquear leitores.

**Arquivo-chave**: `staging/src/k8s.io/apiserver/pkg/storage/interfaces.go`

---

### 8. Watch + Bookmark (Event Stream Infinito)

**O que é**: `watch.Interface` é um contrato minimalista:

```go
type Interface interface {
    Stop()
    ResultChan() <-chan Event
}
type Event struct {
    Type EventType   // ADDED | MODIFIED | DELETED | BOOKMARK | ERROR
    Object Object
}
```

**Bookmark**: Evento sintético contendo apenas `ResourceVersion`, sem objeto. Permite que clients avancem seu `LastSyncResourceVersion` sem receber todos os objetos. Essencial para reconexão eficiente.

**Por que importa para Cosca**: Nosso sistema de eventos é implícito. Um `watch.Interface` permitiria: agentes assistirem mudanças no knowledge.db, dashboard receber atualizações em tempo real, workflows reagirem a eventos externos. Bookmark resolveria o problema de "onde eu parei?" após desconexão.

**Arquivo-chave**: `staging/src/k8s.io/apimachinery/pkg/watch/watch.go`

---

### 9. CRD + Conversion Webhooks (Extensibilidade sem Recompilar)

**O que é**: `CustomResourceDefinition` registra um novo tipo no API server sem recompilar:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
spec:
  group: mygroup.example.com
  names: { plural: myresources, kind: MyResource }
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema: { openAPIV3Schema: {...} }
      subresources: { status: {}, scale: {} }
  conversion:
    strategy: Webhook
    webhook: { clientConfig: { service: {...} } }
```

**Lifecycle**: create → `NamesAccepted` → `Established` → serve CRUD. Com schema validation (OpenAPI v3 → CEL), defaulting, pruning, e conversion webhooks para migração entre versões.

**Por que importa para Cosca**: Nossos 55+ agentes são compilados no binário. O padrão CRD permitiria: definir novos tipos de agente/skill/workflow como configuração (YAML), com validação de schema, sem recompilar o runtime.

**Arquivo-chave**: `staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1/types.go`

---

### 10. Typed Errors as Canonical Status (StatusError)

**O que é**: Todo erro da API Kubernetes é `*StatusError` envolvendo `metav1.Status`:

```go
type StatusError struct { ErrStatus metav1.Status }
// Status: { Status, Message, Reason, Code int32, Details *StatusDetails }
// StatusDetails: { Name, Group, Kind, Causes []StatusCause, RetryAfterSeconds int32 }
```

**30+ construtores** mapeiam padrões HTTP para erros tipados: `NewNotFound(gr, name)`, `NewAlreadyExists(gr, name)`, `NewConflict(gr, name, err)`, `NewInvalid(gk, name, errs)`, `NewForbidden(gr, name, err)`, `NewServerTimeout(...)`, `NewTooManyRequests(...)`.

**Predicados `Is*`**: `IsNotFound(err)`, `IsConflict(err)`, `IsServerTimeout(err)`, etc. — usando `errors.As` para unwrapping. O caller nunca faz string matching no erro.

**Por que importa para Cosca**: Nossos erros são `error` genérico. O padrão `StatusError` + `Is*` predicates daria: erros tipados com contexto (agente, workflow, step), retry hints (`RetryAfterSeconds`), e causas estruturadas (`Causes []StatusCause`).

**Arquivo-chave**: `staging/src/k8s.io/apimachinery/pkg/api/errors/errors.go`

---

### 11. Determinisim Enforcement (Temporal Workflows)

**O que é**: Workflows Temporais devem ser determinísticos — cada replay deve produzir a mesma sequência de eventos. Abordagens por linguagem:

- **Go**: `workflowcheck` static analyzer detecta chamadas não-determinísticas (time.Now(), rand, map iteration)
- **Python**: Sandbox completo — custom `asyncio` event loop que bloqueia imports não-autorizados, `random`, `time`
- **TypeScript**: `determinism` checks em runtime via `@temporalio/workflow` APIs wrapped

**Por que importa para Cosca**: Nossos workflows (L153) não têm garantia de determinismo — se um step produz resultado diferente no retry, o estado diverge. O padrão de sandbox/static-analysis para workflow determinístico evitaria bugs de replay.

**Arquivo-chave**: `temporalio/sdk-go` (`workflowcheck`), `temporalio/sdk-python` (sandbox)

---

### 12. Test Frameworks com Time Control

**O que é**: Ambos ecossistemas fornecem controle de tempo nos testes:

**Kubernetes**: `testing/clock` — `FakeClock` com `SetTime()`, `Step()`, `After()` fake. Timers e workqueues usam `clock.Clock` interface para serem testáveis.

**Temporal**: `TestWorkflowEnvironment` com `RegisterDelayedCallback()`, time skipping, activity mocking. Testes de workflow não esperam tempo real — avançam instantaneamente.

**Por que importa para Cosca**: Nossos testes de workflow não controlam tempo. Testar timeout e retry requer `time.Sleep` real. O padrão `clock.Clock` interface + `FakeClock` eliminaria lentidão e flakiness.

**Arquivos-chave**: `apimachinery/pkg/util/clock/clock.go`, `temporalio/sdk-go/testsuite`

---

### 13. Multi-Dimensional Autoscaling (Independent Concerns)

**O que é**: Kubernetes decompõe autoscaling em componentes independentes:

- **Horizontal Pod Autoscaler**: escala número de pods (CPU/métricas)
- **Vertical Pod Autoscaler**: ajusta CPU/memory requests por pod
- **Cluster Autoscaler**: adiciona/remove nós
- **Balancer**: redistribui pods entre nós por carga

Cada um é um controller separado com seu próprio reconciliation loop. Eles cooperam via spec/status no API server, não via chamadas diretas.

**Por que importa para Cosca**: Nosso "autoscaling" é manual. Para escalar agentes ou workflows automaticamente, o padrão de decompor em dimensões independentes (cognitiva, recursos, paralelismo) com controllers separados é o caminho.

**Arquivo-chave**: `kubernetes/autoscaler` (multi-component architecture)

---

### 14. Leader Election com Lease (Alta Disponibilidade)

**O que é**: Controller managers usam `LeaderElector` baseado em Lease:

```go
type LeaderElectionConfig struct {
    Lock          rl.Interface      // Lease/ConfigMap/Endpoint
    LeaseDuration time.Duration     // 15s — não-líderes esperam isso antes de forçar
    RenewDeadline time.Duration     // 10s — líder renova dentro dessa janela
    RetryPeriod   time.Duration     // 2s — intervalo entre tentativas
    Callbacks     LeaderCallbacks   // OnStartedLeading, OnStoppedLeading, OnNewLeader
}
```

**Não é Raft.** É lease-based: líder renova periodicamente; se falhar, outro candidato adquire o lease após `LeaseDuration`. Simples e eficaz para HA de controllers.

**Por que importa para Cosca**: Single-node hoje. Para HA, leader election com Lease (não Raft) é suficiente — não precisamos de consenso distribuído, só de "quem é o runtime ativo".

**Arquivo-chave**: `staging/src/k8s.io/client-go/tools/leaderelection/leaderelection.go`

---

### 15. AI Workflows como Workflows Duráveis (Temporal ai-cookbook)

**O que é**: O `temporalio/ai-cookbook` demonstra 4 categorias de padrões para AI:

| Categoria | Padrão | Aplicação |
|-----------|--------|-----------|
| **Foundations** | Structured output | LLM → dataclass tipada, persistida |
| **Foundations** | Retry patterns | Retry durável com exponential backoff |
| **Foundations** | Claim-check | Payloads grandes → external storage |
| **Agents** | Agentic loops | Observe→Decide→Act com cada passo gravado |
| **Agents** | Human-in-the-loop | Workflow pausa em gate, espera aprovação |
| **Deep Research** | Multi-agent orchestration | Planner → Researcher → Synthesizer |
| **Deep Research** | Web search parallelism | Fan-out de searches com `asyncio.gather` |
| **MCP** | Durable MCP servers | Tool calls via Model Context Protocol que sobrevivem a restart |

**Meta-padrão**: AI workflows são inerentemente long-running, unreliable, e stateful. O valor do Temporal é convertê-los em steps determinísticos, duráveis, e retryable.

**Por que importa para Cosca**: Somos uma plataforma de AI orchestration. Esses padrões são exatamente o que nossos agentes precisam: structured output dos LLMs, retry durável, multi-agent orchestration, human-in-the-loop como step normal.

**Arquivo-chave**: `temporalio/ai-cookbook` (4 categories, 8+ patterns)

---

## Patterns Não Copiados

| Pattern | Razão |
|---------|-------|
| **etcd como storage** | SQLite + WAL é suficiente para nossa escala |
| **Protobuf como wire format** | JSON/YAML basta; podemos migrar depois |
| **Prow/Tide merge queue** | Não temos CI multi-repo — GitHub Actions basta |
| **CRD conversion webhooks** | Não temos múltiplas versões de API ainda |
| **Server-Side Apply** | Não precisamos de field-level ownership granular |

---

## Confidence Tracking

| # | Pattern | Source | Confidence |
|---|---------|--------|:----------:|
| 1 | Contract-First + Code Generation | K8s + Temporal | 0.95 |
| 2 | SharedInformer + Lister + Workqueue | K8s client-go | 0.96 |
| 3 | Spec/Status + Reconciliation Loop | K8s controller | 0.96 |
| 4 | Interface Segregation (REST Storage) | K8s apiserver | 0.94 |
| 5 | Two-Phase Admission (Mutate→Validate) | K8s apiserver | 0.93 |
| 6 | Scheduler Plugin Extension Points | K8s scheduler | 0.94 |
| 7 | Optimistic Concurrency (ResourceVersion) | K8s etcd storage | 0.95 |
| 8 | Watch + Bookmark | K8s apimachinery | 0.93 |
| 9 | CRD + Conversion Webhooks | K8s apiextensions | 0.92 |
| 10 | Typed Errors (StatusError + Is* predicates) | K8s apimachinery | 0.91 |
| 11 | Determinism Enforcement | Temporal SDKs | 0.89 |
| 12 | Test Frameworks with Time Control | K8s + Temporal | 0.90 |
| 13 | Multi-Dimensional Autoscaling | K8s autoscaler | 0.88 |
| 14 | Leader Election com Lease | K8s client-go | 0.91 |
| 15 | AI Workflows como Workflows Duráveis | Temporal ai-cookbook | 0.87 |

**Average confidence**: ~0.92

---

## Top 5 Para Implementação Imediata na Cosca

1. **SharedInformer + Workqueue** (Pattern 2) — Cache local + fila assíncrona para agentes
2. **Spec/Status + Reconciliation** (Pattern 3) — Formalizar o modelo declarativo no workflow engine
3. **Interface Segregation** (Pattern 4) — Agentes implementarem interfaces granulares, kernel descobre via type assertion
4. **Admission Chain** (Pattern 5) — Formalizar pipeline de workflow como plugins Mutate→Validate
5. **Scheduler Extension Points** (Pattern 6) — Workflow steps como extension points com plugins registráveis

---

> **Nota**: Este documento sintetiza padrões do ecossistema Kubernetes (API server, controller, scheduler, CRDs, client-go) e do ecossistema Temporal (server, SDKs, ai-cookbook). A análise cobre ~30 repositórios, com foco nos contratos e interfaces que definem a arquitetura.
