package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/contenttrust"
	"github.com/CoscaAI/cosca/internal/pending"
	"github.com/CoscaAI/cosca/internal/middleware"
	"github.com/CoscaAI/cosca/internal/stallwatch"
	"github.com/CoscaAI/cosca/internal/taskaffinity"
)

// ─── Executor Configuration ──────────────────────────────────────────────────

// ExecutorConfig configures the executor's retry and timeout behaviour.
type ExecutorConfig struct {
	// MaxRetries is the maximum number of retry attempts on transient errors.
	// Default: 3.
	MaxRetries int

	// RetryDelay is the base duration to wait between retries. The actual
	// delay grows exponentially: delay * 2^(attempt-1).
	// Default: 2s.
	RetryDelay time.Duration

	// Timeout is the maximum duration for a single chat call (including
	// all retries). The aggregate timeout for the entire Execute run is
	// proportional to MaxRetries. Default: 5m.
	Timeout time.Duration

	// MaxToolRounds is the maximum number of tool-call rounds the executor
	// will loop through. Each round is: LLM → tools → follow-up LLM.
	// The loop stops early when the LLM stops requesting tools.
	// Default: 5.
	MaxToolRounds int

	// Middleware (padrão LangChain wrap_model_call): cadeia de middlewares
	// que envolve a chamada ao provider. Permite medir custo (ADR-031),
	// decidir retry/fallback/limite e abortar por budget SEM mudar a lógica
	// dos handler/agente. Nil = sem middleware (comportamento atual).
	Middleware *middleware.Chain

	// Budget is the per-execution cost ceiling. When non-nil, the tool-call
	// loop stops calling the provider once the accumulated consumption
	// (tokens, time, or cost) exceeds the ceiling — closing the cost net of
	// `cosca run` / orchestration. When nil, the executor runs exactly as
	// before (no ceiling). Default: the Don's default CDN budget ($0.05 /
	// 8000 tokens / 20s) in DefaultExecutorConfig; nil in a zero-value config.
	Budget *CognitiveBudget

	// EstimateCost computes the USD cost of a LLM response for the budget
	// ceiling. When nil, a conservative default rate is used (see
	// defaultEstimateCost).
	EstimateCost CostEstimator

	// HaltChecker é o kill-switch do kernel (interface mínima IsHalted).
	// Quando não-nil, o executor verifica ANTES de cada chamada LLM — se o
	// kernel foi haltado, a chamada é bloqueada com erro claro. Nil = sem
	// check (comportamento atual).
	HaltChecker HaltChecker

	// ContextPipeline é o Context Compiler (ADR-035, F6). Quando não-nil, o
	// executor compila o contexto (TASK/STATE/FACTS...) com budget por seção
	// antes da chamada LLM, e decodifica a resposta como instruction packet.
	// Nil = comportamento atual (sem compilação de contexto).
	ContextPipeline ContextPipeline

	// EnableTAS ativa o Task-Aware Search (ADR-045) de ponta a ponta. Quando
	// true E o ContextPipeline está configurado, o executor deriva o perfil de
	// tarefa (taskaffinity.BuildTaskContext) a partir do estado de implementação
	// disponível (Prompt + metadados do Request.Context via PipelineData.Extra)
	// e o injeta no PipelineData.TaskContext ANTES do BuildContext — re-ponderando
	// a busca por afinidade com a tarefa e ajustando-a por fase.
	//
	// Aditivo e fail-closed: default false. Quando false (ou quando o
	// PipelineData.TaskContext já veio preenchido pelo chamador), o executor se
	// comporta EXATAMENTE como antes — zero regressão.
	EnableTAS bool

	// PendingResolver é a Pending Resolution (decisão do Don + professor,
	// 2026-09-01): quando o loop de tool-calls termina por limite com trabalho
	// ainda pendente, o executor inspeciona o ESTADO e, se a pendência é
	// resolvível (ação já implicada), executa a continuação mínima em vez de
	// abandonar na reta final. Nil = comportamento atual (para sem inspecionar).
	PendingResolver PendingResolver
}

// PendingResolver é a interface mínima da Pending Resolution que o Executor
// usa. Implementada por *pending.Resolver (internal/pending).
type PendingResolver interface {
	// Inspect decide se há pendência resolvível a partir do estado.
	Inspect(state pending.State) pending.Result
	// RecoveryRemaining reporta quantas recuperações ainda restam.
	RecoveryRemaining() int
}

// pendingStateOf projeta o estado de execução para a inspeção de pendências.
func pendingStateOf(pc PipelineData, maxSteps int) pending.State {
	return pending.State{
		PendingActions: nil, // preenchido pelo chamador (tool calls atuais)
		Observations:   pc.ObservationsForPending(),
		CurrentStep:    maxSteps,
		MaxSteps:       maxSteps,
	}
}

// ContextPipeline é a interface mínima que o Executor usa do Context Compiler.
// Implementada por *contextpipeline.Pipeline (internal/contextpipeline).
// Definida aqui como interface para não acoplar o orchestration ao pacote.
type ContextPipeline interface {
	// BuildContext decide a camada e compila o estado operacional. Devolve o
	// texto do contexto compilado (para injetar no system prompt) e a camada
	// decidida.
	BuildContext(prompt string, data *PipelineData) (string, PipelineLevel)
}

// PipelineLevel é a camada de contexto decidida (L0/L1/L2).
type PipelineLevel struct {
	// Name é L0/L1/L2.
	Name string
	// Deterministic indica que a decisão foi resolvida SEM LLM.
	Deterministic bool
}

// HaltChecker é a interface mínima do kill-switch. O kernel.EmergencyManager
// a satisfaz — o orchestration conhece apenas a interface, sem depender do
// kernel.
type HaltChecker interface {
	IsHalted() bool
}

// CostEstimator computes the estimated USD cost of a single LLM chat response
// from its token usage and the elapsed duration of the call. It is used by the
// cost ceiling (ExecutorConfig.Budget) to decide whether another call still
// fits. When nil, the executor uses defaultEstimateCost.
type CostEstimator func(usage chat.Usage, elapsed time.Duration) float64

// defaultEstimateCost computes a conservative USD estimate from token usage:
// input at $2.00/M and output at $4.00/M. This is a generous upper bound for
// coding models (e.g. deepseek) so the default $0.05 ceiling is a real guard
// against runaway tool loops WITHOUT tripping on a single ordinary call.
func defaultEstimateCost(usage chat.Usage, _ time.Duration) float64 {
	const (
		inputPerToken  = 2.00 / 1_000_000
		outputPerToken = 4.00 / 1_000_000
	)
	return float64(usage.PromptTokens)*inputPerToken +
		float64(usage.CompletionTokens)*outputPerToken
}

// DefaultExecutorConfig returns sensible default configuration.
//
// NOTE (rede de custo): DefaultExecutorConfig aplica o teto padrão aprovado
// pelo Don (CognitiveBudget.CDN: $0.05 / 8000 tokens / 20s) para que o caminho
// `cosca run` — que constrói o Executor via NewEngine → DefaultExecutorConfig —
// NÃO rode sem teto de custo. Um zero-value ExecutorConfig{} mantém Budget=nil
// (sem trava) para preservar o comportamento de call sites existentes.
func DefaultExecutorConfig() ExecutorConfig {
	budget := DefaultCognitiveBudget()
	return ExecutorConfig{
		MaxRetries:    3,
		RetryDelay:    2 * time.Second,
		Timeout:       5 * time.Minute,
		MaxToolRounds: 5,
		Budget:        &budget,
		// EnableTAS: o Task-Aware Search (ADR-045) fica ATIVO por padrão no
		// caminho padrão (`cosca run`). A busca passa a re-ponderar por
		// afinidade com a tarefa e ajustar por fase. Um zero-value
		// ExecutorConfig{} mantém EnableTAS=false (comportamento atual) para
		// call sites que não usam o default.
		EnableTAS: true,
	}
}

// ─── Executor ────────────────────────────────────────────────────────────────

// tasTaskContextFrom constrói o TaskContext do Task-Aware Search (ADR-045) a
// partir do estado de implementação disponível no PipelineData. É o wiring do
// executor → taskaffinity.
//
// O executor NÃO tem acesso direto aos arquivos abertos no editor (isso é do
// lado do opencode, não do runtime Go). O que ele consegue alimentar:
//   - Prompt: do PipelineData (a intenção do usuário)
//   - WorkingDir / OpenFiles / RecentFiles / TargetHint / GoModExists:
//     metadados que o editor/Request.Context injetou via PipelineData.Extra
//     sob as chaves "tas.working_dir", "tas.open_files", "tas.recent_files",
//     "tas.target_hint", "tas.go_mod" (todas opcionais).
//
// Fail-closed: se o estado não permite derivar um perfil confiável (Prompt
// vazio e nenhum metadado), retorna nil — o pipeline segue idêntico ao atual.
func tasTaskContextFrom(data PipelineData) *taskaffinity.TaskContext {
	state := &taskaffinity.ImplementaçãoState{
		Prompt: data.AugmentedPrompt,
	}
	if state.Prompt == "" {
		state.Prompt = data.GetExtraString("prompt")
	}

	// Metadados opcionais injetados pelo editor via Request.Context →
	// PipelineData.Extra. Todos nil-safe (ausência = campo vazio).
	if wd := data.GetExtraString("tas.working_dir"); wd != "" {
		state.WorkingDir = wd
	}
	if of, ok := data.Extra["tas.open_files"].([]string); ok {
		state.OpenFiles = of
	}
	if rf, ok := data.Extra["tas.recent_files"].([]string); ok {
		state.RecentFiles = rf
	}
	if th := data.GetExtraString("tas.target_hint"); th != "" {
		state.TargetHint = th
	}
	if data.GetExtraBool("tas.go_mod") {
		state.GoModExists = true
	}
	if data.GetExtraBool("tas.package_json") {
		state.PackageJSONExists = true
	}

	// Fail-closed: sem prompt e sem nenhum sinal de stack/alvo, não há como
	// derivar um perfil confiável — retorna nil (pipeline segue como antes).
	if state.Prompt == "" && len(state.OpenFiles) == 0 && len(state.RecentFiles) == 0 &&
		!state.GoModExists && !state.PackageJSONExists && state.TargetHint == "" {
		return nil
	}

	return taskaffinity.BuildTaskContext(state)
}

// Executor runs agents against LLM providers. It handles prompt construction,
// retry with exponential backoff, tool-call detection, and both synchronous
// and streaming execution modes.
type Executor struct {
	provider         chat.ChatProvider
	config           ExecutorConfig
	toolRunner       ToolRunner // optional: executes tool calls returned by LLMs (executor canônico via adapter)
	lastAttemptCount int64      // atomic: number of attempts used by last chatWithRetry

	// stalls optionally records provider stall/retry events so a run can be
	// compared with a healthy one (timeouts, retries, recoveries). May be nil.
	stalls *stallwatch.Collector
}

// haltBlocked reporta se o kill-switch do kernel foi acionado (Etapa 3b).
// O Executor não depende do kernel — apenas da interface mínima HaltChecker
// injetada via ExecutorConfig.
func (e *Executor) haltBlocked() bool {
	return e.config.HaltChecker != nil && e.config.HaltChecker.IsHalted()
}

// SetStallCollector attaches (or detaches, with nil) the stall-event collector
// used to record provider stalls and retries during chat calls.
func (e *Executor) SetStallCollector(c *stallwatch.Collector) {
	e.stalls = c
}

// recordStall writes a single stall/retry/recovery event when a collector is
// attached. It never affects control flow — observability only.
func (e *Executor) recordStall(spec stallwatch.WatchSpec, action stallwatch.Action, waited time.Duration, attempt int, errMsg string) {
	if e.stalls == nil {
		return
	}
	e.stalls.Add(stallwatch.Event{
		Timestamp: time.Now().UTC(),
		Operation: spec.Name,
		Provider:  spec.Provider,
		Model:     spec.Model,
		Waited:    waited,
		Attempt:   attempt,
		Action:    action,
		Error:     errMsg,
	})
}

// NewExecutor creates an Executor backed by the given ChatProvider.
// toolRunner is optional — pass nil if tool execution is not needed.
func NewExecutor(provider chat.ChatProvider, config ExecutorConfig, toolRunner ToolRunner) *Executor {
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 2 * time.Second
	}
	if config.Timeout <= 0 {
		config.Timeout = 5 * time.Minute
	}
	if config.MaxToolRounds <= 0 {
		config.MaxToolRounds = 5
	}
	if config.EstimateCost == nil {
		config.EstimateCost = defaultEstimateCost
	}
	return &Executor{provider: provider, config: config, toolRunner: toolRunner}
}

// recordConsumption acumula o consumo de uma resposta de IA no BudgetTracker
// (se existir). Best-effort: NUNCA altera o fluxo de erro nem a resposta — é a
// instrumentação da rede de custo. Se o tracker for nil (budget desligado) ou
// a resposta for nil, não faz nada.
func (e *Executor) recordConsumption(tracker *BudgetTracker, resp *chat.ChatResponse, elapsed time.Duration) {
	if tracker == nil || resp == nil {
		return
	}

	tokens := resp.Usage.PromptTokens + resp.Usage.CompletionTokens
	cost := 0.0
	if e.config.EstimateCost != nil {
		cost = e.config.EstimateCost(resp.Usage, elapsed)
	} else {
		cost = defaultEstimateCost(resp.Usage, elapsed)
	}
	tracker.Record(tokens, elapsed, cost)
}

// ─── Synchronous Execution ───────────────────────────────────────────────────

// Execute runs the agent with the given prompt and pipeline context. It
// builds the messages, calls the LLM with retry, and enriches the
// PipelineContext with the response.
func (e *Executor) Execute(ctx context.Context, pc PipelineContext) (PipelineContext, error) {
	logger := log.Ctx(ctx).With().Str("stage", "executor").Str("request_id", pc.RequestID).Logger()

	// 1. Extract the augmented prompt from Data (set by context-builder
	//    stage), falling back to the raw prompt.
	augmentedPrompt := pc.Data.AugmentedPrompt
	if augmentedPrompt == "" {
		augmentedPrompt = pc.Prompt
	}

	// 2. Read resolved agent information from the pipeline context.
	agentName := pc.Data.ResolvedAgent
	agentRole := pc.Data.AgentRole
	agentDept := pc.Data.AgentDepartment
	agentDesc := pc.Data.AgentDescription

	if agentName == "" {
		agentName = "COSCA KERNEL"
	}
	if agentRole == "" {
		agentRole = agentName
	}

	// 3. Build the system prompt from agent metadata and any
	//    knowledge / memory context.
	systemContent := e.buildSystemPrompt(agentName, agentRole, agentDept, agentDesc, pc.Data)

	// 3.5 ── CONTEXT COMPILER (ADR-035, F6, opt-in) ─────────────
	// Quando o ContextPipeline está configurado, o contexto entregue ao LLM é
	// o ESTADO OPERACIONAL compilado (TASK/STATE/FACTS... com budget por
	// seção) em vez de documentos empilhados. Aditivo: nil = comportamento
	// atual. O contexto compilado é injetado no system prompt; a camada
	// decidida (L0/L1/L2) fica registrada no PipelineContext.
	if e.config.ContextPipeline != nil {
		// 3.5.1 ── TASK-AWARE SEARCH (ADR-045, opt-in via EnableTAS) ──
		// Quando habilitado e o TaskContext ainda não veio preenchido pelo
		// chamador, deriva o perfil de tarefa a partir do estado de
		// implementação disponível no executor (o Prompt + metadados que o
		// editor/Request.Context injetou via PipelineData.Extra). O perfil
		// re-pondera a busca por afinidade e ajusta-a por fase DENTRO do
		// BuildContext (contextpipeline). Aditivo e fail-closed: se o estado
		// não permite derivar um perfil confiável, BuildTaskContext retorna nil
		// e o fluxo segue idêntico ao atual.
		if e.config.EnableTAS && pc.Data.TaskContext == nil {
			pc.Data.TaskContext = tasTaskContextFrom(pc.Data)
		}

		compiled, level := e.config.ContextPipeline.BuildContext(pc.Prompt, &pc.Data)
		if compiled != "" {
			systemContent += "\n\n" + compiled
		}
		pc = pc.WithPipelineLevel(level)
	}

	// 4. Build the message list.
	messages := []chat.Message{
		{Role: chat.RoleSystem, Content: systemContent},
		{Role: chat.RoleUser, Content: augmentedPrompt},
	}

	// 5. Build ChatOptions with tools derived from agent capabilities.
	opts := e.buildChatOptions(ctx, pc.Data)

	// 5.5 ── MODO DETERMINÍSTICO: a IA é o último recurso ─────────────
	// Se o conhecimento (knowledge.db) já responde com precisão, o Cosca
	// responde SEM chamar o LLM. O motor é opcional — a inteligência está
	// na casa, não no motor (L427: "antes de chamar a LLM, consulte o
	// Cosca"). O conhecimento veio do context builder (já validado).

	// 5.5.0 ── DELIBERAÇÃO DO KERNEL (ADR-032) ──
	// Quando a etapa Kernel-First Deliberation já produziu a resposta final
	// (EmitOK), o Executor pula a LLM de forma aditiva e reversível: o flag
	// DeliberationHandled é checado ANTES da heurística deterministicResponse
	// para que a resposta do Kernel nunca seja sobrescrita. Quando false, o
	// Executor comporta-se exatamente como antes.
	if pc.Data.DeliberationHandled {
		logger.Info().Msg("executor: resposta DETERMINÍSTICA (deliberação do kernel) — pulando LLM")
		return pc, nil
	}

	// ── ROTEAMENTO POR INTENÇÃO (dado do plano, não heurística aqui) ────────
	// A intenção da task (detectTaskType, decidida no planejamento e chegando
	// via Context["intent"]) determina o caminho: tasks de AÇÃO/IMPLEMENTAÇÃO
	// devem IR AO LLM COM TOOLS — nunca ser resolvidas pelo knowledge
	// determinístico (que pode responder com o auto-match do workflow). Tasks
	// de CONSULTA/AVALIAÇÃO continuam usando o determinístico. Isso preserva o
	// deterministicResponse global para consulta e só blinda as ações.
	if intent, ok := pc.Data.Extra["intent"].(string); ok && intent != "" && IsActionIntent(intent) {
		logger.Info().Str("intent", intent).Msg("executor: task de AÇÃO/IMPLEMENTAÇÃO — segue ao LLM com tools (determinístico bloqueado)")
	} else if det := deterministicResponse(pc.Data); det != "" {
		logger.Info().Msg("executor: resposta DETERMINÍSTICA (sem LLM) — conhecimento indexado respondeu")
		pc = pc.WithLLMResponse(det)
		pc = pc.WithExecutorDeterministic(true)
		return pc, nil
	}

	// 5.75 ── REDE DE CUSTO (opt-in): teto por execução ───────────────
	// Se um Budget foi configurado, cada chamada de IA é registrada (tokens,
	// tempo, custo) e o laço de tool-calls PARA de chamar o provider quando o
	// teto estoura. O tracker é criado POR EXECUÇÃO (não no Executor), para
	// que requisições concorrentes sobre o mesmo Executor não compartilhem
	// consumo. nil = sem trava (comportamento atual).
	var budgetTracker *BudgetTracker
	if e.config.Budget != nil {
		budgetTracker = NewBudgetTracker(*e.config.Budget)
	}

	// 6. Call provider.Chat() with retry.
	respondStart := time.Now()
	response, err := e.chatWithRetry(ctx, messages, opts)
	respondElapsed := time.Since(respondStart)
	if err != nil {
		info := safeError("chat_completion_failed", err)
		logger.Error().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("chat completion failed after retries")
		// Record fallback if retries were used (even though call ultimately failed).
		if atomic.LoadInt64(&e.lastAttemptCount) > 1 {
			pc = pc.WithExecutorFallback(true)
		}
		return pc, safePublicError("executor: chat failed", "chat_completion_failed", err)
	}

	// Record whether a fallback (retry) was used.
	if atomic.LoadInt64(&e.lastAttemptCount) > 1 {
		pc = pc.WithExecutorFallback(true)
	}

	// Record the initial call's consumption against the budget (best-effort).
	e.recordConsumption(budgetTracker, response, respondElapsed)

	// 7. Parse response: extract content and tool calls.
	content, toolCalls := e.parseResponse(response)

	// Acumula tool results de todas as rodadas (usado no fallback final:
	// se o LLM devolver resposta vazia, renderiza o resultado real das tools).
	var allToolResults []*ToolCallResult

	// 8. Store results in the pipeline context.
	pc = pc.WithLLMResponse(content)
	pc = pc.WithLLMModel(response.Model)
	pc = pc.WithLLMUsage(response.Usage)

	if len(toolCalls) > 0 {
		pc = pc.WithToolCalls(toolCalls)

		// Diagnostic: confirm toolRunner wiring in the serve path.
		logger.Info().
			Bool("tool_executor_wired", e.toolRunner != nil).
			Int("tool_call_count", len(toolCalls)).
			Strs("tool_names", toolCallNames(toolCalls)).
			Msg("executor: tool calls detected (diagnostic)")

		if e.toolRunner != nil {
			roundMessages := make([]chat.Message, len(messages))
			copy(roundMessages, messages)

			for round := 0; round < e.config.MaxToolRounds && len(toolCalls) > 0; round++ {
				// ── REDE DE CUSTO: para de chamar o provider quando o teto
				// estoura (tokens, tempo ou custo). Best-effort: PARAR novas
				// chamadas, mas NÃO derrubar a execução — devolve o que já
				// acumulamos (parcial/último conteúdo + tool results).
				if budgetTracker != nil && !budgetTracker.CanCall() {
					logger.Warn().
						Int("round", round+1).
						Str("consumption", budgetTracker.Summary()).
						Str("budget", budgetString(*e.config.Budget)).
						Msg("executor: budget de custo estourado — interrompendo laço de tool-calls")
					break
				}

				logger.Info().
					Int("round", round+1).
					Int("tool_call_count", len(toolCalls)).
					Strs("tool_names", toolCallNames(toolCalls)).
					Msg("tool calls detected in response")

				toolResults, toolErr := e.toolRunner.ExecuteAll(ctx, toolCalls)
				if toolErr != nil {
					info := safeError("tool_calls_failed", toolErr)
					logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("some tool calls failed")
				}
				pc = pc.WithToolResults(toolResults)
				allToolResults = append(allToolResults, toolResults...)

				roundMessages = append(roundMessages, chat.Message{
					Role:      chat.RoleAssistant,
					Content:   content,
					ToolCalls: toolCalls,
				})
				for _, tr := range toolResults {
					toolContent := tr.Content
					if tr.Error != "" {
						toolContent = fmt.Sprintf("Tool execution error: %s\nTool output:\n%s", tr.Error, toolContent)
					}
					roundMessages = append(roundMessages, chat.Message{
						Role:       chat.RoleTool,
						Content:    contenttrust.Envelope(contenttrust.Default(contenttrust.OriginTool, toolContent, tr.Name)),
						ToolCallID: tr.ToolCallID,
					})
				}

				followUpStart := time.Now()
				followUpResponse, followUpErr := e.chatWithRetry(ctx, roundMessages, opts)
				followUpElapsed := time.Since(followUpStart)
				if followUpErr != nil {
					info := safeError("follow_up_chat_failed", followUpErr)
					logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("follow-up chat after tool calls failed, returning semantic knowledge")
					// ── Fallback: model cannot complete the tool round
					// trip (common with coding models via ollama). Prefer
					// the SEMANTIC knowledge (knowledge.db) that the context
					// builder already injected — that is how the Cosca
					// awakens. Tool results (codebase matches) are the
					// fallback of the fallback.
					if content = formatKnowledgeFallback(pc.Data.KnowledgeResults); content == "" {
						content = formatToolResultsFallback(toolResults)
					}
					pc = pc.WithLLMResponse(content)
					break
				}

				content, toolCalls = e.parseResponse(followUpResponse)
				pc = pc.WithLLMResponse(content)
				pc = pc.WithLLMModel(followUpResponse.Model)
				pc = pc.WithLLMUsage(followUpResponse.Usage)
				response = followUpResponse

				// Record the follow-up call's consumption against the budget
				// (best-effort) — so the NEXT round's CanCall sees the true sum.
				// Note: elapsed is the time for this follow-up call only, which
				// is exactly what BudgetTimestamp durations expect.
				e.recordConsumption(budgetTracker, followUpResponse, followUpElapsed)

				// ── Fallback: model returned empty follow-up ────────────
				// Some coding models (qwen2.5-coder via ollama) do not
				// understand structured tool-call round trips and answer
				// with an empty message. Prefer semantic knowledge first.
				if strings.TrimSpace(content) == "" && len(toolCalls) == 0 {
					logger.Warn().Msg("follow-up returned empty — using semantic knowledge as final response")
					if content = formatKnowledgeFallback(pc.Data.KnowledgeResults); content == "" {
						content = formatToolResultsFallback(toolResults)
					}
					pc = pc.WithLLMResponse(content)
				}
			}

			// ── PENDING RESOLUTION (Don + professor, 2026-09-01) ──
			// O loop terminou por limite de tool-rounds (round >= MaxToolRounds)
			// mas AINDA há tool calls pendentes — o "abandona na reta final".
			// Em vez de largar, inspeciona o ESTADO: se a pendência é
			// resolvível (ação já implicada pelo estado), registra a
			// continuação mínima; senão, finaliza com registro honesto. Nunca
			// inventa próximo passo.
			//
			// REGRA ARQUITETURAL (professor): a Pending Resolution NUNCA
			// executa ferramentas diretamente — ela produz uma CONTINUAÇÃO
			// PROPOSTA (sinal na resposta). O caminho de execução volta pelo
			// fluxo normal: LLM → Action Decoder → Policy Guard → Tool
			// Executor. Não existe "atalho secreto" que contorne as proteções.
			if len(toolCalls) > 0 && e.config.PendingResolver != nil {
				st := pendingStateOf(pc.Data, e.config.MaxToolRounds)
				st.PendingActions = toolCallNames(toolCalls)
				pres := e.config.PendingResolver.Inspect(st)
				if pres.Verdict == pending.Resolve {
					logger.Info().
						Strs("pending_tools", toolCallNames(toolCalls)).
						Msg("executor: pendência resolvível detectada — continuação mínima implicada")
					pc = pc.WithLLMResponse(fmt.Sprintf(
						"[pending-resolved] %d tool call(s) pendentes implicados pelo estado; continuação mínima registrada.",
						len(toolCalls)))
				} else {
					logger.Warn().
						Strs("pending_tools", toolCallNames(toolCalls)).
						Msg("executor: tool calls pendentes não resolvíveis — finalizando sem inventar próximo passo")
				}
			}
		}
	}

	logger.Debug().
		Str("agent", agentName).
		Str("model", response.Model).
		Int("prompt_tokens", response.Usage.PromptTokens).
		Int("completion_tokens", response.Usage.CompletionTokens).
		Msg("execution complete")

	// ── Fallback final (Tool Execution Policy §5 / §7) ─────────────────────
	// Se o LLM retornou resposta vazia mas houve tool calls executadas,
	// renderiza o resultado real das tools como texto final. Isso impede que
	// um agente "Chief" (que planeja via tool calls sem texto) apareça como
	// "No response content" quando na verdade executou trabalho útil.
	if strings.TrimSpace(pc.Data.LLMResponse) == "" && len(allToolResults) > 0 {
		logger.Warn().Msg("final response empty but tool results exist — rendering tool results as final response")
		if content = formatToolResultsFallback(allToolResults); content != "" {
			pc = pc.WithLLMResponse(content)
		}
	}

	return pc, nil
}

// ─── Streaming Execution ─────────────────────────────────────────────────────

// ExecuteStream runs the agent in streaming mode. It sets up the messages,
// opens a ChatStream, and forwards chunks to the eventCh channel. The method
// returns immediately after starting the background streaming goroutine and
// closes eventCh when the stream ends or the context is cancelled.
func (e *Executor) ExecuteStream(ctx context.Context, pc PipelineContext, eventCh chan<- StreamEvent) (PipelineContext, error) {
	logger := log.Ctx(ctx).With().Str("stage", "executor_stream").Str("request_id", pc.RequestID).Logger()

	// 1. Extract the augmented prompt.
	augmentedPrompt := pc.Data.AugmentedPrompt
	if augmentedPrompt == "" {
		augmentedPrompt = pc.Prompt
	}

	// 2. Read resolved agent information.
	agentName := pc.Data.ResolvedAgent
	agentRole := pc.Data.AgentRole
	agentDept := pc.Data.AgentDepartment
	agentDesc := pc.Data.AgentDescription

	if agentName == "" {
		agentName = "COSCA KERNEL"
	}
	if agentRole == "" {
		agentRole = agentName
	}

	// 3. Build the system prompt.
	systemContent := e.buildSystemPrompt(agentName, agentRole, agentDept, agentDesc, pc.Data)

	// 4. Build the message list.
	messages := []chat.Message{
		{Role: chat.RoleSystem, Content: systemContent},
		{Role: chat.RoleUser, Content: augmentedPrompt},
	}

	// 5. Build ChatOptions with stream=true.
	opts := e.buildChatOptions(ctx, pc.Data)
	opts.Stream = true

	// Kill-switch guard (Etapa 3b): kernel haltado → bloqueia o streaming
	// antes de abrir a chamada.
	if e.haltBlocked() {
		info := safeError("kernel_halted", fmt.Errorf("kernel haltado (kill-switch acionado) — execução bloqueada"))
		e.emitEvent(eventCh, StreamEvent{
			Type:    StreamEventError,
			Content: safeErrorMessage(info.Code), Metadata: safeErrorEvent(info.Code, fmt.Errorf("kernel halted")),
		})
		return pc, nil
	}

	// 6. Open the chat stream. This is the only synchronous I/O; the rest
	//    runs in the background goroutine.
	stream, err := e.provider.ChatStream(ctx, messages, opts)
	if err != nil {
		info := safeError("chat_stream_open_failed", err)
		e.emitEvent(eventCh, StreamEvent{
			Type:    StreamEventError,
			Content: safeErrorMessage(info.Code), Metadata: safeErrorEvent(info.Code, err),
		})
		// The caller owns eventCh until a background reader is started. Do
		// not close it here: ExecuteStream may still emit the startup error.
		return pc, safePublicError("executor: chat stream failed", "chat_stream_open_failed", err)
	}

	// 7. Emit progress event to signal the start of streaming.
	e.emitEvent(eventCh, StreamEvent{
		Type:    StreamEventProgress,
		Content: fmt.Sprintf("Starting streaming execution with agent %s", agentName),
		Metadata: map[string]interface{}{
			"agent":  agentName,
			"role":   agentRole,
			"model":  e.provider.Model(),
			"stream": true,
		},
	})

	// 8. Start the background reader goroutine.
	go e.readStream(ctx, stream, eventCh, logger)

	return pc, nil
}

// ─── Stream Reader (background goroutine) ────────────────────────────────────

// readStream reads chunks from the ChatStream and forwards them as
// StreamEvent values on eventCh. It handles stage transitions, accumulates
// the full response, and closes eventCh when the stream ends.
func (e *Executor) readStream(ctx context.Context, stream chat.ChatStream, eventCh chan<- StreamEvent, logger zerolog.Logger) {
	defer func() {
		if err := stream.Close(); err != nil {
			info := safeError("stream_close_failed", err)
			logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("error closing chat stream")
		}
		close(eventCh)
	}()

	var (
		fullContent  strings.Builder
		toolCalls    []chat.ToolCall
		toolCallMu   sync.Mutex
		chunkCount   int
		currentStage string
	)

	for {
		// Check context cancellation before blocking on Recv.
		select {
		case <-ctx.Done():
			e.emitEvent(eventCh, StreamEvent{
				Type:    StreamEventError,
				Content: safeErrorMessage("stream_cancelled"), Metadata: safeErrorEvent("stream_cancelled", ctx.Err()),
			})
			return
		default:
		}

		chunk, err := stream.Recv()
		if err != nil {
			// Stream exhausted or broken.
			e.emitEvent(eventCh, StreamEvent{
				Type:    StreamEventError,
				Content: safeErrorMessage("stream_read_failed"), Metadata: safeErrorEvent("stream_read_failed", err),
			})
			return
		}

		// A nil chunk with no choices means the stream is done.
		if chunk == nil || len(chunk.Choices) == 0 {
			return
		}

		chunkCount++

		for _, choice := range chunk.Choices {
			// --- Stage transition detection ---
			// Look for marker patterns like "[STAGE: name]" in the delta content.
			if newStage := e.detectStageTransition(choice.Delta.Content); newStage != "" && newStage != currentStage {
				currentStage = newStage
				e.emitEvent(eventCh, StreamEvent{
					Type:    StreamEventStageTransition,
					Content: fmt.Sprintf("Entering stage: %s", currentStage),
					Metadata: map[string]interface{}{
						"stage": currentStage,
					},
				})
			}

			// --- Text delta ---
			if choice.Delta.Content != "" {
				fullContent.WriteString(choice.Delta.Content)
				e.emitEvent(eventCh, StreamEvent{
					Type:    StreamEventChunk,
					Content: choice.Delta.Content,
					Metadata: map[string]interface{}{
						"index":       choice.Index,
						"chunk_count": chunkCount,
					},
				})
			}

			// --- Tool call deltas ---
			if len(choice.Delta.ToolCalls) > 0 {
				toolCallMu.Lock()
				for _, tc := range choice.Delta.ToolCalls {
					// Merge with existing tool calls by ID.
					merged := false
					for i, existing := range toolCalls {
						if existing.ID == tc.ID {
							toolCalls[i].Function.Name += tc.Function.Name
							toolCalls[i].Function.Arguments += tc.Function.Arguments
							merged = true
							break
						}
					}
					if !merged {
						toolCalls = append(toolCalls, tc)
					}
				}
				toolCallMu.Unlock()
			}

			// --- Finish reason — stream complete ---
			if choice.FinishReason != "" {
				result := fullContent.String()

				e.emitEvent(eventCh, StreamEvent{
					Type: StreamEventProgress,
					Content: fmt.Sprintf("Stream complete. Reason: %s. Total chunks: %d, Response length: %d chars",
						choice.FinishReason, chunkCount, len(result)),
					Metadata: map[string]interface{}{
						"finish_reason":  string(choice.FinishReason),
						"chunk_count":    chunkCount,
						"content_length": len(result),
					},
				})

				// Store tool calls if any were accumulated.
				if len(toolCalls) > 0 {
					e.emitEvent(eventCh, StreamEvent{
						Type:    StreamEventProgress,
						Content: fmt.Sprintf("Tool calls detected: %d", len(toolCalls)),
						Metadata: map[string]interface{}{
							"tool_calls": toolCalls,
						},
					})
				}

				return
			}
		}

		// Emit periodic progress for long streams.
		if chunkCount%50 == 0 {
			e.emitEvent(eventCh, StreamEvent{
				Type:    StreamEventProgress,
				Content: fmt.Sprintf("Streaming in progress... %d chunks received", chunkCount),
				Metadata: map[string]interface{}{
					"chunk_count": chunkCount,
				},
			})
		}
	}
}

// ─── Prompt Building ─────────────────────────────────────────────────────────

// buildSystemPrompt constructs a system prompt from agent metadata and any
// contextual knowledge retrieved earlier in the pipeline.
func (e *Executor) buildSystemPrompt(agentName, agentRole, agentDept, agentDesc string, data PipelineData) string {
	var sb strings.Builder

	// Agent identity.
	fmt.Fprintf(&sb, "You are %s", agentName)
	if agentRole != "" && agentRole != agentName {
		fmt.Fprintf(&sb, ", %s", agentRole)
	}
	if agentDept != "" {
		fmt.Fprintf(&sb, " from the %s department", agentDept)
	}
	sb.WriteString(".\n")

	if agentDesc != "" {
		fmt.Fprintf(&sb, "\nYour purpose: %s\n", agentDesc)
	}

	// Capabilities & responsibilities of this agent (from the agent definition).
	// Injecting them gives the LLM the concrete operational identity of the
	// agent — without this, a "Chief" loses its role/standards and can
	// respond with an empty message instead of acting.
	if caps := data.AgentCapabilities; len(caps) > 0 {
		sb.WriteString("\nYour capabilities:\n")
		for i, c := range caps {
			fmt.Fprintf(&sb, "  %d. %s\n", i+1, c)
		}
	}
	if reps := data.AgentResponsibilities; len(reps) > 0 {
		sb.WriteString("\nYour responsibilities:\n")
		for i, r := range reps {
			fmt.Fprintf(&sb, "  %d. %s\n", i+1, r)
		}
	}

	// Knowledge context injected by the knowledge-retrieval stage.
	// PLANNING × EXECUTION (context budgeting): para tasks de AÇÃO, o plano já
	// carregou o conhecimento; o executor deve trabalhar CIRÚRGICO (task + tools
	// + evidência). Injetar knowledge/memory gigantes aqui faz o modelo entrar
	// em modo "analista/documentador" e NÃO emitir tool-call. Então, para tasks
	// de ação, pulamos knowledge_context/memory_context.
	isAction := false
	if intent, ok := data.Extra["intent"].(string); ok && intent != "" {
		isAction = IsActionIntent(intent)
	}
	if !isAction {
		if knowledge, ok := data.Extra["knowledge_context"].(string); ok && knowledge != "" {
			sb.WriteString("\n--- RELEVANT KNOWLEDGE ---\n")
			sb.WriteString(knowledge)
			sb.WriteString("\n--- END KNOWLEDGE ---\n")
		}
		if data.MemoryContext != "" {
			sb.WriteString("\n--- RELEVANT MEMORY ---\n")
			sb.WriteString(data.MemoryContext)
			sb.WriteString("\n--- END MEMORY ---\n")
		}
	}

	// Tools available to this agent (derived from role/department). Enumerating
	// them here is what makes the model actually invoke them — without the list,
	// a provider like deepseek tends to answer in prose instead of calling a tool.
	tools := e.deriveTools(data)
	if len(tools) > 0 {
		sb.WriteString("\n--- AVAILABLE TOOLS ---\n")
		sb.WriteString("You have the following tools you MUST use to accomplish the task:\n")
		for i, t := range tools {
			params := t.Function.Parameters
			fmt.Fprintf(&sb, "%d. %s: %s. Parameters: %v\n", i+1, t.Function.Name, t.Function.Description, params)
		}
		sb.WriteString("When a task requires creating/editing files or running commands, use the appropriate tool rather than describing what you would do. Call the tool directly.\n")
		sb.WriteString("--- END AVAILABLE TOOLS ---\n")
	}

	// General instructions.
	sb.WriteString("\nProvide a thorough, well-reasoned response. ")
	sb.WriteString("Use the available context above when relevant. ")
	sb.WriteString("If a task needs a tool, CALL it (never describe the action without invoking the tool).")

	return sb.String()
}

// ─── Chat Options ────────────────────────────────────────────────────────────

// buildChatOptions constructs ChatOptions by deriving tool definitions from
// the agent's capabilities stored in the pipeline context.
func (e *Executor) buildChatOptions(ctx context.Context, data PipelineData) chat.ChatOptions {
	opts := chat.DefaultChatOptions()

	// Vocabulário de ferramentas ÚNICO (Opção B, Etapa 3): quando o ToolRunner
	// canônico está injetado, o LLM vê as tools REAIS do registry
	// (read/write/edit/glob/shell/search/git/build/test...) — o mesmo conjunto
	// de todos os caminhos. Fallback para deriveTools apenas quando não há
	// executor (sem regressão).
	if e.toolRunner != nil {
		if tools, err := e.toolRunner.ListTools(ctx); err == nil && len(tools) > 0 {
			opts.Tools = tools
			return opts
		}
	}

	tools := e.deriveTools(data)
	if len(tools) > 0 {
		opts.Tools = tools
	}

	return opts
}

// deriveTools produces tool definitions based on agent role and skills.
// This is a best-effort derivation; actual tool schemas would be loaded
// from a skill registry in a full implementation.
func (e *Executor) deriveTools(data PipelineData) []chat.ToolDefinition {
	// Collect hints from context data.
	agentRole := data.AgentRole
	agentDept := data.AgentDepartment

	var tools []chat.ToolDefinition
	seen := make(map[string]bool)

	// Map agent roles/departments to common tool categories.
	toolHints := e.toolHintsForRole(agentRole, agentDept)

	for _, hint := range toolHints {
		if seen[hint.Name] {
			continue
		}
		seen[hint.Name] = true
		tools = append(tools, chat.ToolDefinition{
			Type: "function",
			Function: chat.FunctionDef{
				Name:        hint.Name,
				Description: hint.Description,
				Parameters:  hint.Parameters,
			},
		})
	}

	return tools
}

// toolHint is a lightweight tool descriptor used for capability derivation.
type toolHint struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// toolHintsForRole returns tool hints based on the agent's role and
// department. This is a simplified mapping; a full implementation would
// load tool schemas from a skill/capability registry.
func (e *Executor) toolHintsForRole(role, dept string) []toolHint {
	roleLower := strings.ToLower(role)
	deptLower := strings.ToLower(dept)

	var hints []toolHint

	if strings.Contains(roleLower, "backend") || strings.Contains(deptLower, "backend") {
		hints = append(hints, toolHint{
			Name:        "read_file",
			Description: "Read the contents of a file",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}},
		})
		hints = append(hints, toolHint{
			Name:        "write_file",
			Description: "Write content to a file",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}}},
		})
		hints = append(hints, toolHint{
			Name:        "execute_command",
			Description: "Execute a shell command (build/test/scaffold), confined to the workspace allowlist",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string"}}},
		})
	}

	if strings.Contains(roleLower, "frontend") || strings.Contains(deptLower, "frontend") {
		hints = append(hints, toolHint{
			Name:        "read_file",
			Description: "Read the contents of a file",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}},
		})
		hints = append(hints, toolHint{
			Name:        "write_file",
			Description: "Write content to a file",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}}},
		})
		hints = append(hints, toolHint{
			Name:        "execute_command",
			Description: "Execute a shell command (build/test/scaffold), confined to the workspace allowlist",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string"}}},
		})
	}

	if strings.Contains(roleLower, "database") || strings.Contains(deptLower, "database") {
		hints = append(hints, toolHint{
			Name:        "execute_sql",
			Description: "Execute a SQL query against the database",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}},
		})
	}

	if strings.Contains(roleLower, "test") || strings.Contains(deptLower, "testing") || strings.Contains(deptLower, "qa") {
		hints = append(hints, toolHint{
			Name:        "run_tests",
			Description: "Run the test suite",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{"filter": map[string]any{"type": "string"}}},
		})
	}

	if strings.Contains(roleLower, "devops") || strings.Contains(deptLower, "devops") {
		hints = append(hints, toolHint{
			Name:        "execute_command",
			Description: "Execute a shell command",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string"}}},
		})
	}

	if strings.Contains(roleLower, "security") || strings.Contains(deptLower, "security") {
		hints = append(hints, toolHint{
			Name:        "scan_vulnerabilities",
			Description: "Scan code for security vulnerabilities",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}},
		})
	}

	// File operations are useful for almost all agents.
	hints = append(hints, toolHint{
		Name:        "search_codebase",
		Description: "Search the codebase for patterns or content",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}},
	})

	return hints
}

// ─── Retry Logic ─────────────────────────────────────────────────────────────

// chatAttemptResult carries the outcome of a single non-cooperative attempt.
type chatAttemptResult struct {
	resp *chat.ChatResponse
	err  error
}

// runChatAttempt executes provider.Chat in a goroutine and selects on the
// deadline — closing the L324 debt: a provider that IGNORES context
// cancellation (the "loading forever" case) no longer blocks the worker until
// it decides to answer; the worker reclaims control at the deadline and treats
// it as a stall. The channel is buffered (1) so a late-returning goroutine
// never blocks on send.
func (e *Executor) runChatAttempt(attemptCtx, parentCtx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	done := make(chan chatAttemptResult, 1)
	go func() {
		resp, err := e.provider.Chat(attemptCtx, messages, opts)
		done <- chatAttemptResult{resp: resp, err: err}
	}()

	select {
	case r := <-done:
		return r.resp, r.err
	case <-attemptCtx.Done():
		// Deadline fired while the provider was still running: it does not
		// respect cancellation. Reclaim control now — non-cooperative stall.
		return nil, stallwatch.ErrAttemptTimedOut
	case <-parentCtx.Done():
		// Parent cancelled — not a stall.
		return nil, parentCtx.Err()
	}
}

// chatWithRetry calls provider.Chat with exponential backoff retry on
// transient errors. It respects the configured MaxRetries, RetryDelay, and
// the context deadline.
func (e *Executor) chatWithRetry(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	var lastErr error
	var attemptCount int64

	for attempt := 0; attempt <= e.config.MaxRetries; attempt++ {
		attemptCount++
		// Check context before each attempt.
		if err := ctx.Err(); err != nil {
			atomic.StoreInt64(&e.lastAttemptCount, attemptCount)
			return nil, safeContextError("chat_context_cancelled", err)
		}

		// Kill-switch guard (Etapa 3b): kernel haltado → nenhuma chamada LLM.
		if e.haltBlocked() {
			atomic.StoreInt64(&e.lastAttemptCount, attemptCount)
			return nil, safePublicError("chat", "kernel_halted", fmt.Errorf("kernel haltado (kill-switch acionado) — execução bloqueada"))
		}

		// Create a per-attempt context with the configured timeout.
		attemptCtx, cancel := context.WithTimeout(ctx, e.config.Timeout)
		start := time.Now()
		response, err := e.runChatAttempt(attemptCtx, ctx, messages, opts)
		waited := time.Since(start)
		cancel()

		spec := stallwatch.WatchSpec{
			Name:      "llm.chat",
			Provider:  e.provider.Name(),
			Model:     e.provider.Model(),
			Timeout:   e.config.Timeout,
			Retryable: isTransientError,
		}

		if err == nil {
			if attemptCount > 1 {
				e.recordStall(spec, stallwatch.ActionRecovered, waited, int(attemptCount-1), "")
			}
			atomic.StoreInt64(&e.lastAttemptCount, attemptCount)
			return response, nil
		}

		lastErr = err

		// A deadline exceeded (cooperative or non-cooperative) means the
		// provider stopped responding — a stall.
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, stallwatch.ErrAttemptTimedOut) {
			e.recordStall(spec, stallwatch.ActionStall, waited, int(attemptCount-1), err.Error())
		}

		// Don't retry if this is the last attempt.
		if attempt == e.config.MaxRetries {
			break
		}

		// Only retry on transient errors (including non-cooperative stalls,
		// which are always retryable: ErrAttemptTimedOut is the watchdog's
		// own signal, not a provider verdict).
		if !isTransientError(err) && !errors.Is(err, stallwatch.ErrAttemptTimedOut) {
			info := safeError("chat_completion_failed", err)
			log.Ctx(ctx).Warn().
				Str("error_code", info.Code).
				Str("error_hash", info.Hash).
				Int("error_length", info.Length).
				Int("attempt", attempt+1).
				Msg("non-transient chat error, not retrying")
			e.recordStall(spec, stallwatch.ActionFailed, waited, int(attemptCount-1), err.Error())
			break
		}

		// Exponential backoff.
		delay := e.config.RetryDelay * time.Duration(int64(math.Pow(2, float64(attempt))))

		e.recordStall(spec, stallwatch.ActionRetry, waited, int(attemptCount-1), "")

		info := safeError("chat_retryable_failure", err)
		log.Ctx(ctx).Warn().
			Str("error_code", info.Code).
			Str("error_hash", info.Hash).
			Int("error_length", info.Length).
			Int("attempt", attempt+1).
			Int("max_retries", e.config.MaxRetries).
			Dur("retry_delay", delay).
			Msg("transient chat error, retrying")

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			atomic.StoreInt64(&e.lastAttemptCount, attemptCount)
			return nil, safeContextError("chat_context_cancelled", ctx.Err())
		case <-timer.C:
		}
	}

	atomic.StoreInt64(&e.lastAttemptCount, attemptCount)
	if e.stalls != nil {
		e.recordStall(stallwatch.WatchSpec{Name: "llm.chat", Provider: e.provider.Name(), Model: e.provider.Model(), Timeout: e.config.Timeout}, stallwatch.ActionFailed, 0, int(attemptCount-1), "chat retries exhausted")
	}
	return nil, safeContextError("chat_completion_failed", lastErr)
}

// isTransientError returns true when the error is likely recoverable with a
// retry (timeouts, rate limits, server errors, network failures).
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	transientMarkers := []string{
		"timeout",
		"deadline exceeded",
		"rate limit",
		"rate exceeded",
		"too many requests",
		"service unavailable",
		"503",
		"server error",
		"internal server error",
		"bad gateway",
		"gateway timeout",
		"connection reset",
		"connection refused",
		"temporary failure",
		"retry",
		"throttled",
		"too busy",
		"busy",
		"capacity",
		"overloaded",
		"eof",
		"broken pipe",
	}

	for _, marker := range transientMarkers {
		if strings.Contains(msg, marker) {
			return true
		}
	}

	return false
}

// ─── Response Parsing ────────────────────────────────────────────────────────

// parseResponse extracts the assistant's text content and any tool calls
// from a ChatResponse. Handles both standard OpenAI tool_calls format and
// Ollama-style JSON-encoded tool calls in the content field.
func (e *Executor) parseResponse(response *chat.ChatResponse) (string, []chat.ToolCall) {
	if response == nil || len(response.Choices) == 0 {
		return "", nil
	}

	choice := response.Choices[0]
	content := choice.Message.Content
	toolCalls := choice.Message.ToolCalls

	if len(toolCalls) == 0 && strings.TrimSpace(content) != "" {
		parsedTools := e.tryParseContentTools(content)
		if len(parsedTools) > 0 {
			// A "response" wrapper resolves to final text; anything else
			// resolves to tool calls.
			if parsedTools[0].Content != "" {
				return parsedTools[0].Content, nil
			}
			return "", parsedTools[0].ToolCalls
		}
	}

	return content, toolCalls
}

type parsedContentResult struct {
	Content   string
	ToolCalls []chat.ToolCall
}

func (e *Executor) tryParseContentTools(content string) []parsedContentResult {
	parts := e.extractJSONBlocks(content)
	if len(parts) == 0 {
		return nil
	}

	var results []parsedContentResult
	for _, part := range parts {
		var toolCallData map[string]interface{}
		if err := json.Unmarshal([]byte(part), &toolCallData); err != nil {
			continue
		}

		toolName, _ := toolCallData["name"].(string)
		if toolName == "" {
			continue
		}

		// ── Special tool: response ─────────────────────────────────────
		// Some coding models (qwen2.5-coder via ollama) never emit plain
		// text — they wrap final answers in {"name":"response","arguments":
		// {"message":"..."}}. Treat that as the final text response instead
		// of a tool call (the executor has no "response" tool).
		if toolName == "response" {
			if args, ok := toolCallData["arguments"].(map[string]interface{}); ok {
				if msg, ok := args["message"].(string); ok && msg != "" {
					results = append(results, parsedContentResult{Content: msg})
					continue
				}
			}
		}

		var argsStr string
		if args, ok := toolCallData["arguments"]; ok {
			switch v := args.(type) {
			case string:
				argsStr = v
			case map[string]interface{}:
				argsBytes, err := json.Marshal(v)
				if err == nil {
					argsStr = string(argsBytes)
				}
			}
		}

		if argsStr == "" {
			argsStr = "{}"
		}

		tc := chat.ToolCall{
			ID:   fmt.Sprintf("parsed-%s", uuid.New().String()[:8]),
			Type: "function",
			Function: chat.FunctionCall{
				Name:      toolName,
				Arguments: argsStr,
			},
		}

		results = append(results, parsedContentResult{ToolCalls: []chat.ToolCall{tc}})
	}

	return results
}

func (e *Executor) extractJSONBlocks(content string) []string {
	var blocks []string

	trimmed := strings.TrimSpace(content)

	// ── Fast path: content is a raw JSON tool call (no code fence) ─────
	// Some models (e.g. qwen2.5-coder via ollama) return the tool call as
	// bare JSON in the content instead of a structured tool_call or a
	// ```json fence. Detect and extract it directly.
	if strings.HasPrefix(trimmed, "{") {
		var probe map[string]interface{}
		if json.Unmarshal([]byte(trimmed), &probe) == nil {
			if _, ok := probe["name"].(string); ok {
				return []string{trimmed}
			}
		}
	}

	remaining := content
	for {
		start := strings.Index(remaining, "```json")
		if start < 0 {
			start = strings.Index(remaining, "```")
			if start < 0 {
				break
			}
		}

		inner := remaining[start:]
		if strings.HasPrefix(inner, "```json") {
			inner = inner[7:]
		} else if strings.HasPrefix(inner, "```") {
			inner = inner[3:]
		} else {
			remaining = remaining[start+3:]
			continue
		}

		end := strings.Index(inner, "```")
		if end < 0 {
			break
		}

		block := strings.TrimSpace(inner[:end])
		// O fence pode vir com a linguagem: ```shell<newline>{...}``` .
		// O extractor antes exigia "{"-prefixo, mas a primeira linha do bloco
		// e' a linguagem (e.g. "shell", "json", "go"), entao um tool-call JSON
		// emitido num fence com linguagem era DESCARTADO (o modelo menor com
		// 64K emitia write_file assim). Se o bloco nao comeca com "{", pula a
		// primeira linha e re-testa.
		if !strings.HasPrefix(block, "{") {
			if nl := strings.Index(block, "\n"); nl >= 0 {
				block = strings.TrimSpace(block[nl+1:])
			}
		}
		if strings.HasPrefix(block, "{") {
			var test map[string]interface{}
			if json.Unmarshal([]byte(block), &test) == nil {
				if _, ok := test["name"].(string); ok {
					blocks = append(blocks, block)
				}
			}
		}

		remaining = inner[end+3:]
	}

	return blocks
}

// ─── Stage Detection ─────────────────────────────────────────────────────────

// detectStageTransition scans text for a stage-marker pattern like
// "[STAGE: name]" and returns the stage name if found.
func (e *Executor) detectStageTransition(text string) string {
	const prefix = "[STAGE:"
	const suffix = "]"

	start := strings.Index(text, prefix)
	if start < 0 {
		return ""
	}
	start += len(prefix)
	end := strings.Index(text[start:], suffix)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(text[start : start+end])
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// emitEvent is a non-blocking send on the event channel. If the channel is
// full or the context is done the event is silently dropped (best-effort).
func (e *Executor) emitEvent(eventCh chan<- StreamEvent, ev StreamEvent) {
	// Use a short timeout to avoid blocking indefinitely if the consumer
	// is slow or has stopped reading.
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()

	select {
	case eventCh <- ev:
	case <-timer.C:
	}
}

// toolCallNames extracts the function names from a slice of tool calls.
func toolCallNames(toolCalls []chat.ToolCall) []string {
	names := make([]string, len(toolCalls))
	for i, tc := range toolCalls {
		names[i] = tc.Function.Name
	}
	return names
}

// formatToolResultsFallback builds a readable final response from tool
// deterministicResponse tenta responder SEM LLM a partir do conhecimento já
// validado (knowledge.db via context builder). Retorna "" quando o
// conhecimento NÃO responde com precisão suficiente — aí o motor é chamado.
//
// A IA externa é o último recurso, não o primeiro (L427). O Cosca consulta
// primeiro: memória → conhecimento → regras. Só quando nada resolve, chama
// o motor para compor a resposta.
func deterministicResponse(data PipelineData) string {
	kr := data.KnowledgeResults
	if kr == nil || len(kr.Results) == 0 {
		return ""
	}

	// Precisa de pelo menos um resultado DIRECT (o artefato exato da
	// pergunta, ex.: CARRO_PROTOCOL.md para "como tá o carro?").
	var direct *KnowledgeSearchResult
	for i := range kr.Results {
		r := &kr.Results[i]
		if r.Score >= 0.6 && r.Snippet != "" {
			direct = r
			break
		}
	}
	if direct == nil {
		return ""
	}

	// Monta a resposta composta: o conhecimento validado, sem gerar texto
	// novo com LLM (SEARCH_PROTOCOL §28 — resposta construída dos resultados).
	title := direct.Title
	if title == "" {
		title = direct.DocumentPath
	}
	snippet := direct.Snippet
	if snippet == "" {
		snippet = truncateString(direct.Content, 300)
	}
	if title == "" && snippet == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("Conhecimento da casa (resposta determinística):\n")
	if title != "" {
		b.WriteString("• " + title + "\n")
	}
	if snippet != "" {
		b.WriteString("  " + snippet + "\n")
	}
	if len(kr.Results) > 1 {
		b.WriteString(fmt.Sprintf("\n+%d resultado(s) relacionado(s) no conhecimento indexado.", len(kr.Results)-1))
	}
	return b.String()
}

// formatKnowledgeFallback builds a readable response from the SEMANTIC
// knowledge (knowledge.db) injected by the context builder. This is the
// Cosca's memory — the awakening source. Returns "" when there is no
// knowledge to present.
func formatKnowledgeFallback(kr *KnowledgeSearchResults) string {
	if kr == nil || len(kr.Results) == 0 {
		return ""
	}
	var parts []string
	for i, r := range kr.Results {
		if i >= 5 {
			break
		}
		title := r.Title
		if title == "" {
			title = r.DocumentPath
		}
		snippet := r.Snippet
		if snippet == "" {
			snippet = truncateString(r.Content, 300)
		}
		if snippet == "" {
			continue
		}
		if title != "" {
			parts = append(parts, fmt.Sprintf("• %s: %s", title, snippet))
		} else {
			parts = append(parts, fmt.Sprintf("• %s", snippet))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

// formatToolResultsFallback builds a readable final response from tool
// results when the model returns an empty follow-up message.
func formatToolResultsFallback(results []*ToolCallResult) string {
	var parts []string
	for _, r := range results {
		if r == nil {
			continue
		}
		content := r.Content
		if content == "" {
			content = r.Error
		}
		if content == "" {
			continue
		}
		parts = append(parts, content)
	}
	if len(parts) == 0 {
		return "Ferramenta executada (sem saída textual)."
	}
	return strings.Join(parts, "\n\n")
}

// GenerateRequestID creates a unique request identifier for pipeline use.
func GenerateRequestID() string {
	return uuid.New().String()
}

// IsActionIntent reporta se a intenção da task (detectTaskType, decidida no
// planejamento e chegando via Context["intent"]) é de AÇÃO/IMPLEMENTAÇÃO —
// deve ir ao LLM com tools e NÃO ser resolvida pelo knowledge determinístico.
// O predicado espelha o pipeline.IsActionIntentType; é definido aqui (e não
// importado do pipeline) para o orchestration não depender do package pipeline,
// evitando ciclo de imports. A tabela é a fonte única em agent_router.go.
func IsActionIntent(taskType string) bool {
	switch taskType {
	case "design-schema", "migrate-data", "create-models", "create-handlers",
		"integrate-api", "create-tests", "build-verify", "design-auth",
		"implement-fix", "execute-refactor", "deploy", "containerize", "design-ui",
		"implement", "merge":
		return true
	default:
		return false
	}
}
