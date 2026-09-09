# Cosca Compute Fabric — Multi-Core Adaptive Execution Engine

> **Status**: Design | **Author**: Cosca Kernel | **Date**: 2026-07-30
> **Don's Order**: "Multi core, bem sincronizado, analisar capacidade da máquina, agir de acordo sem gargalar"
> **Machine**: 16 cores, 31GB RAM (current dev)

---

## 1. Visão

O Compute Fabric é o sistema nervoso do Cosca — ele sente o hardware, distribui carga entre todos os cores disponíveis, e se adapta em tempo real sem intervenção humana.

**Princípios:**
- **Zero configuração** — detecta hardware automaticamente
- **Elástico** — escala workers up/down baseado em carga real
- **Sincronizado** — todo acesso concorrente passa por primitivas formais
- **Não gargala** — backpressure, rate limiting, memory budgeting
- **Observável** — métricas em tempo real de cada core e worker

---

## 2. Arquitetura

```
┌─────────────────────────────────────────────────────────────────┐
│                    COMPUTE FABRIC                                │
│                                                                  │
│  ┌─────────────────┐     ┌──────────────────────────────────┐   │
│  │ HARDWARE PROBE  │     │     ADAPTIVE SCHEDULER           │   │
│  │                 │     │                                  │   │
│  │ CPU: 16 cores   │────▶│  ┌────────────────────────────┐ │   │
│  │ RAM: 31 GB      │     │  │     WORKER POOLS           │ │   │
│  │ GPU: detect     │     │  │                             │ │   │
│  │ IOPS: measure   │     │  │  Pool "agent"    [8 workers]│ │   │
│  │ Load: realtime  │     │  │  Pool "tool"     [4 workers]│ │   │
│  └─────────────────┘     │  │  Pool "index"    [2 workers]│ │   │
│                          │  │  Pool "io"       [2 workers]│ │   │
│                          │  └────────────────────────────┘ │   │
│                          │                                  │   │
│                          │  ┌────────────────────────────┐ │   │
│                          │  │     WORK STEALING          │ │   │
│                          │  │  Idle workers steal from   │ │   │
│                          │  │  busy queues — zero idle   │ │   │
│                          │  │  cores, maximum throughput │ │   │
│                          │  └────────────────────────────┘ │   │
│                          └──────────────────────────────────┘   │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              EXECUTION ORCHESTRATOR                       │   │
│  │                                                           │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐      │   │
│  │  │ Fan-Out     │  │ Pipeline    │  │ Map-Reduce  │      │   │
│  │  │ (parallel   │  │ (sequential │  │ (distributed│      │   │
│  │  │  scatter)   │  │  stages)    │  │  aggregate) │      │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘      │   │
│  │                                                           │   │
│  │  ┌──────────────────────────────────────────────────┐    │   │
│  │  │            BACKPRESSURE SYSTEM                    │    │   │
│  │  │                                                  │    │   │
│  │  │  ┌──────────┐  ┌──────────┐  ┌──────────────┐  │    │   │
│  │  │  │ Circuit  │  │ Rate     │  │ Memory       │  │    │   │
│  │  │  │ Breaker  │  │ Limiter  │  │ Budget       │  │    │   │
│  │  │  └──────────┘  └──────────┘  └──────────────┘  │    │   │
│  │  └──────────────────────────────────────────────────┘    │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              SYNCHRONIZATION LAYER                        │   │
│  │                                                           │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │   │
│  │  │ errgroup │  │ semaphore│  │ barreira │  │ channels │ │   │
│  │  │ (go std) │  │ (weighted│  │ (sync)   │  │ (buffered│ │   │
│  │  │          │  │  go std) │  │          │  │  select) │ │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘ │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. Hardware Probe (Detector de Capacidade)

```
HardwareProbe {
    CPU:
        PhysicalCores:  16      // runtime.NumCPU()
        LogicalCores:   16      // (com HT, seria 2x)
        Model:          "AMD Ryzen 7" // /proc/cpuinfo
        Frequency:      3.8GHz  // /proc/cpuinfo
        CacheL3:        32MB    // /sys/devices/system/cpu/

    Memory:
        TotalRAM:       31GB    // /proc/meminfo
        AvailableRAM:   23GB    // realtime, atualiza a cada probe
        SwapTotal:      4GB
        PageSize:       4KB

    GPU (opcional):
        Available:      false   // nvidia-smi ou rocm-smi
        Model:          ""
        VRAM:           0

    IO:
        DiskType:       "NVMe"  // /sys/block/
        IOPS:           500K    // benchmark rápido na inicialização
        Throughput:     3.5GB/s

    Load (realtime):
        Load1:          0.5     // /proc/loadavg
        Load5:          0.8
        Load15:         0.6
        CPUUsage:       12%     // /proc/stat, calculado
        MemoryUsage:    15%
}

Métodos:
    Probe() → Snapshot atual
    StartProbing(interval) → Atualiza em background
    RecommendWorkers(poolType) → Número ideal de workers
    CanSpawn(agentType) → bool (tem recurso?)
    HealthReport() → JSON para /health
```

---

## 4. Worker Pools (Multi-Core por Tipo de Tarefa)

```
WorkerPool {
    Name:           "agent"
    Workers:        8           // 50% dos cores para agentes
    MinWorkers:     2           // piso
    MaxWorkers:     12          // teto (75% dos cores)
    QueueSize:      100         // buffer de tasks pendentes
    StealEnabled:   true        // work stealing ativo

    Métricas:
        ActiveWorkers:  8
        IdleWorkers:    2
        QueuedTasks:    15
        CompletedTasks: 1247
        AvgLatency:     340ms
        P99Latency:     1.2s
        StealsPerSecond: 3.4
}

WorkerPool "agent"      → 8 workers  (CPU-bound: raciocínio LLM, routing)
WorkerPool "tool"       → 4 workers  (IO-bound: arquivos, shell, web)
WorkerPool "index"      → 2 workers  (CPU-bound: embeddings, FTS5)
WorkerPool "io"         → 2 workers  (IO-bound: disco, rede)
WorkerPool "sandbox"    → 2 workers  (isolado, bwrap processos)
                               ────
                               18 workers nos 16 cores
                               (alguns IO-bound não saturam core)
```

**Distribuição adaptativa:**
```
Cores disponíveis: 16
├── Reservados para sistema: 2 (sempre)
├── Pool "agent":        max(2, min(12, cores*0.5))   = 8
├── Pool "tool":         max(1, min(6,  cores*0.25))  = 4
├── Pool "index":        max(1, min(4,  cores*0.125)) = 2
├── Pool "io":           max(1, min(2,  cores*0.125)) = 2
└── Pool "sandbox":      max(1, min(4,  cores*0.125)) = 2
```

---

## 5. Work Stealing (Roubo de Tarefa)

Quando um worker fica ocioso, ele rouba tasks da fila de outro pool:

```
Worker A (Pool "agent", idle):
    1. Verifica própria fila → vazia
    2. Tenta roubar do Pool "tool" → tem 1 task → rouba
    3. Executa a task como se fosse do seu pool
    4. Métricas registram o steal

Algoritmo: randomized work stealing (como Go scheduler)
    - Tenta 3 pools aleatórios antes de dormir
    - Prioriza pools do mesmo tipo (CPU-bound rouba de CPU-bound)
    - Se nenhum pool tem trabalho → dorme 100ms → tenta de novo
```

**Benefício:** Nenhum core fica parado enquanto há trabalho em qualquer fila.

---

## 6. Execution Orchestrator (Padrões de Paralelismo)

### Fan-Out (Scatter)
Múltiplos agentes em paralelo, resultados agregados:

```
task := "Analisar cobertura de testes em 5 pacotes"
         │
         ▼
    ┌─────────────────────────────────┐
    │  Fan-Out para 5 workers         │
    │  ┌──────┐ ┌──────┐ ┌──────┐    │
    │  │ pkg1 │ │ pkg2 │ │ pkg3 │ ... │
    │  └──┬───┘ └──┬───┘ └──┬───┘    │
    └─────┼────────┼────────┼─────────┘
          │        │        │
          ▼        ▼        ▼
    ┌─────────────────────────────────┐
    │  Aggregate results              │
    │  "pkg1: 95%, pkg2: 87%, ..."   │
    └─────────────────────────────────┘
```

### Pipeline (Sequential Stages)
Cada stage processa em paralelo dentro de si:

```
Request → [Context Build] → [Route] → [LLM Call] → [Tool Exec] → [Response]
              │                │            │             │
              ▼                ▼            ▼             ▼
         4 workers        2 workers    8 workers     4 workers
         (IO-heavy)      (CPU-light)  (CPU+IO)     (IO-heavy)
```

### Map-Reduce (Aggregate)
Subagentes processam chunks independentes:

```
"Refatorar 10.000 linhas de código"
         │
         ▼
    Map: 8 workers, cada um processa ~1250 linhas
         │
         ▼
    Reduce: 1 worker agrega resultados, resolve conflitos
         │
         ▼
    Commit: 1 worker aplica as mudanças
```

---

## 7. Backpressure System (Anti-Gargalo)

### Circuit Breaker
```
Estado: CLOSED (normal) → OPEN (cortado) → HALF_OPEN (testando)

CircuitBreaker {
    FailureThreshold:   5       // Abre após 5 falhas consecutivas
    SuccessThreshold:   2       // Fecha após 2 sucessos no HALF_OPEN
    Timeout:            30s     // Tempo em OPEN antes de tentar HALF_OPEN

    Por pool:
        Pool "agent":
            State:      CLOSED
            Failures:   0
            LastError:  nil
}
```

### Rate Limiter
```
RateLimiter {
    TokensPerSecond:    100     // Para LLM calls (custo $$)
    BurstSize:          10      // Permite rajadas curtas

    TokensPerSecond:    1000    // Para tool calls (locais)
    BurstSize:          50

    Algoritmo: token bucket por categoria
}
```

### Memory Budget
```
MemoryBudget {
    TotalAvailable:     23GB    // Da probe
    ReservedSystem:     4GB     // Sempre livre pro SO
    ReservedRuntime:    2GB     // Runtime Go + estruturas
    AgentPool:          8GB     // Máximo para agentes
    ToolPool:           4GB     // Máximo para ferramentas
    SessionCache:       2GB     // Cache de sessões
    Headroom:           3GB     // Margem de segurança

    Current:
        AgentUsage:     1.2GB   // Monitorado em tempo real
        ToolUsage:      0.5GB
        SessionUsage:   0.3GB

    Alocação:
        Allocate(agentType, estimatedMB) → bool
        Release(agentType, actualMB)
        WouldExceed(agentType, estimatedMB) → bool
}
```

---

## 8. Synchronization Layer (Primitivas Formais)

Toda concorrência usa primitivas da stdlib + patterns verificados com `-race`:

```go
// Fan-out com errgroup — uma falha cancela todas
g, ctx := errgroup.WithContext(ctx)
for _, task := range tasks {
    task := task
    g.Go(func() error {
        return executeTask(ctx, task)
    })
}
if err := g.Wait(); err != nil {
    // Pelo menos uma falhou
}

// Semáforo ponderado — limita concorrência por peso
sem := semaphore.NewWeighted(8) // 8 "slots" de CPU
for _, task := range tasks {
    if err := sem.Acquire(ctx, task.Weight); err != nil {
        return err
    }
    go func(t Task) {
        defer sem.Release(t.Weight)
        executeTask(ctx, t)
    }(task)
}

// Pipeline com canais bufferizados — backpressure natural
stage1 := make(chan Request, 100)
stage2 := make(chan Processed, 100)
stage3 := make(chan Result, 100)

// Workers drenam canais, canal cheio = backpressure automático
for i := 0; i < workers; i++ {
    go stage1Worker(stage1, stage2)
    go stage2Worker(stage2, stage3)
}

// Barreira — espera todos chegarem antes de prosseguir
var wg sync.WaitGroup
for i := 0; i < numWorkers; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        doPhase1(id)
    }(i)
}
wg.Wait() // Barreira — todos terminaram Phase 1
// Agora todos prosseguem para Phase 2
```

---

## 9. Integração com o Runtime

O Compute Fabric é um **Subsystem** do Runtime — mesma interface, mesmo lifecycle:

```go
// internal/compute/fabric.go
type Fabric struct {
    probe     *HardwareProbe
    scheduler *AdaptiveScheduler
    pools     map[string]*WorkerPool
    breaker   *CircuitBreaker
    limiter   *RateLimiter
    budget    *MemoryBudget
    metrics   *FabricMetrics
}

// Implementa Subsystem interface
func (f *Fabric) Name() string { return "compute-fabric" }
func (f *Fabric) Start(ctx context.Context) error {
    f.probe.StartProbing(5 * time.Second)
    f.scheduler.Start()
    return nil
}
func (f *Fabric) Stop(ctx context.Context) error {
    f.scheduler.Drain()
    f.probe.Stop()
    return nil
}
func (f *Fabric) Health() ComponentStatus { return f.metrics.OverallHealth() }
```

**No lifecycle do Runtime:**
```
Phase Init:
    knowledge → discovery → memory → cache → COMPUTE_FABRIC → plugins → editors → watcher

Phase Start:
    knowledge → discovery → memory → cache → COMPUTE_FABRIC → plugins → editors → watcher

Phase Stop:
    watcher → editors → plugins → COMPUTE_FABRIC → cache → memory → discovery → knowledge
```

**Uso pelos agentes:**
```go
// Agente pede permissão ao Fabric antes de spawnar
if !fabric.CanSpawn("cosca-database") {
    return ErrResourceExhausted
}

// Agente submete task ao pool correto
result, err := fabric.Submit(ctx, Task{
    Type:    "agent",
    Weight:  2,          // 2 "slots" de CPU (task pesada)
    Budget:  512 * MB,   // reserva 512MB
    Timeout: 30 * time.Second,
    Func:    func(ctx context.Context) (interface{}, error) {
        return agent.Execute(ctx, prompt)
    },
})

// Ou usa fan-out para paralelismo automático
results := fabric.FanOut(ctx, tasks, FanOutConfig{
    MaxConcurrency: 8,
    Timeout:        60 * time.Second,
})
```

---

## 10. Métricas em Tempo Real

```
$ cosca fabric status
┌─────────────────────────────────────────────────────────────┐
│                    COMPUTE FABRIC                            │
│                                                              │
│  Hardware:                                                    │
│    CPU: 16 cores (AMD Ryzen 7)    Load: 0.5 0.8 0.6         │
│    RAM: 23/31 GB available        Usage: 15%                 │
│    GPU: none                       IOPS: 500K                │
│                                                              │
│  Worker Pools:                                                │
│    agent:    ████████░░  8/12 workers   Queue: 3             │
│    tool:     ████░░░░░░  4/6  workers   Queue: 0             │
│    index:    ██░░░░░░░░  2/4  workers   Queue: 0             │
│    io:       ██░░░░░░░░  2/2  workers   Queue: 1             │
│    sandbox:  ██░░░░░░░░  2/4  workers   Queue: 0             │
│                                                              │
│  Throughput:                                                  │
│    Tasks/sec: 47.3     Steals/sec: 3.4                       │
│    Avg latency: 340ms  P99 latency: 1.2s                     │
│                                                              │
│  Backpressure:                                                │
│    Circuit breakers: all CLOSED                               │
│    Rate limiter: 47/100 tokens (agent pool)                  │
│    Memory: 2.0/17 GB used (agent+tools)                      │
└─────────────────────────────────────────────────────────────┘
```

---

## 11. Algoritmo de Escalonamento Adaptativo

```
A cada 5 segundos (intervalo configurável):

1. PROBE: Atualiza HardwareProbe (CPU load, RAM available)

2. ANALISA:
   loadFactor = avg(load1, load5, load15) / numCores
   queueDepth = soma de todas as filas
   avgLatency = média móvel das últimas 100 tasks

3. DECIDE:
   if loadFactor < 0.5 && queueDepth > 0:
       → SCALE UP: +1 worker no pool mais sobrecarregado

   if loadFactor > 0.8:
       → SCALE DOWN: -1 worker no pool menos utilizado

   if avgLatency > threshold:
       → ALERTA: pool "agent" está saturado

   if memoryUsage > 80%:
       → BACKPRESSURE: recusar novas tasks, drenar fila

   if circuitBreaker == OPEN:
       → RECOVERY: tentar HALF_OPEN após timeout

4. EXECUTA: Aplica as decisões (spawn/kill workers)
```

---

## 12. Garantias de Sincronização

| Garantia | Como |
|----------|------|
| **Zero race conditions** | Todo acesso a estado compartilhado usa `sync.Mutex`/`sync.RWMutex`. CI roda com `-race`. |
| **Zero deadlocks** | Lock ordering definido: fabric.mu → pool.mu → worker.mu. Sempre na mesma ordem. |
| **Zero goroutine leaks** | Todo `go func()` tem `defer wg.Done()` e escuta `ctx.Done()`. |
| **Graceful shutdown** | `Drain()` sinaliza workers, espera tasks em andamento terminarem (com timeout). |
| **Fair scheduling** | Cada pool tem seu próprio channel. Work stealing é randomizado. Nenhum pool morre de fome. |
| **Deterministic fan-out** | `FanOut` espera TODOS os workers terminarem ou primeiro erro (com `errgroup`). |

---

## 13. API Pública

```go
// Submeter uma task (bloqueante ou com contexto)
func (f *Fabric) Submit(ctx context.Context, task Task) (Result, error)

// Submeter múltiplas tasks em paralelo
func (f *Fabric) FanOut(ctx context.Context, tasks []Task, cfg FanOutConfig) ([]Result, error)

// Pipeline sequencial com paralelismo interno em cada stage
func (f *Fabric) Pipeline(ctx context.Context, stages []PipelineStage, input interface{}) (Result, error)

// Verificar se há recurso disponível
func (f *Fabric) CanSpawn(agentType string) bool

// Reservar memória para um agente
func (f *Fabric) AllocateMemory(agentType string, bytes int64) bool
func (f *Fabric) ReleaseMemory(agentType string, bytes int64)

// Status em tempo real
func (f *Fabric) Status() FabricStatus
func (f *Fabric) Metrics() FabricMetrics

// Lifecycle (Subsystem interface)
func (f *Fabric) Name() string
func (f *Fabric) Start(ctx context.Context) error
func (f *Fabric) Stop(ctx context.Context) error
func (f *Fabric) Health() ComponentStatus
```

---

## 14. Plano de Implementação (Fase independente no workflow)

### Fase 2.5: Compute Fabric

| # | Task | Agent | Linhas estimadas |
|---|------|-------|-----------------|
| CF.1 | `internal/compute/hardware.go` — HardwareProbe com CPU, RAM, GPU, IOPS, load | cosca-backend | ~300 |
| CF.2 | `internal/compute/pool.go` — WorkerPool com spawn/kill/steal | cosca-backend | ~400 |
| CF.3 | `internal/compute/scheduler.go` — AdaptiveScheduler com algoritmo de scale | cosca-backend | ~300 |
| CF.4 | `internal/compute/breaker.go` — CircuitBreaker por pool | cosca-backend | ~150 |
| CF.5 | `internal/compute/limiter.go` — RateLimiter token bucket | cosca-backend | ~100 |
| CF.6 | `internal/compute/budget.go` — MemoryBudget com allocate/release | cosca-backend | ~200 |
| CF.7 | `internal/compute/fabric.go` — Fabric integrando tudo (Subsystem interface) | cosca-backend | ~300 |
| CF.8 | `internal/compute/orchestrator.go` — FanOut, Pipeline, Map-Reduce | cosca-backend | ~300 |
| CF.9 | Integrar no Runtime lifecycle (init/start/stop hooks) | cosca-architecture | ~50 |
| CF.10 | Testes unitários (cada componente) + integração (Fabric completo) | cosca-testing | ~2000 |
| CF.11 | Testes de concorrência com `-race` + `-count=100` | cosca-testing | ~500 |
| CF.12 | Métricas exportáveis (Prometheus + CLI `fabric status`) | cosca-backend | ~200 |

**Total estimado:** ~4.500 linhas de código + ~2.500 linhas de teste = ~7.000 linhas
