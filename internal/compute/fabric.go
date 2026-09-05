package compute

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// Fabric — Multi-Core Adaptive Execution Engine
// =============================================================================

// Fabric is the central orchestrator for multi-core execution.
// It implements the runtime.Subsystem interface for lifecycle management.
type Fabric struct {
	mu      sync.RWMutex
	probe   *HardwareProbe
	pools   map[string]*WorkerPool
	cfg     FabricConfig
	running bool

	// GPU executor (nil por default — pipeline GPU dorme até ser configurado).
	gpu GPUExecutor

	// Backpressure
	breaker *CircuitBreaker
	limiter *RateLimiter
	budget  *MemoryBudget

	// Orquestration
	orchestrator *ExecutionOrchestrator
}

// FabricConfig configures the Compute Fabric.
//
// The *Percent fields are fractions of the detected logical cores allocated
// to each worker pool. They are enforced in Start() (via RecommendWorkers),
// so changing them changes the pool ceilings proportionally on any machine.
// The defaults below reproduce the historical per-core multipliers
// (0.75/0.375/0.25/0.25/0.25) while scaling up on larger machines.
type FabricConfig struct {
	ProbeInterval  time.Duration // How often to sample hardware (default: 5s)
	CPUPercent     float64       // % of cores for agent pool (default: 0.75)
	ToolPercent    float64       // % of cores for tool pool (default: 0.375)
	IndexPercent   float64       // % of cores for index pool (default: 0.25)
	IOPercent      float64       // % of cores for IO pool (default: 0.25)
	SandboxPercent float64       // % of cores for sandbox pool (default: 0.25)
}

// DefaultFabricConfig returns sensible defaults tuned for a typical dev machine.
// The multipliers match the historical hardcoded scaling (agent 0.75*cores,
// tool 0.375*cores, index/io/sandbox 0.25*cores), now expressed as config.
func DefaultFabricConfig() FabricConfig {
	return FabricConfig{
		ProbeInterval:  5 * time.Second,
		CPUPercent:     0.75,  // agent (histórico: 0.75*cores)
		ToolPercent:    0.375, // tool (histórico: 0.375*cores)
		IndexPercent:   0.25,
		IOPercent:      0.25,
		SandboxPercent: 0.25,
	}
}

// LoadFabricConfig returns DefaultFabricConfig() overlaid with the
// COSCA_FABRIC_*_PERCENT environment variables. Each override must parse as a
// float in the open interval (0, 1.0]; invalid or missing values keep the
// default. This is how operators tune pool sizing without recompiling.
func LoadFabricConfig() FabricConfig {
	cfg := DefaultFabricConfig()
	cfg.CPUPercent = envPercent("COSCA_FABRIC_CPU_PERCENT", cfg.CPUPercent)
	cfg.ToolPercent = envPercent("COSCA_FABRIC_TOOL_PERCENT", cfg.ToolPercent)
	cfg.IndexPercent = envPercent("COSCA_FABRIC_INDEX_PERCENT", cfg.IndexPercent)
	cfg.IOPercent = envPercent("COSCA_FABRIC_IO_PERCENT", cfg.IOPercent)
	cfg.SandboxPercent = envPercent("COSCA_FABRIC_SANDBOX_PERCENT", cfg.SandboxPercent)
	return cfg
}

// envPercent parses an environment variable as a pool size percentage
// (0 < v <= maxPercent). On parse error or out-of-range it returns the
// provided default, so a bad override never breaks the fabric.
//
// Values ABOVE 1.0 are allowed (over-subscription): pools of I/O-bound work
// (index/io/sandbox) and the adaptive scheduler benefit from ceilings larger
// than the core count — workers waiting on I/O don't consume CPU. The hard
// cap is maxPercent (default 4.0 = 4× cores); the maxSafetyCeiling in Start()
// remains the final anti-abuse guard.
func envPercent(key string, def float64) float64 {
	return envPercentMax(key, def, 4.0)
}

// envPercentMax é o núcleo de envPercent com teto configurável (para testes).
func envPercentMax(key string, def, maxPercent float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 || v > maxPercent {
		return def
	}
	return v
}

// NewFabric creates a new Compute Fabric.
func NewFabric(cfg FabricConfig) *Fabric {
	f := &Fabric{
		probe:   NewHardwareProbe(cfg.ProbeInterval),
		pools:   make(map[string]*WorkerPool),
		cfg:     cfg,
		breaker: NewCircuitBreaker(),
		limiter: NewRateLimiter(),
		budget:  NewMemoryBudget(),
	}

	// Create pools — sizes will be adjusted during Start() based on actual hardware.
	f.pools["agent"] = NewWorkerPool(PoolConfig{
		Name: "agent", MinWorkers: 2, MaxWorkers: 12, QueueSize: 100,
		IdleTimeout: 30 * time.Second,
	})
	f.pools["tool"] = NewWorkerPool(PoolConfig{
		Name: "tool", MinWorkers: 1, MaxWorkers: 6, QueueSize: 100,
		IdleTimeout: 30 * time.Second,
	})
	f.pools["index"] = NewWorkerPool(PoolConfig{
		Name: "index", MinWorkers: 1, MaxWorkers: 4, QueueSize: 50,
		IdleTimeout: 60 * time.Second,
	})
	f.pools["io"] = NewWorkerPool(PoolConfig{
		Name: "io", MinWorkers: 1, MaxWorkers: 4, QueueSize: 50,
		IdleTimeout: 60 * time.Second,
	})
	f.pools["sandbox"] = NewWorkerPool(PoolConfig{
		Name: "sandbox", MinWorkers: 1, MaxWorkers: 4, QueueSize: 50,
		IdleTimeout: 60 * time.Second,
	})
	// Pool gpu — dormente (MinWorkers 0) até um executor ser configurado e o
	// Start() detectar GPU disponível; o worker sobe sob demanda via ScaleTo.
	f.pools["gpu"] = NewWorkerPool(PoolConfig{
		Name: "gpu", MinWorkers: 0, MaxWorkers: 1, QueueSize: 8,
		IdleTimeout: 60 * time.Second,
	})

	f.orchestrator = &ExecutionOrchestrator{fabric: f}
	return f
}

// =============================================================================
// Subsystem interface (runtime.Subsystem)
// =============================================================================

// Name returns the subsystem name.
func (f *Fabric) Name() string { return "compute-fabric" }

// Start initializes all pools and begins hardware probing.
func (f *Fabric) Start(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.running {
		return nil
	}

	// Take initial probe to size pools correctly.
	f.probe.Start()
	time.Sleep(100 * time.Millisecond) // Wait for first probe.

	// Scale pools based on detected hardware.
	f.probe.mu.RLock()
	totalRAM := f.probe.snapshot.TotalRAM
	f.probe.mu.RUnlock()

	// Update pool max workers based on actual cores, driven by the config's
	// per-pool percentages. Ceilings now scale proportionally to the detected
	// cores (no more fixed 12/6/4/4/4 clamps): the default config reproduces
	// the historical multipliers, and larger machines get proportionally
	// larger pools. maxSafetyCeiling is only an anti-abuse guard against
	// pathological probe results — it is NOT tuning.
	const maxSafetyCeiling = 64 // teto de segurança anti-patológico, não tuning
	f.pools["agent"].cfg.MaxWorkers = f.probe.RecommendWorkers(f.cfg.CPUPercent, 2, maxSafetyCeiling)
	f.pools["tool"].cfg.MaxWorkers = f.probe.RecommendWorkers(f.cfg.ToolPercent, 1, maxSafetyCeiling/2)
	f.pools["index"].cfg.MaxWorkers = f.probe.RecommendWorkers(f.cfg.IndexPercent, 1, maxSafetyCeiling/4)
	f.pools["io"].cfg.MaxWorkers = f.probe.RecommendWorkers(f.cfg.IOPercent, 1, maxSafetyCeiling/4)
	f.pools["sandbox"].cfg.MaxWorkers = f.probe.RecommendWorkers(f.cfg.SandboxPercent, 1, maxSafetyCeiling/4)

	// Pool gpu: dorme (MaxWorkers 0) até que um executor esteja configurado E o
	// backend de inferência esteja disponível. O worker sobe sob demanda no
	// SubmitGPU via ScaleTo — MinWorkers permanece 0 (dormente sem GPU).
	if f.gpu != nil && f.gpu.Available(ctx) {
		f.pools["gpu"].cfg.MaxWorkers = 1
	} else {
		f.pools["gpu"].cfg.MaxWorkers = 0
	}

	// Set memory budget based on available RAM.
	f.budget.SetTotalBytes(totalRAM)

	// Start all pools with minimum workers.
	for _, pool := range f.pools {
		pool.Start()
	}

	// Start adaptive scheduler.
	go f.schedulerLoop(ctx)

	f.running = true
	return nil
}

// Stop gracefully shuts down all pools and stops probing.
func (f *Fabric) Stop(ctx context.Context) error {
	// CRITICAL FIX (audit 2026-08-01): the previous implementation held f.mu
	// while calling pool.Stop() -> wg.Wait(). If any worker was executing a
	// task that needed f.mu (e.g. a task submitting to another pool), Stop
	// deadlocked forever: the worker waits for f.mu, Stop holds f.mu waiting
	// for the worker. f.mu is now released before stopping pools; it is only
	// held to flip the running flag atomically.
	f.mu.Lock()
	if !f.running {
		f.mu.Unlock()
		return nil
	}
	f.running = false
	f.mu.Unlock()

	// Stop outside the lock: pool.Stop() blocks (bounded by the pool's
	// anti-freeze drain timeout) and must not hold f.mu while waiting for
	// workers that may need it. Each pool.Stop() is itself time-bounded
	// (see WorkerPool.Stop), so this loop cannot freeze the shutdown.
	f.probe.Stop()
	for _, pool := range f.pools {
		pool.Stop()
	}
	return nil
}

// Health returns the overall health of the fabric.
func (f *Fabric) Health() string {
	snap := f.probe.Snapshot()
	if f.IsOverloaded() || f.IsMemoryPressured() {
		return "degraded"
	}
	_ = snap
	return "healthy"
}

// =============================================================================
// Public API
// =============================================================================

// Submit enqueues a task to the specified pool.
func (f *Fabric) Submit(ctx context.Context, poolName string, task Task) (TaskResult, error) {
	pool, ok := f.pools[poolName]
	if !ok {
		return TaskResult{}, fmt.Errorf("unknown pool: %s", poolName)
	}

	// Circuit breaker check.
	if f.breaker.IsOpen(poolName) {
		return TaskResult{}, fmt.Errorf("circuit breaker open for pool %s", poolName)
	}

	// Rate limiter check.
	if !f.limiter.Allow(poolName) {
		return TaskResult{}, fmt.Errorf("rate limit exceeded for pool %s", poolName)
	}

	// Memory budget check.
	memBudget := uint64(task.Weight * 10 * 1024 * 1024) // ~10MB per weight unit.
	if !f.budget.TryAllocate(poolName, memBudget) {
		return TaskResult{}, fmt.Errorf("memory budget exceeded for pool %s", poolName)
	}
	defer f.budget.Release(poolName, memBudget)

	result, err := pool.Submit(ctx, task)

	// Update circuit breaker.
	if err != nil {
		f.breaker.RecordFailure(poolName)
	} else if result.Err != nil {
		f.breaker.RecordFailure(poolName)
	} else {
		f.breaker.RecordSuccess(poolName)
	}

	return result, err
}

// SetGPUExecutor configura o executor de inferência GPU (ex: NewOllamaExecutor).
// Sem executor o pipeline GPU fica dormindo e SubmitGPU retorna erro.
func (f *Fabric) SetGPUExecutor(e GPUExecutor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gpu = e
}

// GPUExecutor returns the configured GPU executor, or nil if none was set.
func (f *Fabric) GPUExecutor() GPUExecutor {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.gpu
}

// SubmitGPU submete uma carga de inferência GPU ao pool "gpu". O executor é
// invocado dentro de um worker do pool (Weight 2) e o resultado é convertido de
// volta para GPUResult. Se o pool estiver dormindo (MaxWorkers 0 — sem GPU
// disponível no Start), o Submit com timeout curto retorna erro — comportamento
// aceitável e documentado: sem GPU não há pipeline GPU.
func (f *Fabric) SubmitGPU(ctx context.Context, req GPURequest) (GPUResult, error) {
	f.mu.RLock()
	exec := f.gpu
	f.mu.RUnlock()

	if exec == nil {
		return GPUResult{}, fmt.Errorf("GPU executor não configurado (SetGPUExecutor)")
	}

	// Guard de VRAM fail-fast (mesma regra do executor).
	if req.VRAMEstimateMB > 0 && req.VRAMEstimateMB > exec.GPUInfo().VRAMGB*1024 {
		return GPUResult{}, fmt.Errorf(
			"VRAM insuficiente: carga estima %d MB, GPU tem %d GB",
			req.VRAMEstimateMB, exec.GPUInfo().VRAMGB)
	}

	task := Task{
		ID:      "gpu:" + shortTaskID(req),
		Weight:  2,
		Timeout: req.Timeout,
		Fn: func(ctx context.Context) (interface{}, error) {
			return exec.Execute(ctx, req)
		},
	}

	// Worker sobe sob demanda: o pool gpu nasce dormente (MinWorkers 0). ScaleTo
	// respeita o MaxWorkers (0 sem GPU disponível no Start, 1 com GPU) — quando
	// MaxWorkers é 0, nenhum worker é criado e o Submit abaixo falha no timeout
	// curto, que é o comportamento documentado para "sem GPU no pipeline".
	f.pools["gpu"].ScaleTo(1)

	// Pool dormindo (sem worker) → timeout curto falha rápido em vez de travar.
	submitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	result, err := f.Submit(submitCtx, "gpu", task)
	if err != nil {
		return GPUResult{}, err
	}
	if result.Err != nil {
		return GPUResult{}, result.Err
	}

	gpuResult, ok := result.Value.(GPUResult)
	if !ok {
		return GPUResult{}, fmt.Errorf("resultado inesperado do executor GPU (%T)", result.Value)
	}
	return gpuResult, nil
}

// shortTaskID gera um sufixo curto e estável para o ID da task (evita alocar
// um ID por caractere do prompt). Usa o modelo + tamanho do prompt.
func shortTaskID(req GPURequest) string {
	model := req.Model
	if model == "" {
		model = "unknown"
	}
	return fmt.Sprintf("%s:%d", model, len(req.Prompt))
}
func (f *Fabric) FanOut(ctx context.Context, poolName string, tasks []Task) ([]TaskResult, error) {
	results := make([]TaskResult, len(tasks))
	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t Task) {
			defer wg.Done()
			result, err := f.Submit(ctx, poolName, t)
			if err != nil {
				errOnce.Do(func() { firstErr = err })
				return
			}
			results[idx] = result
		}(i, task)
	}

	wg.Wait()
	return results, firstErr
}

// IsOverloaded returns true when the system is under heavy load.
func (f *Fabric) IsOverloaded() bool { return f.probe.IsOverloaded() }

// IsMemoryPressured returns true when RAM usage exceeds 80%.
func (f *Fabric) IsMemoryPressured() bool { return f.probe.IsMemoryPressured() }

// Pool returns a worker pool by name (nil if not found).
func (f *Fabric) Pool(name string) *WorkerPool {
	return f.pools[name]
}

// Snapshot returns the latest hardware reading.
func (f *Fabric) Snapshot() HardwareSnapshot { return f.probe.Snapshot() }

// =============================================================================
// Status Report
// =============================================================================

// StatusReport returns a formatted multi-line status report of the fabric.
func (f *Fabric) StatusReport() string {
	snap := f.probe.Snapshot()

	var b strings.Builder
	b.WriteString("COMPUTE FABRIC STATUS\n")
	b.WriteString("══════════════════════\n\n")

	b.WriteString("Hardware:\n")
	fmt.Fprintf(&b, "  CPU: %d logical cores\n", snap.LogicalCores)
	fmt.Fprintf(&b, "  Load: %.2f %.2f %.2f\n", snap.Load1, snap.Load5, snap.Load15)
	fmt.Fprintf(&b, "  RAM: %s / %s (%.1f%% used)\n",
		formatBytes(snap.UsedRAM), formatBytes(snap.TotalRAM), snap.MemoryUsage)
	if snap.GPU.Vendor != GPUNone {
		fmt.Fprintf(&b, "  GPU: %s/%s %d GB (%s)\n",
			snap.GPU.Vendor, snap.GPU.Model, snap.GPU.VRAMGB, snap.GPU.Compute)
	}
	b.WriteString("\n")

	b.WriteString("Worker Pools:\n")
	for _, name := range []string{"agent", "tool", "index", "io", "sandbox", "gpu"} {
		pool := f.pools[name]
		s := pool.Stats()
		bar := progressBar(s.ActiveWorkers, pool.cfg.MaxWorkers, 10)
		fmt.Fprintf(&b, "  %-8s %s  %d/%d workers  queue=%d  done=%d  lat=%v\n",
			name, bar, s.ActiveWorkers, pool.cfg.MaxWorkers,
			s.QueueDepth, s.Completed, s.AvgLatency.Truncate(time.Millisecond))
	}
	b.WriteString("\n")

	b.WriteString("Backpressure:\n")
	fmt.Fprintf(&b, "  Circuit breakers: %s\n", f.breaker.Status())
	fmt.Fprintf(&b, "  Rate limiter:     %s\n", f.limiter.Status())
	fmt.Fprintf(&b, "  Memory:           %s / %s budget\n",
		formatBytes(f.budget.UsedBytes()), formatBytes(f.budget.TotalBytes()))

	return b.String()
}

// progressBar returns an ASCII progress bar like "████░░░░░░".
func progressBar(current, max, width int) string {
	if max == 0 {
		return strings.Repeat("░", width)
	}
	filled := (current * width) / max
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// =============================================================================
// Adaptive Scheduler Loop
// =============================================================================

func (f *Fabric) schedulerLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f.adapt(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (f *Fabric) adapt(ctx context.Context) {
	_ = ctx
	snap := f.probe.Snapshot()
	cores := snap.LogicalCores
	if cores == 0 {
		cores = 4
	}

	loadFactor := snap.Load1 / float64(cores)

	for name, pool := range f.pools {
		stats := pool.Stats()
		current := stats.ActiveWorkers
		target := current

		switch {
		case loadFactor < 0.5 && stats.QueueDepth > 0:
			// System has headroom, pool has backlog — scale up.
			target = current + 1
		case loadFactor > 0.8:
			// System overloaded — scale down non-critical pools.
			if name == "agent" {
				target = current // Keep agent pool stable.
			} else {
				target = current - 1
			}
		case stats.QueueDepth > pool.cfg.MaxWorkers*2:
			// Significant backlog — scale up.
			target = current + 2
		case stats.QueueDepth == 0 && current > pool.cfg.MinWorkers:
			// No work — scale down excess workers.
			target = current - 1
		}

		if target != current {
			pool.ScaleTo(target)
		}
	}
}

// =============================================================================
// Execution Orchestrator (simplified — FanOut/Pipeline/MapReduce)
// =============================================================================

// ExecutionOrchestrator provides high-level execution patterns.
type ExecutionOrchestrator struct {
	fabric *Fabric
}

// FanOutConfig configures a fan-out operation.
type FanOutConfig struct {
	MaxConcurrency int
	Timeout        time.Duration
}
