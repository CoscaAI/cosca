// Package bootstrap composes the Cosca engine stack — knowledge, memory,
// runtime, compute fabric and the background daemon — into a single
// initialization sequence shared by `cosca serve` and `cosca runtime start`.
//
// FASE 1 (DDNA-2026-08-07-001): the composition previously inlined in
// internal/cli/serve.go:265-358 now lives here, so the standalone runtime
// daemon (`cosca runtime start`) reuses the EXACT same engine wiring as the
// REST server. This guarantees that a daemon started via `runtime start`
// behaves identically to the engine half of `cosca serve`.
package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/adapter"
	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/circadian"
	"github.com/CoscaAI/cosca/internal/compute"
	"github.com/CoscaAI/cosca/internal/contextpipeline"
	"github.com/CoscaAI/cosca/internal/di"
	"github.com/CoscaAI/cosca/internal/diagnostics"
	"github.com/CoscaAI/cosca/internal/embed"
	"github.com/CoscaAI/cosca/internal/integrity"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/pending"
	"github.com/CoscaAI/cosca/internal/pipeline"
	rt "github.com/CoscaAI/cosca/internal/runtime"
	"github.com/CoscaAI/cosca/internal/toolrun"
)

// PipelineConfig controls pipeline component wiring during bootstrap.
type PipelineConfig struct {
	// Enabled enables full pipeline creation (Planner+StepRunner+Recovery).
	// When false, pipeline fields are nil but the shared Runner and
	// Orchestrator are still created (they're needed for basic run).
	Enabled bool
	// HistoryDir is where workflow history and checkpoints are stored.
	// Default: ".cosca/history".
	HistoryDir string
	// MaxRetries is the recovery loop max retries. Default: 3.
	MaxRetries int
	// DurableStore enables durable execution with event sourcing and
	// checkpointing. Default: true.
	DurableStore bool
}

// Config configures the engine composition.
type Config struct {
	// DataDir is the data directory shared by all engines (.cosca by
	// convention). Knowledge DB, runtime state and daemon PID file live
	// under it.
	DataDir string
	// Logger receives the engine lifecycle logs. The caller should attach
	// its own component context (e.g. "serve" or "runtime").
	Logger zerolog.Logger
	// EnableDaemon activates the background daemon: PID file, health
	// watchdog, sync loop and automatic knowledge DB backup. `cosca
	// runtime start` always enables it; `cosca serve` follows the
	// DefaultRuntimeConfig default (enabled — decision by Don 2026-07-31).
	EnableDaemon bool
	// RuntimeStandalone enables standalone runtime mode. When true, the
	// bootstrap does NOT create a daemon (even if EnableDaemon is true),
	// does NOT create the compute fabric, and does NOT register compute as
	// a runtime subsystem. Use with --api-only or --runtime-standalone.
	// (FASE 3, DDNA-2026-08-07-001)
	RuntimeStandalone bool
	// EmbeddingProvider optionally overrides the knowledge engine's embedding
	// provider (e.g. "local"). Empty means auto-detection.
	EmbeddingProvider string
	// EmbeddingBaseURL optionally points the embedding provider at a custom
	// base URL (e.g. a local OpenAI-compatible embeddings server). Empty means
	// the provider default endpoint.
	EmbeddingBaseURL string
	// EmbeddingModel optionally overrides the embedding provider's model name.
	EmbeddingModel string

	// EmbeddingDigest pina o digest esperado do modelo (L376).
	EmbeddingDigest string
	// EmbeddingAPIKey optionally overrides the embedding provider's API key.
	EmbeddingAPIKey string
	// EmbeddingDimensions optionally overrides the embedding provider's output
	// dimensions. 0 means the provider default.
	EmbeddingDimensions int
	// WatchFrameworkDir optionally points at a directory tree to watch for
	// automatic re-indexing (create/modify/delete/rename → IndexDocument/
	// IndexChanged/IndexRemoved). Empty disables the file watcher.
	WatchFrameworkDir string
	// Pipeline configures the autonomous pipeline subsystem.
	Pipeline PipelineConfig
	// AgentResolver is an optional agent resolver for the orchestration
	// engine. When nil the engine degrades gracefully (no agent routing).
	// Set after bootstrap when managers are available (Phase 2).
	AgentResolver orchestration.AgentResolver
	// SkillResolver is an optional skill resolver for the orchestration
	// engine. When nil the engine degrades gracefully (no skill pipeline).
	// Set after bootstrap when managers are available (Phase 2).
	SkillResolver orchestration.SkillResolver
	// WorkspaceDir is the project root for tool execution (search_codebase,
	// read_file, etc.). When set, the orchestration engine wires a
	// ToolExecutor. Inside the jail this is COSCA_PROJECT_DIR.
	//
	// DEPRECADO (Opção B, Etapa 3): o executor de ferramentas é montado via
	// ToolRunner (internal/toolrun) — o executor canônico com sandbox+
	// permission. Este campo é mantido para compat de chamadas.
	WorkspaceDir string

	// DeliberateConfig configures the Kernel-First Deliberation stage
	// (ADR-032). Zero-value (unset) is fail-closed: Enabled=false preserves
	// the legacy orchestration path bit-for-bit. Only an explicit
	// enabled:true in config turns the stage on.
	DeliberateConfig orchestration.DeliberateConfig

	// HaltChecker é o kill-switch do kernel (Etapa 3b). Quando não-nil, o
	// orchestration (via serve/run) bloqueia chamadas LLM se o kernel foi
	// haltado — o botão de emergência cobre o caminho do servidor.
	HaltChecker orchestration.HaltChecker

	// ContextCompiler habilita o Context Compiler (ADR-035 F6) no
	// orchestration: o contexto entregue ao LLM vira o estado operacional
	// compilado (TASK/STATE/FACTS...) com budget por seção. Opt-in; vazio =
	// comportamento atual (sem compilação).
	ContextCompiler ContextCompilerConfig

	// PendingResolution habilita a Pending Resolution (Don + professor,
	// 2026-09-01) no orchestration: quando o loop de tool-calls termina por
	// limite com trabalho pendente, o executor inspeciona o ESTADO e resolve a
	// continuação mínima implicada em vez de abandonar na reta final.
	PendingResolution PendingResolutionConfig
}

// ContextCompilerConfig configura o Context Compiler (ADR-035 F6) no serve.
type ContextCompilerConfig struct {
	// Enabled liga o pipeline no orchestration.
	Enabled bool
	// MaxTokens é o teto do contexto compilado (0 = sem teto).
	MaxTokens int
}

// PendingResolutionConfig configura a Pending Resolution no orchestration.
type PendingResolutionConfig struct {
	// Enabled liga a inspeção de pendências no Executor.
	Enabled bool
	// RecoverySteps é o teto próprio de recuperações (default 2 — nunca vira
	// loop).
	RecoverySteps int
}

// NewDefaultConfig returns a Config with safe defaults suitable for
// production use. Pipeline is disabled by default (opt-in for safety).
func NewDefaultConfig() Config {
	return Config{
		Pipeline: PipelineConfig{
			Enabled:      false,
			HistoryDir:   ".cosca/history",
			MaxRetries:   3,
			DurableStore: true,
		},
	}
}

// Result carries the composed engines back to the caller so the REST API,
// gRPC services and the shutdown sequence can keep using them.
type Result struct {
	// Knowledge may be nil when the engine fails to create/init
	// (warn-and-continue — serve keeps running without it).
	Knowledge *knowledge.Engine
	// Memory may be nil when the engine fails to create
	// (warn-and-continue).
	Memory *memory.MemoryEngine
	// RouteResolver is the deterministic module router (modlink) built from
	// DefaultRoutes. Never nil. It is the FASE 1 routing/scope decision-maker
	// shared by the serve REST /v1/knowledge/search and any modular adapter.
	// (New — additive; existing engine wiring is unaffected.)
	RouteResolver *modlink.Resolver

	// Runtime is always non-nil (rt.New has no error return).
	Runtime *rt.Runtime
	// Daemon is non-nil when EnableDaemon is set. It is registered on the
	// runtime even when Start fails (so the shutdown sequence can still
	// query IsRunning and stop it safely).
	Daemon *rt.Daemon

	// ── Orchestrator & Pipeline Runner (always created) ──────────────
	// Runner is the shared pipeline.Runner wrapping the orchestrator.
	Runner pipeline.Runner
	// Orchestrator is the shared orchestration engine.
	Orchestrator orchestration.Orchestrator

	// ── Pipeline components (nil when Pipeline.Enabled=false) ─────────
	// Planner decomposes user intent into executable task graphs.
	Planner *pipeline.Planner
	// StepRunner executes workflow steps as real pipeline tasks.
	StepRunner *pipeline.StepRunner
	// RecoveryLoop provides automatic retry on build/test failure.
	RecoveryLoop *pipeline.RecoveryLoop
	// Reconciler detects and fixes drift between desired and actual state.
	Reconciler *pipeline.Reconciler
	// PluginRegistry manages pipeline extension plugins.
	PluginRegistry *pipeline.PluginRegistry
	// CheckpointStore persists workflow state for crash recovery.
	CheckpointStore *pipeline.CheckpointStore
	// WorkflowHistory is the append-only event store for workflow history.
	WorkflowHistory *pipeline.WorkflowHistory
	// CMITracker records cognitive maturity indicators.
	CMITracker *pipeline.CMITracker
	// PostTaskHook enforces auto-evolution protocol stages 7-8.
	PostTaskHook *pipeline.PostTaskHook

	// ── Compute Fabric (multi-core adaptive execution engine) ────────
	// Fabric is the compute fabric with worker pools, GPU executor, and
	// backpressure. Nil when RuntimeStandalone is true.
	Fabric *compute.Fabric

	// Services is the declarative dependency-injection container
	// (Backstage/Spotify Pattern #2). Built-in engines (knowledge, memory)
	// are exposed as root-scoped singleton services so agents and
	// departments can declare the deps they need and have them injected,
	// instead of reaching into this struct by name. Never nil after
	// Compose (empty container if no engines were built).
	Services *di.Container
}

// Compose initializes the engine stack: knowledge, memory, runtime,
// compute fabric and (when EnableDaemon) the background daemon.
//
// (FASE 3, DDNA-2026-08-07-001): when RuntimeStandalone is true the
// compute fabric and daemon are NOT created — the API server delegates
// those responsibilities to the standalone runtime daemon.
//
// Failures of the optional engines are logged and tolerated — the same
// warn-and-continue semantics as the original serve composition. The only
// hard error is daemon.Start() failure under EnableDaemon, because the PID
// file contract (stop/status read <dir>/cosca.pid) depends on it.
func Compose(cfg Config) (*Result, error) {
	// ── Family Chain Integrity Check ──────────────────────────────────
	// Block startup if the Ed25519 family chain is broken (tampered files).
	// This runs BEFORE any engine initialization.
	coscaRoot, _ := os.Getwd()
	if fi, err := os.Stat(filepath.Join(coscaRoot, ".cosca", "family_chain.dat")); err == nil && fi.Size() > 0 {
		cfg.Logger.Info().Msg("verifying family chain integrity...")
		info, checkErr := integrity.Check(coscaRoot)
		if checkErr != nil {
			cfg.Logger.Fatal().Err(checkErr).Msg("integrity check failed — startup blocked")
			return nil, fmt.Errorf("integrity: %w", checkErr)
		}
		if !info.Valid {
			for _, e := range info.Errors {
				cfg.Logger.Error().Str("component", "integrity").Msg(e)
			}
			cfg.Logger.Fatal().Int("errors", len(info.Errors)).Msg("family chain breach detected — startup blocked")
			return nil, fmt.Errorf("integrity: chain breach — %d files tampered", len(info.Errors))
		}
		cfg.Logger.Info().Int("blocks", info.Blocks).Int("files", info.Files).Msg("family chain integrity verified")
	}

	// ── Fallback Materialization Safety Net ─────────────────────────────
	// If .cosca/fallback/ is missing or empty, consumers (knowledge compiler,
	// ORC, cosca-indexer) silently break on fresh installs. Materialize it
	// from the embedded framework so the fallback tree is always present.
	if coscaRoot != "" {
		fallbackDir := filepath.Join(coscaRoot, ".cosca", "fallback")
		missing, empty := true, true
		if entries, err := os.ReadDir(fallbackDir); err == nil {
			missing = false
			for _, en := range entries {
				if en.IsDir() {
					empty = false
					break
				}
			}
		}
		if missing || empty {
			cfg.Logger.Info().Msg("fallback tree missing or empty — materializing from embedded framework")
			if err := embed.MaterializeFallback(coscaRoot); err != nil {
				cfg.Logger.Warn().Err(err).Msg("fallback materialization failed (continuing without it)")
			}
		}
	}

	res := &Result{}

	// ── Route Resolver (FASE 1 routing/scope) ──────────────────────────
	// The deterministic modular router (ADR-013 §3.2) built from the production
	// route registry. It decides the bounded search space for a query; the
	// semantic search never chooses the space. Exposed on the Result so the
	// REST /v1/knowledge/search handler and the modular adapter can confine.
	routeRegistry := modlink.DefaultRoutes()
	res.RouteResolver = modlink.NewResolver(routeRegistry)
	cfg.Logger.Info().Int("routes", len(routeRegistry)).Msg("route resolver initialized")

	// ── Knowledge Engine ──────────────────────────────────────────────
	// (FASE 3, DDNA-2026-08-07-001): skipped in standalone mode — the
	// knowledge engine belongs to the runtime daemon, not the API server.
	// The REST handlers delegate to the daemon via gRPC when the local
	// engine is nil.
	if !cfg.RuntimeStandalone {
		keCfg := knowledge.DefaultConfig()
		keCfg.DBPath = filepath.Join(cfg.DataDir, "knowledge.db")
		keCfg.RootDir, _ = os.Getwd()
		if cfg.EmbeddingProvider != "" {
			keCfg.EmbeddingProvider = cfg.EmbeddingProvider
		}
		keCfg.EmbeddingBaseURL = cfg.EmbeddingBaseURL
		keCfg.EmbeddingModel = cfg.EmbeddingModel
		keCfg.EmbeddingDigest = cfg.EmbeddingDigest
		keCfg.EmbeddingAPIKey = cfg.EmbeddingAPIKey
		keCfg.EmbeddingDimensions = cfg.EmbeddingDimensions
		if cfg.WatchFrameworkDir != "" {
			keCfg.WatchEnabled = true
		}
		ke, keErr := knowledge.New(keCfg)
		if keErr != nil {
			cfg.Logger.Warn().Err(keErr).Msg("failed to create knowledge engine, continuing without it")
		} else {
			var initErr error
			if initErr = ke.Init(); initErr != nil {
				cfg.Logger.Warn().Err(initErr).Msg("failed to initialize knowledge engine, continuing")
			} else {
				cfg.Logger.Info().Msg("knowledge engine initialized")
			}
			res.Knowledge = ke
			// Auto re-indexing: watch the framework docs tree so edits are
			// reflected in the knowledge base without manual re-runs.
			if cfg.WatchFrameworkDir != "" && initErr == nil {
				if wErr := ke.WatchDirectory(cfg.WatchFrameworkDir); wErr != nil {
					cfg.Logger.Warn().Err(wErr).Str("dir", cfg.WatchFrameworkDir).
						Msg("framework watcher not started")
				}
			}
		}
	} else {
		cfg.Logger.Info().Msg("knowledge engine skipped (runtime standalone — delegated to daemon via gRPC)")
	}

	// ── Memory Engine ─────────────────────────────────────────────────
	// (FASE 3, DDNA-2026-08-07-001): skipped in standalone mode — same
	// rationale as the knowledge engine above.
	if !cfg.RuntimeStandalone {
		memCfg := memory.EngineConfig{
			DataDir: cfg.DataDir,
		}
		mem, memErr := memory.NewEngine(
			memory.WithConfig(memCfg),
			memory.WithLogger(cfg.Logger.With().Str("subsystem", "memory").Logger()),
		)
		if memErr != nil {
			cfg.Logger.Warn().Err(memErr).Msg("failed to create memory engine, continuing without it")
		} else {
			cfg.Logger.Info().Msg("memory engine initialized")
		}
		res.Memory = mem
	} else {
		cfg.Logger.Info().Msg("memory engine skipped (runtime standalone — delegated to daemon via gRPC)")
	}

	// ── Runtime ───────────────────────────────────────────────────────
	rtCfg := rt.DefaultRuntimeConfig()
	rtCfg.DataDir = cfg.DataDir
	rtCfg.RuntimeDir = filepath.Join(cfg.DataDir, "runtime")
	rtInstance := rt.New(
		rt.WithConfig(rtCfg),
		rt.WithLogger(cfg.Logger.With().Str("subsystem", "runtime").Logger()),
	)
	res.Runtime = rtInstance

	// ── Compute Fabric — multi-core adaptive execution engine ─────────
	// Probes hardware (CPU/RAM), sizes worker pools, and provides work
	// stealing + backpressure for agent/tool/index/io/sandbox workloads.
	// Wrapped in computeSubsystem to adapt the Fabric's string health
	// report to the runtime.Subsystem ComponentStatus contract.
	// GPU executor (Ollama + ROCm): o pool gpu só ativa quando o ProbeGPU
	// detecta GPU real — dentro do jail (sem /sys, sem /dev/kfd) o executor
	// fica dormente por design (MaxWorkers 0), sem custo nem erro.
	// (FASE 3, DDNA-2026-08-07-001): skipped in standalone runtime mode —
	// the compute fabric belongs to the runtime daemon, not the API server.
	if !cfg.RuntimeStandalone {
		fab := compute.NewFabric(compute.LoadFabricConfig())
		fab.SetGPUExecutor(compute.NewOllamaExecutor())
		rtInstance.RegisterCompute(&computeSubsystem{fabric: fab})
		res.Fabric = fab
	}

	// Start the runtime (starts health loop, etc.)
	startCtx, startCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := rtInstance.Start(startCtx); err != nil {
		cfg.Logger.Warn().Err(err).Msg("runtime start completed with issues")
	}
	startCancel()
	cfg.Logger.Info().Str("state", rtInstance.State().Current().String()).Msg("runtime started")

	// ── Background daemon (opt-in) ────────────────────────────────────
	// Manages the PID file, the health watchdog, log rotation, the periodic
	// sync loop, and the automatic knowledge DB backup. The automatic backup
	// is wired to the knowledge engine: on each BackupInterval tick the
	// engine snapshots its SQLite DB into <dir>/backups/ (the sqlite default
	// BackupDir) under an auto-* name, and the daemon prunes old auto-*
	// backups past MaxBackups. Nil engine (knowledge disabled) is a no-op so
	// the daemon still runs with its other features.
	// (FASE 3, DDNA-2026-08-07-001): daemon is skipped in standalone
	// runtime mode even when EnableDaemon is true — the API server does
	// not own the PID file or the sync/backup loop.
	if cfg.EnableDaemon && !cfg.RuntimeStandalone {
		daemonCfg := rt.DefaultDaemonConfig()
		daemonCfg.PIDPath = rtCfg.PidFile
		if daemonCfg.PIDPath == "" {
			daemonCfg.PIDPath = filepath.Join(cfg.DataDir, "cosca.pid")
		}
		// Retention pruning globs auto-*.db in BackupDir, which must
		// match where the knowledge engine's DB snapshots land.
		daemonCfg.BackupDir = filepath.Join(cfg.DataDir, "backups")
		daemonCfg.BackupFunc = func(ctx context.Context) error {
			if res.Knowledge == nil {
				return nil
			}
			_, err := res.Knowledge.Snapshot(rt.AutoBackupName())
			return err
		}
		// Circadian ORC (Operational Rest Cycle): manutenção não-atendida no
		// daemon (fio solto da auditoria — antes só rodava via `cosca
		// circadian watch` manual). Intervalo de 30s (default do scheduler).
		daemonCfg.ORCInterval = 30 * time.Second
		daemonCfg.ORCFunc = func(ctx context.Context) error {
			_, err := circadian.RunORC(ctx, cfg.DataDir)
			return err
		}
		daemon := rt.NewDaemon(rtInstance, daemonCfg)
		rtInstance.RegisterDaemon(daemon)
		res.Daemon = daemon
		if err := daemon.Start(); err != nil {
			cfg.Logger.Warn().Err(err).Msg("daemon failed to start")
			return res, fmt.Errorf("daemon failed to start: %w", err)
		}
		cfg.Logger.Info().Str("pid_file", daemonCfg.PIDPath).Msg("daemon started")
	}

	// ── Orchestrator & Pipeline Runner (always created) ──────────────
	// The shared orchestrator engine is needed for basic /v1/run
	// operations. Pipeline-specific components (Planner, StepRunner,
	// RecoveryLoop, etc.) are gated behind cfg.Pipeline.Enabled.
	{
		// Build knowledge searcher from the knowledge engine.
		var knowledgeSearcher orchestration.KnowledgeSearcher
		if res.Knowledge != nil {
			knowledgeSearcher = adapter.NewKnowledgeAdapter(res.Knowledge)
		}

		// Build memory retriever/storer from the memory engine.
		var memRetriever orchestration.MemoryRetriever
		var memStorer orchestration.MemoryStorer
		if res.Memory != nil {
			memAdapter := adapter.NewMemoryAdapter(res.Memory)
			memRetriever = memAdapter
			memStorer = memAdapter
		}

		orchConfig := orchestration.DefaultOrchestratorConfig()
		// Kernel-First Deliberation (ADR-032 / ADR-033): opt-in via config.
		// The zero value (caller enabled neither flag) is fail-closed —
		// Enabled=false AND ShadowMode=false keep the legacy path exactly as
		// today. The config is forwarded when EITHER mode is on (the engine
		// decides in the Execute branch which behavior applies).
		if cfg.DeliberateConfig.Enabled || cfg.DeliberateConfig.ShadowMode {
			orchConfig.DeliberateConfig = cfg.DeliberateConfig
		}
		if memRetriever != nil {
			orchConfig.EnableMAG = true
			orchConfig.MAGConfig = orchestration.DefaultMAGConfig()
		}

		// Executor de ferramentas ÚNICO (Opção B): o executor canônico
		// (registry real + sandbox + permission) é montado via internal/toolrun
		// e injetado no orchestration — o serve usa as MESMAS tools reais de
		// todos os caminhos.
		if cfg.WorkspaceDir != "" {
			orchConfig.ToolRunner = toolrun.Build(toolrun.Config{Workspace: cfg.WorkspaceDir})
		}

		// Context Compiler (ADR-035 F6, opt-in): quando habilitado, o
		// orchestration compila o contexto (TASK/STATE/FACTS...) com budget
		// por seção antes da chamada LLM — o "menor contexto suficiente para
		// cada decisão" do professor.
		if cfg.ContextCompiler.Enabled {
			orchConfig.ContextPipeline = contextpipeline.New(contextpipeline.Config{
				MaxTokens: cfg.ContextCompiler.MaxTokens,
			})
			cfg.Logger.Info().Msg("context compiler enabled (ADR-035 F6)")
		}

		// Pending Resolution (Don + professor, 2026-09-01, opt-in): quando o
		// loop de tool-calls termina por limite com trabalho pendente, o
		// executor inspeciona o ESTADO e resolve a continuação mínima
		// implicada — "termina o que estava quase terminado, sem inventar".
		if cfg.PendingResolution.Enabled {
			steps := cfg.PendingResolution.RecoverySteps
			if steps <= 0 {
				steps = 2
			}
			orchConfig.PendingResolver = pending.New(steps)
			cfg.Logger.Info().Int("recovery_steps", steps).Msg("pending resolution enabled (Don + professor)")
		}

		orchEngine := orchestration.NewFactory(orchestration.FactoryConfig{
			Knowledge:       knowledgeSearcher,
			MemoryRetriever: memRetriever,
			MemoryStorer:    memStorer,
			AgentResolver:   cfg.AgentResolver,
			SkillResolver:   cfg.SkillResolver,
			ChatProvider:    chat.GetRegistry(),
			Config:          orchConfig,
			WorkspaceDir:    cfg.WorkspaceDir,
			HaltChecker:     cfg.HaltChecker,
		})
		res.Orchestrator = orchEngine
		res.Runner = pipeline.NewOrchAdapter(orchEngine)
		cfg.Logger.Info().Msg("orchestration engine initialized")

		// ── Pipeline components (opt-in) ────────────────────────────
		if cfg.Pipeline.Enabled {
			historyDir := cfg.Pipeline.HistoryDir
			if historyDir == "" {
				historyDir = ".cosca/history"
			}

			history, histErr := pipeline.NewWorkflowHistory(historyDir)
			if histErr != nil {
				cfg.Logger.Warn().Err(histErr).Msg("pipeline: workflow history unavailable")
			}

			var checkpointStore *pipeline.CheckpointStore
			if history != nil {
				checkpointStore, _ = pipeline.NewCheckpointStore(
					filepath.Join(historyDir, "checkpoints"),
				)
			}

			classifier := diagnostics.NewClassifier()
			recoveryLoop := pipeline.NewRecoveryLoop(classifier)
			if cfg.Pipeline.MaxRetries > 0 {
				recoveryLoop.MaxRetries = cfg.Pipeline.MaxRetries
			}

			var stepRunner *pipeline.StepRunner
			if cfg.Pipeline.DurableStore && history != nil && checkpointStore != nil {
				stepRunner = pipeline.NewDurableStepRunner(res.Runner, history, checkpointStore, recoveryLoop)
			} else {
				stepRunner = pipeline.NewStepRunner(res.Runner)
			}

			reconciler := pipeline.NewReconciler(stepRunner)
			pluginRegistry := pipeline.NewPluginRegistry()
			cmiTracker := pipeline.NewCMITracker()
			postTaskHook := pipeline.NewPostTaskHook(
				filepath.Join(cfg.DataDir, "memory", "agent"),
			)
			planner := pipeline.NewPlanner(knowledgeSearcher, cfg.AgentResolver)

			res.WorkflowHistory = history
			res.CheckpointStore = checkpointStore
			res.RecoveryLoop = recoveryLoop
			res.StepRunner = stepRunner
			res.Reconciler = reconciler
			res.PluginRegistry = pluginRegistry
			res.CMITracker = cmiTracker
			res.PostTaskHook = postTaskHook
			res.Planner = planner

			cfg.Logger.Info().
				Bool("durable", cfg.Pipeline.DurableStore).
				Str("history_dir", historyDir).
				Int("max_retries", cfg.Pipeline.MaxRetries).
				Msg("pipeline components initialized")
		}
	}

	// ── Declarative Service Registry (Backstage/Spotify Pattern #2) ────
	// Expose the built engines as DI services so agents and sub-systems can
	// declare dependencies and have them injected. Additive — the explicit
	// wiring above remains the source of truth for lifecycle.
	res.Services = RegisterEngines(res)
	cfg.Logger.Info().Str("services", res.Services.String()).
		Msg("declarative service registry initialized")

	return res, nil
}

// computeSubsystem adapts *compute.Fabric to the runtime.Subsystem interface.
//
// The Fabric's Health() returns a plain string ("healthy"/"degraded") while
// runtime.Subsystem requires Health() to return a runtime.ComponentStatus.
// This adapter bridges the two contracts at the integration point so the
// Fabric itself stays a self-contained compute package.
//
// Moved from internal/cli/serve.go:53-79 (FASE 1) so both serve and the
// standalone runtime daemon share the exact same wiring.
type computeSubsystem struct {
	fabric *compute.Fabric
}

var _ rt.Subsystem = (*computeSubsystem)(nil)

func (c *computeSubsystem) Name() string                    { return c.fabric.Name() }
func (c *computeSubsystem) Start(ctx context.Context) error { return c.fabric.Start(ctx) }
func (c *computeSubsystem) Stop(ctx context.Context) error  { return c.fabric.Stop(ctx) }

// Health translates the Fabric's string health into a runtime ComponentStatus.
func (c *computeSubsystem) Health() rt.ComponentStatus {
	switch c.fabric.Health() {
	case "degraded":
		return rt.StatusDegraded
	case "unhealthy":
		return rt.StatusUnhealthy
	default:
		return rt.StatusHealthy
	}
}
