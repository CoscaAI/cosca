package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/deliberate"
	"github.com/CoscaAI/cosca/internal/shadow"
)

// ─── Orchestrator Configuration ──────────────────────────────────────────────

// OrchestratorConfig configures the orchestration engine.
type OrchestratorConfig struct {
	// Pipeline is the default pipeline definition applied to every request.
	Pipeline PipelineDefinition

	// EnableMAG enables Memory-Augmented Generation.
	EnableMAG bool

	// MAGConfig configures MAG behaviour when EnableMAG is true.
	MAGConfig MAGConfig

	// EnableStreaming enables streaming output by default.
	EnableStreaming bool

	// EmbedCacheConfig configures the embedding cache used by the context
	// builder to avoid redundant knowledge searches. When nil, caching is
	// disabled.
	EmbedCacheConfig *EmbedCacheConfig

	// Embedder is the embedding provider used by the semantic router.
	// When nil, semantic routing is disabled even if SemanticRouterConfig
	// has Enabled=true.
	Embedder Embedder

	// SemanticRouterConfig enables embedding-based agent routing.
	// When non-nil and Enabled=true, the engine uses vector similarity
	// instead of keyword matching for agent selection.
	SemanticRouterConfig *SemanticRouterConfig

	// ChainConfig enables multi-agent chain execution. When non-nil and
	// Enabled is true, the engine will attempt dynamic task decomposition
	// via the chain executor.
	ChainConfig *ChainConfig

	// WorkspaceDir is the root directory for tool execution (read_file,
	// write_file, execute_command, etc.). When non-empty, the engine creates
	// a ToolExecutor with this workspace root.
	WorkspaceDir string

	// Sandbox is the optional per-command execution sandbox (chat.Sandbox)
	// passed to the ToolExecutor. When set (e.g. by the terminal wiring running
	// OUTSIDE the jail), every execute_command tool call runs inside the
	// sandbox confined to the workspace. When nil, the enclosing jail provides
	// isolation.
	Sandbox chat.Sandbox

	// Budget is the per-execution cost ceiling forwarded to the Executor. When
	// nil, the engine uses the executor's default ceiling (the Don's $0.05 /
	// 8000 tokens / 20s) — so `cosca run` gets protected out of the box. When
	// set, it overrides the ceiling on every execution.
	Budget *CognitiveBudget

	// AllowedCommands é a allowlist de comandos que o toolExecutor pode rodar
	// via execute_command, confinada ao WorkspaceDir. Quando vazia, usa o
	// default seguro do executor. Permite ampliar por projeto (ex.: build de
	// um monorepo JS) SEM abrir comando arbitrário — só o que está listado.
	AllowedCommands []string

	// DeliberateConfig configures the Kernel-First Deliberation stage
	// (ADR-032). When Enabled is false (the default, fail-closed / LEI DO
	// COFRE), the stage is a no-op and the flow is EXACTLY the current one.
	DeliberateConfig DeliberateConfig

	// ShadowStore is the store where the Cognitive Shadow Mode (ADR-033)
	// persists its counterfactual observations.
	//   - When nil and ShadowMode is true, the engine falls back to
	//     shadow.DefaultStore() (the project's `.cosca/shadow/`).
	//   - When ShadowMode is false, it is ignored (nothing is recorded).
	// Injectable so tests can isolate the store in a temp dir.
	ShadowStore *shadow.Store
}

// DefaultOrchestratorConfig returns sensible default configuration.
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		Pipeline:        PipelineDefinition{},
		EnableMAG:       false,
		MAGConfig:       DefaultMAGConfig(),
		EnableStreaming: false,
		// Fail-closed: deliberation disabled by default (ADR-032).
		DeliberateConfig: DefaultDeliberateConfig(),
	}
}

// ─── Engine ──────────────────────────────────────────────────────────────────

// Engine is the concrete implementation of the Orchestrator interface. It
// wires together all port interfaces — KnowledgeSearcher, MemoryRetriever,
// MemoryStorer, AgentResolver, SkillResolver, and ChatProvider — into a
// fully-functional orchestration pipeline.
type Engine struct {
	contextBuilder *ContextBuilder
	router         *Router
	semanticRouter *SemanticRouter
	executor       *Executor
	pipeline       *Pipeline
	mag            *MAG
	chainExecutor  *ChainExecutor
	deliberator    *Deliberator
	shadowStore    *shadow.Store
	config         OrchestratorConfig
	metrics        *OrchestrationMetrics
}

// Compile-time check: Engine implements Orchestrator.
var _ Orchestrator = (*Engine)(nil)

// Deprecated: use NewFactory instead.
//
// NewEngine creates a fully wired orchestration engine.
// All dependencies are optional — the engine degrades gracefully when any
// dependency is nil (except the executor, which is required for LLM calls).
func NewEngine(
	knowledge KnowledgeSearcher,
	memoryRetriever MemoryRetriever,
	memoryStorer MemoryStorer,
	agents AgentResolver,
	skills SkillResolver,
	chatProvider ChatProvider,
	config OrchestratorConfig,
	metrics *OrchestrationMetrics,
) *Engine {
	// Create embed cache if configured.
	var embedCache *EmbedCache
	if config.EmbedCacheConfig != nil {
		embedCache = NewEmbedCache(*config.EmbedCacheConfig)
	}

	var ctxBuilder *ContextBuilder
	if knowledge != nil || memoryRetriever != nil {
		ctxBuilder = NewContextBuilder(knowledge, memoryRetriever, embedCache)
	}

	var router *Router
	if agents != nil {
		router = NewRouter(agents)
	}

	// Semantic routing is opt-in. Keep the keyword router available as the
	// semantic router's fallback so disabling semantic routing (or an embedding
	// failure/low-confidence match) preserves the existing routing behavior.
	var semanticRouter *SemanticRouter
	if config.SemanticRouterConfig != nil && config.SemanticRouterConfig.Enabled && config.Embedder != nil && agents != nil {
		semanticRouter = NewSemanticRouter(agents, config.Embedder, router, *config.SemanticRouterConfig)
	}

	var executor *Executor
	if chatProvider != nil {
		var toolExecutor *ToolExecutor
		if config.WorkspaceDir != "" {
			toolCfg := DefaultToolExecutorConfig()
			toolCfg.WorkspaceDir = config.WorkspaceDir
			toolCfg.Sandbox = config.Sandbox
			// Allowlist de comandos ampliada por projeto (ex.: build JS), SEM
			// abrir comando arbitrário. Se vazia, mantém o default seguro.
			if len(config.AllowedCommands) > 0 {
				toolCfg.AllowedCommands = append(toolCfg.AllowedCommands, config.AllowedCommands...)
			}
			toolExecutor = NewToolExecutor(toolCfg)
		}
		executorCfg := DefaultExecutorConfig()
		// Forward an explicit budget ceiling (overrides the executor's default).
		if config.Budget != nil {
			executorCfg.Budget = config.Budget
		}
		executor = NewExecutor(chatProvider, executorCfg, toolExecutor)
	}

	var pipeline *Pipeline
	if skills != nil {
		pipeline = NewPipeline(skills)
	}

	var mag *MAG
	if config.EnableMAG && memoryRetriever != nil && memoryStorer != nil {
		mag = NewMAG(memoryRetriever, memoryStorer, config.MAGConfig)
	}

	// Chain executor is created when chain execution is configured.
	// It wraps the engine itself, so we create it after the engine exists.
	var chainExec *ChainExecutor
	if config.ChainConfig != nil && config.ChainConfig.Enabled {
		chainExec = NewChainExecutor(nil, agents, skills, *config.ChainConfig) // engine set below
	}

	// Kernel-First Deliberation (ADR-032 / ADR-033): created whenever the
	// feature flag is enabled (authoritative) OR the Shadow observability flag
	// is on (observational). When both are false (the default, fail-closed),
	// the deliberator stays nil and the stage is a no-op.
	var deliberator *Deliberator
	if config.DeliberateConfig.Enabled || config.DeliberateConfig.ShadowMode {
		deliberator = NewDeliberator(config.DeliberateConfig)
	}

	// Cognitive Shadow Mode (ADR-033): the store is resolved only when the
	// Shadow flag is on. When nil, fall back to the project's `.cosca/shadow/`.
	var shadowStore *shadow.Store
	if config.DeliberateConfig.ShadowMode {
		if config.ShadowStore != nil {
			shadowStore = config.ShadowStore
		} else {
			shadowStore = shadow.DefaultStore()
		}
	}

	if metrics == nil {
		metrics = NewOrchestrationMetrics()
	}

	eng := &Engine{
		contextBuilder: ctxBuilder,
		router:         router,
		semanticRouter: semanticRouter,
		executor:       executor,
		pipeline:       pipeline,
		mag:            mag,
		chainExecutor:  chainExec,
		deliberator:    deliberator,
		shadowStore:    shadowStore,
		config:         config,
		metrics:        metrics,
	}

	// Wire the engine into the chain executor (it needs a reference to the
	// orchestrator to make sub-calls).
	if chainExec != nil {
		chainExec.engine = eng
	}

	return eng
}

// Metrics returns the engine's metrics collector. Never returns nil.
func (e *Engine) Metrics() *OrchestrationMetrics {
	return e.metrics
}

// recordShadow persiste a observação contrafactual do Cognitive Shadow Mode
// (ADR-033). É fire-and-forget e fail-closed (LEI DO COFRE): NUNCA deve alterar
// a resposta principal. Qualquer panic no caminho do Shadow é descartado
// silenciosamente (debug) e o request segue intacto para o Executor — o Shadow
// nunca bloqueia e nunca falha o request.
func (e *Engine) recordShadow(ctx context.Context, pc PipelineContext, trace DeliberationTrace, elapsed time.Duration) {
	logger := log.Ctx(ctx).With().Str("stage", "shadow").Str("request_id", pc.RequestID).Logger()
	if e.shadowStore == nil {
		logger.Debug().Msg("shadow: no store configured — observation discarded")
		return
	}
	// G1/LEI DO COFRE: qualquer panic é descartado (a resposta principal fica
	// intacta). O Shadow apenas recomenda (contrafactual) — nunca autoriza.
	defer func() {
		if r := recover(); r != nil {
			logger.Debug().Interface("panic", r).Msg("shadow: panic discarded — response unaffected")
		}
	}()

	st := toShadowTrace(pc, trace, elapsed)
	if err := e.shadowStore.Append(st); err != nil {
		// Fail-closed: persistência nunca derruba a execução.
		logger.Debug().Err(err).Msg("shadow: record append failed — discarded")
		return
	}
	logger.Info().
		Str("decision", string(st.Decision)).
		Float64("confidence", st.Confidence).
		Float64("convergence", st.Convergence).
		Bool("would_escalate", st.WouldEscalate).
		Int("positions", st.Positions).
		Str("reason", st.Reason).
		Msg("COSCA Cognitive Shadow Mode — kernel decided (NOT applied)")
}

// ─── Execute ─────────────────────────────────────────────────────────────────

// Execute implements Orchestrator.Execute. It runs the full orchestration
// pipeline synchronously:
//  1. MAG pre-execution memory retrieval
//  2. Context Builder (knowledge + memory search)
//  3. Router (agent selection)
//  4. Executor (LLM call)
//  5. Pipeline (skill execution)
//  6. MAG post-execution storage
func (e *Engine) Execute(ctx context.Context, req *Request) (*Result, error) {
	startTime := time.Now()
	if req == nil {
		return nil, fmt.Errorf("orchestration: request is nil")
	}

	// Generate request ID if not provided
	if req.ID == "" {
		req.ID = GenerateRequestID()
	}

	logger := log.Ctx(ctx).With().Str("request_id", req.ID).Logger()
	logger.Info().Str("prompt_hash", promptHash(req.Prompt)).Int("prompt_length", len(req.Prompt)).Msg("orchestration started")

	// 1. Initialize pipeline context
	pc := NewPipelineContext(req.ID, req.Prompt)
	if req.Context != nil {
		for k, v := range req.Context {
			pc = pc.WithContextData(k, v)
		}
	}

	// 2. MAG: Pre-execution memory retrieval
	if e.config.EnableMAG && e.mag != nil {
		done := e.metrics.RecordStageStart("mag_retrieve")
		var err error
		pc, err = e.mag.AugmentContext(ctx, pc)
		if err != nil {
			info := safeError("mag_augment_failed", err)
			logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("MAG augment context failed, continuing")
			done(false)
		} else {
			done(true)
			// Record retrieved memory count.
			if count := len(pc.Data.RetrievedMemories); count > 0 {
				e.metrics.RecordMAGRetrieve(count)
			}
		}
	}

	// 3. Context Builder: Knowledge + Memory search
	if e.contextBuilder != nil {
		done := e.metrics.RecordStageStart("context_builder")
		var err error
		pc, err = e.contextBuilder.Build(ctx, pc)
		if err != nil {
			info := safeError("context_build_failed", err)
			logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("context builder failed, continuing")
			done(false)
		} else {
			done(true)
		}

		// Track embed cache hit/miss.
		if pc.Data.KnowledgeCacheHit {
			e.metrics.RecordEmbedCacheHit()
		} else if pc.Data.KnowledgeResults != nil {
			// Only count as miss if a knowledge search actually happened
			// (we have results and it wasn't a cache hit).
			e.metrics.RecordEmbedCacheMiss()
		}
	}

	// 4. Router: Agent selection. Semantic routing is opt-in; when it is not
	// configured, retain the keyword router unchanged.
	if e.semanticRouter != nil || e.router != nil {
		done := e.metrics.RecordStageStart("router")
		var err error
		if e.semanticRouter != nil {
			pc, err = e.semanticRouter.Route(ctx, pc)
		} else {
			pc, err = e.router.Route(ctx, pc)
		}
		if err != nil {
			info := safeError("routing_failed", err)
			logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("router failed, continuing")
			done(false)
		} else {
			done(true)
		}
		// Record routing method.
		if pc.Data.RouterMethod != "" {
			e.metrics.RecordRouterDecision(pc.Data.RouterMethod)
		}
	}

	// 4.5 DELIBERATION (Kernel-First, deterministic, zero-LLM — ADR-032/033).
	// The Kernel thinks first: it deliberates deterministically on the
	// evidence the pipeline already collected BEFORE calling the LLM.
	// Two modes share the SAME Deliberate() call and SAME gates (reuse, not
	// duplication — ADR-015):
	//   - ATIVE (Enabled=true): the verdict GOVERNS the response. EmitOK
	//     responds WITHOUT the LLM; EmitWithReservations/Escalate rewrites
	//     AugmentedPrompt with the CLEAN context. (ADR-032, current behavior.)
	//   - SHADOW (ShadowMode=true): the verdict OBSERVES the response. It
	//     registers the counterfactual (ShadowTrace) but NEVER applies it —
	//     no WithLLMResponse, no WithDeliberationHandled, no WithAugmentedPrompt.
	//     The request continues to the Executor EXACTLY as today. (ADR-033.)
	// Fail-closed (LEI DO COFRE): when neither is on, or on any error, pc
	// passes through unchanged and the Executor runs exactly as today.
	delibCfg := e.config.DeliberateConfig
	if (delibCfg.Enabled || delibCfg.ShadowMode) && e.deliberator != nil {
		done := e.metrics.RecordStageStart("deliberation")
		callStart := time.Now()
		trace, err := e.deliberator.Deliberate(ctx, pc)
		elapsed := time.Since(callStart)
		if err != nil {
			info := safeError("deliberation_failed", err)
			logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("deliberation failed, continuing with legacy path (fail-closed)")
			done(false)
		} else {
			done(true)
			pc = pc.WithDeliberationTrace(&trace)

			// SHADOW (ADR-033): register the counterfactual, NEVER apply the
			// verdict. Runs FIRST so the counterfactual reflects the same
			// pipeline state the active mode would see; it never mutates pc.
			if delibCfg.ShadowMode {
				e.recordShadow(ctx, pc, trace, elapsed)
			}

			// ATIVE (ADR-032): apply the verdict (authoritative).
			if delibCfg.Enabled {
				switch trace.Verdict {
				case deliberate.EmitOK:
					// The Kernel is confident: respond WITHOUT the LLM.
					resp := BuildDeterministicResponse(pc.Data, trace)
					if resp != "" {
						pc = pc.WithLLMResponse(resp)
						pc = pc.WithExecutorDeterministic(true)
						pc = pc.WithDeliberationHandled(true)
						logger.Info().Float64("confidence", trace.Confidence.Final).Msg("deliberation: EmitOK — responding WITHOUT LLM")
					} else {
						// No response could be assembled; fall through to the LLM.
						logger.Warn().Msg("deliberation: EmitOK but empty deterministic response — falling through to LLM")
					}
				case deliberate.EmitWithReservations, deliberate.Escalate:
					// The Kernel is uncertain: hand the LLM a CLEAN context.
					clean := BuildCleanContext(pc.Data, trace, delibCfg)
					pc = pc.WithAugmentedPrompt(clean)
					logger.Info().Str("verdict", string(trace.Verdict)).Float64("confidence", trace.Confidence.Final).Msg("deliberation: uncertain — clean context handed to LLM")
				}
			}
		}
	}

	// 5. Executor: LLM call
	if e.executor != nil {
		done := e.metrics.RecordStageStart("executor")
		var err error
		pc, err = e.executor.Execute(ctx, pc)
		if err != nil {
			info := safeError("executor_failed", err)
			logger.Error().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("executor failed")
			done(false)
			return nil, safeContextError("executor_failed", err)
		}
		done(true)

		// Record LLM metrics.
		tokens := 0
		if usage, ok := pc.Data.LLMUsage.(chat.Usage); ok {
			tokens = usage.PromptTokens + usage.CompletionTokens
		}
		e.metrics.RecordLLMCall(true, tokens, pc.Data.ExecutorFallback)
	}

	// 6. Pipeline: Skill execution (optional)
	if e.pipeline != nil && len(e.config.Pipeline.Steps) > 0 {
		done := e.metrics.RecordStageStart("pipeline")
		var err error
		pc, err = e.pipeline.Execute(ctx, pc, e.config.Pipeline)
		if err != nil {
			info := safeError("pipeline_failed", err)
			logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("pipeline execution failed, continuing")
			done(false)
		} else {
			done(true)
		}
	}

	// 7. Build result
	result := e.buildResult(pc, startTime)

	// 8. MAG: Store result
	if e.config.EnableMAG && e.mag != nil {
		done := e.metrics.RecordStageStart("mag_store")
		stored, err := e.mag.StoreResult(ctx, pc, result)
		if err != nil {
			info := safeError("mag_store_failed", err)
			logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("MAG store result failed")
			e.metrics.RecordMAGStore(false)
			done(false)
		} else {
			e.metrics.RecordMAGStore(true)
			done(true)
			if stored != nil {
				result.MemoryID = stored.ID
			}
		}
	}

	// 9. Record overall request outcome.
	requestDuration := time.Since(startTime)
	e.metrics.RecordRequest(true, requestDuration)

	logger.Info().
		Str("agent", result.Agent).
		Int("skills_count", len(result.SkillsUsed)).
		Dur("duration", result.Duration).
		Msg("orchestration complete")

	return result, nil
}

// ─── ExecuteStream ───────────────────────────────────────────────────────────

// ExecuteStream implements Orchestrator.ExecuteStream. It runs the
// orchestration pipeline in streaming mode: the pre-execution stages
// (MAG, context builder, router) run synchronously, then the executor
// streams LLM chunks through the returned channel.
func (e *Engine) ExecuteStream(ctx context.Context, req *Request) (<-chan StreamEvent, error) {
	if req == nil {
		return nil, fmt.Errorf("orchestration: request is nil")
	}
	if req.ID == "" {
		req.ID = GenerateRequestID()
	}

	pc := NewPipelineContext(req.ID, req.Prompt)
	if req.Context != nil {
		for k, v := range req.Context {
			pc = pc.WithContextData(k, v)
		}
	}

	eventCh := make(chan StreamEvent, 100)

	go e.runStreamingPipeline(ctx, req, pc, eventCh)

	return eventCh, nil
}

// runStreamingPipeline executes the pipeline stages before the LLM call, then
// hands off to the executor for streaming. The executor's internal goroutine
// is responsible for closing eventCh when the chat stream ends.
func (e *Engine) runStreamingPipeline(ctx context.Context, req *Request, pc PipelineContext, eventCh chan<- StreamEvent) {
	logger := log.Ctx(ctx).With().Str("request_id", req.ID).Logger()

	// Helper to emit a stream event without blocking the caller indefinitely.
	emit := func(ev StreamEvent) {
		timer := time.NewTimer(500 * time.Millisecond)
		defer timer.Stop()
		select {
		case eventCh <- ev:
		case <-timer.C:
		case <-ctx.Done():
		}
	}

	// 1. MAG: Pre-execution memory retrieval
	if e.config.EnableMAG && e.mag != nil {
		var err error
		pc, err = e.mag.AugmentContext(ctx, pc)
		if err != nil {
			info := safeError("mag_augment_failed", err)
			logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("MAG augment context failed, continuing")
		}
	}

	// 2. Context Builder
	if e.contextBuilder != nil {
		var err error
		pc, err = e.contextBuilder.Build(ctx, pc)
		if err != nil {
			info := safeError("context_build_failed", err)
			logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("context builder failed, continuing")
		}
	}

	// 3. Router: Agent selection. Use semantic routing when enabled, with its
	// keyword router fallback; otherwise preserve the keyword router path.
	if e.semanticRouter != nil || e.router != nil {
		var err error
		if e.semanticRouter != nil {
			pc, err = e.semanticRouter.Route(ctx, pc)
		} else {
			pc, err = e.router.Route(ctx, pc)
		}
		if err != nil {
			info := safeError("routing_failed", err)
			logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("router failed, continuing")
		}

		agentName := pc.Data.ResolvedAgent
		if agentName != "" {
			emit(StreamEvent{
				Type:    StreamEventProgress,
				Content: fmt.Sprintf("Agent resolved: %s", agentName),
				Metadata: map[string]interface{}{
					"agent": agentName,
				},
			})
		}
	}

	// 4. Executor streaming — hands off to the executor's internal goroutine.
	if e.executor != nil {
		_, err := e.executor.ExecuteStream(ctx, pc, eventCh)
		if err != nil {
			info := safeError("executor_stream_failed", err)
			logger.Error().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("executor stream failed to start")
			emit(StreamEvent{
				Type:    StreamEventError,
				Content: safeErrorMessage("executor_stream_failed"), Metadata: safeErrorEvent("executor_stream_failed", err),
			})
			close(eventCh)
			return
		}
		// On success, the executor's readStream goroutine owns eventCh and
		// closes it when the stream ends. Do NOT close eventCh here.
		return
	}

	// No executor configured — close the channel.
	close(eventCh)
}

// ExecuteChain runs a pre-defined multi-agent chain via the chain executor.
// The ChainDefinition specifies which agents to invoke, their prompts,
// and their execution dependencies. When the chain executor is not
// configured, an error is returned.
func (e *Engine) ExecuteChain(ctx context.Context, def ChainDefinition) (*ChainResult, error) {
	if e.chainExecutor == nil {
		return nil, fmt.Errorf("orchestration: chain executor not configured")
	}
	return e.chainExecutor.Execute(ctx, def)
}

// ExecuteChainDynamic decomposes a complex request and runs a dynamically
// determined chain of agents. The CEO agent analyzes the request and
// determines which specialists to invoke.
func (e *Engine) ExecuteChainDynamic(ctx context.Context, prompt string) (*ChainResult, error) {
	if e.chainExecutor == nil {
		return nil, fmt.Errorf("orchestration: chain executor not configured")
	}
	return e.chainExecutor.ExecuteDynamic(ctx, prompt)
}

// ExecuteChainParallel runs multiple agents on the same prompt in parallel.
func (e *Engine) ExecuteChainParallel(ctx context.Context, prompt string, agentNames []string) (*ChainResult, error) {
	if e.chainExecutor == nil {
		return nil, fmt.Errorf("orchestration: chain executor not configured")
	}
	return e.chainExecutor.ExecuteParallel(ctx, prompt, agentNames)
}

// ─── Result Builder ──────────────────────────────────────────────────────────

// buildResult assembles the Result from the pipeline context.
func (e *Engine) buildResult(pc PipelineContext, startTime time.Time) *Result {
	response := pc.Data.LLMResponse
	if response == "" {
		response = ""
	}

	agent := pc.Data.ResolvedAgent
	if agent == "" {
		agent = "COSCA KERNEL"
	}

	skillsUsed := pc.Data.SkillsUsed

	memoryID := pc.Data.MemoryID

	// Extrai as tool executions do PipelineData para alimentar a evidência de
	// artefato (ADR-031). pc.Data.ToolResults é interface{} (contém
	// []*ToolCallResult) por compatibilidade legada; normalizamos aqui.
	toolExecs := normalizeToolResults(pc.Data.ToolResults)

	return &Result{
		ID:             pc.RequestID,
		Response:       response,
		Agent:          agent,
		SkillsUsed:     skillsUsed,
		MemoryID:       memoryID,
		ToolExecutions: toolExecs,
		Duration:       time.Since(startTime),
	}
}

// normalizeToolResults converte o ToolResults legado (interface{} contendo
// []*ToolCallResult, possivelmente nil) em []ToolCallResult por valor, para
// o Result carregar uma cópia segura das tools executadas.
func normalizeToolResults(raw interface{}) []ToolCallResult {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []*ToolCallResult:
		if len(v) == 0 {
			return nil
		}
		out := make([]ToolCallResult, 0, len(v))
		for _, tr := range v {
			if tr != nil {
				out = append(out, *tr)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []ToolCallResult:
		out := make([]ToolCallResult, len(v))
		copy(out, v)
		return out
	default:
		return nil
	}
}
