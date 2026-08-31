package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/CoscaAI/cosca/api/stream"
	"github.com/CoscaAI/cosca/internal/adapter"
	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/chat"
	chatprovider "github.com/CoscaAI/cosca/internal/chat/provider"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/oracle"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/pipeline"
	"github.com/CoscaAI/cosca/internal/skills"
)

const maxPromptLength = 10000

// RunHandler handles the POST /v1/run and POST /v1/run/stream endpoints
// for AI orchestration.
type RunHandler struct {
	agentsMgr    *agents.Manager
	skillsMgr    *skills.Manager
	chatRegistry *chat.ChatRegistry
	execStore    *orchestration.ExecutionStore
	auditStore   *audit.Store
	hub          *stream.Hub

	// oracleGate é o bloqueio do Oráculo (ORACLE_PROTOCOL) — valida o
	// significado de toda request antes da execução. Fail-closed.
	oracleGate *oracle.Gate

	// memoryEngine and knowledgeEngine back the orchestration engine's MAG
	// (memory) and context-builder (knowledge) stages. Optional (nil-safe):
	// when nil the engine degrades gracefully, exactly like `cosca run`.
	memoryEngine    *memory.MemoryEngine
	knowledgeEngine *knowledge.Engine

	// knowledgeSearcher overrides knowledgeEngine when set. Enables gRPC-based
	// knowledge search against a standalone runtime daemon (Single Owner Model).
	knowledgeSearcher orchestration.KnowledgeSearcher

	// memoryRetriever and memoryStorer override memoryEngine when set.
	// Enables gRPC-based memory operations against the runtime daemon
	// (Single Owner Model — same pattern as knowledgeSearcher).
	memoryRetriever orchestration.MemoryRetriever
	memoryStorer    orchestration.MemoryStorer

	// timeout is the request timeout for chat executions. Zero means the
	// default of 5 minutes applies (see requestCtx).
	timeout time.Duration

	// registryFactory is a factory for creating a fresh ChatRegistry per
	// request. Defaults to chat.NewChatRegistry when not set. Using a
	// per-request registry prevents a request-level provider override from
	// leaking into other requests via the global singleton.
	registryFactory func() *chat.ChatRegistry

	// ── Pipeline components (nil when pipeline is disabled) ─────────────
	pipelineRunner   pipeline.Runner
	pipelinePlanner  *pipeline.Planner
	stepRunner       *pipeline.StepRunner
	stepRecoveryLoop *pipeline.RecoveryLoop
	stepPostTaskHook *pipeline.PostTaskHook
	stepCMITracker   *pipeline.CMITracker
	stepHistory      *pipeline.WorkflowHistory
	stepCheckpoint   *pipeline.CheckpointStore

	// deliberateConfig opt-in for the Kernel-First Deliberation stage
	// (ADR-032). Zero-value (never set) is fail-closed: Enabled=false
	// preserves the legacy path exactly.
	deliberateConfig orchestration.DeliberateConfig
}

// NewRunHandler creates a new RunHandler.
func NewRunHandler(agentsMgr *agents.Manager, chatRegistry *chat.ChatRegistry, auditStore *audit.Store) *RunHandler {
	return &RunHandler{
		agentsMgr:    agentsMgr,
		chatRegistry: chatRegistry,
		execStore:    orchestration.GetExecutionStore(),
		auditStore:   auditStore,
		// O Oráculo valida o significado de toda request (ORACLE_PROTOCOL).
		oracleGate: oracle.NewGate(),
		registryFactory: func() *chat.ChatRegistry {
			return chat.NewChatRegistry()
		},
	}
}

// SetHub sets the WebSocket Hub for broadcasting real-time events.
// May be nil if WebSocket is disabled.
func (h *RunHandler) SetHub(hub *stream.Hub) {
	h.hub = hub
}

// SetTimeout sets the request timeout for chat executions. Defaults to 5 minutes.
func (h *RunHandler) SetTimeout(d time.Duration) {
	if d > 0 {
		h.timeout = d
	}
}

// SetSkillsManager wires the skills manager into the orchestration engine's
// skill resolver. Optional (nil-safe).
func (h *RunHandler) SetSkillsManager(mgr *skills.Manager) {
	h.skillsMgr = mgr
}

// SetDeliberateConfig opts the /v1/run engine into the Kernel-First
// Deliberation stage (ADR-032). Fail-closed (LEI DO COFRE): when never
// called, the stage stays disabled and the legacy path is preserved.
func (h *RunHandler) SetDeliberateConfig(cfg orchestration.DeliberateConfig) {
	h.deliberateConfig = cfg
}

// SetMemoryEngine wires the memory engine into the orchestration engine's
// MAG (Memory-Augmented Generation) stage. Optional (nil-safe): when nil,
// memory is disabled for the request.
func (h *RunHandler) SetMemoryEngine(e *memory.MemoryEngine) {
	h.memoryEngine = e
}

// SetMemoryRetriever wires a direct MemoryRetriever (e.g. gRPC-backed)
// into the orchestration engine, bypassing the local memory.Engine.
// Optional (nil-safe): when nil, falls back to SetMemoryEngine.
func (h *RunHandler) SetMemoryRetriever(mr orchestration.MemoryRetriever) {
	h.memoryRetriever = mr
}

// SetMemoryStorer wires a direct MemoryStorer (e.g. gRPC-backed).
// Optional (nil-safe): when nil, falls back to SetMemoryEngine.
func (h *RunHandler) SetMemoryStorer(ms orchestration.MemoryStorer) {
	h.memoryStorer = ms
}

// SetKnowledgeEngine wires the knowledge engine into the orchestration
// engine's context builder. Optional (nil-safe): when nil, knowledge search
// is disabled for the request.
func (h *RunHandler) SetKnowledgeEngine(e *knowledge.Engine) {
	h.knowledgeEngine = e
}

// SetKnowledgeSearcher wires a direct KnowledgeSearcher (e.g. gRPC-backed)
// into the orchestration engine, bypassing the local knowledge.Engine.
// This is the preferred path under the Single Owner Model: the runtime
// daemon owns the knowledge engine; the serve delegates via gRPC.
// Optional (nil-safe): when nil, falls back to SetKnowledgeEngine.
func (h *RunHandler) SetKnowledgeSearcher(ks orchestration.KnowledgeSearcher) {
	h.knowledgeSearcher = ks
}

// SetRegistryFactory overrides the per-request registry factory.
func (h *RunHandler) SetRegistryFactory(f func() *chat.ChatRegistry) {
	if f != nil {
		h.registryFactory = f
	}
}

// SetPipelineRunner wires the shared pipeline Runner from bootstrap.
func (h *RunHandler) SetPipelineRunner(r pipeline.Runner) {
	h.pipelineRunner = r
}

// SetPipelinePlanner wires the shared pipeline Planner from bootstrap.
func (h *RunHandler) SetPipelinePlanner(p *pipeline.Planner) {
	h.pipelinePlanner = p
}

// SetPipelineComponents wires the full pipeline subsystem from bootstrap.
// All parameters are nil-safe — pipeline mode only activates when Runner
// and Planner are both set.
func (h *RunHandler) SetPipelineComponents(
	sr *pipeline.StepRunner,
	rl *pipeline.RecoveryLoop,
	pt *pipeline.PostTaskHook,
	cm *pipeline.CMITracker,
	wh *pipeline.WorkflowHistory,
	cs *pipeline.CheckpointStore,
) {
	h.stepRunner = sr
	h.stepRecoveryLoop = rl
	h.stepPostTaskHook = pt
	h.stepCMITracker = cm
	h.stepHistory = wh
	h.stepCheckpoint = cs
}

// requestCtx returns a context derived from the request with the configured
// execution timeout applied. Falls back to 5 minutes when no timeout has
// been set via SetTimeout.
func (h *RunHandler) requestCtx(r *http.Request) (context.Context, context.CancelFunc) {
	timeout := h.timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute // default
	}
	return context.WithTimeout(r.Context(), timeout)
}

// runRequest is the expected JSON body for POST /v1/run and POST /v1/run/stream.
type runRequest struct {
	Prompt   string `json:"prompt"`
	Agent    string `json:"agent,omitempty"`
	Provider string `json:"provider,omitempty"`
	Stream   bool   `json:"stream,omitempty"`
}

// runResponse is the JSON response for POST /v1/run.
type runResponse struct {
	Response   string   `json:"response"`
	Agent      string   `json:"agent"`
	SkillsUsed []string `json:"skills_used,omitempty"`
	DurationMs int64    `json:"duration_ms"`
	MemoryID   string   `json:"memory_id"`
}

// promptPreviewLength caps how much of a user prompt is persisted in audit
// logs. Prompts frequently contain secrets pasted by the user (API keys,
// credentials, private context), so the full text is never written to the
// audit database — only a SHA-256 hash (for correlation) and a short preview.
const promptPreviewLength = 100

// promptAuditInfo returns a safe representation of a user prompt for audit
// logging:
//
//	prompt_hash    — SHA-256 hex digest of the full prompt (for correlation)
//	prompt_preview — first 100 characters of the prompt (truncated, valid UTF-8)
//
// The full prompt is deliberately NOT included so secrets pasted into
// prompts never land in cleartext on disk (audit.db).
func promptAuditInfo(prompt string) map[string]string {
	hash := sha256.Sum256([]byte(prompt))
	runes := []rune(prompt)
	if len(runes) > promptPreviewLength {
		runes = runes[:promptPreviewLength]
	}
	return map[string]string{
		"prompt_hash":    hex.EncodeToString(hash[:]),
		"prompt_preview": string(runes),
	}
}

// withPromptAuditInfo merges the safe prompt audit fields into a details map.
func withPromptAuditInfo(details map[string]string, prompt string) map[string]string {
	for k, v := range promptAuditInfo(prompt) {
		details[k] = v
	}
	return details
}

// workspaceDir returns the project root for tool execution. Inside the jail
// (COSCA_JAILED=1) the workspace is mounted at "/", so "/" is the project
// root (COSCA_PROJECT_DIR is the host path and does not exist in the bubble).
func workspaceDir() string {
	if os.Getenv("COSCA_JAILED") == "1" {
		return "/"
	}
	if d := os.Getenv("COSCA_PROJECT_DIR"); d != "" {
		return d
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

// resolveRegistry returns a registry for this request. When a provider is
// requested explicitly, a fresh registry is created, registered, and selected
// so the override does not leak into the shared/default registry.
func (h *RunHandler) resolveRegistry(ctx context.Context, req runRequest) (*chat.ChatRegistry, error) {
	reg := h.chatRegistry
	if req.Provider != "" {
		// Fresh registry per request — no state leakage.
		reg = h.registryFactory()
		if err := chatprovider.RegisterChatProviders(reg, nil); err != nil {
			return nil, fmt.Errorf("register chat providers: %w", err)
		}
		cfg := chat.DefaultChatRegistryConfig()
		cfg.Primary = req.Provider
		if err := reg.Select(ctx, cfg); err != nil {
			return nil, fmt.Errorf("select provider %q: %w", req.Provider, err)
		}
		return reg, nil
	}
	// Default: use the shared registry (which may be nil → global singleton).
	if reg == nil {
		reg = chat.GetRegistry()
	}
	return reg, nil
}

// Execute handles POST /v1/run — executes a prompt through the SAME AI
// orchestration engine used by `cosca run` (internal/cli/run.go). The engine
// injects memory (MAG) and knowledge context, routes to the best agent, and
// returns the final result. The response JSON contract is unchanged.
//
// When pipeline mode is enabled (bootstrap pipeline components are wired and
// no per-request provider override), execution routes through the full
// autonomous pipeline: Planner → StepRunner → RecoveryLoop → DoD → CMI.
func (h *RunHandler) Execute(w http.ResponseWriter, r *http.Request) {
	var req runRequest
	limitBody(w, r, bodyLimitLarge)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	if len(req.Prompt) > maxPromptLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("prompt too long: max %d characters", maxPromptLength))
		return
	}

	// ── ORACLE_PROTOCOL: o bloqueio do Oráculo na fronteira do Cofre ──
	// Toda request que entra no serve é um pacote semântico. O Gate valida
	// o significado antes de qualquer execução: intenção presente, resultado
	// coerente, semântica mínima. Fail-closed: o que não prova, não passa.
	// (ORACLE_PROTOCOL §0 — o Oráculo recebe, interpreta, valida e classifica
	// semanticamente aquilo que o Cosca externo trouxe para dentro do Cofre.)
	oracleVerdict := h.oracleGate.Evaluate(oracle.SemanticPackage{
		Intent:  "responder ao prompt do usuário",
		Result:  req.Prompt,
		Source:  "external",
		Context: "request do usuário via API",
	})
	if oracleVerdict.Decision == oracle.Reject {
		LogEvent(h.auditStore, r, "oracle.reject", "oracle",
			audit.DetailsJSON(map[string]string{
				"reason":   oracleVerdict.Reason,
				"prompt":   req.Prompt,
				"decision": string(oracleVerdict.Decision),
			}), "warn")
		writeError(w, http.StatusBadRequest, "request rejeitada pelo oráculo: "+oracleVerdict.Reason)
		return
	}

	// ── Pipeline path: Planner → StepRunner → RecoveryLoop → DoD ──────
	// Activated only when pipeline components are wired AND no per-request
	// provider override. Provider overrides fall back to the legacy direct
	// path to avoid passing a request-scoped registry into shared pipeline
	// components (concurrency safety).
	if h.pipelineRunner != nil && h.pipelinePlanner != nil && req.Provider == "" {
		h.executeWithPipeline(w, r, req)
		return
	}

	// Resolve the registry for this request. An explicit provider override
	// is applied to a fresh, per-request registry so it never leaks into
	// the shared/default registry.
	registry, err := h.resolveRegistry(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	// Check that we have a provider available.
	if registry.Name() == "chat-registry" || registry.Model() == "" {
		writeError(w, http.StatusServiceUnavailable, "no chat provider configured — use 'cosca provider set' to configure one")
		return
	}

	ctx, cancel := h.requestCtx(r)
	defer cancel()

	// Build the orchestration engine with the request-scoped registry so a
	// provider override never leaks into other requests.
	engine := h.buildEngine(registry)

	// Wrap in the unified Runner interface.
	runner := pipeline.NewOrchAdapter(engine)

	pipeReq := pipeline.RunRequest{
		Prompt:  req.Prompt,
		Agent:   req.Agent,
		Options: pipeline.RunOptions{},
	}

	// Execute through unified Runner.
	startTime := time.Now()
	result, err := runner.Run(ctx, pipeReq)
	if err != nil {
		LogEvent(h.auditStore, r, "orchestration.run", "orchestration",
			audit.DetailsJSON(withPromptAuditInfo(map[string]string{
				"agent": req.Agent, "provider": registry.Name(), "error": err.Error(),
			}, req.Prompt)), "error")
		writeError(w, http.StatusInternalServerError, "chat completion failed: "+err.Error())
		return
	}
	duration := time.Since(startTime)

	// Preserve the response contract: agent falls back to the previous
	// default name when the engine did not resolve one, and memory_id is
	// always present (falling back to a generated ID when MAG is disabled).
	agentName := result.Agent
	if agentName == "" {
		agentName = "COSCA KERNEL"
	}
	memoryID := result.MemoryID
	if memoryID == "" {
		memoryID = uuid.New().String()
	}

	// Store execution in history.
	h.execStore.StoreExecution(
		claimsSubject(r), req.Prompt, result.Response, agentName, registry.Name(),
		registry.Model(), string(orchestration.ExecutionStatusSuccess),
		duration.Milliseconds(), result.SkillsUsed, memoryID,
	)

	LogEvent(h.auditStore, r, "orchestration.run", "orchestration",
		audit.DetailsJSON(withPromptAuditInfo(map[string]string{
			"agent": agentName, "provider": registry.Name(),
		}, req.Prompt)), "success")

	resp := runResponse{
		Response:   result.Response,
		Agent:      agentName,
		SkillsUsed: result.SkillsUsed,
		DurationMs: duration.Milliseconds(),
		MemoryID:   memoryID,
	}

	writeJSON(w, http.StatusOK, resp)
}

// executeWithPipeline runs the request through the autonomous pipeline:
// Planner → StepRunner (durable + recovery) → DoD → PostTaskHook → CMI.
// The result is converted back to the legacy runResponse contract.
func (h *RunHandler) executeWithPipeline(w http.ResponseWriter, r *http.Request, req runRequest) {
	ctx, cancel := h.requestCtx(r)
	defer cancel()

	registry := chat.GetRegistry()

	// Check that we have a provider available.
	if registry.Name() == "chat-registry" || registry.Model() == "" {
		writeError(w, http.StatusServiceUnavailable, "no chat provider configured — use 'cosca provider set' to configure one")
		return
	}

	startTime := time.Now()

	// 1. Plan: decompose prompt into task graph.
	plan := h.pipelinePlanner.Plan(req.Prompt)
	if req.Agent != "" && len(plan.Tasks) > 0 {
		plan.Tasks[0].Agent = req.Agent
	}

	// 2. Build a per-request StepRunner using shared components.
	var stepRunner *pipeline.StepRunner
	if h.stepHistory != nil && h.stepCheckpoint != nil && h.stepRecoveryLoop != nil {
		stepRunner = pipeline.NewDurableStepRunner(
			h.pipelineRunner, h.stepHistory, h.stepCheckpoint, h.stepRecoveryLoop,
		)
	} else {
		stepRunner = pipeline.NewStepRunner(h.pipelineRunner)
	}
	// /v1/run is a chat/assistant path — do not run a full go build+test
	// after every step (slow, fragile inside the jail). Verification is
	// available via the explicit qgate/esteira flows instead.
	stepRunner.SetVerification(false, false)
	stepRunner.SetPlanID(plan.ID)

	// 3. Execute plan through StepRunner (parallel, with dependency DAG).
	_, runErr := stepRunner.RunPlanParallel(ctx, plan, nil)
	if runErr != nil {
		LogEvent(h.auditStore, r, "orchestration.run", "pipeline",
			audit.DetailsJSON(withPromptAuditInfo(map[string]string{
				"agent":    req.Agent,
				"provider": registry.Name(),
				"error":    runErr.Error(),
				"plan_id":  plan.ID,
				"intent":   plan.IntentType,
			}, req.Prompt)), "error")
		writeError(w, http.StatusInternalServerError, "pipeline execution failed: "+runErr.Error())
		return
	}

	// 4. Collect results from completed tasks.
	var responseText strings.Builder
	var finalAgent string
	var skillsUsed []string
	var memoryID string

	for i, task := range plan.Tasks {
		if task.Result != nil && task.Result.Output != "" {
			if i > 0 {
				responseText.WriteString("\n\n")
			}
			responseText.WriteString(task.Result.Output)
			if task.Result.Agent != "" {
				finalAgent = task.Result.Agent
			}
		}
		if task.Result != nil && task.Result.TraceID != "" {
			memoryID = task.Result.TraceID
		}
		if task.Status == pipeline.TaskCompleted {
			skillsUsed = append(skillsUsed, task.Agent)
		}
	}

	if finalAgent == "" {
		finalAgent = req.Agent
	}
	if finalAgent == "" {
		finalAgent = "COSCA KERNEL"
	}
	if memoryID == "" {
		memoryID = uuid.New().String()
	}

	response := responseText.String()
	if response == "" {
		// All tasks failed or produced empty output — extract errors.
		var errs []string
		for _, task := range plan.Tasks {
			if task.Result != nil && task.Result.Error != "" {
				errs = append(errs, task.Result.Error)
			}
		}
		response = strings.Join(errs, "; ")
		if response == "" {
			response = "Pipeline completed with no output."
		}
	}

	// 5. Definition of Done.
	dod := pipeline.StandardDoD()
	dodReport := dod.Validate(ctx, plan)
	_ = dodReport

	// 6. PostTaskHook (auto-evolution stages 7-8).
	if h.stepPostTaskHook != nil {
		for _, task := range plan.Tasks {
			outcome := string(task.Status)
			_ = h.stepPostTaskHook.RecordTask(ctx, pipeline.EvolutionRecord{
				AgentName: task.Agent,
				TaskType:  plan.IntentType,
				Outcome:   outcome,
				Technique: "pipeline-step",
				Level:     3,
				Learned:   []string{fmt.Sprintf("task %s: %s", task.ID, outcome)},
			})
			_ = h.stepPostTaskHook.UpdateCapability(ctx, task.Agent, outcome)
		}
	}

	// 7. CMI Update — record the ACTUAL outcome, never a fabricated success.
	if h.stepCMITracker != nil {
		completed, _ := plan.Progress()
		failed := 0
		for _, task := range plan.Tasks {
			if task.Status == pipeline.TaskFailed {
				failed++
			}
		}
		outcome := "success"
		if failed > 0 && completed == 0 {
			outcome = "failed"
		} else if failed > 0 {
			outcome = "partial"
		}
		h.stepCMITracker.RecordTask(outcome, completed, failed, 0)
	}

	duration := time.Since(startTime)
	providerName := registry.Name()

	// Store execution in history.
	h.execStore.StoreExecution(
		claimsSubject(r), req.Prompt, response, finalAgent, providerName,
		registry.Model(), string(orchestration.ExecutionStatusSuccess),
		duration.Milliseconds(), skillsUsed, memoryID,
	)

	LogEvent(h.auditStore, r, "orchestration.run", "pipeline",
		audit.DetailsJSON(withPromptAuditInfo(map[string]string{
			"agent":    finalAgent,
			"provider": providerName,
			"plan_id":  plan.ID,
		}, req.Prompt)), "success")

	resp := runResponse{
		Response:   response,
		Agent:      finalAgent,
		SkillsUsed: skillsUsed,
		DurationMs: duration.Milliseconds(),
		MemoryID:   memoryID,
	}

	writeJSON(w, http.StatusOK, resp)
}

// buildEngine assembles the orchestration engine for a single request using
// the same components and options as `cosca run` (internal/cli/run.go):
// memory MAG adapter, knowledge searcher, agent + skill resolvers, and the
// request-scoped chat registry as the provider. Every optional adapter is
// nil-safe — the engine degrades gracefully exactly like the CLI.
func (h *RunHandler) buildEngine(registry *chat.ChatRegistry) *orchestration.Engine {
	orchConfig := orchestration.DefaultOrchestratorConfig()
	// Kernel-First Deliberation (ADR-032 / ADR-033): opt-in via
	// SetDeliberateConfig. Zero-value (never set) is fail-closed —
	// Enabled=false AND ShadowMode=false preserve the legacy path exactly.
	// The config is forwarded when EITHER mode is on (the engine's Execute
	// branch decides which behavior applies).
	if h.deliberateConfig.Enabled || h.deliberateConfig.ShadowMode {
		orchConfig.DeliberateConfig = h.deliberateConfig
	}

	var memRetriever orchestration.MemoryRetriever
	var memStorer orchestration.MemoryStorer
	if h.memoryRetriever != nil {
		// gRPC-backed retriever (Single Owner Model: runtime owns memory)
		memRetriever = h.memoryRetriever
	}
	if h.memoryStorer != nil {
		memStorer = h.memoryStorer
	}
	// Fallback: local memory engine adapter (when gRPC not configured)
	if memRetriever == nil && h.memoryEngine != nil {
		memAdapter := adapter.NewMemoryAdapter(h.memoryEngine)
		memRetriever = memAdapter
		memStorer = memAdapter
	}
	if memRetriever != nil {
		orchConfig.EnableMAG = true
		orchConfig.MAGConfig = orchestration.DefaultMAGConfig()
	}

	var knowledgeSearcher orchestration.KnowledgeSearcher
	if h.knowledgeSearcher != nil {
		// gRPC-backed searcher (Single Owner Model: runtime owns knowledge)
		knowledgeSearcher = h.knowledgeSearcher
	} else if h.knowledgeEngine != nil {
		// Legacy: local knowledge engine adapter
		knowledgeSearcher = adapter.NewKnowledgeAdapter(h.knowledgeEngine)
	}

	var agentResolver orchestration.AgentResolver
	if h.agentsMgr != nil {
		agentResolver = adapter.NewAgentResolverAdapter(h.agentsMgr)
	}

	var skillResolver orchestration.SkillResolver
	if h.skillsMgr != nil {
		skillResolver = adapter.NewSkillResolverAdapter(h.skillsMgr)
	}

	return orchestration.NewFactory(orchestration.FactoryConfig{
		Knowledge:       knowledgeSearcher,
		MemoryRetriever: memRetriever,
		MemoryStorer:    memStorer,
		AgentResolver:   agentResolver,
		SkillResolver:   skillResolver,
		ChatProvider:    registry,
		Config:          orchConfig,
		WorkspaceDir:    workspaceDir(),
	})
}

// buildAgentSystemPrompt creates a system prompt based on agent metadata.
func buildAgentSystemPrompt(agent *agents.Agent) string {
	var prompt string

	prompt = "You are " + agent.Name
	if agent.Role != "" && agent.Role != agent.Name {
		prompt += ", " + agent.Role
	}
	if agent.Department != "" {
		prompt += " from the " + agent.Department + " department"
	}
	prompt += ".\n"

	if agent.Description != "" {
		prompt += "\nYour purpose: " + agent.Description + "\n"
	}

	if agent.Mission != "" {
		prompt += "\nYour mission: " + agent.Mission + "\n"
	}

	if len(agent.Responsibilities) > 0 {
		prompt += "\nYour responsibilities:\n"
		for _, resp := range agent.Responsibilities {
			prompt += "- " + resp + "\n"
		}
	}

	prompt += "\nProvide a thorough, well-reasoned response. Use your expertise to help the user."

	return prompt
}

// Stream handles POST /v1/run/stream — executes a prompt via the LLM and
// streams the response back as Server-Sent Events (SSE).
//
// NOTE (unification scope): this endpoint intentionally stays on the direct
// chat-stream path instead of the orchestration engine used by /v1/run. The
// SSE wire protocol is its own contract (event types thinking/response/done,
// consumed by the web dashboard's useOrchestrationStream and the TypeScript
// SDK), while the engine's ExecuteStream emits different event types
// (progress/chunk/stage_transition/error). Mapping them would change the SSE
// shape and risk breaking existing stream consumers. Unifying streaming is
// tracked separately from the non-streaming /v1/run unification.
func (h *RunHandler) Stream(w http.ResponseWriter, r *http.Request) {
	var req runRequest
	limitBody(w, r, bodyLimitLarge)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	if len(req.Prompt) > maxPromptLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("prompt too long: max %d characters", maxPromptLength))
		return
	}

	// Resolve the registry for this request. An explicit provider override
	// is applied to a fresh, per-request registry so it never leaks into
	// the shared/default registry.
	registry, err := h.resolveRegistry(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	if registry.Name() == "chat-registry" || registry.Model() == "" {
		writeError(w, http.StatusServiceUnavailable, "no chat provider configured — use 'cosca provider set' to configure one")
		return
	}

	// Determine the agent and system prompt.
	agentName := req.Agent
	systemContent := "You are a helpful AI assistant. Provide thorough, well-reasoned responses."

	if req.Agent != "" && h.agentsMgr != nil {
		agent, err := h.agentsMgr.Get(req.Agent)
		if err == nil {
			agentName = agent.Name
			systemContent = buildAgentSystemPrompt(agent)
		}
	}
	if agentName == "" {
		agentName = "COSCA KERNEL"
	}

	// Build messages.
	messages := []chat.Message{
		{Role: chat.RoleSystem, Content: systemContent},
		{Role: chat.RoleUser, Content: req.Prompt},
	}

	// Set up SSE writer and headers.
	sw, err := stream.NewSSEWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	defer sw.Close()

	// Broadcast chat started event to WebSocket subscribers.
	if h.hub != nil {
		h.hub.BroadcastEvent([]string{"chat"}, "chat_started", map[string]string{
			"agent":    agentName,
			"provider": registry.Name(),
		})
	}

	// Send initial "thinking" event.
	_ = sw.WriteEvent(stream.EventThinking, "Analyzing request and preparing response...")

	// Execute.
	startTime := time.Now()
	ctx, cancel := h.requestCtx(r)
	defer cancel()

	opts := chat.DefaultChatOptions()
	opts.Stream = true

	chatStream, err := registry.ChatStream(ctx, messages, opts)
	if err != nil {
		LogEvent(h.auditStore, r, "orchestration.stream", "orchestration",
			audit.DetailsJSON(withPromptAuditInfo(map[string]string{
				"agent": agentName, "provider": registry.Name(), "error": err.Error(),
			}, req.Prompt)), "error")
		_ = sw.WriteError(fmt.Errorf("chat stream failed: %v", err))
		return
	}
	defer func() { _ = chatStream.Close() }()

	var fullResponse strings.Builder
	var model string
	skillsUsed := deriveSkillsFromRequest(req.Prompt, agentName, systemContent)

	for {
		select {
		case <-ctx.Done():
			_ = sw.WriteError(fmt.Errorf("request cancelled or timed out"))
			return
		default:
		}

		chunk, err := chatStream.Recv()
		if err != nil {
			// Stream ended — treat as successful completion if we have content.
			break
		}

		if chunk.Model != "" {
			model = chunk.Model
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				fullResponse.WriteString(choice.Delta.Content)
				_ = sw.WriteEvent(stream.EventResponse, choice.Delta.Content)
			}
		}
	}

	duration := time.Since(startTime)

	// Store execution in history.
	memoryID := uuid.New().String()
	h.execStore.StoreExecution(
		claimsSubject(r), req.Prompt, fullResponse.String(), agentName, registry.Name(),
		model, string(orchestration.ExecutionStatusSuccess),
		duration.Milliseconds(), skillsUsed, memoryID,
	)

	LogEvent(h.auditStore, r, "orchestration.stream", "orchestration",
		audit.DetailsJSON(withPromptAuditInfo(map[string]string{
			"agent": agentName, "provider": registry.Name(),
		}, req.Prompt)), "success")

	// Send final "done" event.
	_ = sw.WriteDoneWithDuration(duration.Milliseconds())

	// Broadcast chat ended event to WebSocket subscribers.
	if h.hub != nil {
		h.hub.BroadcastEvent([]string{"chat"}, "chat_ended", map[string]string{
			"agent":       agentName,
			"provider":    registry.Name(),
			"tokens":      fmt.Sprintf("%d", fullResponse.Len()),
			"duration_ms": fmt.Sprintf("%d", duration.Milliseconds()),
		})
	}
}

// deriveSkillsFromRequest derives skill names from the prompt and agent
// metadata using the orchestration package's keyword-based skill derivation.
func deriveSkillsFromRequest(prompt, agentName, systemContent string) []string {
	agentTexts := []string{agentName, systemContent}
	return orchestration.DeriveSkills(prompt, agentTexts)
}
